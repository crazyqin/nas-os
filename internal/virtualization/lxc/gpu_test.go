// Package lxc GPU透传测试
package lxc

import (
	"testing"
)

func TestGPUPassthroughConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *GPUPassthroughConfig
		wantErr bool
	}{
		{
			name: "valid config with GPUIndex",
			config: &GPUPassthroughConfig{
				GPUIndex: 0,
				Vendor:   "nvidia",
				Mode:     GPUModeShared,
			},
			wantErr: false,
		},
		{
			name: "valid config with GPUUUID",
			config: &GPUPassthroughConfig{
				GPUUUID: "GPU-12345678-1234-1234-1234-123456789abc",
				Vendor:  "nvidia",
				Mode:    GPUModeFull,
			},
			wantErr: false,
		},
		{
			name: "valid config with PCIAddress",
			config: &GPUPassthroughConfig{
				PCIAddress: "0000:01:00.0",
				Vendor:     "amd",
				Mode:       GPUModeVFIO,
			},
			wantErr: false,
		},
		{
			name: "invalid config - no GPU identifier",
			config: &GPUPassthroughConfig{
				Vendor: "nvidia",
			},
			wantErr: true,
		},
		{
			name: "invalid vendor",
			config: &GPUPassthroughConfig{
				GPUIndex: 0,
				Vendor:   "unknown",
			},
			wantErr: true,
		},
		{
			name: "default vendor nvidia",
			config: &GPUPassthroughConfig{
				GPUIndex: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{backend: BackendIncus}
			err := m.validateGPUDevice(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGPUDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildGPUDeviceConfig(t *testing.T) {
	m := &Manager{backend: BackendIncus}

	// Test NVIDIA config
	nvidiaConfig := &GPUPassthroughConfig{
		GPUIndex: 0,
		Vendor:   "nvidia",
		Mode:     GPUModeShared,
		NVIDIAConfig: &NVIDIAGPUConfig{
			EnableCUDA:  true,
			EnableNVENC: true,
			CUDAVersion: "11.8",
		},
	}

	device := m.buildGPUDeviceConfig(nvidiaConfig)
	if device.Type != "gpu" {
		t.Errorf("device type should be gpu, got %s", device.Type)
	}

	if device.Config["vendor"] != "nvidia" {
		t.Errorf("vendor should be nvidia, got %s", device.Config["vendor"])
	}

	if device.Config["gpuindex"] != "0" {
		t.Errorf("gpuindex should be 0, got %s", device.Config["gpuindex"])
	}

	// Test AMD config
	amdConfig := &GPUPassthroughConfig{
		GPUIndex: 1,
		Vendor:   "amd",
		Mode:     GPUModeShared,
		AMDConfig: &AMDGPUConfig{
			EnableROCm: true,
			ROCmVersion: "5.6",
		},
	}

	device = m.buildGPUDeviceConfig(amdConfig)
	if device.Config["vendor"] != "amd" {
		t.Errorf("vendor should be amd, got %s", device.Config["vendor"])
	}

	if device.Config["amd.rocm.enabled"] != "true" {
		t.Errorf("amd.rocm.enabled should be true")
	}

	// Test Intel config
	intelConfig := &GPUPassthroughConfig{
		GPUIndex: 0,
		Vendor:   "intel",
		Mode:     GPUModeShared,
		IntelConfig: &IntelGPUConfig{
			EnableLevel0: true,
			EnableQSV:    true,
		},
	}

	device = m.buildGPUDeviceConfig(intelConfig)
	if device.Config["vendor"] != "intel" {
		t.Errorf("vendor should be intel, got %s", device.Config["vendor"])
	}

	if device.Config["intel.level0.enabled"] != "true" {
		t.Errorf("intel.level0.enabled should be true")
	}
}

func TestGetDeviceName(t *testing.T) {
	tests := []struct {
		name     string
		config   *GPUPassthroughConfig
		expected string
	}{
		{
			name: "with GPUID",
			config: &GPUPassthroughConfig{
				GPUID: "nvidia0",
			},
			expected: "nvidia0",
		},
		{
			name: "with GPUIndex",
			config: &GPUPassthroughConfig{
				GPUIndex: 1,
			},
			expected: "gpu1",
		},
		{
			name: "with PCIAddress",
			config: &GPUPassthroughConfig{
				PCIAddress: "0000:01:00.0",
			},
			expected: "gpu-0000-01-00-0",
		},
		{
			name:     "empty config",
			config:   &GPUPassthroughConfig{},
			expected: "gpu0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.getDeviceName(); got != tt.expected {
				t.Errorf("getDeviceName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetNVIDIAVisibleDevices(t *testing.T) {
	m := &Manager{backend: BackendIncus}

	tests := []struct {
		name     string
		config   *GPUPassthroughConfig
		expected string
	}{
		{
			name: "with UUID",
			config: &GPUPassthroughConfig{
				GPUUUID: "GPU-123456",
			},
			expected: "GPU-123456",
		},
		{
			name: "with Index",
			config: &GPUPassthroughConfig{
				GPUIndex: 0,
			},
			expected: "0",
		},
		{
			name: "with PCIAddress",
			config: &GPUPassthroughConfig{
				PCIAddress: "0000:01:00.0",
			},
			expected: "0000:01:00.0",
		},
		{
			name:     "empty config",
			config:   &GPUPassthroughConfig{},
			expected: "all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.getNVIDIAVisibleDevices(tt.config); got != tt.expected {
				t.Errorf("getNVIDIAVisibleDevices() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseMemorySize(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"8192 MiB", 8192},
		{"8192 MIB", 8192},
		{"1024 MB", 1024},
		{"4 GB", 4096},
		{"4 GiB", 4096},
		{"2048 KB", 2},
		{"512", 512},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseMemorySize(tt.input); got != tt.expected {
				t.Errorf("parseMemorySize(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"10", 10},
		{" 5 ", 5},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseInt(tt.input); got != tt.expected {
				t.Errorf("parseInt(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGPUModeConstants(t *testing.T) {
	// Verify mode constants
	if GPUModeFull != "full" {
		t.Errorf("GPUModeFull should be 'full'")
	}
	if GPUModeShared != "shared" {
		t.Errorf("GPUModeShared should be 'shared'")
	}
	if GPUModeMIG != "mig" {
		t.Errorf("GPUModeMIG should be 'mig'")
	}
	if GPUModeVFIO != "vfio" {
		t.Errorf("GPUModeVFIO should be 'vfio'")
	}
}

func TestNVIDIAGPUConfigCapabilities(t *testing.T) {
	m := &Manager{backend: BackendIncus}

	config := &NVIDIAGPUConfig{
		EnableCUDA:   true,
		EnableNVENC:  true,
		EnableNVDEC:  true,
		EnableNVDISP: true,
	}

	deviceConfig := make(map[string]string)
	m.buildNVIDIADeviceConfig(config, deviceConfig)

	expected := "utility,compute,video,video.decode,display"
	if deviceConfig["nvidia.driver.capabilities"] != expected {
		t.Errorf("capabilities = %v, want %v", deviceConfig["nvidia.driver.capabilities"], expected)
	}
}

func TestNVIDIAMIGConfig(t *testing.T) {
	m := &Manager{backend: BackendIncus}

	config := &NVIDIAGPUConfig{
		MIGEnabled: true,
		MIGGI:      0,
		MIGCI:      1,
	}

	deviceConfig := make(map[string]string)
	m.buildNVIDIADeviceConfig(config, deviceConfig)

	if deviceConfig["nvidia.mig.enabled"] != "true" {
		t.Errorf("MIG should be enabled")
	}

	if deviceConfig["nvidia.mig.gi"] != "0" {
		t.Errorf("MIGGI should be 0")
	}

	if deviceConfig["nvidia.mig.ci"] != "1" {
		t.Errorf("MIGCI should be 1")
	}
}

func TestNVIDIAMPSConfig(t *testing.T) {
	m := &Manager{backend: BackendIncus}

	config := &NVIDIAGPUConfig{
		MPSEnabled:    true,
		MPSPipeDir:    "/tmp/mps",
		MPSThreadPool: 4,
	}

	deviceConfig := make(map[string]string)
	m.buildNVIDIADeviceConfig(config, deviceConfig)

	if deviceConfig["nvidia.mps.enabled"] != "true" {
		t.Errorf("MPS should be enabled")
	}

	if deviceConfig["nvidia.mps.pipedir"] != "/tmp/mps" {
		t.Errorf("MPSPipeDir should be /tmp/mps")
	}

	if deviceConfig["nvidia.mps.threadpool"] != "4" {
		t.Errorf("MPSThreadPool should be 4")
	}
}

func TestGPUDeviceInfo(t *testing.T) {
	info := GPUDeviceInfo{
		ID:           "nvidia0",
		Index:        0,
		UUID:         "GPU-123456",
		Vendor:       "nvidia",
		Model:        "RTX 3080",
		MemoryTotal:  10240,
		MemoryUsed:   2048,
		Status:       "available",
		Mode:         GPUModeShared,
		MIGAvailable: true,
	}

	if info.ID != "nvidia0" {
		t.Errorf("ID mismatch")
	}

	if info.MemoryTotal != 10240 {
		t.Errorf("MemoryTotal mismatch")
	}

	if !info.MIGAvailable {
		t.Errorf("MIGAvailable should be true")
	}
}