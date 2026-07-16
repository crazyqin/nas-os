// Package contentseo 提供内容搜索引擎优化功能
package contentseo

import (
	"time"
)

// IndexStatus 索引状态.
type IndexStatus string

const (
	IndexStatusIdle       IndexStatus = "idle"       // 空闲
	IndexStatusIndexing   IndexStatus = "indexing"   // 索引中
	IndexStatusRebuilding IndexStatus = "rebuilding" // 重建中
	IndexStatusFailed     IndexStatus = "failed"     // 失败
)

// SortBy 排序方式.
type SortBy string

const (
	SortByRelevance SortBy = "relevance" // 相关性
	SortByTimeDesc  SortBy = "time_desc" // 时间降序
	SortByTimeAsc   SortBy = "time_asc"  // 时间升序
	SortBySizeDesc  SortBy = "size_desc" // 大小降序
	SortBySizeAsc   SortBy = "size_asc"  // 大小升序
)

// Filter 搜索过滤条件.
type Filter struct {
	Field    string      `json:"field"`    // 过滤字段
	Operator string      `json:"operator"` // 操作符: eq, ne, gt, lt, gte, lte, contains
	Value    interface{} `json:"value"`    // 过滤值
}

// ContentIndex 内容索引条目.
type ContentIndex struct {
	FileID    string    `json:"file_id"`    // 文件ID
	Path      string    `json:"path"`       // 文件路径
	Title     string    `json:"title"`      // 标题
	Content   string    `json:"content"`    // 内容
	Tags      []string  `json:"tags"`       // 标签
	Language  string    `json:"language"`   // 语言
	IndexedAt time.Time `json:"indexed_at"` // 索引时间
	Score     float64   `json:"score"`      // 相关性分数
}

// SearchQuery 搜索查询.
type SearchQuery struct {
	Keyword  string   `json:"keyword"`           // 搜索关键词
	Filters  []Filter `json:"filters,omitempty"` // 过滤条件
	SortBy   SortBy   `json:"sort_by"`           // 排序方式
	Page     int      `json:"page"`              // 页码 (从 1 开始)
	PageSize int      `json:"page_size"`         // 每页数量
}

// SearchResult 搜索结果.
type SearchResult struct {
	Items []ContentIndex `json:"items"` // 结果列表
	Total int            `json:"total"` // 总数
	Page  int            `json:"page"`  // 当前页
	Took  time.Duration  `json:"took"`  // 耗时
}

// QueryStat 查询统计.
type QueryStat struct {
	Keyword string `json:"keyword"` // 查询关键词
	Count   int    `json:"count"`   // 查询次数
}

// SearchStats 搜索统计.
type SearchStats struct {
	TotalIndexed int         `json:"total_indexed"` // 已索引总数
	TopQueries   []QueryStat `json:"top_queries"`   // 热门查询
	IndexSize    int64       `json:"index_size"`    // 索引大小 (bytes)
	LastIndexed  time.Time   `json:"last_indexed"`  // 最后索引时间
}

// IndexStatusInfo 索引状态信息.
type IndexStatusInfo struct {
	Status       IndexStatus `json:"status"`        // 索引状态
	TotalFiles   int         `json:"total_files"`   // 总文件数
	IndexedFiles int         `json:"indexed_files"` // 已索引文件数
	Progress     float64     `json:"progress"`      // 进度百分比
	StartedAt    *time.Time  `json:"started_at"`    // 开始时间
	UpdatedAt    time.Time   `json:"updated_at"`    // 更新时间
}

// RebuildRequest 重建索引请求.
type RebuildRequest struct {
	FullRebuild bool `json:"full_rebuild"` // 是否全量重建
}
