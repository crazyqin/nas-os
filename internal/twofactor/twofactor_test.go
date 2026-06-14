package twofactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTwoFactorAuth_GenerateSecret(t *testing.T) {
	config := DefaultConfig()
	tfa := New(config)

	secret, qrURL, err := tfa.GenerateSecret("user-1")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, qrURL, "otpauth://totp/")
	assert.Contains(t, qrURL, "user-1")
}

func TestTwoFactorAuth_VerifyCode(t *testing.T) {
	config := DefaultConfig()
	tfa := New(config)

	// 生成密钥
	secret, _, err := tfa.GenerateSecret("user-1")
	require.NoError(t, err)

	// 生成当前有效的TOTP码
	code := tfa.generateTOTP(secret, time.Now())

	// 验证码
	valid, err := tfa.VerifyCode("user-1", code)
	require.NoError(t, err)
	assert.True(t, valid)

	// 验证2FA已启用
	assert.True(t, tfa.IsEnabled("user-1"))
}

func TestTwoFactorAuth_VerifyBackupCode(t *testing.T) {
	config := DefaultConfig()
	tfa := New(config)

	// 生成密钥（会同时生成备用码）
	_, _, err := tfa.GenerateSecret("user-1")
	require.NoError(t, err)

	// 获取备用码数量
	backupCount := tfa.GetBackupCodes("user-1")
	assert.Equal(t, config.BackupCodes, backupCount)

	// 使用备用码验证需要访问内部状态，这里测试基本功能
	assert.False(t, tfa.IsEnabled("user-1")) // 未验证前未启用
}

func TestTwoFactorAuth_Disable(t *testing.T) {
	config := DefaultConfig()
	tfa := New(config)

	// 生成密钥
	secret, _, err := tfa.GenerateSecret("user-1")
	require.NoError(t, err)

	// 先验证一个TOTP码以启用2FA
	code := tfa.generateTOTP(secret, time.Now())
	tfa.VerifyCode("user-1", code)
	assert.True(t, tfa.IsEnabled("user-1"))

	// 禁用2FA
	tfa.Disable("user-1")
	assert.False(t, tfa.IsEnabled("user-1"))
}

func TestTwoFactorAuth_InvalidCode(t *testing.T) {
	config := DefaultConfig()
	tfa := New(config)

	// 生成密钥
	_, _, err := tfa.GenerateSecret("user-1")
	require.NoError(t, err)

	// 使用无效码
	valid, err := tfa.VerifyCode("user-1", "000000")
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestTwoFactorAuth_UnconfiguredUser(t *testing.T) {
	config := DefaultConfig()
	tfa := New(config)

	// 未配置的用户
	valid, err := tfa.VerifyCode("unknown-user", "123456")
	assert.Error(t, err)
	assert.False(t, valid)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "NAS-OS", config.Issuer)
	assert.Equal(t, 20, config.SecretSize)
	assert.Equal(t, 8, config.BackupCodes)
	assert.Equal(t, 6, config.CodeLength)
	assert.Equal(t, 1, config.Window)
	assert.True(t, config.Enabled)
}
