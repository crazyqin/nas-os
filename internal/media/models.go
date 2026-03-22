// Package media 提供媒体库管理、元数据刮削和流媒体服务
package media

import (
	"time"
)

// MetadataCache 元数据缓存接口
type MetadataCache interface {
	Get(category, key string) (interface{}, bool)
	Set(category, key string, value interface{}, ttl time.Duration)
}

// SeasonInfo 季信息
type SeasonInfo struct {
	SeasonNumber int           `json:"seasonNumber"`
	Name         string        `json:"name"`
	Overview     string        `json:"overview"`
	AirDate      string        `json:"airDate"`
	EpisodeCount int           `json:"episodeCount"`
	PosterPath   string        `json:"posterPath"`
	Episodes     []EpisodeInfo `json:"episodes,omitempty"`
}

// EpisodeInfo 剧集信息
type EpisodeInfo struct {
	EpisodeNumber int     `json:"episodeNumber"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"airDate"`
	StillPath     string  `json:"stillPath"`
	Runtime       int     `json:"runtime"`
	Rating        float64 `json:"rating"`
	VoteCount     int     `json:"voteCount"`
}

// ScanResult 扫描结果
type ScanResult struct {
	LibraryID    string    `json:"libraryId"`
	TotalFiles   int       `json:"totalFiles"`
	NewFiles     int       `json:"newFiles"`
	UpdatedFiles int       `json:"updatedFiles"`
	RemovedFiles int       `json:"removedFiles"`
	Errors       []string  `json:"errors,omitempty"`
	Duration     int64     `json:"duration"` // 毫秒
	ScannedAt    time.Time `json:"scannedAt"`
}

// ScraperResult 刮削结果
type ScraperResult struct {
	Item     *Item       `json:"item"`
	Metadata interface{} `json:"metadata"`
	Source   string      `json:"source"`
	Error    string      `json:"error,omitempty"`
}

// PosterWallItem 海报墙项目
type PosterWallItem struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	PosterURL   string  `json:"posterUrl"`
	BackdropURL string  `json:"backdropUrl,omitempty"`
	Rating      float64 `json:"rating"`
	Year        string  `json:"year"`
	Type        Type    `json:"type"`
	IsFavorite  bool    `json:"isFavorite"`
}

// MediaFilter 媒体过滤条件
type MediaFilter struct {
	Query      string  `json:"query,omitempty"`
	Type       Type    `json:"type,omitempty"`
	Genre      string  `json:"genre,omitempty"`
	Year       string  `json:"year,omitempty"`
	MinRating  float64 `json:"minRating,omitempty"`
	IsFavorite *bool   `json:"isFavorite,omitempty"`
	SortBy     string  `json:"sortBy,omitempty"`    // name, rating, date, size
	SortOrder  string  `json:"sortOrder,omitempty"` // asc, desc
	Limit      int     `json:"limit,omitempty"`
	Offset     int     `json:"offset,omitempty"`
}
