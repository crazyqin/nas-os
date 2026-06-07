// Package hometheater 提供家庭影院系统功能，包括媒体库管理、自动刮削、转码、流媒体服务等。
// 支持 4K HDR 转码、HLS/DASH 自适应码率、DLNA 投屏、字幕管理等。
package hometheater

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

// 家庭影院相关错误.
var (
	// ErrMediaNotFound 媒体文件不存在.
	ErrMediaNotFound = errors.New("媒体文件不存在")
	// ErrMovieNotFound 电影不存在.
	ErrMovieNotFound = errors.New("电影不存在")
	// ErrTVShowNotFound 剧集不存在.
	ErrTVShowNotFound = errors.New("剧集不存在")
	// ErrPlaylistNotFound 播放列表不存在.
	ErrPlaylistNotFound = errors.New("播放列表不存在")
	// ErrPlaylistExists 播放列表已存在.
	ErrPlaylistExists = errors.New("播放列表已存在")
	// ErrTranscodeFailed 转码失败.
	ErrTranscodeFailed = errors.New("转码失败")
	// ErrUnsupportedCodec 不支持的编码格式.
	ErrUnsupportedCodec = errors.New("不支持的编码格式")
	// ErrScanInProgress 扫描正在进行中.
	ErrScanInProgress = errors.New("扫描正在进行中")
	// ErrInvalidPath 无效路径.
	ErrInvalidPath = errors.New("无效路径")
	// ErrStreamNotFound 流不存在.
	ErrStreamNotFound = errors.New("流不存在")
	// ErrSubtitleNotFound 字幕不存在.
	ErrSubtitleNotFound = errors.New("字幕不存在")
	// ErrDLNADeviceNotFound DLNA设备未找到.
	ErrDLNADeviceNotFound = errors.New("DLNA设备未找到")
	// ErrPlaybackFailed 播放失败.
	ErrPlaybackFailed = errors.New("播放失败")
	// ErrInvalidProfile 无效的转码配置.
	ErrInvalidProfile = errors.New("无效的转码配置")
)

// ========== 媒体类型 ==========

// MediaType 媒体类型.
type MediaType string

const (
	// MediaTypeMovie 电影.
	MediaTypeMovie MediaType = "movie"
	// MediaTypeTVShow 电视剧.
	MediaTypeTVShow MediaType = "tv_show"
	// MediaTypeEpisode 剧集.
	MediaTypeEpisode MediaType = "episode"
	// MediaTypeTrailer 预告片.
	MediaTypeTrailer MediaType = "trailer"
	// MediaTypeExtra 花絮.
	MediaTypeExtra MediaType = "extra"
)

// ========== 视频编码 ==========

// VideoCodec 视频编码格式.
type VideoCodec string

const (
	// CodecH264 H.264/AVC.
	CodecH264 VideoCodec = "h264"
	// CodecH265 H.265/HEVC.
	CodecH265 VideoCodec = "h265"
	// CodecVP9 VP9.
	CodecVP9 VideoCodec = "vp9"
	// CodecAV1 AV1.
	CodecAV1 VideoCodec = "av1"
)

// ========== 音频编码 ==========

// AudioCodec 音频编码格式.
type AudioCodec string

const (
	// AudioCodecAAC AAC.
	AudioCodecAAC AudioCodec = "aac"
	// AudioCodecAC3 AC3/Dolby Digital.
	AudioCodecAC3 AudioCodec = "ac3"
	// AudioCodecDTS DTS.
	AudioCodecDTS AudioCodec = "dts"
	// AudioCodecEAC3 E-AC3/Dolby Digital Plus.
	AudioCodecEAC3 AudioCodec = "eac3"
	// AudioCodecTrueHD Dolby TrueHD.
	AudioCodecTrueHD AudioCodec = "truehd"
	// AudioCodecDTS_HD DTS-HD.
	AudioCodecDTS_HD AudioCodec = "dts_hd"
	// AudioCodecOpus Opus.
	AudioCodecOpus AudioCodec = "opus"
)

// ========== 流媒体协议 ==========

// StreamProtocol 流媒体协议.
type StreamProtocol string

const (
	// ProtocolHLS HLS协议.
	ProtocolHLS StreamProtocol = "hls"
	// ProtocolDASH DASH协议.
	ProtocolDASH StreamProtocol = "dash"
	// ProtocolDLNA DLNA协议.
	ProtocolDLNA StreamProtocol = "dlna"
	// ProtocolDirect 直接播放.
	ProtocolDirect StreamProtocol = "direct"
)

// ========== 字幕格式 ==========

// SubtitleFormat 字幕格式.
type SubtitleFormat string

const (
	// SubtitleSRT SRT格式.
	SubtitleSRT SubtitleFormat = "srt"
	// SubtitleASS ASS/SSA格式.
	SubtitleASS SubtitleFormat = "ass"
	// SubtitleVTT WebVTT格式.
	SubtitleVTT SubtitleFormat = "vtt"
	// SubtitlePGS PGS图形字幕.
	SubtitlePGS SubtitleFormat = "pgs"
)

// ========== 硬件加速 ==========

// HardwareAccel 硬件加速类型.
type HardwareAccel string

const (
	// AccelNone 不使用硬件加速.
	AccelNone HardwareAccel = "none"
	// AccelVAAPI VAAPI (Intel/AMD).
	AccelVAAPI HardwareAccel = "vaapi"
	// AccelNVENC NVIDIA NVENC.
	AccelNVENC HardwareAccel = "nvenc"
	// AccelQSV Intel Quick Sync Video.
	AccelQSV HardwareAccel = "qsv"
	// AccelRKMPP Rockchip MPP.
	AccelRKMPP HardwareAccel = "rkmpp"
	// AccelV4L2 V4L2 (ARM).
	AccelV4L2 HardwareAccel = "v4l2"
)

// ========== 核心数据结构 ==========

// MediaLibrary 媒体库.
type MediaLibrary struct {
	ID           string    `json:"id"`            // 唯一标识
	Name         string    `json:"name"`          // 媒体库名称
	Path         string    `json:"path"`          // 根目录路径
	Type         MediaType `json:"type"`          // 媒体类型
	Description  string    `json:"description"`   // 描述
	Enabled      bool      `json:"enabled"`       // 是否启用
	AutoScan     bool      `json:"auto_scan"`     // 是否自动扫描
	ScanInterval int       `json:"scan_interval"` // 扫描间隔（分钟）
	MovieCount   int       `json:"movie_count"`   // 电影数量
	ShowCount    int       `json:"show_count"`    // 剧集数量
	TotalSize    int64     `json:"total_size"`    // 总大小（字节）
	CreatedAt    time.Time `json:"created_at"`    // 创建时间
	UpdatedAt    time.Time `json:"updated_at"`    // 更新时间
}

// Movie 电影信息.
type Movie struct {
	ID            string         `json:"id"`             // 唯一标识
	LibraryID     string         `json:"library_id"`     // 所属媒体库ID
	Title         string         `json:"title"`          // 标题
	OriginalTitle string         `json:"original_title"` // 原始标题
	Year          int            `json:"year"`           // 上映年份
	Rating        float64        `json:"rating"`         // 评分
	VoteCount     int            `json:"vote_count"`     // 投票数
	Overview      string         `json:"overview"`       // 简介
	Genres        []string       `json:"genres"`         // 类型
	Tags          []string       `json:"tags"`           // 标签
	Directors     []string       `json:"directors"`      // 导演
	Cast          []string       `json:"cast"`           // 演员
	Studio        string         `json:"studio"`         // 制片商
	Runtime       int            `json:"runtime"`        // 时长（分钟）
	ReleaseDate   string         `json:"release_date"`   // 上映日期
	PosterPath    string         `json:"poster_path"`    // 海报路径
	BackdropPath  string         `json:"backdrop_path"`  // 背景图路径
	TrailerURL    string         `json:"trailer_url"`    // 预告片URL
	TMDBID        int            `json:"tmdb_id"`        // TMDB ID
	IMDBID        string         `json:"imdb_id"`        // IMDB ID
	VideoInfo     *VideoInfo     `json:"video_info"`     // 视频信息
	AudioTracks   []*AudioTrack  `json:"audio_tracks"`   // 音轨
	Subtitles     []*Subtitle    `json:"subtitles"`      // 字幕
	FilePath      string         `json:"file_path"`      // 文件路径
	FileSize      int64          `json:"file_size"`      // 文件大小
	PlayCount     int64          `json:"play_count"`     // 播放次数
	LastPlayed    *time.Time     `json:"last_played"`    // 最后播放时间
	IsFavorite    bool           `json:"is_favorite"`    // 是否收藏
	WatchProgress *WatchProgress `json:"watch_progress"` // 观看进度
	CreatedAt     time.Time      `json:"created_at"`     // 入库时间
	UpdatedAt     time.Time      `json:"updated_at"`     // 更新时间
}

// TVShow 电视剧信息.
type TVShow struct {
	ID            string    `json:"id"`             // 唯一标识
	LibraryID     string    `json:"library_id"`     // 所属媒体库ID
	Title         string    `json:"title"`          // 标题
	OriginalTitle string    `json:"original_title"` // 原始标题
	Year          int       `json:"year"`           // 首播年份
	EndYear       int       `json:"end_year"`       // 完结年份
	Rating        float64   `json:"rating"`         // 评分
	VoteCount     int       `json:"vote_count"`     // 投票数
	Overview      string    `json:"overview"`       // 简介
	Genres        []string  `json:"genres"`         // 类型
	Tags          []string  `json:"tags"`           // 标签
	Network       string    `json:"network"`        // 播出网络
	Status        string    `json:"status"`         // 状态（returning/ended/cancelled）
	SeasonCount   int       `json:"season_count"`   // 季数
	EpisodeCount  int       `json:"episode_count"`  // 集数
	Runtime       int       `json:"runtime"`        // 平均时长（分钟）
	FirstAirDate  string    `json:"first_air_date"` // 首播日期
	PosterPath    string    `json:"poster_path"`    // 海报路径
	BackdropPath  string    `json:"backdrop_path"`  // 背景图路径
	TMDBID        int       `json:"tmdb_id"`        // TMDB ID
	IMDBID        string    `json:"imdb_id"`        // IMDB ID
	Seasons       []*Season `json:"seasons"`        // 季列表
	CreatedAt     time.Time `json:"created_at"`     // 入库时间
	UpdatedAt     time.Time `json:"updated_at"`     // 更新时间
}

// Season 季信息.
type Season struct {
	ID           string     `json:"id"`            // 唯一标识
	ShowID       string     `json:"show_id"`       // 所属剧集ID
	SeasonNumber int        `json:"season_number"` // 季号
	Name         string     `json:"name"`          // 名称
	Overview     string     `json:"overview"`      // 简介
	EpisodeCount int        `json:"episode_count"` // 集数
	AirDate      string     `json:"air_date"`      // 播出日期
	PosterPath   string     `json:"poster_path"`   // 海报路径
	TMDBID       int        `json:"tmdb_id"`       // TMDB ID
	Episodes     []*Episode `json:"episodes"`      // 剧集列表
}

// Episode 剧集信息.
type Episode struct {
	ID            string         `json:"id"`             // 唯一标识
	ShowID        string         `json:"show_id"`        // 所属剧集ID
	SeasonID      string         `json:"season_id"`      // 所属季ID
	SeasonNumber  int            `json:"season_number"`  // 季号
	EpisodeNumber int            `json:"episode_number"` // 集号
	Title         string         `json:"title"`          // 标题
	Overview      string         `json:"overview"`       // 简介
	Runtime       int            `json:"runtime"`        // 时长（分钟）
	AirDate       string         `json:"air_date"`       // 播出日期
	Rating        float64        `json:"rating"`         // 评分
	StillPath     string         `json:"still_path"`     // 剧照路径
	TMDBID        int            `json:"tmdb_id"`        // TMDB ID
	VideoInfo     *VideoInfo     `json:"video_info"`     // 视频信息
	AudioTracks   []*AudioTrack  `json:"audio_tracks"`   // 音轨
	Subtitles     []*Subtitle    `json:"subtitles"`      // 字幕
	FilePath      string         `json:"file_path"`      // 文件路径
	FileSize      int64          `json:"file_size"`      // 文件大小
	PlayCount     int64          `json:"play_count"`     // 播放次数
	LastPlayed    *time.Time     `json:"last_played"`    // 最后播放时间
	WatchProgress *WatchProgress `json:"watch_progress"` // 观看进度
	CreatedAt     time.Time      `json:"created_at"`     // 入库时间
	UpdatedAt     time.Time      `json:"updated_at"`     // 更新时间
}

// VideoInfo 视频信息.
type VideoInfo struct {
	Codec       VideoCodec `json:"codec"`         // 视频编码
	Width       int        `json:"width"`         // 宽度
	Height      int        `json:"height"`        // 高度
	Bitrate     int        `json:"bitrate"`       // 码率（kbps）
	FrameRate   float64    `json:"frame_rate"`    // 帧率
	AspectRatio string     `json:"aspect_ratio"`  // 宽高比
	Profile     string     `json:"profile"`       // 编码配置
	Level       string     `json:"level"`         // 编码级别
	PixelFormat string     `json:"pixel_format"`  // 像素格式
	ColorSpace  string     `json:"color_space"`   // 色彩空间
	ColorRange  string     `json:"color_range"`   // 色彩范围
	HDR         *HDRInfo   `json:"hdr,omitempty"` // HDR信息
	Duration    float64    `json:"duration"`      // 时长（秒）
	Container   string     `json:"container"`     // 容器格式
	BitDepth    int        `json:"bit_depth"`     // 位深度
}

// HDRInfo HDR信息.
type HDRInfo struct {
	Type      string `json:"type"`       // HDR10, HDR10+, DolbyVision, HLG
	MaxCLL    int    `json:"max_cll"`    // MaxCLL (nits)
	MaxFALL   int    `json:"max_fall"`   // MaxFALL (nits)
	DVProfile string `json:"dv_profile"` // Dolby Vision Profile
}

// AudioTrack 音轨信息.
type AudioTrack struct {
	Index         int        `json:"index"`          // 流索引
	Codec         AudioCodec `json:"codec"`          // 音频编码
	Channels      int        `json:"channels"`       // 声道数
	ChannelLayout string     `json:"channel_layout"` // 声道布局（stereo, 5.1, 7.1）
	Bitrate       int        `json:"bitrate"`        // 码率（kbps）
	SampleRate    int        `json:"sample_rate"`    // 采样率（Hz）
	Language      string     `json:"language"`       // 语言
	Title         string     `json:"title"`          // 标题
	IsDefault     bool       `json:"is_default"`     // 是否默认
	IsCommentary  bool       `json:"is_commentary"`  // 是否评论音轨
}

// Subtitle 字幕信息.
type Subtitle struct {
	ID         string         `json:"id"`          // 唯一标识
	Index      int            `json:"index"`       // 流索引
	Format     SubtitleFormat `json:"format"`      // 字幕格式
	Language   string         `json:"language"`    // 语言
	Title      string         `json:"title"`       // 标题
	IsDefault  bool           `json:"is_default"`  // 是否默认
	IsForced   bool           `json:"is_forced"`   // 是否强制字幕
	IsExternal bool           `json:"is_external"` // 是否外部字幕
	FilePath   string         `json:"file_path"`   // 外部字幕文件路径
}

// TranscodeProfile 转码配置.
type TranscodeProfile struct {
	ID            string        `json:"id"`             // 唯一标识
	Name          string        `json:"name"`           // 配置名称
	Description   string        `json:"description"`    // 描述
	VideoCodec    VideoCodec    `json:"video_codec"`    // 视频编码
	AudioCodec    AudioCodec    `json:"audio_codec"`    // 音频编码
	Width         int           `json:"width"`          // 输出宽度
	Height        int           `json:"height"`         // 输出高度
	VideoBitrate  int           `json:"video_bitrate"`  // 视频码率（kbps）
	AudioBitrate  int           `json:"audio_bitrate"`  // 音频码率（kbps）
	FrameRate     float64       `json:"frame_rate"`     // 帧率
	Preset        string        `json:"preset"`         // 编码预设（ultrafast/fast/medium/slow）
	Profile       string        `json:"profile"`        // 编码配置（baseline/main/high）
	Level         string        `json:"level"`          // 编码级别
	HardwareAccel HardwareAccel `json:"hardware_accel"` // 硬件加速
	TwoPass       bool          `json:"two_pass"`       // 是否两遍编码
	Priority      int           `json:"priority"`       // 优先级
	Enabled       bool          `json:"enabled"`        // 是否启用
}

// WatchProgress 观看进度.
type WatchProgress struct {
	Position    float64   `json:"position"`     // 当前位置（秒）
	Duration    float64   `json:"duration"`     // 总时长（秒）
	Percentage  float64   `json:"percentage"`   // 观看百分比
	Completed   bool      `json:"completed"`    // 是否看完
	LastUpdated time.Time `json:"last_updated"` // 最后更新时间
}

// PlaybackState 播放状态.
type PlaybackState string

const (
	// PlaybackIdle 空闲.
	PlaybackIdle PlaybackState = "idle"
	// PlaybackPlaying 播放中.
	PlaybackPlaying PlaybackState = "playing"
	// PlaybackPaused 暂停.
	PlaybackPaused PlaybackState = "paused"
	// PlaybackStopped 停止.
	PlaybackStopped PlaybackState = "stopped"
	// PlaybackBuffering 缓冲中.
	PlaybackBuffering PlaybackState = "buffering"
)

// Playlist 播放列表.
type Playlist struct {
	ID           string       `json:"id"`            // 唯一标识
	Name         string       `json:"name"`          // 列表名称
	Description  string       `json:"description"`   // 描述
	UserID       string       `json:"user_id"`       // 用户ID
	Items        []*MediaItem `json:"items"`         // 媒体项列表
	CurrentIndex int          `json:"current_index"` // 当前播放索引
	Shuffle      bool         `json:"shuffle"`       // 是否随机播放
	Repeat       RepeatMode   `json:"repeat"`        // 循环模式
	CreatedAt    time.Time    `json:"created_at"`    // 创建时间
	UpdatedAt    time.Time    `json:"updated_at"`    // 更新时间
}

// RepeatMode 循环模式.
type RepeatMode string

const (
	// RepeatOff 不循环.
	RepeatOff RepeatMode = "off"
	// RepeatAll 列表循环.
	RepeatAll RepeatMode = "all"
	// RepeatOne 单曲循环.
	RepeatOne RepeatMode = "one"
)

// MediaItem 媒体项.
type MediaItem struct {
	ID       string    `json:"id"`        // 媒体ID
	Type     MediaType `json:"type"`      // 媒体类型
	Title    string    `json:"title"`     // 标题
	Duration int       `json:"duration"`  // 时长（秒）
	FilePath string    `json:"file_path"` // 文件路径
	AddedAt  time.Time `json:"added_at"`  // 添加时间
}

// StreamSession 流媒体会话.
type StreamSession struct {
	ID            string            `json:"id"`             // 会话ID
	MediaID       string            `json:"media_id"`       // 媒体ID
	UserID        string            `json:"user_id"`        // 用户ID
	Protocol      StreamProtocol    `json:"protocol"`       // 协议
	Profile       *TranscodeProfile `json:"profile"`        // 转码配置
	State         PlaybackState     `json:"state"`          // 播放状态
	Position      float64           `json:"position"`       // 当前位置（秒）
	DeviceID      string            `json:"device_id"`      // 设备ID
	DeviceName    string            `json:"device_name"`    // 设备名称
	ClientIP      string            `json:"client_ip"`      // 客户端IP
	UserAgent     string            `json:"user_agent"`     // 用户代理
	Bitrate       int               `json:"bitrate"`        // 当前码率
	Bandwidth     int               `json:"bandwidth"`      // 估计带宽
	StartTime     time.Time         `json:"start_time"`     // 开始时间
	LastHeartbeat time.Time         `json:"last_heartbeat"` // 最后心跳
}

// TranscodeJob 转码任务.
type TranscodeJob struct {
	ID         string          `json:"id"`          // 任务ID
	MediaID    string          `json:"media_id"`    // 媒体ID
	ProfileID  string          `json:"profile_id"`  // 配置ID
	Status     TranscodeStatus `json:"status"`      // 任务状态
	Progress   float64         `json:"progress"`    // 进度百分比
	InputPath  string          `json:"input_path"`  // 输入路径
	OutputPath string          `json:"output_path"` // 输出路径
	Error      string          `json:"error"`       // 错误信息
	StartTime  time.Time       `json:"start_time"`  // 开始时间
	EndTime    *time.Time      `json:"end_time"`    // 结束时间
	Stats      *TranscodeStats `json:"stats"`       // 转码统计
}

// TranscodeStatus 转码状态.
type TranscodeStatus string

const (
	// TranscodePending 等待中.
	TranscodePending TranscodeStatus = "pending"
	// TranscodeRunning 转码中.
	TranscodeRunning TranscodeStatus = "running"
	// TranscodeCompleted 完成.
	TranscodeCompleted TranscodeStatus = "completed"
	// TranscodeFailed 失败.
	TranscodeFailed TranscodeStatus = "failed"
	// TranscodeCancelled 已取消.
	TranscodeCancelled TranscodeStatus = "cancelled"
)

// TranscodeStats 转码统计.
type TranscodeStats struct {
	Duration      float64 `json:"duration"`       // 已转码时长（秒）
	TotalDuration float64 `json:"total_duration"` // 总时长（秒）
	Speed         float64 `json:"speed"`          // 转码速度（倍速）
	FPS           float64 `json:"fps"`            // 帧率
	Bitrate       int     `json:"bitrate"`        // 当前码率（kbps）
	Size          int64   `json:"size"`           // 已输出大小（字节）
	FrameCount    int64   `json:"frame_count"`    // 已处理帧数
}

// DLNADevice DLNA设备.
type DLNADevice struct {
	ID           string    `json:"id"`           // 设备ID
	Name         string    `json:"name"`         // 设备名称
	Type         string    `json:"type"`         // 设备类型
	UDN          string    `json:"udn"`          // 唯一设备名
	Location     string    `json:"location"`     // 设备地址
	IconURL      string    `json:"icon_url"`     // 图标URL
	Manufacturer string    `json:"manufacturer"` // 制造商
	ModelName    string    `json:"model_name"`   // 型号
	Capabilities []string  `json:"capabilities"` // 支持的格式
	Online       bool      `json:"online"`       // 是否在线
	LastSeen     time.Time `json:"last_seen"`    // 最后发现时间
}

// UserConfig 用户配置.
type UserConfig struct {
	UserID          string `json:"user_id"`          // 用户ID
	PreferredLang   string `json:"preferred_lang"`   // 首选语言
	SubtitleEnabled bool   `json:"subtitle_enabled"` // 是否显示字幕
	SubtitleLang    string `json:"subtitle_lang"`    // 字幕语言
	SubtitleSize    int    `json:"subtitle_size"`    // 字幕大小
	AudioLang       string `json:"audio_lang"`       // 音频语言首选
	AutoPlay        bool   `json:"auto_play"`        // 是否自动播放下一集
	DefaultQuality  string `json:"default_quality"`  // 默认画质
	HWAccelEnabled  bool   `json:"hw_accel_enabled"` // 是否启用硬件加速
	MaxBandwidth    int    `json:"max_bandwidth"`    // 最大带宽（kbps）
	Theme           string `json:"theme"`            // 主题
	ViewMode        string `json:"view_mode"`        // 视图模式
}

// ScanResult 扫描结果.
type ScanResult struct {
	LibraryID   string    `json:"library_id"`   // 媒体库ID
	TotalFiles  int       `json:"total_files"`  // 总文件数
	NewMovies   int       `json:"new_movies"`   // 新增电影数
	NewShows    int       `json:"new_shows"`    // 新增剧集数
	NewEpisodes int       `json:"new_episodes"` // 新增剧集数
	Updated     int       `json:"updated"`      // 更新数
	Deleted     int       `json:"deleted"`      // 删除数
	Errors      int       `json:"errors"`       // 错误数
	StartTime   time.Time `json:"start_time"`   // 开始时间
	EndTime     time.Time `json:"end_time"`     // 结束时间
	Duration    float64   `json:"duration"`     // 耗时（秒）
}

// MediaStats 媒体统计.
type MediaStats struct {
	TotalMovies    int   `json:"total_movies"`    // 电影总数
	TotalShows     int   `json:"total_shows"`     // 剧集总数
	TotalEpisodes  int   `json:"total_episodes"`  // 剧集总集数
	TotalSize      int64 `json:"total_size"`      // 总大小（字节）
	TotalDuration  int64 `json:"total_duration"`  // 总时长（秒）
	TotalPlays     int64 `json:"total_plays"`     // 总播放次数
	ActiveSessions int   `json:"active_sessions"` // 活跃会话数
	StorageUsed    int64 `json:"storage_used"`    // 已用存储
}
