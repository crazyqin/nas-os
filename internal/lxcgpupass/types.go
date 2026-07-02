// Package lxcgpupass 实现 LXC 容器 GPU 直通管理
// 对标 TrueNAS 26 的 LXC GPU Passthrough 功能
// 提供 GPU 设备检测、分配、移除和状态查询能力
package lxcgpupass

import (
	"fmt"
	"time"
)

// GPUVendor GPU 厂商类型.
type GPUVendor string

const (
	GPUVendorNVIDIA  GPUVendor = "nvidia"  // NVIDIA GPU
	GPUVendorAMD     GPUVendor = "amd"     // AMD GPU
	GPUVendorIntel   GPUVendor = "intel"   // Intel GPU
	GPUVendorUnknown GPUVendor = "unknown" // 未知厂商
)

// AssignmentState GPU 分配状态.
type AssignmentState string

const (
	AssignmentStateActive   AssignmentState = "active"   // 已激活
	AssignmentStateInactive AssignmentState = "inactive" // 未激活
	AssignmentStateError    AssignmentState = "error"    // 错误
)

// GPUDevice GPU 设备信息.
type GPUDevice struct {
	PCIAddress  string    `json:"pciAddress"`  // PCI 地址，如 0000:01:00.0
	VendorID    string    `json:"vendorId"`    // 厂商 ID（如 10de=NVIDIA）
	DeviceID    string    `json:"deviceId"`    // 设备 ID
	Model       string    `json:"model"`       // GPU 型号描述
	Vendor      GPUVendor `json:"vendor"`      // 厂商枚举
	Driver      string    `json:"driver"`      // 当前驱动名称
	VRAM        uint64    `json:"vram"`        // 显存大小（MB）
	NUMANode    int       `json:"numaNode"`    // NUMA 节点编号
	DevicePath  string    `json:"devicePath"`  // 设备文件路径
	IOMMUGroup  int       `json:"iommuGroup"`  // IOMMU 分组编号
	Available   bool      `json:"available"`   // 是否可用于分配
	Assigned    bool      `json:"assigned"`    // 是否已分配给容器
	ContainerID string    `json:"containerId"` // 当前分配的容器 ID（空表示未分配）
	UpdatedAt   time.Time `json:"updatedAt"`   // 最后更新时间
}

// GPUAssignment GPU 分配记录.
type GPUAssignment struct {
	ID          string          `json:"id"`              // 分配记录 ID
	ContainerID string          `json:"containerId"`     // LXC 容器 ID
	GPUPCIAddr  string          `json:"gpuPciAddr"`      // GPU PCI 地址
	State       AssignmentState `json:"state"`           // 分配状态
	AssignedAt  time.Time       `json:"assignedAt"`      // 分配时间
	Error       string          `json:"error,omitempty"` // 错误信息
}

// AssignRequest 分配 GPU 到容器请求.
type AssignRequest struct {
	ContainerID string `json:"containerId" binding:"required"` // 容器 ID
	GPUPCIAddr  string `json:"gpuPciAddr" binding:"required"`  // GPU PCI 地址
}

// RemoveRequest 从容器移除 GPU 请求.
type RemoveRequest struct {
	ContainerID string `json:"containerId" binding:"required"` // 容器 ID
	GPUPCIAddr  string `json:"gpuPciAddr binding:"`            // GPU PCI 地址（可选，为空则移除该容器所有 GPU）
	Force       bool   `json:"force"`                          // 是否强制移除
}

// GPUStatus GPU 分配状态总览.
type GPUStatus struct {
	TotalGPUs     int              `json:"totalGpus"`     // GPU 总数
	AvailableGPUs int              `json:"availableGpus"` // 可用 GPU 数
	AssignedGPUs  int              `json:"assignedGpus"`  // 已分配 GPU 数
	Assignments   []*GPUAssignment `json:"assignments"`   // 所有分配记录
}

// APIResponse 统一 API 响应格式.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Config 模块配置.
type Config struct {
	SysPCIBase string // /sys/bus/pci/devices 基路径
	DevBase    string // /dev 基路径
	LXCBase    string // LXC 容器配置根目录
	CGroupBase string // cgroup v2 基路径
}

// DefaultConfig 默认配置.
func DefaultConfig() *Config {
	return &Config{
		SysPCIBase: "/sys/bus/pci/devices",
		DevBase:    "/dev",
		LXCBase:    "/var/lib/lxc",
		CGroupBase: "/sys/fs/cgroup",
	}
}

// Validate 校验 PCI 地址格式（简化版：xxxx:xx:xx.x）.
func ValidatePCIAddr(addr string) error {
	if len(addr) < len("0000:00:00.0") {
		return fmt.Errorf("PCI 地址格式无效: %s", addr)
	}
	// 基本格式检查
	parts := 0
	for _, c := range addr {
		if c == ':' || c == '.' {
			parts++
		}
	}
	if parts != 3 {
		return fmt.Errorf("PCI 地址格式无效: %s（期望 xxxx:xx:xx.x）", addr)
	}
	return nil
}
