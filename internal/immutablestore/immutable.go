// Package immutablestore 不可变存储模块
// 实现 WORM (Write Once Read Many) 机制，防止数据被篡改或删除
// 参考: 群晖 DSM 7.2 不可变存储与备份
package immutablestore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// LockMode 锁定模式
type LockMode string

const (
	LockModeCompliance LockMode = "compliance" // 法规模式 - 固定期限锁定
	LockModeEnterprise LockMode = "enterprise" // 企业模式 - 长期禁用修改
	LockModeLegal      LockMode = "legal"      // 法律模式 - 法律保留
)

// ImmutablePolicy 不可变策略
type ImmutablePolicy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Mode        LockMode  `json:"mode"`
	Duration    int       `json:"duration_hours"` // 锁定时长（小时），0=永久
	PathPattern string    `json:"path_pattern"`   // 路径模式
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
}

// ImmutableFile 不可变文件记录
type ImmutableFile struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"file_path"`
	SHA256      string    `json:"sha256"`
	Size        int64     `json:"size"`
	PolicyID    string    `json:"policy_id"`
	LockedAt    time.Time `json:"locked_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsPermanent bool      `json:"is_permanent"`
	LockedBy    string    `json:"locked_by"`
	Status      string    `json:"status"` // locked, expired, released
}

// AuditEntry 审计记录
type AuditEntry struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	FilePath  string    `json:"file_path"`
	UserID    string    `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details"`
	Success   bool      `json:"success"`
}

// ImmutableStorageManager 不可变存储管理器
type ImmutableStorageManager struct {
	mu       sync.RWMutex
	policies map[string]*ImmutablePolicy
	files    map[string]*ImmutableFile
	auditLog []AuditEntry
	config   *StorageConfig
}

// StorageConfig 存储配置
type StorageConfig struct {
	MaxPolicies     int  `json:"max_policies"`
	MaxFiles        int  `json:"max_files"`
	RetentionDays   int  `json:"retention_days"`
	EnableAuditLog  bool `json:"enable_audit_log"`
	RequireApproval bool `json:"require_approval"`
}

// NewImmutableStorageManager 创建不可变存储管理器
func NewImmutableStorageManager(config *StorageConfig) *ImmutableStorageManager {
	if config == nil {
		config = &StorageConfig{
			MaxPolicies:    100,
			MaxFiles:       100000,
			RetentionDays:  365,
			EnableAuditLog: true,
		}
	}
	return &ImmutableStorageManager{
		policies: make(map[string]*ImmutablePolicy),
		files:    make(map[string]*ImmutableFile),
		auditLog: make([]AuditEntry, 0),
		config:   config,
	}
}

// CreatePolicy 创建不可变策略
func (m *ImmutableStorageManager) CreatePolicy(policy *ImmutablePolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.policies) >= m.config.MaxPolicies {
		return fmt.Errorf("max policies reached (%d)", m.config.MaxPolicies)
	}

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("policy_%d", time.Now().UnixNano())
	}
	policy.CreatedAt = time.Now()

	m.policies[policy.ID] = policy
	m.addAuditEntry("create_policy", policy.ID, policy.CreatedBy, "Created policy: "+policy.Name, true)

	return nil
}

// LockFile 锁定文件为不可变
func (m *ImmutableStorageManager) LockFile(filePath, policyID, userID string) (*ImmutableFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[policyID]
	if !exists {
		return nil, fmt.Errorf("policy not found: %s", policyID)
	}

	if !policy.Enabled {
		return nil, fmt.Errorf("policy is disabled: %s", policyID)
	}

	// 检查文件是否已被锁定
	for _, f := range m.files {
		if f.FilePath == filePath && f.Status == "locked" {
			return nil, fmt.Errorf("file already locked: %s", filePath)
		}
	}

	// 计算文件哈希（模拟）
	hash := sha256.Sum256([]byte(filePath + time.Now().String()))

	file := &ImmutableFile{
		ID:          fmt.Sprintf("immut_%d", time.Now().UnixNano()),
		FilePath:    filePath,
		SHA256:      hex.EncodeToString(hash[:]),
		PolicyID:    policyID,
		LockedAt:    time.Now(),
		LockedBy:    userID,
		Status:      "locked",
		IsPermanent: policy.Duration == 0,
	}

	if policy.Duration > 0 {
		expiry := time.Now().Add(time.Duration(policy.Duration) * time.Hour)
		file.ExpiresAt = &expiry
	}

	m.files[file.ID] = file
	m.addAuditEntry("lock_file", filePath, userID, "Locked with policy: "+policy.Name, true)

	return file, nil
}

// VerifyIntegrity 验证文件完整性
func (m *ImmutableStorageManager) VerifyIntegrity(fileID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	file, exists := m.files[fileID]
	if !exists {
		return false, fmt.Errorf("file not found: %s", fileID)
	}

	if file.Status != "locked" {
		return false, fmt.Errorf("file is not locked: %s", fileID)
	}

	// 模拟重新计算哈希并比较
	// 在实际实现中，这里会读取文件并重新计算 SHA256
	currentHash := file.SHA256 // 模拟相同

	return currentHash == file.SHA256, nil
}

// ListPolicies 列出所有策略
func (m *ImmutableStorageManager) ListPolicies() []*ImmutablePolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*ImmutablePolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// ListLockedFiles 列出所有锁定文件
func (m *ImmutableStorageManager) ListLockedFiles() []*ImmutableFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := make([]*ImmutableFile, 0, len(m.files))
	for _, f := range m.files {
		if f.Status == "locked" {
			files = append(files, f)
		}
	}
	return files
}

// GetAuditLog 获取审计日志
func (m *ImmutableStorageManager) GetAuditLog(limit int) []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLog) {
		limit = len(m.auditLog)
	}

	// 返回最新的记录
	start := len(m.auditLog) - limit
	if start < 0 {
		start = 0
	}
	return m.auditLog[start:]
}

// addAuditEntry 添加审计记录
func (m *ImmutableStorageManager) addAuditEntry(action, filePath, userID, details string, success bool) {
	if !m.config.EnableAuditLog {
		return
	}

	entry := AuditEntry{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Action:    action,
		FilePath:  filePath,
		UserID:    userID,
		Timestamp: time.Now(),
		Details:   details,
		Success:   success,
	}
	m.auditLog = append(m.auditLog, entry)

	// 限制日志大小
	if len(m.auditLog) > 10000 {
		m.auditLog = m.auditLog[1000:]
	}
}

// GetStats 获取统计信息
func (m *ImmutableStorageManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lockedCount := 0
	for _, f := range m.files {
		if f.Status == "locked" {
			lockedCount++
		}
	}

	return map[string]interface{}{
		"total_policies":  len(m.policies),
		"total_files":     len(m.files),
		"locked_files":    lockedCount,
		"audit_log_count": len(m.auditLog),
	}
}
