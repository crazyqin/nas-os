// Package lxcmanager 实现LXC容器管理
// 对标 TrueNAS 25.04 LXC/Incus 容器支持
// 支持容器生命周期管理、资源限制、网络配置
package lxcmanager

import (
	"fmt"
	"sync"
	"time"
)

// ContainerState 容器状态
type ContainerState int

const (
	ContainerStateStopped ContainerState = iota
	ContainerStateStarting
	ContainerStateRunning
	ContainerStateStopping
	ContainerStateError
	ContainerStateFrozen
)

// ContainerType 容器类型
type ContainerType int

const (
	ContainerTypeLXC ContainerType = iota
	ContainerTypeDocker
	ContainerTypeVM
)

// Container 容器
type Container struct {
	ID          string
	Name        string
	Type        ContainerType
	State       ContainerState
	Image       string
	Command     []string
	Args        []string
	Env         map[string]string
	Labels      map[string]string
	Resources   *ResourceConfig
	Network     *NetworkConfig
	Volumes     []*VolumeMount
	Ports       []*PortMapping
	CreatedAt   time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
	ExitCode    int
	PID         int
	CPUUsage    float64
	MemoryUsage int64
	Metadata    map[string]string
}

// ResourceConfig 资源配置
type ResourceConfig struct {
	CPULimit      float64 // CPU限制（核心数）
	MemoryLimit   int64   // 内存限制（字节）
	MemorySwap    int64   // 内存+交换限制
	CPUShares     int     // CPU份额
	CPUSet        string  // CPU集合
	IOWeight      int     // IO权重
	PidsLimit     int     // 进程数限制
	StorageLimit  int64   // 存储限制
	NetworkBandwidth int64 // 网络带宽限制
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	Hostname    string
	DomainName  string
	DNSServers  []string
	SearchDomains []string
	Interfaces  []*NetworkInterface
	Gateway     string
	IPV4Address string
	IPV6Address string
	NetworkMode string
	EnableSSH   bool
}

// NetworkInterface 网络接口
type NetworkInterface struct {
	Name      string
	Type      string // bridge, macvlan, physical
	Parent    string
	IPAddress string
	Netmask   string
	Gateway   string
	MAC       string
	MTU       int
}

// VolumeMount 卷挂载
type VolumeMount struct {
	Source      string
	Destination string
	ReadOnly    bool
	Type        string // bind, volume, tmpfs
	Options     []string
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string // tcp, udp
	HostIP        string
}

// ContainerManager 容器管理器
type ContainerManager struct {
	mu         sync.RWMutex
	config     ManagerConfig
	containers map[string]*Container
	stats      ManagerStats
	running    bool
	stopCh     chan struct{}
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	DefaultImage     string
	RootDir          string
	LogDir           string
	MaxContainers    int
	EnableCheckpoint bool
	EnableSnapshot   bool
	DefaultResources *ResourceConfig
	NetworkBridge    string
	DNSServer        string
}

// ManagerStats 管理器统计
type ManagerStats struct {
	TotalContainers   int
	RunningContainers int
	StoppedContainers int
	ErrorContainers   int
	TotalCPUUsage     float64
	TotalMemoryUsage  int64
	TotalNetworkRx    int64
	TotalNetworkTx    int64
}

// NewContainerManager 创建容器管理器
func NewContainerManager(config ManagerConfig) *ContainerManager {
	if config.MaxContainers <= 0 {
		config.MaxContainers = 100
	}
	if config.DefaultImage == "" {
		config.DefaultImage = "ubuntu:22.04"
	}
	if config.RootDir == "" {
		config.RootDir = "/var/lib/lxc"
	}
	if config.LogDir == "" {
		config.LogDir = "/var/log/lxc"
	}
	if config.NetworkBridge == "" {
		config.NetworkBridge = "lxcbr0"
	}
	if config.DNSServer == "" {
		config.DNSServer = "8.8.8.8"
	}

	return &ContainerManager{
		config:     config,
		containers: make(map[string]*Container),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动容器管理器
func (m *ContainerManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("container manager already running")
	}

	m.running = true
	go m.monitorLoop()

	return nil
}

// Stop 停止容器管理器
func (m *ContainerManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	close(m.stopCh)
	m.running = false

	// 停止所有运行中的容器
	for _, container := range m.containers {
		if container.State == ContainerStateRunning {
			m.stopContainer(container.ID)
		}
	}

	return nil
}

// CreateContainer 创建容器
func (m *ContainerManager) CreateContainer(name string, config *Container) (*Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查容器数量限制
	if len(m.containers) >= m.config.MaxContainers {
		return nil, fmt.Errorf("maximum containers reached: %d", m.config.MaxContainers)
	}

	// 检查名称是否已存在
	for _, c := range m.containers {
		if c.Name == name {
			return nil, fmt.Errorf("container name already exists: %s", name)
		}
	}

	// 生成容器ID
	id := fmt.Sprintf("lxc-%d", time.Now().UnixNano())

	// 设置默认值
	if config.Image == "" {
		config.Image = m.config.DefaultImage
	}
	if config.Resources == nil {
		config.Resources = m.config.DefaultResources
	}
	if config.Env == nil {
		config.Env = make(map[string]string)
	}
	if config.Labels == nil {
		config.Labels = make(map[string]string)
	}
	if config.Metadata == nil {
		config.Metadata = make(map[string]string)
	}

	container := &Container{
		ID:        id,
		Name:      name,
		Type:      config.Type,
		State:     ContainerStateStopped,
		Image:     config.Image,
		Command:   config.Command,
		Args:      config.Args,
		Env:       config.Env,
		Labels:    config.Labels,
		Resources: config.Resources,
		Network:   config.Network,
		Volumes:   config.Volumes,
		Ports:     config.Ports,
		CreatedAt: time.Now(),
		Metadata:  config.Metadata,
	}

	m.containers[id] = container
	m.stats.TotalContainers++
	m.stats.StoppedContainers++

	return container, nil
}

// StartContainer 启动容器
func (m *ContainerManager) StartContainer(id string) error {
	m.mu.Lock()
	container, exists := m.containers[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("container not found: %s", id)
	}

	if container.State == ContainerStateRunning {
		m.mu.Unlock()
		return fmt.Errorf("container already running: %s", id)
	}

	container.State = ContainerStateStarting
	m.mu.Unlock()

	// 模拟容器启动
	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	container.StartedAt = &now
	container.State = ContainerStateRunning
	container.PID = int(time.Now().UnixNano() % 10000)
	m.stats.RunningContainers++
	m.stats.StoppedContainers--

	return nil
}

// StopContainer 停止容器
func (m *ContainerManager) StopContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.stopContainer(id)
}

// stopContainer 内部停止容器
func (m *ContainerManager) stopContainer(id string) error {
	container, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("container not found: %s", id)
	}

	if container.State != ContainerStateRunning {
		return fmt.Errorf("container not running: %s", id)
	}

	container.State = ContainerStateStopping

	// 模拟容器停止
	time.Sleep(50 * time.Millisecond)

	now := time.Now()
	container.FinishedAt = &now
	container.State = ContainerStateStopped
	container.ExitCode = 0
	m.stats.RunningContainers--
	m.stats.StoppedContainers++

	return nil
}

// RestartContainer 重启容器
func (m *ContainerManager) RestartContainer(id string) error {
	if err := m.StopContainer(id); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	return m.StartContainer(id)
}

// DeleteContainer 删除容器
func (m *ContainerManager) DeleteContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("container not found: %s", id)
	}

	if container.State == ContainerStateRunning {
		return fmt.Errorf("container is running, stop it first: %s", id)
	}

	delete(m.containers, id)
	m.stats.TotalContainers--
	m.stats.StoppedContainers--

	return nil
}

// GetContainer 获取容器信息
func (m *ContainerManager) GetContainer(id string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, exists := m.containers[id]
	if !exists {
		return nil, fmt.Errorf("container not found: %s", id)
	}

	return container, nil
}

// GetContainerByName 根据名称获取容器
func (m *ContainerManager) GetContainerByName(name string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, container := range m.containers {
		if container.Name == name {
			return container, nil
		}
	}

	return nil, fmt.Errorf("container not found: %s", name)
}

// ListContainers 列出所有容器
func (m *ContainerManager) ListContainers() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()

	containers := make([]*Container, 0, len(m.containers))
	for _, container := range m.containers {
		containers = append(containers, container)
	}

	return containers
}

// ListRunningContainers 列出运行中的容器
func (m *ContainerManager) ListRunningContainers() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()

	containers := make([]*Container, 0)
	for _, container := range m.containers {
		if container.State == ContainerStateRunning {
			containers = append(containers, container)
		}
	}

	return containers
}

// GetStats 获取统计信息
func (m *ContainerManager) GetStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// FreezeContainer 冻结容器
func (m *ContainerManager) FreezeContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("container not found: %s", id)
	}

	if container.State != ContainerStateRunning {
		return fmt.Errorf("container not running: %s", id)
	}

	container.State = ContainerStateFrozen

	return nil
}

// UnfreezeContainer 解冻容器
func (m *ContainerManager) UnfreezeContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("container not found: %s", id)
	}

	if container.State != ContainerStateFrozen {
		return fmt.Errorf("container not frozen: %s", id)
	}

	container.State = ContainerStateRunning

	return nil
}

// UpdateResources 更新容器资源
func (m *ContainerManager) UpdateResources(id string, resources *ResourceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("container not found: %s", id)
	}

	container.Resources = resources

	return nil
}

// monitorLoop 监控循环
func (m *ContainerManager) monitorLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.monitor()
		}
	}
}

// monitor 监控容器状态
func (m *ContainerManager) monitor() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新容器资源使用
	for _, container := range m.containers {
		if container.State == ContainerStateRunning {
			// 模拟资源使用更新
			container.CPUUsage = 0.1
			container.MemoryUsage = 100 * 1024 * 1024 // 100MB
		}
	}

	// 更新统计
	var totalCPU float64
	var totalMem int64
	var running, stopped, errored int

	for _, container := range m.containers {
		switch container.State {
		case ContainerStateRunning:
			running++
			totalCPU += container.CPUUsage
			totalMem += container.MemoryUsage
		case ContainerStateStopped:
			stopped++
		case ContainerStateError:
			errored++
		}
	}

	m.stats.RunningContainers = running
	m.stats.StoppedContainers = stopped
	m.stats.ErrorContainers = errored
	m.stats.TotalCPUUsage = totalCPU
	m.stats.TotalMemoryUsage = totalMem
}

// DefaultManagerConfig 默认管理器配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		DefaultImage:  "ubuntu:22.04",
		RootDir:       "/var/lib/lxc",
		LogDir:        "/var/log/lxc",
		MaxContainers: 100,
		EnableCheckpoint: false,
		EnableSnapshot: true,
		DefaultResources: &ResourceConfig{
			CPULimit:    1.0,
			MemoryLimit: 512 * 1024 * 1024, // 512MB
			MemorySwap:  1024 * 1024 * 1024, // 1GB
			CPUShares:   1024,
			IOWeight:    500,
			PidsLimit:   100,
		},
		NetworkBridge: "lxcbr0",
		DNSServer:     "8.8.8.8",
	}
}

// ContainerStateToString 转换容器状态为字符串
func ContainerStateToString(state ContainerState) string {
	switch state {
	case ContainerStateStopped:
		return "stopped"
	case ContainerStateStarting:
		return "starting"
	case ContainerStateRunning:
		return "running"
	case ContainerStateStopping:
		return "stopping"
	case ContainerStateError:
		return "error"
	case ContainerStateFrozen:
		return "frozen"
	default:
		return "unknown"
	}
}
