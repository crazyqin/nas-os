package reports

import "time"

// LegacyCostSummary 兼容旧测试读取 TotalMonthlyCost 字段。
type LegacyCostSummary struct {
	TotalMonthlyCost float64 `json:"total_monthly_cost"`
	TotalCapacityGB  float64 `json:"total_capacity_gb"`
	TotalUsedGB      float64 `json:"total_used_gb"`
	AvgUsagePercent  float64 `json:"avg_usage_percent"`
	PotentialSavings float64 `json:"potential_savings"`
	HealthScore      int     `json:"health_score"`
	VolumeCount      int     `json:"volume_count"`
}

// LegacyCostAnalysisReport 为旧测试和旧 API 提供兼容结构。
type LegacyCostAnalysisReport struct {
	ID            string                 `json:"id"`
	GeneratedAt   time.Time              `json:"generated_at"`
	VolumeCosts   []StorageCostResult    `json:"volume_costs"`
	UserCosts     []UserCostAnalysis     `json:"user_costs"`
	TotalCost     LegacyCostSummary      `json:"total_cost"`
	TrendAnalysis CostTrendAnalysis      `json:"trend_analysis"`
	Optimization  []CostOptimizationItem `json:"optimization"`
}

// CostOptimizationItem 兼容旧优化项类型。
type CostOptimizationItem struct {
	ID       string  `json:"id"`
	Priority string  `json:"priority"`
	Savings  float64 `json:"savings"`
}
