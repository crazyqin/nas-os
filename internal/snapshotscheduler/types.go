// Package snapshotscheduler 提供自动化快照调度管理功能。
// 参考 TrueNAS ZFS 快照管理、群晖 Btrfs 快照，支持定时快照、保留策略、克隆、回滚等。
package snapshotscheduler

import (
	"time"
)

// ============================================================
// 类型定义
// ============================================================

// SnapshotStatus 快照状态
type SnapshotStatus string

const (
	StatusPending  SnapshotStatus = "pending"
	StatusCreating SnapshotStatus = "creating"
	StatusActive   SnapshotStatus = "active"
	StatusDeleting SnapshotStatus = "deleting"
	StatusDeleted  SnapshotStatus = "deleted"
	StatusError    SnapshotStatus = "error"
)

// ScheduleFrequency 调度频率
type ScheduleFrequency string

const (
	FreqMinutely ScheduleFrequency = "minutely"
	FreqHourly   ScheduleFrequency = "hourly"
	FreqDaily    ScheduleFrequency = "daily"
	FreqWeekly   ScheduleFrequency = "weekly"
	FreqMonthly  ScheduleFrequency = "monthly"
	FreqCustom   ScheduleFrequency = "custom"
)

// RetentionUnit 保留单位
type RetentionUnit string

const (
	RetainByCount RetentionUnit = "count"
	RetainByAge   RetentionUnit = "age"
	RetainBySize  RetentionUnit = "size"
)

// FileSystemType 文件系统类型
type FileSystemType string

const (
	FSZFS       FileSystemType = "zfs"
	FSBtrfs     FileSystemType = "btrfs"
	FSExt4      FileSystemType = "ext4"
	FSSimulated FileSystemType = "simulated"
)

// Snapshot 快照
type Snapshot struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	VolumePath  string         `json:"volume_path"`
	FilePath    string         `json:"file_path"`
	Size        int64          `json:"size"`
	Status      SnapshotStatus `json:"status"`
	FSType      FileSystemType `json:"fs_type"`
	ParentID    string         `json:"parent_id,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Description string         `json:"description,omitempty"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
}

// Schedule 快照调度计划
type Schedule struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	VolumePath string            `json:"volume_path"`
	Frequency  ScheduleFrequency `json:"frequency"`
	// Cron 表达式（当 Frequency 为 custom 时使用）
	CronExpr string `json:"cron_expr,omitempty"`
	// 具体时间配置
	Hour       int  `json:"hour,omitempty"`
	Minute     int  `json:"minute,omitempty"`
	DayOfWeek  int  `json:"day_of_week,omitempty"` // 0=Sunday
	DayOfMonth int  `json:"day_of_month,omitempty"`
	Enabled    bool `json:"enabled"`
	// 保留策略
	Retention RetentionPolicy `json:"retention"`
	// 快照前脚本
	PreScript string `json:"pre_script,omitempty"`
	// 快照后脚本
	PostScript string     `json:"post_script,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	NextRunAt  *time.Time `json:"next_run_at,omitempty"`
	RunCount   int        `json:"run_count"`
	FailCount  int        `json:"fail_count"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	Unit         RetentionUnit `json:"unit"`
	MaxCount     int           `json:"max_count,omitempty"`      // 最大保留数量
	MaxAgeDays   int           `json:"max_age_days,omitempty"`   // 最大保留天数
	MaxSizeGB    int64         `json:"max_size_gb,omitempty"`    // 最大总大小(GB)
	MinKeepCount int           `json:"min_keep_count,omitempty"` // 最少保留数量（防止全部过期）
}

// CloneResult 克隆结果
type CloneResult struct {
	CloneID    string `json:"clone_id"`
	SourceID   string `json:"source_id"`
	TargetPath string `json:"target_path"`
	Size       int64  `json:"size"`
}

// RollbackRequest 回滚请求
type RollbackRequest struct {
	SnapshotID string `json:"snapshot_id" binding:"required"`
	TargetPath string `json:"target_path,omitempty"`
	Force      bool   `json:"force,omitempty"`
}

// SchedulerStats 调度器统计
type SchedulerStats struct {
	TotalSnapshots  int                    `json:"total_snapshots"`
	TotalSchedules  int                    `json:"total_schedules"`
	ActiveSchedules int                    `json:"active_schedules"`
	TotalSizeBytes  int64                  `json:"total_size_bytes"`
	ByStatus        map[SnapshotStatus]int `json:"by_status"`
	ByVolume        map[string]int         `json:"by_volume"`
	LastSnapshotAt  *time.Time             `json:"last_snapshot_at,omitempty"`
	NextSnapshotAt  *time.Time             `json:"next_snapshot_at,omitempty"`
}
