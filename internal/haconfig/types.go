package haconfig

import "time"

// HAState 高可用状态类型.
type HAState string

// 高可用状态枚举.
const (
	HAStateActive   HAState = "active"
	HAStateStandby  HAState = "standby"
	HAStateFailover HAState = "failover"
)

// EventType 事件类型.
const (
	EventTypeFailover     = "failover"
	EventTypeHeartbeat    = "heartbeat"
	EventTypeConfigChange = "config-change"
)

// NodeRole 节点角色.
const (
	NodeRolePrimary   = "primary"
	NodeRoleSecondary = "secondary"
)

// NodeStatus 节点连接状态.
const (
	NodeStatusOnline  = "online"
	NodeStatusOffline = "offline"
	NodeStatusUnknown = "unknown"
)

// HAStatusResponse 高可用状态响应.
type HAStatusResponse struct {
	State          HAState          `json:"state"`
	PrimaryNode    string           `json:"primary_node"`
	SecondaryNode  string           `json:"secondary_node"`
	LastHeartbeat  time.Time        `json:"last_heartbeat"`
	FailoverCount  int              `json:"failover_count"`
	Uptime         time.Duration    `json:"uptime"`
	HealthyNodes   int              `json:"healthy_nodes"`
	TotalNodes     int              `json:"total_nodes"`
	NodeStates     map[string]HANode `json:"node_states"`
}

// HANode 高可用节点信息.
type HANode struct {
	ID        string        `json:"id"`
	Address   string        `json:"address"`
	Role      string        `json:"role"`
	Status    string        `json:"status"`
	LastSeen  time.Time     `json:"last_seen"`
	Uptime    time.Duration `json:"uptime"`
	IsHealthy bool          `json:"is_healthy"`
}

// HAConfigResponse 高可用配置响应.
type HAConfigResponse struct {
	VirtualIP        string        `json:"virtual_ip"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	FailoverTimeout  time.Duration `json:"failover_timeout"`
	AutoFailback     bool          `json:"auto_failback"`
	Preempt          bool          `json:"preempt"`
	PeerNodes        []string      `json:"peer_nodes"`
}

// HAConfigUpdateRequest 更新高可用配置请求.
type HAConfigUpdateRequest struct {
	VirtualIP        *string        `json:"virtual_ip,omitempty"`
	HeartbeatInterval *time.Duration `json:"heartbeat_interval,omitempty"`
	FailoverTimeout  *time.Duration `json:"failover_timeout,omitempty"`
	AutoFailback     *bool          `json:"auto_failback,omitempty"`
	Preempt          *bool          `json:"preempt,omitempty"`
	PeerNodes        []string       `json:"peer_nodes,omitempty"`
}

// FailoverRequest 故障转移请求.
type FailoverRequest struct {
	TargetNode string `json:"target_node"`
	Reason     string `json:"reason"`
}

// FailoverResponse 故障转移响应.
type FailoverResponse struct {
	Success   bool   `json:"success"`
	NewPrimary string `json:"new_primary"`
	Message   string `json:"message"`
}

// FailbackResponse 故障回切响应.
type FailbackResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
}

// HAEvent 高可用事件.
type HAEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	NodeID    string    `json:"node_id"`
}

// EventsResponse 事件列表响应.
type EventsResponse struct {
	Events []HAEvent `json:"events"`
	Total  int       `json:"total"`
}

// NodesResponse 节点列表响应.
type NodesResponse struct {
	Nodes []HANode `json:"nodes"`
	Total int      `json:"total"`
}

// FailoverPolicy 故障转移策略.
type FailoverPolicy struct {
	AutoFailover   bool          `json:"auto_failover"`
	FailoverTimeout time.Duration `json:"failover_timeout"`
	MaxRetries     int           `json:"max_retries"`
	CooldownPeriod time.Duration `json:"cooldown_period"`
}

// TestResponse 连通性测试响应.
type TestResponse struct {
	Success    bool          `json:"success"`
	Latency    time.Duration `json:"latency"`
	Message    string        `json:"message"`
	TestTarget string        `json:"test_target"`
}

// ErrorResponse 错误响应.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SuccessResponse 通用成功响应.
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
