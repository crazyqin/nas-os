package unifiedsearch

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := &SearchConfig{
		MaxHistory:      100,
		MaxHotSearches:  50,
		FuzzyThreshold:  0.8,
		HighlightPre:    "<mark>",
		HighlightPost:   "</mark>",
		SummaryLength:   200,
		MaxPageSize:     100,
		DefaultPageSize: 20,
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.config.FuzzyThreshold != 0.8 {
		t.Errorf("Expected FuzzyThreshold 0.8, got %f", manager.config.FuzzyThreshold)
	}
}

func TestNewManagerNilConfig(t *testing.T) {
	manager := NewManager(nil)
	if manager == nil {
		t.Fatal("NewManager(nil) returned nil")
	}
	if manager.config == nil {
		t.Fatal("Expected default config, got nil")
	}
}

func TestAddDocumentAndSearch(t *testing.T) {
	config := &SearchConfig{
		MaxHistory:      100,
		MaxHotSearches:  50,
		FuzzyThreshold:  0.6,
		HighlightPre:    "<mark>",
		HighlightPost:   "</mark>",
		SummaryLength:   200,
		MaxPageSize:     100,
		DefaultPageSize: 20,
	}

	manager := NewManager(config)

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
		Query:     "NAS storage",
		PageSize:  10,
		Page:      1,
		Highlight: true,
	}

	resp, err := manager.Search(req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 result, got %d", resp.Total)
	}
}

func TestRemoveDocument(t *testing.T) {
	manager := NewManager(nil)

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
	manager := NewManager(nil)

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
	manager := NewManager(nil)

	_, err := manager.Search(nil)
	if err == nil {
		t.Fatal("Expected error for nil query, got nil")
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	manager := NewManager(nil)

	_, err := manager.Search(&SearchQuery{})
	if err == nil {
		t.Fatal("Expected error for empty query, got nil")
	}
}

func TestAddDocumentNil(t *testing.T) {
	manager := NewManager(nil)

	err := manager.AddDocument(nil)
	if err == nil {
		t.Fatal("Expected error for nil document, got nil")
	}
}

func TestAddDocumentEmptyPath(t *testing.T) {
	manager := NewManager(nil)

	err := manager.AddDocument(&SearchIndex{Name: "test"})
	if err == nil {
		t.Fatal("Expected error for empty path, got nil")
	}
}

func TestUpdateDocument(t *testing.T) {
	manager := NewManager(nil)

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

	err := manager.UpdateDocument(&UpdateIndexRequest{
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
	manager := NewManager(nil)

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

func TestPauseResumeIndex(t *testing.T) {
	manager := NewManager(nil)

	// Can't pause when not building
	err := manager.PauseIndex()
	if err == nil {
		t.Log("Expected error when pausing non-building index")
	}

	// Can't resume when not paused
	err = manager.ResumeIndex()
	if err == nil {
		t.Log("Expected error when resuming non-paused index")
	}
}

func TestSearchHistory(t *testing.T) {
	manager := NewManager(nil)

	// Add some documents first
	entry := &SearchIndex{
		Path:        "/docs/history-test.md",
		Name:        "History Test",
		Content:     "search history test content",
		ContentType: ContentTypeFile,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	}
	manager.AddDocument(entry)

	// Search to generate history
	manager.Search(&SearchQuery{Query: "history test", Page: 1, PageSize: 10})

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
		MaxHistory:     500,
		FuzzyThreshold: 0.7,
	}
	manager := NewManager(config)

	cfg := manager.GetConfig()
	if cfg.MaxHistory != 500 {
		t.Errorf("Expected MaxHistory 500, got %d", cfg.MaxHistory)
	}
	if cfg.FuzzyThreshold != 0.7 {
		t.Errorf("Expected FuzzyThreshold 0.7, got %f", cfg.FuzzyThreshold)
	}
}

func TestUpdateConfig(t *testing.T) {
	manager := NewManager(nil)

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
	manager := NewManager(nil)

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
	manager := NewManager(nil)

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

	if resp.Total != 1 {
		t.Errorf("Expected 1 result, got %d", resp.Total)
	}
}
