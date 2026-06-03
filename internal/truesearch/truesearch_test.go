package truesearch

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func createTestDocument(id, path, name, content string) *Document {
	return &Document{
		ID:        id,
		Path:      path,
		Name:      name,
		Extension: GetFileExtension(name),
		Size:      int64(len(content)),
		FileType:  ClassifyFileType(GetFileExtension(name)),
		Content:   content,
		ModTime:   time.Now(),
		IndexTime: time.Now(),
	}
}

func TestManagerStartStop(t *testing.T) {
	m := setupTestManager(t)

	if m.IsRunning() {
		t.Error("expected manager to not be running initially")
	}

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !m.IsRunning() {
		t.Error("expected manager to be running after Start")
	}

	// 启动两次应该报错
	if err := m.Start(ctx); err == nil {
		t.Error("expected error when starting twice")
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if m.IsRunning() {
		t.Error("expected manager to not be running after Stop")
	}
}

func TestManagerDisabled(t *testing.T) {
	cfg := DefaultTrueSearchConfig()
	cfg.Enabled = false
	m := NewManager(zap.NewNop(), cfg)

	ctx := context.Background()
	if err := m.Start(ctx); err == nil {
		t.Error("expected error when disabled")
	}
}

func TestIndexDocument(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	doc := createTestDocument("doc1", "/test/file1.txt", "file1.txt", "Hello World Test Content")

	if err := m.IndexDocument(doc); err != nil {
		t.Fatalf("IndexDocument failed: %v", err)
	}

	// 验证文档已索引
	got, err := m.GetDocument("doc1")
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if got.Path != "/test/file1.txt" {
		t.Errorf("expected path '/test/file1.txt', got '%s'", got.Path)
	}
	if got.Name != "file1.txt" {
		t.Errorf("expected name 'file1.txt', got '%s'", got.Name)
	}
}

func TestIndexDocumentWhenNotRunning(t *testing.T) {
	m := setupTestManager(t)

	doc := createTestDocument("doc1", "/test/file1.txt", "file1.txt", "content")

	if err := m.IndexDocument(doc); err == nil {
		t.Error("expected error when not running")
	}
}

func TestIncrementalUpdate(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	// 第一次索引
	doc := createTestDocument("doc1", "/test/file1.txt", "file1.txt", "initial content")
	doc.ModTime = time.Now().Add(-time.Hour)
	if err := m.IndexDocument(doc); err != nil {
		t.Fatalf("first IndexDocument failed: %v", err)
	}

	stats1 := m.GetStats()
	if stats1.TotalDocuments != 1 {
		t.Errorf("expected 1 document, got %d", stats1.TotalDocuments)
	}

	// 相同时间，应该跳过
	doc2 := createTestDocument("doc1", "/test/file1.txt", "file1.txt", "same content")
	doc2.ModTime = doc.ModTime
	if err := m.IndexDocument(doc2); err != nil {
		t.Fatalf("second IndexDocument failed: %v", err)
	}

	// 更新后的内容
	doc3 := createTestDocument("doc1", "/test/file1.txt", "file1.txt", "updated content")
	doc3.ModTime = time.Now()
	if err := m.IndexDocument(doc3); err != nil {
		t.Fatalf("third IndexDocument failed: %v", err)
	}

	got, _ := m.GetDocument("doc1")
	if got.Content != "updated content" {
		t.Errorf("expected content 'updated content', got '%s'", got.Content)
	}
}

func TestRemoveDocument(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	doc := createTestDocument("doc1", "/test/file1.txt", "file1.txt", "content to remove")
	m.IndexDocument(doc)

	if err := m.RemoveDocument("doc1"); err != nil {
		t.Fatalf("RemoveDocument failed: %v", err)
	}

	_, err := m.GetDocument("doc1")
	if err == nil {
		t.Error("expected error for removed document")
	}

	// 移除不存在的文档应该报错
	if err := m.RemoveDocument("nonexistent"); err == nil {
		t.Error("expected error when removing nonexistent document")
	}
}

func TestBatchIndex(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	docs := []*Document{
		createTestDocument("doc1", "/test/file1.txt", "file1.txt", "content one"),
		createTestDocument("doc2", "/test/file2.txt", "file2.txt", "content two"),
		createTestDocument("doc3", "/test/file3.txt", "file3.txt", "content three"),
	}

	indexed, err := m.IndexBatch(docs)
	if err != nil {
		t.Fatalf("IndexBatch failed: %v", err)
	}
	if indexed != 3 {
		t.Errorf("expected 3 indexed, got %d", indexed)
	}

	stats := m.GetStats()
	if stats.TotalDocuments != 3 {
		t.Errorf("expected 3 documents, got %d", stats.TotalDocuments)
	}
}

func TestSearchFilename(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	docs := []*Document{
		createTestDocument("doc1", "/photos/vacation.jpg", "vacation.jpg", "beach sunset photo"),
		createTestDocument("doc2", "/docs/report.pdf", "report.pdf", "quarterly financial report"),
		createTestDocument("doc3", "/photos/vacation2.jpg", "vacation2.jpg", "mountain hiking photo"),
	}
	m.IndexBatch(docs)

	// 搜索文件名
	resp, err := m.SearchFilename("vacation", 10)
	if err != nil {
		t.Fatalf("SearchFilename failed: %v", err)
	}
	if resp.TotalHits != 2 {
		t.Errorf("expected 2 hits, got %d", resp.TotalHits)
	}

	for _, r := range resp.Results {
		if !r.FilenameMatch {
			t.Error("expected filename match")
		}
	}
}

func TestSearchContent(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	docs := []*Document{
		createTestDocument("doc1", "/test/file1.txt", "file1.txt", "the quick brown fox jumps over the lazy dog"),
		createTestDocument("doc2", "/test/file2.txt", "file2.txt", "a lazy cat sleeps all day"),
		createTestDocument("doc3", "/test/file3.txt", "file3.txt", "the fox is clever and quick"),
	}
	m.IndexBatch(docs)

	resp, err := m.SearchContent("lazy", 10)
	if err != nil {
		t.Fatalf("SearchContent failed: %v", err)
	}
	if resp.TotalHits != 2 {
		t.Errorf("expected 2 hits for 'lazy', got %d", resp.TotalHits)
	}
}

func TestSearchAll(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	docs := []*Document{
		createTestDocument("doc1", "/test/report.txt", "report.txt", "annual financial report"),
		createTestDocument("doc2", "/test/photo.jpg", "report_photo.jpg", "photo of the building"),
		createTestDocument("doc3", "/test/data.csv", "data.csv", "sales report data"),
	}
	m.IndexBatch(docs)

	// 搜索 "report" 应该匹配文件名和内容
	resp, err := m.Search(&SearchQuery{
		Query: "report",
		Mode:  SearchModeAll,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if resp.TotalHits < 2 {
		t.Errorf("expected at least 2 hits, got %d", resp.TotalHits)
	}
}

func TestSearchWithFilters(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	now := time.Now()
	docs := []*Document{
		{
			ID: "doc1", Path: "/test/old.txt", Name: "old.txt",
			Extension: ".txt", Size: 100, FileType: FileTypeDocument,
			Content: "old document content", ModTime: now.Add(-48 * time.Hour),
		},
		{
			ID: "doc2", Path: "/test/new.txt", Name: "new.txt",
			Extension: ".txt", Size: 200, FileType: FileTypeDocument,
			Content: "new document content", ModTime: now,
		},
		{
			ID: "doc3", Path: "/test/image.jpg", Name: "image.jpg",
			Extension: ".jpg", Size: 50000, FileType: FileTypeImage,
			Content: "image description", ModTime: now,
		},
	}

	for _, doc := range docs {
		m.IndexDocument(doc)
	}

	// 文件类型过滤
	resp, err := m.Search(&SearchQuery{
		Query:     "document",
		FileTypes: []FileType{FileTypeDocument},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Search with file type filter failed: %v", err)
	}
	for _, r := range resp.Results {
		if r.FileType != FileTypeDocument {
			t.Errorf("expected document type, got %s", r.FileType)
		}
	}

	// 时间过滤
	after := now.Add(-24 * time.Hour)
	resp, err = m.Search(&SearchQuery{
		Query: "content",
		After: &after,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search with time filter failed: %v", err)
	}
	for _, r := range resp.Results {
		if r.ModTime.Before(after) {
			t.Error("expected result after specified time")
		}
	}

	// 大小过滤
	minSize := int64(1000)
	resp, err = m.Search(&SearchQuery{
		Query:   "content",
		MinSize: &minSize,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Search with size filter failed: %v", err)
	}
	for _, r := range resp.Results {
		if r.Size < minSize {
			t.Errorf("expected size >= %d, got %d", minSize, r.Size)
		}
	}
}

func TestSearchPagination(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	docs := make([]*Document, 20)
	for i := 0; i < 20; i++ {
		docs[i] = createTestDocument(
			fmt.Sprintf("doc%d", i),
			fmt.Sprintf("/test/file%d.txt", i),
			fmt.Sprintf("file%d.txt", i),
			"content",
		)
	}
	m.IndexBatch(docs)

	// 第一页
	resp1, _ := m.Search(&SearchQuery{
		Query: "file",
		Mode:  SearchModeFilename,
		Limit: 5,
	})

	// 第二页
	resp2, _ := m.Search(&SearchQuery{
		Query: "file",
		Mode:  SearchModeFilename,
		Limit: 5,
		Offset: 5,
	})

	if resp1.TotalHits != 20 {
		t.Errorf("expected total 20, got %d", resp1.TotalHits)
	}
	if len(resp1.Results) != 5 {
		t.Errorf("expected 5 results on page 1, got %d", len(resp1.Results))
	}
	if len(resp2.Results) != 5 {
		t.Errorf("expected 5 results on page 2, got %d", len(resp2.Results))
	}

	// 确保结果不重复
	for _, r1 := range resp1.Results {
		for _, r2 := range resp2.Results {
			if r1.DocID == r2.DocID {
				t.Error("found duplicate result across pages")
			}
		}
	}
}

func TestSearchSorting(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	now := time.Now()
	docs := []*Document{
		{
			ID: "doc1", Path: "/test/a.txt", Name: "a.txt",
			Size: 100, ModTime: now.Add(-3 * time.Hour),
			Content: "test search", FileType: FileTypeDocument,
		},
		{
			ID: "doc2", Path: "/test/b.txt", Name: "b.txt",
			Size: 300, ModTime: now.Add(-1 * time.Hour),
			Content: "test search", FileType: FileTypeDocument,
		},
		{
			ID: "doc3", Path: "/test/c.txt", Name: "c.txt",
			Size: 200, ModTime: now,
			Content: "test search", FileType: FileTypeDocument,
		},
	}
	for _, doc := range docs {
		m.IndexDocument(doc)
	}

	// 按日期排序
	resp, _ := m.Search(&SearchQuery{
		Query: "test",
		Sort:  SortByDate,
		Limit: 10,
	})
	if len(resp.Results) >= 2 {
		if resp.Results[0].ModTime.Before(resp.Results[1].ModTime) {
			t.Error("expected results sorted by date descending")
		}
	}

	// 按大小排序
	resp, _ = m.Search(&SearchQuery{
		Query: "test",
		Sort:  SortBySize,
		Limit: 10,
	})
	if len(resp.Results) >= 2 {
		if resp.Results[0].Size < resp.Results[1].Size {
			t.Error("expected results sorted by size descending")
		}
	}

	// 按名称排序
	resp, _ = m.Search(&SearchQuery{
		Query: "test",
		Sort:  SortByName,
		Limit: 10,
	})
	if len(resp.Results) >= 2 {
		if resp.Results[0].Name > resp.Results[1].Name {
			t.Error("expected results sorted by name ascending")
		}
	}
}

func TestSearchHighlight(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	doc := createTestDocument("doc1", "/test/test.txt", "test.txt", "this is a test document with test content")
	m.IndexDocument(doc)

	resp, _ := m.Search(&SearchQuery{
		Query: "test",
		Limit: 10,
	})

	if len(resp.Results) == 0 {
		t.Fatal("expected results")
	}

	r := resp.Results[0]
	if r.HighlightName == "" {
		t.Error("expected highlighted filename")
	}
	if r.FilenameMatch && r.HighlightSnip == "" {
		// 如果有内容匹配，应该有高亮摘要
		// 注意：这里可能没有，因为搜索词在文件名中
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	_, err := m.Search(&SearchQuery{
		Query: "",
		Limit: 10,
	})
	if err == nil {
		t.Error("expected error for empty query")
	}

	_, err = m.Search(nil)
	if err == nil {
		t.Error("expected error for nil query")
	}
}

func TestAutoComplete(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	docs := []*Document{
		createTestDocument("doc1", "/test/report.pdf", "report.pdf", "content"),
		createTestDocument("doc2", "/test/report2.pdf", "report2.pdf", "content"),
		createTestDocument("doc3", "/test/readme.txt", "readme.txt", "content"),
	}
	m.IndexBatch(docs)

	suggestions := m.AutoComplete("rep", 10)
	if len(suggestions) < 2 {
		t.Errorf("expected at least 2 suggestions, got %d", len(suggestions))
	}

	for _, s := range suggestions {
		if s != "report.pdf" && s != "report2.pdf" {
			t.Errorf("unexpected suggestion: %s", s)
		}
	}
}

func TestGetStats(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	docs := []*Document{
		createTestDocument("doc1", "/test/file1.txt", "file1.txt", "hello world"),
		createTestDocument("doc2", "/test/file2.txt", "file2.txt", "world peace"),
	}
	m.IndexBatch(docs)

	stats := m.GetStats()
	if stats.TotalDocuments != 2 {
		t.Errorf("expected 2 documents, got %d", stats.TotalDocuments)
	}
	if stats.Status != IndexStatusReady {
		t.Errorf("expected status ready, got %s", stats.Status)
	}
}

func TestGetConfig(t *testing.T) {
	m := setupTestManager(t)

	cfg := m.GetConfig()
	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
	if cfg.MaxResults != 100 {
		t.Errorf("expected max results 100, got %d", cfg.MaxResults)
	}
	if cfg.HighlightTag != "em" {
		t.Errorf("expected highlight tag 'em', got '%s'", cfg.HighlightTag)
	}
}

func TestUpdateConfig(t *testing.T) {
	m := setupTestManager(t)

	newCfg := DefaultTrueSearchConfig()
	newCfg.MaxResults = 200
	newCfg.SnippetLen = 300

	if err := m.UpdateConfig(newCfg); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	cfg := m.GetConfig()
	if cfg.MaxResults != 200 {
		t.Errorf("expected max results 200, got %d", cfg.MaxResults)
	}
	if cfg.SnippetLen != 300 {
		t.Errorf("expected snippet len 300, got %d", cfg.SnippetLen)
	}

	// nil config should error
	if err := m.UpdateConfig(nil); err == nil {
		t.Error("expected error for nil config")
	}
}

func TestRebuildIndex(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	// 初始文档
	docs1 := []*Document{
		createTestDocument("doc1", "/test/file1.txt", "file1.txt", "original content"),
	}
	m.IndexBatch(docs1)

	// 重建索引
	docs2 := []*Document{
		createTestDocument("doc2", "/test/file2.txt", "file2.txt", "new content"),
		createTestDocument("doc3", "/test/file3.txt", "file3.txt", "another content"),
	}

	if err := m.RebuildIndex(ctx, docs2); err != nil {
		t.Fatalf("RebuildIndex failed: %v", err)
	}

	stats := m.GetStats()
	if stats.TotalDocuments != 2 {
		t.Errorf("expected 2 documents after rebuild, got %d", stats.TotalDocuments)
	}

	// 旧文档应该不存在
	_, err := m.GetDocument("doc1")
	if err == nil {
		t.Error("expected old document to be removed after rebuild")
	}

	// 新文档应该存在
	_, err = m.GetDocument("doc2")
	if err != nil {
		t.Error("expected new document to exist after rebuild")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10

	// 并发索引
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			doc := createTestDocument(
				fmt.Sprintf("doc%d", id),
				fmt.Sprintf("/test/file%d.txt", id),
				fmt.Sprintf("file%d.txt", id),
				fmt.Sprintf("content for document %d", id),
			)
			m.IndexDocument(doc)
		}(i)
	}
	wg.Wait()

	// 并发搜索
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Search(&SearchQuery{
				Query: "content",
				Limit: 10,
			})
		}()
	}
	wg.Wait()

	stats := m.GetStats()
	if stats.TotalDocuments != int64(numGoroutines) {
		t.Errorf("expected %d documents, got %d", numGoroutines, stats.TotalDocuments)
	}
}

func TestClassifyFileType(t *testing.T) {
	tests := []struct {
		ext  string
		want FileType
	}{
		{".txt", FileTypeDocument},
		{".pdf", FileTypeDocument},
		{".jpg", FileTypeImage},
		{".png", FileTypeImage},
		{".mp4", FileTypeVideo},
		{".mp3", FileTypeAudio},
		{".go", FileTypeCode},
		{".zip", FileTypeArchive},
		{".xyz", FileTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := ClassifyFileType(tt.ext)
			if got != tt.want {
				t.Errorf("ClassifyFileType(%s) = %s, want %s", tt.ext, got, tt.want)
			}
		})
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/test/file.txt", ".txt"},
		{"/test/file.tar.gz", ".gz"},
		{"/test/file", ""},
		{"/test/.hidden", ".hidden"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := GetFileExtension(tt.path)
			if got != tt.want {
				t.Errorf("GetFileExtension(%s) = %s, want %s", tt.path, got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultTrueSearchConfig()

	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
	if cfg.MaxContentLen != 1024*1024 {
		t.Errorf("expected max content len 1048576, got %d", cfg.MaxContentLen)
	}
	if cfg.MinTermLen != 2 {
		t.Errorf("expected min term len 2, got %d", cfg.MinTermLen)
	}
	if cfg.MaxResults != 100 {
		t.Errorf("expected max results 100, got %d", cfg.MaxResults)
	}
	if cfg.HighlightTag != "em" {
		t.Errorf("expected highlight tag 'em', got '%s'", cfg.HighlightTag)
	}
	if cfg.Workers != 4 {
		t.Errorf("expected workers 4, got %d", cfg.Workers)
	}
}

func TestListDocuments(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()

	docs := []*Document{
		createTestDocument("doc1", "/test/file1.txt", "file1.txt", "content1"),
		createTestDocument("doc2", "/test/file2.txt", "file2.txt", "content2"),
		createTestDocument("doc3", "/test/file3.txt", "file3.txt", "content3"),
	}
	m.IndexBatch(docs)

	list := m.ListDocuments(0)
	if len(list) != 3 {
		t.Errorf("expected 3 documents, got %d", len(list))
	}

	list = m.ListDocuments(2)
	if len(list) != 2 {
		t.Errorf("expected 2 documents with limit, got %d", len(list))
	}
}
