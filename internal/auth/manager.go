package auth

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MFAManager 双因素认证管理器.
type MFAManager struct {
	mu            sync.RWMutex
	configs       map[string]*MFAConfig // userID -> MFAConfig
	configPath    string
	smsManager    *SMSManager
	backupManager *BackupCodeManager
	webauthnMgr   *WebAuthnManager
	issuer        string
	encryption    *SecretEncryption // encrypts TOTP secrets at rest
}

// MFAStatus MFA 状态.
type MFAStatus struct {
	Enabled          bool   `json:"enabled"`
	TOTPEnabled      bool   `json:"totp_enabled"`
	SMSEnabled       bool   `json:"sms_enabled"`
	WebAuthnEnabled  bool   `json:"webauthn_enabled"`
	BackupCodesCount int    `json:"backup_codes_count"`
	Phone            string `json:"phone,omitempty"`
}

// MFASession MFA 临时会话（用于登录流程）.
type MFASession struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	Token       string    `json:"token"` // 临时令牌
	ExpiresAt   time.Time `json:"expires_at"`
	MFARequired bool      `json:"mfa_required"`
	MFAType     string    `json:"mfa_type"` // totp, sms, webauthn
	Verified    bool      `json:"verified"`
}

var (
	// ErrMFANotConfigured indicates MFA is not configured for the user.
	ErrMFANotConfigured = errors.New("双因素认证未配置")
	// ErrMFASessionExpired indicates the MFA session has expired.
	ErrMFASessionExpired = errors.New("MFA 会话已过期")
	// ErrMFAAlreadyEnabled indicates MFA is already enabled.
	ErrMFAAlreadyEnabled = errors.New("双因素认证已启用")
)

const totpSecretEncPrefix = "enc:v1:"

// MFAConfigRequiresManager reports whether mfa-config.json contains at least one
// user with MFA enabled. Used to fail-closed when NewMFAManager fails but the
// deployment already relies on 2FA (do not silently drop MFA).
// Returns false when path is empty, missing, unreadable, or empty of enabled MFA.
func MFAConfigRequiresManager(configPath string) bool {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return false
	}
	data, err := os.ReadFile(configPath)
	if err != nil || len(data) == 0 {
		return false
	}
	var configs map[string]*MFAConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		// Corrupt file that was meant to hold MFA — treat as required.
		return true
	}
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		if cfg.Enabled || cfg.TOTPEnabled || cfg.SMSEnabled || cfg.WebAuthnEnabled {
			return true
		}
	}
	return false
}

// NewMFAManager 创建 MFA 管理器.
func NewMFAManager(configPath, issuer string, smsProvider SMSProvider) (*MFAManager, error) {
	m := &MFAManager{
		configs:    make(map[string]*MFAConfig),
		configPath: configPath,
		issuer:     issuer,
	}

	// 创建子管理器
	m.smsManager = NewSMSManager(smsProvider)
	m.backupManager = NewBackupCodeManager()

	// Machine-local key for TOTP secrets at rest (alongside mfa config).
	if configPath != "" {
		keyPath := filepath.Join(filepath.Dir(configPath), "mfa-master.key")
		se := NewSecretEncryption(keyPath)
		if err := se.EnsureKey(); err != nil {
			return nil, fmt.Errorf("initialize MFA encryption key: %w", err)
		}
		m.encryption = se
	}

	// WebAuthn RPID/origins: env overrides for real hostnames (defaults stay localhost).
	// NAS_OS_WEBAUTHN_RPID=nas.example.com
	// NAS_OS_WEBAUTHN_ORIGINS=https://nas.example.com,http://127.0.0.1:8080
	webauthnCfg := DefaultWebAuthnConfig(issuer)
	var err error
	m.webauthnMgr, err = NewWebAuthnManager(webauthnCfg)
	if err != nil {
		return nil, fmt.Errorf("创建 WebAuthn 管理器失败：%w", err)
	}

	// 加载配置
	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// loadConfig 加载配置.
func (m *MFAManager) loadConfig() error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败：%w", err)
	}

	var configs map[string]*MFAConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("解析配置文件失败：%w", err)
	}

	// Decrypt TOTP secrets into memory (plaintext only in process).
	for _, cfg := range configs {
		if cfg == nil || cfg.TOTPSecret == "" {
			continue
		}
		plain, err := m.decryptSecret(cfg.TOTPSecret)
		if err != nil {
			return fmt.Errorf("decrypt TOTP secret for %s: %w", cfg.UserID, err)
		}
		cfg.TOTPSecret = plain
	}

	m.configs = configs
	return nil
}

// saveConfig 保存配置.
func (m *MFAManager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	// Encrypt secrets for disk only; leave in-memory configs plaintext.
	disk := make(map[string]*MFAConfig, len(m.configs))
	for id, cfg := range m.configs {
		if cfg == nil {
			continue
		}
		copyCfg := *cfg
		if copyCfg.TOTPSecret != "" {
			enc, err := m.encryptSecret(copyCfg.TOTPSecret)
			if err != nil {
				return fmt.Errorf("encrypt TOTP secret: %w", err)
			}
			copyCfg.TOTPSecret = enc
		}
		disk[id] = &copyCfg
	}

	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败：%w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}

	tmp := m.configPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败：%w", err)
	}
	if err := os.Rename(tmp, m.configPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("写入配置文件失败：%w", err)
	}

	return nil
}

func (m *MFAManager) encryptSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	// Already encrypted (re-save path) — leave as-is.
	if strings.HasPrefix(plain, totpSecretEncPrefix) {
		return plain, nil
	}
	if m.encryption == nil || !m.encryption.IsInitialized() {
		return "", ErrEncryptionNotInitialized
	}
	enc, err := m.encryption.Encrypt(plain)
	if err != nil {
		return "", err
	}
	return totpSecretEncPrefix + enc, nil
}

func (m *MFAManager) decryptSecret(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, totpSecretEncPrefix) {
		// Legacy plaintext on disk — accept and re-encrypt on next save.
		return stored, nil
	}
	if m.encryption == nil || !m.encryption.IsInitialized() {
		return "", ErrEncryptionNotInitialized
	}
	return m.encryption.Decrypt(strings.TrimPrefix(stored, totpSecretEncPrefix))
}

// GetConfig 获取用户 MFA 配置.
func (m *MFAManager) GetConfig(userID string) *MFAConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configs[userID]
}

// GetStatus 获取用户 MFA 状态.
func (m *MFAManager) GetStatus(userID string) *MFAStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg := m.configs[userID]
	if cfg == nil {
		return &MFAStatus{
			Enabled:          false,
			TOTPEnabled:      false,
			SMSEnabled:       false,
			WebAuthnEnabled:  false,
			BackupCodesCount: 0,
		}
	}

	return &MFAStatus{
		Enabled:          cfg.Enabled,
		TOTPEnabled:      cfg.TOTPEnabled,
		SMSEnabled:       cfg.SMSEnabled,
		WebAuthnEnabled:  cfg.WebAuthnEnabled,
		BackupCodesCount: m.backupManager.GetUnusedCount(userID),
		Phone:            cfg.Phone,
	}
}

// ========== TOTP 相关 ==========

// SetupTOTP 设置 TOTP.
func (m *MFAManager) SetupTOTP(userID, username string) (*TOTPSetup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已启用 TOTP
	if cfg, ok := m.configs[userID]; ok && cfg.TOTPEnabled {
		return nil, fmt.Errorf("TOTP 已启用")
	}

	// 生成 TOTP 设置
	setup, err := SetupTOTP(m.issuer, username)
	if err != nil {
		return nil, err
	}

	// 确保配置存在
	if m.configs[userID] == nil {
		m.configs[userID] = &MFAConfig{
			UserID:    userID,
			CreatedAt: time.Now(),
		}
	}

	// Memory holds plaintext; saveConfig encrypts to disk.
	m.configs[userID].TOTPSecret = setup.Secret
	m.configs[userID].UpdatedAt = time.Now()

	// 不保存 URI 和 QR 码，只返回给客户端
	_ = m.saveConfig()

	return setup, nil
}

// EnableTOTP 启用 TOTP（用户扫描 QR 码后调用）.
func (m *MFAManager) EnableTOTP(userID, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := m.configs[userID]
	if cfg == nil || cfg.TOTPSecret == "" {
		return fmt.Errorf("请先设置 TOTP")
	}

	if cfg.TOTPEnabled {
		return fmt.Errorf("TOTP 已启用")
	}

	// 验证初始验证码
	if !VerifyTOTP(cfg.TOTPSecret, code) {
		return fmt.Errorf("验证码无效")
	}

	cfg.TOTPEnabled = true
	cfg.Enabled = true
	cfg.UpdatedAt = time.Now()

	_ = m.saveConfig()
	return nil
}

// DisableTOTP 禁用 TOTP.
func (m *MFAManager) DisableTOTP(userID, verifyCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := m.configs[userID]
	if cfg == nil || !cfg.TOTPEnabled {
		return fmt.Errorf("TOTP 未启用")
	}

	// 需要验证当前 TOTP 码或备份码
	valid := VerifyTOTP(cfg.TOTPSecret, verifyCode)
	if !valid {
		// 尝试备份码
		if err := m.backupManager.VerifyBackupCode(userID, verifyCode); err != nil {
			return fmt.Errorf("验证失败")
		}
	}

	cfg.TOTPEnabled = false
	cfg.TOTPSecret = ""
	cfg.UpdatedAt = time.Now()

	// 检查是否还有其他 MFA 方式
	if !cfg.SMSEnabled && !cfg.WebAuthnEnabled {
		cfg.Enabled = false
	}

	_ = m.saveConfig()
	return nil
}

// ========== 短信验证码相关 ==========

// SendSMSCode 发送短信验证码.
func (m *MFAManager) SendSMSCode(userID, phone string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 确保配置存在
	if m.configs[userID] == nil {
		m.configs[userID] = &MFAConfig{
			UserID:    userID,
			CreatedAt: time.Now(),
		}
	}

	m.configs[userID].Phone = phone
	m.configs[userID].UpdatedAt = time.Now()
	if err := m.saveConfig(); err != nil {
		return err
	}

	return m.smsManager.SendCode(phone)
}

// EnableSMS 启用短信验证.
func (m *MFAManager) EnableSMS(userID, phone, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证短信验证码
	if err := m.smsManager.VerifyCode(phone, code); err != nil {
		return err
	}

	cfg := m.configs[userID]
	if cfg == nil {
		cfg = &MFAConfig{
			UserID:    userID,
			CreatedAt: time.Now(),
		}
		m.configs[userID] = cfg
	}

	cfg.SMSEnabled = true
	cfg.Phone = phone
	cfg.Enabled = true
	cfg.UpdatedAt = time.Now()

	if err := m.saveConfig(); err != nil {
		return err
	}
	return nil
}

// DisableSMS 禁用短信验证.
func (m *MFAManager) DisableSMS(userID, verifyCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := m.configs[userID]
	if cfg == nil || !cfg.SMSEnabled {
		return fmt.Errorf("短信验证未启用")
	}

	// 验证当前短信验证码
	if err := m.smsManager.VerifyCode(cfg.Phone, verifyCode); err != nil {
		return err
	}

	cfg.SMSEnabled = false
	cfg.Phone = ""
	cfg.UpdatedAt = time.Now()

	// 检查是否还有其他 MFA 方式
	if !cfg.TOTPEnabled && !cfg.WebAuthnEnabled {
		cfg.Enabled = false
	}

	if err := m.saveConfig(); err != nil {
		return err
	}
	return nil
}

// ========== 备份码相关 ==========

// GenerateBackupCodes 生成备份码.
func (m *MFAManager) GenerateBackupCodes(userID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := m.configs[userID]
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("请先启用双因素认证")
	}

	// 使旧的备份码失效
	m.backupManager.InvalidateAll(userID)

	// 生成新的备份码
	codes, err := m.backupManager.GenerateBackupCodes(userID, 10)
	if err != nil {
		return nil, err
	}

	return codes, nil
}

// VerifyBackupCode 验证备份码.
func (m *MFAManager) VerifyBackupCode(userID, code string) error {
	return m.backupManager.VerifyBackupCode(userID, code)
}

// ========== WebAuthn 相关 ==========

// BeginWebAuthnRegistration 开始 WebAuthn 注册.
func (m *MFAManager) BeginWebAuthnRegistration(userID, username, displayName string) (string, interface{}, error) {
	if m.webauthnMgr == nil {
		return "", nil, fmt.Errorf("WebAuthn 未配置")
	}
	return m.webauthnMgr.BeginRegistration(userID, username, displayName)
}

// FinishWebAuthnRegistration 完成 WebAuthn 注册.
func (m *MFAManager) FinishWebAuthnRegistration(sessionID string, responseData interface{}) error {
	if m.webauthnMgr == nil {
		return fmt.Errorf("WebAuthn 未配置")
	}

	_, err := m.webauthnMgr.FinishRegistration(sessionID, responseData)
	if err != nil {
		return err
	}

	// 启用 WebAuthn
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.configs[userIDFromSession(sessionID)] == nil {
		m.configs[userIDFromSession(sessionID)] = &MFAConfig{
			UserID:    userIDFromSession(sessionID),
			CreatedAt: time.Now(),
		}
	}

	m.configs[userIDFromSession(sessionID)].WebAuthnEnabled = true
	m.configs[userIDFromSession(sessionID)].Enabled = true
	m.configs[userIDFromSession(sessionID)].UpdatedAt = time.Now()
	if err := m.saveConfig(); err != nil {
		return err
	}

	return nil
}

// BeginWebAuthnAuthentication 开始 WebAuthn 认证.
func (m *MFAManager) BeginWebAuthnAuthentication(userID string) (string, interface{}, error) {
	if m.webauthnMgr == nil {
		return "", nil, fmt.Errorf("WebAuthn 未配置")
	}
	return m.webauthnMgr.BeginAuthentication(userID)
}

// FinishWebAuthnAuthentication 完成 WebAuthn 认证.
func (m *MFAManager) FinishWebAuthnAuthentication(sessionID string, responseData interface{}) (string, error) {
	if m.webauthnMgr == nil {
		return "", fmt.Errorf("WebAuthn 未配置")
	}
	return m.webauthnMgr.FinishAuthentication(sessionID, responseData)
}

// GetWebAuthnCredentials 获取用户的 WebAuthn 凭据.
func (m *MFAManager) GetWebAuthnCredentials(userID string) []*WebAuthnCredential {
	if m.webauthnMgr == nil {
		return nil
	}
	return m.webauthnMgr.GetCredentials(userID)
}

// RemoveWebAuthnCredential 移除 WebAuthn 凭据.
func (m *MFAManager) RemoveWebAuthnCredential(userID, credentialID string) error {
	if m.webauthnMgr == nil {
		return fmt.Errorf("WebAuthn 未配置")
	}
	return m.webauthnMgr.RemoveCredential(userID, credentialID)
}

// ========== MFA 会话管理 ==========

// sessions 临时存储 MFA 会话.
var (
	mfaSessionsMu sync.RWMutex
	mfaSessions   = make(map[string]*MFASession)
)

// CreateMFASession 创建 MFA 会话（登录流程中使用）.
func (m *MFAManager) CreateMFASession(userID, username string, mfaType string) (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	tokenStr := fmt.Sprintf("%x", token)

	session := &MFASession{
		UserID:      userID,
		Username:    username,
		Token:       tokenStr,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
		MFARequired: true,
		MFAType:     mfaType,
	}

	mfaSessionsMu.Lock()
	mfaSessions[tokenStr] = session
	mfaSessionsMu.Unlock()

	return tokenStr, nil
}

// GetMFASession 获取 MFA 会话.
func (m *MFAManager) GetMFASession(token string) (*MFASession, error) {
	mfaSessionsMu.RLock()
	session, ok := mfaSessions[token]
	mfaSessionsMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("会话不存在")
	}

	if time.Now().After(session.ExpiresAt) {
		mfaSessionsMu.Lock()
		delete(mfaSessions, token)
		mfaSessionsMu.Unlock()
		return nil, ErrMFASessionExpired
	}

	return session, nil
}

// CompleteMFASession 完成 MFA 会话.
func (m *MFAManager) CompleteMFASession(token string) error {
	mfaSessionsMu.Lock()
	defer mfaSessionsMu.Unlock()

	session, ok := mfaSessions[token]
	if !ok {
		return fmt.Errorf("会话不存在")
	}

	session.Verified = true
	return nil
}

// DeleteMFASession 删除 MFA 会话.
func (m *MFAManager) DeleteMFASession(token string) {
	mfaSessionsMu.Lock()
	defer mfaSessionsMu.Unlock()
	delete(mfaSessions, token)
}

// ========== 验证 MFA ==========

// VerifyMFA 验证 MFA（登录流程中使用）.
func (m *MFAManager) VerifyMFA(userID, mfaType, code string, responseData interface{}) error {
	m.mu.RLock()
	cfg := m.configs[userID]
	m.mu.RUnlock()

	if cfg == nil || !cfg.Enabled {
		return nil // 未启用 MFA，不需要验证
	}

	switch mfaType {
	case "totp":
		if !cfg.TOTPEnabled {
			return fmt.Errorf("TOTP 未启用")
		}
		if !VerifyTOTP(cfg.TOTPSecret, code) {
			// 尝试备份码
			if err := m.backupManager.VerifyBackupCode(userID, code); err != nil {
				return fmt.Errorf("验证码无效")
			}
		}
		return nil

	case "sms":
		if !cfg.SMSEnabled {
			return fmt.Errorf("短信验证未启用")
		}
		return m.smsManager.VerifyCode(cfg.Phone, code)

	case "webauthn":
		if !cfg.WebAuthnEnabled {
			return fmt.Errorf("WebAuthn 未启用")
		}
		// WebAuthn 通过 responseData 验证
		if responseData == nil {
			return fmt.Errorf("缺少认证数据")
		}
		// 需要在外部调用 FinishWebAuthnAuthentication
		return nil

	default:
		return fmt.Errorf("未知的 MFA 类型")
	}
}

// RequireMFA 检查用户是否需要 MFA.
func (m *MFAManager) RequireMFA(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg := m.configs[userID]
	return cfg != nil && cfg.Enabled
}

// GetMFAType 获取用户启用的 MFA 类型.
func (m *MFAManager) GetMFAType(userID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg := m.configs[userID]
	if cfg == nil || !cfg.Enabled {
		return ""
	}

	// 优先级：WebAuthn > TOTP > SMS
	if cfg.WebAuthnEnabled {
		return "webauthn"
	}
	if cfg.TOTPEnabled {
		return "totp"
	}
	if cfg.SMSEnabled {
		return "sms"
	}
	return ""
}

// helper function to extract userID from session.
func userIDFromSession(sessionID string) string {
	// This is a placeholder - in real implementation,
	// you would store and retrieve userID from session
	return sessionID
}
