package lxcgpu

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DeviceManager GPU设备发现与管理器
// 负责扫描、识别、监控GPU设备，支持NVIDIA和AMD显卡
type DeviceManager struct {
	mu       sync.RWMutex
	devices  map[string]*GPUDevice // PCI地址 -> 设备
	lxcCfg   *LXCConfig
	stopCh   chan struct{}
	polling  bool
}

// NewDeviceManager 创建设备管理器
func NewDeviceManager(lxcCfg *LXCConfig) *DeviceManager {
	if lxcCfg == nil {
		lxcCfg = DefaultLXCConfig()
	}
	return &DeviceManager{
		devices: make(map[string]*GPUDevice),
		lxcCfg:  lxcCfg,
		stopCh:  make(chan struct{}),
	}
}

// DiscoverDevices 扫描并发现系统中的GPU设备
// 通过/sys/bus/pci/devices/和nvidia-smi等工具识别GPU
func (dm *DeviceManager) DiscoverDevices() ([]*GPUDevice, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 扫描PCI总线
	pciDevices, err := dm.scanPCIDevices()
	if err != nil {
		return nil, fmt.Errorf("扫描PCI设备失败: %w", err)
	}

	// 更新设备列表，保留已有分配信息
	for pciAddr, device := range pciDevices {
		if existing, exists := dm.devices[pciAddr]; exists {
			// 保留分配信息
			device.Assignments = existing.Assignments
			device.UpdatedAt = time.Now()
		}
		dm.devices[pciAddr] = device
	}

	// 检测离线设备
	for pciAddr := range dm.devices {
		if _, found := pciDevices[pciAddr]; !found {
			dm.devices[pciAddr].Available = false
			dm.devices[pciAddr].UpdatedAt = time.Now()
		}
	}

	// 尝试获取NVIDIA设备详细信息
	dm.enrichNVIDIAInfo()

	// 尝试获取AMD设备详细信息
	dm.enrichAMDInfo()

	result := make([]*GPUDevice, 0, len(dm.devices))
	for _, d := range dm.devices {
		result = append(result, d)
	}
	return result, nil
}

// scanPCIDevices 扫描PCI总线上的GPU设备
func (dm *DeviceManager) scanPCIDevices() (map[string]*GPUDevice, error) {
	devices := make(map[string]*GPUDevice)

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
		// 0x03 = 显示控制器，0x0300 = VGA兼容，0x0302 = 3D控制器
		if !strings.HasPrefix(classStr, "0x03") {
			continue
		}

		// 读取厂商ID和设备ID
		vendorBytes, _ := os.ReadFile(filepath.Join(devicePath, "vendor"))
		deviceIDBytes, _ := os.ReadFile(filepath.Join(devicePath, "device"))

		vendorID := strings.TrimSpace(string(vendorBytes))
		deviceID := strings.TrimSpace(string(deviceIDBytes))

		// 确定厂商
		vendor := GPUVendorUnknown
		switch vendorID {
		case "0x10de":
			vendor = GPUVendorNVIDIA
		case "0x1002":
			vendor = GPUVendorAMD
		case "0x8086":
			vendor = GPUVendorIntel
		}

		// 读取当前驱动
		driver := "none"
		driverLink := filepath.Join(devicePath, "driver")
		if target, err := os.Readlink(driverLink); err == nil {
			driver = filepath.Base(target)
		}

		// 读取NUMA节点
		numaNode := -1
		numaBytes, err := os.ReadFile(filepath.Join(devicePath, "numa_node"))
		if err == nil {
			fmt.Sscanf(strings.TrimSpace(string(numaBytes)), "%d", &numaNode)
		}

		// 构建设备路径
		devPath := ""
		if driver == "vfio-pci" {
			iommuLink := filepath.Join(devicePath, "iommu_group")
			if target, err := os.Readlink(iommuLink); err == nil {
				groupPath := filepath.Base(target)
				var iommuGroup int
				fmt.Sscanf(groupPath, "%d", &iommuGroup)
				devPath = fmt.Sprintf("/dev/vfio/%d", iommuGroup)
			}
		}

		model := fmt.Sprintf("%s GPU (%s)", vendor, pciAddr)

		device := &GPUDevice{
			PCIAddress: pciAddr,
			VendorID:   vendorID,
			DeviceID:   deviceID,
			Model:      model,
			Vendor:     vendor,
			Driver:     driver,
			NUMANode:   numaNode,
			DevicePath: devPath,
			Available:  true,
			UpdatedAt:  time.Now(),
		}

		devices[pciAddr] = device
	}

	return devices, nil
}

// enrichNVIDIAInfo 使用nvidia-smi补充NVIDIA设备信息
func (dm *DeviceManager) enrichNVIDIAInfo() {
	// 检查nvidia-smi是否可用
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return
	}

	// 查询GPU信息：PCI地址、型号、显存、驱动版本
	cmd := exec.Command("nvidia-smi", "--query-gpu=pci.bus_id,name,memory.total,memory.used,driver_version,compute_cap",
		"--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ",", 6)
		if len(parts) < 5 {
			continue
		}

		// nvidia-smi返回的PCI地址格式: 00000000:01:00.0，需要转换
		pciAddr := dm.normalizePCIAddr(strings.TrimSpace(parts[0]))

		dm.mu.Lock()
		device, exists := dm.devices[pciAddr]
		if exists {
			device.Model = strings.TrimSpace(parts[1])
			fmt.Sscanf(strings.TrimSpace(parts[2]), "%d", &device.VRAM)
			fmt.Sscanf(strings.TrimSpace(parts[3]), "%d", &device.VRAMUsed)
			device.Capabilities.DriverVersion = strings.TrimSpace(parts[4])
			if len(parts) >= 6 {
				device.Capabilities.ComputeCapability = strings.TrimSpace(parts[5])
			}
			device.Capabilities.SupportsMPS = true // NVIDIA GPU支持MPS
			device.Vendor = GPUVendorNVIDIA
			device.UpdatedAt = time.Now()
		}
		dm.mu.Unlock()
	}
}

// enrichAMDInfo 使用rocm-smi补充AMD设备信息
func (dm *DeviceManager) enrichAMDInfo() {
	// 检查rocm-smi是否可用
	if _, err := exec.LookPath("rocm-smi"); err != nil {
		return
	}

	cmd := exec.Command("rocm-smi", "--showmeminfo", "vram", "--csv")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i, line := range lines {
		if i == 0 { // 跳过标题行
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		// AMD GPU的PCI地址通常在第一列
		_ = parts // 解析AMD格式（取决于rocm-smi版本）
	}
}

// normalizePCIAddr 标准化PCI地址格式
// nvidia-smi返回 00000000:01:00.0，sysfs使用 0000:01:00.0
func (dm *DeviceManager) normalizePCIAddr(addr string) string {
	// 去掉前导零，保留标准格式
	addr = strings.TrimSpace(addr)
	if len(addr) > 12 {
		// 00000000:01:00.0 -> 0000:01:00.0
		addr = addr[len(addr)-12:]
	}
	return addr
}

// GetDevice 获取指定GPU设备
func (dm *DeviceManager) GetDevice(pciAddr string) (*GPUDevice, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	device, exists := dm.devices[pciAddr]
	if !exists {
		return nil, fmt.Errorf("GPU设备不存在: %s", pciAddr)
	}
	return device, nil
}

// ListDevices 列出所有GPU设备
func (dm *DeviceManager) ListDevices() []*GPUDevice {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	devices := make([]*GPUDevice, 0, len(dm.devices))
	for _, d := range dm.devices {
		devices = append(devices, d)
	}
	return devices
}

// ListAvailableDevices 列出可用GPU设备
func (dm *DeviceManager) ListAvailableDevices() []*GPUDevice {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	devices := make([]*GPUDevice, 0)
	for _, d := range dm.devices {
		if d.Available && len(d.Assignments) == 0 {
			devices = append(devices, d)
		}
	}
	return devices
}

// GetDeviceForContainer 获取容器的GPU设备列表
func (dm *DeviceManager) GetDeviceForContainer(containerID string) []*GPUDevice {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make([]*GPUDevice, 0)
	for _, d := range dm.devices {
		for _, assign := range d.Assignments {
			if assign.ContainerID == containerID && assign.State == AssignmentStateActive {
				result = append(result, d)
				break
			}
		}
	}
	return result
}

// UpdateDeviceAssignment 更新设备分配信息
func (dm *DeviceManager) UpdateDeviceAssignment(pciAddr string, assignment *LXCGPUAssignment) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	device, exists := dm.devices[pciAddr]
	if !exists {
		return fmt.Errorf("GPU设备不存在: %s", pciAddr)
	}

	// 查找并更新或追加分配记录
	found := false
	for i, a := range device.Assignments {
		if a.ID == assignment.ID {
			device.Assignments[i] = assignment
			found = true
			break
		}
	}
	if !found {
		device.Assignments = append(device.Assignments, assignment)
	}

	device.UpdatedAt = time.Now()
	return nil
}

// RemoveDeviceAssignment 移除设备分配记录
func (dm *DeviceManager) RemoveDeviceAssignment(pciAddr, assignmentID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	device, exists := dm.devices[pciAddr]
	if !exists {
		return fmt.Errorf("GPU设备不存在: %s", pciAddr)
	}

	for i, a := range device.Assignments {
		if a.ID == assignmentID {
			device.Assignments = append(device.Assignments[:i], device.Assignments[i+1:]...)
			device.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("分配记录不存在: %s", assignmentID)
}

// StartPolling 启动设备状态轮询
// 定期检测GPU温度、使用率等信息
func (dm *DeviceManager) StartPolling(interval time.Duration) {
	if dm.polling {
		return
	}
	dm.polling = true

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				dm.pollGPUStats()
			case <-dm.stopCh:
				return
			}
		}
	}()
}

// StopPolling 停止设备状态轮询
func (dm *DeviceManager) StopPolling() {
	if dm.polling {
		close(dm.stopCh)
		dm.polling = false
	}
}

// pollGPUStats 轮询GPU统计信息（温度、使用率等）
func (dm *DeviceManager) pollGPUStats() {
	// NVIDIA设备：使用nvidia-smi查询
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		cmd := exec.Command("nvidia-smi", "--query-gpu=pci.bus_id,temperature.gpu,utilization.gpu,memory.used",
			"--format=csv,noheader,nounits")
		output, err := cmd.Output()
		if err != nil {
			return
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		dm.mu.Lock()
		for _, line := range lines {
			parts := strings.SplitN(line, ",", 4)
			if len(parts) < 4 {
				continue
			}
			pciAddr := dm.normalizePCIAddr(strings.TrimSpace(parts[0]))
			if device, exists := dm.devices[pciAddr]; exists {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &device.Temperature)
				fmt.Sscanf(strings.TrimSpace(parts[3]), "%d", &device.VRAMUsed)
				device.UpdatedAt = time.Now()
			}
		}
		dm.mu.Unlock()
	}
}

// GetContainerGPUs 获取容器已分配的GPU设备ID列表
func (dm *DeviceManager) GetContainerGPUs(containerID string) []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	gpuAddrs := make([]string, 0)
	for _, d := range dm.devices {
		for _, assign := range d.Assignments {
			if assign.ContainerID == containerID {
				gpuAddrs = append(gpuAddrs, d.PCIAddress)
				break
			}
		}
	}
	return gpuAddrs
}

// IsDeviceAvailable 检查设备是否可用于指定容器
func (dm *DeviceManager) IsDeviceAvailable(pciAddr string, shareMode ShareMode) (bool, string) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	device, exists := dm.devices[pciAddr]
	if !exists {
		return false, "GPU设备不存在"
	}
	if !device.Available {
		return false, "GPU设备不可用"
	}

	// 独占模式：必须无任何分配
	if shareMode == ShareModeExclusive || shareMode == "" {
		if len(device.Assignments) > 0 {
			return false, "GPU已被占用，无法独占分配"
		}
	}

	// MPS模式：检查是否超过最大实例数
	if shareMode == ShareModeMPS && device.Capabilities.MaxInstances > 0 {
		activeCount := 0
		for _, a := range device.Assignments {
			if a.State == AssignmentStateActive {
				activeCount++
			}
		}
		if activeCount >= device.Capabilities.MaxInstances {
			return false, fmt.Sprintf("GPU MPS实例已满(%d/%d)", activeCount, device.Capabilities.MaxInstances)
		}
	}

	return true, ""
}
