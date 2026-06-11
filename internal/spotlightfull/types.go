// Package spotlightfull 提供统一全文搜索引擎功能
// 参考 TrueNAS 26 的 Spotlight Search + WebShare 集成，实现桌面级搜索体验
// 支持文件名搜索、内容搜索、元数据搜索、模糊匹配、中文分词、macOS Spotlight 协议兼容
package spotlightfull

import (
	"fmt"
	"sync"
	"time"
)

// MatchType 匹配类型
type MatchType string

const (
	MatchFileName  MatchType = "filename"  // 文件名匹配
	MatchContent   MatchType = "content"   // 内容匹配
	MatchMetadata  MatchType = "metadata"  // 元数据匹配
	MatchFuzzy     MatchType = "fuzzy"     // 模糊匹配
	MatchPath      MatchType = "path"      // 路径匹配
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

// SearchFilter 搜索过滤器
type SearchFilter struct {
	Query     string     `json:"query"`               // 搜索关键词
	FileTypes []FileType `json:"file_types,omitempty"` // 文件类型过滤
	MinSize   *int64     `json:"min_size,omitempty"`   // 最小文件大小(字节)
	MaxSize   *int64     `json:"max_size,omitempty"`   // 最大文件大小(字节)
	After     *time.Time `json:"after,omitempty"`      // 修改时间晚于
	Before    *time.Time `json:"before,omitempty"`     // 修改时间早于
	PathScope string     `json:"path_scope,omitempty"` // 路径范围
	Page      int        `json:"page"`                 // 页码(从1开始)
	PageSize  int        `json:"page_size"`            // 每页数量
	SortBy    string     `json:"sort_by"`              // 排序字段: relevance, date, size, name
	SortOrder string     `json:"sort_order"`           // 排序方向: asc, desc
}

// SearchResult 单条搜索结果
type SearchResult struct {
	ID         string    `json:"id"`                   // 文档ID
	Path       string    `json:"path"`                 // 文件路径
	Name       string    `json:"name"`                 // 文件名
	Extension  string    `json:"extension"`            // 扩展名
	FileType   FileType  `json:"file_type"`            // 文件类型
	Size       int64     `json:"size"`                 // 文件大小(字节)
	MimeType   string    `json:"mime_type"`            // MIME类型
	MatchType  MatchType `json:"match_type"`           // 匹配类型
	Highlights []string  `json:"highlights,omitempty"` // 高亮片段
	Score      float64   `json:"score"`                // 相关性评分(0.0~1.0)
	Thumbnail  string    `json:"thumbnail,omitempty"`  // 缩略图URL
	ModifiedAt time.Time `json:"modified_at"`          // 修改时间
	IndexedAt  time.Time `json:"indexed_at"`           // 索引时间
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Results  []SearchResult `json:"results"`            // 搜索结果
	Total    int            `json:"total"`              // 总匹配数
	Page     int            `json:"page"`               // 当前页
	PageSize int            `json:"page_size"`          // 每页数量
	Query    string         `json:"query"`              // 原始查询
	Duration string         `json:"duration"`           // 搜索耗时
	Suggests []string       `json:"suggestions,omitempty"` // 搜索建议
}

// IndexStats 索引统计信息
type IndexStats struct {
	TotalFiles   int64     `json:"total_files"`    // 索引文件总数
	TotalTerms   int64     `json:"total_terms"`    // 索引词项总数
	IndexSize    int64     `json:"index_size"`     // 索引占用空间(字节)
	LastUpdateAt time.Time `json:"last_update_at"` // 最后更新时间
	IsBuilding   bool      `json:"is_building"`    // 是否正在构建
	Progress     float64   `json:"progress"`       // 构建进度(0.0~1.0)
}

// RebuildRequest 索引重建请求
type RebuildRequest struct {
	Force   bool `json:"force"`   // 是否强制全量重建
	Async   bool `json:"async"`   // 是否异步执行
}

// RebuildResponse 索引重建响应
type RebuildResponse struct {
	Status  string `json:"status"`  // 状态: started, completed, error
	Message string `json:"message"` // 详细信息
	TaskID  string `json:"task_id,omitempty"` // 异步任务ID
}

// IndexEntry 索引条目（内部使用）
type IndexEntry struct {
	ID         string            `json:"id"`          // 唯一标识
	Path       string            `json:"path"`        // 文件路径
	Name       string            `json:"name"`        // 文件名
	Extension  string            `json:"extension"`   // 扩展名
	FileType   FileType          `json:"file_type"`   // 文件类型
	Size       int64             `json:"size"`        // 文件大小
	MimeType   string            `json:"mime_type"`   // MIME类型
	Content    string            `json:"content"`     // 提取的文本内容
	Tags       []string          `json:"tags"`        // 标签
	Metadata   map[string]string `json:"metadata"`    // 元数据
	ModifiedAt time.Time         `json:"modified_at"` // 修改时间
	IndexedAt  time.Time         `json:"indexed_at"`  // 索引时间
}

// termPositions 词项在文档中的位置信息
type termPositions struct {
	Fields    []string `json:"fields"`    // 出现的字段列表
	Count     int      `json:"count"`     // 出现次数
	Positions []int    `json:"positions"` // 在内容中的位置偏移
}

// trieNode 前缀树节点（用于自动补全和模糊匹配）
type trieNode struct {
	children map[rune]*trieNode
	isEnd    bool
	terms    []string // 以该节点结尾的词项
}

// invertedIndex 倒排索引
type invertedIndex struct {
	mu       sync.RWMutex
	index    map[string]map[string]*termPositions // term -> docID -> positions
	docs     map[string]*IndexEntry               // docID -> IndexEntry
	trieRoot *trieNode                            // 前缀树根节点
	totalDocs int64                               // 文档总数
	totalTerms int                                // 词项总数
}

// SearchEngine 统一搜索引擎
type SearchEngine struct {
	mu          sync.RWMutex
	index       *invertedIndex
	tokenizer   *cjkTokenizer
	config      *EngineConfig
	stopCh      chan struct{}
	isBuilding  bool
	progress    float64
	lastUpdate  time.Time
}

// EngineConfig 引擎配置
type EngineConfig struct {
	IndexDir       string `json:"index_dir"`       // 索引存储目录
	MaxIndexSize   int64  `json:"max_index_size"`  // 最大索引大小(字节)
	MinTermLength  int    `json:"min_term_length"`  // 最小词项长度
	MaxTermLength  int    `json:"max_term_length"`  // 最大词项长度
	EnableCJK      bool   `json:"enable_cjk"`       // 启用中文分词
	BatchSize      int    `json:"batch_size"`       // 索引批次大小
	MaxResults     int    `json:"max_results"`      // 最大返回结果数
	ThumbnailWidth int    `json:"thumbnail_width"`  // 缩略图宽度
}

// cjkTokenizer 中文分词器（简易实现）
type cjkTokenizer struct {
	dict     map[string]bool // 词典
	stopWord map[string]bool // 停用词
}

// FileIndexer 文件索引器
type FileIndexer struct {
	mu          sync.RWMutex
	engine      *SearchEngine
	config      *IndexerConfig
	isRunning   bool
	stopCh      chan struct{}
	lastIndexed time.Time
	indexedCount int64
}

// IndexerConfig 索引器配置
type IndexerConfig struct {
	ScanPaths       []string      `json:"scan_paths"`       // 扫描路径列表
	ExcludePatterns []string      `json:"exclude_patterns"` // 排除模式
	MaxFileSize     int64         `json:"max_file_size"`    // 最大索引文件大小
	ScanInterval    time.Duration `json:"scan_interval"`    // 增量扫描间隔
	EnableSSDOpt    bool          `json:"enable_ssd_opt"`   // SSD优化存储
	WorkerCount     int           `json:"worker_count"`     // 并发工作线程数
}

// APIError API 错误响应
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// APIResponse 通用 API 响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// DefaultEngineConfig 返回默认引擎配置
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		IndexDir:       "/var/lib/nas-os/spotlightfull/index",
		MaxIndexSize:   1 << 30, // 1GB
		MinTermLength:  1,
		MaxTermLength:  64,
		EnableCJK:      true,
		BatchSize:      1000,
		MaxResults:     10000,
		ThumbnailWidth: 256,
	}
}

// DefaultIndexerConfig 返回默认索引器配置
func DefaultIndexerConfig() *IndexerConfig {
	return &IndexerConfig{
		ScanPaths:       []string{"/data"},
		ExcludePatterns: []string{".git", ".svn", "node_modules", ".DS_Store", "Thumbs.db"},
		MaxFileSize:     50 << 20, // 50MB
		ScanInterval:    5 * time.Minute,
		EnableSSDOpt:    true,
		WorkerCount:     4,
	}
}
