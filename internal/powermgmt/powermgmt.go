// Package powermgmt 电源管理 - 定时开关机/休眠/WOL
// 对标飞牛fnOS电源管理，智能节能调度
package powermgmt

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// PowerState 电源状态
type PowerState string

const (
	PowerOn        PowerState = "on"
	PowerOff       PowerState = "off"
	PowerSleep     PowerState = "sleep"
	PowerHibernate PowerState = "hibernate"
	PowerWaking    PowerState = "waking"
)

// ScheduleType 调度类型
type ScheduleType string

const (
	ScheduleOnce    ScheduleType = "once"
	ScheduleDaily   ScheduleType = "daily"
	ScheduleWeekly  ScheduleType = "weekly"
	ScheduleMonthly ScheduleType = "monthly"
)

// ActionType 动作类型
type ActionType string

const (
	ActionPowerOn   ActionType = "power_on"
	ActionPowerOff  ActionType = "power_off"
	ActionSleep     ActionType = "sleep"
	ActionHibernate ActionType = "hibernate"
	ActionReboot    ActionType = "reboot"
	ActionWakeOnLan ActionType = "wake_on_lan"
)

// WakeTarget WOL 目标设备
type WakeTarget struct {
	Name       string `json:"name"`
	MACAddress string `json:"mac_address"`
	IPAddress  string `json:"ip_address"`
	Broadcast  string `json:"broadcast"`
	Port       int    `json:"port"`
	Enabled    bool   `json:"enabled"`
}

// PowerSchedule 电源调度任务
type PowerSchedule struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      ScheduleType   `json:"type"`
	Action    ActionType     `json:"action"`
	Enabled   bool           `json:"enabled"`
	Hour      int            `json:"hour"`
	Minute    int            `json:"minute"`
	Weekdays  []time.Weekday `json:"weekdays"`
	MonthDay  int            `json:"month_day"`
	NextRun   time.Time      `json:"next_run"`
	LastRun   time.Time      `json:"last_run"`
	RunCount  int            `json:"run_count"`
	LastError string         `json:"last_error"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// PowerEvent 电源事件
type PowerEvent struct {
	ID        string     `json:"id"`
	Timestamp time.Time  `json:"timestamp"`
	Action    ActionType `json:"action"`
	Source    string     `json:"source"` // schedule/user/wol
	Success   bool       `json:"success"`
	Error     string     `json:"error"`
	StateFrom PowerState `json:"state_from"`
	StateTo   PowerState `json:"state_to"`
}

// IdleConfig 空闲配置
type IdleConfig struct {
	Enabled         bool          `json:"enabled"`
	IdleTimeout     time.Duration `json:"idle_timeout"`
	IdleAction      ActionType    `json:"idle_action"`
	MonitorCPU      bool          `json:"monitor_cpu"`
	MonitorDisk     bool          `json:"monitor_disk"`
	MonitorNetwork  bool          `json:"monitor_network"`
	CPUThreshold    float64       `json:"cpu_threshold"`     // below this = idle
	DiskIOThreshold int64         `json:"disk_io_threshold"` // bytes/s
	NetIOThreshold  int64         `json:"net_io_threshold"`  // bytes/s
}

// Manager 电源管理器
type Manager struct {
	mu            sync.RWMutex
	state         PowerState
	schedules     map[string]*PowerSchedule
	wakeTargets   map[string]*WakeTarget
	events        []PowerEvent
	idleConfig    IdleConfig
	maxEvents     int
	startTime     time.Time
	onStateChange func(PowerState, PowerState)
}

// NewManager 创建电源管理器
func NewManager() *Manager {
	return &Manager{
		state:       PowerOn,
		schedules:   make(map[string]*PowerSchedule),
		wakeTargets: make(map[string]*WakeTarget),
		events:      make([]PowerEvent, 0),
		maxEvents:   10000,
		startTime:   time.Now(),
	}
}

// GetState 获取当前电源状态
func (m *Manager) GetState() PowerState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// SetState 设置电源状态
func (m *Manager) SetState(state PowerState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state == m.state {
		return nil
	}

	oldState := m.state
	m.state = state
	m.addEvent(ActionType("state_change"), "user", true, "", oldState, state)

	if m.onStateChange != nil {
		go m.onStateChange(oldState, state)
	}
	return nil
}

// SetStateChangeCallback 设置状态变更回调
func (m *Manager) SetStateChangeCallback(cb func(PowerState, PowerState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStateChange = cb
}

// AddSchedule 添加调度任务
func (m *Manager) AddSchedule(schedule PowerSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if schedule.ID == "" {
		return errors.New("schedule ID required")
	}
	if _, exists := m.schedules[schedule.ID]; exists {
		return errors.New("schedule already exists")
	}

	schedule.CreatedAt = time.Now()
	schedule.UpdatedAt = time.Now()
	schedule.NextRun = m.calculateNextRun(schedule)
	m.schedules[schedule.ID] = &schedule
	return nil
}

// RemoveSchedule 移除调度任务
func (m *Manager) RemoveSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.schedules[id]; !exists {
		return errors.New("schedule not found")
	}

	delete(m.schedules, id)
	return nil
}

// UpdateSchedule 更新调度任务
func (m *Manager) UpdateSchedule(id string, update func(*PowerSchedule)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	schedule, exists := m.schedules[id]
	if !exists {
		return errors.New("schedule not found")
	}

	update(schedule)
	schedule.UpdatedAt = time.Now()
	schedule.NextRun = m.calculateNextRun(*schedule)
	return nil
}

// GetSchedule 获取调度任务
func (m *Manager) GetSchedule(id string) (*PowerSchedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedule, exists := m.schedules[id]
	if !exists {
		return nil, errors.New("schedule not found")
	}
	return schedule, nil
}

// ListSchedules 列出所有调度任务
func (m *Manager) ListSchedules() []PowerSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedules := make([]PowerSchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, *s)
	}
	return schedules
}

// EnableSchedule 启用调度
func (m *Manager) EnableSchedule(id string) error {
	return m.UpdateSchedule(id, func(s *PowerSchedule) {
		s.Enabled = true
	})
}

// DisableSchedule 禁用调度
func (m *Manager) DisableSchedule(id string) error {
	return m.UpdateSchedule(id, func(s *PowerSchedule) {
		s.Enabled = false
	})
}

// AddWakeTarget 添加 WOL 目标
func (m *Manager) AddWakeTarget(target WakeTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if target.MACAddress == "" {
		return errors.New("MAC address required")
	}
	if _, exists := m.wakeTargets[target.MACAddress]; exists {
		return errors.New("target already exists")
	}

	if target.Port == 0 {
		target.Port = 9
	}
	if target.Broadcast == "" {
		target.Broadcast = "255.255.255.255"
	}

	m.wakeTargets[target.MACAddress] = &target
	return nil
}

// RemoveWakeTarget 移除 WOL 目标
func (m *Manager) RemoveWakeTarget(mac string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.wakeTargets[mac]; !exists {
		return errors.New("target not found")
	}

	delete(m.wakeTargets, mac)
	return nil
}

// ListWakeTargets 列出 WOL 目标
func (m *Manager) ListWakeTargets() []WakeTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := make([]WakeTarget, 0, len(m.wakeTargets))
	for _, t := range m.wakeTargets {
		targets = append(targets, *t)
	}
	return targets
}

// WakeDevice 唤醒设备
func (m *Manager) WakeDevice(mac string) error {
	m.mu.RLock()
	target, exists := m.wakeTargets[mac]
	m.mu.RUnlock()

	if !exists {
		return errors.New("target not found")
	}
	if !target.Enabled {
		return errors.New("target disabled")
	}

	// In production, send actual WOL magic packet
	m.mu.Lock()
	m.addEvent(ActionWakeOnLan, "user", true, "", m.state, m.state)
	m.mu.Unlock()

	return nil
}

// SetIdleConfig 设置空闲配置
func (m *Manager) SetIdleConfig(config IdleConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idleConfig = config
}

// GetIdleConfig 获取空闲配置
func (m *Manager) GetIdleConfig() IdleConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.idleConfig
}

// GetEvents 获取事件日志
func (m *Manager) GetEvents(limit int) []PowerEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	start := len(m.events) - limit
	result := make([]PowerEvent, limit)
	copy(result, m.events[start:])
	return result
}

// ClearEvents 清除事件
func (m *Manager) ClearEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = m.events[:0]
}

// GetUptime 获取运行时间
func (m *Manager) GetUptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == PowerOff {
		return 0
	}
	return time.Since(m.startTime)
}

// ExportConfig 导出配置
func (m *Manager) ExportConfig() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := struct {
		State       PowerState                `json:"state"`
		Schedules   map[string]*PowerSchedule `json:"schedules"`
		WakeTargets map[string]*WakeTarget    `json:"wake_targets"`
		IdleConfig  IdleConfig                `json:"idle_config"`
	}{
		State:       m.state,
		Schedules:   m.schedules,
		WakeTargets: m.wakeTargets,
		IdleConfig:  m.idleConfig,
	}

	return json.MarshalIndent(config, "", "  ")
}

// ImportConfig 导入配置
func (m *Manager) ImportConfig(data []byte) error {
	var config struct {
		Schedules   map[string]*PowerSchedule `json:"schedules"`
		WakeTargets map[string]*WakeTarget    `json:"wake_targets"`
		IdleConfig  IdleConfig                `json:"idle_config"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if config.Schedules != nil {
		m.schedules = config.Schedules
	}
	if config.WakeTargets != nil {
		m.wakeTargets = config.WakeTargets
	}
	m.idleConfig = config.IdleConfig
	return nil
}

func (m *Manager) calculateNextRun(schedule PowerSchedule) time.Time {
	now := time.Now()
	hour, minute := schedule.Hour, schedule.Minute

	switch schedule.Type {
	case ScheduleOnce:
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next

	case ScheduleDaily:
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next

	case ScheduleWeekly:
		for _, wd := range schedule.Weekdays {
			daysUntil := int(wd - now.Weekday())
			if daysUntil <= 0 {
				daysUntil += 7
			}
			next := time.Date(now.Year(), now.Month(), now.Day()+daysUntil, hour, minute, 0, 0, now.Location())
			if next.After(now) {
				return next
			}
		}
		return now.Add(7 * 24 * time.Hour)

	case ScheduleMonthly:
		next := time.Date(now.Year(), now.Month(), schedule.MonthDay, hour, minute, 0, 0, now.Location())
		if next.Before(now) {
			next = next.AddDate(0, 1, 0)
		}
		return next
	}

	return now.Add(24 * time.Hour)
}

func (m *Manager) addEvent(action ActionType, source string, success bool, errMsg string, from, to PowerState) {
	event := PowerEvent{
		ID:        time.Now().Format("20060102150405.000000"),
		Timestamp: time.Now(),
		Action:    action,
		Source:    source,
		Success:   success,
		Error:     errMsg,
		StateFrom: from,
		StateTo:   to,
	}
	m.events = append(m.events, event)
	if len(m.events) > m.maxEvents {
		m.events = m.events[1:]
	}
}
