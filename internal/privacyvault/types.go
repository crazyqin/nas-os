// Package privacyvault 实现隐私保险箱功能，提供端到端加密、零知识架构、
// 安全文件分享、密钥分片、自动锁定和安全审计能力。
// 参考群晖 DSM 的加密保险库设计，支持 AES-256-GCM 加密与 PBKDF2 密钥派生。
package privacyvault

import (
	"fmt"
	"time"
)

// ============================================================
// 加密算法常量
// ============================================================

// EncryptionAlgorithm 加密算法类型
type EncryptionAlgorithm string

const (
	// AlgoAES256GCM AES-256-GCM 认证加密算法
	AlgoAES256GCM EncryptionAlgorithm = "aes-256-gcm"
	// AlgoAES256XTS AES-256-XTS 磁盘加密算法
	AlgoAES256XTS EncryptionAlgorithm = "aes-256-xts"
	// AlgoChaCha20Poly1305 ChaCha20-Poly1305 认证加密算法
	AlgoChaCha20Poly1305 EncryptionAlgorithm = "chacha20-poly1305"
)

// ============================================================
// 保险库状态常量
// ============================================================

// VaultStatus 保险库状态
type VaultStatus string

const (
	StatusLocked    VaultStatus = "locked"
	StatusUnlocked  VaultStatus = "unlocked"
	StatusDestroyed VaultStatus = "destroyed"
)

// ============================================================
// 访问策略类型
// ============================================================

// AccessLevel 访问级别
type AccessLevel string

const (
	AccessReadOnly  AccessLevel = "read_only"
	AccessReadWrite AccessLevel = "read_write"
	AccessAdmin     AccessLevel = "admin"
	AccessOwner     AccessLevel = "owner"
)

// SharePermission 分享权限
type SharePermission string

const (
	ShareView     SharePermission = "view"
	ShareDownload SharePermission = "download"
	ShareEdit     SharePermission = "edit"
)

// ============================================================
// 数据结构定义
// ============================================================

// Vault 隐私保险库实例
type Vault struct {
	// ID 保险库唯一标识符
	ID string `json:"id"`
	// Name 保险库名称
	Name string `json:"name"`
	// Description 保险库描述
	Description string `json:"description"`
	// Type 保险库类型
	Type string `json:"type"` // "standard", "hidden", "ephemeral"
	// Status 保险库状态
	Status VaultStatus `json:"status"`
	// Algorithm 加密算法
	Algorithm EncryptionAlgorithm `json:"algorithm"`
	// KeyID 关联的加密密钥标识
	KeyID string `json:"key_id"`
	// Size 保险库总容量（字节）
	Size int64 `json:"size"`
	// UsedSpace 已用空间（字节）
	UsedSpace int64 `json:"used_space"`
	// FileCount 文件数量
	FileCount int `json:"file_count"`
	// MountPoint 挂载路径
	MountPoint string `json:"mount_point"`
	// AutoLockMinutes 自动锁定时间（分钟），0 表示不自动锁定
	AutoLockMinutes int `json:"auto_lock_minutes"`
	// DenyExists 是否支持合理否认（隐藏保险库）
	DenyExists bool `json:"deny_exists"`
	// OwnerID 所有者用户 ID
	OwnerID string `json:"owner_id"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// AccessedAt 最后访问时间
	AccessedAt time.Time `json:"accessed_at"`
}

// Secret 保险库中的加密条目
type Secret struct {
	// ID 条目唯一标识符
	ID string `json:"id"`
	// VaultID 所属保险库 ID
	VaultID string `json:"vault_id"`
	// Name 条目名称
	Name string `json:"name"`
	// Type 条目类型（"file", "note", "credential", "key"）
	Type string `json:"type"`
	// EncryptedData 加密后的数据
	EncryptedData []byte `json:"encrypted_data"`
	// DataSize 原始数据大小（字节）
	DataSize int64 `json:"data_size"`
	// Hash 数据完整性校验哈希（SHA-256）
	Hash string `json:"hash"`
	// ShredPasses 安全擦除覆写次数
	ShredPasses int `json:"shred_passes"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// ModifiedAt 修改时间
	ModifiedAt time.Time `json:"modified_at"`
}

// AccessPolicy 访问控制策略
type AccessPolicy struct {
	// ID 策略唯一标识符
	ID string `json:"id"`
	// VaultID 关联保险库 ID
	VaultID string `json:"vault_id"`
	// UserID 目标用户 ID
	UserID string `json:"user_id"`
	// Level 访问级别
	Level AccessLevel `json:"level"`
	// AllowedIPs 允许访问的 IP 列表（空表示不限制）
	AllowedIPs []string `json:"allowed_ips,omitempty"`
	// ExpiresAt 过期时间（nil 表示永不过期）
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// MaxAccessCount 最大访问次数（0 表示不限制）
	MaxAccessCount int `json:"max_access_count"`
	// AccessCount 已访问次数
	AccessCount int `json:"access_count"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// AuditLog 审计日志条目
type AuditLog struct {
	// ID 日志唯一标识符
	ID string `json:"id"`
	// VaultID 关联保险库 ID
	VaultID string `json:"vault_id"`
	// UserID 操作用户 ID
	UserID string `json:"user_id"`
	// Action 操作类型
	Action string `json:"action"` // "create", "unlock", "lock", "access", "share", "delete", "key_rotate"
	// Resource 操作的资源标识
	Resource string `json:"resource,omitempty"`
	// Success 操作是否成功
	Success bool `json:"success"`
	// Details 操作详情
	Details string `json:"details,omitempty"`
	// IPAddress 操作来源 IP
	IPAddress string `json:"ip_address,omitempty"`
	// UserAgent 用户代理
	UserAgent string `json:"user_agent,omitempty"`
	// Timestamp 操作时间戳
	Timestamp time.Time `json:"timestamp"`
}

// VaultItem 保险库中的存储条目（文件或文件夹）
type VaultItem struct {
	// ID 条目唯一标识符
	ID string `json:"id"`
	// VaultID 所属保险库 ID
	VaultID string `json:"vault_id"`
	// Name 条目名称
	Name string `json:"name"`
	// Path 条目路径
	Path string `json:"path"`
	// IsDir 是否为目录
	IsDir bool `json:"is_dir"`
	// Size 条目大小（字节）
	Size int64 `json:"size"`
	// EncryptedSize 加密后大小（字节）
	EncryptedSize int64 `json:"encrypted_size"`
	// ContentType 内容类型
	ContentType string `json:"content_type"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// ModifiedAt 修改时间
	ModifiedAt time.Time `json:"modified_at"`
}

// ShareLink 安全分享链接
type ShareLink struct {
	// ID 链接唯一标识符
	ID string `json:"id"`
	// SecretID 关联条目 ID
	SecretID string `json:"secret_id"`
	// VaultID 关联保险库 ID
	VaultID string `json:"vault_id"`
	// Token 分享令牌
	Token string `json:"token"`
	// Permission 分享权限
	Permission SharePermission `json:"permission"`
	// Password 分享密码（可选）
	Password string `json:"password,omitempty"`
	// MaxDownloads 最大下载次数（0 表示不限制）
	MaxDownloads int `json:"max_downloads"`
	// DownloadCount 已下载次数
	DownloadCount int `json:"download_count"`
	// ExpiresAt 过期时间
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedBy 创建者用户 ID
	CreatedBy string `json:"created_by"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// KeyShare 密钥分片
type KeyShare struct {
	// ID 分片唯一标识符
	ID string `json:"id"`
	// VaultID 关联保险库 ID
	VaultID string `json:"vault_id"`
	// ShareIndex 分片索引（从 1 开始）
	ShareIndex int `json:"share_index"`
	// Threshold 恢复所需最小分片数
	Threshold int `json:"threshold"`
	// TotalShares 总分片数
	TotalShares int `json:"total_shares"`
	// EncryptedShare 加密后的分片数据
	EncryptedShare []byte `json:"encrypted_share"`
	// HolderID 分片持有者用户 ID
	HolderID string `json:"holder_id"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// 配置与统计
// ============================================================

// PrivacyVaultConfig 隐私保险箱全局配置
type PrivacyVaultConfig struct {
	// Enabled 是否启用隐私保险箱
	Enabled bool `json:"enabled"`
	// DefaultAlgorithm 默认加密算法
	DefaultAlgorithm EncryptionAlgorithm `json:"default_algorithm"`
	// AutoLockMinutes 默认自动锁定时间（分钟）
	AutoLockMinutes int `json:"auto_lock_minutes"`
	// MaxVaults 最大保险库数量
	MaxVaults int `json:"max_vaults"`
	// ShredPasses 默认安全擦除覆写次数
	ShredPasses int `json:"shred_passes"`
	// AuditEnabled 是否启用审计日志
	AuditEnabled bool `json:"audit_enabled"`
	// HiddenVaultsAllowed 是否允许创建隐藏保险库
	HiddenVaultsAllowed bool `json:"hidden_vaults_allowed"`
	// KeyRotationDays 密钥轮换周期（天）
	KeyRotationDays int `json:"key_rotation_days"`
	// MaxFailedAttempts 最大解锁失败次数
	MaxFailedAttempts int `json:"max_failed_attempts"`
}

// VaultStats 保险库统计信息
type VaultStats struct {
	// TotalVaults 保险库总数
	TotalVaults int `json:"total_vaults"`
	// LockedVaults 已锁定保险库数
	LockedVaults int `json:"locked_vaults"`
	// UnlockedVaults 已解锁保险库数
	UnlockedVaults int `json:"unlocked_vaults"`
	// HiddenVaults 隐藏保险库数
	HiddenVaults int `json:"hidden_vaults"`
	// TotalSize 总容量（字节）
	TotalSize int64 `json:"total_size"`
	// UsedSpace 已用空间（字节）
	UsedSpace int64 `json:"used_space"`
	// TotalSecrets 加密条目总数
	TotalSecrets int `json:"total_secrets"`
	// TotalShareLinks 活跃分享链接数
	TotalShareLinks int `json:"total_share_links"`
	// LastActivity 最后活动时间
	LastActivity time.Time `json:"last_activity"`
}

// ============================================================
// 错误类型
// ============================================================

// PrivacyVaultError 保险库操作错误
type PrivacyVaultError struct {
	// Code 错误码
	Code string `json:"code"`
	// Message 错误描述
	Message string `json:"message"`
	// Err 内部错误（可选）
	Err error `json:"-"`
}

// Error 实现 error 接口
func (e *PrivacyVaultError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 返回内部错误
func (e *PrivacyVaultError) Unwrap() error {
	return e.Err
}

// 预定义错误
var (
	ErrVaultNotFound      = &PrivacyVaultError{Code: "VAULT_NOT_FOUND", Message: "保险库不存在"}
	ErrVaultAlreadyExists = &PrivacyVaultError{Code: "VAULT_ALREADY_EXISTS", Message: "保险库已存在"}
	ErrVaultLocked        = &PrivacyVaultError{Code: "VAULT_LOCKED", Message: "保险库已锁定"}
	ErrVaultUnlocked      = &PrivacyVaultError{Code: "VAULT_UNLOCKED", Message: "保险库已解锁"}
	ErrInvalidPassphrase  = &PrivacyVaultError{Code: "INVALID_PASSPHRASE", Message: "密码错误"}
	ErrMaxAttemptsReached = &PrivacyVaultError{Code: "MAX_ATTEMPTS_REACHED", Message: "解锁尝试次数已超出上限"}
	ErrSecretNotFound     = &PrivacyVaultError{Code: "SECRET_NOT_FOUND", Message: "加密条目不存在"}
	ErrShareLinkExpired   = &PrivacyVaultError{Code: "SHARE_LINK_EXPIRED", Message: "分享链接已过期"}
	ErrAccessDenied       = &PrivacyVaultError{Code: "ACCESS_DENIED", Message: "访问被拒绝"}
	ErrPolicyViolation    = &PrivacyVaultError{Code: "POLICY_VIOLATION", Message: "违反访问策略"}
	ErrInvalidAlgorithm   = &PrivacyVaultError{Code: "INVALID_ALGORITHM", Message: "不支持的加密算法"}
	ErrKeyShareInvalid    = &PrivacyVaultError{Code: "KEY_SHARE_INVALID", Message: "密钥分片无效"}
)

// NewPrivacyVaultError 创建包含内部错误的 PrivacyVaultError
func NewPrivacyVaultError(code, message string, err error) *PrivacyVaultError {
	return &PrivacyVaultError{Code: code, Message: message, Err: err}
}

// DefaultConfig 返回默认配置
func DefaultConfig() *PrivacyVaultConfig {
	return &PrivacyVaultConfig{
		Enabled:             true,
		DefaultAlgorithm:    AlgoAES256GCM,
		AutoLockMinutes:     30,
		MaxVaults:           10,
		ShredPasses:         3,
		AuditEnabled:        true,
		HiddenVaultsAllowed: false,
		KeyRotationDays:     90,
		MaxFailedAttempts:   5,
	}
}
