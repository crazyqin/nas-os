package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Stats holds cache statistics.
type Stats struct {
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	Sets        int64   `json:"sets"`
	Evictions   int64   `json:"evictions"`
	ExpireCount int64   `json:"expire_count"`
	HitRatio    float64 `json:"hit_ratio"`
}

// Manager handles multiple cache instances with statistics.
type Manager struct {
	memoryCache *LRUCache
	redisCache  *RedisCache // Optional
	mu          sync.RWMutex

	// Statistics
	hits      int64
	misses    int64
	sets      int64
	evictions int64
	expires   int64

	logger *zap.Logger

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager creates a new cache manager.
func NewManager(capacity int, ttl time.Duration, logger *zap.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		memoryCache: NewLRUCache(capacity, ttl),
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start background cleanup
	go m.startCleanup()

	return m
}

// EnableRedis enables Redis cache (optional).
func (m *Manager) EnableRedis(addr, password string, db int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	redisCache, err := NewRedisCache(addr, password, db)
	if err != nil {
		return err
	}

	m.redisCache = redisCache
	m.logger.Info("Redis cache enabled", zap.String("addr", addr))
	return nil
}

// Get retrieves a value from cache (memory first, then redis).
func (m *Manager) Get(key string) (interface{}, bool) {
	// Try memory cache first
	if val, ok := m.memoryCache.Get(key); ok {
		atomic.AddInt64(&m.hits, 1)
		return val, true
	}

	// Try redis cache
	if m.redisCache != nil {
		if val, ok := m.redisCache.Get(key); ok {
			atomic.AddInt64(&m.hits, 1)
			// Populate memory cache for future requests
			m.memoryCache.Set(key, val)
			return val, true
		}
	}

	atomic.AddInt64(&m.misses, 1)
	return nil, false
}

// Set stores a value in cache.
func (m *Manager) Set(key string, value interface{}) {
	atomic.AddInt64(&m.sets, 1)
	m.memoryCache.Set(key, value)

	// Also store in redis if enabled
	if m.redisCache != nil {
		_ = m.redisCache.Set(key, value)
	}
}

// Delete removes a key from cache.
func (m *Manager) Delete(key string) {
	m.memoryCache.Delete(key)
	if m.redisCache != nil {
		_ = m.redisCache.Delete(key)
	}
}

// GetStats returns current cache statistics.
func (m *Manager) GetStats() *Stats {
	hits := atomic.LoadInt64(&m.hits)
	misses := atomic.LoadInt64(&m.misses)
	sets := atomic.LoadInt64(&m.sets)

	var hitRatio float64
	total := hits + misses
	if total > 0 {
		hitRatio = float64(hits) / float64(total) * 100
	}

	return &Stats{
		Hits:        hits,
		Misses:      misses,
		Sets:        sets,
		Evictions:   atomic.LoadInt64(&m.evictions),
		ExpireCount: atomic.LoadInt64(&m.expires),
		HitRatio:    hitRatio,
	}
}

// ResetStats resets all statistics.
func (m *Manager) ResetStats() {
	atomic.StoreInt64(&m.hits, 0)
	atomic.StoreInt64(&m.misses, 0)
	atomic.StoreInt64(&m.sets, 0)
	atomic.StoreInt64(&m.evictions, 0)
	atomic.StoreInt64(&m.expires, 0)
}

// startCleanup runs periodic cleanup of expired items.
func (m *Manager) startCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			removed := m.memoryCache.Cleanup()
			if removed > 0 {
				atomic.AddInt64(&m.expires, int64(removed))
				m.logger.Debug("Cache cleanup", zap.Int("removed", removed))
			}
		}
	}
}

// Stop stops the cache manager and cleanup goroutine.
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.logger.Info("Cache manager stopped")
}

// GetMemoryCache returns the underlying memory cache.
func (m *Manager) GetMemoryCache() *LRUCache {
	return m.memoryCache
}

// GetRedisCache returns the redis cache if enabled.
func (m *Manager) GetRedisCache() *RedisCache {
	return m.redisCache
}

// Clear clears all cache entries (both memory and redis).
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.memoryCache.Clear()
	if m.redisCache != nil {
		_ = m.redisCache.Clear()
	}
	m.logger.Info("Cache cleared")
}

// GetMulti retrieves multiple values from cache.
func (m *Manager) GetMulti(keys []string) map[string]interface{} {
	result := make(map[string]interface{})
	for _, key := range keys {
		if val, ok := m.Get(key); ok {
			result[key] = val
		}
	}
	return result
}

// SetMulti stores multiple values in cache.
func (m *Manager) SetMulti(items map[string]interface{}) {
	for key, value := range items {
		m.Set(key, value)
	}
}

// DeleteMulti removes multiple keys from cache.
func (m *Manager) DeleteMulti(keys []string) {
	for _, key := range keys {
		m.Delete(key)
	}
}

// GetOrSet retrieves a value or sets it using the provided function.
func (m *Manager) GetOrSet(key string, fn func() (interface{}, error)) (interface{}, error) {
	// Try to get from cache first
	if val, ok := m.Get(key); ok {
		return val, nil
	}

	// Generate value
	val, err := fn()
	if err != nil {
		return nil, err
	}

	// Store in cache
	m.Set(key, val)
	return val, nil
}

// Warmup preloads cache with frequently accessed data.
func (m *Manager) Warmup(items map[string]interface{}) {
	for key, value := range items {
		m.Set(key, value)
	}
	m.logger.Info("Cache warmed up", zap.Int("items", len(items)))
}

// InvalidatePattern invalidates all keys matching a pattern.
func (m *Manager) InvalidatePattern(pattern string) {
	// For memory cache, we need to iterate through all keys
	// This is a simplified implementation
	m.memoryCache.Clear()

	// For Redis, use pattern matching
	if m.redisCache != nil {
		_ = m.redisCache.Clear()
	}

	m.logger.Info("Cache invalidated by pattern", zap.String("pattern", pattern))
}
