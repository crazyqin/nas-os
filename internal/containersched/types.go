// Package containersched 提供智能容器调度功能，参考 TrueNAS SCALE 的容器编排能力
package containersched

import (
	"time"
)

// ========== 节点与资源类型 ==========

// Node 调度节点
type Node struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Host          string            `json:"host"`
	Role          NodeRole          `json:"role"`
	Status        NodeStatus        `json:"status"`
	Resources     *NodeResources    `json:"resources"`
	Labels        map[string]string `json:"labels,omitempty"`
	Taints        []Taint           `json:"taints,omitempty"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// NodeRole 节点角色
type NodeRole string

const (
	NodeRoleMaster NodeRole = "master"
	NodeRoleWorker NodeRole = "worker"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeStatusReady      NodeStatus = "ready"
	NodeStatusNotReady   NodeStatus = "not_ready"
	NodeStatusScheduling NodeStatus = "scheduling"
	NodeStatusDraining   NodeStatus = "draining"
	NodeStatusOffline    NodeStatus = "offline"
	NodeStatusPowerSave  NodeStatus = "power_save"
)

// NodeResources 节点资源
type NodeResources struct {
	CPU       CPUResource    `json:"cpu"`
	Memory    MemoryResource `json:"memory"`
	DiskIO    DiskIOResource `json:"disk_io"`
	Network   NetworkResource `json:"network"`
	GPU       *GPUResource   `json:"gpu,omitempty"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// CPUResource CPU 资源
type CPUResource struct {
	TotalCores   int     `json:"total_cores"`
	UsedCores    float64 `json:"used_cores"`
	FreeCores    float64 `json:"free_cores"`
	UsagePercent float64 `json:"usage_percent"`
}

// MemoryResource 内存资源
type MemoryResource struct {
	TotalBytes   int64   `json:"total_bytes"`
	UsedBytes    int64   `json:"used_bytes"`
	FreeBytes    int64   `json:"free_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// DiskIOResource 磁盘 IO 资源
type DiskIOResource struct {
	ReadBPS      int64   `json:"read_bps"`
	WriteBPS     int64   `json:"write_bps"`
	ReadIOPS     int64   `json:"read_iops"`
	WriteIOPS    int64   `json:"write_iops"`
	UsagePercent float64 `json:"usage_percent"`
}

// NetworkResource 网络资源
type NetworkResource struct {
	BandwidthBPS int64   `json:"bandwidth_bps"`
	UsedBPS      int64   `json:"used_bps"`
	UsagePercent float64 `json:"usage_percent"`
}

// GPUResource GPU 资源
type GPUResource struct {
	TotalDevices int     `json:"total_devices"`
	UsedDevices  int     `json:"used_devices"`
	MemoryTotal  int64   `json:"memory_total"`
	MemoryUsed   int64   `json:"memory_used"`
	UsagePercent float64 `json:"usage_percent"`
}

// Taint 节点污点
type Taint struct {
	Key    string      `json:"key"`
	Value  string      `json:"value,omitempty"`
	Effect TaintEffect `json:"effect"`
}

// TaintEffect 污点效果
type TaintEffect string

const (
	TaintEffectNoSchedule       TaintEffect = "NoSchedule"
	TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"
	TaintEffectNoExecute        TaintEffect = "NoExecute"
)

// ========== 容器调度请求 ==========

// ScheduleRequest 调度请求
type ScheduleRequest struct {
	ContainerID   string            `json:"container_id" binding:"required"`
	ContainerName string            `json:"container_name"`
	Image         string            `json:"image" binding:"required"`
	Resources     *ResourceRequest  `json:"resources"`
	Constraints   *ScheduleConstraints `json:"constraints,omitempty"`
	Priority      Priority          `json:"priority"`
	Labels        map[string]string `json:"labels,omitempty"`
	RequestedAt   time.Time         `json:"requested_at"`
}

// ResourceRequest 资源请求
type ResourceRequest struct {
	CPUCores     float64 `json:"cpu_cores"`
	MemoryBytes  int64   `json:"memory_bytes"`
	DiskIOBPS    int64   `json:"disk_io_bps"`
	NetworkBPS   int64   `json:"network_bps"`
	GPUDevices   int     `json:"gpu_devices"`
}

// ScheduleConstraints 调度约束
type ScheduleConstraints struct {
	NodeSelector    map[string]string    `json:"node_selector,omitempty"`
	Affinity        []AffinityRule       `json:"affinity,omitempty"`
	AntiAffinity    []AffinityRule       `json:"anti_affinity,omitempty"`
	Tolerations     []Toleration         `json:"tolerations,omitempty"`
	PreferredNodes  []string             `json:"preferred_nodes,omitempty"`
	ExcludedNodes   []string             `json:"excluded_nodes,omitempty"`
	TopologyZone    string               `json:"topology_zone,omitempty"`
}

// AffinityRule 亲和性规则
type AffinityRule struct {
	TargetContainer string            `json:"target_container"`
	Weight          int               `json:"weight"`
	Labels          map[string]string `json:"labels,omitempty"`
}

// Toleration 容忍
type Toleration struct {
	Key      string      `json:"key"`
	Operator string      `json:"operator"` // Equal, Exists
	Value    string      `json:"value,omitempty"`
	Effect   TaintEffect `json:"effect"`
}

// Priority 优先级
type Priority int

const (
	PriorityLow      Priority = 1
	PriorityNormal   Priority = 5
	PriorityHigh     Priority = 8
	PriorityCritical Priority = 10
)

// ========== 调度结果 ==========

// ScheduleResult 调度结果
type ScheduleResult struct {
	ContainerID string    `json:"container_id"`
	NodeID      string    `json:"node_id"`
	NodeName    string    `json:"node_name"`
	Score       int       `json:"score"`
	Reason      string    `json:"reason"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
}

// ========== 调度队列 ==========

// QueueItem 队列项
type QueueItem struct {
	Request   *ScheduleRequest `json:"request"`
	Status    QueueItemStatus  `json:"status"`
	Attempts  int              `json:"attempts"`
	LastError string           `json:"last_error,omitempty"`
	QueuedAt  time.Time        `json:"queued_at"`
	StartedAt *time.Time       `json:"started_at,omitempty"`
}

// QueueItemStatus 队列项状态
type QueueItemStatus string

const (
	QueueItemStatusPending    QueueItemStatus = "pending"
	QueueItemStatusProcessing QueueItemStatus = "processing"
	QueueItemStatusScheduled  QueueItemStatus = "scheduled"
	QueueItemStatusFailed     QueueItemStatus = "failed"
	QueueItemStatusCancelled  QueueItemStatus = "cancelled"
)

// ========== 自动扩缩容 ==========

// AutoScalePolicy 自动扩缩容策略
type AutoScalePolicy struct {
	ID            string        `json:"id"`
	ContainerName string        `json:"container_name"`
	Enabled       bool          `json:"enabled"`
	MinReplicas   int           `json:"min_replicas"`
	MaxReplicas   int           `json:"max_replicas"`
	Metrics       []ScaleMetric `json:"metrics"`
	Cooldown      time.Duration `json:"cooldown"`
	ScaleUpStep   int           `json:"scale_up_step"`
	ScaleDownStep int           `json:"scale_down_step"`
	LastScaleAt   *time.Time    `json:"last_scale_at,omitempty"`
}

// ScaleMetric 扩缩容指标
type ScaleMetric struct {
	Type    MetricType `json:"type"`
	Target  float64    `json:"target"`
	Current float64    `json:"current"`
}

// MetricType 指标类型
type MetricType string

const (
	MetricTypeCPU       MetricType = "cpu"
	MetricTypeMemory    MetricType = "memory"
	MetricTypeRequests  MetricType = "requests"
	MetricTypeCustom    MetricType = "custom"
)

// ========== 节能模式 ==========

// PowerSaveConfig 节能模式配置
type PowerSaveConfig struct {
	Enabled           bool    `json:"enabled"`
	Threshold         float64 `json:"threshold"`          // 资源使用率阈值
	MinActiveNodes    int     `json:"min_active_nodes"`   // 最小活跃节点数
	ConsolidationTime string  `json:"consolidation_time"` // 整合时间窗口
}

// ========== 容器放置记录 ==========

// Placement 容器放置记录
type Placement struct {
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	NodeID        string    `json:"node_id"`
	NodeName      string    `json:"node_name"`
	Resources     *ResourceRequest `json:"resources"`
	ScheduledAt   time.Time `json:"scheduled_at"`
	Priority      Priority  `json:"priority"`
}

// ========== API 请求/响应类型 ==========

// CreateNodeRequest 创建节点请求
type CreateNodeRequest struct {
	Name   string            `json:"name" binding:"required"`
	Host   string            `json:"host" binding:"required"`
	Role   NodeRole          `json:"role"`
	Labels map[string]string `json:"labels,omitempty"`
	Taints []Taint           `json:"taints,omitempty"`
}

// UpdateNodeRequest 更新节点请求
type UpdateNodeRequest struct {
	Name   *string           `json:"name,omitempty"`
	Role   *NodeRole         `json:"role,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	Taints []Taint           `json:"taints,omitempty"`
}

// UpdateNodeResourcesRequest 更新节点资源请求
type UpdateNodeResourcesRequest struct {
	Resources *NodeResources `json:"resources" binding:"required"`
}

// EnqueueRequest 入队请求
type EnqueueRequest struct {
	ContainerID   string            `json:"container_id" binding:"required"`
	ContainerName string            `json:"container_name"`
	Image         string            `json:"image" binding:"required"`
	Resources     *ResourceRequest  `json:"resources"`
	Constraints   *ScheduleConstraints `json:"constraints,omitempty"`
	Priority      Priority          `json:"priority"`
	Labels        map[string]string `json:"labels,omitempty"`
}

// UpdateAutoScaleRequest 更新自动扩缩容请求
type UpdateAutoScaleRequest struct {
	Enabled       *bool          `json:"enabled,omitempty"`
	MinReplicas   *int           `json:"min_replicas,omitempty"`
	MaxReplicas   *int           `json:"max_replicas,omitempty"`
	Metrics       []ScaleMetric  `json:"metrics,omitempty"`
	Cooldown      *time.Duration `json:"cooldown,omitempty"`
	ScaleUpStep   *int           `json:"scale_up_step,omitempty"`
	ScaleDownStep *int           `json:"scale_down_step,omitempty"`
}

// UpdatePowerSaveRequest 更新节能模式请求
type UpdatePowerSaveRequest struct {
	Enabled           *bool    `json:"enabled,omitempty"`
	Threshold         *float64 `json:"threshold,omitempty"`
	MinActiveNodes    *int     `json:"min_active_nodes,omitempty"`
	ConsolidationTime *string  `json:"consolidation_time,omitempty"`
}

// ScheduleStats 调度统计
type ScheduleStats struct {
	TotalScheduled   int     `json:"total_scheduled"`
	TotalFailed      int     `json:"total_failed"`
	PendingInQueue   int     `json:"pending_in_queue"`
	ActiveNodes      int     `json:"active_nodes"`
	TotalContainers  int     `json:"total_containers"`
	AverageScore     float64 `json:"average_score"`
	LastScheduledAt  *time.Time `json:"last_scheduled_at,omitempty"`
}

// ========== 标准响应 ==========

// Response 标准 API 响应
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
