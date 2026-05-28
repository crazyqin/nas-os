package ragsearch

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:         true,
		IndexPath:       "/tmp/ragsearch-test",
		MaxResults:      100,
		VectorDimension: 128,
		BM25K1:          1.2,
		BM25B:           0.75,
		RRFK:            60,
		HistoryMaxSize:  1000,
		SuggestionLimit: 10,
		FreshnessWeight: 0.1,
		SemanticWeight:  0.4,
		FullTextWeight:  0.5,
	}

	manager := NewManager(config)
	assert.NotNil(t, manager)
	assert.True(t, manager.config.Enabled)
	assert.Equal(t, 128, manager.config.VectorDimension)
}

func TestNewManagerDefaultConfig(t *testing.T) {
	manager := NewManager(nil)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
	assert.Equal(t, 128, manager.config.VectorDimension)
	assert.Equal(t, 1.2, manager.config.BM25K1)
}

func TestAddDocument(t *testing.T) {
	manager := NewManager(DefaultConfig())

	doc := &Document{
		ID:       "doc-1",
		Title:    "Test Document",
		Content:  "This is a test document about NAS storage systems",
		Path:     "/docs/test.md",
		DocType:  DocTypeFile,
		MimeType: "text/markdown",
		Size:     1024,
		Tags:     []string{"test", "nas"},
		Source:   "files",
	}

	err := manager.AddDocument(doc)
	assert.NoError(t, err)

	entry, err := manager.GetDocument("doc-1")
	assert.NoError(t, err)
	assert.NotNil(t, entry)
	assert.Equal(t, "Test Document", entry.Title)
	assert.Equal(t, DocTypeFile, entry.DocType)
	assert.NotEmpty(t, entry.Embedding)
	assert.NotEmpty(t, entry.TermFreq)
}

func TestAddDocumentNil(t *testing.T) {
	manager := NewManager(DefaultConfig())
	err := manager.AddDocument(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestAddDocumentEmptyID(t *testing.T) {
	manager := NewManager(DefaultConfig())
	doc := &Document{
		Title:   "No ID",
		Content: "test",
	}
	err := manager.AddDocument(doc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestUpdateDocument(t *testing.T) {
	manager := NewManager(DefaultConfig())

	doc := &Document{
		ID:      "doc-1",
		Title:   "Original",
		Content: "Original content",
		DocType: DocTypeFile,
	}

	err := manager.AddDocument(doc)
	assert.NoError(t, err)

	doc.Title = "Updated"
	doc.Content = "Updated content with more information"
	err = manager.UpdateDocument(doc)
	assert.NoError(t, err)

	entry, err := manager.GetDocument("doc-1")
	assert.NoError(t, err)
	assert.Equal(t, "Updated", entry.Title)
}

func TestRemoveDocument(t *testing.T) {
	manager := NewManager(DefaultConfig())

	doc := &Document{
		ID:      "doc-1",
		Title:   "To Remove",
		Content: "This document will be removed",
		DocType: DocTypeFile,
	}

	manager.AddDocument(doc)
	stats := manager.GetStats()
	assert.Equal(t, int64(1), stats.TotalEntries)

	err := manager.RemoveDocument("doc-1")
	assert.NoError(t, err)

	stats = manager.GetStats()
	assert.Equal(t, int64(0), stats.TotalEntries)

	_, err = manager.GetDocument("doc-1")
	assert.Error(t, err)
}

func TestRemoveDocumentNotFound(t *testing.T) {
	manager := NewManager(DefaultConfig())
	err := manager.RemoveDocument("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFullTextSearch(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "NAS Storage Guide", Content: "This guide covers NAS storage setup and configuration", DocType: DocTypeFile, Source: "files"},
		{ID: "2", Title: "Backup Strategy", Content: "Important backup strategies for your NAS system", DocType: DocTypeFile, Source: "files"},
		{ID: "3", Title: "Network Configuration", Content: "Configure your network for optimal NAS performance", DocType: DocTypeFile, Source: "files"},
		{ID: "4", Title: "Docker on NAS", Content: "Running Docker containers on your NAS storage", DocType: DocTypeFile, Source: "files"},
	}

	for _, doc := range docs {
		err := manager.AddDocument(doc)
		assert.NoError(t, err)
	}

	query := &SearchQuery{
		Query: "NAS storage",
		Mode:  ModeFullText,
		Limit: 10,
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Greater(t, resp.TotalHits, 0)
	assert.GreaterOrEqual(t, resp.SearchTime, int64(0))
}

func TestSemanticSearch(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "Machine Learning", Content: "Introduction to machine learning algorithms and neural networks", DocType: DocTypeFile, Source: "files"},
		{ID: "2", Title: "Deep Learning", Content: "Deep learning with convolutional neural networks", DocType: DocTypeFile, Source: "files"},
		{ID: "3", Title: "Cooking Recipe", Content: "How to make delicious pasta with tomato sauce", DocType: DocTypeFile, Source: "files"},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	query := &SearchQuery{
		Query: "artificial intelligence and neural networks",
		Mode:  ModeSemantic,
		Limit: 10,
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestHybridSearch(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "NAS Setup Guide", Content: "Complete guide to setting up your NAS storage system", DocType: DocTypeFile, Source: "files"},
		{ID: "2", Title: "Storage Management", Content: "Managing storage pools and volumes on NAS", DocType: DocTypeFile, Source: "files"},
		{ID: "3", Title: "Photo Backup", Content: "Automatic photo backup from mobile to NAS", DocType: DocTypePhoto, Source: "photos"},
		{ID: "4", Title: "Docker Guide", Content: "Running Docker containers on your NAS", DocType: DocTypeFile, Source: "files"},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	query := &SearchQuery{
		Query: "NAS storage",
		Mode:  ModeHybrid,
		Limit: 10,
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Greater(t, resp.TotalHits, 0)

	// Check that results have rank scores
	for _, r := range resp.Results {
		assert.NotNil(t, r.RankScore)
		assert.GreaterOrEqual(t, r.RankScore.BM25Score, float64(0))
	}
}

func TestSearchWithFilters(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "Photo 1", Content: "A beautiful sunset photo", DocType: DocTypePhoto, Source: "photos", Size: 5000000},
		{ID: "2", Title: "Document 1", Content: "Important business document", DocType: DocTypeDoc, Source: "files", Size: 100000},
		{ID: "3", Title: "Video 1", Content: "Family vacation video", DocType: DocTypeVideo, Source: "videos", Size: 500000000},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	// Filter by doc type
	query := &SearchQuery{
		Query: "beautiful",
		Mode:  ModeFullText,
		Limit: 10,
		Filters: &SearchFilter{
			DocTypes: []DocumentType{DocTypePhoto},
		},
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	for _, r := range resp.Results {
		assert.Equal(t, DocTypePhoto, r.DocType)
	}
}

func TestSearchWithDateFilter(t *testing.T) {
	manager := NewManager(DefaultConfig())

	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour)

	docs := []*Document{
		{ID: "1", Title: "Recent", Content: "Recent document about NAS", DocType: DocTypeFile, ModifiedAt: now},
		{ID: "2", Title: "Old", Content: "Old document about NAS", DocType: DocTypeFile, ModifiedAt: old},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	dateFrom := now.Add(-7 * 24 * time.Hour)
	query := &SearchQuery{
		Query: "document",
		Mode:  ModeFullText,
		Limit: 10,
		Filters: &SearchFilter{
			DateFrom: &dateFrom,
		},
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	for _, r := range resp.Results {
		assert.True(t, r.ModifiedAt.After(dateFrom) || r.ModifiedAt.Equal(dateFrom))
	}
}

func TestSearchWithSizeFilter(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "Small", Content: "Small NAS file", DocType: DocTypeFile, Size: 100},
		{ID: "2", Title: "Large", Content: "Large NAS file", DocType: DocTypeFile, Size: 1000000},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	sizeMin := int64(500)
	query := &SearchQuery{
		Query: "NAS",
		Mode:  ModeFullText,
		Limit: 10,
		Filters: &SearchFilter{
			SizeMin: &sizeMin,
		},
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	for _, r := range resp.Results {
		assert.GreaterOrEqual(t, r.Size, sizeMin)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	manager := NewManager(DefaultConfig())

	query := &SearchQuery{
		Query: "",
		Mode:  ModeHybrid,
		Limit: 10,
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.TotalHits)
	assert.Empty(t, resp.Results)
}

func TestSearchNilQuery(t *testing.T) {
	manager := NewManager(DefaultConfig())
	_, err := manager.Search(nil)
	assert.Error(t, err)
}

func TestSortResults(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "Alpha", Content: "NAS storage", DocType: DocTypeFile, ModifiedAt: time.Now().Add(-2 * time.Hour), Size: 100},
		{ID: "2", Title: "Beta", Content: "NAS storage", DocType: DocTypeFile, ModifiedAt: time.Now(), Size: 500},
		{ID: "3", Title: "Gamma", Content: "NAS storage", DocType: DocTypeFile, ModifiedAt: time.Now().Add(-1 * time.Hour), Size: 300},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	// Sort by name
	query := &SearchQuery{
		Query: "NAS",
		Mode:  ModeFullText,
		Sort:  SortName,
		Limit: 10,
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.Len(t, resp.Results, 3)
	assert.Equal(t, "Alpha", resp.Results[0].Title)
	assert.Equal(t, "Beta", resp.Results[1].Title)
	assert.Equal(t, "Gamma", resp.Results[2].Title)

	// Sort by size
	query.Sort = SortSize
	resp, err = manager.Search(query)
	assert.NoError(t, err)
	assert.Equal(t, "Beta", resp.Results[0].Title)
}

func TestGetSuggestions(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "NAS Configuration", Content: "Configure your NAS", DocType: DocTypeFile},
		{ID: "2", Title: "NAS Backup", Content: "Backup your NAS", DocType: DocTypeFile},
		{ID: "3", Title: "Network Setup", Content: "Setup your network", DocType: DocTypeFile},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	req := &SuggestionRequest{
		Prefix: "na",
		Limit:  5,
	}

	resp, err := manager.GetSuggestions(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Suggestions)
}

func TestGetSuggestionsEmptyPrefix(t *testing.T) {
	manager := NewManager(DefaultConfig())
	req := &SuggestionRequest{Prefix: ""}
	resp, err := manager.GetSuggestions(req)
	assert.NoError(t, err)
	assert.Empty(t, resp.Suggestions)
}

func TestGetSuggestionsNilRequest(t *testing.T) {
	manager := NewManager(DefaultConfig())
	_, err := manager.GetSuggestions(nil)
	assert.Error(t, err)
}

func TestSearchHistory(t *testing.T) {
	manager := NewManager(DefaultConfig())

	doc := &Document{
		ID:      "1",
		Title:   "Test",
		Content: "Test NAS content",
		DocType: DocTypeFile,
	}
	manager.AddDocument(doc)

	// Perform some searches
	manager.Search(&SearchQuery{Query: "NAS", Mode: ModeFullText, Limit: 10})
	manager.Search(&SearchQuery{Query: "storage", Mode: ModeFullText, Limit: 10})
	manager.Search(&SearchQuery{Query: "NAS", Mode: ModeFullText, Limit: 10})

	history := manager.GetSearchHistory(10)
	assert.NotNil(t, history)
	assert.Len(t, history, 3)
	assert.Equal(t, "NAS", history[len(history)-1].Query)
}

func TestHotQueries(t *testing.T) {
	manager := NewManager(DefaultConfig())

	doc := &Document{
		ID:      "1",
		Title:   "Test",
		Content: "Test NAS storage content",
		DocType: DocTypeFile,
	}
	manager.AddDocument(doc)

	// Multiple searches for same query
	for i := 0; i < 5; i++ {
		manager.Search(&SearchQuery{Query: "NAS", Mode: ModeFullText, Limit: 10})
	}
	manager.Search(&SearchQuery{Query: "other", Mode: ModeFullText, Limit: 10})

	hot := manager.GetHotQueries(10)
	assert.NotNil(t, hot)
	assert.NotEmpty(t, hot)
	assert.Equal(t, "NAS", hot[0].Query)
}

func TestGetStats(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "File 1", Content: "Content 1", DocType: DocTypeFile},
		{ID: "2", Title: "Photo 1", Content: "Content 2", DocType: DocTypePhoto},
		{ID: "3", Title: "File 2", Content: "Content 3", DocType: DocTypeFile},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	stats := manager.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(3), stats.TotalEntries)
	assert.Equal(t, int64(2), stats.EntriesByType[string(DocTypeFile)])
	assert.Equal(t, int64(1), stats.EntriesByType[string(DocTypePhoto)])
	assert.Equal(t, "healthy", stats.IndexHealth)
}

func TestRebuildIndex(t *testing.T) {
	manager := NewManager(DefaultConfig())

	doc := &Document{
		ID:      "1",
		Title:   "Test",
		Content: "Test content",
		DocType: DocTypeFile,
	}
	manager.AddDocument(doc)

	stats := manager.GetStats()
	assert.Equal(t, int64(1), stats.TotalEntries)

	err := manager.RebuildIndex()
	assert.NoError(t, err)

	stats = manager.GetStats()
	assert.Equal(t, int64(0), stats.TotalEntries)
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"hello world", []string{"hello", "world"}},
		{"NAS-Storage System", []string{"nas", "storage", "system"}},
		{"a", nil}, // single char skipped
		{"test123 abc", []string{"test123", "abc"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := tokenize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float64
		expected float64
	}{
		{"identical", []float64{1, 0, 0}, []float64{1, 0, 0}, 1.0},
		{"orthogonal", []float64{1, 0, 0}, []float64{0, 1, 0}, 0.0},
		{"opposite", []float64{1, 0, 0}, []float64{-1, 0, 0}, -1.0},
		{"empty", []float64{}, []float64{}, 0.0},
		{"different lengths", []float64{1, 2}, []float64{1}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestRankByScore(t *testing.T) {
	scores := map[string]float64{
		"doc1": 0.5,
		"doc2": 0.9,
		"doc3": 0.1,
		"doc4": 0.7,
	}

	ranked := rankByScore(scores)
	assert.Len(t, ranked, 4)
	assert.Equal(t, "doc2", ranked[0])
	assert.Equal(t, "doc4", ranked[1])
	assert.Equal(t, "doc1", ranked[2])
	assert.Equal(t, "doc3", ranked[3])
}

func TestGenerateHighlight(t *testing.T) {
	content := "This is a long document about NAS storage systems and how to configure them properly for home use"

	tests := []struct {
		name      string
		tokens    []string
		empty     bool
	}{
		{"found", []string{"nas", "storage"}, false},
		{"not found", []string{"xyz", "abc"}, false},
		{"empty", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateHighlight(content, tt.tokens)
			if tt.empty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestStop(t *testing.T) {
	manager := NewManager(DefaultConfig())
	assert.NotPanics(t, func() {
		manager.Stop()
	})
}

func TestPagination(t *testing.T) {
	manager := NewManager(DefaultConfig())

	for i := 0; i < 20; i++ {
		doc := &Document{
			ID:      fmt.Sprintf("doc-%d", i),
			Title:   fmt.Sprintf("Document %d about NAS", i),
			Content: fmt.Sprintf("NAS storage document number %d", i),
			DocType: DocTypeFile,
		}
		manager.AddDocument(doc)
	}

	// Page 1
	query := &SearchQuery{
		Query:  "NAS",
		Mode:   ModeFullText,
		Limit:  5,
		Offset: 0,
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.Len(t, resp.Results, 5)
	assert.True(t, resp.HasMore)

	// Page 2
	query.Offset = 5
	resp, err = manager.Search(query)
	assert.NoError(t, err)
	assert.Len(t, resp.Results, 5)
	assert.True(t, resp.HasMore)
}

func TestSearchWithFacets(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "File", Content: "NAS file content", DocType: DocTypeFile},
		{ID: "2", Title: "Photo", Content: "NAS photo content", DocType: DocTypePhoto},
		{ID: "3", Title: "Video", Content: "NAS video content", DocType: DocTypeVideo},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	query := &SearchQuery{
		Query:  "NAS",
		Mode:   ModeFullText,
		Limit:  10,
		Facets: true,
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.NotNil(t, resp.Facets)
	assert.NotEmpty(t, resp.Facets)
}

func TestBM25ScoreCalculation(t *testing.T) {
	config := DefaultConfig()
	config.BM25K1 = 1.2
	config.BM25B = 0.75
	manager := NewManager(config)

	docs := []*Document{
		{ID: "1", Title: "NAS Guide", Content: "Complete NAS storage guide for beginners", DocType: DocTypeFile},
		{ID: "2", Title: "Network", Content: "Network configuration for home use", DocType: DocTypeFile},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	query := &SearchQuery{
		Query: "NAS storage",
		Mode:  ModeFullText,
		Limit: 10,
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.Greater(t, resp.TotalHits, 0)

	// First result should have higher score
	if len(resp.Results) >= 2 {
		assert.Greater(t, resp.Results[0].Score, resp.Results[1].Score)
	}
}

func TestSearchHighlight(t *testing.T) {
	manager := NewManager(DefaultConfig())

	doc := &Document{
		ID:      "1",
		Title:   "NAS Setup",
		Content: "This is a comprehensive guide to setting up your NAS storage system for home and office use",
		DocType: DocTypeFile,
	}
	manager.AddDocument(doc)

	query := &SearchQuery{
		Query:     "NAS",
		Mode:      ModeFullText,
		Limit:     10,
		Highlight: true,
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Results)

	if len(resp.Results) > 0 {
		assert.NotEmpty(t, resp.Results[0].Highlight)
	}
}

func TestMultipleDocTypes(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "File", Content: "NAS file", DocType: DocTypeFile},
		{ID: "2", Title: "Note", Content: "NAS note", DocType: DocTypeNote},
		{ID: "3", Title: "Email", Content: "NAS email", DocType: DocTypeEmail},
		{ID: "4", Title: "PDF", Content: "NAS PDF document", DocType: DocTypePDF},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	stats := manager.GetStats()
	assert.Equal(t, int64(4), stats.TotalEntries)
	assert.Equal(t, int64(1), stats.EntriesByType[string(DocTypeFile)])
	assert.Equal(t, int64(1), stats.EntriesByType[string(DocTypeNote)])
	assert.Equal(t, int64(1), stats.EntriesByType[string(DocTypeEmail)])
	assert.Equal(t, int64(1), stats.EntriesByType[string(DocTypePDF)])
}

func TestSearchWithTagsFilter(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "Tagged", Content: "NAS content", DocType: DocTypeFile, Tags: []string{"important", "backup"}},
		{ID: "2", Title: "Untagged", Content: "NAS content", DocType: DocTypeFile, Tags: []string{}},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	query := &SearchQuery{
		Query: "NAS",
		Mode:  ModeFullText,
		Limit: 10,
		Filters: &SearchFilter{
			Tags: []string{"important"},
		},
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	for _, r := range resp.Results {
		assert.Contains(t, r.Tags, "important")
	}
}

func TestSearchWithSourceFilter(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "File", Content: "NAS content", DocType: DocTypeFile, Source: "files"},
		{ID: "2", Title: "Photo", Content: "NAS content", DocType: DocTypePhoto, Source: "photos"},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	query := &SearchQuery{
		Query: "NAS",
		Mode:  ModeFullText,
		Limit: 10,
		Filters: &SearchFilter{
			Sources: []string{"photos"},
		},
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)

	for _, r := range resp.Results {
		assert.Equal(t, "photos", r.Source)
	}
}

func TestSearchWithMimeTypeFilter(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "Markdown", Content: "NAS content", DocType: DocTypeFile, MimeType: "text/markdown"},
		{ID: "2", Title: "PDF", Content: "NAS content", DocType: DocTypePDF, MimeType: "application/pdf"},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	query := &SearchQuery{
		Query: "NAS",
		Mode:  ModeFullText,
		Limit: 10,
		Filters: &SearchFilter{
			MimeType: "text/markdown",
		},
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)

	for _, r := range resp.Results {
		assert.Equal(t, "text/markdown", r.MimeType)
	}
}

func TestSearchWithPathPrefixFilter(t *testing.T) {
	manager := NewManager(DefaultConfig())

	docs := []*Document{
		{ID: "1", Title: "Doc1", Content: "NAS content", DocType: DocTypeFile, Path: "/docs/important/"},
		{ID: "2", Title: "Doc2", Content: "NAS content", DocType: DocTypeFile, Path: "/photos/vacation/"},
	}

	for _, doc := range docs {
		manager.AddDocument(doc)
	}

	query := &SearchQuery{
		Query: "NAS",
		Mode:  ModeFullText,
		Limit: 10,
		Filters: &SearchFilter{
			PathPrefix: "/docs/",
		},
	}

	resp, err := manager.Search(query)
	assert.NoError(t, err)

	for _, r := range resp.Results {
		assert.True(t, len(r.Path) >= 6 && r.Path[:6] == "/docs/")
	}
}
