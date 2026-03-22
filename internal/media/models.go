// Package media provides media library management functionality
// including video scanning, metadata scraping, and poster generation
package media

import "time"

// MediaType represents the type of media content
type MediaType string

const (
	// MediaTypeMovie represents a movie.
	MediaTypeMovie MediaType = "movie"
	// MediaTypeTVShow represents a TV show series.
	MediaTypeTVShow MediaType = "tv"
	// MediaTypeEpisode represents a TV episode.
	MediaTypeEpisode MediaType = "episode"
	// MediaTypeUnknown represents an unknown media type.
	MediaTypeUnknown MediaType = "unknown"
)

// VideoFile represents a video file on disk
type VideoFile struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Filename   string    `json:"filename"`
	Size       int64     `json:"size"`
	Duration   int       `json:"duration,omitempty"` // seconds
	Width      int       `json:"width,omitempty"`
	Height     int       `json:"height,omitempty"`
	Codec      string    `json:"codec,omitempty"`
	Bitrate    int       `json:"bitrate,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
}

// MediaMetadata represents scraped metadata from TMDB
type MediaMetadata struct {
	ID            string    `json:"id"`
	TMDBID        int       `json:"tmdb_id"`
	IMDBID        string    `json:"imdb_id,omitempty"`
	Type          MediaType `json:"type"`
	Title         string    `json:"title"`
	OriginalTitle string    `json:"original_title,omitempty"`
	Overview      string    `json:"overview,omitempty"`
	Tagline       string    `json:"tagline,omitempty"`
	PosterPath    string    `json:"poster_path,omitempty"`
	BackdropPath  string    `json:"backdrop_path,omitempty"`
	Rating        float64   `json:"rating,omitempty"`
	VoteCount     int       `json:"vote_count,omitempty"`
	ReleaseDate   string    `json:"release_date,omitempty"`
	Runtime       int       `json:"runtime,omitempty"` // minutes
	Genres        []string  `json:"genres,omitempty"`
	Cast          []Cast    `json:"cast,omitempty"`
	Directors     []string  `json:"directors,omitempty"`
	Studios       []string  `json:"studios,omitempty"`
	Countries     []string  `json:"countries,omitempty"`
	Languages     []string  `json:"languages,omitempty"`
	ScrapedAt     time.Time `json:"scraped_at"`
}

// Cast represents an actor/actress
type Cast struct {
	Name        string `json:"name"`
	Character   string `json:"character,omitempty"`
	ProfilePath string `json:"profile_path,omitempty"`
	Order       int    `json:"order,omitempty"`
}

// TVShowMetadata represents a TV show with seasons
type TVShowMetadata struct {
	MediaMetadata
	Seasons          []Season `json:"seasons,omitempty"`
	NumberOfSeasons  int      `json:"number_of_seasons,omitempty"`
	NumberOfEpisodes int      `json:"number_of_episodes,omitempty"`
	Status           string   `json:"status,omitempty"`
	Networks         []string `json:"networks,omitempty"`
}

// Season represents a TV show season
type Season struct {
	SeasonNumber int       `json:"season_number"`
	Name         string    `json:"name,omitempty"`
	Overview     string    `json:"overview,omitempty"`
	PosterPath   string    `json:"poster_path,omitempty"`
	AirDate      string    `json:"air_date,omitempty"`
	Episodes     []Episode `json:"episodes,omitempty"`
}

// Episode represents a TV episode
type Episode struct {
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	Name          string `json:"name,omitempty"`
	Overview      string `json:"overview,omitempty"`
	StillPath     string `json:"still_path,omitempty"`
	AirDate       string `json:"air_date,omitempty"`
	Runtime       int    `json:"runtime,omitempty"`
}

// MediaLibrary represents a media library collection
type MediaLibrary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Type        MediaType `json:"type"`
	MediaCount  int       `json:"media_count"`
	TotalSize   int64     `json:"total_size"`
	LastScanned time.Time `json:"last_scanned"`
}

// ScanResult represents the result of a media scan
type ScanResult struct {
	LibraryID    string        `json:"library_id"`
	TotalFiles   int           `json:"total_files"`
	NewFiles     int           `json:"new_files"`
	UpdatedFiles int           `json:"updated_files"`
	RemovedFiles int           `json:"removed_files"`
	Errors       []ScanError   `json:"errors,omitempty"`
	Duration     time.Duration `json:"duration"`
}

// ScanError represents an error during scanning
type ScanError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// SupportedExtensions contains supported video file extensions.
var SupportedExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".m4v":  true,
	".ts":   true,
	".m2ts": true,
}
