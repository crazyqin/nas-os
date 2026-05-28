// Package aifilesum 提供AI智能文件摘要生成功能
package aifilesum

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrFileNotFound 文件不存在.
	ErrFileNotFound = errors.New("文件不存在")
	// ErrUnsupportedFormat 不支持的文件格式.
	ErrUnsupportedFormat = errors.New("不支持的文件格式")
	// ErrTaskNotFound 摘要任务不存在.
	ErrTaskNotFound = errors.New("摘要任务不存在")
	// ErrTaskAlreadyRunning 任务已在运行.
	ErrTaskAlreadyRunning = errors.New("摘要任务已在运行中")
	// ErrTaskNotRunning 任务不在运行中.
	ErrTaskNotRunning = errors.New("任务不在运行中")
	// ErrSummaryNotFound 摘要不存在.
	ErrSummaryNotFound = errors.New("摘要不存在")
	// ErrQueueFull 队列已满.
	ErrQueueFull = errors.New("处理队列已满")
	// ErrInvalidLanguage 不支持的语言.
	ErrInvalidLanguage = errors.New("不支持的语言")
)

// ========== 文件类型 ==========

// FileType 文件类型.
type FileType string

const (
	// FileTypeDocument 文档类型（PDF/DOCX/TXT等）.
	FileTypeDocument FileType = "document"
	// FileTypeImage 图片类型（JPG/PNG/GIF等）.
	FileTypeImage FileType = "image"
	// FileTypeVideo 视频类型（MP4/MKV/AVI等）.
	FileTypeVideo FileType = "video"
	// FileTypeUnknown 未知类型.
	FileTypeUnknown FileType = "unknown"
)

// ========== 任务状态 ==========

// TaskStatus 任务状态.
type TaskStatus string

const (
	// TaskStatusPending 等待中.
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning 运行中.
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusCompleted 已完成.
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 失败.
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled 已取消.
	TaskStatusCancelled TaskStatus = "cancelled"
)

// ========== 语言 ==========

// Language 语言类型.
type Language string

const (
	// LanguageAuto 自动检测.
	LanguageAuto Language = "auto"
	// LanguageChinese 中文.
	LanguageChinese Language = "zh"
	// LanguageEnglish 英文.
	LanguageEnglish Language = "en"
	// LanguageJapanese 日文.
	LanguageJapanese Language = "ja"
	// LanguageKorean 韩文.
	LanguageKorean Language = "ko"
)

// ========== 文件信息 ==========

// FileInfo 文件信息.
type FileInfo struct {
	// Path 文件路径.
	Path string `json:"path"`
	// Name 文件名.
	Name string `json:"name"`
	// Size 文件大小（字节）.
	Size int64 `json:"size"`
	// Extension 文件扩展名.
	Extension string `json:"extension"`
	// FileType 文件类型.
	FileType FileType `json:"file_type"`
	// MimeType MIME类型.
	MimeType string `json:"mime_type"`
	// ModTime 修改时间.
	ModTime time.Time `json:"mod_time"`
}

// ========== 摘要结果 ==========

// Summary 摘要结果.
type Summary struct {
	// ID 摘要ID.
	ID string `json:"id"`
	// FileID 文件ID.
	FileID string `json:"file_id"`
	// FileInfo 文件信息.
	FileInfo *FileInfo `json:"file_info"`
	// ContentType 内容类型.
	ContentType string `json:"content_type"`
	// SummaryText 摘要文本.
	SummaryText string `json:"summary_text"`
	// Title 提取的标题.
	Title string `json:"title,omitempty"`
	// Tags 标签列表.
	Tags []string `json:"tags,omitempty"`
	// Language 检测到的语言.
	Language Language `json:"language"`
	// Keywords 关键词列表.
	Keywords []string `json:"keywords,omitempty"`
	// WordCount 原文词数.
	WordCount int `json:"word_count"`
	// SummaryWordCount 摘要词数.
	SummaryWordCount int `json:"summary_word_count"`
	// CompressionRatio 压缩率.
	CompressionRatio float64 `json:"compression_ratio"`
	// ImageDescription 图片描述（仅图片类型）.
	ImageDescription string `json:"image_description,omitempty"`
	// VideoKeyFrames 视频关键帧信息（仅视频类型）.
	VideoKeyFrames []VideoKeyFrame `json:"video_key_frames,omitempty"`
	// VideoDuration 视频时长（秒）.
	VideoDuration float64 `json:"video_duration,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// ProcessedAt 处理时间.
	ProcessedAt time.Time `json:"processed_at"`
	// Duration 处理耗时（毫秒）.
	Duration int64 `json:"duration_ms"`
}

// VideoKeyFrame 视频关键帧.
type VideoKeyFrame struct {
	// Timestamp 时间戳（秒）.
	Timestamp float64 `json:"timestamp"`
	// Description 帧描述.
	Description string `json:"description"`
	// FramePath 帧图片路径（如果保存了的话）.
	FramePath string `json:"frame_path,omitempty"`
}

// ========== 批量处理任务 ==========

// SummarizeTask 摘要任务.
type SummarizeTask struct {
	// ID 任务ID.
	ID string `json:"id"`
	// Status 任务状态.
	Status TaskStatus `json:"status"`
	// Files 待处理文件列表.
	Files []FileInfo `json:"files"`
	// Options 处理选项.
	Options *SummarizeOptions `json:"options"`
	// TotalFiles 总文件数.
	TotalFiles int `json:"total_files"`
	// ProcessedFiles 已处理文件数.
	ProcessedFiles int `json:"processed_files"`
	// FailedFiles 失败文件数.
	FailedFiles int `json:"failed_files"`
	// Results 处理结果.
	Results []*Summary `json:"results"`
	// Errors 错误信息.
	Errors []TaskError `json:"errors,omitempty"`
	// Progress 进度百分比.
	Progress float64 `json:"progress"`
	// StartedAt 开始时间.
	StartedAt time.Time `json:"started_at"`
	// CompletedAt 完成时间.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TaskError 任务错误.
type TaskError struct {
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Error 错误信息.
	Error string `json:"error"`
}

// ========== 配置 ==========

// SummarizeOptions 摘要选项.
type SummarizeOptions struct {
	// Language 目标语言.
	Language Language `json:"language,omitempty"`
	// MaxSummaryLength 最大摘要长度（字数）.
	MaxSummaryLength int `json:"max_summary_length,omitempty"`
	// ExtractKeywords 是否提取关键词.
	ExtractKeywords bool `json:"extract_keywords,omitempty"`
	// ExtractTags 是否提取标签.
	ExtractTags bool `json:"extract_tags,omitempty"`
	// GenerateDescription 是否生成图片描述（图片类型）.
	GenerateDescription bool `json:"generate_description,omitempty"`
	// ExtractKeyFrames 是否提取视频关键帧（视频类型）.
	ExtractKeyFrames bool `json:"extract_key_frames,omitempty"`
	// CacheResults 是否缓存结果.
	CacheResults bool `json:"cache_results,omitempty"`
}

// SummarizerConfig 摘要引擎配置.
type SummarizerConfig struct {
	// AIEndpoint AI服务端点.
	AIEndpoint string `json:"ai_endpoint"`
	// AIModel AI模型名称.
	AIModel string `json:"ai_model"`
	// APIKey API密钥.
	APIKey string `json:"api_key"`
	// MaxConcurrent 最大并发数.
	MaxConcurrent int `json:"max_concurrent"`
	// MaxQueueSize 最大队列大小.
	MaxQueueSize int `json:"max_queue_size"`
	// CacheTTL 缓存过期时间（秒）.
	CacheTTL int `json:"cache_ttl"`
	// SupportedLanguages 支持的语言列表.
	SupportedLanguages []Language `json:"supported_languages"`
	// MaxFileSizeMB 最大文件大小（MB）.
	MaxFileSizeMB int `json:"max_file_size_mb"`
	// VideoFrameIntervalSec 视频关键帧提取间隔（秒）.
	VideoFrameIntervalSec int `json:"video_frame_interval_sec"`
}

// ========== 缓存相关 ==========

// CacheEntry 缓存条目.
type CacheEntry struct {
	// Summary 摘要结果.
	Summary *Summary `json:"summary"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt 过期时间.
	ExpiresAt time.Time `json:"expires_at"`
	// AccessCount 访问次数.
	AccessCount int `json:"access_count"`
}

// IndexEntry 索引条目.
type IndexEntry struct {
	// FileID 文件唯一标识.
	FileID string `json:"file_id"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// SummaryID 摘要ID.
	SummaryID string `json:"summary_id"`
	// Tags 标签.
	Tags []string `json:"tags"`
	// Keywords 关键词.
	Keywords []string `json:"keywords"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
}

// ========== 统计信息 ==========

// Stats 统计信息.
type Stats struct {
	// TotalSummaries 总摘要数.
	TotalSummaries int `json:"total_summaries"`
	// TotalFiles 总处理文件数.
	TotalFiles int `json:"total_files"`
	// DocumentSummaries 文档摘要数.
	DocumentSummaries int `json:"document_summaries"`
	// ImageSummaries 图片摘要数.
	ImageSummaries int `json:"image_summaries"`
	// VideoSummaries 视频摘要数.
	VideoSummaries int `json:"video_summaries"`
	// CacheHits 缓存命中次数.
	CacheHits int `json:"cache_hits"`
	// CacheMisses 缓存未命中次数.
	CacheMisses int `json:"cache_misses"`
	// AvgProcessingTimeMs 平均处理时间（毫秒）.
	AvgProcessingTimeMs float64 `json:"avg_processing_time_ms"`
	// LastUpdated 最后更新时间.
	LastUpdated time.Time `json:"last_updated"`
}
