// Package scheduler 提供定时任务调度器功能
// Version: v2.49.0 - 定时任务调度器模块
package scheduler

import (
	"context"
	"time"
)

// TaskStatus 任务状态.
type TaskStatus string

const (
	// TaskStatusPending 任务等待中.
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning 任务正在运行.
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusCompleted 任务已完成.
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 任务失败.
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled 任务已取消.
	TaskStatusCancelled TaskStatus = "cancelled"
	// TaskStatusPaused 任务已暂停.
	TaskStatusPaused TaskStatus = "paused"
)

// TaskPriority 任务优先级.
type TaskPriority int

const (
	// PriorityLow 低优先级.
	PriorityLow TaskPriority = 1
	// PriorityNormal 普通优先级.
	PriorityNormal TaskPriority = 5
	// PriorityHigh 高优先级.
	PriorityHigh TaskPriority = 10
)

// TaskType 任务类型.
type TaskType string

const (
	// TaskTypeCron Cron表达式任务.
	TaskTypeCron TaskType = "cron"
	// TaskTypeOneTime 一次性任务.
	TaskTypeOneTime TaskType = "onetime"
	// TaskTypeInterval 间隔任务.
	TaskTypeInterval TaskType = "interval"
	// TaskTypeEvent 事件触发任务.
	TaskTypeEvent TaskType = "event"
	// TaskTypeDependent 依赖任务.
	TaskTypeDependent TaskType = "dependent"
)

// RetryPolicy 重试策略.
type RetryPolicy string

const (
	// RetryPolicyNone 无重试策略.
	RetryPolicyNone RetryPolicy = "none"
	// RetryPolicyFixed 固定间隔重试.
	RetryPolicyFixed RetryPolicy = "fixed"
	// RetryPolicyExponential 指数退避重试.
	RetryPolicyExponential RetryPolicy = "exponential"
)

// ExecutionStatus 执行状态.
type ExecutionStatus string

const (
	// ExecutionStatusStarted 执行已开始.
	ExecutionStatusStarted ExecutionStatus = "started"
	// ExecutionStatusCompleted 执行已完成.
	ExecutionStatusCompleted ExecutionStatus = "completed"
	// ExecutionStatusFailed 执行失败.
	ExecutionStatusFailed ExecutionStatus = "failed"
	// ExecutionStatusTimeout 执行超时.
	ExecutionStatusTimeout ExecutionStatus = "timeout"
	// ExecutionStatusCancelled 执行已取消.
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// Task 任务定义.
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

// TaskExecution 任务执行记录.
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

// TaskLog 任务日志.
type TaskLog struct {
	ID          string                 `json:"id"`
	ExecutionID string                 `json:"executionId"`
	TaskID      string                 `json:"taskId"`
	Timestamp   time.Time              `json:"timestamp"`
	Level       string                 `json:"level"` // debug, info, warn, error
	Message     string                 `json:"message"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// TaskDependency 任务依赖.
type TaskDependency struct {
	TaskID     string   `json:"taskId"`
	DependsOn  []string `json:"dependsOn"`
	Condition  string   `json:"condition"` // all, any
	Timeout    string   `json:"timeout,omitempty"`
	FailAction string   `json:"failAction,omitempty"` // skip, fail, continue
}

// Config 调度器配置.
type Config struct {
	MaxConcurrentTasks int           `json:"maxConcurrentTasks"`
	DefaultTimeout     time.Duration `json:"defaultTimeout"`
	LogRetention       time.Duration `json:"logRetention"`
	StoragePath        string        `json:"storagePath"`
	EnableRecovery     bool          `json:"enableRecovery"`
	HeartbeatInterval  time.Duration `json:"heartbeatInterval"`
}

// TaskFilter 任务过滤条件.
type TaskFilter struct {
	Status   TaskStatus `json:"status,omitempty"`
	Type     TaskType   `json:"type,omitempty"`
	Group    string     `json:"group,omitempty"`
	Tags     []string   `json:"tags,omitempty"`
	Enabled  *bool      `json:"enabled,omitempty"`
	Page     int        `json:"page,omitempty"`
	PageSize int        `json:"pageSize,omitempty"`
}

// ExecutionFilter 执行记录过滤条件.
type ExecutionFilter struct {
	TaskID    string          `json:"taskId,omitempty"`
	Status    ExecutionStatus `json:"status,omitempty"`
	StartTime *time.Time      `json:"startTime,omitempty"`
	EndTime   *time.Time      `json:"endTime,omitempty"`
	Page      int             `json:"page,omitempty"`
	PageSize  int             `json:"pageSize,omitempty"`
}

// Stats 调度器统计.
type Stats struct {
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

// RunRequest 运行请求.
type RunRequest struct {
	TaskID      string                 `json:"taskId"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	TriggeredBy string                 `json:"triggeredBy,omitempty"`
}

// RunResponse 运行响应.
type RunResponse struct {
	ExecutionID string          `json:"executionId"`
	Status      ExecutionStatus `json:"status"`
	Message     string          `json:"message"`
}

// TaskHandler 任务处理器接口.
type TaskHandler interface {
	Name() string
	Execute(ctx context.Context, task *Task) (map[string]interface{}, error)
}

// Context 上下文（用于避免导入 context 包）.
type Context interface {
	Done() <-chan struct{}
	Err() error
	Deadline() (time.Time, bool)
}
