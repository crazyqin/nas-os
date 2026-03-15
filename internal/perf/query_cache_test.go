// Package perf 提供查询缓存功能测试
package perf

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQueryCache(t *testing.T) {
	config := CacheConfig{
		MaxItems:     100,
		MaxMemory:    1024 * 1024,
		DefaultTTL:   time.Minute,
	}

	cache := NewQueryCache(config)
	require.NotNil(t, cache)
	assert.Equal(t, 100, cache.config.MaxItems)
}

func TestNewQueryCache_DefaultConfig(t *testing.T) {
	cache := NewQueryCache(CacheConfig{})
	require.NotNil(t, cache)
	assert.Equal(t, DefaultCacheConfig.MaxItems, cache.config.MaxItems)
	assert.Equal(t, DefaultCacheConfig.MaxMemory, cache.config.MaxMemory)
	assert.Equal(t, DefaultCacheConfig.DefaultTTL, cache.config.DefaultTTL)
}

func TestQueryCache_Set_Get(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0, // 禁用自动清理
	})

	// 设置缓存
	err := cache.Set("key1", "value1")
	require.NoError(t, err)

	// 获取缓存
	value, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", value)

	// 获取不存在的缓存
	_, ok = cache.Get("nonexistent")
	assert.False(t, ok)
}

func TestQueryCache_Set_WithTTL(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	err := cache.Set("key1", "value1", time.Second)
	require.NoError(t, err)

	value, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", value)

	// 等待过期
	time.Sleep(time.Second + time.Millisecond*100)

	_, ok = cache.Get("key1")
	assert.False(t, ok, "缓存应该已过期")
}

func TestQueryCache_GetWithTTL(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	ttl := time.Minute * 5
	err := cache.Set("key1", "value1", ttl)
	require.NoError(t, err)

	value, remainingTTL, ok := cache.GetWithTTL("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", value)
	assert.GreaterOrEqual(t, remainingTTL.Seconds(), (time.Minute*4).Seconds())
	assert.LessOrEqual(t, remainingTTL.Seconds(), ttl.Seconds())
}

func TestQueryCache_Delete(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")

	// 删除存在的缓存
	deleted := cache.Delete("key1")
	assert.True(t, deleted)

	_, ok := cache.Get("key1")
	assert.False(t, ok)

	// 删除不存在的缓存
	deleted = cache.Delete("key1")
	assert.False(t, deleted)
}

func TestQueryCache_Clear(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	assert.Equal(t, 3, cache.Size())

	cache.Clear()

	assert.Equal(t, 0, cache.Size())
}

func TestQueryCache_LRUEviction(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   3,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// 访问 key1，使其成为最近使用
	cache.Get("key1")

	// 添加第4个，应该淘汰 key2（最久未使用）
	cache.Set("key4", "value4")

	_, ok := cache.Get("key1")
	assert.True(t, ok, "key1 应该存在")

	_, ok = cache.Get("key2")
	assert.False(t, ok, "key2 应该被淘汰")

	_, ok = cache.Get("key3")
	assert.True(t, ok, "key3 应该存在")

	_, ok = cache.Get("key4")
	assert.True(t, ok, "key4 应该存在")
}

func TestQueryCache_MemoryEviction(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  100, // 非常小的内存限制
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	// 设置会触发内存淘汰
	cache.Set("key1", "this is a longer string to use memory")
	cache.Set("key2", "another string")

	// 因为内存限制，应该有淘汰发生
	stats := cache.Stats()
	assert.GreaterOrEqual(t, stats.Evictions, int64(1))
}

func TestQueryCache_Cleanup(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Millisecond * 100,
		CleanupInterval: 0, // 手动清理
	})

	cache.Set("key1", "value1", time.Millisecond*50)
	cache.Set("key2", "value2", time.Millisecond*200)

	// 等待 key1 过期
	time.Sleep(time.Millisecond * 100)

	// 清理过期条目
	cleaned := cache.Cleanup()
	assert.Equal(t, 1, cleaned)

	_, ok := cache.Get("key1")
	assert.False(t, ok, "key1 应该已被清理")

	_, ok = cache.Get("key2")
	assert.True(t, ok, "key2 应该仍然存在")
}

func TestQueryCache_Has(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")

	assert.True(t, cache.Has("key1"))
	assert.False(t, cache.Has("nonexistent"))

	// 测试过期情况
	cache.Set("key2", "value2", time.Millisecond*50)
	time.Sleep(time.Millisecond * 100)
	assert.False(t, cache.Has("key2"))
}

func TestQueryCache_Keys(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	keys := cache.Keys()
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "key1")
	assert.Contains(t, keys, "key2")
	assert.Contains(t, keys, "key3")
}

func TestQueryCache_Size_Memory(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	assert.Equal(t, 2, cache.Size())
	assert.Greater(t, cache.Memory(), int64(0))
}

func TestQueryCache_Stats(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:     100,
		MaxMemory:    1024 * 1024,
		DefaultTTL:   time.Minute,
		EnableStats:  true,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")

	// 命中
	cache.Get("key1")

	// 未命中
	cache.Get("nonexistent")

	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(1), stats.TotalSets)
	assert.Equal(t, int64(1), stats.TotalItems)
	assert.Greater(t, stats.HitRate(), 0.0)
}

func TestQueryCache_HitRate(t *testing.T) {
	stats := CacheStats{
		Hits:   8,
		Misses: 2,
	}

	assert.InDelta(t, 0.8, stats.HitRate(), 0.01)

	// 无请求时
	emptyStats := CacheStats{}
	assert.Equal(t, 0.0, emptyStats.HitRate())
}

func TestQueryCache_SetWithTags(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	err := cache.SetWithTags("key1", "value1", []string{"tag1", "tag2"})
	require.NoError(t, err)

	item, ok := cache.GetItem("key1")
	assert.True(t, ok)
	assert.Contains(t, item.Tags, "tag1")
	assert.Contains(t, item.Tags, "tag2")
}

func TestQueryCache_InvalidateByTag(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.SetWithTags("key1", "value1", []string{"tag1", "tag2"})
	cache.SetWithTags("key2", "value2", []string{"tag1"})
	cache.SetWithTags("key3", "value3", []string{"tag3"})

	// 按标签失效
	count := cache.InvalidateByTag("tag1")
	assert.Equal(t, 2, count)

	_, ok := cache.Get("key1")
	assert.False(t, ok)

	_, ok = cache.Get("key2")
	assert.False(t, ok)

	_, ok = cache.Get("key3")
	assert.True(t, ok)
}

func TestQueryCache_InvalidateByTags(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.SetWithTags("key1", "value1", []string{"tag1"})
	cache.SetWithTags("key2", "value2", []string{"tag2"})
	cache.SetWithTags("key3", "value3", []string{"tag3"})

	count := cache.InvalidateByTags([]string{"tag1", "tag2"})
	assert.Equal(t, 2, count)
}

func TestQueryCache_GetOrSet(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	callCount := 0
	fn := func() (interface{}, error) {
		callCount++
		return "computed_value", nil
	}

	// 第一次应该调用函数
	value, err := cache.GetOrSet("key1", fn)
	require.NoError(t, err)
	assert.Equal(t, "computed_value", value)
	assert.Equal(t, 1, callCount)

	// 第二次应该从缓存获取
	value, err = cache.GetOrSet("key1", fn)
	require.NoError(t, err)
	assert.Equal(t, "computed_value", value)
	assert.Equal(t, 1, callCount, "不应该再次调用函数")
}

func TestQueryCache_GetItem(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")

	item, ok := cache.GetItem("key1")
	assert.True(t, ok)
	assert.Equal(t, "key1", item.Key)
	assert.Equal(t, "value1", item.Value)
	assert.Greater(t, item.Size, int64(0))
	assert.False(t, item.CreatedAt.IsZero())

	// 不存在的条目
	_, ok = cache.GetItem("nonexistent")
	assert.False(t, ok)
}

func TestQueryCache_EstimateSize(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected int64
	}{
		{nil, 0},
		{"hello", 5},
		{[]byte("hello"), 5},
		{42, 8},
		{3.14, 8},
		{true, 8},
	}

	for _, tt := range tests {
		size := estimateSize(tt.value)
		assert.Equal(t, tt.expected, size)
	}
}

func TestQueryCache_Export(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	entries := cache.Export()
	assert.Len(t, entries, 2)

	// 验证导出的条目
	keys := make(map[string]bool)
	for _, entry := range entries {
		keys[entry.Key] = true
		assert.Greater(t, entry.Size, int64(0))
		assert.GreaterOrEqual(t, entry.TTL, int64(0))
	}
	assert.True(t, keys["key1"])
	assert.True(t, keys["key2"])
}

func TestQueryCache_Warmup(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	items := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	count := cache.Warmup(items, time.Minute)
	assert.Equal(t, 3, count)

	assert.Equal(t, 3, cache.Size())
}

func TestQueryCache_ResetStats(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:     100,
		MaxMemory:    1024 * 1024,
		DefaultTTL:   time.Minute,
		EnableStats:  true,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")
	cache.Get("key1")
	cache.Get("nonexistent")

	stats := cache.Stats()
	assert.Greater(t, stats.Hits, int64(0))
	assert.Greater(t, stats.Misses, int64(0))

	cache.ResetStats()

	stats = cache.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	// TotalItems 应该保留
	assert.Equal(t, int64(1), stats.TotalItems)
}

func TestQueryCache_Overwrite(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   100,
		MaxMemory:  1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	cache.Set("key1", "value1")
	cache.Set("key1", "value2") // 覆盖

	value, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value2", value)
	assert.Equal(t, 1, cache.Size())
}

func TestQueryCache_Concurrent(t *testing.T) {
	cache := NewQueryCache(CacheConfig{
		MaxItems:   1000,
		MaxMemory:  10 * 1024 * 1024,
		DefaultTTL: time.Minute,
		CleanupInterval: 0,
	})

	done := make(chan bool)

	// 并发写入
	for i := 0; i < 10; i++ {
		go func(idx int) {
			for j := 0; j < 100; j++ {
				key := string(rune('A' + idx))
				cache.Set(key, idx*100+j)
			}
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cache.Get(string(rune('A' + j%10)))
			}
			done <- true
		}(i)
	}

	// 等待所有协程完成
	for i := 0; i < 15; i++ {
		<-done
	}

	// 应该没有崩溃，缓存正常工作
	assert.LessOrEqual(t, cache.Size(), 1000)
}