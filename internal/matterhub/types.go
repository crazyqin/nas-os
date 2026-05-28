// Package matterhub 提供 Matter/Thread 智能家居中枢功能
// 支持 Matter 设备配对、Thread 边界路由器管理、设备控制、场景与自动化
package matterhub

import (
	"context"
	"sync"
	"time"
)

// ============================================================
// 设备类型与状态
// ============================================================

// DeviceType Matter 设备类型
type DeviceType string

const (
	DeviceTypeOnOffLight      DeviceType = "on_off_light"
	DeviceTypeDimmableLight   DeviceType = "dimmable_light"
	DeviceTypeColorLight      DeviceType = "color_light"
	DeviceTypeOnOffSwitch     DeviceType = "on_off_switch"
	DeviceTypeDimmableSwitch  DeviceType = "dimmable_switch"
	DeviceTypeOnOffPlug       DeviceType = "on_off_plug"
	DeviceTypeDimmablePlug    DeviceType = "dimmable_plug"
	DeviceTypeThermostat      DeviceType = "thermostat"
	DeviceTypeDoorLock        DeviceType = "door_lock"
	DeviceTypeContactSensor   DeviceType = "contact_sensor"
	DeviceTypeMotionSensor    DeviceType = "motion_sensor"
	DeviceTypeTemperatureSensor DeviceType = "temperature_sensor"
	DeviceTypeHumiditySensor  DeviceType = "humidity_sensor"
	DeviceTypeLightSensor     DeviceType = "light_sensor"
	DeviceTypeOccupancySensor DeviceType = "occupancy_sensor"
	DeviceTypeWindowCovering  DeviceType = "window_covering"
	DeviceTypeFan             DeviceType = "fan"
	DeviceTypeAirPurifier     DeviceType = "air_purifier"
	DeviceTypeSpeaker         DeviceType = "speaker"
	DeviceTypeOther           DeviceType = "other"
)

// DeviceState 设备在线状态
type DeviceState string

const (
	DeviceStateOnline      DeviceState = "online"
	DeviceStateOffline     DeviceState = "offline"
	DeviceStateCommissioning DeviceState = "commissioning"
	DeviceStateUnknown     DeviceState = "unknown"
)

// CommissionStatus 配对状态
type CommissionStatus string

const (
	CommissionStatusPending    CommissionStatus = "pending"
	CommissionStatusInProgress CommissionStatus = "in_progress"
	CommissionStatusSuccess    CommissionStatus = "success"
	CommissionStatusFailed     CommissionStatus = "failed"
	CommissionStatusTimeout    CommissionStatus = "timeout"
)

// ============================================================
// 触发器与自动化
// ============================================================

// TriggerType 触发器类型
type TriggerType string

const (
	TriggerTypeDevice      TriggerType = "device"
	TriggerTypeTime        TriggerType = "time"
	TriggerTypeSunrise     TriggerType = "sunrise"
	TriggerTypeSunset      TriggerType = "sunset"
	TriggerTypeManual      TriggerType = "manual"
	TriggerTypeTemperature TriggerType = "temperature"
)

// ActionType 动作类型
type ActionType string

const (
	ActionTypeDeviceControl ActionType = "device_control"
	ActionTypeScene         ActionType = "scene"
	ActionTypeNotification  ActionType = "notification"
	ActionTypeDelay         ActionType = "delay"
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
)

// ============================================================
// 数据结构
// ============================================================

// MatterDevice Matter 设备
type MatterDevice struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	VendorID       uint16            `json:"vendor_id"`
	ProductID      uint16            `json:"product_id"`
	Type           DeviceType        `json:"type"`
	State          DeviceState       `json:"state"`
	NodeID         uint64            `json:"node_id"`
	FabricIndex    uint8             `json:"fabric_index"`
	Endpoint       uint16            `json:"endpoint"`
	RoomID         string            `json:"room_id"`
	GroupIDs       []string          `json:"group_ids"`
	Capabilities   []string          `json:"capabilities"`
	Firmware       string            `json:"firmware"`
	Manufacturer   string            `json:"manufacturer"`
	Model          string            `json:"model"`
	SerialNumber   string            `json:"serial_number"`
	IPAddress      string            `json:"ip_address"`
	MacAddress     string            `json:"mac_address"`
	IsThreadDevice bool              `json:"is_thread_device"`
	ThreadInfo     *ThreadDeviceInfo `json:"thread_info,omitempty"`
	Attributes     map[string]any    `json:"attributes"`
	LastSeen       time.Time         `json:"last_seen"`
	CommissionedAt *time.Time        `json:"commissioned_at,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// ThreadDeviceInfo Thread 设备信息
type ThreadDeviceInfo struct {
	ExtendedAddress string `json:"extended_address"`
	Rloc16          uint16 `json:"rloc16"`
	RouterID        uint16 `json:"router_id"`
	ChildCount      int    `json:"child_count"`
	IsRouter        bool   `json:"is_router"`
	PartitionID     uint32 `json:"partition_id"`
}

// ThreadBorderRouter Thread 边界路由器
type ThreadBorderRouter struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	InterfaceName   string    `json:"interface_name"`
	State           string    `json:"state"`
	NetworkName     string    `json:"network_name"`
	ExtendedPanID   string    `json:"extended_pan_id"`
	PanID           uint16    `json:"pan_id"`
	Channel         uint8     `json:"channel"`
	RouterID        uint16    `json:"router_id"`
	BorderAgentID   string    `json:"border_agent_id"`
	IsActive        bool      `json:"is_active"`
	ChildCount      int       `json:"child_count"`
	ExternalRoutes  []string  `json:"external_routes"`
	OnMeshPrefixes  []string  `json:"on_mesh_prefixes"`
	IPAddress       string    `json:"ip_address"`
	FirmwareVersion string    `json:"firmware_version"`
	LastSeen        time.Time `json:"last_seen"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CommissionRequest 配对请求
type CommissionRequest struct {
	SetupCode     string `json:"setup_code"`
	SetupPIN      uint32 `json:"setup_pin"`
	Discriminator uint16 `json:"discriminator"`
	IPAddress     string `json:"ip_address"`
	WirelessSSID  string `json:"wireless_ssid,omitempty"`
	TimeoutSec    int    `json:"timeout_sec,omitempty"`
}

// CommissionResult 配对结果
type CommissionResult struct {
	Status    CommissionStatus `json:"status"`
	DeviceID  string           `json:"device_id,omitempty"`
	NodeID    uint64           `json:"node_id,omitempty"`
	Error     string           `json:"error,omitempty"`
	StartedAt time.Time        `json:"started_at"`
	EndedAt   *time.Time       `json:"ended_at,omitempty"`
}

// Trigger 自动化触发器
type Trigger struct {
	Type     TriggerType `json:"type"`
	DeviceID string      `json:"device_id,omitempty"`
	Field    string      `json:"field,omitempty"`
	CronExpr string      `json:"cron_expr,omitempty"`
	TimeStr  string      `json:"time_str,omitempty"`
}

// Condition 自动化条件
type Condition struct {
	DeviceID string             `json:"device_id"`
	Field    string             `json:"field"`
	Value    any                `json:"value"`
	Operator ComparisonOperator `json:"operator"`
}

// Action 自动化动作
type Action struct {
	Type       ActionType     `json:"type"`
	DeviceID   string         `json:"device_id,omitempty"`
	SceneID    string         `json:"scene_id,omitempty"`
	Message    string         `json:"message,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	DelayMs    int64          `json:"delay_ms,omitempty"`
}

// Scene 场景
type Scene struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Trigger     Trigger    `json:"trigger"`
	Conditions  []Condition `json:"conditions,omitempty"`
	Actions     []Action   `json:"actions"`
	Enabled     bool       `json:"enabled"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	RunCount    int        `json:"run_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Automation 自动化规则（场景别名）
type Automation = Scene

// DeviceGroup 设备分组
type DeviceGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RoomID    string    `json:"room_id"`
	DeviceIDs []string  `json:"device_ids"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeviceEvent 设备事件
type DeviceEvent struct {
	DeviceID   string         `json:"device_id"`
	DeviceName string         `json:"device_name,omitempty"`
	Type       string         `json:"type"`
	State      map[string]any `json:"state,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// DashboardSummary 仪表盘摘要
type DashboardSummary struct {
	TotalDevices       int            `json:"total_devices"`
	OnlineDevices      int            `json:"online_devices"`
	OfflineDevices     int            `json:"offline_devices"`
	TotalBRs           int            `json:"total_border_routers"`
	ActiveBRs          int            `json:"active_border_routers"`
	TotalScenes        int            `json:"total_scenes"`
	ActiveScenes       int            `json:"active_scenes"`
	TotalGroups        int            `json:"total_groups"`
	DevicesByType      map[string]int `json:"devices_by_type"`
	DevicesByRoom      map[string]int `json:"devices_by_room"`
	RecentEvents       []DeviceEvent  `json:"recent_events"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// Config 中枢配置
type Config struct {
	Enabled             bool   `json:"enabled"`
	ListenAddress       string `json:"listen_address"`
	FabricID            uint64 `json:"fabric_id"`
	AdminVendorID       uint16 `json:"admin_vendor_id"`
	ThreadNetworkName   string `json:"thread_network_name"`
	ThreadChannel       uint8  `json:"thread_channel"`
	ThreadPanID         uint16 `json:"thread_pan_id"`
	DiscoveryEnabled    bool   `json:"discovery_enabled"`
	AutoCommission      bool   `json:"auto_commission"`
	CommissionTimeoutSec int   `json:"commission_timeout_sec"`
	MaxEvents           int    `json:"max_events"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Enabled:             true,
		ListenAddress:       "0.0.0.0:5540",
		FabricID:            1,
		AdminVendorID:       0xFFF1,
		ThreadNetworkName:   "MatterHub",
		ThreadChannel:       15,
		ThreadPanID:         0x1234,
		DiscoveryEnabled:    true,
		AutoCommission:      false,
		CommissionTimeoutSec: 120,
		MaxEvents:           1000,
	}
}

// Hub Matter/Thread 智能家居中枢
type Hub struct {
	config          *Config
	devices         map[string]*MatterDevice
	borderRouters   map[string]*ThreadBorderRouter
	scenes          map[string]*Scene
	automations     map[string]*Automation
	groups          map[string]*DeviceGroup
	commissionTasks map[string]*CommissionResult
	events          []DeviceEvent
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewHub 创建中枢实例
func NewHub(config *Config) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	if config.MaxEvents <= 0 {
		config.MaxEvents = 1000
	}
	return &Hub{
		config:          config,
		devices:         make(map[string]*MatterDevice),
		borderRouters:   make(map[string]*ThreadBorderRouter),
		scenes:          make(map[string]*Scene),
		automations:     make(map[string]*Automation),
		groups:          make(map[string]*DeviceGroup),
		commissionTasks: make(map[string]*CommissionResult),
		events:          make([]DeviceEvent, 0),
		ctx:             ctx,
		cancel:          cancel,
	}
}
