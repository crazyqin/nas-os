// Package clip provides CLIP model integration for text-to-image search
// Implements natural language photo search using CLIP (Contrastive Language-Image Pre-training)
// Inspired by fnOS AI photo album and Synology Photos
package clip

import (
	"context"
	"time"
)

// Config holds CLIP service configuration
type Config struct {
	// Model settings
	ModelName    string `json:"model_name"`     // "clip-vit-base-32", "clip-vit-large-14", etc.
	ModelPath    string `json:"model_path"`     // path to model weights
	ImageSize    int    `json:"image_size"`     // input image size (224 or 336)
	EmbeddingDim int    `json:"embedding_dim"`  // embedding dimension (512 or 768)
	UseGPU       bool   `json:"use_gpu"`        // use GPU acceleration

	// Index settings
	IndexType     string `json:"index_type"`     // "flat", "ivf", "hnsw"
	IndexPath     string `json:"index_path"`     // path to persist index
	IndexCapacity int    `json:"index_capacity"` // max vectors in index

	// Search settings
	DefaultTopK    int     `json:"default_top_k"`    // default number of results
	MinSimilarity  float64 `json:"min_similarity"`   // minimum similarity threshold
	BatchSize      int     `json:"batch_size"`       // batch size for processing
	MaxTextLength  int     `json:"max_text_length"`  // max text query length
	CacheSize      int     `json:"cache_size"`       // text embedding cache size
	CacheTTL       int     `json:"cache_ttl"`        // cache TTL in seconds

	// Performance settings
	NumWorkers     int `json:"num_workers"`     // worker goroutines for indexing
	QueueSize      int `json:"queue_size"`      // indexing queue size
	EnablePrefetch bool `json:"enable_prefetch"` // prefetch embeddings
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		ModelName:      "clip-vit-base-32",
		ImageSize:      224,
		EmbeddingDim:   512,
		UseGPU:         false,
		IndexType:      "flat",
		IndexCapacity:  100000,
		DefaultTopK:    20,
		MinSimilarity:  0.2,
		BatchSize:      32,
		MaxTextLength:  200,
		CacheSize:      1000,
		CacheTTL:       3600,
		NumWorkers:     4,
		QueueSize:      1000,
		EnablePrefetch: true,
	}
}

// Embedding represents a feature vector
type Embedding struct {
	Vector    []float32 `json:"vector"`
	Dim       int       `json:"dim"`
	ModelName string    `json:"model_name"`
	Norm      float64   `json:"norm"` // L2 norm
}

// ImageEmbedding represents an image feature vector with metadata
type ImageEmbedding struct {
	ID          string    `json:"id"`
	PhotoID     string    `json:"photo_id"`
	Path        string    `json:"path"`
	Embedding   Embedding `json:"embedding"`
	Tags        []string  `json:"tags,omitempty"`        // extracted tags
	Caption     string    `json:"caption,omitempty"`     // auto-generated caption
	IndexedAt   time.Time `json:"indexed_at"`
	ProcessedAt time.Time `json:"processed_at"`
}

// TextEmbedding represents a text feature vector
type TextEmbedding struct {
	Query     string    `json:"query"`
	Embedding Embedding `json:"embedding"`
	CreatedAt time.Time `json:"created_at"`
}

// SearchResult represents a search result
type SearchResult struct {
	PhotoID    string    `json:"photo_id"`
	Path       string    `json:"path"`
	Score      float64   `json:"score"`       // similarity score 0-1
	Rank       int       `json:"rank"`        // result rank
	MatchType  MatchType `json:"match_type"`  // how the match was found
	Highlights []string  `json:"highlights,omitempty"` // matched tags/captions
}

// MatchType indicates how a search match was found
type MatchType string

const (
	MatchTypeSemantic MatchType = "semantic" // CLIP semantic match
	MatchTypeTag      MatchType = "tag"      // tag-based match
	MatchTypeCaption  MatchType = "caption"  // caption-based match
	MatchTypeHybrid   MatchType = "hybrid"   // combined match
)

// SearchRequest represents a search request
type SearchRequest struct {
	Query       string    `json:"query"`
	TopK        int       `json:"top_k,omitempty"`
	MinScore    float64   `json:"min_score,omitempty"`
	Filters     *Filters  `json:"filters,omitempty"`
	SortBy      string    `json:"sort_by,omitempty"`   // "relevance", "date", "quality"
	SortDesc    bool      `json:"sort_desc"`
	IncludeTags bool      `json:"include_tags"`
}

// Filters for search refinement
type Filters struct {
	DateFrom   *time.Time `json:"date_from,omitempty"`
	DateTo     *time.Time `json:"date_to,omitempty"`
	Categories []string   `json:"categories,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	MinWidth   int        `json:"min_width,omitempty"`
	MinHeight  int        `json:"min_height,omitempty"`
	Formats    []string   `json:"formats,omitempty"`
}

// SearchResponse contains search results
type SearchResponse struct {
	Query       string         `json:"query"`
	Results     []SearchResult `json:"results"`
	Total       int            `json:"total"`
	QueryTime   int64          `json:"query_time_ms"`
	Embedding   Embedding      `json:"embedding,omitempty"` // query embedding
	Suggestions []string       `json:"suggestions,omitempty"` // query suggestions
}

// IndexRequest represents an indexing request
type IndexRequest struct {
	PhotoID   string   `json:"photo_id"`
	Path      string   `json:"path"`
	Tags      []string `json:"tags,omitempty"`
	Caption   string   `json:"caption,omitempty"`
	Priority  int      `json:"priority,omitempty"` // higher = more urgent
}

// IndexResponse represents indexing result
type IndexResponse struct {
	PhotoID     string        `json:"photo_id"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	ProcessTime time.Duration `json:"process_time"`
}

// BatchIndexRequest for batch indexing
type BatchIndexRequest struct {
	Photos []IndexRequest `json:"photos"`
}

// BatchIndexResponse for batch indexing results
type BatchIndexResponse struct {
	Total       int            `json:"total"`
	Success     int            `json:"success"`
	Failed      int            `json:"failed"`
	Results     []IndexResponse `json:"results"`
	ProcessTime time.Duration  `json:"process_time"`
}

// Stats holds service statistics
type Stats struct {
	TotalImages    int64         `json:"total_images"`
	TotalIndexed   int64         `json:"total_indexed"`
	IndexSize      int64         `json:"index_size_bytes"`
	CacheSize      int           `json:"cache_size"`
	CacheHitRate   float64       `json:"cache_hit_rate"`
	AvgSearchTime  time.Duration `json:"avg_search_time_ms"`
	TotalSearches  int64         `json:"total_searches"`
	ModelInfo      ModelInfo     `json:"model_info"`
}

// ModelInfo contains model information
type ModelInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	ImageSize   int    `json:"image_size"`
	EmbeddingDim int   `json:"embedding_dim"`
	GPUEnabled  bool   `json:"gpu_enabled"`
}

// IndexStats holds index statistics
type IndexStats struct {
	TotalVectors  int64     `json:"total_vectors"`
	IndexSize     int64     `json:"index_size_bytes"`
	LastUpdated   time.Time `json:"last_updated"`
	IndexType     string    `json:"index_type"`
	Capacity      int       `json:"capacity"`
	Utilization   float64   `json:"utilization"` // % of capacity used
}

// CLIPModel defines the interface for CLIP models
type CLIPModel interface {
	// EncodeImage generates embedding for an image
	EncodeImage(ctx context.Context, imagePath string) (*Embedding, error)

	// EncodeImageFromBytes generates embedding from image bytes
	EncodeImageFromBytes(ctx context.Context, data []byte) (*Embedding, error)

	// EncodeText generates embedding for text
	EncodeText(ctx context.Context, text string) (*Embedding, error)

	// BatchEncodeImages encodes multiple images in parallel
	BatchEncodeImages(ctx context.Context, paths []string) ([]*Embedding, error)

	// BatchEncodeTexts encodes multiple texts in parallel
	BatchEncodeTexts(ctx context.Context, texts []string) ([]*Embedding, error)

	// GetModelInfo returns model information
	GetModelInfo() ModelInfo

	// Close releases model resources
	Close() error
}

// VectorIndex defines the interface for vector storage and search
type VectorIndex interface {
	// Add adds a vector to the index
	Add(ctx context.Context, id string, vector []float32, metadata map[string]any) error

	// BatchAdd adds multiple vectors
	BatchAdd(ctx context.Context, ids []string, vectors [][]float32, metadata []map[string]any) error

	// Search finds nearest neighbors
	Search(ctx context.Context, query []float32, topK int, minScore float64) ([]SearchResult, error)

	// Delete removes a vector from the index
	Delete(ctx context.Context, id string) error

	// Get retrieves a vector by ID
	Get(ctx context.Context, id string) (*ImageEmbedding, bool)

	// Size returns the number of vectors in the index
	Size() int64

	// Clear removes all vectors
	Clear(ctx context.Context) error

	// Save persists the index to disk
	Save(ctx context.Context, path string) error

	// Load loads the index from disk
	Load(ctx context.Context, path string) error

	// GetStats returns index statistics
	GetStats() IndexStats
}

// TextSearchService defines the text-to-image search service interface
type TextSearchService interface {
	// Search performs text-to-image search
	Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)

	// Index indexes a photo
	Index(ctx context.Context, req *IndexRequest) (*IndexResponse, error)

	// BatchIndex indexes multiple photos
	BatchIndex(ctx context.Context, req *BatchIndexRequest) (*BatchIndexResponse, error)

	// Remove removes a photo from the index
	Remove(ctx context.Context, photoID string) error

	// GetStats returns service statistics
	GetStats(ctx context.Context) (*Stats, error)

	// Close shuts down the service
	Close() error
}

// Errors
var (
	ErrModelNotLoaded    = &CLIPError{Code: "model_not_loaded", Message: "CLIP model not loaded"}
	ErrIndexFull         = &CLIPError{Code: "index_full", Message: "Index capacity exceeded"}
	ErrInvalidInput      = &CLIPError{Code: "invalid_input", Message: "Invalid input"}
	ErrImageNotFound     = &CLIPError{Code: "image_not_found", Message: "Image file not found"}
	ErrEncodingFailed    = &CLIPError{Code: "encoding_failed", Message: "Failed to encode input"}
	ErrSearchFailed      = &CLIPError{Code: "search_failed", Message: "Search failed"}
	ErrIndexCorrupted    = &CLIPError{Code: "index_corrupted", Message: "Index corrupted"}
)

// CLIPError represents a CLIP service error
type CLIPError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *CLIPError) Error() string {
	return e.Message
}

// IsCLIPError checks if error is a CLIPError
func IsCLIPError(err error) bool {
	_, ok := err.(*CLIPError)
	return ok
}