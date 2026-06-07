package smartenergyoptimizer

import (
	"time"
)

// EnergyReading represents a power consumption measurement
type EnergyReading struct {
	Timestamp    time.Time `json:"timestamp"`
	PowerWatts   float64   `json:"power_watts"`
	DeviceID     string    `json:"device_id"`
	DeviceType   string    `json:"device_type"`
	EnergyWh     float64   `json:"energy_wh"`
	TemperatureC float64   `json:"temperature_c,omitempty"`
}

// PowerForecast represents a predicted power consumption
type PowerForecast struct {
	Timestamp      time.Time `json:"timestamp"`
	ForecastWatts  float64   `json:"forecast_watts"`
	Confidence     float64   `json:"confidence"`
	ForecastMethod string    `json:"forecast_method"`
}

// CarbonIntensity represents carbon emission data for a time period
type CarbonIntensity struct {
	Timestamp        time.Time `json:"timestamp"`
	IntensityGPerKWh float64   `json:"intensity_g_per_kwh"`
	Region           string    `json:"region"`
	IsLowCarbon      bool      `json:"is_low_carbon"`
}

// CarbonAwareSchedule represents a scheduled task with carbon awareness
type CarbonAwareSchedule struct {
	TaskID         string    `json:"task_id"`
	TaskName       string    `json:"task_name"`
	ScheduledTime  time.Time `json:"scheduled_time"`
	EstimatedPower float64   `json:"estimated_power"`
	CarbonSaving   float64   `json:"carbon_saving"`
	Priority       int       `json:"priority"`
}

// DeviceState represents the state of a device (HDD/SSD)
type DeviceState struct {
	DeviceID       string    `json:"device_id"`
	DeviceType     string    `json:"device_type"`
	State          string    `json:"state"` // active, standby, sleeping
	LastAccessTime time.Time `json:"last_access_time"`
	IdleDuration   int64     `json:"idle_duration_seconds"`
	PowerUsage     float64   `json:"power_usage_watts"`
}

// SleepPolicy defines a device sleep policy
type SleepPolicy struct {
	PolicyID          string `json:"policy_id"`
	DeviceType        string `json:"device_type"`
	IdleTimeoutSec    int    `json:"idle_timeout_seconds"`
	MinSleepDuration  int    `json:"min_sleep_duration_seconds"`
	Enabled           bool   `json:"enabled"`
	SpindownThreshold int    `json:"spindown_threshold"` // access count per hour
}

// EnergyCost represents energy cost information
type EnergyCost struct {
	Period     string  `json:"period"`
	TotalKWh   float64 `json:"total_kwh"`
	CostPerKWh float64 `json:"cost_per_kwh"`
	TotalCost  float64 `json:"total_cost"`
	Currency   string  `json:"currency"`
	RateType   string  `json:"rate_type"` // peak, off-peak, flat
}

// TariffPlan represents an electricity tariff plan
type TariffPlan struct {
	PlanID       string      `json:"plan_id"`
	Name         string      `json:"name"`
	Currency     string      `json:"currency"`
	PeakRate     float64     `json:"peak_rate"`
	OffPeakRate  float64     `json:"off_peak_rate"`
	FlatRate     float64     `json:"flat_rate"`
	PeakHours    []TimeRange `json:"peak_hours"`
	OffPeakHours []TimeRange `json:"off_peak_hours"`
	IsFlatRate   bool        `json:"is_flat_rate"`
}

// TimeRange represents a time range
type TimeRange struct {
	Start string `json:"start"` // HH:MM format
	End   string `json:"end"`   // HH:MM format
}

// PowerBudget represents a power consumption budget
type PowerBudget struct {
	BudgetID       string    `json:"budget_id"`
	Name           string    `json:"name"`
	PeriodType     string    `json:"period_type"` // monthly, yearly
	TargetKWh      float64   `json:"target_kwh"`
	CurrentKWh     float64   `json:"current_kwh"`
	RemainingKWh   float64   `json:"remaining_kwh"`
	UsagePercent   float64   `json:"usage_percent"`
	StartDate      time.Time `json:"start_date"`
	EndDate        time.Time `json:"end_date"`
	AlertThreshold float64   `json:"alert_threshold"` // percentage
}

// EnergyReport represents an energy consumption report
type EnergyReport struct {
	ReportID        string             `json:"report_id"`
	ReportType      string             `json:"report_type"` // daily, weekly, monthly
	Period          string             `json:"period"`
	GeneratedAt     time.Time          `json:"generated_at"`
	TotalEnergyKWh  float64            `json:"total_energy_kwh"`
	TotalCost       float64            `json:"total_cost"`
	Currency        string             `json:"currency"`
	AveragePower    float64            `json:"average_power"`
	PeakPower       float64            `json:"peak_power"`
	DeviceBreakdown map[string]float64 `json:"device_breakdown"`
	Savings         *SavingsSummary    `json:"savings,omitempty"`
	Recommendations []string           `json:"recommendations,omitempty"`
}

// SavingsSummary represents energy savings summary
type SavingsSummary struct {
	EnergySavedKWh float64 `json:"energy_saved_kwh"`
	CostSaved      float64 `json:"cost_saved"`
	CarbonSavedKg  float64 `json:"carbon_saved_kg"`
	SavingsPercent float64 `json:"savings_percent"`
}

// MLModel represents a machine learning model for prediction
type MLModel struct {
	ModelID        string    `json:"model_id"`
	ModelType      string    `json:"model_type"` // linear_regression, arima, lstm
	TrainedAt      time.Time `json:"trained_at"`
	TrainingPoints int       `json:"training_points"`
	RMSE           float64   `json:"rmse"`
	MAE            float64   `json:"mae"`
	IsReady        bool      `json:"is_ready"`
}

// PowerPredictionRequest represents a request for power prediction
type PowerPredictionRequest struct {
	HorizonMinutes int      `json:"horizon_minutes"`
	DeviceIDs      []string `json:"device_ids,omitempty"`
	Granularity    int      `json:"granularity_minutes"`
}

// PowerPredictionResponse represents the response for power prediction
type PowerPredictionResponse struct {
	Predictions []PowerForecast `json:"predictions"`
	Model       MLModel         `json:"model"`
	TotalKWh    float64         `json:"total_kwh"`
}

// CarbonScheduleRequest represents a request for carbon-aware scheduling
type CarbonScheduleRequest struct {
	Tasks    []TaskToSchedule `json:"tasks"`
	Region   string           `json:"region"`
	MaxDelay int              `json:"max_delay_hours"`
}

// TaskToSchedule represents a task that needs carbon-aware scheduling
type TaskToSchedule struct {
	TaskID            string  `json:"task_id"`
	TaskName          string  `json:"task_name"`
	EstimatedPower    float64 `json:"estimated_power_watts"`
	EstimatedDuration int     `json:"estimated_duration_minutes"`
	Priority          int     `json:"priority"`
}

// CarbonScheduleResponse represents the response for carbon-aware scheduling
type CarbonScheduleResponse struct {
	Schedules      []CarbonAwareSchedule `json:"schedules"`
	TotalSavings   float64               `json:"total_savings_kg"`
	OptimalWindows []CarbonIntensity     `json:"optimal_windows"`
}

// EnergyCostRequest represents a request for energy cost calculation
type EnergyCostRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	GroupBy   string `json:"group_by"` // day, week, month
}

// EnergyCostResponse represents the response for energy cost calculation
type EnergyCostResponse struct {
	TotalCost  float64      `json:"total_cost"`
	TotalKWh   float64      `json:"total_kwh"`
	Currency   string       `json:"currency"`
	Breakdown  []EnergyCost `json:"breakdown"`
	TariffPlan TariffPlan   `json:"tariff_plan"`
}

// BudgetStatusResponse represents the response for budget status
type BudgetStatusResponse struct {
	Budgets []PowerBudget `json:"budgets"`
	Alerts  []BudgetAlert `json:"alerts,omitempty"`
}

// BudgetAlert represents a budget alert
type BudgetAlert struct {
	BudgetID   string    `json:"budget_id"`
	BudgetName string    `json:"budget_name"`
	Message    string    `json:"message"`
	Level      string    `json:"level"` // warning, critical
	Timestamp  time.Time `json:"timestamp"`
}

// EnergyReportRequest represents a request for generating an energy report
type EnergyReportRequest struct {
	ReportType string `json:"report_type"` // daily, weekly, monthly
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
}

// EnergyReportResponse represents the response for energy report
type EnergyReportResponse struct {
	Reports []EnergyReport `json:"reports"`
	Summary EnergyReport   `json:"summary"`
}
