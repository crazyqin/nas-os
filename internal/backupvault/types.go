// Package backupvault 提供 3-2-1 备份策略管理功能。
// 支持备份计划调度、备份链管理、数据加密、备份验证和恢复演练。
package backupvault

import "time"

// BackupStrategy 备份策略类型
type BackupStrategy string

const (
	StrategyFull        BackupStrategy = "full"        // 全量备份
	StrategyIncremental BackupStrategy = "incremental" // 增量备份
	StrategyDifferential BackupStrategy = "differential" // 差异备份
)

// BackupStatus 备份任务状态
type BackupStatus string

const (
	StatusIdle     BackupStatus = "idle"
	StatusRunning  BackupStatus = "running"
	StatusSuccess  BackupStatus = "success"
	StatusFailed   BackupStatus = "failed"
	StatusPaused   BackupStatus = "paused"
)

// MediaType 存储介质类型
type MediaType string

const (
	MediaLocal    MediaType = "local"    // 本地磁盘
	MediaNAS      MediaType = "nas"      // NAS 存储
	MediaCloud    MediaType = "cloud"    // 云存储
	MediaTape     MediaType = "tape"     // 磁带
	MediaRemote   MediaType = "remote"   // 远程服务器
)

// ScheduleType 调度类型
type ScheduleType string

const (
	ScheduleOnce     ScheduleType = "once"     // 一次性
	ScheduleHourly   ScheduleType = "hourly"   // 每小时
	ScheduleDaily    ScheduleType = "daily"    // 每天
	ScheduleWeekly   ScheduleType = "weekly"   // 每周
	ScheduleMonthly  ScheduleType = "monthly"  // 每月
)

// VaultConfig 备份保险箱配置
type VaultConfig struct {
	Name             string `json:"name"`
	DataDir          string `json:"data_dir"`           // 数据存储目录
	EncryptionKey    string `json:"encryption_key"`     // AES-256 加密密钥（32字节）
	MaxConcurrent    int    `json:"max_concurrent"`     // 最大并发备份数
	VerifyAfterBackup bool   `json:"verify_after_backup"` // 备份后自动验证
	RetentionDays    int    `json:"retention_days"`     // 默认保留天数
}

// DefaultVaultConfig 默认配置
func DefaultVaultConfig() *VaultConfig {
	return &VaultConfig{
		Name:              "BackupVault",
		DataDir:           "/var/lib/backupvault",
		MaxConcurrent:     3,
		VerifyAfterBackup: true,
		RetentionDays:     30,
	}
}

// BackupJob 备份任务
type BackupJob struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Source      string           `json:"source"`           // 源路径
	Destinations []Destination   `json:"destinations"`     // 目标存储列表（3-2-1 策略）
	Schedule    *Schedule        `json:"schedule,omitempty"`
	Strategy    BackupStrategy   `json:"strategy"`         // 全量/增量/差异
	Encryption  *EncryptionConfig `json:"encryption,omitempty"`
	Retention   *RetentionPolicy  `json:"retention,omitempty"`
	Status      BackupStatus     `json:"status"`
	LastRun     time.Time        `json:"last_run,omitempty"`
	NextRun     time.Time        `json:"next_run,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// Destination 备份目标存储
type Destination struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      MediaType `json:"type"`       // 存储介质类型
	Path      string    `json:"path"`       // 存储路径
	IsOffsite bool      `json:"is_offsite"` // 是否异地存储
	Enabled   bool      `json:"enabled"`
}

// Schedule 备份调度配置
type Schedule struct {
	Type     ScheduleType `json:"type"`               // 调度类型
	Interval int          `json:"interval,omitempty"`  // 间隔（小时/天/周/月）
	Time     string       `json:"time,omitempty"`      // 执行时间 HH:MM
	DayOfWeek int         `json:"day_of_week,omitempty"` // 星期几（0=周日）
	DayOfMonth int        `json:"day_of_month,omitempty"` // 每月几号
	Enabled  bool         `json:"enabled"`
}

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"` // 加密算法，如 "aes-256-gcm"
	KeyID     string `json:"key_id"`   // 密钥标识
}

// DefaultEncryptionConfig 默认加密配置
func DefaultEncryptionConfig() *EncryptionConfig {
	return &EncryptionConfig{
		Enabled:   true,
		Algorithm: "aes-256-gcm",
	}
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	KeepLast    int `json:"keep_last"`    // 保留最近N个备份
	KeepDaily   int `json:"keep_daily"`   // 保留最近N天的每日备份
	KeepWeekly  int `json:"keep_weekly"`  // 保留最近N周的每周备份
	KeepMonthly int `json:"keep_monthly"` // 保留最近N月的每月备份
	KeepYearly  int `json:"keep_yearly"`  // 保留最近N年的每年备份
	MaxAgeDays  int `json:"max_age_days"` // 最大保留天数
}

// DefaultRetentionPolicy 默认保留策略
func DefaultRetentionPolicy() *RetentionPolicy {
	return &RetentionPolicy{
		KeepLast:    10,
		KeepDaily:   7,
		KeepWeekly:  4,
		KeepMonthly: 6,
		KeepYearly:  2,
		MaxAgeDays:  365,
	}
}

// BackupChain 备份链（全量 + 增量/差异）
type BackupChain struct {
	ID            string          `json:"id"`
	JobID         string          `json:"job_id"`
	FullBackup    *RestorePoint   `json:"full_backup"`              // 全量备份点
	Incrementals  []*RestorePoint `json:"incrementals,omitempty"`   // 增量/差异备份点
	TotalSize     int64           `json:"total_size"`               // 链总大小
	ChainLength   int             `json:"chain_length"`             // 链长度
	CreatedAt     time.Time       `json:"created_at"`
}

// RestorePoint 恢复点
type RestorePoint struct {
	ID          string    `json:"id"`
	JobID       string    `json:"job_id"`
	ChainID     string    `json:"chain_id,omitempty"`
	Strategy    BackupStrategy `json:"strategy"`          // 此恢复点使用的策略
	Timestamp   time.Time `json:"timestamp"`
	Size        int64     `json:"size"`                // 数据大小（字节）
	Checksum    string    `json:"checksum"`            // SHA-256 校验和
	Verified    bool      `json:"verified"`            // 是否已验证
	Encrypted   bool      `json:"encrypted"`           // 是否已加密
	Destination string    `json:"destination"`          // 存储位置
	CreatedAt   time.Time `json:"created_at"`
}

// BackupResult 备份执行结果
type BackupResult struct {
	JobID        string        `json:"job_id"`
	RestorePoint *RestorePoint `json:"restore_point"`
	Strategy     BackupStrategy `json:"strategy"`
	Size         int64         `json:"size"`
	Duration     time.Duration `json:"duration"`
	Verified     bool          `json:"verified"`
	Error        string        `json:"error,omitempty"`
}

// ComplianceReport 合规报告（3-2-1 策略检查）
type ComplianceReport struct {
	GeneratedAt    time.Time          `json:"generated_at"`
	TotalJobs      int                `json:"total_jobs"`
	CompliantJobs  int                `json:"compliant_jobs"`
	NonCompliant   int                `json:"non_compliant"`
	Violations     []Violation        `json:"violations,omitempty"`
	Summary        *ComplianceSummary `json:"summary"`
}

// Violation 合规违规项
type Violation struct {
	JobID       string `json:"job_id"`
	JobName     string `json:"job_name"`
	Rule        string `json:"rule"`         // 违反的规则
	Description string `json:"description"`  // 违规描述
	Severity    string `json:"severity"`     // 严重程度: low/medium/high/critical
}

// ComplianceSummary 合规摘要
type ComplianceSummary struct {
	TotalCopies       int  `json:"total_copies"`        // 总副本数
	HasTwoMedia       bool `json:"has_two_media"`       // 是否有2种介质
	HasOffsite        bool `json:"has_offsite"`         // 是否有异地备份
	LastVerified      time.Time `json:"last_verified"`   // 最后验证时间
	TotalRestorePoints int   `json:"total_restore_points"` // 总恢复点数
}

// RestoreRequest 恢复请求
type RestoreRequest struct {
	RestorePointID string `json:"restore_point_id"`
	Destination    string `json:"destination"`    // 恢复目标路径
	Overwrite      bool   `json:"overwrite"`      // 是否覆盖已有文件
	DryRun         bool   `json:"dry_run"`        // 试运行
}
