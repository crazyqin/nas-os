// Package gpuresource 实现 GPU 资源管理器
// 支持 GPU 设备发现、资源分配、使用率监控、温度功耗监控和多容器共享GPU
package gpuresource

import (
	"errors"
	"fmt"
	"time"
)

// 预定义错误
var (
	// ErrGPUNotFound GPU设备不存在
	ErrGPUNotFound = errors.New("gpu device not found")
	// ErrGPUAlreadyAllocated GPU已被分配
	ErrGPUAlreadyAllocated = errors.New("gpu already allocated")
	// ErrInvalidAllocation 无效的分配请求
	ErrInvalidAllocation = errors.New("invalid allocation request")
	// ErrMaxAllocationsReached 已达最大分配数量
	ErrMaxAllocationsReached = errors.New("max allocations reached")
	// ErrGPUUnavailable GPU不可用
	ErrGPUUnavailable = errors.New("gpu unavailable")
	// ErrInsufficientMemory 显存不足
	ErrInsufficientMemory = errors.New("insufficient GPU memory")
	// ErrAllocationNotFound 分配记录不存在
	ErrAllocationNotFound = errors.New("allocation not found")
	// ErrManagerClosed 管理器已关闭
	ErrManagerClosed = errors.New("manager closed")
	// ErrSharingNotSupported 不支持GPU共享
	ErrSharingNotSupported = errors.New("GPU sharing not supported")
)

// DeviceState GPU设备状态
type DeviceState string

const (
	// DeviceStateAvailable 设备可用
	DeviceStateAvailable DeviceState = "available"
	// DeviceStateAllocated 设备已分配
	DeviceStateAllocated DeviceState = "allocated"
	// DeviceStateBusy 设备繁忙
	DeviceStateBusy DeviceState = "busy"
	// DeviceStateError 设备错误
	DeviceStateError DeviceState = "error"
	// DeviceStateOffline 设备离线
	DeviceStateOffline DeviceState = "offline"
	// DeviceStateShared 设备共享中
	DeviceStateShared DeviceState = "shared"
)

// SharingMode GPU共享模式
type SharingMode string

const (
	// SharingModeNone 不共享（独占）
	SharingModeNone SharingMode = "none"
	// SharingModeMIG NVIDIA MIG（Multi-Instance GPU）模式
	SharingModeMIG SharingMode = "mig"
	// SharingModeMPS NVIDIA MPS（Multi-Process Service）模式
	SharingModeMPS SharingMode = "mps"
	// SharingModeTimeSliced 时间片共享
	SharingModeTimeSliced SharingMode = "time_sliced"
)

// AllocationPriority 分配优先级
type AllocationPriority int

const (
	// PriorityLow 低优先级
	PriorityLow AllocationPriority = 0
	// PriorityNormal 普通优先级
	PriorityNormal AllocationPriority = 1
	// PriorityHigh 高优先级
	PriorityHigh AllocationPriority = 2
	// PriorityCritical 关键优先级
	PriorityCritical AllocationPriority = 3
)

// String 返回优先级字符串表示
func (p AllocationPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// GPUDevice GPU设备信息
type GPUDevice struct {
	// ID 设备唯一标识
	ID string `json:"id"`
	// Name GPU名称（如 NVIDIA GeForce RTX 4090）
	Name string `json:"name"`
	// UUID GPU UUID
	UUID string `json:"uuid"`
	// Vendor 厂商（nvidia, amd, intel）
	Vendor string `json:"vendor"`
	// Model GPU型号
	Model string `json:"model"`
	// DriverVersion 驱动版本
	DriverVersion string `json:"driver_version"`
	// DevicePath 设备路径（如 /dev/nvidia0）
	DevicePath string `json:"device_path"`
	// PCIAddress PCI地址
	PCIAddress string `json:"pci_address"`
	// MemoryTotal 总显存（MB）
	MemoryTotal uint64 `json:"memory_total"`
	// MemoryUsed 已用显存（MB）
	MemoryUsed uint64 `json:"memory_used"`
	// MemoryFree 可用显存（MB）
	MemoryFree uint64 `json:"memory_free"`
	// Temperature 当前温度（°C）
	Temperature float64 `json:"temperature"`
	// TemperatureLimit 温度限制（°C）
	TemperatureLimit float64 `json:"temperature_limit"`
	// PowerUsage 当前功耗（W）
	PowerUsage float64 `json:"power_usage"`
	// PowerLimit 功耗限制（W）
	PowerLimit float64 `json:"power_limit"`
	// Utilization GPU核心利用率（0-100%）
	Utilization float64 `json:"utilization"`
	// MemoryUtilization 显存利用率（0-100%）
	MemoryUtilization float64 `json:"memory_utilization"`
	// State 设备状态
	State DeviceState `json:"state"`
	// SharingMode 当前共享模式
	SharingMode SharingMode `json:"sharing_mode"`
	// CUDACores CUDA核心数（NVIDIA）或流处理器数（AMD）
	ComputeUnits int `json:"compute_units"`
	// SupportsMIG 是否支持MIG
	SupportsMIG bool `json:"supports_mig"`
	// SupportsMPS 是否支持MPS
	SupportsMPS bool `json:"supports_mps"`
	// MIGInstances MIG实例列表
	MIGInstances []MIGInstance `json:"mig_instances,omitempty"`
	// LastUpdated 最后更新时间
	LastUpdated time.Time `json:"last_updated"`
}

// MIGInstance MIG实例信息
type MIGInstance struct {
	// ID 实例ID
	ID string `json:"id"`
	// ProfileName MIG配置名称（如 1g.5gb, 2g.10gb）
	ProfileName string `json:"profile_name"`
	// GPUInstanceID GPU实例ID
	GPUInstanceID int `json:"gpu_instance_id"`
	// ComputeInstanceID 计算实例ID
	ComputeInstanceID int `json:"compute_instance_id"`
	// MemorySize 实例显存大小（MB）
	MemorySize uint64 `json:"memory_size"`
	// SMCount SM数量
	SMCount int `json:"sm_count"`
	// AllocatedTo 分配给的容器/VM ID
	AllocatedTo string `json:"allocated_to,omitempty"`
	// State 实例状态
	State DeviceState `json:"state"`
}

// GPUAllocation GPU分配记录
type GPUAllocation struct {
	// AllocationID 分配ID
	AllocationID string `json:"allocation_id"`
	// ContainerID 容器/VM ID
	ContainerID string `json:"container_id"`
	// GPUID GPU设备ID
	GPUID string `json:"gpu_id"`
	// MIGInstanceID MIG实例ID（MIG模式下使用）
	MIGInstanceID string `json:"mig_instance_id,omitempty"`
	// MemoryLimit 分配的显存限制（MB）
	MemoryLimit uint64 `json:"memory_limit"`
	// ComputeLimit 计算资源限制百分比（MPS模式下使用，0-100）
	ComputeLimit float64 `json:"compute_limit"`
	// Priority 分配优先级
	Priority AllocationPriority `json:"priority"`
	// SharingMode 共享模式
	SharingMode SharingMode `json:"sharing_mode"`
	// Exclusive 是否独占
	Exclusive bool `json:"exclusive"`
	// DevicePaths 透传给容器的设备路径列表
	DevicePaths []string `json:"device_paths"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// GPUStats GPU统计信息
type GPUStats struct {
	// TotalGPUs GPU总数
	TotalGPUs int `json:"total_gpus"`
	// AvailableGPUs 可用GPU数
	AvailableGPUs int `json:"available_gpus"`
	// AllocatedGPUs 已分配GPU数
	AllocatedGPUs int `json:"allocated_gpus"`
	// SharedGPUs 共享中GPU数
	SharedGPUs int `json:"shared_gpus"`
	// TotalMemory 总显存（MB）
	TotalMemory uint64 `json:"total_memory"`
	// UsedMemory 已用显存（MB）
	UsedMemory uint64 `json:"used_memory"`
	// FreeMemory 可用显存（MB）
	FreeMemory uint64 `json:"free_memory"`
	// AvgTemperature 平均温度（°C）
	AvgTemperature float64 `json:"avg_temperature"`
	// AvgPowerUsage 平均功耗（W）
	AvgPowerUsage float64 `json:"avg_power_usage"`
	// AvgUtilization 平均GPU利用率（%）
	AvgUtilization float64 `json:"avg_utilization"`
	// ActiveAllocations 活跃分配数
	ActiveAllocations int `json:"active_allocations"`
	// DeviceStats 各设备统计
	DeviceStats []DeviceStats `json:"device_stats"`
	// CollectedAt 统计采集时间
	CollectedAt time.Time `json:"collected_at"`
}

// DeviceStats 单个GPU设备统计
type DeviceStats struct {
	// GPUID GPU设备ID
	GPUID string `json:"gpu_id"`
	// Name GPU名称
	Name string `json:"name"`
	// State 设备状态
	State DeviceState `json:"state"`
	// MemoryUsed 已用显存（MB）
	MemoryUsed uint64 `json:"memory_used"`
	// MemoryTotal 总显存（MB）
	MemoryTotal uint64 `json:"memory_total"`
	// Temperature 温度（°C）
	Temperature float64 `json:"temperature"`
	// PowerUsage 功耗（W）
	PowerUsage float64 `json:"power_usage"`
	// Utilization GPU利用率（%）
	Utilization float64 `json:"utilization"`
	// ActiveAllocations 该GPU上的活跃分配数
	ActiveAllocations int `json:"active_allocations"`
}

// GPUConfig GPU资源管理器配置
type GPUConfig struct {
	// Enabled 是否启用GPU资源管理
	Enabled bool `json:"enabled"`
	// DeviceFilter 设备路径过滤（如 ["/dev/nvidia0", "/dev/nvidia1"]）
	DeviceFilter []string `json:"device_filter,omitempty"`
	// DefaultSharingMode 默认共享模式
	DefaultSharingMode SharingMode `json:"default_sharing_mode"`
	// MaxAllocationsPerGPU 每个GPU最大分配数
	MaxAllocationsPerGPU int `json:"max_allocations_per_gpu"`
	// MaxTotalAllocations 全局最大分配数
	MaxTotalAllocations int `json:"max_total_allocations"`
	// DefaultMemoryLimitMB 默认显存限制（MB）
	DefaultMemoryLimitMB uint64 `json:"default_memory_limit_mb"`
	// DefaultComputeLimit 默认计算资源限制百分比（MPS）
	DefaultComputeLimit float64 `json:"default_compute_limit"`
	// TemperatureThreshold 温度告警阈值（°C）
	TemperatureThreshold float64 `json:"temperature_threshold"`
	// PowerThreshold 功耗告警阈值（W），0表示不限制
	PowerThreshold float64 `json:"power_threshold"`
	// MonitorIntervalSec 监控采集间隔（秒）
	MonitorIntervalSec int `json:"monitor_interval_sec"`
	// HealthCheckIntervalSec 健康检查间隔（秒）
	HealthCheckIntervalSec int `json:"health_check_interval_sec"`
}

// DefaultGPUConfig 返回默认GPU配置
func DefaultGPUConfig() *GPUConfig {
	return &GPUConfig{
		Enabled:                true,
		DeviceFilter:           nil,
		DefaultSharingMode:     SharingModeNone,
		MaxAllocationsPerGPU:   1,
		MaxTotalAllocations:    10,
		DefaultMemoryLimitMB:   4096,
		DefaultComputeLimit:    50.0,
		TemperatureThreshold:   85.0,
		PowerThreshold:         0,
		MonitorIntervalSec:     5,
		HealthCheckIntervalSec: 30,
	}
}

// Validate 校验配置合法性
func (c *GPUConfig) Validate() error {
	if c.MaxAllocationsPerGPU < 0 {
		return fmt.Errorf("MaxAllocationsPerGPU must be >= 0, got %d", c.MaxAllocationsPerGPU)
	}
	if c.MaxTotalAllocations < 0 {
		return fmt.Errorf("MaxTotalAllocations must be >= 0, got %d", c.MaxTotalAllocations)
	}
	if c.DefaultComputeLimit < 0 || c.DefaultComputeLimit > 100 {
		return fmt.Errorf("DefaultComputeLimit must be 0-100, got %.1f", c.DefaultComputeLimit)
	}
	if c.MonitorIntervalSec < 0 {
		return fmt.Errorf("MonitorIntervalSec must be >= 0, got %d", c.MonitorIntervalSec)
	}
	if c.HealthCheckIntervalSec < 0 {
		return fmt.Errorf("HealthCheckIntervalSec must be >= 0, got %d", c.HealthCheckIntervalSec)
	}
	return nil
}

// HealthStatus GPU健康状态
type HealthStatus struct {
	// Overall 整体状态（healthy / warning / critical）
	Overall string `json:"overall"`
	// DeviceHealth 各设备健康状态
	DeviceHealth map[string]DeviceHealthStatus `json:"device_health"`
	// Warnings 警告信息
	Warnings []string `json:"warnings"`
	// Errors 错误信息
	Errors []string `json:"errors"`
	// CheckedAt 检查时间
	CheckedAt time.Time `json:"checked_at"`
}

// DeviceHealthStatus 单设备健康状态
type DeviceHealthStatus struct {
	// Healthy 是否健康
	Healthy bool `json:"healthy"`
	// TemperatureOK 温度正常
	TemperatureOK bool `json:"temperature_ok"`
	// PowerOK 功耗正常
	PowerOK bool `json:"power_ok"`
	// MemoryOK 显存正常
	MemoryOK bool `json:"memory_ok"`
	// Message 状态消息
	Message string `json:"message"`
}
