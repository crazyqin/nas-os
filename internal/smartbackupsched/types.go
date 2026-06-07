// Package smartbackupsched 实现 AI 智能备份调度器，基于数据变更频率和模式自动调度备份任务。
package smartbackupsched

import (
	"time"
)

// ========== 调度策略 ==========

// Strategy 备份策略类型.
type Strategy string

const (
	// StrategyFull 全量备份.
	StrategyFull Strategy = "full"
	// StrategyIncremental 增量备份.
	StrategyIncremental Strategy = "incremental"
	// StrategyDifferential 差异备份.
	StrategyDifferential Strategy = "differential"
	// StrategyAuto 由 AI 自动选择最优策略.
	StrategyAuto Strategy = "auto"
)

// BackupTier 备份层级.
type BackupTier string

const (
	// TierLocal 本地备份.
	TierLocal BackupTier = "local"
	// TierRemote 异地备份.
	TierRemote BackupTier = "remote"
	// TierCloud 云端备份.
	TierCloud BackupTier = "cloud"
)

// RiskLevel 风险等级.
type RiskLevel string

const (
	// RiskLow 低风险.
	RiskLow RiskLevel = "low"
	// RiskMedium 中等风险.
	RiskMedium RiskLevel = "medium"
	// RiskHigh 高风险.
	RiskHigh RiskLevel = "high"
	// RiskCritical 极高风险.
	RiskCritical RiskLevel = "critical"
)

// TaskStatus 任务状态.
type TaskStatus string

const (
	// StatusPending 待执行.
	StatusPending TaskStatus = "pending"
	// StatusRunning 执行中.
	StatusRunning TaskStatus = "running"
	// StatusCompleted 已完成.
	StatusCompleted TaskStatus = "completed"
	// StatusFailed 失败.
	StatusFailed TaskStatus = "failed"
	// StatusRetrying 重试中.
	StatusRetrying TaskStatus = "retrying"
	// StatusDegraded 已降级.
	StatusDegraded TaskStatus = "degraded"
	// StatusCancelled 已取消.
	StatusCancelled TaskStatus = "cancelled"
)

// ========== 核心数据结构 ==========

// ScheduleConfig 智能调度配置.
type ScheduleConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// 备份源和目标
	SourcePath  string       `json:"sourcePath"`
	TargetPaths []TargetPath `json:"targetPaths"`

	// 策略配置
	Strategy       Strategy `json:"strategy"`
	AutoStrategy   bool     `json:"autoStrategy"`             // 启用 AI 自动策略选择
	ForceFullAfter int      `json:"forceFullAfter,omitempty"` // 每 N 次增量后强制全量

	// 调度窗口
	BackupWindows []BackupWindow `json:"backupWindows"`
	PeakHours     []TimeRange    `json:"peakHours,omitempty"` // 业务高峰期，避开调度

	// 重试策略
	MaxRetries     int           `json:"maxRetries"`
	RetryInterval  time.Duration `json:"retryInterval"`
	DegradedOnFail bool          `json:"degradedOnFail"` // 失败后是否降级到下一 Tier

	// 保留策略
	RetentionDays int `json:"retentionDays"`
	MaxBackups    int `json:"maxBackups"`

	// 容量规划
	StorageWarnPercent  int `json:"storageWarnPercent"`  // 存储空间预警阈值百分比
	StorageLimitPercent int `json:"storageLimitPercent"` // 存储空间限制阈值百分比

	// 元数据
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TargetPath 备份目标路径.
type TargetPath struct {
	Tier     BackupTier `json:"tier"` // local, remote, cloud
	Path     string     `json:"path"`
	Host     string     `json:"host,omitempty"` // 异地/云端主机
	User     string     `json:"user,omitempty"`
	Port     int        `json:"port,omitempty"`
	Priority int        `json:"priority"` // 优先级，数字越小优先级越高
	Enabled  bool       `json:"enabled"`
}

// BackupWindow 备份窗口.
type BackupWindow struct {
	Name      string   `json:"name"`
	StartTime string   `json:"startTime"`          // HH:MM 格式
	EndTime   string   `json:"endTime"`            // HH:MM 格式
	Days      []string `json:"days"`               // monday, tuesday, ...
	Strategy  Strategy `json:"strategy,omitempty"` // 可选，覆盖全局策略
}

// TimeRange 时间范围.
type TimeRange struct {
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
}

// ========== 变更分析 ==========

// ChangePattern 数据变更模式.
type ChangePattern struct {
	Path            string        `json:"path"`
	ChangeFrequency float64       `json:"changeFrequency"` // 每小时平均变更次数
	ChangeRate      float64       `json:"changeRate"`      // 变更数据量占总量比率
	AvgChangeSize   int64         `json:"avgChangeSize"`   // 平均每次变更大小（字节）
	PeakChangeHour  int           `json:"peakChangeHour"`  // 变更高峰小时
	LastChangeAt    time.Time     `json:"lastChangeAt"`
	SampleDuration  time.Duration `json:"sampleDuration"`
	TotalChanges    int64         `json:"totalChanges"`
}

// StrategyRecommendation AI 策略推荐.
type StrategyRecommendation struct {
	Recommended       Strategy      `json:"recommended"`
	Confidence        float64       `json:"confidence"` // 0-1
	Reasons           []string      `json:"reasons"`
	EstimatedDuration time.Duration `json:"estimatedDuration"`
	EstimatedSize     int64         `json:"estimatedSize"` // 预估备份大小（字节）
}

// ========== 风险评估 ==========

// RiskAssessment 风险评估结果.
type RiskAssessment struct {
	Level           RiskLevel    `json:"level"`
	Score           float64      `json:"score"`       // 0-100，越高越危险
	SuccessRate     float64      `json:"successRate"` // 预测成功率 0-100
	Factors         []RiskFactor `json:"factors"`
	Recommendations []string     `json:"recommendations"`
	AssessedAt      time.Time    `json:"assessedAt"`
}

// RiskFactor 风险因素.
type RiskFactor struct {
	Name        string  `json:"name"`
	Impact      float64 `json:"impact"` // 对总分的贡献 0-100
	Description string  `json:"description"`
}

// ========== 容量规划 ==========

// CapacityForecast 容量预测.
type CapacityForecast struct {
	CurrentUsage    int64     `json:"currentUsage"`    // 当前使用量（字节）
	TotalCapacity   int64     `json:"totalCapacity"`   // 总容量（字节"`
	UsagePercent    float64   `json:"usagePercent"`    // 使用率百分比
	PredictedGrowth int64     `json:"predictedGrowth"` // 未来7天预估增长量
	DaysUntilFull   int       `json:"daysUntilFull"`   // 预计几天后满，-1 表示不担心
	ForecastDate    time.Time `json:"forecastDate"`
}

// ========== 备份任务 ==========

// BackupTask 备份任务.
type BackupTask struct {
	ID         string     `json:"id"`
	ConfigID   string     `json:"configId"`
	Strategy   Strategy   `json:"strategy"`
	Tier       BackupTier `json:"tier"`
	Status     TaskStatus `json:"status"`
	SourcePath string     `json:"sourcePath"`
	TargetPath string     `json:"targetPath"`

	// 进度
	Progress   int   `json:"progress"` // 0-100
	TotalSize  int64 `json:"totalSize"`
	TotalFiles int64 `json:"totalFiles"`
	Speed      int64 `json:"speed"` // bytes/sec

	// 时间
	StartTime   time.Time  `json:"startTime"`
	EndTime     time.Time  `json:"endTime,omitempty"`
	NextRetryAt *time.Time `json:"nextRetryAt,omitempty"`

	// 重试信息
	RetryCount int    `json:"retryCount"`
	MaxRetries int    `json:"maxRetries"`
	Error      string `json:"error,omitempty"`

	// 风险评估
	RiskAssessment *RiskAssessment `json:"riskAssessment,omitempty"`

	// 审计
	CreatedBy string `json:"createdBy"` // "scheduler", "manual", "retry"
}

// ========== 审计日志 ==========

// AuditEntry 审计日志条目.
type AuditEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"` // create, update, delete, execute, retry, degrade, alert
	ConfigID  string    `json:"configId,omitempty"`
	TaskID    string    `json:"taskId,omitempty"`
	Actor     string    `json:"actor"` // "system", "scheduler", "user"
	Details   string    `json:"details"`
	Success   bool      `json:"success"`
}

// ========== 统计 ==========

// SchedulerStats 调度器统计.
type SchedulerStats struct {
	TotalConfigs    int     `json:"totalConfigs"`
	EnabledConfigs  int     `json:"enabledConfigs"`
	TotalTasks      int     `json:"totalTasks"`
	RunningTasks    int     `json:"runningTasks"`
	CompletedTasks  int     `json:"completedTasks"`
	FailedTasks     int     `json:"failedTasks"`
	RetryingTasks   int     `json:"retryingTasks"`
	SuccessRate     float64 `json:"successRate"`
	AvgDuration     int64   `json:"avgDuration"` // 平均耗时（秒）
	TotalBackupSize int64   `json:"totalBackupSize"`
}

// ========== 辅助 ==========

// HealthCheckResult 健康检查结果.
type HealthCheckResult struct {
	Status    string                 `json:"status"` // healthy, degraded, unhealthy
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details"`
}
