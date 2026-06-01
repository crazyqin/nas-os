// Package homeautomation 提供智能家居自动化引擎功能
// 支持设备管理、场景联动、规则引擎、定时任务、传感器数据处理等
package homeautomation

import "time"

// DeviceType 设备类型
type DeviceType string

const (
	DeviceLight       DeviceType = "light"
	DeviceSwitch      DeviceType = "switch"
	DeviceDimmer      DeviceType = "dimmer"
	DeviceThermostat  DeviceType = "thermostat"
	DeviceSensor      DeviceType = "sensor"
	DeviceLock        DeviceType = "lock"
	DeviceCamera      DeviceType = "camera"
	DeviceDoorbell    DeviceType = "doorbell"
	DeviceBlind       DeviceType = "blind"
	DeviceCurtain     DeviceType = "curtain"
	DeviceFan         DeviceType = "fan"
	DeviceAC          DeviceType = "ac"
	DeviceHeater      DeviceType = "heater"
	DeviceHumidifier  DeviceType = "humidifier"
	DevicePlug        DeviceType = "plug"
	DeviceOutlet      DeviceType = "outlet"
	DeviceSpeaker     DeviceType = "speaker"
	DeviceDisplay     DeviceType = "display"
	DeviceValve       DeviceType = "valve"
	DeviceGarageDoor  DeviceType = "garage_door"
	DeviceIrrigation  DeviceType = "irrigation"
	DevicePool        DeviceType = "pool"
	DeviceEVCharger   DeviceType = "ev_charger"
	DeviceSolarPanel  DeviceType = "solar_panel"
	DeviceBattery     DeviceType = "battery"
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceOnline  DeviceStatus = "online"
	DeviceOffline DeviceStatus = "offline"
	DeviceError   DeviceStatus = "error"
	DeviceUnknown DeviceStatus = "unknown"
)

// Connectivity 连接方式
type Connectivity string

const (
	ConnWiFi     Connectivity = "wifi"
	ConnZigbee   Connectivity = "zigbee"
	ConnZWave    Connectivity = "zwave"
	ConnBluetooth Connectivity = "bluetooth"
	ConnMatter   Connectivity = "matter"
	ConnThread   Connectivity = "thread"
	ConnRF433    Connectivity = "rf433"
	ConnRF868    Connectivity = "rf868"
	ConnIR       Connectivity = "ir"
	ConnSerial   Connectivity = "serial"
	ConnModbus   Connectivity = "modbus"
	ConnKNX      Connectivity = "knx"
)

// TriggerType 触发器类型
type TriggerType string

const (
	TriggerDeviceState  TriggerType = "device_state"
	TriggerDeviceEvent  TriggerType = "device_event"
	TriggerTime         TriggerType = "time"
	TriggerSunrise      TriggerType = "sunrise"
	TriggerSunset       TriggerType = "sunset"
	TriggerSensorValue  TriggerType = "sensor_value"
	TriggerGeofence     TriggerType = "geofence"
	TriggerWebhook      TriggerType = "webhook"
	TriggerManual       TriggerType = "manual"
)

// ActionType 动作类型
type ActionType string

const (
	ActionDeviceControl  ActionType = "device_control"
	ActionSceneActivate  ActionType = "scene_activate"
	ActionNotification   ActionType = "notification"
	ActionWebhook        ActionType = "webhook"
	ActionDelay          ActionType = "delay"
	ActionCondition      ActionType = "condition"
	ActionScript         ActionType = "script"
	ActionLog            ActionType = "log"
)

// ConditionOperator 条件运算符
type ConditionOperator string

const (
	OpEqual        ConditionOperator = "eq"
	OpNotEqual     ConditionOperator = "neq"
	OpGreater      ConditionOperator = "gt"
	OpGreaterEqual ConditionOperator = "gte"
	OpLess         ConditionOperator = "lt"
	OpLessEqual    ConditionOperator = "lte"
	OpContains     ConditionOperator = "contains"
	OpIn           ConditionOperator = "in"
	OpBetween      ConditionOperator = "between"
	OpAnd          ConditionOperator = "and"
	OpOr           ConditionOperator = "or"
	OpNot          ConditionOperator = "not"
)

// AutomationDevice 自动化设备
type AutomationDevice struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Type         DeviceType        `json:"type"`
	Manufacturer string            `json:"manufacturer"`
	Model        string            `json:"model"`
	Firmware     string            `json:"firmware"`
	Status       DeviceStatus      `json:"status"`
	Connectivity Connectivity      `json:"connectivity"`
	Room         string            `json:"room"`
	Zone         string            `json:"zone"`
	Area         string            `json:"area"`
	State        map[string]interface{} `json:"state"`
	Capabilities []string          `json:"capabilities"`
	Tags         []string          `json:"tags"`
	LastSeen     time.Time         `json:"last_seen"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	BatteryLevel *int              `json:"battery_level,omitempty"`
	SignalStrength *int            `json:"signal_strength,omitempty"`
	IPAddress    string            `json:"ip_address,omitempty"`
	MACAddress   string            `json:"mac_address,omitempty"`
	Integration  string            `json:"integration"`
	Disabled     bool              `json:"disabled"`
}

// Scene 场景
type Scene struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Icon        string        `json:"icon"`
	Devices     []SceneDevice `json:"devices"`
	Actions     []SceneAction `json:"actions"`
	Tags        []string      `json:"tags"`
	Enabled     bool          `json:"enabled"`
	Favorite    bool          `json:"favorite"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	LastActivated *time.Time  `json:"last_activated,omitempty"`
	ActivationCount int       `json:"activation_count"`
}

// SceneDevice 场景设备状态
type SceneDevice struct {
	DeviceID string                 `json:"device_id"`
	State    map[string]interface{} `json:"state"`
}

// SceneAction 场景动作
type SceneAction struct {
	Type     ActionType `json:"type"`
	Target   string     `json:"target"`
	Command  string     `json:"command"`
	Value    interface{} `json:"value,omitempty"`
	Delay    int        `json:"delay,omitempty"`
}

// Automation 自动化规则
type Automation struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Enabled     bool        `json:"enabled"`
	Triggers    []Trigger   `json:"triggers"`
	Conditions  []Condition `json:"conditions"`
	Actions     []Action    `json:"actions"`
	Mode        string      `json:"mode"`
	MaxRuns     int         `json:"max_runs"`
	CurrentRuns int         `json:"current_runs"`
	LastTriggered *time.Time `json:"last_triggered,omitempty"`
	Tags        []string    `json:"tags"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Logging     bool        `json:"logging"`
}

// Trigger 触发器
type Trigger struct {
	Type     TriggerType `json:"type"`
	DeviceID string      `json:"device_id,omitempty"`
	Property string      `json:"property,omitempty"`
	Value    interface{} `json:"value,omitempty"`
	CronExpr string      `json:"cron_expr,omitempty"`
	TimeStr  string      `json:"time_str,omitempty"`
	Offset   int         `json:"offset,omitempty"`
}

// Condition 条件
type Condition struct {
	Type     ConditionOperator `json:"type"`
	DeviceID string            `json:"device_id,omitempty"`
	Property string            `json:"property,omitempty"`
	Value    interface{}       `json:"value,omitempty"`
	Children []Condition       `json:"children,omitempty"`
}

// Action 动作
type Action struct {
	Type       ActionType `json:"type"`
	DeviceID   string     `json:"device_id,omitempty"`
	SceneID    string     `json:"scene_id,omitempty"`
	Command    string     `json:"command,omitempty"`
	Value      interface{} `json:"value,omitempty"`
	Delay      int        `json:"delay,omitempty"`
	Repeat     int        `json:"repeat,omitempty"`
	Message    string     `json:"message,omitempty"`
	WebhookURL string     `json:"webhook_url,omitempty"`
}

// Room 房间
type Room struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Floor       string   `json:"floor"`
	Icon        string   `json:"icon"`
	Devices     []string `json:"devices"`
	Temperature *float64 `json:"temperature,omitempty"`
	Humidity    *float64 `json:"humidity,omitempty"`
}

// Zone 区域
type Zone struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Rooms []string `json:"rooms"`
	Scenes []string `json:"scenes"`
}

// SensorData 传感器数据
type SensorData struct {
	DeviceID  string    `json:"device_id"`
	Property  string    `json:"property"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	Timestamp time.Time `json:"timestamp"`
}

// AutomationLog 自动化日志
type AutomationLog struct {
	ID           string    `json:"id"`
	AutomationID string    `json:"automation_id"`
	AutomationName string  `json:"automation_name"`
	TriggerType  string    `json:"trigger_type"`
	TriggerDevice string   `json:"trigger_device,omitempty"`
	ConditionsMet bool     `json:"conditions_met"`
	ActionsRun   int       `json:"actions_run"`
	Success      bool      `json:"success"`
	Error        string    `json:"error,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	Duration     int64     `json:"duration"`
}

// AutomationStats 自动化统计
type AutomationStats struct {
	TotalAutomations   int     `json:"total_automations"`
	ActiveAutomations  int     `json:"active_automations"`
	TotalTriggers      int64   `json:"total_triggers"`
	TriggersToday      int     `json:"triggers_today"`
	SuccessRate        float64 `json:"success_rate"`
	TotalDevices       int     `json:"total_devices"`
	OnlineDevices      int     `json:"online_devices"`
	TotalScenes        int     `json:"total_scenes"`
	TotalRooms         int     `json:"total_rooms"`
	AvgResponseTime    float64 `json:"avg_response_time"`
}

// Integration 集成
type Integration struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Enabled     bool      `json:"enabled"`
	Config      map[string]interface{} `json:"config"`
	Devices     []string  `json:"devices"`
	Status      string    `json:"status"`
	LastSync    time.Time `json:"last_sync"`
	CreatedAt   time.Time `json:"created_at"`
}

// Schedule 定时任务
type Schedule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CronExpr  string    `json:"cron_expr"`
	Action    Action    `json:"action"`
	Enabled   bool      `json:"enabled"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	NextRun   *time.Time `json:"next_run,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
