// Package aiphoto 提供 AI 智能相册功能
// 对标飞牛fnOS的AI相册功能
// 支持人脸识别、场景分类、智能搜索、时间线/地图视图、自动去重
package aiphoto

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrPhotoNotFound 照片未找到
	ErrPhotoNotFound = errors.New("照片未找到")
	// ErrFaceNotFound 人脸未找到
	ErrFaceNotFound = errors.New("人脸未找到")
	// ErrPersonNotFound 人物未找到
	ErrPersonNotFound = errors.New("人物未找到")
	// ErrAlbumNotFound 相册未找到
	ErrAlbumNotFound = errors.New("相册未找到")
	// ErrInvalidImage 无效图片
	ErrInvalidImage = errors.New("无效图片")
	// ErrModelNotFound AI模型未找到
	ErrModelNotFound = errors.New("AI模型未找到")
)

// ========== 场景分类 ==========

// SceneCategory 场景分类
type SceneCategory string

const (
	// SceneLandscape 风景
	SceneLandscape SceneCategory = "landscape"
	// ScenePortrait 人物
	ScenePortrait SceneCategory = "portrait"
	// SceneFood 美食
	SceneFood SceneCategory = "food"
	// SceneDocument 文档
	SceneDocument SceneCategory = "document"
	// SceneAnimal 动物
	SceneAnimal SceneCategory = "animal"
	// SceneArchitecture 建筑
	SceneArchitecture SceneCategory = "architecture"
	// SceneVehicle 交通工具
	SceneVehicle SceneCategory = "vehicle"
	// SceneIndoor 室内
	SceneIndoor SceneCategory = "indoor"
	// SceneNight 夜景
	SceneNight SceneCategory = "night"
	// SceneOther 其他
	SceneOther SceneCategory = "other"
)

// ========== 人脸相关 ==========

// Face 人脸信息
type Face struct {
	ID        string    `json:"id"`
	PhotoID   string    `json:"photo_id"`
	PersonID  string    `json:"person_id,omitempty"` // 关联的人物ID
	BoundingBox BoundingBox `json:"bounding_box"`
	Confidence float64   `json:"confidence"` // 置信度 0-1
	Embedding  []float32 `json:"embedding,omitempty"` // 人脸特征向量
	Age        int       `json:"age,omitempty"`
	Gender     string    `json:"gender,omitempty"` // male, female
	Emotion    string    `json:"emotion,omitempty"` // happy, sad, neutral, etc.
	CreatedAt  time.Time `json:"created_at"`
}

// BoundingBox 边界框
type BoundingBox struct {
	X      int `json:"x"`      // 左上角X
	Y      int `json:"y"`      // 左上角Y
	Width  int `json:"width"`  // 宽度
	Height int `json:"height"` // 高度
}

// Person 人物信息
type Person struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	FaceCount   int       `json:"face_count"`   // 关联的人脸数量
	PhotoCount  int       `json:"photo_count"`  // 包含此人物的照片数量
	CoverFaceID string    `json:"cover_face_id"` // 封面人脸ID
	IsNamed     bool      `json:"is_named"`      // 是否已命名
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ========== 照片相关 ==========

// Photo 照片信息
type Photo struct {
	ID            string          `json:"id"`
	Path          string          `json:"path"`
	Filename      string          `json:"filename"`
	MimeType      string          `json:"mime_type"`
	Size          int64           `json:"size"` // bytes
	Width         int             `json:"width"`
	Height        int             `json:"height"`
	Orientation   int             `json:"orientation"` // EXIF方向

	// 时间信息
	TakenAt       time.Time       `json:"taken_at"`     // 拍摄时间
	CreatedAt     time.Time       `json:"created_at"`   // 创建时间
	ModifiedAt    time.Time       `json:"modified_at"`  // 修改时间

	// AI分析结果
	Scene         SceneCategory   `json:"scene"`
	SceneConfidence float64       `json:"scene_confidence"`
	Faces         []*Face         `json:"faces,omitempty"`
	Tags          []string        `json:"tags,omitempty"`
	Description   string          `json:"description,omitempty"` // AI生成的描述

	// 位置信息
	Latitude      float64         `json:"latitude,omitempty"`
	Longitude     float64         `json:"longitude,omitempty"`
	LocationName  string          `json:"location_name,omitempty"`

	// 哈希（用于去重）
	PerceptualHash string         `json:"perceptual_hash"` // 感知哈希
	AverageHash    string         `json:"average_hash"`    // 均值哈希

	// 元数据
	EXIF          map[string]string `json:"exif,omitempty"`
	IsFavorite    bool              `json:"is_favorite"`
	IsHidden      bool              `json:"is_hidden"`
	AlbumIDs      []string          `json:"album_ids,omitempty"`
}

// PhotoCreateRequest 创建照片请求
type PhotoCreateRequest struct {
	Path        string `json:"path"`
	Analyze     bool   `json:"analyze"`      // 是否进行AI分析
	DetectFaces bool   `json:"detect_faces"` // 是否检测人脸
}

// PhotoSearchRequest 搜索照片请求
type PhotoSearchRequest struct {
	Query       string          `json:"query,omitempty"`        // 文字描述搜索
	Scene       SceneCategory   `json:"scene,omitempty"`        // 场景过滤
	PersonID    string          `json:"person_id,omitempty"`    // 人物过滤
	AlbumID     string          `json:"album_id,omitempty"`     // 相册过滤
	Tags        []string        `json:"tags,omitempty"`         // 标签过滤
	StartDate   *time.Time      `json:"start_date,omitempty"`   // 开始日期
	EndDate     *time.Time      `json:"end_date,omitempty"`     // 结束日期
	Location    *LocationFilter `json:"location,omitempty"`     // 位置过滤
	IsFavorite  *bool           `json:"is_favorite,omitempty"`  // 收藏过滤
	SortBy      string          `json:"sort_by,omitempty"`      // 排序字段
	SortOrder   string          `json:"sort_order,omitempty"`   // asc, desc
	Page        int             `json:"page"`
	PageSize    int             `json:"page_size"`
}

// LocationFilter 位置过滤
type LocationFilter struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RadiusKm  float64 `json:"radius_km"` // 半径（公里）
}

// PhotoSearchResult 搜索结果
type PhotoSearchResult struct {
	Photos     []*Photo `json:"photos"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
	TotalPages int      `json:"total_pages"`
}

// ========== 相册相关 ==========

// AlbumType 相册类型
type AlbumType string

const (
	// AlbumTypeUser 用户创建的相册
	AlbumTypeUser AlbumType = "user"
	// AlbumTypeSmart 智能相册（AI自动分类）
	AlbumTypeSmart AlbumType = "smart"
	// AlbumTypeTimeline 时间线相册
	AlbumTypeTimeline AlbumType = "timeline"
	// AlbumTypeMap 地图相册
	AlbumTypeMap AlbumType = "map"
	// AlbumTypeFace 人物相册
	AlbumTypeFace AlbumType = "face"
)

// Album 相册信息
type Album struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Type        AlbumType `json:"type"`
	CoverURL    string    `json:"cover_url,omitempty"`
	PhotoCount  int       `json:"photo_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 智能相册条件
	SmartRules  *SmartRules `json:"smart_rules,omitempty"`
}

// SmartRules 智能相册规则
type SmartRules struct {
	Scene      SceneCategory `json:"scene,omitempty"`
	PersonID   string        `json:"person_id,omitempty"`
	Tags       []string      `json:"tags,omitempty"`
	MinRating  int           `json:"min_rating,omitempty"`
	TimeRange  string        `json:"time_range,omitempty"` // today, week, month, year
}

// ========== 去重相关 ==========

// DuplicateGroup 重复照片组
type DuplicateGroup struct {
	ID       string   `json:"id"`
	PhotoIDs []string `json:"photo_ids"`
	Hash     string   `json:"hash"`
	Score    float64  `json:"score"` // 相似度 0-1
}

// ========== 统计相关 ==========

// PhotoStats 照片统计
type PhotoStats struct {
	TotalPhotos    int                    `json:"total_photos"`
	TotalFaces     int                    `json:"total_faces"`
	TotalPersons   int                    `json:"total_persons"`
	TotalAlbums    int                    `json:"total_albums"`
	NamedPersons   int                    `json:"named_persons"`
	UnnamedPersons int                    `json:"unnamed_persons"`
	SceneCounts    map[SceneCategory]int  `json:"scene_counts"`
	YearCounts     map[int]int            `json:"year_counts"` // 年份 -> 数量
	MonthCounts    map[string]int         `json:"month_counts"` // YYYY-MM -> 数量
	TopTags        []TagCount             `json:"top_tags"`
	StorageUsed    int64                  `json:"storage_used"` // bytes
}

// TagCount 标签计数
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ========== 配置相关 ==========

// PhotoConfig 相册配置
type PhotoConfig struct {
	// 存储配置
	PhotoDir       string `json:"photo_dir"`        // 照片存储目录
	ThumbnailDir   string `json:"thumbnail_dir"`    // 缩略图目录
	MaxThumbnailSize int  `json:"max_thumbnail_size"` // 最大缩略图尺寸

	// AI配置
	FaceDetectionEnabled  bool    `json:"face_detection_enabled"`
	SceneClassificationEnabled bool `json:"scene_classification_enabled"`
	AutoTaggingEnabled    bool    `json:"auto_tagging_enabled"`
	FaceConfidenceThreshold float64 `json:"face_confidence_threshold"` // 人脸置信度阈值

	// 去重配置
	DedupEnabled      bool    `json:"dedup_enabled"`
	DedupSimilarityThreshold float64 `json:"dedup_similarity_threshold"` // 相似度阈值

	// 性能配置
	MaxConcurrentAnalysis int `json:"max_concurrent_analysis"` // 最大并发分析数
	AnalysisQueueSize     int `json:"analysis_queue_size"`     // 分析队列大小
}

// DefaultPhotoConfig 默认配置
func DefaultPhotoConfig() *PhotoConfig {
	return &PhotoConfig{
		PhotoDir:       "/data/photos",
		ThumbnailDir:   "/data/photos/.thumbnails",
		MaxThumbnailSize: 300,
		FaceDetectionEnabled:  true,
		SceneClassificationEnabled: true,
		AutoTaggingEnabled:    true,
		FaceConfidenceThreshold: 0.7,
		DedupEnabled:      true,
		DedupSimilarityThreshold: 0.95,
		MaxConcurrentAnalysis: 4,
		AnalysisQueueSize:     100,
	}
}
