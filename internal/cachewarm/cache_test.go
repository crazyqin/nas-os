package cachewarm

import (
	"fmt"
	"testing"
	"time"
)

func TestNewSmartCache(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 1024*1024)
	if cache == nil {
		t.Fatal("NewSmartCache returned nil")
	}
	if cache.policy != PolicyLRU {
		t.Errorf("expected LRU, got %s", cache.policy)
	}
}

func TestSetAndGet(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 1024)
	cache.Set("key1", "value1", 64, 0)

	val, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}
}

func TestCacheMiss(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 1024)
	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected miss")
	}
	stats := cache.GetStats()
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestLRUEviction(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 128)
	cache.Set("a", "1", 64, 0)
	cache.Set("b", "2", 64, 0)
	cache.Set("c", "3", 64, 0) // should evict "a"

	_, ok := cache.Get("a")
	if ok {
		t.Error("expected a to be evicted")
	}
	_, ok = cache.Get("c")
	if !ok {
		t.Error("expected c to exist")
	}
}

func TestLFUEviction(t *testing.T) {
	cache := NewSmartCache(PolicyLFU, 128)
	cache.Set("a", "1", 64, 0)
	cache.Get("a") // freq=2
	cache.Get("a") // freq=3
	cache.Set("b", "2", 64, 0) // freq=1
	cache.Set("c", "3", 64, 0) // should evict "b" (lowest freq)

	_, ok := cache.Get("b")
	if ok {
		t.Error("expected b to be evicted")
	}
}

func TestTTL(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 1024)
	cache.Set("key", "val", 64, 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)
	_, ok := cache.Get("key")
	if ok {
		t.Error("expected key to expire")
	}
}

func TestDelete(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 1024)
	cache.Set("key", "val", 64, 0)
	ok := cache.Delete("key")
	if !ok {
		t.Error("expected true")
	}
	_, ok = cache.Get("key")
	if ok {
		t.Error("expected deleted")
	}
}

func TestWarmTask(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 1024)
	keys := []string{"k1", "k2", "k3"}
	task := cache.CreateWarmTask("warm1", keys, WarmScheduled)
	if task.Status != "pending" {
		t.Errorf("expected pending, got %s", task.Status)
	}

	cache.ExecuteWarmTask("warm1", func(key string) (interface{}, int64, error) {
		return "val_" + key, 32, nil
	})

	if task.Warmed != 3 {
		t.Errorf("expected 3 warmed, got %d", task.Warmed)
	}
	if task.Status != "completed" {
		t.Errorf("expected completed, got %s", task.Status)
	}
}

func TestPredictHotKeys(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 1024)
	cache.Set("hot", "1", 64, 0)
	cache.Set("cold", "2", 64, 0)

	for i := 0; i < 10; i++ {
		cache.Get("hot")
	}

	keys := cache.PredictHotKeys(2)
	if len(keys) == 0 {
		t.Fatal("expected hot keys")
	}
	if keys[0] != "hot" {
		t.Errorf("expected hot first, got %s", keys[0])
	}
}

func TestStats(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 1024)
	cache.Set("a", "1", 64, 0)
	cache.Get("a")
	cache.Get("a")
	cache.Get("missing")

	stats := cache.GetStats()
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.EntryCount != 1 {
		t.Errorf("expected 1 entry, got %d", stats.EntryCount)
	}
}

func TestUpdateExistingKey(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 1024)
	cache.Set("key", "old", 64, 0)
	cache.Set("key", "new", 64, 0)

	val, _ := cache.Get("key")
	if val != "new" {
		t.Errorf("expected new, got %v", val)
	}
}

func TestWarmTaskLoadError(t *testing.T) {
	cache := NewSmartCache(PolicyLRU, 1024)
	task := cache.CreateWarmTask("warm1", []string{"k1", "k2"}, WarmOnDemand)
	cache.ExecuteWarmTask("warm1", func(key string) (interface{}, int64, error) {
		if key == "k1" {
			return nil, 0, fmt.Errorf("load error")
		}
		return "val", 32, nil
	})
	if task.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", task.Failed)
	}
	if task.Warmed != 1 {
		t.Errorf("expected 1 warmed, got %d", task.Warmed)
	}
}
