// Package unifiedmonitor 多节点统一监控面板
// 集中监控多个 NAS 节点，提供实时指标聚合、跨节点告警关联、统一健康评分和节点间延迟监测
package unifiedmonitor

import (
	"sync"
	"time"
)

// ========== 节点与集群 ==========

// ClusterNode 集群节点
type ClusterNode struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Hostname     string            `json:"hostname"`
	IPAddress    string            `json:"ip_address"`
	Role         NodeRole          `json:"role"`
	Status       NodeStatus        `json:"status"`
	Metrics      NodeMetrics       `json:"metrics"`
	LastSeen     time.Time         `json:"last_seen"`
	RegisteredAt time.Time         `json:"registered_at"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// NodeRole 节点角色
type NodeRole string

const (
	RoleLeader  NodeRole = "leader"
	RoleWorker  NodeRole = "worker"
	RoleWitness NodeRole = "witness"
)

// NodeStatus 节点运行状态
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
	NodeStatusDegraded NodeStatus = "degraded"
)

// NodeMetrics 节点实时指标
type NodeMetrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemPercent    float64 `json:"mem_percent"`
	MemTotalGB    float64 `json:"mem_total_gb"`
	MemUsedGB     float64 `json:"mem_used_gb"`
	DiskPercent   float64 `json:"disk_percent"`
	DiskTotalGB   float64 `json:"disk_total_gb"`
	DiskUsedGB    float64 `json:"disk_used_gb"`
	NetInBytes    int64   `json:"net_in_bytes"`
	NetOutBytes   int64   `json:"net_out_bytes"`
	NetBandwidth  float64 `json:"net_bandwidth_mbps"`
	LoadAvg1      float64 `json:"load_avg_1"`
	LoadAvg5      float64 `json:"load_avg_5"`
	LoadAvg15     float64 `json:"load_avg_15"`
	Temperature   float64 `json:"temperature"`
	UptimeSeconds int64   `json:"uptime_seconds"`
}

// ========== 指标 ==========

// MetricPoint 指标数据点
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	NodeID    string            `json:"node_id"`
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// AggregatedMetrics 聚合指标
type AggregatedMetrics struct {
	Name      string      `json:"name"`
	Min       float64     `json:"min"`
	Max       float64     `json:"max"`
	Avg       float64     `json:"avg"`
	Sum       float64     `json:"sum"`
	Count     int         `json:"count"`
	PerNode   map[string]float64 `json:"per_node"`
	Timestamp time.Time   `json:"timestamp"`
}

// ========== 健康评分 ==========

// ClusterHealthScore 集群健康评分
type ClusterHealthScore struct {
	Score    int              `json:"score"`    // 0-100
	Level    string           `json:"level"`    // good/warning/critical
	PerNode  map[string]int   `json:"per_node"` // 各节点得分
	Details  map[string]int   `json:"details"`  // 各项得分
	LastEval time.Time        `json:"last_eval"`
}

// ========== 节点间延迟 ==========

// NodeLatency 节点间延迟
type NodeLatency struct {
	SourceNodeID string        `json:"source_node_id"`
	TargetNodeID string        `json:"target_node_id"`
	Latency      time.Duration `json:"latency"`
	Jitter       time.Duration `json:"jitter"`
	PacketLoss   float64       `json:"packet_loss"`
	MeasuredAt   time.Time     `json:"measured_at"`
}

// LatencyMatrix 延迟矩阵
type LatencyMatrix struct {
	Nodes    []string              `json:"nodes"`
	Matrix   map[string]map[string]time.Duration `json:"matrix"`
	AvgMs    float64               `json:"avg_ms"`
	MaxMs    float64               `json:"max_ms"`
	MinMs    float64               `json:"min_ms"`
	Updated  time.Time             `json:"updated"`
}

// ========== 告警 ==========

// AlertRule 告警规则
type AlertRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        RuleType          `json:"type"`
	Metric      string            `json:"metric"`
	Condition   AlertCondition    `json:"condition"`
	Threshold   float64           `json:"threshold"`
	Duration    time.Duration     `json:"duration"`
	Severity    AlertSeverity     `json:"severity"`
	NodeIDs     []string          `json:"node_ids,omitempty"` // 空表示所有节点
	Labels      map[string]string `json:"labels,omitempty"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
}

// RuleType 规则类型
type RuleType string

const (
	RuleTypeThreshold RuleType = "threshold"
	RuleTypeTrend     RuleType = "trend"
	RuleTypeAnomaly   RuleType = "anomaly"
)

// AlertCondition 告警条件
type AlertCondition string

const (
	ConditionAbove        AlertCondition = "above"
	ConditionBelow        AlertCondition = "below"
	ConditionEqual        AlertCondition = "equal"
	ConditionRateIncrease AlertCondition = "rate_increase"
	ConditionRateDecrease AlertCondition = "rate_decrease"
)

// AlertSeverity 告警严重级别
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// Alert 告警实例
type Alert struct {
	ID         string            `json:"id"`
	RuleID     string            `json:"rule_id"`
	RuleName   string            `json:"rule_name"`
	Severity   AlertSeverity     `json:"severity"`
	Message    string            `json:"message"`
	Value      float64           `json:"value"`
	Threshold  float64           `json:"threshold"`
	NodeID     string            `json:"node_id"`
	Labels     map[string]string `json:"labels,omitempty"`
	Status     AlertStatus       `json:"status"`
	Triggered  time.Time         `json:"triggered"`
	Resolved   *time.Time        `json:"resolved,omitempty"`
}

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"
	AlertStatusResolved AlertStatus = "resolved"
	AlertStatusSilenced AlertStatus = "silenced"
)

// ========== 跨节点告警关联 ==========

// CorrelatedAlert 关联告警
type CorrelatedAlert struct {
	RootCause   string   `json:"root_cause"`
	RelatedIDs  []string `json:"related_ids"`
	Description string   `json:"description"`
	Severity    AlertSeverity `json:"severity"`
	Timestamp   time.Time `json:"timestamp"`
}

// ========== 仪表板 ==========

// DashboardData 仪表板数据
type DashboardData struct {
	ClusterHealth  ClusterHealthScore          `json:"cluster_health"`
	Nodes          []ClusterNode               `json:"nodes"`
	ActiveAlerts   []Alert                     `json:"active_alerts"`
	Correlated     []CorrelatedAlert           `json:"correlated_alerts"`
	Latency        LatencyMatrix               `json:"latency"`
	Aggregated     map[string]AggregatedMetrics `json:"aggregated"`
	TopIssues      []string                    `json:"top_issues"`
	Timestamp      time.Time                   `json:"timestamp"`
}

// ========== 存储接口 ==========

// MetricStore 指标存储接口
type MetricStore interface {
	Store(point MetricPoint) error
	Query(name string, nodeID string, start, end time.Time) ([]MetricPoint, error)
	Aggregate(name string, start, end time.Time) (*AggregatedMetrics, error)
}

// AlertStore 告警存储接口
type AlertStore interface {
	Store(alert Alert) error
	Query(status AlertStatus, limit int) ([]Alert, error)
	UpdateStatus(alertID string, status AlertStatus) error
}

// ========== 管理器 ==========

// Manager 统一监控管理器
type Manager struct {
	mu           sync.RWMutex
	nodes        map[string]*ClusterNode
	rules        map[string]*AlertRule
	alerts       map[string]*Alert
	correlated   []CorrelatedAlert
	latency      map[string]map[string]*NodeLatency
	metricStore  MetricStore
	alertStore   AlertStore
	config       ManagerConfig
	stopCh       chan struct{}
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	MetricRetention     time.Duration `json:"metric_retention"`
	AlertDedupWindow    time.Duration `json:"alert_dedup_window"`
	MaxAlerts           int           `json:"max_alerts"`
	OfflineThreshold    time.Duration `json:"offline_threshold"`
	LatencyCheckInterval time.Duration `json:"latency_check_interval"`
}

// DefaultConfig 默认配置
func DefaultConfig() ManagerConfig {
	return ManagerConfig{
		HealthCheckInterval:  30 * time.Second,
		MetricRetention:      7 * 24 * time.Hour,
		AlertDedupWindow:     5 * time.Minute,
		MaxAlerts:            1000,
		OfflineThreshold:     90 * time.Second,
		LatencyCheckInterval: 60 * time.Second,
	}
}

// NewManager 创建管理器
func NewManager(metricStore MetricStore, alertStore AlertStore, config ManagerConfig) *Manager {
	return &Manager{
		nodes:       make(map[string]*ClusterNode),
		rules:       make(map[string]*AlertRule),
		alerts:      make(map[string]*Alert),
		correlated:  make([]CorrelatedAlert, 0),
		latency:     make(map[string]map[string]*NodeLatency),
		metricStore: metricStore,
		alertStore:  alertStore,
		config:      config,
		stopCh:      make(chan struct{}),
	}
}
