package security

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiter controls concurrent sync tasks and bandwidth.
type RateLimiter struct {
	mu             sync.Mutex
	maxConcurrent  int
	activeCount    int
	bandwidthLimit int64 // bytes per second, 0 = unlimited
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(maxConcurrent int, bandwidthLimitBps int64) *RateLimiter {
	return &RateLimiter{
		maxConcurrent:  maxConcurrent,
		bandwidthLimit: bandwidthLimitBps,
	}
}

// AcquireSlot attempts to acquire a sync slot. Returns error if at capacity.
func (rl *RateLimiter) AcquireSlot() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.activeCount >= rl.maxConcurrent {
		return fmt.Errorf("concurrent sync limit reached (%d/%d)", rl.activeCount, rl.maxConcurrent)
	}
	rl.activeCount++
	return nil
}

// ReleaseSlot releases a sync slot.
func (rl *RateLimiter) ReleaseSlot() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.activeCount > 0 {
		rl.activeCount--
	}
}

// ActiveCount returns the number of currently active syncs.
func (rl *RateLimiter) ActiveCount() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.activeCount
}

// TokenBucket for bandwidth limiting.
type TokenBucket struct {
	mu         sync.Mutex
	tokens     int64
	maxTokens  int64
	rate       int64 // tokens per second
	lastRefill time.Time
}

// NewTokenBucket creates a bandwidth token bucket.
func NewTokenBucket(rateBps int64) *TokenBucket {
	return &TokenBucket{
		tokens:     rateBps, // start full
		maxTokens:  rateBps,
		rate:       rateBps,
		lastRefill: time.Now(),
	}
}

// Allow checks if n bytes can be transferred, consuming tokens if allowed.
func (tb *TokenBucket) Allow(n int64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}
	return false
}

// Wait blocks until n bytes can be transferred.
func (tb *TokenBucket) Wait(n int64) {
	for {
		if tb.Allow(n) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += int64(elapsed * float64(tb.rate))
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now
}
