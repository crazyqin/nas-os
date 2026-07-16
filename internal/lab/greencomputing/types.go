package greencomputing

import (
	"time"
)

// EnergyReading represents a real-time energy reading.
type EnergyReading struct {
	Timestamp    time.Time `json:"timestamp"`
	TotalWatts   float64   `json:"total_watts"`
	CPUWatts     float64   `json:"cpu_watts"`
	DiskWatts    float64   `json:"disk_watts"`
	NetworkWatts float64   `json:"network_watts"`
	FanWatts     float64   `json:"fan_watts"`
	OtherWatts   float64   `json:"other_watts"`
	Source       string    `json:"source"`
}

// CarbonFootprint represents carbon emission data.
type CarbonFootprint struct {
	Period      string             `json:"period"`
	EnergyKWh   float64            `json:"energy_kwh"`
	CarbonKg    float64            `json:"carbon_kg"`
	CarbonG     float64            `json:"carbon_g"`
	OffsetCost  float64            `json:"offset_cost"`
	TreesNeeded float64            `json:"trees_needed"`
	Breakdown   map[string]float64 `json:"breakdown"`
}

// SleepStrategy represents an intelligent sleep strategy.
type SleepStrategy struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	IdleThreshold  time.Duration `json:"idle_threshold"`
	DiskSpindown   bool          `json:"disk_spindown"`
	CPUGovernor    string        `json:"cpu_governor"`
	LEDEnabled     bool          `json:"led_enabled"`
	WakeOnLAN      bool          `json:"wake_on_lan"`
	ScheduledSleep string        `json:"scheduled_sleep"`
	ScheduledWake  string        `json:"scheduled_wake"`
	IsActive       bool          `json:"is_active"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// EfficiencyReport represents an energy efficiency report.
type EfficiencyReport struct {
	Period          string            `json:"period"`
	GeneratedAt     time.Time         `json:"generated_at"`
	TotalEnergyKWh  float64           `json:"total_energy_kwh"`
	AvgPowerWatts   float64           `json:"avg_power_watts"`
	PeakPowerWatts  float64           `json:"peak_power_watts"`
	MinPowerWatts   float64           `json:"min_power_watts"`
	EfficiencyScore float64           `json:"efficiency_score"`
	CarbonKg        float64           `json:"carbon_kg"`
	CostEstimate    float64           `json:"cost_estimate"`
	Recommendations []*Recommendation `json:"recommendations"`
	Trends          *EfficiencyTrends `json:"trends"`
}

// Recommendation represents an optimization recommendation.
type Recommendation struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	Priority     int       `json:"priority"`
	SavingsKWh   float64   `json:"savings_kwh"`
	SavingsKg    float64   `json:"savings_kg"`
	SavingsCost  float64   `json:"savings_cost"`
	EstimatedROI string    `json:"estimated_roi"`
	CreatedAt    time.Time `json:"created_at"`
}

// EfficiencyTrends represents efficiency trends.
type EfficiencyTrends struct {
	EnergyTrend     string  `json:"energy_trend"`
	CarbonTrend     string  `json:"carbon_trend"`
	EfficiencyTrend string  `json:"efficiency_trend"`
	WeekOverWeek    float64 `json:"week_over_week"`
	MonthOverMonth  float64 `json:"month_over_month"`
}

// GreenScore represents the overall green computing score.
type GreenScore struct {
	Score     float64            `json:"score"`
	Grade     string             `json:"grade"`
	Breakdown map[string]float64 `json:"breakdown"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// EnergySource constants.
const (
	SourceGrid  = "grid"
	SourceSolar = "solar"
	SourceWind  = "wind"
)

// Carbon intensity constants (gCO2/kWh).
const (
	DefaultGridIntensity  = 400.0
	DefaultSolarIntensity = 50.0
	DefaultWindIntensity  = 10.0
)

// Carbon offset cost (USD per kg CO2).
const DefaultCarbonOffsetCost = 0.02

// Tree absorption rate (kg CO2 per tree per year).
const DefaultTreeAbsorptionKg = 22.0
