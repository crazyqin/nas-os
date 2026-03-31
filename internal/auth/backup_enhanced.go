package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BackupCodeStatus 备份码状态.
type BackupCodeStatus struct {
	Code       string     `json:"code"`
	CreatedAt  time.Time  `json:"created_at"`
	Used       bool       `json:"used"`
	UsedAt     *time.Time `json:"used_at,omitempty"`
	UsedIP     string     `json:"used_ip,omitempty"`
	UsedReason string     `json:"used_reason,omitempty"`
}

// EnhancedBackupCodeConfig 增强版备份码配置.
type EnhancedBackupCodeConfig struct {
	Count          int           `json:"count"`           // 生成数量
	Format         string        `json:"format"`          // 格式：simple (XXXX-XXXX) 或 enhanced (XXXX-XXXX-XXXX)
	ExpiryDays     int           `json:"expiry_days"`     // 过期天数（0表示永不过期）
	MaxUsage       int           `json:"max_usage"`       // 单码最大使用次数（默认1）
	StoragePath    string        `json:"storage_path"`    // 存储路径
	EncryptionKey  string        `json:"encryption_key"`  // 加密密钥（可选）
}

// DefaultEnhancedBackupCodeConfig 默认配置.
var DefaultEnhancedBackupCodeConfig = EnhancedBackupCodeConfig{
	Count:      10,
	Format:     "enhanced",
	ExpiryDays: 90,
	MaxUsage:   1,
}

// EnhancedBackupCodeManager 增强版备份码管理器.
type EnhancedBackupCodeManager struct {
	mu         sync.RWMutex
	config     EnhancedBackupCodeConfig
	codes      map[string]map[string]*BackupCodeStatus // userID -> codeHash -> status
	encryption *SecretEncryption
	auditLog   *SecurityAuditLogger
}

// NewEnhancedBackupCodeManager 创建增强版备份码管理器.
func NewEnhancedBackupCodeManager(config EnhancedBackupCodeConfig, encryption *SecretEncryption, auditLog *SecurityAuditLogger) *EnhancedBackupCodeManager {
	if config.Count <= 0 {
		config.Count = DefaultEnhancedBackupCodeConfig.Count
	}
	if config.Format == "" {
		config.Format = DefaultEnhancedBackupCodeConfig.Format
	}
	if config.MaxUsage <= 0 {
		config.MaxUsage = DefaultEnhancedBackupCodeConfig.MaxUsage
	}

	m := &EnhancedBackupCodeManager{
		config:     config,
		codes:      make(map[string]map[string]*BackupCodeStatus),
		encryption: encryption,
		auditLog:   auditLog,
	}

	// 加载已存储的备份码
	if config.StoragePath != "" {
		_ = m.load() // 加载失败不影响初始化
	}

	return m
}

// GenerateBackupCodesEnhanced 生成增强版备份码.
func (m *EnhancedBackupCodeManager) GenerateBackupCodesEnhanced(userID, ip string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清除旧的备份码
	delete(m.codes, userID)
	m.codes[userID] = make(map[string]*BackupCodeStatus)

	now := time.Now()
	plainCodes := make([]string, m.config.Count)

	for i := 0; i < m.config.Count; i++ {
		var code string
		var err error

		if m.config.Format == "enhanced" {
			code, err = GenerateRandomBackupCode()
		} else {
			code, err = generateBackupCode()
		}

		if err != nil {
			return nil, fmt.Errorf("生成备份码失败：%w", err)
		}

		plainCodes[i] = code

		// 计算哈希存储（不存储明文）
		codeHash := hashBackupCodeEnhanced(code)

		m.codes[userID][codeHash] = &BackupCodeStatus{
			Code:      codeHash,
			CreatedAt: now,
			Used:      false,
		}
	}

	// 记录审计日志
	if m.auditLog != nil {
		m.auditLog.Log(SecurityAuditEntry{
			Category: "mfa",
			Event:    "backup_codes_generated",
			UserID:   userID,
			IP:       ip,
			Status:   "success",
			Details: map[string]interface{}{
				"count":      m.config.Count,
				"format":     m.config.Format,
				"expiry_days": m.config.ExpiryDays,
			},
		})
	}

	// 保存
	_ = m.save()

	return plainCodes, nil
}

// VerifyBackupCodeEnhanced 验证增强版备份码.
func (m *EnhancedBackupCodeManager) VerifyBackupCodeEnhanced(userID, code, ip, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	userCodes, ok := m.codes[userID]
	if !ok {
		if m.auditLog != nil {
			m.auditLog.Log(SecurityAuditEntry{
				Category: "mfa",
				Event:    "backup_code_verify_failed",
				UserID:   userID,
				IP:       ip,
				Status:   "failure",
				Reason:   "用户无备份码",
			})
		}
		return fmt.Errorf("备份码不存在")
	}

	codeHash := hashBackupCodeEnhanced(code)
	backupCode, ok := userCodes[codeHash]
	if !ok {
		if m.auditLog != nil {
			m.auditLog.Log(SecurityAuditEntry{
				Category: "mfa",
				Event:    "backup_code_verify_failed",
				UserID:   userID,
				IP:       ip,
				Status:   "failure",
				Reason:   "备份码无效",
			})
		}
		return fmt.Errorf("%s", ErrBackupCodeInvalid)
	}

	// 检查过期
	if m.config.ExpiryDays > 0 {
		expiryDate := backupCode.CreatedAt.AddDate(0, 0, m.config.ExpiryDays)
		if time.Now().After(expiryDate) {
			if m.auditLog != nil {
				m.auditLog.Log(SecurityAuditEntry{
					Category: "mfa",
					Event:    "backup_code_expired",
					UserID:   userID,
					IP:       ip,
					Status:   "failure",
					Reason:   "备份码已过期",
					Details: map[string]interface{}{
						"created_at": backupCode.CreatedAt,
						"expiry_date": expiryDate,
					},
				})
			}
			return fmt.Errorf("备份码已过期")
		}
	}

	// 检查使用次数
	if backupCode.Used {
		if m.auditLog != nil {
			m.auditLog.Log(SecurityAuditEntry{
				Category: "mfa",
				Event:    "backup_code_already_used",
				UserID:   userID,
				IP:       ip,
				Status:   "failure",
				Reason:   "备份码已使用",
				Details: map[string]interface{}{
					"used_at": backupCode.UsedAt,
					"used_ip": backupCode.UsedIP,
				},
			})
		}
		return fmt.Errorf("%s", ErrBackupCodeUsed)
	}

	// 标记为已使用
	now := time.Now()
	backupCode.Used = true
	backupCode.UsedAt = &now
	backupCode.UsedIP = ip
	backupCode.UsedReason = reason

	// 记录审计日志
	if m.auditLog != nil {
		m.auditLog.Log(SecurityAuditEntry{
			Category: "mfa",
			Event:    "backup_code_used",
			UserID:   userID,
			IP:       ip,
			Status:   "success",
			Reason:   reason,
			Details: map[string]interface{}{
				"remaining_codes": m.GetUnusedCount(userID),
			},
		})
	}

	// 保存
	_ = m.save()

	return nil
}

// GetUnusedCount 获取未使用的备份码数量.
func (m *EnhancedBackupCodeManager) GetUnusedCount(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userCodes, ok := m.codes[userID]
	if !ok {
		return 0
	}

	count := 0
	now := time.Now()

	for _, code := range userCodes {
		if !code.Used {
			// 检查是否过期
			if m.config.ExpiryDays > 0 {
				expiryDate := code.CreatedAt.AddDate(0, 0, m.config.ExpiryDays)
				if now.After(expiryDate) {
					continue
				}
			}
			count++
		}
	}

	return count
}

// GetTotalCount 获取总备份码数量.
func (m *EnhancedBackupCodeManager) GetTotalCount(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userCodes, ok := m.codes[userID]
	if !ok {
		return 0
	}

	return len(userCodes)
}

// GetUsedCodesHistory 获取已使用备份码历史（审计用途）.
func (m *EnhancedBackupCodeManager) GetUsedCodesHistory(userID string) []*BackupCodeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userCodes, ok := m.codes[userID]
	if !ok {
		return nil
	}

	used := make([]*BackupCodeStatus, 0)
	for _, code := range userCodes {
		if code.Used {
			// 复制，不返回完整哈希
			used = append(used, &BackupCodeStatus{
				Code:       code.Code[:8] + "...", // 只显示部分哈希
				CreatedAt:  code.CreatedAt,
				Used:       true,
				UsedAt:     code.UsedAt,
				UsedIP:     code.UsedIP,
				UsedReason: code.UsedReason,
			})
		}
	}

	return used
}

// InvalidateAll 清除所有备份码.
func (m *EnhancedBackupCodeManager) InvalidateAll(userID, ip, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.codes, userID)

	if m.auditLog != nil {
		m.auditLog.Log(SecurityAuditEntry{
			Category: "mfa",
			Event:    "backup_codes_invalidated",
			UserID:   userID,
			IP:       ip,
			Status:   "success",
			Reason:   reason,
		})
	}

	_ = m.save()

	return nil
}

// RegenerateCodes 重新生成备份码（需要验证）.
func (m *EnhancedBackupCodeManager) RegenerateCodes(userID, ip, verifyCode string, verifyFunc func(code string) bool) ([]string, error) {
	m.mu.RLock()
	hasCodes := len(m.codes[userID]) > 0
	m.mu.RUnlock()

	if hasCodes {
		// 验证现有备份码或TOTP
		if !verifyFunc(verifyCode) {
			return nil, fmt.Errorf("验证失败")
		}
	}

	return m.GenerateBackupCodesEnhanced(userID, ip)
}

// CheckLowCodesWarning 检查是否需要备份码不足警告.
func (m *EnhancedBackupCodeManager) CheckLowCodesWarning(userID string) (bool, int) {
	unused := m.GetUnusedCount(userID)
	threshold := m.config.Count / 3 // 低于1/3时警告
	return unused <= threshold, unused
}

// load 加载备份码.
func (m *EnhancedBackupCodeManager) load() error {
	if m.config.StoragePath == "" {
		return nil
	}

	data, err := os.ReadFile(m.config.StoragePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// 如果有加密，先解密
	if m.encryption != nil {
		decrypted, err := m.encryption.Decrypt(string(data))
		if err != nil {
			return err
		}
		data = []byte(decrypted)
	}

	return json.Unmarshal(data, &m.codes)
}

// save 保存备份码.
func (m *EnhancedBackupCodeManager) save() error {
	if m.config.StoragePath == "" {
		return nil
	}

	data, err := json.MarshalIndent(m.codes, "", "  ")
	if err != nil {
		return err
	}

	// 如果有加密，加密存储
	if m.encryption != nil {
		encrypted, err := m.encryption.Encrypt(string(data))
		if err != nil {
			return err
		}
		data = []byte(encrypted)
	}

	if err := os.MkdirAll(filepath.Dir(m.config.StoragePath), 0700); err != nil {
		return err
	}

	return os.WriteFile(m.config.StoragePath, data, 0600)
}

// GetStats 获取统计信息.
func (m *EnhancedBackupCodeManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalUsers := len(m.codes)
	totalCodes := 0
	totalUsed := 0
	totalUnused := 0

	for _, userCodes := range m.codes {
		for _, code := range userCodes {
			totalCodes++
			if code.Used {
				totalUsed++
			} else {
				totalUnused++
			}
		}
	}

	return map[string]interface{}{
		"total_users":  totalUsers,
		"total_codes":  totalCodes,
		"used_codes":   totalUsed,
		"unused_codes": totalUnused,
		"config_count": m.config.Count,
		"expiry_days":  m.config.ExpiryDays,
		"format":       m.config.Format,
	}
}

// hashBackupCodeEnhanced 使用 SHA256 哈希备份码（增强版）
func hashBackupCodeEnhanced(code string) string {
	h := sha256.New()
	h.Write([]byte(code))
	return hex.EncodeToString(h.Sum(nil))
}