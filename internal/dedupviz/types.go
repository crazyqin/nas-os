// Package dedupviz 提供文件去重可视化功能，对标 TrueNAS 重复数据删除。
// 支持扫描指定目录的重复文件、基于 SHA256 去重、可视化数据统计、安全删除建议。
package dedupviz

import "time"

// FileType 文件类型分类
type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeDocument FileType = "document"
	FileTypeArchive  FileType = "archive"
	FileTypeCode     FileType = "code"
	FileTypeOther    FileType = "other"
)

// ScanStatus 扫描状态
type ScanStatus string

const (
	ScanStatusIdle      ScanStatus = "idle"
	ScanStatusScanning  ScanStatus = "scanning"
	ScanStatusAnalyzing ScanStatus = "analyzing"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
	ScanStatusCancelled ScanStatus = "cancelled"
)

// DuplicateFile 重复文件
type DuplicateFile struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	Hash       string    `json:"hash"`
	ModifiedAt time.Time `json:"modified_at"`
	IsOriginal bool      `json:"is_original"` // 是否为建议保留的副本
}

// DuplicateGroup 重复文件组
type DuplicateGroup struct {
	Hash          string            `json:"hash"`
	FileType      FileType          `json:"file_type"`
	FileCount     int               `json:"file_count"`
	FileSize      int64             `json:"file_size"`      // 单个文件大小
	TotalSize     int64             `json:"total_size"`     // 总占用空间
	WastedSize    int64             `json:"wasted_size"`    // 可回收空间
	Files         []DuplicateFile   `json:"files"`
	KeepPath      string            `json:"keep_path"`      // 建议保留的文件路径
	KeepReason    string            `json:"keep_reason"`    // 保留原因
}

// ScanResult 扫描结果
type ScanResult struct {
	ScanID          string           `json:"scan_id"`
	Status          ScanStatus       `json:"status"`
	TargetPaths     []string         `json:"target_paths"`
	TotalFiles      int              `json:"total_files"`
	TotalSize       int64            `json:"total_size"`
	DuplicateGroups int              `json:"duplicate_groups"`
	DuplicateFiles  int              `json:"duplicate_files"`
	WastedSize      int64            `json:"wasted_size"`
	Groups          []DuplicateGroup `json:"groups"`
	StartedAt       time.Time        `json:"started_at"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	Duration        time.Duration    `json:"duration"`
	Progress        float64          `json:"progress"` // 0-100
	Error           string           `json:"error,omitempty"`
}

// VisualizationData 可视化数据
type VisualizationData struct {
	Overview       OverviewData        `json:"overview"`
	BySize         []SizeDistribution  `json:"by_size"`
	ByType         []TypeDistribution  `json:"by_type"`
	ByDirectory    []DirDistribution   `json:"by_directory"`
	TopDuplicates  []DuplicateGroup    `json:"top_duplicates"`
	TimelineData   []TimelinePoint     `json:"timeline_data,omitempty"`
}

// OverviewData 概览数据
type OverviewData struct {
	TotalFiles       int     `json:"total_files"`
	TotalSize        int64   `json:"total_size"`
	DuplicateFiles   int     `json:"duplicate_files"`
	DuplicateSize    int64   `json:"duplicate_size"`
	WastedSize       int64   `json:"wasted_size"`
	DedupRatio       float64 `json:"dedup_ratio"`       // 去重比例
	UniqueFiles      int     `json:"unique_files"`
	UniqueSize       int64   `json:"unique_size"`
	AvgDuplicateSize int64   `json:"avg_duplicate_size"`
}

// SizeDistribution 按大小分布
type SizeDistribution struct {
	RangeLabel string  `json:"range_label"` // "0-1MB", "1-10MB", etc.
	MinSize    int64   `json:"min_size"`
	MaxSize    int64   `json:"max_size"`
	Count      int     `json:"count"`
	TotalSize  int64   `json:"total_size"`
	WastedSize int64   `json:"wasted_size"`
	Percentage float64 `json:"percentage"`
}

// TypeDistribution 按类型分布
type TypeDistribution struct {
	FileType   FileType `json:"file_type"`
	Count      int      `json:"count"`
	TotalSize  int64    `json:"total_size"`
	WastedSize int64    `json:"wasted_size"`
	Percentage float64  `json:"percentage"`
}

// DirDistribution 按目录分布
type DirDistribution struct {
	Directory    string  `json:"directory"`
	FileCount    int     `json:"file_count"`
	DupCount     int     `json:"dup_count"`
	TotalSize    int64   `json:"total_size"`
	WastedSize   int64   `json:"wasted_size"`
	Percentage   float64 `json:"percentage"`
}

// TimelinePoint 时间线数据点
type TimelinePoint struct {
	Timestamp    time.Time `json:"timestamp"`
	DuplicateCount int   `json:"duplicate_count"`
	WastedSize   int64     `json:"wasted_size"`
}

// DeleteRequest 删除请求
type DeleteRequest struct {
	GroupHash string   `json:"group_hash" binding:"required"`
	KeepPath  string   `json:"keep_path" binding:"required"` // 保留的文件
	DryRun    bool     `json:"dry_run"`
}

// DeleteResult 删除结果
type DeleteResult struct {
	DeletedFiles []string `json:"deleted_files"`
	FailedFiles  []FailedFile `json:"failed_files,omitempty"`
	FreedSpace   int64    `json:"freed_space"`
	DryRun       bool     `json:"dry_run"`
}

// FailedFile 删除失败的文件
type FailedFile struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	MinSize    int64  `json:"min_size,omitempty"`    // 最小文件大小
	MaxSize    int64  `json:"max_size,omitempty"`    // 最大文件大小
	FileType   FileType `json:"file_type,omitempty"` // 文件类型过滤
	KeepPolicy string `json:"keep_policy"`           // newest, oldest, shortest_path
	DryRun     bool   `json:"dry_run"`
}

// ScanConfig 扫描配置
type ScanConfig struct {
	TargetPaths    []string `json:"target_paths"`
	ExcludePaths   []string `json:"exclude_paths"`
	ExcludePatterns []string `json:"exclude_patterns"`
	MinFileSize    int64    `json:"min_file_size"`    // 最小文件大小
	MaxFileSize    int64    `json:"max_file_size"`    // 最大文件大小
	MaxDepth       int      `json:"max_depth"`        // 最大扫描深度
	FollowLinks    bool     `json:"follow_links"`     // 跟随符号链接
	NumWorkers     int      `json:"num_workers"`      // 并发数
}

// DefaultScanConfig 默认扫描配置
func DefaultScanConfig() *ScanConfig {
	return &ScanConfig{
		MinFileSize: 1024,       // 1KB
		MaxFileSize: 10737418240, // 10GB
		MaxDepth:    20,
		FollowLinks: false,
		NumWorkers:  4,
		ExcludePaths: []string{
			"/proc", "/sys", "/dev", "/tmp",
			"/var/cache", "/var/tmp",
		},
		ExcludePatterns: []string{
			"*.tmp", "*.swp", "*.bak",
			".DS_Store", "Thumbs.db",
		},
	}
}

// DedupvizConfig 去重可视化配置
type DedupvizConfig struct {
	Enabled         bool     `json:"enabled"`
	DefaultPaths    []string `json:"default_paths"`
	AutoScanEnabled bool     `json:"auto_scan_enabled"`
	AutoScanCron    string   `json:"auto_scan_cron"`     // cron 表达式
	RetentionDays   int      `json:"retention_days"`     // 扫描结果保留天数
	MaxResults      int      `json:"max_results"`        // 最大结果数
	HistoryEnabled  bool     `json:"history_enabled"`
}

// DefaultDedupvizConfig 默认配置
func DefaultDedupvizConfig() *DedupvizConfig {
	return &DedupvizConfig{
		Enabled:         true,
		DefaultPaths:    []string{"/home", "/data"},
		AutoScanEnabled: false,
		AutoScanCron:    "0 3 * * 0", // 每周日凌晨3点
		RetentionDays:   30,
		MaxResults:      10000,
		HistoryEnabled:  true,
	}
}

// 获取文件类型的扩展名映射
var fileTypeExtensions = map[FileType][]string{
	FileTypeImage:    {".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff"},
	FileTypeVideo:    {".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v"},
	FileTypeAudio:    {".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a"},
	FileTypeDocument: {".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".rtf", ".odt"},
	FileTypeArchive:  {".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar", ".tgz"},
	FileTypeCode:     {".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".rs", ".rb", ".php", ".html", ".css", ".json", ".xml", ".yaml", ".yml", ".sh"},
}
