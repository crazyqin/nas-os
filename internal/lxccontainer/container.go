package lxccontainer

import (
	"fmt"
	"time"
)

// ContainerManager 容器生命周期管理.
type ContainerManager struct {
	containers map[string]*Container
}

// NewContainerManager 创建容器管理器.
func NewContainerManager() *ContainerManager {
	return &ContainerManager{
		containers: make(map[string]*Container),
	}
}

// Manager 统一管理器，整合容器、网络、模板管理.
type Manager struct {
	containers *ContainerManager
	network    *NetworkManager
	templates  *TemplateManager
}

// NewManager 创建统一管理器.
func NewManager() *Manager {
	return &Manager{
		containers: NewContainerManager(),
		network:    NewNetworkManager(),
		templates:  NewTemplateManager(),
	}
}

// CreateBridge 创建网桥.
func (m *Manager) CreateBridge(name, subnet, gateway string) (*Bridge, error) {
	return m.network.CreateBridge(name, subnet, gateway)
}

// AllocateIP 分配 IP 地址.
func (m *Manager) AllocateIP(bridgeName string) (string, error) {
	return m.network.AllocateIP(bridgeName)
}

// ReleaseIP 释放 IP 地址.
func (m *Manager) ReleaseIP(bridgeName, ip string) error {
	return m.network.ReleaseIP(bridgeName, ip)
}

// DeleteBridge 删除网桥.
func (m *Manager) DeleteBridge(name string) error {
	return m.network.DeleteBridge(name)
}

// CreateContainer 创建容器.
func (m *Manager) CreateContainer(req CreateRequest) (*Container, error) {
	// 验证模板存在
	if !m.templates.Exists(req.Template) {
		return nil, fmt.Errorf("模板 %s 不存在", req.Template)
	}

	// 验证网络配置
	if req.Network.Mode == NetworkModeStatic && req.Network.IPAddress == "" {
		return nil, fmt.Errorf("静态网络模式需要指定 IP 地址")
	}

	return m.containers.Create(req)
}

// StartContainer 启动容器.
func (m *Manager) StartContainer(id string) error {
	return m.containers.Start(id)
}

// StopContainer 停止容器.
func (m *Manager) StopContainer(id string) error {
	return m.containers.Stop(id)
}

// DeleteContainer 删除容器.
func (m *Manager) DeleteContainer(id string) error {
	return m.containers.Delete(id)
}

// GetStats 获取容器统计.
func (m *Manager) GetStats(id string) (*Stats, error) {
	return m.containers.GetStats(id)
}

// ContainerCount 返回容器数量.
func (m *Manager) ContainerCount() int {
	return m.containers.Count()
}

// ListTemplates 列出所有模板.
func (m *Manager) ListTemplates() []*Template {
	return m.templates.List()
}

// RegisterTemplate 注册模板.
func (m *Manager) RegisterTemplate(t *Template) error {
	return m.templates.Register(t)
}

// DeleteTemplate 删除模板.
func (m *Manager) DeleteTemplate(name string) error {
	return m.templates.Delete(name)
}

// Create 创建容器 (不启动).
func (cm *ContainerManager) Create(req CreateRequest) (*Container, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("容器名称不能为空")
	}
	if req.Template == "" {
		return nil, fmt.Errorf("模板名称不能为空")
	}

	// 检查重名
	for _, c := range cm.containers {
		if c.Name == req.Name {
			return nil, fmt.Errorf("容器 %s 已存在", req.Name)
		}
	}

	now := time.Now()
	c := &Container{
		ID:        fmt.Sprintf("lxc-%d", now.UnixNano()),
		Name:      req.Name,
		Template:  req.Template,
		Status:    StatusStopped,
		CreatedAt: now,
		UpdatedAt: now,
		Resources: req.Resources,
		Network:   req.Network,
		Volumes:   req.Volumes,
		Hostname:  req.Hostname,
		Tags:      req.Tags,
	}

	if c.Hostname == "" {
		c.Hostname = req.Name
	}
	if c.Resources.CPUCores == 0 {
		c.Resources.CPUCores = 1
	}
	if c.Resources.MemoryMB == 0 {
		c.Resources.MemoryMB = 512
	}
	if c.Resources.DiskGB == 0 {
		c.Resources.DiskGB = 10
	}

	cm.containers[c.ID] = c
	return c, nil
}

// Start 启动容器.
func (cm *ContainerManager) Start(id string) error {
	c, ok := cm.containers[id]
	if !ok {
		return fmt.Errorf("容器 %s 不存在", id)
	}
	if !ValidTransition(c.Status, StatusRunning) {
		return fmt.Errorf("容器 %s 当前状态 %s 不允许启动", id, c.Status)
	}

	c.Status = StatusRunning
	now := time.Now()
	c.StartedAt = &now
	c.UpdatedAt = now
	return nil
}

// Stop 停止容器.
func (cm *ContainerManager) Stop(id string) error {
	c, ok := cm.containers[id]
	if !ok {
		return fmt.Errorf("容器 %s 不存在", id)
	}
	if !ValidTransition(c.Status, StatusStopped) {
		return fmt.Errorf("容器 %s 当前状态 %s 不允许停止", id, c.Status)
	}

	c.Status = StatusStopped
	c.UpdatedAt = time.Now()
	return nil
}

// Restart 重启容器.
func (cm *ContainerManager) Restart(id string) error {
	c, ok := cm.containers[id]
	if !ok {
		return fmt.Errorf("容器 %s 不存在", id)
	}
	if c.Status != StatusRunning {
		return fmt.Errorf("容器 %s 未运行，无法重启", id)
	}

	c.Status = StatusRebooting
	c.UpdatedAt = time.Now()

	// 模拟重启：直接转为运行
	c.Status = StatusRunning
	now := time.Now()
	c.StartedAt = &now
	c.UpdatedAt = now
	return nil
}

// Delete 删除容器.
func (cm *ContainerManager) Delete(id string) error {
	c, ok := cm.containers[id]
	if !ok {
		return fmt.Errorf("容器 %s 不存在", id)
	}
	if !ValidTransition(c.Status, StatusDeleting) {
		return fmt.Errorf("容器 %s 当前状态 %s 不允许删除", id, c.Status)
	}

	c.Status = StatusDeleting
	delete(cm.containers, id)
	return nil
}

// Get 获取容器.
func (cm *ContainerManager) Get(id string) (*Container, error) {
	c, ok := cm.containers[id]
	if !ok {
		return nil, fmt.Errorf("容器 %s 不存在", id)
	}
	return c, nil
}

// List 列出所有容器.
func (cm *ContainerManager) List() []*Container {
	result := make([]*Container, 0, len(cm.containers))
	for _, c := range cm.containers {
		result = append(result, c)
	}
	return result
}

// ListByStatus 按状态过滤容器.
func (cm *ContainerManager) ListByStatus(status Status) []*Container {
	var result []*Container
	for _, c := range cm.containers {
		if c.Status == status {
			result = append(result, c)
		}
	}
	return result
}

// Count 返回容器数量.
func (cm *ContainerManager) Count() int {
	return len(cm.containers)
}

// GetStats 获取容器资源统计.
func (cm *ContainerManager) GetStats(id string) (*Stats, error) {
	c, ok := cm.containers[id]
	if !ok {
		return nil, fmt.Errorf("容器 %s 不存在", id)
	}
	if c.Status != StatusRunning {
		return nil, fmt.Errorf("容器 %s 未运行", id)
	}

	// 模拟统计
	return &Stats{
		CPUPercent:  12.5,
		MemoryMB:    c.Resources.MemoryMB / 2,
		MemoryLimit: c.Resources.MemoryMB,
		DiskUsedMB:  c.Resources.DiskGB * 512,
		NetRxBytes:  1024 * 100,
		NetTxBytes:  1024 * 50,
		PIDs:        25,
	}, nil
}

// UpdateResources 更新容器资源限制.
func (cm *ContainerManager) UpdateResources(id string, res ResourceLimit) error {
	c, ok := cm.containers[id]
	if !ok {
		return fmt.Errorf("容器 %s 不存在", id)
	}
	if res.CPUCores < 0 || res.MemoryMB == 0 {
		return fmt.Errorf("无效的资源配置")
	}
	c.Resources = res
	c.UpdatedAt = time.Now()
	return nil
}

// validateResources 验证资源限制配置.
func validateResources(res ResourceLimit) error {
	if res.MemoryMB == 0 {
		return fmt.Errorf("内存不能为0")
	}
	if res.CPUCores < 0 {
		return fmt.Errorf("CPU核心数不能为负数")
	}
	if res.CPUPercent < 0 || res.CPUPercent > 100 {
		return fmt.Errorf("CPU百分比必须在0-100之间")
	}
	if res.ProcessMax < 0 {
		return fmt.Errorf("最大进程数不能为负数")
	}
	return nil
}

// AddVolume 添加存储卷挂载.
func (cm *ContainerManager) AddVolume(id string, vol VolumeMount) error {
	c, ok := cm.containers[id]
	if !ok {
		return fmt.Errorf("容器 %s 不存在", id)
	}
	if vol.Source == "" || vol.Destination == "" {
		return fmt.Errorf("源路径和目标路径不能为空")
	}
	c.Volumes = append(c.Volumes, vol)
	c.UpdatedAt = time.Now()
	return nil
}
