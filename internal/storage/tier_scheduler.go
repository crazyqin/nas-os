// Package storage 提供存储分层调度器
// 实现热冷数据自动迁移调度，参考 Synology DSM 7.3 群晖分层存储
package storage

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TierScheduler 分层存储调度器
// 负责协调热数据提升和冷数据降级的调度执行
type TierScheduler struct {
	mu sync.RWMutex

	// 调度器配置
	config SchedulerConfig

	// 热冷池管理器
	hotColdManager *HotColdManager

	// 迁移策略管理器
	policyManager *MigrationPolicyManager

	// 调度任务队列
	taskQueue []*ScheduledTask

	// 运行状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// 日志
	logger *slog.Logger

	// 统计
	stats SchedulerStats

	// 回调
	onTaskComplete func(task *ScheduledTask)
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	// 检查间隔
	CheckInterval time.Duration `json:"checkInterval"`

	// 最大并发任务数
	MaxConcurrentTasks int `json:"maxConcurrentTasks"`

	// 热数据阈值（访问次数）
	HotAccessThreshold int64 `json:"hotAccessThreshold"`

	// 冷数据阈值（未访问天数）
	ColdAgeDays int `json:"coldAgeDays"`

	// SSD空间保护比例（保留百分比）
	SSDSpaceReservePercent int `json:"ssdSpaceReservePercent"`

	// 迁移速率限制（MB/s）
	MigrationRateLimit int64 `json:"migrationRateLimit"`

	// 启用智能预测
	EnablePrediction bool `json:"enablePrediction"`

	// 预测模型更新间隔
	PredictionUpdateInterval time.Duration `json:"predictionUpdateInterval"`

	// 任务超时时间
	TaskTimeout time.Duration `json:"taskTimeout"`

	// 失败重试次数
	MaxRetryCount int `json:"maxRetryCount"`

	// 调度策略
	ScheduleStrategy ScheduleStrategy `json:"scheduleStrategy"`
}

// ScheduleStrategy 调度策略类型
type ScheduleStrategy string

const (
	// ScheduleStrategyPriority 优先级调度（热数据优先）
	ScheduleStrategyPriority ScheduleStrategy = "priority"

	// ScheduleStrategyBalance 平衡调度（热冷数据交替）
	ScheduleStrategyBalance ScheduleStrategy = "balance"

	// ScheduleStrategySpace 空间优先（根据空间压力调度）
	ScheduleStrategySpace ScheduleStrategy = "space"

	// ScheduleStrategyTime 时间窗口调度（按策略时间执行）
	ScheduleStrategyTime ScheduleStrategy = "time"
)

// DefaultSchedulerConfig 默认调度器配置
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		CheckInterval:            1 * time.Hour,
		MaxConcurrentTasks:       3,
		HotAccessThreshold:       100,
		ColdAgeDays:              30,
		SSDSpaceReservePercent:   20,
		MigrationRateLimit:       100, // 100 MB/s
		EnablePrediction:         false,
		PredictionUpdateInterval: 24 * time.Hour,
		TaskTimeout:              2 * time.Hour,
		MaxRetryCount:            3,
		ScheduleStrategy:         ScheduleStrategyPriority,
	}
}

// ScheduledTask 调度任务
type ScheduledTask struct {
	ID string `json:"id"`

	// 任务类型
	Type MigrationType `json:"type"`

	// 关联策略ID
	PolicyID string `json:"policyId"`

	// 状态
	Status TaskStatus `json:"status"`

	// 优先级（越大越高）
	Priority int `json:"priority"`

	// 创建时间
	CreatedAt time.Time `json:"createdAt"`

	// 调度时间
	ScheduledAt time.Time `json:"scheduledAt"`

	// 开始时间
	StartedAt time.Time `json:"startedAt"`

	// 完成时间
	CompletedAt time.Time `json:"completedAt"`

	// 文件列表
	Files []MigrationFileInfo `json:"files"`

	// 统计
	TotalFiles    int64 `json:"totalFiles"`
	TotalBytes    int64 `json:"totalBytes"`
	ProcessedFiles int64 `json:"processedFiles"`
	ProcessedBytes int64 `json:"processedBytes"`
	FailedFiles   int64 `json:"failedFiles"`

	// 重试次数
	RetryCount int `json:"retryCount"`

	// 错误信息
	Errors []TaskError `json:"errors"`

	// 预测得分（智能调度时使用）
	PredictionScore float64 `json:"predictionScore"`
}

// MigrationType 迁移类型
type MigrationType string

const (
	// MigrationTypePromote 提升（HDD → SSD）
	MigrationTypePromote MigrationType = "promote"

	// MigrationTypeDemote 降级（SSD → HDD）
	MigrationTypeDemote MigrationType = "demote"

	// MigrationTypeArchive 归档（HDD → Cloud）
	MigrationTypeArchive MigrationType = "archive"

	// MigrationTypePin 固定（锁定到指定层）
	MigrationTypePin MigrationType = "pin"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusScheduled  TaskStatus = "scheduled"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusPartial    TaskStatus = "partial"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusRetrying   TaskStatus = "retrying"
)

// MigrationFileInfo 迁移文件信息
type MigrationFileInfo struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"modTime"`
	AccessTime time.Time `json:"accessTime"`
	AccessCount int64    `json:"accessCount"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}

// TaskError 任务错误
type TaskError struct {
	Path    string    `json:"path"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// SchedulerStats 调度器统计
type SchedulerStats struct {
	// 任务统计
	TotalTasks     int64 `json:"totalTasks"`
	CompletedTasks int64 `json:"completedTasks"`
	FailedTasks    int64 `json:"failedTasks"`

	// 数据统计
	PromotedFiles int64 `json:"promotedFiles"`
	PromotedBytes int64 `json:"promotedBytes"`
	DemotedFiles  int64 `json:"demotedFiles"`
	DemotedBytes  int64 `json:"demotedBytes"`

	// 时间统计
	TotalPromoteTime time.Duration `json:"totalPromoteTime"`
	TotalDemoteTime  time.Duration `json:"totalDemoteTime"`

	// 效率统计
	AveragePromoteSpeed float64 `json:"averagePromoteSpeed"` // MB/s
	AverageDemoteSpeed  float64 `json:"averageDemoteSpeed"`  // MB/s

	// 最后更新时间
	LastUpdate time.Time `json:"lastUpdate"`
}

// NewTierScheduler 创建分层存储调度器
func NewTierScheduler(config SchedulerConfig) *TierScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &TierScheduler{
		config:     config,
		taskQueue:  make([]*ScheduledTask, 0),
		ctx:        ctx,
		cancel:     cancel,
		logger:     slog.Default(),
	}
}

// SetHotColdManager 设置热冷池管理器
func (s *TierScheduler) SetHotColdManager(manager *HotColdManager) {
	s.mu.Lock()
	s.hotColdManager = manager
	s.mu.Unlock()
}

// SetPolicyManager 设置迁移策略管理器
func (s *TierScheduler) SetPolicyManager(manager *MigrationPolicyManager) {
	s.mu.Lock()
	s.policyManager = manager
	s.mu.Unlock()
}

// Initialize 初始化调度器
func (s *TierScheduler) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 初始化统计
	s.stats = SchedulerStats{
		LastUpdate: time.Now(),
	}

	s.logger.Info("分层存储调度器初始化完成",
		"checkInterval", s.config.CheckInterval,
		"maxConcurrent", s.config.MaxConcurrentTasks,
		"strategy", s.config.ScheduleStrategy,
	)

	return nil
}

// Start 启动调度器
func (s *TierScheduler) Start() error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	// 启动调度引擎
	go s.runSchedulerEngine()

	// 启动任务执行器
	go s.runTaskExecutor()

	// 启动统计收集器
	go s.runStatsCollector()

	s.logger.Info("分层存储调度器已启动")

	return nil
}

// Stop 停止调度器
func (s *TierScheduler) Stop() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	s.cancel()

	// 取消所有待执行任务
	s.cancelPendingTasks()

	s.logger.Info("分层存储调度器已停止")
}

// cancelPendingTasks 取消待执行任务
func (s *TierScheduler) cancelPendingTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.taskQueue {
		if task.Status == TaskStatusPending || task.Status == TaskStatusScheduled {
			task.Status = TaskStatusCancelled
			task.CompletedAt = time.Now()
		}
	}
}

// ==================== 调度引擎 ====================

// runSchedulerEngine 运行调度引擎
func (s *TierScheduler) runSchedulerEngine() {
	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.evaluateAndSchedule()
		}
	}
}

// evaluateAndSchedule 评估并调度迁移任务
func (s *TierScheduler) evaluateAndSchedule() {
	s.mu.RLock()
	hotColdManager := s.hotColdManager
	policyManager := s.policyManager
	s.mu.RUnlock()

	if hotColdManager == nil || policyManager == nil {
		return
	}

	// 获取活跃策略
	activePolicies := policyManager.GetActivePolicies()
	if len(activePolicies) == 0 {
		return
	}

	// 根据调度策略生成任务
	switch s.config.ScheduleStrategy {
	case ScheduleStrategyPriority:
		s.scheduleByPriority(activePolicies, hotColdManager)
	case ScheduleStrategyBalance:
		s.scheduleByBalance(activePolicies, hotColdManager)
	case ScheduleStrategySpace:
		s.scheduleBySpace(activePolicies, hotColdManager)
	case ScheduleStrategyTime:
		s.scheduleByTime(activePolicies, hotColdManager)
	}
}

// scheduleByPriority 优先级调度
func (s *TierScheduler) scheduleByPriority(policies []*MigrationPolicy, manager *HotColdManager) {
	// 先调度热数据提升（高优先级）
	for _, policy := range policies {
		if policy.Type == PolicyTypePromote && policy.Enabled {
			task := s.createPromoteTask(policy, manager)
			if task != nil {
				task.Priority = 100 // 高优先级
				s.addTask(task)
			}
			break // 只创建一个提升任务
		}
	}

	// 再调度冷数据降级（低优先级）
	for _, policy := range policies {
		if policy.Type == PolicyTypeDemote && policy.Enabled {
			task := s.createDemoteTask(policy, manager)
			if task != nil {
				task.Priority = 50 // 低优先级
				s.addTask(task)
			}
			break
		}
	}
}

// scheduleByBalance 平衡调度
func (s *TierScheduler) scheduleByBalance(policies []*MigrationPolicy, manager *HotColdManager) {
	// 交替创建提升和降级任务
	promoteTaskCreated := false
	demoteTaskCreated := false

	for _, policy := range policies {
		if policy.Enabled {
			if policy.Type == PolicyTypePromote && !promoteTaskCreated {
				task := s.createPromoteTask(policy, manager)
				if task != nil {
					task.Priority = 70
					s.addTask(task)
					promoteTaskCreated = true
				}
			} else if policy.Type == PolicyTypeDemote && !demoteTaskCreated {
				task := s.createDemoteTask(policy, manager)
				if task != nil {
					task.Priority = 60
					s.addTask(task)
					demoteTaskCreated = true
				}
			}
		}
	}
}

// scheduleBySpace 空间优先调度
func (s *TierScheduler) scheduleBySpace(policies []*MigrationPolicy, manager *HotColdManager) {
	// 获取空间状态
	ssdStats, _ := manager.GetHotPoolStats()
	hddStats, _ := manager.GetColdPoolStats()

	// SSD空间压力大时，优先降级
	ssdPressure := float64(ssdStats.UsedPercent) / float64(100 - s.config.SSDSpaceReservePercent)
	hddPressure := float64(hddStats.UsedPercent) / 100.0

	for _, policy := range policies {
		if policy.Enabled {
			if policy.Type == PolicyTypeDemote && ssdPressure > 0.8 {
				// SSD压力大，优先降级
				task := s.createDemoteTask(policy, manager)
				if task != nil {
					task.Priority = 100
					s.addTask(task)
				}
			} else if policy.Type == PolicyTypePromote && ssdPressure < 0.5 && hddPressure < 0.9 {
				// SSD空间充足，可以提升
				task := s.createPromoteTask(policy, manager)
				if task != nil {
					task.Priority = 80
					s.addTask(task)
				}
			}
		}
	}
}

// scheduleByTime 时间窗口调度
func (s *TierScheduler) scheduleByTime(policies []*MigrationPolicy, manager *HotColdManager) {
	now := time.Now()

	for _, policy := range policies {
		if policy.Enabled && policy.Schedule != "" {
			// 检查是否在调度窗口内
			if policy.NextRun.IsZero() || now.After(policy.NextRun) || now.Equal(policy.NextRun) {
				var task *ScheduledTask
				if policy.Type == PolicyTypePromote {
					task = s.createPromoteTask(policy, manager)
				} else if policy.Type == PolicyTypeDemote {
					task = s.createDemoteTask(policy, manager)
				}

				if task != nil {
					task.Priority = policy.Priority
					task.ScheduledAt = now
					s.addTask(task)

					// 更新下次运行时间
					s.mu.Lock()
					if s.policyManager != nil {
						s.policyManager.UpdatePolicyNextRun(policy.ID, now.Add(s.config.CheckInterval))
					}
					s.mu.Unlock()
				}
			}
		}
	}
}

// createPromoteTask 创建热数据提升任务
func (s *TierScheduler) createPromoteTask(policy *MigrationPolicy, manager *HotColdManager) *ScheduledTask {
	// 获取冷池中的热数据候选
	hotCandidates := manager.GetHotCandidates(s.config.HotAccessThreshold)
	if len(hotCandidates) == 0 {
		return nil
	}

	// 获取热池可用空间
	ssdStats, err := manager.GetHotPoolStats()
	if err != nil {
		return nil
	}

	// 计算可用空间（扣除保留空间）
	reservedSpace := ssdStats.Capacity * int64(s.config.SSDSpaceReservePercent) / 100
	availableSpace := ssdStats.Capacity - ssdStats.Used - reservedSpace

	if availableSpace <= 0 {
		s.logger.Warn("SSD热池空间不足，无法提升热数据")
		return nil
	}

	// 筛选可迁移文件
	var files []MigrationFileInfo
	var totalSize int64

	for _, candidate := range hotCandidates {
		if totalSize + candidate.Size > availableSpace {
			break // 空间不足
		}

		files = append(files, MigrationFileInfo{
			Path:        candidate.Path,
			Size:        candidate.Size,
			ModTime:     candidate.ModTime,
			AccessTime:  candidate.AccessTime,
			AccessCount: candidate.AccessCount,
			Status:      "pending",
		})
		totalSize += candidate.Size
	}

	if len(files) == 0 {
		return nil
	}

	task := &ScheduledTask{
		ID:          "task_" + uuid.New().String()[:8],
		Type:        MigrationTypePromote,
		PolicyID:    policy.ID,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
		Files:       files,
		TotalFiles:  int64(len(files)),
		TotalBytes:  totalSize,
	}

	return task
}

// createDemoteTask 创建冷数据降级任务
func (s *TierScheduler) createDemoteTask(policy *MigrationPolicy, manager *HotColdManager) *ScheduledTask {
	// 获取热池中的冷数据候选
	coldCandidates := manager.GetColdCandidates(s.config.ColdAgeDays)
	if len(coldCandidates) == 0 {
		return nil
	}

	// 创建迁移文件列表
	var files []MigrationFileInfo
	var totalSize int64

	for _, candidate := range coldCandidates {
		files = append(files, MigrationFileInfo{
			Path:        candidate.Path,
			Size:        candidate.Size,
			ModTime:     candidate.ModTime,
			AccessTime:  candidate.AccessTime,
			AccessCount: candidate.AccessCount,
			Status:      "pending",
		})
		totalSize += candidate.Size
	}

	if len(files) == 0 {
		return nil
	}

	task := &ScheduledTask{
		ID:          "task_" + uuid.New().String()[:8],
		Type:        MigrationTypeDemote,
		PolicyID:    policy.ID,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
		Files:       files,
		TotalFiles:  int64(len(files)),
		TotalBytes:  totalSize,
	}

	return task
}

// addTask 添加任务到队列
func (s *TierScheduler) addTask(task *ScheduledTask) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 按优先级插入队列
	insertIndex := len(s.taskQueue)
	for i, existingTask := range s.taskQueue {
		if task.Priority > existingTask.Priority {
			insertIndex = i
			break
		}
	}

	s.taskQueue = append(s.taskQueue[:insertIndex], append([]*ScheduledTask{task}, s.taskQueue[insertIndex:]...)...)
	s.stats.TotalTasks++

	s.logger.Info("调度任务已创建",
		"taskId", task.ID,
		"type", task.Type,
		"files", task.TotalFiles,
		"bytes", task.TotalBytes,
		"priority", task.Priority,
	)
}

// ==================== 任务执行器 ====================

// runTaskExecutor 运行任务执行器
func (s *TierScheduler) runTaskExecutor() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.executeNextTask()
		}
	}
}

// executeNextTask 执行下一个任务
func (s *TierScheduler) executeNextTask() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查并发限制
	runningCount := 0
	for _, task := range s.taskQueue {
		if task.Status == TaskStatusRunning {
			runningCount++
		}
	}

	if runningCount >= s.config.MaxConcurrentTasks {
		return
	}

	// 获取下一个待执行任务
	var nextTask *ScheduledTask
	for _, task := range s.taskQueue {
		if task.Status == TaskStatusPending || task.Status == TaskStatusScheduled {
			nextTask = task
			break
		}
	}

	if nextTask == nil {
		return
	}

	// 异步执行任务
	nextTask.Status = TaskStatusRunning
	nextTask.StartedAt = time.Now()

	go s.executeTask(nextTask)
}

// executeTask 执行迁移任务
func (s *TierScheduler) executeTask(task *ScheduledTask) {
	ctx, cancel := context.WithTimeout(s.ctx, s.config.TaskTimeout)
	defer cancel()

	s.mu.RLock()
	manager := s.hotColdManager
	s.mu.RUnlock()

	if manager == nil {
		s.markTaskFailed(task, "热冷池管理器未初始化")
		return
	}

	var err error
	switch task.Type {
	case MigrationTypePromote:
		err = s.executePromote(ctx, task, manager)
	case MigrationTypeDemote:
		err = s.executeDemote(ctx, task, manager)
	}

	if err != nil {
		s.handleTaskError(task, err)
	} else {
		s.markTaskCompleted(task)
	}
}

// executePromote 执行热数据提升
func (s *TierScheduler) executePromote(ctx context.Context, task *ScheduledTask, manager *HotColdManager) error {
	for i := range task.Files {
		file := &task.Files[i]

		// 检查上下文
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 执行迁移
		err := manager.PromoteFile(file.Path)
		if err != nil {
			file.Status = "failed"
			file.Error = err.Error()
			task.FailedFiles++
			task.Errors = append(task.Errors, TaskError{
				Path:    file.Path,
				Message: err.Error(),
				Time:    time.Now(),
			})
		} else {
			file.Status = "completed"
		}

		// 更新进度
		s.mu.Lock()
		task.ProcessedFiles++
		task.ProcessedBytes += file.Size
		s.mu.Unlock()
	}

	return nil
}

// executeDemote 执行冷数据降级
func (s *TierScheduler) executeDemote(ctx context.Context, task *ScheduledTask, manager *HotColdManager) error {
	for i := range task.Files {
		file := &task.Files[i]

		// 检查上下文
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 执行迁移
		err := manager.DemoteFile(file.Path)
		if err != nil {
			file.Status = "failed"
			file.Error = err.Error()
			task.FailedFiles++
			task.Errors = append(task.Errors, TaskError{
				Path:    file.Path,
				Message: err.Error(),
				Time:    time.Now(),
			})
		} else {
			file.Status = "completed"
		}

		// 更新进度
		s.mu.Lock()
		task.ProcessedFiles++
		task.ProcessedBytes += file.Size
		s.mu.Unlock()
	}

	return nil
}

// handleTaskError 处理任务错误
func (s *TierScheduler) handleTaskError(task *ScheduledTask, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.RetryCount++
	if task.RetryCount < s.config.MaxRetryCount {
		task.Status = TaskStatusRetrying
		task.Errors = append(task.Errors, TaskError{
			Path:    "",
			Message: "任务失败，准备重试: " + err.Error(),
			Time:    time.Now(),
		})
		s.logger.Warn("任务失败，准备重试",
			"taskId", task.ID,
			"retryCount", task.RetryCount,
			"error", err,
		)
	} else {
		s.markTaskFailedLocked(task, err.Error())
	}
}

// markTaskFailed 标记任务失败
func (s *TierScheduler) markTaskFailed(task *ScheduledTask, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markTaskFailedLocked(task, message)
}

// markTaskFailedLocked 标记任务失败（已锁定）
func (s *TierScheduler) markTaskFailedLocked(task *ScheduledTask, message string) {
	task.Status = TaskStatusFailed
	task.CompletedAt = time.Now()
	task.Errors = append(task.Errors, TaskError{
		Message: message,
		Time:    time.Now(),
	})
	s.stats.FailedTasks++

	s.logger.Error("任务执行失败",
		"taskId", task.ID,
		"type", task.Type,
		"failedFiles", task.FailedFiles,
	)
}

// markTaskCompleted 标记任务完成
func (s *TierScheduler) markTaskCompleted(task *ScheduledTask) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.CompletedAt = time.Now()

	if task.FailedFiles == 0 {
		task.Status = TaskStatusCompleted
	} else if task.FailedFiles < task.TotalFiles {
		task.Status = TaskStatusPartial
	} else {
		task.Status = TaskStatusFailed
	}

	s.stats.CompletedTasks++

	// 更新迁移统计
	if task.Type == MigrationTypePromote {
		s.stats.PromotedFiles += task.ProcessedFiles
		s.stats.PromotedBytes += task.ProcessedBytes
		s.stats.TotalPromoteTime += task.CompletedAt.Sub(task.StartedAt)
	} else if task.Type == MigrationTypeDemote {
		s.stats.DemotedFiles += task.ProcessedFiles
		s.stats.DemotedBytes += task.ProcessedBytes
		s.stats.TotalDemoteTime += task.CompletedAt.Sub(task.StartedAt)
	}

	s.logger.Info("任务执行完成",
		"taskId", task.ID,
		"type", task.Type,
		"status", task.Status,
		"processedFiles", task.ProcessedFiles,
		"processedBytes", task.ProcessedBytes,
	)

	// 回调
	if s.onTaskComplete != nil {
		s.onTaskComplete(task)
	}
}

// ==================== 统计收集器 ====================

// runStatsCollector 运行统计收集器
func (s *TierScheduler) runStatsCollector() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.updateStats()
		}
	}
}

// updateStats 更新统计
func (s *TierScheduler) updateStats() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 计算平均速度
	if s.stats.PromotedBytes > 0 && s.stats.TotalPromoteTime > 0 {
		s.stats.AveragePromoteSpeed = float64(s.stats.PromotedBytes) / float64(s.stats.TotalPromoteTime.Seconds()) / 1024 / 1024
	}
	if s.stats.DemotedBytes > 0 && s.stats.TotalDemoteTime > 0 {
		s.stats.AverageDemoteSpeed = float64(s.stats.DemotedBytes) / float64(s.stats.TotalDemoteTime.Seconds()) / 1024 / 1024
	}

	s.stats.LastUpdate = time.Now()
}

// ==================== 公共API ====================

// GetTask 获取任务
func (s *TierScheduler) GetTask(id string) (*ScheduledTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, task := range s.taskQueue {
		if task.ID == id {
			return task, nil
		}
	}

	return nil, ErrTaskNotFound
}

// ListTasks 列出任务
func (s *TierScheduler) ListTasks(limit int) []*ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ScheduledTask, 0, limit)
	for _, task := range s.taskQueue {
		result = append(result, task)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// CancelTask 取消任务
func (s *TierScheduler) CancelTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.taskQueue {
		if task.ID == id {
			if task.Status == TaskStatusRunning {
				return ErrTaskRunning
			}
			task.Status = TaskStatusCancelled
			task.CompletedAt = time.Now()
			return nil
		}
	}

	return ErrTaskNotFound
}

// GetStats 获取统计
func (s *TierScheduler) GetStats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// GetQueueLength 获取队列长度
func (s *TierScheduler) GetQueueLength() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, task := range s.taskQueue {
		if task.Status == TaskStatusPending || task.Status == TaskStatusScheduled {
			count++
		}
	}
	return count
}

// SetTaskCompleteCallback 设置任务完成回调
func (s *TierScheduler) SetTaskCompleteCallback(callback func(task *ScheduledTask)) {
	s.mu.Lock()
	s.onTaskComplete = callback
	s.mu.Unlock()
}

// ForceSchedule 强制调度（手动触发）
func (s *TierScheduler) ForceSchedule(migrationType MigrationType) (*ScheduledTask, error) {
	s.mu.RLock()
	manager := s.hotColdManager
	policyManager := s.policyManager
	s.mu.RUnlock()

	if manager == nil {
		return nil, ErrManagerNotInitialized
	}

	// 创建临时策略
	tempPolicy := &MigrationPolicy{
		ID:      "manual_" + uuid.New().String()[:8],
		Enabled: true,
	}

	var task *ScheduledTask
	if migrationType == MigrationTypePromote {
		tempPolicy.Type = PolicyTypePromote
		task = s.createPromoteTask(tempPolicy, manager)
	} else if migrationType == MigrationTypeDemote {
		tempPolicy.Type = PolicyTypeDemote
		task = s.createDemoteTask(tempPolicy, manager)
	}

	if task == nil {
		return nil, ErrNoCandidates
	}

	task.Priority = 200 // 手动触发最高优先级
	s.addTask(task)

	return task, nil
}

// 错误定义
var (
	ErrTaskNotFound         = &SchedulerError{Message: "任务不存在"}
	ErrTaskRunning          = &SchedulerError{Message: "任务正在运行，无法取消"}
	ErrManagerNotInitialized = &SchedulerError{Message: "管理器未初始化"}
	ErrNoCandidates         = &SchedulerError{Message: "没有可迁移的数据"}
)

// SchedulerError 调度器错误
type SchedulerError struct {
	Message string
}

func (e *SchedulerError) Error() string {
	return e.Message
}