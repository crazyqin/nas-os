// Package nasclusterfed 提供NAS联邦集群管理功能
// v2.53.0 - 多NAS集群联邦管理
package nasclusterfed

import (
	"time"
)

// ClusterStatus 定义集群状态
type ClusterStatus string

const (
	// ClusterStatusOnline 集群在线
	ClusterStatusOnline ClusterStatus = "online"
	// ClusterStatusOffline 集群离线
	ClusterStatusOffline ClusterStatus = "offline"
	// ClusterStatusSyncing 同步中
	ClusterStatusSyncing ClusterStatus = "syncing"
	// ClusterStatusDegraded 降级状态
	ClusterStatusDegraded ClusterStatus = "degraded"
	// ClusterStatusError 错误状态
	ClusterStatusError ClusterStatus = "error"
)

// ClusterRole 定义集群角色
type ClusterRole string

const (
	// ClusterRoleLeader 主集群
	ClusterRoleLeader ClusterRole = "leader"
	// ClusterRoleFollower 从集群
	ClusterRoleFollower ClusterRole = "follower"
	// ClusterRoleWitness 见证集群
	ClusterRoleWitness ClusterRole = "witness"
)

// SyncMode 定义同步模式
type SyncMode string

const (
	// SyncModeFull 全量同步
	SyncModeFull SyncMode = "full"
	// SyncModeIncremental 增量同步
	SyncModeIncremental SyncMode = "incremental"
	// SyncModeRealtime 实时同步
	SyncModeRealtime SyncMode = "realtime"
)

// DiscoveryMethod 定义集群发现方式
type DiscoveryMethod string

const (
	// DiscoveryStatic 静态配置
	DiscoveryStatic DiscoveryMethod = "static"
	// DiscoveryMulticast 组播发现
	DiscoveryMulticast DiscoveryMethod = "multicast"
	// DiscoveryDNS DNS发现
	DiscoveryDNS DiscoveryMethod = "dns"
	// DiscoveryConsul Consul服务发现
	DiscoveryConsul DiscoveryMethod = "consul"
)

// ClusterNode 集群节点
type ClusterNode struct {
	ID          string            `json:"id"`
	Hostname    string            `json:"hostname"`
	IP          string            `json:"ip"`
	Port        int               `json:"port"`
	Role        ClusterRole       `json:"role"`
	Status      ClusterStatus     `json:"status"`
	Labels      map[string]string `json:"labels,omitempty"`
	CPUCores    int               `json:"cpuCores"`
	MemoryGB    int               `json:"memoryGB"`
	StorageTB   float64           `json:"storageTB"`
	LoadScore   float64           `json:"loadScore"`
	LastSeen    time.Time         `json:"lastSeen"`
	ConnectedAt time.Time         `json:"connectedAt"`
}

// Cluster 集群定义
type Cluster struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Region      string            `json:"region"`
	Nodes       []*ClusterNode    `json:"nodes"`
	Status      ClusterStatus     `json:"status"`
	Role        ClusterRole       `json:"role"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// SyncTask 同步任务
type SyncTask struct {
	ID              string     `json:"id"`
	SourceClusterID string     `json:"sourceClusterId"`
	TargetClusterID string     `json:"targetClusterId"`
	Mode            SyncMode   `json:"mode"`
	Status          string     `json:"status"`
	Progress        float64    `json:"progress"`
	BytesTotal      int64      `json:"bytesTotal"`
	BytesSynced     int64      `json:"bytesSynced"`
	FilesTotal      int        `json:"filesTotal"`
	FilesSynced     int        `json:"filesSynced"`
	StartedAt       time.Time  `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	Error           string     `json:"error,omitempty"`
}

// SyncPolicy 同步策略
type SyncPolicy struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Mode          SyncMode      `json:"mode"`
	Schedule      string        `json:"schedule,omitempty"`
	BandwidthMBps int           `json:"bandwidthMBps,omitempty"`
	Compression   bool          `json:"compression"`
	Encryption    bool          `json:"encryption"`
	RetryCount    int           `json:"retryCount"`
	RetryDelay    time.Duration `json:"retryDelay"`
	Enabled       bool          `json:"enabled"`
}

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	Strategy            string        `json:"strategy"`
	HealthCheckPath     string        `json:"healthCheckPath"`
	HealthCheckInterval time.Duration `json:"healthCheckInterval"`
	MaxRetries          int           `json:"maxRetries"`
	StickySession       bool          `json:"stickySession"`
	Weighted            bool          `json:"weighted"`
}

// ClusterMetrics 集群指标
type ClusterMetrics struct {
	ClusterID      string    `json:"clusterId"`
	CPUUsage       float64   `json:"cpuUsage"`
	MemoryUsage    float64   `json:"memoryUsage"`
	StorageUsage   float64   `json:"storageUsage"`
	NetworkInMbps  float64   `json:"networkInMbps"`
	NetworkOutMbps float64   `json:"networkOutMbps"`
	IOPS           int64     `json:"iops"`
	ActiveTasks    int       `json:"activeTasks"`
	HealthScore    float64   `json:"healthScore"`
	CPUCores       int       `json:"cpuCores"`
	MemoryGB       int       `json:"memoryGB"`
	StorageTB      float64   `json:"storageTB"`
	Timestamp      time.Time `json:"timestamp"`
}

// FederationEvent 联邦事件
type FederationEvent struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	ClusterID string            `json:"clusterId"`
	Message   string            `json:"message"`
	Severity  string            `json:"severity"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
