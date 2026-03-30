package gpu

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TaskPriority 任务优先级
type TaskPriority int

const (
	PriorityCritical TaskPriority = 100 // 关键任务
	PriorityHigh     TaskPriority = 75  // 高优先级
	PriorityNormal   TaskPriority = 50  // 正常优先级
	PriorityLow      TaskPriority = 25  // 低优先级
	PriorityBackground TaskPriority = 10 // 后台任务
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// GPUTask GPU任务
type GPUTask struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Priority     TaskPriority      `json:"priority"`
	Status       TaskStatus        `json:"status"`
	
	// 资源需求
	MemorySize   uint64            `json:"memory_size"`   // 显存需求(bytes)
	Duration     time.Duration     `json:"duration"`      // 预估执行时间
	Requirements *GPURequirements  `json:"requirements"`
	
	// 执行信息
	GPUID        string            `json:"gpu_id,omitempty"`
	AllocatedAt  *time.Time        `json:"allocated_at,omitempty"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	
	// 元数据
	SubmittedAt  time.Time         `json:"submitted_at"`
	UserID       string            `json:"user_id,omitempty"`
	Group        string            `json:"group,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	
	// 回调
	OnStart      func(ctx context.Context, gpu *GPUDevice) error
	OnComplete   func(ctx context.Context, gpu *GPUDevice) error
	OnFail       func(ctx context.Context, gpu *GPUDevice, err error)
	
	// 内部状态
	allocation   *GPUAllocation
	retryCount   int
	maxRetries   int
	index        int // 优先级队列索引
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	// 队列配置
	MaxQueueSize       int           `json:"max_queue_size"`
	MaxConcurrentTasks int           `json:"max_concurrent_tasks"`
	
	// 调度策略
	SchedulingPolicy   SchedulingPolicy `json:"scheduling_policy"`
	
	// 超时配置
	TaskTimeout        time.Duration `json:"task_timeout"`
	QueueTimeout       time.Duration `json:"queue_timeout"`
	
	// 重试配置
	MaxRetries         int           `json:"max_retries"`
	RetryDelay         time.Duration `json:"retry_delay"`
	
	// 亲和性配置
	EnableNUMA         bool          `json:"enable_numa"`
	EnableAffinity     bool          `json:"enable_affinity"`
	
	// 日志
	Logger             *zap.Logger   `json:"-"`
}

// SchedulingPolicy 调度策略
type SchedulingPolicy string

const (
	PolicyFIFO        SchedulingPolicy = "fifo"        // 先进先出
	PriorityBased     SchedulingPolicy = "priority"    // 优先级调度
	FairShare         SchedulingPolicy = "fair_share"  // 公平共享
	ResourceAware     SchedulingPolicy = "resource"    // 资源感知
	Hybrid            SchedulingPolicy = "hybrid"      // 混合策略
)

// DefaultSchedulerConfig 默认配置
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		MaxQueueSize:       100,
		MaxConcurrentTasks: 4,
		SchedulingPolicy:   PriorityBased,
		TaskTimeout:        30 * time.Minute,
		QueueTimeout:       1 * time.Hour,
		MaxRetries:         3,
		RetryDelay:         5 * time.Second,
		EnableNUMA:         true,
		EnableAffinity:     true,
		Logger:             zap.NewNop(),
	}
}

// GPUScheduler GPU调度器
// 实现GPU任务优先级队列和显存动态分配
type GPUScheduler struct {
	// GPU管理器
	manager *GPUManager
	
	// 任务队列
	priorityQueue *TaskPriorityQueue
	waitQueue     []*GPUTask // 等待资源的任务
	
	// 运行中的任务
	runningTasks map[string]*GPUTask
	allocations  map[string]*GPUAllocation
	
	// 配置
	config *SchedulerConfig
	
	// 调度控制
	stopChan     chan struct{}
	running      bool
	dispatcherWg sync.WaitGroup
	
	// 事件通道
	taskComplete chan string
	
	// 统计
	stats *SchedulerStats
	
	mu sync.Mutex
	
	logger *zap.Logger
}

// SchedulerStats 调度统计
type SchedulerStats struct {
	TotalSubmitted   int64 `json:"total_submitted"`
	TotalCompleted   int64 `json:"total_completed"`
	TotalFailed      int64 `json:"total_failed"`
	QueueLength      int   `json:"queue_length"`
	RunningTasks     int   `json:"running_tasks"`
	WaitQueueLength  int   `json:"wait_queue_length"`
	AvgWaitTime      int64 `json:"avg_wait_time_ms"`
	AvgExecutionTime int64 `json:"avg_execution_time_ms"`
}

// NewGPUScheduler 创建调度器
func NewGPUScheduler(manager *GPUManager, config *SchedulerConfig) (*GPUScheduler, error) {
	if config == nil {
		config = DefaultSchedulerConfig()
	}
	if config.Logger == nil {
		config.Logger = zap.NewNop()
	}
	
	s := &GPUScheduler{
		manager:       manager,
		priorityQueue: NewTaskPriorityQueue(),
		waitQueue:     make([]*GPUTask, 0),
		runningTasks:  make(map[string]*GPUTask),
		allocations:   make(map[string]*GPUAllocation),
		config:        config,
		stopChan:      make(chan struct{}),
		taskComplete:  make(chan string, 100),
		stats:         &SchedulerStats{},
		logger:        config.Logger,
	}
	
	// 注册GPU事件处理器
	manager.RegisterEventHandler(s)
	
	return s, nil
}

// Start 启动调度器
func (s *GPUScheduler) Start() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	
	// 启动调度循环
	s.dispatcherWg.Add(1)
	go s.dispatchLoop()
	
	// 启动任务执行监控
	s.dispatcherWg.Add(1)
	go s.taskMonitor()
	
	s.logger.Info("GPU调度器启动")
}

// Stop 停止调度器
func (s *GPUScheduler) Stop() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	
	close(s.stopChan)
	s.dispatcherWg.Wait()
	
	s.logger.Info("GPU调度器停止")
}

// dispatchLoop 调度循环
func (s *GPUScheduler) dispatchLoop() {
	defer s.dispatcherWg.Done()
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.dispatch()
		case taskID := <-s.taskComplete:
			s.handleTaskComplete(taskID)
		}
	}
}

// dispatch 尝试调度任务
func (s *GPUScheduler) dispatch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 检查是否有可用GPU和待调度任务
	if s.priorityQueue.Len() == 0 && len(s.waitQueue) == 0 {
		return
	}
	
	// 检查并发限制
	if len(s.runningTasks) >= s.config.MaxConcurrentTasks {
		return
	}
	
	// 尝试从优先级队列调度
	for s.priorityQueue.Len() > 0 && len(s.runningTasks) < s.config.MaxConcurrentTasks {
		task := heap.Pop(s.priorityQueue).(*GPUTask)
		
		// 尝试分配GPU
		alloc, err := s.tryAllocate(task)
		if err != nil {
			// 无法分配，放入等待队列
			s.waitQueue = append(s.waitQueue, task)
			task.Status = TaskStatusQueued
			s.logger.Debug("任务进入等待队列",
				zap.String("task_id", task.ID),
				zap.Int("priority", int(task.Priority)),
				zap.Error(err))
			continue
		}
		
		// 分配成功，启动任务
		s.startTask(task, alloc)
	}
	
	// 尝试从等待队列调度（按优先级）
	s.sortWaitQueue()
	for i := 0; i < len(s.waitQueue) && len(s.runningTasks) < s.config.MaxConcurrentTasks; i++ {
		task := s.waitQueue[i]
		
		alloc, err := s.tryAllocate(task)
		if err != nil {
			continue
		}
		
		// 移除等待队列
		s.waitQueue = append(s.waitQueue[:i], s.waitQueue[i+1:]...)
		i-- // 调整索引
		
		s.startTask(task, alloc)
	}
}

// tryAllocate 尝试分配GPU
func (s *GPUScheduler) tryAllocate(task *GPUTask) (*GPUAllocation, error) {
	req := &GPUAllocationRequest{
		TaskID:     task.ID,
		MemorySize: task.MemorySize,
		Priority:   int(task.Priority),
		Requirements: task.Requirements,
	}
	
	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	alloc, err := s.manager.AllocateGPU(ctx, req)
	if err != nil {
		return nil, err
	}
	
	return alloc, nil
}

// startTask 启动任务
func (s *GPUScheduler) startTask(task *GPUTask, alloc *GPUAllocation) {
	task.Status = TaskStatusRunning
	task.GPUID = alloc.GPUID
	task.allocation = alloc
	now := time.Now()
	task.AllocatedAt = &now
	
	s.runningTasks[task.ID] = task
	s.allocations[task.ID] = alloc
	
	s.logger.Info("任务启动",
		zap.String("task_id", task.ID),
		zap.String("gpu_id", alloc.GPUID),
		zap.Uint64("memory", alloc.MemorySize))
	
	// 启动任务执行
	go s.executeTask(task)
}

// executeTask 执行任务
func (s *GPUScheduler) executeTask(task *GPUTask) {
	ctx, cancel := context.WithTimeout(context.Background(), s.config.TaskTimeout)
	defer cancel()
	
	// 获取GPU信息
	gpu := s.manager.GetGPU(task.GPUID)
	if gpu == nil {
		s.handleTaskError(task, fmt.Errorf("GPU不存在: %s", task.GPUID))
		return
	}
	
	// 记录开始时间
	startTime := time.Now()
	task.StartedAt = &startTime
	
	// 执行任务回调
	var err error
	if task.OnStart != nil {
		err = task.OnStart(ctx, gpu)
	}
	
	if err != nil {
		s.handleTaskError(task, err)
		return
	}
	
	// 模拟任务执行（实际应用中这里是真实任务）
	// 这里简化为等待Duration或直到超时
	if task.Duration > 0 {
		select {
		case <-ctx.Done():
			s.handleTaskError(task, ctx.Err())
			return
		case <-time.After(task.Duration):
			// 任务完成
		}
	} else {
		// 无预设时长，等待外部完成信号
		select {
		case <-ctx.Done():
			s.handleTaskError(task, ctx.Err())
			return
		}
	}
	
	// 执行完成回调
	if task.OnComplete != nil {
		if err := task.OnComplete(ctx, gpu); err != nil {
			s.handleTaskError(task, err)
			return
		}
	}
	
	// 记录完成时间
	completedTime := time.Now()
	task.CompletedAt = &completedTime
	task.Status = TaskStatusCompleted
	
	// 发送完成信号
	s.taskComplete <- task.ID
}

// handleTaskError 处理任务错误
func (s *GPUScheduler) handleTaskError(task *GPUTask, err error) {
	s.logger.Error("任务执行失败",
		zap.String("task_id", task.ID),
		zap.Error(err))
	
	// 执行失败回调
	gpu := s.manager.GetGPU(task.GPUID)
	if task.OnFail != nil && gpu != nil {
		task.OnFail(context.Background(), gpu, err)
	}
	
	// 重试处理
	task.retryCount++
	if task.retryCount < task.maxRetries && task.retryCount < s.config.MaxRetries {
		s.logger.Info("任务将重试",
			zap.String("task_id", task.ID),
			zap.Int("retry", task.retryCount))
		
		// 释放GPU
		if task.allocation != nil {
			s.manager.ReleaseGPUByTaskID(task.ID)
		}
		
		// 重新入队
		s.mu.Lock()
		task.Status = TaskStatusPending
		task.GPUID = ""
		task.allocation = nil
		task.AllocatedAt = nil
		delete(s.runningTasks, task.ID)
		delete(s.allocations, task.ID)
		heap.Push(s.priorityQueue, task)
		s.mu.Unlock()
		
		return
	}
	
	// 标记为失败
	task.Status = TaskStatusFailed
	s.taskComplete <- task.ID
}

// handleTaskComplete 处理任务完成
func (s *GPUScheduler) handleTaskComplete(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	task, exists := s.runningTasks[taskID]
	if !exists {
		return
	}
	
	// 释放GPU资源
	if task.allocation != nil {
		s.manager.ReleaseGPUByTaskID(task.ID)
	}
	
	// 更新状态
	delete(s.runningTasks, taskID)
	delete(s.allocations, taskID)
	
	// 更新统计
	if task.Status == TaskStatusCompleted {
		s.stats.TotalCompleted++
		if task.StartedAt != nil && task.CompletedAt != nil {
			execTime := task.CompletedAt.Sub(*task.StartedAt).Milliseconds()
			s.stats.AvgExecutionTime = (s.stats.AvgExecutionTime + execTime) / 2
		}
	} else {
		s.stats.TotalFailed++
	}
	
	s.stats.RunningTasks = len(s.runningTasks)
	
	s.logger.Info("任务完成",
		zap.String("task_id", taskID),
		zap.String("status", string(task.Status)))
}

// taskMonitor 任务监控
func (s *GPUScheduler) taskMonitor() {
	defer s.dispatcherWg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.checkTaskHealth()
		}
	}
}

// checkTaskHealth 检查任务健康状态
func (s *GPUScheduler) checkTaskHealth() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	
	for _, task := range s.runningTasks {
		// 检查超时
		if task.allocation != nil && task.allocation.ExpiresAt != nil {
			if now.After(*task.allocation.ExpiresAt) {
				s.logger.Warn("任务超时",
					zap.String("task_id", task.ID))
				task.Status = TaskStatusFailed
				s.taskComplete <- task.ID
			}
		}
	}
}

// sortWaitQueue 按优先级排序等待队列
func (s *GPUScheduler) sortWaitQueue() {
	// 简单排序（高优先级在前）
	for i := 0; i < len(s.waitQueue)-1; i++ {
		for j := i + 1; j < len(s.waitQueue); j++ {
			if s.waitQueue[j].Priority > s.waitQueue[i].Priority {
				s.waitQueue[i], s.waitQueue[j] = s.waitQueue[j], s.waitQueue[i]
			}
		}
	}
}

// ========== 任务提交 ==========

// SubmitTask 提交任务
func (s *GPUScheduler) SubmitTask(task *GPUTask) error {
	if task.ID == "" {
		task.ID = GenerateTaskID()
	}
	
	// 设置默认值
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	if task.maxRetries == 0 {
		task.maxRetries = s.config.MaxRetries
	}
	
	task.SubmittedAt = time.Now()
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 检查队列限制
	if s.priorityQueue.Len() + len(s.waitQueue) >= s.config.MaxQueueSize {
		return ErrQueueFull
	}
	
	// 入队
	heap.Push(s.priorityQueue, task)
	s.stats.TotalSubmitted++
	s.stats.QueueLength = s.priorityQueue.Len()
	
	s.logger.Info("任务提交",
		zap.String("task_id", task.ID),
		zap.Int("priority", int(task.Priority)),
		zap.Uint64("memory", task.MemorySize))
	
	return nil
}

// CancelTask 取消任务
func (s *GPUScheduler) CancelTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 检查运行中的任务
	if task, exists := s.runningTasks[taskID]; exists {
		task.Status = TaskStatusCancelled
		s.taskComplete <- taskID
		return nil
	}
	
	// 检查等待队列
	for i, task := range s.waitQueue {
		if task.ID == taskID {
			s.waitQueue = append(s.waitQueue[:i], s.waitQueue[i+1:]...)
			task.Status = TaskStatusCancelled
			return nil
		}
	}
	
	// 检查优先级队列
	for i, task := range s.priorityQueue.tasks {
		if task.ID == taskID {
			heap.Remove(s.priorityQueue, i)
			task.Status = TaskStatusCancelled
			return nil
		}
	}
	
	return ErrTaskNotFound
}

// GetTask 获取任务状态
func (s *GPUScheduler) GetTask(taskID string) (*GPUTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 检查运行中的任务
	if task, exists := s.runningTasks[taskID]; exists {
		return task, nil
	}
	
	// 检查等待队列
	for _, task := range s.waitQueue {
		if task.ID == taskID {
			return task, nil
		}
	}
	
	// 检查优先级队列
	for _, task := range s.priorityQueue.tasks {
		if task.ID == taskID {
			return task, nil
		}
	}
	
	return nil, ErrTaskNotFound
}

// ListTasks 列出任务
func (s *GPUScheduler) ListTasks(status TaskStatus) []*GPUTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	var result []*GPUTask
	
	// 运行中的任务
	for _, task := range s.runningTasks {
		if status == "" || task.Status == status {
			result = append(result, task)
		}
	}
	
	// 等待队列
	for _, task := range s.waitQueue {
		if status == "" || task.Status == status {
			result = append(result, task)
		}
	}
	
	// 优先级队列
	for _, task := range s.priorityQueue.tasks {
		if status == "" || task.Status == status {
			result = append(result, task)
		}
	}
	
	return result
}

// GetStats 获取统计
func (s *GPUScheduler) GetStats() *SchedulerStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.stats.QueueLength = s.priorityQueue.Len()
	s.stats.RunningTasks = len(s.runningTasks)
	s.stats.WaitQueueLength = len(s.waitQueue)
	
	return s.stats
}

// ========== GPUEventHandler实现 ==========

func (s *GPUScheduler) OnGPUAdded(gpu *GPUDevice) {
	s.logger.Info("GPU添加", zap.String("gpu_id", gpu.ID))
}

func (s *GPUScheduler) OnGPURemoved(gpu *GPUDevice) {
	s.logger.Info("GPU移除", zap.String("gpu_id", gpu.ID))
	
	// 检查受影响的任务
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, task := range s.runningTasks {
		if task.GPUID == gpu.ID {
			task.Status = TaskStatusFailed
			s.taskComplete <- task.ID
		}
	}
}

func (s *GPUScheduler) OnGPUStatusChange(gpu *GPUDevice, oldStatus, newStatus GPUStatus) {
	s.logger.Info("GPU状态变更",
		zap.String("gpu_id", gpu.ID),
		zap.String("old", string(oldStatus)),
		zap.String("new", string(newStatus)))
}

func (s *GPUScheduler) OnGPUAllocated(gpu *GPUDevice, taskID string, memory uint64) {
	s.logger.Debug("GPU分配",
		zap.String("gpu_id", gpu.ID),
		zap.String("task_id", taskID),
		zap.Uint64("memory", memory))
}

func (s *GPUScheduler) OnGPUReleased(gpu *GPUDevice, taskID string, memory uint64) {
	s.logger.Debug("GPU释放",
		zap.String("gpu_id", gpu.ID),
		zap.String("task_id", taskID),
		zap.Uint64("memory", memory))
}

// ========== 优先级队列实现 ==========

// TaskPriorityQueue 任务优先级队列
type TaskPriorityQueue struct {
	tasks []*GPUTask
}

// NewTaskPriorityQueue 创建优先级队列
func NewTaskPriorityQueue() *TaskPriorityQueue {
	return &TaskPriorityQueue{
		tasks: make([]*GPUTask, 0),
	}
}

// Len 实现heap.Interface
func (pq *TaskPriorityQueue) Len() int {
	return len(pq.tasks)
}

// Less 比较优先级（高优先级在前）
func (pq *TaskPriorityQueue) Less(i, j int) bool {
	// 优先级高的排前面
	if pq.tasks[i].Priority != pq.tasks[j].Priority {
		return pq.tasks[i].Priority > pq.tasks[j].Priority
	}
	
	// 优先级相同，提交时间早的排前面
	return pq.tasks[i].SubmittedAt.Before(pq.tasks[j].SubmittedAt)
}

// Swap 交换
func (pq *TaskPriorityQueue) Swap(i, j int) {
	pq.tasks[i], pq.tasks[j] = pq.tasks[j], pq.tasks[i]
	pq.tasks[i].index = i
	pq.tasks[j].index = j
}

// Push 入队
func (pq *TaskPriorityQueue) Push(x interface{}) {
	task := x.(*GPUTask)
	task.index = len(pq.tasks)
	pq.tasks = append(pq.tasks, task)
}

// Pop 出队
func (pq *TaskPriorityQueue) Pop() interface{} {
	old := pq.tasks
	n := len(old)
	task := old[n-1]
	task.index = -1
	pq.tasks = old[:n-1]
	return task
}

// ========== 错误定义 ==========

var (
	ErrQueueFull   = fmt.Errorf("任务队列已满")
	ErrTaskNotFound = fmt.Errorf("任务不存在")
)

// GenerateTaskID 生成任务ID
func GenerateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}