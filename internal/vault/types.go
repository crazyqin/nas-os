// Package vault 实现加密保险库功能，参考群晖 DSM 7.3 的 Vault Encryption。
// 提供保险库的创建、锁定、解锁、删除及加密操作支持。
package vault

import (
	"fmt"
	"time"
)

// 支持的加密算法常量
const (
	AlgorithmAES256GCM       = "aes-256-gcm"
	AlgorithmChaCha20Poly1305 = "chacha20-poly1305"
)

// 保险库状态常量
const (
	StatusLocked   = "locked"
	StatusUnlocked = "unlocked"
	StatusError    = "error"
)

// 密钥派生算法常量
const (
	KeyDerivationArgon2id = "argon2id"
	KeyDerivationPBKDF2   = "pbkdf2"
)

// Vault 表示一个加密保险库实例。
type Vault struct {
	// ID 保险库唯一标识符
	ID string `json:"id"`
	// Name 保险库名称
	Name string `json:"name"`
	// Description 保险库描述信息
	Description string `json:"description"`
	// MountPath 保险库挂载路径
	MountPath string `json:"mount_path"`
	// KeyID 关联的加密密钥标识
	KeyID string `json:"key_id"`
	// Algorithm 加密算法，支持 aes-256-gcm 和 chacha20-poly1305
	Algorithm string `json:"algorithm"`
	// Status 保险库当前状态：locked / unlocked / error
	Status string `json:"status"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// LastAccessAt 最后访问时间
	LastAccessAt time.Time `json:"last_access_at"`
	// FileCount 保险库内文件数量
	FileCount int64 `json:"file_count"`
	// TotalSize 保险库内文件总大小（字节）
	TotalSize int64 `json:"total_size"`
	// VerificationToken 加密验证令牌（hex 编码），用于验证解锁密码
	VerificationToken string `json:"verification_token,omitempty"`
}

// VaultConfig 保险库全局配置。
type VaultConfig struct {
	// DefaultAlgorithm 默认加密算法
	DefaultAlgorithm string `json:"default_algorithm"`
	// AutoLockMinutes 自动锁定时间（分钟），0 表示不自动锁定
	AutoLockMinutes int `json:"auto_lock_minutes"`
	// MaxFailedAttempts 最大解锁失败尝试次数
	MaxFailedAttempts int `json:"max_failed_attempts"`
	// KeyDerivation 密钥派生算法，支持 argon2id 和 pbkdf2
	KeyDerivation string `json:"key_derivation"`
}

// VaultStats 保险库统计信息。
type VaultStats struct {
	// TotalVaults 保险库总数
	TotalVaults int `json:"total_vaults"`
	// UnlockedVaults 已解锁的保险库数量
	UnlockedVaults int `json:"unlocked_vaults"`
	// TotalFiles 所有保险库的文件总数
	TotalFiles int64 `json:"total_files"`
	// TotalSize 所有保险库的文件总大小（字节）
	TotalSize int64 `json:"total_size"`
	// EncryptionOps 累计加密操作次数
	EncryptionOps int64 `json:"encryption_ops"`
}

// VaultError 保险库操作错误类型，实现 error 接口。
type VaultError struct {
	// Code 错误码
	Code string `json:"code"`
	// Message 错误描述信息
	Message string `json:"message"`
	// Err 内部错误（可选）
	Err error `json:"-"`
}

// Error 实现 error 接口，返回可读的错误信息。
func (e *VaultError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 返回内部错误，支持 errors.Is / errors.As。
func (e *VaultError) Unwrap() error {
	return e.Err
}

// 预定义错误码
var (
	// ErrVaultNotFound 保险库不存在
	ErrVaultNotFound = &VaultError{Code: "VAULT_NOT_FOUND", Message: "保险库不存在"}
	// ErrVaultAlreadyExists 保险库名称已存在
	ErrVaultAlreadyExists = &VaultError{Code: "VAULT_ALREADY_EXISTS", Message: "保险库已存在"}
	// ErrVaultLocked 保险库已锁定，无法执行操作
	ErrVaultLocked = &VaultError{Code: "VAULT_LOCKED", Message: "保险库已锁定"}
	// ErrVaultAlreadyUnlocked 保险库已处于解锁状态
	ErrVaultAlreadyUnlocked = &VaultError{Code: "VAULT_ALREADY_UNLOCKED", Message: "保险库已解锁"}
	// ErrInvalidPassphrase 密码错误
	ErrInvalidPassphrase = &VaultError{Code: "INVALID_PASSPHRASE", Message: "密码错误"}
	// ErrInvalidAlgorithm 不支持的加密算法
	ErrInvalidAlgorithm = &VaultError{Code: "INVALID_ALGORITHM", Message: "不支持的加密算法"}
	// ErrMaxAttemptsExceeded 解锁失败次数超过上限
	ErrMaxAttemptsExceeded = &VaultError{Code: "MAX_ATTEMPTS_EXCEEDED", Message: "解锁尝试次数已超出上限"}
	// ErrInvalidPath 无效的挂载路径
	ErrInvalidPath = &VaultError{Code: "INVALID_PATH", Message: "无效的挂载路径"}
	// ErrVaultNotEmpty 保险库非空，无法删除
	ErrVaultNotEmpty = &VaultError{Code: "VAULT_NOT_EMPTY", Message: "保险库非空，无法删除"}
)

// NewVaultError 创建一个包含内部错误的 VaultError。
func NewVaultError(code, message string, err error) *VaultError {
	return &VaultError{Code: code, Message: message, Err: err}
}
