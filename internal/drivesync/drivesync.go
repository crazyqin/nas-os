// Package drivesync Drive同步 - 文件同步与协作
// 对标群晖Synology Drive
package drivesync

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// SyncStatus 同步状态
type SyncStatus string

const (
	SyncStatusPending    SyncStatus = "pending"
	SyncStatusSyncing    SyncStatus = "syncing"
	SyncStatusSynced     SyncStatus = "synced"
	SyncStatusConflict   SyncStatus = "conflict"
	SyncStatusError      SyncStatus = "error"
	SyncStatusPaused     SyncStatus = "paused"
)

// ConflictResolution 冲突解决策略
type ConflictResolution string

const (
	ConflictKeepLocal    ConflictResolution = "keep_local"
	ConflictKeepRemote   ConflictResolution = "keep_remote"
	ConflictKeepBoth     ConflictResolution = "keep_both"
	ConflictAskUser      ConflictResolution = "ask_user"
	ConflictLastModified ConflictResolution = "last_modified"
)

// FileVersion 文件版本
type FileVersion struct {
	VersionID   string    `json:"version_id"`
	FilePath    string    `json:"file_path"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
	ModifiedBy  string    `json:"modified_by"`
	ModifiedAt  time.Time `json:"modified_at"`
	Comment     string    `json:"comment,omitempty"`
	IsCurrent   bool      `json:"is_current"`
}

// SyncTask 同步任务
type SyncTask struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	LocalPath       string             `json:"local_path"`
	RemotePath      string             `json:"remote_path"`
	Status          SyncStatus         `json:"status"`
	Direction       SyncDirection      `json:"direction"`
	ConflictPolicy  ConflictResolution `json:"conflict_policy"`
	Enabled         bool               `json:"enabled"`
	LastSyncAt      time.Time          `json:"last_sync_at"`
	NextSyncAt      time.Time          `json:"next_sync_at"`
	SyncInterval    int                `json:"sync_interval"` // 分钟
	LastError       string             `json:"last_error,omitempty"`
	FileCount       int                `json:"file_count"`
	TotalSize       int64              `json:"total_size"`
	SyncedCount     int                `json:"synced_count"`
	ConflictCount   int                `json:"conflict_count"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

// SyncDirection 同步方向
type SyncDirection string

const (
	DirectionBidirectional SyncDirection = "bidirectional"
	DirectionLocalToRemote SyncDirection = "local_to_remote"
	DirectionRemoteToLocal SyncDirection = "remote_to_local"
)

// SyncFile 同步文件记录
type SyncFile struct {
	ID           string     `json:"id"`
	TaskID       string     `json:"task_id"`
	FilePath     string     `json:"file_path"`
	LocalHash    string     `json:"local_hash"`
	RemoteHash   string     `json:"remote_hash"`
	Size         int64      `json:"size"`
	Status       SyncStatus `json:"status"`
	LastSyncAt   time.Time  `json:"last_sync_at"`
	ConflictWith string     `json:"conflict_with,omitempty"`
}

// DriveSyncManager Drive同步管理器
type DriveSyncManager struct {
	mu       sync.RWMutex
	tasks    map[string]*SyncTask
	files    map[string][]*SyncFile // taskID -> files
	versions map[string][]*FileVersion // filePath -> versions
	config   *DriveSyncConfig
}

// DriveSyncConfig Drive同步配置
type DriveSyncConfig struct {
	MaxVersions       int  `json:"max_versions"`
	AutoSync          bool `json:"auto_sync"`
	SyncInterval      int  `json:"sync_interval"` // 分钟
	MaxFileSize       int64 `json:"max_file_size"` // 字节
	ExcludePatterns   []string `json:"exclude_patterns"`
	EnableVersioning  bool `json:"enable_versioning"`
	EnableConflictLog bool `json:"enable_conflict_log"`
	BandwidthLimit    int  `json:"bandwidth_limit"` // KB/s, 0=无限
}

// DefaultDriveSyncConfig 默认配置
func DefaultDriveSyncConfig() *DriveSyncConfig {
	return &DriveSyncConfig{
		MaxVersions:       32,
		AutoSync:          true,
		SyncInterval:      5,
		MaxFileSize:       10 * 1024 * 1024 * 1024, // 10GB
		EnableVersioning:  true,
		EnableConflictLog: true,
		BandwidthLimit:    0,
	}
}

// NewDriveSyncManager 创建Drive同步管理器
func NewDriveSyncManager(config *DriveSyncConfig) *DriveSyncManager {
	if config == nil {
		config = DefaultDriveSyncConfig()
	}

	return &DriveSyncManager{
		tasks:    make(map[string]*SyncTask),
		files:    make(map[string][]*SyncFile),
		versions: make(map[string][]*FileVersion),
		config:   config,
	}
}

// CreateTask 创建同步任务
func (m *DriveSyncManager) CreateTask(task *SyncTask) error {
	if task == nil {
		return errors.New("task is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查路径是否已存在
	for _, existing := range m.tasks {
		if existing.LocalPath == task.LocalPath && existing.RemotePath == task.RemotePath {
			return errors.New("sync task already exists for these paths")
		}
	}

	// 设置默认值
	now := time.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	task.Status = SyncStatusPending

	if task.SyncInterval == 0 {
		task.SyncInterval = m.config.SyncInterval
	}

	if task.ConflictPolicy == "" {
		task.ConflictPolicy = ConflictLastModified
	}

	if task.Direction == "" {
		task.Direction = DirectionBidirectional
	}

	m.tasks[task.ID] = task

	return nil
}

// GetTask 获取同步任务
func (m *DriveSyncManager) GetTask(id string) (*SyncTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	return task, exists
}

// UpdateTask 更新同步任务
func (m *DriveSyncManager) UpdateTask(id string, update func(*SyncTask)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return errors.New("task not found: " + id)
	}

	update(task)
	task.UpdatedAt = time.Now()

	return nil
}

// DeleteTask 删除同步任务
func (m *DriveSyncManager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[id]; !exists {
		return errors.New("task not found: " + id)
	}

	delete(m.tasks, id)
	delete(m.files, id)

	return nil
}

// ListTasks 列出同步任务
func (m *DriveSyncManager) ListTasks() []*SyncTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*SyncTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// StartSync 开始同步
func (m *DriveSyncManager) StartSync(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return errors.New("task not found: " + taskID)
	}

	task.Status = SyncStatusSyncing
	task.UpdatedAt = time.Now()

	return nil
}

// CompleteSync 完成同步
func (m *DriveSyncManager) CompleteSync(taskID string, syncedCount, conflictCount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return errors.New("task not found: " + taskID)
	}

	task.Status = SyncStatusSynced
	task.LastSyncAt = time.Now()
	task.NextSyncAt = time.Now().Add(time.Duration(task.SyncInterval) * time.Minute)
	task.SyncedCount = syncedCount
	task.ConflictCount = conflictCount
	task.UpdatedAt = time.Now()

	return nil
}

// PauseSync 暂停同步
func (m *DriveSyncManager) PauseSync(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return errors.New("task not found: " + taskID)
	}

	task.Status = SyncStatusPaused
	task.UpdatedAt = time.Now()

	return nil
}

// ResumeSync 恢复同步
func (m *DriveSyncManager) ResumeSync(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return errors.New("task not found: " + taskID)
	}

	task.Status = SyncStatusPending
	task.UpdatedAt = time.Now()

	return nil
}

// SetSyncError 设置同步错误
func (m *DriveSyncManager) SetSyncError(taskID string, err string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return errors.New("task not found: " + taskID)
	}

	task.Status = SyncStatusError
	task.LastError = err
	task.UpdatedAt = time.Now()

	return nil
}

// AddSyncFile 添加同步文件记录
func (m *DriveSyncManager) AddSyncFile(taskID string, file *SyncFile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; !exists {
		return errors.New("task not found: " + taskID)
	}

	file.TaskID = taskID
	m.files[taskID] = append(m.files[taskID], file)

	return nil
}

// GetSyncFiles 获取同步文件列表
func (m *DriveSyncManager) GetSyncFiles(taskID string) []*SyncFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.files[taskID]
}

// AddFileVersion 添加文件版本
func (m *DriveSyncManager) AddFileVersion(filePath string, version *FileVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.versions[filePath]

	// 检查是否超过最大版本数
	if len(versions) >= m.config.MaxVersions {
		// 移除最旧的版本
		oldestIdx := 0
		oldestTime := versions[0].ModifiedAt
		for i, v := range versions {
			if v.ModifiedAt.Before(oldestTime) {
				oldestTime = v.ModifiedAt
				oldestIdx = i
			}
		}
		versions = append(versions[:oldestIdx], versions[oldestIdx+1:]...)
	}

	// 标记当前版本
	for _, v := range versions {
		v.IsCurrent = false
	}
	version.IsCurrent = true

	m.versions[filePath] = append(versions, version)

	return nil
}

// GetFileVersions 获取文件版本列表
func (m *DriveSyncManager) GetFileVersions(filePath string) []*FileVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.versions[filePath]
}

// CalculateChecksum 计算文件校验和
func CalculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// GetStats 获取统计信息
func (m *DriveSyncManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_tasks":     len(m.tasks),
		"active_tasks":    0,
		"paused_tasks":    0,
		"error_tasks":     0,
		"total_files":     0,
		"conflict_files":  0,
		"total_versions":  0,
	}

	for _, task := range m.tasks {
		switch task.Status {
		case SyncStatusSyncing, SyncStatusPending, SyncStatusSynced:
			stats["active_tasks"] = stats["active_tasks"].(int) + 1
		case SyncStatusPaused:
			stats["paused_tasks"] = stats["paused_tasks"].(int) + 1
		case SyncStatusError:
			stats["error_tasks"] = stats["error_tasks"].(int) + 1
		}
		stats["total_files"] = stats["total_files"].(int) + task.FileCount
		stats["conflict_files"] = stats["conflict_files"].(int) + task.ConflictCount
	}

	for _, files := range m.files {
		stats["total_files"] = stats["total_files"].(int) + len(files)
	}

	for _, versions := range m.versions {
		stats["total_versions"] = stats["total_versions"].(int) + len(versions)
	}

	return stats
}

// GetConfig 获取配置
func (m *DriveSyncManager) GetConfig() *DriveSyncConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// UpdateConfig 更新配置
func (m *DriveSyncManager) UpdateConfig(config *DriveSyncConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
}
