// Package zeroknowledge 提供零知识加密功能
// 刑部 - Zero Knowledge Encryption Module
//
// 零知识加密确保服务端永远无法获取用户的明文密钥。
// 所有加密操作在客户端完成，服务端仅存储密文。
package zeroknowledge

import (
	"time"
)

// ========== 常量 ==========

const (
	// DefaultPBKDF2Iterations PBKDF2 默认迭代次数.
	DefaultPBKDF2Iterations = 100000
	// DefaultArgon2Memory Argon2 默认内存使用（KB）.
	DefaultArgon2Memory = 64 * 1024
	// DefaultArgon2Iterations Argon2 默认迭代次数.
	DefaultArgon2Iterations = 3
	// DefaultArgon2Parallelism Argon2 默认并行度.
	DefaultArgon2Parallelism = 4
	// SaltLength 盐值长度（字节）.
	SaltLength = 32
	// KeyLength 密钥长度（字节）- AES-256.
	KeyLength = 32
	// NonceLength AES-GCM nonce 长度（字节）.
	NonceLength = 12
	// MinShardShares 最小分片数.
	MinShardShares = 2
	// MaxShardShares 最大分片数.
	MaxShardShares = 10
)

// ========== 枚举类型 ==========

// KeyDerivationAlgorithm 密钥派生算法.
type KeyDerivationAlgorithm string

const (
	// KDFPBKDF2 PBKDF2 算法.
	KDFPBKDF2 KeyDerivationAlgorithm = "PBKDF2"
	// KDFArgon2 Argon2id 算法.
	KDFArgon2 KeyDerivationAlgorithm = "Argon2id"
)

// EncryptionAlgorithm 加密算法.
type EncryptionAlgorithm string

const (
	// EncAES256GCM AES-256-GCM 认证加密.
	EncAES256GCM EncryptionAlgorithm = "AES-256-GCM"
)

// ShareStatus 分片状态.
type ShareStatus string

const (
	// ShareActive 分片有效.
	ShareActive ShareStatus = "active"
	// ShareUsed 分片已使用.
	ShareUsed ShareStatus = "used"
	// ShareRevoked 分片已撤销.
	ShareRevoked ShareStatus = "revoked"
)

// AuditAction 审计动作类型.
type AuditAction string

const (
	// AuditKeyCreated 密钥创建.
	AuditKeyCreated AuditAction = "key_created"
	// AuditKeyDerived 密钥派生.
	AuditKeyDerived AuditAction = "key_derived"
	// AuditFileEncrypted 文件加密.
	AuditFileEncrypted AuditAction = "file_encrypted"
	// AuditFileDecrypted 文件解密.
	AuditFileDecrypted AuditAction = "file_decrypted"
	// AuditShareCreated 分片创建.
	AuditShareCreated AuditAction = "share_created"
	// AuditShareUsed 分片使用.
	AuditShareUsed AuditAction = "share_used"
	// AuditShareRevoked 分片撤销.
	AuditShareRevoked AuditAction = "share_revoked"
	// AuditKeyRecovered 密钥恢复.
	AuditKeyRecovered AuditAction = "key_recovered"
	// AuditFileShared 文件共享.
	AuditFileShared AuditAction = "file_shared"
	// AuditShareAccessed 共享访问.
	AuditShareAccessed AuditAction = "share_accessed"
)

// ========== 数据结构 ==========

// EncryptedKey 加密后的密钥材料.
type EncryptedKey struct {
	ID            string                `json:"id"`
	UserID        string                `json:"user_id"`
	EncryptedData string                `json:"encrypted_data"` // Base64 编码的加密密钥
	Salt          string                `json:"salt"`           // Base64 编码的盐值
	KDF           KeyDerivationAlgorithm `json:"kdf"`
	Algorithm     EncryptionAlgorithm   `json:"algorithm"`
	KDFIterations int                   `json:"kdf_iterations"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

// EncryptedFile 加密文件元数据.
type EncryptedFile struct {
	ID            string              `json:"id"`
	UserID        string              `json:"user_id"`
	Filename      string              `json:"filename"`
	FileSize      int64               `json:"file_size"`
	MimeType      string              `json:"mime_type"`
	EncryptedPath string              `json:"encrypted_path"` // 服务端存储路径
	KeyID         string              `json:"key_id"`         // 关联的加密密钥 ID
	Algorithm     EncryptionAlgorithm `json:"algorithm"`
	Checksum      string              `json:"checksum"` // 密文校验和
	CreatedAt     time.Time           `json:"created_at"`
}

// KeyShare 密钥分片.
type KeyShare struct {
	ID        string      `json:"id"`
	KeyID     string      `json:"key_id"`
	UserID    string      `json:"user_id"`
	ShareData string      `json:"share_data"` // Base64 编码的分片数据
	ShareIndex int        `json:"share_index"`
	Threshold int         `json:"threshold"` // 恢复所需最小分片数
	TotalShares int       `json:"total_shares"`
	Status    ShareStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UsedAt    *time.Time  `json:"used_at,omitempty"`
}

// SharedFile 零知识文件共享.
type SharedFile struct {
	ID            string    `json:"id"`
	FileID        string    `json:"file_id"`
	OwnerID       string    `json:"owner_id"`
	RecipientID   string    `json:"recipient_id"`
	EncryptedDEK  string    `json:"encrypted_dek"` // 用接收方公钥加密的数据加密密钥
	Permission    string    `json:"permission"`     // read, write
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// AuditLog 审计日志.
type AuditLog struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Action    AuditAction `json:"action"`
	Resource  string      `json:"resource"`
	Details   string      `json:"details,omitempty"`
	IPAddress string      `json:"ip_address,omitempty"`
	UserAgent string      `json:"user_agent,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ========== 配置结构 ==========

// ZKConfig 零知识加密配置.
type ZKConfig struct {
	// DefaultKDF 默认密钥派生算法.
	DefaultKDF KeyDerivationAlgorithm `json:"default_kdf"`
	// PBKDF2Iterations PBKDF2 迭代次数.
	PBKDF2Iterations int `json:"pbkdf2_iterations"`
	// Argon2Memory Argon2 内存使用（KB）.
	Argon2Memory uint32 `json:"argon2_memory"`
	// Argon2Iterations Argon2 迭代次数.
	Argon2Iterations uint32 `json:"argon2_iterations"`
	// Argon2Parallelism Argon2 并行度.
	Argon2Parallelism uint8 `json:"argon2_parallelism"`
	// DefaultThreshold 默认 Shamir 分片阈值.
	DefaultThreshold int `json:"default_threshold"`
	// DefaultTotalShares 默认分片总数.
	DefaultTotalShares int `json:"default_total_shares"`
	// MaxFileShareDuration 文件共享最大时长.
	MaxFileShareDuration time.Duration `json:"max_file_share_duration"`
	// AuditLogEnabled 是否启用审计日志.
	AuditLogEnabled bool `json:"audit_log_enabled"`
}

// DefaultZKConfig 返回默认配置.
func DefaultZKConfig() *ZKConfig {
	return &ZKConfig{
		DefaultKDF:           KDFArgon2,
		PBKDF2Iterations:     DefaultPBKDF2Iterations,
		Argon2Memory:         DefaultArgon2Memory,
		Argon2Iterations:     DefaultArgon2Iterations,
		Argon2Parallelism:    DefaultArgon2Parallelism,
		DefaultThreshold:     3,
		DefaultTotalShares:   5,
		MaxFileShareDuration: 7 * 24 * time.Hour,
		AuditLogEnabled:      true,
	}
}

// ========== API 请求/响应结构 ==========

// DeriveKeyRequest 密钥派生请求.
type DeriveKeyRequest struct {
	UserID   string                `json:"user_id" validate:"required"`
	Password string                `json:"password" validate:"required,min=8"`
	KDF      KeyDerivationAlgorithm `json:"kdf,omitempty"`
}

// DeriveKeyResponse 密钥派生响应.
type DeriveKeyResponse struct {
	KeyID     string                `json:"key_id"`
	Algorithm KeyDerivationAlgorithm `json:"algorithm"`
	Salt      string                `json:"salt"`
	Message   string                `json:"message"`
}

// EncryptFileRequest 文件加密请求.
type EncryptFileRequest struct {
	UserID   string `json:"user_id" validate:"required"`
	KeyID    string `json:"key_id" validate:"required"`
	Filename string `json:"filename" validate:"required"`
	// PlaintextData 客户端加密前的明文数据（Base64 编码，仅用于 API 测试）
	PlaintextData string `json:"plaintext_data,omitempty"`
}

// EncryptFileResponse 文件加密响应.
type EncryptFileResponse struct {
	FileID        string `json:"file_id"`
	EncryptedPath string `json:"encrypted_path"`
	Checksum      string `json:"checksum"`
}

// DecryptFileRequest 文件解密请求.
type DecryptFileRequest struct {
	UserID string `json:"user_id" validate:"required"`
	FileID string `json:"file_id" validate:"required"`
	KeyID  string `json:"key_id" validate:"required"`
}

// DecryptFileResponse 文件解密响应.
type DecryptFileResponse struct {
	FileID        string `json:"file_id"`
	Filename      string `json:"filename"`
	DecryptedData string `json:"decrypted_data"` // Base64 编码
}

// CreateSharesRequest 创建分片请求.
type CreateSharesRequest struct {
	UserID       string `json:"user_id" validate:"required"`
	KeyID        string `json:"key_id" validate:"required"`
	Threshold    int    `json:"threshold" validate:"required,min=2,max=10"`
	TotalShares  int    `json:"total_shares" validate:"required,min=2,max=10"`
}

// CreateSharesResponse 创建分片响应.
type CreateSharesResponse struct {
	Shares []ShareInfo `json:"shares"`
}

// ShareInfo 分片信息.
type ShareInfo struct {
	ShareID    string `json:"share_id"`
	ShareIndex int    `json:"share_index"`
	ShareData  string `json:"share_data"`
}

// RecoverKeyRequest 密钥恢复请求.
type RecoverKeyRequest struct {
	UserID   string   `json:"user_id" validate:"required"`
	KeyID    string   `json:"key_id" validate:"required"`
	ShareIDs []string `json:"share_ids" validate:"required,min=2"`
}

// RecoverKeyResponse 密钥恢复响应.
type RecoverKeyResponse struct {
	KeyID          string `json:"key_id"`
	RecoveredKey   string `json:"recovered_key"` // Base64 编码
	SharesUsed     int    `json:"shares_used"`
}

// ShareFileRequest 文件共享请求.
type ShareFileRequest struct {
	FileID      string  `json:"file_id" validate:"required"`
	OwnerID     string  `json:"owner_id" validate:"required"`
	RecipientID string  `json:"recipient_id" validate:"required"`
	Permission  string  `json:"permission" validate:"required,oneof=read write"`
	ExpiresIn   int     `json:"expires_in,omitempty"` // 小时数
}

// ShareFileResponse 文件共享响应.
type ShareFileResponse struct {
	ShareID   string `json:"share_id"`
	FileID    string `json:"file_id"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// ListFilesResponse 文件列表响应.
type ListFilesResponse struct {
	Files []EncryptedFile `json:"files"`
	Total int             `json:"total"`
}

// ListSharesResponse 分片列表响应.
type ListSharesResponse struct {
	Shares []KeyShare `json:"shares"`
	Total  int        `json:"total"`
}

// ListAuditLogsResponse 审计日志响应.
type ListAuditLogsResponse struct {
	Logs  []AuditLog `json:"logs"`
	Total int        `json:"total"`
}

// ShamirShare Shamir 秘密分片（内部使用）.
type ShamirShare struct {
	X int    `json:"x"`
	Y []byte `json:"y"`
}
