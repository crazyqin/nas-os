// Package capacityforecast 提供存储容量预测、增长分析、告警、What-If 模拟和扩容建议功能
package capacityforecast

import (
	"time"
)

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertInfo      AlertLevel = "info"
	AlertWarning   AlertLevel = "warning"
	AlertCritical  AlertLevel = "critical"
	AlertEmergency AlertLevel = "emergency"
)

// TrendDirection 趋势方向.
type TrendDirection string

const (
	TrendIncreasing TrendDirection = "increasing"
	TrendDecreasing TrendDirection = "decreasing"
	TrendStable     TrendDirection = "stable"
	TrendUnknown    TrendDirection = "unknown"
)

// DataType 数据类型.
type DataType string

const (
	DataTypeMedia    DataType = "media"
	DataTypeBackup   DataType = "backup"
	DataTypeDocument DataType = "document"
	DataTypeApp      DataType = "app"
	DataTypeSystem   DataType = "system"
	DataTypeOther    DataType = "other"
)

// ForecastMethod 预测方法.
type ForecastMethod string

const (
	MethodLinearRegression ForecastMethod = "linear_regression"
	MethodMovingAverage    ForecastMethod = "moving_average"
	MethodExponential      ForecastMethod = "exponential_smoothing"
)

// ExpansionType 扩容类型.
type ExpansionType string

const (
	ExpansionAddDisk    ExpansionType = "add_disk"
	ExpansionReplaceAll ExpansionType = "replace_all"
	ExpansionAddPool    ExpansionType = "add_pool"
	ExpansionCloudTier  ExpansionType = "cloud_tier"
)

// CapacitySnapshot 容量快照.
type CapacitySnapshot struct {
	Timestamp   time.Time          `json:"timestamp"`
	TotalBytes  int64              `json:"total_bytes"`
	UsedBytes   int64              `json:"used_bytes"`
	FreeBytes   int64              `json:"free_bytes"`
	UsedPercent float64            `json:"used_percent"`
	ByType      map[DataType]int64 `json:"by_type,omitempty"`
}

// Forecast 容量预测结果.
type Forecast struct {
	ID                string         `json:"id"`
	TargetDate        time.Time      `json:"target_date"`
	CurrentUsage      float64        `json:"current_usage"`
	PredictedUsage    float64        `json:"predicted_usage"`
	PredictedBytes    int64          `json:"predicted_bytes"`
	Trend             TrendDirection `json:"trend"`
	Method            ForecastMethod `json:"method"`
	Confidence        float64        `json:"confidence"`
	DaysUntilFull     int            `json:"days_until_full"`
	EstimatedFullDate *time.Time     `json:"estimated_full_date,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

// GrowthRate 增长率分析结果.
type GrowthRate struct {
	DataType           DataType       `json:"data_type"`
	DailyGrowthBytes   int64          `json:"daily_growth_bytes"`
	WeeklyGrowthBytes  int64          `json:"weekly_growth_bytes"`
	MonthlyGrowthBytes int64          `json:"monthly_growth_bytes"`
	GrowthPercent      float64        `json:"growth_percent"`
	Trend              TrendDirection `json:"trend"`
	CurrentSizeBytes   int64          `json:"current_size_bytes"`
	TotalShare         float64        `json:"total_share"`
}

// GrowthAnalysis 增长分析汇总.
type GrowthAnalysis struct {
	OverallGrowthRate float64      `json:"overall_growth_rate"`
	OverallDailyBytes int64        `json:"overall_daily_bytes"`
	ByType            []GrowthRate `json:"by_type"`
	AnalysisTime      time.Time    `json:"analysis_time"`
}

// CapacityAlert 容量告警.
type CapacityAlert struct {
	ID           string     `json:"id"`
	Level        AlertLevel `json:"level"`
	Message      string     `json:"message"`
	Threshold    float64    `json:"threshold"`
	Current      float64    `json:"current"`
	DataType     DataType   `json:"data_type,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	Dismissed    bool       `json:"dismissed"`
	Acknowledged bool       `json:"acknowledged"`
}

// WhatIfScenario What-If 模拟场景.
type WhatIfScenario struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Modifications   []Modification    `json:"modifications"`
	SimulatedResult *SimulationResult `json:"simulated_result,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}

// Modification 容量修改项.
type Modification struct {
	Type        string   `json:"type"`      // "add_data", "remove_data", "add_capacity"
	DataType    DataType `json:"data_type"` // 仅 add_data/remove_data 时使用
	AmountBytes int64    `json:"amount_bytes"`
	Description string   `json:"description"`
}

// SimulationResult 模拟结果.
type SimulationResult struct {
	ProjectedTotalBytes   int64       `json:"projected_total_bytes"`
	ProjectedUsedBytes    int64       `json:"projected_used_bytes"`
	ProjectedFreeBytes    int64       `json:"projected_free_bytes"`
	ProjectedUsage        float64     `json:"projected_usage"`
	ProjectedDaysToFull   int         `json:"projected_days_to_full"`
	ComparisonWithCurrent *Comparison `json:"comparison_with_current,omitempty"`
	EstimatedFullDate     *time.Time  `json:"estimated_full_date,omitempty"`
}

// Comparison 对比结果.
type Comparison struct {
	UsageChange      float64 `json:"usage_change"`
	FreeBytesChange  int64   `json:"free_bytes_change"`
	DaysToFullChange int     `json:"days_to_full_change"`
}

// ExpansionPlan 扩容方案.
type ExpansionPlan struct {
	ID                 string        `json:"id"`
	Type               ExpansionType `json:"type"`
	Name               string        `json:"name"`
	Description        string        `json:"description"`
	AddCapacityBytes   int64         `json:"add_capacity_bytes"`
	AddCapacityDisplay string        `json:"add_capacity_display"`
	EstimatedCost      float64       `json:"estimated_cost"`
	CostCurrency       string        `json:"cost_currency"`
	CostPerTB          float64       `json:"cost_per_tb"`
	DaysSupported      int           `json:"days_supported"`
	Urgency            string        `json:"urgency"`
	Pros               []string      `json:"pros"`
	Cons               []string      `json:"cons"`
	Rank               int           `json:"rank"`
}

// ExpansionRecommendation 扩容建议汇总.
type ExpansionRecommendation struct {
	CurrentTotalBytes int64           `json:"current_total_bytes"`
	CurrentUsedBytes  int64           `json:"current_used_bytes"`
	DailyGrowthBytes  int64           `json:"daily_growth_bytes"`
	DaysUntilFull     int             `json:"days_until_full"`
	Plans             []ExpansionPlan `json:"plans"`
	RecommendedPlan   *ExpansionPlan  `json:"recommended_plan,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

// ForecastConfig 配置.
type ForecastConfig struct {
	Enabled             bool          `json:"enabled"`
	WarningThreshold    float64       `json:"warning_threshold"`     // 80%
	CriticalThreshold   float64       `json:"critical_threshold"`    // 90%
	EmergencyThreshold  float64       `json:"emergency_threshold"`   // 95%
	ForecastDays        int           `json:"forecast_days"`         // 预测天数
	MinDataPoints       int           `json:"min_data_points"`       // 最少数据点
	MovingAverageWindow int           `json:"moving_average_window"` // 移动平均窗口
	MaxSnapshots        int           `json:"max_snapshots"`         // 最大快照数
	SnapshotInterval    time.Duration `json:"snapshot_interval"`     // 快照间隔
	CostPerTBMonth      float64       `json:"cost_per_tb_month"`     // 每 TB 每月成本
	CostCurrency        string        `json:"cost_currency"`         // 货币单位
	ExpansionTargetDays int           `json:"expansion_target_days"` // 扩容目标天数
}

// DefaultConfig 返回默认配置.
func DefaultConfig() ForecastConfig {
	return ForecastConfig{
		Enabled:             true,
		WarningThreshold:    80.0,
		CriticalThreshold:   90.0,
		EmergencyThreshold:  95.0,
		ForecastDays:        90,
		MinDataPoints:       7,
		MovingAverageWindow: 7,
		MaxSnapshots:        2160,
		SnapshotInterval:    1 * time.Hour,
		CostPerTBMonth:      20.0,
		CostCurrency:        "USD",
		ExpansionTargetDays: 180,
	}
}
