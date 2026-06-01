// Package edgeorchestrator 提供边缘计算编排功能
package edgeorchestrator

import (
	"sync"
	"time"
)

// ========== 边缘节点类型 ==========

// EdgeNodeStatus 边缘节点状态
type EdgeNodeStatus string

const (
	NodeStatusOnline      EdgeNodeStatus = "online"
	NodeStatusOffline     EdgeNodeStatus = "offline"
	NodeStatusDraining    EdgeNodeStatus = "draining"
	NodeStatusMaintenance EdgeNodeStatus = "maintenance"
)

// EdgeNode 边缘计算节点
type EdgeNode struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	IPAddress        string            `json:"ip_address"`
	Region           string            `json:"region"`
	Zone             string            `json:"zone"`
	Status           EdgeNodeStatus    `json:"status"`
	CPUCores         int               `json:"cpu_cores"`
	MemoryMB         int64             `json:"memory_mb"`
	DiskGB           int64             `json:"disk_gb"`
	GPUCount         int               `json:"gpu_count"`
	GPUModel         string            `json:"gpu_model,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	Taints           []Taint           `json:"taints,omitempty"`
	Capabilities     []string          `json:"capabilities,omitempty"`
	CurrentCPUUsage  float64           `json:"current_cpu_usage"`
	CurrentMemUsage  float64           `json:"current_mem_usage"`
	RunningTasks     int               `json:"running_tasks"`
	MaxTasks         int               `json:"max_tasks"`
	LastHeartbeat    time.Time         `json:"last_heartbeat"`
	RegisteredAt     time.Time         `json:"registered_at"`
	OperatingSystem  string            `json:"operating_system,omitempty"`
	Architecture     string            `json:"architecture,omitempty"`
	EndpointURL      string            `json:"endpoint_url,omitempty"`
}

// Taint 节点污点
type Taint struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"` // NoSchedule, PreferNoSchedule, NoExecute
}

// ========== 边缘任务类型 ==========

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusScheduled TaskStatus = "scheduled"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskPriority 任务优先级
type TaskPriority int

const (
	PriorityLow    TaskPriority = 0
	PriorityNormal TaskPriority = 1
	PriorityHigh   TaskPriority = 2
	PriorityUrgent TaskPriority = 3
)

// EdgeTask 边缘计算任务
type EdgeTask struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description,omitempty"`
	Type             TaskType          `json:"type"`
	Status           TaskStatus        `json:"status"`
	Priority         TaskPriority      `json:"priority"`
	AssignedNodeID   string            `json:"assigned_node_id,omitempty"`
	Image            string            `json:"image"`
	Command          []string          `json:"command,omitempty"`
	Args             []string          `json:"args,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	CPURequest       float64           `json:"cpu_request"`
	MemoryRequestMB  int64             `json:"memory_request_mb"`
	GPURequest       int               `json:"gpu_request,omitempty"`
	Timeout          time.Duration     `json:"timeout"`
	RetryCount       int               `json:"retry_count"`
	MaxRetries       int               `json:"max_retries"`
	NodeSelector     map[string]string `json:"node_selector,omitempty"`
	Affinity         *AffinityRule     `json:"affinity,omitempty"`
	Result           *TaskResult       `json:"result,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	ScheduledAt      *time.Time        `json:"scheduled_at,omitempty"`
	StartedAt        *time.Time        `json:"started_at,omitempty"`
	CompletedAt      *time.Time        `json:"completed_at,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
}

// TaskType 任务类型
type TaskType string

const (
	TaskTypeGeneral   TaskType = "general"
	TaskTypeAIInfer   TaskType = "ai_inference"
	TaskTypeDataProc  TaskType = "data_processing"
	TaskTypeML        TaskType = "machine_learning"
	TaskTypeVideoProc TaskType = "video_processing"
	TaskTypeIoT       TaskType = "iot_processing"
)

// AffinityRule 亲和性规则
type AffinityRule struct {
	PreferredNodes  []string          `json:"preferred_nodes,omitempty"`
	RequiredLabels  map[string]string `json:"required_labels,omitempty"`
	RequiredZones   []string          `json:"required_zones,omitempty"`
	AntiAffinity    []string          `json:"anti_affinity,omitempty"`
	SpreadAcrossZones bool            `json:"spread_across_zones,omitempty"`
}

// TaskResult 任务执行结果
type TaskResult struct {
	ExitCode   int             `json:"exit_code"`
	Output     string          `json:"output,omitempty"`
	Error      string          `json:"error,omitempty"`
	Artifacts  []Artifact      `json:"artifacts,omitempty"`
	Metrics    *TaskMetrics    `json:"metrics,omitempty"`
	CompletedAt time.Time      `json:"completed_at"`
}

// Artifact 任务产物
type Artifact struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	MimeType  string `json:"mime_type,omitempty"`
}

// TaskMetrics 任务运行指标
type TaskMetrics struct {
	CPUUsageAvg    float64       `json:"cpu_usage_avg"`
	MemUsageAvgMB  float64       `json:"mem_usage_avg_mb"`
	GPUUsageAvg    float64       `json:"gpu_usage_avg,omitempty"`
	NetworkInBytes int64         `json:"network_in_bytes"`
	NetworkOutBytes int64        `json:"network_out_bytes"`
	Duration       time.Duration `json:"duration"`
}

// ========== AI 推理任务类型 ==========

// AIInferenceTask AI推理任务
type AIInferenceTask struct {
	TaskID        string             `json:"task_id"`
	ModelName     string             `json:"model_name"`
	ModelVersion  string             `json:"model_version"`
	Framework     string             `json:"framework"` // onnx, tensorflow, pytorch, tensorrt
	InputType     string             `json:"input_type"` // image, text, audio, video
	InputData     []byte             `json:"input_data,omitempty"`
	InputURL      string             `json:"input_url,omitempty"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	BatchSize     int                `json:"batch_size"`
	Priority      TaskPriority       `json:"priority"`
	MaxLatency    time.Duration      `json:"max_latency"`
	ResultChan    chan *InferenceResult `json:"-"`
}

// InferenceResult 推理结果
type InferenceResult struct {
	TaskID      string                 `json:"task_id"`
	Predictions []Prediction           `json:"predictions"`
	Latency     time.Duration          `json:"latency"`
	ModelInfo   *ModelInfo             `json:"model_info,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// Prediction 单个预测结果
type Prediction struct {
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	BBox       *BoundingBox       `json:"bbox,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// BoundingBox 边界框
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ModelInfo 模型信息
type ModelInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Framework  string `json:"framework"`
	InputShape []int  `json:"input_shape"`
	Parameters int64  `json:"parameters"`
}

// ========== 调度器类型 ==========

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	Strategy           SchedulingStrategy `json:"strategy"`
	MaxTasksPerNode    int                `json:"max_tasks_per_node"`
	HeartbeatTimeout   time.Duration      `json:"heartbeat_timeout"`
	HealthCheckInterval time.Duration     `json:"health_check_interval"`
	EnableGPU          bool               `json:"enable_gpu"`
	EnableAffinity     bool               `json:"enable_affinity"`
	EnableTaints       bool               `json:"enable_taints"`
}

// SchedulingStrategy 调度策略
type SchedulingStrategy string

const (
	StrategyRoundRobin  SchedulingStrategy = "round_robin"
	StrategyLeastLoad   SchedulingStrategy = "least_load"
	StrategyRandom      SchedulingStrategy = "random"
	StrategyBinPack     SchedulingStrategy = "bin_pack"
	StrategySpread      SchedulingStrategy = "spread"
)

// ========== 同步和监控类型 ==========

// SyncStatus 同步状态
type SyncStatus struct {
	NodeID          string    `json:"node_id"`
	LastSyncTime    time.Time `json:"last_sync_time"`
	SyncedTasks     int       `json:"synced_tasks"`
	PendingSyncs    int       `json:"pending_syncs"`
	FailedSyncs     int       `json:"failed_syncs"`
	SyncLatencyMs   float64   `json:"sync_latency_ms"`
}

// ClusterMetrics 集群指标
type ClusterMetrics struct {
	TotalNodes      int       `json:"total_nodes"`
	OnlineNodes     int       `json:"online_nodes"`
	OfflineNodes    int       `json:"offline_nodes"`
	TotalTasks      int       `json:"total_tasks"`
	RunningTasks    int       `json:"running_tasks"`
	PendingTasks    int       `json:"pending_tasks"`
	CompletedTasks  int       `json:"completed_tasks"`
	FailedTasks     int       `json:"failed_tasks"`
	TotalCPUCores   int       `json:"total_cpu_cores"`
	UsedCPUCores    float64   `json:"used_cpu_cores"`
	TotalMemoryMB   int64     `json:"total_memory_mb"`
	UsedMemoryMB    int64     `json:"used_memory_mb"`
	TotalGPUs       int       `json:"total_gpus"`
	UsedGPUs        int       `json:"used_gpus"`
	AvgTaskLatency  float64   `json:"avg_task_latency_ms"`
	Timestamp       time.Time `json:"timestamp"`
}

// NodeHealth 节点健康状态
type NodeHealth struct {
	NodeID        string    `json:"node_id"`
	Healthy       bool      `json:"healthy"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryPercent float64   `json:"memory_percent"`
	DiskPercent   float64   `json:"disk_percent"`
	GPUPercent    float64   `json:"gpu_percent,omitempty"`
	NetworkInBps  int64     `json:"network_in_bps"`
	NetworkOutBps int64     `json:"network_out_bps"`
	Uptime        int64     `json:"uptime_seconds"`
	LastCheck     time.Time `json:"last_check"`
	Warnings      []string  `json:"warnings,omitempty"`
}

// ========== 请求/响应类型 ==========

// RegisterNodeRequest 注册节点请求
type RegisterNodeRequest struct {
	Name         string            `json:"name" binding:"required"`
	IPAddress    string            `json:"ip_address" binding:"required"`
	Region       string            `json:"region"`
	Zone         string            `json:"zone"`
	CPUCores     int               `json:"cpu_cores" binding:"required,min=1"`
	MemoryMB     int64             `json:"memory_mb" binding:"required,min=1"`
	DiskGB       int64             `json:"disk_gb"`
	GPUCount     int               `json:"gpu_count"`
	GPUModel     string            `json:"gpu_model"`
	Labels       map[string]string `json:"labels"`
	Capabilities []string          `json:"capabilities"`
	MaxTasks     int               `json:"max_tasks"`
	EndpointURL  string            `json:"endpoint_url"`
}

// SubmitTaskRequest 提交任务请求
type SubmitTaskRequest struct {
	Name            string            `json:"name" binding:"required"`
	Description     string            `json:"description"`
	Type            TaskType          `json:"type"`
	Priority        TaskPriority      `json:"priority"`
	Image           string            `json:"image" binding:"required"`
	Command         []string          `json:"command"`
	Args            []string          `json:"args"`
	Env             map[string]string `json:"env"`
	CPURequest      float64           `json:"cpu_request"`
	MemoryRequestMB int64             `json:"memory_request_mb"`
	GPURequest      int               `json:"gpu_request"`
	TimeoutSec      int               `json:"timeout_sec"`
	MaxRetries      int               `json:"max_retries"`
	NodeSelector    map[string]string `json:"node_selector"`
	Affinity        *AffinityRule     `json:"affinity"`
	Labels          map[string]string `json:"labels"`
}

// SubmitInferenceRequest 提交推理请求
type SubmitInferenceRequest struct {
	ModelName    string                 `json:"model_name" binding:"required"`
	ModelVersion string                 `json:"model_version"`
	Framework    string                 `json:"framework" binding:"required"`
	InputType    string                 `json:"input_type" binding:"required"`
	InputURL     string                 `json:"input_url"`
	Parameters   map[string]interface{} `json:"parameters"`
	BatchSize    int                    `json:"batch_size"`
	Priority     TaskPriority           `json:"priority"`
	MaxLatencyMs int64                  `json:"max_latency_ms"`
}

// ListTasksRequest 列表任务请求
type ListTasksRequest struct {
	Status   string `form:"status"`
	Type     string `form:"type"`
	NodeID   string `form:"node_id"`
	Priority int    `form:"priority"`
	Limit    int    `form:"limit"`
	Offset   int    `form:"offset"`
}

// ========== Manager ==========

// Manager 边缘编排器管理器
type Manager struct {
	mu              sync.RWMutex
	nodes           map[string]*EdgeNode
	tasks           map[string]*EdgeTask
	inferenceTasks  map[string]*AIInferenceTask
	schedulerConfig *SchedulerConfig
	syncStatuses    map[string]*SyncStatus
	stopCh          chan struct{}
}

// NewManager 创建管理器实例
func NewManager(config *SchedulerConfig) *Manager {
	if config == nil {
		config = &SchedulerConfig{
			Strategy:            StrategyLeastLoad,
			MaxTasksPerNode:     50,
			HeartbeatTimeout:    30 * time.Second,
			HealthCheckInterval: 10 * time.Second,
			EnableGPU:           true,
			EnableAffinity:      true,
			EnableTaints:        true,
		}
	}
	return &Manager{
		nodes:           make(map[string]*EdgeNode),
		tasks:           make(map[string]*EdgeTask),
		inferenceTasks:  make(map[string]*AIInferenceTask),
		schedulerConfig: config,
		syncStatuses:    make(map[string]*SyncStatus),
		stopCh:          make(chan struct{}),
	}
}
