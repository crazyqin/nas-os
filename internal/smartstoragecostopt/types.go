// Package smartstoragecostopt 提供智能存储成本分析引擎
// 对标群晖DSM 7.3的存储效率分析功能
// 支持成本计算、优化建议、ROI分析、趋势预测、多维报表和预算告警
package smartstoragecostopt

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrInvalidInput 无效输入.
	ErrInvalidInput = errors.New("无效输入")
	// ErrNoData 无数据.
	ErrNoData = errors.New("无数据")
	// ErrBudgetExceeded 预算超限.
	ErrBudgetExceeded = errors.New("预算超限")
	// ErrInsufficientHistory 历史数据不足.
	ErrInsufficientHistory = errors.New("历史数据不足")
	// ErrInvalidTier 无效存储层级.
	ErrInvalidTier = errors.New("无效存储层级")
)

// ========== 存储层级定义 ==========

// StorageTier 存储层级.
type StorageTier string

const (
	// TierNVMe NVMe SSD 层级.
	TierNVMe StorageTier = "nvme"
	// TierSSD SATA SSD 层级.
	TierSSD StorageTier = "ssd"
	// TierHDD HDD 层级.
	TierHDD StorageTier = "hdd"
	// TierCloud 云存储层级.
	TierCloud StorageTier = "cloud"
	// TierTape 磁带归档层级.
	TierTape StorageTier = "tape"
)

// TierProfile 存储层级画像.
type TierProfile struct {
	Tier           StorageTier `json:"tier"`
	Name           string      `json:"name"`
	CostPerTBMonth float64     `json:"cost_per_tb_month"` // 元/TB/月
	ReadSpeedMBps  int         `json:"read_speed_mbps"`
	WriteSpeedMBps int         `json:"write_speed_mbps"`
	LatencyMs      float64     `json:"latency_ms"`
	Reliability    string      `json:"reliability"`
	IOPIOPS        int         `json:"iops"`          // IOPS
	BandwidthMBps  int         `json:"bandwidth_mbps"` // 最大带宽
}

// DefaultTierProfiles 默认存储层级画像.
var DefaultTierProfiles = map[StorageTier]TierProfile{
	TierNVMe: {
		Tier:           TierNVMe,
		Name:           "NVMe SSD",
		CostPerTBMonth: 500.0,
		ReadSpeedMBps:  3500,
		WriteSpeedMBps: 3000,
		LatencyMs:      0.1,
		Reliability:    "99.999%",
		IOPIOPS:        1000000,
		BandwidthMBps:  7000,
	},
	TierSSD: {
		Tier:           TierSSD,
		Name:           "SATA SSD",
		CostPerTBMonth: 300.0,
		ReadSpeedMBps:  550,
		WriteSpeedMBps: 520,
		LatencyMs:      0.5,
		Reliability:    "99.999%",
		IOPIOPS:        100000,
		BandwidthMBps:  600,
	},
	TierHDD: {
		Tier:           TierHDD,
		Name:           "HDD",
		CostPerTBMonth: 100.0,
		ReadSpeedMBps:  200,
		WriteSpeedMBps: 180,
		LatencyMs:      5.0,
		Reliability:    "99.99%",
		IOPIOPS:        200,
		BandwidthMBps:  250,
	},
	TierCloud: {
		Tier:           TierCloud,
		Name:           "云存储",
		CostPerTBMonth: 50.0,
		ReadSpeedMBps:  100,
		WriteSpeedMBps: 50,
		LatencyMs:      50.0,
		Reliability:    "99.999999999%",
		IOPIOPS:        5000,
		BandwidthMBps:  200,
	},
	TierTape: {
		Tier:           TierTape,
		Name:           "磁带归档",
		CostPerTBMonth: 10.0,
		ReadSpeedMBps:  300,
		WriteSpeedMBps: 200,
		LatencyMs:      60000.0, // 检索延迟高
		Reliability:    "99.999999999%",
		IOPIOPS:        10,
		BandwidthMBps:  300,
	},
}

// ========== 访问模式定义 ==========

// AccessPattern 访问模式.
type AccessPattern string

const (
	// AccessHot 热数据（频繁访问）.
	AccessHot AccessPattern = "hot"
	// AccessWarm 温数据（偶尔访问）.
	AccessWarm AccessPattern = "warm"
	// AccessCold 冷数据（很少访问）.
	AccessCold AccessPattern = "cold"
	// AccessFrozen 冻结数据（几乎不访问）.
	AccessFrozen AccessPattern = "frozen"
)

// ========== 成本计算类型 ==========

// CostRecord 成本记录.
type CostRecord struct {
	ID          string      `json:"id"`
	DeptID      string      `json:"dept_id"`     // 部门ID
	ProjectID   string      `json:"project_id"`  // 项目ID
	UserID      string      `json:"user_id"`     // 用户ID
	Path        string      `json:"path"`
	Tier        StorageTier `json:"tier"`
	UsedBytes   int64       `json:"used_bytes"`
	SizeBytes   int64       `json:"size_bytes"`
	AccessCount int64       `json:"access_count"` // 月访问次数
	Timestamp   time.Time   `json:"timestamp"`
	MonthlyCost float64     `json:"monthly_cost"` // 当月成本（元）
}

// CostBreakdown 成本明细.
type CostBreakdown struct {
	TotalCost    float64                  `json:"total_cost"`
	ByTier       map[StorageTier]float64  `json:"by_tier"`
	ByDept       map[string]float64       `json:"by_dept"`
	ByProject    map[string]float64       `json:"by_project"`
	ByUser       map[string]float64       `json:"by_user"`
	ByAccess     map[AccessPattern]float64 `json:"by_access_pattern"`
	TierDetails  []TierCostDetail         `json:"tier_details"`
	AnalyzedAt   time.Time                `json:"analyzed_at"`
}

// TierCostDetail 层级成本明细.
type TierCostDetail struct {
	Tier          StorageTier `json:"tier"`
	Name          string      `json:"name"`
	TotalBytes    int64       `json:"total_bytes"`
	UsedBytes     int64       `json:"used_bytes"`
	UsagePercent  float64     `json:"usage_percent"`
	MonthlyCost   float64     `json:"monthly_cost"`
	CostPerTB     float64     `json:"cost_per_tb"`
	Records       int         `json:"records"`
}

// ========== 优化建议类型 ==========

// OptimizationRecommendation 优化建议.
type OptimizationRecommendation struct {
	ID              string      `json:"id"`
	Type            string      `json:"type"`     // migrate, compress, archive, tier_down
	Priority        string      `json:"priority"` // critical, high, medium, low
	Title           string      `json:"title"`
	Description     string      `json:"description"`
	SourcePath      string      `json:"source_path"`
	SourceTier      StorageTier `json:"source_tier"`
	TargetTier      StorageTier `json:"target_tier,omitempty"`
	CurrentCost     float64     `json:"current_cost"`
	OptimizedCost   float64     `json:"optimized_cost"`
	SavingsPerMonth float64     `json:"savings_per_month"`
	SavingsPercent  float64     `json:"savings_percent"`
	AccessPattern   AccessPattern `json:"access_pattern"`
	Reason          string      `json:"reason"`
	Confidence      float64     `json:"confidence"` // 0-1 置信度
}

// ========== ROI分析类型 ==========

// ROIAnalysis ROI分析结果.
type ROIAnalysis struct {
	TotalInvestment    float64           `json:"total_investment"`     // 总投资（元）
	AnnualSavings      float64           `json:"annual_savings"`       // 年节省（元）
	MonthlySavings     float64           `json:"monthly_savings"`      // 月节省（元）
	PaybackMonths      float64           `json:"payback_months"`       // 回本周期（月）
	ROI3Year           float64           `json:"roi_3_year"`           // 3年ROI（%）
	ROI5Year           float64           `json:"roi_5_year"`           // 5年ROI（%）
	TotalSavings3Year  float64           `json:"total_savings_3_year"` // 3年总节省
	TotalSavings5Year  float64           `json:"total_savings_5_year"` // 5年总节省
	BreakEvenDate      time.Time         `json:"break_even_date"`      // 回本日期
	CurrentCost        float64           `json:"current_cost"`         // 当前月度成本
	ProjectedCost      float64           `json:"projected_cost"`       // 优化后月度成本
	InvestmentItems    []InvestmentItem  `json:"investment_items"`     // 投资项目明细
	AnalyzedAt         time.Time         `json:"analyzed_at"`
}

// InvestmentItem 投资项.
type InvestmentItem struct {
	Name         string  `json:"name"`
	Cost         float64 `json:"cost"`
	MonthlySaving float64 `json:"monthly_saving"`
	PaybackMonths float64 `json:"payback_months"`
}

// ========== 趋势预测类型 ==========

// CostTrend 成本趋势.
type CostTrend struct {
	Month          string       `json:"month"`           // YYYY-MM
	Cost           float64      `json:"cost"`
	UsedTB         float64      `json:"used_tb"`
	CostPerTB      float64      `json:"cost_per_tb"`
	GrowthRate     float64      `json:"growth_rate"`     // 环比增长率
	ProjectedCost  *float64     `json:"projected_cost,omitempty"` // 预测成本
}

// CostForecast 成本预测.
type CostForecast struct {
	History        []CostTrend  `json:"history"`
	Projections    []CostTrend  `json:"projections"`
	MonthlyGrowthRate float64   `json:"monthly_growth_rate"` // 平均月增长率
	CurrentCost    float64      `json:"current_cost"`
	ProjectedCost6M float64     `json:"projected_cost_6m"`   // 6个月后预测
	ProjectedCost1Y float64     `json:"projected_cost_1y"`   // 1年后预测
	TrendDirection string       `json:"trend_direction"`     // rising/stable/declining
	Confidence     float64      `json:"confidence"`          // 预测置信度
	ForecastAt     time.Time    `json:"forecast_at"`
}

// ========== 多维度报表类型 ==========

// DepartmentReport 部门报表.
type DepartmentReport struct {
	DeptID         string            `json:"dept_id"`
	DeptName       string            `json:"dept_name"`
	TotalCost      float64           `json:"total_cost"`
	TotalUsedTB    float64           `json:"total_used_tb"`
	CostPerTB      float64           `json:"cost_per_tb"`
	ByTier         map[StorageTier]float64 `json:"by_tier"`
	ByProject      map[string]float64      `json:"by_project"`
	UserCount      int               `json:"user_count"`
	GrowthRate     float64           `json:"growth_rate"`
	RankByCost     int               `json:"rank_by_cost"`
}

// ProjectReport 项目报表.
type ProjectReport struct {
	ProjectID   string            `json:"project_id"`
	ProjectName string            `json:"project_name"`
	DeptID      string            `json:"dept_id"`
	TotalCost   float64           `json:"total_cost"`
	TotalUsedTB float64           `json:"total_used_tb"`
	ByTier      map[StorageTier]float64 `json:"by_tier"`
	UserCount   int               `json:"user_count"`
}

// UserReport 用户报表.
type UserReport struct {
	UserID         string            `json:"user_id"`
	Username       string            `json:"username"`
	DeptID         string            `json:"dept_id"`
	ProjectID      string            `json:"project_id"`
	TotalCost      float64           `json:"total_cost"`
	TotalUsedTB    float64           `json:"total_used_tb"`
	ByTier         map[StorageTier]float64 `json:"by_tier"`
	AccessPattern  AccessPattern     `json:"access_pattern"`
	TopPaths       []UserTopPath     `json:"top_paths"`
}

// UserTopPath 用户高频访问路径.
type UserTopPath struct {
	Path        string  `json:"path"`
	UsedBytes   int64   `json:"used_bytes"`
	Cost        float64 `json:"cost"`
	AccessCount int64   `json:"access_count"`
}

// MultiDimReport 多维度报表.
type MultiDimReport struct {
	ReportID      string             `json:"report_id"`
	GeneratedAt   time.Time          `json:"generated_at"`
	Summary       ReportSummary      `json:"summary"`
	Departments   []DepartmentReport `json:"departments"`
	Projects      []ProjectReport    `json:"projects"`
	Users         []UserReport       `json:"users"`
}

// ReportSummary 报表摘要.
type ReportSummary struct {
	TotalCost       float64 `json:"total_cost"`
	TotalUsedTB     float64 `json:"total_used_tb"`
	TotalRecords    int     `json:"total_records"`
	AvgCostPerTB    float64 `json:"avg_cost_per_tb"`
	TopDept         string  `json:"top_dept"`
	TopDeptCost     float64 `json:"top_dept_cost"`
	MonthlyGrowth   float64 `json:"monthly_growth"`
}

// ========== 预算告警类型 ==========

// BudgetConfig 预算配置.
type BudgetConfig struct {
	DeptID           string    `json:"dept_id"`
	ProjectID        string    `json:"project_id"`
	UserID           string    `json:"user_id"`
	MonthlyBudget    float64   `json:"monthly_budget"`    // 月预算（元）
	AlertThreshold   float64   `json:"alert_threshold"`   // 告警阈值（0-1，如0.8=80%）
	NotifyEmail      string    `json:"notify_email"`
	NotifyWebhook    string    `json:"notify_webhook"`
	Enabled          bool      `json:"enabled"`
}

// BudgetStatus 预算状态.
type BudgetStatus struct {
	BudgetID         string    `json:"budget_id"`
	DeptID           string    `json:"dept_id"`
	ProjectID        string    `json:"project_id"`
	UserID           string    `json:"user_id"`
	MonthlyBudget    float64   `json:"monthly_budget"`
	CurrentSpend     float64   `json:"current_spend"`
	UsagePercent     float64   `json:"usage_percent"`
	Remaining        float64   `json:"remaining"`
	Status           string    `json:"status"` // normal, warning, critical, exceeded
	AlertTriggered   bool      `json:"alert_triggered"`
	LastAlertAt      *time.Time `json:"last_alert_at,omitempty"`
	AnalyzedAt       time.Time `json:"analyzed_at"`
}

// BudgetAlert 预算告警.
type BudgetAlert struct {
	AlertID      string    `json:"alert_id"`
	BudgetID     string    `json:"budget_id"`
	Level        string    `json:"level"` // warning, critical, exceeded
	Title        string    `json:"title"`
	Message      string    `json:"message"`
	CurrentSpend float64   `json:"current_spend"`
	Budget       float64   `json:"budget"`
	UsagePercent float64   `json:"usage_percent"`
	TriggeredAt  time.Time `json:"triggered_at"`
	Notified     bool      `json:"notified"`
}

// ========== 引擎配置 ==========

// EngineConfig 引擎配置.
type EngineConfig struct {
	// DefaultTierProfiles 自定义层级画像.
	DefaultTierProfiles map[StorageTier]TierProfile
	// IdleThresholdDays 闲置阈值天数.
	IdleThresholdDays int
	// ColdAccessThreshold 冷数据月访问次数阈值.
	ColdAccessThreshold int64
	// FrozenAccessThreshold 冻结数据月访问次数阈值.
	FrozenAccessThreshold int64
	// GrowthForecastMonths 预测月份数.
	GrowthForecastMonths int
	// DefaultAlertThreshold 默认告警阈值.
	DefaultAlertThreshold float64
}

// DefaultEngineConfig 返回默认引擎配置.
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		IdleThresholdDays:     90,
		ColdAccessThreshold:   10,
		FrozenAccessThreshold: 1,
		GrowthForecastMonths:  12,
		DefaultAlertThreshold: 0.8,
	}
}

// ========== 辅助方法 ==========

// GetAccessPattern 根据访问次数判断访问模式.
func (c *EngineConfig) GetAccessPattern(accessCount int64) AccessPattern {
	switch {
	case accessCount > 100:
		return AccessHot
	case accessCount > 10:
		return AccessWarm
	case accessCount > 1:
		return AccessCold
	default:
		return AccessFrozen
	}
}
