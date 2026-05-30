package smartphoto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, true)

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.storagePath != tmpDir {
		t.Errorf("Expected storagePath %s, got %s", tmpDir, manager.storagePath)
	}

	if !manager.aiEnabled {
		t.Error("Expected AI to be enabled")
	}
}

func TestCreateAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	album := manager.CreateAlbum("Test Album", "Description", false, nil)

	if album == nil {
		t.Fatal("Expected album to be created")
	}

	if album.Name != "Test Album" {
		t.Errorf("Expected name 'Test Album', got %s", album.Name)
	}

	if album.Description != "Description" {
		t.Errorf("Expected description 'Description', got %s", album.Description)
	}

	if album.IsSmart {
		t.Error("Expected album not to be smart")
	}
}

func TestGetAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	created := manager.CreateAlbum("Test", "Desc", false, nil)

	found, ok := manager.GetAlbum(created.ID)
	if !ok {
		t.Fatal("Expected to find album")
	}

	if found.ID != created.ID {
		t.Errorf("Expected ID %s, got %s", created.ID, found.ID)
	}
}

func TestListAlbums(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	manager.CreateAlbum("Album 1", "", false, nil)
	manager.CreateAlbum("Album 2", "", false, nil)

	albums := manager.ListAlbums()

	if len(albums) != 2 {
		t.Errorf("Expected 2 albums, got %d", len(albums))
	}
}

func TestCreatePerson(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	person := manager.CreatePerson("John Doe")

	if person == nil {
		t.Fatal("Expected person to be created")
	}

	if person.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got %s", person.Name)
	}
}

func TestGetPerson(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	created := manager.CreatePerson("Jane")

	found, ok := manager.GetPerson(created.ID)
	if !ok {
		t.Fatal("Expected to find person")
	}

	if found.Name != "Jane" {
		t.Errorf("Expected name 'Jane', got %s", found.Name)
	}
}

func TestListPersons(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	manager.CreatePerson("Person 1")
	manager.CreatePerson("Person 2")
	manager.CreatePerson("Person 3")

	persons := manager.ListPersons()

	if len(persons) != 3 {
		t.Errorf("Expected 3 persons, got %d", len(persons))
	}
}

func TestCreateShare(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	req := &ShareRequest{
		PhotoIDs:      []string{"photo1", "photo2"},
		ExpiresIn:     24,
		AllowDownload: true,
		MaxViews:      100,
	}

	share := manager.CreateShare(req)

	if share == nil {
		t.Fatal("Expected share to be created")
	}

	if len(share.PhotoIDs) != 2 {
		t.Errorf("Expected 2 photo IDs, got %d", len(share.PhotoIDs))
	}

	if share.ExpiresAt == nil {
		t.Error("Expected expiration time to be set")
	}

	if share.MaxViews != 100 {
		t.Errorf("Expected max views 100, got %d", share.MaxViews)
	}
}

func TestGetShare(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	req := &ShareRequest{
		PhotoIDs: []string{"photo1"},
	}

	created := manager.CreateShare(req)

	found, ok := manager.GetShare(created.ID)
	if !ok {
		t.Fatal("Expected to find share")
	}

	if found.ID != created.ID {
		t.Errorf("Expected ID %s, got %s", created.ID, found.ID)
	}
}

func TestImportPhotos(t *testing.T) {
	tmpDir := t.TempDir()
	storageDir := filepath.Join(tmpDir, "storage")
	sourceDir := filepath.Join(tmpDir, "source")

	os.MkdirAll(storageDir, 0755)
	os.MkdirAll(sourceDir, 0755)

	// Create test files
	testFiles := []string{"test1.jpg", "test2.png", "test3.gif"}
	for i, name := range testFiles {
		path := filepath.Join(sourceDir, name)
		os.WriteFile(path, []byte(fmt.Sprintf("test content %d", i)), 0644)
	}

	manager := NewManager(storageDir, false)

	req := &ImportRequest{
		SourcePath:     sourceDir,
		Recursive:      true,
		DuplicateCheck: true,
		AIAnalysis:     false,
	}

	status, err := manager.Import(context.Background(), req)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Wait for import to complete
	time.Sleep(100 * time.Millisecond)

	finalStatus, ok := manager.GetImportStatus(status.ID)
	if !ok {
		t.Fatal("Expected to find import status")
	}

	if finalStatus.Total != 3 {
		t.Errorf("Expected 3 total, got %d", finalStatus.Total)
	}

	if finalStatus.Processed != 3 {
		t.Errorf("Expected 3 processed, got %d", finalStatus.Processed)
	}
}

func TestSearchPhotos(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	// Create test photos directly
	now := time.Now()
	photos := []*Photo{
		{ID: "p1", Filename: "photo1.jpg", CreatedAt: now, Rating: 5},
		{ID: "p2", Filename: "photo2.jpg", CreatedAt: now.Add(-time.Hour), Rating: 3},
		{ID: "p3", Filename: "vacation.jpg", CreatedAt: now.Add(-2 * time.Hour), Rating: 4, IsFavorite: true},
	}

	for _, p := range photos {
		manager.photos[p.ID] = p
	}

	// Test basic search
	req := &SearchRequest{
		Page:     1,
		PageSize: 10,
	}

	result := manager.Search(req)
	if result.Total != 3 {
		t.Errorf("Expected 3 results, got %d", result.Total)
	}

	// Test search with rating filter
	req.Rating = 4
	result = manager.Search(req)
	if result.Total != 2 {
		t.Errorf("Expected 2 results with rating >= 4, got %d", result.Total)
	}

	// Test search with favorite filter
	req.Rating = 0
	fav := true
	req.IsFavorite = &fav
	result = manager.Search(req)
	if result.Total != 1 {
		t.Errorf("Expected 1 favorite result, got %d", result.Total)
	}

	// Test search with query
	req.IsFavorite = nil
	req.Query = "vacation"
	result = manager.Search(req)
	if result.Total != 1 {
		t.Errorf("Expected 1 result for 'vacation', got %d", result.Total)
	}
}

func TestUpdatePhoto(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	photo := &Photo{
		ID:        "test-photo",
		Filename:  "test.jpg",
		CreatedAt: time.Now(),
	}
	manager.photos[photo.ID] = photo

	updates := map[string]interface{}{
		"is_favorite": true,
		"rating":      5,
		"comments":    "Great photo!",
	}

	updated, err := manager.UpdatePhoto(photo.ID, updates)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if !updated.IsFavorite {
		t.Error("Expected photo to be favorite")
	}

	if updated.Rating != 5 {
		t.Errorf("Expected rating 5, got %d", updated.Rating)
	}

	if updated.Comments != "Great photo!" {
		t.Errorf("Expected comments 'Great photo!', got %s", updated.Comments)
	}
}

func TestDeletePhoto(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	photo := &Photo{
		ID:        "test-photo",
		Filename:  "test.jpg",
		Path:      filepath.Join(tmpDir, "test.jpg"),
		CreatedAt: time.Now(),
	}
	os.WriteFile(photo.Path, []byte("test"), 0644)
	manager.photos[photo.ID] = photo

	err := manager.DeletePhoto(photo.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, ok := manager.GetPhoto(photo.ID)
	if ok {
		t.Error("Expected photo to be deleted")
	}

	if _, err := os.Stat(photo.Path); !os.IsNotExist(err) {
		t.Error("Expected file to be deleted")
	}
}

func TestGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	// Add test data
	now := time.Now()
	manager.photos["p1"] = &Photo{ID: "p1", Size: 1024, CreatedAt: now}
	manager.photos["p2"] = &Photo{ID: "p2", Size: 2048, CreatedAt: now}
	manager.albums["a1"] = &Album{ID: "a1"}
	manager.persons["pe1"] = &Person{ID: "pe1"}

	stats := manager.GetStats()

	if stats.TotalPhotos != 2 {
		t.Errorf("Expected 2 photos, got %d", stats.TotalPhotos)
	}

	if stats.TotalAlbums != 1 {
		t.Errorf("Expected 1 album, got %d", stats.TotalAlbums)
	}

	if stats.TotalPersons != 1 {
		t.Errorf("Expected 1 person, got %d", stats.TotalPersons)
	}

	if stats.TotalSize != 3072 {
		t.Errorf("Expected total size 3072, got %d", stats.TotalSize)
	}
}

func TestFindDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	// Create photos with same hash
	manager.photos["p1"] = &Photo{ID: "p1", Hash: "abc123", Size: 1024}
	manager.photos["p2"] = &Photo{ID: "p2", Hash: "abc123", Size: 1024}
	manager.photos["p3"] = &Photo{ID: "p3", Hash: "def456", Size: 512}

	groups := manager.FindDuplicates()

	if len(groups) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(groups))
	}

	if len(groups[0].Photos) != 2 {
		t.Errorf("Expected 2 photos in group, got %d", len(groups[0].Photos))
	}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	// Create duplicate photos
	manager.photos["p1"] = &Photo{ID: "p1", Hash: "abc123", Size: 1024, Path: filepath.Join(tmpDir, "p1.jpg")}
	manager.photos["p2"] = &Photo{ID: "p2", Hash: "abc123", Size: 1024, Path: filepath.Join(tmpDir, "p2.jpg")}

	req := &CleanupRequest{
		Duplicates: true,
		DryRun:     false,
	}

	result := manager.Cleanup(req)

	if result.Duplicates != 1 {
		t.Errorf("Expected 1 duplicate, got %d", result.Duplicates)
	}

	if result.TotalRemoved != 1 {
		t.Errorf("Expected 1 removed, got %d", result.TotalRemoved)
	}
}

func TestDetectMimeType(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, false)

	tests := []struct {
		path     string
		expected string
	}{
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"photo.png", "image/png"},
		{"photo.gif", "image/gif"},
		{"photo.webp", "image/webp"},
		{"file.unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		result := manager.detectMimeType(tt.path)
		if result != tt.expected {
			t.Errorf("detectMimeType(%s): expected %s, got %s", tt.path, tt.expected, result)
		}
	}
}
