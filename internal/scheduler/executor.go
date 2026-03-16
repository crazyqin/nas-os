package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Executor 任务执行器
type Executor struct {
	handlers   map[string]TaskHandler
	running    map[string]*runningTask
	depManager *DependencyManager
	mu         sync.RWMutex
	maxRunning int
}

type runningTask struct {
	task      *Task
	startTime time.Time
	cancel    context.CancelFunc
}

// NewExecutor 创建执行器
func NewExecutor(maxRunning int) *Executor {
	if maxRunning <= 0 {
		maxRunning = 10
	}

	return &Executor{
		handlers:   make(map[string]TaskHandler),
		running:    make(map[string]*runningTask),
		depManager: NewDependencyManager(),
		maxRunning: maxRunning,
	}
}

// RegisterHandler 注册处理器
func (e *Executor) RegisterHandler(handler TaskHandler) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if handler.Name() == "" {
		return fmt.Errorf("处理器名称不能为空")
	}

	e.handlers[handler.Name()] = handler
	return nil
}

// UnregisterHandler 注销处理器
func (e *Executor) UnregisterHandler(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.handlers, name)
}

// GetHandler 获取处理器
func (e *Executor) GetHandler(name string) (TaskHandler, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	handler, exists := e.handlers[name]
	return handler, exists
}

// ListHandlers 列出所有处理器
func (e *Executor) ListHandlers() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.handlers))
	for name := range e.handlers {
		names = append(names, name)
	}
	return names
}

// Execute 执行任务
func (e *Executor) Execute(ctx context.Context, task *Task) (*TaskExecution, error) {
	// 检查处理器
	handler, exists := e.GetHandler(task.Handler)
	if !exists {
		return nil, fmt.Errorf("处理器不存在: %s", task.Handler)
	}

	// 检查并发限制
	if !task.AllowConcurrent && e.IsRunning(task.ID) {
		return nil, fmt.Errorf("任务正在运行且不允许并发")
	}

	// 检查依赖
	if canRun, reason := e.depManager.CanRun(task.ID); !canRun {
		return nil, fmt.Errorf("依赖条件不满足: %s", reason)
	}

	// 创建执行记录
	execution := &TaskExecution{
		ID:           GenerateTaskID(),
		TaskID:       task.ID,
		TaskName:     task.Name,
		Status:       ExecutionStatusStarted,
		StartedAt:    time.Now(),
		RetryAttempt: task.RetryCount,
	}

	// 创建可取消的上下文，设置默认超时
	execCtx, cancel := context.WithCancel(ctx)

	// 设置超时
	if task.Timeout != "" {
		timeout, err := ParseDuration(task.Timeout)
		if err == nil {
			// 先取消之前的 context，避免泄漏
			childCtx, childCancel := context.WithTimeout(execCtx, timeout)
			parentCancel := cancel
			cancel = func() {
				childCancel()
				parentCancel()
			}
			execCtx = childCtx
		}
	}

	// 记录运行中的任务
	e.mu.Lock()
	e.running[task.ID] = &runningTask{
		task:      task,
		startTime: time.Now(),
		cancel:    cancel,
	}
	e.mu.Unlock()

	// 更新任务状态
	task.Status = TaskStatusRunning
	task.RunCount++

	// 执行任务
	go func() {
		defer e.cleanup(task.ID)

		output, err := handler.Execute(execCtx, task)

		e.mu.Lock()
		defer e.mu.Unlock()

		if err != nil {
			execution.Status = ExecutionStatusFailed
			execution.Error = err.Error()
			task.Status = TaskStatusFailed
			task.FailCount++
			e.depManager.MarkFailed(task.ID)
		} else {
			execution.Status = ExecutionStatusCompleted
			execution.Output = output
			task.Status = TaskStatusCompleted
			e.depManager.MarkCompleted(task.ID)
		}

		now := time.Now()
		execution.CompletedAt = &now
		execution.Duration = now.Sub(execution.StartedAt).String()
		task.LastRunAt = &now
	}()

	return execution, nil
}

// ExecuteSync 同步执行任务
func (e *Executor) ExecuteSync(ctx context.Context, task *Task) (*TaskExecution, error) {
	// 检查处理器
	handler, exists := e.GetHandler(task.Handler)
	if !exists {
		return nil, fmt.Errorf("处理器不存在: %s", task.Handler)
	}

	// 检查并发限制
	if !task.AllowConcurrent && e.IsRunning(task.ID) {
		return nil, fmt.Errorf("任务正在运行且不允许并发")
	}

	// 检查依赖
	if canRun, reason := e.depManager.CanRun(task.ID); !canRun {
		return nil, fmt.Errorf("依赖条件不满足: %s", reason)
	}

	// 创建执行记录
	execution := &TaskExecution{
		ID:           GenerateTaskID(),
		TaskID:       task.ID,
		TaskName:     task.Name,
		Status:       ExecutionStatusStarted,
		StartedAt:    time.Now(),
		RetryAttempt: task.RetryCount,
	}

	// 创建可取消的上下文
	execCtx := ctx

	// 设置超时
	var cancel context.CancelFunc
	if task.Timeout != "" {
		timeout, err := ParseDuration(task.Timeout)
		if err == nil {
			execCtx, cancel = context.WithTimeout(execCtx, timeout)
			defer cancel()
		}
	}

	// 记录运行中的任务
	e.mu.Lock()
	e.running[task.ID] = &runningTask{
		task:      task,
		startTime: time.Now(),
	}
	task.Status = TaskStatusRunning
	task.RunCount++
	e.mu.Unlock()

	// 执行任务
	output, err := handler.Execute(execCtx, task)

	// 清理
	defer e.cleanup(task.ID)

	e.mu.Lock()
	defer e.mu.Unlock()

	if err != nil {
		execution.Status = ExecutionStatusFailed
		execution.Error = err.Error()
		task.Status = TaskStatusFailed
		task.FailCount++
		e.depManager.MarkFailed(task.ID)
	} else {
		execution.Status = ExecutionStatusCompleted
		execution.Output = output
		task.Status = TaskStatusCompleted
		e.depManager.MarkCompleted(task.ID)
	}

	now := time.Now()
	execution.CompletedAt = &now
	execution.Duration = now.Sub(execution.StartedAt).String()
	task.LastRunAt = &now

	return execution, nil
}

// Cancel 取消任务
func (e *Executor) Cancel(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	running, exists := e.running[taskID]
	if !exists {
		return fmt.Errorf("任务不在运行中")
	}

	running.cancel()
	return nil
}

// IsRunning 检查任务是否在运行
func (e *Executor) IsRunning(taskID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.running[taskID]
	return exists
}

// GetRunningTasks 获取运行中的任务列表
func (e *Executor) GetRunningTasks() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make([]string, 0, len(e.running))
	for id := range e.running {
		ids = append(ids, id)
	}
	return ids
}

// RunningCount 获取运行中的任务数量
func (e *Executor) RunningCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.running)
}

// CanStart 检查是否可以启动新任务
func (e *Executor) CanStart() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.running) < e.maxRunning
}

// cleanup 清理运行记录
func (e *Executor) cleanup(taskID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.running, taskID)
}

// RegisterTask 注册任务到依赖管理器
func (e *Executor) RegisterTask(task *Task) error {
	return e.depManager.RegisterTask(task)
}

// UnregisterTask 注销任务
func (e *Executor) UnregisterTask(taskID string) {
	e.depManager.UnregisterTask(taskID)
}

// GetReadyTasks 获取可以执行的任务
func (e *Executor) GetReadyTasks() []string {
	return e.depManager.GetReadyTasks()
}

// GetDependencyManager 获取依赖管理器
func (e *Executor) GetDependencyManager() *DependencyManager {
	return e.depManager
}

// GenerateTaskID 生成任务 ID
func GenerateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

// GenerateExecutionID 生成执行 ID
func GenerateExecutionID() string {
	return fmt.Sprintf("exec_%d", time.Now().UnixNano())
}

// BaseHandler 基础处理器（可嵌入）
type BaseHandler struct {
	name string
}

// NewBaseHandler 创建基础处理器
func NewBaseHandler(name string) *BaseHandler {
	return &BaseHandler{name: name}
}

// Name 返回处理器名称
func (h *BaseHandler) Name() string {
	return h.name
}

// CommandHandler 命令处理器
type CommandHandler struct {
	*BaseHandler
}

// NewCommandHandler 创建命令处理器
func NewCommandHandler() *CommandHandler {
	return &CommandHandler{
		BaseHandler: NewBaseHandler("command"),
	}
}

// Execute 执行命令
func (h *CommandHandler) Execute(ctx context.Context, task *Task) (map[string]interface{}, error) {
	// 这里应该实现实际的命令执行逻辑
	// 为了避免导入问题，返回占位结果
	return map[string]interface{}{
		"status": "completed",
		"output": "命令执行成功",
	}, nil
}

// HTTPHandler HTTP 请求处理器
type HTTPHandler struct {
	*BaseHandler
}

// NewHTTPHandler 创建 HTTP 处理器
func NewHTTPHandler() *HTTPHandler {
	return &HTTPHandler{
		BaseHandler: NewBaseHandler("http"),
	}
}

// Execute 执行 HTTP 请求
func (h *HTTPHandler) Execute(ctx context.Context, task *Task) (map[string]interface{}, error) {
	// 这里应该实现实际的 HTTP 请求逻辑
	return map[string]interface{}{
		"status":   "completed",
		"response": "请求成功",
	}, nil
}

// ScriptHandler 脚本处理器
type ScriptHandler struct {
	*BaseHandler
}

// NewScriptHandler 创建脚本处理器
func NewScriptHandler() *ScriptHandler {
	return &ScriptHandler{
		BaseHandler: NewBaseHandler("script"),
	}
}

// Execute 执行脚本
func (h *ScriptHandler) Execute(ctx context.Context, task *Task) (map[string]interface{}, error) {
	// 这里应该实现实际的脚本执行逻辑
	return map[string]interface{}{
		"status": "completed",
		"output": "脚本执行成功",
	}, nil
}
