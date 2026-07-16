package mediacenter

import (
	"time"
)

// MediaType 媒体类型.
type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"
	MediaTypeTVShow MediaType = "tvshow"
	MediaTypeMusic  MediaType = "music"
	MediaTypePhoto  MediaType = "photo"
	MediaTypeOther  MediaType = "other"
)

// MediaStatus 媒体状态.
type MediaStatus string

const (
	MediaStatusAvailable MediaStatus = "available"
	MediaStatusPlaying   MediaStatus = "playing"
	MediaStatusPaused    MediaStatus = "paused"
	MediaStatusError     MediaStatus = "error"
)

// MediaItem 媒体项.
type MediaItem struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	Type       MediaType     `json:"type"`
	FilePath   string        `json:"filePath"`
	FileSize   int64         `json:"fileSize"`
	Duration   int           `json:"duration"`
	Resolution string        `json:"resolution"`
	Codec      string        `json:"codec"`
	Bitrate    int           `json:"bitrate"`
	Status     MediaStatus   `json:"status"`
	Metadata   MediaMetadata `json:"metadata"`
	CreatedAt  time.Time     `json:"createdAt"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}

// MediaMetadata 媒体元数据.
type MediaMetadata struct {
	Title       string   `json:"title"`
	Artist      string   `json:"artist,omitempty"`
	Album       string   `json:"album,omitempty"`
	Genre       string   `json:"genre,omitempty"`
	Year        int      `json:"year,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Session 播放会话.
type Session struct {
	ID        string    `json:"id"`
	MediaID   string    `json:"mediaId"`
	UserID    string    `json:"userId"`
	Status    string    `json:"status"`
	Position  int       `json:"position"`
	StartedAt time.Time `json:"startedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
