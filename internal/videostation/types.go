// Package videostation 提供视频站功能，对标群晖 Video Station
package videostation

import "time"

// VideoStatus 视频状态
type VideoStatus string

const (
	VideoStatusReady    VideoStatus = "ready"
	VideoStatusIndexing VideoStatus = "indexing"
	VideoStatusError    VideoStatus = "error"
)

// TranscodeStatus 转码状态
type TranscodeStatus string

const (
	TranscodeStatusPending   TranscodeStatus = "pending"
	TranscodeStatusRunning   TranscodeStatus = "running"
	TranscodeStatusCompleted TranscodeStatus = "completed"
	TranscodeStatusFailed    TranscodeStatus = "failed"
	TranscodeStatusCancelled TranscodeStatus = "cancelled"
)

// TranscodeFormat 转码输出格式
type TranscodeFormat string

const (
	FormatHLS  TranscodeFormat = "hls"
	FormatDASH TranscodeFormat = "dash"
	FormatMP4  TranscodeFormat = "mp4"
)

// SubtitleType 字幕类型
type SubtitleType string

const (
	SubtitleTypeEmbedded SubtitleType = "embedded"
	SubtitleTypeExternal SubtitleType = "external"
)

// VideoCodec 视频编码格式
type VideoCodec string

const (
	CodecH264 VideoCodec = "h264"
	CodecH265 VideoCodec = "h265"
	CodecVP9  VideoCodec = "vp9"
	CodecAV1  VideoCodec = "av1"
)

// AudioCodec 音频编码格式
type AudioCodec string

const (
	AACCodec  AudioCodec = "aac"
	MP3Codec  AudioCodec = "mp3"
	AC3Codec  AudioCodec = "ac3"
	OpusCodec AudioCodec = "opus"
)

// HardwareAccel 硬件加速类型
type HardwareAccel string

const (
	HWAccelNone HardwareAccel = "none"
	HWAccelVAAPI HardwareAccel = "vaapi"
	HWAccelNVENC HardwareAccel = "nvenc"
	HWAccelQSV   HardwareAccel = "qsv"
	HWAccelRKMPP HardwareAccel = "rkmpp"
)

// Video 视频元数据
type Video struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	FilePath    string      `json:"file_path"`
	FileName    string      `json:"file_name"`
	FileSize    int64       `json:"file_size"`
	Duration    float64     `json:"duration"` // 秒
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	VideoCodec  VideoCodec  `json:"video_codec"`
	AudioCodec  AudioCodec  `json:"audio_codec"`
	Bitrate     int64       `json:"bitrate"`      // bps
	Framerate   float64     `json:"framerate"`    // fps
	Container   string      `json:"container"`    // mp4, mkv, avi 等
	PosterURL   string      `json:"poster_url,omitempty"`
	ThumbURL    string      `json:"thumb_url,omitempty"`
	LibraryID   string      `json:"library_id"`
	Tags        []string    `json:"tags,omitempty"`
	Category    string      `json:"category,omitempty"`
	Genre       string      `json:"genre,omitempty"`
	Year        int         `json:"year,omitempty"`
	Rating      float64     `json:"rating,omitempty"`
	Subtitles   []Subtitle  `json:"subtitles,omitempty"`
	Status      VideoStatus `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	IndexedAt   *time.Time  `json:"indexed_at,omitempty"`
}

// Subtitle 字幕信息
type Subtitle struct {
	ID       string       `json:"id"`
	VideoID  string       `json:"video_id"`
	Language string       `json:"language"`
	Label    string       `json:"label"`
	Type     SubtitleType `json:"type"`
	FilePath string       `json:"file_path,omitempty"`
	Codec    string       `json:"codec,omitempty"`
	IsDefault bool        `json:"is_default"`
}

// TranscodeJob 转码任务
type TranscodeJob struct {
	ID            string          `json:"id"`
	VideoID       string          `json:"video_id"`
	Status        TranscodeStatus `json:"status"`
	Progress      float64         `json:"progress"`       // 0-100
	Format        TranscodeFormat `json:"format"`
	Resolution    string          `json:"resolution"`     // 1080p, 720p, 480p
	VideoBitrate  int64           `json:"video_bitrate"`  // bps
	AudioBitrate  int64           `json:"audio_bitrate"`  // bps
	HWAccel       HardwareAccel   `json:"hw_accel"`
	OutputPath    string          `json:"output_path,omitempty"`
	Error         string          `json:"error,omitempty"`
	StartedAt     *time.Time      `json:"started_at,omitempty"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// PlaySession 播放会话
type PlaySession struct {
	ID          string    `json:"id"`
	VideoID     string    `json:"video_id"`
	UserID      string    `json:"user_id"`
	Position    float64   `json:"position"`     // 当前播放位置（秒）
	Duration    float64   `json:"duration"`     // 视频总时长（秒）
	Progress    float64   `json:"progress"`     // 播放进度百分比 0-100
	DeviceType  string    `json:"device_type"`  // web, mobile, tv
	DeviceName  string    `json:"device_name"`
	UserAgent   string    `json:"user_agent,omitempty"`
	IPAddress   string    `json:"ip_address,omitempty"`
	LastUpdated time.Time `json:"last_updated"`
	CreatedAt   time.Time `json:"created_at"`
}

// VideoLibrary 视频库配置
type VideoLibrary struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Description  string    `json:"description,omitempty"`
	VideoCount   int       `json:"video_count"`
	TotalSize    int64     `json:"total_size"`
	LastScanned  *time.Time `json:"last_scanned,omitempty"`
	AutoScan     bool      `json:"auto_scan"`
	ScanInterval int       `json:"scan_interval"` // 分钟
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// VideoStats 视频统计
type VideoStats struct {
	TotalVideos     int           `json:"total_videos"`
	TotalLibraries  int           `json:"total_libraries"`
	TotalSize       int64         `json:"total_size"`
	TotalDuration   float64       `json:"total_duration"` // 秒
	ActiveSessions  int           `json:"active_sessions"`
	ActiveTranscodes int          `json:"active_transcodes"`
	CodecBreakdown  map[string]int `json:"codec_breakdown"`
	FormatBreakdown map[string]int `json:"format_breakdown"`
	RecentlyPlayed  []Video       `json:"recently_played,omitempty"`
}

// ScanResult 扫描结果
type ScanResult struct {
	LibraryID   string `json:"library_id"`
	NewVideos   int    `json:"new_videos"`
	Updated     int    `json:"updated"`
	Removed     int    `json:"removed"`
	Errors      int    `json:"errors"`
	ScannedAt   time.Time `json:"scanned_at"`
}

// PlayRequest 播放请求
type PlayRequest struct {
	Quality   string `json:"quality,omitempty"`   // original, 1080p, 720p, 480p
	Format    string `json:"format,omitempty"`    // hls, dash, direct
	AudioTrack int   `json:"audio_track,omitempty"`
	SubtitleID string `json:"subtitle_id,omitempty"`
}

// PlayResponse 播放响应
type PlayResponse struct {
	VideoID     string `json:"video_id"`
	StreamURL   string `json:"stream_url"`
	Format      string `json:"format"`
	Quality     string `json:"quality"`
	Duration    float64 `json:"duration"`
	Position    float64 `json:"position"` // 续播位置
	SessionID   string `json:"session_id"`
}

// TranscodeRequest 转码请求
type TranscodeRequest struct {
	Format     TranscodeFormat `json:"format"`
	Resolution string          `json:"resolution"`
	VideoBitrate int64         `json:"video_bitrate,omitempty"`
	AudioBitrate int64         `json:"audio_bitrate,omitempty"`
	HWAccel    HardwareAccel   `json:"hw_accel,omitempty"`
}

// UpdateVideoRequest 更新视频请求
type UpdateVideoRequest struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Category    string   `json:"category,omitempty"`
	Genre       string   `json:"genre,omitempty"`
	Year        int      `json:"year,omitempty"`
	Rating      float64  `json:"rating,omitempty"`
}

// CreateLibraryRequest 创建视频库请求
type CreateLibraryRequest struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Description  string `json:"description,omitempty"`
	AutoScan     bool   `json:"auto_scan"`
	ScanInterval int    `json:"scan_interval,omitempty"`
}

// SessionUpdateRequest 会话更新请求
type SessionUpdateRequest struct {
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
}
