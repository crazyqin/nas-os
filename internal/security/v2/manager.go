package securityv2

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// SecurityManagerV2 安全管理器 v2
type SecurityManagerV2 struct {
	mfa        *MFAManager
	encryption *EncryptionManager
	alerting   *AlertingManager
	config     SecurityConfigV2
	mu         sync.RWMutex
}

// SecurityConfigV2 安全配置 v2
type SecurityConfigV2 struct {
	MFA        MFAConfig        `json:"mfa"`
	Encryption EncryptionConfig `json:"encryption"`
	Alerting   AlertingConfig   `json:"alerting"`
}

// NewSecurityManagerV2 创建安全管理器 v2
func NewSecurityManagerV2() *SecurityManagerV2 {
	return &SecurityManagerV2{
		mfa:        NewMFAManager(),
		encryption: NewEncryptionManager(),
		alerting:   NewAlertingManager(),
		config: SecurityConfigV2{
			MFA: MFAConfig{
				Enabled:        true,
				RequiredFor:    []string{"admin"},
				TOTPEnabled:    true,
				SMSEnabled:     false,
				EmailEnabled:   true,
				RecoveryCodes:  10,
				CodeLength:     6,
				ValidityPeriod: 30,
			},
			Encryption: EncryptionConfig{
				Enabled:         true,
				Algorithm:       "aes-256-gcm",
				KeyDerivation:   "argon2id",
				MasterKeyPath:   "/var/lib/nas-os/security/master.key",
				SaltPath:        "/var/lib/nas-os/security/salt",
				EncryptedPrefix: ".encrypted_",
			},
			Alerting: AlertingConfig{
				Enabled:         true,
				EmailEnabled:    false,
				WeComEnabled:    false,
				WebhookEnabled:  false,
				MinSeverity:     "medium",
				RateLimit:       10,
			},
		},
	}
}

// GetMFAManager 获取 MFA 管理器
func (sm *SecurityManagerV2) GetMFAManager() *MFAManager {
	return sm.mfa
}

// GetEncryptionManager 获取加密管理器
func (sm *SecurityManagerV2) GetEncryptionManager() *EncryptionManager {
	return sm.encryption
}

// GetAlertingManager 获取告警管理器
func (sm *SecurityManagerV2) GetAlertingManager() *AlertingManager {
	return sm.alerting
}

// GetConfig 获取安全配置
func (sm *SecurityManagerV2) GetConfig() SecurityConfigV2 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.config
}

// UpdateConfig 更新安全配置
func (sm *SecurityManagerV2) UpdateConfig(config SecurityConfigV2) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.config = config

	// 同步配置到子模块
	sm.mfa.UpdateConfig(config.MFA)
	sm.encryption.UpdateConfig(config.Encryption)
	sm.alerting.UpdateConfig(config.Alerting)

	return nil
}

// Initialize 初始化安全系统
func (sm *SecurityManagerV2) Initialize(masterPassword string) error {
	// 初始化加密系统
	if err := sm.encryption.Initialize(masterPassword); err != nil {
		return err
	}

	return nil
}

// SendSecurityAlert 发送安全告警
func (sm *SecurityManagerV2) SendSecurityAlert(severity, alertType, title, description, sourceIP, username string, details map[string]interface{}) error {
	alert := SecurityAlertV2{
		ID:          generateAlertIDV2(),
		Timestamp:   time.Now(),
		Severity:    severity,
		Type:        alertType,
		Title:       title,
		Description: description,
		SourceIP:    sourceIP,
		Username:    username,
		Details:     details,
	}

	return sm.alerting.SendAlert(alert)
}

// generateAlertIDV2 生成告警 ID
func generateAlertIDV2() string {
	return "alert-" + uuid.New().String()
}

// GetSecurityDashboard 获取安全仪表板数据
func (sm *SecurityManagerV2) GetSecurityDashboard() map[string]interface{} {
	mfaStats := sm.getMFAStats()
	alertStats := sm.alerting.GetAlertStats()
	encryptionStatus := sm.getEncryptionStatus()

	return map[string]interface{}{
		"mfa":         mfaStats,
		"alerting":    alertStats,
		"encryption":  encryptionStatus,
		"timestamp":   time.Now().Format(time.RFC3339),
	}
}

func (sm *SecurityManagerV2) getMFAStats() map[string]interface{} {
	sm.mfa.mu.RLock()
	defer sm.mfa.mu.RUnlock()

	totalUsers := len(sm.mfa.userSecrets)
	enabledUsers := 0
	totpEnabled := 0
	smsEnabled := 0
	emailEnabled := 0

	for _, secret := range sm.mfa.userSecrets {
		if secret.Enabled {
			enabledUsers++
		}
		if secret.TOTPSecret != "" {
			totpEnabled++
		}
		if secret.SMSPhone != "" {
			smsEnabled++
		}
		if secret.Email != "" {
			emailEnabled++
		}
	}

	return map[string]interface{}{
		"total_users":     totalUsers,
		"enabled_users":   enabledUsers,
		"totp_enabled":    totpEnabled,
		"sms_enabled":     smsEnabled,
		"email_enabled":   emailEnabled,
		"coverage_rate":   float64(enabledUsers) / float64(totalUsers) * 100,
	}
}

func (sm *SecurityManagerV2) getEncryptionStatus() map[string]interface{} {
	dirs := sm.encryption.GetEncryptedDirectories()
	isInitialized := sm.encryption.IsInitialized()

	return map[string]interface{}{
		"initialized":        isInitialized,
		"encrypted_dirs":     len(dirs),
		"algorithm":          sm.config.Encryption.Algorithm,
		"key_derivation":     sm.config.Encryption.KeyDerivation,
	}
}
