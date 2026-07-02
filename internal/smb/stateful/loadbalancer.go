package stateful

import (
	"hash/fnv"
	"sync"
	"sync/atomic"
)

// StrategyType 负载均衡策略类型.
type StrategyType string

const (
	StrategyRoundRobin StrategyType = "roundrobin" // 轮询
	StrategyLeastConn  StrategyType = "leastconn"  // 最少连接
	StrategyIPHash     StrategyType = "iphash"     // IP哈希
)

// NodeWeight 节点权重配置.
type NodeWeight struct {
	NodeID string
	Weight int // 1-100，默认50
}

// SMBClientLoadBalancer SMB客户端负载均衡器
// Phase3: 与StatefulFailoverManager集成，提供多策略节点选择.
type SMBClientLoadBalancer struct {
	manager  *StatefulFailoverManager
	strategy StrategyType

	mu       sync.RWMutex
	weights  map[string]int    // nodeID -> weight
	counters map[string]*int64 // nodeID -> active connection counter (atomic)
	rrIndex  uint64            // round-robin atomic counter
}

// NewSMBClientLoadBalancer 创建负载均衡器.
func NewSMBClientLoadBalancer(manager *StatefulFailoverManager, strategy StrategyType) *SMBClientLoadBalancer {
	lb := &SMBClientLoadBalancer{
		manager:  manager,
		strategy: strategy,
		weights:  make(map[string]int),
		counters: make(map[string]*int64),
	}

	// 初始化本地节点和所有peer节点的权重和计数器
	lb.ensureNode(manager.localNode.NodeID)
	for _, peer := range manager.peerNodes {
		lb.ensureNode(peer.NodeID)
	}

	return lb
}

// ensureNode 确保节点在权重表和计数器中存在.
func (lb *SMBClientLoadBalancer) ensureNode(nodeID string) {
	if _, ok := lb.weights[nodeID]; !ok {
		lb.weights[nodeID] = 50 // 默认权重
	}
	if _, ok := lb.counters[nodeID]; !ok {
		lb.counters[nodeID] = new(int64)
	}
}

// SetWeight 设置节点权重.
func (lb *SMBClientLoadBalancer) SetWeight(nodeID string, weight int) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	if weight < 1 {
		weight = 1
	}
	if weight > 100 {
		weight = 100
	}
	lb.weights[nodeID] = weight
}

// GetWeight 获取节点权重.
func (lb *SMBClientLoadBalancer) GetWeight(nodeID string) int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	if w, ok := lb.weights[nodeID]; ok {
		return w
	}
	return 50
}

// SetStrategy 切换负载均衡策略.
func (lb *SMBClientLoadBalancer) SetStrategy(strategy StrategyType) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.strategy = strategy
}

// GetStrategy 获取当前策略.
func (lb *SMBClientLoadBalancer) GetStrategy() StrategyType {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.strategy
}

// SelectNode 根据策略选择最佳节点
// clientIP用于iphash策略，可以为空（其他策略不使用）.
func (lb *SMBClientLoadBalancer) SelectNode(clientIP string) *FailoverNode {
	lb.mu.RLock()
	strategy := lb.strategy
	lb.mu.RUnlock()

	healthy := lb.getHealthyNodes()
	if len(healthy) == 0 {
		return nil
	}
	if len(healthy) == 1 {
		return healthy[0]
	}

	switch strategy {
	case StrategyRoundRobin:
		return lb.selectRoundRobin(healthy)
	case StrategyLeastConn:
		return lb.selectLeastConn(healthy)
	case StrategyIPHash:
		return lb.selectIPHash(healthy, clientIP)
	default:
		return lb.selectRoundRobin(healthy)
	}
}

// selectRoundRobin 加权轮询选择.
func (lb *SMBClientLoadBalancer) selectRoundRobin(nodes []*FailoverNode) *FailoverNode {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// 构建加权节点列表，权重越高出现次数越多
	var weighted []*FailoverNode
	for _, n := range nodes {
		w := lb.weights[n.NodeID]
		if w <= 0 {
			w = 1
		}
		for i := 0; i < w; i++ {
			weighted = append(weighted, n)
		}
	}
	if len(weighted) == 0 {
		return nil
	}

	idx := atomic.AddUint64(&lb.rrIndex, 1)
	return weighted[idx%uint64(len(weighted))]
}

// selectLeastConn 最少连接选择.
func (lb *SMBClientLoadBalancer) selectLeastConn(nodes []*FailoverNode) *FailoverNode {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var best *FailoverNode
	minLoad := int64(-1)

	for _, n := range nodes {
		counter, ok := lb.counters[n.NodeID]
		if !ok {
			continue
		}
		load := atomic.LoadInt64(counter)
		// 考虑权重：有效负载 = 实际负载 / 权重
		w := lb.weights[n.NodeID]
		if w <= 0 {
			w = 1
		}
		effectiveLoad := load * 100 / int64(w)
		if minLoad < 0 || effectiveLoad < minLoad {
			minLoad = effectiveLoad
			best = n
		}
	}
	return best
}

// selectIPHash IP哈希选择（会话亲和性）.
func (lb *SMBClientLoadBalancer) selectIPHash(nodes []*FailoverNode, clientIP string) *FailoverNode {
	if clientIP == "" {
		// 无IP时退化为轮询
		return lb.selectRoundRobin(nodes)
	}

	h := fnv.New32a()
	h.Write([]byte(clientIP))
	hashVal := h.Sum32()

	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// 加权哈希：构建加权列表
	var weighted []*FailoverNode
	for _, n := range nodes {
		w := lb.weights[n.NodeID]
		if w <= 0 {
			w = 1
		}
		for i := 0; i < w; i++ {
			weighted = append(weighted, n)
		}
	}
	if len(weighted) == 0 {
		return nil
	}

	return weighted[hashVal%uint32(len(weighted))]
}

// getHealthyNodes 获取所有健康节点（包括本地节点）.
func (lb *SMBClientLoadBalancer) getHealthyNodes() []*FailoverNode {
	lb.manager.mu.RLock()
	defer lb.manager.mu.RUnlock()

	var nodes []*FailoverNode

	// 检查本地节点
	if lb.manager.localNode.Status == NodeStatusActive ||
		lb.manager.localNode.Status == NodeStatusStandby {
		nodes = append(nodes, lb.manager.localNode)
	}

	// 检查peer节点
	for _, peer := range lb.manager.peerNodes {
		if peer.Status == NodeStatusActive ||
			peer.Status == NodeStatusStandby ||
			peer.Status == NodeStatusDegraded {
			nodes = append(nodes, peer)
		}
	}

	return nodes
}

// IncrConn 增加节点连接计数.
func (lb *SMBClientLoadBalancer) IncrConn(nodeID string) {
	lb.mu.RLock()
	counter, ok := lb.counters[nodeID]
	lb.mu.RUnlock()
	if ok {
		atomic.AddInt64(counter, 1)
	}
}

// DecrConn 减少节点连接计数.
func (lb *SMBClientLoadBalancer) DecrConn(nodeID string) {
	lb.mu.RLock()
	counter, ok := lb.counters[nodeID]
	lb.mu.RUnlock()
	if ok {
		atomic.AddInt64(counter, -1)
	}
}

// GetConnCount 获取节点当前连接数.
func (lb *SMBClientLoadBalancer) GetConnCount(nodeID string) int64 {
	lb.mu.RLock()
	counter, ok := lb.counters[nodeID]
	lb.mu.RUnlock()
	if !ok {
		return 0
	}
	return atomic.LoadInt64(counter)
}

// DistributionStats 会话分布统计.
type DistributionStats struct {
	NodeID      string     `json:"node_id"`
	Weight      int        `json:"weight"`
	ActiveConns int64      `json:"active_conns"`
	Sessions    int        `json:"sessions"`
	Status      NodeStatus `json:"status"`
}

// GetDistributionStats 获取会话分布统计.
func (lb *SMBClientLoadBalancer) GetDistributionStats() []DistributionStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	lb.manager.mu.RLock()
	defer lb.manager.mu.RUnlock()

	allNodes := make(map[string]*FailoverNode)
	allNodes[lb.manager.localNode.NodeID] = lb.manager.localNode
	for id, peer := range lb.manager.peerNodes {
		allNodes[id] = peer
	}

	var stats []DistributionStats
	for nodeID, node := range allNodes {
		w := lb.weights[nodeID]
		conns := int64(0)
		if c, ok := lb.counters[nodeID]; ok {
			conns = atomic.LoadInt64(c)
		}
		sessions := lb.manager.registry.GetByNode(nodeID)

		stats = append(stats, DistributionStats{
			NodeID:      nodeID,
			Weight:      w,
			ActiveConns: conns,
			Sessions:    len(sessions),
			Status:      node.Status,
		})
	}
	return stats
}
