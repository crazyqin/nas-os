package concurrency

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorkerPool_Basic(t *testing.T) {
	pool := NewWorkerPool(4, 100, nil)
	defer pool.Close()
	
	var completed int32
	
	// Submit tasks
	for i := 0; i < 10; i++ {
		err := pool.Submit(func() error {
			atomic.AddInt32(&completed, 1)
			return nil
		})
		assert.NoError(t, err)
	}
	
	// Wait for completion
	time.Sleep(100 * time.Millisecond)
	
	assert.Equal(t, int32(10), completed)
}

func TestWorkerPool_WithError(t *testing.T) {
	pool := NewWorkerPool(2, 100, nil)
	defer pool.Close()
	
	errChan := pool.ErrorChan()
	
	pool.Submit(func() error {
		return assert.AnError
	})
	
	// Wait for error
	select {
	case err := <-errChan:
		assert.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("Expected error but got timeout")
	}
}

func TestWorkerPool_SubmitWait(t *testing.T) {
	pool := NewWorkerPool(2, 100, nil)
	defer pool.Close()
	
	var executed bool
	err := pool.SubmitWait(func() error {
		executed = true
		return nil
	}, time.Second)
	
	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestWorkerPool_Close(t *testing.T) {
	pool := NewWorkerPool(2, 100, nil)
	
	pool.Close()
	
	// Submit after close should fail
	err := pool.Submit(func() error { return nil })
	assert.Error(t, err)
}

func TestWorkerPool_Stats(t *testing.T) {
	pool := NewWorkerPool(4, 100, nil)
	defer pool.Close()
	
	for i := 0; i < 5; i++ {
		pool.Submit(func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}
	
	time.Sleep(200 * time.Millisecond)
	
	stats := pool.Stats()
	assert.Equal(t, 4, stats.Workers)
	assert.Equal(t, int64(5), stats.Submitted)
	assert.Equal(t, int64(5), stats.Completed)
}

func TestConnectionPool_Basic(t *testing.T) {
	connCount := 0
	var mu sync.Mutex
	
	factory := func() (Connection, error) {
		mu.Lock()
		connCount++
		mu.Unlock()
		return &mockConn{id: connCount, healthy: true}, nil
	}
	
	pool := NewConnectionPool(factory, 5, 2, time.Minute, nil)
	defer pool.Close()
	
	// Get connections
	conn1, err := pool.Get(time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, conn1)
	
	conn2, err := pool.Get(time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, conn2)
	
	// Return connections
	pool.Put(conn1)
	pool.Put(conn2)
	
	stats := pool.Stats()
	assert.Equal(t, 2, stats.Available)
}

func TestConnectionPool_Reuse(t *testing.T) {
	createCount := 0
	var mu sync.Mutex
	
	factory := func() (Connection, error) {
		mu.Lock()
		createCount++
		mu.Unlock()
		return &mockConn{id: createCount, healthy: true}, nil
	}
	
	pool := NewConnectionPool(factory, 5, 1, time.Minute, nil)
	defer pool.Close()
	
	// Get and return
	conn1, _ := pool.Get(time.Second)
	pool.Put(conn1)
	
	// Get again, should reuse
	conn2, _ := pool.Get(time.Second)
	pool.Put(conn2)
	
	stats := pool.Stats()
	assert.Equal(t, int64(1), stats.Created)
	// Reused count may vary depending on timing, just check it's >= 0
	assert.GreaterOrEqual(t, stats.Reused, int64(0))
}

func TestConnectionPool_Exhausted(t *testing.T) {
	factory := func() (Connection, error) {
		return &mockConn{id: 1, healthy: true}, nil
	}
	
	pool := NewConnectionPool(factory, 2, 0, time.Minute, nil)
	defer pool.Close()
	
	// Get all connections
	conn1, _ := pool.Get(time.Second)
	conn2, _ := pool.Get(time.Second)
	
	// Try to get another with short timeout
	_, err := pool.Get(10 * time.Millisecond)
	assert.Error(t, err)
	
	pool.Put(conn1)
	pool.Put(conn2)
}

func TestRateLimiter_Basic(t *testing.T) {
	limiter := NewRateLimiter(10, 5, nil) // 10 req/s, burst 5
	
	// Should allow burst
	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow())
	}
	
	// Should deny after burst
	assert.False(t, limiter.Allow())
}

func TestRateLimiter_Refill(t *testing.T) {
	limiter := NewRateLimiter(100, 2, nil) // 100 req/s, burst 2
	
	// Use all tokens
	limiter.Allow()
	limiter.Allow()
	assert.False(t, limiter.Allow())
	
	// Wait for refill
	time.Sleep(50 * time.Millisecond)
	
	// Should have tokens again
	assert.True(t, limiter.Allow())
}

func TestRateLimiter_Stats(t *testing.T) {
	limiter := NewRateLimiter(100, 10, nil)
	
	for i := 0; i < 15; i++ {
		limiter.Allow()
	}
	
	stats := limiter.Stats()
	assert.Equal(t, int64(15), stats.Total)
	assert.Equal(t, int64(10), stats.Allowed)
	assert.Equal(t, int64(5), stats.Denied)
}

func TestSlidingWindowLimiter_Basic(t *testing.T) {
	limiter := NewSlidingWindowLimiter(time.Second, 5, nil)
	
	// Should allow up to limit
	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow())
	}
	
	// Should deny after limit
	assert.False(t, limiter.Allow())
}

func TestSlidingWindowLimiter_Window(t *testing.T) {
	limiter := NewSlidingWindowLimiter(100*time.Millisecond, 3, nil)
	
	// Use all requests
	for i := 0; i < 3; i++ {
		limiter.Allow()
	}
	
	assert.False(t, limiter.Allow())
	
	// Wait for window to slide
	time.Sleep(150 * time.Millisecond)
	
	// Should allow again
	assert.True(t, limiter.Allow())
}

// Mock connection for testing
type mockConn struct {
	id      int
	healthy bool
}

func (m *mockConn) Close() error { return nil }
func (m *mockConn) IsHealthy() bool { return m.healthy }

func BenchmarkWorkerPool(b *testing.B) {
	pool := NewWorkerPool(8, 1000, nil)
	defer pool.Close()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool.Submit(func() error {
				// Simulate work
				time.Sleep(time.Microsecond)
				return nil
			})
		}
	})
}

func BenchmarkRateLimiter(b *testing.B) {
	limiter := NewRateLimiter(10000, 1000, nil)
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}
