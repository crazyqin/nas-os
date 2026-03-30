// Package ai provides AI-powered photo album features
// Features: semantic search, scene recognition, location clustering, baby album tracking
package ai

import (
	"time"
)

// PhotoVector represents a photo with its AI-generated feature vector
type PhotoVector struct {
	PhotoID     string    `json:"photo_id"`
	ImageVector []float32 `json:"image_vector"` // CLIP image embedding (512-dim)
	TextVector  []float32 `json:"text_vector"`  // CLIP text embedding for captions
	SceneVector []float32 `json:"scene_vector"` // Scene classification embedding
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SemanticSearchResult represents a semantic search result
type SemanticSearchResult struct {
	PhotoID   string  `json:"photo_id"`
	Score     float64 `json:"score"`      // Similarity score 0-1
	MatchType string  `json:"match_type"` // "text", "image", "hybrid"
}

// SceneRecognitionResult represents scene recognition output
type SceneRecognitionResult struct {
	PhotoID    string       `json:"photo_id"`
	Primary    SceneInfo    `json:"primary"`
	Categories []SceneInfo  `json:"categories"`
	Objects    []ObjectInfo `json:"objects"`
	Colors     []ColorInfo  `json:"colors"`
	Mood       string       `json:"mood"`        // "happy", "calm", "energetic", "melancholic"
	TimeOfDay  string       `json:"time_of_day"` // "morning", "afternoon", "evening", "night"
	Season     string       `json:"season"`      // "spring", "summer", "autumn", "winter"
}

// SceneInfo represents a detected scene with confidence
type SceneInfo struct {
	Category    string   `json:"category"`               // "beach", "mountain", "city", "forest", etc.
	SubCategory string   `json:"sub_category,omitempty"` // "sunset_beach", "snow_mountain"
	Confidence  float64  `json:"confidence"`             // 0-1
	Labels      []string `json:"labels,omitempty"`       // Additional labels
}

// ObjectInfo represents a detected object
type ObjectInfo struct {
	Label      string       `json:"label"`
	Confidence float64      `json:"confidence"`
	Box        *BoundingBox `json:"box,omitempty"`
	Attributes []string     `json:"attributes,omitempty"` // "red", "large", etc.
}

// ColorInfo represents a dominant color
type ColorInfo struct {
	Hex     string  `json:"hex"`
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

// BoundingBox represents an object's location in image
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// LocationCluster represents a geographic cluster of photos
type LocationCluster struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"` // User-defined or auto-generated name
	CenterLat  float64    `json:"center_lat"`
	CenterLng  float64    `json:"center_lng"`
	Radius     float64    `json:"radius"` // meters
	PhotoCount int        `json:"photo_count"`
	PhotoIDs   []string   `json:"photo_ids"`
	DateRange  DateRange  `json:"date_range"`
	PlaceInfo  *PlaceInfo `json:"place_info,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// DateRange represents a date range
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// PlaceInfo represents location metadata from geocoding
type PlaceInfo struct {
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Province    string `json:"province"`
	City        string `json:"city"`
	District    string `json:"district"`
	POI         string `json:"poi"` // Point of interest name
	PlaceID     string `json:"place_id"`
}

// BabyAlbum represents a baby growth tracking album
type BabyAlbum struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	BirthDate    time.Time     `json:"birth_date"`
	Gender       string        `json:"gender"` // "male", "female", "unknown"
	CoverPhotoID string        `json:"cover_photo_id,omitempty"`
	Milestones   []Milestone   `json:"milestones"`
	GrowthPhotos []GrowthPhoto `json:"growth_photos"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// Milestone represents a baby milestone
type Milestone struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // "first_smile", "first_step", "first_word", etc.
	Date        time.Time `json:"date"`
	PhotoID     string    `json:"photo_id,omitempty"`
	Description string    `json:"description"`
	AgeMonths   int       `json:"age_months"`
}

// GrowthPhoto represents a photo in the baby growth timeline
type GrowthPhoto struct {
	PhotoID   string    `json:"photo_id"`
	Date      time.Time `json:"date"`
	AgeMonths int       `json:"age_months"`
	AgeDays   int       `json:"age_days"`
	Notes     string    `json:"notes,omitempty"`
	FaceID    string    `json:"face_id"` // Reference to detected face
}

// FaceGrowthTracker tracks face changes over time for baby album
type FaceGrowthTracker struct {
	BabyID       string        `json:"baby_id"`
	FaceHistory  []FaceRecord  `json:"face_history"`
	GrowthPoints []GrowthPoint `json:"growth_points"`
}

// FaceRecord represents a face detection at a point in time
type FaceRecord struct {
	FaceID      string      `json:"face_id"`
	PhotoID     string      `json:"photo_id"`
	Date        time.Time   `json:"date"`
	AgeMonths   int         `json:"age_months"`
	Embedding   []float32   `json:"embedding"`
	BoundingBox BoundingBox `json:"bounding_box"`
	Landmarks   []Landmark  `json:"landmarks,omitempty"`
}

// Landmark represents a facial landmark
type Landmark struct {
	Type string  `json:"type"` // "left_eye", "right_eye", "nose_tip", "mouth_center"
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

// GrowthPoint represents a growth measurement point
type GrowthPoint struct {
	Date      time.Time `json:"date"`
	AgeMonths int       `json:"age_months"`
	Metric    string    `json:"metric"` // "face_width", "eye_distance", "face_ratio"
	Value     float64   `json:"value"`
}

// ModelConfig holds AI model configuration
type ModelConfig struct {
	// CLIP model settings
	CLIPModelPath string `json:"clip_model_path"`
	CLIPModelType string `json:"clip_model_type"` // "vit-b-32", "vit-l-14", "rn50"
	CLIPDevice    string `json:"clip_device"`     // "cpu", "cuda"
	CLIPBatchSize int    `json:"clip_batch_size"`

	// Face recognition settings
	FaceModelPath  string  `json:"face_model_path"`
	FaceModelType  string  `json:"face_model_type"` // "arcface", "insightface"
	FaceMinSize    int     `json:"face_min_size"`
	FaceConfThresh float64 `json:"face_conf_thresh"`

	// Scene classification settings
	SceneModelPath string `json:"scene_model_path"`
	SceneModelType string `json:"scene_model_type"` // "resnet50", "efficientnet"

	// ONNX Runtime settings
	ONNXRuntimePath string `json:"onnx_runtime_path"`
	UseONNXRuntime  bool   `json:"use_onnx_runtime"`

	// Processing settings
	MaxWorkers int  `json:"max_workers"`
	EnableGPU  bool `json:"enable_gpu"`
	BatchSize  int  `json:"batch_size"`

	// Cache settings
	VectorCacheSize int  `json:"vector_cache_size"`
	EnableCache     bool `json:"enable_cache"`
}

// DefaultModelConfig returns default model configuration
func DefaultModelConfig() *ModelConfig {
	return &ModelConfig{
		CLIPModelType: "vit-b-32",
		CLIPDevice:    "cpu",
		CLIPBatchSize: 32,

		FaceModelType:  "arcface",
		FaceMinSize:    30,
		FaceConfThresh: 0.8,

		SceneModelType: "resnet50",

		UseONNXRuntime:  false,
		MaxWorkers:      4,
		EnableGPU:       false,
		BatchSize:       16,
		VectorCacheSize: 100000,
		EnableCache:     true,
	}
}

// SearchResult represents unified search result
type SearchResult struct {
	PhotoID   string                 `json:"photo_id"`
	Score     float64                `json:"score"`
	MatchInfo map[string]interface{} `json:"match_info"`
	PhotoInfo *PhotoInfo             `json:"photo_info,omitempty"`
}

// PhotoInfo represents basic photo information for search results
type PhotoInfo struct {
	Path     string     `json:"path"`
	Filename string     `json:"filename"`
	TakenAt  *time.Time `json:"taken_at,omitempty"`
	Width    int        `json:"width"`
	Height   int        `json:"height"`
	Scene    string     `json:"scene,omitempty"`
	Location *PlaceInfo `json:"location,omitempty"`
}

// SearchQuery represents a unified search query
type SearchQuery struct {
	// Text search
	TextQuery string `json:"text_query,omitempty"`

	// Image similarity (find similar photos)
	SimilarToPhotoID string `json:"similar_to_photo_id,omitempty"`

	// Filters
	PersonIDs []string   `json:"person_ids,omitempty"`
	Location  *PlaceInfo `json:"location,omitempty"`
	DateFrom  *time.Time `json:"date_from,omitempty"`
	DateTo    *time.Time `json:"date_to,omitempty"`
	Scenes    []string   `json:"scenes,omitempty"`
	Objects   []string   `json:"objects,omitempty"`
	Colors    []string   `json:"colors,omitempty"`

	// Baby album specific
	BabyID    string `json:"baby_id,omitempty"`
	AgeMonths *int   `json:"age_months,omitempty"`

	// Pagination
	Limit  int `json:"limit"`
	Offset int `json:"offset"`

	// Sort
	SortBy   string `json:"sort_by"` // "relevance", "date", "quality"
	SortDesc bool   `json:"sort_desc"`
}

// VectorIndex interface for vector similarity search
type VectorIndex interface {
	// Add adds a vector to the index
	Add(id string, vector []float32) error

	// AddBatch adds multiple vectors
	AddBatch(ids []string, vectors [][]float32) error

	// Search finds k nearest neighbors
	Search(query []float32, k int) ([]SearchResult, error)

	// SearchWithFilter finds nearest neighbors with filters
	SearchWithFilter(query []float32, k int, filter func(id string) bool) ([]SearchResult, error)

	// Delete removes a vector from index
	Delete(id string) error

	// Size returns index size
	Size() int
}
