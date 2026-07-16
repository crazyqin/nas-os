// Package energydashboard 提供能耗监控与电力优化仪表盘功能。
// 包含实时功耗监控、电力成本计算、能效评分、节能策略调度、历史能耗报表和碳排放估算。
package energydashboard

import "time"

// ComponentType 硬件组件类型.
type ComponentType string

const (
	ComponentCPU    ComponentType = "cpu"
	ComponentDisk   ComponentType = "disk"
	ComponentFan    ComponentType = "fan"
	ComponentSystem ComponentType = "system"
)

// PowerState 电源状态.
type PowerState string

const (
	PowerStateActive  PowerState = "active"
	PowerStateIdle    PowerState = "idle"
	PowerStateStandby PowerState = "standby"
	PowerStateSleep   PowerState = "sleep"
	PowerStateOff     PowerState = "off"
)

// EnergyReportPeriod 能耗报表周期.
type EnergyReportPeriod string

const (
	PeriodDaily   EnergyReportPeriod = "daily"
	PeriodWeekly  EnergyReportPeriod = "weekly"
	PeriodMonthly EnergyReportPeriod = "monthly"
	PeriodYearly  EnergyReportPeriod = "yearly"
)

// SleepPolicy 休眠策略类型.
type SleepPolicy string

const (
	SleepPolicyScheduled  SleepPolicy = "scheduled"
	SleepPolicyIdle       SleepPolicy = "idle"
	SleepPolicyPowerSaver SleepPolicy = "power_saver"
)

// PowerReading 功耗读数.
type PowerReading struct {
	ID          string        `json:"id"`
	Component   ComponentType `json:"component"`
	DeviceName  string        `json:"device_name"`
	PowerWatts  float64       `json:"power_watts"`
	Temperature float64       `json:"temperature,omitempty"`
	State       PowerState    `json:"state"`
	Timestamp   time.Time     `json:"timestamp"`
}

// SystemPowerSnapshot 系统功耗快照.
type SystemPowerSnapshot struct {
	ID             string         `json:"id"`
	CPUPower       float64        `json:"cpu_power"`
	DiskPower      float64        `json:"disk_power"`
	FanPower       float64        `json:"fan_power"`
	TotalPower     float64        `json:"total_power"`
	CPUTemperature float64        `json:"cpu_temperature"`
	Components     []PowerReading `json:"components"`
	Timestamp      time.Time      `json:"timestamp"`
}

// ElectricityRate 电价配置.
type ElectricityRate struct {
	ID           string     `json:"id"`
	Region       string     `json:"region" binding:"required"`
	Currency     string     `json:"currency"`
	ProviderName string     `json:"provider_name,omitempty"`
	Rates        []RateTier `json:"rates" binding:"required,min=1"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// RateTier 分时电价阶梯.
type RateTier struct {
	Name      string  `json:"name"`
	StartTime string  `json:"start_time"` // HH:MM 格式
	EndTime   string  `json:"end_time"`   // HH:MM 格式
	PriceKWh  float64 `json:"price_kwh"`  // 每度电价格
}

// EnergyCost 能耗费用.
type EnergyCost struct {
	ID        string             `json:"id"`
	Region    string             `json:"region"`
	Period    EnergyReportPeriod `json:"period"`
	StartDate time.Time          `json:"start_date"`
	EndDate   time.Time          `json:"end_date"`
	KWh       float64            `json:"kwh"`
	Cost      float64            `json:"cost"`
	Currency  string             `json:"currency"`
	Breakdown []CostBreakdown    `json:"breakdown,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
}

// CostBreakdown 费用细分.
type CostBreakdown struct {
	Component  ComponentType `json:"component"`
	KWh        float64       `json:"kwh"`
	Cost       float64       `json:"cost"`
	Percentage float64       `json:"percentage"`
}

// EfficiencyScore 能效评分.
type EfficiencyScore struct {
	ID              string    `json:"id"`
	TotalStorageTB  float64   `json:"total_storage_tb"`
	TotalPowerWatts float64   `json:"total_power_watts"`
	WattsPerTB      float64   `json:"watts_per_tb"`
	Score           int       `json:"score"`  // 0-100
	Rating          string    `json:"rating"` // A+, A, B, C, D
	Recommendations []string  `json:"recommendations,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SleepSchedule 休眠计划.
type SleepSchedule struct {
	ID           string      `json:"id"`
	Name         string      `json:"name" binding:"required"`
	Policy       SleepPolicy `json:"policy" binding:"required"`
	TargetDevice string      `json:"target_device"`
	StartTime    string      `json:"start_time,omitempty"` // HH:MM
	EndTime      string      `json:"end_time,omitempty"`   // HH:MM
	IdleMinutes  int         `json:"idle_minutes,omitempty"`
	Enabled      bool        `json:"enabled"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// EnergyReport 能耗报表.
type EnergyReport struct {
	ID              string             `json:"id"`
	Period          EnergyReportPeriod `json:"period"`
	StartDate       time.Time          `json:"start_date"`
	EndDate         time.Time          `json:"end_date"`
	TotalKWh        float64            `json:"total_kwh"`
	TotalCost       float64            `json:"total_cost"`
	Currency        string             `json:"currency"`
	CarbonKg        float64            `json:"carbon_kg"`
	Components      []ComponentReport  `json:"components"`
	DailyAverage    float64            `json:"daily_average"`
	PeakPower       float64            `json:"peak_power"`
	PeakTime        time.Time          `json:"peak_time"`
	EfficiencyScore int                `json:"efficiency_score"`
	CreatedAt       time.Time          `json:"created_at"`
}

// ComponentReport 组件能耗报表.
type ComponentReport struct {
	Component   ComponentType `json:"component"`
	DeviceName  string        `json:"device_name"`
	TotalKWh    float64       `json:"total_kwh"`
	AvgPower    float64       `json:"avg_power"`
	MaxPower    float64       `json:"max_power"`
	UptimeHours float64       `json:"uptime_hours"`
}

// CarbonEstimate 碳排放估算.
type CarbonEstimate struct {
	ID             string             `json:"id"`
	KWh            float64            `json:"kwh"`
	CarbonKg       float64            `json:"carbon_kg"`
	Factor         float64            `json:"factor"` // kg CO2 per kWh
	Region         string             `json:"region"`
	Period         EnergyReportPeriod `json:"period"`
	StartDate      time.Time          `json:"start_date"`
	EndDate        time.Time          `json:"end_date"`
	EquivalentTree float64            `json:"equivalent_tree"` // 等效种树量
	CreatedAt      time.Time          `json:"created_at"`
}

// DashboardSummary 仪表盘总览.
type DashboardSummary struct {
	CurrentPower    *SystemPowerSnapshot `json:"current_power"`
	TodayKWh        float64              `json:"today_kwh"`
	TodayCost       float64              `json:"today_cost"`
	MonthKWh        float64              `json:"month_kwh"`
	MonthCost       float64              `json:"month_cost"`
	Currency        string               `json:"currency"`
	EfficiencyScore *EfficiencyScore     `json:"efficiency_score"`
	CarbonToday     float64              `json:"carbon_today"`
	ActiveSchedules int                  `json:"active_schedules"`
	LastUpdated     time.Time            `json:"last_updated"`
}

// DashboardConfig 能耗仪表盘配置.
type DashboardConfig struct {
	Enabled          bool    `json:"enabled"`
	MonitorInterval  int     `json:"monitor_interval"` // 采样间隔秒数
	Region           string  `json:"region"`
	Currency         string  `json:"currency"`
	CarbonFactor     float64 `json:"carbon_factor"` // kg CO2 per kWh
	TotalStorageTB   float64 `json:"total_storage_tb"`
	ReportRetention  int     `json:"report_retention"` // 保留天数
	EnableSleepSched bool    `json:"enable_sleep_sched"`
}

// DefaultDashboardConfig 默认配置.
func DefaultDashboardConfig() *DashboardConfig {
	return &DashboardConfig{
		Enabled:          true,
		MonitorInterval:  60,
		Region:           "cn-default",
		Currency:         "CNY",
		CarbonFactor:     0.5810, // 中国电网平均碳排放因子
		TotalStorageTB:   0,
		ReportRetention:  365,
		EnableSleepSched: true,
	}
}

// GetDefaultRates 获取默认电价（中国居民用电）.
func GetDefaultRates() []RateTier {
	return []RateTier{
		{Name: "峰时", StartTime: "08:00", EndTime: "22:00", PriceKWh: 0.56},
		{Name: "谷时", StartTime: "22:00", EndTime: "08:00", PriceKWh: 0.28},
	}
}

// SupportedPeriods 支持的报表周期.
func SupportedPeriods() []EnergyReportPeriod {
	return []EnergyReportPeriod{PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodYearly}
}

// IsValidPeriod 校验周期是否有效.
func IsValidPeriod(p EnergyReportPeriod) bool {
	for _, period := range SupportedPeriods() {
		if period == p {
			return true
		}
	}
	return false
}
