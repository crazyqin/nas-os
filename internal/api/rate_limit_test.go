// Package api 提供 API 限流测试
package api

import (
	"sync"
	"testing"
	"time"
)

func TestTokenBucketLimiter(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5) // 10 req/s, burst 5

	// 初始突发应该全部通过
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// 突发后应该被限制
	if limiter.Allow() {
		t.Error("Expected request to be denied after burst")
	}

	// 等待令牌补充
	time.Sleep(200 * time.Millisecond)

	// 应该有新令牌
	if !limiter.Allow() {
		t.Error("Expected request to be allowed after waiting")
	}
}

func TestTokenBucketLimiterStats(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)

	// 允许 5 个请求
	for i := 0; i < 5; i++ {
		limiter.Allow()
	}

	stats := limiter.Stats()
	if stats.Total != 5 {
		t.Errorf("Expected total 5, got %d", stats.Total)
	}
	if stats.Allowed != 5 {
		t.Errorf("Expected allowed 5, got %d", stats.Allowed)
	}

	// 尝试更多请求（应该被拒绝）
	limiter.Allow()

	stats = limiter.Stats()
	if stats.Denied != 1 {
		t.Errorf("Expected denied 1, got %d", stats.Denied)
	}
}

func TestSlidingWindowLimiter(t *testing.T) {
	limiter := NewSlidingWindowLimiter(100*time.Millisecond, 3)

	// 应该允许 3 个请求
	for i := 0; i < 3; i++ {
		if !limiter.Allow() {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// 第 4 个应该被拒绝
	if limiter.Allow() {
		t.Error("Expected request to be denied")
	}

	// 等待窗口过去
	time.Sleep(150 * time.Millisecond)

	// 应该有新配额
	if !limiter.Allow() {
		t.Error("Expected request to be allowed after window")
	}
}

func TestSlidingWindowLimiterStats(t *testing.T) {
	limiter := NewSlidingWindowLimiter(time.Minute, 5)

	for i := 0; i < 7; i++ {
		limiter.Allow()
	}

	stats := limiter.Stats()
	if stats.Total != 7 {
		t.Errorf("Expected total 7, got %d", stats.Total)
	}
	if stats.Allowed != 5 {
		t.Errorf("Expected allowed 5, got %d", stats.Allowed)
	}
	if stats.Denied != 2 {
		t.Errorf("Expected denied 2, got %d", stats.Denied)
	}
}

func TestRateLimiterStore(t *testing.T) {
	config := RateLimitConfig{
		Rate:  10,
		Burst: 5,
	}
	store := NewRateLimiterStore(config)

	// 获取同一 key 应该返回同一限流器
	limiter1 := store.Get("key1")
	limiter2 := store.Get("key1")
	if limiter1 != limiter2 {
		t.Error("Expected same limiter for same key")
	}

	// 不同 key 应该返回不同限流器
	limiter3 := store.Get("key2")
	if limiter1 == limiter3 {
		t.Error("Expected different limiters for different keys")
	}
}

func TestRateLimiterStoreConcurrent(t *testing.T) {
	config := RateLimitConfig{
		Rate:  1000,
		Burst: 1000,
	}
	store := NewRateLimiterStore(config)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Get("concurrent-key")
		}()
	}
	wg.Wait()
}

func TestRateLimitConfigDefaults(t *testing.T) {
	// 测试默认配置
	if DefaultRateLimitConfig.Rate != 100 {
		t.Errorf("Expected default rate 100, got %f", DefaultRateLimitConfig.Rate)
	}
	if DefaultRateLimitConfig.Burst != 200 {
		t.Errorf("Expected default burst 200, got %d", DefaultRateLimitConfig.Burst)
	}
	if DefaultRateLimitConfig.MaxRequests != 1000 {
		t.Errorf("Expected default max requests 1000, got %d", DefaultRateLimitConfig.MaxRequests)
	}
}

func TestIPRateLimiter(t *testing.T) {
	limiter := NewIPRateLimiter(10, 5)
	if limiter == nil {
		t.Fatal("Expected IPRateLimiter, got nil")
	}

	// Middleware 应该返回函数
	middleware := limiter.Middleware()
	if middleware == nil {
		t.Error("Expected middleware function")
	}
}

func TestIntToStr(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{1234, "1234"},
	}

	for _, tt := range tests {
		result := intToStr(tt.input)
		if result != tt.expected {
			t.Errorf("intToStr(%d) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}
