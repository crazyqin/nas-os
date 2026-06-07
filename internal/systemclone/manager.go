// Package systemclone - 系统克隆管理器
package systemclone

import (
	"fmt"
	"sync"
	"time"
)

// Manager 系统克隆管理器
type Manager struct {
	mu       sync.RWMutex
	tasks    map[string]*DiskCloneTask
	images   map[string]*BackupImage
	restores map[string]*RestoreTask
	pxe      map[string]*PXEDeployConfig
	stats    *CloneStats
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		tasks:    make(map[string]*DiskCloneTask),
		images:   make(map[string]*BackupImage),
		restores: make(map[string]*RestoreTask),
		pxe:      make(map[string]*PXEDeployConfig),
		stats:    &CloneStats{},
	}
}

// CreateCloneTask 创建克隆任务
func (m *Manager) CreateCloneTask(task *DiskCloneTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.ID == "" {
		task.ID = fmt.Sprintf("clone-%d", time.Now().UnixNano())
	}
	task.Status = CloneStatusPending
	task.CreatedAt = time.Now()
	m.tasks[task.ID] = task
	m.stats.TotalClones++
	return nil
}

// StartClone 启动克隆任务
func (m *Manager) StartClone(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Status = CloneStatusRunning
	now := time.Now()
	task.StartedAt = &now

	// 模拟克隆完成
	go func() {
		time.Sleep(3 * time.Second)
		m.mu.Lock()
		defer m.mu.Unlock()
		task.Status = CloneStatusCompleted
		task.Progress = 100
		task.BytesCopied = task.BytesTotal
		completed := time.Now()
		task.CompletedAt = &completed
		m.stats.SuccessfulClones++
	}()

	return nil
}

// GetCloneTask 获取克隆任务
func (m *Manager) GetCloneTask(id string) (*DiskCloneTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return task, nil
}

// ListCloneTasks 列出克隆任务
func (m *Manager) ListCloneTasks() []*DiskCloneTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*DiskCloneTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// CreateImage 创建备份镜像
func (m *Manager) CreateImage(image *BackupImage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if image.ID == "" {
		image.ID = fmt.Sprintf("img-%d", time.Now().UnixNano())
	}
	image.CreatedAt = time.Now()
	m.images[image.ID] = image
	m.stats.TotalImages++
	m.stats.TotalImageSize += image.SizeBytes
	return nil
}

// ListImages 列出镜像
func (m *Manager) ListImages() []*BackupImage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	images := make([]*BackupImage, 0, len(m.images))
	for _, img := range m.images {
		images = append(images, img)
	}
	return images
}

// RestoreFromImage 从镜像恢复
func (m *Manager) RestoreFromImage(imageID, targetDisk string) (*RestoreTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.images[imageID]; !ok {
		return nil, fmt.Errorf("image %s not found", imageID)
	}

	task := &RestoreTask{
		ID:         fmt.Sprintf("restore-%d", time.Now().UnixNano()),
		ImageID:    imageID,
		TargetDisk: targetDisk,
		Status:     CloneStatusRunning,
		Progress:   0,
		CreatedAt:  time.Now(),
	}
	m.restores[task.ID] = task
	m.stats.TotalRestores++

	// 模拟恢复完成
	go func() {
		time.Sleep(5 * time.Second)
		m.mu.Lock()
		defer m.mu.Unlock()
		task.Status = CloneStatusCompleted
		task.Progress = 100
		completed := time.Now()
		task.CompletedAt = &completed
	}()

	return task, nil
}

// ConfigurePXE 配置 PXE 部署
func (m *Manager) ConfigurePXE(config *PXEDeployConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == "" {
		config.ID = fmt.Sprintf("pxe-%d", time.Now().UnixNano())
	}
	m.pxe[config.ID] = config
	return nil
}

// GetStats 获取统计
func (m *Manager) GetStats() *CloneStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}
