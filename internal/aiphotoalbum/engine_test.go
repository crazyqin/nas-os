package aiphotoalbum

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPhotoEngine(t *testing.T) {
	engine := NewPhotoEngine(nil)
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.photos)
	assert.NotNil(t, engine.persons)
	assert.NotNil(t, engine.albums)
	assert.NotNil(t, engine.faceIndex)
	assert.NotNil(t, engine.tagIndex)
	assert.NotNil(t, engine.embeddingIdx)
}

func TestPhotoEngine_StartStop(t *testing.T) {
	engine := NewPhotoEngine(nil)

	err := engine.Start()
	require.NoError(t, err)
	assert.True(t, engine.running)

	err = engine.Start()
	require.NoError(t, err)

	err = engine.Stop()
	require.NoError(t, err)
	assert.False(t, engine.running)

	err = engine.Stop()
	require.NoError(t, err)
}

func TestPhotoEngine_AddPhoto(t *testing.T) {
	engine := NewPhotoEngine(nil)

	photo := &Photo{
		ID:       "photo-1",
		FileName: "test.jpg",
		FileSize: 1024,
		Tags:     []string{"vacation", "beach"},
		ShotAt:   time.Now(),
	}

	err := engine.AddPhoto(photo)
	require.NoError(t, err)
	assert.Equal(t, int64(1), engine.stats.TotalPhotos)

	// 添加无ID照片
	invalidPhoto := &Photo{FileName: "invalid.jpg"}
	err = engine.AddPhoto(invalidPhoto)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidPhotoID, err)
}

func TestPhotoEngine_RemovePhoto(t *testing.T) {
	engine := NewPhotoEngine(nil)

	photo := &Photo{
		ID:       "photo-1",
		FileName: "test.jpg",
		Tags:     []string{"vacation"},
		Faces: []FaceDetection{
			{PersonID: "person-1"},
		},
	}

	engine.AddPhoto(photo)

	err := engine.RemovePhoto("photo-1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), engine.stats.TotalPhotos)

	err = engine.RemovePhoto("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrPhotoNotFound, err)
}

func TestPhotoEngine_TextSearch(t *testing.T) {
	engine := NewPhotoEngine(nil)
	engine.Start()

	// 添加测试照片
	photos := []*Photo{
		{
			ID:       "photo-1",
			FileName: "beach_sunset.jpg",
			Tags:     []string{"beach", "sunset", "vacation"},
			ShotAt:   time.Now().Add(-24 * time.Hour),
		},
		{
			ID:       "photo-2",
			FileName: "mountain_view.jpg",
			Tags:     []string{"mountain", "landscape"},
			ShotAt:   time.Now().Add(-48 * time.Hour),
		},
		{
			ID:       "photo-3",
			FileName: "beach_party.jpg",
			Tags:     []string{"beach", "party", "friends"},
			ShotAt:   time.Now().Add(-12 * time.Hour),
		},
	}

	for _, p := range photos {
		engine.AddPhoto(p)
	}

	// 测试文本搜索
	result, err := engine.TextSearch(&TextSearchQuery{
		Text:  "beach",
		Limit: 10,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.TotalCount)

	// 测试无文本查询
	result, err = engine.TextSearch(&TextSearchQuery{
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalCount)

	// 测试禁用文本搜索
	engine.config.TextSearchEnabled = false
	_, err = engine.TextSearch(&TextSearchQuery{Text: "test"})
	assert.Error(t, err)
	assert.Equal(t, ErrTextSearchDisabled, err)
}

func TestPhotoEngine_GetPhotosByPerson(t *testing.T) {
	engine := NewPhotoEngine(nil)

	person := &Person{
		ID:   "person-1",
		Name: "Alice",
	}
	engine.RegisterPerson(person)

	photos := []*Photo{
		{
			ID:   "photo-1",
			Faces: []FaceDetection{
				{PersonID: "person-1"},
			},
		},
		{
			ID:   "photo-2",
			Faces: []FaceDetection{
				{PersonID: "person-1"},
			},
		},
		{
			ID:   "photo-3",
			Faces: []FaceDetection{
				{PersonID: "person-2"},
			},
		},
	}

	for _, p := range photos {
		engine.AddPhoto(p)
	}

	result, err := engine.GetPhotosByPerson("person-1")
	require.NoError(t, err)
	assert.Len(t, result, 2)

	_, err = engine.GetPhotosByPerson("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrPersonNotFound, err)
}

func TestPhotoEngine_GetPhotosByTag(t *testing.T) {
	engine := NewPhotoEngine(nil)

	photos := []*Photo{
		{ID: "photo-1", Tags: []string{"vacation", "beach"}},
		{ID: "photo-2", Tags: []string{"vacation", "mountain"}},
		{ID: "photo-3", Tags: []string{"party"}},
	}

	for _, p := range photos {
		engine.AddPhoto(p)
	}

	result, err := engine.GetPhotosByTag("vacation")
	require.NoError(t, err)
	assert.Len(t, result, 2)

	_, err = engine.GetPhotosByTag("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrTagNotFound, err)
}

func TestPhotoEngine_CreateAlbum(t *testing.T) {
	engine := NewPhotoEngine(nil)

	album := &Album{
		ID:   "album-1",
		Name: "Summer Vacation",
	}

	err := engine.CreateAlbum(album)
	require.NoError(t, err)
	assert.Equal(t, int64(1), engine.stats.TotalAlbums)

	invalidAlbum := &Album{Name: "Invalid"}
	err = engine.CreateAlbum(invalidAlbum)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidAlbumID, err)
}

func TestPhotoEngine_AddPhotosToAlbum(t *testing.T) {
	engine := NewPhotoEngine(nil)

	album := &Album{
		ID:   "album-1",
		Name: "Test Album",
	}
	engine.CreateAlbum(album)

	photos := []*Photo{
		{ID: "photo-1"},
		{ID: "photo-2"},
	}
	for _, p := range photos {
		engine.AddPhoto(p)
	}

	err := engine.AddPhotosToAlbum("album-1", []string{"photo-1", "photo-2"})
	require.NoError(t, err)
	assert.Equal(t, 2, album.PhotoCount)

	// 测试重复添加
	err = engine.AddPhotosToAlbum("album-1", []string{"photo-1"})
	require.NoError(t, err)
	assert.Equal(t, 2, album.PhotoCount)

	err = engine.AddPhotosToAlbum("non-existent", []string{"photo-1"})
	assert.Error(t, err)
	assert.Equal(t, ErrAlbumNotFound, err)
}

func TestPhotoEngine_RegisterPerson(t *testing.T) {
	engine := NewPhotoEngine(nil)

	person := &Person{
		ID:   "person-1",
		Name: "Bob",
	}

	err := engine.RegisterPerson(person)
	require.NoError(t, err)
	assert.Equal(t, int64(1), engine.stats.TotalPersons)

	invalidPerson := &Person{Name: "Invalid"}
	err = engine.RegisterPerson(invalidPerson)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidPersonID, err)
}

func TestPhotoEngine_RecognizeFace(t *testing.T) {
	engine := NewPhotoEngine(nil)

	person := &Person{
		ID:        "person-1",
		Name:      "Alice",
		Embedding: []float32{0.1, 0.2, 0.3, 0.4, 0.5},
	}
	engine.RegisterPerson(person)

	// 测试识别
	embedding := []float32{0.11, 0.21, 0.31, 0.41, 0.51}
	result, score, err := engine.RecognizeFace(embedding)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, score, 0.0)

	// 测试禁用人脸识别
	engine.config.FaceDetectionEnabled = false
	_, _, err = engine.RecognizeFace(embedding)
	assert.Error(t, err)
	assert.Equal(t, ErrFaceDetectionDisabled, err)
}

func TestPhotoEngine_GetStats(t *testing.T) {
	engine := NewPhotoEngine(nil)

	engine.AddPhoto(&Photo{ID: "photo-1", Tags: []string{"test"}})
	engine.AddPhoto(&Photo{ID: "photo-2", Tags: []string{"test", "vacation"}})

	stats := engine.GetStats()
	assert.Equal(t, int64(2), stats.TotalPhotos)
	assert.Equal(t, int64(0), stats.TotalPersons)
	assert.Equal(t, int64(0), stats.TotalAlbums)
}

func TestPhotoEngine_FilterByDate(t *testing.T) {
	engine := NewPhotoEngine(nil)

	now := time.Now()
	photos := []*Photo{
		{ID: "photo-1", ShotAt: now.Add(-24 * time.Hour)},
		{ID: "photo-2", ShotAt: now.Add(-48 * time.Hour)},
		{ID: "photo-3", ShotAt: now.Add(-72 * time.Hour)},
	}

	for _, p := range photos {
		engine.AddPhoto(p)
	}

	startDate := now.Add(-48 * time.Hour)
	endDate := now.Add(-12 * time.Hour)

	result, err := engine.TextSearch(&TextSearchQuery{
		StartDate: &startDate,
		EndDate:   &endDate,
		Limit:     10,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.TotalCount)
}

func TestPhotoEngine_FilterByTags(t *testing.T) {
	engine := NewPhotoEngine(nil)

	photos := []*Photo{
		{ID: "photo-1", Tags: []string{"beach", "sunset"}},
		{ID: "photo-2", Tags: []string{"beach", "party"}},
		{ID: "photo-3", Tags: []string{"mountain"}},
	}

	for _, p := range photos {
		engine.AddPhoto(p)
	}

	result, err := engine.TextSearch(&TextSearchQuery{
		Tags:  []string{"beach"},
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.TotalCount)
}

func TestPhotoEngine_MultipleFaces(t *testing.T) {
	engine := NewPhotoEngine(nil)

	person1 := &Person{ID: "person-1", Name: "Alice"}
	person2 := &Person{ID: "person-2", Name: "Bob"}
	engine.RegisterPerson(person1)
	engine.RegisterPerson(person2)

	photo := &Photo{
		ID: "photo-1",
		Faces: []FaceDetection{
			{PersonID: "person-1"},
			{PersonID: "person-2"},
		},
	}
	engine.AddPhoto(photo)

	photos1, _ := engine.GetPhotosByPerson("person-1")
	photos2, _ := engine.GetPhotosByPerson("person-2")
	assert.Len(t, photos1, 1)
	assert.Len(t, photos2, 1)
}

func TestPhotoEngine_EmbeddingIndex(t *testing.T) {
	engine := NewPhotoEngine(nil)

	photo := &Photo{
		ID:        "photo-1",
		Embedding: []float32{0.1, 0.2, 0.3},
	}
	engine.AddPhoto(photo)

	stats := engine.GetStats()
	assert.Equal(t, int64(1), stats.IndexSize)
}

func TestPhotoEngine_DefaultConfig(t *testing.T) {
	config := DefaultAlbumConfig()
	assert.True(t, config.FaceDetectionEnabled)
	assert.True(t, config.TextSearchEnabled)
	assert.True(t, config.AutoClassification)
	assert.Equal(t, 4, config.MaxConcurrentWorkers)
	assert.Equal(t, 300, config.ThumbnailSize)
	assert.Equal(t, 0.85, config.FaceMatchThreshold)
	assert.Equal(t, "clip-vit-base", config.TextSearchModel)
	assert.Equal(t, 24*time.Hour, config.CacheExpiration)
}

func TestPhotoEngine_AlbumSharing(t *testing.T) {
	engine := NewPhotoEngine(nil)

	album := &Album{
		ID:         "album-1",
		Name:       "Shared Album",
		OwnerID:    "user-1",
		SharedWith: []string{"user-2", "user-3"},
	}

	err := engine.CreateAlbum(album)
	require.NoError(t, err)
	assert.Len(t, album.SharedWith, 2)
}

func TestPhotoEngine_GPSInfo(t *testing.T) {
	engine := NewPhotoEngine(nil)

	photo := &Photo{
		ID: "photo-1",
		GPS: &GPSInfo{
			Latitude:  31.2304,
			Longitude: 121.4737,
			Address:   "Shanghai, China",
		},
	}
	engine.AddPhoto(photo)

	result, err := engine.TextSearch(&TextSearchQuery{
		Text:  "Shanghai",
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
}

func TestPhotoEngine_ConcurrentAccess(t *testing.T) {
	engine := NewPhotoEngine(nil)
	engine.Start()

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			photo := &Photo{
				ID:       "photo-" + string(rune(id)),
				FileName: "test.jpg",
				Tags:     []string{"test"},
			}
			engine.AddPhoto(photo)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := engine.GetStats()
	assert.Equal(t, int64(10), stats.TotalPhotos)
}
