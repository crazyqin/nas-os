// Package gpupassthrough GPU直通管理核心模块
package gpupassthrough

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager GPU直通管理器
type Manager struct {
	config           *Config
	devices          map[string]*GPUDevice            // PCI地址 -> 设备
	vmAssigns        map[string][]VMAssignment        // VM ID -> 分配列表
	containerAssigns map[string][]ContainerAssignment // 容器ID -> 分配列表
	alerts           []GPUAlert
	mu               sync.RWMutex
	stopCh           chan struct{}
}

// NewManager 创建GPU直通管理器
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	mgr := &Manager{
		config:           config,
		devices:          make(map[string]*GPUDevice),
		vmAssigns:        make(map[string][]VMAssignment),
		containerAssigns: make(map[string][]ContainerAssignment),
		alerts:           make([]GPUAlert, 0),
		stopCh:           make(chan struct{}),
	}

	// 加载持久化配置
	if err := mgr.loadConfig(); err != nil {
		// 配置文件不存在时忽略错误
		fmt.Printf("加载GPU直通配置: %v\n", err)
	}

	return mgr
}

// DiscoverDevices 发现GPU设备
func (m *Manager) DiscoverDevices() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 扫描PCI设备
	newDevices, err := m.scanPCIDevices()
	if err != nil {
		return fmt.Errorf("扫描PCI设备失败: %w", err)
	}

	// 更新设备列表
	for pciAddr, device := range newDevices {
		if existing, exists := m.devices[pciAddr]; exists {
			// 保留分配信息
			device.VMAssignments = existing.VMAssignments
			device.ContainerAssign = existing.ContainerAssign
		}
		m.devices[pciAddr] = device
	}

	return nil
}

// scanPCIDevices 扫描PCI总线上的GPU设备
func (m *Manager) scanPCIDevices() (map[string]*GPUDevice, error) {
	devices := make(map[string]*GPUDevice)

	// 读取 /sys/bus/pci/devices/ 目录
	pciBasePath := "/sys/bus/pci/devices"
	entries, err := os.ReadDir(pciBasePath)
	if err != nil {
		return nil, fmt.Errorf("读取PCI设备目录失败: %w", err)
	}

	for _, entry := range entries {
		pciAddr := entry.Name()
		devicePath := filepath.Join(pciBasePath, pciAddr)

		// 读取设备类别，筛选显示设备(0x0300xx)和3D加速器(0x0302xx)
		classBytes, err := os.ReadFile(filepath.Join(devicePath, "class"))
		if err != nil {
			continue
		}
		classStr := strings.TrimSpace(string(classBytes))
		if !strings.HasPrefix(classStr, "0x03") {
			continue
		}

		// 读取厂商ID和设备ID
		vendorBytes, _ := os.ReadFile(filepath.Join(devicePath, "vendor"))
		deviceIDBytes, _ := os.ReadFile(filepath.Join(devicePath, "device"))

		vendorID := strings.TrimSpace(string(vendorBytes))
		deviceID := strings.TrimSpace(string(deviceIDBytes))

		// 确定厂商名称
		vendor := "unknown"
		switch vendorID {
		case "0x10de":
			vendor = "nvidia"
		case "0x1002":
			vendor = "amd"
		case "0x8086":
			vendor = "intel"
		}

		// 读取当前驱动
		driver := "none"
		driverLink := filepath.Join(devicePath, "driver")
		if target, err := os.Readlink(driverLink); err == nil {
			driver = filepath.Base(target)
		}

		// 确定绑定状态
		bindState := BindStateUnbind
		switch driver {
		case "vfio-pci":
			bindState = BindStateVfio
		case "nvidia", "amdgpu", "i915":
			bindState = BindStateNative
		}

		// 读取IOMMU组
		iommuGroup := -1
		iommuLink := filepath.Join(devicePath, "iommu_group")
		if target, err := os.Readlink(iommuLink); err == nil {
			groupPath := filepath.Base(target)
			fmt.Sscanf(groupPath, "%d", &iommuGroup)
		}

		// 读取NUMA节点
		numaNode := -1
		numaBytes, err := os.ReadFile(filepath.Join(devicePath, "numa_node"))
		if err == nil {
			fmt.Sscanf(strings.TrimSpace(string(numaBytes)), "%d", &numaNode)
		}

		// 构建设备路径
		devPath := ""
		if bindState == BindStateVfio && iommuGroup >= 0 {
			devPath = fmt.Sprintf("/dev/vfio/%d", iommuGroup)
		}

		// 读取型号信息（从 uevent 或 vendor/device 文件）
		model := fmt.Sprintf("%s GPU (%s)", vendor, pciAddr)

		device := &GPUDevice{
			PCIAddress: pciAddr,
			VendorID:   vendorID,
			DeviceID:   deviceID,
			Model:      model,
			Vendor:     vendor,
			Driver:     driver,
			BindState:  bindState,
			IOMMUGroup: iommuGroup,
			NUMANode:   numaNode,
			DevicePath: devPath,
			Status:     DeviceStatusAvailable,
			UpdatedAt:  time.Now(),
		}

		devices[pciAddr] = device
	}

	return devices, nil
}

// ListDevices 列出所有GPU设备
func (m *Manager) ListDevices() []*GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*GPUDevice, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, device)
	}
	return devices
}

// GetDevice 获取指定GPU设备详情
func (m *Manager) GetDevice(pciAddr string) (*GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[pciAddr]
	if !exists {
		return nil, fmt.Errorf("GPU设备不存在: %s", pciAddr)
	}

	return device, nil
}

// AssignGPU 分配GPU给VM或容器
func (m *Manager) AssignGPU(pciAddr string, req *AssignRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[pciAddr]
	if !exists {
		return fmt.Errorf("GPU设备不存在: %s", pciAddr)
	}

	if device.Status == DeviceStatusOffline {
		return fmt.Errorf("GPU设备已离线: %s", pciAddr)
	}

	// 根据目标类型分配
	switch req.TargetType {
	case "vm":
		// 检查是否已分配给该VM
		for _, assign := range device.VMAssignments {
			if assign.VMID == req.TargetID {
				return fmt.Errorf("GPU已分配给VM: %s", req.TargetID)
			}
		}

		// 独占模式下检查是否已有分配
		if req.ShareMode == string(ShareModeExclusive) || req.ShareMode == "" {
			if len(device.VMAssignments) > 0 || len(device.ContainerAssign) > 0 {
				return fmt.Errorf("GPU已被占用，无法独占分配")
			}
		}

		assignment := VMAssignment{
			VMID:       req.TargetID,
			GPUPCIAddr: pciAddr,
			Status:     "active",
			AssignedAt: time.Now(),
		}
		device.VMAssignments = append(device.VMAssignments, assignment)
		m.vmAssigns[req.TargetID] = append(m.vmAssigns[req.TargetID], assignment)

	case "container":
		// 检查是否已分配给该容器
		for _, assign := range device.ContainerAssign {
			if assign.ContainerID == req.TargetID {
				return fmt.Errorf("GPU已分配给容器: %s", req.TargetID)
			}
		}

		// 独占模式下检查是否已有分配
		if req.ShareMode == string(ShareModeExclusive) || req.ShareMode == "" {
			if len(device.VMAssignments) > 0 || len(device.ContainerAssign) > 0 {
				return fmt.Errorf("GPU已被占用，无法独占分配")
			}
		}

		shareMode := ShareMode(req.ShareMode)
		if shareMode == "" {
			shareMode = ShareModeExclusive
		}

		assignment := ContainerAssignment{
			ContainerID: req.TargetID,
			GPUPCIAddr:  pciAddr,
			ShareMode:   shareMode,
			MemoryLimit: req.MemoryLimit,
			AssignedAt:  time.Now(),
		}
		device.ContainerAssign = append(device.ContainerAssign, assignment)
		m.containerAssigns[req.TargetID] = append(m.containerAssigns[req.TargetID], assignment)

	default:
		return fmt.Errorf("不支持的目标类型: %s", req.TargetType)
	}

	// 更新设备状态
	device.Status = DeviceStatusAssigned
	device.UpdatedAt = time.Now()

	// 持久化配置
	return m.saveConfig()
}

// UnassignGPU 取消GPU分配
func (m *Manager) UnassignGPU(pciAddr, targetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[pciAddr]
	if !exists {
		return fmt.Errorf("GPU设备不存在: %s", pciAddr)
	}

	found := false

	// 尝试从VM分配中移除
	for i, assign := range device.VMAssignments {
		if assign.VMID == targetID {
			device.VMAssignments = append(device.VMAssignments[:i], device.VMAssignments[i+1:]...)
			// 从全局映射中移除
			assigns := m.vmAssigns[targetID]
			for j, a := range assigns {
				if a.GPUPCIAddr == pciAddr {
					m.vmAssigns[targetID] = append(assigns[:j], assigns[j+1:]...)
					break
				}
			}
			found = true
			break
		}
	}

	// 尝试从容器分配中移除
	if !found {
		for i, assign := range device.ContainerAssign {
			if assign.ContainerID == targetID {
				device.ContainerAssign = append(device.ContainerAssign[:i], device.ContainerAssign[i+1:]...)
				// 从全局映射中移除
				assigns := m.containerAssigns[targetID]
				for j, a := range assigns {
					if a.GPUPCIAddr == pciAddr {
						m.containerAssigns[targetID] = append(assigns[:j], assigns[j+1:]...)
						break
					}
				}
				found = true
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("未找到分配记录: GPU=%s, 目标=%s", pciAddr, targetID)
	}

	// 如果没有分配，恢复可用状态
	if len(device.VMAssignments) == 0 && len(device.ContainerAssign) == 0 {
		device.Status = DeviceStatusAvailable
	}

	device.UpdatedAt = time.Now()

	// 持久化配置
	return m.saveConfig()
}

// GetDeviceStats 获取GPU设备统计信息
func (m *Manager) GetDeviceStats(pciAddr string) (*GPUStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[pciAddr]
	if !exists {
		return nil, fmt.Errorf("GPU设备不存在: %s", pciAddr)
	}

	// 返回当前缓存的统计信息
	stats := &GPUStats{
		PCIAddress:  pciAddr,
		GPUUsage:    0, // 实际使用需要nvidia-smi或类似工具
		MemoryUsage: 0,
		MemoryUsed:  device.VRAMUsed,
		MemoryTotal: device.VRAM,
		Temperature: device.Temperature,
		PowerUsage:  device.PowerUsage,
		UpdatedAt:   time.Now(),
	}

	if device.VRAM > 0 {
		stats.MemoryUsage = float64(device.VRAMUsed) / float64(device.VRAM) * 100
	}

	return stats, nil
}

// BindVFIO 绑定GPU到VFIO驱动
func (m *Manager) BindVFIO(pciAddr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[pciAddr]
	if !exists {
		return fmt.Errorf("GPU设备不存在: %s", pciAddr)
	}

	if device.BindState == BindStateVfio {
		return fmt.Errorf("GPU已绑定到VFIO驱动")
	}

	// 检查是否有活跃分配
	if device.Status == DeviceStatusAssigned {
		return fmt.Errorf("GPU有活跃分配，无法切换驱动")
	}

	// 执行VFIO绑定（模拟）
	if err := m.bindDriver(pciAddr, "vfio-pci"); err != nil {
		return fmt.Errorf("绑定VFIO驱动失败: %w", err)
	}

	device.Driver = "vfio-pci"
	device.BindState = BindStateVfio
	device.UpdatedAt = time.Now()

	return m.saveConfig()
}

// UnbindVFIO 解绑GPU驱动
func (m *Manager) UnbindVFIO(pciAddr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[pciAddr]
	if !exists {
		return fmt.Errorf("GPU设备不存在: %s", pciAddr)
	}

	if device.BindState != BindStateVfio {
		return fmt.Errorf("GPU未绑定到VFIO驱动")
	}

	// 检查是否有活跃分配
	if device.Status == DeviceStatusAssigned {
		return fmt.Errorf("GPU有活跃分配，无法解绑驱动")
	}

	// 执行解绑（模拟）
	if err := m.unbindDriver(pciAddr); err != nil {
		return fmt.Errorf("解绑驱动失败: %w", err)
	}

	device.Driver = "none"
	device.BindState = BindStateUnbind
	device.UpdatedAt = time.Now()

	return m.saveConfig()
}

// bindDriver 绑定驱动（内部方法）
func (m *Manager) bindDriver(pciAddr, driver string) error {
	// 写入驱动绑定文件
	bindPath := "/sys/bus/pci/drivers/vfio-pci/bind"
	if err := os.WriteFile(bindPath, []byte(pciAddr), 0644); err != nil {
		return fmt.Errorf("写入绑定文件失败: %w", err)
	}
	return nil
}

// unbindDriver 解绑驱动（内部方法）
func (m *Manager) unbindDriver(pciAddr string) error {
	// 写入解绑文件
	unbindPath := "/sys/bus/pci/drivers/vfio-pci/unbind"
	if err := os.WriteFile(unbindPath, []byte(pciAddr), 0644); err != nil {
		return fmt.Errorf("写入解绑文件失败: %w", err)
	}
	return nil
}

// GetAllAssignments 获取所有分配信息
func (m *Manager) GetAllAssignments() ([]VMAssignment, []ContainerAssignment) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vmAssigns := make([]VMAssignment, 0)
	for _, assigns := range m.vmAssigns {
		vmAssigns = append(vmAssigns, assigns...)
	}

	containerAssigns := make([]ContainerAssignment, 0)
	for _, assigns := range m.containerAssigns {
		containerAssigns = append(containerAssigns, assigns...)
	}

	return vmAssigns, containerAssigns
}

// GetAlerts 获取GPU告警列表
func (m *Manager) GetAlerts() []GPUAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.alerts
}

// CheckAlerts 检查并生成告警
func (m *Manager) CheckAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, device := range m.devices {
		// 温度告警
		if device.Temperature >= m.config.AlertTempError {
			m.addAlert(AlertLevelError, device.PCIAddress,
				fmt.Sprintf("GPU温度过高: %d°C (阈值: %d°C)", device.Temperature, m.config.AlertTempError))
		} else if device.Temperature >= m.config.AlertTempWarning {
			m.addAlert(AlertLevelWarning, device.PCIAddress,
				fmt.Sprintf("GPU温度偏高: %d°C (阈值: %d°C)", device.Temperature, m.config.AlertTempWarning))
		}

		// 功耗告警
		if device.PowerUsage > m.config.AlertPowerLimit {
			m.addAlert(AlertLevelWarning, device.PCIAddress,
				fmt.Sprintf("GPU功耗超标: %dW (阈值: %dW)", device.PowerUsage, m.config.AlertPowerLimit))
		}
	}
}

// addAlert 添加告警
func (m *Manager) addAlert(level AlertLevel, pciAddr, message string) {
	alert := GPUAlert{
		Level:      level,
		PCIAddress: pciAddr,
		Message:    message,
		Timestamp:  time.Now(),
	}
	m.alerts = append(m.alerts, alert)

	// 只保留最近100条告警
	if len(m.alerts) > 100 {
		m.alerts = m.alerts[len(m.alerts)-100:]
	}
}

// saveConfig 保存配置到文件
func (m *Manager) saveConfig() error {
	data := struct {
		Devices          map[string]*GPUDevice            `json:"devices"`
		VMAssignments    map[string][]VMAssignment        `json:"vmAssignments"`
		ContainerAssigns map[string][]ContainerAssignment `json:"containerAssignments"`
	}{
		Devices:          m.devices,
		VMAssignments:    m.vmAssigns,
		ContainerAssigns: m.containerAssigns,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(m.config.ConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.config.ConfigPath, jsonData, 0644)
}

// loadConfig 从文件加载配置
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.config.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	config := struct {
		Devices          map[string]*GPUDevice            `json:"devices"`
		VMAssignments    map[string][]VMAssignment        `json:"vmAssignments"`
		ContainerAssigns map[string][]ContainerAssignment `json:"containerAssignments"`
	}{}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	if config.Devices != nil {
		m.devices = config.Devices
	}
	if config.VMAssignments != nil {
		m.vmAssigns = config.VMAssignments
	}
	if config.ContainerAssigns != nil {
		m.containerAssigns = config.ContainerAssigns
	}

	return nil
}

// Close 关闭管理器
func (m *Manager) Close() error {
	close(m.stopCh)
	return nil
}
