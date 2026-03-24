// Package media provides unified metadata scraping from multiple sources
// Supports TMDB, Douban with intelligent source selection and caching
package media

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ScraperSource represents a metadata source
type ScraperSource string

const (
	// SourceTMDB represents The Movie Database source
	SourceTMDB    ScraperSource = "tmdb"
	// SourceDouban represents Douban source
	SourceDouban  ScraperSource = "douban"
	SourceIMDB    ScraperSource = "imdb"
	SourceAuto    ScraperSource = "auto" // Auto-select best source
)

// UnifiedScraperConfig configuration for unified scraper
type UnifiedScraperConfig struct {
	// Primary source (default: auto)
	PrimarySource ScraperSource `json:"primarySource"`
	// Fallback sources in order
	FallbackSources []ScraperSource `json:"fallbackSources"`
	// TMDB API key
	TMDBAPIKey string `json:"tmdbApiKey"`
	// Douban API key
	DoubanAPIKey string `json:"doubanApiKey"`
	// Language preference (zh-CN, en-US, etc.)
	Language string `json:"language"`
	// Enable caching
	EnableCache bool `json:"enableCache"`
	// Cache TTL
	CacheTTL time.Duration `json:"cacheTtl"`
	// Request timeout
	Timeout time.Duration `json:"timeout"`
	// Enable Chinese title preference
	PreferChineseTitle bool `json:"preferChineseTitle"`
}

// UnifiedScraper provides unified metadata scraping from multiple sources
// Inspired by fnOS media library and Synology DS photo
type UnifiedScraper struct {
	config    UnifiedScraperConfig
	tmdb      *TMDBScraper
	douban    *DoubanProvider
	cache     *Cache
	rateLimit map[ScraperSource]*rateLimiter
	mu        sync.RWMutex
}

// rateLimiter simple rate limiter
type rateLimiter struct {
	tokens    int
	maxTokens int
	interval  time.Duration
	lastTime  time.Time
}

// NewUnifiedScraper creates a new unified scraper
func NewUnifiedScraper(config UnifiedScraperConfig, cache *Cache) *UnifiedScraper {
	if config.Language == "" {
		config.Language = "zh-CN"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 24 * time.Hour
	}
	if len(config.FallbackSources) == 0 {
		config.FallbackSources = []ScraperSource{SourceDouban, SourceTMDB}
	}

	scraper := &UnifiedScraper{
		config:    config,
		cache:     cache,
		rateLimit: make(map[ScraperSource]*rateLimiter),
	}

	// Initialize scrapers
	if config.TMDBAPIKey != "" {
		scraper.tmdb = NewTMDBScraper(TMDBConfig{
			APIKey:  config.TMDBAPIKey,
			Timeout: config.Timeout,
		}, cache)
	}

	if config.DoubanAPIKey != "" {
		scraper.douban = NewDoubanProvider(config.DoubanAPIKey)
	}

	// Initialize rate limiters
	scraper.rateLimit[SourceTMDB] = &rateLimiter{maxTokens: 40, tokens: 40, interval: 10 * time.Second}
	scraper.rateLimit[SourceDouban] = &rateLimiter{maxTokens: 30, tokens: 30, interval: 60 * time.Second}

	return scraper
}

// ScrapeMovie scrapes movie metadata from configured sources
func (s *UnifiedScraper) ScrapeMovie(ctx context.Context, title string, year int) (*MediaMetadata, error) {
	cacheKey := fmt.Sprintf("movie:%s:%d", title, year)

	// Check cache first
	if s.config.EnableCache && s.cache != nil {
		if meta, ok := s.cache.GetMetadata(cacheKey); ok {
			if mm, ok := meta.(*MediaMetadata); ok {
				return mm, nil
			}
		}
	}

	// Determine source order
	sources := s.getSourceOrder()

	var lastErr error
	for _, source := range sources {
		if !s.checkRateLimit(source) {
			continue
		}

		metadata, err := s.scrapeMovieFromSource(ctx, source, title, year)
		if err != nil {
			lastErr = err
			continue
		}

		// Enrich with additional data if needed
		metadata = s.enrichMetadata(ctx, metadata, source)

		// Cache result
		if s.config.EnableCache && s.cache != nil {
			s.cache.SetMetadata(cacheKey, metadata)
		}

		return metadata, nil
	}

	return nil, fmt.Errorf("failed to scrape movie from all sources: %v", lastErr)
}

// ScrapeTVShow scrapes TV show metadata
func (s *UnifiedScraper) ScrapeTVShow(ctx context.Context, title string) (*TVShowMetadata, error) {
	cacheKey := fmt.Sprintf("tv:%s", title)

	if s.config.EnableCache && s.cache != nil {
		if meta, ok := s.cache.GetMetadata(cacheKey); ok {
			if tv, ok := meta.(*TVShowMetadata); ok {
				return tv, nil
			}
		}
	}

	sources := s.getSourceOrder()

	var lastErr error
	for _, source := range sources {
		if !s.checkRateLimit(source) {
			continue
		}

		metadata, err := s.scrapeTVFromSource(ctx, source, title)
		if err != nil {
			lastErr = err
			continue
		}

		if s.config.EnableCache && s.cache != nil {
			s.cache.SetMetadata(cacheKey, metadata)
		}

		return metadata, nil
	}

	return nil, fmt.Errorf("failed to scrape TV show from all sources: %v", lastErr)
}

// ScrapeByIMDBID scrapes metadata by IMDB ID
func (s *UnifiedScraper) ScrapeByIMDBID(ctx context.Context, imdbID string) (*MediaMetadata, error) {
	if s.tmdb == nil {
		return nil, fmt.Errorf("TMDB scraper not configured")
	}

	// Use TMDB's find API to get by IMDB ID
	// This requires additional TMDB API implementation
	return nil, fmt.Errorf("IMDB lookup not implemented")
}

// AutoScrape automatically detects media type and scrapes
func (s *UnifiedScraper) AutoScrape(ctx context.Context, filename string) (interface{}, error) {
	scanner := NewScanner(s.cache)
	title, year, season, _ := scanner.ParseFilename(filename)
	mediaType := scanner.DetectMediaType(filename)

	switch mediaType {
	case MediaTypeTVShow:
		if season > 0 {
			// It's an episode, scrape the show
			return s.ScrapeTVShow(ctx, title)
		}
		return s.ScrapeTVShow(ctx, title)
	case MediaTypeMovie:
		return s.ScrapeMovie(ctx, title, year)
	default:
		// Try movie first, then TV
		movie, err := s.ScrapeMovie(ctx, title, year)
		if err == nil {
			return movie, nil
		}
		return s.ScrapeTVShow(ctx, title)
	}
}

// getSourceOrder returns the order of sources to try
func (s *UnifiedScraper) getSourceOrder() []ScraperSource {
	if s.config.PrimarySource != SourceAuto {
		sources := []ScraperSource{s.config.PrimarySource}
		sources = append(sources, s.config.FallbackSources...)
		return sources
	}

	// Auto: prefer Chinese content from Douban, fallback to TMDB
	if s.config.Language == "zh-CN" || s.config.PreferChineseTitle {
		return []ScraperSource{SourceDouban, SourceTMDB}
	}

	return []ScraperSource{SourceTMDB, SourceDouban}
}

// scrapeMovieFromSource scrapes from a specific source
func (s *UnifiedScraper) scrapeMovieFromSource(ctx context.Context, source ScraperSource, title string, year int) (*MediaMetadata, error) {
	switch source {
	case SourceTMDB:
		if s.tmdb == nil {
			return nil, fmt.Errorf("TMDB scraper not configured")
		}
		return s.tmdb.SearchMovie(ctx, title, year)

	case SourceDouban:
		if s.douban == nil {
			return nil, fmt.Errorf("douban scraper not configured")
		}
		movies, err := s.douban.SearchMovie(title)
		if err != nil {
			return nil, err
		}
		if len(movies) == 0 {
			return nil, fmt.Errorf("no results from Douban")
		}
		// Convert to MediaMetadata
		m := movies[0]
		return &MediaMetadata{
			ID:            m.ID,
			Title:         m.Title,
			OriginalTitle: m.OriginalTitle,
			Overview:      m.Overview,
			Rating:        m.Rating,
			VoteCount:     m.VoteCount,
			ReleaseDate:   m.ReleaseDate,
			Genres:        m.Genres,
			Directors:     m.Directors,
			PosterPath:    m.PosterPath,
			ScrapedAt:     time.Now(),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported source: %s", source)
	}
}

// scrapeTVFromSource scrapes TV from a specific source
func (s *UnifiedScraper) scrapeTVFromSource(ctx context.Context, source ScraperSource, title string) (*TVShowMetadata, error) {
	switch source {
	case SourceTMDB:
		if s.tmdb == nil {
			return nil, fmt.Errorf("TMDB scraper not configured")
		}
		return s.tmdb.SearchTVShow(ctx, title)

	case SourceDouban:
		if s.douban == nil {
			return nil, fmt.Errorf("douban scraper not configured")
		}
		shows, err := s.douban.SearchTV(title)
		if err != nil {
			return nil, err
		}
		if len(shows) == 0 {
			return nil, fmt.Errorf("no results from Douban")
		}
		// Convert to TVShowMetadata
		sh := shows[0]
		return &TVShowMetadata{
			MediaMetadata: MediaMetadata{
				ID:            sh.ID,
				Title:         sh.Name,
				OriginalTitle: sh.OriginalName,
				Overview:      sh.Overview,
				Rating:        sh.Rating,
				VoteCount:     sh.VoteCount,
				ReleaseDate:   sh.FirstAirDate,
				Genres:        sh.Genres,
				PosterPath:    sh.PosterPath,
				ScrapedAt:     time.Now(),
			},
			NumberOfSeasons:  sh.Seasons,
			NumberOfEpisodes: sh.Episodes,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported source: %s", source)
	}
}

// enrichMetadata enriches metadata with additional data from other sources
func (s *UnifiedScraper) enrichMetadata(ctx context.Context, meta *MediaMetadata, primarySource ScraperSource) *MediaMetadata {
	// If primary is TMDB and we prefer Chinese, try to get Chinese title from Douban
	if primarySource == SourceTMDB && s.config.PreferChineseTitle && s.douban != nil {
		// Try to enrich with Chinese data
		movies, err := s.douban.SearchMovie(meta.Title)
		if err == nil && len(movies) > 0 {
			// If original title is different, use Chinese title
			if movies[0].Title != meta.Title && movies[0].Title != "" {
				meta.OriginalTitle = meta.Title
				meta.Title = movies[0].Title
			}
		}
	}

	return meta
}

// checkRateLimit checks if we can make a request
func (s *UnifiedScraper) checkRateLimit(source ScraperSource) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	rl, ok := s.rateLimit[source]
	if !ok {
		return true
	}

	now := time.Now()
	if now.Sub(rl.lastTime) > rl.interval {
		rl.tokens = rl.maxTokens
		rl.lastTime = now
	}

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// BatchScrapeResult represents the result of batch scraping
type BatchScrapeResult struct {
	Total     int                   `json:"total"`
	Success   int                   `json:"success"`
	Failed    int                   `json:"failed"`
	Cached    int                   `json:"cached"`
	Results   map[string]interface{} `json:"results"`
	Errors    map[string]string     `json:"errors"`
	Duration  time.Duration         `json:"duration"`
}

// BatchScrape scrapes multiple items in parallel
func (s *UnifiedScraper) BatchScrape(ctx context.Context, items []ScrapeItem, workers int) *BatchScrapeResult {
	if workers <= 0 {
		workers = 3
	}

	result := &BatchScrapeResult{
		Total:   len(items),
		Results: make(map[string]interface{}),
		Errors:  make(map[string]string),
	}

	start := time.Now()
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Create semaphore for worker limit
	sem := make(chan struct{}, workers)

	for _, item := range items {
		wg.Add(1)
		go func(it ScrapeItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var meta interface{}
			var err error

			switch it.Type {
			case MediaTypeMovie:
				meta, err = s.ScrapeMovie(ctx, it.Title, it.Year)
			case MediaTypeTVShow:
				meta, err = s.ScrapeTVShow(ctx, it.Title)
			default:
				meta, err = s.AutoScrape(ctx, it.Filename)
			}

			mu.Lock()
			if err != nil {
				result.Failed++
				result.Errors[it.ID] = err.Error()
			} else {
				result.Success++
				result.Results[it.ID] = meta
			}
			mu.Unlock()
		}(item)
	}

	wg.Wait()
	result.Duration = time.Since(start)
	return result
}

// ScrapeItem represents an item to scrape
type ScrapeItem struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Year     int       `json:"year,omitempty"`
	Type     MediaType `json:"type,omitempty"`
	Filename string    `json:"filename,omitempty"`
}

// GetAvailableSources returns configured sources
func (s *UnifiedScraper) GetAvailableSources() []ScraperSource {
	sources := make([]ScraperSource, 0)
	if s.tmdb != nil {
		sources = append(sources, SourceTMDB)
	}
	if s.douban != nil {
		sources = append(sources, SourceDouban)
	}
	return sources
}

// ClearCache clears the scraper cache
func (s *UnifiedScraper) ClearCache() {
	if s.cache != nil {
		s.cache.Clear()
	}
}