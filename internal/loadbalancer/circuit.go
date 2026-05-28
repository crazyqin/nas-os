// Package loadbalancer - 熔断器模式实现
package loadbalancer

import (
	"sync"
	"time"
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config CircuitBreakerConfig

	// 状态
	state     CircuitState
	failures  int
	successes int

	// 半开状态计数
	halfOpenRequests int

	// 打开时间
	openedAt time.Time

	// 统计
	totalRequests int64
	lastFailure   time.Time
	lastSuccess   time.Time

	mu sync.RWMutex
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
	}
}

// Execute 执行请求
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.config.Enabled {
		return fn()
	}

	// 检查是否允许执行
	if !cb.allow() {
		return ErrCircuitOpen
	}

	// 执行请求
	err := fn()

	// 记录结果
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

// allow 检查是否允许执行
func (cb *CircuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// 检查是否应该转为半开
		if time.Since(cb.openedAt) >= cb.config.Timeout {
			cb.state = CircuitHalfOpen
			cb.halfOpenRequests = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		// 半开状态下限制请求数
		cb.halfOpenRequests++
		return cb.halfOpenRequests <= cb.config.MaxRequests
	default:
		return false
	}
}

// recordSuccess 记录成功
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++
	cb.lastSuccess = time.Now()

	switch cb.state {
	case CircuitClosed:
		// 重置失败计数
		cb.failures = 0
	case CircuitHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			// 转为关闭状态
			cb.state = CircuitClosed
			cb.failures = 0
			cb.successes = 0
		}
	}
}

// recordFailure 记录失败
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++
	cb.lastFailure = time.Now()

	switch cb.state {
	case CircuitClosed:
		cb.failures++
		if cb.failures >= cb.config.FailureThreshold {
			// 转为打开状态
			cb.state = CircuitOpen
			cb.openedAt = time.Now()
		}
	case CircuitHalfOpen:
		// 半开状态下失败，直接转为打开
		cb.state = CircuitOpen
		cb.openedAt = time.Now()
		cb.successes = 0
	}
}

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 获取熔断器统计
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	stats := CircuitBreakerStats{
		State:         cb.state,
		Failures:      cb.failures,
		Successes:     cb.successes,
		TotalRequests: cb.totalRequests,
		LastFailure:   cb.lastFailure,
		LastSuccess:   cb.lastSuccess,
	}

	if cb.state == CircuitOpen {
		stats.OpenedAt = &cb.openedAt
	}

	return stats
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0
	cb.totalRequests = 0
	cb.openedAt = time.Time{}
	cb.lastFailure = time.Time{}
	cb.lastSuccess = time.Time{}
}

// ForceOpen 强制打开熔断器
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitOpen
	cb.openedAt = time.Now()
}

// ForceClose 强制关闭熔断器
func (cb *CircuitBreaker) ForceClose() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
}

// ============================================================
// 后端熔断器管理
// ============================================================

// BackendCircuitBreakers 后端熔断器管理
type BackendCircuitBreakers struct {
	config    CircuitBreakerConfig
	breakers  map[string]*CircuitBreaker
	mu        sync.RWMutex
}

// NewBackendCircuitBreakers 创建后端熔断器管理器
func NewBackendCircuitBreakers(config CircuitBreakerConfig) *BackendCircuitBreakers {
	return &BackendCircuitBreakers{
		config:   config,
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Get 获取后端熔断器
func (bcb *BackendCircuitBreakers) Get(backendID string) *CircuitBreaker {
	bcb.mu.RLock()
	breaker, exists := bcb.breakers[backendID]
	bcb.mu.RUnlock()

	if exists {
		return breaker
	}

	// 创建新的熔断器
	bcb.mu.Lock()
	defer bcb.mu.Unlock()

	// 双重检查
	if breaker, exists = bcb.breakers[backendID]; exists {
		return breaker
	}

	breaker = NewCircuitBreaker(bcb.config)
	bcb.breakers[backendID] = breaker
	return breaker
}

// Remove 移除后端熔断器
func (bcb *BackendCircuitBreakers) Remove(backendID string) {
	bcb.mu.Lock()
	defer bcb.mu.Unlock()
	delete(bcb.breakers, backendID)
}

// GetAll 获取所有后端熔断器统计
func (bcb *BackendCircuitBreakers) GetAll() map[string]CircuitBreakerStats {
	bcb.mu.RLock()
	defer bcb.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats, len(bcb.breakers))
	for id, breaker := range bcb.breakers {
		stats[id] = breaker.GetStats()
	}
	return stats
}

// ResetAll 重置所有熔断器
func (bcb *BackendCircuitBreakers) ResetAll() {
	bcb.mu.RLock()
	defer bcb.mu.RUnlock()

	for _, breaker := range bcb.breakers {
		breaker.Reset()
	}
}

// ============================================================
// 错误定义
// ============================================================

// ErrCircuitOpen 熔断器打开错误
var ErrCircuitOpen = &CircuitOpenError{}

// CircuitOpenError 熔断器打开错误类型
type CircuitOpenError struct{}

func (e *CircuitOpenError) Error() string {
	return "circuit breaker is open"
}

// IsCircuitOpen 检查是否为熔断器打开错误
func IsCircuitOpen(err error) bool {
	_, ok := err.(*CircuitOpenError)
	return ok
}
