// Package apikeymgr 用户API Key管理
// 支持创建、轮换、吊销用户级别的API密钥，对标TrueNAS User-linked API Keys。
// 支持细粒度权限控制、使用统计和自动过期。
package apikeymgr

import (
	"errors"
	"sync"
	"time"
)

// KeyStatus 密钥状态
type KeyStatus string

const (
	StatusActive  KeyStatus = "active"
	StatusExpired KeyStatus = "expired"
	StatusRevoked KeyStatus = "revoked"
)

// Permission 权限
type Permission string

const (
	PermRead     Permission = "read"
	PermWrite    Permission = "write"
	PermDelete   Permission = "delete"
	PermAdmin    Permission = "admin"
	PermShare    Permission = "share"
	PermDownload Permission = "download"
)

// APIKey API密钥
type APIKey struct {
	ID          string            `json:"id"`
	UserID      string            `json:"user_id"`
	Name        string            `json:"name"`
	KeyHash     string            `json:"key_hash"`
	Prefix      string            `json:"prefix"` // 密钥前缀，用于识别
	Permissions []Permission      `json:"permissions"`
	Status      KeyStatus         `json:"status"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
	UsageCount  int64             `json:"usage_count"`
	RateLimit   int               `json:"rate_limit"` // 每分钟请求限制
	CreatedAt   time.Time         `json:"created_at"`
	RevokedAt   *time.Time        `json:"revoked_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// CreateKeyRequest 创建密钥请求
type CreateKeyRequest struct {
	UserID      string       `json:"user_id"`
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
	ExpiresIn   int          `json:"expires_in_days"` // 0=永不过期
	RateLimit   int          `json:"rate_limit"`
}

// KeyUsageStats 使用统计
type KeyUsageStats struct {
	KeyID      string     `json:"key_id"`
	TotalCalls int64      `json:"total_calls"`
	Last24h    int64      `json:"last_24h_calls"`
	LastUsedAt *time.Time `json:"last_used_at"`
	AvgLatency float64    `json:"avg_latency_ms"`
}

// APIKeyManager API Key管理器
type APIKeyManager struct {
	mu     sync.RWMutex
	keys   map[string]*APIKey
	config ManagerConfig
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	MaxKeysPerUser   int    `json:"max_keys_per_user"`
	DefaultRateLimit int    `json:"default_rate_limit"`
	KeyLength        int    `json:"key_length"`
	HashAlgorithm    string `json:"hash_algorithm"`
	AllowExpired     bool   `json:"allow_expired"`
}

// DefaultManagerConfig 默认配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxKeysPerUser:   10,
		DefaultRateLimit: 60,
		KeyLength:        32,
		HashAlgorithm:    "sha256",
		AllowExpired:     false,
	}
}

// 预定义错误
var (
	ErrKeyNotFound    = errors.New("API key not found")
	ErrKeyRevoked     = errors.New("API key is revoked")
	ErrKeyExpired     = errors.New("API key is expired")
	ErrMaxKeysReached = errors.New("max keys per user reached")
	ErrInvalidPerms   = errors.New("invalid permissions")
	ErrNameRequired   = errors.New("key name is required")
	ErrUserIDRequired = errors.New("user ID is required")
)
