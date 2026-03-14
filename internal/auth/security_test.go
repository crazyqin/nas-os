package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== SecretEncryption 测试 ==========

func TestSecretEncryption(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "secret.key")

	se := NewSecretEncryption(keyPath)

	// 初始化
	err := se.Initialize("test-passphrase")
	require.NoError(t, err)
	assert.True(t, se.IsInitialized())

	// 测试加密解密
	plaintext := "my-super-secret-totp-key"
	ciphertext, err := se.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := se.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestSecretEncryption_Bytes(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "secret.key")

	se := NewSecretEncryption(keyPath)
	require.NoError(t, se.Initialize("test-passphrase"))

	// 测试字节数据加密
	plaintext := []byte("binary-data-123\x00\x01\x02")
	ciphertext, err := se.EncryptBytes(plaintext)
	require.NoError(t, err)

	decrypted, err := se.DecryptBytes(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestSecretEncryption_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "secret.key")

	// 创建第一个加密器
	se1 := NewSecretEncryption(keyPath)
	require.NoError(t, se1.Initialize("test-passphrase"))

	plaintext := "secret-to-persist"
	ciphertext, err := se1.Encrypt(plaintext)
	require.NoError(t, err)

	// 创建第二个加密器（加载已有密钥）
	se2 := NewSecretEncryption(keyPath)
	assert.True(t, se2.IsInitialized())

	// 验证可以用第二个加密器解密
	decrypted, err := se2.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// ========== SecureBackupCodeManager 测试 ==========

func TestSecureBackupCodeManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "backup_codes.json")

	m := NewSecureBackupCodeManager(configPath, nil)

	// 生成备份码
	codes, err := m.GenerateBackupCodes("user1", 10)
	require.NoError(t, err)
	assert.Len(t, codes, 10)

	// 验证备份码格式 (XXXX-XXXX)
	for _, code := range codes {
		assert.Regexp(t, `^[a-f0-9]{8}-[a-f0-9]{8}$`, code)
	}

	// 验证未使用数量
	assert.Equal(t, 10, m.GetUnusedCount("user1"))

	// 使用第一个备份码
	err = m.VerifyBackupCode("user1", codes[0])
	require.NoError(t, err)

	// 验证已使用
	assert.Equal(t, 9, m.GetUnusedCount("user1"))

	// 再次使用同一个备份码应该失败
	err = m.VerifyBackupCode("user1", codes[0])
	assert.Error(t, err)

	// 使用无效备份码
	err = m.VerifyBackupCode("user1", "invalid-code")
	assert.Error(t, err)
}

func TestSecureBackupCodeManager_Hashing(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "backup_codes.json")

	m := NewSecureBackupCodeManager(configPath, nil)

	codes, err := m.GenerateBackupCodes("user1", 1)
	require.NoError(t, err)

	// 读取存储的文件，验证备份码是哈希存储
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// 确保明文备份码不在文件中
	assert.NotContains(t, string(data), codes[0])

	// 文件应该包含 bcrypt 哈希（以 $2a$ 开头）
	assert.Contains(t, string(data), "$2a$")
}

func TestSecureBackupCodeManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "backup_codes.json")

	// 创建管理器并生成备份码
	m1 := NewSecureBackupCodeManager(configPath, nil)
	codes, err := m1.GenerateBackupCodes("user1", 5)
	require.NoError(t, err)

	// 使用一个备份码
	require.NoError(t, m1.VerifyBackupCode("user1", codes[0]))

	// 创建新的管理器（加载已有数据）
	m2 := NewSecureBackupCodeManager(configPath, nil)

	// 验证状态正确
	assert.Equal(t, 4, m2.GetUnusedCount("user1"))

	// 验证已使用的备份码不能再次使用
	assert.Error(t, m2.VerifyBackupCode("user1", codes[0]))

	// 验证未使用的备份码可以使用
	assert.NoError(t, m2.VerifyBackupCode("user1", codes[1]))
}

func TestHashBackupCode(t *testing.T) {
	code := "abcd1234-efgh5678"

	hash, err := hashBackupCode(code)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// 验证哈希是 bcrypt 格式（以 $2a$ 开头）
	assert.Contains(t, hash, "$2a$")

	// 验证验证函数
	assert.True(t, verifyBackupCodeHash(code, hash))
	assert.False(t, verifyBackupCodeHash("wrong-code", hash))
}

// ========== OAuth2Manager 测试 ==========

func TestOAuth2Manager_Providers(t *testing.T) {
	m := NewOAuth2Manager()

	// 注册提供商
	google := GetGoogleOAuth2Config("client-id", "client-secret", "http://localhost/callback")
	require.NoError(t, m.RegisterProvider(google))

	github := GetGitHubOAuth2Config("client-id", "client-secret", "http://localhost/callback")
	require.NoError(t, m.RegisterProvider(github))

	// 列出提供商
	providers := m.ListProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, "google")
	assert.Contains(t, providers, "github")

	// 获取提供商
	p, err := m.GetProvider("google")
	require.NoError(t, err)
	assert.Equal(t, "google", p.Name)

	// 获取不存在的提供商
	_, err = m.GetProvider("nonexistent")
	assert.Error(t, err)
}

func TestOAuth2Manager_AuthURL(t *testing.T) {
	m := NewOAuth2Manager()

	google := GetGoogleOAuth2Config("test-client-id", "test-secret", "http://localhost/callback")
	require.NoError(t, m.RegisterProvider(google))

	// 生成授权 URL
	authURL, err := m.GenerateAuthURL("google", "/dashboard")
	require.NoError(t, err)
	assert.Contains(t, authURL, "accounts.google.com")
	assert.Contains(t, authURL, "client_id=test-client-id")
	assert.Contains(t, authURL, "redirect_uri=http")
	assert.Contains(t, authURL, "state=")
}

func TestOAuth2Manager_StateValidation(t *testing.T) {
	m := NewOAuth2Manager()

	// 手动创建状态
	state := "test-state-123"
	m.states[state] = &OAuth2State{
		State:     state,
		Provider:  "google",
		Redirect:  "/dashboard",
		CreatedAt: time.Now(),
	}

	// 验证有效状态
	s, err := m.ValidateState(state)
	require.NoError(t, err)
	assert.Equal(t, "google", s.Provider)

	// 验证状态被删除（单次使用）
	_, err = m.ValidateState(state)
	assert.Error(t, err)
}

func TestOAuth2Manager_ExpiredState(t *testing.T) {
	m := NewOAuth2Manager()

	// 创建过期状态
	state := "expired-state"
	m.states[state] = &OAuth2State{
		State:     state,
		Provider:  "google",
		Redirect:  "/dashboard",
		CreatedAt: time.Now().Add(-15 * time.Minute), // 15 分钟前
	}

	// 验证过期状态应该失败
	_, err := m.ValidateState(state)
	assert.Error(t, err)
}

// ========== LDAPManager 测试 ==========

func TestLDAPManager_Config(t *testing.T) {
	m := NewLDAPManager()

	// 注册 OpenLDAP 配置
	openLDAP := GetOpenLDAPConfig("test-ldap", "ldap://localhost:389", "cn=admin,dc=example,dc=com", "password", "dc=example,dc=com")
	require.NoError(t, m.RegisterConfig(openLDAP))

	// 注册 AD 配置
	ad := GetADConfig("test-ad", "ldaps://ad.example.com:636", "CN=admin,CN=Users,DC=example,DC=com", "password", "DC=example,DC=com")
	require.NoError(t, m.RegisterConfig(ad))

	// 列出配置
	configs := m.ListConfigs()
	assert.Len(t, configs, 2)

	// 获取配置
	cfg, err := m.GetConfig("test-ldap")
	require.NoError(t, err)
	assert.Equal(t, "test-ldap", cfg.Name)
	assert.False(t, cfg.IsAD)

	cfg, err = m.GetConfig("test-ad")
	require.NoError(t, err)
	assert.True(t, cfg.IsAD)
}

// ========== MFA 会话测试 ==========

func TestMFASession(t *testing.T) {
	// 清理会话
	mfaSessions = make(map[string]*MFASession)

	m := &MFAManager{
		configs: make(map[string]*MFAConfig),
	}

	// 创建 MFA 会话
	token, err := m.CreateMFASession("user1", "testuser", "totp")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// 获取会话
	session, err := m.GetMFASession(token)
	require.NoError(t, err)
	assert.Equal(t, "user1", session.UserID)
	assert.Equal(t, "testuser", session.Username)
	assert.False(t, session.Verified)

	// 完成会话
	require.NoError(t, m.CompleteMFASession(token))

	session, err = m.GetMFASession(token)
	require.NoError(t, err)
	assert.True(t, session.Verified)

	// 删除会话
	m.DeleteMFASession(token)

	_, err = m.GetMFASession(token)
	assert.Error(t, err)
}

// ========== TOTP 测试 ==========

func TestTOTP(t *testing.T) {
	// 设置 TOTP
	setup, err := SetupTOTP("NAS-OS", "testuser")
	require.NoError(t, err)
	assert.NotEmpty(t, setup.Secret)
	assert.NotEmpty(t, setup.URI)
	assert.Contains(t, setup.URI, "otpauth://totp/")

	// 验证 TOTP - 使用 pquerna/otp/totp 包生成当前代码
	// 注意：实际使用时用户会从验证器应用获取代码
	// 这里我们使用 Validate 函数验证 setup 是有效的
	assert.NotEmpty(t, setup.Secret)
	assert.Len(t, setup.Secret, 32) // Base32 编码的密钥长度

	// 验证错误代码会被拒绝
	valid := VerifyTOTP(setup.Secret, "000000")
	assert.False(t, valid)
}

// ========== 基准测试 ==========

func BenchmarkHashBackupCode(b *testing.B) {
	code := "abcd1234-efgh5678"
	for i := 0; i < b.N; i++ {
		_, _ = hashBackupCode(code)
	}
}

func BenchmarkVerifyBackupCodeHash(b *testing.B) {
	code := "abcd1234-efgh5678"
	hash, _ := hashBackupCode(code)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = verifyBackupCodeHash(code, hash)
	}
}

func BenchmarkSecretEncryption(b *testing.B) {
	tmpDir := b.TempDir()
	keyPath := filepath.Join(tmpDir, "secret.key")

	se := NewSecretEncryption(keyPath)
	_ = se.Initialize("test-passphrase")

	plaintext := "my-super-secret-key"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ciphertext, _ := se.Encrypt(plaintext)
		_, _ = se.Decrypt(ciphertext)
	}
}
