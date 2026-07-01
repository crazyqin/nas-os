// Package clustermgr 提供集群管理器功能
// 参考群晖 DSM Cluster Manager，实现多节点集群管理、工作负载迁移、
// QoS 控管、集中化保护及节点健康监控
package clustermgr

import (
	"time"
)

// ========== 节点角色定义 ==========

// NodeRole 节点角色.
type NodeRole string

const (
	RoleLeader    NodeRole = "leader"    // 主节点
	RoleFollower  NodeRole = "follower"  // 从节点
	RoleWitness   NodeRole = "witness"   // 见证节点
	RoleStandby   NodeRole = "standby"   // 待命节点
)

// NodeStatus 节点运行状态.
type NodeStatus string

const (
	NodeOnline       NodeStatus = "online"       // 在线
	NodeOffline      NodeStatus = "offline"      // 离线
	NodeDegraded     NodeStatus = "degraded"     // 降级
	NodeMaintenance  NodeStatus = "maintenance"  // 维护中
	NodeJoining      NodeStatus = "joining"      // 加入中
	NodeLeaving      NodeStatus = "leaving"      // 离开中
)

// ========== 集群状态定义 ==========

// ClusterStatus 集群整体状态.
type ClusterStatus string

const (
	ClusterHealthy   ClusterStatus = "healthy"   // 健康
	ClusterWarning   ClusterStatus = "warning"   // 警告
	ClusterCritical  ClusterStatus = "critical"  // 严重
	ClusterDegraded  ClusterStatus = "degraded"  // 降级
)

// ========== 工作负载迁移 ==========

// MigrationStatus 迁移状态.
type MigrationStatus string

const (
	MigrationPending   MigrationStatus = "pending"
	MigrationRunning   MigrationStatus = "running"
	MigrationCompleted MigrationStatus = "completed"
	MigrationFailed    MigrationStatus = "failed"
	MigrationCancelled MigrationStatus = "cancelled"
)

// ========== QoS 规则 ==========

// QoSCategory QoS 类别.
type QoSCategory string

const (
	QoSCPU         QoSCategory = "cpu"         // CPU 限制
	QoSMemory      QoSCategory = "memory"      // 内存限制
	QoSNetwork     QoSCategory = "network"     // 网络带宽
	QoSStorage     QoSCategory = "storage"     // 存储 IOPS
	QoSIOPS         QoSCategory = "iops"        // IOPS 限制
	QoSConnections QoSCategory = "connections"  // 连接数
)

// QoSAction QoS 动作.
type QoSAction string

const (
	QoSActionThrottle  QoSAction = "throttle"  // 限流
	QoSActionReject     QoSAction = "reject"    // 拒绝
	QoSActionQueue      QoSAction = "queue"     // 排队
	QoSActionMigrate   QoSAction = "migrate"    // 迁移工作负载
)

// ========== 集中化保护 ==========

// ProtectionType 保护类型.
type ProtectionType string

const (
	ProtectionFailover     ProtectionType = "failover"      // 故障切换
	ProtectionReplication  ProtectionType = "replication"    // 证明数据复制
	ProtectionSnapshot     ProtectionType = "snapshot"       // 快照保护
	ProtectionBackup       ProtectionType = "backup"         // 备份保护
	ProtectionQuorum       ProtectionType = "quorum"         // 仲裁保护
)

// ProtectionLevel 保护级别.
type ProtectionLevel string

const (
	ProtectionFull    ProtectionLevel = "full"     // 完全保护
	ProtectionPartial ProtectionLevel = "partial"  // 部分保护
	ProtectionNone    ProtectionLevel = "none"     // 无保护
)

// ========== 核心类型 ==========

// ClusterNode 集群节点信息.
type ClusterNode struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Role          NodeRole       `json:"role"`
	Status        NodeStatus     `json:"status"`
	Address       string         `json:"address"`
	Port          int            `json:"port"`
	Model         string         `json:"model,omitempty"`
	Serial        string         `json:"serial,omitempty"`
	CPUCores      int            `json:"cpuCores"`
	MemoryBytes   int64          `json:"memoryBytes"`
	StorageBytes  int64          `json:"storageBytes"`
	UsedStorage   int64          `json:"usedStorage"`
	WorkloadCount int            `json:"workloadCount"`
	JoinedAt      time.Time      `json:"joinedAt"`
	LastHeartbeat time.Time      `json:"lastHeartbeat"`
	Version       string         `json:"version,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
}

// Cluster 集群信息.
type Cluster struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Status          ClusterStatus  `json:"status"`
	LeaderID        string         `json:"leaderId"`
	Nodes           []ClusterNode  `json:"nodes"`
	NodeCount       int            `json:"nodeCount"`
	HealthyNodes    int            `json:"healthyNodes"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	FaultTolerance  int            `json:"faultTolerance"` // 可容忍故障节点数
}

// WorkloadMigration 工作负载迁移任务.
type WorkloadMigration struct {
	ID            string           `json:"id"`
	SourceNodeID  string           `json:"sourceNodeId"`
	TargetNodeID  string           `json:"targetNodeId"`
	WorkloadID    string           `json:"workloadId"`
	WorkloadName  string           `json:"workloadName"`
	Status        MigrationStatus  `json:"status"`
	Progress      float64          `json:"progress"` // 0-100
	Reason        string           `json:"reason,omitempty"`
	StartedAt     time.Time        `json:"startedAt,omitempty"`
	FinishedAt    time.Time        `json:"finishedAt,omitempty"`
	Error         string           `json:"error,omitempty"`
}

// QoSRule QoS 规则.
type QoSRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Category    QoSCategory  `json:"category"`
	NodeID      string       `json:"nodeId,omitempty"`   // 空表示全局
	WorkloadID  string       `json:"workloadId,omitempty"`
	Limit       int64        `json:"limit"`             // 限制值（单位取决于类别）
	Burst       int64        `json:"burst,omitempty"`   // 突发值
	Action      QoSAction    `json:"action"`
	Priority    int          `json:"priority"`         // 1-100，越高越优先
	Enabled     bool         `json:"enabled"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// CentralizedProtection 集中化保护策略.
type CentralizedProtection struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Type            ProtectionType   `json:"type"`
	Level           ProtectionLevel  `json:"level"`
	NodeIDs         []string         `json:"nodeIds"`          // 受保护节点
	WorkloadIDs     []string         `json:"workloadIds,omitempty"`
	AutoFailover    bool             `json:"autoFailover"`
	MaxFailoverTime int              `json:"maxFailoverTimeSec,omitempty"` // 最大故障切换时间（秒）
	ReplicaCount    int              `json:"replicaCount,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	Enabled         bool             `json:"enabled"`
}

// NodeHealth 节点健康详情.
type NodeHealth struct {
	NodeID          string         `json:"nodeId"`
	Status          NodeStatus     `json:"status"`
	CPUUsage        float64        `json:"cpuUsage"`        // 0-100
	MemoryUsage     float64        `json:"memoryUsage"`     // 0-100
	DiskUsage       float64        `json:"diskUsage"`       // 0-100
	NetworkThroughput float64      `json:"networkThroughput"` // MB/s
	Temperature     float64        `json:"temperature"`     // °C
	Uptime          int64          `json:"uptime"`          // 秒
	LoadAvg         [3]float64     `json:"loadAvg"`         // 1/5/15 分钟负载
	Errors          []string       `json:"errors,omitempty"`
	CheckedAt       time.Time      `json:"checkedAt"`
}

// ========== 请求/响应类型 ==========

// CreateClusterRequest 创建集群请求.
type CreateClusterRequest struct {
	Name           string `json:"name" binding:"required"`
	LeaderNodeName string `json:"leaderNodeName" binding:"required"`
	LeaderAddress  string `json:"leaderAddress" binding:"required"`
	LeaderPort     int    `json:"leaderPort" binding:"omitempty,min=1,max=65535"`
}

// AddNodeRequest 添加节点请求.
type AddNodeRequest struct {
	ClusterID string `json:"clusterId" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Address   string `json:"address" binding:"required"`
	Port      int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Role      NodeRole `json:"role" binding:"required"`
}

// RemoveNodeRequest 移除节点请求.
type RemoveNodeRequest struct {
	ClusterID string `json:"clusterId" binding:"required"`
	NodeID     string `json:"nodeId" binding:"required"`
	Force      bool   `json:"force"`
	MigrateWorkloads bool `json:"migrateWorkloads"`
}

// MigrateWorkloadRequest 工作负载迁移请求.
type MigrateWorkloadRequest struct {
	ClusterID    string `json:"clusterId" binding:"required"`
	WorkloadID   string `json:"workloadId" binding:"required"`
	TargetNodeID string `json:"targetNodeId" binding:"required"`
	Reason       string `json:"reason,omitempty"`
}

// CreateQoSRuleRequest 创建 QoS 规则请求.
type CreateQoSRuleRequest struct {
	ClusterID  string      `json:"clusterId" binding:"required"`
	Name       string      `json:"name" binding:"required"`
	Category   QoSCategory `json:"category" binding:"required"`
	NodeID     string      `json:"nodeId,omitempty"`
	WorkloadID string      `json:"workloadId,omitempty"`
	Limit      int64       `json:"limit" binding:"required"`
	Burst      int64       `json:"burst,omitempty"`
	Action     QoSAction   `json:"action" binding:"required"`
	Priority   int         `json:"priority" binding:"omitempty,min=1,max=100"`
}

// CreateProtectionRequest 创建保护策略请求.
type CreateProtectionRequest struct {
	ClusterID       string          `json:"clusterId" binding:"required"`
	Name            string          `json:"name" binding:"required"`
	Type            ProtectionType  `json:"type" binding:"required"`
	Level           ProtectionLevel `json:"level" binding:"required"`
	NodeIDs         []string        `json:"nodeIds" binding:"required,min=1"`
	WorkloadIDs     []string        `json:"workloadIds,omitempty"`
	AutoFailover    bool            `json:"autoFailover"`
	MaxFailoverTime int             `json:"maxFailoverTimeSec,omitempty"`
	ReplicaCount    int             `json:"replicaCount,omitempty"`
}

// ClusterResponse 集群操作响应.
type ClusterResponse struct {
	ClusterID string  `json:"clusterId"`
	Success   bool    `json:"success"`
	Message   string  `json:"message"`
}

// NodeListResponse 节点列表响应.
type NodeListResponse struct {
	ClusterID string        `json:"clusterId"`
	Nodes     []ClusterNode `json:"nodes"`
}

// MigrationResponse 迁移响应.
type MigrationResponse struct {
	MigrationID string  `json:"migrationId"`
	Status      MigrationStatus `json:"status"`
	Progress    float64 `json:"progress"`
	Message     string  `json:"message,omitempty"`
}

// HealthResponse 健康状态响应.
type HealthResponse struct {
	ClusterID string       `json:"clusterId"`
	Nodes     []NodeHealth `json:"nodes"`
	OverallStatus ClusterStatus `json:"overallStatus"`
}

// ========== 内部模型 ==========

// clusterState 内部集群状态.
type clusterState struct {
	id              string
	name            string
	status          ClusterStatus
	leaderID        string
	nodes           map[string]*ClusterNode
	migrations      map[string]*WorkloadMigration
	qosRules        map[string]*QoSRule
	protections     map[string]*CentralizedProtection
	healthRecords   map[string]*NodeHealth
	createdAt       time.Time
	updatedAt       time.Time
	faultTolerance  int
}