// Package timebackup 提供文件/目录版本备份与恢复功能
package timebackup

import (
	"time"
)

// SnapshotStrategy 快照策略类型.
type SnapshotStrategy string

const (
	StrategyCopy  SnapshotStrategy = "copy"  // 文件复制
	StrategyBtrfs SnapshotStrategy = "btrfs" // btrfs 快照（需内核支持）
)

// TaskStatus 备份任务状态.
type TaskStatus string

const (
	TaskStatusIdle     TaskStatus = "idle"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusSuccess  TaskStatus = "success"
	TaskStatusFailed   TaskStatus = "failed"
	TaskStatusDisabled TaskStatus = "disabled"
)

// RetentionMode 保留策略模式.
type RetentionMode string

const (
	RetentionByCount RetentionMode = "count" // 按数量保留
	RetentionByTime  RetentionMode = "time"  // 按时间保留
	RetentionBySpace RetentionMode = "space" // 按空间限制
)

// Snapshot 文件/目录快照.
type Snapshot struct {
	ID          string            `json:"id"`
	TaskID      string            `json:"task_id"`
	SourcePath  string            `json:"source_path"`
	SnapshotDir string            `json:"snapshot_dir"`
	Size        int64             `json:"size"`
	FileCount   int               `json:"file_count"`
	Strategy    SnapshotStrategy  `json:"strategy"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// Version 文件版本信息（用于列表展示）.
type Version struct {
	SnapshotID string    `json:"snapshot_id"`
	TaskID     string    `json:"task_id"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	FileCount  int       `json:"file_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// DiffEntry 文件差异条目.
type DiffEntry struct {
	Path       string `json:"path"`
	Change     string `json:"change"` // added, removed, modified
	SizeOld    int64  `json:"size_old,omitempty"`
	SizeNew    int64  `json:"size_new,omitempty"`
	ModTimeOld string `json:"mod_time_old,omitempty"`
	ModTimeNew string `json:"mod_time_new,omitempty"`
}

// DiffResult 两个快照间的对比结果.
type DiffResult struct {
	SnapshotOld string       `json:"snapshot_old"`
	SnapshotNew string       `json:"snapshot_new"`
	Entries     []*DiffEntry `json:"entries"`
	Summary     DiffSummary  `json:"summary"`
}

// DiffSummary 差异摘要.
type DiffSummary struct {
	Added    int `json:"added"`
	Removed  int `json:"removed"`
	Modified int `json:"modified"`
	Total    int `json:"total"`
}

// RetentionPolicy 保留策略配置.
type RetentionPolicy struct {
	Mode       RetentionMode `json:"mode"`
	MaxCount   int           `json:"max_count,omitempty"`    // 最大保留数量
	MaxAgeDays int           `json:"max_age_days,omitempty"` // 最大保留天数
	MaxSizeGB  float64       `json:"max_size_gb,omitempty"`  // 最大占用空间 (GB)
}

// BackupTask 备份任务定义.
type BackupTask struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	SourcePath    string           `json:"source_path"`
	Strategy      SnapshotStrategy `json:"strategy"`
	Schedule      string           `json:"schedule"` // cron 表达式，空表示仅手动
	Retention     RetentionPolicy  `json:"retention"`
	Status        TaskStatus       `json:"status"`
	Enabled       bool             `json:"enabled"`
	LastRun       *time.Time       `json:"last_run,omitempty"`
	LastError     string           `json:"last_error,omitempty"`
	SnapshotCount int              `json:"snapshot_count"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// CreateTaskRequest 创建备份任务请求.
type CreateTaskRequest struct {
	Name       string           `json:"name" binding:"required"`
	SourcePath string           `json:"source_path" binding:"required"`
	Strategy   SnapshotStrategy `json:"strategy"`
	Schedule   string           `json:"schedule"`
	Retention  *RetentionPolicy `json:"retention"`
}

// TriggerRequest 手动触发请求.
type TriggerRequest struct {
	TaskID string `json:"task_id" binding:"required"`
}

// RestoreRequest 恢复请求.
type RestoreRequest struct {
	SnapshotID string `json:"snapshot_id" binding:"required"`
	TargetPath string `json:"target_path" binding:"required"`
	Overwrite  bool   `json:"overwrite"`
}

// ListVersionsRequest 查询版本列表请求.
type ListVersionsRequest struct {
	TaskID string `form:"task_id"`
	Path   string `form:"path"`
	Limit  int    `form:"limit"`
}

// DiffRequest 版本对比请求.
type DiffRequest struct {
	SnapshotOld string `json:"snapshot_old" binding:"required"`
	SnapshotNew string `json:"snapshot_new" binding:"required"`
}

// response 标准 API 响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
