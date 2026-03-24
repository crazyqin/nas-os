// Package clip - Text-to-image search service implementation
package clip

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TextSearchServiceImpl implements TextSearchService
type TextSearchServiceImpl struct {
	config  *Config
	model   CLIPModel
	index   VectorIndex
	cache   *EmbeddingCache
	stats   atomic.Value
	running bool
	mu      sync.RWMutex
}

// NewTextSearchService creates a new text search service
func NewTextSearchService(config *Config) (*TextSearchServiceImpl, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Initialize CLIP model
	model, err := NewCLIPModel(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize CLIP model: %w", err)
	}

	// Initialize vector index
	index, err := NewVectorIndex(config)
	if err != nil {
		_ = model.Close()
		return nil, fmt.Errorf("failed to initialize vector index: %w", err)
	}

	// Initialize cache
	cache := NewEmbeddingCache(config.CacheSize, time.Duration(config.CacheTTL)*time.Second)

	service := &TextSearchServiceImpl{
		config:  config,
		model:   model,
		index:   index,
		cache:   cache,
		running: true,
	}

	// Load existing index if path specified
	if config.IndexPath != "" {
		if err := index.Load(context.Background(), config.IndexPath); err != nil {
			// Log warning but continue
			fmt.Printf("Warning: failed to load index: %v\n", err)
		}
	}

	service.updateStats()

	return service, nil
}

// NewTextSearchServiceWithModel creates service with custom model
func NewTextSearchServiceWithModel(config *Config, model CLIPModel) (*TextSearchServiceImpl, error) {
	if config == nil {
		config = DefaultConfig()
	}

	index, err := NewVectorIndex(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize vector index: %w", err)
	}

	cache := NewEmbeddingCache(config.CacheSize, time.Duration(config.CacheTTL)*time.Second)

	service := &TextSearchServiceImpl{
		config:  config,
		model:   model,
		index:   index,
		cache:   cache,
		running: true,
	}

	// Load existing index if path specified
	if config.IndexPath != "" {
		if err := index.Load(context.Background(), config.IndexPath); err != nil {
			// Log warning but continue
			fmt.Printf("Warning: failed to load index: %v\n", err)
		}
	}

	service.updateStats()

	return service, nil
}

// Search performs text-to-image search
func (s *TextSearchServiceImpl) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if !s.running {
		return nil, ErrModelNotLoaded
	}

	start := time.Now()

	// Set defaults
	if req.TopK <= 0 {
		req.TopK = s.config.DefaultTopK
	}
	if req.MinScore == 0 {
		req.MinScore = s.config.MinSimilarity
	}

	// Check cache first
	cachedEmbedding, hit := s.cache.GetTextEmbedding(req.Query)
	var queryEmbedding *Embedding
	var err error

	if hit {
		queryEmbedding = cachedEmbedding
	} else {
		// Encode text query
		queryEmbedding, err = s.model.EncodeText(ctx, req.Query)
		if err != nil {
			return nil, fmt.Errorf("failed to encode query: %w", err)
		}

		// Cache the embedding
		s.cache.SetTextEmbedding(req.Query, queryEmbedding)
	}

	// Search vector index
	results, err := s.index.Search(ctx, queryEmbedding.Vector, req.TopK, req.MinScore)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Apply filters if specified
	if req.Filters != nil {
		results = s.applyFilters(results, req.Filters)
	}

	// Apply sorting if specified
	if req.SortBy != "" && req.SortBy != "relevance" {
		results = s.sortResults(results, req.SortBy, req.SortDesc)
	}

	// Generate suggestions
	suggestions := s.generateSuggestions(req.Query, results)

	queryTime := time.Since(start)

	return &SearchResponse{
		Query:       req.Query,
		Results:     results,
		Total:       len(results),
		QueryTime:   queryTime.Milliseconds(),
		Embedding:   *queryEmbedding,
		Suggestions: suggestions,
	}, nil
}

// Index indexes a photo
func (s *TextSearchServiceImpl) Index(ctx context.Context, req *IndexRequest) (*IndexResponse, error) {
	if !s.running {
		return nil, ErrModelNotLoaded
	}

	start := time.Now()

	// Encode image
	embedding, err := s.model.EncodeImage(ctx, req.Path)
	if err != nil {
		return &IndexResponse{
			PhotoID: req.PhotoID,
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Prepare metadata
	metadata := map[string]any{
		"photo_id": req.PhotoID,
		"path":     req.Path,
		"tags":     req.Tags,
		"caption":  req.Caption,
	}

	// Add to index
	if err := s.index.Add(ctx, req.PhotoID, embedding.Vector, metadata); err != nil {
		return &IndexResponse{
			PhotoID: req.PhotoID,
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	s.updateStats()

	return &IndexResponse{
		PhotoID:     req.PhotoID,
		Success:     true,
		ProcessTime: time.Since(start),
	}, nil
}

// BatchIndex indexes multiple photos
func (s *TextSearchServiceImpl) BatchIndex(ctx context.Context, req *BatchIndexRequest) (*BatchIndexResponse, error) {
	if !s.running {
		return nil, ErrModelNotLoaded
	}

	start := time.Now()
	results := make([]IndexResponse, len(req.Photos))
	var success, failed int

	// Process in batches
	for i, photo := range req.Photos {
		resp, err := s.Index(ctx, &photo)
		if err != nil {
			results[i] = IndexResponse{
				PhotoID: photo.PhotoID,
				Success: false,
				Error:   err.Error(),
			}
			failed++
		} else {
			results[i] = *resp
			if resp.Success {
				success++
			} else {
				failed++
			}
		}
	}

	s.updateStats()

	return &BatchIndexResponse{
		Total:       len(req.Photos),
		Success:     success,
		Failed:      failed,
		Results:     results,
		ProcessTime: time.Since(start),
	}, nil
}

// Remove removes a photo from the index
func (s *TextSearchServiceImpl) Remove(ctx context.Context, photoID string) error {
	err := s.index.Delete(ctx, photoID)
	if err == nil {
		s.updateStats()
	}
	return err
}

// GetStats returns service statistics.
func (s *TextSearchServiceImpl) GetStats(ctx context.Context) (*Stats, error) {
	stats := s.stats.Load()
	if stats == nil {
		return &Stats{}, nil
	}
	result, ok := stats.(*Stats)
	if !ok {
		return &Stats{}, nil
	}
	return result, nil
}

// Close shuts down the service
func (s *TextSearchServiceImpl) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false

	// Save index if path specified
	if s.config.IndexPath != "" {
		if err := s.index.Save(context.Background(), s.config.IndexPath); err != nil {
			fmt.Printf("Warning: failed to save index: %v\n", err)
		}
	}

	// Close model
	if s.model != nil {
		_ = s.model.Close()
	}

	// Clear cache
	s.cache.Clear()

	return nil
}

// --- Internal Methods ---

func (s *TextSearchServiceImpl) applyFilters(results []SearchResult, filters *Filters) []SearchResult {
	// Simplified filter implementation
	// Production: integrate with actual photo metadata
	filtered := make([]SearchResult, 0)

	for _, r := range results {
		include := true

		// Date filtering would be done with actual photo metadata
		// Category filtering would need scene classification data
		// Tag filtering would need tag data

		if include {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

func (s *TextSearchServiceImpl) sortResults(results []SearchResult, sortBy string, desc bool) []SearchResult {
	// Results are already sorted by relevance
	// For other sorts, would need photo metadata
	return results
}

func (s *TextSearchServiceImpl) generateSuggestions(query string, results []SearchResult) []string {
	// Simplified suggestion generation
	// Production: use query expansion, related terms, etc.
	suggestions := make([]string, 0)

	if len(results) > 0 {
		// Could suggest based on common tags in results
		suggestions = append(suggestions, "类似搜索: "+query+" 照片")
	}

	return suggestions
}

func (s *TextSearchServiceImpl) updateStats() {
	indexStats := s.index.GetStats()
	cacheStats := s.cache.GetStats()

	stats := &Stats{
		TotalImages:   indexStats.TotalVectors,
		TotalIndexed:  indexStats.TotalVectors,
		IndexSize:     indexStats.IndexSize,
		CacheSize:     cacheStats.Size,
		CacheHitRate:  cacheStats.HitRate,
		ModelInfo:     s.model.GetModelInfo(),
		TotalSearches: cacheStats.TotalQueries,
	}

	s.stats.Store(stats)
}

// --- Embedding Cache ---

// EmbeddingCache caches text embeddings
type EmbeddingCache struct {
	textCache map[string]*cacheEntry
	imageCache map[string]*cacheEntry
	maxSize   int
	ttl       time.Duration
	stats     cacheStats
	mu        sync.RWMutex
}

type cacheEntry struct {
	embedding *Embedding
	expiresAt time.Time
}

type cacheStats struct {
	Size        int
	Hits        int64
	Misses      int64
	TotalQueries int64
}

// NewEmbeddingCache creates a new embedding cache
func NewEmbeddingCache(maxSize int, ttl time.Duration) *EmbeddingCache {
	return &EmbeddingCache{
		textCache:  make(map[string]*cacheEntry),
		imageCache: make(map[string]*cacheEntry),
		maxSize:    maxSize,
		ttl:        ttl,
	}
}

// GetTextEmbedding retrieves cached text embedding
func (c *EmbeddingCache) GetTextEmbedding(text string) (*Embedding, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.stats.TotalQueries++

	entry, exists := c.textCache[text]
	if !exists || time.Now().After(entry.expiresAt) {
		c.stats.Misses++
		return nil, false
	}

	c.stats.Hits++
	return entry.embedding, true
}

// SetTextEmbedding caches a text embedding
func (c *EmbeddingCache) SetTextEmbedding(text string, embedding *Embedding) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.textCache) >= c.maxSize {
		c.evictOldest()
	}

	c.textCache[text] = &cacheEntry{
		embedding: embedding,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.stats.Size = len(c.textCache) + len(c.imageCache)
}

// GetImageEmbedding retrieves cached image embedding
func (c *EmbeddingCache) GetImageEmbedding(imageID string) (*Embedding, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.stats.TotalQueries++

	entry, exists := c.imageCache[imageID]
	if !exists || time.Now().After(entry.expiresAt) {
		c.stats.Misses++
		return nil, false
	}

	c.stats.Hits++
	return entry.embedding, true
}

// SetImageEmbedding caches an image embedding
func (c *EmbeddingCache) SetImageEmbedding(imageID string, embedding *Embedding) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.imageCache) >= c.maxSize {
		c.evictOldest()
	}

	c.imageCache[imageID] = &cacheEntry{
		embedding: embedding,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.stats.Size = len(c.textCache) + len(c.imageCache)
}

// Clear clears the cache
func (c *EmbeddingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.textCache = make(map[string]*cacheEntry)
	c.imageCache = make(map[string]*cacheEntry)
	c.stats.Size = 0
}

// GetStats returns cache statistics
func (c *EmbeddingCache) GetStats() struct {
	Size        int
	HitRate     float64
	TotalQueries int64
} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hitRate := float64(0)
	if c.stats.TotalQueries > 0 {
		hitRate = float64(c.stats.Hits) / float64(c.stats.TotalQueries)
	}

	return struct {
		Size        int
		HitRate     float64
		TotalQueries int64
	}{
		Size:        c.stats.Size,
		HitRate:     hitRate,
		TotalQueries: c.stats.TotalQueries,
	}
}

// evictOldest removes oldest entries
func (c *EmbeddingCache) evictOldest() {
	// Simple eviction: remove expired entries first
	now := time.Now()
	for k, v := range c.textCache {
		if now.After(v.expiresAt) {
			delete(c.textCache, k)
		}
	}
	for k, v := range c.imageCache {
		if now.After(v.expiresAt) {
			delete(c.imageCache, k)
		}
	}

	// If still at capacity, remove random entries
	for len(c.textCache)+len(c.imageCache) >= c.maxSize {
		for k := range c.textCache {
			delete(c.textCache, k)
			break
		}
	}
}

// --- Query Processor ---

// QueryProcessor processes and enhances search queries
type QueryProcessor struct {
	stopWords map[string]bool
	synonyms  map[string][]string
}

// NewQueryProcessor creates a new query processor
func NewQueryProcessor() *QueryProcessor {
	qp := &QueryProcessor{
		stopWords: make(map[string]bool),
		synonyms:  make(map[string][]string),
	}

	// Common stop words
	stopWords := []string{"a", "an", "the", "is", "are", "was", "were", "be", "been", "being", "have", "has", "had", "do", "does", "did", "will", "would", "could", "should", "of", "in", "on", "at", "by", "for", "with", "about", "to", "from"}
	for _, w := range stopWords {
		qp.stopWords[w] = true
	}

	// Synonyms for common photo queries
	qp.synonyms["photo"] = []string{"picture", "image", "shot"}
	qp.synonyms["picture"] = []string{"photo", "image"}
	qp.synonyms["dog"] = []string{"puppy", "canine"}
	qp.synonyms["cat"] = []string{"kitten", "feline"}
	qp.synonyms["baby"] = []string{"infant", "toddler"}

	return qp
}

// Process processes a search query
func (qp *QueryProcessor) Process(query string) string {
	// Lowercase
	query = toLower(query)

	// Remove stop words
	words := splitWords(query)
	filtered := make([]string, 0)
	for _, w := range words {
		if !qp.stopWords[w] {
			filtered = append(filtered, w)
		}
	}

	// Join back
	result := ""
	for i, w := range filtered {
		if i > 0 {
			result += " "
		}
		result += w
	}

	return result
}

// Expand expands a query with synonyms
func (qp *QueryProcessor) Expand(query string) []string {
	words := splitWords(query)
	expanded := []string{query}

	for _, w := range words {
		if syns, ok := qp.synonyms[w]; ok {
			for _, syn := range syns {
				newQuery := ""
				for _, word := range words {
					if word == w {
						newQuery += syn + " "
					} else {
						newQuery += word + " "
					}
				}
				expanded = append(expanded, newQuery[:len(newQuery)-1])
			}
		}
	}

	return expanded
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			result[i] = byte(c + 32)
		} else {
			result[i] = byte(c)
		}
	}
	return string(result)
}