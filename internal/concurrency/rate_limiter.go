package concurrency

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	rate       float64 // tokens per second
	burst      int     // max tokens
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex

	// Statistics
	total   int64
	allowed int64
	denied  int64

	logger *zap.Logger
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(rate float64, burst int, logger *zap.Logger) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: time.Now(),
		logger:     logger,
	}
}

// Allow checks if a request is allowed.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.total++

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastUpdate).Seconds()
	r.tokens += elapsed * r.rate
	if r.tokens > float64(r.burst) {
		r.tokens = float64(r.burst)
	}
	r.lastUpdate = now

	// Check if we have tokens
	if r.tokens >= 1.0 {
		r.tokens--
		r.allowed++
		return true
	}

	r.denied++
	return false
}

// Wait blocks until a token is available.
func (r *RateLimiter) Wait() {
	for !r.Allow() {
		time.Sleep(time.Millisecond * 10)
	}
}

// WaitTimeout waits for a token with timeout.
func (r *RateLimiter) WaitTimeout(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.Allow() {
			return true
		}
		time.Sleep(time.Millisecond * 10)
	}
	return false
}

// Stats returns limiter statistics.
func (r *RateLimiter) Stats() RateLimiterStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	return RateLimiterStats{
		Rate:    r.rate,
		Burst:   r.burst,
		Tokens:  r.tokens,
		Total:   r.total,
		Allowed: r.allowed,
		Denied:  r.denied,
	}
}

// RateLimiterStats holds rate limiter statistics.
type RateLimiterStats struct {
	Rate    float64 `json:"rate"`
	Burst   int     `json:"burst"`
	Tokens  float64 `json:"tokens"`
	Total   int64   `json:"total"`
	Allowed int64   `json:"allowed"`
	Denied  int64   `json:"denied"`
}

// SetRate updates the rate limit.
func (r *RateLimiter) SetRate(rate float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rate = rate
}

// SetBurst updates the burst size.
func (r *RateLimiter) SetBurst(burst int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.burst = burst
}

// Reset resets the limiter state.
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens = float64(r.burst)
	r.lastUpdate = time.Now()
	r.total = 0
	r.allowed = 0
	r.denied = 0
}

// SlidingWindowLimiter implements a sliding window rate limiter.
type SlidingWindowLimiter struct {
	windowSize  time.Duration
	maxRequests int
	requests    []time.Time
	mu          sync.Mutex

	// Statistics
	total   int64
	allowed int64
	denied  int64

	logger *zap.Logger
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter.
func NewSlidingWindowLimiter(
	windowSize time.Duration,
	maxRequests int,
	logger *zap.Logger,
) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		windowSize:  windowSize,
		maxRequests: maxRequests,
		requests:    make([]time.Time, 0, maxRequests),
		logger:      logger,
	}
}

// Allow checks if a request is allowed.
func (r *SlidingWindowLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.total++

	now := time.Now()
	windowStart := now.Add(-r.windowSize)

	// Remove old requests
	valid := 0
	for _, t := range r.requests {
		if t.After(windowStart) {
			r.requests[valid] = t
			valid++
		}
	}
	r.requests = r.requests[:valid]

	// Check if under limit
	if len(r.requests) < r.maxRequests {
		r.requests = append(r.requests, now)
		r.allowed++
		return true
	}

	r.denied++
	return false
}

// Stats returns limiter statistics.
func (r *SlidingWindowLimiter) Stats() SlidingWindowStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	return SlidingWindowStats{
		WindowSize:  r.windowSize,
		MaxRequests: r.maxRequests,
		Current:     len(r.requests),
		Total:       r.total,
		Allowed:     r.allowed,
		Denied:      r.denied,
	}
}

// SlidingWindowStats holds sliding window limiter statistics.
type SlidingWindowStats struct {
	WindowSize  time.Duration `json:"window_size"`
	MaxRequests int           `json:"max_requests"`
	Current     int           `json:"current"`
	Total       int64         `json:"total"`
	Allowed     int64         `json:"allowed"`
	Denied      int64         `json:"denied"`
}
