// Package media provides media library management functionality
// including video scanning, metadata scraping, and poster generation
package media

import "time"

// MediaType represents the type of media content
type MediaType string

const (
	// MediaTypeMovie represents a movie.
	MediaTypeMovie MediaType = "movie"
	// MediaTypeTVShow represents a TV show series.
	MediaTypeTVShow MediaType = "tv"
	// MediaTypeEpisode represents a TV episode.
	MediaTypeEpisode MediaType = "episode"
	// MediaTypeUnknown represents an unknown media type.
	MediaTypeUnknown MediaType = "unknown"
)

// VideoFile represents a video file on disk
type VideoFile struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Filename   string    `json:"filename"`
	Size       int64     `json:"size"`
	Duration   int       `json:"duration,omitempty"` // seconds
	Width      int       `json:"width,omitempty"`
	Height     int       `json:"height,omitempty"`
	Codec      string    `json:"codec,omitempty"`
	Bitrate    int       `json:"bitrate,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
}

// MediaMetadata represents scraped metadata from TMDB
type MediaMetadata struct {
	ID            string    `json:"id"`
	TMDBID        int       `json:"tmdb_id"`
	IMDBID        string    `json:"imdb_id,omitempty"`
	Type          MediaType `json:"type"`
	Title         string    `json:"title"`
	OriginalTitle string    `json:"original_title,omitempty"`
	Overview      string    `json:"overview,omitempty"`
	Tagline       string    `json:"tagline,omitempty"`
	PosterPath    string    `json:"poster_path,omitempty"`
	BackdropPath  string    `json:"backdrop_path,omitempty"`
	Rating        float64   `json:"rating,omitempty"`
	VoteCount     int       `json:"vote_count,omitempty"`
	ReleaseDate   string    `json:"release_date,omitempty"`
	Runtime       int       `json:"runtime,omitempty"` // minutes
	Genres        []string  `json:"genres,omitempty"`
	Cast          []Cast    `json:"cast,omitempty"`
	Directors     []string  `json:"directors,omitempty"`
	Studios       []string  `json:"studios,omitempty"`
	Countries     []string  `json:"countries,omitempty"`
	Languages     []string  `json:"languages,omitempty"`
	ScrapedAt     time.Time `json:"scraped_at"`
}

// Cast represents an actor/actress
type Cast struct {
	Name        string `json:"name"`
	Character   string `json:"character,omitempty"`
	ProfilePath string `json:"profile_path,omitempty"`
	Order       int    `json:"order,omitempty"`
}

// TVShowMetadata represents a TV show with seasons
type TVShowMetadata struct {
	MediaMetadata
	Seasons          []Season `json:"seasons,omitempty"`
	NumberOfSeasons  int      `json:"number_of_seasons,omitempty"`
	NumberOfEpisodes int      `json:"number_of_episodes,omitempty"`
	Status           string   `json:"status,omitempty"`
	Networks         []string `json:"networks,omitempty"`
}

// Season represents a TV show season
type Season struct {
	SeasonNumber int       `json:"season_number"`
	Name         string    `json:"name,omitempty"`
	Overview     string    `json:"overview,omitempty"`
	PosterPath   string    `json:"poster_path,omitempty"`
	AirDate      string    `json:"air_date,omitempty"`
	Episodes     []Episode `json:"episodes,omitempty"`
}

// Episode represents a TV episode
type Episode struct {
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	Name          string `json:"name,omitempty"`
	Overview      string `json:"overview,omitempty"`
	StillPath     string `json:"still_path,omitempty"`
	AirDate       string `json:"air_date,omitempty"`
	Runtime       int    `json:"runtime,omitempty"`
}

// MediaLibrary represents a media library collection
type MediaLibrary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Type        MediaType `json:"type"`
	MediaCount  int       `json:"media_count"`
	TotalSize   int64     `json:"total_size"`
	LastScanned time.Time `json:"last_scanned"`
}

// ScanResult represents the result of a media scan
type ScanResult struct {
	LibraryID    string        `json:"library_id"`
	TotalFiles   int           `json:"total_files"`
	NewFiles     int           `json:"new_files"`
	UpdatedFiles int           `json:"updated_files"`
	RemovedFiles int           `json:"removed_files"`
	Errors       []ScanError   `json:"errors,omitempty"`
	Duration     time.Duration `json:"duration"`
}

// ScanError represents an error during scanning
type ScanError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// SupportedExtensions contains supported video file extensions.
// 扩展支持更多格式，包括蓝光原盘格式
var SupportedExtensions = map[string]bool{
	// 标准视频格式
	".mp4":    true, // MPEG-4 Part 14
	".mkv":    true, // Matroska
	".avi":    true, // AVI
	".mov":    true, // QuickTime
	".wmv":    true, // Windows Media Video
	".flv":    true, // Flash Video
	".webm":   true, // WebM
	".m4v":    true, // MPEG-4 Video
	".ts":     true, // MPEG Transport Stream
	".m2ts":   true, // MPEG-2 Transport Stream (蓝光)
	".mts":    true, // AVCHD
	".vob":    true, // DVD Video Object
	".mpg":    true, // MPEG
	".mpeg":   true, // MPEG
	".mpe":    true, // MPEG
	".3gp":    true, // 3GPP
	".3g2":    true, // 3GPP2
	".ogv":    true, // Ogg Video
	".ogg":    true, // Ogg
	".rm":     true, // RealMedia
	".rmvb":   true, // RealMedia Variable Bitrate
	".asf":    true, // Advanced Systems Format
	".divx":   true, // DivX
	".f4v":    true, // Flash Video (MP4 container)
	".hevc":   true, // HEVC raw stream
	".h264":   true, // H.264 raw stream
	".h265":   true, // H.265 raw stream
	".avc":    true, // AVC raw stream
	".vp9":    true, // VP9 raw stream
	".av1":    true, // AV1 raw stream
	".dav":    true, // DVR-AV (监控录像)
	".wtv":    true, // Windows Recorded TV Show
	".dvr-ms": true, // Microsoft DVR
	".iso":    true, // ISO镜像（蓝光/DVD原盘）
	".m3u":    true, // M3U播放列表
	".m3u8":   true, // M3U8 HLS播放列表
	".pls":    true, // PLS播放列表
}

// MediaCategory 媒体分类
type MediaCategory string

const (
	CategoryMovie       MediaCategory = "movie"       // 电影
	CategoryTVShow      MediaCategory = "tv"          // 电视剧
	CategoryDocumentary MediaCategory = "documentary" // 纪录片
	CategoryAnimation   MediaCategory = "animation"   // 动画/动漫
	CategoryMusic       MediaCategory = "music"       // 音乐MV
	CategorySport       MediaCategory = "sport"       // 体育
	CategoryVariety     MediaCategory = "variety"     // 综艺
	CategoryNews        MediaCategory = "news"        // 新闻
	CategoryEducational MediaCategory = "educational" // 教育
	CategoryOther       MediaCategory = "other"       // 其他
)

// VideoQuality 视频质量等级
type VideoQuality string

const (
	QualitySD     VideoQuality = "sd"     // 标清 480p
	QualityHD     VideoQuality = "hd"     // 高清 720p
	QualityFHD    VideoQuality = "fhd"    // 全高清 1080p
	QualityUHD    VideoQuality = "uhd"    // 超高清 4K 2160p
	Quality8K     VideoQuality = "8k"     // 8K
	QualityBluRay VideoQuality = "bluray" // 蓝光原盘
	QualityRemux  VideoQuality = "remux"  // 蓝光Remux
	QualityWEBDL  VideoQuality = "webdl"  // WEB-DL
	QualityHDTV   VideoQuality = "hdtv"   // HDTV
)

// HDRType HDR类型 - 使用dolby_config.go中的HDRFormat
type HDRType = HDRFormat

// AudioType 音频类型 - 使用dolby_config.go中的AudioCodec
type AudioType = AudioCodec

// IntelligentClassification 智能分类结果
type IntelligentClassification struct {
	Category     MediaCategory `json:"category"`
	SubCategory  string        `json:"sub_category,omitempty"`
	Quality      VideoQuality  `json:"quality"`
	HDR          HDRType       `json:"hdr"`
	Audio        AudioType     `json:"audio"`
	Resolution   string        `json:"resolution"` // 1920x1080
	BitDepth     int           `json:"bit_depth"`  // 8, 10, 12
	FrameRate    float64       `json:"frame_rate"`
	Confidence   float64       `json:"confidence"`
	DetectedTags []string      `json:"detected_tags"`
}

// MediaCollection 媒体合集/系列
type MediaCollection struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Type       MediaType `json:"type"`
	Items      []string  `json:"items"` // 媒体ID列表
	PosterPath string    `json:"poster_path,omitempty"`
	Overview   string    `json:"overview,omitempty"`
	TotalCount int       `json:"total_count"`
}

// WatchProgress 观看进度
type WatchProgress struct {
	MediaID     string    `json:"media_id"`
	FilePath    string    `json:"file_path"`
	Position    int       `json:"position"` // 秒
	Duration    int       `json:"duration"` // 总时长
	Percentage  float64   `json:"percentage"`
	LastWatched time.Time `json:"last_watched"`
	Completed   bool      `json:"completed"`
}
