package photos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== Photo 结构测试 ==========

func TestPhoto_Structure(t *testing.T) {
	now := time.Now()
	photo := Photo{
		ID:            "photo-1",
		Filename:      "IMG_0001.jpg",
		Path:          "/photos/2024/IMG_0001.jpg",
		AlbumID:       "album-1",
		UserID:        "user-1",
		Size:          1024 * 1024 * 2, // 2MB
		MimeType:      "image/jpeg",
		Width:         1920,
		Height:        1080,
		Duration:      0,
		TakenAt:       now,
		UploadedAt:    now,
		ModifiedAt:    now,
		ThumbnailPath: "/thumbnails/photo-1.jpg",
		IsFavorite:    true,
		IsHidden:      false,
		Tags:          []string{"vacation", "beach"},
	}

	assert.Equal(t, "photo-1", photo.ID)
	assert.Equal(t, "IMG_0001.jpg", photo.Filename)
	assert.Equal(t, "image/jpeg", photo.MimeType)
	assert.True(t, photo.IsFavorite)
	assert.False(t, photo.IsHidden)
	assert.Len(t, photo.Tags, 2)
}

func TestPhoto_Video(t *testing.T) {
	photo := Photo{
		ID:       "video-1",
		Filename: "VID_0001.mp4",
		MimeType: "video/mp4",
		Duration: 60, // 60秒
	}

	assert.Equal(t, "video/mp4", photo.MimeType)
	assert.Equal(t, 60, photo.Duration)
}

func TestPhoto_WithEXIF(t *testing.T) {
	photo := Photo{
		ID:       "photo-1",
		Filename: "DSC_0001.jpg",
		EXIF: &EXIFData{
			Make:         "Nikon",
			Model:        "D850",
			FNumber:      2.8,
			ISO:          400,
			FocalLength:  50.0,
			GPSLatitude:  31.2304,
			GPSLongitude: 121.4737,
		},
	}

	assert.NotNil(t, photo.EXIF)
	assert.Equal(t, "Nikon", photo.EXIF.Make)
	assert.Equal(t, "D850", photo.EXIF.Model)
	assert.Equal(t, 2.8, photo.EXIF.FNumber)
}

// ========== EXIFData 结构测试 ==========

func TestEXIFData_Structure(t *testing.T) {
	exif := EXIFData{
		Make:            "Canon",
		Model:           "EOS R5",
		LensMake:        "Canon",
		LensModel:       "RF 50mm f/1.2L USM",
		ExposureTime:    "1/500",
		FNumber:         1.2,
		ISO:             100,
		FocalLength:     50.0,
		FocalLength35mm: 50,
		Flash:           false,
		WhiteBalance:    "Auto",
		Orientation:     1,
	}

	assert.Equal(t, "Canon", exif.Make)
	assert.Equal(t, "EOS R5", exif.Model)
	assert.Equal(t, 1.2, exif.FNumber)
	assert.Equal(t, 100, exif.ISO)
	assert.False(t, exif.Flash)
}

// ========== FaceInfo 结构测试 ==========

func TestFaceInfo_Structure(t *testing.T) {
	face := FaceInfo{
		ID:   "face-1",
		Name: "John Doe",
		Bounds: Rectangle{
			X: 100, Y: 150, Width: 80, Height: 100,
		},
		Confidence: 0.95,
		Age:        30,
		Gender:     "male",
		Emotion:    "happy",
	}

	assert.Equal(t, "face-1", face.ID)
	assert.Equal(t, "John Doe", face.Name)
	assert.InDelta(t, 0.95, float64(face.Confidence), 0.001)
	assert.Equal(t, 30, face.Age)
	assert.Equal(t, "male", face.Gender)
}

// ========== Rectangle 结构测试 ==========

func TestRectangle_Structure(t *testing.T) {
	rect := Rectangle{
		X:      10,
		Y:      20,
		Width:  100,
		Height: 200,
	}

	assert.Equal(t, 10, rect.X)
	assert.Equal(t, 20, rect.Y)
	assert.Equal(t, 100, rect.Width)
	assert.Equal(t, 200, rect.Height)
}

// ========== LocationInfo 结构测试 ==========

func TestLocationInfo_Structure(t *testing.T) {
	loc := LocationInfo{
		Latitude:  31.2304,
		Longitude: 121.4737,
		Altitude:  10.0,
		City:      "Shanghai",
		Country:   "China",
		Location:  "Bund, Shanghai",
	}

	assert.Equal(t, 31.2304, loc.Latitude)
	assert.Equal(t, 121.4737, loc.Longitude)
	assert.Equal(t, "Shanghai", loc.City)
}

// ========== DeviceInfo 结构测试 ==========

func TestDeviceInfo_Structure(t *testing.T) {
	device := DeviceInfo{
		Brand: "Apple",
		Model: "iPhone 15 Pro",
		OS:    "iOS 17.0",
		App:   "Camera",
	}

	assert.Equal(t, "Apple", device.Brand)
	assert.Equal(t, "iPhone 15 Pro", device.Model)
}

// ========== ShareInfo 结构测试 ==========

func TestShareInfo_Structure(t *testing.T) {
	share := ShareInfo{
		ShareID:       "share-1",
		ShareURL:      "https://photos.example.com/s/share-1",
		Password:      "secret",
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		ViewCount:     10,
		DownloadCount: 2,
		AllowDownload: true,
	}

	assert.Equal(t, "share-1", share.ShareID)
	assert.Equal(t, 10, share.ViewCount)
	assert.True(t, share.AllowDownload)
}

// ========== EditOperation 结构测试 ==========

func TestEditOperation_Structure(t *testing.T) {
	edit := EditOperation{
		Type: "crop",
		Params: map[string]interface{}{
			"x":      100,
			"y":      100,
			"width":  800,
			"height": 600,
		},
		Timestamp: time.Now(),
	}

	assert.Equal(t, "crop", edit.Type)
	assert.NotNil(t, edit.Params)
}

// ========== Album 结构测试 ==========

func TestAlbum_Structure(t *testing.T) {
	now := time.Now()
	album := Album{
		ID:           "album-1",
		Name:         "Vacation 2024",
		Description:  "Summer vacation photos",
		UserID:       "user-1",
		CoverPhotoID: "photo-1",
		PhotoCount:   50,
		IsShared:     true,
		IsFavorite:   true,
		CreatedAt:    now,
		UpdatedAt:    now,
		Tags:         []string{"vacation", "summer"},
		Location:     "Hawaii",
	}

	assert.Equal(t, "album-1", album.ID)
	assert.Equal(t, "Vacation 2024", album.Name)
	assert.Equal(t, 50, album.PhotoCount)
	assert.True(t, album.IsShared)
}

func TestAlbum_WithParent(t *testing.T) {
	album := Album{
		ID:       "album-2",
		Name:     "Day 1",
		ParentID: "album-1",
	}

	assert.Equal(t, "album-1", album.ParentID)
}

// ========== ShareTarget 结构测试 ==========

func TestShareTarget_Structure(t *testing.T) {
	target := ShareTarget{
		UserID:     "user-2",
		Username:   "john",
		Permission: "view",
		SharedAt:   time.Now(),
	}

	assert.Equal(t, "user-2", target.UserID)
	assert.Equal(t, "view", target.Permission)
}

// ========== ThumbnailConfig 测试 ==========

func TestDefaultThumbnailConfig(t *testing.T) {
	config := DefaultThumbnailConfig

	assert.Equal(t, 128, config.SmallSize)
	assert.Equal(t, 512, config.MediumSize)
	assert.Equal(t, 1024, config.LargeSize)
	assert.Equal(t, 2048, config.OriginalMax)
	assert.Equal(t, 85, config.Quality)
}

func TestThumbnailConfig_Custom(t *testing.T) {
	config := ThumbnailConfig{
		SmallSize:   64,
		MediumSize:  256,
		LargeSize:   512,
		OriginalMax: 1024,
		Quality:     90,
	}

	assert.Equal(t, 64, config.SmallSize)
	assert.Equal(t, 90, config.Quality)
}

// ========== UploadSession 结构测试 ==========

func TestUploadSession_Structure(t *testing.T) {
	now := time.Now()
	session := UploadSession{
		SessionID:      "session-1",
		UserID:         "user-1",
		Filename:       "large_video.mp4",
		TotalSize:      100 * 1024 * 1024, // 100MB
		UploadedSize:   50 * 1024 * 1024,  // 50MB uploaded
		ChunkSize:      5 * 1024 * 1024,   // 5MB chunks
		TotalChunks:    20,
		UploadedChunks: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		CreatedAt:      now,
		ExpiresAt:      now.Add(24 * time.Hour),
		TempPath:       "/tmp/uploads/session-1",
	}

	assert.Equal(t, "session-1", session.SessionID)
	assert.Equal(t, int64(100*1024*1024), session.TotalSize)
	assert.Equal(t, 20, session.TotalChunks)
	assert.Len(t, session.UploadedChunks, 10)
}

// ========== PhotoQuery 结构测试 ==========

func TestPhotoQuery_Structure(t *testing.T) {
	now := time.Now()
	favorite := true

	query := PhotoQuery{
		AlbumID:    "album-1",
		UserID:     "user-1",
		StartDate:  now.Add(-30 * 24 * time.Hour),
		EndDate:    now,
		Tags:       []string{"vacation"},
		IsFavorite: &favorite,
		MimeType:   "image/jpeg",
		SortBy:     "takenAt",
		SortOrder:  "desc",
		Limit:      20,
		Offset:     0,
	}

	assert.Equal(t, "album-1", query.AlbumID)
	assert.Equal(t, "image/jpeg", query.MimeType)
	assert.True(t, *query.IsFavorite)
	assert.Equal(t, 20, query.Limit)
}

func TestPhotoQuery_Empty(t *testing.T) {
	query := PhotoQuery{}

	assert.Empty(t, query.AlbumID)
	assert.Empty(t, query.Tags)
	assert.Zero(t, query.Limit)
}

// ========== TimelineGroup 结构测试 ==========

func TestTimelineGroup_Structure(t *testing.T) {
	group := TimelineGroup{
		Period: "2024-03",
		Photos: []*Photo{
			{ID: "photo-1"},
			{ID: "photo-2"},
		},
		Count:    2,
		Location: "Tokyo",
	}

	assert.Equal(t, "2024-03", group.Period)
	assert.Len(t, group.Photos, 2)
	assert.Equal(t, 2, group.Count)
}

// ========== Person 结构测试 ==========

func TestPerson_Structure(t *testing.T) {
	now := time.Now()
	person := Person{
		ID:           "person-1",
		Name:         "Alice",
		PhotoCount:   25,
		CoverPhotoID: "photo-1",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	assert.Equal(t, "person-1", person.ID)
	assert.Equal(t, "Alice", person.Name)
	assert.Equal(t, 25, person.PhotoCount)
}

// ========== AIClassification 结构测试 ==========

func TestAIClassification_Structure(t *testing.T) {
	ai := AIClassification{
		PhotoID: "photo-1",
		Faces: []FaceInfo{
			{ID: "face-1", Name: "Alice"},
		},
		Objects:      []string{"cat", "chair", "window"},
		Scene:        "indoor",
		Colors:       []string{"#FF5733", "#33FF57"},
		IsNSFW:       false,
		Confidence:   0.92,
		QualityScore: 85.5,
		AutoTags:     []string{"portrait", "indoor"},
	}

	assert.Equal(t, "photo-1", ai.PhotoID)
	assert.Len(t, ai.Objects, 3)
	assert.False(t, ai.IsNSFW)
	assert.Equal(t, float32(85.5), ai.QualityScore)
}

// ========== QualityMetrics 结构测试 ==========

func TestQualityMetrics_Structure(t *testing.T) {
	metrics := QualityMetrics{
		Brightness:   128.0,
		Contrast:     0.8,
		Sharpness:    0.75,
		Colorfulness: 0.65,
		Composition:  0.85,
		OverallScore: 78.5,
	}

	assert.Equal(t, 128.0, metrics.Brightness)
	assert.Equal(t, float32(78.5), metrics.OverallScore)
}

// ========== 边界测试 ==========

func TestPhoto_Empty(t *testing.T) {
	photo := Photo{}

	assert.Empty(t, photo.ID)
	assert.Empty(t, photo.Filename)
	assert.Zero(t, photo.Size)
	assert.Nil(t, photo.EXIF)
	assert.Nil(t, photo.Tags)
}

func TestAlbum_Empty(t *testing.T) {
	album := Album{}

	assert.Empty(t, album.ID)
	assert.Zero(t, album.PhotoCount)
	assert.False(t, album.IsShared)
}

// ========== 基准测试 ==========

func BenchmarkPhoto_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Photo{
			ID:         "photo-1",
			Filename:   "test.jpg",
			MimeType:   "image/jpeg",
			UploadedAt: time.Now(),
		}
	}
}

func BenchmarkAlbum_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Album{
			ID:        "album-1",
			Name:      "Test Album",
			UserID:    "user-1",
			CreatedAt: time.Now(),
		}
	}
}

func BenchmarkPhotoQuery_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PhotoQuery{
			Limit:     20,
			SortBy:    "takenAt",
			SortOrder: "desc",
		}
	}
}
