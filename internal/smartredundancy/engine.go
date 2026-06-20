package smartredundancy

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// RedundancyLevel 冗余级别.
type RedundancyLevel int

const (
	RedundancyNone RedundancyLevel = iota
	RedundancyMirror
	RedundancyRAID5
	RedundancyRAID6
	RedundancyTriple
	RedundancyErasureCoding
)

// NodeState 节点状态.
type NodeState string

const (
	NodeStateOnline  NodeState = "online"
	NodeStateOffline NodeState = "offline"
	NodeStateDegraded NodeState = "degraded"
	NodeStateSyncing NodeState = "syncing"
)

// StorageNode 存储节点.
type StorageNode struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Address     string        `json:"address"`
	State       NodeState     `json:"state"`
	Capacity    int64         `json:"capacity"`
	Used        int64         `json:"used"`
	Health      float64       `json:"health"` // 0-100
	LastSeen    time.Time     `json:"last_seen"`
	Metadata    map[string]string `json:"metadata"`
}

// RedundancyPolicy 冗余策略.
type RedundancyPolicy struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Level       RedundancyLevel `json:"level"`
	MinNodes    int             `json:"min_nodes"`
	GeoAware    bool            `json:"geo_aware"`    // 地理感知
	AutoHeal    bool            `json:"auto_heal"`    // 自动修复
	StripSize   int             `json:"strip_size"`   // 条带大小(KB)
	ParityCount int             `json:"parity_count"` // 校验块数
}

// DataPlacement 数据放置决策.
type DataPlacement struct {
	Primary   string   `json:"primary"`   // 主节点
	Secondary []string `json:"secondary"` // 副本节点
	Parity    []string `json:"parity"`    // 校验节点
	Strategy  string   `json:"strategy"`  // 放置策略
}

// Engine 智能冗余引擎.
type Engine struct {
	nodes    map[string]*StorageNode
	policies map[string]*RedundancyPolicy
	health   map[string]*NodeHealthMetrics
	logger   *zap.Logger
	mu       sync.RWMutex
}

// NodeHealthMetrics 节点健康指标.
type NodeHealthMetrics struct {
	CPUUsage    float64   `json:"cpu_usage"`
	MemUsage    float64   `json:"mem_usage"`
	DiskLatency float64   `json:"disk_latency"` // ms
	NetworkLat  float64   `json:"network_lat"`  // ms
	ErrorRate   float64   `json:"error_rate"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FailoverEvent 故障转移事件.
type FailoverEvent struct {
	ID          string    `json:"id"`
	SourceNode  string    `json:"source_node"`
	TargetNode  string    `json:"target_node"`
	Reason      string    `json:"reason"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Status      string    `json:"status"`
	DataSize    int64     `json:"data_size"`
}

// NewEngine 创建智能冗余引擎.
func NewEngine(logger *zap.Logger) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Engine{
		nodes:    make(map[string]*StorageNode),
		policies: make(map[string]*RedundancyPolicy),
		health:   make(map[string]*NodeHealthMetrics),
		logger:   logger,
	}
}

// RegisterNode 注册存储节点.
func (e *Engine) RegisterNode(node *StorageNode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if node.ID == "" {
		return ErrInvalidNodeID
	}
	node.LastSeen = time.Now()
	if node.Metadata == nil {
		node.Metadata = make(map[string]string)
	}
	e.nodes[node.ID] = node
	e.logger.Info("节点已注册", zap.String("id", node.ID), zap.String("name", node.Name))
	return nil
}

// GetNode 获取节点信息.
func (e *Engine) GetNode(id string) (*StorageNode, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	node, ok := e.nodes[id]
	return node, ok
}

// ListNodes 列出所有节点.
func (e *Engine) ListNodes() []*StorageNode {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	nodes := make([]*StorageNode, 0, len(e.nodes))
	for _, n := range e.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// GetOnlineNodes 获取在线节点.
func (e *Engine) GetOnlineNodes() []*StorageNode {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	var online []*StorageNode
	for _, n := range e.nodes {
		if n.State == NodeStateOnline {
			online = append(online, n)
		}
	}
	return online
}

// UpdateNodeHealth 更新节点健康状态.
func (e *Engine) UpdateNodeHealth(nodeID string, metrics *NodeHealthMetrics) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if _, ok := e.nodes[nodeID]; !ok {
		return ErrNodeNotFound
	}
	metrics.UpdatedAt = time.Now()
	e.health[nodeID] = metrics
	return nil
}

// CalculatePlacement 计算数据放置策略.
func (e *Engine) CalculatePlacement(policy *RedundancyPolicy, dataSize int64) (*DataPlacement, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	online := e.getOnlineNodesSorted()
	if len(online) < policy.MinNodes {
		return nil, ErrInsufficientNodes
	}
	
	placement := &DataPlacement{
		Strategy: policy.Level.String(),
	}
	
	switch policy.Level {
	case RedundancyMirror:
		placement.Primary = online[0].ID
		placement.Secondary = []string{online[1].ID}
	case RedundancyRAID5:
		placement.Primary = online[0].ID
		for i := 1; i < len(online) && i < policy.MinNodes; i++ {
			placement.Secondary = append(placement.Secondary, online[i].ID)
		}
		placement.Parity = []string{online[len(online)-1].ID}
	case RedundancyRAID6:
		placement.Primary = online[0].ID
		for i := 1; i < len(online)-2 && i < policy.MinNodes-2; i++ {
			placement.Secondary = append(placement.Secondary, online[i].ID)
		}
		placement.Parity = []string{online[len(online)-2].ID, online[len(online)-1].ID}
	case RedundancyTriple:
		placement.Primary = online[0].ID
		placement.Secondary = []string{online[1].ID, online[2].ID}
	default:
		placement.Primary = online[0].ID
	}
	
	return placement, nil
}

// getOnlineNodesSorted 按健康度排序的在线节点.
func (e *Engine) getOnlineNodesSorted() []*StorageNode {
	var online []*StorageNode
	for _, n := range e.nodes {
		if n.State == NodeStateOnline {
			online = append(online, n)
		}
	}
	
	// 按健康度降序排序
	for i := 0; i < len(online); i++ {
		for j := i + 1; j < len(online); j++ {
			if online[j].Health > online[i].Health {
				online[i], online[j] = online[j], online[i]
			}
		}
	}
	return online
}

// TriggerFailover 触发故障转移.
func (e *Engine) TriggerFailover(sourceID, reason string) (*FailoverEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	source, ok := e.nodes[sourceID]
	if !ok {
		return nil, ErrNodeNotFound
	}
	
	// 标记源节点为离线
	source.State = NodeStateOffline
	
	// 查找最佳目标节点
	var target *StorageNode
	bestHealth := -1.0
	for _, n := range e.nodes {
		if n.ID != sourceID && n.State == NodeStateOnline && n.Health > bestHealth {
			target = n
			bestHealth = n.Health
		}
	}
	
	if target == nil {
		return nil, ErrNoAvailableTarget
	}
	
	event := &FailoverEvent{
		ID:         generateID(),
		SourceNode: sourceID,
		TargetNode: target.ID,
		Reason:     reason,
		StartTime:  time.Now(),
		Status:     "in_progress",
	}
	
	e.logger.Warn("触发故障转移",
		zap.String("source", sourceID),
		zap.String("target", target.ID),
		zap.String("reason", reason),
	)
	
	return event, nil
}

// GetClusterStatus 获取集群状态.
func (e *Engine) GetClusterStatus() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	total, online, offline, degraded := 0, 0, 0, 0
	var totalCap, usedCap int64
	
	for _, n := range e.nodes {
		total++
		totalCap += n.Capacity
		usedCap += n.Used
		switch n.State {
		case NodeStateOnline:
			online++
		case NodeStateOffline:
			offline++
		case NodeStateDegraded:
			degraded++
		}
	}
	
	return map[string]interface{}{
		"total_nodes":  total,
		"online":       online,
		"offline":      offline,
		"degraded":     degraded,
		"total_capacity": totalCap,
		"used_capacity":  usedCap,
		"health_score":   e.calculateClusterHealth(),
	}
}

// calculateClusterHealth 计算集群整体健康度.
func (e *Engine) calculateClusterHealth() float64 {
	if len(e.nodes) == 0 {
		return 0
	}
	total := 0.0
	for _, n := range e.nodes {
		total += n.Health
	}
	return total / float64(len(e.nodes))
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomHex(8)
}

func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[time.Now().UnixNano()%16]
	}
	return string(b)
}

// Level.String 返回冗余级别名称.
func (l RedundancyLevel) String() string {
	switch l {
	case RedundancyNone:
		return "none"
	case RedundancyMirror:
		return "mirror"
	case RedundancyRAID5:
		return "raid5"
	case RedundancyRAID6:
		return "raid6"
	case RedundancyTriple:
		return "triple"
	case RedundancyErasureCoding:
		return "erasure_coding"
	default:
		return "unknown"
	}
}
