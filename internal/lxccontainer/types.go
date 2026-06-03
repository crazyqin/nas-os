package lxccontainer

import "time"

// Status 容器状态.
type Status string

const (
	StatusRunning   Status = "running"
	StatusStopped   Status = "stopped"
	StatusCreating  Status = "creating"
	StatusDeleting  Status = "deleting"
	StatusStarting  Status = "starting"
	StatusStopping  Status = "stopping"
	StatusRebooting Status = "rebooting"
	StatusFailed    Status = "failed"
)

// NetworkMode 网络模式.
type NetworkMode string

const (
	NetworkModeBridge NetworkMode = "bridge"
	NetworkModeNAT    NetworkMode = "nat"
	NetworkModeStatic NetworkMode = "static"
	NetworkModeNone   NetworkMode = "none"
)

// ContainerState 容器状态机.
var ContainerState = map[Status][]Status{
	StatusCreating:  {StatusRunning, StatusFailed},
	StatusRunning:   {StatusStopped, StatusRebooting, StatusDeleting},
	StatusStopped:   {StatusStarting, StatusRunning, StatusDeleting},
	StatusStarting:  {StatusRunning, StatusFailed},
	StatusStopping:  {StatusStopped},
	StatusRebooting: {StatusRunning, StatusFailed},
	StatusDeleting:  {},
	StatusFailed:    {StatusStarting, StatusDeleting},
}

// ValidTransition 检查状态转换是否合法.
func ValidTransition(from, to Status) bool {
	targets, ok := ContainerState[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// ResourceLimit 容器资源限制.
type ResourceLimit struct {
	CPUCores    int    `json:"cpuCores"`    // CPU 核心数
	CPUPercent  int    `json:"cpuPercent"`  // CPU 使用百分比限制 (1-100)
	MemoryMB    uint64 `json:"memoryMB"`    // 内存限制 (MB)
	SwapMB      uint64 `json:"swapMB"`      // Swap 限制 (MB)
	DiskGB      uint64 `json:"diskGB"`      // 磁盘配额 (GB)
	ProcessMax  int    `json:"processMax"`  // 最大进程数
	BandwidthUp int    `json:"bandwidthUp"` // 上行带宽限制 (Mbps)
	BandwidthDn int    `json:"bandwidthDn"` // 下行带宽限制 (Mbps)
}

// NetworkConfig 容器网络配置.
type NetworkConfig struct {
	Mode      NetworkMode `json:"mode"`
	Bridge    string      `json:"bridge"`    // 桥接网络名
	IPAddress string      `json:"ipAddress"` // 静态 IP
	Subnet    string      `json:"subnet"`    // 子网掩码
	Gateway   string      `json:"gateway"`   // 默认网关
	DNS       []string    `json:"dns"`       // DNS 服务器
	MACAddr   string      `json:"macAddr"`   // MAC 地址
	Ports     []PortMap   `json:"ports"`     // 端口映射
}

// PortMap 端口映射.
type PortMap struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol"` // tcp/udp
}

// VolumeMount 存储卷挂载.
type VolumeMount struct {
	Source      string `json:"source"`      // 宿主机路径
	Destination string `json:"destination"` // 容器内路径
	ReadOnly    bool   `json:"readOnly"`
}

// Container LXC 容器信息.
type Container struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Template    string         `json:"template"`
	Status      Status         `json:"status"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	StartedAt   *time.Time     `json:"startedAt,omitempty"`
	Resources   ResourceLimit  `json:"resources"`
	Network     NetworkConfig  `json:"network"`
	Volumes     []VolumeMount  `json:"volumes"`
	Hostname    string         `json:"hostname"`
	Tags        map[string]string `json:"tags"`
}

// CreateRequest 创建容器请求.
type CreateRequest struct {
	Name     string            `json:"name"`
	Template string            `json:"template"`
	Hostname string            `json:"hostname"`
	Resources ResourceLimit    `json:"resources"`
	Network  NetworkConfig     `json:"network"`
	Volumes  []VolumeMount     `json:"volumes"`
	Tags     map[string]string `json:"tags"`
}

// Template 容器模板.
type Template struct {
	Name        string            `json:"name"`
	Distro      string            `json:"distro"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	ImageURL    string            `json:"imageURL"`
	Packages    []string          `json:"packages"`
	SizeMB      int               `json:"sizeMB"`
	Metadata    map[string]string `json:"metadata"`
}

// Stats 容器资源统计.
type Stats struct {
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryMB    uint64  `json:"memoryMB"`
	MemoryLimit uint64  `json:"memoryLimit"`
	DiskUsedMB  uint64  `json:"diskUsedMB"`
	NetRxBytes  uint64  `json:"netRxBytes"`
	NetTxBytes  uint64  `json:"netTxBytes"`
	PIDs        int     `json:"pids"`
}
