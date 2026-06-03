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
