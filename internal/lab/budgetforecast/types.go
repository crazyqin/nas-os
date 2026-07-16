// Package budgetforecast 提供存储预算预测功能，基于历史使用数据预测未来存储成本。
// 差异化优势：竞品（TrueNAS/群晖/飞牛）均无内置预算预测功能。
package budgetforecast

import "time"

// UsageSnapshot 使用快照.
type UsageSnapshot struct {
	Date       time.Time `json:"date"`
	UsedBytes  int64     `json:"used_bytes"`
	TotalBytes int64     `json:"total_bytes"`
	CostPerTB  float64   `json:"cost_per_tb"` // 每TB月成本（元）
}

// ForecastPoint 预测数据点.
type ForecastPoint struct {
	Date          time.Time `json:"date"`
	PredictedGB   float64   `json:"predicted_gb"`
	PredictedCost float64   `json:"predicted_cost"` // 预测月成本（元）
	Confidence    float64   `json:"confidence"`     // 置信度 0~1
}

// BudgetAlert 预算告警.
type BudgetAlert struct {
	Threshold     float64   `json:"threshold"`      // 阈值（元/月）
	PredictedDate time.Time `json:"predicted_date"` // 预计触发日期
	Severity      string    `json:"severity"`       // info / warning / critical
}

// ForecastResult 预测结果.
type ForecastResult struct {
	GeneratedAt   time.Time       `json:"generated_at"`
	HistoryDays   int             `json:"history_days"`
	HistoryPoints []UsageSnapshot `json:"history_points"`
	Forecast      []ForecastPoint `json:"forecast"`
	Alerts        []BudgetAlert   `json:"alerts"`
	// Summary
	CurrentUsageGB  float64 `json:"current_usage_gb"`
	MonthlyGrowthGB float64 `json:"monthly_growth_gb"` // 月均增长 GB
	DaysUntilFull   int     `json:"days_until_full"`   // 预计多少天后磁盘满
	MonthlyCostNow  float64 `json:"monthly_cost_now"`  // 当前月成本
	AnnualCostEst   float64 `json:"annual_cost_est"`   // 预估年成本
}
