// Package gpumanager 提供多厂商GPU自动检测、健康监控和能力评估功能。
// 支持NVIDIA、AMD、Intel GPU，学习飞牛fnOS的AMD显卡支持方案。
package gpumanager

import (
	"time"
)

// GPUVendor GPU厂商类型
type GPUVendor string

const (
	VendorNVIDIA GPUVendor = "nvidia"
	VendorAMD    GPUVendor = "amd"
	VendorIntel  GPUVendor = "intel"
	VendorUnknown GPUVendor = "unknown"
)

// GPUDevice GPU设备信息
type GPUDevice struct {
	ID            string    `json:"id"`            // 设备ID
	UUID          string    `json:"uuid"`          // GPU UUID
	Name          string    `json:"name"`          // GPU名称
	FullName      string    `json:"fullName"`      // 完整设备名称
	Model         string    `json:"model"`         // GPU型号
	Vendor        GPUVendor `json:"vendor"`        // 厂商
	Driver        string    `json:"driver"`        // 驱动版本
	DriverOK      bool      `json:"driverOk"`      // 驱动状态正常
	DevicePath    string    `json:"devicePath"`    // 设备路径
	PCIAddress    string    `json:"pciAddress"`    // PCI地址
	PCIID         string    `json:"pciId"`         // PCI设备ID (vendor:device)
	MemoryTotal   uint64    `json:"memoryTotal"`   // 总显存(MB)
	MemoryUsed    uint64    `json:"memoryUsed"`    // 已用显存(MB)
	MemoryFree    uint64    `json:"memoryFree"`    // 可用显存(MB)
	Temperature   int       `json:"temperature"`   // 温度(°C)
	PowerUsage    uint64    `json:"powerUsage"`    // 当前功耗(W)
	PowerLimit    uint64    `json:"powerLimit"`    // 功率限制(W)
	Utilization   float64   `json:"utilization"`   // GPU利用率(%)
	MemUtil       float64   `json:"memUtil"`       // 显存利用率(%)
	FanSpeed      int       `json:"fanSpeed"`      // 风扇转速(%)
	ClockSM       int       `json:"clockSM"`       // 核心频率(MHz)
	ClockMemory   int       `json:"clockMemory"`   // 显存频率(MHz)
	Status        GPUStatus `json:"status"`        // 设备状态
	Capabilities  *GPUCapabilities `json:"capabilities"` // 设备能力
	LastUpdated   time.Time `json:"lastUpdated"`   // 最后更新时间
}

// GPUStatus GPU状态
type GPUStatus string

const (
	StatusHealthy  GPUStatus = "healthy"  // 健康
	StatusWarning  GPUStatus = "warning"  // 警告
	StatusCritical GPUStatus = "critical" // 严重
	StatusOffline  GPUStatus = "offline"  // 离线
	StatusError    GPUStatus = "error"    // 错误
)

// GPUCapabilities GPU能力信息
type GPUCapabilities struct {
	// 硬件转码能力
	TranscodeCapable bool     `json:"transcodeCapable"` // 支持硬件转码
	TranscodeFormats []string `json:"transcodeFormats"` // 支持的转码格式 (h264, h265, av1等)
	DecodeFormats    []string `json:"decodeFormats"`    // 支持的解码格式
	EncodeFormats    []string `json:"encodeFormats"`    // 支持的编码格式
	MaxEncodeStreams int      `json:"maxEncodeStreams"`  // 最大编码并发数
	MaxDecodeStreams int      `json:"maxDecodeStreams"`  // 最大解码并发数

	// AI推理能力
	InferenceCapable bool     `json:"inferenceCapable"` // 支持AI推理
	InferenceBackends []string `json:"inferenceBackends"` // 支持的推理后端 (cuda, rocm, opencl, vulkan)
	CUDACores        int      `json:"cudaCores"`        // CUDA核心数 (NVIDIA)
	ComputeCapability string  `json:"computeCapability"` // 计算能力 (NVIDIA)
	RTCores          int      `json:"rtCores"`          // RT核心数 (NVIDIA)
	TensorCores      int      `json:"tensorCores"`      // Tensor核心数 (NVIDIA)
	StreamProcessors int      `json:"streamProcessors"` // 流处理器数 (AMD)
	RayAccelerators  int      `json:"rayAccelerators"`  // 光线加速器 (AMD)
	XMXEngines       int      `json:"xmxEgines"`        // XMX引擎 (Intel)
	VectorEngines    int      `json:"vectorEngines"`    // 向量引擎 (Intel)

	// 通用能力
	MaxWorkGroupSize int    `json:"maxWorkGroupSize"` // 最大工作组大小
	MaxMemoryAlloc   uint64 `json:"maxMemoryAlloc"`   // 最大单次内存分配(MB)
	DoublePrecision  bool   `json:"doublePrecision"`  // 支持双精度浮点
	HalfPrecision    bool   `json:"halfPrecision"`    // 支持半精度浮点
	Virtualization   bool   `json:"virtualization"`   // 支持GPU虚拟化 (vGPU/MxGPU/SR-IOV)
	Passthrough      bool   `json:"passthrough"`      // 支持直通
}

// GPUCapabilityReport GPU能力报告
type GPUCapabilityReport struct {
	Timestamp       time.Time        `json:"timestamp"`
	SystemGPUs      int              `json:"systemGpus"`
	Devices         []*GPUDevice     `json:"devices"`
	Summary         *CapabilitySummary `json:"summary"`
	Recommendations []string         `json:"recommendations"`
}

// CapabilitySummary 能力摘要
type CapabilitySummary struct {
	TotalGPUs         int    `json:"totalGpus"`
	NvidiaGPUs        int    `json:"nvidiaGpus"`
	AmdGPUs           int    `json:"amdGpus"`
	IntelGPUs         int    `json:"intelGpus"`
	TotalMemoryMB     uint64 `json:"totalMemoryMb"`
	TranscodeCapable  int    `json:"transcodeCapable"`  // 支持转码的GPU数
	InferenceCapable  int    `json:"inferenceCapable"`  // 支持AI推理的GPU数
	BestForTranscode  string `json:"bestForTranscode"`  // 最佳转码GPU ID
	BestForInference  string `json:"bestForInference"`  // 最佳推理GPU ID
}

// GPUHealthReport GPU健康报告
type GPUHealthReport struct {
	Timestamp     time.Time          `json:"timestamp"`
	OverallStatus GPUStatus          `json:"overallStatus"`
	Devices       []*GPUHealthStatus `json:"devices"`
	Warnings      []string           `json:"warnings,omitempty"`
	Errors        []string           `json:"errors,omitempty"`
}

// GPUHealthStatus 单个GPU健康状态
type GPUHealthStatus struct {
	DeviceID    string    `json:"deviceId"`
	DeviceName  string    `json:"deviceName"`
	Status      GPUStatus `json:"status"`
	Temperature int       `json:"temperature"`
	TempStatus  string    `json:"tempStatus"`  // normal, warm, hot, critical
	PowerStatus string    `json:"powerStatus"` // normal, high, overlimit
	MemoryStatus string   `json:"memoryStatus"` // normal, high, full
	Warnings    []string  `json:"warnings,omitempty"`
	Errors      []string  `json:"errors,omitempty"`
	LastUpdated time.Time `json:"lastUpdated"`
}

// TranscodeCapability 转码能力详情
type TranscodeCapability struct {
	DeviceID       string   `json:"deviceId"`
	DeviceName     string   `json:"deviceName"`
	Vendor         GPUVendor `json:"vendor"`
	Capable        bool     `json:"capable"`
	Engine         string   `json:"engine"`         // NVENC/NVDEC/AMF/VA-API/QSV
	H264Encode     bool     `json:"h264Encode"`     // H.264编码
	H264Decode     bool     `json:"h264Decode"`     // H.264解码
	H265Encode     bool     `json:"h265Encode"`     // H.265/HEVC编码
	H265Decode     bool     `json:"h265Decode"`     // H.265/HEVC解码
	AV1Encode      bool     `json:"av1Encode"`      // AV1编码
	AV1Decode      bool     `json:"av1Decode"`      // AV1解码
	VP8Encode      bool     `json:"vp8Encode"`      // VP8编码
	VP8Decode      bool     `json:"vp8Decode"`      // VP8解码
	VP9Encode      bool     `json:"vp9Encode"`      // VP9编码
	VP9Decode      bool     `json:"vp9Decode"`      // VP9解码
	MaxStreams     int      `json:"maxStreams"`      // 最大并发流
	MaxResolution  string   `json:"maxResolution"`   // 最大分辨率
	HDRSupport     bool     `json:"hdrSupport"`      // HDR支持
	BitDepth8      bool     `json:"bitDepth8"`       // 8-bit支持
	BitDepth10     bool     `json:"bitDepth10"`      // 10-bit支持
}

// InferenceCapability AI推理能力详情
type InferenceCapability struct {
	DeviceID        string   `json:"deviceId"`
	DeviceName      string   `json:"deviceName"`
	Vendor          GPUVendor `json:"vendor"`
	Capable         bool     `json:"capable"`
	Backend         string   `json:"backend"`         // CUDA/ROCm/OpenCL/Vulkan
	Frameworks      []string `json:"frameworks"`      // 支持的框架 (tensorrt, onnxruntime, pytorch等)
	FP16Performance bool     `json:"fp16Performance"` // FP16性能良好
	FP32Performance bool     `json:"fp32Performance"` // FP32性能良好
	INT8Performance bool     `json:"int8Performance"` // INT8量化支持
	MaxBatchSize    int      `json:"maxBatchSize"`    // 最大批次大小
	EstimatedTOPS   float64  `json:"estimatedTops"`   // 估算算力(TOPS)
	ModelsSupported []string `json:"modelsSupported"` // 推荐支持的模型规模
	DeviceMemoryMB  uint64   `json:"deviceMemoryMb"`  // 可用显存(MB)
}

// GPUManagerConfig GPU管理器配置
type GPUManagerConfig struct {
	Enabled            bool          `json:"enabled"`            // 是否启用GPU管理
	ScanInterval       time.Duration `json:"scanInterval"`       // 扫描间隔
	HealthCheckEnabled bool          `json:"healthCheckEnabled"` // 是否启用健康检查
	HealthThresholds   *HealthThresholds `json:"healthThresholds"` // 健康阈值
	DeviceFilter       []GPUVendor   `json:"deviceFilter"`       // 设备过滤 (空=全部)
}

// HealthThresholds 健康阈值配置
type HealthThresholds struct {
	TempWarning    int     `json:"tempWarning"`    // 温度警告阈值(°C), 默认80
	TempCritical   int     `json:"tempCritical"`   // 温度严重阈值(°C), 默认90
	MemWarning     float64 `json:"memWarning"`     // 显存警告阈值(%), 默认85
	MemCritical    float64 `json:"memCritical"`    // 显存严重阈值(%), 默认95
	PowerWarning   float64 `json:"powerWarning"`   // 功耗警告阈值(%), 默认90
	PowerCritical  float64 `json:"powerCritical"`  // 功耗严重阈值(%), 默认100
	UtilWarning    float64 `json:"utilWarning"`    // 利用率警告阈值(%), 默认95
}

// DefaultHealthThresholds 返回默认健康阈值
func DefaultHealthThresholds() *HealthThresholds {
	return &HealthThresholds{
		TempWarning:   80,
		TempCritical:  90,
		MemWarning:    85.0,
		MemCritical:   95.0,
		PowerWarning:  90.0,
		PowerCritical: 100.0,
		UtilWarning:   95.0,
	}
}

// DefaultGPUManagerConfig 返回默认配置
func DefaultGPUManagerConfig() *GPUManagerConfig {
	return &GPUManagerConfig{
		Enabled:            true,
		ScanInterval:       60 * time.Second,
		HealthCheckEnabled: true,
		HealthThresholds:   DefaultHealthThresholds(),
		DeviceFilter:       nil, // 检测所有厂商
	}
}
