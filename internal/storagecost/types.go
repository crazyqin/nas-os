// Package storagecost - 存储成本分析模块
// TCO分析、容量预测、成本优化建议、存储效率报告、预算规划、多维对比
package storagecost

import (
	"time"
)

// ============================================================
// TCO (总拥有成本) 类型
// ============================================================

// StorageAsset 存储资产信息
type StorageAsset struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"` // "hdd", "ssd", "nvme", "tape", "cloud"
	CapacityTB      float64   `json:"capacity_tb"`
	PurchaseDate    time.Time `json:"purchase_date"`
	PurchaseCost    float64   `json:"purchase_cost"`    // 采购成本 (元)
	WarrantyYears   int       `json:"warranty_years"`    // 保修年限
	AnnualPowerKWh  float64   `json:"annual_power_kwh"` // 年耗电量 (kWh)
	PowerCostPerKWh float64   `json:"power_cost_per_kwh"` // 电费单价 (元/kWh)
	RackUnits       int       `json:"rack_units"`        // 占用机架单位
	RackCostPerUnit float64   `json:"rack_cost_per_unit"` // 机架单位年租金 (元)
}

// TCOConfig TCO计算配置
type TCOConfig struct {
	AnalysisPeriodYears  int     `json:"analysis_period_years"`  // 分析周期 (年), 默认 5
	DiscountRate         float64 `json:"discount_rate"`           // 折现率, 默认 0.05
	MaintenanceRate      float64 `json:"maintenance_rate"`        // 年维护费率 (占采购成本比例), 默认 0.10
	DepreciationMethod   string  `json:"depreciation_method"`     // 折旧方法: "straight_line", "declining_balance"
	IncludeOpportunityCost bool   `json:"include_opportunity_cost"` // 是否包含机会成本
}

// DefaultTCOConfig 默认TCO配置
func DefaultTCOConfig() TCOConfig {
	return TCOConfig{
		AnalysisPeriodYears:    5,
		DiscountRate:           0.05,
		MaintenanceRate:        0.10,
		DepreciationMethod:     "straight_line",
		IncludeOpportunityCost: false,
	}
}

// TCOResult TCO分析结果
type TCOResult struct {
	AssetID          string             `json:"asset_id"`
	AssetName        string             `json:"asset_name"`
	AnalysisPeriod   int                `json:"analysis_period"` // 年
	TotalCost        float64            `json:"total_cost"`      // 总成本 (元)
	CostPerTB        float64            `json:"cost_per_tb"`     // 每TB成本 (元/年)
	CostBreakdown    CostBreakdown      `json:"cost_breakdown"`
	AnnualCosts      []AnnualCost       `json:"annual_costs"`    // 年度成本明细
	NPV              float64            `json:"npv"`             // 净现值
	CalculatedAt     time.Time          `json:"calculated_at"`
}

// CostBreakdown 成本构成明细
type BreakdownItem struct {
	Category    string  `json:"category"`    // "hardware", "power", "cooling", "rack", "maintenance", "labor"
	Amount      float64 `json:"amount"`      // 金额 (元)
	Percentage  float64 `json:"percentage"`  // 占比 (%)
}

type CostBreakdown struct {
	Hardware    float64 `json:"hardware"`
	Power       float64 `json:"power"`
	Cooling     float64 `json:"cooling"`
	Rack        float64 `json:"rack"`
	Maintenance float64 `json:"maintenance"`
	Labor       float64 `json:"labor"`
	Total       float64 `json:"total"`
}

// AnnualCost 年度成本
type AnnualCost struct {
	Year        int     `json:"year"`
	Hardware    float64 `json:"hardware"`     // 含折旧
	Power       float64 `json:"power"`
	Cooling     float64 `json:"cooling"`
	Rack        float64 `json:"rack"`
	Maintenance float64 `json:"maintenance"`
	Labor       float64 `json:"labor"`
	Total       float64 `json:"total"`
}

// ============================================================
// 容量预测类型
// ============================================================

// CapacitySample 容量采样点
type CapacitySample struct {
	Timestamp     time.Time `json:"timestamp"`
	UsedTB        float64   `json:"used_tb"`        // 已使用 (TB)
	TotalTB       float64   `json:"total_tb"`       // 总容量 (TB)
	Utilization   float64   `json:"utilization"`    // 利用率 (%)
}

// GrowthTrend 增长趋势
type GrowthTrend struct {
	Period           string  `json:"period"`            // "daily", "weekly", "monthly"
	GrowthRate       float64 `json:"growth_rate"`       // 增长率 (%)
	GrowthTB         float64 `json:"growth_tb"`         // 增长量 (TB)
	AvgDailyGrowthGB float64 `json:"avg_daily_growth_gb"` // 平均日增长 (GB)
}

// CapacityForecast 容量预测结果
type CapacityForecast struct {
	CurrentUsedTB      float64          `json:"current_used_tb"`
	CurrentTotalTB     float64          `json:"current_total_tb"`
	CurrentUtilization float64          `json:"current_utilization"`
	GrowthTrend        GrowthTrend      `json:"growth_trend"`
	Forecasts          []ForecastPoint  `json:"forecasts"`           // 未来预测点
	RunwayDays         int              `json:"runway_days"`         // 容量耗尽天数
	RunwayDate         *time.Time       `json:"runway_date"`         // 容量耗尽日期
	Confidence         float64          `json:"confidence"`          // 预测置信度 (%)
	Recommendation     string           `json:"recommendation"`      // 建议
	CalculatedAt       time.Time        `json:"calculated_at"`
}

// ForecastPoint 预测数据点
type ForecastPoint struct {
	Date           time.Time `json:"date"`
	ProjectedUsedTB float64  `json:"projected_used_tb"`
	Utilization    float64   `json:"utilization"`
	Confidence     float64   `json:"confidence"` // 置信区间 (%)
}

// ForecastConfig 预测配置
type ForecastConfig struct {
	ForecastMonths   int     `json:"forecast_months"`    // 预测月数, 默认 12
	ModelType        string  `json:"model_type"`         // "linear", "exponential", "moving_average"
	MovingAvgWindow  int     `json:"moving_avg_window"`  // 移动平均窗口 (天), 默认 30
	AlertThreshold   float64 `json:"alert_threshold"`    // 告警阈值 (%), 默认 80
}

// DefaultForecastConfig 默认预测配置
func DefaultForecastConfig() ForecastConfig {
	return ForecastConfig{
		ForecastMonths:  12,
		ModelType:       "linear",
		MovingAvgWindow: 30,
		AlertThreshold:  80,
	}
}

// ============================================================
// 成本优化建议类型
// ============================================================

// DataTier 数据分层
type DataTier struct {
	Name        string  `json:"name"`        // "hot", "warm", "cold", "archive"
	StorageType string  `json:"storage_type"` // "nvme", "ssd", "hdd", "tape", "cloud"
	CapacityTB  float64 `json:"capacity_tb"`
	UsedTB      float64 `json:"used_tb"`
	CostPerTB   float64 `json:"cost_per_tb"`  // 每TB年成本 (元)
	AccessFreq  string  `json:"access_freq"`  // "high", "medium", "low", "rare"
}

// TieringSuggestion 分层建议
type TieringSuggestion struct {
	SourceTier       string  `json:"source_tier"`
	TargetTier       string  `json:"target_tier"`
	EligibleDataTB   float64 `json:"eligible_data_tb"`   // 可迁移数据量 (TB)
	AnnualSaving     float64 `json:"annual_saving"`      // 年节省 (元)
	SavingPercent    float64 `json:"saving_percent"`     // 节省比例 (%)
	Confidence       float64 `json:"confidence"`         // 置信度 (%)
	Rationale        string  `json:"rationale"`          // 建议理由
}

// DeduplicationBenefit 去重收益
type DeduplicationBenefit struct {
	TotalDataTB       float64 `json:"total_data_tb"`
	DedupRatio        float64 `json:"dedup_ratio"`        // 去重比
	SpaceSavedTB      float64 `json:"space_saved_tb"`     // 节省空间 (TB)
	CostSavedPerYear  float64 `json:"cost_saved_per_year"` // 年节省成本 (元)
	DedupEnabled      bool    `json:"dedup_enabled"`
}

// CompressionBenefit 压缩收益
type CompressionBenefit struct {
	TotalDataTB       float64 `json:"total_data_tb"`
	CompressionRatio  float64 `json:"compression_ratio"` // 压缩比
	SpaceSavedTB      float64 `json:"space_saved_tb"`
	CostSavedPerYear  float64 `json:"cost_saved_per_year"`
	CompressionEnabled bool   `json:"compression_enabled"`
}

// OptimizationReport 优化建议报告
type OptimizationReport struct {
	TieringSuggestions   []TieringSuggestion   `json:"tiering_suggestions"`
	Deduplication        DeduplicationBenefit   `json:"deduplication"`
	Compression          CompressionBenefit     `json:"compression"`
	TotalAnnualSaving    float64               `json:"total_annual_saving"`
	TopRecommendations   []Recommendation       `json:"top_recommendations"`
	GeneratedAt          time.Time             `json:"generated_at"`
}

// Recommendation 优化建议
type Recommendation struct {
	Priority    string  `json:"priority"`    // "high", "medium", "low"
	Category    string  `json:"category"`    // "tiering", "dedup", "compression", "retirement"
	Title       string  `json:"title"`
	Description string  `json:"description"`
	SavingEst   float64 `json:"saving_est"`  // 预估节省 (元/年)
	Effort      string  `json:"effort"`      // "low", "medium", "high"
}

// ============================================================
// 存储效率报告类型
// ============================================================

// EfficiencyMetrics 效率指标
type EfficiencyMetrics struct {
	CompressionRatio  float64 `json:"compression_ratio"`  // 压缩比
	DeduplicationRatio float64 `json:"deduplication_ratio"` // 去重比
	ThinProvisionRatio float64 `json:"thin_provision_ratio"` // 精简配置比
	SpaceUtilization   float64 `json:"space_utilization"`   // 空间利用率 (%)
	RawCapacityTB      float64 `json:"raw_capacity_tb"`     // 原始容量 (TB)
	UsableCapacityTB   float64 `json:"usable_capacity_tb"`  // 可用容量 (TB)
	EffectiveCapacityTB float64 `json:"effective_capacity_tb"` // 有效容量 (含压缩去重后)
	OverheadPercent    float64 `json:"overhead_percent"`    // 开销占比 (%)
}

// EfficiencyReport 效率报告
type EfficiencyReport struct {
	Overall         EfficiencyMetrics            `json:"overall"`
	ByStoragePool   map[string]EfficiencyMetrics `json:"by_storage_pool"`
	ByDataType      map[string]EfficiencyMetrics `json:"by_data_type"` // "vm", "database", "file", "backup"
	Trend           []EfficiencyTrend            `json:"trend"`        // 效率趋势
	Benchmarks      EfficiencyBenchmark          `json:"benchmarks"`   // 行业基准对比
	GeneratedAt     time.Time                    `json:"generated_at"`
}

// EfficiencyTrend 效率趋势
type EfficiencyTrend struct {
	Date              time.Time `json:"date"`
	CompressionRatio  float64   `json:"compression_ratio"`
	DedupRatio        float64   `json:"dedup_ratio"`
	Utilization       float64   `json:"utilization"`
}

// EfficiencyBenchmark 效率基准
type EfficiencyBenchmark struct {
	IndustryAvgCompression float64 `json:"industry_avg_compression"`
	IndustryAvgDedup       float64 `json:"industry_avg_dedup"`
	IndustryAvgUtilization float64 `json:"industry_avg_utilization"`
	YourRank               string  `json:"your_rank"` // "top_quartile", "above_average", "average", "below_average"
}

// ============================================================
// 预算规划类型
// ============================================================

// BudgetPlan 预算计划
type BudgetPlan struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	FiscalYear      int             `json:"fiscal_year"`
	TotalBudget     float64         `json:"total_budget"`      // 总预算 (元)
	AllocatedBudget float64         `json:"allocated_budget"`  // 已分配预算
	RemainingBudget float64         `json:"remaining_budget"`  // 剩余预算
	LineItems       []BudgetLineItem `json:"line_items"`
	Status          string          `json:"status"` // "draft", "approved", "executing", "completed"
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// BudgetLineItem 预算项目
type BudgetLineItem struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"`    // "expansion", "replacement", "upgrade", "maintenance"
	Description string    `json:"description"`
	Amount      float64   `json:"amount"`      // 预算金额 (元)
	Quantity    int       `json:"quantity"`
	UnitCost    float64   `json:"unit_cost"`
	PlannedDate time.Time `json:"planned_date"` // 计划采购日期
	Status      string    `json:"status"`       // "pending", "approved", "ordered", "delivered"
	Priority    string    `json:"priority"`     // "critical", "high", "medium", "low"
}

// CostTrend 成本趋势
type CostTrend struct {
	Period        string  `json:"period"`         // "2024-Q1", "2024-Q2", etc.
	HardwareCost  float64 `json:"hardware_cost"`
	OperatingCost float64 `json:"operating_cost"`
	CloudCost     float64 `json:"cloud_cost"`
	TotalCost     float64 `json:"total_cost"`
	CapacityTB    float64 `json:"capacity_tb"`
	CostPerTB     float64 `json:"cost_per_tb"`
}

// ProcurementRecommendation 采购建议
type ProcurementRecommendation struct {
	AssetType     string    `json:"asset_type"`     // "hdd", "ssd", "nvme"
	CapacityTB    float64   `json:"capacity_tb"`
	Quantity      int       `json:"quantity"`
	EstimatedCost float64   `json:"estimated_cost"`
	Reason        string    `json:"reason"`
	Urgency       string    `json:"urgency"`        // "immediate", "next_quarter", "next_year"
	Justification string    `json:"justification"`
}

// ============================================================
// 多维对比类型
// ============================================================

// ComparisonDimension 对比维度
type ComparisonDimension string

const (
	DimCostPerTB      ComparisonDimension = "cost_per_tb"
	DimPerformance    ComparisonDimension = "performance"
	DimReliability    ComparisonDimension = "reliability"
	DimScalability    ComparisonDimension = "scalability"
	DimEfficiency     ComparisonDimension = "efficiency"
	DimTCO            ComparisonDimension = "tco"
)

// StorageOption 存储方案选项
type StorageOption struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Type            string             `json:"type"` // "on_premise", "hybrid", "cloud"
	CapacityTB      float64            `json:"capacity_tb"`
	CostPerTBYear   float64            `json:"cost_per_tb_year"`  // 年成本/TB (元)
	TCO5Year        float64            `json:"tco_5_year"`        // 5年TCO
	IOPSCapability  int                `json:"iops_capability"`
	ThroughputMBps  int                `json:"throughput_mbps"`
	Availability    float64            `json:"availability"`       // 可用性 (%)
	ScalabilityScore float64           `json:"scalability_score"`  // 0-100
	Features        []string           `json:"features"`
	Scores          map[string]float64 `json:"scores"`             // 各维度得分
}

// ComparisonResult 对比结果
type ComparisonResult struct {
	Options       []StorageOption    `json:"options"`
	Dimensions    []ComparisonDimension `json:"dimensions"`
	Scores        map[string]map[string]float64 `json:"scores"` // [option_id][dimension]score
	Rankings      []OptionRanking    `json:"rankings"`
	BestOption    *OptionRanking     `json:"best_option"`
	GeneratedAt   time.Time          `json:"generated_at"`
}

// OptionRanking 方案排名
type OptionRanking struct {
	OptionID      string  `json:"option_id"`
	OptionName    string  `json:"option_name"`
	OverallScore  float64 `json:"overall_score"` // 综合得分 (0-100)
	Rank          int     `json:"rank"`
	Strengths     []string `json:"strengths"`
	Weaknesses    []string `json:"weaknesses"`
}

// ============================================================
// 通用类型
// ============================================================

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// StoragePool 存储池信息
type StoragePool struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	RaidLevel     string  `json:"raid_level"`
	TotalTB       float64 `json:"total_tb"`
	UsedTB        float64 `json:"used_tb"`
	AvailableTB   float64 `json:"available_tb"`
	Utilization   float64 `json:"utilization"`
	DiskCount     int     `json:"disk_count"`
	DiskType      string  `json:"disk_type"`
}

// APIError API错误响应
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}
