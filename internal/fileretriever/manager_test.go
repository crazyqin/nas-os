package fileretriever

import (
	"testing"
)

func TestNewFileRetrieverManager(t *testing.T) {
	manager := NewFileRetrieverManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	if manager.config == nil {
		t.Fatal("Expected default config")
	}
}

func TestIndexFile(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	entry := &FileEntry{
		Path:      "/data/test.txt",
		Name:      "test.txt",
		Size:      1024,
		Extension: ".txt",
		IsDir:     false,
	}

	err := manager.IndexFile(entry)
	if err != nil {
		t.Fatalf("IndexFile failed: %v", err)
	}

	if entry.IndexedAt.IsZero() {
		t.Error("Expected IndexedAt to be set")
	}
}

func TestIndexFileEmptyPath(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	err := manager.IndexFile(&FileEntry{})
	if err == nil {
		t.Error("Expected error for empty path")
	}
}

func TestGetEntry(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	entry := &FileEntry{
		Path: "/data/test.txt",
		Name: "test.txt",
		Size: 1024,
	}
	manager.IndexFile(entry)

	fetched, err := manager.GetEntry("/data/test.txt")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if fetched.Name != "test.txt" {
		t.Errorf("Expected name 'test.txt', got '%s'", fetched.Name)
	}
}

func TestGetEntryNotFound(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	_, err := manager.GetEntry("/nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent entry")
	}
}

func TestRemoveEntry(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	entry := &FileEntry{Path: "/data/test.txt", Name: "test.txt"}
	manager.IndexFile(entry)

	err := manager.RemoveEntry("/data/test.txt")
	if err != nil {
		t.Fatalf("RemoveEntry failed: %v", err)
	}

	_, err = manager.GetEntry("/data/test.txt")
	if err == nil {
		t.Error("Expected error after removal")
	}
}

func TestIndexBatch(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	entries := []*FileEntry{
		{Path: "/data/file1.txt", Name: "file1.txt"},
		{Path: "/data/file2.txt", Name: "file2.txt"},
		{Path: "/data/file3.txt", Name: "file3.txt"},
	}

	count, err := manager.IndexBatch(entries)
	if err != nil {
		t.Fatalf("IndexBatch failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 indexed, got %d", count)
	}
}

func TestSearch(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	manager.IndexFile(&FileEntry{Path: "/data/test.txt", Name: "test.txt", Extension: ".txt"})
	manager.IndexFile(&FileEntry{Path: "/data/test.log", Name: "test.log", Extension: ".log"})
	manager.IndexFile(&FileEntry{Path: "/data/readme.md", Name: "readme.md", Extension: ".md"})

	req := &SearchRequest{
		Query: "test",
	}

	task, err := manager.Search(req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if task.Total != 2 {
		t.Errorf("Expected 2 results, got %d", task.Total)
	}
}

func TestSearchWithPath(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	manager.IndexFile(&FileEntry{Path: "/data/test.txt", Name: "test.txt"})
	manager.IndexFile(&FileEntry{Path: "/backup/test.txt", Name: "test.txt"})

	req := &SearchRequest{
		Query: "test",
		Path:  "/data",
	}

	task, err := manager.Search(req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if task.Total != 1 {
		t.Errorf("Expected 1 result, got %d", task.Total)
	}
}

func TestSearchWithExtension(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	manager.IndexFile(&FileEntry{Path: "/data/test.txt", Name: "test.txt", Extension: ".txt"})
	manager.IndexFile(&FileEntry{Path: "/data/test.log", Name: "test.log", Extension: ".log"})

	req := &SearchRequest{
		Extension: ".txt",
	}

	task, err := manager.Search(req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if task.Total != 1 {
		t.Errorf("Expected 1 result, got %d", task.Total)
	}
}

func TestGetStats(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	manager.IndexFile(&FileEntry{Path: "/data/test.txt", Name: "test.txt"})

	stats := manager.GetStats()
	if stats["total_indexed"] != 1 {
		t.Errorf("Expected 1 indexed, got %v", stats["total_indexed"])
	}
}

func TestClearIndex(t *testing.T) {
	manager := NewFileRetrieverManager(nil)

	manager.IndexFile(&FileEntry{Path: "/data/test.txt", Name: "test.txt"})
	manager.ClearIndex()

	stats := manager.GetStats()
	if stats["total_indexed"] != 0 {
		t.Errorf("Expected 0 indexed after clear, got %v", stats["total_indexed"])
	}
}
