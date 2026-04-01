// Package scraper provides media metadata scraping functionality.
// Supports TMDB, IMDB, and other metadata sources.
package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// MetadataSource represents metadata source type.
type MetadataSource string

const (
	SourceTMDB MetadataSource = "tmdb"
	SourceIMDB MetadataSource = "imdb"
	SourceDouban MetadataSource = "douban"
)

// MediaType represents media type.
type MediaType string

const (
	MediaTypeMovie MediaType = "movie"
	MediaTypeTV    MediaType = "tv"
)

// MediaMetadata represents scraped media metadata.
type MediaMetadata struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	OriginalTitle string           `json:"original_title"`
	Year         int               `json:"year"`
	Type         MediaType         `json:"type"`
	Source       MetadataSource    `json:"source"`
	IMDBID       string            `json:"imdb_id"`
	TMDBID       int               `json:"tmdb_id"`
	PosterURL    string            `json:"poster_url"`
	BackdropURL  string            `json:"backdrop_url"`
	Rating       float64           `json:"rating"`
	Votes        int               `json:"votes"`
	Runtime      int               `json:"runtime"` // minutes
	Genres       []string          `json:"genres"`
	Overview     string            `json:"overview"`
	Language     string            `json:"language"`
	Country      string            `json:"country"`
	Cast         []CastMember      `json:"cast"`
	Director     string            `json:"director"`
	Seasons      int               `json:"seasons"` // for TV
	Episodes     int               `json:"episodes"` // for TV
	Status       string            `json:"status"`
	ReleaseDate  string            `json:"release_date"`
	ExtraData    map[string]string `json:"extra_data"`
}

// CastMember represents cast member.
type CastMember struct {
	Name      string `json:"name"`
	Character string `json:"character"`
	Order     int    `json:"order"`
}

// ScraperConfig holds scraper configuration.
type ScraperConfig struct {
	TMDBAPIKey   string        `json:"tmdb_api_key"`
	TMDBBaseURL  string        `json:"tmdb_base_url"`
	Timeout      time.Duration `json:"timeout"`
	CacheEnabled bool          `json:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl"`
	Language     string        `json:"language"` // zh-CN, en-US, etc.
}

// TMDBScraper scrapes metadata from TMDB.
type TMDBScraper struct {
	config  *ScraperConfig
	cache   *MetadataCache
	client  *http.Client
	mu      sync.RWMutex
}

// NewTMDBScraper creates a new TMDB scraper.
func NewTMDBScraper(cfg *ScraperConfig) *TMDBScraper {
	if cfg == nil {
		cfg = &ScraperConfig{
			TMDBBaseURL:  "https://api.themoviedb.org/3",
			Timeout:      10 * time.Second,
			CacheEnabled: true,
			CacheTTL:     24 * time.Hour,
			Language:     "zh-CN",
		}
	}

	return &TMDBScraper{
		config: cfg,
		cache:  NewMetadataCache(cfg.CacheTTL),
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

// SearchMovie searches for movies by title.
func (s *TMDBScraper) SearchMovie(ctx context.Context, title string, year int) ([]MediaMetadata, error) {
	if s.config.TMDBAPIKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	// Check cache
	cacheKey := fmt.Sprintf("movie_search:%s:%d", title, year)
	if cached, ok := s.cache.Get(cacheKey); ok {
		return cached, nil
	}

	// Build URL
	params := url.Values{}
	params.Set("api_key", s.config.TMDBAPIKey)
	params.Set("query", title)
	params.Set("language", s.config.Language)
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}

	searchURL := fmt.Sprintf("%s/search/movie?%s", s.config.TMDBBaseURL, params.Encode())

	resp, err := s.client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("TMDB search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB returned status %d", resp.StatusCode)
	}

	var result tmdbMovieSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode TMDB response: %w", err)
	}

	metadata := make([]MediaMetadata, 0)
	for _, movie := range result.Results {
		m := s.parseMovieResult(movie)
		metadata = append(metadata, m)
	}

	// Cache result
	s.cache.Set(cacheKey, metadata)

	return metadata, nil
}

// SearchTV searches for TV shows by title.
func (s *TMDBScraper) SearchTV(ctx context.Context, title string, year int) ([]MediaMetadata, error) {
	if s.config.TMDBAPIKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	// Check cache
	cacheKey := fmt.Sprintf("tv_search:%s:%d", title, year)
	if cached, ok := s.cache.Get(cacheKey); ok {
		return cached, nil
	}

	params := url.Values{}
	params.Set("api_key", s.config.TMDBAPIKey)
	params.Set("query", title)
	params.Set("language", s.config.Language)
	if year > 0 {
		params.Set("first_air_date_year", fmt.Sprintf("%d", year))
	}

	searchURL := fmt.Sprintf("%s/search/tv?%s", s.config.TMDBBaseURL, params.Encode())

	resp, err := s.client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("TMDB search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB returned status %d", resp.StatusCode)
	}

	var result tmdbTVSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode TMDB response: %w", err)
	}

	metadata := make([]MediaMetadata, 0)
	for _, tv := range result.Results {
		m := s.parseTVResult(tv)
		metadata = append(metadata, m)
	}

	s.cache.Set(cacheKey, metadata)

	return metadata, nil
}

// GetMovieDetails gets detailed movie metadata by TMDB ID.
func (s *TMDBScraper) GetMovieDetails(ctx context.Context, tmdbID int) (*MediaMetadata, error) {
	cacheKey := fmt.Sprintf("movie_detail:%d", tmdbID)
	if cached, ok := s.cache.GetSingle(cacheKey); ok {
		return cached, nil
	}

	params := url.Values{}
	params.Set("api_key", s.config.TMDBAPIKey)
	params.Set("language", s.config.Language)
	params.Set("append_to_response", "credits")

	detailURL := fmt.Sprintf("%s/movie/%d?%s", s.config.TMDBBaseURL, tmdbID, params.Encode())

	resp, err := s.client.Get(detailURL)
	if err != nil {
		return nil, fmt.Errorf("TMDB movie detail failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB returned status %d", resp.StatusCode)
	}

	var movie tmdbMovieDetail
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, fmt.Errorf("decode TMDB response: %w", err)
	}

	metadata := s.parseMovieDetail(movie)
	s.cache.SetSingle(cacheKey, metadata)

	return metadata, nil
}

// GetTVDetails gets detailed TV metadata by TMDB ID.
func (s *TMDBScraper) GetTVDetails(ctx context.Context, tmdbID int) (*MediaMetadata, error) {
	cacheKey := fmt.Sprintf("tv_detail:%d", tmdbID)
	if cached, ok := s.cache.GetSingle(cacheKey); ok {
		return cached, nil
	}

	params := url.Values{}
	params.Set("api_key", s.config.TMDBAPIKey)
	params.Set("language", s.config.Language)
	params.Set("append_to_response", "credits")

	detailURL := fmt.Sprintf("%s/tv/%d?%s", s.config.TMDBBaseURL, tmdbID, params.Encode())

	resp, err := s.client.Get(detailURL)
	if err != nil {
		return nil, fmt.Errorf("TMDB TV detail failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB returned status %d", resp.StatusCode)
	}

	var tv tmdbTVDetail
	if err := json.NewDecoder(resp.Body).Decode(&tv); err != nil {
		return nil, fmt.Errorf("decode TMDB response: %w", err)
	}

	metadata := s.parseTVDetail(tv)
	s.cache.SetSingle(cacheKey, metadata)

	return metadata, nil
}

// parseMovieResult parses TMDB movie search result.
func (s *TMDBScraper) parseMovieResult(movie tmdbMovie) MediaMetadata {
	year := 0
	if len(movie.ReleaseDate) >= 4 {
		year = parseInt(movie.ReleaseDate[:4])
	}

	return MediaMetadata{
		ID:            fmt.Sprintf("tmdb_%d", movie.ID),
		Title:         movie.Title,
		OriginalTitle: movie.OriginalTitle,
		Year:          year,
		Type:          MediaTypeMovie,
		Source:        SourceTMDB,
		TMDBID:        movie.ID,
		PosterURL:     s.buildImageURL(movie.PosterPath),
		BackdropURL:   s.buildImageURL(movie.BackdropPath),
		Rating:        movie.VoteAverage,
		Votes:         movie.VoteCount,
		Overview:      movie.Overview,
		Language:      movie.OriginalLanguage,
		ReleaseDate:   movie.ReleaseDate,
	}
}

// parseTVResult parses TMDB TV search result.
func (s *TMDBScraper) parseTVResult(tv tmdbTV) MediaMetadata {
	year := 0
	if len(tv.FirstAirDate) >= 4 {
		year = parseInt(tv.FirstAirDate[:4])
	}

	return MediaMetadata{
		ID:            fmt.Sprintf("tmdb_%d", tv.ID),
		Title:         tv.Name,
		OriginalTitle: tv.OriginalName,
		Year:          year,
		Type:          MediaTypeTV,
		Source:        SourceTMDB,
		TMDBID:        tv.ID,
		PosterURL:     s.buildImageURL(tv.PosterPath),
		BackdropURL:   s.buildImageURL(tv.BackdropPath),
		Rating:        tv.VoteAverage,
		Votes:         tv.VoteCount,
		Overview:      tv.Overview,
		Language:      tv.OriginalLanguage,
	}
}

// parseMovieDetail parses TMDB movie detail response.
func (s *TMDBScraper) parseMovieDetail(movie tmdbMovieDetail) *MediaMetadata {
	year := 0
	if len(movie.ReleaseDate) >= 4 {
		year = parseInt(movie.ReleaseDate[:4])
	}

	genres := make([]string, 0)
	for _, g := range movie.Genres {
		genres = append(genres, g.Name)
	}

	cast := make([]CastMember, 0)
	director := ""
	if movie.Credits != nil {
		for _, c := range movie.Credits.Cast {
			cast = append(cast, CastMember{
				Name:      c.Name,
				Character: c.Character,
				Order:     c.Order,
			})
		}
		for _, c := range movie.Credits.Crew {
			if c.Job == "Director" {
				director = c.Name
				break
			}
		}
	}

	return &MediaMetadata{
		ID:            fmt.Sprintf("tmdb_%d", movie.ID),
		Title:         movie.Title,
		OriginalTitle: movie.OriginalTitle,
		Year:          year,
		Type:          MediaTypeMovie,
		Source:        SourceTMDB,
		IMDBID:        movie.IMDBID,
		TMDBID:        movie.ID,
		PosterURL:     s.buildImageURL(movie.PosterPath),
		BackdropURL:   s.buildImageURL(movie.BackdropPath),
		Rating:        movie.VoteAverage,
		Votes:         movie.VoteCount,
		Runtime:       movie.Runtime,
		Genres:        genres,
		Overview:      movie.Overview,
		Language:      movie.OriginalLanguage,
		Country:       movie.OriginCountry,
		Cast:          cast,
		Director:      director,
		ReleaseDate:   movie.ReleaseDate,
	}
}

// parseTVDetail parses TMDB TV detail response.
func (s *TMDBScraper) parseTVDetail(tv tmdbTVDetail) *MediaMetadata {
	year := 0
	if len(tv.FirstAirDate) >= 4 {
		year = parseInt(tv.FirstAirDate[:4])
	}

	genres := make([]string, 0)
	for _, g := range tv.Genres {
		genres = append(genres, g.Name)
	}

	cast := make([]CastMember, 0)
	if tv.Credits != nil {
		for _, c := range tv.Credits.Cast {
			cast = append(cast, CastMember{
				Name:      c.Name,
				Character: c.Character,
				Order:     c.Order,
			})
		}
	}

	return &MediaMetadata{
		ID:            fmt.Sprintf("tmdb_%d", tv.ID),
		Title:         tv.Name,
		OriginalTitle: tv.OriginalName,
		Year:          year,
		Type:          MediaTypeTV,
		Source:        SourceTMDB,
		TMDBID:        tv.ID,
		PosterURL:     s.buildImageURL(tv.PosterPath),
		BackdropURL:   s.buildImageURL(tv.BackdropPath),
		Rating:        tv.VoteAverage,
		Votes:         tv.VoteCount,
		Genres:        genres,
		Overview:      tv.Overview,
		Language:      tv.OriginalLanguage,
		Country:       tv.OriginCountry,
		Cast:          cast,
		Seasons:       tv.NumberOfSeasons,
		Episodes:      tv.NumberOfEpisodes,
		Status:        tv.Status,
		ReleaseDate:   tv.FirstAirDate,
	}
}

// buildImageURL builds full image URL from path.
func (s *TMDBScraper) buildImageURL(path string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", path)
}

// TMDB response types
type tmdbMovieSearchResult struct {
	Page    int         `json:"page"`
	Results []tmdbMovie `json:"results"`
}

type tmdbTVSearchResult struct {
	Page    int       `json:"page"`
	Results []tmdbTV `json:"results"`
}

type tmdbMovie struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	Overview         string  `json:"overview"`
	OriginalLanguage string  `json:"original_language"`
	ReleaseDate      string  `json:"release_date"`
}

type tmdbTV struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	OriginalName     string  `json:"original_name"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	Overview         string  `json:"overview"`
	OriginalLanguage string  `json:"original_language"`
	FirstAirDate     string  `json:"first_air_date"`
}

type tmdbMovieDetail struct {
	ID               int          `json:"id"`
	Title            string       `json:"title"`
	OriginalTitle    string       `json:"original_title"`
	IMDBID           string       `json:"imdb_id"`
	PosterPath       string       `json:"poster_path"`
	BackdropPath     string       `json:"backdrop_path"`
	VoteAverage      float64      `json:"vote_average"`
	VoteCount        int          `json:"vote_count"`
	Runtime          int          `json:"runtime"`
	Genres           []tmdbGenre  `json:"genres"`
	Overview         string       `json:"overview"`
	OriginalLanguage string       `json:"original_language"`
	OriginCountry    string       `json:"origin_country"`
	ReleaseDate      string       `json:"release_date"`
	Credits          *tmdbCredits `json:"credits"`
}

type tmdbTVDetail struct {
	ID               int          `json:"id"`
	Name             string       `json:"name"`
	OriginalName     string       `json:"original_name"`
	PosterPath       string       `json:"poster_path"`
	BackdropPath     string       `json:"backdrop_path"`
	VoteAverage      float64      `json:"vote_average"`
	VoteCount        int          `json:"vote_count"`
	NumberOfSeasons  int          `json:"number_of_seasons"`
	NumberOfEpisodes int          `json:"number_of_episodes"`
	Genres           []tmdbGenre  `json:"genres"`
	Overview         string       `json:"overview"`
	OriginalLanguage string       `json:"original_language"`
	OriginCountry    string       `json:"origin_country"`
	Status           string       `json:"status"`
	FirstAirDate     string       `json:"first_air_date"`
	Credits          *tmdbCredits `json:"credits"`
}

type tmdbGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type tmdbCredits struct {
	Cast []tmdbCast `json:"cast"`
	Crew []tmdbCrew `json:"crew"`
}

type tmdbCast struct {
	Name      string `json:"name"`
	Character string `json:"character"`
	Order     int    `json:"order"`
}

type tmdbCrew struct {
	Name string `json:"name"`
	Job  string `json:"job"`
}

func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}