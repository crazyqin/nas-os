package lxcmanager

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ContainerStatus 容器状态
type ContainerStatus string

const (
	StatusCreated   ContainerStatus = "created"
	StatusRunning   ContainerStatus = "running"
	StatusStopped   ContainerStatus = "stopped"
	StatusPaused    ContainerStatus = "paused"
	StatusFrozen    ContainerStatus = "frozen"
	StatusError     ContainerStatus = "error"
	StatusMigrating ContainerStatus = "migrating"
)

// NetworkType 网络类型
type NetworkType string

const (
	NetBridge   NetworkType = "bridge"
	NetVLAN     NetworkType = "vlan"
	NetMACVLAN  NetworkType = "macvlan"
	NetPhysical NetworkType = "physical"
	NetNone     NetworkType = "none"
)

// ContainerConfig 容器配置
type ContainerConfig struct {
	Name        string            `json:"name"`
	Template    string            `json:"template"`
	Hostname    string            `json:"hostname"`
	CPULimit    int               `json:"cpuLimit"`    // CPU 核心数限制
	CPUShares   int               `json:"cpuShares"`   // CPU 权重 (1024 为基准)
	MemoryLimit int64             `json:"memoryLimit"` // 内存限制 (bytes)
	SwapLimit   int64             `json:"swapLimit"`   // Swap 限制 (bytes)
	DiskLimit   int64             `json:"diskLimit"`   // 磁盘限制 (bytes)
	Network     NetworkConfig     `json:"network"`
	Env         map[string]string `json:"env"`
	AutoStart   bool              `json:"autoStart"`
	Privileged  bool              `json:"privileged"`
	RootFSPath  string            `json:"rootfsPath"`
	Labels      map[string]string `json:"labels"`
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	Type      NetworkType `json:"type"`
	Bridge    string      `json:"bridge"`
	IPAddress string      `json:"ipAddress"`
	Gateway   string      `json:"gateway"`
	Subnet    string      `json:"subnet"`
	DNS       []string    `json:"dns"`
	VLANID    int         `json:"vlanId"`
	MAC       string      `json:"mac"`
}

// ResourceLimits 资源限制
type ResourceLimits struct {
	CPUQuota    int64  `json:"cpuQuota"`
	CPUPeriod   int64  `json:"cpuPeriod"`
	MemoryLimit int64  `json:"memoryLimit"`
	SwapLimit   int64  `json:"swapLimit"`
	PidsLimit   int64  `json:"pidsLimit"`
	BlkioWeight uint16 `json:"blkioWeight"`
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Status    ContainerStatus `json:"status"`
	PID       int             `json:"pid"`
	CreatedAt time.Time       `json:"createdAt"`
	StartedAt *time.Time      `json:"startedAt"`
	Config    ContainerConfig `json:"config"`
	Usage     ResourceUsage   `json:"usage"`
}

// ResourceUsage 资源使用量
type ResourceUsage struct {
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryUsed  int64   `json:"memoryUsed"`
	MemoryMax   int64   `json:"memoryMax"`
	DiskUsed    int64   `json:"diskUsed"`
	Pids        int64   `json:"pids"`
	NetInBytes  int64   `json:"netInBytes"`
	NetOutBytes int64   `json:"netOutBytes"`
}

// TemplateInfo 模板信息
type TemplateInfo struct {
	Name        string    `json:"name"`
	Alias       string    `json:"alias"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"createdAt"`
	Description string    `json:"description"`
}

// ManagerConfig 管器配置
type ManagerConfig struct {
	StoragePath   string `json:"storagePath"`
	BridgeName    string `json:"bridgeName"`
	DefaultSubnet string `json:"defaultSubnet"`
	MaxContainers int    `json:"maxContainers"`
	TemplatePath  string `json:"templatePath"`
	LogPath       string `json:"logPath"`
}

// DefaultManagerConfig 默认管理器配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		StoragePath:   "/var/lib/nas-os/lxcmanager",
		BridgeName:    "lxcmgr0",
		DefaultSubnet: "10.10.0.0/16",
		MaxContainers: 200,
		TemplatePath:  "/var/lib/nas-os/lxcmanager/templates",
		LogPath:       "/var/log/nas-os/lxcmanager",
	}
}

// Manager LXC 容器管理器
type Manager struct {
	mu         sync.RWMutex
	config     *ManagerConfig
	containers map[string]*ContainerInfo
	templates  map[string]*TemplateInfo
}

// NewManager 创建 LXC 容器管理器
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	return &Manager{
		config:     config,
		containers: make(map[string]*ContainerInfo),
		templates:  make(map[string]*TemplateInfo),
	}
}

// CreateContainer 创建容器
func (m *Manager) CreateContainer(ctx context.Context, cfg ContainerConfig) (*ContainerInfo, error) {
	// TODO: 实际调用 lxc-create / lxc sdk 创建容器
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.containers) >= m.config.MaxContainers {
		return nil, fmt.Errorf("已达最大容器数限制 %d", m.config.MaxContainers)
	}
	if _, exists := m.containers[cfg.Name]; exists {
		return nil, fmt.Errorf("容器 %s 已存在", cfg.Name)
	}
	if cfg.CPULimit <= 0 {
		cfg.CPULimit = 1
	}
	if cfg.MemoryLimit <= 0 {
		cfg.MemoryLimit = 512 * 1024 * 1024 // 512MB default
	}
	if cfg.Network.Type == "" {
		cfg.Network = NetworkConfig{
			Type:   NetBridge,
			Bridge: m.config.BridgeName,
		}
	}

	info := &ContainerInfo{
		ID:        generateID(),
		Name:      cfg.Name,
		Status:    StatusCreated,
		Config:    cfg,
		CreatedAt: time.Now(),
	}

	m.containers[cfg.Name] = info
	return info, nil
}

// DestroyContainer 销毁容器
func (m *Manager) DestroyContainer(ctx context.Context, name string) error {
	// TODO: 实际调用 lxc-destroy 销毁容器
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.containers[name]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", name)
	}
	if info.Status == StatusRunning {
		return fmt.Errorf("容器 %s 正在运行，请先停止", name)
	}

	delete(m.containers, name)
	return nil
}

// StartContainer 启动容器
func (m *Manager) StartContainer(ctx context.Context, name string) error {
	// TODO: 实际调用 lxc-start
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.containers[name]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", name)
	}
	if info.Status == StatusRunning {
		return fmt.Errorf("容器 %s 已在运行", name)
	}

	info.Status = StatusRunning
	now := time.Now()
	info.StartedAt = &now
	return nil
}

// StopContainer 停止容器
func (m *Manager) StopContainer(ctx context.Context, name string) error {
	// TODO: 实际调用 lxc-stop
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.containers[name]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", name)
	}
	if info.Status != StatusRunning && info.Status != StatusPaused {
		return fmt.Errorf("容器 %s 未在运行", name)
	}

	info.Status = StatusStopped
	return nil
}

// PauseContainer 暂停容器
func (m *Manager) PauseContainer(ctx context.Context, name string) error {
	// TODO: 实际调用 lxc-freeze
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.containers[name]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", name)
	}
	if info.Status != StatusRunning {
		return fmt.Errorf("只有运行中的容器可以暂停")
	}

	info.Status = StatusFrozen
	return nil
}

// GetContainer 获取容器信息
func (m *Manager) GetContainer(name string) (*ContainerInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.containers[name]
	if !exists {
		return nil, fmt.Errorf("容器 %s 不存在", name)
	}
	return info, nil
}

// ListContainers 列出所有容器
func (m *Manager) ListContainers() []*ContainerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ContainerInfo, 0, len(m.containers))
	for _, info := range m.containers {
		result = append(result, info)
	}
	return result
}

// SetResourceLimits 设置资源限制
func (m *Manager) SetResourceLimits(ctx context.Context, name string, limits ResourceLimits) error {
	// TODO: 实际设置 cgroup 限制
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.containers[name]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", name)
	}

	if limits.MemoryLimit > 0 {
		info.Config.MemoryLimit = limits.MemoryLimit
	}
	if limits.CPUQuota > 0 {
		info.Config.CPUShares = int(limits.CPUQuota)
	}

	return nil
}

// ConfigureNetwork 配置容器网络
func (m *Manager) ConfigureNetwork(ctx context.Context, name string, netCfg NetworkConfig) error {
	// TODO: 实际配置容器网络
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.containers[name]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", name)
	}

	info.Config.Network = netCfg
	return nil
}

// ListTemplates 列出可用模板
func (m *Manager) ListTemplates() []*TemplateInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TemplateInfo, 0, len(m.templates))
	for _, t := range m.templates {
		result = append(result, t)
	}
	return result
}

// RegisterTemplate 注册模板
func (m *Manager) RegisterTemplate(info TemplateInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.templates[info.Name]; exists {
		return fmt.Errorf("模板 %s 已存在", info.Name)
	}
	m.templates[info.Name] = &info
	return nil
}

// DeleteTemplate 删除模板
func (m *Manager) DeleteTemplate(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.templates[name]; !exists {
		return fmt.Errorf("模板 %s 不存在", name)
	}
	delete(m.templates, name)
	return nil
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("lxc-%d", time.Now().UnixNano())
}
