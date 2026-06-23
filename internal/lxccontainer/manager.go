package lxccontainer

import (
	"fmt"
)

// Manager 统一管理器，整合容器、网络、模板、快照管理.
type Manager struct {
	containers *ContainerManager
	network    *NetworkManager
	templates  *TemplateManager
	snapshots  *SnapshotManager
}

// NewManager 创建统一管理器.
func NewManager() *Manager {
	return &Manager{
		containers: NewContainerManager(),
		network:    NewNetworkManager(),
		templates:  NewTemplateManager(),
		snapshots:  NewSnapshotManager(),
	}
}

// validateResources 验证资源配置合法性.
func validateResources(res ResourceLimit) error {
	if res.MemoryMB == 0 {
		return fmt.Errorf("内存不能为0")
	}
	if res.CPUCores < 0 {
		return fmt.Errorf("CPU核心数不能为负")
	}
	if res.CPUPercent < 0 || res.CPUPercent > 100 {
		return fmt.Errorf("CPU百分比必须在0-100之间")
	}
	if res.ProcessMax < 0 {
		return fmt.Errorf("最大进程数不能为负")
	}
	return nil
}

// ===== 容器操作 =====

// CreateContainer 创建容器.
func (m *Manager) CreateContainer(req CreateRequest) (*Container, error) {
	// 验证资源
	if err := validateResources(req.Resources); err != nil {
		return nil, fmt.Errorf("资源验证失败: %w", err)
	}

	// 验证模板
	if !m.templates.Exists(req.Template) {
		return nil, fmt.Errorf("模板 %s 不存在", req.Template)
	}

	// 验证网络配置
	if req.Network.Mode == NetworkModeStatic {
		if req.Network.IPAddress == "" {
			return nil, fmt.Errorf("静态IP模式需要指定IP地址")
		}
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

// GetContainer 获取容器.
func (m *Manager) GetContainer(id string) (*Container, error) {
	return m.containers.Get(id)
}

// ListContainers 列出所有容器.
func (m *Manager) ListContainers() []*Container {
	return m.containers.List()
}

// ContainerCount 返回容器数量.
func (m *Manager) ContainerCount() int {
	return m.containers.Count()
}

// GetStats 获取容器统计.
func (m *Manager) GetStats(id string) (*Stats, error) {
	return m.containers.GetStats(id)
}

// ===== 网络操作 =====

// CreateBridge 创建网桥.
func (m *Manager) CreateBridge(name, subnet, gateway string) (*Bridge, error) {
	return m.network.CreateBridge(name, subnet, gateway)
}

// DeleteBridge 删除网桥.
func (m *Manager) DeleteBridge(name string) error {
	return m.network.DeleteBridge(name)
}

// AllocateIP 分配IP.
func (m *Manager) AllocateIP(bridgeName string) (string, error) {
	return m.network.AllocateIP(bridgeName)
}

// ReleaseIP 释放IP.
func (m *Manager) ReleaseIP(bridgeName, ip string) error {
	return m.network.ReleaseIP(bridgeName, ip)
}

// ListBridges 列出网桥.
func (m *Manager) ListBridges() []*Bridge {
	return m.network.ListBridges()
}

// ===== 模板操作 =====

// ListTemplates 列出模板.
func (m *Manager) ListTemplates() []*Template {
	return m.templates.List()
}

// GetTemplate 获取模板.
func (m *Manager) GetTemplate(name string) (*Template, error) {
	return m.templates.Get(name)
}

// RegisterTemplate 注册模板.
func (m *Manager) RegisterTemplate(t *Template) error {
	return m.templates.Register(t)
}

// DeleteTemplate 删除模板.
func (m *Manager) DeleteTemplate(name string) error {
	return m.templates.Delete(name)
}

// ===== 快照操作 =====

// CreateSnapshot 创建容器快照.
func (m *Manager) CreateSnapshot(req SnapshotCreateRequest) (*Snapshot, error) {
	c, err := m.containers.Get(req.ContainerID)
	if err != nil {
		return nil, err
	}
	return m.snapshots.CreateSnapshot(req.ContainerID, req.Name, c)
}

// RestoreSnapshot 恢复快照.
func (m *Manager) RestoreSnapshot(req SnapshotRestoreRequest) error {
	c, err := m.containers.Get(req.ContainerID)
	if err != nil {
		return err
	}
	return m.snapshots.RestoreSnapshot(req.SnapshotID, c)
}

// DeleteSnapshot 删除快照.
func (m *Manager) DeleteSnapshot(snapshotID string) error {
	return m.snapshots.DeleteSnapshot(snapshotID)
}

// ListSnapshots 列出容器快照.
func (m *Manager) ListSnapshots(containerID string) []*Snapshot {
	return m.snapshots.ListSnapshots(containerID)
}

// GetSnapshot 获取快照.
func (m *Manager) GetSnapshot(snapshotID string) (*Snapshot, error) {
	return m.snapshots.GetSnapshot(snapshotID)
}
