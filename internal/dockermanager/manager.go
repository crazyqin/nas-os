// Package dockermanager Docker 容器管理模块
// 学习群晖 Container Manager / TrueNAS Apps
package dockermanager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Container 容器信息
type Container struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Tag         string            `json:"tag"`
	Status      string            `json:"status"` // running, stopped, paused, created
	State       string            `json:"state"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   time.Time         `json:"started_at,omitempty"`
	FinishedAt  time.Time         `json:"finished_at,omitempty"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMount     `json:"volumes"`
	Env         map[string]string `json:"env"`
	Labels      map[string]string `json:"labels"`
	Networks    []string          `json:"networks"`
	Resources   ResourceLimits    `json:"resources"`
	RestartPolicy string          `json:"restart_policy"`
	HealthCheck *HealthCheck       `json:"health_check,omitempty"`
	Logs        []LogEntry        `json:"logs,omitempty"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // tcp, udp
	HostIP        string `json:"host_ip,omitempty"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
	VolumeName    string `json:"volume_name,omitempty"`
}

// ResourceLimits 资源限制
type ResourceLimits struct {
	CPUShares    int64   `json:"cpu_shares"`
	MemoryLimit  int64   `json:"memory_limit"`  // bytes
	MemorySwap   int64   `json:"memory_swap"`
	CPUQuota     int64   `json:"cpu_quota"`
	CPUPeriod    int64   `json:"cpu_period"`
	BlkioWeight  int     `json:"blkio_weight"`
}

// HealthCheck 健康检查
type HealthCheck struct {
	Test        []string      `json:"test"`
	Interval    time.Duration `json:"interval"`
	Timeout     time.Duration `json:"timeout"`
	Retries     int           `json:"retries"`
	StartPeriod time.Duration `json:"start_period"`
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"` // stdout, stderr
	Message   string    `json:"message"`
}

// Stack Compose 栈
type Stack struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	Services   map[string]Service    `json:"services"`
	Networks   map[string]Network    `json:"networks"`
	Volumes    map[string]Volume     `json:"volumes"`
	Status     string                `json:"status"`
	ComposeFile string               `json:"compose_file"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

// Service 服务定义
type Service struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Ports        []PortMapping     `json:"ports"`
	Volumes      []VolumeMount     `json:"volumes"`
	Env          map[string]string `json:"env"`
	Labels       map[string]string `json:"labels"`
	DependsOn    []string          `json:"depends_on"`
	Networks     []string          `json:"networks"`
	Resources    ResourceLimits    `json:"resources"`
	Replicas     int               `json:"replicas"`
	RestartPolicy string           `json:"restart_policy"`
}

// Network 网络定义
type Network struct {
	Name     string            `json:"name"`
	Driver   string            `json:"driver"`
	IPAM     IPAM              `json:"ipam"`
	Labels   map[string]string `json:"labels"`
	External bool              `json:"external"`
}

// IPAM IP 地址管理
type IPAM struct {
	Driver string      `json:"driver"`
	Config []IPAMConfig `json:"config"`
}

// IPAMConfig IPAM 配置
type IPAMConfig struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
	IPRange string `json:"ip_range,omitempty"`
}

// Volume 卷定义
type Volume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	DriverOpts map[string]string `json:"driver_opts"`
	Labels     map[string]string `json:"labels"`
	External   bool              `json:"external"`
}

// AppTemplate 应用模板（应用商店）
type AppTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Icon        string            `json:"icon"`
	Image       string            `json:"image"`
	Version     string            `json:"version"`
	Tags        []string          `json:"tags"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMount     `json:"volumes"`
	Env         map[string]string `json:"env"`
	EnvTemplate []EnvTemplate     `json:"env_template"`
	MinCPU      float64           `json:"min_cpu"`
	MinMemory   int64             `json:"min_memory"`
	Author      string            `json:"author"`
	License     string            `json:"license"`
	Website     string            `json:"website"`
	Stars       int               `json:"stars"`
	Downloads   int               `json:"downloads"`
}

// EnvTemplate 环境变量模板
type EnvTemplate struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Default     string `json:"default"`
	Required    bool   `json:"required"`
	Type        string `json:"type"` // string, number, boolean, password
}

// Manager Docker 管理器
type Manager struct {
	mu         sync.RWMutex
	containers map[string]*Container
	stacks     map[string]*Stack
	templates  map[string]*AppTemplate
	networks   map[string]*Network
	volumes    map[string]*Volume
}

// NewManager 创建 Docker 管理器
func NewManager() *Manager {
	m := &Manager{
		containers: make(map[string]*Container),
		stacks:     make(map[string]*Stack),
		templates:  make(map[string]*AppTemplate),
		networks:   make(map[string]*Network),
		volumes:    make(map[string]*Volume),
	}
	m.loadDefaultTemplates()
	return m
}

// ListContainers 列出容器
func (m *Manager) ListContainers(ctx context.Context, all bool) ([]Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	containers := make([]Container, 0)
	for _, c := range m.containers {
		if all || c.Status == "running" {
			containers = append(containers, *c)
		}
	}

	return containers, nil
}

// GetContainer 获取容器详情
func (m *Manager) GetContainer(ctx context.Context, id string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, exists := m.containers[id]
	if !exists {
		return nil, fmt.Errorf("container not found: %s", id)
	}

	return container, nil
}

// CreateContainer 创建容器
func (m *Manager) CreateContainer(ctx context.Context, name, image, tag string, opts ...ContainerOption) (*Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tag == "" {
		tag = "latest"
	}

	container := &Container{
		ID:        generateID(),
		Name:      name,
		Image:     image,
		Tag:       tag,
		Status:    "created",
		CreatedAt: time.Now(),
		Env:       make(map[string]string),
		Labels:    make(map[string]string),
		Resources: ResourceLimits{
			CPUShares:   1024,
			MemoryLimit: 512 * 1024 * 1024, // 512MB
		},
		RestartPolicy: "unless-stopped",
	}

	// 应用选项
	for _, opt := range opts {
		opt(container)
	}

	m.containers[container.ID] = container
	log.Printf("Container created: %s (%s)", name, container.ID)

	return container, nil
}

// StartContainer 启动容器
func (m *Manager) StartContainer(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("container not found: %s", id)
	}

	if container.Status == "running" {
		return fmt.Errorf("container already running: %s", id)
	}

	container.Status = "running"
	container.StartedAt = time.Now()
	log.Printf("Container started: %s", container.Name)

	return nil
}

// StopContainer 停止容器
func (m *Manager) StopContainer(ctx context.Context, id string, timeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("container not found: %s", id)
	}

	if container.Status != "running" {
		return fmt.Errorf("container not running: %s", id)
	}

	container.Status = "stopped"
	container.FinishedAt = time.Now()
	log.Printf("Container stopped: %s", container.Name)

	return nil
}

// RestartContainer 重启容器
func (m *Manager) RestartContainer(ctx context.Context, id string, timeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("container not found: %s", id)
	}

	container.Status = "running"
	container.StartedAt = time.Now()
	log.Printf("Container restarted: %s", container.Name)

	return nil
}

// RemoveContainer 删除容器
func (m *Manager) RemoveContainer(ctx context.Context, id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("container not found: %s", id)
	}

	if container.Status == "running" && !force {
		return fmt.Errorf("container is running, use force=true to remove")
	}

	delete(m.containers, id)
	log.Printf("Container removed: %s", container.Name)

	return nil
}

// GetContainerLogs 获取容器日志
func (m *Manager) GetContainerLogs(ctx context.Context, id string, tail int) ([]LogEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, exists := m.containers[id]
	if !exists {
		return nil, fmt.Errorf("container not found: %s", id)
	}

	if tail <= 0 || tail > len(container.Logs) {
		return container.Logs, nil
	}

	return container.Logs[len(container.Logs)-tail:], nil
}

// DeployStack 部署 Compose 栈
func (m *Manager) DeployStack(ctx context.Context, name, composeContent string) (*Stack, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 解析 compose 文件
	stack := &Stack{
		ID:          generateID(),
		Name:        name,
		Services:    make(map[string]Service),
		Networks:    make(map[string]Network),
		Volumes:     make(map[string]Volume),
		Status:      "deploying",
		ComposeFile: composeContent,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// TODO: 实际解析 compose 文件
	// 简化实现
	stack.Status = "running"
	m.stacks[stack.ID] = stack

	log.Printf("Stack deployed: %s", name)
	return stack, nil
}

// ListStacks 列出栈
func (m *Manager) ListStacks(ctx context.Context) ([]Stack, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stacks := make([]Stack, 0, len(m.stacks))
	for _, s := range m.stacks {
		stacks = append(stacks, *s)
	}

	return stacks, nil
}

// RemoveStack 删除栈
func (m *Manager) RemoveStack(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stack, exists := m.stacks[id]
	if !exists {
		return fmt.Errorf("stack not found: %s", id)
	}

	delete(m.stacks, id)
	log.Printf("Stack removed: %s", stack.Name)

	return nil
}

// ListTemplates 列出应用模板
func (m *Manager) ListTemplates(ctx context.Context, category string) ([]AppTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]AppTemplate, 0)
	for _, t := range m.templates {
		if category == "" || t.Category == category {
			templates = append(templates, *t)
		}
	}

	return templates, nil
}

// InstallTemplate 安装应用模板
func (m *Manager) InstallTemplate(ctx context.Context, templateID string, envVars map[string]string) (*Stack, error) {
	m.mu.RLock()
	template, exists := m.templates[templateID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	// 合并环境变量
	env := make(map[string]string)
	for k, v := range template.Env {
		env[k] = v
	}
	for k, v := range envVars {
		env[k] = v
	}

	// 创建栈
	stack := &Stack{
		ID:       generateID(),
		Name:     template.Name,
		Services: map[string]Service{
			template.Name: {
				Name:  template.Name,
				Image: template.Image,
				Ports: template.Ports,
				Volumes: template.Volumes,
				Env:   env,
			},
		},
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.mu.Lock()
	m.stacks[stack.ID] = stack
	m.mu.Unlock()

	log.Printf("Template installed: %s", template.Name)
	return stack, nil
}

// CreateNetwork 创建网络
func (m *Manager) CreateNetwork(ctx context.Context, name, driver string) (*Network, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	network := &Network{
		Name:   name,
		Driver: driver,
		Labels: make(map[string]string),
	}

	m.networks[name] = network
	log.Printf("Network created: %s", name)

	return network, nil
}

// CreateVolume 创建卷
func (m *Manager) CreateVolume(ctx context.Context, name, driver string) (*Volume, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	volume := &Volume{
		Name:       name,
		Driver:     driver,
		DriverOpts: make(map[string]string),
		Labels:     make(map[string]string),
	}

	m.volumes[name] = volume
	log.Printf("Volume created: %s", name)

	return volume, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats(ctx context.Context) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runningCount := 0
	stoppedCount := 0
	for _, c := range m.containers {
		if c.Status == "running" {
			runningCount++
		} else {
			stoppedCount++
		}
	}

	return map[string]interface{}{
		"containers_running": runningCount,
		"containers_stopped": stoppedCount,
		"stacks":            len(m.stacks),
		"networks":          len(m.networks),
		"volumes":           len(m.volumes),
		"templates":         len(m.templates),
	}, nil
}

// ContainerOption 容器选项函数
type ContainerOption func(*Container)

// WithPorts 设置端口映射
func WithPorts(ports []PortMapping) ContainerOption {
	return func(c *Container) {
		c.Ports = ports
	}
}

// WithVolumes 设置卷挂载
func WithVolumes(volumes []VolumeMount) ContainerOption {
	return func(c *Container) {
		c.Volumes = volumes
	}
}

// WithEnv 设置环境变量
func WithEnv(env map[string]string) ContainerOption {
	return func(c *Container) {
		for k, v := range env {
			c.Env[k] = v
		}
	}
}

// WithResourceLimits 设置资源限制
func WithResourceLimits(limits ResourceLimits) ContainerOption {
	return func(c *Container) {
		c.Resources = limits
	}
}

// WithRestartPolicy 设置重启策略
func WithRestartPolicy(policy string) ContainerOption {
	return func(c *Container) {
		c.RestartPolicy = policy
	}
}

func (m *Manager) loadDefaultTemplates() {
	// 加载默认应用模板
	defaultTemplates := []*AppTemplate{
		{
			ID:          "nginx",
			Name:        "Nginx",
			Description: "High-performance HTTP server and reverse proxy",
			Category:    "web",
			Image:       "nginx:latest",
			Ports:       []PortMapping{{HostPort: 80, ContainerPort: 80}},
			MinCPU:      0.5,
			MinMemory:   128 * 1024 * 1024,
			Author:      "Nginx",
			License:     "BSD-2-Clause",
		},
		{
			ID:          "redis",
			Name:        "Redis",
			Description: "In-memory data structure store",
			Category:    "database",
			Image:       "redis:latest",
			Ports:       []PortMapping{{HostPort: 6379, ContainerPort: 6379}},
			MinCPU:      0.5,
			MinMemory:   256 * 1024 * 1024,
			Author:      "Redis",
			License:     "BSD-3-Clause",
		},
		{
			ID:          "postgres",
			Name:        "PostgreSQL",
			Description: "Advanced open source relational database",
			Category:    "database",
			Image:       "postgres:16",
			Ports:       []PortMapping{{HostPort: 5432, ContainerPort: 5432}},
			MinCPU:      1.0,
			MinMemory:   512 * 1024 * 1024,
			Author:      "PostgreSQL",
			License:     "PostgreSQL",
		},
	}

	for _, t := range defaultTemplates {
		m.templates[t.ID] = t
	}
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Export 导出容器和栈信息
func (m *Manager) Export(ctx context.Context) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := map[string]interface{}{
		"containers": m.containers,
		"stacks":     m.stacks,
		"networks":   m.networks,
		"volumes":    m.volumes,
	}

	return json.MarshalIndent(data, "", "  ")
}
