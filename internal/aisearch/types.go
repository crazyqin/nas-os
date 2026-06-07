// Package aisearch 提供 AI 语义搜索引擎，支持跨 NAS 内容的全文检索和语义匹配
package aisearch

import (
	"fmt"
	"sync"
	"time"
)

// SearchMode 搜索模式
type SearchMode string

const (
	SearchModeFullText SearchMode = "fulltext" // 全文检索
	SearchModeSemantic SearchMode = "semantic" // 语义搜索
	SearchModeHybrid   SearchMode = "hybrid"   // 混合搜索（全文 + 语义）
)

// FileType 文件类型
type FileType string

const (
	FileTypeDocument FileType = "document" // 文档
	FileTypeImage    FileType = "image"    // 图片
	FileTypeVideo    FileType = "video"    // 视频
	FileTypeAudio    FileType = "audio"    // 音频
	FileTypeArchive  FileType = "archive"  // 压缩包
	FileTypeCode     FileType = "code"     // 代码
	FileTypeOther    FileType = "other"    // 其他
)

// IndexStatus 索引状态
type IndexStatus string

const (
	IndexStatusPending  IndexStatus = "pending"  // 待索引
	IndexStatusIndexing IndexStatus = "indexing" // 索引中
	IndexStatusIndexed  IndexStatus = "indexed"  // 已索引
	IndexStatusFailed   IndexStatus = "failed"   // 索引失败
	IndexStatusOutdated IndexStatus = "outdated" // 需要更新
)

// SortOrder 排序方式
type SortOrder string

const (
	SortOrderRelevance SortOrder = "relevance" // 相关性
	SortOrderTimeDesc  SortOrder = "time_desc" // 时间降序
	SortOrderTimeAsc   SortOrder = "time_asc"  // 时间升序
	SortOrderSizeDesc  SortOrder = "size_desc" // 大小降序
	SortOrderSizeAsc   SortOrder = "size_asc"  // 大小升序
	SortOrderFrequency SortOrder = "frequency" // 使用频率
)

// SearchQuery 搜索查询
type SearchQuery struct {
	Keyword   string     `json:"keyword"`             // 搜索关键词
	Mode      SearchMode `json:"mode"`                // 搜索模式
	FileTypes []FileType `json:"fileTypes,omitempty"` // 文件类型过滤
	Tags      []string   `json:"tags,omitempty"`      // 标签过滤
	DateFrom  *time.Time `json:"dateFrom,omitempty"`  // 起始日期
	DateTo    *time.Time `json:"dateTo,omitempty"`    // 结束日期
	SizeMin   *int64     `json:"sizeMin,omitempty"`   // 最小文件大小 (bytes)
	SizeMax   *int64     `json:"sizeMax,omitempty"`   // 最大文件大小 (bytes)
	Paths     []string   `json:"paths,omitempty"`     // 限定路径
	Sort      SortOrder  `json:"sort"`                // 排序方式
	Page      int        `json:"page"`                // 页码 (从 1 开始)
	PageSize  int        `json:"pageSize"`            // 每页数量
	Semantic  bool       `json:"semantic,omitempty"`  // 是否启用语义搜索
	Vector    []float64  `json:"vector,omitempty"`    // 语义向量 (由引擎生成)
}

// SearchResult 搜索结果
type SearchResult struct {
	ID          string            `json:"id"`
	FilePath    string            `json:"filePath"`
	FileName    string            `json:"fileName"`
	FileType    FileType          `json:"fileType"`
	FileSize    int64             `json:"fileSize"`
	ModifiedAt  time.Time         `json:"modifiedAt"`
	CreatedAt   time.Time         `json:"createdAt"`
	Score       float64           `json:"score"`      // 综合得分
	Highlights  []Highlight       `json:"highlights"` // 高亮片段
	Snippet     string            `json:"snippet"`    // 内容摘要
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	VectorScore float64           `json:"vectorScore"` // 语义相似度得分
	TextScore   float64           `json:"textScore"`   // 全文检索得分
}

// Highlight 高亮片段
type Highlight struct {
	Field   string `json:"field"`   // 高亮字段
	Content string `json:"content"` // 高亮内容
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Query       string         `json:"query"`
	Total       int            `json:"total"`
	Page        int            `json:"page"`
	PageSize    int            `json:"pageSize"`
	Results     []SearchResult `json:"results"`
	Suggestions []string       `json:"suggestions,omitempty"` // 搜索建议
	Facets      *SearchFacets  `json:"facets,omitempty"`      // 分面统计
	QueryTime   time.Duration  `json:"queryTime"`
}

// SearchFacets 搜索分面统计
type SearchFacets struct {
	FileTypes map[FileType]int `json:"fileTypes"` // 按文件类型统计
	Tags      map[string]int   `json:"tags"`      // 按标签统计
	Paths     map[string]int   `json:"paths"`     // 按路径统计
}

// SearchIndex 搜索索引条目
type SearchIndex struct {
	ID          string            `json:"id"`
	FilePath    string            `json:"filePath"`
	FileName    string            `json:"fileName"`
	FileType    FileType          `json:"fileType"`
	FileSize    int64             `json:"fileSize"`
	ModifiedAt  time.Time         `json:"modifiedAt"`
	CreatedAt   time.Time         `json:"createdAt"`
	ContentHash string            `json:"contentHash"` // 内容哈希
	Content     string            `json:"content"`     // 提取的文本内容
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Vector      []float64         `json:"vector,omitempty"` // 语义向量
	Status      IndexStatus       `json:"status"`
	IndexedAt   *time.Time        `json:"indexedAt,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// SearchConfig 搜索引擎配置
type SearchConfig struct {
	IndexDir         string        `json:"indexDir"`         // 索引存储目录
	MaxResults       int           `json:"maxResults"`       // 最大返回结果数
	PageSize         int           `json:"pageSize"`         // 默认每页数量
	EnableSemantic   bool          `json:"enableSemantic"`   // 启用语义搜索
	VectorDimension  int           `json:"vectorDimension"`  // 向量维度
	SimilarityThresh float64       `json:"similarityThresh"` // 相似度阈值
	IndexWorkers     int           `json:"indexWorkers"`     // 索引并发数
	IndexInterval    time.Duration `json:"indexInterval"`    // 增量索引间隔
	CacheSize        int           `json:"cacheSize"`        // 缓存大小
	CacheTTL         time.Duration `json:"cacheTTL"`         // 缓存过期时间
	MaxFileSize      int64         `json:"maxFileSize"`      // 最大可索引文件大小
	SupportedTypes   []FileType    `json:"supportedTypes"`   // 支持索引的文件类型
}

// SearchStats 搜索统计
type SearchStats struct {
	TotalDocuments   int64     `json:"totalDocuments"`
	TotalSize        int64     `json:"totalSize"`
	IndexedDocuments int64     `json:"indexedDocuments"`
	PendingDocuments int64     `json:"pendingDocuments"`
	FailedDocuments  int64     `json:"failedDocuments"`
	LastIndexTime    time.Time `json:"lastIndexTime"`
	IndexSize        int64     `json:"indexSize"`
	SearchCount      int64     `json:"searchCount"`
	AvgQueryTime     float64   `json:"avgQueryTime"` // ms
	HotWords         []HotWord `json:"hotWords"`
}

// HotWord 热词
type HotWord struct {
	Word  string `json:"word"`
	Count int64  `json:"count"`
}

// SearchHistory 搜索历史
type SearchHistory struct {
	ID          string     `json:"id"`
	Keyword     string     `json:"keyword"`
	Mode        SearchMode `json:"mode"`
	ResultCount int        `json:"resultCount"`
	SearchedAt  time.Time  `json:"searchedAt"`
}

// Suggestion 搜索建议
type Suggestion struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
	Count int64   `json:"count"`
}

// ValidationError 参数校验错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("参数校验失败 [%s]: %s", e.Field, e.Message)
}

// Validate 校验 SearchQuery
func (q *SearchQuery) Validate() error {
	if q.Keyword == "" {
		return &ValidationError{Field: "keyword", Message: "不能为空"}
	}
	if q.Mode != "" && q.Mode != SearchModeFullText && q.Mode != SearchModeSemantic && q.Mode != SearchModeHybrid {
		return &ValidationError{Field: "mode", Message: "必须是 fulltext / semantic / hybrid"}
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}
	if q.Sort != "" && q.Sort != SortOrderRelevance && q.Sort != SortOrderTimeDesc && q.Sort != SortOrderTimeAsc &&
		q.Sort != SortOrderSizeDesc && q.Sort != SortOrderSizeAsc && q.Sort != SortOrderFrequency {
		return &ValidationError{Field: "sort", Message: "无效的排序方式"}
	}
	if q.DateFrom != nil && q.DateTo != nil && q.DateFrom.After(*q.DateTo) {
		return &ValidationError{Field: "dateRange", Message: "起始日期不能晚于结束日期"}
	}
	if q.SizeMin != nil && q.SizeMax != nil && *q.SizeMin > *q.SizeMax {
		return &ValidationError{Field: "sizeRange", Message: "最小大小不能大于最大大小"}
	}
	return nil
}

// DefaultSearchConfig 默认搜索配置
func DefaultSearchConfig() *SearchConfig {
	return &SearchConfig{
		IndexDir:         "/var/lib/nas-os/search-index",
		MaxResults:       1000,
		PageSize:         20,
		EnableSemantic:   true,
		VectorDimension:  384,
		SimilarityThresh: 0.7,
		IndexWorkers:     4,
		IndexInterval:    5 * time.Minute,
		CacheSize:        1000,
		CacheTTL:         10 * time.Minute,
		MaxFileSize:      100 * 1024 * 1024, // 100MB
		SupportedTypes: []FileType{
			FileTypeDocument, FileTypeImage, FileTypeVideo,
			FileTypeAudio, FileTypeArchive, FileTypeCode,
		},
	}
}

// SearchEngine 搜索引擎接口
type SearchEngine interface {
	// Search 执行搜索
	Search(query *SearchQuery) (*SearchResponse, error)
	// IndexDocument 索引单个文档
	IndexDocument(doc *SearchIndex) error
	// IndexBatch 批量索引
	IndexBatch(docs []*SearchIndex) error
	// DeleteDocument 删除文档索引
	DeleteDocument(id string) error
	// UpdateDocument 更新文档索引
	UpdateDocument(doc *SearchIndex) error
	// GetStats 获取搜索统计
	GetStats() (*SearchStats, error)
	// Suggest 搜索建议
	Suggest(prefix string, limit int) ([]Suggestion, error)
	// Close 关闭引擎
	Close() error
}

// FileCrawler 文件爬虫接口
type FileCrawler interface {
	// Crawl 遍历目录
	Crawl(rootPath string, callback func(*FileInfo) error) error
	// Watch 监听文件变化
	Watch(paths []string, callback func(*FileEvent) error) error
	// Stop 停止爬虫
	Stop() error
}

// FileInfo 文件信息
type FileInfo struct {
	Path       string            `json:"path"`
	Name       string            `json:"name"`
	Size       int64             `json:"size"`
	ModifiedAt time.Time         `json:"modifiedAt"`
	CreatedAt  time.Time         `json:"createdAt"`
	IsDir      bool              `json:"isDir"`
	Extension  string            `json:"extension"`
	MimeType   string            `json:"mimeType"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// FileEvent 文件事件
type FileEvent struct {
	Type     string    `json:"type"` // create, modify, delete
	FilePath string    `json:"filePath"`
	FileInfo *FileInfo `json:"fileInfo,omitempty"`
}

// ContentExtractor 内容提取器接口
type ContentExtractor interface {
	// Extract 提取文件文本内容
	Extract(filePath string) (string, error)
	// CanExtract 是否支持该文件类型
	CanExtract(fileType FileType, mimeType string) bool
}

// VectorEncoder 向量编码器接口
type VectorEncoder interface {
	// Encode 将文本编码为向量
	Encode(text string) ([]float64, error)
	// EncodeBatch 批量编码
	EncodeBatch(texts []string) ([][]float64, error)
	// Dimension 向量维度
	Dimension() int
}

// CacheItem 缓存项
type CacheItem struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
	ExpiresAt time.Time   `json:"expiresAt"`
}

// SearchCache 搜索缓存
type SearchCache struct {
	mu      sync.RWMutex
	items   map[string]*CacheItem
	maxSize int
	ttl     time.Duration
	hits    int64
	misses  int64
}
