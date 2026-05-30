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
