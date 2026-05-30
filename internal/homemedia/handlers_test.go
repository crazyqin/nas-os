package homemedia

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.storagePath != tmpDir {
		t.Errorf("Expected storagePath %s, got %s", tmpDir, manager.storagePath)
	}
}

func TestScanMedia(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "media")
	os.MkdirAll(sourceDir, 0755)

	// Create test files with different content
	testFiles := map[string]string{
		"movie1.mp4": "movie 1 content",
		"movie2.mkv": "movie 2 content",
		"song1.mp3":  "song 1 content",
	}
	for name, content := range testFiles {
		path := filepath.Join(sourceDir, name)
		os.WriteFile(path, []byte(content), 0644)
	}

	manager := NewManager(tmpDir)

	req := &ScanRequest{
		Path:      sourceDir,
		Recursive: true,
	}

	status, err := manager.Scan(context.Background(), req)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Wait for scan to complete (async goroutine)
	deadline := time.After(5 * time.Second)
	for {
		finalStatus, ok := manager.GetScanStatus(status.ID)
		if ok && finalStatus.Status == "completed" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("Scan did not complete in time")
		case <-time.After(50 * time.Millisecond):
		}
	}

	finalStatus, ok := manager.GetScanStatus(status.ID)
	if !ok {
		t.Fatal("Expected to find scan status")
	}

	if finalStatus.Total != 3 {
		t.Errorf("Expected 3 total, got %d", finalStatus.Total)
	}

	if finalStatus.NewFiles != 3 {
		t.Errorf("Expected 3 new files, got %d", finalStatus.NewFiles)
	}
}

func TestCreateCollection(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	collection := manager.CreateCollection("My Movies", "Favorite movies", "movie")

	if collection == nil {
		t.Fatal("Expected collection to be created")
	}

	if collection.Name != "My Movies" {
		t.Errorf("Expected name 'My Movies', got %s", collection.Name)
	}

	if collection.Type != "movie" {
		t.Errorf("Expected type 'movie', got %s", collection.Type)
	}
}

func TestGetCollection(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	created := manager.CreateCollection("Test", "Desc", "movie")

	found, ok := manager.GetCollection(created.ID)
	if !ok {
		t.Fatal("Expected to find collection")
	}

	if found.ID != created.ID {
		t.Errorf("Expected ID %s, got %s", created.ID, found.ID)
	}
}

func TestListCollections(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	manager.CreateCollection("Collection 1", "", "movie")
	manager.CreateCollection("Collection 2", "", "tv")

	collections := manager.ListCollections()

	if len(collections) != 2 {
		t.Errorf("Expected 2 collections, got %d", len(collections))
	}
}

func TestCreatePlaylist(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	playlist := manager.CreatePlaylist("My Playlist", "Test playlist")

	if playlist == nil {
		t.Fatal("Expected playlist to be created")
	}

	if playlist.Name != "My Playlist" {
		t.Errorf("Expected name 'My Playlist', got %s", playlist.Name)
	}

	if playlist.RepeatMode != "none" {
		t.Errorf("Expected repeat mode 'none', got %s", playlist.RepeatMode)
	}
}

func TestGetPlaylist(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	created := manager.CreatePlaylist("Test", "Desc")

	found, ok := manager.GetPlaylist(created.ID)
	if !ok {
		t.Fatal("Expected to find playlist")
	}

	if found.ID != created.ID {
		t.Errorf("Expected ID %s, got %s", created.ID, found.ID)
	}
}

func TestListPlaylists(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	manager.CreatePlaylist("Playlist 1", "")
	manager.CreatePlaylist("Playlist 2", "")
	manager.CreatePlaylist("Playlist 3", "")

	playlists := manager.ListPlaylists()

	if len(playlists) != 3 {
		t.Errorf("Expected 3 playlists, got %d", len(playlists))
	}
}

func TestAddToPlaylist(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Create a media file
	media := &MediaFile{
		ID:        "test-media",
		Filename:  "test.mp4",
		CreatedAt: time.Now(),
	}
	manager.media[media.ID] = media

	playlist := manager.CreatePlaylist("Test", "")

	err := manager.AddToPlaylist(playlist.ID, media.ID)
	if err != nil {
		t.Fatalf("Failed to add to playlist: %v", err)
	}

	found, _ := manager.GetPlaylist(playlist.ID)
	if len(found.Items) != 1 {
		t.Errorf("Expected 1 item in playlist, got %d", len(found.Items))
	}
}

func TestStartPlayback(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	media := &MediaFile{
		ID:        "test-media",
		Filename:  "test.mp4",
		Duration:  3600,
		CreatedAt: time.Now(),
	}
	manager.media[media.ID] = media

	session, err := manager.StartPlayback(media.ID, "user1", "device1", "TV")
	if err != nil {
		t.Fatalf("Failed to start playback: %v", err)
	}

	if session.MediaID != media.ID {
		t.Errorf("Expected media ID %s, got %s", media.ID, session.MediaID)
	}

	if session.Status != "playing" {
		t.Errorf("Expected status 'playing', got %s", session.Status)
	}

	updatedMedia, _ := manager.GetMedia(media.ID)
	if updatedMedia.WatchCount != 1 {
		t.Errorf("Expected watch count 1, got %d", updatedMedia.WatchCount)
	}
}

func TestUpdatePlaybackProgress(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	media := &MediaFile{
		ID:        "test-media",
		Filename:  "test.mp4",
		Duration:  3600,
		CreatedAt: time.Now(),
	}
	manager.media[media.ID] = media

	session, _ := manager.StartPlayback(media.ID, "user1", "device1", "TV")

	err := manager.UpdatePlaybackProgress(session.ID, 1800)
	if err != nil {
		t.Fatalf("Failed to update progress: %v", err)
	}

	found, _ := manager.GetMedia(media.ID)
	if found.Progress != 50 {
		t.Errorf("Expected progress 50, got %f", found.Progress)
	}
}

func TestStopPlayback(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	media := &MediaFile{
		ID:        "test-media",
		Filename:  "test.mp4",
		Duration:  3600,
		CreatedAt: time.Now(),
	}
	manager.media[media.ID] = media

	session, _ := manager.StartPlayback(media.ID, "user1", "device1", "TV")

	err := manager.StopPlayback(session.ID)
	if err != nil {
		t.Fatalf("Failed to stop playback: %v", err)
	}
}

func TestSearchMedia(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Add test media
	media := []*MediaFile{
		{ID: "m1", Filename: "action_movie.mp4", Rating: 5, CreatedAt: time.Now()},
		{ID: "m2", Filename: "comedy.mp4", Rating: 3, CreatedAt: time.Now()},
		{ID: "m3", Filename: "documentary.mkv", Rating: 4, CreatedAt: time.Now()},
	}

	for _, m := range media {
		manager.media[m.ID] = m
	}

	// Test basic search
	req := &MediaSearchRequest{
		Page:     1,
		PageSize: 10,
	}

	result := manager.Search(req)
	if result.Total != 3 {
		t.Errorf("Expected 3 results, got %d", result.Total)
	}

	// Test search with query
	req.Query = "action"
	result = manager.Search(req)
	if result.Total != 1 {
		t.Errorf("Expected 1 result for 'action', got %d", result.Total)
	}

	// Test search with rating
	req.Query = ""
	req.Rating = 4
	result = manager.Search(req)
	if result.Total != 2 {
		t.Errorf("Expected 2 results with rating >= 4, got %d", result.Total)
	}
}

func TestUpdateMedia(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	media := &MediaFile{
		ID:        "test-media",
		Filename:  "test.mp4",
		CreatedAt: time.Now(),
	}
	manager.media[media.ID] = media

	updates := map[string]interface{}{
		"is_favorite": true,
		"rating":      5,
	}

	updated, err := manager.UpdateMedia(media.ID, updates)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if !updated.IsFavorite {
		t.Error("Expected media to be favorite")
	}

	if updated.Rating != 5 {
		t.Errorf("Expected rating 5, got %d", updated.Rating)
	}
}

func TestDeleteMedia(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	media := &MediaFile{
		ID:        "test-media",
		Filename:  "test.mp4",
		Path:      filepath.Join(tmpDir, "test.mp4"),
		CreatedAt: time.Now(),
	}
	os.WriteFile(media.Path, []byte("test"), 0644)
	manager.media[media.ID] = media

	err := manager.DeleteMedia(media.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, ok := manager.GetMedia(media.ID)
	if ok {
		t.Error("Expected media to be deleted")
	}
}

func TestGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Add test data
	manager.media["m1"] = &MediaFile{ID: "m1", Size: 1024, Duration: 3600, CreatedAt: time.Now()}
	manager.media["m2"] = &MediaFile{ID: "m2", Size: 2048, Duration: 7200, CreatedAt: time.Now()}

	stats := manager.GetStats()

	if stats.TotalMedia != 2 {
		t.Errorf("Expected 2 media, got %d", stats.TotalMedia)
	}

	if stats.TotalSize != 3072 {
		t.Errorf("Expected total size 3072, got %d", stats.TotalSize)
	}

	if stats.TotalDuration != 10800 {
		t.Errorf("Expected total duration 10800, got %d", stats.TotalDuration)
	}
}

func TestDetectQuality(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	tests := []struct {
		filename string
		expected string
	}{
		{"movie.1080p.mp4", "1080p"},
		{"movie.720p.mkv", "720p"},
		{"movie.4k.mp4", "4k"},
		{"movie.mp4", "unknown"},
	}

	for _, tt := range tests {
		result := manager.detectQuality(tt.filename)
		if result != tt.expected {
			t.Errorf("detectQuality(%s): expected %s, got %s", tt.filename, tt.expected, result)
		}
	}
}

func TestDetectMimeType(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	tests := []struct {
		ext      string
		expected string
	}{
		{".mp4", "video/mp4"},
		{".mkv", "video/x-matroska"},
		{".mp3", "audio/mpeg"},
		{".flac", "audio/flac"},
		{".unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		result := manager.detectMimeType(tt.ext)
		if result != tt.expected {
			t.Errorf("detectMimeType(%s): expected %s, got %s", tt.ext, tt.expected, result)
		}
	}
}
