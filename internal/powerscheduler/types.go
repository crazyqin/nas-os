package powerscheduler

import (
	"errors"
	"time"
)

var (
	ErrScheduleNotFound    = errors.New("schedule not found")
	ErrScheduleExists      = errors.New("schedule already exists")
	ErrInvalidSchedule     = errors.New("invalid schedule config")
	ErrManagerClosed       = errors.New("manager closed")
)

// PowerAction 功耗动作
type PowerAction string

const (
	ActionSuspend   PowerAction = "suspend"
	ActionHibernate PowerAction = "hibernate"
	ActionShutdown  PowerAction = "shutdown"
	ActionWakeOnLan PowerAction = "wol"
	ActionThrottle  PowerAction = "throttle"
)

// DayOfWeek 星期
type DayOfWeek int

const (
	Sunday    DayOfWeek = 0
	Monday    DayOfWeek = 1
	Tuesday   DayOfWeek = 2
	Wednesday DayOfWeek = 3
	Thursday  DayOfWeek = 4
	Friday    DayOfWeek = 5
	Saturday  DayOfWeek = 6
)

// Schedule 功耗调度
type Schedule struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Action    PowerAction `json:"action"`
	Time      string      `json:"time"` // HH:MM
	Days      []DayOfWeek `json:"days"`
	Enabled   bool        `json:"enabled"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	LastRun   *time.Time  `json:"last_run,omitempty"`
	NextRun   *time.Time  `json:"next_run,omitempty"`
}

// PowerState 功耗状态
type PowerState struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskIOPS      int64   `json:"disk_iops"`
	NetworkMbps   float64 `json:"network_mbps"`
	TemperatureC  float64 `json:"temperature_c"`
	PowerWatts    float64 `json:"power_watts"`
	ThrottleLevel int     `json:"throttle_level"`
}

// ThrottleConfig 功耗节流配置
type ThrottleConfig struct {
	MaxCPUPercent   float64 `json:"max_cpu_percent"`
	MaxMemoryMB     int64   `json:"max_memory_mb"`
	SpinDownDisks   bool    `json:"spin_down_disks"`
	ReduceNetworkQoS bool   `json:"reduce_network_qos"`
}
