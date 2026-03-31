package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// EnhancedTOTPConfig 增强版 TOTP 配置.
type EnhancedTOTPConfig struct {
	Algorithm     string        `json:"algorithm"`      // SHA1, SHA256, SHA512
	Digits        int           `json:"digits"`         // 验证码位数（6或8）
	Period        uint          `json:"period"`         // 时间周期（秒）
	Skew          uint          `json:"skew"`           // 允许的时间窗口偏差
	RateLimit     int           `json:"rate_limit"`     // 每分钟最大验证次数
	ReplayWindow  time.Duration `json:"replay_window"`  // 重放攻击防护窗口
	Issuer        string        `json:"issuer"`         // 发行者名称
	AccountPrefix string        `json:"account_prefix"` // 账户名前缀
}

// DefaultEnhancedTOTPConfig 默认增强 TOTP 配置.
var DefaultEnhancedTOTPConfig = EnhancedTOTPConfig{
	Algorithm:     "SHA1",
	Digits:        6,
	Period:        30,
	Skew:          1,
	RateLimit:     10,
	ReplayWindow:  5 * time.Minute,
	Issuer:        "NAS-OS",
	AccountPrefix: "",
}

// EnhancedTOTPManager 增强版 TOTP 管理器.
type EnhancedTOTPManager struct {
	mu             sync.RWMutex
	config         EnhancedTOTPConfig
	usedCodes      map[string]*UsedCodeRecord // userID -> 最近使用的验证码记录
	attemptCount   map[string]*TOTPAttemptCount // userID -> 验证尝试计数
	encryption     *SecretEncryption
	auditLogger    *SecurityAuditLogger
}

// UsedCodeRecord 已使用验证码记录（防重放攻击）.
type UsedCodeRecord struct {
	Code      string    `json:"code"`
	UsedAt    time.Time `json:"used_at"`
	Period    uint      `json:"period"` // 使用的时间周期
}

// TOTPAttemptCount TOTP 验证尝试计数.
type TOTPAttemptCount struct {
	Count     int       `json:"count"`
	ResetAt   time.Time `json:"reset_at"` // 计数重置时间
	LastCode  string    `json:"last_code"` // 最后尝试的验证码
}

// NewEnhancedTOTPManager 创建增强版 TOTP 管理器.
func NewEnhancedTOTPManager(config EnhancedTOTPConfig, encryption *SecretEncryption, auditLogger *SecurityAuditLogger) *EnhancedTOTPManager {
	if config.Algorithm == "" {
		config.Algorithm = DefaultEnhancedTOTPConfig.Algorithm
	}
	if config.Digits == 0 {
		config.Digits = DefaultEnhancedTOTPConfig.Digits
	}
	if config.Period == 0 {
		config.Period = DefaultEnhancedTOTPConfig.Period
	}
	if config.RateLimit == 0 {
		config.RateLimit = DefaultEnhancedTOTPConfig.RateLimit
	}
	if config.ReplayWindow == 0 {
		config.ReplayWindow = DefaultEnhancedTOTPConfig.ReplayWindow
	}

	return &EnhancedTOTPManager{
		config:       config,
		usedCodes:    make(map[string]*UsedCodeRecord),
		attemptCount: make(map[string]*TOTPAttemptCount),
		encryption:   encryption,
		auditLogger:  auditLogger,
	}
}

// GenerateTOTPSecretEnhanced 生成增强版 TOTP 密钥.
func (m *EnhancedTOTPManager) GenerateTOTPSecretEnhanced(accountName string) (*TOTPSetup, error) {
	issuer := m.config.Issuer
	if issuer == "" {
		issuer = DefaultEnhancedTOTPConfig.Issuer
	}

	if m.config.AccountPrefix != "" {
		accountName = m.config.AccountPrefix + "_" + accountName
	}

	// 生成密钥
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Digits:      otpDigits(m.config.Digits),
		Algorithm:   otpAlgorithm(m.config.Algorithm),
		Period:      m.config.Period,
	})
	if err != nil {
		return nil, err
	}

	// 生成 QR 码
	qrCode, err := GenerateTOTPQRCode(key.String())
	if err != nil {
		return nil, err
	}

	return &TOTPSetup{
		Secret:      key.Secret(),
		URI:         key.String(),
		QRCode:      qrCode,
		Issuer:      issuer,
		AccountName: accountName,
	}, nil
}

// VerifyTOTPEnhanced 增强版 TOTP 验证（含速率限制和重放防护）.
func (m *EnhancedTOTPManager) VerifyTOTPEnhanced(userID, secret, code, ip string) (bool, string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 1. 检查速率限制
	attempts := m.attemptCount[userID]
	if attempts == nil {
		attempts = &TOTPAttemptCount{
			ResetAt: now.Add(time.Minute),
		}
		m.attemptCount[userID] = attempts
	}

	// 检查是否需要重置计数
	if now.After(attempts.ResetAt) {
		attempts.Count = 0
		attempts.ResetAt = now.Add(time.Minute)
	}

	// 检查速率限制
	if attempts.Count >= m.config.RateLimit {
		if m.auditLogger != nil {
			m.auditLogger.Log(SecurityAuditEntry{
				Category: "mfa",
				Event:    "totp_rate_limited",
				UserID:   userID,
				IP:       ip,
				Status:   "failure",
				Reason:   "验证次数超过限制",
				Details: map[string]interface{}{
					"rate_limit": m.config.RateLimit,
					"count":      attempts.Count,
				},
			})
		}
		return false, "验证次数超过限制，请稍后再试"
	}

	// 2. 检查重放攻击
	usedRecord := m.usedCodes[userID]
	if usedRecord != nil {
		// 如果在重放窗口内且验证码相同
		if now.Sub(usedRecord.UsedAt) < m.config.ReplayWindow && usedRecord.Code == code {
			if m.auditLogger != nil {
				m.auditLogger.Log(SecurityAuditEntry{
					Category: "mfa",
					Event:    "totp_replay_detected",
					UserID:   userID,
					IP:       ip,
					Status:   "failure",
					Reason:   "验证码已被使用（重放攻击）",
					Details: map[string]interface{}{
						"code":        code,
						"used_at":     usedRecord.UsedAt,
					},
				})
			}
			return false, "验证码已使用，请等待新验证码"
		}
	}

	// 3. 执行验证
	valid := totp.Validate(code, secret)

	// 记录尝试
	attempts.Count++
	attempts.LastCode = code

	if valid {
		// 记录已使用的验证码
		m.usedCodes[userID] = &UsedCodeRecord{
			Code:   code,
			UsedAt: now,
			Period: m.config.Period,
		}

		// 重置失败计数
		attempts.Count = 0

		if m.auditLogger != nil {
			m.auditLogger.Log(SecurityAuditEntry{
				Category: "mfa",
				Event:    "totp_verify_success",
				UserID:   userID,
				IP:       ip,
				Status:   "success",
			})
		}

		return true, ""
	}

	if m.auditLogger != nil {
		m.auditLogger.Log(SecurityAuditEntry{
			Category: "mfa",
			Event:    "totp_verify_failed",
			UserID:   userID,
			IP:       ip,
			Status:   "failure",
			Reason:   "验证码无效",
		})
	}

	return false, "验证码无效"
}

// GetRemainingAttempts 获取剩余验证次数.
func (m *EnhancedTOTPManager) GetRemainingAttempts(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	attempts := m.attemptCount[userID]
	if attempts == nil {
		return m.config.RateLimit
	}

	// 检查是否需要重置
	if time.Now().After(attempts.ResetAt) {
		return m.config.RateLimit
	}

	return m.config.RateLimit - attempts.Count
}

// ClearAttemptCount 清除验证尝试计数.
func (m *EnhancedTOTPManager) ClearAttemptCount(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.attemptCount, userID)
}

// GetConfig 获取配置.
func (m *EnhancedTOTPManager) GetConfig() EnhancedTOTPConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *EnhancedTOTPManager) UpdateConfig(config EnhancedTOTPConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// Cleanup 清理过期记录.
func (m *EnhancedTOTPManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 清理过期的已使用验证码记录
	for userID, record := range m.usedCodes {
		if now.Sub(record.UsedAt) > m.config.ReplayWindow {
			delete(m.usedCodes, userID)
		}
	}

	// 清理过期的尝试计数
	for userID, attempts := range m.attemptCount {
		if now.After(attempts.ResetAt) {
			delete(m.attemptCount, userID)
		}
	}
}

// GetStats 获取统计信息.
func (m *EnhancedTOTPManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"algorithm":      m.config.Algorithm,
		"digits":         m.config.Digits,
		"period":         m.config.Period,
		"skew":           m.config.Skew,
		"rate_limit":     m.config.RateLimit,
		"replay_window":  m.config.ReplayWindow.String(),
		"tracked_users":  len(m.usedCodes),
		"active_attempts": len(m.attemptCount),
	}
}

// 辅助函数

func otpDigits(digits int) otp.Digits {
	if digits == 8 {
		return otp.DigitsEight
	}
	return otp.DigitsSix
}

func otpAlgorithm(algorithm string) otp.Algorithm {
	switch algorithm {
	case "SHA256":
		return otp.AlgorithmSHA256
	case "SHA512":
		return otp.AlgorithmSHA512
	default:
		return otp.AlgorithmSHA1
	}
}

// GenerateRandomBackupCode 生成随机备份码（增强格式）.
func GenerateRandomBackupCode() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	hexStr := hex.EncodeToString(b)
	// 格式：XXXX-XXXX-XXXX (更易读)
	return hexStr[:4] + "-" + hexStr[4:8] + "-" + hexStr[8:12], nil
}