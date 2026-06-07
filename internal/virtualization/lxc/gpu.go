// Package lxc GPU透传支持
package lxc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GPUPassthroughConfig GPU透传配置
type GPUPassthroughConfig struct {
	// GPU设备配置
	GPUID      string `json:"gpuId"`      // GPU设备ID (如 nvidia0)
	GPUIndex   int    `json:"gpuIndex"`   // GPU索引 (0, 1, 2...)
	GPUUUID    string `json:"gpuUuid"`    // GPU UUID
	PCIAddress string `json:"pciAddress"` // PCI地址 (如 0000:01:00.0)
	Vendor     string `json:"vendor"`     // 厂商 (nvidia, amd, intel)

	// 透传模式
	Mode       GPUMode `json:"mode"`       // 透传模式
	AccessType string  `json:"accessType"` // 访问类型

	// 资源限制
	MemoryLimit  uint64 `json:"memoryLimit"`  // 显存限制(MB)
	ComputeLimit uint64 `json:"computeLimit"` // 计算限制(0-100%)

	// NVIDIA特定配置
	NVIDIAConfig *NVIDIAGPUConfig `json:"nvidiaConfig,omitempty"`

	// AMD特定配置
	AMDConfig *AMDGPUConfig `json:"amdConfig,omitempty"`

	// Intel特定配置
	IntelConfig *IntelGPUConfig `json:"intelConfig,omitempty"`
}

// GPUMode GPU透传模式
type GPUMode string

const (
	// GPUModeFull 完全透传 - 设备完全分配给容器
	GPUModeFull GPUMode = "full"
	// GPUModeShared 共享模式 - 多容器共享GPU
	GPUModeShared GPUMode = "shared"
	// GPUModeVirtual 虚拟GPU - 使用vGPU技术
	GPUModeVirtual GPUMode = "virtual"
	// GPUModeMIG MIG模式 - NVIDIA Multi-Instance GPU
	GPUModeMIG GPUMode = "mig"
	// GPUModeVFIO VFIO透传 - 使用VFIO驱动
	GPUModeVFIO GPUMode = "vfio"
)

// NVIDIAGPUConfig NVIDIA GPU特定配置
type NVIDIAGPUConfig struct {
	// CUDA配置
	CUDAVersion  string `json:"cudaVersion"`  // CUDA版本
	EnableCUDA   bool   `json:"enableCuda"`   // 启用CUDA
	EnableNVENC  bool   `json:"enableNvenc"`  // 启用NVENC编码
	EnableNVDEC  bool   `json:"enableNvdec"`  // 启用NVDEC解码
	EnableNVDISP bool   `json:"enableNvdisp"` // 启用显示功能

	// MIG配置
	MIGEnabled  bool     `json:"migEnabled"`  // 启用MIG
	MIGGI       int      `json:"migGi"`       // MIG GPU实例ID
	MIGCI       int      `json:"migCi"`       // MIG计算实例ID
	MIGProfiles []string `json:"migProfiles"` // MIG profile列表

	// MPS配置
	MPSEnabled    bool   `json:"mpsEnabled"`    // 启用MPS
	MPSPipeDir    string `json:"mpsPipeDir"`    // MPS管道目录
	MPSLogDir     string `json:"mpsLogDir"`     // MPS日志目录
	MPSThreadPool int    `json:"mpsThreadPool"` // MPS线程池大小

	// 驱动挂载
	MountDriver   bool   `json:"mountDriver"`   // 挂载驱动库
	MountCUDA     bool   `json:"mountCuda"`     // 挂载CUDA库
	MountNVML     bool   `json:"mountNvml"`     // 挂载NVML库
	DriverVersion string `json:"driverVersion"` // 驱动版本
}

// AMDGPUConfig AMD GPU特定配置
type AMDGPUConfig struct {
	// ROCm配置
	ROCmVersion  string `json:"rocmVersion"`  // ROCm版本
	EnableROCm   bool   `json:"enableRocm"`   // 启用ROCm
	EnableHIP    bool   `json:"enableHip"`    // 启用HIP
	EnableOpenCL bool   `json:"enableOpencl"` // 启用OpenCL

	// VRAM配置
	VRAMLimit uint64 `json:"vramLimit"` // VRAM限制(MB)
	SRAMLimit uint64 `json:"sramLimit"` // SRAM限制(MB)

	// 驱动挂载
	MountDriver   bool   `json:"mountDriver"`   // 挂载amdgpu驱动
	MountROCm     bool   `json:"mountRocm"`     // 挂载ROCm库
	DriverVersion string `json:"driverVersion"` // 驱动版本
}

// IntelGPUConfig Intel GPU特定配置
type IntelGPUConfig struct {
	// OneAPI配置
	OneAPIVersion string `json:"oneapiVersion"` // OneAPI版本
	EnableLevel0  bool   `json:"enableLevel0"`  // 启用Level Zero
	EnableOpenCL  bool   `json:"enableOpencl"`  // 启用OpenCL
	EnableQSV     bool   `json:"enableQsv"`     // 启用QuickSync Video

	// 驱动挂载
	MountDriver   bool   `json:"mountDriver"`   // 挂载i915驱动
	MountOneAPI   bool   `json:"mountOneapi"`   // 挂载OneAPI库
	DriverVersion string `json:"driverVersion"` // 驱动版本
}

// GPUDeviceInfo GPU设备信息
type GPUDeviceInfo struct {
	ID           string  `json:"id"`
	Index        int     `json:"index"`
	UUID         string  `json:"uuid"`
	PCIAddress   string  `json:"pciAddress"`
	Vendor       string  `json:"vendor"`
	Model        string  `json:"model"`
	Driver       string  `json:"driver"`
	MemoryTotal  uint64  `json:"memoryTotal"` // 总显存(MB)
	MemoryUsed   uint64  `json:"memoryUsed"`  // 已用显存(MB)
	Status       string  `json:"status"`
	AttachedTo   string  `json:"attachedTo"`   // 已附加的容器
	Mode         GPUMode `json:"mode"`         // 当前透传模式
	MIGAvailable bool    `json:"migAvailable"` // MIG可用
}

// AttachGPU 将GPU附加到LXC容器
func (m *Manager) AttachGPU(ctx context.Context, containerName string, config *GPUPassthroughConfig) error {
	// 验证GPU设备
	if err := m.validateGPUDevice(config); err != nil {
		return fmt.Errorf("GPU设备验证失败: %w", err)
	}

	// 检查容器是否存在且运行
	container, err := m.GetContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("获取容器信息失败: %w", err)
	}

	if container.Status != StatusRunning {
		return fmt.Errorf("容器 %s 未运行，无法附加GPU", containerName)
	}

	// 根据厂商生成设备配置
	deviceConfig := m.buildGPUDeviceConfig(config)

	// 使用lxc config device add添加GPU设备
	args := []string{"config", "device", "add", containerName, config.getDeviceName(), deviceConfig.Type}
	for k, v := range deviceConfig.Config {
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}

	cmd := m.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("附加GPU失败: %w, output: %s", err, string(output))
	}

	// 添加GPU相关配置项
	if config.MemoryLimit > 0 {
		m.UpdateContainer(ctx, containerName, map[string]string{
			"nvidia.driver.capabilities": "compute,utility",
		})
	}

	// 处理厂商特定配置
	switch config.Vendor {
	case "nvidia":
		return m.configureNVIDIAGPU(ctx, containerName, config)
	case "amd":
		return m.configureAMDGPU(ctx, containerName, config)
	case "intel":
		return m.configureIntelGPU(ctx, containerName, config)
	}

	return nil
}

// DetachGPU 从LXC容器分离GPU
func (m *Manager) DetachGPU(ctx context.Context, containerName, gpuID string) error {
	args := []string{"config", "device", "remove", containerName, gpuID}

	cmd := m.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("分离GPU失败: %w, output: %s", err, string(output))
	}

	return nil
}

// ListGPUDevices 列出可用GPU设备
func (m *Manager) ListGPUDevices(ctx context.Context) ([]GPUDeviceInfo, error) {
	var devices []GPUDeviceInfo

	// 检测NVIDIA GPU
	nvidiaDevices, err := m.detectNVIDIAGPUs(ctx)
	if err == nil {
		devices = append(devices, nvidiaDevices...)
	}

	// 检测AMD GPU
	amdDevices, err := m.detectAMDGPUs(ctx)
	if err == nil {
		devices = append(devices, amdDevices...)
	}

	// 检测Intel GPU
	intelDevices, err := m.detectIntelGPUs(ctx)
	if err == nil {
		devices = append(devices, intelDevices...)
	}

	return devices, nil
}

// GetContainerGPUStatus 获取容器的GPU状态
func (m *Manager) GetContainerGPUStatus(ctx context.Context, containerName string) ([]GPUDeviceInfo, error) {
	container, err := m.GetContainer(ctx, containerName)
	if err != nil {
		return nil, fmt.Errorf("获取容器信息失败: %w", err)
	}

	var gpuDevices []GPUDeviceInfo
	for name, device := range container.Devices {
		if device.Type == "gpu" || strings.HasPrefix(name, "gpu") {
			info := GPUDeviceInfo{
				ID:     name,
				Vendor: device.Config["vendor"],
				Mode:   GPUMode(device.Config["mode"]),
			}
			if idx, ok := device.Config["gpuindex"]; ok {
				info.Index = parseInt(idx)
			}
			if uuid, ok := device.Config["gpuuuid"]; ok {
				info.UUID = uuid
			}
			gpuDevices = append(gpuDevices, info)
		}
	}

	return gpuDevices, nil
}

// validateGPUDevice 验证GPU设备
func (m *Manager) validateGPUDevice(config *GPUPassthroughConfig) error {
	if config.GPUID == "" && config.GPUIndex < 0 && config.GPUUUID == "" && config.PCIAddress == "" {
		return fmt.Errorf("必须指定GPUID、GPUIndex、GPUUUID或PCIAddress")
	}

	// 验证厂商
	switch config.Vendor {
	case "nvidia", "amd", "intel":
		break
	default:
		if config.Vendor == "" {
			config.Vendor = "nvidia" // 默认NVIDIA
		} else {
			return fmt.Errorf("不支持的GPU厂商: %s", config.Vendor)
		}
	}

	// 验证透传模式
	switch config.Mode {
	case GPUModeFull, GPUModeShared, GPUModeVirtual, GPUModeMIG, GPUModeVFIO:
		break
	default:
		config.Mode = GPUModeShared // 默认共享模式
	}

	return nil
}

// buildGPUDeviceConfig 构建GPU设备配置
func (m *Manager) buildGPUDeviceConfig(config *GPUPassthroughConfig) Device {
	device := Device{
		Type:   "gpu",
		Config: make(map[string]string),
	}

	// 设置GPU标识
	if config.GPUIndex >= 0 {
		device.Config["gpuindex"] = fmt.Sprintf("%d", config.GPUIndex)
	}
	if config.GPUUUID != "" {
		device.Config["gpuuuid"] = config.GPUUUID
	}
	if config.PCIAddress != "" {
		device.Config["pciaddress"] = config.PCIAddress
	}

	// 设置厂商
	device.Config["vendor"] = config.Vendor

	// 设置模式
	device.Config["mode"] = string(config.Mode)

	// 资源限制
	if config.MemoryLimit > 0 {
		device.Config["memorylimit"] = fmt.Sprintf("%dMB", config.MemoryLimit)
	}
	if config.ComputeLimit > 0 {
		device.Config["computelimit"] = fmt.Sprintf("%d", config.ComputeLimit)
	}

	// NVIDIA特定配置
	if config.Vendor == "nvidia" && config.NVIDIAConfig != nil {
		m.buildNVIDIADeviceConfig(config.NVIDIAConfig, device.Config)
	}

	// AMD特定配置
	if config.Vendor == "amd" && config.AMDConfig != nil {
		m.buildAMDDeviceConfig(config.AMDConfig, device.Config)
	}

	// Intel特定配置
	if config.Vendor == "intel" && config.IntelConfig != nil {
		m.buildIntelDeviceConfig(config.IntelConfig, device.Config)
	}

	return device
}

// buildNVIDIADeviceConfig 构建NVIDIA设备配置
func (m *Manager) buildNVIDIADeviceConfig(config *NVIDIAGPUConfig, deviceConfig map[string]string) {
	// CUDA能力
	capabilities := "utility"
	if config.EnableCUDA {
		capabilities += ",compute"
	}
	if config.EnableNVENC {
		capabilities += ",video"
	}
	if config.EnableNVDEC {
		capabilities += ",video.decode"
	}
	if config.EnableNVDISP {
		capabilities += ",display"
	}
	deviceConfig["nvidia.driver.capabilities"] = capabilities

	// CUDA版本
	if config.CUDAVersion != "" {
		deviceConfig["nvidia.cuda.version"] = config.CUDAVersion
	}

	// MIG配置
	if config.MIGEnabled {
		deviceConfig["nvidia.mig.enabled"] = "true"
		if config.MIGGI >= 0 {
			deviceConfig["nvidia.mig.gi"] = fmt.Sprintf("%d", config.MIGGI)
		}
		if config.MIGCI >= 0 {
			deviceConfig["nvidia.mig.ci"] = fmt.Sprintf("%d", config.MIGCI)
		}
	}

	// MPS配置
	if config.MPSEnabled {
		deviceConfig["nvidia.mps.enabled"] = "true"
		if config.MPSPipeDir != "" {
			deviceConfig["nvidia.mps.pipedir"] = config.MPSPipeDir
		}
		if config.MPSThreadPool > 0 {
			deviceConfig["nvidia.mps.threadpool"] = fmt.Sprintf("%d", config.MPSThreadPool)
		}
	}

	// 驱动挂载
	if config.DriverVersion != "" {
		deviceConfig["nvidia.driver.version"] = config.DriverVersion
	}
}

// buildAMDDeviceConfig 构建AMD设备配置
func (m *Manager) buildAMDDeviceConfig(config *AMDGPUConfig, deviceConfig map[string]string) {
	// ROCm能力
	if config.EnableROCm {
		deviceConfig["amd.rocm.enabled"] = "true"
	}
	if config.EnableHIP {
		deviceConfig["amd.hip.enabled"] = "true"
	}
	if config.EnableOpenCL {
		deviceConfig["amd.opencl.enabled"] = "true"
	}

	// VRAM限制
	if config.VRAMLimit > 0 {
		deviceConfig["amd.vram.limit"] = fmt.Sprintf("%dMB", config.VRAMLimit)
	}

	// ROCm版本
	if config.ROCmVersion != "" {
		deviceConfig["amd.rocm.version"] = config.ROCmVersion
	}
}

// buildIntelDeviceConfig 构建Intel设备配置
func (m *Manager) buildIntelDeviceConfig(config *IntelGPUConfig, deviceConfig map[string]string) {
	// OneAPI能力
	if config.EnableLevel0 {
		deviceConfig["intel.level0.enabled"] = "true"
	}
	if config.EnableOpenCL {
		deviceConfig["intel.opencl.enabled"] = "true"
	}
	if config.EnableQSV {
		deviceConfig["intel.qsv.enabled"] = "true"
	}

	// OneAPI版本
	if config.OneAPIVersion != "" {
		deviceConfig["intel.oneapi.version"] = config.OneAPIVersion
	}
}

// configureNVIDIAGPU 配置NVIDIA GPU
func (m *Manager) configureNVIDIAGPU(ctx context.Context, containerName string, config *GPUPassthroughConfig) error {
	if config.NVIDIAConfig == nil {
		return nil
	}

	// 添加环境变量
	envVars := map[string]string{}
	envVars["NVIDIA_VISIBLE_DEVICES"] = m.getNVIDIAVisibleDevices(config)
	envVars["NVIDIA_DRIVER_CAPABILITIES"] = "compute,utility"

	// MIG环境变量
	if config.NVIDIAConfig.MIGEnabled {
		envVars["NVIDIA_MIG_GPU_DEVICES"] = fmt.Sprintf("%d:%d", config.NVIDIAConfig.MIGGI, config.NVIDIAConfig.MIGCI)
	}

	// MPS环境变量
	if config.NVIDIAConfig.MPSEnabled {
		envVars["CUDA_MPS_PIPE_DIRECTORY"] = config.NVIDIAConfig.MPSPipeDir
		if config.NVIDIAConfig.MPSPipeDir == "" {
			envVars["CUDA_MPS_PIPE_DIRECTORY"] = "/tmp/nvidia-mps"
		}
		envVars["CUDA_MPS_LOG_DIRECTORY"] = config.NVIDIAConfig.MPSLogDir
		if config.NVIDIAConfig.MPSLogDir == "" {
			envVars["CUDA_MPS_LOG_DIRECTORY"] = "/tmp/nvidia-mps-log"
		}
	}

	// 设置环境变量
	for k, v := range envVars {
		key := fmt.Sprintf("environment.%s", k)
		m.UpdateContainer(ctx, containerName, map[string]string{key: v})
	}

	// 挂载CUDA库（如果需要）
	if config.NVIDIAConfig.MountCUDA && config.NVIDIAConfig.CUDAVersion != "" {
		cudaPath := fmt.Sprintf("/usr/local/cuda-%s", config.NVIDIAConfig.CUDAVersion)
		args := []string{"config", "device", "add", containerName, "cuda-lib", "disk"}
		args = append(args, fmt.Sprintf("source=%s", cudaPath), fmt.Sprintf("path=%s", cudaPath))
		cmd := m.cmd(args...)
		cmd.Run() // 忽略错误，可能已经挂载
	}

	return nil
}

// configureAMDGPU 配置AMD GPU
func (m *Manager) configureAMDGPU(ctx context.Context, containerName string, config *GPUPassthroughConfig) error {
	if config.AMDConfig == nil {
		return nil
	}

	envVars := map[string]string{}

	// ROCm环境变量
	if config.AMDConfig.EnableROCm {
		envVars["ROCm_PATH"] = "/opt/rocm"
		envVars["HIP_PATH"] = "/opt/rocm/hip"
	}

	// VRAM限制环境变量
	if config.AMDConfig.VRAMLimit > 0 {
		envVars["GPU_MAX_VRAM_ALLOC_SIZE"] = fmt.Sprintf("%d", config.AMDConfig.VRAMLimit*1024*1024)
	}

	// 设置环境变量
	for k, v := range envVars {
		key := fmt.Sprintf("environment.%s", k)
		m.UpdateContainer(ctx, containerName, map[string]string{key: v})
	}

	// 挂载ROCm库
	if config.AMDConfig.MountROCm && config.AMDConfig.ROCmVersion != "" {
		rocmPath := fmt.Sprintf("/opt/rocm-%s", config.AMDConfig.ROCmVersion)
		args := []string{"config", "device", "add", containerName, "rocm-lib", "disk"}
		args = append(args, fmt.Sprintf("source=%s", rocmPath), fmt.Sprintf("path=/opt/rocm"))
		cmd := m.cmd(args...)
		cmd.Run()
	}

	return nil
}

// configureIntelGPU 配置Intel GPU
func (m *Manager) configureIntelGPU(ctx context.Context, containerName string, config *GPUPassthroughConfig) error {
	if config.IntelConfig == nil {
		return nil
	}

	envVars := map[string]string{}

	// OneAPI环境变量
	if config.IntelConfig.EnableLevel0 {
		envVars["ONEAPI_DEVICE_SELECTOR"] = "level_zero:gpu"
	}
	if config.IntelConfig.EnableOpenCL {
		envVars["OCL_ICD_VENDORS"] = "/etc/OpenCL/vendors/intel.icd"
	}

	// 设置环境变量
	for k, v := range envVars {
		key := fmt.Sprintf("environment.%s", k)
		m.UpdateContainer(ctx, containerName, map[string]string{key: v})
	}

	// 挂载OneAPI库
	if config.IntelConfig.MountOneAPI && config.IntelConfig.OneAPIVersion != "" {
		oneapiPath := fmt.Sprintf("/opt/intel/oneapi-%s", config.IntelConfig.OneAPIVersion)
		args := []string{"config", "device", "add", containerName, "oneapi-lib", "disk"}
		args = append(args, fmt.Sprintf("source=%s", oneapiPath), fmt.Sprintf("path=/opt/intel/oneapi"))
		cmd := m.cmd(args...)
		cmd.Run()
	}

	return nil
}

// getNVIDIAVisibleDevices 获取NVIDIA可见设备列表
func (m *Manager) getNVIDIAVisibleDevices(config *GPUPassthroughConfig) string {
	if config.GPUUUID != "" {
		return config.GPUUUID
	}
	if config.GPUIndex >= 0 {
		return fmt.Sprintf("%d", config.GPUIndex)
	}
	if config.PCIAddress != "" {
		// 需要查询对应的UUID
		return config.PCIAddress
	}
	return "all"
}

// getDeviceName 获取设备名称
func (config *GPUPassthroughConfig) getDeviceName() string {
	if config.GPUID != "" {
		return config.GPUID
	}
	if config.GPUIndex >= 0 {
		return fmt.Sprintf("gpu%d", config.GPUIndex)
	}
	if config.PCIAddress != "" {
		return fmt.Sprintf("gpu-%s", strings.ReplaceAll(config.PCIAddress, ":", "-"))
	}
	return "gpu0"
}

// detectNVIDIAGPUs 检测NVIDIA GPU
func (m *Manager) detectNVIDIAGPUs(ctx context.Context) ([]GPUDeviceInfo, error) {
	var devices []GPUDeviceInfo

	// 检查nvidia-smi是否可用
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=index,uuid,name,memory.total,memory.used,pci.bus_id,driver_version", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi执行失败: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ", ")
		if len(parts) < 7 {
			continue
		}

		device := GPUDeviceInfo{
			Vendor:      "nvidia",
			Index:       parseInt(parts[0]),
			UUID:        strings.TrimSpace(parts[1]),
			Model:       strings.TrimSpace(parts[2]),
			MemoryTotal: parseMemorySize(parts[3]),
			MemoryUsed:  parseMemorySize(parts[4]),
			PCIAddress:  strings.TrimSpace(parts[5]),
			Driver:      strings.TrimSpace(parts[6]),
			ID:          fmt.Sprintf("nvidia%d", parseInt(parts[0])),
			Status:      "available",
			Mode:        GPUModeShared,
		}

		// 检查MIG支持
		migCmd := exec.CommandContext(ctx, "nvidia-smi", "-i", parts[0], "--query-gpu=mig.mode.current", "--format=csv,noheader")
		migOutput, err := migCmd.Output()
		if err == nil && strings.Contains(string(migOutput), "Enabled") {
			device.MIGAvailable = true
		}

		devices = append(devices, device)
	}

	return devices, nil
}

// detectAMDGPUs 检测AMD GPU
func (m *Manager) detectAMDGPUs(ctx context.Context) ([]GPUDeviceInfo, error) {
	var devices []GPUDeviceInfo

	// 使用rocm-smi检测
	cmd := exec.CommandContext(ctx, "rocm-smi", "--showid", "--showname", "--showmeminfo", "vram", "--showdriverversion")
	output, err := cmd.Output()
	if err != nil {
		// 如果rocm-smi不可用，尝试lspci
		return m.detectAMDGPUsByLSPCI(ctx)
	}

	lines := strings.Split(string(output), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, "GPU") && strings.Contains(line, "Card") {
			device := GPUDeviceInfo{
				Vendor: "amd",
				Status: "available",
				Mode:   GPUModeShared,
			}

			// 解析GPU信息
			for j := i; j < len(lines) && j < i+10; j++ {
				l := lines[j]
				if strings.Contains(l, "GPU ID:") {
					device.ID = strings.TrimSpace(strings.Split(l, ":")[1])
				}
				if strings.Contains(l, "Card Series:") {
					device.Model = strings.TrimSpace(strings.Split(l, ":")[1])
				}
				if strings.Contains(l, "VRAM Total") {
					device.MemoryTotal = parseMemorySize(l)
				}
				if strings.Contains(l, "Driver version:") {
					device.Driver = strings.TrimSpace(strings.Split(l, ":")[1])
				}
			}

			if device.ID != "" {
				devices = append(devices, device)
			}
		}
	}

	return devices, nil
}

// detectAMDGPUsByLSPCI 通过lspci检测AMD GPU
func (m *Manager) detectAMDGPUsByLSPCI(ctx context.Context) ([]GPUDeviceInfo, error) {
	var devices []GPUDeviceInfo

	cmd := exec.CommandContext(ctx, "lspci", "-nn", "-d", "::0300")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if strings.Contains(line, "AMD") || strings.Contains(line, "Advanced Micro Devices") {
			device := GPUDeviceInfo{
				ID:         fmt.Sprintf("amd%d", i),
				Index:      i,
				Vendor:     "amd",
				PCIAddress: strings.Fields(line)[0],
				Model:      strings.TrimSpace(line[strings.Index(line, "]")+2:]),
				Status:     "available",
				Mode:       GPUModeShared,
			}
			devices = append(devices, device)
		}
	}

	return devices, nil
}

// detectIntelGPUs 检测Intel GPU
func (m *Manager) detectIntelGPUs(ctx context.Context) ([]GPUDeviceInfo, error) {
	var devices []GPUDeviceInfo

	// 检查Intel GPU设备文件
	intelGPUPaths := []string{
		"/dev/dri/card0",
		"/dev/dri/card1",
		"/dev/dri/renderD128",
		"/dev/dri/renderD129",
	}

	for i, path := range intelGPUPaths {
		if _, err := os.Stat(path); err == nil {
			device := GPUDeviceInfo{
				ID:     fmt.Sprintf("intel%d", i),
				Index:  i,
				Vendor: "intel",
				Model:  "Intel Integrated GPU",
				Status: "available",
				Mode:   GPUModeShared,
				Driver: "i915",
			}

			// 获取更多信息
			cmd := exec.CommandContext(ctx, "lspci", "-nn", "-d", "::0300")
			output, err := cmd.Output()
			if err == nil {
				lines := strings.Split(string(output), "\n")
				for _, line := range lines {
					if strings.Contains(line, "Intel") {
						device.Model = strings.TrimSpace(line[strings.Index(line, "]")+2:])
						device.PCIAddress = strings.Fields(line)[0]
						break
					}
				}
			}

			devices = append(devices, device)
		}
	}

	return devices, nil
}

// parseInt 解析整数
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

// parseMemorySize 解析内存大小
func parseMemorySize(s string) uint64 {
	s = strings.TrimSpace(strings.ToLower(s))

	var value uint64
	var unit string

	_, _ = fmt.Sscanf(s, "%d%s", &value, &unit)

	switch unit {
	case "kb", "kib":
		return value / 1024
	case "mb", "mib":
		return value
	case "gb", "gib":
		return value * 1024
	default:
		// 尝试解析为MiB格式 (如 "8192 MiB")
		parts := strings.Fields(s)
		if len(parts) >= 2 {
			var v uint64
			n, _ := fmt.Sscanf(parts[0], "%d", &v)
			if n == 1 && strings.Contains(parts[1], "Mi") {
				return v
			}
			if n == 1 && strings.Contains(parts[1], "Gi") {
				return v * 1024
			}
		}
		return value
	}
}
