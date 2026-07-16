// Package smarthub 实现智能家居中枢模块
// 统一管理 Zigbee/Z-Wave/WiFi/BLE 设备，支持场景自动化、语音控制、能耗监控
package smarthub

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	ErrDeviceNotFound       = errors.New("device not found")
	ErrDeviceOffline        = errors.New("device offline")
	ErrSceneNotFound        = errors.New("scene not found")
	ErrProtocolNotSupported = errors.New("protocol not supported")
	ErrInvalidCommand       = errors.New("invalid command")
	ErrHubNotRunning        = errors.New("hub not running")
	ErrDuplicateDevice      = errors.New("duplicate device")
)

// ========== 设备协议 ==========

// Protocol 通信协议.
type Protocol string

const (
	ProtocolZigbee Protocol = "zigbee"
	ProtocolZWave  Protocol = "zwave"
	ProtocolWiFi   Protocol = "wifi"
	ProtocolBLE    Protocol = "ble"
	ProtocolMatter Protocol = "matter"
	ProtocolThread Protocol = "thread"
)

// ========== 设备类型 ==========

// DeviceType 设备类型.
type DeviceType string

const (
	DeviceTypeLight      DeviceType = "light"
	DeviceTypeSwitch     DeviceType = "switch"
	DeviceTypeSensor     DeviceType = "sensor"
	DeviceTypeThermostat DeviceType = "thermostat"
	DeviceTypeCamera     DeviceType = "camera"
	DeviceTypeLock       DeviceType = "lock"
	DeviceTypePlug       DeviceType = "plug"
	DeviceTypeBlind      DeviceType = "blind"
	DeviceTypeFan        DeviceType = "fan"
	DeviceTypeSpeaker    DeviceType = "speaker"
	DeviceTypeGateway    DeviceType = "gateway"
	DeviceTypeOther      DeviceType = "other"
)

// ========== 设备状态 ==========

// DeviceState 设备在线状态.
type DeviceState string

const (
	StateOnline  DeviceState = "online"
	StateOffline DeviceState = "offline"
	StateUnknown DeviceState = "unknown"
)

// ========== 设备定义 ==========

// Device 智能设备.
type Device struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       DeviceType        `json:"type"`
	Protocol   Protocol          `json:"protocol"`
	Room       string            `json:"room"`
	State      DeviceState       `json:"state"`
	Properties map[string]string `json:"properties"`
	Battery    int               `json:"battery"` // 0-100, -1=不适用
	Firmware   string            `json:"firmware"`
	LastSeen   time.Time         `json:"last_seen"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Tags       []string          `json:"tags"`
}

// ========== 场景定义 ==========

// Scene 智能场景.
type Scene struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Actions     []SceneAction `json:"actions"`
	Enabled     bool          `json:"enabled"`
	TriggerType TriggerType   `json:"trigger_type"`
	TriggerSpec string        `json:"trigger_spec"` // cron表达式或条件
	LastRun     time.Time     `json:"last_run"`
	RunCount    int64         `json:"run_count"`
	CreatedAt   time.Time     `json:"created_at"`
}

// TriggerType 触发类型.
type TriggerType string

const (
	TriggerManual    TriggerType = "manual"
	TriggerSchedule  TriggerType = "schedule"
	TriggerDevice    TriggerType = "device"
	TriggerCondition TriggerType = "condition"
)

// SceneAction 场景动作.
type SceneAction struct {
	DeviceID   string            `json:"device_id"`
	Command    string            `json:"command"`
	Parameters map[string]string `json:"parameters"`
	Delay      time.Duration     `json:"delay"` // 延迟执行
}

// ========== 自动化规则 ==========

// Automation 自动化规则.
type Automation struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Enabled      bool          `json:"enabled"`
	Conditions   []Condition   `json:"conditions"`
	Actions      []SceneAction `json:"actions"`
	LogicOp      string        `json:"logic_op"` // "and" / "or"
	LastTrigger  time.Time     `json:"last_trigger"`
	TriggerCount int64         `json:"trigger_count"`
	CreatedAt    time.Time     `json:"created_at"`
}

// Condition 触发条件.
type Condition struct {
	DeviceID string `json:"device_id"`
	Property string `json:"property"`
	Operator string `json:"operator"` // eq, neq, gt, lt, gte, lte
	Value    string `json:"value"`
}

// ========== 能耗统计 ==========

// EnergyRecord 能耗记录.
type EnergyRecord struct {
	DeviceID  string    `json:"device_id"`
	Timestamp time.Time `json:"timestamp"`
	Power     float64   `json:"power"`  // 瓦特
	Energy    float64   `json:"energy"` // 千瓦时
	Voltage   float64   `json:"voltage"`
	Current   float64   `json:"current"`
}

// EnergyStats 能耗统计.
type EnergyStats struct {
	DeviceID        string  `json:"device_id"`
	TotalEnergy     float64 `json:"total_energy"` // 千瓦时
	AvgPower        float64 `json:"avg_power"`    // 瓦特
	PeakPower       float64 `json:"peak_power"`
	DailyCost       float64 `json:"daily_cost"` // 元
	MonthlyCost     float64 `json:"monthly_cost"`
	CarbonFootprint float64 `json:"carbon_footprint"` // kg CO2
}

// HubConfig 中枢配置.
type HubConfig struct {
	ListenAddr        string        `json:"listen_addr"`
	ZigbeePort        string        `json:"zigbee_port"`
	ZWavePort         string        `json:"zwave_port"`
	MQTTBroker        string        `json:"mqtt_broker"`
	DiscoveryInterval time.Duration `json:"discovery_interval"`
	DeviceTimeout     time.Duration `json:"device_timeout"`
	EnableEnergy      bool          `json:"enable_energy"`
	TariffPerKWh      float64       `json:"tariff_per_kwh"` // 电价
}
