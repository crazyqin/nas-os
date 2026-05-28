package smartarchive

import (
	"time"
)

// StorageTier 存储层级类型.
type StorageTier string

const (
	// TierHot 热数据层：高性能 SSD，频繁访问.
	TierHot StorageTier = "hot"
	// TierWarm 温数据层：普通 SSD/HDD，偶尔访问.
	TierWarm StorageTier = "warm"
	// TierCold 冷数据层：大容量 HDD，很少访问.
	TierCold StorageTier = "cold"
	// TierIce 冰冻层：归档存储，几乎不访问.
	TierIce StorageTier = "ice"
)

// ArchiveAction 归档动作类型.
type ArchiveAction string

const (
	// ArchiveActionMove 移动数据到目标层.
	ArchiveActionMove ArchiveAction = "move"
	// ArchiveActionCompress 压缩后归档.
	ArchiveActionCompress ArchiveAction = "compress"
	// ArchiveActionDeduplicate 去重后归档.
	ArchiveActionDeduplicate ArchiveAction = "deduplicate"
	// ArchiveActionDelete 删除数据.
	ArchiveActionDelete ArchiveAction = "delete"
	// ArchiveActionSnapshot 快照归档.
	ArchiveActionSnapshot ArchiveAction = "snapshot"
)

// JobStatus 归档任务状态.
type JobStatus string

const (
	// JobStatusPending 待执行.
	JobStatusPending JobStatus = "pending"
	// JobStatusRunning 执行中.
	JobStatusRunning JobStatus = "running"
	// JobStatusCompleted 已完成.
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed 失败.
	JobStatusFailed JobStatus = "failed"
	// JobStatusCancelled 已取消.
	JobStatusCancelled JobStatus = "cancelled"
	// JobStatusPaused 已暂停.
	JobStatusPaused JobStatus = "paused"
)

// CompressionAlgorithm 压缩算法.
type CompressionAlgorithm string

const (
	// CompressionNone 不压缩.
	CompressionNone CompressionAlgorithm = "none"
	// CompressionGzip Gzip 压缩.
	CompressionGzip CompressionAlgorithm = "gzip"
	// CompressionZstd Zstandard 压缩（高速高压缩比）.
	CompressionZstd CompressionAlgorithm = "zstd"
	// CompressionLZ4 LZ4 压缩（极速低压缩比）.
	CompressionLZ4 CompressionAlgorithm = "lz4"
	// CompressionBrotli Brotli 压缩（适合文本）.
	CompressionBrotli CompressionAlgorithm = "brotli"
	// CompressionXZ XZ 压缩（高压缩比慢速）.
	CompressionXZ CompressionAlgorithm = "xz"
)

// RetentionAction 保留策略动作.
type RetentionAction string

const (
	// RetentionActionArchive 归档到冷存储.
	RetentionActionArchive RetentionAction = "archive"
	// RetentionActionDelete 删除数据.
	RetentionActionDelete RetentionAction = "delete"
	// RetentionActionNotify 通知管理员.
	RetentionActionNotify RetentionAction = "notify"
	// RetentionActionMoveToIce 移动到冰冻层.
	RetentionActionMoveToIce RetentionAction = "move_to_ice"
)

// ComplianceStatus 合规状态.
type ComplianceStatus string

const (
	// ComplianceStatusCompliant 合规.
	ComplianceStatusCompliant ComplianceStatus = "compliant"
	// ComplianceStatusNonCompliant 不合规.
	ComplianceStatusNonCompliant ComplianceStatus = "non_compliant"
	// ComplianceStatusWarning 警告.
	ComplianceStatusWarning ComplianceStatus = "warning"
	// ComplianceStatusExempt 豁免.
	ComplianceStatusExempt ComplianceStatus = "exempt"
)

// ArchivePolicy 归档策略.
type ArchivePolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"` // 优先级，数字越大越优先

	// 条件规则
	Conditions PolicyConditions `json:"conditions"`

	// 动作配置
	Action       ArchiveAction  `json:"action"`
	TargetTier   StorageTier    `json:"targetTier"`
	Compression  CompressionAlgorithm `json:"compression,omitempty"`

	// 调度配置
	Schedule     string `json:"schedule,omitempty"` // Cron 表达式
	BatchSize    int    `json:"batchSize"`          // 每批处理文件数
	MaxDuration  time.Duration `json:"maxDuration"` // 单次运行最大时长

	// 元数据
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedBy string    `json:"createdBy,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
}

// PolicyConditions 策略条件.
type PolicyConditions struct {
	// 访问频率条件
	MinAccessCount  int64         `json:"minAccessCount,omitempty"`
	MaxAccessCount  int64         `json:"maxAccessCount,omitempty"`
	LastAccessBefore time.Time    `json:"lastAccessBefore,omitempty"`
	AccessIdleDays  int           `json:"accessIdleDays,omitempty"` // 闲置天数

	// 时间条件
	CreatedBefore   time.Time     `json:"createdBefore,omitempty"`
	ModifiedBefore  time.Time     `json:"modifiedBefore,omitempty"`
	FileAgeDays     int           `json:"fileAgeDays,omitempty"` // 文件年龄（天）

	// 大小条件
	MinFileSize     int64         `json:"minFileSize,omitempty"` // 字节
	MaxFileSize     int64         `json:"maxFileSize,omitempty"` // 字节

	// 类型条件
	FileExtensions  []string      `json:"fileExtensions,omitempty"` // 文件扩展名
	MimeTypes       []string      `json:"mimeTypes,omitempty"`      // MIME 类型
	PathPatterns    []string      `json:"pathPatterns,omitempty"`   // 路径模式
	ExcludePatterns []string      `json:"excludePatterns,omitempty"`

	// 存储层条件
	SourceTiers     []StorageTier `json:"sourceTiers,omitempty"` // 仅处理这些层的数据

	// 自定义标签
	RequiredTags    []string      `json:"requiredTags,omitempty"`
	ExcludeTags     []string      `json:"excludeTags,omitempty"`
}

// ArchiveJob 归档任务.
type ArchiveJob struct {
	ID         string    `json:"id"`
	PolicyID   string    `json:"policyId,omitempty"`
	PolicyName string    `json:"policyName,omitempty"`
	Status     JobStatus `json:"status"`
	Action     ArchiveAction `json:"action"`

	// 时间信息
	CreatedAt   time.Time `json:"createdAt"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
	CompletedAt time.Time `json:"completedAt,omitempty"`

	// 目标配置
	SourceTier StorageTier `json:"sourceTier"`
	TargetTier StorageTier `json:"targetTier"`
	Compression CompressionAlgorithm `json:"compression,omitempty"`

	// 文件统计
	TotalFiles     int64 `json:"totalFiles"`
	TotalBytes     int64 `json:"totalBytes"`
	ProcessedFiles int64 `json:"processedFiles"`
	ProcessedBytes int64 `json:"processedBytes"`
	FailedFiles    int64 `json:"failedFiles"`
	SkippedFiles   int64 `json:"skippedFiles"`

	// 压缩统计
	OriginalBytes  int64 `json:"originalBytes,omitempty"`
	CompressedBytes int64 `json:"compressedBytes,omitempty"`

	// 错误信息
	Errors []ArchiveError `json:"errors,omitempty"`

	// 估算
	EstimatedDuration time.Duration `json:"estimatedDuration,omitempty"`
	EstimatedSaving   int64         `json:"estimatedSaving,omitempty"` // 预计节省空间
}

// ArchiveError 归档错误.
type ArchiveError struct {
	FilePath  string    `json:"filePath"`
	ErrorCode string    `json:"errorCode"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Retryable bool      `json:"retryable"`
}

// RetentionRule 保留规则.
type RetentionRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`

	// 保留条件
	Conditions RetentionConditions `json:"conditions"`

	// 保留动作
	Action      RetentionAction `json:"action"`
	GracePeriod time.Duration   `json:"gracePeriod"` // 宽限期

	// 合规配置
	ComplianceRequired bool   `json:"complianceRequired"` // 是否需要合规检查
	LegalHold          bool   `json:"legalHold"`          // 法律保留
	RegulationRef      string `json:"regulationRef,omitempty"` // 法规引用

	// 元数据
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RetentionConditions 保留条件.
type RetentionConditions struct {
	// 时间条件
	MaxAge          time.Duration `json:"maxAge,omitempty"`          // 最大保留时间
	MinAge          time.Duration `json:"minAge,omitempty"`          // 最小保留时间
	ExpiresBefore   time.Time     `json:"expiresBefore,omitempty"`  // 在此日期前过期

	// 标签条件
	RequiredTags    []string      `json:"requiredTags,omitempty"`
	ExcludeTags     []string      `json:"excludeTags,omitempty"`

	// 路径条件
	PathPatterns    []string      `json:"pathPatterns,omitempty"`
	ExcludePaths    []string      `json:"excludePaths,omitempty"`

	// 大小条件
	MinSize         int64         `json:"minSize,omitempty"`
	MaxSize         int64         `json:"maxSize,omitempty"`

	// 类型条件
	FileExtensions  []string      `json:"fileExtensions,omitempty"`
}

// StorageTierConfig 存储层配置.
type StorageTierConfig struct {
	Tier        StorageTier `json:"tier"`
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	Capacity    int64       `json:"capacity"`    // 字节
	Used        int64       `json:"used"`        // 字节
	Threshold   int         `json:"threshold"`   // 使用率阈值（百分比）
	CostPerGB   float64     `json:"costPerGB"`   // 每GB每月成本（元）
	IOPSMax     int         `json:"iopsMax"`     // 最大 IOPS
	ThroughputMax int64     `json:"throughputMax"` // 最大吞吐量（MB/s）
	Enabled     bool        `json:"enabled"`
	Encrypted   bool        `json:"encrypted"`   // 是否加密
	Compressed  bool        `json:"compressed"`  // 是否默认压缩
	Redundancy  int         `json:"redundancy"`  // 冗余副本数
}

// AccessPattern 访问模式.
type AccessPattern struct {
	FilePath     string    `json:"filePath"`
	Size         int64     `json:"size"`
	Extension    string    `json:"extension"`
	MimeType     string    `json:"mimeType,omitempty"`
	CurrentTier  StorageTier `json:"currentTier"`

	// 访问统计
	TotalAccesses  int64     `json:"totalAccesses"`
	LastAccess     time.Time `json:"lastAccess"`
	FirstAccess    time.Time `json:"firstAccess"`
	ReadCount      int64     `json:"readCount"`
	WriteCount     int64     `json:"writeCount"`
	ReadBytes      int64     `json:"readBytes"`
	WriteBytes     int64     `json:"writeBytes"`

	// 计算指标
	AccessFrequency float64       `json:"accessFrequency"` // 次/天
	IdleDuration    time.Duration `json:"idleDuration"`    // 闲置时长
	HeatScore       float64       `json:"heatScore"`       // 热度评分 0-100
	TrendScore      float64       `json:"trendScore"`      // 趋势评分（正=升温，负=降温）

	// 推荐
	RecommendedTier StorageTier   `json:"recommendedTier"`
	RecommendedAction ArchiveAction `json:"recommendedAction"`
	Confidence      float64       `json:"confidence"` // 推荐置信度 0-1

	// 元数据
	Tags        []string  `json:"tags,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	LastModified time.Time `json:"lastModified"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ArchiveRecord 归档记录.
type ArchiveRecord struct {
	ID           string        `json:"id"`
	JobID        string        `json:"jobId"`
	PolicyID     string        `json:"policyId,omitempty"`
	FilePath     string        `json:"filePath"`
	Action       ArchiveAction `json:"action"`
	SourceTier   StorageTier   `json:"sourceTier"`
	TargetTier   StorageTier   `json:"targetTier"`
	Compression  CompressionAlgorithm `json:"compression,omitempty"`

	// 大小统计
	OriginalSize   int64 `json:"originalSize"`
	ProcessedSize  int64 `json:"processedSize"`
	CompressedSize int64 `json:"compressedSize,omitempty"`

	// 校验
	ChecksumBefore string `json:"checksumBefore,omitempty"`
	ChecksumAfter  string `json:"checksumAfter,omitempty"`

	// 时间
	ArchivedAt time.Time `json:"archivedAt"`
	ExpiresAt  time.Time `json:"expiresAt,omitempty"`

	// 状态
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	Restored  bool   `json:"restored"` // 是否已恢复
}

// CostReport 成本报告.
type CostReport struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Period      string    `json:"period"` // 日/周/月

	// 当前成本
	CurrentCost  float64                  `json:"currentCost"`
	CostByTier   map[StorageTier]float64  `json:"costByTier"`
	CostByMonth  []MonthlyCost            `json:"costByMonth,omitempty"`

	// 存储统计
	TotalStorage   int64                    `json:"totalStorage"`
	StorageByTier  map[StorageTier]int64    `json:"storageByTier"`

	// 优化建议
	Suggestions    []CostSuggestion         `json:"suggestions,omitempty"`
	PotentialSaving float64                 `json:"potentialSaving"` // 潜在节省

	// 预测
	ForecastNextMonth float64               `json:"forecastNextMonth"`
	ForecastTrend     string                `json:"forecastTrend"` // up/down/stable
}

// MonthlyCost 月度成本.
type MonthlyCost struct {
	Month  string  `json:"month"` // YYYY-MM
	Cost   float64 `json:"cost"`
	Storage int64  `json:"storage"`
}

// CostSuggestion 成本优化建议.
type CostSuggestion struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"` // tier_migration/compression/dedup/cleanup
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Impact      string  `json:"impact"`      // high/medium/low
	SavingEst   float64 `json:"savingEst"`   // 预计节省金额
	SpaceSaving int64   `json:"spaceSaving"` // 预计节省空间
	Priority    int     `json:"priority"`
	AutoApply   bool    `json:"autoApply"` // 是否可自动执行
}

// AnalyzerConfig 分析器配置.
type AnalyzerConfig struct {
	// 分析间隔
	AnalysisInterval time.Duration `json:"analysisInterval"`

	// 热度计算参数
	HotThreshold     float64       `json:"hotThreshold"`     // 热数据阈值
	WarmThreshold    float64       `json:"warmThreshold"`    // 温数据阈值
	ColdThreshold    float64       `json:"coldThreshold"`    // 冷数据阈值
	DecayFactor      float64       `json:"decayFactor"`      // 衰减因子

	// 采样配置
	SampleSize       int           `json:"sampleSize"`       // 采样大小
	SampleInterval   time.Duration `json:"sampleInterval"`   // 采样间隔

	// 预测配置
	PredictionWindow time.Duration `json:"predictionWindow"` // 预测窗口
	TrendSensitivity float64       `json:"trendSensitivity"` // 趋势灵敏度

	// 存储配置
	StorageBackend   string        `json:"storageBackend"`   // memory/redis/sqlite
	StoragePath      string        `json:"storagePath"`
	MaxRecords       int           `json:"maxRecords"`
	RetentionDays    int           `json:"retentionDays"`
}

// DefaultAnalyzerConfig 默认分析器配置.
func DefaultAnalyzerConfig() AnalyzerConfig {
	return AnalyzerConfig{
		AnalysisInterval: 1 * time.Hour,
		HotThreshold:     80.0,
		WarmThreshold:    40.0,
		ColdThreshold:    10.0,
		DecayFactor:      0.95,
		SampleSize:       10000,
		SampleInterval:   5 * time.Minute,
		PredictionWindow: 7 * 24 * time.Hour,
		TrendSensitivity: 0.1,
		StorageBackend:   "memory",
		MaxRecords:       100000,
		RetentionDays:    90,
	}
}

// SchedulerConfig 调度器配置.
type SchedulerConfig struct {
	// 调度间隔
	MinInterval      time.Duration `json:"minInterval"`
	MaxInterval      time.Duration `json:"maxInterval"`
	AdaptiveInterval bool          `json:"adaptiveInterval"` // 自适应间隔

	// 执行限制
	MaxConcurrent    int           `json:"maxConcurrent"`    // 最大并发任务数
	MaxFilesPerJob   int           `json:"maxFilesPerJob"`   // 每个任务最大文件数
	MaxBytesPerJob   int64         `json:"maxBytesPerJob"`   // 每个任务最大字节数
	JobTimeout       time.Duration `json:"jobTimeout"`       // 任务超时

	// 重试配置
	MaxRetries       int           `json:"maxRetries"`
	RetryDelay       time.Duration `json:"retryDelay"`
	RetryBackoff     float64       `json:"retryBackoff"`     // 退避因子

	// 资源限制
	MaxCPUPercent    float64       `json:"maxCPUPercent"`    // CPU 使用率上限
	MaxIOPSPercent   float64       `json:"maxIOPSPercent"`   // IOPS 使用率上限
	QuietHours       []TimeRange   `json:"quietHours"`       // 静默时段
}

// TimeRange 时间范围.
type TimeRange struct {
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
}

// DefaultSchedulerConfig 默认调度器配置.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		MinInterval:      5 * time.Minute,
		MaxInterval:      1 * time.Hour,
		AdaptiveInterval: true,
		MaxConcurrent:    3,
		MaxFilesPerJob:   1000,
		MaxBytesPerJob:   10 * 1024 * 1024 * 1024, // 10GB
		JobTimeout:       2 * time.Hour,
		MaxRetries:       3,
		RetryDelay:       30 * time.Second,
		RetryBackoff:     2.0,
		MaxCPUPercent:    30.0,
		MaxIOPSPercent:   50.0,
		QuietHours: []TimeRange{
			{Start: "02:00", End: "06:00"},
		},
	}
}

// CompressionProfile 压缩配置文件.
type CompressionProfile struct {
	Algorithm   CompressionAlgorithm `json:"algorithm"`
	Level       int                  `json:"level"`       // 压缩级别 1-9
	MinSize     int64                `json:"minSize"`     // 最小文件大小
	Extensions  []string             `json:"extensions"`  // 适用扩展名
	SpeedScore  float64              `json:"speedScore"`  // 速度评分 0-100
	RatioScore  float64              `json:"ratioScore"`  // 压缩比评分 0-100
	CPUCost     float64              `json:"cpuCost"`     // CPU 成本 0-1
}

// ArchiveSummary 归档摘要.
type ArchiveSummary struct {
	GeneratedAt    time.Time `json:"generatedAt"`
	TotalArchived  int64     `json:"totalArchived"`
	TotalSize      int64     `json:"totalSize"`
	SavedSpace     int64     `json:"savedSpace"`
	SavedCost      float64   `json:"savedCost"`

	// 按层级统计
	TierStats      map[StorageTier]*TierArchiveStats `json:"tierStats"`

	// 按策略统计
	PolicyStats    map[string]*PolicyArchiveStats    `json:"policyStats"`

	// 最近任务
	RecentJobs     []*ArchiveJob                     `json:"recentJobs,omitempty"`

	// 健康状态
	HealthScore    float64                           `json:"healthScore"` // 0-100
	Issues         []string                          `json:"issues,omitempty"`
}

// TierArchiveStats 层级归档统计.
type TierArchiveStats struct {
	Tier          StorageTier `json:"tier"`
	TotalFiles    int64       `json:"totalFiles"`
	TotalSize     int64       `json:"totalSize"`
	CompressedSize int64      `json:"compressedSize"`
	CompressionRatio float64  `json:"compressionRatio"`
	Utilization   float64     `json:"utilization"`   // 使用率
	CostPerMonth  float64     `json:"costPerMonth"`
}

// PolicyArchiveStats 策略归档统计.
type PolicyArchiveStats struct {
	PolicyID      string      `json:"policyId"`
	PolicyName    string      `json:"policyName"`
	Executions    int64       `json:"executions"`
	FilesArchived int64       `json:"filesArchived"`
	TotalSize     int64       `json:"totalSize"`
	SavedSpace    int64       `json:"savedSpace"`
	LastExecution time.Time   `json:"lastExecution"`
	SuccessRate   float64     `json:"successRate"`
}

// AuditEntry 审计条目.
type AuditEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"eventType"` // policy_created/job_started/file_archived/...
	Actor     string    `json:"actor"`     // 操作者
	Resource  string    `json:"resource"`  // 资源路径/ID
	Action    string    `json:"action"`    // 动作
	Details   string    `json:"details,omitempty"`
	Status    string    `json:"status"` // success/failed
	IP        string    `json:"ip,omitempty"`
}
