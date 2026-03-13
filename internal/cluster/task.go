package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// 任务状态
const (
	TaskStatusPending   = "pending"
	TaskStatusScheduled = "scheduled"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
	TaskStatusTimeout   = "timeout"
	TaskStatusRetrying  = "retrying"
)

// 任务优先级
const (
	TaskPriorityLow      = 1
	TaskPriorityNormal   = 5
	TaskPriorityHigh     = 10
	TaskPriorityCritical = 100
)

// 任务类型
const (
	TaskTypeCompute   = "compute"
	TaskTypeInference = "inference"
	TaskTypeData      = "data"
	TaskTypeSync      = "sync"
	TaskTypeBatch     = "batch"
	TaskTypeStream    = "stream"
)

// TaskRequirements 任务需求
type TaskRequirements struct {
	CPU          int               `json:"cpu"`          // 最小 CPU 核心数
	Memory       int64             `json:"memory"`       // 最小内存 (MB)
	Storage      int64             `json:"storage"`      // 最小存储 (GB)
	GPU          bool              `json:"gpu"`          // 是否需要 GPU
	Capabilities uint32            `json:"capabilities"` // 需要的能力位图
	Region       string            `json:"region"`       // 区域限制
	Zone         string            `json:"zone"`         // 可用区限制
	NodeType     string            `json:"node_type"`    // 节点类型限制
	Labels       map[string]string `json:"labels"`       // 标签选择器
}

// TaskConfig 任务配置
type TaskConfig struct {
	Timeout     int      `json:"timeout"`     // 超时时间（秒）
	MaxRetries  int      `json:"max_retries"` // 最大重试次数
	RetryDelay  int      `json:"retry_delay"` // 重试延迟（秒）
	Parallelism int      `json:"parallelism"` // 并行度
	Priority    int      `json:"priority"`    // 优先级
	Tags        []string `json:"tags"`        // 标签
}

// Task 任务定义
type Task struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Payload      json.RawMessage        `json:"payload"`      // 任务数据
	Requirements TaskRequirements       `json:"requirements"` // 资源需求
	Config       TaskConfig             `json:"config"`       // 任务配置
	Status       string                 `json:"status"`
	Priority     int                    `json:"priority"`
	NodeID       string                 `json:"node_id"` // 分配的节点
	Result       *TaskResult            `json:"result,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	ScheduledAt  time.Time              `json:"scheduled_at"`
	StartedAt    time.Time              `json:"started_at"`
	CompletedAt  time.Time              `json:"completed_at"`
	Attempts     int                    `json:"attempts"`
	LastError    string                 `json:"last_error"`
	ParentTaskID string                 `json:"parent_task_id,omitempty"`
	ChildTaskIDs []string               `json:"child_task_ids,omitempty"`
	CallbackURL  string                 `json:"callback_url,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TaskResult 任务结果
type TaskResult struct {
	TaskID    string                 `json:"task_id"`
	NodeID    string                 `json:"node_id"`
	Success   bool                   `json:"success"`
	Data      json.RawMessage        `json:"data"`
	Error     string                 `json:"error,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
	Duration  time.Duration          `json:"duration"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

// TaskSchedule 任务调度配置
type TaskSchedule struct {
	TaskID   string    `json:"task_id"`
	Schedule string    `json:"schedule"` // cron 表达式
	Enabled  bool      `json:"enabled"`
	LastRun  time.Time `json:"last_run"`
	NextRun  time.Time `json:"next_run"`
	RunCount int       `json:"run_count"`
}

// TaskSchedulerConfig 调度器配置
type TaskSchedulerConfig struct {
	DataDir          string `json:"data_dir"`
	MaxConcurrent    int    `json:"max_concurrent"`    // 最大并发任务数
	TaskTimeout      int    `json:"task_timeout"`      // 默认超时（秒）
	RetryAttempts    int    `json:"retry_attempts"`    // 默认重试次数
	ScheduleInterval int    `json:"schedule_interval"` // 调度间隔（秒）
}

// TaskScheduler 任务调度器
type TaskScheduler struct {
	config         TaskSchedulerConfig
	tasks          map[string]*Task
	tasksMutex     sync.RWMutex
	schedules      map[string]*TaskSchedule
	schedulesMutex sync.RWMutex
	pending        chan *Task
	running        map[string]*Task
	runningMutex   sync.RWMutex
	completed      []*TaskResult
	// completedMutex sync.RWMutex - 保留用于未来需要并发控制 completed 的场景
	cron        *cron.Cron
	ctx         context.Context
	cancel      context.CancelFunc
	logger      *zap.Logger
	edgeManager *EdgeNodeManager
	resultAgg   *ResultAggregator
	callbacks   TaskCallbacks
}

// TaskCallbacks 任务回调
type TaskCallbacks struct {
	OnTaskCreated   func(task *Task)
	OnTaskScheduled func(task *Task, nodeID string)
	OnTaskStarted   func(task *Task)
	OnTaskCompleted func(task *Task, result *TaskResult)
	OnTaskFailed    func(task *Task, err error)
}

// NewTaskScheduler 创建任务调度器
func NewTaskScheduler(config TaskSchedulerConfig, logger *zap.Logger) (*TaskScheduler, error) {
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/edge/tasks"
	}
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = 100
	}
	if config.TaskTimeout == 0 {
		config.TaskTimeout = 300 // 5 分钟
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.ScheduleInterval == 0 {
		config.ScheduleInterval = 5
	}

	ctx, cancel := context.WithCancel(context.Background())

	ts := &TaskScheduler{
		config:    config,
		tasks:     make(map[string]*Task),
		schedules: make(map[string]*TaskSchedule),
		pending:   make(chan *Task, 1000),
		running:   make(map[string]*Task),
		completed: make([]*TaskResult, 0),
		cron:      cron.New(cron.WithSeconds()),
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
	}

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("创建任务数据目录失败：%w", err)
	}

	// 加载持久化任务
	if err := ts.loadTasks(); err != nil {
		logger.Warn("加载任务失败", zap.Error(err))
	}

	return ts, nil
}

// Initialize 初始化任务调度器
func (ts *TaskScheduler) Initialize() error {
	ts.logger.Info("初始化任务调度器")

	// 启动 cron
	ts.cron.Start()

	// 启动调度工作线程
	for i := 0; i < ts.config.MaxConcurrent; i++ {
		go ts.worker(i)
	}

	// 启动定时任务调度
	go ts.scheduleWorker()

	// 恢复未完成的任务
	ts.recoverTasks()

	ts.logger.Info("任务调度器初始化完成")
	return nil
}

// SetEdgeManager 设置边缘节点管理器
func (ts *TaskScheduler) SetEdgeManager(manager *EdgeNodeManager) {
	ts.edgeManager = manager
}

// SetResultAggregator 设置结果聚合器
func (ts *TaskScheduler) SetResultAggregator(agg *ResultAggregator) {
	ts.resultAgg = agg
}

// SetCallbacks 设置回调
func (ts *TaskScheduler) SetCallbacks(callbacks TaskCallbacks) {
	ts.callbacks = callbacks
}

// CreateTask 创建任务
func (ts *TaskScheduler) CreateTask(task *Task) error {
	ts.tasksMutex.Lock()
	defer ts.tasksMutex.Unlock()

	if task.ID == "" {
		task.ID = generateTaskID()
	}
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	if task.Priority == 0 {
		task.Priority = TaskPriorityNormal
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.Config.Timeout == 0 {
		task.Config.Timeout = ts.config.TaskTimeout
	}
	if task.Config.MaxRetries == 0 {
		task.Config.MaxRetries = ts.config.RetryAttempts
	}

	ts.tasks[task.ID] = task

	ts.logger.Info("创建任务",
		zap.String("task_id", task.ID),
		zap.String("type", task.Type),
		zap.Int("priority", task.Priority))

	// 加入待调度队列（在锁外执行，避免在 channel 阻塞时持有锁）
	ts.tasksMutex.Unlock()
	ts.pending <- task
	ts.tasksMutex.Lock()

	// 触发回调
	if ts.callbacks.OnTaskCreated != nil {
		go ts.callbacks.OnTaskCreated(task)
	}

	// 持久化（使用不加锁版本）
	return ts.saveTasksLocked()
}

// GetTask 获取任务
func (ts *TaskScheduler) GetTask(taskID string) (*Task, bool) {
	ts.tasksMutex.RLock()
	defer ts.tasksMutex.RUnlock()

	task, exists := ts.tasks[taskID]
	return task, exists
}

// GetTasks 获取所有任务
func (ts *TaskScheduler) GetTasks() []*Task {
	ts.tasksMutex.RLock()
	defer ts.tasksMutex.RUnlock()

	tasks := make([]*Task, 0, len(ts.tasks))
	for _, task := range ts.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetTasksByStatus 按状态获取任务
func (ts *TaskScheduler) GetTasksByStatus(status string) []*Task {
	ts.tasksMutex.RLock()
	defer ts.tasksMutex.RUnlock()

	tasks := make([]*Task, 0)
	for _, task := range ts.tasks {
		if task.Status == status {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GetTasksByNode 获取节点上的任务
func (ts *TaskScheduler) GetTasksByNode(nodeID string) []*Task {
	ts.tasksMutex.RLock()
	defer ts.tasksMutex.RUnlock()

	tasks := make([]*Task, 0)
	for _, task := range ts.tasks {
		if task.NodeID == nodeID {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// CancelTask 取消任务
func (ts *TaskScheduler) CancelTask(taskID string) error {
	ts.tasksMutex.Lock()
	defer ts.tasksMutex.Unlock()

	task, exists := ts.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在：%s", taskID)
	}

	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
		return fmt.Errorf("任务已完成，无法取消")
	}

	task.Status = TaskStatusCancelled
	ts.logger.Info("取消任务", zap.String("task_id", taskID))

	return ts.saveTasksLocked()
}

// RetryTask 重试任务
func (ts *TaskScheduler) RetryTask(taskID string) error {
	ts.tasksMutex.Lock()
	defer ts.tasksMutex.Unlock()

	task, exists := ts.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在：%s", taskID)
	}

	if task.Status != TaskStatusFailed && task.Status != TaskStatusTimeout {
		return fmt.Errorf("只有失败或超时的任务可以重试")
	}

	task.Status = TaskStatusPending
	task.Attempts = 0
	task.LastError = ""
	task.NodeID = ""

	ts.pending <- task
	ts.logger.Info("重试任务", zap.String("task_id", taskID))

	return ts.saveTasksLocked()
}

// CreateScheduledTask 创建定时任务
func (ts *TaskScheduler) CreateScheduledTask(task *Task, schedule string) error {
	if err := ts.CreateTask(task); err != nil {
		return err
	}

	ts.schedulesMutex.Lock()
	defer ts.schedulesMutex.Unlock()

	ts.schedules[task.ID] = &TaskSchedule{
		TaskID:   task.ID,
		Schedule: schedule,
		Enabled:  true,
	}

	// 添加 cron 任务
	cronSpec := schedule
	ts.cron.AddFunc(cronSpec, func() {
		ts.triggerScheduledTask(task.ID)
	})

	ts.logger.Info("创建定时任务",
		zap.String("task_id", task.ID),
		zap.String("schedule", schedule))

	return ts.saveSchedules()
}

// triggerScheduledTask 触发定时任务
func (ts *TaskScheduler) triggerScheduledTask(taskID string) {
	ts.schedulesMutex.Lock()
	if sched, exists := ts.schedules[taskID]; exists {
		sched.LastRun = time.Now()
		sched.RunCount++
	}
	ts.schedulesMutex.Unlock()

	// 获取任务模板并创建新实例
	task, exists := ts.GetTask(taskID)
	if !exists {
		return
	}

	newTask := &Task{
		Name:         task.Name,
		Type:         task.Type,
		Payload:      task.Payload,
		Requirements: task.Requirements,
		Config:       task.Config,
		Priority:     task.Priority,
		ParentTaskID: taskID,
	}

	ts.CreateTask(newTask)
}

// worker 任务处理工作线程
func (ts *TaskScheduler) worker(id int) {
	for {
		select {
		case <-ts.ctx.Done():
			return
		case task := <-ts.pending:
			ts.processTask(task)
		}
	}
}

// processTask 处理任务
func (ts *TaskScheduler) processTask(task *Task) {
	// 选择节点
	if ts.edgeManager == nil {
		ts.logger.Error("边缘节点管理器未设置")
		ts.markTaskFailed(task, fmt.Errorf("边缘节点管理器未设置"))
		return
	}

	node, err := ts.edgeManager.SelectBestNode(task.Requirements)
	if err != nil {
		ts.logger.Warn("选择节点失败", zap.String("task_id", task.ID), zap.Error(err))
		// 重新入队等待
		time.Sleep(time.Second * time.Duration(ts.config.ScheduleInterval))
		if task.Status == TaskStatusPending {
			ts.pending <- task
		}
		return
	}

	// 调度任务到节点
	ts.scheduleTaskToNode(task, node.ID)
}

// scheduleTaskToNode 调度任务到节点
func (ts *TaskScheduler) scheduleTaskToNode(task *Task, nodeID string) {
	ts.tasksMutex.Lock()
	task.Status = TaskStatusScheduled
	task.NodeID = nodeID
	task.ScheduledAt = time.Now()
	ts.tasksMutex.Unlock()

	ts.logger.Info("调度任务到节点",
		zap.String("task_id", task.ID),
		zap.String("node_id", nodeID))

	// 触发回调
	if ts.callbacks.OnTaskScheduled != nil {
		go ts.callbacks.OnTaskScheduled(task, nodeID)
	}

	// 更新节点任务计数
	if ts.edgeManager != nil {
		_, exists := ts.edgeManager.GetNode(nodeID)
		if exists {
			ts.edgeManager.UpdateNodeStatus(nodeID, EdgeNodeStatusBusy)
		}
	}

	// 执行任务（实际实现中会发送到边缘节点）
	go ts.executeTask(task)
}

// executeTask 执行任务
func (ts *TaskScheduler) executeTask(task *Task) {
	ts.runningMutex.Lock()
	ts.running[task.ID] = task
	ts.runningMutex.Unlock()

	ts.tasksMutex.Lock()
	task.Status = TaskStatusRunning
	task.StartedAt = time.Now()
	task.Attempts++
	ts.tasksMutex.Unlock()

	ts.logger.Info("开始执行任务",
		zap.String("task_id", task.ID),
		zap.Int("attempt", task.Attempts))

	// 触发回调
	if ts.callbacks.OnTaskStarted != nil {
		go ts.callbacks.OnTaskStarted(task)
	}

	// 设置超时
	timeout := time.Duration(task.Config.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(ts.ctx, timeout)
	defer cancel()

	// 模拟任务执行（实际实现中会调用边缘节点 API）
	result := ts.simulateTaskExecution(ctx, task)

	// 处理结果
	ts.runningMutex.Lock()
	delete(ts.running, task.ID)
	ts.runningMutex.Unlock()

	if result.Success {
		ts.markTaskCompleted(task, result)
	} else {
		ts.handleTaskFailure(task, result)
	}
}

// simulateTaskExecution 模拟任务执行
func (ts *TaskScheduler) simulateTaskExecution(ctx context.Context, task *Task) *TaskResult {
	// 实际实现中这里会发送 HTTP/gRPC 请求到边缘节点
	// 这里简化为模拟执行

	select {
	case <-ctx.Done():
		return &TaskResult{
			TaskID:    task.ID,
			NodeID:    task.NodeID,
			Success:   false,
			Error:     "任务超时",
			StartTime: task.StartedAt,
			EndTime:   time.Now(),
		}
	case <-time.After(2 * time.Second):
		// 模拟任务完成
		result := &TaskResult{
			TaskID:    task.ID,
			NodeID:    task.NodeID,
			Success:   true,
			StartTime: task.StartedAt,
			EndTime:   time.Now(),
			Duration:  time.Since(task.StartedAt),
		}

		// 添加到结果聚合器
		if ts.resultAgg != nil {
			ts.resultAgg.SubmitResult(result)
		}

		return result
	}
}

// markTaskCompleted 标记任务完成
func (ts *TaskScheduler) markTaskCompleted(task *Task, result *TaskResult) {
	ts.tasksMutex.Lock()
	defer ts.tasksMutex.Unlock()

	task.Status = TaskStatusCompleted
	task.Result = result
	task.CompletedAt = time.Now()

	ts.logger.Info("任务完成",
		zap.String("task_id", task.ID),
		zap.Duration("duration", result.Duration))

	// 触发回调
	if ts.callbacks.OnTaskCompleted != nil {
		go ts.callbacks.OnTaskCompleted(task, result)
	}

	// 更新节点状态
	if ts.edgeManager != nil && task.NodeID != "" {
		node, exists := ts.edgeManager.GetNode(task.NodeID)
		if exists {
			node.TasksRunning--
		}
	}

	_ = ts.saveTasks()
}

// handleTaskFailure 处理任务失败
func (ts *TaskScheduler) handleTaskFailure(task *Task, result *TaskResult) {
	ts.tasksMutex.Lock()
	defer ts.tasksMutex.Unlock()

	task.LastError = result.Error

	// 检查是否需要重试
	if task.Attempts < task.Config.MaxRetries {
		task.Status = TaskStatusRetrying
		ts.logger.Warn("任务失败，准备重试",
			zap.String("task_id", task.ID),
			zap.Int("attempt", task.Attempts),
			zap.Int("max_retries", task.Config.MaxRetries),
			zap.String("error", result.Error))

		// 延迟重试
		go func() {
			time.Sleep(time.Duration(task.Config.RetryDelay) * time.Second)
			task.Status = TaskStatusPending
			task.NodeID = ""
			ts.pending <- task
		}()

		return
	}

	// 最终失败
	task.Status = TaskStatusFailed
	task.Result = result
	task.CompletedAt = time.Now()

	ts.logger.Error("任务最终失败",
		zap.String("task_id", task.ID),
		zap.String("error", result.Error))

	// 触发回调
	if ts.callbacks.OnTaskFailed != nil {
		go ts.callbacks.OnTaskFailed(task, fmt.Errorf("%s", result.Error))
	}

	_ = ts.saveTasks()
}

// markTaskFailed 标记任务失败
func (ts *TaskScheduler) markTaskFailed(task *Task, err error) {
	ts.tasksMutex.Lock()
	defer ts.tasksMutex.Unlock()

	task.Status = TaskStatusFailed
	task.LastError = err.Error()
	task.CompletedAt = time.Now()

	if ts.callbacks.OnTaskFailed != nil {
		go ts.callbacks.OnTaskFailed(task, err)
	}
}

// scheduleWorker 定时任务调度工作线程
func (ts *TaskScheduler) scheduleWorker() {
	ticker := time.NewTicker(time.Duration(ts.config.ScheduleInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ts.ctx.Done():
			return
		case <-ticker.C:
			ts.checkSchedules()
		}
	}
}

// checkSchedules 检查定时任务
func (ts *TaskScheduler) checkSchedules() {
	ts.schedulesMutex.RLock()
	defer ts.schedulesMutex.RUnlock()

	now := time.Now()
	for _, sched := range ts.schedules {
		if !sched.Enabled {
			continue
		}

		// 计算下次运行时间
		// 简化处理，实际应该解析 cron 表达式
		_ = now
	}
}

// recoverTasks 恢复未完成的任务
func (ts *TaskScheduler) recoverTasks() {
	ts.tasksMutex.RLock()
	defer ts.tasksMutex.RUnlock()

	for _, task := range ts.tasks {
		if task.Status == TaskStatusPending || task.Status == TaskStatusScheduled {
			ts.pending <- task
			ts.logger.Info("恢复任务", zap.String("task_id", task.ID))
		}
	}
}

// GetStats 获取调度统计
func (ts *TaskScheduler) GetStats() map[string]interface{} {
	ts.tasksMutex.RLock()
	ts.runningMutex.RLock()
	defer ts.tasksMutex.RUnlock()
	ts.runningMutex.RUnlock()

	stats := map[string]interface{}{
		"total_tasks":   len(ts.tasks),
		"pending_tasks": len(ts.pending),
		"running_tasks": len(ts.running),
		"by_status":     make(map[string]int),
	}

	byStatus := stats["by_status"].(map[string]int)
	for _, task := range ts.tasks {
		byStatus[task.Status]++
	}

	return stats
}

// Shutdown 关闭任务调度器
func (ts *TaskScheduler) Shutdown() error {
	ts.cancel()
	ts.cron.Stop()
	_ = ts.saveTasks()
	_ = ts.saveSchedules()
	ts.logger.Info("任务调度器已关闭")
	return nil
}

// 持久化

// saveTasksLocked 保存任务（调用者需持有锁）
func (ts *TaskScheduler) saveTasksLocked() error {
	tasksFile := filepath.Join(ts.config.DataDir, "tasks.json")

	data, err := json.MarshalIndent(ts.tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(tasksFile, data, 0644)
}

func (ts *TaskScheduler) saveTasks() error {
	ts.tasksMutex.RLock()
	defer ts.tasksMutex.RUnlock()
	return ts.saveTasksLocked()
}

func (ts *TaskScheduler) loadTasks() error {
	tasksFile := filepath.Join(ts.config.DataDir, "tasks.json")

	data, err := os.ReadFile(tasksFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &ts.tasks)
}

func (ts *TaskScheduler) saveSchedules() error {
	ts.schedulesMutex.RLock()
	defer ts.schedulesMutex.RUnlock()

	schedulesFile := filepath.Join(ts.config.DataDir, "schedules.json")

	data, err := json.MarshalIndent(ts.schedules, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(schedulesFile, data, 0644)
}

// 辅助函数

func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}
