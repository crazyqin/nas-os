// Package costpredict - 成本预测模块
// 基于历史数据的成本预测、存储增长趋势预测、预算超支告警
package costpredict

import (
	"time"
)

// ============================================================
// 预测模型类型
// ============================================================

// PredictionMethod 预测方法
type PredictionMethod string

const (
	MethodLinearRegression     PredictionMethod = "linear_regression"     // 线性回归
	MethodExponentialSmoothing PredictionMethod = "exponential_smoothing" // 指数平滑
	MethodMovingAverage        PredictionMethod = "moving_average"        // 移动平均
)

// ConfidenceLevel 置信度水平
type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "high"
	ConfidenceMedium ConfidenceLevel = "medium"
	ConfidenceLow    ConfidenceLevel = "low"
)

// ForecastHorizon 预测时间范围
type ForecastHorizon string

const (
	Horizon3Months ForecastHorizon = "3_months"
	Horizon6Months ForecastHorizon = "6_months"
	Horizon1Year   ForecastHorizon = "1_year"
	Horizon2Years  ForecastHorizon = "2_years"
)

// ============================================================
// 历史数据类型
// ============================================================

// GrowthRecord 增长记录
type GrowthRecord struct {
	ID         string    `json:"id"`
	Date       time.Time `json:"date"`
	StorageGB  float64   `json:"storage_gb"`
	GrowthGB   float64   `json:"growth_gb"`   // 增长量
	GrowthRate float64   `json:"growth_rate"` // 增长率
	Provider   string    `json:"provider"`
	Tier       string    `json:"tier"`
	CreatedAt  time.Time `json:"created_at"`
}

// ============================================================
// 预测结果类型
// ============================================================

// ForecastPoint 预测点
type ForecastPoint struct {
	Date       time.Time       `json:"date"`
	StorageGB  float64         `json:"storage_gb"`
	Cost       float64         `json:"cost"`
	Confidence ConfidenceLevel `json:"confidence"`
	UpperBound float64         `json:"upper_bound"` // 上界
	LowerBound float64         `json:"lower_bound"` // 下界
}

// ForecastResult 预测结果
type ForecastResult struct {
	ID             string           `json:"id"`
	Method         PredictionMethod `json:"method"`
	Horizon        ForecastHorizon  `json:"horizon"`
	HistoricalData []CostRecord     `json:"historical_data"`
	Forecasts      []ForecastPoint  `json:"forecasts"`
	Accuracy       float64          `json:"accuracy"` // 预测准确度 (0-100)
	Confidence     ConfidenceLevel  `json:"confidence"`
	Trend          string           `json:"trend"` // "increasing", "decreasing", "stable"
	GrowthRate     float64          `json:"growth_rate"`
	GeneratedAt    time.Time        `json:"generated_at"`
}

// GrowthForecast 增长预测
type GrowthForecast struct {
	ID              string        `json:"id"`
	CurrentStorage  float64       `json:"current_storage_gb"`
	Forecasts       []GrowthPoint `json:"forecasts"`
	GrowthRate      float64       `json:"growth_rate"`
	StorageCapacity float64       `json:"storage_capacity_gb"` // 预计达到容量的时间
	DaysToCapacity  int           `json:"days_to_capacity"`
	GeneratedAt     time.Time     `json:"generated_at"`
}

// GrowthPoint 增长预测点
type GrowthPoint struct {
	Date      time.Time `json:"date"`
	StorageGB float64   `json:"storage_gb"`
	GrowthGB  float64   `json:"growth_gb"`
}

// ============================================================
// 预算告警类型
// ============================================================

// AlertType 告警类型
type AlertType string

const (
	AlertBudgetExceeded AlertType = "budget_exceeded" // 预算超支
	AlertBudgetWarning  AlertType = "budget_warning"  // 预算警告
	AlertGrowthSpike    AlertType = "growth_spike"    // 增长异常
	AlertCostAnomaly    AlertType = "cost_anomaly"    // 成本异常
)

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityWarning  AlertSeverity = "warning"
	SeverityInfo     AlertSeverity = "info"
)

// AlertConfig 告警配置
type AlertConfig struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Type              AlertType `json:"type"`
	Budget            float64   `json:"budget"`
	WarningThreshold  float64   `json:"warning_threshold"`  // 警告阈值 (百分比)
	CriticalThreshold float64   `json:"critical_threshold"` // 严重阈值 (百分比)
	Enabled           bool      `json:"enabled"`
	Provider          string    `json:"provider,omitempty"`
	Region            string    `json:"region,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// AlertConfigRequest 告警配置请求
type AlertConfigRequest struct {
	Name              string    `json:"name" binding:"required"`
	Type              AlertType `json:"type" binding:"required"`
	Budget            float64   `json:"budget" binding:"required,gt=0"`
	WarningThreshold  float64   `json:"warning_threshold" binding:"required,gte=0,lte=100"`
	CriticalThreshold float64   `json:"critical_threshold" binding:"required,gte=0,lte=100"`
	Enabled           bool      `json:"enabled"`
	Provider          string    `json:"provider"`
	Region            string    `json:"region"`
}

// ============================================================
// 报告类型
// ============================================================

// PredictionReport 预测报告
type PredictionReport struct {
	ID              string         `json:"id"`
	Title           string         `json:"title"`
	Summary         ReportSummary  `json:"summary"`
	CostForecast    ForecastResult `json:"cost_forecast"`
	GrowthForecast  GrowthForecast `json:"growth_forecast"`
	Alerts          []BudgetAlert  `json:"alerts"`
	Recommendations []string       `json:"recommendations"`
	GeneratedAt     time.Time      `json:"generated_at"`
	ValidUntil      time.Time      `json:"valid_until"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	CurrentMonthlyCost float64 `json:"current_monthly_cost"`
	ForecastedCost     float64 `json:"forecasted_cost"`
	CostChange         float64 `json:"cost_change"` // 百分比
	CurrentStorage     float64 `json:"current_storage_gb"`
	ForecastedStorage  float64 `json:"forecasted_storage_gb"`
	GrowthRate         float64 `json:"growth_rate"`
	BudgetUtilization  float64 `json:"budget_utilization"` // 百分比
	ActiveAlerts       int     `json:"active_alerts"`
}

// ============================================================
// 配置类型
// ============================================================

// CostPredictConfig 成本预测配置
type CostPredictConfig struct {
	Enabled             bool             `json:"enabled"`
	DefaultMethod       PredictionMethod `json:"default_method"`
	DefaultHorizon      ForecastHorizon  `json:"default_horizon"`
	MinHistoryMonths    int              `json:"min_history_months"`   // 最少历史月数
	MaxForecastMonths   int              `json:"max_forecast_months"`  // 最大预测月数
	ConfidenceThreshold float64          `json:"confidence_threshold"` // 置信度阈值
	UpdateIntervalHours int              `json:"update_interval_hours"`
}

// DefaultCostPredictConfig 默认配置
func DefaultCostPredictConfig() CostPredictConfig {
	return CostPredictConfig{
		Enabled:             true,
		DefaultMethod:       MethodLinearRegression,
		DefaultHorizon:      Horizon1Year,
		MinHistoryMonths:    3,
		MaxForecastMonths:   24,
		ConfidenceThreshold: 0.7,
		UpdateIntervalHours: 24,
	}
}
