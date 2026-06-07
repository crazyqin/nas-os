package edgecompute

import "time"

// WorkloadType 工作负载类型
type WorkloadType string

const (
	WorkloadContainer WorkloadType = "container"
	WorkloadFunction  WorkloadType = "function"
	WorkloadAI        WorkloadType = "ai_inference"
	WorkloadStream    WorkloadType = "stream_processing"
	WorkloadBatch     WorkloadType = "batch"
	WorkloadDaemon    WorkloadType = "daemon"
	WorkloadCron      WorkloadType = "cron"
)

// WorkloadStatus 工作负载状态
type WorkloadStatus string

const (
	StatusPending   WorkloadStatus = "pending"
	StatusDeploying WorkloadStatus = "deploying"
	StatusRunning   WorkloadStatus = "running"
	StatusStopping  WorkloadStatus = "stopping"
	StatusStopped   WorkloadStatus = "stopped"
	StatusFailed    WorkloadStatus = "failed"
	StatusScaling   WorkloadStatus = "scaling"
	StatusUpdating  WorkloadStatus = "updating"
)

// FunctionRuntime 函数运行时
type FunctionRuntime string

const (
	RuntimePython    FunctionRuntime = "python"
	RuntimeNode      FunctionRuntime = "nodejs"
	RuntimeGo        FunctionRuntime = "go"
	RuntimeRust      FunctionRuntime = "rust"
	RuntimeJava      FunctionRuntime = "java"
	RuntimeDotNet    FunctionRuntime = "dotnet"
	RuntimeWasm      FunctionRuntime = "wasm"
	RuntimeCustom    FunctionRuntime = "custom"
	RuntimeContainer FunctionRuntime = "container"
)

// AIModelType AI 模型类型
type AIModelType string

const (
	ModelLLM       AIModelType = "llm"
	ModelVision    AIModelType = "vision"
	ModelSpeech    AIModelType = "speech"
	ModelEmbedding AIModelType = "embedding"
	ModelDiffusion AIModelType = "diffusion"
	ModelCustom    AIModelType = "custom"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeReady    NodeStatus = "ready"
	NodeNotReady NodeStatus = "not_ready"
	NodeDraining NodeStatus = "draining"
	NodeOffline  NodeStatus = "offline"
	NodeUnknown  NodeStatus = "unknown"
)

// EdgeNode 边缘节点
type EdgeNode struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Location      string            `json:"location"`
	IPAddress     string            `json:"ip_address"`
	MACAddress    string            `json:"mac_address"`
	Status        NodeStatus        `json:"status"`
	CPUCores      int               `json:"cpu_cores"`
	MemoryMB      int               `json:"memory_mb"`
	DiskGB        int               `json:"disk_gb"`
	GPUCount      int               `json:"gpu_count"`
	GPUModel      string            `json:"gpu_model,omitempty"`
	Architecture  string            `json:"architecture"`
	OS            string            `json:"os"`
	Labels        map[string]string `json:"labels"`
	Taints        []string          `json:"taints,omitempty"`
	Resources     NodeResources     `json:"resources"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// NodeResources 节点资源
type NodeResources struct {
	CPUUsed       float64 `json:"cpu_used"`
	CPUTotal      float64 `json:"cpu_total"`
	MemoryUsedMB  int     `json:"memory_used_mb"`
	MemoryTotalMB int     `json:"memory_total_mb"`
	DiskUsedGB    int     `json:"disk_used_gb"`
	DiskTotalGB   int     `json:"disk_total_gb"`
	GPUUsed       float64 `json:"gpu_used"`
	GPUTotal      float64 `json:"gpu_total"`
	PodCount      int     `json:"pod_count"`
	MaxPods       int     `json:"max_pods"`
}

// Workload 工作负载
type Workload struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Type          WorkloadType      `json:"type"`
	Status        WorkloadStatus    `json:"status"`
	Priority      int               `json:"priority"`
	NodeID        string            `json:"node_id"`
	Image         string            `json:"image,omitempty"`
	Version       string            `json:"version"`
	Replicas      int               `json:"replicas"`
	ReadyReplicas int               `json:"ready_replicas"`
	CPUCores      float64           `json:"cpu_cores"`
	MemoryMB      int               `json:"memory_mb"`
	GPUCount      int               `json:"gpu_count"`
	Ports         []PortMapping     `json:"ports,omitempty"`
	EnvVars       map[string]string `json:"env_vars,omitempty"`
	Volumes       []VolumeMount     `json:"volumes,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	HealthCheck   *HealthCheck      `json:"health_check,omitempty"`
	RestartPolicy string            `json:"restart_policy"`
	MaxRetries    int               `json:"max_retries"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
	Triggers      []Trigger         `json:"triggers,omitempty"`
	Resources     ResourceRequest   `json:"resources"`
	NodeSelector  map[string]string `json:"node_selector,omitempty"`
	Tolerations   []Toleration      `json:"tolerations,omitempty"`
	Affinity      *Affinity         `json:"affinity,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	LastDeployed  *time.Time        `json:"last_deployed,omitempty"`
	DeploymentID  string            `json:"deployment_id"`
}

// PortMapping 端口映射
type PortMapping struct {
	Name          string `json:"name"`
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port,omitempty"`
	Protocol      string `json:"protocol"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	ReadOnly  bool   `json:"read_only"`
	HostPath  string `json:"host_path,omitempty"`
	Type      string `json:"type"`
}

// HealthCheck 健康检查
type HealthCheck struct {
	Type                string `json:"type"`
	Path                string `json:"path,omitempty"`
	Port                int    `json:"port,omitempty"`
	Command             string `json:"command,omitempty"`
	IntervalSeconds     int    `json:"interval_seconds"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	FailureThreshold    int    `json:"failure_threshold"`
	SuccessThreshold    int    `json:"success_threshold"`
	InitialDelaySeconds int    `json:"initial_delay_seconds"`
}

// Trigger 触发器
type Trigger struct {
	Type       string            `json:"type"`
	Cron       string            `json:"cron,omitempty"`
	Webhook    string            `json:"webhook,omitempty"`
	Event      string            `json:"event,omitempty"`
	Conditions map[string]string `json:"conditions,omitempty"`
}

// ResourceRequest 资源请求
type ResourceRequest struct {
	CPUCores    float64 `json:"cpu_cores"`
	CPU         float64 `json:"cpu"`
	MemoryMB    int     `json:"memory_mb"`
	Memory      int     `json:"memory"`
	GPUCount    int     `json:"gpu_count"`
	StorageGB   int     `json:"storage_gb"`
	BandwidthMB int     `json:"bandwidth_mb"`
}

// Toleration 容忍
type Toleration struct {
	Key      string `json:"key"`
	Operator string `json:"operator"`
	Value    string `json:"value,omitempty"`
	Effect   string `json:"effect"`
}

// Affinity 亲和性
type Affinity struct {
	NodeAffinity    *NodeAffinity    `json:"node_affinity,omitempty"`
	PodAffinity     *PodAffinity     `json:"pod_affinity,omitempty"`
	PodAntiAffinity *PodAntiAffinity `json:"pod_anti_affinity,omitempty"`
}

// NodeAffinity 节点亲和性
type NodeAffinity struct {
	Required  []NodeSelector  `json:"required,omitempty"`
	Preferred []PreferredTerm `json:"preferred,omitempty"`
}

// PodAffinity Pod 亲和性
type PodAffinity struct {
	Required  []PodSelectorTerm `json:"required,omitempty"`
	Preferred []WeightedPodTerm `json:"preferred,omitempty"`
}

// PodAntiAffinity Pod 反亲和性
type PodAntiAffinity struct {
	Required  []PodSelectorTerm `json:"required,omitempty"`
	Preferred []WeightedPodTerm `json:"preferred,omitempty"`
}

// NodeSelector 节点选择器
type NodeSelector struct {
	MatchLabels      map[string]string `json:"match_labels,omitempty"`
	MatchExpressions []SelectorExpr    `json:"match_expressions,omitempty"`
}

// SelectorExpr 选择器表达式
type SelectorExpr struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// PreferredTerm 首选项
type PreferredTerm struct {
	Weight     int          `json:"weight"`
	Preference NodeSelector `json:"preference"`
}

// PodSelectorTerm Pod 选择器项
type PodSelectorTerm struct {
	MatchLabels      map[string]string `json:"match_labels,omitempty"`
	MatchExpressions []SelectorExpr    `json:"match_expressions,omitempty"`
}

// WeightedPodTerm 加权 Pod 项
type WeightedPodTerm struct {
	Weight     int             `json:"weight"`
	Preference PodSelectorTerm `json:"preference"`
}

// FunctionState 函数状态
// FunctionState 函数状态
type FunctionState string

const (
	StateActive    FunctionState = "active"
	StateInactive  FunctionState = "inactive"
	StateError     FunctionState = "error"
	StateDeploying FunctionState = "deploying"
)

// FunctionConfig 函数配置
type FunctionConfig struct {
	Timeout    int `json:"timeout"`
	Memory     int `json:"memory"`
	MaxRetries int `json:"max_retries"`
}

// Config 边缘计算配置
type Config struct {
	Enabled        bool `json:"enabled"`
	MaxFunctions   int  `json:"max_functions"`
	MaxWorkloads   int  `json:"max_workloads"`
	DefaultTimeout int  `json:"default_timeout"`
	WasmEnabled    bool `json:"wasm_enabled"`
	GPUEnabled     bool `json:"gpu_enabled"`
	AutoScaling    bool `json:"auto_scaling"`
}

// Function 函数计算
type Function struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Runtime     FunctionRuntime   `json:"runtime"`
	Handler     string            `json:"handler"`
	Code        string            `json:"code"`
	CodePath    string            `json:"code_path"`
	State       FunctionState     `json:"state"`
	Config      FunctionConfig    `json:"config"`
	MemoryMB    int               `json:"memory_mb"`
	TimeoutSec  int               `json:"timeout_sec"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Trigger     Trigger           `json:"trigger"`
	Status      string            `json:"status"`
	InvokeCount int64             `json:"invoke_count"`
	LastError   string            `json:"last_error,omitempty"`
	LastInvoked *time.Time        `json:"last_invoked,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// AIInferenceTask AI 推理任务
type AIInferenceTask struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	ModelType   AIModelType `json:"model_type"`
	ModelName   string      `json:"model_name"`
	ModelPath   string      `json:"model_path"`
	GPURequired bool        `json:"gpu_required"`
	GPUCount    int         `json:"gpu_count"`
	MaxBatch    int         `json:"max_batch"`
	MaxTokens   int         `json:"max_tokens"`
	Status      string      `json:"status"`
	NodeID      string      `json:"node_id"`
	InvokeCount int64       `json:"invoke_count"`
	AvgLatency  float64     `json:"avg_latency"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// DeploymentRecord 部署记录
type DeploymentRecord struct {
	ID           string     `json:"id"`
	WorkloadID   string     `json:"workload_id"`
	WorkloadName string     `json:"workload_name"`
	Version      string     `json:"version"`
	Status       string     `json:"status"`
	NodeID       string     `json:"node_id"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Duration     int64      `json:"duration"`
	Message      string     `json:"message,omitempty"`
	RollbackID   string     `json:"rollback_id,omitempty"`
}

// EdgeCluster 边缘集群
type EdgeCluster struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Nodes       []EdgeNode    `json:"nodes"`
	Workloads   []Workload    `json:"workloads"`
	Status      string        `json:"status"`
	Version     string        `json:"version"`
	Network     NetworkConfig `json:"network"`
	Storage     StorageConfig `json:"storage"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	PodCIDR     string `json:"pod_cidr"`
	ServiceCIDR string `json:"service_cidr"`
	DNS         string `json:"dns"`
	Proxy       string `json:"proxy,omitempty"`
	MTU         int    `json:"mtu"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	DefaultClass string         `json:"default_class"`
	Classes      []StorageClass `json:"classes"`
}

// StorageClass 存储类
type StorageClass struct {
	Name          string            `json:"name"`
	Provisioner   string            `json:"provisioner"`
	Parameters    map[string]string `json:"parameters,omitempty"`
	ReclaimPolicy string            `json:"reclaim_policy"`
}

// WorkloadMetrics 工作负载指标
type WorkloadMetrics struct {
	WorkloadID  string    `json:"workload_id"`
	Timestamp   time.Time `json:"timestamp"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage int       `json:"memory_usage"`
	NetworkIn   int64     `json:"network_in"`
	NetworkOut  int64     `json:"network_out"`
	DiskRead    int64     `json:"disk_read"`
	DiskWrite   int64     `json:"disk_write"`
	Replicas    int       `json:"replicas"`
	Restarts    int       `json:"restarts"`
}

// ClusterStats 集群统计
type ClusterStats struct {
	TotalNodes       int     `json:"total_nodes"`
	ReadyNodes       int     `json:"ready_nodes"`
	TotalWorkloads   int     `json:"total_workloads"`
	RunningWorkloads int     `json:"running_workloads"`
	TotalFunctions   int     `json:"total_functions"`
	TotalAITasks     int     `json:"total_ai_tasks"`
	TotalCPU         float64 `json:"total_cpu"`
	UsedCPU          float64 `json:"used_cpu"`
	TotalMemoryMB    int     `json:"total_memory_mb"`
	UsedMemoryMB     int     `json:"used_memory_mb"`
	TotalGPU         int     `json:"total_gpu"`
	UsedGPU          int     `json:"used_gpu"`
	Deployments      int     `json:"deployments"`
}

// EdgeEvent 边缘事件
type EdgeEvent struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	NodeID     string    `json:"node_id"`
	WorkloadID string    `json:"workload_id,omitempty"`
	Message    string    `json:"message"`
	Severity   string    `json:"severity"`
	Timestamp  time.Time `json:"timestamp"`
}
