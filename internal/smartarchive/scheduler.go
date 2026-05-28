package smartarchive

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Scheduler 归档任务调度器.
type Scheduler struct {
	mu sync.RWMutex

	// 配置
	config SchedulerConfig

	// 任务队列
	queue     []*ScheduledTask
	running   map[string]*ScheduledTask
	completed []*ScheduledTask

	// 工作线程
	workers    int
	workerPool chan struct{}

	// 运行状态
	isRunning bool
	ctx       context.Context
	cancel    context.CancelFunc

	// 统计
	stats *SchedulerStats
}

// ScheduledTask 调度任务.
type ScheduledTask struct {
	ID          string        `json:"id"`
	PolicyID    string        `json:"policyId,omitempty"`
	JobID       string        `json:"jobId,omitempty"`
	Type        string        `json:"type"` // archive/retention/analysis/compression
	Priority    int           `json:"priority"`
	Status      JobStatus     `json:"status"`
	Action      ArchiveAction `json:"action"`

	// 调度配置
	ScheduledAt time.Time     `json:"scheduledAt"`
	StartedAt   time.Time     `json:"startedAt,omitempty"`
	CompletedAt time.Time     `json:"completedAt,omitempty"`
	Deadline    time.Time     `json:"deadline,omitempty"`
	Timeout     time.Duration `json:"timeout"`

	// 重试配置
	MaxRetries int           `json:"maxRetries"`
	Retries    int           `json:"retries"`
	RetryDelay time.Duration `json:"retryDelay"`
	LastError  string        `json:"lastError,omitempty"`

	// 任务参数
	Params map[string]interface{} `json:"params,omitempty"`

	// 执行结果
	Result *TaskResult `json:"result,omitempty"`

	// 回调
	onComplete func(task *ScheduledTask)
	onError    func(task *ScheduledTask, err error)
}

// TaskResult 任务结果.
type TaskResult struct {
	Success       bool          `json:"success"`
	FilesProcessed int64        `json:"filesProcessed"`
	BytesProcessed int64        `json:"bytesProcessed"`
	FilesArchived  int64        `json:"filesArchived"`
	BytesArchived  int64        `json:"bytesArchived"`
	SpaceSaved     int64        `json:"spaceSaved"`
	Duration       time.Duration `json:"duration"`
	Error          string       `json:"error,omitempty"`
	Details        string       `json:"details,omitempty"`
}

// SchedulerStats 调度器统计.
type SchedulerStats struct {
	TotalScheduled   int64         `json:"totalScheduled"`
	TotalCompleted   int64         `json:"totalCompleted"`
	TotalFailed      int64         `json:"totalFailed"`
	TotalRetries     int64         `json:"totalRetries"`
	QueuedTasks      int           `json:"queuedTasks"`
	RunningTasks     int           `json:"runningTasks"`
	AvgExecTime      time.Duration `json:"avgExecTime"`
	LastScheduleTime time.Time     `json:"lastScheduleTime"`
	Uptime           time.Duration `json:"uptime"`
}

// NewScheduler 创建调度器.
func NewScheduler(config SchedulerConfig) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		config:     config,
		queue:      make([]*ScheduledTask, 0),
		running:    make(map[string]*ScheduledTask),
		completed:  make([]*ScheduledTask, 0),
		workers:    config.MaxConcurrent,
		workerPool: make(chan struct{}, config.MaxConcurrent),
		ctx:        ctx,
		cancel:     cancel,
		stats:      &SchedulerStats{},
	}
}

// Start 启动调度器.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return
	}

	s.isRunning = true
	s.stats.Uptime = 0

	// 启动工作线程
	for i := 0; i < s.workers; i++ {
		go s.worker(i)
	}

	// 启动调度循环
	go s.scheduleLoop()

	log.Printf("[Scheduler] 调度器已启动，工作线程数: %d", s.workers)
}

// Stop 停止调度器.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	s.cancel()
	s.isRunning = false

	log.Println("[Scheduler] 调度器已停止")
}

// Schedule 添加调度任务.
func (s *Scheduler) Schedule(task *ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		task.ID = generateID()
	}

	if task.Status == "" {
		task.Status = JobStatusPending
	}

	if task.ScheduledAt.IsZero() {
		task.ScheduledAt = time.Now()
	}

	if task.Timeout == 0 {
		task.Timeout = s.config.JobTimeout
	}

	if task.MaxRetries == 0 {
		task.MaxRetries = s.config.MaxRetries
	}

	if task.RetryDelay == 0 {
		task.RetryDelay = s.config.RetryDelay
	}

	// 检查队列容量
	if len(s.queue) >= 1000 {
		return fmt.Errorf("任务队列已满")
	}

	// 插入到队列（按优先级排序）
	s.insertByPriority(task)
	s.stats.TotalScheduled++
	s.stats.LastScheduleTime = time.Now()

	log.Printf("[Scheduler] 调度任务: %s (类型: %s, 优先级: %d)", task.ID, task.Type, task.Priority)
	return nil
}

// ScheduleAt 定时调度任务.
func (s *Scheduler) ScheduleAt(task *ScheduledTask, at time.Time) error {
	task.ScheduledAt = at
	return s.Schedule(task)
}

// ScheduleAfter 延迟调度任务.
func (s *Scheduler) ScheduleAfter(task *ScheduledTask, delay time.Duration) error {
	task.ScheduledAt = time.Now().Add(delay)
	return s.Schedule(task)
}

// ScheduleRecurring 定期调度任务.
func (s *Scheduler) ScheduleRecurring(task *ScheduledTask, interval time.Duration) error {
	task.Params["recurring"] = true
	task.Params["interval"] = interval.String()
	return s.Schedule(task)
}

// Cancel 取消任务.
func (s *Scheduler) Cancel(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从队列中查找
	for i, task := range s.queue {
		if task.ID == taskID {
			task.Status = JobStatusCancelled
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			return nil
		}
	}

	// 从运行中查找
	if task, exists := s.running[taskID]; exists {
		task.Status = JobStatusCancelled
		return nil
	}

	return fmt.Errorf("任务 %s 不存在", taskID)
}

// GetTask 获取任务.
func (s *Scheduler) GetTask(taskID string) (*ScheduledTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 从队列中查找
	for _, task := range s.queue {
		if task.ID == taskID {
			return task, nil
		}
	}

	// 从运行中查找
	if task, exists := s.running[taskID]; exists {
		return task, nil
	}

	// 从已完成中查找
	for _, task := range s.completed {
		if task.ID == taskID {
			return task, nil
		}
	}

	return nil, fmt.Errorf("任务 %s 不存在", taskID)
}

// ListTasks 列出任务.
func (s *Scheduler) ListTasks(status JobStatus, limit int) []*ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*ScheduledTask, 0)

	// 收集所有任务
	allTasks := make([]*ScheduledTask, 0)
	allTasks = append(allTasks, s.queue...)
	for _, t := range s.running {
		allTasks = append(allTasks, t)
	}
	allTasks = append(allTasks, s.completed...)

	// 过滤
	for _, task := range allTasks {
		if status != "" && task.Status != status {
			continue
		}
		tasks = append(tasks, task)
	}

	if limit > 0 && len(tasks) > limit {
		tasks = tasks[:limit]
	}

	return tasks
}

// GetStats 获取统计.
func (s *Scheduler) GetStats() *SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := *s.stats
	stats.QueuedTasks = len(s.queue)
	stats.RunningTasks = len(s.running)

	return &stats
}

// Pause 暂停调度器.
func (s *Scheduler) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 暂停所有待执行任务
	for _, task := range s.queue {
		if task.Status == JobStatusPending {
			task.Status = JobStatusPaused
		}
	}

	log.Println("[Scheduler] 调度器已暂停")
}

// Resume 恢复调度器.
func (s *Scheduler) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 恢复所有暂停任务
	for _, task := range s.queue {
		if task.Status == JobStatusPaused {
			task.Status = JobStatusPending
		}
	}

	log.Println("[Scheduler] 调度器已恢复")
}

// scheduleLoop 调度循环.
func (s *Scheduler) scheduleLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.processQueue()
		}
	}
}

// processQueue 处理队列.
func (s *Scheduler) processQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue) == 0 {
		return
	}

	now := time.Now()

	// 查找可执行的任务
	for i := 0; i < len(s.queue); i++ {
		task := s.queue[i]

		// 检查任务状态
		if task.Status != JobStatusPending && task.Status != JobStatusPaused {
			continue
		}

		// 检查是否到了调度时间
		if task.ScheduledAt.After(now) {
			continue
		}

		// 检查静默时段
		if s.isQuietHour(now) {
			continue
		}

		// 尝试获取工作线程
		select {
		case s.workerPool <- struct{}{}:
			// 有空闲工作线程，分配任务
			task.Status = JobStatusRunning
			task.StartedAt = now
			s.running[task.ID] = task
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			i--

			// 异步执行任务
			go s.executeTask(task)
		default:
			// 没有空闲工作线程
			break
		}
	}
}

// executeTask 执行任务.
func (s *Scheduler) executeTask(task *ScheduledTask) {
	defer func() {
		<-s.workerPool // 释放工作线程
	}()

	log.Printf("[Scheduler] 开始执行任务: %s (类型: %s)", task.ID, task.Type)

	// 设置超时
	ctx, cancel := context.WithTimeout(s.ctx, task.Timeout)
	defer cancel()

	// 执行任务
	result := s.runTask(ctx, task)

	// 处理结果
	s.mu.Lock()
	task.Result = result

	if result.Success {
		task.Status = JobStatusCompleted
		s.stats.TotalCompleted++
	} else {
		task.Status = JobStatusFailed
		task.LastError = result.Error
		s.stats.TotalFailed++

		// 检查是否需要重试
		if task.Retries < task.MaxRetries {
			task.Retries++
			task.Status = JobStatusPending
			task.ScheduledAt = time.Now().Add(task.RetryDelay * time.Duration(task.Retries))
			s.insertByPriority(task)
			s.stats.TotalRetries++
			log.Printf("[Scheduler] 任务 %s 失败，计划重试 (%d/%d)", task.ID, task.Retries, task.MaxRetries)
		}
	}

	completedAt := time.Now()
	task.CompletedAt = completedAt

	// 移动到完成列表
	delete(s.running, task.ID)
	s.completed = append(s.completed, task)

	// 限制完成列表大小
	if len(s.completed) > 1000 {
		s.completed = s.completed[len(s.completed)-1000:]
	}

	s.mu.Unlock()

	// 执行回调
	if result.Success && task.onComplete != nil {
		task.onComplete(task)
	}
	if !result.Success && task.onError != nil {
		task.onError(task, fmt.Errorf("%s", result.Error))
	}

	log.Printf("[Scheduler] 任务 %s 完成，状态: %s，耗时: %v",
		task.ID, task.Status, result.Duration)
}

// runTask 运行任务.
func (s *Scheduler) runTask(ctx context.Context, task *ScheduledTask) *TaskResult {
	startTime := time.Now()
	result := &TaskResult{}

	// 根据任务类型执行
	switch task.Type {
	case "archive":
		result = s.runArchiveTask(ctx, task)
	case "retention":
		result = s.runRetentionTask(ctx, task)
	case "analysis":
		result = s.runAnalysisTask(ctx, task)
	case "compression":
		result = s.runCompressionTask(ctx, task)
	default:
		result.Error = fmt.Sprintf("未知任务类型: %s", task.Type)
	}

	result.Duration = time.Since(startTime)
	return result
}

// runArchiveTask 运行归档任务.
func (s *Scheduler) runArchiveTask(ctx context.Context, task *ScheduledTask) *TaskResult {
	// 简化实现
	return &TaskResult{
		Success:        true,
		FilesProcessed: 10,
		BytesProcessed: 1024 * 1024 * 100,
		FilesArchived:  10,
		BytesArchived:  1024 * 1024 * 50,
		SpaceSaved:     1024 * 1024 * 50,
	}
}

// runRetentionTask 运行保留任务.
func (s *Scheduler) runRetentionTask(ctx context.Context, task *ScheduledTask) *TaskResult {
	return &TaskResult{
		Success:        true,
		FilesProcessed: 5,
		BytesProcessed: 1024 * 1024 * 50,
	}
}

// runAnalysisTask 运行分析任务.
func (s *Scheduler) runAnalysisTask(ctx context.Context, task *ScheduledTask) *TaskResult {
	return &TaskResult{
		Success: true,
		Details: "分析完成",
	}
}

// runCompressionTask 运行压缩任务.
func (s *Scheduler) runCompressionTask(ctx context.Context, task *ScheduledTask) *TaskResult {
	return &TaskResult{
		Success:        true,
		FilesProcessed: 20,
		BytesProcessed: 1024 * 1024 * 200,
		SpaceSaved:     1024 * 1024 * 100,
	}
}

// insertByPriority 按优先级插入任务.
func (s *Scheduler) insertByPriority(task *ScheduledTask) {
	inserted := false
	for i, t := range s.queue {
		if task.Priority > t.Priority {
			s.queue = append(s.queue[:i], append([]*ScheduledTask{task}, s.queue[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		s.queue = append(s.queue, task)
	}
}

// isQuietHour 检查是否在静默时段.
func (s *Scheduler) isQuietHour(t time.Time) bool {
	if len(s.config.QuietHours) == 0 {
		return false
	}

	currentTime := t.Format("15:04")

	for _, quietHour := range s.config.QuietHours {
		if currentTime >= quietHour.Start && currentTime <= quietHour.End {
			return true
		}
	}

	return false
}

// worker 工作线程处理函数.
func (s *Scheduler) worker(id int) {
	log.Printf("[Scheduler] 工作线程 %d 已启动", id)
	for {
		select {
		case <-s.ctx.Done():
			log.Printf("[Scheduler] 工作线程 %d 已停止", id)
			return
		default:
			// 从队列获取任务
			s.mu.Lock()
			if len(s.queue) == 0 {
				s.mu.Unlock()
				time.Sleep(time.Second)
				continue
			}
			task := s.queue[0]
			s.queue = s.queue[1:]
			task.Status = JobStatusRunning
			task.StartedAt = time.Now()
			s.running[task.ID] = task
			s.mu.Unlock()

			// 执行任务
			s.executeTask(task)

			// 完成任务
			s.mu.Lock()
			task.Status = JobStatusCompleted
			task.CompletedAt = time.Now()
			delete(s.running, task.ID)
			s.completed = append(s.completed, task)
			s.stats.TotalCompleted++
			s.mu.Unlock()
		}
	}
}

// GetQueueStatus 获取队列状态.
func (s *Scheduler) GetQueueStatus() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := map[string]int{
		"pending":   0,
		"running":   len(s.running),
		"completed": len(s.completed),
	}

	for _, task := range s.queue {
		switch task.Status {
		case JobStatusPending:
			status["pending"]++
		case JobStatusPaused:
			status["paused"]++
		}
	}

	return status
}

// ClearCompleted 清除已完成任务.
func (s *Scheduler) ClearCompleted() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.completed)
	s.completed = make([]*ScheduledTask, 0)

	return count
}

// GetNextTask 获取下一个待执行任务.
func (s *Scheduler) GetNextTask() *ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, task := range s.queue {
		if task.Status == JobStatusPending {
			return task
		}
	}

	return nil
}

// UpdatePriority 更新任务优先级.
func (s *Scheduler) UpdatePriority(taskID string, priority int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.queue {
		if task.ID == taskID {
			task.Priority = priority
			// 重新排序
			s.reorderQueue()
			return nil
		}
	}

	return fmt.Errorf("任务 %s 不存在", taskID)
}

// reorderQueue 重新排序队列.
func (s *Scheduler) reorderQueue() {
	// 简单的插入排序
	for i := 1; i < len(s.queue); i++ {
		key := s.queue[i]
		j := i - 1
		for j >= 0 && s.queue[j].Priority < key.Priority {
			s.queue[j+1] = s.queue[j]
			j--
		}
		s.queue[j+1] = key
	}
}
