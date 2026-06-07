// Package photoai 提供照片AI管理功能，包括智能分类、人脸聚类、EXIF提取、智能相册等。
// 参考群晖 Synology Photos 和飞牛 AI 相册。
package photoai

import "time"

// PhotoStatus 照片状态
type PhotoStatus string

const (
	StatusPending    PhotoStatus = "pending"
	StatusProcessing PhotoStatus = "processing"
	StatusReady      PhotoStatus = "ready"
	StatusFailed     PhotoStatus = "failed"
)

// PhotoCategory 照片分类
type PhotoCategory string

const (
	CategoryLandscape PhotoCategory = "landscape"
	CategoryPortrait  PhotoCategory = "portrait"
	CategoryFood      PhotoCategory = "food"
	CategoryAnimal    PhotoCategory = "animal"
	CategoryDocument  PhotoCategory = "document"
	CategoryVehicle   PhotoCategory = "vehicle"
	CategoryBuilding  PhotoCategory = "building"
	CategoryPlant     PhotoCategory = "plant"
	CategoryOther     PhotoCategory = "other"
)

// Photo 照片信息
type Photo struct {
	ID         string          `json:"id"`
	Filename   string          `json:"filename"`
	FilePath   string          `json:"file_path"`
	FileSize   int64           `json:"file_size"`
	MimeType   string          `json:"mime_type"`
	Width      int             `json:"width"`
	Height     int             `json:"height"`
	Status     PhotoStatus     `json:"status"`
	Categories []PhotoCategory `json:"categories,omitempty"`
	Tags       []string        `json:"tags,omitempty"`
	Score      float64         `json:"score"`                // 美学评分 0-100
	Duplicates []string        `json:"duplicates,omitempty"` // 重复照片ID列表
	EXIF       *EXIFData       `json:"exif,omitempty"`
	Faces      []*FaceInfo     `json:"faces,omitempty"`
	Albums     []string        `json:"albums,omitempty"` // 所属智能相册ID
	IsFavorite bool            `json:"is_favorite"`
	ShareLinks []*ShareLink    `json:"share_links,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	TakenAt    *time.Time      `json:"taken_at,omitempty"` // 拍摄时间
}

// EXIFData EXIF元数据
type EXIFData struct {
	CameraMake   string    `json:"camera_make,omitempty"`
	CameraModel  string    `json:"camera_model,omitempty"`
	LensModel    string    `json:"lens_model,omitempty"`
	ISO          int       `json:"iso,omitempty"`
	Aperture     float64   `json:"aperture,omitempty"` // 光圈 f 值
	ShutterSpeed string    `json:"shutter_speed,omitempty"`
	FocalLength  float64   `json:"focal_length,omitempty"`
	Flash        bool      `json:"flash,omitempty"`
	Orientation  int       `json:"orientation,omitempty"`
	GPS          *GPSData  `json:"gps,omitempty"`
	TakenAt      time.Time `json:"taken_at"`
}

// GPSData GPS坐标
type GPSData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
	Address   string  `json:"address,omitempty"` // 反向地理编码地址
}

// FaceInfo 人脸信息
type FaceInfo struct {
	ID          string    `json:"id"`
	PersonID    string    `json:"person_id,omitempty"` // 聚类后的人物ID
	PersonName  string    `json:"person_name,omitempty"`
	Confidence  float64   `json:"confidence"` // 置信度 0-1
	BoundingBox *Rect     `json:"bounding_box"`
	Embedding   []float64 `json:"embedding,omitempty"` // 人脸特征向量
	CreatedAt   time.Time `json:"created_at"`
}

// Rect 矩形区域
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Person 人物信息（聚类结果）
type Person struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	PhotoCount int       `json:"photo_count"`
	CoverPhoto string    `json:"cover_photo,omitempty"` // 封面照片ID
	FaceIDs    []string  `json:"face_ids,omitempty"`
	PhotoIDs   []string  `json:"photo_ids,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SmartAlbum 智能相册
type SmartAlbum struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Type        AlbumType   `json:"type"`
	Rules       []AlbumRule `json:"rules"` // 自动归类规则
	PhotoCount  int         `json:"photo_count"`
	CoverPhoto  string      `json:"cover_photo,omitempty"`
	PhotoIDs    []string    `json:"photo_ids,omitempty"`
	IsSystem    bool        `json:"is_system"` // 系统预设相册
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// AlbumType 相册类型
type AlbumType string

const (
	AlbumTypeTime     AlbumType = "time"     // 按时间
	AlbumTypeLocation AlbumType = "location" // 按地点
	AlbumTypePerson   AlbumType = "person"   // 按人物
	AlbumTypeTag      AlbumType = "tag"      // 按标签
	AlbumTypeScore    AlbumType = "score"    // 按评分
	AlbumTypeCustom   AlbumType = "custom"   // 自定义规则
)

// AlbumRule 相册规则
type AlbumRule struct {
	Field    string      `json:"field"`    // category, tag, person_id, location, date_range, score
	Operator string      `json:"operator"` // eq, contains, gt, lt, between, in
	Value    interface{} `json:"value"`
}

// ShareLink 分享链接
type ShareLink struct {
	ID        string     `json:"id"`
	Token     string     `json:"token"`
	PhotoIDs  []string   `json:"photo_ids"`
	Password  string     `json:"password,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	MaxViews  int        `json:"max_views,omitempty"` // 最大查看次数
	ViewCount int        `json:"view_count"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy string     `json:"created_by,omitempty"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Keywords   string          `json:"keywords,omitempty"`
	Categories []PhotoCategory `json:"categories,omitempty"`
	Tags       []string        `json:"tags,omitempty"`
	PersonIDs  []string        `json:"person_ids,omitempty"`
	DateFrom   *time.Time      `json:"date_from,omitempty"`
	DateTo     *time.Time      `json:"date_to,omitempty"`
	Location   *LocationFilter `json:"location,omitempty"`
	MinScore   *float64        `json:"min_score,omitempty"`
	SortBy     string          `json:"sort_by,omitempty"`    // date, score, filename
	SortOrder  string          `json:"sort_order,omitempty"` // asc, desc
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
}

// LocationFilter 位置过滤
type LocationFilter struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RadiusKm  float64 `json:"radius_km"` // 半径（公里）
}

// SearchResult 搜索结果
type SearchResult struct {
	Photos   []*Photo `json:"photos"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

// ScanRequest 扫描请求
type ScanRequest struct {
	Directory   string `json:"directory" binding:"required"`
	Recursive   bool   `json:"recursive"`
	ForceRescan bool   `json:"force_rescan"` // 强制重新扫描
}

// ScanResult 扫描结果
type ScanResult struct {
	TotalFound  int      `json:"total_found"`
	NewImported int      `json:"new_imported"`
	Skipped     int      `json:"skipped"`
	Errors      []string `json:"errors,omitempty"`
	Duration    string   `json:"duration"`
}

// ImportRequest 导入请求
type ImportRequest struct {
	Paths     []string `json:"paths" binding:"required,min=1"` // 文件或目录路径
	Recursive bool     `json:"recursive"`
}

// ImportResult 导入结果
type ImportResult struct {
	TotalFiles int      `json:"total_files"`
	Imported   int      `json:"imported"`
	Skipped    int      `json:"skipped"`
	Failed     int      `json:"failed"`
	Errors     []string `json:"errors,omitempty"`
}

// ThumbnailConfig 缩略图配置
type ThumbnailConfig struct {
	Sizes   []ThumbnailSize `json:"sizes"`
	Quality int             `json:"quality"` // 1-100
	Format  string          `json:"format"`  // jpeg, webp
}

// ThumbnailSize 缩略图尺寸
type ThumbnailSize struct {
	Name   string `json:"name"` // small, medium, large
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// DuplicateGroup 重复照片组
type DuplicateGroup struct {
	Hash     string   `json:"hash"` // 感知哈希
	PhotoIDs []string `json:"photo_ids"`
}

// PhotoAIConfig 照片AI配置
type PhotoAIConfig struct {
	Enabled                   bool             `json:"enabled"`
	LibraryPath               string           `json:"library_path"`   // 照片库根目录
	ThumbnailPath             string           `json:"thumbnail_path"` // 缩略图存储目录
	DBPath                    string           `json:"db_path"`        // 数据库路径
	AIEnabled                 bool             `json:"ai_enabled"`     // AI功能开关
	FaceClusteringEnabled     bool             `json:"face_clustering"`
	ScoreEnabled              bool             `json:"score_enabled"` // 美学评分开关
	DuplicateDetectionEnabled bool             `json:"duplicate_detection"`
	ThumbnailConfig           *ThumbnailConfig `json:"thumbnail_config"`
	MaxConcurrency            int              `json:"max_concurrency"`   // 并发处理数
	SupportedFormats          []string         `json:"supported_formats"` // 支持的图片格式
}

// DefaultPhotoAIConfig 默认配置
func DefaultPhotoAIConfig() *PhotoAIConfig {
	return &PhotoAIConfig{
		Enabled:                   true,
		LibraryPath:               "/data/photos",
		ThumbnailPath:             "/data/thumbnails",
		DBPath:                    "/data/photoai.db",
		AIEnabled:                 true,
		FaceClusteringEnabled:     true,
		ScoreEnabled:              true,
		DuplicateDetectionEnabled: true,
		MaxConcurrency:            4,
		SupportedFormats:          []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".heif", ".tiff", ".bmp", ".raw", ".cr2", ".nef", ".arw"},
		ThumbnailConfig: &ThumbnailConfig{
			Quality: 85,
			Format:  "jpeg",
			Sizes: []ThumbnailSize{
				{Name: "small", Width: 150, Height: 150},
				{Name: "medium", Width: 400, Height: 400},
				{Name: "large", Width: 800, Height: 800},
			},
		},
	}
}

// AlbumRequest 创建/更新相册请求
type AlbumRequest struct {
	Name        string      `json:"name" binding:"required"`
	Description string      `json:"description,omitempty"`
	Type        AlbumType   `json:"type" binding:"required"`
	Rules       []AlbumRule `json:"rules" binding:"required,min=1"`
}

// ShareRequest 创建分享请求
type ShareRequest struct {
	PhotoIDs  []string `json:"photo_ids" binding:"required,min=1"`
	Password  string   `json:"password,omitempty"`
	ExpiresIn int      `json:"expires_in,omitempty"` // 过期时间（小时）
	MaxViews  int      `json:"max_views,omitempty"`
}

// RenamePersonRequest 重命名人物请求
type RenamePersonRequest struct {
	Name string `json:"name" binding:"required"`
}

// FavoriteRequest 收藏请求
type FavoriteRequest struct {
	IsFavorite bool `json:"is_favorite"`
}

// BatchTagRequest 批量标签请求
type BatchTagRequest struct {
	PhotoIDs []string `json:"photo_ids" binding:"required,min=1"`
	Tags     []string `json:"tags" binding:"required,min=1"`
	Action   string   `json:"action"` // add, remove, set
}

// AIAnalysisResult AI分析结果
type AIAnalysisResult struct {
	Categories []PhotoCategory `json:"categories"`
	Tags       []string        `json:"tags"`
	Score      float64         `json:"score"`
	Faces      []*FaceInfo     `json:"faces,omitempty"`
}
