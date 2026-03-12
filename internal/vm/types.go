package vm

import "time"

// VMStatus 虚拟机状态
type VMStatus string

const (
	VMStatusRunning    VMStatus = "running"
	VMStatusStopped    VMStatus = "stopped"
	VMStatusPaused     VMStatus = "paused"
	VMStatusCreating   VMStatus = "creating"
	VMStatusDeleting   VMStatus = "deleting"
	VMStatusSnapshot   VMStatus = "snapshotting"
	VMStatusRestoring  VMStatus = "restoring"
)

// VMType 虚拟机类型
type VMType string

const (
	VMTypeLinux   VMType = "linux"
	VMTypeWindows VMType = "windows"
	VMTypeOther   VMType = "other"
)

// VM 虚拟机信息
type VM struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        VMType            `json:"type"`
	Status      VMStatus          `json:"status"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	
	// 资源配置
	CPU        int    `json:"cpu"`        // CPU 核心数
	Memory     uint64 `json:"memory"`     // 内存大小 (MB)
	DiskSize   uint64 `json:"diskSize"`   // 磁盘大小 (GB)
	Network    string `json:"network"`    // 网络模式：bridge/nat
	
	// 镜像信息
	ISOPath    string `json:"isoPath"`    // ISO 镜像路径
	DiskPath   string `json:"diskPath"`   // 磁盘镜像路径
	
	// VNC 配置
	VNCPort    int    `json:"vncPort"`    // VNC 端口
	VNCEnabled bool   `json:"vncEnabled"` // 是否启用 VNC
	
	// 硬件直通
	USBDevices []string `json:"usbDevices"` // USB 设备 ID 列表
	PCIDevices []string `json:"pciDevices"` // PCIe 设备 ID 列表
	
	// 标签
	Tags map[string]string `json:"tags"`
}

// VMConfig 虚拟机配置
type VMConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        VMType            `json:"type"`
	CPU         int               `json:"cpu"`
	Memory      uint64            `json:"memory"`
	DiskSize    uint64            `json:"diskSize"`
	Network     string            `json:"network"`
	ISOPath     string            `json:"isoPath"`
	VNCEnabled  bool              `json:"vncEnabled"`
	USBDevices  []string          `json:"usbDevices"`
	PCIDevices  []string          `json:"pciDevices"`
	Tags        map[string]string `json:"tags"`
}

// VMStats 虚拟机统计信息
type VMStats struct {
	CPUUsage    float64 `json:"cpuUsage"`    // CPU 使用率 (%)
	MemoryUsage uint64  `json:"memoryUsage"` // 内存使用 (MB)
	DiskRead    uint64  `json:"diskRead"`    // 磁盘读取 (bytes)
	DiskWrite   uint64  `json:"diskWrite"`   // 磁盘写入 (bytes)
	NetRX       uint64  `json:"netRx"`       // 网络接收 (bytes)
	NetTX       uint64  `json:"netTx"`       // 网络发送 (bytes)
}

// ISOImage ISO 镜像信息
type ISOImage struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       uint64    `json:"size"` // 文件大小 (bytes)
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	IsUploaded bool      `json:"isUploaded"` // 是否为用户上传
	URL        string    `json:"url"`        // 下载 URL (如果是内置镜像)
	OS         string    `json:"os"`         // 操作系统类型
}

// VMSnapshot 虚拟机快照
type VMSnapshot struct {
	ID          string    `json:"id"`
	VMID        string    `json:"vmId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	Size        uint64    `json:"size"` // 快照大小 (bytes)
	Status      string    `json:"status"` // creating/ready/restoring
}

// VMTemplate 虚拟机模板
type VMTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        VMType            `json:"type"`
	CPU         int               `json:"cpu"`
	Memory      uint64            `json:"memory"`
	DiskSize    uint64            `json:"diskSize"`
	Network     string            `json:"network"`
	OS          string            `json:"os"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	Tags        map[string]string `json:"tags"`
}

// VNCConnection VNC 连接信息
type VNCConnection struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token"` // noVNC 连接 token
}

// USBDevice USB 设备信息
type USBDevice struct {
	ID          string `json:"id"`
	VendorID    string `json:"vendorId"`
	ProductID   string `json:"productId"`
	Manufacturer string `json:"manufacturer"`
	Product     string `json:"product"`
	InUse       bool   `json:"inUse"` // 是否已被 VM 使用
}

// PCIDevice PCIe 设备信息
type PCIDevice struct {
	ID        string `json:"id"`
	BDF       string `json:"bdf"` // Bus:Device.Function
	VendorID  string `json:"vendorId"`
	DeviceID  string `json:"deviceId"`
	Name      string `json:"name"`
	InUse     bool   `json:"inUse"` // 是否已被 VM 使用
	Driver    string `json:"driver"` // 当前驱动
}
