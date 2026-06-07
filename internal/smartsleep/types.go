package smartsleep

import (
	"errors"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	ErrDiskNotFound        = errors.New("disk not found")
	ErrScheduleConflict    = errors.New("schedule conflict")
	ErrInvalidConfig       = errors.New("invalid config")
	ErrDiskAlreadySleeping = errors.New("disk already sleeping")
)

// ========== 磁盘状态常量 ==========

const (
	StateActive   = "active"
	StateStandby  = "standby"
	StateSleep    = "sleep"
	StateSpindown = "spindown"
)

// ========== 配置 ==========

// Config 智能休眠配置.
type Config struct {
	// Enabled 是否启用智能休眠
	Enabled bool `json:"enabled"`
	// DefaultIdleSec 默认空闲休眠时间（秒）
	DefaultIdleSec int `json:"default_idle_sec"`
	// PredictionWindowMin 预测窗口（分钟）
	PredictionWindowMin int `json:"prediction_window_min"`
	// WeekendMode 周末模式: "aggressive" / "normal" / "disabled"
	WeekendMode string `json:"weekend_mode"`
	// ElectricityRate 电价（元/kWh）
	ElectricityRate float64 `json:"electricity_rate"`
	// CarbonFactor 碳排放因子（kg CO2/kWh）
	CarbonFactor float64 `json:"carbon_factor"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Enabled:             true,
		DefaultIdleSec:      1800,
		PredictionWindowMin: 30,
		WeekendMode:         "aggressive",
		ElectricityRate:     0.6,
		CarbonFactor:        0.5,
	}
}

// ========== 磁盘信息 ==========

// DiskInfo 磁盘基础信息.
type DiskInfo struct {
	// ID 磁盘唯一标识
	ID string `json:"id"`
	// Device 设备路径（如 /dev/sda）
	Device string `json:"device"`
	// Model 型号
	Model string `json:"model"`
	// Serial 序列号
	Serial string `json:"serial"`
	// State 当前状态
	State string `json:"state"`
	// LastAccess 最后访问时间
	LastAccess time.Time `json:"last_access"`
	// LastSleepTime 最后进入休眠时间
	LastSleepTime time.Time `json:"last_sleep_time"`
	// WakeCount 唤醒次数
	WakeCount int64 `json:"wake_count"`
	// TotalSleepSeconds 累计休眠秒数
	TotalSleepSeconds int64 `json:"total_sleep_seconds"`
	// WattsWhenActive 活跃功耗（瓦）
	WattsWhenActive float64 `json:"watts_when_active"`
	// WattsWhenSleep 休眠功耗（瓦）
	WattsWhenSleep float64 `json:"watts_when_sleep"`
}

// ========== 访问模式 ==========

// AccessPattern 磁盘访问模式.
type AccessPattern struct {
	// DiskID 磁盘ID
	DiskID string `json:"disk_id"`
	// HourlyAccess 24小时访问分布（0-23点）
	HourlyAccess [24]int `json:"hourly_access"`
	// WeeklyAccess 7天访问分布（0=周日，6=周六）
	WeeklyAccess [7]int `json:"weekly_access"`
	// AvgIdleSeconds 平均空闲时间（秒）
	AvgIdleSeconds float64 `json:"avg_idle_seconds"`
	// PeakHours 高峰时段
	PeakHours []int `json:"peak_hours"`
	// QuietHours 安静时段
	QuietHours []int `json:"quiet_hours"`
	// TotalRecords 总记录数
	TotalRecords int `json:"total_records"`
	// LastUpdated 最后更新时间
	LastUpdated time.Time `json:"last_updated"`
}

// ========== 休眠预测 ==========

// SleepPrediction 休眠预测结果.
type SleepPrediction struct {
	// DiskID 磁盘ID
	DiskID string `json:"disk_id"`
	// ShouldSleep 是否建议休眠
	ShouldSleep bool `json:"should_sleep"`
	// PredictedIdleMinutes 预计空闲时间（分钟）
	PredictedIdleMinutes float64 `json:"predicted_idle_minutes"`
	// Confidence 置信度（0-1）
	Confidence float64 `json:"confidence"`
	// Reason 建议原因
	Reason string `json:"reason"`
	// NextWakeTime 预计下次唤醒时间
	NextWakeTime time.Time `json:"next_wake_time"`
}

// ========== 周末策略 ==========

// WeekendPolicy 周末差异化策略.
type WeekendPolicy struct {
	// WeekendMode 周末模式
	WeekendMode string `json:"weekend_mode"`
	// WeekendIdleSec 周末空闲休眠时间（秒）
	WeekendIdleSec int `json:"weekend_idle_sec"`
	// WeekdayIdleSec 工作日空闲休眠时间（秒）
	WeekdayIdleSec int `json:"weekday_idle_sec"`
	// WeekendWakeHours 周末保持唤醒时段
	WakeHours []int `json:"wake_hours"`
	// WorkdayWakeHours 工作日保持唤醒时段
	WorkdayWakeHours []int `json:"workday_wake_hours"`
}

// ========== 定时任务联动 ==========

// ScheduledTask 定时任务信息.
type ScheduledTask struct {
	// ID 任务ID
	ID string `json:"id"`
	// Name 任务名称
	Name string `json:"name"`
	// CronExpr cron表达式
	CronExpr string `json:"cron_expr"`
	// DiskIDs 关联磁盘ID列表
	DiskIDs []string `json:"disk_ids"`
	// PreWakeSec 提前唤醒时间（秒）
	PreWakeSec int `json:"pre_wake_sec"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
}

// ========== 节能统计 ==========

// EnergyStats 节能统计.
type EnergyStats struct {
	// TotalSleepHours 累计休眠小时数
	TotalSleepHours float64 `json:"total_sleep_hours"`
	// WattsSaved 节省功率（瓦）
	WattsSaved float64 `json:"watts_saved"`
	// KWhSaved 节省电量（kWh）
	KWhSaved float64 `json:"kwh_saved"`
	// CostSaved 节省费用（元）
	CostSaved float64 `json:"cost_saved"`
	// CO2Reduced 减少碳排放（kg）
	CO2Reduced float64 `json:"co2_reduced_kg"`
	// TreesEquivalent 等效植树（棵）
	TreesEquivalent float64 `json:"trees_equivalent"`
	// DailyStats 每日统计
	DailyStats []DailyEnergyStat `json:"daily_stats"`
}

// DailyEnergyStat 每日节能统计.
type DailyEnergyStat struct {
	// Date 日期
	Date string `json:"date"`
	// SleepHours 休眠小时数
	SleepHours float64 `json:"sleep_hours"`
	// KWhSaved 节省电量
	KWhSaved float64 `json:"kwh_saved"`
	// CostSaved 节省费用
	CostSaved float64 `json:"cost_saved"`
}

// ========== 紧急唤醒 ==========

// WakeRequest 唤醒请求.
type WakeRequest struct {
	// DiskID 磁盘ID
	DiskID string `json:"disk_id"`
	// Reason 唤醒原因
	Reason string `json:"reason"`
	// Priority 优先级: "low" / "normal" / "high" / "urgent"
	Priority string `json:"priority"`
	// RequestedBy 请求来源
	RequestedBy string `json:"requested_by"`
}

// WakeResponse 唤醒响应.
type WakeResponse struct {
	// DiskID 磁盘ID
	DiskID string `json:"disk_id"`
	// Success 是否成功
	Success bool `json:"success"`
	// WakeTimeMs 唤醒耗时（毫秒）
	WakeTimeMs int64 `json:"wake_time_ms"`
	// Message 附加信息
	Message string `json:"message"`
}

// ========== 管理器 ==========

// Manager 智能休眠管理器.
type Manager struct {
	mu          sync.RWMutex
	config      *Config
	disks       map[string]*DiskInfo
	patterns    map[string]*AccessPattern
	predictions map[string]*SleepPrediction
	policy      *WeekendPolicy
	tasks       map[string]*ScheduledTask
	stats       *EnergyStats
	accessLog   []AccessRecord
	wakeChan    chan WakeRequest
}

// AccessRecord 访问记录.
type AccessRecord struct {
	DiskID    string    `json:"disk_id"`
	Timestamp time.Time `json:"timestamp"`
	IOBytes   int64     `json:"io_bytes"`
	OpType    string    `json:"op_type"`
}
