// Package lxcha 实现 LXC HA 故障转移模块
// 对标 TrueNAS 26 的 LXC HA Failover 支持
// 提供 LXC 容器状态监控、HA 节点间容器迁移、故障检测与自动故障转移能力
package lxcha

import (
	"time"
)

// ========== 常量定义 ==========

// ContainerState LXC 容器运行状态.
type ContainerState string

const (
	StateRunning   ContainerState = "running"   // 运行中
	StateStopped   ContainerState = "stopped"   // 已停止
	StateFrozen    ContainerState = "frozen"    // 已冻结
	StateMigrating ContainerState = "migrating" // 迁移中
	StateError     ContainerState = "error"     // 错误
	StateUnknown   ContainerState = "unknown"   // 未知
)

// FailoverPolicyType 故障转移策略类型.
type FailoverPolicyType string

const (
	PolicyAuto   FailoverPolicyType = "auto"   // 自动故障转移
	PolicyManual FailoverPolicyType = "manual" // 仅手动故障转移
	PPolicyNone  FailoverPolicyType = "none"   // 禁用故障转移
)

// FailoverStateType 故障转移状态.
type FailoverStateType string

const (
	FStateHealthy  FailoverStateType = "healthy"  // 健康
	FStateDegraded FailoverStateType = "degraded" // 降级
	FStateFailed   FailoverStateType = "failed"   // 失败
	FStateFailover FailoverStateType = "failover" // 故障转移中
	FStateRecovery FailoverStateType = "recovery" // 恢复中
)

// NodeType HA 集群中的节点角色.
type NodeType string

const (
	NodeRolePrimary NodeType = "primary" // 主节点
	NodeRoleBackup  NodeType = "backup"  // 备份节点
	NodeRoleWitness NodeType = "witness" // 见证节点
)

// NodeState HA 节点状态.
type NodeState string

const (
	NodeStateOnline  NodeState = "online"  // 在线
	NodeStateOffline NodeState = "offline" // 离线
	NodeStateStandby NodeState = "standby" // 待机
)

// ========== IP 管理类型 ==========

// StaticIPConfig 静态 IP 配置.
type StaticIPConfig struct {
	Interface string   `json:"interface"`         // 网络接口名 (eth0)
	Address   string   `json:"address"`           // IP 地址 (192.168.1.100/24)
	Gateway   string   `json:"gateway,omitempty"` // 网关
	DNS       []string `json:"dns,omitempty"`     // DNS 服务器列表
	VLAN      int      `json:"vlan,omitempty"`    // VLAN ID (0=无)
}

// IPReservation IP 预留记录，确保迁移后 IP 不会冲突.
type IPReservation struct {
	IP          string    `json:"ip"`          // 预留的 IP 地址
	ContainerID string    `json:"containerId"` // 所属容器
	NodeID      string    `json:"nodeId"`      // 当前持有该 IP 的节点
	ReservedAt  time.Time `json:"reservedAt"`  // 预留时间
}

// ========== 容器与节点类型 ==========

// LXCContainer LXC 容器信息.
type LXCContainer struct {
	ID           string             `json:"id"`                     // 容器 ID (lxc-100)
	Name         string             `json:"name"`                   // 容器名称
	State        ContainerState     `json:"state"`                  // 运行状态
	NodeID       string             `json:"nodeId"`                 // 所在 HA 节点 ID
	TargetNodeID string             `json:"targetNodeId,omitempty"` // 迁移目标节点（迁移中）
	IPConfigs    []*StaticIPConfig  `json:"ipConfigs,omitempty"`    // 静态 IP 配置列表
	HAEnabled    bool               `json:"haEnabled"`              // 是否启用 HA
	Policy       FailoverPolicyType `json:"policy"`                 // 故障转移策略
	Priority     int                `json:"priority"`               // 故障转移优先级 (0=最高)
	CPU          int                `json:"cpu"`                    // CPU 核数限制
	Memory       int64              `json:"memory"`                 // 内存限制 (MB)
	AutoStart    bool               `json:"autoStart"`              // 是否自动启动
	Uptime       int64              `json:"uptime"`                 // 运行时长 (秒)
	CreatedAt    time.Time          `json:"createdAt"`              // 创建时间
	UpdatedAt    time.Time          `json:"updatedAt"`              // 最后更新时间
	Error        string             `json:"error,omitempty"`        // 错误信息
}

// HANode HA 集群节点.
type HANode struct {
	ID         string    `json:"id"`         // 节点 ID
	Name       string    `json:"name"`       // 节点名称
	Role       NodeType  `json:"role"`       // 节点角色 (primary/backup/witness)
	State      NodeState `json:"state"`      // 节点状态
	Address    string    `json:"address"`    // 集群通信地址
	Port       int       `json:"port"`       // 集群通信端口
	Containers int       `json:"containers"` // 当前运行容器数
	CPUUsage   float64   `json:"cpuUsage"`   // CPU 使用率 (0-100)
	MemUsage   float64   `json:"memUsage"`   // 内存使用率 (0-100)
	LastSeen   time.Time `json:"lastSeen"`   // 最后心跳时间
}

// ========== 故障转移配置 ==========

// FailoverPolicy 故障转移策略.
type FailoverPolicy struct {
	ContainerID    string             `json:"containerId"`    // 容器 ID
	Type           FailoverPolicyType `json:"type"`           // 策略类型
	PreferredNode  string             `json:"preferredNode"`  // 优先迁移到的节点
	MaxRetries     int                `json:"maxRetries"`     // 最大重试次数
	HealthCheckInt int                `json:"healthCheckInt"` // 健康检查间隔 (秒)
	FailoverDelay  int                `json:"failoverDelay"`  // 故障转移延迟 (秒)
}

// FailoverState 故障转移全局状态.
type FailoverState struct {
	ContainerID string            `json:"containerId"`           // 容器 ID
	State       FailoverStateType `json:"state"`                 // 当前状态
	SourceNode  string            `json:"sourceNode"`            // 源节点
	TargetNode  string            `json:"targetNode"`            // 目标节点
	StartedAt   time.Time         `json:"startedAt"`             // 故障转移开始时间
	CompletedAt time.Time         `json:"completedAt,omitempty"` // 完成时间
	RetryCount  int               `json:"retryCount"`            // 重试次数
	Error       string            `json:"error,omitempty"`       // 错误信息
	CheckPoint  string            `json:"checkpoint,omitempty"`  // 恢复检查点
}

// FailoverEvent 故障转移事件记录.
type FailoverEvent struct {
	ID          string            `json:"id"`              // 事件 ID
	ContainerID string            `json:"containerId"`     // 容器 ID
	SourceNode  string            `json:"sourceNode"`      // 源节点
	TargetNode  string            `json:"targetNode"`      // 目标节点
	Reason      string            `json:"reason"`          // 故障转移原因
	Success     bool              `json:"success"`         // 是否成功
	EndState    FailoverStateType `json:"endState"`        // 最终状态
	Timestamp   time.Time         `json:"timestamp"`       // 事件时间
	Error       string            `json:"error,omitempty"` // 错误信息
}

// ContainerMigrateConfig 容器迁移配置.
type ContainerMigrateConfig struct {
	ContainerID   string `json:"containerId"`   // 容器 ID
	SourceNode    string `json:"sourceNode"`    // 源节点
	TargetNode    string `json:"targetNode"`    // 目标节点
	Online        bool   `json:"online"`        // 是否在线迁移
	Timeout       int    `json:"timeout"`       // 超时时间 (秒)
	KeepIP        bool   `json:"keepIP"`        // 迁移后是否保持原 IP
	RestartPolicy string `json:"restartPolicy"` // 重启策略 (always/on-failure/no)
}

// MigrateResult 迁移操作结果.
type MigrateResult struct {
	ContainerID string    `json:"containerId"`     // 容器 ID
	Success     bool      `json:"success"`         // 是否成功
	SourceNode  string    `json:"sourceNode"`      // 源节点
	TargetNode  string    `json:"targetNode"`      // 目标节点
	Duration    float64   `json:"duration"`        // 耗时 (秒)
	Error       string    `json:"error,omitempty"` // 错误信息
	CompletedAt time.Time `json:"completedAt"`     // 完成时间
}

// FailoverHistoryEntry 故障转移历史记录.
type FailoverHistoryEntry struct {
	ID          string            `json:"id"`                   // 记录 ID
	ContainerID string            `json:"containerId"`          // 容器 ID
	SourceNode  string            `json:"sourceNode"`           // 源节点
	TargetNode  string            `json:"targetNode"`           // 目标节点
	Reason      string            `json:"reason"`               // 原因描述
	Success     bool              `json:"success"`              // 是否成功
	State       FailoverStateType `json:"state"`                // 最终状态
	StartedAt   time.Time         `json:"startedAt"`            // 开始时间
	FinishedAt  time.Time         `json:"finishedAt,omitempty"` // 结束时间
	Error       string            `json:"error,omitempty"`      // 错误信息
}

// ========== 请求/响应类型 ==========

// RegisterContainerRequest 注册容器到 HA 管理请求.
type RegisterContainerRequest struct {
	ContainerID string             `json:"containerId" binding:"required"` // 容器 ID
	Policy      FailoverPolicyType `json:"policy"`                         // 故障转移策略
	Priority    int                `json:"priority"`                       // 优先级
	IPConfigs   []*StaticIPConfig  `json:"ipConfigs,omitempty"`            // 静态 IP 配置
}

// UpdatePolicyRequest 更新故障转移策略请求.
type UpdatePolicyRequest struct {
	ContainerID    string             `json:"containerId" binding:"required"`
	Type           FailoverPolicyType `json:"type"`
	PreferredNode  string             `json:"preferredNode"`
	MaxRetries     int                `json:"maxRetries"`
	HealthCheckInt int                `json:"healthCheckInt"`
	FailoverDelay  int                `json:"failoverDelay"`
}

// TriggerFailoverRequest 手动触发故障转移请求.
type TriggerFailoverRequest struct {
	ContainerID string `json:"containerId" binding:"required"`
	TargetNode  string `json:"targetNode"`
	Force       bool   `json:"force"`
}

// MigrateRequest 容器迁移请求.
type MigrateRequest struct {
	ContainerID string `json:"containerId" binding:"required"`
	TargetNode  string `json:"targetNode" binding:"required"`
	Online      bool   `json:"online"`
	Timeout     int    `json:"timeout"`
	KeepIP      bool   `json:"keepIP"`
}

// ReserveIPRequest IP 预留请求.
type ReserveIPRequest struct {
	IP          string `json:"ip" binding:"required"`
	ContainerID string `json:"containerId" binding:"required"`
	NodeID      string `json:"nodeId" binding:"required"`
}

// ========== API 响应 ==========

// APIResponse 统一 API 响应格式.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ========== 模块配置 ==========

// Config 模块配置.
type Config struct {
	ClusterName        string // 集群名称
	HealthCheckSeconds int    // 健康检查间隔（秒）
	HANodesDir         string // HA 节点配置目录
	ContainerBase      string // LXC 容器配置根目录
	NodeID             string // 当前节点 ID
}

// DefaultConfig 默认配置.
func DefaultConfig() *Config {
	return &Config{
		ClusterName:        "default",
		HealthCheckSeconds: 10,
		HANodesDir:         "/etc/nas-os/ha",
		ContainerBase:      "/var/lib/lxc",
		NodeID:             "node-1",
	}
}

// HAStatus HA 集群状态总览.
type HAStatus struct {
	TotalNodes      int                     `json:"totalNodes"`      // 节点总数
	OnlineNodes     int                     `json:"onlineNodes"`     // 在线节点数
	TotalContainers int                     `json:"totalContainers"` // 容器总数
	HAContainers    int                     `json:"haContainers"`    // HA 容器数
	FailoverEvents  int                     `json:"failoverEvents"`  // 故障转移事件数
	ActiveFailovers int                     `json:"activeFailovers"` // 进行中的故障转移
	IPReservations  int                     `json:"ipReservations"`  // IP 预留数
	History         []*FailoverHistoryEntry `json:"history"`         // 历史记录
}
