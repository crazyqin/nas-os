package taskqueue

import (
	"errors"
	"sync"
	"time"
)

// ========== 任务状态 ==========

// TaskStatus 任务状态.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"   // 等待执行
	StatusReady     TaskStatus = "ready"     // 就绪（依赖已满足）
	StatusRunning   TaskStatus = "running"   // 执行中
	StatusSuccess   TaskStatus = "success"   // 成功
	StatusFailed    TaskStatus = "failed"    // 失败
	StatusCancelled TaskStatus = "cancelled" // 已取消
	StatusRetrying  TaskStatus = "retrying"  // 重试中
	StatusTimeout   TaskStatus = "timeout"   // 超时
	StatusDead      TaskStatus = "dead"      // 死信（重试耗尽）
)

// ========== 任务优先级 ==========

// TaskPriority 任务优先级.
type TaskPriority int

const (
	PriorityLow    TaskPriority = 0
	PriorityNormal TaskPriority = 1
	PriorityHigh   TaskPriority = 2
	PriorityUrgent TaskPriority = 3
)

func (p TaskPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityUrgent:
		return "urgent"
	default:
		return "unknown"
	}
}

// ParsePriority 解析优先级字符串.
func ParsePriority(s string) TaskPriority {
	switch s {
	case "low":
		return PriorityLow
	case "normal":
		return PriorityNormal
	case "high":
		return PriorityHigh
	case "urgent":
		return PriorityUrgent
	default:
		return PriorityNormal
	}
}

// ========== 错误定义 ==========

var (
	ErrTaskNotFound      = errors.New("任务不存在")
	ErrTaskNotCancellable = errors.New("任务不可取消")
	ErrTaskNotRunnable   = errors.New("任务不可运行")
	ErrCycleDetected     = errors.New("依赖关系存在循环")
	ErrDuplicateDep      = errors.New("重复依赖")
	ErrSelfDependency    = errors.New("不能依赖自身")
	ErrQueueFull         = errors.New("队列已满")
	ErrQueueStopped      = errors.New("队列已停止")
)

// ========== 任务定义 ==========

// TaskHandler 任务处理函数.
// 返回 error 表示失败，nil 表示成功.
type TaskHandler func(ctx *TaskContext) error

// TaskContext 任务执行上下文.
type TaskContext struct {
	TaskID   string                 `json:"task_id"`
	Name     string                 `json:"name"`
	Payload  map[string]interface{} `json:"payload"`
	Progress float64                `json:"progress"` // 0.0 - 1.0

	cancel chan struct{}
	mu     sync.RWMutex
}

// Cancel 取消信号.
func (ctx *TaskContext) Cancel() <-chan struct{} {
	return ctx.cancel
}

// SetProgress 设置进度 (0.0 - 1.0).
func (ctx *TaskContext) SetProgress(p float64) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	ctx.Progress = p
}

// GetProgress 获取进度.
func (ctx *TaskContext) GetProgress() float64 {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.Progress
}

// ProgressCallback 进度回调函数.
type ProgressCallback func(taskID string, progress float64)

// CompletionCallback 完成回调函数.
type CompletionCallback func(taskID string, status TaskStatus, err error)

// Task 任务定义.
type Task struct {
	mu          sync.RWMutex `json:"-"`
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Status      TaskStatus             `json:"status"`
	Priority    TaskPriority           `json:"priority"`
	Payload     map[string]interface{} `json:"payload,omitempty"`

	// 重试配置
	MaxRetries    int           `json:"max_retries"`
	RetryCount    int           `json:"retry_count"`
	RetryDelay    time.Duration `json:"retry_delay"`
	BackoffFactor float64       `json:"backoff_factor"` // 退避因子

	// 超时配置
	Timeout time.Duration `json:"timeout"`

	// 依赖关系
	Dependencies []string `json:"dependencies,omitempty"` // 依赖的任务ID列表

	// 时间戳
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// 执行信息
	Progress    float64 `json:"progress"` // 0.0 - 1.0
	Error       string  `json:"error,omitempty"`
	Result      interface{} `json:"result,omitempty"`

	// 内部字段
	handler   TaskHandler          `json:"-"`
	onProgress ProgressCallback    `json:"-"`
	onComplete CompletionCallback  `json:"-"`
	cancel     chan struct{}        `json:"-"`
	cancelOnce sync.Once           `json:"-"`
}

// ========== 队列统计 ==========

// QueueStats 队列统计信息.
type QueueStats struct {
	TotalTasks     int            `json:"total_tasks"`
	ByStatus       map[string]int `json:"by_status"`
	ByPriority     map[string]int `json:"by_priority"`
	RunningWorkers int            `json:"running_workers"`
	MaxWorkers     int            `json:"max_workers"`
	QueueSize      int            `json:"queue_size"`
	DeadLetterSize int            `json:"dead_letter_size"`
	AvgWaitTime    float64        `json:"avg_wait_time_ms"`
	AvgExecTime    float64        `json:"avg_exec_time_ms"`
}

// ========== 配置 ==========

// Config 队列配置.
type Config struct {
	Enabled         bool          `json:"enabled"`
	MaxWorkers      int           `json:"max_workers"`       // 最大并发Worker数
	MaxQueueSize    int           `json:"max_queue_size"`    // 最大队列长度 (0=无限制)
	DefaultRetry    int           `json:"default_retry"`     // 默认重试次数
	DefaultTimeout  time.Duration `json:"default_timeout"`   // 默认超时
	DefaultDelay    time.Duration `json:"default_retry_delay"` // 默认重试延迟
	DeadLetterLimit int           `json:"dead_letter_limit"` // 死信队列上限
	PollInterval    time.Duration `json:"poll_interval"`     // 调度轮询间隔
}

// DefaultConfig 默认配置.
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		MaxWorkers:      4,
		MaxQueueSize:    0,
		DefaultRetry:    3,
		DefaultTimeout:  30 * time.Second,
		DefaultDelay:    1 * time.Second,
		DeadLetterLimit: 1000,
		PollInterval:    100 * time.Millisecond,
	}
}

// ========== 过滤器 ==========

// TaskFilter 任务过滤器.
type TaskFilter struct {
	Status   []TaskStatus   `json:"status,omitempty"`
	Priority []TaskPriority `json:"priority,omitempty"`
	Name     string         `json:"name,omitempty"`
	Limit    int            `json:"limit,omitempty"`
	Offset   int            `json:"offset,omitempty"`
}
