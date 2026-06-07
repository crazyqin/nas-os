// Package twofactor 实现双因素认证模块，对标 TrueNAS 2FA
package twofactor

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"hash"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TwoFactor 双因素认证
type TwoFactor struct {
	mu          sync.RWMutex
	users       map[string]*User2FA
	config      *Config
	backupCodes map[string][]string
}

// Config 双因素认证配置
type Config struct {
	Issuer        string `json:"issuer"`
	Period        uint   `json:"period"`
	Digits        int    `json:"digits"`
	Algorithm     string `json:"algorithm"`
	BackupCodeLen int    `json:"backup_code_len"`
	BackupCodeNum int    `json:"backup_code_num"`
	Skew          int    `json:"skew"`
}

// User2FA 用户 2FA 信息
type User2FA struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	Secret      string    `json:"secret"`
	Enabled     bool      `json:"enabled"`
	Verified    bool      `json:"verified"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used"`
	BackupCodes []string  `json:"backup_codes"`
}

// TOTPConfig TOTP 配置
type TOTPConfig struct {
	Secret    string
	Period    uint
	Digits    int
	Algorithm string
}

// HOTPConfig HOTP 配置
type HOTPConfig struct {
	Secret    string
	Counter   uint64
	Digits    int
	Algorithm string
}

// NewTwoFactor 创建双因素认证
func NewTwoFactor(config *Config) *TwoFactor {
	return &TwoFactor{
		users:       make(map[string]*User2FA),
		config:      config,
		backupCodes: make(map[string][]string),
	}
}

// EnableUser 启用用户 2FA
func (tf *TwoFactor) EnableUser(userID, username string) (*User2FA, error) {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	if _, exists := tf.users[userID]; exists {
		return nil, fmt.Errorf("2FA already enabled for user %s", userID)
	}

	// 生成密钥
	secret, err := tf.generateSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}

	// 生成备份码
	backupCodes, err := tf.generateBackupCodes()
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	user := &User2FA{
		UserID:      userID,
		Username:    username,
		Secret:      secret,
		Enabled:     true,
		Verified:    false,
		CreatedAt:   time.Now(),
		BackupCodes: backupCodes,
	}

	tf.users[userID] = user
	tf.backupCodes[userID] = backupCodes

	return user, nil
}

// DisableUser 禁用用户 2FA
func (tf *TwoFactor) DisableUser(userID string) error {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	if _, exists := tf.users[userID]; !exists {
		return fmt.Errorf("2FA not enabled for user %s", userID)
	}

	delete(tf.users, userID)
	delete(tf.backupCodes, userID)
	return nil
}

// VerifyTOTP 验证 TOTP 代码
func (tf *TwoFactor) VerifyTOTP(userID, code string) (bool, error) {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	user, exists := tf.users[userID]
	if !exists {
		return false, fmt.Errorf("2FA not enabled for user %s", userID)
	}

	if !user.Enabled {
		return false, fmt.Errorf("2FA disabled for user %s", userID)
	}

	config := &TOTPConfig{
		Secret:    user.Secret,
		Period:    tf.config.Period,
		Digits:    tf.config.Digits,
		Algorithm: tf.config.Algorithm,
	}

	// 验证当前时间窗口
	valid := tf.verifyTOTPCode(config, code, tf.config.Skew)
	if valid {
		user.LastUsed = time.Now()
		if !user.Verified {
			user.Verified = true
		}
	}

	return valid, nil
}

// VerifyHOTP 验证 HOTP 代码
func (tf *TwoFactor) VerifyHOTP(userID, code string, counter uint64) (bool, error) {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	user, exists := tf.users[userID]
	if !exists {
		return false, fmt.Errorf("2FA not enabled for user %s", userID)
	}

	config := &HOTPConfig{
		Secret:    user.Secret,
		Counter:   counter,
		Digits:    tf.config.Digits,
		Algorithm: tf.config.Algorithm,
	}

	valid := tf.verifyHOTPCode(config, code)
	if valid {
		user.LastUsed = time.Now()
	}

	return valid, nil
}

// VerifyBackupCode 验证备份码
func (tf *TwoFactor) VerifyBackupCode(userID, code string) (bool, error) {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	user, exists := tf.users[userID]
	if !exists {
		return false, fmt.Errorf("2FA not enabled for user %s", userID)
	}

	// 查找并删除已使用的备份码
	for i, backupCode := range user.BackupCodes {
		if backupCode == code {
			// 删除已使用的备份码
			user.BackupCodes = append(user.BackupCodes[:i], user.BackupCodes[i+1:]...)
			user.LastUsed = time.Now()
			return true, nil
		}
	}

	return false, nil
}

// RegenerateBackupCodes 重新生成备份码
func (tf *TwoFactor) RegenerateBackupCodes(userID string) ([]string, error) {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	user, exists := tf.users[userID]
	if !exists {
		return nil, fmt.Errorf("2FA not enabled for user %s", userID)
	}

	backupCodes, err := tf.generateBackupCodes()
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	user.BackupCodes = backupCodes
	tf.backupCodes[userID] = backupCodes

	return backupCodes, nil
}

// GetQRCodeURL 获取二维码 URL
func (tf *TwoFactor) GetQRCodeURL(userID string) (string, error) {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	user, exists := tf.users[userID]
	if !exists {
		return "", fmt.Errorf("2FA not enabled for user %s", userID)
	}

	otpURL := url.URL{
		Scheme: "otpauth",
		Host:   "totp",
		Path:   fmt.Sprintf("%s:%s", tf.config.Issuer, user.Username),
	}

	q := otpURL.Query()
	q.Set("secret", user.Secret)
	q.Set("issuer", tf.config.Issuer)
	q.Set("period", fmt.Sprintf("%d", tf.config.Period))
	q.Set("digits", fmt.Sprintf("%d", tf.config.Digits))
	q.Set("algorithm", tf.config.Algorithm)
	otpURL.RawQuery = q.Encode()

	return otpURL.String(), nil
}

// GetUserStatus 获取用户 2FA 状态
func (tf *TwoFactor) GetUserStatus(userID string) (*User2FA, error) {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	user, exists := tf.users[userID]
	if !exists {
		return nil, fmt.Errorf("2FA not enabled for user %s", userID)
	}

	return user, nil
}

// GetStats 获取统计信息
func (tf *TwoFactor) GetStats() map[string]interface{} {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	enabledCount := 0
	verifiedCount := 0

	for _, user := range tf.users {
		if user.Enabled {
			enabledCount++
		}
		if user.Verified {
			verifiedCount++
		}
	}

	return map[string]interface{}{
		"total_users":    len(tf.users),
		"enabled_users":  enabledCount,
		"verified_users": verifiedCount,
		"issuer":         tf.config.Issuer,
		"period":         tf.config.Period,
		"digits":         tf.config.Digits,
		"algorithm":      tf.config.Algorithm,
	}
}

// generateSecret 生成密钥
func (tf *TwoFactor) generateSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}

	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// generateBackupCodes 生成备份码
func (tf *TwoFactor) generateBackupCodes() ([]string, error) {
	codes := make([]string, tf.config.BackupCodeNum)

	for i := 0; i < tf.config.BackupCodeNum; i++ {
		code, err := tf.generateRandomCode(tf.config.BackupCodeLen)
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}

	return codes, nil
}

// generateRandomCode 生成随机代码
func (tf *TwoFactor) generateRandomCode(length int) (string, error) {
	const charset = "0123456789"
	code := make([]byte, length)

	for i := 0; i < length; i++ {
		randomByte := make([]byte, 1)
		_, err := rand.Read(randomByte)
		if err != nil {
			return "", err
		}
		code[i] = charset[randomByte[0]%byte(len(charset))]
	}

	return string(code), nil
}

// verifyTOTPCode 验证 TOTP 代码
func (tf *TwoFactor) verifyTOTPCode(config *TOTPConfig, code string, skew int) bool {
	currentTime := time.Now().Unix()
	period := int64(config.Period)

	for i := -skew; i <= skew; i++ {
		counter := uint64((currentTime + int64(i)*period) / period)
		expectedCode := tf.generateTOTPCode(config.Secret, counter, config.Digits, config.Algorithm)
		if hmac.Equal([]byte(code), []byte(expectedCode)) {
			return true
		}
	}

	return false
}

// verifyHOTPCode 验证 HOTP 代码
func (tf *TwoFactor) verifyHOTPCode(config *HOTPConfig, code string) bool {
	expectedCode := tf.generateHOTPCode(config.Secret, config.Counter, config.Digits, config.Algorithm)
	return hmac.Equal([]byte(code), []byte(expectedCode))
}

// generateTOTPCode 生成 TOTP 代码
func (tf *TwoFactor) generateTOTPCode(secret string, counter uint64, digits int, algorithm string) string {
	return tf.generateHOTPCode(secret, counter, digits, algorithm)
}

// generateHOTPCode 生成 HOTP 代码
func (tf *TwoFactor) generateHOTPCode(secret string, counter uint64, digits int, algorithm string) string {
	// 解码密钥
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return ""
	}

	// 转换计数器为字节
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	// 选择哈希算法
	var hashFunc func() hash.Hash
	switch strings.ToUpper(algorithm) {
	case "SHA256":
		hashFunc = sha256.New
	case "SHA512":
		hashFunc = sha512.New
	default:
		hashFunc = sha1.New
	}

	// 计算 HMAC
	mac := hmac.New(hashFunc, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	// 动态截断
	offset := sum[len(sum)-1] & 0x0F
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7FFFFFFF

	// 生成代码
	mod := uint32(math.Pow10(digits))
	otp := code % mod

	return fmt.Sprintf("%0*d", digits, otp)
}

// GenerateRecoveryCodes 生成恢复码
func (tf *TwoFactor) GenerateRecoveryCodes(userID string, count int) ([]string, error) {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	user, exists := tf.users[userID]
	if !exists {
		return nil, fmt.Errorf("2FA not enabled for user %s", userID)
	}

	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, err := tf.generateRandomCode(tf.config.BackupCodeLen)
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}

	// 添加到用户备份码
	user.BackupCodes = append(user.BackupCodes, codes...)
	tf.backupCodes[userID] = user.BackupCodes

	return codes, nil
}

// IsEnabled 检查用户是否启用 2FA
func (tf *TwoFactor) IsEnabled(userID string) bool {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	user, exists := tf.users[userID]
	return exists && user.Enabled
}

// IsVerified 检查用户是否已验证 2FA
func (tf *TwoFactor) IsVerified(userID string) bool {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	user, exists := tf.users[userID]
	return exists && user.Verified
}
