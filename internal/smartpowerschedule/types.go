// Package smartpowerschedule 提供智能电源调度功能，包括 UPS 管理、电池监控、峰谷电价调度。
package smartpowerschedule

import "time"

// PowerSource 电源类型.
type PowerSource string

const (
	PowerSourceGrid    PowerSource = "grid"    // 电网
	PowerSourceUPS     PowerSource = "ups"     // UPS
	PowerSourceSolar   PowerSource = "solar"   // 太阳能
	PowerSourceBattery PowerSource = "battery" // 电池
)

// PowerStatus 电源状态.
type PowerStatus string

const (
	PowerStatusNormal   PowerStatus = "normal"   // 正常
	PowerStatusWarning  PowerStatus = "warning"  // 警告
	PowerStatusCritical PowerStatus = "critical" // 危急
	PowerStatusOffline  PowerStatus = "offline"  // 离线
)

// BatteryStatus 电池状态.
type BatteryStatus string

const (
	BatteryCharging    BatteryStatus = "charging"
	BatteryDischarging BatteryStatus = "discharging"
	BatteryFull        BatteryStatus = "full"
	BatteryLow         BatteryStatus = "low"
	BatteryCritical    BatteryStatus = "critical"
	BatteryError       BatteryStatus = "error"
)

// TimeOfUsePeriod 峰谷时段.
type TimeOfUsePeriod string

const (
	TOUPeak     TimeOfUsePeriod = "peak"      // 高峰
	TOUOffPeak  TimeOfUsePeriod = "off_peak"  // 低谷
	TOUShoulder TimeOfUsePeriod = "shoulder"  // 平段
	TOUSuperOff TimeOfUsePeriod = "super_off" // 超级低谷
)

// ScheduleAction 调度动作.
type ScheduleAction string

const (
	ActionPowerOn          ScheduleAction = "power_on"
	ActionPowerOff         ScheduleAction = "power_off"
	ActionReducePower      ScheduleAction = "reduce_power"
	ActionSwitchSource     ScheduleAction = "switch_source"
	ActionChargeBattery    ScheduleAction = "charge_battery"
	ActionDischargeBattery ScheduleAction = "discharge_battery"
	ActionNotify           ScheduleAction = "notify"
)

// DevicePowerState 设备电源状态.
type DevicePowerState struct {
	DeviceID    string      `json:"device_id"`
	DeviceName  string      `json:"device_name"`
	IsPoweredOn bool        `json:"is_powered_on"`
	PowerUsageW float64     `json:"power_usage_w"`
	MaxPowerW   float64     `json:"max_power_w"`
	PowerSource PowerSource `json:"power_source"`
	LastChanged time.Time   `json:"last_changed"`
}

// UPSInfo UPS 信息.
type UPSInfo struct {
	ID                  string        `json:"id"`
	Name                string        `json:"name"`
	Model               string        `json:"model"`
	Status              PowerStatus   `json:"status"`
	BatteryLevel        float64       `json:"battery_level"` // 0-100
	BatteryStatus       BatteryStatus `json:"battery_status"`
	BatteryHealth       float64       `json:"battery_health"` // 0-100
	BatteryCycles       int           `json:"battery_cycles"`
	BatteryTempC        float64       `json:"battery_temp_c"`
	InputVoltageV       float64       `json:"input_voltage_v"`
	OutputVoltageV      float64       `json:"output_voltage_v"`
	LoadPercent         float64       `json:"load_percent"` // 0-100
	PowerCapacityW      float64       `json:"power_capacity_w"`
	EstimatedRuntimeMin float64       `json:"estimated_runtime_min"`
	LastTestTime        time.Time     `json:"last_test_time"`
	LastTestResult      string        `json:"last_test_result"`
	FirmwareVersion     string        `json:"firmware_version"`
	ConnectedDevices    []string      `json:"connected_devices"`
	LastUpdated         time.Time     `json:"last_updated"`
}

// PowerUsageRecord 用电记录.
type PowerUsageRecord struct {
	ID          string          `json:"id"`
	DeviceID    string          `json:"device_id"`
	DeviceName  string          `json:"device_name"`
	PowerUsageW float64         `json:"power_usage_w"`
	Duration    time.Duration   `json:"duration"`
	EnergyWh    float64         `json:"energy_wh"`   // 瓦时
	CostAmount  float64         `json:"cost_amount"` // 费用
	TimePeriod  TimeOfUsePeriod `json:"time_period"`
	RecordedAt  time.Time       `json:"recorded_at"`
}

// PowerSchedule 电源调度计划.
type PowerSchedule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	IsActive    bool            `json:"is_active"`
	DeviceIDs   []string        `json:"device_ids"`
	Action      ScheduleAction  `json:"action"`
	TimePeriod  TimeOfUsePeriod `json:"time_period"`
	StartTime   string          `json:"start_time"`   // HH:MM
	EndTime     string          `json:"end_time"`     // HH:MM
	DaysOfWeek  []int           `json:"days_of_week"` // 0=Sunday, 1=Monday, ...
	Priority    int             `json:"priority"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// PowerScheduleRequest 调度计划请求.
type PowerScheduleRequest struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description,omitempty"`
	DeviceIDs   []string        `json:"device_ids" binding:"required"`
	Action      ScheduleAction  `json:"action" binding:"required"`
	TimePeriod  TimeOfUsePeriod `json:"time_period,omitempty"`
	StartTime   string          `json:"start_time" binding:"required"`
	EndTime     string          `json:"end_time" binding:"required"`
	DaysOfWeek  []int           `json:"days_of_week"`
	Priority    int             `json:"priority,omitempty"`
}

// TOUConfig 峰谷电价配置.
type TOUConfig struct {
	Enabled      bool              `json:"enabled"`
	Currency     string            `json:"currency"`
	PeakRate     float64           `json:"peak_rate"`      // 高峰电价
	OffPeakRate  float64           `json:"off_peak_rate"`  // 低谷电价
	ShoulderRate float64           `json:"shoulder_rate"`  // 平段电价
	SuperOffRate float64           `json:"super_off_rate"` // 超低谷电价
	Periods      []TOUPeriodConfig `json:"periods"`
}

// TOUPeriodConfig 峰谷时段配置.
type TOUPeriodConfig struct {
	Period    TimeOfUsePeriod `json:"period"`
	StartTime string          `json:"start_time"` // HH:MM
	EndTime   string          `json:"end_time"`   // HH:MM
	Rate      float64         `json:"rate"`
}

// PowerConfig 电源调度配置.
type PowerConfig struct {
	Enabled                  bool       `json:"enabled"`
	UPSMonitorEnabled        bool       `json:"ups_monitor_enabled"`
	UPSMonitorIntervalSec    int        `json:"ups_monitor_interval_sec"`
	BatteryLowThreshold      float64    `json:"battery_low_threshold"` // 0-100
	BatteryCriticalThreshold float64    `json:"battery_critical_threshold"`
	AutoSwitchOnPowerFail    bool       `json:"auto_switch_on_power_fail"`
	ShutdownOnBatteryLow     bool       `json:"shutdown_on_battery_low"`
	ShutdownDelayMin         int        `json:"shutdown_delay_min"`
	TOUConfig                *TOUConfig `json:"tou_config,omitempty"`
	MaxPowerBudgetW          float64    `json:"max_power_budget_w"`
	PeakShavingEnabled       bool       `json:"peak_shaving_enabled"`
	PeakShavingTargetW       float64    `json:"peak_shaving_target_w"`
}

// DefaultPowerConfig 默认电源配置.
func DefaultPowerConfig() *PowerConfig {
	return &PowerConfig{
		Enabled:                  true,
		UPSMonitorEnabled:        true,
		UPSMonitorIntervalSec:    30,
		BatteryLowThreshold:      20.0,
		BatteryCriticalThreshold: 10.0,
		AutoSwitchOnPowerFail:    true,
		ShutdownOnBatteryLow:     true,
		ShutdownDelayMin:         5,
		MaxPowerBudgetW:          2000.0,
		PeakShavingEnabled:       false,
		PeakShavingTargetW:       1500.0,
		TOUConfig: &TOUConfig{
			Enabled:      true,
			Currency:     "CNY",
			PeakRate:     1.2,
			OffPeakRate:  0.4,
			ShoulderRate: 0.8,
			SuperOffRate: 0.2,
			Periods: []TOUPeriodConfig{
				{Period: TOUPeak, StartTime: "08:00", EndTime: "11:00", Rate: 1.2},
				{Period: TOUPeak, StartTime: "18:00", EndTime: "23:00", Rate: 1.2},
				{Period: TOUShoulder, StartTime: "06:00", EndTime: "08:00", Rate: 0.8},
				{Period: TOUShoulder, StartTime: "11:00", EndTime: "18:00", Rate: 0.8},
				{Period: TOUOffPeak, StartTime: "23:00", EndTime: "06:00", Rate: 0.4},
			},
		},
	}
}

// PowerEvent 电源事件.
type PowerEvent struct {
	ID           string      `json:"id"`
	EventType    string      `json:"event_type"` // power_outage, power_restore, battery_low, ups_switch, etc.
	Source       PowerSource `json:"source"`
	Message      string      `json:"message"`
	Severity     PowerStatus `json:"severity"`
	BatteryLevel float64     `json:"battery_level,omitempty"`
	Timestamp    time.Time   `json:"timestamp"`
}

// LoadPrediction 负载预测.
type LoadPrediction struct {
	ID             string        `json:"id"`
	PredictedLoadW float64       `json:"predicted_load_w"`
	Confidence     float64       `json:"confidence"` // 0-1
	TimeAhead      time.Duration `json:"time_ahead"`
	PredictedAt    time.Time     `json:"predicted_at"`
	ValidUntil     time.Time     `json:"valid_until"`
}

// CostSummary 费用汇总.
type CostSummary struct {
	Period         string                      `json:"period"`
	TotalEnergyKWh float64                     `json:"total_energy_kwh"`
	TotalCost      float64                     `json:"total_cost"`
	Currency       string                      `json:"currency"`
	ByPeriod       map[TimeOfUsePeriod]float64 `json:"by_period"`
	ByDevice       map[string]float64          `json:"by_device"`
	SavingsEst     float64                     `json:"savings_est"` // 预估节省
}
