// Package storagecost 提供存储成本分析功能，支持按池/卷/目录分析存储成本、预测未来支出、优化建议。
package storagecost

import "time"

// StorageTier 存储层级
type StorageTier string

const (
	TierHot     StorageTier = "hot"     // 热存储 - SSD/NVMe
	TierWarm    StorageTier = "warm"    // 温存储 - HDD
	TierCold    StorageTier = "cold"    // 冷存储 - 归档
	TierGlacier StorageTier = "glacier" // 冰川存储 - 深度归档
)

// CostCategory 成本类别
type CostCategory string

const (
	CategoryStorage   CostCategory = "storage"   // 存储成本
	CategoryTransfer  CostCategory = "transfer"  // 传输成本
	CategoryRequest   CostCategory = "request"   // 请求成本
	CategoryRetrieval CostCategory = "retrieval" // 取回成本
)

// OptimizationType 优化类型
type OptimizationType string

const (
	OptTierMigration   OptimizationType = "tier_migration"   // 层级迁移
	OptDeduplication   OptimizationType = "deduplication"    // 去重
	OptCompression     OptimizationType = "compression"      // 压缩
	OptCleanup         OptimizationType = "cleanup"          // 清理
	OptLifecyclePolicy OptimizationType = "lifecycle_policy" // 生命周期策略
)

// CostBreakdown 成本明细
type CostBreakdown struct {
	ID          string       `json:"id"`
	Pool        string       `json:"pool"`
	Volume      string       `json:"volume,omitempty"`
	Directory   string       `json:"directory,omitempty"`
	Tier        StorageTier  `json:"tier"`
	Category    CostCategory `json:"category"`
	SizeBytes   int64        `json:"size_bytes"`
	CostPerGB   float64      `json:"cost_per_gb"`
	TotalCost   float64      `json:"total_cost"`
	Currency    string       `json:"currency"`
	PeriodStart time.Time    `json:"period_start"`
	PeriodEnd   time.Time    `json:"period_end"`
	CreatedAt   time.Time    `json:"created_at"`
}

// CostReport 成本报告
type CostReport struct {
	ID          string                   `json:"id"`
	ReportName  string                   `json:"report_name"`
	TotalCost   float64                  `json:"total_cost"`
	Currency    string                   `json:"currency"`
	Breakdowns  []*CostBreakdown         `json:"breakdowns"`
	ByTier      map[StorageTier]float64  `json:"by_tier"`
	ByPool      map[string]float64       `json:"by_pool"`
	ByCategory  map[CostCategory]float64 `json:"by_category"`
	Trend       *CostTrend               `json:"trend,omitempty"`
	PeriodStart time.Time                `json:"period_start"`
	PeriodEnd   time.Time                `json:"period_end"`
	GeneratedAt time.Time                `json:"generated_at"`
}

// CostTrend 成本趋势
type CostTrend struct {
	DailyCosts    []DailyCost `json:"daily_costs"`
	MonthlyGrowth float64     `json:"monthly_growth"`
	ProjectedCost float64     `json:"projected_cost"`
}

// DailyCost 每日成本
type DailyCost struct {
	Date time.Time `json:"date"`
	Cost float64   `json:"cost"`
}

// CostForecast 成本预测
type CostForecast struct {
	ID           string            `json:"id"`
	ForecastName string            `json:"forecast_name"`
	CurrentCost  float64           `json:"current_cost"`
	ForecastCost float64           `json:"forecast_cost"`
	Currency     string            `json:"currency"`
	Months       int               `json:"months"`
	GrowthRate   float64           `json:"growth_rate"`
	Confidence   float64           `json:"confidence"`
	MonthlyData  []MonthlyForecast `json:"monthly_data"`
	Assumptions  []string          `json:"assumptions"`
	CreatedAt    time.Time         `json:"created_at"`
}

// MonthlyForecast 月度预测
type MonthlyForecast struct {
	Month         string  `json:"month"`
	ProjectedGB   float64 `json:"projected_gb"`
	ProjectedCost float64 `json:"projected_cost"`
}

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	ID              string           `json:"id"`
	Type            OptimizationType `json:"type"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	EstimatedSaving float64          `json:"estimated_saving"`
	Currency        string           `json:"currency"`
	Priority        int              `json:"priority"`
	TargetPool      string           `json:"target_pool,omitempty"`
	TargetVolume    string           `json:"target_volume,omitempty"`
	TargetDirectory string           `json:"target_directory,omitempty"`
	CurrentTier     StorageTier      `json:"current_tier,omitempty"`
	RecommendedTier StorageTier      `json:"recommended_tier,omitempty"`
	SavingsPercent  float64          `json:"savings_percent"`
	Complexity      string           `json:"complexity"`
	RiskLevel       string           `json:"risk_level"`
	CreatedAt       time.Time        `json:"created_at"`
}

// StorageCostConfig 存储成本配置
type StorageCostConfig struct {
	Enabled         bool                    `json:"enabled"`
	DefaultCurrency string                  `json:"default_currency"`
	Currency        string                  `json:"currency"` // 测试兼容字段
	TierPricing     map[StorageTier]float64 `json:"tier_pricing"`
	TransferRate    float64                 `json:"transfer_rate"`
	RequestRate     float64                 `json:"request_rate"`
	AlertThreshold  float64                 `json:"alert_threshold"`
	ForecastMonths  int                     `json:"forecast_months"`
	BudgetLimit     float64                 `json:"budget_limit"`      // 测试兼容字段
	DefaultPriceSSD float64                 `json:"default_price_ssd"` // 测试兼容字段
	DefaultPriceHDD float64                 `json:"default_price_hdd"` // 测试兼容字段
}

// DefaultStorageCostConfig 默认配置
func DefaultStorageCostConfig() *StorageCostConfig {
	return &StorageCostConfig{
		Enabled:         true,
		DefaultCurrency: "CNY",
		TierPricing: map[StorageTier]float64{
			TierHot:     0.50,
			TierWarm:    0.20,
			TierCold:    0.08,
			TierGlacier: 0.03,
		},
		TransferRate:   0.10,
		RequestRate:    0.01,
		AlertThreshold: 1000,
		ForecastMonths: 12,
	}
}
