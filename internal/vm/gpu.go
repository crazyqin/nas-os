// Package vm 提供GPU直通增强功能
package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ========== GPU直通配置 ==========

// GPUType GPU类型
type GPUType string

const (
	GPUTypeNVIDIA GPUType = "nvidia"
	GPUTypeAMD    GPUType = "amd"
	GPUTypeIntel  GPUType = "intel"
)

// GPUDevice GPU设备
type GPUDevice struct {
	ID           string `json:"id"`
	PCIAddress   string `json:"pci_address"`
	Type         GPUType `json:"type"`
	Model        string `json:"model"`
	VendorID     string `json:"vendor_id"`
	DeviceID     string `json:"device_id"`
	Driver       string `json:"driver"`
	MemoryMB     int    `json:"memory_mb"`
	AttachedToVM string `json:"attached_to_vm,omitempty"`
	Status       string `json:"status"`
}

// GPUPassthroughConfig GPU直通配置
type GPUPassthroughConfig struct {
	DeviceID      string `json:"device_id"`
	VMID          string `json:"vm_id"`
	EnableVNC     bool   `json:"enable_vnc"`
	DisplayMode   string `json:"display_mode"` // none, vnc, spice
	ROMFile       string `json:"rom_file,omitempty"`
	EnableAudio   bool   `json:"enable_audio"`
	EnableUSB     bool   `json:"enable_usb"`
	BusType       string `json:"bus_type"` // pci, pcie
	MultiFunction bool   `json:"multi_function"`
}

// GPUPassthroughManager GPU直通管理器
type GPUPassthroughManager struct {
	mu       sync.RWMutex
	gpus     map[string]*GPUDevice
	vms      map[string]*VMConfig
}

// VMConfig 虚拟机配置
type VMConfig struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Status     string               `json:"status"`
	GPUs       []GPUPassthroughConfig `json:"gpus"`
	CreatedAt  time.Time            `json:"created_at"`
}

// NewGPUPassthroughManager 创建GPU直通管理器
func NewGPUPassthroughManager() *GPUPassthroughManager {
	return &GPUPassthroughManager{
		gpus: make(map[string]*GPUDevice),
		vms:  make(map[string]*VMConfig),
	}
}

// ScanGPUs 扫描GPU设备
func (m *GPUPassthroughManager) ScanGPUs(ctx context.Context) ([]GPUDevice, error) {
	// 使用lspci扫描GPU
	cmd := exec.CommandContext(ctx, "lspci", "-nn", "-d", "::0300")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("扫描GPU失败: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var gpus []GPUDevice

	for _, line := range lines {
		if line == "" {
			continue
		}

		// 解析PCI地址和设备信息
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		gpu := GPUDevice{
			ID:         fmt.Sprintf("gpu-%s", parts[0]),
			PCIAddress: parts[0],
			Status:     "available",
		}

		// 判断GPU类型
		if strings.Contains(line, "NVIDIA") {
			gpu.Type = GPUTypeNVIDIA
		} else if strings.Contains(line, "AMD") || strings.Contains(line, "Advanced Micro Devices") {
			gpu.Type = GPUTypeAMD
		} else if strings.Contains(line, "Intel") {
			gpu.Type = GPUTypeIntel
		}

		// 提取型号
		modelStart := strings.Index(line, "]")
		if modelStart > 0 && modelStart+2 < len(line) {
			gpu.Model = strings.TrimSpace(line[modelStart+2:])
		}

		gpus = append(gpus, gpu)
		m.gpus[gpu.ID] = &gpu
	}

	return gpus, nil
}

// ListGPUs 列出GPU设备
func (m *GPUPassthroughManager) ListGPUs() []GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	gpus := make([]GPUDevice, 0, len(m.gpus))
	for _, gpu := range m.gpus {
		gpus = append(gpus, *gpu)
	}
	return gpus
}

// AttachGPU 将GPU直通给VM
func (m *GPUPassthroughManager) AttachGPU(config GPUPassthroughConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	gpu, ok := m.gpus[config.DeviceID]
	if !ok {
		return fmt.Errorf("GPU设备不存在: %s", config.DeviceID)
	}

	if gpu.AttachedToVM != "" {
		return fmt.Errorf("GPU已被占用: %s", gpu.AttachedToVM)
	}

	// 检查IOMMU是否启用
	if err := m.checkIOMMU(); err != nil {
		return fmt.Errorf("IOMMU未启用: %w", err)
	}

	// 绑定到vfio-pci驱动
	if err := m.bindToVFIO(gpu.PCIAddress); err != nil {
		return fmt.Errorf("绑定VFIO失败: %w", err)
	}

	gpu.AttachedToVM = config.VMID
	gpu.Status = "attached"

	// 更新VM配置
	vm, ok := m.vms[config.VMID]
	if ok {
		vm.GPUs = append(vm.GPUs, config)
	}

	return nil
}

// DetachGPU 从VM分离GPU
func (m *GPUPassthroughManager) DetachGPU(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	gpu, ok := m.gpus[deviceID]
	if !ok {
		return fmt.Errorf("GPU设备不存在: %s", deviceID)
	}

	if gpu.AttachedToVM == "" {
		return fmt.Errorf("GPU未绑定到任何VM")
	}

	// 恢复原始驱动
	if err := m.unbindFromVFIO(gpu.PCIAddress, string(gpu.Type)); err != nil {
		return fmt.Errorf("恢复驱动失败: %w", err)
	}

	gpu.AttachedToVM = ""
	gpu.Status = "available"

	return nil
}

// GetGPUPerformance 获取GPU性能指标
func (m *GPUPassthroughManager) GetGPUPerformance(deviceID string) (GPUPerformance, error) {
	m.mu.RLock()
	gpu, ok := m.gpus[deviceID]
	m.mu.RUnlock()

	if !ok {
		return GPUPerformance{}, fmt.Errorf("GPU设备不存在: %s", deviceID)
	}

	// 基本性能数据
	perf := GPUPerformance{
		DeviceID:    deviceID,
		Timestamp:   time.Now(),
		MemoryUsed:  0,
		MemoryTotal: gpu.MemoryMB,
	}

	// 根据GPU类型获取详细指标
	switch gpu.Type {
	case GPUTypeNVIDIA:
		m.getNVIDIAMetrics(gpu.PCIAddress, &perf)
	case GPUTypeAMD:
		m.getAMDMetrics(gpu.PCIAddress, &perf)
	}

	return perf, nil
}

// GPUPerformance GPU性能数据
type GPUPerformance struct {
	DeviceID    string    `json:"device_id"`
	Timestamp   time.Time `json:"timestamp"`
	GPUUsage    float64   `json:"gpu_usage"`
	MemoryUsed  int       `json:"memory_used"`
	MemoryTotal int       `json:"memory_total"`
	Temperature int       `json:"temperature"`
	PowerDraw   float64   `json:"power_draw"`
}

// 内部方法
func (m *GPUPassthroughManager) checkIOMMU() error {
	// 检查IOMMU是否在内核启动参数中启用
	cmd := exec.Command("grep", "-q", "iommu=on", "/proc/cmdline")
	return cmd.Run()
}

func (m *GPUPassthroughManager) bindToVFIO(pciAddr string) error {
	// 将设备绑定到vfio-pci驱动
	cmd := exec.Command("echo", "vfio-pci", ">", fmt.Sprintf("/sys/bus/pci/devices/%s/driver_override", pciAddr))
	return cmd.Run()
}

func (m *GPUPassthroughManager) unbindFromVFIO(pciAddr, gpuType string) error {
	// 恢复原始驱动
	var driver string
	switch gpuType {
	case string(GPUTypeNVIDIA):
		driver = "nvidia"
	case string(GPUTypeAMD):
		driver = "amdgpu"
	default:
		driver = ""
	}

	if driver != "" {
		cmd := exec.Command("echo", driver, ">", fmt.Sprintf("/sys/bus/pci/devices/%s/driver_override", pciAddr))
		return cmd.Run()
	}
	return nil
}

func (m *GPUPassthroughManager) getNVIDIAMetrics(pciAddr string, perf *GPUPerformance) {
	// 使用nvidia-smi获取指标
	cmd := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu,memory.used,temperature.gpu,power.draw", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) >= 4 {
		fmt.Sscanf(parts[0], " %f", &perf.GPUUsage)
		fmt.Sscanf(parts[1], " %d", &perf.MemoryUsed)
		fmt.Sscanf(parts[2], " %d", &perf.Temperature)
		fmt.Sscanf(parts[3], " %f", &perf.PowerDraw)
	}
}

func (m *GPUPassthroughManager) getAMDMetrics(pciAddr string, perf *GPUPerformance) {
	// 使用rocm-smi获取AMD GPU指标
	cmd := exec.Command("rocm-smi", "--showuse", "--showmeminfo", "--showtemp")
	cmd.Run() // 简化实现
}

// Export 导出配置
func (m *GPUPassthroughManager) Export() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := map[string]interface{}{
		"gpus": m.gpus,
		"vms":  m.vms,
	}
	return json.MarshalIndent(data, "", "  ")
}