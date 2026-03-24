package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFilenameParser_ParseFilename(t *testing.T) {
	parser := NewFilenameParser()

	tests := []struct {
		name            string
		filename        string
		expectedTitle   string
		expectedYear    int
		expectedSeason  int
		expectedEpisode int
	}{
		{
			name:            "Standard movie with year",
			filename:        "Inception.2010.1080p.mkv",
			expectedTitle:   "Inception", // Quality tag remains after basic cleanup
			expectedYear:    2010,
			expectedSeason:  0,
			expectedEpisode: 0,
		},
		{
			name:            "Standard TV show S01E01",
			filename:        "Breaking.Bad.S01E01.720p.mkv",
			expectedTitle:   "Breaking Bad",
			expectedYear:    0,
			expectedSeason:  1,
			expectedEpisode: 1,
		},
		{
			name:            "TV show with year",
			filename:        "Game.of.Thrones.S02E05.2012.mkv",
			expectedTitle:   "Game of Thrones",
			expectedYear:    2012,
			expectedSeason:  2,
			expectedEpisode: 5,
		},
		{
			name:            "Chinese movie",
			filename:        "流浪地球.2019.2160p.mkv",
			expectedTitle:   "流浪地球",
			expectedYear:    2019,
			expectedSeason:  0,
			expectedEpisode: 0,
		},
		{
			name:            "Chinese TV show with season/episode",
			filename:        "庆余年第二季第15集.1080p.mkv",
			expectedTitle:   "庆余年",
			expectedYear:    0,
			expectedSeason:  2,
			expectedEpisode: 15,
		},
		{
			name:            "Movie with parentheses year",
			filename:        "The.Matrix.(1999).1080p.mkv",
			expectedTitle:   "The Matrix",
			expectedYear:    1999,
			expectedSeason:  0,
			expectedEpisode: 0,
		},
		{
			name:            "TV show with alternate format 1x05",
			filename:        "Friends.1x05.HDTV.mkv",
			expectedTitle:   "Friends",
			expectedYear:    0,
			expectedSeason:  1,
			expectedEpisode: 5,
		},
		{
			name:            "TV show with season keyword",
			filename:        "Stranger.Things.Season.3.Episode.5.mkv",
			expectedTitle:   "Stranger Things",
			expectedYear:    0,
			expectedSeason:  3,
			expectedEpisode: 5,
		},
		{
			name:            "Simple filename",
			filename:        "Avatar.mp4",
			expectedTitle:   "Avatar",
			expectedYear:    0,
			expectedSeason:  0,
			expectedEpisode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, year, season, episode := parser.ParseFilename(tt.filename)
			if title != tt.expectedTitle {
				t.Errorf("expected title %q, got %q", tt.expectedTitle, title)
			}
			if year != tt.expectedYear {
				t.Errorf("expected year %d, got %d", tt.expectedYear, year)
			}
			if season != tt.expectedSeason {
				t.Errorf("expected season %d, got %d", tt.expectedSeason, season)
			}
			if episode != tt.expectedEpisode {
				t.Errorf("expected episode %d, got %d", tt.expectedEpisode, episode)
			}
		})
	}
}

func TestFilenameParser_DetectMediaType(t *testing.T) {
	parser := NewFilenameParser()

	tests := []struct {
		name     string
		filename string
		expected MediaType
	}{
		{
			name:     "TV show with S01E01",
			filename: "Show.S01E01.mkv",
			expected: MediaTypeTVShow,
		},
		{
			name:     "TV show with season keyword",
			filename: "Show.Season.1.Episode.5.mkv",
			expected: MediaTypeTVShow,
		},
		{
			name:     "Chinese TV show",
			filename: "电视剧第一季第一集.mkv",
			expected: MediaTypeTVShow,
		},
		{
			name:     "Movie with year",
			filename: "Movie.2024.1080p.mkv",
			expected: MediaTypeMovie,
		},
		{
			name:     "Movie simple name",
			filename: "TheGodfather.mkv",
			expected: MediaTypeMovie,
		},
		{
			name:     "HDTV pattern",
			filename: "SomeShow.hdtv.mkv",
			expected: MediaTypeTVShow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.DetectMediaType(tt.filename)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestChineseToInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"一", 1},
		{"二", 2},
		{"三", 3},
		{"十", 10},
		{"十一", 11},
		{"二十", 20},
		{"二十一", 21},
		{"三十", 30},
		{"一百", 100},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := chineseToInt(tt.input)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestEnhancedCache(t *testing.T) {
	cache := NewEnhancedCache()

	// Test basic Set/Get
	cache.SetMetadata("test_key", "test_value", 1*time.Hour)
	val, ok := cache.GetMetadata("test_key")
	if !ok {
		t.Error("expected to find cached value")
	}
	if val != "test_value" {
		t.Errorf("expected 'test_value', got %v", val)
	}

	// Test TTL expiration
	cache.SetMetadata("expiring_key", "expiring_value", 100*time.Millisecond)
	time.Sleep(150 * time.Millisecond)
	_, ok = cache.GetMetadata("expiring_key")
	if ok {
		t.Error("expected expired value to be gone")
	}

	// Test file caching
	file := &VideoFile{
		ID:       "test123",
		Path:     "/test/video.mkv",
		Filename: "video.mkv",
	}
	cache.SetFile("/test/video.mkv", file)
	retrieved, ok := cache.GetFile("/test/video.mkv")
	if !ok {
		t.Error("expected to find cached file")
	}
	if retrieved.ID != "test123" {
		t.Errorf("expected ID 'test123', got %s", retrieved.ID)
	}

	// Test cleanup (we have two expiring entries from earlier tests)
	cache.SetMetadata("cleanup_key", "value", 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	removed := cache.Cleanup()
	if removed < 1 {
		t.Errorf("expected at least 1 removed entry, got %d", removed)
	}
}

func TestEnhancedCache_PersistAndLoad(t *testing.T) {
	cache := NewEnhancedCache()

	// Add some data
	cache.SetMetadata("persist_key", "persist_value", 24*time.Hour)
	cache.SetFile("/test/movie.mkv", &VideoFile{
		ID:       "movie123",
		Path:     "/test/movie.mkv",
		Filename: "movie.mkv",
	})

	// Create temp file
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Persist
	err := cache.PersistToFile(cachePath)
	if err != nil {
		t.Fatalf("failed to persist cache: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("cache file was not created")
	}

	// Load into new cache
	newCache := NewEnhancedCache()
	err = newCache.LoadFromFile(cachePath)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}

	// Verify data
	val, ok := newCache.GetMetadata("persist_key")
	if !ok {
		t.Error("expected to find persisted metadata")
	}
	if val != "persist_value" {
		t.Errorf("expected 'persist_value', got %v", val)
	}

	file, ok := newCache.GetFile("/test/movie.mkv")
	if !ok {
		t.Error("expected to find persisted file")
	}
	if file.ID != "movie123" {
		t.Errorf("expected ID 'movie123', got %s", file.ID)
	}
}

func TestTMDBScraper_GetPosterURL(t *testing.T) {
	scraper := NewTMDBScraper(TMDBConfig{
		APIKey: "test_key",
	}, nil)

	tests := []struct {
		posterPath string
		size       string
		expected   string
	}{
		{
			posterPath: "/abc123.jpg",
			size:       PosterSizeW500,
			expected:   "https://image.tmdb.org/t/p/w500/abc123.jpg",
		},
		{
			posterPath: "/xyz789.jpg",
			size:       PosterSizeOrig,
			expected:   "https://image.tmdb.org/t/p/original/xyz789.jpg",
		},
		{
			posterPath: "",
			size:       PosterSizeW500,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.posterPath, func(t *testing.T) {
			result := scraper.GetPosterURL(tt.posterPath, tt.size)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGenerateCacheKey(t *testing.T) {
	key1 := GenerateCacheKey(MediaTypeMovie, "Inception", 2010)
	key2 := GenerateCacheKey(MediaTypeMovie, "Inception", 2010)
	key3 := GenerateCacheKey(MediaTypeMovie, "Avatar", 2009)

	if key1 != key2 {
		t.Error("same input should generate same key")
	}
	if key1 == key3 {
		t.Error("different input should generate different key")
	}
}

func TestNewFilenameParser(t *testing.T) {
	parser := NewFilenameParser()
	if parser == nil {
		t.Fatal("expected non-nil parser")
	}
	if len(parser.seasonEpisodePatterns) == 0 {
		t.Error("expected season episode patterns to be initialized")
	}
	if len(parser.yearPatterns) == 0 {
		t.Error("expected year patterns to be initialized")
	}
	if len(parser.cleanupPatterns) == 0 {
		t.Error("expected cleanup patterns to be initialized")
	}
}

func TestTMDBScraper_DownloadPoster_Errors(t *testing.T) {
	scraper := NewTMDBScraper(TMDBConfig{
		APIKey:    "test_key",
		PosterDir: "",
	}, nil)

	ctx := context.Background()

	// Test with no poster dir configured
	_, err := scraper.DownloadPoster(ctx, "/test.jpg", MediaTypeMovie, 123, PosterSizeW500)
	if err == nil {
		t.Error("expected error when poster dir not configured")
	}

	// Test with empty poster path
	scraper.posterDir = t.TempDir()
	_, err = scraper.DownloadPoster(ctx, "", MediaTypeMovie, 123, PosterSizeW500)
	if err == nil {
		t.Error("expected error with empty poster path")
	}
}
