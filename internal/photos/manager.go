// Package photos 智能相册管理模块
// 学习群晖 Photos / 飞牛相册的优秀功能
package photos

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager 相册管理器
type Manager struct {
	mu          sync.RWMutex
	photos      map[string]*Photo
	albums      map[string]*Album
	persons     map[string]*Person
	storagePath string
	indexPath   string
	photosDir   string       // 照片存储目录
	dataDir     string       // 数据存储目录
	thumbsDir   string       // 缩略图存储目录
	cacheDir    string       // 缓存存储目录
	config      *PhotoConfig // 配置
}

// NewManager 创建相册管理器
func NewManager(storagePath string) *Manager {
	m := &Manager{
		photos:      make(map[string]*Photo),
		albums:      make(map[string]*Album),
		persons:     make(map[string]*Person),
		storagePath: storagePath,
		indexPath:   filepath.Join(storagePath, ".index"),
		photosDir:   filepath.Join(storagePath, "photos"),
		dataDir:     filepath.Join(storagePath, "data"),
		thumbsDir:   filepath.Join(storagePath, "thumbnails"),
		cacheDir:    filepath.Join(storagePath, "cache"),
		config: &PhotoConfig{
			MaxUploadSize:    100 * 1024 * 1024,
			Enabled:          true,
			StoragePath:      storagePath,
			SupportedFormats: []string{".jpg", ".png", ".heic", ".mp4"},
			ThumbnailConfig: &ThumbnailConfig{
				Quality: 80,
			},
		},
	}
	go m.startAutoIndex()
	return m
}

// ImportPhoto 导入照片
func (m *Manager) ImportPhoto(ctx context.Context, filePath string, userID string) (*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	// 生成唯一ID
	photoID := generateID()

	// 提取元数据
	photo := &Photo{
		ID:         photoID,
		Filename:   filepath.Base(filePath),
		Path:       filePath,
		Size:       uint64(info.Size()),
		Format:     getFormat(filePath),
		UploadedAt: time.Now(),
		ModifiedAt: info.ModTime(),
		Tags:       []string{},
		Albums:     []string{},
		Faces:      []FaceInfo{},
		Comments:   []PhotoComment{},
	}

	// 尝试提取 EXIF 数据
	m.extractEXIF(photo, filePath)

	// 生成缩略图
	thumbnailPath, err := m.generateThumbnail(photoID, filePath)
	if err == nil {
		photo.Thumbnail = thumbnailPath
	}

	// 保存到内存和磁盘
	m.photos[photoID] = photo
	m.savePhotoIndex()

	log.Printf("Photo imported: %s (%s)", photo.Filename, photoID)
	return photo, nil
}

// CreateAlbum 创建相册
func (m *Manager) CreateAlbum(ctx context.Context, name string, description string, ownerID string) (*Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	albumID := generateID()
	album := &Album{
		ID:          albumID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		OwnerID:     ownerID,
		Tags:        []string{},
	}

	m.albums[albumID] = album
	m.saveAlbumIndex()

	log.Printf("Album created: %s (%s)", name, albumID)
	return album, nil
}

// AddPhotoToAlbum 添加照片到相册
func (m *Manager) AddPhotoToAlbum(ctx context.Context, photoID string, albumID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return fmt.Errorf("photo not found: %s", photoID)
	}

	album, exists := m.albums[albumID]
	if !exists {
		return fmt.Errorf("album not found: %s", albumID)
	}

	// 检查是否已在相册中
	for _, id := range photo.Albums {
		if id == albumID {
			return nil // 已存在
		}
	}

	photo.Albums = append(photo.Albums, albumID)
	album.PhotoCount++
	album.UpdatedAt = time.Now()

	if album.CoverPhoto == "" {
		album.CoverPhoto = photoID
	}

	return nil
}

// SearchPhotos 搜索照片
func (m *Manager) SearchPhotos(ctx context.Context, query SearchQuery) (*SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []Photo

	for _, photo := range m.photos {
		if m.matchesQuery(photo, query) {
			results = append(results, *photo)
		}
	}

	// 排序
	m.sortPhotos(results, query.SortBy, query.SortOrder)

	// 分页
	total := len(results)
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
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

	return &SearchResult{
		Photos:     results[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// RecognizeFaces 人脸识别
func (m *Manager) RecognizeFaces(ctx context.Context, photoID string) ([]FaceInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return nil, fmt.Errorf("photo not found: %s", photoID)
	}

	// TODO: 调用人脸识别服务
	// 这里返回模拟数据
	faces := []FaceInfo{
		{
			ID:   "face_001",
			Name: "Unknown",
			Bounds: Rectangle{
				X:      30,
				Y:      20,
				Width:  15,
				Height: 20,
			},
			Confidence: 0.95,
		},
	}

	photo.Faces = faces
	return faces, nil
}

// GetTimeline 获取时间线（按月/年分组）
func (m *Manager) GetTimeline(ctx context.Context, groupBy string) (map[string][]Photo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	timeline := make(map[string][]Photo)

	for _, photo := range m.photos {
		var key string
		switch groupBy {
		case "year":
			key = photo.TakenAt.Format("2006")
		case "month":
			key = photo.TakenAt.Format("2006-01")
		case "day":
			key = photo.TakenAt.Format("2006-01-02")
		default:
			key = photo.TakenAt.Format("2006-01")
		}
		timeline[key] = append(timeline[key], *photo)
	}

	return timeline, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats(ctx context.Context) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalSize := uint64(0)
	formatCount := make(map[string]int)
	deviceCount := make(map[string]int)

	for _, photo := range m.photos {
		totalSize += photo.Size
		formatCount[photo.Format]++
		if photo.DeviceModel != "" {
			deviceCount[photo.DeviceModel]++
		}
	}

	return map[string]interface{}{
		"total_photos":   len(m.photos),
		"total_albums":   len(m.albums),
		"total_persons":  len(m.persons),
		"total_size":     totalSize,
		"formats":        formatCount,
		"devices":        deviceCount,
	}, nil
}

// 内部方法

func (m *Manager) matchesQuery(photo *Photo, query SearchQuery) bool {
	// 关键词匹配
	if query.Keyword != "" {
		// 搜索文件名、标签、评论
		found := false
		if contains(photo.Filename, query.Keyword) {
			found = true
		}
		for _, tag := range photo.Tags {
			if contains(tag, query.Keyword) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 标签匹配
	if len(query.Tags) > 0 {
		hasTag := false
		for _, qt := range query.Tags {
			for _, pt := range photo.Tags {
				if pt == qt {
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

	// 相册匹配
	if query.AlbumID != "" {
		found := false
		for _, id := range photo.Albums {
			if id == query.AlbumID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 日期范围
	if query.DateFrom != nil && photo.TakenAt.Before(*query.DateFrom) {
		return false
	}
	if query.DateTo != nil && photo.TakenAt.After(*query.DateTo) {
		return false
	}

	// 评分
	if query.Rating > 0 && photo.Rating < query.Rating {
		return false
	}

	// 收藏
	if query.IsFavorite != nil && photo.IsFavorite != *query.IsFavorite {
		return false
	}

	// 格式
	if query.Format != "" && photo.Format != query.Format {
		return false
	}

	// 设备
	if query.DeviceModel != "" && photo.DeviceModel != query.DeviceModel {
		return false
	}

	return true
}

func (m *Manager) sortPhotos(photos []Photo, sortBy, sortOrder string) {
	// 简化实现，实际应使用 sort.Slice
}

func (m *Manager) extractEXIF(photo *Photo, filePath string) {
	// TODO: 提取 EXIF 数据
}

func (m *Manager) generateThumbnail(photoID, filePath string) (string, error) {
	// TODO: 生成缩略图
	thumbDir := filepath.Join(m.storagePath, "thumbnails")
	os.MkdirAll(thumbDir, 0755)
	return filepath.Join(thumbDir, photoID+".jpg"), nil
}

func (m *Manager) startAutoIndex() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		m.reindex()
	}
}

func (m *Manager) reindex() {
	// TODO: 重新索引照片
}

func (m *Manager) savePhotoIndex() {
	// TODO: 保存照片索引到磁盘
}

func (m *Manager) saveAlbumIndex() {
	// TODO: 保存相册索引到磁盘
}

// savePersons 保存人物数据到磁盘
func (m *Manager) savePersons() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.Marshal(m.persons)
	if err != nil {
		return fmt.Errorf("marshal persons failed: %w", err)
	}

	path := filepath.Join(m.dataDir, "persons.json")
	if err := os.MkdirAll(m.dataDir, 0750); err != nil {
		return fmt.Errorf("create data dir failed: %w", err)
	}

	return os.WriteFile(path, data, 0640)
}

func getFormat(filename string) string {
	ext := filepath.Ext(filename)
	if len(ext) > 0 {
		return ext[1:]
	}
	return ""
}

// Export 导出照片列表为 JSON
func (m *Manager) Export(ctx context.Context) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photos := make([]Photo, 0, len(m.photos))
	for _, p := range m.photos {
		photos = append(photos, *p)
	}

	return json.MarshalIndent(photos, "", "  ")
}

// DeletePhoto 删除照片
func (m *Manager) DeletePhoto(photoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.photos[photoID]; !exists {
		return fmt.Errorf("photo not found: %s", photoID)
	}

	delete(m.photos, photoID)
	return nil
}

// resizeDimensions 计算缩放后的尺寸，保持宽高比
func resizeDimensions(width, height, maxSize int) (int, int) {
	if width <= maxSize && height <= maxSize {
		return width, height
	}

	ratio := float64(width) / float64(height)
	if width > height {
		newWidth := maxSize
		newHeight := int(float64(newWidth) / ratio)
		return newWidth, newHeight
	}
	newHeight := maxSize
	newWidth := int(float64(newHeight) * ratio)
	return newWidth, newHeight
}

// GetPhoto 获取照片
func (m *Manager) GetPhoto(photoID string) (*Photo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return nil, fmt.Errorf("photo not found: %s", photoID)
	}
	return photo, nil
}

// GetAlbum 获取相册
func (m *Manager) GetAlbum(albumID string) (*Album, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	album, exists := m.albums[albumID]
	if !exists {
		return nil, fmt.Errorf("album not found: %s", albumID)
	}
	return album, nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *PhotoConfig {
	return &PhotoConfig{
		MaxUploadSize: 100 * 1024 * 1024, // 100MB
		Enabled:       true,
		StoragePath:   m.storagePath,
	}
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *PhotoConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	return nil
}

// CreatePerson 创建人物
func (m *Manager) CreatePerson(name string) (*Person, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	person := &Person{
		ID:   generateID(),
		Name: name,
	}
	m.persons[person.ID] = person
	return person, nil
}

// ListPersons 列出人物
func (m *Manager) ListPersons() []*Person {
	m.mu.RLock()
	defer m.mu.RUnlock()

	persons := make([]*Person, 0, len(m.persons))
	for _, p := range m.persons {
		persons = append(persons, p)
	}
	return persons
}

// UpdatePerson 更新人物
func (m *Manager) UpdatePerson(personID string, name string) (*Person, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	person, exists := m.persons[personID]
	if !exists {
		return nil, fmt.Errorf("person not found: %s", personID)
	}
	person.Name = name
	return person, nil
}

// DeletePerson 删除人物
func (m *Manager) DeletePerson(personID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.persons[personID]; !exists {
		return fmt.Errorf("person not found: %s", personID)
	}
	delete(m.persons, personID)
	return nil
}

// QueryPhotos 查询照片
func (m *Manager) QueryPhotos(query *PhotoQuery) ([]*Photo, int, error) {
	// 将 PhotoQuery 转换为 SearchQuery
	sq := SearchQuery{
		AlbumID:   query.AlbumID,
		UserID:    query.UserID,
		Tags:      query.Tags,
		Scene:     query.Scene,
		SortBy:    query.SortBy,
		SortOrder: query.SortOrder,
		Page:      query.Limit,
		PageSize:  query.Offset,
	}
	if query.StartDate != (time.Time{}) {
		sq.DateFrom = &query.StartDate
	}
	if query.EndDate != (time.Time{}) {
		sq.DateTo = &query.EndDate
	}
	result, err := m.SearchPhotos(context.Background(), sq)
	if err != nil {
		return nil, 0, err
	}
	photos := make([]*Photo, len(result.Photos))
	for i := range result.Photos {
		photos[i] = &result.Photos[i]
	}
	return photos, result.Total, nil
}

// ListAlbums 列出相册
func (m *Manager) ListAlbums(userID ...string) []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()

	albums := make([]*Album, 0, len(m.albums))
	for _, a := range m.albums {
		if len(userID) == 0 || a.OwnerID == userID[0] {
			albums = append(albums, a)
		}
	}
	return albums
}

// UpdateAlbum 更新相册
func (m *Manager) UpdateAlbum(albumID string, name string, description string) (*Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	album, exists := m.albums[albumID]
	if !exists {
		return nil, fmt.Errorf("album not found: %s", albumID)
	}
	if name != "" {
		album.Name = name
	}
	if description != "" {
		album.Description = description
	}
	return album, nil
}

// DeleteAlbum 删除相册
func (m *Manager) DeleteAlbum(albumID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.albums[albumID]; !exists {
		return fmt.Errorf("album not found: %s", albumID)
	}
	delete(m.albums, albumID)
	return nil
}

// RemovePhotoFromAlbum 从相册移除照片
func (m *Manager) RemovePhotoFromAlbum(ctx context.Context, photoID string, albumID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return fmt.Errorf("photo not found: %s", photoID)
	}

	for i, aid := range photo.Albums {
		if aid == albumID {
			photo.Albums = append(photo.Albums[:i], photo.Albums[i+1:]...)
			return nil
		}
	}
	return nil
}

// ToggleFavorite 切换收藏状态
func (m *Manager) ToggleFavorite(photoID string) (*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return nil, fmt.Errorf("photo not found: %s", photoID)
	}
	photo.IsFavorite = !photo.IsFavorite
	return photo, nil
}

// saveConfig 保存配置
func (m *Manager) saveConfig() error {
	// 简化实现 - 配置保存到内存
	return nil
}
