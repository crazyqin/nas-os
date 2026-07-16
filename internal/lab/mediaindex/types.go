// Package mediaindex 提供媒体索引功能
package mediaindex

import "time"

// MediaType 媒体类型.
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
	MediaTypeAudio MediaType = "audio"
)

// MediaFile 媒体文件.
type MediaFile struct {
	ID          string            `json:"id"`
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	Type        MediaType         `json:"type"`
	MIMEType    string            `json:"mime_type"`
	Size        int64             `json:"size"`
	Checksum    string            `json:"checksum,omitempty"`
	Width       int               `json:"width,omitempty"`
	Height      int               `json:"height,omitempty"`
	Duration    float64           `json:"duration,omitempty"` // seconds
	TakenAt     *time.Time        `json:"taken_at,omitempty"`
	ModifiedAt  time.Time         `json:"modified_at"`
	IndexedAt   time.Time         `json:"indexed_at"`
	EXIF        map[string]string `json:"exif,omitempty"`
	GPS         *GPSInfo          `json:"gps,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Collections []string          `json:"collections,omitempty"`
	IsDuplicate bool              `json:"is_duplicate"`
	DuplicateOf string            `json:"duplicate_of,omitempty"`
}

// GPSInfo GPS坐标信息.
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
	Location  string  `json:"location,omitempty"` // 逆地理编码结果
}

// Thumbnail 缩略图.
type Thumbnail struct {
	ID       string    `json:"id"`
	FileID   string    `json:"file_id"`
	Path     string    `json:"path"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	Size     int64     `json:"size"`
	CreateAt time.Time `json:"create_at"`
}

// MediaTag 媒体标签.
type MediaTag struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Color    string    `json:"color,omitempty"`
	FileIDs  []string  `json:"file_ids,omitempty"`
	CreateAt time.Time `json:"create_at"`
}

// MediaCollection 媒体合集.
type MediaCollection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CoverID     string    `json:"cover_id,omitempty"`
	FileIDs     []string  `json:"file_ids"`
	CreateAt    time.Time `json:"create_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MediaIndex 索引统计.
type MediaIndex struct {
	TotalFiles     int            `json:"total_files"`
	TotalSize      int64          `json:"total_size"`
	ByType         map[string]int `json:"by_type"`
	LastIndexed    time.Time      `json:"last_indexed"`
	IndexedDirs    []string       `json:"indexed_dirs"`
	DuplicateCount int            `json:"duplicate_count"`
}

// SearchQuery 搜索查询.
type SearchQuery struct {
	Keyword     string     `json:"keyword,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Type        MediaType  `json:"type,omitempty"`
	DateFrom    *time.Time `json:"date_from,omitempty"`
	DateTo      *time.Time `json:"date_to,omitempty"`
	MinSize     int64      `json:"min_size,omitempty"`
	MaxSize     int64      `json:"max_size,omitempty"`
	Location    string     `json:"location,omitempty"`
	Collections []string   `json:"collections,omitempty"`
	SortBy      string     `json:"sort_by,omitempty"`    // name, date, size
	SortOrder   string     `json:"sort_order,omitempty"` // asc, desc
	Page        int        `json:"page"`
	PageSize    int        `json:"page_size"`
}

// SearchResult 搜索结果.
type SearchResult struct {
	Files    []*MediaFile `json:"files"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

// TimelineGroup 时间线分组.
type TimelineGroup struct {
	Date  string       `json:"date"` // YYYY-MM-DD
	Count int          `json:"count"`
	Files []*MediaFile `json:"files"`
}
