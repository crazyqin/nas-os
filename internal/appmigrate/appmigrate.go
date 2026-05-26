// Package appmigrate 应用池迁移管理
// 灵感来源: TrueNAS 25.10 应用池迁移
package appmigrate

import (
	"fmt"
	"sync"
	"time"
)

// MigrationStatus 迁移状态
type MigrationStatus string

const (
	StatusPending    MigrationStatus = "pending"
	StatusRunning    MigrationStatus = "running"
	StatusCompleted  MigrationStatus = "completed"
	StatusFailed     MigrationStatus = "failed"
	StatusRolledBack MigrationStatus = "rolled_back"
)

// App 应用信息
type App struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PoolID     string `json:"pool_id"`
	SizeBytes  int64  `json:"size_bytes"`
	Containers int    `json:"containers"`
	Volumes    int    `json:"volumes"`
}

// MigrationTask 迁移任务
type MigrationTask struct {
	ID           string          `json:"id"`
	AppID        string          `json:"app_id"`
	AppName      string          `json:"app_name"`
	SourcePool   string          `json:"source_pool"`
	TargetPool   string          `json:"target_pool"`
	Status       MigrationStatus `json:"status"`
	Progress     float64         `json:"progress"` // 0-100
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
	BytesTotal   int64           `json:"bytes_total"`
	BytesCopied  int64           `json:"bytes_copied"`
}

// MigrationManager 迁移管理器
type MigrationManager struct {
	mu         sync.RWMutex
	apps       map[string]*App
	tasks      map[string]*MigrationTask
	taskOrder  []string
}

// NewMigrationManager 创建迁移管理器
func NewMigrationManager() *MigrationManager {
	return &MigrationManager{
		apps:  make(map[string]*App),
		tasks: make(map[string]*MigrationTask),
	}
}

// RegisterApp 注册应用
func (mm *MigrationManager) RegisterApp(app *App) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.apps[app.ID] = app
}

// GetApp 获取应用
func (mm *MigrationManager) GetApp(id string) (*App, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	app, ok := mm.apps[id]
	return app, ok
}

// ListApps 列出指定池的应用
func (mm *MigrationManager) ListApps(poolID string) []*App {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	apps := make([]*App, 0)
	for _, app := range mm.apps {
		if poolID == "" || app.PoolID == poolID {
			apps = append(apps, app)
		}
	}
	return apps
}

// StartMigration 开始迁移
func (mm *MigrationManager) StartMigration(appID, targetPool string) (*MigrationTask, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	app, exists := mm.apps[appID]
	if !exists {
		return nil, fmt.Errorf("app %s not found", appID)
	}

	if app.PoolID == targetPool {
		return nil, fmt.Errorf("app %s is already on pool %s", appID, targetPool)
	}

	// 检查是否有进行中的迁移
	for _, task := range mm.tasks {
		if task.AppID == appID && task.Status == StatusRunning {
			return nil, fmt.Errorf("app %s has an active migration", appID)
		}
	}

	now := time.Now()
	task := &MigrationTask{
		ID:         fmt.Sprintf("mig-%s-%d", appID, now.Unix()),
		AppID:      appID,
		AppName:    app.Name,
		SourcePool: app.PoolID,
		TargetPool: targetPool,
		Status:     StatusRunning,
		StartedAt:  &now,
		BytesTotal: app.SizeBytes,
	}

	mm.tasks[task.ID] = task
	mm.taskOrder = append(mm.taskOrder, task.ID)
	return task, nil
}

// UpdateProgress 更新迁移进度
func (mm *MigrationManager) UpdateProgress(taskID string, progress float64, bytesCopied int64) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	task, exists := mm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != StatusRunning {
		return fmt.Errorf("task %s is not running", taskID)
	}

	task.Progress = progress
	task.BytesCopied = bytesCopied

	if progress >= 100 {
		task.Status = StatusCompleted
		task.Progress = 100
		now := time.Now()
		task.CompletedAt = &now

		// 更新应用池归属
		if app, ok := mm.apps[task.AppID]; ok {
			app.PoolID = task.TargetPool
		}
	}
	return nil
}

// FailTask 标记任务失败
func (mm *MigrationManager) FailTask(taskID, reason string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	task, exists := mm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Status = StatusFailed
	task.ErrorMessage = reason
	now := time.Now()
	task.CompletedAt = &now
	return nil
}

// Rollback 回滚迁移
func (mm *MigrationManager) Rollback(taskID string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	task, exists := mm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status == StatusCompleted {
		// 恢复应用到原始池
		if app, ok := mm.apps[task.AppID]; ok {
			app.PoolID = task.SourcePool
		}
	}

	task.Status = StatusRolledBack
	now := time.Now()
	task.CompletedAt = &now
	return nil
}

// GetTask 获取任务
func (mm *MigrationManager) GetTask(taskID string) (*MigrationTask, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	task, ok := mm.tasks[taskID]
	return task, ok
}

// ListTasks 列出任务
func (mm *MigrationManager) ListTasks(status MigrationStatus) []*MigrationTask {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	tasks := make([]*MigrationTask, 0)
	for _, id := range mm.taskOrder {
		task := mm.tasks[id]
		if status == "" || task.Status == status {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// EstimateMigrationTime 估算迁移时间
func (mm *MigrationManager) EstimateMigrationTime(appID string, bandwidthMBps int) (time.Duration, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	app, exists := mm.apps[appID]
	if !exists {
		return 0, fmt.Errorf("app %s not found", appID)
	}

	if bandwidthMBps <= 0 {
		bandwidthMBps = 100 // 默认 100 MB/s
	}

	bytesPerSec := int64(bandwidthMBps) * 1024 * 1024
	seconds := app.SizeBytes / bytesPerSec
	return time.Duration(seconds) * time.Second, nil
}
