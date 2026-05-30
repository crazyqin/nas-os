package smarthomehub

import (
	"time"
)

// DeviceProtocol 设备通信协议
type DeviceProtocol string

const (
	ProtocolMatter DeviceProtocol = "Matter"
	ProtocolZigbee DeviceProtocol = "Zigbee"
	ProtocolZWave  DeviceProtocol = "Z-Wave"
	ProtocolWiFi   DeviceProtocol = "WiFi"
	ProtocolBLE    DeviceProtocol = "BLE"
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeLight      DeviceType = "Light"
	DeviceTypeThermostat DeviceType = "Thermostat"
	DeviceTypeSensor     DeviceType = "Sensor"
	DeviceTypeLock       DeviceType = "Lock"
	DeviceTypeCamera     DeviceType = "Camera"
	DeviceTypeSwitch     DeviceType = "Switch"
	DeviceTypeFan        DeviceType = "Fan"
	DeviceTypeBlind      DeviceType = "Blind"
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	StatusOnline  DeviceStatus = "online"
	StatusOffline DeviceStatus = "offline"
	StatusUnknown DeviceStatus = "unknown"
)

// TriggerType 触发类型
type TriggerType string

const (
	TriggerManual   TriggerType = "Manual"
	TriggerSchedule TriggerType = "Schedule"
	TriggerSensor   TriggerType = "Sensor"
	TriggerSunset   TriggerType = "Sunset"
)

// ConditionOperator 条件运算符
type ConditionOperator string

const (
	OperatorGT      ConditionOperator = "GT"
	OperatorLT      ConditionOperator = "LT"
	OperatorEQ      ConditionOperator = "EQ"
	OperatorBetween ConditionOperator = "BETWEEN"
)

// Device 智能设备
type Device struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Type            DeviceType        `json:"type"`
	Protocol        DeviceProtocol    `json:"protocol"`
	Room            string            `json:"room"`
	Status          DeviceStatus      `json:"status"`
	Capabilities    []string          `json:"capabilities"`
	FirmwareVersion string            `json:"firmware_version"`
	BatteryLevel    int               `json:"battery_level"`
	LastSeen        time.Time         `json:"last_seen"`
	Online          bool              `json:"online"`
	State           map[string]interface{} `json:"state,omitempty"`
}

// Scene 智能场景
type Scene struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Icon          string        `json:"icon"`
	Actions       []SceneAction `json:"actions"`
	TriggerType   TriggerType   `json:"trigger_type"`
	TriggerConfig map[string]interface{} `json:"trigger_config,omitempty"`
	Enabled       bool          `json:"enabled"`
	LastTriggered *time.Time    `json:"last_triggered,omitempty"`
}

// SceneAction 场景动作
type SceneAction struct {
	DeviceID   string                 `json:"device_id"`
	Command    string                 `json:"command"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Delay      int                    `json:"delay,omitempty"`
}

// Automation 自动化规则
type Automation struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Conditions  []Condition    `json:"conditions"`
	Actions     []SceneAction  `json:"actions"`
	Enabled     bool           `json:"enabled"`
	Cooldown    int            `json:"cooldown"`
	LastFired   *time.Time     `json:"last_fired,omitempty"`
}

// Condition 触发条件
type Condition struct {
	DeviceID string            `json:"device_id"`
	Operator ConditionOperator `json:"operator"`
	Value    interface{}       `json:"value"`
	Duration int               `json:"duration,omitempty"`
}

// Room 房间
type Room struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Icon      string   `json:"icon"`
	DeviceIDs []string `json:"device_ids"`
}

// HubConfig 中枢配置
type HubConfig struct {
	MatterEnabled    bool   `json:"matter_enabled"`
	ZigbeePort       string `json:"zigbee_port"`
	ZWavePort        string `json:"zwave_port"`
	DiscoveryTimeout int    `json:"discovery_timeout"`
	MaxDevices       int    `json:"max_devices"`
}
