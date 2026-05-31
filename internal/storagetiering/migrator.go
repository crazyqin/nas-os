package storagetiering

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// StorageBackend 存储后端接口（用于测试和解耦）
type StorageBackend interface {
	// ReadFile 读取文件内容（用于 checksum 计算）
	ReadFile(path string) ([]byte, error)
	// CopyFile 复制文件到目标层级
	CopyFile(path string, fromTier, toTier Tier) error
	// DeleteFile 从源层级删除文件
	DeleteFile(path string, tier Tier) error
	// FileExists 检查文件是否存在
	FileExists(path string, tier Tier) bool
}

// Migrator 数据迁移执行器
// 支持暂停/恢复/取消，迁移前后数据校验
type Migrator struct {
	mu       sync.RWMutex
	config   MigratorConfig
	logger   *zap.Logger
	backend  StorageBackend

	// 任务管理
	tasks    map[string]*MigrationTask // id -> task
	history  []MigrationHistoryItem
	eventCh  chan *MigrationTask

	// 控制
	running   bool
	stopCh    chan struct{}
	pauseCh   chan struct{}
	resumeCh  chan struct{}
	paused    bool
	wg        sync.WaitGroup

	// 统计
	totalMigrations int64
}

// NewMigrator 创建迁移执行器
func NewMigrator(config MigratorConfig, backend StorageBackend, logger *zap.Logger) *Migrator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Migrator{
		config:   config,
		logger:   logger,
		backend:  backend,
		tasks:    make(map[string]*MigrationTask),
		history:  make([]MigrationHistoryItem, 0, 1000),
		eventCh:  make(chan *MigrationTask, 100),
		stopCh:   make(chan struct{}),
		pauseCh:  make(chan struct{}),
		resumeCh: make(chan struct{}),
	}
}

// Start 启动迁移器
func (m *Migrator) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("migrator already running")
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	// 启动工作者
	for i := 0; i < m.config.MaxConcurrent; i++ {
		m.wg.Add(1)
		go m.worker(ctx, i)
	}

	m.logger.Info("migrator started", zap.Int("workers", m.config.MaxConcurrent))
	return nil
}

// Stop 停止迁移器
func (m *Migrator) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	// 如果暂停中，也关闭暂停信号
	if m.paused {
		close(m.resumeCh)
	}
	m.mu.Unlock()

	m.wg.Wait()
	m.logger.Info("migrator stopped")
}

// Submit 提交迁移任务
func (m *Migrator) Submit(task *MigrationTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("migrator not running")
	}

	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.State = StatePending
	task.CreatedAt = time.Now()

	m.tasks[task.ID] = task

	select {
	case m.eventCh <- task:
	default:
		m.logger.Warn("task queue full, task will be picked up later", zap.String("id", task.ID))
	}

	m.logger.Info("task submitted", zap.String("id", task.ID), zap.String("file", task.FileFilePath()))
	return nil
}

// SubmitBatch 批量提交任务
func (m *Migrator) SubmitBatch(tasks []*MigrationTask) int {
	submitted := 0
	for _, t := range tasks {
		if err := m.Submit(t); err != nil {
			m.logger.Error("failed to submit task", zap.Error(err))
			continue
		}
		submitted++
	}
	return submitted
}

// Pause 暂停所有迁移
func (m *Migrator) Pause() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("migrator not running")
	}
	if m.paused {
		return fmt.Errorf("already paused")
	}

	m.paused = true
	m.resumeCh = make(chan struct{})
	m.logger.Info("migrator paused")
	return nil
}

// Resume 恢复迁移
func (m *Migrator) Resume() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("migrator not running")
	}
	if !m.paused {
		return fmt.Errorf("not paused")
	}

	m.paused = false
	close(m.resumeCh)
	m.logger.Info("migrator resumed")
	return nil
}

// CancelTask 取消单个任务
func (m *Migrator) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.State == StateCompleted || task.State == StateCancelled {
		return fmt.Errorf("task already in terminal state: %s", task.State)
	}

	task.State = StateCancelled
	m.logger.Info("task cancelled", zap.String("id", taskID))
	return nil
}

// GetTask 获取任务状态
func (m *Migrator) GetTask(taskID string) (*MigrationTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[taskID]
	if !ok {
		return nil, false
	}
	cp := *task
	return &cp, true
}

// GetHistory 获取迁移历史
func (m *Migrator) GetHistory(limit int) []MigrationHistoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	// 返回最近的记录
	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}
	result := make([]MigrationHistoryItem, limit)
	copy(result, m.history[start:])
	return result
}

// ActiveCount 返回正在执行的任务数
func (m *Migrator) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, t := range m.tasks {
		if t.State == StateRunning {
			count++
		}
	}
	return count
}

// TotalMigrations 返回总迁移次数
func (m *Migrator) TotalMigrations() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalMigrations
}

// EventChannel 返回事件通道
func (m *Migrator) EventChannel() <-chan *MigrationTask {
	return m.eventCh
}

// ============================================================
// Worker
// ============================================================

func (m *Migrator) worker(ctx context.Context, id int) {
	defer m.wg.Done()
	m.logger.Debug("worker started", zap.Int("worker_id", id))

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case task := <-m.eventCh:
			m.processTask(ctx, task, id)
		}
	}
}

func (m *Migrator) processTask(ctx context.Context, task *MigrationTask, workerID int) {
	// 检查暂停状态
	m.mu.RLock()
	if m.paused {
		m.mu.RUnlock()
		select {
		case <-m.resumeCh:
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		}
	} else {
		m.mu.RUnlock()
	}

	// 检查任务状态
	m.mu.RLock()
	if task.State == StateCancelled {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	// 标记运行中
	m.mu.Lock()
	task.State = StateRunning
	now := time.Now()
	task.StartedAt = &now
	m.mu.Unlock()

	m.logger.Info("task started",
		zap.String("id", task.ID),
		zap.String("file", task.FilePath),
		zap.String("from", task.FromTier.String()),
		zap.String("to", task.ToTier.String()),
		zap.Int("worker", workerID))

	// 执行迁移
	err := m.executeMigration(ctx, task)

	m.mu.Lock()
	completedAt := time.Now()
	task.CompletedAt = &completedAt
	if err != nil {
		task.State = StateFailed
		task.Error = err.Error()
		m.logger.Error("task failed", zap.String("id", task.ID), zap.Error(err))
	} else {
		task.State = StateCompleted
		task.Progress = 100
		m.totalMigrations++
		m.logger.Info("task completed", zap.String("id", task.ID))
	}

	// 记录历史
	m.history = append(m.history, MigrationHistoryItem{
		TaskID:    task.ID,
		FilePath:  task.FilePath,
		FromTier:  task.FromTier,
		ToTier:    task.ToTier,
		FileSize:  task.FileSize,
		State:     task.State,
		Reason:    task.Reason,
		Timestamp: completedAt,
	})
	m.mu.Unlock()
}

// executeMigration 执行文件迁移
func (m *Migrator) executeMigration(ctx context.Context, task *MigrationTask) error {
	if m.backend == nil {
		// 模拟模式（测试用）
		return m.simulateMigration(ctx, task)
	}

	// 1. 计算源文件 checksum
	if m.config.VerifyChecksum {
		srcData, err := m.backend.ReadFile(task.FilePath)
		if err != nil {
			return fmt.Errorf("read source: %w", err)
		}
		task.ChecksumSrc = computeChecksum(srcData)
		m.logger.Debug("source checksum", zap.String("checksum", task.ChecksumSrc))
	}

	// 2. 检查暂停
	if err := m.checkPause(ctx); err != nil {
		return err
	}

	// 3. 复制到目标层
	if err := m.backend.CopyFile(task.FilePath, task.FromTier, task.ToTier); err != nil {
		return fmt.Errorf("copy to target: %w", err)
	}
	task.Progress = 50

	// 4. 验证目标文件 checksum
	if m.config.VerifyChecksum && m.backend != nil {
		dstData, err := m.backend.ReadFile(task.FilePath)
		if err != nil {
			return fmt.Errorf("read target: %w", err)
		}
		task.ChecksumDst = computeChecksum(dstData)
		if task.ChecksumSrc != task.ChecksumDst {
			// 校验失败，删除目标
			_ = m.backend.DeleteFile(task.FilePath, task.ToTier)
			return fmt.Errorf("checksum mismatch: src=%s dst=%s", task.ChecksumSrc, task.ChecksumDst)
		}
	}
	task.Progress = 90

	// 5. 检查暂停
	if err := m.checkPause(ctx); err != nil {
		return err
	}

	// 6. 删除源文件
	if err := m.backend.DeleteFile(task.FilePath, task.FromTier); err != nil {
		m.logger.Warn("failed to delete source, keeping target", zap.Error(err))
	}

	task.Progress = 100
	return nil
}

// simulateMigration 模拟迁移（测试用）
func (m *Migrator) simulateMigration(ctx context.Context, task *MigrationTask) error {
	task.ChecksumSrc = "simulated-checksum"
	task.ChecksumDst = "simulated-checksum"
	task.Progress = 100
	return nil
}

// checkPause 检查暂停状态
func (m *Migrator) checkPause(ctx context.Context) error {
	m.mu.RLock()
	if !m.paused {
		m.mu.RUnlock()
		return nil
	}
	resumeCh := m.resumeCh
	m.mu.RUnlock()

	select {
	case <-resumeCh:
		return nil
	case <-m.stopCh:
		return fmt.Errorf("migrator stopped")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ============================================================
// 辅助函数
// ============================================================

// FileFilePath 为了兼容，返回文件路径（别名）
func (t *MigrationTask) FileFilePath() string {
	return t.FilePath
}

// computeChecksum 计算 CRC32 校验和
func computeChecksum(data []byte) string {
	// 简单 CRC32 实现
	var crc uint32 = 0xFFFFFFFF
	for _, b := range data {
		crc ^= uint32(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xEDB88320
			} else {
				crc >>= 1
			}
		}
	}
	crc ^= 0xFFFFFFFF
	return fmt.Sprintf("%08x", crc)
}
