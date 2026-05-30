// Package vaultencryption - 保险库加密模块
// 使用保险库密码解锁加密卷，提供灵活安全的数据访问
// 参考群晖 DSM 7.3 的 "Convenient encryption: Unlock encrypted volumes with a vault password"
package vaultencryption

import (
	"time"
)

// ============================================================
// 配置类型
// ============================================================

// VaultConfig 保险库配置
type VaultConfig struct {
	AutoLockTimeout  time.Duration `json:"auto_lock_timeout"`   // 自动锁定超时，默认30分钟
	MaxRetryAttempts int          `json:"max_retry_attempts"`   // 最大重试次数，默认5
	RetryLockout     time.Duration `json:"retry_lockout"`       // 重试锁定时间，默认15分钟
	KeyDerivation    string       `json:"key_derivation"`       // 密钥派生算法: argon2, scrypt, pbkdf2
	MemoryCost       int          `json:"memory_cost"`          // 内存成本（KB）
	TimeCost         int          `json:"time_cost"`            // 时间成本
	Parallelism      int          `json:"parallelism"`          // 并行度
}

// DefaultVaultConfig 默认保险库配置
func DefaultVaultConfig() VaultConfig {
	return VaultConfig{
		AutoLockTimeout:  30 * time.Minute,
		MaxRetryAttempts: 5,
		RetryLockout:     15 * time.Minute,
		KeyDerivation:    "argon2",
		MemoryCost:       65536, // 64MB
		TimeCost:         3,
		Parallelism:      4,
	}
}

// ============================================================
// 保险库密钥类型
// ============================================================

// VaultKey 保险库密钥
type VaultKey struct {
	ID             string    `json:"id"`              // 密钥ID
	Name           string    `json:"name"`            // 密钥名称
	Description    string    `json:"description"`     // 描述
	KeyHash        string    `json:"key_hash"`        // 密钥哈希（加密存储）
	Salt           string    `json:"salt"`            // 盐值
	Algorithm      string    `json:"algorithm"`       // 加密算法
	CreatedAt      time.Time `json:"created_at"`      // 创建时间
	LastUsedAt     time.Time `json:"last_used_at"`    // 最后使用时间
	ExpiresAt      *time.Time `json:"expires_at"`     // 过期时间
	IsActive       bool      `json:"is_active"`       // 是否激活
	UsageCount     int       `json:"usage_count"`     // 使用次数
}

// KeyStatus 密钥状态枚举
type KeyStatus string

const (
	KeyStatusActive   KeyStatus = "active"   // 激活
	KeyStatusExpired  KeyStatus = "expired"  // 过期
	KeyStatusRevoked  KeyStatus = "revoked"  // 已撤销
	KeyStatusLocked   KeyStatus = "locked"   // 已锁定
)

// ============================================================
// 加密卷类型
// ============================================================

// EncryptedVolume 加密卷
type EncryptedVolume struct {
	ID              string     `json:"id"`               // 卷ID
	Name            string     `json:"name"`             // 卷名称
	Device          string     `json:"device"`           // 设备路径
	MountPoint      string     `json:"mount_point"`      // 挂载点
	FileSystem      string     `json:"file_system"`      // 文件系统类型
	TotalSize       int64      `json:"total_size"`       // 总大小（字节）
	UsedSize        int64      `json:"used_size"`        // 已用大小（字节）
	EncryptionAlgo  string     `json:"encryption_algo"`  // 加密算法
	KeyID           string     `json:"key_id"`           // 关联密钥ID
	IsLocked        bool       `json:"is_locked"`        // 是否锁定
	IsMounted       bool       `json:"is_mounted"`       // 是否挂载
	LockedAt        *time.Time `json:"locked_at"`        // 锁定时间
	UnlockedAt      *time.Time `json:"unlocked_at"`      // 解锁时间
	CreatedAt       time.Time  `json:"created_at"`       // 创建时间
	UpdatedAt       time.Time  `json:"updated_at"`       // 更新时间
}

// VolumeStatus 卷状态枚举
type VolumeStatus string

const (
	VolumeStatusLocked     VolumeStatus = "locked"     // 已锁定
	VolumeStatusUnlocked   VolumeStatus = "unlocked"   // 已解锁
	VolumeStatusMounting   VolumeStatus = "mounting"   // 挂载中
	VolumeStatusMounted    VolumeStatus = "mounted"    // 已挂载
	VolumeStatusError      VolumeStatus = "error"      // 错误
)

// ============================================================
// 解锁请求/响应类型
// ============================================================

// UnlockRequest 解锁请求
type UnlockRequest struct {
	VolumeID string `json:"volume_id"` // 卷ID
	Password string `json:"password"`  // 保险库密码
}

// UnlockResponse 解锁响应
type UnlockResponse struct {
	Success    bool   `json:"success"`     // 是否成功
	VolumeID   string `json:"volume_id"`   // 卷ID
	MountPoint string `json:"mount_point"` // 挂载点
	Message    string `json:"message"`     // 消息
}

// LockRequest 锁定请求
type LockRequest struct {
	VolumeID string `json:"volume_id"` // 卷ID
	Force    bool   `json:"force"`     // 是否强制锁定（即使有进程在使用）
}

// ============================================================
// 密钥管理类型
// ============================================================

// CreateKeyRequest 创建密钥请求
type CreateKeyRequest struct {
	Name        string `json:"name"`         // 密钥名称
	Description string `json:"description"`  // 描述
	Password    string `json:"password"`     // 密码
	ExpiresIn   int    `json:"expires_in"`   // 过期时间（天），0表示永不过期
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	KeyID       string `json:"key_id"`       // 密钥ID
	OldPassword string `json:"old_password"` // 旧密码
	NewPassword string `json:"new_password"` // 新密码
}

// ============================================================
// 统计类型
// ============================================================

// VaultStats 保险库统计
type VaultStats struct {
	TotalKeys       int       `json:"total_keys"`        // 总密钥数
	ActiveKeys      int       `json:"active_keys"`       // 激活密钥数
	ExpiredKeys     int       `json:"expired_keys"`      // 过期密钥数
	TotalVolumes    int       `json:"total_volumes"`     // 总卷数
	LockedVolumes   int       `json:"locked_volumes"`    // 锁定卷数
	UnlockedVolumes int       `json:"unlocked_volumes"`  // 解锁卷数
	LastUnlockTime  time.Time `json:"last_unlock_time"`  // 最后解锁时间
	FailedAttempts  int       `json:"failed_attempts"`   // 失败尝试次数
}

// ============================================================
// 审计日志类型
// ============================================================

// AuditAction 审计动作枚举
type AuditAction string

const (
	ActionUnlock      AuditAction = "unlock"       // 解锁
	ActionLock        AuditAction = "lock"         // 锁定
	ActionCreateKey   AuditAction = "create_key"   // 创建密钥
	ActionDeleteKey   AuditAction = "delete_key"   // 删除密钥
	ActionChangePass  AuditAction = "change_pass"  // 修改密码
	ActionMount       AuditAction = "mount"        // 挂载
	ActionUnmount     AuditAction = "unmount"      // 卸载
)

// 审计动作别名（兼容 manager.go 中的引用）
const (
	AuditActionCreateKey  = ActionCreateKey
	AuditActionDeleteKey  = ActionDeleteKey
	AuditActionChangePass = ActionChangePass
)

// AuditLog 审计日志
type AuditLog struct {
	ID        string      `json:"id"`         // 日志ID
	Action    AuditAction `json:"action"`     // 动作
	VolumeID  string      `json:"volume_id"`  // 卷ID
	KeyID     string      `json:"key_id"`     // 密钥ID
	UserID    string      `json:"user_id"`    // 用户ID
	Success   bool        `json:"success"`    // 是否成功
	Message   string      `json:"message"`    // 消息
	IPAddress string      `json:"ip_address"` // IP地址
	Timestamp time.Time   `json:"timestamp"`  // 时间戳
}

// ============================================================
// HTTP 请求/响应类型
// ============================================================

// APIResponse 通用API响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// VolumeListResponse 卷列表响应
type VolumeListResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Data    []EncryptedVolume `json:"data,omitempty"`
}

// VolumeResponse 单卷响应
type VolumeResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    EncryptedVolume `json:"data"`
}

// KeyListResponse 密钥列表响应
type KeyListResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    []VaultKey `json:"data,omitempty"`
}

// KeyResponse 单密钥响应
type KeyResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    VaultKey `json:"data"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    VaultStats `json:"data"`
}

// AuditLogResponse 审计日志响应
type AuditLogResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    []AuditLog  `json:"data,omitempty"`
}

// ============================================================
// 加密配置类型
// ============================================================

// EncryptionAlgorithm 加密算法
type EncryptionAlgorithm string

const (
	AlgoAES256XTS EncryptionAlgorithm = "aes-256-xts" // AES-256-XTS
	AlgoAES256GCM EncryptionAlgorithm = "aes-256-gcm" // AES-256-GCM
	AlgoChaCha20  EncryptionAlgorithm = "chacha20"     // ChaCha20-Poly1305
)

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Algorithm    EncryptionAlgorithm `json:"algorithm"`     // 加密算法
	KeySize      int                 `json:"key_size"`      // 密钥大小（位）
	BlockSize    int                 `json:"block_size"`    // 块大小
	SectorSize   int                 `json:"sector_size"`   // 扇区大小
	UseHardware  bool                `json:"use_hardware"`  // 是否使用硬件加速
}
