// Package quickrestore 快速恢复
// 对标群晖 Snapshot Replication，支持从快照/回收站/备份快速恢复文件
package quickrestore

import (
	"fmt"
	"sync"
	"time"
)

// RestoreSource 恢复来源.
type RestoreSource string

const (
	SourceSnapshot RestoreSource = "snapshot" // 从快照恢复
	SourceRecycle  RestoreSource = "recycle"  // 从回收站恢复
	SourceBackup   RestoreSource = "backup"   // 从备份恢复
)

// RestoreStatus 恢复状态.
type RestoreStatus string

const (
	StatusPending   RestoreStatus = "pending"
	StatusRunning   RestoreStatus = "running"
	StatusCompleted RestoreStatus = "completed"
	StatusFailed    RestoreStatus = "failed"
	StatusCancelled RestoreStatus = "cancelled"
)

// RestorePoint 恢复点.
type RestorePoint struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Source      RestoreSource `json:"source"`
	Path        string        `json:"path"`           // 快照/备份路径
	Timestamp   time.Time     `json:"timestamp"`      // 恢复点时间
	FileCount   int           `json:"file_count"`     // 包含的文件数
	TotalSize   int64         `json:"total_size"`     // 总大小（字节）
	IsAutomatic bool          `json:"is_automatic"`   // 是否自动创建
	Tags        []string      `json:"tags,omitempty"` // 标签
	CreatedAt   time.Time     `json:"created_at"`
	ExpiresAt   *time.Time    `json:"expires_at,omitempty"` // 过期时间
}

// FileDiff 文件差异.
type FileDiff struct {
	Path        string    `json:"path"`
	CurrentSize int64     `json:"current_size"`
	RestoreSize int64     `json:"restore_size"`
	CurrentMod  time.Time `json:"current_modified"`
	RestoreMod  time.Time `json:"restore_modified"`
	Status      string    `json:"status"` // added/modified/deleted
	HasConflict bool      `json:"has_conflict"`
}

// RestorePreview 恢复预览.
type RestorePreview struct {
	PointID       string     `json:"point_id"`
	TargetPath    string     `json:"target_path"`
	TotalFiles    int        `json:"total_files"`
	AddedFiles    int        `json:"added_files"`
	ModifiedFiles int        `json:"modified_files"`
	DeletedFiles  int        `json:"deleted_files"`
	Conflicts     int        `json:"conflicts"`
	Diffs         []FileDiff `json:"diffs"`
	EstimatedSize int64      `json:"estimated_size"`
	EstimatedTime int        `json:"estimated_time_seconds"`
}

// RestoreRequest 恢复请求.
type RestoreRequest struct {
	PointID    string   `json:"point_id" binding:"required"`
	TargetPath string   `json:"target_path" binding:"required"`
	Files      []string `json:"files,omitempty"` // 指定恢复的文件，为空则全部恢复
	Overwrite  bool     `json:"overwrite"`       // 是否覆盖已有文件
	DryRun     bool     `json:"dry_run"`         // 试运行
}

// BatchRestoreRequest 批量恢复请求.
type BatchRestoreRequest struct {
	Requests []RestoreRequest `json:"requests" binding:"required,min=1"`
}

// RestoreTask 恢复任务.
type RestoreTask struct {
	ID            string        `json:"id"`
	PointID       string        `json:"point_id"`
	TargetPath    string        `json:"target_path"`
	Status        RestoreStatus `json:"status"`
	TotalFiles    int           `json:"total_files"`
	RestoredFiles int           `json:"restored_files"`
	FailedFiles   int           `json:"failed_files"`
	TotalSize     int64         `json:"total_size"`
	RestoredSize  int64         `json:"restored_size"`
	Progress      float64       `json:"progress"` // 0-100
	Error         string        `json:"error,omitempty"`
	StartedAt     *time.Time    `json:"started_at,omitempty"`
	CompletedAt   *time.Time    `json:"completed_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
}

// RestoreHistory 恢复历史记录.
type RestoreHistory struct {
	ID         string        `json:"id"`
	TaskID     string        `json:"task_id"`
	PointID    string        `json:"point_id"`
	PointName  string        `json:"point_name"`
	Source     RestoreSource `json:"source"`
	TargetPath string        `json:"target_path"`
	FileCount  int           `json:"file_count"`
	TotalSize  int64         `json:"total_size"`
	Status     RestoreStatus `json:"status"`
	Duration   int           `json:"duration_seconds"`
	Operator   string        `json:"operator"`
	CreatedAt  time.Time     `json:"created_at"`
}

// CreatePointRequest 创建恢复点请求.
type CreatePointRequest struct {
	Name        string        `json:"name" binding:"required"`
	Description string        `json:"description"`
	Source      RestoreSource `json:"source" binding:"required"`
	Path        string        `json:"path" binding:"required"`
	Tags        []string      `json:"tags,omitempty"`
	ExpiresIn   int           `json:"expires_in_days,omitempty"` // 过期天数
}

// Manager 快速恢复管理器.
type Manager struct {
	mu      sync.RWMutex
	points  map[string]*RestorePoint
	tasks   map[string]*RestoreTask
	history []*RestoreHistory
	stopCh  chan struct{}
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		points:  make(map[string]*RestorePoint),
		tasks:   make(map[string]*RestoreTask),
		history: make([]*RestoreHistory, 0),
		stopCh:  make(chan struct{}),
	}
}

// CreatePoint 创建恢复点.
func (m *Manager) CreatePoint(req *CreatePointRequest) (*RestorePoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("恢复点名称不能为空")
	}
	if req.Source == "" {
		return nil, fmt.Errorf("恢复来源不能为空")
	}
	if req.Path == "" {
		return nil, fmt.Errorf("恢复路径不能为空")
	}

	point := &RestorePoint{
		ID:          fmt.Sprintf("rp_%d", time.Now().UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Source:      req.Source,
		Path:        req.Path,
		Timestamp:   time.Now(),
		Tags:        req.Tags,
		CreatedAt:   time.Now(),
	}

	if req.ExpiresIn > 0 {
		expires := time.Now().AddDate(0, 0, req.ExpiresIn)
		point.ExpiresAt = &expires
	}

	m.points[point.ID] = point
	return point, nil
}

// ListPoints 列出所有恢复点.
func (m *Manager) ListPoints() []RestorePoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RestorePoint, 0, len(m.points))
	for _, p := range m.points {
		result = append(result, *p)
	}
	return result
}

// GetPoint 获取恢复点.
func (m *Manager) GetPoint(id string) (*RestorePoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	point, exists := m.points[id]
	if !exists {
		return nil, fmt.Errorf("恢复点不存在: %s", id)
	}
	return point, nil
}

// DeletePoint 删除恢复点.
func (m *Manager) DeletePoint(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.points[id]; !exists {
		return false
	}
	delete(m.points, id)
	return true
}

// PreviewRestore 恢复预览.
func (m *Manager) PreviewRestore(pointID, targetPath string, files []string) (*RestorePreview, error) {
	m.mu.RLock()
	point, exists := m.points[pointID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("恢复点不存在: %s", pointID)
	}

	// 模拟预览
	preview := &RestorePreview{
		PointID:    pointID,
		TargetPath: targetPath,
		TotalFiles: point.FileCount,
		Diffs:      make([]FileDiff, 0),
	}

	// 模拟部分文件差异
	if point.FileCount > 0 {
		preview.AddedFiles = point.FileCount / 3
		preview.ModifiedFiles = point.FileCount / 3
		preview.DeletedFiles = point.FileCount - preview.AddedFiles - preview.ModifiedFiles
		preview.Conflicts = point.FileCount / 10
		preview.EstimatedSize = point.TotalSize
		preview.EstimatedTime = point.FileCount / 100 // 简单估算
	}

	return preview, nil
}

// ExecuteRestore 执行恢复.
func (m *Manager) ExecuteRestore(req *RestoreRequest) (*RestoreTask, error) {
	m.mu.RLock()
	point, exists := m.points[req.PointID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("恢复点不存在: %s", req.PointID)
	}

	if req.TargetPath == "" {
		return nil, fmt.Errorf("目标路径不能为空")
	}

	task := &RestoreTask{
		ID:         fmt.Sprintf("task_%d", time.Now().UnixNano()),
		PointID:    req.PointID,
		TargetPath: req.TargetPath,
		Status:     StatusPending,
		TotalFiles: point.FileCount,
		TotalSize:  point.TotalSize,
		CreatedAt:  time.Now(),
	}

	if len(req.Files) > 0 {
		task.TotalFiles = len(req.Files)
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 异步执行恢复
	go m.executeRestore(task, point, req)

	return task, nil
}

func (m *Manager) executeRestore(task *RestoreTask, point *RestorePoint, req *RestoreRequest) {
	m.mu.Lock()
	now := time.Now()
	task.Status = StatusRunning
	task.StartedAt = &now
	m.mu.Unlock()

	// 模拟恢复过程
	totalFiles := task.TotalFiles
	for i := 0; i < totalFiles; i++ {
		m.mu.Lock()
		task.RestoredFiles++
		task.RestoredSize += point.TotalSize / int64(totalFiles)
		task.Progress = float64(task.RestoredFiles) / float64(totalFiles) * 100
		m.mu.Unlock()
	}

	// 完成
	m.mu.Lock()
	completed := time.Now()
	task.Status = StatusCompleted
	task.CompletedAt = &completed
	task.Progress = 100

	// 添加到历史记录
	history := &RestoreHistory{
		ID:         fmt.Sprintf("hist_%d", time.Now().UnixNano()),
		TaskID:     task.ID,
		PointID:    point.ID,
		PointName:  point.Name,
		Source:     point.Source,
		TargetPath: task.TargetPath,
		FileCount:  task.RestoredFiles,
		TotalSize:  task.RestoredSize,
		Status:     StatusCompleted,
		Duration:   int(completed.Sub(*task.StartedAt).Seconds()),
		CreatedAt:  time.Now(),
	}
	m.history = append(m.history, history)
	m.mu.Unlock()
}

// BatchRestore 批量恢复.
func (m *Manager) BatchRestore(requests []RestoreRequest) ([]*RestoreTask, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("恢复请求列表不能为空")
	}

	tasks := make([]*RestoreTask, 0, len(requests))
	for _, req := range requests {
		task, err := m.ExecuteRestore(&req)
		if err != nil {
			return nil, fmt.Errorf("批量恢复失败: %v", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetTask 获取恢复任务.
func (m *Manager) GetTask(id string) (*RestoreTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", id)
	}
	return task, nil
}

// ListTasks 列出所有任务.
func (m *Manager) ListTasks() []RestoreTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RestoreTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, *t)
	}
	return result
}

// GetHistory 获取恢复历史.
func (m *Manager) GetHistory() []RestoreHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RestoreHistory, len(m.history))
	for i, h := range m.history {
		if h != nil {
			result[i] = *h
		}
	}
	return result
}
