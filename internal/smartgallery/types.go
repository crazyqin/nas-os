// Package smartgallery 提供智能相册功能，支持人脸识别、场景分类和时间线生成。
// 在现有海报墙功能基础上，增加智能照片管理能力。
package smartgallery

import "time"

// Photo 照片
type Photo struct {
	ID         string     `json:"id"`
	FilePath   string     `json:"file_path"`
	FileName   string     `json:"file_name"`
	MimeType   string     `json:"mime_type"`
	Size       int64      `json:"size"`
	Width      int        `json:"width,omitempty"`
	Height     int        `json:"height,omitempty"`
	ShotAt     time.Time  `json:"shot_at"`
	UploadedAt time.Time  `json:"uploaded_at"`
	GPS        *GPSInfo   `json:"gps,omitempty"`
	Camera     string     `json:"camera,omitempty"`
	Tags       []SmartTag `json:"tags,omitempty"`
	Faces      []Face     `json:"faces,omitempty"`
	Scenes     []Scene    `json:"scenes,omitempty"`
	IsFavorite bool       `json:"is_favorite"`
	IsHidden   bool       `json:"is_hidden"`
	Rating     int        `json:"rating,omitempty"` // 1-5
}

// GPSInfo GPS 信息
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// Face 人脸
type Face struct {
	ID          string      `json:"id"`
	PhotoID     string      `json:"photo_id"`
	PersonID    string      `json:"person_id,omitempty"`
	PersonName  string      `json:"person_name,omitempty"`
	BoundingBox BoundingBox `json:"bounding_box"`
	Confidence  float64     `json:"confidence"` // 0-1
}

// BoundingBox 边界框
type BoundingBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Scene 场景
type Scene struct {
	ID         string  `json:"id"`
	PhotoID    string  `json:"photo_id"`
	Label      string  `json:"label"`       // 海滩、山脉、城市等
	Category   string  `json:"category"`    // nature, urban, indoor, etc.
	Confidence float64 `json:"confidence"`  // 0-1
}

// SmartTag 智能标签
type SmartTag struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Category   string  `json:"category"`   // object, scene, activity, emotion
	Confidence float64 `json:"confidence"` // 0-1
}

// DuplicateGroup 重复照片组
type DuplicateGroup struct {
	ID         string   `json:"id"`
	PhotoIDs   []string `json:"photo_ids"`
	Hash       string   `json:"hash"`       // 感知哈希
	Similarity float64  `json:"similarity"` // 相似度
	SavedSize  int64    `json:"saved_size"` // 去重后可节省的空间
}

// Timeline 时间线
type Timeline struct {
	Date      string  `json:"date"` // YYYY-MM-DD
	Photos    []Photo `json:"photos"`
	Count     int     `json:"count"`
	Highlight *Photo  `json:"highlight,omitempty"` // 当日精选
}

// Person 人物
type Person struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	PhotoCount int      `json:"photo_count"`
	CoverPhoto string   `json:"cover_photo,omitempty"`
	FaceIDs    []string `json:"face_ids"`
}

// PhotoSearchRequest 搜索请求
type PhotoSearchRequest struct {
	Query      string   `json:"query,omitempty"`
	DateFrom   string   `json:"date_from,omitempty"` // YYYY-MM-DD
	DateTo     string   `json:"date_to,omitempty"`   // YYYY-MM-DD
	Tags       []string `json:"tags,omitempty"`
	Scenes     []string `json:"scenes,omitempty"`
	Persons    []string `json:"persons,omitempty"`
	IsFavorite *bool    `json:"is_favorite,omitempty"`
	Page       int      `json:"page,omitempty"`
	PageSize   int      `json:"page_size,omitempty"`
}

// PhotoSearchResult 搜索结果
type PhotoSearchResult struct {
	Photos   []Photo `json:"photos"`
	Total    int     `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

// PhotoGallery 相册（照片集合）
type PhotoGallery struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CoverURL    string    `json:"cover_url,omitempty"`
	PhotoCount  int       `json:"photo_count"`
	Type        string    `json:"type"` // manual, smart, timeline, face, scene
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
