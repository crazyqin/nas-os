package ipmi

import "time"

// IPMIConfig IPMI 配置.
type IPMIConfig struct {
	DefaultHost     string        `json:"default_host"`
	DefaultPort     int           `json:"default_port"`
	DefaultUser     string        `json:"default_user"`
	DefaultPassword string        `json:"-"` // 不序列化
	PollInterval    time.Duration `json:"poll_interval"`
	EventLimit      int           `json:"event_limit"`
	SensorThreshold float64       `json:"sensor_threshold"`
	Timeout         time.Duration `json:"timeout"`
	RetryCount      int           `json:"retry_count"`
}

// IPMIDevice IPMI 设备.
type IPMIDevice struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Host         string            `json:"host"`
	Port         int               `json:"port"`
	Username     string            `json:"username"`
	Password     string            `json:"-"` // 不序列化
	Status       DeviceStatus      `json:"status"`
	PowerState   PowerState        `json:"power_state"`
	BMCVersion   string            `json:"bmc_version"`
	SerialNo     string            `json:"serial_no"`
	Model        string            `json:"model"`
	Manufacturer string            `json:"manufacturer"`
	FirmwareVer  string            `json:"firmware_ver"`
	Attributes   map[string]string `json:"attributes"`
	LastSeen     time.Time         `json:"last_seen"`
	CreatedAt    time.Time         `json:"created_at"`
}

// DeviceStatus 设备状态.
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusError   DeviceStatus = "error"
)

// PowerState 电源状态.
type PowerState string

const (
	PowerStateOn      PowerState = "on"
	PowerStateOff     PowerState = "off"
	PowerStateUnknown PowerState = "unknown"
)

// Sensor 传感器.
type Sensor struct {
	ID          string       `json:"id"`
	DeviceID    string       `json:"device_id"`
	Name        string       `json:"name"`
	Type        SensorType   `json:"type"`
	Value       float64      `json:"value"`
	Unit        string       `json:"unit"`
	Threshold   float64      `json:"threshold"`
	Status      SensorStatus `json:"status"`
	Reading     float64      `json:"reading"`
	Min         float64      `json:"min"`
	Max         float64      `json:"max"`
	LastReading time.Time    `json:"last_reading"`
}

// SensorType 传感器类型.
type SensorType string

const (
	SensorTypeTemperature SensorType = "temperature"
	SensorTypeVoltage     SensorType = "voltage"
	SensorTypeFan         SensorType = "fan"
	SensorTypePower       SensorType = "power"
	SensorTypeCurrent     SensorType = "current"
	SensorTypeMemory      SensorType = "memory"
	SensorTypeCPU         SensorType = "cpu"
)

// SensorStatus 传感器状态.
type SensorStatus string

const (
	SensorStatusNormal   SensorStatus = "normal"
	SensorStatusWarning  SensorStatus = "warning"
	SensorStatusCritical SensorStatus = "critical"
)

// SystemEvent 系统事件.
type SystemEvent struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"device_id"`
	Type      EventType `json:"type"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

// EventType 事件类型.
type EventType string

const (
	EventTypePower    EventType = "power"
	EventTypeSensor   EventType = "sensor"
	EventTypeHardware EventType = "hardware"
	EventTypeSecurity EventType = "security"
	EventTypeNetwork  EventType = "network"
	EventTypeSystem   EventType = "system"
)

// IPMIStats 统计信息.
type IPMIStats struct {
	TotalDevices   int `json:"total_devices"`
	OnlineDevices  int `json:"online_devices"`
	OfflineDevices int `json:"offline_devices"`
	TotalSensors   int `json:"total_sensors"`
	TotalEvents    int `json:"total_events"`
}

// SOLSession SOL 会话.
type SOLSession struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"device_id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	Bytes     int64     `json:"bytes"`
}
