package aiphotoalbum

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// 错误定义
var (
	ErrInvalidPhotoID      = errors.New("invalid photo ID")
	ErrPhotoNotFound       = errors.New("photo not found")
	ErrPersonNotFound      = errors.New("person not found")
	ErrTagNotFound         = errors.New("tag not found")
	ErrInvalidAlbumID      = errors.New("invalid album ID")
	ErrAlbumNotFound       = errors.New("album not found")
	ErrInvalidPersonID     = errors.New("invalid person ID")
	ErrTextSearchDisabled  = errors.New("text search is disabled")
	ErrFaceDetectionDisabled = errors.New("face detection is disabled")
	ErrFaceNotRecognized   = errors.New("face not recognized")
)

// AlbumConfig 相册配置
type AlbumConfig struct {
	FaceDetectionEnabled  bool          `json:"face_detection_enabled"`
	TextSearchEnabled     bool          `json:"text_search_enabled"`
	AutoClassification    bool          `json:"auto_classification"`
	MaxConcurrentWorkers  int           `json:"max_concurrent_workers"`
	ThumbnailSize         int           `json:"thumbnail_size"`
	FaceMatchThreshold    float64       `json:"face_match_threshold"`
	TextSearchModel       string        `json:"text_search_model"`
	CacheExpiration       time.Duration `json:"cache_expiration"`
}

// DefaultAlbumConfig 默认配置
func DefaultAlbumConfig() *AlbumConfig {
	return &AlbumConfig{
		FaceDetectionEnabled:  true,
		TextSearchEnabled:     true,
		AutoClassification:    true,
		MaxConcurrentWorkers:  4,
		ThumbnailSize:         300,
		FaceMatchThreshold:    0.85,
		TextSearchModel:       "clip-vit-base",
		CacheExpiration:       24 * time.Hour,
	}
}

// Photo 照片元数据
type Photo struct {
	ID          string            `json:"id"`
	FilePath    string            `json:"file_path"`
	FileName    string            `json:"file_name"`
	FileSize    int64             `json:"file_size"`
	MimeType    string            `json:"mime_type"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	ShotAt      time.Time         `json:"shot_at"`
	UploadedAt  time.Time         `json:"uploaded_at"`
	GPS         *GPSInfo          `json:"gps,omitempty"`
	Camera      string            `json:"camera,omitempty"`
	Tags        []string          `json:"tags"`
	Faces       []FaceDetection   `json:"faces,omitempty"`
	Embedding   []float32         `json:"embedding,omitempty"`
	Labels      []string          `json:"labels"`
	IsFavorite  bool              `json:"is_favorite"`
	IsHidden    bool              `json:"is_hidden"`
	AlbumIDs    []string          `json:"album_ids"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// GPSInfo GPS信息
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// FaceDetection 人脸检测结果
type FaceDetection struct {
	PersonID    string    `json:"person_id"`
	PersonName  string    `json:"person_name,omitempty"`
	BoundingBox Rect      `json:"bounding_box"`
	Confidence  float64   `json:"confidence"`
	Embedding   []float32 `json:"embedding,omitempty"`
	Age         int       `json:"age,omitempty"`
	Gender      string    `json:"gender,omitempty"`
	Expression  string    `json:"expression,omitempty"`
}

// Rect 矩形区域
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Person 人物信息
type Person struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	FaceCount   int       `json:"face_count"`
	PhotoCount  int       `json:"photo_count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Embedding   []float32 `json:"embedding,omitempty"`
	IsConfirmed bool      `json:"is_confirmed"`
}

// Album 相册
type Album struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CoverURL    string    `json:"cover_url,omitempty"`
	PhotoCount  int       `json:"photo_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	IsAuto      bool      `json:"is_auto"`
	AutoRule    string    `json:"auto_rule,omitempty"`
	OwnerID     string    `json:"owner_id"`
	SharedWith  []string  `json:"shared_with,omitempty"`
}

// TextSearchQuery 文本搜索查询
type TextSearchQuery struct {
	Text      string     `json:"text"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	PersonID  string     `json:"person_id,omitempty"`
	AlbumID   string     `json:"album_id,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
	SortBy    string     `json:"sort_by,omitempty"`
	SortOrder string     `json:"sort_order,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Photos     []*Photo `json:"photos"`
	TotalCount int      `json:"total_count"`
	Query      string   `json:"query"`
	Duration   int64    `json:"duration_ms"`
}

// PhotoEngine AI相册引擎
type PhotoEngine struct {
	mu           sync.RWMutex
	config       *AlbumConfig
	photos       map[string]*Photo
	persons      map[string]*Person
	albums       map[string]*Album
	faceIndex    map[string][]string
	tagIndex     map[string][]string
	embeddingIdx *EmbeddingIndex
	running      bool
	stopCh       chan struct{}
	stats        *EngineStats
}

// EngineStats 引擎统计
type EngineStats struct {
	TotalPhotos      int64     `json:"total_photos"`
	TotalPersons     int64     `json:"total_persons"`
	TotalAlbums      int64     `json:"total_albums"`
	TotalFaces       int64     `json:"total_faces"`
	IndexSize        int64     `json:"index_size"`
	SearchCount      int64     `json:"search_count"`
	AvgSearchTime    float64   `json:"avg_search_time_ms"`
	LastIndexedAt    time.Time `json:"last_indexed_at"`
	StorageUsedBytes int64     `json:"storage_used_bytes"`
}

// EmbeddingIndex 向量索引
type EmbeddingIndex struct {
	mu        sync.RWMutex
	vectors   map[string][]float32
	dimension int
}

// NewPhotoEngine 创建相册引擎
func NewPhotoEngine(config *AlbumConfig) *PhotoEngine {
	if config == nil {
		config = DefaultAlbumConfig()
	}
	return &PhotoEngine{
		config:    config,
		photos:    make(map[string]*Photo),
		persons:   make(map[string]*Person),
		albums:    make(map[string]*Album),
		faceIndex: make(map[string][]string),
		tagIndex:  make(map[string][]string),
		embeddingIdx: &EmbeddingIndex{
			vectors:   make(map[string][]float32),
			dimension: 512,
		},
		stats: &EngineStats{},
	}
}

// Start 启动引擎
func (pe *PhotoEngine) Start() error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if pe.running {
		return nil
	}

	pe.running = true
	pe.stopCh = make(chan struct{})
	return nil
}

// Stop 停止引擎
func (pe *PhotoEngine) Stop() error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if !pe.running {
		return nil
	}

	close(pe.stopCh)
	pe.running = false
	return nil
}

// AddPhoto 添加照片
func (pe *PhotoEngine) AddPhoto(photo *Photo) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if photo.ID == "" {
		return ErrInvalidPhotoID
	}

	photo.UploadedAt = time.Now()
	pe.photos[photo.ID] = photo

	// 更新索引
	if len(photo.Embedding) > 0 {
		pe.embeddingIdx.mu.Lock()
		pe.embeddingIdx.vectors[photo.ID] = photo.Embedding
		pe.embeddingIdx.mu.Unlock()
	}

	for _, tag := range photo.Tags {
		pe.tagIndex[tag] = append(pe.tagIndex[tag], photo.ID)
	}

	for _, face := range photo.Faces {
		if face.PersonID != "" {
			pe.faceIndex[face.PersonID] = append(pe.faceIndex[face.PersonID], photo.ID)
		}
	}

	pe.stats.TotalPhotos++
	return nil
}

// RemovePhoto 删除照片
func (pe *PhotoEngine) RemovePhoto(photoID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	photo, exists := pe.photos[photoID]
	if !exists {
		return ErrPhotoNotFound
	}

	// 清理索引
	pe.embeddingIdx.mu.Lock()
	delete(pe.embeddingIdx.vectors, photoID)
	pe.embeddingIdx.mu.Unlock()

	for _, tag := range photo.Tags {
		ids := pe.tagIndex[tag]
		for i, id := range ids {
			if id == photoID {
				pe.tagIndex[tag] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	for _, face := range photo.Faces {
		if face.PersonID != "" {
			ids := pe.faceIndex[face.PersonID]
			for i, id := range ids {
				if id == photoID {
					pe.faceIndex[face.PersonID] = append(ids[:i], ids[i+1:]...)
					break
				}
			}
		}
	}

	delete(pe.photos, photoID)
	pe.stats.TotalPhotos--
	return nil
}

// TextSearch 以文搜图 - 对标飞牛"以文搜图"功能
func (pe *PhotoEngine) TextSearch(query *TextSearchQuery) (*SearchResult, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	start := time.Now()

	if !pe.config.TextSearchEnabled {
		return nil, ErrTextSearchDisabled
	}

	if query.Limit <= 0 {
		query.Limit = 50
	}

	// 基于向量相似度搜索
	var results []*Photo

	if len(query.Text) > 0 {
		// 获取查询文本的向量
		queryEmbedding := pe.getTextEmbedding(query.Text)
		if queryEmbedding == nil {
			// 回退到标签匹配
			results = pe.searchByTags(query)
		} else {
			results = pe.searchByEmbedding(queryEmbedding, query)
		}
	} else {
		// 无文本查询，按时间排序
		for _, photo := range pe.photos {
			if pe.matchesFilter(photo, query) {
				results = append(results, photo)
			}
		}
	}

	// 排序
	pe.sortPhotos(results, query.SortBy, query.SortOrder)

	// 分页
	totalCount := len(results)
	startIdx := query.Offset
	if startIdx > len(results) {
		startIdx = len(results)
	}
	endIdx := startIdx + query.Limit
	if endIdx > len(results) {
		endIdx = len(results)
	}

	pe.stats.SearchCount++
	duration := time.Since(start).Milliseconds()

	return &SearchResult{
		Photos:     results[startIdx:endIdx],
		TotalCount: totalCount,
		Query:      query.Text,
		Duration:   duration,
	}, nil
}

// searchByEmbedding 向量相似度搜索
func (pe *PhotoEngine) searchByEmbedding(queryEmbedding []float32, query *TextSearchQuery) []*Photo {
	type scoredPhoto struct {
		photo *Photo
		score float64
	}

	var scored []scoredPhoto

	pe.embeddingIdx.mu.RLock()
	defer pe.embeddingIdx.mu.RUnlock()

	for photoID, embedding := range pe.embeddingIdx.vectors {
		photo, exists := pe.photos[photoID]
		if !exists {
			continue
		}

		if !pe.matchesFilter(photo, query) {
			continue
		}

		score := cosineSimilarity(queryEmbedding, embedding)
		if score > 0.5 {
			scored = append(scored, scoredPhoto{photo: photo, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	results := make([]*Photo, len(scored))
	for i, s := range scored {
		results[i] = s.photo
	}

	return results
}

// searchByTags 标签匹配搜索
func (pe *PhotoEngine) searchByTags(query *TextSearchQuery) []*Photo {
	keywords := extractKeywords(query.Text)

	photoScores := make(map[string]int)

	for _, keyword := range keywords {
		for tag, photoIDs := range pe.tagIndex {
			if containsIgnoreCase(tag, keyword) {
				for _, photoID := range photoIDs {
					photoScores[photoID]++
				}
			}
		}

		for _, photo := range pe.photos {
			if containsIgnoreCase(photo.FileName, keyword) {
				photoScores[photo.ID]++
			}
			if containsIgnoreCase(photo.Camera, keyword) {
				photoScores[photo.ID]++
			}
			if photo.GPS != nil && containsIgnoreCase(photo.GPS.Address, keyword) {
				photoScores[photo.ID]++
			}
		}
	}

	type scoredPhoto struct {
		photo *Photo
		score int
	}

	var scored []scoredPhoto
	for photoID, score := range photoScores {
		if photo, exists := pe.photos[photoID]; exists {
			if pe.matchesFilter(photo, query) {
				scored = append(scored, scoredPhoto{photo: photo, score: score})
			}
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	results := make([]*Photo, len(scored))
	for i, s := range scored {
		results[i] = s.photo
	}

	return results
}

// matchesFilter 检查照片是否匹配过滤条件
func (pe *PhotoEngine) matchesFilter(photo *Photo, query *TextSearchQuery) bool {
	if photo.IsHidden {
		return false
	}

	if query.StartDate != nil && photo.ShotAt.Before(*query.StartDate) {
		return false
	}
	if query.EndDate != nil && photo.ShotAt.After(*query.EndDate) {
		return false
	}

	if query.PersonID != "" {
		found := false
		for _, face := range photo.Faces {
			if face.PersonID == query.PersonID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if query.AlbumID != "" {
		found := false
		for _, albumID := range photo.AlbumIDs {
			if albumID == query.AlbumID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(query.Tags) > 0 {
		photoTags := make(map[string]bool)
		for _, tag := range photo.Tags {
			photoTags[tag] = true
		}
		for _, tag := range query.Tags {
			if !photoTags[tag] {
				return false
			}
		}
	}

	return true
}

// sortPhotos 排序照片
func (pe *PhotoEngine) sortPhotos(photos []*Photo, sortBy, sortOrder string) {
	sort.Slice(photos, func(i, j int) bool {
		if sortOrder == "asc" {
			return photos[i].ShotAt.Before(photos[j].ShotAt)
		}
		return photos[i].ShotAt.After(photos[j].ShotAt)
	})
}

// GetPhotosByPerson 获取人物照片
func (pe *PhotoEngine) GetPhotosByPerson(personID string) ([]*Photo, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	photoIDs, exists := pe.faceIndex[personID]
	if !exists {
		return nil, ErrPersonNotFound
	}

	photos := make([]*Photo, 0, len(photoIDs))
	for _, photoID := range photoIDs {
		if photo, exists := pe.photos[photoID]; exists {
			photos = append(photos, photo)
		}
	}

	return photos, nil
}

// GetPhotosByTag 获取标签照片
func (pe *PhotoEngine) GetPhotosByTag(tag string) ([]*Photo, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	photoIDs, exists := pe.tagIndex[tag]
	if !exists {
		return nil, ErrTagNotFound
	}

	photos := make([]*Photo, 0, len(photoIDs))
	for _, photoID := range photoIDs {
		if photo, exists := pe.photos[photoID]; exists {
			photos = append(photos, photo)
		}
	}

	return photos, nil
}

// CreateAlbum 创建相册
func (pe *PhotoEngine) CreateAlbum(album *Album) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if album.ID == "" {
		return ErrInvalidAlbumID
	}

	album.CreatedAt = time.Now()
	album.UpdatedAt = time.Now()
	pe.albums[album.ID] = album
	pe.stats.TotalAlbums++

	return nil
}

// AddPhotosToAlbum 添加照片到相册
func (pe *PhotoEngine) AddPhotosToAlbum(albumID string, photoIDs []string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	album, exists := pe.albums[albumID]
	if !exists {
		return ErrAlbumNotFound
	}

	for _, photoID := range photoIDs {
		if photo, exists := pe.photos[photoID]; exists {
			found := false
			for _, id := range photo.AlbumIDs {
				if id == albumID {
					found = true
					break
				}
			}
			if !found {
				photo.AlbumIDs = append(photo.AlbumIDs, albumID)
				album.PhotoCount++
			}
		}
	}

	album.UpdatedAt = time.Now()
	return nil
}

// RegisterPerson 注册人物
func (pe *PhotoEngine) RegisterPerson(person *Person) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if person.ID == "" {
		return ErrInvalidPersonID
	}

	person.FirstSeen = time.Now()
	person.LastSeen = time.Now()
	pe.persons[person.ID] = person
	pe.stats.TotalPersons++

	return nil
}

// RecognizeFace 人脸识别
func (pe *PhotoEngine) RecognizeFace(embedding []float32) (*Person, float64, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	if !pe.config.FaceDetectionEnabled {
		return nil, 0, ErrFaceDetectionDisabled
	}

	var bestPerson *Person
	var bestScore float64

	for _, person := range pe.persons {
		if len(person.Embedding) == 0 {
			continue
		}

		score := cosineSimilarity(embedding, person.Embedding)
		if score > bestScore && score >= pe.config.FaceMatchThreshold {
			bestScore = score
			bestPerson = person
		}
	}

	if bestPerson == nil {
		return nil, 0, ErrFaceNotRecognized
	}

	return bestPerson, bestScore, nil
}

// GetStats 获取统计信息
func (pe *PhotoEngine) GetStats() *EngineStats {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	stats := *pe.stats
	stats.TotalPersons = int64(len(pe.persons))
	stats.TotalAlbums = int64(len(pe.albums))

	pe.embeddingIdx.mu.RLock()
	stats.IndexSize = int64(len(pe.embeddingIdx.vectors))
	pe.embeddingIdx.mu.RUnlock()

	return &stats
}

// getTextEmbedding 获取文本向量 (简化实现)
func (pe *PhotoEngine) getTextEmbedding(text string) []float32 {
	return nil
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	return x * 0.5
}

// extractKeywords 提取关键词
func extractKeywords(text string) []string {
	return []string{text}
}

// containsIgnoreCase 忽略大小写包含检查
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
