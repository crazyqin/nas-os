// Package smartcostoptimizer 提供智能存储成本优化功能，支持成本计算、趋势分析、
// 优化建议生成、ROI 计算及报告导出，参考群晖 DSM 7.3 存储效率分析。
package smartcostoptimizer

import (
	"time"
)

// StorageType 存储介质类型.
type StorageType string

// 存储类型常量定义.
const (
	// StorageTypeSSD 固态硬盘.
	StorageTypeSSD StorageType = "ssd"
	// StorageTypeHDD 机械硬盘.
	StorageTypeHDD StorageType = "hdd"
	// StorageTypeNVMe NVMe 高速盘.
	StorageTypeNVMe StorageType = "nvme"
	// StorageTypeTape 磁带归档.
	StorageTypeTape StorageType = "tape"
	// StorageTypeCloud 云存储.
	StorageTypeCloud StorageType = "cloud"
	// StorageTypeUnknown 未知类型.
	StorageTypeUnknown StorageType = "unknown"
)

// DataTemperature 数据温度（访问频率）.
type DataTemperature string

// 数据温度常量定义.
const (
	// TempHot 热数据，频繁访问.
	TempHot DataTemperature = "hot"
	// TempWarm 温数据，偶尔访问.
	TempWarm DataTemperature = "warm"
	// TempCold 冷数据，很少访问.
	TempCold DataTemperature = "cold"
	// TempFrozen 冻结数据，几乎不访问.
	TempFrozen DataTemperature = "frozen"
)

// OptimizationStrategy 优化策略.
type OptimizationStrategy string

// 优化策略常量定义.
const (
	// StrategyColdMigration 冷数据迁移.
	StrategyColdMigration OptimizationStrategy = "cold_migration"
	// StrategyDeduplication 去重.
	StrategyDeduplication OptimizationStrategy = "deduplication"
	// StrategyCompression 压缩.
	StrategyCompression OptimizationStrategy = "compression"
	// StrategyTiering 自动分层.
	StrategyTiering OptimizationStrategy = "tiering"
	// StrategyCleanup 清理过期数据.
	StrategyCleanup OptimizationStrategy = "cleanup"
	// StrategyArchivePolicy 归档策略.
	StrategyArchivePolicy OptimizationStrategy = "archive_policy"
)

// ExportFormat 导出格式.
type ExportFormat string

// 导出格式常量定义.
const (
	// ExportJSON JSON 格式.
	ExportJSON ExportFormat = "json"
	// ExportCSV CSV 格式.
	ExportCSV ExportFormat = "csv"
)

// TrendGranularity 趋势粒度.
type TrendGranularity string

// 趋势粒度常量定义.
const (
	// TrendDaily 日粒度.
	TrendDaily TrendGranularity = "daily"
	// TrendWeekly 周粒度.
	TrendWeekly TrendGranularity = "weekly"
	// TrendMonthly 月粒度.
	TrendMonthly TrendGranularity = "monthly"
	// TrendYearly 年粒度.
	TrendYearly TrendGranularity = "yearly"
)

// ============================================================
// 存储资源与定价
// ============================================================

// StorageAsset 存储资产（设备/卷）
type StorageAsset struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Type          StorageType `json:"type"`
	CapacityBytes int64       `json:"capacity_bytes"`
	UsedBytes     int64       `json:"used_bytes"`
	PurchaseCost  float64     `json:"purchase_cost"` // 购置成本
	MonthlyOpex   float64     `json:"monthly_opex"`  // 月运营成本
	WarrantyYears int         `json:"warranty_years"`
	PurchaseDate  time.Time   `json:"purchase_date"`
	Pool          string      `json:"pool"`
	Volume        string      `json:"volume,omitempty"`
	Provider      string      `json:"provider,omitempty"` // 本地/阿里云/AWS 等
	Labels        []string    `json:"labels,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
}

// PricingRule 定价规则
type PricingRule struct {
	StorageType     StorageType `json:"storage_type"`
	PricePerGBMonth float64     `json:"price_per_gb_month"` // 元/GB/月
	TransferPerGB   float64     `json:"transfer_per_gb"`    // 元/GB 传输费
	RetrievalPerGB  float64     `json:"retrieval_per_gb"`   // 元/GB 取回费
	RequestPer1K    float64     `json:"request_per_1k"`     // 元/千次请求
}

// SmartCostConfig 智能成本优化器配置
type SmartCostConfig struct {
	Enabled            bool                        `json:"enabled"`
	DefaultCurrency    string                      `json:"default_currency"`
	PricingRules       map[StorageType]PricingRule `json:"pricing_rules"`
	ColdThresholdDays  int                         `json:"cold_threshold_days"`  // 未访问天数判定冷数据
	UtilizationWarnPct float64                     `json:"utilization_warn_pct"` // 利用率低于此值告警
	DedupRatio         float64                     `json:"dedup_ratio"`          // 预估去重率
	CompressRatio      float64                     `json:"compress_ratio"`       // 预估压缩率
	ReportRetention    int                         `json:"report_retention_days"`
}

// DefaultSmartCostConfig 返回默认配置
func DefaultSmartCostConfig() *SmartCostConfig {
	return &SmartCostConfig{
		Enabled:         true,
		DefaultCurrency: "CNY",
		PricingRules: map[StorageType]PricingRule{
			StorageTypeNVMe:  {StorageTypeNVMe, 0.80, 0.15, 0.20, 0.01},
			StorageTypeSSD:   {StorageTypeSSD, 0.50, 0.12, 0.15, 0.01},
			StorageTypeHDD:   {StorageTypeHDD, 0.20, 0.08, 0.10, 0.005},
			StorageTypeTape:  {StorageTypeTape, 0.05, 0.03, 0.30, 0.002},
			StorageTypeCloud: {StorageTypeCloud, 0.15, 0.10, 0.25, 0.005},
		},
		ColdThresholdDays:  90,
		UtilizationWarnPct: 20.0,
		DedupRatio:         0.30,
		CompressRatio:      0.40,
		ReportRetention:    365,
	}
}

// ============================================================
// 成本记录与汇总
// ============================================================

// CostEntry 单条成本记录
type CostEntry struct {
	ID          string      `json:"id"`
	AssetID     string      `json:"asset_id"`
	AssetName   string      `json:"asset_name"`
	StorageType StorageType `json:"storage_type"`
	CapacityGB  float64     `json:"capacity_gb"`
	UsedGB      float64     `json:"used_gb"`
	PricePerGB  float64     `json:"price_per_gb_month"`
	TotalCost   float64     `json:"total_cost"` // 本期总成本
	PeriodStart time.Time   `json:"period_start"`
	PeriodEnd   time.Time   `json:"period_end"`
	RecordedAt  time.Time   `json:"recorded_at"`
}

// CostSummary 成本汇总
type CostSummary struct {
	TotalCost       float64                 `json:"total_cost"`
	TotalCapacityGB float64                 `json:"total_capacity_gb"`
	TotalUsedGB     float64                 `json:"total_used_gb"`
	AvgUtilization  float64                 `json:"avg_utilization_pct"`
	ByType          map[StorageType]float64 `json:"by_type"`
	ByPool          map[string]float64      `json:"by_pool"`
	Currency        string                  `json:"currency"`
	PeriodStart     time.Time               `json:"period_start"`
	PeriodEnd       time.Time               `json:"period_end"`
}

// ============================================================
// 成本趋势
// ============================================================

// TrendPoint 趋势数据点
type TrendPoint struct {
	Date   time.Time `json:"date"`
	Cost   float64   `json:"cost"`
	UsedGB float64   `json:"used_gb"`
	FreeGB float64   `json:"free_gb"`
}

// CostTrend 成本趋势分析结果
type CostTrend struct {
	Granularity   TrendGranularity `json:"granularity"`
	Points        []TrendPoint     `json:"points"`
	GrowthRate    float64          `json:"growth_rate"`    // 成本增长率
	ProjectedNext float64          `json:"projected_next"` // 下期预测成本
	PeriodStart   time.Time        `json:"period_start"`
	PeriodEnd     time.Time        `json:"period_end"`
}

// ============================================================
// 优化建议
// ============================================================

// OptimizationSuggestion 智能优化建议
type OptimizationSuggestion struct {
	ID              string               `json:"id"`
	Strategy        OptimizationStrategy `json:"strategy"`
	Title           string               `json:"title"`
	Description     string               `json:"description"`
	EstimatedSaving float64              `json:"estimated_saving"`
	SavingsPercent  float64              `json:"savings_percent"`
	Currency        string               `json:"currency"`
	Priority        int                  `json:"priority"` // 1 最高
	TargetAssets    []string             `json:"target_assets"`
	CurrentType     StorageType          `json:"current_type,omitempty"`
	RecommendedType StorageType          `json:"recommended_type,omitempty"`
	Complexity      string               `json:"complexity"` // low/medium/high
	RiskLevel       string               `json:"risk_level"` // low/medium/high
	Details         string               `json:"details,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
}

// ============================================================
// ROI 计算
// ============================================================

// ROIInput 投资回报率计算输入
type ROIInput struct {
	InvestmentCost float64 `json:"investment_cost"` // 投资成本
	AnnualSaving   float64 `json:"annual_saving"`   // 年节省
	AnnualOpex     float64 `json:"annual_opex"`     // 年运维成本
	ProjectYears   int     `json:"project_years"`   // 项目年限
	DiscountRate   float64 `json:"discount_rate"`   // 折现率（如 0.08 = 8%）
}

// ROIResult 投资回报率计算结果
type ROIResult struct {
	InvestmentCost  float64     `json:"investment_cost"`
	TotalSaving     float64     `json:"total_saving"`
	TotalOpex       float64     `json:"total_opex"`
	NetProfit       float64     `json:"net_profit"`
	ROIPercent      float64     `json:"roi_percent"`
	PaybackMonths   float64     `json:"payback_months"`
	NPV             float64     `json:"npv"` // 净现值
	IRR             float64     `json:"irr"` // 内部收益率
	AnnualBreakdown []AnnualROI `json:"annual_breakdown"`
}

// AnnualROI 年度 ROI 明细
type AnnualROI struct {
	Year         int     `json:"year"`
	Saving       float64 `json:"saving"`
	Opex         float64 `json:"opex"`
	NetCashFlow  float64 `json:"net_cash_flow"`
	CumulativeCF float64 `json:"cumulative_cf"`
	DiscountedCF float64 `json:"discounted_cf"`
}

// ============================================================
// 冷数据检测
// ============================================================

// ColdDataInfo 冷数据信息
type ColdDataInfo struct {
	AssetID       string          `json:"asset_id"`
	AssetName     string          `json:"asset_name"`
	Volume        string          `json:"volume"`
	Directory     string          `json:"directory,omitempty"`
	SizeBytes     int64           `json:"size_bytes"`
	LastAccess    time.Time       `json:"last_access"`
	DaysSince     int             `json:"days_since_access"`
	CurrentType   StorageType     `json:"current_type"`
	Temperature   DataTemperature `json:"temperature"`
	SuggestedType StorageType     `json:"suggested_type"`
	PotentialSave float64         `json:"potential_save"`
}

// ============================================================
// 报告导出
// ============================================================

// CostReport 智能成本报告
type CostReport struct {
	ID          string                    `json:"id"`
	ReportName  string                    `json:"report_name"`
	Summary     *CostSummary              `json:"summary"`
	Trend       *CostTrend                `json:"trend,omitempty"`
	Suggestions []*OptimizationSuggestion `json:"suggestions,omitempty"`
	ColdData    []*ColdDataInfo           `json:"cold_data,omitempty"`
	ROI         *ROIResult                `json:"roi,omitempty"`
	GeneratedAt time.Time                 `json:"generated_at"`
	Format      ExportFormat              `json:"format"`
}
