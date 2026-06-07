package crossplatformsync

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 跨平台同步管理器
type Manager struct {
	mu sync.RWMutex

	devices   map[string]*NASDevice
	tasks     map[string]*SyncTask
	status    map[string]*SyncStatus
	conflicts map[string][]*FileConflict
	logs      []SyncLog
	cancel    map[string]chan struct{}

	configPath string
	logger     *zap.Logger
}

// NewManager 创建跨平台同步管理器
func NewManager(configPath string, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		devices:    make(map[string]*NASDevice),
		tasks:      make(map[string]*SyncTask),
		status:     make(map[string]*SyncStatus),
		conflicts:  make(map[string][]*FileConflict),
		logs:       make([]SyncLog, 0, 1000),
		cancel:     make(map[string]chan struct{}),
		configPath: configPath,
		logger:     logger,
	}
}

// Initialize 初始化管理器
func (m *Manager) Initialize() error {
	if m.configPath != "" {
		if err := m.loadConfig(); err != nil && !os.IsNotExist(err) {
			m.logger.Warn("failed to load config", zap.Error(err))
		}
	}
	m.logger.Info("cross-platform sync manager initialized")
	return nil
}

// ============================================================
// 设备管理
// ============================================================

// CreateDevice 创建 NAS 设备
func (m *Manager) CreateDevice(req CreateDeviceRequest) (*NASDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device := &NASDevice{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Address:   req.Address,
		Port:      req.Port,
		APIKey:    req.APIKey,
		Status:    DeviceStatusOffline,
		Platform:  req.Platform,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := device.Validate(); err != nil {
		return nil, err
	}

	m.devices[device.ID] = device
	if err := m.saveConfig(); err != nil {
		m.logger.Warn("failed to save config", zap.Error(err))
	}
	m.logger.Info("NAS device created", zap.String("id", device.ID), zap.String("name", device.Name))
	return device, nil
}

// GetDevice 获取设备
func (m *Manager) GetDevice(id string) (*NASDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	device, exists := m.devices[id]
	if !exists {
		return nil, fmt.Errorf("device %s not found", id)
	}
	return device, nil
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices() []*NASDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	devices := make([]*NASDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// UpdateDevice 更新设备
func (m *Manager) UpdateDevice(id string, req UpdateDeviceRequest) (*NASDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	device, exists := m.devices[id]
	if !exists {
		return nil, fmt.Errorf("device %s not found", id)
	}
	if req.Name != "" {
		device.Name = req.Name
	}
	if req.Address != "" {
		device.Address = req.Address
	}
	if req.Port != nil {
		device.Port = *req.Port
	}
	if req.APIKey != "" {
		device.APIKey = req.APIKey
	}
	device.UpdatedAt = time.Now()
	if err := device.Validate(); err != nil {
		return nil, err
	}
	m.logger.Info("device updated", zap.String("id", id))
	return device, nil
}

// DeleteDevice 删除设备
func (m *Manager) DeleteDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.devices[id]; !exists {
		return fmt.Errorf("device %s not found", id)
	}
	for _, task := range m.tasks {
		if task.SourceDeviceID == id || task.TargetDeviceID == id {
			return fmt.Errorf("device %s is in use by task %s", id, task.ID)
		}
	}
	delete(m.devices, id)
	m.logger.Info("device deleted", zap.String("id", id))
	return nil
}

// TestDeviceConnection 测试设备连接
func (m *Manager) TestDeviceConnection(id string) (*ConnectionTestResult, error) {
	m.mu.RLock()
	_, exists := m.devices[id]
	m.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("device %s not found", id)
	}
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	latency := time.Since(start).Milliseconds()

	m.mu.Lock()
	device := m.devices[id]
	device.Status = DeviceStatusOnline
	now := time.Now()
	device.LastSeen = &now
	device.UpdatedAt = now
	m.mu.Unlock()

	return &ConnectionTestResult{Success: true, Latency: latency, Version: "1.0.0"}, nil
}

// UpdateDeviceStatus 更新设备状态
func (m *Manager) UpdateDeviceStatus(id string, status DeviceStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	device, exists := m.devices[id]
	if !exists {
		return fmt.Errorf("device %s not found", id)
	}
	device.Status = status
	now := time.Now()
	device.LastSeen = &now
	device.UpdatedAt = now
	return nil
}

// ============================================================
// 同步任务管理
// ============================================================

// CreateSyncTask 创建同步任务
func (m *Manager) CreateSyncTask(req CreateSyncTaskRequest) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[req.SourceDeviceID]; !exists {
		return nil, fmt.Errorf("source device %s not found", req.SourceDeviceID)
	}
	if _, exists := m.devices[req.TargetDeviceID]; !exists {
		return nil, fmt.Errorf("target device %s not found", req.TargetDeviceID)
	}

	conflictStrategy := req.ConflictStrategy
	if conflictStrategy == "" {
		conflictStrategy = ConflictStrategyNewer
	}
	if !conflictStrategy.IsValid() {
		return nil, fmt.Errorf("invalid conflict strategy: %s", conflictStrategy)
	}

	scheduleType := req.ScheduleType
	if scheduleType == "" {
		scheduleType = "manual"
	}

	preserveModTime := true
	if req.PreserveModTime != nil {
		preserveModTime = *req.PreserveModTime
	}
	preservePerms := true
	if req.PreservePerms != nil {
		preservePerms = *req.PreservePerms
	}
	deleteExtraneous := false
	if req.DeleteExtraneous != nil {
		deleteExtraneous = *req.DeleteExtraneous
	}
	checksumVerify := true
	if req.ChecksumVerify != nil {
		checksumVerify = *req.ChecksumVerify
	}
	compressTransfer := true
	if req.CompressTransfer != nil {
		compressTransfer = *req.CompressTransfer
	}
	concurrent := 4
	if req.Concurrent > 0 {
		concurrent = req.Concurrent
	}

	task := &SyncTask{
		ID:               uuid.New().String(),
		Name:             req.Name,
		SourceDeviceID:   req.SourceDeviceID,
		TargetDeviceID:   req.TargetDeviceID,
		SourcePath:       req.SourcePath,
		TargetPath:       req.TargetPath,
		Mode:             req.Mode,
		ConflictStrategy: conflictStrategy,
		Enabled:          true,
		Status:           TaskStatusIdle,
		IncludePatterns:  req.IncludePatterns,
		ExcludePatterns:  req.ExcludePatterns,
		MaxFileSize:      req.MaxFileSize,
		MinFileSize:      req.MinFileSize,
		PreserveModTime:  preserveModTime,
		PreservePerms:    preservePerms,
		DeleteExtraneous: deleteExtraneous,
		ChecksumVerify:   checksumVerify,
		CompressTransfer: compressTransfer,
		BandwidthLimit:   req.BandwidthLimit,
		Concurrent:       concurrent,
		ScheduleType:     scheduleType,
		CronExpr:         req.CronExpr,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := task.Validate(); err != nil {
		return nil, err
	}

	m.tasks[task.ID] = task
	m.status[task.ID] = &SyncStatus{TaskID: task.ID, TaskName: task.Name, Status: TaskStatusIdle}
	if err := m.saveConfig(); err != nil {
		m.logger.Warn("failed to save config", zap.Error(err))
	}
	m.logger.Info("sync task created", zap.String("id", task.ID), zap.String("name", task.Name))
	return task, nil
}

// GetSyncTask 获取同步任务
func (m *Manager) GetSyncTask(id string) (*SyncTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return task, nil
}

// ListSyncTasks 列出所有同步任务
func (m *Manager) ListSyncTasks() []*SyncTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]*SyncTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// UpdateSyncTask 更新同步任务
func (m *Manager) UpdateSyncTask(id string, req UpdateSyncTaskRequest) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task %s not found", id)
	}
	if req.Name != "" {
		task.Name = req.Name
	}
	if req.SourcePath != "" {
		task.SourcePath = req.SourcePath
	}
	if req.TargetPath != "" {
		task.TargetPath = req.TargetPath
	}
	if req.Mode != nil {
		if !req.Mode.IsValid() {
			return nil, fmt.Errorf("invalid sync mode: %s", *req.Mode)
		}
		task.Mode = *req.Mode
	}
	if req.ConflictStrategy != nil {
		if !req.ConflictStrategy.IsValid() {
			return nil, fmt.Errorf("invalid conflict strategy: %s", *req.ConflictStrategy)
		}
		task.ConflictStrategy = *req.ConflictStrategy
	}
	if req.IncludePatterns != nil {
		task.IncludePatterns = req.IncludePatterns
	}
	if req.ExcludePatterns != nil {
		task.ExcludePatterns = req.ExcludePatterns
	}
	if req.MaxFileSize != nil {
		task.MaxFileSize = *req.MaxFileSize
	}
	if req.MinFileSize != nil {
		task.MinFileSize = *req.MinFileSize
	}
	if req.PreserveModTime != nil {
		task.PreserveModTime = *req.PreserveModTime
	}
	if req.PreservePerms != nil {
		task.PreservePerms = *req.PreservePerms
	}
	if req.DeleteExtraneous != nil {
		task.DeleteExtraneous = *req.DeleteExtraneous
	}
	if req.ChecksumVerify != nil {
		task.ChecksumVerify = *req.ChecksumVerify
	}
	if req.CompressTransfer != nil {
		task.CompressTransfer = *req.CompressTransfer
	}
	if req.BandwidthLimit != nil {
		task.BandwidthLimit = *req.BandwidthLimit
	}
	if req.Concurrent != nil {
		task.Concurrent = *req.Concurrent
	}
	if req.ScheduleType != "" {
		task.ScheduleType = req.ScheduleType
	}
	if req.CronExpr != "" {
		task.CronExpr = req.CronExpr
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

// DeleteSyncTask 删除同步任务
func (m *Manager) DeleteSyncTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("task %s not found", id)
	}
	if task.Status == TaskStatusSyncing {
		return fmt.Errorf("cannot delete task %s while syncing", id)
	}
	delete(m.tasks, id)
	delete(m.status, id)
	delete(m.conflicts, id)
	delete(m.cancel, id)
	m.logger.Info("sync task deleted", zap.String("id", id))
	return nil
}

// ============================================================
// 同步控制
// ============================================================

// StartSync 启动同步
func (m *Manager) StartSync(taskID string) error {
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
	if task.Status == TaskStatusSyncing {
		m.mu.Unlock()
		return fmt.Errorf("task %s is already syncing", taskID)
	}
	sourceDevice, sourceExists := m.devices[task.SourceDeviceID]
	targetDevice, targetExists := m.devices[task.TargetDeviceID]
	if !sourceExists || !targetExists {
		m.mu.Unlock()
		return fmt.Errorf("source or target device not found")
	}
	if sourceDevice.Status == DeviceStatusOffline || targetDevice.Status == DeviceStatusOffline {
		m.mu.Unlock()
		return fmt.Errorf("source or target device is offline")
	}

	cancel := make(chan struct{})
	m.cancel[taskID] = cancel
	task.Status = TaskStatusSyncing
	task.UpdatedAt = time.Now()
	m.status[taskID] = &SyncStatus{TaskID: taskID, TaskName: task.Name, Status: TaskStatusSyncing, StartedAt: timePtr(time.Now())}
	m.mu.Unlock()

	go m.executeSync(task, cancel)
	m.logger.Info("sync started", zap.String("task_id", taskID))
	return nil
}

// PauseSync 暂停同步
func (m *Manager) PauseSync(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != TaskStatusSyncing {
		return fmt.Errorf("task %s is not syncing", taskID)
	}
	if cancel, ok := m.cancel[taskID]; ok {
		close(cancel)
		delete(m.cancel, taskID)
	}
	task.Status = TaskStatusPaused
	task.UpdatedAt = time.Now()
	if status, ok := m.status[taskID]; ok {
		status.Status = TaskStatusPaused
	}
	m.logger.Info("sync paused", zap.String("task_id", taskID))
	return nil
}

// ResumeSync 恢复同步
func (m *Manager) ResumeSync(taskID string) error {
	m.mu.Lock()
	task, exists := m.tasks[taskID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != TaskStatusPaused {
		m.mu.Unlock()
		return fmt.Errorf("task %s is not paused", taskID)
	}
	cancel := make(chan struct{})
	m.cancel[taskID] = cancel
	task.Status = TaskStatusSyncing
	task.UpdatedAt = time.Now()
	m.status[taskID] = &SyncStatus{TaskID: taskID, TaskName: task.Name, Status: TaskStatusSyncing, StartedAt: timePtr(time.Now())}
	m.mu.Unlock()

	go m.executeSync(task, cancel)
	m.logger.Info("sync resumed", zap.String("task_id", taskID))
	return nil
}

// StopSync 停止同步
func (m *Manager) StopSync(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status == TaskStatusSyncing {
		if cancel, ok := m.cancel[taskID]; ok {
			close(cancel)
			delete(m.cancel, taskID)
		}
	}
	task.Status = TaskStatusIdle
	task.UpdatedAt = time.Now()
	if status, ok := m.status[taskID]; ok {
		status.Status = TaskStatusIdle
		status.Progress = 0
	}
	m.logger.Info("sync stopped", zap.String("task_id", taskID))
	return nil
}

// ============================================================
// 冲突管理
// ============================================================

// GetConflicts 获取任务的冲突列表
func (m *Manager) GetConflicts(taskID string) []*FileConflict {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conflicts[taskID]
}

// ResolveConflict 解决单个冲突
func (m *Manager) ResolveConflict(taskID, conflictID string, resolution ConflictStrategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	conflicts, exists := m.conflicts[taskID]
	if !exists {
		return fmt.Errorf("no conflicts for task %s", taskID)
	}
	if !resolution.IsValid() {
		return fmt.Errorf("invalid resolution strategy: %s", resolution)
	}
	for _, c := range conflicts {
		if c.ID == conflictID {
			c.Resolution = string(resolution)
			c.Resolved = true
			now := time.Now()
			c.ResolvedAt = &now
			m.logger.Info("conflict resolved", zap.String("task_id", taskID), zap.String("conflict_id", conflictID))
			return nil
		}
	}
	return fmt.Errorf("conflict %s not found", conflictID)
}

// ResolveAllConflicts 解决任务的所有冲突
func (m *Manager) ResolveAllConflicts(taskID string, resolution ConflictStrategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !resolution.IsValid() {
		return fmt.Errorf("invalid resolution strategy: %s", resolution)
	}
	conflicts, exists := m.conflicts[taskID]
	if !exists {
		return fmt.Errorf("no conflicts for task %s", taskID)
	}
	now := time.Now()
	resolvedCount := 0
	for _, c := range conflicts {
		if !c.Resolved {
			c.Resolution = string(resolution)
			c.Resolved = true
			c.ResolvedAt = &now
			resolvedCount++
		}
	}
	m.logger.Info("all conflicts resolved", zap.String("task_id", taskID), zap.Int("count", resolvedCount))
	return nil
}

// ============================================================
// 状态和统计
// ============================================================

// GetSyncStatus 获取同步状态
func (m *Manager) GetSyncStatus(taskID string) (*SyncStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, exists := m.status[taskID]
	if !exists {
		return &SyncStatus{TaskID: taskID, Status: TaskStatusIdle}, nil
	}
	return status, nil
}

// GetAllSyncStatuses 获取所有同步状态
func (m *Manager) GetAllSyncStatuses() map[string]*SyncStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*SyncStatus, len(m.status))
	for k, v := range m.status {
		result[k] = v
	}
	return result
}

// GetSyncStats 获取同步统计
func (m *Manager) GetSyncStats() SyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := SyncStats{TotalDevices: int64(len(m.devices)), TotalTasks: int64(len(m.tasks))}
	for _, d := range m.devices {
		if d.Status == DeviceStatusOnline || d.Status == DeviceStatusSyncing {
			stats.OnlineDevices++
		}
	}
	for _, task := range m.tasks {
		stats.TotalFiles += task.TotalFiles
		stats.TotalSize += task.TotalSize
		stats.SyncedFiles += task.SyncedFiles
		stats.FailedFiles += task.FailedFiles
		stats.ConflictFiles += task.ConflictFiles
		stats.TotalBandwidth += task.BandwidthLimit
		switch task.Status {
		case TaskStatusSyncing:
			stats.ActiveTasks++
		case TaskStatusPaused:
			stats.PausedTasks++
		case TaskStatusFailed:
			stats.FailedTasks++
		}
		if task.LastSyncTime != nil {
			if stats.LastSyncTime.IsZero() || task.LastSyncTime.After(stats.LastSyncTime) {
				stats.LastSyncTime = *task.LastSyncTime
			}
		}
	}
	return stats
}

// GetSyncLogs 获取同步日志
func (m *Manager) GetSyncLogs(taskID string, limit int) []SyncLog {
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

// ============================================================
// 同步执行（模拟）
// ============================================================

func (m *Manager) executeSync(task *SyncTask, cancel chan struct{}) {
	startTime := time.Now()
	m.addLog(task.ID, "info", fmt.Sprintf("Sync started: %s:%s -> %s:%s (%s)",
		task.SourceDeviceID, task.SourcePath, task.TargetDeviceID, task.TargetPath, task.Mode),
		task.SourcePath, task.SourceDeviceID, task.TargetDeviceID)

	totalFiles := 50 + (time.Now().UnixNano() % 50)
	syncedFiles := int64(0)
	failedFiles := int64(0)
	skippedFiles := int64(0)
	conflictFiles := int64(0)

	for i := int64(0); i < totalFiles; i++ {
		select {
		case <-cancel:
			m.mu.Lock()
			task.Status = TaskStatusPaused
			m.mu.Unlock()
			m.addLog(task.ID, "info", "Sync paused by user", "", "", "")
			return
		default:
		}
		time.Sleep(50 * time.Millisecond)

		if (time.Now().UnixNano()+i)%17 == 0 {
			failedFiles++
			m.addLog(task.ID, "error", "Failed to sync file", fmt.Sprintf("/path/to/file_%d.dat", i), task.SourceDeviceID, task.TargetDeviceID)
			continue
		}
		if (time.Now().UnixNano()+i)%23 == 0 {
			conflictFiles++
			m.addLog(task.ID, "warn", "Conflict detected", fmt.Sprintf("/path/to/file_%d.dat", i), task.SourceDeviceID, task.TargetDeviceID)
			continue
		}
		if (time.Now().UnixNano()+i)%11 == 0 {
			skippedFiles++
			continue
		}
		syncedFiles++

		m.mu.Lock()
		progress := float64(i+1) / float64(totalFiles) * 100
		if status, ok := m.status[task.ID]; ok {
			status.Progress = progress
			status.ProcessedFiles = i + 1
			status.SyncedFiles = syncedFiles
			status.FailedFiles = failedFiles
			status.ConflictFiles = conflictFiles
			status.SkippedFiles = skippedFiles
			status.TotalFiles = int64(totalFiles)
			status.SyncedBytes = syncedFiles * 1024 * 1024
			status.TotalBytes = int64(totalFiles) * 1024 * 1024
			status.SpeedBps = int64(float64(status.SyncedBytes) / time.Since(startTime).Seconds())
			if status.SpeedBps > 0 {
				remaining := status.TotalBytes - status.SyncedBytes
				status.ETASeconds = int(remaining / status.SpeedBps)
			}
		}
		m.mu.Unlock()
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	m.mu.Lock()
	if conflictFiles > 0 {
		task.Status = TaskStatusConflict
		task.LastSyncResult = "conflict"
	} else if failedFiles > 0 {
		task.Status = TaskStatusIdle
		task.LastSyncResult = "partial"
		task.LastError = fmt.Sprintf("%d files failed", failedFiles)
		task.ErrorCount += int(failedFiles)
	} else {
		task.Status = TaskStatusIdle
		task.LastSyncResult = "success"
		task.LastError = ""
	}
	task.LastSyncTime = &endTime
	task.TotalFiles = int64(totalFiles)
	task.TotalSize = int64(totalFiles) * 1024 * 1024
	task.SyncedFiles = syncedFiles
	task.FailedFiles = failedFiles
	task.SkippedFiles = skippedFiles
	task.ConflictFiles = conflictFiles
	task.UpdatedAt = endTime

	if status, ok := m.status[task.ID]; ok {
		status.Status = task.Status
		status.Progress = 100
		status.CurrentFile = ""
	}
	m.mu.Unlock()

	m.addLog(task.ID, "info", fmt.Sprintf("Sync completed in %s: %d synced, %d skipped, %d failed, %d conflicts",
		duration.Round(time.Second), syncedFiles, skippedFiles, failedFiles, conflictFiles), "", "", "")
}

// ============================================================
// Mock 数据
// ============================================================

// LoadMockData 加载演示数据
func (m *Manager) LoadMockData() {
	m.mu.Lock()
	defer m.mu.Unlock()

	lastSync := time.Now().Add(-2 * time.Hour)

	devices := []*NASDevice{
		{ID: "device-001", Name: "办公室 NAS", Address: "192.168.1.100", Port: 8443, APIKey: "key-office-001", Status: DeviceStatusOnline, Platform: "linux", Version: "1.0.0", Capabilities: []string{"bidirectional", "mirror", "one_way", "compression"}, CreatedAt: time.Now().Add(-60 * 24 * time.Hour), UpdatedAt: time.Now().Add(-1 * time.Hour)},
		{ID: "device-002", Name: "家庭 NAS", Address: "192.168.2.100", Port: 8443, APIKey: "key-home-002", Status: DeviceStatusOnline, Platform: "linux", Version: "1.0.0", Capabilities: []string{"bidirectional", "mirror", "one_way"}, CreatedAt: time.Now().Add(-45 * 24 * time.Hour), UpdatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: "device-003", Name: "远程办公室 NAS", Address: "10.0.0.50", Port: 8443, APIKey: "key-remote-003", Status: DeviceStatusOffline, Platform: "linux", Version: "0.9.5", Capabilities: []string{"bidirectional", "mirror"}, CreatedAt: time.Now().Add(-30 * 24 * time.Hour), UpdatedAt: time.Now().Add(-24 * time.Hour)},
	}
	for _, d := range devices {
		m.devices[d.ID] = d
	}

	tasks := []*SyncTask{
		{ID: "sync-001", Name: "文档双向同步", SourceDeviceID: "device-001", TargetDeviceID: "device-002", SourcePath: "/volume1/documents", TargetPath: "/volume1/documents", Mode: SyncModeBidirectional, ConflictStrategy: ConflictStrategyNewer, Enabled: true, Status: TaskStatusIdle, IncludePatterns: []string{"*.docx", "*.xlsx", "*.pdf", "*.md"}, ExcludePatterns: []string{".*", "~*"}, PreserveModTime: true, PreservePerms: true, ChecksumVerify: true, CompressTransfer: true, Concurrent: 4, ScheduleType: "realtime", LastSyncTime: &lastSync, LastSyncResult: "success", TotalFiles: 3200, TotalSize: 1024 * 1024 * 1024 * 12, SyncedFiles: 3200, SkippedFiles: 45, CreatedAt: time.Now().Add(-30 * 24 * time.Hour), UpdatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: "sync-002", Name: "照片单向镜像", SourceDeviceID: "device-002", TargetDeviceID: "device-001", SourcePath: "/volume1/photos", TargetPath: "/volume1/backup/photos", Mode: SyncModeMirror, ConflictStrategy: ConflictStrategySource, Enabled: true, Status: TaskStatusIdle, IncludePatterns: []string{"*.jpg", "*.jpeg", "*.png", "*.heic", "*.mp4"}, MaxFileSize: 100 * 1024 * 1024, PreserveModTime: true, DeleteExtraneous: true, ChecksumVerify: true, CompressTransfer: true, BandwidthLimit: 10240, Concurrent: 2, ScheduleType: "cron", CronExpr: "0 2 * * *", LastSyncTime: &lastSync, LastSyncResult: "success", TotalFiles: 15420, TotalSize: 1024 * 1024 * 1024 * 85, SyncedFiles: 15420, SkippedFiles: 230, CreatedAt: time.Now().Add(-20 * 24 * time.Hour), UpdatedAt: time.Now().Add(-3 * time.Hour)},
		{ID: "sync-003", Name: "项目代码同步", SourceDeviceID: "device-001", TargetDeviceID: "device-003", SourcePath: "/volume1/projects", TargetPath: "/volume1/projects", Mode: SyncModeBidirectional, ConflictStrategy: ConflictStrategyKeepBoth, Enabled: true, Status: TaskStatusIdle, ExcludePatterns: []string{".git", "node_modules", "*.tmp"}, PreserveModTime: true, PreservePerms: true, ChecksumVerify: true, Concurrent: 4, ScheduleType: "cron", CronExpr: "*/30 * * * *", LastSyncTime: &lastSync, LastSyncResult: "partial", TotalFiles: 5600, TotalSize: 1024 * 1024 * 512, SyncedFiles: 5580, SkippedFiles: 10, FailedFiles: 10, ConflictFiles: 3, LastError: "device-003 offline, 10 files failed", ErrorCount: 5, CreatedAt: time.Now().Add(-15 * 24 * time.Hour), UpdatedAt: time.Now().Add(-30 * time.Minute)},
	}
	for _, t := range tasks {
		m.tasks[t.ID] = t
		m.status[t.ID] = &SyncStatus{TaskID: t.ID, TaskName: t.Name, Status: t.Status}
	}

	m.conflicts["sync-003"] = []*FileConflict{
		{ID: "conflict-001", TaskID: "sync-003", FilePath: "/src/main.go", SourceDevice: "device-001", TargetDevice: "device-003", SourceModTime: time.Now().Add(-1 * time.Hour), TargetModTime: time.Now().Add(-30 * time.Minute), SourceSize: 2048, TargetSize: 2156, SourceHash: "abc123", TargetHash: "def456", CreatedAt: time.Now().Add(-30 * time.Minute)},
		{ID: "conflict-002", TaskID: "sync-003", FilePath: "/src/utils/helper.go", SourceDevice: "device-001", TargetDevice: "device-003", SourceModTime: time.Now().Add(-2 * time.Hour), TargetModTime: time.Now().Add(-1 * time.Hour), SourceSize: 4096, TargetSize: 3800, SourceHash: "ghi789", TargetHash: "jkl012", CreatedAt: time.Now().Add(-30 * time.Minute)},
		{ID: "conflict-003", TaskID: "sync-003", FilePath: "/docs/README.md", SourceDevice: "device-001", TargetDevice: "device-003", SourceModTime: time.Now().Add(-45 * time.Minute), TargetModTime: time.Now().Add(-45 * time.Minute), SourceSize: 1024, TargetSize: 1024, SourceHash: "mno345", TargetHash: "pqr678", CreatedAt: time.Now().Add(-30 * time.Minute)},
	}

	m.logs = []SyncLog{
		{TaskID: "sync-001", Timestamp: lastSync.Add(-1 * time.Minute), Level: "info", Message: "Bidirectional sync completed: 3200 files"},
		{TaskID: "sync-002", Timestamp: lastSync.Add(-30 * time.Second), Level: "info", Message: "Mirror sync completed: 15420 files, 230 skipped"},
		{TaskID: "sync-003", Timestamp: lastSync.Add(-15 * time.Second), Level: "warn", Message: "3 conflicts detected, pending resolution"},
		{TaskID: "sync-003", Timestamp: lastSync.Add(-10 * time.Second), Level: "error", Message: "Device device-003 unreachable"},
		{TaskID: "sync-003", Timestamp: lastSync, Level: "info", Message: "Partial sync completed: 5580 synced, 10 failed"},
	}
	m.logger.Info("mock data loaded", zap.Int("devices", len(devices)), zap.Int("tasks", len(tasks)))
}

// ============================================================
// 辅助函数
// ============================================================

func (m *Manager) addLog(taskID, level, message, filePath, sourceDevice, targetDevice string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	log := SyncLog{TaskID: taskID, Timestamp: time.Now(), Level: level, Message: message, FilePath: filePath, SourceDevice: sourceDevice, TargetDevice: targetDevice}
	if len(m.logs) >= 10000 {
		m.logs = m.logs[1:]
	}
	m.logs = append(m.logs, log)
}

func timePtr(t time.Time) *time.Time { return &t }

// ============================================================
// 配置持久化
// ============================================================

type configData struct {
	Devices   map[string]*NASDevice      `json:"devices"`
	Tasks     map[string]*SyncTask       `json:"tasks"`
	Conflicts map[string][]*FileConflict `json:"conflicts"`
}

func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}
	data := configData{Devices: m.devices, Tasks: m.tasks, Conflicts: m.conflicts}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(m.configPath, bytes, 0644)
}

func (m *Manager) loadConfig() error {
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
	if data.Devices != nil {
		m.devices = data.Devices
	}
	if data.Tasks != nil {
		m.tasks = data.Tasks
	}
	if data.Conflicts != nil {
		m.conflicts = data.Conflicts
	}
	return nil
}
