// Package userapikey 提供用户关联 API Key 管理功能。
// 支持创建、撤销、轮换 API Key，设置过期时间，以及权限绑定。
// 参考 TrueNAS 的用户关联 API Key（可过期/可撤销）设计。
package userapikey

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// 常见错误
var (
	ErrKeyNotFound      = errors.New("api key not found")
	ErrKeyRevoked       = errors.New("api key already revoked")
	ErrKeyExpired       = errors.New("api key expired")
	ErrPermissionDenied = errors.New("permission denied")
	ErrInvalidInput     = errors.New("invalid input")
)

// KeyStatus API Key 状态
type KeyStatus string

const (
	KeyStatusActive  KeyStatus = "active"
	KeyStatusRevoked KeyStatus = "revoked"
	KeyStatusExpired KeyStatus = "expired"
)

// Permission API Key 权限
type Permission struct {
	Resource string   `json:"resource"` // 资源标识，如 "storage:pool0" 或 "system:*"
	Actions  []string `json:"actions"`  // 允许的操作，如 ["read", "write", "delete"]
}

// APIKey 用户关联的 API Key
type APIKey struct {
	ID          string       `json:"id"`
	UserID      string       `json:"user_id"`
	Name        string       `json:"name"`
	KeyHash     string       `json:"-"`      // 仅存储哈希，不暴露原始 Key
	Prefix      string       `json:"prefix"` // Key 前缀，用于识别（如 "nas_k3x..."）
	Status      KeyStatus    `json:"status"`
	Permissions []Permission `json:"permissions"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	RevokedAt   *time.Time   `json:"revoked_at,omitempty"`
	LastUsedAt  *time.Time   `json:"last_used_at,omitempty"`
	Description string       `json:"description,omitempty"`
}

// CreateKeyRequest 创建 API Key 请求
type CreateKeyRequest struct {
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description,omitempty"`
	Permissions []Permission `json:"permissions"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
}

// RotateKeyResult 轮换 API Key 结果
type RotateKeyResult struct {
	ID     string `json:"id"`
	NewKey string `json:"new_key"` // 仅在创建/轮换时返回完整 Key
	Prefix string `json:"prefix"`
}

// ListKeysOptions 列出 API Key 的查询选项
type ListKeysOptions struct {
	Status *KeyStatus `json:"status,omitempty"`
	Offset int        `json:"offset,omitempty"`
	Limit  int        `json:"limit,omitempty"`
}

// Manager API Key 管理器
type Manager struct {
	// TODO: 替换为持久化存储
	keys map[string]*APIKey // id -> key
}

// NewManager 创建 API Key 管理器
func NewManager() *Manager {
	return &Manager{
		keys: make(map[string]*APIKey),
	}
}

// generateKey 生成随机 API Key，返回 (rawKey, hash, prefix, error)
func generateKey() (string, string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", "", err
	}
	encoded := hex.EncodeToString(raw)
	// 格式: nas_<前6字符>...<后4字符>
	prefix := "nas_" + encoded[:6]
	// TODO: 使用真实哈希算法（如 SHA-256）存储 Key
	hash := encoded // 暂存完整值用于演示
	return encoded, hash, prefix, nil
}

// CreateKey 为用户创建 API Key，返回原始 Key（仅此一次可见）
func (m *Manager) CreateKey(userID string, req *CreateKeyRequest) (*RotateKeyResult, error) {
	if userID == "" || req.Name == "" {
		return nil, ErrInvalidInput
	}

	rawKey, hash, prefix, err := generateKey()
	if err != nil {
		return nil, err
	}

	// TODO: 使用 uuid 生成 ID
	id := prefix + "_" + hex.EncodeToString([]byte(userID))[:8]

	now := time.Now()
	key := &APIKey{
		ID:          id,
		UserID:      userID,
		Name:        req.Name,
		KeyHash:     hash,
		Prefix:      prefix,
		Status:      KeyStatusActive,
		Permissions: req.Permissions,
		ExpiresAt:   req.ExpiresAt,
		CreatedAt:   now,
		Description: req.Description,
	}

	m.keys[id] = key

	return &RotateKeyResult{
		ID:     id,
		NewKey: rawKey,
		Prefix: prefix,
	}, nil
}

// RevokeKey 撤销 API Key
func (m *Manager) RevokeKey(userID, keyID string) error {
	key, ok := m.keys[keyID]
	if !ok {
		return ErrKeyNotFound
	}
	if key.UserID != userID {
		return ErrPermissionDenied
	}
	if key.Status == KeyStatusRevoked {
		return ErrKeyRevoked
	}

	now := time.Now()
	key.Status = KeyStatusRevoked
	key.RevokedAt = &now

	// TODO: 通知相关服务 Key 已撤销
	return nil
}

// RotateKey 轮换 API Key（撤销旧 Key，创建新 Key）
func (m *Manager) RotateKey(userID, keyID string) (*RotateKeyResult, error) {
	if err := m.RevokeKey(userID, keyID); err != nil {
		return nil, err
	}

	oldKey := m.keys[keyID]
	req := &CreateKeyRequest{
		Name:        oldKey.Name,
		Description: oldKey.Description,
		Permissions: oldKey.Permissions,
		ExpiresAt:   oldKey.ExpiresAt,
	}

	return m.CreateKey(userID, req)
}

// ValidateKey 验证 API Key，返回关联的 APIKey 信息
func (m *Manager) ValidateKey(rawKey string) (*APIKey, error) {
	// TODO: 使用真实哈希比对
	for _, key := range m.keys {
		if key.KeyHash == rawKey {
			if key.Status == KeyStatusRevoked {
				return nil, ErrKeyRevoked
			}
			if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
				key.Status = KeyStatusExpired
				return nil, ErrKeyExpired
			}
			now := time.Now()
			key.LastUsedAt = &now
			return key, nil
		}
	}
	return nil, ErrKeyNotFound
}

// ListKeys 列出用户的 API Key
func (m *Manager) ListKeys(userID string, opts *ListKeysOptions) []*APIKey {
	var result []*APIKey
	for _, key := range m.keys {
		if key.UserID != userID {
			continue
		}
		if opts != nil && opts.Status != nil && key.Status != *opts.Status {
			continue
		}
		result = append(result, key)
	}
	return result
}

// GetKey 获取单个 API Key 详情
func (m *Manager) GetKey(userID, keyID string) (*APIKey, error) {
	key, ok := m.keys[keyID]
	if !ok {
		return nil, ErrKeyNotFound
	}
	if key.UserID != userID {
		return nil, ErrPermissionDenied
	}
	return key, nil
}
