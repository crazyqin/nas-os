// Package clustermgr 提供分布式集群管理功能
package clustermgr

import (
	"fmt"
	"hash/crc32"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalancer 负载均衡器接口.
type LoadBalancer interface {
	// Select 选择节点.
	Select(nodes []*Node, key string) (*Node, error)
	// UpdateNodes 更新节点列表.
	UpdateNodes(nodes []*Node)
	// GetStrategy 获取策略.
	GetStrategy() LoadBalanceStrategy
}

// RoundRobinBalancer 轮询负载均衡器.
type RoundRobinBalancer struct {
	mu      sync.RWMutex
	counter uint64
	nodes   []*Node
}

// NewRoundRobinBalancer 创建轮询负载均衡器.
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

// Select 选择节点（轮询）.
func (b *RoundRobinBalancer) Select(nodes []*Node, key string) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	// 使用原子操作增加计数器
	idx := atomic.AddUint64(&b.counter, 1)
	selected := nodes[idx%uint64(len(nodes))]
	return selected, nil
}

// UpdateNodes 更新节点列表.
func (b *RoundRobinBalancer) UpdateNodes(nodes []*Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes = nodes
}

// GetStrategy 获取策略.
func (b *RoundRobinBalancer) GetStrategy() LoadBalanceStrategy {
	return StrategyRoundRobin
}

// WeightedBalancer 加权负载均衡器.
type WeightedBalancer struct {
	mu    sync.RWMutex
	nodes []*Node
}

// NewWeightedBalancer 创建加权负载均衡器.
func NewWeightedBalancer() *WeightedBalancer {
	return &WeightedBalancer{}
}

// Select 选择节点（加权随机）.
func (b *WeightedBalancer) Select(nodes []*Node, key string) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	// 计算总权重
	totalWeight := 0
	for _, node := range nodes {
		weight := node.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
	}

	// 随机选择
	r := rand.Intn(totalWeight)
	for _, node := range nodes {
		weight := node.Weight
		if weight <= 0 {
			weight = 1
		}
		r -= weight
		if r < 0 {
			return node, nil
		}
	}

	// 不应该到达这里
	return nodes[0], nil
}

// UpdateNodes 更新节点列表.
func (b *WeightedBalancer) UpdateNodes(nodes []*Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes = nodes
}

// GetStrategy 获取策略.
func (b *WeightedBalancer) GetStrategy() LoadBalanceStrategy {
	return StrategyWeighted
}

// LeastConnBalancer 最少连接负载均衡器.
type LeastConnBalancer struct {
	mu    sync.RWMutex
	nodes []*Node
}

// NewLeastConnBalancer 创建最少连接负载均衡器.
func NewLeastConnBalancer() *LeastConnBalancer {
	return &LeastConnBalancer{}
}

// Select 选择节点（最少连接）.
func (b *LeastConnBalancer) Select(nodes []*Node, key string) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	// 找到连接数最少的节点
	var selected *Node
	minConns := int(^uint(0) >> 1) // 最大int值

	for _, node := range nodes {
		if node.Connections < minConns {
			minConns = node.Connections
			selected = node
		}
	}

	if selected == nil {
		return nodes[0], nil
	}

	return selected, nil
}

// UpdateNodes 更新节点列表.
func (b *LeastConnBalancer) UpdateNodes(nodes []*Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes = nodes
}

// GetStrategy 获取策略.
func (b *LeastConnBalancer) GetStrategy() LoadBalanceStrategy {
	return StrategyLeastConn
}

// IPHashBalancer IP哈希负载均衡器.
type IPHashBalancer struct {
	mu    sync.RWMutex
	nodes []*Node
}

// NewIPHashBalancer 创建IP哈希负载均衡器.
func NewIPHashBalancer() *IPHashBalancer {
	return &IPHashBalancer{}
}

// Select 选择节点（IP哈希）.
func (b *IPHashBalancer) Select(nodes []*Node, key string) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	// 使用key的CRC32哈希值选择节点
	hash := crc32.ChecksumIEEE([]byte(key))
	idx := hash % uint32(len(nodes))
	return nodes[idx], nil
}

// UpdateNodes 更新节点列表.
func (b *IPHashBalancer) UpdateNodes(nodes []*Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes = nodes
}

// GetStrategy 获取策略.
func (b *IPHashBalancer) GetStrategy() LoadBalanceStrategy {
	return StrategyIPHash
}

// RandomBalancer 随机负载均衡器.
type RandomBalancer struct {
	mu    sync.RWMutex
	nodes []*Node
	rng   *rand.Rand
}

// NewRandomBalancer 创建随机负载均衡器.
func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select 选择节点（随机）.
func (b *RandomBalancer) Select(nodes []*Node, key string) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	idx := b.rng.Intn(len(nodes))
	return nodes[idx], nil
}

// UpdateNodes 更新节点列表.
func (b *RandomBalancer) UpdateNodes(nodes []*Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes = nodes
}

// GetStrategy 获取策略.
func (b *RandomBalancer) GetStrategy() LoadBalanceStrategy {
	return StrategyRandom
}

// CompositeBalancer 复合负载均衡器.
// 支持多种策略组合，例如：先按可用区过滤，再按权重选择.
type CompositeBalancer struct {
	mu        sync.RWMutex
	balancers []LoadBalancer
	nodes     []*Node
}

// NewCompositeBalancer 创建复合负载均衡器.
func NewCompositeBalancer(balancers []LoadBalancer) *CompositeBalancer {
	return &CompositeBalancer{
		balancers: balancers,
	}
}

// Select 选择节点（复合策略）.
func (b *CompositeBalancer) Select(nodes []*Node, key string) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	// 依次应用每个负载均衡器
	currentNodes := nodes
	for _, balancer := range b.balancers {
		if len(currentNodes) == 0 {
			break
		}

		node, err := balancer.Select(currentNodes, key)
		if err != nil {
			continue
		}

		// 如果只有一个负载均衡器，直接返回
		if len(b.balancers) == 1 {
			return node, nil
		}

		// 否则，将选中的节点作为下一轮的候选
		currentNodes = []*Node{node}
	}

	if len(currentNodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	return currentNodes[0], nil
}

// UpdateNodes 更新节点列表.
func (b *CompositeBalancer) UpdateNodes(nodes []*Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes = nodes
	for _, balancer := range b.balancers {
		balancer.UpdateNodes(nodes)
	}
}

// GetStrategy 获取策略.
func (b *CompositeBalancer) GetStrategy() LoadBalanceStrategy {
	return "composite"
}

// ZoneAwareBalancer 可用区感知负载均衡器.
type ZoneAwareBalancer struct {
	mu              sync.RWMutex
	zoneBalancers   map[string]LoadBalancer
	defaultBalancer LoadBalancer
	nodes           []*Node
}

// NewZoneAwareBalancer 创建可用区感知负载均衡器.
func NewZoneAwareBalancer(strategy LoadBalanceStrategy) *ZoneAwareBalancer {
	return &ZoneAwareBalancer{
		zoneBalancers:   make(map[string]LoadBalancer),
		defaultBalancer: NewLoadBalancer(strategy),
	}
}

// Select 选择节点（可用区感知）.
func (b *ZoneAwareBalancer) Select(nodes []*Node, key string) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	// 按可用区分组
	zoneNodes := make(map[string][]*Node)
	for _, node := range nodes {
		zone := node.Zone
		if zone == "" {
			zone = "default"
		}
		zoneNodes[zone] = append(zoneNodes[zone], node)
	}

	// 如果只有一个可用区，使用默认负载均衡器
	if len(zoneNodes) == 1 {
		for _, zoneNodeList := range zoneNodes {
			return b.defaultBalancer.Select(zoneNodeList, key)
		}
	}

	// 多个可用区：优先选择与key相同可用区的节点
	// 这里简化实现：随机选择一个可用区，然后在该可用区内负载均衡
	var zones []string
	for zone := range zoneNodes {
		zones = append(zones, zone)
	}

	selectedZone := zones[rand.Intn(len(zones))]
	zoneNodeList := zoneNodes[selectedZone]

	// 获取或创建该可用区的负载均衡器
	b.mu.RLock()
	balancer, ok := b.zoneBalancers[selectedZone]
	b.mu.RUnlock()

	if !ok {
		b.mu.Lock()
		balancer = b.defaultBalancer
		b.zoneBalancers[selectedZone] = balancer
		b.mu.Unlock()
	}

	return balancer.Select(zoneNodeList, key)
}

// UpdateNodes 更新节点列表.
func (b *ZoneAwareBalancer) UpdateNodes(nodes []*Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes = nodes

	// 更新每个可用区的负载均衡器
	zoneNodes := make(map[string][]*Node)
	for _, node := range nodes {
		zone := node.Zone
		if zone == "" {
			zone = "default"
		}
		zoneNodes[zone] = append(zoneNodes[zone], node)
	}

	for zone, zoneNodeList := range zoneNodes {
		balancer, ok := b.zoneBalancers[zone]
		if !ok {
			balancer = NewLoadBalancer(b.defaultBalancer.GetStrategy())
			b.zoneBalancers[zone] = balancer
		}
		balancer.UpdateNodes(zoneNodeList)
	}
}

// GetStrategy 获取策略.
func (b *ZoneAwareBalancer) GetStrategy() LoadBalanceStrategy {
	return "zone_aware"
}

// NewLoadBalancer 创建负载均衡器.
func NewLoadBalancer(strategy LoadBalanceStrategy) LoadBalancer {
	switch strategy {
	case StrategyRoundRobin:
		return NewRoundRobinBalancer()
	case StrategyWeighted:
		return NewWeightedBalancer()
	case StrategyLeastConn:
		return NewLeastConnBalancer()
	case StrategyIPHash:
		return NewIPHashBalancer()
	case StrategyRandom:
		return NewRandomBalancer()
	default:
		return NewRoundRobinBalancer()
	}
}

// LoadBalancerFactory 负载均衡器工厂.
type LoadBalancerFactory struct {
	mu        sync.RWMutex
	balancers map[LoadBalanceStrategy]LoadBalancer
}

// NewLoadBalancerFactory 创建负载均衡器工厂.
func NewLoadBalancerFactory() *LoadBalancerFactory {
	return &LoadBalancerFactory{
		balancers: make(map[LoadBalanceStrategy]LoadBalancer),
	}
}

// GetOrCreate 获取或创建负载均衡器.
func (f *LoadBalancerFactory) GetOrCreate(strategy LoadBalanceStrategy) LoadBalancer {
	f.mu.RLock()
	balancer, ok := f.balancers[strategy]
	f.mu.RUnlock()

	if ok {
		return balancer
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// 双重检查
	if balancer, ok := f.balancers[strategy]; ok {
		return balancer
	}

	balancer = NewLoadBalancer(strategy)
	f.balancers[strategy] = balancer
	return balancer
}

// LoadBalancerWithFallback 带降级的负载均衡器.
type LoadBalancerWithFallback struct {
	mu       sync.RWMutex
	primary  LoadBalancer
	fallback LoadBalancer
	nodes    []*Node
}

// NewLoadBalancerWithFallback 创建带降级的负载均衡器.
func NewLoadBalancerWithFallback(primary, fallback LoadBalancer) *LoadBalancerWithFallback {
	return &LoadBalancerWithFallback{
		primary:  primary,
		fallback: fallback,
	}
}

// Select 选择节点（带降级）.
func (b *LoadBalancerWithFallback) Select(nodes []*Node, key string) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	// 尝试主负载均衡器
	node, err := b.primary.Select(nodes, key)
	if err == nil {
		return node, nil
	}

	// 降级到备用负载均衡器
	return b.fallback.Select(nodes, key)
}

// UpdateNodes 更新节点列表.
func (b *LoadBalancerWithFallback) UpdateNodes(nodes []*Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes = nodes
	b.primary.UpdateNodes(nodes)
	b.fallback.UpdateNodes(nodes)
}

// GetStrategy 获取策略.
func (b *LoadBalancerWithFallback) GetStrategy() LoadBalanceStrategy {
	return b.primary.GetStrategy()
}

// LoadBalancerWithHealthCheck 带健康检查的负载均衡器.
type LoadBalancerWithHealthCheck struct {
	mu           sync.RWMutex
	inner        LoadBalancer
	nodes        []*Node
	healthyNodes []*Node
}

// NewLoadBalancerWithHealthCheck 创建带健康检查的负载均衡器.
func NewLoadBalancerWithHealthCheck(inner LoadBalancer) *LoadBalancerWithHealthCheck {
	return &LoadBalancerWithHealthCheck{
		inner: inner,
	}
}

// Select 选择节点（带健康检查）.
func (b *LoadBalancerWithHealthCheck) Select(nodes []*Node, key string) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	// 过滤健康节点
	b.mu.RLock()
	healthyNodes := b.healthyNodes
	b.mu.RUnlock()

	if len(healthyNodes) == 0 {
		// 如果没有健康节点，使用所有节点
		healthyNodes = nodes
	}

	return b.inner.Select(healthyNodes, key)
}

// UpdateNodes 更新节点列表.
func (b *LoadBalancerWithHealthCheck) UpdateNodes(nodes []*Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes = nodes
	b.healthyNodes = nodes
	b.inner.UpdateNodes(nodes)
}

// UpdateHealthyNodes 更新健康节点列表.
func (b *LoadBalancerWithHealthCheck) UpdateHealthyNodes(healthyNodes []*Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.healthyNodes = healthyNodes
	b.inner.UpdateNodes(healthyNodes)
}

// GetStrategy 获取策略.
func (b *LoadBalancerWithHealthCheck) GetStrategy() LoadBalanceStrategy {
	return b.inner.GetStrategy()
}
