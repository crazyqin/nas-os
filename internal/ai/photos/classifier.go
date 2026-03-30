// Package photos - Classifier interfaces and core abstractions
package photos

import (
	"context"
	"image"
	"time"
)

// Classifier is the main interface for photo classification
type Classifier interface {
	// Classify performs full classification on a photo
	Classify(ctx context.Context, photo *Photo) (*ClassificationResult, error)

	// ClassifyBatch classifies multiple photos in parallel
	ClassifyBatch(ctx context.Context, photos []*Photo) ([]*ClassificationResult, error)
}

// FaceDetector detects and recognizes faces in photos
type FaceDetector interface {
	// DetectFaces finds all faces in an image
	DetectFaces(ctx context.Context, img image.Image) ([]FaceDetection, error)

	// ExtractEmbedding extracts face embedding from detected face
	ExtractEmbedding(ctx context.Context, img image.Image, face FaceDetection) ([]float32, error)

	// CompareFaces computes similarity between two face embeddings
	CompareFaces(embedding1, embedding2 []float32) float64

	// ClusterFaces groups face embeddings into person clusters
	ClusterFaces(faces []FaceDetection) (*ClusterResult, error)
}

// SceneClassifier classifies the scene/subject of a photo
type SceneClassifier interface {
	// ClassifyScene analyzes the scene and returns categories
	ClassifyScene(ctx context.Context, img image.Image) (*SceneClassification, error)

	// DetectObjects finds specific objects in the image
	DetectObjects(ctx context.Context, img image.Image) ([]ObjectDetection, error)

	// AnalyzeQuality computes photo quality metrics
	AnalyzeQuality(ctx context.Context, img image.Image) (*PhotoQuality, error)
}

// Geocoder performs reverse geocoding (GPS -> location info)
type Geocoder interface {
	// ReverseGeocode converts GPS coordinates to location info
	ReverseGeocode(ctx context.Context, lat, lng float64) (*Location, error)

	// BatchReverseGeocode processes multiple coordinates
	BatchReverseGeocode(ctx context.Context, coords []GPSInfo) ([]*Location, error)

	// Search searches for locations by name
	Search(ctx context.Context, query string) ([]Location, error)
}

// ImageLoader loads images from storage
type ImageLoader interface {
	// Load loads an image from the given path
	Load(ctx context.Context, path string) (image.Image, error)

	// LoadThumbnail loads a thumbnail-sized version
	LoadThumbnail(ctx context.Context, path string, size int) (image.Image, error)

	// GetMetadata extracts EXIF and other metadata
	GetMetadata(ctx context.Context, path string) (*Photo, error)
}

// AlbumManager manages photo albums
type AlbumManager interface {
	// CreatePersonAlbums creates albums for each recognized person
	CreatePersonAlbums(ctx context.Context, persons []Person) error

	// CreateLocationAlbums creates albums based on locations
	CreateLocationAlbums(ctx context.Context, locations []Location) error

	// CreateDateAlbums creates albums based on date ranges
	CreateDateAlbums(ctx context.Context, photos []*Photo) error

	// CreateSmartAlbums creates albums based on rules
	CreateSmartAlbums(ctx context.Context, rules []AlbumRule) error

	// GetAlbum retrieves an album by ID
	GetAlbum(ctx context.Context, id string) (*Album, error)

	// ListAlbums lists all albums
	ListAlbums(ctx context.Context, albumType AlbumType) ([]Album, error)

	// AddToAlbum adds photos to an album
	AddToAlbum(ctx context.Context, albumID string, photoIDs []string) error

	// RemoveFromAlbum removes photos from an album
	RemoveFromAlbum(ctx context.Context, albumID string, photoIDs []string) error
}

// AlbumRule defines a smart album rule
type AlbumRule struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Criteria RuleCriteria `json:"criteria"`
	Enabled  bool         `json:"enabled"`
}

// RuleCriteria defines matching criteria for smart albums
type RuleCriteria struct {
	// Date range
	DateFrom *time.Time `json:"date_from,omitempty"`
	DateTo   *time.Time `json:"date_to,omitempty"`

	// Location
	Countries []string `json:"countries,omitempty"`
	Cities    []string `json:"cities,omitempty"`

	// Scene
	Categories []SceneCategory `json:"categories,omitempty"`

	// People
	PersonIDs []string `json:"person_ids,omitempty"`

	// Camera
	CameraMakes  []string `json:"camera_makes,omitempty"`
	CameraModels []string `json:"camera_models,omitempty"`

	// Quality
	MinQuality float64 `json:"min_quality,omitempty"`
	MaxQuality float64 `json:"max_quality,omitempty"`

	// Tags
	Tags    []string `json:"tags,omitempty"`
	TagMode string   `json:"tag_mode,omitempty"` // "any", "all"
}

// Indexer manages photo indexing and search
type Indexer interface {
	// Index adds or updates a photo in the index
	Index(ctx context.Context, photo *Photo) error

	// IndexBatch indexes multiple photos
	IndexBatch(ctx context.Context, photos []*Photo) error

	// Delete removes a photo from the index
	Delete(ctx context.Context, photoID string) error

	// Search finds photos matching query
	Search(ctx context.Context, query SearchQuery) (*SearchResult, error)

	// GetByPerson retrieves all photos with a specific person
	GetByPerson(ctx context.Context, personID string) ([]*Photo, error)

	// GetByLocation retrieves photos from a location
	GetByLocation(ctx context.Context, location Location) ([]*Photo, error)

	// GetByDateRange retrieves photos within a date range
	GetByDateRange(ctx context.Context, start, end time.Time) ([]*Photo, error)

	// GetByCategory retrieves photos by scene category
	GetByCategory(ctx context.Context, category SceneCategory) ([]*Photo, error)
}

// SearchQuery represents a search query
type SearchQuery struct {
	Text       string          `json:"text,omitempty"`
	PersonIDs  []string        `json:"person_ids,omitempty"`
	Location   *Location       `json:"location,omitempty"`
	DateFrom   *time.Time      `json:"date_from,omitempty"`
	DateTo     *time.Time      `json:"date_to,omitempty"`
	Categories []SceneCategory `json:"categories,omitempty"`
	Tags       []string        `json:"tags,omitempty"`
	CameraMake string          `json:"camera_make,omitempty"`
	MinQuality float64         `json:"min_quality,omitempty"`
	Limit      int             `json:"limit,omitempty"`
	Offset     int             `json:"offset,omitempty"`
	SortBy     string          `json:"sort_by,omitempty"` // "date", "quality", "relevance"
	SortDesc   bool            `json:"sort_desc"`
}

// SearchResult contains search results
type SearchResult struct {
	Photos    []*Photo `json:"photos"`
	Total     int      `json:"total"`
	Facets    Facets   `json:"facets"`
	QueryTime int64    `json:"query_time_ms"`
}

// Facets contains aggregation information for search results
type Facets struct {
	Years      map[int]int    `json:"years"`      // year -> count
	Months     map[string]int `json:"months"`     // "2024-01" -> count
	Categories map[string]int `json:"categories"` // category -> count
	People     map[string]int `json:"people"`     // person_id -> count
	Locations  map[string]int `json:"locations"`  // city -> count
	Cameras    map[string]int `json:"cameras"`    // make/model -> count
}

// Storage persists photo metadata and classification results
type Storage interface {
	// SavePhoto saves photo metadata
	SavePhoto(ctx context.Context, photo *Photo) error

	// SavePhotoBatch saves multiple photos
	SavePhotoBatch(ctx context.Context, photos []*Photo) error

	// GetPhoto retrieves a photo by ID
	GetPhoto(ctx context.Context, id string) (*Photo, error)

	// DeletePhoto deletes a photo
	DeletePhoto(ctx context.Context, id string) error

	// SaveFace saves a face detection
	SaveFace(ctx context.Context, face *FaceDetection) error

	// GetFacesByPhoto retrieves all faces for a photo
	GetFacesByPhoto(ctx context.Context, photoID string) ([]FaceDetection, error)

	// SavePerson saves a person
	SavePerson(ctx context.Context, person *Person) error

	// GetPerson retrieves a person by ID
	GetPerson(ctx context.Context, id string) (*Person, error)

	// ListPersons lists all persons
	ListPersons(ctx context.Context) ([]Person, error)

	// SaveAlbum saves an album
	SaveAlbum(ctx context.Context, album *Album) error

	// GetAlbum retrieves an album
	GetAlbum(ctx context.Context, id string) (*Album, error)
}

// Cache caches classification results for performance
type Cache interface {
	// Get retrieves cached classification
	Get(ctx context.Context, photoID string) (*ClassificationResult, bool)

	// Set stores classification in cache
	Set(ctx context.Context, photoID string, result *ClassificationResult) error

	// Delete removes cached result
	Delete(ctx context.Context, photoID string) error

	// Clear clears all cache
	Clear(ctx context.Context) error
}
