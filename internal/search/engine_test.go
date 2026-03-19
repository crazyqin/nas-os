package search

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestEngine_BasicSearch(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "search-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testFiles := map[string]string{
		"test1.txt":         "Hello World, this is a test file for search engine.",
		"test2.txt":         "Go programming language is awesome for building search.",
		"test3.md":          "# Markdown File\n\nThis is a markdown file with search content.",
		"subdir/nested.txt": "Nested file with search keyword.",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 创建索引目录
	indexDir, err := os.MkdirTemp("", "search-index-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(indexDir)

	// 创建搜索引擎
	config := IndexConfig{
		IndexPath:    filepath.Join(indexDir, "test.bleve"),
		MaxFileSize:  1024 * 1024,
		Workers:      2,
		IndexContent: true,
		BatchSize:    10,
		TextExts:     []string{".txt", ".md"},
		ExcludeDirs:  []string{},
		ExcludeFiles: []string{},
	}

	logger := zap.NewNop()
	engine, err := NewEngine(config, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 索引目录
	if err := engine.IndexDirectory(tmpDir); err != nil {
		t.Fatal(err)
	}

	// 测试搜索
	tests := []struct {
		name     string
		query    string
		minCount int
	}{
		{"搜索 search", "search", 4},
		{"搜索 file", "file", 3},
		{"搜索 Go", "Go", 1},
		{"搜索 markdown", "markdown", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Search(Request{
				Query: tt.query,
				Limit: 10,
			})
			if err != nil {
				t.Fatal(err)
			}

			if result.Total < tt.minCount {
				t.Errorf("期望至少 %d 个结果，实际 %d", tt.minCount, result.Total)
			}

			t.Logf("查询 '%s' 找到 %d 个结果", tt.query, result.Total)
			for _, hit := range result.Results {
				t.Logf("  - %s (score: %.2f)", hit.Path, hit.Score)
			}
		})
	}
}

func TestEngine_FileTypeFilter(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "search-filter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建不同类型的文件
	testFiles := map[string]string{
		"doc.txt":     "Text file with search keyword",
		"code.go":     "package main\n\n// search function",
		"config.json": `{"key": "search", "value": 123}`,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 创建索引目录
	indexDir, err := os.MkdirTemp("", "search-filter-index-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(indexDir)

	// 创建搜索引擎
	config := IndexConfig{
		IndexPath:    filepath.Join(indexDir, "test.bleve"),
		MaxFileSize:  1024 * 1024,
		Workers:      2,
		IndexContent: true,
		BatchSize:    10,
		TextExts:     []string{".txt", ".go", ".json"},
		ExcludeDirs:  []string{},
		ExcludeFiles: []string{},
	}

	logger := zap.NewNop()
	engine, err := NewEngine(config, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 索引目录
	if err := engine.IndexDirectory(tmpDir); err != nil {
		t.Fatal(err)
	}

	// 只搜索 .txt 文件
	result, err := engine.Search(Request{
		Query: "search",
		Types: []string{".txt"},
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Total != 1 {
		t.Errorf("期望 1 个结果，实际 %d", result.Total)
	}
}

func TestEngine_Highlight(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "search-highlight-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	content := "This is a test file with search keyword. Search is important for finding content."
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 创建索引目录
	indexDir, err := os.MkdirTemp("", "search-highlight-index-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(indexDir)

	// 创建搜索引擎
	config := IndexConfig{
		IndexPath:    filepath.Join(indexDir, "test.bleve"),
		MaxFileSize:  1024 * 1024,
		Workers:      2,
		IndexContent: true,
		BatchSize:    10,
		TextExts:     []string{".txt"},
		ExcludeDirs:  []string{},
		ExcludeFiles: []string{},
	}

	logger := zap.NewNop()
	engine, err := NewEngine(config, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 索引目录
	if err := engine.IndexDirectory(tmpDir); err != nil {
		t.Fatal(err)
	}

	// 搜索并检查高亮
	result, err := engine.Search(Request{
		Query: "search",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Total == 0 {
		t.Fatal("期望至少 1 个结果")
	}

	// 检查高亮
	for _, hit := range result.Results {
		if len(hit.Highlights) > 0 {
			t.Logf("高亮结果:")
			for _, h := range hit.Highlights {
				t.Logf("  字段 %s:", h.Field)
				for _, f := range h.Fragments {
					t.Logf("    %s", f)
				}
			}
		}
	}
}

func TestHighlighter(t *testing.T) {
	h := NewHighlighter()

	text := "This is a test text with search keyword for testing search functionality."
	query := "search"

	highlighted := h.HighlightText(text, query)
	t.Logf("高亮结果: %s", highlighted)

	fragments := h.HighlightWithContext(text, query, 20)
	t.Logf("带上下文的片段:")
	for _, f := range fragments {
		t.Logf("  %s", f)
	}
}

func TestEngine_Stats(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "search-stats-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	for i := 0; i < 5; i++ {
		content := "Test file " + string(rune('a'+i))
		if err := os.WriteFile(filepath.Join(tmpDir, "test"+string(rune('0'+i))+".txt"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 创建索引目录
	indexDir, err := os.MkdirTemp("", "search-stats-index-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(indexDir)

	// 创建搜索引擎
	config := IndexConfig{
		IndexPath:    filepath.Join(indexDir, "test.bleve"),
		MaxFileSize:  1024 * 1024,
		Workers:      2,
		IndexContent: true,
		BatchSize:    10,
		TextExts:     []string{".txt"},
		ExcludeDirs:  []string{},
		ExcludeFiles: []string{},
	}

	logger := zap.NewNop()
	engine, err := NewEngine(config, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 索引目录
	if err := engine.IndexDirectory(tmpDir); err != nil {
		t.Fatal(err)
	}

	// 获取统计
	stats := engine.Stats()
	t.Logf("索引统计:")
	t.Logf("  总文件数: %d", stats.TotalFiles)
	t.Logf("  已索引文件数: %d", stats.IndexedFiles)
	t.Logf("  索引耗时: %v", stats.IndexDuration)
	t.Logf("  最后索引时间: %v", stats.LastIndexed)

	if stats.IndexedFiles < 5 {
		t.Errorf("期望至少 5 个已索引文件，实际 %d", stats.IndexedFiles)
	}
}
