// Package scheduler 提供定时任务调度器功能
// Version: v2.49.0 - 定时任务调度器模块
package scheduler

import (
	"context"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusPaused    TaskStatus = "paused"
)

// TaskPriority 任务优先级
type TaskPriority int

const (
	PriorityLow    TaskPriority = 1
	PriorityNormal TaskPriority = 5
	PriorityHigh   TaskPriority = 10
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeCron      TaskType = "cron"      // Cron 表达式任务
	TaskTypeOneTime   TaskType = "onetime"   // 一次性任务
	TaskTypeInterval  TaskType = "interval"  // 间隔任务
	TaskTypeEvent     TaskType = "event"     // 事件触发任务
	TaskTypeDependent TaskType = "dependent" // 依赖任务
)

// RetryPolicy 重试策略
type RetryPolicy string

const (
	RetryPolicyNone        RetryPolicy = "none"
	RetryPolicyFixed       RetryPolicy = "fixed"
	RetryPolicyExponential RetryPolicy = "exponential"
)

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	ExecutionStatusStarted   ExecutionStatus = "started"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusTimeout   ExecutionStatus = "timeout"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// Task 任务定义
type Task struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Type        TaskType     `json:"type"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`

	// 调度配置
	CronExpression string     `json:"cronExpression,omitempty"` // Cron 表达式
	Interval       string     `json:"interval,omitempty"`       // 间隔时间
	ScheduledAt    *time.Time `json:"scheduledAt,omitempty"`    // 计划执行时间
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`      // 过期时间

	// 任务内容
	Handler    string                 `json:"handler"`              // 处理器名称
	Parameters map[string]interface{} `json:"parameters,omitempty"` // 任务参数

	// 依赖管理
	Dependencies    []string `json:"dependencies,omitempty"`    // 依赖的任务ID列表
	DependCondition string   `json:"dependCondition,omitempty"` // 依赖条件: all, any

	// 重试配置
	RetryPolicy   RetryPolicy `json:"retryPolicy,omitempty"`
	MaxRetries    int         `json:"maxRetries,omitempty"`
	RetryInterval string      `json:"retryInterval,omitempty"`
	RetryCount    int         `json:"retryCount,omitempty"`

	// 超时配置
	Timeout string `json:"timeout,omitempty"`

	// 并发控制
	AllowConcurrent bool `json:"allowConcurrent"` // 是否允许并发执行
	MaxInstances    int  `json:"maxInstances,omitempty"`

	// 标签和分类
	Tags    []string `json:"tags,omitempty"`
	Group   string   `json:"group,omitempty"`
	Enabled bool     `json:"enabled"`

	// 元数据
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	LastRunAt *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt *time.Time `json:"nextRunAt,omitempty"`

	// 统计
	RunCount     int `json:"runCount"`
	SuccessCount int `json:"successCount"`
	FailCount    int `json:"failCount"`
}

// TaskExecution 任务执行记录
type TaskExecution struct {
	ID           string                 `json:"id"`
	TaskID       string                 `json:"taskId"`
	TaskName     string                 `json:"taskName"`
	Status       ExecutionStatus        `json:"status"`
	StartedAt    time.Time              `json:"startedAt"`
	CompletedAt  *time.Time             `json:"completedAt,omitempty"`
	Duration     string                 `json:"duration,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Output       map[string]interface{} `json:"output,omitempty"`
	RetryAttempt int                    `json:"retryAttempt"`
	TriggeredBy  string                 `json:"triggeredBy,omitempty"` // manual, schedule, dependency
	NodeID       string                 `json:"nodeId,omitempty"`
}

// TaskLog 任务日志
type TaskLog struct {
	ID          string                 `json:"id"`
	ExecutionID string                 `json:"executionId"`
	TaskID      string                 `json:"taskId"`
	Timestamp   time.Time              `json:"timestamp"`
	Level       string                 `json:"level"` // debug, info, warn, error
	Message     string                 `json:"message"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// TaskDependency 任务依赖
type TaskDependency struct {
	TaskID     string   `json:"taskId"`
	DependsOn  []string `json:"dependsOn"`
	Condition  string   `json:"condition"` // all, any
	Timeout    string   `json:"timeout,omitempty"`
	FailAction string   `json:"failAction,omitempty"` // skip, fail, continue
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	MaxConcurrentTasks int           `json:"maxConcurrentTasks"`
	DefaultTimeout     time.Duration `json:"defaultTimeout"`
	LogRetention       time.Duration `json:"logRetention"`
	StoragePath        string        `json:"storagePath"`
	EnableRecovery     bool          `json:"enableRecovery"`
	HeartbeatInterval  time.Duration `json:"heartbeatInterval"`
}

// TaskFilter 任务过滤条件
type TaskFilter struct {
	Status   TaskStatus `json:"status,omitempty"`
	Type     TaskType   `json:"type,omitempty"`
	Group    string     `json:"group,omitempty"`
	Tags     []string   `json:"tags,omitempty"`
	Enabled  *bool      `json:"enabled,omitempty"`
	Page     int        `json:"page,omitempty"`
	PageSize int        `json:"pageSize,omitempty"`
}

// ExecutionFilter 执行记录过滤条件
type ExecutionFilter struct {
	TaskID    string          `json:"taskId,omitempty"`
	Status    ExecutionStatus `json:"status,omitempty"`
	StartTime *time.Time      `json:"startTime,omitempty"`
	EndTime   *time.Time      `json:"endTime,omitempty"`
	Page      int             `json:"page,omitempty"`
	PageSize  int             `json:"pageSize,omitempty"`
}

// SchedulerStats 调度器统计
type SchedulerStats struct {
	TotalTasks       int            `json:"totalTasks"`
	RunningTasks     int            `json:"runningTasks"`
	PendingTasks     int            `json:"pendingTasks"`
	CompletedTasks   int            `json:"completedTasks"`
	FailedTasks      int            `json:"failedTasks"`
	PausedTasks      int            `json:"pausedTasks"`
	TotalExecutions  int            `json:"totalExecutions"`
	TodayExecutions  int            `json:"todayExecutions"`
	AvgExecutionTime string         `json:"avgExecutionTime"`
	SuccessRate      float64        `json:"successRate"`
	TaskGroupStats   map[string]int `json:"taskGroupStats"`
}

// RunRequest 运行请求
type RunRequest struct {
	TaskID      string                 `json:"taskId"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	TriggeredBy string                 `json:"triggeredBy,omitempty"`
}

// RunResponse 运行响应
type RunResponse struct {
	ExecutionID string          `json:"executionId"`
	Status      ExecutionStatus `json:"status"`
	Message     string          `json:"message"`
}

// TaskHandler 任务处理器接口
type TaskHandler interface {
	Name() string
	Execute(ctx context.Context, task *Task) (map[string]interface{}, error)
}

// Context 上下文（用于避免导入 context 包）
type Context interface {
	Done() <-chan struct{}
	Err() error
	Deadline() (time.Time, bool)
}
