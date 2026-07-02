// Package truesearch 实现全文搜索引擎 (TrueSearch Phase 2)
// 本文件包含性能优化的单元测试和基准测试。
package truesearch

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─── IndexCache 测试 ──────────────────────────────────────────

func TestIndexCache_BasicOperations(t *testing.T) {
	cache := NewIndexCache(100)

	// Put & Get
	cache.Put("key1", []byte("value1"))
	val, ok := cache.Get("key1")
	require.True(t, ok)
	assert.Equal(t, "value1", string(val))

	// Miss
	_, ok = cache.Get("nonexistent")
	assert.False(t, ok)

	// Update existing
	cache.Put("key1", []byte("value1_updated"))
	val, ok = cache.Get("key1")
	require.True(t, ok)
	assert.Equal(t, "value1_updated", string(val))
}

func TestIndexCache_Eviction(t *testing.T) {
	cache := NewIndexCache(3)

	cache.Put("a", []byte("1"))
	cache.Put("b", []byte("2"))
	cache.Put("c", []byte("3"))

	// 访问 a 使其变为最近使用
	_, _ = cache.Get("a")

	// 插入 d，应该驱逐 b（最久未使用）
	cache.Put("d", []byte("4"))

	_, ok := cache.Get("b")
	assert.False(t, ok, "b should have been evicted")

	_, ok = cache.Get("a")
	assert.True(t, ok, "a should still be cached")

	_, ok = cache.Get("c")
	assert.True(t, ok, "c should still be cached")

	_, ok = cache.Get("d")
	assert.True(t, ok, "d should be cached")
}

func TestIndexCache_Remove(t *testing.T) {
	cache := NewIndexCache(10)
	cache.Put("key1", []byte("value1"))
	cache.Put("key2", []byte("value2"))

	cache.Remove("key1")
	_, ok := cache.Get("key1")
	assert.False(t, ok)

	_, ok = cache.Get("key2")
	assert.True(t, ok)
}

func TestIndexCache_Clear(t *testing.T) {
	cache := NewIndexCache(10)
	cache.Put("key1", []byte("value1"))
	cache.Put("key2", []byte("value2"))

	cache.Clear()

	stats := cache.Stats()
	assert.Equal(t, 0, stats.ItemsCount)

	_, ok := cache.Get("key1")
	assert.False(t, ok)
}

func TestIndexCache_Stats(t *testing.T) {
	cache := NewIndexCache(10)

	cache.Put("key1", []byte("value1"))

	// Hit
	_, _ = cache.Get("key1")
	// Miss
	_, _ = cache.Get("nonexistent")

	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, 1, stats.ItemsCount)
	assert.Equal(t, int64(6), stats.SizeBytes) // "value1" = 6 bytes
}

func TestIndexCache_Concurrent(t *testing.T) {
	cache := NewIndexCache(1000)
	var wg sync.WaitGroup

	// 并发写入
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cache.Put(fmt.Sprintf("key-%d", idx), []byte(fmt.Sprintf("value-%d", idx)))
		}(i)
	}

	// 并发读取
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cache.Get(fmt.Sprintf("key-%d", idx))
		}(i)
	}

	wg.Wait()

	stats := cache.Stats()
	assert.True(t, stats.ItemsCount > 0)
	assert.True(t, stats.Hits > 0 || stats.Misses > 0)
}

// ─── QueryCache 测试 ──────────────────────────────────────────

func TestQueryCache_BasicOperations(t *testing.T) {
	qc := NewQueryCache(100, 5*time.Minute)

	req := SearchRequest{Query: "test", MaxResults: 10}
	resp := &SearchResponse{
		Results: []SearchResult{{Path: "/test/file.txt", Name: "file.txt", Score: 1.0}},
		Total:   1,
		TookMs:  5,
	}

	// Put & Get
	qc.Put(req, resp)
	cached, ok := qc.Get(req)
	require.True(t, ok)
	assert.Equal(t, 1, cached.Total)
	assert.Equal(t, "file.txt", cached.Results[0].Name)

	// Miss
	missReq := SearchRequest{Query: "nonexistent"}
	_, ok = qc.Get(missReq)
	assert.False(t, ok)
}

func TestQueryCache_TTLExpiration(t *testing.T) {
	qc := NewQueryCache(100, 50*time.Millisecond)

	req := SearchRequest{Query: "test"}
	resp := &SearchResponse{Total: 1, TookMs: 5}

	qc.Put(req, resp)

	// 立即获取应该命中
	_, ok := qc.Get(req)
	assert.True(t, ok)

	// 等待过期
	time.Sleep(60 * time.Millisecond)

	// 过期后应该未命中
	_, ok = qc.Get(req)
	assert.False(t, ok)
}

func TestQueryCache_Invalidate(t *testing.T) {
	qc := NewQueryCache(100, 5*time.Minute)

	req := SearchRequest{Query: "test"}
	resp := &SearchResponse{Total: 1}

	qc.Put(req, resp)
	qc.Invalidate()

	_, ok := qc.Get(req)
	assert.False(t, ok)
}

func TestQueryCache_CloneResponse(t *testing.T) {
	qc := NewQueryCache(100, 5*time.Minute)

	req := SearchRequest{Query: "test"}
	resp := &SearchResponse{
		Results: []SearchResult{{Path: "/original.txt", Name: "original.txt"}},
		Total:   1,
	}

	qc.Put(req, resp)

	// 修改原始响应不应影响缓存
	resp.Results[0].Name = "modified"

	cached, ok := qc.Get(req)
	require.True(t, ok)
	assert.Equal(t, "original.txt", cached.Results[0].Name)
}

func TestQueryCache_Eviction(t *testing.T) {
	qc := NewQueryCache(3, 5*time.Minute)

	qc.Put(SearchRequest{Query: "q1"}, &SearchResponse{Total: 1})
	qc.Put(SearchRequest{Query: "q2"}, &SearchResponse{Total: 2})
	qc.Put(SearchRequest{Query: "q3"}, &SearchResponse{Total: 3})

	// 访问 q1
	_, _ = qc.Get(SearchRequest{Query: "q1"})

	// 插入 q4，应该驱逐 q2
	qc.Put(SearchRequest{Query: "q4"}, &SearchResponse{Total: 4})

	_, ok := qc.Get(SearchRequest{Query: "q2"})
	assert.False(t, ok, "q2 should have been evicted")

	_, ok = qc.Get(SearchRequest{Query: "q1"})
	assert.True(t, ok, "q1 should still be cached")
}

func TestQueryCache_Stats(t *testing.T) {
	qc := NewQueryCache(100, 5*time.Minute)

	req := SearchRequest{Query: "test"}
	qc.Put(req, &SearchResponse{Total: 1})

	qc.Get(req)                          // hit
	qc.Get(SearchRequest{Query: "miss"}) // miss

	stats := qc.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
}

// ─── PerformanceManager 测试 ──────────────────────────────────

func TestPerformanceManager_FileHashTracking(t *testing.T) {
	logger := newTestLogger(t)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)

	// 新文件
	assert.True(t, pm.HasFileChanged("/test/file.txt", 12345))

	// 记录哈希
	pm.SetFileHash("/test/file.txt", 12345)

	// 相同哈希 = 未变化
	assert.False(t, pm.HasFileChanged("/test/file.txt", 12345))

	// 不同哈希 = 已变化
	assert.True(t, pm.HasFileChanged("/test/file.txt", 67890))

	// 移除
	pm.RemoveFileHash("/test/file.txt")
	assert.True(t, pm.HasFileChanged("/test/file.txt", 12345))
}

func TestPerformanceManager_IncrementalStats(t *testing.T) {
	logger := newTestLogger(t)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)

	pm.SetFileHash("/file1.txt", 1)
	pm.SetFileHash("/file2.txt", 2)
	pm.SetFileHash("/file3.txt", 3)

	stats := pm.IncrementalStats()
	assert.Equal(t, 3, stats.TrackedFiles)
}

func TestPerformanceManager_QueryCacheIntegration(t *testing.T) {
	logger := newTestLogger(t)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)

	req := SearchRequest{Query: "test"}
	resp := &SearchResponse{Total: 1, TookMs: 5}

	// 缓存查询
	pm.PutCachedQuery(req, resp)

	// 获取缓存
	cached, ok := pm.GetCachedQuery(req)
	require.True(t, ok)
	assert.Equal(t, 1, cached.Total)

	// 使失效
	pm.InvalidateQueryCache()
	_, ok = pm.GetCachedQuery(req)
	assert.False(t, ok)
}

func TestPerformanceManager_Close(t *testing.T) {
	logger := newTestLogger(t)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)

	pm.SetFileHash("/file.txt", 1)
	pm.PutCachedQuery(SearchRequest{Query: "test"}, &SearchResponse{Total: 1})

	pm.Close()

	// 关闭后缓存应清空
	_, ok := pm.GetCachedQuery(SearchRequest{Query: "test"})
	assert.False(t, ok)
}

// ─── BatchIndexer 测试 ────────────────────────────────────────

func TestBatchIndexer_BatchIndexFiles(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     5,
		SupportedExts: []string{".txt"},
		ExcludeDirs:   []string{},
	}

	idx, err := NewIndexer(cfg, logger)
	require.NoError(t, err)
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)
	defer pm.Close()

	bi := NewBatchIndexer(idx, extractor, pm, logger)

	// 创建测试文件
	var paths []string
	for i := 0; i < 10; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file_%d.txt", i))
		content := fmt.Sprintf("content for file %d about topic %d", i, i%3)
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
		paths = append(paths, path)
	}

	result := bi.BatchIndexFiles(paths)

	assert.Equal(t, 10, result.Total)
	assert.Equal(t, 10, result.Indexed)
	assert.Equal(t, 0, result.Skipped)
	assert.Equal(t, 0, result.Failed)
	assert.True(t, result.Duration > 0)

	// 第二次索引相同文件应该全部跳过（增量更新）
	result2 := bi.BatchIndexFiles(paths)
	assert.Equal(t, 10, result2.Total)
	assert.Equal(t, 10, result2.Skipped)
	assert.Equal(t, 0, result2.Indexed)
}

func TestBatchIndexer_BatchIndexDirectory(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     3,
		SupportedExts: []string{".txt", ".md"},
		ExcludeDirs:   []string{".git"},
	}

	idx, err := NewIndexer(cfg, logger)
	require.NoError(t, err)
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)
	defer pm.Close()

	bi := NewBatchIndexer(idx, extractor, pm, logger)

	// 创建测试目录结构
	dataDir := filepath.Join(dir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "sub"), 0755))

	for i := 0; i < 5; i++ {
		path := filepath.Join(dataDir, fmt.Sprintf("file_%d.txt", i))
		require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf("content %d", i)), 0644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "sub", "nested.md"), []byte("# Nested"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "ignore.xyz"), []byte("ignored"), 0644))

	// 首次索引
	result := bi.BatchIndexDirectory(dataDir)
	assert.True(t, result.Total >= 6) // 5 txt + 1 md (ignore.xyz 被跳过)
	assert.True(t, result.Indexed >= 6)
	assert.Equal(t, 0, result.Failed)

	// 第二次索引应该全部跳过
	result2 := bi.BatchIndexDirectory(dataDir)
	assert.Equal(t, result2.Skipped, result2.Total)
	assert.Equal(t, 0, result2.Indexed)
}

func TestBatchIndexer_IncrementalUpdate(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt"},
		ExcludeDirs:   []string{},
	}

	idx, err := NewIndexer(cfg, logger)
	require.NoError(t, err)
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)
	defer pm.Close()

	bi := NewBatchIndexer(idx, extractor, pm, logger)

	// 创建并索引文件
	path := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("original content"), 0644))

	result := bi.BatchIndexFiles([]string{path})
	assert.Equal(t, 1, result.Indexed)

	// 修改文件后重新索引
	time.Sleep(10 * time.Millisecond) // 确保修改时间不同
	require.NoError(t, os.WriteFile(path, []byte("updated content with new info"), 0644))

	result2 := bi.BatchIndexFiles([]string{path})
	assert.Equal(t, 1, result2.Indexed, "modified file should be re-indexed")
	assert.Equal(t, 0, result2.Skipped)
}

func TestBatchIndexer_DeletedFiles(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt"},
		ExcludeDirs:   []string{},
	}

	idx, err := NewIndexer(cfg, logger)
	require.NoError(t, err)
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)
	defer pm.Close()

	bi := NewBatchIndexer(idx, extractor, pm, logger)

	// 创建并索引文件
	dataDir := filepath.Join(dir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0755))

	file1 := filepath.Join(dataDir, "file1.txt")
	file2 := filepath.Join(dataDir, "file2.txt")
	require.NoError(t, os.WriteFile(file1, []byte("content1"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("content2"), 0644))

	bi.BatchIndexDirectory(dataDir)

	// 删除 file2
	require.NoError(t, os.Remove(file2))

	// 重新索引目录，应该检测到删除
	result := bi.BatchIndexDirectory(dataDir)
	assert.True(t, result.DeletedFiles > 0, "should detect deleted files")
}

func TestBatchIndexResult_Errors(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt"},
		ExcludeDirs:   []string{},
	}

	idx, err := NewIndexer(cfg, logger)
	require.NoError(t, err)
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)
	defer pm.Close()

	bi := NewBatchIndexer(idx, extractor, pm, logger)

	// 尝试索引不存在的文件
	result := bi.BatchIndexFiles([]string{"/nonexistent/path/file.txt"})
	assert.True(t, result.Failed > 0)
	assert.True(t, len(result.Errors) > 0)
}

// ─── OptimizedSearch 测试 ─────────────────────────────────────

func TestOptimizedSearch_CacheHit(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt"},
		ExcludeDirs:   []string{},
	}

	idx, err := NewIndexer(cfg, logger)
	require.NoError(t, err)
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)

	// 创建并索引测试文件
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("cacheable content for search"), 0644))
	require.NoError(t, idx.IndexFile(path, extractor))

	time.Sleep(100 * time.Millisecond)

	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)
	defer pm.Close()

	req := SearchRequest{Query: "cacheable", MaxResults: 10}

	// 第一次搜索 - 未命中缓存
	resp1, err := OptimizedSearch(idx, pm, req)
	require.NoError(t, err)
	assert.True(t, resp1.Total > 0)

	// 第二次搜索 - 应该命中缓存
	resp2, err := OptimizedSearch(idx, pm, req)
	require.NoError(t, err)
	assert.Equal(t, resp1.Total, resp2.Total)

	// 验证缓存命中
	qcStats := pm.QueryCacheStats()
	require.NotNil(t, qcStats)
	assert.True(t, qcStats.Hits > 0, "should have cache hits")
}

// ─── computeFileHash 测试 ─────────────────────────────────────

func TestComputeFileHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("test content"), 0644))

	info, err := os.Stat(path)
	require.NoError(t, err)

	hash1 := computeFileHash(path, info)

	// 修改文件
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, os.WriteFile(path, []byte("modified content"), 0644))
	info2, err := os.Stat(path)
	require.NoError(t, err)

	hash2 := computeFileHash(path, info2)

	// 哈希应该不同
	assert.NotEqual(t, hash1, hash2)
}

// ─── SSDAwareConfig 测试 ──────────────────────────────────────

func TestSSDAwareConfig(t *testing.T) {
	cfg := SSDAwareConfig("/tmp/test")

	// 应该返回有效配置
	assert.True(t, cfg.IndexCacheSize > 0)
	assert.True(t, cfg.QueryCacheSize > 0)
	assert.True(t, cfg.BatchSize > 0)
}

func TestDetectSSD(t *testing.T) {
	// 在测试环境中，detectSSD 应该返回一个布尔值而不崩溃
	result := detectSSD("/tmp")
	_ = result // 只验证不 panic
}

// ─── 性能状态测试 ─────────────────────────────────────────────

func TestPerformanceManager_GetPerformanceStatus(t *testing.T) {
	logger := newTestLogger(t)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)
	defer pm.Close()

	pm.SetFileHash("/file1.txt", 1)
	pm.PutCachedQuery(SearchRequest{Query: "test"}, &SearchResponse{Total: 1})

	status := pm.GetPerformanceStatus("/tmp")
	assert.Equal(t, 1, status.Incremental.TrackedFiles)
	// IsSSD 在测试环境中可能为 true 或 false，只验证不 panic
	_ = status.IsSSD
}

// ─── 基准测试 ─────────────────────────────────────────────────

func BenchmarkIndexCache_Put(b *testing.B) {
	cache := NewIndexCache(10000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.Put(fmt.Sprintf("key-%d", i), []byte(fmt.Sprintf("value-%d", i)))
	}
}

func BenchmarkIndexCache_Get(b *testing.B) {
	cache := NewIndexCache(10000)
	for i := 0; i < 1000; i++ {
		cache.Put(fmt.Sprintf("key-%d", i), []byte(fmt.Sprintf("value-%d", i)))
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.Get(fmt.Sprintf("key-%d", i%1000))
	}
}

func BenchmarkIndexCache_Concurrent(b *testing.B) {
	cache := NewIndexCache(10000)
	for i := 0; i < 1000; i++ {
		cache.Put(fmt.Sprintf("key-%d", i), []byte(fmt.Sprintf("value-%d", i)))
	}
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Get(fmt.Sprintf("key-%d", i%1000))
			i++
		}
	})
}

func BenchmarkQueryCache_Put(b *testing.B) {
	qc := NewQueryCache(10000, 5*time.Minute)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := SearchRequest{Query: fmt.Sprintf("query-%d", i), MaxResults: 10}
		qc.Put(req, &SearchResponse{Total: i, TookMs: int64(i)})
	}
}

func BenchmarkQueryCache_Get(b *testing.B) {
	qc := NewQueryCache(10000, 5*time.Minute)
	for i := 0; i < 1000; i++ {
		req := SearchRequest{Query: fmt.Sprintf("query-%d", i), MaxResults: 10}
		qc.Put(req, &SearchResponse{Total: i})
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := SearchRequest{Query: fmt.Sprintf("query-%d", i%1000), MaxResults: 10}
		qc.Get(req)
	}
}

func BenchmarkBatchIndexFiles(b *testing.B) {
	dir := b.TempDir()
	logger := newTestLoggerBench(b)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "bench.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     100,
		SupportedExts: []string{".txt"},
		ExcludeDirs:   []string{},
	}

	idx, err := NewIndexer(cfg, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)
	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)
	defer pm.Close()

	bi := NewBatchIndexer(idx, extractor, pm, logger)

	// 创建测试文件
	var paths []string
	for i := 0; i < 100; i++ {
		path := filepath.Join(dir, fmt.Sprintf("bench_%d.txt", i))
		content := fmt.Sprintf("benchmark content file %d topic %d", i, i%5)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
		paths = append(paths, path)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 每次使用不同的文件以避免增量跳过
		for j := range paths {
			path := filepath.Join(dir, fmt.Sprintf("bench_%d_%d.txt", i, j))
			_ = os.WriteFile(path, []byte(fmt.Sprintf("content %d-%d", i, j)), 0644)
			paths[j] = path
		}
		bi.BatchIndexFiles(paths)
	}
}

func BenchmarkOptimizedSearch(b *testing.B) {
	dir := b.TempDir()
	logger := newTestLoggerBench(b)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "bench.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     100,
		SupportedExts: []string{".txt"},
		ExcludeDirs:   []string{},
	}

	idx, err := NewIndexer(cfg, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)

	// 创建并索引文件
	for i := 0; i < 50; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file_%d.txt", i))
		content := fmt.Sprintf("document %d about golang programming database network", i)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
		if err := idx.IndexFile(path, extractor); err != nil {
			b.Fatal(err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	pm := NewPerformanceManager(DefaultPerformanceConfig(), logger)
	defer pm.Close()

	req := SearchRequest{Query: "golang programming", MaxResults: 20}

	// 第一次搜索填充缓存
	_, _ = OptimizedSearch(idx, pm, req)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = OptimizedSearch(idx, pm, req)
	}
}

func BenchmarkSearch_NoCache(b *testing.B) {
	dir := b.TempDir()
	logger := newTestLoggerBench(b)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "bench.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     100,
		SupportedExts: []string{".txt"},
		ExcludeDirs:   []string{},
	}

	idx, err := NewIndexer(cfg, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)

	for i := 0; i < 50; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file_%d.txt", i))
		content := fmt.Sprintf("document %d about golang programming database network", i)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
		if err := idx.IndexFile(path, extractor); err != nil {
			b.Fatal(err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	req := SearchRequest{Query: "golang programming", MaxResults: 20}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = idx.Search(req)
	}
}

// newTestLoggerBench 创建基准测试用 logger（静默）。
func newTestLoggerBench(b *testing.B) *zap.Logger {
	b.Helper()
	logger, err := zap.NewProduction()
	if err != nil {
		b.Fatal(err)
	}
	return logger
}
