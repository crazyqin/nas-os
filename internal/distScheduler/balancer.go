// Package distScheduler 负载均衡器，支持多节点任务分发
package distScheduler

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// Balancer 负载均衡器
type Balancer struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	config   *Config
	rrIndex  int64 // round-robin 计数器
}

// NewBalancer 创建负载均衡器
func NewBalancer(logger *zap.Logger, config *Config) *Balancer {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultConfig()
	}
	return &Balancer{
		logger: logger,
		config: config,
	}
}

// SelectNode 根据策略选择节点
func (b *Balancer) SelectNode(nodes map[string]*Node, task *Task, strategy Strategy) (string, error) {
	// 过滤可用节点
	available := make([]*Node, 0)
	for _, node := range nodes {
		if node.Status == NodeStatusOnline && node.TaskCount < b.config.MaxConcurrent {
			available = append(available, node)
		}
	}

	if len(available) == 0 {
		return "", fmt.Errorf("no available nodes")
	}

	switch strategy {
	case StrategyRoundRobin:
		return b.roundRobin(available), nil
	case StrategyLeastLoad:
		return b.leastLoad(available), nil
	case StrategyResource:
		return b.resourceAware(available, task), nil
	case StrategyAffinity:
		return b.affinity(available, task), nil
	case StrategyRandom:
		return b.randomPick(available), nil
	default:
		return b.leastLoad(available), nil
	}
}

// roundRobin 轮询策略
func (b *Balancer) roundRobin(nodes []*Node) string {
	idx := atomic.AddInt64(&b.rrIndex, 1)
	return nodes[int(idx)%len(nodes)].ID
}

// leastLoad 最小负载策略
func (b *Balancer) leastLoad(nodes []*Node) string {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].TaskCount < nodes[j].TaskCount
	})
	return nodes[0].ID
}

// resourceAware 资源感知策略
func (b *Balancer) resourceAware(nodes []*Node, task *Task) string {
	best := nodes[0]
	bestScore := b.computeResourceScore(best)

	for _, node := range nodes[1:] {
		score := b.computeResourceScore(node)
		if score > bestScore {
			best = node
			bestScore = score
		}
	}

	return best.ID
}

// computeResourceScore 计算节点资源评分
func (b *Balancer) computeResourceScore(node *Node) float64 {
	if node.Resources == nil {
		return 0.5
	}

	res := node.Resources
	score := 0.0
	count := 0

	if res.CPU.Total > 0 {
		score += res.CPU.Available / res.CPU.Total
		count++
	}
	if res.Memory.Total > 0 {
		score += res.Memory.Available / res.Memory.Total
		count++
	}

	if count > 0 {
		return score / float64(count)
	}
	return 0.5
}

// affinity 亲和性策略（优先选择相同标签的节点）
func (b *Balancer) affinity(nodes []*Node, task *Task) string {
	if task.Tags == nil || len(task.Tags) == 0 {
		return b.leastLoad(nodes)
	}

	best := nodes[0]
	bestMatch := 0

	for _, node := range nodes {
		match := 0
		for k, v := range task.Tags {
			if node.Tags[k] == v {
				match++
			}
		}
		if match > bestMatch {
			best = node
			bestMatch = match
		}
	}

	if bestMatch == 0 {
		return b.leastLoad(nodes)
	}
	return best.ID
}

// randomPick 随机选择
func (b *Balancer) randomPick(nodes []*Node) string {
	return nodes[rand.Intn(len(nodes))].ID
}

// GetLoadDistribution 获取负载分布
func (b *Balancer) GetLoadDistribution(nodes map[string]*Node) map[string]int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	dist := make(map[string]int)
	for id, node := range nodes {
		dist[id] = node.TaskCount
	}
	return dist
}

// IsBalanced 检查是否负载均衡
func (b *Balancer) IsBalanced(nodes map[string]*Node, threshold float64) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(nodes) == 0 {
		return true
	}

	counts := make([]int, 0, len(nodes))
	for _, node := range nodes {
		if node.Status == NodeStatusOnline {
			counts = append(counts, node.TaskCount)
		}
	}

	if len(counts) == 0 {
		return true
	}

	min, max := counts[0], counts[0]
	for _, c := range counts {
		if c < min {
			min = c
		}
		if c > max {
			max = c
		}
	}

	// 如果最大值为0，完全均衡
	if max == 0 {
		return true
	}

	// 差异在阈值范围内
	return float64(max-min)/float64(max) <= threshold
}
