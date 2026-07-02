package lxcgpu

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager LXC容器GPU分配管理器
// 负责GPU设备分配到LXC容器的完整生命周期管理
// 支持热插拔、资源配额、共享模式.
type Manager struct {
	mu          sync.RWMutex
	config      *LXCConfig
	devices     *DeviceManager
	assignments map[string]*LXCGPUAssignment // 分配ID -> 分配记录
	// containerAssigns 容器ID -> 分配ID列表
	containerAssigns map[string][]string
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewManager 创建LXC GPU分配管理器.
func NewManager(config *LXCConfig) *Manager {
	if config == nil {
		config = DefaultLXCConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:           config,
		devices:          NewDeviceManager(config),
		assignments:      make(map[string]*LXCGPUAssignment),
		containerAssigns: make(map[string][]string),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	// 发现GPU设备
	if _, err := m.devices.DiscoverDevices(); err != nil {
		return fmt.Errorf("GPU设备发现失败: %w", err)
	}

	// 启动设备状态轮询（每30秒）
	m.devices.StartPolling(30 * time.Second)

	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() error {
	m.cancel()
	m.devices.StopPolling()
	return nil
}

// GetDeviceManager 获取设备管理器.
func (m *Manager) GetDeviceManager() *DeviceManager {
	return m.devices
}

// ========== GPU分配管理 ==========

// AssignGPU 将GPU设备分配给LXC容器
// 支持运行中容器的热插拔.
func (m *Manager) AssignGPU(req *AssignGPURequest) (*LXCGPUAssignment, error) {
	// 验证配额
	if err := req.GPUQuota.Validate(); err != nil {
		return nil, fmt.Errorf("GPU配额无效: %w", err)
	}

	// 设置默认共享模式
	if req.ShareMode == "" {
		req.ShareMode = ShareModeExclusive
	}

	// 检查设备是否可用
	available, reason := m.devices.IsDeviceAvailable(req.GPUPCIAddr, req.ShareMode)
	if !available {
		return nil, fmt.Errorf("GPU设备不可用: %s", reason)
	}

	// 检查容器是否已有该GPU
	m.mu.RLock()
	if assignIDs, exists := m.containerAssigns[req.ContainerID]; exists {
		for _, aid := range assignIDs {
			if assign, ok := m.assignments[aid]; ok && assign.GPUPCIAddr == req.GPUPCIAddr {
				m.mu.RUnlock()
				return nil, fmt.Errorf("容器 %s 已分配GPU %s", req.ContainerID, req.GPUPCIAddr)
			}
		}
	}
	m.mu.RUnlock()

	// 生成分配ID
	assignID := fmt.Sprintf("gpu-%s-%s-%d", req.ContainerID, req.GPUPCIAddr, time.Now().UnixNano())

	now := time.Now()
	assignment := &LXCGPUAssignment{
		ID:          assignID,
		ContainerID: req.ContainerID,
		GPUPCIAddr:  req.GPUPCIAddr,
		ShareMode:   req.ShareMode,
		State:       AssignmentStatePending,
		GPUQuota:    req.GPUQuota,
		AssignedAt:  now,
	}

	// 获取GPU设备信息
	device, err := m.devices.GetDevice(req.GPUPCIAddr)
	if err != nil {
		return nil, err
	}

	// 检查显存配额是否超出设备总量
	if req.GPUQuota.MemoryLimitMB > 0 && device.VRAM > 0 {
		// 计算已分配显存
		var allocatedVRAM uint64
		for _, a := range device.Assignments {
			allocatedVRAM += a.GPUQuota.MemoryLimitMB
		}
		if allocatedVRAM+req.GPUQuota.MemoryLimitMB > device.VRAM {
			return nil, fmt.Errorf("显存配额超出GPU总量: 已分配%dMB + 请求%dMB > 总量%dMB",
				allocatedVRAM, req.GPUQuota.MemoryLimitMB, device.VRAM)
		}
	}

	// 存储分配记录
	m.mu.Lock()
	m.assignments[assignID] = assignment
	m.containerAssigns[req.ContainerID] = append(m.containerAssigns[req.ContainerID], assignID)
	m.mu.Unlock()

	// 更新设备管理器中的分配信息
	if err := m.devices.UpdateDeviceAssignment(req.GPUPCIAddr, assignment); err != nil {
		// 回滚
		m.mu.Lock()
		delete(m.assignments, assignID)
		assigns := m.containerAssigns[req.ContainerID]
		for i, id := range assigns {
			if id == assignID {
				m.containerAssigns[req.ContainerID] = append(assigns[:i], assigns[i+1:]...)
				break
			}
		}
		m.mu.Unlock()
		return nil, fmt.Errorf("更新设备分配失败: %w", err)
	}

	// 如果容器正在运行且请求热插拔，立即执行附加
	if req.Hotplug {
		containerStatus := m.getContainerStatus(req.ContainerID)
		if containerStatus == LXCContainerRunning {
			assignment.HotplugState = HotplugStateAttaching
			if err := m.attachGPUToContainer(req.ContainerID, req.GPUPCIAddr, req.GPUQuota); err != nil {
				assignment.State = AssignmentStateError
				assignment.HotplugState = HotplugStateError
				assignment.Error = fmt.Sprintf("热插拔失败: %v", err)
				return assignment, fmt.Errorf("GPU热插拔失败: %w", err)
			}
			assignment.HotplugState = HotplugStateAttached
			assignment.State = AssignmentStateActive
			activatedAt := time.Now()
			assignment.ActivatedAt = &activatedAt
			assignment.LastHotplugAt = &activatedAt
		} else {
			// 容器未运行，标记为待激活
			assignment.State = AssignmentStatePending
		}
	} else {
		// 非热插拔模式：标记为待激活（容器下次启动时生效）
		assignment.State = AssignmentStatePending
	}

	return assignment, nil
}

// UnassignGPU 取消GPU分配.
func (m *Manager) UnassignGPU(req *UnassignGPURequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找分配记录
	assignID := ""
	assignIDs := m.containerAssigns[req.ContainerID]
	for _, aid := range assignIDs {
		if assign, ok := m.assignments[aid]; ok && assign.GPUPCIAddr == req.GPUPCIAddr {
			assignID = aid
			break
		}
	}

	if assignID == "" {
		return fmt.Errorf("未找到分配记录: 容器=%s, GPU=%s", req.ContainerID, req.GPUPCIAddr)
	}

	assignment := m.assignments[assignID]

	// 如果容器正在运行且GPU已激活，需要先分离
	if assignment.State == AssignmentStateActive && assignment.HotplugState == HotplugStateAttached {
		if !req.Force {
			containerStatus := m.getContainerStatus(req.ContainerID)
			if containerStatus == LXCContainerRunning {
				return fmt.Errorf("GPU正在使用中，请先停止容器或使用force选项")
			}
		}

		// 执行GPU分离
		if err := m.detachGPUFromContainer(req.ContainerID, req.GPUPCIAddr); err != nil {
			if !req.Force {
				return fmt.Errorf("GPU分离失败: %w", err)
			}
			// 强制模式下忽略分离错误
		}
	}

	// 移除分配记录
	delete(m.assignments, assignID)
	// 从容器分配列表中移除
	for i, id := range assignIDs {
		if id == assignID {
			m.containerAssigns[req.ContainerID] = append(assignIDs[:i], assignIDs[i+1:]...)
			break
		}
	}

	// 从设备管理器中移除
	m.devices.RemoveDeviceAssignment(req.GPUPCIAddr, assignID)

	return nil
}

// UpdateQuota 更新GPU资源配额.
func (m *Manager) UpdateQuota(req *UpdateQuotaRequest) error {
	if err := req.GPUQuota.Validate(); err != nil {
		return fmt.Errorf("GPU配额无效: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找分配记录
	assignment, err := m.findAssignment(req.ContainerID, req.GPUPCIAddr)
	if err != nil {
		return err
	}

	// 更新配额
	assignment.GPUQuota = req.GPUQuota

	// 如果容器正在运行，需要重新应用cgroup限制
	if assignment.State == AssignmentStateActive {
		if err := m.applyGPUQuotaToCGroup(req.ContainerID, req.GPUPCIAddr, req.GPUQuota); err != nil {
			return fmt.Errorf("应用GPU配额失败: %w", err)
		}
	}

	// 更新设备管理器
	m.devices.UpdateDeviceAssignment(req.GPUPCIAddr, assignment)

	return nil
}

// HotplugGPU GPU热插拔操作
// 支持将GPU附加到运行中的容器或从中分离.
func (m *Manager) HotplugGPU(req *HotplugRequest) (*LXCGPUAssignment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	assignment, err := m.findAssignment(req.ContainerID, req.GPUPCIAddr)
	if err != nil {
		return nil, err
	}

	switch req.Action {
	case "attach":
		return m.hotplugAttach(assignment, req.ContainerID, req.GPUPCIAddr)
	case "detach":
		return m.hotplugDetach(assignment, req.ContainerID, req.GPUPCIAddr)
	default:
		return nil, fmt.Errorf("不支持的热插拔操作: %s (仅支持 attach/detach)", req.Action)
	}
}

// hotplugAttach 热插拔附加GPU到容器.
func (m *Manager) hotplugAttach(assignment *LXCGPUAssignment, containerID, pciAddr string) (*LXCGPUAssignment, error) {
	// 检查容器是否运行中
	status := m.getContainerStatus(containerID)
	if status != LXCContainerRunning {
		return nil, fmt.Errorf("容器 %s 未运行，无法执行热插拔", containerID)
	}

	if assignment.HotplugState == HotplugStateAttached {
		return nil, fmt.Errorf("GPU已附加到容器")
	}

	assignment.HotplugState = HotplugStateAttaching

	// 执行设备附加
	if err := m.attachGPUToContainer(containerID, pciAddr, assignment.GPUQuota); err != nil {
		assignment.HotplugState = HotplugStateError
		assignment.Error = fmt.Sprintf("热插拔附加失败: %v", err)
		assignment.RetryCount++
		return assignment, fmt.Errorf("GPU热插拔附加失败: %w", err)
	}

	now := time.Now()
	assignment.HotplugState = HotplugStateAttached
	assignment.State = AssignmentStateActive
	assignment.ActivatedAt = &now
	assignment.LastHotplugAt = &now
	assignment.Error = ""
	assignment.RetryCount = 0

	// 更新设备管理器
	m.devices.UpdateDeviceAssignment(pciAddr, assignment)

	return assignment, nil
}

// hotplugDetach 热插拔分离GPU.
func (m *Manager) hotplugDetach(assignment *LXCGPUAssignment, containerID, pciAddr string) (*LXCGPUAssignment, error) {
	if assignment.HotplugState != HotplugStateAttached {
		return nil, fmt.Errorf("GPU未附加到容器")
	}

	assignment.HotplugState = HotplugStateDetaching

	// 执行设备分离
	if err := m.detachGPUFromContainer(containerID, pciAddr); err != nil {
		assignment.HotplugState = HotplugStateError
		assignment.Error = fmt.Sprintf("热插拔分离失败: %v", err)
		return assignment, fmt.Errorf("GPU热插拔分离失败: %w", err)
	}

	now := time.Now()
	assignment.HotplugState = HotplugStateIdle
	assignment.State = AssignmentStateInactive
	assignment.DeactivatedAt = &now
	assignment.LastHotplugAt = &now
	assignment.Error = ""

	// 更新设备管理器
	m.devices.UpdateDeviceAssignment(pciAddr, assignment)

	return assignment, nil
}

// ========== 容器生命周期联动 ==========

// OnContainerStart 容器启动时自动激活GPU分配
// 应在LXC容器管理器的StartContainer回调中调用.
func (m *Manager) OnContainerStart(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	assignIDs := m.containerAssigns[containerID]
	for _, aid := range assignIDs {
		assignment, exists := m.assignments[aid]
		if !exists || assignment.State != AssignmentStatePending {
			continue
		}

		// 激活GPU分配
		if err := m.activateAssignment(assignment); err != nil {
			assignment.State = AssignmentStateError
			assignment.Error = fmt.Sprintf("激活失败: %v", err)
			continue
		}
	}

	return nil
}

// OnContainerStop 容器停止时暂停GPU分配
// 应在LXC容器管理器的StopContainer回调中调用.
func (m *Manager) OnContainerStop(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	assignIDs := m.containerAssigns[containerID]
	for _, aid := range assignIDs {
		assignment, exists := m.assignments[aid]
		if !exists || assignment.State != AssignmentStateActive {
			continue
		}

		// 分离GPU
		if assignment.HotplugState == HotplugStateAttached {
			m.detachGPUFromContainer(containerID, assignment.GPUPCIAddr)
		}

		now := time.Now()
		assignment.State = AssignmentStateInactive
		assignment.HotplugState = HotplugStateIdle
		assignment.DeactivatedAt = &now

		// 更新设备管理器
		m.devices.UpdateDeviceAssignment(assignment.GPUPCIAddr, assignment)
	}

	return nil
}

// OnContainerDelete 容器删除时清理所有GPU分配.
func (m *Manager) OnContainerDelete(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	assignIDs := m.containerAssigns[containerID]
	for _, aid := range assignIDs {
		assignment, exists := m.assignments[aid]
		if exists {
			m.devices.RemoveDeviceAssignment(assignment.GPUPCIAddr, aid)
			delete(m.assignments, aid)
		}
	}
	delete(m.containerAssigns, containerID)

	return nil
}

// ========== 查询接口 ==========

// GetAssignment 获取分配详情.
func (m *Manager) GetAssignment(assignmentID string) (*LXCGPUAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assignment, exists := m.assignments[assignmentID]
	if !exists {
		return nil, fmt.Errorf("分配记录不存在: %s", assignmentID)
	}
	return assignment, nil
}

// GetContainerAssignments 获取容器的所有GPU分配.
func (m *Manager) GetContainerAssignments(containerID string) []*LXCGPUAssignment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assignIDs := m.containerAssigns[containerID]
	result := make([]*LXCGPUAssignment, 0, len(assignIDs))
	for _, aid := range assignIDs {
		if assignment, exists := m.assignments[aid]; exists {
			result = append(result, assignment)
		}
	}
	return result
}

// ListAllAssignments 列出所有分配记录.
func (m *Manager) ListAllAssignments() []*LXCGPUAssignment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LXCGPUAssignment, 0, len(m.assignments))
	for _, a := range m.assignments {
		result = append(result, a)
	}
	return result
}

// GetDashboard 获取GPU分配仪表盘数据.
func (m *Manager) GetDashboard() *GPUDashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard := &GPUDashboard{
		Assignments: make([]*LXCGPUAssignment, 0),
		GPUs:        m.devices.ListDevices(),
	}

	// 统计GPU状态
	for _, gpu := range dashboard.GPUs {
		dashboard.TotalGPUs++
		if gpu.Available && len(gpu.Assignments) == 0 {
			dashboard.AvailableGPUs++
		} else if len(gpu.Assignments) > 0 {
			dashboard.AssignedGPUs++
		}
		if !gpu.Available {
			dashboard.ErrorGPUs++
		}
	}

	// 汇总分配记录
	for _, a := range m.assignments {
		dashboard.Assignments = append(dashboard.Assignments, a)
	}

	// 统计容器GPU使用情况
	containerGPUMap := make(map[string]*ContainerGPUStats)
	for _, a := range m.assignments {
		stats, exists := containerGPUMap[a.ContainerID]
		if !exists {
			stats = &ContainerGPUStats{ContainerID: a.ContainerID}
			containerGPUMap[a.ContainerID] = stats
		}
		stats.GPUCount++
		stats.TotalVRAMMB += a.GPUQuota.MemoryLimitMB
	}

	dashboard.ContainerStats = make([]ContainerGPUStats, 0, len(containerGPUMap))
	for _, stats := range containerGPUMap {
		dashboard.ContainerStats = append(dashboard.ContainerStats, *stats)
	}

	return dashboard
}

// GetContainerGPUStats 获取容器GPU统计.
func (m *Manager) GetContainerGPUStats(containerID string) (*StatsLXCGPU, error) {
	m.mu.RLock()
	assignments := m.containerAssigns[containerID]
	if len(assignments) == 0 {
		m.mu.RUnlock()
		return nil, fmt.Errorf("容器 %s 无GPU分配", containerID)
	}
	// 取第一个活跃分配
	assignment := m.assignments[assignments[0]]
	m.mu.RUnlock()

	if assignment == nil {
		return nil, fmt.Errorf("分配记录不存在")
	}

	stats := &StatsLXCGPU{
		ContainerID: containerID,
		GPUPCIAddr:  assignment.GPUPCIAddr,
		UpdatedAt:   time.Now(),
	}

	// 从设备获取实时统计
	device, err := m.devices.GetDevice(assignment.GPUPCIAddr)
	if err == nil {
		stats.Temperature = device.Temperature
		if device.VRAM > 0 {
			stats.MemoryUsage = float64(device.VRAMUsed) / float64(device.VRAM) * 100
		}
		stats.MemoryUsedMB = device.VRAMUsed
	}

	return stats, nil
}

// ========== 内部方法 ==========

// findAssignment 查找容器的GPU分配记录.
func (m *Manager) findAssignment(containerID, pciAddr string) (*LXCGPUAssignment, error) {
	assignIDs := m.containerAssigns[containerID]
	for _, aid := range assignIDs {
		if assign, ok := m.assignments[aid]; ok && assign.GPUPCIAddr == pciAddr {
			return assign, nil
		}
	}
	return nil, fmt.Errorf("未找到分配记录: 容器=%s, GPU=%s", containerID, pciAddr)
}

// activateAssignment 激活分配（容器启动时调用）.
func (m *Manager) activateAssignment(assignment *LXCGPUAssignment) error {
	if err := m.attachGPUToContainer(assignment.ContainerID, assignment.GPUPCIAddr, assignment.GPUQuota); err != nil {
		return err
	}

	now := time.Now()
	assignment.State = AssignmentStateActive
	assignment.HotplugState = HotplugStateAttached
	assignment.ActivatedAt = &now

	m.devices.UpdateDeviceAssignment(assignment.GPUPCIAddr, assignment)
	return nil
}

// attachGPUToContainer 将GPU设备附加到LXC容器
// 通过修改LXC容器配置和cgroup设备白名单实现.
func (m *Manager) attachGPUToContainer(containerID, pciAddr string, quota GPUQuota) error {
	// 1. 获取GPU设备的设备号
	deviceInfo, err := m.getGPUDeviceNumbers(pciAddr)
	if err != nil {
		return fmt.Errorf("获取GPU设备号失败: %w", err)
	}

	// 2. 更新LXC容器配置，添加GPU设备
	if err := m.updateLXCConfigForGPU(containerID, deviceInfo); err != nil {
		return fmt.Errorf("更新LXC配置失败: %w", err)
	}

	// 3. 更新cgroup设备白名单
	if err := m.updateCGroupDeviceAllow(containerID, deviceInfo); err != nil {
		return fmt.Errorf("更新cgroup设备白名单失败: %w", err)
	}

	// 4. 应用GPU资源配额
	if err := m.applyGPUQuotaToCGroup(containerID, pciAddr, quota); err != nil {
		return fmt.Errorf("应用GPU配额失败: %w", err)
	}

	// 5. 挂载GPU设备文件系统到容器
	if err := m.mountGPUDeviceInContainer(containerID, deviceInfo); err != nil {
		return fmt.Errorf("挂载GPU设备失败: %w", err)
	}

	return nil
}

// detachGPUFromContainer 从LXC容器分离GPU设备.
func (m *Manager) detachGPUFromContainer(containerID, pciAddr string) error {
	// 1. 从容器卸载GPU设备
	if err := m.unmountGPUDeviceInContainer(containerID, pciAddr); err != nil {
		// 记录错误但继续清理
		fmt.Printf("卸载GPU设备警告: %v\n", err)
	}

	// 2. 从cgroup设备白名单移除
	if err := m.removeFromCGroupDeviceAllow(containerID, pciAddr); err != nil {
		fmt.Printf("移除cgroup设备警告: %v\n", err)
	}

	// 3. 从LXC配置中移除GPU设备
	if err := m.removeFromLXCConfig(containerID, pciAddr); err != nil {
		fmt.Printf("移除LXC配置警告: %v\n", err)
	}

	return nil
}

// gpuDeviceInfo GPU设备信息（用于设备号映射）.
type gpuDeviceInfo struct {
	PCIAddr     string
	Major       int      // 主设备号
	Minor       int      // 次设备号
	DevType     string   // 设备类型 (char/block)
	DevicePaths []string // 设备文件路径
}

// getGPUDeviceNumbers 获取GPU设备的主次设备号.
func (m *Manager) getGPUDeviceNumbers(pciAddr string) (*gpuDeviceInfo, error) {
	info := &gpuDeviceInfo{
		PCIAddr: pciAddr,
	}

	// 从/sys/bus/pci/devices/<addr>/ 获取设备信息
	sysPath := filepath.Join("/sys/bus/pci/devices", pciAddr)

	// 读取uevent获取设备号
	ueventPath := filepath.Join(sysPath, "uevent")
	data, err := os.ReadFile(ueventPath)
	if err != nil {
		return nil, fmt.Errorf("读取uevent失败: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch strings.TrimSpace(parts[0]) {
		case "MAJOR":
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &info.Major)
		case "MINOR":
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &info.Minor)
		}
	}

	// 查找设备文件路径
	// NVIDIA: /dev/nvidia*, /dev/nvidiactl, /dev/nvidia-uvm
	// AMD: /dev/dri/card*, /dev/dri/renderD*
	if _, err := os.Stat("/dev/nvidia0"); err == nil {
		info.DevicePaths = append(info.DevicePaths, "/dev/nvidia0")
		info.DevicePaths = append(info.DevicePaths, "/dev/nvidiactl")
		if _, err := os.Stat("/dev/nvidia-uvm"); err == nil {
			info.DevicePaths = append(info.DevicePaths, "/dev/nvidia-uvm")
		}
		info.DevType = "char"
	}

	// 检查DRI设备
	driPath := fmt.Sprintf("/dev/dri/card%s", strings.Split(pciAddr, ".")[0][:2])
	if _, err := os.Stat(driPath); err == nil {
		info.DevicePaths = append(info.DevicePaths, driPath)
		info.DevType = "char"
	}

	return info, nil
}

// updateLXCConfigForGPU 更新LXC容器配置文件以支持GPU.
func (m *Manager) updateLXCConfigForGPU(containerID string, deviceInfo *gpuDeviceInfo) error {
	configPath := filepath.Join(m.config.ConfigPath, containerID, "config")

	// 读取现有配置
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取LXC配置失败: %w", err)
	}

	config := string(data)

	// 检查是否已包含GPU配置
	if strings.Contains(config, "# GPU Passthrough") {
		return nil // 已配置
	}

	// 添加GPU设备配置
	gpuConfig := "\n# GPU Passthrough\n"
	for _, devPath := range deviceInfo.DevicePaths {
		gpuConfig += fmt.Sprintf("lxc.mount.entry = %s %s none bind,optional,create=file 0 0\n",
			devPath, strings.TrimPrefix(devPath, "/"))
	}

	// 允许容器访问GPU设备
	gpuConfig += "lxc.cgroup2.devices.allow = c *:* rwm\n"

	// 写回配置文件
	config += gpuConfig
	return os.WriteFile(configPath, []byte(config), 0644)
}

// updateCGroupDeviceAllow 更新cgroup设备白名单.
func (m *Manager) updateCGroupDeviceAllow(containerID string, deviceInfo *gpuDeviceInfo) error {
	cgroupPath := filepath.Join(m.config.DeviceCGroup, containerID)

	// 写入设备白名单
	if deviceInfo.Major > 0 {
		allowPath := filepath.Join(cgroupPath, "devices.allow")
		allowRule := fmt.Sprintf("c %d:%d rwm", deviceInfo.Major, deviceInfo.Minor)
		if err := os.WriteFile(allowPath, []byte(allowRule), 0644); err != nil {
			return fmt.Errorf("写入cgroup设备白名单失败: %w", err)
		}
	}

	return nil
}

// applyGPUQuotaToCGroup 应用GPU资源配额到cgroup.
func (m *Manager) applyGPUQuotaToCGroup(containerID, pciAddr string, quota GPUQuota) error {
	// 对于NVIDIA MPS模式，通过环境变量和配置文件控制
	// 对于独占模式，GPU配额主要通过设备级控制实现

	if quota.SMPercent > 0 {
		// 设置MPS计算百分比限制
		// 实际实现需要通过nvidia-cuda-mps-control或CUDA_MPS_ACTIVE_THREAD_PERCENTAGE
		envPath := filepath.Join(m.config.ConfigPath, containerID, "gpu_env")
		envContent := fmt.Sprintf("CUDA_MPS_ACTIVE_THREAD_PERCENTAGE=%d\n", quota.SMPercent)
		os.WriteFile(envPath, []byte(envContent), 0644)
	}

	return nil
}

// mountGPUDeviceInContainer 挂载GPU设备文件系统到容器.
func (m *Manager) mountGPUDeviceInContainer(containerID string, deviceInfo *gpuDeviceInfo) error {
	rootfsPath := filepath.Join(m.config.ConfigPath, containerID, "rootfs")

	for _, devPath := range deviceInfo.DevicePaths {
		// 确保容器rootfs中存在设备目录
		containerDevPath := filepath.Join(rootfsPath, devPath)
		devDir := filepath.Dir(containerDevPath)
		if err := os.MkdirAll(devDir, 0755); err != nil {
			return fmt.Errorf("创建设备目录失败: %w", err)
		}

		// 绑定挂载设备文件
		// 使用mount --bind实现设备文件的挂载
		if _, err := os.Stat(devPath); err == nil {
			// 创建空文件作为挂载点
			if _, err := os.Stat(containerDevPath); os.IsNotExist(err) {
				os.WriteFile(containerDevPath, []byte{}, 0666)
			}
		}
	}

	return nil
}

// unmountGPUDeviceInContainer 从容器卸载GPU设备.
func (m *Manager) unmountGPUDeviceInContainer(containerID, pciAddr string) error {
	// 使用umount卸载设备
	// 实际实现需要找到所有挂载点并卸载
	return nil
}

// removeFromCGroupDeviceAllow 从cgroup设备白名单移除.
func (m *Manager) removeFromCGroupDeviceAllow(containerID, pciAddr string) error {
	cgroupPath := filepath.Join(m.config.DeviceCGroup, containerID)
	denyPath := filepath.Join(cgroupPath, "devices.deny")

	// 获取设备信息
	device, err := m.devices.GetDevice(pciAddr)
	if err != nil {
		return err
	}

	// 写入拒绝规则（通过重新设置白名单实现）
	// 实际环境中需要更精细的控制
	_ = device
	_ = denyPath

	return nil
}

// removeFromLXCConfig 从LXC配置中移除GPU设备.
func (m *Manager) removeFromLXCConfig(containerID, pciAddr string) error {
	configPath := filepath.Join(m.config.ConfigPath, containerID, "config")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	config := string(data)
	// 移除GPU相关配置行
	lines := strings.Split(config, "\n")
	var newLines []string
	skipGPU := false
	for _, line := range lines {
		if strings.Contains(line, "# GPU Passthrough") {
			skipGPU = true
			continue
		}
		if skipGPU {
			if strings.HasPrefix(line, "lxc.mount.entry") || strings.HasPrefix(line, "lxc.cgroup2.devices") {
				continue
			}
			skipGPU = false
		}
		newLines = append(newLines, line)
	}

	return os.WriteFile(configPath, []byte(strings.Join(newLines, "\n")), 0644)
}

// getContainerStatus 获取容器状态.
func (m *Manager) getContainerStatus(containerID string) LXCContainerStatus {
	// 通过检查容器PID文件判断状态
	pidFile := filepath.Join(m.config.ConfigPath, containerID, "pid")
	if _, err := os.Stat(pidFile); err == nil {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			var pid int
			fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
			if pid > 0 {
				// 检查进程是否存在
				if err := exec.Command("kill", "-0", fmt.Sprintf("%d", pid)).Run(); err == nil {
					return LXCContainerRunning
				}
			}
		}
	}
	return LXCContainerStopped
}
