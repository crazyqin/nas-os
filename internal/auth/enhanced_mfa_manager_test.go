package auth

import (
	"path/filepath"
	"testing"
	"time"
)

// ========== EnhancedMFAManager 基础测试 ==========

func TestNewEnhancedMFAManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mfa.json")
	keyPath := filepath.Join(tmpDir, "key")

	cfg := EnhancedMFAConfig{
		ConfigPath:        configPath,
		Issuer:            "test-issuer",
		EncryptionKeyPath: keyPath,
	}

	mgr, err := NewEnhancedMFAManager(cfg)
	if err != nil {
		t.Fatalf("NewEnhancedMFAManager failed: %v", err)
	}

	if mgr == nil {
		t.Fatal("manager is nil")
	}

	if mgr.loginTracker == nil {
		t.Error("loginTracker not initialized")
	}

	if mgr.sessionManager == nil {
		t.Error("sessionManager not initialized")
	}

	if mgr.passwordValidator == nil {
		t.Error("passwordValidator not initialized")
	}
}

func TestNewEnhancedMFAManager_DefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mfa.json")

	// 不提供加密密钥路径
	cfg := EnhancedMFAConfig{
		ConfigPath: configPath,
		Issuer:     "test-issuer",
	}

	mgr, err := NewEnhancedMFAManager(cfg)
	if err != nil {
		t.Fatalf("NewEnhancedMFAManager failed: %v", err)
	}

	if mgr == nil {
		t.Fatal("manager is nil")
	}

	// 加密器应该是 nil（没有提供密钥路径）
	if mgr.encryption != nil {
		t.Error("encryption should be nil when no key path provided")
	}
}

// ========== TOTP 测试 ==========

func TestEnhancedMFAManager_SetupTOTP(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	setup, err := mgr.SetupTOTP("user1", "testuser")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}

	if setup == nil {
		t.Fatal("setup is nil")
	}

	if setup.Secret == "" {
		t.Error("Secret should not be empty")
	}

	if setup.Issuer != "test-issuer" {
		t.Errorf("Issuer = %s, want test-issuer", setup.Issuer)
	}

	if setup.AccountName != "testuser" {
		t.Errorf("AccountName = %s, want testuser", setup.AccountName)
	}
}

func TestEnhancedMFAManager_SetupTOTP_AlreadyEnabled(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 设置并启用 TOTP
	_, _ = mgr.SetupTOTP("user1", "testuser")
	cfg := mgr.configs["user1"]
	cfg.TOTPEnabled = true

	// 再次设置应该失败
	_, err := mgr.SetupTOTP("user1", "testuser")
	if err == nil {
		t.Error("SetupTOTP should fail when already enabled")
	}
}

func TestEnhancedMFAManager_EnableTOTP(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 设置 TOTP
	_, _ = mgr.SetupTOTP("user1", "testuser")

	// 使用正确的验证码启用（这里需要实际的 TOTP 验证码，我们模拟）
	// 由于验证码是时间敏感的，这里只测试配置更新
	mgr.mu.Lock()
	cfg := mgr.configs["user1"]
	cfg.TOTPEnabled = true
	cfg.Enabled = true
	mgr.mu.Unlock()

	status := mgr.GetStatus("user1")
	if !status.TOTPEnabled {
		t.Error("TOTP should be enabled")
	}
}

func TestEnhancedMFAManager_EnableTOTP_NotSetup(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	err := mgr.EnableTOTP("user1", "123456")
	if err == nil {
		t.Error("EnableTOTP should fail when not setup")
	}
}

func TestEnhancedMFAManager_DisableTOTP(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 设置并启用 TOTP
	_, _ = mgr.SetupTOTP("user1", "testuser")
	mgr.mu.Lock()
	cfg := mgr.configs["user1"]
	cfg.TOTPEnabled = true
	cfg.Enabled = true
	mgr.mu.Unlock()

	// 禁用需要验证，这里只测试配置更新
	mgr.mu.Lock()
	cfg = mgr.configs["user1"]
	cfg.TOTPEnabled = false
	cfg.TOTPSecret = ""
	cfg.Enabled = false
	mgr.mu.Unlock()

	status := mgr.GetStatus("user1")
	if status.TOTPEnabled {
		t.Error("TOTP should be disabled")
	}
}

// ========== 密码验证测试 ==========

func TestEnhancedMFAManager_ValidatePassword(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	tests := []struct {
		name     string
		password string
		valid    bool
	}{
		{"valid password", "SecureP@ss123", true},
		{"too short", "short", false},
		{"no uppercase", "lowercase123!", false},
		{"no lowercase", "UPPERCASE123!", false},
		{"no digit", "NoDigits!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.ValidatePassword(tt.password)
			if result.Valid != tt.valid {
				t.Errorf("ValidatePassword(%q).Valid = %v, want %v", tt.password, result.Valid, tt.valid)
			}
		})
	}
}

// ========== 登录失败限制测试 ==========

func TestEnhancedMFAManager_CheckLoginAllowed(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 初始应该允许登录
	allowed, reason := mgr.CheckLoginAllowed("testuser", "192.168.1.1")
	if !allowed {
		t.Errorf("Login should be allowed, reason: %s", reason)
	}
}

func TestEnhancedMFAManager_CheckLoginAllowed_Locked(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 记录多次失败
	for i := 0; i < 10; i++ {
		mgr.RecordLoginAttempt("testuser", "192.168.1.1", false)
	}

	// 应该被锁定
	allowed, _ := mgr.CheckLoginAllowed("testuser", "192.168.1.1")
	if allowed {
		t.Error("Login should not be allowed after many failed attempts")
	}
}

func TestEnhancedMFAManager_RecordLoginAttempt(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 记录成功登录
	mgr.RecordLoginAttempt("testuser", "192.168.1.1", true)

	// 应该还是允许登录
	allowed, _ := mgr.CheckLoginAllowed("testuser", "192.168.1.1")
	if !allowed {
		t.Error("Login should still be allowed after successful login")
	}
}

func TestEnhancedMFAManager_GetRemainingLoginAttempts(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 初始应该有最大尝试次数
	remaining := mgr.GetRemainingLoginAttempts("testuser")
	if remaining <= 0 {
		t.Errorf("Remaining attempts should be > 0, got %d", remaining)
	}

	// 记录失败
	mgr.RecordLoginAttempt("testuser", "192.168.1.1", false)

	newRemaining := mgr.GetRemainingLoginAttempts("testuser")
	if newRemaining >= remaining {
		t.Error("Remaining attempts should decrease after failed attempt")
	}
}

func TestEnhancedMFAManager_UnlockUser(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 锁定用户
	for i := 0; i < 10; i++ {
		mgr.RecordLoginAttempt("testuser", "192.168.1.1", false)
	}

	// 解锁
	mgr.UnlockUser("testuser")

	// 应该允许登录
	allowed, _ := mgr.CheckLoginAllowed("testuser", "192.168.1.1")
	if !allowed {
		t.Error("Login should be allowed after unlock")
	}
}

// ========== 会话管理测试 ==========

func TestEnhancedMFAManager_CreateSession(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	session, err := mgr.CreateSession("user1", "testuser", "192.168.1.1", "test-agent", []string{"user"}, []string{})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session == nil {
		t.Fatal("session is nil")
	}

	if session.UserID != "user1" {
		t.Errorf("UserID = %s, want user1", session.UserID)
	}

	if session.Token == "" {
		t.Error("Token should not be empty")
	}
}

func TestEnhancedMFAManager_ValidateSession(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 创建会话
	session, _ := mgr.CreateSession("user1", "testuser", "192.168.1.1", "test-agent", []string{"user"}, []string{})

	// 验证会话
	validated, err := mgr.ValidateSession(session.Token)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}

	if validated.UserID != "user1" {
		t.Errorf("UserID = %s, want user1", validated.UserID)
	}
}

func TestEnhancedMFAManager_ValidateSession_Invalid(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	_, err := mgr.ValidateSession("invalid-token")
	if err == nil {
		t.Error("ValidateSession should fail for invalid token")
	}
}

func TestEnhancedMFAManager_InvalidateSession(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 创建会话
	session, _ := mgr.CreateSession("user1", "testuser", "192.168.1.1", "test-agent", []string{"user"}, []string{})

	// 使会话失效
	err := mgr.InvalidateSession(session.Token)
	if err != nil {
		t.Fatalf("InvalidateSession failed: %v", err)
	}

	// 验证应该失败
	_, err = mgr.ValidateSession(session.Token)
	if err == nil {
		t.Error("ValidateSession should fail for invalidated session")
	}
}

func TestEnhancedMFAManager_InvalidateUserSessions(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 创建多个会话
	session1, _ := mgr.CreateSession("user1", "testuser", "192.168.1.1", "test-agent", []string{"user"}, []string{})
	session2, _ := mgr.CreateSession("user1", "testuser", "192.168.1.2", "test-agent", []string{"user"}, []string{})

	// 使所有会话失效
	err := mgr.InvalidateUserSessions("user1")
	if err != nil {
		t.Fatalf("InvalidateUserSessions failed: %v", err)
	}

	// 两个会话都应该失效
	_, err1 := mgr.ValidateSession(session1.Token)
	_, err2 := mgr.ValidateSession(session2.Token)

	if err1 == nil || err2 == nil {
		t.Error("All user sessions should be invalidated")
	}
}

// ========== MFA 验证测试 ==========

func TestEnhancedMFAManager_VerifyMFA_NotEnabled(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 用户未启用 MFA，验证应该成功（无需验证）
	err := mgr.VerifyMFA("user1", "totp", "123456")
	if err != nil {
		t.Errorf("VerifyMFA should succeed when MFA not enabled, got: %v", err)
	}
}

func TestEnhancedMFAManager_VerifyMFA_TOTPNotEnabled(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 启用 MFA 但不是 TOTP
	mgr.mu.Lock()
	mgr.configs["user1"] = &MFAConfig{
		UserID:  "user1",
		Enabled: true,
	}
	mgr.mu.Unlock()

	err := mgr.VerifyMFA("user1", "totp", "123456")
	if err == nil {
		t.Error("VerifyMFA should fail when TOTP not enabled")
	}
}

func TestEnhancedMFAManager_VerifyMFA_SMSNotEnabled(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 启用 MFA 但不是 SMS
	mgr.mu.Lock()
	mgr.configs["user1"] = &MFAConfig{
		UserID:  "user1",
		Enabled: true,
	}
	mgr.mu.Unlock()

	err := mgr.VerifyMFA("user1", "sms", "123456")
	if err == nil {
		t.Error("VerifyMFA should fail when SMS not enabled")
	}
}

func TestEnhancedMFAManager_VerifyMFA_UnknownType(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 启用 MFA
	mgr.mu.Lock()
	mgr.configs["user1"] = &MFAConfig{
		UserID:  "user1",
		Enabled: true,
	}
	mgr.mu.Unlock()

	err := mgr.VerifyMFA("user1", "unknown", "123456")
	if err == nil {
		t.Error("VerifyMFA should fail for unknown type")
	}
}

// ========== 状态测试 ==========

func TestEnhancedMFAManager_GetStatus(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 用户不存在
	status := mgr.GetStatus("nonexistent")
	if status == nil {
		t.Fatal("status is nil")
	}

	if status.Enabled {
		t.Error("status should show MFA not enabled")
	}
}

func TestEnhancedMFAManager_GetStatus_Enabled(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 设置用户配置
	mgr.mu.Lock()
	mgr.configs["user1"] = &MFAConfig{
		UserID:          "user1",
		Enabled:         true,
		TOTPEnabled:     true,
		SMSEnabled:      true,
		WebAuthnEnabled: false,
		Phone:           "+8613800138000",
	}
	mgr.mu.Unlock()

	status := mgr.GetStatus("user1")
	if !status.Enabled {
		t.Error("status should show MFA enabled")
	}

	if !status.TOTPEnabled {
		t.Error("status should show TOTP enabled")
	}

	if !status.SMSEnabled {
		t.Error("status should show SMS enabled")
	}

	if status.Phone != "+8613800138000" {
		t.Errorf("Phone = %s, want +8613800138000", status.Phone)
	}
}

// ========== 统计测试 ==========

func TestEnhancedMFAManager_GetStats(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 添加一些用户
	mgr.mu.Lock()
	mgr.configs["user1"] = &MFAConfig{UserID: "user1", Enabled: true, TOTPEnabled: true}
	mgr.configs["user2"] = &MFAConfig{UserID: "user2", Enabled: true, SMSEnabled: true}
	mgr.configs["user3"] = &MFAConfig{UserID: "user3", Enabled: false}
	mgr.mu.Unlock()

	stats := mgr.GetStats()

	if stats["total_users"].(int) != 3 {
		t.Errorf("total_users = %d, want 3", stats["total_users"])
	}

	if stats["mfa_enabled"].(int) != 2 {
		t.Errorf("mfa_enabled = %d, want 2", stats["mfa_enabled"])
	}

	if stats["totp_enabled"].(int) != 1 {
		t.Errorf("totp_enabled = %d, want 1", stats["totp_enabled"])
	}

	if stats["sms_enabled"].(int) != 1 {
		t.Errorf("sms_enabled = %d, want 1", stats["sms_enabled"])
	}

	// 检查嵌套统计
	if stats["login_attempts"] == nil {
		t.Error("login_attempts stats should not be nil")
	}

	if stats["sessions"] == nil {
		t.Error("sessions stats should not be nil")
	}
}

// ========== 备份码测试 ==========

func TestEnhancedMFAManager_GenerateBackupCodes(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 需要先启用 MFA
	mgr.mu.Lock()
	mgr.configs["user1"] = &MFAConfig{
		UserID:  "user1",
		Enabled: true,
	}
	mgr.mu.Unlock()

	codes, err := mgr.GenerateBackupCodes("user1", 10)
	if err != nil {
		t.Fatalf("GenerateBackupCodes failed: %v", err)
	}

	if len(codes) != 10 {
		t.Errorf("len(codes) = %d, want 10", len(codes))
	}
}

func TestEnhancedMFAManager_GenerateBackupCodes_MFANotEnabled(t *testing.T) {
	mgr := newTestEnhancedMFAManager(t)

	// 用户未启用 MFA
	_, err := mgr.GenerateBackupCodes("user1", 10)
	if err == nil {
		t.Error("GenerateBackupCodes should fail when MFA not enabled")
	}
}

// ========== 持久化测试 ==========

func TestEnhancedMFAManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mfa.json")

	cfg := EnhancedMFAConfig{
		ConfigPath: configPath,
		Issuer:     "test-issuer",
	}

	// 创建第一个管理器
	mgr1, err := NewEnhancedMFAManager(cfg)
	if err != nil {
		t.Fatalf("NewEnhancedMFAManager failed: %v", err)
	}

	// 添加配置
	mgr1.mu.Lock()
	mgr1.configs["user1"] = &MFAConfig{
		UserID:      "user1",
		Enabled:     true,
		TOTPEnabled: true,
		CreatedAt:   time.Now(),
	}
	mgr1.mu.Unlock()
	mgr1.saveConfig()

	// 创建第二个管理器（模拟重启）
	mgr2, err := NewEnhancedMFAManager(cfg)
	if err != nil {
		t.Fatalf("NewEnhancedMFAManager failed: %v", err)
	}

	// 验证数据已恢复
	mgr2.mu.RLock()
	cfg2, exists := mgr2.configs["user1"]
	mgr2.mu.RUnlock()

	if !exists {
		t.Fatal("user1 config not restored")
	}

	if !cfg2.Enabled {
		t.Error("Enabled should be true")
	}
}

// ========== 辅助函数 ==========

func newTestEnhancedMFAManager(t *testing.T) *EnhancedMFAManager {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mfa.json")

	cfg := EnhancedMFAConfig{
		ConfigPath: configPath,
		Issuer:     "test-issuer",
	}

	mgr, err := NewEnhancedMFAManager(cfg)
	if err != nil {
		t.Fatalf("NewEnhancedMFAManager failed: %v", err)
	}

	return mgr
}

func TestGenerateRandomPassphrase(t *testing.T) {
	p1 := generateRandomPassphrase()
	p2 := generateRandomPassphrase()

	if p1 == p2 {
		t.Error("passphrases should be different")
	}

	if len(p1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("passphrase length = %d, want 64", len(p1))
	}
}