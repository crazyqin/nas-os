package semanticsearch

import (
	"testing"
)

func TestNewSemanticEngine(t *testing.T) {
	engine := NewSemanticEngine(nil)
	if engine == nil {
		t.Fatal("NewSemanticEngine returned nil")
	}
}

func TestIndexDocument(t *testing.T) {
	engine := NewSemanticEngine(nil)

	t.Run("index valid document", func(t *testing.T) {
		doc := &Document{
			ID:      "doc1",
			Title:   "Test Document",
			Content: "This is a test document about storage management",
			Type:    "text",
		}
		if err := engine.IndexDocument(doc); err != nil {
			t.Fatalf("IndexDocument failed: %v", err)
		}
	})

	t.Run("index nil document", func(t *testing.T) {
		if err := engine.IndexDocument(nil); err == nil {
			t.Error("Expected error for nil document")
		}
	})

	t.Run("index document with empty ID", func(t *testing.T) {
		doc := &Document{Title: "No ID"}
		if err := engine.IndexDocument(doc); err == nil {
			t.Error("Expected error for empty ID")
		}
	})
}

func TestSearch(t *testing.T) {
	engine := NewSemanticEngine(nil)

	// 索引测试文档
	docs := []*Document{
		{ID: "doc1", Title: "Storage Guide", Content: "How to manage storage on NAS devices", Type: "guide"},
		{ID: "doc2", Title: "Network Setup", Content: "Configure network settings for optimal performance", Type: "guide"},
		{ID: "doc3", Title: "Backup Strategy", Content: "Best practices for data backup and recovery", Type: "guide"},
	}

	for _, doc := range docs {
		engine.IndexDocument(doc)
	}

	t.Run("search with results", func(t *testing.T) {
		query := &SearchQuery{
			Text:  "storage management",
			Limit: 10,
		}
		results, err := engine.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected search results")
		}
	})

	t.Run("search with nil query", func(t *testing.T) {
		_, err := engine.Search(nil)
		if err == nil {
			t.Error("Expected error for nil query")
		}
	})

	t.Run("search with empty text", func(t *testing.T) {
		query := &SearchQuery{Text: ""}
		_, err := engine.Search(query)
		if err == nil {
			t.Error("Expected error for empty text")
		}
	})

	t.Run("search with limit", func(t *testing.T) {
		query := &SearchQuery{
			Text:  "storage",
			Limit: 1,
		}
		results, err := engine.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(results) > 1 {
			t.Errorf("Expected at most 1 result, got %d", len(results))
		}
	})
}

func TestRemoveDocument(t *testing.T) {
	engine := NewSemanticEngine(nil)

	doc := &Document{ID: "doc1", Title: "Test", Content: "Test content"}
	engine.IndexDocument(doc)

	t.Run("remove existing document", func(t *testing.T) {
		if err := engine.RemoveDocument("doc1"); err != nil {
			t.Fatalf("RemoveDocument failed: %v", err)
		}
	})

	t.Run("remove non-existent document", func(t *testing.T) {
		if err := engine.RemoveDocument("non-existent"); err == nil {
			t.Error("Expected error for non-existent document")
		}
	})
}

func TestGetDocument(t *testing.T) {
	engine := NewSemanticEngine(nil)

	doc := &Document{ID: "doc1", Title: "Test", Content: "Content"}
	engine.IndexDocument(doc)

	t.Run("get existing document", func(t *testing.T) {
		result, err := engine.GetDocument("doc1")
		if err != nil {
			t.Fatalf("GetDocument failed: %v", err)
		}
		if result.Title != "Test" {
			t.Errorf("Expected title 'Test', got '%s'", result.Title)
		}
	})

	t.Run("get non-existent document", func(t *testing.T) {
		_, err := engine.GetDocument("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent document")
		}
	})
}

func TestGetMetrics(t *testing.T) {
	engine := NewSemanticEngine(nil)

	engine.IndexDocument(&Document{ID: "doc1", Title: "Test", Content: "Content"})

	metrics := engine.GetMetrics()
	if metrics == nil {
		t.Fatal("GetMetrics returned nil")
	}
	if metrics.TotalDocuments != 1 {
		t.Errorf("Expected 1 document, got %d", metrics.TotalDocuments)
	}
}
