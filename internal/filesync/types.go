// Package filesync - 文件同步模块
// 对标群晖 Synology Drive，支持多端文件同步、版本控制、冲突解决
package filesync

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SyncTask 同步任务
type SyncTask struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	LocalPath   string            `json:"local_path"`
	RemotePath  string            `json:"remote_path"`
	DeviceID    string            `json:"device_id"`
	Direction   string            `json:"direction"` // bidirectional, upload, download
	Status      string            `json:"status"`    // idle, syncing, paused, error
	Enabled     bool              `json:"enabled"`
	LastSync    time.Time         `json:"last_sync"`
	LastError   string            `json:"last_error,omitempty"`
	FileCount   int               `json:"file_count"`
	TotalSize   int64             `json:"total_size_bytes"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// SyncFile 同步文件记录
type SyncFile struct {
	ID           string    `json:"id"`
	TaskID       string    `json:"task_id"`
	Path         string    `json:"path"`
	Size         int64     `json:"size_bytes"`
	Checksum     string    `json:"checksum"`
	ModTime      time.Time `json:"mod_time"`
	Version      int       `json:"version"`
	ConflictFlag bool      `json:"conflict"`
	SyncStatus   string    `json:"sync_status"` // synced, pending, conflict, error
}

// SyncConflict 同步冲突
type SyncConflict struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	FilePath    string    `json:"file_path"`
	LocalModTime time.Time `json:"local_mod_time"`
	RemoteModTime time.Time `json:"remote_mod_time"`
	LocalSize   int64     `json:"local_size"`
	RemoteSize  int64     `json:"remote_size"`
	Resolution  string    `json:"resolution"` // keep_local, keep_remote, keep_both, pending
	ResolvedAt  time.Time `json:"resolved_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// SyncVersion 文件版本
type SyncVersion struct {
	ID        string    `json:"id"`
	FileID    string    `json:"file_id"`
	Version   int       `json:"version"`
	Size      int64     `json:"size_bytes"`
	Checksum  string    `json:"checksum"`
	ModTime   time.Time `json:"mod_time"`
	Comment   string    `json:"comment,omitempty"`
	CreatedBy string    `json:"created_by"`
}

// DeviceInfo 设备信息
type DeviceInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // desktop, mobile, server
	OS          string    `json:"os"`
	LastSeen    time.Time `json:"last_seen"`
	Online      bool      `json:"online"`
	SyncEnabled bool      `json:"sync_enabled"`
}

// SyncStats 同步统计
type SyncStats struct {
	TotalFiles    int   `json:"total_files"`
	SyncedFiles   int   `json:"synced_files"`
	PendingFiles  int   `json:"pending_files"`
	ConflictFiles int   `json:"conflict_files"`
	ErrorFiles    int   `json:"error_files"`
	TotalSize     int64 `json:"total_size_bytes"`
	LastSyncTime  time.Time `json:"last_sync_time"`
}

// SyncManager 文件同步管理器
type SyncManager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	storagePath string

	tasks     map[string]*SyncTask
	files     map[string][]*SyncFile     // taskID -> files
	conflicts map[string][]*SyncConflict // taskID -> conflicts
	versions  map[string][]*SyncVersion  // fileID -> versions
	devices   map[string]*DeviceInfo

	// 同步控制
	syncCtx    context.Context
	syncCancel context.CancelFunc
}

// NewSyncManager 创建同步管理器
func NewSyncManager(logger *zap.Logger, storagePath string) *SyncManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &SyncManager{
		logger:      logger,
		storagePath: storagePath,
		tasks:       make(map[string]*SyncTask),
		files:       make(map[string][]*SyncFile),
		conflicts:   make(map[string][]*SyncConflict),
		versions:    make(map[string][]*SyncVersion),
		devices:     make(map[string]*DeviceInfo),
		syncCtx:     ctx,
		syncCancel:  cancel,
	}
}

// CreateTask 创建同步任务
func (sm *SyncManager) CreateTask(ctx context.Context, task *SyncTask) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if task.ID == "" {
		task.ID = fmt.Sprintf("sync-%d", time.Now().UnixNano())
	}
	if _, exists := sm.tasks[task.ID]; exists {
		return fmt.Errorf("task %s already exists", task.ID)
	}

	task.Status = "idle"
	task.Enabled = true
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	sm.tasks[task.ID] = task

	sm.logger.Info("同步任务已创建", zap.String("id", task.ID), zap.String("name", task.Name))
	return nil
}

// UpdateTask 更新同步任务
func (sm *SyncManager) UpdateTask(ctx context.Context, task *SyncTask) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	existing, exists := sm.tasks[task.ID]
	if !exists {
		return fmt.Errorf("task %s not found", task.ID)
	}

	task.CreatedAt = existing.CreatedAt
	task.UpdatedAt = time.Now()
	sm.tasks[task.ID] = task
	return nil
}

// DeleteTask 删除同步任务
func (sm *SyncManager) DeleteTask(ctx context.Context, taskID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.tasks[taskID]; !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	delete(sm.tasks, taskID)
	delete(sm.files, taskID)
	delete(sm.conflicts, taskID)

	sm.logger.Info("同步任务已删除", zap.String("id", taskID))
	return nil
}

// GetTask 获取同步任务
func (sm *SyncManager) GetTask(ctx context.Context, taskID string) (*SyncTask, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	task, exists := sm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return task, nil
}

// ListTasks 列出所有同步任务
func (sm *SyncManager) ListTasks(ctx context.Context) []*SyncTask {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	tasks := make([]*SyncTask, 0, len(sm.tasks))
	for _, t := range sm.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// StartSync 开始同步
func (sm *SyncManager) StartSync(ctx context.Context, taskID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	task, exists := sm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Status = "syncing"
	task.UpdatedAt = time.Now()

	sm.logger.Info("同步已开始", zap.String("task", taskID))
	return nil
}

// StopSync 停止同步
func (sm *SyncManager) StopSync(ctx context.Context, taskID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	task, exists := sm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Status = "paused"
	task.UpdatedAt = time.Now()

	sm.logger.Info("同步已暂停", zap.String("task", taskID))
	return nil
}

// RecordSyncFile 记录同步文件
func (sm *SyncManager) RecordSyncFile(ctx context.Context, file *SyncFile) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.tasks[file.TaskID]; !exists {
		return fmt.Errorf("task %s not found", file.TaskID)
	}

	if file.ID == "" {
		file.ID = fmt.Sprintf("file-%d", time.Now().UnixNano())
	}

	sm.files[file.TaskID] = append(sm.files[file.TaskID], file)
	return nil
}

// GetSyncFiles 获取同步文件列表
func (sm *SyncManager) GetSyncFiles(ctx context.Context, taskID string) []*SyncFile {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.files[taskID]
}

// ReportConflict 上报冲突
func (sm *SyncManager) ReportConflict(ctx context.Context, conflict *SyncConflict) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.tasks[conflict.TaskID]; !exists {
		return fmt.Errorf("task %s not found", conflict.TaskID)
	}

	if conflict.ID == "" {
		conflict.ID = fmt.Sprintf("conflict-%d", time.Now().UnixNano())
	}
	conflict.Resolution = "pending"
	conflict.CreatedAt = time.Now()

	sm.conflicts[conflict.TaskID] = append(sm.conflicts[conflict.TaskID], conflict)

	sm.logger.Warn("同步冲突",
		zap.String("task", conflict.TaskID),
		zap.String("file", conflict.FilePath))
	return nil
}

// ResolveConflict 解决冲突
func (sm *SyncManager) ResolveConflict(ctx context.Context, conflictID, resolution string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, conflicts := range sm.conflicts {
		for _, c := range conflicts {
			if c.ID == conflictID {
				c.Resolution = resolution
				c.ResolvedAt = time.Now()
				return nil
			}
		}
	}
	return fmt.Errorf("conflict %s not found", conflictID)
}

// GetConflicts 获取冲突列表
func (sm *SyncManager) GetConflicts(ctx context.Context, taskID string) []*SyncConflict {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.conflicts[taskID]
}

// AddVersion 添加文件版本
func (sm *SyncManager) AddVersion(ctx context.Context, version *SyncVersion) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if version.ID == "" {
		version.ID = fmt.Sprintf("ver-%d", time.Now().UnixNano())
	}

	sm.versions[version.FileID] = append(sm.versions[version.FileID], version)
	return nil
}

// GetVersions 获取文件版本历史
func (sm *SyncManager) GetVersions(ctx context.Context, fileID string) []*SyncVersion {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.versions[fileID]
}

// RegisterDevice 注册设备
func (sm *SyncManager) RegisterDevice(ctx context.Context, device *DeviceInfo) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if device.ID == "" {
		device.ID = fmt.Sprintf("dev-%d", time.Now().UnixNano())
	}

	device.LastSeen = time.Now()
	device.Online = true
	sm.devices[device.ID] = device

	sm.logger.Info("设备已注册", zap.String("id", device.ID), zap.String("name", device.Name))
	return nil
}

// ListDevices 列出设备
func (sm *SyncManager) ListDevices(ctx context.Context) []*DeviceInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	devices := make([]*DeviceInfo, 0, len(sm.devices))
	for _, d := range sm.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetSyncStats 获取同步统计
func (sm *SyncManager) GetSyncStats(ctx context.Context, taskID string) *SyncStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := &SyncStats{}
	files := sm.files[taskID]
	for _, f := range files {
		stats.TotalFiles++
		stats.TotalSize += f.Size
		switch f.SyncStatus {
		case "synced":
			stats.SyncedFiles++
		case "pending":
			stats.PendingFiles++
		case "conflict":
			stats.ConflictFiles++
		case "error":
			stats.ErrorFiles++
		}
	}

	task := sm.tasks[taskID]
	if task != nil {
		stats.LastSyncTime = task.LastSync
	}

	return stats
}

// CalculateChecksum 计算文件校验和
func CalculateChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ScanDirectory 扫描目录获取文件列表
func ScanDirectory(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// Stop 停止同步管理器
func (sm *SyncManager) Stop() {
	sm.syncCancel()
	sm.logger.Info("同步管理器已停止")
}
