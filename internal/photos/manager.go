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
}

// NewManager 创建相册管理器
func NewManager(storagePath string) *Manager {
	m := &Manager{
		photos:      make(map[string]*Photo),
		albums:      make(map[string]*Album),
		persons:     make(map[string]*Person),
		storagePath: storagePath,
		indexPath:   filepath.Join(storagePath, ".index"),
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
		Size:       info.Size(),
		Format:     getFormat(filePath),
		UploadedAt: time.Now(),
		ModifiedAt: info.ModTime(),
		Tags:       []string{},
		Albums:     []string{},
		Faces:      []Face{},
		Comments:   []Comment{},
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
func (m *Manager) RecognizeFaces(ctx context.Context, photoID string) ([]Face, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return nil, fmt.Errorf("photo not found: %s", photoID)
	}

	// TODO: 调用人脸识别服务
	// 这里返回模拟数据
	faces := []Face{
		{
			PersonID:   "person_001",
			PersonName: "Unknown",
			X:          0.3,
			Y:          0.2,
			Width:      0.15,
			Height:     0.2,
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

	totalSize := int64(0)
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
	if !query.DateFrom.IsZero() && photo.TakenAt.Before(query.DateFrom) {
		return false
	}
	if !query.DateTo.IsZero() && photo.TakenAt.After(query.DateTo) {
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
