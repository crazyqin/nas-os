// Package spotlightfull - 测试
package spotlightfull

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- 分词器测试 ----

func TestCJKTokenizer_Tokenize(t *testing.T) {
	tokenizer := newCJKTokenizer()

	tests := []struct {
		name     string
		input    string
		expected []string // 至少包含这些词项
	}{
		{
			name:     "英文分词",
			input:    "hello world test",
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "中文单字和bigram",
			input:    "自然语言处理",
			expected: []string{"自然", "语言", "处理", "自", "然", "语", "言", "处", "理"},
		},
		{
			name:     "中英混合",
			input:    "Go语言编程",
			expected: []string{"Go", "语言", "编", "程", "语", "言"},
		},
		{
			name:     "数字和字母",
			input:    "test123 abc",
			expected: []string{"test123", "abc"},
		},
		{
			name:     "停用词过滤",
			input:    "我是中国人",
			expected: []string{"中国", "中", "国"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizer.tokenize(tt.input)
			tokenSet := make(map[string]bool)
			for _, tok := range tokens {
				tokenSet[tok] = true
			}
			for _, exp := range tt.expected {
				if !tokenSet[exp] {
					t.Errorf("期望包含词项 %q, 但未找到。实际分词结果: %v", exp, tokens)
				}
			}
		})
	}
}

// ---- 文本工具函数测试 ----

func TestTokenizeText(t *testing.T) {
	tests := []struct {
		input    string
		minCount int
	}{
		{"hello world", 2},
		{"test-file_name.go", 1},
		{"中文文件名", 5}, // 每个汉字单独成词
		{"", 0},
	}

	for _, tt := range tests {
		tokens := tokenizeText(tt.input)
		if len(tokens) < tt.minCount {
			t.Errorf("tokenizeText(%q): 期望至少 %d 个词项, 得到 %d", tt.input, tt.minCount, len(tokens))
		}
	}
}

func TestHighlightText(t *testing.T) {
	result := highlightText("这是一个测试文件", "测试")
	if !strings.Contains(result, "【测试】") {
		t.Errorf("高亮结果不正确: %s", result)
	}

	result = highlightText("no match here", "xyz")
	if result != "no match here" {
		t.Errorf("无匹配时不应修改原文: %s", result)
	}
}

func TestExtractSnippets(t *testing.T) {
	content := strings.Repeat("Hello world. ", 100)
	snippets := extractSnippets(content, "world", 3, 20)
	if len(snippets) == 0 {
		t.Error("应该提取到至少一个片段")
	}
	for _, s := range snippets {
		if !strings.Contains(strings.ToLower(s), "world") {
			t.Errorf("片段中应包含关键词: %s", s)
		}
	}
}

func TestSizeGroup(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "empty"},
		{512, "tiny"},
		{1024 * 100, "small"},
		{1024 * 1024 * 50, "medium"},
		{1024 * 1024 * 500, "large"},
		{1024 * 1024 * 1024 * 2, "huge"},
	}

	for _, tt := range tests {
		result := sizeGroup(tt.size)
		if result != tt.expected {
			t.Errorf("sizeGroup(%d): 期望 %s, 得到 %s", tt.size, tt.expected, result)
		}
	}
}

func TestClassifyFileType(t *testing.T) {
	tests := []struct {
		path     string
		expected FileType
	}{
		{"/data/doc.pdf", FileTypeDocument},
		{"/data/photo.jpg", FileTypeImage},
		{"/data/movie.mp4", FileTypeVideo},
		{"/data/song.mp3", FileTypeAudio},
		{"/data/archive.zip", FileTypeArchive},
		{"/data/main.go", FileTypeCode},
		{"/data/unknown.xyz", FileTypeOther},
	}

	for _, tt := range tests {
		result := classifyFileType(tt.path)
		if result != tt.expected {
			t.Errorf("classifyFileType(%q): 期望 %s, 得到 %s", tt.path, tt.expected, result)
		}
	}
}

func TestDetectMimeType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/data/file.html", "text/html; charset=utf-8"},
		{"/data/file.json", "application/json"},
		{"/data/file.unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		result := detectMimeType(tt.path)
		if result != tt.expected {
			t.Errorf("detectMimeType(%q): 期望 %s, 得到 %s", tt.path, tt.expected, result)
		}
	}
}

// ---- 搜索引擎测试 ----

func newTestEngine(t *testing.T) (*SearchEngine, string) {
	t.Helper()
	dir := t.TempDir()
	config := &EngineConfig{
		IndexDir:       dir,
		MaxIndexSize:   1 << 20,
		MinTermLength:  1,
		MaxTermLength:  64,
		EnableCJK:      true,
		BatchSize:      100,
		MaxResults:     1000,
		ThumbnailWidth: 256,
	}
	engine, err := NewSearchEngine(config)
	if err != nil {
		t.Fatalf("创建搜索引擎失败: %v", err)
	}
	return engine, dir
}

func TestSearchEngine_AddAndSearch(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	// 添加文档
	docs := []*IndexEntry{
		{
			ID:         "doc1",
			Path:       "/data/技术文档/Go语言入门.pdf",
			Name:       "Go语言入门.pdf",
			Extension:  ".pdf",
			FileType:   FileTypeDocument,
			Size:       1024 * 100,
			MimeType:   "application/pdf",
			Content:    "Go语言是一门开源的编程语言，由Google开发，并发编程是其核心特性",
			Tags:       []string{"Go", "编程", "入门"},
			Metadata:   map[string]string{"author": "张三"},
			ModifiedAt: time.Now().Add(-24 * time.Hour),
		},
		{
			ID:         "doc2",
			Path:       "/data/照片/风景.jpg",
			Name:       "风景.jpg",
			Extension:  ".jpg",
			FileType:   FileTypeImage,
			Size:       1024 * 1024 * 5,
			MimeType:   "image/jpeg",
			Content:    "",
			Tags:       []string{"风景", "摄影"},
			Metadata:   map[string]string{"camera": "Canon"},
			ModifiedAt: time.Now(),
		},
		{
			ID:         "doc3",
			Path:       "/data/代码/main.go",
			Name:       "main.go",
			Extension:  ".go",
			FileType:   FileTypeCode,
			Size:       2048,
			MimeType:   "text/x-go",
			Content:    "package main\nfunc main() { fmt.Println(\"hello world\") }",
			Tags:       []string{"Go", "代码"},
			Metadata:   map[string]string{},
			ModifiedAt: time.Now().Add(-1 * time.Hour),
		},
	}

	for _, doc := range docs {
		if err := engine.AddDocument(doc); err != nil {
			t.Fatalf("添加文档失败: %v", err)
		}
	}

	// 验证统计
	stats := engine.GetStats()
	if stats.TotalFiles != 3 {
		t.Errorf("索引文件数: 期望 3, 得到 %d", stats.TotalFiles)
	}

	// 测试文件名搜索
	t.Run("文件名搜索", func(t *testing.T) {
		filter := &SearchFilter{Query: "风景", Page: 1, PageSize: 20}
		resp, err := engine.Search(filter)
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}
		if resp.Total == 0 {
			t.Error("应该搜到包含 '风景' 的文档")
		}
		found := false
		for _, r := range resp.Results {
			if r.ID == "doc2" {
				found = true
				if r.MatchType != MatchFileName && r.MatchType != MatchFuzzy {
					t.Errorf("匹配类型应为 filename 或 fuzzy, 得到 %s", r.MatchType)
				}
			}
		}
		if !found {
			t.Error("搜索结果中应包含 doc2")
		}
	})

	// 测试内容搜索
	t.Run("内容搜索", func(t *testing.T) {
		filter := &SearchFilter{Query: "并发编程", Page: 1, PageSize: 20}
		resp, err := engine.Search(filter)
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}
		if resp.Total == 0 {
			t.Error("应该搜到包含 '并发编程' 的文档")
		}
	})

	// 测试模糊搜索
	t.Run("模糊搜索", func(t *testing.T) {
		filter := &SearchFilter{Query: "hello", Page: 1, PageSize: 20}
		resp, err := engine.Search(filter)
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}
		found := false
		for _, r := range resp.Results {
			if r.ID == "doc3" {
				found = true
			}
		}
		if !found {
			t.Error("应该模糊匹配到 doc3 中的 'hello'")
		}
	})

	// 测试文件类型过滤
	t.Run("文件类型过滤", func(t *testing.T) {
		filter := &SearchFilter{
			Query:     "Go",
			FileTypes: []FileType{FileTypeCode},
			Page:      1,
			PageSize:  20,
		}
		resp, err := engine.Search(filter)
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}
		for _, r := range resp.Results {
			if r.FileType != FileTypeCode {
				t.Errorf("过滤后不应出现非代码文件: %s (类型: %s)", r.Path, r.FileType)
			}
		}
	})

	// 测试大小范围过滤
	t.Run("大小范围过滤", func(t *testing.T) {
		minSize := int64(1)
		maxSize := int64(5000)
		filter := &SearchFilter{
			Query:    "Go",
			MinSize:  &minSize,
			MaxSize:  &maxSize,
			Page:     1,
			PageSize: 20,
		}
		resp, err := engine.Search(filter)
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}
		for _, r := range resp.Results {
			if r.Size > maxSize || r.Size < minSize {
				t.Errorf("文件大小超出范围: %s (大小: %d)", r.Path, r.Size)
			}
		}
	})

	// 测试路径范围过滤
	t.Run("路径范围过滤", func(t *testing.T) {
		filter := &SearchFilter{
			Query:     "Go",
			PathScope: "/data/代码/",
			Page:      1,
			PageSize:  20,
		}
		resp, err := engine.Search(filter)
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}
		for _, r := range resp.Results {
			if !strings.HasPrefix(r.Path, "/data/代码/") {
				t.Errorf("路径过滤后不应出现其他路径的文件: %s", r.Path)
			}
		}
	})
}

func TestSearchEngine_RemoveDocument(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	entry := &IndexEntry{
		ID:         "test-remove",
		Path:       "/data/test.txt",
		Name:       "test.txt",
		Extension:  ".txt",
		FileType:   FileTypeDocument,
		Size:       100,
		Content:    "测试内容 搜索关键词",
		ModifiedAt: time.Now(),
	}

	engine.AddDocument(entry)

	// 确认文档存在
	filter := &SearchFilter{Query: "搜索关键词", Page: 1, PageSize: 20}
	resp, _ := engine.Search(filter)
	if resp.Total == 0 {
		t.Fatal("添加后应该能搜到文档")
	}

	// 移除文档
	engine.RemoveDocument("test-remove")

	// 确认文档已移除
	resp, _ = engine.Search(filter)
	if resp.Total != 0 {
		t.Error("移除后不应该搜到文档")
	}
}

func TestSearchEngine_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	config := &EngineConfig{
		IndexDir:     dir,
		MaxIndexSize: 1 << 20,
		EnableCJK:    true,
		BatchSize:    100,
		MaxResults:   1000,
	}

	// 创建引擎并添加文档
	engine1, _ := NewSearchEngine(config)
	entry := &IndexEntry{
		ID:         "persist-test",
		Path:       "/data/persist.txt",
		Name:       "persist.txt",
		Extension:  ".txt",
		FileType:   FileTypeDocument,
		Size:       200,
		Content:    "持久化测试数据",
		ModifiedAt: time.Now(),
	}
	engine1.AddDocument(entry)

	// 保存索引
	if err := engine1.SaveIndex(); err != nil {
		t.Fatalf("保存索引失败: %v", err)
	}
	engine1.Stop()

	// 创建新引擎，应自动加载索引
	engine2, _ := NewSearchEngine(config)
	defer engine2.Stop()

	filter := &SearchFilter{Query: "持久化测试", Page: 1, PageSize: 20}
	resp, err := engine2.Search(filter)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if resp.Total == 0 {
		t.Error("加载索引后应该能搜到持久化的文档")
	}
}

func TestSearchEngine_Pagination(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	// 添加多个文档
	for i := 0; i < 25; i++ {
		entry := &IndexEntry{
			ID:         fmt.Sprintf("page-doc-%d", i),
			Path:       fmt.Sprintf("/data/page/doc_%d.txt", i),
			Name:       fmt.Sprintf("doc_%d.txt", i),
			Extension:  ".txt",
			FileType:   FileTypeDocument,
			Size:       int64(i * 100),
			Content:    "分页测试文档内容",
			ModifiedAt: time.Now(),
		}
		engine.AddDocument(entry)
	}

	// 第一页
	filter := &SearchFilter{Query: "分页测试", Page: 1, PageSize: 10}
	resp, err := engine.Search(filter)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if len(resp.Results) != 10 {
		t.Errorf("第一页应返回10条结果, 得到 %d", len(resp.Results))
	}
	if resp.Total != 25 {
		t.Errorf("总数应为25, 得到 %d", resp.Total)
	}

	// 第三页（最后一页）
	filter.Page = 3
	resp, err = engine.Search(filter)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if len(resp.Results) != 5 {
		t.Errorf("最后一页应返回5条结果, 得到 %d", len(resp.Results))
	}
}

func TestSearchEngine_EmptyQuery(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	filter := &SearchFilter{Query: "", Page: 1, PageSize: 20}
	resp, err := engine.Search(filter)
	if err != nil {
		t.Fatalf("空查询不应报错: %v", err)
	}
	if resp.Total != 0 {
		t.Error("空查询应返回0结果")
	}
}

// ---- 索引器测试 ----

func TestFileIndexer_FullScan(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	// 创建测试目录结构
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdata")
	os.MkdirAll(testDir, 0755)

	// 创建测试文件
	os.WriteFile(filepath.Join(testDir, "readme.txt"), []byte("这是一个测试文档"), 0644)
	os.WriteFile(filepath.Join(testDir, "code.go"), []byte("package main\nfunc main() {}"), 0644)
	os.WriteFile(filepath.Join(testDir, ".hidden"), []byte("隐藏文件"), 0644) // 应被跳过

	config := &IndexerConfig{
		ScanPaths:       []string{testDir},
		ExcludePatterns: []string{".git", ".svn"},
		MaxFileSize:     10 << 20,
		ScanInterval:    time.Minute,
		EnableSSDOpt:    true,
		WorkerCount:     2,
	}

	indexer := NewFileIndexer(engine, config)
	ctx := context.Background()

	if err := indexer.RunFullScan(ctx); err != nil {
		t.Fatalf("全量扫描失败: %v", err)
	}

	// 验证索引内容
	stats := engine.GetStats()
	if stats.TotalFiles < 2 {
		t.Errorf("应索引至少2个文件, 得到 %d", stats.TotalFiles)
	}

	// 测试搜索索引的内容
	filter := &SearchFilter{Query: "测试文档", Page: 1, PageSize: 20}
	resp, err := engine.Search(filter)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if resp.Total == 0 {
		t.Error("应该能搜到索引的文档内容")
	}
}

func TestFileIndexer_ExcludedPaths(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	tmpDir := t.TempDir()

	// 创建 .git 目录（应被排除）
	gitDir := filepath.Join(tmpDir, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0644)

	// 创建正常文件
	os.WriteFile(filepath.Join(tmpDir, "normal.txt"), []byte("正常文件"), 0644)

	config := &IndexerConfig{
		ScanPaths:       []string{tmpDir},
		ExcludePatterns: []string{".git"},
		MaxFileSize:     10 << 20,
		WorkerCount:     1,
	}

	indexer := NewFileIndexer(engine, config)
	indexer.RunFullScan(context.Background())

	// .git 目录下的文件不应被索引
	filter := &SearchFilter{Query: "git config", Page: 1, PageSize: 20}
	resp, _ := engine.Search(filter)
	for _, r := range resp.Results {
		if strings.Contains(r.Path, ".git") {
			t.Errorf("不应索引 .git 目录下的文件: %s", r.Path)
		}
	}
}

func TestFileIndexer_IsRunning(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	config := &IndexerConfig{
		ScanPaths:   []string{t.TempDir()},
		WorkerCount: 1,
	}

	indexer := NewFileIndexer(engine, config)
	if indexer.IsRunning() {
		t.Error("索引器初始状态不应为运行中")
	}
}

// ---- HTTP Handlers 测试 ----

func TestHandlers_Search(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	// 添加测试数据
	engine.AddDocument(&IndexEntry{
		ID:         "http-test-1",
		Path:       "/data/handler_test.txt",
		Name:       "handler_test.txt",
		Extension:  ".txt",
		FileType:   FileTypeDocument,
		Size:       500,
		Content:    "HTTP handler 测试内容",
		ModifiedAt: time.Now(),
	})

	indexer := NewFileIndexer(engine, nil)
	handlers := NewHandlers(engine, indexer)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// 测试 GET /api/v1/search
	t.Run("GET搜索", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=handler测试&type=document", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200, 得到 %d", w.Code)
		}

		var resp APIResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Code != 200 {
			t.Errorf("响应码应为 200, 得到 %d", resp.Code)
		}
	})

	// 测试缺少参数
	t.Run("缺少q参数", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("期望状态码 400, 得到 %d", w.Code)
		}
	})

	// 测试 POST 搜索
	t.Run("POST搜索", func(t *testing.T) {
		body := strings.NewReader(`{"query":"handler测试","page":1,"page_size":20}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/search", body)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200, 得到 %d", w.Code)
		}
	})
}

func TestHandlers_Stats(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	indexer := NewFileIndexer(engine, nil)
	handlers := NewHandlers(engine, indexer)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/index/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 得到 %d", w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != 200 {
		t.Errorf("响应码应为 200, 得到 %d", resp.Code)
	}
}

func TestHandlers_Rebuild(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	indexer := NewFileIndexer(engine, nil)
	handlers := NewHandlers(engine, indexer)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// 同步重建
	t.Run("同步重建", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/index/rebuild", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200, 得到 %d", w.Code)
		}
	})

	// 异步重建
	t.Run("异步重建", func(t *testing.T) {
		body := strings.NewReader(`{"async":true}`)
		req := httptest.NewRequest(http.MethodPost, "/index/rebuild", body)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("期望状态码 202, 得到 %d", w.Code)
		}
	})
}

func TestHandlers_MethodNotAllowed(t *testing.T) {
	engine, _ := newTestEngine(t)
	defer engine.Stop()

	indexer := NewFileIndexer(engine, nil)
	handlers := NewHandlers(engine, indexer)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// PUT 方法不应被允许
	req := httptest.NewRequest(http.MethodPut, "/api/v1/search?q=test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("期望状态码 405, 得到 %d", w.Code)
	}
}

func TestParseFileTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"document", 1},
		{"image,video", 2},
		{"invalid", 0},
		{"", 0},
		{"Document", 1}, // 大小写不敏感
	}

	for _, tt := range tests {
		result := parseFileTypes(tt.input)
		if len(result) != tt.expected {
			t.Errorf("parseFileTypes(%q): 期望 %d 个类型, 得到 %d", tt.input, tt.expected, len(result))
		}
	}
}

func TestParseTimePtr(t *testing.T) {
	// 有效日期
	result := parseTimePtr("2024-01-15")
	if result == nil {
		t.Error("应该解析有效日期")
	}

	// 有效 RFC3339
	result = parseTimePtr("2024-01-15T10:30:00Z")
	if result == nil {
		t.Error("应该解析有效 RFC3339 时间")
	}

	// 空字符串
	result = parseTimePtr("")
	if result != nil {
		t.Error("空字符串应返回 nil")
	}

	// 无效格式
	result = parseTimePtr("invalid")
	if result != nil {
		t.Error("无效格式应返回 nil")
	}
}

func TestGenerateDocID(t *testing.T) {
	id1 := generateDocID("/data/test.txt")
	id2 := generateDocID("/data/test.txt")
	id3 := generateDocID("/data/other.txt")

	if id1 != id2 {
		t.Error("相同路径应生成相同ID")
	}
	if id1 == id3 {
		t.Error("不同路径应生成不同ID")
	}
}
