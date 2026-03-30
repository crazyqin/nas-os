// Package gpu 容器GPU挂载支持
package gpu

import (
	"fmt"
	"os"
	"strings"
)

// ContainerGPUConfig 容器GPU配置
type ContainerGPUConfig struct {
	// GPU设备分配
	GPUIDs         []string `json:"gpuIds"`         // GPU设备ID列表
	GPUIndices     []int    `json:"gpuIndices"`     // GPU索引列表 (如0,1)
	GPUUUIDs       []string `json:"gpuUuids"`       // GPU UUID列表
	GPUAll         bool     `json:"gpuAll"`         // 使用所有可用GPU

	// 资源限制
	MemoryLimit    uint64   `json:"memoryLimit"`    // 显存限制(MB), 0表示无限制
	ComputeLimit   uint64   `json:"computeLimit"`   // 计算限制(%), 0-100, 0表示无限制
	EnableMPS      bool     `json:"enableMps"`      // 启用MPS (Multi-Process Service)

	// 设备挂载
	DevicePaths    []string `json:"devicePaths"`    // 自定义设备路径
	IncludeUVM     bool     `json:"includeUvm"`     // 包含NVIDIA UVM设备
	IncludeCtl     bool     `json:"includeCtl"`     // 包含nvidiactl设备
	IncludeModeset bool     `json:"includeModeset"` // 包含nvidia-modeset设备

	// 驱动和库挂载
	DriverVersion  string   `json:"driverVersion"`  // 驱动版本(可选)
	CUDAVersion    string   `json:"cudaVersion"`    // CUDA版本(可选)
	MountDriver    bool     `json:"mountDriver"`    // 挂载驱动库
	MountCUDA      bool     `json:"mountCuda"`      // 挂载CUDA库
	CustomLibs     []string `json:"customLibs"`     // 自定义库路径

	// 环境变量
	EnvVars        map[string]string `json:"envVars"` // 自定义环境变量

	// NVIDIA Container Toolkit配置
	NvidiaRuntime  bool     `json:"nvidiaRuntime"` // 使用nvidia运行时
	NvidiaCDI      bool     `json:"nvidiaCdi"`     // 使用CDI规范
	CDIAnnotation  string   `json:"cdiAnnotation"` // CDI注解
}

// DefaultContainerGPUConfig 默认容器GPU配置
func DefaultContainerGPUConfig() *ContainerGPUConfig {
	return &ContainerGPUConfig{
		GPUAll:         false,
		MemoryLimit:    0,
		ComputeLimit:   0,
		IncludeUVM:     true,
		IncludeCtl:     true,
		IncludeModeset: false,
		MountDriver:    false,
		MountCUDA:      false,
		NvidiaRuntime:  true,
		NvidiaCDI:      false,
		EnvVars:        make(map[string]string),
	}
}

// GenerateDockerGPUArgs 生成Docker GPU参数
func GenerateDockerGPUArgs(config *ContainerGPUConfig, devices []*GPUDevice) []string {
	if config == nil {
		config = DefaultContainerGPUConfig()
	}

	args := []string{}

	// 使用nvidia运行时
	if config.NvidiaRuntime && !config.NvidiaCDI {
		// Docker 19.03+ 使用 --gpus 参数
		if config.GPUAll {
			args = append(args, "--gpus", "all")
		} else if len(config.GPUIndices) > 0 {
			// 指定GPU索引
			gpuList := make([]string, len(config.GPUIndices))
			for i, idx := range config.GPUIndices {
				gpuList[i] = fmt.Sprintf("%d", idx)
			}
			args = append(args, "--gpus", fmt.Sprintf("device=%s", strings.Join(gpuList, ",")))
		} else if len(config.GPUUUIDs) > 0 {
			// 指定GPU UUID
			args = append(args, "--gpus", fmt.Sprintf("device=%s", strings.Join(config.GPUUUIDs, ",")))
		} else if len(config.GPUIDs) > 0 {
			// 指定GPU设备ID
			args = append(args, "--gpus", strings.Join(config.GPUIDs, ","))
		} else if len(devices) > 0 {
			// 从设备列表自动选择
			gpuIndices := make([]string, len(devices))
			for i, device := range devices {
				gpuIndices[i] = extractGPUIndex(device.ID)
			}
			args = append(args, "--gpus", fmt.Sprintf("device=%s", strings.Join(gpuIndices, ",")))
		}

		// 显存限制
		if config.MemoryLimit > 0 {
			args = append(args, "--gpus", fmt.Sprintf("capabilities=utility,compute,video"),
				"--gpus", fmt.Sprintf("limits.memory=%dMiB", config.MemoryLimit))
		}

		// 计算限制
		if config.ComputeLimit > 0 && config.ComputeLimit < 100 {
			args = append(args, "--gpus", fmt.Sprintf("limits.compute=%d", config.ComputeLimit))
		}
	}

	// CDI模式
	if config.NvidiaCDI {
		// 使用CDI设备注解
		if len(config.GPUIndices) > 0 {
			for _, idx := range config.GPUIndices {
				args = append(args, "--device", fmt.Sprintf("nvidia.com/gpu=%d", idx))
			}
		} else if config.GPUAll {
			args = append(args, "--device", "nvidia.com/gpu=all")
		}
	}

	// 手动设备挂载（不使用NVIDIA Toolkit）
	if !config.NvidiaRuntime && !config.NvidiaCDI {
		// 手动挂载设备
		devicePaths := config.DevicePaths
		if len(devicePaths) == 0 && len(devices) > 0 {
			devicePaths = buildDevicePathsFromConfig(config, devices)
		}

		for _, path := range devicePaths {
			args = append(args, "--device", path)
		}

		// 挂载CUDA库
		if config.MountCUDA && config.CUDAVersion != "" {
			cudaPath := fmt.Sprintf("/usr/local/cuda-%s/lib64", config.CUDAVersion)
			args = append(args, "-v", fmt.Sprintf("%s:%s", cudaPath, cudaPath))
		}

		// 自定义库挂载
		for _, libPath := range config.CustomLibs {
			args = append(args, "-v", fmt.Sprintf("%s:%s", libPath, libPath))
		}
	}

	// 环境变量
	for k, v := range config.EnvVars {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// NVIDIA_VISIBLE_DEVICES 环境变量
	if len(config.GPUIndices) > 0 || len(config.GPUUUIDs) > 0 {
		visibleDevices := ""
		if len(config.GPUIndices) > 0 {
			visibleDevices = strings.Join(config.GPUIndicesToStrings(), ",")
		} else {
			visibleDevices = strings.Join(config.GPUUUIDs, ",")
		}
		args = append(args, "-e", fmt.Sprintf("NVIDIA_VISIBLE_DEVICES=%s", visibleDevices))
	}

	// NVIDIA_DRIVER_CAPABILITIES
	if config.NvidiaRuntime {
		capabilities := "utility,compute"
		args = append(args, "-e", fmt.Sprintf("NVIDIA_DRIVER_CAPABILITIES=%s", capabilities))
	}

	return args
}

// GPUIndicesToStrings 将GPU索引转换为字符串
func (c *ContainerGPUConfig) GPUIndicesToStrings() []string {
	result := make([]string, len(c.GPUIndices))
	for i, idx := range c.GPUIndices {
		result[i] = fmt.Sprintf("%d", idx)
	}
	return result
}

// buildDevicePathsFromConfig 根据配置构建设备路径
func buildDevicePathsFromConfig(config *ContainerGPUConfig, devices []*GPUDevice) []string {
	paths := []string{}

	for _, device := range devices {
		// 主设备
		paths = append(paths, device.DevicePath)

		// UVM设备
		if config.IncludeUVM {
			if _, err := os.Stat("/dev/nvidia-uvm"); err == nil {
				paths = append(paths, "/dev/nvidia-uvm")
			}
			if _, err := os.Stat("/dev/nvidia-uvm-tools"); err == nil {
				paths = append(paths, "/dev/nvidia-uvm-tools")
			}
		}

		// CTL设备
		if config.IncludeCtl {
			if _, err := os.Stat("/dev/nvidiactl"); err == nil {
				paths = append(paths, "/dev/nvidiactl")
			}
		}

		// Modeset设备
		if config.IncludeModeset {
			if _, err := os.Stat("/dev/nvidia-modeset"); err == nil {
				paths = append(paths, "/dev/nvidia-modeset")
			}
		}
	}

	return paths
}

// extractGPUIndex 从GPU ID提取索引
func extractGPUIndex(id string) string {
	// nvidia0 -> 0
	return strings.TrimPrefix(id, "nvidia")
}

// GenerateDockerComposeGPUConfig 生成Docker Compose GPU配置
func GenerateDockerComposeGPUConfig(config *ContainerGPUConfig, devices []*GPUDevice) map[string]interface{} {
	if config == nil {
		config = DefaultContainerGPUConfig()
	}

	gpuConfig := map[string]interface{}{}

	// deploy.resources.reservations.devices
	if config.NvidiaRuntime {
		reservations := map[string]interface{}{}
		devicesConfig := []map[string]interface{}{}

		if config.GPUAll {
			devicesConfig = append(devicesConfig, map[string]interface{}{
				"capabilities": []string{"gpu"},
			})
		} else {
			gpuIndices := config.GPUIndicesToStrings()
			if len(gpuIndices) == 0 && len(devices) > 0 {
				gpuIndices = make([]string, len(devices))
				for i, device := range devices {
					gpuIndices[i] = extractGPUIndex(device.ID)
				}
			}

			if len(gpuIndices) > 0 {
				devicesConfig = append(devicesConfig, map[string]interface{}{
					"capabilities": []string{"gpu"},
					"device_ids":   gpuIndices,
				})
			}
		}

		if config.MemoryLimit > 0 {
			reservations["memory"] = fmt.Sprintf("%dMiB", config.MemoryLimit)
		}

		reservations["devices"] = devicesConfig
		gpuConfig["deploy"] = map[string]interface{}{
			"resources": map[string]interface{}{
				"reservations": reservations,
			},
		}
	}

	// 环境变量
	envVars := map[string]string{}
	for k, v := range config.EnvVars {
		envVars[k] = v
	}

	if len(config.GPUIndices) > 0 {
		envVars["NVIDIA_VISIBLE_DEVICES"] = strings.Join(config.GPUIndicesToStrings(), ",")
	}
	envVars["NVIDIA_DRIVER_CAPABILITIES"] = "utility,compute"

	if len(envVars) > 0 {
		gpuConfig["environment"] = envVars
	}

	// 运行时
	if config.NvidiaRuntime {
		gpuConfig["runtime"] = "nvidia"
	}

	return gpuConfig
}

// ValidateGPUConfig 验证GPU配置
func ValidateGPUConfig(config *ContainerGPUConfig, manager *Manager) error {
	if config == nil {
		return nil
	}

	// 检查GPU索引是否有效
	if len(config.GPUIndices) > 0 {
		availableGPUs := manager.ListGPUs(nil)
		for _, idx := range config.GPUIndices {
			found := false
			for _, gpu := range availableGPUs {
				if extractGPUIndex(gpu.ID) == fmt.Sprintf("%d", idx) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("GPU索引 %d 不存在", idx)
			}
		}
	}

	// 检查GPU UUID是否有效
	if len(config.GPUUUIDs) > 0 {
		availableGPUs := manager.ListGPUs(nil)
		for _, uuid := range config.GPUUUIDs {
			found := false
			for _, gpu := range availableGPUs {
				if gpu.UUID == uuid {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("GPU UUID %s 不存在", uuid)
			}
		}
	}

	// 检查显存限制是否合理
	if config.MemoryLimit > 0 {
		availableGPUs := manager.ListGPUs(nil)
		if len(availableGPUs) > 0 {
			for _, gpu := range availableGPUs {
				if config.MemoryLimit > gpu.MemoryTotal {
					return fmt.Errorf("显存限制 %d MB 超过GPU %s 的总显存 %d MB",
						config.MemoryLimit, gpu.ID, gpu.MemoryTotal)
				}
			}
		}
	}

	// 检查计算限制范围
	if config.ComputeLimit > 100 {
		return fmt.Errorf("计算限制 %d 超出有效范围(0-100)", config.ComputeLimit)
	}

	return nil
}

// MergeContainerGPUConfig 合并容器GPU配置
func MergeContainerGPUConfig(base *ContainerGPUConfig, override *ContainerGPUConfig) *ContainerGPUConfig {
	if base == nil {
		base = DefaultContainerGPUConfig()
	}
	if override == nil {
		return base
	}

	result := &ContainerGPUConfig{
		GPUAll:         base.GPUAll || override.GPUAll,
		GPUIDs:         mergeStrings(base.GPUIDs, override.GPUIDs),
		GPUIndices:     mergeInts(base.GPUIndices, override.GPUIndices),
		GPUUUIDs:       mergeStrings(base.GPUUUIDs, override.GPUUUIDs),
		MemoryLimit:    override.MemoryLimit,
		ComputeLimit:   override.ComputeLimit,
		EnableMPS:      base.EnableMPS || override.EnableMPS,
		DevicePaths:    mergeStrings(base.DevicePaths, override.DevicePaths),
		IncludeUVM:     base.IncludeUVM || override.IncludeUVM,
		IncludeCtl:     base.IncludeCtl || override.IncludeCtl,
		IncludeModeset: base.IncludeModeset || override.IncludeModeset,
		DriverVersion:  override.DriverVersion,
		CUDAVersion:    override.CUDAVersion,
		MountDriver:    base.MountDriver || override.MountDriver,
		MountCUDA:      base.MountCUDA || override.MountCUDA,
		CustomLibs:     mergeStrings(base.CustomLibs, override.CustomLibs),
		NvidiaRuntime:  base.NvidiaRuntime || override.NvidiaRuntime,
		NvidiaCDI:      base.NvidiaCDI || override.NvidiaCDI,
		CDIAnnotation:  override.CDIAnnotation,
		EnvVars:        mergeMaps(base.EnvVars, override.EnvVars),
	}

	// 如果override未设置MemoryLimit，使用base的值
	if override.MemoryLimit == 0 && base.MemoryLimit > 0 {
		result.MemoryLimit = base.MemoryLimit
	}

	return result
}

// mergeStrings 合并字符串数组
func mergeStrings(a, b []string) []string {
	result := a
	for _, s := range b {
		if !containsString(result, s) {
			result = append(result, s)
		}
	}
	return result
}

// mergeInts 合并整数数组
func mergeInts(a, b []int) []int {
	result := a
	for _, i := range b {
		if !containsInt(result, i) {
			result = append(result, i)
		}
	}
	return result
}

// mergeMaps 合并map
func mergeMaps(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

// containsString 检查字符串数组是否包含指定字符串
func containsString(arr []string, s string) bool {
	for _, a := range arr {
		if a == s {
			return true
		}
	}
	return false
}

// containsInt 检查整数数组是否包含指定整数
func containsInt(arr []int, i int) bool {
	for _, a := range arr {
		if a == i {
			return true
		}
	}
	return false
}

// GetGPUContainerRuntime 获取GPU容器运行时名称
func GetGPUContainerRuntime(config *ContainerGPUConfig) string {
	if config == nil || !config.NvidiaRuntime {
		return "runc"
	}

	if config.NvidiaCDI {
		return "nvidia-cdi"
	}

	return "nvidia"
}

// IsGPUConfigured 检查容器是否配置了GPU
func IsGPUConfigured(config *ContainerGPUConfig) bool {
	if config == nil {
		return false
	}

	return config.GPUAll ||
		len(config.GPUIDs) > 0 ||
		len(config.GPUIndices) > 0 ||
		len(config.GPUUUIDs) > 0 ||
		len(config.DevicePaths) > 0
}