// Package smarthomehub 提供智能家居控制中心功能
// 学习 Home Assistant 与群晖 DSM 智能家居集成
// 支持 Matter、HomeKit、Zigbee、Z-Wave 多协议

package smarthomehub

import (
	"fmt"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// DeviceProtocol 设备协议
type DeviceProtocol string

const (
	ProtocolMatter   DeviceProtocol = "matter"
	ProtocolHomeKit  DeviceProtocol = "homekit"
	ProtocolZigbee   DeviceProtocol = "zigbee"
	ProtocolZWave    DeviceProtocol = "zwave"
	ProtocolWiFi     DeviceProtocol = "wifi"
	ProtocolBluetooth DeviceProtocol = "bluetooth"
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeLight      DeviceType = "light"
	DeviceTypeSwitch     DeviceType = "switch"
	DeviceTypeSensor     DeviceType = "sensor"
	DeviceTypeThermostat DeviceType = "thermostat"
	DeviceTypeLock       DeviceType = "lock"
	DeviceTypeCamera     DeviceType = "camera"
	DeviceTypeSpeaker    DeviceType = "speaker"
	DeviceTypeBlind      DeviceType = "blind"
	DeviceTypeFan        DeviceType = "fan"
	DeviceTypeOutlet     DeviceType = "outlet"
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusBusy    DeviceStatus = "busy"
	DeviceStatusError   DeviceStatus = "error"
)

// Device 智能设备
type Device struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        DeviceType        `json:"type"`
	Protocol    DeviceProtocol    `json:"protocol"`
	Room        string            `json:"room"`
	Status      DeviceStatus      `json:"status"`
	State       map[string]interface{} `json:"state"`
	Capabilities []string         `json:"capabilities"`
	Firmware    string            `json:"firmware"`
	LastSeen    time.Time         `json:"last_seen"`
	Battery     int               `json:"battery"`
	Signal      int               `json:"signal"`
	Metadata    map[string]string `json:"metadata"`
}

// Room 房间
type Room struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Icon      string   `json:"icon"`
	Devices   []string `json:"devices"`
	Scenes    []string `json:"scenes"`
	Automations []string `json:"automations"`
}

// Scene 场景
type Scene struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Icon        string            `json:"icon"`
	Actions     []SceneAction     `json:"actions"`
	IsFavorite  bool              `json:"is_favorite"`
	LastUsed    *time.Time        `json:"last_used,omitempty"`
}

// SceneAction 场景动作
type SceneAction struct {
	DeviceID string                 `json:"device_id"`
	Action   string                 `json:"action"`
	Params   map[string]interface{} `json:"params"`
	Delay    int                    `json:"delay"`
}

// Automation 自动化
type Automation struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Enabled     bool                `json:"enabled"`
	Trigger     AutomationTrigger   `json:"trigger"`
	Conditions  []AutomationCondition `json:"conditions"`
	Actions     []AutomationAction  `json:"actions"`
	LastTrigger *time.Time          `json:"last_trigger,omitempty"`
	TriggerCount int                `json:"trigger_count"`
}

// AutomationTrigger 自动化触发器
type AutomationTrigger struct {
	Type      string            `json:"type"`
	DeviceID  string            `json:"device_id,omitempty"`
	Property  string            `json:"property,omitempty"`
	Value     interface{}       `json:"value,omitempty"`
	Schedule  string            `json:"schedule,omitempty"`
	Params    map[string]interface{} `json:"params,omitempty"`
}

// AutomationCondition 自动化条件
type AutomationCondition struct {
	Type     string      `json:"type"`
	DeviceID string      `json:"device_id,omitempty"`
	Property string      `json:"property,omitempty"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// AutomationAction 自动化动作
type AutomationAction struct {
	Type     string                 `json:"type"`
	DeviceID string                 `json:"device_id,omitempty"`
	Service  string                 `json:"service,omitempty"`
	Params   map[string]interface{} `json:"params,omitempty"`
	Delay    int                    `json:"delay,omitempty"`
}

// Schedule 定时任务
type Schedule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Cron      string    `json:"cron"`
	Enabled   bool      `json:"enabled"`
	Action    AutomationAction `json:"action"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	NextRun   *time.Time `json:"next_run,omitempty"`
}

// Manager 智能家居管理器
type Manager struct {
	mu           sync.RWMutex
	devices      map[string]*Device
	rooms        map[string]*Room
	scenes       map[string]*Scene
	automations  map[string]*Automation
	schedules    map[string]*Schedule
	protocols    map[DeviceProtocol]bool
	eventLog     []Event
	maxDevices   int
}

// Event 事件
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	DeviceID  string                 `json:"device_id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		devices:     make(map[string]*Device),
		rooms:       make(map[string]*Room),
		scenes:      make(map[string]*Scene),
		automations: make(map[string]*Automation),
		schedules:   make(map[string]*Schedule),
		protocols: map[DeviceProtocol]bool{
			ProtocolMatter:   true,
			ProtocolHomeKit:  true,
			ProtocolZigbee:   true,
			ProtocolZWave:    true,
			ProtocolWiFi:     true,
			ProtocolBluetooth: true,
		},
		eventLog:   make([]Event, 0),
		maxDevices: 500,
	}
}

// RegisterDevice 注册设备
func (m *Manager) RegisterDevice(device *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.devices) >= m.maxDevices {
		return fmt.Errorf("已达到最大设备数: %d", m.maxDevices)
	}

	if !m.protocols[device.Protocol] {
		return fmt.Errorf("不支持的协议: %s", device.Protocol)
	}

	device.LastSeen = time.Now()
	if device.Status == "" {
		device.Status = DeviceStatusOnline
	}
	if device.State == nil {
		device.State = make(map[string]interface{})
	}
	if device.Metadata == nil {
		device.Metadata = make(map[string]string)
	}

	m.devices[device.ID] = device
	return nil
}

// UpdateDeviceState 更新设备状态
func (m *Manager) UpdateDeviceState(deviceID string, state map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	for k, v := range state {
		device.State[k] = v
	}
	device.LastSeen = time.Now()

	return nil
}

// CreateRoom 创建房间
func (m *Manager) CreateRoom(room *Room) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rooms[room.ID] = room
	return nil
}

// CreateScene 创建场景
func (m *Manager) CreateScene(scene *Scene) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.scenes[scene.ID] = scene
	return nil
}

// ActivateScene 激活场景
func (m *Manager) ActivateScene(sceneID string) error {
	m.mu.RLock()
	scene, exists := m.scenes[sceneID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("场景不存在: %s", sceneID)
	}

	now := time.Now()
	scene.LastUsed = &now

	m.addEvent("scene_activated", "", map[string]interface{}{
		"scene_id": sceneID,
		"name":     scene.Name,
	})

	return nil
}

// CreateAutomation 创建自动化
func (m *Manager) CreateAutomation(automation *Automation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.automations[automation.ID] = automation
	return nil
}

// GetDevice 获取设备
func (m *Manager) GetDevice(deviceID string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	return device, nil
}

// ListDevices 列出设备
func (m *Manager) ListDevices(room string, deviceType DeviceType) []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var devices []*Device
	for _, d := range m.devices {
		if (room == "" || d.Room == room) && (deviceType == "" || d.Type == deviceType) {
			devices = append(devices, d)
		}
	}

	return devices
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_devices":  len(m.devices),
		"online_devices": 0,
		"rooms":          len(m.rooms),
		"scenes":         len(m.scenes),
		"automations":    len(m.automations),
		"schedules":      len(m.schedules),
		"protocols":      make(map[string]int),
	}

	protocols := stats["protocols"].(map[string]int)
	for _, d := range m.devices {
		if d.Status == DeviceStatusOnline {
			stats["online_devices"] = stats["online_devices"].(int) + 1
		}
		protocols[string(d.Protocol)]++
	}

	return stats
}

func (m *Manager) addEvent(eventType string, deviceID string, data map[string]interface{}) {
	m.eventLog = append(m.eventLog, Event{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:      eventType,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Data:      data,
	})
}

// Close 关闭管理器
func (m *Manager) Close() error {
	return nil
}
