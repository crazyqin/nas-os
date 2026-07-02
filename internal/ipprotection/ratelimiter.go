package ipprotection

import (
	"sync"
	"time"
)

// ==================== 令牌桶限流器 ====================

// TokenBucket 令牌桶.
type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64   // 当前令牌数
	maxTokens  float64   // 桶容量
	refillRate float64   // 每秒补充令牌数
	lastRefill time.Time // 上次补充时间
}

// NewTokenBucket 创建令牌桶.
func NewTokenBucket(maxTokens int, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:     float64(maxTokens),
		maxTokens:  float64(maxTokens),
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow 尝试消费一个令牌，返回是否允许.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// AllowN 尝试消费 N 个令牌.
func (tb *TokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}
	return false
}

// refill 补充令牌.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now
}

// Tokens 返回当前令牌数.
func (tb *TokenBucket) Tokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	return tb.tokens
}

// Reset 重置令牌桶.
func (tb *TokenBucket) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.tokens = tb.maxTokens
	tb.lastRefill = time.Now()
}

// ==================== IP 限流管理器 ====================

// RateLimiterManager IP 级别限流管理器.
type RateLimiterManager struct {
	mu       sync.RWMutex
	buckets  map[string]*TokenBucket // IP -> TokenBucket
	config   *IPProtectionConfig
	stopChan chan struct{}
}

// NewRateLimiterManager 创建限流管理器.
func NewRateLimiterManager(config *IPProtectionConfig) *RateLimiterManager {
	if config == nil {
		config = DefaultIPProtectionConfig()
	}

	rlm := &RateLimiterManager{
		buckets:  make(map[string]*TokenBucket),
		config:   config,
		stopChan: make(chan struct{}),
	}

	// 启动清理协程
	go rlm.cleanupLoop()

	return rlm
}

// Allow 检查 IP 是否允许请求.
func (rlm *RateLimiterManager) Allow(ip string) bool {
	rlm.mu.Lock()
	bucket, exists := rlm.buckets[ip]
	if !exists {
		bucket = NewTokenBucket(rlm.config.RateLimitBurst, rlm.config.RateLimitRequestsPerSecond)
		rlm.buckets[ip] = bucket
	}
	rlm.mu.Unlock()

	return bucket.Allow()
}

// AllowN 检查 IP 是否允许 N 个请求.
func (rlm *RateLimiterManager) AllowN(ip string, n int) bool {
	rlm.mu.Lock()
	bucket, exists := rlm.buckets[ip]
	if !exists {
		bucket = NewTokenBucket(rlm.config.RateLimitBurst, rlm.config.RateLimitRequestsPerSecond)
		rlm.buckets[ip] = bucket
	}
	rlm.mu.Unlock()

	return bucket.AllowN(n)
}

// Tokens 返回 IP 当前令牌数.
func (rlm *RateLimiterManager) Tokens(ip string) float64 {
	rlm.mu.RLock()
	defer rlm.mu.RUnlock()

	if bucket, exists := rlm.buckets[ip]; exists {
		return bucket.Tokens()
	}
	return float64(rlm.config.RateLimitBurst)
}

// Reset 重置 IP 的限流状态.
func (rlm *RateLimiterManager) Reset(ip string) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	if bucket, exists := rlm.buckets[ip]; exists {
		bucket.Reset()
	}
}

// Remove 移除 IP 的限流桶.
func (rlm *RateLimiterManager) Remove(ip string) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	delete(rlm.buckets, ip)
}

// TrackedIPs 返回被跟踪的 IP 数量.
func (rlm *RateLimiterManager) TrackedIPs() int {
	rlm.mu.RLock()
	defer rlm.mu.RUnlock()

	return len(rlm.buckets)
}

// cleanupLoop 定期清理过期的令牌桶.
func (rlm *RateLimiterManager) cleanupLoop() {
	interval := rlm.config.RateLimitCleanupInterval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rlm.cleanup()
		case <-rlm.stopChan:
			return
		}
	}
}

// cleanup 清理长时间不活跃的桶.
func (rlm *RateLimiterManager) cleanup() {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	// 如果桶满令牌且超过清理间隔的 2 倍未使用，移除
	for ip, bucket := range rlm.buckets {
		if bucket.Tokens() >= bucket.maxTokens*0.99 {
			// 令牌几乎满了说明很久没用了
			delete(rlm.buckets, ip)
		}
	}
}

// Stop 停止限流管理器.
func (rlm *RateLimiterManager) Stop() {
	close(rlm.stopChan)
}
