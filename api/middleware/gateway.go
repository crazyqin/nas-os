// Package middleware provides HTTP middleware for the API
// Version: 2.36.0 - API Gateway with Rate Limiting, Circuit Breaker, and Retry

package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ============================================================
// Rate Limiting Middleware
// ============================================================

// RateLimitConfig 配置限流中间件.
type RateLimitConfig struct {
	// RequestsPerSecond 每秒请求数
	RequestsPerSecond float64
	// Burst 突发容量
	Burst int
	// KeyFunc 用于生成限流键的函数，默认使用客户端 IP
	KeyFunc func(*gin.Context) string
	// OnLimited 当请求被限流时的处理函数
	OnLimited func(*gin.Context)
	// SkipCondition 跳过限流的条件
	SkipCondition func(*gin.Context) bool
}

// DefaultRateLimitConfig 默认限流配置.
var DefaultRateLimitConfig = RateLimitConfig{
	RequestsPerSecond: 100,
	Burst:             200,
	KeyFunc: func(c *gin.Context) string {
		return c.ClientIP()
	},
	OnLimited: func(c *gin.Context) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    429,
			"message": "rate limit exceeded, please try again later",
		})
		c.Abort()
	},
}

// RateLimiter 令牌桶限流器.
type RateLimiter struct {
	rate       float64   // 每秒令牌数
	burst      int       // 最大令牌数
	tokens     float64   // 当前令牌数
	lastUpdate time.Time // 上次更新时间
	mu         sync.Mutex

	// 统计
	totalRequests int64
	allowed       int64
	denied        int64
}

// NewRateLimiter 创建新的限流器.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
}

// Allow 检查是否允许请求.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.totalRequests++

	// 根据时间补充令牌
	now := time.Now()
	elapsed := now.Sub(r.lastUpdate).Seconds()
	r.tokens += elapsed * r.rate
	if r.tokens > float64(r.burst) {
		r.tokens = float64(r.burst)
	}
	r.lastUpdate = now

	// 检查是否有足够的令牌
	if r.tokens >= 1.0 {
		r.tokens--
		r.allowed++
		return true
	}

	r.denied++
	return false
}

// Stats 返回限流器统计信息.
func (r *RateLimiter) Stats() map[string]int64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return map[string]int64{
		"total_requests": r.totalRequests,
		"allowed":        r.allowed,
		"denied":         r.denied,
	}
}

// RateLimiterStore 限流器存储接口.
type RateLimiterStore interface {
	Get(key string) *RateLimiter
	Delete(key string)
	Clear()
	Stats() map[string]map[string]int64
}

// InMemoryRateLimiterStore 内存限流器存储.
type InMemoryRateLimiterStore struct {
	limiters map[string]*RateLimiter
	config   RateLimitConfig
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewInMemoryRateLimiterStore 创建内存限流器存储.
func NewInMemoryRateLimiterStore(config RateLimitConfig, logger *zap.Logger) *InMemoryRateLimiterStore {
	store := &InMemoryRateLimiterStore{
		limiters: make(map[string]*RateLimiter),
		config:   config,
		logger:   logger,
	}

	// 启动清理协程
	go store.cleanup()

	return store
}

// Get 获取或创建限流器.
func (s *InMemoryRateLimiterStore) Get(key string) *RateLimiter {
	s.mu.RLock()
	limiter, exists := s.limiters[key]
	s.mu.RUnlock()

	if exists {
		return limiter
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 双重检查
	if limiter, exists = s.limiters[key]; exists {
		return limiter
	}

	limiter = NewRateLimiter(s.config.RequestsPerSecond, s.config.Burst)
	s.limiters[key] = limiter
	return limiter
}

// Delete 删除限流器.
func (s *InMemoryRateLimiterStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.limiters, key)
}

// Clear 清空所有限流器.
func (s *InMemoryRateLimiterStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.limiters = make(map[string]*RateLimiter)
}

// Stats 返回所有限流器的统计信息.
func (s *InMemoryRateLimiterStore) Stats() map[string]map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]map[string]int64)
	for key, limiter := range s.limiters {
		stats[key] = limiter.Stats()
	}
	return stats
}

// cleanup 定期清理不活跃的限流器.
func (s *InMemoryRateLimiterStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		for key, limiter := range s.limiters {
			// 如果超过 10 分钟没有请求，删除限流器
			limiter.mu.Lock()
			inactive := time.Since(limiter.lastUpdate) > 10*time.Minute
			limiter.mu.Unlock()

			if inactive {
				delete(s.limiters, key)
			}
		}
		s.mu.Unlock()
	}
}

// RateLimitMiddleware 创建限流中间件.
func RateLimitMiddleware(config RateLimitConfig, logger *zap.Logger) gin.HandlerFunc {
	if config.RequestsPerSecond <= 0 {
		config.RequestsPerSecond = DefaultRateLimitConfig.RequestsPerSecond
	}
	if config.Burst <= 0 {
		config.Burst = DefaultRateLimitConfig.Burst
	}
	if config.KeyFunc == nil {
		config.KeyFunc = DefaultRateLimitConfig.KeyFunc
	}
	if config.OnLimited == nil {
		config.OnLimited = DefaultRateLimitConfig.OnLimited
	}

	store := NewInMemoryRateLimiterStore(config, logger)

	return func(c *gin.Context) {
		// 检查是否跳过限流
		if config.SkipCondition != nil && config.SkipCondition(c) {
			c.Next()
			return
		}

		key := config.KeyFunc(c)
		limiter := store.Get(key)

		if !limiter.Allow() {
			// 设置 RateLimit 响应头
			c.Header("X-RateLimit-Limit", strconv.Itoa(config.Burst))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))

			config.OnLimited(c)
			return
		}

		// 设置 RateLimit 响应头
		limiter.mu.Lock()
		remaining := int(limiter.tokens)
		limiter.mu.Unlock()
		c.Header("X-RateLimit-Limit", strconv.Itoa(config.Burst))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		c.Next()
	}
}

// ============================================================
// Circuit Breaker Middleware
// ============================================================

// CircuitState 熔断器状态.
type CircuitState int32

const (
	// StateClosed 关闭状态（正常）.
	StateClosed CircuitState = iota
	// StateOpen 开启状态（熔断）.
	StateOpen
	// StateHalfOpen 半开启状态（试探）.
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig 熔断器配置.
type CircuitBreakerConfig struct {
	// FailureThreshold 故障阈值，超过此数量触发熔断
	FailureThreshold int
	// SuccessThreshold 成功阈值，半开启状态下达到此数量转为关闭
	SuccessThreshold int
	// Timeout 熔断超时时间，超时后进入半开启状态
	Timeout time.Duration
	// HalfOpenRequests 半开启状态下允许的试探请求数
	HalfOpenRequests int
	// OnStateChange 状态变化回调
	OnStateChange func(from, to CircuitState)
	// OnError 错误判断函数，返回 true 表示需要计入故障
	OnError func(*gin.Context, error) bool
}

// DefaultCircuitBreakerConfig 默认熔断器配置.
var DefaultCircuitBreakerConfig = CircuitBreakerConfig{
	FailureThreshold: 5,
	SuccessThreshold: 3,
	Timeout:          30 * time.Second,
	HalfOpenRequests: 1,
	OnStateChange: func(from, to CircuitState) {
		// 默认状态变化日志
	},
	OnError: func(c *gin.Context, err error) bool {
		// 默认：所有错误都计入故障
		return true
	},
}

// CircuitBreaker 熔断器.
type CircuitBreaker struct {
	config CircuitBreakerConfig

	state           int32 // CircuitState
	failures        int32
	successes       int32
	requests        int32
	lastFailureTime time.Time
	mu              sync.RWMutex

	// 统计
	totalRequests   int64
	totalFailures   int64
	totalSuccesses  int64
	totalRejected   int64
	stateChanges    int64
	lastStateChange time.Time

	logger *zap.Logger
}

// NewCircuitBreaker 创建熔断器.
func NewCircuitBreaker(config CircuitBreakerConfig, logger *zap.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		config:          config,
		state:           int32(StateClosed),
		lastStateChange: time.Now(),
		logger:          logger,
	}
}

// State 获取当前状态.
func (cb *CircuitBreaker) State() CircuitState {
	return CircuitState(atomic.LoadInt32(&cb.state))
}

// setState 设置状态.
func (cb *CircuitBreaker) setState(newState CircuitState) {
	oldState := cb.State()
	if oldState == newState {
		return
	}

	atomic.StoreInt32(&cb.state, int32(newState))
	cb.mu.Lock()
	cb.lastStateChange = time.Now()
	cb.stateChanges++
	cb.mu.Unlock()

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(oldState, newState)
	}

	cb.logger.Info("Circuit breaker state changed",
		zap.String("from", oldState.String()),
		zap.String("to", newState.String()),
	)
}

// Allow 检查是否允许请求.
func (cb *CircuitBreaker) Allow() bool {
	state := cb.State()

	switch state {
	case StateClosed:
		return true

	case StateOpen:
		// 检查是否超时，超时则进入半开启状态
		cb.mu.RLock()
		since := time.Since(cb.lastFailureTime)
		cb.mu.RUnlock()

		if since >= cb.config.Timeout {
			cb.setState(StateHalfOpen)
			atomic.StoreInt32(&cb.successes, 0)
			return true
		}

		atomic.AddInt64(&cb.totalRejected, 1)
		return false

	case StateHalfOpen:
		// 半开启状态下限制请求数
		requests := atomic.AddInt32(&cb.requests, 1)
		if requests <= int32(cb.config.HalfOpenRequests) {
			return true
		}
		atomic.AddInt64(&cb.totalRejected, 1)
		return false

	default:
		return false
	}
}

// RecordSuccess 记录成功.
func (cb *CircuitBreaker) RecordSuccess() {
	atomic.AddInt64(&cb.totalRequests, 1)
	atomic.AddInt64(&cb.totalSuccesses, 1)

	state := cb.State()
	switch state {
	case StateClosed:
		// 关闭状态下重置故障计数
		atomic.StoreInt32(&cb.failures, 0)

	case StateHalfOpen:
		successes := atomic.AddInt32(&cb.successes, 1)
		if successes >= int32(cb.config.SuccessThreshold) {
			cb.setState(StateClosed)
			atomic.StoreInt32(&cb.failures, 0)
			atomic.StoreInt32(&cb.requests, 0)
		}
	}
}

// RecordFailure 记录故障.
func (cb *CircuitBreaker) RecordFailure() {
	atomic.AddInt64(&cb.totalRequests, 1)
	atomic.AddInt64(&cb.totalFailures, 1)

	state := cb.State()
	switch state {
	case StateClosed:
		failures := atomic.AddInt32(&cb.failures, 1)
		if failures >= int32(cb.config.FailureThreshold) {
			cb.mu.Lock()
			cb.lastFailureTime = time.Now()
			cb.mu.Unlock()
			cb.setState(StateOpen)
		}

	case StateHalfOpen:
		// 半开启状态下任何故障都会重新开启熔断
		cb.mu.Lock()
		cb.lastFailureTime = time.Now()
		cb.mu.Unlock()
		cb.setState(StateOpen)
	}
}

// Stats 返回熔断器统计信息.
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":             cb.State().String(),
		"failures":          atomic.LoadInt32(&cb.failures),
		"successes":         atomic.LoadInt32(&cb.successes),
		"total_requests":    atomic.LoadInt64(&cb.totalRequests),
		"total_failures":    atomic.LoadInt64(&cb.totalFailures),
		"total_successes":   atomic.LoadInt64(&cb.totalSuccesses),
		"total_rejected":    atomic.LoadInt64(&cb.totalRejected),
		"state_changes":     cb.stateChanges,
		"last_state_change": cb.lastStateChange,
	}
}

// Reset 重置熔断器.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.StoreInt32(&cb.state, int32(StateClosed))
	atomic.StoreInt32(&cb.failures, 0)
	atomic.StoreInt32(&cb.successes, 0)
	atomic.StoreInt32(&cb.requests, 0)
	cb.lastFailureTime = time.Time{}
}

// CircuitBreakerMiddleware 创建熔断器中间件.
func CircuitBreakerMiddleware(config CircuitBreakerConfig, logger *zap.Logger) gin.HandlerFunc {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = DefaultCircuitBreakerConfig.FailureThreshold
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = DefaultCircuitBreakerConfig.SuccessThreshold
	}
	if config.Timeout <= 0 {
		config.Timeout = DefaultCircuitBreakerConfig.Timeout
	}
	if config.OnStateChange == nil {
		config.OnStateChange = DefaultCircuitBreakerConfig.OnStateChange
	}
	if config.OnError == nil {
		config.OnError = DefaultCircuitBreakerConfig.OnError
	}

	cb := NewCircuitBreaker(config, logger)

	return func(c *gin.Context) {
		if !cb.Allow() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    503,
				"message": "service temporarily unavailable due to circuit breaker",
				"state":   cb.State().String(),
			})
			c.Abort()
			return
		}

		// 包装响应写入器以捕获错误
		blw := &bodyLogWriter{body: make([]byte, 0), ResponseWriter: c.Writer}
		c.Writer = blw

		c.Next()

		// 检查响应状态码
		if c.Writer.Status() >= 500 {
			cb.RecordFailure()
		} else {
			cb.RecordSuccess()
		}
	}
}

// bodyLogWriter 用于捕获响应体.
type bodyLogWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

// ============================================================
// Retry Middleware
// ============================================================

// RetryConfig 重试配置.
type RetryConfig struct {
	// MaxAttempts 最大尝试次数（包括首次请求）
	MaxAttempts int
	// InitialDelay 初始延迟
	InitialDelay time.Duration
	// MaxDelay 最大延迟
	MaxDelay time.Duration
	// Multiplier 延迟倍数
	Multiplier float64
	// RetryableStatusCodes 可重试的状态码
	RetryableStatusCodes []int
	// RetryableErrors 可重试的错误判断函数
	RetryableErrors func(error) bool
	// OnRetry 重试回调
	OnRetry func(attempt int, delay time.Duration, err error)
}

// DefaultRetryConfig 默认重试配置.
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:          3,
	InitialDelay:         100 * time.Millisecond,
	MaxDelay:             1 * time.Second,
	Multiplier:           2.0,
	RetryableStatusCodes: []int{502, 503, 504},
	RetryableErrors: func(err error) bool {
		return true // 默认所有错误都重试
	},
	OnRetry: func(attempt int, delay time.Duration, err error) {
		// 默认重试日志
	},
}

// RetryMiddleware 创建重试中间件.
func RetryMiddleware(config RetryConfig, logger *zap.Logger) gin.HandlerFunc {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = DefaultRetryConfig.MaxAttempts
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = DefaultRetryConfig.InitialDelay
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = DefaultRetryConfig.MaxDelay
	}
	if config.Multiplier <= 0 {
		config.Multiplier = DefaultRetryConfig.Multiplier
	}
	if len(config.RetryableStatusCodes) == 0 {
		config.RetryableStatusCodes = DefaultRetryConfig.RetryableStatusCodes
	}
	if config.RetryableErrors == nil {
		config.RetryableErrors = DefaultRetryConfig.RetryableErrors
	}
	if config.OnRetry == nil {
		config.OnRetry = DefaultRetryConfig.OnRetry
	}

	return func(c *gin.Context) {
		// 检查是否是幂等请求（GET, HEAD, OPTIONS, PUT, DELETE）
		method := c.Request.Method
		if method != http.MethodGet && method != http.MethodHead &&
			method != http.MethodOptions && method != http.MethodPut &&
			method != http.MethodDelete {
			// 非幂等请求不重试
			c.Next()
			return
		}

		// 重试逻辑
		var lastErr error
		delay := config.InitialDelay

		for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
			// 如果是重试，等待一段时间
			if attempt > 1 {
				config.OnRetry(attempt, delay, lastErr)
				time.Sleep(delay)
				delay = time.Duration(float64(delay) * config.Multiplier)
				if delay > config.MaxDelay {
					delay = config.MaxDelay
				}
			}

			// 执行请求
			c.Next()

			// 检查是否需要重试
			statusCode := c.Writer.Status()
			if !isRetryableStatus(statusCode, config.RetryableStatusCodes) {
				return
			}

			// 检查错误
			if len(c.Errors) > 0 {
				lastErr = c.Errors.Last().Err
				if !config.RetryableErrors(lastErr) {
					return
				}

				// 如果还有重试机会，重置上下文
				if attempt < config.MaxAttempts {
					logger.Warn("Retrying request",
						zap.Int("attempt", attempt),
						zap.Int("status", statusCode),
						zap.Error(lastErr),
					)
					// 注意：这里需要重新创建请求，但 Gin 不支持直接重试
					// 实际实现中可能需要使用反向代理或其他方式
				}
			} else {
				return
			}
		}
	}
}

// isRetryableStatus 检查状态码是否可重试.
func isRetryableStatus(status int, retryableCodes []int) bool {
	for _, code := range retryableCodes {
		if status == code {
			return true
		}
	}
	return false
}

// ============================================================
// API Gateway Middleware
// ============================================================

// GatewayConfig API 网关配置.
type GatewayConfig struct {
	// RateLimit 限流配置
	RateLimit RateLimitConfig
	// CircuitBreaker 熔断器配置
	CircuitBreaker CircuitBreakerConfig
	// Retry 重试配置
	Retry RetryConfig
	// EnableRateLimit 启用限流
	EnableRateLimit bool
	// EnableCircuitBreaker 启用熔断
	EnableCircuitBreaker bool
	// EnableRetry 启用重试
	EnableRetry bool
	// Timeout 请求超时
	Timeout time.Duration
}

// DefaultGatewayConfig 默认网关配置.
var DefaultGatewayConfig = GatewayConfig{
	RateLimit:            DefaultRateLimitConfig,
	CircuitBreaker:       DefaultCircuitBreakerConfig,
	Retry:                DefaultRetryConfig,
	EnableRateLimit:      true,
	EnableCircuitBreaker: true,
	EnableRetry:          true,
	Timeout:              30 * time.Second,
}

// GatewayMiddleware 创建 API 网关中间件.
func GatewayMiddleware(config GatewayConfig, logger *zap.Logger) gin.HandlerFunc {
	var middlewares []gin.HandlerFunc

	// 添加限流
	if config.EnableRateLimit {
		middlewares = append(middlewares, RateLimitMiddleware(config.RateLimit, logger))
	}

	// 添加熔断器
	if config.EnableCircuitBreaker {
		middlewares = append(middlewares, CircuitBreakerMiddleware(config.CircuitBreaker, logger))
	}

	// 添加重试
	if config.EnableRetry {
		middlewares = append(middlewares, RetryMiddleware(config.Retry, logger))
	}

	// 添加超时
	if config.Timeout > 0 {
		middlewares = append(middlewares, TimeoutMiddleware(config.Timeout, logger))
	}

	return func(c *gin.Context) {
		for _, mw := range middlewares {
			mw(c)
			if c.IsAborted() {
				return
			}
		}
	}
}

// TimeoutMiddleware 创建超时中间件.
func TimeoutMiddleware(timeout time.Duration, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		// 创建完成通道
		done := make(chan struct{})

		go func() {
			defer close(done)
			c.Next()
		}()

		select {
		case <-done:
			// 请求完成
		case <-ctx.Done():
			// 请求超时
			logger.Warn("Request timeout",
				zap.String("path", c.Request.URL.Path),
				zap.Duration("timeout", timeout),
			)
			c.JSON(http.StatusGatewayTimeout, gin.H{
				"code":    504,
				"message": "request timeout",
			})
			c.Abort()
		}
	}
}

// GatewayStats 网关统计.
type GatewayStats struct {
	mu sync.RWMutex

	rateLimitStats      map[string]map[string]int64
	circuitBreakerStats map[string]interface{}
	totalRequests       int64
	totalTimeouts       int64
	totalRetries        int64
}

// NewGatewayStats 创建网关统计.
func NewGatewayStats() *GatewayStats {
	return &GatewayStats{
		rateLimitStats:      make(map[string]map[string]int64),
		circuitBreakerStats: make(map[string]interface{}),
	}
}

// UpdateRateLimitStats 更新限流统计.
func (s *GatewayStats) UpdateRateLimitStats(stats map[string]map[string]int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rateLimitStats = stats
}

// UpdateCircuitBreakerStats 更新熔断器统计.
func (s *GatewayStats) UpdateCircuitBreakerStats(stats map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.circuitBreakerStats = stats
}

// GetStats 获取所有统计.
func (s *GatewayStats) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"rate_limit":      s.rateLimitStats,
		"circuit_breaker": s.circuitBreakerStats,
		"total_requests":  atomic.LoadInt64(&s.totalRequests),
		"total_timeouts":  atomic.LoadInt64(&s.totalTimeouts),
		"total_retries":   atomic.LoadInt64(&s.totalRetries),
	}
}

// GetGatewayStatusHandler 获取网关状态处理器.
func GetGatewayStatusHandler(store RateLimiterStore, cb *CircuitBreaker) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"rate_limit":      store.Stats(),
				"circuit_breaker": cb.Stats(),
			},
		})
	}
}

// MarshalJSON 实现 JSON 序列化.
func (s *GatewayStats) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.GetStats())
}

// ParseGatewayConfig 解析网关配置.
func ParseGatewayConfig(data []byte) (*GatewayConfig, error) {
	var config GatewayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse gateway config: %w", err)
	}
	return &config, nil
}

// Validate 验证网关配置.
func (c *GatewayConfig) Validate() error {
	if c.RateLimit.RequestsPerSecond <= 0 && c.EnableRateLimit {
		return fmt.Errorf("rate limit requests per second must be positive")
	}
	if c.CircuitBreaker.FailureThreshold <= 0 && c.EnableCircuitBreaker {
		return fmt.Errorf("circuit breaker failure threshold must be positive")
	}
	if c.Retry.MaxAttempts <= 0 && c.EnableRetry {
		return fmt.Errorf("retry max attempts must be positive")
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultGatewayConfig.Timeout
	}
	return nil
}
