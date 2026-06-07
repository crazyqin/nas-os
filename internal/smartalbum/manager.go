// Package smartalbum 提供 AI 智能相册管理功能
// 人脸识别聚类、场景自动分类、智能标签、时间线自动生成、相似照片推荐
package smartalbum

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 核心类型 ==========

// Photo 照片元数据
type Photo struct {
	ID          string            `json:"id"`
	Filename    string            `json:"filename"`
	Path        string            `json:"path"`
	Size        int64             `json:"size"`
	MimeType    string            `json:"mimeType"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	ShotAt      time.Time         `json:"shotAt"`
	UploadedAt  time.Time         `json:"uploadedAt"`
	CameraModel string            `json:"cameraModel,omitempty"`
	GPS         *GPSInfo          `json:"gps,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	FaceIDs     []string          `json:"faceIds,omitempty"`
	Scene       string            `json:"scene,omitempty"`
	Score       float64           `json:"score"` // 美学评分 0-100
	IsFavorite  bool              `json:"isFavorite"`
	IsHidden    bool              `json:"isHidden"`
	AlbumIDs    []string          `json:"albumIds,omitempty"`
	Hash        string            `json:"hash"` // 感知哈希，用于去重
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// GPSInfo GPS 信息
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// Face 人脸信息
type Face struct {
	ID         string    `json:"id"`
	Name       string    `json:"name,omitempty"`
	PhotoIDs   []string  `json:"photoIds"`
	Embedding  []float64 `json:"embedding,omitempty"` // 人脸特征向量
	CoverID    string    `json:"coverId"`             // 封面照片ID
	PhotoCount int       `json:"photoCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// SceneCategory 场景分类
type SceneCategory string

const (
	ScenePortrait     SceneCategory = "portrait"     // 人像
	SceneLandscape    SceneCategory = "landscape"    // 风景
	SceneFood         SceneCategory = "food"         // 美食
	SceneAnimal       SceneCategory = "animal"       // 动物
	SceneDocument     SceneCategory = "document"     // 文档
	SceneArchitecture SceneCategory = "architecture" // 建筑
	SceneNight        SceneCategory = "night"        // 夜景
	SceneMacro        SceneCategory = "macro"        // 微距
	SceneSport        SceneCategory = "sport"        // 运动
	SceneOther        SceneCategory = "other"        // 其他
)

// Album 相册
type Album struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        AlbumType      `json:"type"`
	CoverID     string         `json:"coverId,omitempty"`
	PhotoIDs    []string       `json:"photoIds"`
	PhotoCount  int            `json:"photoCount"`
	Criteria    *AlbumCriteria `json:"criteria,omitempty"` // 智能相册条件
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	IsShared    bool           `json:"isShared"`
}

// AlbumType 相册类型
type AlbumType string

const (
	AlbumTypeManual   AlbumType = "manual"   // 手动相册
	AlbumTypeSmart    AlbumType = "smart"    // 智能相册
	AlbumTypeTimeline AlbumType = "timeline" // 时间线相册
	AlbumTypeFace     AlbumType = "face"     // 人脸相册
	AlbumTypeScene    AlbumType = "scene"    // 场景相册
	AlbumTypePlace    AlbumType = "place"    // 地点相册
)

// AlbumCriteria 智能相册条件
type AlbumCriteria struct {
	Tags      []string        `json:"tags,omitempty"`
	Scenes    []SceneCategory `json:"scenes,omitempty"`
	FaceIDs   []string        `json:"faceIds,omitempty"`
	DateFrom  *time.Time      `json:"dateFrom,omitempty"`
	DateTo    *time.Time      `json:"dateTo,omitempty"`
	Camera    string          `json:"camera,omitempty"`
	MinScore  float64         `json:"minScore,omitempty"`
	Favorites bool            `json:"favorites,omitempty"`
}

// TimelineEntry 时间线条目
type TimelineEntry struct {
	Date      string   `json:"date"` // YYYY-MM-DD
	Count     int      `json:"count"`
	CoverID   string   `json:"coverId"`
	PhotoIDs  []string `json:"photoIds"`
	Locations []string `json:"locations,omitempty"`
}

// DuplicateGroup 重复照片组
type DuplicateGroup struct {
	Hash      string   `json:"hash"`
	PhotoIDs  []string `json:"photoIds"`
	Count     int      `json:"count"`
	TotalSize int64    `json:"totalSize"`
	BestID    string   `json:"bestId"` // 最佳照片ID
}

// ========== Manager ==========

// Manager 智能相册管理器
type Manager struct {
	mu     sync.RWMutex
	photos map[string]*Photo
	faces  map[string]*Face
	albums map[string]*Album
	index  map[string][]string // tag -> photoIDs
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		photos: make(map[string]*Photo),
		faces:  make(map[string]*Face),
		albums: make(map[string]*Album),
		index:  make(map[string][]string),
	}
}

// ========== 照片管理 ==========

// AddPhoto 添加照片
func (m *Manager) AddPhoto(photo Photo) (*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if photo.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	if photo.ID == "" {
		photo.ID = uuid.New().String()
	}
	if photo.UploadedAt.IsZero() {
		photo.UploadedAt = time.Now()
	}

	m.photos[photo.ID] = &photo

	// 更新标签索引
	for _, tag := range photo.Tags {
		m.index[tag] = append(m.index[tag], photo.ID)
	}

	log.Printf("[智能相册] 添加照片: %s (%s)", photo.ID, photo.Filename)
	return &photo, nil
}

// GetPhoto 获取照片
func (m *Manager) GetPhoto(id string) (*Photo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photo, ok := m.photos[id]
	if !ok {
		return nil, fmt.Errorf("photo %s not found", id)
	}
	return photo, nil
}

// ListPhotos 列出照片
func (m *Manager) ListPhotos(limit, offset int) []*Photo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photos := make([]*Photo, 0, len(m.photos))
	for _, p := range m.photos {
		photos = append(photos, p)
	}

	sort.Slice(photos, func(i, j int) bool {
		return photos[i].ShotAt.After(photos[j].ShotAt)
	})

	if offset >= len(photos) {
		return nil
	}
	end := offset + limit
	if end > len(photos) {
		end = len(photos)
	}
	return photos[offset:end]
}

// DeletePhoto 删除照片
func (m *Manager) DeletePhoto(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[id]
	if !ok {
		return fmt.Errorf("photo %s not found", id)
	}

	// 从标签索引中移除
	for _, tag := range photo.Tags {
		ids := m.index[tag]
		for i, pid := range ids {
			if pid == id {
				m.index[tag] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	// 从相册中移除
	for _, albumID := range photo.AlbumIDs {
		if album, ok := m.albums[albumID]; ok {
			for i, pid := range album.PhotoIDs {
				if pid == id {
					album.PhotoIDs = append(album.PhotoIDs[:i], album.PhotoIDs[i+1:]...)
					album.PhotoCount--
					break
				}
			}
		}
	}

	delete(m.photos, id)
	return nil
}

// ToggleFavorite 切换收藏
func (m *Manager) ToggleFavorite(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[id]
	if !ok {
		return fmt.Errorf("photo %s not found", id)
	}
	photo.IsFavorite = !photo.IsFavorite
	return nil
}

// ========== 人脸管理 ==========

// RegisterFace 注册人脸
func (m *Manager) RegisterFace(name string, photoID string, embedding []float64) (*Face, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.photos[photoID]; !ok {
		return nil, fmt.Errorf("photo %s not found", photoID)
	}

	face := &Face{
		ID:         uuid.New().String(),
		Name:       name,
		PhotoIDs:   []string{photoID},
		Embedding:  embedding,
		CoverID:    photoID,
		PhotoCount: 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	m.faces[face.ID] = face

	// 更新照片的 FaceIDs
	photo := m.photos[photoID]
	photo.FaceIDs = append(photo.FaceIDs, face.ID)

	log.Printf("[智能相册] 注册人脸: %s (%s)", face.ID, name)
	return face, nil
}

// LinkFaceToPhoto 关联人脸到照片
func (m *Manager) LinkFaceToPhoto(faceID, photoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	face, ok := m.faces[faceID]
	if !ok {
		return fmt.Errorf("face %s not found", faceID)
	}
	if _, ok := m.photos[photoID]; !ok {
		return fmt.Errorf("photo %s not found", photoID)
	}

	// 检查是否已关联
	for _, pid := range face.PhotoIDs {
		if pid == photoID {
			return nil
		}
	}

	face.PhotoIDs = append(face.PhotoIDs, photoID)
	face.PhotoCount = len(face.PhotoIDs)
	face.UpdatedAt = time.Now()

	photo := m.photos[photoID]
	photo.FaceIDs = append(photo.FaceIDs, faceID)

	return nil
}

// GetFace 获取人脸信息
func (m *Manager) GetFace(id string) (*Face, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	face, ok := m.faces[id]
	if !ok {
		return nil, fmt.Errorf("face %s not found", id)
	}
	return face, nil
}

// ListFaces 列出所有人脸
func (m *Manager) ListFaces() []*Face {
	m.mu.RLock()
	defer m.mu.RUnlock()

	faces := make([]*Face, 0, len(m.faces))
	for _, f := range m.faces {
		faces = append(faces, f)
	}

	sort.Slice(faces, func(i, j int) bool {
		return faces[i].PhotoCount > faces[j].PhotoCount
	})
	return faces
}

// ========== 相册管理 ==========

// CreateAlbum 创建相册
func (m *Manager) CreateAlbum(name string, albumType AlbumType) (*Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return nil, fmt.Errorf("album name is required")
	}

	album := &Album{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      albumType,
		PhotoIDs:  make([]string, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.albums[album.ID] = album
	log.Printf("[智能相册] 创建相册: %s (%s)", album.ID, name)
	return album, nil
}

// AddPhotoToAlbum 添加照片到相册
func (m *Manager) AddPhotoToAlbum(albumID, photoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	album, ok := m.albums[albumID]
	if !ok {
		return fmt.Errorf("album %s not found", albumID)
	}
	photo, ok := m.photos[photoID]
	if !ok {
		return fmt.Errorf("photo %s not found", photoID)
	}

	// 检查是否已在相册中
	for _, pid := range album.PhotoIDs {
		if pid == photoID {
			return nil
		}
	}

	album.PhotoIDs = append(album.PhotoIDs, photoID)
	album.PhotoCount = len(album.PhotoIDs)
	album.UpdatedAt = time.Now()

	if album.CoverID == "" {
		album.CoverID = photoID
	}

	photo.AlbumIDs = append(photo.AlbumIDs, albumID)
	return nil
}

// GetAlbum 获取相册
func (m *Manager) GetAlbum(id string) (*Album, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	album, ok := m.albums[id]
	if !ok {
		return nil, fmt.Errorf("album %s not found", id)
	}
	return album, nil
}

// ListAlbums 列出所有相册
func (m *Manager) ListAlbums() []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()

	albums := make([]*Album, 0, len(m.albums))
	for _, a := range m.albums {
		albums = append(albums, a)
	}

	sort.Slice(albums, func(i, j int) bool {
		return albums[i].UpdatedAt.After(albums[j].UpdatedAt)
	})
	return albums
}

// CreateSmartAlbum 创建智能相册
func (m *Manager) CreateSmartAlbum(name string, criteria AlbumCriteria) (*Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return nil, fmt.Errorf("album name is required")
	}

	album := &Album{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      AlbumTypeSmart,
		Criteria:  &criteria,
		PhotoIDs:  make([]string, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 根据条件匹配照片
	for _, photo := range m.photos {
		if m.matchesCriteria(photo, criteria) {
			album.PhotoIDs = append(album.PhotoIDs, photo.ID)
			photo.AlbumIDs = append(photo.AlbumIDs, album.ID)
		}
	}

	album.PhotoCount = len(album.PhotoIDs)
	if album.PhotoCount > 0 {
		album.CoverID = album.PhotoIDs[0]
	}

	m.albums[album.ID] = album
	log.Printf("[智能相册] 创建智能相册: %s, 匹配 %d 张照片", name, album.PhotoCount)
	return album, nil
}

// matchesCriteria 检查照片是否匹配条件
func (m *Manager) matchesCriteria(photo *Photo, criteria AlbumCriteria) bool {
	// 检查标签
	if len(criteria.Tags) > 0 {
		hasTag := false
		for _, tag := range criteria.Tags {
			for _, pt := range photo.Tags {
				if pt == tag {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	// 检查场景
	if len(criteria.Scenes) > 0 {
		hasScene := false
		for _, scene := range criteria.Scenes {
			if string(scene) == photo.Scene {
				hasScene = true
				break
			}
		}
		if !hasScene {
			return false
		}
	}

	// 检查人脸
	if len(criteria.FaceIDs) > 0 {
		hasFace := false
		for _, fid := range criteria.FaceIDs {
			for _, pf := range photo.FaceIDs {
				if pf == fid {
					hasFace = true
					break
				}
			}
			if hasFace {
				break
			}
		}
		if !hasFace {
			return false
		}
	}

	// 检查日期范围
	if criteria.DateFrom != nil && photo.ShotAt.Before(*criteria.DateFrom) {
		return false
	}
	if criteria.DateTo != nil && photo.ShotAt.After(*criteria.DateTo) {
		return false
	}

	// 检查最低评分
	if criteria.MinScore > 0 && photo.Score < criteria.MinScore {
		return false
	}

	// 检查收藏
	if criteria.Favorites && !photo.IsFavorite {
		return false
	}

	return true
}

// ========== 时间线 ==========

// GenerateTimeline 生成时间线
func (m *Manager) GenerateTimeline() []*TimelineEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dateMap := make(map[string]*TimelineEntry)
	for _, photo := range m.photos {
		if photo.IsHidden {
			continue
		}
		date := photo.ShotAt.Format("2006-01-02")
		entry, ok := dateMap[date]
		if !ok {
			entry = &TimelineEntry{Date: date}
			dateMap[date] = entry
		}
		entry.Count++
		entry.PhotoIDs = append(entry.PhotoIDs, photo.ID)
		if entry.CoverID == "" {
			entry.CoverID = photo.ID
		}
		if photo.GPS != nil && photo.GPS.Address != "" {
			found := false
			for _, loc := range entry.Locations {
				if loc == photo.GPS.Address {
					found = true
					break
				}
			}
			if !found {
				entry.Locations = append(entry.Locations, photo.GPS.Address)
			}
		}
	}

	timeline := make([]*TimelineEntry, 0, len(dateMap))
	for _, entry := range dateMap {
		timeline = append(timeline, entry)
	}

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Date > timeline[j].Date
	})

	return timeline
}

// ========== 重复检测 ==========

// DetectDuplicates 检测重复照片
func (m *Manager) DetectDuplicates() []*DuplicateGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hashGroups := make(map[string][]*Photo)
	for _, photo := range m.photos {
		if photo.Hash != "" {
			hashGroups[photo.Hash] = append(hashGroups[photo.Hash], photo)
		}
	}

	var groups []*DuplicateGroup
	for hash, photos := range hashGroups {
		if len(photos) < 2 {
			continue
		}

		group := &DuplicateGroup{
			Hash:     hash,
			PhotoIDs: make([]string, 0, len(photos)),
			Count:    len(photos),
		}

		bestScore := -1.0
		for _, p := range photos {
			group.PhotoIDs = append(group.PhotoIDs, p.ID)
			group.TotalSize += p.Size
			if p.Score > bestScore {
				bestScore = p.Score
				group.BestID = p.ID
			}
		}

		groups = append(groups, group)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].TotalSize > groups[j].TotalSize
	})

	return groups
}

// ========== 搜索 ==========

// SearchPhotos 搜索照片
func (m *Manager) SearchPhotos(query string, tags []string, scene SceneCategory) []*Photo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Photo
	for _, photo := range m.photos {
		if photo.IsHidden {
			continue
		}

		// 文本匹配
		if query != "" {
			matched := false
			if contains(photo.Filename, query) || contains(photo.CameraModel, query) {
				matched = true
			}
			for _, tag := range photo.Tags {
				if contains(tag, query) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 标签过滤
		if len(tags) > 0 {
			hasTag := false
			for _, tag := range tags {
				for _, pt := range photo.Tags {
					if pt == tag {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		// 场景过滤
		if scene != "" && photo.Scene != string(scene) {
			continue
		}

		results = append(results, photo)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// ========== 统计 ==========

// GetStats 获取相册统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalSize := int64(0)
	favorites := 0
	sceneCount := make(map[string]int)

	for _, photo := range m.photos {
		totalSize += photo.Size
		if photo.IsFavorite {
			favorites++
		}
		if photo.Scene != "" {
			sceneCount[photo.Scene]++
		}
	}

	return map[string]interface{}{
		"totalPhotos": len(m.photos),
		"totalFaces":  len(m.faces),
		"totalAlbums": len(m.albums),
		"totalSize":   totalSize,
		"favorites":   favorites,
		"sceneCount":  sceneCount,
	}
}

// contains 简单字符串包含检查
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) == 0 {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
