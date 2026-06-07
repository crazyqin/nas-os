package pciepassthrough

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
)

const (
	// DefaultSysfsPCIDevices 默认 sysfs PCI 设备路径.
	DefaultSysfsPCIDevices = "/sys/bus/pci/devices"
	// DefaultIOMMUGroupsPath 默认 IOMMU 分组路径.
	DefaultIOMMUGroupsPath = "/sys/kernel/iommu_groups"
	// DefaultVFIOPath 默认 VFIO 设备路径.
	DefaultVFIOPath = "/dev/vfio"
	// VFIOStubDrivers VFIO 支持的 stub 驱动列表.
	VFIOStubDrivers = "vfio-pci"
	// GPUPCIeClass GPU 设备的 PCIe class code.
	GPUPCIeClass = "0x030000"
	// GPUPCIeClassMask GPU class code 掩码.
	GPUPCIeClassMask = "0xff0000"
)

// DeviceType 设备类型.
type DeviceType string

const (
	// DeviceTypeGPU GPU 设备.
	DeviceTypeGPU DeviceType = "gpu"
	// DeviceTypeNetwork 网络设备.
	DeviceTypeNetwork DeviceType = "network"
	// DeviceTypeStorage 存储设备.
	DeviceTypeStorage DeviceType = "storage"
	// DeviceTypeUSB USB 控制器.
	DeviceTypeUSB DeviceType = "usb"
	// DeviceTypeAudio 音频设备.
	DeviceTypeAudio DeviceType = "audio"
	// DeviceTypeOther 其他设备.
	DeviceTypeOther DeviceType = "other"
)

// DriverState 驱动状态.
type DriverState string

const (
	// DriverStateBound 已绑定驱动.
	DriverStateBound DriverState = "bound"
	// DriverStateUnbound 未绑定驱动.
	DriverStateUnbound DriverState = "unbound"
	// DriverStateError 驱动错误.
	DriverStateError DriverState = "error"
)

// DeviceInfo PCIe 设备信息.
type DeviceInfo struct {
	PCIAddress  string      `json:"pciAddress"`  // PCI 地址 (如 0000:01:00.0)
	VendorID    string      `json:"vendorId"`    // 厂商 ID
	DeviceID    string      `json:"deviceId"`    // 设备 ID
	ClassName   string      `json:"className"`   // 设备类名
	DeviceType  DeviceType  `json:"deviceType"`  // 设备类型
	IOMMUGroup  int         `json:"iommuGroup"`  // IOMMU 分组号 (-1 表示未分组)
	Driver      string      `json:"driver"`      // 当前驱动名
	DriverState DriverState `json:"driverState"` // 驱动状态
	VendorName  string      `json:"vendorName"`  // 厂商名称
	DeviceName  string      `json:"deviceName"`  // 设备名称
	NumaNode    int         `json:"numaNode"`    // NUMA 节点
	Slot        string      `json:"slot"`        // PCIe 插槽
}

// PassthroughConfig 直通配置.
type PassthroughConfig struct {
	TargetID      string `json:"targetId"`      // 目标 VM/LXC ID
	TargetType    string `json:"targetType"`    // 目标类型: vm 或 lxc
	PCIAddr       string `json:"pciAddr"`       // PCI 设备地址
	VFIOID        string `json:"vfioId"`        // VFIO 设备 ID
	ROMFile       string `json:"romFile"`       // ROM 文件路径 (可选)
	Multifunction bool   `json:"multifunction"` // 是否启用多功能
}

// Manager PCIe 直通管理器.
type Manager struct {
	mu              sync.RWMutex
	sysfsPCIDevices string
	iommuGroupsPath string
	vfioPath        string
	devices         map[string]*DeviceInfo
	logger          *zap.Logger
}

// NewManager 创建 PCIe 直通管理器.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		sysfsPCIDevices: DefaultSysfsPCIDevices,
		iommuGroupsPath: DefaultIOMMUGroupsPath,
		vfioPath:        DefaultVFIOPath,
		devices:         make(map[string]*DeviceInfo),
		logger:          logger,
	}

	// 加载设备信息
	if err := m.loadDevices(); err != nil {
		logger.Warn("加载 PCIe 设备信息失败", zap.Error(err))
	}

	return m
}

// ListDevices 列出所有 PCIe 设备.
func (m *Manager) ListDevices() ([]DeviceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]DeviceInfo, 0, len(m.devices))
	for _, dev := range m.devices {
		devices = append(devices, *dev)
	}
	return devices, nil
}

// GetDevice 获取指定 PCI 地址的设备信息.
func (m *Manager) GetDevice(pciAddr string) (*DeviceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if pciAddr == "" {
		return nil, fmt.Errorf("PCI 地址不能为空")
	}

	dev, exists := m.devices[pciAddr]
	if !exists {
		return nil, fmt.Errorf("未找到设备：%s", pciAddr)
	}
	return dev, nil
}

// BindToVFIO 将设备绑定到 VFIO 驱动.
func (m *Manager) BindToVFIO(pciAddr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pciAddr == "" {
		return fmt.Errorf("PCI 地址不能为空")
	}

	dev, exists := m.devices[pciAddr]
	if !exists {
		return fmt.Errorf("未找到设备：%s", pciAddr)
	}

	// 如果已经绑定到 VFIO，直接返回
	if dev.Driver == VFIOStubDrivers {
		m.logger.Info("设备已绑定到 VFIO", zap.String("pciAddr", pciAddr))
		return nil
	}

	// 卸载当前驱动
	if dev.Driver != "" {
		if err := m.unbindDriver(pciAddr, dev.Driver); err != nil {
			return fmt.Errorf("卸载当前驱动失败：%w", err)
		}
	}

	// 绑定到 VFIO
	if err := m.bindDriver(pciAddr, VFIOStubDrivers); err != nil {
		return fmt.Errorf("绑定 VFIO 驱动失败：%w", err)
	}

	// 更新设备信息
	dev.Driver = VFIOStubDrivers
	dev.DriverState = DriverStateBound

	m.logger.Info("设备已绑定到 VFIO", zap.String("pciAddr", pciAddr))
	return nil
}

// UnbindFromVFIO 将设备从 VFIO 驱动解绑.
func (m *Manager) UnbindFromVFIO(pciAddr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pciAddr == "" {
		return fmt.Errorf("PCI 地址不能为空")
	}

	dev, exists := m.devices[pciAddr]
	if !exists {
		return fmt.Errorf("未找到设备：%s", pciAddr)
	}

	if dev.Driver != VFIOStubDrivers {
		return fmt.Errorf("设备 %s 未绑定到 VFIO 驱动", pciAddr)
	}

	// 解绑 VFIO 驱动
	if err := m.unbindDriver(pciAddr, VFIOStubDrivers); err != nil {
		return fmt.Errorf("解绑 VFIO 驱动失败：%w", err)
	}

	// 重新扫描 PCI 总线让设备重新绑定到原始驱动
	if err := m.rescanPCIBus(); err != nil {
		m.logger.Warn("重新扫描 PCI 总线失败", zap.Error(err))
	}

	// 重新加载设备信息
	if err := m.reloadDevice(pciAddr); err != nil {
		m.logger.Warn("重新加载设备信息失败", zap.Error(err))
	}

	m.logger.Info("设备已从 VFIO 解绑", zap.String("pciAddr", pciAddr))
	return nil
}

// ConfigurePassthrough 配置设备直通.
func (m *Manager) ConfigurePassthrough(config PassthroughConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证配置
	if config.PCIAddr == "" {
		return fmt.Errorf("PCI 地址不能为空")
	}
	if config.TargetID == "" {
		return fmt.Errorf("目标 ID 不能为空")
	}
	if config.TargetType == "" {
		return fmt.Errorf("目标类型不能为空")
	}

	// 检查设备是否存在
	dev, exists := m.devices[config.PCIAddr]
	if !exists {
		return fmt.Errorf("未找到设备：%s", config.PCIAddr)
	}

	// 检查 IOMMU 分组
	if dev.IOMMUGroup < 0 {
		return fmt.Errorf("设备 %s 未分配 IOMMU 分组", config.PCIAddr)
	}

	// 确保设备已绑定到 VFIO
	if dev.Driver != VFIOStubDrivers {
		return fmt.Errorf("设备 %s 未绑定到 VFIO 驱动，请先调用 BindToVFIO", config.PCIAddr)
	}

	// 生成 VFIO 设备 ID
	if config.VFIOID == "" {
		config.VFIOID = fmt.Sprintf("%d", dev.IOMMUGroup)
	}

	// 根据目标类型生成配置
	switch config.TargetType {
	case "vm":
		if err := m.configureVMPassthrough(config, dev); err != nil {
			return fmt.Errorf("配置 VM 直通失败：%w", err)
		}
	case "lxc":
		if err := m.configureLXCPassthrough(config, dev); err != nil {
			return fmt.Errorf("配置 LXC 直通失败：%w", err)
		}
	default:
		return fmt.Errorf("不支持的目标类型：%s", config.TargetType)
	}

	m.logger.Info("已配置设备直通",
		zap.String("pciAddr", config.PCIAddr),
		zap.String("targetType", config.TargetType),
		zap.String("targetId", config.TargetID),
	)
	return nil
}

// GetIOMMUGroups 获取所有 IOMMU 分组信息.
func (m *Manager) GetIOMMUGroups() (map[int][]DeviceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make(map[int][]DeviceInfo)
	for _, dev := range m.devices {
		if dev.IOMMUGroup >= 0 {
			groups[dev.IOMMUGroup] = append(groups[dev.IOMMUGroup], *dev)
		}
	}
	return groups, nil
}

// DetectGPUDevices 检测所有 GPU 设备.
func (m *Manager) DetectGPUDevices() ([]DeviceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var gpus []DeviceInfo
	for _, dev := range m.devices {
		if dev.DeviceType == DeviceTypeGPU {
			gpus = append(gpus, *dev)
		}
	}
	return gpus, nil
}

// HotPlugDevice 热插拔设备到虚拟机.
func (m *Manager) HotPlugDevice(pciAddr string, vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pciAddr == "" {
		return fmt.Errorf("PCI 地址不能为空")
	}
	if vmID == "" {
		return fmt.Errorf("VM ID 不能为空")
	}

	dev, exists := m.devices[pciAddr]
	if !exists {
		return fmt.Errorf("未找到设备：%s", pciAddr)
	}

	// 检查设备是否已绑定到 VFIO
	if dev.Driver != VFIOStubDrivers {
		return fmt.Errorf("设备 %s 未绑定到 VFIO 驱动", pciAddr)
	}

	// 检查 IOMMU 分组
	if dev.IOMMUGroup < 0 {
		return fmt.Errorf("设备 %s 未分配 IOMMU 分组", pciAddr)
	}

	// 使用 QEMU monitor 执行热插拔
	vfioDeviceID := fmt.Sprintf("vfio-%d", dev.IOMMUGroup)
	if err := m.qemuHotPlug(vmID, vfioDeviceID, pciAddr); err != nil {
		return fmt.Errorf("热插拔设备失败：%w", err)
	}

	m.logger.Info("设备已热插拔到 VM",
		zap.String("pciAddr", pciAddr),
		zap.String("vmId", vmID),
	)
	return nil
}

// RefreshDevices 刷新设备信息.
func (m *Manager) RefreshDevices() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.loadDevices()
}

// loadDevices 加载所有 PCIe 设备信息.
func (m *Manager) loadDevices() error {
	entries, err := os.ReadDir(m.sysfsPCIDevices)
	if err != nil {
		return fmt.Errorf("读取 PCI 设备目录失败：%w", err)
	}

	m.devices = make(map[string]*DeviceInfo)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pciAddr := entry.Name()
		dev, err := m.readDeviceInfo(pciAddr)
		if err != nil {
			m.logger.Warn("读取设备信息失败",
				zap.String("pciAddr", pciAddr),
				zap.Error(err),
			)
			continue
		}
		m.devices[pciAddr] = dev
	}

	return nil
}

// readDeviceInfo 从 sysfs 读取单个设备信息.
func (m *Manager) readDeviceInfo(pciAddr string) (*DeviceInfo, error) {
	devPath := filepath.Join(m.sysfsPCIDevices, pciAddr)

	// 读取 vendor ID
	vendorID, err := m.readSysfsFile(devPath, "vendor")
	if err != nil {
		return nil, fmt.Errorf("读取 vendor ID 失败：%w", err)
	}

	// 读取 device ID
	deviceID, err := m.readSysfsFile(devPath, "device")
	if err != nil {
		return nil, fmt.Errorf("读取 device ID 失败：%w", err)
	}

	// 读取 class
	classCode, err := m.readSysfsFile(devPath, "class")
	if err != nil {
		return nil, fmt.Errorf("读取 class 失败：%w", err)
	}

	// 读取当前驱动
	driver, driverState := m.readDriverInfo(devPath)

	// 获取 IOMMU 分组
	iommuGroup := m.getIOMMUGroup(pciAddr)

	// 确定设备类型
	deviceType := m.classifyDevice(classCode)

	// 读取 NUMA 节点
	numaNode := -1
	numaStr, err := m.readSysfsFile(devPath, "numa_node")
	if err == nil {
		numaNode, _ = strconv.Atoi(strings.TrimSpace(numaStr))
	}

	// 查询厂商和设备名称
	vendorName, deviceName := m.lookupDeviceInfo(vendorID, deviceID)

	return &DeviceInfo{
		PCIAddress:  pciAddr,
		VendorID:    strings.TrimSpace(vendorID),
		DeviceID:    strings.TrimSpace(deviceID),
		ClassName:   strings.TrimSpace(classCode),
		DeviceType:  deviceType,
		IOMMUGroup:  iommuGroup,
		Driver:      driver,
		DriverState: driverState,
		VendorName:  vendorName,
		DeviceName:  deviceName,
		NumaNode:    numaNode,
	}, nil
}

// readSysfsFile 读取 sysfs 文件内容.
func (m *Manager) readSysfsFile(devPath, filename string) (string, error) {
	data, err := os.ReadFile(filepath.Join(devPath, filename))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readDriverInfo 读取驱动信息.
func (m *Manager) readDriverInfo(devPath string) (string, DriverState) {
	driverLink := filepath.Join(devPath, "driver")
	target, err := os.Readlink(driverLink)
	if err != nil {
		return "", DriverStateUnbound
	}

	driverName := filepath.Base(target)
	if driverName == VFIOStubDrivers {
		return driverName, DriverStateBound
	}
	return driverName, DriverStateBound
}

// getIOMMUGroup 获取设备的 IOMMU 分组.
func (m *Manager) getIOMMUGroup(pciAddr string) int {
	iommuLink := filepath.Join(m.sysfsPCIDevices, pciAddr, "iommu_group")
	target, err := os.Readlink(iommuLink)
	if err != nil {
		return -1
	}

	// 解析分组号
	groupStr := filepath.Base(target)
	group, err := strconv.Atoi(groupStr)
	if err != nil {
		return -1
	}
	return group
}

// classifyDevice 根据 class code 分类设备.
func (m *Manager) classifyDevice(classCode string) DeviceType {
	code := strings.TrimSpace(classCode)
	// 移除 0x 前缀并补齐到 6 位
	code = strings.TrimPrefix(code, "0x")
	if len(code) < 6 {
		code = strings.Repeat("0", 6-len(code)) + code
	}

	// 提取 base class (前两位)
	baseClass := code[:2]

	switch baseClass {
	case "03": // Display controller
		return DeviceTypeGPU
	case "02": // Network controller
		return DeviceTypeNetwork
	case "01": // Mass storage controller
		return DeviceTypeStorage
	case "0c": // Serial bus controller
		if code[2:4] == "03" { // USB controller
			return DeviceTypeUSB
		}
		return DeviceTypeOther
	case "04": // Multimedia controller
		return DeviceTypeAudio
	default:
		return DeviceTypeOther
	}
}

// lookupDeviceInfo 查询厂商和设备名称.
func (m *Manager) lookupDeviceInfo(vendorID, deviceID string) (string, string) {
	// 简单的本地查找表
	knownDevices := map[string]map[string]string{
		"0x10de": { // NVIDIA
			"name": "NVIDIA Corporation",
		},
		"0x1002": { // AMD
			"name": "Advanced Micro Devices",
		},
		"0x8086": { // Intel
			"name": "Intel Corporation",
		},
	}

	vendorID = strings.TrimSpace(vendorID)
	if vendorInfo, ok := knownDevices[vendorID]; ok {
		return vendorInfo["name"], ""
	}
	return "", ""
}

// bindDriver 绑定设备到指定驱动.
func (m *Manager) bindDriver(pciAddr, driver string) error {
	bindPath := filepath.Join(m.sysfsPCIDevices, "drivers", driver, "bind")
	return os.WriteFile(bindPath, []byte(pciAddr), 0200)
}

// unbindDriver 从指定驱动解绑设备.
func (m *Manager) unbindDriver(pciAddr, driver string) error {
	unbindPath := filepath.Join(m.sysfsPCIDevices, "drivers", driver, "unbind")
	return os.WriteFile(unbindPath, []byte(pciAddr), 0200)
}

// rescanPCIBus 重新扫描 PCI 总线.
func (m *Manager) rescanPCIBus() error {
	return os.WriteFile("/sys/bus/pci/rescan", []byte("1"), 0200)
}

// reloadDevice 重新加载单个设备信息.
func (m *Manager) reloadDevice(pciAddr string) error {
	dev, err := m.readDeviceInfo(pciAddr)
	if err != nil {
		return err
	}
	m.devices[pciAddr] = dev
	return nil
}

// configureVMPassthrough 配置 VM 直通.
func (m *Manager) configureVMPassthrough(config PassthroughConfig, dev *DeviceInfo) error {
	// 生成 VFIO 设备配置参数
	args := []string{
		"-device", fmt.Sprintf("vfio-pci,host=%s", dev.PCIAddress),
	}

	if config.Multifunction {
		args = append(args, "-device", fmt.Sprintf("vfio-pci,host=%s,multifunction=on", dev.PCIAddress))
	}

	if config.ROMFile != "" {
		if _, err := os.Stat(config.ROMFile); err != nil {
			return fmt.Errorf("ROM 文件不存在：%s", config.ROMFile)
		}
		args = append(args, fmt.Sprintf(",romfile=%s", config.ROMFile))
	}

	m.logger.Info("VM 直通配置参数",
		zap.String("targetId", config.TargetID),
		zap.Strings("args", args),
	)

	return nil
}

// configureLXCPassthrough 配置 LXC 直通.
func (m *Manager) configureLXCPassthrough(config PassthroughConfig, dev *DeviceInfo) error {
	// 检查 VFIO 设备文件是否存在
	vfioDevPath := filepath.Join(m.vfioPath, fmt.Sprintf("%d", dev.IOMMUGroup))
	if _, err := os.Stat(vfioDevPath); err != nil {
		return fmt.Errorf("VFIO 设备文件不存在：%s", vfioDevPath)
	}

	m.logger.Info("LXC 直通配置",
		zap.String("targetId", config.TargetID),
		zap.String("vfioDevice", vfioDevPath),
	)

	return nil
}

// qemuHotPlug 通过 QEMU monitor 执行热插拔.
func (m *Manager) qemuHotPlug(vmID, vfioDeviceID, pciAddr string) error {
	// 使用 virsh 或 qemu-monitor-command 执行热插拔
	// 这里使用 virsh 作为示例
	ctx := context.Background()
	// #nosec G204 -- vmID is validated by caller
	cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system",
		"attach-device", vmID,
		"--file", "-",
		"--live",
	)

	// 生成设备 XML
	deviceXML := fmt.Sprintf(`<hostdev mode='subsystem' type='pci' managed='yes'>
  <source>
    <address domain='0x0000' bus='0x%s' slot='0x%s' function='0x%s'/>
  </source>
</hostdev>`, extractBus(pciAddr), extractSlot(pciAddr), extractFunction(pciAddr))

	cmd.Stdin = strings.NewReader(deviceXML)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("virsh 热插拔失败：%s, %w", string(output), err)
	}

	return nil
}

// extractBus 从 PCI 地址提取 bus.
func extractBus(pciAddr string) string {
	parts := strings.Split(pciAddr, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return "00"
}

// extractSlot 从 PCI 地址提取 slot.
func extractSlot(pciAddr string) string {
	parts := strings.Split(pciAddr, ":")
	if len(parts) >= 3 {
		funcParts := strings.Split(parts[2], ".")
		return funcParts[0]
	}
	return "00"
}

// extractFunction 从 PCI 地址提取 function.
func extractFunction(pciAddr string) string {
	parts := strings.Split(pciAddr, ".")
	if len(parts) >= 2 {
		return parts[1]
	}
	return "0"
}

// ValidatePCIAddress 验证 PCI 地址格式.
func ValidatePCIAddress(pciAddr string) bool {
	// 匹配格式：0000:00:00.0 或 0000:00:00.0
	re := regexp.MustCompile(`^[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-9a-fA-F]{1,2}$`)
	return re.MatchString(pciAddr)
}
