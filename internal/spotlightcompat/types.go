// Package spotlightcompat 实现 SMB Spotlight 协议兼容层
// 对标 TrueNAS 26 TrueSearch，支持 macOS Finder 原生搜索
package spotlightcompat

import "time"

// SpotlightConfig Spotlight 服务配置
type SpotlightConfig struct {
	Enabled         bool     `json:"enabled"`
	IndexPath       string   `json:"indexPath"`
	MaxIndexSize    int64    `json:"maxIndexSize"`    // 最大索引大小(MB)
	IndexInterval   int      `json:"indexInterval"`   // 索引更新间隔(秒)
	FileExtensions  []string `json:"fileExtensions"`  // 索引的文件扩展名
	ExcludePatterns []string `json:"excludePatterns"` // 排除模式
	SMBShares       []string `json:"smbShares"`       // 关联的SMB共享
	ProtocolVersion string   `json:"protocolVersion"` // SMB2/SMB3
}

// SpotlightIndex 索引条目
type SpotlightIndex struct {
	ID          string            `json:"id"`
	FilePath    string            `json:"filePath"`
	FileName    string            `json:"fileName"`
	FileType    string            `json:"fileType"`
	Size        int64             `json:"size"`
	CreatedAt   time.Time         `json:"createdAt"`
	ModifiedAt  time.Time         `json:"modifiedAt"`
	IndexedAt   time.Time         `json:"indexedAt"`
	ContentHash string            `json:"contentHash"`
	Metadata    map[string]string `json:"metadata"`
	Tags        []string          `json:"tags"`
	FullPath    string            `json:"fullPath"`
	SharePath   string            `json:"sharePath"`
	IsDir       bool              `json:"isDir"`
	Permissions string            `json:"permissions"`
}

// SpotlightSearchRequest 搜索请求
type SpotlightSearchRequest struct {
	Query         string            `json:"query"`
	FileType      string            `json:"fileType,omitempty"`
	Extensions    []string          `json:"extensions,omitempty"`
	DateFrom      *time.Time        `json:"dateFrom,omitempty"`
	DateTo        *time.Time        `json:"dateTo,omitempty"`
	MinSize       *int64            `json:"minSize,omitempty"`
	MaxSize       *int64            `json:"maxSize,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	SortBy        string            `json:"sortBy,omitempty"`    // name/date/size/relevance
	SortOrder     string            `json:"sortOrder,omitempty"` // asc/desc
	Page          int               `json:"page"`
	PageSize      int               `json:"pageSize"`
	Filters       map[string]string `json:"filters,omitempty"`
	SharePath     string            `json:"sharePath,omitempty"`
	ContentSearch bool              `json:"contentSearch,omitempty"`
}

// SpotlightSearchResponse 搜索响应
type SpotlightSearchResponse struct {
	Results     []SpotlightResult `json:"results"`
	TotalCount  int               `json:"totalCount"`
	Page        int               `json:"page"`
	PageSize    int               `json:"pageSize"`
	QueryTimeMs int64             `json:"queryTimeMs"`
	Suggestions []string          `json:"suggestions,omitempty"`
	Facets      map[string]int    `json:"facets,omitempty"`
}

// SpotlightResult 搜索结果
type SpotlightResult struct {
	Index      SpotlightIndex `json:"index"`
	Score      float64        `json:"score"`
	Highlights []string       `json:"highlights,omitempty"`
	Preview    string         `json:"preview,omitempty"`
}

// SpotlightStatus 服务状态
type SpotlightStatus struct {
	Running        bool      `json:"running"`
	TotalIndexed   int       `json:"totalIndexed"`
	IndexSizeMB    float64   `json:"indexSizeMB"`
	LastIndexedAt  time.Time `json:"lastIndexedAt"`
	IndexingRate   float64   `json:"indexingRate"` // 文件/秒
	QueryRate      float64   `json:"queryRate"`    // 查询/分钟
	AvgQueryMs     int64     `json:"avgQueryMs"`
	Uptime         string    `json:"uptime"`
	ProtocolCompat string    `json:"protocolCompat"` // SMB2/SMB3/Spotlight
	ConnectedMacs  int       `json:"connectedMacs"`
	ShareCount     int       `json:"shareCount"`
}

// SpotlightStats 统计信息
type SpotlightStats struct {
	TotalSearches  int64         `json:"totalSearches"`
	AvgResponseMs  int64         `json:"avgResponseMs"`
	P95ResponseMs  int64         `json:"p95ResponseMs"`
	P99ResponseMs  int64         `json:"p99ResponseMs"`
	TopQueries     []QueryStat   `json:"topQueries"`
	IndexStats     IndexStats    `json:"indexStats"`
	ProtocolStats  ProtocolStats `json:"protocolStats"`
	HourlySearches []int64       `json:"hourlySearches"`
}

// QueryStat 查询统计
type QueryStat struct {
	Query    string    `json:"query"`
	Count    int64     `json:"count"`
	AvgMs    int64     `json:"avgMs"`
	LastUsed time.Time `json:"lastUsed"`
}

// IndexStats 索引统计
type IndexStats struct {
	TotalFiles    int            `json:"totalFiles"`
	TotalDirs     int            `json:"totalDirs"`
	TotalSizeMB   float64        `json:"totalSizeMB"`
	FileTypeDist  map[string]int `json:"fileTypeDist"`
	LastFullIndex time.Time      `json:"lastFullIndex"`
	IndexErrors   int            `json:"indexErrors"`
}

// ProtocolStats 协议统计
type ProtocolStats struct {
	SMB2Connections  int   `json:"smb2Connections"`
	SMB3Connections  int   `json:"smb3Connections"`
	SpotlightQueries int64 `json:"spotlightQueries"`
	FailedQueries    int64 `json:"failedQueries"`
}

// IndexTask 索引任务
type IndexTask struct {
	ID         string    `json:"id"`
	SharePath  string    `json:"sharePath"`
	Status     string    `json:"status"` // pending/running/completed/failed
	Progress   float64   `json:"progress"`
	StartedAt  time.Time `json:"startedAt"`
	FilesDone  int       `json:"filesDone"`
	FilesTotal int       `json:"filesTotal"`
	Error      string    `json:"error,omitempty"`
}
