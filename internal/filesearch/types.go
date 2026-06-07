// Package filesearch 提供全局文件搜索功能，对标群晖Universal Search
package filesearch

import (
	"time"
)

// FileType 文件类型.
type FileType string

const (
	FileTypeAll      FileType = ""
	FileTypeDocument FileType = "document"
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeArchive  FileType = "archive"
	FileTypeCode     FileType = "code"
	FileTypeOther    FileType = "other"
)

// SortBy 排序方式.
type SortBy string

const (
	SortByRelevance SortBy = "relevance"
	SortByName      SortBy = "name"
	SortBySize      SortBy = "size"
	SortByDate      SortBy = "date"
	SortByType      SortBy = "type"
)

// SortOrder 排序方向.
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// SearchResult 搜索结果.
type SearchResult struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Extension  string    `json:"extension"`
	Size       int64     `json:"size"`
	FileType   FileType  `json:"file_type"`
	ModTime    time.Time `json:"mod_time"`
	IsDir      bool      `json:"is_dir"`
	Score      float64   `json:"score"`               // 相关性分数
	MatchCount int       `json:"match_count"`         // 匹配次数
	Highlight  string    `json:"highlight,omitempty"` // 高亮片段
	Tags       []string  `json:"tags,omitempty"`
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Query    string    `json:"query" binding:"required"`
	Path     string    `json:"path"`     // 搜索路径
	Type     FileType  `json:"type"`     // 文件类型过滤
	MinSize  int64     `json:"min_size"` // 最小大小
	MaxSize  int64     `json:"max_size"` // 最大大小
	After    time.Time `json:"after"`    // 修改时间-开始
	Before   time.Time `json:"before"`   // 修改时间-结束
	Tags     []string  `json:"tags"`     // 标签过滤
	Sort     SortBy    `json:"sort"`
	Order    SortOrder `json:"order"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// SearchResponse 搜索响应.
type SearchResponse struct {
	Items     []SearchResult `json:"items"`
	Total     int            `json:"total"`
	Page      int            `json:"page"`
	PageSize  int            `json:"page_size"`
	QueryTime int64          `json:"query_time_ms"` // 查询耗时
	Facets    *SearchFacets  `json:"facets,omitempty"`
}

// SearchFacets 搜索分面.
type SearchFacets struct {
	FileTypes  map[FileType]int `json:"file_types"`
	Extensions map[string]int   `json:"extensions"`
	Sizes      []SizeRange      `json:"sizes"`
}

// SizeRange 大小范围.
type SizeRange struct {
	Label string `json:"label"`
	Min   int64  `json:"min"`
	Max   int64  `json:"max"`
	Count int    `json:"count"`
}

// IndexStatus 索引状态.
type IndexStatus struct {
	TotalFiles    int64     `json:"total_files"`
	IndexedFiles  int64     `json:"indexed_files"`
	LastIndexTime time.Time `json:"last_index_time"`
	IsIndexing    bool      `json:"is_indexing"`
	Progress      float64   `json:"progress"`
}
