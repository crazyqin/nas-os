// Package tiering 存储分层类型定义
package tiering

import (
	"time"
)

// TierType 存储层类型.
type TierType string

const (
	// TierTypeSSD SSD 存储层.
	TierTypeSSD TierType = "ssd"
	// TierTypeHDD HDD 存储层.
	TierTypeHDD TierType = "hdd"
	// TierTypeCloud 云存储层.
	TierTypeCloud TierType = "cloud"
	// TierTypeMemory 内存缓存层.
	TierTypeMemory TierType = "memory"
)

// TierConfig 存储层配置.
type TierConfig struct {
	Type       TierType `json:"type"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Capacity   int64    `json:"capacity"`
	Used       int64    `json:"used"`
	Enabled    bool     `json:"enabled"`
	Priority   int      `json:"priority"`
	Threshold  int      `json:"threshold"`
	ProviderID string   `json:"providerId,omitempty"`
	MinFree    int64    `json:"minFree,omitempty"`
}

// PolicyEngineConfig 策略引擎配置.
type PolicyEngineConfig struct {
	EnableAutoTier bool          `json:"enableAutoTier"`
	CheckInterval  time.Duration `json:"checkInterval"`
	MaxConcurrent  int           `json:"maxConcurrent"`
	HotThreshold   int64         `json:"hotThreshold,omitempty"`
	WarmThreshold  int64         `json:"warmThreshold,omitempty"`
	ColdAgeHours   int64         `json:"coldAgeHours,omitempty"`
}

// DefaultPolicyEngineConfig 默认策略引擎配置.
func DefaultPolicyEngineConfig() PolicyEngineConfig {
	return PolicyEngineConfig{
		EnableAutoTier: true,
		CheckInterval:  5 * time.Minute,
		MaxConcurrent:  3,
		HotThreshold:   100,
		WarmThreshold:  10,
		ColdAgeHours:   720, // 30 days
	}
}

// PolicyAction 策略动作类型.
type PolicyAction string

const (
	// PolicyActionMove 移动文件.
	PolicyActionMove PolicyAction = "move"
	// PolicyActionCopy 复制文件.
	PolicyActionCopy PolicyAction = "copy"
	// PolicyActionArchive 归档文件.
	PolicyActionArchive PolicyAction = "archive"
	// PolicyActionDelete 删除文件.
	PolicyActionDelete PolicyAction = "delete"
)

// PolicyStatus 策略状态.
type PolicyStatus string

const (
	// PolicyStatusEnabled 启用.
	PolicyStatusEnabled PolicyStatus = "enabled"
	// PolicyStatusDisabled 禁用.
	PolicyStatusDisabled PolicyStatus = "disabled"
)

// ScheduleType 调度类型.
type ScheduleType string

const (
	// ScheduleTypeManual 手动执行.
	ScheduleTypeManual ScheduleType = "manual"
	// ScheduleTypeInterval 间隔执行.
	ScheduleTypeInterval ScheduleType = "interval"
	// ScheduleTypeCron Cron 表达式执行.
	ScheduleTypeCron ScheduleType = "cron"
)

// Policy 分层策略.
type Policy struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Description     string        `json:"description,omitempty"`
	Enabled         bool          `json:"enabled"`
	Status          PolicyStatus  `json:"status"`
	SourceTier      TierType      `json:"sourceTier"`
	TargetTier      TierType      `json:"targetTier"`
	Action          PolicyAction  `json:"action"`
	FilePatterns    []string      `json:"filePatterns,omitempty"`
	ExcludePatterns []string      `json:"excludePatterns,omitempty"`
	MinAccessCount  int64         `json:"minAccessCount,omitempty"`
	MaxAccessAge    time.Duration `json:"maxAccessAge,omitempty"`
	MinFileSize     int64         `json:"minFileSize,omitempty"`
	MaxFileSize     int64         `json:"maxFileSize,omitempty"`
	ScheduleType    ScheduleType  `json:"scheduleType"`
	ScheduleExpr    string        `json:"scheduleExpr,omitempty"`
	DryRun          bool          `json:"dryRun"`
	PreserveOrigin  bool          `json:"preserveOrigin"`
	VerifyAfter     bool          `json:"verifyAfter,omitempty"`
	Priority        int           `json:"priority"`
	LastRun         time.Time     `json:"lastRun,omitempty"`
	NextRun         time.Time     `json:"nextRun,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

// AccessFrequency 访问频率.
type AccessFrequency string

const (
	// AccessFrequencyHot 热数据.
	AccessFrequencyHot AccessFrequency = "hot"
	// AccessFrequencyWarm 温数据.
	AccessFrequencyWarm AccessFrequency = "warm"
	// AccessFrequencyCold 冷数据.
	AccessFrequencyCold AccessFrequency = "cold"
)

// FileAccessRecord 文件访问记录.
type FileAccessRecord struct {
	Path          string          `json:"path"`
	Size          int64           `json:"size"`
	ModTime       time.Time       `json:"modTime"`
	CurrentTier   TierType        `json:"currentTier"`
	AccessCount   int64           `json:"accessCount"`
	AccessTime    time.Time       `json:"accessTime"`
	LastAccess    time.Time       `json:"lastAccess"`
	LastModified  time.Time       `json:"lastModified,omitempty"`
	Frequency     AccessFrequency `json:"frequency"`
	ReadBytes     int64           `json:"readBytes"`
	WriteBytes    int64           `json:"writeBytes"`
	AccessHistory []time.Time     `json:"accessHistory,omitempty"`
}

// MigrateStatus 迁移状态.
type MigrateStatus string

const (
	// MigrateStatusPending 等待中.
	MigrateStatusPending MigrateStatus = "pending"
	// MigrateStatusRunning 运行中.
	MigrateStatusRunning MigrateStatus = "running"
	// MigrateStatusCompleted 已完成.
	MigrateStatusCompleted MigrateStatus = "completed"
	// MigrateStatusFailed 失败.
	MigrateStatusFailed MigrateStatus = "failed"
	// MigrateStatusCancelled 已取消.
	MigrateStatusCancelled MigrateStatus = "cancelled"
)

// MigrateFile 迁移文件.
type MigrateFile struct {
	Path       string    `json:"path"`
	SourcePath string    `json:"sourcePath,omitempty"`
	TargetPath string    `json:"targetPath,omitempty"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"modTime"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}

// MigrateError 迁移错误.
type MigrateError struct {
	Path    string    `json:"path"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// MigrateTask 迁移任务.
type MigrateTask struct {
	ID              string         `json:"id"`
	PolicyID        string         `json:"policyId,omitempty"`
	Status          MigrateStatus  `json:"status"`
	SourceTier      TierType       `json:"sourceTier"`
	TargetTier      TierType       `json:"targetTier"`
	Action          PolicyAction   `json:"action"`
	TotalFiles      int            `json:"totalFiles"`
	TotalBytes      int64          `json:"totalBytes"`
	ProcessedFiles  int            `json:"processedFiles"`
	ProcessedBytes  int64          `json:"processedBytes"`
	FailedFiles     int            `json:"failedFiles"`
	Files           []MigrateFile  `json:"files"`
	Errors          []MigrateError `json:"errors,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	StartedAt       time.Time      `json:"startedAt,omitempty"`
	CompletedAt     time.Time      `json:"completedAt,omitempty"`
}

// MigrateRequest 迁移请求.
type MigrateRequest struct {
	Paths      []string     `json:"paths"`
	SourceTier TierType     `json:"sourceTier"`
	TargetTier TierType     `json:"targetTier"`
	Action     PolicyAction `json:"action"`
	Pattern    string       `json:"pattern,omitempty"`
	MinSize    int64        `json:"minSize,omitempty"`
	MaxSize    int64        `json:"maxSize,omitempty"`
	MinAge     time.Duration `json:"minAge,omitempty"`
	DryRun     bool         `json:"dryRun"`
	Preserve   bool         `json:"preserve"`
}

// Status 分层状态.
type Status struct {
	Enabled       bool                    `json:"enabled"`
	RunningTasks  int                     `json:"runningTasks"`
	PendingTasks  int                     `json:"pendingTasks"`
	LastMigration time.Time               `json:"lastMigration"`
	Tiers         map[TierType]*TierConfig `json:"tiers"`
	Policies      int                     `json:"policies"`
	ActivePolicy  int                     `json:"activePolicy"`
}

// TierStats 存储层统计.
type TierStats struct {
	Type          TierType `json:"type"`
	Name          string   `json:"name"`
	Capacity      int64    `json:"capacity"`
	Used          int64    `json:"used"`
	Available     int64    `json:"available"`
	UsagePercent  float64  `json:"usagePercent"`
	TotalFiles    int64    `json:"totalFiles"`
	TotalBytes    int64    `json:"totalBytes"`
	HotFiles      int64    `json:"hotFiles"`
	HotBytes      int64    `json:"hotBytes"`
	WarmFiles     int64    `json:"warmFiles"`
	WarmBytes     int64    `json:"warmBytes"`
	ColdFiles     int64    `json:"coldFiles"`
	ColdBytes     int64    `json:"coldBytes"`
	LastUpdated   time.Time `json:"lastUpdated"`
}

// AccessStats 访问统计.
type AccessStats struct {
	TotalAccesses   int64                   `json:"totalAccesses"`
	TotalRecords    int64                   `json:"totalRecords"`
	TotalBytes      int64                   `json:"totalBytes"`
	TotalFiles      int64                   `json:"totalFiles"`
	TotalReadBytes  int64                   `json:"totalReadBytes"`
	TotalWriteBytes int64                   `json:"totalWriteBytes"`
	HotFiles        int64                   `json:"hotFiles"`
	WarmFiles       int64                   `json:"warmFiles"`
	ColdFiles       int64                   `json:"coldFiles"`
	ByTier          map[TierType]*TierStats `json:"byTier"`
	LastUpdated     time.Time               `json:"lastUpdated"`
}

// StatsReport 统计报告.
type StatsReport struct {
	GeneratedAt time.Time                `json:"generatedAt"`
	Tiers       map[TierType]*TierStats  `json:"tiers"`
	Summary     *Summary                 `json:"summary"`
}

// Summary 总体统计.
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

// SSDCacheOptimizeResult SSD 缓存优化结果.
type SSDCacheOptimizeResult struct {
	StartTime          time.Time `json:"startTime"`
	EndTime            time.Time `json:"endTime"`
	Duration           time.Duration `json:"duration"`
	Tier               TierType  `json:"tier"`
	ColdFilesIdentified int      `json:"coldFilesIdentified"`
	HotFilesIdentified  int      `json:"hotFilesIdentified"`
	DemotedFiles       int       `json:"demotedFiles"`
	DemotedBytes       int64     `json:"demotedBytes"`
	PromotedFiles      int       `json:"promotedFiles"`
	PromotedBytes      int64     `json:"promotedBytes"`
	FailedDemotions    int       `json:"failedDemotions"`
	FailedPromotions   int       `json:"failedPromotions"`
	Tasks              []string  `json:"tasks"`
}

// AutoMigrateResult 自动迁移结果.
type AutoMigrateResult struct {
	StartTime time.Time                      `json:"startTime"`
	EndTime   time.Time                      `json:"endTime"`
	Duration  time.Duration                  `json:"duration"`
	Tiers     map[TierType]*TierMigrationStats `json:"tiers"`
}

// TierMigrationStats 存储层迁移统计.
type TierMigrationStats struct {
	TierType         TierType             `json:"tierType"`
	FilesToMigrate   []*FileAccessRecord  `json:"filesToMigrate,omitempty"`
	TotalMigrateBytes int64               `json:"totalMigrateBytes"`
}
