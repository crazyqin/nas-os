// Package cloudbillfc 提供云存储成本预测能力
// 支持多云provider价格对比、用量趋势分析、成本预测及优化建议
package cloudbillfc

import "time"

// ForecastConfig 预测配置.
type ForecastConfig struct {
	Provider        string  `json:"provider"`          // 云服务商名称
	StorageGB       float64 `json:"storage_gb"`        // 当前存储用量（GB）
	MonthlyEgressGB float64 `json:"monthly_egress_gb"` // 月出口流量（GB）
	MonthlyAPI10K   float64 `json:"monthly_api_10k"`   // 月API调用（万次）
	GrowthRateMonth float64 `json:"growth_rate_month"` // 月增长率（如0.05=5%）
	Months          int     `json:"months"`            // 预测月数
}

// ProviderPricing 云服务商定价.
type ProviderPricing struct {
	ProviderName     string      `json:"provider_name"`      // 服务商名称
	StoragePerGB     float64     `json:"storage_per_gb"`     // 存储每GB月费（元）
	EgressPerGB      float64     `json:"egress_per_gb"`      // 出口流量每GB（元）
	APIPer10K        float64     `json:"api_per_10k"`        // 每万次API（元）
	RequestPer10K    float64     `json:"request_per_10k"`    // 每万次请求（元）
	FreeEgressGB     float64     `json:"free_egress_gb"`     // 免费出口流量（GB/月）
	FreeAPI10K       float64     `json:"free_api_10k"`       // 免费API调用（万次/月）
	MinMonthlyCharge float64     `json:"min_monthly_charge"` // 最低月费（元）
	TieredPricing    bool        `json:"tiered_pricing"`     // 是否分级定价
	Tiers            []PriceTier `json:"tiers"`              // 分级定价表
}

// PriceTier 分级定价.
type PriceTier struct {
	MinGB   float64 `json:"min_gb"`   // 起始容量（GB）
	MaxGB   float64 `json:"max_gb"`   // 结束容量（GB），-1表示无上限
	PriceGB float64 `json:"price_gb"` // 该区间每GB价格
}

// CostTrend 成本趋势.
type CostTrend struct {
	Month       int     `json:"month"`        // 第几月（0=当前）
	Date        string  `json:"date"`         // 日期标识 YYYY-MM
	StorageGB   float64 `json:"storage_gb"`   // 存储用量（GB）
	MonthlyCost float64 `json:"monthly_cost"` // 月度费用（元）
	EgressCost  float64 `json:"egress_cost"`  // 出口流量费
	APICost     float64 `json:"api_cost"`     // API费
	StorageCost float64 `json:"storage_cost"` // 存储费
}

// MonthlyForecast 月度预测结果.
type MonthlyForecast struct {
	ProviderName string         `json:"provider_name"` // 服务商名称
	Config       ForecastConfig `json:"config"`        // 预测配置
	Trends       []CostTrend    `json:"trends"`        // 月度趋势
	TotalCost    float64        `json:"total_cost"`    // 预测期总费用（元）
	AvgMonthly   float64        `json:"avg_monthly"`   // 月均费用（元）
	PeakMonthly  float64        `json:"peak_monthly"`  // 峰值月费（元）

	// 趋势分析
	GrowthPercent      float64 `json:"growth_percent"`       // 预测期间成本增长率（%）
	FinalMonthlyCost   float64 `json:"final_monthly_cost"`   // 末期月费
	InitialMonthlyCost float64 `json:"initial_monthly_cost"` // 首期月费
}

// CloudBillingForecast 云存储成本预测报告.
type CloudBillingForecast struct {
	GeneratedAt   time.Time         `json:"generated_at"`  // 生成时间
	Months        int               `json:"months"`        // 预测月数
	Config        ForecastConfig    `json:"config"`        // 预测配置
	Forecasts     []MonthlyForecast `json:"forecasts"`     // 各服务商预测
	BestProvider  string            `json:"best_provider"` // 最优服务商
	Optimizations []OptimizationTip `json:"optimizations"` // 优化建议
}

// OptimizationTip 优化建议.
type OptimizationTip struct {
	Type          string  `json:"type"`           // 建议类型：storage/egress/api/tier/lifecycle
	Title         string  `json:"title"`          // 建议标题
	Description   string  `json:"description"`    // 详细说明
	SavingPercent float64 `json:"saving_percent"` // 预计节省百分比
	SavingAmount  float64 `json:"saving_amount"`  // 预计节省金额（元/月）
}

// ProviderComparison 服务商对比项.
type ProviderComparison struct {
	ProviderName string  `json:"provider_name"`  // 服务商名称
	MonthlyCost  float64 `json:"monthly_cost"`   // 当前月费
	YearlyCost   float64 `json:"yearly_cost"`    // 年费
	FiveYearCost float64 `json:"five_year_cost"` // 5年费用
	CostPerGB    float64 `json:"cost_per_gb"`    // 每GB月成本
}
