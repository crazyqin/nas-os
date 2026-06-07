// Package gpupassthrough GPU直通管理模块
package gpupassthrough

import (
	"time"
)

// GPUDevice GPU设备信息
type GPUDevice struct {
	PCIAddress      string                `json:"pciAddress"`           // PCI地址 (如 0000:01:00.0)
	VendorID        string                `json:"vendorId"`             // 厂商ID
	DeviceID        string                `json:"deviceId"`             // 设备ID
	Model           string                `json:"model"`                // GPU型号 (如 NVIDIA GeForce RTX 3080)
	Vendor          string                `json:"vendor"`               // 厂商名称 (nvidia, amd, intel)
	Driver          string                `json:"driver"`               // 当前驱动 (nvidia, vfio-pci 等)
	VRAM            uint64                `json:"vram"`                 // 显存(MB)
	VRAMUsed        uint64                `json:"vramUsed"`             // 已用显存(MB)
	Temperature     int                   `json:"temperature"`          // 温度(°C)
	PowerUsage      uint64                `json:"powerUsage"`           // 当前功耗(W)
	PowerLimit      uint64                `json:"powerLimit"`           // 功率限制(W)
	BindState       BindState             `json:"bindState"`            // 绑定状态
	IOMMUGroup      int                   `json:"iommuGroup"`           // IOMMU组
	NUMANode        int                   `json:"numaNode"`             // NUMA节点
	DevicePath      string                `json:"devicePath"`           // 设备路径 (如 /dev/vfio/0)
	Status          DeviceStatus          `json:"status"`               // 设备状态
	VMAssignments   []VMAssignment        `json:"vmAssignments"`        // VM分配列表
	ContainerAssign []ContainerAssignment `json:"containerAssignments"` // 容器分配列表
	UpdatedAt       time.Time             `json:"updatedAt"`            // 更新时间
}

// VMAssignment VM分配信息
type VMAssignment struct {
	VMID       string     `json:"vmId"`                // 虚拟机ID
	GPUPCIAddr string     `json:"gpuPciAddr"`          // GPU PCI地址
	Status     string     `json:"status"`              // 分配状态 (active, inactive, pending)
	AssignedAt time.Time  `json:"assignedAt"`          // 分配时间
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"` // 过期时间(可选)
}

// ContainerAssignment 容器分配信息
type ContainerAssignment struct {
	ContainerID string    `json:"containerId"` // 容器ID
	GPUPCIAddr  string    `json:"gpuPciAddr"`  // GPU PCI地址
	ShareMode   ShareMode `json:"shareMode"`   // 共享模式
	MemoryLimit uint64    `json:"memoryLimit"` // 显存限制(MB)
	AssignedAt  time.Time `json:"assignedAt"`  // 分配时间
}

// GPUStats GPU实时统计
type GPUStats struct {
	PCIAddress  string    `json:"pciAddress"`  // PCI地址
	GPUUsage    float64   `json:"gpuUsage"`    // GPU使用率(%)
	MemoryUsage float64   `json:"memoryUsage"` // 显存使用率(%)
	MemoryUsed  uint64    `json:"memoryUsed"`  // 已用显存(MB)
	MemoryTotal uint64    `json:"memoryTotal"` // 总显存(MB)
	Temperature int       `json:"temperature"` // 温度(°C)
	PowerUsage  uint64    `json:"powerUsage"`  // 功耗(W)
	FanSpeed    int       `json:"fanSpeed"`    // 风扇转速(%)
	ClockSM     int       `json:"clockSm"`     // SM频率(MHz)
	ClockMemory int       `json:"clockMemory"` // 显存频率(MHz)
	UpdatedAt   time.Time `json:"updatedAt"`   // 更新时间
}

// BindState 绑定状态
type BindState string

const (
	BindStateNative BindState = "native" // 使用原生驱动
	BindStateVfio   BindState = "vfio"   // 绑定到vfio-pci
	BindStateUnbind BindState = "unbind" // 未绑定任何驱动
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusAvailable DeviceStatus = "available" // 可用
	DeviceStatusAssigned  DeviceStatus = "assigned"  // 已分配
	DeviceStatusError     DeviceStatus = "error"     // 错误
	DeviceStatusOffline   DeviceStatus = "offline"   // 离线
)

// ShareMode GPU共享模式
type ShareMode string

const (
	ShareModeExclusive ShareMode = "exclusive" // 独占模式
	ShareModeTimeslice ShareMode = "timeslice" // 时间片共享
	ShareModeMPS       ShareMode = "mps"       // NVIDIA MPS共享
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo    AlertLevel = "info"    // 信息
	AlertLevelWarning AlertLevel = "warning" // 警告
	AlertLevelError   AlertLevel = "error"   // 错误
)

// GPUAlert GPU告警
type GPUAlert struct {
	Level      AlertLevel `json:"level"`      // 告警级别
	PCIAddress string     `json:"pciAddress"` // GPU PCI地址
	Message    string     `json:"message"`    // 告警消息
	Timestamp  time.Time  `json:"timestamp"`  // 告警时间
}

// AssignRequest 分配请求
type AssignRequest struct {
	TargetType  string `json:"targetType"`  // 目标类型 (vm, container)
	TargetID    string `json:"targetId"`    // 目标ID
	ShareMode   string `json:"shareMode"`   // 共享模式
	MemoryLimit uint64 `json:"memoryLimit"` // 显存限制(MB)
}

// BindRequest 绑定请求
type BindRequest struct {
	Driver string `json:"driver"` // 目标驱动 (vfio-pci, nvidia 等)
}

// Response 统一API响应
type Response struct {
	Code    int         `json:"code"`    // 响应码 (0=成功)
	Message string      `json:"message"` // 响应消息
	Data    interface{} `json:"data"`    // 响应数据
}

// Config GPU直通配置
type Config struct {
	Enabled          bool   `json:"enabled"`          // 是否启用
	ConfigPath       string `json:"configPath"`       // 配置文件路径
	AlertTempWarning int    `json:"alertTempWarning"` // 温度告警阈值(°C)
	AlertTempError   int    `json:"alertTempError"`   // 温度错误阈值(°C)
	AlertPowerLimit  uint64 `json:"alertPowerLimit"`  // 功耗告警阈值(W)
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:          true,
		ConfigPath:       "/etc/nas-os/gpupassthrough.json",
		AlertTempWarning: 80,
		AlertTempError:   90,
		AlertPowerLimit:  300,
	}
}
