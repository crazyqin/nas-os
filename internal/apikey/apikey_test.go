package apikey

import (
	"testing"
	"time"
)

func TestCreateKey(t *testing.T) {
	manager := NewManager()
	
	req := &CreateKeyRequest{
		Name:        "测试密钥",
		Description: "用于测试",
		UserID:      "user1",
		Permissions: []string{"read"},
		Scopes:      []string{"read_only"},
		ExpiresIn:   24, // 24小时后过期
	}
	
	key, err := manager.CreateKey(req)
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}
	
	if key.Name != "测试密钥" {
		t.Errorf("期望名称 '测试密钥'，实际 '%s'", key.Name)
	}
	if key.UserID != "user1" {
		t.Errorf("期望用户ID 'user1'，实际 '%s'", key.UserID)
	}
	if key.Status != StatusActive {
		t.Errorf("期望状态 '%s'，实际 '%s'", StatusActive, key.Status)
	}
	if key.Key == "" {
		t.Error("密钥值不应为空")
	}
	if key.ExpiresAt == nil {
		t.Error("过期时间不应为空")
	}
}

func TestCreateKeyWithoutExpiration(t *testing.T) {
	manager := NewManager()
	
	req := &CreateKeyRequest{
		Name:   "永久密钥",
		UserID: "user1",
	}
	
	key, err := manager.CreateKey(req)
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}
	
	if key.ExpiresAt != nil {
		t.Error("永久密钥不应有过期时间")
	}
}

func TestCreateDuplicateKeyName(t *testing.T) {
	manager := NewManager()
	
	req := &CreateKeyRequest{
		Name:   "重复名称",
		UserID: "user1",
	}
	
	_, err := manager.CreateKey(req)
	if err != nil {
		t.Fatalf("第一次创建失败: %v", err)
	}
	
	_, err = manager.CreateKey(req)
	if err == nil {
		t.Error("期望返回重复错误，但没有")
	}
}

func TestValidateKey(t *testing.T) {
	manager := NewManager()
	
	req := &CreateKeyRequest{
		Name:        "验证测试",
		UserID:      "user1",
		Permissions: []string{"read"},
		Scopes:      []string{"read_only"},
	}
	
	key, _ := manager.CreateKey(req)
	
	// 验证有效密钥
	validateReq := &ValidateKeyRequest{
		Key: key.Key,
	}
	
	result := manager.ValidateKey(validateReq)
	if !result.Valid {
		t.Errorf("密钥应有效，错误: %s", result.Error)
	}
	if result.UserID != "user1" {
		t.Errorf("期望用户ID 'user1'，实际 '%s'", result.UserID)
	}
}

func TestValidateInvalidKey(t *testing.T) {
	manager := NewManager()
	
	validateReq := &ValidateKeyRequest{
		Key: "invalid_key",
	}
	
	result := manager.ValidateKey(validateReq)
	if result.Valid {
		t.Error("无效密钥不应验证通过")
	}
}

func TestValidateExpiredKey(t *testing.T) {
	manager := NewManager()
	
	// 创建已过期的密钥
	req := &CreateKeyRequest{
		Name:      "过期密钥",
		UserID:    "user1",
		ExpiresIn: -1, // 已过期
	}
	
	key, _ := manager.CreateKey(req)
	
	// 手动设置过期时间为过去
	expiredTime := time.Now().Add(-1 * time.Hour)
	key.ExpiresAt = &expiredTime
	
	validateReq := &ValidateKeyRequest{
		Key: key.Key,
	}
	
	result := manager.ValidateKey(validateReq)
	if result.Valid {
		t.Error("过期密钥不应验证通过")
	}
}

func TestRevokeKey(t *testing.T) {
	manager := NewManager()
	
	req := &CreateKeyRequest{
		Name:   "待撤销",
		UserID: "user1",
	}
	
	key, _ := manager.CreateKey(req)
	
	err := manager.RevokeKey(key.ID, "admin", "测试撤销")
	if err != nil {
		t.Fatalf("撤销密钥失败: %v", err)
	}
	
	// 验证密钥已撤销
	validateReq := &ValidateKeyRequest{
		Key: key.Key,
	}
	
	result := manager.ValidateKey(validateReq)
	if result.Valid {
		t.Error("已撤销的密钥不应验证通过")
	}
}

func TestDeleteKey(t *testing.T) {
	manager := NewManager()
	
	req := &CreateKeyRequest{
		Name:   "待删除",
		UserID: "user1",
	}
	
	key, _ := manager.CreateKey(req)
	
	// 只能删除已撤销的密钥
	err := manager.DeleteKey(key.ID)
	if err == nil {
		t.Error("不应删除活跃的密钥")
	}
	
	// 先撤销
	manager.RevokeKey(key.ID, "admin", "测试")
	
	// 现在可以删除
	err = manager.DeleteKey(key.ID)
	if err != nil {
		t.Fatalf("删除密钥失败: %v", err)
	}
	
	// 验证密钥已删除
	_, err = manager.GetKey(key.ID)
	if err == nil {
		t.Error("密钥应已删除")
	}
}

func TestGetUserKeys(t *testing.T) {
	manager := NewManager()
	
	// 创建多个密钥
	manager.CreateKey(&CreateKeyRequest{Name: "密钥1", UserID: "user1"})
	manager.CreateKey(&CreateKeyRequest{Name: "密钥2", UserID: "user1"})
	manager.CreateKey(&CreateKeyRequest{Name: "密钥3", UserID: "user2"})
	
	keys := manager.GetUserKeys("user1")
	if len(keys) != 2 {
		t.Errorf("期望2个密钥，实际 %d", len(keys))
	}
	
	// 验证密钥值为空
	for _, key := range keys {
		if key.Key != "" {
			t.Error("列表中的密钥值应为空")
		}
	}
}

func TestGetStats(t *testing.T) {
	manager := NewManager()
	
	// 创建不同状态的密钥
	key1, _ := manager.CreateKey(&CreateKeyRequest{Name: "活跃1", UserID: "user1"})
	manager.CreateKey(&CreateKeyRequest{Name: "活跃2", UserID: "user1"})
	manager.RevokeKey(key1.ID, "admin", "测试")
	
	stats := manager.GetStats()
	if stats.TotalKeys != 2 {
		t.Errorf("期望总密钥数2，实际 %d", stats.TotalKeys)
	}
	if stats.ActiveKeys != 1 {
		t.Errorf("期望活跃密钥数1，实际 %d", stats.ActiveKeys)
	}
	if stats.RevokedKeys != 1 {
		t.Errorf("期望撤销密钥数1，实际 %d", stats.RevokedKeys)
	}
}

func TestCleanupExpiredKeys(t *testing.T) {
	manager := NewManager()
	
	// 创建即将过期的密钥
	req := &CreateKeyRequest{
		Name:      "即将过期",
		UserID:    "user1",
		ExpiresIn: -1, // 已过期
	}
	
	key, _ := manager.CreateKey(req)
	
	// 手动设置过期时间
	expiredTime := time.Now().Add(-1 * time.Hour)
	key.ExpiresAt = &expiredTime
	
	count := manager.CleanupExpiredKeys()
	if count != 1 {
		t.Errorf("期望清理1个密钥，实际 %d", count)
	}
	
	// 验证状态已更新
	updatedKey, _ := manager.GetKey(key.ID)
	if updatedKey.Status != StatusExpired {
		t.Errorf("期望状态 '%s'，实际 '%s'", StatusExpired, updatedKey.Status)
	}
}

func TestAuditLogs(t *testing.T) {
	manager := NewManager()
	
	// 创建密钥（会产生审计日志）
	key, _ := manager.CreateKey(&CreateKeyRequest{Name: "审计测试", UserID: "user1"})
	
	// 撤销密钥（会产生审计日志）
	manager.RevokeKey(key.ID, "admin", "测试审计")
	
	// 获取审计日志
	logs := manager.GetAuditLogs("", 10)
	if len(logs) < 2 {
		t.Errorf("期望至少2条审计日志，实际 %d", len(logs))
	}
	
	// 获取特定密钥的审计日志
	keyLogs := manager.GetAuditLogs(key.ID, 10)
	if len(keyLogs) != 2 {
		t.Errorf("期望2条密钥审计日志，实际 %d", len(keyLogs))
	}
}

func TestPermissions(t *testing.T) {
	manager := NewManager()
	
	permissions := manager.GetPermissions()
	if len(permissions) == 0 {
		t.Error("应有默认权限")
	}
	
	// 验证包含基本权限
	found := false
	for _, p := range permissions {
		if p.ID == "read" {
			found = true
			break
		}
	}
	if !found {
		t.Error("应包含read权限")
	}
}

func TestScopes(t *testing.T) {
	manager := NewManager()
	
	scopes := manager.GetScopes()
	if len(scopes) == 0 {
		t.Error("应有默认作用域")
	}
	
	// 验证包含基本作用域
	found := false
	for _, s := range scopes {
		if s.ID == "read_only" {
			found = true
			break
		}
	}
	if !found {
		t.Error("应包含read_only作用域")
	}
}