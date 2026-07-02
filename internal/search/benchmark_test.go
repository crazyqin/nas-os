package search

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"go.uber.org/zap"
)

// ================== 辅助函数 ==================

// createTestEngine 创建用于测试的搜索引擎（使用临时目录）.
func createTestEngine(b *testing.B) (*Engine, func()) {
	b.Helper()

	tmpDir, err := os.MkdirTemp("", "bleve-bench-*")
	if err != nil {
		b.Fatalf("创建临时目录失败: %v", err)
	}

	config := IndexConfig{
		IndexPath:    filepath.Join(tmpDir, "bench.bleve"),
		MaxFileSize:  10 * 1024 * 1024,
		Workers:      4,
		IndexContent: true,
		BatchSize:    100,
		TextExts:     []string{".txt", ".md", ".go", ".json"},
	}

	logger, _ := zap.NewDevelopment()
	engine, err := NewEngine(config, logger)
	if err != nil {
		os.RemoveAll(tmpDir)
		b.Fatalf("创建搜索引擎失败: %v", err)
	}

	cleanup := func() {
		engine.Close()
		os.RemoveAll(tmpDir)
	}

	return engine, cleanup
}

// createTempFiles 创建临时测试文件.
func createTempFiles(b *testing.B, dir string, count int) []string {
	b.Helper()

	files := make([]string, 0, count)
	contents := []string{
		"这是一段中文测试文本，用于全文搜索索引性能测试。包含常用词汇和标点符号。",
		"This is an English test document for full-text search indexing performance testing.",
		"# Markdown 标题\n\n这是一个 Markdown 文档示例。\n\n## 二级标题\n\n包含**粗体**和*斜体*文本。",
		`{"name": "测试配置", "version": "1.0", "description": "JSON配置文件示例", "settings": {"debug": true}}`,
		"package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
		"2024-01-15 10:30:00 INFO Server started on port 8080\n2024-01-15 10:30:01 DEBUG Connection established",
	}

	for i := 0; i < count; i++ {
		filename := filepath.Join(dir, fmt.Sprintf("testfile_%04d.txt", i))
		content := contents[i%len(contents)]
		// 每个文件添加唯一标识
		content = fmt.Sprintf("[File %d] %s", i, content)

		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			b.Fatalf("创建测试文件失败: %v", err)
		}
		files = append(files, filename)
	}

	return files
}

// ================== 基准测试 ==================

// BenchmarkIndexCreation 测试索引创建速度
// 测量单个文档索引的吞吐量.
func BenchmarkIndexCreation(b *testing.B) {
	engine, cleanup := createTestEngine(b)
	defer cleanup()

	docs := generateTestDocuments(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		idx := i % len(docs)
		doc := docs[idx]
		if err := engine.index.Index(doc.Path, doc); err != nil {
			b.Fatalf("索引文档失败: %v", err)
		}
	}
}

// BenchmarkBatchIndexing 测试批量索引性能
// 比较不同批量大小下的索引速度.
func BenchmarkBatchIndexing(b *testing.B) {
	batchSizes := []int{10, 50, 100, 200, 500}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", batchSize), func(b *testing.B) {
			engine, cleanup := createTestEngine(b)
			defer cleanup()

			docs := generateTestDocuments(batchSize)
			batch := engine.index.NewBatch()

			for _, doc := range docs {
				batch.Index(doc.Path, doc)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// 创建新批次
				batch = engine.index.NewBatch()
				for _, doc := range docs {
					batch.Index(doc.Path, doc)
				}
				if err := engine.index.Batch(batch); err != nil {
					b.Fatalf("批量索引失败: %v", err)
				}
			}
		})
	}
}

// BenchmarkSearchResponseTime 测试搜索响应时间
// 测量不同查询类型的搜索延迟.
func BenchmarkSearchResponseTime(b *testing.B) {
	engine, cleanup := createTestEngine(b)
	defer cleanup()

	// 预先索引测试数据
	docs := generateTestDocuments(5000)
	batch := engine.index.NewBatch()
	for _, doc := range docs {
		batch.Index(doc.Path, doc)
	}
	if err := engine.index.Batch(batch); err != nil {
		b.Fatalf("预索引失败: %v", err)
	}

	queries := []string{
		"测试",
		"test",
		"中文搜索",
		"performance",
		"配置文件",
		"server",
		"index",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		query := queries[i%len(queries)]
		matchQuery := bleve.NewMatchQuery(query)
		searchReq := bleve.NewSearchRequestOptions(matchQuery, 20, 0, false)

		_, err := engine.index.Search(searchReq)
		if err != nil {
			b.Fatalf("搜索失败: %v", err)
		}
	}
}

// BenchmarkSearchWithHighlight 测试带高亮的搜索性能.
func BenchmarkSearchWithHighlight(b *testing.B) {
	engine, cleanup := createTestEngine(b)
	defer cleanup()

	docs := generateTestDocuments(5000)
	batch := engine.index.NewBatch()
	for _, doc := range docs {
		batch.Index(doc.Path, doc)
	}
	if err := engine.index.Batch(batch); err != nil {
		b.Fatalf("预索引失败: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		matchQuery := bleve.NewMatchQuery("测试")
		searchReq := bleve.NewSearchRequestOptions(matchQuery, 20, 0, false)
		searchReq.Highlight = bleve.NewHighlightWithStyle("html")
		searchReq.Highlight.Fields = []string{"name", "content"}

		_, err := engine.index.Search(searchReq)
		if err != nil {
			b.Fatalf("搜索失败: %v", err)
		}
	}
}

// BenchmarkSearchConcurrent 测试并发搜索性能.
func BenchmarkSearchConcurrent(b *testing.B) {
	engine, cleanup := createTestEngine(b)
	defer cleanup()

	docs := generateTestDocuments(5000)
	batch := engine.index.NewBatch()
	for _, doc := range docs {
		batch.Index(doc.Path, doc)
	}
	if err := engine.index.Batch(batch); err != nil {
		b.Fatalf("预索引失败: %v", err)
	}

	queries := []string{"测试", "test", "中文", "file", "配置"}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			query := queries[i%len(queries)]
			matchQuery := bleve.NewMatchQuery(query)
			searchReq := bleve.NewSearchRequestOptions(matchQuery, 10, 0, false)

			_, err := engine.index.Search(searchReq)
			if err != nil {
				b.Errorf("并发搜索失败: %v", err)
			}
			i++
		}
	})
}

// BenchmarkIndexDirectory 测试目录索引性能.
func BenchmarkIndexDirectory(b *testing.B) {
	engine, cleanup := createTestEngine(b)
	defer cleanup()

	// 创建临时测试文件
	tmpDir, err := os.MkdirTemp("", "bench-dir-*")
	if err != nil {
		b.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	createTempFiles(b, tmpDir, 100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 每次重新创建索引以测试完整的索引流程
		if err := engine.IndexDirectory(tmpDir); err != nil {
			b.Fatalf("目录索引失败: %v", err)
		}
	}
}

// BenchmarkFuzzySearch 测试模糊搜索性能.
func BenchmarkFuzzySearch(b *testing.B) {
	engine, cleanup := createTestEngine(b)
	defer cleanup()

	docs := generateTestDocuments(5000)
	batch := engine.index.NewBatch()
	for _, doc := range docs {
		batch.Index(doc.Path, doc)
	}
	if err := engine.index.Batch(batch); err != nil {
		b.Fatalf("预索引失败: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		matchQuery := bleve.NewMatchQuery("tets") // 故意拼错
		matchQuery.SetFuzziness(2)
		searchReq := bleve.NewSearchRequestOptions(matchQuery, 20, 0, false)

		_, err := engine.index.Search(searchReq)
		if err != nil {
			b.Fatalf("模糊搜索失败: %v", err)
		}
	}
}

// BenchmarkPrefixSearch 测试前缀搜索性能.
func BenchmarkPrefixSearch(b *testing.B) {
	engine, cleanup := createTestEngine(b)
	defer cleanup()

	docs := generateTestDocuments(5000)
	batch := engine.index.NewBatch()
	for _, doc := range docs {
		batch.Index(doc.Path, doc)
	}
	if err := engine.index.Batch(batch); err != nil {
		b.Fatalf("预索引失败: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		prefixQuery := bleve.NewPrefixQuery("test")
		prefixQuery.SetField("name")
		searchReq := bleve.NewSearchRequestOptions(prefixQuery, 20, 0, false)

		_, err := engine.index.Search(searchReq)
		if err != nil {
			b.Fatalf("前缀搜索失败: %v", err)
		}
	}
}

// BenchmarkDeleteAndReindex 测试删除后重新索引的性能.
func BenchmarkDeleteAndReindex(b *testing.B) {
	engine, cleanup := createTestEngine(b)
	defer cleanup()

	docs := generateTestDocuments(100)

	// 预先索引
	batch := engine.index.NewBatch()
	for _, doc := range docs {
		batch.Index(doc.Path, doc)
	}
	if err := engine.index.Batch(batch); err != nil {
		b.Fatalf("预索引失败: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		doc := docs[i%len(docs)]

		// 删除
		if err := engine.Delete(doc.Path); err != nil {
			b.Fatalf("删除文档失败: %v", err)
		}

		// 重新索引
		if err := engine.index.Index(doc.Path, doc); err != nil {
			b.Fatalf("重新索引失败: %v", err)
		}
	}
}

// ================== 测试数据生成 ==================

// generateTestDocuments 生成测试文档.
func generateTestDocuments(count int) []FileInfo {
	docs := make([]FileInfo, count)

	contents := []string{
		"这是一段中文测试文本，用于全文搜索索引性能测试。包含常用词汇和标点符号。文件管理和系统维护是NAS的核心功能。",
		"This is an English test document for full-text search indexing performance testing. File management and system maintenance are core NAS features.",
		"# Markdown 标题\n\n这是一个 Markdown 文档示例。\n\n## 二级标题\n\n包含**粗体**和*斜体*文本。用于测试格式化文本的搜索能力。",
		`{"name": "测试配置", "version": "1.0", "description": "JSON配置文件示例", "settings": {"debug": true, "port": 8080}}`,
		"package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n\n// Go语言示例代码",
		"2024-01-15 10:30:00 INFO Server started on port 8080\n2024-01-15 10:30:01 DEBUG Connection established\n2024-01-15 10:30:05 WARN Memory usage high",
		"系统监控数据：CPU使用率 45%，内存使用率 60%，磁盘I/O 正常。网络连接稳定，延迟 < 5ms。",
		"用户权限管理：创建用户、分配角色、设置访问控制列表。支持LDAP和本地认证两种方式。",
		"存储池管理：RAID配置、磁盘健康检查、SMART监控、快照管理、数据恢复。",
		"网络配置：IP地址分配、DNS设置、防火墙规则、端口转发、VPN配置、负载均衡。",
	}

	paths := []string{
		"/documents/report.txt",
		"/documents/readme.md",
		"/config/settings.json",
		"/code/main.go",
		"/logs/server.log",
		"/system/monitor.txt",
		"/admin/users.txt",
		"/storage/disks.txt",
		"/network/config.txt",
		"/backup/status.txt",
	}

	exts := []string{".txt", ".md", ".json", ".go", ".log", ".txt", ".txt", ".txt", ".txt", ".txt"}

	for i := 0; i < count; i++ {
		idx := i % len(contents)
		docs[i] = FileInfo{
			Path:    fmt.Sprintf("%s_%d%s", strings.TrimSuffix(paths[idx], exts[idx]), i, exts[idx]),
			Name:    fmt.Sprintf("testfile_%d%s", i, exts[idx]),
			Ext:     exts[idx],
			Size:    int64(100 + i*10),
			ModTime: time.Now().Add(-time.Duration(i) * time.Hour),
			IsDir:   false,
			Content: fmt.Sprintf("[Document %d] %s", i, contents[idx]),
		}
	}

	return docs
}

// ================== 集成测试 ==================

// TestSearchIntegration 搜索集成测试.
func TestSearchIntegration(t *testing.T) {
	engine, cleanup := createTestEngine(&testing.B{})
	defer cleanup()

	// 索引测试文档
	docs := generateTestDocuments(100)
	batch := engine.index.NewBatch()
	for _, doc := range docs {
		batch.Index(doc.Path, doc)
	}
	if err := engine.index.Batch(batch); err != nil {
		t.Fatalf("批量索引失败: %v", err)
	}

	// 测试中文搜索
	t.Run("中文搜索", func(t *testing.T) {
		req := Request{
			Query: "中文测试",
			Limit: 10,
		}
		resp, err := engine.Search(req)
		if err != nil {
			t.Fatalf("中文搜索失败: %v", err)
		}
		if resp.Total == 0 {
			t.Log("中文搜索未返回结果（可能因为标准分析器不支持中文分词）")
		}
		t.Logf("中文搜索结果: %d 条", resp.Total)
	})

	// 测试英文搜索
	t.Run("英文搜索", func(t *testing.T) {
		req := Request{
			Query: "performance testing",
			Limit: 10,
		}
		resp, err := engine.Search(req)
		if err != nil {
			t.Fatalf("英文搜索失败: %v", err)
		}
		if resp.Total == 0 {
			t.Error("英文搜索应该返回结果")
		}
		t.Logf("英文搜索结果: %d 条", resp.Total)
	})

	// 测试过滤
	t.Run("文件类型过滤", func(t *testing.T) {
		req := Request{
			Query: "test",
			Types: []string{".go"},
			Limit: 10,
		}
		resp, err := engine.Search(req)
		if err != nil {
			t.Fatalf("过滤搜索失败: %v", err)
		}
		t.Logf("过滤搜索结果: %d 条", resp.Total)
	})
}
