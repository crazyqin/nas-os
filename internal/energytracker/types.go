// Package energytracker 提供碳排放追踪与能源成本分析功能
package energytracker

import (
	"time"
)

// EnergyReading 能耗读数.
type EnergyReading struct {
	ID         string    `json:"id"`
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	PowerWatts float64   `json:"power_watts"` // 功耗瓦特
	Timestamp  time.Time `json:"timestamp"`
	Service    string    `json:"service,omitempty"` // 关联服务
}

// CarbonFootprint 碳排放数据.
type CarbonFootprint struct {
	DeviceID     string    `json:"device_id"`
	DeviceName   string    `json:"device_name"`
	EnergyKWh    float64   `json:"energy_kwh"`    // 能耗千瓦时
	CarbonKg     float64   `json:"carbon_kg"`     // 碳排放千克
	CarbonFactor float64   `json:"carbon_factor"` // 碳排放因子 kgCO2/kWh
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`
}

// EnergyReport 能源报告.
type EnergyReport struct {
	ID               string            `json:"id"`
	Period           ReportPeriod      `json:"period"`
	StartTime        time.Time         `json:"start_time"`
	EndTime          time.Time         `json:"end_time"`
	TotalEnergyKWh   float64           `json:"total_energy_kwh"`
	TotalCarbonKg    float64           `json:"total_carbon_kg"`
	TotalCostCents   int64             `json:"total_cost_cents"` // 费用分
	DeviceBreakdown  []DeviceEnergy    `json:"device_breakdown"`
	ServiceBreakdown []ServiceEnergy   `json:"service_breakdown"`
	HourlyTrend      []HourlyEnergy    `json:"hourly_trend"`
	OptimizationTips []OptimizationTip `json:"optimization_tips"`
	GeneratedAt      time.Time         `json:"generated_at"`
}

// DeviceEnergy 设备能耗明细.
type DeviceEnergy struct {
	DeviceID      string  `json:"device_id"`
	DeviceName    string  `json:"device_name"`
	EnergyKWh     float64 `json:"energy_kwh"`
	CarbonKg      float64 `json:"carbon_kg"`
	CostCents     int64   `json:"cost_cents"`
	Percentage    float64 `json:"percentage"` // 占比
	AvgPowerWatts float64 `json:"avg_power_watts"`
}

// ServiceEnergy 服务能耗明细.
type ServiceEnergy struct {
	ServiceName string  `json:"service_name"`
	EnergyKWh   float64 `json:"energy_kwh"`
	CarbonKg    float64 `json:"carbon_kg"`
	CostCents   int64   `json:"cost_cents"`
	Percentage  float64 `json:"percentage"`
	DeviceCount int     `json:"device_count"`
}

// HourlyEnergy 小时能耗趋势.
type HourlyEnergy struct {
	Hour      int     `json:"hour"` // 0-23
	EnergyKWh float64 `json:"energy_kwh"`
	AvgPower  float64 `json:"avg_power_watts"`
}

// OptimizationTip 节能建议.
type OptimizationTip struct {
	Category     string  `json:"category"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	SavingsKWh   float64 `json:"savings_kwh"`
	SavingsCents int64   `json:"savings_cents"`
	Priority     string  `json:"priority"` // high, medium, low
}

// PowerConfig 功耗配置.
type PowerConfig struct {
	CarbonFactor     float64 `json:"carbon_factor"`         // 碳排放因子 kgCO2/kWh
	PricePerKWhCents int64   `json:"price_per_kwh_cents"`   // 电价 分/kWh
	SamplingInterval int     `json:"sampling_interval_sec"` // 采样间隔秒
	IdleThreshold    float64 `json:"idle_threshold_watts"`  // 空闲阈值瓦特
}

// ReportPeriod 报告周期.
type ReportPeriod string

const (
	PeriodDaily   ReportPeriod = "daily"
	PeriodWeekly  ReportPeriod = "weekly"
	PeriodMonthly ReportPeriod = "monthly"
)

// StatsRequest 统计请求.
type StatsRequest struct {
	DeviceID string `json:"device_id,omitempty"`
	Service  string `json:"service,omitempty"`
}

// ReportRequest 报告请求.
type ReportRequest struct {
	Period    ReportPeriod `json:"period" binding:"required"`
	StartTime *time.Time   `json:"start_time,omitempty"`
	EndTime   *time.Time   `json:"end_time,omitempty"`
}

// TrackRequest 追踪请求.
type TrackRequest struct {
	DeviceID   string  `json:"device_id" binding:"required"`
	DeviceName string  `json:"device_name" binding:"required"`
	PowerWatts float64 `json:"power_watts" binding:"required,min=0"`
	Service    string  `json:"service,omitempty"`
}
