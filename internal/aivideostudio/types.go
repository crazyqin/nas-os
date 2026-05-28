// Package aivideostudio 提供AI视频工作室功能，包括智能视频转码、场景检测、
// 视频摘要生成、质量增强、字幕自动生成等。支持H.264/H.265/AV1多编码格式，
// 对标群晖Surveillance Station视频管理与TrueNAS Media处理能力。
package aivideostudio

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

// AI视频工作室相关错误.
var (
	// ErrVideoNotFound 视频文件不存在.
	ErrVideoNotFound = errors.New("视频文件不存在")
	// ErrTaskNotFound 任务不存在.
	ErrTaskNotFound = errors.New("任务不存在")
	// ErrTranscodeFailed 转码失败.
	ErrTranscodeFailed = errors.New("转码失败")
	// ErrUnsupportedCodec 不支持的编码格式.
	ErrUnsupportedCodec = errors.New("不支持的编码格式")
	// ErrSceneDetectionFailed 场景检测失败.
	ErrSceneDetectionFailed = errors.New("场景检测失败")
	// ErrThumbnailFailed 缩略图生成失败.
	ErrThumbnailFailed = errors.New("缩略图生成失败")
	// ErrSummaryFailed 摘要生成失败.
	ErrSummaryFailed = errors.New("摘要生成失败")
	// ErrEnhancementFailed 视频增强失败.
	ErrEnhancementFailed = errors.New("视频增强失败")
	// ErrSubtitleFailed 字幕生成失败.
	ErrSubtitleFailed = errors.New("字幕生成失败")
	// ErrInvalidPath 无效路径.
	ErrInvalidPath = errors.New("无效路径")
	// ErrBatchFailed 批量任务失败.
	ErrBatchFailed = errors.New("批量任务失败")
	// ErrUnsupportedFormat 不支持的视频格式.
	ErrUnsupportedFormat = errors.New("不支持的视频格式")
	// ErrTaskAlreadyRunning 任务已在运行中.
	ErrTaskAlreadyRunning = errors.New("任务已在运行中")
	// ErrInsufficientSpace 磁盘空间不足.
	ErrInsufficientSpace = errors.New("磁盘空间不足")
)

// ========== 编码格式 ==========

// VideoCodec 视频编码格式.
type VideoCodec string

const (
	// CodecH264 H.264/AVC编码.
	CodecH264 VideoCodec = "h264"
	// CodecH265 H.265/HEVC编码.
	CodecH265 VideoCodec = "h265"
	// CodecAV1 AV1编码.
	CodecAV1 VideoCodec = "av1"
	// CodecVP9 VP9编码.
	CodecVP9 VideoCodec = "vp9"
)

// ========== 任务状态 ==========

// TaskStatus 任务状态.
type TaskStatus string

const (
	// TaskStatusPending 等待执行.
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning 执行中.
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusCompleted 已完成.
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 失败.
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled 已取消.
	TaskStatusCancelled TaskStatus = "cancelled"
	// TaskStatusPaused 已暂停.
	TaskStatusPaused TaskStatus = "paused"
)

// ========== 任务类型 ==========

// TaskType 任务类型.
type TaskType string

const (
	// TaskTypeTranscode 转码任务.
	TaskTypeTranscode TaskType = "transcode"
	// TaskTypeSceneDetection 场景检测任务.
	TaskTypeSceneDetection TaskType = "scene_detection"
	// TaskTypeThumbnail 缩略图生成任务.
	TaskTypeThumbnail TaskType = "thumbnail"
	// TaskTypePreview 预览生成任务.
	TaskTypePreview TaskType = "preview"
	// TaskTypeSummary 视频摘要任务.
	TaskTypeSummary TaskType = "summary"
	// TaskTypeEnhancement 视频增强任务.
	TaskTypeEnhancement TaskType = "enhancement"
	// TaskTypeSubtitle 字幕生成任务.
	TaskTypeSubtitle TaskType = "subtitle"
	// TaskTypeMetadata 元数据分析任务.
	TaskTypeMetadata TaskType = "metadata"
	// TaskTypeBatchConvert 批量转换任务.
	TaskTypeBatchConvert TaskType = "batch_convert"
)

// ========== 增强类型 ==========

// EnhancementType 视频增强类型.
type EnhancementType string

const (
	// EnhancementSuperResolution 超分辨率增强.
	EnhancementSuperResolution EnhancementType = "super_resolution"
	// EnhancementDenoise 降噪处理.
	EnhancementDenoise EnhancementType = "denoise"
	// EnhancementSharpen 锐化处理.
	EnhancementSharpen EnhancementType = "sharpen"
	// EnhancementColorCorrect 色彩校正.
	EnhancementColorCorrect EnhancementType = "color_correct"
	// EnhancementStabilize 防抖处理.
	EnhancementStabilize EnhancementType = "stabilize"
	// EnhancementHDR HDR增强.
	EnhancementHDR EnhancementType = "hdr"
)

// ========== 分辨率预设 ==========

// ResolutionPreset 分辨率预设.
type ResolutionPreset string

const (
	// Resolution4K 4K分辨率 (3840x2160).
	Resolution4K ResolutionPreset = "4k"
	// Resolution1080p 1080p分辨率 (1920x1080).
	Resolution1080p ResolutionPreset = "1080p"
	// Resolution720p 720p分辨率 (1280x720).
	Resolution720p ResolutionPreset = "720p"
	// Resolution480p 480p分辨率 (854x480).
	Resolution480p ResolutionPreset = "480p"
)

// ========== 数据结构 ==========

// VideoInfo 视频文件信息.
type VideoInfo struct {
	// ID 视频唯一标识.
	ID string `json:"id"`
	// FileName 文件名.
	FileName string `json:"file_name"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// FileSize 文件大小(字节).
	FileSize int64 `json:"file_size"`
	// Duration 时长(秒).
	Duration float64 `json:"duration"`
	// Width 宽度(像素).
	Width int `json:"width"`
	// Height 高度(像素).
	Height int `json:"height"`
	// Codec 编码格式.
	Codec VideoCodec `json:"codec"`
	// Bitrate 码率(bps).
	Bitrate int64 `json:"bitrate"`
	// FrameRate 帧率.
	FrameRate float64 `json:"frame_rate"`
	// Format 容器格式.
	Format string `json:"format"`
	// HasAudio 是否有音频.
	HasAudio bool `json:"has_audio"`
	// AudioCodec 音频编码.
	AudioCodec string `json:"audio_codec,omitempty"`
	// AudioBitrate 音频码率.
	AudioBitrate int64 `json:"audio_bitrate,omitempty"`
	// Subtitles 已有字幕列表.
	Subtitles []string `json:"subtitles,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// TranscodeProfile 转码配置.
type TranscodeProfile struct {
	// Name 配置名称.
	Name string `json:"name"`
	// TargetCodec 目标编码格式.
	TargetCodec VideoCodec `json:"target_codec"`
	// TargetResolution 目标分辨率.
	TargetResolution ResolutionPreset `json:"target_resolution"`
	// TargetBitrate 目标码率(bps).
	TargetBitrate int64 `json:"target_bitrate"`
	// Quality 质量等级(0-100).
	Quality int `json:"quality"`
	// TwoPass 是否启用两遍编码.
	TwoPass bool `json:"two_pass"`
	// HardwareAccel 是否启用硬件加速.
	HardwareAccel bool `json:"hardware_accel"`
	// PreserveAudio 是否保留原始音频.
	PreserveAudio bool `json:"preserve_audio"`
	// MaxFps 最大帧率限制.
	MaxFps float64 `json:"max_fps,omitempty"`
}

// SceneInfo 场景信息.
type SceneInfo struct {
	// SceneIndex 场景序号.
	SceneIndex int `json:"scene_index"`
	// StartTime 开始时间(秒).
	StartTime float64 `json:"start_time"`
	// EndTime 结束时间(秒).
	EndTime float64 `json:"end_time"`
	// Duration 时长(秒).
	Duration float64 `json:"duration"`
	// Description 场景描述.
	Description string `json:"description,omitempty"`
	// Tags 场景标签.
	Tags []string `json:"tags,omitempty"`
	// KeyFrameTime 关键帧时间点.
	KeyFrameTime float64 `json:"key_frame_time"`
	// ThumbnailPath 缩略图路径.
	ThumbnailPath string `json:"thumbnail_path,omitempty"`
}

// VideoSummary 视频摘要.
type VideoSummary struct {
	// VideoID 视频ID.
	VideoID string `json:"video_id"`
	// Title 自动生成的标题.
	Title string `json:"title"`
	// Summary 摘要内容.
	Summary string `json:"summary"`
	// KeyMoments 关键时刻列表.
	KeyMoments []KeyMoment `json:"key_moments"`
	// Topics 主题标签.
	Topics []string `json:"topics"`
	// Language 识别的语言.
	Language string `json:"language"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
}

// KeyMoment 关键时刻.
type KeyMoment struct {
	// Time 时间点(秒).
	Time float64 `json:"time"`
	// Description 描述.
	Description string `json:"description"`
	// Importance 重要性(0-1).
	Importance float64 `json:"importance"`
}

// SubtitleTrack 字幕轨道.
type SubtitleTrack struct {
	// ID 字幕ID.
	ID string `json:"id"`
	// VideoID 视频ID.
	VideoID string `json:"video_id"`
	// Language 语言.
	Language string `json:"language"`
	// Format 格式(srt/vtt/ass).
	Format string `json:"format"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Entries 字幕条目.
	Entries []SubtitleEntry `json:"entries"`
	// GeneratedBy 生成方式(ai/manual).
	GeneratedBy string `json:"generated_by"`
	// Accuracy 准确度(0-100).
	Accuracy float64 `json:"accuracy,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
}

// SubtitleEntry 字幕条目.
type SubtitleEntry struct {
	// Index 序号.
	Index int `json:"index"`
	// StartTime 开始时间(毫秒).
	StartTime int64 `json:"start_time"`
	// EndTime 结束时间(毫秒).
	EndTime int64 `json:"end_time"`
	// Text 文本内容.
	Text string `json:"text"`
	// Speaker 说话人(可选).
	Speaker string `json:"speaker,omitempty"`
	// Confidence 置信度(0-1).
	Confidence float64 `json:"confidence,omitempty"`
}

// VideoTask 视频处理任务.
type VideoTask struct {
	// ID 任务ID.
	ID string `json:"id"`
	// Type 任务类型.
	Type TaskType `json:"type"`
	// Status 任务状态.
	Status TaskStatus `json:"status"`
	// VideoID 关联的视频ID.
	VideoID string `json:"video_id"`
	// InputPath 输入文件路径.
	InputPath string `json:"input_path"`
	// OutputPath 输出文件路径.
	OutputPath string `json:"output_path,omitempty"`
	// Progress 进度(0-100).
	Progress float64 `json:"progress"`
	// ErrorMessage 错误信息.
	ErrorMessage string `json:"error_message,omitempty"`
	// Params 任务参数.
	Params map[string]interface{} `json:"params,omitempty"`
	// Result 任务结果.
	Result interface{} `json:"result,omitempty"`
	// StartedAt 开始时间.
	StartedAt *time.Time `json:"started_at,omitempty"`
	// CompletedAt 完成时间.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// ThumbnailConfig 缩略图配置.
type ThumbnailConfig struct {
	// Width 宽度.
	Width int `json:"width"`
	// Height 高度.
	Height int `json:"height"`
	// Quality 质量(0-100).
	Quality int `json:"quality"`
	// Count 生成数量.
	Count int `json:"count"`
	// Interval 间隔(秒), 0表示均匀分布.
	Interval float64 `json:"interval"`
	// Format 格式(jpg/png/webp).
	Format string `json:"format"`
	// SpriteSheet 是否生成雪碧图.
	SpriteSheet bool `json:"sprite_sheet"`
}

// PreviewConfig 预览配置.
type PreviewConfig struct {
	// Duration 预览时长(秒).
	Duration float64 `json:"duration"`
	// Quality 质量(0-100).
	Quality int `json:"quality"`
	// IncludeAudio 是否包含音频.
	IncludeAudio bool `json:"include_audio"`
	// Watermark 水印文本.
	Watermark string `json:"watermark,omitempty"`
}

// MetadataAnalysis 元数据分析结果.
type MetadataAnalysis struct {
	// VideoID 视频ID.
	VideoID string `json:"video_id"`
	// BasicInfo 基本信息.
	BasicInfo VideoInfo `json:"basic_info"`
	// QualityScore 质量评分(0-100).
	QualityScore float64 `json:"quality_score"`
	// CompressionRatio 压缩比.
	CompressionRatio float64 `json:"compression_ratio"`
	// BitrateAnalysis 码率分析.
	BitrateAnalysis BitrateAnalysis `json:"bitrate_analysis"`
	// ResolutionAnalysis 分辨率分析.
	ResolutionAnalysis ResolutionAnalysis `json:"resolution_analysis"`
	// AudioAnalysis 音频分析.
	AudioAnalysis AudioAnalysis `json:"audio_analysis"`
	// Recommendations 优化建议.
	Recommendations []string `json:"recommendations"`
	// AnalyzedAt 分析时间.
	AnalyzedAt time.Time `json:"analyzed_at"`
}

// BitrateAnalysis 码率分析.
type BitrateAnalysis struct {
	// AverageBitrate 平均码率.
	AverageBitrate int64 `json:"average_bitrate"`
	// PeakBitrate 峰值码率.
	PeakBitrate int64 `json:"peak_bitrate"`
	// MinBitrate 最低码率.
	MinBitrate int64 `json:"min_bitrate"`
	// BitrateVariance 码率方差.
	BitrateVariance float64 `json:"bitrate_variance"`
	// IsVBR 是否为可变码率.
	IsVBR bool `json:"is_vbr"`
}

// ResolutionAnalysis 分辨率分析.
type ResolutionAnalysis struct {
	// NativeResolution 原生分辨率.
	NativeResolution string `json:"native_resolution"`
	// EffectiveResolution 有效分辨率.
	EffectiveResolution string `json:"effective_resolution"`
	// IsUpscaled 是否为上采样.
	IsUpscaled bool `json:"is_upscaled"`
	// PixelFormat 像素格式.
	PixelFormat string `json:"pixel_format"`
	// ColorSpace 色彩空间.
	ColorSpace string `json:"color_space"`
}

// AudioAnalysis 音频分析.
type AudioAnalysis struct {
	// Channels 声道数.
	Channels int `json:"channels"`
	// SampleRate 采样率.
	SampleRate int `json:"sample_rate"`
	// BitDepth 位深度.
	BitDepth int `json:"bit_depth"`
	// Loudness 响度(LUFS).
	Loudness float64 `json:"loudness"`
	// DynamicRange 动态范围.
	DynamicRange float64 `json:"dynamic_range"`
}

// BatchConvertRequest 批量转换请求.
type BatchConvertRequest struct {
	// InputPaths 输入文件路径列表.
	InputPaths []string `json:"input_paths"`
	// OutputDir 输出目录.
	OutputDir string `json:"output_dir"`
	// Profile 转码配置.
	Profile TranscodeProfile `json:"profile"`
	// NamingPattern 命名模式, {name}为原文件名, {ext}为新扩展名.
	NamingPattern string `json:"naming_pattern"`
	// Overwrite 是否覆盖已有文件.
	Overwrite bool `json:"overwrite"`
}

// BatchConvertResult 批量转换结果.
type BatchConvertResult struct {
	// TotalCount 总数.
	TotalCount int `json:"total_count"`
	// SuccessCount 成功数.
	SuccessCount int `json:"success_count"`
	// FailedCount 失败数.
	FailedCount int `json:"failed_count"`
	// SkippedCount 跳过数.
	SkippedCount int `json:"skipped_count"`
	// Results 各文件结果.
	Results []SingleConvertResult `json:"results"`
	// StartedAt 开始时间.
	StartedAt time.Time `json:"started_at"`
	// CompletedAt 完成时间.
	CompletedAt time.Time `json:"completed_at"`
}

// SingleConvertResult 单个文件转换结果.
type SingleConvertResult struct {
	// InputPath 输入路径.
	InputPath string `json:"input_path"`
	// OutputPath 输出路径.
	OutputPath string `json:"output_path,omitempty"`
	// Status 状态.
	Status TaskStatus `json:"status"`
	// ErrorMessage 错误信息.
	ErrorMessage string `json:"error_message,omitempty"`
	// Duration 处理时长(秒).
	Duration float64 `json:"duration"`
}

// StatsInfo 系统统计信息.
type StatsInfo struct {
	// TotalVideos 视频总数.
	TotalVideos int `json:"total_videos"`
	// TotalTasks 任务总数.
	TotalTasks int `json:"total_tasks"`
	// RunningTasks 运行中任务数.
	RunningTasks int `json:"running_tasks"`
	// CompletedTasks 已完成任务数.
	CompletedTasks int `json:"completed_tasks"`
	// FailedTasks 失败任务数.
	FailedTasks int `json:"failed_tasks"`
	// TotalStorageUsed 总存储使用(字节).
	TotalStorageUsed int64 `json:"total_storage_used"`
	// GPUNam GPU名称.
	GPUName string `json:"gpu_name,omitempty"`
	// GPUUtilization GPU使用率.
	GPUUtilization float64 `json:"gpu_utilization,omitempty"`
}
