// Package digitalsignage 数字标牌服务
// 对标飞牛fnOS数字标牌功能
package digitalsignage

import (
	"time"
)

// ContentType 内容类型
type ContentType string

const (
	ContentTypeImage  ContentType = "image"  // 图片
	ContentTypeVideo  ContentType = "video"  // 视频
	ContentTypeWeb    ContentType = "web"    // 网页
	ContentTypeText   ContentType = "text"   // 文本
	ContentTypeWidget ContentType = "widget" // 组件
)

// ContentStatus 内容状态
type ContentStatus string

const (
	ContentStatusActive   ContentStatus = "active"
	ContentStatusInactive ContentStatus = "inactive"
	ContentStatusExpired  ContentStatus = "expired"
)

// Content 内容定义
type Content struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Type      ContentType   `json:"type"`
	Status    ContentStatus `json:"status"`
	URL       string        `json:"url,omitempty"`       // 远程URL
	FilePath  string        `json:"file_path,omitempty"` // 本地文件路径
	Text      string        `json:"text,omitempty"`      // 文本内容
	Duration  time.Duration `json:"duration"`            // 播放时长
	Width     int           `json:"width,omitempty"`     // 宽度
	Height    int           `json:"height,omitempty"`    // 高度
	FileSize  int64         `json:"file_size,omitempty"` // 文件大小
	MimeType  string        `json:"mime_type,omitempty"` // MIME类型
	Tags      []string      `json:"tags,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"` // 过期时间
}

// PlaylistStatus 播放列表状态
type PlaylistStatus string

const (
	PlaylistStatusActive   PlaylistStatus = "active"
	PlaylistStatusInactive PlaylistStatus = "inactive"
)

// PlaylistItem 播放列表项
type PlaylistItem struct {
	ContentID  string        `json:"content_id"`
	Order      int           `json:"order"`
	Duration   time.Duration `json:"duration"`             // 覆盖内容默认时长
	Transition string        `json:"transition,omitempty"` // 过渡效果: fade, slide, none
}

// Playlist 播放列表
type Playlist struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Status      PlaylistStatus `json:"status"`
	Items       []PlaylistItem `json:"items"`
	Loop        bool           `json:"loop"` // 是否循环播放
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ScheduleType 排程类型
type ScheduleType string

const (
	ScheduleTypeFixed  ScheduleType = "fixed"  // 固定时间播放
	ScheduleTypeLoop   ScheduleType = "loop"   // 循环播放
	ScheduleTypeUrgent ScheduleType = "urgent" // 紧急插播
)

// Schedule 排程定义
type Schedule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	PlaylistID  string       `json:"playlist_id"`
	DeviceGroup string       `json:"device_group,omitempty"` // 设备组
	DeviceIDs   []string     `json:"device_ids,omitempty"`   // 指定设备
	Type        ScheduleType `json:"type"`
	Enabled     bool         `json:"enabled"`
	StartTime   time.Time    `json:"start_time"`
	EndTime     *time.Time   `json:"end_time,omitempty"`
	Priority    int          `json:"priority"`       // 优先级，数字越大优先级越高
	Cron        string       `json:"cron,omitempty"` // cron表达式
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusError   DeviceStatus = "error"
)

// Device 设备定义
type Device struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Group           string       `json:"group"`
	Status          DeviceStatus `json:"status"`
	IP              string       `json:"ip,omitempty"`
	MAC             string       `json:"mac,omitempty"`
	Resolution      string       `json:"resolution,omitempty"`  // 分辨率，如 1920x1080
	Orientation     string       `json:"orientation,omitempty"` // 横屏/竖屏: landscape, portrait
	LastSeen        *time.Time   `json:"last_seen,omitempty"`
	CurrentContent  string       `json:"current_content,omitempty"`  // 当前播放内容
	CurrentPlaylist string       `json:"current_playlist,omitempty"` // 当前播放列表
	Volume          int          `json:"volume"`                     // 音量 0-100
	Brightness      int          `json:"brightness"`                 // 亮度 0-100
	Tags            []string     `json:"tags,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// DeviceGroup 设备组
type DeviceGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	DeviceIDs   []string  `json:"device_ids"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LayoutType 布局类型
type LayoutType string

const (
	LayoutTypeFullscreen LayoutType = "fullscreen" // 全屏
	LayoutTypeSplit2     LayoutType = "split2"     // 双分屏
	LayoutTypeSplit4     LayoutType = "split4"     // 四分屏
	LayoutTypePIP        LayoutType = "pip"        // 画中画
	LayoutTypeCustom     LayoutType = "custom"     // 自定义
)

// LayoutZone 布局区域
type LayoutZone struct {
	ID     string `json:"id"`
	X      int    `json:"x"`      // 左上角X坐标
	Y      int    `json:"y"`      // 左上角Y坐标
	Width  int    `json:"width"`  // 宽度百分比 0-100
	Height int    `json:"height"` // 高度百分比 0-100
}

// Template 布局模板
type Template struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        LayoutType   `json:"type"`
	Zones       []LayoutZone `json:"zones"`
	Preview     string       `json:"preview,omitempty"` // 预览图路径
	IsDefault   bool         `json:"is_default"`
	CreatedAt   time.Time    `json:"created_at"`
}

// PlaybackStatus 播放状态
type PlaybackStatus struct {
	DeviceID     string    `json:"device_id"`
	PlaylistID   string    `json:"playlist_id"`
	ContentID    string    `json:"content_id"`
	ContentIndex int       `json:"content_index"`
	Progress     float64   `json:"progress"` // 播放进度 0-1
	StartedAt    time.Time `json:"started_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
