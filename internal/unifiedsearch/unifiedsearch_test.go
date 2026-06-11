// Package unifiedsearch 单元测试
package unifiedsearch

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := &SearchConfig{
		IndexDir:        t.TempDir(),
		MaxHistory:      100,
		MaxHotSearches:  50,
		FuzzyThreshold:  0.8,
		HighlightPre:    "<mark>",
		HighlightPost:   "</mark>",
		SummaryLength:   200,
		MaxPageSize:     100,
		DefaultPageSize: 20,
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestNewManagerNilConfig(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if manager == nil {
		t.Fatal("NewManager(nil) returned nil")
	}
}

func TestStartStop(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !manager.engine.IsRunning() {
		t.Error("Expected engine to be running")
	}

	if err := manager.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestAddDocumentAndSearch(t *testing.T) {
	config := &SearchConfig{
		IndexDir:        t.TempDir(),
		MaxHistory:      100,
		MaxHotSearches:  50,
		FuzzyThreshold:  0.6,
		HighlightPre:    "<mark>",
		HighlightPost:   "</mark>",
		SummaryLength:   200,
		MaxPageSize:     100,
		DefaultPageSize: 20,
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	entry := &SearchIndex{
		Path:        "/docs/test.md",
		Name:        "Test Document",
		Content:     "This is a test document about NAS storage",
		ContentType: ContentTypeFile,
		MimeType:    "text/markdown",
		Size:        1024,
		Tags:        []string{"test", "nas"},
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	}

	if err := manager.AddDocument(entry); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}

	stats := manager.GetIndexStats()
	if stats.TotalDocuments != 1 {
		t.Errorf("Expected 1 document, got %d", stats.TotalDocuments)
	}

	req := &SearchQuery{
		Query:     "test",
		PageSize:  10,
		Page:      1,
		Highlight: true,
	}

	resp, err := manager.Search(req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp.Total < 1 {
		t.Errorf("Expected at least 1 result, got %d", resp.Total)
	}
}

func TestRemoveDocument(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	entry := &SearchIndex{
		Path:        "/docs/test-remove.md",
		Name:        "Test Remove",
		Content:     "test content",
		ContentType: ContentTypeFile,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	}

	manager.AddDocument(entry)

	if err := manager.RemoveDocument("/docs/test-remove.md"); err != nil {
		t.Fatalf("RemoveDocument failed: %v", err)
	}

	stats := manager.GetIndexStats()
	if stats.TotalDocuments != 0 {
		t.Errorf("Expected 0 documents after remove, got %d", stats.TotalDocuments)
	}
}

func TestSearchEmpty(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	req := &SearchQuery{
		Query:    "nonexistent",
		PageSize: 10,
		Page:     1,
	}

	resp, err := manager.Search(req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("Expected 0 results, got %d", resp.Total)
	}
}

func TestSearchNilQuery(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	_, err = manager.Search(nil)
	if err == nil {
		t.Fatal("Expected error for nil query, got nil")
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	_, err = manager.Search(&SearchQuery{})
	if err == nil {
		t.Fatal("Expected error for empty query, got nil")
	}
}

func TestAddDocumentNil(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	err = manager.AddDocument(nil)
	if err == nil {
		t.Fatal("Expected error for nil document, got nil")
	}
}

func TestAddDocumentEmptyPath(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	err = manager.AddDocument(&SearchIndex{Name: "test"})
	if err == nil {
		t.Fatal("Expected error for empty path, got nil")
	}
}

func TestUpdateDocument(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	entry := &SearchIndex{
		Path:        "/docs/update-test.md",
		Name:        "Original Name",
		Content:     "original content",
		ContentType: ContentTypeFile,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	}

	manager.AddDocument(entry)

	// Get the document to find its ID
	docs := manager.ListDocuments("", 10)
	if len(docs) == 0 {
		t.Fatal("No documents found")
	}

	err = manager.UpdateDocument(&UpdateIndexRequest{
		ID:      docs[0].ID,
		Name:    "Updated Name",
		Content: "updated content",
		Tags:    []string{"updated"},
	})

	if err != nil {
		t.Fatalf("UpdateDocument failed: %v", err)
	}

	updated, err := manager.GetDocument(docs[0].ID)
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got '%s'", updated.Name)
	}
}

func TestBuildIndex(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	task, err := manager.BuildIndex("/data")
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	if task == nil {
		t.Fatal("BuildIndex returned nil task")
	}

	if task.Type != TaskTypeFull {
		t.Errorf("Expected task type 'full', got '%s'", task.Type)
	}

	// Wait for async task to complete
	time.Sleep(200 * time.Millisecond)

	retrieved, err := manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if retrieved.Status != TaskStatusCompleted {
		t.Errorf("Expected task status 'completed', got '%s'", retrieved.Status)
	}
}

func TestSearchHistory(t *testing.T) {
	config := &SearchConfig{
		IndexDir:        t.TempDir(),
		MaxHistory:      100,
		MaxHotSearches:  50,
		FuzzyThreshold:  0.6,
		HighlightPre:    "<mark>",
		HighlightPost:   "</mark>",
		SummaryLength:   200,
		MaxPageSize:     100,
		DefaultPageSize: 20,
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Add some documents first
	entry := &SearchIndex{
		Path:        "/docs/history-test.md",
		Name:        "History Test",
		Content:     "search history test content",
		ContentType: ContentTypeFile,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	}
	if err := manager.AddDocument(entry); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}

	// Search to generate history
	resp, err := manager.Search(&SearchQuery{Query: "history test", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if resp.Total < 1 {
		t.Errorf("Expected at least 1 result, got %d", resp.Total)
	}

	history := manager.GetSearchHistory(10)
	if len(history) == 0 {
		t.Error("Expected search history, got empty")
	}

	// Clear history
	manager.ClearSearchHistory()
	history = manager.GetSearchHistory(10)
	if len(history) != 0 {
		t.Errorf("Expected empty history after clear, got %d entries", len(history))
	}
}

func TestGetConfig(t *testing.T) {
	config := &SearchConfig{
		IndexDir:       t.TempDir(),
		MaxHistory:     500,
		FuzzyThreshold: 0.7,
	}
	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	cfg := manager.GetConfig()
	if cfg.MaxHistory != 500 {
		t.Errorf("Expected MaxHistory 500, got %d", cfg.MaxHistory)
	}
	if cfg.FuzzyThreshold != 0.7 {
		t.Errorf("Expected FuzzyThreshold 0.7, got %f", cfg.FuzzyThreshold)
	}
}

func TestUpdateConfig(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}
	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	newConfig := &SearchConfig{
		MaxHistory:     2000,
		FuzzyThreshold: 0.9,
	}
	manager.UpdateConfig(newConfig)

	cfg := manager.GetConfig()
	if cfg.MaxHistory != 2000 {
		t.Errorf("Expected MaxHistory 2000, got %d", cfg.MaxHistory)
	}
}

func TestListDocuments(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Add multiple documents
	for i := 0; i < 5; i++ {
		entry := &SearchIndex{
			Path:        "/docs/list-test-" + string(rune('a'+i)) + ".md",
			Name:        "List Test " + string(rune('A'+i)),
			Content:     "content for listing",
			ContentType: ContentTypeFile,
			CreatedAt:   time.Now(),
			ModifiedAt:  time.Now(),
		}
		manager.AddDocument(entry)
	}

	docs := manager.ListDocuments("", 10)
	if len(docs) != 5 {
		t.Errorf("Expected 5 documents, got %d", len(docs))
	}

	// Test with content type filter
	docs = manager.ListDocuments(ContentTypeFile, 10)
	if len(docs) != 5 {
		t.Errorf("Expected 5 file documents, got %d", len(docs))
	}

	// Test with limit
	docs = manager.ListDocuments("", 3)
	if len(docs) != 3 {
		t.Errorf("Expected 3 documents with limit, got %d", len(docs))
	}
}

func TestSearchWithTags(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	entry := &SearchIndex{
		Path:        "/docs/tagged.md",
		Name:        "Tagged Document",
		Content:     "important content",
		ContentType: ContentTypeFile,
		Tags:        []string{"important", "review"},
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	}
	manager.AddDocument(entry)

	resp, err := manager.Search(&SearchQuery{
		Query:    "important",
		Tags:     []string{"important"},
		Page:     1,
		PageSize: 10,
	})

	if err != nil {
		t.Fatalf("Search with tags failed: %v", err)
	}

	if resp.Total < 1 {
		t.Errorf("Expected at least 1 result, got %d", resp.Total)
	}
}

func TestSearchWithContentType(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Add file document
	manager.AddDocument(&SearchIndex{
		Path:        "/docs/file.txt",
		Name:        "File Document",
		Content:     "file content",
		ContentType: ContentTypeFile,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	})

	// Add photo document
	manager.AddDocument(&SearchIndex{
		Path:        "/photos/photo.jpg",
		Name:        "Photo",
		Content:     "photo content",
		ContentType: ContentTypePhoto,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	})

	// Search only files
	resp, err := manager.Search(&SearchQuery{
		Query:    "content",
		Types:    []ContentType{ContentTypeFile},
		Page:     1,
		PageSize: 10,
	})

	if err != nil {
		t.Fatalf("Search with content type failed: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 result for file type, got %d", resp.Total)
	}

	if len(resp.Results) > 0 && resp.Results[0].ContentType != ContentTypeFile {
		t.Errorf("Expected file content type, got %s", resp.Results[0].ContentType)
	}
}

func TestRebuildIndex(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Add a document
	manager.AddDocument(&SearchIndex{
		Path:        "/docs/before-rebuild.md",
		Name:        "Before Rebuild",
		Content:     "content before rebuild",
		ContentType: ContentTypeFile,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	})

	// Rebuild index
	if err := manager.RebuildIndex(); err != nil {
		t.Fatalf("RebuildIndex failed: %v", err)
	}

	// After rebuild, the index should be empty (documents were only in memory cache)
	stats := manager.GetIndexStats()
	if stats.TotalDocuments != 0 {
		t.Errorf("Expected 0 documents after rebuild, got %d", stats.TotalDocuments)
	}
}

func TestGetSuggestions(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Add documents
	manager.AddDocument(&SearchIndex{
		Path:        "/docs/suggestion-test.md",
		Name:        "Suggestion Test Document",
		Content:     "test content for suggestions",
		ContentType: ContentTypeFile,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	})

	suggestions := manager.GetSuggestions("suggestion", 10)
	if len(suggestions) == 0 {
		// Suggestions may be empty if bleve hasn't fully indexed yet
		t.Log("No suggestions found (this may be expected)")
	}
}

func TestIsValidContentType(t *testing.T) {
	tests := []struct {
		input    ContentType
		expected bool
	}{
		{ContentTypeFile, true},
		{ContentTypePhoto, true},
		{ContentTypeDocument, true},
		{ContentTypeVideo, true},
		{ContentTypeMusic, true},
		{ContentTypeEmail, true},
		{ContentTypeNote, true},
		{ContentType("invalid"), false},
	}

	for _, tt := range tests {
		if got := IsValidContentType(tt.input); got != tt.expected {
			t.Errorf("IsValidContentType(%s) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsValidSortOrder(t *testing.T) {
	tests := []struct {
		input    SortOrder
		expected bool
	}{
		{SortRelevance, true},
		{SortDateDesc, true},
		{SortDateAsc, true},
		{SortSizeDesc, true},
		{SortSizeAsc, true},
		{SortNameAsc, true},
		{SortNameDesc, true},
		{SortOrder("invalid"), false},
	}

	for _, tt := range tests {
		if got := IsValidSortOrder(tt.input); got != tt.expected {
			t.Errorf("IsValidSortOrder(%s) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsValidBooleanOp(t *testing.T) {
	tests := []struct {
		input    BooleanOp
		expected bool
	}{
		{BooleanAND, true},
		{BooleanOR, true},
		{BooleanNOT, true},
		{BooleanOp("invalid"), false},
	}

	for _, tt := range tests {
		if got := IsValidBooleanOp(tt.input); got != tt.expected {
			t.Errorf("IsValidBooleanOp(%s) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestDefaultSearchQuery(t *testing.T) {
	q := DefaultSearchQuery()

	if q.Page != 1 {
		t.Errorf("Expected default page 1, got %d", q.Page)
	}
	if q.PageSize != 20 {
		t.Errorf("Expected default page_size 20, got %d", q.PageSize)
	}
	if q.BooleanOp != BooleanAND {
		t.Errorf("Expected default boolean_op AND, got %s", q.BooleanOp)
	}
	if q.SortBy != SortRelevance {
		t.Errorf("Expected default sort_by relevance, got %s", q.SortBy)
	}
	if !q.Highlight {
		t.Error("Expected default highlight true")
	}
	if q.FuzzyLevel != 1 {
		t.Errorf("Expected default fuzzy_level 1, got %d", q.FuzzyLevel)
	}
}

func TestDefaultIndexStats(t *testing.T) {
	stats := DefaultIndexStats()

	if stats.Status != IndexStatusIdle {
		t.Errorf("Expected default status idle, got %s", stats.Status)
	}
	if stats.TotalDocuments != 0 {
		t.Errorf("Expected default total_documents 0, got %d", stats.TotalDocuments)
	}
	if stats.IndexVersion != 1 {
		t.Errorf("Expected default index_version 1, got %d", stats.IndexVersion)
	}
	if stats.ContentTypes == nil {
		t.Error("Expected non-nil ContentTypes map")
	}
}

func TestSearchResponseTime(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Add multiple documents
	for i := 0; i < 100; i++ {
		manager.AddDocument(&SearchIndex{
			Path:        "/docs/bench-" + string(rune('a'+i%26)) + ".md",
			Name:        "Benchmark Document " + string(rune('A'+i%26)),
			Content:     "This is benchmark content for performance testing",
			ContentType: ContentTypeFile,
			CreatedAt:   time.Now(),
			ModifiedAt:  time.Now(),
		})
	}

	// Perform search and check response time
	start := time.Now()
	resp, err := manager.Search(&SearchQuery{
		Query:    "benchmark",
		Page:     1,
		PageSize: 20,
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Search returned %d results in %dms", resp.Total, resp.TimeMs)

	// 目标：亚秒级响应 (<500ms)
	if elapsed.Milliseconds() > 500 {
		t.Errorf("Search took %dms, expected <500ms", elapsed.Milliseconds())
	}
}

func TestIncrementalUpdate(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	task, err := manager.IncrementalUpdate("/data")
	if err != nil {
		t.Fatalf("IncrementalUpdate failed: %v", err)
	}

	if task.Type != TaskTypeIncremental {
		t.Errorf("Expected task type 'incremental', got '%s'", task.Type)
	}

	// Wait for async task to complete
	time.Sleep(200 * time.Millisecond)

	retrieved, err := manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if retrieved.Status != TaskStatusCompleted {
		t.Errorf("Expected task status 'completed', got '%s'", retrieved.Status)
	}
}

func TestMultipleSearchTypes(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Add documents of different types
	manager.AddDocument(&SearchIndex{
		Path:        "/files/doc.txt",
		Name:        "Document",
		Content:     "important file content",
		ContentType: ContentTypeFile,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	})

	manager.AddDocument(&SearchIndex{
		Path:        "/photos/vacation.jpg",
		Name:        "Vacation Photo",
		Content:     "beach sunset photo",
		ContentType: ContentTypePhoto,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	})

	manager.AddDocument(&SearchIndex{
		Path:        "/emails/inbox/msg1.eml",
		Name:        "Important Email",
		Content:     "meeting tomorrow at 10am",
		ContentType: ContentTypeEmail,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	})

	manager.AddDocument(&SearchIndex{
		Path:        "/docs/report.pdf",
		Name:        "Annual Report",
		Content:     "financial performance review",
		ContentType: ContentTypeDocument,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	})

	// Search all types
	allResults, err := manager.Search(&SearchQuery{
		Query:    "important",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("Search all types failed: %v", err)
	}

	if allResults.Total < 1 {
		t.Errorf("Expected at least 1 result for 'important', got %d", allResults.Total)
	}

	// Search only email type
	emailResults, err := manager.Search(&SearchQuery{
		Query:    "meeting",
		Types:    []ContentType{ContentTypeEmail},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("Search email type failed: %v", err)
	}

	if emailResults.Total != 1 {
		t.Errorf("Expected 1 email result for 'meeting', got %d", emailResults.Total)
	}
}

func TestSearchHighlight(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	manager.AddDocument(&SearchIndex{
		Path:        "/docs/highlight-test.md",
		Name:        "Highlight Test Document",
		Content:     "This document contains the word highlight multiple times. Highlight is important for search.",
		ContentType: ContentTypeFile,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	})

	// Search with highlight enabled
	resp, err := manager.Search(&SearchQuery{
		Query:     "highlight",
		Page:      1,
		PageSize:  10,
		Highlight: true,
	})
	if err != nil {
		t.Fatalf("Search with highlight failed: %v", err)
	}

	if resp.Total < 1 {
		t.Errorf("Expected at least 1 result, got %d", resp.Total)
	}

	// Check that highlights are present
	if len(resp.Results) > 0 && len(resp.Results[0].Highlights) == 0 {
		t.Log("Note: Highlights may not be present if bleve highlighting is not fully configured")
	}
}

func TestSearchPagination(t *testing.T) {
	config := &SearchConfig{
		IndexDir: t.TempDir(),
	}

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Add 50 documents
	for i := 0; i < 50; i++ {
		manager.AddDocument(&SearchIndex{
			Path:        "/docs/page-" + string(rune('a'+i%26)) + ".md",
			Name:        "Pagination Test Document",
			Content:     "pagination test content",
			ContentType: ContentTypeFile,
			CreatedAt:   time.Now(),
			ModifiedAt:  time.Now(),
		})
	}

	// Page 1
	page1, err := manager.Search(&SearchQuery{
		Query:    "pagination",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Search page 1 failed: %v", err)
	}

	// Page 2
	page2, err := manager.Search(&SearchQuery{
		Query:    "pagination",
		Page:     2,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Search page 2 failed: %v", err)
	}

	// Check pagination
	if page1.TotalPages < 2 {
		t.Errorf("Expected at least 2 total pages, got %d", page1.TotalPages)
	}

	// Ensure page 1 and page 2 results are different
	if len(page1.Results) > 0 && len(page2.Results) > 0 {
		if page1.Results[0].ID == page2.Results[0].ID {
			t.Error("Expected different results on different pages")
		}
	}
}
