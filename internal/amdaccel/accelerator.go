package amdaccel

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// AMDGPUInfo AMD显卡信息
type AMDGPUInfo struct {
	Name     string
	Driver   string
	Memory   int64
	Arch     string // GCN5, RDNA1, RDNA2, RDNA3, RDNA4
	PCIID    string
}

// AMDAccelerator AMD显卡加速器
type AMDAccelerator struct {
	gpus   []AMDGPUInfo
	mu     sync.RWMutex
	config AcceleratorConfig
}

// AcceleratorConfig 加速器配置
type AcceleratorConfig struct {
	EnableVideoTranscode bool
	EnableAIInference    bool
	MemoryLimit          int64
}

// NewAMDAccelerator 创建AMD加速器
func NewAMDAccelerator(config AcceleratorConfig) *AMDAccelerator {
	accel := &AMDAccelerator{
		config: config,
	}
	accel.detectGPUs()
	return accel
}

// detectGPUs 检测AMD显卡
func (a *AMDAccelerator) detectGPUs() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 使用lspci检测AMD显卡
	cmd := exec.Command("lspci", "-v")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "VGA") && strings.Contains(line, "AMD") {
			// 解析显卡信息
			gpu := AMDGPUInfo{
				Name:   parseGPUName(line),
				Driver: "amdgpu",
				Arch:   detectArchitecture(line),
			}
			a.gpus = append(a.gpus, gpu)
		}
	}
}

// parseGPUName 解析显卡名称
func parseGPUName(line string) string {
	// 简化实现
	if idx := strings.Index(line, "AMD"); idx != -1 {
		return strings.TrimSpace(line[idx:])
	}
	return "AMD GPU"
}

// detectArchitecture 检测显卡架构
func detectArchitecture(line string) string {
	line = strings.ToLower(line)
	if strings.Contains(line, "rdna4") || strings.Contains(line, "9000") {
		return "RDNA4"
	}
	if strings.Contains(line, "rdna3") || strings.Contains(line, "7000") {
		return "RDNA3"
	}
	if strings.Contains(line, "rdna2") || strings.Contains(line, "6000") {
		return "RDNA2"
	}
	if strings.Contains(line, "rdna") || strings.Contains(line, "5000") {
		return "RDNA1"
	}
	return "GCN5"
}

// GetGPUCount 获取显卡数量
func (a *AMDAccelerator) GetGPUCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.gpus)
}

// GetGPUInfo 获取显卡信息
func (a *AMDAccelerator) GetGPUInfo(index int) (*AMDGPUInfo, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if index < 0 || index >= len(a.gpus) {
		return nil, fmt.Errorf("invalid GPU index: %d", index)
	}
	return &a.gpus[index], nil
}

// TranscodeVideo 视频转码（使用AMD硬件加速）
func (a *AMDAccelerator) TranscodeVideo(input, output string, options TranscodeOptions) error {
	if !a.config.EnableVideoTranscode {
		return fmt.Errorf("video transcode disabled")
	}

	if len(a.gpus) == 0 {
		return fmt.Errorf("no AMD GPU available")
	}

	// 使用VA-API进行硬件加速转码
	args := []string{
		"-hwaccel", "vaapi",
		"-hwaccel_device", "/dev/dri/renderD128",
		"-i", input,
		"-c:v", "h264_vaapi",
		"-qp", "23",
		output,
	}

	cmd := exec.Command("ffmpeg", args...)
	return cmd.Run()
}

// TranscodeOptions 转码选项
type TranscodeOptions struct {
	Codec   string
	Quality int
	Bitrate string
}

// IsAvailable 检查加速器是否可用
func (a *AMDAccelerator) IsAvailable() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.gpus) > 0
}
