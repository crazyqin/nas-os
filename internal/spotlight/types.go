// Package spotlight 提供全文搜索引擎功能
package spotlight

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// FileType 文件类型
type FileType string

const (
	FileTypeDocument FileType = "document"
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeArchive  FileType = "archive"
	FileTypeCode     FileType = "code"
	FileTypeOther    FileType = "other"
)

// Document 索引文档
type Document struct {
	ID        string            `json:"id"`
	Path      string            `json:"path"`
	Name      string            `json:"name"`
	Extension string            `json:"extension"`
	Size      int64             `json:"size"`
	MimeType  string            `json:"mime_type"`
	FileType  FileType          `json:"file_type"`
	Content   string            `json:"content,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	IndexedAt time.Time         `json:"indexed_at"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query      string     `json:"query" form:"q"`
	Path       string     `json:"path" form:"path"`
	FileType   FileType   `json:"file_type" form:"file_type"`
	Extensions []string   `json:"extensions" form:"extensions"`
	Tags       []string   `json:"tags" form:"tags"`
	MinSize    *int64     `json:"min_size" form:"min_size"`
	MaxSize    *int64     `json:"max_size" form:"max_size"`
	After      *time.Time `json:"after" form:"after"`
	Before     *time.Time `json:"before" form:"before"`
	Page       int        `json:"page" form:"page"`
	PageSize   int        `json:"page_size" form:"page_size"`
	SortBy     string     `json:"sort_by" form:"sort_by"`       // relevance, date, size, name
	SortOrder  string     `json:"sort_order" form:"sort_order"` // asc, desc
}

// SearchResult 搜索结果
type SearchResult struct {
	Documents  []ScoredDocument `json:"documents"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
	Query      string           `json:"query"`
	Duration   string           `json:"duration"`
	Suggestions []string        `json:"suggestions,omitempty"`
}

// ScoredDocument 带评分的文档
type ScoredDocument struct {
	Document
	Score       float64  `json:"score"`
	Highlights  []string `json:"highlights,omitempty"`
	MatchReason string   `json:"match_reason,omitempty"`
}

// SuggestRequest 搜索建议请求
type SuggestRequest struct {
	Query    string `json:"query" form:"q"`
	Limit    int    `json:"limit" form:"limit"`
}

// SuggestResponse 搜索建议响应
type SuggestResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
	Query       string       `json:"query"`
}

// Suggestion 单个搜索建议
type Suggestion struct {
	Text        string  `json:"text"`
	Score       float64 `json:"score"`
	Type        string  `json:"type"` // completion, correction, related
	Description string  `json:"description,omitempty"`
}

// IndexStats 索引统计
type IndexStats struct {
	TotalDocuments  int       `json:"total_documents"`
	TotalTerms      int       `json:"total_terms"`
	IndexSize       int64     `json:"index_size"`
	LastIndexedAt   time.Time `json:"last_indexed_at"`
	IndexDuration   string    `json:"index_duration"`
	DocumentsByType map[FileType]int `json:"documents_by_type"`
}

// invertedIndex 倒排索引
type invertedIndex struct {
	mu       sync.RWMutex
	index    map[string]map[string]positions // term -> docID -> positions
	docs     map[string]*Document            // docID -> Document
	trieRoot *trieNode                        // 前缀树根节点
}

// positions 词项在文档中的位置
type positions struct {
	Fields    []string `json:"fields"`    // 出现的字段 (name, content, tags)
	Count     int      `json:"count"`     // 出现次数
	Positions []int    `json:"positions"` // 在content中的位置
}

// trieNode 前缀树节点
type trieNode struct {
	children map[rune]*trieNode
	isEnd    bool
	terms    []string // 以该节点结尾的所有词项
	count    int      // 词频
}

// Manager 管理器
type Manager struct {
	logger    *zap.Logger
	index     *invertedIndex
	tokenizer *tokenizer
	config    *SpotlightConfig
	stopCh    chan struct{}
}

// Handlers HTTP 处理器
type Handlers struct {
	logger *zap.Logger
	mgr    *Manager
}

// SpotlightConfig 配置
type SpotlightConfig struct {
	MaxIndexSize     int    `json:"max_index_size"`
	MinTermLength    int    `json:"min_term_length"`
	MaxTermLength    int    `json:"max_term_length"`
	EnableCJK       bool   `json:"enable_cjk"`
	EnableStemming   bool   `json:"enable_stemming"`
	IndexBatchSize   int    `json:"index_batch_size"`
	SearchTimeout    int    `json:"search_timeout_ms"`
	MaxResults       int    `json:"max_results"`
	SuggestionLimit  int    `json:"suggestion_limit"`
}

// tokenizer 分词器
type tokenizer struct {
	stopWords map[string]bool
}

// newIndex 创建新倒排索引
func newIndex() *invertedIndex {
	return &invertedIndex{
		index:    make(map[string]map[string]positions),
		docs:     make(map[string]*Document),
		trieRoot: newTrieNode(),
	}
}

// newTrieNode 创建新前缀树节点
func newTrieNode() *trieNode {
	return &trieNode{
		children: make(map[rune]*trieNode),
	}
}

// DefaultConfig 默认配置
func DefaultConfig() *SpotlightConfig {
	return &SpotlightConfig{
		MaxIndexSize:    1000000,
		MinTermLength:   2,
		MaxTermLength:   100,
		EnableCJK:       true,
		EnableStemming:  true,
		IndexBatchSize:  1000,
		SearchTimeout:   5000,
		MaxResults:      1000,
		SuggestionLimit: 10,
	}
}

// response 通用响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
