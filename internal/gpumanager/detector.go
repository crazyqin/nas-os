package gpumanager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Detector 多厂商GPU检测器.
type Detector struct {
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewDetector 创建GPU检测器.
func NewDetector(logger *slog.Logger) *Detector {
	if logger == nil {
		logger = slog.Default()
	}
	return &Detector{logger: logger}
}

// DetectAll 检测所有GPU设备.
func (d *Detector) DetectAll(ctx context.Context) ([]*GPUDevice, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var allDevices []*GPUDevice

	// 检测NVIDIA GPU
	nvidiaDevices, err := d.detectNVIDIA(ctx)
	if err != nil {
		d.logger.Warn("NVIDIA GPU检测失败", "error", err)
	} else {
		allDevices = append(allDevices, nvidiaDevices...)
	}

	// 检测AMD GPU
	amdDevices, err := d.detectAMD(ctx)
	if err != nil {
		d.logger.Warn("AMD GPU检测失败", "error", err)
	} else {
		allDevices = append(allDevices, amdDevices...)
	}

	// 检测Intel GPU
	intelDevices, err := d.detectIntel(ctx)
	if err != nil {
		d.logger.Warn("Intel GPU检测失败", "error", err)
	} else {
		allDevices = append(allDevices, intelDevices...)
	}

	d.logger.Info("GPU检测完成", "total", len(allDevices),
		"nvidia", len(nvidiaDevices), "amd", len(amdDevices), "intel", len(intelDevices))

	return allDevices, nil
}

// detectNVIDIA 检测NVIDIA GPU (通过nvidia-smi).
func (d *Detector) detectNVIDIA(ctx context.Context) ([]*GPUDevice, error) {
	// 检查nvidia-smi是否可用
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return nil, fmt.Errorf("nvidia-smi未安装")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 使用nvidia-smi查询GPU信息
	// CSV格式: index, uuid, name, driver_version, memory.total, memory.used, memory.free, temperature.gpu, power.draw, power.limit, utilization.gpu, fan.speed, clocks.current.graphics, clocks.current.memory
	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=index,uuid,name,driver_version,memory.total,memory.used,memory.free,temperature.gpu,power.draw,power.limit,utilization.gpu,fan.speed,clocks.current.graphics,clocks.current.memory,pci.bus_id",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi执行失败: %w", err)
	}

	var devices []*GPUDevice
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 15 {
			continue
		}

		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		device := &GPUDevice{
			ID:           fmt.Sprintf("nvidia%s", parts[0]),
			UUID:         parts[1],
			Name:         parts[2],
			FullName:     parts[2],
			Vendor:       VendorNVIDIA,
			Driver:       parts[3],
			DriverOK:     true,
			PCIAddress:   parts[14],
			Status:       StatusHealthy,
			LastUpdated:  time.Now(),
			Capabilities: &GPUCapabilities{},
		}

		// 解析数值
		device.MemoryTotal, _ = parseUint64(parts[4])
		device.MemoryUsed, _ = parseUint64(parts[5])
		device.MemoryFree, _ = parseUint64(parts[6])
		device.Temperature, _ = parseInt(parts[7])
		device.PowerUsage, _ = parseUint64(parts[8])
		device.PowerLimit, _ = parseUint64(parts[9])
		device.Utilization, _ = parseFloat(parts[10])
		device.FanSpeed, _ = parseInt(parts[11])
		device.ClockSM, _ = parseInt(parts[12])
		device.ClockMemory, _ = parseInt(parts[13])

		if device.MemoryTotal > 0 {
			device.MemUtil = float64(device.MemoryUsed) / float64(device.MemoryTotal) * 100
		}

		// 设置设备路径
		idx, _ := strconv.Atoi(parts[0])
		device.DevicePath = fmt.Sprintf("/dev/nvidia%d", idx)
		device.Model = extractGPUModel(device.Name)

		// 检测NVIDIA能力
		d.detectNVIDIACapabilities(ctx, device)

		devices = append(devices, device)
	}

	return devices, nil
}

// detectNVIDIACapabilities 检测NVIDIA GPU能力.
func (d *Detector) detectNVIDIACapabilities(ctx context.Context, device *GPUDevice) {
	caps := device.Capabilities

	// 查询GPU详细信息
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 查询CUDA核心数、计算能力等
	cmd := exec.CommandContext(queryCtx, "nvidia-smi",
		"-i", strings.TrimPrefix(device.ID, "nvidia"),
		"--query-gpu=compute_cap",
		"--format=csv,noheader,nounits")

	if output, err := cmd.Output(); err == nil {
		computeCap := strings.TrimSpace(string(output))
		caps.ComputeCapability = computeCap
		caps.CUDACores = estimateCUDACores(device.Name, computeCap)
	}

	// 推断转码能力 (基于GPU架构)
	caps.TranscodeCapable = true
	caps.TranscodeFormats = []string{"h264", "h265"}
	caps.DecodeFormats = []string{"h264", "h265", "av1", "vp8", "vp9"}
	caps.EncodeFormats = []string{"h264", "h265"}
	caps.MaxEncodeStreams = 3
	caps.MaxDecodeStreams = 3

	// AV1支持 (RTX 30系列及以上)
	if isRTX30OrAbove(device.Name) {
		caps.EncodeFormats = append(caps.EncodeFormats, "av1")
		caps.MaxEncodeStreams = 3
	}

	// 推断AI推理能力
	caps.InferenceCapable = true
	caps.InferenceBackends = []string{"cuda", "tensorrt"}
	caps.HalfPrecision = true
	caps.DoublePrecision = true
	caps.Passthrough = true

	// Tensor核心 (Volta及以上架构)
	caps.TensorCores = estimateTensorCores(device.Name, caps.ComputeCapability)
	caps.RTCores = estimateRTCores(device.Name)

	// 显存相关能力
	if device.MemoryTotal > 0 {
		caps.MaxMemoryAlloc = device.MemoryTotal * 90 / 100 // 90%
	}

	caps.MaxWorkGroupSize = 1024
}

// detectAMD 检测AMD GPU (通过ROCm工具或sysfs).
func (d *Detector) detectAMD(ctx context.Context) ([]*GPUDevice, error) {
	var devices []*GPUDevice

	// 方法1: 通过rocm-smi检测
	rocmDevices, err := d.detectAMDViaROCm(ctx)
	if err == nil {
		devices = append(devices, rocmDevices...)
	}

	// 方法2: 通过sysfs检测 (无需ROCm)
	if len(devices) == 0 {
		sysfsDevices, err := d.detectAMDViaSysfs(ctx)
		if err == nil {
			devices = append(devices, sysfsDevices...)
		}
	}

	// 方法3: 通过lspci检测
	if len(devices) == 0 {
		lspciDevices, err := d.detectAMDViaLspci(ctx)
		if err == nil {
			devices = append(devices, lspciDevices...)
		}
	}

	return devices, nil
}

// detectAMDViaROCm 通过rocm-smi检测AMD GPU.
func (d *Detector) detectAMDViaROCm(ctx context.Context) ([]*GPUDevice, error) {
	if _, err := exec.LookPath("rocm-smi"); err != nil {
		return nil, fmt.Errorf("rocm-smi未安装")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// rocm-smi --showid --showproductname --showmeminfo vram --showtemp --showpower --showuse --showclocks --showfan --csv
	cmd := exec.CommandContext(ctx, "rocm-smi",
		"--showid", "--showproductname", "--showmeminfo", "vram",
		"--showtemp", "--showpower", "--showuse", "--showclocks", "--showfan",
		"--csv")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rocm-smi执行失败: %w", err)
	}

	var devices []*GPUDevice
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// 解析CSV输出 (跳过标题行)
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 8 {
			continue
		}

		for j := range parts {
			parts[j] = strings.TrimSpace(parts[j])
		}

		deviceID, _ := strconv.Atoi(parts[0])
		device := &GPUDevice{
			ID:           fmt.Sprintf("amd%d", deviceID),
			Vendor:       VendorAMD,
			Driver:       "amdgpu",
			DriverOK:     true,
			Status:       StatusHealthy,
			LastUpdated:  time.Now(),
			Capabilities: &GPUCapabilities{},
		}

		// 解析GPU名称
		if len(parts) > 1 {
			device.Name = parts[1]
			device.FullName = parts[1]
			device.Model = extractGPUModel(parts[1])
		}

		// 解析显存 (rocm-smi输出可能不同格式)
		if len(parts) > 2 {
			device.MemoryTotal, _ = parseUint64(parts[2])
		}
		if len(parts) > 3 {
			device.MemoryUsed, _ = parseUint64(parts[3])
		}

		device.MemoryFree = device.MemoryTotal - device.MemoryUsed
		if device.MemoryTotal > 0 {
			device.MemUtil = float64(device.MemoryUsed) / float64(device.MemoryTotal) * 100
		}

		// 检测AMD能力
		d.detectAMDCapabilities(ctx, device)

		devices = append(devices, device)
	}

	return devices, nil
}

// detectAMDViaSysfs 通过sysfs检测AMD GPU.
func (d *Detector) detectAMDViaSysfs(ctx context.Context) ([]*GPUDevice, error) {
	// 检查/sys/class/drm/card*设备
	cardDirs, err := filepath.Glob("/sys/class/drm/card[0-9]*")
	if err != nil {
		return nil, err
	}

	var devices []*GPUDevice

	for _, cardDir := range cardDirs {
		// 读取vendor ID
		vendorPath := filepath.Join(cardDir, "device", "vendor")
		vendorBytes, err := os.ReadFile(vendorPath)
		if err != nil {
			continue
		}
		vendorID := strings.TrimSpace(string(vendorBytes))

		// AMD vendor ID: 0x1002
		if vendorID != "0x1002" {
			continue
		}

		device := &GPUDevice{
			Vendor:       VendorAMD,
			Driver:       "amdgpu",
			DriverOK:     true,
			Status:       StatusHealthy,
			LastUpdated:  time.Now(),
			Capabilities: &GPUCapabilities{},
		}

		// 读取设备路径
		cardName := filepath.Base(cardDir)
		device.DevicePath = fmt.Sprintf("/dev/dri/%s", cardName)

		// 读取PCI地址
		if pciBytes, err := os.ReadFile(filepath.Join(cardDir, "device", "uevent")); err == nil {
			lines := strings.Split(string(pciBytes), "\n")
			for _, l := range lines {
				if strings.HasPrefix(l, "PCI_SLOT_NAME=") {
					device.PCIAddress = strings.TrimPrefix(l, "PCI_SLOT_NAME=")
				}
			}
		}

		// 读取设备ID
		deviceIDPath := filepath.Join(cardDir, "device", "device")
		if idBytes, err := os.ReadFile(deviceIDPath); err == nil {
			device.PCIID = fmt.Sprintf("1002:%s", strings.TrimSpace(string(idBytes)))
		}

		// 生成设备ID
		idx := strings.TrimPrefix(cardName, "card")
		device.ID = fmt.Sprintf("amd%s", idx)
		device.Name = fmt.Sprintf("AMD GPU (card%s)", idx)
		device.FullName = device.Name

		// 读取显存信息 (如果有)
		vramPath := filepath.Join(cardDir, "device", "mem_info_vram_total")
		if vramBytes, err := os.ReadFile(vramPath); err == nil {
			if vram, err := parseUint64(strings.TrimSpace(string(vramBytes))); err == nil {
				device.MemoryTotal = vram / (1024 * 1024) // 转换为MB
			}
		}

		vramUsedPath := filepath.Join(cardDir, "device", "mem_info_vram_used")
		if usedBytes, err := os.ReadFile(vramUsedPath); err == nil {
			if used, err := parseUint64(strings.TrimSpace(string(usedBytes))); err == nil {
				device.MemoryUsed = used / (1024 * 1024)
			}
		}

		device.MemoryFree = device.MemoryTotal - device.MemoryUsed
		if device.MemoryTotal > 0 {
			device.MemUtil = float64(device.MemoryUsed) / float64(device.MemoryTotal) * 100
		}

		// 检测AMD能力
		d.detectAMDCapabilities(ctx, device)

		devices = append(devices, device)
	}

	return devices, nil
}

// detectAMDViaLspci 通过lspci检测AMD GPU.
func (d *Detector) detectAMDViaLspci(ctx context.Context) ([]*GPUDevice, error) {
	if _, err := exec.LookPath("lspci"); err != nil {
		return nil, fmt.Errorf("lspci未安装")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lspci", "-nn")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lspci执行失败: %w", err)
	}

	var devices []*GPUDevice
	lines := strings.Split(string(output), "\n")
	idx := 0

	for _, line := range lines {
		// 匹配VGA/3D/Display控制器中的AMD设备
		if !strings.Contains(line, "AMD") && !strings.Contains(line, "ATI") {
			continue
		}

		if !strings.Contains(line, "VGA") && !strings.Contains(line, "3D") && !strings.Contains(line, "Display") {
			continue
		}

		device := &GPUDevice{
			ID:           fmt.Sprintf("amd%d", idx),
			Vendor:       VendorAMD,
			Driver:       "amdgpu",
			DriverOK:     true,
			Status:       StatusHealthy,
			LastUpdated:  time.Now(),
			Capabilities: &GPUCapabilities{},
		}

		// 解析PCI地址和设备名
		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 1 {
			device.PCIAddress = parts[0]
		}
		if len(parts) >= 2 {
			// 提取设备名 (去除PCI ID部分)
			namePart := parts[1]
			re := regexp.MustCompile(`\[([0-9a-f]{4}:[0-9a-f]{4})\]`)
			matches := re.FindStringSubmatch(namePart)
			if len(matches) > 1 {
				device.PCIID = "1002:" + matches[1][5:]
			}

			// 清理名称
			namePart = re.ReplaceAllString(namePart, "")
			namePart = strings.TrimSpace(namePart)
			if idx := strings.LastIndex(namePart, "("); idx > 0 {
				namePart = strings.TrimSpace(namePart[:idx])
			}
			device.Name = namePart
			device.FullName = namePart
			device.Model = extractGPUModel(namePart)
		}

		devices = append(devices, device)
		idx++
	}

	return devices, nil
}

// detectAMDCapabilities 检测AMD GPU能力.
func (d *Detector) detectAMDCapabilities(ctx context.Context, device *GPUDevice) {
	caps := device.Capabilities

	// 检测ROCm是否可用
	if _, err := exec.LookPath("rocm-smi"); err == nil {
		caps.InferenceCapable = true
		caps.InferenceBackends = []string{"rocm", "opencl"}
	} else {
		caps.InferenceBackends = []string{"opencl"}
	}

	// 检测VA-API (AMD硬件转码)
	if _, err := exec.LookPath("vainfo"); err == nil {
		caps.TranscodeCapable = true
		caps.TranscodeFormats = []string{"h264", "h265"}
		caps.DecodeFormats = []string{"h264", "h265", "vp8", "vp9"}
		caps.EncodeFormats = []string{"h264", "h265"}

		// RDNA2+支持AV1
		if isRDNA2OrAbove(device.Name) {
			caps.DecodeFormats = append(caps.DecodeFormats, "av1")
			caps.EncodeFormats = append(caps.EncodeFormats, "av1")
		}

		caps.MaxEncodeStreams = 3
		caps.MaxDecodeStreams = 3
	} else {
		caps.TranscodeCapable = false
	}

	// AMD特定能力
	caps.StreamProcessors = estimateStreamProcessors(device.Name)
	caps.HalfPrecision = true
	caps.DoublePrecision = true
	caps.Passthrough = true
	caps.Virtualization = true // AMD MxGPU

	if device.MemoryTotal > 0 {
		caps.MaxMemoryAlloc = device.MemoryTotal * 90 / 100
	}
	caps.MaxWorkGroupSize = 1024
}

// detectIntel 检测Intel GPU (通过VA-API或sysfs).
func (d *Detector) detectIntel(ctx context.Context) ([]*GPUDevice, error) {
	var devices []*GPUDevice

	// 通过sysfs检测
	sysfsDevices, err := d.detectIntelViaSysfs(ctx)
	if err == nil {
		devices = append(devices, sysfsDevices...)
	}

	// 通过lspci补充
	if len(devices) == 0 {
		lspciDevices, err := d.detectIntelViaLspci(ctx)
		if err == nil {
			devices = append(devices, lspciDevices...)
		}
	}

	return devices, nil
}

// detectIntelViaSysfs 通过sysfs检测Intel GPU.
func (d *Detector) detectIntelViaSysfs(ctx context.Context) ([]*GPUDevice, error) {
	cardDirs, err := filepath.Glob("/sys/class/drm/card[0-9]*")
	if err != nil {
		return nil, err
	}

	var devices []*GPUDevice

	for _, cardDir := range cardDirs {
		vendorPath := filepath.Join(cardDir, "device", "vendor")
		vendorBytes, err := os.ReadFile(vendorPath)
		if err != nil {
			continue
		}
		vendorID := strings.TrimSpace(string(vendorBytes))

		// Intel vendor ID: 0x8086
		if vendorID != "0x8086" {
			continue
		}

		// 检查是否为GPU (而不是其他PCI设备)
		classPath := filepath.Join(cardDir, "device", "class")
		classBytes, err := os.ReadFile(classPath)
		if err != nil {
			continue
		}
		classID := strings.TrimSpace(string(classBytes))
		// VGA (0x030000), 3D (0x030200), Display (0x038000)
		if !strings.HasPrefix(classID, "0x03") {
			continue
		}

		device := &GPUDevice{
			Vendor:       VendorIntel,
			Driver:       "i915",
			DriverOK:     true,
			Status:       StatusHealthy,
			LastUpdated:  time.Now(),
			Capabilities: &GPUCapabilities{},
		}

		cardName := filepath.Base(cardDir)
		device.DevicePath = fmt.Sprintf("/dev/dri/%s", cardName)

		// 读取PCI地址
		if pciBytes, err := os.ReadFile(filepath.Join(cardDir, "device", "uevent")); err == nil {
			lines := strings.Split(string(pciBytes), "\n")
			for _, l := range lines {
				if strings.HasPrefix(l, "PCI_SLOT_NAME=") {
					device.PCIAddress = strings.TrimPrefix(l, "PCI_SLOT_NAME=")
				}
			}
		}

		// 读取设备ID
		deviceIDPath := filepath.Join(cardDir, "device", "device")
		if idBytes, err := os.ReadFile(deviceIDPath); err == nil {
			device.PCIID = fmt.Sprintf("8086:%s", strings.TrimSpace(string(idBytes)))
		}

		idx := strings.TrimPrefix(cardName, "card")
		device.ID = fmt.Sprintf("intel%s", idx)
		device.Name = fmt.Sprintf("Intel GPU (card%s)", idx)
		device.FullName = device.Name

		// 检测Intel能力
		d.detectIntelCapabilities(ctx, device)

		devices = append(devices, device)
	}

	return devices, nil
}

// detectIntelViaLspci 通过lspci检测Intel GPU.
func (d *Detector) detectIntelViaLspci(ctx context.Context) ([]*GPUDevice, error) {
	if _, err := exec.LookPath("lspci"); err != nil {
		return nil, fmt.Errorf("lspci未安装")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lspci", "-nn")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lspci执行失败: %w", err)
	}

	var devices []*GPUDevice
	lines := strings.Split(string(output), "\n")
	idx := 0

	for _, line := range lines {
		if !strings.Contains(line, "Intel") {
			continue
		}

		// 匹配Intel GPU
		if !strings.Contains(line, "VGA") && !strings.Contains(line, "3D") && !strings.Contains(line, "Display") {
			continue
		}

		// 排除非GPU设备 (如网卡等)
		if strings.Contains(line, "Ethernet") || strings.Contains(line, "Network") {
			continue
		}

		device := &GPUDevice{
			ID:           fmt.Sprintf("intel%d", idx),
			Vendor:       VendorIntel,
			Driver:       "i915",
			DriverOK:     true,
			Status:       StatusHealthy,
			LastUpdated:  time.Now(),
			Capabilities: &GPUCapabilities{},
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 1 {
			device.PCIAddress = parts[0]
		}
		if len(parts) >= 2 {
			namePart := parts[1]
			re := regexp.MustCompile(`\[([0-9a-f]{4}:[0-9a-f]{4})\]`)
			matches := re.FindStringSubmatch(namePart)
			if len(matches) > 1 {
				device.PCIID = "8086:" + matches[1][5:]
			}

			namePart = re.ReplaceAllString(namePart, "")
			namePart = strings.TrimSpace(namePart)
			device.Name = namePart
			device.FullName = namePart
			device.Model = extractGPUModel(namePart)
		}

		detectIntelCapabilities(ctx, device)

		devices = append(devices, device)
		idx++
	}

	return devices, nil
}

// detectIntelCapabilities 检测Intel GPU能力.
func (d *Detector) detectIntelCapabilities(ctx context.Context, device *GPUDevice) {
	detectIntelCapabilities(ctx, device)
}

func detectIntelCapabilities(ctx context.Context, device *GPUDevice) {
	caps := device.Capabilities

	// 检测VA-API
	if _, err := exec.LookPath("vainfo"); err == nil {
		caps.TranscodeCapable = true
		caps.TranscodeFormats = []string{"h264", "h265"}
		caps.DecodeFormats = []string{"h264", "h265", "vp8", "vp9", "av1"}
		caps.EncodeFormats = []string{"h264", "h265"}
		caps.MaxEncodeStreams = 3
		caps.MaxDecodeStreams = 3
	}

	// Intel特定能力
	caps.InferenceCapable = true
	caps.InferenceBackends = []string{"opencl", "openvino"}
	caps.HalfPrecision = true
	caps.DoublePrecision = false
	caps.Passthrough = false // Intel通常使用SR-IOV
	caps.Virtualization = true

	if device.MemoryTotal > 0 {
		caps.MaxMemoryAlloc = device.MemoryTotal * 80 / 100
	}
	caps.MaxWorkGroupSize = 512
}

// 辅助函数

func parseUint64(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "[N/A]" || s == "N/A" {
		return 0, nil
	}
	// 移除单位
	s = strings.TrimSuffix(s, " MiB")
	s = strings.TrimSuffix(s, " MB")
	s = strings.TrimSuffix(s, " W")
	s = strings.TrimSpace(s)
	return strconv.ParseUint(s, 10, 64)
}

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "[N/A]" || s == "N/A" {
		return 0, nil
	}
	s = strings.TrimSuffix(s, " C")
	s = strings.TrimSpace(s)
	return strconv.Atoi(s)
}

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "[N/A]" || s == "N/A" {
		return 0, nil
	}
	s = strings.TrimSuffix(s, " %")
	s = strings.TrimSpace(s)
	return strconv.ParseFloat(s, 64)
}

func extractGPUModel(name string) string {
	// 从完整名称提取型号
	name = strings.TrimSpace(name)

	// 移除厂商前缀
	prefixes := []string{"NVIDIA", "AMD", "Intel", "GeForce", "Radeon", "Arc", "Radeon(TM)", "Graphics"}
	for _, prefix := range prefixes {
		name = strings.ReplaceAll(name, prefix, "")
	}

	return strings.TrimSpace(name)
}

func isRTX30OrAbove(name string) bool {
	name = strings.ToLower(name)
	if strings.Contains(name, "rtx 30") || strings.Contains(name, "rtx 40") || strings.Contains(name, "rtx 50") {
		return true
	}
	if strings.Contains(name, "a100") || strings.Contains(name, "a30") || strings.Contains(name, "a40") {
		return true
	}
	if strings.Contains(name, "h100") || strings.Contains(name, "h200") {
		return true
	}
	return false
}

func isRDNA2OrAbove(name string) bool {
	name = strings.ToLower(name)
	// RDNA2: RX 6000系列及以上
	if strings.Contains(name, "rx 6") || strings.Contains(name, "rx 7") || strings.Contains(name, "rx 8") {
		return true
	}
	if strings.Contains(name, "rx 7") { // RDNA3
		return true
	}
	if strings.Contains(name, "w6") || strings.Contains(name, "w7") { // 工作站卡
		return true
	}
	return false
}

func estimateCUDACores(name string, computeCap string) int {
	name = strings.ToLower(name)

	// 常见GPU的CUDA核心数估算
	if strings.Contains(name, "rtx 4090") {
		return 16384
	}
	if strings.Contains(name, "rtx 4080") {
		return 9728
	}
	if strings.Contains(name, "rtx 4070") {
		return 5888
	}
	if strings.Contains(name, "rtx 3090") {
		return 10496
	}
	if strings.Contains(name, "rtx 3080") {
		return 8704
	}
	if strings.Contains(name, "rtx 3070") {
		return 5888
	}
	if strings.Contains(name, "rtx 3060") {
		return 3584
	}
	if strings.Contains(name, "a100") {
		return 6912
	}
	if strings.Contains(name, "h100") {
		return 16896
	}

	return 0 // 未知
}

func estimateTensorCores(name string, computeCap string) int {
	name = strings.ToLower(name)

	// 只有Volta及以后架构有Tensor核心
	if !strings.Contains(computeCap, "7") && !strings.Contains(computeCap, "8") && !strings.Contains(computeCap, "9") {
		return 0
	}

	// 简化估算
	if strings.Contains(name, "rtx 4090") {
		return 512
	}
	if strings.Contains(name, "rtx 3090") {
		return 328
	}

	return 0
}

func estimateRTCores(name string) int {
	name = strings.ToLower(name)

	// 只有RTX系列有RT核心
	if strings.Contains(name, "rtx") {
		if strings.Contains(name, "4090") {
			return 128
		}
		if strings.Contains(name, "3090") {
			return 82
		}
	}

	return 0
}

func estimateStreamProcessors(name string) int {
	name = strings.ToLower(name)

	// 常见AMD GPU流处理器数
	if strings.Contains(name, "7900 xtx") {
		return 12288
	}
	if strings.Contains(name, "7900 xt") {
		return 10752
	}
	if strings.Contains(name, "7800 xt") {
		return 3840
	}
	if strings.Contains(name, "6900 xt") {
		return 5120
	}
	if strings.Contains(name, "6800 xt") {
		return 4608
	}

	return 0
}
