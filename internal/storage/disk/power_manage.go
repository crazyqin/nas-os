// Package disk provides disk power management for NAS-OS
// Implements disk spin-down/spin-up policies for energy saving
package disk

import (
	"context"
	"fmt"
	"time"
)

// PowerMode represents disk power states.
type PowerMode string

const (
	PowerModeActive  PowerMode = "active"  // Disk is active and spinning
	PowerModeIdle    PowerMode = "idle"    // Disk is idle but spinning
	PowerModeStandby PowerMode = "standby" // Disk is in standby (spun down)
	PowerModeSleep   PowerMode = "sleep"   // Disk is in sleep mode
)

// PowerPolicy defines disk power management policy.
type PowerPolicy struct {
	// IdleTimeout is the time before disk enters idle state
	IdleTimeout time.Duration `json:"idle_timeout"`

	// StandbyTimeout is the time before disk spins down
	StandbyTimeout time.Duration `json:"standby_timeout"`

	// SleepTimeout is the time before disk enters sleep mode
	SleepTimeout time.Duration `json:"sleep_timeout"`

	// EnableAPM enables Advanced Power Management
	EnableAPM bool `json:"enable_apm"`

	// APMLevel is the APM level (1-255, lower = more power saving)
	APMLevel int `json:"apm_level"`

	// EnableSMART enables SMART monitoring during standby
	EnableSMART bool `json:"enable_smart"`
}

// DefaultPowerPolicy returns the default power policy.
func DefaultPowerPolicy() *PowerPolicy {
	return &PowerPolicy{
		IdleTimeout:    5 * time.Minute,
		StandbyTimeout: 30 * time.Minute,
		SleepTimeout:   60 * time.Minute,
		EnableAPM:      true,
		APMLevel:       128, // Balanced power/performance
		EnableSMART:    true,
	}
}

// DiskPowerManager manages disk power states.
type DiskPowerManager struct {
	policy    *PowerPolicy
	disks     map[string]*DiskPowerState
	monitor   *PowerMonitor
	configDir string
}

// DiskPowerState tracks individual disk power state.
type DiskPowerState struct {
	DiskID        string        `json:"disk_id"`
	CurrentMode   PowerMode     `json:"current_mode"`
	LastActivity  time.Time     `json:"last_activity"`
	SpindownCount int           `json:"spindown_count"`
	SpinupCount   int           `json:"spinup_count"`
	TotalIdleTime time.Duration `json:"total_idle_time"`
}

// PowerMonitor monitors disk activity.
type PowerMonitor struct {
	checkInterval     time.Duration
	activityThreshold int // IO operations threshold
}

// NewDiskPowerManager creates a new disk power manager.
func NewDiskPowerManager(policy *PowerPolicy, configDir string) *DiskPowerManager {
	if policy == nil {
		policy = DefaultPowerPolicy()
	}

	return &DiskPowerManager{
		policy:    policy,
		disks:     make(map[string]*DiskPowerState),
		monitor:   &PowerMonitor{checkInterval: 30 * time.Second, activityThreshold: 5},
		configDir: configDir,
	}
}

// RegisterDisk registers a disk for power management.
func (m *DiskPowerManager) RegisterDisk(diskID string) error {
	if _, exists := m.disks[diskID]; exists {
		return fmt.Errorf("disk %s already registered", diskID)
	}

	m.disks[diskID] = &DiskPowerState{
		DiskID:        diskID,
		CurrentMode:   PowerModeActive,
		LastActivity:  time.Now(),
		SpindownCount: 0,
		SpinupCount:   0,
		TotalIdleTime: 0,
	}

	return nil
}

// UnregisterDisk removes a disk from power management.
func (m *DiskPowerManager) UnregisterDisk(diskID string) error {
	if _, exists := m.disks[diskID]; !exists {
		return fmt.Errorf("disk %s not registered", diskID)
	}

	delete(m.disks, diskID)
	return nil
}

// UpdateActivity updates disk activity timestamp.
func (m *DiskPowerManager) UpdateActivity(diskID string) error {
	state, exists := m.disks[diskID]
	if !exists {
		return fmt.Errorf("disk %s not registered", diskID)
	}

	state.LastActivity = time.Now()

	// If disk was in standby, spin it up
	if state.CurrentMode == PowerModeStandby || state.CurrentMode == PowerModeSleep {
		state.CurrentMode = PowerModeActive
		state.SpinupCount++
	}

	return nil
}

// CheckPowerStates checks all disks and updates power states.
func (m *DiskPowerManager) CheckPowerStates(ctx context.Context) error {
	now := time.Now()

	for diskID, state := range m.disks {
		idleDuration := now.Sub(state.LastActivity)

		// Check if disk should transition to lower power state
		switch state.CurrentMode {
		case PowerModeActive:
			if idleDuration >= m.policy.IdleTimeout {
				state.CurrentMode = PowerModeIdle
			}
		case PowerModeIdle:
			if idleDuration >= m.policy.StandbyTimeout {
				state.CurrentMode = PowerModeStandby
				state.SpindownCount++
				state.TotalIdleTime += idleDuration

				// Execute spindown command
				if err := m.spindownDisk(diskID); err != nil {
					// Log error but continue
					fmt.Printf("spindown error for disk %s: %v\n", diskID, err)
				}
			}
		case PowerModeStandby:
			if idleDuration >= m.policy.SleepTimeout && m.policy.SleepTimeout > 0 {
				state.CurrentMode = PowerModeSleep
				if err := m.sleepDisk(diskID); err != nil {
					fmt.Printf("sleep error for disk %s: %v\n", diskID, err)
				}
			}
		}
	}

	return nil
}

// spindownDisk sends spindown command to disk.
func (m *DiskPowerManager) spindownDisk(diskID string) error {
	// Implementation depends on disk type (HDD/SSD)
	// For HDD: use hdparm -y or SCSI SYNCHRONIZE CACHE
	// For SSD: no spindown needed
	fmt.Printf("Spinning down disk %s\n", diskID)
	return nil
}

// sleepDisk sends sleep command to disk.
func (m *DiskPowerManager) sleepDisk(diskID string) error {
	fmt.Printf("Putting disk %s to sleep\n", diskID)
	return nil
}

// SpinupDisk manually spins up a disk.
func (m *DiskPowerManager) SpinupDisk(diskID string) error {
	state, exists := m.disks[diskID]
	if !exists {
		return fmt.Errorf("disk %s not registered", diskID)
	}

	state.CurrentMode = PowerModeActive
	state.LastActivity = time.Now()
	state.SpinupCount++

	fmt.Printf("Spinning up disk %s\n", diskID)
	return nil
}

// GetDiskPowerState returns the power state of a disk.
func (m *DiskPowerManager) GetDiskPowerState(diskID string) (*DiskPowerState, error) {
	state, exists := m.disks[diskID]
	if !exists {
		return nil, fmt.Errorf("disk %s not registered", diskID)
	}

	return state, nil
}

// GetAllDiskPowerStates returns all disk power states.
func (m *DiskPowerManager) GetAllDiskPowerStates() map[string]*DiskPowerState {
	return m.disks
}

// SetPowerPolicy updates the power policy.
func (m *DiskPowerManager) SetPowerPolicy(policy *PowerPolicy) error {
	if policy == nil {
		return fmt.Errorf("policy cannot be nil")
	}

	m.policy = policy
	return nil
}

// GetPowerPolicy returns the current power policy.
func (m *DiskPowerManager) GetPowerPolicy() *PowerPolicy {
	return m.policy
}

// PowerStats returns aggregate power statistics.
type PowerStats struct {
	TotalDisks       int `json:"total_disks"`
	ActiveDisks      int `json:"active_disks"`
	IdleDisks        int `json:"idle_disks"`
	StandbyDisks     int `json:"standby_disks"`
	SleepDisks       int `json:"sleep_disks"`
	TotalSpindowns   int `json:"total_spindowns"`
	TotalSpinups     int `json:"total_spinups"`
	EstimatedSavings kWh `json:"estimated_savings"` // Estimated energy savings
}

// kWh represents kilowatt-hours.
type kWh float64

// GetPowerStats returns aggregate power statistics.
func (m *DiskPowerManager) GetPowerStats() *PowerStats {
	stats := &PowerStats{TotalDisks: len(m.disks)}

	for _, state := range m.disks {
		switch state.CurrentMode {
		case PowerModeActive:
			stats.ActiveDisks++
		case PowerModeIdle:
			stats.IdleDisks++
		case PowerModeStandby:
			stats.StandbyDisks++
		case PowerModeSleep:
			stats.SleepDisks++
		}
		stats.TotalSpindowns += state.SpindownCount
		stats.TotalSpinups += state.SpinupCount
	}

	// Estimate energy savings (approximate)
	// Typical HDD: ~10W active, ~1W standby
	// Savings = (standby_hours * 9W) / 1000
	standbyHours := float64(stats.StandbyDisks) * 24 // Assume 24h standby for standby disks
	stats.EstimatedSavings = kWh(standbyHours * 9 / 1000)

	return stats
}

// StartPowerMonitoring starts periodic power state monitoring.
func (m *DiskPowerManager) StartPowerMonitoring(ctx context.Context) {
	ticker := time.NewTicker(m.monitor.checkInterval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				m.CheckPowerStates(ctx)
			}
		}
	}()
}
