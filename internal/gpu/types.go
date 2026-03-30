// Package gpu GPU容器调度模块
package gpu

import (
	"time"
)

// GPUDevice GPU设备信息
type GPUDevice struct {
	ID          string    `json:"id"`          // 设备ID (如 nvidia0)
	UUID        string    `json:"uuid"`        // GPU UUID
	Name        string    `json:"name"`        // GPU名称 (如 NVIDIA GeForce RTX 3080)
	Model       string    `json:"model"`       // GPU型号
	Vendor      string    `json:"vendor"`      // 厂商 (nvidia, amd, intel)
	Driver      string    `json:"driver"`      // 驱动版本
	MemoryTotal uint64    `json:"memoryTotal"` // 总显存(MB)
	MemoryUsed  uint64    `json:"memoryUsed"`  // 已用显存(MB)
	MemoryFree  uint64    `json:"memoryFree"`  // 可用显存(MB)
	CUDAcores   int       `json:"cudaCores"`   // CUDA核心数
	PowerLimit  uint64    `json:"powerLimit"`  // 功率限制(W)
	PowerUsage  uint64    `json:"powerUsage"`  // 当前功耗(W)
	Temperature int       `json:"temperature"` // 温度(°C)
	Status      GPUStatus `json:"status"`      // 设备状态
	Allocated   bool      `json:"allocated"`   // 是否已分配
	AllocatedTo string    `json:"allocatedTo"` // 分配给的容器/VM ID
	AllocatedAt time.Time `json:"allocatedAt"` // 分配时间
	DevicePath  string    `json:"devicePath"`  // 设备路径 (如 /dev/nvidia0)
	PCIAddress  string    `json:"pciAddress"`  // PCI地址
}

// GPUStatus GPU状态
type GPUStatus string

const (
	GPUStatusAvailable GPUStatus = "available" // 可用
	GPUStatusAllocated GPUStatus = "allocated" // 已分配
	GPUStatusBusy      GPUStatus = "busy"      // 繁忙
	GPUStatusError     GPUStatus = "error"     // 错误
	GPUStatusOffline   GPUStatus = "offline"   // 离线
)

// GPUAllocation GPU分配请求
type GPUAllocation struct {
	RequestID   string             `json:"requestId"`   // 请求ID
	ContainerID string             `json:"containerId"` // 容器ID
	GPUID       string             `json:"gpuId"`       // GPU设备ID
	MemoryLimit uint64             `json:"memoryLimit"` // 显存限制(MB)
	CUDALimit   int                `json:"cudaLimit"`   // CUDA核心限制
	Priority    AllocationPriority `json:"priority"`    // 分配优先级
	Exclusive   bool               `json:"exclusive"`   // 是否独占
	CreatedAt   time.Time          `json:"createdAt"`   // 创建时间
}

// AllocationPriority 分配优先级
type AllocationPriority string

const (
	PriorityLow      AllocationPriority = "low"      // 低优先级
	PriorityNormal   AllocationPriority = "normal"   // 正常优先级
	PriorityHigh     AllocationPriority = "high"     // 高优先级
	PriorityCritical AllocationPriority = "critical" // 关键优先级
)

// GPUAllocationResult GPU分配结果
type GPUAllocationResult struct {
	Success     bool     `json:"success"`     // 是否成功
	RequestID   string   `json:"requestId"`   // 请求ID
	GPUID       string   `json:"gpuId"`       // 分配的GPU ID
	DevicePaths []string `json:"devicePaths"` // 设备路径列表
	MemoryLimit uint64   `json:"memoryLimit"` // 显存限制
	CUDALimit   int      `json:"cudaLimit"`   // CUDA核心限制
	Message     string   `json:"message"`     // 结果消息
}

// GPUReleaseRequest GPU释放请求
type GPUReleaseRequest struct {
	RequestID   string `json:"requestId"`   // 请求ID
	ContainerID string `json:"containerId"` // 容器ID
	GPUID       string `json:"gpuId"`       // GPU设备ID (可选)
}

// GPUStats GPU统计信息
type GPUStats struct {
	TotalGPUs      int             `json:"totalGpus"`      // GPU总数
	AvailableGPUs  int             `json:"availableGpus"`  // 可用GPU数
	AllocatedGPUs  int             `json:"allocatedGpus"`  // 已分配GPU数
	TotalMemory    uint64          `json:"totalMemory"`    // 总显存(MB)
	UsedMemory     uint64          `json:"usedMemory"`     // 已用显存(MB)
	FreeMemory     uint64          `json:"freeMemory"`     // 可用显存(MB)
	AvgTemperature int             `json:"avgTemperature"` // 平均温度
	AvgPowerUsage  uint64          `json:"avgPowerUsage"`  // 平均功耗
	Utilization    float64         `json:"utilization"`    // GPU利用率(%)
	Allocations    []GPUAllocation `json:"allocations"`    // 当前分配列表
	HealthStatus   GPUHealthStatus `json:"healthStatus"`   // 健康状态
}

// GPUHealthStatus GPU健康状态
type GPUHealthStatus struct {
	Status    string          `json:"status"`    // 整体状态 (healthy, warning, critical)
	Warnings  []string        `json:"warnings"`  // 警告信息
	Errors    []string        `json:"errors"`    // 错误信息
	LastCheck time.Time       `json:"lastCheck"` // 最后检查时间
	DriverOK  bool            `json:"driverOk"`  // 驱动状态
	DevicesOK map[string]bool `json:"devicesOk"` // 各设备状态
}

// GPUConfig GPU配置
type GPUConfig struct {
	GPUEnabled          bool     `json:"gpuEnabled"`          // 是否启用GPU
	GPUDevices          []string `json:"gpuDevices"`          // GPU设备路径列表
	DefaultMemoryLimit  string   `json:"defaultMemoryLimit"`  // 默认显存限制 (如 "4g")
	DefaultCUDACores    int      `json:"defaultCudaCores"`    // 默认CUDA核心限制
	SchedulerPolicy     string   `json:"schedulerPolicy"`     // 调度策略 (round-robin, priority, exclusive)
	MaxAllocations      int      `json:"maxAllocations"`      // 最大同时分配数
	HealthCheckInterval int      `json:"healthCheckInterval"` // 健康检查间隔(秒)
	MonitorInterval     int      `json:"monitorInterval"`     // 监控间隔(秒)
}

// DefaultGPUConfig 默认GPU配置
func DefaultGPUConfig() *GPUConfig {
	return &GPUConfig{
		GPUEnabled:          true,
		GPUDevices:          []string{},
		DefaultMemoryLimit:  "4g",
		DefaultCUDACores:    1000,
		SchedulerPolicy:     "round-robin",
		MaxAllocations:      10,
		HealthCheckInterval: 30,
		MonitorInterval:     5,
	}
}

// GPUDeviceFilter GPU设备过滤器
type GPUDeviceFilter struct {
	Vendor       string    // 厂商过滤
	Model        string    // 型号过滤
	MinMemory    uint64    // 最小显存(MB)
	MinCUDACores int       // 最小CUDA核心数
	Status       GPUStatus // 状态过滤
	ExcludeIDs   []string  // 排除的设备ID
	OnlyFree     bool      // 只返回空闲设备
}

// GPUAllocationPolicy GPU分配策略接口
type GPUAllocationPolicy interface {
	// SelectGPU 选择合适的GPU设备
	SelectGPU(devices []*GPUDevice, req *GPUAllocation) (*GPUDevice, error)
	// Name 返回策略名称
	Name() string
}
