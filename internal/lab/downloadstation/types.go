// Package downloadstation 提供下载站管理功能。
// 支持 HTTP/FTP/磁力链接/BT 下载任务管理、下载队列管理、
// 下载速度限制和统计、下载历史记录、RSS 订阅自动下载、文件校验。
package downloadstation

import (
	"strings"
	"sync"
	"time"
)

// DownloadType 下载类型.
type DownloadType string

const (
	DownloadTypeHTTP   DownloadType = "http"   // HTTP/HTTPS 下载
	DownloadTypeFTP    DownloadType = "ftp"    // FTP 下载
	DownloadTypeMagnet DownloadType = "magnet" // 磁力链接下载
	DownloadTypeBT     DownloadType = "bt"     // BT 种子下载
)

// TaskStatus 下载任务状态.
type TaskStatus string

const (
	TaskStatusPending     TaskStatus = "pending"     // 等待下载
	TaskStatusQueued      TaskStatus = "queued"      // 已加入队列
	TaskStatusDownloading TaskStatus = "downloading" // 下载中
	TaskStatusPaused      TaskStatus = "paused"      // 已暂停
	TaskStatusCompleted   TaskStatus = "completed"   // 已完成
	TaskStatusFailed      TaskStatus = "failed"      // 失败
	TaskStatusSeeding     TaskStatus = "seeding"     // 做种中（BT）
)

// ChecksumType 校验类型.
type ChecksumType string

const (
	ChecksumMD5    ChecksumType = "md5"
	ChecksumSHA256 ChecksumType = "sha256"
)

// Priority 优先级.
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
)

// DownloadTask 下载任务.
type DownloadTask struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	URL          string       `json:"url"`
	Type         DownloadType `json:"type"`
	Status       TaskStatus   `json:"status"`
	FilePath     string       `json:"filePath"`     // 保存路径
	FileName     string       `json:"fileName"`     // 文件名
	FileSize     int64        `json:"fileSize"`     // 文件大小（字节）
	Downloaded   int64        `json:"downloaded"`   // 已下载大小
	Speed        int64        `json:"speed"`        // 当前速度（字节/秒）
	Progress     float64      `json:"progress"`     // 进度百分比 0-100
	Priority     Priority     `json:"priority"`     // 优先级
	MaxSpeed     int64        `json:"maxSpeed"`     // 速度限制（字节/秒），0=不限制
	RetryCount   int          `json:"retryCount"`   // 重试次数
	MaxRetries   int          `json:"maxRetries"`   // 最大重试次数
	ErrorMsg     string       `json:"errorMsg"`     // 错误信息
	Checksum     string       `json:"checksum"`     // 文件校验值
	ChecksumType ChecksumType `json:"checksumType"` // 校验类型
	CreatedAt    time.Time    `json:"createdAt"`
	StartedAt    *time.Time   `json:"startedAt,omitempty"`
	CompletedAt  *time.Time   `json:"completedAt,omitempty"`
	UpdatedAt    time.Time    `json:"updatedAt"`

	// BT 特有字段
	TorrentPath string  `json:"torrentPath,omitempty"` // 种子文件路径
	MagnetURI   string  `json:"magnetUri,omitempty"`   // 磁力链接
	SeedTime    int     `json:"seedTime,omitempty"`    // 做种时间（分钟）
	SeedRatio   float64 `json:"seedRatio,omitempty"`   // 做种比率
	Peers       int     `json:"peers,omitempty"`       // 当前连接数
	Seeds       int     `json:"seeds,omitempty"`       // 当前做种数

	mu sync.RWMutex `json:"-"` // 用于并发安全
}

// GetProgress 获取进度（并发安全）.
func (t *DownloadTask) GetProgress() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Progress
}

// SetProgress 设置进度（并发安全）.
func (t *DownloadTask) SetProgress(progress float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Progress = progress
}

// GetSpeed 获取速度（并发安全）.
func (t *DownloadTask) GetSpeed() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Speed
}

// SetSpeed 设置速度（并发安全）.
func (t *DownloadTask) SetSpeed(speed int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Speed = speed
}

// DownloadStats 下载统计信息.
type DownloadStats struct {
	TotalTasks      int     `json:"totalTasks"`      // 总任务数
	ActiveTasks     int     `json:"activeTasks"`     // 活跃任务数
	CompletedTasks  int     `json:"completedTasks"`  // 完成任务数
	FailedTasks     int     `json:"failedTasks"`     // 失败任务数
	TotalDownloaded int64   `json:"totalDownloaded"` // 总下载量（字节）
	TotalSize       int64   `json:"totalSize"`       // 总文件大小（字节）
	AverageSpeed    int64   `json:"averageSpeed"`    // 平均速度（字节/秒）
	CurrentSpeed    int64   `json:"currentSpeed"`    // 当前总速度（字节/秒）
	TotalTime       int64   `json:"totalTime"`       // 总耗时（秒）
	SuccessRate     float64 `json:"successRate"`     // 成功率
}

// QueueConfig 队列配置.
type QueueConfig struct {
	MaxConcurrent   int    `json:"maxConcurrent"`   // 最大并发下载数
	MaxSpeedTotal   int64  `json:"maxSpeedTotal"`   // 总速度限制（字节/秒），0=不限制
	MaxSpeedPerTask int64  `json:"maxSpeedPerTask"` // 单任务速度限制（字节/秒），0=不限制
	AutoStart       bool   `json:"autoStart"`       // 自动开始下载
	RetryDelay      int    `json:"retryDelay"`      // 重试延迟（秒）
	MaxRetries      int    `json:"maxRetries"`      // 默认最大重试次数
	DownloadDir     string `json:"downloadDir"`     // 默认下载目录
}

// DefaultQueueConfig 默认队列配置.
func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		MaxConcurrent:   3,
		MaxSpeedTotal:   0,
		MaxSpeedPerTask: 0,
		AutoStart:       true,
		RetryDelay:      30,
		MaxRetries:      3,
		DownloadDir:     "/downloads",
	}
}

// RSSFeed RSS 订阅源.
type RSSFeed struct {
	ID          string     `json:"id"`
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Enabled     bool       `json:"enabled"`
	Interval    int        `json:"interval"` // 检查间隔（分钟）
	LastCheck   *time.Time `json:"lastCheck,omitempty"`
	NextCheck   *time.Time `json:"nextCheck,omitempty"`
	Filter      RSSFilter  `json:"filter"` // 过滤规则
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// RSSFilter RSS 过滤规则.
type RSSFilter struct {
	IncludePatterns []string `json:"includePatterns,omitempty"` // 包含模式（正则）
	ExcludePatterns []string `json:"excludePatterns,omitempty"` // 排除模式（正则）
	MaxSize         int64    `json:"maxSize,omitempty"`         // 最大文件大小（字节）
	MinSize         int64    `json:"minSize,omitempty"`         // 最小文件大小（字节）
	AutoDownload    bool     `json:"autoDownload"`              // 自动下载
	DownloadDir     string   `json:"downloadDir,omitempty"`     // 下载目录
}

// RSSItem RSS 条目.
type RSSItem struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description"`
	Size        int64     `json:"size"`
	PubDate     time.Time `json:"pubDate"`
	Downloaded  bool      `json:"downloaded"`
	TaskID      string    `json:"taskId,omitempty"` // 关联的下载任务 ID
}

// HistoryEntry 下载历史记录.
type HistoryEntry struct {
	ID           string       `json:"id"`
	TaskID       string       `json:"taskId"`
	Name         string       `json:"name"`
	URL          string       `json:"url"`
	Type         DownloadType `json:"type"`
	FilePath     string       `json:"filePath"`
	FileSize     int64        `json:"fileSize"`
	Checksum     string       `json:"checksum,omitempty"`
	ChecksumType ChecksumType `json:"checksumType,omitempty"`
	Status       TaskStatus   `json:"status"`
	ErrorMsg     string       `json:"errorMsg,omitempty"`
	StartedAt    time.Time    `json:"startedAt"`
	CompletedAt  time.Time    `json:"completedAt"`
	Duration     int64        `json:"duration"`     // 耗时（秒）
	AverageSpeed int64        `json:"averageSpeed"` // 平均速度（字节/秒）
}

// SpeedStats 速度统计.
type SpeedStats struct {
	Timestamp   time.Time `json:"timestamp"`
	Speed       int64     `json:"speed"`       // 当前速度（字节/秒）
	ActiveTasks int       `json:"activeTasks"` // 活跃任务数
	TotalSpeed  int64     `json:"totalSpeed"`  // 总速度（字节/秒）
}

// CreateTaskRequest 创建下载任务请求.
type CreateTaskRequest struct {
	URL          string       `json:"url" binding:"required"`
	Name         string       `json:"name,omitempty"`
	Type         DownloadType `json:"type,omitempty"`     // 自动检测
	FilePath     string       `json:"filePath,omitempty"` // 保存路径
	Priority     Priority     `json:"priority,omitempty"`
	MaxSpeed     int64        `json:"maxSpeed,omitempty"` // 速度限制
	MaxRetries   int          `json:"maxRetries,omitempty"`
	Checksum     string       `json:"checksum,omitempty"` // 预期校验值
	ChecksumType ChecksumType `json:"checksumType,omitempty"`
	TorrentPath  string       `json:"torrentPath,omitempty"` // 种子文件路径
	SeedTime     int          `json:"seedTime,omitempty"`    // 做种时间
	SeedRatio    float64      `json:"seedRatio,omitempty"`   // 做种比率
}

// UpdateTaskRequest 更新任务请求.
type UpdateTaskRequest struct {
	Priority   Priority `json:"priority,omitempty"`
	MaxSpeed   int64    `json:"maxSpeed,omitempty"`
	MaxRetries int      `json:"maxRetries,omitempty"`
}

// BatchRequest 批量操作请求.
type BatchRequest struct {
	TaskIDs []string `json:"taskIds" binding:"required"`
}

// AddRSSRequest 添加 RSS 订阅请求.
type AddRSSRequest struct {
	URL      string    `json:"url" binding:"required"`
	Title    string    `json:"title,omitempty"`
	Interval int       `json:"interval,omitempty"` // 检查间隔（分钟）
	Filter   RSSFilter `json:"filter,omitempty"`
}

// FileCategory 文件分类.
type FileCategory string

const (
	CategoryDocument FileCategory = "document" // 文档（pdf, doc, docx, txt, rtf, odt, xls, xlsx, ppt, pptx, csv）
	CategoryVideo    FileCategory = "video"    // 视频（mp4, avi, mkv, mov, wmv, flv, webm, m4v, mpg, mpeg）
	CategoryMusic    FileCategory = "music"    // 音乐（mp3, wav, flac, aac, ogg, wma, m4a, aiff）
	CategoryImage    FileCategory = "image"    // 图片（jpg, jpeg, png, gif, bmp, svg, webp, tiff, ico）
	CategoryArchive  FileCategory = "archive"  // 压缩包（zip, rar, 7z, tar, gz, bz2, xz, iso）
	CategoryOther    FileCategory = "other"    // 其他
)

// ClassifyFile 根据文件扩展名自动分类.
func ClassifyFile(fileName string) FileCategory {
	ext := getExtension(fileName)
	switch ext {
	case ".pdf", ".doc", ".docx", ".txt", ".rtf", ".odt",
		".xls", ".xlsx", ".ppt", ".pptx", ".csv",
		".epub", ".mobi":
		return CategoryDocument
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv",
		".webm", ".m4v", ".mpg", ".mpeg", ".ts", ".vob":
		return CategoryVideo
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma",
		".m4a", ".aiff", ".opus":
		return CategoryMusic
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg",
		".webp", ".tiff", ".tif", ".ico", ".heic", ".heif":
		return CategoryImage
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2",
		".xz", ".iso", ".dmg":
		return CategoryArchive
	default:
		return CategoryOther
	}
}

// getExtension 获取文件扩展名（小写）.
func getExtension(fileName string) string {
	for i := len(fileName) - 1; i >= 0; i-- {
		if fileName[i] == '.' {
			return strings.ToLower(fileName[i:])
		}
	}
	return ""
}

// GetCategoryDir 获取分类对应的子目录名.
func GetCategoryDir(category FileCategory) string {
	switch category {
	case CategoryDocument:
		return "documents"
	case CategoryVideo:
		return "videos"
	case CategoryMusic:
		return "music"
	case CategoryImage:
		return "images"
	case CategoryArchive:
		return "archives"
	default:
		return "others"
	}
}

// DownloadSchedule 下载计划.
type DownloadSchedule struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	TaskIDs    []string  `json:"taskIds"`    // 关联的任务 ID
	StartTime  string    `json:"startTime"`  // 开始时间 HH:MM
	EndTime    string    `json:"endTime"`    // 结束时间 HH:MM
	DaysOfWeek []int     `json:"daysOfWeek"` // 星期几（0=周日, 1=周一, ... 6=周六）
	MaxSpeed   int64     `json:"maxSpeed"`   // 该时段速度限制（字节/秒）
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
