// Package loadbalancer - 负载均衡器核心实现
package loadbalancer

import (
	"hash/fnv"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNoHealthyBackend 没有健康后端错误.
var ErrNoHealthyBackend = &NoHealthyBackendError{}

// NoHealthyBackendError 没有健康后端错误类型.
type NoHealthyBackendError struct{}

func (e *NoHealthyBackendError) Error() string {
	return "no healthy backend available"
}

// Balancer 负载均衡器核心.
type Balancer struct {
	config    LBConfig
	backends  []*Backend
	algorithm Algorithm

	// 轮询计数器
	roundRobinIndex uint64

	// 加权轮询状态
	weightedState *weightedRoundRobinState

	mu sync.RWMutex
}

// weightedRoundRobinState 加权轮询状态.
type weightedRoundRobinState struct {
	currentWeight int
	index         int
	gcdWeight     int
	maxWeight     int
}

// NewBalancer 创建负载均衡器.
func NewBalancer(config LBConfig) *Balancer {
	b := &Balancer{
		config:    config,
		algorithm: config.Algorithm,
	}

	// 初始化后端
	for _, bc := range config.Backends {
		backend := &Backend{
			ID:        bc.ID,
			Name:      bc.Name,
			URL:       bc.URL,
			Weight:    bc.Weight,
			MaxConns:  bc.MaxConns,
			Tags:      bc.Tags,
			IsHealthy: true,
			AddedAt:   time.Now(),
		}
		if backend.Weight <= 0 {
			backend.Weight = 1
		}
		b.backends = append(b.backends, backend)
	}

	// 初始化加权轮询
	if b.algorithm == AlgorithmWeightedRoundRobin {
		b.initWeightedRoundRobin()
	}

	return b
}

// initWeightedRoundRobin 初始化加权轮询.
func (b *Balancer) initWeightedRoundRobin() {
	if len(b.backends) == 0 {
		return
	}

	// 计算最大权重和GCD
	weights := make([]int, len(b.backends))
	for i, backend := range b.backends {
		weights[i] = backend.Weight
	}

	gcd := weights[0]
	maxW := weights[0]
	for _, w := range weights[1:] {
		gcd = gcdInt(gcd, w)
		if w > maxW {
			maxW = w
		}
	}

	b.weightedState = &weightedRoundRobinState{
		currentWeight: 0,
		index:         -1,
		gcdWeight:     gcd,
		maxWeight:     maxW,
	}
}

// Select 选择后端.
func (b *Balancer) Select(r *http.Request) (*Backend, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// 获取健康后端
	healthyBackends := b.getHealthyBackends()
	if len(healthyBackends) == 0 {
		return nil, ErrNoHealthyBackend
	}

	switch b.algorithm {
	case AlgorithmRoundRobin:
		return b.selectRoundRobin(healthyBackends), nil
	case AlgorithmWeightedRoundRobin:
		return b.selectWeightedRoundRobin(healthyBackends), nil
	case AlgorithmLeastConn:
		return b.selectLeastConn(healthyBackends), nil
	case AlgorithmIPHash:
		return b.selectIPHash(healthyBackends, r), nil
	default:
		return b.selectRoundRobin(healthyBackends), nil
	}
}

// getHealthyBackends 获取健康后端列表.
func (b *Balancer) getHealthyBackends() []*Backend {
	var healthy []*Backend
	for _, backend := range b.backends {
		if backend.IsHealthy {
			// 检查连接数限制
			if backend.MaxConns > 0 && backend.ActiveConns >= int64(backend.MaxConns) {
				continue
			}
			healthy = append(healthy, backend)
		}
	}
	return healthy
}

// selectRoundRobin 轮询选择.
func (b *Balancer) selectRoundRobin(backends []*Backend) *Backend {
	index := atomic.AddUint64(&b.roundRobinIndex, 1)
	return backends[index%uint64(len(backends))]
}

// selectWeightedRoundRobin 加权轮询选择 (平滑加权轮询 - Nginx算法).
func (b *Balancer) selectWeightedRoundRobin(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	if b.weightedState == nil {
		return backends[0]
	}

	state := b.weightedState

	// 查找下一个后端
	best := -1
	totalWeight := 0

	for i := 0; i < len(backends); i++ {
		state.index = (state.index + 1) % len(backends)
		backend := backends[state.index]

		// 第一次循环或权重>=currentWeight
		if best == -1 || backend.Weight >= backends[best].Weight {
			best = state.index
		}

		totalWeight += backend.Weight
	}

	if best >= 0 {
		return backends[best]
	}

	return backends[0]
}

// selectLeastConn 最少连接选择.
func (b *Balancer) selectLeastConn(backends []*Backend) *Backend {
	var selected *Backend
	minConns := int64(^uint64(0) >> 1) // MaxInt64

	for _, backend := range backends {
		conns := atomic.LoadInt64(&backend.ActiveConns)
		if conns < minConns {
			minConns = conns
			selected = backend
		}
	}

	return selected
}

// selectIPHash IP哈希选择.
func (b *Balancer) selectIPHash(backends []*Backend, r *http.Request) *Backend {
	ip := getClientIP(r)
	hash := fnv.New32a()
	hash.Write([]byte(ip))
	index := hash.Sum32() % uint32(len(backends))
	return backends[index]
}

// getClientIP 获取客户端IP.
func getClientIP(r *http.Request) string {
	// 优先从X-Forwarded-For获取
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// 取第一个IP
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// 从X-Real-IP获取
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// 从RemoteAddr获取
	return r.RemoteAddr
}

// AddBackend 添加后端.
func (b *Balancer) AddBackend(backend *Backend) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.backends = append(b.backends, backend)
	if b.algorithm == AlgorithmWeightedRoundRobin {
		b.initWeightedRoundRobin()
	}
}

// RemoveBackend 移除后端.
func (b *Balancer) RemoveBackend(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, backend := range b.backends {
		if backend.ID == id {
			b.backends = append(b.backends[:i], b.backends[i+1:]...)
			if b.algorithm == AlgorithmWeightedRoundRobin {
				b.initWeightedRoundRobin()
			}
			return true
		}
	}
	return false
}

// GetBackend 获取后端.
func (b *Balancer) GetBackend(id string) *Backend {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, backend := range b.backends {
		if backend.ID == id {
			return backend
		}
	}
	return nil
}

// GetBackends 获取所有后端.
func (b *Balancer) GetBackends() []*Backend {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*Backend, len(b.backends))
	copy(result, b.backends)
	return result
}

// GetHealthyBackends 获取所有健康后端.
func (b *Balancer) GetHealthyBackends() []*Backend {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getHealthyBackends()
}

// UpdateAlgorithm 更新负载均衡算法.
func (b *Balancer) UpdateAlgorithm(algo Algorithm) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.algorithm = algo
	if algo == AlgorithmWeightedRoundRobin {
		b.initWeightedRoundRobin()
	}
}

// UpdateBackendWeight 更新后端权重.
func (b *Balancer) UpdateBackendWeight(id string, weight int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, backend := range b.backends {
		if backend.ID == id {
			backend.mu.Lock()
			backend.Weight = weight
			backend.mu.Unlock()

			if b.algorithm == AlgorithmWeightedRoundRobin {
				b.initWeightedRoundRobin()
			}
			return true
		}
	}
	return false
}

// gcdInt 计算最大公约数.
func gcdInt(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
