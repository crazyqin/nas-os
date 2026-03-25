package securityv2

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- TOTP (RFC 6238) requires HMAC-SHA1, this is not used for password hashing
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nas-os/internal/cache"

	"go.uber.org/zap"
)

// generateSecureCode 生成安全的随机验证码.
func generateSecureCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("生成随机数失败: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// MFAManager 双因素认证管理器.
type MFAManager struct {
	config      MFAConfig
	userSecrets map[string]*MFASecret // UserID -> MFA 密钥
	mu          sync.RWMutex
	cache       *cache.Manager // 缓存管理器，用于存储临时验证码
	// 通知回调
	sendEmailFunc   func(to, subject, body string) error
	sendSMSFunc     func(to, message string) error
	sendWebhookFunc func(url, event string, data map[string]interface{}) error
}

// MFAConfig MFA 配置.
type MFAConfig struct {
	Enabled        bool     `json:"enabled"`
	RequiredFor    []string `json:"required_for"` // 哪些角色必须启用 MFA（admin, user, guest）
	TOTPEnabled    bool     `json:"totp_enabled"`
	SMSEnabled     bool     `json:"sms_enabled"`
	EmailEnabled   bool     `json:"email_enabled"`
	RecoveryCodes  int      `json:"recovery_codes"`  // 生成的恢复码数量
	CodeLength     int      `json:"code_length"`     // 验证码长度（默认 6）
	ValidityPeriod int      `json:"validity_period"` // TOTP 有效期（秒，默认 30）
	SMSService     string   `json:"sms_service"`     // 短信服务商
	SMSSender      string   `json:"sms_sender"`      // 短信发送号码
	EmailSender    string   `json:"email_sender"`    // 邮件发送地址
}

// MFASecret 用户 MFA 密钥.
type MFASecret struct {
	UserID          string     `json:"user_id"`
	Username        string     `json:"username"`
	TOTPSecret      string     `json:"totp_secret"`      // TOTP 密钥（Base32 编码）
	SMSPhone        string     `json:"sms_phone"`        // 手机号
	Email           string     `json:"email"`            // 邮箱
	RecoveryCodes   []string   `json:"recovery_codes"`   // 恢复码（加盐哈希存储）
	Enabled         bool       `json:"enabled"`          // MFA 是否启用
	PreferredMethod string     `json:"preferred_method"` // 首选验证方式（totp, sms, email）
	CreatedAt       time.Time  `json:"created_at"`
	LastUsed        *time.Time `json:"last_used,omitempty"`
}

// MFAVerificationResult MFA 验证结果.
type MFAVerificationResult struct {
	Success        bool   `json:"success"`
	Method         string `json:"method"`
	Message        string `json:"message"`
	RequiresMFA    bool   `json:"requires_mfa"`
	TemporaryToken string `json:"temporary_token,omitempty"` // 临时令牌（用于 MFA 验证期间）
}

// NewMFAManager 创建 MFA 管理器.
func NewMFAManager() *MFAManager {
	// 创建缓存管理器（10000 容量，10 分钟 TTL）
	logger := zap.NewNop() // 使用空 logger，实际使用时可以注入
	cacheMgr := cache.NewManager(10000, 10*time.Minute, logger)

	return &MFAManager{
		config: MFAConfig{
			Enabled:        true,
			RequiredFor:    []string{"admin"},
			TOTPEnabled:    true,
			SMSEnabled:     false,
			EmailEnabled:   true,
			RecoveryCodes:  10,
			CodeLength:     6,
			ValidityPeriod: 30,
		},
		userSecrets: make(map[string]*MFASecret),
		cache:       cacheMgr,
	}
}

// NewMFAManagerWithCache 创建 MFA 管理器（带自定义缓存）.
func NewMFAManagerWithCache(cacheMgr *cache.Manager) *MFAManager {
	return &MFAManager{
		config: MFAConfig{
			Enabled:        true,
			RequiredFor:    []string{"admin"},
			TOTPEnabled:    true,
			SMSEnabled:     false,
			EmailEnabled:   true,
			RecoveryCodes:  10,
			CodeLength:     6,
			ValidityPeriod: 30,
		},
		userSecrets: make(map[string]*MFASecret),
		cache:       cacheMgr,
	}
}

// SetCache 设置缓存管理器.
func (mm *MFAManager) SetCache(cacheMgr *cache.Manager) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.cache = cacheMgr
}

// SetSendEmailFunc 设置邮件发送回调.
func (mm *MFAManager) SetSendEmailFunc(fn func(to, subject, body string) error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.sendEmailFunc = fn
}

// SetSendSMSFunc 设置短信发送回调.
func (mm *MFAManager) SetSendSMSFunc(fn func(to, message string) error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.sendSMSFunc = fn
}

// SetSendWebhookFunc 设置 Webhook 发送回调.
func (mm *MFAManager) SetSendWebhookFunc(fn func(url, event string, data map[string]interface{}) error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.sendWebhookFunc = fn
}

// GenerateTOTPSecret 生成 TOTP 密钥.
func (mm *MFAManager) GenerateTOTPSecret() string {
	// 生成 20 字节的随机密钥
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		panic(err) // crypto/rand 失败是致命错误
	}

	// Base32 编码
	return strings.ToUpper(base32.StdEncoding.EncodeToString(secret))
}

// GenerateTOTPCode 生成 TOTP 验证码.
func (mm *MFAManager) GenerateTOTPCode(secret string, timestamp time.Time) (string, error) {
	// 解码 Base32 密钥
	decoded, err := base32.StdEncoding.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", err
	}

	// 计算时间步长（30 秒）
	timeStep := timestamp.Unix() / int64(mm.config.ValidityPeriod)

	// 将时间步长转换为 8 字节大端序
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(timeStep))

	// 计算 HMAC-SHA1
	h := hmac.New(sha1.New, decoded)
	h.Write(buf[:])
	digest := h.Sum(nil)

	// 动态截断
	offset := digest[len(digest)-1] & 0x0F
	code := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7FFFFFFF

	// 取模得到指定位数的验证码
	code = code % uint32(math.Pow10(mm.config.CodeLength))

	// 格式化为固定长度
	format := fmt.Sprintf("%%0%dd", mm.config.CodeLength)
	return fmt.Sprintf(format, code), nil
}

// VerifyTOTPCode 验证 TOTP 验证码.
func (mm *MFAManager) VerifyTOTPCode(secret, code string) bool {
	// 允许前后各一个时间窗口的误差（防止时钟不同步）
	for offset := -1; offset <= 1; offset++ {
		timestamp := time.Now().Add(time.Duration(offset) * time.Duration(mm.config.ValidityPeriod) * time.Second)
		expectedCode, err := mm.GenerateTOTPCode(secret, timestamp)
		if err != nil {
			continue
		}
		if hmac.Equal([]byte(code), []byte(expectedCode)) {
			return true
		}
	}
	return false
}

// GenerateRecoveryCodes 生成恢复码.
func (mm *MFAManager) GenerateRecoveryCodes() []string {
	codes := make([]string, mm.config.RecoveryCodes)
	for i := 0; i < mm.config.RecoveryCodes; i++ {
		// 生成 8 位随机字母数字组合
		bytes := make([]byte, 4)
		if _, err := rand.Read(bytes); err != nil {
			panic(err) // crypto/rand 失败是致命错误
		}
		codes[i] = hex.EncodeToString(bytes)
	}
	return codes
}

// HashRecoveryCode 哈希恢复码（存储时使用）.
func (mm *MFAManager) HashRecoveryCode(code string) string {
	// 使用 HMAC 哈希恢复码
	h := hmac.New(sha1.New, []byte("nas-os-recovery-code-salt"))
	h.Write([]byte(code))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyRecoveryCode 验证恢复码.
func (mm *MFAManager) VerifyRecoveryCode(storedHashes []string, code string) bool {
	codeHash := mm.HashRecoveryCode(code)
	for _, hash := range storedHashes {
		if hmac.Equal([]byte(codeHash), []byte(hash)) {
			return true
		}
	}
	return false
}

// RemoveUsedRecoveryCode 移除已使用的恢复码.
func (mm *MFAManager) RemoveUsedRecoveryCode(userID, code string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	secret, exists := mm.userSecrets[userID]
	if !exists {
		return fmt.Errorf("用户 MFA 配置不存在")
	}

	codeHash := mm.HashRecoveryCode(code)
	newHashes := make([]string, 0, len(secret.RecoveryCodes))
	found := false

	for _, hash := range secret.RecoveryCodes {
		if hmac.Equal([]byte(codeHash), []byte(hash)) {
			found = true
			continue // 跳过已使用的恢复码
		}
		newHashes = append(newHashes, hash)
	}

	if !found {
		return fmt.Errorf("恢复码不存在")
	}

	secret.RecoveryCodes = newHashes
	return nil
}

// SetupMFA 为用户设置 MFA.
func (mm *MFAManager) SetupMFA(userID, username, phone, email string) (*MFASecret, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// 生成 TOTP 密钥
	totpSecret := mm.GenerateTOTPSecret()

	// 生成恢复码
	recoveryCodes := mm.GenerateRecoveryCodes()
	hashedCodes := make([]string, len(recoveryCodes))
	for i, code := range recoveryCodes {
		hashedCodes[i] = mm.HashRecoveryCode(code)
	}

	secret := &MFASecret{
		UserID:          userID,
		Username:        username,
		TOTPSecret:      totpSecret,
		SMSPhone:        phone,
		Email:           email,
		RecoveryCodes:   hashedCodes,
		Enabled:         false, // 初始为禁用，用户确认后再启用
		PreferredMethod: "totp",
		CreatedAt:       time.Now(),
	}

	mm.userSecrets[userID] = secret

	// 保存恢复码（仅显示一次）
	if err := mm.saveRecoveryCodes(userID, recoveryCodes); err != nil {
		return nil, err
	}

	return secret, nil
}

// saveRecoveryCodes 保存恢复码到文件（仅用于演示，实际应该加密存储）.
func (mm *MFAManager) saveRecoveryCodes(userID string, codes []string) error {
	dir := "/var/lib/nas-os/security/recovery-codes"
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	filePath := filepath.Join(dir, fmt.Sprintf("%s.txt", userID))
	content := fmt.Sprintf("用户恢复码 - 请安全保存，仅显示一次\n生成时间：%s\n\n", time.Now().Format(time.RFC3339))
	for i, code := range codes {
		content += fmt.Sprintf("%d. %s\n", i+1, code)
	}

	return os.WriteFile(filePath, []byte(content), 0600)
}

// EnableMFA 启用 MFA.
func (mm *MFAManager) EnableMFA(userID, method string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	secret, exists := mm.userSecrets[userID]
	if !exists {
		return fmt.Errorf("用户 MFA 配置不存在")
	}

	secret.Enabled = true
	secret.PreferredMethod = method
	now := time.Now()
	secret.LastUsed = &now

	return nil
}

// DisableMFA 禁用 MFA.
func (mm *MFAManager) DisableMFA(userID, verificationCode string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	secret, exists := mm.userSecrets[userID]
	if !exists {
		return fmt.Errorf("用户 MFA 配置不存在")
	}

	// 验证当前 MFA
	if !mm.verifyCode(secret, verificationCode) {
		return fmt.Errorf("验证码错误")
	}

	secret.Enabled = false
	return nil
}

// VerifyMFA 验证 MFA（登录时使用）.
func (mm *MFAManager) VerifyMFA(userID, code, method string) (*MFAVerificationResult, error) {
	mm.mu.RLock()
	secret, exists := mm.userSecrets[userID]
	mm.mu.RUnlock()

	if !exists {
		return &MFAVerificationResult{
			Success:     false,
			Message:     "用户 MFA 配置不存在",
			RequiresMFA: false,
		}, nil
	}

	if !secret.Enabled {
		return &MFAVerificationResult{
			Success:     true,
			Message:     "MFA 未启用",
			RequiresMFA: false,
		}, nil
	}

	// 验证代码
	if !mm.verifyCode(secret, code) {
		return &MFAVerificationResult{
			Success:     false,
			Method:      method,
			Message:     "验证码错误",
			RequiresMFA: true,
		}, nil
	}

	// 更新最后使用时间
	mm.mu.Lock()
	now := time.Now()
	secret.LastUsed = &now
	mm.mu.Unlock()

	return &MFAVerificationResult{
		Success:     true,
		Method:      method,
		Message:     "MFA 验证成功",
		RequiresMFA: false,
	}, nil
}

// verifyCode 验证代码（支持多种方式）.
func (mm *MFAManager) verifyCode(secret *MFASecret, code string) bool {
	// 尝试 TOTP
	if mm.VerifyTOTPCode(secret.TOTPSecret, code) {
		return true
	}

	// 尝试恢复码
	if mm.VerifyRecoveryCode(secret.RecoveryCodes, code) {
		// 移除已使用的恢复码
		_ = mm.RemoveUsedRecoveryCode(secret.UserID, code)
		return true
	}

	return false
}

// SendSMSCode 发送短信验证码.
func (mm *MFAManager) SendSMSCode(userID string) (string, error) {
	mm.mu.RLock()
	secret, exists := mm.userSecrets[userID]
	mm.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("用户配置不存在")
	}

	if secret.SMSPhone == "" {
		return "", fmt.Errorf("未绑定手机号")
	}

	// 生成 6 位随机验证码
	code, err := generateSecureCode()
	if err != nil {
		return "", err
	}

	// 发送短信
	if mm.sendSMSFunc != nil {
		message := fmt.Sprintf("【NAS-OS】您的验证码是：%s，%d 分钟内有效", code, mm.config.ValidityPeriod/60)
		if err := mm.sendSMSFunc(secret.SMSPhone, message); err != nil {
			return "", err
		}
	}

	// 使用缓存存储验证码（带过期时间）
	if mm.cache != nil {
		cacheKey := fmt.Sprintf("sms_code:%s", userID)
		mm.cache.Set(cacheKey, &SMSVerificationData{
			Code:      code,
			Phone:     secret.SMSPhone,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Duration(mm.config.ValidityPeriod) * time.Second),
		})
	}

	return code, nil
}

// SMSVerificationData 短信验证码数据.
type SMSVerificationData struct {
	Code      string    `json:"code"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// VerifySMSCode 验证短信验证码.
func (mm *MFAManager) VerifySMSCode(userID, code string) bool {
	if mm.cache == nil {
		return false
	}

	cacheKey := fmt.Sprintf("sms_code:%s", userID)
	val, ok := mm.cache.Get(cacheKey)
	if !ok {
		return false
	}

	data, ok := val.(*SMSVerificationData)
	if !ok {
		return false
	}

	// 检查是否过期
	if time.Now().After(data.ExpiresAt) {
		mm.cache.Delete(cacheKey)
		return false
	}

	// 验证码匹配
	if data.Code == code {
		// 验证成功后删除验证码
		mm.cache.Delete(cacheKey)
		return true
	}

	return false
}

// SendEmailCode 发送邮件验证码.
func (mm *MFAManager) SendEmailCode(userID string) (string, error) {
	mm.mu.RLock()
	secret, exists := mm.userSecrets[userID]
	mm.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("用户配置不存在")
	}

	if secret.Email == "" {
		return "", fmt.Errorf("未绑定邮箱")
	}

	// 生成 6 位随机验证码
	code, err := generateSecureCode()
	if err != nil {
		return "", err
	}

	// 发送邮件
	if mm.sendEmailFunc != nil {
		subject := "【NAS-OS】验证码"
		body := fmt.Sprintf(`
<html>
<body>
<h2>NAS-OS 验证码</h2>
<p>您的验证码是：<strong style="font-size: 24px; color: #2563EB;">%s</strong></p>
<p>验证码 %d 分钟内有效，请勿泄露给他人。</p>
<p>如果您没有请求验证码，请忽略此邮件。</p>
</body>
</html>`, code, mm.config.ValidityPeriod/60)

		if err := mm.sendEmailFunc(secret.Email, subject, body); err != nil {
			return "", err
		}
	}

	return code, nil
}

// GetMFAStatus 获取用户 MFA 状态.
func (mm *MFAManager) GetMFAStatus(userID string) (map[string]interface{}, error) {
	mm.mu.RLock()
	secret, exists := mm.userSecrets[userID]
	mm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("用户 MFA 配置不存在")
	}

	return map[string]interface{}{
		"enabled":                  secret.Enabled,
		"totp_enabled":             mm.config.TOTPEnabled,
		"sms_enabled":              mm.config.SMSEnabled && secret.SMSPhone != "",
		"email_enabled":            mm.config.EmailEnabled && secret.Email != "",
		"preferred_method":         secret.PreferredMethod,
		"phone_masked":             mm.maskPhone(secret.SMSPhone),
		"email_masked":             mm.maskEmail(secret.Email),
		"recovery_codes_remaining": len(secret.RecoveryCodes),
		"created_at":               secret.CreatedAt,
		"last_used":                secret.LastUsed,
	}, nil
}

// maskPhone 隐藏手机号.
func (mm *MFAManager) maskPhone(phone string) string {
	if len(phone) < 7 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// maskEmail 隐藏邮箱.
func (mm *MFAManager) maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	username := parts[0]
	if len(username) > 2 {
		username = username[:2] + "***"
	}
	return username + "@" + parts[1]
}

// GetConfig 获取 MFA 配置.
func (mm *MFAManager) GetConfig() MFAConfig {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.config
}

// UpdateConfig 更新 MFA 配置.
func (mm *MFAManager) UpdateConfig(config MFAConfig) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.config = config
	return nil
}

// IsRequiredForRole 检查指定角色是否必须启用 MFA.
func (mm *MFAManager) IsRequiredForRole(role string) bool {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, r := range mm.config.RequiredFor {
		if r == role {
			return true
		}
	}
	return false
}

// RegenerateRecoveryCodes 重新生成恢复码.
func (mm *MFAManager) RegenerateRecoveryCodes(userID string) ([]string, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	secret, exists := mm.userSecrets[userID]
	if !exists {
		return nil, fmt.Errorf("用户 MFA 配置不存在")
	}

	// 生成新恢复码
	recoveryCodes := mm.GenerateRecoveryCodes()
	hashedCodes := make([]string, len(recoveryCodes))
	for i, code := range recoveryCodes {
		hashedCodes[i] = mm.HashRecoveryCode(code)
	}

	secret.RecoveryCodes = hashedCodes

	// 保存恢复码
	if err := mm.saveRecoveryCodes(userID, recoveryCodes); err != nil {
		return nil, err
	}

	return recoveryCodes, nil
}

// UpdatePhone 更新手机号.
func (mm *MFAManager) UpdatePhone(userID, phone string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	secret, exists := mm.userSecrets[userID]
	if !exists {
		return fmt.Errorf("用户 MFA 配置不存在")
	}

	secret.SMSPhone = phone
	return nil
}

// UpdateEmail 更新邮箱.
func (mm *MFAManager) UpdateEmail(userID, email string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	secret, exists := mm.userSecrets[userID]
	if !exists {
		return fmt.Errorf("用户 MFA 配置不存在")
	}

	secret.Email = email
	return nil
}
