// Package crossplatformsync provides cross-platform NAS file synchronization.
// Supports multi-device sync with bidirectional/mirror modes,
// conflict detection/resolution, sync monitoring, and complete API.
// Reference: Synology Cloud Sync / rsync / Syncthing
package crossplatformsync

import (
	"fmt"
	"time"
)

// ============================================================
// 设备信息
// ============================================================

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusSyncing DeviceStatus = "syncing"
	DeviceStatusError   DeviceStatus = "error"
)

// NASDevice NAS 设备信息
type NASDevice struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Address      string       `json:"address"` // IP 或域名
	Port         int          `json:"port"`    // 同步服务端口
	APIKey       string       `json:"api_key"` // API 密钥
	Status       DeviceStatus `json:"status"`
	LastSeen     *time.Time   `json:"last_seen,omitempty"`
	Version      string       `json:"version"`      // NAS-OS 版本
	Platform     string       `json:"platform"`     // 平台类型 (linux/windows/macos)
	Capabilities []string     `json:"capabilities"` // 支持的功能
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// Validate 验证设备信息
func (d *NASDevice) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("device name is required")
	}
	if d.Address == "" {
		return fmt.Errorf("device address is required")
	}
	if d.Port <= 0 || d.Port > 65535 {
		return fmt.Errorf("invalid port: %d", d.Port)
	}
	return nil
}

// ============================================================
// 同步模式
// ============================================================

// SyncMode 同步模式
type SyncMode string

const (
	SyncModeBidirectional SyncMode = "bidirectional" // 双向同步
	SyncModeMirror        SyncMode = "mirror"        // 单向镜像（源 -> 目标）
	SyncModeOneWay        SyncMode = "one_way"       // 单向同步（仅新增/更新）
)

// IsValid 检查同步模式是否有效
func (m SyncMode) IsValid() bool {
	switch m {
	case SyncModeBidirectional, SyncModeMirror, SyncModeOneWay:
		return true
	}
	return false
}

// ============================================================
// 冲突策略
// ============================================================

// ConflictStrategy 冲突解决策略
type ConflictStrategy string

const (
	ConflictStrategySource   ConflictStrategy = "source"    // 源设备优先
	ConflictStrategyTarget   ConflictStrategy = "target"    // 目标设备优先
	ConflictStrategyNewer    ConflictStrategy = "newer"     // 较新文件优先
	ConflictStrategyLarger   ConflictStrategy = "larger"    // 较大文件优先
	ConflictStrategyKeepBoth ConflictStrategy = "keep_both" // 保留两者（重命名冲突文件）
	ConflictStrategyManual   ConflictStrategy = "manual"    // 手动解决
)

// IsValid 检查冲突策略是否有效
func (s ConflictStrategy) IsValid() bool {
	switch s {
	case ConflictStrategySource, ConflictStrategyTarget, ConflictStrategyNewer,
		ConflictStrategyLarger, ConflictStrategyKeepBoth, ConflictStrategyManual:
		return true
	}
	return false
}

// ============================================================
// 同步任务状态
// ============================================================

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusIdle      TaskStatus = "idle"
	TaskStatusSyncing   TaskStatus = "syncing"
	TaskStatusPaused    TaskStatus = "paused"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusConflict  TaskStatus = "conflict" // 存在待解决的冲突
)

// ============================================================
// 文件冲突信息
// ============================================================

// FileConflict 文件冲突信息
type FileConflict struct {
	ID            string     `json:"id"`
	TaskID        string     `json:"task_id"`
	FilePath      string     `json:"file_path"`       // 冲突文件路径
	SourceDevice  string     `json:"source_device"`   // 源设备 ID
	TargetDevice  string     `json:"target_device"`   // 目标设备 ID
	SourceModTime time.Time  `json:"source_mod_time"` // 源文件修改时间
	TargetModTime time.Time  `json:"target_mod_time"` // 目标文件修改时间
	SourceSize    int64      `json:"source_size"`     // 源文件大小
	TargetSize    int64      `json:"target_size"`     // 目标文件大小
	SourceHash    string     `json:"source_hash"`     // 源文件哈希
	TargetHash    string     `json:"target_hash"`     // 目标文件哈希
	Resolution    string     `json:"resolution"`      // 解决方案 (source/target/newer/larger/keep_both)
	Resolved      bool       `json:"resolved"`        // 是否已解决
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ============================================================
// 同步任务
// ============================================================

// SyncTask 同步任务
type SyncTask struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	SourceDeviceID   string           `json:"source_device_id"`  // 源设备 ID
	TargetDeviceID   string           `json:"target_device_id"`  // 目标设备 ID
	SourcePath       string           `json:"source_path"`       // 源路径
	TargetPath       string           `json:"target_path"`       // 目标路径
	Mode             SyncMode         `json:"mode"`              // 同步模式
	ConflictStrategy ConflictStrategy `json:"conflict_strategy"` // 冲突策略
	Enabled          bool             `json:"enabled"`
	Status           TaskStatus       `json:"status"`

	// 过滤规则
	IncludePatterns []string `json:"include_patterns,omitempty"` // 包含模式 (glob)
	ExcludePatterns []string `json:"exclude_patterns,omitempty"` // 排除模式 (glob)
	MaxFileSize     int64    `json:"max_file_size,omitempty"`    // 最大文件大小 (bytes)
	MinFileSize     int64    `json:"min_file_size,omitempty"`    // 最小文件大小 (bytes)

	// 同步选项
	PreserveModTime  bool `json:"preserve_mod_time"` // 保留修改时间
	PreservePerms    bool `json:"preserve_perms"`    // 保留权限
	DeleteExtraneous bool `json:"delete_extraneous"` // 删除目标多余文件
	ChecksumVerify   bool `json:"checksum_verify"`   // 校验和验证
	CompressTransfer bool `json:"compress_transfer"` // 压缩传输
	BandwidthLimit   int  `json:"bandwidth_limit"`   // 带宽限制 (KB/s), 0=不限制
	Concurrent       int  `json:"concurrent"`        // 并发传输数

	// 调度
	ScheduleType string `json:"schedule_type"` // manual/cron/realtime
	CronExpr     string `json:"cron_expr,omitempty"`

	// 统计
	LastSyncTime   *time.Time `json:"last_sync_time,omitempty"`
	LastSyncResult string     `json:"last_sync_result,omitempty"`
	TotalFiles     int64      `json:"total_files"`
	TotalSize      int64      `json:"total_size"`
	SyncedFiles    int64      `json:"synced_files"`
	SkippedFiles   int64      `json:"skipped_files"`
	FailedFiles    int64      `json:"failed_files"`
	ConflictFiles  int64      `json:"conflict_files"`
	LastError      string     `json:"last_error,omitempty"`
	ErrorCount     int        `json:"error_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate 验证同步任务
func (t *SyncTask) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if t.SourceDeviceID == "" {
		return fmt.Errorf("source device ID is required")
	}
	if t.TargetDeviceID == "" {
		return fmt.Errorf("target device ID is required")
	}
	if t.SourceDeviceID == t.TargetDeviceID {
		return fmt.Errorf("source and target device must be different")
	}
	if t.SourcePath == "" {
		return fmt.Errorf("source path is required")
	}
	if t.TargetPath == "" {
		return fmt.Errorf("target path is required")
	}
	if !t.Mode.IsValid() {
		return fmt.Errorf("invalid sync mode: %s", t.Mode)
	}
	if !t.ConflictStrategy.IsValid() {
		return fmt.Errorf("invalid conflict strategy: %s", t.ConflictStrategy)
	}
	return nil
}

// ============================================================
// 同步状态
// ============================================================

// SyncStatus 同步状态
type SyncStatus struct {
	TaskID         string      `json:"task_id"`
	TaskName       string      `json:"task_name"`
	Status         TaskStatus  `json:"status"`
	Progress       float64     `json:"progress"` // 0-100
	CurrentFile    string      `json:"current_file"`
	CurrentAction  string      `json:"current_action"` // sync/delete/conflict
	TotalFiles     int64       `json:"total_files"`
	ProcessedFiles int64       `json:"processed_files"`
	SyncedFiles    int64       `json:"synced_files"`
	FailedFiles    int64       `json:"failed_files"`
	ConflictFiles  int64       `json:"conflict_files"`
	SkippedFiles   int64       `json:"skipped_files"`
	TotalBytes     int64       `json:"total_bytes"`
	SyncedBytes    int64       `json:"synced_bytes"`
	SpeedBps       int64       `json:"speed_bps"`   // 当前速度 (bytes/sec)
	ETASeconds     int         `json:"eta_seconds"` // 预计剩余秒数
	StartedAt      *time.Time  `json:"started_at,omitempty"`
	EstimatedEnd   *time.Time  `json:"estimated_end,omitempty"`
	Errors         []SyncError `json:"errors,omitempty"`
}

// SyncError 同步错误
type SyncError struct {
	FilePath  string    `json:"file_path"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// ============================================================
// 同步日志
// ============================================================

// SyncLog 同步日志条目
type SyncLog struct {
	TaskID       string    `json:"task_id"`
	Timestamp    time.Time `json:"timestamp"`
	Level        string    `json:"level"` // info/warn/error
	Message      string    `json:"message"`
	FilePath     string    `json:"file_path,omitempty"`
	SourceDevice string    `json:"source_device,omitempty"`
	TargetDevice string    `json:"target_device,omitempty"`
}

// ============================================================
// 同步统计
// ============================================================

// SyncStats 同步统计汇总
type SyncStats struct {
	TotalDevices   int64     `json:"total_devices"`
	OnlineDevices  int64     `json:"online_devices"`
	TotalTasks     int64     `json:"total_tasks"`
	ActiveTasks    int64     `json:"active_tasks"`
	PausedTasks    int64     `json:"paused_tasks"`
	FailedTasks    int64     `json:"failed_tasks"`
	TotalFiles     int64     `json:"total_files"`
	TotalSize      int64     `json:"total_size"`
	SyncedFiles    int64     `json:"synced_files"`
	FailedFiles    int64     `json:"failed_files"`
	ConflictFiles  int64     `json:"conflict_files"`
	TotalBandwidth int       `json:"total_bandwidth"` // KB/s
	LastSyncTime   time.Time `json:"last_sync_time,omitempty"`
}

// ============================================================
// 创建/更新请求
// ============================================================

// CreateDeviceRequest 创建设备请求
type CreateDeviceRequest struct {
	Name     string `json:"name" binding:"required"`
	Address  string `json:"address" binding:"required"`
	Port     int    `json:"port" binding:"required"`
	APIKey   string `json:"api_key,omitempty"`
	Platform string `json:"platform,omitempty"`
}

// UpdateDeviceRequest 更新设备请求
type UpdateDeviceRequest struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
	Port    *int   `json:"port,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
}

// CreateSyncTaskRequest 创建同步任务请求
type CreateSyncTaskRequest struct {
	Name             string           `json:"name" binding:"required"`
	SourceDeviceID   string           `json:"source_device_id" binding:"required"`
	TargetDeviceID   string           `json:"target_device_id" binding:"required"`
	SourcePath       string           `json:"source_path" binding:"required"`
	TargetPath       string           `json:"target_path" binding:"required"`
	Mode             SyncMode         `json:"mode" binding:"required"`
	ConflictStrategy ConflictStrategy `json:"conflict_strategy"`
	IncludePatterns  []string         `json:"include_patterns,omitempty"`
	ExcludePatterns  []string         `json:"exclude_patterns,omitempty"`
	MaxFileSize      int64            `json:"max_file_size,omitempty"`
	MinFileSize      int64            `json:"min_file_size,omitempty"`
	PreserveModTime  *bool            `json:"preserve_mod_time,omitempty"`
	PreservePerms    *bool            `json:"preserve_perms,omitempty"`
	DeleteExtraneous *bool            `json:"delete_extraneous,omitempty"`
	ChecksumVerify   *bool            `json:"checksum_verify,omitempty"`
	CompressTransfer *bool            `json:"compress_transfer,omitempty"`
	BandwidthLimit   int              `json:"bandwidth_limit,omitempty"`
	Concurrent       int              `json:"concurrent,omitempty"`
	ScheduleType     string           `json:"schedule_type,omitempty"`
	CronExpr         string           `json:"cron_expr,omitempty"`
}

// UpdateSyncTaskRequest 更新同步任务请求
type UpdateSyncTaskRequest struct {
	Name             string            `json:"name,omitempty"`
	SourcePath       string            `json:"source_path,omitempty"`
	TargetPath       string            `json:"target_path,omitempty"`
	Mode             *SyncMode         `json:"mode,omitempty"`
	ConflictStrategy *ConflictStrategy `json:"conflict_strategy,omitempty"`
	IncludePatterns  []string          `json:"include_patterns,omitempty"`
	ExcludePatterns  []string          `json:"exclude_patterns,omitempty"`
	MaxFileSize      *int64            `json:"max_file_size,omitempty"`
	MinFileSize      *int64            `json:"min_file_size,omitempty"`
	PreserveModTime  *bool             `json:"preserve_mod_time,omitempty"`
	PreservePerms    *bool             `json:"preserve_perms,omitempty"`
	DeleteExtraneous *bool             `json:"delete_extraneous,omitempty"`
	ChecksumVerify   *bool             `json:"checksum_verify,omitempty"`
	CompressTransfer *bool             `json:"compress_transfer,omitempty"`
	BandwidthLimit   *int              `json:"bandwidth_limit,omitempty"`
	Concurrent       *int              `json:"concurrent,omitempty"`
	ScheduleType     string            `json:"schedule_type,omitempty"`
	CronExpr         string            `json:"cron_expr,omitempty"`
	Enabled          *bool             `json:"enabled,omitempty"`
}

// ResolveConflictRequest 解决冲突请求
type ResolveConflictRequest struct {
	Resolution ConflictStrategy `json:"resolution" binding:"required"` // source/target/newer/larger/keep_both
}

// ============================================================
// 设备连接测试结果
// ============================================================

// ConnectionTestResult 连接测试结果
type ConnectionTestResult struct {
	Success bool   `json:"success"`
	Latency int64  `json:"latency_ms"` // 延迟毫秒
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}
