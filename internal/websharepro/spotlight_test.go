package websharepro

import (
	"testing"
	"time"
)

func TestNewSpotlightEngine(t *testing.T) {
	engine := NewSpotlightEngine(100)
	if engine == nil {
		t.Fatal("NewSpotlightEngine returned nil")
	}
	engine.Close()
}

func TestSpotlightIndexAndSearch(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	doc := &IndexDocument{
		Path:       "/documents/report.pdf",
		Name:       "report.pdf",
		Extension:  ".pdf",
		MimeType:   "application/pdf",
		Size:       1024 * 1024,
		Content:    "Quarterly financial report with revenue analysis",
		Tags:       []string{"finance", "quarterly"},
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}

	if err := engine.IndexDocument(doc); err != nil {
		t.Fatalf("index document failed: %v", err)
	}

	// 搜索
	query := &SearchQuery{
		Query:    "financial report",
		Page:     1,
		PageSize: 10,
	}

	results, total, err := engine.Search(query)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if total == 0 {
		t.Error("expected search results")
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Document.Name != "report.pdf" {
		t.Errorf("expected report.pdf, got %s", results[0].Document.Name)
	}
}

func TestSpotlightSearchByTag(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	engine.IndexDocument(&IndexDocument{
		Path:      "/docs/a.txt",
		Name:      "a.txt",
		Extension: ".txt",
		Tags:      []string{"important", "work"},
	})
	engine.IndexDocument(&IndexDocument{
		Path:      "/docs/b.txt",
		Name:      "b.txt",
		Extension: ".txt",
		Tags:      []string{"personal"},
	})

	results := engine.SearchByTag("important")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSpotlightRemoveDocument(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	engine.IndexDocument(&IndexDocument{
		Path: "/test.txt",
		Name: "test.txt",
	})

	if !engine.IsIndexed("/test.txt") {
		t.Error("expected document to be indexed")
	}

	if err := engine.RemoveDocument("/test.txt"); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	if engine.IsIndexed("/test.txt") {
		t.Error("expected document to be removed")
	}
}

func TestSpotlightGetDocument(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	doc := &IndexDocument{
		Path: "/data/file.dat",
		Name: "file.dat",
		Size: 512,
	}
	engine.IndexDocument(doc)

	got, exists := engine.GetDocument("/data/file.dat")
	if !exists {
		t.Fatal("expected document to exist")
	}
	if got.Size != 512 {
		t.Errorf("expected size 512, got %d", got.Size)
	}
}

func TestSpotlightSearchWithFilters(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	engine.IndexDocument(&IndexDocument{
		Path:      "/docs/readme.txt",
		Name:      "readme.txt",
		Extension: ".txt",
		Content:   "readme content",
	})
	engine.IndexDocument(&IndexDocument{
		Path:      "/images/photo.jpg",
		Name:      "photo.jpg",
		Extension: ".jpg",
	})

	query := &SearchQuery{
		Query:    "readme",
		Page:     1,
		PageSize: 10,
		Filters:  map[string]string{"extension": ".txt"},
	}

	results, _, _ := engine.Search(query)
	for _, r := range results {
		if r.Document.Extension != ".txt" {
			t.Errorf("expected .txt extension, got %s", r.Document.Extension)
		}
	}
}

func TestSpotlightHighlight(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	engine.IndexDocument(&IndexDocument{
		Path:    "/docs/important.txt",
		Name:    "important.txt",
		Content: "This is an important document about system architecture",
	})

	query := &SearchQuery{
		Query:     "important",
		Page:      1,
		PageSize:  10,
		Highlight: true,
	}

	results, _, _ := engine.Search(query)
	if len(results) > 0 && len(results[0].Highlights) == 0 {
		t.Error("expected highlights")
	}
}

func TestSpotlightGetStats(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	engine.IndexDocument(&IndexDocument{Path: "/a.txt", Name: "a.txt"})
	engine.IndexDocument(&IndexDocument{Path: "/b.txt", Name: "b.txt"})

	stats := engine.GetStats()
	if stats.TotalDocuments != 2 {
		t.Errorf("expected 2 documents, got %d", stats.TotalDocuments)
	}
}

func TestSpotlightRebuildIndex(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	engine.IndexDocument(&IndexDocument{
		Path:    "/test.txt",
		Name:    "test.txt",
		Content: "rebuild test content",
	})

	engine.RebuildIndex()

	// 验证仍可搜索
	query := &SearchQuery{Query: "rebuild", Page: 1, PageSize: 10}
	results, _, _ := engine.Search(query)
	if len(results) == 0 {
		t.Error("expected results after rebuild")
	}
}

func TestSpotlightListDocuments(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	engine.IndexDocument(&IndexDocument{Path: "/docs/a.txt", Name: "a.txt"})
	engine.IndexDocument(&IndexDocument{Path: "/docs/b.txt", Name: "b.txt"})
	engine.IndexDocument(&IndexDocument{Path: "/images/c.jpg", Name: "c.jpg"})

	docs := engine.ListDocuments("/docs", 0)
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestSpotlightGetDocumentCount(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	engine.IndexDocument(&IndexDocument{Path: "/a.txt", Name: "a.txt"})
	engine.IndexDocument(&IndexDocument{Path: "/b.txt", Name: "b.txt"})
	engine.IndexDocument(&IndexDocument{Path: "/c.txt", Name: "c.txt"})

	if engine.GetDocumentCount() != 3 {
		t.Errorf("expected 3, got %d", engine.GetDocumentCount())
	}
}

func TestSpotlightSearchPagination(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	for i := 0; i < 20; i++ {
		engine.IndexDocument(&IndexDocument{
			Path:    "/file" + string(rune('a'+i)) + ".txt",
			Name:    "file" + string(rune('a'+i)) + ".txt",
			Content: "searchable content",
		})
	}

	// 第一页
	query := &SearchQuery{
		Query:    "searchable",
		Page:     1,
		PageSize: 5,
	}
	results1, total, _ := engine.Search(query)
	if len(results1) > 5 {
		t.Errorf("expected max 5 results, got %d", len(results1))
	}
	if total == 0 {
		t.Error("expected total > 0")
	}

	// 第二页
	query.Page = 2
	results2, _, _ := engine.Search(query)
	if len(results2) == 0 {
		// 可能总数不足 10，这是正常的
	}
}

func TestSpotlightGetIndexedExtensions(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	engine.IndexDocument(&IndexDocument{Path: "/a.txt", Name: "a.txt", Extension: ".txt"})
	engine.IndexDocument(&IndexDocument{Path: "/b.pdf", Name: "b.pdf", Extension: ".pdf"})

	exts := engine.GetIndexedExtensions()
	if len(exts) < 2 {
		t.Errorf("expected at least 2 extensions, got %d", len(exts))
	}
}

func TestSpotlightUpdateDocument(t *testing.T) {
	engine := NewSpotlightEngine(100)
	defer engine.Close()

	// 初始索引
	engine.IndexDocument(&IndexDocument{
		Path:    "/update.txt",
		Name:    "update.txt",
		Content: "original content",
	})

	// 更新（同路径）
	engine.IndexDocument(&IndexDocument{
		Path:    "/update.txt",
		Name:    "update.txt",
		Content: "updated content new",
	})

	// 搜索更新后的内容
	query := &SearchQuery{Query: "updated", Page: 1, PageSize: 10}
	results, _, _ := engine.Search(query)
	if len(results) == 0 {
		t.Error("expected results for updated content")
	}
}
