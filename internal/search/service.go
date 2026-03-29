// Package search 全局搜索服务接口定义
// 参考: TrueNAS Electric Eel 全局搜索功能
package search

import (
	"context"
	"time"
)

// SearchService 全局搜索服务接口
// 提供统一的搜索入口，支持多种数据源搜索
type SearchService interface {
	// GlobalSearch 执行全局搜索
	// 返回多种类型的结果：文件、设置、应用、容器、元数据、API、文档、日志
	GlobalSearch(ctx context.Context, req SearchRequest) (*SearchResponse, error)

	// SearchFiles 文件搜索
	// 搜索文件系统中的文件和目录
	SearchFiles(ctx context.Context, req FileSearchRequest) (*FileSearchResponse, error)

	// SearchSettings 设置搜索
	// 搜索系统设置项
	SearchSettings(query string, limit int) []SearchResult

	// SearchApps 应用搜索
	// 搜索已安装的应用
	SearchApps(query string, limit int) []SearchResult

	// SearchContainers 容器搜索
	// 搜索Docker容器
	SearchContainers(query string, limit int) []SearchResult

	// SearchAPIs API端点搜索
	// 搜索系统API端点
	SearchAPIs(query string, limit int) []SearchResult

	// SearchDocs 文档搜索
	// 搜索系统文档
	SearchDocs(query string, limit int) []SearchResult

	// SearchLogs 日志搜索
	// 搜索系统日志
	SearchLogs(ctx context.Context, req LogSearchRequest) (*LogSearchResponse, error)

	// RegisterIndexer 注册索引器
	// 用于自定义数据源索引
	RegisterIndexer(name string, indexer Indexer) error

	// BuildIndex 构建索引
	// 全量重建搜索索引
	BuildIndex(ctx context.Context) error

	// UpdateIndex 更新索引
	// 增量更新搜索索引
	UpdateIndex(ctx context.Context, paths []string) error

	// GetStats 获取搜索统计
	// 返回搜索服务的统计信息
	GetStats() *SearchStats

	// ClearHistory 清除搜索历史
	ClearHistory() error

	// GetSuggestions 获取搜索建议
	// 根据查询返回建议的关键词
	GetSuggestions(query string) []string
}

// Indexer 索引器接口
// 用于自定义数据源的内容索引
type Indexer interface {
	// Name 返回索引器名称
	Name() string

	// Index 执行索引
	// 返回索引的条目数量
	Index(ctx context.Context) (int64, error)

	// Search 搜索索引内容
	Search(query string, limit int) []SearchResult

	// Clear 清空索引
	Clear() error

	// Stats 返回索引统计
	Stats() IndexStats
}

// FileSearchService 文件搜索服务接口
type FileSearchService interface {
	// Search 搜索文件
	Search(ctx context.Context, req FileSearchRequest) (*FileSearchResponse, error)

	// IndexPath 索引指定路径
	IndexPath(ctx context.Context, path string, recursive bool) error

	// RemovePath 从索引中移除路径
	RemovePath(path string) error

	// GetFileInfo 获取文件信息
	GetFileInfo(path string) (*FileInfo, error)

	// WatchChanges 监听文件变化
	// 返回变化的文件路径
	WatchChanges(ctx context.Context) (<-chan FileChangeEvent, error)
}

// SettingsSearchService 设置搜索服务接口
type SettingsSearchService interface {
	// Search 搜索设置项
	Search(query string, limit int) []SearchResult

	// Register 注册设置项
	Register(items []SettingItem) error

	// Update 更新设置项
	Update(item SettingItem) error

	// Remove 移除设置项
	Remove(id string) error

	// GetByCategory 按分类获取
	GetByCategory(category string) []SettingItem

	// GetAll 获取所有设置项
	GetAll() []SettingItem
}

// AppSearchService 应用搜索服务接口
type AppSearchService interface {
	// Search 搜索应用
	Search(query string, limit int) []SearchResult

	// Register 注册应用
	Register(items []AppItem) error

	// Update 更新应用信息
	Update(item AppItem) error

	// Remove 移除应用
	Remove(id string) error

	// GetByCategory 按分类获取
	GetByCategory(category string) []AppItem

	// GetInstalled 获取已安装应用
	GetInstalled() []AppItem
}

// APISearchService API搜索服务接口
type APISearchService interface {
	// Search 搜索API端点
	Search(query string, limit int) []SearchResult

	// Register 注册API端点
	Register(endpoints []APIEndpoint) error

	// Update 更新API端点信息
	Update(endpoint APIEndpoint) error

	// Remove 移除API端点
	Remove(id string) error

	// GetByTag 按标签获取
	GetByTag(tag string) []APIEndpoint

	// GetByMethod 按方法获取
	GetByMethod(method string) []APIEndpoint

	// GetAll 获取所有API端点
	GetAll() []APIEndpoint
}

// DocSearchService 文档搜索服务接口
type DocSearchService interface {
	// Search 搜索文档
	Search(query string, limit int) []SearchResult

	// Register 注册文档
	Register(docs []DocumentItem) error

	// Update 更新文档
	Update(doc DocumentItem) error

	// Remove 移除文档
	Remove(id string) error

	// GetByType 按类型获取
	GetByType(docType string) []DocumentItem

	// GetByCategory 按分类获取
	GetByCategory(category string) []DocumentItem

	// GetAll 获取所有文档
	GetAll() []DocumentItem
}

// LogSearchService 日志搜索服务接口
type LogSearchService interface {
	// Search 搜索日志
	Search(ctx context.Context, req LogSearchRequest) (*LogSearchResponse, error)

	// Stream 实时日志流
	Stream(ctx context.Context, filter LogFilter) (<-chan LogEntry, error)

	// RegisterSource 注册日志源
	RegisterSource(source LogSource) error

	// RemoveSource 移除日志源
	RemoveSource(id string) error

	// GetSources 获取所有日志源
	GetSources() []LogSource
}

// --- 数据结构定义 ---

// ResultType 类型别名，引用global.go中的GlobalSearchResultType
type ResultType = GlobalSearchResultType

// SearchRequest 基础搜索请求
type SearchRequest struct {
	Query      string        `json:"query"`      // 搜索查询
	Types      []ResultType  `json:"types"`      // 结果类型过滤
	Limit      int           `json:"limit"`      // 每种类型限制
	TotalLimit int           `json:"totalLimit"` // 总结果限制
	MinScore   float64       `json:"minScore"`   // 最小匹配分数
	IncludeRaw bool          `json:"includeRaw"` // 包含原始数据
	Fuzzy      bool          `json:"fuzzy"`      // 模糊搜索
	Locale     string        `json:"locale"`     // 语言环境
	SortBy     string        `json:"sortBy"`     // 排序字段
	SortDesc   bool          `json:"sortDesc"`   // 降序排序
}

// SearchResponse 基础搜索响应
type SearchResponse struct {
	Query       string            `json:"query"`
	Took        time.Duration     `json:"took"`
	Total       int               `json:"total"`
	Results     []SearchResult    `json:"results"`
	Suggestions []string          `json:"suggestions"`
	Facets      map[string]int    `json:"facets"`
	Errors      []SearchError     `json:"errors,omitempty"`
}

// SearchResult 搜索结果项
type SearchResult struct {
	Type        ResultType `json:"type"`
	Score       float64    `json:"score"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Path        string     `json:"path"`
	Icon        string     `json:"icon"`
	Category    string     `json:"category"`
	MatchType   string     `json:"matchType"`
	MatchField  string     `json:"matchField"`
	RawData     any        `json:"rawData,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ResultType 结果类型 (从global.go引用)
// type ResultType string

// FileSearchRequest 文件搜索请求
type FileSearchRequest struct {
	Query      string     `json:"query"`
	Paths      []string   `json:"paths"`      // 搜索路径限制
	Extensions []string   `json:"extensions"` // 扩展名过滤
	MinSize    int64      `json:"minSize"`    // 最小文件大小
	MaxSize    int64      `json:"maxSize"`    // 最大文件大小
	FromDate   *time.Time `json:"fromDate"`   // 修改时间起始
	ToDate     *time.Time `json:"toDate"`     // 修改时间结束
	Content    bool       `json:"content"`    // 是否搜索内容
	Limit      int        `json:"limit"`
	Offset     int        `json:"offset"`
	SortBy     string     `json:"sortBy"`
	SortDesc   bool       `json:"sortDesc"`
}

// FileSearchResponse 文件搜索响应
type FileSearchResponse struct {
	Query     string        `json:"query"`
	Took      time.Duration `json:"took"`
	Total     int           `json:"total"`
	Files     []FileResult  `json:"files"`
	Truncated bool          `json:"truncated"`
}

// FileResult 文件搜索结果
type FileResult struct {
	Path       string      `json:"path"`
	Name       string      `json:"name"`
	Ext        string      `json:"ext"`
	Size       int64       `json:"size"`
	ModTime    time.Time   `json:"modTime"`
	IsDir      bool        `json:"isDir"`
	Score      float64     `json:"score"`
	Highlights []Highlight `json:"highlights,omitempty"`
}

// LogFilter 日志过滤条件
type LogFilter struct {
	Sources    []string    `json:"sources"`
	Levels     []string    `json:"levels"`
	Components []string    `json:"components"`
	FromDate   *time.Time  `json:"fromDate"`
	ToDate     *time.Time  `json:"toDate"`
	Query      string      `json:"query"`
}

// LogSource 日志源定义
type LogSource struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"` // file, syslog, journald, docker
	Path        string   `json:"path"`
	Format      string   `json:"format"` // json, text, syslog
	Components  []string `json:"components"`
	Description string   `json:"description"`
}

// FileChangeEvent 文件变化事件
type FileChangeEvent struct {
	Path      string    `json:"path"`
	Type      string    `json:"type"` // create, modify, delete
	Timestamp time.Time `json:"timestamp"`
}

// SearchError 搜索错误
type SearchError struct {
	Type    ResultType `json:"type"`
	Message string     `json:"message"`
}

// --- 以下类型已在其他文件中定义，此处为接口完整性引用 ---

// SettingItem 设置项（定义在 settings.go）
// AppItem 应用项（定义在 apps.go）
// ContainerItem 容器项（定义在 apps.go）
// APIEndpoint API端点（定义在 api_registry.go）
// DocumentItem 文档项（定义在 doc_registry.go）
// IndexStats 索引统计（定义在 engine.go）
// FileInfo 文件信息（定义在 engine.go）
// SearchStats 搜索统计（定义在 global.go）