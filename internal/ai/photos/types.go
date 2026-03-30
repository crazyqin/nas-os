// Package photos provides AI-powered photo classification for NAS-OS
// Features: face recognition, location-based grouping, scene classification
// Inspired by Synology Photos and fnOS photo management
package photos

import (
	"time"
)

// Photo represents a photo entity with metadata
type Photo struct {
	ID         string               `json:"id"`
	Path       string               `json:"path"`
	Filename   string               `json:"filename"`
	Size       int64                `json:"size"`
	Width      int                  `json:"width"`
	Height     int                  `json:"height"`
	Format     string               `json:"format"` // jpeg, png, heic, etc.
	CreatedAt  time.Time            `json:"created_at"`
	ModifiedAt time.Time            `json:"modified_at"`
	TakenAt    *time.Time           `json:"taken_at,omitempty"` // EXIF DateTimeOriginal
	GPS        *GPSInfo             `json:"gps,omitempty"`
	Camera     *CameraInfo          `json:"camera,omitempty"`
	Faces      []FaceDetection      `json:"faces,omitempty"`
	Scene      *SceneClassification `json:"scene,omitempty"`
	Tags       []string             `json:"tags,omitempty"`
	Hash       string               `json:"hash"` // perceptual hash for dedup
	Thumbnail  string               `json:"thumbnail,omitempty"`
	Metadata   map[string]any       `json:"metadata,omitempty"`
}

// GPSInfo contains GPS coordinates and derived location
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`  // -90 to 90
	Longitude float64 `json:"longitude"` // -180 to 180
	Altitude  float64 `json:"altitude,omitempty"`

	// Derived location info (from reverse geocoding)
	Location *Location `json:"location,omitempty"`
}

// Location is the reverse-geocoded location
type Location struct {
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	Province    string `json:"province,omitempty"` // 省/州
	City        string `json:"city,omitempty"`     // 市
	District    string `json:"district,omitempty"` // 区/县
	Street      string `json:"street,omitempty"`   // 街道
	POI         string `json:"poi,omitempty"`      // 兴趣点名称
	PlaceID     string `json:"place_id,omitempty"` // 地点唯一标识
}

// CameraInfo contains camera make/model from EXIF
type CameraInfo struct {
	Make         string `json:"make,omitempty"`          // Apple, Canon, Nikon, etc.
	Model        string `json:"model,omitempty"`         // iPhone 15 Pro, EOS R5, etc.
	Lens         string `json:"lens,omitempty"`          // Lens model
	Aperture     string `json:"aperture,omitempty"`      // f/1.8
	ShutterSpeed string `json:"shutter_speed,omitempty"` // 1/125
	ISO          int    `json:"iso,omitempty"`
	FocalLength  string `json:"focal_length,omitempty"` // 24mm
}

// FaceDetection represents a detected face in a photo
type FaceDetection struct {
	ID          string      `json:"id"`
	PhotoID     string      `json:"photo_id"`
	BoundingBox BoundingBox `json:"bounding_box"`
	Landmarks   []Landmark  `json:"landmarks,omitempty"`
	Embedding   []float32   `json:"embedding,omitempty"` // 512-d face embedding
	Quality     float64     `json:"quality"`             // detection confidence
	BlurScore   float64     `json:"blur_score,omitempty"`
	PersonID    string      `json:"person_id,omitempty"`   // assigned person
	PersonName  string      `json:"person_name,omitempty"` // user-defined name
	ClusterID   int         `json:"cluster_id,omitempty"`  // for clustering
	CreatedAt   time.Time   `json:"created_at"`
}

// BoundingBox represents face location in image (normalized 0-1)
type BoundingBox struct {
	X      float64 `json:"x"`      // top-left x
	Y      float64 `json:"y"`      // top-left y
	Width  float64 `json:"width"`  // width
	Height float64 `json:"height"` // height
}

// Landmark represents a facial landmark point
type Landmark struct {
	Type string  `json:"type"` // "left_eye", "right_eye", "nose", "left_mouth", "right_mouth"
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

// Person represents a recognized person cluster
type Person struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name,omitempty"`                // user-defined name
	RepresentativeFace string    `json:"representative_face,omitempty"` // representative face photo
	FaceCount          int       `json:"face_count"`
	CoverPhotoID       string    `json:"cover_photo_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// SceneClassification contains scene analysis results
type SceneClassification struct {
	PrimaryCategory SceneCategory     `json:"primary_category"`
	Categories      []CategoryScore   `json:"categories"`
	Objects         []ObjectDetection `json:"objects,omitempty"`
	Colors          []ColorInfo       `json:"colors,omitempty"`
	Quality         PhotoQuality      `json:"quality"`
	AestheticScore  float64           `json:"aesthetic_score,omitempty"` // 0-1, photo aesthetic quality
}

// SceneCategory is the main scene type
type SceneCategory string

// Scene category constants
const (
	ScenePortrait     SceneCategory = "portrait"     // 人像
	SceneLandscape    SceneCategory = "landscape"    // 风景
	SceneFood         SceneCategory = "food"         // 美食
	ScenePet          SceneCategory = "pet"          // 宠物
	SceneArchitecture SceneCategory = "architecture" // 建筑
	SceneNature       SceneCategory = "nature"       // 自然风光
	SceneNight        SceneCategory = "night"        // 夜景
	SceneBeach        SceneCategory = "beach"        // 海滩
	SceneMountain     SceneCategory = "mountain"     // 山景
	SceneCity         SceneCategory = "city"         // 城市街景
	SceneIndoor       SceneCategory = "indoor"       // 室内
	SceneEvent        SceneCategory = "event"        // 活动庆典
	SceneDocument     SceneCategory = "document"     // 文档/截图
	SceneVehicle      SceneCategory = "vehicle"      // 交通工具
	SceneOther        SceneCategory = "other"        // 其他
)

// CategoryScore represents a category with confidence
type CategoryScore struct {
	Category SceneCategory `json:"category"`
	Score    float64       `json:"score"`              // 0-1 confidence
	SubType  string        `json:"sub_type,omitempty"` // e.g., "cat" for pet, "pizza" for food
}

// ObjectDetection represents a detected object
type ObjectDetection struct {
	Label string      `json:"label"` // "person", "car", "dog", etc.
	Score float64     `json:"score"`
	Box   BoundingBox `json:"box,omitempty"`
}

// ColorInfo represents a dominant color
type ColorInfo struct {
	Hex     string  `json:"hex"`
	Name    string  `json:"name"`    // "red", "blue", "green", etc.
	Percent float64 `json:"percent"` // percentage in image
}

// PhotoQuality contains quality metrics
type PhotoQuality struct {
	Score      float64 `json:"score"` // overall quality 0-1
	Sharpness  float64 `json:"sharpness"`
	Exposure   float64 `json:"exposure"` // 0=underexposed, 1=overexposed
	Contrast   float64 `json:"contrast"`
	IsBlurred  bool    `json:"is_blurred"`
	IsLowLight bool    `json:"is_low_light"`
	HasRedEye  bool    `json:"has_red_eye"`
}

// Album represents an auto-generated or user-created album
type Album struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Type         AlbumType  `json:"type"`
	PhotoCount   int        `json:"photo_count"`
	CoverPhotoID string     `json:"cover_photo_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	Location     *Location  `json:"location,omitempty"`  // for location albums
	PersonID     string     `json:"person_id,omitempty"` // for person albums
}

// AlbumType indicates how the album was created
type AlbumType string

// Album type constants
const (
	AlbumTypeAuto     AlbumType = "auto"     // auto-generated
	AlbumTypePerson   AlbumType = "person"   // person album
	AlbumTypeLocation AlbumType = "location" // location album
	AlbumTypeDate     AlbumType = "date"     // date-based album
	AlbumTypeScene    AlbumType = "scene"    // scene-based album
	AlbumTypeSmart    AlbumType = "smart"    // smart album (rule-based)
	AlbumTypeUser     AlbumType = "user"     // user-created
)

// ClassificationResult is the result of classifying a single photo
type ClassificationResult struct {
	PhotoID     string               `json:"photo_id"`
	Faces       []FaceDetection      `json:"faces,omitempty"`
	Scene       *SceneClassification `json:"scene,omitempty"`
	Location    *Location            `json:"location,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
	Error       string               `json:"error,omitempty"`
	ProcessTime time.Duration        `json:"process_time"`
}

// ClusterResult represents face clustering results
type ClusterResult struct {
	Persons      []Person        `json:"persons"`
	Unassigned   []FaceDetection `json:"unassigned"` // faces not assigned to a cluster
	ClusterCount int             `json:"cluster_count"`
}

// Config holds photo classification service configuration
type Config struct {
	// Face recognition settings
	EnableFaceRecognition bool    `json:"enable_face_recognition"`
	FaceModel             string  `json:"face_model"`             // "arcface", "facenet", etc.
	FaceMinSize           int     `json:"face_min_size"`          // minimum face size in pixels
	FaceClusterThreshold  float64 `json:"face_cluster_threshold"` // 0-1, similarity threshold

	// Scene classification settings
	EnableSceneClassification bool   `json:"enable_scene_classification"`
	SceneModel                string `json:"scene_model"` // "clip", "resnet", etc.
	SceneTopK                 int    `json:"scene_top_k"` // return top K categories

	// Location settings
	EnableLocationLookup bool   `json:"enable_location_lookup"`
	GeocodingProvider    string `json:"geocoding_provider"` // "nominatim", "google", "baidu"
	GeocodingAPIKey      string `json:"geocoding_api_key,omitempty"`
	GeocodingCacheSize   int    `json:"geocoding_cache_size"`

	// Processing settings
	BatchSize  int  `json:"batch_size"`
	MaxWorkers int  `json:"max_workers"`
	EnableGPU  bool `json:"enable_gpu"`

	// Cache settings
	CacheEnabled bool `json:"cache_enabled"`
	CacheTTL     int  `json:"cache_ttl_seconds"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		EnableFaceRecognition: true,
		FaceModel:             "arcface",
		FaceMinSize:           30,
		FaceClusterThreshold:  0.6,

		EnableSceneClassification: true,
		SceneModel:                "clip",
		SceneTopK:                 5,

		EnableLocationLookup: true,
		GeocodingProvider:    "nominatim",
		GeocodingCacheSize:   10000,

		BatchSize:  32,
		MaxWorkers: 4,
		EnableGPU:  false,

		CacheEnabled: true,
		CacheTTL:     86400, // 24 hours
	}
}
