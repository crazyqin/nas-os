// Package storagetiering 迁移调度器
package storagetiering

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scheduler 迁移调度器
type Scheduler struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	config      *StorageTieringConfig
	analyzer    *Analyzer
	tasks       map[string]*MigrationTask
	migrationCh chan *MigrationTask
	stopCh      chan struct{}
	isRunning   bool
	stats       *MigrationStats
}

// NewScheduler 创建迁移调度器
func NewScheduler(logger *zap.Logger, config *StorageTieringConfig, analyzer *Analyzer) *Scheduler {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultStorageTieringConfig()
	}
	if analyzer == nil {
		analyzer = NewAnalyzer(config)
	}

	return &Scheduler{
		logger:      logger,
		config:      config,
		analyzer:    analyzer,
		tasks:       make(map[string]*MigrationTask),
		migrationCh: make(chan *MigrationTask, config.MigrationBatchSize*2),
		stopCh:      make(chan struct{}),
		stats: &MigrationStats{
			ID: generateID(),
		},
	}
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return
	}

	s.isRunning = true
	go s.run()

	s.logger.Info("migration scheduler started")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	s.isRunning = false
	close(s.stopCh)

	s.logger.Info("migration scheduler stopped")
}

// run 调度器主循环
func (s *Scheduler) run() {
	for {
		select {
		case <-s.stopCh:
			return
		case task := <-s.migrationCh:
			s.processMigration(task)
		}
	}
}

// processMigration 处理迁移任务
func (s *Scheduler) processMigration(task *MigrationTask) {
	s.mu.Lock()
	task.State = StateRunning
	startedAt := time.Now()
	task.StartedAt = &startedAt
	s.mu.Unlock()

	s.logger.Info("starting migration",
		zap.String("task_id", task.ID),
		zap.String("file", task.FilePath),
		zap.String("from", task.FromTier.String()),
		zap.String("to", task.ToTier.String()))

	// 模拟迁移过程
	err := s.simulateMigration(task)

	s.mu.Lock()
	defer s.mu.Unlock()

	completedAt := time.Now()
	task.CompletedAt = &completedAt

	if err != nil {
		task.State = StateFailed
		task.Error = err.Error()
		s.stats.FailedCount++
		s.logger.Error("migration failed",
			zap.String("task_id", task.ID),
			zap.Error(err))
	} else {
		task.State = StateCompleted
		task.Progress = 100
		s.stats.SuccessfulCount++
		s.stats.TotalBytesMoved += task.FileSize
		s.logger.Info("migration completed",
			zap.String("task_id", task.ID))
	}

	s.stats.TotalMigrations++
	s.stats.LastMigrationAt = &completedAt
	s.stats.UpdatedAt = time.Now()
}

// simulateMigration 模拟迁移过程
func (s *Scheduler) simulateMigration(task *MigrationTask) error {
	// 模拟迁移延迟（根据文件大小）
	// 实际实现中，这里会调用存储系统的复制/移动API
	task.Progress = 50
	time.Sleep(10 * time.Millisecond) // 模拟IO操作
	task.Progress = 100
	return nil
}

// ScheduleMigration 调度迁移任务
func (s *Scheduler) ScheduleMigration(file *FileMetadata, targetTier TierLevel, ruleID string) (*MigrationTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.config.Enabled {
		return nil, ErrStorageTieringDisabled
	}

	if file.IsPinned {
		s.logger.Debug("skipping pinned file",
			zap.String("file_id", file.ID))
		return nil, ErrFilePinned
	}

	if file.CurrentTier == targetTier {
		return nil, ErrAlreadyInTier
	}

	// 检查文件大小限制
	if file.SizeBytes < s.config.MinFileSizeBytes {
		return nil, ErrFileTooSmall
	}
	if file.SizeBytes > s.config.MaxFileSizeBytes {
		return nil, ErrFileTooLarge
	}

	// 转换 TierLevel 到 Tier
	fromTier := tierLevelToTier(file.CurrentTier)
	toTier := tierLevelToTier(targetTier)

	task := &MigrationTask{
		ID:       generateID(),
		FilePath: file.Path,
		FileSize: file.SizeBytes,
		FromTier: fromTier,
		ToTier:   toTier,
		State:    StatePending,
		Reason:   ruleID,
		CreatedAt: time.Now(),
	}

	s.tasks[task.ID] = task

	// 异步发送到迁移通道
	go func() {
		s.migrationCh <- task
	}()

	s.logger.Info("migration scheduled",
		zap.String("task_id", task.ID),
		zap.String("file", file.Path))

	return task, nil
}

// tierLevelToTier 转换 TierLevel 到 Tier
func tierLevelToTier(level TierLevel) Tier {
	switch level {
	case TierLevelHot:
		return TierSSD
	case TierLevelWarm:
		return TierHDD
	case TierLevelCold:
		return TierCold
	default:
		return TierHDD
	}
}

// GetTask 获取迁移任务
func (s *Scheduler) GetTask(id string) (*MigrationTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有迁移任务
func (s *Scheduler) ListTasks() []*MigrationTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*MigrationTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// ListTasksByState 按状态列出迁移任务
func (s *Scheduler) ListTasksByState(state MigrationState) []*MigrationTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*MigrationTask
	for _, t := range s.tasks {
		if t.State == state {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// CancelTask 取消迁移任务
func (s *Scheduler) CancelTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}

	if task.State == StateCompleted {
		return ErrTaskAlreadyCompleted
	}

	if task.State == StateRunning {
		// 实际实现中需要中断正在进行的迁移
		task.State = StateCancelled
	} else if task.State == StatePending {
		task.State = StateCancelled
	}

	s.stats.CancelledCount++

	s.logger.Info("migration task cancelled",
		zap.String("task_id", id))

	return nil
}

// GetStats 获取迁移统计
func (s *Scheduler) GetStats() *MigrationStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := *s.stats
	return &stats
}

// IsRunning 检查调度器是否运行中
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// GetQueueLength 获取队列长度
func (s *Scheduler) GetQueueLength() int {
	return len(s.migrationCh)
}
