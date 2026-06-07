// Package homesec 提供家庭安防系统核心管理逻辑
package homesec

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 家庭安防管理器
type Manager struct {
	mu        sync.RWMutex
	devices   map[string]*Device
	zones     map[string]*Zone
	events    []*Event
	rules     map[string]*AlarmRule
	schedules map[string]*Schedule
	panel     *Panel
	armCode   string // 默认安防码
}

// NewManager 创建家庭安防管理器
func NewManager() *Manager {
	m := &Manager{
		devices:   make(map[string]*Device),
		zones:     make(map[string]*Zone),
		events:    make([]*Event, 0),
		rules:     make(map[string]*AlarmRule),
		schedules: make(map[string]*Schedule),
		armCode:   "1234",
		panel: &Panel{
			ID:        "panel-main",
			Name:      "主安防面板",
			Status:    PanelDisarmed,
			ZoneIDs:   []string{},
			UpdatedAt: time.Now(),
		},
	}

	// 初始化默认报警规则
	m.initDefaultRules()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// initDefaultRules 初始化默认报警规则
func (m *Manager) initDefaultRules() {
	defaultRules := []*AlarmRule{
		{
			ID:   "rule-intrusion",
			Name: "入侵报警",
			Conditions: []Condition{
				{DeviceType: DeviceDoorWindow, Status: StatusTriggered},
				{DeviceType: DeviceMotion, Status: StatusTriggered},
			},
			Actions: []Action{
				{ID: generateID(), Type: ActionSiren, Target: "siren-main"},
				{ID: generateID(), Type: ActionNotify, Target: "admin"},
				{ID: generateID(), Type: ActionCamera, Target: "camera-front"},
			},
			Enabled:   true,
			Priority:  10,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:   "rule-fire",
			Name: "火灾报警",
			Conditions: []Condition{
				{DeviceType: DeviceSmoke, Status: StatusTriggered},
			},
			Actions: []Action{
				{ID: generateID(), Type: ActionSiren, Target: "siren-main"},
				{ID: generateID(), Type: ActionNotify, Target: "admin"},
				{ID: generateID(), Type: ActionWebhook, Target: "fire-dept"},
			},
			Enabled:   true,
			Priority:  10,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:   "rule-water",
			Name: "水浸报警",
			Conditions: []Condition{
				{DeviceType: DeviceWater, Status: StatusTriggered},
			},
			Actions: []Action{
				{ID: generateID(), Type: ActionNotify, Target: "admin"},
				{ID: generateID(), Type: ActionWebhook, Target: "plumber"},
			},
			Enabled:   true,
			Priority:  5,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, rule := range defaultRules {
		m.rules[rule.ID] = rule
	}
}

// AddDevice 添加设备
func (m *Manager) AddDevice(device *Device) (*Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		device.ID = generateID()
	}

	if _, exists := m.devices[device.ID]; exists {
		return nil, fmt.Errorf("设备 %s 已存在", device.ID)
	}

	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()

	if device.Status == "" {
		device.Status = StatusDisarmed
	}

	m.devices[device.ID] = device
	return device, nil
}

// UpdateDevice 更新设备
func (m *Manager) UpdateDevice(id string, device *Device) (*Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.devices[id]
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", id)
	}

	device.ID = id
	device.CreatedAt = existing.CreatedAt
	device.UpdatedAt = time.Now()

	m.devices[id] = device
	return device, nil
}

// DeleteDevice 删除设备
func (m *Manager) DeleteDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[id]; !exists {
		return fmt.Errorf("设备 %s 不存在", id)
	}

	delete(m.devices, id)
	return nil
}

// GetDevice 获取设备
func (m *Manager) GetDevice(id string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[id]
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", id)
	}

	return device, nil
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices() ([]Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]Device, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, *device)
	}

	return devices, nil
}

// CreateZone 创建区域
func (m *Manager) CreateZone(zone *Zone) (*Zone, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if zone.ID == "" {
		zone.ID = generateID()
	}

	if _, exists := m.zones[zone.ID]; exists {
		return nil, fmt.Errorf("区域 %s 已存在", zone.ID)
	}

	zone.CreatedAt = time.Now()
	zone.UpdatedAt = time.Now()

	m.zones[zone.ID] = zone

	// 更新面板的区域列表
	m.panel.ZoneIDs = append(m.panel.ZoneIDs, zone.ID)
	m.panel.UpdatedAt = time.Now()

	return zone, nil
}

// UpdateZone 更新区域
func (m *Manager) UpdateZone(id string, zone *Zone) (*Zone, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.zones[id]
	if !exists {
		return nil, fmt.Errorf("区域 %s 不存在", id)
	}

	zone.ID = id
	zone.CreatedAt = existing.CreatedAt
	zone.UpdatedAt = time.Now()

	m.zones[id] = zone
	return zone, nil
}

// DeleteZone 删除区域
func (m *Manager) DeleteZone(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.zones[id]; !exists {
		return fmt.Errorf("区域 %s 不存在", id)
	}

	delete(m.zones, id)

	// 从面板中移除
	for i, zoneID := range m.panel.ZoneIDs {
		if zoneID == id {
			m.panel.ZoneIDs = append(m.panel.ZoneIDs[:i], m.panel.ZoneIDs[i+1:]...)
			break
		}
	}
	m.panel.UpdatedAt = time.Now()

	return nil
}

// ArmZone 布防区域
func (m *Manager) ArmZone(zoneID string, mode ArmMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	zone, exists := m.zones[zoneID]
	if !exists {
		return fmt.Errorf("区域 %s 不存在", zoneID)
	}

	zone.Armed = true
	zone.UpdatedAt = time.Now()

	// 记录布防事件
	event := &Event{
		ID:        generateID(),
		ZoneID:    zoneID,
		Type:      EventArm,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("区域 %s 已布防 (%s模式)", zone.Name, mode),
		Severity:  SeverityInfo,
	}
	m.events = append(m.events, event)

	// 更新面板状态
	m.updatePanelStatus()

	return nil
}

// DisarmZone 撤防区域
func (m *Manager) DisarmZone(zoneID string, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if code != m.armCode {
		return fmt.Errorf("安防码错误")
	}

	zone, exists := m.zones[zoneID]
	if !exists {
		return fmt.Errorf("区域 %s 不存在", zoneID)
	}

	zone.Armed = false
	zone.Bypass = false
	zone.UpdatedAt = time.Now()

	// 记录撤防事件
	event := &Event{
		ID:        generateID(),
		ZoneID:    zoneID,
		Type:      EventDisarm,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("区域 %s 已撤防", zone.Name),
		Severity:  SeverityInfo,
	}
	m.events = append(m.events, event)

	// 更新面板状态
	m.updatePanelStatus()

	return nil
}

// BypassZone 绕过区域
func (m *Manager) BypassZone(zoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	zone, exists := m.zones[zoneID]
	if !exists {
		return fmt.Errorf("区域 %s 不存在", zoneID)
	}

	zone.Bypass = true
	zone.UpdatedAt = time.Now()

	return nil
}

// TriggerAlarm 触发报警
func (m *Manager) TriggerAlarm(deviceID string, eventType EventType) (*Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", deviceID)
	}

	// 更新设备状态
	device.Status = StatusTriggered
	device.UpdatedAt = time.Now()

	// 查找设备所在的区域
	zoneID := ""
	for _, zone := range m.zones {
		for _, id := range zone.DeviceIDs {
			if id == deviceID {
				zoneID = zone.ID
				break
			}
		}
		if zoneID != "" {
			break
		}
	}

	// 创建事件
	event := &Event{
		ID:        generateID(),
		DeviceID:  deviceID,
		ZoneID:    zoneID,
		Type:      eventType,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("设备 %s 触发报警", device.Name),
		Severity:  SeverityCritical,
	}
	m.events = append(m.events, event)

	return event, nil
}

// GetEvents 获取事件列表
func (m *Manager) GetEvents(zoneID string, from, to time.Time, limit int) ([]Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filtered := make([]Event, 0)
	for _, event := range m.events {
		if zoneID != "" && event.ZoneID != zoneID {
			continue
		}
		if !from.IsZero() && event.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && event.Timestamp.After(to) {
			continue
		}
		filtered = append(filtered, *event)
	}

	// 按时间倒序排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// 应用限制
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

// AcknowledgeEvent 确认事件
func (m *Manager) AcknowledgeEvent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, event := range m.events {
		if event.ID == id {
			event.Acked = true
			return nil
		}
	}

	return fmt.Errorf("事件 %s 不存在", id)
}

// CreateAlarmRule 创建报警规则
func (m *Manager) CreateAlarmRule(rule *AlarmRule) (*AlarmRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID()
	}

	if _, exists := m.rules[rule.ID]; exists {
		return nil, fmt.Errorf("规则 %s 已存在", rule.ID)
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.rules[rule.ID] = rule
	return rule, nil
}

// UpdateAlarmRule 更新报警规则
func (m *Manager) UpdateAlarmRule(id string, rule *AlarmRule) (*AlarmRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则 %s 不存在", id)
	}

	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()

	m.rules[id] = rule
	return rule, nil
}

// DeleteAlarmRule 删除报警规则
func (m *Manager) DeleteAlarmRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("规则 %s 不存在", id)
	}

	delete(m.rules, id)
	return nil
}

// EvaluateRules 评估规则
func (m *Manager) EvaluateRules(event *Event) ([]Action, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var actions []Action

	// 获取触发设备
	device, exists := m.devices[event.DeviceID]
	if !exists {
		return actions, nil
	}

	// 评估所有启用的规则
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}

		// 检查条件是否匹配
		match := false
		for _, condition := range rule.Conditions {
			if condition.DeviceType == device.Type && condition.Status == device.Status {
				match = true
				break
			}
		}

		if match {
			actions = append(actions, rule.Actions...)
		}
	}

	return actions, nil
}

// ExecuteAction 执行动作
func (m *Manager) ExecuteAction(action Action) error {
	// 这里是动作执行的占位实现
	// 实际应该调用相应的硬件接口或服务
	switch action.Type {
	case ActionNotify:
		// 发送通知
	case ActionSiren:
		// 触发警报器
	case ActionLight:
		// 控制灯光
	case ActionCamera:
		// 控制摄像头
	case ActionSnapshot:
		// 拍照快照
	case ActionWebhook:
		// 调用 Webhook
	}

	return nil
}

// CreateSchedule 创建布防计划
func (m *Manager) CreateSchedule(schedule *Schedule) (*Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if schedule.ID == "" {
		schedule.ID = generateID()
	}

	if _, exists := m.schedules[schedule.ID]; exists {
		return nil, fmt.Errorf("计划 %s 已存在", schedule.ID)
	}

	schedule.CreatedAt = time.Now()
	schedule.UpdatedAt = time.Now()

	m.schedules[schedule.ID] = schedule
	return schedule, nil
}

// UpdateSchedule 更新布防计划
func (m *Manager) UpdateSchedule(id string, schedule *Schedule) (*Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.schedules[id]
	if !exists {
		return nil, fmt.Errorf("计划 %s 不存在", id)
	}

	schedule.ID = id
	schedule.CreatedAt = existing.CreatedAt
	schedule.UpdatedAt = time.Now()

	m.schedules[id] = schedule
	return schedule, nil
}

// DeleteSchedule 删除布防计划
func (m *Manager) DeleteSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.schedules[id]; !exists {
		return fmt.Errorf("计划 %s 不存在", id)
	}

	delete(m.schedules, id)
	return nil
}

// CheckSchedules 检查并执行布防计划
func (m *Manager) CheckSchedules() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	currentTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
	currentDay := strings.ToLower(now.Weekday().String())

	for _, schedule := range m.schedules {
		if !schedule.Enabled {
			continue
		}

		// 检查今天是否在计划中
		dayMatch := false
		for _, day := range schedule.Days {
			if strings.ToLower(day) == currentDay {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			continue
		}

		// 检查是否到达布防时间
		if currentTime == schedule.ArmTime {
			for _, zoneID := range schedule.ZoneIDs {
				if zone, exists := m.zones[zoneID]; exists {
					zone.Armed = true
					zone.UpdatedAt = now

					event := &Event{
						ID:        generateID(),
						ZoneID:    zoneID,
						Type:      EventArm,
						Timestamp: now,
						Message:   fmt.Sprintf("区域 %s 按计划自动布防", zone.Name),
						Severity:  SeverityInfo,
					}
					m.events = append(m.events, event)
				}
			}
		}

		// 检查是否到达撤防时间
		if currentTime == schedule.DisarmTime {
			for _, zoneID := range schedule.ZoneIDs {
				if zone, exists := m.zones[zoneID]; exists {
					zone.Armed = false
					zone.Bypass = false
					zone.UpdatedAt = now

					event := &Event{
						ID:        generateID(),
						ZoneID:    zoneID,
						Type:      EventDisarm,
						Timestamp: now,
						Message:   fmt.Sprintf("区域 %s 按计划自动撤防", zone.Name),
						Severity:  SeverityInfo,
					}
					m.events = append(m.events, event)
				}
			}
		}
	}

	return nil
}

// GetPanelStatus 获取面板状态
func (m *Manager) GetPanelStatus() (*Panel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.panel, nil
}

// ArmAll 全部布防
func (m *Manager) ArmAll(mode ArmMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, zone := range m.zones {
		if zone.Type != Zone24H { // 24小时区域始终布防
			zone.Armed = true
			zone.UpdatedAt = time.Now()
		}
	}

	// 记录事件
	event := &Event{
		ID:        generateID(),
		Type:      EventArm,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("全部区域已布防 (%s模式)", mode),
		Severity:  SeverityInfo,
	}
	m.events = append(m.events, event)

	// 更新面板状态
	m.updatePanelStatus()

	return nil
}

// DisarmAll 全部撤防
func (m *Manager) DisarmAll(code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if code != m.armCode {
		return fmt.Errorf("安防码错误")
	}

	for _, zone := range m.zones {
		if zone.Type != Zone24H { // 24小时区域不撤防
			zone.Armed = false
			zone.Bypass = false
			zone.UpdatedAt = time.Now()
		}
	}

	// 记录事件
	event := &Event{
		ID:        generateID(),
		Type:      EventDisarm,
		Timestamp: time.Now(),
		Message:   "全部区域已撤防",
		Severity:  SeverityInfo,
	}
	m.events = append(m.events, event)

	// 更新面板状态
	m.updatePanelStatus()

	return nil
}

// updatePanelStatus 更新面板状态
func (m *Manager) updatePanelStatus() {
	armedCount := 0
	totalCount := 0

	for _, zone := range m.zones {
		if zone.Type != Zone24H {
			totalCount++
			if zone.Armed {
				armedCount++
			}
		}
	}

	if totalCount == 0 {
		m.panel.Status = PanelDisarmed
	} else if armedCount == totalCount {
		m.panel.Status = PanelArmedAway
	} else if armedCount > 0 {
		m.panel.Status = PanelArmedHome
	} else {
		m.panel.Status = PanelDisarmed
	}

	m.panel.UpdatedAt = time.Now()
}

// GetSecurityScore 获取安防评分
func (m *Manager) GetSecurityScore() (int, map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	details := make(map[string]interface{})
	score := 100

	// 检查设备状态
	deviceIssues := 0
	for _, device := range m.devices {
		if !device.Enabled {
			deviceIssues++
		}
		if device.Battery < 20 {
			deviceIssues++
		}
		if device.Status == StatusTampered {
			deviceIssues += 2
		}
	}
	details["device_issues"] = deviceIssues
	score -= deviceIssues * 5

	// 检查区域布防状态
	unarmedZones := 0
	for _, zone := range m.zones {
		if !zone.Armed && zone.Type != Zone24H {
			unarmedZones++
		}
	}
	details["unarmed_zones"] = unarmedZones
	score -= unarmedZones * 10

	// 检查未确认事件
	unackedEvents := 0
	for _, event := range m.events {
		if !event.Acked && event.Severity == SeverityCritical {
			unackedEvents++
		}
	}
	details["unacked_critical_events"] = unackedEvents
	score -= unackedEvents * 15

	// 检查规则启用状态
	disabledRules := 0
	for _, rule := range m.rules {
		if !rule.Enabled {
			disabledRules++
		}
	}
	details["disabled_rules"] = disabledRules
	score -= disabledRules * 5

	// 确保分数在 0-100 范围内
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	details["total_devices"] = len(m.devices)
	details["total_zones"] = len(m.zones)
	details["total_rules"] = len(m.rules)
	details["panel_status"] = m.panel.Status

	return score, details, nil
}
