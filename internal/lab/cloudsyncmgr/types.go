// Package cloudsyncmgr provides multi-cloud synchronization management.
// Reference: Synology Cloud Sync feature set.
package cloudsyncmgr

import (
	"fmt"
	"time"
)

// 常量别名 - 兼容 manager.go 引用.
const (
	ProviderS3       = ProviderType("s3")
	ProviderOSS      = ProviderType("oss")
	ProviderB2       = ProviderType("b2")
	ProviderOneDrive = ProviderType("onedrive")

	DirectionUpload   = SyncDirection("upload")
	DirectionDownload = SyncDirection("download")
	DirectionBiSync   = SyncDirection("bisync")

	StatusIdle     = TaskStatus("idle")
	StatusSyncing  = TaskStatus("syncing")
	StatusPaused   = TaskStatus("paused")
	StatusError    = TaskStatus("error")
	StatusComplete = TaskStatus("completed")
)

// ConflictPolicy 冲突解决策略.
type ConflictPolicy string

const (
	ConflictLocalWins  ConflictPolicy = "local_wins"  // 本地优先
	ConflictRemoteWins ConflictPolicy = "remote_wins" // 云端优先
	ConflictNewest     ConflictPolicy = "newest"      // 最新优先
	ConflictRename     ConflictPolicy = "rename"      // 重命名保留两者
	ConflictManual     ConflictPolicy = "manual"      // 手动解决
)

// ScheduleMode 调度模式.
type ScheduleMode string

const (
	ScheduleManual   ScheduleMode = "manual"
	ScheduleInterval ScheduleMode = "interval" // 固定间隔
	ScheduleCron     ScheduleMode = "cron"     // Cron 表达式
	ScheduleRealtime ScheduleMode = "realtime" // 实时触发
)

// SyncConfig 同步任务配置.
type SyncConfig struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Enabled          bool              `json:"enabled"`
	Provider         ProviderType      `json:"provider"`
	ProviderConfig   map[string]string `json:"provider_config"` // 后端认证配置
	LocalPath        string            `json:"local_path"`
	RemotePath       string            `json:"remote_path"`
	Direction        SyncDirection     `json:"direction"`
	ConflictPolicy   ConflictPolicy    `json:"conflict_policy"`
	ScheduleMode     ScheduleMode      `json:"schedule_mode"`
	ScheduleInterval time.Duration     `json:"schedule_interval_sec,omitempty"` // interval 模式（JSON 序列化为秒数）
	ScheduleCron     string            `json:"schedule_cron,omitempty"`         // cron 模式
	BandwidthLimit   int64             `json:"bandwidth_limit"`                 // 字节/秒，0=不限
	EncryptInTransit bool              `json:"encrypt_in_transit"`
	FilterPatterns   []string          `json:"filter_patterns,omitempty"` // 排除模式
	MaxRetries       int               `json:"max_retries"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// Validate 校验配置合法性.
func (c *SyncConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("任务名称不能为空")
	}
	if c.Provider == "" {
		return fmt.Errorf("提供商类型不能为空")
	}
	if c.LocalPath == "" {
		return fmt.Errorf("本地路径不能为空")
	}
	if c.RemotePath == "" {
		return fmt.Errorf("远程路径不能为空")
	}
	switch c.Direction {
	case DirectionUpload, DirectionDownload, DirectionBiSync:
	default:
		return fmt.Errorf("无效的同步方向: %s", c.Direction)
	}
	switch c.ConflictPolicy {
	case ConflictLocalWins, ConflictRemoteWins, ConflictNewest, ConflictRename, ConflictManual:
	default:
		return fmt.Errorf("无效的冲突策略: %s", c.ConflictPolicy)
	}
	switch c.ScheduleMode {
	case ScheduleManual, ScheduleInterval, ScheduleCron, ScheduleRealtime:
	default:
		return fmt.Errorf("无效的调度模式: %s", c.ScheduleMode)
	}
	if c.ScheduleMode == ScheduleInterval && c.ScheduleInterval <= 0 {
		return fmt.Errorf("间隔模式需要设置间隔时间")
	}
	if c.ScheduleMode == ScheduleCron && c.ScheduleCron == "" {
		return fmt.Errorf("Cron 模式需要设置 Cron 表达式")
	}
	return nil
}

// SyncTask 运行中的同步任务.
type SyncTask struct {
	Config     SyncConfig    `json:"config"`
	Status     TaskStatus    `json:"status"`
	Progress   *SyncProgress `json:"progress,omitempty"`
	Error      string        `json:"error,omitempty"`
	LastSyncAt *time.Time    `json:"last_sync_at,omitempty"`
	NextSyncAt *time.Time    `json:"next_sync_at,omitempty"`
}

// SyncProgress 同步进度.
type SyncProgress struct {
	TotalFiles   int64      `json:"total_files"`
	SyncedFiles  int64      `json:"synced_files"`
	FailedFiles  int64      `json:"failed_files"`
	SkippedFiles int64      `json:"skipped_files"`
	TotalBytes   int64      `json:"total_bytes"`
	SyncedBytes  int64      `json:"synced_bytes"`
	CurrentFile  string     `json:"current_file"`
	TransferRate float64    `json:"transfer_rate"` // 字节/秒
	StartedAt    time.Time  `json:"started_at"`
	EstimatedETA *time.Time `json:"estimated_eta,omitempty"`
}

// Percent 返回同步完成百分比 (0-100).
func (p *SyncProgress) Percent() float64 {
	if p.TotalFiles == 0 {
		return 0
	}
	return float64(p.SyncedFiles+p.SkippedFiles) / float64(p.TotalFiles) * 100
}

// SyncStatus 同步状态汇总.
type SyncStatus struct {
	TaskID     string        `json:"task_id"`
	TaskName   string        `json:"task_name"`
	Status     TaskStatus    `json:"status"`
	Progress   *SyncProgress `json:"progress,omitempty"`
	Error      string        `json:"error,omitempty"`
	LastSyncAt *time.Time    `json:"last_sync_at,omitempty"`
}

// ConflictInfo 冲突信息.
type ConflictInfo struct {
	LocalPath     string         `json:"local_path"`
	RemotePath    string         `json:"remote_path"`
	LocalModTime  time.Time      `json:"local_mod_time"`
	RemoteModTime time.Time      `json:"remote_mod_time"`
	LocalSize     int64          `json:"local_size"`
	RemoteSize    int64          `json:"remote_size"`
	DetectedAt    time.Time      `json:"detected_at"`
	Resolution    ConflictPolicy `json:"resolution,omitempty"`
	Resolved      bool           `json:"resolved"`
}

// SyncEvent 同步事件 (用于进度追踪和日志).
type SyncEvent struct {
	TaskID    string    `json:"task_id"`
	Type      string    `json:"type"` // "start", "progress", "complete", "error", "conflict"
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
