package media

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewLibraryManager(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	lm := NewLibraryManager(configPath)
	if lm == nil {
		t.Fatal("NewLibraryManager() returned nil")
	}

	if len(lm.libraries) != 0 {
		t.Errorf("Expected empty libraries map, got %d", len(lm.libraries))
	}
}

func TestCreateLibrary(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	// 创建媒体目录
	mediaPath := filepath.Join(tempDir, "movies")
	if err := os.MkdirAll(mediaPath, 0755); err != nil {
		t.Fatalf("Failed to create media directory: %v", err)
	}

	lib, err := lm.CreateLibrary("Movies", mediaPath, MediaTypeMovie)
	if err != nil {
		t.Fatalf("CreateLibrary() returned error: %v", err)
	}

	if lib == nil {
		t.Fatal("CreateLibrary() returned nil library")
	}

	if lib.ID == "" {
		t.Error("Library ID should not be empty")
	}

	if lib.Name != "Movies" {
		t.Errorf("Library Name = %s, expected 'Movies'", lib.Name)
	}

	if lib.Type != MediaTypeMovie {
		t.Errorf("Library Type = %s, expected 'movie'", lib.Type)
	}

	if !lib.Enabled {
		t.Error("Library should be enabled by default")
	}
}

func TestCreateLibraryInvalidPath(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	_, err := lm.CreateLibrary("Movies", "/nonexistent/path", MediaTypeMovie)
	if err == nil {
		t.Error("CreateLibrary() should return error for nonexistent path")
	}
}

func TestGetLibrary(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	mediaPath := filepath.Join(tempDir, "movies")
	os.MkdirAll(mediaPath, 0755)

	lib, _ := lm.CreateLibrary("Movies", mediaPath, MediaTypeMovie)

	// 获取存在的库
	retrieved := lm.GetLibrary(lib.ID)
	if retrieved == nil {
		t.Error("GetLibrary() should find existing library")
		return
	}

	if retrieved.ID != lib.ID {
		t.Errorf("GetLibrary() ID = %s, expected %s", retrieved.ID, lib.ID)
	}

	// 获取不存在的库
	retrieved = lm.GetLibrary("nonexistent")
	if retrieved != nil {
		t.Error("GetLibrary() should return nil for nonexistent library")
	}
}

func TestListLibraries(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	mediaPath1 := filepath.Join(tempDir, "movies")
	mediaPath2 := filepath.Join(tempDir, "tv")
	os.MkdirAll(mediaPath1, 0755)
	os.MkdirAll(mediaPath2, 0755)

	lm.CreateLibrary("Movies", mediaPath1, MediaTypeMovie)
	lm.CreateLibrary("TV Shows", mediaPath2, MediaTypeTV)

	libs := lm.ListLibraries()
	if len(libs) != 2 {
		t.Errorf("ListLibraries() returned %d libraries, expected 2", len(libs))
	}
}

func TestDeleteLibrary(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	mediaPath := filepath.Join(tempDir, "movies")
	os.MkdirAll(mediaPath, 0755)

	lib, _ := lm.CreateLibrary("Movies", mediaPath, MediaTypeMovie)

	err := lm.DeleteLibrary(lib.ID)
	if err != nil {
		t.Fatalf("DeleteLibrary() returned error: %v", err)
	}

	retrieved := lm.GetLibrary(lib.ID)
	if retrieved != nil {
		t.Error("Library should be deleted")
	}

	// 删除不存在的库
	err = lm.DeleteLibrary("nonexistent")
	if err == nil {
		t.Error("DeleteLibrary() should return error for nonexistent library")
	}
}

func TestUpdateLibrary(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	mediaPath := filepath.Join(tempDir, "movies")
	os.MkdirAll(mediaPath, 0755)

	lib, _ := lm.CreateLibrary("Movies", mediaPath, MediaTypeMovie)

	updates := map[string]interface{}{
		"name":        "Updated Movies",
		"description": "My movie collection",
		"autoScan":    true,
	}

	err := lm.UpdateLibrary(lib.ID, updates)
	if err != nil {
		t.Fatalf("UpdateLibrary() returned error: %v", err)
	}

	retrieved := lm.GetLibrary(lib.ID)
	if retrieved.Name != "Updated Movies" {
		t.Errorf("Updated Name = %s, expected 'Updated Movies'", retrieved.Name)
	}

	if retrieved.Description != "My movie collection" {
		t.Errorf("Updated Description = %s, expected 'My movie collection'", retrieved.Description)
	}
}

func TestAddMetadataProvider(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	mockProvider := &mockMetadataProvider{}
	lm.AddMetadataProvider(mockProvider)

	if len(lm.metadataProviders) != 1 {
		t.Errorf("Expected 1 metadata provider, got %d", len(lm.metadataProviders))
	}
}

func TestScanLibrary(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	// 创建媒体目录和文件
	mediaPath := filepath.Join(tempDir, "movies")
	os.MkdirAll(mediaPath, 0755)

	// 创建一些媒体文件
	videoFile := filepath.Join(mediaPath, "test.mp4")
	if err := os.WriteFile(videoFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	lib, _ := lm.CreateLibrary("Movies", mediaPath, MediaTypeMovie)

	err := lm.ScanLibrary(lib.ID)
	if err != nil {
		t.Fatalf("ScanLibrary() returned error: %v", err)
	}

	// 检查扫描结果
	retrieved := lm.GetLibrary(lib.ID)
	if retrieved.LastScanTime == nil {
		t.Error("LastScanTime should be set after scan")
	}
}

func TestMediaItemIsFavorite(t *testing.T) {
	item := &MediaItem{
		ID:         "item1",
		Name:       "Test Movie",
		Type:       MediaTypeMovie,
		IsFavorite: true,
	}

	if !item.IsFavorite {
		t.Error("IsFavorite should be true")
	}

	item.IsFavorite = false
	if item.IsFavorite {
		t.Error("IsFavorite should be false")
	}
}

func TestMediaItemLastPlayed(t *testing.T) {
	now := time.Now()
	item := &MediaItem{
		ID:         "item1",
		Name:       "Test Movie",
		Type:       MediaTypeMovie,
		LastPlayed: &now,
		PlayCount:  5,
	}

	if item.LastPlayed == nil {
		t.Error("LastPlayed should not be nil")
	}

	if item.PlayCount != 5 {
		t.Errorf("PlayCount = %d, expected 5", item.PlayCount)
	}
}

func TestMediaLibraryEnabled(t *testing.T) {
	lib := &MediaLibrary{
		ID:      "lib1",
		Name:    "Test",
		Enabled: true,
	}

	if !lib.Enabled {
		t.Error("Library should be enabled")
	}

	lib.Enabled = false
	if lib.Enabled {
		t.Error("Library should be disabled")
	}
}

func TestMediaLibraryAutoScan(t *testing.T) {
	lib := &MediaLibrary{
		ID:           "lib1",
		Name:         "Test",
		AutoScan:     true,
		ScanInterval: 60,
	}

	if !lib.AutoScan {
		t.Error("AutoScan should be true")
	}

	if lib.ScanInterval != 60 {
		t.Errorf("ScanInterval = %d, expected 60", lib.ScanInterval)
	}
}

func TestDetectMediaType(t *testing.T) {
	tests := []struct {
		filename string
		expected MediaType
	}{
		{"movie.mp4", MediaTypeMovie},
		{"song.mp3", MediaTypeMusic},
		{"photo.jpg", MediaTypePhoto},
		{"video.avi", MediaTypeMovie},
		{"track.flac", MediaTypeMusic},
		{"image.png", MediaTypePhoto},
	}

	for _, tt := range tests {
		result := detectMediaType(tt.filename)
		if result != tt.expected {
			t.Errorf("detectMediaType(%s) = %s, expected %s", tt.filename, result, tt.expected)
		}
	}
}

// mockMetadataProvider 用于测试的模拟元数据提供商
type mockMetadataProvider struct{}

func (m *mockMetadataProvider) Name() string {
	return "mock"
}

func (m *mockMetadataProvider) SearchMovie(query string) ([]*MovieInfo, error) {
	return []*MovieInfo{}, nil
}

func (m *mockMetadataProvider) GetMovie(id string) (*MovieInfo, error) {
	return &MovieInfo{}, nil
}

func (m *mockMetadataProvider) SearchTV(query string) ([]*TVShowInfo, error) {
	return []*TVShowInfo{}, nil
}

func (m *mockMetadataProvider) GetTV(id string) (*TVShowInfo, error) {
	return &TVShowInfo{}, nil
}

func (m *mockMetadataProvider) SearchMusic(query string) ([]*MusicAlbumInfo, error) {
	return []*MusicAlbumInfo{}, nil
}

func (m *mockMetadataProvider) GetMusic(id string) (*MusicAlbumInfo, error) {
	return &MusicAlbumInfo{}, nil
}

// detectMediaType 检测媒体类型（用于测试）
func detectMediaType(filename string) MediaType {
	ext := filepath.Ext(filename)
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv":
		return MediaTypeMovie
	case ".mp3", ".flac", ".wav", ".aac", ".ogg":
		return MediaTypeMusic
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp":
		return MediaTypePhoto
	default:
		return MediaTypeMovie
	}
}

func TestMediaItemFields(t *testing.T) {
	now := time.Now()
	item := &MediaItem{
		ID:          "test-id",
		Path:        "/path/to/file.mp4",
		Name:        "Test Movie",
		Type:        MediaTypeMovie,
		Size:        1024000,
		ModifiedTime: now,
		Rating:      8.5,
		PlayCount:   10,
		Tags:        []string{"action", "sci-fi"},
	}

	if item.ID != "test-id" {
		t.Error("ID mismatch")
	}
	if item.Type != MediaTypeMovie {
		t.Error("Type mismatch")
	}
	if item.Rating != 8.5 {
		t.Error("Rating mismatch")
	}
	if len(item.Tags) != 2 {
		t.Error("Tags count mismatch")
	}
}

func TestMediaLibraryFields(t *testing.T) {
	now := time.Now()
	lib := &MediaLibrary{
		ID:             "lib-id",
		Name:           "My Movies",
		Description:    "Personal collection",
		Path:           "/movies",
		Type:           MediaTypeMovie,
		Enabled:        true,
		AutoScan:       true,
		ScanInterval:   30,
		LastScanTime:   &now,
		MetadataSource: "tmdb",
	}

	if lib.ID != "lib-id" {
		t.Error("ID mismatch")
	}
	if lib.MetadataSource != "tmdb" {
		t.Error("MetadataSource mismatch")
	}
	if lib.ScanInterval != 30 {
		t.Error("ScanInterval mismatch")
	}
}

func TestListLibraries_Empty(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	libs := lm.ListLibraries()
	if len(libs) != 0 {
		t.Errorf("Expected empty list, got %d", len(libs))
	}
}

func TestUpdateLibrary_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	updates := map[string]interface{}{"name": "New Name"}
	err := lm.UpdateLibrary("nonexistent", updates)
	if err == nil {
		t.Error("UpdateLibrary should return error for nonexistent library")
	}
}

func TestScanLibrary_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	err := lm.ScanLibrary("nonexistent")
	if err == nil {
		t.Error("ScanLibrary should return error for nonexistent library")
	}
}

func TestLibraryManager_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	lm := NewLibraryManager(configPath)

	mediaPath := filepath.Join(tempDir, "movies")
	os.MkdirAll(mediaPath, 0755)

	// Create library
	lib, err := lm.CreateLibrary("Movies", mediaPath, MediaTypeMovie)
	if err != nil {
		t.Fatalf("Failed to create library: %v", err)
	}

	// Concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = lm.GetLibrary(lib.ID)
			_ = lm.ListLibraries()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMediaType_Constants(t *testing.T) {
	tests := []struct {
		mediaType MediaType
		expected  string
	}{
		{MediaTypeMovie, "movie"},
		{MediaTypeTV, "tv"},
		{MediaTypeMusic, "music"},
		{MediaTypePhoto, "photo"},
	}

	for _, tt := range tests {
		if string(tt.mediaType) != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, string(tt.mediaType))
		}
	}
}

func TestMovieInfo_Fields(t *testing.T) {
	info := &MovieInfo{
		ID:            "movie-1",
		Title:         "Test Movie",
		OriginalTitle: "Original Title",
		ReleaseDate:   "2024-01-01",
		Overview:      "A test movie",
		PosterPath:    "/poster.jpg",
		BackdropPath:  "/backdrop.jpg",
		Rating:        8.5,
		VoteCount:     1000,
		Runtime:       120,
		Genres:        []string{"Action", "Drama"},
	}

	if info.Title != "Test Movie" {
		t.Error("Title mismatch")
	}
	if info.ReleaseDate != "2024-01-01" {
		t.Error("ReleaseDate mismatch")
	}
	if len(info.Genres) != 2 {
		t.Error("Genres count mismatch")
	}
}

func TestTVShowInfo_Fields(t *testing.T) {
	info := &TVShowInfo{
		ID:           "tv-1",
		Name:         "Test Show",
		OriginalName: "Original Name",
		FirstAirDate: "2024-01-01",
		Overview:     "A test show",
		PosterPath:   "/poster.jpg",
		BackdropPath: "/backdrop.jpg",
		Rating:       9.0,
		VoteCount:    2000,
		Seasons:      3,
		Episodes:     30,
		Genres:       []string{"Drama"},
	}

	if info.Name != "Test Show" {
		t.Error("Name mismatch")
	}
	if info.Seasons != 3 {
		t.Error("Seasons mismatch")
	}
}

func TestMusicAlbumInfo_Fields(t *testing.T) {
	info := &MusicAlbumInfo{
		ID:          "album-1",
		Title:       "Test Album",
		Artist:      "Test Artist",
		ReleaseDate: "2024-01-01",
		TotalTracks: 12,
		Genres:      []string{"Rock", "Alternative"},
		CoverArt:    "/cover.jpg",
	}

	if info.Title != "Test Album" {
		t.Error("Title mismatch")
	}
	if info.Artist != "Test Artist" {
		t.Error("Artist mismatch")
	}
	if len(info.Genres) != 2 {
		t.Error("Genres count mismatch")
	}
}

func TestPlayHistory_Fields(t *testing.T) {
	now := time.Now()
	history := &PlayHistory{
		ID:         "history-1",
		MediaID:    "media-1",
		MediaName:  "Test Movie",
		MediaType:  MediaTypeMovie,
		LibraryID:  "lib-1",
		Position:   3600,
		Duration:   7200,
		PlayedAt:   now,
		Completed:  false,
	}

	if history.MediaID != "media-1" {
		t.Error("MediaID mismatch")
	}
	if history.Position != 3600 {
		t.Error("Position mismatch")
	}
	if history.Completed {
		t.Error("Completed should be false")
	}
}
