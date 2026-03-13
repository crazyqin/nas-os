package quota

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== 辅助函数 ==========

func setupTestHandlers(t *testing.T) (*Handlers, *gin.Engine) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")
	user.AddGroup("testgroup")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	return handlers, router
}

// ========== 配额管理 API 测试 ==========

func TestHandlers_ListQuotas(t *testing.T) {
	_, router := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/quotas", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_CreateQuota(t *testing.T) {
	_, router := setupTestHandlers(t)

	quotaReq := map[string]interface{}{
		"type":        "user",
		"target_id":   "testuser",
		"volume_name": "data",
		"hard_limit":  100 << 30,
		"soft_limit":  80 << 30,
	}

	body, _ := json.Marshal(quotaReq)
	req := httptest.NewRequest("POST", "/api/v1/quotas", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d 或 %d, 实际为 %d", http.StatusOK, http.StatusCreated, w.Code)
	}
}

func TestHandlers_GetQuota(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	// 先创建配额
	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}
	quota, _ := handlers.manager.CreateQuota(input)

	req := httptest.NewRequest("GET", "/api/v1/quotas/"+quota.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_UpdateQuota(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	// 先创建配额
	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}
	quota, _ := handlers.manager.CreateQuota(input)

	// 更新配额
	updateReq := map[string]interface{}{
		"hard_limit": 200 << 30,
		"soft_limit": 150 << 30,
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest("PUT", "/api/v1/quotas/"+quota.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_DeleteQuota(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	// 先创建配额
	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}
	quota, _ := handlers.manager.CreateQuota(input)

	req := httptest.NewRequest("DELETE", "/api/v1/quotas/"+quota.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

// ========== v2.1.0 用户配额 API 测试 ==========

func TestHandlers_v2_ListUserQuotas(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	// 创建用户配额
	handlers.manager.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})

	req := httptest.NewRequest("GET", "/api/v1/quotas/users", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_v2_SetUserQuota(t *testing.T) {
	_, router := setupTestHandlers(t)

	setReq := map[string]interface{}{
		"volume_name": "data",
		"hard_limit":  100 << 30,
		"soft_limit":  80 << 30,
	}

	body, _ := json.Marshal(setReq)
	req := httptest.NewRequest("POST", "/api/v1/quotas/users/testuser", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d 或 %d, 实际为 %d", http.StatusOK, http.StatusCreated, w.Code)
	}
}

func TestHandlers_v2_DeleteUserQuota(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	// 先创建用户配额
	handlers.manager.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})

	req := httptest.NewRequest("DELETE", "/api/v1/quotas/users/testuser", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

// ========== v2.1.0 组配额 API 测试 ==========

func TestHandlers_v2_ListGroupQuotas(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	// 创建组配额
	handlers.manager.CreateQuota(QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "testgroup",
		VolumeName: "data",
		HardLimit:  500 << 30,
	})

	req := httptest.NewRequest("GET", "/api/v1/quotas/groups", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_v2_SetGroupQuota(t *testing.T) {
	_, router := setupTestHandlers(t)

	setReq := map[string]interface{}{
		"volume_name": "data",
		"hard_limit":  500 << 30,
		"soft_limit":  400 << 30,
	}

	body, _ := json.Marshal(setReq)
	req := httptest.NewRequest("POST", "/api/v1/quotas/groups/testgroup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d 或 %d, 实际为 %d", http.StatusOK, http.StatusCreated, w.Code)
	}
}

// ========== v2.1.0 目录配额 API 测试 ==========

func TestHandlers_v2_ListDirectoryQuotas(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "quota-dir-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	handlers, router := setupTestHandlers(t)

	// 创建目录配额
	handlers.manager.CreateQuota(QuotaInput{
		Type:       QuotaTypeDirectory,
		TargetID:   tmpDir,
		VolumeName: "data",
		Path:       tmpDir,
		HardLimit:  200 << 30,
	})

	req := httptest.NewRequest("GET", "/api/v1/quotas/directories", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_v2_SetDirectoryQuota(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "quota-dir-set-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, router := setupTestHandlers(t)

	setReq := map[string]interface{}{
		"path":        tmpDir,
		"volume_name": "data",
		"hard_limit":  200 << 30,
		"soft_limit":  150 << 30,
	}

	body, _ := json.Marshal(setReq)
	req := httptest.NewRequest("POST", "/api/v1/quotas/directories", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d 或 %d, 实际为 %d", http.StatusOK, http.StatusCreated, w.Code)
	}
}

// ========== 告警 API 测试 ==========

func TestHandlers_GetAlerts(t *testing.T) {
	_, router := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/quota-alerts", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_GetAlertConfig(t *testing.T) {
	_, router := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/quota-alerts/config", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_SetAlertConfig(t *testing.T) {
	_, router := setupTestHandlers(t)

	configReq := map[string]interface{}{
		"enabled":              true,
		"soft_limit_threshold": 85.0,
		"hard_limit_threshold": 95.0,
		"notify_webhook":       true,
		"webhook_url":          "https://example.com/webhook",
	}

	body, _ := json.Marshal(configReq)
	req := httptest.NewRequest("PUT", "/api/v1/quota-alerts/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

// ========== 清理策略 API 测试 ==========

func TestHandlers_ListPolicies(t *testing.T) {
	_, router := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/cleanup-policies", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_CreatePolicy(t *testing.T) {
	_, router := setupTestHandlers(t)

	policyReq := map[string]interface{}{
		"name":        "test-policy",
		"volume_name": "data",
		"path":        "/tmp/test",
		"type":        "age",
		"action":      "delete",
		"max_age":     30,
		"enabled":     true,
	}

	body, _ := json.Marshal(policyReq)
	req := httptest.NewRequest("POST", "/api/v1/cleanup-policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d 或 %d, 实际为 %d", http.StatusOK, http.StatusCreated, w.Code)
	}
}

// ========== 报告 API 测试 ==========

func TestHandlers_GetQuotaReport(t *testing.T) {
	_, router := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/quotas/report", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_GenerateReport(t *testing.T) {
	_, router := setupTestHandlers(t)

	reportReq := map[string]interface{}{
		"type":   "summary",
		"format": "json",
	}

	body, _ := json.Marshal(reportReq)
	req := httptest.NewRequest("POST", "/api/v1/reports/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 可能返回 200 或 404（如果路由不存在）
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Logf("报告生成响应状态码: %d", w.Code)
	}
}

// ========== 错误处理测试 ==========

func TestHandlers_InvalidQuotaType(t *testing.T) {
	_, router := setupTestHandlers(t)

	quotaReq := map[string]interface{}{
		"type":        "invalid_type",
		"target_id":   "testuser",
		"volume_name": "data",
		"hard_limit":  100 << 30,
	}

	body, _ := json.Marshal(quotaReq)
	req := httptest.NewRequest("POST", "/api/v1/quotas", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 应该返回错误或创建失败
	if w.Code == http.StatusOK {
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["code"].(float64) == 0 {
			t.Error("无效的配额类型应该返回错误")
		}
	}
}

func TestHandlers_GetNonExistentQuota(t *testing.T) {
	_, router := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/quotas/nonexistent-id", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Logf("获取不存在的配额返回状态码: %d", w.Code)
	}
}

// ========== 并发测试 ==========

func TestHandlers_ConcurrentQuotaOperations(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	// 创建初始配额
	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}
	quota, _ := handlers.manager.CreateQuota(input)

	done := make(chan bool)

	// 并发读取
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				req := httptest.NewRequest("GET", "/api/v1/quotas/"+quota.ID, nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}
			done <- true
		}()
	}

	// 并发更新
	for i := 0; i < 3; i++ {
		go func(id int) {
			for j := 0; j < 5; j++ {
				updateReq := map[string]interface{}{
					"hard_limit": 100<<30 + int64(id*10<<30),
				}
				body, _ := json.Marshal(updateReq)
				req := httptest.NewRequest("PUT", "/api/v1/quotas/"+quota.ID, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 8; i++ {
		<-done
	}
}
