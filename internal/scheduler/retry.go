package scheduler

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// RetryManager 重试管理器.
type RetryManager struct {
	retries   map[string]*retryState
	mu        sync.RWMutex
	scheduler *Scheduler
}

type retryState struct {
	task      *Task
	attempts  int
	nextRetry time.Time
	lastError error
	cancel    context.CancelFunc
}

// NewRetryManager 创建重试管理器.
func NewRetryManager() *RetryManager {
	return &RetryManager{
		retries: make(map[string]*retryState),
	}
}

// SetScheduler 设置调度器.
func (rm *RetryManager) SetScheduler(s *Scheduler) {
	rm.scheduler = s
}

// ShouldRetry 判断是否应该重试.
func (rm *RetryManager) ShouldRetry(task *Task, err error) bool {
	if task.RetryPolicy == RetryPolicyNone {
		return false
	}

	if task.MaxRetries <= 0 {
		return false
	}

	rm.mu.RLock()
	state, exists := rm.retries[task.ID]
	rm.mu.RUnlock()

	if !exists {
		return true
	}

	return state.attempts < task.MaxRetries
}

// CalculateDelay 计算重试延迟.
func (rm *RetryManager) CalculateDelay(task *Task, attempt int) time.Duration {
	switch task.RetryPolicy {
	case RetryPolicyFixed:
		return rm.parseInterval(task.RetryInterval)

	case RetryPolicyExponential:
		baseDelay := rm.parseInterval(task.RetryInterval)
		if baseDelay == 0 {
			baseDelay = time.Second
		}
		// 指数退避: baseDelay * 2^attempt
		delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
		// 最大延迟 1 小时
		if delay > time.Hour {
			delay = time.Hour
		}
		return delay

	default:
		return rm.parseInterval(task.RetryInterval)
	}
}

func (rm *RetryManager) parseInterval(s string) time.Duration {
	if s == "" {
		return 0
	}
	d, err := ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

// ScheduleRetry 安排重试.
func (rm *RetryManager) ScheduleRetry(task *Task, err error) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, exists := rm.retries[task.ID]
	if !exists {
		state = &retryState{
			task: task,
		}
		rm.retries[task.ID] = state
	}

	state.attempts++
	state.lastError = err

	if state.attempts >= task.MaxRetries {
		return fmt.Errorf("已达到最大重试次数: %d", task.MaxRetries)
	}

	delay := rm.CalculateDelay(task, state.attempts)
	state.nextRetry = time.Now().Add(delay)

	// 更新任务的重试计数
	task.RetryCount = state.attempts

	// 安排重试
	go func() {
		time.Sleep(delay)

		rm.mu.RLock()
		state, exists := rm.retries[task.ID]
		rm.mu.RUnlock()

		if !exists {
			return
		}

		if state.cancel != nil {
			// 已取消
			return
		}

		// 触发重试
		if rm.scheduler != nil {
			_, _ = rm.scheduler.RunTask(task.ID)
		}
	}()

	return nil
}

// CancelRetry 取消重试.
func (rm *RetryManager) CancelRetry(taskID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if state, exists := rm.retries[taskID]; exists {
		if state.cancel != nil {
			state.cancel()
		}
		delete(rm.retries, taskID)
	}
}

// GetRetryState 获取重试状态.
func (rm *RetryManager) GetRetryState(taskID string) (*RetryInfo, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	state, exists := rm.retries[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不在重试队列中")
	}

	return &RetryInfo{
		TaskID:     taskID,
		Attempts:   state.attempts,
		MaxRetries: state.task.MaxRetries,
		NextRetry:  state.nextRetry,
		LastError:  state.lastError,
	}, nil
}

// GetPendingRetries 获取等待重试的任务列表.
func (rm *RetryManager) GetPendingRetries() []*RetryInfo {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*RetryInfo, 0, len(rm.retries))
	for taskID, state := range rm.retries {
		result = append(result, &RetryInfo{
			TaskID:     taskID,
			Attempts:   state.attempts,
			MaxRetries: state.task.MaxRetries,
			NextRetry:  state.nextRetry,
		})
	}

	return result
}

// Clear 清除所有重试状态.
func (rm *RetryManager) Clear() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for _, state := range rm.retries {
		if state.cancel != nil {
			state.cancel()
		}
	}
	rm.retries = make(map[string]*retryState)
}

// RetryInfo 重试信息.
type RetryInfo struct {
	TaskID     string    `json:"taskId"`
	Attempts   int       `json:"attempts"`
	MaxRetries int       `json:"maxRetries"`
	NextRetry  time.Time `json:"nextRetry"`
	LastError  error     `json:"lastError,omitempty"`
}

// RetryExecutor 带重试的执行器包装.
type RetryExecutor struct {
	executor *Executor
	retryMgr *RetryManager
}

// NewRetryExecutor 创建带重试的执行器.
func NewRetryExecutor(executor *Executor) *RetryExecutor {
	return &RetryExecutor{
		executor: executor,
		retryMgr: NewRetryManager(),
	}
}

// Execute 执行任务（带重试）.
func (re *RetryExecutor) Execute(ctx context.Context, task *Task) (*TaskExecution, error) {
	execution, err := re.executor.ExecuteSync(ctx, task)

	if err == nil {
		// 执行成功，清除重试状态
		re.retryMgr.CancelRetry(task.ID)
		return execution, nil
	}

	// 执行失败，判断是否需要重试
	if re.retryMgr.ShouldRetry(task, err) {
		if scheduleErr := re.retryMgr.ScheduleRetry(task, err); scheduleErr != nil {
			return execution, scheduleErr
		}
		return execution, fmt.Errorf("任务失败，已安排重试: %w", err)
	}

	return execution, fmt.Errorf("任务失败: %w", err)
}

// GetRetryManager 获取重试管理器.
func (re *RetryExecutor) GetRetryManager() *RetryManager {
	return re.retryMgr
}

// DefaultRetryConfig 默认重试配置.
type DefaultRetryConfig struct {
	Policy     RetryPolicy
	MaxRetries int
	Interval   time.Duration
}

// DefaultRetryConfigs 默认重试配置映射.
var DefaultRetryConfigs = map[TaskType]DefaultRetryConfig{
	TaskTypeCron: {
		Policy:     RetryPolicyExponential,
		MaxRetries: 3,
		Interval:   time.Minute,
	},
	TaskTypeOneTime: {
		Policy:     RetryPolicyFixed,
		MaxRetries: 3,
		Interval:   5 * time.Minute,
	},
	TaskTypeInterval: {
		Policy:     RetryPolicyExponential,
		MaxRetries: 5,
		Interval:   time.Second,
	},
	TaskTypeDependent: {
		Policy:     RetryPolicyFixed,
		MaxRetries: 2,
		Interval:   time.Minute,
	},
}

// ApplyDefaultRetryConfig 应用默认重试配置.
func ApplyDefaultRetryConfig(task *Task) {
	if task.RetryPolicy == "" {
		if config, ok := DefaultRetryConfigs[task.Type]; ok {
			task.RetryPolicy = config.Policy
			task.MaxRetries = config.MaxRetries
			task.RetryInterval = config.Interval.String()
		}
	}
}

// RetryableError 可重试的错误.
type RetryableError struct {
	Err       error
	Retryable bool
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError 创建可重试错误.
func NewRetryableError(err error, retryable bool) *RetryableError {
	return &RetryableError{
		Err:       err,
		Retryable: retryable,
	}
}

// IsRetryable 检查错误是否可重试.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if retryable, ok := err.(*RetryableError); ok {
		return retryable.Retryable
	}

	// 默认所有错误都可以重试
	return true
}

// RetryContext 重试上下文.
type RetryContext struct {
	Attempt    int
	MaxAttempt int
	LastError  error
	StartTime  time.Time
}

// RetryFunc 重试函数.
type RetryFunc func(ctx context.Context, rctx *RetryContext) error

// DoRetry 执行重试.
func DoRetry(ctx context.Context, maxAttempts int, interval time.Duration, fn RetryFunc) error {
	rctx := &RetryContext{
		MaxAttempt: maxAttempts,
		StartTime:  time.Now(),
	}

	for rctx.Attempt = 1; rctx.Attempt <= maxAttempts; rctx.Attempt++ {
		err := fn(ctx, rctx)
		if err == nil {
			return nil
		}

		rctx.LastError = err

		if rctx.Attempt >= maxAttempts {
			return fmt.Errorf("重试次数耗尽: %w", err)
		}

		// 等待
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}

	return nil
}

// DoRetryWithBackoff 执行重试（指数退避）.
func DoRetryWithBackoff(ctx context.Context, maxAttempts int, baseInterval time.Duration, fn RetryFunc) error {
	rctx := &RetryContext{
		MaxAttempt: maxAttempts,
		StartTime:  time.Now(),
	}

	for rctx.Attempt = 1; rctx.Attempt <= maxAttempts; rctx.Attempt++ {
		err := fn(ctx, rctx)
		if err == nil {
			return nil
		}

		rctx.LastError = err

		if rctx.Attempt >= maxAttempts {
			return fmt.Errorf("重试次数耗尽: %w", err)
		}

		// 指数退避
		delay := time.Duration(float64(baseInterval) * math.Pow(2, float64(rctx.Attempt-1)))
		if delay > time.Hour {
			delay = time.Hour
		}

		// 等待
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil
}
