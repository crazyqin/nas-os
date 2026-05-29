// Package unifiedsearch 提供统一全文搜索功能，支持文件名、文件内容、EXIF、标签、元数据的全文索引与搜索。
// 参考群晖 Universal Search 实现，支持多类型搜索、模糊搜索、布尔查询、过滤器、高亮摘要等。
package unifiedsearch

import (
	"time"
)

// IndexStatus 索引状态
type IndexStatus string

const (
	IndexStatusIdle     IndexStatus = "idle"
	IndexStatusBuilding IndexStatus = "building"
	IndexStatusPaused   IndexStatus = "paused"
	IndexStatusError    IndexStatus = "error"
)

// ContentType 内容类型
type ContentType string

const (
	ContentTypeFile    ContentType = "file"
	ContentTypePhoto   ContentType = "photo"
	ContentTypeDocument ContentType = "document"
	ContentTypeVideo   ContentType = "video"
	ContentTypeMusic   ContentType = "music"
)

// BooleanOp 布尔操作符
type BooleanOp string

const (
	BooleanAND BooleanOp = "AND"
	BooleanOR  BooleanOp = "OR"
	BooleanNOT BooleanOp = "NOT"
)

// SortOrder 排序方式
type SortOrder string

const (
	SortRelevance SortOrder = "relevance"
	SortDateDesc  SortOrder = "date_desc"
	SortDateAsc   SortOrder = "date_asc"
	SortSizeDesc  SortOrder = "size_desc"
	SortSizeAsc   SortOrder = "size_asc"
	SortNameAsc   SortOrder = "name_asc"
	SortNameDesc  SortOrder = "name_desc"
)

// IndexTaskType 索引任务类型
type IndexTaskType string

const (
	TaskTypeFull       IndexTaskType = "full"
	TaskTypeIncremental IndexTaskType = "incremental"
	TaskTypeDelete     IndexTaskType = "delete"
)

// IndexTaskStatus 索引任务状态
type IndexTaskStatus string

const (
	TaskStatusPending    IndexTaskStatus = "pending"
	TaskStatusRunning    IndexTaskStatus = "running"
	TaskStatusCompleted  IndexTaskStatus = "completed"
	TaskStatusFailed     IndexTaskStatus = "failed"
	TaskStatusCancelled  IndexTaskStatus = "cancelled"
)

// SearchIndex 搜索索引条目
type SearchIndex struct {
	ID          string            `json:"id"`
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	Extension   string            `json:"extension,omitempty"`
	ContentType ContentType       `json:"content_type"`
	MimeType    string            `json:"mime_type,omitempty"`
	Size        int64             `json:"size"`
	Content     string            `json:"content,omitempty"`     // 提取的文本内容
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`    // EXIF、音频标签等
	Highlights  map[string]string `json:"highlights,omitempty"`  // 字段 -> 高亮片段
	Score       float64           `json:"score,omitempty"`       // 相关度评分
	CreatedAt   time.Time         `json:"created_at"`
	ModifiedAt  time.Time         `json:"modified_at"`
	IndexedAt   time.Time         `json:"indexed_at"`
}

// SearchResult 搜索结果
type SearchResult struct {
	ID          string            `json:"id"`
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	Extension   string            `json:"extension,omitempty"`
	ContentType ContentType       `json:"content_type"`
	MimeType    string            `json:"mime_type,omitempty"`
	Size        int64             `json:"size"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Summary     string            `json:"summary,omitempty"`     // 内容摘要
	Highlights  map[string]string `json:"highlights,omitempty"`  // 高亮匹配
	Score       float64           `json:"score"`                 // 相关度评分
	CreatedAt   time.Time         `json:"created_at"`
	ModifiedAt  time.Time         `json:"modified_at"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Query       string            `json:"query" binding:"required"`
	Types       []ContentType     `json:"types,omitempty"`       // 按内容类型过滤
	Tags        []string          `json:"tags,omitempty"`        // 按标签过滤
	Path        string            `json:"path,omitempty"`        // 路径前缀过滤
	DateFrom    *time.Time        `json:"date_from,omitempty"`   // 日期范围开始
	DateTo      *time.Time        `json:"date_to,omitempty"`     // 日期范围结束
	SizeMin     *int64            `json:"size_min,omitempty"`    // 最小文件大小
	SizeMax     *int64            `json:"size_max,omitempty"`    // 最大文件大小
	BooleanOp   BooleanOp         `json:"boolean_op,omitempty"`  // 布尔操作符
	SortBy      SortOrder         `json:"sort_by,omitempty"`     // 排序方式
	Page        int               `json:"page,omitempty"`        // 页码（从1开始）
	PageSize    int               `json:"page_size,omitempty"`   // 每页大小
	Highlight   bool              `json:"highlight,omitempty"`   // 是否高亮
	Fuzzy       bool              `json:"fuzzy,omitempty"`       // 模糊搜索
	FuzzyLevel  int               `json:"fuzzy_level,omitempty"` // 模糊级别 1-2
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Query      string          `json:"query"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalPages int             `json:"total_pages"`
	Results    []SearchResult  `json:"results"`
	Suggestions []string       `json:"suggestions,omitempty"` // 搜索建议
	TimeMs     int64           `json:"time_ms"`               // 搜索耗时
}

// Filter 搜索过滤器
type Filter struct {
	Types      []ContentType `json:"types,omitempty"`
	Tags       []string      `json:"tags,omitempty"`
	PathPrefix string        `json:"path_prefix,omitempty"`
	DateFrom   *time.Time    `json:"date_from,omitempty"`
	DateTo     *time.Time    `json:"date_to,omitempty"`
	SizeMin    *int64        `json:"size_min,omitempty"`
	SizeMax    *int64        `json:"size_max,omitempty"`
}

// IndexTask 索引任务
type IndexTask struct {
	ID          string          `json:"id"`
	Type        IndexTaskType   `json:"type"`
	Status      IndexTaskStatus `json:"status"`
	Path        string          `json:"path,omitempty"`        // 索引路径
	TotalFiles  int             `json:"total_files"`           // 总文件数
	IndexedFiles int            `json:"indexed_files"`         // 已索引文件数
	FailedFiles int             `json:"failed_files"`          // 失败文件数
	Error       string          `json:"error,omitempty"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// IndexStats 索引统计信息
type IndexStats struct {
	Status         IndexStatus `json:"status"`
	TotalDocuments int         `json:"total_documents"`
	TotalSize      int64       `json:"total_size"`     // 索引总大小（字节）
	LastIndexedAt  *time.Time  `json:"last_indexed_at,omitempty"`
	IndexVersion   int         `json:"index_version"`
	ContentTypes   map[ContentType]int `json:"content_types"` // 各类型数量
}

// SearchHistory 搜索历史
type SearchHistory struct {
	ID        string    `json:"id"`
	Query     string    `json:"query"`
	ResultCount int     `json:"result_count"`
	SearchedAt time.Time `json:"searched_at"`
}

// HotSearch 热门搜索
type HotSearch struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

// IndexRequest 索引请求
type IndexRequest struct {
	Path string `json:"path" binding:"required"`
}

// IndexResponse 索引响应
type IndexResponse struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

// UpdateIndexRequest 更新索引请求
type UpdateIndexRequest struct {
	ID      string            `json:"id" binding:"required"`
	Name    string            `json:"name,omitempty"`
	Tags    []string          `json:"tags,omitempty"`
	Content string            `json:"content,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SuggestRequest 搜索建议请求
type SuggestRequest struct {
	Query string `json:"query" binding:"required"`
	Limit int    `json:"limit,omitempty"`
}

// SuggestResponse 搜索建议响应
type SuggestResponse struct {
	Suggestions []string `json:"suggestions"`
}

// DefaultSearchQuery 默认搜索查询参数
func DefaultSearchQuery() SearchQuery {
	return SearchQuery{
		Page:       1,
		PageSize:   20,
		BooleanOp:  BooleanAND,
		SortBy:     SortRelevance,
		Highlight:  true,
		FuzzyLevel: 1,
	}
}

// DefaultIndexStats 默认索引统计
func DefaultIndexStats() *IndexStats {
	return &IndexStats{
		Status:       IndexStatusIdle,
		ContentTypes: make(map[ContentType]int),
		IndexVersion: 1,
	}
}

// IsValidContentType 检查内容类型是否有效
func IsValidContentType(ct ContentType) bool {
	switch ct {
	case ContentTypeFile, ContentTypePhoto, ContentTypeDocument, ContentTypeVideo, ContentTypeMusic:
		return true
	default:
		return false
	}
}

// IsValidSortOrder 检查排序方式是否有效
func IsValidSortOrder(so SortOrder) bool {
	switch so {
	case SortRelevance, SortDateDesc, SortDateAsc, SortSizeDesc, SortSizeAsc, SortNameAsc, SortNameDesc:
		return true
	default:
		return false
	}
}

// IsValidBooleanOp 检查布尔操作符是否有效
func IsValidBooleanOp(op BooleanOp) bool {
	switch op {
	case BooleanAND, BooleanOR, BooleanNOT:
		return true
	default:
		return false
	}
}
