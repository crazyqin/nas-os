package cluster

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 边缘负载均衡算法.
const (
	EdgeLBStrategyRoundRobin  = "round-robin"
	EdgeLBStrategyLeastLoad   = "least-load"
	EdgeLBStrategyResource    = "resource"
	EdgeLBStrategyLatency     = "latency"
	EdgeLBStrategyWeighted    = "weighted"
	EdgeLBStrategyGeoLocation = "geo-location"
	EdgeLBStrategyCapability  = "capability"
)

// EdgeLBConfig 边缘负载均衡配置.
type EdgeLBConfig struct {
	Strategy         string        `json:"strategy"`
	HealthCheckInt   time.Duration `json:"health_check_interval"`
	HealthTimeout    time.Duration `json:"health_timeout"`
	MaxRetry         int           `json:"max_retry"`
	StickySession    bool          `json:"sticky_session"`
	LocationWeight   float64       `json:"location_weight"`   // 位置权重
	ResourceWeight   float64       `json:"resource_weight"`   // 资源权重
	LatencyWeight    float64       `json:"latency_weight"`    // 延迟权重
	CapabilityWeight float64       `json:"capability_weight"` // 能力权重
}

// EdgeLoadBalancer 边缘负载均衡器.
type EdgeLoadBalancer struct {
	config       EdgeLBConfig
	configMutex  sync.RWMutex
	edgeManager  *EdgeNodeManager
	sessions     map[string]string // session -> nodeID
	sessionMutex sync.RWMutex
	stats        EdgeLBStats
	statsMutex   sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	logger       *zap.Logger
}

// EdgeLBStats 边缘负载均衡统计.
type EdgeLBStats struct {
	TotalRequests   int64                       `json:"total_requests"`
	SuccessRequests int64                       `json:"success_requests"`
	FailedRequests  int64                       `json:"failed_requests"`
	AvgLatency      time.Duration               `json:"avg_latency"`
	RequestsPerSec  float64                     `json:"requests_per_sec"`
	NodeStats       map[string]*EdgeNodeLBStats `json:"node_stats"`
}

// EdgeNodeLBStats 边缘节点负载均衡统计.
type EdgeNodeLBStats struct {
	NodeID      string        `json:"node_id"`
	Requests    int64         `json:"requests"`
	Successes   int64         `json:"successes"`
	Failures    int64         `json:"failures"`
	AvgLatency  time.Duration `json:"avg_latency"`
	LastRequest time.Time     `json:"last_request"`
	LastError   string        `json:"last_error"`
}

// NewEdgeLoadBalancer 创建边缘负载均衡器.
func NewEdgeLoadBalancer(config EdgeLBConfig, edgeManager *EdgeNodeManager, logger *zap.Logger) (*EdgeLoadBalancer, error) {
	if config.Strategy == "" {
		config.Strategy = EdgeLBStrategyLeastLoad
	}
	if config.HealthCheckInt == 0 {
		config.HealthCheckInt = 10 * time.Second
	}
	if config.HealthTimeout == 0 {
		config.HealthTimeout = 5 * time.Second
	}
	if config.MaxRetry == 0 {
		config.MaxRetry = 3
	}
	if config.LocationWeight == 0 {
		config.LocationWeight = 0.3
	}
	if config.ResourceWeight == 0 {
		config.ResourceWeight = 0.4
	}
	if config.LatencyWeight == 0 {
		config.LatencyWeight = 0.2
	}
	if config.CapabilityWeight == 0 {
		config.CapabilityWeight = 0.1
	}

	ctx, cancel := context.WithCancel(context.Background())

	elb := &EdgeLoadBalancer{
		config:      config,
		edgeManager: edgeManager,
		sessions:    make(map[string]string),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
	}

	// 初始化统计
	elb.stats.NodeStats = make(map[string]*EdgeNodeLBStats)

	return elb, nil
}

// Initialize 初始化边缘负载均衡器.
func (elb *EdgeLoadBalancer) Initialize() error {
	elb.logger.Info("初始化边缘负载均衡器", zap.String("strategy", elb.config.Strategy))

	// 启动健康检查
	go elb.healthCheckWorker()

	// 启动统计收集
	go elb.statsWorker()

	elb.logger.Info("边缘负载均衡器初始化完成")
	return nil
}

// SelectNode 选择节点.
func (elb *EdgeLoadBalancer) SelectNode(req SelectNodeRequest) (*EdgeNode, error) {
	// 检查会话保持
	if elb.config.StickySession && req.SessionID != "" {
		elb.sessionMutex.RLock()
		if nodeID, exists := elb.sessions[req.SessionID]; exists {
			elb.sessionMutex.RUnlock()
			if node, exists := elb.edgeManager.GetNode(nodeID); exists && elb.isNodeAvailable(node) {
				return node, nil
			}
		} else {
			elb.sessionMutex.RUnlock()
		}
	}

	// 获取可用节点
	nodes := elb.getAvailableNodes(req.Requirements)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("没有可用的边缘节点")
	}

	// 根据策略选择节点
	var selected *EdgeNode
	switch elb.config.Strategy {
	case EdgeLBStrategyRoundRobin:
		selected = elb.selectRoundRobin(nodes, req)
	case EdgeLBStrategyLeastLoad:
		selected = elb.selectLeastLoad(nodes, req)
	case EdgeLBStrategyResource:
		selected = elb.selectByResource(nodes, req)
	case EdgeLBStrategyLatency:
		selected = elb.selectByLatency(nodes, req)
	case EdgeLBStrategyWeighted:
		selected = elb.selectWeighted(nodes, req)
	case EdgeLBStrategyGeoLocation:
		selected = elb.selectByGeoLocation(nodes, req)
	case EdgeLBStrategyCapability:
		selected = elb.selectByCapability(nodes, req)
	default:
		selected = elb.selectLeastLoad(nodes, req)
	}

	if selected == nil {
		return nil, fmt.Errorf("无法选择合适的边缘节点")
	}

	// 记录会话
	if elb.config.StickySession && req.SessionID != "" {
		elb.sessionMutex.Lock()
		elb.sessions[req.SessionID] = selected.ID
		elb.sessionMutex.Unlock()
	}

	return selected, nil
}

// SelectNodeRequest 选择节点请求.
type SelectNodeRequest struct {
	SessionID    string                 `json:"session_id"`
	ClientIP     string                 `json:"client_ip"`
	ClientLat    float64                `json:"client_lat"`
	ClientLng    float64                `json:"client_lng"`
	Requirements TaskRequirements       `json:"requirements"`
	Preferences  map[string]interface{} `json:"preferences"`
}

// isNodeAvailable 检查节点是否可用.
func (elb *EdgeLoadBalancer) isNodeAvailable(node *EdgeNode) bool {
	return node.Status == EdgeNodeStatusOnline || node.Status == EdgeNodeStatusIdle
}

// getAvailableNodes 获取可用节点.
func (elb *EdgeLoadBalancer) getAvailableNodes(req TaskRequirements) []*EdgeNode {
	nodes := elb.edgeManager.GetAvailableNodes()

	// 过滤满足要求的节点
	filtered := make([]*EdgeNode, 0)
	for _, node := range nodes {
		if elb.nodeMeetsRequirements(node, req) {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

// nodeMeetsRequirements 检查节点是否满足要求.
func (elb *EdgeLoadBalancer) nodeMeetsRequirements(node *EdgeNode, req TaskRequirements) bool {
	if req.CPU > 0 && node.Capabilities.CPU < req.CPU {
		return false
	}
	if req.Memory > 0 && node.Capabilities.Memory < req.Memory {
		return false
	}
	if req.GPU && !node.Capabilities.GPU {
		return false
	}
	if req.Capabilities > 0 && node.Capabilities.Caps&req.Capabilities == 0 {
		return false
	}
	if req.Region != "" && node.Location.Region != req.Region {
		return false
	}
	if req.Zone != "" && node.Location.Zone != req.Zone {
		return false
	}
	if req.NodeType != "" && node.Type != req.NodeType {
		return false
	}
	return true
}

// selectRoundRobin 轮询选择.
func (elb *EdgeLoadBalancer) selectRoundRobin(nodes []*EdgeNode, req SelectNodeRequest) *EdgeNode {
	if len(nodes) == 0 {
		return nil
	}

	// 简单轮询
	elb.statsMutex.Lock()
	idx := int(elb.stats.TotalRequests) % len(nodes)
	elb.statsMutex.Unlock()

	return nodes[idx]
}

// selectLeastLoad 最小负载选择.
func (elb *EdgeLoadBalancer) selectLeastLoad(nodes []*EdgeNode, req SelectNodeRequest) *EdgeNode {
	if len(nodes) == 0 {
		return nil
	}

	var selected *EdgeNode
	minScore := math.MaxFloat64

	for _, node := range nodes {
		score := elb.calculateLoadScore(node)
		if score < minScore {
			minScore = score
			selected = node
		}
	}

	return selected
}

// calculateLoadScore 计算负载得分（越低越好）.
func (elb *EdgeLoadBalancer) calculateLoadScore(node *EdgeNode) float64 {
	cpuScore := node.Resources.CPUUsed / 100.0
	memScore := node.Resources.MemoryUsed / 100.0
	taskScore := float64(node.TasksRunning) / 10.0 // 假设最多 10 个并发任务

	return cpuScore*0.4 + memScore*0.3 + taskScore*0.3
}

// selectByResource 按资源选择.
func (elb *EdgeLoadBalancer) selectByResource(nodes []*EdgeNode, req SelectNodeRequest) *EdgeNode {
	if len(nodes) == 0 {
		return nil
	}

	var selected *EdgeNode
	maxScore := -1.0

	for _, node := range nodes {
		score := elb.calculateResourceScore(node)
		if score > maxScore {
			maxScore = score
			selected = node
		}
	}

	return selected
}

// calculateResourceScore 计算资源得分（越高越好）.
func (elb *EdgeLoadBalancer) calculateResourceScore(node *EdgeNode) float64 {
	cpuAvail := float64(node.Capabilities.CPU) * (1 - node.Resources.CPUUsed/100.0)
	memAvail := float64(node.Capabilities.Memory) * (1 - node.Resources.MemoryUsed/100.0)

	return cpuAvail*0.5 + memAvail*0.5
}

// selectByLatency 按延迟选择.
func (elb *EdgeLoadBalancer) selectByLatency(nodes []*EdgeNode, req SelectNodeRequest) *EdgeNode {
	if len(nodes) == 0 {
		return nil
	}

	var selected *EdgeNode
	minLatency := time.Duration(math.MaxInt64)

	for _, node := range nodes {
		// 计算估计延迟（基于距离或历史数据）
		latency := elb.estimateLatency(node, req)
		if latency < minLatency {
			minLatency = latency
			selected = node
		}
	}

	return selected
}

// estimateLatency 估计延迟.
func (elb *EdgeLoadBalancer) estimateLatency(node *EdgeNode, req SelectNodeRequest) time.Duration {
	// 如果有客户端位置信息，计算地理距离
	if req.ClientLat != 0 && req.ClientLng != 0 && node.Location.Latitude != 0 {
		distance := elb.calculateGeoDistance(req.ClientLat, req.ClientLng, node.Location.Latitude, node.Location.Longitude)
		// 简单估计：每 100km 约 1ms
		return time.Duration(distance/100) * time.Millisecond
	}

	// 使用统计中的平均延迟
	elb.statsMutex.RLock()
	if stats, exists := elb.stats.NodeStats[node.ID]; exists {
		elb.statsMutex.RUnlock()
		return stats.AvgLatency
	}
	elb.statsMutex.RUnlock()

	// 默认延迟
	return 50 * time.Millisecond
}

// calculateGeoDistance 计算地理距离（km）.
func (elb *EdgeLoadBalancer) calculateGeoDistance(lat1, lng1, lat2, lng2 float64) float64 {
	// 简化的距离计算
	dLat := lat2 - lat1
	dLng := lng2 - lng1
	return math.Sqrt(dLat*dLat+dLng*dLng) * 111 // 纬度每度约 111km
}

// selectWeighted 加权选择.
func (elb *EdgeLoadBalancer) selectWeighted(nodes []*EdgeNode, req SelectNodeRequest) *EdgeNode {
	if len(nodes) == 0 {
		return nil
	}

	var selected *EdgeNode
	maxScore := -1.0

	for _, node := range nodes {
		// 综合评分
		loadScore := 1 - elb.calculateLoadScore(node)
		resourceScore := elb.calculateResourceScore(node) / 100

		// 确保各分数在 0-1 范围内
		if loadScore < 0 {
			loadScore = 0
		}
		if resourceScore < 0 {
			resourceScore = 0
		}

		// 计算延迟得分，确保非负
		latency := float64(elb.estimateLatency(node, req))
		latencyScore := 1 - latency/float64(100*time.Millisecond)
		if latencyScore < 0 {
			latencyScore = 0
		}

		score := loadScore*elb.config.ResourceWeight +
			resourceScore*elb.config.ResourceWeight +
			latencyScore*elb.config.LatencyWeight +
			float64(node.Weight)/100.0*0.1

		if score > maxScore {
			maxScore = score
			selected = node
		}
	}

	return selected
}

// selectByGeoLocation 按地理位置选择.
func (elb *EdgeLoadBalancer) selectByGeoLocation(nodes []*EdgeNode, req SelectNodeRequest) *EdgeNode {
	if len(nodes) == 0 {
		return nil
	}

	if req.ClientLat == 0 && req.ClientLng == 0 {
		// 没有位置信息，退回到最小负载
		return elb.selectLeastLoad(nodes, req)
	}

	var selected *EdgeNode
	minDistance := math.MaxFloat64

	for _, node := range nodes {
		if node.Location.Latitude == 0 && node.Location.Longitude == 0 {
			continue
		}

		distance := elb.calculateGeoDistance(req.ClientLat, req.ClientLng, node.Location.Latitude, node.Location.Longitude)
		if distance < minDistance {
			minDistance = distance
			selected = node
		}
	}

	if selected == nil {
		return nodes[0]
	}

	return selected
}

// selectByCapability 按能力选择.
func (elb *EdgeLoadBalancer) selectByCapability(nodes []*EdgeNode, req SelectNodeRequest) *EdgeNode {
	if len(nodes) == 0 {
		return nil
	}

	// 计算每个节点的能力匹配得分
	var selected *EdgeNode
	maxScore := -1

	for _, node := range nodes {
		score := 0
		if node.Capabilities.GPU {
			score += 10
		}
		if node.Capabilities.AI {
			score += 10
		}
		score += int(node.Capabilities.Caps)

		// 考虑负载
		loadFactor := 1 - elb.calculateLoadScore(node)
		score = int(float64(score) * loadFactor)

		if score > maxScore {
			maxScore = score
			selected = node
		}
	}

	return selected
}

// RecordRequest 记录请求.
func (elb *EdgeLoadBalancer) RecordRequest(nodeID string, success bool, latency time.Duration) {
	elb.statsMutex.Lock()
	defer elb.statsMutex.Unlock()

	elb.stats.TotalRequests++
	if success {
		elb.stats.SuccessRequests++
	} else {
		elb.stats.FailedRequests++
	}

	// 更新平均延迟
	if elb.stats.AvgLatency == 0 {
		elb.stats.AvgLatency = latency
	} else {
		elb.stats.AvgLatency = (elb.stats.AvgLatency*9 + latency) / 10
	}

	// 更新节点统计
	if _, exists := elb.stats.NodeStats[nodeID]; !exists {
		elb.stats.NodeStats[nodeID] = &EdgeNodeLBStats{NodeID: nodeID}
	}

	nodeStats := elb.stats.NodeStats[nodeID]
	nodeStats.Requests++
	nodeStats.LastRequest = time.Now()
	if success {
		nodeStats.Successes++
	} else {
		nodeStats.Failures++
	}

	if nodeStats.AvgLatency == 0 {
		nodeStats.AvgLatency = latency
	} else {
		nodeStats.AvgLatency = (nodeStats.AvgLatency*9 + latency) / 10
	}
}

// GetStats 获取统计.
func (elb *EdgeLoadBalancer) GetStats() EdgeLBStats {
	elb.statsMutex.RLock()
	defer elb.statsMutex.RUnlock()

	return elb.stats
}

// ClearSession 清除会话.
func (elb *EdgeLoadBalancer) ClearSession(sessionID string) {
	elb.sessionMutex.Lock()
	defer elb.sessionMutex.Unlock()

	delete(elb.sessions, sessionID)
}

// ClearAllSessions 清除所有会话.
func (elb *EdgeLoadBalancer) ClearAllSessions() {
	elb.sessionMutex.Lock()
	defer elb.sessionMutex.Unlock()

	elb.sessions = make(map[string]string)
}

// healthCheckWorker 健康检查工作线程.
func (elb *EdgeLoadBalancer) healthCheckWorker() {
	elb.configMutex.RLock()
	interval := elb.config.HealthCheckInt
	elb.configMutex.RUnlock()
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-elb.ctx.Done():
			return
		case <-ticker.C:
			elb.checkNodesHealth()
		}
	}
}

// checkNodesHealth 检查节点健康.
func (elb *EdgeLoadBalancer) checkNodesHealth() {
	nodes := elb.edgeManager.GetNodes()

	for _, node := range nodes {
		// 简化：基于心跳判断
		// 实际实现应该发送 HTTP/gRPC 健康检查请求
		_ = node
	}
}

// statsWorker 统计工作线程.
func (elb *EdgeLoadBalancer) statsWorker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var lastRequests int64
	lastTime := time.Now()

	for {
		select {
		case <-elb.ctx.Done():
			return
		case <-ticker.C:
			elb.statsMutex.Lock()
			now := time.Now()
			elapsed := now.Sub(lastTime).Seconds()
			if elapsed > 0 {
				elb.stats.RequestsPerSec = float64(elb.stats.TotalRequests-lastRequests) / elapsed
			}
			lastRequests = elb.stats.TotalRequests
			lastTime = now
			elb.statsMutex.Unlock()
		}
	}
}

// UpdateConfig 更新配置.
func (elb *EdgeLoadBalancer) UpdateConfig(config EdgeLBConfig) {
	elb.configMutex.Lock()
	elb.config = config
	elb.configMutex.Unlock()
	elb.logger.Info("边缘负载均衡配置已更新", zap.String("strategy", config.Strategy))
}

// GetConfig 获取配置.
func (elb *EdgeLoadBalancer) GetConfig() EdgeLBConfig {
	elb.configMutex.RLock()
	defer elb.configMutex.RUnlock()
	return elb.config
}

// Shutdown 关闭边缘负载均衡器.
func (elb *EdgeLoadBalancer) Shutdown() error {
	elb.cancel()
	elb.logger.Info("边缘负载均衡器已关闭")
	return nil
}
