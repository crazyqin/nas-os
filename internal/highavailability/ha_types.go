// Package highavailability 提供高可用集群管理功能，支持 Active/Passive 模式、
// 健康检查心跳、自动故障检测与切换、VIP 漂移管理、资源锁定防脑裂等。
package highavailability

import (
	"sync"
	"time"
)

// HAMode 高可用集群模式
type HAMode string

const (
	// ModeActivePassive 主备模式，一个活跃节点一个备用节点
	ModeActivePassive HAMode = "active_passive"
	// ModeActiveActive 双活模式，两个节点同时提供服务
	ModeActiveActive HAMode = "active_active"
)

// NodeRole 节点角色
type NodeRole string

const (
	// RoleActive 活跃节点
	RoleActive NodeRole = "active"
	// RoleStandby 备用节点
	RoleStandby NodeRole = "standby"
	// RoleUnknown 未知状态
	RoleUnknown NodeRole = "unknown"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	// StatusOnline 在线
	StatusOnline NodeStatus = "online"
	// StatusOffline 离线
	StatusOffline NodeStatus = "offline"
	// StatusDegraded 降级（部分功能不可用）
	StatusDegraded NodeStatus = "degraded"
	// StatusFailed 故障
	StatusFailed NodeStatus = "failed"
)

// ClusterNode 集群节点
type ClusterNode struct {
	// ID 节点唯一标识
	ID string `json:"id"`
	// Address 节点网络地址 (host:port)
	Address string `json:"address"`
	// Role 节点角色
	Role NodeRole `json:"role"`
	// Status 节点状态
	Status NodeStatus `json:"status"`
	// LastHeartbeat 最后心跳时间
	LastHeartbeat time.Time `json:"last_heartbeat"`
	// Metadata 节点元数据
	Metadata map[string]string `json:"metadata,omitempty"`
}

// HAConfig 高可用配置
type HAConfig struct {
	// Mode 集群模式
	Mode HAMode `json:"mode"`
	// HeartbeatInterval 心跳间隔
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	// HeartbeatTimeout 心跳超时（判定节点故障的时间）
	HeartbeatTimeout time.Duration `json:"heartbeat_timeout"`
	// VIP 虚拟 IP 地址
	VIP string `json:"vip"`
	// VIPInterface VIP 绑定的网络接口
	VIPInterface string `json:"vip_interface"`
	// LockResource 锁定资源名称（防脑裂）
	LockResource string `json:"lock_resource"`
	// LockTTL 锁定超时时间
	LockTTL time.Duration `json:"lock_ttl"`
	// FailoverDelay 故障切换延迟（防抖动）
	FailoverDelay time.Duration `json:"failover_delay"`
	// MaxRetries 最大重试次数
	MaxRetries int `json:"max_retries"`
}

// DefaultHAConfig 返回默认高可用配置
func DefaultHAConfig() *HAConfig {
	return &HAConfig{
		Mode:              ModeActivePassive,
		HeartbeatInterval: 5 * time.Second,
		HeartbeatTimeout:  15 * time.Second,
		VIP:               "",
		VIPInterface:      "eth0",
		LockResource:      "ha_leader",
		LockTTL:           30 * time.Second,
		FailoverDelay:     10 * time.Second,
		MaxRetries:        3,
	}
}

// FailoverEvent 故障切换事件
type FailoverEvent struct {
	// ID 事件唯一标识
	ID string `json:"id"`
	// Timestamp 事件发生时间
	Timestamp time.Time `json:"timestamp"`
	// Type 事件类型
	Type FailoverEventType `json:"type"`
	// FailedNodeID 故障节点 ID
	FailedNodeID string `json:"failed_node_id"`
	// PromotedNodeID 提升为活跃节点的 ID
	PromotedNodeID string `json:"promoted_node_id"`
	// Reason 事件原因
	Reason string `json:"reason"`
	// Duration 切换耗时
	Duration time.Duration `json:"duration"`
}

// FailoverEventType 故障切换事件类型
type FailoverEventType string

const (
	// EventFailover 故障切换
	EventFailover FailoverEventType = "failover"
	// EventFailback 故障恢复（原主节点恢复）
	EventFailback FailoverEventType = "failback"
	// EventManualSwitchover 手动切换
	EventManualSwitchover FailoverEventType = "manual_switchover"
	// EventNodeJoined 节点加入
	EventNodeJoined FailoverEventType = "node_joined"
	// EventNodeLeft 节点离开
	EventNodeLeft FailoverEventType = "node_left"
)

// LockInfo 资源锁信息
type LockInfo struct {
	// HolderID 持有锁的节点 ID
	HolderID string `json:"holder_id"`
	// AcquiredAt 获取锁的时间
	AcquiredAt time.Time `json:"acquired_at"`
	// ExpiresAt 锁过期时间
	ExpiresAt time.Time `json:"expires_at"`
	// Version 锁版本号
	Version int64 `json:"version"`
}

// clusterState 集群内部状态（非导出）
type clusterState struct {
	mu         sync.RWMutex
	nodes      map[string]*ClusterNode
	localNode  *ClusterNode
	lock       *LockInfo
	events     []FailoverEvent
	lastLeader string
}
