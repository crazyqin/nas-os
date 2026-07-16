package smartalbum

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(mgr.photos) != 0 {
		t.Errorf("expected 0 photos, got %d", len(mgr.photos))
	}
}

func TestAddPhoto(t *testing.T) {
	mgr := NewManager()

	photo := Photo{
		Filename: "test.jpg",
		Path:     "/photos/test.jpg",
		Size:     1024,
		MimeType: "image/jpeg",
		ShotAt:   time.Now(),
	}

	result, err := mgr.AddPhoto(photo)
	if err != nil {
		t.Fatalf("AddPhoto failed: %v", err)
	}
	if result.ID == "" {
		t.Error("photo ID should not be empty")
	}
}

func TestGetPhoto(t *testing.T) {
	mgr := NewManager()

	photo := Photo{
		Filename: "test.jpg",
		Path:     "/photos/test.jpg",
		Size:     1024,
	}
	added, _ := mgr.AddPhoto(photo)

	result, err := mgr.GetPhoto(added.ID)
	if err != nil {
		t.Fatalf("GetPhoto failed: %v", err)
	}
	if result.Filename != "test.jpg" {
		t.Errorf("expected filename 'test.jpg', got '%s'", result.Filename)
	}
}

func TestListPhotos(t *testing.T) {
	mgr := NewManager()

	for i := 0; i < 5; i++ {
		mgr.AddPhoto(Photo{
			Filename: "photo.jpg",
			Path:     "/photos/photo.jpg",
			Size:     1024,
			ShotAt:   time.Now().Add(time.Duration(i) * time.Hour),
		})
	}

	photos := mgr.ListPhotos(3, 0)
	if len(photos) != 3 {
		t.Errorf("expected 3 photos, got %d", len(photos))
	}
}

func TestDeletePhoto(t *testing.T) {
	mgr := NewManager()

	photo := Photo{Filename: "test.jpg", Path: "/photos/test.jpg"}
	added, _ := mgr.AddPhoto(photo)

	err := mgr.DeletePhoto(added.ID)
	if err != nil {
		t.Fatalf("DeletePhoto failed: %v", err)
	}

	_, err = mgr.GetPhoto(added.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestToggleFavorite(t *testing.T) {
	mgr := NewManager()

	photo := Photo{Filename: "test.jpg", Path: "/photos/test.jpg"}
	added, _ := mgr.AddPhoto(photo)

	err := mgr.ToggleFavorite(added.ID)
	if err != nil {
		t.Fatalf("ToggleFavorite failed: %v", err)
	}

	result, _ := mgr.GetPhoto(added.ID)
	if !result.IsFavorite {
		t.Error("expected photo to be favorite")
	}
}

func TestFaceManagement(t *testing.T) {
	mgr := NewManager()

	photo := Photo{Filename: "face.jpg", Path: "/photos/face.jpg"}
	added, _ := mgr.AddPhoto(photo)

	face, err := mgr.RegisterFace("John", added.ID, []float64{0.1, 0.2, 0.3})
	if err != nil {
		t.Fatalf("RegisterFace failed: %v", err)
	}
	if face.Name != "John" {
		t.Errorf("expected face name 'John', got '%s'", face.Name)
	}
}

func TestAlbumManagement(t *testing.T) {
	mgr := NewManager()

	album, err := mgr.CreateAlbum("Vacation", AlbumTypeManual)
	if err != nil {
		t.Fatalf("CreateAlbum failed: %v", err)
	}

	photo := Photo{Filename: "beach.jpg", Path: "/photos/beach.jpg"}
	added, _ := mgr.AddPhoto(photo)

	err = mgr.AddPhotoToAlbum(album.ID, added.ID)
	if err != nil {
		t.Fatalf("AddPhotoToAlbum failed: %v", err)
	}
}

func TestSemanticSearch(t *testing.T) {
	mgr := NewManager()

	// 添加带嵌入向量的照片
	photo1 := Photo{
		Filename:  "sunset.jpg",
		Path:      "/photos/sunset.jpg",
		Embedding: []float64{0.9, 0.1, 0.0},
	}
	mgr.AddPhoto(photo1)

	photo2 := Photo{
		Filename:  "beach.jpg",
		Path:      "/photos/beach.jpg",
		Embedding: []float64{0.8, 0.2, 0.1},
	}
	mgr.AddPhoto(photo2)

	// 语义搜索
	queryEmbedding := []float64{0.85, 0.15, 0.05}
	results := mgr.SemanticSearch(queryEmbedding, 5, 0.5)
	if len(results) == 0 {
		t.Error("expected search results")
	}
}

func TestFindSimilarPhotos(t *testing.T) {
	mgr := NewManager()

	photo1, _ := mgr.AddPhoto(Photo{
		Filename:  "sunset1.jpg",
		Path:      "/photos/sunset1.jpg",
		Embedding: []float64{0.9, 0.1, 0.0},
	})

	mgr.AddPhoto(Photo{
		Filename:  "sunset2.jpg",
		Path:      "/photos/sunset2.jpg",
		Embedding: []float64{0.85, 0.15, 0.05},
	})

	mgr.AddPhoto(Photo{
		Filename:  "document.jpg",
		Path:      "/photos/document.jpg",
		Embedding: []float64{0.1, 0.1, 0.9},
	})

	results, err := mgr.FindSimilarPhotos(photo1.ID, 2)
	if err != nil {
		t.Fatalf("FindSimilarPhotos failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected similar photos")
	}
}

func TestMapClusters(t *testing.T) {
	mgr := NewManager()

	// 添加带 GPS 的照片
	photo1 := Photo{
		Filename: "location1.jpg",
		GPS: &GPSInfo{
			Latitude:  39.9042,
			Longitude: 116.4074,
			City:      "北京",
		},
	}
	mgr.AddPhoto(photo1)

	photo2 := Photo{
		Filename: "location2.jpg",
		GPS: &GPSInfo{
			Latitude:  39.9050,
			Longitude: 116.4080,
			City:      "北京",
		},
	}
	mgr.AddPhoto(photo2)

	clusters := mgr.GetMapClusters(nil, 10)
	if len(clusters) == 0 {
		t.Error("expected map clusters")
	}
}

func TestGetPhotosByLocation(t *testing.T) {
	mgr := NewManager()

	photo := Photo{
		Filename: "beijing.jpg",
		GPS: &GPSInfo{
			Latitude:  39.9042,
			Longitude: 116.4074,
			City:      "北京",
		},
	}
	mgr.AddPhoto(photo)

	results := mgr.GetPhotosByLocation("北京", 10)
	if len(results) == 0 {
		t.Error("expected photos in Beijing")
	}
}

func TestAutoTag(t *testing.T) {
	mgr := NewManager()

	photo := Photo{
		Filename: "sunset.jpg",
		ShotAt:   time.Date(2024, 6, 15, 18, 30, 0, 0, time.Local),
		GPS: &GPSInfo{
			City: "上海",
		},
	}
	added, _ := mgr.AddPhoto(photo)

	tags, err := mgr.AutoTag(added.ID)
	if err != nil {
		t.Fatalf("AutoTag failed: %v", err)
	}

	// 应该包含时间相关的标签
	hasTimeTag := false
	for _, tag := range tags {
		if tag == "傍晚" || tag == "日落" {
			hasTimeTag = true
			break
		}
	}
	if !hasTimeTag {
		t.Error("expected time-related tag")
	}
}

func TestDetectDuplicates(t *testing.T) {
	mgr := NewManager()

	mgr.AddPhoto(Photo{
		Filename: "photo.jpg",
		Hash:     "abc123",
		Size:     1024,
		Score:    80,
	})

	photo2, _ := mgr.AddPhoto(Photo{
		Filename: "photo_copy.jpg",
		Hash:     "abc123",
		Size:     1024,
		Score:    90,
	})

	groups := mgr.DetectDuplicates()
	if len(groups) == 0 {
		t.Error("expected duplicate groups")
	}
	if groups[0].BestID != photo2.ID {
		t.Error("expected higher score photo to be best")
	}
}

func TestGenerateTimeline(t *testing.T) {
	mgr := NewManager()

	mgr.AddPhoto(Photo{
		Filename: "photo1.jpg",
		ShotAt:   time.Date(2024, 6, 15, 10, 0, 0, 0, time.Local),
		GPS:      &GPSInfo{Address: "天安门"},
	})

	mgr.AddPhoto(Photo{
		Filename: "photo2.jpg",
		ShotAt:   time.Date(2024, 6, 15, 14, 0, 0, 0, time.Local),
		GPS:      &GPSInfo{Address: "故宫"},
	})

	timeline := mgr.GenerateTimeline()
	if len(timeline) == 0 {
		t.Error("expected timeline entries")
	}
}

func TestGetStats(t *testing.T) {
	mgr := NewManager()

	mgr.AddPhoto(Photo{
		Filename:   "photo.jpg",
		Size:       1024,
		IsFavorite: true,
		Scene:      "landscape",
		Embedding:  []float64{0.1, 0.2},
	})

	stats := mgr.GetStats()
	if stats["totalPhotos"] != 1 {
		t.Errorf("expected 1 photo, got %v", stats["totalPhotos"])
	}
	if stats["favorites"] != 1 {
		t.Errorf("expected 1 favorite, got %v", stats["favorites"])
	}
}

func TestBatchAddEmbeddings(t *testing.T) {
	mgr := NewManager()

	photo1, _ := mgr.AddPhoto(Photo{Filename: "photo1.jpg"})
	photo2, _ := mgr.AddPhoto(Photo{Filename: "photo2.jpg"})

	embeddings := map[string][]float64{
		photo1.ID: {0.1, 0.2, 0.3},
		photo2.ID: {0.4, 0.5, 0.6},
	}

	count := mgr.BatchAddEmbeddings(embeddings)
	if count != 2 {
		t.Errorf("expected 2 embeddings added, got %d", count)
	}
}

func TestSmartAlbum(t *testing.T) {
	mgr := NewManager()

	// 添加照片
	photo1 := Photo{
		Filename:   "sunset.jpg",
		Scene:      "landscape",
		IsFavorite: true,
		Score:      90,
	}
	mgr.AddPhoto(photo1)

	photo2 := Photo{
		Filename: "portrait.jpg",
		Scene:    "portrait",
		Score:    80,
	}
	mgr.AddPhoto(photo2)

	// 创建智能相册
	criteria := AlbumCriteria{
		Scenes:    []SceneCategory{SceneLandscape},
		MinScore:  85,
		Favorites: true,
	}

	album, err := mgr.CreateSmartAlbum("Best Landscapes", criteria)
	if err != nil {
		t.Fatalf("CreateSmartAlbum failed: %v", err)
	}

	if album.PhotoCount != 1 {
		t.Errorf("expected 1 photo in album, got %d", album.PhotoCount)
	}
}
