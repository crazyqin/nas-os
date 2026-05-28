// Package secretmgr 提供统一密钥/凭据管理
package secretmgr

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrSecretNotFound 密钥不存在.
	ErrSecretNotFound = errors.New("密钥不存在")
	// ErrSecretAlreadyExists 密钥已存在.
	ErrSecretAlreadyExists = errors.New("密钥已存在")
	// ErrAccessDenied 访问被拒绝.
	ErrAccessDenied = errors.New("访问被拒绝")
	// ErrSecretExpired 密钥已过期.
	ErrSecretExpired = errors.New("密钥已过期")
)

// ========== 核心类型 ==========

// SecretType 密钥类型.
type SecretType string

const (
	// SecretTypePassword 密码.
	SecretTypePassword SecretType = "password"
	// SecretTypeAPIKey API密钥.
	SecretTypeAPIKey SecretType = "api_key"
	// SecretTypeToken 令牌.
	SecretTypeToken SecretType = "token"
	// SecretTypeCertificate 证书.
	SecretTypeCertificate SecretType = "certificate"
	// SecretTypeSSHKey SSH密钥.
	SecretTypeSSHKey SecretType = "ssh_key"
	// SecretTypeGeneric 通用.
	SecretTypeGeneric SecretType = "generic"
)

// SecretStatus 密钥状态.
type SecretStatus string

const (
	// SecretStatusActive 活跃.
	SecretStatusActive SecretStatus = "active"
	// SecretStatusExpired 已过期.
	SecretStatusExpired SecretStatus = "expired"
	// SecretStatusRevoked 已撤销.
	SecretStatusRevoked SecretStatus = "revoked"
)

// ========== 数据结构 ==========

// Secret 密钥.
type Secret struct {
	ID          string       `json:"id"`           // 密钥ID
	Name        string       `json:"name"`         // 密钥名称
	Type        SecretType   `json:"type"`         // 密钥类型
	Description string       `json:"description"`  // 描述
	Value       string       `json:"value"`        // 密钥值（加密存储）
	Metadata    map[string]string `json:"metadata"` // 元数据
	Tags        []string     `json:"tags"`         // 标签
	Status      SecretStatus `json:"status"`       // 状态
	ExpiresAt   *time.Time   `json:"expires_at"`   // 过期时间
	LastUsed    *time.Time   `json:"last_used"`    // 最后使用
	RotateDays  int          `json:"rotate_days"`  // 轮换天数
	Version     int          `json:"version"`      // 版本号
	CreatedAt   time.Time    `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time    `json:"updated_at"`   // 更新时间
}

// SecretVersion 密钥版本.
type SecretVersion struct {
	Version   int       `json:"version"`    // 版本号
	Value     string    `json:"value"`      // 密钥值
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// AccessLog 访问日志.
type AccessLog struct {
	SecretID  string    `json:"secret_id"`  // 密钥ID
	Action    string    `json:"action"`     // 操作
	User      string    `json:"user"`       // 用户
	IP        string    `json:"ip"`         // IP地址
	Timestamp time.Time `json:"timestamp"`  // 时间戳
}

// SecretStats 密钥统计.
type SecretStats struct {
	TotalSecrets   int64 `json:"total_secrets"`   // 总密钥数
	ActiveSecrets  int64 `json:"active_secrets"`  // 活跃密钥数
	ExpiredSecrets int64 `json:"expired_secrets"` // 过期密钥数
	TotalAccess    int64 `json:"total_access"`    // 总访问次数
}

// CreateSecretRequest 创建密钥请求.
type CreateSecretRequest struct {
	Name        string            `json:"name" binding:"required"`
	Type        SecretType        `json:"type"`
	Description string            `json:"description"`
	Value       string            `json:"value" binding:"required"`
	Metadata    map[string]string `json:"metadata"`
	Tags        []string          `json:"tags"`
	ExpiresAt   *time.Time        `json:"expires_at"`
	RotateDays  int               `json:"rotate_days"`
}
