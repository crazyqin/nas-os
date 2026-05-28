package smartalbum

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestAddPhoto(t *testing.T) {
	m := NewManager()

	photo, err := m.AddPhoto(Photo{
		Filename: "test.jpg",
		Path:     "/photos/test.jpg",
		Size:     1024000,
		MimeType: "image/jpeg",
		Width:    1920,
		Height:   1080,
		ShotAt:   time.Now(),
		Tags:     []string{"vacation", "beach"},
		Scene:    string(SceneLandscape),
		Score:    85.5,
	})
	if err != nil {
		t.Fatalf("add photo failed: %v", err)
	}
	if photo.ID == "" {
		t.Error("expected photo ID")
	}

	// 空文件名
	_, err = m.AddPhoto(Photo{})
	if err == nil {
		t.Error("expected error for empty filename")
	}
}

func TestGetPhoto(t *testing.T) {
	m := NewManager()

	photo, _ := m.AddPhoto(Photo{Filename: "test.jpg", ShotAt: time.Now()})

	fetched, err := m.GetPhoto(photo.ID)
	if err != nil {
		t.Fatalf("get photo failed: %v", err)
	}
	if fetched.Filename != "test.jpg" {
		t.Errorf("expected 'test.jpg', got '%s'", fetched.Filename)
	}

	_, err = m.GetPhoto("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent photo")
	}
}

func TestListPhotos(t *testing.T) {
	m := NewManager()

	for i := 0; i < 5; i++ {
		m.AddPhoto(Photo{
			Filename: "photo.jpg",
			ShotAt:   time.Now().Add(time.Duration(i) * time.Hour),
		})
	}

	photos := m.ListPhotos(3, 0)
	if len(photos) != 3 {
		t.Errorf("expected 3, got %d", len(photos))
	}

	photos = m.ListPhotos(10, 3)
	if len(photos) != 2 {
		t.Errorf("expected 2, got %d", len(photos))
	}
}

func TestDeletePhoto(t *testing.T) {
	m := NewManager()

	photo, _ := m.AddPhoto(Photo{Filename: "test.jpg", Tags: []string{"tag1"}, ShotAt: time.Now()})

	err := m.DeletePhoto(photo.ID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = m.GetPhoto(photo.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestToggleFavorite(t *testing.T) {
	m := NewManager()

	photo, _ := m.AddPhoto(Photo{Filename: "test.jpg", ShotAt: time.Now()})

	m.ToggleFavorite(photo.ID)
	fetched, _ := m.GetPhoto(photo.ID)
	if !fetched.IsFavorite {
		t.Error("expected favorite")
	}

	m.ToggleFavorite(photo.ID)
	fetched, _ = m.GetPhoto(photo.ID)
	if fetched.IsFavorite {
		t.Error("expected not favorite")
	}
}

func TestFaceManagement(t *testing.T) {
	m := NewManager()

	photo1, _ := m.AddPhoto(Photo{Filename: "p1.jpg", ShotAt: time.Now()})
	photo2, _ := m.AddPhoto(Photo{Filename: "p2.jpg", ShotAt: time.Now()})

	// 注册人脸
	face, err := m.RegisterFace("张三", photo1.ID, []float64{0.1, 0.2, 0.3})
	if err != nil {
		t.Fatalf("register face failed: %v", err)
	}
	if face.Name != "张三" {
		t.Errorf("expected '张三', got '%s'", face.Name)
	}

	// 关联到另一张照片
	err = m.LinkFaceToPhoto(face.ID, photo2.ID)
	if err != nil {
		t.Fatalf("link face failed: %v", err)
	}

	fetched, _ := m.GetFace(face.ID)
	if fetched.PhotoCount != 2 {
		t.Errorf("expected 2 photos, got %d", fetched.PhotoCount)
	}

	// 列出人脸
	faces := m.ListFaces()
	if len(faces) != 1 {
		t.Errorf("expected 1 face, got %d", len(faces))
	}
}

func TestAlbumManagement(t *testing.T) {
	m := NewManager()

	photo, _ := m.AddPhoto(Photo{Filename: "test.jpg", ShotAt: time.Now()})

	// 创建相册
	album, err := m.CreateAlbum("我的相册", AlbumTypeManual)
	if err != nil {
		t.Fatalf("create album failed: %v", err)
	}
	if album.Name != "我的相册" {
		t.Errorf("expected '我的相册', got '%s'", album.Name)
	}

	// 添加照片
	err = m.AddPhotoToAlbum(album.ID, photo.ID)
	if err != nil {
		t.Fatalf("add to album failed: %v", err)
	}

	fetched, _ := m.GetAlbum(album.ID)
	if fetched.PhotoCount != 1 {
		t.Errorf("expected 1 photo, got %d", fetched.PhotoCount)
	}
	if fetched.CoverID != photo.ID {
		t.Error("expected cover to be set")
	}

	// 列出相册
	albums := m.ListAlbums()
	if len(albums) != 1 {
		t.Errorf("expected 1 album, got %d", len(albums))
	}
}

func TestSmartAlbum(t *testing.T) {
	m := NewManager()

	m.AddPhoto(Photo{
		Filename: "beach.jpg",
		Tags:     []string{"vacation", "beach"},
		Scene:    string(SceneLandscape),
		Score:    90,
		ShotAt:   time.Now(),
	})
	m.AddPhoto(Photo{
		Filename: "food.jpg",
		Tags:     []string{"food"},
		Scene:    string(SceneFood),
		Score:    70,
		ShotAt:   time.Now(),
	})
	m.AddPhoto(Photo{
		Filename: "sunset.jpg",
		Tags:     []string{"vacation", "sunset"},
		Scene:    string(SceneLandscape),
		Score:    95,
		ShotAt:   time.Now(),
	})

	// 按标签创建智能相册
	album, err := m.CreateSmartAlbum("度假精选", AlbumCriteria{
		Tags: []string{"vacation"},
	})
	if err != nil {
		t.Fatalf("create smart album failed: %v", err)
	}
	if album.PhotoCount != 2 {
		t.Errorf("expected 2 photos, got %d", album.PhotoCount)
	}

	// 按场景创建
	album2, _ := m.CreateSmartAlbum("风景照", AlbumCriteria{
		Scenes: []SceneCategory{SceneLandscape},
	})
	if album2.PhotoCount != 2 {
		t.Errorf("expected 2 photos, got %d", album2.PhotoCount)
	}

	// 按评分创建
	album3, _ := m.CreateSmartAlbum("高分照片", AlbumCriteria{
		MinScore: 80,
	})
	if album3.PhotoCount != 2 {
		t.Errorf("expected 2 photos, got %d", album3.PhotoCount)
	}
}

func TestTimeline(t *testing.T) {
	m := NewManager()

	m.AddPhoto(Photo{Filename: "1.jpg", ShotAt: time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)})
	m.AddPhoto(Photo{Filename: "2.jpg", ShotAt: time.Date(2025, 6, 1, 14, 0, 0, 0, time.UTC)})
	m.AddPhoto(Photo{Filename: "3.jpg", ShotAt: time.Date(2025, 6, 2, 10, 0, 0, 0, time.UTC)})

	timeline := m.GenerateTimeline()
	if len(timeline) != 2 {
		t.Errorf("expected 2 entries, got %d", len(timeline))
	}
	if timeline[0].Count != 1 { // 6月2日
		t.Errorf("expected 1, got %d", timeline[0].Count)
	}
	if timeline[1].Count != 2 { // 6月1日
		t.Errorf("expected 2, got %d", timeline[1].Count)
	}
}

func TestDetectDuplicates(t *testing.T) {
	m := NewManager()

	m.AddPhoto(Photo{Filename: "a.jpg", Hash: "abc123", Size: 1000, Score: 80, ShotAt: time.Now()})
	m.AddPhoto(Photo{Filename: "b.jpg", Hash: "abc123", Size: 1000, Score: 90, ShotAt: time.Now()})
	m.AddPhoto(Photo{Filename: "c.jpg", Hash: "def456", Size: 500, ShotAt: time.Now()})

	dups := m.DetectDuplicates()
	if len(dups) != 1 {
		t.Errorf("expected 1 duplicate group, got %d", len(dups))
	}
	if dups[0].Count != 2 {
		t.Errorf("expected 2, got %d", dups[0].Count)
	}
	if dups[0].BestID == "" {
		t.Error("expected best ID")
	}
}

func TestSearchPhotos(t *testing.T) {
	m := NewManager()

	m.AddPhoto(Photo{Filename: "beach-sunset.jpg", Tags: []string{"beach"}, Scene: string(SceneLandscape), Score: 90, ShotAt: time.Now()})
	m.AddPhoto(Photo{Filename: "cat.jpg", Tags: []string{"pet"}, Scene: string(SceneAnimal), Score: 80, ShotAt: time.Now()})

	// 按文件名搜索
	results := m.SearchPhotos("beach", nil, "")
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}

	// 按标签搜索
	results = m.SearchPhotos("", []string{"pet"}, "")
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}

	// 按场景搜索
	results = m.SearchPhotos("", nil, SceneLandscape)
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestStats(t *testing.T) {
	m := NewManager()

	m.AddPhoto(Photo{Filename: "1.jpg", Size: 1000, Tags: []string{"a"}, ShotAt: time.Now()})
	m.AddPhoto(Photo{Filename: "2.jpg", Size: 2000, ShotAt: time.Now(), IsFavorite: true})

	stats := m.GetStats()
	if stats["totalPhotos"] != 2 {
		t.Errorf("expected 2, got %v", stats["totalPhotos"])
	}
	if stats["favorites"] != 1 {
		t.Errorf("expected 1, got %v", stats["favorites"])
	}
	if stats["totalSize"] != int64(3000) {
		t.Errorf("expected 3000, got %v", stats["totalSize"])
	}
}
