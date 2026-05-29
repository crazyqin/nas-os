// Package lxcorchestrator 提供 LXC 容器编排引擎，支持多容器依赖管理、资源调度、网络隔离和存储卷管理。
// 基于 TrueNAS 26 的 LXC 支持，提供完整的容器生命周期管理。
package lxcorchestrator

import (
	"time"
)

// ContainerState 容器状态
type ContainerState string

const (
	StateCreated   ContainerState = "created"
	StateStarting  ContainerState = "starting"
	StateRunning   ContainerState = "running"
	StateStopping  ContainerState = "stopping"
	StateStopped   ContainerState = "stopped"
	StateError     ContainerState = "error"
	StateDestroyed ContainerState = "destroyed"
)

// NetworkMode 网络模式
type NetworkMode string

const (
	NetworkBridge   NetworkMode = "bridge"
	NetworkHost     NetworkMode = "host"
	NetworkNone     NetworkMode = "none"
	NetworkCustom   NetworkMode = "custom"
)

// RestartPolicy 重启策略
type RestartPolicy string

const (
	RestartAlways    RestartPolicy = "always"
	RestartOnFailure RestartPolicy = "on-failure"
	RestartNever     RestartPolicy = "never"
	RestartUnlessStopped RestartPolicy = "unless-stopped"
)

// ResourceLimits 资源限制
type ResourceLimits struct {
	CPUShares    int    `json:"cpu_shares,omitempty"`    // CPU 份额 (10-1024)
	CPUQuota     int    `json:"cpu_quota,omitempty"`     // CPU 配额 (微秒)
	MemoryLimit  int64  `json:"memory_limit,omitempty"`  // 内存限制 (字节)
	MemorySwap   int64  `json:"memory_swap,omitempty"`   // 内存+交换区限制
	IOWeight     int    `json:"io_weight,omitempty"`     // IO 权重 (10-1000)
	ProcLimit    int    `json:"proc_limit,omitempty"`    // 进程数限制
	OpenFiles    int    `json:"open_files,omitempty"`    // 最大打开文件数
}

// ContainerConfig 容器配置
type ContainerConfig struct {
	ID            string            `json:"id"`
	Name          string            `json:"name" binding:"required"`
	Image         string            `json:"image" binding:"required"`
	Hostname      string            `json:"hostname,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Entrypoint    []string          `json:"entrypoint,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	NetworkMode   NetworkMode       `json:"network_mode,omitempty"`
	Ports         []PortMapping     `json:"ports,omitempty"`
	Volumes       []VolumeMount     `json:"volumes,omitempty"`
	Resources     ResourceLimits    `json:"resources,omitempty"`
	RestartPolicy RestartPolicy     `json:"restart_policy,omitempty"`
	Privileged    bool              `json:"privileged,omitempty"`
	Capabilities  []string          `json:"capabilities,omitempty"`
	Dependencies  []string          `json:"dependencies,omitempty"` // 依赖的容器 ID 列表
	Tags          []string          `json:"tags,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port" binding:"required"`
	ContainerPort int    `json:"container_port" binding:"required"`
	Protocol      string `json:"protocol,omitempty"` // tcp, udp
	HostIP        string `json:"host_ip,omitempty"`
}

// VolumeMount 存储卷挂载
type VolumeMount struct {
	VolumeName   string `json:"volume_name" binding:"required"`
	MountPath    string `json:"mount_path" binding:"required"`
	ReadOnly     bool   `json:"read_only,omitempty"`
	Propagation  string `json:"propagation,omitempty"` // shared, slave, private, rshared, rslave, rprivate
}

// ContainerInstance 容器实例
type ContainerInstance struct {
	Config      ContainerConfig `json:"config"`
	State       ContainerState  `json:"state"`
	PID         int             `json:"pid,omitempty"`
	IPAddress   string          `json:"ip_address,omitempty"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	StoppedAt   *time.Time      `json:"stopped_at,omitempty"`
	Error       string          `json:"error,omitempty"`
	RestartCount int            `json:"restart_count"`
}

// ContainerTemplate 容器模板
type ContainerTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description,omitempty"`
	Category    string            `json:"category,omitempty"`
	Image       string            `json:"image" binding:"required"`
	Config      ContainerConfig   `json:"config"`
	Variables   []TemplateVariable `json:"variables,omitempty"`
	IsBuiltin   bool              `json:"is_builtin,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TemplateVariable 模板变量
type TemplateVariable struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Type        string `json:"type,omitempty"` // string, int, bool
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name" binding:"required"`
	Mode        NetworkMode       `json:"mode" binding:"required"`
	Subnet      string            `json:"subnet,omitempty"`      // CIDR 格式: 192.168.1.0/24
	Gateway     string            `json:"gateway,omitempty"`
	DNS         []string          `json:"dns,omitempty"`
	BridgeName  string            `json:"bridge_name,omitempty"`
	Isolated    bool              `json:"isolated,omitempty"`    // 网络隔离
	Containers  []string          `json:"containers,omitempty"`  // 容器 ID 列表
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// VolumeConfig 存储卷配置
type VolumeConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name" binding:"required"`
	Driver      string            `json:"driver,omitempty"`      // local, nfs, cifs, zfs
	MountPoint  string            `json:"mount_point,omitempty"`
	Size        int64             `json:"size,omitempty"`        // 字节
	Options     map[string]string `json:"options,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Containers  []string          `json:"containers,omitempty"`  // 使用此卷的容器 ID
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// OrchestratorConfig 编排器配置
type OrchestratorConfig struct {
	Enabled         bool              `json:"enabled"`
	MaxContainers   int               `json:"max_containers"`
	DefaultNetwork  string            `json:"default_network"`
	DefaultVolume   string            `json:"default_volume"`
	LogLevel        string            `json:"log_level"`
	AutoRestart     bool              `json:"auto_restart"`
	HealthCheck     bool              `json:"health_check"`
	HealthInterval  int               `json:"health_interval"` // 秒
	Labels          map[string]string `json:"labels,omitempty"`
}

// DeployRequest 部署请求
type DeployRequest struct {
	TemplateID  string            `json:"template_id,omitempty"`
	Name        string            `json:"name" binding:"required"`
	Image       string            `json:"image,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	Count       int               `json:"count,omitempty"` // 批量部署数量
	NetworkID   string            `json:"network_id,omitempty"`
	VolumeIDs   []string          `json:"volume_ids,omitempty"`
	Resources   ResourceLimits    `json:"resources,omitempty"`
	RestartPolicy RestartPolicy   `json:"restart_policy,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Dependencies []string         `json:"dependencies,omitempty"`
}

// ScaleRequest 扩缩容请求
type ScaleRequest struct {
	ContainerID string `json:"container_id" binding:"required"`
	Count       int    `json:"count" binding:"required,min=0"`
}

// ContainerStats 容器统计信息
type ContainerStats struct {
	ContainerID   string    `json:"container_id"`
	CPUUsage      float64   `json:"cpu_usage"`       // 百分比
	MemoryUsage   int64     `json:"memory_usage"`     // 字节
	MemoryLimit   int64     `json:"memory_limit"`
	NetworkRx     int64     `json:"network_rx"`       // 字节
	NetworkTx     int64     `json:"network_tx"`
	DiskRead      int64     `json:"disk_read"`
	DiskWrite     int64     `json:"disk_write"`
	PIDs          int       `json:"pids"`
	Timestamp     time.Time `json:"timestamp"`
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	ContainerID string    `json:"container_id"`
	Healthy     bool      `json:"healthy"`
	Message     string    `json:"message,omitempty"`
	Latency     int64     `json:"latency"` // 毫秒
	Timestamp   time.Time `json:"timestamp"`
}

// OrchestrationStatus 编排状态
type OrchestrationStatus struct {
	TotalContainers   int                    `json:"total_containers"`
	RunningContainers int                    `json:"running_containers"`
	StoppedContainers int                    `json:"stopped_containers"`
	ErrorContainers   int                    `json:"error_containers"`
	TotalNetworks     int                    `json:"total_networks"`
	TotalVolumes      int                    `json:"total_volumes"`
	Resources         ResourceUsage          `json:"resources"`
	Containers        []ContainerInstance    `json:"containers,omitempty"`
	Networks          []NetworkConfig        `json:"networks,omitempty"`
	Volumes           []VolumeConfig         `json:"volumes,omitempty"`
}

// ResourceUsage 资源使用情况
type ResourceUsage struct {
	TotalCPU     float64 `json:"total_cpu"`      // 百分比
	TotalMemory  int64   `json:"total_memory"`   // 字节
	TotalDisk    int64   `json:"total_disk"`
	AvailableCPU float64 `json:"available_cpu"`
	AvailableMemory int64 `json:"available_memory"`
}

// DefaultOrchestratorConfig 默认编排器配置
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		Enabled:        true,
		MaxContainers:  100,
		DefaultNetwork: "lxc-bridge",
		DefaultVolume:  "lxc-data",
		LogLevel:       "info",
		AutoRestart:    true,
		HealthCheck:    true,
		HealthInterval: 30,
		Labels: map[string]string{
			"managed-by": "lxcorchestrator",
		},
	}
}
