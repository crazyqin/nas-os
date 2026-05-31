// Package wanreplanner provides WAN link planning, load balancing,
// failover, QoS, and bandwidth prediction for multi-WAN environments.
package wanreplanner

import (
	"errors"
	"sync"
	"time"
)

// ============================================================
// 常量与错误
// ============================================================

// LinkStatus WAN链路状态
type LinkStatus string

const (
	LinkStatusUp      LinkStatus = "up"
	LinkStatusDown    LinkStatus = "down"
	LinkStatusDegraded LinkStatus = "degraded"
	LinkStatusUnknown LinkStatus = "unknown"
)

// LoadBalanceStrategy 负载均衡策略
type LoadBalanceStrategy string

const (
	StrategyRoundRobin    LoadBalanceStrategy = "round_robin"
	StrategyWeighted      LoadBalanceStrategy = "weighted"
	StrategyLeastConn     LoadBalanceStrategy = "least_conn"
	StrategySourceHash    LoadBalanceStrategy = "source_hash"
)

// QoSPriority 流量优先级
type QoSPriority int

const (
	QoSPriorityCritical QoSPriority = 0
	QoSPriorityHigh     QoSPriority = 1
	QoSPriorityMedium   QoSPriority = 2
	QoSPriorityLow      QoSPriority = 3
)

// ProbeType 探测类型
type ProbeType string

const (
	ProbePing ProbeType = "ping"
	ProbeTCP  ProbeType = "tcp"
	ProbeHTTP ProbeType = "http"
)

var (
	ErrLinkNotFound      = errors.New("link not found")
	ErrLinkAlreadyExists = errors.New("link already exists")
	ErrNoAvailableLinks  = errors.New("no available links")
	ErrInvalidConfig     = errors.New("invalid config")
	ErrPlannerNotRunning = errors.New("planner not running")
	ErrRuleNotFound      = errors.New("rule not found")
	ErrInvalidStrategy   = errors.New("invalid strategy")
	ErrInsufficientData  = errors.New("insufficient historical data")
)

// ============================================================
// 配置
// ============================================================

// PlannerConfig 规划器配置
type PlannerConfig struct {
	Strategy          LoadBalanceStrategy `json:"strategy"`
	HealthCheckInterval time.Duration     `json:"health_check_interval"`
	FailoverDelay     time.Duration       `json:"failover_delay"`     // 故障切换延迟
	FallbackDelay     time.Duration       `json:"fallback_delay"`     // 回切延迟
	ProbeTimeout      time.Duration       `json:"probe_timeout"`
	PredictionWindow  time.Duration       `json:"prediction_window"`  // 预测窗口
	HistoryRetention  time.Duration       `json:"history_retention"`  // 历史数据保留
}

// DefaultPlannerConfig 默认配置
func DefaultPlannerConfig() PlannerConfig {
	return PlannerConfig{
		Strategy:            StrategyRoundRobin,
		HealthCheckInterval: 10 * time.Second,
		FailoverDelay:       3 * time.Second,
		FallbackDelay:       30 * time.Second,
		ProbeTimeout:        5 * time.Second,
		PredictionWindow:    1 * time.Hour,
		HistoryRetention:    24 * time.Hour,
	}
}

// ============================================================
// WAN链路
// ============================================================

// WANLink WAN链路
type WANLink struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Gateway     string     `json:"gateway"`
	Interface   string     `json:"interface"`
	Weight      int        `json:"weight"`       // 负载均衡权重
	Bandwidth   int64      `json:"bandwidth"`    // 带宽 (bytes/s)
	Status      LinkStatus `json:"status"`
	Latency     time.Duration `json:"latency"`
	PacketLoss  float64    `json:"packet_loss"`  // 0.0 - 1.0
	Jitter      time.Duration `json:"jitter"`
	ActiveConns int        `json:"active_conns"`
	Score       float64    `json:"score"`        // 综合质量评分 0-100
	LastCheck   time.Time  `json:"last_check"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ============================================================
// 健康探测
// ============================================================

// ProbeTarget 探测目标
type ProbeTarget struct {
	Type     ProbeType `json:"type"`
	Host     string    `json:"host"`
	Port     int       `json:"port"`     // TCP探测端口
	Path     string    `json:"path"`     // HTTP探测路径
	Expected int       `json:"expected"` // HTTP期望状态码
}

// ProbeResult 探测结果
type ProbeResult struct {
	LinkID    string        `json:"link_id"`
	Target    ProbeTarget   `json:"target"`
	Success   bool          `json:"success"`
	Latency   time.Duration `json:"latency"`
	Timestamp time.Time     `json:"timestamp"`
	Error     string        `json:"error,omitempty"`
}

// ============================================================
// 故障切换
// ============================================================

// FailoverEvent 故障切换事件
type FailoverEvent struct {
	ID          string        `json:"id"`
	FromLinkID  string        `json:"from_link_id"`
	ToLinkID    string        `json:"to_link_id"`
	Reason      string        `json:"reason"`
	Duration    time.Duration `json:"duration"`
	Timestamp   time.Time     `json:"timestamp"`
	IsFallback  bool          `json:"is_fallback"` // 是否为回切
}

// ============================================================
// QoS
// ============================================================

// QoSRule QoS规则
type QoSRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Priority    QoSPriority `json:"priority"`
	Protocol    string      `json:"protocol"`     // tcp, udp, any
	SrcPort     int         `json:"src_port"`     // 0 = any
	DstPort     int         `json:"dst_port"`     // 0 = any
	MaxBandwidth int64      `json:"max_bandwidth"` // 限速 bytes/s, 0 = unlimited
	MinBandwidth int64      `json:"min_bandwidth"` // 保证带宽 bytes/s
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
}

// TrafficClass 流量分类
type TrafficClass struct {
	Name        string      `json:"name"`
	Priority    QoSPriority `json:"priority"`
	MatchCount  int64       `json:"match_count"`
	TotalBytes  int64       `json:"total_bytes"`
}

// ============================================================
// 带宽预测
// ============================================================

// BandwidthSample 带宽采样
type BandwidthSample struct {
	Timestamp    time.Time `json:"timestamp"`
	LinkID       string    `json:"link_id"`
	BytesIn      int64     `json:"bytes_in"`
	BytesOut     int64     `json:"bytes_out"`
	Utilization  float64   `json:"utilization"` // 0.0 - 1.0
}

// PredictionResult 预测结果
type PredictionResult struct {
	LinkID         string    `json:"link_id"`
	EstimatedBW    int64     `json:"estimated_bandwidth"`
	Confidence     float64   `json:"confidence"` // 0.0 - 1.0
	PredictedAt    time.Time `json:"predicted_at"`
	ValidUntil     time.Time `json:"valid_until"`
}

// ============================================================
// 核心引擎结构
// ============================================================

// WANPlanner WAN链路规划器
type WANPlanner struct {
	mu            sync.RWMutex
	config        PlannerConfig
	links         map[string]*WANLink
	rules         map[string]*QoSRule
	probes        []ProbeTarget
	history       []BandwidthSample
	failoverLog   []FailoverEvent
	rrIndex       int // round-robin 计数器
	running       bool
	stopCh        chan struct{}
}
