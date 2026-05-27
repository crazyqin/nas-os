package dockergui

import (
	"fmt"
	"sync"
	"time"
)

// ContainerState 容器状态
type ContainerState string

const (
	StateRunning  ContainerState = "running"
	StateStopped  ContainerState = "stopped"
	StatePaused   ContainerState = "paused"
	StateCreating ContainerState = "creating"
	StateError    ContainerState = "error"
)

// Container Docker容器
type Container struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Image       string         `json:"image"`
	State       ContainerState `json:"state"`
	Status      string         `json:"status"`
	Ports       []PortMapping  `json:"ports"`
	Volumes     []string       `json:"volumes"`
	Networks    []string       `json:"networks"`
	CPUPercent  float64        `json:"cpuPercent"`
	MemBytes    int64          `json:"memBytes"`
	MemLimit    int64          `json:"memLimit"`
	CreatedAt   time.Time      `json:"createdAt"`
	StartedAt   time.Time      `json:"startedAt"`
	Restarts    int            `json:"restarts"`
	Health      string         `json:"health"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIp"`
}

// ComposeProject Docker Compose项目
type ComposeProject struct {
	Name       string             `json:"name"`
	Services   []ComposeService   `json:"services"`
	Networks   []string           `json:"networks"`
	Volumes    []string           `json:"volumes"`
	Status     string             `json:"status"`
	ConfigPath string             `json:"configPath"`
	CreatedAt  time.Time          `json:"createdAt"`
}

// ComposeService Compose服务
type ComposeService struct {
	Name      string         `json:"name"`
	Image     string         `json:"image"`
	State     ContainerState `json:"state"`
	Replicas  int            `json:"replicas"`
	Ports     []PortMapping  `json:"ports"`
	Volumes   []string       `json:"volumes"`
	Env       []string       `json:"env"`
	DependsOn []string       `json:"dependsOn"`
}

// Image Docker镜像
type Image struct {
	ID         string    `json:"id"`
	Repository string    `json:"repository"`
	Tag        string    `json:"tag"`
	SizeBytes  int64     `json:"sizeBytes"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Network Docker网络
type Network struct {
	Name     string   `json:"name"`
	Driver   string   `json:"driver"`
	Subnet   string   `json:"subnet"`
	Gateway  string   `json:"gateway"`
	IPRange  string   `json:"ipRange"`
	Internal bool     `json:"internal"`
}

// Manager Docker管理器
type Manager struct {
	mu         sync.RWMutex
	containers map[string]*Container
	projects   map[string]*ComposeProject
	images     map[string]*Image
	networks   map[string]*Network
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		containers: make(map[string]*Container),
		projects:   make(map[string]*ComposeProject),
		images:     make(map[string]*Image),
		networks:   make(map[string]*Network),
	}
}

// ListContainers 列出容器
func (m *Manager) ListContainers(all bool) []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	containers := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		if !all && c.State != StateRunning {
			continue
		}
		containers = append(containers, c)
	}
	return containers
}

// GetContainer 获取容器详情
func (m *Manager) GetContainer(id string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.containers[id]
	if !ok {
		return nil, fmt.Errorf("container %s not found", id)
	}
	return c, nil
}

// StartContainer 启动容器
func (m *Manager) StartContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container %s not found", id)
	}
	c.State = StateRunning
	c.StartedAt = time.Now()
	return nil
}

// StopContainer 停止容器
func (m *Manager) StopContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container %s not found", id)
	}
	c.State = StateStopped
	return nil
}

// RestartContainer 重启容器
func (m *Manager) RestartContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container %s not found", id)
	}
	c.State = StateRunning
	c.Restarts++
	c.StartedAt = time.Now()
	return nil
}

// RemoveContainer 删除容器
func (m *Manager) RemoveContainer(id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.containers[id]; !ok {
		return fmt.Errorf("container %s not found", id)
	}
	delete(m.containers, id)
	return nil
}

// GetContainerLogs 获取容器日志
func (m *Manager) GetContainerLogs(id string, tail int) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.containers[id]; !ok {
		return nil, fmt.Errorf("container %s not found", id)
	}
	return []string{}, nil
}

// GetContainerStats 获取容器资源使用
func (m *Manager) GetContainerStats(id string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.containers[id]
	if !ok {
		return nil, fmt.Errorf("container %s not found", id)
	}
	return map[string]interface{}{
		"cpuPercent": c.CPUPercent,
		"memBytes":   c.MemBytes,
		"memLimit":   c.MemLimit,
	}, nil
}

// ListComposeProjects 列出Compose项目
func (m *Manager) ListComposeProjects() []*ComposeProject {
	m.mu.RLock()
	defer m.mu.RUnlock()
	projects := make([]*ComposeProject, 0, len(m.projects))
	for _, p := range m.projects {
		projects = append(projects, p)
	}
	return projects
}

// GetComposeProject 获取Compose项目
func (m *Manager) GetComposeProject(name string) (*ComposeProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.projects[name]
	if !ok {
		return nil, fmt.Errorf("project %s not found", name)
	}
	return p, nil
}

// DeployCompose 部署Compose项目
func (m *Manager) DeployCompose(name, config string) (*ComposeProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.projects[name]; exists {
		return nil, fmt.Errorf("project %s already exists", name)
	}
	project := &ComposeProject{
		Name:      name,
		Status:    "running",
		CreatedAt: time.Now(),
	}
	m.projects[name] = project
	return project, nil
}

// StopCompose 停止Compose项目
func (m *Manager) StopCompose(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.projects[name]
	if !ok {
		return fmt.Errorf("project %s not found", name)
	}
	p.Status = "stopped"
	return nil
}

// RemoveCompose 删除Compose项目
func (m *Manager) RemoveCompose(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.projects[name]; !ok {
		return fmt.Errorf("project %s not found", name)
	}
	delete(m.projects, name)
	return nil
}

// ListImages 列出镜像
func (m *Manager) ListImages() []*Image {
	m.mu.RLock()
	defer m.mu.RUnlock()
	images := make([]*Image, 0, len(m.images))
	for _, img := range m.images {
		images = append(images, img)
	}
	return images
}

// PullImage 拉取镜像
func (m *Manager) PullImage(repository, tag string) (*Image, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	img := &Image{
		ID:         fmt.Sprintf("%s:%s", repository, tag),
		Repository: repository,
		Tag:        tag,
		CreatedAt:  time.Now(),
	}
	m.images[img.ID] = img
	return img, nil
}

// RemoveImage 删除镜像
func (m *Manager) RemoveImage(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.images[id]; !ok {
		return fmt.Errorf("image %s not found", id)
	}
	delete(m.images, id)
	return nil
}

// ListNetworks 列出网络
func (m *Manager) ListNetworks() []*Network {
	m.mu.RLock()
	defer m.mu.RUnlock()
	networks := make([]*Network, 0, len(m.networks))
	for _, n := range m.networks {
		networks = append(networks, n)
	}
	return networks
}

// CreateNetwork 创建网络
func (m *Manager) CreateNetwork(name, driver, subnet string) (*Network, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.networks[name]; exists {
		return nil, fmt.Errorf("network %s already exists", name)
	}
	n := &Network{
		Name:   name,
		Driver: driver,
		Subnet: subnet,
	}
	m.networks[name] = n
	return n, nil
}
