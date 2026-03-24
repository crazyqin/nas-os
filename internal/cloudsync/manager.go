package cloudsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 云同步管理器
type Manager struct {
	mu sync.RWMutex

	// 配置
	configPath string

	// 提供商
	providers       map[string]*ProviderConfig
	activeProviders map[string]Provider

	// 同步任务
	tasks    map[string]*SyncTask
	engines  map[string]*SyncEngine
	statuses map[string]*SyncStatus

	// 调度器
	scheduler *Scheduler

	// 回调
	onTaskComplete func(taskID string, status *SyncStatus)
}

// NewManager 创建云同步管理器
func NewManager(configPath string) *Manager {
	return &Manager{
		configPath:      configPath,
		providers:       make(map[string]*ProviderConfig),
		activeProviders: make(map[string]Provider),
		tasks:           make(map[string]*SyncTask),
		engines:         make(map[string]*SyncEngine),
		statuses:        make(map[string]*SyncStatus),
		scheduler:       NewScheduler(),
	}
}

// Initialize 初始化管理器
func (m *Manager) Initialize() error {
	if err := m.loadConfig(); err != nil {
		// 配置文件不存在是正常的
		m.mu.Lock()
		m.providers = make(map[string]*ProviderConfig)
		m.tasks = make(map[string]*SyncTask)
		m.mu.Unlock()
	}

	// 启动调度器
	go m.scheduler.Run()

	return nil
}

// ==================== 提供商管理 ====================

// CreateProvider 创建云存储提供商
func (m *Manager) CreateProvider(config ProviderConfig) (*ProviderConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证配置
	if config.Name == "" {
		return nil, fmt.Errorf("提供商名称不能为空")
	}

	if config.Type == "" {
		return nil, fmt.Errorf("提供商类型不能为空")
	}

	// 生成 ID
	if config.ID == "" {
		config.ID = "provider_" + uuid.New().String()[:8]
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	config.Enabled = true

	// 验证配置
	if err := m.validateProviderConfig(&config); err != nil {
		return nil, err
	}

	m.providers[config.ID] = &config

	if err := m.saveConfigLocked(); err != nil {
		delete(m.providers, config.ID)
		return nil, err
	}

	return &config, nil
}

// GetProvider 获取提供商配置
func (m *Manager) GetProvider(id string) (*ProviderConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, ok := m.providers[id]
	if !ok {
		return nil, fmt.Errorf("提供商不存在: %s", id)
	}

	return config, nil
}

// ListProviders 列出所有提供商
func (m *Manager) ListProviders() []*ProviderConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*ProviderConfig
	for _, config := range m.providers {
		list = append(list, config)
	}
	return list
}

// UpdateProvider 更新提供商
func (m *Manager) UpdateProvider(id string, config ProviderConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.providers[id]; !ok {
		return fmt.Errorf("提供商不存在: %s", id)
	}

	config.ID = id
	config.UpdatedAt = time.Now()

	// 验证配置
	if err := m.validateProviderConfig(&config); err != nil {
		return err
	}

	m.providers[id] = &config

	// 清除已初始化的客户端
	delete(m.activeProviders, id)

	return m.saveConfigLocked()
}

// DeleteProvider 删除提供商
func (m *Manager) DeleteProvider(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.providers[id]; !ok {
		return fmt.Errorf("提供商不存在: %s", id)
	}

	// 检查是否有任务使用此提供商
	for _, task := range m.tasks {
		if task.ProviderID == id {
			return fmt.Errorf("存在使用此提供商的同步任务，请先删除任务")
		}
	}

	delete(m.providers, id)
	delete(m.activeProviders, id)

	return m.saveConfigLocked()
}

// TestProvider 测试提供商连接
func (m *Manager) TestProvider(id string) (*ConnectionTestResult, error) {
	m.mu.RLock()
	config, ok := m.providers[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("提供商不存在: %s", id)
	}

	provider, err := NewProvider(context.Background(), config)
	if err != nil {
		return &ConnectionTestResult{
			Success:  false,
			Provider: config.Type,
			Error:    err.Error(),
			Message:  fmt.Sprintf("初始化失败: %v", err),
		}, nil
	}
	defer func() { _ = provider.Close() }()

	return provider.TestConnection(context.Background())
}

// getOrCreateProvider 获取或创建提供商实例
func (m *Manager) getOrCreateProvider(id string) (Provider, error) {
	m.mu.RLock()
	if provider, ok := m.activeProviders[id]; ok {
		m.mu.RUnlock()
		return provider, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if provider, ok := m.activeProviders[id]; ok {
		return provider, nil
	}

	config, ok := m.providers[id]
	if !ok {
		return nil, fmt.Errorf("提供商不存在: %s", id)
	}

	provider, err := NewProvider(context.Background(), config)
	if err != nil {
		return nil, err
	}

	m.activeProviders[id] = provider
	return provider, nil
}

// validateProviderConfig 验证提供商配置
func (m *Manager) validateProviderConfig(config *ProviderConfig) error {
	switch config.Type {
	case ProviderAliyunOSS, ProviderTencentCOS, ProviderAWSS3, ProviderBackblazeB2, ProviderS3Compatible:
		if config.AccessKey == "" {
			return fmt.Errorf("access key 不能为空")
		}
		if config.SecretKey == "" {
			return fmt.Errorf("secret key 不能为空")
		}
		if config.Bucket == "" {
			return fmt.Errorf("bucket 不能为空")
		}

	case ProviderWebDAV:
		if config.Endpoint == "" {
			return fmt.Errorf("endpoint 不能为空")
		}

	case ProviderGoogleDrive:
		if config.RefreshToken == "" {
			return fmt.Errorf("refresh token 不能为空")
		}

	case ProviderOneDrive:
		if config.RefreshToken == "" {
			return fmt.Errorf("refresh token 不能为空")
		}

	// 中国网盘验证
	case Provider115:
		if config.AccessToken == "" {
			return fmt.Errorf("access token 不能为空")
		}

	case ProviderQuark:
		if config.AccessToken == "" {
			return fmt.Errorf("access token 不能为空")
		}

	case ProviderAliyunPan:
		if config.RefreshToken == "" {
			return fmt.Errorf("refresh token 不能为空")
		}

	default:
		return fmt.Errorf("不支持的提供商类型: %s", config.Type)
	}

	return nil
}

// ==================== 同步任务管理 ====================

// CreateSyncTask 创建同步任务
func (m *Manager) CreateSyncTask(task SyncTask) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证
	if task.Name == "" {
		return nil, fmt.Errorf("任务名称不能为空")
	}

	if task.ProviderID == "" {
		return nil, fmt.Errorf("提供商不能为空")
	}

	if _, ok := m.providers[task.ProviderID]; !ok {
		return nil, fmt.Errorf("提供商不存在: %s", task.ProviderID)
	}

	if task.LocalPath == "" {
		return nil, fmt.Errorf("本地路径不能为空")
	}

	if task.RemotePath == "" {
		return nil, fmt.Errorf("远程路径不能为空")
	}

	// 生成 ID
	if task.ID == "" {
		task.ID = "task_" + uuid.New().String()[:8]
	}

	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	task.Status = TaskStatusIdle

	// 设置默认值
	if task.Direction == "" {
		task.Direction = SyncDirectionBidirect
	}
	if task.Mode == "" {
		task.Mode = SyncModeSync
	}
	if task.ScheduleType == "" {
		task.ScheduleType = ScheduleTypeManual
	}
	if task.ConflictStrategy == "" {
		task.ConflictStrategy = ConflictStrategyNewer
	}

	m.tasks[task.ID] = &task

	if err := m.saveConfigLocked(); err != nil {
		delete(m.tasks, task.ID)
		return nil, err
	}

	// 如果是定时任务，添加到调度器
	if task.ScheduleType != ScheduleTypeManual {
		if err := m.scheduleTask(&task); err != nil {
			return nil, fmt.Errorf("添加定时任务失败: %w", err)
		}
	}

	return &task, nil
}

// GetSyncTask 获取同步任务
func (m *Manager) GetSyncTask(id string) (*SyncTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", id)
	}

	return task, nil
}

// ListSyncTasks 列出所有同步任务
func (m *Manager) ListSyncTasks() []*SyncTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*SyncTask
	for _, task := range m.tasks {
		list = append(list, task)
	}
	return list
}

// UpdateSyncTask 更新同步任务
func (m *Manager) UpdateSyncTask(id string, task SyncTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[id]; !ok {
		return fmt.Errorf("任务不存在: %s", id)
	}

	task.ID = id
	task.UpdatedAt = time.Now()

	m.tasks[id] = &task

	// 更新调度
	m.scheduler.RemoveTask(id)
	if task.ScheduleType != ScheduleTypeManual && task.Enabled {
		if err := m.scheduleTask(&task); err != nil {
			return fmt.Errorf("添加定时任务失败: %w", err)
		}
	}

	return m.saveConfigLocked()
}

// DeleteSyncTask 删除同步任务
func (m *Manager) DeleteSyncTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[id]; !ok {
		return fmt.Errorf("任务不存在: %s", id)
	}

	// 停止正在运行的任务
	if engine, ok := m.engines[id]; ok {
		engine.Cancel()
		delete(m.engines, id)
	}

	m.scheduler.RemoveTask(id)
	delete(m.tasks, id)
	delete(m.statuses, id)

	return m.saveConfigLocked()
}

// ==================== 同步执行 ====================

// RunSyncTask 执行同步任务
func (m *Manager) RunSyncTask(taskID string) (*SyncStatus, error) {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	// 检查是否正在运行
	if status, ok := m.statuses[taskID]; ok && status.Status == TaskStatusRunning {
		m.mu.RUnlock()
		return status, nil
	}
	m.mu.RUnlock()

	// 获取提供商
	provider, err := m.getOrCreateProvider(task.ProviderID)
	if err != nil {
		return nil, err
	}

	// 创建同步引擎
	engine := NewSyncEngine(provider, task)

	m.mu.Lock()
	m.engines[taskID] = engine
	m.statuses[taskID] = engine.GetStatus()
	m.mu.Unlock()

	// 设置回调
	engine.SetCallbacks(
		func(status *SyncStatus) {
			m.mu.Lock()
			m.statuses[taskID] = status
			m.mu.Unlock()
		},
		func(status *SyncStatus) {
			m.mu.Lock()
			task.LastSync = time.Now()
			task.LastError = ""
			if status.Status == TaskStatusFailed {
				if len(status.Errors) > 0 {
					task.LastError = status.Errors[len(status.Errors)-1].Error
				}
			}
			m.mu.Unlock()

			if m.onTaskComplete != nil {
				m.onTaskComplete(taskID, status)
			}
		},
		func(err error, path string) {
			// 错误处理
		},
		func(conflict *ConflictInfo) ConflictStrategy {
			return task.ConflictStrategy
		},
	)

	// 异步执行
	go func() {
		_ = engine.Run(context.Background())

		m.mu.Lock()
		delete(m.engines, taskID)
		m.mu.Unlock()
	}()

	return engine.GetStatus(), nil
}

// PauseSyncTask 暂停同步任务
func (m *Manager) PauseSyncTask(taskID string) error {
	m.mu.RLock()
	engine, ok := m.engines[taskID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务未在运行: %s", taskID)
	}

	engine.Pause()
	return nil
}

// ResumeSyncTask 恢复同步任务
func (m *Manager) ResumeSyncTask(taskID string) error {
	m.mu.RLock()
	engine, ok := m.engines[taskID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务未在运行: %s", taskID)
	}

	engine.Resume()
	return nil
}

// CancelSyncTask 取消同步任务
func (m *Manager) CancelSyncTask(taskID string) error {
	m.mu.RLock()
	engine, ok := m.engines[taskID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务未在运行: %s", taskID)
	}

	engine.Cancel()
	return nil
}

// GetSyncStatus 获取同步状态
func (m *Manager) GetSyncStatus(taskID string) (*SyncStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, ok := m.statuses[taskID]
	if !ok {
		// 返回空闲状态
		return &SyncStatus{
			TaskID: taskID,
			Status: TaskStatusIdle,
		}, nil
	}

	return status, nil
}

// GetAllSyncStatuses 获取所有任务状态
func (m *Manager) GetAllSyncStatuses() map[string]*SyncStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*SyncStatus)
	for id, status := range m.statuses {
		result[id] = status
	}

	// 添加未运行的任务
	for id := range m.tasks {
		if _, ok := result[id]; !ok {
			result[id] = &SyncStatus{
				TaskID: id,
				Status: TaskStatusIdle,
			}
		}
	}

	return result
}

// ==================== 调度 ====================

// scheduleTask 添加任务到调度器
func (m *Manager) scheduleTask(task *SyncTask) error {
	switch task.ScheduleType {
	case ScheduleTypeInterval:
		return m.scheduler.AddIntervalTask(task.ID, task.ScheduleExpr, func() {
			_, _ = m.RunSyncTask(task.ID)
		})

	case ScheduleTypeCron:
		return m.scheduler.AddCronTask(task.ID, task.ScheduleExpr, func() {
			_, _ = m.RunSyncTask(task.ID)
		})
	}
	return nil
}

// ==================== 统计 ====================

// GetStats 获取统计信息
func (m *Manager) GetStats() *SyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SyncStats{
		TotalProviders: int64(len(m.providers)),
		TotalTasks:     int64(len(m.tasks)),
	}

	var activeCount int64
	var totalSynced int64
	var totalBytes int64
	var lastSync time.Time

	for _, task := range m.tasks {
		if status, ok := m.statuses[task.ID]; ok {
			if status.Status == TaskStatusRunning {
				activeCount++
			}
			totalSynced += status.UploadedFiles + status.DownloadedFiles
			totalBytes += status.TransferredBytes

			if task.LastSync.After(lastSync) {
				lastSync = task.LastSync
			}
		}
	}

	stats.ActiveTasks = activeCount
	stats.TotalSynced = totalSynced
	stats.TotalBytes = totalBytes
	stats.TotalBytesHuman = humanBytes(totalBytes)
	stats.LastSyncTime = lastSync

	return stats
}

// ==================== 配置持久化 ====================

type configData struct {
	Providers map[string]*ProviderConfig `json:"providers"`
	Tasks     map[string]*SyncTask       `json:"tasks"`
}

func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg configData
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	m.mu.Lock()
	m.providers = cfg.Providers
	m.tasks = cfg.Tasks
	m.mu.Unlock()

	return nil
}

func (m *Manager) saveConfigLocked() error {
	cfg := configData{
		Providers: m.providers,
		Tasks:     m.tasks,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0640)
}

// ==================== 辅助函数 ====================

func humanBytes(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case size >= TB:
		return fmt.Sprintf("%.2f TB", float64(size)/TB)
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}
