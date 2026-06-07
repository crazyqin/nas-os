// Package upsmanager 提供 UPS 电源管理功能
// UPS 设备发现与连接、电源状态监控、优雅关机策略、电源事件记录、多UPS支持、硬件健康监控
package upsmanager

import "time"

// ========== UPS 协议类型 ==========

// Protocol UPS 通信协议
type Protocol string

const (
	// ProtocolUSBHID USB HID 协议
	ProtocolUSBHID Protocol = "usb_hid"
	// ProtocolSNMP SNMP 协议
	ProtocolSNMP Protocol = "snmp"
	// ProtocolNUT NUT (Network UPS Tools) 协议
	ProtocolNUT Protocol = "nut"
)

// ========== UPS 状态类型 ==========

// UPSStatus UPS 设备状态
type UPSStatus string

const (
	// UPSStatusOnline 在线（市电供电）
	UPSStatusOnline UPSStatus = "online"
	// UPSStatusOnBattery 电池供电
	UPSStatusOnBattery UPSStatus = "on_battery"
	// UPSStatusLowBattery 低电量
	UPSStatusLowBattery UPSStatus = "low_battery"
	// UPSStatusCharging 充电中
	UPSStatusCharging UPSStatus = "charging"
	// UPSStatusFault 故障
	UPSStatusFault UPSStatus = "fault"
	// UPSStatusDisconnected 断开连接
	UPSStatusDisconnected UPSStatus = "disconnected"
	// UPSStatusUnknown 未知
	UPSStatusUnknown UPSStatus = "unknown"
)

// ========== 核心类型 ==========

// UPSDevice UPS 设备信息
type UPSDevice struct {
	ID           string    `json:"id"`                     // 设备唯一标识
	Name         string    `json:"name"`                   // 设备名称
	Model        string    `json:"model"`                  // 设备型号
	Manufacturer string    `json:"manufacturer,omitempty"` // 制造商
	SerialNumber string    `json:"serialNumber,omitempty"` // 序列号
	Firmware     string    `json:"firmware,omitempty"`     // 固件版本
	Protocol     Protocol  `json:"protocol"`               // 通信协议
	Address      string    `json:"address"`                // 连接地址（USB路径/IP:端口）
	Port         int       `json:"port,omitempty"`         // 端口号（SNMP/NUT）
	Status       UPSStatus `json:"status"`                 // 当前状态
	IsPrimary    bool      `json:"isPrimary"`              // 是否为主UPS
	ConnectedAt  time.Time `json:"connectedAt,omitempty"`  // 连接时间
	LastSeen     time.Time `json:"lastSeen"`               // 最后通信时间
	CreatedAt    time.Time `json:"createdAt"`              // 创建时间
}

// PowerStatus 电源状态信息
type PowerStatus struct {
	UPSID         string      `json:"upsId"`         // UPS ID
	Status        UPSStatus   `json:"status"`        // UPS 状态
	InputVoltage  float64     `json:"inputVoltage"`  // 输入电压 (V)
	OutputVoltage float64     `json:"outputVoltage"` // 输出电压 (V)
	InputFreq     float64     `json:"inputFreq"`     // 输入频率 (Hz)
	Load          float64     `json:"load"`          // 负载百分比 (%)
	Battery       BatteryInfo `json:"battery"`       // 电池信息
	Temperature   float64     `json:"temperature"`   // UPS 温度 (°C)
	RuntimeLeft   int         `json:"runtimeLeft"`   // 剩余运行时间 (分钟)
	UpdatedAt     time.Time   `json:"updatedAt"`     // 更新时间
}

// BatteryInfo 电池信息
type BatteryInfo struct {
	Charge      float64    `json:"charge"`                // 电量百分比 (%)
	Voltage     float64    `json:"voltage"`               // 电池电压 (V)
	Current     float64    `json:"current"`               // 充放电电流 (A)，正值为充电，负值为放电
	Temperature float64    `json:"temperature"`           // 电池温度 (°C)
	Health      string     `json:"health"`                // 健康状态 (good/replace/unknown)
	Capacity    float64    `json:"capacity"`              // 电池容量 (%)
	ReplaceDate *time.Time `json:"replaceDate,omitempty"` // 建议更换日期
	UpdatedAt   time.Time  `json:"updatedAt"`             // 更新时间
}

// HardwareHealth 硬件健康信息（IPMI/sensors）
type HardwareHealth struct {
	UPSID     string           `json:"upsId"`
	CPUTemp   float64          `json:"cpuTemp"`   // CPU 温度 (°C)
	DiskTemps []DiskTemp       `json:"diskTemps"` // 磁盘温度列表
	FanSpeeds []FanSpeed       `json:"fanSpeeds"` // 风扇转速列表
	Voltages  []VoltageReading `json:"voltages"`  // 电压读数
	UpdatedAt time.Time        `json:"updatedAt"`
}

// DiskTemp 磁盘温度
type DiskTemp struct {
	Device string  `json:"device"` // 设备名
	Temp   float64 `json:"temp"`   // 温度 (°C)
}

// FanSpeed 风扇转速
type FanSpeed struct {
	Name string `json:"name"` // 风扇名称
	RPM  int    `json:"rpm"`  // 转速
}

// VoltageReading 电压读数
type VoltageReading struct {
	Name  string  `json:"name"`  // 名称
	Value float64 `json:"value"` // 电压值 (V)
}

// ========== 电源事件 ==========

// PowerEventType 电源事件类型
type PowerEventType string

const (
	// EventPowerOut 市电断电
	EventPowerOut PowerEventType = "power_out"
	// EventPowerRestore 市电恢复
	EventPowerRestore PowerEventType = "power_restore"
	// EventBatteryLow 电池低电量
	EventBatteryLow PowerEventType = "battery_low"
	// EventBatteryCritical 电池严重低电量
	EventBatteryCritical PowerEventType = "battery_critical"
	// EventUPSSwitch 主备切换
	EventUPSSwitch PowerEventType = "ups_switch"
	// EventUPSConnected UPS 连接
	EventUPSSwitch2 PowerEventType = "ups_connected"
	// EventUPSDisconnected UPS 断开
	EventUPSDisconnected PowerEventType = "ups_disconnected"
	// EventShutdownInitiated 关机已启动
	EventShutdownInitiated PowerEventType = "shutdown_initiated"
	// EventShutdownComplete 关机完成
	EventShutdownComplete PowerEventType = "shutdown_complete"
	// EventHardwareAlert 硬件告警
	EventHardwareAlert PowerEventType = "hardware_alert"
)

// PowerEvent 电源事件
type PowerEvent struct {
	ID        string         `json:"id"`
	UPSID     string         `json:"upsId"`
	Type      PowerEventType `json:"type"`
	Message   string         `json:"message"`
	Details   string         `json:"details,omitempty"`
	Severity  string         `json:"severity"` // info/warning/critical
	Timestamp time.Time      `json:"timestamp"`
}

// ========== 关机策略 ==========

// ShutdownPolicy 优雅关机策略
type ShutdownPolicy struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Enabled          bool      `json:"enabled"`
	BatteryThreshold float64   `json:"batteryThreshold"` // 电池电量阈值 (%)
	DelaySeconds     int       `json:"delaySeconds"`     // 关机延迟（秒）
	RuntimeThreshold int       `json:"runtimeThreshold"` // 剩余运行时间阈值（分钟）
	NotifyBefore     int       `json:"notifyBefore"`     // 关机前通知时间（秒）
	Command          string    `json:"command"`          // 自定义关机命令
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// ========== 电源统计 ==========

// PowerStats 电源统计数据
type PowerStats struct {
	UPSID             string     `json:"upsId"`
	TotalEvents       int        `json:"totalEvents"`                // 总事件数
	PowerOutCount     int        `json:"powerOutCount"`              // 断电次数
	PowerOutDuration  int        `json:"powerOutDuration"`           // 累计断电时长（秒）
	BatteryDrainCount int        `json:"batteryDrainCount"`          // 电池放电次数
	AverageRuntime    float64    `json:"averageRuntime"`             // 平均电池运行时间（分钟）
	LastPowerOut      *time.Time `json:"lastPowerOut,omitempty"`     // 最近一次断电时间
	LastPowerRestore  *time.Time `json:"lastPowerRestore,omitempty"` // 最近一次恢复时间
	UptimePercent     float64    `json:"uptimePercent"`              // 可用性百分比
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// ========== 请求/响应类型 ==========

// DiscoverRequest UPS 设备发现请求
type DiscoverRequest struct {
	Protocol Protocol `json:"protocol" binding:"required"` // 通信协议
	Address  string   `json:"address,omitempty"`           // 可选的地址范围（SNMP扫描范围）
	Port     int      `json:"port,omitempty"`              // 可选端口
}

// ConnectRequest UPS 设备连接请求
type ConnectRequest struct {
	Name      string   `json:"name" binding:"required"`     // 设备名称
	Protocol  Protocol `json:"protocol" binding:"required"` // 通信协议
	Address   string   `json:"address" binding:"required"`  // 连接地址
	Port      int      `json:"port,omitempty"`              // 端口号
	IsPrimary bool     `json:"isPrimary"`                   // 是否为主UPS
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	PollInterval   int     `json:"pollInterval,omitempty"`   // 轮询间隔（秒）
	AlertThreshold float64 `json:"alertThreshold,omitempty"` // 告警电量阈值
	AutoSwitch     bool    `json:"autoSwitch"`               // 自动主备切换
	HistoryMax     int     `json:"historyMax,omitempty"`     // 最大历史记录数
}

// SetShutdownPolicyRequest 设置关机策略请求
type SetShutdownPolicyRequest struct {
	Name             string  `json:"name" binding:"required"`
	Enabled          bool    `json:"enabled"`
	BatteryThreshold float64 `json:"batteryThreshold" binding:"required"`
	DelaySeconds     int     `json:"delaySeconds"`
	RuntimeThreshold int     `json:"runtimeThreshold"`
	NotifyBefore     int     `json:"notifyBefore"`
	Command          string  `json:"command"`
}

// UPSStatusResponse UPS 状态响应
type UPSStatusResponse struct {
	UPS    *UPSDevice      `json:"ups"`
	Power  *PowerStatus    `json:"power,omitempty"`
	Health *HardwareHealth `json:"health,omitempty"`
}

// EventQueryParams 事件查询参数
type EventQueryParams struct {
	UPSID    string `form:"upsId"`
	Type     string `form:"type"`
	Severity string `form:"severity"`
	Limit    int    `form:"limit,default=50"`
	Offset   int    `form:"offset,default=0"`
}

// ========== 配置 ==========

// Config UPS 管理器配置
type Config struct {
	PollInterval   int     `json:"pollInterval"`   // 轮询间隔（秒），默认 10
	AlertThreshold float64 `json:"alertThreshold"` // 告警电量阈值 (%), 默认 20
	AutoSwitch     bool    `json:"autoSwitch"`     // 自动主备切换，默认 true
	HistoryMax     int     `json:"historyMax"`     // 最大历史记录数，默认 10000
	IPMIEnabled    bool    `json:"ipmiEnabled"`    // 是否启用 IPMI 监控
	SensorsPath    string  `json:"sensorsPath"`    // lm-sensors 路径
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		PollInterval:   10,
		AlertThreshold: 20.0,
		AutoSwitch:     true,
		HistoryMax:     10000,
		IPMIEnabled:    false,
		SensorsPath:    "/usr/bin/sensors",
	}
}
