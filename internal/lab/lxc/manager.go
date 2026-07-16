package lxc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	Enabled       bool     `json:"enabled"`
	StoragePath   string   `json:"storagePath"`
	BridgeName    string   `json:"bridgeName"`
	SubnetCIDR    string   `json:"subnetCIDR"`
	MaxContainers int      `json:"maxContainers"`
	SnapshotPath  string   `json:"snapshotPath"`
	HAEnabled     bool     `json:"haEnabled"`
	HAPeerNodes   []string `json:"haPeerNodes"`
}

// DefaultManagerConfig 默认管理器配置.
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		Enabled:       true,
		StoragePath:   "/var/lib/nas-os/lxc",
		BridgeName:    "lxcbr0",
		SubnetCIDR:    "10.0.3.0/24",
		MaxContainers: 100,
		SnapshotPath:  "/var/lib/nas-os/lxc/snapshots",
		HAEnabled:     false,
	}
}

// Manager LXC 容器统一管理器.
type Manager struct {
	mu         sync.RWMutex
	config     *ManagerConfig
	containers map[string]*Container
	templates  *TemplateManager
	snapshots  map[string]*Snapshot // snapshotID -> snapshot
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewManager 创建 LXC 容器管理器.
func NewManager(cfg *ManagerConfig) *Manager {
	if cfg == nil {
		cfg = DefaultManagerConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:     cfg,
		containers: make(map[string]*Container),
		templates:  NewTemplateManager(),
		snapshots:  make(map[string]*Snapshot),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}

	// 创建存储目录
	if err := os.MkdirAll(m.config.StoragePath, 0755); err != nil {
		return fmt.Errorf("创建存储目录失败: %w", err)
	}
	if err := os.MkdirAll(m.config.SnapshotPath, 0755); err != nil {
		return fmt.Errorf("创建快照目录失败: %w", err)
	}

	// 检查 LXC 工具链
	if _, err := exec.LookPath("lxc-start"); err != nil {
		return fmt.Errorf("LXC 未安装: %w", err)
	}

	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() error {
	m.cancel()
	m.wg.Wait()
	return nil
}

// ========== 容器生命周期管理 ==========

// CreateContainer 创建容器.
func (m *Manager) CreateContainer(req CreateRequest) (*Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查容器数量限制
	if len(m.containers) >= m.config.MaxContainers {
		return nil, fmt.Errorf("已达到最大容器数 %d", m.config.MaxContainers)
	}

	// 验证模板
	if !m.templates.Exists(req.Template) {
		return nil, fmt.Errorf("模板 %s 不存在", req.Template)
	}

	// 检查重名
	for _, c := range m.containers {
		if c.Name == req.Name {
			return nil, fmt.Errorf("容器 %s 已存在", req.Name)
		}
	}

	// 获取模板默认资源
	tmplRes, _ := m.templates.GetDefaultResources(req.Template)

	// 合并资源配置
	res := req.Resources
	if res.CPUCores == 0 {
		res.CPUCores = tmplRes.CPUCores
	}
	if res.MemoryMB == 0 {
		res.MemoryMB = tmplRes.MemoryMB
	}
	if res.DiskGB == 0 {
		res.DiskGB = tmplRes.DiskGB
	}

	// 验证资源限制
	if err := res.Validate(); err != nil {
		return nil, fmt.Errorf("资源限制无效: %w", err)
	}

	// 设置默认网络
	net := req.Network
	if net.Mode == "" {
		net.Mode = NetworkBridge
	}

	now := time.Now()
	container := &Container{
		ID:        fmt.Sprintf("lxc-%s-%d", req.Name, now.UnixNano()),
		Name:      req.Name,
		Template:  req.Template,
		Status:    StatusCreated,
		Hostname:  req.Hostname,
		CreatedAt: now,
		UpdatedAt: now,
		Resources: res,
		Network:   net,
		Volumes:   req.Volumes,
		Tags:      req.Tags,
	}

	if container.Hostname == "" {
		container.Hostname = req.Name
	}

	// 创建容器目录
	containerDir := filepath.Join(m.config.StoragePath, container.ID)
	if err := os.MkdirAll(containerDir, 0755); err != nil {
		return nil, fmt.Errorf("创建容器目录失败: %w", err)
	}

	// 生成 LXC 配置文件
	if err := m.generateConfig(container); err != nil {
		os.RemoveAll(containerDir)
		return nil, fmt.Errorf("生成配置失败: %w", err)
	}

	m.containers[container.ID] = container
	return container, nil
}

// StartContainer 启动容器.
func (m *Manager) StartContainer(id string) error {
	m.mu.Lock()
	container, exists := m.containers[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("容器 %s 不存在", id)
	}

	if !ValidContainerTransition(container.Status, StatusStarting) {
		return fmt.Errorf("容器 %s 当前状态 %s 不允许启动", id, container.Status)
	}

	container.Status = StatusStarting
	container.UpdatedAt = time.Now()

	// 启动 LXC 容器
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lxc-start", "-n", container.ID, "-d")
	if err := cmd.Run(); err != nil {
		container.Status = StatusError
		container.Error = fmt.Sprintf("启动失败: %v", err)
		return fmt.Errorf("启动容器失败: %w", err)
	}

	now := time.Now()
	container.Status = StatusRunning
	container.StartedAt = &now
	container.UpdatedAt = now
	container.Error = ""

	// 启动健康检查
	if container.HealthCheck != nil && container.HealthCheck.Enabled {
		m.wg.Add(1)
		go m.healthCheckLoop(container)
	}

	return nil
}

// StopContainer 停止容器.
func (m *Manager) StopContainer(id string) error {
	m.mu.Lock()
	container, exists := m.containers[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("容器 %s 不存在", id)
	}

	if !ValidContainerTransition(container.Status, StatusStopping) {
		return fmt.Errorf("容器 %s 当前状态 %s 不允许停止", id, container.Status)
	}

	container.Status = StatusStopping
	container.UpdatedAt = time.Now()

	ctx, cancel := context.WithTimeout(m.ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lxc-stop", "-n", id)
	if err := cmd.Run(); err != nil {
		container.Status = StatusError
		container.Error = fmt.Sprintf("停止失败: %v", err)
		return fmt.Errorf("停止容器失败: %w", err)
	}

	now := time.Now()
	container.Status = StatusStopped
	container.StoppedAt = &now
	container.StartedAt = nil
	container.UpdatedAt = now
	container.Error = ""

	return nil
}

// RestartContainer 重启容器.
func (m *Manager) RestartContainer(id string) error {
	m.mu.RLock()
	container, exists := m.containers[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("容器 %s 不存在", id)
	}

	if container.Status != StatusRunning {
		return fmt.Errorf("容器 %s 未运行，无法重启", id)
	}

	if err := m.StopContainer(id); err != nil {
		return err
	}

	// 短暂等待
	time.Sleep(1 * time.Second)

	if err := m.StartContainer(id); err != nil {
		return err
	}

	m.mu.Lock()
	container.RestartCount++
	m.mu.Unlock()

	return nil
}

// DeleteContainer 删除容器.
func (m *Manager) DeleteContainer(id string) error {
	m.mu.Lock()
	container, exists := m.containers[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("容器 %s 不存在", id)
	}

	// 运行中的容器需要先停止
	if container.Status == StatusRunning || container.Status == StatusPaused {
		m.mu.Unlock()
		if err := m.StopContainer(id); err != nil {
			return err
		}
		m.mu.Lock()
	}

	// 删除所有快照
	for _, snap := range container.Snapshots {
		snapDir := filepath.Join(m.config.SnapshotPath, snap.ID)
		os.RemoveAll(snapDir)
		delete(m.snapshots, snap.ID)
	}

	// 删除容器目录
	containerDir := filepath.Join(m.config.StoragePath, id)
	if err := os.RemoveAll(containerDir); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("删除容器目录失败: %w", err)
	}

	delete(m.containers, id)
	m.mu.Unlock()

	return nil
}

// GetContainer 获取容器.
func (m *Manager) GetContainer(id string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, exists := m.containers[id]
	if !exists {
		return nil, fmt.Errorf("容器 %s 不存在", id)
	}
	return container, nil
}

// ListContainers 列出所有容器.
func (m *Manager) ListContainers() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		result = append(result, c)
	}
	return result
}

// ListByStatus 按状态列出容器.
func (m *Manager) ListByStatus(status ContainerStatus) []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Container
	for _, c := range m.containers {
		if c.Status == status {
			result = append(result, c)
		}
	}
	return result
}

// GetStats 获取容器资源统计.
func (m *Manager) GetStats(id string) (*Stats, error) {
	m.mu.RLock()
	container, exists := m.containers[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("容器 %s 不存在", id)
	}
	if container.Status != StatusRunning {
		return nil, fmt.Errorf("容器 %s 未运行", id)
	}

	stats := &Stats{
		MemoryLimit: container.Resources.MemoryMB,
		DiskLimitMB: container.Resources.DiskGB * 1024,
		Timestamp:   time.Now(),
	}

	// 读取 cgroup 统计
	cgroupPath := filepath.Join("/sys/fs/cgroup/lxc", id)
	if data, err := os.ReadFile(filepath.Join(cgroupPath, "memory.current")); err == nil {
		var memBytes int64
		fmt.Sscanf(string(data), "%d", &memBytes)
		stats.MemoryMB = uint64(memBytes / 1024 / 1024)
	}

	return stats, nil
}

// UpdateResources 更新容器资源限制.
func (m *Manager) UpdateResources(id string, res ResourceLimit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", id)
	}

	if err := res.Validate(); err != nil {
		return fmt.Errorf("资源限制无效: %w", err)
	}

	container.Resources = res
	container.UpdatedAt = time.Now()

	// 如果容器运行中，更新 cgroup 配置
	if container.Status == StatusRunning {
		m.applyCgroupLimits(container)
	}

	return nil
}

// ========== 资源限制 ==========

// applyCgroupLimits 应用 cgroup 资源限制.
func (m *Manager) applyCgroupLimits(container *Container) error {
	cgroupPath := filepath.Join("/sys/fs/cgroup/lxc", container.ID)

	// 设置 CPU 限制
	if container.Resources.CPUPercent > 0 {
		quota := container.Resources.CPUPercent * 1000
		period := 100000
		cpuMax := fmt.Sprintf("%d %d", quota, period)
		os.WriteFile(filepath.Join(cgroupPath, "cpu.max"), []byte(cpuMax), 0644)
	}

	// 设置内存限制
	if container.Resources.MemoryMB > 0 {
		memBytes := container.Resources.MemoryMB * 1024 * 1024
		os.WriteFile(filepath.Join(cgroupPath, "memory.max"), []byte(fmt.Sprintf("%d", memBytes)), 0644)
	}

	// 设置进程数限制
	if container.Resources.ProcessMax > 0 {
		os.WriteFile(filepath.Join(cgroupPath, "pids.max"), []byte(fmt.Sprintf("%d", container.Resources.ProcessMax)), 0644)
	}

	return nil
}

// generateConfig 生成 LXC 配置文件.
func (m *Manager) generateConfig(container *Container) error {
	configPath := filepath.Join(m.config.StoragePath, container.ID, "config")

	config := fmt.Sprintf(`# NAS-OS LXC Container Config
# Generated: %s

# 基本配置
lxc.uts.name = %s
lxc.arch = amd64
lxc.rootfs.path = dir:%s/rootfs

# 容器行为
lxc.start.auto = 0
lxc.start.delay = 0
lxc.start.order = 0
lxc.tty.max = 4
lxc.pty.max = 1024

# 网络配置
lxc.net.0.type = veth
lxc.net.0.link = %s
lxc.net.0.flags = up
lxc.net.0.name = eth0

# 资源限制 (cgroup)
lxc.cgroup2.cpu.max = %d00000 100000
lxc.cgroup2.memory.max = %d
lxc.cgroup2.memory.swap.max = %d
`,
		time.Now().Format(time.RFC3339),
		container.Hostname,
		filepath.Join(m.config.StoragePath, container.ID),
		m.config.BridgeName,
		container.Resources.CPUCores,
		container.Resources.MemoryMB*1024*1024,
		container.Resources.MemorySwapMB*1024*1024,
	)

	// 添加进程数限制
	if container.Resources.ProcessMax > 0 {
		config += fmt.Sprintf("lxc.cgroup2.pids.max = %d\n", container.Resources.ProcessMax)
	}

	// 添加端口映射说明（通过 iptables 在运行时设置）
	if len(container.Network.Ports) > 0 {
		config += "\n# 端口映射\n"
		for _, port := range container.Network.Ports {
			config += fmt.Sprintf("# %s:%d -> %d/%s\n", m.config.BridgeName, port.HostPort, port.ContainerPort, port.Protocol)
		}
	}

	// 添加卷挂载
	for i, vol := range container.Volumes {
		config += fmt.Sprintf("\nlxc.mount.%d = %s %s none bind,%s 0 0\n",
			i, vol.Source, vol.Destination, map[bool]string{true: "ro", false: "rw"}[vol.ReadOnly])
	}

	return os.WriteFile(configPath, []byte(config), 0644)
}

// ========== 快照备份 ==========

// CreateSnapshot 创建容器快照.
func (m *Manager) CreateSnapshot(containerID, name, description string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}

	snapID := fmt.Sprintf("snap-%s-%d", containerID, time.Now().UnixNano())

	snapshot := &Snapshot{
		ID:          snapID,
		ContainerID: containerID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}

	// 创建快照目录
	snapDir := filepath.Join(m.config.SnapshotPath, snapID)
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return nil, fmt.Errorf("创建快照目录失败: %w", err)
	}

	// 复制容器数据到快照目录（实际环境中使用 lxc-snapshot 或 ZFS 快照）
	containerDir := filepath.Join(m.config.StoragePath, containerID)
	cmd := exec.Command("cp", "-a", containerDir, filepath.Join(snapDir, "data"))
	if err := cmd.Run(); err != nil {
		os.RemoveAll(snapDir)
		return nil, fmt.Errorf("创建快照失败: %w", err)
	}

	// 计算快照大小
	var sizeMB uint64
	filepath.Walk(snapDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			sizeMB += uint64(info.Size()) / 1024 / 1024
		}
		return nil
	})
	snapshot.SizeMB = sizeMB

	container.Snapshots = append(container.Snapshots, *snapshot)
	container.UpdatedAt = time.Now()

	m.snapshots[snapID] = snapshot

	return snapshot, nil
}

// RestoreSnapshot 从快照恢复容器.
func (m *Manager) RestoreSnapshot(snapshotID string) error {
	m.mu.Lock()
	snapshot, exists := m.snapshots[snapshotID]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("快照 %s 不存在", snapshotID)
	}

	m.mu.RLock()
	container, exists := m.containers[snapshot.ContainerID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("容器 %s 不存在", snapshot.ContainerID)
	}

	// 如果容器运行中，先停止
	if container.Status == StatusRunning {
		if err := m.StopContainer(container.ID); err != nil {
			return fmt.Errorf("停止容器失败: %w", err)
		}
	}

	// 从快照恢复数据
	snapDataDir := filepath.Join(m.config.SnapshotPath, snapshotID, "data")
	containerDir := filepath.Join(m.config.StoragePath, container.ID)

	// 备份当前数据
	backupDir := containerDir + ".bak"
	os.Rename(containerDir, backupDir)

	// 恢复快照数据
	cmd := exec.Command("cp", "-a", snapDataDir, containerDir)
	if err := cmd.Run(); err != nil {
		// 恢复失败，还原备份
		os.RemoveAll(containerDir)
		os.Rename(backupDir, containerDir)
		return fmt.Errorf("恢复快照失败: %w", err)
	}

	// 清理备份
	os.RemoveAll(backupDir)

	m.mu.Lock()
	container.Status = StatusStopped
	container.UpdatedAt = time.Now()
	m.mu.Unlock()

	return nil
}

// DeleteSnapshot 删除快照.
func (m *Manager) DeleteSnapshot(snapshotID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot, exists := m.snapshots[snapshotID]
	if !exists {
		return fmt.Errorf("快照 %s 不存在", snapshotID)
	}

	// 从容器快照列表中移除
	container, exists := m.containers[snapshot.ContainerID]
	if exists {
		for i, snap := range container.Snapshots {
			if snap.ID == snapshotID {
				container.Snapshots = append(container.Snapshots[:i], container.Snapshots[i+1:]...)
				break
			}
		}
		container.UpdatedAt = time.Now()
	}

	// 删除快照目录
	snapDir := filepath.Join(m.config.SnapshotPath, snapshotID)
	if err := os.RemoveAll(snapDir); err != nil {
		return fmt.Errorf("删除快照目录失败: %w", err)
	}

	delete(m.snapshots, snapshotID)
	return nil
}

// ListSnapshots 列出容器的所有快照.
func (m *Manager) ListSnapshots(containerID string) ([]Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, exists := m.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}

	return container.Snapshots, nil
}

// ========== 网络隔离 ==========

// CreateIsolatedNetwork 创建隔离网络命名空间.
func (m *Manager) CreateIsolatedNetwork(containerID string) error {
	m.mu.RLock()
	container, exists := m.containers[containerID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	container.Network.Isolated = true
	container.Network.Mode = NetworkIsolated

	// 创建 veth pair 并配置 iptables 规则
	// 实际环境中需要调用 ip netns 和 iptables
	return nil
}

// AddPortMapping 添加端口映射.
func (m *Manager) AddPortMapping(containerID string, port PortMap) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	// 检查端口冲突
	for _, p := range container.Network.Ports {
		if p.HostPort == port.HostPort && p.Protocol == port.Protocol {
			return fmt.Errorf("端口 %d/%s 已被映射", port.HostPort, port.Protocol)
		}
	}

	container.Network.Ports = append(container.Network.Ports, port)
	container.UpdatedAt = time.Now()

	// 如果容器运行中，动态添加 iptables 规则
	if container.Status == StatusRunning {
		return m.applyPortMapping(container, port)
	}

	return nil
}

// RemovePortMapping 移除端口映射.
func (m *Manager) RemovePortMapping(containerID string, hostPort int, protocol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	for i, port := range container.Network.Ports {
		if port.HostPort == hostPort && port.Protocol == protocol {
			container.Network.Ports = append(container.Network.Ports[:i], container.Network.Ports[i+1:]...)
			container.UpdatedAt = time.Now()

			// 如果容器运行中，动态移除 iptables 规则
			if container.Status == StatusRunning {
				m.removePortMapping(container, port)
			}
			return nil
		}
	}

	return fmt.Errorf("端口映射 %d/%s 不存在", hostPort, protocol)
}

func (m *Manager) applyPortMapping(container *Container, port PortMap) error {
	// 实际环境中使用 iptables 添加 DNAT 规则
	// iptables -t nat -A PREROUTING -p tcp --dport <hostPort> -j DNAT --to-destination <containerIP>:<containerPort>
	return nil
}

func (m *Manager) removePortMapping(container *Container, port PortMap) error {
	// 实际环境中使用 iptables 移除 DNAT 规则
	return nil
}

// ========== 存储卷管理 ==========

// AddVolume 添加存储卷.
func (m *Manager) AddVolume(containerID string, vol VolumeMount) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	if vol.Source == "" || vol.Destination == "" {
		return fmt.Errorf("源路径和目标路径不能为空")
	}

	// 检查是否已有相同挂载点
	for _, v := range container.Volumes {
		if v.Destination == vol.Destination {
			return fmt.Errorf("挂载点 %s 已存在", vol.Destination)
		}
	}

	container.Volumes = append(container.Volumes, vol)
	container.UpdatedAt = time.Now()

	return nil
}

// RemoveVolume 移除存储卷.
func (m *Manager) RemoveVolume(containerID, destination string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	for i, vol := range container.Volumes {
		if vol.Destination == destination {
			container.Volumes = append(container.Volumes[:i], container.Volumes[i+1:]...)
			container.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("挂载点 %s 不存在", destination)
}

// ========== 健康检查 ==========

// healthCheckLoop 健康检查循环.
func (m *Manager) healthCheckLoop(container *Container) {
	defer m.wg.Done()

	hc := container.HealthCheck
	if hc.Interval == 0 {
		hc.Interval = 30 * time.Second
	}
	if hc.Timeout == 0 {
		hc.Timeout = 10 * time.Second
	}
	if hc.Retries == 0 {
		hc.Retries = 3
	}

	// 等待启动期
	if hc.StartPeriod > 0 {
		select {
		case <-time.After(hc.StartPeriod):
		case <-m.ctx.Done():
			return
		}
	}

	ticker := time.NewTicker(hc.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.mu.RLock()
			status := container.Status
			m.mu.RUnlock()

			if status != StatusRunning {
				return
			}

			healthy := m.runHealthCheck(container)

			m.mu.Lock()
			now := time.Now()
			hc.LastCheck = &now
			hc.LastResult = healthy

			if !healthy {
				hc.UnhealthyCount++
				if hc.UnhealthyCount >= hc.Retries {
					// 标记为不健康
					container.Status = StatusError
					container.Error = "健康检查失败"
					container.UpdatedAt = now

					// 高可用故障转移
					if container.HAConfig != nil && container.HAConfig.Enabled {
						m.handleFailover(container)
					}
				}
			} else {
				hc.UnhealthyCount = 0
			}
			m.mu.Unlock()

		case <-m.ctx.Done():
			return
		}
	}
}

// runHealthCheck 执行健康检查.
func (m *Manager) runHealthCheck(container *Container) bool {
	hc := container.HealthCheck
	if hc == nil || hc.Command == "" {
		return true
	}

	ctx, cancel := context.WithTimeout(m.ctx, hc.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lxc-execute", "-n", container.ID, "--", "/bin/sh", "-c", hc.Command)
	return cmd.Run() == nil
}

// ========== 高可用支持 ==========

// handleFailover 处理故障转移.
func (m *Manager) handleFailover(container *Container) {
	ha := container.HAConfig
	if ha == nil || !ha.Enabled {
		return
	}

	ha.CurrentRestarts++

	// 检查是否超过最大重启次数
	if ha.MaxRestarts > 0 && ha.CurrentRestarts > ha.MaxRestarts {
		container.Error = fmt.Sprintf("超过最大重启次数 %d", ha.MaxRestarts)
		return
	}

	// 延迟重启
	if ha.RestartDelay > 0 {
		time.AfterFunc(ha.RestartDelay, func() {
			m.StartContainer(container.ID)
		})
	} else {
		m.StartContainer(container.ID)
	}

	now := time.Now()
	ha.LastFailover = &now
}

// EnableHA 为容器启用高可用.
func (m *Manager) EnableHA(containerID string, haCfg HAConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	haCfg.Enabled = true
	container.HAConfig = &haCfg
	container.UpdatedAt = time.Now()

	return nil
}

// DisableHA 为容器禁用高可用.
func (m *Manager) DisableHA(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	container.HAConfig = nil
	container.UpdatedAt = time.Now()

	return nil
}

// ========== 批量操作 ==========

// BatchStart 批量启动容器.
func (m *Manager) BatchStart(ids []string) map[string]error {
	result := make(map[string]error)
	for _, id := range ids {
		result[id] = m.StartContainer(id)
	}
	return result
}

// BatchStop 批量停止容器.
func (m *Manager) BatchStop(ids []string) map[string]error {
	result := make(map[string]error)
	for _, id := range ids {
		result[id] = m.StopContainer(id)
	}
	return result
}

// BatchDelete 批量删除容器.
func (m *Manager) BatchDelete(ids []string) map[string]error {
	result := make(map[string]error)
	for _, id := range ids {
		result[id] = m.DeleteContainer(id)
	}
	return result
}

// ========== 系统信息 ==========

// ContainerCount 容器数量.
func (m *Manager) ContainerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.containers)
}

// TemplateManager 获取模板管理器.
func (m *Manager) TemplateManager() *TemplateManager {
	return m.templates
}

// StatusSummary 系统状态摘要.
type StatusSummary struct {
	TotalContainers int  `json:"totalContainers"`
	Running         int  `json:"running"`
	Stopped         int  `json:"stopped"`
	Error           int  `json:"error"`
	TotalTemplates  int  `json:"totalTemplates"`
	TotalSnapshots  int  `json:"totalSnapshots"`
	MaxContainers   int  `json:"maxContainers"`
	HAEnabled       bool `json:"haEnabled"`
}

// GetStatusSummary 获取系统状态摘要.
func (m *Manager) GetStatusSummary() StatusSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := StatusSummary{
		TotalContainers: len(m.containers),
		TotalTemplates:  m.templates.Count(),
		TotalSnapshots:  len(m.snapshots),
		MaxContainers:   m.config.MaxContainers,
		HAEnabled:       m.config.HAEnabled,
	}

	for _, c := range m.containers {
		switch c.Status {
		case StatusRunning:
			summary.Running++
		case StatusStopped, StatusCreated:
			summary.Stopped++
		case StatusError:
			summary.Error++
		}
	}

	return summary
}
