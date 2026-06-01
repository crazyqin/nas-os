package smartdatalifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Migrator 数据迁移器
// 管理热/温/冷数据分层迁移
type Migrator struct {
	config  MigrationConfig
	manager *Manager
	logger  *zap.Logger

	// 运行中的迁移
	runningMigrations sync.Map // map[string]*MigrationTask
}

// NewMigrator 创建迁移器
func NewMigrator(config MigrationConfig, manager *Manager, logger *zap.Logger) *Migrator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Migrator{
		config:  config,
		manager: manager,
		logger:  logger,
	}
}

// Run 执行迁移检查
func (m *Migrator) Run(ctx context.Context) error {
	m.logger.Info("migration check started")

	// 获取活跃数据项
	items := m.manager.ListItems(StageActive, 0, 0)

	migrationsCreated := 0

	for _, item := range items {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 检查是否需要迁移
		targetStage, reason := m.evaluateMigration(item)
		if targetStage == "" {
			continue
		}

		// 创建迁移任务
		task := &MigrationTask{
			ID:          fmt.Sprintf("mig-%s-%d", item.ID, time.Now().UnixNano()),
			SourcePath:  item.Path,
			TargetPath:  m.getTargetPath(item.Path, targetStage),
			SourceStage: item.Stage,
			TargetStage: targetStage,
			Size:        item.Size,
			Reason:      reason,
		}

		m.manager.AddMigrationTask(task)
		migrationsCreated++

		m.logger.Info("migration task created",
			zap.String("task_id", task.ID),
			zap.String("item_id", item.ID),
			zap.String("source_stage", string(item.Stage)),
			zap.String("target_stage", string(targetStage)),
			zap.String("reason", reason))
	}

	// 执行待处理的迁移
	if err := m.executePendingMigrations(ctx); err != nil {
		return err
	}

	m.logger.Info("migration check completed",
		zap.Int("migrations_created", migrationsCreated))

	return nil
}

// evaluateMigration 评估是否需要迁移
func (m *Migrator) evaluateMigration(item *DataItem) (LifecycleStage, string) {
	// 跳过法律冻结
	if item.LegalHold {
		return "", ""
	}

	// 计算空闲天数
	idleDays := time.Since(item.AccessedAt).Hours() / 24

	// 基于空闲时间的迁移策略
	if m.config.ArchiveAfterDays > 0 && idleDays >= float64(m.config.ArchiveAfterDays) {
		return StageArchive, fmt.Sprintf("idle for %.0f days (threshold: %d)", idleDays, m.config.ArchiveAfterDays)
	}

	if m.config.ColdAfterDays > 0 && idleDays >= float64(m.config.ColdAfterDays) {
		return StageCold, fmt.Sprintf("idle for %.0f days (threshold: %d)", idleDays, m.config.ColdAfterDays)
	}

	if m.config.WarmAfterDays > 0 && idleDays >= float64(m.config.WarmAfterDays) {
		return StageWarm, fmt.Sprintf("idle for %.0f days (threshold: %d)", idleDays, m.config.WarmAfterDays)
	}

	return "", ""
}

// getTargetPath 获取目标路径
func (m *Migrator) getTargetPath(sourcePath string, targetStage LifecycleStage) string {
	// 根据目标阶段生成路径
	// 实际实现中应该根据存储配置来确定
	switch targetStage {
	case StageWarm:
		return "/warm" + sourcePath
	case StageCold:
		return "/cold" + sourcePath
	case StageArchive:
		return "/archive" + sourcePath
	default:
		return sourcePath
	}
}

// executePendingMigrations 执行待处理的迁移
func (m *Migrator) executePendingMigrations(ctx context.Context) error {
	pendingTasks := m.manager.ListMigrationTasks(MigrationPending, m.config.BatchSize, 0)

	if len(pendingTasks) == 0 {
		return nil
	}

	// 限制并发数
	semaphore := make(chan struct{}, m.config.MaxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []string

	for _, task := range pendingTasks {
		if ctx.Err() != nil {
			break
		}

		// 检查最小空闲时间
		if m.config.MinIdleDays > 0 {
			item, ok := m.manager.GetItem(task.ID)
			if ok {
				idleDays := time.Since(item.AccessedAt).Hours() / 24
				if idleDays < float64(m.config.MinIdleDays) {
					continue
				}
			}
		}

		wg.Add(1)
		go func(t *MigrationTask) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := m.executeMigration(ctx, t); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("task %s: %v", t.ID, err))
				mu.Unlock()
			}
		}(task)
	}

	wg.Wait()

	if len(errors) > 0 {
		m.logger.Warn("some migrations failed", zap.Strings("errors", errors))
	}

	return nil
}

// executeMigration 执行单个迁移任务
func (m *Migrator) executeMigration(ctx context.Context, task *MigrationTask) error {
	startTime := time.Now()

	// 更新状态为运行中
	m.manager.mu.Lock()
	if t, ok := m.manager.migrationTasks[task.ID]; ok {
		t.Status = MigrationRunning
		t.StartedAt = &startTime
	}
	m.manager.mu.Unlock()

	// 记录运行中的迁移
	m.runningMigrations.Store(task.ID, task)
	defer m.runningMigrations.Delete(task.ID)

	// 执行迁移（这里模拟实际迁移）
	// 实际实现中应该调用存储层进行数据移动
	if err := m.simulateMigration(ctx, task); err != nil {
		// 标记失败
		m.manager.mu.Lock()
		if t, ok := m.manager.migrationTasks[task.ID]; ok {
			t.Status = MigrationFailed
			t.Error = err.Error()
			now := time.Now()
			t.CompletedAt = &now
		}
		m.manager.mu.Unlock()
		return err
	}

	// 更新数据项阶段
	itemID := task.ID // 简化：实际应该从任务中获取关联的数据项ID
	if err := m.manager.UpdateItemStage(itemID, task.TargetStage, "migration"); err != nil {
		m.logger.Warn("failed to update item stage after migration",
			zap.String("task_id", task.ID),
			zap.Error(err))
	}

	// 标记完成
	m.manager.mu.Lock()
	if t, ok := m.manager.migrationTasks[task.ID]; ok {
		t.Status = MigrationCompleted
		now := time.Now()
		t.CompletedAt = &now
	}
	m.manager.mu.Unlock()

	m.logger.Info("migration completed",
		zap.String("task_id", task.ID),
		zap.String("source", string(task.SourceStage)),
		zap.String("target", string(task.TargetStage)),
		zap.Duration("duration", time.Since(startTime)))

	return nil
}

// simulateMigration 模拟迁移过程
func (m *Migrator) simulateMigration(ctx context.Context, task *MigrationTask) error {
	// 模拟迁移延迟
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// GetRunningMigrations 获取运行中的迁移
func (m *Migrator) GetRunningMigrations() []*MigrationTask {
	result := make([]*MigrationTask, 0)
	m.runningMigrations.Range(func(key, value interface{}) bool {
		if task, ok := value.(*MigrationTask); ok {
			result = append(result, task)
		}
		return true
	})
	return result
}

// ForceMigration 强制迁移
func (m *Migrator) ForceMigration(ctx context.Context, itemID string, targetStage LifecycleStage) (*MigrationTask, error) {
	item, ok := m.manager.GetItem(itemID)
	if !ok {
		return nil, fmt.Errorf("item not found: %s", itemID)
	}

	// 法律冻结检查
	if item.LegalHold {
		return nil, fmt.Errorf("item %s is under legal hold", itemID)
	}

	task := &MigrationTask{
		ID:          fmt.Sprintf("force-%s-%d", itemID, time.Now().UnixNano()),
		SourcePath:  item.Path,
		TargetPath:  m.getTargetPath(item.Path, targetStage),
		SourceStage: item.Stage,
		TargetStage: targetStage,
		Size:        item.Size,
		Reason:      "forced migration",
	}

	m.manager.AddMigrationTask(task)

	// 立即执行
	if err := m.executeMigration(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

// GetMigrationStats 获取迁移统计
func (m *Migrator) GetMigrationStats() map[string]interface{} {
	m.manager.mu.RLock()
	defer m.manager.mu.RUnlock()

	stats := map[string]interface{}{
		"pending":   0,
		"running":   0,
		"completed": 0,
		"failed":    0,
		"total":     0,
	}

	for _, task := range m.manager.migrationTasks {
		stats["total"] = stats["total"].(int) + 1
		switch task.Status {
		case MigrationPending:
			stats["pending"] = stats["pending"].(int) + 1
		case MigrationRunning:
			stats["running"] = stats["running"].(int) + 1
		case MigrationCompleted:
			stats["completed"] = stats["completed"].(int) + 1
		case MigrationFailed:
			stats["failed"] = stats["failed"].(int) + 1
		}
	}

	return stats
}
