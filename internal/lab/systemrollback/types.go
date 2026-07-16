// Package systemrollback 实现系统快照回滚模块
// 基于 btrfs 快照实现系统级快照、一键回滚、自动快照策略、差异对比
package systemrollback

import (
	"errors"
	"time"
)

var (
	ErrSnapshotNotFound = errors.New("snapshot not found")
	ErrSnapshotExists   = errors.New("snapshot already exists")
	ErrRollbackFailed   = errors.New("rollback failed")
	ErrPolicyNotFound   = errors.New("policy not found")
	ErrSystemBusy       = errors.New("system busy")
)

// SnapshotType 快照类型.
type SnapshotType string

const (
	SnapshotManual    SnapshotType = "manual"
	SnapshotAuto      SnapshotType = "auto"
	SnapshotPreUpdate SnapshotType = "pre_update"
	SnapshotScheduled SnapshotType = "scheduled"
)

// SnapshotStatus 快照状态.
type SnapshotStatus string

const (
	StatusCreating SnapshotStatus = "creating"
	StatusReady    SnapshotStatus = "ready"
	StatusRolling  SnapshotStatus = "rolling_back"
	StatusDeleting SnapshotStatus = "deleting"
	StatusFailed   SnapshotStatus = "failed"
)

// SystemSnapshot 系统快照.
type SystemSnapshot struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Type          SnapshotType   `json:"type"`
	Status        SnapshotStatus `json:"status"`
	Path          string         `json:"path"`
	Size          int64          `json:"size"`      // bytes
	ParentID      string         `json:"parent_id"` // 增量快照的父快照
	IsIncremental bool           `json:"is_incremental"`
	Tags          []string       `json:"tags"`
	RollbackCount int            `json:"rollback_count"`
	LastRollback  time.Time      `json:"last_rollback"`
	CreatedAt     time.Time      `json:"created_at"`
	ExpiresAt     *time.Time     `json:"expires_at"`
}

// SnapshotPolicy 快照策略.
type SnapshotPolicy struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Enabled       bool         `json:"enabled"`
	Schedule      string       `json:"schedule"` // cron 表达式
	SnapshotType  SnapshotType `json:"snapshot_type"`
	MaxSnapshots  int          `json:"max_snapshots"`  // 最大保留数
	RetentionDays int          `json:"retention_days"` // 保留天数
	AutoCleanup   bool         `json:"auto_cleanup"`
	Incremental   bool         `json:"incremental"`
	CompressType  string       `json:"compress_type"` // zstd, lzo, none
	LastRun       time.Time    `json:"last_run"`
	NextRun       time.Time    `json:"next_run"`
	CreatedAt     time.Time    `json:"created_at"`
}

// RollbackRequest 回滚请求.
type RollbackRequest struct {
	SnapshotID    string `json:"snapshot_id"`
	RebootAfter   bool   `json:"reboot_after"`
	BackupCurrent bool   `json:"backup_current"` // 回滚前先备份当前状态
	DryRun        bool   `json:"dry_run"`        // 仅检查，不实际回滚
}

// RollbackResult 回滚结果.
type RollbackResult struct {
	Success       bool      `json:"success"`
	SnapshotID    string    `json:"snapshot_id"`
	BackupID      string    `json:"backup_id"` // 回滚前的备份快照 ID
	Duration      string    `json:"duration"`
	RebootPending bool      `json:"reboot_pending"`
	Error         string    `json:"error"`
	Changes       ChangeSet `json:"changes"`
}

// ChangeSet 变更集.
type ChangeSet struct {
	FilesAdded    int   `json:"files_added"`
	FilesModified int   `json:"files_modified"`
	FilesDeleted  int   `json:"files_deleted"`
	BytesChanged  int64 `json:"bytes_changed"`
}

// DiffResult 快照差异.
type DiffResult struct {
	Snapshot1   string    `json:"snapshot1"`
	Snapshot2   string    `json:"snapshot2"`
	ChangeSet   ChangeSet `json:"changeset"`
	GeneratedAt time.Time `json:"generated_at"`
}

// RollbackStats 统计.
type RollbackStats struct {
	TotalSnapshots   int            `json:"total_snapshots"`
	ReadySnapshots   int            `json:"ready_snapshots"`
	TotalPolicies    int            `json:"total_policies"`
	ActivePolicies   int            `json:"active_policies"`
	TotalSize        int64          `json:"total_size"`
	LastSnapshot     time.Time      `json:"last_snapshot"`
	LastRollback     time.Time      `json:"last_rollback"`
	TypeDistribution map[string]int `json:"type_distribution"`
}

// RollbackConfig 配置.
type RollbackConfig struct {
	SnapshotRoot    string `json:"snapshot_root"`
	SystemRoot      string `json:"system_root"`
	MaxConcurrent   int    `json:"max_concurrent"`
	CompressDefault string `json:"compress_default"`
}
