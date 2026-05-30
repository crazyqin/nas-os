// Package smartstoragecost - 智能存储成本分析器
// 多层级存储成本计算、历史趋势分析、成本优化建议、云存储对比、成本预测
package smartstoragecost

import (
	"time"
)

// ============================================================
// 存储层级类型
// ============================================================

// StorageTierType 存储介质类型
type StorageTierType string

const (
	// TierHDD 机械硬盘
	TierHDD StorageTierType = "hdd"
	// TierSSD SATA固态硬盘
	TierSSD StorageTierType = "ssd"
	// TierNVMe NVMe固态硬盘
	TierNVMe StorageTierType = "nvme"
	// TierCloud 云存储
	TierCloud StorageTierType = "cloud"
)

// StorageTier 存储层级配置
type StorageTier struct {
	Type             StorageTierType `json:"type"`
	Name             string          `json:"name"`
	CostPerTBMonth   float64         `json:"cost_per_tb_month"`   // 每TB月成本 (元)
	IOPSPerTB        int             `json:"iops_per_tb"`         // 每TB IOPS
	ThroughputMBpsTB int             `json:"throughput_mbps_tb"`  // 每TB吞吐 (MB/s)
	LatencyMs        float64         `json:"latency_ms"`          // 平均延迟 (ms)
	Durability       string          `json:"durability"`          // 耐久性指标 "99.999999999%"
	AvailSLA         float64         `json:"avail_sla"`           // 可用性 SLA (%)
	MinCapacityTB    float64         `json:"min_capacity_tb"`     // 最小容量
	MaxCapacityTB    float64         `json:"max_capacity_tb"`     // 最大容量
}

// ============================================================
// 成本记录
// ============================================================

// CostRecord 成本记录
type CostRecord struct {
	ID           string          `json:"id"`
	Timestamp    time.Time       `json:"timestamp"`
	TierType     StorageTierType `json:"tier_type"`
	CapacityTB   float64         `json:"capacity_tb"`
	UsedTB       float64         `json:"used_tb"`
	CostPerTB    float64         `json:"cost_per_tb"`     // 每TB月成本 (元)
	TotalCost    float64         `json:"total_cost"`       // 本月总成本 (元)
	Category     string          `json:"category"`         // "hardware", "power", "bandwidth", "subscription"
	Provider     string          `json:"provider"`         // "local", "aws", "azure", "aliyun", etc.
	Region       string          `json:"region"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ============================================================
// 成本报告
// ============================================================

// CostReport 成本报告
type CostReport struct {
	ReportID       string              `json:"report_id"`
	GeneratedAt    time.Time           `json:"generated_at"`
	Period         ReportPeriod        `json:"period"`
	Summary        CostSummary         `json:"summary"`
	TierBreakdown  []TierCostDetail    `json:"tier_breakdown"`
	TrendData      []TrendPoint        `json:"trend_data"`
	TopCostDrivers []CostDriver        `json:"top_cost_drivers"`
	YoYChange      *YearOverYearChange `json:"yoy_change,omitempty"`
}

// ReportPeriod 报告时间范围
type ReportPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Label string    `json:"label"` // "2024-Q1", "2024-05", etc.
}

// CostSummary 成本摘要
type CostSummary struct {
	TotalMonthlyCost  float64 `json:"total_monthly_cost"`   // 本月总成本 (元)
	TotalCapacityTB   float64 `json:"total_capacity_tb"`    // 总容量 (TB)
	TotalUsedTB       float64 `json:"total_used_tb"`        // 已用容量 (TB)
	AvgCostPerTB      float64 `json:"avg_cost_per_tb"`      // 平均每TB月成本 (元)
	Utilization       float64 `json:"utilization"`          // 利用率 (%)
	WastedCost        float64 `json:"wasted_cost"`          // 闲置成本 (元)
	CostChangePercent float64 `json:"cost_change_percent"`  // 环比变化 (%)
}

// TierCostDetail 单层级成本详情
type TierCostDetail struct {
	TierType     StorageTierType `json:"tier_type"`
	TierName     string          `json:"tier_name"`
	CapacityTB   float64         `json:"capacity_tb"`
	UsedTB       float64         `json:"used_tb"`
	Utilization  float64         `json:"utilization"`
	CostPerTB    float64         `json:"cost_per_tb"`
	MonthlyCost  float64         `json:"monthly_cost"`
	SharePercent float64         `json:"share_percent"` // 占总成本比例 (%)
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Date        time.Time `json:"date"`
	TotalCost   float64   `json:"total_cost"`
	CapacityTB  float64   `json:"capacity_tb"`
	UsedTB      float64   `json:"used_tb"`
	CostPerTB   float64   `json:"cost_per_tb"`
	Utilization float64   `json:"utilization"`
}

// CostDriver 成本驱动因素
type CostDriver struct {
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Percentage  float64 `json:"percentage"`
	Trend       string  `json:"trend"` // "increasing", "stable", "decreasing"
}

// YearOverYearChange 同比变化
type YearOverYearChange struct {
	CurrentYearCost  float64 `json:"current_year_cost"`
	PreviousYearCost float64 `json:"previous_year_cost"`
	ChangePercent    float64 `json:"change_percent"`
	AbsChange        float64 `json:"abs_change"`
}

// ============================================================
// 成本预测
// ============================================================

// Forecast 成本预测结果
type Forecast struct {
	GeneratedAt     time.Time         `json:"generated_at"`
	HorizonMonths   int               `json:"horizon_months"`
	CurrentCost     float64           `json:"current_monthly_cost"`
	ProjectedCosts  []ForecastPoint   `json:"projected_costs"`
	GrowthModel     string            `json:"growth_model"` // "linear", "exponential", "seasonal"
	GrowthRate      float64           `json:"growth_rate"`  // 月增长率 (%)
	RSquared        float64           `json:"r_squared"`    // 拟合优度
	ConfidenceLevel float64           `json:"confidence_level"`
	Scenarios       []ForecastScenario `json:"scenarios"`
}

// ForecastPoint 预测数据点
type ForecastPoint struct {
	Month           time.Time `json:"month"`
	ProjectedCost   float64   `json:"projected_cost"`
	LowerBound      float64   `json:"lower_bound"` // 置信下界
	UpperBound      float64   `json:"upper_bound"` // 置信上界
	ProjectedTB     float64   `json:"projected_tb"`
}

// ForecastScenario 预测场景
type ForecastScenario struct {
	Name            string  `json:"name"`             // "optimistic", "baseline", "pessimistic"
	Description     string  `json:"description"`
	GrowthRate      float64 `json:"growth_rate"`      // 月增长率 (%)
	TotalCost12Mo   float64 `json:"total_cost_12mo"`  // 12个月总成本
	AvgMonthlyCost  float64 `json:"avg_monthly_cost"` // 月均成本
}

// ============================================================
// 优化建议
// ============================================================

// Optimization 优化建议
type Optimization struct {
	GeneratedAt       time.Time              `json:"generated_at"`
	TotalSaving       float64                `json:"total_annual_saving"` // 年节省总额 (元)
	SavingPercent     float64                `json:"saving_percent"`      // 节省比例 (%)
	Suggestions       []OptimizationSuggestion `json:"suggestions"`
	QuickWins         []QuickWin             `json:"quick_wins"`
	StrategicMoves    []StrategicMove        `json:"strategic_moves"`
	RiskAssessment    RiskAssessment         `json:"risk_assessment"`
}

// OptimizationSuggestion 优化建议项
type OptimizationSuggestion struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Category    string  `json:"category"`    // "tiering", "rightsizing", "compression", "dedup", "cloud_migration"
	Priority    string  `json:"priority"`    // "critical", "high", "medium", "low"
	Impact      string  `json:"impact"`      // "high", "medium", "low"
	Effort      string  `json:"effort"`      // "low", "medium", "high"
	SavingEst   float64 `json:"saving_est"`  // 预估年节省 (元)
	Description string  `json:"description"`
	Rationale   string  `json:"rationale"`
	Steps       []string `json:"steps,omitempty"`
}

// QuickWin 快速优化项（低投入高回报）
type QuickWin struct {
	Title        string  `json:"title"`
	SavingEst    float64 `json:"saving_est"`
	DaysToImplement int  `json:"days_to_implement"`
	Description  string  `json:"description"`
}

// StrategicMove 战略优化项（长期规划）
type StrategicMove struct {
	Title         string  `json:"title"`
	SavingEst     float64 `json:"saving_est"`
	MonthsToROI   int     `json:"months_to_roi"`
	CAPEXRequired float64 `json:"capex_required"`
	Description   string  `json:"description"`
}

// RiskAssessment 风险评估
type RiskAssessment struct {
	OverallRisk   string   `json:"overall_risk"` // "low", "medium", "high"
	RiskFactors   []string `json:"risk_factors"`
	Mitigations   []string `json:"mitigations"`
}

// ============================================================
// 多方案对比
// ============================================================

// CompareRequest 多方案对比请求
type CompareRequest struct {
	Scenarios     []CompareScenario `json:"scenarios" binding:"required,min=2"`
	PeriodMonths  int               `json:"period_months"`  // 对比周期 (月), 默认 36
	CapacityTB    float64           `json:"capacity_tb"`    // 目标容量 (TB)
	GrowthRate    float64           `json:"growth_rate"`    // 月增长率 (%)
}

// CompareScenario 对比方案
type CompareScenario struct {
	Name           string  `json:"name"`
	TierType       StorageTierType `json:"tier_type"`
	InitialCost    float64 `json:"initial_cost"`     // 初始投入 (元)
	MonthlyPerTB   float64 `json:"monthly_per_tb"`   // 每TB月成本 (元)
	GrowthCap      float64 `json:"growth_cap"`       // 扩容成本系数
	IncludeCloud   bool    `json:"include_cloud"`    // 是否含云备份
}

// CompareResult 对比结果
type CompareResult struct {
	GeneratedAt    time.Time         `json:"generated_at"`
	PeriodMonths   int               `json:"period_months"`
	CapacityTB     float64           `json:"capacity_tb"`
	Results        []ScenarioResult  `json:"results"`
	BestOption     string            `json:"best_option"`
	BestSavings    float64           `json:"best_savings"`
	Analysis       string            `json:"analysis"`
}

// ScenarioResult 单方案结果
type ScenarioResult struct {
	Name           string  `json:"name"`
	TierType       StorageTierType `json:"tier_type"`
	TotalCost      float64 `json:"total_cost"`       // 周期总成本
	MonthlyCost    float64 `json:"monthly_cost"`     // 月均成本
	CostPerTB      float64 `json:"cost_per_tb"`      // 每TB月成本
	FinalCapacity  float64 `json:"final_capacity"`   // 最终容量
	TotalSavings   float64 `json:"total_savings"`    // 相比最贵方案节省
	SavingsPercent float64 `json:"savings_percent"`
	Rank           int     `json:"rank"`
}

// ============================================================
// 通用类型
// ============================================================

// APIError API错误响应
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}
