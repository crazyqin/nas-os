// Package api 提供 API 限流中间件
package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	// 每秒请求数
	Rate float64
	// 突发容量
	Burst int
	// 窗口大小（用于滑动窗口算法）
	WindowSize time.Duration
	// 窗口内最大请求数
	MaxRequests int
	// 限流键提取函数
	KeyFunc func(*gin.Context) string
	// 超限时的消息
	Message string
}

// DefaultRateLimitConfig 默认限流配置
var DefaultRateLimitConfig = RateLimitConfig{
	Rate:        100,         // 100 req/s
	Burst:       200,         // 突发 200
	WindowSize:  time.Minute, // 1 分钟窗口
	MaxRequests: 1000,        // 每分钟最多 1000 请求
	Message:     "请求过于频繁，请稍后再试",
}

// RateLimiter 接口
type RateLimiter interface {
	Allow() bool
	Stats() RateLimiterStats
}

// RateLimiterStats 限流器统计
type RateLimiterStats struct {
	Total   int64
	Allowed int64
	Denied  int64
}

// TokenBucketLimiter 令牌桶限流器
type TokenBucketLimiter struct {
	rate       float64
	burst      int
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
	stats      RateLimiterStats
}

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(rate float64, burst int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
}

// Allow 检查是否允许请求
func (l *TokenBucketLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.stats.Total++

	// 根据时间补充令牌
	now := time.Now()
	elapsed := now.Sub(l.lastUpdate).Seconds()
	l.tokens += elapsed * l.rate
	if l.tokens > float64(l.burst) {
		l.tokens = float64(l.burst)
	}
	l.lastUpdate = now

	// 检查是否有令牌
	if l.tokens >= 1.0 {
		l.tokens--
		l.stats.Allowed++
		return true
	}

	l.stats.Denied++
	return false
}

// Stats 返回统计信息
func (l *TokenBucketLimiter) Stats() RateLimiterStats {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stats
}

// SlidingWindowLimiter 滑动窗口限流器
type SlidingWindowLimiter struct {
	windowSize  time.Duration
	maxRequests int
	requests    []time.Time
	mu          sync.Mutex
	stats       RateLimiterStats
}

// NewSlidingWindowLimiter 创建滑动窗口限流器
func NewSlidingWindowLimiter(windowSize time.Duration, maxRequests int) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		windowSize:  windowSize,
		maxRequests: maxRequests,
		requests:    make([]time.Time, 0, maxRequests),
	}
}

// Allow 检查是否允许请求
func (l *SlidingWindowLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.stats.Total++

	now := time.Now()
	windowStart := now.Add(-l.windowSize)

	// 移除过期请求
	valid := 0
	for _, t := range l.requests {
		if t.After(windowStart) {
			l.requests[valid] = t
			valid++
		}
	}
	l.requests = l.requests[:valid]

	// 检查是否在限制内
	if len(l.requests) < l.maxRequests {
		l.requests = append(l.requests, now)
		l.stats.Allowed++
		return true
	}

	l.stats.Denied++
	return false
}

// Stats 返回统计信息
func (l *SlidingWindowLimiter) Stats() RateLimiterStats {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stats
}

// RateLimiterStore 限流器存储
type RateLimiterStore struct {
	limiters sync.Map
	config   RateLimitConfig
}

// NewRateLimiterStore 创建限流器存储
func NewRateLimiterStore(config RateLimitConfig) *RateLimiterStore {
	return &RateLimiterStore{
		config: config,
	}
}

// Get 获取或创建限流器
func (s *RateLimiterStore) Get(key string) RateLimiter {
	if v, ok := s.limiters.Load(key); ok {
		return v.(RateLimiter)
	}

	limiter := NewTokenBucketLimiter(s.config.Rate, s.config.Burst)
	v, _ := s.limiters.LoadOrStore(key, limiter)
	return v.(RateLimiter)
}

// Cleanup 清理过期限流器
func (s *RateLimiterStore) Cleanup() {
	s.limiters.Range(func(key, value interface{}) bool {
		limiter := value.(RateLimiter)
		stats := limiter.Stats()
		// 如果长时间没有请求，删除限流器
		if stats.Total == stats.Allowed && stats.Total > 0 {
			s.limiters.Delete(key)
		}
		return true
	})
}

// DefaultKeyFunc 默认键提取函数（按 IP）
func DefaultKeyFunc(c *gin.Context) string {
	return c.ClientIP()
}

// RateLimit 限流中间件
func RateLimit(config RateLimitConfig) gin.HandlerFunc {
	if config.Rate == 0 {
		config = DefaultRateLimitConfig
	}
	if config.KeyFunc == nil {
		config.KeyFunc = DefaultKeyFunc
	}
	if config.Message == "" {
		config.Message = DefaultRateLimitConfig.Message
	}

	store := NewRateLimiterStore(config)

	return func(c *gin.Context) {
		key := config.KeyFunc(c)
		limiter := store.Get(key)

		if !limiter.Allow() {
			TooManyRequests(c, config.Message)
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitByUser 按用户限流中间件
func RateLimitByUser(rate float64, burst int) gin.HandlerFunc {
	config := RateLimitConfig{
		Rate:    rate,
		Burst:   burst,
		Message: "请求过于频繁，请稍后再试",
		KeyFunc: func(c *gin.Context) string {
			// 优先使用用户 ID
			if userID, exists := c.Get("user_id"); exists {
				return userID.(string)
			}
			// 否则使用 IP
			return c.ClientIP()
		},
	}
	return RateLimit(config)
}

// RateLimitByIP 按 IP 限流中间件
func RateLimitByIP(rate float64, burst int) gin.HandlerFunc {
	config := RateLimitConfig{
		Rate:    rate,
		Burst:   burst,
		Message: "请求过于频繁，请稍后再试",
		KeyFunc: DefaultKeyFunc,
	}
	return RateLimit(config)
}

// RateLimitByEndpoint 按端点限流中间件
func RateLimitByEndpoint(rate float64, burst int) gin.HandlerFunc {
	config := RateLimitConfig{
		Rate:    rate,
		Burst:   burst,
		Message: "该接口请求过于频繁，请稍后再试",
		KeyFunc: func(c *gin.Context) string {
			return c.ClientIP() + ":" + c.FullPath()
		},
	}
	return RateLimit(config)
}

// SlidingWindowRateLimit 滑动窗口限流中间件
func SlidingWindowRateLimit(windowSize time.Duration, maxRequests int) gin.HandlerFunc {
	store := &swLimiterStore{
		limiters: sync.Map{},
		window:   windowSize,
		maxReqs:  maxRequests,
	}

	return func(c *gin.Context) {
		key := c.ClientIP()
		limiter := store.Get(key)

		if !limiter.Allow() {
			TooManyRequests(c, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}

		c.Next()
	}
}

type swLimiterStore struct {
	limiters sync.Map
	window   time.Duration
	maxReqs  int
}

func (s *swLimiterStore) Get(key string) RateLimiter {
	if v, ok := s.limiters.Load(key); ok {
		return v.(RateLimiter)
	}

	limiter := NewSlidingWindowLimiter(s.window, s.maxReqs)
	v, _ := s.limiters.LoadOrStore(key, limiter)
	return v.(RateLimiter)
}

// IPRateLimiter IP 级别限流器
type IPRateLimiter struct {
	store *RateLimiterStore
}

// NewIPRateLimiter 创建 IP 限流器
func NewIPRateLimiter(rate float64, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		store: NewRateLimiterStore(RateLimitConfig{
			Rate:    rate,
			Burst:   burst,
			KeyFunc: DefaultKeyFunc,
		}),
	}
}

// Middleware 返回 Gin 中间件
func (l *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		limiter := l.store.Get(c.ClientIP())
		if !limiter.Allow() {
			TooManyRequests(c, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitHeaders 添加限流响应头
func RateLimitHeaders(rate, burst int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-RateLimit-Limit", intToStr(rate))
		c.Header("X-RateLimit-Burst", intToStr(burst))
		c.Next()
	}
}

func intToStr(n int) string {
	if n <= 0 {
		return "0"
	}
	// 简单转换
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

// ConditionalRateLimit 条件限流中间件
// 只对特定条件的请求进行限流
func ConditionalRateLimit(condition func(*gin.Context) bool, rate float64, burst int) gin.HandlerFunc {
	limiter := NewTokenBucketLimiter(rate, burst)

	return func(c *gin.Context) {
		if !condition(c) {
			c.Next()
			return
		}

		if !limiter.Allow() {
			TooManyRequests(c, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

// StrictRateLimit 严格限流（用于敏感接口）
func StrictRateLimit() gin.HandlerFunc {
	return RateLimitByIP(10, 20) // 10 req/s, burst 20
}

// NormalRateLimit 普通限流
func NormalRateLimit() gin.HandlerFunc {
	return RateLimitByIP(100, 200) // 100 req/s, burst 200
}

// RelaxedRateLimit 宽松限流
func RelaxedRateLimit() gin.HandlerFunc {
	return RateLimitByIP(1000, 2000) // 1000 req/s, burst 2000
}

// APIRateLimit API 限流中间件（根据路径自动选择）
func APIRateLimit() gin.HandlerFunc {
	strictPaths := map[string]bool{
		"/api/v1/auth/login":    true,
		"/api/v1/auth/register": true,
		"/api/v1/auth/reset":    true,
	}

	return func(c *gin.Context) {
		path := c.FullPath()

		if strictPaths[path] {
			StrictRateLimit()(c)
			return
		}

		NormalRateLimit()(c)
	}
}

// RateLimitStats 限流统计中间件
func RateLimitStats() gin.HandlerFunc {
	var (
		mu         sync.Mutex
		total      int64
		byEndpoint = make(map[string]int64)
		byIP       = make(map[string]int64)
	)

	return func(c *gin.Context) {
		c.Next()

		mu.Lock()
		defer mu.Unlock()

		total++
		byEndpoint[c.FullPath()]++
		byIP[c.ClientIP()]++
	}
}

// GetRateLimitStats 获取限流统计
func GetRateLimitStats() (total int64, byEndpoint, byIP map[string]int64) {
	// 这里应该从全局统计中获取
	// 简化实现
	return 0, nil, nil
}

// RateLimitExceededHandler 自定义限流处理
func RateLimitExceededHandler(handler func(*gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler(c)
	}
}

// RetryAfter 返回重试时间
func RetryAfter(seconds int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Retry-After", intToStr(seconds))
		c.Next()
	}
}

// RateLimitResponseHeaders 设置限流响应头
func RateLimitResponseHeaders(limit, remaining int, reset time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-RateLimit-Limit", intToStr(limit))
		c.Header("X-RateLimit-Remaining", intToStr(remaining))
		c.Header("X-RateLimit-Reset", intToStr(int(reset.Unix())))
		c.Next()
	}
}

// CombinedRateLimit 组合限流（IP + 用户）
func CombinedRateLimit(ipRate, userRate float64, ipBurst, userBurst int) gin.HandlerFunc {
	ipLimiter := NewTokenBucketLimiter(ipRate, ipBurst)
	userLimiters := NewRateLimiterStore(RateLimitConfig{
		Rate:  userRate,
		Burst: userBurst,
	})

	return func(c *gin.Context) {
		// 先检查 IP 限流
		if !ipLimiter.Allow() {
			TooManyRequests(c, "IP 请求过于频繁")
			c.Abort()
			return
		}

		// 再检查用户限流
		if userID, exists := c.Get("user_id"); exists {
			limiter := userLimiters.Get(userID.(string))
			if !limiter.Allow() {
				TooManyRequests(c, "用户请求过于频繁")
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// HealthCheckBypass 健康检查绕过限流
func HealthCheckBypass(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == path {
			c.Next()
			return
		}
		// 应用其他限流
		NormalRateLimit()(c)
	}
}

// RateLimitWithError 限流并返回详细错误
func RateLimitWithError(rate float64, burst int) gin.HandlerFunc {
	limiter := NewTokenBucketLimiter(rate, burst)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			stats := limiter.Stats()
			c.JSON(http.StatusTooManyRequests, Response{
				Code:    CodeTooManyRequests,
				Message: "请求过于频繁，请稍后再试",
				Data: map[string]interface{}{
					"retry_after": 1, // 1 秒后重试
					"total":       stats.Total,
					"denied":      stats.Denied,
				},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
