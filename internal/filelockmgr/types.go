package filelockmgr

import "time"

// LockType 锁类型
type LockType string

const (
	// LockTypeExclusive 独占锁
	LockTypeExclusive LockType = "exclusive"
	// LockTypeShared 共享锁
	LockTypeShared LockType = "shared"
)

// ConflictStrategy 冲突策略
type ConflictStrategy string

const (
	// StrategyReject 拒绝新请求
	StrategyReject ConflictStrategy = "reject"
	// StrategyQueue 排队等待
	StrategyQueue ConflictStrategy = "queue"
	// StrategyPreempt 高优先级抢占
	StrategyPreempt ConflictStrategy = "preempt"
)

// FileLockEntry 文件锁条目
type FileLockEntry struct {
	ID           string    `json:"id"`
	FilePath     string    `json:"file_path"`
	LockType     LockType  `json:"lock_type"`
	LockedBy     string    `json:"locked_by"`
	LockedByName string    `json:"locked_by_name"`
	Reason       string    `json:"reason"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	Priority     int       `json:"priority"`
}

// LockConflict 锁冲突记录
type LockConflict struct {
	ID              string    `json:"id"`
	FilePath        string    `json:"file_path"`
	RequesterID     string    `json:"requester_id"`
	CurrentHolderID string    `json:"current_holder_id"`
	ConflictType    string    `json:"conflict_type"`
	ResolvedAt      time.Time `json:"resolved_at,omitempty"`
	Resolution      string    `json:"resolution,omitempty"`
}

// LockStats 锁统计信息
type LockStats struct {
	TotalLocks       int     `json:"total_locks"`
	ExclusiveLocks   int     `json:"exclusive_locks"`
	SharedLocks      int     `json:"shared_locks"`
	PendingConflicts int     `json:"pending_conflicts"`
	AvgLockDuration  float64 `json:"avg_lock_duration"`
}

// LockRequest 锁请求
type LockRequest struct {
	FilePath     string   `json:"file_path" binding:"required"`
	LockType     LockType `json:"lock_type" binding:"required"`
	LockedBy     string   `json:"locked_by" binding:"required"`
	LockedByName string   `json:"locked_by_name" binding:"required"`
	Reason       string   `json:"reason"`
	Duration     int64    `json:"duration"`
	UpgradeFrom  string   `json:"upgrade_from,omitempty"`
	Priority     int      `json:"priority"`
}

// LockPolicy 锁策略配置
type LockPolicy struct {
	MaxLockDuration  time.Duration    `json:"max_lock_duration"`
	AutoExpire       bool             `json:"auto_expire"`
	AllowUpgrade     bool             `json:"allow_upgrade"`
	MaxLocksPerUser  int              `json:"max_locks_per_user"`
	ConflictStrategy ConflictStrategy `json:"conflict_strategy"`
}

// RefreshRequest 续期请求
type RefreshRequest struct {
	Duration int64 `json:"duration" binding:"required"`
}

// ForceReleaseRequest 强制释放请求
type ForceReleaseRequest struct {
	AdminID string `json:"admin_id" binding:"required"`
}
