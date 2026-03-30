// Package storage provides disk power management functionality.
// Learning from fnOS: on-demand disk wake-up to extend disk lifespan.
package storage

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DiskPowerState represents the current power state of a disk.
type DiskPowerState string

const (
	DiskPowerActive   DiskPowerState = "active"   // Disk is spinning and active
	DiskPowerIdle     DiskPowerState = "idle"     // Disk is idle but spinning
	DiskPowerStandby  DiskPowerState = "standby"  // Disk in standby mode
	DiskPowerSleeping DiskPowerState = "sleeping" // Disk is in sleep mode
)

// DiskPowerConfig defines power management configuration for a disk.
type DiskPowerConfig struct {
	// Device path (e.g., /dev/sda)
	DevicePath string `json:"device_path"`

	// Idle timeout before entering standby (seconds)
	IdleTimeoutSeconds int `json:"idle_timeout_seconds" default:"300"`

	// Standby timeout before sleep (seconds)
	StandbyTimeoutSeconds int `json:"standby_timeout_seconds" default:"600"`

	// Sleep timeout before deep sleep (seconds)
	SleepTimeoutSeconds int `json:"sleep_timeout_seconds" default:"1200"`

	// Enable automatic wake-up detection
	AutoWakeDetection bool `json:"auto_wake_detection" default:"true"`

	// Delay wake-up strategy (ms) - staggered wake-up to avoid power spikes
	DelayWakeUpMs int `json:"delay_wakeup_ms" default:"500"`

	// Maximum concurrent wake-ups to limit power surge
	MaxConcurrentWakeUps int `json:"max_concurrent_wakeups" default:"3"`

	// Enable SMART monitoring for power events
	SmartMonitoring bool `json:"smart_monitoring" default:"true"`
}

// DiskPowerEvent represents a power state change event.
type DiskPowerEvent struct {
	DevicePath   string         `json:"device_path"`
	PreviousState DiskPowerState `json:"previous_state"`
	NewState     DiskPowerState `json:"new_state"`
	Timestamp    time.Time      `json:"timestamp"`
	Reason       string         `json:"reason"`
}

// DiskPowerManager manages disk power states and wake-up schedules.
type DiskPowerManager struct {
	configs    map[string]*DiskPowerConfig
	states     map[string]DiskPowerState
	lastAccess map[string]time.Time
	events     []DiskPowerEvent
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc

	// Wake-up queue for staggered wake-ups
	wakeUpQueue chan string
	wakeUpSem   chan struct{} // Semaphore for max concurrent wake-ups

	// Callbacks for system integration
	onStateChange func(event DiskPowerEvent)
	onWakeUp      func(devicePath string) error
}

// NewDiskPowerManager creates a new disk power manager.
func NewDiskPowerManager(ctx context.Context) *DiskPowerManager {
	childCtx, cancel := context.WithCancel(ctx)
	return &DiskPowerManager{
		configs:     make(map[string]*DiskPowerConfig),
		states:      make(map[string]DiskPowerState),
		lastAccess:  make(map[string]time.Time),
		events:      make([]DiskPowerEvent, 0),
		ctx:         childCtx,
		cancel:      cancel,
		wakeUpQueue: make(chan string, 100),
		wakeUpSem:   make(chan struct{}, 3), // Default max concurrent
	}
}

// RegisterDisk registers a disk for power management.
func (m *DiskPowerManager) RegisterDisk(config *DiskPowerConfig) error {
	if config.DevicePath == "" {
		return fmt.Errorf("device path is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Set defaults
	if config.IdleTimeoutSeconds == 0 {
		config.IdleTimeoutSeconds = 300
	}
	if config.StandbyTimeoutSeconds == 0 {
		config.StandbyTimeoutSeconds = 600
	}
	if config.SleepTimeoutSeconds == 0 {
		config.SleepTimeoutSeconds = 1200
	}
	if config.DelayWakeUpMs == 0 {
		config.DelayWakeUpMs = 500
	}
	if config.MaxConcurrentWakeUps == 0 {
		config.MaxConcurrentWakeUps = 3
	}

	m.configs[config.DevicePath] = config
	m.states[config.DevicePath] = DiskPowerActive
	m.lastAccess[config.DevicePath] = time.Now()

	// Update semaphore capacity
	m.wakeUpSem = make(chan struct{}, config.MaxConcurrentWakeUps)

	return nil
}

// UnregisterDisk removes a disk from power management.
func (m *DiskPowerManager) UnregisterDisk(devicePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.configs, devicePath)
	delete(m.states, devicePath)
	delete(m.lastAccess, devicePath)
}

// RecordAccess records disk access to prevent premature sleep.
func (m *DiskPowerManager) RecordAccess(devicePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.configs[devicePath]; exists {
		m.lastAccess[devicePath] = time.Now()
		// If disk was sleeping, initiate wake-up
		if m.states[devicePath] == DiskPowerSleeping || m.states[devicePath] == DiskPowerStandby {
			m.initiateWakeUp(devicePath)
		}
	}
}

// initiateWakeUp queues a disk for staggered wake-up.
func (m *DiskPowerManager) initiateWakeUp(devicePath string) {
	select {
	case m.wakeUpQueue <- devicePath:
	default:
		// Queue full, wake-up already pending
	}
}

// WakeUpDisk explicitly wakes up a disk.
func (m *DiskPowerManager) WakeUpDisk(devicePath string) error {
	m.mu.RLock()
	config, exists := m.configs[devicePath]
	currentState := m.states[devicePath]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("disk %s not registered", devicePath)
	}

	if currentState == DiskPowerActive {
		return nil // Already active
	}

	// Acquire semaphore for concurrent wake-up limit
	select {
	case m.wakeUpSem <- struct{}{}:
		defer func() { <-m.wakeUpSem }()
	case <-m.ctx.Done():
		return m.ctx.Err()
	}

	// Apply delay strategy
	if config.DelayWakeUpMs > 0 {
		time.Sleep(time.Duration(config.DelayWakeUpMs) * time.Millisecond)
	}

	// Update state
	m.mu.Lock()
	m.states[devicePath] = DiskPowerActive
	m.lastAccess[devicePath] = time.Now()
	event := DiskPowerEvent{
		DevicePath:    devicePath,
		PreviousState: currentState,
		NewState:      DiskPowerActive,
		Timestamp:     time.Now(),
		Reason:        "explicit_wakeup",
	}
	m.events = append(m.events, event)
	m.mu.Unlock()

	// Callback
	if m.onWakeUp != nil {
		if err := m.onWakeUp(devicePath); err != nil {
			return err
		}
	}
	if m.onStateChange != nil {
		m.onStateChange(event)
	}

	return nil
}

// GetState returns the current power state of a disk.
func (m *DiskPowerManager) GetState(devicePath string) (DiskPowerState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[devicePath]
	if !exists {
		return "", fmt.Errorf("disk %s not registered", devicePath)
	}
	return state, nil
}

// GetConfig returns the power configuration for a disk.
func (m *DiskPowerManager) GetConfig(devicePath string) (*DiskPowerConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.configs[devicePath]
	if !exists {
		return nil, fmt.Errorf("disk %s not registered", devicePath)
	}
	return config, nil
}

// SetState manually sets a disk's power state.
func (m *DiskPowerManager) SetState(devicePath string, state DiskPowerState, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.configs[devicePath]; !exists {
		return fmt.Errorf("disk %s not registered", devicePath)
	}

	prevState := m.states[devicePath]
	m.states[devicePath] = state

	event := DiskPowerEvent{
		DevicePath:    devicePath,
		PreviousState: prevState,
		NewState:      state,
		Timestamp:     time.Now(),
		Reason:        reason,
	}
	m.events = append(m.events, event)

	if m.onStateChange != nil {
		m.onStateChange(event)
	}

	return nil
}

// Start begins the power management monitoring loop.
func (m *DiskPowerManager) Start() {
	go m.monitorLoop()
	go m.wakeUpLoop()
}

// Stop stops the power manager.
func (m *DiskPowerManager) Stop() {
	m.cancel()
}

// monitorLoop checks for idle disks and transitions them to lower power states.
func (m *DiskPowerManager) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkIdleDisks()
		case <-m.ctx.Done():
			return
		}
	}
}

// checkIdleDisks transitions idle disks to lower power states.
func (m *DiskPowerManager) checkIdleDisks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for devicePath, lastAccess := range m.lastAccess {
		config := m.configs[devicePath]
		currentState := m.states[devicePath]
		idleDuration := now.Sub(lastAccess).Seconds()

		var newState DiskPowerState
		var reason string

		switch currentState {
		case DiskPowerActive:
			if idleDuration >= float64(config.IdleTimeoutSeconds) {
				newState = DiskPowerIdle
				reason = "idle_timeout"
			}
		case DiskPowerIdle:
			if idleDuration >= float64(config.StandbyTimeoutSeconds) {
				newState = DiskPowerStandby
				reason = "standby_timeout"
			}
		case DiskPowerStandby:
			if idleDuration >= float64(config.SleepTimeoutSeconds) {
				newState = DiskPowerSleeping
				reason = "sleep_timeout"
			}
		}

		if newState != "" && newState != currentState {
			m.states[devicePath] = newState
			event := DiskPowerEvent{
				DevicePath:    devicePath,
				PreviousState: currentState,
				NewState:      newState,
				Timestamp:     now,
				Reason:        reason,
			}
			m.events = append(m.events, event)

			if m.onStateChange != nil {
				m.onStateChange(event)
			}
		}
	}
}

// wakeUpLoop processes queued wake-up requests.
func (m *DiskPowerManager) wakeUpLoop() {
	for {
		select {
		case devicePath := <-m.wakeUpQueue:
			m.WakeUpDisk(devicePath)
		case <-m.ctx.Done():
			return
		}
	}
}

// GetEvents returns recent power events.
func (m *DiskPowerManager) GetEvents(limit int) []DiskPowerEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	// Return last N events
	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}
	return m.events[start:]
}

// SetCallbacks allows custom callbacks for power events.
func (m *DiskPowerManager) SetCallbacks(
	onStateChange func(event DiskPowerEvent),
	onWakeUp func(devicePath string) error,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStateChange = onStateChange
	m.onWakeUp = onWakeUp
}

// GetAllStates returns all registered disk states.
func (m *DiskPowerManager) GetAllStates() map[string]DiskPowerState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]DiskPowerState)
	for k, v := range m.states {
		result[k] = v
	}
	return result
}

// GetDiskCount returns the number of registered disks.
func (m *DiskPowerManager) GetDiskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.configs)
}