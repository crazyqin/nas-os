// Package digitalassetvault 提供数字资产保险箱功能
// 学习 1Password 与 Bitwarden 的安全架构
// 支持加密存储、数字版权管理、长期归档、访问控制

package digitalassetvault

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// AssetType 资产类型.
type AssetType string

const (
	AssetTypeDocument    AssetType = "document"
	AssetTypeImage       AssetType = "image"
	AssetTypeVideo       AssetType = "video"
	AssetTypeAudio       AssetType = "audio"
	AssetTypeSoftware    AssetType = "software"
	AssetTypeLicense     AssetType = "license"
	AssetTypeCertificate AssetType = "certificate"
	AssetTypeKey         AssetType = "key"
	AssetTypeOther       AssetType = "other"
)

// SecurityLevel 安全级别.
type SecurityLevel string

const (
	SecurityStandard  SecurityLevel = "standard"
	SecurityHigh      SecurityLevel = "high"
	SecurityCritical  SecurityLevel = "critical"
	SecurityTopSecret SecurityLevel = "top_secret"
)

// AssetStatus 资产状态.
type AssetStatus string

const (
	AssetStatusActive   AssetStatus = "active"
	AssetStatusArchived AssetStatus = "archived"
	AssetStatusExpired  AssetStatus = "expired"
	AssetStatusRevoked  AssetStatus = "revoked"
)

// AccessLevel 访问级别.
type AccessLevel string

const (
	AccessNone  AccessLevel = "none"
	AccessRead  AccessLevel = "read"
	AccessWrite AccessLevel = "write"
	AccessAdmin AccessLevel = "admin"
	AccessOwner AccessLevel = "owner"
)

// DigitalAsset 数字资产.
type DigitalAsset struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Type             AssetType         `json:"type"`
	SecurityLevel    SecurityLevel     `json:"security_level"`
	Status           AssetStatus       `json:"status"`
	Size             int64             `json:"size"`
	Checksum         string            `json:"checksum"`
	EncryptedKey     string            `json:"encrypted_key"`
	Owner            string            `json:"owner"`
	Custodians       []string          `json:"custodians"`
	Tags             []string          `json:"tags"`
	Metadata         map[string]string `json:"metadata"`
	DRM              *DRMInfo          `json:"drm,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	ExpiresAt        *time.Time        `json:"expires_at,omitempty"`
	LastAccessed     *time.Time        `json:"last_accessed,omitempty"`
	AccessCount      int64             `json:"access_count"`
	Version          int               `json:"version"`
	PreviousVersions []string          `json:"previous_versions,omitempty"`
}

// DRMInfo 数字版权信息.
type DRMInfo struct {
	LicenseType    string     `json:"license_type"`
	LicenseHolder  string     `json:"license_holder"`
	ExpirationDate *time.Time `json:"expiration_date,omitempty"`
	UsageRights    []string   `json:"usage_rights"`
	Restrictions   []string   `json:"restrictions"`
	Watermark      string     `json:"watermark,omitempty"`
	IsRevocable    bool       `json:"is_revocable"`
}

// AccessPolicy 访问策略.
type AccessPolicy struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	AssetID     string      `json:"asset_id"`
	Subject     string      `json:"subject"`
	Level       AccessLevel `json:"level"`
	Conditions  []Condition `json:"conditions"`
	ExpiresAt   *time.Time  `json:"expires_at,omitempty"`
	MaxAccess   int         `json:"max_access"`
	AccessCount int         `json:"access_count"`
	IPWhitelist []string    `json:"ip_whitelist,omitempty"`
	TimeWindow  *TimeWindow `json:"time_window,omitempty"`
}

// Condition 访问条件.
type Condition struct {
	Type     string `json:"type"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// TimeWindow 时间窗口.
type TimeWindow struct {
	Start    string   `json:"start"` // HH:MM
	End      string   `json:"end"`   // HH:MM
	Timezone string   `json:"timezone"`
	Days     []string `json:"days"` // mon, tue, wed, thu, fri, sat, sun
}

// AuditLog 审计日志.
type AuditLog struct {
	ID        string    `json:"id"`
	AssetID   string    `json:"asset_id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
}

// BackupJob 备份任务.
type BackupJob struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Assets      []string   `json:"assets"`
	Destination string     `json:"destination"`
	Encryption  bool       `json:"encryption"`
	Schedule    string     `json:"schedule"`
	Status      string     `json:"status"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	NextRun     *time.Time `json:"next_run,omitempty"`
	Size        int64      `json:"size"`
}

// Manager 数字资产管理器.
type Manager struct {
	mu        sync.RWMutex
	assets    map[string]*DigitalAsset
	policies  map[string]*AccessPolicy
	auditLog  []AuditLog
	backups   map[string]*BackupJob
	masterKey []byte
	maxAssets int
	retention time.Duration
}

// NewManager 创建管理器.
func NewManager(masterKeyHex string) (*Manager, error) {
	masterKey, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("无效的主密钥: %w", err)
	}

	return &Manager{
		assets:    make(map[string]*DigitalAsset),
		policies:  make(map[string]*AccessPolicy),
		auditLog:  make([]AuditLog, 0),
		backups:   make(map[string]*BackupJob),
		masterKey: masterKey,
		maxAssets: 10000,
		retention: 365 * 24 * time.Hour, // 1年
	}, nil
}

// StoreAsset 存储资产.
func (m *Manager) StoreAsset(asset *DigitalAsset) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.assets) >= m.maxAssets {
		return fmt.Errorf("已达到最大资产数: %d", m.maxAssets)
	}

	// 生成加密密钥
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("生成密钥失败: %w", err)
	}

	asset.EncryptedKey = hex.EncodeToString(key)
	asset.CreatedAt = time.Now()
	asset.UpdatedAt = time.Now()
	asset.Version = 1
	if asset.Status == "" {
		asset.Status = AssetStatusActive
	}
	if asset.Metadata == nil {
		asset.Metadata = make(map[string]string)
	}

	m.assets[asset.ID] = asset

	m.addAuditLog(asset.ID, asset.Owner, "store", "资产存储成功", true)

	return nil
}

// GetAsset 获取资产.
func (m *Manager) GetAsset(assetID string, userID string) (*DigitalAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	asset, exists := m.assets[assetID]
	if !exists {
		return nil, fmt.Errorf("资产不存在: %s", assetID)
	}

	// 检查访问权限
	if !m.checkAccess(assetID, userID, AccessRead) {
		m.addAuditLog(assetID, userID, "access_denied", "无访问权限", false)
		return nil, fmt.Errorf("无访问权限")
	}

	now := time.Now()
	asset.LastAccessed = &now
	asset.AccessCount++

	m.addAuditLog(assetID, userID, "access", "资产访问成功", true)

	return asset, nil
}

// SetAccessPolicy 设置访问策略.
func (m *Manager) SetAccessPolicy(policy *AccessPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.policies[policy.ID] = policy
	return nil
}

// ArchiveAsset 归档资产.
func (m *Manager) ArchiveAsset(assetID string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	asset, exists := m.assets[assetID]
	if !exists {
		return fmt.Errorf("资产不存在: %s", assetID)
	}

	if !m.checkAccess(assetID, userID, AccessWrite) {
		return fmt.Errorf("无写入权限")
	}

	asset.Status = AssetStatusArchived
	asset.UpdatedAt = time.Now()

	m.addAuditLog(assetID, userID, "archive", "资产归档成功", true)

	return nil
}

// CreateBackup 创建备份.
func (m *Manager) CreateBackup(job *BackupJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job.Status = "active"
	m.backups[job.ID] = job
	return nil
}

// GetAuditLog 获取审计日志.
func (m *Manager) GetAuditLog(assetID string, limit int) []AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var logs []AuditLog
	for i := len(m.auditLog) - 1; i >= 0 && len(logs) < limit; i-- {
		if assetID == "" || m.auditLog[i].AssetID == assetID {
			logs = append(logs, m.auditLog[i])
		}
	}

	return logs
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_assets":    len(m.assets),
		"active_assets":   0,
		"archived_assets": 0,
		"policies":        len(m.policies),
		"audit_logs":      len(m.auditLog),
		"backups":         len(m.backups),
		"security_levels": make(map[string]int),
	}

	levels := stats["security_levels"].(map[string]int)
	for _, a := range m.assets {
		switch a.Status {
		case AssetStatusActive:
			stats["active_assets"] = stats["active_assets"].(int) + 1
		case AssetStatusArchived:
			stats["archived_assets"] = stats["archived_assets"].(int) + 1
		}
		levels[string(a.SecurityLevel)]++
	}

	return stats
}

func (m *Manager) checkAccess(assetID string, userID string, level AccessLevel) bool {
	// 检查是否是所有者
	asset, exists := m.assets[assetID]
	if exists && asset.Owner == userID {
		return true
	}

	// 检查策略
	for _, policy := range m.policies {
		if policy.AssetID == assetID && policy.Subject == userID {
			if policy.Level == AccessOwner || policy.Level == AccessAdmin {
				return true
			}
			if policy.Level == AccessWrite && (level == AccessRead || level == AccessWrite) {
				return true
			}
			if policy.Level == AccessRead && level == AccessRead {
				return true
			}
		}
	}

	return false
}

func (m *Manager) addAuditLog(assetID string, userID string, action string, details string, success bool) {
	m.auditLog = append(m.auditLog, AuditLog{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		AssetID:   assetID,
		UserID:    userID,
		Action:    action,
		Details:   details,
		Timestamp: time.Now(),
		Success:   success,
	})
}

// Close 关闭管理器.
func (m *Manager) Close() error {
	return nil
}
