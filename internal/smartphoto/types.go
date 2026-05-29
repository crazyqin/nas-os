package smartphoto

import (
	"time"
)

// Photo 照片信息
type Photo struct {
	ID            string    `json:"id"`
	Filename      string    `json:"filename"`
	Path          string    `json:"path"`
	Size          int64     `json:"size"`
	MimeType      string    `json:"mime_type"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	ThumbnailPath string    `json:"thumbnail_path,omitempty"`

	// EXIF 元数据
	EXIF EXIFData `json:"exif"`

	// GPS 信息
	GPS *GPSData `json:"gps,omitempty"`

	// 时间信息
	TakenAt   time.Time `json:"taken_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// AI 分析结果
	Faces     []Face      `json:"faces,omitempty"`
	Objects   []ObjectTag `json:"objects,omitempty"`
	Scenes    []SceneTag  `json:"scenes,omitempty"`
	Labels    []string    `json:"labels,omitempty"`

	// 相册关联
	AlbumIDs  []string `json:"album_ids,omitempty"`

	// 重复检测
	Hash      string `json:"hash"`
	IsDuplicate bool  `json:"is_duplicate"`
	DuplicateIDs []string `json:"duplicate_ids,omitempty"`

	// 状态
	Status    PhotoStatus `json:"status"`
	IsFavorite bool       `json:"is_favorite"`
	IsHidden   bool       `json:"is_hidden"`
}

// EXIFData EXIF 元数据
type EXIFData struct {
	CameraMake   string    `json:"camera_make,omitempty"`
	CameraModel  string    `json:"camera_model,omitempty"`
	Software     string    `json:"software,omitempty"`
	DateTime     time.Time `json:"date_time,omitempty"`
	ExposureTime string    `json:"exposure_time,omitempty"`
	FNumber      float64   `json:"f_number,omitempty"`
	ISO          int       `json:"iso,omitempty"`
	FocalLength  float64   `json:"focal_length,omitempty"`
	Flash        bool      `json:"flash,omitempty"`
	Orientation  int       `json:"orientation,omitempty"`
}

// GPSData GPS 位置信息
type GPSData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
	Location  string  `json:"location,omitempty"`
	City      string  `json:"city,omitempty"`
	Country   string  `json:"country,omitempty"`
}

// Face 人脸信息
type Face struct {
	ID        string    `json:"id"`
	PersonID  string    `json:"person_id,omitempty"`
	PersonName string   `json:"person_name,omitempty"`
	PhotoID   string    `json:"photo_id"`
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Width     float64   `json:"width"`
	Height    float64   `json:"height"`
	Confidence float64  `json:"confidence"`
	Embedding []float64 `json:"embedding,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Person 人物信息
type Person struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	FaceCount int       `json:"face_count"`
	PhotoCount int      `json:"photo_count"`
	CoverPhotoID string `json:"cover_photo_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ObjectTag 物体标签
type ObjectTag struct {
	ID         string    `json:"id"`
	PhotoID    string    `json:"photo_id"`
	Label      string    `json:"label"`
	Confidence float64   `json:"confidence"`
	X          float64   `json:"x"`
	Y          float64   `json:"y"`
	Width      float64   `json:"width"`
	Height     float64   `json:"height"`
	CreatedAt  time.Time `json:"created_at"`
}

// SceneTag 场景标签
type SceneTag struct {
	ID         string    `json:"id"`
	PhotoID    string    `json:"photo_id"`
	Label      string    `json:"label"`
	Confidence float64   `json:"confidence"`
	CreatedAt  time.Time `json:"created_at"`
}

// Album 相册
type Album struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Type        AlbumType   `json:"type"`
	CoverPhotoID string     `json:"cover_photo_id,omitempty"`
	PhotoCount  int         `json:"photo_count"`
	PhotoIDs    []string    `json:"photo_ids,omitempty"`

	// 智能相册规则
	Rules       *AlbumRules `json:"rules,omitempty"`

	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// AlbumType 相册类型
type AlbumType string

const (
	AlbumTypeManual    AlbumType = "manual"    // 手动相册
	AlbumTypePerson    AlbumType = "person"    // 人物相册
	AlbumTypeLocation  AlbumType = "location"  // 地点相册
	AlbumTypeTime      AlbumType = "time"      // 时间相册
	AlbumTypeScene     AlbumType = "scene"     // 场景相册
	AlbumTypeSmart     AlbumType = "smart"     // 智能相册（自定义规则）
)

// AlbumRules 智能相册规则
type AlbumRules struct {
	// 人物规则
	PersonIDs   []string `json:"person_ids,omitempty"`

	// 地点规则
	Locations   []LocationRule `json:"locations,omitempty"`

	// 时间规则
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`

	// 标签规则
	Labels      []string `json:"labels,omitempty"`

	// 场景规则
	Scenes      []string `json:"scenes,omitempty"`

	// 相机规则
	CameraMake  string `json:"camera_make,omitempty"`
	CameraModel string `json:"camera_model,omitempty"`

	// 组合方式
	MatchAll    bool `json:"match_all"` // true=AND, false=OR
}

// LocationRule 地点规则
type LocationRule struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Radius    float64 `json:"radius"` // 公里
	Name      string  `json:"name,omitempty"`
}

// PhotoStatus 照片状态
type PhotoStatus string

const (
	PhotoStatusPending  PhotoStatus = "pending"   // 等待处理
	PhotoStatusIndexing PhotoStatus = "indexing"   // 索引中
	PhotoStatusReady    PhotoStatus = "ready"      // 就绪
	PhotoStatusError    PhotoStatus = "error"      // 错误
)

// SearchQuery 搜索查询
type SearchQuery struct {
	Keywords    []string   `json:"keywords,omitempty"`
	PersonIDs   []string   `json:"person_ids,omitempty"`
	PersonNames []string   `json:"person_names,omitempty"`
	Labels      []string   `json:"labels,omitempty"`
	Scenes      []string   `json:"scenes,omitempty"`
	Locations   []LocationRule `json:"locations,omitempty"`

	// 时间范围
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`

	// 相机
	CameraMake  string     `json:"camera_make,omitempty"`
	CameraModel string     `json:"camera_model,omitempty"`

	// 分页
	Page        int        `json:"page"`
	PageSize    int        `json:"page_size"`

	// 排序
	SortBy      string     `json:"sort_by,omitempty"` // date, created, filename
	SortOrder   string     `json:"sort_order,omitempty"` // asc, desc
}

// SearchResult 搜索结果
type SearchResult struct {
	Photos     []Photo `json:"photos"`
	TotalCount int     `json:"total_count"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
	HasMore    bool    `json:"has_more"`
}

// PhotoStats 照片统计
type PhotoStats struct {
	TotalPhotos    int            `json:"total_photos"`
	TotalAlbums    int            `json:"total_albums"`
	TotalPersons   int            `json:"total_persons"`
	TotalSize      int64          `json:"total_size"`
	PhotosByDate   map[string]int `json:"photos_by_date"`
	TopLabels      []LabelCount   `json:"top_labels"`
	TopLocations   []LocationCount `json:"top_locations"`
}

// LabelCount 标签统计
type LabelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// LocationCount 地点统计
type LocationCount struct {
	Location string `json:"location"`
	Count    int    `json:"count"`
}

// BatchOperation 批量操作请求
type BatchOperation struct {
	PhotoIDs []string `json:"photo_ids"`
	Action   string   `json:"action"` // move, delete, tag, favorite, hide
	AlbumID  string   `json:"album_id,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// BatchResult 批量操作结果
type BatchResult struct {
	Success    int      `json:"success"`
	Failed     int      `json:"failed"`
	Errors     []string `json:"errors,omitempty"`
}

// ThumbnailSize 缩略图尺寸
type ThumbnailSize string

const (
	ThumbnailSmall  ThumbnailSize = "small"   // 150x150
	ThumbnailMedium ThumbnailSize = "medium"  // 300x300
	ThumbnailLarge  ThumbnailSize = "large"   // 600x600
)

// PaginationParams 分页参数
type PaginationParams struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// Validate 验证分页参数
func (p *PaginationParams) Validate() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
}

// Offset 计算偏移量
func (p *PaginationParams) Offset() int {
	return (p.Page - 1) * p.PageSize
}
