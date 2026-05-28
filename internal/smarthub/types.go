// Package smarthub provides smart home hub functionality for NAS-OS.
// Device discovery, protocol gateways, automation scenes, and energy monitoring.
package smarthub

import (
	"time"
)

// ============================================================
// 设备发现类型
// ============================================================

// DiscoveryMethod 发现方式
type DiscoveryMethod string

const (
	DiscoveryMDNS  DiscoveryMethod = "mdns"  // mDNS/Bonjour
	DiscoverySSDP  DiscoveryMethod = "ssdp"  // SSDP/UPnP
	DiscoveryBLE   DiscoveryMethod = "ble"   // Bluetooth LE
	DiscoveryManual DiscoveryMethod = "manual" // 手动添加
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	StatusOnline  DeviceStatus = "online"
	StatusOffline DeviceStatus = "offline"
	StatusUnknown DeviceStatus = "unknown"
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeLight     DeviceType = "light"
	DeviceTypeSwitch    DeviceType = "switch"
	DeviceTypeSensor    DeviceType = "sensor"
	DeviceTypeThermostat DeviceType = "thermostat"
	DeviceTypeCamera    DeviceType = "camera"
	DeviceTypeLock      DeviceType = "lock"
	DeviceTypePlug      DeviceType = "plug"
	DeviceTypeOther     DeviceType = "other"
)

// Protocol 通信协议
type Protocol string

const (
	ProtocolZigbee Protocol = "zigbee"
	ProtocolZWave  Protocol = "zwave"
	ProtocolMatter Protocol = "matter"
	ProtocolBLE    Protocol = "ble"
	ProtocolWiFi   Protocol = "wifi"
)

// Device 智能设备
type Device struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        DeviceType      `json:"type"`
	Protocol    Protocol        `json:"protocol"`
	Manufacturer string         `json:"manufacturer,omitempty"`
	Model       string          `json:"model,omitempty"`
	FirmwareVer string          `json:"firmware_version,omitempty"`
	MACAddress  string          `json:"mac_address,omitempty"`
	IPAddress   string          `json:"ip_address,omitempty"`
	RoomID      string          `json:"room_id,omitempty"`
	GroupIDs    []string        `json:"group_ids,omitempty"`
	Status      DeviceStatus    `json:"status"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	LastSeen    time.Time       `json:"last_seen"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// DeviceDiscoveryResult 设备发现结果
type DeviceDiscoveryResult struct {
	Devices   []*Device `json:"devices"`
	Method    DiscoveryMethod `json:"method"`
	ScannedAt time.Time `json:"scanned_at"`
}

// ============================================================
// 协议网关类型
// ============================================================

// GatewayStatus 网关状态
type GatewayStatus string

const (
	GatewayRunning  GatewayStatus = "running"
	GatewayStopped  GatewayStatus = "stopped"
	GatewayError    GatewayStatus = "error"
)

// ProtocolGateway 协议网关
type ProtocolGateway struct {
	ID          string        `json:"id"`
	Protocol    Protocol      `json:"protocol"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Port        int           `json:"port"`
	Status      GatewayStatus `json:"status"`
	DeviceCount int           `json:"device_count"`
	Config      map[string]interface{} `json:"config,omitempty"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	ErrorMsg    string        `json:"error_message,omitempty"`
}

// ============================================================
// 设备分组类型
// ============================================================

// DeviceGroup 设备分组
type DeviceGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	DeviceIDs   []string  `json:"device_ids"`
	RoomID      string    `json:"room_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Room 房间
type Room struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	DeviceIDs []string `json:"device_ids,omitempty"`
}

// ============================================================
// 场景自动化类型
// ============================================================

// TriggerType 触发类型
type TriggerType string

const (
	TriggerDeviceState TriggerType = "device_state" // 设备状态变化
	TriggerSchedule    TriggerType = "schedule"     // 定时触发
	TriggerCondition   TriggerType = "condition"    // 条件触发
	TriggerManual      TriggerType = "manual"       // 手动触发
)

// ActionType 动作类型
type ActionType string

const (
	ActionSetProperty  ActionType = "set_property"  // 设置属性
	ActionRunScene     ActionType = "run_scene"     // 执行场景
	ActionSendNotification ActionType = "send_notification" // 发送通知
)

// Scene 场景
type Scene struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Enabled     bool           `json:"enabled"`
	Triggers    []Trigger      `json:"triggers"`
	Actions     []Action       `json:"actions"`
	LastRun     *time.Time     `json:"last_run,omitempty"`
	RunCount    int            `json:"run_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Trigger 触发条件
type Trigger struct {
	Type       TriggerType    `json:"type"`
	DeviceID   string         `json:"device_id,omitempty"`
	Property   string         `json:"property,omitempty"`
	Value      interface{}    `json:"value,omitempty"`
	Schedule   string         `json:"schedule,omitempty"` // cron 表达式
	Conditions []Condition    `json:"conditions,omitempty"`
}

// Condition 条件
type Condition struct {
	DeviceID string      `json:"device_id"`
	Property string      `json:"property"`
	Operator string      `json:"operator"` // eq, neq, gt, lt, gte, lte
	Value    interface{} `json:"value"`
}

// Action 执行动作
// Scene 场景定义已移至上方

// Action 动作定义
type Action struct {
	Type       ActionType      `json:"type"`
	DeviceID   string          `json:"device_id,omitempty"`
	SceneID    string          `json:"scene_id,omitempty"`
	Property   string          `json:"property,omitempty"`
	Value      interface{}     `json:"value,omitempty"`
	Delay      time.Duration   `json:"delay,omitempty"`
}

// SceneExecution 场景执行记录
type SceneExecution struct {
	ID        string    `json:"id"`
	SceneID   string    `json:"scene_id"`
	Trigger   string    `json:"trigger"`
	Status    string    `json:"status"` // success, failed
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// ============================================================
// 能耗监控类型
// ============================================================

// EnergyReading 能耗读数
type EnergyReading struct {
	DeviceID  string    `json:"device_id"`
	Power     float64   `json:"power"`      // 当前功率 (W)
	Energy    float64   `json:"energy"`     // 累计电量 (kWh)
	Voltage   float64   `json:"voltage"`    // 电压 (V)
	Current   float64   `json:"current"`    // 电流 (A)
	Timestamp time.Time `json:"timestamp"`
}

// EnergyStats 能耗统计
type EnergyStats struct {
	DeviceID      string    `json:"device_id"`
	TotalEnergy   float64   `json:"total_energy"`    // 总电量 (kWh)
	CurrentPower  float64   `json:"current_power"`   // 当前功率 (W)
	AvgPower      float64   `json:"avg_power"`       // 平均功率 (W)
	MaxPower      float64   `json:"max_power"`       // 最大功率 (W)
	MinPower      float64   `json:"min_power"`       // 最小功率 (W)
	DailyEnergy   float64   `json:"daily_energy"`    // 今日电量 (kWh)
	WeeklyEnergy  float64   `json:"weekly_energy"`   // 本周电量 (kWh)
	MonthlyEnergy float64   `json:"monthly_energy"`  // 本月电量 (kWh)
	ReadingCount  int       `json:"reading_count"`
	LastReading   time.Time `json:"last_reading"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// EnergyAlert 能耗告警
type EnergyAlert struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"device_id"`
	Type      string    `json:"type"`      // high_power, abnormal_usage
	Message   string    `json:"message"`
	Threshold float64   `json:"threshold"`
	Actual    float64   `json:"actual"`
	Level     string    `json:"level"`     // warning, critical
	Timestamp time.Time `json:"timestamp"`
}

// ============================================================
// 语音控制类型
// ============================================================

// VoiceCommand 语音指令
type VoiceCommand struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Language  string    `json:"language"`
	Intent    string    `json:"intent"`
	DeviceID  string    `json:"device_id,omitempty"`
	Action    string    `json:"action,omitempty"`
	Value     interface{} `json:"value,omitempty"`
	Confidence float64  `json:"confidence"`
	Processed bool      `json:"processed"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// VoiceResponse 语音响应
type VoiceResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ============================================================
// 请求/响应类型
// ============================================================

// DiscoverDevicesRequest 设备发现请求
type DiscoverDevicesRequest struct {
	Methods []DiscoveryMethod `json:"methods"`
	TimeoutSec int            `json:"timeout_sec"`
}

// CreateDeviceRequest 创建设备请求
type CreateDeviceRequest struct {
	Name        string   `json:"name" binding:"required"`
	Type        DeviceType `json:"type" binding:"required"`
	Protocol    Protocol `json:"protocol" binding:"required"`
	Manufacturer string  `json:"manufacturer,omitempty"`
	Model       string   `json:"model,omitempty"`
	MACAddress  string   `json:"mac_address,omitempty"`
	IPAddress   string   `json:"ip_address,omitempty"`
	RoomID      string   `json:"room_id,omitempty"`
}

// UpdateDeviceRequest 更新设备请求
type UpdateDeviceRequest struct {
	Name     string       `json:"name,omitempty"`
	RoomID   string       `json:"room_id,omitempty"`
	GroupIDs []string     `json:"group_ids,omitempty"`
}

// ControlDeviceRequest 控制设备请求
type ControlDeviceRequest struct {
	Property string      `json:"property" binding:"required"`
	Value    interface{} `json:"value" binding:"required"`
}

// CreateGroupRequest 创建分组请求
type CreateGroupRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description,omitempty"`
	DeviceIDs   []string `json:"device_ids"`
	RoomID      string   `json:"room_id,omitempty"`
}

// CreateSceneRequest 创建场景请求
type CreateSceneRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description,omitempty"`
	Triggers    []Trigger `json:"triggers" binding:"required"`
	Actions     []Action  `json:"actions" binding:"required"`
}

// VoiceCommandRequest 语音指令请求
type VoiceCommandRequest struct {
	Text     string `json:"text" binding:"required"`
	Language string `json:"language"`
}

// RegisterRoutes registers smarthub API routes.
