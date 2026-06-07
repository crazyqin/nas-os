package iscsiblockclone

import (
	"fmt"
	"time"
)

// NewBlockCloneManager 创建块克隆管理器
func NewBlockCloneManager(cfg ManagerConfig) *BlockCloneManager {
	return &BlockCloneManager{
		config: cfg,
		luns:   make(map[string]*LUNInfo),
		tasks:  make(map[string]*BlockCloneTask),
	}
}

// RegisterLUN 注册LUN
func (m *BlockCloneManager) RegisterLUN(lun *LUNInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.luns[lun.ID]; exists {
		return ErrLUNExists
	}
	m.luns[lun.ID] = lun
	return nil
}

// UnregisterLUN 注销LUN
func (m *BlockCloneManager) UnregisterLUN(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.luns[id]; !exists {
		return ErrLUNNotFound
	}
	delete(m.luns, id)
	return nil
}

// GetLUN 获取LUN信息
func (m *BlockCloneManager) GetLUN(id string) (*LUNInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	lun, exists := m.luns[id]
	if !exists {
		return nil, ErrLUNNotFound
	}
	return lun, nil
}

// ListLUNs 列出所有LUN
func (m *BlockCloneManager) ListLUNs() []*LUNInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*LUNInfo, 0, len(m.luns))
	for _, lun := range m.luns {
		result = append(result, lun)
	}
	return result
}

// CloneLUN 克隆LUN
func (m *BlockCloneManager) CloneLUN(sourceID, targetName string, cloneType CloneType) (*BlockCloneTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	source, exists := m.luns[sourceID]
	if !exists {
		return nil, ErrLUNNotFound
	}

	// 检查并发限制
	activeCount := 0
	for _, t := range m.tasks {
		if t.Status == StatusInProgress {
			activeCount++
		}
	}
	if activeCount >= m.config.MaxConcurrentClones {
		return nil, ErrMaxConcurrent
	}

	taskID := fmt.Sprintf("clone-%d", time.Now().UnixNano())
	task := &BlockCloneTask{
		ID:        taskID,
		SourceLUN: sourceID,
		TargetLUN: targetName,
		Type:      cloneType,
		Status:    StatusPending,
		SizeBytes: source.SizeBytes,
		StartedAt: time.Now(),
	}
	m.tasks[taskID] = task

	// 异步执行克隆
	go m.executeClone(task)

	return task, nil
}

func (m *BlockCloneManager) executeClone(task *BlockCloneTask) {
	m.mu.Lock()
	task.Status = StatusInProgress
	m.mu.Unlock()

	// 模拟块克隆过程
	steps := 10
	for i := 0; i < steps; i++ {
		time.Sleep(10 * time.Millisecond)
		m.mu.Lock()
		task.Progress = float64(i+1) / float64(steps) * 100
		task.ClonedBytes = int64(float64(task.SizeBytes) * task.Progress / 100)
		task.SpeedMBps = float64(task.ClonedBytes) / (1024 * 1024) / (time.Since(task.StartedAt).Seconds() + 0.001)
		m.mu.Unlock()
	}

	now := time.Now()
	m.mu.Lock()
	task.Status = StatusCompleted
	task.Progress = 100
	task.ClonedBytes = task.SizeBytes
	task.CompletedAt = &now
	m.stats.TotalClones++
	m.stats.SuccessfulClones++
	m.stats.TotalBytesCloned += task.SizeBytes
	m.stats.AverageSpeedMBps = task.SpeedMBps
	m.mu.Unlock()
}

// GetTask 获取克隆任务
func (m *BlockCloneManager) GetTask(taskID string) (*BlockCloneTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, exists := m.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有任务
func (m *BlockCloneManager) ListTasks() []*BlockCloneTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*BlockCloneTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result
}

// GetStats 获取克隆统计
func (m *BlockCloneManager) GetStats() CloneStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}
