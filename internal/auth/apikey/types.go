// Package apikey 提供用户 API 密钥管理功能
// 对标 TrueNAS 25.04 STIG V-253800 要求：API 密钥安全存储、访问控制和审计
package apikey

import (
	"time"
)

// APIKey API 密钥结构
type APIKey struct {
	ID          string     `json:"id"`           // 密钥唯一标识
	Name        string     `json:"name"`         // 密钥名称（用户可读）
	KeyHash     string     `json:"key_hash"`     // 密钥哈希（SHA-256，不存储原始密钥）
	KeyPrefix   string     `json:"key_prefix"`   // 密钥前缀（用于识别，如 "nas_"）
	UserID      string     `json:"user_id"`      // 所属用户 ID
	Permissions []string   `json:"permissions"`  // 权限范围
	Scopes      []Scope    `json:"scopes"`       // API 访问范围
	RateLimit   int        `json:"rate_limit"`   // 每分钟请求限制
	ExpiresAt   *time.Time `json:"expires_at"`   // 过期时间（可选）
	CreatedAt   time.Time  `json:"created_at"`   // 创建时间
	UpdatedAt   *time.Time `json:"updated_at"`   // 更新时间
	LastUsedAt  *time.Time `json:"last_used_at"` // 最后使用时间
	UsedCount   int64      `json:"used_count"`   // 使用次数统计
	Enabled     bool       `json:"enabled"`      // 是否启用
	Description string     `json:"description"`  // 密钥描述
	SourceIPs   []string   `json:"source_ips"`   // 允许的源 IP（可选，空表示不限制）
}

// Scope API 访问范围
type Scope struct {
	Resource string   `json:"resource"` // 资源类型：storage, user, system, container, vm
	Actions  []string `json:"actions"`  // 操作：read, write, delete, admin
}

// APIKeyCreateRequest 创建密钥请求
type APIKeyCreateRequest struct {
	Name        string     `json:"name" binding:"required,min=3,max=64"`
	Permissions []string   `json:"permissions"`
	Scopes      []Scope    `json:"scopes"`
	RateLimit   int        `json:"rate_limit"` // 默认 60
	ExpiresAt   *time.Time `json:"expires_at"`
	Description string     `json:"description"`
	SourceIPs   []string   `json:"source_ips"` // CIDR 格式
}

// APIKeyCreateResponse 创建密钥响应
type APIKeyCreateResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Key       string     `json:"key"` // 仅在创建时返回原始密钥
	KeyPrefix string     `json:"key_prefix"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	Warning   string     `json:"warning"` // 安全警告
}

// APIKeyListResponse 密钥列表响应
type APIKeyListResponse struct {
	Keys  []APIKeySummary `json:"keys"`
	Total int             `json:"total"`
}

// APIKeySummary 密钥摘要（不含敏感信息）
type APIKeySummary struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	KeyPrefix   string     `json:"key_prefix"`
	Permissions []string   `json:"permissions"`
	Scopes      []Scope    `json:"scopes"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	UsedCount   int64      `json:"used_count"`
	Enabled     bool       `json:"enabled"`
	IsExpired   bool       `json:"is_expired"`
}

// APIKeyUpdateRequest 更新密钥请求
type APIKeyUpdateRequest struct {
	Name        *string    `json:"name"`
	Permissions []string   `json:"permissions"`
	Scopes      []Scope    `json:"scopes"`
	RateLimit   *int       `json:"rate_limit"`
	ExpiresAt   *time.Time `json:"expires_at"`
	Enabled     *bool      `json:"enabled"`
	Description *string    `json:"description"`
	SourceIPs   []string   `json:"source_ips"`
}

// APIKeyUsage 使用记录
type APIKeyUsage struct {
	KeyID      string    `json:"key_id"`
	Timestamp  time.Time `json:"timestamp"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	SourceIP   string    `json:"source_ip"`
	StatusCode int       `json:"status_code"`
	ResponseMs int       `json:"response_ms"`
}

// APIKeyPolicy 密钥策略（STIG 要求）
type APIKeyPolicy struct {
	MinKeyLength       int  `json:"min_key_length"`       // 最小密钥长度，默认 32
	MaxKeysPerUser     int  `json:"max_keys_per_user"`    // 每用户最大密钥数，默认 10
	DefaultRateLimit   int  `json:"default_rate_limit"`   // 默认速率限制，默认 60/分钟
	MaxExpiryDays      int  `json:"max_expiry_days"`      // 最大有效期天数，默认 365
	RequireScope       bool `json:"require_scope"`        // 是否必须指定范围
	RequireSourceIP    bool `json:"require_source_ip"`    // 是否必须指定源 IP
	EnableAudit        bool `json:"enable_audit"`         // 启用审计日志
	AutoDisableExpired bool `json:"auto_disable_expired"` // 自动禁用过期密钥
}

// DefaultAPIKeyPolicy 默认密钥策略（符合 STIG 要求）
var DefaultAPIKeyPolicy = APIKeyPolicy{
	MinKeyLength:       32,
	MaxKeysPerUser:     10,
	DefaultRateLimit:   60,
	MaxExpiryDays:      365,
	RequireScope:       true,
	RequireSourceIP:    false, // 可选
	EnableAudit:        true,
	AutoDisableExpired: true,
}

// 错误定义
var (
	ErrKeyNotFound       = "API 密钥不存在"
	ErrKeyExpired        = "API 密钥已过期"
	ErrKeyDisabled       = "API 密钥已禁用"
	ErrKeyInvalid        = "API 密钥无效"
	ErrRateLimitExceeded = "请求速率超出限制"
	ErrIPNotAllowed      = "源 IP 不在允许列表中"
	ErrPermissionDenied  = "权限不足"
	ErrMaxKeysExceeded   = "已达到最大密钥数量限制"
	ErrKeyFormatInvalid  = "密钥格式无效"
)
