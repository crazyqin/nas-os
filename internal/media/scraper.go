package media

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"time"
)

// TMDBScraper scrapes metadata from The Movie Database
type TMDBScraper struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	cache      *Cache
}

// TMDBConfig holds TMDB API configuration
type TMDBConfig struct {
	APIKey  string
	BaseURL string // defaults to https://api.themoviedb.org/3
	Timeout time.Duration
}

// NewTMDBScraper creates a new TMDB scraper
func NewTMDBScraper(config TMDBConfig, cache *Cache) *TMDBScraper {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.themoviedb.org/3"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &TMDBScraper{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		cache: cache,
	}
}

// SearchMovie searches for a movie by title
func (s *TMDBScraper) SearchMovie(ctx context.Context, title string, year int) (*MediaMetadata, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("movie:%s:%d", title, year)
	if s.cache != nil {
		if meta, ok := s.cache.GetMetadata(cacheKey); ok {
			if mm, ok := meta.(*MediaMetadata); ok {
				return mm, nil
			}
		}
	}

	// Build search URL
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("query", title)
	params.Set("language", "zh-CN")
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}

	searchURL := fmt.Sprintf("%s/search/movie?%s", s.baseURL, params.Encode())

	// Make request
	resp, err := s.makeRequest(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("search movie failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var searchResp tmdbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("parse search response failed: %w", err)
	}

	if len(searchResp.Results) == 0 {
		return nil, fmt.Errorf("no movie found for: %s", title)
	}

	// Get detailed info for the first result
	movie := searchResp.Results[0]
	metadata, err := s.GetMovieDetails(ctx, movie.ID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if s.cache != nil {
		s.cache.SetMetadata(cacheKey, metadata)
	}

	return metadata, nil
}

// GetMovieDetails gets detailed movie information
func (s *TMDBScraper) GetMovieDetails(ctx context.Context, tmdbID int) (*MediaMetadata, error) {
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("language", "zh-CN")
	params.Set("append_to_response", "credits")

	detailsURL := fmt.Sprintf("%s/movie/%d?%s", s.baseURL, tmdbID, params.Encode())

	resp, err := s.makeRequest(ctx, detailsURL)
	if err != nil {
		return nil, fmt.Errorf("get movie details failed: %w", err)
	}
	defer resp.Body.Close()

	var movie tmdbMovieDetails
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, fmt.Errorf("parse movie details failed: %w", err)
	}

	return s.convertMovieToMetadata(&movie), nil
}

// SearchTVShow searches for a TV show by title
func (s *TMDBScraper) SearchTVShow(ctx context.Context, title string) (*TVShowMetadata, error) {
	cacheKey := fmt.Sprintf("tv:%s", title)
	if s.cache != nil {
		if meta, ok := s.cache.GetMetadata(cacheKey); ok {
			if tv, ok := meta.(*TVShowMetadata); ok {
				return tv, nil
			}
		}
	}

	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("query", title)
	params.Set("language", "zh-CN")

	searchURL := fmt.Sprintf("%s/search/tv?%s", s.baseURL, params.Encode())

	resp, err := s.makeRequest(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("search tv show failed: %w", err)
	}
	defer resp.Body.Close()

	var searchResp tmdbTVSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("parse tv search response failed: %w", err)
	}

	if len(searchResp.Results) == 0 {
		return nil, fmt.Errorf("no tv show found for: %s", title)
	}

	tv := searchResp.Results[0]
	metadata, err := s.GetTVShowDetails(ctx, tv.ID)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		s.cache.SetMetadata(cacheKey, metadata)
	}

	return metadata, nil
}

// GetTVShowDetails gets detailed TV show information
func (s *TMDBScraper) GetTVShowDetails(ctx context.Context, tmdbID int) (*TVShowMetadata, error) {
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("language", "zh-CN")

	detailsURL := fmt.Sprintf("%s/tv/%d?%s", s.baseURL, tmdbID, params.Encode())

	resp, err := s.makeRequest(ctx, detailsURL)
	if err != nil {
		return nil, fmt.Errorf("get tv details failed: %w", err)
	}
	defer resp.Body.Close()

	var tv tmdbTVDetails
	if err := json.NewDecoder(resp.Body).Decode(&tv); err != nil {
		return nil, fmt.Errorf("parse tv details failed: %w", err)
	}

	return s.convertTVToMetadata(&tv), nil
}

// makeRequest makes an HTTP request with proper headers
func (s *TMDBScraper) makeRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "nas-os/1.0")

	return s.httpClient.Do(req)
}

// convertMovieToMetadata converts TMDB movie response to MediaMetadata
func (s *TMDBScraper) convertMovieToMetadata(movie *tmdbMovieDetails) *MediaMetadata {
	metadata := &MediaMetadata{
		TMDBID:        movie.ID,
		Type:          MediaTypeMovie,
		Title:         movie.Title,
		OriginalTitle: movie.OriginalTitle,
		Overview:      movie.Overview,
		Tagline:       movie.Tagline,
		PosterPath:    movie.PosterPath,
		BackdropPath:  movie.BackdropPath,
		Rating:        movie.VoteAverage,
		VoteCount:     movie.VoteCount,
		ReleaseDate:   movie.ReleaseDate,
		Runtime:       movie.Runtime,
		ScrapedAt:     time.Now(),
	}

	// Extract genres
	for _, g := range movie.Genres {
		metadata.Genres = append(metadata.Genres, g.Name)
	}

	// Extract cast
	if movie.Credits != nil {
		for i, c := range movie.Credits.Cast {
			if i >= 10 {
				break
			}
			metadata.Cast = append(metadata.Cast, Cast{
				Name:        c.Name,
				Character:   c.Character,
				ProfilePath: c.ProfilePath,
				Order:       c.Order,
			})
		}
	}

	// Extract directors
	if movie.Credits != nil {
		for _, c := range movie.Credits.Crew {
			if c.Job == "Director" {
				metadata.Directors = append(metadata.Directors, c.Name)
			}
		}
	}

	return metadata
}

// convertTVToMetadata converts TMDB TV response to TVShowMetadata
func (s *TMDBScraper) convertTVToMetadata(tv *tmdbTVDetails) *TVShowMetadata {
	metadata := &TVShowMetadata{
		MediaMetadata: MediaMetadata{
			TMDBID:        tv.ID,
			Type:          MediaTypeTVShow,
			Title:         tv.Name,
			OriginalTitle: tv.OriginalName,
			Overview:      tv.Overview,
			PosterPath:    tv.PosterPath,
			BackdropPath:  tv.BackdropPath,
			Rating:        tv.VoteAverage,
			VoteCount:     tv.VoteCount,
			ReleaseDate:   tv.FirstAirDate,
			ScrapedAt:     time.Now(),
		},
		NumberOfSeasons:  tv.NumberOfSeasons,
		NumberOfEpisodes: tv.NumberOfEpisodes,
		Status:          tv.Status,
	}

	for _, g := range tv.Genres {
		metadata.Genres = append(metadata.Genres, g.Name)
	}

	for _, n := range tv.Networks {
		metadata.Networks = append(metadata.Networks, n.Name)
	}

	for _, s := range tv.Seasons {
		season := Season{
			SeasonNumber: s.SeasonNumber,
			Name:         s.Name,
			Overview:     s.Overview,
			PosterPath:   s.PosterPath,
			AirDate:      s.AirDate,
		}
		metadata.Seasons = append(metadata.Seasons, season)
	}

	return metadata
}

// TMDB response types
type tmdbSearchResponse struct {
	Page    int `json:"page"`
	Results []struct {
		ID            int     `json:"id"`
		Title         string  `json:"title"`
		OriginalTitle string  `json:"original_title"`
		Overview      string  `json:"overview"`
		PosterPath    string  `json:"poster_path"`
		BackdropPath  string  `json:"backdrop_path"`
		ReleaseDate   string  `json:"release_date"`
		VoteAverage   float64 `json:"vote_average"`
		VoteCount     int     `json:"vote_count"`
	} `json:"results"`
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
}

type tmdbMovieDetails struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	Tagline       string  `json:"tagline"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	ReleaseDate   string  `json:"release_date"`
	Runtime       int     `json:"runtime"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	Genres        []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	Credits *struct {
		Cast []struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			Character    string `json:"character"`
			ProfilePath  string `json:"profile_path"`
			Order        int    `json:"order"`
		} `json:"cast"`
		Crew []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Job         string `json:"job"`
			Department  string `json:"department"`
		} `json:"crew"`
	} `json:"credits"`
}

type tmdbTVSearchResponse struct {
	Results []struct {
		ID           int     `json:"id"`
		Name         string  `json:"name"`
		OriginalName string  `json:"original_name"`
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"poster_path"`
		BackdropPath string  `json:"backdrop_path"`
		FirstAirDate string  `json:"first_air_date"`
		VoteAverage  float64 `json:"vote_average"`
		VoteCount    int     `json:"vote_count"`
	} `json:"results"`
}

type tmdbTVDetails struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	OriginalName     string  `json:"original_name"`
	Overview         string  `json:"overview"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	FirstAirDate     string  `json:"first_air_date"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	NumberOfSeasons  int     `json:"number_of_seasons"`
	NumberOfEpisodes int     `json:"number_of_episodes"`
	Status           string  `json:"status"`
	Genres           []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	Networks []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"networks"`
	Seasons []struct {
		ID           int    `json:"id"`
		SeasonNumber int    `json:"season_number"`
		Name         string `json:"name"`
		Overview     string `json:"overview"`
		PosterPath   string `json:"poster_path"`
		AirDate      string `json:"air_date"`
		EpisodeCount int    `json:"episode_count"`
	} `json:"seasons"`
}

// GetPosterURL returns the full URL for a poster image
func (s *TMDBScraper) GetPosterURL(posterPath string, size string) string {
	if posterPath == "" {
		return ""
	}
	baseURL := "https://image.tmdb.org/t/p/"
	if size == "" {
		size = "w500"
	}
	return fmt.Sprintf("%s%s%s", baseURL, size, posterPath)
}

// ScrapeVideoFile scrapes metadata for a video file
func (s *TMDBScraper) ScrapeVideoFile(ctx context.Context, scanner *Scanner, videoPath string) (*MediaMetadata, error) {
	// Parse filename
	title, year, _, _ := scanner.ParseFilename(filepath.Base(videoPath))

	// Detect media type
	mediaType := scanner.DetectMediaType(filepath.Base(videoPath))

	switch mediaType {
	case MediaTypeTVShow:
		tvMeta, err := s.SearchTVShow(ctx, title)
		if err != nil {
			return nil, err
		}
		return &tvMeta.MediaMetadata, nil
	case MediaTypeMovie:
		return s.SearchMovie(ctx, title, year)
	default:
		// Try movie first
		meta, err := s.SearchMovie(ctx, title, year)
		if err == nil {
			return meta, nil
		}
		// Try TV show
		tvMeta, err := s.SearchTVShow(ctx, title)
		if err != nil {
			return nil, fmt.Errorf("could not identify media: %s", title)
		}
		return &tvMeta.MediaMetadata, nil
	}
}