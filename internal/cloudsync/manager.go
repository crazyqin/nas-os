package cloudsync

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 类型别名，供 realtime_sync.go 等模块使用
type Manager = CloudSyncManager

// CloudSyncManager 云同步管理器
type CloudSyncManager struct {
	mu sync.RWMutex

	// 存储
	connections map[string]*ConnectionConfig
	providers   map[string]*ProviderItem
	tasks       map[string]*SyncTask
	status      map[string]*SyncStatus
	logs        []SyncLog

	// 运行时状态
	cancel map[string]chan struct{} // 任务取消信号

	configPath string
	logger     *zap.Logger
}

// NewManager 创建云同步管理器（兼容旧接口，configPath 暂未使用）
func NewManager(configPath string) *Manager {
	m := NewCloudSyncManager(zap.NewNop())
	m.configPath = configPath
	return m
}

// NewCloudSyncManager 创建云同步管理器
func NewCloudSyncManager(logger *zap.Logger) *CloudSyncManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CloudSyncManager{
		connections: make(map[string]*ConnectionConfig),
		providers:   make(map[string]*ProviderItem),
		tasks:       make(map[string]*SyncTask),
		status:      make(map[string]*SyncStatus),
		logs:        make([]SyncLog, 0, 1000),
		cancel:      make(map[string]chan struct{}),
		logger:      logger,
	}
}

// Initialize 初始化管理器
func (m *CloudSyncManager) Initialize() error {
	if m.configPath != "" {
		if err := m.loadConfig(); err != nil && !os.IsNotExist(err) {
			m.logger.Warn("failed to load config", zap.Error(err))
		}
	}
	m.logger.Info("cloud sync manager initialized", zap.String("config", m.configPath))
	return nil
}

// CreateProvider 创建提供商实例
func (m *CloudSyncManager) CreateProvider(config ProviderConfig) (*ProviderItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider := &ProviderItem{
		ID:        uuid.New().String(),
		Name:      config.Name,
		Type:      config.Type,
		Enabled:   true,
		Bucket:    config.Bucket,
		Config:    config,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.providers[provider.ID] = provider

	if err := m.saveConfig(); err != nil {
		m.logger.Warn("failed to save config", zap.Error(err))
	}

	m.logger.Info("cloud provider created",
		zap.String("id", provider.ID),
		zap.String("name", provider.Name),
		zap.String("type", string(provider.Type)))

	return provider, nil
}

// GetProvider 获取提供商实例
func (m *CloudSyncManager) GetProvider(id string) (*ProviderItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, exists := m.providers[id]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", id)
	}
	return provider, nil
}

// ListProviders 列出所有提供商
func (m *CloudSyncManager) ListProviders() []*ProviderItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]*ProviderItem, 0, len(m.providers))
	for _, p := range m.providers {
		providers = append(providers, p)
	}
	return providers
}

// CreateSyncTask 创建同步任务（兼容旧接口）
func (m *CloudSyncManager) CreateSyncTask(task SyncTask) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	if task.Status == "" {
		task.Status = StatusIdle
	}
	if task.ConflictStrategy == "" {
		task.ConflictStrategy = ConflictStrategyNewer
	}
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	m.tasks[task.ID] = &task

	if err := m.saveConfig(); err != nil {
		m.logger.Warn("failed to save config", zap.Error(err))
	}

	m.logger.Info("sync task created",
		zap.String("id", task.ID),
		zap.String("name", task.Name))

	return &task, nil
}

// GetSyncTask 获取同步任务（兼容旧接口）
func (m *CloudSyncManager) GetSyncTask(id string) (*SyncTask, error) {
	return m.GetTask(id)
}

// DeleteSyncTask 删除同步任务（兼容旧接口）
func (m *CloudSyncManager) DeleteSyncTask(id string) error {
	return m.DeleteTask(id)
}

// UpdateProvider 更新提供商配置
func (m *CloudSyncManager) UpdateProvider(id string, config ProviderConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider, exists := m.providers[id]
	if !exists {
		return fmt.Errorf("provider %s not found", id)
	}

	provider.Config = config
	if config.Name != "" {
		provider.Name = config.Name
	}
	if config.Type != "" {
		provider.Type = config.Type
	}
	if config.Bucket != "" {
		provider.Bucket = config.Bucket
	}
	provider.UpdatedAt = time.Now()

	return nil
}

// DeleteProvider 删除提供商
func (m *CloudSyncManager) DeleteProvider(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.providers[id]; !exists {
		return fmt.Errorf("provider %s not found", id)
	}

	// 检查是否有任务引用此提供商
	for _, task := range m.tasks {
		if task.ConnectionID == id {
			return fmt.Errorf("provider %s is in use by task %s", id, task.ID)
		}
	}

	delete(m.providers, id)
	m.logger.Info("cloud provider deleted", zap.String("id", id))
	return nil
}

// GetStats 获取统计信息
func (m *CloudSyncManager) GetStats() SyncStats {
	return m.GetSyncStats()
}

// ListSyncTasks 列出所有同步任务（兼容旧接口）
func (m *CloudSyncManager) ListSyncTasks() []*SyncTask {
	return m.ListTasks()
}

// UpdateSyncTask 更新同步任务（兼容旧接口）
func (m *CloudSyncManager) UpdateSyncTask(id string, task SyncTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("task %s not found", id)
	}

	if task.Name != "" {
		existing.Name = task.Name
	}
	if task.LocalPath != "" {
		existing.LocalPath = task.LocalPath
	}
	if task.RemotePath != "" {
		existing.RemotePath = task.RemotePath
	}
	if task.Direction != "" {
		existing.Direction = task.Direction
	}
	existing.UpdatedAt = time.Now()

	return nil
}

// GetAllSyncStatuses 获取所有同步任务状态
func (m *CloudSyncManager) GetAllSyncStatuses() map[string]*SyncStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*SyncStatus, len(m.status))
	for k, v := range m.status {
		result[k] = v
	}
	return result
}

// ============================================================
// 辅助函数
// ============================================================

// humanBytes 将字节数转换为人类可读的格式
func humanBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ============================================================
// 连接管理
// ============================================================

// CreateConnection 创建云存储连接
func (m *CloudSyncManager) CreateConnection(req CreateConnectionRequest) (*ConnectionConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn := &ConnectionConfig{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Backend:     req.Backend,
		Endpoint:    req.Endpoint,
		Region:      req.Region,
		Bucket:      req.Bucket,
		AccessKey:   req.AccessKey,
		SecretKey:   req.SecretKey,
		BasePath:    req.BasePath,
		AccountName: req.AccountName,
		ProjectID:   req.ProjectID,
		UseSSL:      req.UseSSL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := conn.Validate(); err != nil {
		return nil, err
	}

	m.connections[conn.ID] = conn
	m.logger.Info("cloud connection created",
		zap.String("id", conn.ID),
		zap.String("name", conn.Name),
		zap.String("backend", string(conn.Backend)))

	return conn, nil
}

// GetConnection 获取连接配置
func (m *CloudSyncManager) GetConnection(id string) (*ConnectionConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[id]
	if !exists {
		return nil, fmt.Errorf("connection %s not found", id)
	}
	return conn, nil
}

// ListConnections 列出所有连接
func (m *CloudSyncManager) ListConnections() []*ConnectionConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conns := make([]*ConnectionConfig, 0, len(m.connections))
	for _, c := range m.connections {
		conns = append(conns, c)
	}
	return conns
}

// UpdateConnection 更新连接配置
func (m *CloudSyncManager) UpdateConnection(id string, req UpdateConnectionRequest) (*ConnectionConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[id]
	if !exists {
		return nil, fmt.Errorf("connection %s not found", id)
	}

	if req.Name != "" {
		conn.Name = req.Name
	}
	if req.Endpoint != "" {
		conn.Endpoint = req.Endpoint
	}
	if req.Region != "" {
		conn.Region = req.Region
	}
	if req.Bucket != "" {
		conn.Bucket = req.Bucket
	}
	if req.AccessKey != "" {
		conn.AccessKey = req.AccessKey
	}
	if req.SecretKey != "" {
		conn.SecretKey = req.SecretKey
	}
	if req.BasePath != "" {
		conn.BasePath = req.BasePath
	}
	if req.AccountName != "" {
		conn.AccountName = req.AccountName
	}
	if req.ProjectID != "" {
		conn.ProjectID = req.ProjectID
	}
	if req.UseSSL != nil {
		conn.UseSSL = *req.UseSSL
	}
	conn.UpdatedAt = time.Now()

	if err := conn.Validate(); err != nil {
		return nil, err
	}

	m.logger.Info("cloud connection updated", zap.String("id", id))
	return conn, nil
}

// DeleteConnection 删除连接
func (m *CloudSyncManager) DeleteConnection(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.connections[id]; !exists {
		return fmt.Errorf("connection %s not found", id)
	}

	// 检查是否有任务引用此连接
	for _, task := range m.tasks {
		if task.ConnectionID == id {
			return fmt.Errorf("connection %s is in use by task %s", id, task.ID)
		}
	}

	delete(m.connections, id)
	m.logger.Info("cloud connection deleted", zap.String("id", id))
	return nil
}

// ============================================================
// 同步任务管理
// ============================================================

// CreateTask 创建同步任务
func (m *CloudSyncManager) CreateTask(req CreateTaskRequest) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证连接或提供商存在
	if _, exists := m.connections[req.ConnectionID]; !exists {
		// 尝试作为提供商 ID
		if _, exists := m.providers[req.ConnectionID]; !exists {
			return nil, fmt.Errorf("connection or provider %s not found", req.ConnectionID)
		}
	}

	// 设置默认值
	conflictPolicy := req.ConflictPolicy
	if conflictPolicy == "" {
		conflictPolicy = ConflictLocalFirst
	}
	if !conflictPolicy.IsValid() {
		return nil, fmt.Errorf("invalid conflict policy: %s", conflictPolicy)
	}

	schedule := SyncSchedule{Type: ScheduleManual}
	if req.Schedule != nil {
		schedule = *req.Schedule
	}

	transfer := DefaultTransferConfig()
	if req.Transfer != nil {
		transfer = *req.Transfer
	}

	filter := FileFilter{}
	if req.Filter != nil {
		filter = *req.Filter
	}

	task := &SyncTask{
		ID:             uuid.New().String(),
		Name:           req.Name,
		ConnectionID:   req.ConnectionID,
		LocalPath:      req.LocalPath,
		RemotePath:     req.RemotePath,
		Mode:           req.Mode,
		ConflictPolicy: conflictPolicy,
		Filter:         filter,
		Schedule:       schedule,
		Transfer:       transfer,
		Status:         StatusIdle,
		Enabled:        true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := task.Validate(); err != nil {
		return nil, err
	}

	m.tasks[task.ID] = task
	m.status[task.ID] = &SyncStatus{
		TaskID:   task.ID,
		TaskName: task.Name,
		Status:   StatusIdle,
	}

	m.logger.Info("sync task created",
		zap.String("id", task.ID),
		zap.String("name", task.Name),
		zap.String("mode", string(task.Mode)))

	return task, nil
}

// GetTask 获取同步任务
func (m *CloudSyncManager) GetTask(id string) (*SyncTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return task, nil
}

// ListTasks 列出所有同步任务
func (m *CloudSyncManager) ListTasks() []*SyncTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*SyncTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// UpdateTask 更新同步任务
func (m *CloudSyncManager) UpdateTask(id string, req UpdateTaskRequest) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task %s not found", id)
	}

	if req.Name != "" {
		task.Name = req.Name
	}
	if req.LocalPath != "" {
		task.LocalPath = req.LocalPath
	}
	if req.RemotePath != "" {
		task.RemotePath = req.RemotePath
	}
	if req.Mode != nil {
		if !req.Mode.IsValid() {
			return nil, fmt.Errorf("invalid sync mode: %s", *req.Mode)
		}
		task.Mode = *req.Mode
	}
	if req.ConflictPolicy != nil {
		if !req.ConflictPolicy.IsValid() {
			return nil, fmt.Errorf("invalid conflict policy: %s", *req.ConflictPolicy)
		}
		task.ConflictPolicy = *req.ConflictPolicy
	}
	if req.Filter != nil {
		task.Filter = *req.Filter
	}
	if req.Schedule != nil {
		task.Schedule = *req.Schedule
	}
	if req.Transfer != nil {
		task.Transfer = *req.Transfer
	}
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}
	task.UpdatedAt = time.Now()

	if err := task.Validate(); err != nil {
		return nil, err
	}

	m.logger.Info("sync task updated", zap.String("id", id))
	return task, nil
}

// DeleteTask 删除同步任务
func (m *CloudSyncManager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("task %s not found", id)
	}

	if task.Status == StatusSyncing {
		return fmt.Errorf("cannot delete task %s while syncing", id)
	}

	delete(m.tasks, id)
	delete(m.status, id)
	delete(m.cancel, id)

	m.logger.Info("sync task deleted", zap.String("id", id))
	return nil
}

// ============================================================
// 同步控制
// ============================================================

// StartSync 启动同步
func (m *CloudSyncManager) StartSync(taskID string) error {
	m.mu.Lock()

	task, exists := m.tasks[taskID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}

	if !task.Enabled {
		m.mu.Unlock()
		return fmt.Errorf("task %s is disabled", taskID)
	}

	if task.Status == StatusSyncing {
		m.mu.Unlock()
		return fmt.Errorf("task %s is already syncing", taskID)
	}

	// 创建取消通道
	cancel := make(chan struct{})
	m.cancel[taskID] = cancel

	task.Status = StatusSyncing
	task.UpdatedAt = time.Now()
	m.status[taskID] = &SyncStatus{
		TaskID:    taskID,
		TaskName:  task.Name,
		Status:    StatusSyncing,
		StartedAt: timePtr(time.Now()),
	}

	m.mu.Unlock()

	// 异步执行同步
	go m.executeSync(task, cancel)

	m.logger.Info("sync started", zap.String("task_id", taskID))
	return nil
}

// PauseSync 暂停同步
func (m *CloudSyncManager) PauseSync(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != StatusSyncing {
		return fmt.Errorf("task %s is not syncing", taskID)
	}

	// 发送取消信号
	if cancel, ok := m.cancel[taskID]; ok {
		close(cancel)
		delete(m.cancel, taskID)
	}

	task.Status = StatusPaused
	task.UpdatedAt = time.Now()
	if status, ok := m.status[taskID]; ok {
		status.Status = StatusPaused
	}

	m.logger.Info("sync paused", zap.String("task_id", taskID))
	return nil
}

// ResumeSync 恢复同步
func (m *CloudSyncManager) ResumeSync(taskID string) error {
	m.mu.Lock()

	task, exists := m.tasks[taskID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != StatusPaused {
		m.mu.Unlock()
		return fmt.Errorf("task %s is not paused", taskID)
	}

	// 创建新的取消通道
	cancel := make(chan struct{})
	m.cancel[taskID] = cancel

	task.Status = StatusSyncing
	task.UpdatedAt = time.Now()
	m.status[taskID] = &SyncStatus{
		TaskID:    taskID,
		TaskName:  task.Name,
		Status:    StatusSyncing,
		StartedAt: timePtr(time.Now()),
	}

	m.mu.Unlock()

	// 异步执行同步
	go m.executeSync(task, cancel)

	m.logger.Info("sync resumed", zap.String("task_id", taskID))
	return nil
}

// StopSync 停止同步（重置为idle）
func (m *CloudSyncManager) StopSync(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status == StatusSyncing {
		if cancel, ok := m.cancel[taskID]; ok {
			close(cancel)
			delete(m.cancel, taskID)
		}
	}

	task.Status = StatusIdle
	task.UpdatedAt = time.Now()
	if status, ok := m.status[taskID]; ok {
		status.Status = StatusIdle
		status.Progress = 0
	}

	m.logger.Info("sync stopped", zap.String("task_id", taskID))
	return nil
}

// ============================================================
// 同步执行（模拟）
// ============================================================

// executeSync 执行同步任务（模拟实现）
func (m *CloudSyncManager) executeSync(task *SyncTask, cancel chan struct{}) {
	startTime := time.Now()

	m.addLog(task.ID, "info", fmt.Sprintf("Sync started: %s -> %s (%s)",
		task.LocalPath, task.RemotePath, task.Mode), "")

	// 模拟同步过程
	totalFiles := 50 + (time.Now().UnixNano() % 50) // 随机文件数
	syncedFiles := int64(0)
	failedFiles := int64(0)
	skippedFiles := int64(0)

	for i := int64(0); i < totalFiles; i++ {
		// 检查取消信号
		select {
		case <-cancel:
			m.mu.Lock()
			task.Status = StatusPaused
			m.mu.Unlock()
			m.addLog(task.ID, "info", "Sync paused by user", "")
			return
		default:
		}

		// 模拟文件同步延迟
		time.Sleep(50 * time.Millisecond)

		// 模拟随机失败
		if (time.Now().UnixNano()+i)%17 == 0 {
			failedFiles++
			m.addLog(task.ID, "error", "Failed to sync file",
				fmt.Sprintf("/path/to/file_%d.dat", i))
			continue
		}

		// 模拟跳过
		if (time.Now().UnixNano()+i)%11 == 0 {
			skippedFiles++
			continue
		}

		syncedFiles++

		// 更新进度
		m.mu.Lock()
		progress := float64(i+1) / float64(totalFiles) * 100
		if status, ok := m.status[task.ID]; ok {
			status.Progress = progress
			status.BytesSynced = syncedFiles * 1024 * 1024
			status.BytesTotal = int64(totalFiles) * 1024 * 1024
			status.SpeedBps = int64(float64(status.BytesSynced) / time.Since(startTime).Seconds())
			if status.SpeedBps > 0 {
				remaining := status.BytesTotal - status.BytesSynced
				status.ETASeconds = int(remaining / status.SpeedBps)
			}
		}
		m.mu.Unlock()
	}

	// 完成
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	m.mu.Lock()
	task.Status = StatusIdle
	task.LastSyncTime = &endTime
	task.TotalFiles = int64(totalFiles)
	task.TotalSize = int64(totalFiles) * 1024 * 1024
	task.SyncedFiles = syncedFiles
	task.FailedFiles = failedFiles
	task.SkippedFiles = skippedFiles
	task.UpdatedAt = endTime

	if failedFiles > 0 {
		task.LastSyncResult = "partial"
		task.LastError = fmt.Sprintf("%d files failed", failedFiles)
		task.ErrorCount += int(failedFiles)
	} else {
		task.LastSyncResult = "success"
		task.LastError = ""
	}

	if status, ok := m.status[task.ID]; ok {
		status.Status = StatusIdle
		status.Progress = 100
		status.CurrentFile = ""
	}
	m.mu.Unlock()

	m.addLog(task.ID, "info", fmt.Sprintf("Sync completed in %s: %d synced, %d skipped, %d failed",
		duration.Round(time.Second), syncedFiles, skippedFiles, failedFiles), "")
}

// ============================================================
// 状态和统计
// ============================================================

// GetSyncStatus 获取同步状态
func (m *CloudSyncManager) GetSyncStatus(taskID string) (*SyncStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.status[taskID]
	if !exists {
		// 返回默认状态
		return &SyncStatus{
			TaskID: taskID,
			Status: StatusIdle,
		}, nil
	}
	return status, nil
}

// GetSyncLogs 获取同步日志
func (m *CloudSyncManager) GetSyncLogs(taskID string, limit int) []SyncLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var logs []SyncLog
	for _, log := range m.logs {
		if taskID == "" || log.TaskID == taskID {
			logs = append(logs, log)
		}
	}

	if limit > 0 && limit < len(logs) {
		logs = logs[len(logs)-limit:]
	}
	return logs
}

// GetSyncStats 获取同步统计
func (m *CloudSyncManager) GetSyncStats() SyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := SyncStats{
		TotalProviders: int64(len(m.providers)),
		TotalTasks:     int64(len(m.tasks)),
	}

	for _, task := range m.tasks {
		stats.TotalFiles += task.TotalFiles
		stats.TotalSize += task.TotalSize
		stats.SyncedFiles += task.SyncedFiles
		stats.FailedFiles += task.FailedFiles
		stats.TotalBandwidth += task.Transfer.BandwidthLimit

		switch task.Status {
		case StatusSyncing:
			stats.ActiveTasks++
		case StatusPaused:
			stats.PausedTasks++
		case StatusError:
			stats.ErrorTasks++
		}
	}

	return stats
}

// GetStorageUsage 获取存储用量（模拟）
func (m *CloudSyncManager) GetStorageUsage(connID string) (*StorageUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[connID]
	if !exists {
		return nil, fmt.Errorf("connection %s not found", connID)
	}

	// 模拟数据
	totalBytes := int64(1024 * 1024 * 1024 * 100) // 100GB
	usedBytes := int64(1024 * 1024 * 1024 * 35)    // 35GB

	return &StorageUsage{
		ConnectionID: connID,
		Backend:      conn.Backend,
		Bucket:       conn.Bucket,
		TotalBytes:   totalBytes,
		UsedBytes:    usedBytes,
		FreeBytes:    totalBytes - usedBytes,
		ObjectCount:  1234,
		QuotaBytes:   0,
	}, nil
}

// ============================================================
// Mock 数据
// ============================================================

// LoadMockData 加载演示数据
func (m *CloudSyncManager) LoadMockData() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建演示连接
	connections := []*ConnectionConfig{
		{
			ID:        "conn-s3-001",
			Name:      "AWS S3 Backup",
			Backend:   BackendS3,
			Region:    "us-east-1",
			Bucket:    "nas-backup-primary",
			AccessKey: "AKIAIOSFODNN7EXAMPLE",
			SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			BasePath:  "nas-backup/",
			UseSSL:    true,
			CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt: time.Now().Add(-2 * time.Hour),
		},
		{
			ID:          "conn-oss-002",
			Name:        "阿里云 OSS 冷存储",
			Backend:     BackendOSS,
			Endpoint:    "oss-cn-hangzhou.aliyuncs.com",
			Bucket:      "nas-cold-storage",
			AccessKey:   "LTAI5tPcHoFxFjq2example",
			SecretKey:   "aBcDeFgHiJkLmNoPqRsTuVwXyZexample",
			BasePath:    "archive/",
			UseSSL:      true,
			CreatedAt:   time.Now().Add(-15 * 24 * time.Hour),
			UpdatedAt:   time.Now().Add(-24 * time.Hour),
		},
		{
			ID:        "conn-minio-003",
			Name:      "MinIO 本地备份",
			Backend:   BackendMinIO,
			Endpoint:  "192.168.1.100:9000",
			Bucket:    "nas-local-backup",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			UseSSL:    false,
			CreatedAt: time.Now().Add(-60 * 24 * time.Hour),
			UpdatedAt: time.Now().Add(-12 * time.Hour),
		},
	}

	for _, c := range connections {
		m.connections[c.ID] = c
	}

	// 创建演示任务
	lastSync := time.Now().Add(-2 * time.Hour)
	tasks := []*SyncTask{
		{
			ID:             "task-001",
			Name:           "照片备份到S3",
			ConnectionID:   "conn-s3-001",
			LocalPath:      "/volume1/photos",
			RemotePath:     "photos/",
			Mode:           SyncModeUploadOnly,
			ConflictPolicy: ConflictLocalFirst,
			Filter: FileFilter{
				IncludeExtensions: []string{".jpg", ".jpeg", ".png", ".heic", ".mp4"},
				MaxFileSize:       100 * 1024 * 1024, // 100MB
				ExcludeHidden:     true,
			},
			Schedule: SyncSchedule{
				Type:     ScheduleCron,
				CronExpr: "0 2 * * *", // 每天凌晨2点
			},
			Transfer: TransferConfig{
				BandwidthLimit:      10240, // 10MB/s
				ConcurrentTransfers: 4,
				EncryptionEnabled:   true,
				MaxRetries:          3,
				RetryDelaySec:       5,
			},
			Status:         StatusIdle,
			Enabled:        true,
			LastSyncTime:   &lastSync,
			LastSyncResult: "success",
			TotalFiles:     15420,
			TotalSize:      1024 * 1024 * 1024 * 85, // 85GB
			SyncedFiles:    15420,
			SkippedFiles:   230,
			FailedFiles:    0,
			CreatedAt:      time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt:      time.Now().Add(-2 * time.Hour),
		},
		{
			ID:             "task-002",
			Name:           "文档双向同步",
			ConnectionID:   "conn-oss-002",
			LocalPath:      "/volume1/documents",
			RemotePath:     "documents/",
			Mode:           SyncModeBidirectional,
			ConflictPolicy: ConflictKeepBoth,
			Filter: FileFilter{
				IncludeExtensions: []string{".docx", ".xlsx", ".pptx", ".pdf", ".md"},
				ExcludeHidden:     true,
			},
			Schedule: SyncSchedule{
				Type:     ScheduleRealtime,
			},
			Transfer: TransferConfig{
				ConcurrentTransfers: 2,
				EncryptionEnabled:   false,
				MaxRetries:          5,
				RetryDelaySec:       10,
			},
			Status:         StatusIdle,
			Enabled:        true,
			LastSyncTime:   &lastSync,
			LastSyncResult: "success",
			TotalFiles:     3200,
			TotalSize:      1024 * 1024 * 1024 * 12, // 12GB
			SyncedFiles:    3200,
			SkippedFiles:   45,
			FailedFiles:    2,
			CreatedAt:      time.Now().Add(-15 * 24 * time.Hour),
			UpdatedAt:      time.Now().Add(-1 * time.Hour),
		},
		{
			ID:             "task-003",
			Name:           "数据库备份下载",
			ConnectionID:   "conn-minio-003",
			LocalPath:      "/volume1/db-backup",
			RemotePath:     "db-backup/",
			Mode:           SyncModeDownloadOnly,
			ConflictPolicy: ConflictRemoteFirst,
			Filter: FileFilter{
				IncludeExtensions: []string{".sql", ".gz", ".tar", ".bak"},
				MaxFileSize:       5 * 1024 * 1024 * 1024, // 5GB
			},
			Schedule: SyncSchedule{
				Type:     ScheduleCron,
				CronExpr: "0 */6 * * *", // 每6小时
			},
			Transfer: TransferConfig{
				BandwidthLimit:      5120, // 5MB/s
				ConcurrentTransfers: 2,
				EncryptionEnabled:   true,
				EncryptionKey:       "YWJjZGVmZzEyMzQ1Njc4OTA=", // base64 encoded
				MaxRetries:          3,
				RetryDelaySec:       5,
			},
			Status:         StatusIdle,
			Enabled:        true,
			LastSyncTime:   &lastSync,
			LastSyncResult: "partial",
			TotalFiles:     120,
			TotalSize:      1024 * 1024 * 1024 * 45, // 45GB
			SyncedFiles:    118,
			SkippedFiles:   0,
			FailedFiles:    2,
			LastError:      "2 files failed: connection timeout",
			ErrorCount:     2,
			CreatedAt:      time.Now().Add(-60 * 24 * time.Hour),
			UpdatedAt:      time.Now().Add(-30 * time.Minute),
		},
		{
			ID:             "task-004",
			Name:           "视频归档（已暂停）",
			ConnectionID:   "conn-oss-002",
			LocalPath:      "/volume1/videos",
			RemotePath:     "archive/videos/",
			Mode:           SyncModeUploadOnly,
			ConflictPolicy: ConflictLocalFirst,
			Filter: FileFilter{
				IncludeExtensions: []string{".mp4", ".mkv", ".avi", ".mov"},
				MinFileSize:       10 * 1024 * 1024, // 10MB
			},
			Schedule: SyncSchedule{
				Type: ScheduleManual,
			},
			Transfer: TransferConfig{
				BandwidthLimit:      2048, // 2MB/s
				ConcurrentTransfers: 1,
				EncryptionEnabled:   false,
				MaxRetries:          3,
				RetryDelaySec:       30,
			},
			Status:      StatusPaused,
			Enabled:     false,
			TotalFiles:  0,
			TotalSize:   0,
			SyncedFiles: 0,
			CreatedAt:   time.Now().Add(-7 * 24 * time.Hour),
			UpdatedAt:   time.Now().Add(-5 * 24 * time.Hour),
		},
	}

	for _, t := range tasks {
		m.tasks[t.ID] = t
		m.status[t.ID] = &SyncStatus{
			TaskID:   t.ID,
			TaskName: t.Name,
			Status:   t.Status,
		}
	}

	// 演示日志
	m.logs = []SyncLog{
		{TaskID: "task-001", Timestamp: lastSync.Add(-30 * time.Second), Level: "info", Message: "Sync completed: 15420 files synced"},
		{TaskID: "task-001", Timestamp: lastSync.Add(-1 * time.Minute), Level: "info", Message: "Sync started: /volume1/photos -> photos/ (upload_only)"},
		{TaskID: "task-002", Timestamp: lastSync.Add(-15 * time.Second), Level: "warn", Message: "Conflict resolved: report.docx (kept both versions)"},
		{TaskID: "task-002", Timestamp: lastSync.Add(-10 * time.Second), Level: "info", Message: "Sync completed: 3200 files synced, 2 failed"},
		{TaskID: "task-002", Timestamp: lastSync.Add(-5 * time.Second), Level: "error", Message: "Failed to sync: /volume1/documents/old/data.xlsx", FilePath: "/volume1/documents/old/data.xlsx", Error: "connection timeout"},
		{TaskID: "task-003", Timestamp: lastSync, Level: "info", Message: "Sync completed: 118 files synced, 2 failed"},
	}

	m.logger.Info("mock data loaded",
		zap.Int("connections", len(connections)),
		zap.Int("tasks", len(tasks)))
}

// RunSyncTask 执行同步任务（供 realtime_sync.go 调用）
func (m *CloudSyncManager) RunSyncTask(taskID string) (interface{}, error) {
	err := m.StartSync(taskID)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// ============================================================
// 配置持久化
// ============================================================

// configData 配置数据结构
type configData struct {
	Providers map[string]*ProviderItem `json:"providers"`
	Tasks     map[string]*SyncTask     `json:"tasks"`
}

// saveConfig 保存配置到文件
func (m *CloudSyncManager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	data := configData{
		Providers: m.providers,
		Tasks:     m.tasks,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(m.configPath, bytes, 0644)
}

// loadConfig 从文件加载配置
func (m *CloudSyncManager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	bytes, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var data configData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if data.Providers != nil {
		m.providers = data.Providers
	}
	if data.Tasks != nil {
		m.tasks = data.Tasks
	}

	return nil
}

// ============================================================
// 辅助函数
// ============================================================

// addLog 添加日志
func (m *CloudSyncManager) addLog(taskID, level, message, filePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log := SyncLog{
		TaskID:    taskID,
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		FilePath:  filePath,
	}

	// 限制日志数量
	if len(m.logs) >= 10000 {
		m.logs = m.logs[1:]
	}
	m.logs = append(m.logs, log)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
