package hybridpool

import (
	"time"
)

// PoolStatus 存储池状态.
type PoolStatus string

const (
	// PoolStatusOnline 在线状态.
	PoolStatusOnline PoolStatus = "online"
	// PoolStatusDegraded 降级状态.
	PoolStatusDegraded PoolStatus = "degraded"
	// PoolStatusFaulted 故障状态.
	PoolStatusFaulted PoolStatus = "faulted"
	// PoolStatusOffline 离线状态.
	PoolStatusOffline PoolStatus = "offline"
)

// TierType 存储层类型.
type TierType string

const (
	// TierTypeFlash Flash 存储层 (SSD/NVMe).
	TierTypeFlash TierType = "flash"
	// TierTypeHDD HDD 存储层.
	TierTypeHDD TierType = "hdd"
)

// DataClassification 数据分类.
type DataClassification string

const (
	// DataClassificationHot 热数据.
	DataClassificationHot DataClassification = "hot"
	// DataClassificationWarm 温数据.
	DataClassificationWarm DataClassification = "warm"
	// DataClassificationCold 冷数据.
	DataClassificationCold DataClassification = "cold"
)

// MigrationTrigger 迁移触发条件.
type MigrationTrigger string

const (
	// MigrationTriggerAccess 基于访问频率触发.
	MigrationTriggerAccess MigrationTrigger = "access"
	// MigrationTriggerAge 基于数据年龄触发.
	MigrationTriggerAge MigrationTrigger = "age"
	// MigrationTriggerSize 基于文件大小触发.
	MigrationTriggerSize MigrationTrigger = "size"
	// MigrationTriggerManual 手动触发.
	MigrationTriggerManual MigrationTrigger = "manual"
)

// HybridPool 混合存储池.
type HybridPool struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Status      PoolStatus `json:"status"`
	Description string     `json:"description,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`

	// 存储层配置
	FlashTier *TierConfig `json:"flashTier"`
	HDDTier   *TierConfig `json:"hddTier"`

	// 迁移策略
	MigrationPolicy *MigrationPolicy `json:"migrationPolicy"`

	// 容量信息
	TotalCapacity int64 `json:"totalCapacity"`
	UsedCapacity  int64 `json:"usedCapacity"`
	FlashUsed     int64 `json:"flashUsed"`
	HDDUsed       int64 `json:"hddUsed"`

	// 性能指标
	Performance *PerformanceMetrics `json:"performance,omitempty"`
}

// TierConfig 存储层配置.
type TierConfig struct {
	Type       TierType `json:"type"`
	Name       string   `json:"name"`
	Devices    []string `json:"devices"`    // 设备列表
	Path       string   `json:"path"`       // 挂载路径
	Capacity   int64    `json:"capacity"`   // 总容量 (bytes)
	Used       int64    `json:"used"`       // 已使用 (bytes)
	Available  int64    `json:"available"`  // 可用 (bytes)
	UsagePct   float64  `json:"usagePct"`   // 使用率 (%)
	RAIDLevel  string   `json:"raidLevel"`  // RAID 级别
	Enabled    bool     `json:"enabled"`
	Healthy    bool     `json:"healthy"`
}

// MigrationPolicy 数据迁移策略.
type MigrationPolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`

	// 迁移触发条件
	Trigger MigrationTrigger `json:"trigger"`

	// 热数据阈值
	HotAccessCount  int64         `json:"hotAccessCount"`  // 访问次数阈值
	HotAccessWindow time.Duration `json:"hotAccessWindow"` // 访问时间窗口

	// 冷数据阈值
	ColdAgeThreshold time.Duration `json:"coldAgeThreshold"` // 数据年龄阈值
	ColdAccessCount  int64         `json:"coldAccessCount"`  // 最大访问次数

	// 迁移限制
	MaxConcurrentMigrations int   `json:"maxConcurrentMigrations"` // 最大并发迁移数
	MinFileSize             int64 `json:"minFileSize"`             // 最小文件大小 (bytes)
	MaxFileSize             int64 `json:"maxFileSize"`             // 最大文件大小 (bytes)

	// 调度
	ScheduleEnabled bool   `json:"scheduleEnabled"`
	ScheduleCron    string `json:"scheduleCron,omitempty"` // Cron 表达式

	// 保护设置
	ReserveFlashPct float64 `json:"reserveFlashPct"` // Flash 预留空间 (%)
	VerifyAfterMove bool    `json:"verifyAfterMove"` // 迁移后校验
}

// PerformanceMetrics 性能指标.
type PerformanceMetrics struct {
	Timestamp time.Time `json:"timestamp"`

	// IOPS
	FlashIOPS   int64 `json:"flashIOPS"`
	HDDIOPS     int64 `json:"hddIOPS"`
	TotalIOPS   int64 `json:"totalIOPS"`
	ReadIOPS    int64 `json:"readIOPS"`
	WriteIOPS   int64 `json:"writeIOPS"`

	// 吞吐量 (bytes/sec)
	FlashThroughput int64 `json:"flashThroughput"`
	HDDThroughput   int64 `json:"hddThroughput"`
	TotalThroughput int64 `json:"totalThroughput"`
	ReadThroughput  int64 `json:"readThroughput"`
	WriteThroughput int64 `json:"writeThroughput"`

	// 延迟 (microseconds)
	FlashLatencyAvg int64 `json:"flashLatencyAvg"`
	HDDLatencyAvg   int64 `json:"hddLatencyAvg"`
	TotalLatencyAvg int64 `json:"totalLatencyAvg"`
	FlashLatencyP99 int64 `json:"flashLatencyP99"`
	HDDLatencyP99   int64 `json:"hddLatencyP99"`

	// 缓存命中率
	CacheHitRate  float64 `json:"cacheHitRate"`
	CacheMissRate float64 `json:"cacheMissRate"`
}

// PoolStats 存储池统计.
type PoolStats struct {
	PoolID    string    `json:"poolId"`
	PoolName  string    `json:"poolName"`
	Timestamp time.Time `json:"timestamp"`

	// 容量统计
	TotalCapacity int64   `json:"totalCapacity"`
	UsedCapacity  int64   `json:"usedCapacity"`
	FreeCapacity  int64   `json:"freeCapacity"`
	UsagePercent  float64 `json:"usagePercent"`

	// 分层统计
	FlashCapacity int64   `json:"flashCapacity"`
	FlashUsed     int64   `json:"flashUsed"`
	FlashFree     int64   `json:"flashFree"`
	FlashUsagePct float64 `json:"flashUsagePct"`
	HDDCapacity   int64   `json:"hddCapacity"`
	HDDUsed       int64   `json:"hddUsed"`
	HDDFree       int64   `json:"hddFree"`
	HDDUsagePct   float64 `json:"hddUsagePct"`

	// 数据分布
	HotDataSize  int64 `json:"hotDataSize"`
	WarmDataSize int64 `json:"warmDataSize"`
	ColdDataSize int64 `json:"coldDataSize"`

	// 性能指标
	Performance *PerformanceMetrics `json:"performance"`

	// 迁移统计
	PendingMigrations  int `json:"pendingMigrations"`
	ActiveMigrations   int `json:"activeMigrations"`
	CompletedMigrations int `json:"completedMigrations"`
}

// MigrationTask 迁移任务.
type MigrationTask struct {
	ID          string `json:"id"`
	PoolID      string `json:"poolId"`
	SourceTier  TierType `json:"sourceTier"`
	TargetTier  TierType `json:"targetTier"`

	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	StartedAt time.Time `json:"startedAt,omitempty"`
	EndedAt   time.Time `json:"endedAt,omitempty"`

	// 迁移文件
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	FileCount  int64  `json:"fileCount"`
	TotalBytes int64  `json:"totalBytes"`

	// 进度
	ProcessedFiles int64   `json:"processedFiles"`
	ProcessedBytes int64   `json:"processedBytes"`
	Progress       float64 `json:"progress"` // 0-100

	// 错误
	Errors []MigrationError `json:"errors,omitempty"`
}

// MigrationError 迁移错误.
type MigrationError struct {
	Path      string    `json:"path"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// CapacityPrediction 容量预测.
type CapacityPrediction struct {
	PoolID      string    `json:"poolId"`
	PredictedAt time.Time `json:"predictedAt"`

	// 当前使用
	CurrentUsage    int64   `json:"currentUsage"`
	CurrentUsagePct float64 `json:"currentUsagePct"`

	// 预测
	DaysUntilFull   int     `json:"daysUntilFull"`
	PredictedFullDate time.Time `json:"predictedFullDate"`

	// 趋势
	DailyGrowthBytes  int64   `json:"dailyGrowthBytes"`
	WeeklyGrowthBytes int64   `json:"weeklyGrowthBytes"`
	GrowthRate        float64 `json:"growthRate"` // 每日增长率 (%)

	// 建议
	Recommendations []string `json:"recommendations,omitempty"`
}

// DataMigrationRequest 数据迁移请求.
type DataMigrationRequest struct {
	SourceTier  TierType `json:"sourceTier"`
	TargetTier  TierType `json:"targetTier"`
	SourcePath  string   `json:"sourcePath,omitempty"`
	TargetPath  string   `json:"targetPath,omitempty"`
	DryRun      bool     `json:"dryRun"`
	Verify      bool     `json:"verify"`
}

// Response API 响应.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse 错误响应.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
