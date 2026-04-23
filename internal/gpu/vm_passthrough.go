// Package gpu VM GPU透传增强
package gpu

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// VMGPUPassthrough VM GPU透传配置
type VMGPUPassthrough struct {
	VMID         string          `json:"vmId"`
	VMName       string          `json:"vmName"`
	GPUDevice    string          `json:"gpuDevice"`    // GPU设备ID
	PCIAddress   string          `json:"pciAddress"`   // PCI地址
	Vendor       string          `json:"vendor"`       // nvidia, amd, intel
	Mode         PassthroughMode `json:"mode"`         // vfio, pci, mig
	Attached     bool            `json:"attached"`     // 是否已附加
	AttachedAt   time.Time       `json:"attachedAt"`   // 附加时间
	IOMMUEnabled bool            `json:"iommuEnabled"` // IOMMU是否启用
	VFIOBound    bool            `json:"vfioBound"`    // 是否绑定VFIO
}

// VMGPUManager VM GPU管理器
type VMGPUManager struct {
	manager      *Manager
	logger       Logger
	passthroughs map[string]*VMGPUPassthrough // key: vmId-gpuId
	mu           sync.RWMutex
}

// Logger 简化日志接口
type Logger interface {
	Info(msg string, fields ...Field)
	Debug(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
}

// Field 日志字段
type Field struct {
	Key   string
	Value interface{}
}

// NewVMGPUManager 创建VM GPU管理器
func NewVMGPUManager(manager *Manager, logger Logger) *VMGPUManager {
	return &VMGPUManager{
		manager:      manager,
		logger:       logger,
		passthroughs: make(map[string]*VMGPUPassthrough),
	}
}

// AttachGPUToVM GPU附加到虚拟机
func (vmgm *VMGPUManager) AttachGPUToVM(ctx context.Context, vmName, gpuDevice string, mode PassthroughMode) error {
	vmgm.mu.Lock()
	defer vmgm.mu.Unlock()

	// 检查GPU状态
	device, err := vmgm.manager.GetGPU(gpuDevice)
	if err != nil {
		return fmt.Errorf("GPU设备不存在: %w", err)
	}

	if device.Allocated {
		return fmt.Errorf("GPU %s 已被分配给 %s", gpuDevice, device.AllocatedTo)
	}

	// 检查IOMMU支持
	iommuEnabled, err := vmgm.checkIOMMU()
	if err != nil {
		return fmt.Errorf("IOMMU检查失败: %w", err)
	}

	if !iommuEnabled && mode == PassthroughModeVFIO {
		return fmt.Errorf("VFIO透传需要IOMMU支持")
	}

	// 构建透传配置
	pt := &VMGPUPassthrough{
		VMID:         vmName,
		VMName:       vmName,
		GPUDevice:    gpuDevice,
		PCIAddress:   device.PCIAddress,
		Vendor:       device.Vendor,
		Mode:         mode,
		IOMMUEnabled: iommuEnabled,
	}

	// VFIO模式需要绑定驱动
	if mode == PassthroughModeVFIO {
		if err := vmgm.bindToVFIO(device.PCIAddress); err != nil {
			return fmt.Errorf("VFIO绑定失败: %w", err)
		}
		pt.VFIOBound = true
	}

	// 更新VM配置（实际实现需要调用libvirt API）
	ptKey := fmt.Sprintf("%s-%s", vmName, gpuDevice)
	vmgm.passthroughs[ptKey] = pt

	vmgm.logger.Info("GPU附加到VM",
		Field{Key: "vmName", Value: vmName},
		Field{Key: "gpuDevice", Value: gpuDevice},
		Field{Key: "mode", Value: mode})

	return nil
}

// DetachGPUFromVM 从虚拟机分离GPU
func (vmgm *VMGPUManager) DetachGPUFromVM(ctx context.Context, vmName, gpuDevice string) error {
	vmgm.mu.Lock()
	defer vmgm.mu.Unlock()

	ptKey := fmt.Sprintf("%s-%s", vmName, gpuDevice)
	pt, exists := vmgm.passthroughs[ptKey]
	if !exists {
		return fmt.Errorf("GPU透传配置不存在")
	}

	// 如果是VFIO模式，恢复原始驱动
	if pt.VFIOBound {
		if err := vmgm.unbindFromVFIO(pt.PCIAddress, pt.Vendor); err != nil {
			vmgm.logger.Warn("VFIO解绑失败", Field{Key: "error", Value: err})
		}
	}

	// 释放GPU
	if err := vmgm.manager.ReleaseGPU(&GPUReleaseRequest{
		ContainerID: vmName,
		GPUID:       gpuDevice,
	}); err != nil {
		vmgm.logger.Warn("GPU释放失败", Field{Key: "error", Value: err})
	}

	delete(vmgm.passthroughs, ptKey)

	vmgm.logger.Info("GPU从VM分离",
		Field{Key: "vmName", Value: vmName},
		Field{Key: "gpuDevice", Value: gpuDevice})

	return nil
}

// ListVMGPUs 列出VM的GPU配置
func (vmgm *VMGPUManager) ListVMGPUs(vmName string) []*VMGPUPassthrough {
	vmgm.mu.RLock()
	defer vmgm.mu.RUnlock()

	var result []*VMGPUPassthrough
	for key, pt := range vmgm.passthroughs {
		if strings.HasPrefix(key, vmName+"-") {
			result = append(result, pt)
		}
	}

	return result
}

// checkIOMMU 检查IOMMU是否启用
func (vmgm *VMGPUManager) checkIOMMU() (bool, error) {
	// 检查内核参数
	cmd := exec.Command("grep", "-q", "iommu=on", "/proc/cmdline")
	if err := cmd.Run(); err != nil {
		// 尝试检查intel_iommu=on (Intel)
		cmd = exec.Command("grep", "-q", "intel_iommu=on", "/proc/cmdline")
		if err := cmd.Run(); err != nil {
			// 尝试检查amd_iommu=on (AMD)
			cmd = exec.Command("grep", "-q", "amd_iommu=on", "/proc/cmdline")
			if err := cmd.Run(); err != nil {
				return false, nil
			}
		}
	}

	// 检查IOMMU设备是否存在
	if _, err := os.Stat("/sys/kernel/iommu_groups"); err == nil {
		return true, nil
	}

	return false, nil
}

// bindToVFIO 将设备绑定到VFIO驱动
func (vmgm *VMGPUManager) bindToVFIO(pciAddr string) error {
	// 1. 获取设备ID
	vendorID, deviceID, err := vmgm.getPCIDeviceIDs(pciAddr)
	if err != nil {
		return fmt.Errorf("获取设备ID失败: %w", err)
	}

	// 2. 检查vfio-pci驱动是否加载
	cmd := exec.Command("modprobe", "vfio-pci")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("加载vfio-pci驱动失败: %w", err)
	}

	// 3. 创建vfio-pci配置文件（需要root权限）
	configPath := fmt.Sprintf("/sys/bus/pci/drivers/vfio-pci/new_id")
	configCmd := exec.Command("sh", "-c",
		fmt.Sprintf("echo '%s %s' > %s", vendorID, deviceID, configPath))
	if err := configCmd.Run(); err != nil {
		return fmt.Errorf("添加VFIO配置失败: %w", err)
	}

	// 4. 绑定设备到vfio-pci
	overridePath := fmt.Sprintf("/sys/bus/pci/devices/%s/driver_override", pciAddr)
	overrideCmd := exec.Command("sh", "-c",
		fmt.Sprintf("echo 'vfio-pci' > %s", overridePath))
	if err := overrideCmd.Run(); err != nil {
		return fmt.Errorf("设置驱动override失败: %w", err)
	}

	// 5. 重新绑定驱动
	unbindPath := fmt.Sprintf("/sys/bus/pci/devices/%s/driver/unbind", pciAddr)
	unbindCmd := exec.Command("sh", "-c",
		fmt.Sprintf("echo '%s' > %s", pciAddr, unbindPath))
	unbindCmd.Run() // 忽略错误，可能已经解绑

	probePath := fmt.Sprintf("/sys/bus/pci/drivers/vfio-pci/bind", pciAddr)
	probeCmd := exec.Command("sh", "-c",
		fmt.Sprintf("echo '%s' > %s", pciAddr, probePath))
	if err := probeCmd.Run(); err != nil {
		return fmt.Errorf("绑定VFIO失败: %w", err)
	}

	return nil
}

// unbindFromVFIO 从VFIO解绑，恢复原始驱动
func (vmgm *VMGPUManager) unbindFromVFIO(pciAddr, vendor string) error {
	// 确定原始驱动
	var driver string
	switch vendor {
	case "nvidia":
		driver = "nvidia"
	case "amd":
		driver = "amdgpu"
	case "intel":
		driver = "i915"
	default:
		driver = "nvidia" // 默认
	}

	// 从VFIO解绑
	unbindPath := fmt.Sprintf("/sys/bus/pci/drivers/vfio-pci/unbind", pciAddr)
	unbindCmd := exec.Command("sh", "-c",
		fmt.Sprintf("echo '%s' > %s", pciAddr, unbindPath))
	unbindCmd.Run()

	// 设置原始驱动override
	overridePath := fmt.Sprintf("/sys/bus/pci/devices/%s/driver_override", pciAddr)
	overrideCmd := exec.Command("sh", "-c",
		fmt.Sprintf("echo '%s' > %s", driver, overridePath))
	if err := overrideCmd.Run(); err != nil {
		return fmt.Errorf("恢复驱动失败: %w", err)
	}

	// 绑定原始驱动
	probePath := fmt.Sprintf("/sys/bus/pci/drivers/%s/bind", driver)
	probeCmd := exec.Command("sh", "-c",
		fmt.Sprintf("echo '%s' > %s", pciAddr, probePath))
	if err := probeCmd.Run(); err != nil {
		return fmt.Errorf("绑定原始驱动失败: %w", err)
	}

	return nil
}

// getPCIDeviceIDs 获取PCI设备的Vendor ID和Device ID
func (vmgm *VMGPUManager) getPCIDeviceIDs(pciAddr string) (string, string, error) {
	cmd := exec.Command("lspci", "-n", "-s", pciAddr)
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	// 解析输出: 01:00.0 0300: 10de:2206 (格式)
	line := strings.TrimSpace(string(output))
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return "", "", fmt.Errorf("无法解析lspci输出")
	}

	// 提取vendor:device ID
	idParts := strings.Split(parts[2], ":")
	if len(idParts) != 2 {
		return "", "", fmt.Errorf("无法解析设备ID")
	}

	return idParts[0], idParts[1], nil
}

// GenerateLibvirtXML 生成Libvirt XML片段
func (vmgm *VMGPUManager) GenerateLibvirtXML(passthrough *VMGPUPassthrough) string {
	var xmlBuilder strings.Builder

	// PCI设备配置
	xmlBuilder.WriteString(`<hostdev mode='subsystem' type='pci' managed='yes'>`)
	xmlBuilder.WriteString(`<source>`)
	xmlBuilder.WriteString(`<address domain='0x0000' bus='0x01' slot='0x00' function='0x0'/>`) // 需要根据实际PCI地址调整
	xmlBuilder.WriteString(`</source>`)
	xmlBuilder.WriteString(`<address type='pci' domain='0x0000' bus='0x06' slot='0x00' function='0x0'/>`) // VM内地址
	xmlBuilder.WriteString(`</hostdev>`)

	// VFIO配置选项
	if passthrough.VFIOBound {
		// 添加ROM文件配置（可选）
		xmlBuilder.WriteString(`<rom bar='on' file='/path/to/gpu.rom'/>`) // ROM文件路径需要配置
	}

	return xmlBuilder.String()
}

// MIGGPUManager MIG GPU管理器
type MIGGPUManager struct {
	manager *Manager
	logger  Logger
	migInstances map[string]*MIGInstance // key: gi-ci
	mu      sync.RWMutex
}

// MIGInstance MIG实例
type MIGInstance struct {
	GPUID       string    `json:"gpuId"`
	GPUIndex    int       `json:"gpuIndex"`
	GI          int       `json:"gi"`          // GPU Instance ID
	CI          int       `json:"ci"`          // Compute Instance ID
	Profile     string    `json:"profile"`     // MIG profile
	MemoryMB    uint64    `json:"memoryMb"`    // 分配的显存
	CreatedAt   time.Time `json:"createdAt"`
	AttachedTo  string    `json:"attachedTo"`  // 附加目标
}

// NewMIGGPUManager 创建MIG管理器
func NewMIGGPUManager(manager *Manager, logger Logger) *MIGGPUManager {
	return &MIGGPUManager{
		manager:      manager,
		logger:       logger,
		migInstances: make(map[string]*MIGInstance),
	}
}

// CreateMIGInstance 创建MIG实例
func (mgm *MIGGPUManager) CreateMIGInstance(ctx context.Context, gpuIndex int, profile string) (*MIGInstance, error) {
	// 检查GPU是否支持MIG
	cmd := exec.CommandContext(ctx, "nvidia-smi", "-i", fmt.Sprintf("%d", gpuIndex), "--query-gpu=mig.mode.current", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("检查MIG状态失败: %w", err)
	}

	if !strings.Contains(string(output), "Enabled") {
		// 启用MIG模式
		enableCmd := exec.CommandContext(ctx, "nvidia-smi", "-i", fmt.Sprintf("%d", gpuIndex), "-mig", "1")
		if err := enableCmd.Run(); err != nil {
			return nil, fmt.Errorf("启用MIG模式失败: %w", err)
		}
	}

	// 创建MIG GPU实例
	giCmd := exec.CommandContext(ctx, "nvidia-smi", "mig", "-cgi", profile, "-i", fmt.Sprintf("%d", gpuIndex))
	giOutput, err := giCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("创建MIG GI失败: %w", err)
	}

	// 解析GI ID
	gi := mgm.parseGIID(string(giOutput))

	// 创建MIG计算实例
	ciCmd := exec.CommandContext(ctx, "nvidia-smi", "mig", "-cci", "-gi", fmt.Sprintf("%d", gi), "-i", fmt.Sprintf("%d", gpuIndex))
	ciOutput, err := ciCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("创建MIG CI失败: %w", err)
	}

	// 解析CI ID
	ci := mgm.parseCIID(string(ciOutput))

	// 获取profile显存大小
	memoryMB := mgm.getProfileMemory(profile)

	instance := &MIGInstance{
		GPUID:     fmt.Sprintf("nvidia%d", gpuIndex),
		GPUIndex:  gpuIndex,
		GI:        gi,
		CI:        ci,
		Profile:   profile,
		MemoryMB:  memoryMB,
		CreatedAt: time.Now(),
	}

	// 保存实例
	key := fmt.Sprintf("%d-%d-%d", gpuIndex, gi, ci)
	mgm.mu.Lock()
	mgm.migInstances[key] = instance
	mgm.mu.Unlock()

	mgm.logger.Info("MIG实例创建",
		Field{Key: "gpuIndex", Value: gpuIndex},
		Field{Key: "gi", Value: gi},
		Field{Key: "ci", Value: ci},
		Field{Key: "profile", Value: profile})

	return instance, nil
}

// DestroyMIGInstance 销毁MIG实例
func (mgm *MIGGPUManager) DestroyMIGInstance(ctx context.Context, gpuIndex, gi, ci int) error {
	// 销毁计算实例
	ciCmd := exec.CommandContext(ctx, "nvidia-smi", "mig", "-dci", "-gi", fmt.Sprintf("%d", gi), "-i", fmt.Sprintf("%d", gpuIndex))
	if err := ciCmd.Run(); err != nil {
		mgm.logger.Warn("销毁MIG CI失败", Field{Key: "error", Value: err})
	}

	// 销毁GPU实例
	giCmd := exec.CommandContext(ctx, "nvidia-smi", "mig", "-dgi", "-gi", fmt.Sprintf("%d", gi), "-i", fmt.Sprintf("%d", gpuIndex))
	if err := giCmd.Run(); err != nil {
		return fmt.Errorf("销毁MIG GI失败: %w", err)
	}

	// 删除实例记录
	key := fmt.Sprintf("%d-%d-%d", gpuIndex, gi, ci)
	mgm.mu.Lock()
	delete(mgm.migInstances, key)
	mgm.mu.Unlock()

	mgm.logger.Info("MIG实例销毁",
		Field{Key: "gpuIndex", Value: gpuIndex},
		Field{Key: "gi", Value: gi},
		Field{Key: "ci", Value: ci})

	return nil
}

// ListMIGInstances 列出MIG实例
func (mgm *MIGGPUManager) ListMIGInstances(gpuIndex int) []*MIGInstance {
	mgm.mu.RLock()
	defer mgm.mu.RUnlock()

	var result []*MIGInstance
	for key, instance := range mgm.migInstances {
		if gpuIndex < 0 || strings.HasPrefix(key, fmt.Sprintf("%d-", gpuIndex)) {
			result = append(result, instance)
		}
	}

	return result
}

// parseGIID 解析GI ID
func (mgm *MIGGPUManager) parseGIID(output string) int {
	// 输出格式: "GPU instance ID 0 created"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "GPU instance ID") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "ID" && i+1 < len(parts) {
					var id int
					_, _ = fmt.Sscanf(parts[i+1], "%d", &id)
					return id
				}
			}
		}
	}
	return 0
}

// parseCIID 解析CI ID
func (mgm *MIGGPUManager) parseCIID(output string) int {
	// 输出格式: "Compute instance ID 0 created"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Compute instance ID") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "ID" && i+1 < len(parts) {
					var id int
					_, _ = fmt.Sscanf(parts[i+1], "%d", &id)
					return id
				}
			}
		}
	}
	return 0
}

// getProfileMemory 获取profile对应的显存大小
func (mgm *MIGGPUManager) getProfileMemory(profile string) uint64 {
	// A100 MIG Profiles
	switch profile {
	case "1g.5gb":
		return 5120
	case "2g.10gb":
		return 10240
	case "3g.20gb":
		return 20480
	case "4g.20gb":
		return 20480
	case "7g.40gb":
		return 40960
	default:
		return 0
	}
}

// GetMIGInstanceID 获取MIG实例ID（用于CUDA_VISIBLE_DEVICES）
func (mgm *MIGGPUManager) GetMIGInstanceID(instance *MIGInstance) string {
	return fmt.Sprintf("MIG-%d/%d/%d", instance.GPUIndex, instance.GI, instance.CI)
}