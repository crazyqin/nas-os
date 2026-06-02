// Package nascommander NAS舰队指挥官
// 提供多NAS集中管理、远程监控、统一分发配置、跨站点同步、健康聚合功能
// 对标 TrueCommand 集中管理平台
package nascommander

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"  // 在线
	NodeStatusOffline NodeStatus = "offline" // 离线
	NodeStatusDegraded NodeStatus = "degraded" // 降级
	NodeStatusMaintenance NodeStatus = "maintenance" // 维护中
)

// NodeRole 节点角色
type NodeRole string

const (
	RolePrimary   NodeRole = "primary"   // 主节点
	RoleSecondary NodeRole = "secondary" // 备用节点
	RoleEdge      NodeRole = "edge"      // 边缘节点
	RoleArchive   NodeRole = "archive"   // 归档节点
)

// ClusterStatus 集群状态
type ClusterStatus string

const (
	ClusterHealthy  ClusterStatus = "healthy"  // 健康
	ClusterWarning  ClusterStatus = "warning"  // 警告
	ClusterCritical ClusterStatus = "critical" // 严重
	ClusterUnknown  ClusterStatus = "unknown"  // 未知
)

// NASNode NAS节点
type NASNode struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Hostname     string            `json:"hostname"`
	IPAddress    string            `json:"ip_address"`
	Port         int               `json:"port"`
	Role         NodeRole          `json:"role"`
	Status       NodeStatus        `json:"status"`
	Version      string            `json:"version"`
	Location     string            `json:"location,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Metrics      *NodeMetrics      `json:"metrics,omitempty"`
	Config       *NodeConfig       `json:"config,omitempty"`
	LastSeen     time.Time         `json:"last_seen"`
	RegisteredAt time.Time         `json:"registered_at"`
}

// NodeMetrics 节点指标
type NodeMetrics struct {
	CPUUsage      float64 `json:"cpu_usage"`       // CPU使用率 0-100
	MemoryUsage   float64 `json:"memory_usage"`    // 内存使用率 0-100
	DiskUsage     float64 `json:"disk_usage"`      // 磁盘使用率 0-100
	NetworkIn     int64   `json:"network_in"`      // 入站流量 bytes/s
	NetworkOut    int64   `json:"network_out"`     // 出站流量 bytes/s
	Temperature   float64 `json:"temperature"`     // 温度 °C
	Uptime        int64   `json:"uptime"`          // 运行时间 seconds
	LoadAvg1      float64 `json:"load_avg_1"`      // 1分钟负载
	LoadAvg5      float64 `json:"load_avg_5"`      // 5分钟负载
	LoadAvg15     float64 `json:"load_avg_15"`     // 15分钟负载
	TotalDisk     int64   `json:"total_disk"`      // 总磁盘空间
	UsedDisk      int64   `json:"used_disk"`       // 已用磁盘空间
	TotalMemory   int64   `json:"total_memory"`    // 总内存
	UsedMemory    int64   `json:"used_memory"`     // 已用内存
}

// NodeConfig 节点配置
type NodeConfig struct {
	AutoUpdate    bool              `json:"auto_update"`
	AlertEmail    string            `json:"alert_email,omitempty"`
	SSHEnabled    bool              `json:"ssh_enabled"`
	SSHPort       int               `json:"ssh_port"`
	TLSEnabled    bool              `json:"tls_enabled"`
	APIEndpoint   string            `json:"api_endpoint,omitempty"`
	CustomOptions map[string]string `json:"custom_options,omitempty"`
}

// Cluster 集群
type Cluster struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Nodes       []*NASNode    `json:"nodes"`
	Status      ClusterStatus `json:"status"`
	Config      *ClusterConfig `json:"config"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// ClusterConfig 集群配置
type ClusterConfig struct {
	LoadBalance    bool   `json:"load_balance"`     // 负载均衡
	Failover       bool   `json:"failover"`         // 自动故障转移
	SyncInterval   string `json:"sync_interval"`    // 同步间隔
	HealthCheck    string `json:"health_check"`     // 健康检查间隔
	AlertThreshold float64 `json:"alert_threshold"` // 告警阈值
}

// SyncTask 同步任务
type SyncTask struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	SourceNode  string    `json:"source_node"`
	TargetNodes []string  `json:"target_nodes"`
	ConfigPath  string    `json:"config_path"`
	Status      string    `json:"status"`
	LastSync    time.Time `json:"last_sync"`
	NextSync    time.Time `json:"next_sync"`
	Error       string    `json:"error,omitempty"`
}

// Alert 聚合告警
type Alert struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	Level     string    `json:"level"` // info, warning, critical
	Category  string    `json:"category"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	Acked     bool      `json:"acked"`
	AckedBy   string    `json:"acked_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	AckedAt   *time.Time `json:"acked_at,omitempty"`
}

// Commander 舰队指挥官
type Commander struct {
	mu       sync.RWMutex
	config   *Config
	nodes    map[string]*NASNode
	clusters map[string]*Cluster
	syncs    map[string]*SyncTask
	alerts   []*Alert
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// Config 指挥官配置
type Config struct {
	DiscoveryInterval time.Duration `json:"discovery_interval"` // 节点发现间隔
	HealthInterval    time.Duration `json:"health_interval"`    // 健康检查间隔
	SyncInterval      time.Duration `json:"sync_interval"`      // 同步间隔
	AlertRetention    time.Duration `json:"alert_retention"`    // 告警保留时间
	MaxNodes          int           `json:"max_nodes"`          // 最大节点数
	EnableAutoSync    bool          `json:"enable_auto_sync"`   // 自动同步
	EnableFailover    bool          `json:"enable_failover"`    // 自动故障转移
}

// NewCommander 创建新的指挥官
func NewCommander(config *Config) *Commander {
	if config == nil {
		config = &Config{
			DiscoveryInterval: 5 * time.Minute,
			HealthInterval:    time.Minute,
			SyncInterval:      10 * time.Minute,
			AlertRetention:    7 * 24 * time.Hour,
			MaxNodes:          100,
			EnableAutoSync:    true,
			EnableFailover:    true,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Commander{
		config:   config,
		nodes:    make(map[string]*NASNode),
		clusters: make(map[string]*Cluster),
		syncs:    make(map[string]*SyncTask),
		alerts:   make([]*Alert, 0),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// RegisterNode 注册节点
func (c *Commander) RegisterNode(node *NASNode) error {
	if node == nil {
		return errors.New("node cannot be nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.nodes[node.ID]; exists {
		return fmt.Errorf("node %s already registered", node.ID)
	}

	node.RegisteredAt = time.Now()
	node.LastSeen = time.Now()
	if node.Status == "" {
		node.Status = NodeStatusOnline
	}
	c.nodes[node.ID] = node
	return nil
}

// UnregisterNode 注销节点
func (c *Commander) UnregisterNode(nodeID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	delete(c.nodes, nodeID)
	return nil
}

// GetNode 获取节点
func (c *Commander) GetNode(nodeID string) (*NASNode, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	node, exists := c.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}
	return node, nil
}

// ListNodes 列出所有节点
func (c *Commander) ListNodes() []*NASNode {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nodes := make([]*NASNode, 0, len(c.nodes))
	for _, node := range c.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// UpdateNodeMetrics 更新节点指标
func (c *Commander) UpdateNodeMetrics(nodeID string, metrics *NodeMetrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	node.Metrics = metrics
	node.LastSeen = time.Now()
	return nil
}

// CreateCluster 创建集群
func (c *Commander) CreateCluster(cluster *Cluster) error {
	if cluster == nil {
		return errors.New("cluster cannot be nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.clusters[cluster.ID]; exists {
		return fmt.Errorf("cluster %s already exists", cluster.ID)
	}

	cluster.CreatedAt = time.Now()
	cluster.UpdatedAt = time.Now()
	c.clusters[cluster.ID] = cluster
	return nil
}

// GetCluster 获取集群
func (c *Commander) GetCluster(clusterID string) (*Cluster, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cluster, exists := c.clusters[clusterID]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}
	return cluster, nil
}

// AddAlert 添加告警
func (c *Commander) AddAlert(alert *Alert) {
	c.mu.Lock()
	defer c.mu.Unlock()

	alert.CreatedAt = time.Now()
	c.alerts = append(c.alerts, alert)
}

// GetAlerts 获取告警列表
func (c *Commander) GetAlerts(acked bool) []*Alert {
	c.mu.RLock()
	defer c.mu.RUnlock()

	alerts := make([]*Alert, 0)
	for _, alert := range c.alerts {
		if alert.Acked == acked {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// AcknowledgeAlert 确认告警
func (c *Commander) AcknowledgeAlert(alertID, ackedBy string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, alert := range c.alerts {
		if alert.ID == alertID {
			alert.Acked = true
			alert.AckedBy = ackedBy
			now := time.Now()
			alert.AckedAt = &now
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", alertID)
}

// GetClusterStatus 获取集群整体状态
func (c *Commander) GetClusterStatus() ClusterStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.nodes) == 0 {
		return ClusterUnknown
	}

	onlineCount := 0
	for _, node := range c.nodes {
		if node.Status == NodeStatusOnline {
			onlineCount++
		}
	}

	ratio := float64(onlineCount) / float64(len(c.nodes))
	if ratio >= 0.8 {
		return ClusterHealthy
	} else if ratio >= 0.5 {
		return ClusterWarning
	}
	return ClusterCritical
}

// Start 启动指挥官
func (c *Commander) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return errors.New("commander is already running")
	}
	c.running = true
	return nil
}

// Stop 停止指挥官
func (c *Commander) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return errors.New("commander is not running")
	}
	c.running = false
	c.cancel()
	return nil
}
