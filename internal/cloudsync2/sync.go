// Package cloudsync2 实现智能文件同步引擎 v2
// 学习群晖 Drive 高级同步功能，提供双向同步、冲突解决、版本控制
package cloudsync2

import (
	"fmt"
	"sync"
	"time"
)

// SyncMode 同步模式
type SyncMode string

const (
	// SyncModeBidirectional 双向同步
	SyncModeBidirectional SyncMode = "bidirectional"
	// SyncModeOneWay 单向同步
	SyncModeOneWay SyncMode = "one_way"
	// SyncModeMirror 镜像同步
	SyncModeMirror SyncMode = "mirror"
	// SyncModeIncremental 增量同步
	SyncModeIncremental SyncMode = "incremental"
)

// SyncStatus 同步状态
type SyncStatus string

const (
	// SyncStatusIdle 空闲
	SyncStatusIdle SyncStatus = "idle"
	// SyncStatusSyncing 同步中
	SyncStatusSyncing SyncStatus = "syncing"
	// SyncStatusPaused 暂停
	SyncStatusPaused SyncStatus = "paused"
	// SyncStatusError 错误
	SyncStatusError SyncStatus = "error"
	// SyncStatusConflict 冲突
	SyncStatusConflict SyncStatus = "conflict"
)

// ConflictResolution 冲突解决策略
type ConflictResolution string

const (
	// ConflictKeepLocal 保留本地
	ConflictKeepLocal ConflictResolution = "keep_local"
	// ConflictKeepRemote 保留远程
	ConflictKeepRemote ConflictResolution = "keep_remote"
	// ConflictKeepBoth 保留两者
	ConflictKeepBoth ConflictResolution = "keep_both"
	// ConflictManual 手动解决
	ConflictManual ConflictResolution = "manual"
	// ConflictLatest 保留最新
	ConflictLatest ConflictResolution = "latest"
)

// SyncConfig 同步配置
type SyncConfig struct {
	// ID 任务ID
	ID string `json:"id"`
	// Name 任务名称
	Name string `json:"name"`
	// SourcePath 源路径
	SourcePath string `json:"sourcePath"`
	// TargetPath 目标路径
	TargetPath string `json:"targetPath"`
	// Mode 同步模式
	Mode SyncMode `json:"mode"`
	// ConflictResolution 冲突解决策略
	ConflictResolution ConflictResolution `json:"conflictResolution"`
	// ExcludePatterns 排除模式
	ExcludePatterns []string `json:"excludePatterns"`
	// IncludePatterns 包含模式
	IncludePatterns []string `json:"includePatterns"`
	// MaxFileSize 最大文件大小 (bytes)
	MaxFileSize int64 `json:"maxFileSize"`
	// EnableVersioning 启用版本控制
	EnableVersioning bool `json:"enableVersioning"`
	// MaxVersions 最大版本数
	MaxVersions int `json:"maxVersions"`
	// Schedule 同步调度
	Schedule string `json:"schedule"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
}

// SyncTask 同步任务
type SyncTask struct {
	// Config 配置
	Config SyncConfig `json:"config"`
	// Status 状态
	Status SyncStatus `json:"status"`
	// LastSync 上次同步时间
	LastSync time.Time `json:"lastSync"`
	// NextSync 下次同步时间
	NextSync time.Time `json:"nextSync"`
	// TotalFiles 总文件数
	TotalFiles int `json:"totalFiles"`
	// SyncedFiles 已同步文件数
	SyncedFiles int `json:"syncedFiles"`
	// FailedFiles 失败文件数
	FailedFiles int `json:"failedFiles"`
	// ConflictFiles 冲突文件数
	ConflictFiles int `json:"conflictFiles"`
	// TotalSize 总大小
	TotalSize int64 `json:"totalSize"`
	// SyncedSize 已同步大小
	SyncedSize int64 `json:"syncedSize"`
	// Speed 同步速度 (bytes/sec)
	Speed int64 `json:"speed"`
	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// SyncConflict 同步冲突
type SyncConflict struct {
	// ID 冲突ID
	ID string `json:"id"`
	// TaskID 任务ID
	TaskID string `json:"taskId"`
	// FilePath 文件路径
	FilePath string `json:"filePath"`
	// LocalModTime 本地修改时间
	LocalModTime time.Time `json:"localModTime"`
	// RemoteModTime 远程修改时间
	RemoteModTime time.Time `json:"remoteModTime"`
	// LocalSize 本地大小
	LocalSize int64 `json:"localSize"`
	// RemoteSize 远程大小
	RemoteSize int64 `json:"remoteSize"`
	// Resolution 解决策略
	Resolution ConflictResolution `json:"resolution"`
	// Resolved 是否已解决
	Resolved bool `json:"resolved"`
	// ResolvedAt 解决时间
	ResolvedAt time.Time `json:"resolvedAt,omitempty"`
}

// SyncHistory 同步历史
type SyncHistory struct {
	// ID 历史ID
	ID string `json:"id"`
	// TaskID 任务ID
	TaskID string `json:"taskId"`
	// StartTime 开始时间
	StartTime time.Time `json:"startTime"`
	// EndTime 结束时间
	EndTime time.Time `json:"endTime"`
	// Duration 持续时间
	Duration time.Duration `json:"duration"`
	// FilesSynced 同步文件数
	FilesSynced int `json:"filesSynced"`
	// FilesFailed 失败文件数
	FilesFailed int `json:"filesFailed"`
	// BytesSynced 同步字节数
	BytesSynced int64 `json:"bytesSynced"`
	// Status 状态
	Status SyncStatus `json:"status"`
	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// SyncEngine 同步引擎
type SyncEngine struct {
	mu       sync.RWMutex
	tasks    map[string]*SyncTask
	conflicts map[string]*SyncConflict
	history  []SyncHistory
	running  bool
}

// NewSyncEngine 创建同步引擎
func NewSyncEngine() *SyncEngine {
	return &SyncEngine{
		tasks:    make(map[string]*SyncTask),
		conflicts: make(map[string]*SyncConflict),
	}
}

// AddTask 添加同步任务
func (e *SyncEngine) AddTask(config SyncConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task := &SyncTask{
		Config: config,
		Status: SyncStatusIdle,
	}
	e.tasks[config.ID] = task
	return nil
}

// RemoveTask 移除同步任务
func (e *SyncEngine) RemoveTask(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.tasks, taskID)
	return nil
}

// StartTask 启动同步任务
func (e *SyncEngine) StartTask(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = SyncStatusSyncing
	return nil
}

// StopTask 停止同步任务
func (e *SyncEngine) StopTask(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = SyncStatusPaused
	return nil
}

// GetTask 获取同步任务
func (e *SyncEngine) GetTask(taskID string) (*SyncTask, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// ListTasks 列出同步任务
func (e *SyncEngine) ListTasks() []*SyncTask {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tasks := make([]*SyncTask, 0, len(e.tasks))
	for _, task := range e.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetConflicts 获取冲突列表
func (e *SyncEngine) GetConflicts(taskID string) []*SyncConflict {
	e.mu.RLock()
	defer e.mu.RUnlock()

	conflicts := make([]*SyncConflict, 0)
	for _, conflict := range e.conflicts {
		if taskID == "" || conflict.TaskID == taskID {
			conflicts = append(conflicts, conflict)
		}
	}
	return conflicts
}

// ResolveConflict 解决冲突
func (e *SyncEngine) ResolveConflict(conflictID string, resolution ConflictResolution) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	conflict, ok := e.conflicts[conflictID]
	if !ok {
		return fmt.Errorf("conflict not found: %s", conflictID)
	}

	conflict.Resolution = resolution
	conflict.Resolved = true
	conflict.ResolvedAt = time.Now()
	return nil
}

// GetHistory 获取同步历史
func (e *SyncEngine) GetHistory(taskID string, limit int) []SyncHistory {
	e.mu.RLock()
	defer e.mu.RUnlock()

	history := make([]SyncHistory, 0)
	for _, h := range e.history {
		if taskID == "" || h.TaskID == taskID {
			history = append(history, h)
		}
	}

	if limit > 0 && len(history) > limit {
		history = history[len(history)-limit:]
	}
	return history
}

// Start 启动引擎
func (e *SyncEngine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("engine already running")
	}

	e.running = true
	return nil
}

// Stop 停止引擎
func (e *SyncEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.running = false
	return nil
}

// IsRunning 是否运行中
func (e *SyncEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.running
}
