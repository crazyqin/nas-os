package smartsearch2

import (
	"testing"
	"time"
)

func TestNewSearchService(t *testing.T) {
	config := SearchConfig{
		IndexDir:        "/tmp/test-index",
		MaxIndexSize:    1024 * 1024,
		IndexInterval:   time.Minute,
		EnableContent:   true,
		SpotlightCompat: true,
	}
	svc := NewSearchService(config)
	if svc == nil {
		t.Fatal("NewSearchService 返回 nil")
	}
}

func TestNewSearchServiceDefaults(t *testing.T) {
	svc := NewSearchService(SearchConfig{})
	if svc == nil {
		t.Fatal("NewSearchService 返回 nil")
	}
}

func TestSearchServiceStartStop(t *testing.T) {
	svc := NewSearchService(SearchConfig{IndexDir: "/tmp/test-index"})
	if err := svc.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if err := svc.Stop(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	svc := NewSearchService(SearchConfig{IndexDir: "/tmp/test-index"})
	svc.Start()
	defer svc.Stop()

	_, err := svc.Search("", SearchOptions{})
	if err == nil {
		t.Fatal("空查询应返回错误")
	}
}

func TestGetStats(t *testing.T) {
	svc := NewSearchService(SearchConfig{IndexDir: "/tmp/test-index"})
	svc.Start()
	defer svc.Stop()

	stats := svc.GetStats()
	if stats.TotalIndexed != 0 {
		t.Fatalf("初始索引数应为0，实际 %d", stats.TotalIndexed)
	}
}

func TestSearchResult(t *testing.T) {
	r := SearchResult{
		ID:   "test-1",
		Path: "/data/test.txt",
		Name: "test.txt",
		Size: 1024,
	}
	if r.ID != "test-1" {
		t.Fatal("ID不匹配")
	}
}

func TestIndexEntry(t *testing.T) {
	e := IndexEntry{
		ID:        "entry-1",
		Path:      "/data/file.txt",
		Name:      "file.txt",
		Extension: ".txt",
		Size:      2048,
		MimeType:  "text/plain",
		Content:   "hello world",
	}
	if e.Content != "hello world" {
		t.Fatal("Content不匹配")
	}
}

func TestSearchStats(t *testing.T) {
	stats := SearchStats{
		TotalIndexed:  100,
		TotalSearches: 50,
		IndexSize:     1024 * 1024,
		AvgSearchTime: 1.5,
	}
	if stats.TotalIndexed != 100 {
		t.Fatal("TotalIndexed不匹配")
	}
}

func TestSearchConfig(t *testing.T) {
	config := SearchConfig{
		IndexDir:        "/var/lib/nas-os/search",
		MaxIndexSize:    5 * 1024 * 1024 * 1024,
		IndexInterval:   15 * time.Minute,
		EnableContent:   true,
		EnableOCR:       true,
		SpotlightCompat: true,
		MaxFileSize:     100 * 1024 * 1024,
	}
	if config.IndexDir != "/var/lib/nas-os/search" {
		t.Fatal("IndexDir不匹配")
	}
	if !config.EnableContent {
		t.Fatal("EnableContent应为true")
	}
	if !config.SpotlightCompat {
		t.Fatal("SpotlightCompat应为true")
	}
}
