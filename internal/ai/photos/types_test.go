// Package photos provides AI-powered photo classification tests
package photos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ==================== Types Tests ====================

func TestPhoto(t *testing.T) {
	now := time.Now()
	photo := Photo{
		ID:         "photo1",
		Path:       "/photos/2024/test.jpg",
		Filename:   "test.jpg",
		Size:       1024000,
		Width:      1920,
		Height:     1080,
		Format:     "jpeg",
		CreatedAt:  now,
		ModifiedAt: now,
		Hash:       "abc123",
	}

	assert.Equal(t, "photo1", photo.ID)
	assert.Equal(t, "/photos/2024/test.jpg", photo.Path)
	assert.Equal(t, "jpeg", photo.Format)
	assert.Equal(t, 1920, photo.Width)
	assert.Equal(t, 1080, photo.Height)
}

func TestGPSInfo(t *testing.T) {
	gps := GPSInfo{
		Latitude:  39.9042,
		Longitude: 116.4074,
		Altitude:  50.0,
		Location: &Location{
			Country:     "China",
			CountryCode: "CN",
			Province:    "Beijing",
			City:        "Beijing",
		},
	}

	assert.Equal(t, 39.9042, gps.Latitude)
	assert.Equal(t, 116.4074, gps.Longitude)
	assert.Equal(t, "China", gps.Location.Country)
	assert.Equal(t, "Beijing", gps.Location.City)
}

func TestFaceDetection(t *testing.T) {
	face := FaceDetection{
		ID:      "face1",
		PhotoID: "photo1",
		BoundingBox: BoundingBox{
			X: 0.1, Y: 0.2, Width: 0.3, Height: 0.3,
		},
		Embedding:  make([]float32, 512),
		Quality:    0.95,
		PersonID:   "person1",
		PersonName: "John",
		CreatedAt:  time.Now(),
	}

	assert.Equal(t, "face1", face.ID)
	assert.Equal(t, "photo1", face.PhotoID)
	assert.Equal(t, 0.95, face.Quality)
	assert.Equal(t, "John", face.PersonName)
	assert.Len(t, face.Embedding, 512)
}

func TestBoundingBox(t *testing.T) {
	box := BoundingBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4}

	assert.Equal(t, 0.1, box.X)
	assert.Equal(t, 0.2, box.Y)
	assert.Equal(t, 0.3, box.Width)
	assert.Equal(t, 0.4, box.Height)

	// Verify normalized values (0-1 range)
	assert.True(t, box.X >= 0 && box.X <= 1)
	assert.True(t, box.Y >= 0 && box.Y <= 1)
	assert.True(t, box.Width >= 0 && box.Width <= 1)
	assert.True(t, box.Height >= 0 && box.Height <= 1)
}

func TestSceneClassification(t *testing.T) {
	scene := SceneClassification{
		PrimaryCategory: ScenePortrait,
		Categories: []CategoryScore{
			{Category: ScenePortrait, Score: 0.9},
			{Category: SceneIndoor, Score: 0.3},
		},
		Objects: []ObjectDetection{
			{Label: "person", Score: 0.95},
		},
		Colors: []ColorInfo{
			{Hex: "#FF5733", Name: "red", Percent: 0.25},
		},
		Quality: PhotoQuality{
			Score:     0.85,
			Sharpness: 0.9,
			IsBlurred: false,
		},
		AestheticScore: 0.75,
	}

	assert.Equal(t, ScenePortrait, scene.PrimaryCategory)
	assert.Len(t, scene.Categories, 2)
	assert.Len(t, scene.Objects, 1)
	assert.Len(t, scene.Colors, 1)
	assert.Equal(t, 0.85, scene.Quality.Score)
}

func TestPhotoQuality(t *testing.T) {
	quality := PhotoQuality{
		Score:      0.8,
		Sharpness:  0.85,
		Exposure:   0.5,
		Contrast:   0.7,
		IsBlurred:  false,
		IsLowLight: false,
		HasRedEye:  false,
	}

	assert.Equal(t, 0.8, quality.Score)
	assert.False(t, quality.IsBlurred)
	assert.False(t, quality.IsLowLight)
}

func TestAlbum(t *testing.T) {
	now := time.Now()
	startDate := now.AddDate(0, -1, 0)

	album := Album{
		ID:           "album1",
		Name:         "Summer Vacation",
		Type:         AlbumTypeDate,
		PhotoCount:   150,
		CoverPhotoID: "photo1",
		CreatedAt:    now,
		UpdatedAt:    now,
		StartDate:    &startDate,
		EndDate:      &now,
	}

	assert.Equal(t, "album1", album.ID)
	assert.Equal(t, "Summer Vacation", album.Name)
	assert.Equal(t, AlbumTypeDate, album.Type)
	assert.Equal(t, 150, album.PhotoCount)
}

func TestPerson(t *testing.T) {
	person := Person{
		ID:                 "person1",
		Name:               "Alice",
		RepresentativeFace: "face1",
		FaceCount:          50,
		CoverPhotoID:       "photo1",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	assert.Equal(t, "person1", person.ID)
	assert.Equal(t, "Alice", person.Name)
	assert.Equal(t, 50, person.FaceCount)
}

// ==================== Config Tests ====================

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.True(t, config.EnableFaceRecognition)
	assert.Equal(t, "arcface", config.FaceModel)
	assert.Equal(t, 30, config.FaceMinSize)
	assert.Equal(t, 0.6, config.FaceClusterThreshold)

	assert.True(t, config.EnableSceneClassification)
	assert.Equal(t, "clip", config.SceneModel)
	assert.Equal(t, 5, config.SceneTopK)

	assert.True(t, config.EnableLocationLookup)
	assert.Equal(t, "nominatim", config.GeocodingProvider)

	assert.Equal(t, 32, config.BatchSize)
	assert.Equal(t, 4, config.MaxWorkers)
	assert.False(t, config.EnableGPU)

	assert.True(t, config.CacheEnabled)
	assert.Equal(t, 86400, config.CacheTTL)
}

func TestFaceRecognitionConfig_Default(t *testing.T) {
	config := DefaultFaceRecognitionConfig()

	assert.Equal(t, "arcface", config.Model)
	assert.Equal(t, 30, config.MinFaceSize)
	assert.Equal(t, 0.8, config.ConfidenceThresh)
	assert.Equal(t, 0.6, config.ClusterThreshold)
	assert.Equal(t, 50, config.MaxFacesPerPhoto)
	assert.False(t, config.UseGPU)
}

// ==================== Scene Category Tests ====================

func TestSceneCategoryConstants(t *testing.T) {
	categories := []SceneCategory{
		ScenePortrait, SceneLandscape, SceneFood, ScenePet,
		SceneArchitecture, SceneNature, SceneNight, SceneBeach,
		SceneMountain, SceneCity, SceneIndoor, SceneEvent,
		SceneDocument, SceneVehicle, SceneOther,
	}

	for _, cat := range categories {
		assert.NotEmpty(t, string(cat))
	}
}

func TestAlbumTypeConstants(t *testing.T) {
	types := []AlbumType{
		AlbumTypeAuto, AlbumTypePerson, AlbumTypeLocation,
		AlbumTypeDate, AlbumTypeScene, AlbumTypeSmart, AlbumTypeUser,
	}

	for _, at := range types {
		assert.NotEmpty(t, string(at))
	}
}

// ==================== ClassificationResult Tests ====================

func TestClassificationResult(t *testing.T) {
	result := ClassificationResult{
		PhotoID: "photo1",
		Faces: []FaceDetection{
			{ID: "face1", PhotoID: "photo1", Quality: 0.9},
		},
		Scene: &SceneClassification{
			PrimaryCategory: ScenePortrait,
		},
		Tags:        []string{"outdoor", "sunset"},
		ProcessTime: 150 * time.Millisecond,
	}

	assert.Equal(t, "photo1", result.PhotoID)
	assert.Len(t, result.Faces, 1)
	assert.NotNil(t, result.Scene)
	assert.Len(t, result.Tags, 2)
	assert.Equal(t, 150*time.Millisecond, result.ProcessTime)
}

func TestClusterResult(t *testing.T) {
	result := ClusterResult{
		Persons: []Person{
			{ID: "person1", Name: "Alice", FaceCount: 10},
			{ID: "person2", Name: "Bob", FaceCount: 8},
		},
		Unassigned: []FaceDetection{
			{ID: "face99", Quality: 0.5},
		},
		ClusterCount: 2,
	}

	assert.Len(t, result.Persons, 2)
	assert.Len(t, result.Unassigned, 1)
	assert.Equal(t, 2, result.ClusterCount)
}

// ==================== ObjectDetection Tests ====================

func TestObjectDetection(t *testing.T) {
	obj := ObjectDetection{
		Label: "car",
		Score: 0.92,
		Box: BoundingBox{
			X: 0.2, Y: 0.3, Width: 0.4, Height: 0.3,
		},
	}

	assert.Equal(t, "car", obj.Label)
	assert.Equal(t, 0.92, obj.Score)
	assert.NotNil(t, obj.Box)
}

// ==================== ColorInfo Tests ====================

func TestColorInfo(t *testing.T) {
	color := ColorInfo{
		Hex:     "#3366FF",
		Name:    "blue",
		Percent: 0.35,
	}

	assert.Equal(t, "#3366FF", color.Hex)
	assert.Equal(t, "blue", color.Name)
	assert.Equal(t, 0.35, color.Percent)
}

// ==================== Location Tests ====================

func TestLocation(t *testing.T) {
	loc := Location{
		Country:     "China",
		CountryCode: "CN",
		Province:    "Guangdong",
		City:        "Shenzhen",
		District:    "Nanshan",
		Street:      "Keji Road",
		POI:         "Tencent Building",
		PlaceID:     "place123",
	}

	assert.Equal(t, "China", loc.Country)
	assert.Equal(t, "CN", loc.CountryCode)
	assert.Equal(t, "Guangdong", loc.Province)
	assert.Equal(t, "Shenzhen", loc.City)
}

// ==================== CameraInfo Tests ====================

func TestCameraInfo(t *testing.T) {
	cam := CameraInfo{
		Make:         "Apple",
		Model:        "iPhone 15 Pro",
		Lens:         "iPhone 15 Pro back triple camera",
		Aperture:     "f/1.8",
		ShutterSpeed: "1/125",
		ISO:          100,
		FocalLength:  "24mm",
	}

	assert.Equal(t, "Apple", cam.Make)
	assert.Equal(t, "iPhone 15 Pro", cam.Model)
	assert.Equal(t, "f/1.8", cam.Aperture)
	assert.Equal(t, 100, cam.ISO)
}