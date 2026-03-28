// Package storage provides disk power management functionality.
// Enables disk sleep/wake cycles to extend disk lifespan and reduce power consumption.
package storage

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PowerState represents disk power state.
type PowerState string

const (
	PowerStateActive   PowerState = "active"   // Disk is active
	PowerStateIdle     PowerState = "idle"     // Disk is idle
	PowerStateStandby  PowerState = "standby"  // Disk in standby
	PowerStateSleep    PowerState = "sleep"    // Disk in sleep mode
	PowerStateUnknown  PowerState = "unknown"  // State unknown
)

// PowerConfig holds power management configuration.
type PowerConfig struct {
	IdleTimeout      time.Duration `yaml:"idle_timeout"`      // Time before idle -> standby
	StandbyTimeout   time.Duration `yaml:"standby_timeout"`   // Time before standby -> sleep
	WakeupOnAccess   bool          `yaml:"wakeup_on_access"`  // Wake disk on access
	SmartPredictLife bool          `yaml:"smart_predict_life"` // Enable SMART lifespan prediction
	MaxPowerCycles   int           `yaml:"max_power_cycles"`  // Max cycles per day (protect disk)
}

// DiskPowerStatus represents power status of a disk.
type DiskPowerStatus struct {
	Device         string     `json:"device"`
	State          PowerState `json:"state"`
	LastActive     time.Time  `json:"last_active"`
	LastTransition time.Time  `json:"last_transition"`
	PowerCycles    int        `json:"power_cycles_today"`
	EstimatedLife  int        `json:"estimated_life_days"` // SMART prediction
	HealthScore    int        `json:"health_score"`        // 0-100
}

// PowerManager manages disk power states.
type PowerManager struct {
	config   PowerConfig
	statuses map[string]*DiskPowerStatus
	mu       sync.RWMutex
	stopCh   chan struct{}
}

// NewPowerManager creates a new power manager.
func NewPowerManager(config PowerConfig) *PowerManager {
	return &PowerManager{
		config:   config,
		statuses: make(map[string]*DiskPowerStatus),
		stopCh:   make(chan struct{}),
	}
}

// Start begins power management monitoring.
func (pm *PowerManager) Start(ctx context.Context) error {
	go pm.monitorLoop(ctx)
	return nil
}

// Stop stops power management.
func (pm *PowerManager) Stop() {
	close(pm.stopCh)
}

// RegisterDisk registers a disk for power management.
func (pm *PowerManager) RegisterDisk(device string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.statuses[device] = &DiskPowerStatus{
		Device:         device,
		State:          PowerStateActive,
		LastActive:     time.Now(),
		LastTransition: time.Now(),
		PowerCycles:    0,
		EstimatedLife:  365 * 5, // Default 5 years
		HealthScore:    100,
	}
}

// GetStatus returns power status for a disk.
func (pm *PowerManager) GetStatus(device string) (*DiskPowerStatus, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	status, ok := pm.statuses[device]
	if !ok {
		return nil, fmt.Errorf("disk %s not registered", device)
	}
	return status, nil
}

// GetAllStatuses returns all disk power statuses.
func (pm *PowerManager) GetAllStatuses() []*DiskPowerStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	result := make([]*DiskPowerStatus, 0, len(pm.statuses))
	for _, status := range pm.statuses {
		result = append(result, status)
	}
	return result
}

// WakeDisk wakes a disk from sleep/standby.
func (pm *PowerManager) WakeDisk(device string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	status, ok := pm.statuses[device]
	if !ok {
		return fmt.Errorf("disk %s not registered", device)
	}
	
	// Check power cycle limit
	if status.PowerCycles >= pm.config.MaxPowerCycles && pm.config.MaxPowerCycles > 0 {
		return fmt.Errorf("disk %s exceeded max power cycles today", device)
	}
	
	// Transition to active
	if status.State != PowerStateActive {
		status.State = PowerStateActive
		status.LastTransition = time.Now()
		status.PowerCycles++
	}
	status.LastActive = time.Now()
	
	return nil
}

// SetIdle marks a disk as idle (called after no activity detected).
func (pm *PowerManager) SetIdle(device string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	status, ok := pm.statuses[device]
	if !ok {
		return
	}
	
	if status.State == PowerStateActive {
		status.State = PowerStateIdle
		status.LastTransition = time.Now()
	}
}

// monitorLoop monitors disk activity and transitions power states.
func (pm *PowerManager) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.checkTransitions()
		}
	}
}

// checkTransitions checks and performs power state transitions.
func (pm *PowerManager) checkTransitions() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	now := time.Now()
	
	for device, status := range pm.statuses {
		idleDuration := now.Sub(status.LastActive)
		
		// Active -> Idle (immediate)
		if status.State == PowerStateActive && idleDuration > 10*time.Second {
			status.State = PowerStateIdle
			status.LastTransition = now
		}
		
		// Idle -> Standby
		if status.State == PowerStateIdle && idleDuration > pm.config.IdleTimeout {
			status.State = PowerStateStandby
			status.LastTransition = now
		}
		
		// Standby -> Sleep
		if status.State == PowerStateStandby && idleDuration > pm.config.StandbyTimeout {
			status.State = PowerStateSleep
			status.LastTransition = now
		}
	}
}

// UpdateSMARTData updates SMART health data for a disk.
func (pm *PowerManager) UpdateSMARTData(device string, lifeDays int, healthScore int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	status, ok := pm.statuses[device]
	if !ok {
		return
	}
	
	status.EstimatedLife = lifeDays
	status.HealthScore = healthScore
}

// CalculatePowerCost estimates power cost savings.
func (pm *PowerManager) CalculatePowerCost(device string, activeWatts int, sleepWatts int, costPerKWh float64) float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	status, ok := pm.statuses[device]
	if !ok {
		return 0
	}
	
	// Calculate time in each state (simplified)
	sleepHours := float64(now.Sub(status.LastActive).Hours())
	if sleepHours < 0 {
		sleepHours = 0
	}
	
	// Power savings: (activeWatts - sleepWatts) * sleepHours / 1000 * costPerKWh
	savings := float64(activeWatts-sleepWatts) * sleepHours / 1000.0 * costPerKWh
	return savings
}

var now = time.Now // for testing