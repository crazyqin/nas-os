package dsmaagent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AgentTask 代理任务
type AgentTask struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      TaskStatus             `json:"status"`
	Priority    TaskPriority           `json:"priority"`
	Steps       []*TaskStep            `json:"steps"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TaskStep 任务步骤
type TaskStep struct {
	Name        string                 `json:"name"`
	Status      StepStatus             `json:"status"`
	Action      string                 `json:"action"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskCompleted
	TaskFailed
	TaskCancelled
)

// StepStatus 步骤状态
type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepCompleted
	StepFailed
	StepSkipped
)

// TaskPriority 任务优先级
type TaskPriority int

const (
	PriorityLow TaskPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// AgentAction 代理动作
type AgentAction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Handler     ActionHandler          `json:"-"`
	Parameters  []ActionParameter      `json:"parameters"`
}

// ActionHandler 动作处理器
type ActionHandler func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)

// ActionParameter 动作参数
type ActionParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// AgentMetrics 代理指标
type AgentMetrics struct {
	TotalTasks      int64         `json:"total_tasks"`
	CompletedTasks  int64         `json:"completed_tasks"`
	FailedTasks     int64         `json:"failed_tasks"`
	AverageTaskTime time.Duration `json:"average_task_time"`
	ActiveTasks     int           `json:"active_tasks"`
	RegisteredActions int         `json:"registered_actions"`
}

// DSMAgent 智能代理引擎
type DSMAgent struct {
	mu      sync.RWMutex
	actions map[string]*AgentAction
	tasks   map[string]*AgentTask
	metrics *AgentMetrics
	workers int
	taskCh  chan *AgentTask
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *slog.Logger
}

// NewDSMAgent 创建智能代理引擎
func NewDSMAgent(workers int, logger *slog.Logger) *DSMAgent {
	if workers <= 0 {
		workers = 4
	}
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	agent := &DSMAgent{
		actions: make(map[string]*AgentAction),
		tasks:   make(map[string]*AgentTask),
		metrics: &AgentMetrics{},
		workers: workers,
		taskCh:  make(chan *AgentTask, 100),
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
	}

	// 注册内置动作
	agent.registerBuiltinActions()

	return agent
}

// Start 启动代理引擎
func (a *DSMAgent) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := 0; i < a.workers; i++ {
		go a.worker(i)
	}

	a.logger.Info("DSM Agent已启动", "workers", a.workers)
	return nil
}

// Stop 停止代理引擎
func (a *DSMAgent) Stop() error {
	a.cancel()
	a.logger.Info("DSM Agent已停止")
	return nil
}

// RegisterAction 注册动作
func (a *DSMAgent) RegisterAction(action *AgentAction) error {
	if action == nil {
		return errors.New("action cannot be nil")
	}
	if action.Name == "" {
		return errors.New("action name cannot be empty")
	}
	if action.Handler == nil {
		return errors.New("action handler cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.actions[action.Name]; exists {
		return fmt.Errorf("action %s already exists", action.Name)
	}

	a.actions[action.Name] = action
	a.metrics.RegisteredActions = len(a.actions)

	a.logger.Info("动作已注册", "name", action.Name)
	return nil
}

// SubmitTask 提交任务
func (a *DSMAgent) SubmitTask(task *AgentTask) (string, error) {
	if task == nil {
		return "", errors.New("task cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	task.Status = TaskPending
	task.CreatedAt = time.Now()

	a.tasks[task.ID] = task
	a.metrics.TotalTasks++

	// 发送到工作队列
	select {
	case a.taskCh <- task:
		a.logger.Info("任务已提交", "id", task.ID, "name", task.Name)
	default:
		task.Status = TaskFailed
		task.Error = "task queue full"
		return task.ID, errors.New("task queue is full")
	}

	return task.ID, nil
}

// GetTask 获取任务状态
func (a *DSMAgent) GetTask(taskID string) (*AgentTask, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	task, exists := a.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return task, nil
}

// CancelTask 取消任务
func (a *DSMAgent) CancelTask(taskID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	task, exists := a.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != TaskPending && task.Status != TaskRunning {
		return fmt.Errorf("task %s cannot be cancelled (status: %d)", taskID, task.Status)
	}

	task.Status = TaskCancelled
	now := time.Now()
	task.CompletedAt = &now

	a.logger.Info("任务已取消", "id", taskID)
	return nil
}

// GetMetrics 获取指标
func (a *DSMAgent) GetMetrics() *AgentMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	metrics := *a.metrics
	metrics.ActiveTasks = 0
	for _, task := range a.tasks {
		if task.Status == TaskRunning {
			metrics.ActiveTasks++
		}
	}

	return &metrics
}

// ListActions 列出已注册动作
func (a *DSMAgent) ListActions() []*AgentAction {
	a.mu.RLock()
	defer a.mu.RUnlock()

	actions := make([]*AgentAction, 0, len(a.actions))
	for _, action := range a.actions {
		actions = append(actions, action)
	}

	return actions
}

// ExecuteAction 执行动作
func (a *DSMAgent) ExecuteAction(ctx context.Context, actionName string, params map[string]interface{}) (map[string]interface{}, error) {
	a.mu.RLock()
	action, exists := a.actions[actionName]
	a.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("action %s not found", actionName)
	}

	// 验证必需参数
	for _, param := range action.Parameters {
		if param.Required {
			if _, ok := params[param.Name]; !ok {
				if param.Default != nil {
					params[param.Name] = param.Default
				} else {
					return nil, fmt.Errorf("required parameter %s missing", param.Name)
				}
			}
		}
	}

	start := time.Now()
	result, err := action.Handler(ctx, params)
	duration := time.Since(start)

	if err != nil {
		a.logger.Error("动作执行失败", "action", actionName, "error", err, "duration", duration)
		return nil, err
	}

	a.logger.Info("动作执行成功", "action", actionName, "duration", duration)
	return result, nil
}

// worker 工作协程
func (a *DSMAgent) worker(id int) {
	a.logger.Debug("工作协程已启动", "worker", id)

	for {
		select {
		case <-a.ctx.Done():
			a.logger.Debug("工作协程已停止", "worker", id)
			return
		case task := <-a.taskCh:
			a.executeTask(task)
		}
	}
}

// executeTask 执行任务
func (a *DSMAgent) executeTask(task *AgentTask) {
	a.mu.Lock()
	task.Status = TaskRunning
	now := time.Now()
	task.StartedAt = &now
	a.mu.Unlock()

	a.logger.Info("开始执行任务", "id", task.ID, "name", task.Name)

	result := make(map[string]interface{})

	for _, step := range task.Steps {
		select {
		case <-a.ctx.Done():
			task.Status = TaskCancelled
			return
		default:
		}

		step.Status = StepRunning
		stepNow := time.Now()
		step.StartedAt = &stepNow

		a.logger.Debug("执行步骤", "task", task.ID, "step", step.Name, "action", step.Action)

		stepResult, err := a.ExecuteAction(a.ctx, step.Action, step.Parameters)
		if err != nil {
			step.Status = StepFailed
			step.Error = err.Error()
			stepEnd := time.Now()
			step.CompletedAt = &stepEnd

			task.Status = TaskFailed
			task.Error = err.Error()
			taskEnd := time.Now()
			task.CompletedAt = &taskEnd

			a.mu.Lock()
			a.metrics.FailedTasks++
			a.mu.Unlock()

			a.logger.Error("任务执行失败", "id", task.ID, "step", step.Name, "error", err)
			return
		}

		step.Status = StepCompleted
		step.Result = stepResult
		stepEnd := time.Now()
		step.CompletedAt = &stepEnd

		// 合并结果
		for k, v := range stepResult {
			result[k] = v
		}
	}

	task.Status = TaskCompleted
	task.Result = result
	taskEnd := time.Now()
	task.CompletedAt = &taskEnd

	a.mu.Lock()
	a.metrics.CompletedTasks++
	if a.metrics.CompletedTasks > 0 {
		totalDuration := time.Duration(0)
		completedCount := 0
		for _, t := range a.tasks {
			if t.Status == TaskCompleted && t.StartedAt != nil && t.CompletedAt != nil {
				totalDuration += t.CompletedAt.Sub(*t.StartedAt)
				completedCount++
			}
		}
		if completedCount > 0 {
			a.metrics.AverageTaskTime = totalDuration / time.Duration(completedCount)
		}
	}
	a.mu.Unlock()

	a.logger.Info("任务执行完成", "id", task.ID, "name", task.Name)
}

// registerBuiltinActions 注册内置动作
func (a *DSMAgent) registerBuiltinActions() {
	// 文件操作动作
	a.actions["file.read"] = &AgentAction{
		Name:        "file.read",
		Description: "读取文件内容",
		Handler: func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
			path, _ := params["path"].(string)
			if path == "" {
				return nil, errors.New("path is required")
			}
			return map[string]interface{}{"path": path, "status": "simulated"}, nil
		},
		Parameters: []ActionParameter{
			{Name: "path", Type: "string", Description: "文件路径", Required: true},
		},
	}

	// 存储操作动作
	a.actions["storage.check"] = &AgentAction{
		Name:        "storage.check",
		Description: "检查存储状态",
		Handler: func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"total":     1024 * 1024 * 1024 * 100,
				"used":      1024 * 1024 * 1024 * 45,
				"available": 1024 * 1024 * 1024 * 55,
				"health":    "healthy",
			}, nil
		},
	}

	// 网络操作动作
	a.actions["network.test"] = &AgentAction{
		Name:        "network.test",
		Description: "测试网络连接",
		Handler: func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
			host, _ := params["host"].(string)
			if host == "" {
				host = "8.8.8.8"
			}
			return map[string]interface{}{
				"host":       host,
				"reachable":  true,
				"latency_ms": 15,
			}, nil
		},
		Parameters: []ActionParameter{
			{Name: "host", Type: "string", Description: "目标主机", Required: false, Default: "8.8.8.8"},
		},
	}

	a.metrics.RegisteredActions = len(a.actions)
}
