// Package scrubsmart 提供智能避峰 Scrub 调度功能。
package scrubsmart

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrScrubNotRunning Scrub 未运行.
	ErrScrubNotRunning = errors.New("Scrub 未运行")
	// ErrScrubAlreadyRunning Scrub 已在运行.
	ErrScrubAlreadyRunning = errors.New("Scrub 已在运行")
	// ErrNoActiveScrub 无活跃 Scrub 任务.
	ErrNoActiveScrub = errors.New("无活跃 Scrub 任务")
	// ErrInvalidWindow 无效的避峰窗口.
	ErrInvalidWindow = errors.New("无效的避峰窗口配置")
)

// ========== 星期枚举 ==========

// Weekday 星期枚举.
type Weekday int

const (
	WeekdaySunday    Weekday = 0
	WeekdayMonday    Weekday = 1
	WeekdayTuesday   Weekday = 2
	WeekdayWednesday Weekday = 3
	WeekdayThursday  Weekday = 4
	WeekdayFriday    Weekday = 5
	WeekdaySaturday  Weekday = 6
)

// ========== Scrub 状态 ==========

// ScrubState Scrub 执行状态.
type ScrubState string

const (
	StateIdle     ScrubState = "idle"     // 空闲
	StateRunning  ScrubState = "running"  // 运行中
	StatePaused   ScrubState = "paused"   // 已暂停（避峰）
	StateManual   ScrubState = "manual"   // 手动暂停
	StateComplete ScrubState = "complete" // 已完成
	StateError    ScrubState = "error"    // 出错
)

// ========== 避峰配置 ==========

// AvoidanceWindow 避峰时间窗口.
type AvoidanceWindow struct {
	// Name 窗口名称，如"工作日白天".
	Name string `json:"name"`
	// Weekdays 生效的星期几（0=周日, 1-6=周一到周六）.
	Weekdays []Weekday `json:"weekdays"`
	// StartHour 开始小时（24小时制）.
	StartHour int `json:"start_hour"`
	// StartMinute 开始分钟.
	StartMinute int `json:"start_minute"`
	// EndHour 结束小时（24小时制）.
	EndHour int `json:"end_hour"`
	// EndMinute 结束分钟.
	EndMinute int `json:"end_minute"`
}

// Validate 验证窗口配置.
func (w *AvoidanceWindow) Validate() error {
	if w.StartHour < 0 || w.StartHour > 23 {
		return ErrInvalidWindow
	}
	if w.EndHour < 0 || w.EndHour > 23 {
		return ErrInvalidWindow
	}
	if w.StartMinute < 0 || w.StartMinute > 59 {
		return ErrInvalidWindow
	}
	if w.EndMinute < 0 || w.EndMinute > 59 {
		return ErrInvalidWindow
	}
	if len(w.Weekdays) == 0 {
		return ErrInvalidWindow
	}
	for _, d := range w.Weekdays {
		if d < 0 || d > 6 {
			return ErrInvalidWindow
		}
	}
	return nil
}

// Contains 检查给定时间是否在避峰窗口内.
func (w *AvoidanceWindow) Contains(t time.Time) bool {
	weekday := Weekday(t.Weekday())
	found := false
	for _, d := range w.Weekdays {
		if d == weekday {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	start := w.StartHour*60 + w.StartMinute
	end := w.EndHour*60 + w.EndMinute
	now := t.Hour()*60 + t.Minute()

	if start <= end {
		// 同一天内，如 9:00-18:00
		return now >= start && now < end
	}
	// 跨午夜，如 22:00-06:00
	return now >= start || now < end
}

// ========== 配置 ==========

// Config Scrub 智能调度配置.
type Config struct {
	// Enabled 是否启用智能避峰调度.
	Enabled bool `json:"enabled"`
	// AvoidanceWindows 避峰时间窗口列表.
	AvoidanceWindows []AvoidanceWindow `json:"avoidance_windows"`
	// IOWriteThresholdMBps IO 写入阈值（MB/s），超过此值暂停 Scrub.
	IOWriteThresholdMBps float64 `json:"io_write_threshold_mbps"`
	// IOReadThresholdMBps IO 读取阈值（MB/s），超过此值暂停 Scrub.
	IOReadThresholdMBps float64 `json:"io_read_threshold_mbps"`
	// IOCheckIntervalSeconds IO 负载检查间隔（秒）.
	IOCheckIntervalSeconds int `json:"io_check_interval_seconds"`
	// TargetPool 目标存储池名称.
	TargetPool string `json:"target_pool"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		AvoidanceWindows: []AvoidanceWindow{
			{
				Name:        "工作日白天",
				Weekdays:    []Weekday{WeekdayMonday, WeekdayTuesday, WeekdayWednesday, WeekdayThursday, WeekdayFriday},
				StartHour:   9,
				StartMinute: 0,
				EndHour:     18,
				EndMinute:   0,
			},
		},
		IOWriteThresholdMBps:   50.0,
		IOReadThresholdMBps:    100.0,
		IOCheckIntervalSeconds: 30,
		TargetPool:             "",
	}
}

// ========== 状态 ==========

// ScrubProgress Scrub 进度信息.
type ScrubProgress struct {
	// Percentage 完成百分比 0-100.
	Percentage float64 `json:"percentage"`
	// BytesScanned 已扫描字节数.
	BytesScanned int64 `json:"bytes_scanned"`
	// BytesTotal 总字节数.
	BytesTotal int64 `json:"bytes_total"`
	// Errors 错误数.
	Errors int64 `json:"errors"`
	// Duration 已运行时长.
	Duration time.Duration `json:"duration"`
	// EstimatedRemaining 预估剩余时长.
	EstimatedRemaining time.Duration `json:"estimated_remaining"`
}

// IOLoad 当前 IO 负载.
type IOLoad struct {
	// ReadMBps 当前读取速度 MB/s.
	ReadMBps float64 `json:"read_mbps"`
	// WriteMBps 当前写入速度 MB/s.
	WriteMBps float64 `json:"write_mbps"`
	// Timestamp 采集时间.
	Timestamp time.Time `json:"timestamp"`
}

// Status Scrub 智能调度状态.
type Status struct {
	// State Scrub 当前状态.
	State ScrubState `json:"state"`
	// Pool 存储池名称.
	Pool string `json:"pool"`
	// Progress 进度信息.
	Progress *ScrubProgress `json:"progress,omitempty"`
	// IOLoad 当前 IO 负载.
	IOLoad *IOLoad `json:"io_load,omitempty"`
	// InAvoidanceWindow 是否在避峰窗口内.
	InAvoidanceWindow bool `json:"in_avoidance_window"`
	// NextResume 预计恢复时间.
	NextResume *time.Time `json:"next_resume,omitempty"`
	// LastError 最近错误.
	LastError string `json:"last_error,omitempty"`
	// Config 当前配置.
	Config Config `json:"config"`
}

// ========== API 请求/响应 ==========

// PauseRequest 手动暂停请求.
type PauseRequest struct {
	// Reason 暂停原因.
	Reason string `json:"reason"`
}

// ResumeRequest 手动恢复请求.
type ResumeRequest struct {
	// Force 强制恢复（忽略避峰窗口）.
	Force bool `json:"force"`
}
