// Package storagetiering 提供智能存储分层引擎，支持数据热度分析、自动分层策略、分层迁移调度。
package storagetiering

import (
	"fmt"
	"time"
)

// ============================================================================
// 存储层级
// ============================================================================

// Tier 存储层级（用于迁移器和策略引擎）
type Tier int

const (
	TierSSD  Tier = 1 // SSD 热存储
	TierHDD  Tier = 2 // HDD 温存储
	TierCold Tier = 3 // 冷存储/云端
)

// String 返回层级名称
func (t Tier) String() string {
	switch t {
	case TierSSD:
		return "ssd"
	case TierHDD:
		return "hdd"
	case TierCold:
		return "cold"
	default:
		return "unknown"
	}
}

// ============================================================================
// 迁移状态
// ============================================================================

// MigrationState 迁移状态
type MigrationState string

const (
	StatePending   MigrationState = "pending"
	StateRunning   MigrationState = "running"
	StateCompleted MigrationState = "completed"
	StateFailed    MigrationState = "failed"
	StateCancelled MigrationState = "cancelled"
)

// ============================================================================
// 配置结构
// ============================================================================

// Config 存储分层引擎配置
type Config struct {
	Enabled   bool             `json:"enabled"`
	Tiers     []TierCapacity   `json:"tiers"`
	Analyzer  AnalyzerConfig   `json:"analyzer"`
	Policy    PolicyConfig     `json:"policy"`
	Migrator  MigratorConfig   `json:"migrator"`
}

// Validate 验证配置
func (c Config) Validate() error {
	if len(c.Tiers) == 0 {
		return fmt.Errorf("at least one tier must be configured")
	}
	return nil
}

// AnalyzerConfig 分析器配置
type AnalyzerConfig struct {
	AnalysisInterval time.Duration `json:"analysis_interval"`
	SampleSize       int           `json:"sample_size"`
}

// PolicyConfig 策略配置
type PolicyConfig struct {
	Thresholds       ThresholdConfig   `json:"thresholds"`
	CapacityHighPct  float64           `json:"capacity_high_pct"`
	CapacityLowPct   float64           `json:"capacity_low_pct"`
	FileTypeBoosts   map[string]float64 `json:"file_type_boosts"`
	LargeFilePenalty float64           `json:"large_file_penalty"`
}

// ThresholdConfig 热度阈值配置
type ThresholdConfig struct {
	HotMinScore  float64 `json:"hot_min_score"`
	WarmMinScore float64 `json:"warm_min_score"`
}

// MigratorConfig 迁移器配置
type MigratorConfig struct {
	MaxConcurrent   int  `json:"max_concurrent"`
	VerifyChecksum  bool `json:"verify_checksum"`
	BandwidthMBps   int  `json:"bandwidth_mbps"`
}

// TierCapacity 层级容量
type TierCapacity struct {
	Tier       Tier   `json:"tier"`
	TotalBytes int64  `json:"total_bytes"`
	MountPoint string `json:"mount_point"`
}

// ============================================================================
// 文件相关结构
// ============================================================================

// FileEntry 文件条目（用于分析器）
type FileEntry struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	CurrentTier Tier      `json:"current_tier"`
	HeatScore   float64   `json:"heat_score"`
	AccessCount int64     `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
	CreatedAt   time.Time `json:"created_at"`
	IsPinned    bool      `json:"is_pinned"`
}

// AccessRecord 访问记录
type AccessRecord struct {
	FilePath  string    `json:"file_path"`
	Timestamp time.Time `json:"timestamp"`
	ReadBytes int64     `json:"read_bytes"`
}

// ============================================================================
// 迁移相关结构
// ============================================================================

// MigrationTask 迁移任务（用于迁移器）
type MigrationTask struct {
	ID           string         `json:"id"`
	FilePath     string         `json:"file_path"`
	FileSize     int64          `json:"file_size"`
	FromTier     Tier           `json:"from_tier"`
	ToTier       Tier           `json:"to_tier"`
	State        MigrationState `json:"state"`
	Progress     int            `json:"progress"` // 0-100
	Reason       string         `json:"reason"`
	ChecksumSrc  string         `json:"checksum_src,omitempty"`
	ChecksumDst  string         `json:"checksum_dst,omitempty"`
	Error        string         `json:"error,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	StartedAt    *time.Time     `json:"started_at,omitempty"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
}

// MigrationHistoryItem 迁移历史记录
type MigrationHistoryItem struct {
	TaskID    string         `json:"task_id"`
	FilePath  string         `json:"file_path"`
	FromTier  Tier           `json:"from_tier"`
	ToTier    Tier           `json:"to_tier"`
	FileSize  int64          `json:"file_size"`
	State     MigrationState `json:"state"`
	Reason    string         `json:"reason"`
	Timestamp time.Time      `json:"timestamp"`
}

// ============================================================================
// 统计相关结构
// ============================================================================

// Stats 引擎统计信息
type Stats struct {
	Tiers            []TierStats             `json:"tiers"`
	TotalMigrations  int64                   `json:"total_migrations"`
	ActiveMigrations int                     `json:"active_migrations"`
	HitRate          float64                 `json:"hit_rate"`
	RecentHistory    []MigrationHistoryItem  `json:"recent_history"`
	LastAnalysis     time.Time               `json:"last_analysis"`
}

// TierStats 层级统计
type TierStats struct {
	Tier       Tier    `json:"tier"`
	TotalBytes int64   `json:"total_bytes"`
	UsedBytes  int64   `json:"used_bytes"`
	FreeBytes  int64   `json:"free_bytes"`
	UsageRatio float64 `json:"usage_ratio"`
}

// ============================================================================
// 兼容性类型（保留旧代码支持）
// ============================================================================

// TierLevel 存储层级（旧版本兼容）
type TierLevel string

const (
	TierLevelHot  TierLevel = "hot"
	TierLevelWarm TierLevel = "warm"
	TierLevelCold TierLevel = "cold"
)

// StoragePoolType 存储池类型
type StoragePoolType string

const (
	StoragePoolSSD   StoragePoolType = "ssd"
	StoragePoolHDD   StoragePoolType = "hdd"
	StoragePoolCloud StoragePoolType = "cloud"
)

// AccessPattern 访问模式
type AccessPattern string

const (
	AccessPatternSequential AccessPattern = "sequential"
	AccessPatternRandom     AccessPattern = "random"
	AccessPatternWriteOnce  AccessPattern = "write_once"
	AccessPatternReadWrite  AccessPattern = "read_write"
)

// MigrationStatus 迁移状态（旧版本兼容）
type MigrationStatus string

const (
	MigrationStatusPending   MigrationStatus = "pending"
	MigrationStatusRunning   MigrationStatus = "running"
	MigrationStatusCompleted MigrationStatus = "completed"
	MigrationStatusFailed    MigrationStatus = "failed"
	MigrationStatusCancelled MigrationStatus = "cancelled"
)

// HeatLevel 热度等级
type HeatLevel string

const (
	HeatLevelHot    HeatLevel = "hot"
	HeatLevelWarm   HeatLevel = "warm"
	HeatLevelCold   HeatLevel = "cold"
	HeatLevelFrozen HeatLevel = "frozen"
)

// FileMetadata 文件元数据
type FileMetadata struct {
	ID             string        `json:"id"`
	Path           string        `json:"path"`
	SizeBytes      int64         `json:"size_bytes"`
	CurrentTier    TierLevel     `json:"current_tier"`
	AccessPattern  AccessPattern `json:"access_pattern"`
	AccessCount    int64         `json:"access_count"`
	LastAccessAt   time.Time     `json:"last_access_at"`
	LastModifiedAt time.Time     `json:"last_modified_at"`
	CreatedAt      time.Time     `json:"created_at"`
	HeatScore      float64       `json:"heat_score"`
	HeatLevel      HeatLevel     `json:"heat_level"`
	IsPinned       bool          `json:"is_pinned"`
	MigratedAt     *time.Time    `json:"migrated_at,omitempty"`
	Tags           []string      `json:"tags,omitempty"`
}

// StoragePool 存储池
type StoragePool struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Type           StoragePoolType `json:"type"`
	Tier           TierLevel       `json:"tier"`
	CapacityBytes  int64           `json:"capacity_bytes"`
	UsedBytes      int64           `json:"used_bytes"`
	AvailableBytes int64           `json:"available_bytes"`
	IsActive       bool            `json:"is_active"`
	ReadSpeedMBs   float64         `json:"read_speed_mbs"`
	WriteSpeedMBs  float64         `json:"write_speed_mbs"`
	IOPS           int             `json:"iops"`
	LatencyMs      float64         `json:"latency_ms"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// TieringRule 分层规则
type TieringRule struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	IsActive       bool            `json:"is_active"`
	Priority       int             `json:"priority"`
	MinAgeDays     int             `json:"min_age_days,omitempty"`
	MaxAgeDays     int             `json:"max_age_days,omitempty"`
	MinSizeBytes   int64           `json:"min_size_bytes,omitempty"`
	MaxSizeBytes   int64           `json:"max_size_bytes,omitempty"`
	MinAccessCount int64           `json:"min_access_count,omitempty"`
	MaxAccessCount int64           `json:"max_access_count,omitempty"`
	MinHeatScore   float64         `json:"min_heat_score,omitempty"`
	MaxHeatScore   float64         `json:"max_heat_score,omitempty"`
	AccessPatterns []AccessPattern `json:"access_patterns,omitempty"`
	TargetTier     TierLevel       `json:"target_tier"`
	IncludeTags    []string        `json:"include_tags,omitempty"`
	ExcludeTags    []string        `json:"exclude_tags,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// TieringStats 分层统计
type TieringStats struct {
	ID           string    `json:"id"`
	Tier         TierLevel `json:"tier"`
	FileCount    int64     `json:"file_count"`
	TotalBytes   int64     `json:"total_bytes"`
	AvgHeatScore float64   `json:"avg_heat_score"`
	UsedPercent  float64   `json:"used_percent"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// MigrationStats 迁移统计
type MigrationStats struct {
	ID              string     `json:"id"`
	TotalMigrations int64      `json:"total_migrations"`
	SuccessfulCount int64      `json:"successful_count"`
	FailedCount     int64      `json:"failed_count"`
	CancelledCount  int64      `json:"cancelled_count"`
	TotalBytesMoved int64      `json:"total_bytes_moved"`
	AvgDurationMs   float64    `json:"avg_duration_ms"`
	LastMigrationAt *time.Time `json:"last_migration_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// AnalysisReport 分析报告
type AnalysisReport struct {
	ID               string              `json:"id"`
	GeneratedAt      time.Time           `json:"generated_at"`
	TierStats        []*TieringStats     `json:"tier_stats"`
	MigrationStats   *MigrationStats     `json:"migration_stats"`
	TotalFiles       int64               `json:"total_files"`
	TotalBytes       int64               `json:"total_bytes"`
	HeatDistribution map[HeatLevel]int64 `json:"heat_distribution"`
	Recommendations  []string            `json:"recommendations,omitempty"`
}

// StorageTieringConfig 存储分层配置
type StorageTieringConfig struct {
	Enabled                 bool    `json:"enabled"`
	AnalysisIntervalMs      int64   `json:"analysis_interval_ms"`
	MigrationBatchSize      int     `json:"migration_batch_size"`
	MaxConcurrentMigrations int     `json:"max_concurrent_migrations"`
	HeatDecayDays           int     `json:"heat_decay_days"`
	HeatThresholdHot        float64 `json:"heat_threshold_hot"`
	HeatThresholdWarm       float64 `json:"heat_threshold_warm"`
	HeatThresholdCold       float64 `json:"heat_threshold_cold"`
	AutoTieringEnabled      bool    `json:"auto_tiering_enabled"`
	MigrationBandwidthMBps  int     `json:"migration_bandwidth_mbps"`
	MinFileSizeBytes        int64   `json:"min_file_size_bytes"`
	MaxFileSizeBytes        int64   `json:"max_file_size_bytes"`
}

// DefaultStorageTieringConfig 默认配置
func DefaultStorageTieringConfig() *StorageTieringConfig {
	return &StorageTieringConfig{
		Enabled:                 true,
		AnalysisIntervalMs:      60000,
		MigrationBatchSize:      100,
		MaxConcurrentMigrations: 5,
		HeatDecayDays:           30,
		HeatThresholdHot:        70.0,
		HeatThresholdWarm:       40.0,
		HeatThresholdCold:       10.0,
		AutoTieringEnabled:      true,
		MigrationBandwidthMBps:  100,
		MinFileSizeBytes:        1024,
		MaxFileSizeBytes:        1024 * 1024 * 1024 * 10,
	}
}
