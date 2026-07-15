// Package containerpro 提供容器管理功能
package containerpro

import (
	"time"
)

// Container 容器信息.
type Container struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Status       string            `json:"status"`
	State        string            `json:"state"`
	CreatedAt    time.Time         `json:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	Ports        []PortMapping     `json:"ports"`
	Volumes      []VolumeMount     `json:"volumes"`
	Networks     []string          `json:"networks"`
	CPUUsage     float64           `json:"cpu_usage"`
	MemoryUsage  int64             `json:"memory_usage"`
	MemoryLimit  int64             `json:"memory_limit"`
	NetworkIO    NetworkIO         `json:"network_io"`
	RestartCount int               `json:"restart_count"`
	Labels       map[string]string `json:"labels"`
	Environment  []string          `json:"environment"`
}

// PortMapping 端口映射.
type PortMapping struct {
	HostIP        string `json:"host_ip"`
	HostPort      string `json:"host_port"`
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// VolumeMount 卷挂载.
type VolumeMount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}

// NetworkIO 网络 IO 统计.
type NetworkIO struct {
	RxBytes   int64 `json:"rx_bytes"`
	TxBytes   int64 `json:"tx_bytes"`
	RxPackets int64 `json:"rx_packets"`
	TxPackets int64 `json:"tx_packets"`
}

// ComposeProject Compose 项目.
type ComposeProject struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Path      string           `json:"path"`
	Services  []ComposeService `json:"services"`
	Status    string           `json:"status"`
	Networks  []string         `json:"networks"`
	Volumes   []string         `json:"volumes"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// ComposeService Compose 服务定义.
type ComposeService struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Replicas    int               `json:"replicas"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMount     `json:"volumes"`
	Environment map[string]string `json:"environment"`
	DependsOn   []string          `json:"depends_on"`
	HealthCheck *HealthCheck      `json:"health_check,omitempty"`
	Resources   *Resources        `json:"resources,omitempty"`
}

// HealthCheck 健康检查配置.
type HealthCheck struct {
	Test        []string `json:"test"`
	Interval    string   `json:"interval"`
	Timeout     string   `json:"timeout"`
	Retries     int      `json:"retries"`
	StartPeriod string   `json:"start_period"`
}

// Resources 资源限制.
type Resources struct {
	CPU    float64 `json:"cpu"`
	Memory string  `json:"memory"`
}

// ImageInfo 镜像信息.
type ImageInfo struct {
	ID          string            `json:"id"`
	RepoTags    []string          `json:"repo_tags"`
	Size        int64             `json:"size"`
	CreatedAt   time.Time         `json:"created_at"`
	Labels      map[string]string `json:"labels"`
	VirtualSize int64             `json:"virtual_size"`
	SharedSize  int64             `json:"shared_size"`
}

// Registry 镜像仓库.
type Registry struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	URL      string    `json:"url"`
	Type     string    `json:"type"` // DockerHub, Harbor, ACR, GitHub
	Username string    `json:"username"`
	Status   string    `json:"status"`
	LastSync time.Time `json:"last_sync"`
}

// ContainerStats 容器统计信息.
type ContainerStats struct {
	CPU     CPUStats     `json:"cpu"`
	Memory  MemoryStats  `json:"memory"`
	Network NetworkIO    `json:"network"`
	BlockIO BlockIOStats `json:"block_io"`
	PIDs    int          `json:"pids"`
}

// CPUStats CPU 统计.
type CPUStats struct {
	Usage       float64 `json:"usage"`
	SystemUsage int64   `json:"system_usage"`
	OnlineCPUs  int     `json:"online_cpus"`
}

// MemoryStats 内存统计.
type MemoryStats struct {
	Usage    int64 `json:"usage"`
	MaxUsage int64 `json:"max_usage"`
	Limit    int64 `json:"limit"`
}

// BlockIOStats 块 IO 统计.
type BlockIOStats struct {
	ReadBytes  int64 `json:"read_bytes"`
	WriteBytes int64 `json:"write_bytes"`
	ReadOps    int64 `json:"read_ops"`
	WriteOps   int64 `json:"write_ops"`
}

// ContainerProConfig 容器专业管理配置.
type ContainerProConfig struct {
	DockerHost      string   `json:"docker_host"`
	ComposePath     string   `json:"compose_path"`
	RegistryMirrors []string `json:"registry_mirrors"`
	DefaultRegistry string   `json:"default_registry"`
	AutoRestart     bool     `json:"auto_restart"`
	LogMaxSize      string   `json:"log_max_size"`
}
