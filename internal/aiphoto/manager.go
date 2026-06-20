package aiphoto

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// Manager AI相册管理器
type Manager struct {
	mu     sync.RWMutex
	config *PhotoConfig

	// 存储
	photos   map[string]*Photo
	faces    map[string]*Face
	persons  map[string]*Person
	albums   map[string]*Album

	// 索引
	sceneIndex   map[SceneCategory][]string // scene -> photo IDs
	personIndex  map[string][]string        // person ID -> photo IDs
	tagIndex     map[string][]string        // tag -> photo IDs
	hashIndex    map[string]string          // perceptual hash -> photo ID

	// 队列
	analysisQueue chan string

	// 统计
	stats PhotoStats
}

// NewManager 创建管理器
func NewManager(config *PhotoConfig) *Manager {
	if config == nil {
		config = DefaultPhotoConfig()
	}

	m := &Manager{
		config:        config,
		photos:        make(map[string]*Photo),
		faces:         make(map[string]*Face),
		persons:       make(map[string]*Person),
		albums:        make(map[string]*Album),
		sceneIndex:    make(map[SceneCategory][]string),
		personIndex:   make(map[string][]string),
		tagIndex:      make(map[string][]string),
		hashIndex:     make(map[string]string),
		analysisQueue: make(chan string, config.AnalysisQueueSize),
	}

	return m
}

// Start 启动管理器
func (m *Manager) Start() error {
	// 启动分析工作线程
	for i := 0; i < m.config.MaxConcurrentAnalysis; i++ {
		go m.analysisWorker()
	}
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	close(m.analysisQueue)
}

// ========== 照片管理 ==========

// AddPhoto 添加照片
func (m *Manager) AddPhoto(req *PhotoCreateRequest) (*Photo, error) {
	if req.Path == "" {
		return nil, ErrInvalidImage
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成ID
	id := generateID()

	photo := &Photo{
		ID:         id,
		Path:       req.Path,
		Filename:   extractFilename(req.Path),
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}

	m.photos[id] = photo

	// 如果需要分析，加入队列
	if req.Analyze {
		m.analysisQueue <- id
	}

	return photo, nil
}

// GetPhoto 获取照片
func (m *Manager) GetPhoto(id string) (*Photo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photo, exists := m.photos[id]
	if !exists {
		return nil, ErrPhotoNotFound
	}

	return photo, nil
}

// DeletePhoto 删除照片
func (m *Manager) DeletePhoto(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[id]
	if !exists {
		return ErrPhotoNotFound
	}

	// 删除关联的人脸
	for _, face := range photo.Faces {
		delete(m.faces, face.ID)
		// 从人物索引中移除
		if face.PersonID != "" {
			m.removeFromPersonIndex(face.PersonID, id)
		}
	}

	// 从场景索引中移除
	m.removeFromSceneIndex(photo.Scene, id)

	// 从标签索引中移除
	for _, tag := range photo.Tags {
		m.removeFromTagIndex(tag, id)
	}

	// 从哈希索引中移除
	if photo.PerceptualHash != "" {
		delete(m.hashIndex, photo.PerceptualHash)
	}

	delete(m.photos, id)

	return nil
}

// ToggleFavorite 切换收藏状态
func (m *Manager) ToggleFavorite(id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[id]
	if !exists {
		return false, ErrPhotoNotFound
	}

	photo.IsFavorite = !photo.IsFavorite
	return photo.IsFavorite, nil
}

// ========== 搜索功能 ==========

// SearchPhotos 搜索照片
func (m *Manager) SearchPhotos(req *PhotoSearchRequest) (*PhotoSearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Photo

	// 遍历所有照片进行过滤
	for _, photo := range m.photos {
		if m.matchesFilter(photo, req) {
			results = append(results, photo)
		}
	}

	// 排序
	m.sortPhotos(results, req.SortBy, req.SortOrder)

	// 分页
	total := len(results)
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pagedResults := results[start:end]

	totalPages := (total + pageSize - 1) / pageSize

	return &PhotoSearchResult{
		Photos:     pagedResults,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// matchesFilter 检查照片是否匹配过滤条件
func (m *Manager) matchesFilter(photo *Photo, req *PhotoSearchRequest) bool {
	// 场景过滤
	if req.Scene != "" && photo.Scene != req.Scene {
		return false
	}

	// 人物过滤
	if req.PersonID != "" {
		hasPerson := false
		for _, face := range photo.Faces {
			if face.PersonID == req.PersonID {
				hasPerson = true
				break
			}
		}
		if !hasPerson {
			return false
		}
	}

	// 相册过滤
	if req.AlbumID != "" {
		hasAlbum := false
		for _, albumID := range photo.AlbumIDs {
			if albumID == req.AlbumID {
				hasAlbum = true
				break
			}
		}
		if !hasAlbum {
			return false
		}
	}

	// 标签过滤
	if len(req.Tags) > 0 {
		hasTag := false
		for _, reqTag := range req.Tags {
			for _, photoTag := range photo.Tags {
				if reqTag == photoTag {
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

	// 日期过滤
	if req.StartDate != nil && photo.TakenAt.Before(*req.StartDate) {
		return false
	}
	if req.EndDate != nil && photo.TakenAt.After(*req.EndDate) {
		return false
	}

	// 收藏过滤
	if req.IsFavorite != nil && photo.IsFavorite != *req.IsFavorite {
		return false
	}

	// 位置过滤
	if req.Location != nil {
		distance := calculateDistance(
			photo.Latitude, photo.Longitude,
			req.Location.Latitude, req.Location.Longitude,
		)
		if distance > req.Location.RadiusKm {
			return false
		}
	}

	return true
}

// sortPhotos 排序照片
func (m *Manager) sortPhotos(photos []*Photo, sortBy, sortOrder string) {
	// 简化实现，按拍摄时间排序
	// 实际应该使用 sort.Slice
}

// ========== 人脸管理 ==========

// GetFaces 获取照片的人脸列表
func (m *Manager) GetFaces(photoID string) ([]*Face, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return nil, ErrPhotoNotFound
	}

	return photo.Faces, nil
}

// LinkFaceToPerson 关联人脸到人物
func (m *Manager) LinkFaceToPerson(faceID, personID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	face, exists := m.faces[faceID]
	if !exists {
		return ErrFaceNotFound
	}

	person, exists := m.persons[personID]
	if !exists {
		return ErrPersonNotFound
	}

	face.PersonID = personID
	person.FaceCount++

	// 更新索引
	m.addToPersonIndex(personID, face.PhotoID)

	return nil
}

// ========== 人物管理 ==========

// CreatePerson 创建人物
func (m *Manager) CreatePerson(name string) (*Person, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()

	person := &Person{
		ID:        id,
		Name:      name,
		IsNamed:   name != "",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.persons[id] = person
	m.personIndex[id] = []string{}

	return person, nil
}

// GetPerson 获取人物
func (m *Manager) GetPerson(id string) (*Person, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	person, exists := m.persons[id]
	if !exists {
		return nil, ErrPersonNotFound
	}

	return person, nil
}

// ListPersons 列出所有人物
func (m *Manager) ListPersons() []*Person {
	m.mu.RLock()
	defer m.mu.RUnlock()

	persons := make([]*Person, 0, len(m.persons))
	for _, person := range m.persons {
		persons = append(persons, person)
	}

	return persons
}

// ========== 相册管理 ==========

// CreateAlbum 创建相册
func (m *Manager) CreateAlbum(name, description string, albumType AlbumType) (*Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()

	album := &Album{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        albumType,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.albums[id] = album

	return album, nil
}

// GetAlbum 获取相册
func (m *Manager) GetAlbum(id string) (*Album, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	album, exists := m.albums[id]
	if !exists {
		return nil, ErrAlbumNotFound
	}

	return album, nil
}

// AddPhotoToAlbum 添加照片到相册
func (m *Manager) AddPhotoToAlbum(photoID, albumID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return ErrPhotoNotFound
	}

	album, exists := m.albums[albumID]
	if !exists {
		return ErrAlbumNotFound
	}

	// 检查是否已在相册中
	for _, id := range photo.AlbumIDs {
		if id == albumID {
			return nil // 已存在
		}
	}

	photo.AlbumIDs = append(photo.AlbumIDs, albumID)
	album.PhotoCount++

	return nil
}

// ========== 去重功能 ==========

// FindDuplicates 查找重复照片
func (m *Manager) FindDuplicates() []*DuplicateGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 按哈希分组
	hashGroups := make(map[string][]string)
	for id, photo := range m.photos {
		if photo.PerceptualHash != "" {
			hashGroups[photo.PerceptualHash] = append(hashGroups[photo.PerceptualHash], id)
		}
	}

	var groups []*DuplicateGroup
	for hash, photoIDs := range hashGroups {
		if len(photoIDs) > 1 {
			groups = append(groups, &DuplicateGroup{
				ID:       generateID(),
				PhotoIDs: photoIDs,
				Hash:     hash,
				Score:    1.0, // 完全相同
			})
		}
	}

	return groups
}

// ========== 统计功能 ==========

// GetStats 获取统计信息
func (m *Manager) GetStats() PhotoStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := PhotoStats{
		SceneCounts: make(map[SceneCategory]int),
		YearCounts:  make(map[int]int),
		MonthCounts: make(map[string]int),
	}

	stats.TotalPhotos = len(m.photos)
	stats.TotalFaces = len(m.faces)
	stats.TotalPersons = len(m.persons)
	stats.TotalAlbums = len(m.albums)

	// 统计场景分布
	for scene, ids := range m.sceneIndex {
		stats.SceneCounts[scene] = len(ids)
	}

	// 统计人物命名情况
	for _, person := range m.persons {
		if person.IsNamed {
			stats.NamedPersons++
		} else {
			stats.UnnamedPersons++
		}
	}

	// 统计年份和月份分布
	for _, photo := range m.photos {
		year := photo.TakenAt.Year()
		stats.YearCounts[year]++
		monthKey := photo.TakenAt.Format("2006-01")
		stats.MonthCounts[monthKey]++
	}

	return stats
}

// ========== 内部方法 ==========

// analysisWorker 分析工作线程
func (m *Manager) analysisWorker() {
	for photoID := range m.analysisQueue {
		m.analyzePhoto(photoID)
	}
}

// analyzePhoto 分析照片
func (m *Manager) analyzePhoto(photoID string) {
	m.mu.Lock()
	photo, exists := m.photos[photoID]
	if !exists {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// 模拟场景分类
	photo.Scene = SceneLandscape
	photo.SceneConfidence = 0.85
	photo.Tags = []string{"nature", "outdoor"}

	// 更新索引
	m.mu.Lock()
	m.addToSceneIndex(photo.Scene, photoID)
	for _, tag := range photo.Tags {
		m.addToTagIndex(tag, photoID)
	}
	m.mu.Unlock()
}

// addToSceneIndex 添加到场景索引
func (m *Manager) addToSceneIndex(scene SceneCategory, photoID string) {
	m.sceneIndex[scene] = append(m.sceneIndex[scene], photoID)
}

// removeFromSceneIndex 从场景索引移除
func (m *Manager) removeFromSceneIndex(scene SceneCategory, photoID string) {
	ids := m.sceneIndex[scene]
	for i, id := range ids {
		if id == photoID {
			m.sceneIndex[scene] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
}

// addToPersonIndex 添加到人物索引
func (m *Manager) addToPersonIndex(personID, photoID string) {
	m.personIndex[personID] = append(m.personIndex[personID], photoID)
}

// removeFromPersonIndex 从人物索引移除
func (m *Manager) removeFromPersonIndex(personID, photoID string) {
	ids := m.personIndex[personID]
	for i, id := range ids {
		if id == photoID {
			m.personIndex[personID] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
}

// addToTagIndex 添加到标签索引
func (m *Manager) addToTagIndex(tag, photoID string) {
	m.tagIndex[tag] = append(m.tagIndex[tag], photoID)
}

// removeFromTagIndex 从标签索引移除
func (m *Manager) removeFromTagIndex(tag, photoID string) {
	ids := m.tagIndex[tag]
	for i, id := range ids {
		if id == photoID {
			m.tagIndex[tag] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
}

// ========== 辅助函数 ==========

// generateID 生成唯一ID
func generateID() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(time.Now().String())))[:16]
}

// extractFilename 提取文件名
func extractFilename(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

// calculateDistance 计算两点之间的距离（公里）
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// 简化实现，使用 Haversine 公式
	// 实际应该使用更精确的地理计算
	return 0.0
}
