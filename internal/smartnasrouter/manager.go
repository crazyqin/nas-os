// Package smartnasrouter 提供智能NAS路由功能
package smartnasrouter

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 智能路由管理器
type Manager struct {
	mu          sync.RWMutex
	nodes       map[string]*Node
	rules       map[string]*RouteRule
	healthCfg   HealthCheckConfig
	failovers   []*FailoverEvent
	probeResults map[string][]*ProbeResult
	totalReqs   int64
	failedReqs  int64
	roundRobin  int
}

// NewManager 创建路由管理器
func NewManager(healthCfg *HealthCheckConfig) *Manager {
	cfg := DefaultHealthCheckConfig()
	if healthCfg != nil {
		cfg = *healthCfg
	}
	return &Manager{
		nodes:        make(map[string]*Node),
		rules:        make(map[string]*RouteRule),
		healthCfg:    cfg,
		failovers:    make([]*FailoverEvent, 0),
		probeResults: make(map[string][]*ProbeResult),
	}
}

// ========== 节点管理 ==========

// AddNode 添加节点
func (m *Manager) AddNode(req AddNodeRequest) (*Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查重复
	for _, n := range m.nodes {
		if n.Host == req.Host && n.Port == req.Port {
			return nil, ErrNodeAlreadyExists
		}
	}

	weight := req.Weight
	if weight <= 0 {
		weight = 50
	}
	if weight > 100 {
		return nil, ErrInvalidWeight
	}
	maxConns := req.MaxConns
	if maxConns <= 0 {
		maxConns = 100
	}

	now := time.Now()
	node := &Node{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Host:      req.Host,
		Port:      req.Port,
		Status:    NodeStatusOnline,
		Region:    req.Region,
		Weight:    weight,
		MaxConns:  maxConns,
		Tags:      req.Tags,
		CreatedAt: now,
		UpdatedAt: now,
		LastSeen:  now,
	}
	m.nodes[node.ID] = node
	return node, nil
}

// GetNode 获取节点
func (m *Manager) GetNode(id string) (*Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

// UpdateNode 更新节点
func (m *Manager) UpdateNode(id string, req UpdateNodeRequest) (*Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}

	if req.Name != "" {
		node.Name = req.Name
	}
	if req.Host != "" {
		node.Host = req.Host
	}
	if req.Port > 0 {
		node.Port = req.Port
	}
	if req.Region != "" {
		node.Region = req.Region
	}
	if req.Weight != nil {
		if *req.Weight < 0 || *req.Weight > 100 {
			return nil, ErrInvalidWeight
		}
		node.Weight = *req.Weight
	}
	if req.MaxConns != nil {
		node.MaxConns = *req.MaxConns
	}
	if req.Status != "" {
		node.Status = req.Status
	}
	if req.Tags != nil {
		node.Tags = req.Tags
	}
	node.UpdatedAt = time.Now()
	return node, nil
}

// DeleteNode 删除节点
func (m *Manager) DeleteNode(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nodes[id]; !ok {
		return ErrNodeNotFound
	}
	delete(m.nodes, id)
	delete(m.probeResults, id)
	return nil
}

// ListNodes 列出所有节点
func (m *Manager) ListNodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*Node, 0, len(m.nodes))
	for _, n := range m.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// ========== 路由规则 ==========

// AddRule 添加路由规则
func (m *Manager) AddRule(rule RouteRule) *RouteRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	rule.CreatedAt = time.Now()
	m.rules[rule.ID] = &rule
	return &rule
}

// ListRules 列出所有路由规则
func (m *Manager) ListRules() []*RouteRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*RouteRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	return rules
}

// DeleteRule 删除路由规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return ErrNodeNotFound
	}
	delete(m.rules, id)
	return nil
}

// ========== 延迟探测 ==========

// ProbeNode 探测单个节点延迟
func (m *Manager) ProbeNode(nodeID string) (*ProbeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return nil, ErrNodeNotFound
	}

	// 模拟延迟探测
	latency := m.simulateProbe(node)
	result := &ProbeResult{
		NodeID:    nodeID,
		Latency:   latency,
		Success:   true,
		Timestamp: time.Now(),
	}

	// 更新节点延迟
	node.Latency = latency
	node.LastProbe = result.Timestamp
	node.LastSeen = result.Timestamp

	// 保存探测结果
	m.probeResults[nodeID] = append(m.probeResults[nodeID], result)
	if len(m.probeResults[nodeID]) > 100 {
		m.probeResults[nodeID] = m.probeResults[nodeID][1:]
	}

	return result, nil
}

// ProbeAll 探测所有节点
func (m *Manager) ProbeAll() []*ProbeResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := make([]*ProbeResult, 0, len(m.nodes))
	for _, node := range m.nodes {
		if node.Status == NodeStatusMaintenance {
			continue
		}

		latency := m.simulateProbe(node)
		result := &ProbeResult{
			NodeID:    node.ID,
			Latency:   latency,
			Success:   true,
			Timestamp: time.Now(),
		}

		node.Latency = latency
		node.LastProbe = result.Timestamp
		node.LastSeen = result.Timestamp

		m.probeResults[node.ID] = append(m.probeResults[node.ID], result)
		if len(m.probeResults[node.ID]) > 100 {
			m.probeResults[node.ID] = m.probeResults[node.ID][1:]
		}

		results = append(results, result)
	}
	return results
}

// simulateProbe 模拟延迟探测
func (m *Manager) simulateProbe(node *Node) int64 {
	// 模拟延迟：基于负载计算
	baseLatency := int64(10)
	if node.CPUUsage > 80 {
		baseLatency += int64(node.CPUUsage - 80)
	}
	if node.MemoryUsage > 80 {
		baseLatency += int64((node.MemoryUsage - 80) / 2)
	}
	// 添加随机抖动
	jitter := rand.Int63n(20) - 10
	return baseLatency + jitter
}

// ========== 路由决策 ==========

// Route 获取路由决策
func (m *Manager) Route(req RouteRequest) (*RouteDecision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalReqs++

	// 查找匹配的路由规则
	strategy := req.Strategy
	var targetNodeIDs []string

	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		if rule.SourceRegion != "" && rule.SourceRegion != req.SourceRegion {
			continue
		}
		if strategy == "" {
			strategy = rule.Strategy
		}
		if len(rule.TargetNodes) > 0 {
			targetNodeIDs = rule.TargetNodes
		}
		break // 取第一个匹配的规则（按优先级排序）
	}

	if strategy == "" {
		strategy = StrategyWeighted
	}

	// 获取候选节点
	candidates := m.getCandidates(targetNodeIDs)
	if len(candidates) == 0 {
		m.failedReqs++
		return nil, ErrNoHealthyNodes
	}

	// 根据策略选择节点
	var selected *Node
	var score float64
	var reason string

	switch strategy {
	case StrategyRoundRobin:
		selected, reason = m.selectRoundRobin(candidates)
		score = m.calculateScore(selected)
	case StrategyWeighted:
		selected, score, reason = m.selectWeighted(candidates)
	case StrategyLeastConn:
		selected, score, reason = m.selectLeastConn(candidates)
	case StrategyLatency:
		selected, score, reason = m.selectLatency(candidates)
	case StrategyGeo:
		selected, score, reason = m.selectGeo(candidates, req.SourceRegion)
	default:
		selected, score, reason = m.selectWeighted(candidates)
	}

	if selected == nil {
		m.failedReqs++
		return nil, ErrNoHealthyNodes
	}

	return &RouteDecision{
		NodeID:    selected.ID,
		NodeName:  selected.Name,
		Host:      selected.Host,
		Port:      selected.Port,
		Strategy:  strategy,
		Score:     score,
		Latency:   selected.Latency,
		Reason:    reason,
		DecidedAt: time.Now(),
	}, nil
}

// getCandidates 获取候选节点
func (m *Manager) getCandidates(targetIDs []string) []*Node {
	var candidates []*Node

	if len(targetIDs) > 0 {
		for _, id := range targetIDs {
			if node, ok := m.nodes[id]; ok && node.Status == NodeStatusOnline {
				candidates = append(candidates, node)
			}
		}
	} else {
		for _, node := range m.nodes {
			if node.Status == NodeStatusOnline {
				candidates = append(candidates, node)
			}
		}
	}
	return candidates
}

// selectRoundRobin 轮询选择
func (m *Manager) selectRoundRobin(nodes []*Node) (*Node, string) {
	idx := m.roundRobin % len(nodes)
	m.roundRobin++
	return nodes[idx], fmt.Sprintf("轮询选择，索引 %d", idx)
}

// selectWeighted 加权选择
func (m *Manager) selectWeighted(nodes []*Node) (*Node, float64, string) {
	type scored struct {
		node  *Node
		score float64
	}

	var scoredList []scored
	for _, n := range nodes {
		s := m.calculateScore(n)
		scoredList = append(scoredList, scored{n, s})
	}

	sort.Slice(scoredList, func(i, j int) bool {
		return scoredList[i].score > scoredList[j].score
	})

	best := scoredList[0]
	return best.node, best.score, fmt.Sprintf("综合评分最高 %.1f", best.score)
}

// selectLeastConn 最少连接选择
func (m *Manager) selectLeastConn(nodes []*Node) (*Node, float64, string) {
	var best *Node
	bestScore := -1.0

	for _, n := range nodes {
		connRatio := float64(n.CurrConns) / float64(n.MaxConns)
		score := (1 - connRatio) * 100
		if score > bestScore {
			bestScore = score
			best = n
		}
	}

	return best, bestScore, fmt.Sprintf("连接数最少 %d/%d", best.CurrConns, best.MaxConns)
}

// selectLatency 最低延迟选择
func (m *Manager) selectLatency(nodes []*Node) (*Node, float64, string) {
	var best *Node
	bestLatency := int64(math.MaxInt64)

	for _, n := range nodes {
		if n.Latency < bestLatency {
			bestLatency = n.Latency
			best = n
		}
	}

	score := math.Max(0, 100-float64(bestLatency))
	return best, score, fmt.Sprintf("延迟最低 %dms", bestLatency)
}

// selectGeo 地理位置选择
func (m *Manager) selectGeo(nodes []*Node, region string) (*Node, float64, string) {
	// 优先选择同区域节点
	for _, n := range nodes {
		if n.Region == region {
			score := m.calculateScore(n)
			return n, score, fmt.Sprintf("同区域 %s 优先", region)
		}
	}

	// 无同区域节点，选择综合评分最高的
	return m.selectWeighted(nodes)
}

// calculateScore 计算节点综合评分
func (m *Manager) calculateScore(n *Node) float64 {
	weightScore := float64(n.Weight) * 0.3
	cpuScore := (100 - n.CPUUsage) * 0.2
	memScore := (100 - n.MemoryUsage) * 0.15
	diskScore := (100 - n.DiskUsage) * 0.15
	latencyScore := math.Max(0, 100-float64(n.Latency)) * 0.2
	return weightScore + cpuScore + memScore + diskScore + latencyScore
}

// ========== 故障转移 ==========

// TriggerFailover 触发故障转移
func (m *Manager) TriggerFailover(nodeID, reason string) (*FailoverEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return nil, ErrNodeNotFound
	}

	// 标记节点为离线
	node.Status = NodeStatusOffline
	node.UpdatedAt = time.Now()

	// 找到最佳替代节点
	var bestNode *Node
	bestScore := -1.0
	for _, n := range m.nodes {
		if n.ID == nodeID || n.Status != NodeStatusOnline {
			continue
		}
		score := m.calculateScore(n)
		if score > bestScore {
			bestScore = score
			bestNode = n
		}
	}

	event := &FailoverEvent{
		ID:         uuid.New().String(),
		FromNodeID: nodeID,
		Reason:     reason,
		Timestamp:  time.Now(),
	}

	if bestNode != nil {
		event.ToNodeID = bestNode.ID
		bestNode.CurrConns += node.CurrConns // 接管连接
	}

	m.failovers = append(m.failovers, event)
	return event, nil
}

// RecoverNode 恢复节点
func (m *Manager) RecoverNode(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return ErrNodeNotFound
	}

	node.Status = NodeStatusOnline
	node.FailCount = 0
	node.UpdatedAt = time.Now()
	return nil
}

// ========== 统计 ==========

// GetStats 获取路由统计
func (m *Manager) GetStats() RouterStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := RouterStats{
		TotalNodes:    len(m.nodes),
		TotalRequests: m.totalReqs,
		FailedRequests: m.failedReqs,
		TotalRoutes:   len(m.rules),
	}

	var totalLatency int64
	var latencyCount int
	for _, r := range m.rules {
		if r.Enabled {
			stats.ActiveRoutes++
		}
	}

	for _, n := range m.nodes {
		switch n.Status {
		case NodeStatusOnline:
			stats.OnlineNodes++
		case NodeStatusOffline:
			stats.OfflineNodes++
		case NodeStatusDegraded:
			stats.DegradedNodes++
		}
		if n.Latency > 0 {
			totalLatency += n.Latency
			latencyCount++
		}
	}

	if latencyCount > 0 {
		stats.AvgLatency = float64(totalLatency) / float64(latencyCount)
	}
	if m.totalReqs > 0 {
		stats.SuccessRate = float64(m.totalReqs-m.failedReqs) / float64(m.totalReqs) * 100
	}

	return stats
}

// GetFailoverEvents 获取故障转移事件
func (m *Manager) GetFailoverEvents() []*FailoverEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]*FailoverEvent, len(m.failovers))
	copy(events, m.failovers)
	return events
}

// UpdateNodeMetrics 更新节点指标
func (m *Manager) UpdateNodeMetrics(nodeID string, cpu, mem, disk float64, conns int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return ErrNodeNotFound
	}

	node.CPUUsage = cpu
	node.MemoryUsage = mem
	node.DiskUsage = disk
	node.CurrConns = conns
	node.LastSeen = time.Now()
	node.UpdatedAt = time.Now()

	// 自动降级检测
	if cpu > 90 || mem > 90 || disk > 95 {
		if node.Status == NodeStatusOnline {
			node.Status = NodeStatusDegraded
		}
	}

	return nil
}
