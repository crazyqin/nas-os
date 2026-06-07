// Package hybridstorage provides hybrid storage management with tiered storage,
// SSD caching, deduplication, and quota management.
// Inspired by Synology Hybrid Share and TrueNAS Fusion Pools.
package hybridstorage

import (
	"time"
)

// ============================================================
// 存储层类型
// ============================================================

// StorageTier 存储层类型
type StorageTier string

const (
	TierHot  StorageTier = "hot"  // 热层: SSD/NVMe, 高性能低延迟
	TierWarm StorageTier = "warm" // 温层: HDD, 中等性能大容量
	TierCold StorageTier = "cold" // 冷层: 云存储/磁带, 低成本归档
)

// TierConfig 存储层配置
type TierConfig struct {
	Type           StorageTier `json:"type"`
	DevicePaths    []string    `json:"device_paths"`    // 设备路径列表
	TotalBytes     int64       `json:"total_bytes"`     // 总容量
	UsedBytes      int64       `json:"used_bytes"`      // 已用容量
	AvailableBytes int64       `json:"available_bytes"` // 可用容量
	IOPSMax        int         `json:"iops_max"`        // 最大IOPS
	ThroughputMBps int         `json:"throughput_mbps"` // 最大吞吐 MB/s
	LatencyMs      float64     `json:"latency_ms"`      // 平均延迟 ms
	HealthStatus   string      `json:"health_status"`   // healthy, degraded, failed
	Enabled        bool        `json:"enabled"`
}

// DefaultTierConfig 默认存储层配置
func DefaultTierConfig(tierType StorageTier) TierConfig {
	switch tierType {
	case TierHot:
		return TierConfig{
			Type:           TierHot,
			IOPSMax:        100000,
			ThroughputMBps: 3500,
			LatencyMs:      0.1,
			HealthStatus:   "healthy",
			Enabled:        true,
		}
	case TierWarm:
		return TierConfig{
			Type:           TierWarm,
			IOPSMax:        200,
			ThroughputMBps: 200,
			LatencyMs:      5.0,
			HealthStatus:   "healthy",
			Enabled:        true,
		}
	case TierCold:
		return TierConfig{
			Type:           TierCold,
			IOPSMax:        50,
			ThroughputMBps: 100,
			LatencyMs:      50.0,
			HealthStatus:   "healthy",
			Enabled:        true,
		}
	default:
		return TierConfig{Enabled: true, HealthStatus: "healthy"}
	}
}

// ============================================================
// 数据分层策略
// ============================================================

// TieringPolicy 数据分层策略
type TieringPolicy string

const (
	PolicyAccessFrequency TieringPolicy = "access_frequency" // 基于访问频率
	PolicyFileSize        TieringPolicy = "file_size"        // 基于文件大小
	PolicyFileType        TieringPolicy = "file_type"        // 基于文件类型
	PolicyCombined        TieringPolicy = "combined"         // 综合策略
)

// TieringRule 分层规则
type TieringRule struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Policy     TieringPolicy `json:"policy"`
	Priority   int           `json:"priority"` // 优先级, 数字越小优先级越高
	Enabled    bool          `json:"enabled"`
	TargetTier StorageTier   `json:"target_tier"` // 目标层

	// 访问频率策略参数
	AccessThreshold int `json:"access_threshold,omitempty"` // N天内访问次数阈值

	// 文件大小策略参数
	MinFileSize int64 `json:"min_file_size,omitempty"` // 最小文件大小 (bytes)
	MaxFileSize int64 `json:"max_file_size,omitempty"` // 最大文件大小 (bytes)

	// 文件类型策略参数
	FileExtensions []string `json:"file_extensions,omitempty"` // 文件扩展名列表

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultTieringRules 默认分层规则
func DefaultTieringRules() []TieringRule {
	now := time.Now()
	return []TieringRule{
		{
			ID:              "rule-hot-frequent",
			Name:            "热层-频繁访问",
			Policy:          PolicyAccessFrequency,
			Priority:        1,
			Enabled:         true,
			TargetTier:      TierHot,
			AccessThreshold: 7, // 7天内访问过
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "rule-warm-normal",
			Name:            "温层-正常访问",
			Policy:          PolicyAccessFrequency,
			Priority:        2,
			Enabled:         true,
			TargetTier:      TierWarm,
			AccessThreshold: 30, // 30天内访问过
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:         "rule-cold-archive",
			Name:       "冷层-归档",
			Policy:     PolicyAccessFrequency,
			Priority:   3,
			Enabled:    true,
			TargetTier: TierCold,
			// 超过30天未访问
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:          "rule-hot-small",
			Name:        "热层-小文件",
			Policy:      PolicyFileSize,
			Priority:    4,
			Enabled:     true,
			TargetTier:  TierHot,
			MaxFileSize: 10 * 1024 * 1024, // < 10MB
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "rule-cold-large",
			Name:        "冷层-大文件",
			Policy:      PolicyFileSize,
			Priority:    5,
			Enabled:     true,
			TargetTier:  TierCold,
			MinFileSize: 1024 * 1024 * 1024, // > 1GB
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

// ============================================================
// 分层迁移任务
// ============================================================

// MigrationTaskStatus 迁移任务状态
type MigrationTaskStatus string

const (
	MigrationPending   MigrationTaskStatus = "pending"
	MigrationRunning   MigrationTaskStatus = "running"
	MigrationPaused    MigrationTaskStatus = "paused"
	MigrationCompleted MigrationTaskStatus = "completed"
	MigrationFailed    MigrationTaskStatus = "failed"
	MigrationCancelled MigrationTaskStatus = "cancelled"
)

// MigrationTask 分层迁移任务
type MigrationTask struct {
	ID            string              `json:"id"`
	RuleID        string              `json:"rule_id"` // 关联的规则ID
	SourceTier    StorageTier         `json:"source_tier"`
	TargetTier    StorageTier         `json:"target_tier"`
	Status        MigrationTaskStatus `json:"status"`
	FileCount     int                 `json:"file_count"`     // 待迁移文件数
	MigratedCount int                 `json:"migrated_count"` // 已迁移文件数
	TotalBytes    int64               `json:"total_bytes"`    // 待迁移总大小
	MigratedBytes int64               `json:"migrated_bytes"` // 已迁移大小
	Progress      float64             `json:"progress"`       // 0-100
	Error         string              `json:"error,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
	StartedAt     *time.Time          `json:"started_at,omitempty"`
	CompletedAt   *time.Time          `json:"completed_at,omitempty"`
}

// ============================================================
// 存储池
// ============================================================

// StoragePoolStatus 存储池状态
type StoragePoolStatus string

const (
	PoolOnline    StoragePoolStatus = "online"
	PoolDegraded  StoragePoolStatus = "degraded"
	PoolOffline   StoragePoolStatus = "offline"
	PoolCreating  StoragePoolStatus = "creating"
	PoolExpanding StoragePoolStatus = "expanding"
)

// StoragePool 存储池
type StoragePool struct {
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	Status         StoragePoolStatus           `json:"status"`
	Tiers          map[StorageTier]*TierConfig `json:"tiers"`
	TotalBytes     int64                       `json:"total_bytes"`
	UsedBytes      int64                       `json:"used_bytes"`
	AvailableBytes int64                       `json:"available_bytes"`
	RAIDLevel      string                      `json:"raid_level"` // none, raid0, raid1, raid5, raid6, raidz
	FileSystem     string                      `json:"filesystem"` // zfs, ext4, xfs, btrfs
	CreatedAt      time.Time                   `json:"created_at"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

// ============================================================
// SSD缓存
// ============================================================

// SSDCacheMode SSD缓存模式
type SSDCacheMode string

const (
	CacheModeRead  SSDCacheMode = "read"  // 读缓存
	CacheModeWrite SSDCacheMode = "write" // 写缓存
	CacheModeBoth  SSDCacheMode = "both"  // 读写缓存
)

// SSDCacheConfig SSD缓存配置
type SSDCacheConfig struct {
	ID             string       `json:"id"`
	PoolID         string       `json:"pool_id"`
	DevicePaths    []string     `json:"device_paths"`
	Mode           SSDCacheMode `json:"mode"`
	CacheSizeBytes int64        `json:"cache_size_bytes"`
	BlockSizeKB    int          `json:"block_size_kb"` // 缓存块大小
	HitRate        float64      `json:"hit_rate"`      // 缓存命中率 0-1
	UsedBytes      int64        `json:"used_bytes"`
	DirtyBytes     int64        `json:"dirty_bytes"` // 写缓存脏数据量
	Status         string       `json:"status"`      // active, degraded, disabled
	Enabled        bool         `json:"enabled"`
	CreatedAt      time.Time    `json:"created_at"`
}

// DefaultSSDCacheConfig 默认SSD缓存配置
func DefaultSSDCacheConfig() SSDCacheConfig {
	return SSDCacheConfig{
		Mode:        CacheModeBoth,
		BlockSizeKB: 64,
		Status:      "active",
		Enabled:     true,
	}
}

// ============================================================
// 数据去重
// ============================================================

// DedupLevel 去重级别
type DedupLevel string

const (
	DedupBlock DedupLevel = "block" // 块级去重
	DedupFile  DedupLevel = "file"  // 文件级去重
)

// DedupStats 去重统计
type DedupStats struct {
	ID              string     `json:"id"`
	PoolID          string     `json:"pool_id"`
	Level           DedupLevel `json:"level"`
	Enabled         bool       `json:"enabled"`
	TotalFiles      int64      `json:"total_files"`
	UniqueFiles     int64      `json:"unique_files"`
	TotalBytes      int64      `json:"total_bytes"`
	DedupedBytes    int64      `json:"deduped_bytes"`    // 去重后大小
	SavedBytes      int64      `json:"saved_bytes"`      // 节省空间
	DedupRatio      float64    `json:"dedup_ratio"`      // 去重率
	ProcessingQueue int        `json:"processing_queue"` // 处理队列长度
	LastScanTime    time.Time  `json:"last_scan_time"`
}

// ============================================================
// 存储配额
// ============================================================

// QuotaType 配额类型
type QuotaType string

const (
	QuotaUser  QuotaType = "user"  // 按用户
	QuotaShare QuotaType = "share" // 按共享
	QuotaGroup QuotaType = "group" // 按组
)

// StorageQuota 存储配额
type StorageQuota struct {
	ID             string    `json:"id"`
	Type           QuotaType `json:"type"`
	TargetID       string    `json:"target_id"`   // 用户ID/共享ID/组ID
	TargetName     string    `json:"target_name"` // 用户名/共享名/组名
	PoolID         string    `json:"pool_id"`
	HardLimitBytes int64     `json:"hard_limit_bytes"` // 硬限制
	SoftLimitBytes int64     `json:"soft_limit_bytes"` // 软限制
	UsedBytes      int64     `json:"used_bytes"`
	UsagePercent   float64   `json:"usage_percent"` // 使用率 0-100
	WarnAt         float64   `json:"warn_at"`       // 告警阈值百分比, 默认80
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// DefaultQuota 默认配额配置
func DefaultQuota() StorageQuota {
	return StorageQuota{
		WarnAt:  80.0,
		Enabled: true,
	}
}

// ============================================================
// 性能监控
// ============================================================

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	PoolID    string    `json:"pool_id"`
	Timestamp time.Time `json:"timestamp"`

	// IOPS
	ReadIOPS  float64 `json:"read_iops"`
	WriteIOPS float64 `json:"write_iops"`
	TotalIOPS float64 `json:"total_iops"`

	// 吞吐量
	ReadMBps  float64 `json:"read_mbps"`
	WriteMBps float64 `json:"write_mbps"`
	TotalMBps float64 `json:"total_mbps"`

	// 延迟
	ReadLatencyMs  float64 `json:"read_latency_ms"`
	WriteLatencyMs float64 `json:"write_latency_ms"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`

	// 队列深度
	QueueDepth int `json:"queue_depth"`

	// 缓存命中率
	CacheHitRate float64 `json:"cache_hit_rate"`
}

// ============================================================
// 健康评分
// ============================================================

// HealthScore 健康评分
type HealthScore struct {
	PoolID          string    `json:"pool_id"`
	OverallScore    float64   `json:"overall_score"`   // 0-100
	StorageScore    float64   `json:"storage_score"`   // 存储容量健康度
	PerfScore       float64   `json:"perf_score"`      // 性能健康度
	TierScore       float64   `json:"tier_score"`      // 数据分层健康度
	CacheScore      float64   `json:"cache_score"`     // 缓存健康度
	DedupScore      float64   `json:"dedup_score"`     // 去重健康度
	QuotaScore      float64   `json:"quota_score"`     // 配额健康度
	Status          string    `json:"status"`          // excellent, good, fair, poor, critical
	Recommendations []string  `json:"recommendations"` // 改进建议
	UpdatedAt       time.Time `json:"updated_at"`
}
