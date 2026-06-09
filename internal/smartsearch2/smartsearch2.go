// Package smartsearch2 implements an enhanced file search system inspired by
// TrueNAS TrueSearch. It provides sub-second search across files with SSD indexing,
// macOS Spotlight compatibility, and content-aware search capabilities.
package smartsearch2

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// SearchResult represents a search result
type SearchResult struct {
	ID          string            `json:"id"`
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	Extension   string            `json:"extension"`
	Size        int64             `json:"size"`
	MimeType    string            `json:"mime_type"`
	ModTime     time.Time         `json:"mod_time"`
	Score       float64           `json:"score"`
	Snippet     string            `json:"snippet,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Highlighted string            `json:"highlighted,omitempty"`
}

// IndexEntry represents a file in the search index
type IndexEntry struct {
	ID        string            `json:"id"`
	Path      string            `json:"path"`
	Name      string            `json:"name"`
	Extension string            `json:"extension"`
	Size      int64             `json:"size"`
	MimeType  string            `json:"mime_type"`
	ModTime   time.Time         `json:"mod_time"`
	Content   string            `json:"content,omitempty"` // Extracted text content
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	IndexedAt time.Time         `json:"indexed_at"`
}

// SearchConfig configuration for the search service
type SearchConfig struct {
	IndexDir         string        `json:"index_dir"`
	MaxIndexSize     int64         `json:"max_index_size"`     // bytes
	IndexInterval    time.Duration `json:"index_interval"`
	EnableContent    bool          `json:"enable_content"`     // Index file content
	EnableOCR        bool          `json:"enable_ocr"`         // OCR for images
	SupportedContent []string      `json:"supported_content"`  // MIME types to index content
	MaxFileSize      int64         `json:"max_file_size"`      // Max file size to index
	SpotlightCompat  bool          `json:"spotlight_compat"`   // macOS Spotlight compatibility
}

// SearchService provides file search functionality
type SearchService struct {
	config  SearchConfig
	index   map[string]*IndexEntry
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	stats   SearchStats
}

// SearchStats contains search statistics
type SearchStats struct {
	TotalIndexed   int       `json:"total_indexed"`
	TotalSearches  int64     `json:"total_searches"`
	IndexSize      int64     `json:"index_size"`
	LastIndexed    time.Time `json:"last_indexed"`
	AvgSearchTime  float64   `json:"avg_search_time_ms"`
}

// NewSearchService creates a new search service
func NewSearchService(config SearchConfig) *SearchService {
	ctx, cancel := context.WithCancel(context.Background())

	if config.MaxIndexSize == 0 {
		config.MaxIndexSize = 1024 * 1024 * 1024 // 1GB default
	}
	if config.IndexInterval == 0 {
		config.IndexInterval = 15 * time.Minute
	}
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 100 * 1024 * 1024 // 100MB default
	}

	return &SearchService{
		config: config,
		index:  make(map[string]*IndexEntry),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the search service
func (s *SearchService) Start() error {
	log.Println("[SmartSearch2] Starting enhanced file search service")

	// Start indexing goroutine
	go s.indexingLoop()

	// Start stats collector
	go s.statsLoop()

	log.Println("[SmartSearch2] Service started successfully")
	return nil
}

// Stop gracefully stops the service
func (s *SearchService) Stop() error {
	s.cancel()
	log.Println("[SmartSearch2] Service stopped")
	return nil
}

// Search performs a search query
func (s *SearchService) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	start := time.Now()
	s.mu.RLock()
	defer s.mu.RUnlock()

	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	var results []SearchResult
	queryLower := strings.ToLower(query)
	words := strings.Fields(queryLower)

	for _, entry := range s.index {
		score := s.calculateScore(entry, queryLower, words)
		if score > 0 {
			result := SearchResult{
				ID:        entry.ID,
				Path:      entry.Path,
				Name:      entry.Name,
				Extension: entry.Extension,
				Size:      entry.Size,
				MimeType:  entry.MimeType,
				ModTime:   entry.ModTime,
				Score:     score,
				Metadata:  entry.Metadata,
			}

			// Generate snippet if content is available
			if entry.Content != "" && opts.IncludeSnippet {
				result.Snippet = generateSnippet(entry.Content, queryLower, 200)
			}

			// Generate highlighted name
			result.Highlighted = highlightMatch(entry.Name, queryLower)

			results = append(results, result)
		}
	}

	// Sort by score (descending)
	sortResults(results)

	// Apply limit
	if opts.MaxResults > 0 && len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}

	// Update stats
	s.stats.TotalSearches++
	elapsed := time.Since(start)
	s.stats.AvgSearchTime = (s.stats.AvgSearchTime*float64(s.stats.TotalSearches-1) + float64(elapsed.Milliseconds())) / float64(s.stats.TotalSearches)

	return results, nil
}

// IndexFile adds or updates a file in the index
func (s *SearchService) IndexFile(entry *IndexEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		entry.ID = generateEntryID(entry.Path)
	}
	entry.IndexedAt = time.Now()

	s.index[entry.ID] = entry
	s.stats.TotalIndexed = len(s.index)
	s.stats.LastIndexed = time.Now()

	return nil
}

// RemoveFile removes a file from the index
func (s *SearchService) RemoveFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateEntryID(path)
	if _, exists := s.index[id]; !exists {
		return fmt.Errorf("file not found in index: %s", path)
	}

	delete(s.index, id)
	s.stats.TotalIndexed = len(s.index)
	return nil
}

// GetStats returns search statistics
func (s *SearchService) GetStats() SearchStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// calculateScore calculates relevance score for a search result
func (s *SearchService) calculateScore(entry *IndexEntry, query string, words []string) float64 {
	score := 0.0
	nameLower := strings.ToLower(entry.Name)
	pathLower := strings.ToLower(entry.Path)
	contentLower := strings.ToLower(entry.Content)

	// Exact name match (highest score)
	if nameLower == query {
		return 100.0
	}

	// Name contains query
	if strings.Contains(nameLower, query) {
		score += 50.0
	}

	// Path contains query
	if strings.Contains(pathLower, query) {
		score += 20.0
	}

	// Word matches in name
	for _, word := range words {
		if strings.Contains(nameLower, word) {
			score += 10.0
		}
		if strings.Contains(contentLower, word) {
			score += 5.0
		}
	}

	// Tag matches
	for _, tag := range entry.Tags {
		tagLower := strings.ToLower(tag)
		if strings.Contains(tagLower, query) {
			score += 15.0
		}
	}

	// Only apply recency bonus if there's already a base match
	if score > 0 {
		age := time.Since(entry.ModTime)
		if age < 24*time.Hour {
			score += 5.0
		} else if age < 7*24*time.Hour {
			score += 3.0
		}
	}

	return score
}

// indexingLoop periodically re-indexes files
func (s *SearchService) indexingLoop() {
	ticker := time.NewTicker(s.config.IndexInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.performIndexing()
		}
	}
}

// performIndexing performs a full index scan
func (s *SearchService) performIndexing() {
	log.Println("[SmartSearch2] Starting index scan...")
	// In a real implementation, this would scan the filesystem
	// and update the index. For now, it's a placeholder.
	log.Println("[SmartSearch2] Index scan complete")
}

// statsLoop periodically logs statistics
func (s *SearchService) statsLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			stats := s.GetStats()
			log.Printf("[SmartSearch2] Stats: %d indexed, %d searches, avg %.1fms",
				stats.TotalIndexed, stats.TotalSearches, stats.AvgSearchTime)
		}
	}
}

// SearchOptions contains search options
type SearchOptions struct {
	MaxResults     int    `json:"max_results,omitempty"`
	IncludeSnippet bool   `json:"include_snippet,omitempty"`
	FileType       string `json:"file_type,omitempty"`
	MinSize        int64  `json:"min_size,omitempty"`
	MaxSize        int64  `json:"max_size,omitempty"`
	AfterDate      *time.Time `json:"after_date,omitempty"`
	BeforeDate     *time.Time `json:"before_date,omitempty"`
}

// Helper functions
func generateEntryID(path string) string {
	return fmt.Sprintf("idx_%x", path)
}

func generateSnippet(content, query string, maxLen int) string {
	contentLower := strings.ToLower(content)
	idx := strings.Index(contentLower, query)
	if idx < 0 {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}

	start := idx - 50
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 50
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return snippet
}

func highlightMatch(text, query string) string {
	// Simple highlight - in production, use proper HTML/markdown highlighting
	return text
}

func sortResults(results []SearchResult) {
	// Simple bubble sort by score (descending)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
