// Package smartnasrouter 提供智能NAS路由功能
// 根据网络状况、服务器负载、地理位置智能选择最佳NAS节点
// 包含：负载均衡、故障转移、延迟探测、路由决策引擎
package smartnasrouter

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNodeNotFound 节点不存在.
	ErrNodeNotFound = errors.New("节点不存在")
	// ErrNoHealthyNodes 没有健康节点可用.
	ErrNoHealthyNodes = errors.New("没有健康节点可用")
	// ErrNodeAlreadyExists 节点已存在.
	ErrNodeAlreadyExists = errors.New("节点已存在")
	// ErrInvalidWeight 无效权重值.
	ErrInvalidWeight = errors.New("无效的权重值，范围 0-100")
	// ErrProbeFailed 探测失败.
	ErrProbeFailed = errors.New("延迟探测失败")
	// ErrRouteNotFound 未找到可用路由.
	ErrRouteNotFound = errors.New("未找到可用路由")
)

// ========== 节点状态 ==========

// NodeStatus 节点状态.
type NodeStatus string

const (
	// NodeStatusOnline 在线.
	NodeStatusOnline NodeStatus = "online"
	// NodeStatusOffline 离线.
	NodeStatusOffline NodeStatus = "offline"
	// NodeStatusDegraded 降级.
	NodeStatusDegraded NodeStatus = "degraded"
	// NodeStatusMaintenance 维护中.
	NodeStatusMaintenance NodeStatus = "maintenance"
)

// ========== 负载均衡策略 ==========

// BalanceStrategy 负载均衡策略.
type BalanceStrategy string

const (
	// StrategyRoundRobin 轮询.
	StrategyRoundRobin BalanceStrategy = "round_robin"
	// StrategyWeighted 加权.
	StrategyWeighted BalanceStrategy = "weighted"
	// StrategyLeastConn 最少连接.
	StrategyLeastConn BalanceStrategy = "least_conn"
	// StrategyLatency 最低延迟.
	StrategyLatency BalanceStrategy = "latency"
	// StrategyGeo 地理位置优先.
	StrategyGeo BalanceStrategy = "geo"
)

// ========== 节点信息 ==========

// Node NAS节点信息.
type Node struct {
	ID          string     `json:"id"`           // 节点唯一标识
	Name        string     `json:"name"`         // 节点名称
	Host        string     `json:"host"`         // 主机地址
	Port        int        `json:"port"`         // 端口
	Status      NodeStatus `json:"status"`       // 节点状态
	Region      string     `json:"region"`       // 地域标识
	Weight      int        `json:"weight"`       // 权重 (0-100)
	MaxConns    int        `json:"maxConns"`     // 最大连接数
	CurrConns   int        `json:"currConns"`    // 当前连接数
	CPUUsage    float64    `json:"cpuUsage"`     // CPU使用率 (%)
	MemoryUsage float64    `json:"memoryUsage"`  // 内存使用率 (%)
	DiskUsage   float64    `json:"diskUsage"`    // 磁盘使用率 (%)
	NetworkIn   int64      `json:"networkIn"`    // 入站带宽 (bytes/s)
	NetworkOut  int64      `json:"networkOut"`   // 出站带宽 (bytes/s)
	Latency     int64      `json:"latency"`      // 延迟 (ms)
	LastProbe   time.Time  `json:"lastProbe"`    // 最后探测时间
	LastSeen    time.Time  `json:"lastSeen"`     // 最后活跃时间
	FailCount   int        `json:"failCount"`    // 连续失败次数
	Tags        []string   `json:"tags,omitempty"` // 标签
	CreatedAt   time.Time  `json:"createdAt"`    // 创建时间
	UpdatedAt   time.Time  `json:"updatedAt"`    // 更新时间
}

// ========== 探测结果 ==========

// ProbeResult 探测结果.
type ProbeResult struct {
	NodeID    string    `json:"nodeId"`    // 节点ID
	Latency   int64     `json:"latency"`   // 延迟 (ms)
	Success   bool      `json:"success"`   // 是否成功
	Error     string    `json:"error"`     // 错误信息
	Timestamp time.Time `json:"timestamp"` // 探测时间
}

// ========== 路由决策 ==========

// RouteDecision 路由决策结果.
type RouteDecision struct {
	NodeID      string          `json:"nodeId"`      // 选中节点ID
	NodeName    string          `json:"nodeName"`    // 节点名称
	Host        string          `json:"host"`        // 主机地址
	Port        int             `json:"port"`        // 端口
	Strategy    BalanceStrategy `json:"strategy"`    // 使用的策略
	Score       float64         `json:"score"`       // 节点评分 (0-100)
	Latency     int64           `json:"latency"`     // 预估延迟
	Reason      string          `json:"reason"`      // 选择原因
	DecidedAt   time.Time       `json:"decidedAt"`   // 决策时间
}

// ========== 路由规则 ==========

// RouteRule 路由规则.
type RouteRule struct {
	ID          string          `json:"id"`          // 规则ID
	Name        string          `json:"name"`        // 规则名称
	Priority    int             `json:"priority"`    // 优先级 (越小越优先)
	SourceRegion string         `json:"sourceRegion"` // 来源地域
	TargetNodes []string        `json:"targetNodes"` // 目标节点列表
	Strategy    BalanceStrategy `json:"strategy"`    // 负载均衡策略
	Enabled     bool            `json:"enabled"`     // 是否启用
	CreatedAt   time.Time       `json:"createdAt"`   // 创建时间
}

// ========== 健康检查配置 ==========

// HealthCheckConfig 健康检查配置.
type HealthCheckConfig struct {
	IntervalSeconds int  `json:"intervalSeconds"` // 检查间隔(秒)
	TimeoutSeconds  int  `json:"timeoutSeconds"`  // 超时时间(秒)
	MaxFailCount    int  `json:"maxFailCount"`    // 最大失败次数
	Enabled         bool `json:"enabled"`         // 是否启用
}

// DefaultHealthCheckConfig 返回默认健康检查配置.
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		IntervalSeconds: 30,
		TimeoutSeconds:  5,
		MaxFailCount:    3,
		Enabled:         true,
	}
}

// ========== 路由统计 ==========

// RouterStats 路由统计信息.
type RouterStats struct {
	TotalNodes     int     `json:"totalNodes"`     // 总节点数
	OnlineNodes    int     `json:"onlineNodes"`    // 在线节点数
	OfflineNodes   int     `json:"offlineNodes"`   // 离线节点数
	DegradedNodes  int     `json:"degradedNodes"`  // 降级节点数
	TotalRoutes    int     `json:"totalRoutes"`    // 总路由规则数
	ActiveRoutes   int     `json:"activeRoutes"`   // 活跃路由数
	AvgLatency     float64 `json:"avgLatency"`     // 平均延迟(ms)
	TotalRequests  int64   `json:"totalRequests"`  // 总请求数
	FailedRequests int64   `json:"failedRequests"` // 失败请求数
	SuccessRate    float64 `json:"successRate"`    // 成功率(%)
}

// ========== 故障转移事件 ==========

// FailoverEvent 故障转移事件.
type FailoverEvent struct {
	ID          string    `json:"id"`          // 事件ID
	FromNodeID  string    `json:"fromNodeId"`  // 源节点
	ToNodeID    string    `json:"toNodeId"`    // 目标节点
	Reason      string    `json:"reason"`      // 转移原因
	Timestamp   time.Time `json:"timestamp"`   // 发生时间
	RecoveryTime int64    `json:"recoveryTime"` // 恢复耗时(ms)
}

// ========== 请求/响应结构 ==========

// AddNodeRequest 添加节点请求.
type AddNodeRequest struct {
	Name     string   `json:"name" binding:"required"`
	Host     string   `json:"host" binding:"required"`
	Port     int      `json:"port" binding:"required"`
	Region   string   `json:"region"`
	Weight   int      `json:"weight"`
	MaxConns int      `json:"maxConns"`
	Tags     []string `json:"tags,omitempty"`
}

// UpdateNodeRequest 更新节点请求.
type UpdateNodeRequest struct {
	Name     string   `json:"name"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Region   string   `json:"region"`
	Weight   *int     `json:"weight"`
	MaxConns *int     `json:"maxConns"`
	Status   NodeStatus `json:"status"`
	Tags     []string `json:"tags,omitempty"`
}

// RouteRequest 路由请求.
type RouteRequest struct {
	SourceRegion string          `json:"sourceRegion"` // 来源地域
	Strategy     BalanceStrategy `json:"strategy"`     // 策略 (可选，覆盖规则)
	ClientIP     string          `json:"clientIP"`     // 客户端IP
	Hint         string          `json:"hint"`         // 附加提示
}
