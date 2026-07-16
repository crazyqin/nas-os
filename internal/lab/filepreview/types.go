// Package filepreview 提供文件预览功能
// 支持图片缩略图、视频关键帧、文档在线预览、音频波形可视化和3D模型预览
// 所有预览结果通过缓存系统管理，避免重复生成
package filepreview

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrUnsupportedFormat 不支持的文件格式.
	ErrUnsupportedFormat = errors.New("不支持的文件格式")
	// ErrFileNotFound 文件不存在.
	ErrFileNotFound = errors.New("文件不存在")
	// ErrCacheMiss 缓存未命中.
	ErrCacheMiss = errors.New("缓存未命中")
	// ErrCacheExpired 缓存已过期.
	ErrCacheExpired = errors.New("缓存已过期")
	// ErrGenerationFailed 预览生成失败.
	ErrGenerationFailed = errors.New("预览生成失败")
	// ErrInvalidSize 无效的尺寸参数.
	ErrInvalidSize = errors.New("无效的尺寸参数")
	// ErrFFmpegNotFound FFmpeg 未安装.
	ErrFFmpegNotFound = errors.New("FFmpeg 未安装")
	// ErrInvalidTimeRange 无效的时间范围.
	ErrInvalidTimeRange = errors.New("无效的时间范围")
	// ErrModelLoadFailed 模型加载失败.
	ErrModelLoadFailed = errors.New("模型加载失败")
)

// ========== 文件类型定义 ==========

// FileType 文件类型.
type FileType string

const (
	// FileTypeImage 图片文件.
	FileTypeImage FileType = "image"
	// FileTypeVideo 视频文件.
	FileTypeVideo FileType = "video"
	// FileTypeAudio 音频文件.
	FileTypeAudio FileType = "audio"
	// FileTypeDocument 文档文件.
	FileTypeDocument FileType = "document"
	// FileType3D 3D模型文件.
	FileType3D FileType = "3d"
	// FileTypeUnknown 未知类型.
	FileTypeUnknown FileType = "unknown"
)

// ========== 图片格式定义 ==========

// ImageFormat 图片格式.
type ImageFormat string

const (
	// FormatJPEG JPEG 格式.
	FormatJPEG ImageFormat = "jpeg"
	// FormatPNG PNG 格式.
	FormatPNG ImageFormat = "png"
	// FormatGIF GIF 格式.
	FormatGIF ImageFormat = "gif"
	// FormatWebP WebP 格式.
	FormatWebP ImageFormat = "webp"
	// FormatHEIC HEIC 格式.
	FormatHEIC ImageFormat = "heic"
	// FormatRAW RAW 格式（CR2/NEF/ARW/DNG）.
	FormatRAW ImageFormat = "raw"
	// FormatBMP BMP 格式.
	FormatBMP ImageFormat = "bmp"
	// FormatTIFF TIFF 格式.
	FormatTIFF ImageFormat = "tiff"
)

// ========== 视频格式定义 ==========

// VideoFormat 视频格式.
type VideoFormat string

const (
	// VideoMP4 MP4 格式.
	VideoMP4 VideoFormat = "mp4"
	// VideoMKV MKV 格式.
	VideoMKV VideoFormat = "mkv"
	// VideoAVI AVI 格式.
	VideoAVI VideoFormat = "avi"
	// VideoMOV MOV 格式.
	VideoMOV VideoFormat = "mov"
	// VideoWMV WMV 格式.
	VideoWMV VideoFormat = "wmv"
	// VideoWebM WebM 格式.
	VideoWebM VideoFormat = "webm"
	// VideoFLV FLV 格式.
	VideoFLV VideoFormat = "flv"
)

// ========== 音频格式定义 ==========

// AudioFormat 音频格式.
type AudioFormat string

const (
	// AudioMP3 MP3 格式.
	AudioMP3 AudioFormat = "mp3"
	// AudioFLAC FLAC 格式.
	AudioFLAC AudioFormat = "flac"
	// AudioWAV WAV 格式.
	AudioWAV AudioFormat = "wav"
	// AudioAAC AAC 格式.
	AudioAAC AudioFormat = "aac"
	// AudioOGG OGG 格式.
	AudioOGG AudioFormat = "ogg"
	// AudioOPUS OPUS 格式.
	AudioOPUS AudioFormat = "opus"
	// AudioM4A M4A 格式.
	AudioM4A AudioFormat = "m4a"
)

// ========== 文档格式定义 ==========

// DocumentFormat 文档格式.
type DocumentFormat string

const (
	// DocPDF PDF 格式.
	DocPDF DocumentFormat = "pdf"
	// DocDOCX Word DOCX 格式.
	DocDOCX DocumentFormat = "docx"
	// DocXLSX Excel XLSX 格式.
	DocXLSX DocumentFormat = "xlsx"
	// DocPPTX PowerPoint PPTX 格式.
	DocPPTX DocumentFormat = "pptx"
	// DocMarkdown Markdown 格式.
	DocMarkdown DocumentFormat = "markdown"
	// DocHTML HTML 格式.
	DocHTML DocumentFormat = "html"
	// DocTXT 纯文本格式.
	DocTXT DocumentFormat = "txt"
	// DocCSV CSV 格式.
	DocCSV DocumentFormat = "csv"
)

// ========== 3D模型格式定义 ==========

// Model3DFormat 3D模型格式.
type Model3DFormat string

const (
	// ModelGLTF glTF 2.0 格式.
	ModelGLTF Model3DFormat = "gltf"
	// ModelGLB glTF 二进制格式.
	ModelGLB Model3DFormat = "glb"
	// ModelOBJ OBJ 格式.
	ModelOBJ Model3DFormat = "obj"
	// ModelSTL STL 格式.
	ModelSTL Model3DFormat = "stl"
	// ModelFBX FBX 格式.
	ModelFBX Model3DFormat = "fbx"
	// ModelPLY PLY 格式.
	ModelPLY Model3DFormat = "ply"
)

// ========== 缩略图尺寸预设 ==========

// ThumbnailSize 缩略图尺寸预设.
type ThumbnailSize string

const (
	// SizeSmall 小缩略图 150x150.
	SizeSmall ThumbnailSize = "small"
	// SizeMedium 中缩略图 300x300.
	SizeMedium ThumbnailSize = "medium"
	// SizeLarge 大缩略图 600x600.
	SizeLarge ThumbnailSize = "large"
	// SizeXL 超大缩略图 1200x1200.
	SizeXL ThumbnailSize = "xl"
)

// ========== 核心数据结构 ==========

// PreviewRequest 预览请求.
type PreviewRequest struct {
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// FileType 文件类型（可选，自动检测）.
	FileType FileType `json:"file_type,omitempty"`
	// Width 输出宽度.
	Width int `json:"width,omitempty"`
	// Height 输出高度.
	Height int `json:"height,omitempty"`
	// Quality 输出质量 (1-100).
	Quality int `json:"quality,omitempty"`
	// PageNumber 文档页码（从1开始）.
	PageNumber int `json:"page_number,omitempty"`
	// Timestamp 视频时间戳（秒）.
	Timestamp float64 `json:"timestamp,omitempty"`
	// Format 输出格式.
	Format string `json:"format,omitempty"`
	// Password 加密文件的解锁密码（用于加密 PDF 等）.
	Password string `json:"password,omitempty"`
}

// PreviewResult 预览结果.
type PreviewResult struct {
	// FilePath 源文件路径.
	FilePath string `json:"file_path"`
	// FileType 文件类型.
	FileType FileType `json:"file_type"`
	// PreviewPath 预览文件路径.
	PreviewPath string `json:"preview_path"`
	// ContentType MIME 类型.
	ContentType string `json:"content_type"`
	// Width 输出宽度.
	Width int `json:"width,omitempty"`
	// Height 输出高度.
	Height int `json:"height,omitempty"`
	// FileSize 预览文件大小（字节）.
	FileSize int64 `json:"file_size"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
	// Duration 视频/音频时长（秒）.
	Duration float64 `json:"duration,omitempty"`
	// PageCount 文档总页数.
	PageCount int `json:"page_count,omitempty"`
	// PageNumber 当前页码.
	PageNumber int `json:"page_number,omitempty"`
	// Metadata 附加元数据.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ImageInfo 图片信息.
type ImageInfo struct {
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Format 图片格式.
	Format ImageFormat `json:"format"`
	// Width 原始宽度.
	Width int `json:"width"`
	// Height 原始高度.
	Height int `json:"height"`
	// FileSize 文件大小（字节）.
	FileSize int64 `json:"file_size"`
	// ColorSpace 色彩空间.
	ColorSpace string `json:"color_space,omitempty"`
	// BitDepth 位深度.
	BitDepth int `json:"bit_depth,omitempty"`
	// HasAlpha 是否有透明通道.
	HasAlpha bool `json:"has_alpha"`
	// EXIF EXIF 信息.
	EXIF map[string]string `json:"exif,omitempty"`
}

// VideoInfo 视频信息.
type VideoInfo struct {
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Format 视频格式.
	Format VideoFormat `json:"format"`
	// Width 视频宽度.
	Width int `json:"width"`
	// Height 视频高度.
	Height int `json:"height"`
	// Duration 时长（秒）.
	Duration float64 `json:"duration"`
	// Bitrate 比特率（bps）.
	Bitrate int64 `json:"bitrate"`
	// FPS 帧率.
	FPS float64 `json:"fps"`
	// Codec 视频编码.
	Codec string `json:"codec"`
	// AudioCodec 音频编码.
	AudioCodec string `json:"audio_codec,omitempty"`
	// FileSize 文件大小（字节）.
	FileSize int64 `json:"file_size"`
}

// AudioInfo 音频信息.
type AudioInfo struct {
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Format 音频格式.
	Format AudioFormat `json:"format"`
	// Duration 时长（秒）.
	Duration float64 `json:"duration"`
	// SampleRate 采样率（Hz）.
	SampleRate int `json:"sample_rate"`
	// Channels 声道数.
	Channels int `json:"channels"`
	// Bitrate 比特率（bps）.
	Bitrate int64 `json:"bitrate"`
	// Codec 音频编码.
	Codec string `json:"codec"`
	// FileSize 文件大小（字节）.
	FileSize int64 `json:"file_size"`
}

// DocumentInfo 文档信息.
type DocumentInfo struct {
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Format 文档格式.
	Format DocumentFormat `json:"format"`
	// PageCount 总页数.
	PageCount int `json:"page_count"`
	// FileSize 文件大小（字节）.
	FileSize int64 `json:"file_size"`
	// Title 文档标题.
	Title string `json:"title,omitempty"`
	// Author 作者.
	Author string `json:"author,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// Model3DInfo 3D模型信息.
type Model3DInfo struct {
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Format 模型格式.
	Format Model3DFormat `json:"format"`
	// VertexCount 顶点数.
	VertexCount int `json:"vertex_count"`
	// FaceCount 面数.
	FaceCount int `json:"face_count"`
	// HasTexture 是否有纹理.
	HasTexture bool `json:"has_texture"`
	// FileSize 文件大小（字节）.
	FileSize int64 `json:"file_size"`
}

// WaveformData 波形数据.
type WaveformData struct {
	// Samples 采样点数据（归一化到 0-1）.
	Samples []float64 `json:"samples"`
	// Duration 时长（秒）.
	Duration float64 `json:"duration"`
	// SampleRate 采样率.
	SampleRate int `json:"sample_rate"`
	// Channels 声道数.
	Channels int `json:"channels"`
	// Peaks 波峰数据（用于快速渲染）.
	Peaks []float64 `json:"peaks,omitempty"`
}

// KeyFrame 关键帧信息.
type KeyFrame struct {
	// Timestamp 时间戳（秒）.
	Timestamp float64 `json:"timestamp"`
	// PreviewPath 预览图路径.
	PreviewPath string `json:"preview_path"`
	// Width 宽度.
	Width int `json:"width"`
	// Height 高度.
	Height int `json:"height"`
	// IsKeyFrame 是否为关键帧.
	IsKeyFrame bool `json:"is_key_frame"`
}

// CacheEntry 缓存条目.
type CacheEntry struct {
	// Key 缓存键.
	Key string `json:"key"`
	// FilePath 预览文件路径.
	FilePath string `json:"file_path"`
	// SourcePath 源文件路径.
	SourcePath string `json:"source_path"`
	// FileSize 文件大小（字节）.
	FileSize int64 `json:"file_size"`
	// ContentType MIME 类型.
	ContentType string `json:"content_type"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// AccessedAt 最后访问时间.
	AccessedAt time.Time `json:"accessed_at"`
	// AccessCount 访问次数.
	AccessCount int `json:"access_count"`
	// ExpiresAt 过期时间.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// SourceModTime 源文件修改时间（用于失效检测）.
	SourceModTime time.Time `json:"source_mod_time"`
}

// CacheStats 缓存统计.
type CacheStats struct {
	// TotalEntries 总条目数.
	TotalEntries int `json:"total_entries"`
	// TotalSize 总大小（字节）.
	TotalSize int64 `json:"total_size"`
	// HitCount 命中次数.
	HitCount int64 `json:"hit_count"`
	// MissCount 未命中次数.
	MissCount int64 `json:"miss_count"`
	// EvictionCount 淘汰次数.
	EvictionCount int64 `json:"eviction_count"`
	// MaxSize 最大缓存大小（字节）.
	MaxSize int64 `json:"max_size"`
}

// CacheConfig 缓存配置.
type CacheConfig struct {
	// CacheDir 缓存目录.
	CacheDir string `json:"cache_dir"`
	// MaxSize 最大缓存大小（字节），默认 1GB.
	MaxSize int64 `json:"max_size"`
	// MaxEntries 最大条目数，默认 10000.
	MaxEntries int `json:"max_entries"`
	// DefaultTTL 默认过期时间，默认 7天.
	DefaultTTL time.Duration `json:"default_ttl"`
	// CleanupInterval 清理间隔，默认 1小时.
	CleanupInterval time.Duration `json:"cleanup_interval"`
	// ThumbnailSizes 要预生成的缩略图尺寸.
	ThumbnailSizes []ThumbnailSize `json:"thumbnail_sizes"`
}

// PreviewConfig 预览配置.
type PreviewConfig struct {
	// Cache 缓存配置.
	Cache CacheConfig `json:"cache"`
	// FFmpegPath FFmpeg 路径，默认 "ffmpeg".
	FFmpegPath string `json:"ffmpeg_path"`
	// FFprobePath FFprobe 路径，默认 "ffprobe".
	FFprobePath string `json:"ffprobe_path"`
	// LibreOfficePath LibreOffice 路径，默认 "libreoffice".
	LibreOfficePath string `json:"libreoffice_path"`
	// MaxFileSize 最大可预览文件大小（字节），默认 500MB.
	MaxFileSize int64 `json:"max_file_size"`
	// MaxConcurrent 最大并发生成数，默认 4.
	MaxConcurrent int `json:"max_concurrent"`
	// DefaultQuality 默认图片质量，默认 80.
	DefaultQuality int `json:"default_quality"`
	// DefaultThumbnailWidth 默认缩略图宽度，默认 300.
	DefaultThumbnailWidth int `json:"default_thumbnail_width"`
	// DefaultThumbnailHeight 默认缩略图高度，默认 300.
	DefaultThumbnailHeight int `json:"default_thumbnail_height"`
}

// DefaultPreviewConfig 返回默认预览配置.
func DefaultPreviewConfig() *PreviewConfig {
	return &PreviewConfig{
		Cache: CacheConfig{
			CacheDir:        "/var/cache/nas-os/preview",
			MaxSize:         1 << 30, // 1GB
			MaxEntries:      10000,
			DefaultTTL:      7 * 24 * time.Hour,
			CleanupInterval: 1 * time.Hour,
			ThumbnailSizes:  []ThumbnailSize{SizeSmall, SizeMedium, SizeLarge},
		},
		FFmpegPath:             "ffmpeg",
		FFprobePath:            "ffprobe",
		LibreOfficePath:        "libreoffice",
		MaxFileSize:            500 << 20, // 500MB
		MaxConcurrent:          4,
		DefaultQuality:         80,
		DefaultThumbnailWidth:  300,
		DefaultThumbnailHeight: 300,
	}
}

// ========== 工具函数 ==========

// DetectFileType 根据文件扩展名检测文件类型.
func DetectFileType(filename string) FileType {
	ext := getExtLower(filename)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".heif",
		".bmp", ".tiff", ".tif", ".cr2", ".nef", ".arw", ".dng", ".raw":
		return FileTypeImage
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm", ".flv", ".m4v", ".ts":
		return FileTypeVideo
	case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".opus", ".m4a", ".wma":
		return FileTypeAudio
	case ".pdf", ".docx", ".xlsx", ".pptx", ".doc", ".xls", ".ppt",
		".md", ".html", ".htm", ".txt", ".csv", ".rtf":
		return FileTypeDocument
	case ".gltf", ".glb", ".obj", ".stl", ".fbx", ".ply":
		return FileType3D
	default:
		return FileTypeUnknown
	}
}

// DetectImageFormat 检测图片格式.
func DetectImageFormat(filename string) ImageFormat {
	ext := getExtLower(filename)
	switch ext {
	case ".jpg", ".jpeg":
		return FormatJPEG
	case ".png":
		return FormatPNG
	case ".gif":
		return FormatGIF
	case ".webp":
		return FormatWebP
	case ".heic", ".heif":
		return FormatHEIC
	case ".bmp":
		return FormatBMP
	case ".tiff", ".tif":
		return FormatTIFF
	case ".cr2", ".nef", ".arw", ".dng", ".raw":
		return FormatRAW
	default:
		return ""
	}
}

// DetectVideoFormat 检测视频格式.
func DetectVideoFormat(filename string) VideoFormat {
	ext := getExtLower(filename)
	switch ext {
	case ".mp4":
		return VideoMP4
	case ".mkv":
		return VideoMKV
	case ".avi":
		return VideoAVI
	case ".mov":
		return VideoMOV
	case ".wmv":
		return VideoWMV
	case ".webm":
		return VideoWebM
	case ".flv":
		return VideoFLV
	default:
		return ""
	}
}

// DetectAudioFormat 检测音频格式.
func DetectAudioFormat(filename string) AudioFormat {
	ext := getExtLower(filename)
	switch ext {
	case ".mp3":
		return AudioMP3
	case ".flac":
		return AudioFLAC
	case ".wav":
		return AudioWAV
	case ".aac":
		return AudioAAC
	case ".ogg":
		return AudioOGG
	case ".opus":
		return AudioOPUS
	case ".m4a":
		return AudioM4A
	default:
		return ""
	}
}

// DetectDocumentFormat 检测文档格式.
func DetectDocumentFormat(filename string) DocumentFormat {
	ext := getExtLower(filename)
	switch ext {
	case ".pdf":
		return DocPDF
	case ".docx":
		return DocDOCX
	case ".xlsx":
		return DocXLSX
	case ".pptx":
		return DocPPTX
	case ".md", ".markdown":
		return DocMarkdown
	case ".html", ".htm":
		return DocHTML
	case ".txt":
		return DocTXT
	case ".csv":
		return DocCSV
	default:
		return ""
	}
}

// DetectModel3DFormat 检测3D模型格式.
func DetectModel3DFormat(filename string) Model3DFormat {
	ext := getExtLower(filename)
	switch ext {
	case ".gltf":
		return ModelGLTF
	case ".glb":
		return ModelGLB
	case ".obj":
		return ModelOBJ
	case ".stl":
		return ModelSTL
	case ".fbx":
		return ModelFBX
	case ".ply":
		return ModelPLY
	default:
		return ""
	}
}

// GetThumbnailDimensions 根据预设获取缩略图尺寸.
func GetThumbnailDimensions(size ThumbnailSize) (width, height int) {
	switch size {
	case SizeSmall:
		return 150, 150
	case SizeMedium:
		return 300, 300
	case SizeLarge:
		return 600, 600
	case SizeXL:
		return 1200, 1200
	default:
		return 300, 300
	}
}

// IsImageSupported 检查图片格式是否支持.
func IsImageSupported(filename string) bool {
	return DetectImageFormat(filename) != ""
}

// IsVideoSupported 检查视频格式是否支持.
func IsVideoSupported(filename string) bool {
	return DetectVideoFormat(filename) != ""
}

// IsAudioSupported 检查音频格式是否支持.
func IsAudioSupported(filename string) bool {
	return DetectAudioFormat(filename) != ""
}

// IsDocumentSupported 检查文档格式是否支持.
func IsDocumentSupported(filename string) bool {
	return DetectDocumentFormat(filename) != ""
}

// IsModel3DSupported 检查3D模型格式是否支持.
func IsModel3DSupported(filename string) bool {
	return DetectModel3DFormat(filename) != ""
}

// getExtLower 获取小写文件扩展名.
func getExtLower(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
		if filename[i] == '/' || filename[i] == '\\' {
			break
		}
	}
	return ""
}
