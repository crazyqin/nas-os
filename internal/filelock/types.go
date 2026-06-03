// Package filelock 提供文件锁定协作功能，支持独占锁定、共享锁定、超时自动释放、
// 冲突检测、管理员强制解锁及锁定历史审计。参考群晖 DSM 7.3 的文件锁定机制。
package filelock

import "time"

// LockType 锁类型
type LockType string

const (
	// LockTypeExclusive 独占锁定：同一时间只允许一个用户锁定
	LockTypeExclusive LockType = "exclusive"
	// LockTypeShared 共享锁定：允许多个用户以只读方式同时锁定
	LockTypeShared LockType = "shared"
)

// LockStatus 锁状态
type LockStatus string

const (
	// LockStatusActive 活跃状态
	LockStatusActive LockStatus = "active"
	// LockStatusExpired 已过期
	LockStatusExpired LockStatus = "expired"
	// LockStatusReleased 已释放
	LockStatusReleased LockStatus = "released"
	// LockStatusForceReleased 被管理员强制释放
	LockStatusForceReleased LockStatus = "force_released"
)

// LockInfo 锁的详细信息
type LockInfo struct {
	// 锁定的文件路径
	FilePath string `json:"file_path"`
	// 锁定的用户ID
	UserID string `json:"user_id"`
	// 锁定的用户名
	UserName string `json:"user_name"`
	// 锁类型
	LockType LockType `json:"lock_type"`
	// 获取锁的时间
	AcquiredAt time.Time `json:"acquired_at"`
	// 过期时间
	ExpiresAt time.Time `json:"expires_at"`
	// 锁定备注
	Comment string `json:"comment,omitempty"`
}

// FileLock 文件锁记录
type FileLock struct {
	// 唯一标识
	ID string `json:"id"`
	// 文件路径
	FilePath string `json:"file_path"`
	// 锁类型
	LockType LockType `json:"lock_type"`
	// 锁状态
	Status LockStatus `json:"status"`
	// 锁定的用户ID
	UserID string `json:"user_id"`
	// 锁定的用户名
	UserName string `json:"user_name"`
	// 共享锁定的用户列表（仅共享锁有效）
	SharedUsers []LockInfo `json:"shared_users,omitempty"`
	// 获取锁的时间
	AcquiredAt time.Time `json:"acquired_at"`
	// 最后续期时间
	LastRenewedAt time.Time `json:"last_renewed_at"`
	// 过期时间
	ExpiresAt time.Time `json:"expires_at"`
	// 释放时间（如果已释放）
	ReleasedAt *time.Time `json:"released_at,omitempty"`
	// 释放者ID（如果被管理员强制释放）
	ReleasedBy string `json:"released_by,omitempty"`
	// 锁定备注
	Comment string `json:"comment,omitempty"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// LockPolicy 锁定策略
type LockPolicy struct {
	// 是否启用文件锁定
	Enabled bool `json:"enabled"`
	// 默认锁类型
	DefaultLockType LockType `json:"default_lock_type"`
	// 独占锁最大持续时间（分钟）
	ExclusiveLockMaxDuration int `json:"exclusive_lock_max_duration"`
	// 共享锁最大持续时间（分钟）
	SharedLockMaxDuration int `json:"shared_lock_max_duration"`
	// 是否允许用户续期
	AllowRenewal bool `json:"allow_renewal"`
	// 最大续期次数
	MaxRenewals int `json:"max_renewals"`
	// 是否允许管理员强制解锁
	AllowForceRelease bool `json:"allow_force_release"`
	// 自动清理过期锁的间隔（分钟）
	CleanupInterval int `json:"cleanup_interval"`
	// 锁历史保留天数
	HistoryRetentionDays int `json:"history_retention_days"`
	// 最大并发锁数量（每个用户）
	MaxLocksPerUser int `json:"max_locks_per_user"`
}

// LockStats 锁定统计信息
type LockStats struct {
	// 活跃锁总数
	ActiveLocks int `json:"active_locks"`
	// 独占锁数量
	ExclusiveLocks int `json:"exclusive_locks"`
	// 共享锁数量
	SharedLocks int `json:"shared_locks"`
	// 过期锁数量（未清理）
	ExpiredLocks int `json:"expired_locks"`
	// 今日获取锁次数
	TodayAcquisitions int `json:"today_acquisitions"`
	// 今日释放锁次数
	TodayReleases int `json:"today_releases"`
	// 今日强制释放次数
	TodayForceReleases int `json:"today_force_releases"`
	// 活跃用户数
	ActiveUsers int `json:"active_users"`
	// 锁定冲突次数（今日）
	TodayConflicts int `json:"today_conflicts"`
}

// AcquireRequest 获取锁请求
type AcquireRequest struct {
	// 文件路径
	FilePath string `json:"file_path" binding:"required"`
	// 用户ID
	UserID string `json:"user_id" binding:"required"`
	// 用户名
	UserName string `json:"user_name" binding:"required"`
	// 锁类型（可选，默认使用策略中的默认类型）
	LockType LockType `json:"lock_type,omitempty"`
	// 锁定时长（分钟，可选，默认使用策略中的最大时长）
	Duration int `json:"duration,omitempty"`
	// 备注
	Comment string `json:"comment,omitempty"`
}

// ReleaseRequest 释放锁请求
type ReleaseRequest struct {
	// 锁ID
	LockID string `json:"lock_id" binding:"required"`
	// 用户ID
	UserID string `json:"user_id" binding:"required"`
}

// RenewRequest 续期请求
type RenewRequest struct {
	// 锁ID
	LockID string `json:"lock_id" binding:"required"`
	// 用户ID
	UserID string `json:"user_id" binding:"required"`
	// 续期时长（分钟，可选）
	Duration int `json:"duration,omitempty"`
}

// ForceReleaseRequest 强制释放请求
type ForceReleaseRequest struct {
	// 锁ID
	LockID string `json:"lock_id" binding:"required"`
	// 管理员用户ID
	AdminID string `json:"admin_id" binding:"required"`
	// 强制释放原因
	Reason string `json:"reason,omitempty"`
}

// LockHistoryEntry 锁定历史记录
type LockHistoryEntry struct {
	// 唯一标识
	ID string `json:"id"`
	// 文件路径
	FilePath string `json:"file_path"`
	// 操作类型
	Action string `json:"action"`
	// 操作用户ID
	UserID string `json:"user_id"`
	// 操作用户名
	UserName string `json:"user_name"`
	// 锁类型
	LockType LockType `json:"lock_type,omitempty"`
	// 操作详情
	Detail string `json:"detail,omitempty"`
	// 操作时间
	Timestamp time.Time `json:"timestamp"`
}

// ListLocksRequest 列出锁请求
type ListLocksRequest struct {
	// 按文件路径过滤（支持前缀匹配）
	FilePath string `json:"file_path,omitempty"`
	// 按用户ID过滤
	UserID string `json:"user_id,omitempty"`
	// 按状态过滤
	Status LockStatus `json:"status,omitempty"`
	// 按锁类型过滤
	LockType LockType `json:"lock_type,omitempty"`
	// 页码（从1开始）
	Page int `json:"page,omitempty"`
	// 每页数量
	PageSize int `json:"page_size,omitempty"`
}

// DefaultLockPolicy 默认锁定策略
func DefaultLockPolicy() *LockPolicy {
	return &LockPolicy{
		Enabled:                true,
		DefaultLockType:        LockTypeExclusive,
		ExclusiveLockMaxDuration: 480,  // 8小时
		SharedLockMaxDuration:    1440, // 24小时
		AllowRenewal:           true,
		MaxRenewals:            3,
		AllowForceRelease:      true,
		CleanupInterval:        5,     // 5分钟
		HistoryRetentionDays:   90,
		MaxLocksPerUser:        10,
	}
}
