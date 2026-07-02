// Package filemanager - 高级文件管理器
// 文件浏览、操作、预览、分享、搜索、版本管理
// 参考群晖 File Station 和飞牛文件管理器设计
package filemanager

import (
	"time"
)

// ============================================================
// 文件浏览类型
// ============================================================

// FileType 文件类型.
type FileType string

const (
	FileTypeFile      FileType = "file"
	FileTypeDirectory FileType = "directory"
	FileTypeSymlink   FileType = "symlink"
)

// FileNode 文件节点（树形结构）.
type FileNode struct {
	Name          string      `json:"name"`
	Path          string      `json:"path"`
	Type          FileType    `json:"type"`
	Size          int64       `json:"size"`
	ModTime       time.Time   `json:"mod_time"`
	CreateTime    time.Time   `json:"create_time,omitempty"`
	AccessTime    time.Time   `json:"access_time,omitempty"`
	Mode          string      `json:"mode"`
	MIMEType      string      `json:"mime_type"`
	Extension     string      `json:"extension,omitempty"`
	IsHidden      bool        `json:"is_hidden"`
	Children      []*FileNode `json:"children,omitempty"`
	ChildrenCount int         `json:"children_count,omitempty"`
	SymlinkTarget string      `json:"symlink_target,omitempty"`
}

// DirectoryListing 目录列表.
type DirectoryListing struct {
	Path      string      `json:"path"`
	Parent    string      `json:"parent,omitempty"`
	Items     []*FileNode `json:"items"`
	Total     int         `json:"total"`
	TotalSize int64       `json:"total_size"`
	FreeSpace int64       `json:"free_space"`
	UsedSpace int64       `json:"used_space"`
}

// TreeOptions 树形目录选项.
type TreeOptions struct {
	MaxDepth    int  `json:"max_depth"`    // 最大深度，默认3
	ShowHidden  bool `json:"show_hidden"`  // 显示隐藏文件
	IncludeSize bool `json:"include_size"` // 包含大小信息
}

// DefaultTreeOptions 默认树形选项.
func DefaultTreeOptions() TreeOptions {
	return TreeOptions{
		MaxDepth:    3,
		ShowHidden:  false,
		IncludeSize: true,
	}
}

// ============================================================
// 文件操作类型
// ============================================================

// OperationType 操作类型.
type OperationType string

const (
	OpCopy     OperationType = "copy"
	OpMove     OperationType = "move"
	OpDelete   OperationType = "delete"
	OpRename   OperationType = "rename"
	OpCompress OperationType = "compress"
	OpExtract  OperationType = "extract"
)

// OperationStatus 操作状态.
type OperationStatus string

const (
	StatusPending   OperationStatus = "pending"
	StatusRunning   OperationStatus = "running"
	StatusCompleted OperationStatus = "completed"
	StatusFailed    OperationStatus = "failed"
	StatusCancelled OperationStatus = "cancelled"
)

// FileOperation 文件操作记录.
type FileOperation struct {
	ID          string          `json:"id"`
	Type        OperationType   `json:"type"`
	Status      OperationStatus `json:"status"`
	Source      []string        `json:"source"`
	Destination string          `json:"destination,omitempty"`
	Progress    float64         `json:"progress"` // 0-100
	TotalFiles  int             `json:"total_files"`
	Processed   int             `json:"processed"`
	TotalSize   int64           `json:"total_size"`
	Error       string          `json:"error,omitempty"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	CreatedBy   string          `json:"created_by"`
}

// BatchOperation 批量操作请求.
type BatchOperation struct {
	Operation   OperationType `json:"operation" binding:"required"`
	Sources     []string      `json:"sources" binding:"required,min=1"`
	Destination string        `json:"destination,omitempty"`
	Overwrite   bool          `json:"overwrite"`
}

// CompressOptions 压缩选项.
type CompressOptions struct {
	Format   string   `json:"format"` // zip, tar.gz, tar.bz2, 7z
	Level    int      `json:"level"`  // 压缩级别 1-9
	Password string   `json:"password,omitempty"`
	Sources  []string `json:"sources" binding:"required,min=1"`
	Target   string   `json:"target" binding:"required"`
}

// ExtractOptions 解压选项.
type ExtractOptions struct {
	Source      string `json:"source" binding:"required"`
	Destination string `json:"destination" binding:"required"`
	Password    string `json:"password,omitempty"`
	Overwrite   bool   `json:"overwrite"`
}

// ============================================================
// 文件预览类型
// ============================================================

// PreviewType 预览类型.
type PreviewType string

const (
	PreviewImage    PreviewType = "image"
	PreviewVideo    PreviewType = "video"
	PreviewAudio    PreviewType = "audio"
	PreviewDocument PreviewType = "document"
	PreviewCode     PreviewType = "code"
	PreviewText     PreviewType = "text"
	PreviewPDF      PreviewType = "pdf"
	PreviewMarkdown PreviewType = "markdown"
	PreviewNone     PreviewType = "none"
)

// PreviewInfo 预览信息.
type PreviewInfo struct {
	Path     string      `json:"path"`
	Name     string      `json:"name"`
	Type     PreviewType `json:"type"`
	MIMEType string      `json:"mime_type"`
	Size     int64       `json:"size"`
	ModTime  time.Time   `json:"mod_time"`

	// 图片信息
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`

	// 视频/音频信息
	Duration float64 `json:"duration,omitempty"` // 秒
	Bitrate  int     `json:"bitrate,omitempty"`
	Codec    string  `json:"codec,omitempty"`

	// 文档信息
	PageCount int    `json:"page_count,omitempty"`
	Author    string `json:"author,omitempty"`

	// 代码信息
	Language  string `json:"language,omitempty"`
	LineCount int    `json:"line_count,omitempty"`

	// 缩略图
	Thumbnail string `json:"thumbnail,omitempty"` // base64 或 URL
}

// ThumbnailConfig 缩略图配置.
type ThumbnailConfig struct {
	Enabled     bool `json:"enabled"`
	MaxWidth    int  `json:"max_width"`     // 默认 256
	MaxHeight   int  `json:"max_height"`    // 默认 256
	Quality     int  `json:"quality"`       // JPEG质量 1-100, 默认 80
	CacheMaxAge int  `json:"cache_max_age"` // 缓存时间（秒），默认 3600
}

// DefaultThumbnailConfig 默认缩略图配置.
func DefaultThumbnailConfig() ThumbnailConfig {
	return ThumbnailConfig{
		Enabled:     true,
		MaxWidth:    256,
		MaxHeight:   256,
		Quality:     80,
		CacheMaxAge: 3600,
	}
}

// ============================================================
// 文件分享类型
// ============================================================

// ShareLink 分享链接.
type ShareLink struct {
	ID            string     `json:"id"`
	Path          string     `json:"path"`
	Name          string     `json:"name"`
	Token         string     `json:"token"`
	Password      string     `json:"password,omitempty"` // 哈希后的密码
	HasPassword   bool       `json:"has_password"`
	MaxDownloads  int        `json:"max_downloads"` // 0=无限制
	DownloadCount int        `json:"download_count"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	CreatedBy     string     `json:"created_by"`
	Permission    string     `json:"permission"` // "view", "download"
	Enabled       bool       `json:"enabled"`
}

// CreateShareRequest 创建分享请求.
type CreateShareRequest struct {
	Path         string     `json:"path" binding:"required"`
	Password     string     `json:"password,omitempty"`
	MaxDownloads int        `json:"max_downloads"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Permission   string     `json:"permission"` // "view", "download"
}

// ShareStats 分享统计.
type ShareStats struct {
	TotalLinks     int `json:"total_links"`
	ActiveLinks    int `json:"active_links"`
	ExpiredLinks   int `json:"expired_links"`
	TotalDownloads int `json:"total_downloads"`
}

// ============================================================
// 文件搜索类型
// ============================================================

// SearchQuery 搜索查询.
type SearchQuery struct {
	Keyword       string     `json:"keyword" binding:"required"`
	Path          string     `json:"path"`           // 搜索根目录
	Extensions    []string   `json:"extensions"`     // 文件扩展名过滤
	MinSize       *int64     `json:"min_size"`       // 最小文件大小
	MaxSize       *int64     `json:"max_size"`       // 最大文件大小
	ModAfter      *time.Time `json:"mod_after"`      // 修改时间起始
	ModBefore     *time.Time `json:"mod_before"`     // 修改时间结束
	FileType      FileType   `json:"file_type"`      // 文件类型过滤
	ContentSearch bool       `json:"content_search"` // 全文检索
	MaxResults    int        `json:"max_results"`    // 最大结果数，默认100
	SortBy        string     `json:"sort_by"`        // "name", "size", "mod_time"
	SortOrder     string     `json:"sort_order"`     // "asc", "desc"
}

// SearchResult 搜索结果.
type SearchResult struct {
	Items     []*FileNode `json:"items"`
	Total     int         `json:"total"`
	Truncated bool        `json:"truncated"`
	Query     SearchQuery `json:"query"`
	Duration  int64       `json:"duration_ms"` // 搜索耗时（毫秒）
}

// ============================================================
// 文件版本管理
// ============================================================

// FileVersion 文件版本.
type FileVersion struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	Version   int       `json:"version"`
	Size      int64     `json:"size"`
	Checksum  string    `json:"checksum"`
	Comment   string    `json:"comment,omitempty"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// VersionConfig 版本管理配置.
type VersionConfig struct {
	Enabled        bool `json:"enabled"`
	MaxVersions    int  `json:"max_versions"`      // 每文件最大版本数，默认10
	MaxTotalSizeMB int  `json:"max_total_size_mb"` // 最大总大小(MB)，默认1024
	AutoVersion    bool `json:"auto_version"`      // 自动版本管理
}

// DefaultVersionConfig 默认版本管理配置.
func DefaultVersionConfig() VersionConfig {
	return VersionConfig{
		Enabled:        true,
		MaxVersions:    10,
		MaxTotalSizeMB: 1024,
		AutoVersion:    true,
	}
}

// ============================================================
// 收藏夹和快捷方式
// ============================================================

// Favorite 收藏项.
type Favorite struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Type      FileType  `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
	OrderBy   int       `json:"order_by"` // 排序顺序
}

// Shortcut 快捷方式.
type Shortcut struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Icon      string    `json:"icon,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// 拖拽操作
// ============================================================

// DragDropRequest 拖拽请求.
type DragDropRequest struct {
	Sources     []string `json:"sources" binding:"required,min=1"`
	Destination string   `json:"destination" binding:"required"`
	Action      string   `json:"action"` // "copy", "move"
}

// ============================================================
// 文件属性
// ============================================================

// FileAttributes 文件属性.
type FileAttributes struct {
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	Type          FileType  `json:"type"`
	Size          int64     `json:"size"`
	Mode          string    `json:"mode"`
	ModeOctal     string    `json:"mode_octal"`
	Owner         string    `json:"owner"`
	Group         string    `json:"group"`
	UID           int       `json:"uid"`
	GID           int       `json:"gid"`
	MIMEType      string    `json:"mime_type"`
	Extension     string    `json:"extension"`
	ModTime       time.Time `json:"mod_time"`
	AccessTime    time.Time `json:"access_time"`
	CreateTime    time.Time `json:"create_time"`
	IsHidden      bool      `json:"is_hidden"`
	IsSymlink     bool      `json:"is_symlink"`
	SymlinkTarget string    `json:"symlink_target,omitempty"`
	Inode         uint64    `json:"inode,omitempty"`
	Links         uint64    `json:"links,omitempty"`
}

// DiskUsage 磁盘使用情况.
type DiskUsage struct {
	Path        string  `json:"path"`
	Total       int64   `json:"total"`
	Free        int64   `json:"free"`
	Used        int64   `json:"used"`
	UsedPercent float64 `json:"used_percent"`
}

// ============================================================
// 管理器配置
// ============================================================

// Config 文件管理器配置.
type Config struct {
	RootPath       string          `json:"root_path"`       // 根目录路径
	MaxUploadSize  int64           `json:"max_upload_size"` // 最大上传大小（字节）
	TempDir        string          `json:"temp_dir"`        // 临时目录
	Thumbnails     ThumbnailConfig `json:"thumbnails"`
	Versions       VersionConfig   `json:"versions"`
	AllowedExts    []string        `json:"allowed_extensions"` // 允许的扩展名，空=全部
	BlockedExts    []string        `json:"blocked_extensions"` // 禁止的扩展名
	EnablePreview  bool            `json:"enable_preview"`
	EnableShare    bool            `json:"enable_share"`
	EnableVersions bool            `json:"enable_versions"`
	EnableSearch   bool            `json:"enable_search"`
}

// DefaultConfig 默认配置.
func DefaultConfig(rootPath string) Config {
	return Config{
		RootPath:       rootPath,
		MaxUploadSize:  10 * 1024 * 1024 * 1024, // 10GB
		TempDir:        "/tmp/nas-filemanager",
		Thumbnails:     DefaultThumbnailConfig(),
		Versions:       DefaultVersionConfig(),
		AllowedExts:    []string{},
		BlockedExts:    []string{},
		EnablePreview:  true,
		EnableShare:    true,
		EnableVersions: true,
		EnableSearch:   true,
	}
}
