// Package sandbox 提供安全沙箱隔离环境管理功能
package sandbox

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrSandboxNotFound 沙箱不存在.
	ErrSandboxNotFound = errors.New("沙箱不存在")
	// ErrSandboxAlreadyExists 沙箱已存在.
	ErrSandboxAlreadyExists = errors.New("沙箱已存在")
	// ErrSandboxRunning 沙箱正在运行.
	ErrSandboxRunning = errors.New("沙箱正在运行")
	// ErrSandboxStopped 沙箱已停止.
	ErrSandboxStopped = errors.New("沙箱已停止")
	// ErrInvalidResourceLimit 无效的资源限制.
	ErrInvalidResourceLimit = errors.New("无效的资源限制配置")
	// ErrSnapshotNotFound 快照不存在.
	ErrSnapshotNotFound = errors.New("快照不存在")
	// ErrSnapshotAlreadyExists 快照已存在.
	ErrSnapshotAlreadyExists = errors.New("快照已存在")
	// ErrIsolationFailed 隔离设置失败.
	ErrIsolationFailed = errors.New("隔离设置失败")
	// ErrResourceExceeded 资源超限.
	ErrResourceExceeded = errors.New("资源使用超出限制")
	// ErrInvalidConfig 无效配置.
	ErrInvalidConfig = errors.New("无效的沙箱配置")
)

// ========== 沙箱状态 ==========

// SandboxStatus 沙箱状态.
type SandboxStatus string

const (
	// SandboxStatusCreated 已创建.
	SandboxStatusCreated SandboxStatus = "created"
	// SandboxStatusRunning 运行中.
	SandboxStatusRunning SandboxStatus = "running"
	// SandboxStatusPaused 已暂停.
	SandboxStatusPaused SandboxStatus = "paused"
	// SandboxStatusStopped 已停止.
	SandboxStatusStopped SandboxStatus = "stopped"
	// SandboxStatusError 错误状态.
	SandboxStatusError SandboxStatus = "error"
)

// ========== 隔离级别 ==========

// IsolationLevel 隔离级别.
type IsolationLevel string

const (
	// IsolationLevelBasic 基础隔离（进程和文件系统）.
	IsolationLevelBasic IsolationLevel = "basic"
	// IsolationLevelStandard 标准隔离（增加网络隔离）.
	IsolationLevelStandard IsolationLevel = "standard"
	// IsolationLevelStrict 严格隔离（完整资源限制）.
	IsolationLevelStrict IsolationLevel = "strict"
	// IsolationLevelMaximum 最大隔离（完全隔离，包括内核命名空间）.
	IsolationLevelMaximum IsolationLevel = "maximum"
)

// ========== 资源限制 ==========

// ResourceLimit 资源限制配置.
type ResourceLimit struct {
	// CPUCores CPU核心数限制.
	CPUCores float64 `json:"cpu_cores"`
	// CPUShares CPU份额（相对权重）.
	CPUShares int `json:"cpu_shares"`
	// MemoryMB 内存限制（MB）.
	MemoryMB int `json:"memory_mb"`
	// MemorySwapMB 交换内存限制（MB，-1表示不限制）.
	MemorySwapMB int `json:"memory_swap_mb"`
	// DiskIOMBps 磁盘IO限制（MB/s）.
	DiskIOMBps float64 `json:"disk_io_mbps"`
	// DiskIOPS 磁盘IOPS限制.
	DiskIOPS int `json:"disk_iops"`
	// NetworkBandwidthMbps 网络带宽限制（Mbps）.
	NetworkBandwidthMbps float64 `json:"network_bandwidth_mbps"`
	// PIDsLimit 进程数限制.
	PIDsLimit int `json:"pids_limit"`
	// OpenFilesLimit 打开文件数限制.
	OpenFilesLimit int `json:"open_files_limit"`
}

// DefaultResourceLimit 返回默认资源限制.
func DefaultResourceLimit() *ResourceLimit {
	return &ResourceLimit{
		CPUCores:              1.0,
		CPUShares:             1024,
		MemoryMB:              512,
		MemorySwapMB:          -1,
		DiskIOMBps:            100,
		DiskIOPS:              1000,
		NetworkBandwidthMbps:  100,
		PIDsLimit:             256,
		OpenFilesLimit:        1024,
	}
}

// ========== 网络隔离 ==========

// NetworkIsolation 网络隔离配置.
type NetworkIsolation struct {
	// Enabled 是否启用网络隔离.
	Enabled bool `json:"enabled"`
	// Mode 网络模式: none, bridge, host, custom.
	Mode NetworkMode `json:"mode"`
	// BridgeName 桥接网络名称.
	BridgeName string `json:"bridge_name,omitempty"`
	// Subnet 子网 CIDR.
	Subnet string `json:"subnet,omitempty"`
	// Gateway 网关地址.
	Gateway string `json:"gateway,omitempty"`
	// DNS DNS服务器列表.
	DNS []string `json:"dns,omitempty"`
	// AllowedPorts 允许的端口列表.
	AllowedPorts []PortRange `json:"allowed_ports,omitempty"`
	// BlockedPorts 阻止的端口列表.
	BlockedPorts []PortRange `json:"blocked_ports,omitempty"`
	// AllowOutbound 是否允许出站连接.
	AllowOutbound bool `json:"allow_outbound"`
	// AllowInbound 是否允许入站连接.
	AllowInbound bool `json:"allow_inbound"`
}

// NetworkMode 网络模式.
type NetworkMode string

const (
	// NetworkModeNone 无网络.
	NetworkModeNone NetworkMode = "none"
	// NetworkModeBridge 桥接模式.
	NetworkModeBridge NetworkMode = "bridge"
	// NetworkModeHost 主机模式.
	NetworkModeHost NetworkMode = "host"
	// NetworkModeCustom 自定义网络.
	NetworkModeCustom NetworkMode = "custom"
)

// PortRange 端口范围.
type PortRange struct {
	// Start 起始端口.
	Start int `json:"start"`
	// End 结束端口.
	End int `json:"end"`
	// Protocol 协议: tcp, udp.
	Protocol string `json:"protocol"`
}

// ========== 文件系统隔离 ==========

// FilesystemIsolation 文件系统隔离配置.
type FilesystemIsolation struct {
	// RootFS 根文件系统路径.
	RootFS string `json:"rootfs"`
	// ReadOnly 是否只读.
	ReadOnly bool `json:"read_only"`
	// MountPoints 挂载点列表.
	MountPoints []MountPoint `json:"mount_points,omitempty"`
	// TmpFSSizeMB 临时文件系统大小（MB）.
	TmpFSSizeMB int `json:"tmpfs_size_mb"`
	// AllowedPaths 允许访问的主机路径.
	AllowedPaths []string `json:"allowed_paths,omitempty"`
	// DeniedPaths 禁止访问的主机路径.
	DeniedPaths []string `json:"denied_paths,omitempty"`
}

// MountPoint 挂载点.
type MountPoint struct {
	// Source 源路径.
	Source string `json:"source"`
	// Target 目标路径.
	Target string `json:"target"`
	// ReadOnly 是否只读.
	ReadOnly bool `json:"read_only"`
	// Type 类型: bind, tmpfs, volume.
	Type string `json:"type"`
}

// ========== 沙箱配置 ==========

// SandboxConfig 沙箱配置.
type SandboxConfig struct {
	// Name 沙箱名称.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description,omitempty"`
	// IsolationLevel 隔离级别.
	IsolationLevel IsolationLevel `json:"isolation_level"`
	// ResourceLimit 资源限制.
	ResourceLimit *ResourceLimit `json:"resource_limit"`
	// Network 网络隔离配置.
	Network *NetworkIsolation `json:"network"`
	// Filesystem 文件系统隔离配置.
	Filesystem *FilesystemIsolation `json:"filesystem"`
	// Labels 标签.
	Labels map[string]string `json:"labels,omitempty"`
	// AutoStart 是否自动启动.
	AutoStart bool `json:"auto_start"`
	// MaxLifetime 最大生存时间（秒，0表示不限制）.
	MaxLifetime int `json:"max_lifetime"`
}

// DefaultSandboxConfig 返回默认沙箱配置.
func DefaultSandboxConfig(name string) *SandboxConfig {
	return &SandboxConfig{
		Name:           name,
		IsolationLevel: IsolationLevelStandard,
		ResourceLimit:  DefaultResourceLimit(),
		Network: &NetworkIsolation{
			Enabled:       true,
			Mode:          NetworkModeBridge,
			AllowOutbound: true,
			AllowInbound:  false,
		},
		Filesystem: &FilesystemIsolation{
			ReadOnly:     false,
			TmpFSSizeMB:  100,
		},
		AutoStart:   false,
		MaxLifetime: 0,
	}
}

// ========== 沙箱实例 ==========

// Sandbox 沙箱实例.
type Sandbox struct {
	// ID 沙箱唯一标识.
	ID string `json:"id"`
	// Config 配置.
	Config *SandboxConfig `json:"config"`
	// Status 状态.
	Status SandboxStatus `json:"status"`
	// PID 进程ID（运行时）.
	PID int `json:"pid,omitempty"`
	// IPAddress IP地址（运行时）.
	IPAddress string `json:"ip_address,omitempty"`
	// RootPath 沙箱根目录.
	RootPath string `json:"root_path"`
	// ResourceUsage 资源使用情况.
	ResourceUsage *ResourceUsage `json:"resource_usage,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// StartedAt 启动时间.
	StartedAt *time.Time `json:"started_at,omitempty"`
	// StoppedAt 停止时间.
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
	// Error 错误信息.
	Error string `json:"error,omitempty"`
	// SnapshotCount 快照数量.
	SnapshotCount int `json:"snapshot_count"`
}

// ResourceUsage 资源使用情况.
type ResourceUsage struct {
	// CPUUsage CPU使用率（百分比）.
	CPUUsage float64 `json:"cpu_usage"`
	// MemoryUsageMB 内存使用（MB）.
	MemoryUsageMB int `json:"memory_usage_mb"`
	// MemoryUsagePercent 内存使用率（百分比）.
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	// DiskReadBytes 磁盘读取字节数.
	DiskReadBytes int64 `json:"disk_read_bytes"`
	// DiskWriteBytes 磁盘写入字节数.
	DiskWriteBytes int64 `json:"disk_write_bytes"`
	// NetworkRxBytes 网络接收字节数.
	NetworkRxBytes int64 `json:"network_rx_bytes"`
	// NetworkTxBytes 网络发送字节数.
	NetworkTxBytes int64 `json:"network_tx_bytes"`
	// PIDCount 当前进程数.
	PIDCount int `json:"pid_count"`
	// Timestamp 数据采集时间.
	Timestamp time.Time `json:"timestamp"`
}

// ========== 快照 ==========

// Snapshot 沙箱快照.
type Snapshot struct {
	// ID 快照唯一标识.
	ID string `json:"id"`
	// SandboxID 所属沙箱ID.
	SandboxID string `json:"sandbox_id"`
	// Name 快照名称.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description,omitempty"`
	// SizeBytes 快照大小（字节）.
	SizeBytes int64 `json:"size_bytes"`
	// Type 快照类型.
	Type SnapshotType `json:"type"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// ParentID 父快照ID（增量快照）.
	ParentID string `json:"parent_id,omitempty"`
	// Labels 标签.
	Labels map[string]string `json:"labels,omitempty"`
}

// SnapshotType 快照类型.
type SnapshotType string

const (
	// SnapshotTypeFull 全量快照.
	SnapshotTypeFull SnapshotType = "full"
	// SnapshotTypeIncremental 增量快照.
	SnapshotTypeIncremental SnapshotType = "incremental"
)

// ========== API请求/响应类型 ==========

// CreateSandboxRequest 创建沙箱请求.
type CreateSandboxRequest struct {
	// Config 沙箱配置.
	Config *SandboxConfig `json:"config"`
	// FromSnapshot 从快照恢复（可选）.
	FromSnapshot string `json:"from_snapshot,omitempty"`
}

// UpdateSandboxRequest 更新沙箱请求.
type UpdateSandboxRequest struct {
	// Description 描述.
	Description string `json:"description,omitempty"`
	// ResourceLimit 资源限制.
	ResourceLimit *ResourceLimit `json:"resource_limit,omitempty"`
	// Labels 标签.
	Labels map[string]string `json:"labels,omitempty"`
	// AutoStart 是否自动启动.
	AutoStart *bool `json:"auto_start,omitempty"`
}

// CreateSnapshotRequest 创建快照请求.
type CreateSnapshotRequest struct {
	// Name 快照名称.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description,omitempty"`
	// Type 快照类型.
	Type SnapshotType `json:"type"`
	// Labels 标签.
	Labels map[string]string `json:"labels,omitempty"`
}

// SandboxListResponse 沙箱列表响应.
type SandboxListResponse struct {
	// Total 总数.
	Total int `json:"total"`
	// Items 沙箱列表.
	Items []*Sandbox `json:"items"`
}

// SnapshotListResponse 快照列表响应.
type SnapshotListResponse struct {
	// Total 总数.
	Total int `json:"total"`
	// Items 快照列表.
	Items []*Snapshot `json:"items"`
}

// SandboxStats 沙箱统计信息.
type SandboxStats struct {
	// TotalSandbox 总沙箱数.
	TotalSandbox int `json:"total_sandbox"`
	// RunningSandbox 运行中沙箱数.
	RunningSandbox int `json:"running_sandbox"`
	// StoppedSandbox 已停止沙箱数.
	StoppedSandbox int `json:"stopped_sandbox"`
	// TotalSnapshots 总快照数.
	TotalSnapshots int `json:"total_snapshots"`
	// TotalSnapshotSizeBytes 总快照大小.
	TotalSnapshotSizeBytes int64 `json:"total_snapshot_size_bytes"`
	// ResourceUsage 总资源使用.
	ResourceUsage *ResourceUsage `json:"resource_usage"`
}
