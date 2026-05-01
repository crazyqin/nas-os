// Package audiostation 提供音乐中心管理功能
package audiostation

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

// 音乐中心相关错误.
var (
	// ErrTrackNotFound 音乐文件不存在.
	ErrTrackNotFound = errors.New("音乐文件不存在")
	// ErrAlbumNotFound 专辑不存在.
	ErrAlbumNotFound = errors.New("专辑不存在")
	// ErrArtistNotFound 艺术家不存在.
	ErrArtistNotFound = errors.New("艺术家不存在")
	// ErrPlaylistNotFound 播放列表不存在.
	ErrPlaylistNotFound = errors.New("播放列表不存在")
	// ErrPlaylistExists 播放列表已存在.
	ErrPlaylistExists = errors.New("播放列表已存在")
	// ErrQueueEmpty 播放队列为空.
	ErrQueueEmpty = errors.New("播放队列为空")
	// ErrQueueIndexInvalid 队列索引无效.
	ErrQueueIndexInvalid = errors.New("队列索引无效")
	// ErrScanInProgress 扫描正在进行中.
	ErrScanInProgress = errors.New("扫描正在进行中")
	// ErrInvalidPath 无效路径.
	ErrInvalidPath = errors.New("无效路径")
	// ErrDLNADeviceNotFound DLNA设备未找到.
	ErrDLNADeviceNotFound = errors.New("DLNA设备未找到")
	// ErrUnsupportedFormat 不支持的音频格式.
	ErrUnsupportedFormat = errors.New("不支持的音频格式")
)

// ========== 播放模式 ==========

// PlayMode 播放模式.
type PlayMode string

// 播放模式常量.
const (
	// PlayModeOrder 顺序播放.
	PlayModeOrder PlayMode = "order"
	// PlayModeRandom 随机播放.
	PlayModeRandom PlayMode = "random"
	// PlayModeRepeatOne 单曲循环.
	PlayModeRepeatOne PlayMode = "repeat_one"
	// PlayModeRepeatAll 列表循环.
	PlayModeRepeatAll PlayMode = "repeat_all"
)

// ========== 音频格式 ==========

// AudioFormat 音频格式.
type AudioFormat string

// 支持的音频格式.
const (
	FormatMP3  AudioFormat = "mp3"
	FormatFLAC AudioFormat = "flac"
	FormatAAC  AudioFormat = "aac"
	FormatWAV  AudioFormat = "wav"
	FormatOGG  AudioFormat = "ogg"
	FormatAPE  AudioFormat = "ape"
	FormatWMA  AudioFormat = "wma"
	FormatOPUS AudioFormat = "opus"
)

// SupportedFormats 支持的音频格式列表.
var SupportedFormats = []AudioFormat{
	FormatMP3, FormatFLAC, FormatAAC, FormatWAV,
	FormatOGG, FormatAPE, FormatWMA, FormatOPUS,
}

// ========== 核心数据结构 ==========

// Track 音乐文件信息.
type Track struct {
	ID         string      `json:"id"`          // 唯一标识
	Title      string      `json:"title"`       // 标题
	Artist     string      `json:"artist"`      // 艺术家
	Album      string      `json:"album"`       // 专辑
	AlbumArtist string     `json:"album_artist"` // 专辑艺术家
	Genre      string      `json:"genre"`       // 流派
	Year       int         `json:"year"`        // 年份
	TrackNum   int         `json:"track_num"`   // 曲目号
	DiscNum    int         `json:"disc_num"`    // 碟片号
	Duration   int         `json:"duration"`    // 时长（秒）
	Bitrate    int         `json:"bitrate"`     // 码率（kbps）
	SampleRate int         `json:"sample_rate"` // 采样率（Hz）
	Channels   int         `json:"channels"`    // 声道数
	Format     AudioFormat `json:"format"`      // 音频格式
	FileSize   int64       `json:"file_size"`   // 文件大小（字节）
	FilePath   string      `json:"file_path"`   // 文件路径
	CoverPath  string      `json:"cover_path"`  // 封面图路径
	PlayCount  int64       `json:"play_count"`  // 播放次数
	LastPlayed *time.Time  `json:"last_played"` // 最后播放时间
	IsFavorite bool        `json:"is_favorite"` // 是否收藏
	CreatedAt  time.Time   `json:"created_at"`  // 入库时间
	UpdatedAt  time.Time   `json:"updated_at"`  // 更新时间
}

// Album 专辑信息.
type Album struct {
	ID          string    `json:"id"`          // 唯一标识
	Title       string    `json:"title"`       // 专辑名
	Artist      string    `json:"artist"`      // 专辑艺术家
	Genre       string    `json:"genre"`       // 流派
	Year        int       `json:"year"`        // 年份
	TrackCount  int       `json:"track_count"` // 曲目数
	Duration    int       `json:"duration"`    // 总时长（秒）
	CoverPath   string    `json:"cover_path"`  // 封面图路径
	Tracks      []*Track  `json:"tracks,omitempty"` // 曲目列表（详情时包含）
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
}

// Artist 艺术家信息.
type Artist struct {
	ID         string   `json:"id"`          // 唯一标识
	Name       string   `json:"name"`        // 艺术家名
	AlbumCount int      `json:"album_count"` // 专辑数
	TrackCount int      `json:"track_count"` // 曲目数
	Albums     []*Album `json:"albums,omitempty"` // 专辑列表
}

// Genre 流派信息.
type Genre struct {
	ID         string `json:"id"`          // 唯一标识
	Name       string `json:"name"`        // 流派名
	TrackCount int    `json:"track_count"` // 曲目数
}

// Playlist 播放列表.
type Playlist struct {
	ID          string    `json:"id"`          // 唯一标识
	Name        string    `json:"name"`        // 列表名称
	Description string    `json:"description"` // 描述
	TrackCount  int       `json:"track_count"` // 曲目数
	Duration    int       `json:"duration"`    // 总时长（秒）
	CoverPath   string    `json:"cover_path"`  // 封面图路径
	Tracks      []*Track  `json:"tracks,omitempty"` // 曲目列表（详情时包含）
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`  // 更新时间
}

// QueueItem 播放队列项.
type QueueItem struct {
	Index   int    `json:"index"`   // 队列中的位置
	TrackID string `json:"track_id"` // 音乐ID
	Track   *Track `json:"track,omitempty"` // 音乐详情
}

// PlayQueue 播放队列.
type PlayQueue struct {
	Items       []QueueItem `json:"items"`        // 队列列表
	CurrentIndex int        `json:"current_index"` // 当前播放索引
	Mode        PlayMode    `json:"mode"`          // 播放模式
	TotalCount  int         `json:"total_count"`   // 总曲目数
	TotalDuration int       `json:"total_duration"` // 总时长（秒）
}

// DLNADevice DLNA设备信息.
type DLNADevice struct {
	ID         string `json:"id"`          // 设备ID
	Name       string `json:"name"`        // 设备名称
	Type       string `json:"type"`        // 设备类型
	IP         string `json:"ip"`          // IP地址
	Port       int    `json:"port"`        // 端口
	IconURL    string `json:"icon_url"`    // 图标URL
	IsOnline   bool   `json:"is_online"`   // 是否在线
	IsPlaying  bool   `json:"is_playing"`  // 是否正在播放
	TrackID    string `json:"track_id"`    // 当前播放曲目
}

// ScanStatus 扫描状态.
type ScanStatus struct {
	IsRunning    bool      `json:"is_running"`    // 是否正在扫描
	Progress     float64   `json:"progress"`      // 进度（0-100）
	TotalFiles   int       `json:"total_files"`   // 总文件数
	ScannedFiles int       `json:"scanned_files"` // 已扫描数
	NewFiles     int       `json:"new_files"`     // 新增文件数
	ErrorFiles   int       `json:"error_files"`   // 错误文件数
	StartedAt    time.Time `json:"started_at"`    // 开始时间
	CompletedAt  *time.Time `json:"completed_at"` // 完成时间
}

// LibraryStats 音乐库统计信息.
type LibraryStats struct {
	TotalTracks  int     `json:"total_tracks"`  // 总曲目数
	TotalAlbums  int     `json:"total_albums"`  // 总专辑数
	TotalArtists int     `json:"total_artists"` // 总艺术家数
	TotalGenres  int     `json:"total_genres"`  // 总流派数
	TotalSize    int64   `json:"total_size"`    // 总大小（字节）
	TotalDuration int    `json:"total_duration"` // 总时长（秒）
	FavoriteCount int    `json:"favorite_count"` // 收藏数
}

// PlaybackStats 播放统计.
type PlaybackStats struct {
	TrackID    string    `json:"track_id"`    // 曲目ID
	PlayCount  int64     `json:"play_count"`  // 播放次数
	TotalTime  int       `json:"total_time"`  // 累计播放时长（秒）
	LastPlayed time.Time `json:"last_played"` // 最后播放时间
}

// ========== 请求/响应结构 ==========

// LibraryQuery 音乐库查询参数.
type LibraryQuery struct {
	Search  string `form:"search"`  // 搜索关键词
	Artist  string `form:"artist"`  // 按艺术家过滤
	Album   string `form:"album"`   // 按专辑过滤
	Genre   string `form:"genre"`   // 按流派过滤
	Year    int    `form:"year"`    // 按年份过滤
	Sort    string `form:"sort"`    // 排序字段：title, artist, album, year, duration, play_count
	Order   string `form:"order"`   // 排序方向：asc, desc
	Page    int    `form:"page"`    // 页码（从1开始）
	PerPage int    `form:"per_page"` // 每页数量
}

// PlaylistInput 创建/更新播放列表输入.
type PlaylistInput struct {
	Name        string   `json:"name" binding:"required"`        // 列表名称
	Description string   `json:"description"`                    // 描述
	TrackIDs    []string `json:"track_ids,omitempty"`            // 曲目ID列表
}

// QueueAddRequest 添加到播放队列请求.
type QueueAddRequest struct {
	TrackIDs []string `json:"track_ids" binding:"required"` // 曲目ID列表
	Position int      `json:"position"`                     // 插入位置（-1 表示末尾）
}

// QueueReorderRequest 播放队列重排序请求.
type QueueReorderRequest struct {
	FromIndex int `json:"from_index" binding:"required"` // 原位置
	ToIndex   int `json:"to_index" binding:"required"`   // 目标位置
}

// DLNACastRequest DLNA投送请求.
type DLNACastRequest struct {
	DeviceID string `json:"device_id" binding:"required"` // 设备ID
	TrackID  string `json:"track_id" binding:"required"`  // 曲目ID
}

// ScanRequest 扫描请求.
type ScanRequest struct {
	Paths     []string `json:"paths"`     // 扫描路径（空则使用默认路径）
	Recursive bool     `json:"recursive"` // 是否递归子目录
	Force     bool     `json:"force"`     // 强制重新扫描
}

// APIResponse 通用 API 响应.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// APISuccess 返回成功响应.
func APISuccess(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

// APIError 返回错误响应.
func APIError(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}
