// Package scrubscheduler 提供定时 ZFS Scrub 调度功能，
// 参考 TrueNAS 26 的 Scheduled Scrubbing 设计。
package scrubscheduler

import "time"

// ScrubState 表示 scrub 任务的状态
type ScrubState string

const (
	ScrubStateIdle     ScrubState = "idle"
	ScrubStateRunning  ScrubState = "running"
	ScrubStatePaused   ScrubState = "paused"
	ScrubStateCompleted ScrubState = "completed"
	ScrubStateFailed   ScrubState = "failed"
)

// MaintenanceWindow 定义维护时间窗口，用于避开业务高峰期
type MaintenanceWindow struct {
	// Start 是每天允许开始 scrub 的时间 (HH:MM 格式, 24h)
	Start string `json:"start"`
	// End 是每天必须停止 scrub 的时间 (HH:MM 格式, 24h)
	End string `json:"end"`
}

// ScrubSchedule 定义一个存储池的 scrub 调度配置
type ScrubSchedule struct {
	// ID 是调度的唯一标识
	ID string `json:"id"`
	// PoolName 是 ZFS 存储池名称
	PoolName string `json:"pool_name" binding:"required"`
	// Schedule 是 cron 表达式 (标准 5 字段: minute hour day month weekday)
	Schedule string `json:"schedule" binding:"required"`
	// MaintenanceWindow 定义维护时间窗口
	MaintenanceWindow MaintenanceWindow `json:"maintenance_window"`
	// MaxDuration 是 scrub 最大允许时长 (秒), 0 表示无限制
	MaxDuration int `json:"max_duration"`
	// RetryCount 是失败后的重试次数
	RetryCount int `json:"retry_count"`
	// Enabled 是否启用该调度
	Enabled bool `json:"enabled"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 最后更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// ScrubStatus 表示当前 scrub 运行状态
type ScrubStatus struct {
	// PoolName 是存储池名称
	PoolName string `json:"pool_name"`
	// State 是当前状态
	State ScrubState `json:"state"`
	// Progress 是完成百分比 (0-100)
	Progress float64 `json:"progress"`
	// StartTime 是本次 scrub 开始时间
	StartTime time.Time `json:"start_time,omitempty"`
	// EndTime 是本次 scrub 结束时间
	EndTime time.Time `json:"end_time,omitempty"`
	// Errors 记录遇到的错误
	Errors []string `json:"errors,omitempty"`
	// RetryAttempt 是当前重试次数
	RetryAttempt int `json:"retry_attempt"`
}

// ScrubHistory 记录一次 scrub 的执行结果
type ScrubHistory struct {
	// ID 是记录的唯一标识
	ID string `json:"id"`
	// ScheduleID 关联的调度 ID
	ScheduleID string `json:"schedule_id"`
	// PoolName 是存储池名称
	PoolName string `json:"pool_name"`
	// State 是最终状态
	State ScrubState `json:"state"`
	// Progress 是最终完成百分比
	Progress float64 `json:"progress"`
	// StartTime 开始时间
	StartTime time.Time `json:"start_time"`
	// EndTime 结束时间
	EndTime time.Time `json:"end_time"`
	// DurationSeconds 执行耗时 (秒)
	DurationSeconds float64 `json:"duration_seconds"`
	// Errors 记录遇到的错误
	Errors []string `json:"errors,omitempty"`
	// RetryAttempt 是第几次重试
	RetryAttempt int `json:"retry_attempt"`
	// CreatedAt 记录创建时间
	CreatedAt time.Time `json:"created_at"`
}

// CreateScheduleRequest 创建调度的请求体
type CreateScheduleRequest struct {
	PoolName          string            `json:"pool_name" binding:"required"`
	Schedule          string            `json:"schedule" binding:"required"`
	MaintenanceWindow MaintenanceWindow `json:"maintenance_window"`
	MaxDuration       int               `json:"max_duration"`
	RetryCount        int               `json:"retry_count"`
	Enabled           *bool             `json:"enabled"`
}

// UpdateScheduleRequest 更新调度的请求体
type UpdateScheduleRequest struct {
	Schedule          *string           `json:"schedule"`
	MaintenanceWindow *MaintenanceWindow `json:"maintenance_window"`
	MaxDuration       *int              `json:"max_duration"`
	RetryCount        *int              `json:"retry_count"`
	Enabled           *bool             `json:"enabled"`
}

// ScrubSchedulerConfig 调度器配置
type ScrubSchedulerConfig struct {
	// DefaultMaintenanceWindow 全局默认维护窗口
	DefaultMaintenanceWindow MaintenanceWindow `json:"default_maintenance_window"`
	// DefaultMaxDuration 默认最大时长 (秒)
	DefaultMaxDuration int `json:"default_max_duration"`
	// DefaultRetryCount 默认重试次数
	DefaultRetryCount int `json:"default_retry_count"`
	// PollIntervalSeconds 状态轮询间隔 (秒)
	PollIntervalSeconds int `json:"poll_interval_seconds"`
	// MaxHistoryRecords 最大历史记录数
	MaxHistoryRecords int `json:"max_history_records"`
}

// DefaultSchedulerConfig 返回默认配置
func DefaultSchedulerConfig() *ScrubSchedulerConfig {
	return &ScrubSchedulerConfig{
		DefaultMaintenanceWindow: MaintenanceWindow{
			Start: "00:00",
			End:   "06:00",
		},
		DefaultMaxDuration:  28800, // 8 hours
		DefaultRetryCount:   3,
		PollIntervalSeconds: 30,
		MaxHistoryRecords:   500,
	}
}
