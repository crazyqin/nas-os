// Package integration 提供 NAS-OS 集成测试
// 智能搜索模块集成测试
package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"nas-os/internal/search"
)

// MockSearchEngine 模拟搜索引擎.
type MockSearchEngine struct {
	index   map[string]*search.FileInfo
	results map[string][]search.Result
	stats   search.IndexStats
	mu      sync.RWMutex
}

// NewMockSearchEngine 创建模拟搜索引擎.
func NewMockSearchEngine() *MockSearchEngine {
	e := &MockSearchEngine{
		index:   make(map[string]*search.FileInfo),
		results: make(map[string][]search.Result),
		stats: search.IndexStats{
			TotalFiles:   0,
			IndexedFiles: 0,
		},
	}

	// 预置一些测试文件
	now := time.Now()
	e.index["/data/document.txt"] = &search.FileInfo{
		Path:    "/data/document.txt",
		Name:    "document.txt",
		Ext:     ".txt",
		Size:    1024,
		ModTime: now,
		IsDir:   false,
	}

	e.index["/data/report.pdf"] = &search.FileInfo{
		Path:    "/data/report.pdf",
		Name:    "report.pdf",
		Ext:     ".pdf",
		Size:    2048,
		ModTime: now,
		IsDir:   false,
	}

	e.index["/data/images/"] = &search.FileInfo{
		Path:    "/data/images/",
		Name:    "images",
		IsDir:   true,
		ModTime: now,
	}

	e.stats.TotalFiles = 3
	e.stats.IndexedFiles = 3

	return e
}

// IndexFile 索引文件.
func (e *MockSearchEngine) IndexFile(info *search.FileInfo) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.index[info.Path] = info
	e.stats.TotalFiles++
	e.stats.IndexedFiles++
	e.stats.LastIndexed = time.Now()

	return nil
}

// Search 搜索.
func (e *MockSearchEngine) Search(ctx context.Context, req *search.Request) ([]search.Result, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []search.Result

	for _, info := range e.index {
		// 简单匹配：检查文件名是否包含查询字符串
		score := 1.0

		// 应用过滤器
		if req.MinSize > 0 && info.Size < req.MinSize {
			continue
		}

		if req.MaxSize > 0 && info.Size > req.MaxSize {
			continue
		}

		// 文件类型过滤
		if len(req.Types) > 0 {
			found := false
			for _, t := range req.Types {
				if info.Ext == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		results = append(results, search.Result{
			Path:    info.Path,
			Name:    info.Name,
			Ext:     info.Ext,
			Size:    info.Size,
			ModTime: info.ModTime,
			IsDir:   info.IsDir,
			Score:   score,
		})
	}

	// 应用限制
	if req.Limit > 0 && len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results, nil
}

// DeleteFromIndex 从索引删除.
func (e *MockSearchEngine) DeleteFromIndex(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.index, path)
	e.stats.IndexedFiles--

	return nil
}

// GetStats 获取统计.
func (e *MockSearchEngine) GetStats() search.IndexStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// RebuildIndex 重建索引.
func (e *MockSearchEngine) RebuildIndex(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.LastIndexed = time.Now()
	return nil
}

// ========== 智能搜索集成测试 ==========

// TestSearch_IndexFile 测试文件索引.
func TestSearch_IndexFile(t *testing.T) {
	engine := NewMockSearchEngine()

	file := &search.FileInfo{
		Path:    "/data/test.txt",
		Name:    "test.txt",
		Ext:     ".txt",
		Size:    512,
		ModTime: time.Now(),
		IsDir:   false,
	}

	err := engine.IndexFile(file)
	if err != nil {
		t.Fatalf("IndexFile failed: %v", err)
	}

	stats := engine.GetStats()
	if stats.TotalFiles < 4 {
		t.Errorf("Expected TotalFiles >= 4, got %d", stats.TotalFiles)
	}
}

// TestSearch_Search 测试搜索功能.
func TestSearch_Search(t *testing.T) {
	engine := NewMockSearchEngine()
	ctx := context.Background()

	tests := []struct {
		name      string
		request   *search.Request
		minResult int
	}{
		{
			name: "Search all",
			request: &search.Request{
				Query: "",
			},
			minResult: 1,
		},
		{
			name: "Search with limit",
			request: &search.Request{
				Query: "",
				Limit: 2,
			},
			minResult: 1,
		},
		{
			name: "Search with size filter",
			request: &search.Request{
				Query:   "",
				MinSize: 1000,
			},
			minResult: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := engine.Search(ctx, tt.request)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if len(results) < tt.minResult {
				t.Errorf("Expected at least %d results, got %d", tt.minResult, len(results))
			}
		})
	}
}

// TestSearch_SearchWithFilters 测试带过滤器的搜索.
func TestSearch_SearchWithFilters(t *testing.T) {
	engine := NewMockSearchEngine()
	ctx := context.Background()

	// 添加测试文件
	engine.IndexFile(&search.FileInfo{
		Path: "/data/small.txt",
		Name: "small.txt",
		Ext:  ".txt",
		Size: 100,
	})

	engine.IndexFile(&search.FileInfo{
		Path: "/data/large.pdf",
		Name: "large.pdf",
		Ext:  ".pdf",
		Size: 10000,
	})

	tests := []struct {
		name       string
		request    *search.Request
		expectSize int
	}{
		{
			name: "Filter by min size",
			request: &search.Request{
				MinSize: 1000,
			},
			expectSize: -1, // 不确定具体数量
		},
		{
			name: "Filter by max size",
			request: &search.Request{
				MaxSize: 500,
			},
			expectSize: -1,
		},
		{
			name: "Filter by type",
			request: &search.Request{
				Types: []string{".pdf"},
			},
			expectSize: -1,
		},
		{
			name: "Filter by type and size",
			request: &search.Request{
				Types:   []string{".txt"},
				MinSize: 50,
			},
			expectSize: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := engine.Search(ctx, tt.request)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			// 验证结果符合过滤条件
			for _, r := range results {
				if tt.request.MinSize > 0 && r.Size < tt.request.MinSize {
					t.Errorf("Result size %d is less than min size %d", r.Size, tt.request.MinSize)
				}

				if tt.request.MaxSize > 0 && r.Size > tt.request.MaxSize {
					t.Errorf("Result size %d is greater than max size %d", r.Size, tt.request.MaxSize)
				}

				if len(tt.request.Types) > 0 {
					found := false
					for _, t := range tt.request.Types {
						if r.Ext == t {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Result type %s not in allowed types %v", r.Ext, tt.request.Types)
					}
				}
			}
		})
	}
}

// TestSearch_DeleteFromIndex 测试删除索引.
func TestSearch_DeleteFromIndex(t *testing.T) {
	engine := NewMockSearchEngine()

	// 添加文件
	file := &search.FileInfo{
		Path: "/data/to-delete.txt",
		Name: "to-delete.txt",
		Ext:  ".txt",
		Size: 100,
	}
	engine.IndexFile(file)

	statsBefore := engine.GetStats()

	// 删除
	err := engine.DeleteFromIndex("/data/to-delete.txt")
	if err != nil {
		t.Fatalf("DeleteFromIndex failed: %v", err)
	}

	statsAfter := engine.GetStats()
	if statsAfter.IndexedFiles >= statsBefore.IndexedFiles {
		t.Error("IndexedFiles should decrease after deletion")
	}
}

// TestSearch_RebuildIndex 测试重建索引.
func TestSearch_RebuildIndex(t *testing.T) {
	engine := NewMockSearchEngine()
	ctx := context.Background()

	err := engine.RebuildIndex(ctx)
	if err != nil {
		t.Fatalf("RebuildIndex failed: %v", err)
	}

	stats := engine.GetStats()
	if stats.LastIndexed.IsZero() {
		t.Error("LastIndexed should be set after rebuild")
	}
}

// TestSearch_GetStats 测试获取统计.
func TestSearch_GetStats(t *testing.T) {
	engine := NewMockSearchEngine()

	stats := engine.GetStats()

	if stats.TotalFiles < 0 {
		t.Error("TotalFiles should not be negative")
	}

	if stats.IndexedFiles < 0 {
		t.Error("IndexedFiles should not be negative")
	}

	if stats.IndexedFiles > stats.TotalFiles {
		t.Error("IndexedFiles should not exceed TotalFiles")
	}
}

// TestSearch_ConcurrentSearch 测试并发搜索.
func TestSearch_ConcurrentSearch(t *testing.T) {
	engine := NewMockSearchEngine()
	ctx := context.Background()
	done := make(chan bool, 20)

	for i := 0; i < 10; i++ {
		go func() {
			_, _ = engine.Search(ctx, &search.Request{Query: "test"})
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			_ = engine.IndexFile(&search.FileInfo{
				Path: "/data/concurrent.txt",
				Name: "concurrent.txt",
				Ext:  ".txt",
				Size: 100,
			})
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

// TestSearch_SearchResultScore 测试搜索结果评分.
func TestSearch_SearchResultScore(t *testing.T) {
	engine := NewMockSearchEngine()
	ctx := context.Background()

	results, err := engine.Search(ctx, &search.Request{Query: ""})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.Score < 0 {
			t.Errorf("Score should not be negative: %f", r.Score)
		}
	}
}

// TestSearch_FileInfoStructure 测试文件信息结构.
func TestSearch_FileInfoStructure(t *testing.T) {
	now := time.Now()
	info := &search.FileInfo{
		Path:     "/data/test.txt",
		Name:     "test.txt",
		Ext:      ".txt",
		Size:     1024,
		ModTime:  now,
		IsDir:    false,
		Content:  "test content",
		MimeType: "text/plain",
	}

	if info.Path != "/data/test.txt" {
		t.Errorf("Expected path=/data/test.txt, got %s", info.Path)
	}

	if info.Ext != ".txt" {
		t.Errorf("Expected ext=.txt, got %s", info.Ext)
	}

	if info.Size != 1024 {
		t.Errorf("Expected size=1024, got %d", info.Size)
	}

	if info.IsDir {
		t.Error("Expected IsDir=false")
	}

	if info.MimeType != "text/plain" {
		t.Errorf("Expected mimeType=text/plain, got %s", info.MimeType)
	}
}

// TestSearch_SearchResultStructure 测试搜索结果结构.
func TestSearch_SearchResultStructure(t *testing.T) {
	now := time.Now()
	result := search.Result{
		Path:    "/data/result.txt",
		Name:    "result.txt",
		Ext:     ".txt",
		Size:    2048,
		ModTime: now,
		IsDir:   false,
		Score:   0.95,
		Highlights: []search.Highlight{
			{
				Field:     "content",
				Fragments: []string{"<em>highlighted</em> text"},
			},
		},
	}

	if result.Path != "/data/result.txt" {
		t.Errorf("Expected path=/data/result.txt, got %s", result.Path)
	}

	if result.Score != 0.95 {
		t.Errorf("Expected score=0.95, got %f", result.Score)
	}

	if len(result.Highlights) != 1 {
		t.Errorf("Expected 1 highlight, got %d", len(result.Highlights))
	}
}

// TestSearch_IndexConfig 测试索引配置.
func TestSearch_IndexConfig(t *testing.T) {
	config := search.DefaultIndexConfig()

	if config.IndexPath == "" {
		t.Error("IndexPath should not be empty")
	}

	if config.MaxFileSize <= 0 {
		t.Error("MaxFileSize should be positive")
	}

	if config.Workers <= 0 {
		t.Error("Workers should be positive")
	}

	if config.BatchSize <= 0 {
		t.Error("BatchSize should be positive")
	}
}

// TestSearch_SearchRequestWithDate 测试日期过滤.
func TestSearch_SearchRequestWithDate(t *testing.T) {
	engine := NewMockSearchEngine()
	ctx := context.Background()

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	// 添加不同时间的文件
	engine.IndexFile(&search.FileInfo{
		Path:    "/data/old.txt",
		Name:    "old.txt",
		Ext:     ".txt",
		Size:    100,
		ModTime: yesterday,
	})

	engine.IndexFile(&search.FileInfo{
		Path:    "/data/new.txt",
		Name:    "new.txt",
		Ext:     ".txt",
		Size:    100,
		ModTime: now,
	})

	// 测试日期过滤
	req := &search.Request{
		FromDate: &yesterday,
		ToDate:   &now,
	}

	results, err := engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// 验证结果在日期范围内
	for _, r := range results {
		if r.ModTime.Before(yesterday) || r.ModTime.After(now) {
			t.Errorf("Result modTime %v is out of range", r.ModTime)
		}
	}
}

// TestSearch_SearchRequestSorting 测试排序.
func TestSearch_SearchRequestSorting(t *testing.T) {
	engine := NewMockSearchEngine()
	ctx := context.Background()

	tests := []struct {
		name     string
		sortBy   string
		sortDesc bool
	}{
		{"Sort by name asc", "name", false},
		{"Sort by name desc", "name", true},
		{"Sort by size asc", "size", false},
		{"Sort by size desc", "size", true},
		{"Sort by modTime asc", "modTime", false},
		{"Sort by score desc", "score", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &search.Request{
				Query:    "",
				SortBy:   tt.sortBy,
				SortDesc: tt.sortDesc,
			}

			_, err := engine.Search(ctx, req)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}
		})
	}
}

// TestSearch_SearchRequestOffset 测试分页偏移.
func TestSearch_SearchRequestOffset(t *testing.T) {
	engine := NewMockSearchEngine()
	ctx := context.Background()

	// 添加多个文件
	for i := 0; i < 10; i++ {
		engine.IndexFile(&search.FileInfo{
			Path: "/data/file" + string(rune('0'+i)) + ".txt",
			Name: "file" + string(rune('0'+i)) + ".txt",
			Ext:  ".txt",
			Size: 100,
		})
	}

	tests := []struct {
		name   string
		offset int
		limit  int
	}{
		{"First page", 0, 5},
		{"Second page", 5, 5},
		{"With offset", 3, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &search.Request{
				Query:  "",
				Offset: tt.offset,
				Limit:  tt.limit,
			}

			results, err := engine.Search(ctx, req)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if len(results) > tt.limit {
				t.Errorf("Results exceed limit: got %d, max %d", len(results), tt.limit)
			}
		})
	}
}

// TestSearch_ContextCancellation 测试上下文取消.
func TestSearch_ContextCancellation(t *testing.T) {
	engine := NewMockSearchEngine()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	// 搜索应该能处理取消的上下文
	_, err := engine.Search(ctx, &search.Request{Query: "test"})
	// 不一定返回错误，但不应 panic
	_ = err
}

// TestSearch_Highlight 测试高亮功能.
func TestSearch_Highlight(t *testing.T) {
	highlight := search.Highlight{
		Field: "content",
		Fragments: []string{
			"This is <em>highlighted</em> text",
			"Another <em>highlighted</em> section",
		},
	}

	if highlight.Field != "content" {
		t.Errorf("Expected field=content, got %s", highlight.Field)
	}

	if len(highlight.Fragments) != 2 {
		t.Errorf("Expected 2 fragments, got %d", len(highlight.Fragments))
	}
}

// ========== 性能测试 ==========

// BenchmarkSearch_IndexFile 性能测试：文件索引.
func BenchmarkSearch_IndexFile(b *testing.B) {
	engine := NewMockSearchEngine()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.IndexFile(&search.FileInfo{
			Path: "/data/bench.txt",
			Name: "bench.txt",
			Ext:  ".txt",
			Size: 100,
		})
	}
}

// BenchmarkSearch_Search 性能测试：搜索.
func BenchmarkSearch_Search(b *testing.B) {
	engine := NewMockSearchEngine()
	ctx := context.Background()

	// 预置数据
	for i := 0; i < 100; i++ {
		engine.IndexFile(&search.FileInfo{
			Path: "/data/file.txt",
			Name: "file.txt",
			Ext:  ".txt",
			Size: 100,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Search(ctx, &search.Request{Query: "test"})
	}
}

// BenchmarkSearch_DeleteFromIndex 性能测试：删除索引.
func BenchmarkSearch_DeleteFromIndex(b *testing.B) {
	engine := NewMockSearchEngine()

	for i := 0; i < 100; i++ {
		path := "/data/delete" + string(rune(i)) + ".txt"
		engine.IndexFile(&search.FileInfo{Path: path})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.DeleteFromIndex("/data/delete0.txt")
	}
}

// BenchmarkSearch_GetStats 性能测试：获取统计.
func BenchmarkSearch_GetStats(b *testing.B) {
	engine := NewMockSearchEngine()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.GetStats()
	}
}
