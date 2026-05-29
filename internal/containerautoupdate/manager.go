// Package containerautoupdate 提供容器自动更新管理核心业务逻辑
package containerautoupdate

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 容器自动更新管理器.
type Manager struct {
	containers map[string]*ContainerConfig
	updates    map[string]*ContainerUpdate
	history    []*ContainerUpdate
	mu         sync.RWMutex
}

// NewManager 创建容器自动更新管理器.
func NewManager() *Manager {
	return &Manager{
		containers: make(map[string]*ContainerConfig),
		updates:    make(map[string]*ContainerUpdate),
		history:    make([]*ContainerUpdate, 0),
	}
}

// ========== 容器管理 ==========

// AddContainer 添加容器配置.
func (m *Manager) AddContainer(req AddContainerRequest) *ContainerConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	tag := req.Tag
	if tag == "" {
		tag = "latest"
	}

	// 设置默认值
	policy := req.Policy
	if policy.MaxRetries == 0 {
		policy.MaxRetries = 3
	}

	rollback := req.Rollback
	if rollback.MaxHistory == 0 {
		rollback.MaxHistory = 5
	}
	if rollback.RollbackTimeout == 0 {
		rollback.RollbackTimeout = 5 * time.Minute
	}

	healthCheck := req.HealthCheck
	if healthCheck.Interval == 0 {
		healthCheck.Interval = 10 * time.Second
	}
	if healthCheck.Timeout == 0 {
		healthCheck.Timeout = 5 * time.Second
	}
	if healthCheck.Retries == 0 {
		healthCheck.Retries = 3
	}
	if healthCheck.ExpectedStatus == 0 {
		healthCheck.ExpectedStatus = 200
	}

	container := &ContainerConfig{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Image:       req.Image,
		Tag:         tag,
		Enabled:     true,
		Policy:      policy,
		Rollback:    rollback,
		HealthCheck: healthCheck,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.containers[container.ID] = container
	return container
}

// RemoveContainer 移除容器配置.
func (m *Manager) RemoveContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.containers[id]; !ok {
		return fmt.Errorf("container %q not found", id)
	}
	delete(m.containers, id)
	return nil
}

// GetContainer 获取容器配置.
func (m *Manager) GetContainer(id string) (*ContainerConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, ok := m.containers[id]
	if !ok {
		return nil, fmt.Errorf("container %q not found", id)
	}

	cp := *container
	return &cp, nil
}

// ListContainers 列出所有容器配置.
func (m *Manager) ListContainers() []*ContainerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	containers := make([]*ContainerConfig, 0, len(m.containers))
	for _, c := range m.containers {
		cp := *c
		containers = append(containers, &cp)
	}
	return containers
}

// UpdateContainer 更新容器配置.
func (m *Manager) UpdateContainer(id string, req AddContainerRequest) (*ContainerConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, ok := m.containers[id]
	if !ok {
		return nil, fmt.Errorf("container %q not found", id)
	}

	if req.Name != "" {
		container.Name = req.Name
	}
	if req.Image != "" {
		container.Image = req.Image
	}
	if req.Tag != "" {
		container.Tag = req.Tag
	}
	container.Policy = req.Policy
	container.Rollback = req.Rollback
	container.HealthCheck = req.HealthCheck
	container.UpdatedAt = time.Now()

	cp := *container
	return &cp, nil
}

// ========== 更新操作 ==========

// CheckUpdates 检查所有启用容器的更新.
func (m *Manager) CheckUpdates() ([]*ContainerUpdate, error) {
	m.mu.RLock()
	containers := make([]*ContainerConfig, 0)
	for _, c := range m.containers {
		if c.Enabled {
			cp := *c
			containers = append(containers, &cp)
		}
	}
	m.mu.RUnlock()

	updates := make([]*ContainerUpdate, 0)
	for _, container := range containers {
		hasUpdate, newTag := m.checkImageUpdate(container.Image, container.Tag)
		if hasUpdate {
			update := &ContainerUpdate{
				ID:            uuid.New().String(),
				ContainerID:   container.ID,
				ContainerName: container.Name,
				OldImage:      container.Image,
				OldTag:        container.Tag,
				NewImage:      container.Image,
				NewTag:        newTag,
				Status:        StatusPending,
				StartedAt:     time.Now(),
			}

			m.mu.Lock()
			m.updates[update.ID] = update
			m.mu.Unlock()

			updates = append(updates, update)
		}
	}

	return updates, nil
}

// CheckContainerUpdate 检查单个容器的更新.
func (m *Manager) CheckContainerUpdate(containerID string) (*ContainerUpdate, error) {
	m.mu.RLock()
	container, ok := m.containers[containerID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("container %q not found", containerID)
	}
	containerCopy := *container
	m.mu.RUnlock()

	hasUpdate, newTag := m.checkImageUpdate(containerCopy.Image, containerCopy.Tag)
	if !hasUpdate {
		return nil, nil
	}

	update := &ContainerUpdate{
		ID:            uuid.New().String(),
		ContainerID:   containerCopy.ID,
		ContainerName: containerCopy.Name,
		OldImage:      containerCopy.Image,
		OldTag:        containerCopy.Tag,
		NewImage:      containerCopy.Image,
		NewTag:        newTag,
		Status:        StatusPending,
		StartedAt:     time.Now(),
	}

	m.mu.Lock()
	m.updates[update.ID] = update
	m.mu.Unlock()

	return update, nil
}

// ApplyUpdate 应用更新.
func (m *Manager) ApplyUpdate(containerID string, newTag string) (*ContainerUpdate, error) {
	m.mu.RLock()
	container, ok := m.containers[containerID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("container %q not found", containerID)
	}
	containerCopy := *container
	m.mu.RUnlock()

	// 确定新标签
	tag := newTag
	if tag == "" {
		tag = "latest"
	}

	// 创建更新记录
	update := &ContainerUpdate{
		ID:            uuid.New().String(),
		ContainerID:   containerCopy.ID,
		ContainerName: containerCopy.Name,
		OldImage:      containerCopy.Image,
		OldTag:        containerCopy.Tag,
		NewImage:      containerCopy.Image,
		NewTag:        tag,
		Status:        StatusPulling,
		StartedAt:     time.Now(),
	}

	m.mu.Lock()
	m.updates[update.ID] = update
	m.mu.Unlock()

	// 模拟拉取镜像
	if err := m.pullImage(containerCopy.Image, tag); err != nil {
		update.Status = StatusFailed
		update.Error = err.Error()
		completedAt := time.Now()
		update.CompletedAt = &completedAt
		update.Duration = completedAt.Sub(update.StartedAt).Milliseconds()

		m.mu.Lock()
		m.history = append(m.history, update)
		m.mu.Unlock()

		return update, err
	}

	// 模拟停止容器
	update.Status = StatusStopping
	m.stopContainer(containerCopy.Name)

	// 模拟启动新容器
	update.Status = StatusStarting
	if err := m.startContainer(containerCopy.Name, containerCopy.Image, tag); err != nil {
		update.Status = StatusFailed
		update.Error = err.Error()
		completedAt := time.Now()
		update.CompletedAt = &completedAt
		update.Duration = completedAt.Sub(update.StartedAt).Milliseconds()

		// 尝试回滚
		if containerCopy.Rollback.AutoRollback {
			m.performRollback(containerCopy, update)
		}

		m.mu.Lock()
		m.history = append(m.history, update)
		m.mu.Unlock()

		return update, err
	}

	// 健康检查
	if containerCopy.HealthCheck.Enabled {
		update.Status = StatusHealthCheck
		if err := m.performHealthCheck(containerCopy); err != nil {
			update.Status = StatusFailed
			update.Error = fmt.Sprintf("health check failed: %v", err)
			completedAt := time.Now()
			update.CompletedAt = &completedAt
			update.Duration = completedAt.Sub(update.StartedAt).Milliseconds()

			// 自动回滚
			if containerCopy.Rollback.AutoRollback {
				m.performRollback(containerCopy, update)
			}

			m.mu.Lock()
			m.history = append(m.history, update)
			m.mu.Unlock()

			return update, err
		}
	}

	// 更新成功
	update.Status = StatusCompleted
	completedAt := time.Now()
	update.CompletedAt = &completedAt
	update.Duration = completedAt.Sub(update.StartedAt).Milliseconds()

	// 更新容器配置
	m.mu.Lock()
	m.containers[containerCopy.ID].Tag = tag
	m.containers[containerCopy.ID].UpdatedAt = time.Now()
	m.history = append(m.history, update)
	m.mu.Unlock()

	return update, nil
}

// Rollback 回滚到上一个版本.
func (m *Manager) Rollback(containerID string, updateID string) (*ContainerUpdate, error) {
	m.mu.RLock()
	container, ok := m.containers[containerID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("container %q not found", containerID)
	}
	containerCopy := *container
	m.mu.RUnlock()

	// 查找要回滚的更新
	var targetUpdate *ContainerUpdate
	if updateID != "" {
		m.mu.RLock()
		update, exists := m.updates[updateID]
		m.mu.RUnlock()
		if !exists {
			return nil, fmt.Errorf("update %q not found", updateID)
		}
		targetUpdate = update
	} else {
		// 查找最近一次成功的更新
		m.mu.RLock()
		for i := len(m.history) - 1; i >= 0; i-- {
			if m.history[i].ContainerID == containerID && m.history[i].Status == StatusCompleted {
				targetUpdate = m.history[i]
				break
			}
		}
		m.mu.RUnlock()

		if targetUpdate == nil {
			return nil, fmt.Errorf("no completed update found for container %q", containerID)
		}
	}

	// 创建回滚记录
	rollbackUpdate := &ContainerUpdate{
		ID:            uuid.New().String(),
		ContainerID:   containerCopy.ID,
		ContainerName: containerCopy.Name,
		OldImage:      containerCopy.Image,
		OldTag:        containerCopy.Tag,
		NewImage:      targetUpdate.OldImage,
		NewTag:        targetUpdate.OldTag,
		Status:        StatusStopping,
		StartedAt:     time.Now(),
	}

	m.mu.Lock()
	m.updates[rollbackUpdate.ID] = rollbackUpdate
	m.mu.Unlock()

	// 停止当前容器
	m.stopContainer(containerCopy.Name)

	// 启动旧版本
	rollbackUpdate.Status = StatusStarting
	if err := m.startContainer(containerCopy.Name, targetUpdate.OldImage, targetUpdate.OldTag); err != nil {
		rollbackUpdate.Status = StatusFailed
		rollbackUpdate.Error = err.Error()
		completedAt := time.Now()
		rollbackUpdate.CompletedAt = &completedAt
		rollbackUpdate.Duration = completedAt.Sub(rollbackUpdate.StartedAt).Milliseconds()

		m.mu.Lock()
		m.history = append(m.history, rollbackUpdate)
		m.mu.Unlock()

		return rollbackUpdate, err
	}

	// 回滚成功
	rollbackUpdate.Status = StatusRolledBack
	completedAt := time.Now()
	rollbackUpdate.CompletedAt = &completedAt
	rollbackUpdate.Duration = completedAt.Sub(rollbackUpdate.StartedAt).Milliseconds()

	// 更新容器配置
	m.mu.Lock()
	m.containers[containerCopy.ID].Tag = targetUpdate.OldTag
	m.containers[containerCopy.ID].UpdatedAt = time.Now()
	m.history = append(m.history, rollbackUpdate)
	m.mu.Unlock()

	return rollbackUpdate, nil
}

// HealthCheck 执行健康检查.
func (m *Manager) HealthCheck(containerID string) (bool, error) {
	m.mu.RLock()
	container, ok := m.containers[containerID]
	if !ok {
		m.mu.RUnlock()
		return false, fmt.Errorf("container %q not found", containerID)
	}
	containerCopy := *container
	m.mu.RUnlock()

	if !containerCopy.HealthCheck.Enabled {
		return true, nil
	}

	err := m.performHealthCheck(containerCopy)
	if err != nil {
		return false, err
	}
	return true, nil
}

// ========== 历史管理 ==========

// GetUpdateHistory 获取更新历史.
func (m *Manager) GetUpdateHistory(limit int) []*ContainerUpdate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*ContainerUpdate, limit)
	for i, u := range m.history[start:] {
		cp := *u
		result[i] = &cp
	}
	return result
}

// GetContainerHistory 获取特定容器的更新历史.
func (m *Manager) GetContainerHistory(containerID string, limit int) []*ContainerUpdate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	containerHistory := make([]*ContainerUpdate, 0)
	for _, u := range m.history {
		if u.ContainerID == containerID {
			containerHistory = append(containerHistory, u)
		}
	}

	if limit <= 0 || limit > len(containerHistory) {
		limit = len(containerHistory)
	}

	start := len(containerHistory) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*ContainerUpdate, limit)
	for i, u := range containerHistory[start:] {
		cp := *u
		result[i] = &cp
	}
	return result
}

// GetStats 获取更新统计.
func (m *Manager) GetStats() *UpdateStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &UpdateStats{}
	if len(m.history) == 0 {
		return stats
	}

	stats.TotalUpdates = len(m.history)
	stats.LastUpdateTime = m.history[len(m.history)-1].StartedAt

	for _, u := range m.history {
		switch u.Status {
		case StatusCompleted:
			stats.SuccessCount++
		case StatusFailed:
			stats.FailedCount++
		case StatusRolledBack:
			stats.RollbackCount++
		}
	}

	return stats
}

// ClearHistory 清除历史记录.
func (m *Manager) ClearHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updates = make(map[string]*ContainerUpdate)
	m.history = make([]*ContainerUpdate, 0)
}

// ========== 内部方法 ==========

// checkImageUpdate 检查镜像是否有更新（模拟）.
func (m *Manager) checkImageUpdate(image string, currentTag string) (bool, string) {
	// 模拟：总是返回有更新
	// 实际实现应该查询 Docker Registry
	return true, currentTag + "-updated"
}

// pullImage 拉取镜像（模拟）.
func (m *Manager) pullImage(image string, tag string) error {
	// 模拟拉取延迟
	time.Sleep(100 * time.Millisecond)
	return nil
}

// stopContainer 停止容器（模拟）.
func (m *Manager) stopContainer(name string) {
	// 模拟停止延迟
	time.Sleep(50 * time.Millisecond)
}

// startContainer 启动容器（模拟）.
func (m *Manager) startContainer(name string, image string, tag string) error {
	// 模拟启动延迟
	time.Sleep(50 * time.Millisecond)
	return nil
}

// performHealthCheck 执行健康检查（模拟）.
func (m *Manager) performHealthCheck(container ContainerConfig) error {
	// 模拟健康检查
	time.Sleep(container.HealthCheck.Interval)
	return nil
}

// performRollback 执行回滚.
func (m *Manager) performRollback(container ContainerConfig, failedUpdate *ContainerUpdate) {
	// 查找上一个成功的版本
	m.mu.RLock()
	var lastGoodTag string
	for i := len(m.history) - 1; i >= 0; i-- {
		if m.history[i].ContainerID == container.ID && m.history[i].Status == StatusCompleted {
			lastGoodTag = m.history[i].OldTag
			break
		}
	}
	m.mu.RUnlock()

	if lastGoodTag == "" {
		lastGoodTag = "latest"
	}

	// 停止失败的容器
	m.stopContainer(container.Name)

	// 启动旧版本
	m.startContainer(container.Name, container.Image, lastGoodTag)

	// 更新状态
	failedUpdate.Status = StatusRolledBack
	failedUpdate.Error = failedUpdate.Error + " (auto-rolled back)"
}
