package twofactor

import (
	"testing"
)

func TestNewTwoFactor(t *testing.T) {
	config := &Config{
		Issuer:        "NAS-OS",
		Period:        30,
		Digits:        6,
		Algorithm:     "SHA1",
		BackupCodeLen: 8,
		BackupCodeNum: 10,
		Skew:          1,
	}

	tf := NewTwoFactor(config)
	if tf == nil {
		t.Fatal("NewTwoFactor returned nil")
	}

	if tf.config != config {
		t.Error("config not set correctly")
	}
}

func TestEnableUser(t *testing.T) {
	config := &Config{
		Issuer:        "NAS-OS",
		Period:        30,
		Digits:        6,
		Algorithm:     "SHA1",
		BackupCodeLen: 8,
		BackupCodeNum: 10,
		Skew:          1,
	}

	tf := NewTwoFactor(config)

	// 启用用户 2FA
	user, err := tf.EnableUser("user1", "testuser")
	if err != nil {
		t.Fatalf("EnableUser failed: %v", err)
	}

	if user.UserID != "user1" {
		t.Errorf("expected user ID 'user1', got '%s'", user.UserID)
	}

	if !user.Enabled {
		t.Error("expected user to be enabled")
	}

	if user.Verified {
		t.Error("expected user to not be verified yet")
	}

	if len(user.BackupCodes) != 10 {
		t.Errorf("expected 10 backup codes, got %d", len(user.BackupCodes))
	}
}

func TestEnableDuplicateUser(t *testing.T) {
	config := &Config{
		Issuer:        "NAS-OS",
		Period:        30,
		Digits:        6,
		Algorithm:     "SHA1",
		BackupCodeLen: 8,
		BackupCodeNum: 10,
		Skew:          1,
	}

	tf := NewTwoFactor(config)

	// 启用用户 2FA
	tf.EnableUser("user1", "testuser")

	// 尝试再次启用
	_, err := tf.EnableUser("user1", "testuser")
	if err == nil {
		t.Error("expected error when enabling duplicate user")
	}
}

func TestDisableUser(t *testing.T) {
	config := &Config{
		Issuer:        "NAS-OS",
		Period:        30,
		Digits:        6,
		Algorithm:     "SHA1",
		BackupCodeLen: 8,
		BackupCodeNum: 10,
		Skew:          1,
	}

	tf := NewTwoFactor(config)

	// 启用用户 2FA
	tf.EnableUser("user1", "testuser")

	// 禁用用户 2FA
	err := tf.DisableUser("user1")
	if err != nil {
		t.Fatalf("DisableUser failed: %v", err)
	}

	// 验证用户已禁用
	if tf.IsEnabled("user1") {
		t.Error("expected user to be disabled")
	}
}

func TestIsEnabled(t *testing.T) {
	config := &Config{
		Issuer:        "NAS-OS",
		Period:        30,
		Digits:        6,
		Algorithm:     "SHA1",
		BackupCodeLen: 8,
		BackupCodeNum: 10,
		Skew:          1,
	}

	tf := NewTwoFactor(config)

	// 未启用的用户
	if tf.IsEnabled("user1") {
		t.Error("expected user to not be enabled")
	}

	// 启用用户
	tf.EnableUser("user1", "testuser")

	if !tf.IsEnabled("user1") {
		t.Error("expected user to be enabled")
	}
}

func TestIsVerified(t *testing.T) {
	config := &Config{
		Issuer:        "NAS-OS",
		Period:        30,
		Digits:        6,
		Algorithm:     "SHA1",
		BackupCodeLen: 8,
		BackupCodeNum: 10,
		Skew:          1,
	}

	tf := NewTwoFactor(config)

	// 启用用户
	tf.EnableUser("user1", "testuser")

	// 未验证
	if tf.IsVerified("user1") {
		t.Error("expected user to not be verified")
	}
}

func TestGetQRCodeURL(t *testing.T) {
	config := &Config{
		Issuer:        "NAS-OS",
		Period:        30,
		Digits:        6,
		Algorithm:     "SHA1",
		BackupCodeLen: 8,
		BackupCodeNum: 10,
		Skew:          1,
	}

	tf := NewTwoFactor(config)

	// 启用用户
	tf.EnableUser("user1", "testuser")

	// 获取二维码 URL
	url, err := tf.GetQRCodeURL("user1")
	if err != nil {
		t.Fatalf("GetQRCodeURL failed: %v", err)
	}

	if url == "" {
		t.Error("expected non-empty QR code URL")
	}

	// 验证 URL 格式
	if !contains(url, "otpauth://totp/NAS-OS:testuser") {
		t.Errorf("unexpected QR code URL format: %s", url)
	}
}

func TestRegenerateBackupCodes(t *testing.T) {
	config := &Config{
		Issuer:        "NAS-OS",
		Period:        30,
		Digits:        6,
		Algorithm:     "SHA1",
		BackupCodeLen: 8,
		BackupCodeNum: 10,
		Skew:          1,
	}

	tf := NewTwoFactor(config)

	// 启用用户
	user, _ := tf.EnableUser("user1", "testuser")
	oldCodes := user.BackupCodes

	// 重新生成备份码
	newCodes, err := tf.RegenerateBackupCodes("user1")
	if err != nil {
		t.Fatalf("RegenerateBackupCodes failed: %v", err)
	}

	if len(newCodes) != 10 {
		t.Errorf("expected 10 backup codes, got %d", len(newCodes))
	}

	// 验证新旧代码不同
	same := true
	for i, code := range newCodes {
		if code != oldCodes[i] {
			same = false
			break
		}
	}

	if same {
		t.Error("expected new backup codes to be different from old ones")
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{
		Issuer:        "NAS-OS",
		Period:        30,
		Digits:        6,
		Algorithm:     "SHA1",
		BackupCodeLen: 8,
		BackupCodeNum: 10,
		Skew:          1,
	}

	tf := NewTwoFactor(config)

	// 添加数据
	tf.EnableUser("user1", "testuser1")
	tf.EnableUser("user2", "testuser2")

	stats := tf.GetStats()

	if stats["total_users"] != 2 {
		t.Errorf("expected 2 total users, got %v", stats["total_users"])
	}

	if stats["enabled_users"] != 2 {
		t.Errorf("expected 2 enabled users, got %v", stats["enabled_users"])
	}

	if stats["issuer"] != "NAS-OS" {
		t.Errorf("expected issuer 'NAS-OS', got %v", stats["issuer"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}
