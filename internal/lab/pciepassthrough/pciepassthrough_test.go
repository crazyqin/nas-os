package pciepassthrough

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
	if manager.devices == nil {
		t.Error("devices map should be initialized")
	}
	if manager.sysfsPCIDevices != DefaultSysfsPCIDevices {
		t.Errorf("expected sysfsPCIDevices = %s, got %s", DefaultSysfsPCIDevices, manager.sysfsPCIDevices)
	}
}

func TestNewManager_NilLogger(t *testing.T) {
	manager := NewManager(nil)

	if manager == nil {
		t.Fatal("Manager should not be nil even with nil logger")
	}
	if manager.logger == nil {
		t.Error("logger should be initialized even with nil input")
	}
}

func TestManager_ListDevices_Empty(t *testing.T) {
	// Create manager with custom temp path
	tmpDir := t.TempDir()
	devicesDir := filepath.Join(tmpDir, "devices")
	if err := os.MkdirAll(devicesDir, 0750); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	manager := &Manager{
		sysfsPCIDevices: devicesDir,
		iommuGroupsPath: filepath.Join(tmpDir, "iommu_groups"),
		vfioPath:        filepath.Join(tmpDir, "vfio"),
		devices:         make(map[string]*DeviceInfo),
		logger:          zap.NewNop(),
	}

	devices, err := manager.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
}

func TestManager_GetDevice_EmptyAddress(t *testing.T) {
	manager := NewManager(zap.NewNop())

	_, err := manager.GetDevice("")
	if err == nil {
		t.Error("GetDevice should return error for empty address")
	}
}

func TestManager_GetDevice_NotFound(t *testing.T) {
	manager := NewManager(zap.NewNop())

	_, err := manager.GetDevice("0000:01:00.0")
	if err == nil {
		t.Error("GetDevice should return error for non-existent device")
	}
}

func TestManager_GetDevice_Found(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress: "0000:01:00.0",
				VendorID:   "0x10de",
				DeviceID:   "0x1234",
				DeviceType: DeviceTypeGPU,
			},
		},
		logger: zap.NewNop(),
	}

	dev, err := manager.GetDevice("0000:01:00.0")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	if dev.PCIAddress != "0000:01:00.0" {
		t.Errorf("expected PCIAddress = 0000:01:00.0, got %s", dev.PCIAddress)
	}
	if dev.DeviceType != DeviceTypeGPU {
		t.Errorf("expected DeviceType = gpu, got %s", dev.DeviceType)
	}
}

func TestManager_BindToVFIO_EmptyAddress(t *testing.T) {
	manager := NewManager(zap.NewNop())

	err := manager.BindToVFIO("")
	if err == nil {
		t.Error("BindToVFIO should return error for empty address")
	}
}

func TestManager_BindToVFIO_DeviceNotFound(t *testing.T) {
	manager := NewManager(zap.NewNop())

	err := manager.BindToVFIO("0000:01:00.0")
	if err == nil {
		t.Error("BindToVFIO should return error for non-existent device")
	}
}

func TestManager_UnbindFromVFIO_EmptyAddress(t *testing.T) {
	manager := NewManager(zap.NewNop())

	err := manager.UnbindFromVFIO("")
	if err == nil {
		t.Error("UnbindFromVFIO should return error for empty address")
	}
}

func TestManager_UnbindFromVFIO_DeviceNotFound(t *testing.T) {
	manager := NewManager(zap.NewNop())

	err := manager.UnbindFromVFIO("0000:01:00.0")
	if err == nil {
		t.Error("UnbindFromVFIO should return error for non-existent device")
	}
}

func TestManager_UnbindFromVFIO_NotBoundToVFIO(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress:  "0000:01:00.0",
				Driver:      "nvidia",
				DriverState: DriverStateBound,
			},
		},
		logger: zap.NewNop(),
	}

	err := manager.UnbindFromVFIO("0000:01:00.0")
	if err == nil {
		t.Error("UnbindFromVFIO should return error when device not bound to VFIO")
	}
}

func TestManager_ConfigurePassthrough_EmptyPCIAddr(t *testing.T) {
	manager := NewManager(zap.NewNop())

	config := PassthroughConfig{
		TargetID:   "100",
		TargetType: "vm",
		PCIAddr:    "",
	}

	err := manager.ConfigurePassthrough(config)
	if err == nil {
		t.Error("ConfigurePassthrough should return error for empty PCI address")
	}
}

func TestManager_ConfigurePassthrough_EmptyTargetID(t *testing.T) {
	manager := NewManager(zap.NewNop())

	config := PassthroughConfig{
		TargetID:   "",
		TargetType: "vm",
		PCIAddr:    "0000:01:00.0",
	}

	err := manager.ConfigurePassthrough(config)
	if err == nil {
		t.Error("ConfigurePassthrough should return error for empty target ID")
	}
}

func TestManager_ConfigurePassthrough_EmptyTargetType(t *testing.T) {
	manager := NewManager(zap.NewNop())

	config := PassthroughConfig{
		TargetID:   "100",
		TargetType: "",
		PCIAddr:    "0000:01:00.0",
	}

	err := manager.ConfigurePassthrough(config)
	if err == nil {
		t.Error("ConfigurePassthrough should return error for empty target type")
	}
}

func TestManager_ConfigurePassthrough_DeviceNotFound(t *testing.T) {
	manager := NewManager(zap.NewNop())

	config := PassthroughConfig{
		TargetID:   "100",
		TargetType: "vm",
		PCIAddr:    "0000:01:00.0",
	}

	err := manager.ConfigurePassthrough(config)
	if err == nil {
		t.Error("ConfigurePassthrough should return error for non-existent device")
	}
}

func TestManager_ConfigurePassthrough_InvalidIOMMUGroup(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress: "0000:01:00.0",
				IOMMUGroup: -1,
			},
		},
		logger: zap.NewNop(),
	}

	config := PassthroughConfig{
		TargetID:   "100",
		TargetType: "vm",
		PCIAddr:    "0000:01:00.0",
	}

	err := manager.ConfigurePassthrough(config)
	if err == nil {
		t.Error("ConfigurePassthrough should return error for invalid IOMMU group")
	}
}

func TestManager_ConfigurePassthrough_NotBoundToVFIO(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress:  "0000:01:00.0",
				IOMMUGroup:  1,
				Driver:      "nvidia",
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

	err := manager.ConfigurePassthrough(config)
	if err == nil {
		t.Error("ConfigurePassthrough should return error when device not bound to VFIO")
	}
}

func TestManager_ConfigurePassthrough_UnsupportedTargetType(t *testing.T) {
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
		TargetType: "invalid",
		PCIAddr:    "0000:01:00.0",
	}

	err := manager.ConfigurePassthrough(config)
	if err == nil {
		t.Error("ConfigurePassthrough should return error for unsupported target type")
	}
}

func TestManager_GetIOMMUGroups_Empty(t *testing.T) {
	manager := &Manager{
		devices: make(map[string]*DeviceInfo),
		logger:  zap.NewNop(),
	}

	groups, err := manager.GetIOMMUGroups()
	if err != nil {
		t.Fatalf("GetIOMMUGroups failed: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

func TestManager_GetIOMMUGroups_WithDevices(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress: "0000:01:00.0",
				IOMMUGroup: 1,
			},
			"0000:01:00.1": {
				PCIAddress: "0000:01:00.1",
				IOMMUGroup: 1,
			},
			"0000:02:00.0": {
				PCIAddress: "0000:02:00.0",
				IOMMUGroup: 2,
			},
			"0000:03:00.0": {
				PCIAddress: "0000:03:00.0",
				IOMMUGroup: -1, // 未分组
			},
		},
		logger: zap.NewNop(),
	}

	groups, err := manager.GetIOMMUGroups()
	if err != nil {
		t.Fatalf("GetIOMMUGroups failed: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	if len(groups[1]) != 2 {
		t.Errorf("expected 2 devices in group 1, got %d", len(groups[1]))
	}

	if len(groups[2]) != 1 {
		t.Errorf("expected 1 device in group 2, got %d", len(groups[2]))
	}
}

func TestManager_DetectGPUDevices_Empty(t *testing.T) {
	manager := &Manager{
		devices: make(map[string]*DeviceInfo),
		logger:  zap.NewNop(),
	}

	gpus, err := manager.DetectGPUDevices()
	if err != nil {
		t.Fatalf("DetectGPUDevices failed: %v", err)
	}
	if len(gpus) != 0 {
		t.Errorf("expected 0 GPUs, got %d", len(gpus))
	}
}

func TestManager_DetectGPUDevices_WithGPUs(t *testing.T) {
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
				DeviceType: DeviceTypeGPU,
			},
		},
		logger: zap.NewNop(),
	}

	gpus, err := manager.DetectGPUDevices()
	if err != nil {
		t.Fatalf("DetectGPUDevices failed: %v", err)
	}
	if len(gpus) != 2 {
		t.Errorf("expected 2 GPUs, got %d", len(gpus))
	}
}

func TestManager_HotPlugDevice_EmptyPCIAddr(t *testing.T) {
	manager := NewManager(zap.NewNop())

	err := manager.HotPlugDevice("", "100")
	if err == nil {
		t.Error("HotPlugDevice should return error for empty PCI address")
	}
}

func TestManager_HotPlugDevice_EmptyVMID(t *testing.T) {
	manager := NewManager(zap.NewNop())

	err := manager.HotPlugDevice("0000:01:00.0", "")
	if err == nil {
		t.Error("HotPlugDevice should return error for empty VM ID")
	}
}

func TestManager_HotPlugDevice_DeviceNotFound(t *testing.T) {
	manager := NewManager(zap.NewNop())

	err := manager.HotPlugDevice("0000:01:00.0", "100")
	if err == nil {
		t.Error("HotPlugDevice should return error for non-existent device")
	}
}

func TestManager_HotPlugDevice_NotBoundToVFIO(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress:  "0000:01:00.0",
				Driver:      "nvidia",
				DriverState: DriverStateBound,
			},
		},
		logger: zap.NewNop(),
	}

	err := manager.HotPlugDevice("0000:01:00.0", "100")
	if err == nil {
		t.Error("HotPlugDevice should return error when device not bound to VFIO")
	}
}

func TestManager_HotPlugDevice_InvalidIOMMUGroup(t *testing.T) {
	manager := &Manager{
		devices: map[string]*DeviceInfo{
			"0000:01:00.0": {
				PCIAddress:  "0000:01:00.0",
				Driver:      VFIOStubDrivers,
				DriverState: DriverStateBound,
				IOMMUGroup:  -1,
			},
		},
		logger: zap.NewNop(),
	}

	err := manager.HotPlugDevice("0000:01:00.0", "100")
	if err == nil {
		t.Error("HotPlugDevice should return error for invalid IOMMU group")
	}
}

func TestClassifyDevice(t *testing.T) {
	tests := []struct {
		name      string
		classCode string
		expected  DeviceType
	}{
		{
			name:      "GPU device",
			classCode: "0x030000",
			expected:  DeviceTypeGPU,
		},
		{
			name:      "GPU device without prefix",
			classCode: "030000",
			expected:  DeviceTypeGPU,
		},
		{
			name:      "Network device",
			classCode: "0x020000",
			expected:  DeviceTypeNetwork,
		},
		{
			name:      "Storage device",
			classCode: "0x010000",
			expected:  DeviceTypeStorage,
		},
		{
			name:      "USB controller",
			classCode: "0x0c0300",
			expected:  DeviceTypeUSB,
		},
		{
			name:      "Audio device",
			classCode: "0x040000",
			expected:  DeviceTypeAudio,
		},
		{
			name:      "Other device",
			classCode: "0x123456",
			expected:  DeviceTypeOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{logger: zap.NewNop()}
			result := manager.classifyDevice(tt.classCode)
			if result != tt.expected {
				t.Errorf("classifyDevice(%s) = %v, expected %v", tt.classCode, result, tt.expected)
			}
		})
	}
}

func TestValidatePCIAddress(t *testing.T) {
	tests := []struct {
		name     string
		pciAddr  string
		expected bool
	}{
		{
			name:     "Valid address",
			pciAddr:  "0000:01:00.0",
			expected: true,
		},
		{
			name:     "Valid address with function 1",
			pciAddr:  "0000:01:00.1",
			expected: true,
		},
		{
			name:     "Valid address with function 15",
			pciAddr:  "0000:01:00.f",
			expected: true,
		},
		{
			name:     "Invalid format - no domain",
			pciAddr:  "01:00.0",
			expected: false,
		},
		{
			name:     "Invalid format - no function",
			pciAddr:  "0000:01:00",
			expected: false,
		},
		{
			name:     "Invalid format - extra segment",
			pciAddr:  "0000:01:00:00.0",
			expected: false,
		},
		{
			name:     "Empty address",
			pciAddr:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePCIAddress(tt.pciAddr)
			if result != tt.expected {
				t.Errorf("ValidatePCIAddress(%s) = %v, expected %v", tt.pciAddr, result, tt.expected)
			}
		})
	}
}

func TestExtractBus(t *testing.T) {
	tests := []struct {
		name     string
		pciAddr  string
		expected string
	}{
		{
			name:     "Valid address",
			pciAddr:  "0000:01:00.0",
			expected: "01",
		},
		{
			name:     "Short address",
			pciAddr:  "00:00",
			expected: "00",
		},
		{
			name:     "Empty address",
			pciAddr:  "",
			expected: "00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBus(tt.pciAddr)
			if result != tt.expected {
				t.Errorf("extractBus(%s) = %s, expected %s", tt.pciAddr, result, tt.expected)
			}
		})
	}
}

func TestExtractSlot(t *testing.T) {
	tests := []struct {
		name     string
		pciAddr  string
		expected string
	}{
		{
			name:     "Valid address",
			pciAddr:  "0000:01:00.0",
			expected: "00",
		},
		{
			name:     "Different slot",
			pciAddr:  "0000:02:1f.0",
			expected: "1f",
		},
		{
			name:     "Empty address",
			pciAddr:  "",
			expected: "00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSlot(tt.pciAddr)
			if result != tt.expected {
				t.Errorf("extractSlot(%s) = %s, expected %s", tt.pciAddr, result, tt.expected)
			}
		})
	}
}

func TestExtractFunction(t *testing.T) {
	tests := []struct {
		name     string
		pciAddr  string
		expected string
	}{
		{
			name:     "Function 0",
			pciAddr:  "0000:01:00.0",
			expected: "0",
		},
		{
			name:     "Function 1",
			pciAddr:  "0000:01:00.1",
			expected: "1",
		},
		{
			name:     "No function",
			pciAddr:  "0000:01:00",
			expected: "0",
		},
		{
			name:     "Empty address",
			pciAddr:  "",
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFunction(tt.pciAddr)
			if result != tt.expected {
				t.Errorf("extractFunction(%s) = %s, expected %s", tt.pciAddr, result, tt.expected)
			}
		})
	}
}

func TestLookupDeviceInfo(t *testing.T) {
	manager := &Manager{logger: zap.NewNop()}

	tests := []struct {
		name         string
		vendorID     string
		deviceID     string
		expectedName string
	}{
		{
			name:         "NVIDIA",
			vendorID:     "0x10de",
			deviceID:     "0x1234",
			expectedName: "NVIDIA Corporation",
		},
		{
			name:         "AMD",
			vendorID:     "0x1002",
			deviceID:     "0x5678",
			expectedName: "Advanced Micro Devices",
		},
		{
			name:         "Intel",
			vendorID:     "0x8086",
			deviceID:     "0x9abc",
			expectedName: "Intel Corporation",
		},
		{
			name:         "Unknown vendor",
			vendorID:     "0x1234",
			deviceID:     "0x5678",
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, _ := manager.lookupDeviceInfo(tt.vendorID, tt.deviceID)
			if name != tt.expectedName {
				t.Errorf("lookupDeviceInfo(%s, %s) name = %s, expected %s", tt.vendorID, tt.deviceID, name, tt.expectedName)
			}
		})
	}
}

func TestDeviceType_Constants(t *testing.T) {
	if DeviceTypeGPU != "gpu" {
		t.Errorf("DeviceTypeGPU should be 'gpu', got %s", DeviceTypeGPU)
	}
	if DeviceTypeNetwork != "network" {
		t.Errorf("DeviceTypeNetwork should be 'network', got %s", DeviceTypeNetwork)
	}
	if DeviceTypeStorage != "storage" {
		t.Errorf("DeviceTypeStorage should be 'storage', got %s", DeviceTypeStorage)
	}
	if DeviceTypeUSB != "usb" {
		t.Errorf("DeviceTypeUSB should be 'usb', got %s", DeviceTypeUSB)
	}
	if DeviceTypeAudio != "audio" {
		t.Errorf("DeviceTypeAudio should be 'audio', got %s", DeviceTypeAudio)
	}
	if DeviceTypeOther != "other" {
		t.Errorf("DeviceTypeOther should be 'other', got %s", DeviceTypeOther)
	}
}

func TestDriverState_Constants(t *testing.T) {
	if DriverStateBound != "bound" {
		t.Errorf("DriverStateBound should be 'bound', got %s", DriverStateBound)
	}
	if DriverStateUnbound != "unbound" {
		t.Errorf("DriverStateUnbound should be 'unbound', got %s", DriverStateUnbound)
	}
	if DriverStateError != "error" {
		t.Errorf("DriverStateError should be 'error', got %s", DriverStateError)
	}
}

func TestDeviceInfo_Struct(t *testing.T) {
	dev := DeviceInfo{
		PCIAddress:  "0000:01:00.0",
		VendorID:    "0x10de",
		DeviceID:    "0x1234",
		ClassName:   "0x030000",
		DeviceType:  DeviceTypeGPU,
		IOMMUGroup:  1,
		Driver:      "nvidia",
		DriverState: DriverStateBound,
		VendorName:  "NVIDIA Corporation",
		DeviceName:  "GeForce GTX 1080",
		NumaNode:    0,
		Slot:        "01:00.0",
	}

	if dev.PCIAddress != "0000:01:00.0" {
		t.Errorf("expected PCIAddress = 0000:01:00.0, got %s", dev.PCIAddress)
	}
	if dev.DeviceType != DeviceTypeGPU {
		t.Errorf("expected DeviceType = gpu, got %s", dev.DeviceType)
	}
	if dev.IOMMUGroup != 1 {
		t.Errorf("expected IOMMUGroup = 1, got %d", dev.IOMMUGroup)
	}
}

func TestPassthroughConfig_Struct(t *testing.T) {
	config := PassthroughConfig{
		TargetID:      "100",
		TargetType:    "vm",
		PCIAddr:       "0000:01:00.0",
		VFIOID:        "1",
		ROMFile:       "/path/to/rom.rom",
		Multifunction: true,
	}

	if config.TargetID != "100" {
		t.Errorf("expected TargetID = 100, got %s", config.TargetID)
	}
	if config.TargetType != "vm" {
		t.Errorf("expected TargetType = vm, got %s", config.TargetType)
	}
	if config.PCIAddr != "0000:01:00.0" {
		t.Errorf("expected PCIAddr = 0000:01:00.0, got %s", config.PCIAddr)
	}
	if !config.Multifunction {
		t.Error("expected Multifunction = true")
	}
}

func TestConstants(t *testing.T) {
	if DefaultSysfsPCIDevices != "/sys/bus/pci/devices" {
		t.Errorf("expected DefaultSysfsPCIDevices = /sys/bus/pci/devices, got %s", DefaultSysfsPCIDevices)
	}
	if DefaultIOMMUGroupsPath != "/sys/kernel/iommu_groups" {
		t.Errorf("expected DefaultIOMMUGroupsPath = /sys/kernel/iommu_groups, got %s", DefaultIOMMUGroupsPath)
	}
	if DefaultVFIOPath != "/dev/vfio" {
		t.Errorf("expected DefaultVFIOPath = /dev/vfio, got %s", DefaultVFIOPath)
	}
	if VFIOStubDrivers != "vfio-pci" {
		t.Errorf("expected VFIOStubDrivers = vfio-pci, got %s", VFIOStubDrivers)
	}
}
