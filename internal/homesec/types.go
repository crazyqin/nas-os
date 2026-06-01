// Package homesec 提供家庭安防系统功能，包括设备管理、区域布防、报警规则、事件处理等。
package homesec

import "time"

// DeviceType 设备类型
type DeviceType string

const (
	DeviceDoorWindow DeviceType = "door_window" // 门窗传感器
	DeviceMotion     DeviceType = "motion"      // 运动传感器
	DeviceSmoke      DeviceType = "smoke"       // 烟雾传感器
	DeviceWater      DeviceType = "water"       // 水浸传感器
	DeviceGlass      DeviceType = "glass"       // 玻璃破碎传感器
	DevicePanic      DeviceType = "panic"       // 紧急按钮
	DeviceKeypad     DeviceType = "keypad"      // 键盘
	DeviceSiren      DeviceType = "siren"       // 警报器
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	StatusArmed       DeviceStatus = "armed"        // 已布防
	StatusDisarmed    DeviceStatus = "disarmed"      // 已撤防
	StatusTriggered   DeviceStatus = "triggered"     // 已触发
	StatusTampered    DeviceStatus = "tampered"      // 被篡改
	StatusLowBattery  DeviceStatus = "low_battery"   // 低电量
)

// ZoneType 区域类型
type ZoneType string

const (
	ZoneEntryExit    ZoneType = "entry_exit"    // 进出区域
	ZonePerimeter    ZoneType = "perimeter"     // 周界区域
	ZoneInterior     ZoneType = "interior"      // 内部区域
	Zone24H          ZoneType = "24h"           // 24小时区域
)

// EventType 事件类型
type EventType string

const (
	EventArm        EventType = "arm"         // 布防事件
	EventDisarm     EventType = "disarm"      // 撤防事件
	EventTrigger    EventType = "trigger"     // 触发事件
	EventTamper     EventType = "tamper"      // 篡改事件
	EventBatteryLow EventType = "battery_low" // 低电量事件
	EventRestore    EventType = "restore"     // 恢复事件
)

// EventSeverity 事件严重程度
type EventSeverity string

const (
	SeverityInfo     EventSeverity = "info"     // 信息
	SeverityWarning  EventSeverity = "warning"  // 警告
	SeverityCritical EventSeverity = "critical" // 严重
)

// ActionType 动作类型
type ActionType string

const (
	ActionNotify    ActionType = "notify"    // 发送通知
	ActionSiren     ActionType = "siren"     // 触发警报
	ActionLight     ActionType = "light"     // 控制灯光
	ActionCamera    ActionType = "camera"    // 控制摄像头
	ActionSnapshot  ActionType = "snapshot"  // 拍照快照
	ActionWebhook   ActionType = "webhook"   // 调用 Webhook
)

// ArmMode 布防模式
type ArmMode string

const (
	ArmHome ArmMode = "home" // 在家布防
	ArmAway ArmMode = "away" // 离家布防
)

// PanelStatus 面板状态
type PanelStatus string

const (
	PanelArmedHome PanelStatus = "armed_home" // 在家布防
	PanelArmedAway PanelStatus = "armed_away" // 离家布防
	PanelDisarmed  PanelStatus = "disarmed"   // 已撤防
)

// Device 安防设备
type Device struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      DeviceType   `json:"type"`
	Location  string       `json:"location"`
	Status    DeviceStatus `json:"status"`
	Battery   int          `json:"battery"`    // 电量百分比 0-100
	LastSeen  time.Time    `json:"last_seen"`
	Enabled   bool         `json:"enabled"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// Zone 安防区域
type Zone struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	DeviceIDs []string   `json:"device_ids"`
	Type      ZoneType   `json:"type"`
	Armed     bool       `json:"armed"`
	Bypass    bool       `json:"bypass"` // 是否绕过
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Event 安防事件
type Event struct {
	ID        string        `json:"id"`
	DeviceID  string        `json:"device_id"`
	ZoneID    string        `json:"zone_id"`
	Type      EventType     `json:"type"`
	Timestamp time.Time     `json:"timestamp"`
	Message   string        `json:"message"`
	Severity  EventSeverity `json:"severity"`
	Acked     bool          `json:"acked"` // 是否已确认
}

// Condition 报警条件
type Condition struct {
	DeviceType DeviceType `json:"device_type"`
	Status     DeviceStatus `json:"status"`
	Threshold  int        `json:"threshold,omitempty"` // 阈值（如电量）
}

// Action 报警动作
type Action struct {
	ID         string            `json:"id"`
	Type       ActionType        `json:"type"`
	Target     string            `json:"target"`               // 目标设备/地址
	Parameters map[string]string `json:"parameters,omitempty"` // 额外参数
}

// AlarmRule 报警规则
type AlarmRule struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Conditions []Condition `json:"conditions"`
	Actions    []Action    `json:"actions"`
	Enabled    bool        `json:"enabled"`
	Priority   int         `json:"priority"` // 优先级 1-10
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// Schedule 布防计划
type Schedule struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ZoneIDs    []string  `json:"zone_ids"`
	ArmTime    string    `json:"arm_time"`    // HH:MM 格式
	DisarmTime string    `json:"disarm_time"` // HH:MM 格式
	Days       []string  `json:"days"`        // 周一到周日
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Panel 安防面板
type Panel struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Status       PanelStatus  `json:"status"`
	ZoneIDs      []string     `json:"zone_ids"`
	LastArmEvent *Event       `json:"last_arm_event,omitempty"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// SecurityScore 安防评分
type SecurityScore struct {
	Score    int                    `json:"score"` // 0-100
	Details  map[string]interface{} `json:"details"`
}
