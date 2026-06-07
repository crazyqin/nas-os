// Package containerpro 提供容器管理功能
package containerpro

import (
	"fmt"
	"sync"
	"time"
)

// Manager 容器管理器
type Manager struct {
	mu              sync.RWMutex
	containers      map[string]*Container
	composeProjects map[string]*ComposeProject
	images          map[string]*ImageInfo
	registries      map[string]*Registry
	config          *ContainerProConfig
}

// NewManager 创建新的容器管理器
func NewManager(config *ContainerProConfig) *Manager {
	return &Manager{
		containers:      make(map[string]*Container),
		composeProjects: make(map[string]*ComposeProject),
		images:          make(map[string]*ImageInfo),
		registries:      make(map[string]*Registry),
		config:          config,
	}
}

// ListContainers 列出所有容器
func (m *Manager) ListContainers(all bool) ([]Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var containers []Container
	for _, c := range m.containers {
		if !all && c.State != "running" {
			continue
		}
		containers = append(containers, *c)
	}
	return containers, nil
}

// GetContainer 获取容器详情
func (m *Manager) GetContainer(id string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, ok := m.containers[id]
	if !ok {
		return nil, fmt.Errorf("container not found: %s", id)
	}
	return container, nil
}

// StartContainer 启动容器
func (m *Manager) StartContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container not found: %s", id)
	}

	now := time.Now()
	container.State = "running"
	container.Status = "Up Less than a second"
	container.StartedAt = &now
	return nil
}

// StopContainer 停止容器
func (m *Manager) StopContainer(id string, timeout int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container not found: %s", id)
	}

	container.State = "exited"
	container.Status = fmt.Sprintf("Exited (0) %d seconds ago", timeout)
	container.StartedAt = nil
	return nil
}

// RestartContainer 重启容器
func (m *Manager) RestartContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container not found: %s", id)
	}

	now := time.Now()
	container.State = "running"
	container.Status = "Up Less than a second"
	container.StartedAt = &now
	container.RestartCount++
	return nil
}

// RemoveContainer 删除容器
func (m *Manager) RemoveContainer(id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container not found: %s", id)
	}

	if !force && container.State == "running" {
		return fmt.Errorf("container is running: %s (use force=true to remove)", id)
	}

	delete(m.containers, id)
	return nil
}

// GetContainerStats 获取容器统计信息
func (m *Manager) GetContainerStats(id string) (*ContainerStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, ok := m.containers[id]
	if !ok {
		return nil, fmt.Errorf("container not found: %s", id)
	}

	stats := &ContainerStats{
		CPU: CPUStats{
			Usage:       container.CPUUsage,
			SystemUsage: time.Now().UnixNano(),
			OnlineCPUs:  4,
		},
		Memory: MemoryStats{
			Usage:    container.MemoryUsage,
			MaxUsage: container.MemoryUsage,
			Limit:    container.MemoryLimit,
		},
		Network: container.NetworkIO,
		BlockIO: BlockIOStats{
			ReadBytes:  1024 * 1024,
			WriteBytes: 512 * 1024,
			ReadOps:    1000,
			WriteOps:   500,
		},
		PIDs: 10,
	}
	return stats, nil
}

// DeployComposeProject 部署 Compose 项目
func (m *Manager) DeployComposeProject(path string) (*ComposeProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	project := &ComposeProject{
		ID:        fmt.Sprintf("compose-%d", now.UnixNano()),
		Name:      fmt.Sprintf("project-%d", now.UnixNano()),
		Path:      path,
		Status:    "running",
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.composeProjects[project.ID] = project
	return project, nil
}

// ListComposeProjects 列出所有 Compose 项目
func (m *Manager) ListComposeProjects() ([]ComposeProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var projects []ComposeProject
	for _, p := range m.composeProjects {
		projects = append(projects, *p)
	}
	return projects, nil
}

// StopComposeProject 停止 Compose 项目
func (m *Manager) StopComposeProject(projectID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, ok := m.composeProjects[projectID]
	if !ok {
		return fmt.Errorf("compose project not found: %s", projectID)
	}

	project.Status = "stopped"
	project.UpdatedAt = time.Now()
	return nil
}

// PullImage 拉取镜像
func (m *Manager) PullImage(image string, registryID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找 registry
	if registryID != "" {
		if _, ok := m.registries[registryID]; !ok {
			return fmt.Errorf("registry not found: %s", registryID)
		}
	}

	// 模拟拉取镜像
	now := time.Now()
	imageInfo := &ImageInfo{
		ID:        fmt.Sprintf("image-%d", now.UnixNano()),
		RepoTags:  []string{image},
		Size:      100 * 1024 * 1024, // 100MB
		CreatedAt: now,
	}

	m.images[imageInfo.ID] = imageInfo
	return nil
}

// ListImages 列出所有镜像
func (m *Manager) ListImages() ([]ImageInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var images []ImageInfo
	for _, img := range m.images {
		images = append(images, *img)
	}
	return images, nil
}

// AddRegistry 添加镜像仓库
func (m *Manager) AddRegistry(registry Registry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if registry.ID == "" {
		registry.ID = fmt.Sprintf("registry-%d", time.Now().UnixNano())
	}

	m.registries[registry.ID] = &registry
	return nil
}

// ListRegistries 列出所有镜像仓库
func (m *Manager) ListRegistries() ([]Registry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var registries []Registry
	for _, r := range m.registries {
		registries = append(registries, *r)
	}
	return registries, nil
}
