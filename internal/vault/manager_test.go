package vault

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// setupManager 创建测试用的管理器实例。
func setupManager(t *testing.T) *Manager {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	config := VaultConfig{
		DefaultAlgorithm:  AlgorithmAES256GCM,
		AutoLockMinutes:   30,
		MaxFailedAttempts: 3,
		KeyDerivation:     KeyDerivationArgon2id,
	}
	return NewManager(logger, config)
}

// setupRouter 创建测试用的 gin 路由。
func setupRouter(m *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger, _ := zap.NewDevelopment()
	h := NewHandler(m, logger)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)
	return r
}

// TestVaultError_Error 测试 VaultError 的 Error() 方法。
func TestVaultError_Error(t *testing.T) {
	// 无内部错误
	err := &VaultError{Code: "TEST", Message: "测试错误"}
	if err.Error() != "[TEST] 测试错误" {
		t.Errorf("期望 '[TEST] 测试错误'，得到 '%s'", err.Error())
	}

	// 有内部错误
	inner := &VaultError{Code: "INNER", Message: "内部"}
	err2 := &VaultError{Code: "OUTER", Message: "外部", Err: inner}
	expected := "[OUTER] 外部: [INNER] 内部"
	if err2.Error() != expected {
		t.Errorf("期望 '%s'，得到 '%s'", expected, err2.Error())
	}
}

// TestVaultError_Unwrap 测试 VaultError 的 Unwrap() 方法。
func TestVaultError_Unwrap(t *testing.T) {
	inner := &VaultError{Code: "INNER", Message: "内部"}
	err := &VaultError{Code: "OUTER", Message: "外部", Err: inner}
	if err.Unwrap() != inner {
		t.Error("Unwrap 应返回内部错误")
	}

	err2 := &VaultError{Code: "NO_INNER", Message: "无内部错误"}
	if err2.Unwrap() != nil {
		t.Error("无内部错误时 Unwrap 应返回 nil")
	}
}

// TestNewVaultError 测试 NewVaultError 工厂函数。
func TestNewVaultError(t *testing.T) {
	err := NewVaultError("CODE", "消息", nil)
	if err.Code != "CODE" || err.Message != "消息" || err.Err != nil {
		t.Error("NewVaultError 构造参数不正确")
	}
}

// TestNewManager_Defaults 测试 NewManager 使用默认配置。
func TestNewManager_Defaults(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	m := NewManager(logger, VaultConfig{})

	if m.config.DefaultAlgorithm != AlgorithmAES256GCM {
		t.Errorf("默认算法应为 %s，得到 %s", AlgorithmAES256GCM, m.config.DefaultAlgorithm)
	}
	if m.config.AutoLockMinutes != 30 {
		t.Errorf("默认自动锁定应为 30 分钟，得到 %d", m.config.AutoLockMinutes)
	}
	if m.config.MaxFailedAttempts != 5 {
		t.Errorf("默认最大失败次数应为 5，得到 %d", m.config.MaxFailedAttempts)
	}
	if m.config.KeyDerivation != KeyDerivationArgon2id {
		t.Errorf("默认密钥派生应为 argon2id，得到 %s", m.config.KeyDerivation)
	}
}

// TestCreateVault 测试创建保险库。
func TestCreateVault(t *testing.T) {
	m := setupManager(t)

	// 正常创建
	v, err := m.CreateVault("test-vault", "测试保险库", "/mnt/vault", AlgorithmAES256GCM, "testpass")
	if err != nil {
		t.Fatalf("创建保险库失败: %v", err)
	}
	if v.Name != "test-vault" {
		t.Errorf("名称应为 'test-vault'，得到 '%s'", v.Name)
	}
	if v.Status != StatusLocked {
		t.Errorf("新创建的保险库应为 locked 状态，得到 '%s'", v.Status)
	}
	if v.Algorithm != AlgorithmAES256GCM {
		t.Errorf("算法应为 %s，得到 %s", AlgorithmAES256GCM, v.Algorithm)
	}
	if v.ID == "" {
		t.Error("ID 不应为空")
	}

	// 名称重复
	_, err = m.CreateVault("test-vault", "重复", "/mnt/vault2", "", "testpass")
	if err == nil {
		t.Error("重复名称应返回错误")
	}
	if ve, ok := err.(*VaultError); !ok || ve.Code != "VAULT_ALREADY_EXISTS" {
		t.Errorf("应返回 VAULT_ALREADY_EXISTS 错误，得到 %v", err)
	}

	// 名称为空
	_, err = m.CreateVault("", "空名称", "/mnt/vault3", "", "testpass")
	if err == nil {
		t.Error("空名称应返回错误")
	}

	// 路径为空
	_, err = m.CreateVault("no-path", "无路径", "", "", "testpass")
	if err == nil {
		t.Error("空路径应返回错误")
	}

	// 无效算法
	_, err = m.CreateVault("bad-alg", "无效算法", "/mnt/vault4", "invalid", "testpass")
	if err == nil {
		t.Error("无效算法应返回错误")
	}

	// 使用默认算法
	v2, err := m.CreateVault("default-alg", "默认算法", "/mnt/vault5", "", "testpass")
	if err != nil {
		t.Fatalf("使用默认算法创建失败: %v", err)
	}
	if v2.Algorithm != AlgorithmAES256GCM {
		t.Errorf("应使用默认算法 %s，得到 %s", AlgorithmAES256GCM, v2.Algorithm)
	}
}

// TestCreateVault_ChaCha20 测试使用 ChaCha20-Poly1305 创建保险库。
func TestCreateVault_ChaCha20(t *testing.T) {
	m := setupManager(t)

	v, err := m.CreateVault("chacha-vault", "ChaCha20 保险库", "/mnt/chacha", AlgorithmChaCha20Poly1305, "chachapass")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if v.Algorithm != AlgorithmChaCha20Poly1305 {
		t.Errorf("算法应为 %s，得到 %s", AlgorithmChaCha20Poly1305, v.Algorithm)
	}
}

// TestUnlockVault 测试解锁保险库。
func TestUnlockVault(t *testing.T) {
	m := setupManager(t)

	v, _ := m.CreateVault("unlock-test", "解锁测试", "/mnt/unlock", "", "mypassword123")

	// 正常解锁
	err := m.UnlockVault(v.ID, "mypassword123")
	if err != nil {
		t.Fatalf("解锁失败: %v", err)
	}

	// 获取保险库验证状态
	got, _ := m.GetVault(v.ID)
	if got.Status != StatusUnlocked {
		t.Errorf("状态应为 unlocked，得到 %s", got.Status)
	}

	// 重复解锁
	err = m.UnlockVault(v.ID, "mypassword123")
	if err == nil {
		t.Error("重复解锁应返回错误")
	}
	if ve, ok := err.(*VaultError); !ok || ve.Code != "VAULT_ALREADY_UNLOCKED" {
		t.Errorf("应返回 VAULT_ALREADY_UNLOCKED，得到 %v", err)
	}

	// 不存在的 ID
	err = m.UnlockVault("nonexistent", "pass")
	if err == nil {
		t.Error("不存在的 ID 应返回错误")
	}
}

// TestUnlockVault_WrongPassphrase 测试密码错误。
func TestUnlockVault_WrongPassphrase(t *testing.T) {
	m := setupManager(t)

	v, _ := m.CreateVault("wrong-pwd", "密码测试", "/mnt/wrong", "", "correctpassword")

	// 错误密码
	err := m.UnlockVault(v.ID, "wrongpassword")
	if err == nil {
		t.Error("错误密码应返回错误")
	}
	if ve, ok := err.(*VaultError); !ok || ve.Code != "INVALID_PASSPHRASE" {
		t.Errorf("应返回 INVALID_PASSPHRASE，得到 %v", err)
	}
}

// TestUnlockVault_MaxAttempts 测试最大失败次数限制。
func TestUnlockVault_MaxAttempts(t *testing.T) {
	m := setupManager(t)

	v, _ := m.CreateVault("max-attempts", "次数测试", "/mnt/max", "", "correctpassword")

	// 连续错误密码直到超限
	for i := 0; i < 3; i++ {
		_ = m.UnlockVault(v.ID, "wrong")
	}

	// 第 4 次应返回超限错误
	err := m.UnlockVault(v.ID, "wrong")
	if err == nil {
		t.Error("超过最大次数应返回错误")
	}
	if ve, ok := err.(*VaultError); !ok || ve.Code != "MAX_ATTEMPTS_EXCEEDED" {
		t.Errorf("应返回 MAX_ATTEMPTS_EXCEEDED，得到 %v", err)
	}
}

// TestLockVault 测试锁定保险库。
func TestLockVault(t *testing.T) {
	m := setupManager(t)

	v, _ := m.CreateVault("lock-test", "锁定测试", "/mnt/lock", "", "lockpass")

	// 锁定未解锁的保险库（已经是 locked 状态）
	err := m.LockVault(v.ID)
	if err == nil {
		t.Error("锁定已锁定的保险库应返回错误")
	}

	// 解锁后锁定
	_ = m.UnlockVault(v.ID, "lockpass")
	err = m.LockVault(v.ID)
	if err != nil {
		t.Fatalf("锁定失败: %v", err)
	}

	got, _ := m.GetVault(v.ID)
	if got.Status != StatusLocked {
		t.Errorf("应为 locked 状态，得到 %s", got.Status)
	}

	// 不存在的 ID
	err = m.LockVault("nonexistent")
	if err == nil {
		t.Error("不存在的 ID 应返回错误")
	}
}

// TestDeleteVault 测试删除保险库。
func TestDeleteVault(t *testing.T) {
	m := setupManager(t)

	v, _ := m.CreateVault("delete-test", "删除测试", "/mnt/delete", "", "deletepass")

	// 解锁状态下不能删除
	_ = m.UnlockVault(v.ID, "deletepass")
	err := m.DeleteVault(v.ID)
	if err == nil {
		t.Error("解锁状态下删除应返回错误")
	}

	// 锁定后删除
	_ = m.LockVault(v.ID)
	err = m.DeleteVault(v.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	// 验证已删除
	_, err = m.GetVault(v.ID)
	if err == nil {
		t.Error("已删除的保险库不应存在")
	}

	// 不存在的 ID
	err = m.DeleteVault("nonexistent")
	if err == nil {
		t.Error("不存在的 ID 应返回错误")
	}
}

// TestListVaults 测试列出保险库。
func TestListVaults(t *testing.T) {
	m := setupManager(t)

	// 空列表
	vaults := m.ListVaults()
	if len(vaults) != 0 {
		t.Errorf("空列表长度应为 0，得到 %d", len(vaults))
	}

	// 创建多个
	_, _ = m.CreateVault("v1", "保险库1", "/mnt/v1", "", "pass1")
	_, _ = m.CreateVault("v2", "保险库2", "/mnt/v2", "", "pass2")

	vaults = m.ListVaults()
	if len(vaults) != 2 {
		t.Errorf("列表长度应为 2，得到 %d", len(vaults))
	}
}

// TestGetVault 测试获取保险库详情。
func TestGetVault(t *testing.T) {
	m := setupManager(t)

	v, _ := m.CreateVault("get-test", "获取测试", "/mnt/get", "", "getpass")

	got, err := m.GetVault(v.ID)
	if err != nil {
		t.Fatalf("获取失败: %v", err)
	}
	if got.ID != v.ID {
		t.Errorf("ID 不匹配: 期望 %s，得到 %s", v.ID, got.ID)
	}

	// 不存在
	_, err = m.GetVault("nonexistent")
	if err == nil {
		t.Error("不存在的 ID 应返回错误")
	}
}

// TestGetStats 测试统计信息。
func TestGetStats(t *testing.T) {
	m := setupManager(t)

	// 空统计
	stats := m.GetStats()
	if stats.TotalVaults != 0 {
		t.Errorf("空状态下 TotalVaults 应为 0，得到 %d", stats.TotalVaults)
	}

	// 创建并解锁一个
	v, _ := m.CreateVault("stats-test", "统计测试", "/mnt/stats", "", "statspass")
	_ = m.UnlockVault(v.ID, "statspass")

	stats = m.GetStats()
	if stats.TotalVaults != 1 {
		t.Errorf("TotalVaults 应为 1，得到 %d", stats.TotalVaults)
	}
	if stats.UnlockedVaults != 1 {
		t.Errorf("UnlockedVaults 应为 1，得到 %d", stats.UnlockedVaults)
	}

	// 锁定后
	_ = m.LockVault(v.ID)
	stats = m.GetStats()
	if stats.UnlockedVaults != 0 {
		t.Errorf("锁定后 UnlockedVaults 应为 0，得到 %d", stats.UnlockedVaults)
	}
}

// ==================== HTTP Handler 测试 ====================

// TestHandler_CreateVault 测试创建保险库 HTTP 接口。
func TestHandler_CreateVault(t *testing.T) {
	m := setupManager(t)
	r := setupRouter(m)

	body := `{"name":"http-vault","description":"HTTP 测试","mount_path":"/mnt/http"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vaults", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d，得到 %d", http.StatusCreated, w.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.Success {
		t.Error("应返回成功")
	}

	// 重复名称
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/vaults", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Errorf("重复名称期望 %d，得到 %d", http.StatusConflict, w2.Code)
	}

	// 缺少必填字段
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/vaults", bytes.NewBufferString(`{"description":"缺少字段"}`))
	req3.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Errorf("缺少字段期望 %d，得到 %d", http.StatusBadRequest, w3.Code)
	}
}

// TestHandler_ListVaults 测试列出保险库 HTTP 接口。
func TestHandler_ListVaults(t *testing.T) {
	m := setupManager(t)
	r := setupRouter(m)

	// 空列表
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vaults", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 %d，得到 %d", http.StatusOK, w.Code)
	}

	var resp apiResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("应返回成功")
	}

	// 创建后再列表
	body := `{"name":"list-test","mount_path":"/mnt/list"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/vaults", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/vaults", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	var resp3 apiResponse
	_ = json.Unmarshal(w3.Body.Bytes(), &resp3)
	data, _ := json.Marshal(resp3.Data)
	var vaults []Vault
	_ = json.Unmarshal(data, &vaults)
	if len(vaults) != 1 {
		t.Errorf("期望 1 个保险库，得到 %d", len(vaults))
	}
}

// TestHandler_GetVault 测试获取保险库详情 HTTP 接口。
func TestHandler_GetVault(t *testing.T) {
	m := setupManager(t)
	r := setupRouter(m)

	// 先创建
	body := `{"name":"get-http","mount_path":"/mnt/get"}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/vaults", bytes.NewBufferString(body))
	reqCreate.Header.Set("Content-Type", "application/json")
	wCreate := httptest.NewRecorder()
	r.ServeHTTP(wCreate, reqCreate)

	var createResp apiResponse
	_ = json.Unmarshal(wCreate.Body.Bytes(), &createResp)
	data, _ := json.Marshal(createResp.Data)
	var v Vault
	_ = json.Unmarshal(data, &v)

	// 获取详情
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vaults/"+v.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 %d，得到 %d", http.StatusOK, w.Code)
	}

	// 不存在
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/vaults/nonexistent", nil)
	w404 := httptest.NewRecorder()
	r.ServeHTTP(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Errorf("不存在期望 %d，得到 %d", http.StatusNotFound, w404.Code)
	}
}

// TestHandler_UnlockVault 测试解锁保险库 HTTP 接口。
func TestHandler_UnlockVault(t *testing.T) {
	m := setupManager(t)
	r := setupRouter(m)

	// 创建
	body := `{"name":"unlock-http","mount_path":"/mnt/unlock"}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/vaults", bytes.NewBufferString(body))
	reqCreate.Header.Set("Content-Type", "application/json")
	wCreate := httptest.NewRecorder()
	r.ServeHTTP(wCreate, reqCreate)

	var createResp apiResponse
	_ = json.Unmarshal(wCreate.Body.Bytes(), &createResp)
	data, _ := json.Marshal(createResp.Data)
	var v Vault
	_ = json.Unmarshal(data, &v)

	// 解锁
	unlockBody := `{"passphrase":"testpass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vaults/"+v.ID+"/unlock", bytes.NewBufferString(unlockBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 %d，得到 %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// 错误密码
	wrongBody := `{"passphrase":"wrong"}`
	reqWrong := httptest.NewRequest(http.MethodPost, "/api/v1/vaults/"+v.ID+"/unlock", bytes.NewBufferString(wrongBody))
	reqWrong.Header.Set("Content-Type", "application/json")
	wWrong := httptest.NewRecorder()
	r.ServeHTTP(wWrong, reqWrong)

	// 已解锁状态再请求应返回冲突
	if wWrong.Code != http.StatusConflict {
		t.Errorf("已解锁再请求期望 %d，得到 %d", http.StatusConflict, wWrong.Code)
	}

	// 缺少 passphrase
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/vaults/"+v.ID+"/unlock", bytes.NewBufferString(`{}`))
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	r.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Errorf("缺少字段期望 %d，得到 %d", http.StatusBadRequest, wBad.Code)
	}
}

// TestHandler_LockVault 测试锁定保险库 HTTP 接口。
func TestHandler_LockVault(t *testing.T) {
	m := setupManager(t)
	r := setupRouter(m)

	// 创建 + 解锁
	body := `{"name":"lock-http","mount_path":"/mnt/lock"}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/vaults", bytes.NewBufferString(body))
	reqCreate.Header.Set("Content-Type", "application/json")
	wCreate := httptest.NewRecorder()
	r.ServeHTTP(wCreate, reqCreate)

	var createResp apiResponse
	_ = json.Unmarshal(wCreate.Body.Bytes(), &createResp)
	data, _ := json.Marshal(createResp.Data)
	var v Vault
	_ = json.Unmarshal(data, &v)

	unlockBody := `{"passphrase":"lockpass"}`
	reqUnlock := httptest.NewRequest(http.MethodPost, "/api/v1/vaults/"+v.ID+"/unlock", bytes.NewBufferString(unlockBody))
	reqUnlock.Header.Set("Content-Type", "application/json")
	wUnlock := httptest.NewRecorder()
	r.ServeHTTP(wUnlock, reqUnlock)

	// 锁定
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vaults/"+v.ID+"/lock", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 %d，得到 %d", http.StatusOK, w.Code)
	}

	// 重复锁定
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/vaults/"+v.ID+"/lock", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Errorf("重复锁定期望 %d，得到 %d", http.StatusConflict, w2.Code)
	}

	// 不存在
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/vaults/nonexistent/lock", nil)
	w404 := httptest.NewRecorder()
	r.ServeHTTP(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Errorf("不存在期望 %d，得到 %d", http.StatusNotFound, w404.Code)
	}
}

// TestHandler_DeleteVault 测试删除保险库 HTTP 接口。
func TestHandler_DeleteVault(t *testing.T) {
	m := setupManager(t)
	r := setupRouter(m)

	// 创建
	body := `{"name":"delete-http","mount_path":"/mnt/delete"}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/vaults", bytes.NewBufferString(body))
	reqCreate.Header.Set("Content-Type", "application/json")
	wCreate := httptest.NewRecorder()
	r.ServeHTTP(wCreate, reqCreate)

	var createResp apiResponse
	_ = json.Unmarshal(wCreate.Body.Bytes(), &createResp)
	data, _ := json.Marshal(createResp.Data)
	var v Vault
	_ = json.Unmarshal(data, &v)

	// 解锁状态下删除 → 冲突
	unlockBody := `{"passphrase":"delpass"}`
	reqUnlock := httptest.NewRequest(http.MethodPost, "/api/v1/vaults/"+v.ID+"/unlock", bytes.NewBufferString(unlockBody))
	reqUnlock.Header.Set("Content-Type", "application/json")
	wUnlock := httptest.NewRecorder()
	r.ServeHTTP(wUnlock, reqUnlock)

	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/vaults/"+v.ID, nil)
	wDel := httptest.NewRecorder()
	r.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusConflict {
		t.Errorf("解锁删除期望 %d，得到 %d", http.StatusConflict, wDel.Code)
	}

	// 锁定后删除
	reqLock := httptest.NewRequest(http.MethodPost, "/api/v1/vaults/"+v.ID+"/lock", nil)
	wLock := httptest.NewRecorder()
	r.ServeHTTP(wLock, reqLock)

	reqDel2 := httptest.NewRequest(http.MethodDelete, "/api/v1/vaults/"+v.ID, nil)
	wDel2 := httptest.NewRecorder()
	r.ServeHTTP(wDel2, reqDel2)
	if wDel2.Code != http.StatusOK {
		t.Errorf("锁定删除期望 %d，得到 %d", http.StatusOK, wDel2.Code)
	}

	// 不存在
	req404 := httptest.NewRequest(http.MethodDelete, "/api/v1/vaults/nonexistent", nil)
	w404 := httptest.NewRecorder()
	r.ServeHTTP(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Errorf("不存在期望 %d，得到 %d", http.StatusNotFound, w404.Code)
	}
}

// TestHandler_GetStats 测试统计信息 HTTP 接口。
func TestHandler_GetStats(t *testing.T) {
	m := setupManager(t)
	r := setupRouter(m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vaults/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 %d，得到 %d", http.StatusOK, w.Code)
	}

	var resp apiResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("应返回成功")
	}
}

// TestEncryption_AES256GCM 测试 AES-256-GCM 加解密。
func TestEncryption_AES256GCM(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("这是一段测试加密数据 Hello World!")

	encrypted, err := encryptAES256GCM(key, plaintext)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	decrypted, err := decryptAES256GCM(key, encrypted)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("解密结果不匹配: 期望 '%s'，得到 '%s'", plaintext, decrypted)
	}

	// 错误密钥解密
	wrongKey := make([]byte, 32)
	for i := range wrongKey {
		wrongKey[i] = byte(i + 1)
	}
	_, err = decryptAES256GCM(wrongKey, encrypted)
	if err == nil {
		t.Error("错误密钥解密应失败")
	}
}

// TestEncryption_ChaCha20Poly1305 测试 ChaCha20-Poly1305 加解密。
func TestEncryption_ChaCha20Poly1305(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 100)
	}

	plaintext := []byte("ChaCha20-Poly1305 加密测试数据")

	encrypted, err := encryptChaCha20Poly1305(key, plaintext)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	decrypted, err := decryptChaCha20Poly1305(key, encrypted)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("解密结果不匹配")
	}
}

// TestKeyDerivation_Argon2id 测试 Argon2id 密钥派生。
func TestKeyDerivation_Argon2id(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := VaultConfig{
		DefaultAlgorithm: AlgorithmAES256GCM,
		KeyDerivation:    KeyDerivationArgon2id,
	}
	m := NewManager(logger, config)

	key1, err := m.deriveKey("password123", "salt1")
	if err != nil {
		t.Fatalf("派生失败: %v", err)
	}
	if len(key1) != 32 {
		t.Errorf("密钥长度应为 32，得到 %d", len(key1))
	}

	// 相同输入应得到相同密钥
	key2, _ := m.deriveKey("password123", "salt1")
	if string(key1) != string(key2) {
		t.Error("相同输入应产生相同密钥")
	}

	// 不同输入应得到不同密钥
	key3, _ := m.deriveKey("password456", "salt1")
	if string(key1) == string(key3) {
		t.Error("不同密码应产生不同密钥")
	}
}

// TestKeyDerivation_PBKDF2 测试 PBKDF2 密钥派生。
func TestKeyDerivation_PBKDF2(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := VaultConfig{
		DefaultAlgorithm: AlgorithmAES256GCM,
		KeyDerivation:    KeyDerivationPBKDF2,
	}
	m := NewManager(logger, config)

	key1, err := m.deriveKey("password123", "salt1")
	if err != nil {
		t.Fatalf("派生失败: %v", err)
	}
	if len(key1) != 32 {
		t.Errorf("密钥长度应为 32，得到 %d", len(key1))
	}

	// 确定性
	key2, _ := m.deriveKey("password123", "salt1")
	if string(key1) != string(key2) {
		t.Error("相同输入应产生相同密钥")
	}
}

// TestUnlockWithPBKDF2 测试使用 PBKDF2 派生的密码解锁。
func TestUnlockWithPBKDF2(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := VaultConfig{
		DefaultAlgorithm:  AlgorithmAES256GCM,
		MaxFailedAttempts: 5,
		KeyDerivation:     KeyDerivationPBKDF2,
	}
	m := NewManager(logger, config)

	v, err := m.CreateVault("pbkdf2-vault", "PBKDF2 测试", "/mnt/pbkdf2", "", "my-pbkdf2-password")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	err = m.UnlockVault(v.ID, "my-pbkdf2-password")
	if err != nil {
		t.Fatalf("PBKDF2 解锁失败: %v", err)
	}
}

// TestUnlockWithChaCha20 测试使用 ChaCha20-Poly1305 解锁。
func TestUnlockWithChaCha20(t *testing.T) {
	m := setupManager(t)

	v, err := m.CreateVault("chacha-unlock", "ChaCha 解锁", "/mnt/chacha", AlgorithmChaCha20Poly1305, "chacha-password")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	err = m.UnlockVault(v.ID, "chacha-password")
	if err != nil {
		t.Fatalf("ChaCha20 解锁失败: %v", err)
	}
}

// TestFullWorkflow 测试完整工作流程：创建→解锁→锁定→删除。
func TestFullWorkflow(t *testing.T) {
	m := setupManager(t)

	// 1. 创建
	v, err := m.CreateVault("workflow-vault", "完整流程测试", "/mnt/workflow", "", "workflow-pass")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if v.Status != StatusLocked {
		t.Fatal("创建后应为 locked")
	}

	// 2. 解锁
	err = m.UnlockVault(v.ID, "workflow-pass")
	if err != nil {
		t.Fatalf("解锁失败: %v", err)
	}
	got, _ := m.GetVault(v.ID)
	if got.Status != StatusUnlocked {
		t.Fatal("解锁后应为 unlocked")
	}

	// 3. 统计
	stats := m.GetStats()
	if stats.UnlockedVaults != 1 {
		t.Fatalf("UnlockedVaults 应为 1，得到 %d", stats.UnlockedVaults)
	}

	// 4. 锁定
	err = m.LockVault(v.ID)
	if err != nil {
		t.Fatalf("锁定失败: %v", err)
	}

	// 5. 删除
	err = m.DeleteVault(v.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	// 6. 验证删除
	_, err = m.GetVault(v.ID)
	if err == nil {
		t.Fatal("删除后不应存在")
	}
}
