// Package gpumonitor 实现 GPU 监控管理模块
// 支持 NVIDIA/AMD/Intel GPU 实时监控、温度告警、显存管理、CUDA/ROCm 状态
package gpumonitor

import (
	"errors"
	"time"
)

var (
	ErrGPUNotFound     = errors.New("gpu not found")
	ErrGPUNotSupported = errors.New("gpu not supported")
	ErrInvalidQuery    = errors.New("invalid query")
)

// GPUVendor GPU 厂商.
type GPUVendor string

const (
	VendorNVIDIA GPUVendor = "nvidia"
	VendorAMD    GPUVendor = "amd"
	VendorIntel  GPUVendor = "intel"
)

// GPUState GPU 状态.
type GPUState string

const (
	StateIdle       GPUState = "idle"
	StateComputing  GPUState = "computing"
	StateMemoryFull GPUState = "memory_full"
	StateThrottled  GPUState = "throttled"
	StateError      GPUState = "error"
	StateOffline    GPUState = "offline"
)

// GPU 信息.
type GPU struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Vendor         GPUVendor    `json:"vendor"`
	Driver         string       `json:"driver"`
	VRAMTotal      int64        `json:"vram_total"` // MB
	VRAMUsed       int64        `json:"vram_used"`
	Temperature    float64      `json:"temperature"` // ℃
	FanSpeed       int          `json:"fan_speed"`   // 0-100%
	PowerDraw      float64      `json:"power_draw"`  // W
	PowerLimit     float64      `json:"power_limit"`
	CoreClock      int          `json:"core_clock"` // MHz
	MemoryClock    int          `json:"memory_clock"`
	UtilizationGPU int          `json:"utilization_gpu"` // 0-100%
	UtilizationMem int          `json:"utilization_mem"`
	State          GPUState     `json:"state"`
	Processes      []GPUProcess `json:"processes"`
	LastUpdated    time.Time    `json:"last_updated"`
}

// GPUProcess GPU 进程.
type GPUProcess struct {
	PID      int    `json:"pid"`
	Name     string `json:"name"`
	VRAMUsed int64  `json:"vram_used"` // MB
	GPUUtil  int    `json:"gpu_util"`
	Type     string `json:"type"` // "C" (CUDA), "G" (Graphics), "X" (Unknown)
}

// GPUMetrics 历史指标.
type GPUMetrics struct {
	GPUID     string    `json:"gpu_id"`
	Timestamp time.Time `json:"timestamp"`
	Temp      float64   `json:"temp"`
	Power     float64   `json:"power"`
	VRAMUsed  int64     `json:"vram_used"`
	GPUUtil   int       `json:"gpu_util"`
}

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

// GPUAlert GPU 告警.
type GPUAlert struct {
	GPUID     string     `json:"gpu_id"`
	Level     AlertLevel `json:"level"`
	Message   string     `json:"message"`
	Value     float64    `json:"value"`
	Threshold float64    `json:"threshold"`
	Timestamp time.Time  `json:"timestamp"`
}

// GPUConfig GPU 监控配置.
type GPUConfig struct {
	MonitorInterval time.Duration `json:"monitor_interval"`
	TempWarning     float64       `json:"temp_warning"`  // 80℃
	TempCritical    float64       `json:"temp_critical"` // 90℃
	PowerWarning    float64       `json:"power_warning"` // 90% limit
	VRAMWarning     float64       `json:"vram_warning"`  // 80%
	RetentionDays   int           `json:"retention_days"`
}

// GPUStats GPU 统计.
type GPUStats struct {
	TotalGPUs  int            `json:"total_gpus"`
	OnlineGPUs int            `json:"online_gpus"`
	TotalVRAM  int64          `json:"total_vram"` // MB
	UsedVRAM   int64          `json:"used_vram"`
	AvgTemp    float64        `json:"avg_temp"`
	AvgUtil    float64        `json:"avg_util"`
	TotalPower float64        `json:"total_power"`
	VendorDist map[string]int `json:"vendor_dist"`
}
