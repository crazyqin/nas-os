// Package hafailover 高可用故障转移模块
package hafailover

import (
	"sync"
	"time"
)

// NodeRole 节点角色
type NodeRole string

const (
	// RoleActive 活动节点
	RoleActive NodeRole = "active"
	// RoleStandby 备用节点
	RoleStandby NodeRole = "standby"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	// StatusOnline 在线
	StatusOnline NodeStatus = "online"
	// StatusOffline 离线
	StatusOffline NodeStatus = "offline"
	// StatusDegraded 降级（部分服务异常）
	StatusDegraded NodeStatus = "degraded"
	// StatusSyncing 同步中
	StatusSyncing NodeStatus = "syncing"
	// StatusFailed 故障
	StatusFailed NodeStatus = "failed"
)

// HeartbeatLevel 心跳级别
type HeartbeatLevel string

const (
	// HeartbeatNetwork 网络心跳
	HeartbeatNetwork HeartbeatLevel = "network"
	// HeartbeatStorage 存储心跳
	HeartbeatStorage HeartbeatLevel = "storage"
	// HeartbeatService 服务心跳
	HeartbeatService HeartbeatLevel = "service"
)

// SyncState 同步状态
type SyncState string

const (
	// SyncStateIdle 空闲
	SyncStateIdle SyncState = "idle"
	// SyncStateSyncing 同步中
	SyncStateSyncing SyncState = "syncing"
	// SyncStateCompleted 同步完成
	SyncStateCompleted SyncState = "completed"
	// SyncStateFailed 同步失败
	SyncStateFailed SyncState = "failed"
)

// FailoverTrigger 触发方式
type FailoverTrigger string

const (
	// TriggerAuto 自动触发
	TriggerAuto FailoverTrigger = "auto"
	// TriggerManual 手动触发
	TriggerManual FailoverTrigger = "manual"
)

// FailoverEvent 切换事件记录
type FailoverEvent struct {
	// ID 事件唯一标识
	ID string `json:"id"`
	// TriggeredAt 触发时间
	TriggeredAt time.Time `json:"triggered_at"`
	// CompletedAt 完成时间
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// Trigger 触发方式
	Trigger FailoverTrigger `json:"trigger"`
	// FromNode 源节点ID
	FromNode string `json:"from_node"`
	// ToNode 目标节点ID
	ToNode string `json:"to_node"`
	// Reason 切换原因
	Reason string `json:"reason"`
	// Success 是否成功
	Success bool `json:"success"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
	// Duration 耗时（毫秒）
	Duration int64 `json:"duration"`
}

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	// Interval 心跳间隔（秒）
	Interval int `json:"interval"`
	// Timeout 超时时间（秒）
	Timeout int `json:"timeout"`
	// MaxRetries 最大重试次数
	MaxRetries int `json:"max_retries"`
}

// VIPConfig 虚拟IP配置
type VIPConfig struct {
	// Enabled 是否启用VIP漂移
	Enabled bool `json:"enabled"`
	// VIP 虚拟IP地址
	VIP string `json:"vip"`
	// Interface 网络接口
	Interface string `json:"interface"`
	// Netmask 子网掩码
	Netmask string `json:"netmask"`
}

// SyncConfig 数据同步配置
type SyncConfig struct {
	// StorageSync 存储配置同步
	StorageSync bool `json:"storage_sync"`
	// ServiceSync 服务状态同步
	ServiceSync bool `json:"service_sync"`
	// SyncInterval 同步间隔（秒）
	SyncInterval int `json:"sync_interval"`
	// SyncPaths 同步路径列表
	SyncPaths []string `json:"sync_paths,omitempty"`
}

// NodeInfo 节点信息
type NodeInfo struct {
	// ID 节点唯一标识
	ID string `json:"id"`
	// Name 节点名称
	Name string `json:"name"`
	// Hostname 主机名
	Hostname string `json:"hostname"`
	// IP 管理IP
	IP string `json:"ip"`
	// Role 节点角色
	Role NodeRole `json:"role"`
	// Status 节点状态
	Status NodeStatus `json:"status"`
	// LastHeartbeat 上次心跳时间
	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty"`
	// HeartbeatStatus 各级别心跳状态
	HeartbeatStatus map[HeartbeatLevel]bool `json:"heartbeat_status"`
	// Services 运行的服务列表
	Services []string `json:"services,omitempty"`
	// SystemInfo 系统信息
	SystemInfo SystemInfo `json:"system_info"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// SystemInfo 系统资源信息
type SystemInfo struct {
	// CPUPercent CPU使用率
	CPUPercent float64 `json:"cpu_percent"`
	// MemoryPercent 内存使用率
	MemoryPercent float64 `json:"memory_percent"`
	// DiskPercent 磁盘使用率
	DiskPercent float64 `json:"disk_percent"`
	// Uptime 运行时间（秒）
	Uptime int64 `json:"uptime"`
}

// HAConfig 高可用集群配置
type HAConfig struct {
	// ClusterName 集群名称
	ClusterName string `json:"cluster_name"`
	// LocalNodeID 本节点ID
	LocalNodeID string `json:"local_node_id"`
	// PeerNodeID 对端节点ID
	PeerNodeID string `json:"peer_node_id"`
	// AutoFailover 是否自动故障切换
	AutoFailover bool `json:"auto_failover"`
	// FailoverDelay 切换延迟（秒）
	FailoverDelay int `json:"failover_delay"`
	// Heartbeats 心跳配置（按级别）
	Heartbeats map[HeartbeatLevel]HeartbeatConfig `json:"heartbeats"`
	// VIP 虚拟IP配置
	VIP VIPConfig `json:"vip"`
	// Sync 数据同步配置
	Sync SyncConfig `json:"sync"`
}

// HAStatus HA集群整体状态
type HAStatus struct {
	// ClusterName 集群名称
	ClusterName string `json:"cluster_name"`
	// ActiveNode 当前活动节点
	ActiveNode *NodeInfo `json:"active_node"`
	// StandbyNode 当前备用节点
	StandbyNode *NodeInfo `json:"standby_node"`
	// VIPStatus VIP状态
	VIPStatus string `json:"vip_status"`
	// VIPIP VIP地址
	VIPIP string `json:"vip_ip"`
	// SyncState 同步状态
	SyncState SyncState `json:"sync_state"`
	// LastSyncAt 上次同步时间
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`
	// FailoverCount 切换次数
	FailoverCount int `json:"failover_count"`
	// LastFailover 上次切换事件
	LastFailover *FailoverEvent `json:"last_failover,omitempty"`
	// HealthScore 健康分数（0-100）
	HealthScore int `json:"health_score"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// SyncStatus 同步状态详情
type SyncStatus struct {
	// State 同步状态
	State SyncState `json:"state"`
	// Progress 同步进度（0-100）
	Progress int `json:"progress"`
	// LastSyncAt 上次同步时间
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`
	// LastSyncDuration 上次同步耗时（毫秒）
	LastSyncDuration int64 `json:"last_sync_duration"`
	// PendingChanges 待同步变更数
	PendingChanges int `json:"pending_changes"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
}

// FailoverRequest 手动切换请求
type FailoverRequest struct {
	// Reason 切换原因
	Reason string `json:"reason"`
	// Force 强制切换（忽略健康检查）
	Force bool `json:"force"`
}

// Manager HA故障转移管理器
type Manager struct {
	mu             sync.RWMutex
	config         *HAConfig
	localNode      *NodeInfo
	peerNode       *NodeInfo
	events         []FailoverEvent
	vipActive      bool
	syncState      SyncState
	lastSyncAt     *time.Time
	configPath     string
	heartbeatStop  map[HeartbeatLevel]chan struct{}
	failoverActive bool
}
