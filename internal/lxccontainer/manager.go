package lxccontainer

import (
	"fmt"
	"sync"
)

// Manager LXC 容器统一管理器.
type Manager struct {
	mu        sync.RWMutex
	container *ContainerManager
	network   *NetworkManager
	template  *TemplateManager
}

// NewManager 创建 LXC 容器管理器.
func NewManager() *Manager {
	return &Manager{
		container: NewContainerManager(),
		network:   NewNetworkManager(),
		template:  NewTemplateManager(),
	}
}

// CreateContainer 创建容器.
func (m *Manager) CreateContainer(req CreateRequest) (*Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证模板
	if !m.template.Exists(req.Template) {
		return nil, fmt.Errorf("模板 %s 不存在", req.Template)
	}

	// 验证网络配置
	if req.Network.Mode != NetworkModeNone {
		if err := ValidateNetworkConfig(req.Network); err != nil {
			return nil, fmt.Errorf("网络配置无效: %w", err)
		}
	}

	// 验证资源限制
	if err := validateResources(req.Resources); err != nil {
		return nil, fmt.Errorf("资源限制无效: %w", err)
	}

	return m.container.Create(req)
}

// StartContainer 启动容器.
func (m *Manager) StartContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.container.Start(id)
}

// StopContainer 停止容器.
func (m *Manager) StopContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.container.Stop(id)
}

// RestartContainer 重启容器.
func (m *Manager) RestartContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.container.Restart(id)
}

// DeleteContainer 删除容器.
func (m *Manager) DeleteContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.container.Delete(id)
}

// GetContainer 获取容器.
func (m *Manager) GetContainer(id string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.container.Get(id)
}

// ListContainers 列出所有容器.
func (m *Manager) ListContainers() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.container.List()
}

// ListByStatus 按状态过滤.
func (m *Manager) ListByStatus(status Status) []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.container.ListByStatus(status)
}

// GetStats 获取容器统计.
func (m *Manager) GetStats(id string) (*Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.container.GetStats(id)
}

// UpdateResources 更新资源限制.
func (m *Manager) UpdateResources(id string, res ResourceLimit) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.container.UpdateResources(id, res)
}

// AddVolume 添加卷挂载.
func (m *Manager) AddVolume(id string, vol VolumeMount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.container.AddVolume(id, vol)
}

// CreateBridge 创建网桥.
func (m *Manager) CreateBridge(name, subnet, gateway string) (*Bridge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.network.CreateBridge(name, subnet, gateway)
}

// DeleteBridge 删除网桥.
func (m *Manager) DeleteBridge(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.network.DeleteBridge(name)
}

// ListBridges 列出网桥.
func (m *Manager) ListBridges() []*Bridge {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.network.ListBridges()
}

// AllocateIP 分配 IP.
func (m *Manager) AllocateIP(bridgeName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.network.AllocateIP(bridgeName)
}

// ReleaseIP 释放 IP.
func (m *Manager) ReleaseIP(bridgeName, ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.network.ReleaseIP(bridgeName, ip)
}

// RegisterTemplate 注册模板.
func (m *Manager) RegisterTemplate(t *Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.template.Register(t)
}

// ListTemplates 列出模板.
func (m *Manager) ListTemplates() []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.template.List()
}

// GetTemplate 获取模板.
func (m *Manager) GetTemplate(name string) (*Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.template.Get(name)
}

// DeleteTemplate 删除模板.
func (m *Manager) DeleteTemplate(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.template.Delete(name)
}

// TemplateCount 模板数量.
func (m *Manager) TemplateCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.template.Count()
}

// ContainerCount 容器数量.
func (m *Manager) ContainerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.container.Count()
}

// validateResources 验证资源限制.
func validateResources(res ResourceLimit) error {
	if res.CPUCores < 0 {
		return fmt.Errorf("CPU 核心数不能为负")
	}
	if res.CPUPercent < 0 || res.CPUPercent > 100 {
		return fmt.Errorf("CPU 百分比必须在 0-100 之间")
	}
	if res.MemoryMB == 0 {
		return fmt.Errorf("内存限制不能为 0")
	}
	if res.ProcessMax < 0 {
		return fmt.Errorf("最大进程数不能为负")
	}
	return nil
}
