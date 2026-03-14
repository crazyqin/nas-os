package auth

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EnhancedMFAManager 增强版双因素认证管理器
// 支持 TOTP Secret 加密存储、登录失败限制、会话管理
type EnhancedMFAManager struct {
	mu                sync.RWMutex
	configs           map[string]*MFAConfig // userID -> MFAConfig
	configPath        string
	smsManager        *SMSManager
	backupManager     *SecureBackupCodeManager
	webauthnMgr       *WebAuthnManager
	issuer            string
	encryption        *SecretEncryption
	loginTracker      *LoginAttemptTracker
	sessionManager    *SessionManager
	passwordValidator *PasswordValidator
}

// EnhancedMFAConfig 增强版 MFA 配置
type EnhancedMFAConfig struct {
	ConfigPath        string
	Issuer            string
	SMSProvider       SMSProvider
	EncryptionKeyPath string
	LoginAttemptCfg   LoginAttemptConfig
	SessionCfg        SessionConfig
	PasswordPolicy    PasswordPolicy
}

// NewEnhancedMFAManager 创建增强版 MFA 管理器
func NewEnhancedMFAManager(cfg EnhancedMFAConfig) (*EnhancedMFAManager, error) {
	m := &EnhancedMFAManager{
		configs:    make(map[string]*MFAConfig),
		configPath: cfg.ConfigPath,
		issuer:     cfg.Issuer,
	}

	// 初始化加密器
	if cfg.EncryptionKeyPath != "" {
		m.encryption = NewSecretEncryption(cfg.EncryptionKeyPath)
		// 尝试初始化（如果密钥文件不存在，会创建新密钥）
		// 实际使用时应该从安全配置中获取 passphrase
		if !m.encryption.IsInitialized() {
			// 使用系统随机生成的 passphrase
			passphrase := generateRandomPassphrase()
			if err := m.encryption.Initialize(passphrase); err != nil {
				return nil, fmt.Errorf("初始化加密器失败：%w", err)
			}
		}
	}

	// 初始化登录尝试跟踪器
	loginCfg := cfg.LoginAttemptCfg
	if loginCfg.MaxAttempts == 0 {
		loginCfg = DefaultLoginAttemptConfig
	}
	m.loginTracker = NewLoginAttemptTracker(loginCfg)

	// 初始化会话管理器
	sessionCfg := cfg.SessionCfg
	if sessionCfg.TokenExpiry == 0 {
		sessionCfg = DefaultSessionConfig
	}
	m.sessionManager = NewSessionManager(sessionCfg)

	// 初始化密码验证器
	policy := cfg.PasswordPolicy
	if policy.MinLength == 0 {
		policy = DefaultPasswordPolicy
	}
	m.passwordValidator = NewPasswordValidator(policy)

	// 初始化短信管理器
	m.smsManager = NewSMSManager(cfg.SMSProvider)

	// 初始化备份码管理器
	backupPath := ""
	if cfg.ConfigPath != "" {
		backupPath = filepath.Join(filepath.Dir(cfg.ConfigPath), "backup_codes.json")
	}
	m.backupManager = NewSecureBackupCodeManager(backupPath, m.encryption)

	// 创建 WebAuthn 管理器
	webauthnCfg := WebAuthnConfig{
		RPDisplayName: cfg.Issuer,
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:8080", "https://localhost:8080"},
	}
	var err error
	m.webauthnMgr, err = NewWebAuthnManager(webauthnCfg)
	if err != nil {
		return nil, fmt.Errorf("创建 WebAuthn 管理器失败：%w", err)
	}

	// 加载配置
	if cfg.ConfigPath != "" {
		if err := m.loadConfig(); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// loadConfig 加载配置
func (m *EnhancedMFAManager) loadConfig() error {
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

	// 解密 TOTP Secret
	for userID, cfg := range configs {
		if cfg.TOTPSecret != "" && m.encryption != nil {
			decrypted, err := m.encryption.Decrypt(cfg.TOTPSecret)
			if err != nil {
				// 记录警告但继续加载
				fmt.Printf("警告：解密用户 %s 的 TOTP Secret 失败：%v\n", userID, err)
			} else {
				cfg.TOTPSecret = decrypted
			}
		}
	}

	m.configs = configs
	return nil
}

// saveConfig 保存配置
func (m *EnhancedMFAManager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	// 复制配置并加密 TOTP Secret
	configsToSave := make(map[string]*MFAConfig)
	for userID, cfg := range m.configs {
		configCopy := *cfg
		if cfg.TOTPSecret != "" && m.encryption != nil {
			encrypted, err := m.encryption.Encrypt(cfg.TOTPSecret)
			if err != nil {
				return fmt.Errorf("加密 TOTP Secret 失败：%w", err)
			}
			configCopy.TOTPSecret = encrypted
		}
		configsToSave[userID] = &configCopy
	}

	data, err := json.MarshalIndent(configsToSave, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败：%w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败：%w", err)
	}

	return nil
}

// ========== TOTP 相关 ==========

// SetupTOTP 设置 TOTP
func (m *EnhancedMFAManager) SetupTOTP(userID, username string) (*TOTPSetup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg, ok := m.configs[userID]; ok && cfg.TOTPEnabled {
		return nil, fmt.Errorf("TOTP 已启用")
	}

	setup, err := SetupTOTP(m.issuer, username)
	if err != nil {
		return nil, err
	}

	if m.configs[userID] == nil {
		m.configs[userID] = &MFAConfig{
			UserID:    userID,
			CreatedAt: time.Now(),
		}
	}

	// 存储 TOTP Secret（加密存储在 saveConfig 时处理）
	m.configs[userID].TOTPSecret = setup.Secret
	m.configs[userID].UpdatedAt = time.Now()

	_ = m.saveConfig()

	return setup, nil
}

// EnableTOTP 启用 TOTP
func (m *EnhancedMFAManager) EnableTOTP(userID, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := m.configs[userID]
	if cfg == nil || cfg.TOTPSecret == "" {
		return fmt.Errorf("请先设置 TOTP")
	}

	if cfg.TOTPEnabled {
		return fmt.Errorf("TOTP 已启用")
	}

	if !VerifyTOTP(cfg.TOTPSecret, code) {
		// 尝试备份码
		if m.backupManager != nil {
			if err := m.backupManager.VerifyBackupCode(userID, code); err != nil {
				return fmt.Errorf("验证码无效")
			}
		} else {
			return fmt.Errorf("验证码无效")
		}
	}

	cfg.TOTPEnabled = true
	cfg.Enabled = true
	cfg.UpdatedAt = time.Now()

	_ = m.saveConfig()
	return nil
}

// DisableTOTP 禁用 TOTP
func (m *EnhancedMFAManager) DisableTOTP(userID, verifyCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := m.configs[userID]
	if cfg == nil || !cfg.TOTPEnabled {
		return fmt.Errorf("TOTP 未启用")
	}

	valid := VerifyTOTP(cfg.TOTPSecret, verifyCode)
	if !valid {
		if m.backupManager != nil {
			if err := m.backupManager.VerifyBackupCode(userID, verifyCode); err != nil {
				return fmt.Errorf("验证失败")
			}
		} else {
			return fmt.Errorf("验证失败")
		}
	}

	cfg.TOTPEnabled = false
	cfg.TOTPSecret = ""
	cfg.UpdatedAt = time.Now()

	if !cfg.SMSEnabled && !cfg.WebAuthnEnabled {
		cfg.Enabled = false
	}

	_ = m.saveConfig()
	return nil
}

// ========== 密码验证 ==========

// ValidatePassword 验证密码
func (m *EnhancedMFAManager) ValidatePassword(password string, userInfo ...string) PasswordValidationResult {
	return m.passwordValidator.Validate(password, userInfo...)
}

// ========== 登录失败限制 ==========

// CheckLoginAllowed 检查是否允许登录
func (m *EnhancedMFAManager) CheckLoginAllowed(username, ip string) (bool, string) {
	// 检查用户锁定
	if locked, until := m.loginTracker.IsLocked(username); locked {
		remaining := time.Until(until).Round(time.Second)
		return false, fmt.Sprintf("账户已锁定，请 %s 后重试", remaining)
	}

	// 检查 IP 锁定
	if locked, until := m.loginTracker.IsIPLocked(ip); locked {
		remaining := time.Until(until).Round(time.Second)
		return false, fmt.Sprintf("IP 已被锁定，请 %s 后重试", remaining)
	}

	return true, ""
}

// RecordLoginAttempt 记录登录尝试
func (m *EnhancedMFAManager) RecordLoginAttempt(username, ip string, success bool) {
	m.loginTracker.RecordAttempt(username, ip, success)
}

// GetRemainingLoginAttempts 获取剩余登录尝试次数
func (m *EnhancedMFAManager) GetRemainingLoginAttempts(username string) int {
	return m.loginTracker.GetRemainingAttempts(username)
}

// UnlockUser 解锁用户
func (m *EnhancedMFAManager) UnlockUser(username string) {
	m.loginTracker.Unlock(username)
}

// ========== 会话管理 ==========

// CreateSession 创建会话
func (m *EnhancedMFAManager) CreateSession(userID, username, ip, userAgent string, roles, groups []string) (*Session, error) {
	return m.sessionManager.CreateSession(userID, username, ip, userAgent, roles, groups)
}

// ValidateSession 验证会话
func (m *EnhancedMFAManager) ValidateSession(token string) (*Session, error) {
	return m.sessionManager.ValidateSession(token)
}

// InvalidateSession 使会话失效
func (m *EnhancedMFAManager) InvalidateSession(token string) error {
	return m.sessionManager.InvalidateSession(token)
}

// InvalidateUserSessions 使用户所有会话失效
func (m *EnhancedMFAManager) InvalidateUserSessions(userID string) error {
	return m.sessionManager.InvalidateUserSessions(userID)
}

// ========== MFA 验证 ==========

// VerifyMFA 验证 MFA
func (m *EnhancedMFAManager) VerifyMFA(userID, mfaType, code string) error {
	m.mu.RLock()
	cfg := m.configs[userID]
	m.mu.RUnlock()

	if cfg == nil || !cfg.Enabled {
		return nil
	}

	switch mfaType {
	case "totp":
		if !cfg.TOTPEnabled {
			return fmt.Errorf("TOTP 未启用")
		}
		if !VerifyTOTP(cfg.TOTPSecret, code) {
			if m.backupManager != nil {
				if err := m.backupManager.VerifyBackupCode(userID, code); err != nil {
					return fmt.Errorf("验证码无效")
				}
			} else {
				return fmt.Errorf("验证码无效")
			}
		}
		return nil

	case "sms":
		if !cfg.SMSEnabled {
			return fmt.Errorf("短信验证未启用")
		}
		return m.smsManager.VerifyCode(cfg.Phone, code)

	default:
		return fmt.Errorf("未知的 MFA 类型")
	}
}

// GetStatus 获取 MFA 状态
func (m *EnhancedMFAManager) GetStatus(userID string) *MFAStatus {
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

	backupCount := 0
	if m.backupManager != nil {
		backupCount = m.backupManager.GetUnusedCount(userID)
	}

	return &MFAStatus{
		Enabled:          cfg.Enabled,
		TOTPEnabled:      cfg.TOTPEnabled,
		SMSEnabled:       cfg.SMSEnabled,
		WebAuthnEnabled:  cfg.WebAuthnEnabled,
		BackupCodesCount: backupCount,
		Phone:            cfg.Phone,
	}
}

// GenerateBackupCodes 生成备份码
func (m *EnhancedMFAManager) GenerateBackupCodes(userID string, count int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := m.configs[userID]
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("请先启用双因素认证")
	}

	m.backupManager.InvalidateAll(userID)
	codes, err := m.backupManager.GenerateBackupCodes(userID, count)
	if err != nil {
		return nil, err
	}

	return codes, nil
}

// GetStats 获取统计信息
func (m *EnhancedMFAManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_users":      len(m.configs),
		"mfa_enabled":      0,
		"totp_enabled":     0,
		"sms_enabled":      0,
		"webauthn_enabled": 0,
	}

	for _, cfg := range m.configs {
		if cfg.Enabled {
			stats["mfa_enabled"] = stats["mfa_enabled"].(int) + 1
		}
		if cfg.TOTPEnabled {
			stats["totp_enabled"] = stats["totp_enabled"].(int) + 1
		}
		if cfg.SMSEnabled {
			stats["sms_enabled"] = stats["sms_enabled"].(int) + 1
		}
		if cfg.WebAuthnEnabled {
			stats["webauthn_enabled"] = stats["webauthn_enabled"].(int) + 1
		}
	}

	// 添加登录尝试统计
	stats["login_attempts"] = m.loginTracker.GetStats()

	// 添加会话统计
	stats["sessions"] = m.sessionManager.GetStats()

	return stats
}

// 辅助函数

func generateRandomPassphrase() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
