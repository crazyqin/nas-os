package smartphoto

import (
	"time"
)

// Photo represents a photo in the smart album system
type Photo struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	MimeType    string    `json:"mime_type"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	Orientation int       `json:"orientation"`
	TakenAt     time.Time `json:"taken_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Hash        string    `json:"hash"`
	IsFavorite  bool      `json:"is_favorite"`
	IsHidden    bool      `json:"is_hidden"`
	Rating      int       `json:"rating"`
	Comments    string    `json:"comments"`
}

// PhotoMetadata represents EXIF and AI-extracted metadata
type PhotoMetadata struct {
	PhotoID     string            `json:"photo_id"`
	Camera      string            `json:"camera"`
	Lens        string            `json:"lens"`
	ISO         int               `json:"iso"`
	Aperture    float64           `json:"aperture"`
	ShutterSpeed string           `json:"shutter_speed"`
	FocalLength float64           `json:"focal_length"`
	Flash       bool              `json:"flash"`
	GPS         *GPSInfo          `json:"gps,omitempty"`
	Tags        []string          `json:"tags"`
	Faces       []FaceInfo        `json:"faces"`
	Scene       string            `json:"scene"`
	Objects     []string          `json:"objects"`
	Colors      []ColorInfo       `json:"colors"`
	Aesthetic   float64           `json:"aesthetic_score"`
	AIAnalysis  map[string]string `json:"ai_analysis"`
}

// GPSInfo represents GPS coordinates
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Address   string  `json:"address"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
}

// FaceInfo represents a detected face
type FaceInfo struct {
	ID        string  `json:"id"`
	PersonID  string  `json:"person_id,omitempty"`
	Name      string  `json:"name,omitempty"`
	X         int     `json:"x"`
	Y         int     `json:"y"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Confidence float64 `json:"confidence"`
	Age       int     `json:"age"`
	Gender    string  `json:"gender"`
	Emotion   string  `json:"emotion"`
}

// ColorInfo represents dominant colors
type ColorInfo struct {
	Hex       string  `json:"hex"`
	Percentage float64 `json:"percentage"`
	Name      string  `json:"name"`
}

// Person represents a recognized person
type Person struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Avatar      string    `json:"avatar"`
	FaceCount   int       `json:"face_count"`
	PhotoCount  int       `json:"photo_count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	IsFavorite  bool      `json:"is_favorite"`
	Tags        []string  `json:"tags"`
}

// Album represents a photo album
type Album struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CoverPhoto  string    `json:"cover_photo"`
	PhotoCount  int       `json:"photo_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	IsSmart     bool      `json:"is_smart"`
	SmartRules  *SmartRules `json:"smart_rules,omitempty"`
	Tags        []string  `json:"tags"`
	ShareLink   string    `json:"share_link,omitempty"`
	IsPublic    bool      `json:"is_public"`
}

// SmartRules defines rules for smart albums
type SmartRules struct {
	Filters    []Filter `json:"filters"`
	SortBy     string   `json:"sort_by"`
	SortOrder  string   `json:"sort_order"`
	Limit      int      `json:"limit"`
	AutoAdd    bool     `json:"auto_add"`
}

// Filter represents a search filter
type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// SearchRequest represents a search query
type SearchRequest struct {
	Query       string   `json:"query"`
	Tags        []string `json:"tags"`
	Persons     []string `json:"persons"`
	Albums      []string `json:"albums"`
	DateFrom    *time.Time `json:"date_from"`
	DateTo      *time.Time `json:"date_to"`
	Rating      int      `json:"rating"`
	IsFavorite  *bool    `json:"is_favorite"`
	SortBy      string   `json:"sort_by"`
	SortOrder   string   `json:"sort_order"`
	Page        int      `json:"page"`
	PageSize    int      `json:"pageSize"`
}

// SearchResult represents search results
type SearchResult struct {
	Photos     []Photo `json:"photos"`
	Total      int     `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
	HasMore    bool    `json:"has_more"`
}

// ImportRequest represents a photo import request
type ImportRequest struct {
	SourcePath  string   `json:"source_path"`
	Recursive   bool     `json:"recursive"`
	Tags        []string `json:"tags"`
	AlbumID     string   `json:"album_id,omitempty"`
	DuplicateCheck bool  `json:"duplicate_check"`
	AIAnalysis  bool     `json:"ai_analysis"`
}

// ImportStatus represents the status of an import operation
type ImportStatus struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Total       int       `json:"total"`
	Processed   int       `json:"processed"`
	Failed      int       `json:"failed"`
	Skipped     int       `json:"skipped"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Errors      []string  `json:"errors"`
}

// ShareRequest represents a sharing request
type ShareRequest struct {
	PhotoIDs  []string `json:"photo_ids"`
	AlbumID   string   `json:"album_id,omitempty"`
	ExpiresIn int      `json:"expires_in_hours"`
	Password  string   `json:"password,omitempty"`
	MaxViews  int      `json:"max_views"`
	AllowDownload bool `json:"allow_download"`
}

// ShareLink represents a shared link
type ShareLink struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	PhotoIDs    []string  `json:"photo_ids"`
	AlbumID     string    `json:"album_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Password    string    `json:"password,omitempty"`
	MaxViews    int       `json:"max_views"`
	CurrentViews int      `json:"current_views"`
	AllowDownload bool   `json:"allow_download"`
}

// PhotoStats represents photo statistics
type PhotoStats struct {
	TotalPhotos   int            `json:"total_photos"`
	TotalAlbums   int            `json:"total_albums"`
	TotalPersons  int            `json:"total_persons"`
	TotalSize     int64          `json:"total_size"`
	TodayCount    int            `json:"today_count"`
	WeekCount     int            `json:"week_count"`
	MonthCount    int            `json:"month_count"`
	TopTags       []TagCount     `json:"top_tags"`
	TopLocations  []LocationCount `json:"top_locations"`
	CameraStats   []CameraCount  `json:"camera_stats"`
	StorageByMonth []MonthStorage `json:"storage_by_month"`
}

// TagCount represents tag statistics
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// LocationCount represents location statistics
type LocationCount struct {
	Location string `json:"location"`
	Count    int    `json:"count"`
}

// CameraCount represents camera statistics
type CameraCount struct {
	Camera string `json:"camera"`
	Count  int    `json:"count"`
}

// MonthStorage represents monthly storage statistics
type MonthStorage struct {
	Month string `json:"month"`
	Count int    `json:"count"`
	Size  int64  `json:"size"`
}

// DuplicateGroup represents a group of duplicate photos
type DuplicateGroup struct {
	ID       string  `json:"id"`
	Hash     string  `json:"hash"`
	Photos   []Photo `json:"photos"`
	TotalSize int64  `json:"total_size"`
}

// CleanupRequest represents a cleanup request
type CleanupRequest struct {
	Duplicates   bool `json:"duplicates"`
	Screenshots  bool `json:"screenshots"`
	Blurry       bool `json:"blurry"`
	LowQuality   bool `json:"low_quality"`
	DryRun       bool `json:"dry_run"`
}

// CleanupResult represents cleanup results
type CleanupResult struct {
	TotalFound    int   `json:"total_found"`
	TotalRemoved  int   `json:"total_removed"`
	SpaceFreed    int64 `json:"space_freed"`
	Duplicates    int   `json:"duplicates"`
	Screenshots   int   `json:"screenshots"`
	Blurry        int   `json:"blurry"`
	LowQuality    int   `json:"low_quality"`
}
