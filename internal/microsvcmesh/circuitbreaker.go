// Package microsvcmesh 熔断器，支持故障隔离和降级策略
package microsvcmesh

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	name          string
	config        *CircuitBreakerConfig
	state         CircuitState
	failures      int
	successes     int
	totalRequests int
	lastFailure   time.Time
	stateChanged  time.Time
	window        []requestRecord // 滑动窗口
}

// requestRecord 请求记录
type requestRecord struct {
	success   bool
	timestamp time.Time
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(logger *zap.Logger, name string, config *CircuitBreakerConfig) *CircuitBreaker {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		logger:       logger,
		name:         name,
		config:       config,
		state:        CircuitClosed,
		stateChanged: time.Now(),
		window:       make([]requestRecord, 0, config.WindowSize),
	}
}

// State 获取熔断器状态
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Allow 检查是否允许请求通过
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// 检查是否应该转为半开
		if time.Since(cb.stateChanged) > time.Duration(cb.config.Timeout)*time.Second {
			cb.transitionTo(CircuitHalfOpen)
			return true
		}
		return false
	case CircuitHalfOpen:
		// 半开状态下允许少量探测请求
		return true
	}

	return false
}

// RecordSuccess 记录请求成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successes++
	cb.totalRequests++
	cb.window = append(cb.window, requestRecord{success: true, timestamp: time.Now()})
	cb.trimWindow()

	switch cb.state {
	case CircuitHalfOpen:
		if cb.successes >= cb.config.SuccessThreshold {
			cb.transitionTo(CircuitClosed)
		}
	}
}

// RecordFailure 记录请求失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.totalRequests++
	cb.lastFailure = time.Now()
	cb.window = append(cb.window, requestRecord{success: false, timestamp: time.Now()})
	cb.trimWindow()

	switch cb.state {
	case CircuitClosed:
		if cb.shouldTrip() {
			cb.transitionTo(CircuitOpen)
		}
	case CircuitHalfOpen:
		// 半开状态下失败，立即回到熔断
		cb.transitionTo(CircuitOpen)
	}
}

// shouldTrip 检查是否应该触发熔断
func (cb *CircuitBreaker) shouldTrip() bool {
	// 条件1：连续失败超过阈值
	if cb.failures >= cb.config.FailureThreshold {
		return true
	}

	// 条件2：失败率超过阈值（需要足够样本）
	if cb.totalRequests >= cb.config.MinRequests {
		failureRate := float64(cb.failures) / float64(cb.totalRequests)
		if failureRate >= cb.config.FailureRate {
			return true
		}
	}

	return false
}

// transitionTo 状态转换
func (cb *CircuitBreaker) transitionTo(state CircuitState) {
	old := cb.state
	cb.state = state
	cb.stateChanged = time.Now()

	// 重置计数器
	if state == CircuitClosed {
		cb.failures = 0
		cb.successes = 0
		cb.totalRequests = 0
		cb.window = cb.window[:0]
	} else if state == CircuitOpen {
		cb.successes = 0
	} else if state == CircuitHalfOpen {
		cb.failures = 0
		cb.successes = 0
		cb.totalRequests = 0
	}

	cb.logger.Info("circuit breaker state changed",
		zap.String("name", cb.name),
		zap.String("from", string(old)),
		zap.String("to", string(state)),
	)
}

// trimWindow 裁剪滑动窗口
func (cb *CircuitBreaker) trimWindow() {
	if cb.config.WindowSize <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(cb.config.WindowSize) * time.Second)
	start := 0
	for i, r := range cb.window {
		if r.timestamp.After(cutoff) {
			start = i
			break
		}
		if i == len(cb.window)-1 {
			start = len(cb.window)
		}
	}
	if start > 0 {
		cb.window = cb.window[start:]
	}
}

// GetStats 获取熔断器统计
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":           cb.name,
		"state":          string(cb.state),
		"failures":       cb.failures,
		"successes":      cb.successes,
		"total_requests": cb.totalRequests,
		"last_failure":   cb.lastFailure,
		"state_changed":  cb.stateChanged,
	}
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.totalRequests = 0
	cb.window = cb.window[:0]
	cb.stateChanged = time.Now()
}
