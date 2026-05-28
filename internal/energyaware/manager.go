// Package energyaware 提供智能节能调度与功耗管理
// 对标群晖 HDD 休眠 + TrueNAS Power Management，增加 AI 调度
package energyaware

import (
	"context"
	"sync"
	"time"
)

// PowerState 电源状态
type PowerState string

const (
	StateActive    PowerState = "active"
	StateIdle      PowerState = "idle"
	StateStandby   PowerState = "standby"
	StateSleep     PowerState = "sleep"
	StateHibernate PowerState = "hibernate"
)

// DevicePower 设备功耗信息
type DevicePower struct {
	DeviceID    string      `json:"device_id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"` // disk/cpu/fan/psu
	State       PowerState  `json:"state"`
	Watts       float64     `json:"watts"`
	Temperature int         `json:"temperature"`
	LastActive  time.Time   `json:"last_active"`
}

// ScheduleRule 调度规则
type ScheduleRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Enabled     bool        `json:"enabled"`
	StartTime   string      `json:"start_time"` // HH:MM
	EndTime     string      `json:"end_time"`
	Days        []string    `json:"days"`        // mon/tue/wed/thu/fri/sat/sun
	TargetState PowerState  `json:"target_state"`
	Devices     []string    `json:"devices"`      // empty = all
	Priority    int         `json:"priority"`
}

// EnergyStats 能耗统计
type EnergyStats struct {
	TotalWatts     float64                `json:"total_watts"`
	DailyKWh       float64                `json:"daily_kwh"`
	MonthlyKWh     float64                `json:"monthly_kwh"`
	EstimatedCost  float64                `json:"estimated_cost"`
	Devices        []DevicePower          `json:"devices"`
	StateDist      map[PowerState]int     `json:"state_dist"`
	HourlyWatts    []float64              `json:"hourly_watts"`
	SavingTips     []string               `json:"saving_tips"`
	CO2Kg          float64                `json:"co2_kg"`
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	IdleTimeout     time.Duration `json:"idle_timeout"`
	StandbyTimeout  time.Duration `json:"standby_timeout"`
	ElectricityRate float64       `json:"electricity_rate"` // 元/kWh
	CO2Factor       float64       `json:"co2_factor"`       // kg CO2/kWh
	SmartSchedule   bool          `json:"smart_schedule"`
}

// DefaultManagerConfig 默认配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		IdleTimeout:     30 * time.Minute,
		StandbyTimeout:  2 * time.Hour,
		ElectricityRate: 0.55,
		CO2Factor:       0.5703,
		SmartSchedule:   true,
	}
}

// Manager 管理器
type Manager struct {
	config    *ManagerConfig
	devices   map[string]*DevicePower
	rules     []*ScheduleRule
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	stats     *EnergyStats
	wattLog   []float64
}

// NewManager 创建管理器
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:  config,
		devices: make(map[string]*DevicePower),
		rules:   make([]*ScheduleRule, 0),
		ctx:     ctx,
		cancel:  cancel,
		stats:   &EnergyStats{StateDist: make(map[PowerState]int)},
		wattLog: make([]float64, 24),
	}
}

// Start 启动管理器
func (m *Manager) Start() {
	go m.monitorLoop()
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()
}

// RegisterDevice 注册设备
func (m *Manager) RegisterDevice(device *DevicePower) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.devices[device.DeviceID] = device
	m.updateStats()
}

// UpdateDeviceState 更新设备状态
func (m *Manager) UpdateDeviceState(deviceID string, state PowerState, watts float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.devices[deviceID]; ok {
		d.State = state
		d.Watts = watts
		d.LastActive = time.Now()
		m.updateStats()
	}
}

// AddRule 添加调度规则
func (m *Manager) AddRule(rule *ScheduleRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
}

// GetStats 获取能耗统计
func (m *Manager) GetStats() *EnergyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// GetDevices 获取所有设备
func (m *Manager) GetDevices() []DevicePower {
	m.mu.RLock()
	defer m.mu.RUnlock()
	devices := make([]DevicePower, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, *d)
	}
	return devices
}

// GetRules 获取调度规则
func (m *Manager) GetRules() []*ScheduleRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rules := make([]*ScheduleRule, len(m.rules))
	copy(rules, m.rules)
	return rules
}

// RemoveRule 移除规则
func (m *Manager) RemoveRule(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, r := range m.rules {
		if r.ID == id {
			m.rules = append(m.rules[:i], m.rules[i+1:]...)
			return true
		}
	}
	return false
}

// updateStats 更新统计
func (m *Manager) updateStats() {
	stats := &EnergyStats{
		StateDist: make(map[PowerState]int),
		Devices:   make([]DevicePower, 0, len(m.devices)),
	}
	
	var totalWatts float64
	for _, d := range m.devices {
		stats.Devices = append(stats.Devices, *d)
		stats.StateDist[d.State]++
		totalWatts += d.Watts
	}
	
	stats.TotalWatts = totalWatts
	stats.DailyKWh = totalWatts * 24 / 1000
	stats.MonthlyKWh = stats.DailyKWh * 30
	stats.EstimatedCost = stats.MonthlyKWh * m.config.ElectricityRate
	stats.CO2Kg = stats.MonthlyKWh * m.config.CO2Factor
	
	// 节能建议
	stats.SavingTips = m.generateTips()
	
	// 记录小时功耗
	hour := time.Now().Hour()
	if hour < len(m.wattLog) {
		m.wattLog[hour] = totalWatts
	}
	stats.HourlyWatts = m.wattLog
	
	m.stats = stats
}

// generateTips 生成节能建议
func (m *Manager) generateTips() []string {
	var tips []string
	
	idleCount := 0
	for _, d := range m.devices {
		if d.State == StateIdle && time.Since(d.LastActive) > m.config.IdleTimeout {
			idleCount++
		}
	}
	
	if idleCount > 0 {
		tips = append(tips, "有空闲设备可进入待机模式以节省能耗")
	}
	
	totalWatts := m.stats.TotalWatts
	if totalWatts > 200 {
		tips = append(tips, "当前功耗较高，建议检查是否有不必要的服务运行")
	}
	
	if len(tips) == 0 {
		tips = append(tips, "当前能耗正常，继续监控")
	}
	
	return tips
}

// monitorLoop 监控循环
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkIdleDevices()
		}
	}
}

// checkIdleDevices 检查空闲设备
func (m *Manager) checkIdleDevices() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, d := range m.devices {
		if d.State == StateActive && time.Since(d.LastActive) > m.config.IdleTimeout {
			d.State = StateIdle
		}
		if d.State == StateIdle && time.Since(d.LastActive) > m.config.StandbyTimeout {
			d.State = StateStandby
			d.Watts = d.Watts * 0.3 // 待机功耗约为 30%
		}
	}
	m.updateStats()
}

// SetDeviceActive 设置设备为活跃状态
func (m *Manager) SetDeviceActive(deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.devices[deviceID]; ok {
		d.State = StateActive
		d.LastActive = time.Now()
	}
}
