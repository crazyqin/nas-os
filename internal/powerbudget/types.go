// Package powerbudget 提供功率预算管理功能，支持设备功率监控、电费计算、节能策略、功率预算告警。
package powerbudget

import "time"

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

// AlertType 告警类型
type AlertType string

const (
	AlertOverBudget    AlertType = "over_budget"    // 超出预算
	AlertPeakDemand    AlertType = "peak_demand"    // 峰值需求
	AlertHighUsage     AlertType = "high_usage"     // 高使用率
	AlertCostThreshold AlertType = "cost_threshold" // 成本阈值
)

// SavingsType 节能类型
type SavingsType string

const (
	SavingsSchedule   SavingsType = "schedule"   // 定时调度
	SavingsIdle       SavingsType = "idle"       // 空闲关机
	SavingsEfficiency SavingsType = "efficiency" // 效率优化
	SavingsPeakShift  SavingsType = "peak_shift" // 峰值转移
)

// PowerReading 功率读数
type PowerReading struct {
	ID          string    `json:"id"`
	DeviceID    string    `json:"device_id"`
	DeviceName  string    `json:"device_name"`
	DeviceType  string    `json:"device_type"`
	Watts       float64   `json:"watts"`
	Voltage     float64   `json:"voltage"`
	Current     float64   `json:"current"`
	PowerFactor float64   `json:"power_factor"`
	Frequency   float64   `json:"frequency"`
	Timestamp   time.Time `json:"timestamp"`
}

// PowerBudget 功率预算
type PowerBudget struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	MaxWatts         float64   `json:"max_watts"`
	WarningWatts     float64   `json:"warning_watts"`
	DailyKWhLimit    float64   `json:"daily_kwh_limit"`
	MonthlyKWhLimit  float64   `json:"monthly_kwh_limit"`
	DailyCostLimit   float64   `json:"daily_cost_limit"`
	MonthlyCostLimit float64   `json:"monthly_cost_limit"`
	Currency         string    `json:"currency"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// EnergyCost 能源成本
type EnergyCost struct {
	ID              string    `json:"id"`
	Period          string    `json:"period"` // daily, weekly, monthly
	TotalKWh        float64   `json:"total_kwh"`
	AverageWatts    float64   `json:"average_watts"`
	PeakWatts       float64   `json:"peak_watts"`
	TotalCost       float64   `json:"total_cost"`
	Currency        string    `json:"currency"`
	CostPerKWh      float64   `json:"cost_per_kwh"`
	PeakCost        float64   `json:"peak_cost"`
	OffPeakCost     float64   `json:"off_peak_cost"`
	CarbonFootprint float64   `json:"carbon_footprint"` // kg CO2
	PeriodStart     time.Time `json:"period_start"`
	PeriodEnd       time.Time `json:"period_end"`
	CreatedAt       time.Time `json:"created_at"`
}

// SavingsPlan 节能计划
type SavingsPlan struct {
	ID                  string      `json:"id"`
	Name                string      `json:"name"`
	Description         string      `json:"description,omitempty"`
	Type                SavingsType `json:"type"`
	TargetDevice        string      `json:"target_device,omitempty"`
	EstimatedSaving     float64     `json:"estimated_saving"` // kWh/月
	EstimatedCostSaving float64     `json:"estimated_cost_saving"`
	Currency            string      `json:"currency"`
	Schedule            *Schedule   `json:"schedule,omitempty"`
	IsActive            bool        `json:"is_active"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
}

// Schedule 调度配置
type Schedule struct {
	Enabled    bool     `json:"enabled"`
	StartTime  string   `json:"start_time"` // HH:MM
	EndTime    string   `json:"end_time"`   // HH:MM
	DaysOfWeek []string `json:"days_of_week"`
	Action     string   `json:"action"` // power_off, reduce_power, standby
}

// PowerAlert 功率告警
type PowerAlert struct {
	ID             string     `json:"id"`
	Level          AlertLevel `json:"level"`
	Type           AlertType  `json:"type"`
	Title          string     `json:"title"`
	Message        string     `json:"message"`
	DeviceID       string     `json:"device_id,omitempty"`
	CurrentWatts   float64    `json:"current_watts"`
	ThresholdWatts float64    `json:"threshold_watts"`
	BudgetID       string     `json:"budget_id,omitempty"`
	IsRead         bool       `json:"is_read"`
	CreatedAt      time.Time  `json:"created_at"`
}

// PowerBudgetConfig 功率预算配置
type PowerBudgetConfig struct {
	Enabled            bool    `json:"enabled"`
	DefaultCurrency    string  `json:"default_currency"`
	ElectricityRate    float64 `json:"electricity_rate"`     // CNY/kWh
	PeakRate           float64 `json:"peak_rate"`            // 峰时电价
	OffPeakRate        float64 `json:"off_peak_rate"`        // 谷时电价
	CarbonFactor       float64 `json:"carbon_factor"`        // kg CO2/kWh
	AlertCheckInterval int     `json:"alert_check_interval"` // 秒
	MaxAlerts          int     `json:"max_alerts"`
}

// DefaultPowerBudgetConfig 默认配置
func DefaultPowerBudgetConfig() *PowerBudgetConfig {
	return &PowerBudgetConfig{
		Enabled:            true,
		DefaultCurrency:    "CNY",
		ElectricityRate:    0.55,
		PeakRate:           0.85,
		OffPeakRate:        0.35,
		CarbonFactor:       0.785,
		AlertCheckInterval: 60,
		MaxAlerts:          1000,
	}
}
