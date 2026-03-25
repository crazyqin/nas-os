package tiering

import (
	"time"
)

// TierType 存储层级类型.
type TierType string

const (
	// TierTypeSSD SSD 缓存层.
	TierTypeSSD TierType = "ssd"
	// TierTypeHDD HDD 存储层.
	TierTypeHDD TierType = "hdd"
	// TierTypeCloud 云存储归档层.
	TierTypeCloud TierType = "cloud"
	// TierTypeMemory 内存缓存层（可选）.
	TierTypeMemory TierType = "memory"
)

// PolicyAction 策略动作.
type PolicyAction string

const (
	// PolicyActionMove 移动数据.
	PolicyActionMove PolicyAction = "move"
	// PolicyActionCopy 复制数据.
	PolicyActionCopy PolicyAction = "copy"
	// PolicyActionArchive 归档数据.
	PolicyActionArchive PolicyAction = "archive"
	// PolicyActionDelete 删除冷数据.
	PolicyActionDelete PolicyAction = "delete"
)

// PolicyStatus 策略状态.
type PolicyStatus string

const (
	// PolicyStatusEnabled 策略已启用.
	PolicyStatusEnabled PolicyStatus = "enabled"
	// PolicyStatusDisabled 策略已禁用.
	PolicyStatusDisabled PolicyStatus = "disabled"
)

// MigrateStatus 迁移状态.
type MigrateStatus string

const (
	// MigrateStatusPending 迁移待执行.
	MigrateStatusPending MigrateStatus = "pending"
	// MigrateStatusRunning 迁移执行中.
	MigrateStatusRunning MigrateStatus = "running"
	// MigrateStatusCompleted 迁移已完成.
	MigrateStatusCompleted MigrateStatus = "completed"
	// MigrateStatusFailed 迁移失败.
	MigrateStatusFailed MigrateStatus = "failed"
	// MigrateStatusCancelled 迁移已取消.
	MigrateStatusCancelled MigrateStatus = "cancelled"
)

// AccessFrequency 访问频率级别.
type AccessFrequency string

const (
	// AccessFrequencyHot 热数据：频繁访问.
	AccessFrequencyHot AccessFrequency = "hot"
	// AccessFrequencyWarm 温数据：偶尔访问.
	AccessFrequencyWarm AccessFrequency = "warm"
	// AccessFrequencyCold 冷数据：很少访问.
	AccessFrequencyCold AccessFrequency = "cold"
)

// TierConfig 存储层配置.
type TierConfig struct {
	Type       TierType `json:"type"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`       // 存储路径
	Capacity   int64    `json:"capacity"`   // 容量（字节）
	Used       int64    `json:"used"`       // 已使用（字节）
	Threshold  int      `json:"threshold"`  // 使用阈值（百分比）
	Priority   int      `json:"priority"`   // 优先级（数字越大优先级越高）
	Enabled    bool     `json:"enabled"`    // 是否启用
	ProviderID string   `json:"providerId"` // 云提供商 ID（仅云层）
}

// Policy 分层策略.
type Policy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Enabled     bool         `json:"enabled"`
	Status      PolicyStatus `json:"status"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`

	// 分层规则
	SourceTier TierType     `json:"sourceTier"` // 源存储层
	TargetTier TierType     `json:"targetTier"` // 目标存储层
	Action     PolicyAction `json:"action"`     // 执行动作

	// 条件
	MinAccessCount  int64         `json:"minAccessCount"`  // 最小访问次数
	MaxAccessAge    time.Duration `json:"maxAccessAge"`    // 最大访问间隔（小时）
	MinFileSize     int64         `json:"minFileSize"`     // 最小文件大小（字节）
	MaxFileSize     int64         `json:"maxFileSize"`     // 最大文件大小（字节）
	FilePatterns    []string      `json:"filePatterns"`    // 文件匹配模式
	ExcludePatterns []string      `json:"excludePatterns"` // 排除模式

	// 调度
	ScheduleType ScheduleType `json:"scheduleType"` // 调度类型
	ScheduleExpr string       `json:"scheduleExpr"` // Cron 表达式
	LastRun      time.Time    `json:"lastRun,omitempty"`
	NextRun      time.Time    `json:"nextRun,omitempty"`

	// 高级选项
	DryRun         bool `json:"dryRun"`         // 试运行模式
	PreserveOrigin bool `json:"preserveOrigin"` // 保留原文件
	VerifyAfter    bool `json:"verifyAfter"`    // 迁移后验证
}

// ScheduleType 调度类型.
type ScheduleType string

const (
	// ScheduleTypeManual 手动调度.
	ScheduleTypeManual ScheduleType = "manual"
	// ScheduleTypeInterval 间隔调度.
	ScheduleTypeInterval ScheduleType = "interval"
	// ScheduleTypeCron Cron 表达式调度.
	ScheduleTypeCron ScheduleType = "cron"
)

// FileAccessRecord 文件访问记录.
type FileAccessRecord struct {
	Path         string          `json:"path"`
	Size         int64           `json:"size"`
	ModTime      time.Time       `json:"modTime"`
	AccessTime   time.Time       `json:"accessTime"`
	AccessCount  int64           `json:"accessCount"`
	ReadBytes    int64           `json:"readBytes"`
	WriteBytes   int64           `json:"writeBytes"`
	CurrentTier  TierType        `json:"currentTier"`
	Frequency    AccessFrequency `json:"frequency"`
	LastModified time.Time       `json:"lastModified"`
}

// MigrateTask 迁移任务.
type MigrateTask struct {
	ID          string        `json:"id"`
	PolicyID    string        `json:"policyId,omitempty"`
	Status      MigrateStatus `json:"status"`
	CreatedAt   time.Time     `json:"createdAt"`
	StartedAt   time.Time     `json:"startedAt,omitempty"`
	CompletedAt time.Time     `json:"completedAt,omitempty"`

	// 迁移配置
	SourcePath string       `json:"sourcePath"`
	TargetPath string       `json:"targetPath"`
	SourceTier TierType     `json:"sourceTier"`
	TargetTier TierType     `json:"targetTier"`
	Action     PolicyAction `json:"action"`

	// 文件列表
	Files []MigrateFile `json:"files,omitempty"`

	// 进度
	TotalFiles     int64 `json:"totalFiles"`
	TotalBytes     int64 `json:"totalBytes"`
	ProcessedFiles int64 `json:"processedFiles"`
	ProcessedBytes int64 `json:"processedBytes"`
	FailedFiles    int64 `json:"failedFiles"`

	// 错误
	Errors []MigrateError `json:"errors,omitempty"`
}

// MigrateFile 迁移文件.
type MigrateFile struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"modTime"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	SourcePath string    `json:"sourcePath"`
	TargetPath string    `json:"targetPath"`
}

// MigrateError 迁移错误.
type MigrateError struct {
	Path    string    `json:"path"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// TierStats 存储层统计.
type TierStats struct {
	Type         TierType `json:"type"`
	Name         string   `json:"name"`
	TotalFiles   int64    `json:"totalFiles"`
	TotalBytes   int64    `json:"totalBytes"`
	HotFiles     int64    `json:"hotFiles"`
	HotBytes     int64    `json:"hotBytes"`
	WarmFiles    int64    `json:"warmFiles"`
	WarmBytes    int64    `json:"warmBytes"`
	ColdFiles    int64    `json:"coldFiles"`
	ColdBytes    int64    `json:"coldBytes"`
	Capacity     int64    `json:"capacity"`
	Used         int64    `json:"used"`
	Available    int64    `json:"available"`
	UsagePercent float64  `json:"usagePercent"`
}

// AccessStats 访问统计.
type AccessStats struct {
	TotalFiles      int64     `json:"totalFiles"`
	TotalAccesses   int64     `json:"totalAccesses"`
	TotalReadBytes  int64     `json:"totalReadBytes"`
	TotalWriteBytes int64     `json:"totalWriteBytes"`
	HotFiles        int64     `json:"hotFiles"`
	WarmFiles       int64     `json:"warmFiles"`
	ColdFiles       int64     `json:"coldFiles"`
	LastUpdated     time.Time `json:"lastUpdated"`

	// 按存储层统计
	ByTier map[TierType]*TierStats `json:"byTier"`

	// Top 文件
	TopAccessedFiles []FileAccessRecord `json:"topAccessedFiles,omitempty"`
	TopColdFiles     []FileAccessRecord `json:"topColdFiles,omitempty"`
}

// MigrateRequest 手动迁移请求.
type MigrateRequest struct {
	Paths      []string      `json:"paths"`      // 文件/目录路径
	SourceTier TierType      `json:"sourceTier"` // 源存储层
	TargetTier TierType      `json:"targetTier"` // 目标存储层
	Action     PolicyAction  `json:"action"`     // 执行动作
	DryRun     bool          `json:"dryRun"`     // 试运行
	Preserve   bool          `json:"preserve"`   // 保留原文件
	Pattern    string        `json:"pattern"`    // 文件匹配模式
	MinSize    int64         `json:"minSize"`    // 最小文件大小
	MaxSize    int64         `json:"maxSize"`    // 最大文件大小
	MinAge     time.Duration `json:"minAge"`     // 最小文件年龄（小时）
}

// Status 分层状态.
type Status struct {
	Enabled       bool                     `json:"enabled"`
	RunningTasks  int                      `json:"runningTasks"`
	PendingTasks  int                      `json:"pendingTasks"`
	LastMigration time.Time                `json:"lastMigration,omitempty"`
	Tiers         map[TierType]*TierConfig `json:"tiers"`
	Policies      int                      `json:"policies"`
	ActivePolicy  int                      `json:"activePolicy"`
}

// PolicyEngineConfig 策略引擎配置.
type PolicyEngineConfig struct {
	CheckInterval  time.Duration `json:"checkInterval"`  // 检查间隔
	HotThreshold   int64         `json:"hotThreshold"`   // 热数据访问次数阈值
	WarmThreshold  int64         `json:"warmThreshold"`  // 温数据访问次数阈值
	ColdAgeHours   int           `json:"coldAgeHours"`   // 冷数据判断时长（小时）
	MaxConcurrent  int           `json:"maxConcurrent"`  // 最大并发迁移数
	EnableAutoTier bool          `json:"enableAutoTier"` // 启用自动分层
}

// DefaultPolicyEngineConfig 默认策略引擎配置.
func DefaultPolicyEngineConfig() PolicyEngineConfig {
	return PolicyEngineConfig{
		CheckInterval:  1 * time.Hour,
		HotThreshold:   100,
		WarmThreshold:  10,
		ColdAgeHours:   720, // 30 天
		MaxConcurrent:  5,
		EnableAutoTier: true,
	}
}

// StatisticsConfig 统计配置.
type StatisticsConfig struct {
	TrackInterval  time.Duration `json:"trackInterval"`  // 追踪间隔
	RetentionDays  int           `json:"retentionDays"`  // 保留天数
	MaxRecords     int           `json:"maxRecords"`     // 最大记录数
	EnableHotCold  bool          `json:"enableHotCold"`  // 启用热冷分析
	StorageBackend string        `json:"storageBackend"` // 存储后端 (sqlite/redis/memory)
	StoragePath    string        `json:"storagePath"`    // 存储路径
	Enabled        bool          `json:"enabled"`        // 是否启用
}

// MigrationTask 迁移任务（用于 MigrationEngine）.
type MigrationTask struct {
	ID          string        `json:"id"`
	PolicyID    string        `json:"policyId,omitempty"`
	Status      MigrateStatus `json:"status"`
	CreatedAt   time.Time     `json:"createdAt"`
	StartedAt   time.Time     `json:"startedAt,omitempty"`
	CompletedAt time.Time     `json:"completedAt,omitempty"`

	// 迁移配置
	SourcePath string       `json:"sourcePath"`
	TargetPath string       `json:"targetPath"`
	SourceTier TierType     `json:"sourceTier"`
	TargetTier TierType     `json:"targetTier"`
	Action     PolicyAction `json:"action"`

	// 文件列表
	Files []MigrateFile `json:"files,omitempty"`

	// 进度
	TotalFiles     int64 `json:"totalFiles"`
	TotalBytes     int64 `json:"totalBytes"`
	ProcessedFiles int64 `json:"processedFiles"`
	ProcessedBytes int64 `json:"processedBytes"`
	FailedFiles    int64 `json:"failedFiles"`

	// 错误
	Errors []MigrateError `json:"errors,omitempty"`
}

// TierSpec 存储层规格.
type TierSpec struct {
	Type       TierType `json:"type"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Capacity   int64    `json:"capacity"`
	Used       int64    `json:"used"`
	Threshold  int      `json:"threshold"`
	Priority   int      `json:"priority"`
	Enabled    bool     `json:"enabled"`
	ProviderID string   `json:"providerId,omitempty"`
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

// ==================== v2.4.0 新增类型 ====================

// SSDCacheOptimizeResult SSD缓存优化结果.
type SSDCacheOptimizeResult struct {
	StartTime time.Time     `json:"startTime"`
	EndTime   time.Time     `json:"endTime"`
	Duration  time.Duration `json:"duration"`
	Tier      TierType      `json:"tier"`

	// 冷数据统计
	ColdFilesIdentified int   `json:"coldFilesIdentified"`
	DemotedFiles        int   `json:"demotedFiles"`
	DemotedBytes        int64 `json:"demotedBytes"`
	FailedDemotions     int   `json:"failedDemotions"`

	// 热数据统计
	HotFilesIdentified int   `json:"hotFilesIdentified"`
	PromotedFiles      int   `json:"promotedFiles"`
	PromotedBytes      int64 `json:"promotedBytes"`
	FailedPromotions   int   `json:"failedPromotions"`

	// 迁移任务ID列表
	Tasks []string `json:"tasks"`
}

// AutoMigrateResult 自动迁移结果.
type AutoMigrateResult struct {
	StartTime time.Time     `json:"startTime"`
	EndTime   time.Time     `json:"endTime"`
	Duration  time.Duration `json:"duration"`

	// 按存储层的迁移统计
	Tiers map[TierType]*TierMigrationStats `json:"tiers"`
}

// TierMigrationStats 存储层迁移统计.
type TierMigrationStats struct {
	TierType          TierType            `json:"tierType"`
	FilesToMigrate    []*FileAccessRecord `json:"filesToMigrate,omitempty"`
	TotalMigrateBytes int64               `json:"totalMigrateBytes"`
	MigratedFiles     int                 `json:"migratedFiles"`
	FailedFiles       int                 `json:"failedFiles"`
}

// StatsReport 分层统计报告.
type StatsReport struct {
	GeneratedAt time.Time               `json:"generatedAt"`
	Tiers       map[TierType]*TierStats `json:"tiers"`
	Summary     *Summary                `json:"summary"`
}

// Summary 分层统计摘要.
type Summary struct {
	TotalFiles   int64   `json:"totalFiles"`
	TotalBytes   int64   `json:"totalBytes"`
	TotalHot     int64   `json:"totalHot"`
	TotalWarm    int64   `json:"totalWarm"`
	TotalCold    int64   `json:"totalCold"`
	HotPercent   float64 `json:"hotPercent"`
	WarmPercent  float64 `json:"warmPercent"`
	ColdPercent  float64 `json:"coldPercent"`
	HitRateSSD   float64 `json:"hitRateSSD"`
	MigrateTasks int     `json:"migrateTasks"`
	ActivePolicy int     `json:"activePolicy"`
}
