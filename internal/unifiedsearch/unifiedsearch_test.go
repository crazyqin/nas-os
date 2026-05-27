package unifiedsearch

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:         true,
		IndexPath:       "/tmp/search-index",
		MaxResults:      100,
		IndexBatchSize:  50,
		SemanticEnabled: true,
		FuzzyThreshold:  0.8,
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if !manager.config.Enabled {
		t.Error("Expected config.Enabled to be true")
	}
}

func TestIndexAndSearch(t *testing.T) {
	config := &Config{
		Enabled:    true,
		MaxResults: 100,
	}

	manager := NewManager(config)

	entry := &IndexEntry{
		ID:       "test-1",
		Content:  "This is a test document about NAS storage",
		Title:    "Test Document",
		Path:     "/docs/test.md",
		Type:     "file",
		MimeType: "text/markdown",
		Size:     1024,
		Tags:     []string{"test", "nas"},
		Source:   "files",
	}

	if err := manager.Index(entry); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	stats := manager.GetStats()
	if stats.TotalEntries != 1 {
		t.Errorf("Expected 1 entry, got %d", stats.TotalEntries)
	}

	req := &SearchRequest{
		Query:    "NAS storage",
		Scope:    ScopeAll,
		PageSize: 10,
	}

	resp, err := manager.Search(req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp.TotalHits != 1 {
		t.Errorf("Expected 1 result, got %d", resp.TotalHits)
	}
}

func TestRemove(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	entry := &IndexEntry{
		ID:      "test-remove",
		Content: "test",
		Title:   "Test",
		Type:    "file",
		Source:  "files",
	}

	manager.Index(entry)

	if err := manager.Remove("test-remove"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	stats := manager.GetStats()
	if stats.TotalEntries != 0 {
		t.Errorf("Expected 0 entries after remove, got %d", stats.TotalEntries)
	}
}

func TestSearchEmpty(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	req := &SearchRequest{
		Query:    "nonexistent",
		Scope:    ScopeAll,
		PageSize: 10,
	}

	resp, err := manager.Search(req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp.TotalHits != 0 {
		t.Errorf("Expected 0 results, got %d", resp.TotalHits)
	}
}
