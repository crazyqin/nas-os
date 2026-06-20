// Package containermgr 提供容器运行时管理功能，支持 Docker 和 LXC 两种运行时引擎。
// 对标 TrueNAS 26 的 LXC 容器支持，提供统一的容器生命周期管理、镜像管理、
// 网络管理、存储卷管理和资源监控功能。
package containermgr

import "time"

// ========== 容器运行时类型 ==========

// RuntimeType 容器运行时类型.
type RuntimeType string

const (
	// RuntimeDocker Docker 运行时.
	RuntimeDocker RuntimeType = "docker"
	// RuntimeLXC LXC 运行时.
	RuntimeLXC RuntimeType = "lxc"
)

// ========== 容器生命周期状态 ==========

// ContainerState 容器状态.
type ContainerState string

const (
	// StateCreated 容器已创建.
	StateCreated ContainerState = "created"
	// StateRunning 容器运行中.
	StateRunning ContainerState = "running"
	// StatePaused 容器已暂停.
	StatePaused ContainerState = "paused"
	// StateRestarting 容器重启中.
	StateRestarting ContainerState = "restarting"
	// StateRemoving 容器移除中.
	StateRemoving ContainerState = "removing"
	// StateExited 容器已退出.
	StateExited ContainerState = "exited"
	// StateDead 容器异常.
	StateDead ContainerState = "dead"
)

// ========== 容器类型 ==========

// Container 容器信息.
type Container struct {
	ID          string            `json:"id"`                    // 容器唯一标识
	Name        string            `json:"name"`                  // 容器名称
	Image       string            `json:"image"`                 // 镜像名称
	Runtime     RuntimeType       `json:"runtime"`               // 运行时类型
	State       ContainerState    `json:"state"`                 // 容器状态
	Status      string            `json:"status"`                // 状态描述
	CreatedAt   time.Time         `json:"created_at"`            // 创建时间
	StartedAt   *time.Time        `json:"started_at,omitempty"`  // 启动时间
	FinishedAt  *time.Time        `json:"finished_at,omitempty"` // 结束时间
	Ports       []PortMapping     `json:"ports"`                 // 端口映射
	Volumes     []VolumeMount     `json:"volumes"`               // 卷挂载
	Networks    []string          `json:"networks"`              // 所属网络
	Labels      map[string]string `json:"labels"`                // 标签
	Environment map[string]string `json:"environment"`           // 环境变量
	CPUUsage    float64           `json:"cpu_usage"`             // CPU 使用率
	MemUsage    uint64            `json:"mem_usage"`             // 内存使用量（字节）
	MemLimit    uint64            `json:"mem_limit"`             // 内存限制（字节）
	ExitCode    int               `json:"exit_code"`             // 退出码
	Error       string            `json:"error,omitempty"`       // 错误信息
}

// PortMapping 端口映射.
type PortMapping struct {
	HostIP        string `json:"host_ip"`        // 主机 IP
	HostPort      string `json:"host_port"`      // 主机端口
	ContainerPort string `json:"container_port"` // 容器端口
	Protocol      string `json:"protocol"`       // 协议（tcp/udp）
}

// VolumeMount 卷挂载.
type VolumeMount struct {
	Source      string `json:"source"`      // 源路径
	Destination string `json:"destination"` // 目标路径
	Mode        string `json:"mode"`        // 挂载模式（ro/rw）
	RW          bool   `json:"rw"`          // 是否可读写
}

// ========== 容器创建配置 ==========

// ContainerConfig 容器创建配置.
type ContainerConfig struct {
	Name         string            `json:"name" binding:"required"`  // 容器名称
	Image        string            `json:"image" binding:"required"` // 镜像名称
	Runtime      RuntimeType       `json:"runtime"`                  // 运行时类型
	Command      []string          `json:"command,omitempty"`        // 启动命令
	Entrypoint   []string          `json:"entrypoint,omitempty"`     // 入口点
	WorkingDir   string            `json:"working_dir,omitempty"`    // 工作目录
	User         string            `json:"user,omitempty"`           // 运行用户
	Hostname     string            `json:"hostname,omitempty"`       // 主机名
	DomainName   string            `json:"domain_name,omitempty"`    // 域名
	Ports        []PortMapping     `json:"ports,omitempty"`          // 端口映射
	Volumes      []VolumeMount     `json:"volumes,omitempty"`        // 卷挂载
	Environment  map[string]string `json:"environment,omitempty"`    // 环境变量
	Labels       map[string]string `json:"labels,omitempty"`         // 标签
	Networks     []string          `json:"networks,omitempty"`       // 网络列表
	RestartPolicy string           `json:"restart_policy,omitempty"` // 重启策略
	CPULimit     float64           `json:"cpu_limit,omitempty"`      // CPU 限制（核心数）
	MemoryLimit  uint64            `json:"memory_limit,omitempty"`   // 内存限制（字节）
	Privileged   bool              `json:"privileged,omitempty"`     // 特权模式
	CapAdd       []string          `json:"cap_add,omitempty"`        // 添加的能力
	CapDrop      []string          `json:"cap_drop,omitempty"`       // 删除的能力
	Devices      []string          `json:"devices,omitempty"`        // 设备映射
	ExtraHosts   []string          `json:"extra_hosts,omitempty"`    // 额外主机映射
	Tmpfs        []string          `json:"tmpfs,omitempty"`          // tmpfs 挂载
}

// ========== 镜像类型 ==========

// Image 镜像信息.
type Image struct {
	ID         string    `json:"id"`          // 镜像 ID
	Repository string    `json:"repository"`  // 仓库名
	Tag        string    `json:"tag"`         // 标签
	Size       uint64    `json:"size"`        // 大小（字节）
	Created    time.Time `json:"created"`     // 创建时间
	Digest     string    `json:"digest"`      // 内容摘要
	Labels     map[string]string `json:"labels,omitempty"` // 标签
}

// ImageBuildConfig 镜像构建配置.
type ImageBuildConfig struct {
	Dockerfile string            `json:"dockerfile"`          // Dockerfile 路径
	Context    string            `json:"context"`             // 构建上下文
	Tags       []string          `json:"tags"`                // 标签列表
	BuildArgs  map[string]string `json:"build_args,omitempty"` // 构建参数
	NoCache    bool              `json:"no_cache,omitempty"`  // 不使用缓存
	Target     string            `json:"target,omitempty"`    // 多阶段构建目标
	Platform   string            `json:"platform,omitempty"`  // 目标平台
}

// ========== 网络类型 ==========

// Network 网络信息.
type Network struct {
	ID         string            `json:"id"`          // 网络 ID
	Name       string            `json:"name"`        // 网络名称
	Driver     string            `json:"driver"`      // 网络驱动
	Scope      string            `json:"scope"`       // 作用域
	Subnet     string            `json:"subnet"`      // 子网
	Gateway    string            `json:"gateway"`     // 网关
	IPRange    string            `json:"ip_range"`    // IP 范围
	Containers []string          `json:"containers"`  // 关联容器
	Labels     map[string]string `json:"labels"`      // 标签
	Internal   bool              `json:"internal"`    // 是否内部网络
	EnableIPv6 bool              `json:"enable_ipv6"` // 是否启用 IPv6
	Created    time.Time         `json:"created"`     // 创建时间
}

// NetworkCreateConfig 网络创建配置.
type NetworkCreateConfig struct {
	Name       string            `json:"name" binding:"required"` // 网络名称
	Driver     string            `json:"driver"`                  // 网络驱动
	Subnet     string            `json:"subnet,omitempty"`        // 子网
	Gateway    string            `json:"gateway,omitempty"`       // 网关
	IPRange    string            `json:"ip_range,omitempty"`      // IP 范围
	Internal   bool              `json:"internal,omitempty"`      // 是否内部网络
	EnableIPv6 bool              `json:"enable_ipv6,omitempty"`   // 是否启用 IPv6
	Labels     map[string]string `json:"labels,omitempty"`        // 标签
	Options    map[string]string `json:"options,omitempty"`       // 驱动选项
}

// ========== 卷类型 ==========

// Volume 卷信息.
type Volume struct {
	Name       string            `json:"name"`        // 卷名称
	Driver     string            `json:"driver"`      // 卷驱动
	MountPoint string            `json:"mount_point"` // 挂载点
	Size       uint64            `json:"size"`        // 大小（字节）
	Created    time.Time         `json:"created"`     // 创建时间
	Labels     map[string]string `json:"labels"`      // 标签
	Scope      string            `json:"scope"`       // 作用域
	Options    map[string]string `json:"options"`     // 驱动选项
	Status     map[string]string `json:"status"`      // 状态信息
}

// VolumeCreateConfig 卷创建配置.
type VolumeCreateConfig struct {
	Name    string            `json:"name" binding:"required"` // 卷名称
	Driver  string            `json:"driver"`                  // 卷驱动
	Labels  map[string]string `json:"labels,omitempty"`        // 标签
	Options map[string]string `json:"options,omitempty"`       // 驱动选项
}

// ========== 资源监控类型 ==========

// ResourceStats 资源使用统计.
type ResourceStats struct {
	ContainerID string    `json:"container_id"` // 容器 ID
	Timestamp   time.Time `json:"timestamp"`    // 采样时间
	CPU         CPUStats  `json:"cpu"`          // CPU 统计
	Memory      MemStats  `json:"memory"`       // 内存统计
	Network     NetStats  `json:"network"`      // 网络统计
	BlockIO     BlockStats `json:"block_io"`    // 块 I/O 统计
	PIDs        int       `json:"pids"`         // 进程数
}

// CPUStats CPU 使用统计.
type CPUStats struct {
	Usage       float64 `json:"usage"`        // CPU 使用率（百分比）
	SystemUsage uint64  `json:"system_usage"` // 系统 CPU 使用时间
	OnlineCPUs  int     `json:"online_cpus"`  // 在线 CPU 数
	ThrottlePeriods uint64 `json:"throttle_periods"` // 节流周期数
	ThrottledTime   uint64 `json:"throttled_time"`   // 节流时间
}

// MemStats 内存使用统计.
type MemStats struct {
	Usage    uint64  `json:"usage"`     // 内存使用量（字节）
	MaxUsage uint64  `json:"max_usage"` // 最大内存使用量（字节）
	Limit    uint64  `json:"limit"`     // 内存限制（字节）
	Cache    uint64  `json:"cache"`     // 缓存使用量（字节）
	RSS      uint64  `json:"rss"`       // 常驻内存集（字节）
	UsagePct float64 `json:"usage_pct"` // 内存使用率（百分比）
}

// NetStats 网络使用统计.
type NetStats struct {
	RxBytes   uint64 `json:"rx_bytes"`   // 接收字节数
	TxBytes   uint64 `json:"tx_bytes"`   // 发送字节数
	RxPackets uint64 `json:"rx_packets"` // 接收包数
	TxPackets uint64 `json:"tx_packets"` // 发送包数
	RxErrors  uint64 `json:"rx_errors"`  // 接收错误数
	TxErrors  uint64 `json:"tx_errors"`  // 发送错误数
	RxDropped uint64 `json:"rx_dropped"` // 接收丢包数
	TxDropped uint64 `json:"tx_dropped"` // 发送丢包数
}

// BlockStats 块 I/O 使用统计.
type BlockStats struct {
	ReadBytes  uint64 `json:"read_bytes"`  // 读取字节数
	WriteBytes uint64 `json:"write_bytes"` // 写入字节数
	ReadOps    uint64 `json:"read_ops"`    // 读操作数
	WriteOps   uint64 `json:"write_ops"`   // 写操作数
}

// ========== Compose 类型 ==========

// ComposeProject Compose 项目.
type ComposeProject struct {
	Name        string           `json:"name"`        // 项目名称
	ConfigPath  string           `json:"config_path"` // 配置文件路径
	Services    []ComposeService `json:"services"`    // 服务列表
	Networks    []Network        `json:"networks"`    // 网络列表
	Volumes     []Volume         `json:"volumes"`     // 卷列表
	Status      string           `json:"status"`      // 项目状态
	CreatedAt   time.Time        `json:"created_at"`  // 创建时间
	Description string           `json:"description,omitempty"` // 描述
}

// ComposeService Compose 服务.
type ComposeService struct {
	Name        string            `json:"name"`        // 服务名称
	Image       string            `json:"image"`       // 镜像
	Ports       []PortMapping     `json:"ports"`       // 端口映射
	Volumes     []VolumeMount     `json:"volumes"`     // 卷挂载
	Networks    []string          `json:"networks"`    // 网络
	Environment map[string]string `json:"environment"` // 环境变量
	Status      string            `json:"status"`      // 服务状态
	Health      string            `json:"health"`      // 健康状态
	Replicas    int               `json:"replicas"`    // 副本数
}

// ComposeTemplate Compose 模板.
type ComposeTemplate struct {
	Name        string             `json:"name"`        // 模板名称
	Description string             `json:"description"` // 描述
	Category    string             `json:"category"`    // 分类
	Content     string             `json:"content"`     // YAML 内容
	Variables   []TemplateVariable `json:"variables"`   // 变量列表
}

// TemplateVariable 模板变量.
type TemplateVariable struct {
	Name        string `json:"name"`        // 变量名
	Description string `json:"description"` // 描述
	Default     string `json:"default"`     // 默认值
	Required    bool   `json:"required"`    // 是否必需
}

// ComposeUpOptions Compose Up 选项.
type ComposeUpOptions struct {
	Name          string `json:"name"`           // 项目名称
	Build         bool   `json:"build"`          // 是否构建
	RemoveOrphans bool   `json:"remove_orphans"` // 是否移除孤立容器
	ForceRecreate bool   `json:"force_recreate"` // 是否强制重建
	Timeout       int    `json:"timeout"`        // 超时时间（秒）
}

// ComposeDownOptions Compose Down 选项.
type ComposeDownOptions struct {
	RemoveVolumes bool `json:"remove_volumes"` // 是否移除卷
	RemoveImages  bool `json:"remove_images"`  // 是否移除镜像
	Timeout       int  `json:"timeout"`        // 超时时间（秒）
}

// ComposeLogsOptions Compose 日志选项.
type ComposeLogsOptions struct {
	Service    string `json:"service"`    // 服务名
	Tail       int    `json:"tail"`       // 最后 N 行
	Since      string `json:"since"`      // 开始时间
	Until      string `json:"until"`      // 结束时间
	Timestamps bool   `json:"timestamps"` // 是否显示时间戳
	Follow     bool   `json:"follow"`     // 是否跟踪
}

// ========== 系统状态类型 ==========

// SystemStatus 容器系统状态.
type SystemStatus struct {
	DockerRunning bool `json:"docker_running"` // Docker 是否运行
	LXCRunning    bool `json:"lxc_running"`    // LXC 是否运行
	ContainerCount int  `json:"container_count"` // 容器数量
	ImageCount     int  `json:"image_count"`     // 镜像数量
	NetworkCount   int  `json:"network_count"`   // 网络数量
	VolumeCount    int  `json:"volume_count"`    // 卷数量
}

// ========== 请求/响应类型 ==========

// LogOptions 日志选项.
type LogOptions struct {
	Tail       int    `json:"tail"`       // 最后 N 行
	Since      string `json:"since"`      // 开始时间
	Until      string `json:"until"`      // 结束时间
	Timestamps bool   `json:"timestamps"` // 是否显示时间戳
	Follow     bool   `json:"follow"`     // 是否跟踪
}

// ServiceStatus 服务状态.
type ServiceStatus struct {
	Status   string `json:"status"`   // 状态
	Health   string `json:"health"`   // 健康状态
	Replicas int    `json:"replicas"` // 副本数
}
