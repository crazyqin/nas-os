// Package containerfailover 容器 HA 故障转移模块
// 受 TrueNAS 26 容器 HA 功能启发，实现容器在 HA 集群节点间的故障转移。
// 支持 active-passive / active-active 两种模式、静态 IP 迁移、ARP 广播、
// 基于 etcd 模拟后端的状态同步，以及有状态 SMB HA 故障转移。
package containerfailover

import (
	"time"
)

// ========== 容器相关数据模型 ==========

// ContainerStatus 容器运行状态.
type ContainerStatus string

const (
	// ContainerRunning 运行中.
	ContainerRunning ContainerStatus = "running"
	// ContainerStopped 已停止.
	ContainerStopped ContainerStatus = "stopped"
	// ContainerFailed 故障.
	ContainerFailed ContainerStatus = "failed"
	// ContainerFailingOver 故障转移中.
	ContainerFailingOver ContainerStatus = "failing-over"
	// ContainerPending 等待中.
	ContainerPending ContainerStatus = "pending"
)

// Container 容器信息.
type Container struct {
	// ID 容器唯一标识
	ID string `json:"id"`
	// Name 容器名称
	Name string `json:"name"`
	// Image 镜像地址
	Image string `json:"image"`
	// IP 容器绑定的静态 IP（HA 模式下随容器迁移）
	IP string `json:"ip,omitempty"`
	// Status 当前状态
	Status ContainerStatus `json:"status"`
	// Node 当前所在节点 ID
	Node string `json:"node"`
	// PreferredNode 期望运行节点（active-passive 模式下使用）
	PreferredNode string `json:"preferred_node,omitempty"`
	// Ports 端口映射列表
	Ports []PortMapping `json:"ports,omitempty"`
	// Volumes 数据卷挂载列表
	Volumes []VolumeMount `json:"volumes,omitempty"`
	// Labels 自定义标签
	Labels map[string]string `json:"labels,omitempty"`
	//CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// PortMapping 端口映射.
type PortMapping struct {
	// HostPort 宿主机端口
	HostPort int `json:"host_port"`
	// ContainerPort 容器端口
	ContainerPort int `json:"container_port"`
	// Protocol 协议（tcp/udp）
	Protocol string `json:"protocol"`
}

// VolumeMount 数据卷挂载.
type VolumeMount struct {
	// HostPath 宿主机路径
	HostPath string `json:"host_path"`
	// ContainerPath 容器内路径
	ContainerPath string `json:"container_path"`
	// ReadOnly 是否只读
	ReadOnly bool `json:"read_only"`
}

// ========== 故障转移策略 ==========

// FailoverMode 故障转移模式.
type FailoverMode string

const (
	// ModeActivePassive 主备模式：同一时刻仅一个节点运行容器.
	ModeActivePassive FailoverMode = "active-passive"
	// ModeActiveActive 双活模式：容器可在多个节点同时运行，IP 随迁移.
	ModeActiveActive FailoverMode = "active-active"
)

// FailoverPolicy 故障转移策略.
type FailoverPolicy struct {
	// Mode 故障转移模式
	Mode FailoverMode `json:"mode"`
	// HealthCheckInterval 健康检查间隔（秒）
	HealthCheckInterval int `json:"health_check_interval"`
	// FailoverDelay 故障转移延迟（秒），避免脑裂
	FailoverDelay int `json:"failover_delay"`
	// MaxRetryAttempts 最大重试次数
	MaxRetryAttempts int `json:"max_retry_attempts"`
	// HealthCheckTimeout 单次健康检查超时（秒）
	HealthCheckTimeout int `json:"health_check_timeout"`
	// AutoFailover 是否自动故障转移
	AutoFailover bool `json:"auto_failover"`
	// SMBHA 是否启用有状态 SMB HA 故障转移
	SMBHA bool `json:"smb_ha"`
}

// DefaultFailoverPolicy 默认故障转移策略.
func DefaultFailoverPolicy() *FailoverPolicy {
	return &FailoverPolicy{
		Mode:                ModeActivePassive,
		HealthCheckInterval: 5,
		FailoverDelay:       3,
		MaxRetryAttempts:    3,
		HealthCheckTimeout:  5,
		AutoFailover:        true,
		SMBHA:               false,
	}
}

// ========== 故障转移事件 ==========

// FailoverTrigger 触发方式.
type FailoverTrigger string

const (
	// TriggerAuto 自动触发.
	TriggerAuto FailoverTrigger = "auto"
	// TriggerManual 手动触发.
	TriggerManual FailoverTrigger = "manual"
	// TriggerHealthCheck 健康检查触发.
	TriggerHealthCheck FailoverTrigger = "health-check"
)

// FailoverEvent 故障转移事件记录.
type FailoverEvent struct {
	// ID 事件唯一标识
	ID string `json:"id"`
	// ContainerID 容器 ID
	ContainerID string `json:"container_id"`
	// ContainerName 容器名称
	ContainerName string `json:"container_name"`
	// TriggeredAt 触发时间
	TriggeredAt time.Time `json:"triggered_at"`
	// CompletedAt 完成时间
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// Trigger 触发方式
	Trigger FailoverTrigger `json:"trigger"`
	// FromNode 源节点 ID
	FromNode string `json:"from_node"`
	// ToNode 目标节点 ID
	ToNode string `json:"to_node"`
	// Reason 切换原因
	Reason string `json:"reason"`
	// Success 是否成功
	Success bool `json:"success"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
	// Duration 耗时（毫秒）
	Duration int64 `json:"duration"`
	// IPMigrated 是否迁移了 IP
	IPMigrated bool `json:"ip_migrated"`
	// SMBFailover 是否触发了 SMB HA 故障转移
	SMBFailover bool `json:"smb_failover,omitempty"`
}

// ========== 集群节点 ==========

// NodeStatus 节点状态.
type NodeStatus string

const (
	// NodeOnline 在线.
	NodeOnline NodeStatus = "online"
	// NodeOffline 离线.
	NodeOffline NodeStatus = "offline"
	// NodeDegraded 降级.
	NodeDegraded NodeStatus = "degraded"
)

// ClusterNode HA 集群节点.
type ClusterNode struct {
	// ID 节点唯一标识
	ID string `json:"id"`
	// Name 节点名称
	Name string `json:"name"`
	// IP 管理 IP
	IP string `json:"ip"`
	// Status 节点状态
	Status NodeStatus `json:"status"`
	// Containers 该节点上运行的容器 ID 列表
	Containers []string `json:"containers"`
	// LastHeartbeat 上次心跳时间
	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// ========== IP 分配记录 ==========

// IPAllocation IP 分配记录.
type IPAllocation struct {
	// IP 静态 IP 地址
	IP string `json:"ip"`
	// ContainerID 绑定的容器 ID
	ContainerID string `json:"container_id"`
	// Node 当前持有该 IP 的节点 ID
	Node string `json:"node"`
	// Interface 网络接口名
	Interface string `json:"interface"`
	// AllocatedAt 分配时间
	AllocatedAt time.Time `json:"allocated_at"`
	// Active 是否处于活跃状态
	Active bool `json:"active"`
}
