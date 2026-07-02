// Package twofactor 双因素认证模块
// 对标飞牛fnOS的2FA功能，支持TOTP和备用码
package twofactor

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// Config 2FA配置.
type Config struct {
	Issuer      string `json:"issuer"`       // 发行者名称
	SecretSize  int    `json:"secret_size"`  // 密钥长度
	BackupCodes int    `json:"backup_codes"` // 备用码数量
	CodeLength  int    `json:"code_length"`  // 验证码长度
	Window      int    `json:"window"`       // 时间窗口
	Enabled     bool   `json:"enabled"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() Config {
	return Config{
		Issuer:      "NAS-OS",
		SecretSize:  20,
		BackupCodes: 8,
		CodeLength:  6,
		Window:      1,
		Enabled:     true,
	}
}

// TwoFactorAuth 双因素认证管理器.
type TwoFactorAuth struct {
	mu     sync.RWMutex
	config Config
	users  map[string]*UserTwoFactor
}

// UserTwoFactor 用户2FA信息.
type UserTwoFactor struct {
	UserID      string    `json:"user_id"`
	Secret      string    `json:"secret"`
	BackupCodes []string  `json:"backup_codes"`
	Enabled     bool      `json:"enabled"`
	Verified    bool      `json:"verified"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used,omitempty"`
}

// New 创建2FA管理器.
func New(config Config) *TwoFactorAuth {
	return &TwoFactorAuth{
		config: config,
		users:  make(map[string]*UserTwoFactor),
	}
}

// GenerateSecret 为用户生成TOTP密钥.
func (tfa *TwoFactorAuth) GenerateSecret(userID string) (secret string, qrURL string, err error) {
	tfa.mu.Lock()
	defer tfa.mu.Unlock()

	// 生成随机密钥
	randomBytes := make([]byte, tfa.config.SecretSize)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("生成随机数失败: %w", err)
	}

	secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	// 生成备用码
	backupCodes := tfa.generateBackupCodes()

	// 存储用户2FA信息
	tfa.users[userID] = &UserTwoFactor{
		UserID:      userID,
		Secret:      secret,
		BackupCodes: backupCodes,
		Enabled:     false,
		Verified:    false,
		CreatedAt:   time.Now(),
	}

	// 生成TOTP URL (用于二维码)
	qrURL = fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&digits=%d",
		tfa.config.Issuer, userID, secret, tfa.config.Issuer, tfa.config.CodeLength)

	return secret, qrURL, nil
}

// VerifyCode 验证TOTP码.
func (tfa *TwoFactorAuth) VerifyCode(userID, code string) (bool, error) {
	tfa.mu.RLock()
	user, ok := tfa.users[userID]
	tfa.mu.RUnlock()

	if !ok {
		return false, fmt.Errorf("用户 %s 未配置2FA", userID)
	}

	// 检查是否是备用码
	if tfa.verifyBackupCode(userID, code) {
		return true, nil
	}

	// 验证TOTP码
	now := time.Now()
	for i := -tfa.config.Window; i <= tfa.config.Window; i++ {
		expectedCode := tfa.generateTOTP(user.Secret, now.Add(time.Duration(i)*30*time.Second))
		if hmac.Equal([]byte(code), []byte(expectedCode)) {
			// 更新最后使用时间
			tfa.mu.Lock()
			user.LastUsed = time.Now()
			if !user.Enabled {
				user.Enabled = true
				user.Verified = true
			}
			tfa.mu.Unlock()
			return true, nil
		}
	}

	return false, nil
}

// generateTOTP 生成TOTP码.
func (tfa *TwoFactorAuth) generateTOTP(secret string, t time.Time) string {
	// 解码密钥
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return ""
	}

	// 计算时间计数器
	counter := uint64(t.Unix()) / 30

	// 将计数器转为字节
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	// HMAC-SHA1
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	// 动态截取
	offset := sum[len(sum)-1] & 0x0F
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7FFFFFFF

	// 格式化为指定位数
	format := fmt.Sprintf("%%0%dd", tfa.config.CodeLength)
	return fmt.Sprintf(format, uint64(code)%pow10(tfa.config.CodeLength))
}

// generateBackupCodes 生成备用码.
func (tfa *TwoFactorAuth) generateBackupCodes() []string {
	codes := make([]string, tfa.config.BackupCodes)
	for i := 0; i < tfa.config.BackupCodes; i++ {
		b := make([]byte, 4)
		rand.Read(b)
		codes[i] = fmt.Sprintf("%08X", binary.BigEndian.Uint32(b))
	}
	return codes
}

// verifyBackupCode 验证备用码.
func (tfa *TwoFactorAuth) verifyBackupCode(userID, code string) bool {
	tfa.mu.Lock()
	defer tfa.mu.Unlock()

	user, ok := tfa.users[userID]
	if !ok {
		return false
	}

	for i, backupCode := range user.BackupCodes {
		if hmac.Equal([]byte(code), []byte(backupCode)) {
			// 使用后移除备用码
			user.BackupCodes = append(user.BackupCodes[:i], user.BackupCodes[i+1:]...)
			user.LastUsed = time.Now()
			return true
		}
	}

	return false
}

// IsEnabled 检查用户是否启用2FA.
func (tfa *TwoFactorAuth) IsEnabled(userID string) bool {
	tfa.mu.RLock()
	defer tfa.mu.RUnlock()

	user, ok := tfa.users[userID]
	return ok && user.Enabled && user.Verified
}

// Disable 禁用用户2FA.
func (tfa *TwoFactorAuth) Disable(userID string) {
	tfa.mu.Lock()
	defer tfa.mu.Unlock()

	if user, ok := tfa.users[userID]; ok {
		user.Enabled = false
	}
}

// GetBackupCodes 获取剩余备用码数量.
func (tfa *TwoFactorAuth) GetBackupCodes(userID string) int {
	tfa.mu.RLock()
	defer tfa.mu.RUnlock()

	if user, ok := tfa.users[userID]; ok {
		return len(user.BackupCodes)
	}
	return 0
}

func pow10(n int) uint64 {
	result := uint64(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}
