// Package smartdatalifecycle - 智能数据生命周期管理模块
// 自动归档策略、智能清理、数据迁移、保留策略管理
// 基于访问频率、时间、合规要求的全生命周期数据管理
package smartdatalifecycle

import (
	"time"
)

// ============================================================
// 数据生命周期阶段
// ============================================================

// LifecycleStage 生命周期阶段
type LifecycleStage string

const (
	StageActive   LifecycleStage = "active"   // 活跃阶段
	StageWarm     LifecycleStage = "warm"     // 温数据阶段
	StageCold     LifecycleStage = "cold"     // 冷数据阶段
	StageArchive  LifecycleStage = "archive"  // 归档阶段
	StageExpired  LifecycleStage = "expired"  // 过期阶段
	StageDeleted  LifecycleStage = "deleted"  // 已删除阶段
)

// String 返回阶段名称
func (s LifecycleStage) String() string {
	return string(s)
}

// ParseStage 解析阶段名称
func ParseStage(name string) LifecycleStage {
	switch LifecycleStage(name) {
	case StageActive:
		return StageActive
	case StageWarm:
		return StageWarm
	case StageCold:
		return StageCold
	case StageArchive:
		return StageArchive
	case StageExpired:
		return StageExpired
	case StageDeleted:
		return StageDeleted
	default:
		return StageActive
	}
}

// ============================================================
// 数据分类
// ============================================================

// DataClassification 数据分类
type DataClassification string

const (
	ClassificationPublic       DataClassification = "public"        // 公开数据
	ClassificationInternal     DataClassification = "internal"      // 内部数据
	ClassificationConfidential DataClassification = "confidential"  // 机密数据
	ClassificationRestricted   DataClassification = "restricted"    // 受限数据
)

// ============================================================
// 保留策略
// ============================================================

// RetentionAction 保留到期后的动作
type RetentionAction string

const (
	ActionArchive   RetentionAction = "archive"   // 归档
	ActionDelete    RetentionAction = "delete"     // 删除
	ActionMigrate   RetentionAction = "migrate"    // 迁移
	ActionNotify    RetentionAction = "notify"     // 通知
	ActionFreeze    RetentionAction = "freeze"     // 冻结（不可变）
)

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Classification DataClassification `json:"classification"`
	// 保留期限
	RetentionDays  int              `json:"retention_days"`   // 保留天数
	// 到期动作
	ExpirationAction RetentionAction `json:"expiration_action"`
	// 归档层级（如果动作为归档）
	ArchiveTier    string           `json:"archive_tier,omitempty"`
	// 是否可法律冻结
	LegalHoldEnabled bool           `json:"legal_hold_enabled"`
	// 是否合规策略（不可提前删除）
	CompliancePolicy bool           `json:"compliance_policy"`
	// 创建/更新时间
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	// 适用的文件模式（glob）
	FilePatterns   []string         `json:"file_patterns,omitempty"`
	// 适用的路径前缀
	PathPrefixes   []string         `json:"path_prefixes,omitempty"`
}

// ============================================================
// 归档策略
// ============================================================

// ArchiveTrigger 归档触发条件
type ArchiveTrigger string

const (
	TriggerAccessFrequency ArchiveTrigger = "access_frequency" // 基于访问频率
	TriggerLastAccessTime  ArchiveTrigger = "last_access_time" // 基于最后访问时间
	TriggerAge             ArchiveTrigger = "age"              // 基于文件年龄
	TriggerSize            ArchiveTrigger = "size"             // 基于文件大小
	TriggerCombined        ArchiveTrigger = "combined"         // 组合条件
)

// ArchivePolicy 归档策略
type ArchivePolicy struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Enabled        bool           `json:"enabled"`
	Trigger        ArchiveTrigger `json:"trigger"`
	// 触发阈值
	MaxAccessCount    int       `json:"max_access_count,omitempty"`    // 最大访问次数（低于此值触发）
	DaysSinceAccess   int       `json:"days_since_access,omitempty"`   // 距最后访问天数
	FileAgeDays       int       `json:"file_age_days,omitempty"`       // 文件年龄（天）
	MinFileSizeBytes  int64     `json:"min_file_size_bytes,omitempty"` // 最小文件大小
	MaxFileSizeBytes  int64     `json:"max_file_size_bytes,omitempty"` // 最大文件大小
	// 目标层级
	TargetStage   LifecycleStage `json:"target_stage"`
	// 执行计划
	Schedule      string         `json:"schedule,omitempty"` // cron 表达式
	// 适用范围
	FilePatterns  []string       `json:"file_patterns,omitempty"`
	PathPrefixes  []string       `json:"path_prefixes,omitempty"`
	ExcludePatterns []string     `json:"exclude_patterns,omitempty"`
	// 统计
	LastRunAt     *time.Time     `json:"last_run_at,omitempty"`
	TotalArchived int64          `json:"total_archived"`
	TotalBytes    int64          `json:"total_bytes"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// ============================================================
// 数据项元数据
// ============================================================

// DataItem 数据项
type DataItem struct {
	ID             string             `json:"id"`
	Path           string             `json:"path"`
	Name           string             `json:"name"`
	Size           int64              `json:"size"`           // bytes
	ContentType    string             `json:"content_type"`
	Stage          LifecycleStage     `json:"stage"`
	Classification DataClassification `json:"classification"`
	// 时间戳
	CreatedAt      time.Time          `json:"created_at"`
	ModifiedAt     time.Time          `json:"modified_at"`
	AccessedAt     time.Time          `json:"accessed_at"`
	ArchivedAt     *time.Time         `json:"archived_at,omitempty"`
	ExpiresAt      *time.Time         `json:"expires_at,omitempty"`
	DeletedAt      *time.Time         `json:"deleted_at,omitempty"`
	// 访问统计
	AccessCount    int64              `json:"access_count"`
	ReadCount      int64              `json:"read_count"`
	WriteCount     int64              `json:"write_count"`
	// Hash (用于重复检测)
	ContentHash    string             `json:"content_hash,omitempty"`
	// 关联的保留策略
	RetentionPolicyID string          `json:"retention_policy_id,omitempty"`
	// 法律冻结
	LegalHold      bool               `json:"legal_hold"`
	// 标签
	Tags           []string           `json:"tags,omitempty"`
	// 元数据
	Metadata       map[string]string  `json:"metadata,omitempty"`
}

// ============================================================
// 重复数据检测
// ============================================================

// DuplicateGroup 重复数据组
type DuplicateGroup struct {
	ID          string      `json:"id"`
	ContentHash string      `json:"content_hash"`
	Items       []*DataItem `json:"items"`
	TotalSize   int64       `json:"total_size"`     // 总占用空间
	WastedSize  int64       `json:"wasted_size"`    // 浪费的空间
	FirstFound  time.Time   `json:"first_found"`
	LastChecked time.Time   `json:"last_checked"`
}

// DeduplicationResult 去重结果
type DeduplicationResult struct {
	ScannedItems   int       `json:"scanned_items"`
	DuplicateGroups int      `json:"duplicate_groups"`
	TotalDuplicates int      `json:"total_duplicates"`
	WastedSpace    int64     `json:"wasted_space"`     // bytes
	ReclaimedSpace int64     `json:"reclaimed_space"`  // bytes
	ProcessedAt    time.Time `json:"processed_at"`
	Errors         []string  `json:"errors,omitempty"`
}

// ============================================================
// 清理策略
// ============================================================

// CleanupRule 清理规则
type CleanupRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Enabled     bool          `json:"enabled"`
	RuleType    CleanupRuleType `json:"rule_type"`
	// 过期清理配置
	ExpireDays    int           `json:"expire_days,omitempty"`
	// 临时文件清理
	TempFilePatterns []string  `json:"temp_file_patterns,omitempty"`
	// 大文件阈值
	MaxFileAgeDays   int       `json:"max_file_age_days,omitempty"`
	// 空目录清理
	RemoveEmptyDirs  bool      `json:"remove_empty_dirs,omitempty"`
	// 日志清理
	LogRetentionDays int       `json:"log_retention_days,omitempty"`
	// 执行计划
	Schedule      string        `json:"schedule,omitempty"`
	// 统计
	LastRunAt     *time.Time    `json:"last_run_at,omitempty"`
	TotalCleaned  int64         `json:"total_cleaned"`
	TotalFreed    int64         `json:"total_freed"`     // bytes
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// CleanupRuleType 清理规则类型
type CleanupRuleType string

const (
	RuleTypeExpired     CleanupRuleType = "expired"      // 过期数据清理
	RuleTypeTempFiles   CleanupRuleType = "temp_files"   // 临时文件清理
	RuleTypeDuplicates  CleanupRuleType = "duplicates"   // 重复数据清理
	RuleTypeEmptyDirs   CleanupRuleType = "empty_dirs"   // 空目录清理
	RuleTypeOldLogs     CleanupRuleType = "old_logs"     // 旧日志清理
	RuleTypeTrash       CleanupRuleType = "trash"        // 回收站清理
)

// CleanupResult 清理结果
type CleanupResult struct {
	RuleID       string    `json:"rule_id"`
	ItemsDeleted int       `json:"items_deleted"`
	SpaceFreed   int64     `json:"space_freed"` // bytes
	Errors       []string  `json:"errors,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Duration     time.Duration `json:"duration"`
}

// ============================================================
// 数据迁移
// ============================================================

// MigrationTask 迁移任务
type MigrationTask struct {
	ID           string          `json:"id"`
	SourcePath   string          `json:"source_path"`
	TargetPath   string          `json:"target_path"`
	SourceStage  LifecycleStage  `json:"source_stage"`
	TargetStage  LifecycleStage  `json:"target_stage"`
	Size         int64           `json:"size"`
	Status       MigrationStatus `json:"status"`
	Reason       string          `json:"reason"`
	Error        string          `json:"error,omitempty"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	RetryCount   int             `json:"retry_count"`
	CreatedAt    time.Time       `json:"created_at"`
}

// MigrationStatus 迁移状态
type MigrationStatus string

const (
	MigrationPending   MigrationStatus = "pending"
	MigrationRunning   MigrationStatus = "running"
	MigrationCompleted MigrationStatus = "completed"
	MigrationFailed    MigrationStatus = "failed"
	MigrationCancelled MigrationStatus = "cancelled"
)

// ============================================================
// 事件与审计
// ============================================================

// LifecycleEvent 生命周期事件
type LifecycleEvent struct {
	ID          string          `json:"id"`
	EventType   EventType       `json:"event_type"`
	ItemID      string          `json:"item_id"`
	ItemPath    string          `json:"item_path"`
	OldStage    LifecycleStage  `json:"old_stage,omitempty"`
	NewStage    LifecycleStage  `json:"new_stage,omitempty"`
	Details     string          `json:"details"`
	TriggeredBy string          `json:"triggered_by"` // "policy", "manual", "schedule"
	CreatedAt   time.Time       `json:"created_at"`
}

// EventType 事件类型
type EventType string

const (
	EventArchived      EventType = "archived"
	EventMigrated      EventType = "migrated"
	EventCleaned       EventType = "cleaned"
	EventExpired       EventType = "expired"
	EventRetentionSet  EventType = "retention_set"
	EventLegalHold     EventType = "legal_hold"
	EventDeduplicated  EventType = "deduplicated"
	EventRestored      EventType = "restored"
)

// ============================================================
// 配置
// ============================================================

// Config 智能数据生命周期管理配置
type Config struct {
	// 通用
	Enabled           bool   `json:"enabled"`
	DataRoot          string `json:"data_root"`            // 数据根目录
	ScanIntervalSec   int    `json:"scan_interval_sec"`    // 扫描间隔（秒）
	MaxConcurrentOps  int    `json:"max_concurrent_ops"`   // 最大并发操作数
	DryRun            bool   `json:"dry_run"`              // 试运行模式

	// 归档
	Archive ArchiveConfig `json:"archive"`

	// 清理
	Cleanup CleanupConfig `json:"cleanup"`

	// 迁移
	Migration MigrationConfig `json:"migration"`

	// 保留
	Retention RetentionConfig `json:"retention"`

	// 重复数据检测
	Dedup DedupConfig `json:"dedup"`
}

// ArchiveConfig 归档配置
type ArchiveConfig struct {
	Enabled         bool `json:"enabled"`
	CheckIntervalSec int  `json:"check_interval_sec"` // 检查间隔，默认 3600
	MinIdleDays     int  `json:"min_idle_days"`       // 最小空闲天数，默认 30
	BatchSize       int  `json:"batch_size"`          // 批量大小，默认 100
}

// CleanupConfig 清理配置
type CleanupConfig struct {
	Enabled          bool `json:"enabled"`
	CheckIntervalSec int  `json:"check_interval_sec"` // 检查间隔，默认 86400
	TrashRetentionDays int `json:"trash_retention_days"` // 回收站保留天数，默认 30
	BatchSize        int  `json:"batch_size"`
}

// MigrationConfig 迁移配置
type MigrationConfig struct {
	Enabled            bool    `json:"enabled"`
	CheckIntervalSec   int     `json:"check_interval_sec"`
	MaxConcurrent      int     `json:"max_concurrent"`       // 最大并发迁移数，默认 4
	BatchSize          int     `json:"batch_size"`
	MinIdleDays        int     `json:"min_idle_days"`        // 迁移前最小空闲天数
	// 分层阈值（天数）
	WarmAfterDays      int     `json:"warm_after_days"`      // 默认 7
	ColdAfterDays      int     `json:"cold_after_days"`      // 默认 30
	ArchiveAfterDays   int     `json:"archive_after_days"`   // 默认 90
}

// RetentionConfig 保留策略配置
type RetentionConfig struct {
	Enabled           bool `json:"enabled"`
	DefaultRetentionDays int `json:"default_retention_days"` // 默认保留天数，0=永久
	CheckIntervalSec  int  `json:"check_interval_sec"`      // 检查间隔
	GracePeriodDays   int  `json:"grace_period_days"`       // 宽限期
}

// DedupConfig 重复数据检测配置
type DedupConfig struct {
	Enabled          bool   `json:"enabled"`
	CheckIntervalSec int    `json:"check_interval_sec"` // 检查间隔，默认 86400
	Algorithm        string `json:"algorithm"`           // "md5", "sha256", "xxhash"
	MinFileSizeBytes int64  `json:"min_file_size_bytes"` // 最小检测文件大小
	AutoCleanup      bool   `json:"auto_cleanup"`        // 自动清理重复数据
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		Enabled:          true,
		ScanIntervalSec:  3600,
		MaxConcurrentOps: 4,
		DryRun:           false,
		Archive: ArchiveConfig{
			Enabled:          true,
			CheckIntervalSec: 3600,
			MinIdleDays:      30,
			BatchSize:        100,
		},
		Cleanup: CleanupConfig{
			Enabled:            true,
			CheckIntervalSec:   86400,
			TrashRetentionDays: 30,
			BatchSize:          100,
		},
		Migration: MigrationConfig{
			Enabled:          true,
			CheckIntervalSec: 3600,
			MaxConcurrent:    4,
			BatchSize:        100,
			MinIdleDays:      7,
			WarmAfterDays:    7,
			ColdAfterDays:    30,
			ArchiveAfterDays: 90,
		},
		Retention: RetentionConfig{
			Enabled:              true,
			DefaultRetentionDays: 0, // 默认永久保留
			CheckIntervalSec:     86400,
			GracePeriodDays:      7,
		},
		Dedup: DedupConfig{
			Enabled:          true,
			CheckIntervalSec: 86400,
			Algorithm:        "xxhash",
			MinFileSizeBytes: 4096, // 4KB
			AutoCleanup:      false,
		},
	}
}

// ============================================================
// 统计信息
// ============================================================

// LifecycleStats 生命周期统计
type LifecycleStats struct {
	// 数据分布
	StageDistribution  map[LifecycleStage]int64   `json:"stage_distribution"`  // 各阶段文件数
	StageSizes         map[LifecycleStage]int64   `json:"stage_sizes"`         // 各阶段大小（bytes）
	TotalItems         int64                      `json:"total_items"`
	TotalSize          int64                      `json:"total_size"`          // bytes

	// 归档统计
	ArchivedToday      int64   `json:"archived_today"`
	ArchivedThisWeek   int64   `json:"archived_this_week"`
	ArchivedThisMonth  int64   `json:"archived_this_month"`
	TotalArchived      int64   `json:"total_archived"`

	// 清理统计
	CleanedToday       int64   `json:"cleaned_today"`
	SpaceFreedToday    int64   `json:"space_freed_today"`   // bytes
	TotalSpaceFreed    int64   `json:"total_space_freed"`   // bytes

	// 迁移统计
	MigrationsPending  int64   `json:"migrations_pending"`
	MigrationsRunning  int64   `json:"migrations_running"`
	TotalMigrations    int64   `json:"total_migrations"`

	// 重复数据
	DuplicateGroups    int64   `json:"duplicate_groups"`
	DuplicateWaste     int64   `json:"duplicate_waste"`     // bytes

	// 保留策略
	ExpiringThisWeek   int64   `json:"expiring_this_week"`
	ExpiringThisMonth  int64   `json:"expiring_this_month"`
	LegalHolds         int64   `json:"legal_holds"`

	LastUpdated        time.Time `json:"last_updated"`
}

// ============================================================
// API 请求/响应
// ============================================================

// CreatePolicyRequest 创建策略请求
type CreatePolicyRequest struct {
	Name           string             `json:"name" binding:"required"`
	Description    string             `json:"description"`
	Classification DataClassification `json:"classification"`
	RetentionDays  int                `json:"retention_days"`
	ExpirationAction RetentionAction  `json:"expiration_action"`
	CompliancePolicy bool             `json:"compliance_policy"`
	FilePatterns   []string           `json:"file_patterns"`
	PathPrefixes   []string           `json:"path_prefixes"`
}

// CreateArchivePolicyRequest 创建归档策略请求
type CreateArchivePolicyRequest struct {
	Name             string         `json:"name" binding:"required"`
	Description      string         `json:"description"`
	Enabled          bool           `json:"enabled"`
	Trigger          ArchiveTrigger `json:"trigger" binding:"required"`
	MaxAccessCount   int            `json:"max_access_count"`
	DaysSinceAccess  int            `json:"days_since_access"`
	FileAgeDays      int            `json:"file_age_days"`
	TargetStage      LifecycleStage `json:"target_stage"`
	Schedule         string         `json:"schedule"`
	FilePatterns     []string       `json:"file_patterns"`
	PathPrefixes     []string       `json:"path_prefixes"`
	ExcludePatterns  []string       `json:"exclude_patterns"`
}

// CreateCleanupRuleRequest 创建清理规则请求
type CreateCleanupRuleRequest struct {
	Name             string           `json:"name" binding:"required"`
	Description      string           `json:"description"`
	Enabled          bool             `json:"enabled"`
	RuleType         CleanupRuleType  `json:"rule_type" binding:"required"`
	ExpireDays       int              `json:"expire_days"`
	TempFilePatterns []string         `json:"temp_file_patterns"`
	RemoveEmptyDirs  bool             `json:"remove_empty_dirs"`
	LogRetentionDays int              `json:"log_retention_days"`
	Schedule         string           `json:"schedule"`
}

// DryRunResult 试运行结果
type DryRunResult struct {
	ItemsAffected int      `json:"items_affected"`
	TotalSize     int64     `json:"total_size"` // bytes
	Actions       []string  `json:"actions"`
	Warnings      []string  `json:"warnings,omitempty"`
}
