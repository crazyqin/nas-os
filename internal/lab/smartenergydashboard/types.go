// Package smartenergydashboard 提供智能能源仪表盘功能，用于监控和管理 NAS 系统的能源消耗。
// 提供实时功耗监控、历史记录查询、设备能耗分析、预算管理、成本预测和节能建议等功能。
package smartenergydashboard

import "time"

// PowerReading 功耗读数.
type PowerReading struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Wattage   float64   `json:"wattage"`
	Voltage   float64   `json:"voltage"`
	Current   float64   `json:"current"`
	Source    string    `json:"source"` // system/disk/cpu/fan/psu
}

// EnergyRecord 能耗记录.
type EnergyRecord struct {
	ID           string    `json:"id"`
	Date         time.Time `json:"date"`
	KWh          float64   `json:"kwh"`
	Cost         float64   `json:"cost"`
	CarbonKg     float64   `json:"carbon_kg"`
	PeakWattage  float64   `json:"peak_wattage"`
	AvgWattage   float64   `json:"avg_wattage"`
	RuntimeHours float64   `json:"runtime_hours"`
}

// EnergyBudget 能源预算.
type EnergyBudget struct {
	ID               string    `json:"id"`
	MonthlyLimitKWh  float64   `json:"monthly_limit_kwh"`
	MonthlyLimitCost float64   `json:"monthly_limit_cost"`
	AlertThreshold   float64   `json:"alert_threshold"` // 百分比 (0-100)
	CurrentUsage     float64   `json:"current_usage"`
	ProjectedUsage   float64   `json:"projected_usage"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// DevicePower 设备功耗.
type DevicePower struct {
	DeviceID    string  `json:"device_id"`
	DeviceName  string  `json:"device_name"`
	DeviceType  string  `json:"device_type"` // hdd/ssd/cpu/fan/psu/nic
	CurrentWatt float64 `json:"current_watt"`
	DailyKWh    float64 `json:"daily_kwh"`
	MonthlyKWh  float64 `json:"monthly_kwh"`
	Status      string  `json:"status"` // online/offline/standby
}

// EnergyReport 能源报告.
type EnergyReport struct {
	ID          string        `json:"id"`
	Period      string        `json:"period"` // daily/weekly/monthly
	StartDate   time.Time     `json:"start_date"`
	EndDate     time.Time     `json:"end_date"`
	TotalKWh    float64       `json:"total_kwh"`
	TotalCost   float64       `json:"total_cost"`
	CarbonKg    float64       `json:"carbon_kg"`
	TopDevices  []DevicePower `json:"top_devices"`
	Trend       string        `json:"trend"` // up/down/stable
	SavingsTips []string      `json:"savings_tips"`
}

// CostForecast 成本预测.
type CostForecast struct {
	Month         string   `json:"month"`
	ProjectedKWh  float64  `json:"projected_kwh"`
	ProjectedCost float64  `json:"projected_cost"`
	Confidence    float64  `json:"confidence"` // 0-100
	Factors       []string `json:"factors"`
}

// EnergySettings 能源设置.
type EnergySettings struct {
	ElectricityRate   float64   `json:"electricity_rate"` // 元/kWh
	CarbonFactor      float64   `json:"carbon_factor"`    // kg CO2/kWh
	Currency          string    `json:"currency"`         // 货币单位
	MonitoringEnabled bool      `json:"monitoring_enabled"`
	AlertEnabled      bool      `json:"alert_enabled"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// DefaultEnergySettings 默认能源设置.
func DefaultEnergySettings() *EnergySettings {
	return &EnergySettings{
		ElectricityRate:   0.56,  // 元/kWh
		CarbonFactor:      0.785, // kg CO2/kWh (中国电网平均)
		Currency:          "CNY",
		MonitoringEnabled: true,
		AlertEnabled:      true,
		UpdatedAt:         time.Now(),
	}
}

// EnergyTip 节能建议.
type EnergyTip struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Category    string  `json:"category"` // hardware/software/lifestyle
	Impact      string  `json:"impact"`   // low/medium/high
	SavingsKWh  float64 `json:"savings_kwh"`
	SavingsCost float64 `json:"savings_cost"`
}
