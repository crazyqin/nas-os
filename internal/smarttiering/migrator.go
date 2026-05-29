package smarttiering

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Migrator 数据迁移引擎
// 根据访问模式自动迁移冷热数据
type Migrator struct {
	mu        sync.RWMutex
	config    MigratorConfig
	logger    *zap.Logger
	predictor *Predictor

	// 迁移任务队列
	queue     chan *MigrationTask
	events    []MigrationEvent
	eventMu   sync.RWMutex

	// 状态
	running   bool
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// MigrationTask 迁移任务
type MigrationTask struct {
	FilePath string
	FromTier StorageTier
	ToTier   StorageTier
	FileSize int64
	Reason   string
}

// NewMigrator 创建迁移引擎
func NewMigrator(config MigratorConfig, predictor *Predictor, logger *zap.Logger) *Migrator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Migrator{
		config:    config,
		logger:    logger,
		predictor: predictor,
		queue:     make(chan *MigrationTask, config.BatchSize*2),
		events:    make([]MigrationEvent, 0),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动迁移引擎
func (m *Migrator) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("migrator already running")
	}
	m.running = true
	m.mu.Unlock()

	// 启动工作者
	for i := 0; i < m.config.MaxConcurrent; i++ {
		m.wg.Add(1)
		go m.worker(ctx, i)
	}

	// 启动调度器
	m.wg.Add(1)
	go m.scheduler(ctx)

	m.logger.Info("migrator started",
		zap.Int("workers", m.config.MaxConcurrent),
		zap.Int("batch_size", m.config.BatchSize))
	return nil
}

// Stop 停止迁移引擎
func (m *Migrator) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	m.mu.Unlock()

	m.wg.Wait()
	m.logger.Info("migrator stopped")
}

// scheduler 定时检查并生成迁移任务
func (m *Migrator) scheduler(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Duration(m.config.MigrationIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluateAndEnqueue(ctx)
		}
	}
}

// evaluateAndEnqueue 评估文件并生成迁移任务
func (m *Migrator) evaluateAndEnqueue(ctx context.Context) {
	files := m.predictor.GetAllFiles()

	enqueued := 0
	for _, meta := range files {
		if ctx.Err() != nil {
			return
		}
		if enqueued >= m.config.BatchSize {
			break
		}

		// 检查空闲时间
		if !meta.AccessedAt.IsZero() {
			idleHours := time.Since(meta.AccessedAt).Hours()
			if idleHours < m.config.MinIdleHours {
				continue // 最近有访问，跳过
			}
		}

		targetTier, score := m.predictor.PredictTier(meta.Path, m.config)
		if targetTier == meta.CurrentTier {
			continue // 已在正确层级
		}

		// 检查是否需要降级（冷数据）或升级（热数据）
		needsMigration := false
		reason := ""
		if targetTier > meta.CurrentTier && score < m.thresholdForTier(meta.CurrentTier) {
			needsMigration = true
			reason = fmt.Sprintf("heat score %.1f below threshold for %s tier", score, meta.CurrentTier)
		} else if targetTier < meta.CurrentTier && score >= m.thresholdForTier(targetTier) {
			needsMigration = true
			reason = fmt.Sprintf("heat score %.1f above threshold for %s tier", score, targetTier)
		}

		if !needsMigration {
			continue
		}

		task := &MigrationTask{
			FilePath: meta.Path,
			FromTier: meta.CurrentTier,
			ToTier:   targetTier,
			FileSize: meta.Size,
			Reason:   reason,
		}

		select {
		case m.queue <- task:
			enqueued++
		default:
			m.logger.Warn("migration queue full, skipping", zap.String("path", meta.Path))
		}
	}

	if enqueued > 0 {
		m.logger.Info("migration tasks enqueued", zap.Int("count", enqueued))
	}
}

// thresholdForTier 获取层级阈值
func (m *Migrator) thresholdForTier(tier StorageTier) float64 {
	switch tier {
	case TierHot:
		return m.config.HotThreshold
	case TierWarm:
		return m.config.WarmThreshold
	case TierCold:
		return m.config.ColdThreshold
	case TierArchive:
		return 0
	default:
		return 0
	}
}

// worker 迁移工作者
func (m *Migrator) worker(ctx context.Context, id int) {
	defer m.wg.Done()
	m.logger.Debug("migration worker started", zap.Int("worker", id))

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case task, ok := <-m.queue:
			if !ok {
				return
			}
			m.executeMigration(ctx, task)
		}
	}
}

// executeMigration 执行单个迁移任务
func (m *Migrator) executeMigration(ctx context.Context, task *MigrationTask) {
	event := MigrationEvent{
		ID:        uuid.New().String(),
		FilePath:  task.FilePath,
		FromTier:  task.FromTier,
		ToTier:    task.ToTier,
		FileSize:  task.FileSize,
		Reason:    task.Reason,
		Status:    "running",
		StartedAt: time.Now(),
	}

	m.recordEvent(event)

	if m.config.DryRun {
		m.logger.Info("dry run: would migrate",
			zap.String("path", task.FilePath),
			zap.String("from", task.FromTier.String()),
			zap.String("to", task.ToTier.String()))
		event.Status = "completed"
		now := time.Now()
		event.CompletedAt = &now
		event.Duration = now.Sub(event.StartedAt)
		m.updateEvent(event)
		return
	}

	// 执行实际迁移
	err := m.doMigrate(ctx, task)
	if err != nil {
		event.Status = "failed"
		event.Error = err.Error()
		m.logger.Error("migration failed",
			zap.String("path", task.FilePath),
			zap.Error(err))
	} else {
		event.Status = "completed"
		m.logger.Info("migration completed",
			zap.String("path", task.FilePath),
			zap.String("from", task.FromTier.String()),
			zap.String("to", task.ToTier.String()))
	}

	now := time.Now()
	event.CompletedAt = &now
	event.Duration = now.Sub(event.StartedAt)
	m.updateEvent(event)
}

// doMigrate 执行实际的数据迁移
// 这里是迁移的核心逻辑，支持跨存储介质的数据移动
func (m *Migrator) doMigrate(ctx context.Context, task *MigrationTask) error {
	// 模拟迁移过程
	// 实际实现中，这里会：
	// 1. 在目标层级创建文件副本
	// 2. 验证数据完整性（checksum）
	// 3. 原子性切换文件位置
	// 4. 更新文件元数据中的 CurrentTier

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.stopCh:
		return fmt.Errorf("migrator stopped")
	default:
	}

	// 更新预测器中的文件层级
	m.predictor.mu.Lock()
	if meta, ok := m.predictor.files[task.FilePath]; ok {
		meta.CurrentTier = task.ToTier
	}
	m.predictor.mu.Unlock()

	return nil
}

// recordEvent 记录迁移事件
func (m *Migrator) recordEvent(event MigrationEvent) {
	m.eventMu.Lock()
	defer m.eventMu.Unlock()
	m.events = append(m.events, event)
}

// updateEvent 更新迁移事件
func (m *Migrator) updateEvent(event MigrationEvent) {
	m.eventMu.Lock()
	defer m.eventMu.Unlock()
	for i, e := range m.events {
		if e.ID == event.ID {
			m.events[i] = event
			return
		}
	}
}

// GetMigrationEvents 获取迁移事件列表
func (m *Migrator) GetMigrationEvents(limit int) []MigrationEvent {
	m.eventMu.RLock()
	defer m.eventMu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}
	start := len(m.events) - limit
	result := make([]MigrationEvent, limit)
	copy(result, m.events[start:])
	return result
}

// GetQueueSize 获取队列中待迁移任务数
func (m *Migrator) GetQueueSize() int {
	return len(m.queue)
}

// IsRunning 检查是否运行中
func (m *Migrator) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// UpdateConfig 更新配置
func (m *Migrator) UpdateConfig(config MigratorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig 获取配置
func (m *Migrator) GetConfig() MigratorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// ForceMigrate 强制迁移指定文件
func (m *Migrator) ForceMigrate(ctx context.Context, path string, fromTier, toTier StorageTier, fileSize int64) error {
	task := &MigrationTask{
		FilePath: path,
		FromTier: fromTier,
		ToTier:   toTier,
		FileSize: fileSize,
		Reason:   "manual migration",
	}

	select {
	case m.queue <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
