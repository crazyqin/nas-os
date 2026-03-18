// Package backup 备份调度器
// Version: v2.50.0 - 智能备份调度模块
package backup

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// ============================================================================
// 备份调度器
// ============================================================================

// BackupScheduler 备份调度器
// 支持定时备份、优先级队列、备份窗口配置、并发控制、依赖管理
type BackupScheduler struct {
	mu sync.RWMutex

	// 调度器配置
	config *SchedulerConfig

	// Cron 调度器
	cronScheduler *cron.Cron

	// 任务队列
	queue       *PriorityQueue
	runningJobs map[string]*ScheduledJob

	// 并发控制
	semaphore chan struct{}

	// 备份管理器引用
	manager *SmartManager

	// 作业调度映射
	jobEntries map[string]cron.EntryID

	// 状态追踪
	running int32
	stopped int32

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc

	// 日志
	logger *zap.Logger

	// 统计
	stats *SchedulerStats

	// 回调函数
	onJobStart    func(job *ScheduledJob)
	onJobComplete func(job *ScheduledJob)
	onJobFail     func(job *ScheduledJob, err error)
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	// 并发配置
	MaxConcurrent int `json:"max_concurrent"` // 最大并发备份数

	// 队列配置
	QueueSize int `json:"queue_size"` // 队列大小

	// 备份窗口（避开高峰期）
	WindowStartHour int  `json:"window_start_hour"` // 允许开始时间（小时，如 22 表示 22:00）
	WindowEndHour   int  `json:"window_end_hour"`   // 允许结束时间（小时，如 6 表示 06:00）
	WindowEnabled   bool `json:"window_enabled"`    // 是否启用备份窗口

	// 重试配置
	RetryEnabled     bool `json:"retry_enabled"`     // 是否启用重试
	RetryMax         int  `json:"retry_max"`         // 最大重试次数
	RetryDelayMin    int  `json:"retry_delay_min"`   // 重试延迟（分钟）
	RetryExponential bool `json:"retry_exponential"` // 指数退避

	// 超时配置
	DefaultTimeoutMin int `json:"default_timeout_min"` // 默认超时时间（分钟）

	// 调度配置
	Timezone string `json:"timezone"` // 时区
}

// DefaultSchedulerConfig 默认调度器配置
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		MaxConcurrent:     3,
		QueueSize:         100,
		WindowStartHour:   22,
		WindowEndHour:     6,
		WindowEnabled:     true,
		RetryEnabled:      true,
		RetryMax:          3,
		RetryDelayMin:     5,
		RetryExponential:  true,
		DefaultTimeoutMin: 120,
		Timezone:          "Local",
	}
}

// ScheduledJob 调度作业
type ScheduledJob struct {
	ID           string        `json:"id"`
	BackupJobID  string        `json:"backup_job_id"`
	Name         string        `json:"name"`
	Priority     int           `json:"priority"`
	Schedule     string        `json:"schedule"`
	Status       JobStatus     `json:"status"`
	Dependencies []string      `json:"dependencies"`
	Window       *BackupWindow `json:"window,omitempty"`
	Timeout      time.Duration `json:"timeout"`
	MaxRetries   int           `json:"max_retries"`
	RetryCount   int           `json:"retry_count"`
	CreatedAt    time.Time     `json:"created_at"`
	LastRun      *time.Time    `json:"last_run,omitempty"`
	NextRun      *time.Time    `json:"next_run,omitempty"`
	LastError    string        `json:"last_error,omitempty"`
	entryID      cron.EntryID
}

// JobStatus 作业状态
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusRetrying  JobStatus = "retrying"
)

// BackupWindow 备份窗口
type BackupWindow struct {
	StartHour int   `json:"start_hour"`
	EndHour   int   `json:"end_hour"`
	Days      []int `json:"days,omitempty"` // 0=Sunday, 1=Monday, ...
}

// PriorityQueue 优先级队列
type PriorityQueue struct {
	items []*QueueItem
	mu    sync.RWMutex
}

// QueueItem 队列项
type QueueItem struct {
	Job      *ScheduledJob
	Priority int
	AddedAt  time.Time
}

// SchedulerStats 调度器统计
type SchedulerStats struct {
	TotalScheduled   int64         `json:"total_scheduled"`
	TotalCompleted   int64         `json:"total_completed"`
	TotalFailed      int64         `json:"total_failed"`
	TotalRetries     int64         `json:"total_retries"`
	QueueLength      int           `json:"queue_length"`
	RunningJobs      int           `json:"running_jobs"`
	AvgExecutionTime time.Duration `json:"avg_execution_time"`
	LastRunTime      *time.Time    `json:"last_run_time,omitempty"`
	mu               sync.RWMutex
	times            []time.Duration
}

// ============================================================================
// 创建与初始化
// ============================================================================

// NewBackupScheduler 创建备份调度器
func NewBackupScheduler(config *SchedulerConfig, manager *SmartManager, logger *zap.Logger) *BackupScheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	scheduler := &BackupScheduler{
		config:      config,
		queue:       NewPriorityQueue(config.QueueSize),
		runningJobs: make(map[string]*ScheduledJob),
		semaphore:   make(chan struct{}, config.MaxConcurrent),
		manager:     manager,
		jobEntries:  make(map[string]cron.EntryID),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
		stats: &SchedulerStats{
			times: make([]time.Duration, 0, 100),
		},
	}

	// 初始化 cron 调度器
	scheduler.initCronScheduler()

	return scheduler
}

// initCronScheduler 初始化 Cron 调度器
func (s *BackupScheduler) initCronScheduler() {
	// 使用秒级精度 cron
	s.cronScheduler = cron.New(cron.WithSeconds(), cron.WithLocation(time.Local))
}

// Start 启动调度器
func (s *BackupScheduler) Start() error {
	if atomic.SwapInt32(&s.running, 1) == 1 {
		return errors.New("调度器已在运行")
	}

	s.cronScheduler.Start()

	// 启动队列处理器
	go s.processQueue()

	// 启动统计更新器
	go s.updateStats()

	s.logger.Info("备份调度器已启动",
		zap.Int("max_concurrent", s.config.MaxConcurrent),
		zap.Bool("window_enabled", s.config.WindowEnabled),
	)

	return nil
}

// Stop 停止调度器
func (s *BackupScheduler) Stop() error {
	if atomic.SwapInt32(&s.stopped, 1) == 1 {
		return errors.New("调度器已停止")
	}

	// 停止 cron 调度器
	ctx := s.cronScheduler.Stop()
	<-ctx.Done()

	// 取消所有运行中的作业
	s.mu.Lock()
	for _, job := range s.runningJobs {
		job.Status = JobStatusCancelled
	}
	s.mu.Unlock()

	s.cancel()

	s.logger.Info("备份调度器已停止")

	return nil
}

// ============================================================================
// 作业调度
// ============================================================================

// ScheduleJob 调度作业
func (s *BackupScheduler) ScheduleJob(job *ScheduledJob) error {
	if job.ID == "" {
		job.ID = generateUUID()
	}

	if job.Schedule == "" {
		return errors.New("调度表达式不能为空")
	}

	// 验证 cron 表达式
	schedule, err := cron.ParseStandard(job.Schedule)
	if err != nil {
		return fmt.Errorf("无效的 cron 表达式: %w", err)
	}

	// 计算下次运行时间
	now := time.Now()
	nextRun := schedule.Next(now)
	job.NextRun = &nextRun
	job.Status = JobStatusPending

	// 添加到 cron 调度器
	entryID, err := s.cronScheduler.AddFunc(job.Schedule, func() {
		s.executeScheduledJob(job)
	})
	if err != nil {
		return fmt.Errorf("添加调度任务失败: %w", err)
	}

	job.entryID = entryID

	s.mu.Lock()
	s.jobEntries[job.ID] = entryID
	s.mu.Unlock()

	s.stats.mu.Lock()
	s.stats.TotalScheduled++
	s.stats.mu.Unlock()

	s.logger.Info("作业已调度",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("schedule", job.Schedule),
		zap.Time("next_run", nextRun),
	)

	return nil
}

// UnscheduleJob 取消调度作业
func (s *BackupScheduler) UnscheduleJob(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, exists := s.jobEntries[jobID]
	if !exists {
		return fmt.Errorf("作业不存在: %s", jobID)
	}

	s.cronScheduler.Remove(entryID)
	delete(s.jobEntries, jobID)

	s.logger.Info("作业调度已取消", zap.String("job_id", jobID))

	return nil
}

// executeScheduledJob 执行调度作业
func (s *BackupScheduler) executeScheduledJob(job *ScheduledJob) {
	// 检查备份窗口
	if s.config.WindowEnabled && !s.isWithinWindow() {
		s.logger.Debug("当前时间不在备份窗口内，跳过执行",
			zap.String("job_id", job.ID),
		)
		return
	}

	// 检查依赖
	if !s.checkDependencies(job) {
		s.logger.Warn("作业依赖未满足，跳过执行",
			zap.String("job_id", job.ID),
		)
		return
	}

	// 添加到队列
	job.Status = JobStatusQueued
	s.queue.Push(job)

	s.logger.Info("作业已加入队列",
		zap.String("job_id", job.ID),
		zap.Int("priority", job.Priority),
		zap.Int("queue_length", s.queue.Len()),
	)
}

// processQueue 处理队列
func (s *BackupScheduler) processQueue() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 尝试获取信号量
		select {
		case s.semaphore <- struct{}{}:
			// 从队列获取作业
			item := s.queue.Pop()
			if item == nil {
				<-s.semaphore
				time.Sleep(100 * time.Millisecond)
				continue
			}

			job := item.Job

			// 启动 goroutine 执行作业
			go func() {
				defer func() { <-s.semaphore }()

				s.runJob(job)
			}()

		default:
			// 达到并发限制，等待
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// runJob 运行作业
func (s *BackupScheduler) runJob(job *ScheduledJob) {
	s.mu.Lock()
	s.runningJobs[job.ID] = job
	job.Status = JobStatusRunning
	now := time.Now()
	job.LastRun = &now
	s.mu.Unlock()

	// 回调
	if s.onJobStart != nil {
		s.onJobStart(job)
	}

	startTime := time.Now()

	// 执行备份
	activeJob, err := s.manager.RunBackup(job.BackupJobID)
	if err != nil {
		s.handleJobError(job, err)
		return
	}

	// 等待完成或超时
	timeout := job.Timeout
	if timeout == 0 {
		timeout = time.Duration(s.config.DefaultTimeoutMin) * time.Minute
	}

	select {
	case <-time.After(timeout):
		s.handleJobError(job, errors.New("作业超时"))
	case <-s.waitForCompletion(activeJob):
		if activeJob.Status == "completed" {
			s.handleJobComplete(job, time.Since(startTime))
		} else {
			s.handleJobError(job, errors.New(activeJob.Error))
		}
	case <-s.ctx.Done():
		s.handleJobError(job, errors.New("调度器已停止"))
	}

	s.mu.Lock()
	delete(s.runningJobs, job.ID)
	s.mu.Unlock()
}

// waitForCompletion 等待作业完成
func (s *BackupScheduler) waitForCompletion(activeJob *ActiveBackupJob) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			if activeJob.Status != "running" {
				close(ch)
				return
			}
		}
	}()
	return ch
}

// handleJobComplete 处理作业完成
func (s *BackupScheduler) handleJobComplete(job *ScheduledJob, duration time.Duration) {
	job.Status = JobStatusCompleted
	job.LastError = ""

	s.stats.mu.Lock()
	s.stats.TotalCompleted++
	s.stats.times = append(s.stats.times, duration)
	if len(s.stats.times) > 100 {
		s.stats.times = s.stats.times[1:]
	}
	var total time.Duration
	for _, t := range s.stats.times {
		total += t
	}
	s.stats.AvgExecutionTime = total / time.Duration(len(s.stats.times))
	s.stats.mu.Unlock()

	s.logger.Info("作业完成",
		zap.String("job_id", job.ID),
		zap.Duration("duration", duration),
	)

	if s.onJobComplete != nil {
		s.onJobComplete(job)
	}
}

// handleJobError 处理作业错误
func (s *BackupScheduler) handleJobError(job *ScheduledJob, err error) {
	job.LastError = err.Error()

	// 检查是否需要重试
	if s.config.RetryEnabled && job.RetryCount < job.MaxRetries {
		job.RetryCount++
		job.Status = JobStatusRetrying

		delay := time.Duration(s.config.RetryDelayMin) * time.Minute
		if s.config.RetryExponential {
			delay = delay * time.Duration(1<<uint(job.RetryCount-1))
		}

		s.stats.mu.Lock()
		s.stats.TotalRetries++
		s.stats.mu.Unlock()

		s.logger.Warn("作业失败，准备重试",
			zap.String("job_id", job.ID),
			zap.Int("retry_count", job.RetryCount),
			zap.Duration("delay", delay),
			zap.Error(err),
		)

		// 延迟重试
		time.AfterFunc(delay, func() {
			s.queue.Push(job)
		})

		if s.onJobFail != nil {
			s.onJobFail(job, err)
		}
		return
	}

	job.Status = JobStatusFailed

	s.stats.mu.Lock()
	s.stats.TotalFailed++
	s.stats.mu.Unlock()

	s.logger.Error("作业失败",
		zap.String("job_id", job.ID),
		zap.Error(err),
	)

	if s.onJobFail != nil {
		s.onJobFail(job, err)
	}
}

// isWithinWindow 检查是否在备份窗口内
func (s *BackupScheduler) isWithinWindow() bool {
	now := time.Now()
	hour := now.Hour()

	start := s.config.WindowStartHour
	end := s.config.WindowEndHour

	if start > end {
		// 跨天窗口（如 22:00 - 06:00）
		return hour >= start || hour < end
	}

	return hour >= start && hour < end
}

// checkDependencies 检查依赖
func (s *BackupScheduler) checkDependencies(job *ScheduledJob) bool {
	if len(job.Dependencies) == 0 {
		return true
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, depID := range job.Dependencies {
		depJob, exists := s.runningJobs[depID]
		if !exists {
			continue
		}

		// 依赖正在运行，等待
		if depJob.Status == JobStatusRunning || depJob.Status == JobStatusQueued {
			return false
		}

		// 依赖失败
		if depJob.Status == JobStatusFailed {
			return false
		}
	}

	return true
}

// ============================================================================
// 队列管理
// ============================================================================

// NewPriorityQueue 创建优先级队列
func NewPriorityQueue(size int) *PriorityQueue {
	return &PriorityQueue{
		items: make([]*QueueItem, 0, size),
	}
}

// Push 添加元素
func (q *PriorityQueue) Push(job *ScheduledJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	item := &QueueItem{
		Job:      job,
		Priority: job.Priority,
		AddedAt:  time.Now(),
	}

	// 按优先级插入（优先级高的在前）
	idx := sort.Search(len(q.items), func(i int) bool {
		return q.items[i].Priority < item.Priority
	})

	// 插入到正确位置
	q.items = append(q.items[:idx], append([]*QueueItem{item}, q.items[idx:]...)...)
}

// Pop 弹出元素
func (q *PriorityQueue) Pop() *QueueItem {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item
}

// Len 获取队列长度
func (q *PriorityQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.items)
}

// Peek 查看队首元素
func (q *PriorityQueue) Peek() *QueueItem {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.items) == 0 {
		return nil
	}

	return q.items[0]
}

// Clear 清空队列
func (q *PriorityQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = q.items[:0]
}

// ============================================================================
// 手动触发
// ============================================================================

// TriggerJob 手动触发作业
func (s *BackupScheduler) TriggerJob(jobID string, priority int) error {
	// 获取备份作业
	backupJob, err := s.manager.GetJob(jobID)
	if err != nil {
		return err
	}

	scheduledJob := &ScheduledJob{
		ID:          generateUUID(),
		BackupJobID: jobID,
		Name:        backupJob.Name,
		Priority:    priority,
		Status:      JobStatusQueued,
		Timeout:     time.Duration(s.config.DefaultTimeoutMin) * time.Minute,
		MaxRetries:  s.config.RetryMax,
		CreatedAt:   time.Now(),
	}

	s.queue.Push(scheduledJob)

	s.logger.Info("手动触发作业",
		zap.String("job_id", jobID),
		zap.Int("priority", priority),
	)

	return nil
}

// TriggerAllJobs 触发所有作业
func (s *BackupScheduler) TriggerAllJobs() error {
	jobs := s.manager.ListJobs()

	for _, job := range jobs {
		if !job.Enabled {
			continue
		}

		if err := s.TriggerJob(job.ID, job.Priority); err != nil {
			s.logger.Error("触发作业失败",
				zap.String("job_id", job.ID),
				zap.Error(err),
			)
		}
	}

	return nil
}

// ============================================================================
// 回调注册
// ============================================================================

// OnJobStart 注册作业开始回调
func (s *BackupScheduler) OnJobStart(callback func(job *ScheduledJob)) {
	s.onJobStart = callback
}

// OnJobComplete 注册作业完成回调
func (s *BackupScheduler) OnJobComplete(callback func(job *ScheduledJob)) {
	s.onJobComplete = callback
}

// OnJobFail 注册作业失败回调
func (s *BackupScheduler) OnJobFail(callback func(job *ScheduledJob, err error)) {
	s.onJobFail = callback
}

// ============================================================================
// 状态与统计
// ============================================================================

// GetStats 获取统计信息
func (s *BackupScheduler) GetStats() *SchedulerStats {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	stats := &SchedulerStats{
		TotalScheduled:   s.stats.TotalScheduled,
		TotalCompleted:   s.stats.TotalCompleted,
		TotalFailed:      s.stats.TotalFailed,
		TotalRetries:     s.stats.TotalRetries,
		QueueLength:      s.queue.Len(),
		AvgExecutionTime: s.stats.AvgExecutionTime,
	}

	s.mu.RLock()
	stats.RunningJobs = len(s.runningJobs)
	s.mu.RUnlock()

	return stats
}

// GetRunningJobs 获取运行中的作业
func (s *BackupScheduler) GetRunningJobs() []*ScheduledJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*ScheduledJob, 0, len(s.runningJobs))
	for _, job := range s.runningJobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// GetQueue 获取队列中的作业
func (s *BackupScheduler) GetQueue() []*ScheduledJob {
	s.queue.mu.RLock()
	defer s.queue.mu.RUnlock()

	jobs := make([]*ScheduledJob, 0, len(s.queue.items))
	for _, item := range s.queue.items {
		jobs = append(jobs, item.Job)
	}

	return jobs
}

// IsRunning 检查调度器是否运行中
func (s *BackupScheduler) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1 && atomic.LoadInt32(&s.stopped) == 0
}

// updateStats 更新统计
func (s *BackupScheduler) updateStats() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.stats.mu.Lock()
			s.stats.QueueLength = s.queue.Len()
			s.stats.mu.Unlock()
		case <-s.ctx.Done():
			return
		}
	}
}

// ============================================================================
// 备份窗口管理
// ============================================================================

// SetWindow 设置备份窗口
func (s *BackupScheduler) SetWindow(startHour, endHour int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.WindowStartHour = startHour
	s.config.WindowEndHour = endHour
	s.config.WindowEnabled = true

	s.logger.Info("备份窗口已更新",
		zap.Int("start_hour", startHour),
		zap.Int("end_hour", endHour),
	)
}

// DisableWindow 禁用备份窗口
func (s *BackupScheduler) DisableWindow() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.WindowEnabled = false
	s.logger.Info("备份窗口已禁用")
}

// GetWindow 获取备份窗口
func (s *BackupScheduler) GetWindow() *BackupWindow {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.config.WindowEnabled {
		return nil
	}

	return &BackupWindow{
		StartHour: s.config.WindowStartHour,
		EndHour:   s.config.WindowEndHour,
	}
}

// ============================================================================
// 配置热更新
// ============================================================================

// UpdateConfig 更新配置
func (s *BackupScheduler) UpdateConfig(config *SchedulerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新并发限制
	if config.MaxConcurrent != s.config.MaxConcurrent {
		// 重新创建信号量
		newSem := make(chan struct{}, config.MaxConcurrent)
		// 迁移现有信号
		for i := 0; i < len(s.semaphore); i++ {
			newSem <- struct{}{}
		}
		s.semaphore = newSem
	}

	s.config = config

	s.logger.Info("调度器配置已更新",
		zap.Int("max_concurrent", config.MaxConcurrent),
		zap.Bool("window_enabled", config.WindowEnabled),
	)

	return nil
}

// GetConfig 获取当前配置
func (s *BackupScheduler) GetConfig() *SchedulerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}
