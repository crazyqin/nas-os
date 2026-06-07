// Package phototimeline provides photo timeline management for NAS-OS.
package phototimeline

import (
	"fmt"
	"testing"
	"time"
)

func TestTimelineManager_AddPhoto(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	photo := &Photo{
		ID:       "test1",
		Filename: "test.jpg",
		Size:     1024,
		TakenAt:  time.Now(),
	}

	if err := tm.AddPhoto(photo); err != nil {
		t.Errorf("AddPhoto failed: %v", err)
	}

	if tm.Count() != 1 {
		t.Errorf("Expected count 1, got %d", tm.Count())
	}
}

func TestTimelineManager_AddPhoto_Nil(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	if err := tm.AddPhoto(nil); err == nil {
		t.Error("Expected error for nil photo")
	}
}

func TestTimelineManager_AddPhoto_EmptyID(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	photo := &Photo{
		Filename: "test.jpg",
	}

	if err := tm.AddPhoto(photo); err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestTimelineManager_GetPhoto(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	photo := &Photo{
		ID:       "test1",
		Filename: "test.jpg",
		Size:     1024,
		TakenAt:  time.Now(),
	}

	tm.AddPhoto(photo)

	result, err := tm.GetPhoto("test1")
	if err != nil {
		t.Errorf("GetPhoto failed: %v", err)
	}

	if result.ID != "test1" {
		t.Errorf("Expected ID 'test1', got '%s'", result.ID)
	}
}

func TestTimelineManager_GetPhoto_NotFound(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	_, err := tm.GetPhoto("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent photo")
	}
}

func TestTimelineManager_RemovePhoto(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	photo := &Photo{
		ID:       "test1",
		Filename: "test.jpg",
		TakenAt:  time.Now(),
	}

	tm.AddPhoto(photo)

	if err := tm.RemovePhoto("test1"); err != nil {
		t.Errorf("RemovePhoto failed: %v", err)
	}

	if tm.Count() != 0 {
		t.Errorf("Expected count 0, got %d", tm.Count())
	}
}

func TestTimelineManager_RemovePhoto_NotFound(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	if err := tm.RemovePhoto("nonexistent"); err == nil {
		t.Error("Expected error for nonexistent photo")
	}
}

func TestTimelineManager_UpdatePhoto(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	photo := &Photo{
		ID:       "test1",
		Filename: "test.jpg",
		TakenAt:  time.Now(),
	}

	tm.AddPhoto(photo)

	photo.Favorite = true
	photo.Rating = 5

	if err := tm.UpdatePhoto(photo); err != nil {
		t.Errorf("UpdatePhoto failed: %v", err)
	}

	result, _ := tm.GetPhoto("test1")
	if !result.Favorite {
		t.Error("Expected favorite to be true")
	}
	if result.Rating != 5 {
		t.Errorf("Expected rating 5, got %d", result.Rating)
	}
}

func TestTimelineManager_GetTimeline(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	now := time.Now()
	photos := []*Photo{
		{ID: "1", Filename: "1.jpg", TakenAt: now},
		{ID: "2", Filename: "2.jpg", TakenAt: now.AddDate(0, -1, 0)},
		{ID: "3", Filename: "3.jpg", TakenAt: now.AddDate(0, -2, 0)},
	}

	for _, p := range photos {
		tm.AddPhoto(p)
	}

	result, err := tm.GetTimeline(TimelineViewMonth, 1, 10)
	if err != nil {
		t.Errorf("GetTimeline failed: %v", err)
	}

	if len(result.Groups) == 0 {
		t.Error("Expected at least one group")
	}
}

func TestTimelineManager_GetTimeline_Pagination(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	now := time.Now()
	for i := 0; i < 150; i++ {
		photo := &Photo{
			ID:       fmt.Sprintf("photo_%d", i),
			Filename: fmt.Sprintf("photo_%d.jpg", i),
			TakenAt:  now.AddDate(0, 0, -i),
		}
		tm.AddPhoto(photo)
	}

	result, _ := tm.GetTimeline(TimelineViewDay, 1, 50)

	if len(result.Groups) != 50 {
		t.Errorf("Expected 50 groups, got %d", len(result.Groups))
	}

	if !result.HasMore {
		t.Error("Expected HasMore to be true")
	}
}

func TestTimelineManager_GetStats(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	photos := []*Photo{
		{
			ID:       "1",
			Size:     1024,
			TakenAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			EXIF:     EXIFData{CameraModel: "iPhone 15"},
			Location: "Beijing",
		},
		{
			ID:       "2",
			Size:     2048,
			TakenAt:  time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			EXIF:     EXIFData{CameraModel: "iPhone 15"},
			Location: "Shanghai",
		},
	}

	for _, p := range photos {
		tm.AddPhoto(p)
	}

	stats := tm.GetStats()

	if stats.TotalPhotos != 2 {
		t.Errorf("Expected 2 photos, got %d", stats.TotalPhotos)
	}

	if stats.TotalSize != 3072 {
		t.Errorf("Expected size 3072, got %d", stats.TotalSize)
	}
}

func TestTimelineManager_GetPhotosByDateRange(t *testing.T) {
	config := DefaultConfig()
	tm := NewTimelineManager(config)

	photos := []*Photo{
		{ID: "1", TakenAt: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{ID: "2", TakenAt: time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC)},
		{ID: "3", TakenAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
	}

	for _, p := range photos {
		tm.AddPhoto(p)
	}

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC)

	result := tm.GetPhotosByDateRange(from, to)

	if len(result) != 2 {
		t.Errorf("Expected 2 photos, got %d", len(result))
	}
}

func TestAlbumManager_CreateAlbum(t *testing.T) {
	config := DefaultConfig()
	photos := make(map[string]*Photo)
	am := NewAlbumManager(config, photos)

	album := &Album{
		ID:   "album1",
		Name: "Test Album",
		Type: AlbumTypeManual,
	}

	if err := am.CreateAlbum(album); err != nil {
		t.Errorf("CreateAlbum failed: %v", err)
	}
}

func TestAlbumManager_CreateAlbum_Duplicate(t *testing.T) {
	config := DefaultConfig()
	photos := make(map[string]*Photo)
	am := NewAlbumManager(config, photos)

	album := &Album{
		ID:   "album1",
		Name: "Test Album",
		Type: AlbumTypeManual,
	}

	am.CreateAlbum(album)

	if err := am.CreateAlbum(album); err == nil {
		t.Error("Expected error for duplicate album")
	}
}

func TestAlbumManager_GetAlbum(t *testing.T) {
	config := DefaultConfig()
	photos := make(map[string]*Photo)
	am := NewAlbumManager(config, photos)

	album := &Album{
		ID:   "album1",
		Name: "Test Album",
		Type: AlbumTypeManual,
	}

	am.CreateAlbum(album)

	result, err := am.GetAlbum("album1")
	if err != nil {
		t.Errorf("GetAlbum failed: %v", err)
	}

	if result.Name != "Test Album" {
		t.Errorf("Expected name 'Test Album', got '%s'", result.Name)
	}
}

func TestAlbumManager_DeleteAlbum(t *testing.T) {
	config := DefaultConfig()
	photos := make(map[string]*Photo)
	am := NewAlbumManager(config, photos)

	album := &Album{
		ID:   "album1",
		Name: "Test Album",
		Type: AlbumTypeManual,
	}

	am.CreateAlbum(album)

	if err := am.DeleteAlbum("album1"); err != nil {
		t.Errorf("DeleteAlbum failed: %v", err)
	}

	if _, err := am.GetAlbum("album1"); err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestAlbumManager_AddPhotosToAlbum(t *testing.T) {
	config := DefaultConfig()
	photos := map[string]*Photo{
		"photo1": {ID: "photo1", Filename: "photo1.jpg"},
		"photo2": {ID: "photo2", Filename: "photo2.jpg"},
	}
	am := NewAlbumManager(config, photos)

	album := &Album{
		ID:   "album1",
		Name: "Test Album",
		Type: AlbumTypeManual,
	}

	am.CreateAlbum(album)

	if err := am.AddPhotosToAlbum("album1", []string{"photo1", "photo2"}); err != nil {
		t.Errorf("AddPhotosToAlbum failed: %v", err)
	}

	album, _ = am.GetAlbum("album1")
	if album.PhotoCount != 2 {
		t.Errorf("Expected 2 photos, got %d", album.PhotoCount)
	}
}

func TestAlbumManager_RemovePhotosFromAlbum(t *testing.T) {
	config := DefaultConfig()
	photos := map[string]*Photo{
		"photo1": {ID: "photo1", Filename: "photo1.jpg", Albums: []string{"album1"}},
		"photo2": {ID: "photo2", Filename: "photo2.jpg", Albums: []string{"album1"}},
	}
	am := NewAlbumManager(config, photos)

	album := &Album{
		ID:         "album1",
		Name:       "Test Album",
		Type:       AlbumTypeManual,
		PhotoCount: 2,
	}

	am.CreateAlbum(album)

	if err := am.RemovePhotosFromAlbum("album1", []string{"photo1"}); err != nil {
		t.Errorf("RemovePhotosFromAlbum failed: %v", err)
	}

	album, _ = am.GetAlbum("album1")
	if album.PhotoCount != 1 {
		t.Errorf("Expected 1 photo, got %d", album.PhotoCount)
	}
}

func TestAlbumManager_GenerateSmartAlbums(t *testing.T) {
	config := DefaultConfig()
	photos := map[string]*Photo{
		"photo1": {ID: "photo1", People: []string{"Alice"}, Location: "Beijing"},
		"photo2": {ID: "photo2", People: []string{"Alice", "Bob"}, Location: "Beijing"},
		"photo3": {ID: "photo3", People: []string{"Bob"}, EXIF: EXIFData{CameraModel: "iPhone"}},
	}
	am := NewAlbumManager(config, photos)

	if err := am.GenerateSmartAlbums(); err != nil {
		t.Errorf("GenerateSmartAlbums failed: %v", err)
	}

	albums := am.ListAlbums("")
	if len(albums) < 3 {
		t.Errorf("Expected at least 3 smart albums, got %d", len(albums))
	}
}

func TestDedupManager_FindDuplicates(t *testing.T) {
	config := DefaultConfig()
	photos := make(map[string]*Photo)
	dm := NewDedupManager(config, photos)

	photos["photo1"] = &Photo{ID: "photo1", Hash: "abc123", Size: 1024}
	photos["photo2"] = &Photo{ID: "photo2", Hash: "abc123", Size: 1024}
	photos["photo3"] = &Photo{ID: "photo3", Hash: "def456", Size: 512}

	dm.IndexPhoto(photos["photo1"])
	dm.IndexPhoto(photos["photo2"])
	dm.IndexPhoto(photos["photo3"])

	groups := dm.FindDuplicates()

	if len(groups) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(groups))
	}

	if len(groups) > 0 && len(groups[0].Photos) != 2 {
		t.Errorf("Expected 2 photos in group, got %d", len(groups[0].Photos))
	}
}

func TestDedupManager_GetDedupStats(t *testing.T) {
	config := DefaultConfig()
	photos := make(map[string]*Photo)
	dm := NewDedupManager(config, photos)

	photos["photo1"] = &Photo{ID: "photo1", Hash: "abc123", Size: 1024}
	photos["photo2"] = &Photo{ID: "photo2", Hash: "abc123", Size: 1024}

	dm.IndexPhoto(photos["photo1"])
	dm.IndexPhoto(photos["photo2"])

	stats := dm.GetDedupStats()

	if stats.TotalPhotos != 2 {
		t.Errorf("Expected 2 total photos, got %d", stats.TotalPhotos)
	}

	if stats.DuplicateGroups != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", stats.DuplicateGroups)
	}
}

func TestDedupManager_RemoveDuplicates(t *testing.T) {
	config := DefaultConfig()
	photos := make(map[string]*Photo)
	dm := NewDedupManager(config, photos)

	photos["photo1"] = &Photo{ID: "photo1", Hash: "abc123", Size: 1024}
	photos["photo2"] = &Photo{ID: "photo2", Hash: "abc123", Size: 1024}

	dm.IndexPhoto(photos["photo1"])
	dm.IndexPhoto(photos["photo2"])

	groups := dm.FindDuplicates()
	if len(groups) == 0 {
		t.Fatal("No duplicate groups found")
	}

	if err := dm.RemoveDuplicates(groups[0].ID, "photo1"); err != nil {
		t.Errorf("RemoveDuplicates failed: %v", err)
	}

	if !photos["photo2"].Trashed {
		t.Error("Expected photo2 to be trashed")
	}
}

func TestCalculateSimilarity(t *testing.T) {
	tests := []struct {
		hash1    string
		hash2    string
		expected float64
	}{
		{"abcd", "abcd", 1.0},
		{"abcd", "abce", 0.875}, // 2 bits different out of 16
	}

	for _, tt := range tests {
		result := CalculateSimilarity(tt.hash1, tt.hash2)
		if result != tt.expected {
			t.Errorf("CalculateSimilarity(%s, %s) = %f, want %f", tt.hash1, tt.hash2, result, tt.expected)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		slice    []string
		item     string
		expected bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
	}

	for _, tt := range tests {
		result := contains(tt.slice, tt.item)
		if result != tt.expected {
			t.Errorf("contains(%v, %s) = %v, want %v", tt.slice, tt.item, result, tt.expected)
		}
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		slice    []string
		items    []string
		expected bool
	}{
		{[]string{"a", "b"}, []string{"b", "c"}, true},
		{[]string{"a", "b"}, []string{"c", "d"}, false},
	}

	for _, tt := range tests {
		result := containsAny(tt.slice, tt.items)
		if result != tt.expected {
			t.Errorf("containsAny(%v, %v) = %v, want %v", tt.slice, tt.items, result, tt.expected)
		}
	}
}

func TestRemoveFromSlice(t *testing.T) {
	tests := []struct {
		slice    []string
		item     string
		expected []string
	}{
		{[]string{"a", "b", "c"}, "b", []string{"a", "c"}},
		{[]string{"a", "b"}, "c", []string{"a", "b"}},
	}

	for _, tt := range tests {
		result := removeFromSlice(tt.slice, tt.item)
		if len(result) != len(tt.expected) {
			t.Errorf("removeFromSlice length mismatch: got %d, want %d", len(result), len(tt.expected))
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.LibraryPath == "" {
		t.Error("Expected LibraryPath to be set")
	}

	if config.Quality <= 0 || config.Quality > 100 {
		t.Errorf("Invalid Quality: %d", config.Quality)
	}

	if config.SimilarityThreshold <= 0 || config.SimilarityThreshold > 1 {
		t.Errorf("Invalid SimilarityThreshold: %f", config.SimilarityThreshold)
	}
}

func TestAlbumRules_Matches(t *testing.T) {
	config := DefaultConfig()
	photos := make(map[string]*Photo)
	am := NewAlbumManager(config, photos)

	photo := &Photo{
		ID:      "test1",
		TakenAt: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
		Tags:    []string{"vacation", "beach"},
		Rating:  4,
		Trashed: false,
	}

	rules := &AlbumRules{
		Tags:      []string{"vacation"},
		MinRating: 3,
		Operator:  "and",
	}

	if !am.matchesRules(photo, rules) {
		t.Error("Expected photo to match rules")
	}

	photo.Rating = 2
	if am.matchesRules(photo, rules) {
		t.Error("Expected photo not to match rules")
	}
}
