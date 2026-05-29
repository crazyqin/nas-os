package smartfileindex

import (
	"context"
	"testing"
	"time"
)

func TestNewSmartFileIndex(t *testing.T) {
	paths := []string{"/tmp/test"}
	sfi := NewSmartFileIndex(paths, 1000)
	if sfi == nil {
		t.Fatal("expected non-nil SmartFileIndex")
	}
	if sfi.maxEntries != 1000 {
		t.Errorf("expected maxEntries 1000, got %d", sfi.maxEntries)
	}
}

func TestSearch(t *testing.T) {
	sfi := NewSmartFileIndex([]string{}, 1000)
	
	// Add test entries
	sfi.entries["/test/file1.txt"] = &IndexEntry{
		Path:    "/test/file1.txt",
		Name:    "file1.txt",
		Size:    1024,
		ModTime: time.Now(),
	}
	sfi.entries["/test/file2.go"] = &IndexEntry{
		Path:    "/test/file2.go",
		Name:    "file2.go",
		Size:    2048,
		ModTime: time.Now(),
	}

	// Test keyword search
	results, err := sfi.Search(SearchQuery{Keyword: "file1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	// Test size filter
	results, err = sfi.Search(SearchQuery{MinSize: 1500})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestGetStats(t *testing.T) {
	sfi := NewSmartFileIndex([]string{"/tmp"}, 500)
	stats := sfi.GetStats()
	if stats["max_entries"] != 500 {
		t.Errorf("expected max_entries 500, got %v", stats["max_entries"])
	}
}

func TestStartStop(t *testing.T) {
	sfi := NewSmartFileIndex([]string{}, 1000)
	ctx := context.Background()
	
	err := sfi.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Try starting again
	err = sfi.Start(ctx)
	if err == nil {
		t.Error("expected error on double start")
	}

	sfi.Stop()
}
