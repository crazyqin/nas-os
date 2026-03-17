package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Scheduler 调度器
type Scheduler struct {
	tasks      map[string]*Task
	executor   *Executor
	logManager *LogManager
	retryMgr   *RetryManager
	cronTasks  map[string]*cronEntry
	mu         sync.RWMutex
	config     *SchedulerConfig
	running    bool
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

type cronEntry struct {
	taskID string
	expr   *CronExpression
	next   time.Time
}

// NewScheduler 创建调度器
func NewScheduler(config *SchedulerConfig) (*Scheduler, error) {
	if config == nil {
		config = &SchedulerConfig{
			MaxConcurrentTasks: 10,
			DefaultTimeout:     time.Hour,
			LogRetention:       7 * 24 * time.Hour,
			EnableRecovery:     true,
			HeartbeatInterval:  time.Minute,
		}
	}

	s := &Scheduler{
		tasks:     make(map[string]*Task),
		executor:  NewExecutor(config.MaxConcurrentTasks),
		cronTasks: make(map[string]*cronEntry),
		config:    config,
		stopChan:  make(chan struct{}),
	}

	// 初始化日志管理器
	var err error
	logPath := ""
	if config.StoragePath != "" {
		logPath = config.StoragePath + "/logs"
	}
	s.logManager, err = NewLogManager(logPath, 10000, config.LogRetention)
	if err != nil {
		return nil, fmt.Errorf("初始化日志管理器失败: %w", err)
	}

	// 初始化重试管理器
	s.retryMgr = NewRetryManager()
	s.retryMgr.SetScheduler(s)

	// 加载任务
	if config.StoragePath != "" {
		if err := s.loadTasks(); err != nil {
			return nil, fmt.Errorf("加载任务失败: %w", err)
		}
	}

	// 注册内置处理器
	_ = s.executor.RegisterHandler(NewCommandHandler())
	_ = s.executor.RegisterHandler(NewHTTPHandler())
	_ = s.executor.RegisterHandler(NewScriptHandler())

	return s, nil
}

// loadTasks 加载任务
func (s *Scheduler) loadTasks() error {
	data, err := os.ReadFile(s.config.StoragePath + "/tasks.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var tasks []*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}

	for _, task := range tasks {
		s.tasks[task.ID] = task
		_ = s.executor.RegisterTask(task)

		// 恢复 Cron 任务
		if task.Type == TaskTypeCron && task.Enabled {
			_ = s.scheduleCronTask(task)
		}
	}

	return nil
}

// saveTasks 保存任务
func (s *Scheduler) saveTasks() error {
	if s.config.StoragePath == "" {
		return nil
	}

	s.mu.RLock()
	tasks := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.config.StoragePath + "/tasks.json")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.config.StoragePath+"/tasks.json", data, 0644)
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	// 启动调度协程
	s.wg.Add(1)
	go s.runScheduler()

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	close(s.stopChan)
	s.wg.Wait()

	// 保存任务
	_ = s.saveTasks()
}

// runScheduler 运行调度循环
func (s *Scheduler) runScheduler() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

// tick 每秒检查
func (s *Scheduler) tick() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// 检查 Cron 任务
	for taskID, entry := range s.cronTasks {
		if now.After(entry.next) || now.Equal(entry.next) {
			task := s.tasks[taskID]
			if task != nil && task.Enabled && task.Status != TaskStatusRunning {
				go func(t *Task) { _, _ = s.runTaskInternal(t) }(task)
			}
			// 计算下次执行时间
			entry.next = entry.expr.Next(now)
			if task != nil {
				task.NextRunAt = &entry.next
			}
		}
	}

	// 检查一次性任务
	for _, task := range s.tasks {
		if task.Type == TaskTypeOneTime && task.Enabled && task.Status == TaskStatusPending {
			if task.ScheduledAt != nil && (now.After(*task.ScheduledAt) || now.Equal(*task.ScheduledAt)) {
				go func(t *Task) { _, _ = s.runTaskInternal(t) }(task)
			}
		}

		// 检查间隔任务
		if task.Type == TaskTypeInterval && task.Enabled && task.Status != TaskStatusRunning {
			if task.LastRunAt == nil {
				go func(t *Task) { _, _ = s.runTaskInternal(t) }(task)
			} else {
				interval, err := ParseDuration(task.Interval)
				if err == nil {
					nextRun := task.LastRunAt.Add(interval)
					if now.After(nextRun) {
						go func(t *Task) { _, _ = s.runTaskInternal(t) }(task)
					}
				}
			}
		}
	}
}

// scheduleCronTask 安排 Cron 任务
func (s *Scheduler) scheduleCronTask(task *Task) error {
	if task.CronExpression == "" {
		return fmt.Errorf("Cron 表达式为空")
	}

	expr, err := NewCronExpression(task.CronExpression, CronParseOptions{Second: true})
	if err != nil {
		return fmt.Errorf("解析 Cron 表达式失败: %w", err)
	}

	entry := &cronEntry{
		taskID: task.ID,
		expr:   expr,
		next:   expr.Next(time.Now()),
	}

	s.cronTasks[task.ID] = entry
	task.NextRunAt = &entry.next

	return nil
}

// AddTask 添加任务
func (s *Scheduler) AddTask(task *Task) error {
	if task.ID == "" {
		task.ID = GenerateTaskID()
	}

	if task.Name == "" {
		return fmt.Errorf("任务名称不能为空")
	}

	if task.Handler == "" {
		return fmt.Errorf("任务处理器不能为空")
	}

	// 应用默认值
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	if task.Priority == 0 {
		task.Priority = PriorityNormal
	}

	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	// 应用默认重试配置
	ApplyDefaultRetryConfig(task)

	s.mu.Lock()
	s.tasks[task.ID] = task
	if err := s.executor.RegisterTask(task); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("注册任务到执行器失败: %w", err)
	}
	s.mu.Unlock()

	// 如果是 Cron 任务，安排调度
	if task.Type == TaskTypeCron && task.Enabled {
		if err := s.scheduleCronTask(task); err != nil {
			return err
		}
	}

	return s.saveTasks()
}

// UpdateTask 更新任务
func (s *Scheduler) UpdateTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.tasks[task.ID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", task.ID)
	}

	// 保留创建时间和统计信息
	task.CreatedAt = existing.CreatedAt
	task.UpdatedAt = time.Now()
	task.RunCount = existing.RunCount
	task.SuccessCount = existing.SuccessCount
	task.FailCount = existing.FailCount

	s.tasks[task.ID] = task

	// 更新 Cron 任务
	if task.Type == TaskTypeCron {
		delete(s.cronTasks, task.ID)
		if task.Enabled {
			_ = s.scheduleCronTask(task)
		}
	}

	return s.saveTasks()
}

// DeleteTask 删除任务
func (s *Scheduler) DeleteTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[taskID]; !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	delete(s.tasks, taskID)
	delete(s.cronTasks, taskID)
	s.executor.UnregisterTask(taskID)

	return s.saveTasks()
}

// GetTask 获取任务
func (s *Scheduler) GetTask(taskID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	return task, nil
}

// ListTasks 列出任务
func (s *Scheduler) ListTasks(filter *TaskFilter) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Task, 0)
	for _, task := range s.tasks {
		if !s.matchFilter(task, filter) {
			continue
		}
		result = append(result, task)
	}

	// 分页
	if filter != nil {
		page := filter.Page
		if page < 1 {
			page = 1
		}
		pageSize := filter.PageSize
		if pageSize < 1 {
			pageSize = 20
		}

		start := (page - 1) * pageSize
		end := start + pageSize

		if start < len(result) {
			if end > len(result) {
				end = len(result)
			}
			result = result[start:end]
		} else {
			result = []*Task{}
		}
	}

	return result
}

func (s *Scheduler) matchFilter(task *Task, filter *TaskFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Status != "" && task.Status != filter.Status {
		return false
	}
	if filter.Type != "" && task.Type != filter.Type {
		return false
	}
	if filter.Group != "" && task.Group != filter.Group {
		return false
	}
	if filter.Enabled != nil && task.Enabled != *filter.Enabled {
		return false
	}
	if len(filter.Tags) > 0 {
		tagMatch := false
		for _, tag := range filter.Tags {
			for _, t := range task.Tags {
				if t == tag {
					tagMatch = true
					break
				}
			}
		}
		if !tagMatch {
			return false
		}
	}

	return true
}

// RunTask 运行任务
func (s *Scheduler) RunTask(taskID string) (*TaskExecution, error) {
	s.mu.RLock()
	task, exists := s.tasks[taskID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	return s.runTaskInternal(task)
}

func (s *Scheduler) runTaskInternal(task *Task) (*TaskExecution, error) {
	// 检查是否可以运行
	if task.Status == TaskStatusRunning {
		return nil, fmt.Errorf("任务正在运行")
	}

	// 执行任务
	execution, err := s.executor.ExecuteSync(context.Background(), task)

	// 记录日志
	_ = s.logManager.RecordExecution(execution)

	// 处理失败重试
	if err != nil && s.retryMgr.ShouldRetry(task, err) {
		_ = s.retryMgr.ScheduleRetry(task, err)
	}

	// 保存任务状态
	_ = s.saveTasks()

	return execution, err
}

// CancelTask 取消任务
func (s *Scheduler) CancelTask(taskID string) error {
	return s.executor.Cancel(taskID)
}

// PauseTask 暂停任务
func (s *Scheduler) PauseTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	task.Status = TaskStatusPaused
	task.UpdatedAt = time.Now()

	delete(s.cronTasks, taskID)

	return s.saveTasks()
}

// ResumeTask 恢复任务
func (s *Scheduler) ResumeTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	task.Status = TaskStatusPending
	task.UpdatedAt = time.Now()

	// 恢复 Cron 任务
	if task.Type == TaskTypeCron && task.Enabled {
		_ = s.scheduleCronTask(task)
	}

	return s.saveTasks()
}

// EnableTask 启用任务
func (s *Scheduler) EnableTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	task.Enabled = true
	task.UpdatedAt = time.Now()

	if task.Type == TaskTypeCron {
		_ = s.scheduleCronTask(task)
	}

	return s.saveTasks()
}

// DisableTask 禁用任务
func (s *Scheduler) DisableTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	task.Enabled = false
	task.UpdatedAt = time.Now()

	delete(s.cronTasks, taskID)

	return s.saveTasks()
}

// GetStats 获取统计信息
func (s *Scheduler) GetStats() *SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &SchedulerStats{
		TotalTasks:     len(s.tasks),
		TaskGroupStats: make(map[string]int),
	}

	for _, task := range s.tasks {
		switch task.Status {
		case TaskStatusRunning:
			stats.RunningTasks++
		case TaskStatusPending:
			stats.PendingTasks++
		case TaskStatusCompleted:
			stats.CompletedTasks++
		case TaskStatusFailed:
			stats.FailedTasks++
		case TaskStatusPaused:
			stats.PausedTasks++
		}

		if task.Group != "" {
			stats.TaskGroupStats[task.Group]++
		}
	}

	// 执行统计
	todayExecutions := s.logManager.GetTodayExecutions()
	stats.TodayExecutions = len(todayExecutions)

	stats.SuccessRate = calculateSuccessRate(todayExecutions)

	return stats
}

func calculateSuccessRate(executions []*TaskExecution) float64 {
	if len(executions) == 0 {
		return 0
	}

	successCount := 0
	for _, exec := range executions {
		if exec.Status == ExecutionStatusCompleted {
			successCount++
		}
	}

	return float64(successCount) / float64(len(executions)) * 100
}

// GetExecutor 获取执行器
func (s *Scheduler) GetExecutor() *Executor {
	return s.executor
}

// GetLogManager 获取日志管理器
func (s *Scheduler) GetLogManager() *LogManager {
	return s.logManager
}

// RegisterHandler 注册处理器
func (s *Scheduler) RegisterHandler(handler TaskHandler) error {
	return s.executor.RegisterHandler(handler)
}

// GetDependencyGraph 获取依赖图
func (s *Scheduler) GetDependencyGraph() *DependencyGraph {
	return s.executor.GetDependencyManager().GetGraph()
}

// GetNextRunTimes 获取任务的下次执行时间
func (s *Scheduler) GetNextRunTimes(taskID string, n int) []time.Time {
	s.mu.RLock()
	entry, exists := s.cronTasks[taskID]
	s.mu.RUnlock()

	if !exists {
		return nil
	}

	return entry.expr.NextN(time.Now(), n)
}
