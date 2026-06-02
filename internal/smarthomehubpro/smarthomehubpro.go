// Package smarthomehubpro 智能家居Hub Pro
// 提供Matter/Thread/Zigbee/Z-Wave协议支持、设备自动化、场景联动、能源管理、安防集成
// 对标飞牛智能家居中心，支持主流智能家居协议
package smarthomehubpro

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Protocol 智能家居协议
type Protocol string

const (
	ProtocolMatter  Protocol = "matter"  // Matter协议
	ProtocolThread  Protocol = "thread"  // Thread协议
	ProtocolZigbee  Protocol = "zigbee"  // Zigbee协议
	ProtocolZWave   Protocol = "zwave"   // Z-Wave协议
	ProtocolWiFi    Protocol = "wifi"    // WiFi协议
	ProtocolBLE     Protocol = "ble"     // 蓝牙BLE
	ProtocolMQTT    Protocol = "mqtt"    // MQTT协议
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeLight      DeviceType = "light"      // 灯光
	DeviceTypeSwitch     DeviceType = "switch"     // 开关
	DeviceTypeSensor     DeviceType = "sensor"     // 传感器
	DeviceTypeThermostat DeviceType = "thermostat" // 温控器
	DeviceTypeLock       DeviceType = "lock"       // 门锁
	DeviceTypeCamera     DeviceType = "camera"     // 摄像头
	DeviceTypeSpeaker    DeviceType = "speaker"    // 音箱
	DeviceTypePlug       DeviceType = "plug"       // 智能插座
	DeviceTypeBlind      DeviceType = "blind"      // 窗帘
	DeviceTypeFan        DeviceType = "fan"        // 风扇
	DeviceTypeAirCon     DeviceType = "aircon"     // 空调
	DeviceTypeTV         DeviceType = "tv"         // 电视
	DeviceTypeHub        DeviceType = "hub"        // 网关
	DeviceTypeOther      DeviceType = "other"      // 其他
)

// DeviceState 设备状态
type DeviceState string

const (
	StateOnline  DeviceState = "online"  // 在线
	StateOffline DeviceState = "offline" // 离线
	StateUnavail DeviceState = "unavailable" // 不可用
)

// TriggerType 触发器类型
type TriggerType string

const (
	TriggerTime     TriggerType = "time"     // 时间触发
	TriggerDevice   TriggerType = "device"   // 设备触发
	TriggerLocation TriggerType = "location" // 位置触发
	TriggerEvent    TriggerType = "event"    // 事件触发
	TriggerCondition TriggerType = "condition" // 条件触发
)

// ActionType 动作类型
type ActionType string

const (
	ActionDeviceControl ActionType = "device_control" // 设备控制
	ActionNotification  ActionType = "notification"   // 通知
	ActionScene         ActionType = "scene"          // 场景激活
	ActionDelay         ActionType = "delay"          // 延时
	ActionWebhook       ActionType = "webhook"        // Webhook调用
)

// Device 智能设备
type Device struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        DeviceType        `json:"type"`
	Protocol    Protocol          `json:"protocol"`
	Manufacturer string           `json:"manufacturer,omitempty"`
	Model       string            `json:"model,omitempty"`
	Firmware    string            `json:"firmware,omitempty"`
	State       DeviceState       `json:"state"`
	Room        string            `json:"room,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Capabilities []string         `json:"capabilities,omitempty"`
	Battery     *int              `json:"battery,omitempty"`
	LastSeen    time.Time         `json:"last_seen"`
	RegisteredAt time.Time        `json:"registered_at"`
}

// Room 房间
type Room struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Icon        string    `json:"icon,omitempty"`
	DeviceIDs   []string  `json:"device_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

// Scene 场景
type Scene struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Icon        string        `json:"icon,omitempty"`
	Actions     []*SceneAction `json:"actions"`
	Enabled     bool          `json:"enabled"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// SceneAction 场景动作
type SceneAction struct {
	DeviceID   string                 `json:"device_id"`
	Action     string                 `json:"action"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Delay      time.Duration          `json:"delay,omitempty"`
}

// Automation 自动化规则
type Automation struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Enabled     bool         `json:"enabled"`
	Trigger     *Trigger     `json:"trigger"`
	Conditions  []*Condition `json:"conditions,omitempty"`
	Actions     []*Action    `json:"actions"`
	LastTrigger *time.Time   `json:"last_triggered,omitempty"`
	RunCount    int64        `json:"run_count"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Trigger 触发器
type Trigger struct {
	Type       TriggerType          `json:"type"`
	DeviceID   string               `json:"device_id,omitempty"`
	Property   string               `json:"property,omitempty"`
	Value      interface{}          `json:"value,omitempty"`
	Schedule   string               `json:"schedule,omitempty"` // cron expression
	Zone       string               `json:"zone,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// Condition 条件
type Condition struct {
	Type     string      `json:"type"`     // device, time, state
	DeviceID string      `json:"device_id,omitempty"`
	Property string      `json:"property,omitempty"`
	Operator string      `json:"operator"` // eq, ne, gt, lt, gte, lte
	Value    interface{} `json:"value,omitempty"`
}

// Action 动作
type Action struct {
	Type       ActionType           `json:"type"`
	DeviceID   string               `json:"device_id,omitempty"`
	SceneID    string               `json:"scene_id,omitempty"`
	Command    string               `json:"command,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Delay      time.Duration        `json:"delay,omitempty"`
	Webhook    string               `json:"webhook,omitempty"`
}

// EnergyReading 能耗读数
type EnergyReading struct {
	DeviceID  string    `json:"device_id"`
	Power     float64   `json:"power"`      // 功率 W
	Energy    float64   `json:"energy"`     // 能耗 Wh
	Voltage   float64   `json:"voltage"`    // 电压 V
	Current   float64   `json:"current"`    // 电流 A
	Timestamp time.Time `json:"timestamp"`
}

// Hub 智能家居Hub
type Hub struct {
	mu           sync.RWMutex
	config       *Config
	devices      map[string]*Device
	rooms        map[string]*Room
	scenes       map[string]*Scene
	automations  map[string]*Automation
	energyLog    []*EnergyReading
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
}

// Config Hub配置
type Config struct {
	DiscoveryEnabled  bool          `json:"discovery_enabled"`
	DiscoveryInterval time.Duration `json:"discovery_interval"`
	MaxDevices        int           `json:"max_devices"`
	EnergyRetention   time.Duration `json:"energy_retention"`
	AutomationEnabled bool          `json:"automation_enabled"`
	MQTTEndpoint      string        `json:"mqtt_endpoint,omitempty"`
	MatterEnabled     bool          `json:"matter_enabled"`
	ThreadEnabled     bool          `json:"thread_enabled"`
	ZigbeeEnabled     bool          `json:"zigbee_enabled"`
	ZWaveEnabled      bool          `json:"zwave_enabled"`
}

// NewHub 创建新的智能家居Hub
func NewHub(config *Config) *Hub {
	if config == nil {
		config = &Config{
			DiscoveryEnabled:  true,
			DiscoveryInterval: time.Minute,
			MaxDevices:        500,
			EnergyRetention:   30 * 24 * time.Hour,
			AutomationEnabled: true,
			MatterEnabled:     true,
			ThreadEnabled:     true,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		config:      config,
		devices:     make(map[string]*Device),
		rooms:       make(map[string]*Room),
		scenes:      make(map[string]*Scene),
		automations: make(map[string]*Automation),
		energyLog:   make([]*EnergyReading, 0),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// AddDevice 添加设备
func (h *Hub) AddDevice(device *Device) error {
	if device == nil {
		return errors.New("device cannot be nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.devices[device.ID]; exists {
		return fmt.Errorf("device %s already exists", device.ID)
	}

	device.RegisteredAt = time.Now()
	device.LastSeen = time.Now()
	if device.State == "" {
		device.State = StateOnline
	}
	h.devices[device.ID] = device
	return nil
}

// RemoveDevice 移除设备
func (h *Hub) RemoveDevice(deviceID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.devices[deviceID]; !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	delete(h.devices, deviceID)
	return nil
}

// GetDevice 获取设备
func (h *Hub) GetDevice(deviceID string) (*Device, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	device, exists := h.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}
	return device, nil
}

// ListDevices 列出所有设备
func (h *Hub) ListDevices() []*Device {
	h.mu.RLock()
	defer h.mu.RUnlock()

	devices := make([]*Device, 0, len(h.devices))
	for _, d := range h.devices {
		devices = append(devices, d)
	}
	return devices
}

// UpdateDeviceState 更新设备状态
func (h *Hub) UpdateDeviceState(deviceID string, state DeviceState, properties map[string]interface{}) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	device, exists := h.devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	device.State = state
	device.LastSeen = time.Now()
	if properties != nil {
		if device.Properties == nil {
			device.Properties = make(map[string]interface{})
		}
		for k, v := range properties {
			device.Properties[k] = v
		}
	}
	return nil
}

// AddRoom 添加房间
func (h *Hub) AddRoom(room *Room) error {
	if room == nil {
		return errors.New("room cannot be nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.rooms[room.ID]; exists {
		return fmt.Errorf("room %s already exists", room.ID)
	}

	room.CreatedAt = time.Now()
	h.rooms[room.ID] = room
	return nil
}

// AddScene 添加场景
func (h *Hub) AddScene(scene *Scene) error {
	if scene == nil {
		return errors.New("scene cannot be nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	scene.CreatedAt = time.Now()
	scene.UpdatedAt = time.Now()
	h.scenes[scene.ID] = scene
	return nil
}

// ActivateScene 激活场景
func (h *Hub) ActivateScene(sceneID string) error {
	h.mu.RLock()
	scene, exists := h.scenes[sceneID]
	h.mu.RUnlock()

	if !exists {
		return fmt.Errorf("scene %s not found", sceneID)
	}

	if !scene.Enabled {
		return fmt.Errorf("scene %s is disabled", sceneID)
	}

	// 执行场景动作
	for _, action := range scene.Actions {
		h.mu.RLock()
		device, exists := h.devices[action.DeviceID]
		h.mu.RUnlock()

		if exists && device.State == StateOnline {
			// 执行设备控制
			if device.Properties == nil {
				device.Properties = make(map[string]interface{})
			}
			for k, v := range action.Properties {
				device.Properties[k] = v
			}
		}
	}
	return nil
}

// AddAutomation 添加自动化规则
func (h *Hub) AddAutomation(automation *Automation) error {
	if automation == nil {
		return errors.New("automation cannot be nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	automation.CreatedAt = time.Now()
	automation.UpdatedAt = time.Now()
	h.automations[automation.ID] = automation
	return nil
}

// RecordEnergy 记录能耗
func (h *Hub) RecordEnergy(reading *EnergyReading) {
	h.mu.Lock()
	defer h.mu.Unlock()

	reading.Timestamp = time.Now()
	h.energyLog = append(h.energyLog, reading)
}

// GetEnergyStats 获取能耗统计
func (h *Hub) GetEnergyStats(deviceID string, duration time.Duration) (float64, float64) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var totalEnergy, maxPower float64

	for _, reading := range h.energyLog {
		if reading.DeviceID == deviceID && reading.Timestamp.After(cutoff) {
			totalEnergy += reading.Energy
			if reading.Power > maxPower {
				maxPower = reading.Power
			}
		}
	}
	return totalEnergy, maxPower
}

// GetOnlineDevices 获取在线设备数
func (h *Hub) GetOnlineDevices() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, d := range h.devices {
		if d.State == StateOnline {
			count++
		}
	}
	return count
}

// Start 启动Hub
func (h *Hub) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return errors.New("hub is already running")
	}
	h.running = true
	return nil
}

// Stop 停止Hub
func (h *Hub) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return errors.New("hub is not running")
	}
	h.running = false
	h.cancel()
	return nil
}
