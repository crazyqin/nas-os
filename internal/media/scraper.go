package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Image sizes for TMDB posters
const (
	PosterSizeW92   = "w92"
	PosterSizeW154  = "w154"
	PosterSizeW185  = "w185"
	PosterSizeW342  = "w342"
	PosterSizeW500  = "w500"
	PosterSizeW780  = "w780"
	PosterSizeOrig  = "original"
)

// TMDBImageBaseURL is the base URL for TMDB images
const TMDBImageBaseURL = "https://image.tmdb.org/t/p/"

// TMDBScraper scrapes metadata from The Movie Database
type TMDBScraper struct {
	apiKey       string
	baseURL      string
	imageBaseURL string
	httpClient   *http.Client
	cache        *EnhancedCache
	posterDir    string // Directory to store downloaded posters
	mu           sync.RWMutex
}

// TMDBConfig holds TMDB API configuration
type TMDBConfig struct {
	APIKey       string
	BaseURL      string // defaults to https://api.themoviedb.org/3
	ImageBaseURL string // defaults to https://image.tmdb.org/t/p/
	Timeout      time.Duration
	PosterDir    string // Directory to store downloaded posters
}

// NewTMDBScraper creates a new TMDB scraper
func NewTMDBScraper(config TMDBConfig, cache *EnhancedCache) *TMDBScraper {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.themoviedb.org/3"
	}
	if config.ImageBaseURL == "" {
		config.ImageBaseURL = TMDBImageBaseURL
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &TMDBScraper{
		apiKey:       config.APIKey,
		baseURL:      config.BaseURL,
		imageBaseURL: config.ImageBaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		cache:     cache,
		posterDir: config.PosterDir,
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
	defer func() { _ = resp.Body.Close() }()

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
		s.cache.SetMetadata(cacheKey, metadata, 24*time.Hour)
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
		s.cache.SetMetadata(cacheKey, metadata, 24*time.Hour)
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
	defer func() { _ = resp.Body.Close() }()

	var tv tmdbTVDetails
	if err := json.NewDecoder(resp.Body).Decode(&tv); err != nil {
		return nil, fmt.Errorf("parse tv details failed: %w", err)
	}

	return s.convertTVToMetadata(&tv), nil
}

// makeRequest makes an HTTP request with proper headers
func (s *TMDBScraper) makeRequest(ctx context.Context, reqURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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
		Status:           tv.Status,
	}

	for _, g := range tv.Genres {
		metadata.Genres = append(metadata.Genres, g.Name)
	}

	for _, n := range tv.Networks {
		metadata.Networks = append(metadata.Networks, n.Name)
	}

	for _, season := range tv.Seasons {
		seasonData := Season{
			SeasonNumber: season.SeasonNumber,
			Name:         season.Name,
			Overview:     season.Overview,
			PosterPath:   season.PosterPath,
			AirDate:      season.AirDate,
		}
		metadata.Seasons = append(metadata.Seasons, seasonData)
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
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Character   string `json:"character"`
			ProfilePath string `json:"profile_path"`
			Order       int    `json:"order"`
		} `json:"cast"`
		Crew []struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			Job        string `json:"job"`
			Department string `json:"department"`
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
	if size == "" {
		size = PosterSizeW500
	}
	return fmt.Sprintf("%s%s%s", s.imageBaseURL, size, posterPath)
}

// DownloadPoster downloads and saves a poster image locally
func (s *TMDBScraper) DownloadPoster(ctx context.Context, posterPath string, mediaType MediaType, tmdbID int, size string) (string, error) {
	if posterPath == "" {
		return "", fmt.Errorf("empty poster path")
	}

	if s.posterDir == "" {
		return "", fmt.Errorf("poster directory not configured")
	}

	// Create directory if not exists
	typeDir := filepath.Join(s.posterDir, string(mediaType))
	if err := os.MkdirAll(typeDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create poster directory: %w", err)
	}

	// Generate filename
	ext := filepath.Ext(posterPath)
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("%d_%s%s", tmdbID, size, ext)
	localPath := filepath.Join(typeDir, filename)

	// Check if already downloaded
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	// Download image
	posterURL := s.GetPosterURL(posterPath, size)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, posterURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download poster: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download poster: status %d", resp.StatusCode)
	}

	// Create temp file first
	tmpPath := localPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	_ = file.Close()

	// Rename to final path
	if err := os.Rename(tmpPath, localPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to save poster: %w", err)
	}

	return localPath, nil
}

// DownloadAllPosters downloads all poster sizes for a media item
func (s *TMDBScraper) DownloadAllPosters(ctx context.Context, posterPath string, mediaType MediaType, tmdbID int) (map[string]string, error) {
	sizes := []string{PosterSizeW92, PosterSizeW185, PosterSizeW342, PosterSizeW500, PosterSizeW780, PosterSizeOrig}
	results := make(map[string]string)

	for _, size := range sizes {
		path, err := s.DownloadPoster(ctx, posterPath, mediaType, tmdbID, size)
		if err != nil {
			continue // Skip failed downloads
		}
		results[size] = path
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("failed to download any poster sizes")
	}

	return results, nil
}

// ScrapeVideoFile scrapes metadata for a video file
func (s *TMDBScraper) ScrapeVideoFile(ctx context.Context, parser *FilenameParser, videoPath string) (*MediaMetadata, error) {
	// Parse filename with enhanced parser
	title, year, season, episode := parser.ParseFilename(filepath.Base(videoPath))

	// Detect media type with enhanced detection
	mediaType := parser.DetectMediaType(filepath.Base(videoPath))

	switch mediaType {
	case MediaTypeTVShow:
		tvMeta, err := s.SearchTVShow(ctx, title)
		if err != nil {
			return nil, err
		}
		// Add episode info
		if season > 0 && episode > 0 {
			tvMeta.MediaMetadata.Title = fmt.Sprintf("%s S%02dE%02d", tvMeta.MediaMetadata.Title, season, episode)
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

// FilenameParser provides enhanced filename parsing for media files
// Supports Chinese and international naming conventions
type FilenameParser struct {
	// Patterns for TV episode detection
	seasonEpisodePatterns []*regexp.Regexp
	// Patterns for movie year detection
	yearPatterns []*regexp.Regexp
	// Patterns to clean from filenames
	cleanupPatterns []*regexp.Regexp
	// Chinese title detection pattern
	chinesePattern *regexp.Regexp
}

// NewFilenameParser creates a new enhanced filename parser
func NewFilenameParser() *FilenameParser {
	return &FilenameParser{
		seasonEpisodePatterns: []*regexp.Regexp{
			// S01E01, s01e01, S1E1 patterns
			regexp.MustCompile(`(?i)[sS](\d{1,2})[eE](\d{1,3})`),
			// 1x01, 1x1 patterns
			regexp.MustCompile(`(?i)(\d{1,2})[xX](\d{1,3})`),
			// 第1季第1集, 第一季第一集 (Chinese patterns)
			regexp.MustCompile(`第\s*(\d{1,2})\s*季\s*第\s*(\d{1,3})\s*集`),
			regexp.MustCompile(`第\s*([一二三四五六七八九十百]+)\s*季\s*第\s*([一二三四五六七八九十百]+)\s*集`),
			// Season 1 Episode 1
			regexp.MustCompile(`(?i)season\s*(\d{1,2})\s*episode\s*(\d{1,3})`),
			// EP01, Ep.01, E01 patterns (standalone)
			regexp.MustCompile(`(?i)(?:ep|episode)[.\s]*(\d{1,3})(?:[^0-9]|$)`),
			regexp.MustCompile(`(?i)\b[eE](\d{1,3})\b`),
		},
		yearPatterns: []*regexp.Regexp{
			// (2024), [2024]
			regexp.MustCompile(`[\(\[](\d{4})[\)\]]`),
			// .2024. or _2024_ or space delimited
			regexp.MustCompile(`[._\s](\d{4})(?:[._\s]|$)`),
		},
		cleanupPatterns: []*regexp.Regexp{
			// Quality tags
			regexp.MustCompile(`(?i)\b(1080p|720p|2160p|4k|hd|hdr|dvd|bluray|brrip|webrip|web-dl|hdtv|cam|ts)\b`),
			// Codec tags
			regexp.MustCompile(`(?i)\b(x264|x265|hevc|h264|h265|avc|vp9|av1)\b`),
			// Audio tags
			regexp.MustCompile(`(?i)\b(aac|ac3|dts|dd5\.?1|5\.1|7\.1|truehd|atmos|flac)\b`),
			// Source tags
			regexp.MustCompile(`(?i)\b(netflix|amazon|hbo|disney|apple|hulu)\b`),
			// Release group tags
			regexp.MustCompile(`(?i)-[a-z]+$`),
			// Common junk
			regexp.MustCompile(`(?i)\b(www\.[a-z]+\.(com|net|org)|rarbg|yify|yts)\b`),
		},
		chinesePattern: regexp.MustCompile(`[\p{Han}]+`),
	}
}

// ParseFilename extracts potential title, year, season and episode from filename
func (p *FilenameParser) ParseFilename(filename string) (title string, year int, season int, episode int) {
	// Remove extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Try to extract TV episode info first
	for _, pattern := range p.seasonEpisodePatterns {
		if matches := pattern.FindStringSubmatch(name); len(matches) >= 3 {
			// Check if this is a Chinese number pattern
			if strings.Contains(pattern.String(), "一二三四五六七八九十百") {
				season = chineseToInt(matches[1])
				episode = chineseToInt(matches[2])
			} else {
				season = parseIntSafe(matches[1])
				episode = parseIntSafe(matches[2])
			}
			if season > 0 && episode > 0 {
				// Remove episode info from name
				name = pattern.ReplaceAllString(name, "")
				break
			}
		}
	}

	// Single episode pattern (E01 without season)
	if season == 0 {
		for _, pattern := range p.seasonEpisodePatterns {
			if matches := pattern.FindStringSubmatch(name); len(matches) >= 2 {
				episode = parseIntSafe(matches[1])
				if episode > 0 {
					name = pattern.ReplaceAllString(name, "")
					break
				}
			}
		}
	}

	// Try to extract year
	for _, pattern := range p.yearPatterns {
		if matches := pattern.FindStringSubmatch(name); len(matches) >= 2 {
			year = parseIntSafe(matches[1])
			if year >= 1900 && year <= time.Now().Year()+1 {
				// Remove year from name
				name = pattern.ReplaceAllString(name, "")
				break
			}
		}
	}

	// Clean up quality tags and other junk
	for _, pattern := range p.cleanupPatterns {
		name = pattern.ReplaceAllString(name, "")
	}

	// Clean up separators and whitespace
	name = cleanTitle(name)

	return name, year, season, episode
}

// DetectMediaType tries to detect if the content is a movie or TV show
func (p *FilenameParser) DetectMediaType(filename string) MediaType {
	name := strings.ToLower(filename)

	// Check for TV patterns
	for _, pattern := range p.seasonEpisodePatterns {
		if pattern.MatchString(name) {
			return MediaTypeTVShow
		}
	}

	// Chinese TV patterns
	chineseTVPatterns := []string{
		"第", "季", "集", "更新至", "全", "集完",
	}
	for _, kw := range chineseTVPatterns {
		if strings.Contains(name, kw) {
			return MediaTypeTVShow
		}
	}

	// Common TV naming patterns
	tvKeywords := []string{
		"season", "episode", "s01", "s02", "s03", "s04", "s05",
		"e01", "e02", "e03", "hdtv", "web-dl",
	}
	for _, kw := range tvKeywords {
		if strings.Contains(name, kw) {
			return MediaTypeTVShow
		}
	}

	// Check for year pattern (common in movies)
	yearPattern := regexp.MustCompile(`[\(\[](\d{4})[\)\]]`)
	if yearPattern.MatchString(name) {
		return MediaTypeMovie
	}

	// Default to movie if no TV patterns found
	return MediaTypeMovie
}

// parseIntSafe safely parses a string to int
func parseIntSafe(s string) int {
	var result int
	for _, c := range strings.TrimSpace(s) {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

// chineseToInt converts Chinese number characters to int
func chineseToInt(s string) int {
	chineseDigits := map[rune]int{
		'零': 0, '一': 1, '二': 2, '三': 3, '四': 4,
		'五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
		'十': 10, '百': 100,
	}

	result := 0
	temp := 0

	for _, c := range s {
		if val, ok := chineseDigits[c]; ok {
			if val == 10 || val == 100 {
				if temp == 0 {
					temp = 1
				}
				result += temp * val
				temp = 0
			} else {
				temp = val
			}
		}
	}

	return result + temp
}

// EnhancedCache provides caching with TTL support
type EnhancedCache struct {
	metadata map[string]*cacheEntry
	files    map[string]*VideoFile
	mu       sync.RWMutex
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

// NewEnhancedCache creates a new enhanced cache with TTL support
func NewEnhancedCache() *EnhancedCache {
	return &EnhancedCache{
		metadata: make(map[string]*cacheEntry),
		files:    make(map[string]*VideoFile),
	}
}

// GetMetadata retrieves cached metadata by key
func (c *EnhancedCache) GetMetadata(key string) (interface{}, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.metadata[key]
	if !ok {
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.value, true
}

// SetMetadata stores metadata in the cache with TTL
func (c *EnhancedCache) SetMetadata(key string, value interface{}, ttl time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metadata[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// RemoveMetadata removes metadata from the cache
func (c *EnhancedCache) RemoveMetadata(key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.metadata, key)
}

// GetFile retrieves a cached video file by path
func (c *EnhancedCache) GetFile(path string) (*VideoFile, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	file, ok := c.files[path]
	return file, ok
}

// SetFile stores a video file in the cache
func (c *EnhancedCache) SetFile(path string, file *VideoFile) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files[path] = file
}

// RemoveFile removes a video file from the cache
func (c *EnhancedCache) RemoveFile(path string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.files, path)
}

// GetAllFiles returns all cached video files
func (c *EnhancedCache) GetAllFiles() []*VideoFile {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	files := make([]*VideoFile, 0, len(c.files))
	for _, f := range c.files {
		files = append(files, f)
	}
	return files
}

// Clear clears all cached data
func (c *EnhancedCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files = make(map[string]*VideoFile)
	c.metadata = make(map[string]*cacheEntry)
}

// Cleanup removes expired entries
func (c *EnhancedCache) Cleanup() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0
	for key, entry := range c.metadata {
		if now.After(entry.expiresAt) {
			delete(c.metadata, key)
			removed++
		}
	}
	return removed
}

// Size returns the number of cached files and metadata entries
func (c *EnhancedCache) Size() (files int, metadata int) {
	if c == nil {
		return 0, 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.files), len(c.metadata)
}

// GenerateCacheKey generates a unique cache key for a media item
func GenerateCacheKey(mediaType MediaType, title string, year int) string {
	data := fmt.Sprintf("%s:%s:%d", mediaType, title, year)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// PersistToFile saves cache to a file
func (c *EnhancedCache) PersistToFile(path string) error {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	data := struct {
		Files    map[string]*VideoFile `json:"files"`
		Metadata map[string]jsonEntry  `json:"metadata"`
	}{
		Files:    c.files,
		Metadata: make(map[string]jsonEntry),
	}

	for k, v := range c.metadata {
		data.Metadata[k] = jsonEntry{
			Value:     v.value,
			ExpiresAt: v.expiresAt,
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	return encoder.Encode(data)
}

type jsonEntry struct {
	Value     interface{} `json:"value"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// LoadFromFile loads cache from a file
func (c *EnhancedCache) LoadFromFile(path string) error {
	if c == nil {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = file.Close() }()

	var data struct {
		Files    map[string]*VideoFile `json:"files"`
		Metadata map[string]jsonEntry  `json:"metadata"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.files = data.Files
	c.metadata = make(map[string]*cacheEntry)
	for k, v := range data.Metadata {
		c.metadata[k] = &cacheEntry{
			value:     v.Value,
			expiresAt: v.ExpiresAt,
		}
	}

	return nil
}