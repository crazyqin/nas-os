// Package smarthome provides smart home integration for NAS-OS
// Features: Matter/HomeKit/Home Assistant integration, device management, automation
// Competitor benchmark: 对标群晖Home Assistant集成, 超越飞牛智能家居
package smarthome

import (
	"context"
	"sync"
	"time"
)

// ============================================================
// 协议与设备类型
// ============================================================

// Protocol represents smart home protocol
type Protocol string

const (
	ProtocolMatter    Protocol = "matter"
	ProtocolHomeKit   Protocol = "homekit"
	ProtocolZigbee    Protocol = "zigbee"
	ProtocolZWave     Protocol = "zwave"
	ProtocolMQTT      Protocol = "mqtt"
	ProtocolHTTP      Protocol = "http"
	ProtocolWiFi      Protocol = "wifi"
	ProtocolBluetooth Protocol = "bluetooth"
)

// DeviceType represents the type of smart device
type DeviceType string

const (
	DeviceTypeLight      DeviceType = "light"
	DeviceTypeSwitch     DeviceType = "switch"
	DeviceTypeSensor     DeviceType = "sensor"
	DeviceTypeThermostat DeviceType = "thermostat"
	DeviceTypeCamera     DeviceType = "camera"
	DeviceTypeLock       DeviceType = "lock"
	DeviceTypePlug       DeviceType = "plug"
	DeviceTypeFan        DeviceType = "fan"
	DeviceTypeCurtain    DeviceType = "curtain"
	DeviceTypeSpeaker    DeviceType = "speaker"
	DeviceTypeOther      DeviceType = "other"
)

// DeviceStatus represents device online status
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusUnknown DeviceStatus = "unknown"
)

// ============================================================
// 触发器与动作类型
// ============================================================

// TriggerType represents the type of automation trigger
type TriggerType string

const (
	TriggerTypeDevice  TriggerType = "device"
	TriggerTypeTime    TriggerType = "time"
	TriggerTypeSunrise TriggerType = "sunrise"
	TriggerTypeSunset  TriggerType = "sunset"
	TriggerTypeManual  TriggerType = "manual"
)

// ActionType represents the type of automation action
type ActionType string

const (
	ActionTypeDeviceControl ActionType = "device_control"
	ActionTypeNotification  ActionType = "notification"
	ActionTypeDelay         ActionType = "delay"
	ActionTypeScene         ActionType = "scene"
)

// ComparisonOperator represents a comparison operator for conditions
type ComparisonOperator string

const (
	OpEqual        ComparisonOperator = "eq"
	OpNotEqual     ComparisonOperator = "neq"
	OpGreaterThan  ComparisonOperator = "gt"
	OpLessThan     ComparisonOperator = "lt"
	OpGreaterEqual ComparisonOperator = "gte"
	OpLessEqual    ComparisonOperator = "lte"
	OpContains     ComparisonOperator = "contains"
)

// ============================================================
// 数据结构
// ============================================================

// Device represents a smart home device
type Device struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         DeviceType        `json:"type"`
	Protocol     Protocol          `json:"protocol"`
	RoomID       string            `json:"room_id"`
	Status       DeviceStatus      `json:"status"`
	State        map[string]any    `json:"state"`
	Properties   map[string]any    `json:"properties"`
	Capabilities []string          `json:"capabilities"`
	Firmware     string            `json:"firmware"`
	Manufacturer string            `json:"manufacturer"`
	Model        string            `json:"model"`
	IPAddress    string            `json:"ip_address"`
	GroupIDs     []string          `json:"group_ids"`
	Metadata     map[string]string `json:"metadata"`
	LastSeen     time.Time         `json:"last_seen"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// Room represents a room in the house
type Room struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	DeviceIDs []string  `json:"device_ids"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Group represents a device group
type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RoomID    string    `json:"room_id"`
	DeviceIDs []string  `json:"device_ids"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Trigger represents an automation trigger
type Trigger struct {
	Type     TriggerType `json:"type"`
	DeviceID string      `json:"device_id,omitempty"`
	Field    string      `json:"field,omitempty"`
	CronExpr string      `json:"cron_expr,omitempty"`
	TimeStr  string      `json:"time_str,omitempty"`
}

// Condition represents an automation condition
type Condition struct {
	DeviceID string             `json:"device_id"`
	Field    string             `json:"field"`
	Value    any                `json:"value"`
	Operator ComparisonOperator `json:"operator"`
}

// Action represents an automation action
type Action struct {
	Type       ActionType     `json:"type"`
	DeviceID   string         `json:"device_id,omitempty"`
	SceneID    string         `json:"scene_id,omitempty"`
	Message    string         `json:"message,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	DelayMs    int64          `json:"delay_ms,omitempty"`
}

// Scene represents a smart home scene/automation
type Scene struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Trigger     Trigger     `json:"trigger"`
	Conditions  []Condition `json:"conditions,omitempty"`
	Actions     []Action    `json:"actions"`
	Enabled     bool        `json:"enabled"`
	LastRun     *time.Time  `json:"last_run,omitempty"`
	RunCount    int         `json:"run_count"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// ScheduledTask represents a scheduled automation task
type ScheduledTask struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	CronExpr  string     `json:"cron_expr"`
	SceneID   string     `json:"scene_id"`
	Enabled   bool       `json:"enabled"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	RunCount  int        `json:"run_count"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// EnergyReading represents a single energy reading

type EnergyReading struct {
	DeviceID  string    `json:"device_id"`
	PowerW    float64   `json:"power_w"`
	EnergyKWh float64   `json:"energy_kwh"`
	Timestamp time.Time `json:"timestamp"`
}

// EnergyStats represents energy statistics for a device

type EnergyStats struct {
	DeviceID     string    `json:"device_id"`
	TotalKWh     float64   `json:"total_kwh"`
	DailyKWh     float64   `json:"daily_kwh"`
	WeeklyKWh    float64   `json:"weekly_kwh"`
	MonthlyKWh   float64   `json:"monthly_kwh"`
	AvgPowerW    float64   `json:"avg_power_w"`
	PeakPowerW   float64   `json:"peak_power_w"`
	ReadingCount int       `json:"reading_count"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
}

// DashboardSummary represents the dashboard summary

type DashboardSummary struct {
	TotalDevices   int            `json:"total_devices"`
	OnlineDevices  int            `json:"online_devices"`
	OfflineDevices int            `json:"offline_devices"`
	TotalRooms     int            `json:"total_rooms"`
	TotalScenes    int            `json:"total_scenes"`
	ActiveScenes   int            `json:"active_scenes"`
	DevicesByType  map[string]int `json:"devices_by_type"`
	DevicesByRoom  map[string]int `json:"devices_by_room"`
	TotalEnergyKWh float64        `json:"total_energy_kwh"`
	TodayEnergyKWh float64        `json:"today_energy_kwh"`
	RecentEvents   []DeviceEvent  `json:"recent_events"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// DeviceEvent represents a device event
type DeviceEvent struct {
	DeviceID   string         `json:"device_id"`
	DeviceName string         `json:"device_name,omitempty"`
	Type       string         `json:"type"`
	State      map[string]any `json:"state,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// Config represents smart home configuration
type Config struct {
	Enabled          bool   `json:"enabled"`
	MatterEnabled    bool   `json:"matter_enabled"`
	HomeKitEnabled   bool   `json:"homekit_enabled"`
	MQTTBroker       string `json:"mqtt_broker"`
	MQTTPort         int    `json:"mqtt_port"`
	MQTTUsername     string `json:"mqtt_username"`
	MQTTPassword     string `json:"mqtt_password"`
	ZigbeePort       string `json:"zigbee_port"`
	ZWavePort        string `json:"zwave_port"`
	DiscoveryEnabled bool   `json:"discovery_enabled"`
	AutoAddDevices   bool   `json:"auto_add_devices"`
	MaxEvents        int    `json:"max_events"`
}

// Manager manages smart home devices and automations
type Manager struct {
	config      *Config
	devices     map[string]*Device
	rooms       map[string]*Room
	groups      map[string]*Group
	scenes      map[string]*Scene
	automations map[string]*Automation
	tasks       map[string]*ScheduledTask
	events      []DeviceEvent
	energyData  map[string][]EnergyReading
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// Automation represents a smart home automation (alias for Scene)
type Automation = Scene

// Manager lifecycle methods

// Start starts the smart home manager
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	m.initDefaultRooms()
	if m.config.DiscoveryEnabled {
		go m.discoverDevices()
	}
	go m.runAutomationEngine()
	return nil
}

// Stop stops the smart home manager
func (m *Manager) Stop() {
	m.cancel()
}

func (m *Manager) initDefaultRooms() {
	defaultRooms := []struct {
		id   string
		name string
		icon string
	}{
		{"living_room", "客厅", "🛋️"},
		{"bedroom", "卧室", "🛏️"},
		{"kitchen", "厨房", "🍳"},
		{"bathroom", "浴室", "🚿"},
		{"study", "书房", "📚"},
		{"garage", "车库", "🚗"},
		{"garden", "花园", "🌿"},
	}
	for _, r := range defaultRooms {
		m.rooms[r.id] = &Room{ID: r.id, Name: r.name, Icon: r.icon}
	}
}

func (m *Manager) discoverDevices() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// placeholder for device discovery
		}
	}
}

func (m *Manager) runAutomationEngine() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.evaluateAutomations()
		}
	}
}

func (m *Manager) evaluateAutomations() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, auto := range m.automations {
		if !auto.Enabled {
			continue
		}
		m.executeActions(auto.Actions)
	}
}

// NewManager creates a new smart home manager
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	if config.MaxEvents <= 0 {
		config.MaxEvents = 1000
	}
	return &Manager{
		config:      config,
		devices:     make(map[string]*Device),
		rooms:       make(map[string]*Room),
		groups:      make(map[string]*Group),
		scenes:      make(map[string]*Scene),
		automations: make(map[string]*Automation),
		tasks:       make(map[string]*ScheduledTask),
		events:      make([]DeviceEvent, 0),
		energyData:  make(map[string][]EnergyReading),
		ctx:         ctx,
		cancel:      cancel,
	}
}
