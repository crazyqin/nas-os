// Package dlnamedia 提供 DLNA/UPnP 媒体投屏服务
package dlnamedia

import (
	"time"
)

// MediaType 媒体类型.
type MediaType string

const (
	MediaTypeVideo MediaType = "video"
	MediaTypeAudio MediaType = "audio"
	MediaTypePhoto MediaType = "photo"
)

// DeviceType 设备类型.
type DeviceType string

const (
	DeviceTypeRenderer DeviceType = "renderer" // 渲染器（电视、音箱）
	DeviceTypeServer   DeviceType = "server"   // 服务器（NAS、媒体服务器）
	DeviceTypeControl  DeviceType = "control"  // 控制点（手机、遥控器）
)

// PlayState 播放状态.
type PlayState string

const (
	PlayStateStopped PlayState = "STOPPED"
	PlayStatePlaying PlayState = "PLAYING"
	PlayStatePaused  PlayState = "PAUSED"
	PlayStateSeeking PlayState = "TRANSITIONING"
)

// TranscodeProfile 转码配置.
type TranscodeProfile struct {
	VideoCodec   string `json:"video_codec"`   // h264, h265, vp9
	AudioCodec   string `json:"audio_codec"`   // aac, mp3, ac3
	Container    string `json:"container"`     // mp4, mkv, ts
	Resolution   string `json:"resolution"`    // 1080p, 720p, 480p
	Bitrate      int    `json:"bitrate"`       // kbps
	AudioBitrate int    `json:"audio_bitrate"` // kbps
	Framerate    int    `json:"framerate"`     // fps
}

// Subtitle 字幕信息.
type Subtitle struct {
	ID       string `json:"id"`
	FilePath string `json:"file_path"`
	Language string `json:"language"`
	Format   string `json:"format"` // srt, ass, ssa
	IsForced bool   `json:"is_forced"`
}

// MediaItem 媒体文件.
type MediaItem struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	FilePath      string    `json:"file_path"`
	MediaType     MediaType `json:"media_type"`
	MimeType      string    `json:"mime_type"`
	Size          int64     `json:"size"`
	Duration      int64     `json:"duration"` // 秒
	Width         int       `json:"width,omitempty"`
	Height        int       `json:"height,omitempty"`
	Bitrate       int       `json:"bitrate,omitempty"`
	ThumbnailPath string    `json:"thumbnail_path,omitempty"`

	// 音频/视频元数据
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Year        int    `json:"year,omitempty"`
	TrackNumber int    `json:"track_number,omitempty"`

	// 字幕
	Subtitles []Subtitle `json:"subtitles,omitempty"`

	// 媒体库信息
	LibraryID  string   `json:"library_id"`
	FolderPath string   `json:"folder_path"`
	Tags       []string `json:"tags,omitempty"`

	// 时间戳
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ScannedAt time.Time `json:"scanned_at"`
}

// MediaLibrary 媒体库.
type MediaLibrary struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Path         string     `json:"path"`
	MediaType    MediaType  `json:"media_type"` // video, audio, photo, all
	Recursive    bool       `json:"recursive"`
	AutoScan     bool       `json:"auto_scan"`
	ScanInterval int        `json:"scan_interval"` // 分钟
	LastScanAt   *time.Time `json:"last_scan_at,omitempty"`
	ItemCount    int        `json:"item_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// DLNADevice DLNA 设备.
type DLNADevice struct {
	ID           string     `json:"id"`
	UDN          string     `json:"udn"` // Unique Device Name
	FriendlyName string     `json:"friendly_name"`
	DeviceType   DeviceType `json:"device_type"`
	Manufacturer string     `json:"manufacturer"`
	ModelName    string     `json:"model_name"`
	Location     string     `json:"location"` // 设备描述 URL
	IPAddress    string     `json:"ip_address"`
	Port         int        `json:"port"`
	IconURL      string     `json:"icon_url,omitempty"`

	// 设备能力
	SupportedMediaTypes []MediaType       `json:"supported_media_types"`
	MaxResolution       string            `json:"max_resolution,omitempty"` // 1920x1080
	PreferredTranscode  *TranscodeProfile `json:"preferred_transcode,omitempty"`

	// 状态
	IsOnline   bool      `json:"is_online"`
	LastSeenAt time.Time `json:"last_seen_at"`
	GroupID    string    `json:"group_id,omitempty"` // 设备分组
}

// DeviceGroup 设备分组（多房间音频同步）.
type DeviceGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	DeviceIDs   []string  `json:"device_ids"`
	IsSync      bool      `json:"is_sync"` // 是否同步播放
	Volume      int       `json:"volume"`  // 组音量 0-100
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PlaybackSession 播放会话.
type PlaybackSession struct {
	ID          string            `json:"id"`
	DeviceID    string            `json:"device_id"`
	GroupID     string            `json:"group_id,omitempty"`
	CurrentItem *MediaItem        `json:"current_item,omitempty"`
	State       PlayState         `json:"state"`
	Position    int64             `json:"position"` // 当前播放位置（秒）
	Duration    int64             `json:"duration"` // 总时长（秒）
	Volume      int               `json:"volume"`   // 音量 0-100
	IsMuted     bool              `json:"is_muted"`
	Transcode   *TranscodeProfile `json:"transcode,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PlayQueue 播放队列.
type PlayQueue struct {
	ID           string      `json:"id"`
	DeviceID     string      `json:"device_id"`
	Items        []QueueItem `json:"items"`
	CurrentIndex int         `json:"current_index"`
	Shuffle      bool        `json:"shuffle"`
	RepeatMode   string      `json:"repeat_mode"` // off, one, all
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// QueueItem 队列项.
type QueueItem struct {
	Index     int        `json:"index"`
	MediaID   string     `json:"media_id"`
	MediaItem *MediaItem `json:"media_item,omitempty"`
	AddedAt   time.Time  `json:"added_at"`
}

// ContentDirectoryItem 内容目录项（UPnP ContentDirectory）.
type ContentDirectoryItem struct {
	ID          string     `json:"id"`
	ParentID    string     `json:"parent_id"`
	Title       string     `json:"title"`
	IsContainer bool       `json:"is_container"` // 是否为容器（文件夹）
	ItemCount   int        `json:"item_count"`   // 子项数量
	MediaItem   *MediaItem `json:"media_item,omitempty"`
}

// ========== 请求/响应结构 ==========

// CreateLibraryRequest 创建媒体库请求.
type CreateLibraryRequest struct {
	Name         string    `json:"name" binding:"required"`
	Path         string    `json:"path" binding:"required"`
	MediaType    MediaType `json:"media_type"`
	Recursive    bool      `json:"recursive"`
	AutoScan     bool      `json:"auto_scan"`
	ScanInterval int       `json:"scan_interval"`
}

// UpdateLibraryRequest 更新媒体库请求.
type UpdateLibraryRequest struct {
	Name         *string `json:"name,omitempty"`
	Recursive    *bool   `json:"recursive,omitempty"`
	AutoScan     *bool   `json:"auto_scan,omitempty"`
	ScanInterval *int    `json:"scan_interval,omitempty"`
}

// ScanLibraryRequest 扫描媒体库请求.
type ScanLibraryRequest struct {
	LibraryID string `json:"library_id" binding:"required"`
	Force     bool   `json:"force"` // 强制全量扫描
}

// PushMediaRequest 推送媒体请求.
type PushMediaRequest struct {
	DeviceID   string `json:"device_id" binding:"required"`
	MediaID    string `json:"media_id" binding:"required"`
	Position   int64  `json:"position,omitempty"`    // 起始播放位置（秒）
	SubtitleID string `json:"subtitle_id,omitempty"` // 字幕 ID
}

// ControlPlaybackRequest 控制播放请求.
type ControlPlaybackRequest struct {
	Action   string `json:"action" binding:"required"` // play, pause, stop, seek, next, prev
	Position int64  `json:"position,omitempty"`        // seek 时的目标位置（秒）
}

// SetVolumeRequest 设置音量请求.
type SetVolumeRequest struct {
	Level int `json:"level" binding:"required,min=0,max=100"`
}

// CreateGroupRequest 创建设备分组请求.
type CreateGroupRequest struct {
	Name      string   `json:"name" binding:"required"`
	DeviceIDs []string `json:"device_ids" binding:"required"`
	IsSync    bool     `json:"is_sync"`
}

// UpdateGroupRequest 更新设备分组请求.
type UpdateGroupRequest struct {
	Name      *string  `json:"name,omitempty"`
	DeviceIDs []string `json:"device_ids,omitempty"`
	IsSync    *bool    `json:"is_sync,omitempty"`
	Volume    *int     `json:"volume,omitempty"`
}

// ManageQueueRequest 管理播放队列请求.
type ManageQueueRequest struct {
	Action      string   `json:"action" binding:"required"` // add, remove, clear, reorder
	MediaIDs    []string `json:"media_ids,omitempty"`
	Index       int      `json:"index,omitempty"`
	TargetIndex int      `json:"target_index,omitempty"`
}

// SearchMediaRequest 搜索媒体请求.
type SearchMediaRequest struct {
	Query     string    `form:"q"`
	MediaType MediaType `form:"type"`
	LibraryID string    `form:"library_id"`
	Tags      string    `form:"tags"`
	SortBy    string    `form:"sort_by"`    // title, date, size, duration
	SortOrder string    `form:"sort_order"` // asc, desc
	Page      int       `form:"page"`
	PageSize  int       `form:"page_size"`
}

// DiscoverDevicesRequest 发现设备请求.
type DiscoverDevicesRequest struct {
	Timeout int `json:"timeout"` // 秒
}
