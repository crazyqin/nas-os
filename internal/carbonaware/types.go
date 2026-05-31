// Package carbonaware 实现碳感知调度器
// 根据电网碳强度调度高能耗任务（备份、去重、转码等）
// 支持碳排放追踪、绿色能源时段识别、节能模式切换、碳排放报告生成
package carbonaware

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNotRunning 调度器未运行.
	ErrNotRunning = errors.New("carbon aware scheduler not running")
	// ErrAlreadyRunning 调度器已在运行.
	ErrAlreadyRunning = errors.New("carbon aware scheduler already running")
	// ErrTaskNotFound 任务不存在.
	ErrTaskNotFound = errors.New("task not found")
	// ErrReportNotFound 报告不存在.
	ErrReportNotFound = errors.New("report not found")
	// ErrInvalidConfig 配置无效.
	ErrInvalidConfig = errors.New("invalid configuration")
	// ErrWindowNotFound 窗口不存在.
	ErrWindowNotFound = errors.New("time window not found")
)

// ========== 碳强度等级 ==========

// CarbonIntensityLevel 碳强度等级.
type CarbonIntensityLevel string

const (
	// IntensityVeryLow 极低碳强度 (绿色能源高峰).
	IntensityVeryLow CarbonIntensityLevel = "very_low"
	// IntensityLow 低碳强度.
	IntensityLow CarbonIntensityLevel = "low"
	// IntensityMedium 中等碳强度.
	IntensityMedium CarbonIntensityLevel = "medium"
	// IntensityHigh 高碳强度.
	IntensityHigh CarbonIntensityLevel = "high"
	// IntensityVeryHigh 极高碳强度.
	IntensityVeryHigh CarbonIntensityLevel = "very_high"
)

// ========== 任务类型 ==========

// TaskType 任务类型.
type TaskType string

const (
	// TaskBackup 备份任务.
	TaskBackup TaskType = "backup"
	// TaskDedup 去重任务.
	TaskDedup TaskType = "dedup"
	// TaskTranscode 转码任务.
	TaskTranscode TaskType = "transcode"
	// TaskSync 同步任务.
	TaskSync TaskType = "sync"
	// TaskCompression 压缩任务.
	TaskCompression TaskType = "compression"
	// TaskGarbageCollection GC 任务.
	TaskGarbageCollection TaskType = "gc"
)

// ========== 任务优先级 ==========

// TaskPriority 任务优先级.
type TaskPriority string

const (
	// PriorityCritical 紧急 — 不等碳窗口，立即执行.
	PriorityCritical TaskPriority = "critical"
	// PriorityHigh 高优先级 — 等待低碳窗口，最多等1小时.
	PriorityHigh TaskPriority = "high"
	// PriorityNormal 正常 — 等待低碳窗口，最多等6小时.
	PriorityNormal TaskPriority = "normal"
	// PriorityLow 低优先级 — 等待最优绿色窗口，最多等24小时.
	PriorityLow TaskPriority = "low"
)

// ========== 节能模式 ==========

// PowerMode 节能模式.
type PowerMode string

const (
	// PowerModeNormal 正常模式.
	PowerModeNormal PowerMode = "normal"
	// PowerModeEco 节能模式 — 只在低碳时段执行非紧急任务.
	PowerModeEco PowerMode = "eco"
	// PowerModeGreen 极致绿能 — 仅在极低碳强度时段执行.
	PowerModeGreen PowerMode = "green"
	// PowerModeAggressive 积极模式 — 忽略碳强度.
	PowerModeAggressive PowerMode = "aggressive"
)

// ========== 报告周期 ==========

// ReportPeriod 报告周期.
type ReportPeriod string

const (
	// PeriodDaily 日报.
	PeriodDaily ReportPeriod = "daily"
	// PeriodWeekly 周报.
	PeriodWeekly ReportPeriod = "weekly"
	// PeriodMonthly 月报.
	PeriodMonthly ReportPeriod = "monthly"
)

// ========== 能源类型 ==========

// EnergySource 能源类型.
type EnergySource string

const (
	// SourceSolar 太阳能.
	SourceSolar EnergySource = "solar"
	// SourceWind 风能.
	SourceWind EnergySource = "wind"
	// SourceHydro 水电.
	SourceHydro EnergySource = "hydro"
	// SourceNuclear 核电.
	SourceNuclear EnergySource = "nuclear"
	// SourceCoal 煤电.
	SourceCoal EnergySource = "coal"
	// SourceGas 天然气.
	SourceGas EnergySource = "gas"
	// SourceUnknown 未知.
	SourceUnknown EnergySource = "unknown"
)

// ========== 核心配置 ==========

// Config 碳感知调度器配置.
type Config struct {
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// PowerMode 当前节能模式.
	PowerMode PowerMode `json:"powerMode"`
	// CarbonIntensityThreshold 碳强度阈值 (gCO2/kWh)，低于此值视为绿色时段.
	CarbonIntensityThreshold float64 `json:"carbonIntensityThreshold"`
	// VeryLowThreshold 极低碳强度阈值.
	VeryLowThreshold float64 `json:"veryLowThreshold"`
	// LowThreshold 低碳强度阈值.
	LowThreshold float64 `json:"lowThreshold"`
	// MediumThreshold 中碳强度阈值.
	MediumThreshold float64 `json:"mediumThreshold"`
	// HighThreshold 高碳强度阈值.
	HighThreshold float64 `json:"highThreshold"`
	// Region 电网区域 (如 CN-EAST, CN-NORTH).
	Region string `json:"region"`
	// CarbonAPIEndpoint 碳强度 API 端点.
	CarbonAPIEndpoint string `json:"carbonApiEndpoint"`
	// APIKey API 密钥.
	APIKey string `json:"apiKey"`
	// RefreshIntervalMinutes 数据刷新间隔（分钟）.
	RefreshIntervalMinutes int `json:"refreshIntervalMinutes"`
	// ForecastHoursAhead 预测提前小时数.
	ForecastHoursAhead int `json:"forecastHoursAhead"`
	// MaxConcurrentTasks 最大并发任务数.
	MaxConcurrentTasks int `json:"maxConcurrentTasks"`
	// TaskTimeoutHours 任务超时小时数.
	TaskTimeoutHours int `json:"taskTimeoutHours"`
	// EnableSmartPower 是否与 smartpower 模块联动.
	EnableSmartPower bool `json:"enableSmartPower"`
	// EnableSmartEnergy 是否与 smartenergy 模块联动.
	EnableSmartEnergy bool `json:"enableSmartEnergy"`
	// CO2PerKWh 默认碳排放因子 (gCO2/kWh).
	CO2PerKWh float64 `json:"co2PerKWh"`
	// AlertThreshold 告警阈值 — 日碳排放超过此值(gCO2)触发告警.
	AlertThreshold float64 `json:"alertThreshold"`
	// RetentionDays 记录保留天数.
	RetentionDays int `json:"retentionDays"`
}

// ========== 碳强度数据 ==========

// CarbonIntensity 碳强度数据点.
type CarbonIntensity struct {
	// Timestamp 时间戳.
	Timestamp time.Time `json:"timestamp"`
	// Intensity 碳强度 (gCO2/kWh).
	Intensity float64 `json:"intensity"`
	// Level 碳强度等级.
	Level CarbonIntensityLevel `json:"level"`
	// Source 主要能源来源.
	Source EnergySource `json:"source"`
	// RenewablePercent 可再生能源占比 (%).
	RenewablePercent float64 `json:"renewablePercent"`
	// Forecast 是否为预测值.
	Forecast bool `json:"forecast"`
	// Confidence 预测置信度 (0-1).
	Confidence float64 `json:"confidence"`
	// Region 区域.
	Region string `json:"region"`
}

// ========== 绿色能源时段 ==========

// GreenWindow 绿色能源时段窗口.
type GreenWindow struct {
	// ID 窗口ID.
	ID string `json:"id"`
	// StartTime 开始时间.
	StartTime time.Time `json:"startTime"`
	// EndTime 结束时间.
	EndTime time.Time `json:"endTime"`
	// AvgIntensity 平均碳强度.
	AvgIntensity float64 `json:"avgIntensity"`
	// Level 碳强度等级.
	Level CarbonIntensityLevel `json:"level"`
	// RenewablePercent 可再生能源占比.
	RenewablePercent float64 `json:"renewablePercent"`
	// Source 主要能源.
	Source EnergySource `json:"source"`
	// Score 绿色分数 (0-100, 越高越绿色).
	Score float64 `json:"score"`
	// Predicted 是否为预测窗口.
	Predicted bool `json:"predicted"`
}

// ========== 碳排放记录 ==========

// CarbonEmission 碳排放记录.
type CarbonEmission struct {
	// ID 记录ID.
	ID string `json:"id"`
	// TaskID 关联任务ID.
	TaskID string `json:"taskId"`
	// TaskType 任务类型.
	TaskType TaskType `json:"taskType"`
	// Timestamp 时间戳.
	Timestamp time.Time `json:"timestamp"`
	// EnergyUsedKWh 能耗 (kWh).
	EnergyUsedKWh float64 `json:"energyUsedKWh"`
	// CO2Emitted 排放量 (gCO2).
	CO2Emitted float64 `json:"co2Emitted"`
	// CO2Saved 节省量 (gCO2).
	CO2Saved float64 `json:"co2Saved"`
	// CarbonIntensityAtTime 执行时碳强度.
	CarbonIntensityAtTime float64 `json:"carbonIntensityAtTime"`
	// EnergySource 主要能源.
	EnergySource EnergySource `json:"energySource"`
	// GreenWindowUsed 是否使用了绿色窗口.
	GreenWindowUsed bool `json:"greenWindowUsed"`
	// Region 区域.
	Region string `json:"region"`
}

// ========== 碳排放统计 ==========

// CarbonStats 碳排放统计.
type CarbonStats struct {
	// Period 统计周期.
	Period ReportPeriod `json:"period"`
	// StartDate 开始日期.
	StartDate time.Time `json:"startDate"`
	// EndDate 结束日期.
	EndDate time.Time `json:"endDate"`
	// TotalEmissions 总排放 (gCO2).
	TotalEmissions float64 `json:"totalEmissions"`
	// TotalSaved 总节省 (gCO2).
	TotalSaved float64 `json:"totalSaved"`
	// TotalEnergyKWh 总能耗 (kWh).
	TotalEnergyKWh float64 `json:"totalEnergyKWh"`
	// TaskCount 任务数.
	TaskCount int `json:"taskCount"`
	// GreenTaskCount 使用绿色窗口的任务数.
	GreenTaskCount int `json:"greenTaskCount"`
	// AvgCarbonIntensity 平均碳强度.
	AvgCarbonIntensity float64 `json:"avgCarbonIntensity"`
	// EmissionsByType 按任务类型分组排放.
	EmissionsByType map[TaskType]float64 `json:"emissionsByType"`
	// EmissionsByHour 按小时分组排放.
	EmissionsByHour []HourlyEmission `json:"emissionsByHour"`
	// PeakEmissionHour 排放峰值小时.
	PeakEmissionHour int `json:"peakEmissionHour"`
	// LowestEmissionHour 排放最低小时.
	LowestEmissionHour int `json:"lowestEmissionHour"`
	// GreenEnergyPercent 使用绿色能源占比 (%).
	GreenEnergyPercent float64 `json:"greenEnergyPercent"`
	// ComparisonWithPrevious 与上期对比 (%).
	ComparisonWithPrevious float64 `json:"comparisonWithPrevious"`
}

// HourlyEmission 每小时排放.
type HourlyEmission struct {
	// Hour 小时 (0-23).
	Hour int `json:"hour"`
	// Emissions 排放量 (gCO2).
	Emissions float64 `json:"emissions"`
	// TaskCount 任务数.
	TaskCount int `json:"taskCount"`
	// AvgIntensity 平均碳强度.
	AvgIntensity float64 `json:"avgIntensity"`
}

// GreenWindowUsage 绿色窗口利用.
type GreenWindowUsage struct {
	// WindowID 窗口ID.
	WindowID string `json:"windowId"`
	// StartTime 开始时间.
	StartTime time.Time `json:"startTime"`
	// EndTime 结束时间.
	EndTime time.Time `json:"endTime"`
	// DurationMinutes 持续时长.
	DurationMinutes int `json:"durationMinutes"`
	// TasksScheduled 调度任务数.
	TasksScheduled int `json:"tasksScheduled"`
	// CO2Saved 节省碳排放.
	CO2Saved float64 `json:"co2Saved"`
	// UtilizationPercent 利用率 (%).
	UtilizationPercent float64 `json:"utilizationPercent"`
}

// Recommendation 优化建议.
type Recommendation struct {
	// ID 建议ID.
	ID string `json:"id"`
	// Title 标题.
	Title string `json:"title"`
	// Description 描述.
	Description string `json:"description"`
	// PotentialSaving 预估节省 (gCO2).
	PotentialSaving float64 `json:"potentialSaving"`
	// Priority 优先级 (1-5, 1最高).
	Priority int `json:"priority"`
	// Category 分类.
	Category string `json:"category"`
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	// Date 日期.
	Date time.Time `json:"date"`
	// Emissions 排放量.
	Emissions float64 `json:"emissions"`
	// Saved 节省量.
	Saved float64 `json:"saved"`
	// TaskCount 任务数.
	TaskCount int `json:"taskCount"`
}

// ========== 调度器状态 ==========

// SchedulerStatus 调度器状态.
type SchedulerStatus struct {
	// Running 是否运行中.
	Running bool `json:"running"`
	// PowerMode 当前节能模式.
	PowerMode PowerMode `json:"powerMode"`
	// CurrentIntensity 当前碳强度.
	CurrentIntensity *CarbonIntensity `json:"currentIntensity,omitempty"`
	// QueuedTasks 排队任务数.
	QueuedTasks int `json:"queuedTasks"`
	// WaitingTasks 等待窗口任务数.
	WaitingTasks int `json:"waitingTasks"`
	// RunningTasks 运行中任务数.
	RunningTasks int `json:"runningTasks"`
	// NextGreenWindow 下一个绿色窗口.
	NextGreenWindow *GreenWindow `json:"nextGreenWindow,omitempty"`
	// TodayEmissions 今日排放.
	TodayEmissions float64 `json:"todayEmissions"`
	// TodaySaved 今日节省.
	TodaySaved float64 `json:"todaySaved"`
	// GreenTaskPercent 绿色任务占比 (%).
	GreenTaskPercent float64 `json:"greenTaskPercent"`
	// LastRefresh 最后刷新时间.
	LastRefresh time.Time `json:"lastRefresh"`
	// UpcomingWindows 未来绿色窗口.
	UpcomingWindows []GreenWindow `json:"upcomingWindows"`
}

// ========== 仪表板统计 ==========

// DashboardStats 碳感知仪表板统计.
type DashboardStats struct {
	// Status 调度器状态.
	Status SchedulerStatus `json:"status"`
	// DailyStats 今日统计.
	DailyStats *CarbonStats `json:"dailyStats,omitempty"`
	// WeeklyStats 本周统计.
	WeeklyStats *CarbonStats `json:"weeklyStats,omitempty"`
	// MonthlyStats 本月统计.
	MonthlyStats *CarbonStats `json:"monthlyStats,omitempty"`
	// TotalTasksProcessed 总处理任务数.
	TotalTasksProcessed int `json:"totalTasksProcessed"`
	// TotalCO2SavedTotal 历史总节省.
	TotalCO2SavedTotal float64 `json:"totalCO2SavedTotal"`
	// BestGreenDay 最佳绿色日.
	BestGreenDay *TrendPoint `json:"bestGreenDay,omitempty"`
	// AvgGreenPercent 平均绿色任务占比.
	AvgGreenPercent float64 `json:"avgGreenPercent"`
}
