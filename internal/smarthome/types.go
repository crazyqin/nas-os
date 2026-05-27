// Package smarthome provides a smart home center for IoT device management.
// 智能家居中心 - 设备管理、自动化场景、能耗统计
// 参考飞牛fnOS 和群晖 Synology Home 设计
package smarthome

import (
	"sync"
	"time"
)

// ============================================================
// 设备协议和类型
// ============================================================

// Protocol 设备通信协议
type Protocol string

const (
	ProtocolMQTT  Protocol = "mqtt"
	ProtocolHTTP  Protocol = "http"
	ProtocolZigbee Protocol = "zigbee"
)

// DeviceType 设备类型
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

// DeviceStatus 设备在线状态
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusUnknown DeviceStatus = "unknown"
)

// ============================================================
// 设备类型定义
// ============================================================

// Device 智能家居设备
type Device struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        DeviceType        `json:"type"`
	Protocol    Protocol          `json:"protocol"`
	MACAddress  string            `json:"mac_address,omitempty"`
	IPAddress   string            `json:"ip_address,omitempty"`
	MQTTTopic   string            `json:"mqtt_topic,omitempty"`
	RoomID      string            `json:"room_id,omitempty"`
	GroupIDs    []string          `json:"group_ids,omitempty"`
	Status      DeviceStatus      `json:"status"`
	State       map[string]any    `json:"state"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	LastSeen    time.Time         `json:"last_seen"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// DeviceState 设备状态变更记录
type DeviceState struct {
	DeviceID  string         `json:"device_id"`
	State     map[string]any `json:"state"`
	Timestamp time.Time      `json:"timestamp"`
}

// ============================================================
// 房间和分组
// ============================================================

// Room 设备房间
type Room struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon,omitempty"`
	DeviceIDs []string  `json:"device_ids,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Group 设备分组
type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RoomID    string    `json:"room_id,omitempty"`
	DeviceIDs []string  `json:"device_ids,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================
// 自动化场景
// ============================================================

// TriggerType 触发器类型
type TriggerType string

const (
	TriggerTypeDevice  TriggerType = "device"  // 设备状态变化触发
	TriggerTypeTime    TriggerType = "time"    // 定时触发
	TriggerTypeSunrise TriggerType = "sunrise" // 日出触发
	TriggerTypeSunset  TriggerType = "sunset"  // 日落触发
	TriggerTypeManual  TriggerType = "manual"  // 手动触发
)

// ActionType 动作类型
type ActionType string

const (
	ActionTypeDeviceControl ActionType = "device_control" // 控制设备
	ActionTypeNotification  ActionType = "notification"  // 发送通知
	ActionTypeDelay         ActionType = "delay"         // 延时
	ActionTypeScene         ActionType = "scene"         // 触发其他场景
)

// ComparisonOperator 比较运算符
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

// Condition 自动化条件
type Condition struct {
	DeviceID string             `json:"device_id"`
	Field    string             `json:"field"`
	Operator ComparisonOperator `json:"operator"`
	Value    any                `json:"value"`
}

// Trigger 自动化触发器
type Trigger struct {
	Type     TriggerType    `json:"type"`
	DeviceID string         `json:"device_id,omitempty"`
	Field    string         `json:"field,omitempty"`
	CronExpr string         `json:"cron_expr,omitempty"`
	TimeStr  string         `json:"time_str,omitempty"` // HH:MM format
	Timezone string         `json:"timezone,omitempty"`
}

// Action 自动化动作
type Action struct {
	Type       ActionType      `json:"type"`
	DeviceID   string          `json:"device_id,omitempty"`
	Properties map[string]any  `json:"properties,omitempty"`
	DelayMs    int             `json:"delay_ms,omitempty"`
	SceneID    string          `json:"scene_id,omitempty"`
	Message    string          `json:"message,omitempty"`
}

// Scene 自动化场景
type Scene struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Enabled     bool        `json:"enabled"`
	Trigger     Trigger     `json:"trigger"`
	Conditions  []Condition `json:"conditions,omitempty"`
	Actions     []Action    `json:"actions"`
	LastRun     *time.Time  `json:"last_run,omitempty"`
	RunCount    int         `json:"run_count"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// ============================================================
// 定时任务
// ============================================================

// ScheduledTask 定时任务
type ScheduledTask struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	SceneID     string    `json:"scene_id"`
	CronExpr    string    `json:"cron_expr"`
	Enabled     bool      `json:"enabled"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	NextRun     *time.Time `json:"next_run,omitempty"`
	RunCount    int       `json:"run_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ============================================================
// 能耗统计
// ============================================================

// EnergyReading 能耗读数
type EnergyReading struct {
	DeviceID   string    `json:"device_id"`
	PowerW     float64   `json:"power_w"`     // 实时功率 (W)
	EnergyKWh  float64   `json:"energy_kwh"`  // 累计电量 (kWh)
	Timestamp  time.Time `json:"timestamp"`
}

// EnergyStats 能耗统计
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

// ============================================================
// 仪表盘
// ============================================================

// DashboardSummary 仪表盘摘要
type DashboardSummary struct {
	TotalDevices    int            `json:"total_devices"`
	OnlineDevices   int            `json:"online_devices"`
	OfflineDevices  int            `json:"offline_devices"`
	TotalRooms      int            `json:"total_rooms"`
	TotalScenes     int            `json:"total_scenes"`
	ActiveScenes    int            `json:"active_scenes"`
	TotalEnergyKWh  float64        `json:"total_energy_kwh"`
	TodayEnergyKWh  float64        `json:"today_energy_kwh"`
	DevicesByType   map[string]int `json:"devices_by_type"`
	DevicesByRoom   map[string]int `json:"devices_by_room"`
	RecentEvents    []DeviceEvent  `json:"recent_events"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// DeviceEvent 设备事件
type DeviceEvent struct {
	DeviceID  string         `json:"device_id"`
	DeviceName string        `json:"device_name"`
	Type      string         `json:"type"`
	State     map[string]any `json:"state,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// ============================================================
// 管理器配置
// ============================================================

// Config 智能家居配置
type Config struct {
	MQTTBroker      string `json:"mqtt_broker"`
	MQTTClientID    string `json:"mqtt_client_id"`
	MQTTUsername    string `json:"mqtt_username"`
	MQTTPassword    string `json:"mqtt_password"`
	ZigbeePort      string `json:"zigbee_port"`
	DiscoveryInterval int  `json:"discovery_interval"` // 设备发现间隔 (秒)
	HistoryRetentionDays int `json:"history_retention_days"` // 历史记录保留天数
	MaxEvents       int    `json:"max_events"`          // 最大事件数
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		MQTTBroker:           "localhost:1883",
		MQTTClientID:         "nas-os-smarthome",
		DiscoveryInterval:    300,
		HistoryRetentionDays: 30,
		MaxEvents:            1000,
	}
}

// Manager 智能家居管理器
type Manager struct {
	config     Config
	devices    map[string]*Device
	rooms      map[string]*Room
	groups     map[string]*Group
	scenes     map[string]*Scene
	tasks      map[string]*ScheduledTask
	energyData map[string][]EnergyReading
	events     []DeviceEvent
	mu         sync.RWMutex
}

// NewManager 创建智能家居管理器
func NewManager(config Config) *Manager {
	return &Manager{
		config:     config,
		devices:    make(map[string]*Device),
		rooms:      make(map[string]*Room),
		groups:     make(map[string]*Group),
		scenes:     make(map[string]*Scene),
		tasks:      make(map[string]*ScheduledTask),
		energyData: make(map[string][]EnergyReading),
		events:     make([]DeviceEvent, 0, config.MaxEvents),
	}
}
