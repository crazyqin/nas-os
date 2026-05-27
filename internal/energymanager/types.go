// Package energymanager provides power consumption monitoring and optimization for NAS-OS
// Features: Real-time power monitoring, scheduling, cost estimation, green computing
// Competitor benchmark: 对标群晖电源管理, 超越TrueNAS节能能力
package energymanager

import (
	"context"
	"sync"
	"time"
)

// PowerState represents device power state
type PowerState string

const (
	PowerOn      PowerState = "on"
	PowerOff     PowerState = "off"
	PowerStandby PowerState = "standby"
	PowerSuspend PowerState = "suspend"
	PowerHibernate PowerState = "hibernate"
)

// EnergySource represents power source type
type EnergySource string

const (
	SourceGrid    EnergySource = "grid"
	SourceSolar   EnergySource = "solar"
	SourceBattery EnergySource = "battery"
	SourceUPS     EnergySource = "ups"
)

// PowerReading represents a power measurement
type PowerReading struct {
	Timestamp    time.Time   `json:"timestamp"`
	Watts        float64     `json:"watts"`
	Voltage      float64     `json:"voltage"`
	Current      float64     `json:"current_amps"`
	Temperature  float64     `json:"temperature_c"`
	Source       EnergySource `json:"source"`
}

// PowerProfile represents a power management profile
type PowerProfile struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	MaxWatts    float64       `json:"max_watts"`
	IdleWatts   float64       `json:"idle_watts"`
	SpinDownHDD bool          `json:"spin_down_hdd"`
	LEDControl  bool          `json:"led_control"`
	FanCurve    string        `json:"fan_curve"` // quiet, balanced, performance
	Schedule    *Schedule     `json:"schedule"`
	IsDefault   bool          `json:"is_default"`
}

// Schedule represents a power schedule
type Schedule struct {
	Enabled    bool      `json:"enabled"`
	StartTime  string    `json:"start_time"` // HH:MM
	EndTime    string    `json:"end_time"`
	Days       []string  `json:"days"`       // mon, tue, wed, thu, fri, sat, sun
	Action     PowerState `json:"action"`
}

// PowerBudget represents daily/weekly power budget
type PowerBudget struct {
	DailyKWh     float64 `json:"daily_kwh"`
	WeeklyKWh    float64 `json:"weekly_kwh"`
	MonthlyKWh   float64 `json:"monthly_kwh"`
	CostPerKWh   float64 `json:"cost_per_kwh"`
	DailyCost    float64 `json:"daily_cost"`
	WeeklyCost   float64 `json:"weekly_cost"`
	MonthlyCost  float64 `json:"monthly_cost"`
	Currency     string  `json:"currency"`
}

// EnergyStats represents energy statistics
type EnergyStats struct {
	CurrentWatts     float64        `json:"current_watts"`
	AverageWatts     float64        `json:"average_watts"`
	PeakWatts        float64        `json:"peak_watts"`
	MinWatts         float64        `json:"min_watts"`
	TodayKWh         float64        `json:"today_kwh"`
	WeekKWh          float64        `json:"week_kwh"`
	MonthKWh         float64        `json:"month_kwh"`
	TotalKWh         float64        `json:"total_kwh"`
	EstimatedBill    float64        `json:"estimated_bill"`
	CarbonFootprint  float64        `json:"carbon_footprint_kg"`
	PowerState       PowerState     `json:"power_state"`
	ActiveProfile    string         `json:"active_profile"`
	Readings         []*PowerReading `json:"recent_readings"`
	UptimePercent    float64        `json:"uptime_percent"`
}

// CarbonMetrics represents carbon footprint data
type CarbonMetrics struct {
	TotalKgCO2     float64 `json:"total_kg_co2"`
	DailyKgCO2     float64 `json:"daily_kg_co2"`
	TreesEquivalent float64 `json:"trees_equivalent"`
	GridCarbonFactor float64 `json:"grid_carbon_factor"`
}

// Config holds energy manager configuration
type Config struct {
	Enabled           bool    `json:"enabled"`
	MonitoringInterval int    `json:"monitoring_interval_seconds"`
	ElectricityRate   float64 `json:"electricity_rate"`
	Currency          string  `json:"currency"`
	CarbonFactor      float64 `json:"carbon_factor_kg_per_kwh"`
	AlertThresholdW   float64 `json:"alert_threshold_watts"`
	AutoPowerSave     bool    `json:"auto_power_save"`
}

// Manager manages energy and power
type Manager struct {
	config    *Config
	profiles  map[string]*PowerProfile
	readings  []*PowerReading
	stats     *EnergyStats
	state     PowerState
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new energy manager
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:   config,
		profiles: make(map[string]*PowerProfile),
		readings: make([]*PowerReading, 0, 1000),
		state:    PowerOn,
		ctx:      ctx,
		cancel:   cancel,
		stats: &EnergyStats{
			PowerState: PowerOn,
			Readings:   make([]*PowerReading, 0),
		},
	}
}

// RecordReading records a power reading
func (m *Manager) RecordReading(reading *PowerReading) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.readings = append(m.readings, reading)
	if len(m.readings) > 1000 {
		m.readings = m.readings[1:]
	}

	m.stats.CurrentWatts = reading.Watts
	if reading.Watts > m.stats.PeakWatts {
		m.stats.PeakWatts = reading.Watts
	}
	if m.stats.MinWatts == 0 || reading.Watts < m.stats.MinWatts {
		m.stats.MinWatts = reading.Watts
	}

	m.updateAverages()
}

// GetStats returns current energy statistics
func (m *Manager) GetStats() *EnergyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// SetPowerState sets the system power state
func (m *Manager) SetPowerState(state PowerState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	m.stats.PowerState = state
	return nil
}

// CreateProfile creates a new power profile
func (m *Manager) CreateProfile(profile *PowerProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles[profile.ID] = profile
	return nil
}

// GetProfile returns a power profile by ID
func (m *Manager) GetProfile(id string) (*PowerProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	profile, exists := m.profiles[id]
	if !exists {
		return nil, nil
	}
	return profile, nil
}

// EstimateBill estimates electricity bill
func (m *Manager) EstimateBill(days int) *PowerBudget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rate := m.config.ElectricityRate
	if rate == 0 {
		rate = 0.6
	}

	dailyKWh := m.stats.TodayKWh
	monthlyKWh := dailyKWh * 30

	return &PowerBudget{
		DailyKWh:    dailyKWh,
		WeeklyKWh:   dailyKWh * 7,
		MonthlyKWh:  monthlyKWh,
		CostPerKWh:  rate,
		DailyCost:   dailyKWh * rate,
		WeeklyCost:  dailyKWh * 7 * rate,
		MonthlyCost: monthlyKWh * rate,
		Currency:    m.config.Currency,
	}
}

// GetCarbonMetrics returns carbon footprint metrics
func (m *Manager) GetCarbonMetrics() *CarbonMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	factor := m.config.CarbonFactor
	if factor == 0 {
		factor = 0.5
	}

	totalKg := m.stats.TotalKWh * factor
	return &CarbonMetrics{
		TotalKgCO2:       totalKg,
		DailyKgCO2:       m.stats.TodayKWh * factor,
		TreesEquivalent:  totalKg / 21.77, // 1 tree absorbs ~21.77 kg CO2/year
		GridCarbonFactor: factor,
	}
}

func (m *Manager) updateAverages() {
	if len(m.readings) == 0 {
		return
	}
	var sum float64
	for _, r := range m.readings {
		sum += r.Watts
	}
	m.stats.AverageWatts = sum / float64(len(m.readings))
}

// Stop stops the energy manager
func (m *Manager) Stop() {
	m.cancel()
}
