// Package smartalbum 提供 AI 智能相册管理功能
// 人脸识别聚类、场景自动分类、智能标签、时间线自动生成、相似照片推荐
package smartalbum

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 核心类型 ==========

// Photo 照片元数据.
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
	Hash        string            `json:"hash"`                // 感知哈希，用于去重
	Embedding   []float64         `json:"embedding,omitempty"` // CLIP 语义向量
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// GPSInfo GPS 信息.
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
	Address   string  `json:"address,omitempty"`
	City      string  `json:"city,omitempty"`
	Country   string  `json:"country,omitempty"`
}

// MapCluster 地图聚合点.
type MapCluster struct {
	ID        string   `json:"id"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Count     int      `json:"count"`
	PhotoIDs  []string `json:"photoIds"`
	Radius    float64  `json:"radius"` // 聚合半径（米）
}

// MapBounds 地图边界.
type MapBounds struct {
	NorthEast GPSInfo `json:"northEast"`
	SouthWest GPSInfo `json:"southWest"`
}

// Face 人脸信息.
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

// SceneCategory 场景分类.
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

// Album 相册.
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

// AlbumType 相册类型.
type AlbumType string

const (
	AlbumTypeManual   AlbumType = "manual"   // 手动相册
	AlbumTypeSmart    AlbumType = "smart"    // 智能相册
	AlbumTypeTimeline AlbumType = "timeline" // 时间线相册
	AlbumTypeFace     AlbumType = "face"     // 人脸相册
	AlbumTypeScene    AlbumType = "scene"    // 场景相册
	AlbumTypePlace    AlbumType = "place"    // 地点相册
)

// AlbumCriteria 智能相册条件.
type AlbumCriteria struct {
	Tags      []string        `json:"tags,omitempty"`
	Scenes    []SceneCategory `json:"scenes,omitempty"`
	FaceIDs   []string        `json:"faceIds,omitempty"`
	DateFrom  *time.Time      `json:"dateFrom,omitempty"`
	DateTo    *time.Time      `json:"dateTo,omitempty"`
	Camera    string          `json:"camera,omitempty"`
	MinScore  float64         `json:"minScore,omitempty"`
	Favorites bool            `json:"favorites,omitempty"`
	Location  string          `json:"location,omitempty"` // 地点关键词
}

// TimelineEntry 时间线条目.
type TimelineEntry struct {
	Date      string   `json:"date"` // YYYY-MM-DD
	Count     int      `json:"count"`
	CoverID   string   `json:"coverId"`
	PhotoIDs  []string `json:"photoIds"`
	Locations []string `json:"locations,omitempty"`
}

// DuplicateGroup 重复照片组.
type DuplicateGroup struct {
	Hash      string   `json:"hash"`
	PhotoIDs  []string `json:"photoIds"`
	Count     int      `json:"count"`
	TotalSize int64    `json:"totalSize"`
	BestID    string   `json:"bestId"` // 最佳照片ID
}

// ========== Manager ==========

// Manager 智能相册管理器.
type Manager struct {
	mu     sync.RWMutex
	photos map[string]*Photo
	faces  map[string]*Face
	albums map[string]*Album
	index  map[string][]string // tag -> photoIDs
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		photos: make(map[string]*Photo),
		faces:  make(map[string]*Face),
		albums: make(map[string]*Album),
		index:  make(map[string][]string),
	}
}

// ========== 照片管理 ==========

// AddPhoto 添加照片.
func (m *Manager) AddPhoto(photo Photo) (*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if photo.ID == "" {
		photo.ID = uuid.New().String()
	}
	photo.UploadedAt = time.Now()

	// 构建标签索引
	for _, tag := range photo.Tags {
		m.index[tag] = append(m.index[tag], photo.ID)
	}

	m.photos[photo.ID] = &photo
	return &photo, nil
}

// GetPhoto 获取照片.
func (m *Manager) GetPhoto(id string) (*Photo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photo, ok := m.photos[id]
	if !ok {
		return nil, fmt.Errorf("photo not found: %s", id)
	}
	return photo, nil
}

// ListPhotos 列出照片.
func (m *Manager) ListPhotos(limit, offset int) []*Photo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photos := make([]*Photo, 0, len(m.photos))
	for _, p := range m.photos {
		if !p.IsHidden {
			photos = append(photos, p)
		}
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

// DeletePhoto 删除照片.
func (m *Manager) DeletePhoto(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[id]
	if !ok {
		return fmt.Errorf("photo not found: %s", id)
	}

	// 清理标签索引
	for _, tag := range photo.Tags {
		ids := m.index[tag]
		for i, pid := range ids {
			if pid == id {
				m.index[tag] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	delete(m.photos, id)
	return nil
}

// ToggleFavorite 切换收藏状态.
func (m *Manager) ToggleFavorite(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[id]
	if !ok {
		return fmt.Errorf("photo not found: %s", id)
	}
	photo.IsFavorite = !photo.IsFavorite
	return nil
}

// ========== 人脸管理 ==========

// RegisterFace 注册人脸.
func (m *Manager) RegisterFace(name string, photoID string, embedding []float64) (*Face, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[photoID]
	if !ok {
		return nil, fmt.Errorf("photo not found: %s", photoID)
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
	photo.FaceIDs = append(photo.FaceIDs, face.ID)
	return face, nil
}

// LinkFaceToPhoto 关联人脸到照片.
func (m *Manager) LinkFaceToPhoto(faceID, photoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	face, ok := m.faces[faceID]
	if !ok {
		return fmt.Errorf("face not found: %s", faceID)
	}
	photo, ok := m.photos[photoID]
	if !ok {
		return fmt.Errorf("photo not found: %s", photoID)
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

	photo.FaceIDs = append(photo.FaceIDs, faceID)
	return nil
}

// GetFace 获取人脸.
func (m *Manager) GetFace(id string) (*Face, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	face, ok := m.faces[id]
	if !ok {
		return nil, fmt.Errorf("face not found: %s", id)
	}
	return face, nil
}

// ListFaces 列出人脸.
func (m *Manager) ListFaces() []*Face {
	m.mu.RLock()
	defer m.mu.RUnlock()

	faces := make([]*Face, 0, len(m.faces))
	for _, f := range m.faces {
		faces = append(faces, f)
	}
	return faces
}

// ========== 相册管理 ==========

// CreateAlbum 创建相册.
func (m *Manager) CreateAlbum(name string, albumType AlbumType) (*Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	album := &Album{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      albumType,
		PhotoIDs:  []string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.albums[album.ID] = album
	return album, nil
}

// AddPhotoToAlbum 添加照片到相册.
func (m *Manager) AddPhotoToAlbum(albumID, photoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	album, ok := m.albums[albumID]
	if !ok {
		return fmt.Errorf("album not found: %s", albumID)
	}
	photo, ok := m.photos[photoID]
	if !ok {
		return fmt.Errorf("photo not found: %s", photoID)
	}

	// 检查是否已添加
	for _, pid := range album.PhotoIDs {
		if pid == photoID {
			return nil
		}
	}

	album.PhotoIDs = append(album.PhotoIDs, photoID)
	album.PhotoCount = len(album.PhotoIDs)
	album.UpdatedAt = time.Now()

	photo.AlbumIDs = append(photo.AlbumIDs, albumID)
	return nil
}

// GetAlbum 获取相册.
func (m *Manager) GetAlbum(id string) (*Album, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	album, ok := m.albums[id]
	if !ok {
		return nil, fmt.Errorf("album not found: %s", id)
	}
	return album, nil
}

// ListAlbums 列出相册.
func (m *Manager) ListAlbums() []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()

	albums := make([]*Album, 0, len(m.albums))
	for _, a := range m.albums {
		albums = append(albums, a)
	}
	return albums
}

// CreateSmartAlbum 创建智能相册.
func (m *Manager) CreateSmartAlbum(name string, criteria AlbumCriteria) (*Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	album := &Album{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      AlbumTypeSmart,
		Criteria:  &criteria,
		PhotoIDs:  []string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 根据条件匹配照片
	for _, photo := range m.photos {
		if m.matchesCriteria(photo, criteria) {
			album.PhotoIDs = append(album.PhotoIDs, photo.ID)
		}
	}
	album.PhotoCount = len(album.PhotoIDs)

	m.albums[album.ID] = album
	return album, nil
}

// matchesCriteria 检查照片是否匹配条件.
func (m *Manager) matchesCriteria(photo *Photo, criteria AlbumCriteria) bool {
	// 标签匹配
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

	// 场景匹配
	if len(criteria.Scenes) > 0 {
		hasScene := false
		for _, scene := range criteria.Scenes {
			if photo.Scene == string(scene) {
				hasScene = true
				break
			}
		}
		if !hasScene {
			return false
		}
	}

	// 人脸匹配
	if len(criteria.FaceIDs) > 0 {
		hasFace := false
		for _, faceID := range criteria.FaceIDs {
			for _, pfid := range photo.FaceIDs {
				if pfid == faceID {
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

	// 日期范围
	if criteria.DateFrom != nil && photo.ShotAt.Before(*criteria.DateFrom) {
		return false
	}
	if criteria.DateTo != nil && photo.ShotAt.After(*criteria.DateTo) {
		return false
	}

	// 相机型号
	if criteria.Camera != "" && photo.CameraModel != criteria.Camera {
		return false
	}

	// 最低评分
	if criteria.MinScore > 0 && photo.Score < criteria.MinScore {
		return false
	}

	// 收藏
	if criteria.Favorites && !photo.IsFavorite {
		return false
	}

	// 地点关键词
	if criteria.Location != "" && photo.GPS != nil {
		if !contains(photo.GPS.Address, criteria.Location) &&
			!contains(photo.GPS.City, criteria.Location) &&
			!contains(photo.GPS.Country, criteria.Location) {
			return false
		}
	}

	return true
}

// ========== 时间线 ==========

// GenerateTimeline 生成时间线.
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
			entry = &TimelineEntry{
				Date:     date,
				CoverID:  photo.ID,
				PhotoIDs: []string{},
			}
			dateMap[date] = entry
		}
		entry.Count++
		entry.PhotoIDs = append(entry.PhotoIDs, photo.ID)
		if photo.GPS != nil && photo.GPS.Address != "" {
			entry.Locations = appendUnique(entry.Locations, photo.GPS.Address)
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

// DetectDuplicates 检测重复照片.
func (m *Manager) DetectDuplicates() []*DuplicateGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hashGroups := make(map[string][]*Photo)
	for _, photo := range m.photos {
		if photo.Hash != "" {
			hashGroups[photo.Hash] = append(hashGroups[photo.Hash], photo)
		}
	}

	groups := make([]*DuplicateGroup, 0)
	for hash, photos := range hashGroups {
		if len(photos) < 2 {
			continue
		}
		group := &DuplicateGroup{
			Hash:     hash,
			PhotoIDs: make([]string, 0, len(photos)),
			BestID:   photos[0].ID,
		}
		bestScore := photos[0].Score
		for _, p := range photos {
			group.PhotoIDs = append(group.PhotoIDs, p.ID)
			group.TotalSize += p.Size
			if p.Score > bestScore {
				bestScore = p.Score
				group.BestID = p.ID
			}
		}
		group.Count = len(group.PhotoIDs)
		groups = append(groups, group)
	}
	return groups
}

// ========== 搜索功能 ==========

// SearchPhotos 搜索照片.
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

// ========== 语义搜索（新增） ==========

// SemanticSearch 语义搜索 - 使用 CLIP 向量相似度.
func (m *Manager) SemanticSearch(queryEmbedding []float64, topK int, minScore float64) []*Photo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(queryEmbedding) == 0 {
		return nil
	}

	type photoScore struct {
		photo *Photo
		score float64
	}

	scores := make([]photoScore, 0, len(m.photos))
	for _, photo := range m.photos {
		if photo.IsHidden || len(photo.Embedding) == 0 {
			continue
		}
		similarity := cosineSimilarity(queryEmbedding, photo.Embedding)
		if similarity >= minScore {
			scores = append(scores, photoScore{photo: photo, score: similarity})
		}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if topK > 0 && len(scores) > topK {
		scores = scores[:topK]
	}

	results := make([]*Photo, len(scores))
	for i, ps := range scores {
		results[i] = ps.photo
	}
	return results
}

// FindSimilarPhotos 查找相似照片.
func (m *Manager) FindSimilarPhotos(photoID string, topK int) ([]*Photo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photo, ok := m.photos[photoID]
	if !ok {
		return nil, fmt.Errorf("photo not found: %s", photoID)
	}
	if len(photo.Embedding) == 0 {
		return nil, fmt.Errorf("photo has no embedding: %s", photoID)
	}

	type photoScore struct {
		photo *Photo
		score float64
	}

	scores := make([]photoScore, 0, len(m.photos))
	for _, p := range m.photos {
		if p.ID == photoID || p.IsHidden || len(p.Embedding) == 0 {
			continue
		}
		similarity := cosineSimilarity(photo.Embedding, p.Embedding)
		scores = append(scores, photoScore{photo: p, score: similarity})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if topK > 0 && len(scores) > topK {
		scores = scores[:topK]
	}

	results := make([]*Photo, len(scores))
	for i, ps := range scores {
		results[i] = ps.photo
	}
	return results, nil
}

// ========== 地图功能（新增） ==========

// GetMapClusters 获取地图聚合点.
func (m *Manager) GetMapClusters(bounds *MapBounds, zoomLevel int) []*MapCluster {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 收集有 GPS 信息的照片
	type locationPhoto struct {
		photo     *Photo
		latitude  float64
		longitude float64
	}

	var photos []locationPhoto
	for _, photo := range m.photos {
		if photo.IsHidden || photo.GPS == nil {
			continue
		}
		// 检查是否在边界内
		if bounds != nil {
			if photo.GPS.Latitude > bounds.NorthEast.Latitude ||
				photo.GPS.Latitude < bounds.SouthWest.Latitude ||
				photo.GPS.Longitude > bounds.NorthEast.Longitude ||
				photo.GPS.Longitude < bounds.SouthWest.Longitude {
				continue
			}
		}
		photos = append(photos, locationPhoto{
			photo:     photo,
			latitude:  photo.GPS.Latitude,
			longitude: photo.GPS.Longitude,
		})
	}

	// 根据缩放级别计算聚合距离
	clusterRadius := calculateClusterRadius(zoomLevel)

	// 简单的基于距离的聚合算法
	clusters := make([]*MapCluster, 0)
	visited := make(map[string]bool)

	for _, lp := range photos {
		if visited[lp.photo.ID] {
			continue
		}

		cluster := &MapCluster{
			ID:        uuid.New().String(),
			Latitude:  lp.latitude,
			Longitude: lp.longitude,
			PhotoIDs:  []string{lp.photo.ID},
			Radius:    clusterRadius,
		}
		visited[lp.photo.ID] = true

		// 查找附近的照片
		for _, other := range photos {
			if visited[other.photo.ID] {
				continue
			}
			distance := haversineDistance(lp.latitude, lp.longitude, other.latitude, other.longitude)
			if distance <= clusterRadius {
				cluster.PhotoIDs = append(cluster.PhotoIDs, other.photo.ID)
				visited[other.photo.ID] = true
			}
		}

		cluster.Count = len(cluster.PhotoIDs)
		clusters = append(clusters, cluster)
	}

	return clusters
}

// GetPhotosByLocation 按地点获取照片.
func (m *Manager) GetPhotosByLocation(city string, limit int) []*Photo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Photo
	for _, photo := range m.photos {
		if photo.IsHidden || photo.GPS == nil {
			continue
		}
		if contains(photo.GPS.City, city) || contains(photo.GPS.Address, city) {
			results = append(results, photo)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ShotAt.After(results[j].ShotAt)
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

// ========== 统计 ==========

// GetStats 获取相册统计.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalSize := int64(0)
	favorites := 0
	sceneCount := make(map[string]int)
	locationCount := make(map[string]int)
	embeddingCount := 0

	for _, photo := range m.photos {
		totalSize += photo.Size
		if photo.IsFavorite {
			favorites++
		}
		if photo.Scene != "" {
			sceneCount[photo.Scene]++
		}
		if photo.GPS != nil && photo.GPS.City != "" {
			locationCount[photo.GPS.City]++
		}
		if len(photo.Embedding) > 0 {
			embeddingCount++
		}
	}

	return map[string]interface{}{
		"totalPhotos":    len(m.photos),
		"totalFaces":     len(m.faces),
		"totalAlbums":    len(m.albums),
		"totalSize":      totalSize,
		"favorites":      favorites,
		"sceneCount":     sceneCount,
		"locationCount":  locationCount,
		"embeddingCount": embeddingCount,
	}
}

// ========== 辅助函数 ==========

// contains 简单字符串包含检查.
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

// appendUnique 追加唯一值.
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// cosineSimilarity 计算余弦相似度.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// haversineDistance 计算两点间的距离（米）.
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371000 // 地球半径（米）

	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

// calculateClusterRadius 根据缩放级别计算聚合半径.
func calculateClusterRadius(zoomLevel int) float64 {
	// 缩放级别越高，聚合半径越小
	switch {
	case zoomLevel >= 15:
		return 50 // 50米
	case zoomLevel >= 12:
		return 200 // 200米
	case zoomLevel >= 10:
		return 1000 // 1公里
	case zoomLevel >= 8:
		return 5000 // 5公里
	default:
		return 20000 // 20公里
	}
}

// ========== 批量操作 ==========

// BatchAddEmbeddings 批量添加嵌入向量.
func (m *Manager) BatchAddEmbeddings(embeddings map[string][]float64) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for photoID, embedding := range embeddings {
		if photo, ok := m.photos[photoID]; ok {
			photo.Embedding = embedding
			count++
		}
	}
	return count
}

// AutoTag 自动生成标签（基于场景和元数据）.
func (m *Manager) AutoTag(photoID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[photoID]
	if !ok {
		return nil, fmt.Errorf("photo not found: %s", photoID)
	}

	newTags := make([]string, 0)

	// 基于场景生成标签
	if photo.Scene != "" {
		newTags = append(newTags, photo.Scene)
	}

	// 基于时间生成标签
	hour := photo.ShotAt.Hour()
	switch {
	case hour >= 5 && hour < 8:
		newTags = append(newTags, "日出", "清晨")
	case hour >= 8 && hour < 12:
		newTags = append(newTags, "上午")
	case hour >= 12 && hour < 14:
		newTags = append(newTags, "中午")
	case hour >= 14 && hour < 18:
		newTags = append(newTags, "下午")
	case hour >= 18 && hour < 21:
		newTags = append(newTags, "傍晚", "日落")
	default:
		newTags = append(newTags, "夜晚")
	}

	// 基于季节生成标签
	month := photo.ShotAt.Month()
	switch {
	case month >= 3 && month <= 5:
		newTags = append(newTags, "春天")
	case month >= 6 && month <= 8:
		newTags = append(newTags, "夏天")
	case month >= 9 && month <= 11:
		newTags = append(newTags, "秋天")
	default:
		newTags = append(newTags, "冬天")
	}

	// 基于地点生成标签
	if photo.GPS != nil {
		if photo.GPS.City != "" {
			newTags = append(newTags, photo.GPS.City)
		}
	}

	// 去重并添加
	existingTags := make(map[string]bool)
	for _, tag := range photo.Tags {
		existingTags[tag] = true
	}

	for _, tag := range newTags {
		if !existingTags[tag] {
			photo.Tags = append(photo.Tags, tag)
			m.index[tag] = append(m.index[tag], photo.ID)
		}
	}

	return photo.Tags, nil
}
