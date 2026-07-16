// Package lxc 提供增强的 LXC 容器支持模块。
// 对标 TrueNAS 26 的 LXC 容器功能，提供完整的容器生命周期管理、
// 资源限制、网络隔离、存储管理、模板系统、快照备份和高可用支持。
package lxc

import (
	"fmt"
	"time"
)

// ContainerStatus 容器状态.
type ContainerStatus string

const (
	StatusCreated   ContainerStatus = "created"
	StatusStarting  ContainerStatus = "starting"
	StatusRunning   ContainerStatus = "running"
	StatusStopping  ContainerStatus = "stopping"
	StatusStopped   ContainerStatus = "stopped"
	StatusPaused    ContainerStatus = "paused"
	StatusError     ContainerStatus = "error"
	StatusMigrating ContainerStatus = "migrating"
)

// 容器状态转换表.
var containerTransitions = map[ContainerStatus][]ContainerStatus{
	StatusCreated:   {StatusStarting, StatusStopped},
	StatusStarting:  {StatusRunning, StatusError},
	StatusRunning:   {StatusStopping, StatusPaused, StatusMigrating, StatusError},
	StatusStopping:  {StatusStopped, StatusError},
	StatusStopped:   {StatusStarting, StatusCreated},
	StatusPaused:    {StatusRunning, StatusStopping},
	StatusError:     {StatusStarting, StatusStopped},
	StatusMigrating: {StatusRunning, StatusError, StatusStopped},
}

// ValidTransition 检查状态转换是否合法.
func ValidContainerTransition(from, to ContainerStatus) bool {
	targets, ok := containerTransitions[from]
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

// NetworkMode 网络模式.
type NetworkMode string

const (
	NetworkBridge   NetworkMode = "bridge"
	NetworkHost     NetworkMode = "host"
	NetworkNone     NetworkMode = "none"
	NetworkIsolated NetworkMode = "isolated"
)

// RestartPolicy 重启策略.
type RestartPolicy string

const (
	RestartAlways    RestartPolicy = "always"
	RestartOnFailure RestartPolicy = "on-failure"
	RestartNever     RestartPolicy = "never"
)

// ResourceLimit 容器资源限制.
type ResourceLimit struct {
	CPUCores     int    `json:"cpuCores"`     // CPU 核心数
	CPUShares    int    `json:"cpuShares"`    // CPU 份额 (10-1024)
	CPUPercent   int    `json:"cpuPercent"`   // CPU 使用百分比限制 (1-100)
	MemoryMB     uint64 `json:"memoryMB"`     // 内存限制 (MB)
	MemorySwapMB uint64 `json:"memorySwapMB"` // 内存+Swap 限制 (MB)
	DiskGB       uint64 `json:"diskGB"`       // 磁盘配额 (GB)
	ProcessMax   int    `json:"processMax"`   // 最大进程数
	IOWeight     int    `json:"ioWeight"`     // IO 权重 (10-1000)
	BandwidthUp  int    `json:"bandwidthUp"`  // 上行带宽限制 (Mbps)
	BandwidthDn  int    `json:"bandwidthDn"`  // 下行带宽限制 (Mbps)
	OpenFiles    int    `json:"openFiles"`    // 最大打开文件数
}

// Validate 验证资源限制.
func (r ResourceLimit) Validate() error {
	if r.CPUCores < 0 {
		return fmt.Errorf("CPU 核心数不能为负")
	}
	if r.CPUShares < 0 || (r.CPUShares > 0 && r.CPUShares < 10) {
		return fmt.Errorf("CPU 份额必须在 10-1024 之间")
	}
	if r.CPUPercent < 0 || r.CPUPercent > 100 {
		return fmt.Errorf("CPU 百分比必须在 0-100 之间")
	}
	if r.MemoryMB == 0 {
		return fmt.Errorf("内存限制不能为 0")
	}
	if r.ProcessMax < 0 {
		return fmt.Errorf("最大进程数不能为负")
	}
	if r.IOWeight < 0 || r.IOWeight > 1000 {
		return fmt.Errorf("IO 权重必须在 0-1000 之间")
	}
	return nil
}

// NetworkConfig 容器网络配置.
type NetworkConfig struct {
	Mode       NetworkMode `json:"mode"`
	BridgeName string      `json:"bridgeName"` // 桥接网络名
	IPAddress  string      `json:"ipAddress"`  // 静态 IP
	Subnet     string      `json:"subnet"`     // 子网掩码
	Gateway    string      `json:"gateway"`    // 默认网关
	DNS        []string    `json:"dns"`        // DNS 服务器
	MACAddr    string      `json:"macAddr"`    // MAC 地址
	Ports      []PortMap   `json:"ports"`      // 端口映射
	Isolated   bool        `json:"isolated"`   // 网络隔离
}

// PortMap 端口映射.
type PortMap struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"` // tcp, udp
	HostIP        string `json:"hostIP"`
}

// VolumeMount 存储卷挂载.
type VolumeMount struct {
	Source      string `json:"source"`      // 宿主机路径
	Destination string `json:"destination"` // 容器内路径
	ReadOnly    bool   `json:"readOnly"`
	Driver      string `json:"driver"` // local, zfs, nfs
}

// Snapshot 容器快照.
type Snapshot struct {
	ID          string    `json:"id"`
	ContainerID string    `json:"containerId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SizeMB      uint64    `json:"sizeMB"`
	CreatedAt   time.Time `json:"createdAt"`
	ParentID    string    `json:"parentId,omitempty"` // 父快照 ID
}

// HealthCheck 健康检查配置.
type HealthCheck struct {
	Enabled        bool          `json:"enabled"`
	Command        string        `json:"command"`
	Interval       time.Duration `json:"interval"`
	Timeout        time.Duration `json:"timeout"`
	Retries        int           `json:"retries"`
	StartPeriod    time.Duration `json:"startPeriod"`
	LastCheck      *time.Time    `json:"lastCheck,omitempty"`
	LastResult     bool          `json:"lastResult"`
	UnhealthyCount int           `json:"unhealthyCount"`
}

// HAConfig 高可用配置.
type HAConfig struct {
	Enabled         bool          `json:"enabled"`
	FailoverNode    string        `json:"failoverNode"`    // 故障转移目标节点
	AutoRestart     bool          `json:"autoRestart"`     // 自动重启
	RestartDelay    time.Duration `json:"restartDelay"`    // 重启延迟
	MaxRestarts     int           `json:"maxRestarts"`     // 最大重启次数
	CurrentRestarts int           `json:"currentRestarts"` // 当前重启次数
	LastFailover    *time.Time    `json:"lastFailover,omitempty"`
	StandbyState    string        `json:"standbyState"` // active, standby, syncing
}

// Container LXC 容器信息.
type Container struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Template     string            `json:"template"`
	Status       ContainerStatus   `json:"status"`
	Hostname     string            `json:"hostname"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	StartedAt    *time.Time        `json:"startedAt,omitempty"`
	StoppedAt    *time.Time        `json:"stoppedAt,omitempty"`
	Resources    ResourceLimit     `json:"resources"`
	Network      NetworkConfig     `json:"network"`
	Volumes      []VolumeMount     `json:"volumes"`
	Snapshots    []Snapshot        `json:"snapshots"`
	HealthCheck  *HealthCheck      `json:"healthCheck,omitempty"`
	HAConfig     *HAConfig         `json:"haConfig,omitempty"`
	Tags         map[string]string `json:"tags"`
	Error        string            `json:"error,omitempty"`
	PID          int               `json:"pid,omitempty"`
	IPAddress    string            `json:"ipAddress,omitempty"`
	RestartCount int               `json:"restartCount"`
}

// CreateRequest 创建容器请求.
type CreateRequest struct {
	Name        string            `json:"name" binding:"required"`
	Template    string            `json:"template" binding:"required"`
	Hostname    string            `json:"hostname"`
	Resources   ResourceLimit     `json:"resources"`
	Network     NetworkConfig     `json:"network"`
	Volumes     []VolumeMount     `json:"volumes"`
	Tags        map[string]string `json:"tags"`
	HealthCheck *HealthCheck      `json:"healthCheck,omitempty"`
	HAConfig    *HAConfig         `json:"haConfig,omitempty"`
}

// UpdateRequest 更新容器请求.
type UpdateRequest struct {
	Resources   *ResourceLimit    `json:"resources,omitempty"`
	Network     *NetworkConfig    `json:"network,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	HealthCheck *HealthCheck      `json:"healthCheck,omitempty"`
	HAConfig    *HAConfig         `json:"haConfig,omitempty"`
}

// BatchRequest 批量操作请求.
type BatchRequest struct {
	ContainerIDs []string `json:"containerIds" binding:"required"`
}

// Stats 容器资源统计.
type Stats struct {
	CPUPercent  float64   `json:"cpuPercent"`
	CPUShares   int       `json:"cpuShares"`
	MemoryMB    uint64    `json:"memoryMB"`
	MemoryLimit uint64    `json:"memoryLimit"`
	DiskUsedMB  uint64    `json:"diskUsedMB"`
	DiskLimitMB uint64    `json:"diskLimitMB"`
	NetRxBytes  uint64    `json:"netRxBytes"`
	NetTxBytes  uint64    `json:"netTxBytes"`
	PIDs        int       `json:"pids"`
	Timestamp   time.Time `json:"timestamp"`
}

// ContainerListResponse 容器列表响应.
type ContainerListResponse struct {
	Total      int          `json:"total"`
	Running    int          `json:"running"`
	Stopped    int          `json:"stopped"`
	Error      int          `json:"error"`
	Containers []*Container `json:"containers"`
}

// APIResponse 通用 API 响应.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
