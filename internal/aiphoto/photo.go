// Package aiphoto AI相册模块
// 提供智能照片分类、人脸识别、场景识别等功能
// 参考: 飞牛 fnOS AI相册功能
package aiphoto

import (
	"fmt"
	"sync"
	"time"
)

// PhotoCategory 照片分类
type PhotoCategory string

const (
	CategoryPortrait  PhotoCategory = "portrait"   // 人像
	CategoryLandscape PhotoCategory = "landscape"  // 风景
	CategoryFood      PhotoCategory = "food"       // 美食
	CategoryAnimal    PhotoCategory = "animal"     // 动物
	CategoryDocument  PhotoCategory = "document"   // 文档
	CategoryVehicle   PhotoCategory = "vehicle"    // 交通工具
	CategoryBuilding  PhotoCategory = "building"   // 建筑
	CategoryOther     PhotoCategory = "other"      // 其他
)

// FaceInfo 人脸信息
type FaceInfo struct {
	FaceID     string    `json:"face_id"`
	PersonID   string    `json:"person_id"`
	Name       string    `json:"name"`
	Age        int       `json:"age"`
	Gender     string    `json:"gender"`
	Confidence float64   `json:"confidence"`
	BoundingBox BoundingBox `json:"bounding_box"`
}

// BoundingBox 边界框
type BoundingBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// SceneInfo 场景信息
type SceneInfo struct {
	SceneID    string   `json:"scene_id"`
	Name       string   `json:"name"`
	Confidence float64  `json:"confidence"`
	Tags       []string `json:"tags"`
}

// AlbumPhotoMetadata 相册照片元数据（AI相册专用）
type AlbumPhotoMetadata struct {
	ID         string        `json:"id"`
	FilePath   string        `json:"file_path"`
	FileName   string        `json:"file_name"`
	Size       int64         `json:"size"`
	Width      int           `json:"width"`
	Height     int           `json:"height"`
	Format     string        `json:"format"`
	TakenAt    *time.Time    `json:"taken_at,omitempty"`
	UploadedAt time.Time     `json:"uploaded_at"`
	SHA256     string        `json:"sha256"`
	GPSLat     float64       `json:"gps_lat,omitempty"`
	GPSLng     float64       `json:"gps_lng,omitempty"`
	Category   PhotoCategory `json:"category"`
	Faces      []FaceInfo    `json:"faces"`
	Scenes     []SceneInfo   `json:"scenes"`
	Tags       []string      `json:"tags"`
	Favorite   bool          `json:"favorite"`
	Album      string        `json:"album,omitempty"`
}

// Album 相册
type Album struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CoverPhoto  string     `json:"cover_photo"`
	PhotoCount  int        `json:"photo_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	IsSmart     bool       `json:"is_smart"`     // 智能相册
	AutoRule    *SmartRule `json:"auto_rule,omitempty"`
}

// SmartRule 智能相册规则
type SmartRule struct {
	Category  PhotoCategory `json:"category,omitempty"`
	PersonID  string        `json:"person_id,omitempty"`
	SceneName string        `json:"scene_name,omitempty"`
	DateFrom  *time.Time    `json:"date_from,omitempty"`
	DateTo    *time.Time    `json:"date_to,omitempty"`
	Tags      []string      `json:"tags,omitempty"`
}

// AIPhotoManager AI相册管理器
type AIPhotoManager struct {
	mu        sync.RWMutex
	photos    map[string]*AlbumPhotoMetadata
	albums    map[string]*Album
	faces     map[string]*FaceInfo
	modelPath string
	config    *PhotoConfig
}

// PhotoConfig 相册配置
type PhotoConfig struct {
	MaxPhotos            int    `json:"max_photos"`
	EnableFaceDetection  bool   `json:"enable_face_detection"`
	EnableSceneDetection bool   `json:"enable_scene_detection"`
	EnableAutoTagging    bool   `json:"enable_auto_tagging"`
	ThumbnailSize        int    `json:"thumbnail_size"`
	AIModelPath          string `json:"ai_model_path"`
}

// NewAIPhotoManager 创建AI相册管理器
func NewAIPhotoManager(config *PhotoConfig) *AIPhotoManager {
	if config == nil {
		config = &PhotoConfig{
			MaxPhotos:            100000,
			EnableFaceDetection:  true,
			EnableSceneDetection: true,
			EnableAutoTagging:    true,
			ThumbnailSize:        300,
		}
	}
	return &AIPhotoManager{
		photos:    make(map[string]*AlbumPhotoMetadata),
		albums:    make(map[string]*Album),
		faces:     make(map[string]*FaceInfo),
		modelPath: config.AIModelPath,
		config:    config,
	}
}

// AddPhoto 添加照片并进行AI分析
func (m *AIPhotoManager) AddPhoto(filePath, fileName string, size int64) (*AlbumPhotoMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.photos) >= m.config.MaxPhotos {
		return nil, fmt.Errorf("max photos reached (%d)", m.config.MaxPhotos)
	}

	photo := &AlbumPhotoMetadata{
		ID:         fmt.Sprintf("photo_%d", time.Now().UnixNano()),
		FilePath:   filePath,
		FileName:   fileName,
		Size:       size,
		UploadedAt: time.Now(),
		Category:   CategoryOther,
		Faces:      make([]FaceInfo, 0),
		Scenes:     make([]SceneInfo, 0),
		Tags:       make([]string, 0),
	}

	// 模拟AI分析
	if m.config.EnableSceneDetection {
		photo.Category = m.analyzeCategory(filePath)
		photo.Scenes = m.analyzeScenes(filePath)
	}

	if m.config.EnableFaceDetection {
		photo.Faces = m.detectFaces(filePath)
	}

	if m.config.EnableAutoTagging {
		photo.Tags = m.generateTags(photo)
	}

	m.photos[photo.ID] = photo
	return photo, nil
}

// analyzeCategory 分析照片分类（模拟）
func (m *AIPhotoManager) analyzeCategory(filePath string) PhotoCategory {
	categories := []PhotoCategory{
		CategoryPortrait, CategoryLandscape, CategoryFood,
		CategoryAnimal, CategoryDocument, CategoryVehicle,
		CategoryBuilding, CategoryOther,
	}
	for _, cat := range categories {
		if len(filePath) > 0 {
			return cat
		}
	}
	return CategoryOther
}

// analyzeScenes 分析场景（模拟）
func (m *AIPhotoManager) analyzeScenes(filePath string) []SceneInfo {
	return []SceneInfo{
		{
			SceneID:    fmt.Sprintf("scene_%d", time.Now().UnixNano()),
			Name:       "室内",
			Confidence: 0.85,
			Tags:       []string{"室内", "自然光"},
		},
	}
}

// detectFaces 检测人脸（模拟）
func (m *AIPhotoManager) detectFaces(filePath string) []FaceInfo {
	return []FaceInfo{
		{
			FaceID:     fmt.Sprintf("face_%d", time.Now().UnixNano()),
			PersonID:   "person_001",
			Name:       "用户",
			Age:        25,
			Gender:     "male",
			Confidence: 0.92,
			BoundingBox: BoundingBox{
				X:      100,
				Y:      100,
				Width:  150,
				Height: 150,
			},
		},
	}
}

// generateTags 生成标签（模拟）
func (m *AIPhotoManager) generateTags(photo *AlbumPhotoMetadata) []string {
	tags := []string{string(photo.Category)}
	for _, scene := range photo.Scenes {
		tags = append(tags, scene.Tags...)
	}
	return tags
}

// SearchPhotos 搜索照片
func (m *AIPhotoManager) SearchPhotos(query *SmartRule) []*AlbumPhotoMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*AlbumPhotoMetadata, 0)
	for _, photo := range m.photos {
		if m.matchesRule(photo, query) {
			results = append(results, photo)
		}
	}
	return results
}

// matchesRule 检查照片是否匹配规则
func (m *AIPhotoManager) matchesRule(photo *AlbumPhotoMetadata, rule *SmartRule) bool {
	if rule == nil {
		return true
	}

	if rule.Category != "" && photo.Category != rule.Category {
		return false
	}

	if rule.PersonID != "" {
		found := false
		for _, face := range photo.Faces {
			if face.PersonID == rule.PersonID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if rule.SceneName != "" {
		found := false
		for _, scene := range photo.Scenes {
			if scene.Name == rule.SceneName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// CreateAlbum 创建相册
func (m *AIPhotoManager) CreateAlbum(name, description string, isSmart bool, rule *SmartRule) (*Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	album := &Album{
		ID:          fmt.Sprintf("album_%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		IsSmart:     isSmart,
		AutoRule:    rule,
	}

	if isSmart && rule != nil {
		for _, photo := range m.photos {
			if m.matchesRule(photo, rule) {
				photo.Album = album.ID
				album.PhotoCount++
			}
		}
		if album.PhotoCount > 0 {
			for _, photo := range m.photos {
				if photo.Album == album.ID {
					album.CoverPhoto = photo.ID
					break
				}
			}
		}
	}

	m.albums[album.ID] = album
	return album, nil
}

// ListAlbums 列出所有相册
func (m *AIPhotoManager) ListAlbums() []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()

	albums := make([]*Album, 0, len(m.albums))
	for _, a := range m.albums {
		albums = append(albums, a)
	}
	return albums
}

// GetPhoto 获取照片详情
func (m *AIPhotoManager) GetPhoto(photoID string) (*AlbumPhotoMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return nil, fmt.Errorf("photo not found: %s", photoID)
	}
	return photo, nil
}

// ToggleFavorite 切换收藏状态
func (m *AIPhotoManager) ToggleFavorite(photoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return fmt.Errorf("photo not found: %s", photoID)
	}

	photo.Favorite = !photo.Favorite
	return nil
}

// GetStats 获取统计信息
func (m *AIPhotoManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	categoryCount := make(map[PhotoCategory]int)
	favoriteCount := 0
	faceCount := 0

	for _, photo := range m.photos {
		categoryCount[photo.Category]++
		if photo.Favorite {
			favoriteCount++
		}
		faceCount += len(photo.Faces)
	}

	return map[string]interface{}{
		"total_photos":       len(m.photos),
		"total_albums":       len(m.albums),
		"favorite_count":     favoriteCount,
		"face_count":         faceCount,
		"category_breakdown": categoryCount,
	}
}
