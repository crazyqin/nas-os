// Package containerha 提供容器高可用故障转移功能
// 参考 TrueNAS 26 的容器HA特性，支持 LXC/Docker 容器的故障检测和自动故障转移
package containerha

import (
	"sync"
	"time"
)

// ContainerHAConfig 容器HA配置
type ContainerHAConfig struct {
	// ClusterName 集群名称
	ClusterName string `json:"clusterName" yaml:"clusterName"`
	// PrimaryNode 主节点信息
	PrimaryNode NodeConfig `json:"primaryNode" yaml:"primaryNode"`
	// SecondaryNodes 从节点列表
	SecondaryNodes []NodeConfig `json:"secondaryNodes" yaml:"secondaryNodes"`
	// HealthCheckInterval 健康检查间隔（秒）
	HealthCheckInterval int `json:"healthCheckInterval" yaml:"healthCheckInterval"`
	// FailureThreshold 故障阈值（连续失败次数）
	FailureThreshold int `json:"failureThreshold" yaml:"failureThreshold"`
	// AutoFailback 是否自动回切到主节点
	AutoFailback bool `json:"autoFailback" yaml:"autoFailback"`
	// FailbackDelay 自动回切延迟（秒）
	FailbackDelay int `json:"failbackDelay" yaml:"failbackDelay"`
	// HeartbeatTimeout 心跳超时时间（秒）
	HeartbeatTimeout int `json:"heartbeatTimeout" yaml:"heartbeatTimeout"`
	// SyncMode 状态同步模式：checkpoint（检查点）/ realtime（实时）
	SyncMode string `json:"syncMode" yaml:"syncMode"`
	// SyncInterval 同步间隔（秒）
	SyncInterval int `json:"syncInterval" yaml:"syncInterval"`
	// ContainerContainers 要保护的容器列表
	ProtectedContainers []ContainerConfig `json:"protectedContainers" yaml:"protectedContainers"`
	// EnableStaticIP 是否支持静态IP故障转移
	EnableStaticIP bool `json:"enableStaticIP" yaml:"enableStaticIP"`
	// VirtualIPs 虚拟IP配置列表
	VirtualIPs []VirtualIPConfig `json:"virtualIPs" yaml:"virtualIPs"`
	// EnableResourceCheck 是否启用资源检查
	EnableResourceCheck bool `json:"enableResourceCheck" yaml:"enableResourceCheck"`
	// ResourceThresholds 资源阈值配置
	ResourceThresholds ResourceThresholds `json:"resourceThresholds" yaml:"resourceThresholds"`
}

// NodeConfig 节点配置
type NodeConfig struct {
	// ID 节点唯一标识
	ID string `json:"id" yaml:"id"`
	// Address 节点地址（IP或主机名）
	Address string `json:"address" yaml:"address"`
	// Port 节点端口
	Port int `json:"port" yaml:"port"`
	// Role 节点角色：master/slave
	Role string `json:"role" yaml:"role"`
	// Weight 节点权重（用于选举）
	Weight int `json:"weight" yaml:"weight"`
}

// ContainerConfig 容器配置
type ContainerConfig struct {
	// ContainerID 容器ID或名称
	ContainerID string `json:"containerId" yaml:"containerId"`
	// Type 容器类型：lxc/docker
	Type string `json:"type" yaml:"type"`
	// EnableFailover 是否启用故障转移
	EnableFailover bool `json:"enableFailover" yaml:"enableFailover"`
	// Priority 故障转移优先级（数字越小优先级越高）
	Priority int `json:"priority" yaml:"priority"`
	// StaticIP 容器静态IP（如果启用静态IP故障转移）
	StaticIP string `json:"staticIP,omitempty" yaml:"staticIP,omitempty"`
	// HealthCheckPort 健康检查端口
	HealthCheckPort int `json:"healthCheckPort,omitempty" yaml:"healthCheckPort,omitempty"`
	// HealthCheckPath 健康检查路径（HTTP健康检查）
	HealthCheckPath string `json:"healthCheckPath,omitempty" yaml:"healthCheckPath,omitempty"`
}

// VirtualIPConfig 虚拟IP配置
type VirtualIPConfig struct {
	// IP 虚拟IP地址
	IP string `json:"ip" yaml:"ip"`
	// Interface 网络接口
	Interface string `json:"interface" yaml:"interface"`
	// SubnetMask 子网掩码
	SubnetMask string `json:"subnetMask" yaml:"subnetMask"`
	// Gateway 网关地址
	Gateway string `json:"gateway,omitempty" yaml:"gateway,omitempty"`
}

// ResourceThresholds 资源阈值配置
type ResourceThresholds struct {
	// CPUThreshold CPU使用率阈值（百分比）
	CPUThreshold float64 `json:"cpuThreshold" yaml:"cpuThreshold"`
	// MemoryThreshold 内存使用率阈值（百分比）
	MemoryThreshold float64 `json:"memoryThreshold" yaml:"memoryThreshold"`
	// DiskThreshold 磁盘使用率阈值（百分比）
	DiskThreshold float64 `json:"diskThreshold" yaml:"diskThreshold"`
}

// ContainerHANode HA节点信息
type ContainerHANode struct {
	// ID 节点唯一标识
	ID string `json:"id"`
	// Address 节点地址
	Address string `json:"address"`
	// Port 节点端口
	Port int `json:"port"`
	// Role 节点角色：master/slave
	Role string `json:"role"`
	// Status 节点状态：online/offline/degraded
	Status string `json:"status"`
	// LastHeartbeat 最后心跳时间
	LastHeartbeat time.Time `json:"lastHeartbeat"`
	// HealthScore 健康分数（0-100）
	HealthScore int `json:"healthScore"`
	// ResourceUsage 资源使用情况
	ResourceUsage ResourceUsage `json:"resourceUsage"`
	// RunningContainers 运行中的容器数量
	RunningContainers int `json:"runningContainers"`
	// Weight 节点权重
	Weight int `json:"weight"`
	// JoinedAt 节点加入时间
	JoinedAt time.Time `json:"joinedAt"`
	// FailoverCount 故障转移次数
	FailoverCount int `json:"failoverCount"`
	// LastFailoverTime 最后故障转移时间
	LastFailoverTime *time.Time `json:"lastFailoverTime,omitempty"`
}

// ResourceUsage 资源使用情况
type ResourceUsage struct {
	// CPUUsage CPU使用率（百分比）
	CPUUsage float64 `json:"cpuUsage"`
	// MemoryUsage 内存使用率（百分比）
	MemoryUsage float64 `json:"memoryUsage"`
	// DiskUsage 磁盘使用率（百分比）
	DiskUsage float64 `json:"diskUsage"`
	// NetworkIn 网络入站流量（字节/秒）
	NetworkIn int64 `json:"networkIn"`
	// NetworkOut 网络出站流量（字节/秒）
	NetworkOut int64 `json:"networkOut"`
	// ContainerCount 容器数量
	ContainerCount int `json:"containerCount"`
}

// ContainerHAStatus 容器HA状态
type ContainerHAStatus struct {
	// ClusterName 集群名称
	ClusterName string `json:"clusterName"`
	// ClusterStatus 集群状态：healthy/degraded/critical
	ClusterStatus string `json:"clusterStatus"`
	// ActiveMaster 当前活动主节点ID
	ActiveMaster string `json:"activeMaster"`
	// Nodes 节点列表
	Nodes []ContainerHANode `json:"nodes"`
	// RunningContainers 运行中的受保护容器
	RunningContainers []ProtectedContainer `json:"runningContainers"`
	// SyncStatus 同步状态
	SyncStatus SyncStatus `json:"syncStatus"`
	// LastFailoverTime 最后故障转移时间
	LastFailoverTime *time.Time `json:"lastFailoverTime,omitempty"`
	// LastFailoverReason 最后故障转移原因
	LastFailoverReason string `json:"lastFailoverReason,omitempty"`
	// FailoverHistory 故障转移历史
	FailoverHistory []FailoverEvent `json:"failoverHistory"`
	// StartTime 状态查询时间
	StartTime time.Time `json:"startTime"`
	// Uptime 集群运行时间
	Uptime time.Duration `json:"uptime"`
}

// ProtectedContainer 受保护的容器信息
type ProtectedContainer struct {
	// ContainerID 容器ID
	ContainerID string `json:"containerId"`
	// Type 容器类型：lxc/docker
	Type string `json:"type"`
	// Name 容器名称
	Name string `json:"name"`
	// Status 容器状态：running/stopped/paused
	Status string `json:"status"`
	// CurrentNode 当前运行节点ID
	CurrentNode string `json:"currentNode"`
	// OriginalNode 原始节点ID（主节点）
	OriginalNode string `json:"originalNode"`
	// StaticIP 静态IP（如果有）
	StaticIP string `json:"staticIP,omitempty"`
	// HealthStatus 健康状态：healthy/unhealthy/unknown
	HealthStatus string `json:"healthStatus"`
	// LastSyncTime 最后同步时间
	LastSyncTime *time.Time `json:"lastSyncTime,omitempty"`
	// CheckpointPath 检查点路径（如果有）
	CheckpointPath string `json:"checkpointPath,omitempty"`
	// FailoverCount 故障转移次数
	FailoverCount int `json:"failoverCount"`
	// Priority 故障转移优先级
	Priority int `json:"priority"`
}

// SyncStatus 同步状态
type SyncStatus struct {
	// Mode 同步模式
	Mode string `json:"mode"`
	// State 同步状态：syncing/idle/failed
	State string `json:"state"`
	// Progress 同步进度（百分比）
	Progress float64 `json:"progress"`
	// LastSyncTime 最后同步时间
	LastSyncTime *time.Time `json:"lastSyncTime,omitempty"`
	// SyncedContainers 已同步容器数量
	SyncedContainers int `json:"syncedContainers"`
	// PendingContainers 待同步容器数量
	PendingContainers int `json:"pendingContainers"`
	// FailedSyncs 同步失败次数
	FailedSyncs int `json:"failedSyncs"`
	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// FailoverEvent 故障转移事件
type FailoverEvent struct {
	// EventID 事件ID
	EventID string `json:"eventId"`
	// Timestamp 事件时间
	Timestamp time.Time `json:"timestamp"`
	// Type 事件类型：failover/failback/planned
	Type string `json:"type"`
	// SourceNode 源节点ID
	SourceNode string `json:"sourceNode"`
	// TargetNode 目标节点ID
	TargetNode string `json:"targetNode"`
	// AffectedContainers 受影响的容器列表
	AffectedContainers []string `json:"affectedContainers"`
	// Reason 故障转移原因
	Reason string `json:"reason"`
	// Status 事件状态：success/failed/partial
	Status string `json:"status"`
	// Duration 事件持续时间
	Duration time.Duration `json:"duration"`
	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// HeartbeatMessage 心跳消息
type HeartbeatMessage struct {
	// NodeID 节点ID
	NodeID string `json:"nodeId"`
	// Timestamp 心跳时间戳
	Timestamp time.Time `json:"timestamp"`
	// Status 节点状态
	Status string `json:"status"`
	// ResourceUsage 资源使用情况
	ResourceUsage ResourceUsage `json:"resourceUsage"`
	// ContainerStates 容器状态列表
	ContainerStates []ContainerState `json:"containerStates"`
	// SequenceNumber 序列号（用于检测消息丢失）
	SequenceNumber int64 `json:"sequenceNumber"`
}

// ContainerState 容器状态
type ContainerState struct {
	// ContainerID 容器ID
	ContainerID string `json:"containerId"`
	// Type 容器类型
	Type string `json:"type"`
	// Status 容器状态
	Status string `json:"status"`
	// PID 容器进程ID
	PID int `json:"pid,omitempty"`
	// MemoryUsage 内存使用量（字节）
	MemoryUsage int64 `json:"memoryUsage,omitempty"`
	// CPUUsage CPU使用率（百分比）
	CPUUsage float64 `json:"cpuUsage,omitempty"`
	// NetworkState 网络状态
	NetworkState string `json:"networkState,omitempty"`
}

// FailoverRequest 故障转移请求
type FailoverRequest struct {
	// TargetNode 目标节点ID（可选，默认自动选择）
	TargetNode string `json:"targetNode,omitempty"`
	// Containers 指定要故障转移的容器（可选，默认所有受保护容器）
	Containers []string `json:"containers,omitempty"`
	// Reason 故障转移原因
	Reason string `json:"reason"`
	// Force 是否强制故障转移（跳过健康检查）
	Force bool `json:"force,omitempty"`
	// Planned 是否计划内维护
	Planned bool `json:"planned,omitempty"`
}

// FailoverResponse 故障转移响应
type FailoverResponse struct {
	// Success 是否成功
	Success bool `json:"success"`
	// EventID 事件ID
	EventID string `json:"eventId"`
	// Message 消息
	Message string `json:"message"`
	// AffectedContainers 受影响的容器列表
	AffectedContainers []string `json:"affectedContainers,omitempty"`
	// TargetNode 目标节点
	TargetNode string `json:"targetNode"`
	// Warnings 警告信息
	Warnings []string `json:"warnings,omitempty"`
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	// NodeID 节点ID
	NodeID string `json:"nodeId"`
	// Healthy 是否健康
	Healthy bool `json:"healthy"`
	// CheckTime 检查时间
	CheckTime time.Time `json:"checkTime"`
	// ResponseTime 响应时间（毫秒）
	ResponseTime int64 `json:"responseTime"`
	// Failures 连续失败次数
	Failures int `json:"failures"`
	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`
	// ContainerHealth 容器健康状态
	ContainerHealth []ContainerHealthResult `json:"containerHealth,omitempty"`
}

// ContainerHealthResult 容器健康检查结果
type ContainerHealthResult struct {
	// ContainerID 容器ID
	ContainerID string `json:"containerId"`
	// Healthy 是否健康
	Healthy bool `json:"healthy"`
	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`
	// ResponseTime 响应时间（毫秒）
	ResponseTime int64 `json:"responseTime"`
}

// ClusterEvent 集群事件
type ClusterEvent struct {
	// Type 事件类型
	Type string `json:"type"`
	// Source 事件来源
	Source string `json:"source"`
	// Timestamp 事件时间
	Timestamp time.Time `json:"timestamp"`
	// Data 事件数据
	Data interface{} `json:"data"`
}

// FailoverManager 故障转移管理器
type FailoverManager struct {
	// config 配置
	config *ContainerHAConfig
	// nodes 节点信息
	nodes map[string]*ContainerHANode
	// containers 容器信息
	containers map[string]*ProtectedContainer
	// nodeMu 节点锁
	nodeMu sync.RWMutex
	// containerMu 容器锁
	containerMu sync.RWMutex
	// status 状态
	status *ContainerHAStatus
	// statusMu 状态锁
	statusMu sync.RWMutex
	// failoverHistory 故障转移历史
	failoverHistory []FailoverEvent
	// historyMu 历史锁
	historyMu sync.RWMutex
	// healthChecker 健康检查器
	healthChecker *HealthChecker
	// syncManager 同步管理器
	syncManager *SyncManager
	// stopCh 停止信号
	stopCh chan struct{}
	// eventCh 事件通道
	eventCh chan ClusterEvent
	// localNodeID 本地节点ID
	localNodeID string
	// isMaster 是否为主节点
	isMaster bool
	// startTime 启动时间
	startTime time.Time
}

// HealthChecker 健康检查器
type HealthChecker struct {
	// manager 管理器引用
	manager *FailoverManager
	// checkInterval 检查间隔
	checkInterval time.Duration
	// timeout 超时时间
	timeout time.Duration
	// stopCh 停止信号
	stopCh chan struct{}
	// results 检查结果
	results map[string]*HealthCheckResult
	// resultsMu 结果锁
	resultsMu sync.RWMutex
}

// SyncManager 同步管理器
type SyncManager struct {
	// manager 管理器引用
	manager *FailoverManager
	// mode 同步模式
	mode string
	// syncInterval 同步间隔
	syncInterval time.Duration
	// stopCh 停止信号
	stopCh chan struct{}
	// status 同步状态
	status SyncStatus
	// statusMu 状态锁
	statusMu sync.RWMutex
}

// Checkpoint 检查点信息
type Checkpoint struct {
	// ContainerID 容器ID
	ContainerID string `json:"containerId"`
	// NodeID 节点ID
	NodeID string `json:"nodeId"`
	// Path 检查点路径
	Path string `json:"path"`
	// Timestamp 创建时间
	Timestamp time.Time `json:"timestamp"`
	// Size 大小（字节）
	Size int64 `json:"size"`
	// Status 状态：creating/ready/corrupted
	Status string `json:"status"`
	// Metadata 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ContainerMigration 容器迁移信息
type ContainerMigration struct {
	// MigrationID 迁移ID
	MigrationID string `json:"migrationId"`
	// ContainerID 容器ID
	ContainerID string `json:"containerId"`
	// SourceNode 源节点
	SourceNode string `json:"sourceNode"`
	// TargetNode 目标节点
	TargetNode string `json:"targetNode"`
	// StartTime 开始时间
	StartTime time.Time `json:"startTime"`
	// EndTime 结束时间
	EndTime *time.Time `json:"endTime,omitempty"`
	// Status 状态：pending/in-progress/completed/failed
	Status string `json:"status"`
	// Progress 进度（百分比）
	Progress float64 `json:"progress"`
	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// APIError API错误
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// APIResponse API响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}
