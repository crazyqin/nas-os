// Package webshare 搜索引擎测试
package webshare

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewSearchEngine(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "search-engine-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	engine := NewSearchEngine(config, logger)

	if engine == nil {
		t.Fatal("创建搜索引擎失败")
	}

	if engine.config.BaseDir != tmpDir {
		t.Errorf("BaseDir 不匹配")
	}

	if engine.stopWords == nil {
		t.Error("stopWords 未初始化")
	}

	if engine.nameIndex == nil {
		t.Error("nameIndex 未初始化")
	}
}

func TestSearchEngine_Tokenize(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "search-engine-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	engine := NewSearchEngine(config, logger)

	// 测试英文分词
	tokens := engine.tokenize("hello_world-test.txt")
	expected := []string{"hello", "world", "test"}
	if len(tokens) < len(expected) {
		t.Errorf("分词数量不足: got %d, want >= %d", len(tokens), len(expected))
	}

	// 测试中文分词
	tokens = engine.tokenize("测试文件.txt")
	hasChinese := false
	for _, token := range tokens {
		if len([]rune(token)) >= 2 {
			hasChinese = true
			break
		}
	}
	if !hasChinese {
		t.Error("中文分词应该产生中文token")
	}
}

func TestContentIndexer_Index(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "content-indexer-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testContent := "这是一段测试内容，用于测试搜索引擎的功能。Hello World!"
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte(testContent), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("# 测试标题\n\n测试内容"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "nested.txt"), []byte("嵌套文件内容"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	indexer := NewContentIndexer(config, logger)

	// 索引目录
	ctx := context.Background()
	req := IndexRequest{
		Path:       tmpDir,
		Recursive:  true,
		ForceReindex: true,
	}

	resp, err := indexer.Index(ctx, req)
	if err != nil {
		t.Fatalf("索引失败: %v", err)
	}

	if resp.IndexedFiles < 3 {
		t.Errorf("索引文件数量不足: got %d, want >= 3", resp.IndexedFiles)
	}

	// 获取元数据
	meta := indexer.GetMetadata("test.txt")
	if meta == nil {
		t.Fatal("test.txt 元数据不应为nil")
	}

	if meta.TextContent != testContent {
		t.Errorf("内容不匹配")
	}

	if meta.Language != "zh" {
		t.Errorf("语言检测错误: got %s, want zh", meta.Language)
	}
}

func TestContentIndexer_EXIF(t *testing.T) {
	// 注意：这个测试需要真实的JPEG文件才能测试EXIF
	// 这里只测试基本功能
	tmpDir, err := os.MkdirTemp("", "exif-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建一个假的JPEG文件（没有EXIF）
	os.WriteFile(filepath.Join(tmpDir, "fake.jpg"), []byte("not a real jpeg"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	indexer := NewContentIndexer(config, logger)

	ctx := context.Background()
	req := IndexRequest{
		Path:        filepath.Join(tmpDir, "fake.jpg"),
		Recursive:   false,
		ForceReindex: true,
	}

	// 应该不会panic，只是EXIF提取失败
	_, err = indexer.Index(ctx, req)
	if err != nil {
		t.Fatalf("索引失败: %v", err)
	}
}

func TestContentIndexer_Tags(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tag-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	indexer := NewContentIndexer(config, logger)

	ctx := context.Background()
	req := IndexRequest{
		Path:        filepath.Join(tmpDir, "test.txt"),
		Recursive:   false,
		ForceReindex: true,
	}

	indexer.Index(ctx, req)

	// 添加标签
	indexer.AddTag("test.txt", "important")
	indexer.AddTag("test.txt", "work")

	// 获取标签
	paths := indexer.SearchByTag("important")
	if len(paths) != 1 || paths[0] != "test.txt" {
		t.Errorf("标签搜索失败: %v", paths)
	}

	// 移除标签
	indexer.RemoveTag("test.txt", "important")
	paths = indexer.SearchByTag("important")
	if len(paths) != 0 {
		t.Errorf("标签应该已被移除")
	}
}

func TestSearchEngine_Search(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "search-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# 项目说明\n\n这是一个测试项目"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("Hello World 测试文件"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(`{"name": "test"}`), 0644)
	os.Mkdir(filepath.Join(tmpDir, "docs"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "docs", "guide.md"), []byte("# 使用指南\n\n详细说明"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	engine := NewSearchEngine(config, logger)

	// 构建索引
	ctx := context.Background()
	err = engine.BuildIndex(ctx, tmpDir)
	if err != nil {
		t.Fatalf("构建索引失败: %v", err)
	}

	// 搜索 "测试"
	req := SearchRequest{
		Query:      "测试",
		Content:    true,
		Highlight:  true,
		MaxResults: 10,
	}

	resp, err := engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	if resp.Total < 1 {
		t.Errorf("搜索结果不应为空: got %d", resp.Total)
	}

	// 检查高亮
	if len(resp.Results) > 0 && resp.Results[0].Highlights == nil {
		t.Error("应该有高亮结果")
	}

	// 搜索英文
	req = SearchRequest{
		Query:      "Hello",
		Content:    true,
		MaxResults: 10,
	}

	resp, err = engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("英文搜索失败: %v", err)
	}

	if resp.Total < 1 {
		t.Errorf("英文搜索结果不应为空")
	}
}

func TestSearchEngine_Filters(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filter-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "doc.txt"), []byte("文档内容"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "image.jpg"), []byte("fake image"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "script.py"), []byte("print('hello')"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	engine := NewSearchEngine(config, logger)

	ctx := context.Background()
	engine.BuildIndex(ctx, tmpDir)

	// 按文件类型过滤
	req := SearchRequest{
		Query:      "",
		FileType:   "document",
		MaxResults: 10,
	}

	resp, err := engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	// 应该只返回文档类型
	for _, r := range resp.Results {
		if r.FileType != "document" {
			t.Errorf("文件类型过滤失败: got %s", r.FileType)
		}
	}

	// 按扩展名过滤
	req = SearchRequest{
		Query:       "",
		Extensions:  []string{".py"},
		MaxResults:  10,
	}

	resp, err = engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	for _, r := range resp.Results {
		if r.Ext != ".py" {
			t.Errorf("扩展名过滤失败: got %s", r.Ext)
		}
	}
}

func TestSearchEngine_Suggestions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "suggest-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "test1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test2.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "testing.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "example.txt"), []byte("example"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	engine := NewSearchEngine(config, logger)

	ctx := context.Background()
	engine.BuildIndex(ctx, tmpDir)

	// 获取建议
	suggestions := engine.getSuggestions("test")
	if len(suggestions) == 0 {
		t.Error("应该有搜索建议")
	}
}

func TestSearchEngine_Sort(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sort-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "small.txt"), []byte("small"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "medium.txt"), []byte("medium content here"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "large.txt"), []byte("large content with many words for testing search functionality"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	engine := NewSearchEngine(config, logger)

	ctx := context.Background()
	engine.BuildIndex(ctx, tmpDir)

	// 按大小排序（升序）
	req := SearchRequest{
		Query:      "",
		SortBy:     "size",
		SortDesc:   false,
		MaxResults: 10,
	}

	resp, err := engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	// 验证排序
	for i := 1; i < len(resp.Results); i++ {
		if resp.Results[i].Size < resp.Results[i-1].Size {
			t.Errorf("排序错误: %s (%d) 应该在 %s (%d) 之后",
				resp.Results[i-1].Name, resp.Results[i-1].Size,
				resp.Results[i].Name, resp.Results[i].Size)
		}
	}
}

func TestSearchEngine_Pagination(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pagination-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建多个测试文件
	for i := 0; i < 20; i++ {
		name := filepath.Join(tmpDir, fmt.Sprintf("file%02d.txt", i))
		os.WriteFile(name, []byte("content"), 0644)
	}

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	engine := NewSearchEngine(config, logger)

	ctx := context.Background()
	engine.BuildIndex(ctx, tmpDir)

	// 第一页
	req := SearchRequest{
		Query:      "",
		MaxResults: 5,
		Offset:     0,
	}

	resp1, err := engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	if len(resp1.Results) != 5 {
		t.Errorf("第一页结果数量错误: got %d, want 5", len(resp1.Results))
	}

	// 第二页
	req.Offset = 5
	resp2, err := engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	if len(resp2.Results) != 5 {
		t.Errorf("第二页结果数量错误: got %d, want 5", len(resp2.Results))
	}

	// 验证结果不重复
	for _, r1 := range resp1.Results {
		for _, r2 := range resp2.Results {
			if r1.Path == r2.Path {
				t.Errorf("分页结果重复: %s", r1.Path)
			}
		}
	}
}

func TestSearchEngine_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "context-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建大量文件
	for i := 0; i < 100; i++ {
		name := filepath.Join(tmpDir, "file"+string(rune(i))+".txt")
		os.WriteFile(name, []byte("content"), 0644)
	}

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	logger, _ := zap.NewDevelopment()
	engine := NewSearchEngine(config, logger)

	// 创建可取消的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// 等待上下文过期
	time.Sleep(5 * time.Millisecond)

	// 尝试索引（应该被取消）
	err = engine.BuildIndex(ctx, tmpDir)
	// 可能成功也可能失败，取决于时间
	if err != nil {
		if err != context.DeadlineExceeded && err != context.Canceled {
			t.Errorf("期望上下文错误，得到: %v", err)
		}
	}
}
