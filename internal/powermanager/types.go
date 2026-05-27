// Package powermanager 提供智能电源管理功能，涵盖 HDD 休眠、UPS 监控、功耗统计、定时开关机、
// 温度联动风扇、电源事件处理、节能建议、电源仪表盘及多设备联动管理。
package powermanager

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrDiskNotFound 磁盘未找到.
	ErrDiskNotFound = errors.New("disk not found")
	// ErrUPSNotFound UPS 设备未找到.
	ErrUPSNotFound = errors.New("ups not found")
	// ErrScheduleNotFound 定时任务未找到.
	ErrScheduleNotFound = errors.New("schedule not found")
	// ErrDeviceNotFound 设备未找到.
	ErrDeviceNotFound = errors.New("device not found")
	// ErrFanProfileNotFound 风扇配置未找到.
	ErrFanProfileNotFound = errors.New("fan profile not found")
	// ErrSuggestionNotFound 节能建议未找到.
	ErrSuggestionNotFound = errors.New("suggestion not found")
	// ErrEventNotFound 电源事件未找到.
	ErrEventNotFound = errors.New("event not found")
	// ErrHolidayNotFound 节假日未找到.
	ErrHolidayNotFound = errors.New("holiday not found")
)

// ========== HDD 休眠管理 ==========

// DiskState 磁盘运行状态.
type DiskState string

const (
	DiskStateActive  DiskState = "active"  // 活跃
	DiskStateStandby DiskState = "standby" // 待机（休眠）
	DiskStateSleep   DiskState = "sleep"   // 深度休眠
	DiskStateUnknown DiskState = "unknown" // 未知
)

// DiskInfo 磁盘信息.
type DiskInfo struct {
	ID            string    `json:"id"`
	Device        string    `json:"device"`         // 设备路径，如 /dev/sda
	Model         string    `json:"model"`          // 型号
	Serial        string    `json:"serial"`         // 序列号
	CapacityGB    int64     `json:"capacity_gb"`    // 容量（GB）
	State         DiskState `json:"state"`          // 当前状态
	Temperature   int       `json:"temperature"`    // 温度（℃）
	IdleSince     *time.Time `json:"idle_since"`    // 空闲起始时间
	IdleSeconds   int64     `json:"idle_seconds"`   // 已空闲秒数
	WakeCount     int       `json:"wake_count"`     // 唤醒次数
	LastWakeTime  *time.Time `json:"last_wake_time"` // 最近唤醒时间
	SpinDownCount int       `json:"spin_down_count"` // 降速次数
}

// SleepPolicy HDD 休眠策略.
type SleepPolicy struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	IdleThreshold  int    `json:"idle_threshold"`  // 空闲阈值（秒）
	Enabled        bool   `json:"enabled"`
	PerDiskControl bool   `json:"per_disk_control"` // 是否逐盘控制
	Disks          []string `json:"disks,omitempty"` // 受控磁盘列表（逐盘模式）
	ApmLevel       int    `json:"apm_level"`        // APM 等级 (1-255)
	SpindownMode   string `json:"spindown_mode"`    // spindown 模式：standby / sleep
}

// CreateSleepPolicyRequest 创建休眠策略请求.
type CreateSleepPolicyRequest struct {
	Name           string   `json:"name" binding:"required"`
	IdleThreshold  int      `json:"idle_threshold" binding:"required,min=10"`
	Enabled        bool     `json:"enabled"`
	PerDiskControl bool     `json:"per_disk_control"`
	Disks          []string `json:"disks,omitempty"`
	ApmLevel       int      `json:"apm_level"`
	SpindownMode   string   `json:"spindown_mode"`
}

// UpdateSleepPolicyRequest 更新休眠策略请求.
type UpdateSleepPolicyRequest struct {
	Name           *string  `json:"name,omitempty"`
	IdleThreshold  *int     `json:"idle_threshold,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	PerDiskControl *bool    `json:"per_disk_control,omitempty"`
	Disks          []string `json:"disks,omitempty"`
	ApmLevel       *int     `json:"apm_level,omitempty"`
	SpindownMode   *string  `json:"spindown_mode,omitempty"`
}

// ========== UPS 监控 ==========

// UPSStatus UPS 状态.
type UPSStatus string

const (
	UPSStatusOnline  UPSStatus = "online"  // 在线（市电供电）
	UPSStatusBattery UPSStatus = "battery" // 电池供电
	UPSStatusLowBatt UPSStatus = "low_battery" // 低电量
	UPSStatusFault   UPSStatus = "fault"   // 故障
)

// UPSInfo UPS 设备信息.
type UPSInfo struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Model         string    `json:"model"`
	Status        UPSStatus `json:"status"`
	BatteryLevel  int       `json:"battery_level"`  // 电量百分比
	LoadPercent   int       `json:"load_percent"`   // 负载百分比
	Temperature   int       `json:"temperature"`    // 温度（℃）
	RemainingMin  int       `json:"remaining_min"`  // 剩余时间（分钟）
	InputVoltage  float64   `json:"input_voltage"`  // 输入电压
	OutputVoltage float64   `json:"output_voltage"` // 输出电压
	Alerts        []string  `json:"alerts"`         // 告警列表
	LastUpdated   time.Time `json:"last_updated"`
}

// UpdateUPSInfoRequest 更新 UPS 信息请求.
type UpdateUPSInfoRequest struct {
	Name          *string  `json:"name,omitempty"`
	Model         *string  `json:"model,omitempty"`
	Status        *UPSStatus `json:"status,omitempty"`
	BatteryLevel  *int     `json:"battery_level,omitempty"`
	LoadPercent   *int     `json:"load_percent,omitempty"`
	Temperature   *int     `json:"temperature,omitempty"`
	RemainingMin  *int     `json:"remaining_min,omitempty"`
	InputVoltage  *float64 `json:"input_voltage,omitempty"`
	OutputVoltage *float64 `json:"output_voltage,omitempty"`
	Alerts        []string `json:"alerts,omitempty"`
}

// ========== 功耗统计 ==========

// PowerRecord 功耗记录.
type PowerRecord struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Watts     float64   `json:"watts"`     // 实时功率（瓦）
	Source    string    `json:"source"`     // 数据来源：ups / meter / estimate
}

// PowerStats 功耗统计汇总.
type PowerStats struct {
	TotalKWh      float64 `json:"total_kwh"`       // 总用电量（度）
	AvgWatts      float64 `json:"avg_watts"`       // 平均功率
	MaxWatts      float64 `json:"max_watts"`       // 最大功率
	MinWatts      float64 `json:"min_watts"`       // 最小功率
	Days          int     `json:"days"`            // 统计天数
	EstimatedCost float64 `json:"estimated_cost"`  // 预估电费（元）
}

// PowerTrendPoint 功耗趋势点.
type PowerTrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Watts     float64   `json:"watts"`
}

// RecordPowerRequest 记录功耗请求.
type RecordPowerRequest struct {
	Watts  float64 `json:"watts" binding:"required,min=0"`
	Source string  `json:"source"`
}

// ========== 定时开关机 ==========

// ScheduleType 调度类型.
type ScheduleType string

const (
	ScheduleTypeBoot  ScheduleType = "boot"  // 开机
	ScheduleTypeShutdown ScheduleType = "shutdown" // 关机
	ScheduleTypeReboot ScheduleType = "reboot" // 重启
)

// PowerSchedule 定时开关机任务.
type PowerSchedule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        ScheduleType `json:"type"`
	CronExpr    string       `json:"cron_expr"`    // Cron 表达式
	Enabled     bool         `json:"enabled"`
	TargetDevice string      `json:"target_device"` // 目标设备 ID
	HolidaySkip bool         `json:"holiday_skip"`  // 节假日跳过
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	LastRun     *time.Time   `json:"last_run"`      // 最近执行时间
	NextRun     *time.Time   `json:"next_run"`      // 下次执行时间
}

// CreateScheduleRequest 创建定时任务请求.
type CreateScheduleRequest struct {
	Name         string       `json:"name" binding:"required"`
	Type         ScheduleType `json:"type" binding:"required"`
	CronExpr     string       `json:"cron_expr" binding:"required"`
	Enabled      bool         `json:"enabled"`
	TargetDevice string       `json:"target_device"`
	HolidaySkip  bool         `json:"holiday_skip"`
}

// UpdateScheduleRequest 更新定时任务请求.
type UpdateScheduleRequest struct {
	Name         *string       `json:"name,omitempty"`
	Type         *ScheduleType `json:"type,omitempty"`
	CronExpr     *string       `json:"cron_expr,omitempty"`
	Enabled      *bool         `json:"enabled,omitempty"`
	TargetDevice *string       `json:"target_device,omitempty"`
	HolidaySkip  *bool         `json:"holiday_skip,omitempty"`
}

// Holiday 节假日例外.
type Holiday struct {
	ID     string `json:"id"`
	Date   string `json:"date"`   // 日期 YYYY-MM-DD
	Name   string `json:"name"`   // 节假日名称
	IsWork bool   `json:"is_work"` // 是否调休工作日
}

// CreateHolidayRequest 创建节假日请求.
type CreateHolidayRequest struct {
	Date   string `json:"date" binding:"required"`
	Name   string `json:"name" binding:"required"`
	IsWork bool   `json:"is_work"`
}

// WOLRequest 远程唤醒请求.
type WOLRequest struct {
	MACAddress string `json:"mac_address" binding:"required"`
	Broadcast  string `json:"broadcast"` // 广播地址，默认 255.255.255.255
	Port       int    `json:"port"`      // 端口，默认 9
}

// ========== 温度联动风扇控制 ==========

// FanMode 风扇模式.
type FanMode string

const (
	FanModeAuto   FanMode = "auto"   // 自动
	FanModeSilent FanMode = "silent" // 静音
	FanModePerf   FanMode = "performance" // 性能
	FanModeManual FanMode = "manual" // 手动
)

// FanCurvePoint 风扇曲线控制点.
type FanCurvePoint struct {
	TempC    int `json:"temp_c"`    // 温度阈值（℃）
	FanSpeed int `json:"fan_speed"` // 风扇转速百分比 (0-100)
}

// FanProfile 风扇配置.
type FanProfile struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Mode      FanMode          `json:"mode"`
	Curve     []FanCurvePoint  `json:"curve"`      // 风扇曲线
	MinSpeed  int              `json:"min_speed"`   // 最低转速
	MaxSpeed  int              `json:"max_speed"`   // 最高转速
	TargetTemp *int            `json:"target_temp"` // 目标温度（静音/性能模式）
	CurrentSpeed int           `json:"current_speed"` // 当前转速
	Enabled   bool             `json:"enabled"`
}

// CreateFanProfileRequest 创建风扇配置请求.
type CreateFanProfileRequest struct {
	Name       string          `json:"name" binding:"required"`
	Mode       FanMode         `json:"mode" binding:"required"`
	Curve      []FanCurvePoint `json:"curve,omitempty"`
	MinSpeed   int             `json:"min_speed"`
	MaxSpeed   int             `json:"max_speed"`
	TargetTemp *int            `json:"target_temp,omitempty"`
	Enabled    bool            `json:"enabled"`
}

// UpdateFanProfileRequest 更新风扇配置请求.
type UpdateFanProfileRequest struct {
	Name       *string         `json:"name,omitempty"`
	Mode       *FanMode        `json:"mode,omitempty"`
	Curve      []FanCurvePoint `json:"curve,omitempty"`
	MinSpeed   *int            `json:"min_speed,omitempty"`
	MaxSpeed   *int            `json:"max_speed,omitempty"`
	TargetTemp *int            `json:"target_temp,omitempty"`
	Enabled    *bool           `json:"enabled,omitempty"`
}

// ========== 电源事件处理 ==========

// PowerEventType 电源事件类型.
type PowerEventType string

const (
	EventPowerLoss     PowerEventType = "power_loss"     // 断电
	EventPowerRestore  PowerEventType = "power_restore"  // 来电
	EventBatteryLow    PowerEventType = "battery_low"    // UPS 低电量
	EventBatteryOK     PowerEventType = "battery_ok"     // UPS 电量恢复
	EventOverTemp      PowerEventType = "over_temp"      // 过温
	EventFanFailure    PowerEventType = "fan_failure"    // 风扇故障
)

// PowerEvent 电源事件.
type PowerEvent struct {
	ID        string         `json:"id"`
	Type      PowerEventType `json:"type"`
	Message   string         `json:"message"`
	Timestamp time.Time      `json:"timestamp"`
	Handled   bool           `json:"handled"`   // 是否已处理
	Action    string         `json:"action"`    // 执行的动作
}

// PowerEventRule 电源事件处理规则.
type PowerEventRule struct {
	ID          string         `json:"id"`
	EventType   PowerEventType `json:"event_type"`
	Enabled     bool           `json:"enabled"`
	Action      string         `json:"action"`       // shutdown / notify / reboot / wol
	Delay       int            `json:"delay"`        // 延迟执行秒数
	TargetDevs  []string       `json:"target_devices"` // 目标设备
	NotifyMsg   string         `json:"notify_msg"`   // 通知消息
	CreatedAt   time.Time      `json:"created_at"`
}

// CreateEventRuleRequest 创建事件规则请求.
type CreateEventRuleRequest struct {
	EventType  PowerEventType `json:"event_type" binding:"required"`
	Enabled    bool           `json:"enabled"`
	Action     string         `json:"action" binding:"required"`
	Delay      int            `json:"delay"`
	TargetDevs []string       `json:"target_devices,omitempty"`
	NotifyMsg  string         `json:"notify_msg"`
}

// UpdateEventRuleRequest 更新事件规则请求.
type UpdateEventRuleRequest struct {
	EventType  *PowerEventType `json:"event_type,omitempty"`
	Enabled    *bool           `json:"enabled,omitempty"`
	Action     *string         `json:"action,omitempty"`
	Delay      *int            `json:"delay,omitempty"`
	TargetDevs []string        `json:"target_devices,omitempty"`
	NotifyMsg  *string         `json:"notify_msg,omitempty"`
}

// ========== 节能建议 ==========

// SuggestionPriority 建议优先级.
type SuggestionPriority string

const (
	SuggestionPriorityHigh   SuggestionPriority = "high"
	SuggestionPriorityMedium SuggestionPriority = "medium"
	SuggestionPriorityLow    SuggestionPriority = "low"
)

// EnergySuggestion 节能建议.
type EnergySuggestion struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Priority    SuggestionPriority `json:"priority"`
	Category    string             `json:"category"`    // disk / fan / schedule / ups / general
	SavingsKWh  float64            `json:"savings_kwh"` // 预估节省度数/月
	CreatedAt   time.Time          `json:"created_at"`
	Applied     bool               `json:"applied"`     // 是否已采纳
}

// GenerateSuggestionsRequest 生成建议请求.
type GenerateSuggestionsRequest struct {
	Days int `json:"days"` // 分析天数，默认 30
}

// ========== 电源仪表盘 ==========

// DashboardOverview 电源仪表盘概览.
type DashboardOverview struct {
	TotalDisks      int            `json:"total_disks"`
	ActiveDisks     int            `json:"active_disks"`
	SleepingDisks   int            `json:"sleeping_disks"`
	UPS             *UPSInfo       `json:"ups,omitempty"`
	CurrentWatts    float64        `json:"current_watts"`
	TodayKWh        float64        `json:"today_kwh"`
	MonthKWh        float64        `json:"month_kwh"`
	EstMonthlyCost  float64        `json:"est_monthly_cost"`
	FanMode         FanMode        `json:"fan_mode"`
	FanSpeed        int            `json:"fan_speed"`
	CPUTemp         int            `json:"cpu_temp"`
	SystemTemp      int            `json:"system_temp"`
	RecentEvents    []PowerEvent   `json:"recent_events"`
	ActiveSchedules int            `json:"active_schedules"`
	ActiveAlerts    int            `json:"active_alerts"`
}

// PowerHistoryQuery 功耗历史查询.
type PowerHistoryQuery struct {
	Start   string `form:"start"`   // 开始时间 RFC3339
	End     string `form:"end"`     // 结束时间 RFC3339
	Period  string `form:"period"`  // 聚合周期：hour / day / week
	Limit   int    `form:"limit"`
}

// PowerHistoryPoint 历史数据点.
type PowerHistoryPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	AvgWatts   float64   `json:"avg_watts"`
	MaxWatts   float64   `json:"max_watts"`
	MinWatts   float64   `json:"min_watts"`
	TotalKWh   float64   `json:"total_kwh"`
}

// ========== 多设备电源管理 ==========

// DeviceType 设备类型.
type DeviceType string

const (
	DeviceTypeNAS   DeviceType = "nas"
	DeviceTypeUPS   DeviceType = "ups"
	DeviceTypeSwitch DeviceType = "switch"
	DeviceTypeRouter DeviceType = "router"
)

// DeviceStatus 设备状态.
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusSleep   DeviceStatus = "sleep"
	DeviceStatusUnknown DeviceStatus = "unknown"
)

// ManagedDevice 受管设备.
type ManagedDevice struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        DeviceType   `json:"type"`
	IPAddress   string       `json:"ip_address"`
	MACAddress  string       `json:"mac_address"`
	Status      DeviceStatus `json:"status"`
	LastSeen    *time.Time   `json:"last_seen"`
	WOLEnabled  bool         `json:"wol_enabled"`  // 是否支持 WOL
	Tags        []string     `json:"tags,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

// CreateDeviceRequest 创建设备请求.
type CreateDeviceRequest struct {
	Name       string     `json:"name" binding:"required"`
	Type       DeviceType `json:"type" binding:"required"`
	IPAddress  string     `json:"ip_address"`
	MACAddress string     `json:"mac_address"`
	WOLEnabled bool       `json:"wol_enabled"`
	Tags       []string   `json:"tags,omitempty"`
}

// UpdateDeviceRequest 更新设备请求.
type UpdateDeviceRequest struct {
	Name       *string      `json:"name,omitempty"`
	Type       *DeviceType  `json:"type,omitempty"`
	IPAddress  *string      `json:"ip_address,omitempty"`
	MACAddress *string      `json:"mac_address,omitempty"`
	Status     *DeviceStatus `json:"status,omitempty"`
	WOLEnabled *bool        `json:"wol_enabled,omitempty"`
	Tags       []string     `json:"tags,omitempty"`
}

// BulkPowerAction 批量电源操作.
type BulkPowerAction struct {
	Action  string   `json:"action" binding:"required"` // shutdown / reboot / wol
	Devices []string `json:"devices" binding:"required"` // 设备 ID 列表
}
