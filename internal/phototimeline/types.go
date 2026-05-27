// Package phototimeline provides photo timeline management for NAS-OS.
// 照片时间线管理 - 自动组织、智能相册、地图聚合、去重检测
package phototimeline

import (
	"time"
)

// ============================================================
// 照片核心类型
// ============================================================

// Photo 照片元数据
type Photo struct {
	ID          string    `json:"id"`           // 唯一标识
	Hash        string    `json:"hash"`         // 文件哈希 (SHA256)
	Path        string    `json:"path"`         // 文件路径
	Filename    string    `json:"filename"`     // 文件名
	Size        int64     `json:"size"`         // 文件大小 (bytes)
	MimeType    string    `json:"mime_type"`    // MIME 类型
	Width       int       `json:"width"`        // 图片宽度
	Height      int       `json:"height"`       // 图片高度

	// 时间信息
	TakenAt     time.Time `json:"taken_at"`     // 拍摄时间
	ModifiedAt  time.Time `json:"modified_at"`  // 修改时间
	ImportedAt  time.Time `json:"imported_at"`  // 导入时间

	// EXIF 元数据
	EXIF        EXIFData  `json:"exif"`

	// GPS 信息
	Latitude    float64   `json:"latitude,omitempty"`
	Longitude   float64   `json:"longitude,omitempty"`
	Altitude    float64   `json:"altitude,omitempty"`
	Location    string    `json:"location,omitempty"` // 位置名称

	// 标签和分类
	Tags        []string  `json:"tags,omitempty"`
	Labels      []string  `json:"labels,omitempty"`   // AI 标签
	People      []string  `json:"people,omitempty"`   // 人物
	Albums      []string  `json:"albums,omitempty"`   // 所属相册

	// 状态
	Favorite    bool      `json:"favorite"`
	Trashed     bool      `json:"trashed"`
	Rating      int       `json:"rating"` // 0-5

	// 感知哈希 (用于去重)
	PerceptualHash string `json:"perceptual_hash,omitempty"`

	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EXIFData EXIF 元数据
type EXIFData struct {
	CameraMake    string    `json:"camera_make,omitempty"`
	CameraModel   string    `json:"camera_model,omitempty"`
	LensModel     string    `json:"lens_model,omitempty"`
	ISO           int       `json:"iso,omitempty"`
	Aperture      float64   `json:"aperture,omitempty"`     // f-stop
	ShutterSpeed  string    `json:"shutter_speed,omitempty"`
	FocalLength   float64   `json:"focal_length,omitempty"` // mm
	Flash         bool      `json:"flash"`
	WhiteBalance  string    `json:"white_balance,omitempty"`
	Orientation   int       `json:"orientation,omitempty"`

	// 视频特有
	Duration      float64   `json:"duration,omitempty"`     // 秒
	Bitrate       int64     `json:"bitrate,omitempty"`
	Codec         string    `json:"codec,omitempty"`
	FrameRate     float64   `json:"frame_rate,omitempty"`
}

// ============================================================
// 时间线类型
// ============================================================

// TimelineView 时间线视图类型
type TimelineView string

const (
	TimelineViewDay   TimelineView = "day"
	TimelineViewMonth TimelineView = "month"
	TimelineViewYear  TimelineView = "year"
)

// TimelineGroup 时间线分组
type TimelineGroup struct {
	Date      time.Time `json:"date"`       // 分组日期
	View      TimelineView `json:"view"`    // 视图类型
	Photos    []Photo   `json:"photos"`     // 照片列表
	Count     int       `json:"count"`      // 照片数量
	CoverURL  string    `json:"cover_url"`  // 封面照片 URL
}

// TimelineResponse 时间线响应
type TimelineResponse struct {
	Groups    []TimelineGroup `json:"groups"`
	Total     int             `json:"total"`
	Page      int             `json:"page"`
	PageSize  int             `json:"page_size"`
	HasMore   bool            `json:"has_more"`
}

// TimelineStats 时间线统计
type TimelineStats struct {
	TotalPhotos   int            `json:"total_photos"`
	TotalVideos   int            `json:"total_videos"`
	TotalSize     int64          `json:"total_size"`
	Years         int            `json:"years"`
	OldestPhoto   *time.Time     `json:"oldest_photo,omitempty"`
	NewestPhoto   *time.Time     `json:"newest_photo,omitempty"`
	CameraStats   []CameraStat   `json:"camera_stats,omitempty"`
	TopLocations  []LocationStat `json:"top_locations,omitempty"`
}

// CameraStat 相机统计
type CameraStat struct {
	Camera string `json:"camera"`
	Count  int    `json:"count"`
}

// LocationStat 地点统计
type LocationStat struct {
	Location  string  `json:"location"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Count     int     `json:"count"`
}

// ============================================================
// 智能相册类型
// ============================================================

// AlbumType 相册类型
type AlbumType string

const (
	AlbumTypeManual    AlbumType = "manual"    // 手动创建
	AlbumTypePerson    AlbumType = "person"    // 按人物
	AlbumTypeLocation  AlbumType = "location"  // 按地点
	AlbumTypeEvent     AlbumType = "event"     // 按事件
	AlbumTypeDate      AlbumType = "date"      // 按日期
	AlbumTypeTag       AlbumType = "tag"       // 按标签
	AlbumTypeCamera    AlbumType = "camera"    // 按相机
	AlbumTypeSmart     AlbumType = "smart"     // 智能规则
)

// Album 相册
type Album struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Type        AlbumType `json:"type"`
	CoverPhoto  string    `json:"cover_photo,omitempty"` // 封面照片 ID
	PhotoCount  int       `json:"photo_count"`

	// 智能相册规则
	Rules       *AlbumRules `json:"rules,omitempty"`

	// 元数据
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SharedWith  []string  `json:"shared_with,omitempty"`
}

// AlbumRules 智能相册规则
type AlbumRules struct {
	// 日期范围
	DateFrom    *time.Time `json:"date_from,omitempty"`
	DateTo      *time.Time `json:"date_to,omitempty"`

	// 地点范围
	LocationCenter *GeoPoint `json:"location_center,omitempty"`
	LocationRadius float64   `json:"location_radius,omitempty"` // km

	// 标签匹配
	Tags        []string `json:"tags,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	People      []string `json:"people,omitempty"`

	// 相机型号
	CameraMake  string `json:"camera_make,omitempty"`
	CameraModel string `json:"camera_model,omitempty"`

	// 评分
	MinRating   int `json:"min_rating,omitempty"`

	// 文件类型
	MimeTypes   []string `json:"mime_types,omitempty"`

	// 逻辑运算
	Operator    string `json:"operator"` // "and", "or"
}

// GeoPoint 地理坐标点
type GeoPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ============================================================
// 地图聚合类型
// ============================================================

// MapCluster 地图聚合点
type MapCluster struct {
	ID        string   `json:"id"`
	Center    GeoPoint `json:"center"`
	Photos    []Photo  `json:"photos,omitempty"`
	Count     int      `json:"count"`
	Radius    float64  `json:"radius"` // 聚合半径 (km)
	Location  string   `json:"location,omitempty"`
}

// MapBounds 地图边界
type MapBounds struct {
	NorthEast GeoPoint `json:"north_east"`
	SouthWest GeoPoint `json:"south_west"`
}

// MapResponse 地图响应
type MapResponse struct {
	Clusters  []MapCluster `json:"clusters"`
	Total     int          `json:"total"`
	Bounds    MapBounds    `json:"bounds"`
	Zoom      int          `json:"zoom"`
}

// ============================================================
// 去重检测类型
// ============================================================

// DuplicateGroup 重复照片组
type DuplicateGroup struct {
	ID          string  `json:"id"`
	Photos      []Photo `json:"photos"`
	Hash        string  `json:"hash"`         // 感知哈希
	Similarity  float64 `json:"similarity"`   // 相似度 0-1
	TotalSize   int64   `json:"total_size"`   // 总占用空间
	WastedSize  int64   `json:"wasted_size"`  // 浪费空间
	Recommended string  `json:"recommended"`  // 建议保留的照片 ID
}

// DedupStats 去重统计
type DedupStats struct {
	TotalPhotos     int   `json:"total_photos"`
	DuplicateGroups int   `json:"duplicate_groups"`
	DuplicatePhotos int   `json:"duplicate_photos"`
	WastedSpace     int64 `json:"wasted_space"`
}

// ============================================================
// 批量操作类型
// ============================================================

// BatchOperation 批量操作类型
type BatchOperation string

const (
	BatchOpMove   BatchOperation = "move"
	BatchOpCopy   BatchOperation = "copy"
	BatchOpDelete BatchOperation = "delete"
	BatchOpTag    BatchOperation = "tag"
	BatchOpUntag  BatchOperation = "untag"
	BatchOpFav    BatchOperation = "favorite"
	BatchOpUnfav  BatchOperation = "unfavorite"
	BatchOpTrash  BatchOperation = "trash"
	BatchOpRate   BatchOperation = "rate"
)

// BatchRequest 批量操作请求
type BatchRequest struct {
	Operation BatchOperation `json:"operation" binding:"required"`
	PhotoIDs  []string       `json:"photo_ids" binding:"required,min=1"`
	Target    string         `json:"target,omitempty"`   // 目标路径或标签
	Value     string         `json:"value,omitempty"`    // 操作值
}

// BatchResult 批量操作结果
type BatchResult struct {
	Total     int      `json:"total"`
	Success   int      `json:"success"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
}

// ============================================================
// 分享链接类型
// ============================================================

// ShareLink 分享链接
type ShareLink struct {
	ID          string    `json:"id"`
	AlbumID     string    `json:"album_id,omitempty"`
	PhotoIDs    []string  `json:"photo_ids,omitempty"`
	Token       string    `json:"token"`
	Password    string    `json:"password,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	MaxViews    int       `json:"max_views,omitempty"`
	CurrentViews int      `json:"current_views"`
	AllowDownload bool    `json:"allow_download"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// ============================================================
// 搜索类型
// ============================================================

// SearchQuery 搜索查询
type SearchQuery struct {
	// 关键词
	Keyword     string    `json:"keyword,omitempty"`

	// 日期范围
	DateFrom    *time.Time `json:"date_from,omitempty"`
	DateTo      *time.Time `json:"date_to,omitempty"`

	// 地点
	Location    string    `json:"location,omitempty"`
	Latitude    *float64  `json:"latitude,omitempty"`
	Longitude   *float64  `json:"longitude,omitempty"`
	Radius      float64   `json:"radius,omitempty"` // km

	// 标签
	Tags        []string `json:"tags,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	People      []string `json:"people,omitempty"`

	// 相机信息
	CameraMake  string `json:"camera_make,omitempty"`
	CameraModel string `json:"camera_model,omitempty"`

	// 评分
	MinRating   int `json:"min_rating,omitempty"`

	// 文件信息
	MimeType    string `json:"mime_type,omitempty"`
	MinSize     *int64 `json:"min_size,omitempty"`
	MaxSize     *int64 `json:"max_size,omitempty"`

	// 排序
	SortBy    string `json:"sort_by,omitempty"`    // "date", "name", "size", "rating"
	SortOrder string `json:"sort_order,omitempty"` // "asc", "desc"

	// 分页
	Page      int `json:"page"`
	PageSize  int `json:"page_size"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Photos    []Photo `json:"photos"`
	Total     int     `json:"total"`
	Page      int     `json:"page"`
	PageSize  int     `json:"page_size"`
	HasMore   bool    `json:"has_more"`
}

// ============================================================
// 配置类型
// ============================================================

// Config 照片时间线配置
type Config struct {
	// 存储路径
	LibraryPath    string `json:"library_path"`
	ThumbnailPath  string `json:"thumbnail_path"`
	TempPath       string `json:"temp_path"`

	// 缩略图配置
	ThumbnailSizes []int  `json:"thumbnail_sizes"` // 像素宽度, 如 [256, 512, 1024]
	Quality        int    `json:"quality"`         // JPEG 质量 1-100

	// 时间线配置
	DefaultView    TimelineView `json:"default_view"`
	GroupThreshold int          `json:"group_threshold"` // 每组最大照片数

	// 去重配置
	DedupEnabled    bool    `json:"dedup_enabled"`
	SimilarityThreshold float64 `json:"similarity_threshold"` // 相似度阈值 0-1

	// 自动分类
	AutoTagEnabled  bool `json:"auto_tag_enabled"`
	AutoFaceEnabled bool `json:"auto_face_enabled"`
	AutoGPSReverse  bool `json:"auto_gps_reverse"` // GPS 反向解析

	// 限制
	MaxUploadSize   int64 `json:"max_upload_size"`   // 单文件最大上传大小
	MaxBatchSize    int   `json:"max_batch_size"`    // 批量操作最大数量
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		LibraryPath:         "/data/photos",
		ThumbnailPath:       "/data/thumbnails",
		TempPath:            "/tmp/phototimeline",
		ThumbnailSizes:      []int{256, 512, 1024},
		Quality:             85,
		DefaultView:         TimelineViewMonth,
		GroupThreshold:      100,
		DedupEnabled:        true,
		SimilarityThreshold: 0.95,
		AutoTagEnabled:      true,
		AutoFaceEnabled:     false,
		AutoGPSReverse:      true,
		MaxUploadSize:       50 * 1024 * 1024, // 50MB
		MaxBatchSize:        1000,
	}
}
