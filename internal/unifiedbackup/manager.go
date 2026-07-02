package unifiedbackup

import (
	"fmt"
	"sync"
	"time"
)

// Manager 统一备份管理器.
type Manager struct {
	mu            sync.RWMutex
	tasks         map[string]*BackupTask
	restorePoints map[string][]*RestorePoint
	restoreJobs   map[string]*RestoreJob
	config        *ManagerConfig
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	DefaultStoragePath string `json:"default_storage_path"`
	MaxConcurrentTasks int    `json:"max_concurrent_tasks"`
	RetentionDays      int    `json:"retention_days"`
}

// NewManager 创建管理器.
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = &ManagerConfig{
			DefaultStoragePath: "/var/lib/nas-os/backups",
			MaxConcurrentTasks: 3,
			RetentionDays:      30,
		}
	}
	return &Manager{
		tasks:         make(map[string]*BackupTask),
		restorePoints: make(map[string][]*RestorePoint),
		restoreJobs:   make(map[string]*RestoreJob),
		config:        config,
	}
}

// CreateTask 创建备份任务.
func (m *Manager) CreateTask(req *CreateTaskRequest) (*BackupTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("任务名称不能为空")
	}

	if req.Source.Type == "" {
		return nil, ErrInvalidSource
	}

	task := &BackupTask{
		ID:          fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Source:      req.Source,
		Mode:        req.Mode,
		Status:      TaskStatusPending,
		Schedule:    req.Schedule,
		Enabled:     req.Enabled,
		Retention:   req.Retention,
		Encryption:  req.Encryption,
		StoragePath: req.StoragePath,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if task.StoragePath == "" {
		task.StoragePath = m.config.DefaultStoragePath
	}

	m.tasks[task.ID] = task
	return task, nil
}

// GetTask 获取任务.
func (m *Manager) GetTask(id string) (*BackupTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有任务.
func (m *Manager) ListTasks() []*BackupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*BackupTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// UpdateTask 更新任务.
func (m *Manager) UpdateTask(id string, req *UpdateTaskRequest) (*BackupTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}

	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Mode != nil {
		task.Mode = *req.Mode
	}
	if req.Schedule != nil {
		task.Schedule = *req.Schedule
	}
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}
	if req.Retention != nil {
		task.Retention = *req.Retention
	}
	if req.Encryption != nil {
		task.Encryption = *req.Encryption
	}

	task.UpdatedAt = time.Now()
	return task, nil
}

// DeleteTask 删除任务.
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	if task.Status == TaskStatusRunning {
		return ErrTaskRunning
	}

	delete(m.tasks, id)
	delete(m.restorePoints, id)
	return nil
}

// RunTask 执行备份任务.
func (m *Manager) RunTask(id string) error {
	m.mu.Lock()
	task, exists := m.tasks[id]
	if !exists {
		m.mu.Unlock()
		return ErrTaskNotFound
	}
	if task.Status == TaskStatusRunning {
		m.mu.Unlock()
		return ErrTaskRunning
	}
	task.Status = TaskStatusRunning
	task.Progress = 0
	m.mu.Unlock()

	go m.executeBackup(task)
	return nil
}

// executeBackup 执行备份.
func (m *Manager) executeBackup(task *BackupTask) {
	startTime := time.Now()

	// 模拟备份过程
	for i := 0; i <= 100; i += 10 {
		m.mu.Lock()
		if task.Status == TaskStatusFailed {
			m.mu.Unlock()
			return
		}
		task.Progress = float64(i)
		m.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	// 创建恢复点
	restorePoint := &RestorePoint{
		ID:          fmt.Sprintf("rp_%d", time.Now().UnixNano()),
		TaskID:      task.ID,
		TaskName:    task.Name,
		SourceName:  task.Source.Name,
		Mode:        task.Mode,
		Size:        1024 * 1024 * 100, // 100MB 模拟
		FileCount:   100,
		Encrypted:   task.Encryption.Enabled,
		StoragePath: task.StoragePath,
		CreatedAt:   time.Now(),
	}

	m.mu.Lock()
	task.Status = TaskStatusCompleted
	task.Progress = 100
	task.LastStatus = TaskStatusCompleted
	now := time.Now()
	task.LastRunAt = &now
	task.RestorePointCount++
	m.restorePoints[task.ID] = append(m.restorePoints[task.ID], restorePoint)
	task.TotalSize += restorePoint.Size
	m.mu.Unlock()

	_ = time.Since(startTime)
}

// PauseTask 暂停任务.
func (m *Manager) PauseTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}
	if task.Status != TaskStatusRunning {
		return ErrTaskNotRunning
	}

	task.Status = TaskStatusPaused
	return nil
}

// ResumeTask 恢复任务.
func (m *Manager) ResumeTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}
	if task.Status != TaskStatusPaused {
		return ErrTaskNotPaused
	}

	task.Status = TaskStatusRunning
	return nil
}

// GetRestorePoints 获取恢复点.
func (m *Manager) GetRestorePoints(taskID string) ([]*RestorePoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.tasks[taskID]; !exists {
		return nil, ErrTaskNotFound
	}

	return m.restorePoints[taskID], nil
}

// Restore 执行恢复.
func (m *Manager) Restore(req *RestoreRequest) (*RestoreJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找恢复点
	var restorePoint *RestorePoint
	for _, points := range m.restorePoints {
		for _, point := range points {
			if point.ID == req.RestorePointID {
				restorePoint = point
				break
			}
		}
	}

	if restorePoint == nil {
		return nil, ErrRestorePointNotFound
	}

	job := &RestoreJob{
		ID:             fmt.Sprintf("restore_%d", time.Now().UnixNano()),
		RestorePointID: req.RestorePointID,
		TaskID:         restorePoint.TaskID,
		Type:           req.Type,
		Status:         TaskStatusRunning,
		TargetPath:     req.TargetPath,
		StartedAt:      time.Now(),
	}

	m.restoreJobs[job.ID] = job

	go m.executeRestore(job, restorePoint)
	return job, nil
}

// executeRestore 执行恢复.
func (m *Manager) executeRestore(job *RestoreJob, point *RestorePoint) {
	// 模拟恢复过程
	for i := 0; i <= 100; i += 20 {
		m.mu.Lock()
		job.Progress = float64(i)
		m.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	m.mu.Lock()
	job.Status = TaskStatusCompleted
	job.Progress = 100
	job.FilesRestored = point.FileCount
	job.TotalFiles = point.FileCount
	now := time.Now()
	job.CompletedAt = &now
	m.mu.Unlock()
}

// GetRestoreJob 获取恢复任务.
func (m *Manager) GetRestoreJob(id string) (*RestoreJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.restoreJobs[id]
	if !exists {
		return nil, fmt.Errorf("恢复任务不存在: %s", id)
	}
	return job, nil
}

// GetStorageStats 获取存储统计.
func (m *Manager) GetStorageStats() *StorageStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &StorageStats{
		TotalCapacity: 1024 * 1024 * 1024 * 100, // 100GB 模拟
		FreeSpace:     1024 * 1024 * 1024 * 60,  // 60GB 模拟
		TotalTasks:    len(m.tasks),
	}

	for _, points := range m.restorePoints {
		stats.TotalRestorePoints += len(points)
		for _, point := range points {
			stats.TotalBackupSize += point.Size
		}
	}

	stats.UsedSpace = stats.TotalBackupSize
	stats.UsagePercent = float64(stats.UsedSpace) / float64(stats.TotalCapacity) * 100

	return stats
}
