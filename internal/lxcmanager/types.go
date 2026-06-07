// Package lxcmanager 提供 LXC 容器管理功能
// 参考 TrueNAS 25.10 的 LXC Sandbox 功能
// 支持容器模板管理、资源限制、网络隔离、快照备份
package lxcmanager

import (
	"time"
)

// Status 容器状态.
type Status string

const (
	// StatusRunning 运行中.
	StatusRunning Status = "running"
	// StatusStopped 已停止.
	StatusStopped Status = "stopped"
	// StatusFrozen 已冻结.
	StatusFrozen Status = "frozen"
	// StatusCreating 创建中.
	StatusCreating Status = "creating"
	// StatusDeleting 删除中.
	StatusDeleting Status = "deleting"
	// StatusSnapshot 快照中.
	StatusSnapshot Status = "snapshotting"
	// StatusRestoring 恢复中.
	StatusRestoring Status = "restoring"
	// StatusCloning 克隆中.
	StatusCloning Status = "cloning"
)

// NetworkMode 网络模式.
type NetworkMode string

const (
	// NetworkBridge 桥接模式.
	NetworkBridge NetworkMode = "bridge"
	// NetworkNAT NAT 模式.
	NetworkNAT NetworkMode = "nat"
	// NetworkHost 主机模式.
	NetworkHost NetworkMode = "host"
	// NetworkNone 无网络.
	NetworkNone NetworkMode = "none"
)

// ContainerType 容器类型.
type ContainerType string

const (
	// ContainerTypeApp 应用容器.
	ContainerTypeApp ContainerType = "app"
	// ContainerTypeService 服务容器.
	ContainerTypeService ContainerType = "service"
	// ContainerTypeSandbox 沙箱容器（参考 TrueNAS Sandbox）.
	ContainerTypeSandbox ContainerType = "sandbox"
)

// LXC 容器信息.
type LXC struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Hostname    string        `json:"hostname"`
	Status      Status        `json:"status"`
	Type        ContainerType `json:"type"`
	TemplateID  string        `json:"templateId"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	StartedAt   *time.Time    `json:"startedAt,omitempty"`

	// 资源限制
	ResourceLimit ResourceLimit `json:"resourceLimit"`

	// 网络配置
	Network NetworkConfig `json:"network"`

	// 存储配置
	RootFSPath string `json:"rootFsPath"`
	RootFSSize uint64 `json:"rootFsSize"` // GB

	// 安全配置
	Privileged bool     `json:"privileged"`
	CapDrop    []string `json:"capDrop,omitempty"`
	CapAdd     []string `json:"capAdd,omitempty"`
	AppArmor   string   `json:"appArmor,omitempty"`
	Seccomp    string   `json:"seccomp,omitempty"`

	// 挂载点
	MountPoints []MountPoint `json:"mountPoints,omitempty"`

	// 标签
	Tags map[string]string `json:"tags,omitempty"`
}

// ResourceLimit 容器资源限制.
type ResourceLimit struct {
	CPUShares    int    `json:"cpuShares"`    // CPU 权重 (10-1024)
	CPUCores     int    `json:"cpuCores"`     // CPU 核心数限制
	CPUPercent   int    `json:"cpuPercent"`   // CPU 使用率限制 (%)
	MemoryLimit  uint64 `json:"memoryLimit"`  // 内存限制 (MB)
	MemorySwap   uint64 `json:"memorySwap"`   // 内存+Swap 限制 (MB)
	DiskIOLimit  uint64 `json:"diskIoLimit"`  // 磁盘 IO 限制 (bytes/s)
	NetBandwidth uint64 `json:"netBandwidth"` // 网络带宽限制 (bytes/s)
	ProcLimit    int    `json:"procLimit"`    // 进程数限制
	FDLimit      int    `json:"fdLimit"`      // 文件描述符限制
}

// NetworkConfig 容器网络配置.
type NetworkConfig struct {
	Mode       NetworkMode `json:"mode"`
	Bridge     string      `json:"bridge,omitempty"`
	Interface  string      `json:"interface,omitempty"`
	IPAddress  string      `json:"ipAddress,omitempty"`
	Gateway    string      `json:"gateway,omitempty"`
	DNS        []string    `json:"dns,omitempty"`
	MACAddress string      `json:"macAddress,omitempty"`
	VLAN       int         `json:"vlan,omitempty"`
	MTU        int         `json:"mtu,omitempty"`
	Firewall   bool        `json:"firewall"`
	PortMap    []PortMap   `json:"portMap,omitempty"`
}

// PortMap 端口映射.
type PortMap struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"` // tcp/udp
}

// MountPoint 挂载点.
type MountPoint struct {
	HostPath      string `json:"hostPath"`
	ContainerPath string `json:"containerPath"`
	ReadOnly      bool   `json:"readOnly"`
	Type          string `json:"type"` // bind/volume/tmpfs
}

// Stats 容器统计信息.
type Stats struct {
	CPUUsage    float64 `json:"cpuUsage"`    // CPU 使用率 (%)
	MemoryUsage uint64  `json:"memoryUsage"` // 内存使用 (MB)
	MemoryLimit uint64  `json:"memoryLimit"` // 内存限制 (MB)
	MemoryPct   float64 `json:"memoryPct"`   // 内存使用率 (%)
	NetRX       uint64  `json:"netRx"`       // 网络接收 (bytes)
	NetTX       uint64  `json:"netTx"`       // 网络发送 (bytes)
	BlockRead   uint64  `json:"blockRead"`   // 块设备读取 (bytes)
	BlockWrite  uint64  `json:"blockWrite"`  // 块设备写入 (bytes)
	PIDs        int     `json:"pids"`        // 进程数
	Uptime      int64   `json:"uptime"`      // 运行时间 (秒)
}

// Template 容器模板.
type Template struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Distro        string            `json:"distro"`    // ubuntu, debian, alpine, centos 等
	Version       string            `json:"version"`   // 22.04, 12, 3.18 等
	Arch          string            `json:"arch"`      // amd64, arm64
	ImageURL      string            `json:"imageUrl"`  // 模板下载 URL
	ImageHash     string            `json:"imageHash"` // 镜像校验和
	Type          ContainerType     `json:"type"`
	DefaultConfig ResourceLimit     `json:"defaultConfig"`
	MinResources  ResourceLimit     `json:"minResources"`
	MaxResources  ResourceLimit     `json:"maxResources"`
	PreInstalled  []string          `json:"preInstalled,omitempty"` // 预装软件
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
	BuiltIn       bool              `json:"builtIn"` // 是否内置模板
	Tags          map[string]string `json:"tags,omitempty"`
}

// Snapshot 容器快照.
type Snapshot struct {
	ID          string    `json:"id"`
	ContainerID string    `json:"containerId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Stateful    bool      `json:"stateful"` // 是否保存运行状态
	CreatedAt   time.Time `json:"createdAt"`
	Size        uint64    `json:"size"`   // 快照大小 (bytes)
	Status      string    `json:"status"` // creating/ready/restoring
}

// Config 创建容器请求配置.
type Config struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Hostname      string            `json:"hostname"`
	Type          ContainerType     `json:"type"`
	TemplateID    string            `json:"templateId"`
	Privileged    bool              `json:"privileged"`
	ResourceLimit ResourceLimit     `json:"resourceLimit"`
	Network       NetworkConfig     `json:"network"`
	RootFSSize    uint64            `json:"rootFsSize"`
	MountPoints   []MountPoint      `json:"mountPoints,omitempty"`
	CapDrop       []string          `json:"capDrop,omitempty"`
	CapAdd        []string          `json:"capAdd,omitempty"`
	AppArmor      string            `json:"appArmor,omitempty"`
	Seccomp       string            `json:"seccomp,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// UpdateConfig 更新容器配置.
type UpdateConfig struct {
	Description   string            `json:"description,omitempty"`
	Hostname      string            `json:"hostname,omitempty"`
	ResourceLimit *ResourceLimit    `json:"resourceLimit,omitempty"`
	Network       *NetworkConfig    `json:"network,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// CloneConfig 克隆容器配置.
type CloneConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SnapshotID  string `json:"snapshotId,omitempty"` // 从快照克隆
}

// DefaultResourceLimit 返回默认资源限制.
func DefaultResourceLimit() ResourceLimit {
	return ResourceLimit{
		CPUShares:    1024,
		CPUCores:     0, // 不限制
		CPUPercent:   0, // 不限制
		MemoryLimit:  512,
		MemorySwap:   1024,
		DiskIOLimit:  0, // 不限制
		NetBandwidth: 0, // 不限制
		ProcLimit:    256,
		FDLimit:      1024,
	}
}

// DefaultNetworkConfig 返回默认网络配置.
func DefaultNetworkConfig() NetworkConfig {
	return NetworkConfig{
		Mode:     NetworkBridge,
		Bridge:   "lxcbr0",
		Firewall: true,
		MTU:      1500,
		DNS:      []string{"8.8.8.8", "8.8.4.4"},
	}
}
