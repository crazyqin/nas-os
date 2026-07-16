// Package storagemigration 提供存储迁移助手功能，支持从群晖、TrueNAS 等系统迁移数据。
package storagemigration

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrMigrationRunning 表示迁移正在进行中。
	ErrMigrationRunning = errors.New("迁移正在进行中")
	// ErrNoMigration 表示没有活跃的迁移任务。
	ErrNoMigration = errors.New("没有活跃的迁移任务")
	// ErrInvalidSource 表示无效的源系统。
	ErrInvalidSource = errors.New("无效的源系统类型")
)

// ========== 源系统类型 ==========

// SourceSystem 源系统类型。
type SourceSystem string

const (
	// SourceSynology 群晖 DSM。
	SourceSynology SourceSystem = "synology"
	// SourceTrueNAS TrueNAS。
	SourceTrueNAS SourceSystem = "truenas"
	// SourceQNAP 威联通 QTS。
	SourceQNAP SourceSystem = "qnap"
	// SourceUnraid Unraid。
	SourceUnraid SourceSystem = "unraid"
	// SourceGeneric 通用 rsync/SCP 迁移。
	SourceGeneric SourceSystem = "generic"
)

// AllSources 返回所有支持的源系统。
func AllSources() []SourceSystem {
	return []SourceSystem{
		SourceSynology,
		SourceTrueNAS,
		SourceQNAP,
		SourceUnraid,
		SourceGeneric,
	}
}

// ========== 迁移配置 ==========

// MigrationConfig 迁移配置。
type MigrationConfig struct {
	Source     SourceSystem   `json:"source"`
	SourceHost string         `json:"source_host"`
	SourcePort int            `json:"source_port"`
	SourceUser string         `json:"source_user"`
	SourcePath string         `json:"source_path"`
	DestPath   string         `json:"dest_path"`
	Options    MigrateOptions `json:"options"`
}

// MigrateOptions 迁移选项。
type MigrateOptions struct {
	PreserveACL        bool     `json:"preserve_acl"`
	PreserveXattr      bool     `json:"preserve_xattr"`
	PreserveTimestamps bool     `json:"preserve_timestamps"`
	VerifyChecksum     bool     `json:"verify_checksum"`
	BandwidthLimit     int      `json:"bandwidth_limit_mbps"`
	SyncMode           string   `json:"sync_mode"` // "copy", "mirror", "incremental"
	DryRun             bool     `json:"dry_run"`
	ExcludePatterns    []string `json:"exclude_patterns,omitempty"`
}

// DefaultOptions 返回默认迁移选项。
func DefaultOptions() MigrateOptions {
	return MigrateOptions{
		PreserveACL:        true,
		PreserveXattr:      true,
		PreserveTimestamps: true,
		VerifyChecksum:     true,
		BandwidthLimit:     0, // 不限速
		SyncMode:           "copy",
		DryRun:             false,
	}
}

// ========== 迁移状态 ==========

// MigrationStatus 迁移状态。
type MigrationStatus string

const (
	// StatusPending 等待开始。
	StatusPending MigrationStatus = "pending"
	// StatusScanning 扫描源数据。
	StatusScanning MigrationStatus = "scanning"
	// StatusMigrating 迁移中。
	StatusMigrating MigrationStatus = "migrating"
	// StatusVerifying 验证中。
	StatusVerifying MigrationStatus = "verifying"
	// StatusCompleted 完成。
	StatusCompleted MigrationStatus = "completed"
	// StatusFailed 失败。
	StatusFailed MigrationStatus = "failed"
	// StatusCancelled 已取消。
	StatusCancelled MigrationStatus = "cancelled"
)

// ========== 迁移任务 ==========

// MigrationTask 迁移任务。
type MigrationTask struct {
	ID         string          `json:"id"`
	Config     MigrationConfig `json:"config"`
	Status     MigrationStatus `json:"status"`
	Progress   float64         `json:"progress"`
	TotalFiles int64           `json:"total_files"`
	TotalBytes int64           `json:"total_bytes"`
	DoneFiles  int64           `json:"done_files"`
	DoneBytes  int64           `json:"done_bytes"`
	Speed      float64         `json:"speed_mbps"`
	ETA        time.Duration   `json:"eta"`
	ErrorMsg   string          `json:"error,omitempty"`
	StartedAt  time.Time       `json:"started_at"`
	EndedAt    *time.Time      `json:"ended_at,omitempty"`
	Log        []string        `json:"log,omitempty"`
}

// MigrationReport 迁移报告。
type MigrationReport struct {
	TaskID       string        `json:"task_id"`
	Source       SourceSystem  `json:"source"`
	SourceHost   string        `json:"source_host"`
	TotalFiles   int64         `json:"total_files"`
	TotalBytes   int64         `json:"total_bytes"`
	SuccessFiles int64         `json:"success_files"`
	FailedFiles  int64         `json:"failed_files"`
	SkippedFiles int64         `json:"skipped_files"`
	Duration     time.Duration `json:"duration"`
	AvgSpeed     float64       `json:"avg_speed_mbps"`
	ChecksumOK   bool          `json:"checksum_ok"`
	StartedAt    time.Time     `json:"started_at"`
	EndedAt      time.Time     `json:"ended_at"`
	Warnings     []string      `json:"warnings,omitempty"`
	Errors       []string      `json:"errors,omitempty"`
}
