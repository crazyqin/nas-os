// Package storageforecast 提供存储趋势预测与容量规划功能
// 对标群晖 Active Insight Storage Forecast
// 特性：容量趋势分析、增长预测（线性回归+移动平均）、告警阈值、扩容建议、成本估算
package storageforecast

import (
	"time"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
	AlertFull     AlertLevel = "full"
)

// TrendDirection 趋势方向
type TrendDirection string

const (
	TrendIncreasing TrendDirection = "increasing"
	TrendDecreasing TrendDirection = "decreasing"
	TrendStable     TrendDirection = "stable"
	TrendUnknown    TrendDirection = "unknown"
)

// TimeGranularity 时间粒度
type TimeGranularity string

const (
	GranularityDay   TimeGranularity = "day"
	GranularityWeek  TimeGranularity = "week"
	GranularityMonth TimeGranularity = "month"
)

// StoragePool 存储池
type StoragePool struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	TotalBytes  int64     `json:"total_bytes"`
	UsedBytes   int64     `json:"used_bytes"`
	FreeBytes   int64     `json:"free_bytes"`
	UsedPercent float64   `json:"used_percent"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UsageSnapshot 使用量快照
type UsageSnapshot struct {
	Timestamp   time.Time `json:"timestamp"`
	PoolID      string    `json:"pool_id"`
	TotalBytes  int64     `json:"total_bytes"`
	UsedBytes   int64     `json:"used_bytes"`
	FreeBytes   int64     `json:"free_bytes"`
	UsedPercent float64   `json:"used_percent"`
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	UsedBytes   int64     `json:"used_bytes"`
	UsedPercent float64   `json:"used_percent"`
}

// TrendSeries 趋势序列
type TrendSeries struct {
	PoolID      string          `json:"pool_id"`
	PoolName    string          `json:"pool_name"`
	Granularity TimeGranularity `json:"granularity"`
	Points      []TrendPoint    `json:"points"`
}

// ForecastResult 预测结果
type ForecastResult struct {
	PoolID            string         `json:"pool_id"`
	PoolName          string         `json:"pool_name"`
	CurrentUsage      float64        `json:"current_usage"`
	Trend             TrendDirection `json:"trend"`
	DailyGrowthBytes  int64          `json:"daily_growth_bytes"`
	DailyGrowthRate   float64        `json:"daily_growth_rate"`
	DaysUntilFull     int            `json:"days_until_full"`
	EstimatedFullDate *time.Time     `json:"estimated_full_date,omitempty"`
	Confidence        float64        `json:"confidence"`
	AlertLevel        AlertLevel     `json:"alert_level"`
	Suggestions       []string       `json:"suggestions"`
}

// ExpansionRecommendation 扩容建议
type ExpansionRecommendation struct {
	PoolID              string    `json:"pool_id"`
	PoolName            string    `json:"pool_name"`
	CurrentTotalBytes   int64     `json:"current_total_bytes"`
	CurrentUsedBytes    int64     `json:"current_used_bytes"`
	DailyGrowthBytes    int64     `json:"daily_growth_bytes"`
	DaysUntilFull       int       `json:"days_until_full"`
	RecommendedAddBytes int64     `json:"recommended_add_bytes"`
	RecommendedAddSize  string    `json:"recommended_add_size"`
	TargetDays          int       `json:"target_days"`
	EstimatedCost       float64   `json:"estimated_cost"`
	CostCurrency        string    `json:"cost_currency"`
	Urgency             string    `json:"urgency"`
	CreatedAt           time.Time `json:"created_at"`
}

// StorageCostEstimate 存储成本估算
type StorageCostEstimate struct {
	PoolID           string  `json:"pool_id"`
	PoolName         string  `json:"pool_name"`
	TotalBytes       int64   `json:"total_bytes"`
	UsedBytes        int64   `json:"used_bytes"`
	CostPerGBMonth   float64 `json:"cost_per_gb_month"`
	MonthlyCost      float64 `json:"monthly_cost"`
	ProjectedCost3M  float64 `json:"projected_cost_3m"`
	ProjectedCost6M  float64 `json:"projected_cost_6m"`
	ProjectedCost12M float64 `json:"projected_cost_12m"`
	Currency         string  `json:"currency"`
}

// ForecastConfig 配置
type ForecastConfig struct {
	Enabled             bool          `json:"enabled"`
	WarningThreshold    float64       `json:"warning_threshold"`  // 80%
	CriticalThreshold   float64       `json:"critical_threshold"` // 90%
	FullThreshold       float64       `json:"full_threshold"`     // 95%
	ForecastDays        int           `json:"forecast_days"`      // 预测天数
	SnapshotInterval    time.Duration `json:"snapshot_interval"`
	MaxSnapshots        int           `json:"max_snapshots"`
	MinDataPoints       int           `json:"min_data_points"`       // 最少数据点
	MovingAverageWindow int           `json:"moving_average_window"` // 移动平均窗口
	CostPerGBMonth      float64       `json:"cost_per_gb_month"`     // 每 GB 每月成本
	CostCurrency        string        `json:"cost_currency"`         // 货币单位
	ExpansionTargetDays int           `json:"expansion_target_days"` // 扩容目标天数
}

// DefaultConfig 返回默认配置
func DefaultConfig() ForecastConfig {
	return ForecastConfig{
		Enabled:             true,
		WarningThreshold:    80.0,
		CriticalThreshold:   90.0,
		FullThreshold:       95.0,
		ForecastDays:        90,
		SnapshotInterval:    1 * time.Hour,
		MaxSnapshots:        2160, // 90 天 * 24 小时
		MinDataPoints:       7,    // 至少 7 个数据点
		MovingAverageWindow: 7,    // 7 天移动平均
		CostPerGBMonth:      0.02, // $0.02/GB/月
		CostCurrency:        "USD",
		ExpansionTargetDays: 180, // 扩容后至少支撑 180 天
	}
}

// Alert 告警
type Alert struct {
	ID        string     `json:"id"`
	PoolID    string     `json:"pool_id"`
	PoolName  string     `json:"pool_name"`
	Level     AlertLevel `json:"level"`
	Message   string     `json:"message"`
	Threshold float64    `json:"threshold"`
	Current   float64    `json:"current"`
	CreatedAt time.Time  `json:"created_at"`
	Dismissed bool       `json:"dismissed"`
}
