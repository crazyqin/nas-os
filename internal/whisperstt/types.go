// Package whisperstt 提供 Whisper 本地语音转文字服务
// 模型管理、音频转录、多语言支持、字幕生成、音频预处理、转录队列管理
package whisperstt

import (
	"time"
)

// ========== Whisper 模型 ==========

// WhisperModel Whisper 模型
type WhisperModel struct {
	ID            string        `json:"id"`            // 唯一标识
	Name          string        `json:"name"`          // 模型名称 (tiny/base/small/medium/large)
	Size          int64         `json:"size"`          // 模型大小 (字节)
	Languages     []string      `json:"languages"`     // 支持的语言列表
	IsLoaded      bool          `json:"isLoaded"`      // 是否已加载
	GPUSupported  bool          `json:"gpuSupported"`  // 是否支持 GPU
	DownloadURL   string        `json:"downloadUrl"`   // 下载地址
	LocalPath     string        `json:"localPath"`     // 本地路径
	LoadTime      time.Time     `json:"loadTime"`      // 加载时间
	LastUsed      time.Time     `json:"lastUsed"`      // 最后使用时间
}

// ModelSize 模型大小枚举
type ModelSize string

const (
	ModelSizeTiny   ModelSize = "tiny"   // ~39M
	ModelSizeBase   ModelSize = "base"   // ~74M
	ModelSizeSmall  ModelSize = "small"  // ~244M
	ModelSizeMedium ModelSize = "medium" // ~769M
	ModelSizeLarge  ModelSize = "large"  // ~1550M
)

// ========== 转录任务 ==========

// TranscriptionJob 转录任务
type TranscriptionJob struct {
	ID          string              `json:"id"`          // 唯一标识
	FilePath    string              `json:"filePath"`    // 文件路径
	FileName    string              `json:"fileName"`    // 文件名
	Status      JobStatus           `json:"status"`      // 任务状态
	Progress    float64             `json:"progress"`    // 进度 (0-100)
	Priority    int                 `json:"priority"`    // 优先级 (越高越优先)
	Options     TranscriptionOptions `json:"options"`     // 转录选项
	ErrorMsg    string              `json:"errorMsg"`    // 错误信息
	CreatedAt   time.Time           `json:"createdAt"`   // 创建时间
	StartedAt   *time.Time          `json:"startedAt"`   // 开始时间
	CompletedAt *time.Time          `json:"completedAt"` // 完成时间
}

// JobStatus 任务状态
type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"     // 等待中
	JobStatusProcessing JobStatus = "processing" // 处理中
	JobStatusCompleted  JobStatus = "completed"  // 已完成
	JobStatusFailed     JobStatus = "failed"     // 失败
	JobStatusCancelled  JobStatus = "cancelled"  // 已取消
)

// TranscriptionOptions 转录选项
type TranscriptionOptions struct {
	Language       string   `json:"language"`       // 语言代码 (空表示自动检测)
	Task           string   `json:"task"`           // 任务类型 (transcribe/translate)
	Format         string   `json:"format"`         // 输出格式 (text/srt/vtt/ass)
	WordTimestamps bool     `json:"wordTimestamps"` // 是否输出词级时间戳
	BeamSize       int      `json:"beamSize"`       // 束搜索大小
	BestOf         int      `json:"bestOf"`         // 生成候选数
	Temperature    float64  `json:"temperature"`    // 采样温度
	InitialPrompt  string   `json:"initialPrompt"`  // 初始提示词
	SuppressTokens []int    `json:"suppressTokens"` // 抑制的 token ID
}

// ========== 转录结果 ==========

// TranscriptionResult 转录结果
type TranscriptionResult struct {
	ID          string          `json:"id"`          // 唯一标识
	JobID       string          `json:"jobId"`       // 关联任务 ID
	Text        string          `json:"text"`        // 完整文本
	Language    string          `json:"language"`    // 检测到的语言
	Confidence  float64         `json:"confidence"`  // 置信度
	Words       []WordTimestamp `json:"words"`       // 词级时间戳
	Segments    []Segment       `json:"segments"`    // 段落列表
	Duration    float64         `json:"duration"`    // 音频时长 (秒)
	ProcessedAt time.Time       `json:"processedAt"` // 处理时间
}

// WordTimestamp 词级时间戳
type WordTimestamp struct {
	Word       string  `json:"word"`       // 单词
	Start      float64 `json:"start"`      // 开始时间 (秒)
	End        float64 `json:"end"`        // 结束时间 (秒)
	Confidence float64 `json:"confidence"` // 置信度
}

// Segment 段落
type Segment struct {
	ID        int     `json:"id"`        // 段落 ID
	Start     float64 `json:"start"`     // 开始时间 (秒)
	End       float64 `json:"end"`       // 结束时间 (秒)
	Text      string  `json:"text"`      // 文本内容
	Speaker   string  `json:"speaker"`   // 说话人 (可选)
	Words     []WordTimestamp `json:"words"` // 段落内的词级时间戳
}

// ========== 字幕格式 ==========

// SubtitleFormat 字幕格式
type SubtitleFormat struct {
	Type     SubtitleType `json:"type"`     // 字幕类型
	Content  string       `json:"content"`  // 字幕内容
	FilePath string       `json:"filePath"` // 文件路径
}

// SubtitleType 字幕类型
type SubtitleType string

const (
	SubtitleTypeSRT SubtitleType = "srt" // SRT 格式
	SubtitleTypeVTT SubtitleType = "vtt" // VTT 格式
	SubtitleTypeASS SubtitleType = "ass" // ASS 格式
)

// ========== 音频预处理 ==========

// AudioPreprocessConfig 音频预处理配置
type AudioPreprocessConfig struct {
	DenoiseEnabled bool    `json:"denoiseEnabled"` // 是否启用降噪
	VADEnabled     bool    `json:"vadEnabled"`     // 是否启用 VAD 语音活动检测
	SampleRate     int     `json:"sampleRate"`     // 采样率 (Hz)
	Format         string  `json:"format"`         // 目标格式 (wav/mp3/flac)
	BitRate        int     `json:"bitRate"`        // 比特率 (kbps)
	VADThreshold   float64 `json:"vadThreshold"`   // VAD 阈值 (0-1)
}

// DefaultPreprocessConfig 默认预处理配置
func DefaultPreprocessConfig() AudioPreprocessConfig {
	return AudioPreprocessConfig{
		DenoiseEnabled: true,
		VADEnabled:     true,
		SampleRate:     16000,
		Format:         "wav",
		BitRate:        128,
		VADThreshold:   0.5,
	}
}

// ========== 队列统计 ==========

// QueueStats 队列统计
type QueueStats struct {
	QueueLength     int           `json:"queueLength"`     // 队列长度
	Processing      int           `json:"processing"`      // 处理中数
	Completed       int           `json:"completed"`       // 完成数
	Failed          int           `json:"failed"`          // 失败数
	AvgProcessTime  time.Duration `json:"avgProcessTime"`  // 平均处理时间
	TotalProcessTime time.Duration `json:"totalProcessTime"` // 总处理时间
	LastUpdated     time.Time     `json:"lastUpdated"`     // 最后更新时间
}

// ========== GPU 状态 ==========

// GPUMemory GPU 显存信息
type GPUMemory struct {
	Total        int64     `json:"total"`        // 总显存 (字节)
	Used         int64     `json:"used"`         // 已用显存 (字节)
	Available    int64     `json:"available"`    // 可用显存 (字节)
	ModelUsage   int64     `json:"modelUsage"`   // 模型占用 (字节)
	LastUpdated  time.Time `json:"lastUpdated"`  // 最后更新时间
}

// GPUStatus GPU 状态
type GPUStatus struct {
	Available   bool       `json:"available"`   // 是否可用
	DeviceName  string     `json:"deviceName"`  // 设备名称
	CUDAVersion string     `json:"cudaVersion"` // CUDA 版本
	Memory      GPUMemory  `json:"memory"`      // 显存信息
	Temperature float64    `json:"temperature"` // 温度 (°C)
	Utilization float64    `json:"utilization"` // 利用率 (%)
}

// ========== 语言检测 ==========

// LanguageDetect 语言检测结果
type LanguageDetect struct {
	Code       string  `json:"code"`       // 语言代码
	Name       string  `json:"name"`       // 语言名称
	Confidence float64 `json:"confidence"` // 置信度
}

// ========== 服务状态 ==========

// ServiceStatus 服务状态
type ServiceStatus struct {
	Status        string           `json:"status"`        // 服务状态
	Uptime        time.Duration    `json:"uptime"`        // 运行时间
	CurrentModel  string           `json:"currentModel"`  // 当前模型
	ModelsLoaded  int              `json:"modelsLoaded"`  // 已加载模型数
	TotalJobs     int              `json:"totalJobs"`     // 总任务数
	ActiveJobs    int              `json:"activeJobs"`    // 活跃任务数
	GPU           GPUStatus        `json:"gpu"`           // GPU 状态
	Queue         QueueStats       `json:"queue"`         // 队列统计
	StartTime     time.Time        `json:"startTime"`     // 启动时间
}

// ========== 统计信息 ==========

// ServiceStats 服务统计信息
type ServiceStats struct {
	TotalTranscriptions  int           `json:"totalTranscriptions"`  // 总转录次数
	TotalAudioDuration   time.Duration `json:"totalAudioDuration"`   // 总音频时长
	TotalProcessTime     time.Duration `json:"totalProcessTime"`     // 总处理时间
	AvgProcessTime       time.Duration `json:"avgProcessTime"`       // 平均处理时间
	AvgRealTimeFactor    float64       `json:"avgRealTimeFactor"`    // 平均实时因子
	SuccessRate          float64       `json:"successRate"`          // 成功率
	LanguageDistribution map[string]int `json:"languageDistribution"` // 语言分布
	DailyStats           []DailyStat   `json:"dailyStats"`           // 每日统计
}

// DailyStat 每日统计
type DailyStat struct {
	Date         string        `json:"date"`         // 日期 (YYYY-MM-DD)
	Transcriptions int         `json:"transcriptions"` // 转录次数
	AudioDuration time.Duration `json:"audioDuration"` // 音频时长
	ProcessTime   time.Duration `json:"processTime"`   // 处理时间
}

// ========== 请求/响应类型 ==========

// TranscribeRequest 转录请求
type TranscribeRequest struct {
	Language       string   `form:"language"`       // 语言代码
	Task           string   `form:"task"`           // 任务类型
	Format         string   `form:"format"`         // 输出格式
	WordTimestamps bool     `form:"wordTimestamps"` // 词级时间戳
	Priority       int      `form:"priority"`       // 优先级
	InitialPrompt  string   `form:"initialPrompt"`  // 初始提示词
}

// BatchTranscribeRequest 批量转录请求
type BatchTranscribeRequest struct {
	Files    []string            `json:"files" binding:"required"`    // 文件路径列表
	Options  TranscriptionOptions `json:"options"`                     // 转录选项
	Priority int                 `json:"priority"`                     // 优先级
}

// ExportRequest 导出请求
type ExportRequest struct {
	Format   SubtitleType `json:"format" binding:"required"` // 字幕格式
	FilePath string       `json:"filePath"`                  // 保存路径 (可选)
}

// EditResultRequest 编辑结果请求
type EditResultRequest struct {
	Text     string         `json:"text"`     // 修改后的文本
	Segments []Segment      `json:"segments"` // 修改后的段落
}
