// Package containerorch 提供容器编排功能，对标 TrueNAS 应用编排能力
package containerorch

import (
	"time"
)

// ========== 容器编排核心类型 ==========

// OrchestrationProject 编排项目
type OrchestrationProject struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description,omitempty"`
	Namespace   string                    `json:"namespace"`
	Services    map[string]*ServiceConfig `json:"services"`
	Networks    map[string]*NetworkConfig `json:"networks,omitempty"`
	Volumes     map[string]*VolumeConfig  `json:"volumes,omitempty"`
	Status      ProjectStatus             `json:"status"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	Labels      map[string]string         `json:"labels,omitempty"`
}

// ProjectStatus 项目状态
type ProjectStatus string

const (
	ProjectStatusCreating  ProjectStatus = "creating"
	ProjectStatusRunning   ProjectStatus = "running"
	ProjectStatusStopped   ProjectStatus = "stopped"
	ProjectStatusUpdating  ProjectStatus = "updating"
	ProjectStatusError     ProjectStatus = "error"
	ProjectStatusPartial   ProjectStatus = "partial"
	ProjectStatusScaling   ProjectStatus = "scaling"
	ProjectStatusHealing   ProjectStatus = "healing"
)

// ServiceConfig 服务配置
type ServiceConfig struct {
	Name          string                    `json:"name"`
	Image         string                    `json:"image"`
	Tag           string                    `json:"tag,omitempty"`
	Command       []string                  `json:"command,omitempty"`
	Entrypoint    []string                  `json:"entrypoint,omitempty"`
	Environment   map[string]string         `json:"environment,omitempty"`
	Ports         []PortMapping             `json:"ports,omitempty"`
	Volumes       []VolumeMount             `json:"volumes,omitempty"`
	Networks      []string                  `json:"networks,omitempty"`
	DependsOn     []ServiceDependency       `json:"depends_on,omitempty"`
	HealthCheck   *HealthCheckConfig        `json:"health_check,omitempty"`
	Resources     *ResourceLimits           `json:"resources,omitempty"`
	Deploy        *DeployConfig             `json:"deploy,omitempty"`
	RestartPolicy RestartPolicy             `json:"restart_policy,omitempty"`
	Labels        map[string]string         `json:"labels,omitempty"`
	Privileged    bool                      `json:"privileged,omitempty"`
	CapAdd        []string                  `json:"cap_add,omitempty"`
	CapDrop       []string                  `json:"cap_drop,omitempty"`
	Status        ServiceStatus             `json:"status"`
	ContainerIDs  []string                  `json:"container_ids,omitempty"`
	Instances     int                       `json:"instances"`
	DesiredCount  int                       `json:"desired_count"`
}

// ServiceStatus 服务状态
type ServiceStatus string

const (
	ServiceStatusPending   ServiceStatus = "pending"
	ServiceStatusCreating  ServiceStatus = "creating"
	ServiceStatusRunning   ServiceStatus = "running"
	ServiceStatusStopped   ServiceStatus = "stopped"
	ServiceStatusError     ServiceStatus = "error"
	ServiceStatusScaling   ServiceStatus = "scaling"
	ServiceStatusUnhealthy ServiceStatus = "unhealthy"
	ServiceStatusHealing   ServiceStatus = "healing"
)

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol,omitempty"` // tcp, udp
	HostIP        string `json:"host_ip,omitempty"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	ReadOnly    bool   `json:"read_only,omitempty"`
	Type        string `json:"type,omitempty"` // bind, volume, tmpfs
	BindOptions *BindOptions `json:"bind_options,omitempty"`
}

// BindOptions 绑定挂载选项
type BindOptions struct {
	Propagation string `json:"propagation,omitempty"` // rshared, shared, slave, private
}

// ServiceDependency 服务依赖
type ServiceDependency struct {
	ServiceName string        `json:"service_name"`
	Condition   string        `json:"condition"` // service_started, service_healthy, service_completed_successfully
	Restart     bool          `json:"restart,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Test        []string      `json:"test"`
	Interval    time.Duration `json:"interval"`
	Timeout     time.Duration `json:"timeout"`
	Retries     int           `json:"retries"`
	StartPeriod time.Duration `json:"start_period"`
	Disable     bool          `json:"disable,omitempty"`
}

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusStarting  HealthStatus = "starting"
	HealthStatusNone      HealthStatus = "none"
)

// ResourceLimits 资源限制
type ResourceLimits struct {
	CPU    *CPULimit    `json:"cpu,omitempty"`
	Memory *MemoryLimit `json:"memory,omitempty"`
	IO     *IOLimit     `json:"io,omitempty"`
	PidsLimit *int       `json:"pids_limit,omitempty"`
}

// CPULimit CPU 限制
type CPULimit struct {
	Cores       float64 `json:"cores"`       // CPU 核数
	Shares      int     `json:"shares"`       // CPU shares (相对权重)
	Quota       int     `json:"quota"`        // CPU CFS quota (微秒)
	Period      int     `json:"period"`       // CPU CFS period (微秒)
	CpusetCpus  string  `json:"cpuset_cpus"`  // 绑定到特定 CPU 核
	RealtimePeriod  int `json:"realtime_period,omitempty"`
	RealtimeRuntime int `json:"realtime_runtime,omitempty"`
}

// MemoryLimit 内存限制
type MemoryLimit struct {
	Limit       int64  `json:"limit"`       // 内存限制 (字节)
	Reservation int64  `json:"reservation"` // 内存预留 (字节)
	Swap        int64  `json:"swap"`        // Swap 限制 (字节)
	Kernel      int64  `json:"kernel"`      // 内核内存限制 (字节)
	Swappiness  int    `json:"swappiness"`  // Swappiness (0-100)
}

// IOLimit IO 限制
type IOLimit struct {
	DeviceReadBps   int64 `json:"device_read_bps"`   // 读取带宽限制 (字节/秒)
	DeviceWriteBps  int64 `json:"device_write_bps"`  // 写入带宽限制 (字节/秒)
	DeviceReadIOps  int64 `json:"device_read_iops"`  // 读取 IOPS 限制
	DeviceWriteIOps int64 `json:"device_write_iops"` // 写入 IOPS 限制
}

// DeployConfig 部署配置
type DeployConfig struct {
	Replicas      int                  `json:"replicas"`
	AutoScale     *AutoScalePolicy     `json:"auto_scale,omitempty"`
	UpdateConfig  *UpdateConfig        `json:"update_config,omitempty"`
	RollbackConfig *RollbackConfig     `json:"rollback_config,omitempty"`
	Placement     []PlacementConstraint `json:"placement,omitempty"`
	Labels        map[string]string    `json:"labels,omitempty"`
}

// AutoScalePolicy 自动扩缩容策略
type AutoScalePolicy struct {
	Enabled     bool              `json:"enabled"`
	MinReplicas int               `json:"min_replicas"`
	MaxReplicas int               `json:"max_replicas"`
	Metrics     []ScalingMetric   `json:"metrics"`
	Cooldown    time.Duration     `json:"cooldown"`
	ScaleUp     *ScaleRules       `json:"scale_up,omitempty"`
	ScaleDown   *ScaleRules       `json:"scale_down,omitempty"`
}

// ScalingMetric 扩缩容指标
type ScalingMetric struct {
	Type     string  `json:"type"`     // cpu, memory, requests_per_second, custom
	Target   float64 `json:"target"`   // 目标值
	Current  float64 `json:"current"`  // 当前值
	Window   time.Duration `json:"window"` // 采样窗口
}

// ScaleRules 扩缩容规则
type ScaleRules struct {
	StepSize      int           `json:"step_size"`      // 每次扩缩容数量
	StepPercent   int           `json:"step_percent"`   // 每次扩缩容百分比
	Stabilization time.Duration `json:"stabilization"`  // 稳定窗口
}

// UpdateConfig 更新配置
type UpdateConfig struct {
	Parallelism   int           `json:"parallelism"`
	Delay         time.Duration `json:"delay"`
	FailureAction string        `json:"failure_action"` // pause, continue, rollback
	Order         string        `json:"order"`          // stop-first, start-first
	Monitor       time.Duration `json:"monitor"`
	MaxFailureRatio float64     `json:"max_failure_ratio"`
}

// RollbackConfig 回滚配置
type RollbackConfig struct {
	Parallelism   int           `json:"parallelism"`
	Delay         time.Duration `json:"delay"`
	FailureAction string        `json:"failure_action"`
	Order         string        `json:"order"`
	Monitor       time.Duration `json:"monitor"`
	MaxFailureRatio float64     `json:"max_failure_ratio"`
}

// PlacementConstraint 放置约束
type PlacementConstraint struct {
	Type       string `json:"type"`       // node.role, node.platform.os, etc.
	Operator   string `json:"operator"`   // ==, !=
	Value      string `json:"value"`
}

// RestartPolicy 重启策略
type RestartPolicy string

const (
	RestartPolicyAlways    RestartPolicy = "always"
	RestartPolicyOnFailure RestartPolicy = "on-failure"
	RestartPolicyUnlessStop RestartPolicy = "unless-stopped"
	RestartPolicyNo        RestartPolicy = "no"
)

// NetworkConfig 网络配置
type NetworkConfig struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"` // bridge, host, overlay, macvlan
	Internal   bool              `json:"internal,omitempty"`
	Attachable bool              `json:"attachable,omitempty"`
	IPAM       *IPAMConfig       `json:"ipam,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
}

// IPAMConfig IPAM 配置
type IPAMConfig struct {
	Driver string       `json:"driver,omitempty"`
	Config []IPAMPool   `json:"config,omitempty"`
}

// IPAMPool IPAM 地址池
type IPAMPool struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway,omitempty"`
	IPRange string `json:"ip_range,omitempty"`
}

// VolumeConfig 卷配置
type VolumeConfig struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"` // local, nfs, cifs
	DriverOpts map[string]string `json:"driver_opts,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	External   bool              `json:"external,omitempty"`
}

// ========== 日志聚合类型 ==========

// LogEntry 日志条目
type LogEntry struct {
	Timestamp   time.Time         `json:"timestamp"`
	Service     string            `json:"service"`
	ContainerID string            `json:"container_id"`
	Stream      string            `json:"stream"` // stdout, stderr
	Message     string            `json:"message"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// LogQuery 日志查询参数
type LogQuery struct {
	Services    []string  `json:"services,omitempty"`
	Since       time.Time `json:"since,omitempty"`
	Until       time.Time `json:"until,omitempty"`
	Stream      string    `json:"stream,omitempty"` // stdout, stderr, all
	Tail        int       `json:"tail,omitempty"`
	Follow      bool      `json:"follow,omitempty"`
	Timestamps  bool      `json:"timestamps,omitempty"`
	Pattern     string    `json:"pattern,omitempty"`
	Limit       int       `json:"limit,omitempty"`
}

// LogStream 日志流
type LogStream struct {
	Entries chan LogEntry `json:"-"`
	Error   error        `json:"-"`
	Close   func()       `json:"-"`
}

// ========== 依赖排序类型 ==========

// DependencyGraph 依赖图
type DependencyGraph struct {
	Nodes map[string]*DependencyNode
	Edges map[string][]string // node -> dependencies
}

// DependencyNode 依赖节点
type DependencyNode struct {
	Name         string
	Service      *ServiceConfig
	Dependencies []string
	Dependents   []string
	Visited      bool
	InStack      bool
}

// StartupOrder 启动顺序
type StartupOrder struct {
	Stages [][]string `json:"stages"` // 每个阶段可以并行启动的服务
	Total  int        `json:"total"`
}

// ========== 健康监控类型 ==========

// HealthReport 健康报告
type HealthReport struct {
	ProjectID   string                    `json:"project_id"`
	Timestamp   time.Time                 `json:"timestamp"`
	Services    map[string]*ServiceHealth  `json:"services"`
	Overall     HealthStatus               `json:"overall"`
	Uptime      time.Duration              `json:"uptime"`
	Issues      []HealthIssue              `json:"issues,omitempty"`
}

// ServiceHealth 服务健康状态
type ServiceHealth struct {
	ServiceName string                   `json:"service_name"`
	Status      HealthStatus             `json:"status"`
	Instances   []InstanceHealth         `json:"instances"`
	LastCheck   time.Time                `json:"last_check"`
	Uptime      time.Duration            `json:"uptime"`
	Restarts    int                      `json:"restarts"`
	Issues      []HealthIssue            `json:"issues,omitempty"`
}

// InstanceHealth 实例健康状态
type InstanceHealth struct {
	ContainerID string       `json:"container_id"`
	Status      HealthStatus `json:"status"`
	CPU         float64      `json:"cpu_percent"`
	Memory      int64        `json:"memory_bytes"`
	Uptime      time.Duration `json:"uptime"`
	Restarts    int          `json:"restarts"`
}

// HealthIssue 健康问题
type HealthIssue struct {
	Service     string    `json:"service"`
	ContainerID string    `json:"container_id,omitempty"`
	Type        string    `json:"type"` // unhealthy, high_cpu, high_memory, restart_loop, oom
	Severity    string    `json:"severity"` // warning, critical
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Resolved    bool      `json:"resolved"`
}

// ========== 拓扑排序辅助类型 ==========

// TopologicalSorter 拓扑排序器
type TopologicalSorter struct {
	graph    *DependencyGraph
	result   []string
	visited  map[string]bool
	recStack map[string]bool
	hasCycle bool
	cyclePath []string
}

// ========== API 请求/响应类型 ==========

// CreateProjectRequest 创建项目请求
type CreateProjectRequest struct {
	Name        string                    `json:"name" binding:"required"`
	Description string                    `json:"description,omitempty"`
	Namespace   string                    `json:"namespace"`
	Services    map[string]*ServiceConfig `json:"services"`
	Networks    map[string]*NetworkConfig `json:"networks,omitempty"`
	Volumes     map[string]*VolumeConfig  `json:"volumes,omitempty"`
	Labels      map[string]string         `json:"labels,omitempty"`
}

// UpdateProjectRequest 更新项目请求
type UpdateProjectRequest struct {
	Name        *string                    `json:"name,omitempty"`
	Description *string                    `json:"description,omitempty"`
	Services    map[string]*ServiceConfig  `json:"services,omitempty"`
	Networks    map[string]*NetworkConfig  `json:"networks,omitempty"`
	Volumes     map[string]*VolumeConfig   `json:"volumes,omitempty"`
	Labels      map[string]string          `json:"labels,omitempty"`
}

// ScaleServiceRequest 扩缩容请求
type ScaleServiceRequest struct {
	Replicas int  `json:"replicas" binding:"required,min=0"`
	Force    bool `json:"force,omitempty"`
}

// UpdateHealthCheckRequest 更新健康检查请求
type UpdateHealthCheckRequest struct {
	ServiceName string             `json:"service_name" binding:"required"`
	HealthCheck *HealthCheckConfig `json:"health_check" binding:"required"`
}

// UpdateResourceLimitsRequest 更新资源限制请求
type UpdateResourceLimitsRequest struct {
	ServiceName string          `json:"service_name" binding:"required"`
	Resources   *ResourceLimits `json:"resources" binding:"required"`
}

// UpdateAutoScaleRequest 更新自动扩缩容请求
type UpdateAutoScaleRequest struct {
	ServiceName string          `json:"service_name" binding:"required"`
	AutoScale   *AutoScalePolicy `json:"auto_scale" binding:"required"`
}

// ProjectStats 项目统计
type ProjectStats struct {
	ProjectID      string        `json:"project_id"`
	TotalServices  int           `json:"total_services"`
	RunningServices int          `json:"running_services"`
	TotalInstances int           `json:"total_instances"`
	RunningInstances int         `json:"running_instances"`
	TotalCPU       float64       `json:"total_cpu_percent"`
	TotalMemory    int64         `json:"total_memory_bytes"`
	Uptime         time.Duration `json:"uptime"`
}

// AutoScaleEvent 扩缩容事件
type AutoScaleEvent struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	ServiceName string    `json:"service_name"`
	Action      string    `json:"action"` // scale_up, scale_down
	From        int       `json:"from"`
	To          int       `json:"to"`
	Reason      string    `json:"reason"`
	Timestamp   time.Time `json:"timestamp"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
}

// ContainerMetrics 容器指标
type ContainerMetrics struct {
	ContainerID string    `json:"container_id"`
	ServiceName string    `json:"service_name"`
	Timestamp   time.Time `json:"timestamp"`
	CPU         CPUMetrics `json:"cpu"`
	Memory      MemoryMetrics `json:"memory"`
	Network     NetworkMetrics `json:"network"`
	IO          IOMetrics  `json:"io"`
}

// CPUMetrics CPU 指标
type CPUMetrics struct {
	Percent float64 `json:"percent"`
	Usage   int64   `json:"usage"`
	System  int64   `json:"system"`
	ThrottlePeriods int `json:"throttle_periods"`
	ThrottleTime    int `json:"throttle_time"`
}

// MemoryMetrics 内存指标
type MemoryMetrics struct {
	Usage     int64   `json:"usage"`
	MaxUsage  int64   `json:"max_usage"`
	Limit     int64   `json:"limit"`
	Percent   float64 `json:"percent"`
	Cache     int64   `json:"cache"`
	RSS       int64   `json:"rss"`
	Swap      int64   `json:"swap"`
}

// NetworkMetrics 网络指标
type NetworkMetrics struct {
	RxBytes   int64 `json:"rx_bytes"`
	TxBytes   int64 `json:"tx_bytes"`
	RxPackets int64 `json:"rx_packets"`
	TxPackets int64 `json:"tx_packets"`
	RxErrors  int64 `json:"rx_errors"`
	TxErrors  int64 `json:"tx_errors"`
	RxDropped int64 `json:"rx_dropped"`
	TxDropped int64 `json:"tx_dropped"`
}

// IOMetrics IO 指标
type IOMetrics struct {
	ReadBytes  int64 `json:"read_bytes"`
	WriteBytes int64 `json:"write_bytes"`
	ReadOps    int64 `json:"read_ops"`
	WriteOps   int64 `json:"write_ops"`
}

// ========== 容器编排增强类型 ==========

// ComposeStack Docker Compose 栈
type ComposeStack struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ProjectName string            `json:"project_name"`
	ComposeFile string            `json:"compose_file"`     // YAML 内容
	Path        string            `json:"path"`             // 文件路径
	Services    []ComposeService  `json:"services"`
	Status      StackStatus       `json:"status"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// StackStatus 栈状态
type StackStatus string

const (
	StackStatusRunning  StackStatus = "running"
	StackStatusStopped  StackStatus = "stopped"
	StackStatusDeploying StackStatus = "deploying"
	StackStatusError    StackStatus = "error"
	StackStatusPartial  StackStatus = "partial"
)

// ComposeService Compose 服务
type ComposeService struct {
	Name      string   `json:"name"`
	Image     string   `json:"image"`
	Status    string   `json:"status"`
	Ports     []string `json:"ports,omitempty"`
	Replicas  int      `json:"replicas"`
}

// ContainerHealth 容器健康状态
type ContainerHealth struct {
	ContainerID   string        `json:"container_id"`
	ServiceName   string        `json:"service_name"`
	StackID       string        `json:"stack_id"`
	Status        HealthStatus  `json:"status"`
	ChecksPassed  int           `json:"checks_passed"`
	ChecksFailed  int           `json:"checks_failed"`
	LastCheck     time.Time     `json:"last_check"`
	Uptime        time.Duration `json:"uptime"`
	RestartCount  int           `json:"restart_count"`
	CPU           float64       `json:"cpu_percent"`
	Memory        int64         `json:"memory_bytes"`
}

// AutoScaleRule 自动扩缩容规则
type AutoScaleRule struct {
	ID          string        `json:"id"`
	StackID     string        `json:"stack_id"`
	ServiceName string        `json:"service_name"`
	Enabled     bool          `json:"enabled"`
	MinReplicas int           `json:"min_replicas"`
	MaxReplicas int           `json:"max_replicas"`
	MetricType  string        `json:"metric_type"`  // cpu, memory, requests
	TargetValue float64       `json:"target_value"`
	Cooldown    time.Duration `json:"cooldown"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// ImageCache 镜像缓存
type ImageCache struct {
	ImageName   string    `json:"image_name"`
	Tag         string    `json:"tag"`
	Digest      string    `json:"digest"`
	Size        int64     `json:"size_bytes"`
	LastUsed    time.Time `json:"last_used"`
	PullCount   int       `json:"pull_count"`
	Cached      bool      `json:"cached"`
}

// RecoveryPolicy 容器恢复策略
type RecoveryPolicy struct {
	ID               string        `json:"id"`
	StackID          string        `json:"stack_id"`
	ServiceName      string        `json:"service_name"`
	Enabled          bool          `json:"enabled"`
	RestartOnFailure bool          `json:"restart_on_failure"`
	MaxRetries       int           `json:"max_retries"`
	RetryInterval    time.Duration `json:"retry_interval"`
	AutoRemove       bool          `json:"auto_remove"`
	HealthCheck      bool          `json:"health_check"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// DeployStackRequest 部署栈请求
type DeployStackRequest struct {
	Name        string            `json:"name" binding:"required"`
	ComposeFile string            `json:"compose_file" binding:"required"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Pull        bool              `json:"pull"`
}

// SetAutoScaleRequest 设置自动扩缩容请求
type SetAutoScaleRequest struct {
	ServiceName string  `json:"service_name" binding:"required"`
	Enabled     bool    `json:"enabled"`
	MinReplicas int     `json:"min_replicas"`
	MaxReplicas int     `json:"max_replicas"`
	MetricType  string  `json:"metric_type"`
	TargetValue float64 `json:"target_value"`
}

// CacheImageRequest 缓存镜像请求
type CacheImageRequest struct {
	Image string `json:"image" binding:"required"`
	Tag   string `json:"tag"`
}

// SetRecoveryRequest 设置恢复策略请求
type SetRecoveryRequest struct {
	ServiceName      string `json:"service_name" binding:"required"`
	Enabled          bool   `json:"enabled"`
	RestartOnFailure bool   `json:"restart_on_failure"`
	MaxRetries       int    `json:"max_retries"`
	HealthCheck      bool   `json:"health_check"`
}
