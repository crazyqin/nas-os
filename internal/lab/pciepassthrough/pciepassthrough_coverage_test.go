package pciepassthrough

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

// 创建模拟的 sysfs 设备目录结构.
func createMockSysfsDevice(t *testing.T, baseDir, pciAddr string, opts ...func(string)) {
	t.Helper()
	devPath := filepath.Join(baseDir, pciAddr)
	if err := os.MkdirAll(devPath, 0750); err != nil {
		t.Fatalf("failed to create mock device dir: %v", err)
	}

	// 默认文件
	defaults := map[string]string{
		"vendor": "0x10de\n",
		"device": "0x1234\n",
		"class":  "0x030000\n",
	}

	for filename, content := range defaults {
		if err := os.WriteFile(filepath.Join(devPath, filename), []byte(content), 0640); err != nil {
			t.Fatalf("failed to write mock file %s: %v", filename, err)
		}
	}

	// 应用自定义选项
	for _, opt := range opts {
		opt(devPath)
	}
}

// withDriver 设置驱动信息.
func withDriver(driverName string) func(string) {
	return func(devPath string) {
		driverDir := filepath.Join(devPath, "driver")
		if err := os.MkdirAll(driverDir, 0750); err != nil {
			return
		}
		// 创建符号链接指向驱动目录
		targetDir := filepath.Join(filepath.Dir(devPath), "drivers", driverName)
		os.MkdirAll(targetDir, 0750)
		os.Symlink(targetDir, driverDir)
	}
}

// withIOMMUGroup 设置 IOMMU 分组.
func withIOMMUGroup(group int) func(string) {
	return func(devPath string) {
		iommuDir := filepath.Join(devPath, "iommu_group")
		targetDir := filepath.Join(filepath.Dir(devPath), "iommu_groups", "1234")
		os.MkdirAll(targetDir, 0750)
		os.Symlink(targetDir, iommuDir)
	}
}

// withNumaNode 设置 NUMA 节点.
func withNumaNode(node int) func(string) {
	return func(devPath string) {
		content := []byte("0\n")
		if node >= 0 {
			content = []byte("0\n")
		}
		os.WriteFile(filepath.Join(devPath, "numa_node"), content, 0640)
	}
}

func TestManager_LoadDevices(t *testing.T) {
	tmpDir := t.TempDir()
	devicesDir := filepath.Join(tmpDir, "devices")

	// 创建模拟设备
	createMockSysfsDevice(t, devicesDir, "0000:01:00.0", withDriver("nvidia"), withIOMMUGroup(1))
	createMockSysfsDevice(t, devicesDir, "0000:02:00.0", withDriver("vfio-pci"), withIOMMUGroup(2))
	createMockSysfsDevice(t, devicesDir, "0000:03:00.0") // 无驱动，无 IOMMU

	manager := &Manager{
		sysfsPCIDevices: devicesDir,
		iommuGroupsPath: filepath.Join(tmpDir, "iommu_groups"),
		vfioPath:        filepath.Join(tmpDir, "vfio"),
		devices:         make(map[string]*DeviceInfo),
		logger:          zap.NewNop(),
	}

	// 测试 loadDevices
	if err := manager.loadDevices(); err != nil {
		t.Fatalf("loadDevices failed: %v", err)
	}

	devices, _ := manager.ListDevices()
	if len(devices) != 3 {
		t.Errorf("expected 3 devices, got %d", len(devices))
	}
}

func TestManager_ReadDeviceInfo(t *testing.T) {
	tmpDir := t.TempDir()
	devicesDir := filepath.Join(tmpDir, "devices")

	createMockSysfsDevice(t, devicesDir, "0000:01:00.0", withDriver("nvidia"), withIOMMUGroup(1))

	manager := &Manager{
		sysfsPCIDevices: devicesDir,
		iommuGroupsPath: filepath.Join(tmpDir, "iommu_groups"),
		vfioPath:        filepath.Join(tmpDir, "vfio"),
		devices:         make(map[string]*DeviceInfo),
		logger:          zap.NewNop(),
	}

	dev, err := manager.readDeviceInfo("0000:01:00.0")
	if err != nil {
		t.Fatalf("readDeviceInfo failed: %v", err)
	}

	if dev.VendorID != "0x10de" {
		t.Errorf("expected VendorID = 0x10de, got %s", dev.VendorID)
	}
	if dev.DeviceID != "0x1234" {
		t.Errorf("expected DeviceID = 0x1234, got %s", dev.DeviceID)
	}
	if dev.DeviceType != DeviceTypeGPU {
		t.Errorf("expected DeviceType = gpu, got %s", dev.DeviceType)
	}
}

func TestManager_ReadDeviceInfo_NoDriver(t *testing.T) {
	tmpDir := t.TempDir()
	devicesDir := filepath.Join(tmpDir, "devices")

	createMockSysfsDevice(t, devicesDir, "0000:01:00.0")

	manager := &Manager{
		sysfsPCIDevices: devicesDir,
		iommuGroupsPath: filepath.Join(tmpDir, "iommu_groups"),
		vfioPath:        filepath.Join(tmpDir, "vfio"),
		devices:         make(map[string]*DeviceInfo),
		logger:          zap.NewNop(),
	}

	dev, err := manager.readDeviceInfo("0000:01:00.0")
	if err != nil {
		t.Fatalf("readDeviceInfo failed: %v", err)
	}

	if dev.DriverState != DriverStateUnbound {
		t.Errorf("expected DriverState = unbound, got %s", dev.DriverState)
	}
	if dev.Driver != "" {
		t.Errorf("expected Driver = empty, got %s", dev.Driver)
	}
}

func TestManager_ReadDeviceInfo_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	devicesDir := filepath.Join(tmpDir, "devices")

	// 创建目录但不创建文件
	devPath := filepath.Join(devicesDir, "0000:01:00.0")
	os.MkdirAll(devPath, 0750)

	manager := &Manager{
		sysfsPCIDevices: devicesDir,
		iommuGroupsPath: filepath.Join(tmpDir, "iommu_groups"),
		vfioPath:        filepath.Join(tmpDir, "vfio"),
		devices:         make(map[string]*DeviceInfo),
		logger:          zap.NewNop(),
	}

	_, err := manager.readDeviceInfo("0000:01:00.0")
	if err == nil {
		t.Error("readDeviceInfo should fail when vendor file missing")
	}
}

func TestManager_ClassifyDevice_AllTypes(t *testing.T) {
	manager := &Manager{logger: zap.NewNop()}

	tests := []struct {
		classCode string
		expected  DeviceType
	}{
		{"0x030000", DeviceTypeGPU},
		{"0x030200", DeviceTypeGPU},
		{"0x020000", DeviceTypeNetwork},
		{"0x020100", DeviceTypeNetwork},
		{"0x010000", DeviceTypeStorage},
		{"0x010802", DeviceTypeStorage},
		{"0x0c0300", DeviceTypeUSB},
		{"0x0c0330", DeviceTypeUSB},
		{"0x040000", DeviceTypeAudio},
		{"0x040300", DeviceTypeAudio},
		{"0x060000", DeviceTypeOther},
		{"0x080000", DeviceTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.classCode, func(t *testing.T) {
			result := manager.classifyDevice(tt.classCode)
			if result != tt.expected {
				t.Errorf("classifyDevice(%s) = %v, expected %v", tt.classCode, result, tt.expected)
			}
		})
	}
}

func TestManager_ClassifyDevice_ShortCodes(t *testing.T) {
	manager := &Manager{logger: zap.NewNop()}

	tests := []struct {
		classCode string
		expected  DeviceType
	}{
		{"30000", DeviceTypeGPU}, // 5位，补0后为030000
		{"20000", DeviceTypeNetwork},
		{"10000", DeviceTypeStorage},
		{"c0300", DeviceTypeUSB},
		{"40000", DeviceTypeAudio},
	}

	for _, tt := range tests {
		t.Run(tt.classCode, func(t *testing.T) {
			result := manager.classifyDevice(tt.classCode)
			if result != tt.expected {
				t.Errorf("classifyDevice(%s) = %v, expected %v", tt.classCode, result, tt.expected)
			}
		})
	}
}

func TestManager_RefreshDevices(t *testing.T) {
	tmpDir := t.TempDir()
	devicesDir := filepath.Join(tmpDir, "devices")

	createMockSysfsDevice(t, devicesDir, "0000:01:00.0", withDriver("nvidia"))

	manager := &Manager{
		sysfsPCIDevices: devicesDir,
		iommuGroupsPath: filepath.Join(tmpDir, "iommu_groups"),
		vfioPath:        filepath.Join(tmpDir, "vfio"),
		devices:         make(map[string]*DeviceInfo),
		logger:          zap.NewNop(),
	}

	// 初始加载
	if err := manager.loadDevices(); err != nil {
		t.Fatalf("initial loadDevices failed: %v", err)
	}

	devices, _ := manager.ListDevices()
	if len(devices) != 1 {
		t.Fatalf("expected 1 device initially, got %d", len(devices))
	}

	// 添加新设备
	createMockSysfsDevice(t, devicesDir, "0000:02:00.0", withDriver("vfio-pci"))

	// 刷新
	if err := manager.RefreshDevices(); err != nil {
		t.Fatalf("RefreshDevices failed: %v", err)
	}

	devices, _ = manager.ListDevices()
	if len(devices) != 2 {
		t.Errorf("expected 2 devices after refresh, got %d", len(devices))
	}
}

func TestManager_ReadSysfsFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test")
	os.WriteFile(testFile, []byte("test content\n"), 0640)

	manager := &Manager{logger: zap.NewNop()}

	content, err := manager.readSysfsFile(tmpDir, "test")
	if err != nil {
		t.Fatalf("readSysfsFile failed: %v", err)
	}
	if content != "test content\n" {
		t.Errorf("expected 'test content\\n', got '%s'", content)
	}

	// 测试不存在的文件
	_, err = manager.readSysfsFile(tmpDir, "nonexistent")
	if err == nil {
		t.Error("readSysfsFile should fail for non-existent file")
	}
}

func TestManager_ReadDriverInfo(t *testing.T) {
	tmpDir := t.TempDir()
	devPath := filepath.Join(tmpDir, "device")
	driverDir := filepath.Join(devPath, "driver")
	targetDir := filepath.Join(tmpDir, "drivers", "nvidia")

	os.MkdirAll(targetDir, 0750)
	os.MkdirAll(devPath, 0750)
	os.Symlink(targetDir, driverDir)

	manager := &Manager{logger: zap.NewNop()}

	driver, state := manager.readDriverInfo(devPath)
	if driver != "nvidia" {
		t.Errorf("expected driver = nvidia, got %s", driver)
	}
	if state != DriverStateBound {
		t.Errorf("expected state = bound, got %s", state)
	}

	// 测试无驱动的情况
	devPath2 := filepath.Join(tmpDir, "device2")
	os.MkdirAll(devPath2, 0750)

	driver2, state2 := manager.readDriverInfo(devPath2)
	if driver2 != "" {
		t.Errorf("expected driver = empty, got %s", driver2)
	}
	if state2 != DriverStateUnbound {
		t.Errorf("expected state = unbound, got %s", state2)
	}
}

func TestManager_GetIOMMUGroup(t *testing.T) {
	tmpDir := t.TempDir()
	devicesDir := filepath.Join(tmpDir, "devices")
	iommuGroupsDir := filepath.Join(tmpDir, "iommu_groups")

	// 创建带 IOMMU 分组的设备
	devPath := filepath.Join(devicesDir, "0000:01:00.0")
	os.MkdirAll(devPath, 0750)
	groupPath := filepath.Join(iommuGroupsDir, "42")
	os.MkdirAll(groupPath, 0750)
	os.Symlink(groupPath, filepath.Join(devPath, "iommu_group"))

	// 创建无 IOMMU 分组的设备
	devPath2 := filepath.Join(devicesDir, "0000:02:00.0")
	os.MkdirAll(devPath2, 0750)

	manager := &Manager{
		sysfsPCIDevices: devicesDir,
		logger:          zap.NewNop(),
	}

	// 有 IOMMU 分组
	group := manager.getIOMMUGroup("0000:01:00.0")
	if group != 42 {
		t.Errorf("expected IOMMU group 42, got %d", group)
	}

	// 无 IOMMU 分组
	group = manager.getIOMMUGroup("0000:02:00.0")
	if group != -1 {
		t.Errorf("expected IOMMU group -1, got %d", group)
	}
}

func TestManager_ClassifyDevice_NetworkTypes(t *testing.T) {
	manager := &Manager{logger: zap.NewNop()}

	// 测试以太网
	if result := manager.classifyDevice("0x020000"); result != DeviceTypeNetwork {
		t.Errorf("expected network, got %s", result)
	}

	// 测试无线网络
	if result := manager.classifyDevice("0x028000"); result != DeviceTypeNetwork {
		t.Errorf("expected network, got %s", result)
	}
}

func TestManager_ClassifyDevice_SerialBus(t *testing.T) {
	manager := &Manager{logger: zap.NewNop()}

	// USB 控制器
	if result := manager.classifyDevice("0x0c0300"); result != DeviceTypeUSB {
		t.Errorf("expected USB, got %s", result)
	}

	// 非 USB 的串行总线
	if result := manager.classifyDevice("0x0c0000"); result != DeviceTypeOther {
		t.Errorf("expected other, got %s", result)
	}
}

func TestManager_LookupDeviceInfo_AllVendors(t *testing.T) {
	manager := &Manager{logger: zap.NewNop()}

	// NVIDIA
	name, _ := manager.lookupDeviceInfo("0x10de", "0x1234")
	if name != "NVIDIA Corporation" {
		t.Errorf("expected NVIDIA Corporation, got %s", name)
	}

	// AMD
	name, _ = manager.lookupDeviceInfo("0x1002", "0x5678")
	if name != "Advanced Micro Devices" {
		t.Errorf("expected Advanced Micro Devices, got %s", name)
	}

	// Intel
	name, _ = manager.lookupDeviceInfo("0x8086", "0x9abc")
	if name != "Intel Corporation" {
		t.Errorf("expected Intel Corporation, got %s", name)
	}

	// 未知厂商
	name, _ = manager.lookupDeviceInfo("0x9999", "0x1234")
	if name != "" {
		t.Errorf("expected empty name for unknown vendor, got %s", name)
	}
}

func TestManager_ConfigureVMPassthrough(t *testing.T) {
	tmpDir := t.TempDir()
	romFile := filepath.Join(tmpDir, "vbios.rom")
	os.WriteFile(romFile, []byte("test rom"), 0640)

	manager := &Manager{logger: zap.NewNop()}
	dev := &DeviceInfo{
		PCIAddress: "0000:01:00.0",
		IOMMUGroup: 1,
	}

	// 测试基本配置
	config := PassthroughConfig{
		TargetID:   "100",
		TargetType: "vm",
		PCIAddr:    "0000:01:00.0",
		VFIOID:     "1",
	}

	if err := manager.configureVMPassthrough(config, dev); err != nil {
		t.Fatalf("configureVMPassthrough failed: %v", err)
	}

	// 测试带 ROM 文件的配置
	config.ROMFile = romFile
	if err := manager.configureVMPassthrough(config, dev); err != nil {
		t.Fatalf("configureVMPassthrough with ROM failed: %v", err)
	}

	// 测试带多功能的配置
	config.Multifunction = true
	if err := manager.configureVMPassthrough(config, dev); err != nil {
		t.Fatalf("configureVMPassthrough with multifunction failed: %v", err)
	}

	// 测试不存在的 ROM 文件
	config.ROMFile = "/nonexistent/path/rom.rom"
	if err := manager.configureVMPassthrough(config, dev); err == nil {
		t.Error("configureVMPassthrough should fail with non-existent ROM file")
	}
}

func TestManager_ConfigureLXCPassthrough(t *testing.T) {
	tmpDir := t.TempDir()
	vfioPath := filepath.Join(tmpDir, "vfio")

	// 创建 VFIO 设备文件
	vfioDevPath := filepath.Join(vfioPath, "1")
	os.MkdirAll(vfioDevPath, 0750)

	manager := &Manager{
		vfioPath: vfioPath,
		logger:   zap.NewNop(),
	}

	dev := &DeviceInfo{
		PCIAddress:  "0000:01:00.0",
		IOMMUGroup:  1,
		Driver:      VFIOStubDrivers,
		DriverState: DriverStateBound,
	}

	config := PassthroughConfig{
		TargetID:   "200",
		TargetType: "lxc",
		PCIAddr:    "0000:01:00.0",
	}

	// 测试正常配置
	if err := manager.configureLXCPassthrough(config, dev); err != nil {
		t.Fatalf("configureLXCPassthrough failed: %v", err)
	}

	// 测试不存在的 VFIO 设备
	manager.vfioPath = "/nonexistent/path"
	if err := manager.configureLXCPassthrough(config, dev); err == nil {
		t.Error("configureLXCPassthrough should fail with non-existent VFIO path")
	}
}

func TestManager_ConfigurePassthrough_VM(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress:  "0000:01:00.0",
				IOMMUGroup:  1,
				Driver:      VFIOStubDrivers,
				DriverState: DriverStateBound,
			},
		},
		logger: zap.NewNop(),
	}

	config := PassthroughConfig{
		TargetID:   "100",
		TargetType: "vm",
		PCIAddr:    "0000:01:00.0",
	}

	if err := manager.ConfigurePassthrough(config); err != nil {
		t.Fatalf("ConfigurePassthrough VM failed: %v", err)
	}
}

func TestManager_ConfigurePassthrough_LXC(t *testing.T) {
	tmpDir := t.TempDir()
	vfioPath := filepath.Join(tmpDir, "vfio")
	os.MkdirAll(filepath.Join(vfioPath, "1"), 0750)

	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress:  "0000:01:00.0",
				IOMMUGroup:  1,
				Driver:      VFIOStubDrivers,
				DriverState: DriverStateBound,
			},
		},
		vfioPath: vfioPath,
		logger:   zap.NewNop(),
	}

	config := PassthroughConfig{
		TargetID:   "200",
		TargetType: "lxc",
		PCIAddr:    "0000:01:00.0",
	}

	if err := manager.ConfigurePassthrough(config); err != nil {
		t.Fatalf("ConfigurePassthrough LXC failed: %v", err)
	}
}

func TestManager_ConfigurePassthrough_AutoVFIOID(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress:  "0000:01:00.0",
				IOMMUGroup:  42,
				Driver:      VFIOStubDrivers,
				DriverState: DriverStateBound,
			},
		},
		logger: zap.NewNop(),
	}

	config := PassthroughConfig{
		TargetID:   "100",
		TargetType: "vm",
		PCIAddr:    "0000:01:00.0",
		// VFIOID 为空，应自动生成
	}

	if err := manager.ConfigurePassthrough(config); err != nil {
		t.Fatalf("ConfigurePassthrough failed: %v", err)
	}
}

func TestManager_BindToVFIO_AlreadyBound(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress:  "0000:01:00.0",
				Driver:      VFIOStubDrivers,
				DriverState: DriverStateBound,
			},
		},
		logger: zap.NewNop(),
	}

	// 已经绑定到 VFIO，应该成功
	if err := manager.BindToVFIO("0000:01:00.0"); err != nil {
		t.Fatalf("BindToVFIO should succeed for already bound device: %v", err)
	}
}

func TestManager_ListDevices_WithMultipleDevices(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress: "0000:01:00.0",
				DeviceType: DeviceTypeGPU,
			},
			"0000:02:00.0": {
				PCIAddress: "0000:02:00.0",
				DeviceType: DeviceTypeNetwork,
			},
			"0000:03:00.0": {
				PCIAddress: "0000:03:00.0",
				DeviceType: DeviceTypeStorage,
			},
		},
		logger: zap.NewNop(),
	}

	devices, err := manager.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(devices) != 3 {
		t.Errorf("expected 3 devices, got %d", len(devices))
	}
}

func TestManager_GetIOMMUGroups_MultipleGroups(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {IOMMUGroup: 1},
			"0000:01:00.1": {IOMMUGroup: 1},
			"0000:01:00.2": {IOMMUGroup: 1},
			"0000:02:00.0": {IOMMUGroup: 2},
			"0000:03:00.0": {IOMMUGroup: 3},
			"0000:04:00.0": {IOMMUGroup: -1}, // 无分组
		},
		logger: zap.NewNop(),
	}

	groups, err := manager.GetIOMMUGroups()
	if err != nil {
		t.Fatalf("GetIOMMUGroups failed: %v", err)
	}

	if len(groups) != 3 {
		t.Errorf("expected 3 groups, got %d", len(groups))
	}
	if len(groups[1]) != 3 {
		t.Errorf("expected 3 devices in group 1, got %d", len(groups[1]))
	}
	if len(groups[2]) != 1 {
		t.Errorf("expected 1 device in group 2, got %d", len(groups[2]))
	}
	if len(groups[3]) != 1 {
		t.Errorf("expected 1 device in group 3, got %d", len(groups[3]))
	}
}

func TestManager_DetectGPUDevices_Mixed(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {DeviceType: DeviceTypeGPU, PCIAddress: "0000:01:00.0"},
			"0000:02:00.0": {DeviceType: DeviceTypeNetwork, PCIAddress: "0000:02:00.0"},
			"0000:03:00.0": {DeviceType: DeviceTypeGPU, PCIAddress: "0000:03:00.0"},
			"0000:04:00.0": {DeviceType: DeviceTypeStorage, PCIAddress: "0000:04:00.0"},
			"0000:05:00.0": {DeviceType: DeviceTypeGPU, PCIAddress: "0000:05:00.0"},
		},
		logger: zap.NewNop(),
	}

	gpus, err := manager.DetectGPUDevices()
	if err != nil {
		t.Fatalf("DetectGPUDevices failed: %v", err)
	}
	if len(gpus) != 3 {
		t.Errorf("expected 3 GPUs, got %d", len(gpus))
	}
}

func TestExtractBus_MoreCases(t *testing.T) {
	tests := []struct {
		pciAddr  string
		expected string
	}{
		{"0000:00:00.0", "00"},
		{"0000:ff:00.0", "ff"},
		{"0000:1a:00.0", "1a"},
		{"invalid", "00"},
	}

	for _, tt := range tests {
		t.Run(tt.pciAddr, func(t *testing.T) {
			result := extractBus(tt.pciAddr)
			if result != tt.expected {
				t.Errorf("extractBus(%s) = %s, expected %s", tt.pciAddr, result, tt.expected)
			}
		})
	}
}

func TestExtractSlot_MoreCases(t *testing.T) {
	tests := []struct {
		pciAddr  string
		expected string
	}{
		{"0000:00:00.0", "00"},
		{"0000:00:1f.0", "1f"},
		{"0000:00:ab.0", "ab"},
		{"invalid", "00"},
	}

	for _, tt := range tests {
		t.Run(tt.pciAddr, func(t *testing.T) {
			result := extractSlot(tt.pciAddr)
			if result != tt.expected {
				t.Errorf("extractSlot(%s) = %s, expected %s", tt.pciAddr, result, tt.expected)
			}
		})
	}
}

func TestExtractFunction_MoreCases(t *testing.T) {
	tests := []struct {
		pciAddr  string
		expected string
	}{
		{"0000:00:00.0", "0"},
		{"0000:00:00.7", "7"},
		{"0000:00:00.f", "f"},
		{"no-dot", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.pciAddr, func(t *testing.T) {
			result := extractFunction(tt.pciAddr)
			if result != tt.expected {
				t.Errorf("extractFunction(%s) = %s, expected %s", tt.pciAddr, result, tt.expected)
			}
		})
	}
}

func TestValidatePCIAddress_MoreCases(t *testing.T) {
	tests := []struct {
		pciAddr  string
		expected bool
	}{
		{"0000:00:00.0", true},
		{"ffff:ff:ff.f", true},
		{"0000:01:00.00", true},
		{"0000:01:00.", false},
		{"0000:01:00", false},
		{"0000:01:00.000", false}, // function 超过2位
		{"gggg:01:00.0", false},
		{"0000:gg:00.0", false},
		{"0000:01:gg.0", false},
		{"0000:01:00.g", false},
	}

	for _, tt := range tests {
		t.Run(tt.pciAddr, func(t *testing.T) {
			result := ValidatePCIAddress(tt.pciAddr)
			if result != tt.expected {
				t.Errorf("ValidatePCIAddress(%s) = %v, expected %v", tt.pciAddr, result, tt.expected)
			}
		})
	}
}
