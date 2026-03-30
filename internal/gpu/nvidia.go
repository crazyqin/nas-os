// Package gpu NVIDIA GPU提供者
package gpu

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// NVIDIAProvider NVIDIA GPU提供者
type NVIDIAProvider struct {
	logger *zap.Logger
}

// NewNVIDIAProvider 创建NVIDIA提供者
func NewNVIDIAProvider(logger *zap.Logger) (*NVIDIAProvider, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	provider := &NVIDIAProvider{
		logger: logger,
	}

	// 检查nvidia-smi是否可用
	if err := provider.CheckDriver(); err != nil {
		return nil, err
	}

	return provider, nil
}

// CheckDriver 检查NVIDIA驱动状态
func (p *NVIDIAProvider) CheckDriver() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nvidia-smi执行失败: %w", err)
	}

	return nil
}

// ListDevices 列出所有NVIDIA GPU设备
func (p *NVIDIAProvider) ListDevices() ([]*GPUDevice, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=index,uuid,name,pci.bus_id,memory.total,memory.used,memory.free,utilization.gpu,temperature.gpu,power.draw,power.limit,driver_version", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取GPU列表失败: %w", err)
	}

	var devices []*GPUDevice
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		device, err := p.parseGPUInfo(line)
		if err != nil {
			p.logger.Warn("解析GPU信息失败", zap.String("line", line), zap.Error(err))
			continue
		}

		device.Vendor = "nvidia"
		devices = append(devices, device)
	}

	return devices, nil
}

// parseGPUInfo 解析nvidia-smi输出
func (p *NVIDIAProvider) parseGPUInfo(line string) (*GPUDevice, error) {
	// CSV格式: index,uuid,name,pci.bus_id,memory.total,memory.used,memory.free,utilization.gpu,temperature.gpu,power.draw,power.limit,driver_version
	fields := strings.Split(line, ",")
	if len(fields) < 11 {
		return nil, fmt.Errorf("字段数量不足: %d", len(fields))
	}

	device := &GPUDevice{
		DevicePath: "/dev/nvidia" + strings.TrimSpace(fields[0]),
	}

	// 解析索引
	if idx, err := strconv.Atoi(strings.TrimSpace(fields[0])); err == nil {
		device.ID = fmt.Sprintf("nvidia%d", idx)
	} else {
		device.ID = "nvidia" + strings.TrimSpace(fields[0])
	}

	// 解析UUID
	device.UUID = strings.TrimSpace(fields[1])

	// 解析名称
	device.Name = strings.TrimSpace(fields[2])

	// 解析PCI地址
	device.PCIAddress = strings.TrimSpace(fields[3])

	// 解析总显存(MB)
	if mem, err := strconv.ParseUint(strings.TrimSpace(fields[4]), 10, 64); err == nil {
		device.MemoryTotal = mem
	}

	// 解析已用显存(MB)
	if mem, err := strconv.ParseUint(strings.TrimSpace(fields[5]), 10, 64); err == nil {
		device.MemoryUsed = mem
	}

	// 解析可用显存(MB)
	if mem, err := strconv.ParseUint(strings.TrimSpace(fields[6]), 10, 64); err == nil {
		device.MemoryFree = mem
	}

	// 解析GPU利用率(%)
	if util, err := strconv.ParseFloat(strings.TrimSpace(fields[7]), 64); err == nil {
		// 存储在Temperature字段作为临时存储，这里需要修改types
		// 使用PowerUsage存储利用率
		device.PowerUsage = uint64(util)
	}

	// 解析温度
	if temp, err := strconv.Atoi(strings.TrimSpace(fields[8])); err == nil {
		device.Temperature = temp
	}

	// 解析当前功耗(W)
	if power, err := strconv.ParseUint(strings.TrimSpace(fields[9]), 10, 64); err == nil {
		device.PowerUsage = power
	}

	// 解析功率限制(W)
	if power, err := strconv.ParseUint(strings.TrimSpace(fields[10]), 10, 64); err == nil {
		device.PowerLimit = power
	}

	// 解析驱动版本
	if len(fields) > 11 {
		device.Driver = strings.TrimSpace(fields[11])
	}

	// 设置状态
	device.Status = GPUStatusAvailable
	if device.Allocated {
		device.Status = GPUStatusAllocated
	}

	// 估算CUDA核心数（基于GPU型号）
	device.CUDAcores = estimateCUDACores(device.Name)

	return device, nil
}

// GetGPUByIndex 根据索引获取GPU
func (p *NVIDIAProvider) GetGPUByIndex(index int) (*GPUDevice, error) {
	devices, err := p.ListDevices()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		if strings.Contains(device.DevicePath, fmt.Sprintf("/dev/nvidia%d", index)) {
			return device, nil
		}
	}

	return nil, fmt.Errorf("GPU索引 %d 不存在", index)
}

// GetGPUByUUID 根据UUID获取GPU
func (p *NVIDIAProvider) GetGPUByUUID(uuid string) (*GPUDevice, error) {
	devices, err := p.ListDevices()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		if device.UUID == uuid {
			return device, nil
		}
	}

	return nil, fmt.Errorf("GPU UUID %s 不存在", uuid)
}

// SetPowerLimit 设置GPU功率限制
func (p *NVIDIAProvider) SetPowerLimit(index int, limitWatts uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "-i", strconv.Itoa(index), "-pl", strconv.FormatUint(limitWatts, 10))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置功率限制失败: %w, output: %s", err, string(output))
	}

	return nil
}

// SetMemoryLimit 设置GPU显存限制（通过LD_PRELOAD或计算限制）
func (p *NVIDIAProvider) SetMemoryLimit(index int, limitMB uint64) error {
	// 注意: NVIDIA不直接支持显存限制，需要通过应用程序或容器实现
	p.logger.Info("显存限制需要通过容器运行时实现",
		zap.Int("gpuIndex", index),
		zap.Uint64("limitMB", limitMB))
	return nil
}

// ResetLimits 重置GPU限制
func (p *NVIDIAProvider) ResetLimits(index int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "-i", strconv.Itoa(index), "-rgc")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重置GPU限制失败: %w, output: %s", err, string(output))
	}

	return nil
}

// GetDriverVersion 获取驱动版本
func (p *NVIDIAProvider) GetDriverVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取驱动版本失败: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetNVCCVersion 获取CUDA Toolkit版本
func (p *NVIDIAProvider) GetNVCCVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvcc", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("nvcc not found or failed: %w", err)
	}

	// 解析版本号
	re := regexp.MustCompile(`release (\d+\.\d+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) >= 2 {
		return matches[1], nil
	}

	return "", fmt.Errorf("无法解析CUDA版本")
}

// GetComputeCapability 获取GPU计算能力
func (p *NVIDIAProvider) GetComputeCapability(index int) (major, minor int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "-i", strconv.Itoa(index), "--query-gpu=compute_cap", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("获取计算能力失败: %w", err)
	}

	// 格式: major.minor
	parts := strings.Split(strings.TrimSpace(string(output)), ".")
	if len(parts) >= 2 {
		major, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
		minor, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	}

	return major, minor, nil
}

// GetECCMode 获取ECC模式状态
func (p *NVIDIAProvider) GetECCMode(index int) (enabled bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "-i", strconv.Itoa(index), "--query-gpu=ecc.mode", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("获取ECC模式失败: %w", err)
	}

	mode := strings.TrimSpace(string(output))
	enabled = strings.Contains(strings.ToLower(mode), "enabled")

	return enabled, nil
}

// GetPersistenceMode 获取持久化模式状态
func (p *NVIDIAProvider) GetPersistenceMode(index int) (enabled bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "-i", strconv.Itoa(index), "--query-gpu=persistence_mode", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("获取持久化模式失败: %w", err)
	}

	mode := strings.TrimSpace(string(output))
	enabled = strings.Contains(strings.ToLower(mode), "enabled")

	return enabled, nil
}

// SetPersistenceMode 设置持久化模式
func (p *NVIDIAProvider) SetPersistenceMode(index int, enabled bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mode := "Off"
	if enabled {
		mode = "On"
	}

	cmd := exec.CommandContext(ctx, "nvidia-smi", "-i", strconv.Itoa(index), "-pm", mode)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置持久化模式失败: %w, output: %s", err, string(output))
	}

	return nil
}

// GetAccountingStats 获取GPU accounting统计信息
func (p *NVIDIAProvider) GetAccountingStats(index int) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "-i", strconv.Itoa(index), "--query-compute-apps=pid,process_name,used_memory", "--format=csv")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取计算统计失败: %w", err)
	}

	return json.RawMessage(output), nil
}

// GetSupportedClocks 获取支持的时钟频率
func (p *NVIDIAProvider) GetSupportedClocks(index int, type_ string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "-i", strconv.Itoa(index), "--query-supported-clocks=memory", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取支持的时钟失败: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return lines, nil
}

// GetTopology 获取GPU拓扑信息
func (p *NVIDIAProvider) GetTopology() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "topo", "-m")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取拓扑信息失败: %w", err)
	}

	return string(output), nil
}

// GetNVLinkStatus 获取NVLink状态
func (p *NVIDIAProvider) GetNVLinkStatus() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "nvlink", "-s")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取NVLink状态失败: %w", err)
	}

	return string(output), nil
}

// estimateCUDACores 根据GPU名称估算CUDA核心数
func estimateCUDACores(gpuName string) int {
	// 简化版估算
	// 实际CUDA核心数需要通过nvidia-smi或CUDA API获取
	name := strings.ToLower(gpuName)

	// RTX 40系列
	if strings.Contains(name, "4090") {
		return 16384
	}
	if strings.Contains(name, "4080") {
		return 9728
	}
	if strings.Contains(name, "4070") {
		return 5888
	}
	if strings.Contains(name, "4060") {
		return 3072
	}

	// RTX 30系列
	if strings.Contains(name, "3090") {
		return 10496
	}
	if strings.Contains(name, "3080") {
		return 8704
	}
	if strings.Contains(name, "3070") {
		return 5888
	}
	if strings.Contains(name, "3060") {
		return 3584
	}

	// RTX 20系列
	if strings.Contains(name, "2080") {
		return 4352
	}
	if strings.Contains(name, "2070") {
		return 2304
	}
	if strings.Contains(name, "2060") {
		return 1920
	}

	// GTX 16系列
	if strings.Contains(name, "1660") {
		return 1408
	}
	if strings.Contains(name, "1650") {
		return 896
	}

	// A系列
	if strings.Contains(name, "a100") {
		return 6912
	}
	if strings.Contains(name, "a30") {
		return 3584
	}
	if strings.Contains(name, "a10") {
		return 3600
	}
	if strings.Contains(name, "a4000") {
		return 6144
	}
	if strings.Contains(name, "a5000") {
		return 8192
	}
	if strings.Contains(name, "a6000") {
		return 10752
	}

	// 默认值
	return 2048
}
