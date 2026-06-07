package homemedia

import (
	"time"
)

// MediaFile represents a media file in the system
type MediaFile struct {
	ID            string     `json:"id"`
	Filename      string     `json:"filename"`
	Path          string     `json:"path"`
	Size          int64      `json:"size"`
	MimeType      string     `json:"mime_type"`
	Duration      int        `json:"duration_seconds"`
	Resolution    string     `json:"resolution"`
	Width         int        `json:"width"`
	Height        int        `json:"height"`
	Bitrate       int        `json:"bitrate"`
	Codec         string     `json:"codec"`
	AudioCodec    string     `json:"audio_codec"`
	AudioChannels int        `json:"audio_channels"`
	FPS           float64    `json:"fps"`
	Quality       string     `json:"quality"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Hash          string     `json:"hash"`
	IsFavorite    bool       `json:"is_favorite"`
	Rating        int        `json:"rating"`
	WatchCount    int        `json:"watch_count"`
	LastWatched   *time.Time `json:"last_watched,omitempty"`
	Progress      float64    `json:"progress"`
}

// MediaMetadata represents detailed media metadata
type MediaMetadata struct {
	MediaID       string            `json:"media_id"`
	Title         string            `json:"title"`
	OriginalTitle string            `json:"original_title"`
	Description   string            `json:"description"`
	Tagline       string            `json:"tagline"`
	Type          string            `json:"type"`
	Genre         []string          `json:"genre"`
	Tags          []string          `json:"tags"`
	Year          int               `json:"year"`
	ReleaseDate   string            `json:"release_date"`
	Rating        float64           `json:"rating"`
	VoteCount     int               `json:"vote_count"`
	Popularity    float64           `json:"popularity"`
	Language      string            `json:"language"`
	Country       string            `json:"country"`
	Director      []string          `json:"director"`
	Cast          []CastMember      `json:"cast"`
	Writer        []string          `json:"writer"`
	Producer      []string          `json:"producer"`
	Studio        string            `json:"studio"`
	PosterURL     string            `json:"poster_url"`
	BackdropURL   string            `json:"backdrop_url"`
	TrailerURL    string            `json:"trailer_url"`
	ExternalIDs   map[string]string `json:"external_ids"`
	Subtitles     []SubtitleInfo    `json:"subtitles"`
	Chapters      []Chapter         `json:"chapters"`
}

// CastMember represents a cast member
type CastMember struct {
	Name       string `json:"name"`
	Character  string `json:"character"`
	ProfileURL string `json:"profile_url"`
	Order      int    `json:"order"`
}

// SubtitleInfo represents subtitle information
type SubtitleInfo struct {
	Language string `json:"language"`
	Label    string `json:"label"`
	Path     string `json:"path"`
	Format   string `json:"format"`
	Default  bool   `json:"default"`
	Forced   bool   `json:"forced"`
}

// Chapter represents a media chapter
type Chapter struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	StartTime int    `json:"start_time"`
	EndTime   int    `json:"end_time"`
}

// Collection represents a media collection
type Collection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	PosterURL   string    `json:"poster_url"`
	Type        string    `json:"type"`
	MediaCount  int       `json:"media_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SortOrder   int       `json:"sort_order"`
}

// Playlist represents a media playlist
type Playlist struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Items         []PlaylistItem `json:"items"`
	TotalDuration int            `json:"total_duration"`
	IsShuffled    bool           `json:"is_shuffled"`
	RepeatMode    string         `json:"repeat_mode"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// PlaylistItem represents an item in a playlist
type PlaylistItem struct {
	MediaID  string    `json:"media_id"`
	Position int       `json:"position"`
	AddedAt  time.Time `json:"added_at"`
}

// PlaybackSession represents an active playback session
type PlaybackSession struct {
	ID            string    `json:"id"`
	MediaID       string    `json:"media_id"`
	UserID        string    `json:"user_id"`
	DeviceID      string    `json:"device_id"`
	DeviceName    string    `json:"device_name"`
	StartTime     time.Time `json:"start_time"`
	CurrentTime   int       `json:"current_time"`
	Duration      int       `json:"duration"`
	Progress      float64   `json:"progress"`
	Status        string    `json:"status"`
	Quality       string    `json:"quality"`
	AudioTrack    string    `json:"audio_track"`
	SubtitleTrack string    `json:"subtitle_track"`
}

// MediaSearchRequest represents a search request
type MediaSearchRequest struct {
	Query     string   `json:"query"`
	Type      string   `json:"type"`
	Genre     []string `json:"genre"`
	Year      int      `json:"year"`
	Rating    float64  `json:"rating"`
	SortBy    string   `json:"sort_by"`
	SortOrder string   `json:"sort_order"`
	Page      int      `json:"page"`
	PageSize  int      `json:"pageSize"`
}

// MediaSearchResult represents search results
type MediaSearchResult struct {
	Media    []MediaFile `json:"media"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	HasMore  bool        `json:"has_more"`
}

// ScanRequest represents a library scan request
type ScanRequest struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
	AutoMatch bool   `json:"auto_match"`
	Overwrite bool   `json:"overwrite"`
}

// ScanStatus represents the status of a scan operation
type ScanStatus struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Total       int        `json:"total"`
	Processed   int        `json:"processed"`
	NewFiles    int        `json:"new_files"`
	Updated     int        `json:"updated"`
	Failed      int        `json:"failed"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Errors      []string   `json:"errors"`
}

// MatchRequest represents an auto-match request
type MatchRequest struct {
	MediaID    string `json:"media_id"`
	Title      string `json:"title"`
	Year       int    `json:"year"`
	Type       string `json:"type"`
	ExternalID string `json:"external_id,omitempty"`
}

// MatchResult represents match results
type MatchResult struct {
	MediaID    string           `json:"media_id"`
	Matches    []MatchCandidate `json:"matches"`
	Confidence float64          `json:"confidence"`
}

// MatchCandidate represents a potential match
type MatchCandidate struct {
	Title      string  `json:"title"`
	Year       int     `json:"year"`
	Type       string  `json:"type"`
	Source     string  `json:"source"`
	ExternalID string  `json:"external_id"`
	PosterURL  string  `json:"poster_url"`
	Rating     float64 `json:"rating"`
	Confidence float64 `json:"confidence"`
}

// TranscodeRequest represents a transcode request
type TranscodeRequest struct {
	MediaID    string `json:"media_id"`
	Profile    string `json:"profile"`
	Resolution string `json:"resolution"`
	Bitrate    int    `json:"bitrate"`
	AudioCodec string `json:"audio_codec"`
}

// TranscodeStatus represents transcode status
type TranscodeStatus struct {
	ID          string     `json:"id"`
	MediaID     string     `json:"media_id"`
	Status      string     `json:"status"`
	Progress    float64    `json:"progress"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	OutputPath  string     `json:"output_path"`
	OutputSize  int64      `json:"output_size"`
	Error       string     `json:"error,omitempty"`
}

// MediaStats represents media library statistics
type MediaStats struct {
	TotalMedia     int            `json:"total_media"`
	TotalMovies    int            `json:"total_movies"`
	TotalShows     int            `json:"total_shows"`
	TotalEpisodes  int            `json:"total_episodes"`
	TotalMusic     int            `json:"total_music"`
	TotalSize      int64          `json:"total_size"`
	TotalDuration  int            `json:"total_duration"`
	RecentlyAdded  []MediaFile    `json:"recently_added"`
	MostWatched    []MediaFile    `json:"most_watched"`
	TopRated       []MediaFile    `json:"top_rated"`
	GenreStats     []GenreCount   `json:"genre_stats"`
	YearStats      []YearCount    `json:"year_stats"`
	QualityStats   []QualityCount `json:"quality_stats"`
	StorageByMonth []MonthStorage `json:"storage_by_month"`
}

// GenreCount represents genre statistics
type GenreCount struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

// YearCount represents year statistics
type YearCount struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

// QualityCount represents quality statistics
type QualityCount struct {
	Quality string `json:"quality"`
	Count   int    `json:"count"`
}

// MonthStorage represents monthly storage statistics
type MonthStorage struct {
	Month string `json:"month"`
	Count int    `json:"count"`
	Size  int64  `json:"size"`
}

// StreamConfig represents streaming configuration
type StreamConfig struct {
	MediaID        string `json:"media_id"`
	Quality        string `json:"quality"`
	AudioTrack     int    `json:"audio_track"`
	SubtitleTrack  int    `json:"subtitle_track"`
	StartTime      int    `json:"start_time"`
	DirectPlay     bool   `json:"direct_play"`
	ForceTranscode bool   `json:"force_transcode"`
}

// StreamInfo represents streaming information
type StreamInfo struct {
	URL          string `json:"url"`
	Format       string `json:"format"`
	Resolution   string `json:"resolution"`
	Bitrate      int    `json:"bitrate"`
	Codec        string `json:"codec"`
	AudioCodec   string `json:"audio_codec"`
	IsTranscoded bool   `json:"is_transcoded"`
	Duration     int    `json:"duration"`
}
