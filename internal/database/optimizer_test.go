package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
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

func TestQueryCache_Len(t *testing.T) {
	cache := NewQueryCache(time.Minute, 100)

	assert.Equal(t, 0, cache.Len())

	cache.Set("key1", "value1")
	assert.Equal(t, 1, cache.Len())

	cache.Set("key2", "value2")
	assert.Equal(t, 2, cache.Len())

	cache.Delete("key1")
	assert.Equal(t, 1, cache.Len())
}

func TestQueryCache_HitRate(t *testing.T) {
	cache := NewQueryCache(time.Minute, 100)

	// No queries yet
	stats := cache.Stats()
	assert.Equal(t, float64(0), stats.HitRate)

	cache.Set("key1", "value1")

	// 1 hit
	cache.Get("key1")
	stats = cache.Stats()
	assert.Equal(t, float64(100), stats.HitRate)

	// 1 miss
	cache.Get("nonexistent")
	stats = cache.Stats()
	assert.Equal(t, float64(50), stats.HitRate)
}

func TestNewOptimizer(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	logger := zap.NewNop()
	opt := NewOptimizer(db, logger)

	require.NotNil(t, opt)
	assert.NotNil(t, opt.queryCache)
	assert.NotNil(t, opt.pragmas)
	assert.Equal(t, 100*time.Millisecond, opt.slowThreshold)
	assert.False(t, opt.walEnabled)
}

func TestOptimizer_SetSlowThreshold(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	opt := NewOptimizer(db, zap.NewNop())

	newThreshold := 500 * time.Millisecond
	opt.SetSlowThreshold(newThreshold)

	assert.Equal(t, newThreshold, opt.slowThreshold)
}

func TestOptimizer_EnableWAL(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	opt := NewOptimizer(db, zap.NewNop())

	err = opt.EnableWAL()
	assert.NoError(t, err)
	assert.True(t, opt.walEnabled)
}

func TestOptimizer_ConfigurePerformance(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	opt := NewOptimizer(db, zap.NewNop())

	err = opt.ConfigurePerformance()
	assert.NoError(t, err)
	assert.NotEmpty(t, opt.pragmas)
}

func TestOptimizer_CreateIndex(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a test table first
	_, err = db.ExecContext(context.Background(), "CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	opt := NewOptimizer(db, zap.NewNop())

	err = opt.CreateIndex("test_table", "idx_name", "name")
	assert.NoError(t, err)

	// Creating the same index again should not fail
	err = opt.CreateIndex("test_table", "idx_name", "name")
	assert.NoError(t, err)
}

func TestOptimizer_CreateCompositeIndex(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a test table first
	_, err = db.ExecContext(context.Background(), "CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT, email TEXT)")
	require.NoError(t, err)

	opt := NewOptimizer(db, zap.NewNop())

	err = opt.CreateCompositeIndex("test_table", "idx_name_email", "name", "email")
	assert.NoError(t, err)
}

func TestOptimizer_AnalyzeTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a test table first
	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	opt := NewOptimizer(db, zap.NewNop())

	err = opt.AnalyzeTable("test_table")
	assert.NoError(t, err)
}

func TestOptimizer_AnalyzeAll(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a test table first
	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	opt := NewOptimizer(db, zap.NewNop())

	err = opt.AnalyzeAll()
	assert.NoError(t, err)
}

func TestOptimizer_QueryWithCache(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a test table with data
	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test_table (name) VALUES ('test1'), ('test2')")
	require.NoError(t, err)

	opt := NewOptimizer(db, zap.NewNop())

	// First query (cache miss)
	results, err := opt.QueryWithCache("SELECT * FROM test_table")
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// Second query (cache hit)
	results2, err := opt.QueryWithCache("SELECT * FROM test_table")
	assert.NoError(t, err)
	assert.Len(t, results2, 2)

	// Verify cache hit increased
	stats := opt.Stats()
	assert.Equal(t, int64(1), stats.CacheHits)
}

func TestOptimizer_ExecWithTiming(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	opt := NewOptimizer(db, zap.NewNop())

	result, err := opt.ExecWithTiming("INSERT INTO test_table (name) VALUES ('test')")
	assert.NoError(t, err)
	assert.NotNil(t, result)

	stats := opt.Stats()
	assert.Equal(t, int64(1), stats.QueryCount)
}

func TestOptimizer_QueryWithTiming(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	opt := NewOptimizer(db, zap.NewNop())

	rows, err := opt.QueryWithTiming("SELECT * FROM test_table")
	assert.NoError(t, err)
	assert.NoError(t, rows.Err())
	defer rows.Close()

	stats := opt.Stats()
	assert.Equal(t, int64(1), stats.QueryCount)
}

func TestOptimizer_Stats(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	opt := NewOptimizer(db, zap.NewNop())

	stats := opt.Stats()
	assert.Equal(t, int64(0), stats.QueryCount)
	assert.Equal(t, int64(0), stats.SlowQueries)
	assert.False(t, stats.WALEnabled)
}

func TestOptimizer_Vacuum(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	opt := NewOptimizer(db, zap.NewNop())

	err = opt.Vacuum()
	assert.NoError(t, err)
}

func TestOptimizer_Checkpoint(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	opt := NewOptimizer(db, zap.NewNop())

	// WAL not enabled, should return nil
	err = opt.Checkpoint()
	assert.NoError(t, err)

	// Enable WAL and then checkpoint
	err = opt.EnableWAL()
	require.NoError(t, err)
	err = opt.Checkpoint()
	assert.NoError(t, err)
}

func TestOptimizer_GetIndexes(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	opt := NewOptimizer(db, zap.NewNop())

	// Create an index
	err = opt.CreateIndex("test_table", "idx_name", "name")
	require.NoError(t, err)

	indexes, err := opt.GetIndexes()
	assert.NoError(t, err)
	assert.NotEmpty(t, indexes)
}

func TestOptimizer_Close(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	opt := NewOptimizer(db, zap.NewNop())
	opt.queryCache.Set("key1", "value1")

	opt.Close()

	// Cache should be cleared
	assert.Equal(t, 0, opt.queryCache.Len())
}

func TestJoinColumns(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a, b"},
		{[]string{"a", "b", "c"}, "a, b, c"},
		{[]string{}, ""},
	}

	for _, tt := range tests {
		result := joinColumns(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}
