// Package hybridflash 提供 SSD/HDD 智能混合分层存储管理.
//
// 对标 TrueNAS 26 的 OpenZFS 2.4 Hybrid Flash Pools，实现:
// - 数据热度追踪：监控文件/块的访问频率和模式
// - 智能分层策略：热数据→SSD，冷数据→HDD，支持自定义规则
// - 混合池管理：SSD 作为 L2ARC/SLOG 加速层
// - 迁移调度：低峰时段自动迁移，避免影响业务
// - 效率报告：分层命中率、性能提升、空间利用率
package hybridflash

import (
	"time"
)

// FlashType 闪存类型.
type FlashType string

const (
	// FlashTypeSSD SSD 固态硬盘.
	FlashTypeSSD FlashType = "ssd"
	// FlashTypeNVMe NVMe 高速固态硬盘.
	FlashTypeNVMe FlashType = "nvme"
	// FlashTypeHDD HDD 机械硬盘.
	FlashTypeHDD FlashType = "hdd"
)

// CacheRole 缓存角色.
type CacheRole string

const (
	// CacheRoleL2ARC L2ARC 读缓存层.
	CacheRoleL2ARC CacheRole = "l2arc"
	// CacheRoleSLOG SLOG 同步写入日志层.
	CacheRoleSLOG CacheRole = "slog"
	// CacheRoleMetadata 元数据专用层.
	CacheRoleMetadata CacheRole = "metadata"
	// CacheRolePrimary 主存储层.
	CacheRolePrimary CacheRole = "primary"
)

// DataHeatLevel 数据热度级别.
type DataHeatLevel string

const (
	// HeatLevelHot 热数据：高频访问.
	HeatLevelHot DataHeatLevel = "hot"
	// HeatLevelWarm 温数据：中频访问.
	HeatLevelWarm DataHeatLevel = "warm"
	// HeatLevelCold 冷数据：低频访问.
	HeatLevelCold DataHeatLevel = "cold"
	// HeatLevelFrozen 冻结数据：极低频访问.
	HeatLevelFrozen DataHeatLevel = "frozen"
)

// AccessPattern 访问模式.
type AccessPattern string

const (
	// AccessPatternSequential 顺序访问.
	AccessPatternSequential AccessPattern = "sequential"
	// AccessPatternRandom 随机访问.
	AccessPatternRandom AccessPattern = "random"
	// AccessPatternMixed 混合访问.
	AccessPatternMixed AccessPattern = "mixed"
)

// MigrateStatus 迁移任务状态.
type MigrateStatus string

const (
	// MigrateStatusPending 待执行.
	MigrateStatusPending MigrateStatus = "pending"
	// MigrateStatusRunning 执行中.
	MigrateStatusRunning MigrateStatus = "running"
	// MigrateStatusCompleted 已完成.
	MigrateStatusCompleted MigrateStatus = "completed"
	// MigrateStatusFailed 失败.
	MigrateStatusFailed MigrateStatus = "failed"
	// MigrateStatusCancelled 已取消.
	MigrateStatusCancelled MigrateStatus = "cancelled"
)

// ScheduleType 调度类型.
type ScheduleType string

const (
	// ScheduleTypeManual 手动触发.
	ScheduleTypeManual ScheduleType = "manual"
	// ScheduleTypeAutomatic 自动触发.
	ScheduleTypeAutomatic ScheduleType = "automatic"
	// ScheduleTypeScheduled 定时触发.
	ScheduleTypeScheduled ScheduleType = "scheduled"
)

// PoolState 混合池状态.
type PoolState string

const (
	// PoolStateOnline 在线正常.
	PoolStateOnline PoolState = "online"
	// PoolStateDegraded 降级运行.
	PoolStateDegraded PoolState = "degraded"
	// PoolStateFaulted 故障状态.
	PoolStateFaulted PoolState = "faulted"
	// PoolStateOffline 离线.
	PoolStateOffline PoolState = "offline"
)

// FlashDevice 闪存设备配置.
type FlashDevice struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Type       FlashType `json:"type"`
	CacheRole  CacheRole `json:"cacheRole"`
	Capacity   int64     `json:"capacity"`   // 容量（字节）
	Used       int64     `json:"used"`       // 已使用（字节）
	Available  int64     `json:"available"`  // 可用空间（字节）
	ReadSpeed  int64     `json:"readSpeed"`  // 读取速度（MB/s）
	WriteSpeed int64     `json:"writeSpeed"` // 写入速度（MB/s）
	IOPS       int64     `json:"iops"`       // IOPS
	Enabled    bool      `json:"enabled"`
	Health     float64   `json:"health"`     // 健康度 (0-100)
	Temperature int      `json:"temperature"` // 温度（摄氏度）
	WearLevel  float64   `json:"wearLevel"`  // 磨损程度 (0-100)
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// HybridPool 混合存储池.
type HybridPool struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	State         PoolState            `json:"state"`
	FlashDevices  []*FlashDevice       `json:"flashDevices"`
	HDDDevices    []*HDDDevice         `json:"hddDevices"`
	CachePolicies []*CachePolicy       `json:"cachePolicies"`
	TotalCapacity int64                `json:"totalCapacity"`
	TotalUsed     int64                `json:"totalUsed"`
	SSDUsage      float64              `json:"ssdUsage"`   // SSD 使用率
	HDDUsage      float64              `json:"hddUsage"`   // HDD 使用率
	HitRate       float64              `json:"hitRate"`    // 缓存命中率
	IOStats       *IOStatistics        `json:"ioStats"`
	CreatedAt     time.Time            `json:"createdAt"`
	UpdatedAt     time.Time            `json:"updatedAt"`
}

// HDDDevice HDD 设备配置.
type HDDDevice struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Path       string  `json:"path"`
	Capacity   int64   `json:"capacity"`
	Used       int64   `json:"used"`
	Available  int64   `json:"available"`
	ReadSpeed  int64   `json:"readSpeed"`  // MB/s
	WriteSpeed int64   `json:"writeSpeed"` // MB/s
	Enabled    bool    `json:"enabled"`
	Health     float64 `json:"health"`     // 0-100
	RPM        int     `json:"rpm"`        // 转速
}

// CachePolicy 缓存策略.
type CachePolicy struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Description     string           `json:"description,omitempty"`
	Enabled         bool             `json:"enabled"`
	CacheRole       CacheRole        `json:"cacheRole"`
	HeatLevel       DataHeatLevel    `json:"heatLevel"`
	AccessPattern   AccessPattern    `json:"accessPattern"`
	MinBlockSize    int64            `json:"minBlockSize"`    // 最小块大小
	MaxBlockSize    int64            `json:"maxBlockSize"`    // 最大块大小
	MinAccessCount  int64            `json:"minAccessCount"`  // 最小访问次数
	MaxAccessAge    time.Duration    `json:"maxAccessAge"`    // 最大访问间隔
	FilePatterns    []string         `json:"filePatterns"`    // 文件匹配模式
	ExcludePatterns []string         `json:"excludePatterns"` // 排除模式
	Priority        int              `json:"priority"`        // 优先级
	PreferSSD       bool             `json:"preferSsd"`       // 优先使用 SSD
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

// BlockAccessRecord 块访问记录.
type BlockAccessRecord struct {
	BlockID      string          `json:"blockId"`
	PoolID       string          `json:"poolId"`
	FilePath     string          `json:"filePath"`
	Offset       int64           `json:"offset"`
	Size         int64           `json:"size"`
	AccessCount  int64           `json:"accessCount"`
	ReadBytes    int64           `json:"readBytes"`
	WriteBytes   int64           `json:"writeBytes"`
	AccessTime   time.Time       `json:"accessTime"`
	AccessPattern AccessPattern  `json:"accessPattern"`
	HeatLevel    DataHeatLevel   `json:"heatLevel"`
	CurrentTier  FlashType       `json:"currentTier"`
	LastModified time.Time       `json:"lastModified"`
}

// IOMetric IO 指标.
type IOMetric struct {
	Timestamp    time.Time `json:"timestamp"`
	ReadIOPS     int64     `json:"readIops"`
	WriteIOPS    int64     `json:"writeIops"`
	ReadBandwidth  int64   `json:"readBandwidth"`  // MB/s
	WriteBandwidth int64   `json:"writeBandwidth"` // MB/s
	AvgLatency   float64   `json:"avgLatency"`     // ms
	P99Latency   float64   `json:"p99Latency"`     // ms
}

// IOStatistics IO 统计.
type IOStatistics struct {
	TotalReads       int64       `json:"totalReads"`
	TotalWrites      int64       `json:"totalWrites"`
	TotalReadBytes   int64       `json:"totalReadBytes"`
	TotalWriteBytes  int64       `json:"totalWriteBytes"`
	AvgReadLatency   float64     `json:"avgReadLatency"`
	AvgWriteLatency  float64     `json:"avgWriteLatency"`
	HitRateL2ARC     float64     `json:"hitRateL2arc"`
	HitRateSLOG      float64     `json:"hitRateSlog"`
	RecentMetrics    []IOMetric  `json:"recentMetrics"`
}

// MigrateTask 迁移任务.
type MigrateTask struct {
	ID              string        `json:"id"`
	PolicyID        string        `json:"policyId,omitempty"`
	Status          MigrateStatus `json:"status"`
	CreatedAt       time.Time     `json:"createdAt"`
	StartedAt       time.Time     `json:"startedAt,omitempty"`
	CompletedAt     time.Time     `json:"completedAt,omitempty"`

	SourcePath      string        `json:"sourcePath"`
	TargetPath      string        `json:"targetPath"`
	SourceTier      FlashType     `json:"sourceTier"`
	TargetTier      FlashType     `json:"targetTier"`
	BlockSize       int64         `json:"blockSize"`

	TotalBlocks     int64         `json:"totalBlocks"`
	TotalBytes      int64         `json:"totalBytes"`
	ProcessedBlocks int64         `json:"processedBlocks"`
	ProcessedBytes  int64         `json:"processedBytes"`
	FailedBlocks    int64         `json:"failedBlocks"`

	Errors          []MigrateError `json:"errors,omitempty"`
}

// MigrateError 迁移错误.
type MigrateError struct {
	BlockID string    `json:"blockId"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// TieringConfig 分层引擎配置.
type TieringConfig struct {
	Enabled             bool          `json:"enabled"`
	CheckInterval       time.Duration `json:"checkInterval"`       // 检查间隔
	HeatCheckInterval   time.Duration `json:"heatCheckInterval"`   // 热度检查间隔
	AutoMigrateEnabled  bool          `json:"autoMigrateEnabled"`  // 自动迁移开关
	MaxConcurrentMigrates int       `json:"maxConcurrentMigrates"` // 最大并发迁移数
	SSDCapacityThreshold float64     `json:"ssdCapacityThreshold"` // SSD 容量阈值
	HotThreshold        int64         `json:"hotThreshold"`         // 热数据阈值
	WarmThreshold       int64         `json:"warmThreshold"`        // 温数据阈值
	ColdAgeHours        int           `json:"coldAgeHours"`         // 冷数据判断时长
	BlockSize           int64         `json:"blockSize"`            // 默认块大小
	MigrationWindowStart string       `json:"migrationWindowStart"` // 迁移窗口开始时间
	MigrationWindowEnd   string       `json:"migrationWindowEnd"`   // 迁移窗口结束时间
}

// DefaultTieringConfig 默认分层配置.
func DefaultTieringConfig() TieringConfig {
	return TieringConfig{
		Enabled:               true,
		CheckInterval:         5 * time.Minute,
		HeatCheckInterval:     1 * time.Minute,
		AutoMigrateEnabled:    true,
		MaxConcurrentMigrates: 4,
		SSDCapacityThreshold:  0.85,
		HotThreshold:          100,
		WarmThreshold:         10,
		ColdAgeHours:          720, // 30 天
		BlockSize:             128 * 1024, // 128KB
		MigrationWindowStart:  "02:00",
		MigrationWindowEnd:    "06:00",
	}
}

// HeatTrackingConfig 热度追踪配置.
type HeatTrackingConfig struct {
	Enabled          bool          `json:"enabled"`
	WindowSize       time.Duration `json:"windowSize"`       // 统计窗口
	DecayFactor      float64       `json:"decayFactor"`      // 衰减因子
	MinSampleCount   int64         `json:"minSampleCount"`   // 最小样本数
	MaxTrackedBlocks int           `json:"maxTrackedBlocks"` // 最大追踪块数
}

// DefaultHeatTrackingConfig 默认热度追踪配置.
func DefaultHeatTrackingConfig() HeatTrackingConfig {
	return HeatTrackingConfig{
		Enabled:          true,
		WindowSize:       1 * time.Hour,
		DecayFactor:      0.95,
		MinSampleCount:   5,
		MaxTrackedBlocks: 100000,
	}
}

// TieringStatus 分层状态.
type TieringStatus struct {
	Enabled          bool             `json:"enabled"`
	RunningTasks     int              `json:"runningTasks"`
	PendingTasks     int              `json:"pendingTasks"`
	LastMigration    time.Time        `json:"lastMigration,omitempty"`
	TotalBlocks      int64            `json:"totalBlocks"`
	SSDBlocks        int64            `json:"ssdBlocks"`
	HDDBlocks        int64            `json:"hddBlocks"`
	SSDUsagePercent  float64          `json:"ssdUsagePercent"`
	HDDUsagePercent  float64          `json:"hddUsagePercent"`
	HitRateL2ARC     float64          `json:"hitRateL2arc"`
	HitRateSLOG      float64          `json:"hitRateSlog"`
	Pools            []*HybridPool    `json:"policies"`
	Config           *TieringConfig   `json:"config"`
}

// EfficiencyReport 效率报告.
type EfficiencyReport struct {
	GeneratedAt      time.Time                    `json:"generatedAt"`
	Period           string                       `json:"period"`
	OverallHitRate   float64                      `json:"overallHitRate"`
	HitRateByTier    map[FlashType]float64        `json:"hitRateByTier"`
	PerformanceBoost float64                      `json:"performanceBoost"` // 性能提升百分比
	SpaceUtilization *SpaceUtilization             `json:"spaceUtilization"`
	TierDistribution map[FlashType]*TierDistStats  `json:"tierDistribution"`
	TopHotBlocks     []*BlockAccessRecord          `json:"topHotBlocks"`
	TopColdBlocks    []*BlockAccessRecord          `json:"topColdBlocks"`
	Recommendations  []string                      `json:"recommendations"`
}

// SpaceUtilization 空间利用率.
type SpaceUtilization struct {
	TotalCapacity int64   `json:"totalCapacity"`
	TotalUsed     int64   `json:"totalUsed"`
	SSDCapacity   int64   `json:"ssdCapacity"`
	SSDUsed       int64   `json:"ssdUsed"`
	HDDCapacity   int64   `json:"hddCapacity"`
	HDDUsed       int64   `json:"hddUsed"`
	SSDPercent    float64 `json:"ssdPercent"`
	HDDPercent    float64 `json:"hddPercent"`
}

// TierDistStats 层级分布统计.
type TierDistStats struct {
	FlashType   FlashType `json:"flashType"`
	BlockCount  int64     `json:"blockCount"`
	TotalBytes  int64     `json:"totalBytes"`
	HotBlocks   int64     `json:"hotBlocks"`
	WarmBlocks  int64     `json:"warmBlocks"`
	ColdBlocks  int64     `json:"coldBlocks"`
	FrozenBlocks int64    `json:"frozenBlocks"`
	HitRate     float64   `json:"hitRate"`
	AvgLatency  float64   `json:"avgLatency"`
}

// API 响应类型

// Response 标准 API 响应.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// StatusRequest 状态查询请求.
type StatusRequest struct {
	PoolID string `json:"poolId,omitempty"`
}

// ConfigRequest 配置请求.
type ConfigRequest struct {
	TieringConfig  *TieringConfig     `json:"tieringConfig,omitempty"`
	HeatConfig     *HeatTrackingConfig `json:"heatConfig,omitempty"`
	CachePolicies  []*CachePolicy     `json:"cachePolicies,omitempty"`
}

// ReportRequest 报告请求.
type ReportRequest struct {
	Period  string `json:"period"`  // daily, weekly, monthly
	PoolID  string `json:"poolId,omitempty"`
}
