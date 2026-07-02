package energymanager

import (
	"time"
)

// PowerProfile represents a power management profile.
type PowerProfile struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Type        string        `json:"type"`
	MaxWatts    float64       `json:"max_watts"`
	IdleWatts   float64       `json:"idle_watts"`
	SpinDownHDD bool          `json:"spin_down_hdd"`
	FanCurve    string        `json:"fan_curve"`
	Settings    PowerSettings `json:"settings"`
	LEDControl  bool          `json:"led_control,omitempty"`
	IsActive    bool          `json:"is_active"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// PowerSettings represents power management settings.
type PowerSettings struct {
	HDDSpindown       int    `json:"hdd_spindown_minutes"`
	LEDControl        bool   `json:"led_control"`
	FanMode           string `json:"fan_mode"`
	WakeOnLAN         bool   `json:"wake_on_lan"`
	ScheduledPowerOn  string `json:"scheduled_power_on"`
	ScheduledPowerOff string `json:"scheduled_power_off"`
	CPUGovernor       string `json:"cpu_governor"`
	IdleThreshold     int    `json:"idle_threshold_minutes"`
}

// PowerUsage represents current power usage.
type PowerUsage struct {
	Timestamp   time.Time `json:"timestamp"`
	TotalWatts  float64   `json:"total_watts"`
	CPUWatts    float64   `json:"cpu_watts"`
	DiskWatts   float64   `json:"disk_watts"`
	FanWatts    float64   `json:"fan_watts"`
	OtherWatts  float64   `json:"other_watts"`
	Temperature float64   `json:"temperature"`
	CPUTemp     float64   `json:"cpu_temp"`
	DiskTemp    float64   `json:"disk_temp"`
}

// PowerHistory represents power usage history.
type PowerHistory struct {
	Period       string       `json:"period"`
	DataPoints   []PowerUsage `json:"data_points"`
	AvgWatts     float64      `json:"avg_watts"`
	MaxWatts     float64      `json:"max_watts"`
	MinWatts     float64      `json:"min_watts"`
	TotalKWh     float64      `json:"total_kwh"`
	CostEstimate float64      `json:"cost_estimate"`
}

// DiskSleepConfig represents disk sleep configuration.
type DiskSleepConfig struct {
	DiskID       string    `json:"disk_id"`
	DiskName     string    `json:"disk_name"`
	SpindownMin  int       `json:"spindown_minutes"`
	CurrentState string    `json:"current_state"`
	LastActive   time.Time `json:"last_active"`
}

// FanConfig represents fan configuration.
type FanConfig struct {
	FanID       string          `json:"fan_id"`
	Name        string          `json:"name"`
	Mode        string          `json:"mode"`
	CurrentRPM  int             `json:"current_rpm"`
	TargetRPM   int             `json:"target_rpm"`
	Temperature float64         `json:"temperature"`
	Curve       []FanCurvePoint `json:"curve"`
}

// FanCurvePoint represents a fan curve point.
type FanCurvePoint struct {
	Temperature float64 `json:"temperature"`
	Percentage  int     `json:"percentage"`
}

// ScheduleEntry represents a power schedule entry.
type ScheduleEntry struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Time      string    `json:"time"`
	Days      []string  `json:"days"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// EnergyStats represents energy consumption statistics.
type EnergyStats struct {
	TodayKWh     float64        `json:"today_kwh"`
	WeekKWh      float64        `json:"week_kwh"`
	MonthKWh     float64        `json:"month_kwh"`
	YearKWh      float64        `json:"year_kwh"`
	TodayCost    float64        `json:"today_cost"`
	WeekCost     float64        `json:"week_cost"`
	MonthCost    float64        `json:"month_cost"`
	YearCost     float64        `json:"year_cost"`
	CostPerKWh   float64        `json:"cost_per_kwh"`
	AvgDailyKWh  float64        `json:"avg_daily_kwh"`
	CurrentWatts float64        `json:"current_watts"`
	PeakWatts    float64        `json:"peak_watts"`
	PowerState   string         `json:"power_state"`
	History      []PowerHistory `json:"history"`
}

// TemperatureAlert represents a temperature alert.
type TemperatureAlert struct {
	ID           string    `json:"id"`
	Component    string    `json:"component"`
	Temperature  float64   `json:"temperature"`
	Threshold    float64   `json:"threshold"`
	Severity     string    `json:"severity"`
	Timestamp    time.Time `json:"timestamp"`
	Acknowledged bool      `json:"acknowledged"`
}

// PowerReading represents a power reading from a source.
type PowerReading struct {
	Timestamp time.Time `json:"timestamp"`
	Watts     float64   `json:"watts"`
	Voltage   float64   `json:"voltage"`
	Current   float64   `json:"current"`
	Source    string    `json:"source"`
}

// Power source constants.
const (
	SourceGrid  = "grid"
	SourceSolar = "solar"
	SourceUPS   = "ups"
)

// Power state constants.
const (
	PowerNormal  = "normal"
	PowerStandby = "standby"
	PowerSleep   = "sleep"
	PowerOff     = "off"
)

// CarbonMetrics represents carbon emission metrics.
type CarbonMetrics struct {
	GridCarbonFactor float64 `json:"grid_carbon_factor"`
	DailyCO2Kg       float64 `json:"daily_co2_kg"`
	MonthlyCO2Kg     float64 `json:"monthly_co2_kg"`
	YearlyCO2Kg      float64 `json:"yearly_co2_kg"`
}

// PowerBudget represents a power budget estimation.
type PowerBudget struct {
	CostPerKWh  float64 `json:"cost_per_kwh"`
	Currency    string  `json:"currency"`
	DailyCost   float64 `json:"daily_cost"`
	MonthlyCost float64 `json:"monthly_cost"`
	YearlyCost  float64 `json:"yearly_cost"`
}
