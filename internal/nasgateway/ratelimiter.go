// Package nasgateway 提供限流器功能
package nasgateway

import (
	"sync"
	"time"
)

// RateLimiter 限流器.
type RateLimiter struct {
	mu        sync.RWMutex
	algorithm RateLimitAlgorithm
	limit     int
	burst     int
	window    time.Duration
	buckets   map[string]*tokenBucket
	windows   map[string]*slidingWindow
	fixedWins map[string]*fixedWindow
}

// tokenBucket 令牌桶.
type tokenBucket struct {
	tokens   float64
	lastTime time.Time
	rate     float64
	burst    int
}

// slidingWindow 滑动窗口.
type slidingWindow struct {
	windowStart time.Time
	count       int
	prevCount   int
	limit       int
	window      time.Duration
}

// fixedWindow 固定窗口.
type fixedWindow struct {
	windowStart time.Time
	count       int
	limit       int
	window      time.Duration
}

// NewRateLimiter 创建限流器.
func NewRateLimiter(algorithm RateLimitAlgorithm, limit int, burst int) *RateLimiter {
	if limit <= 0 {
		limit = 100
	}
	if burst <= 0 {
		burst = limit * 2
	}

	return &RateLimiter{
		algorithm: algorithm,
		limit:     limit,
		burst:     burst,
		window:    time.Second,
		buckets:   make(map[string]*tokenBucket),
		windows:   make(map[string]*slidingWindow),
		fixedWins: make(map[string]*fixedWindow),
	}
}

// Allow 检查是否允许请求.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	switch rl.algorithm {
	case AlgorithmTokenBucket:
		return rl.allowTokenBucket(key)
	case AlgorithmSlidingWindow:
		return rl.allowSlidingWindow(key)
	case AlgorithmFixedWindow:
		return rl.allowFixedWindow(key)
	case AlgorithmLeakyBucket:
		return rl.allowLeakyBucket(key)
	default:
		return rl.allowTokenBucket(key)
	}
}

// Reset 重置限流器.
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.buckets, key)
	delete(rl.windows, key)
	delete(rl.fixedWins, key)
}

// ResetAll 重置所有限流器.
func (rl *RateLimiter) ResetAll() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.buckets = make(map[string]*tokenBucket)
	rl.windows = make(map[string]*slidingWindow)
	rl.fixedWins = make(map[string]*fixedWindow)
}

// GetLimit 获取限制数.
func (rl *RateLimiter) GetLimit() int {
	return rl.limit
}

// GetBurst 获取突发限制.
func (rl *RateLimiter) GetBurst() int {
	return rl.burst
}

// GetAlgorithm 获取算法.
func (rl *RateLimiter) GetAlgorithm() RateLimitAlgorithm {
	return rl.algorithm
}

// GetCurrentCount 获取当前计数.
func (rl *RateLimiter) GetCurrentCount(key string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	switch rl.algorithm {
	case AlgorithmTokenBucket:
		if bucket, ok := rl.buckets[key]; ok {
			return int(bucket.burst - int(bucket.tokens))
		}
	case AlgorithmSlidingWindow:
		if window, ok := rl.windows[key]; ok {
			return window.count
		}
	case AlgorithmFixedWindow:
		if window, ok := rl.fixedWins[key]; ok {
			return window.count
		}
	}
	return 0
}

// allowTokenBucket 令牌桶算法.
func (rl *RateLimiter) allowTokenBucket(key string) bool {
	now := time.Now()

	bucket, exists := rl.buckets[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:   float64(rl.burst),
			lastTime: now,
			rate:     float64(rl.limit),
			burst:    rl.burst,
		}
		rl.buckets[key] = bucket
	}

	// 计算时间差，补充令牌
	elapsed := now.Sub(bucket.lastTime).Seconds()
	bucket.tokens += elapsed * bucket.rate
	if bucket.tokens > float64(bucket.burst) {
		bucket.tokens = float64(bucket.burst)
	}
	bucket.lastTime = now

	// 尝试消耗令牌
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// allowSlidingWindow 滑动窗口算法.
func (rl *RateLimiter) allowSlidingWindow(key string) bool {
	now := time.Now()

	window, exists := rl.windows[key]
	if !exists {
		window = &slidingWindow{
			windowStart: now,
			count:       0,
			prevCount:   0,
			limit:       rl.limit,
			window:      rl.window,
		}
		rl.windows[key] = window
	}

	// 检查是否需要移动窗口
	elapsed := now.Sub(window.windowStart)
	if elapsed >= window.window {
		// 计算新窗口的起始位置
		windowStart := window.windowStart
		for elapsed >= window.window {
			windowStart = windowStart.Add(window.window)
			elapsed = now.Sub(windowStart)
		}
		window.prevCount = window.count
		window.count = 0
		window.windowStart = windowStart
	}

	// 计算滑动窗口内的请求数
	weight := float64(now.Sub(window.windowStart)) / float64(window.window)
	estimated := float64(window.prevCount)*(1-weight) + float64(window.count)

	if int(estimated) < window.limit {
		window.count++
		return true
	}

	return false
}

// allowFixedWindow 固定窗口算法.
func (rl *RateLimiter) allowFixedWindow(key string) bool {
	now := time.Now()

	window, exists := rl.fixedWins[key]
	if !exists {
		window = &fixedWindow{
			windowStart: now,
			count:       0,
			limit:       rl.limit,
			window:      rl.window,
		}
		rl.fixedWins[key] = window
	}

	// 检查是否需要重置窗口
	if now.Sub(window.windowStart) >= window.window {
		window.windowStart = now
		window.count = 0
	}

	if window.count < window.limit {
		window.count++
		return true
	}

	return false
}

// allowLeakyBucket 漏桶算法.
func (rl *RateLimiter) allowLeakyBucket(key string) bool {
	// 漏桶算法实现
	now := time.Now()

	bucket, exists := rl.buckets[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:   0,
			lastTime: now,
			rate:     float64(rl.limit),
			burst:    rl.burst,
		}
		rl.buckets[key] = bucket
	}

	// 计算泄漏量
	elapsed := now.Sub(bucket.lastTime).Seconds()
	bucket.tokens -= elapsed * bucket.rate
	if bucket.tokens < 0 {
		bucket.tokens = 0
	}
	bucket.lastTime = now

	// 检查桶是否满
	if bucket.tokens < float64(bucket.burst) {
		bucket.tokens++
		return true
	}

	return false
}

// ========== 全局限流器管理 ==========

// GlobalRateLimiterManager 全局限流器管理器.
type GlobalRateLimiterManager struct {
	mu        sync.RWMutex
	limiters  map[string]*RateLimiter
	metrics   *RateLimitMetrics
}

// RateLimitMetrics 限流指标.
type RateLimitMetrics struct {
	TotalRequests int64 `json:"total_requests"`
	Allowed       int64 `json:"allowed"`
	Rejected      int64 `json:"rejected"`
}

// NewGlobalRateLimiterManager 创建全局限流器管理器.
func NewGlobalRateLimiterManager() *GlobalRateLimiterManager {
	return &GlobalRateLimiterManager{
		limiters: make(map[string]*RateLimiter),
		metrics:  &RateLimitMetrics{},
	}
}

// Add 添加限流器.
func (m *GlobalRateLimiterManager) Add(name string, limiter *RateLimiter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limiters[name] = limiter
}

// Remove 移除限流器.
func (m *GlobalRateLimiterManager) Remove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.limiters, name)
}

// Get 获取限流器.
func (m *GlobalRateLimiterManager) Get(name string) *RateLimiter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.limiters[name]
}

// Allow 检查是否允许请求.
func (m *GlobalRateLimiterManager) Allow(name, key string) bool {
	m.mu.RLock()
	limiter, exists := m.limiters[name]
	m.mu.RUnlock()

	if !exists {
		return true // 没有配置限流器，允许所有请求
	}

	m.mu.Lock()
	m.metrics.TotalRequests++
	m.mu.Unlock()

	allowed := limiter.Allow(key)

	m.mu.Lock()
	if allowed {
		m.metrics.Allowed++
	} else {
		m.metrics.Rejected++
	}
	m.mu.Unlock()

	return allowed
}

// GetMetrics 获取限流指标.
func (m *GlobalRateLimiterManager) GetMetrics() *RateLimitMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// ========== 限流键提取 ==========

// RateLimitKeyExtractor 限流键提取器.
type RateLimitKeyExtractor struct {
	keyType RateLimitKey
	keyName string
}

// NewRateLimitKeyExtractor 创建限流键提取器.
func NewRateLimitKeyExtractor(keyType RateLimitKey, keyName string) *RateLimitKeyExtractor {
	return &RateLimitKeyExtractor{
		keyType: keyType,
		keyName: keyName,
	}
}

// Extract 提取限流键.
func (e *RateLimitKeyExtractor) Extract(clientIP, userID, path string, headers map[string]string) string {
	switch e.keyType {
	case KeyIP:
		return clientIP
	case KeyUser:
		if userID != "" {
			return userID
		}
		return clientIP
	case KeyAPI:
		return path
	case KeyHeader:
		if e.keyName != "" {
			if val, ok := headers[e.keyName]; ok {
				return val
			}
		}
		return clientIP
	case KeyGlobal:
		return "global"
	default:
		return clientIP
	}
}
