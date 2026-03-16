package media

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewTMDBProvider(t *testing.T) {
	provider := NewTMDBProvider("test-api-key", "zh-CN")
	if provider == nil {
		t.Fatal("NewTMDBProvider returned nil")
	}

	if provider.apiKey != "test-api-key" {
		t.Errorf("apiKey = %s, want test-api-key", provider.apiKey)
	}

	if provider.language != "zh-CN" {
		t.Errorf("language = %s, want zh-CN", provider.language)
	}
}

func TestNewTMDBProvider_DefaultLanguage(t *testing.T) {
	provider := NewTMDBProvider("test-api-key", "")
	if provider.language != "zh-CN" {
		t.Errorf("Default language = %s, want zh-CN", provider.language)
	}
}

func TestTMDBProvider_GetImageURL(t *testing.T) {
	provider := NewTMDBProvider("test-key", "en")

	tests := []struct {
		path     string
		expected string
	}{
		{"/abc123.jpg", "https://image.tmdb.org/t/p/original/abc123.jpg"},
		{"", ""},
	}

	for _, tt := range tests {
		result := provider.getImageURL(tt.path)
		if result != tt.expected {
			t.Errorf("getImageURL(%s) = %s, want %s", tt.path, result, tt.expected)
		}
	}
}

func TestTMDBProvider_SearchMusic(t *testing.T) {
	provider := NewTMDBProvider("test-key", "en")

	results, err := provider.SearchMusic("test query")
	if err != nil {
		t.Errorf("SearchMusic returned error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("SearchMusic should return empty slice for TMDB")
	}
}

func TestTMDBProvider_GetMusic(t *testing.T) {
	provider := NewTMDBProvider("test-key", "en")

	_, err := provider.GetMusic("test-id")
	if err == nil {
		t.Error("GetMusic should return error for TMDB")
	}
}

func TestTMDBProvider_SearchMovie_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"results": [
				{
					"id": 123,
					"title": "Test Movie",
					"original_title": "Original Test",
					"overview": "Test overview",
					"release_date": "2024-01-01",
					"vote_average": 8.5,
					"vote_count": 1000,
					"poster_path": "/poster.jpg",
					"backdrop_path": "/backdrop.jpg"
				}
			]
		}`))
	}))
	defer server.Close()

	provider := NewTMDBProvider("test-key", "en")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	results, err := provider.SearchMovie("test")
	if err != nil {
		t.Fatalf("SearchMovie returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Title != "Test Movie" {
		t.Errorf("Title = %s, want Test Movie", results[0].Title)
	}

	if results[0].ID != "tmdb_123" {
		t.Errorf("ID = %s, want tmdb_123", results[0].ID)
	}
}

func TestTMDBProvider_SearchTV_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"results": [
				{
					"id": 456,
					"name": "Test Show",
					"original_name": "Original Show",
					"overview": "Test overview",
					"first_air_date": "2023-01-01",
					"vote_average": 9.0,
					"vote_count": 2000,
					"poster_path": "/poster.jpg",
					"backdrop_path": "/backdrop.jpg"
				}
			]
		}`))
	}))
	defer server.Close()

	provider := NewTMDBProvider("test-key", "en")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	results, err := provider.SearchTV("test")
	if err != nil {
		t.Fatalf("SearchTV returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Name != "Test Show" {
		t.Errorf("Name = %s, want Test Show", results[0].Name)
	}
}

func TestTMDBProvider_GetMovie_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": 123,
			"title": "Test Movie Detail",
			"original_title": "Original Test",
			"overview": "Detailed overview",
			"runtime": 120,
			"release_date": "2024-01-01",
			"vote_average": 8.5,
			"vote_count": 1000,
			"poster_path": "/poster.jpg",
			"backdrop_path": "/backdrop.jpg",
			"tagline": "Test tagline",
			"genres": [
				{"id": 1, "name": "Action"},
				{"id": 2, "name": "Drama"}
			],
			"spoken_languages": [{"iso_639_1": "en", "name": "English"}],
			"production_countries": [{"iso_3166_1": "US", "name": "USA"}],
			"credits": {
				"cast": [{"name": "Actor A"}, {"name": "Actor B"}],
				"crew": [{"name": "Director X", "job": "Director"}]
			}
		}`))
	}))
	defer server.Close()

	provider := NewTMDBProvider("test-key", "en")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	movie, err := provider.GetMovie("tmdb_123")
	if err != nil {
		t.Fatalf("GetMovie returned error: %v", err)
	}

	if movie.Title != "Test Movie Detail" {
		t.Errorf("Title = %s, want Test Movie Detail", movie.Title)
	}

	if movie.Runtime != 120 {
		t.Errorf("Runtime = %d, want 120", movie.Runtime)
	}

	if len(movie.Genres) != 2 {
		t.Errorf("Genres count = %d, want 2", len(movie.Genres))
	}

	if len(movie.Directors) != 1 {
		t.Errorf("Directors count = %d, want 1", len(movie.Directors))
	}
}

func TestTMDBProvider_GetTV_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": 456,
			"name": "Test Show Detail",
			"original_name": "Original Show",
			"overview": "Detailed overview",
			"first_air_date": "2023-01-01",
			"last_air_date": "2024-01-01",
			"status": "Returning Series",
			"vote_average": 9.0,
			"vote_count": 2000,
			"poster_path": "/poster.jpg",
			"backdrop_path": "/backdrop.jpg",
			"genres": [{"id": 1, "name": "Drama"}],
			"seasons": [
				{"season_number": 1, "episode_count": 10},
				{"season_number": 2, "episode_count": 10}
			],
			"networks": [{"name": "Netflix"}],
			"created_by": [{"name": "Creator A"}],
			"credits": {
				"cast": [{"name": "Actor X"}]
			},
			"spoken_languages": [{"name": "English"}],
			"origin_country": ["US"]
		}`))
	}))
	defer server.Close()

	provider := NewTMDBProvider("test-key", "en")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	show, err := provider.GetTV("tmdb_456")
	if err != nil {
		t.Fatalf("GetTV returned error: %v", err)
	}

	if show.Name != "Test Show Detail" {
		t.Errorf("Name = %s, want Test Show Detail", show.Name)
	}

	if show.Seasons != 2 {
		t.Errorf("Seasons = %d, want 2", show.Seasons)
	}

	if show.Episodes != 20 {
		t.Errorf("Episodes = %d, want 20", show.Episodes)
	}
}

func TestMetadataProvider_Interface(t *testing.T) {
	// Ensure TMDBProvider implements MetadataProvider interface
	var _ MetadataProvider = NewTMDBProvider("test-key", "en")
}
