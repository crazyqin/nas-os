// Package disk provides disk power management functionality.
// Implements intelligent disk sleep/wake patterns inspired by飞牛fnOS.
package disk

import (
	"context"
	"sync"
	"time"
)

// PowerState represents disk power state.
type PowerState string

const (
	PowerStateActive  PowerState = "active"  // 磁盘活跃
	PowerStateIdle    PowerState = "idle"    // 磁盘空闲
	PowerStateStandby PowerState = "standby" // 磁盘待机
	PowerStateSleep   PowerState = "sleep"   // 磁盘休眠
)

// SleepPolicy defines disk sleep policy.
type SleepPolicy struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	IdleThreshold    time.Duration `json:"idle_threshold"`    // 空闲阈值(秒)
	StandbyThreshold time.Duration `json:"standby_threshold"` // 待机阈值(秒)
	SleepThreshold   time.Duration `json:"sleep_threshold"`   // 休眠阈值(秒)
	Enabled          bool          `json:"enabled"`
	ExcludedDisks    []string      `json:"excluded_disks"` // 排除的磁盘
}

// DiskPowerStatus represents disk power status.
type DiskPowerStatus struct {
	DiskID       string        `json:"disk_id"`
	State        PowerState    `json:"state"`
	LastActivity time.Time     `json:"last_activity"`
	IdleDuration time.Duration `json:"idle_duration"`
	Policy       *SleepPolicy  `json:"policy"`
	WakeCount    int           `json:"wake_count"`    // 唤醒次数
	SleepCount   int           `json:"sleep_count"`   // 休眠次数
	EnergySaved  float64       `json:"energy_saved"`  // kWh节省
}

// PowerManager manages disk power states.
type PowerManager struct {
	mu          sync.RWMutex
	statuses    map[string]*DiskPowerStatus
	policies    map[string]*SleepPolicy
	activityMon *ActivityMonitor
	config      *PowerConfig
}

// PowerConfig holds power management configuration.
type PowerConfig struct {
	CheckInterval    time.Duration `json:"check_interval"`
	DefaultPolicy    string        `json:"default_policy"`
	EnableMonitoring bool          `json:"enable_monitoring"`
}

// NewPowerManager creates a new power manager.
func NewPowerManager(cfg *PowerConfig) *PowerManager {
	if cfg == nil {
		cfg = &PowerConfig{
			CheckInterval:    30 * time.Second,
			DefaultPolicy:    "default",
			EnableMonitoring: true,
		}
	}
	return &PowerManager{
		statuses:    make(map[string]*DiskPowerStatus),
		policies:    make(map[string]*SleepPolicy),
		activityMon: NewActivityMonitor(),
		config:      cfg,
	}
}

// Start starts the power manager monitoring loop.
func (pm *PowerManager) Start(ctx context.Context) error {
	if !pm.config.EnableMonitoring {
		return nil
	}

	go pm.monitorLoop(ctx)
	return nil
}

// monitorLoop monitors disk activity and applies sleep policies.
func (pm *PowerManager) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(pm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.checkDiskStates()
		}
	}
}

// checkDiskStates checks all disk states and applies policies.
func (pm *PowerManager) checkDiskStates() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	for diskID, status := range pm.statuses {
		if status.Policy == nil || !status.Policy.Enabled {
			continue
		}

		// Check if disk is excluded
		for _, excluded := range status.Policy.ExcludedDisks {
			if excluded == diskID {
				continue
			}
		}

		idleDuration := now.Sub(status.LastActivity)

		// Apply sleep thresholds
		if idleDuration >= status.Policy.SleepThreshold && status.State != PowerStateSleep {
			pm.transitionDisk(diskID, PowerStateSleep)
		} else if idleDuration >= status.Policy.StandbyThreshold && status.State != PowerStateStandby {
			pm.transitionDisk(diskID, PowerStateStandby)
		} else if idleDuration >= status.Policy.IdleThreshold && status.State != PowerStateIdle {
			pm.transitionDisk(diskID, PowerStateIdle)
		}

		status.IdleDuration = idleDuration
	}
}

// transitionDisk transitions disk to new power state.
func (pm *PowerManager) transitionDisk(diskID string, newState PowerState) {
	status, ok := pm.statuses[diskID]
	if !ok {
		return
	}

	oldState := status.State
	status.State = newState

	// Update counters
	if newState == PowerStateSleep && oldState != PowerStateSleep {
		status.SleepCount++
		// Calculate energy saved (rough estimate: 10W per sleeping disk)
		status.EnergySaved += float64(pm.config.CheckInterval.Hours()) * 10.0 / 1000.0
	}
	if newState == PowerStateActive && oldState == PowerStateSleep {
		status.WakeCount++
	}
}

// RegisterDisk registers a disk for power management.
func (pm *PowerManager) RegisterDisk(diskID string, policyID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	policy := pm.policies[policyID]
	if policy == nil && policyID != "" {
		policy = pm.policies[pm.config.DefaultPolicy]
	}

	pm.statuses[diskID] = &DiskPowerStatus{
		DiskID:       diskID,
		State:        PowerStateActive,
		LastActivity: time.Now(),
		Policy:       policy,
	}

	return nil
}

// RecordActivity records disk activity (wakes disk if sleeping).
func (pm *PowerManager) RecordActivity(diskID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	status, ok := pm.statuses[diskID]
	if !ok {
		return nil
	}

	// Wake disk if sleeping
	if status.State == PowerStateSleep || status.State == PowerStateStandby {
		pm.transitionDisk(diskID, PowerStateActive)
	}

	status.LastActivity = time.Now()
	status.IdleDuration = 0

	return nil
}

// GetDiskStatus returns disk power status.
func (pm *PowerManager) GetDiskStatus(diskID string) (*DiskPowerStatus, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	status, ok := pm.statuses[diskID]
	if !ok {
		return nil, nil
	}
	return status, nil
}

// GetAllStatuses returns all disk power statuses.
func (pm *PowerManager) GetAllStatuses() map[string]*DiskPowerStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]*DiskPowerStatus)
	for k, v := range pm.statuses {
		result[k] = v
	}
	return result
}

// AddPolicy adds a sleep policy.
func (pm *PowerManager) AddPolicy(policy *SleepPolicy) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.policies[policy.ID] = policy
	return nil
}

// GetPolicy returns a sleep policy.
func (pm *PowerManager) GetPolicy(policyID string) (*SleepPolicy, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	policy, ok := pm.policies[policyID]
	if !ok {
		return nil, nil
	}
	return policy, nil
}

// GetEnergyReport returns energy saving report.
func (pm *PowerManager) GetEnergyReport() *EnergyReport {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	report := &EnergyReport{
		GeneratedAt: time.Now(),
		Disks:       make([]DiskEnergyInfo, 0),
	}

	totalSaved := 0.0
	for diskID, status := range pm.statuses {
		info := DiskEnergyInfo{
			DiskID:      diskID,
			State:       status.State,
			SleepCount:  status.SleepCount,
			WakeCount:   status.WakeCount,
			EnergySaved: status.EnergySaved,
		}
		report.Disks = append(report.Disks, info)
		totalSaved += status.EnergySaved
	}

	report.TotalEnergySaved = totalSaved
	return report
}

// EnergyReport represents energy saving report.
type EnergyReport struct {
	GeneratedAt      time.Time        `json:"generated_at"`
	Disks            []DiskEnergyInfo `json:"disks"`
	TotalEnergySaved float64          `json:"total_energy_saved"` // kWh
}

// DiskEnergyInfo represents disk energy information.
type DiskEnergyInfo struct {
	DiskID      string     `json:"disk_id"`
	State       PowerState `json:"state"`
	SleepCount  int        `json:"sleep_count"`
	WakeCount   int        `json:"wake_count"`
	EnergySaved float64    `json:"energy_saved"` // kWh
}

// DefaultSleepPolicy returns the default sleep policy.
func DefaultSleepPolicy() *SleepPolicy {
	return &SleepPolicy{
		ID:               "default",
		Name:             "默认节能策略",
		IdleThreshold:    5 * time.Minute,
		StandbyThreshold: 15 * time.Minute,
		SleepThreshold:   30 * time.Minute,
		Enabled:          true,
		ExcludedDisks:    []string{},
	}
}