package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLRUCache_Basic(t *testing.T) {
	cache := NewLRUCache(3, time.Minute)

	// Test Set and Get
	cache.Set("key1", "value1")
	val, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)

	// Test non-existent key
	_, ok = cache.Get("nonexistent")
	assert.False(t, ok)
}

func TestLRUCache_Capacity(t *testing.T) {
	cache := NewLRUCache(3, time.Minute)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	assert.Equal(t, 3, cache.Len())

	// Add one more, should evict oldest
	cache.Set("key4", "value4")
	assert.Equal(t, 3, cache.Len())

	// key1 should be evicted
	_, ok := cache.Get("key1")
	assert.False(t, ok)

	// Others should exist
	_, ok = cache.Get("key2")
	assert.True(t, ok)
}

func TestLRUCache_LRU(t *testing.T) {
	cache := NewLRUCache(3, time.Minute)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// Access key1 to make it most recently used
	cache.Get("key1")

	// Add new key, should evict key2 (least recently used)
	cache.Set("key4", "value4")

	_, ok := cache.Get("key1")
	assert.True(t, ok)

	_, ok = cache.Get("key2")
	assert.False(t, ok)
}

func TestLRUCache_TTL(t *testing.T) {
	cache := NewLRUCache(10, 100*time.Millisecond)

	cache.Set("key1", "value1")

	// Should exist immediately
	_, ok := cache.Get("key1")
	assert.True(t, ok)

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, ok = cache.Get("key1")
	assert.False(t, ok)
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(10, time.Minute)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	cache.Delete("key1")

	_, ok := cache.Get("key1")
	assert.False(t, ok)

	_, ok = cache.Get("key2")
	assert.True(t, ok)
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(10, time.Minute)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	cache.Clear()

	assert.Equal(t, 0, cache.Len())
}

func TestLRUCache_Concurrent(t *testing.T) {
	cache := NewLRUCache(1000, time.Minute)

	var wg sync.WaitGroup
	numGoroutines := 100
	numOps := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := string(rune('a'+id%26)) + string(rune('0'+j%10))
				cache.Set(key, j)
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Should not panic and should have reasonable size
	assert.Less(t, cache.Len(), 1000)
}

func TestManager_Basic(t *testing.T) {
	manager := NewManager(100, time.Minute, nil)

	manager.Set("key1", "value1")
	val, ok := manager.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)
}

func TestManager_Stats(t *testing.T) {
	manager := NewManager(100, time.Minute, nil)

	manager.Set("key1", "value1")
	manager.Get("key1") // hit
	manager.Get("key2") // miss

	stats := manager.GetStats()

	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(1), stats.Sets)
	assert.Greater(t, stats.HitRatio, 0.0)
}

func TestManager_ResetStats(t *testing.T) {
	manager := NewManager(100, time.Minute, nil)

	manager.Set("key1", "value1")
	manager.Get("key1")

	manager.ResetStats()

	stats := manager.GetStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
}

func BenchmarkLRUCache_Set(b *testing.B) {
	cache := NewLRUCache(10000, time.Minute)

	for i := 0; i < b.N; i++ {
		cache.Set(i, i)
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	cache := NewLRUCache(10000, time.Minute)

	// Pre-populate
	for i := 0; i < 10000; i++ {
		cache.Set(i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(i % 10000)
	}
}

func BenchmarkLRUCache_Concurrent(b *testing.B) {
	cache := NewLRUCache(10000, time.Minute)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(i, i)
			cache.Get(i)
			i++
		}
	})
}
