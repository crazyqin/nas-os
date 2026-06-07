// Package smartpower 提供智能电源管理功能，涵盖磁盘休眠策略、CPU 频率调节、
// 温度监控与告警、定时开关机计划、功耗统计与报表、唤醒事件管理。
package smartpower

import (
	"errors"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrDiskNotFound 磁盘未找到.
	ErrDiskNotFound = errors.New("disk not found")
	// ErrProfileNotFound 电源方案未找到.
	ErrProfileNotFound = errors.New("profile not found")
	// ErrScheduleNotFound 调度计划未找到.
	ErrScheduleNotFound = errors.New("schedule not found")
	// ErrThermalZoneNotFound 温度区域未找到.
	ErrThermalZoneNotFound = errors.New("thermal zone not found")
	// ErrInvalidConfig 无效配置.
	ErrInvalidConfig = errors.New("invalid config")
)

// ========== 电源管理器配置 ==========

// PowerConfig 电源管理器配置.
type PowerConfig struct {
	// DiskSpindownSec 默认磁盘休眠时间（秒），0 表示禁用
	DiskSpindownSec int `json:"diskSpindownSec" yaml:"diskSpindownSec"`
	// CPUGovernor 默认 CPU 调频策略
	CPUGovernor string `json:"cpuGovernor" yaml:"cpuGovernor"`
	// TempCheckIntervalSec 温度检测间隔（秒）
	TempCheckIntervalSec int `json:"tempCheckIntervalSec" yaml:"tempCheckIntervalSec"`
	// FanSpeed 默认风扇转速（百分比，0-100）
	FanSpeed int `json:"fanSpeed" yaml:"fanSpeed"`
	// Enabled 是否启用智能电源管理
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// DefaultPowerConfig 返回默认配置.
func DefaultPowerConfig() *PowerConfig {
	return &PowerConfig{
		DiskSpindownSec:      1800,
		CPUGovernor:          "ondemand",
		TempCheckIntervalSec: 30,
		FanSpeed:             50,
		Enabled:              true,
	}
}

// ========== 磁盘状态 ==========

// DiskState 磁盘状态.
type DiskState struct {
	// Name 磁盘名称（如 sda, sdb）
	Name string `json:"name"`
	// IsSpinning 是否正在旋转
	IsSpinning bool `json:"isSpinning"`
	// LastAccess 最后访问时间
	LastAccess time.Time `json:"lastAccess"`
	// Temperature 磁盘温度（℃）
	Temperature float64 `json:"temperature"`
	// SpindownTimer 休眠倒计时（秒），0 表示无定时器
	SpindownTimer int `json:"spindownTimer"`
}

// ========== 电源方案 ==========

// PowerProfile 电源方案.
type PowerProfile struct {
	// Name 方案名称
	Name string `json:"name"`
	// CPUGovernor CPU 调频策略 (performance / powersave / ondemand / conservative)
	CPUGovernor string `json:"cpuGovernor"`
	// DiskSpindownSec 磁盘休眠时间（秒）
	DiskSpindownSec int `json:"diskSpindownSec"`
	// FanSpeed 风扇转速（百分比，0-100）
	FanSpeed int `json:"fanSpeed"`
	// WakeSchedule 唤醒计划列表
	WakeSchedule []*WakeSchedule `json:"wakeSchedule,omitempty"`
}

// ========== 功耗统计 ==========

// PowerStats 功耗统计.
type PowerStats struct {
	// CurrentWatts 当前实时功耗（瓦）
	CurrentWatts float64 `json:"currentWatts"`
	// DailyKWh 今日用电量（千瓦时）
	DailyKWh float64 `json:"dailyKWh"`
	// MonthlyKWh 本月用电量（千瓦时）
	MonthlyKWh float64 `json:"monthlyKWh"`
	// CostEstimate 预估月度电费（元）
	CostEstimate float64 `json:"costEstimate"`
}

// ========== 温度区域 ==========

// ThermalZone 温度区域.
type ThermalZone struct {
	// Name 区域名称（如 CPU, HDD, System）
	Name string `json:"name"`
	// Temperature 当前温度（℃）
	Temperature float64 `json:"temperature"`
	// Threshold 告警阈值（℃）
	Threshold float64 `json:"threshold"`
	// Critical 临界温度（℃）
	Critical float64 `json:"critical"`
}

// ========== 唤醒调度 ==========

// WakeSchedule 唤醒/关机调度计划.
type WakeSchedule struct {
	// ID 唯一标识
	ID string `json:"id"`
	// Name 计划名称
	Name string `json:"name"`
	// Action 动作类型: "wake" (开机) / "shutdown" (关机)
	Action string `json:"action"`
	// CronExpr cron 表达式
	CronExpr string `json:"cronExpr"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
}

// ========== 电源管理器 ==========

// PowerManager 电源管理器.
type PowerManager struct {
	config       *PowerConfig
	disks        map[string]*DiskState
	profile      *PowerProfile
	schedules    map[string]*WakeSchedule
	thermalZones map[string]*ThermalZone
	stats        *PowerStats
	mu           sync.RWMutex
}
