package database

import (
	"testing"
	"time"
)

func TestNewQueryCache(t *testing.T) {
	cache := NewQueryCache(time.Minute, 100)
	if cache == nil {
		t.Fatal("NewQueryCache returned nil")
	}
	if cache.cache == nil {
		t.Error("cache map should be initialized")
	}
	if cache.lru == nil {
		t.Error("LRU list should be initialized")
	}
}

func TestQueryCache_SetGet(t *testing.T) {
	cache := NewQueryCache(time.Minute, 100)

	// Set a value
	cache.Set("key1", "value1")

	// Get the value
	val, ok := cache.Get("key1")
	if !ok {
		t.Error("Get should find the key")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}
}

func TestQueryCache_GetNonExistent(t *testing.T) {
	cache := NewQueryCache(time.Minute, 100)

	val, ok := cache.Get("nonexistent")
	if ok {
		t.Error("Get should not find non-existent key")
	}
	if val != nil {
		t.Error("Get should return nil for non-existent key")
	}
}

func TestQueryCache_Expiration(t *testing.T) {
	cache := NewQueryCache(10*time.Millisecond, 100)

	// Set a value
	cache.Set("key1", "value1")

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Get should fail
	val, ok := cache.Get("key1")
	if ok {
		t.Error("Get should not find expired key")
	}
	if val != nil {
		t.Error("Get should return nil for expired key")
	}
}

func TestQueryCache_LRUEviction(t *testing.T) {
	cache := NewQueryCache(time.Minute, 3)

	// Add 3 items (max size)
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// Access key1 to make it recently used
	cache.Get("key1")

	// Add another item - should evict key2 (least recently used)
	cache.Set("key4", "value4")

	// key1 should still exist
	if _, ok := cache.Get("key1"); !ok {
		t.Error("key1 should still exist")
	}

	// key2 should be evicted
	if _, ok := cache.Get("key2"); ok {
		t.Error("key2 should be evicted")
	}

	// key3 should still exist
	if _, ok := cache.Get("key3"); !ok {
		t.Error("key3 should still exist")
	}

	// key4 should exist
	if _, ok := cache.Get("key4"); !ok {
		t.Error("key4 should exist")
	}
}

func TestQueryCache_Delete(t *testing.T) {
	cache := NewQueryCache(time.Minute, 100)

	cache.Set("key1", "value1")
	cache.Delete("key1")

	if _, ok := cache.Get("key1"); ok {
		t.Error("key1 should be deleted")
	}
}

func TestQueryCache_Clear(t *testing.T) {
	cache := NewQueryCache(time.Minute, 100)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Clear()

	if len(cache.cache) != 0 {
		t.Error("cache should be empty after Clear")
	}
	if cache.lru.Len() != 0 {
		t.Error("LRU list should be empty after Clear")
	}
}

func TestQueryCache_Stats(t *testing.T) {
	cache := NewQueryCache(time.Minute, 100)

	cache.Set("key1", "value1")

	// Hit
	cache.Get("key1")
	// Miss
	cache.Get("nonexistent")

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestQueryCache_Overwrite(t *testing.T) {
	cache := NewQueryCache(time.Minute, 100)

	cache.Set("key1", "value1")
	cache.Set("key1", "value2")

	val, ok := cache.Get("key1")
	if !ok {
		t.Fatal("key1 should exist")
	}
	if val != "value2" {
		t.Errorf("expected value2, got %v", val)
	}
}
