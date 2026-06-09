package deepsearch

import (
	"testing"
	"time"
)

func TestNewDeepSearchService(t *testing.T) {
	config := DeepSearchConfig{
		IndexPaths:      []string{"/data/documents", "/data/photos"},
		ExcludePaths:    []string{"/data/tmp"},
		MaxFileSize:     100 * 1024 * 1024,
		EnableOCR:       true,
		EnableTranscription: true,
		EnableVisual:    true,
		EnableEmbedding: true,
		EmbeddingModel:  "all-MiniLM-L6-v2",
		BatchSize:       100,
		WorkerCount:     4,
		IndexInterval:   1 * time.Hour,
	}
	svc := NewDeepSearchService(config)
	if svc == nil {
		t.Fatal("NewDeepSearchService returned nil")
	}
	status := svc.GetServiceStatus()
	if status["indexed_files"] != 0 {
		t.Errorf("expected 0 indexed files, got %v", status["indexed_files"])
	}
	if status["known_persons"] != 0 {
		t.Errorf("expected 0 known persons, got %v", status["known_persons"])
	}
}

func TestServiceStartStop(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		WorkerCount:   2,
		IndexInterval: 1 * time.Hour,
	})
	err := svc.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	err = svc.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestAddPerson(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		WorkerCount:   1,
		IndexInterval: 1 * time.Hour,
	})

	person, err := svc.AddPerson("Alice")
	if err != nil {
		t.Fatalf("AddPerson failed: %v", err)
	}
	if person.Name != "Alice" {
		t.Errorf("expected name 'Alice', got %q", person.Name)
	}
	if person.ID == "" {
		t.Error("expected non-empty person ID")
	}
	if person.FaceCount != 0 {
		t.Errorf("expected 0 face count, got %d", person.FaceCount)
	}
	if person.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestGetPersons(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		WorkerCount:   1,
		IndexInterval: 1 * time.Hour,
	})

	svc.AddPerson("Alice")
	svc.AddPerson("Bob")
	svc.AddPerson("Charlie")

	persons := svc.GetPersons()
	if len(persons) != 3 {
		t.Errorf("expected 3 persons, got %d", len(persons))
	}

	names := make(map[string]bool)
	for _, p := range persons {
		names[p.Name] = true
	}
	for _, expected := range []string{"Alice", "Bob", "Charlie"} {
		if !names[expected] {
			t.Errorf("expected person %q in list", expected)
		}
	}
}

func TestGetStats(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		WorkerCount:   1,
		IndexInterval: 1 * time.Hour,
	})

	stats := svc.GetStats()
	if stats.TotalFiles != 0 {
		t.Errorf("expected 0 total files, got %d", stats.TotalFiles)
	}
	if stats.IndexedFiles != 0 {
		t.Errorf("expected 0 indexed files, got %d", stats.IndexedFiles)
	}
	if stats.PendingFiles != 0 {
		t.Errorf("expected 0 pending files, got %d", stats.PendingFiles)
	}
}

func TestGetFileNotFound(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		WorkerCount:   1,
		IndexInterval: 1 * time.Hour,
	})

	_, err := svc.GetFile("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestFindSimilarNotFound(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		WorkerCount:   1,
		IndexInterval: 1 * time.Hour,
	})

	_, err := svc.FindSimilar("nonexistent-id", 10)
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestSearchEmptyIndex(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		WorkerCount:   1,
		IndexInterval: 1 * time.Hour,
	})

	query := SearchQuery{
		Text:      "test query",
		Type:      SearchFilename,
		Limit:     10,
		Offset:    0,
		SortBy:    "relevance",
		SortOrder: "desc",
	}

	resp, err := svc.Search(query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if resp.Total != 0 {
		t.Errorf("expected 0 results, got %d", resp.Total)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results slice, got %d", len(resp.Results))
	}
	if resp.Query.Text != "test query" {
		t.Errorf("expected query text preserved, got %q", resp.Query.Text)
	}
}

func TestGetServiceStatus(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		EnableOCR:         true,
		EnableTranscription: true,
		EnableVisual:      true,
		EnableEmbedding:   true,
		WorkerCount:       2,
		IndexInterval:     1 * time.Hour,
	})

	svc.AddPerson("Alice")
	svc.AddPerson("Bob")

	status := svc.GetServiceStatus()
	if status["known_persons"] != 2 {
		t.Errorf("expected 2 known persons, got %v", status["known_persons"])
	}
	if status["indexed_files"] != 0 {
		t.Errorf("expected 0 indexed files, got %v", status["indexed_files"])
	}
	cfg, ok := status["config"].(map[string]interface{})
	if !ok {
		t.Fatal("expected config to be a map")
	}
	if cfg["ocr_enabled"] != true {
		t.Errorf("expected ocr_enabled true, got %v", cfg["ocr_enabled"])
	}
	if cfg["semantic_search"] != true {
		t.Errorf("expected semantic_search true, got %v", cfg["semantic_search"])
	}
}

func TestMatchesFiltersFileType(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		WorkerCount:   1,
		IndexInterval: 1 * time.Hour,
	})

	file := &IndexedFile{
		FileType: FileDocument,
		Size:     1024,
		ModTime:  time.Now(),
		Path:     "/data/docs/test.pdf",
	}

	// Matching file type
	query := SearchQuery{
		FileTypes: []FileType{FileDocument, FileImage},
		Limit:     10,
	}
	if !svc.matchesFilters(file, query) {
		t.Error("expected file to match document filter")
	}

	// Non-matching file type
	query2 := SearchQuery{
		FileTypes: []FileType{FileVideo, FileAudio},
		Limit:     10,
	}
	if svc.matchesFilters(file, query2) {
		t.Error("expected file NOT to match video/audio filter")
	}
}

func TestMatchesFiltersSize(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		WorkerCount:   1,
		IndexInterval: 1 * time.Hour,
	})

	file := &IndexedFile{
		Size:    5000,
		ModTime: time.Now(),
		Path:    "/data/test.txt",
	}

	minSize := int64(1000)
	maxSize := int64(10000)
	query := SearchQuery{
		SizeMin: &minSize,
		SizeMax: &maxSize,
		Limit:   10,
	}
	if !svc.matchesFilters(file, query) {
		t.Error("expected file to match size filter (5000 is between 1000 and 10000)")
	}

	tooSmall := int64(6000)
	query2 := SearchQuery{
		SizeMin: &tooSmall,
		Limit:   10,
	}
	if svc.matchesFilters(file, query2) {
		t.Error("expected file NOT to match size filter (5000 < 6000)")
	}

	tooLarge := int64(3000)
	query3 := SearchQuery{
		SizeMax: &tooLarge,
		Limit:   10,
	}
	if svc.matchesFilters(file, query3) {
		t.Error("expected file NOT to match size filter (5000 > 3000)")
	}
}

func TestMatchesFiltersDate(t *testing.T) {
	svc := NewDeepSearchService(DeepSearchConfig{
		WorkerCount:   1,
		IndexInterval: 1 * time.Hour,
	})

	file := &IndexedFile{
		Size:    1024,
		ModTime: time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
		Path:    "/data/test.txt",
	}

	after := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	query := SearchQuery{
		DateFrom: &after,
		DateTo:   &before,
		Limit:    10,
	}
	if !svc.matchesFilters(file, query) {
		t.Error("expected file to be within date range")
	}

	outsideAfter := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	query2 := SearchQuery{
		DateFrom: &outsideAfter,
		Limit:    10,
	}
	if svc.matchesFilters(file, query2) {
		t.Error("expected file NOT to match date filter (file is before dateFrom)")
	}
}
