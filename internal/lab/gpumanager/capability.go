package gpumanager

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CapabilityChecker GPU能力检测器.
type CapabilityChecker struct {
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewCapabilityChecker 创建能力检测器.
func NewCapabilityChecker(logger *slog.Logger) *CapabilityChecker {
	if logger == nil {
		logger = slog.Default()
	}
	return &CapabilityChecker{logger: logger}
}

// CheckTranscodeCapabilities 检测硬件转码能力.
func (cc *CapabilityChecker) CheckTranscodeCapabilities(ctx context.Context, device *GPUDevice) *TranscodeCapability {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	result := &TranscodeCapability{
		DeviceID:   device.ID,
		DeviceName: device.Name,
		Vendor:     device.Vendor,
	}

	switch device.Vendor {
	case VendorNVIDIA:
		cc.checkNVIDIATranscode(ctx, device, result)
	case VendorAMD:
		cc.checkAMDTranscode(ctx, device, result)
	case VendorIntel:
		cc.checkIntelTranscode(ctx, device, result)
	default:
		result.Capable = false
	}

	return result
}

// checkNVIDIATranscode 检测NVIDIA转码能力.
func (cc *CapabilityChecker) checkNVIDIATranscode(ctx context.Context, device *GPUDevice, result *TranscodeCapability) {
	// NVIDIA使用NVENC/NVDEC
	result.Engine = "NVENC/NVDEC"
	result.Capable = true

	// 检查NVENC支持的编码格式
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		// 基于GPU名称推断能力
		name := strings.ToLower(device.Name)

		// 所有支持NVENC的GPU都支持H.264和H.265
		result.H264Encode = true
		result.H264Decode = true
		result.H265Encode = true
		result.H265Decode = true

		// RTX 30系列及以上支持AV1
		if strings.Contains(name, "rtx 30") || strings.Contains(name, "rtx 40") || strings.Contains(name, "rtx 50") {
			result.AV1Encode = true
			result.AV1Decode = true
		}

		// A100/H100等专业卡
		if strings.Contains(name, "a100") || strings.Contains(name, "h100") {
			result.AV1Encode = true
			result.AV1Decode = true
		}

		// VP8/VP9支持 (解码)
		result.VP8Decode = true
		result.VP9Decode = true
	}

	// 最大并发流 (NVIDIA consumer限制为3, professional无限制)
	if strings.Contains(strings.ToLower(device.Name), "rtx") {
		result.MaxStreams = 3
	} else {
		result.MaxStreams = 0 // 无限制 (专业卡)
	}

	result.MaxResolution = "8192x8192"
	result.HDRSupport = true
	result.BitDepth8 = true
	result.BitDepth10 = true
}

// checkAMDTranscode 检测AMD转码能力.
func (cc *CapabilityChecker) checkAMDTranscode(ctx context.Context, device *GPUDevice, result *TranscodeCapability) {
	// AMD使用AMF (Advanced Media Framework)
	result.Engine = "AMF"
	result.Capable = true

	name := strings.ToLower(device.Name)

	// 基本格式支持
	result.H264Encode = true
	result.H264Decode = true
	result.H265Encode = true
	result.H265Decode = true

	// RDNA2+支持AV1
	if strings.Contains(name, "rx 6") || strings.Contains(name, "rx 7") || strings.Contains(name, "rx 8") {
		result.AV1Encode = true
		result.AV1Decode = true
	}

	// VP8/VP9支持
	result.VP8Decode = true
	result.VP9Decode = true

	// AMD通常无流限制
	result.MaxStreams = 0
	result.MaxResolution = "8192x4320"
	result.HDRSupport = true
	result.BitDepth8 = true
	result.BitDepth10 = true
}

// checkIntelTranscode 检测Intel转码能力.
func (cc *CapabilityChecker) checkIntelTranscode(ctx context.Context, device *GPUDevice, result *TranscodeCapability) {
	// Intel使用QSV (Quick Sync Video) 或VA-API
	result.Engine = "QSV/VA-API"
	result.Capable = true

	name := strings.ToLower(device.Name)

	// 基本格式支持
	result.H264Encode = true
	result.H264Decode = true
	result.H265Encode = true
	result.H265Decode = true

	// Arc系列支持AV1
	if strings.Contains(name, "arc") {
		result.AV1Encode = true
		result.AV1Decode = true
	}

	// 12代+核显支持AV1
	if strings.Contains(name, "uhd 7") || strings.Contains(name, "iris xe") {
		result.AV1Decode = true
	}

	// VP8/VP9支持
	result.VP8Decode = true
	result.VP9Decode = true

	result.MaxStreams = 3
	result.MaxResolution = "8192x4320"
	result.HDRSupport = true
	result.BitDepth8 = true
	result.BitDepth10 = true
}

// CheckInferenceCapabilities 检测AI推理能力.
func (cc *CapabilityChecker) CheckInferenceCapabilities(ctx context.Context, device *GPUDevice) *InferenceCapability {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	result := &InferenceCapability{
		DeviceID:       device.ID,
		DeviceName:     device.Name,
		Vendor:         device.Vendor,
		DeviceMemoryMB: device.MemoryTotal,
	}

	switch device.Vendor {
	case VendorNVIDIA:
		cc.checkNVIDIAInference(ctx, device, result)
	case VendorAMD:
		cc.checkAMDInference(ctx, device, result)
	case VendorIntel:
		cc.checkIntelInference(ctx, device, result)
	default:
		result.Capable = false
	}

	return result
}

// checkNVIDIAInference 检测NVIDIA AI推理能力.
func (cc *CapabilityChecker) checkNVIDIAInference(ctx context.Context, device *GPUDevice, result *InferenceCapability) {
	result.Capable = true
	result.Backend = "CUDA"
	result.Frameworks = []string{"tensorrt", "onnxruntime", "pytorch", "tensorflow"}

	name := strings.ToLower(device.Name)

	// FP16/FP32/INT8支持
	result.FP16Performance = true
	result.FP32Performance = true
	result.INT8Performance = true

	// 估算算力 (TOPS)
	caps := device.Capabilities
	if caps != nil {
		computeCap := caps.ComputeCapability
		if strings.HasPrefix(computeCap, "8.") || strings.HasPrefix(computeCap, "9.") {
			// Ampere/Hopper架构
			result.EstimatedTOPS = estimateNVIDIATOPS(name)
		} else if strings.HasPrefix(computeCap, "7.") {
			// Volta/Turing架构
			result.EstimatedTOPS = estimateNVIDIATOPS(name) * 0.5
		}
	}

	// 基于显存推荐模型规模
	if device.MemoryTotal >= 80000 { // 80GB+
		result.ModelsSupported = []string{"70B", "40B", "13B", "7B", "3B"}
		result.MaxBatchSize = 64
	} else if device.MemoryTotal >= 24000 { // 24GB
		result.ModelsSupported = []string{"13B", "7B", "3B", "1B"}
		result.MaxBatchSize = 32
	} else if device.MemoryTotal >= 12000 { // 12GB
		result.ModelsSupported = []string{"7B", "3B", "1B"}
		result.MaxBatchSize = 16
	} else if device.MemoryTotal >= 8000 { // 8GB
		result.ModelsSupported = []string{"3B", "1B"}
		result.MaxBatchSize = 8
	} else {
		result.ModelsSupported = []string{"1B"}
		result.MaxBatchSize = 4
	}
}

// checkAMDInference 检测AMD AI推理能力.
func (cc *CapabilityChecker) checkAMDInference(ctx context.Context, device *GPUDevice, result *InferenceCapability) {
	// 检查ROCm是否可用
	if _, err := exec.LookPath("rocm-smi"); err != nil {
		result.Capable = false
		return
	}

	result.Capable = true
	result.Backend = "ROCm"
	result.Frameworks = []string{"onnxruntime", "pytorch", "tensorflow"}

	name := strings.ToLower(device.Name)

	// FP16/FP32支持
	result.FP16Performance = true
	result.FP32Performance = true
	result.INT8Performance = false // ROCm INT8支持有限

	// 估算算力
	if strings.Contains(name, "7900") {
		result.EstimatedTOPS = 61.0
	} else if strings.Contains(name, "7800") {
		result.EstimatedTOPS = 35.0
	} else if strings.Contains(name, "6900") {
		result.EstimatedTOPS = 25.0
	} else {
		result.EstimatedTOPS = 15.0
	}

	// 基于显存推荐模型规模
	if device.MemoryTotal >= 24000 {
		result.ModelsSupported = []string{"13B", "7B", "3B", "1B"}
		result.MaxBatchSize = 32
	} else if device.MemoryTotal >= 16000 {
		result.ModelsSupported = []string{"7B", "3B", "1B"}
		result.MaxBatchSize = 16
	} else {
		result.ModelsSupported = []string{"3B", "1B"}
		result.MaxBatchSize = 8
	}
}

// checkIntelInference 检测Intel AI推理能力.
func (cc *CapabilityChecker) checkIntelInference(ctx context.Context, device *GPUDevice, result *InferenceCapability) {
	result.Capable = true
	result.Backend = "OpenVINO"
	result.Frameworks = []string{"openvino", "onnxruntime"}

	name := strings.ToLower(device.Name)

	// FP16支持
	result.FP16Performance = true
	result.FP32Performance = true
	result.INT8Performance = true // Intel INT8支持良好

	// 估算算力
	if strings.Contains(name, "arc a770") {
		result.EstimatedTOPS = 19.7
	} else if strings.Contains(name, "arc a750") {
		result.EstimatedTOPS = 15.4
	} else if strings.Contains(name, "arc") {
		result.EstimatedTOPS = 10.0
	} else {
		result.EstimatedTOPS = 5.0 // 核显
	}

	// Intel GPU显存通常共享系统内存
	if device.MemoryTotal >= 16000 {
		result.ModelsSupported = []string{"7B", "3B", "1B"}
		result.MaxBatchSize = 16
	} else {
		result.ModelsSupported = []string{"3B", "1B"}
		result.MaxBatchSize = 8
	}
}

// estimateNVIDIATOPS 估算NVIDIA GPU算力.
func estimateNVIDIATOPS(name string) float64 {
	name = strings.ToLower(name)

	if strings.Contains(name, "h100") {
		return 3958.0
	}
	if strings.Contains(name, "a100") {
		return 624.0
	}
	if strings.Contains(name, "rtx 4090") {
		return 82.6
	}
	if strings.Contains(name, "rtx 4080") {
		return 61.4
	}
	if strings.Contains(name, "rtx 4070") {
		return 46.6
	}
	if strings.Contains(name, "rtx 3090") {
		return 71.0
	}
	if strings.Contains(name, "rtx 3080") {
		return 52.7
	}
	if strings.Contains(name, "rtx 3070") {
		return 40.2
	}
	if strings.Contains(name, "rtx 3060") {
		return 26.4
	}

	return 20.0 // 默认估算
}

// GenerateCapabilityReport 生成GPU能力报告.
func (cc *CapabilityChecker) GenerateCapabilityReport(ctx context.Context, devices []*GPUDevice) *GPUCapabilityReport {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	report := &GPUCapabilityReport{
		Timestamp:  timeNow(),
		SystemGPUs: len(devices),
		Devices:    devices,
		Summary: &CapabilitySummary{
			TotalGPUs: len(devices),
		},
		Recommendations: make([]string, 0),
	}

	var bestTranscode, bestInference *GPUDevice
	var bestTranscodeScore, bestInferenceScore int

	for _, device := range devices {
		// 统计厂商
		switch device.Vendor {
		case VendorNVIDIA:
			report.Summary.NvidiaGPUs++
		case VendorAMD:
			report.Summary.AmdGPUs++
		case VendorIntel:
			report.Summary.IntelGPUs++
		}

		// 累计显存
		report.Summary.TotalMemoryMB += device.MemoryTotal

		// 统计转码能力
		if device.Capabilities != nil && device.Capabilities.TranscodeCapable {
			report.Summary.TranscodeCapable++
			score := calculateTranscodeScore(device)
			if score > bestTranscodeScore {
				bestTranscodeScore = score
				bestTranscode = device
			}
		}

		// 统计推理能力
		if device.Capabilities != nil && device.Capabilities.InferenceCapable {
			report.Summary.InferenceCapable++
			score := calculateInferenceScore(device)
			if score > bestInferenceScore {
				bestInferenceScore = score
				bestInference = device
			}
		}
	}

	if bestTranscode != nil {
		report.Summary.BestForTranscode = bestTranscode.ID
	}
	if bestInference != nil {
		report.Summary.BestForInference = bestInference.ID
	}

	// 生成建议
	report.Recommendations = generateRecommendations(report)

	return report
}

// calculateTranscodeScore 计算转码评分.
func calculateTranscodeScore(device *GPUDevice) int {
	score := 0

	if device.Capabilities == nil {
		return 0
	}

	// 检查编码格式支持
	encodeFormats := make(map[string]bool)
	for _, f := range device.Capabilities.EncodeFormats {
		encodeFormats[f] = true
	}

	if encodeFormats["h264"] {
		score += 10
	}
	if encodeFormats["h265"] {
		score += 20
	}
	if encodeFormats["av1"] {
		score += 30
	}
	if device.Capabilities.MaxEncodeStreams > 0 {
		score += device.Capabilities.MaxEncodeStreams * 5
	} else {
		score += 50 // 无限制
	}

	// 显存加分
	if device.MemoryTotal >= 8000 {
		score += 10
	}

	return score
}

// calculateInferenceScore 计算推理评分.
func calculateInferenceScore(device *GPUDevice) int {
	score := 0

	if device.Capabilities == nil {
		return 0
	}

	// 基础能力
	if device.Capabilities.InferenceCapable {
		score += 20
	}
	if device.Capabilities.HalfPrecision {
		score += 10
	}
	if device.Capabilities.TensorCores > 0 {
		score += 30
	}

	// 显存是关键
	if device.MemoryTotal >= 80000 {
		score += 50
	} else if device.MemoryTotal >= 24000 {
		score += 30
	} else if device.MemoryTotal >= 12000 {
		score += 20
	} else if device.MemoryTotal >= 8000 {
		score += 10
	}

	return score
}

// generateRecommendations 生成建议.
func generateRecommendations(report *GPUCapabilityReport) []string {
	var recs []string

	if report.Summary.TotalGPUs == 0 {
		recs = append(recs, "未检测到GPU设备，建议安装GPU驱动或添加显卡")
		return recs
	}

	if report.Summary.TranscodeCapable == 0 {
		recs = append(recs, "未发现支持硬件转码的GPU，视频转码将使用CPU，性能较差")
	}

	if report.Summary.InferenceCapable == 0 {
		recs = append(recs, "未发现支持AI推理的GPU，AI功能将受限")
	}

	if report.Summary.NvidiaGPUs > 0 && report.Summary.AmdGPUs > 0 {
		recs = append(recs, "检测到混合GPU环境(NVIDIA+AMD)，建议使用NVIDIA进行AI推理")
	}

	if report.Summary.BestForTranscode != "" {
		recs = append(recs, fmt.Sprintf("推荐使用 %s 进行视频转码", report.Summary.BestForTranscode))
	}

	if report.Summary.BestForInference != "" {
		recs = append(recs, fmt.Sprintf("推荐使用 %s 进行AI推理", report.Summary.BestForInference))
	}

	return recs
}

func timeNow() time.Time {
	return time.Now()
}
