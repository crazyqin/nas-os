// Package phototimeline provides photo timeline management for NAS-OS.
package phototimeline

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// AlbumManager 相册管理器
type AlbumManager struct {
	mu       sync.RWMutex
	albums   map[string]*Album
	photos   map[string]*Photo // 共享照片存储引用
	config   Config
}

// NewAlbumManager 创建相册管理器
func NewAlbumManager(config Config, photos map[string]*Photo) *AlbumManager {
	return &AlbumManager{
		albums: make(map[string]*Album),
		photos: photos,
		config: config,
	}
}

// CreateAlbum 创建相册
func (am *AlbumManager) CreateAlbum(album *Album) error {
	if album == nil {
		return fmt.Errorf("album cannot be nil")
	}
	if album.Name == "" {
		return fmt.Errorf("album name is required")
	}
	if album.ID == "" {
		return fmt.Errorf("album ID is required")
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.albums[album.ID]; exists {
		return fmt.Errorf("album already exists: %s", album.ID)
	}

	album.CreatedAt = time.Now()
	album.UpdatedAt = time.Now()
	am.albums[album.ID] = album

	return nil
}

// GetAlbum 获取相册
func (am *AlbumManager) GetAlbum(id string) (*Album, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	album, exists := am.albums[id]
	if !exists {
		return nil, fmt.Errorf("album not found: %s", id)
	}

	return album, nil
}

// UpdateAlbum 更新相册
func (am *AlbumManager) UpdateAlbum(album *Album) error {
	if album == nil || album.ID == "" {
		return fmt.Errorf("invalid album")
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.albums[album.ID]; !exists {
		return fmt.Errorf("album not found: %s", album.ID)
	}

	album.UpdatedAt = time.Now()
	am.albums[album.ID] = album
	return nil
}

// DeleteAlbum 删除相册
func (am *AlbumManager) DeleteAlbum(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.albums[id]; !exists {
		return fmt.Errorf("album not found: %s", id)
	}

	delete(am.albums, id)
	return nil
}

// ListAlbums 列出所有相册
func (am *AlbumManager) ListAlbums(albumType AlbumType) []Album {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []Album
	for _, a := range am.albums {
		if albumType == "" || a.Type == albumType {
			result = append(result, *a)
		}
	}

	return result
}

// AddPhotosToAlbum 添加照片到相册
func (am *AlbumManager) AddPhotosToAlbum(albumID string, photoIDs []string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	album, exists := am.albums[albumID]
	if !exists {
		return fmt.Errorf("album not found: %s", albumID)
	}

	// 验证照片存在
	for _, pid := range photoIDs {
		if _, exists := am.photos[pid]; !exists {
			return fmt.Errorf("photo not found: %s", pid)
		}
	}

	// 添加到相册
	for _, pid := range photoIDs {
		photo := am.photos[pid]
		if !contains(photo.Albums, albumID) {
			photo.Albums = append(photo.Albums, albumID)
		}
	}

	album.PhotoCount += len(photoIDs)
	album.UpdatedAt = time.Now()

	return nil
}

// RemovePhotosFromAlbum 从相册移除照片
func (am *AlbumManager) RemovePhotosFromAlbum(albumID string, photoIDs []string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	album, exists := am.albums[albumID]
	if !exists {
		return fmt.Errorf("album not found: %s", albumID)
	}

	for _, pid := range photoIDs {
		photo, exists := am.photos[pid]
		if !exists {
			continue
		}
		photo.Albums = removeFromSlice(photo.Albums, albumID)
		album.PhotoCount--
	}

	if album.PhotoCount < 0 {
		album.PhotoCount = 0
	}
	album.UpdatedAt = time.Now()

	return nil
}

// GetAlbumPhotos 获取相册照片
func (am *AlbumManager) GetAlbumPhotos(albumID string, page, pageSize int) (*SearchResult, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}

	am.mu.RLock()
	defer am.mu.RUnlock()

	// 检查相册存在
	if _, exists := am.albums[albumID]; !exists {
		return nil, fmt.Errorf("album not found: %s", albumID)
	}

	// 收集相册照片
	var photos []Photo
	for _, p := range am.photos {
		if contains(p.Albums, albumID) && !p.Trashed {
			photos = append(photos, *p)
		}
	}

	// 按拍摄时间排序
	sortPhotosByDate(photos)

	total := len(photos)
	start := (page - 1) * pageSize
	if start >= total {
		return &SearchResult{
			Photos:   []Photo{},
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			HasMore:  false,
		}, nil
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return &SearchResult{
		Photos:   photos[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  end < total,
	}, nil
}

// UpdateSmartAlbums 更新智能相册
func (am *AlbumManager) UpdateSmartAlbums() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, album := range am.albums {
		if album.Rules == nil {
			continue
		}

		// 清空旧的关联
		for _, p := range am.photos {
			p.Albums = removeFromSlice(p.Albums, album.ID)
		}
		album.PhotoCount = 0

		// 重新匹配
		for _, p := range am.photos {
			if am.matchesRules(p, album.Rules) {
				p.Albums = append(p.Albums, album.ID)
				album.PhotoCount++
			}
		}
	}

	return nil
}

// matchesRules 检查照片是否匹配规则
func (am *AlbumManager) matchesRules(photo *Photo, rules *AlbumRules) bool {
	if photo.Trashed {
		return false
	}

	matches := make([]bool, 0)

	// 日期范围
	if rules.DateFrom != nil {
		matches = append(matches, !photo.TakenAt.Before(*rules.DateFrom))
	}
	if rules.DateTo != nil {
		matches = append(matches, !photo.TakenAt.After(*rules.DateTo))
	}

	// 地点范围
	if rules.LocationCenter != nil && rules.LocationRadius > 0 {
		if photo.Latitude != 0 && photo.Longitude != 0 {
			dist := haversineDistance(
				rules.LocationCenter.Latitude,
				rules.LocationCenter.Longitude,
				photo.Latitude,
				photo.Longitude,
			)
			matches = append(matches, dist <= rules.LocationRadius)
		} else {
			matches = append(matches, false)
		}
	}

	// 标签匹配
	if len(rules.Tags) > 0 {
		matches = append(matches, containsAny(photo.Tags, rules.Tags))
	}
	if len(rules.Labels) > 0 {
		matches = append(matches, containsAny(photo.Labels, rules.Labels))
	}
	if len(rules.People) > 0 {
		matches = append(matches, containsAny(photo.People, rules.People))
	}

	// 相机型号
	if rules.CameraMake != "" {
		matches = append(matches, photo.EXIF.CameraMake == rules.CameraMake)
	}
	if rules.CameraModel != "" {
		matches = append(matches, photo.EXIF.CameraModel == rules.CameraModel)
	}

	// 评分
	if rules.MinRating > 0 {
		matches = append(matches, photo.Rating >= rules.MinRating)
	}

	// 文件类型
	if len(rules.MimeTypes) > 0 {
		matches = append(matches, contains(rules.MimeTypes, photo.MimeType))
	}

	// 根据运算符组合结果
	if len(matches) == 0 {
		return true
	}

	if rules.Operator == "or" {
		for _, m := range matches {
			if m {
				return true
			}
		}
		return false
	}

	// 默认 AND
	for _, m := range matches {
		if !m {
			return false
		}
	}
	return true
}

// GenerateSmartAlbums 自动生成智能相册
func (am *AlbumManager) GenerateSmartAlbums() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 按人物生成
	peopleMap := make(map[string][]string)
	for _, p := range am.photos {
		for _, person := range p.People {
			peopleMap[person] = append(peopleMap[person], p.ID)
		}
	}

	for person, photoIDs := range peopleMap {
		albumID := "person:" + person
		if _, exists := am.albums[albumID]; !exists {
			am.albums[albumID] = &Album{
				ID:         albumID,
				Name:       person,
				Type:       AlbumTypePerson,
				PhotoCount: len(photoIDs),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
		}
		for _, pid := range photoIDs {
			photo := am.photos[pid]
			if !contains(photo.Albums, albumID) {
				photo.Albums = append(photo.Albums, albumID)
			}
		}
	}

	// 按地点生成
	locationMap := make(map[string][]string)
	for _, p := range am.photos {
		if p.Location != "" {
			locationMap[p.Location] = append(locationMap[p.Location], p.ID)
		}
	}

	for location, photoIDs := range locationMap {
		albumID := "location:" + location
		if _, exists := am.albums[albumID]; !exists {
			am.albums[albumID] = &Album{
				ID:         albumID,
				Name:       location,
				Type:       AlbumTypeLocation,
				PhotoCount: len(photoIDs),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
		}
		for _, pid := range photoIDs {
			photo := am.photos[pid]
			if !contains(photo.Albums, albumID) {
				photo.Albums = append(photo.Albums, albumID)
			}
		}
	}

	// 按相机生成
	cameraMap := make(map[string][]string)
	for _, p := range am.photos {
		if p.EXIF.CameraModel != "" {
			cameraMap[p.EXIF.CameraModel] = append(cameraMap[p.EXIF.CameraModel], p.ID)
		}
	}

	for camera, photoIDs := range cameraMap {
		albumID := "camera:" + camera
		if _, exists := am.albums[albumID]; !exists {
			am.albums[albumID] = &Album{
				ID:         albumID,
				Name:       camera,
				Type:       AlbumTypeCamera,
				PhotoCount: len(photoIDs),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
		}
		for _, pid := range photoIDs {
			photo := am.photos[pid]
			if !contains(photo.Albums, albumID) {
				photo.Albums = append(photo.Albums, albumID)
			}
		}
	}

	return nil
}

// 辅助函数
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsAny(slice, items []string) bool {
	for _, item := range items {
		if contains(slice, item) {
			return true
		}
	}
	return false
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func sortPhotosByDate(photos []Photo) {
	sort.Slice(photos, func(i, j int) bool {
		return photos[i].TakenAt.After(photos[j].TakenAt)
	})
}
