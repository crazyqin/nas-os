// Package loadbalancer - 请求限流器实现
package loadbalancer

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter 请求限流器接口.
type RateLimiter interface {
	// Allow 检查请求是否允许
	Allow(key string) bool
	// AllowN 检查n个请求是否允许
	AllowN(key string, n int) bool
	// Reserve 预留一个令牌，返回等待时间
	Reserve(key string) time.Duration
	// GetLimit 获取限流结果
	GetLimit(key string) RateLimitResult
	// Reset 重置指定key的限流
	Reset(key string)
	// ResetAll 重置所有限流
	ResetAll()
}

// ============================================================
// 令牌桶限流器
// ============================================================

// TokenBucketLimiter 令牌桶限流器.
type TokenBucketLimiter struct {
	config  RateLimitConfig
	buckets map[string]*tokenBucket
	mu      sync.RWMutex
}

// tokenBucket 令牌桶.
type tokenBucket struct {
	tokens    float64
	maxTokens float64
	rate      float64 // 每秒生成的令牌数
	lastTime  time.Time
	mu        sync.Mutex
}

// NewTokenBucketLimiter 创建令牌桶限流器.
func NewTokenBucketLimiter(config RateLimitConfig) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		config:  config,
		buckets: make(map[string]*tokenBucket),
	}
}

// getBucket 获取或创建令牌桶.
func (tbl *TokenBucketLimiter) getBucket(key string) *tokenBucket {
	tbl.mu.RLock()
	bucket, exists := tbl.buckets[key]
	tbl.mu.RUnlock()

	if exists {
		return bucket
	}

	tbl.mu.Lock()
	defer tbl.mu.Unlock()

	// 双重检查
	if bucket, exists = tbl.buckets[key]; exists {
		return bucket
	}

	bucket = &tokenBucket{
		tokens:    float64(tbl.config.Burst),
		maxTokens: float64(tbl.config.Burst),
		rate:      float64(tbl.config.Rate),
		lastTime:  time.Now(),
	}
	tbl.buckets[key] = bucket
	return bucket
}

// Allow 检查请求是否允许.
func (tbl *TokenBucketLimiter) Allow(key string) bool {
	return tbl.AllowN(key, 1)
}

// AllowN 检查n个请求是否允许.
func (tbl *TokenBucketLimiter) AllowN(key string, n int) bool {
	bucket := tbl.getBucket(key)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// 计算新增令牌
	now := time.Now()
	elapsed := now.Sub(bucket.lastTime).Seconds()
	bucket.tokens += elapsed * bucket.rate
	if bucket.tokens > bucket.maxTokens {
		bucket.tokens = bucket.maxTokens
	}
	bucket.lastTime = now

	// 检查是否有足够的令牌
	if bucket.tokens >= float64(n) {
		bucket.tokens -= float64(n)
		return true
	}

	return false
}

// Reserve 预留令牌.
func (tbl *TokenBucketLimiter) Reserve(key string) time.Duration {
	bucket := tbl.getBucket(key)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// 计算新增令牌
	now := time.Now()
	elapsed := now.Sub(bucket.lastTime).Seconds()
	bucket.tokens += elapsed * bucket.rate
	if bucket.tokens > bucket.maxTokens {
		bucket.tokens = bucket.maxTokens
	}
	bucket.lastTime = now

	// 如果有令牌，立即返回
	if bucket.tokens >= 1 {
		bucket.tokens--
		return 0
	}

	// 计算等待时间
	deficit := 1 - bucket.tokens
	waitTime := time.Duration(deficit/bucket.rate*1000) * time.Millisecond
	bucket.tokens = 0

	return waitTime
}

// GetLimit 获取限流结果.
func (tbl *TokenBucketLimiter) GetLimit(key string) RateLimitResult {
	bucket := tbl.getBucket(key)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// 计算当前令牌数
	now := time.Now()
	elapsed := now.Sub(bucket.lastTime).Seconds()
	currentTokens := bucket.tokens + elapsed*bucket.rate
	if currentTokens > bucket.maxTokens {
		currentTokens = bucket.maxTokens
	}

	return RateLimitResult{
		Allowed:   currentTokens >= 1,
		Remaining: int(currentTokens),
	}
}

// Reset 重置指定key的限流.
func (tbl *TokenBucketLimiter) Reset(key string) {
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	delete(tbl.buckets, key)
}

// ResetAll 重置所有限流.
func (tbl *TokenBucketLimiter) ResetAll() {
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	tbl.buckets = make(map[string]*tokenBucket)
}

// ============================================================
// 滑动窗口限流器
// ============================================================

// SlidingWindowLimiter 滑动窗口限流器.
type SlidingWindowLimiter struct {
	config  RateLimitConfig
	windows map[string]*slidingWindow
	mu      sync.RWMutex
}

// slidingWindow 滑动窗口.
type slidingWindow struct {
	currentCount int
	prevCount    int
	windowStart  time.Time
	windowSize   time.Duration
	limit        int
	mu           sync.Mutex
}

// NewSlidingWindowLimiter 创建滑动窗口限流器.
func NewSlidingWindowLimiter(config RateLimitConfig) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		config:  config,
		windows: make(map[string]*slidingWindow),
	}
}

// getWindow 获取或创建滑动窗口.
func (swl *SlidingWindowLimiter) getWindow(key string) *slidingWindow {
	swl.mu.RLock()
	window, exists := swl.windows[key]
	swl.mu.RUnlock()

	if exists {
		return window
	}

	swl.mu.Lock()
	defer swl.mu.Unlock()

	// 双重检查
	if window, exists = swl.windows[key]; exists {
		return window
	}

	window = &slidingWindow{
		currentCount: 0,
		prevCount:    0,
		windowStart:  time.Now(),
		windowSize:   time.Second,
		limit:        swl.config.Rate,
	}
	swl.windows[key] = window
	return window
}

// Allow 检查请求是否允许.
func (swl *SlidingWindowLimiter) Allow(key string) bool {
	return swl.AllowN(key, 1)
}

// AllowN 检查n个请求是否允许.
func (swl *SlidingWindowLimiter) AllowN(key string, n int) bool {
	window := swl.getWindow(key)
	window.mu.Lock()
	defer window.mu.Unlock()

	// 更新窗口
	swl.updateWindow(window)

	// 计算当前请求数 (加权平均)
	elapsed := time.Since(window.windowStart).Seconds()
	weight := elapsed / window.windowSize.Seconds()
	if weight > 1 {
		weight = 1
	}

	currentCount := float64(window.prevCount)*(1-weight) + float64(window.currentCount)
	if int(currentCount)+n > window.limit {
		return false
	}

	window.currentCount += n
	return true
}

// updateWindow 更新窗口.
func (swl *SlidingWindowLimiter) updateWindow(window *slidingWindow) {
	now := time.Now()
	elapsed := now.Sub(window.windowStart)

	if elapsed >= window.windowSize {
		// 窗口已过期，重置
		window.prevCount = window.currentCount
		window.currentCount = 0
		window.windowStart = now
	}
}

// Reserve 预留请求.
func (swl *SlidingWindowLimiter) Reserve(key string) time.Duration {
	if swl.Allow(key) {
		return 0
	}
	// 等待到下一个窗口
	window := swl.getWindow(key)
	window.mu.Lock()
	defer window.mu.Unlock()

	return window.windowSize - time.Since(window.windowStart)
}

// GetLimit 获取限流结果.
func (swl *SlidingWindowLimiter) GetLimit(key string) RateLimitResult {
	window := swl.getWindow(key)
	window.mu.Lock()
	defer window.mu.Unlock()

	swl.updateWindow(window)

	elapsed := time.Since(window.windowStart).Seconds()
	weight := elapsed / window.windowSize.Seconds()
	if weight > 1 {
		weight = 1
	}

	currentCount := float64(window.prevCount)*(1-weight) + float64(window.currentCount)
	remaining := window.limit - int(currentCount)
	if remaining < 0 {
		remaining = 0
	}

	return RateLimitResult{
		Allowed:   remaining > 0,
		Remaining: remaining,
	}
}

// Reset 重置指定key的限流.
func (swl *SlidingWindowLimiter) Reset(key string) {
	swl.mu.Lock()
	defer swl.mu.Unlock()
	delete(swl.windows, key)
}

// ResetAll 重置所有限流.
func (swl *SlidingWindowLimiter) ResetAll() {
	swl.mu.Lock()
	defer swl.mu.Unlock()
	swl.windows = make(map[string]*slidingWindow)
}

// ============================================================
// HTTP限流中间件
// ============================================================

// RateLimitMiddleware HTTP限流中间件.
type RateLimitMiddleware struct {
	limiter RateLimiter
	config  RateLimitConfig
}

// NewRateLimitMiddleware 创建HTTP限流中间件.
func NewRateLimitMiddleware(config RateLimitConfig) *RateLimitMiddleware {
	var limiter RateLimiter

	switch config.Algorithm {
	case RateLimitSlidingWindow:
		limiter = NewSlidingWindowLimiter(config)
	default:
		limiter = NewTokenBucketLimiter(config)
	}

	return &RateLimitMiddleware{
		limiter: limiter,
		config:  config,
	}
}

// Handler HTTP中间件.
func (rlm *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rlm.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// 获取限流key
		key := rlm.getKey(r)

		// 检查限流
		if !rlm.limiter.Allow(key) {
			limit := rlm.limiter.GetLimit(key)
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rlm.config.Rate))
			w.Header().Set("X-RateLimit-Remaining", "0")
			if limit.RetryAfter > 0 {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", limit.RetryAfter/1000))
			}
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		// 设置限流头
		limit := rlm.limiter.GetLimit(key)
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rlm.config.Rate))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limit.Remaining))

		next.ServeHTTP(w, r)
	})
}

// getKey 获取限流key.
func (rlm *RateLimitMiddleware) getKey(r *http.Request) string {
	if rlm.config.ByIP {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
		return ip
	}
	return "global"
}

// ============================================================
// 限流器工厂
// ============================================================

// NewRateLimiter 创建限流器.
func NewRateLimiter(config RateLimitConfig) RateLimiter {
	switch config.Algorithm {
	case RateLimitSlidingWindow:
		return NewSlidingWindowLimiter(config)
	default:
		return NewTokenBucketLimiter(config)
	}
}

// ============================================================
// IP限流器 (防DDoS)
// ============================================================

// IPRateLimiter IP限流器.
type IPRateLimiter struct {
	limiter   RateLimiter
	config    RateLimitConfig
	blacklist map[string]time.Time
	mu        sync.RWMutex
}

// NewIPRateLimiter 创建IP限流器.
func NewIPRateLimiter(config RateLimitConfig) *IPRateLimiter {
	return &IPRateLimiter{
		limiter:   NewRateLimiter(config),
		config:    config,
		blacklist: make(map[string]time.Time),
	}
}

// Allow 检查IP请求是否允许.
func (irl *IPRateLimiter) Allow(ip string) bool {
	// 检查黑名单
	irl.mu.RLock()
	if expire, exists := irl.blacklist[ip]; exists {
		if time.Now().Before(expire) {
			irl.mu.RUnlock()
			return false
		}
		// 黑名单过期，移除
		irl.mu.RUnlock()
		irl.mu.Lock()
		delete(irl.blacklist, ip)
		irl.mu.Unlock()
		return irl.limiter.Allow(ip)
	}
	irl.mu.RUnlock()

	return irl.limiter.Allow(ip)
}

// BanIP 封禁IP.
func (irl *IPRateLimiter) BanIP(ip string, duration time.Duration) {
	irl.mu.Lock()
	defer irl.mu.Unlock()
	irl.blacklist[ip] = time.Now().Add(duration)
}

// UnbanIP 解封IP.
func (irl *IPRateLimiter) UnbanIP(ip string) {
	irl.mu.Lock()
	defer irl.mu.Unlock()
	delete(irl.blacklist, ip)
}

// IsBanned 检查IP是否被封禁.
func (irl *IPRateLimiter) IsBanned(ip string) bool {
	irl.mu.RLock()
	defer irl.mu.RUnlock()

	expire, exists := irl.blacklist[ip]
	if !exists {
		return false
	}

	if time.Now().Before(expire) {
		return true
	}

	// 过期了
	delete(irl.blacklist, ip)
	return false
}

// GetBlacklist 获取黑名单.
func (irl *IPRateLimiter) GetBlacklist() map[string]time.Time {
	irl.mu.RLock()
	defer irl.mu.RUnlock()

	result := make(map[string]time.Time, len(irl.blacklist))
	for ip, expire := range irl.blacklist {
		if time.Now().Before(expire) {
			result[ip] = expire
		}
	}
	return result
}
