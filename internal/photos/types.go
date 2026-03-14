// Package photos 提供相册管理功能
package photos

import (
	"time"
)

// Photo 照片信息
type Photo struct {
	ID            string          `json:"id"`
	Filename      string          `json:"filename"`
	Path          string          `json:"path"`
	AlbumID       string          `json:"albumId"`
	UserID        string          `json:"userId"`
	Size          uint64          `json:"size"`
	MimeType      string          `json:"mimeType"`
	Width         int             `json:"width"`
	Height        int             `json:"height"`
	Duration      int             `json:"duration"` // 视频时长（秒），照片为 0
	TakenAt       time.Time       `json:"takenAt"`
	UploadedAt    time.Time       `json:"uploadedAt"`
	ModifiedAt    time.Time       `json:"modifiedAt"`
	EXIF          *EXIFData       `json:"exif,omitempty"`
	ThumbnailPath string          `json:"thumbnailPath"`
	IsFavorite    bool            `json:"isFavorite"`
	IsHidden      bool            `json:"isHidden"`
	Tags          []string        `json:"tags"`
	Faces         []FaceInfo      `json:"faces,omitempty"`
	Objects       []string        `json:"objects,omitempty"`
	Scene         string          `json:"scene,omitempty"`
	ColorPalette  []string        `json:"colorPalette,omitempty"`
	Location      *LocationInfo   `json:"location,omitempty"`
	Device        *DeviceInfo     `json:"device,omitempty"`
	ShareInfo     *ShareInfo      `json:"shareInfo,omitempty"`
	EditHistory   []EditOperation `json:"editHistory,omitempty"`
}

// EXIFData EXIF 元数据
type EXIFData struct {
	Make            string  `json:"make,omitempty"`            // 相机制造商
	Model           string  `json:"model,omitempty"`           // 相机型号
	LensMake        string  `json:"lensMake,omitempty"`        // 镜头制造商
	LensModel       string  `json:"lensModel,omitempty"`       // 镜头型号
	DateTime        string  `json:"dateTime,omitempty"`        // 拍摄时间
	ExposureTime    string  `json:"exposureTime,omitempty"`    // 曝光时间
	FNumber         float64 `json:"fNumber,omitempty"`         // 光圈值
	ISO             int     `json:"iso,omitempty"`             // ISO 感光度
	FocalLength     float64 `json:"focalLength,omitempty"`     // 焦距
	FocalLength35mm int     `json:"focalLength35mm,omitempty"` // 35mm 等效焦距
	Flash           bool    `json:"flash,omitempty"`           // 闪光灯
	WhiteBalance    string  `json:"whiteBalance,omitempty"`    // 白平衡
	ExposureProgram string  `json:"exposureProgram,omitempty"` // 曝光程序
	MeteringMode    string  `json:"meteringMode,omitempty"`    // 测光模式
	Orientation     int     `json:"orientation,omitempty"`     // 方向
	Software        string  `json:"software,omitempty"`        // 处理软件
	Artist          string  `json:"artist,omitempty"`          // 艺术家
	Copyright       string  `json:"copyright,omitempty"`       // 版权信息
	GPSLatitude     float64 `json:"gpsLatitude,omitempty"`     // GPS 纬度
	GPSLongitude    float64 `json:"gpsLongitude,omitempty"`    // GPS 经度
	GPSAltitude     float64 `json:"gpsAltitude,omitempty"`     // GPS 海拔
}

// FaceInfo 人脸信息
type FaceInfo struct {
	ID         string    `json:"id"`
	Name       string    `json:"name,omitempty"`    // 如果已识别
	Bounds     Rectangle `json:"bounds"`            // 人脸边界框
	Confidence float32   `json:"confidence"`        // 识别置信度
	Age        int       `json:"age,omitempty"`     // 估计年龄
	Gender     string    `json:"gender,omitempty"`  // 估计性别
	Emotion    string    `json:"emotion,omitempty"` // 表情
}

// Rectangle 矩形区域
type Rectangle struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// LocationInfo 位置信息
type LocationInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
	City      string  `json:"city,omitempty"`
	Country   string  `json:"country,omitempty"`
	Location  string  `json:"location,omitempty"` // 详细位置描述
}

// DeviceInfo 设备信息
type DeviceInfo struct {
	Brand string `json:"brand,omitempty"`
	Model string `json:"model,omitempty"`
	OS    string `json:"os,omitempty"`
	App   string `json:"app,omitempty"`
}

// ShareInfo 分享信息
type ShareInfo struct {
	ShareID       string    `json:"shareId"`
	ShareURL      string    `json:"shareUrl"`
	Password      string    `json:"password,omitempty"`
	ExpiresAt     time.Time `json:"expiresAt,omitempty"`
	ViewCount     int       `json:"viewCount"`
	DownloadCount int       `json:"downloadCount"`
	AllowDownload bool      `json:"allowDownload"`
}

// EditOperation 编辑操作
type EditOperation struct {
	Type      string      `json:"type"`      // crop, rotate, filter, adjust
	Params    interface{} `json:"params"`    // 编辑参数
	Timestamp time.Time   `json:"timestamp"` // 操作时间
}

// Album 相册
type Album struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	UserID       string        `json:"userId"`
	CoverPhotoID string        `json:"coverPhotoId"`
	PhotoCount   int           `json:"photoCount"`
	IsShared     bool          `json:"isShared"`
	IsFavorite   bool          `json:"isFavorite"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	SharedWith   []ShareTarget `json:"sharedWith,omitempty"`
	Tags         []string      `json:"tags"`
	Location     string        `json:"location,omitempty"`
	ParentID     string        `json:"parentId,omitempty"` // 父相册 ID（支持嵌套相册）
}

// ShareTarget 分享目标
type ShareTarget struct {
	UserID     string    `json:"userId"`
	Username   string    `json:"username"`
	Permission string    `json:"permission"` // view, edit, admin
	SharedAt   time.Time `json:"sharedAt"`
}

// ThumbnailConfig 缩略图配置
type ThumbnailConfig struct {
	SmallSize   int `json:"smallSize"`   // 小缩略图（128x128）
	MediumSize  int `json:"mediumSize"`  // 中缩略图（512x512）
	LargeSize   int `json:"largeSize"`   // 大缩略图（1024x1024）
	OriginalMax int `json:"originalMax"` // 原图最大边（2048）
	Quality     int `json:"quality"`     // JPEG 质量（1-100）
}

// DefaultThumbnailConfig 默认缩略图配置
var DefaultThumbnailConfig = ThumbnailConfig{
	SmallSize:   128,
	MediumSize:  512,
	LargeSize:   1024,
	OriginalMax: 2048,
	Quality:     85,
}

// UploadSession 上传会话（用于断点续传）
type UploadSession struct {
	SessionID      string    `json:"sessionId"`
	UserID         string    `json:"userId"`
	Filename       string    `json:"filename"`
	TotalSize      int64     `json:"totalSize"`
	UploadedSize   int64     `json:"uploadedSize"`
	ChunkSize      int64     `json:"chunkSize"`
	TotalChunks    int       `json:"totalChunks"`
	UploadedChunks []int     `json:"uploadedChunks"`
	CreatedAt      time.Time `json:"createdAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
	TempPath       string    `json:"tempPath"`
}

// PhotoQuery 照片查询条件
type PhotoQuery struct {
	AlbumID    string    `json:"albumId,omitempty"`
	UserID     string    `json:"userId,omitempty"`
	StartDate  time.Time `json:"startDate,omitempty"`
	EndDate    time.Time `json:"endDate,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	PersonIDs  []string  `json:"personIds,omitempty"`
	ObjectTags []string  `json:"objectTags,omitempty"`
	Scene      string    `json:"scene,omitempty"`
	City       string    `json:"city,omitempty"`
	Country    string    `json:"country,omitempty"`
	IsFavorite *bool     `json:"isFavorite,omitempty"`
	IsHidden   *bool     `json:"isHidden,omitempty"`
	MimeType   string    `json:"mimeType,omitempty"` // image/jpeg, image/png, video/mp4
	MinWidth   int       `json:"minWidth,omitempty"`
	MinHeight  int       `json:"minHeight,omitempty"`
	Search     string    `json:"search,omitempty"` // 全文搜索
	SortBy     string    `json:"sortBy"`           // takenAt, uploadedAt, modifiedAt
	SortOrder  string    `json:"sortOrder"`        // asc, desc
	Limit      int       `json:"limit"`
	Offset     int       `json:"offset"`
}

// TimelineGroup 时间线分组
type TimelineGroup struct {
	Period   string   `json:"period"` // 2026-03, 2026-03-12
	Photos   []*Photo `json:"photos"`
	Count    int      `json:"count"`
	Location string   `json:"location,omitempty"`
}

// Person 人物（人脸识别结果）
type Person struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	PhotoCount   int       `json:"photoCount"`
	CoverPhotoID string    `json:"coverPhotoId"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// AIClassification AI 分类结果
type AIClassification struct {
	PhotoID      string                 `json:"photoId"`
	Faces        []FaceInfo             `json:"faces"`
	Objects      []string               `json:"objects"`
	Scene        string                 `json:"scene"`
	Colors       []string               `json:"colors"`
	IsNSFW       bool                   `json:"isNsfw"`
	Confidence   float32                `json:"confidence"`
	QualityScore float32                `json:"qualityScore"` // 照片质量评分 (0-100)
	AutoTags     []string               `json:"autoTags"`     // 自动生成的标签
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// QualityMetrics 照片质量指标
type QualityMetrics struct {
	Brightness   float64 `json:"brightness"`   // 亮度 (0-255)
	Contrast     float64 `json:"contrast"`     // 对比度
	Sharpness    float64 `json:"sharpness"`    // 清晰度
	Colorfulness float64 `json:"colorfulness"` // 色彩丰富度
	Composition  float64 `json:"composition"`  // 构图评分 (基于三分法等)
	OverallScore float32 `json:"overallScore"` // 综合质量评分 (0-100)
}
