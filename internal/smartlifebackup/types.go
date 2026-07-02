// Package smartlifebackup 提供备份生命周期管理功能
// 包括智能保留策略、存储成本优化、冷热数据分离等
package smartlifebackup

import (
	"time"
)

// ============================================================================
// 备份策略类型
// ============================================================================

// BackupPolicy 备份策略配置.
type BackupPolicy struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Enabled         bool            `json:"enabled"`
	RetentionRules  []RetentionRule `json:"retention_rules"`
	CompressionType CompressionType `json:"compression_type"`
	Deduplication   bool            `json:"deduplication"`
	ColdStorageAge  time.Duration   `json:"cold_storage_age"` // 超过此时间转为冷存储
	ArchiveAge      time.Duration   `json:"archive_age"`      // 超过此时间归档
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// RetentionRule 备份保留规则.
type RetentionRule struct {
	Name        string       `json:"name"`
	Priority    int          `json:"priority"`
	RetainDays  int          `json:"retain_days"`  // 保留天数
	KeepCount   int          `json:"keep_count"`   // 保留数量
	Interval    TimeInterval `json:"interval"`     // 保留间隔
	StorageTier StorageTier  `json:"storage_tier"` // 存储层级
	Compress    bool         `json:"compress"`     // 是否压缩
	Encrypt     bool         `json:"encrypt"`      // 是否加密
}

// TimeInterval 时间间隔类型.
type TimeInterval string

const (
	// TimeIntervalDaily 每日间隔.
	TimeIntervalDaily TimeInterval = "daily"
	// TimeIntervalWeekly 每周间隔.
	TimeIntervalWeekly TimeInterval = "weekly"
	// TimeIntervalMonthly 每月间隔.
	TimeIntervalMonthly TimeInterval = "monthly"
	// TimeIntervalYearly 每年间隔.
	TimeIntervalYearly TimeInterval = "yearly"
)

// StorageTier 存储层级.
type StorageTier string

const (
	// StorageTierHot 热存储 - 快速访问，成本较高.
	StorageTierHot StorageTier = "hot"
	// StorageTierWarm 温存储 - 中等访问速度，成本适中.
	StorageTierWarm StorageTier = "warm"
	// StorageTierCold 冷存储 - 慢速访问，成本最低.
	StorageTierCold StorageTier = "cold"
	// StorageTierArchive 归档存储 - 最低成本，需要时可检索.
	StorageTierArchive StorageTier = "archive"
)

// CompressionType 压缩类型.
type CompressionType string

const (
	// CompressionNone 不压缩.
	CompressionNone CompressionType = "none"
	// CompressionGzip Gzip压缩.
	CompressionGzip CompressionType = "gzip"
	// CompressionZstd Zstandard压缩 (高效).
	CompressionZstd CompressionType = "zstd"
	// CompressionLz4 LZ4压缩 (快速).
	CompressionLz4 CompressionType = "lz4"
)

// ============================================================================
// 备份任务相关类型
// ============================================================================

// BackupItem 备份项.
type BackupItem struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	SourcePath   string      `json:"source_path"`
	BackupPath   string      `json:"backup_path"`
	Size         int64       `json:"size"`
	CompressSize int64       `json:"compress_size"`
	Checksum     string      `json:"checksum"`
	Tier         StorageTier `json:"tier"`
	CreatedAt    time.Time   `json:"created_at"`
	ExpiresAt    time.Time   `json:"expires_at"`
	AccessCount  int64       `json:"access_count"`
	LastAccess   time.Time   `json:"last_access"`
	Compressed   bool        `json:"compressed"`
	Encrypted    bool        `json:"encrypted"`
	Deduplicated bool        `json:"deduplicated"`
}

// LifecycleTask 生命周期任务.
type LifecycleTask struct {
	ID         string                 `json:"id"`
	Type       TaskType               `json:"type"`
	Status     TaskStatus             `json:"status"`
	BackupID   string                 `json:"backup_id,omitempty"`
	SourceTier StorageTier            `json:"source_tier,omitempty"`
	TargetTier StorageTier            `json:"target_tier,omitempty"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Progress   int                    `json:"progress"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// TaskType 任务类型.
type TaskType string

const (
	// TaskTypeRetention 保留策略执行.
	TaskTypeRetention TaskType = "retention"
	// TaskTypeTierMigration 存储层级迁移.
	TaskTypeTierMigration TaskType = "tier_migration"
	// TaskTypeCompression 压缩任务.
	TaskTypeCompression TaskType = "compression"
	// TaskTypeDeduplication 去重任务.
	TaskTypeDeduplication TaskType = "deduplication"
	// TaskTypeArchive 归档任务.
	TaskTypeArchive TaskType = "archive"
	// TaskTypeCleanup 清理任务.
	TaskTypeCleanup TaskType = "cleanup"
)

// TaskStatus 任务状态.
type TaskStatus string

const (
	// TaskStatusPending 待执行.
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning 执行中.
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusCompleted 已完成.
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 失败.
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled 已取消.
	TaskStatusCancelled TaskStatus = "cancelled"
)

// ============================================================================
// 存储成本相关类型
// ============================================================================

// StorageCost 存储成本配置.
type StorageCost struct {
	// 各层级每GB每月成本
	HotCostPerGB     float64 `json:"hot_cost_per_gb"`
	WarmCostPerGB    float64 `json:"warm_cost_per_gb"`
	ColdCostPerGB    float64 `json:"cold_cost_per_gb"`
	ArchiveCostPerGB float64 `json:"archive_cost_per_gb"`

	// 传输成本（每GB）
	TransferCostPerGB float64 `json:"transfer_cost_per_gb"`

	// API调用成本
	RequestCostPer1000 float64 `json:"request_cost_per_1000"`
}

// CostReport 成本报告.
type CostReport struct {
	GeneratedAt time.Time `json:"generated_at"`
	Period      string    `json:"period"`

	// 各层级存储量和成本
	TierBreakdown []TierCost `json:"tier_breakdown"`

	// 总计
	TotalStorageGB float64 `json:"total_storage_gb"`
	TotalCost      float64 `json:"total_cost"`

	// 优化建议
	Suggestions []CostSuggestion `json:"suggestions,omitempty"`
}

// TierCost 各层级成本.
type TierCost struct {
	Tier        StorageTier `json:"tier"`
	StorageGB   float64     `json:"storage_gb"`
	Cost        float64     `json:"cost"`
	BackupCount int         `json:"backup_count"`
}

// CostSuggestion 成本优化建议.
type CostSuggestion struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Savings     float64 `json:"estimated_savings"`
	Priority    int     `json:"priority"`
}

// ============================================================================
// 调度相关类型
// ============================================================================

// ScheduleConfig 调度配置.
type ScheduleConfig struct {
	// 避开的高峰时段
	PeakHours []PeakHour `json:"peak_hours"`

	// 允许执行的时段
	AllowedWindows []TimeWindow `json:"allowed_windows"`

	// 最大并发任务数
	MaxConcurrent int `json:"max_concurrent"`

	// 任务优先级
	DefaultPriority int `json:"default_priority"`
}

// PeakHour 高峰时段.
type PeakHour struct {
	StartHour int   `json:"start_hour"` // 0-23
	EndHour   int   `json:"end_hour"`   // 0-23
	Days      []int `json:"days"`       // 0=周日, 1=周一, ..., 6=周六
}

// TimeWindow 时间窗口.
type TimeWindow struct {
	StartHour int `json:"start_hour"`
	EndHour   int `json:"end_hour"`
	Priority  int `json:"priority"`
}

// ============================================================================
// 统计和监控类型
// ============================================================================

// LifecycleStats 生命周期统计.
type LifecycleStats struct {
	TotalBackups       int64                 `json:"total_backups"`
	ActiveBackups      int64                 `json:"active_backups"`
	ExpiredBackups     int64                 `json:"expired_backups"`
	TierDistribution   map[StorageTier]int64 `json:"tier_distribution"`
	TotalSizeGB        float64               `json:"total_size_gb"`
	CompressionRatio   float64               `json:"compression_ratio"`
	DeduplicationRatio float64               `json:"deduplication_ratio"`
	TasksProcessed     int64                 `json:"tasks_processed"`
	LastCleanupTime    *time.Time            `json:"last_cleanup_time,omitempty"`
}

// ============================================================================
// API请求/响应类型
// ============================================================================

// LifecycleRequest 生命周期API请求.
type LifecycleRequest struct {
	Action    string          `json:"action"` // create, update, execute, dry_run
	Policy    *BackupPolicy   `json:"policy,omitempty"`
	BackupIDs []string        `json:"backup_ids,omitempty"`
	Options   *ExecuteOptions `json:"options,omitempty"`
}

// LifecycleResponse 生命周期API响应.
type LifecycleResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	TaskID  string      `json:"task_id,omitempty"`
}

// ExecuteOptions 执行选项.
type ExecuteOptions struct {
	DryRun    bool          `json:"dry_run"`
	ForceTier *StorageTier  `json:"force_tier,omitempty"`
	MaxItems  int           `json:"max_items"`
	Timeout   time.Duration `json:"timeout"`
}

// ============================================================================
// 辅助方法
// ============================================================================

// DefaultBackupPolicy 返回默认备份策略.
func DefaultBackupPolicy() *BackupPolicy {
	now := time.Now()
	return &BackupPolicy{
		ID:              "default",
		Name:            "默认备份策略",
		Description:     "智能备份生命周期管理 - 7天内每日、30天内每周、12月内每月、1年以上归档",
		Enabled:         true,
		CompressionType: CompressionGzip,
		Deduplication:   true,
		ColdStorageAge:  30 * 24 * time.Hour,  // 30天后转冷存储
		ArchiveAge:      365 * 24 * time.Hour, // 1年后归档
		CreatedAt:       now,
		UpdatedAt:       now,
		RetentionRules: []RetentionRule{
			{
				Name:        "7天内每日保留",
				Priority:    1,
				RetainDays:  7,
				KeepCount:   7,
				Interval:    TimeIntervalDaily,
				StorageTier: StorageTierHot,
				Compress:    true,
				Encrypt:     false,
			},
			{
				Name:        "30天内每周保留",
				Priority:    2,
				RetainDays:  30,
				KeepCount:   4,
				Interval:    TimeIntervalWeekly,
				StorageTier: StorageTierWarm,
				Compress:    true,
				Encrypt:     false,
			},
			{
				Name:        "12月内每月保留",
				Priority:    3,
				RetainDays:  365,
				KeepCount:   12,
				Interval:    TimeIntervalMonthly,
				StorageTier: StorageTierCold,
				Compress:    true,
				Encrypt:     true,
			},
			{
				Name:        "1年以上归档",
				Priority:    4,
				RetainDays:  0, // 0 表示永久保留
				KeepCount:   0,
				Interval:    TimeIntervalYearly,
				StorageTier: StorageTierArchive,
				Compress:    true,
				Encrypt:     true,
			},
		},
	}
}

// DefaultStorageCost 返回默认存储成本配置.
func DefaultStorageCost() *StorageCost {
	return &StorageCost{
		HotCostPerGB:       0.023,   // $0.023/GB/月
		WarmCostPerGB:      0.0125,  // $0.0125/GB/月
		ColdCostPerGB:      0.004,   // $0.004/GB/月
		ArchiveCostPerGB:   0.00099, // $0.00099/GB/月
		TransferCostPerGB:  0.09,    // $0.09/GB
		RequestCostPer1000: 0.005,   // $0.005/1000 requests
	}
}

// DefaultScheduleConfig 返回默认调度配置.
func DefaultScheduleConfig() *ScheduleConfig {
	return &ScheduleConfig{
		PeakHours: []PeakHour{
			{
				StartHour: 9,
				EndHour:   18,
				Days:      []int{1, 2, 3, 4, 5}, // 工作日
			},
		},
		AllowedWindows: []TimeWindow{
			{
				StartHour: 22,
				EndHour:   6,
				Priority:  1, // 最高优先级时段
			},
			{
				StartHour: 0,
				EndHour:   6,
				Priority:  2,
			},
		},
		MaxConcurrent:   3,
		DefaultPriority: 5,
	}
}
