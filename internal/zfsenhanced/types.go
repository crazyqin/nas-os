// Package zfsenhanced 提供ZFS增强管理功能
package zfsenhanced

import (
	"sync"
	"time"
)

// PoolStatus 池状态
type PoolStatus string

const (
	// PoolStatusOnline 在线
	PoolStatusOnline PoolStatus = "ONLINE"
	// PoolStatusDegraded 降级
	PoolStatusDegraded PoolStatus = "DEGRADED"
	// PoolStatusFaulted 故障
	PoolStatusFaulted PoolStatus = "FAULTED"
	// PoolStatusOffline 离线
	PoolStatusOffline PoolStatus = "OFFLINE"
	// PoolStatusUnavail 不可用
	PoolStatusUnavail PoolStatus = "UNAVAIL"
	// PoolStatusRemoved 已移除
	PoolStatusRemoved PoolStatus = "REMOVED"
)

// RaidType RAID类型
type RaidType string

const (
	// RaidTypeStripe 条带化
	RaidTypeStripe RaidType = "stripe"
	// RaidTypeMirror 镜像
	RaidTypeMirror RaidType = "mirror"
	// RaidTypeRaidz RAIDZ1
	RaidTypeRaidz RaidType = "raidz"
	// RaidTypeRaidz2 RAIDZ2
	RaidTypeRaidz2 RaidType = "raidz2"
	// RaidTypeRaidz3 RAIDZ3
	RaidTypeRaidz3 RaidType = "raidz3"
	// RaidTypeDRAID 分布式RAID
	RaidTypeDRAID RaidType = "draid"
)

// CompressionType 压缩类型
type CompressionType string

const (
	// CompressionOff 关闭
	CompressionOff CompressionType = "off"
	// CompressionLZ4 LZ4压缩
	CompressionLZ4 CompressionType = "lz4"
	// CompressionZSTD ZSTD压缩
	CompressionZSTD CompressionType = "zstd"
	// CompressionZSTD1 ZSTD-1
	CompressionZSTD1 CompressionType = "zstd-1"
	// CompressionZSTD3 ZSTD-3
	CompressionZSTD3 CompressionType = "zstd-3"
	// CompressionZSTD7 ZSTD-7
	CompressionZSTD7 CompressionType = "zstd-7"
	// CompressionZSTD19 ZSTD-19
	CompressionZSTD19 CompressionType = "zstd-19"
	// CompressionGZIP1 GZIP-1
	CompressionGZIP1 CompressionType = "gzip-1"
	// CompressionGZIP6 GZIP-6
	CompressionGZIP6 CompressionType = "gzip-6"
	// CompressionGZIP9 GZIP-9
	CompressionGZIP9 CompressionType = "gzip-9"
	// CompressionZLE ZLE
	CompressionZLE CompressionType = "zle"
	// CompressionLZJB LZJB
	CompressionLZJB CompressionType = "lzjb"
)

// DedupMode 去重模式
type DedupMode string

const (
	// DedupOff 关闭
	DedupOff DedupMode = "off"
	// DedupOn 开启
	DedupOn DedupMode = "on"
	// DedupVerify 校验去重
	DedupVerify DedupMode = "verify"
	// DedupSHA256 SHA256去重
	DedupSHA256 DedupMode = "sha256"
	// DedupSHA512 SHA512去重
	DedupSHA512 DedupMode = "sha512"
	// DedupSkein Skein去重
	DedupSkein DedupMode = "skein"
	// DedupEdonR Edon-R去重
	DedupEdonR DedupMode = "edonr"
)

// AlertSeverity 告警严重级别
type AlertSeverity string

const (
	// AlertSeverityInfo 信息
	AlertSeverityInfo AlertSeverity = "info"
	// AlertSeverityWarning 警告
	AlertSeverityWarning AlertSeverity = "warning"
	// AlertSeverityCritical 严重
	AlertSeverityCritical AlertSeverity = "critical"
	// AlertSeverityEmergency 紧急
	AlertSeverityEmergency AlertSeverity = "emergency"
)

// AlertType 告警类型
type AlertType string

const (
	// AlertTypePoolDegraded 池降级
	AlertTypePoolDegraded AlertType = "pool_degraded"
	// AlertTypeDiskFault 磁盘故障
	AlertTypeDiskFault AlertType = "disk_fault"
	// AlertTypeCapacityThreshold 容量阈值
	AlertTypeCapacityThreshold AlertType = "capacity_threshold"
	// AlertTypeScrubFailed Scrub失败
	AlertTypeScrubFailed AlertType = "scrub_failed"
	// AlertTypeChecksumError 校验和错误
	AlertTypeChecksumError AlertType = "checksum_error"
	// AlertTypeIOError IO错误
	AlertTypeIOError AlertType = "io_error"
	// AlertTypeResilverFailed 重建失败
	AlertTypeResilverFailed AlertType = "resilver_failed"
	// AlertTypeHighLatency 高延迟
	AlertTypeHighLatency AlertType = "high_latency"
)

// SnapshotPolicy 快照策略
type SnapshotPolicy struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	PoolName      string        `json:"pool_name"`
	Dataset       string        `json:"dataset"`
	Schedule      string        `json:"schedule"`      // cron表达式
	RetentionDays int           `json:"retention_days"` // 保留天数
	MaxSnapshots  int           `json:"max_snapshots"`  // 最大快照数
	Prefix        string        `json:"prefix"`         // 快照前缀
	Recursive     bool          `json:"recursive"`      // 是否递归
	Enabled       bool          `json:"enabled"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	LastSnapshot  time.Time     `json:"last_snapshot,omitempty"`
	AutoDestroy   bool          `json:"auto_destroy"` // 自动销毁过期快照
}

// SnapshotInfo 快照信息
type SnapshotInfo struct {
	Name         string    `json:"name"`
	PoolName     string    `json:"pool_name"`
	Dataset      string    `json:"dataset"`
	SnapshotName string    `json:"snapshot_name"`
	CreatedAt    time.Time `json:"created_at"`
	UsedBytes    int64     `json:"used_bytes"`
	ReferBytes   int64     `json:"refer_bytes"`
	Clones       []string  `json:"clones,omitempty"`
	UserProps    map[string]string `json:"user_props,omitempty"`
}

// PoolConfig 池创建配置
type PoolConfig struct {
	Name         string     `json:"name"`
	RaidType     RaidType   `json:"raid_type"`
	Disks        []string   `json:"disks"`
	Spares       []string   `json:"spares,omitempty"`
	BlockSize    int        `json:"block_size,omitempty"`    // 字节
	Ashift       int        `json:"ashift,omitempty"`        // 2^ashift
	Compression  CompressionType `json:"compression,omitempty"`
	Dedup        DedupMode  `json:"dedup,omitempty"`
	Sync         string     `json:"sync,omitempty"`          // standard, always, disabled
	Atime        bool       `json:"atime,omitempty"`
	Xattr        string     `json:"xattr,omitempty"`         // on, off, sa
	AutoExpand   bool       `json:"auto_expand,omitempty"`
	Comment      string     `json:"comment,omitempty"`
}

// PoolInfo 池信息
type PoolInfo struct {
	Name          string        `json:"name"`
	Status        PoolStatus    `json:"status"`
	RaidType      RaidType      `json:"raid_type"`
	SizeBytes     int64         `json:"size_bytes"`
	UsedBytes     int64         `json:"used_bytes"`
	FreeBytes     int64         `json:"free_bytes"`
	UsedPercent   float64       `json:"used_percent"`
	Fragmentation float64       `json:"fragmentation"`
	Health        string        `json:"health"`
	ReadErrors    int64         `json:"read_errors"`
	WriteErrors   int64         `json:"write_errors"`
	ChecksumErrors int64        `json:"checksum_errors"`
	Disks         []DiskInfo    `json:"disks"`
	Spares        []DiskInfo    `json:"spares,omitempty"`
	ScanStatus    string        `json:"scan_status"`
	ScanProgress  float64       `json:"scan_progress"`
	Timestamp     time.Time     `json:"timestamp"`
	Properties    map[string]string `json:"properties,omitempty"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Name          string     `json:"name"`
	Path          string     `json:"path"`
	Status        PoolStatus `json:"status"`
	SizeBytes     int64      `json:"size_bytes"`
	ReadErrors    int64      `json:"read_errors"`
	WriteErrors   int64      `json:"write_errors"`
	ChecksumErrors int64     `json:"checksum_errors"`
	State         string     `json:"state"`
	IsSpare       bool       `json:"is_spare"`
	IsLog         bool       `json:"is_log"`
	IsCache       bool       `json:"is_cache"`
	SMART         *SMARTInfo `json:"smart,omitempty"`
}

// SMARTInfo SMART信息
type SMARTInfo struct {
	Model       string  `json:"model"`
	Serial      string  `json:"serial"`
	Firmware    string  `json:"firmware"`
	Temperature int     `json:"temperature"`
	PowerOnHours int64  `json:"power_on_hours"`
	ReallocatedSectors int64 `json:"reallocated_sectors"`
	PendingSectors     int64 `json:"pending_sectors"`
	HealthStatus       string `json:"health_status"`
}

// Alert 告警信息
type Alert struct {
	ID        string       `json:"id"`
	Type      AlertType    `json:"type"`
	Severity  AlertSeverity `json:"severity"`
	PoolName  string       `json:"pool_name"`
	DiskName  string       `json:"disk_name,omitempty"`
	Message   string       `json:"message"`
	Details   string       `json:"details,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	Acked     bool         `json:"acked"`
	AckedAt   time.Time    `json:"acked_at,omitempty"`
	AckedBy   string       `json:"acked_by,omitempty"`
	Resolved  bool         `json:"resolved"`
	ResolvedAt time.Time   `json:"resolved_at,omitempty"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	CapacityWarningPercent  float64 `json:"capacity_warning_percent"`  // 80%
	CapacityCriticalPercent float64 `json:"capacity_critical_percent"` // 90%
	CapacityEmergencyPercent float64 `json:"capacity_emergency_percent"` // 95%
	ChecksumErrorThreshold  int64   `json:"checksum_error_threshold"`
	IOErrorThreshold        int64   `json:"io_error_threshold"`
	LatencyWarningMs        float64 `json:"latency_warning_ms"`
	LatencyCriticalMs       float64 `json:"latency_critical_ms"`
	EnableEmailAlert        bool    `json:"enable_email_alert"`
	EnableWebhookAlert      bool    `json:"enable_webhook_alert"`
	WebhookURL              string  `json:"webhook_url,omitempty"`
}

// DefaultAlertConfig 默认告警配置
func DefaultAlertConfig() AlertConfig {
	return AlertConfig{
		CapacityWarningPercent:  80,
		CapacityCriticalPercent: 90,
		CapacityEmergencyPercent: 95,
		ChecksumErrorThreshold:  10,
		IOErrorThreshold:        5,
		LatencyWarningMs:        100,
		LatencyCriticalMs:       500,
		EnableEmailAlert:        false,
		EnableWebhookAlert:      false,
	}
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	PoolName         string    `json:"pool_name"`
	Timestamp        time.Time `json:"timestamp"`
	ReadIOPS         int64     `json:"read_iops"`
	WriteIOPS        int64     `json:"write_iops"`
	ReadThroughput   int64     `json:"read_throughput"`   // bytes/s
	WriteThroughput  int64     `json:"write_throughput"`  // bytes/s
	ReadLatencyMs    float64   `json:"read_latency_ms"`
	WriteLatencyMs   float64   `json:"write_latency_ms"`
	ARCMisses        int64     `json:"arc_misses"`
	ARCHits          int64     `json:"arc_hits"`
	ARCSizeBytes     int64     `json:"arc_size_bytes"`
	ARCTargetBytes   int64     `json:"arc_target_bytes"`
	L2ARCCacheHits   int64     `json:"l2arc_cache_hits"`
	L2ARCCacheMisses int64     `json:"l2arc_cache_misses"`
	L2ARCSizeBytes   int64     `json:"l2arc_size_bytes"`
	ZILWriteBytes    int64     `json:"zil_write_bytes"`
	ZILSyncCount     int64     `json:"zil_sync_count"`
	CPUUsage         float64   `json:"cpu_usage"`
	MemUsageBytes    int64     `json:"mem_usage_bytes"`
}

// ARCConfig ARC配置
type ARCConfig struct {
	MaxSizeBytes    int64   `json:"max_size_bytes"`
	MinSizeBytes    int64   `json:"min_size_bytes"`
	PrefetchEnabled bool    `json:"prefetch_enabled"`
	HitRatePercent  float64 `json:"hit_rate_percent"`
	SizeBytes       int64   `json:"size_bytes"`
	TargetBytes     int64   `json:"target_bytes"`
	Hits            int64   `json:"hits"`
	Misses          int64   `json:"misses"`
}

// L2ARCConfig L2ARC配置
type L2ARCConfig struct {
	Enabled       bool     `json:"enabled"`
	Devices       []string `json:"devices"`
	SizeBytes     int64    `json:"size_bytes"`
	WriteSizeBytes int64   `json:"write_size_bytes"`
	Hits          int64    `json:"hits"`
	Misses        int64    `json:"misses"`
	HitRatePercent float64 `json:"hit_rate_percent"`
}

// ZILConfig ZIL配置
type ZILConfig struct {
	Enabled       bool     `json:"enabled"`
	Devices       []string `json:"devices"`
	SyncDisabled  bool     `json:"sync_disabled"`
	WriteBytes    int64    `json:"write_bytes"`
	SyncCount     int64    `json:"sync_count"`
	AvgSyncLatencyMs float64 `json:"avg_sync_latency_ms"`
}

// CapacityTrend 容量趋势
type CapacityTrend struct {
	Timestamp     time.Time `json:"timestamp"`
	TotalBytes    int64     `json:"total_bytes"`
	UsedBytes     int64     `json:"used_bytes"`
	FreeBytes     int64     `json:"free_bytes"`
	UsedPercent   float64   `json:"used_percent"`
	GrowthRateDay float64   `json:"growth_rate_day"` // 每日增长字节
	DaysUntilFull int       `json:"days_until_full"` // 预计几天后满
}

// CompressionStats 压缩统计
type CompressionStats struct {
	PoolName         string  `json:"pool_name"`
	Dataset          string  `json:"dataset"`
	CompressRatio    float64 `json:"compress_ratio"`
	CompressedBytes  int64   `json:"compressed_bytes"`
	UncompressedBytes int64  `json:"uncompressed_bytes"`
	SavedBytes       int64   `json:"saved_bytes"`
	CompressionType  CompressionType `json:"compression_type"`
	ReductionPercent float64 `json:"reduction_percent"`
}

// DedupStats 去重统计
type DedupStats struct {
	PoolName         string   `json:"pool_name"`
	DedupMode        DedupMode `json:"dedup_mode"`
	DedupRatio       float64  `json:"dedup_ratio"`
	DedupTableSize   int64    `json:"dedup_table_size"`
	DedupTableEntries int64   `json:"dedup_table_entries"`
	SavedBytes       int64    `json:"saved_bytes"`
	DuplicatesFound  int64    `json:"duplicates_found"`
	MemoryUsageBytes int64    `json:"memory_usage_bytes"`
}

// MigrationTask 迁移任务
type MigrationTask struct {
	ID           string    `json:"id"`
	SourcePool    string    `json:"source_pool"`
	TargetPool    string    `json:"target_pool"`
	Datasets      []string  `json:"datasets"`
	Status        string    `json:"status"`        // pending, running, completed, failed
	Progress      float64   `json:"progress"`
	BytesTotal    int64     `json:"bytes_total"`
	BytesCopied   int64     `json:"bytes_copied"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time,omitempty"`
	EstimatedTime string    `json:"estimated_time,omitempty"`
	ErrorMsg      string    `json:"error_msg,omitempty"`
}

// ========== 新增类型：ZFSDataset, RAIDZExpansion, IntegrityReport ==========

// ZFSDataset ZFS数据集信息
type ZFSDataset struct {
	Name           string            `json:"name"`
	PoolName       string            `json:"pool_name"`
	Type           string            `json:"type"`            // filesystem, volume
	UsedBytes      int64             `json:"used_bytes"`
	AvailBytes     int64             `json:"avail_bytes"`
	ReferBytes     int64             `json:"refer_bytes"`
	UsedPercent    float64           `json:"used_percent"`
	MountPoint     string            `json:"mount_point"`
	Compression    CompressionType   `json:"compression"`
	Dedup          DedupMode         `json:"dedup"`
	RecordSize     int               `json:"record_size"`
	QuotaBytes     int64             `json:"quota_bytes"`
	ReserveBytes   int64             `json:"reserve_bytes"`
	SnapshotCount  int               `json:"snapshot_count"`
	Clones         []string          `json:"clones,omitempty"`
	Properties     map[string]string `json:"properties,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// RAIDZExpansion RAID-Z扩展任务
type RAIDZExpansion struct {
	ID             string    `json:"id"`
	PoolName       string    `json:"pool_name"`
	Status         string    `json:"status"`          // pending, running, completed, failed
	OldRaidType    RaidType  `json:"old_raid_type"`
	NewRaidType    RaidType  `json:"new_raid_type"`
	NewDisks       []string  `json:"new_disks"`
	Progress       float64   `json:"progress"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time,omitempty"`
	EstimatedTime  string    `json:"estimated_time,omitempty"`
	BytesResilvered int64    `json:"bytes_resilvered"`
	ErrorMsg       string    `json:"error_msg,omitempty"`
}

// IntegrityReport 数据完整性报告
type IntegrityReport struct {
	ID                 string                   `json:"id"`
	PoolName           string                   `json:"pool_name"`
	GeneratedAt        time.Time                `json:"generated_at"`
	OverallStatus      string                   `json:"overall_status"` // healthy, degraded, critical
	HealthScore        float64                  `json:"health_score"`
	LastScrubTime      time.Time                `json:"last_scrub_time,omitempty"`
	ScrubErrors        int64                    `json:"scrub_errors"`
	ScrubRepaired      int64                    `json:"scrub_repaired"`
	ChecksumErrors     int64                    `json:"checksum_errors"`
	ReadErrors         int64                    `json:"read_errors"`
	WriteErrors        int64                    `json:"write_errors"`
	TotalDisks         int                      `json:"total_disks"`
	HealthyDisks       int                      `json:"healthy_disks"`
	DegradedDisks      int                      `json:"degraded_disks"`
	FailedDisks        int                      `json:"failed_disks"`
	DiskDetails        []DiskIntegrityDetail    `json:"disk_details"`
	CheckResults       []IntegrityCheckResult   `json:"check_results,omitempty"`
	Recommendations    []string                 `json:"recommendations,omitempty"`
}

// DiskIntegrityDetail 磁盘完整性详情
type DiskIntegrityDetail struct {
	Name            string  `json:"name"`
	Path            string  `json:"path"`
	Status          string  `json:"status"`
	ReadErrors      int64   `json:"read_errors"`
	WriteErrors     int64   `json:"write_errors"`
	ChecksumErrors  int64   `json:"checksum_errors"`
	SMARTHealth     string  `json:"smart_health"`
	Temperature     int     `json:"temperature"`
	PowerOnHours    int64   `json:"power_on_hours"`
	ReallocatedSectors int64 `json:"reallocated_sectors"`
}

// SnapshotCloneRequest 快照克隆请求
type SnapshotCloneRequest struct {
	Dataset       string `json:"dataset" binding:"required"`
	SnapshotName  string `json:"snapshot_name" binding:"required"`
	TargetDataset string `json:"target_dataset" binding:"required"`
}

// ExpandRAIDZRequest RAID-Z扩展请求
type ExpandRAIDZRequest struct {
	PoolName   string   `json:"pool_name" binding:"required"`
	NewDisks   []string `json:"new_disks" binding:"required,min=1"`
	NewRaidType string  `json:"new_raid_type,omitempty"`
}

// IOBottleneck IO瓶颈分析
type IOBottleneck struct {
	Device        string  `json:"device"`
	Type          string  `json:"type"`        // read, write, mixed
	Utilization   float64 `json:"utilization"` // 0-100%
	AvgQueueDepth float64 `json:"avg_queue_depth"`
	AwaitMs       float64 `json:"await_ms"`     // 平均等待时间
	SVCTmMs       float64 `json:"svctm_ms"`     // 平均服务时间
	Recommendation string `json:"recommendation"`
}

// PerformanceTuningRecommendation 性能调优建议
type PerformanceTuningRecommendation struct {
	Category    string `json:"category"`    // arc, l2arc, zil, compression, dedup, blocksize
	Current     string `json:"current"`
	Recommended string `json:"recommended"`
	Impact      string `json:"impact"`      // high, medium, low
	Description string `json:"description"`
	Priority    int    `json:"priority"`    // 1-5, 1最高
}

// PoolManager 池管理器
type PoolManager struct {
	mu             sync.RWMutex
	pools          map[string]*PoolInfo
	alertConfig    AlertConfig
	alerts         []Alert
	snapshotPolicies map[string]*SnapshotPolicy
	capacityHistory  map[string][]CapacityTrend
	metricsHistory   map[string][]PerformanceMetrics
}

// NewPoolManager 创建池管理器
func NewPoolManager(alertConfig AlertConfig) *PoolManager {
	return &PoolManager{
		pools:            make(map[string]*PoolInfo),
		alertConfig:      alertConfig,
		alerts:           make([]Alert, 0),
		snapshotPolicies: make(map[string]*SnapshotPolicy),
		capacityHistory:  make(map[string][]CapacityTrend),
		metricsHistory:   make(map[string][]PerformanceMetrics),
	}
}
