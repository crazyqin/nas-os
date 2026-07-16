// Package lxcgpu LXC容器GPU直通集成模块
// 实现GPU设备在LXC容器中的分配、热插拔、资源配额管理
// 对标 TrueNAS 26 的LXC容器GPU直通功能
package lxcgpu

import (
	"fmt"
	"time"
)

// GPUVendor GPU厂商类型.
type GPUVendor string

const (
	GPUVendorNVIDIA  GPUVendor = "nvidia"
	GPUVendorAMD     GPUVendor = "amd"
	GPUVendorIntel   GPUVendor = "intel"
	GPUVendorUnknown GPUVendor = "unknown"
)

// ShareMode GPU共享模式.
type ShareMode string

const (
	ShareModeExclusive ShareMode = "exclusive" // 独占模式 - 容器独占GPU
	ShareModeTimeslice ShareMode = "timeslice" // 时间片共享
	ShareModeMPS       ShareMode = "mps"       // NVIDIA MPS共享模式
)

// AssignmentState 分配状态.
type AssignmentState string

const (
	AssignmentStatePending  AssignmentState = "pending"  // 等待分配
	AssignmentStateActive   AssignmentState = "active"   // 已激活
	AssignmentStateInactive AssignmentState = "inactive" // 未激活
	AssignmentStateError    AssignmentState = "error"    // 错误
)

// HotplugState 热插拔状态.
type HotplugState string

const (
	HotplugStateIdle      HotplugState = "idle"      // 空闲
	HotplugStateAttaching HotplugState = "attaching" // 正在附加
	HotplugStateAttached  HotplugState = "attached"  // 已附加
	HotplugStateDetaching HotplugState = "detaching" // 正在分离
	HotplugStateError     HotplugState = "error"     // 错误
)

// GPUDevice GPU设备信息（扩展自gpupassthrough，增加LXC相关字段）.
type GPUDevice struct {
	PCIAddress   string              `json:"pciAddress"`   // PCI地址
	VendorID     string              `json:"vendorId"`     // 厂商ID
	DeviceID     string              `json:"deviceId"`     // 设备ID
	Model        string              `json:"model"`        // GPU型号
	Vendor       GPUVendor           `json:"vendor"`       // 厂商
	Driver       string              `json:"driver"`       // 当前驱动
	VRAM         uint64              `json:"vram"`         // 显存(MB)
	VRAMUsed     uint64              `json:"vramUsed"`     // 已用显存(MB)
	Temperature  int                 `json:"temperature"`  // 温度(°C)
	NUMANode     int                 `json:"numaNode"`     // NUMA节点
	DevicePath   string              `json:"devicePath"`   // 设备路径
	Available    bool                `json:"available"`    // 是否可用
	Assignments  []*LXCGPUAssignment `json:"assignments"`  // 分配列表
	Capabilities GPUCapabilities     `json:"capabilities"` // 设备能力
	UpdatedAt    time.Time           `json:"updatedAt"`    // 更新时间
}

// GPUCapabilities GPU能力信息.
type GPUCapabilities struct {
	SupportsMPS       bool   `json:"supportsMps"`       // 是否支持MPS
	SupportsVGPU      bool   `json:"supportsVgpu"`      // 是否支持vGPU
	MaxInstances      int    `json:"maxInstances"`      // 最大实例数
	ComputeCapability string `json:"computeCapability"` // 计算能力(如 "8.6")
	DriverVersion     string `json:"driverVersion"`     // 驱动版本
	CUDADriverVersion string `json:"cudaDriverVersion"` // CUDA驱动版本
}

// LXCGPUAssignment LXC容器GPU分配记录.
type LXCGPUAssignment struct {
	ID            string          `json:"id"`              // 分配ID
	ContainerID   string          `json:"containerId"`     // 容器ID
	GPUPCIAddr    string          `json:"gpuPciAddr"`      // GPU PCI地址
	ShareMode     ShareMode       `json:"shareMode"`       // 共享模式
	State         AssignmentState `json:"state"`           // 分配状态
	HotplugState  HotplugState    `json:"hotplugState"`    // 热插拔状态
	GPUQuota      GPUQuota        `json:"gpuQuota"`        // GPU资源配额
	AssignedAt    time.Time       `json:"assignedAt"`      // 分配时间
	ActivatedAt   *time.Time      `json:"activatedAt"`     // 激活时间
	DeactivatedAt *time.Time      `json:"deactivatedAt"`   // 停用时间
	LastHotplugAt *time.Time      `json:"lastHotplugAt"`   // 最后热插拔时间
	Error         string          `json:"error,omitempty"` // 错误信息
	RetryCount    int             `json:"retryCount"`      // 重试次数
}

// GPUQuota GPU资源配额.
type GPUQuota struct {
	MemoryLimitMB   uint64 `json:"memoryLimitMb"`   // 显存限制(MB)
	MemoryGuarantee uint64 `json:"memoryGuarantee"` // 显存保证(MB)
	SMPercent       int    `json:"smPercent"`       // SM使用率限制(1-100)
	Priority        int    `json:"priority"`        // 优先级(0-100, 0=最高)
	MaxComputeInst  int    `json:"maxComputeInst"`  // 最大并发计算实例数
}

// Validate 验证GPU配额.
func (q GPUQuota) Validate() error {
	if q.SMPercent < 0 || q.SMPercent > 100 {
		return fmt.Errorf("SM使用率限制必须在0-100之间")
	}
	if q.Priority < 0 || q.Priority > 100 {
		return fmt.Errorf("优先级必须在0-100之间")
	}
	if q.MemoryLimitMB > 0 && q.MemoryGuarantee > q.MemoryLimitMB {
		return fmt.Errorf("显存保证不能超过显存限制")
	}
	return nil
}

// AssignGPURequest 分配GPU请求.
type AssignGPURequest struct {
	ContainerID string    `json:"containerId" binding:"required"` // 容器ID
	GPUPCIAddr  string    `json:"gpuPciAddr" binding:"required"`  // GPU PCI地址
	ShareMode   ShareMode `json:"shareMode"`                      // 共享模式
	GPUQuota    GPUQuota  `json:"gpuQuota"`                       // GPU配额
	Hotplug     bool      `json:"hotplug"`                        // 是否热插拔(运行中容器)
}

// UnassignGPURequest 取消GPU分配请求.
type UnassignGPURequest struct {
	ContainerID string `json:"containerId" binding:"required"` // 容器ID
	GPUPCIAddr  string `json:"gpuPciAddr" binding:"required"`  // GPU PCI地址
	Force       bool   `json:"force"`                          // 强制移除
}

// UpdateQuotaRequest 更新配额请求.
type UpdateQuotaRequest struct {
	ContainerID string   `json:"containerId" binding:"required"` // 容器ID
	GPUPCIAddr  string   `json:"gpuPciAddr" binding:"required"`  // GPU PCI地址
	GPUQuota    GPUQuota `json:"gpuQuota"`                       // 新配额
}

// HotplugRequest 热插拔请求.
type HotplugRequest struct {
	ContainerID string `json:"containerId" binding:"required"` // 容器ID
	GPUPCIAddr  string `json:"gpuPciAddr" binding:"required"`  // GPU PCI地址
	Action      string `json:"action" binding:"required"`      // attach 或 detach
}

// LXCContainerStatus 容器状态（简化版，用于LXC-GPU模块）.
type LXCContainerStatus string

const (
	LXCContainerRunning  LXCContainerStatus = "running"
	LXCContainerStopped  LXCContainerStatus = "stopped"
	LXCContainerStarting LXCContainerStatus = "starting"
	LXCContainerError    LXCContainerStatus = "error"
)

// LXCContainerInfo 容器信息（简化版）.
type LXCContainerInfo struct {
	ID     string             `json:"id"`
	Name   string             `json:"name"`
	Status LXCContainerStatus `json:"status"`
	PID    int                `json:"pid"`
	GPUs   []string           `json:"gpus"` // 已分配的GPU PCI地址列表
}

// GPUDashboard GPU分配仪表盘数据.
type GPUDashboard struct {
	TotalGPUs      int                 `json:"totalGpus"`
	AvailableGPUs  int                 `json:"availableGpus"`
	AssignedGPUs   int                 `json:"assignedGpus"`
	ErrorGPUs      int                 `json:"errorGpus"`
	Assignments    []*LXCGPUAssignment `json:"assignments"`
	GPUs           []*GPUDevice        `json:"gpus"`
	ContainerStats []ContainerGPUStats `json:"containerStats"`
}

// ContainerGPUStats 容器GPU统计.
type ContainerGPUStats struct {
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	GPUCount      int    `json:"gpuCount"`
	TotalVRAMMB   uint64 `json:"totalVramMb"`
	UsedVRAMMB    uint64 `json:"usedVramMb"`
}

// BulkAssignRequest 批量分配请求.
type BulkAssignRequest struct {
	ContainerIDs []string  `json:"containerIds" binding:"required"` // 容器ID列表
	GPUPCIAddr   string    `json:"gpuPciAddr" binding:"required"`   // GPU PCI地址
	ShareMode    ShareMode `json:"shareMode"`                       // 共享模式
	GPUQuota     GPUQuota  `json:"gpuQuota"`                        // GPU配额
}

// LXCConfig LXC配置路径.
type LXCConfig struct {
	ConfigPath    string `json:"configPath"`    // LXC容器配置目录
	DeviceCGroup  string `json:"deviceCgroup"`  // cgroup设备控制器路径
	HotplugSocket string `json:"hotplugSocket"` // 热插拔通信socket
}

// DefaultLXCConfig 默认LXC配置.
func DefaultLXCConfig() *LXCConfig {
	return &LXCConfig{
		ConfigPath:    "/var/lib/lxc",
		DeviceCGroup:  "/sys/fs/cgroup/lxc",
		HotplugSocket: "/var/run/nas-os/lxcgpu.sock",
	}
}

// APIResponse 统一API响应格式.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// StatsLXCGPU LXC GPU统计信息.
type StatsLXCGPU struct {
	ContainerID  string    `json:"containerId"`
	GPUPCIAddr   string    `json:"gpuPciAddr"`
	GPUUsage     float64   `json:"gpuUsage"`     // GPU使用率(%)
	MemoryUsage  float64   `json:"memoryUsage"`  // 显存使用率(%)
	MemoryUsedMB uint64    `json:"memoryUsedMb"` // 已用显存(MB)
	Temperature  int       `json:"temperature"`  // 温度(°C)
	UpdatedAt    time.Time `json:"updatedAt"`
}
