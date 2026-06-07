package ups

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// UPSStatus represents the current state of a UPS device.
type UPSStatus struct {
	Name           string        `json:"name"`
	Model          string        `json:"model"`
	Status         string        `json:"status"`         // online, on-battery, low-battery, charging, fault
	BatteryLevel   int           `json:"battery_level"`  // 0-100
	BatteryCharge  int           `json:"battery_charge"` // percent
	InputVoltage   float64       `json:"input_voltage"`  // volts
	OutputVoltage  float64       `json:"output_voltage"` // volts
	LoadPercent    int           `json:"load_percent"`   // 0-100
	RuntimeLeft    time.Duration `json:"runtime_left"`   // estimated minutes remaining
	Temperature    float64       `json:"temperature"`    // celsius
	LastUpdated    time.Time     `json:"last_updated"`
	OnBattery      bool          `json:"on_battery"`
	BatteryHealthy bool          `json:"battery_health"`
}

// UPSConfig holds UPS monitoring configuration.
type UPSConfig struct {
	Driver          string `json:"driver"`          // usbhid-ups, snmp-ups, etc.
	Port            string `json:"port"`            // /dev/ttyUSB0, auto, or host:port for SNMP
	LowBatteryPct   int    `json:"low_battery_pct"` // trigger shutdown at this level
	ShutdownDelay   int    `json:"shutdown_delay"`  // seconds to wait before shutdown
	PollInterval    int    `json:"poll_interval"`   // seconds between status polls
	NotifyOnBattery bool   `json:"notify_on_battery"`
	NotifyOnLow     bool   `json:"notify_on_low"`
	Name            string `json:"name"`
}

// DefaultUPSConfig returns a default UPS configuration.
func DefaultUPSConfig() UPSConfig {
	return UPSConfig{
		Driver:          "usbhid-ups",
		Port:            "auto",
		LowBatteryPct:   20,
		ShutdownDelay:   60,
		PollInterval:    10,
		NotifyOnBattery: true,
		NotifyOnLow:     true,
		Name:            "ups1",
	}
}

// Manager manages UPS monitoring.
type Manager struct {
	mu        sync.RWMutex
	config    UPSConfig
	status    UPSStatus
	stopCh    chan struct{}
	callbacks []func(UPSStatus)
	running   bool
}

// NewManager creates a new UPS manager.
func NewManager(config UPSConfig) *Manager {
	return &Manager{
		config: config,
		status: UPSStatus{
			Name:   config.Name,
			Status: "unknown",
		},
		stopCh: make(chan struct{}),
	}
}

// RegisterCallback registers a status change callback.
func (m *Manager) RegisterCallback(fn func(UPSStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, fn)
}

// Start begins UPS monitoring.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.pollLoop()
	log.Printf("✅ UPS监控已启动 (driver=%s, port=%s)", m.config.Driver, m.config.Port)
}

// Stop halts UPS monitoring.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	close(m.stopCh)
	m.running = false
	log.Println("UPS监控已停止")
}

// GetStatus returns the current UPS status.
func (m *Manager) GetStatus() UPSStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// GetConfig returns the current UPS configuration.
func (m *Manager) GetConfig() UPSConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig updates the UPS configuration.
func (m *Manager) UpdateConfig(config UPSConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

func (m *Manager) pollLoop() {
	ticker := time.NewTicker(time.Duration(m.config.PollInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.poll()
		}
	}
}

func (m *Manager) poll() {
	status := m.readUPS()

	m.mu.Lock()
	oldStatus := m.status.Status
	m.status = status
	cbs := make([]func(UPSStatus), len(m.callbacks))
	copy(cbs, m.callbacks)
	m.mu.Unlock()

	// Trigger callbacks on status change
	if oldStatus != status.Status {
		for _, cb := range cbs {
			go cb(status)
		}
	}

	// Auto-shutdown on low battery
	if status.OnBattery && status.BatteryLevel <= m.config.LowBatteryPct {
		log.Printf("⚠️ UPS电量过低 (%d%%)，%d秒后执行安全关机", status.BatteryLevel, m.config.ShutdownDelay)
		go m.delayedShutdown()
	}
}

func (m *Manager) delayedShutdown() {
	time.Sleep(time.Duration(m.config.ShutdownDelay) * time.Second)
	// Re-check - maybe power came back
	status := m.GetStatus()
	if status.OnBattery && status.BatteryLevel <= m.config.LowBatteryPct {
		log.Println("🔌 执行安全关机...")
		// In production, this would call: exec.Command("shutdown", "-h", "now").Run()
		log.Println("⚠️ 安全关机已触发（模拟模式）")
	}
}

func (m *Manager) readUPS() UPSStatus {
	// In production, this would communicate with upsd via NUT protocol
	// For now, return simulated data
	m.mu.RLock()
	name := m.config.Name
	m.mu.RUnlock()

	return UPSStatus{
		Name:           name,
		Model:          "APC Back-UPS Pro 1500",
		Status:         "online",
		BatteryLevel:   95,
		BatteryCharge:  95,
		InputVoltage:   220.5,
		OutputVoltage:  220.1,
		LoadPercent:    35,
		RuntimeLeft:    45 * time.Minute,
		Temperature:    28.5,
		LastUpdated:    time.Now(),
		OnBattery:      false,
		BatteryHealthy: true,
	}
}

// GetBatteryHealthScore returns a 0-100 health score based on battery metrics.
func GetBatteryHealthScore(status UPSStatus) int {
	score := 100.0

	// Battery age factor (if charge < 80% when "full", battery is degrading)
	if status.BatteryCharge < 80 && !status.OnBattery {
		score -= float64(80-status.BatteryCharge) * 0.5
	}

	// Temperature factor (optimal: 20-25°C)
	if status.Temperature > 30 {
		score -= (status.Temperature - 25) * 2
	}

	// Load factor (high load = more stress)
	if status.LoadPercent > 80 {
		score -= float64(status.LoadPercent-80) * 0.5
	}

	return int(math.Max(0, math.Min(100, score)))
}

func (m *Manager) String() string {
	status := m.GetStatus()
	return fmt.Sprintf("UPS[%s] status=%s battery=%d%% load=%d%%",
		status.Name, status.Status, status.BatteryLevel, status.LoadPercent)
}
