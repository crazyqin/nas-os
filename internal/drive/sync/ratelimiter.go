package sync

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// RateLimiter implements a token-bucket rate limiter for bandwidth control.
// Inspired by Synology Drive's bandwidth management.
type RateLimiter struct {
	mu       sync.Mutex
	bytesSec int64 // Max bytes per second, 0 = unlimited
	tokens   atomic.Int64
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(bytesPerSec int64) *RateLimiter {
	rl := &RateLimiter{
		bytesSec: bytesPerSec,
	}
	rl.tokens.Store(bytesPerSec)
	return rl
}

// Wait blocks until n bytes can be transferred, respecting context cancellation.
func (r *RateLimiter) Wait(ctx context.Context) error {
	if r.bytesSec <= 0 {
		return nil // Unlimited
	}

	// Simple approach: check rate limit, if exceeded wait briefly
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		current := r.tokens.Load()
		if current > 0 {
			if r.tokens.CompareAndSwap(current, current-1) {
				return nil
			}
			continue
		}

		// Refill tokens periodically
		r.mu.Lock()
		r.tokens.Store(r.bytesSec / 10) // Refill 1/10 per tick
		r.mu.Unlock()

		// Wait ~100ms for token refill
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// SetLimit updates the bandwidth limit.
func (r *RateLimiter) SetLimit(bytesPerSec int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bytesSec = bytesPerSec
	r.tokens.Store(bytesPerSec)
}

// GetLimit returns the current bandwidth limit.
func (r *RateLimiter) GetLimit() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bytesSec
}
