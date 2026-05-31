// Package storagecostanalyzer 存储成本分析器
// 分析不同存储层级的成本（SSD/HDD/冷存储/云存储）
// 提供每TB成本计算、容量趋势预测、成本优化建议、月度/年度报告
// 与 datatiering/smarttier 模块协作
package storagecostanalyzer

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNotRunning 分析器未运行.
	ErrNotRunning = errors.New("storage cost analyzer not running")
	// ErrAlreadyRunning 分析器已在运行.
	ErrAlreadyRunning = errors.New("storage cost analyzer already running")
	// ErrReportNotFound 报告不存在.
	ErrReportNotFound = errors.New("report not found")
	// ErrTierNotFound 存储层级不存在.
	ErrTierNotFound = errors.New("storage tier not found")
	// ErrInvalidConfig 配置无效.
	ErrInvalidConfig = errors.New("invalid configuration")
	// ErrInsufficientData 数据不足，无法预测.
	ErrInsufficientData = errors.New("insufficient data for prediction")
)

// ========== 存储层级类型 ==========

// StorageTier 存储层级类型.
type StorageTier string

const (
	// TierSSD SSD固态硬盘.
	TierSSD StorageTier = "ssd"
	// TierHDD HDD机械硬盘.
	TierHDD StorageTier = "hdd"
	// TierCold 冷存储（归档）.
	TierCold StorageTier = "cold"
	// TierCloud 云存储.
	TierCloud StorageTier = "cloud"
)

// ========== 成本类别 ==========

// CostCategory 成本类别.
type CostCategory string

const (
	// CategoryHardware 硬件成本.
	CategoryHardware CostCategory = "hardware"
	// CategoryPower 电力成本.
	CategoryPower CostCategory = "power"
	// CategoryCooling 散热成本.
	CategoryCooling CostCategory = "cooling"
	// CategoryMaintenance 维护成本.
	CategoryMaintenance CostCategory = "maintenance"
	// CategorySubscription 订阅费用.
	CategorySubscription CostCategory = "subscription"
	// CategoryBandwidth 带宽成本.
	CategoryBandwidth CostCategory = "bandwidth"
)

// ========== 优化建议优先级 ==========

// Priority 优化建议优先级.
type Priority string

const (
	// PriorityCritical 紧急.
	PriorityCritical Priority = "critical"
	// PriorityHigh 高.
	PriorityHigh Priority = "high"
	// PriorityMedium 中.
	PriorityMedium Priority = "medium"
	// PriorityLow 低.
	PriorityLow Priority = "low"
)

// ========== 核心配置 ==========

// Config 存储成本分析器配置.
type Config struct {
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// Currency 货币单位（CNY/USD）.
	Currency string `json:"currency"`
	// ReportRetentionDays 报告保留天数.
	ReportRetentionDays int `json:"reportRetentionDays"`
	// ForecastMonths 预测月数.
	ForecastMonths int `json:"forecastMonths"`
	// AlertThreshold 利用率告警阈值（%）.
	AlertThreshold float64 `json:"alertThreshold"`
	// AutoAnalyze 是否自动分析.
	AutoAnalyze bool `json:"autoAnalyze"`
	// AnalyzeIntervalHours 分析间隔（小时）.
	AnalyzeIntervalHours int `json:"analyzeIntervalHours"`
}

// ========== 存储层级配置 ==========

// TierConfig 存储层级配置.
type TierConfig struct {
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// Name 层级名称.
	Name string `json:"name"`
	// CostPerTBMonth 每TB每月成本.
	CostPerTBMonth float64 `json:"costPerTBMonth"`
	// CapacityTB 总容量（TB）.
	CapacityTB float64 `json:"capacityTB"`
	// UsedTB 已用容量（TB）.
	UsedTB float64 `json:"usedTB"`
	// ReadIOPS 读IOPS.
	ReadIOPS int `json:"readIOPS"`
	// WriteIOPS 写IOPS.
	WriteIOPS int `json:"writeIOPS"`
	// ThroughputMBps 吞吐量（MB/s）.
	ThroughputMBps float64 `json:"throughputMBps"`
	// LatencyMs 延迟（ms）.
	LatencyMs float64 `json:"latencyMs"`
	// Durability 耐久性（如 "99.999999999%"）.
	Durability string `json:"durability"`
	// AvailabilitySLA 可用性SLA（%）.
	AvailabilitySLA float64 `json:"availabilitySLA"`
}

// ========== 成本记录 ==========

// CostRecord 成本记录.
type CostRecord struct {
	// ID 记录ID.
	ID string `json:"id"`
	// Timestamp 时间戳.
	Timestamp time.Time `json:"timestamp"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// Category 成本类别.
	Category CostCategory `json:"category"`
	// Amount 金额.
	Amount float64 `json:"amount"`
	// CapacityTB 容量（TB）.
	CapacityTB float64 `json:"capacityTB"`
	// CostPerTB 每TB成本.
	CostPerTB float64 `json:"costPerTB"`
	// Description 描述.
	Description string `json:"description"`
	// Provider 供应商（local/aws/azure/aliyun）.
	Provider string `json:"provider"`
}

// ========== 容量使用快照 ==========

// CapacitySnapshot 容量使用快照.
type CapacitySnapshot struct {
	// Timestamp 时间戳.
	Timestamp time.Time `json:"timestamp"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// CapacityTB 总容量（TB）.
	CapacityTB float64 `json:"capacityTB"`
	// UsedTB 已用容量（TB）.
	UsedTB float64 `json:"usedTB"`
	// Utilization 利用率（%）.
	Utilization float64 `json:"utilization"`
}

// ========== 成本分析报告 ==========

// CostReport 成本分析报告.
type CostReport struct {
	// ID 报告ID.
	ID string `json:"id"`
	// Title 标题.
	Title string `json:"title"`
	// ReportType 报告类型（monthly/yearly/quarterly）.
	ReportType string `json:"reportType"`
	// PeriodStart 周期开始.
	PeriodStart time.Time `json:"periodStart"`
	// PeriodEnd 周期结束.
	PeriodEnd time.Time `json:"periodEnd"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// TotalCost 总成本.
	TotalCost float64 `json:"totalCost"`
	// TotalCapacityTB 总容量（TB）.
	TotalCapacityTB float64 `json:"totalCapacityTB"`
	// TotalUsedTB 总已用（TB）.
	TotalUsedTB float64 `json:"totalUsedTB"`
	// AvgCostPerTB 平均每TB成本.
	AvgCostPerTB float64 `json:"avgCostPerTB"`
	// OverallUtilization 总体利用率.
	OverallUtilization float64 `json:"overallUtilization"`
	// WastedCost 闲置浪费成本.
	WastedCost float64 `json:"wastedCost"`
	// TierBreakdown 各层级成本明细.
	TierBreakdown []TierCostBreakdown `json:"tierBreakdown"`
	// CostTrend 成本趋势.
	CostTrend []TrendPoint `json:"costTrend"`
	// CostChangePercent 成本环比变化（%）.
	CostChangePercent float64 `json:"costChangePercent"`
	// TopCostDrivers 主要成本驱动因素.
	TopCostDrivers []CostDriver `json:"topCostDrivers"`
	// OptimizationSavings 优化可节省金额.
	OptimizationSavings float64 `json:"optimizationSavings"`
}

// TierCostBreakdown 层级成本明细.
type TierCostBreakdown struct {
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// TierName 层级名称.
	TierName string `json:"tierName"`
	// CapacityTB 容量（TB）.
	CapacityTB float64 `json:"capacityTB"`
	// UsedTB 已用（TB）.
	UsedTB float64 `json:"usedTB"`
	// Utilization 利用率（%）.
	Utilization float64 `json:"utilization"`
	// CostPerTB 每TB成本.
	CostPerTB float64 `json:"costPerTB"`
	// MonthlyCost 月成本.
	MonthlyCost float64 `json:"monthlyCost"`
	// CostShare 成本占比（%）.
	CostShare float64 `json:"costShare"`
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	// Date 日期.
	Date time.Time `json:"date"`
	// TotalCost 总成本.
	TotalCost float64 `json:"totalCost"`
	// CapacityTB 容量（TB）.
	CapacityTB float64 `json:"capacityTB"`
	// UsedTB 已用（TB）.
	UsedTB float64 `json:"usedTB"`
	// CostPerTB 每TB成本.
	CostPerTB float64 `json:"costPerTB"`
}

// CostDriver 成本驱动因素.
type CostDriver struct {
	// Category 类别.
	Category CostCategory `json:"category"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// Amount 金额.
	Amount float64 `json:"amount"`
	// Percentage 占比（%）.
	Percentage float64 `json:"percentage"`
	// Trend 趋势（increasing/stable/decreasing）.
	Trend string `json:"trend"`
	// Description 描述.
	Description string `json:"description"`
}

// ========== 容量预测 ==========

// CapacityForecast 容量预测.
type CapacityForecast struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// CurrentUsedTB 当前已用（TB）.
	CurrentUsedTB float64 `json:"currentUsedTB"`
	// CurrentCapacityTB 当前容量（TB）.
	CurrentCapacityTB float64 `json:"currentCapacityTB"`
	// GrowthRateMBPerDay 日增长量（MB/天）.
	GrowthRateMBPerDay float64 `json:"growthRateMBPerDay"`
	// GrowthModel 增长模型（linear/exponential）.
	GrowthModel string `json:"growthModel"`
	// DaysUntilFull 预计满容量天数.
	DaysUntilFull int `json:"daysUntilFull"`
	// FullDate 预计满容量日期.
	FullDate *time.Time `json:"fullDate,omitempty"`
	// ProjectedUsage 预测使用情况.
	ProjectedUsage []ProjectedPoint `json:"projectedUsage"`
	// ExpansionNeeded 是否需要扩容.
	ExpansionNeeded bool `json:"expansionNeeded"`
	// RecommendedExpansionTB 建议扩容容量（TB）.
	RecommendedExpansionTB float64 `json:"recommendedExpansionTB"`
}

// ProjectedPoint 预测数据点.
type ProjectedPoint struct {
	// Date 日期.
	Date time.Time `json:"date"`
	// ProjectedUsedTB 预测已用（TB）.
	ProjectedUsedTB float64 `json:"projectedUsedTB"`
	// ProjectedUtilization 预测利用率（%）.
	ProjectedUtilization float64 `json:"projectedUtilization"`
	// LowerBound 下界.
	LowerBound float64 `json:"lowerBound"`
	// UpperBound 上界.
	UpperBound float64 `json:"upperBound"`
}

// ========== 成本优化建议 ==========

// OptimizationPlan 成本优化方案.
type OptimizationPlan struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// TotalPotentialSavings 年度总潜在节省.
	TotalPotentialSavings float64 `json:"totalPotentialSavings"`
	// SavingPercent 节省比例（%）.
	SavingPercent float64 `json:"savingPercent"`
	// Suggestions 优化建议列表.
	Suggestions []OptimizationSuggestion `json:"suggestions"`
}

// OptimizationSuggestion 优化建议.
type OptimizationSuggestion struct {
	// ID 建议ID.
	ID string `json:"id"`
	// Title 标题.
	Title string `json:"title"`
	// Category 类别（tiering/rightsizing/compression/dedup/migration）.
	Category string `json:"category"`
	// Priority 优先级.
	Priority Priority `json:"priority"`
	// SourceTier 源存储层级.
	SourceTier StorageTier `json:"sourceTier"`
	// TargetTier 目标存储层级.
	TargetTier StorageTier `json:"targetTier"`
	// AffectedTB 受影响容量（TB）.
	AffectedTB float64 `json:"affectedTB"`
	// CurrentCost 当前成本.
	CurrentCost float64 `json:"currentCost"`
	// OptimizedCost 优化后成本.
	OptimizedCost float64 `json:"optimizedCost"`
	// AnnualSavings 年度节省.
	AnnualSavings float64 `json:"annualSavings"`
	// Description 描述.
	Description string `json:"description"`
	// Rationale 理由.
	Rationale string `json:"rationale"`
	// Steps 操作步骤.
	Steps []string `json:"steps"`
	// Impact 影响（high/medium/low）.
	Impact string `json:"impact"`
	// Effort 工作量（low/medium/high）.
	Effort string `json:"effort"`
}

// ========== 仪表板统计 ==========

// DashboardStats 仪表板统计.
type DashboardStats struct {
	// TotalMonthlyCost 总月成本.
	TotalMonthlyCost float64 `json:"totalMonthlyCost"`
	// TotalCapacityTB 总容量（TB）.
	TotalCapacityTB float64 `json:"totalCapacityTB"`
	// TotalUsedTB 总已用（TB）.
	TotalUsedTB float64 `json:"totalUsedTB"`
	// OverallUtilization 总体利用率（%）.
	OverallUtilization float64 `json:"overallUtilization"`
	// AvgCostPerTB 平均每TB成本.
	AvgCostPerTB float64 `json:"avgCostPerTB"`
	// TierCount 层级数.
	TierCount int `json:"tierCount"`
	// MonthlyReports 本月报告数.
	MonthlyReports int `json:"monthlyReports"`
	// PendingOptimizations 待执行优化数.
	PendingOptimizations int `json:"pendingOptimizations"`
	// PotentialAnnualSavings 年度潜在节省.
	PotentialAnnualSavings float64 `json:"potentialAnnualSavings"`
	// CostChangePercent 成本环比变化（%）.
	CostChangePercent float64 `json:"costChangePercent"`
	// LastAnalyzeTime 最后分析时间.
	LastAnalyzeTime time.Time `json:"lastAnalyzeTime"`
	// NextAnalyzeTime 下次分析时间.
	NextAnalyzeTime time.Time `json:"nextAnalyzeTime"`
	// TierStats 各层级统计.
	TierStats []TierStatsSummary `json:"tierStats"`
	// Alerts 告警列表.
	Alerts []CostAlert `json:"alerts"`
}

// TierStatsSummary 层级统计摘要.
type TierStatsSummary struct {
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// Name 名称.
	Name string `json:"name"`
	// CapacityTB 容量（TB）.
	CapacityTB float64 `json:"capacityTB"`
	// UsedTB 已用（TB）.
	UsedTB float64 `json:"usedTB"`
	// Utilization 利用率（%）.
	Utilization float64 `json:"utilization"`
	// MonthlyCost 月成本.
	MonthlyCost float64 `json:"monthlyCost"`
	// CostPerTB 每TB成本.
	CostPerTB float64 `json:"costPerTB"`
}

// CostAlert 成本告警.
type CostAlert struct {
	// ID 告警ID.
	ID string `json:"id"`
	// Level 级别（warning/critical）.
	Level string `json:"level"`
	// Tier 相关层级.
	Tier StorageTier `json:"tier"`
	// Message 消息.
	Message string `json:"message"`
	// Value 当前值.
	Value float64 `json:"value"`
	// Threshold 阈值.
	Threshold float64 `json:"threshold"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
}
