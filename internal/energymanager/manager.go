package energymanager

import (
	"fmt"
	"sync"
	"time"
)

// Manager manages the energy/power system
type Manager struct {
	mu           sync.RWMutex
	profiles     map[string]*PowerProfile
	schedules    map[string]*ScheduleEntry
	alerts       []*TemperatureAlert
	currentUsage *PowerUsage
	history      []PowerUsage
	config       *EnergyConfig
	powerState   string
}

// Config is an alias for EnergyConfig
type Config = EnergyConfig

// EnergyConfig represents energy manager configuration
type EnergyConfig struct {
	Enabled             bool    `json:"enabled"`
	MonitoringInterval  int     `json:"monitoring_interval"`
	ElectricityRate     float64 `json:"electricity_rate"`
	Currency            string  `json:"currency"`
	AlertThreshold      float64 `json:"alert_threshold"`
	AlertThresholdW     float64 `json:"alert_threshold_w,omitempty"`
	AutoPowerSave       bool    `json:"auto_power_save,omitempty"`
	CarbonFactor        float64 `json:"carbon_factor,omitempty"`
}

// NewManager creates a new energy manager
func NewManager(config *EnergyConfig) *Manager {
	if config == nil {
		config = &EnergyConfig{
			ElectricityRate: 0.12,
			Currency:        "USD",
			AlertThreshold:  85.0,
		}
	}

	return &Manager{
		profiles:  make(map[string]*PowerProfile),
		schedules: make(map[string]*ScheduleEntry),
		config:    config,
	}
}

// CreateProfile creates a new power profile
func (m *Manager) CreateProfile(name, description, profileType string, settings PowerSettings) *PowerProfile {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	profile := &PowerProfile{
		ID:          fmt.Sprintf("profile-%d", now.UnixNano()),
		Name:        name,
		Description: description,
		Type:        profileType,
		Settings:    settings,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.profiles[profile.ID] = profile
	return profile
}

// GetProfile returns a profile by ID
func (m *Manager) GetProfile(id string) (*PowerProfile, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	profile, ok := m.profiles[id]
	return profile, ok
}

// ListProfiles lists all profiles
func (m *Manager) ListProfiles() []*PowerProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profiles := make([]*PowerProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// SetActiveProfile sets the active profile
func (m *Manager) SetActiveProfile(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, ok := m.profiles[id]
	if !ok {
		return fmt.Errorf("profile not found: %s", id)
	}

	for _, p := range m.profiles {
		p.IsActive = false
	}

	profile.IsActive = true
	return nil
}

// GetCurrentPowerUsage returns current power usage
func (m *Manager) GetCurrentPowerUsage() *PowerUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentUsage
}

// GetPowerHistory returns power usage history
func (m *Manager) GetPowerHistory(period string) *PowerHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := &PowerHistory{
		Period: period,
	}

	var totalWatts float64
	var maxWatts float64
	var minWatts float64 = 999999

	for _, usage := range m.history {
		history.DataPoints = append(history.DataPoints, usage)
		totalWatts += usage.TotalWatts
		if usage.TotalWatts > maxWatts {
			maxWatts = usage.TotalWatts
		}
		if usage.TotalWatts < minWatts {
			minWatts = usage.TotalWatts
		}
	}

	if len(m.history) > 0 {
		history.AvgWatts = totalWatts / float64(len(m.history))
		history.MaxWatts = maxWatts
		history.MinWatts = minWatts
		history.TotalKWh = totalWatts * float64(len(m.history)) / 1000.0 / 24.0
		history.CostEstimate = history.TotalKWh * m.config.ElectricityRate
	}

	return history
}

// GetEnergyStats returns energy statistics
func (m *Manager) GetEnergyStats() *EnergyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &EnergyStats{
		CostPerKWh: m.config.ElectricityRate,
	}

	for _, usage := range m.history {
		stats.TodayKWh += usage.TotalWatts / 1000.0 / 24.0
	}

	stats.TodayCost = stats.TodayKWh * m.config.ElectricityRate
	stats.CurrentWatts = m.currentUsage.TotalWatts

	return stats
}

// CreateSchedule creates a power schedule
func (m *Manager) CreateSchedule(name, scheduleType, timeStr string, days []string) *ScheduleEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	schedule := &ScheduleEntry{
		ID:        fmt.Sprintf("schedule-%d", now.UnixNano()),
		Name:      name,
		Type:      scheduleType,
		Time:      timeStr,
		Days:      days,
		IsActive:  true,
		CreatedAt: now,
	}
	m.schedules[schedule.ID] = schedule
	return schedule
}

// ListSchedules lists all schedules
func (m *Manager) ListSchedules() []*ScheduleEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedules := make([]*ScheduleEntry, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// DeleteSchedule deletes a schedule
func (m *Manager) DeleteSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schedules[id]; !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}

	delete(m.schedules, id)
	return nil
}

// GetAlerts returns temperature alerts
func (m *Manager) GetAlerts() []*TemperatureAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alerts
}

// AcknowledgeAlert acknowledges a temperature alert
func (m *Manager) AcknowledgeAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == id {
			alert.Acknowledged = true
			return nil
		}
	}

	return fmt.Errorf("alert not found: %s", id)
}

// CarbonMetrics represents carbon emission metrics
type CarbonMetrics struct {
	GridCarbonFactor float64 `json:"grid_carbon_factor"`
	TotalCO2Kg      float64 `json:"total_co2_kg"`
	TodayCO2Kg      float64 `json:"today_co2_kg"`
}

// GetCarbonMetrics returns carbon emission metrics
func (m *Manager) GetCarbonMetrics() *CarbonMetrics {
	factor := m.config.CarbonFactor
	if factor == 0 {
		factor = 0.5 // default grid carbon factor
	}
	return &CarbonMetrics{
		GridCarbonFactor: factor,
	}
}

// PowerReading represents a power reading
type PowerReading struct {
	Timestamp time.Time `json:"timestamp"`
	Watts     float64   `json:"watts"`
	Voltage   float64   `json:"voltage"`
	Current   float64   `json:"current"`
	Source    string    `json:"source"`
}

// Power source constants
const (
	SourceGrid string = "grid"
	SourceBattery string = "battery"
	SourceSolar string = "solar"
)

// Power state constants
const (
	PowerStandby string = "standby"
	PowerActive  string = "active"
	PowerSleep   string = "sleep"
)

// PowerStats represents power statistics
type PowerStats struct {
	CurrentWatts float64 `json:"current_watts"`
	PowerState   string  `json:"power_state"`
}

// RecordReading records a power reading
func (m *Manager) RecordReading(reading *PowerReading) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentUsage = &PowerUsage{
		Timestamp:  reading.Timestamp,
		TotalWatts: reading.Watts,
	}
}

// GetStats returns current power stats
func (m *Manager) GetStats() *PowerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	watts := 0.0
	if m.currentUsage != nil {
		watts = m.currentUsage.TotalWatts
	}
	return &PowerStats{
		CurrentWatts: watts,
		PowerState:   m.powerState,
	}
}

// SetPowerState sets the power state
func (m *Manager) SetPowerState(state string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.powerState = state
	return nil
}

// EstimateBill estimates the electricity bill
func (m *Manager) EstimateBill(days int) *BillEstimate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	totalKWh := 0.0
	for _, u := range m.history {
		totalKWh += u.TotalWatts / 1000.0
	}
	if len(m.history) > 0 {
		totalKWh = totalKWh / float64(len(m.history)) * float64(days) * 24.0
	}
	return &BillEstimate{
		Days:       days,
		TotalKWh:   totalKWh,
		TotalCost:  totalKWh * m.config.ElectricityRate,
		CostPerKWh: m.config.ElectricityRate,
		Currency:   m.config.Currency,
	}
}

// BillEstimate represents a bill estimate
type BillEstimate struct {
	Days       int     `json:"days"`
	TotalKWh   float64 `json:"total_kwh"`
	TotalCost  float64 `json:"total_cost"`
	CostPerKWh float64 `json:"cost_per_kwh"`
	Currency   string  `json:"currency"`
}

// SaveProfile saves a profile (accepts *PowerProfile, returns error)
func (m *Manager) SaveProfile(profile *PowerProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if profile.ID == "" {
		profile.ID = fmt.Sprintf("profile-%d", time.Now().UnixNano())
	}
	now := time.Now()
	profile.CreatedAt = now
	profile.UpdatedAt = now
	m.profiles[profile.ID] = profile
	return nil
}

// FetchProfile returns a profile by ID (returns error if not found)
func (m *Manager) FetchProfile(id string) (*PowerProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	profile, ok := m.profiles[id]
	if !ok {
		return nil, fmt.Errorf("profile not found: %s", id)
	}
	return profile, nil
}
