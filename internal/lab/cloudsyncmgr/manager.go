package cloudsyncmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 多云同步管理器.
type Manager struct {
	mu sync.RWMutex

	configPath string
	logger     *zap.Logger

	// 任务管理
	tasks     map[string]*SyncTask
	providers map[string]CloudProvider

	// 调度器和带宽限制
	scheduler *TaskScheduler
	limiters  map[string]*BandwidthLimiter

	// 事件通道
	events chan SyncEvent
}

// NewManager 创建多云同步管理器.
func NewManager(configPath string, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		configPath: configPath,
		logger:     logger,
		tasks:      make(map[string]*SyncTask),
		providers:  make(map[string]CloudProvider),
		scheduler:  NewTaskScheduler(),
		limiters:   make(map[string]*BandwidthLimiter),
		events:     make(chan SyncEvent, 100),
	}
}

// Initialize 初始化管理器，加载配置.
func (m *Manager) Initialize() error {
	return m.loadConfig()
}

// loadConfig 从配置文件加载任务.
func (m *Manager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 配置文件不存在是正常的
		}
		return fmt.Errorf("读取配置失败: %w", err)
	}

	var configs []SyncConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	for _, cfg := range configs {
		cfg.CreatedAt = time.Now()
		cfg.UpdatedAt = time.Now()
		task := &SyncTask{
			Config: cfg,
			Status: StatusIdle,
		}
		m.tasks[cfg.ID] = task

		// 创建提供商
		provider, err := CreateProvider(cfg.Provider, cfg.ProviderConfig)
		if err != nil {
			m.logger.Warn("创建提供商失败", zap.String("task", cfg.ID), zap.Error(err))
			continue
		}
		m.providers[cfg.ID] = provider

		// 创建带宽限制器
		m.limiters[cfg.ID] = NewBandwidthLimiter(cfg.BandwidthLimit)

		// 注册调度
		if cfg.Enabled {
			m.scheduleTask(task)
		}
	}

	return nil
}

// saveConfig 保存配置到文件.
func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	m.mu.RLock()
	configs := make([]SyncConfig, 0, len(m.tasks))
	for _, task := range m.tasks {
		configs = append(configs, task.Config)
	}
	m.mu.RUnlock()

	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// CreateTask 创建同步任务.
func (m *Manager) CreateTask(config SyncConfig) (*SyncTask, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查名称唯一性
	for _, t := range m.tasks {
		if t.Config.Name == config.Name {
			return nil, fmt.Errorf("任务名称已存在: %s", config.Name)
		}
	}

	// 生成 ID
	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	// 创建提供商
	provider, err := CreateProvider(config.Provider, config.ProviderConfig)
	if err != nil {
		return nil, fmt.Errorf("创建提供商失败: %w", err)
	}

	task := &SyncTask{
		Config: config,
		Status: StatusIdle,
	}

	m.tasks[config.ID] = task
	m.providers[config.ID] = provider
	m.limiters[config.ID] = NewBandwidthLimiter(config.BandwidthLimit)

	// 如果启用，注册调度
	if config.Enabled {
		m.scheduleTaskLocked(task)
	}

	if err := m.saveConfig(); err != nil {
		m.logger.Warn("保存配置失败", zap.Error(err))
	}

	m.emitEvent(task.Config.ID, "created", fmt.Sprintf("任务创建成功: %s", config.Name))
	return task, nil
}

// GetTask 获取同步任务.
func (m *Manager) GetTask(taskID string) (*SyncTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}
	return task, nil
}

// ListTasks 列出所有同步任务.
func (m *Manager) ListTasks() []*SyncTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*SyncTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// UpdateTask 更新同步任务.
func (m *Manager) UpdateTask(taskID string, config SyncConfig) (*SyncTask, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	// 停止旧调度
	m.scheduler.Remove(taskID)

	config.ID = taskID
	config.CreatedAt = task.Config.CreatedAt
	config.UpdatedAt = time.Now()

	// 更新提供商
	provider, err := CreateProvider(config.Provider, config.ProviderConfig)
	if err != nil {
		return nil, fmt.Errorf("创建提供商失败: %w", err)
	}

	task.Config = config
	m.providers[taskID] = provider
	m.limiters[taskID] = NewBandwidthLimiter(config.BandwidthLimit)

	if config.Enabled {
		m.scheduleTaskLocked(task)
	}

	if err := m.saveConfig(); err != nil {
		m.logger.Warn("保存配置失败", zap.Error(err))
	}

	return task, nil
}

// DeleteTask 删除同步任务.
func (m *Manager) DeleteTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[taskID]; !ok {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	m.scheduler.Remove(taskID)
	delete(m.tasks, taskID)
	delete(m.providers, taskID)
	delete(m.limiters, taskID)

	if err := m.saveConfig(); err != nil {
		m.logger.Warn("保存配置失败", zap.Error(err))
	}

	return nil
}

// StartSync 手动触发同步.
func (m *Manager) StartSync(ctx context.Context, taskID string) error {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	provider, pOk := m.providers[taskID]
	limiter := m.limiters[taskID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务不存在: %s", taskID)
	}
	if !pOk {
		return fmt.Errorf("提供商未初始化: %s", taskID)
	}
	if task.Status == StatusSyncing {
		return fmt.Errorf("任务正在同步中: %s", taskID)
	}

	m.mu.Lock()
	task.Status = StatusSyncing
	task.Error = ""
	now := time.Now()
	task.Progress = &SyncProgress{
		StartedAt: now,
	}
	m.mu.Unlock()

	m.emitEvent(taskID, "start", "同步开始")

	// 模拟同步过程 (实际实现需对接 CloudProvider)
	go m.runSync(ctx, task, provider, limiter)

	return nil
}

// runSync 执行同步逻辑.
func (m *Manager) runSync(ctx context.Context, task *SyncTask, provider CloudProvider, limiter *BandwidthLimiter) {
	taskID := task.Config.ID

	// 测试连接
	if err := provider.TestConnection(ctx); err != nil {
		m.mu.Lock()
		task.Status = StatusError
		task.Error = fmt.Sprintf("连接失败: %v", err)
		m.mu.Unlock()
		m.emitEvent(taskID, "error", task.Error)
		return
	}

	// 模拟同步进度更新
	m.mu.Lock()
	task.Progress.TotalFiles = 10
	task.Progress.TotalBytes = 1024 * 1024 * 100 // 100MB
	m.mu.Unlock()

	for i := int64(1); i <= 10; i++ {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			task.Status = StatusPaused
			m.mu.Unlock()
			m.emitEvent(taskID, "paused", "同步已暂停")
			return
		default:
		}

		// 带宽限制
		chunkSize := int64(1024 * 1024 * 10)
		if limiter != nil && limiter.GetRate() > 0 {
			_ = limiter.Acquire(ctx, chunkSize)
		}

		m.mu.Lock()
		task.Progress.SyncedFiles = i
		task.Progress.SyncedBytes = chunkSize * i
		task.Progress.CurrentFile = fmt.Sprintf("file_%d.dat", i)
		task.Progress.TransferRate = float64(chunkSize) / 0.5 // 模拟速率
		m.mu.Unlock()

		m.emitEvent(taskID, "progress", fmt.Sprintf("同步中: %d/10", i))

		// 模拟延迟
		time.Sleep(50 * time.Millisecond)
	}

	// 完成
	m.mu.Lock()
	task.Status = StatusComplete
	task.Progress.SyncedFiles = 10
	task.Progress.SyncedBytes = task.Progress.TotalBytes
	completedAt := time.Now()
	task.LastSyncAt = &completedAt
	m.mu.Unlock()

	m.emitEvent(taskID, "complete", "同步完成")
}

// StopSync 停止同步.
func (m *Manager) StopSync(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务不存在: %s", taskID)
	}
	if task.Status != StatusSyncing {
		return fmt.Errorf("任务未在同步中")
	}

	task.Status = StatusPaused
	return nil
}

// PauseTask 暂停任务.
func (m *Manager) PauseTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	task.Status = StatusPaused
	m.scheduler.Remove(taskID)
	return nil
}

// ResumeTask 恢复任务.
func (m *Manager) ResumeTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	if task.Config.Enabled {
		m.scheduleTaskLocked(task)
	}
	task.Status = StatusIdle
	return nil
}

// GetAllStatus 获取所有任务状态.
func (m *Manager) GetAllStatus() []*SyncStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]*SyncStatus, 0, len(m.tasks))
	for _, t := range m.tasks {
		statuses = append(statuses, &SyncStatus{
			TaskID:     t.Config.ID,
			TaskName:   t.Config.Name,
			Status:     t.Status,
			Progress:   t.Progress,
			Error:      t.Error,
			LastSyncAt: t.LastSyncAt,
		})
	}
	return statuses
}

// GetStatus 获取单个任务状态.
func (m *Manager) GetStatus(taskID string) (*SyncStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	return &SyncStatus{
		TaskID:     t.Config.ID,
		TaskName:   t.Config.Name,
		Status:     t.Status,
		Progress:   t.Progress,
		Error:      t.Error,
		LastSyncAt: t.LastSyncAt,
	}, nil
}

// Events 返回事件通道.
func (m *Manager) Events() <-chan SyncEvent {
	return m.events
}

// Close 关闭管理器.
func (m *Manager) Close() {
	m.scheduler.Stop()
	close(m.events)
}

// scheduleTask 调度任务 (外部调用).
func (m *Manager) scheduleTask(task *SyncTask) {
	m.scheduleTaskLocked(task)
}

// scheduleTaskLocked 调度任务 (需要持有锁).
func (m *Manager) scheduleTaskLocked(task *SyncTask) {
	taskID := task.Config.ID

	switch task.Config.ScheduleMode {
	case ScheduleInterval:
		interval := task.Config.ScheduleInterval
		if interval <= 0 {
			interval = time.Hour
		}
		m.scheduler.AddIntervalTask(taskID, interval, func() {
			ctx := context.Background()
			if err := m.StartSync(ctx, taskID); err != nil {
				m.logger.Error("定时同步失败", zap.String("task", taskID), zap.Error(err))
			}
		})

	case ScheduleRealtime:
		// 实时模式: 使用较短的轮询间隔 (5 秒)
		m.scheduler.AddIntervalTask(taskID, 5*time.Second, func() {
			m.mu.RLock()
			t := m.tasks[taskID]
			m.mu.RUnlock()
			if t != nil && t.Status != StatusSyncing {
				ctx := context.Background()
				if err := m.StartSync(ctx, taskID); err != nil {
					m.logger.Error("实时同步失败", zap.String("task", taskID), zap.Error(err))
				}
			}
		})

	case ScheduleManual:
		// 不自动调度

	default:
		m.logger.Warn("未知调度模式", zap.String("mode", string(task.Config.ScheduleMode)))
	}
}

// emitEvent 发送同步事件.
func (m *Manager) emitEvent(taskID, eventType, message string) {
	event := SyncEvent{
		TaskID:    taskID,
		Type:      eventType,
		Message:   message,
		Timestamp: time.Now(),
	}

	select {
	case m.events <- event:
	default:
		m.logger.Warn("事件队列已满，丢弃事件", zap.String("task", taskID))
	}
}
