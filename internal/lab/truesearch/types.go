// Package truesearch 提供高性能全文搜索引擎功能，支持十亿级文件的亚秒级搜索。
// 实现倒排索引、增量更新、文件名+内容全文检索、搜索结果高亮和预览。
package truesearch

import (
	"sync"
	"time"
)

// IndexStatus 索引状态.
type IndexStatus string

const (
	IndexStatusPending  IndexStatus = "pending"
	IndexStatusIndexing IndexStatus = "indexing"
	IndexStatusReady    IndexStatus = "ready"
	IndexStatusError    IndexStatus = "error"
	IndexStatusUpdating IndexStatus = "updating"
)

// FileType 文件类型.
type FileType string

const (
	FileTypeDocument FileType = "document"
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeCode     FileType = "code"
	FileTypeArchive  FileType = "archive"
	FileTypeOther    FileType = "other"
)

// SearchMode 搜索模式.
type SearchMode string

const (
	SearchModeFilename SearchMode = "filename"
	SearchModeContent  SearchMode = "content"
	SearchModeAll      SearchMode = "all"
)

// SortOrder 排序方式.
type SortOrder string

const (
	SortByRelevance SortOrder = "relevance"
	SortByDate      SortOrder = "date"
	SortBySize      SortOrder = "size"
	SortByName      SortOrder = "name"
)

// Document 索引文档.
type Document struct {
	ID        string            `json:"id"`
	Path      string            `json:"path"`
	Name      string            `json:"name"`
	Extension string            `json:"extension"`
	Size      int64             `json:"size"`
	FileType  FileType          `json:"file_type"`
	MimeType  string            `json:"mime_type,omitempty"`
	Content   string            `json:"content,omitempty"` // 文件内容（用于索引）
	Checksum  string            `json:"checksum,omitempty"`
	ModTime   time.Time         `json:"mod_time"`
	IndexTime time.Time         `json:"index_time"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// IndexEntry 倒排索引条目.
type IndexEntry struct {
	Term      string   `json:"term"`
	DocIDs    []string `json:"doc_ids"`
	Frequency int      `json:"frequency"` // 文档频率
}

// Posting 倒排列表项.
type Posting struct {
	DocID     string  `json:"doc_id"`
	TermFreq  int     `json:"term_freq"` // 词频
	Positions []int   `json:"positions"` // 词位置
	Score     float64 `json:"score"`     // TF-IDF 分数
}

// SearchResult 搜索结果.
type SearchResult struct {
	DocID          string            `json:"doc_id"`
	Path           string            `json:"path"`
	Name           string            `json:"name"`
	Extension      string            `json:"extension"`
	Size           int64             `json:"size"`
	FileType       FileType          `json:"file_type"`
	MimeType       string            `json:"mime_type,omitempty"`
	ModTime        time.Time         `json:"mod_time"`
	Score          float64           `json:"score"`
	FilenameMatch  bool              `json:"filename_match"`
	ContentMatch   bool              `json:"content_match"`
	HighlightName  string            `json:"highlight_name,omitempty"` // 高亮文件名
	HighlightSnip  string            `json:"highlight_snip,omitempty"` // 内容摘要高亮
	MatchPositions []MatchPosition   `json:"match_positions,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// MatchPosition 匹配位置.
type MatchPosition struct {
	Field string `json:"field"` // "filename" 或 "content"
	Start int    `json:"start"`
	End   int    `json:"end"`
	Term  string `json:"term"`
}

// SearchQuery 搜索查询.
type SearchQuery struct {
	Query     string     `json:"query" binding:"required"`
	Mode      SearchMode `json:"mode,omitempty"`
	Sort      SortOrder  `json:"sort,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
	FileTypes []FileType `json:"file_types,omitempty"`
	Path      string     `json:"path,omitempty"`  // 限定搜索路径
	After     *time.Time `json:"after,omitempty"` // 修改时间过滤
	Before    *time.Time `json:"before,omitempty"`
	MinSize   *int64     `json:"min_size,omitempty"`
	MaxSize   *int64     `json:"max_size,omitempty"`
}

// SearchResponse 搜索响应.
type SearchResponse struct {
	Query      string         `json:"query"`
	TotalHits  int            `json:"total_hits"`
	Results    []SearchResult `json:"results"`
	TookMs     int64          `json:"took_ms"`
	Suggestion string         `json:"suggestion,omitempty"`
}

// IndexStats 索引统计.
type IndexStats struct {
	TotalDocuments int64         `json:"total_documents"`
	TotalTerms     int64         `json:"total_terms"`
	IndexSize      int64         `json:"index_size_bytes"`
	LastUpdateTime time.Time     `json:"last_update_time"`
	Status         IndexStatus   `json:"status"`
	IndexDuration  time.Duration `json:"index_duration"`
}

// TrueSearchConfig 搜索引擎配置.
type TrueSearchConfig struct {
	Enabled         bool   `json:"enabled"`
	IndexDir        string `json:"index_dir"`        // SSD 索引存储目录
	MaxContentLen   int    `json:"max_content_len"`  // 索引最大内容长度
	MinTermLen      int    `json:"min_term_len"`     // 最小词长度
	MaxResults      int    `json:"max_results"`      // 单次最大结果数
	SnippetLen      int    `json:"snippet_len"`      // 摘要长度
	HighlightTag    string `json:"highlight_tag"`    // 高亮标签
	Workers         int    `json:"workers"`          // 并发索引 worker 数
	BatchSize       int    `json:"batch_size"`       // 批量索引大小
	IncrementalMode bool   `json:"incremental_mode"` // 增量更新模式
}

// DefaultTrueSearchConfig 默认配置.
func DefaultTrueSearchConfig() *TrueSearchConfig {
	return &TrueSearchConfig{
		Enabled:         true,
		IndexDir:        "/var/lib/nas-os/truesearch",
		MaxContentLen:   1024 * 1024, // 1MB
		MinTermLen:      2,
		MaxResults:      100,
		SnippetLen:      200,
		HighlightTag:    "em",
		Workers:         4,
		BatchSize:       1000,
		IncrementalMode: true,
	}
}

// invertedIndex 倒排索引核心结构.
type invertedIndex struct {
	mu        sync.RWMutex
	index     map[string][]*Posting // term -> postings
	docs      map[string]*Document  // docID -> document
	docCount  int64
	termCount int64
}

// newInvertedIndex 创建倒排索引.
func newInvertedIndex() *invertedIndex {
	return &invertedIndex{
		index: make(map[string][]*Posting),
		docs:  make(map[string]*Document),
	}
}
