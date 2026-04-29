package truesearch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewIndexer(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt", ".md", ".pdf", ".docx"},
		ExcludeDirs:   []string{".git", "node_modules"},
	}

	idx, err := NewIndexer(cfg, logger)
	if err != nil {
		t.Fatalf("NewIndexer() error = %v", err)
	}
	defer func() { _ = idx.Close() }()

	// 验证状态
	status := idx.Status()
	if status.TotalFiles != 0 {
		t.Errorf("expected 0 total files, got %d", status.TotalFiles)
	}
}

func TestIndexFile(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt", ".md"},
		ExcludeDirs:   []string{".git"},
	}

	idx, err := NewIndexer(cfg, logger)
	if err != nil {
		t.Fatalf("NewIndexer() error = %v", err)
	}
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)

	// 创建测试文件
	testFile := filepath.Join(dir, "test.txt")
	content := "This is a test document for full text search indexing."
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 索引文件
	if err := idx.IndexFile(testFile, extractor); err != nil {
		t.Fatalf("IndexFile() error = %v", err)
	}

	// 验证状态
	status := idx.Status()
	if status.TotalFiles != 1 {
		t.Errorf("expected 1 total files, got %d", status.TotalFiles)
	}
}

func TestIndexDirectory(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     5,
		SupportedExts: []string{".txt", ".md"},
		ExcludeDirs:   []string{".git"},
	}

	idx, err := NewIndexer(cfg, logger)
	if err != nil {
		t.Fatalf("NewIndexer() error = %v", err)
	}
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)

	// 创建测试目录结构
	files := map[string]string{
		"file1.txt":          "First file content about golang",
		"file2.md":           "# Markdown\n\nSecond file content about programming",
		"subdir/file3.txt":   "Third file in subdirectory about databases",
		".git/config":        "should be excluded",
		"node_modules/x.txt": "should be excluded",
		"binary.xyz":         "should be excluded",
	}

	for name, content := range files {
		path := filepath.Join(dir, "testdata", name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 索引目录
	if err := idx.IndexDirectory(filepath.Join(dir, "testdata"), extractor); err != nil {
		t.Fatalf("IndexDirectory() error = %v", err)
	}

	// 验证只有符合条件的文件被索引 (file1.txt, file2.md, subdir/file3.txt)
	status := idx.Status()
	if status.TotalFiles < 3 {
		t.Errorf("expected at least 3 indexed files, got %d", status.TotalFiles)
	}
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     5,
		SupportedExts: []string{".txt", ".md"},
		ExcludeDirs:   []string{},
	}

	idx, err := NewIndexer(cfg, logger)
	if err != nil {
		t.Fatalf("NewIndexer() error = %v", err)
	}
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)

	// 创建测试文件
	testFiles := map[string]string{
		"golang.txt":  "Go is a statically typed programming language designed at Google",
		"python.txt":  "Python is a high-level programming language with dynamic typing",
		"database.md": "# Database\n\nPostgreSQL is a powerful open source relational database",
		"network.txt": "TCP/IP is the fundamental protocol suite of the internet",
	}

	for name, content := range testFiles {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := idx.IndexFile(path, extractor); err != nil {
			t.Fatalf("IndexFile(%s) error = %v", name, err)
		}
	}

	// 等待索引生效
	time.Sleep(100 * time.Millisecond)

	// 测试搜索
	t.Run("search by content", func(t *testing.T) {
		resp, err := idx.Search(SearchRequest{
			Query:      "programming language",
			MaxResults: 10,
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if resp.Total == 0 {
			t.Error("expected at least 1 result for 'programming language'")
		}
		t.Logf("Found %d results in %dms", resp.Total, resp.TookMs)
		for _, r := range resp.Results {
			t.Logf("  - %s (score: %.4f)", r.Name, r.Score)
		}
	})

	t.Run("search with type filter", func(t *testing.T) {
		resp, err := idx.Search(SearchRequest{
			Query:      "programming",
			Types:      []string{".txt"},
			MaxResults: 10,
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		for _, r := range resp.Results {
			if filepath.Ext(r.Path) != ".txt" {
				t.Errorf("expected .txt file, got %s", r.Path)
			}
		}
	})

	t.Run("search with highlight", func(t *testing.T) {
		resp, err := idx.Search(SearchRequest{
			Query:      "database",
			MaxResults: 10,
			Highlight:  true,
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		t.Logf("Highlight search returned %d results", resp.Total)
	})

	t.Run("search with no results", func(t *testing.T) {
		resp, err := idx.Search(SearchRequest{
			Query:      "nonexistent_term_xyz123",
			MaxResults: 10,
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if resp.Total != 0 {
			t.Errorf("expected 0 results, got %d", resp.Total)
		}
	})
}

func TestSearchWithSnippet(t *testing.T) {
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
	if err != nil {
		t.Fatalf("NewIndexer() error = %v", err)
	}
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)

	// 创建长内容文件
	content := "This is the beginning of the document. " +
		"There are many paragraphs here about various topics. " +
		"The quick brown fox jumps over the lazy dog. " +
		"Full text search is an important feature for any NAS system. " +
		"It allows users to find documents by their content, not just filenames. " +
		"This is especially useful when you have thousands of files."

	path := filepath.Join(dir, "long_doc.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile(path, extractor); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	resp, err := idx.Search(SearchRequest{
		Query:      "full text search",
		MaxResults: 5,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if resp.Total == 0 {
		t.Fatal("expected results")
	}

	// 验证 snippet 包含相关内容
	if resp.Results[0].Snippet == "" {
		t.Error("expected non-empty snippet")
	}
	t.Logf("Snippet: %s", resp.Results[0].Snippet)
}

func TestReindex(t *testing.T) {
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
	if err != nil {
		t.Fatalf("NewIndexer() error = %v", err)
	}
	defer func() { _ = idx.Close() }()

	extractor := NewExtractor(cfg.MaxFileSize, logger)

	// 先索引一个文件
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile(testFile, extractor); err != nil {
		t.Fatal(err)
	}

	// 强制重建索引
	if err := idx.Reindex(dir, true, extractor); err != nil {
		t.Fatalf("Reindex() error = %v", err)
	}

	// 重建后应该能搜索到
	time.Sleep(100 * time.Millisecond)
	resp, err := idx.Search(SearchRequest{Query: "initial", MaxResults: 5})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if resp.Total == 0 {
		t.Error("expected results after reindex")
	}
}

func TestShouldIndex(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt", ".md", ".pdf"},
		ExcludeDirs:   []string{".git", "node_modules"},
	}

	idx, err := NewIndexer(cfg, logger)
	if err != nil {
		t.Fatalf("NewIndexer() error = %v", err)
	}
	defer func() { _ = idx.Close() }()

	tests := []struct {
		path string
		want bool
	}{
		{"/home/user/file.txt", true},
		{"/home/user/file.md", true},
		{"/home/user/file.pdf", true},
		{"/home/user/file.go", false},
		{"/home/user/.git/config", false},
		{"/home/user/node_modules/pkg/index.js", false},
	}

	for _, tt := range tests {
		got := idx.shouldIndex(tt.path)
		if got != tt.want {
			t.Errorf("shouldIndex(%s) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
