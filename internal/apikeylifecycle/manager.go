// Package apikeylifecycle 提供API密钥生命周期管理
package apikeylifecycle

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// 错误定义
var (
	ErrKeyNotFound      = errors.New("API密钥不存在")
	ErrKeyExpired       = errors.New("API密钥已过期")
	ErrKeyRevoked       = errors.New("API密钥已撤销")
	ErrKeyLimitExceeded = errors.New("API密钥数量超限")
	ErrInvalidInput     = errors.New("无效输入参数")
)

// KeyStatus 密钥状态
type KeyStatus string

const (
	StatusActive   KeyStatus = "active"   // 活跃
	StatusInactive KeyStatus = "inactive" // 未激活
	StatusExpired  KeyStatus = "expired"  // 已过期
	StatusRevoked  KeyStatus = "revoked"  // 已撤销
)

// Permission 权限类型
type Permission string

const (
	PermReadOnly  Permission = "read_only"  // 只读
	PermReadWrite Permission = "read_write" // 读写
	PermAdmin     Permission = "admin"      // 管理
	PermCustom    Permission = "custom"     // 自定义
)

// APIKey API密钥
type APIKey struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Description   string       `json:"description,omitempty"`
	KeyHash       string       `json:"key_hash"` // 哈希后的密钥
	KeyPrefix     string       `json:"key_prefix"` // 密钥前缀（用于显示）
	UserID        string       `json:"user_id"`
	UserName      string       `json:"user_name,omitempty"`
	Permissions   []Permission `json:"permissions"`
	Status        KeyStatus    `json:"status"`
	ExpiresAt     *time.Time   `json:"expires_at,omitempty"`
	LastUsedAt    *time.Time   `json:"last_used_at,omitempty"`
	LastUsedIP    string       `json:"last_used_ip,omitempty"`
	UsageCount    int64        `json:"usage_count"`
	RateLimit     int          `json:"rate_limit"` // 每分钟请求限制
	IPWhitelist   []string     `json:"ip_whitelist,omitempty"`
	Scopes        []string     `json:"scopes,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	RotatedAt     *time.Time   `json:"rotated_at,omitempty"`
	RotationCount int          `json:"rotation_count"`
}

// AuditEntry 审计记录
type AuditEntry struct {
	ID        string    `json:"id"`
	KeyID     string    `json:"key_id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"` // create, use, rotate, revoke, expire
	IP        string    `json:"ip,omitempty"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// RotationPolicy 轮换策略
type RotationPolicy struct {
	Enabled         bool  `json:"enabled"`
	RotationDays    int   `json:"rotation_days"`    // 自动轮换天数
	NotifyBeforeDays int  `json:"notify_before_days"` // 提前通知天数
	MaxRotations    int   `json:"max_rotations"`    // 最大轮换次数
	AutoExpire      bool  `json:"auto_expire"`      // 自动过期
}

// KeyStats 密钥统计
type KeyStats struct {
	TotalKeys      int   `json:"total_keys"`
	ActiveKeys     int   `json:"active_keys"`
	ExpiredKeys    int   `json:"expired_keys"`
	RevokedKeys    int   `json:"revoked_keys"`
	TotalUsage     int64 `json:"total_usage"`
	AvgUsagePerKey int64 `json:"avg_usage_per_key"`
}

// Manager API密钥生命周期管理器
type Manager struct {
	mu           sync.RWMutex
	keys         map[string]*APIKey
	auditLog     []*AuditEntry
	rotationPolicy *RotationPolicy
	maxKeysPerUser int
	startTime    time.Time
}

// NewManager 创建管理器
func NewManager(maxKeysPerUser int) *Manager {
	if maxKeysPerUser <= 0 {
		maxKeysPerUser = 10
	}

	return &Manager{
		keys:         make(map[string]*APIKey),
		auditLog:     make([]*AuditEntry, 0),
		rotationPolicy: &RotationPolicy{
			Enabled:          true,
			RotationDays:     90,
			NotifyBeforeDays: 7,
			MaxRotations:     12,
			AutoExpire:       true,
		},
		maxKeysPerUser: maxKeysPerUser,
		startTime:      time.Now(),
	}
}

// GenerateKey 生成API密钥
func GenerateKey() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}

	fullKey := hex.EncodeToString(bytes)
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	return fullKey, keyHash, nil
}

// CreateKey 创建API密钥
func (m *Manager) CreateKey(key *APIKey) (string, error) {
	if key == nil || key.Name == "" || key.UserID == "" {
		return "", ErrInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查用户密钥数量限制
	userKeyCount := 0
	for _, k := range m.keys {
		if k.UserID == key.UserID && k.Status == StatusActive {
			userKeyCount++
		}
	}

	if userKeyCount >= m.maxKeysPerUser {
		return "", ErrKeyLimitExceeded
	}

	// 生成密钥
	fullKey, keyHash, err := GenerateKey()
	if err != nil {
		return "", err
	}

	key.ID = fmt.Sprintf("key-%d", time.Now().UnixNano())
	key.KeyHash = keyHash
	key.KeyPrefix = fullKey[:8] + "..."
	key.Status = StatusActive
	key.CreatedAt = time.Now()
	key.UpdatedAt = time.Now()

	m.keys[key.ID] = key

	// 记录审计日志
	m.addAudit(key.ID, key.UserID, "create", "", "API密钥创建成功")

	return fullKey, nil
}

// GetKey 获取密钥信息
func (m *Manager) GetKey(keyID string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, exists := m.keys[keyID]
	if !exists {
		return nil, ErrKeyNotFound
	}
	return key, nil
}

// ListKeys 列出用户密钥
func (m *Manager) ListKeys(userID string) []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*APIKey, 0)
	for _, key := range m.keys {
		if userID == "" || key.UserID == userID {
			result = append(result, key)
		}
	}
	return result
}

// RevokeKey 撤销密钥
func (m *Manager) RevokeKey(keyID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return ErrKeyNotFound
	}

	if key.UserID != userID {
		return errors.New("无权撤销此密钥")
	}

	key.Status = StatusRevoked
	key.UpdatedAt = time.Now()

	m.addAudit(keyID, userID, "revoke", "", "API密钥已撤销")

	return nil
}

// RotateKey 轮换密钥
func (m *Manager) RotateKey(keyID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return "", ErrKeyNotFound
	}

	if key.Status != StatusActive {
		return "", fmt.Errorf("密钥状态 %s 不允许轮换", key.Status)
	}

	// 生成新密钥
	fullKey, keyHash, err := GenerateKey()
	if err != nil {
		return "", err
	}

	key.KeyHash = keyHash
	key.KeyPrefix = fullKey[:8] + "..."
	now := time.Now()
	key.RotatedAt = &now
	key.RotationCount++
	key.UpdatedAt = now

	m.addAudit(keyID, key.UserID, "rotate", "", fmt.Sprintf("密钥已轮换，第 %d 次", key.RotationCount))

	return fullKey, nil
}

// ValidateKey 验证密钥
func (m *Manager) ValidateKey(keyStr string, ip string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hash := sha256.Sum256([]byte(keyStr))
	keyHash := hex.EncodeToString(hash[:])

	for _, key := range m.keys {
		if key.KeyHash == keyHash {
			// 检查状态
			switch key.Status {
			case StatusRevoked:
				return nil, ErrKeyRevoked
			case StatusExpired:
				return nil, ErrKeyExpired
			case StatusInactive:
				return nil, errors.New("密钥未激活")
			}

			// 检查过期
			if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
				key.Status = StatusExpired
				return nil, ErrKeyExpired
			}

			// 检查IP白名单
			if len(key.IPWhitelist) > 0 && ip != "" {
				allowed := false
				for _, allowedIP := range key.IPWhitelist {
					if allowedIP == ip {
						allowed = true
						break
					}
				}
				if !allowed {
					return nil, errors.New("IP不在白名单中")
				}
			}

			// 更新使用统计
			now := time.Now()
			key.LastUsedAt = &now
			key.LastUsedIP = ip
			key.UsageCount++

			return key, nil
		}
	}

	return nil, ErrKeyNotFound
}

// CheckRotation 检查是否需要轮换
func (m *Manager) CheckRotation() []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	needRotation := make([]*APIKey, 0)

	if !m.rotationPolicy.Enabled {
		return needRotation
	}

	for _, key := range m.keys {
		if key.Status != StatusActive {
			continue
		}

		// 检查轮换次数限制
		if m.rotationPolicy.MaxRotations > 0 && key.RotationCount >= m.rotationPolicy.MaxRotations {
			continue
		}

		// 检查是否需要轮换
		daysSinceCreation := int(time.Since(key.CreatedAt).Hours() / 24)
		if key.RotatedAt != nil {
			daysSinceCreation = int(time.Since(*key.RotatedAt).Hours() / 24)
		}

		if daysSinceCreation >= m.rotationPolicy.RotationDays {
			needRotation = append(needRotation, key)
		}
	}

	return needRotation
}

// ExpireKeys 过期密钥
func (m *Manager) ExpireKeys() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	expired := 0
	for _, key := range m.keys {
		if key.Status == StatusActive && key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
			key.Status = StatusExpired
			key.UpdatedAt = time.Now()
			expired++

			m.addAudit(key.ID, key.UserID, "expire", "", "密钥已自动过期")
		}
	}

	return expired
}

// UpdateRotationPolicy 更新轮换策略
func (m *Manager) UpdateRotationPolicy(policy *RotationPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rotationPolicy = policy
}

// GetRotationPolicy 获取轮换策略
func (m *Manager) GetRotationPolicy() *RotationPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rotationPolicy
}

// GetStats 获取统计信息
func (m *Manager) GetStats() *KeyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &KeyStats{
		TotalKeys: len(m.keys),
	}

	for _, key := range m.keys {
		stats.TotalUsage += key.UsageCount
		switch key.Status {
		case StatusActive:
			stats.ActiveKeys++
		case StatusExpired:
			stats.ExpiredKeys++
		case StatusRevoked:
			stats.RevokedKeys++
		}
	}

	if stats.TotalKeys > 0 {
		stats.AvgUsagePerKey = stats.TotalUsage / int64(stats.TotalKeys)
	}

	return stats
}

// GetAuditLog 获取审计日志
func (m *Manager) GetAuditLog(keyID string, limit int) []*AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AuditEntry, 0)
	for i := len(m.auditLog) - 1; i >= 0; i-- {
		if keyID == "" || m.auditLog[i].KeyID == keyID {
			result = append(result, m.auditLog[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// addAudit 添加审计记录
func (m *Manager) addAudit(keyID, userID, action, ip, details string) {
	entry := &AuditEntry{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		KeyID:     keyID,
		UserID:    userID,
		Action:    action,
		IP:        ip,
		Details:   details,
		Timestamp: time.Now(),
	}
	m.auditLog = append(m.auditLog, entry)
}

// UpdateKeyPermissions 更新密钥权限
func (m *Manager) UpdateKeyPermissions(keyID string, permissions []Permission) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return ErrKeyNotFound
	}

	key.Permissions = permissions
	key.UpdatedAt = time.Now()

	return nil
}

// SetKeyExpiration 设置密钥过期时间
func (m *Manager) SetKeyExpiration(keyID string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return ErrKeyNotFound
	}

	key.ExpiresAt = &expiresAt
	key.UpdatedAt = time.Now()

	return nil
}

// GetExpiringKeys 获取即将过期的密钥
func (m *Manager) GetExpiringKeys(days int) []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*APIKey, 0)
	threshold := time.Now().AddDate(0, 0, days)

	for _, key := range m.keys {
		if key.Status == StatusActive && key.ExpiresAt != nil && key.ExpiresAt.Before(threshold) {
			result = append(result, key)
		}
	}

	return result
}
