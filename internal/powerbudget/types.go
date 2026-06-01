// Package powerbudget 提供用电预算管理功能
package powerbudget

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrInvalidPowerWatts 无效的功率值错误.
	ErrInvalidPowerWatts = errors.New("功率值必须非负")
	// ErrInvalidBudgetAmount 无效的预算金额错误.
	ErrInvalidBudgetAmount = errors.New("预算金额必须大于零")
	// ErrInvalidElectricityPrice 无效的电价错误.
	ErrInvalidElectricityPrice = errors.New("电价必须大于零")
	// ErrBudgetNotSet 预算未设置错误.
	ErrBudgetNotSet = errors.New("用电预算未设置")
	// ErrRecordNotFound 记录不存在错误.
	ErrRecordNotFound = errors.New("用电记录不存在")
	// ErrInsufficientData 数据不足错误.
	ErrInsufficientData = errors.New("数据不足，无法进行分析")
	// ErrInvalidDateRange 无效的日期范围错误.
	ErrInvalidDateRange = errors.New("无效的日期范围")
	// ErrEngineNotRunning 引擎未运行错误.
	ErrEngineNotRunning = errors.New("引擎未运行")
	// ErrDeviceNotFound 设备不存在错误.
	ErrDeviceNotFound = errors.New("设备不存在")
)

// ========== 告警级别 ==========

// AlertLevel 告警级别.
type AlertLevel string

// 告警级别常量.
const (
	// AlertLevelInfo 信息级别.
	AlertLevelInfo AlertLevel = "info"
	// AlertLevelWarning 警告级别.
	AlertLevelWarning AlertLevel = "warning"
	// AlertLevelCritical 严重级别.
	AlertLevelCritical AlertLevel = "critical"
	// AlertLevelEmergency 紧急级别.
	AlertLevelEmergency AlertLevel = "emergency"
)

// ========== 告警类型 ==========

// AlertType 告警类型.
type AlertType string

// 告警类型常量.
const (
	// AlertTypeBudgetExceeded 预算超支.
	AlertTypeBudgetExceeded AlertType = "budget_exceeded"
	// AlertTypeBudgetWarning 预算预警.
	AlertTypeBudgetWarning AlertType = "budget_warning"
	// AlertTypeAnomalyPower 异常功耗.
	AlertTypeAnomalyPower AlertType = "anomaly_power"
	// AlertTypeDeviceOverload 设备过载.
	AlertTypeDeviceOverload AlertType = "device_overload"
)

// ========== 统计周期 ==========

// ReportPeriod 报告周期.
type ReportPeriod string

// 报告周期常量.
const (
	// PeriodDaily 日报.
	PeriodDaily ReportPeriod = "daily"
	// PeriodWeekly 周报.
	PeriodWeekly ReportPeriod = "weekly"
	// PeriodMonthly 月报.
	PeriodMonthly ReportPeriod = "monthly"
)

// ========== 趋势方向 ==========

// TrendDirection 趋势方向.
type TrendDirection string

// 趋势方向常量.
const (
	// TrendUp 上升趋势.
	TrendUp TrendDirection = "up"
	// TrendDown 下降趋势.
	TrendDown TrendDirection = "down"
	// TrendStable 稳定趋势.
	TrendStable TrendDirection = "stable"
)

// ========== 核心数据结构 ==========

// PowerRecord 用电记录.
type PowerRecord struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	DeviceID    string    `json:"device_id"`
	DeviceName  string    `json:"device_name"`
	PowerWatts  float64   `json:"power_watts"`  // 实时功率（瓦特）
	EnergyKWh   float64   `json:"energy_kwh"`   // 用电量（千瓦时）
	CostCents   int64     `json:"cost_cents"`    // 成本（分）
	Duration    int64     `json:"duration_sec"`  // 持续时间（秒）
	Service     string    `json:"service"`       // 关联服务
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Budget 用电预算配置.
type Budget struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	MonthlyAmount    float64   `json:"monthly_amount"`    // 月度预算（分）
	ElectricityPrice float64   `json:"electricity_price"` // 电价（分/千瓦时）
	WarningThreshold float64   `json:"warning_threshold"` // 预警阈值（百分比 0-100）
	CriticalThreshold float64  `json:"critical_threshold"` // 严重阈值（百分比 0-100）
	StartDate        time.Time `json:"start_date"`
	EndDate          *time.Time `json:"end_date,omitempty"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// PowerReport 用电报告.
type PowerReport struct {
	ID            string         `json:"id"`
	Period        ReportPeriod   `json:"period"`
	StartTime     time.Time      `json:"start_time"`
	EndTime       time.Time      `json:"end_time"`
	TotalEnergy   float64        `json:"total_energy_kwh"`  // 总用电量
	TotalCost     int64          `json:"total_cost_cents"`  // 总成本
	AvgDailyCost  int64          `json:"avg_daily_cost"`    // 日均成本
	BudgetUsed    float64        `json:"budget_used_pct"`   // 预算使用百分比
	BudgetRemain  int64          `json:"budget_remain"`     // 预算剩余
	DailyTrend    []TrendPoint   `json:"daily_trend"`       // 每日趋势
	TopDevices    []*DevicePower  `json:"top_devices"`       // 高耗电设备
	Trend         TrendDirection `json:"trend"`             // 整体趋势
	Prediction    *Prediction    `json:"prediction,omitempty"` // 预测
	GeneratedAt   time.Time      `json:"generated_at"`
}

// DevicePower 设备功耗画像.
type DevicePower struct {
	DeviceID       string          `json:"device_id"`
	DeviceName     string          `json:"device_name"`
	TotalEnergy    float64         `json:"total_energy_kwh"`   // 总用电量
	TotalCost      int64           `json:"total_cost_cents"`   // 总成本
	AvgPower       float64         `json:"avg_power_watts"`    // 平均功率
	PeakPower      float64         `json:"peak_power_watts"`   // 峰值功率
	UsagePercent   float64         `json:"usage_percent"`      // 使用占比
	RecordCount    int             `json:"record_count"`       // 记录数
	FirstSeen      time.Time       `json:"first_seen"`
	LastSeen       time.Time       `json:"last_seen"`
	HourlyProfile  []HourlyPower   `json:"hourly_profile"`    // 小时级画像
}

// HourlyPower 小时功耗数据.
type HourlyPower struct {
	Hour      int     `json:"hour"`           // 0-23
	AvgPower  float64 `json:"avg_power_watts"` // 平均功率
	Energy    float64 `json:"energy_kwh"`      // 用电量
	RecordNum int     `json:"record_num"`      // 记录数
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	Date   time.Time `json:"date"`
	Energy float64   `json:"energy_kwh"`
	Cost   int64     `json:"cost_cents"`
}

// Prediction 用电预测.
type Prediction struct {
	Method       string  `json:"method"`        // 预测方法
	DaysLeft     int     `json:"days_left"`     // 本月剩余天数
	DailyAvg     float64 `json:"daily_avg_kwh"` // 日均用电量
	PredictedKWh float64 `json:"predicted_kwh"` // 预测月用电量
	PredictedCost int64  `json:"predicted_cost"` // 预测月成本
	Confidence   float64 `json:"confidence"`    // 置信度 0-1
	WillExceed   bool    `json:"will_exceed"`   // 是否会超预算
}

// ========== 预算状态 ==========

// BudgetStatus 预算状态.
type BudgetStatus struct {
	Budget        *Budget  `json:"budget"`
	UsedEnergy    float64  `json:"used_energy_kwh"`
	UsedCost      int64    `json:"used_cost_cents"`
	Remaining     int64    `json:"remaining_cents"`
	UsedPercent   float64  `json:"used_percent"`
	DailyAvg      float64  `json:"daily_avg_kwh"`
	DaysElapsed   int      `json:"days_elapsed"`
	DaysRemaining int      `json:"days_remaining"`
	Trend         TrendDirection `json:"trend"`
	Alerts        []*Alert `json:"active_alerts"`
}

// ========== 告警 ==========

// Alert 告警.
type Alert struct {
	ID          string     `json:"id"`
	Type        AlertType  `json:"type"`
	Level       AlertLevel `json:"level"`
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	DeviceID    string     `json:"device_id,omitempty"`
	Value       float64    `json:"value"`
	Threshold   float64    `json:"threshold"`
	TriggeredAt time.Time  `json:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Active      bool       `json:"active"`
}

// ========== 请求结构 ==========

// RecordPowerRequest 记录用电请求.
type RecordPowerRequest struct {
	DeviceID    string            `json:"device_id" binding:"required"`
	DeviceName  string            `json:"device_name" binding:"required"`
	PowerWatts  float64           `json:"power_watts" binding:"required,min=0"`
	DurationSec int64             `json:"duration_sec"`
	Service     string            `json:"service"`
	Metadata    map[string]string `json:"metadata"`
}

// SetBudgetRequest 设置预算请求.
type SetBudgetRequest struct {
	Name              string  `json:"name"`
	MonthlyAmount     float64 `json:"monthly_amount" binding:"required,gt=0"`
	ElectricityPrice  float64 `json:"electricity_price" binding:"required,gt=0"`
	WarningThreshold  float64 `json:"warning_threshold"`
	CriticalThreshold float64 `json:"critical_threshold"`
}

// ReportRequest 报告请求.
type ReportRequest struct {
	Period    ReportPeriod `json:"period"`
	StartTime *time.Time   `json:"start_time,omitempty"`
	EndTime   *time.Time   `json:"end_time,omitempty"`
	DeviceID  string       `json:"device_id,omitempty"`
}

// ========== 默认配置 ==========

// DefaultElectricityPrice 默认电价（分/千瓦时），约0.56元/度.
const DefaultElectricityPrice float64 = 56.0

// DefaultMonthlyBudget 默认月度预算（分），约200元/月.
const DefaultMonthlyBudget float64 = 20000.0

// DefaultWarningThreshold 默认预警阈值 80%.
const DefaultWarningThreshold float64 = 80.0

// DefaultCriticalThreshold 默认严重阈值 95%.
const DefaultCriticalThreshold float64 = 95.0

// DefaultBudgetConfig 返回默认预算配置.
func DefaultBudgetConfig() *Budget {
	now := time.Now()
	return &Budget{
		Name:              "默认用电预算",
		MonthlyAmount:     DefaultMonthlyBudget,
		ElectricityPrice:  DefaultElectricityPrice,
		WarningThreshold:  DefaultWarningThreshold,
		CriticalThreshold: DefaultCriticalThreshold,
		StartDate:         now,
		Enabled:           true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}
