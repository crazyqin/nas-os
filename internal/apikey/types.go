package apikey

import (
	"time"
)

// APIKey API密钥
type APIKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`         // 密钥名称
	Description string    `json:"description"`  // 描述
	Key         string    `json:"key"`          // 密钥值（仅创建时返回）
	KeyPrefix   string    `json:"key_prefix"`   // 密钥前缀（用于显示）
	UserID      string    `json:"user_id"`      // 所属用户
	Permissions []string  `json:"permissions"`  // 权限列表
	Scopes      []string  `json:"scopes"`       // 作用域
	ExpiresAt   *time.Time `json:"expires_at"`  // 过期时间
	LastUsedAt  *time.Time `json:"last_used_at"` // 最后使用时间
	UsageCount  int64     `json:"usage_count"`   // 使用次数
	Status      KeyStatus `json:"status"`        // 状态
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"` // 撤销时间
	RevokedBy   string    `json:"revoked_by,omitempty"` // 撤销者
	RevokedReason string  `json:"revoked_reason,omitempty"` // 撤销原因
}

// KeyStatus 密钥状态
type KeyStatus string

const (
	StatusActive   KeyStatus = "active"   // 活跃
	StatusExpired  KeyStatus = "expired"  // 已过期
	StatusRevoked  KeyStatus = "revoked"  // 已撤销
	StatusDisabled KeyStatus = "disabled" // 已禁用
)

// Permission 权限定义
type Permission struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"` // 资源类型
	Actions     []string `json:"actions"` // 允许的操作
}

// Scope 作用域定义
type Scope struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Resources   []string `json:"resources"` // 可访问的资源
}

// CreateKeyRequest 创建密钥请求
type CreateKeyRequest struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	UserID      string    `json:"user_id" binding:"required"`
	Permissions []string  `json:"permissions"`
	Scopes      []string  `json:"scopes"`
	ExpiresIn   int       `json:"expires_in"` // 过期时间（小时），0表示永不过期
}

// UpdateKeyRequest 更新密钥请求
type UpdateKeyRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	Scopes      []string `json:"scopes"`
	Status      KeyStatus `json:"status"`
}

// RevokeKeyRequest 撤销密钥请求
type RevokeKeyRequest struct {
	Reason string `json:"reason"` // 撤销原因
}

// ListKeysRequest 列表请求
type ListKeysRequest struct {
	UserID string    `json:"user_id,omitempty"` // 按用户筛选
	Status KeyStatus `json:"status,omitempty"`  // 按状态筛选
	Page   int       `json:"page,omitempty"`
	PageSize int     `json:"page_size,omitempty"`
}

// ListKeysResponse 列表响应
type ListKeysResponse struct {
	Keys     []*APIKey `json:"keys"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// ValidateKeyRequest 验证密钥请求
type ValidateKeyRequest struct {
	Key      string   `json:"key" binding:"required"`
	Resource string   `json:"resource,omitempty"` // 请求的资源
	Action   string   `json:"action,omitempty"`   // 请求的操作
}

// ValidateKeyResponse 验证密钥响应
type ValidateKeyResponse struct {
	Valid     bool     `json:"valid"`
	KeyID     string   `json:"key_id,omitempty"`
	UserID    string   `json:"user_id,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// KeyStats 密钥统计
type KeyStats struct {
	TotalKeys    int `json:"total_keys"`
	ActiveKeys   int `json:"active_keys"`
	ExpiredKeys  int `json:"expired_keys"`
	RevokedKeys  int `json:"revoked_keys"`
	DisabledKeys int `json:"disabled_keys"`
	TotalUsage   int64 `json:"total_usage"`
}

// UserKeyStats 用户密钥统计
type UserKeyStats struct {
	UserID     string `json:"user_id"`
	TotalKeys  int    `json:"total_keys"`
	ActiveKeys int    `json:"active_keys"`
	TotalUsage int64  `json:"total_usage"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	KeyID     string    `json:"key_id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`    // create, use, revoke, update
	Resource  string    `json:"resource,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Details   string    `json:"details,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// APIResponse 通用API响应
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}