// Package ragsearch provides RAG-enhanced unified search for NAS-OS
// Features: Document indexing, BM25 full-text search, vector semantic search,
// hybrid RRF fusion ranking, search filters, suggestions, and search history
// Competitor benchmark: 对标群晖Universal Search + RAG增强, 超越飞牛/TrueNAS搜索能力
package ragsearch

import (
	"time"
)

// DocumentType defines the type of document
type DocumentType string

const (
	DocTypeFile    DocumentType = "file"
	DocTypeNote    DocumentType = "note"
	DocTypeEmail   DocumentType = "email"
	DocTypePhoto   DocumentType = "photo"
	DocTypeVideo   DocumentType = "video"
	DocTypeAudio   DocumentType = "audio"
	DocTypePDF     DocumentType = "pdf"
	DocTypeDoc     DocumentType = "doc"
	DocTypeCode    DocumentType = "code"
	DocTypeArchive DocumentType = "archive"
)

// SearchMode defines search algorithm
type SearchMode string

const (
	ModeFullText SearchMode = "fulltext" // BM25 full-text search
	ModeSemantic SearchMode = "semantic" // Vector semantic search
	ModeHybrid   SearchMode = "hybrid"   // Combined fulltext + semantic with RRF
	ModeAuto     SearchMode = "auto"     // Auto-select best mode
)

// SortOrder defines result sorting
type SortOrder string

const (
	SortRelevance SortOrder = "relevance"
	SortDate      SortOrder = "date"
	SortName      SortOrder = "name"
	SortSize      SortOrder = "size"
	SortScore     SortOrder = "score"
)

// SearchQuery represents a search query with filters
type SearchQuery struct {
	Query     string        `json:"query"`
	Mode      SearchMode    `json:"mode"`
	Sort      SortOrder     `json:"sort"`
	Limit     int           `json:"limit"`
	Offset    int           `json:"offset"`
	Filters   *SearchFilter `json:"filters,omitempty"`
	Highlight bool          `json:"highlight"`
	Facets    bool          `json:"facets"`
}

// SearchFilter defines filters for search queries
type SearchFilter struct {
	DocTypes   []DocumentType `json:"doc_types,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	DateFrom   *time.Time     `json:"date_from,omitempty"`
	DateTo     *time.Time     `json:"date_to,omitempty"`
	SizeMin    *int64         `json:"size_min,omitempty"`
	SizeMax    *int64         `json:"size_max,omitempty"`
	Sources    []string       `json:"sources,omitempty"`
	MimeType   string         `json:"mime_type,omitempty"`
	PathPrefix string         `json:"path_prefix,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID         string                 `json:"id"`
	Title      string                 `json:"title"`
	Content    string                 `json:"content"`
	Path       string                 `json:"path"`
	DocType    DocumentType           `json:"doc_type"`
	MimeType   string                 `json:"mime_type"`
	Size       int64                  `json:"size"`
	Score      float64                `json:"score"`
	RankScore  *RankScore             `json:"rank_score,omitempty"`
	Highlight  string                 `json:"highlight,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Source     string                 `json:"source"`
	CreatedAt  time.Time              `json:"created_at"`
	ModifiedAt time.Time              `json:"modified_at"`
}

// RankScore represents detailed ranking scores
type RankScore struct {
	BM25Score      float64 `json:"bm25_score"`
	SemanticScore  float64 `json:"semantic_score"`
	RRFScore       float64 `json:"rrf_score"`
	FreshnessScore float64 `json:"freshness_score"`
	CombinedScore  float64 `json:"combined_score"`
}

// SearchResponse represents search results
type SearchResponse struct {
	Query       string          `json:"query"`
	TotalHits   int             `json:"total_hits"`
	Results     []*SearchResult `json:"results"`
	Facets      map[string]int  `json:"facets,omitempty"`
	Suggestions []string        `json:"suggestions,omitempty"`
	SearchTime  int64           `json:"search_time_ms"`
	Limit       int             `json:"limit"`
	Offset      int             `json:"offset"`
	HasMore     bool            `json:"has_more"`
}

// Document represents a document to be indexed
type Document struct {
	ID         string                 `json:"id"`
	Title      string                 `json:"title"`
	Content    string                 `json:"content"`
	Path       string                 `json:"path"`
	DocType    DocumentType           `json:"doc_type"`
	MimeType   string                 `json:"mime_type"`
	Size       int64                  `json:"size"`
	Tags       []string               `json:"tags,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Source     string                 `json:"source"`
	CreatedAt  time.Time              `json:"created_at"`
	ModifiedAt time.Time              `json:"modified_at"`
}

// IndexEntry represents an indexed document with computed fields
type IndexEntry struct {
	Document
	IndexedAt     time.Time      `json:"indexed_at"`
	Tokens        []string       `json:"tokens,omitempty"`
	TermFreq      map[string]int `json:"term_freq,omitempty"`
	Embedding     []float64      `json:"embedding,omitempty"`
	ContentLength int            `json:"content_length"`
}

// IndexStats represents index statistics
type IndexStats struct {
	TotalEntries    int64            `json:"total_entries"`
	EntriesByType   map[string]int64 `json:"entries_by_type"`
	IndexSize       int64            `json:"index_size_bytes"`
	LastIndexed     time.Time        `json:"last_indexed"`
	IndexHealth     string           `json:"index_health"`
	VectorDimension int              `json:"vector_dimension"`
}

// SearchHistory represents a search history entry
type SearchHistory struct {
	Query     string    `json:"query"`
	Timestamp time.Time `json:"timestamp"`
	HitCount  int       `json:"hit_count"`
}

// SuggestionRequest represents a suggestion/autocomplete request
type SuggestionRequest struct {
	Prefix   string         `json:"prefix"`
	Limit    int            `json:"limit"`
	DocTypes []DocumentType `json:"doc_types,omitempty"`
}

// SuggestionResponse represents suggestion results
type SuggestionResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
	Query       string       `json:"query"`
}

// Suggestion represents a single suggestion
type Suggestion struct {
	Text      string  `json:"text"`
	Score     float64 `json:"score"`
	Frequency int     `json:"frequency"`
}

// HotQuery represents a popular/trending query
type HotQuery struct {
	Query    string    `json:"query"`
	Count    int       `json:"count"`
	LastUsed time.Time `json:"last_used"`
}

// Config holds RAG search configuration
type Config struct {
	Enabled         bool    `json:"enabled"`
	IndexPath       string  `json:"index_path"`
	MaxResults      int     `json:"max_results"`
	VectorDimension int     `json:"vector_dimension"`
	BM25K1          float64 `json:"bm25_k1"`
	BM25B           float64 `json:"bm25_b"`
	RRFK            int     `json:"rrf_k"`
	HistoryMaxSize  int     `json:"history_max_size"`
	SuggestionLimit int     `json:"suggestion_limit"`
	FreshnessWeight float64 `json:"freshness_weight"`
	SemanticWeight  float64 `json:"semantic_weight"`
	FullTextWeight  float64 `json:"fulltext_weight"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		IndexPath:       "/var/lib/nas-os/search-index",
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
}
