// Package photos 提供相册管理功能
// 本文件实现智能相册、照片分类、去重和缩略图缓存
package photos

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/image/draw"
)

// ========== 智能相册类型 ==========

// SmartAlbumType 智能相册类型
type SmartAlbumType string

const (
	SmartAlbumTypePeople    SmartAlbumType = "people"    // 人物相册
	SmartAlbumTypeLocation  SmartAlbumType = "location"  // 地点相册
	SmartAlbumTypeTime      SmartAlbumType = "time"      // 时间相册
	SmartAlbumTypeScene     SmartAlbumType = "scene"     // 场景相册
	SmartAlbumTypeFavorites SmartAlbumType = "favorites" // 收藏相册
	SmartAlbumTypeRecent    SmartAlbumType = "recent"    // 最近照片
	SmartAlbumTypeSelfie    SmartAlbumType = "selfie"    // 自拍相册
	SmartAlbumTypeScreenshot SmartAlbumType = "screenshot" // 截图相册
	SmartAlbumTypeVideo      SmartAlbumType = "video"     // 视频相册
	SmartAlbumTypePanorama   SmartAlbumType = "panorama"  // 全景相册
)

// SmartAlbum 智能相册（自动分类）
type SmartAlbum struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         SmartAlbumType    `json:"type"`
	Criteria     SmartAlbumCriteria `json:"criteria"`
	PhotoCount   int               `json:"photoCount"`
	CoverPhotoID string            `json:"coverPhotoId"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	AutoUpdate   bool              `json:"autoUpdate"`
	IsSystem     bool              `json:"isSystem"`
	PhotoIDs     []string          `json:"photoIds,omitempty"`
}

// SmartAlbumCriteria 智能相册条件
type SmartAlbumCriteria struct {
	PersonIDs    []string  `json:"personIds,omitempty"`
	Location     string    `json:"location,omitempty"`
	City         string    `json:"city,omitempty"`
	Country      string    `json:"country,omitempty"`
	StartDate    time.Time `json:"startDate,omitempty"`
	EndDate      time.Time `json:"endDate,omitempty"`
	Year         int       `json:"year,omitempty"`
	Month        int       `json:"month,omitempty"`
	Scene        string    `json:"scene,omitempty"`
	IsFavorite   *bool     `json:"isFavorite,omitempty"`
	IsSelfie     *bool     `json:"isSelfie,omitempty"`
	IsScreenshot *bool     `json:"isScreenshot,omitempty"`
	IsVideo      *bool     `json:"isVideo,omitempty"`
	IsPanorama   *bool     `json:"isPanorama,omitempty"`
	MinWidth     int       `json:"minWidth,omitempty"`
	MinHeight    int       `json:"minHeight,omitempty"`
	AspectRatio  float64   `json:"aspectRatio,omitempty"`
}

// ========== 照片分类 ==========

// PhotoClassifier 照片分类器
type PhotoClassifier struct {
	manager *Manager
	mu      sync.RWMutex
	// 索引
	personIndex   map[string][]string // personID -> photoIDs
	locationIndex map[string][]string // location -> photoIDs
	timeIndex     map[string][]string // period -> photoIDs (YYYY-MM)
	sceneIndex    map[string][]string // scene -> photoIDs
	yearIndex     map[int][]string    // year -> photoIDs
}

// NewPhotoClassifier 创建照片分类器
func NewPhotoClassifier(manager *Manager) *PhotoClassifier {
	return &PhotoClassifier{
		manager:       manager,
		personIndex:   make(map[string][]string),
		locationIndex: make(map[string][]string),
		timeIndex:     make(map[string][]string),
		sceneIndex:    make(map[string][]string),
		yearIndex:     make(map[int][]string),
	}
}

// BuildIndex 构建分类索引
func (c *PhotoClassifier) BuildIndex() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 清空现有索引
	c.personIndex = make(map[string][]string)
	c.locationIndex = make(map[string][]string)
	c.timeIndex = make(map[string][]string)
	c.sceneIndex = make(map[string][]string)
	c.yearIndex = make(map[int][]string)

	c.manager.mu.RLock()
	defer c.manager.mu.RUnlock()

	for _, photo := range c.manager.photos {
		// 人物索引
		for _, face := range photo.Faces {
			if face.ID != "" {
				c.personIndex[face.ID] = append(c.personIndex[face.ID], photo.ID)
			}
		}

		// 地点索引
		if photo.Location != nil {
			locKey := c.buildLocationKey(photo.Location)
			if locKey != "" {
				c.locationIndex[locKey] = append(c.locationIndex[locKey], photo.ID)
			}
		}

		// 时间索引
		if !photo.TakenAt.IsZero() {
			period := photo.TakenAt.Format("2006-01")
			c.timeIndex[period] = append(c.timeIndex[period], photo.ID)
			c.yearIndex[photo.TakenAt.Year()] = append(c.yearIndex[photo.TakenAt.Year()], photo.ID)
		}

		// 场景索引
		if photo.Scene != "" {
			c.sceneIndex[photo.Scene] = append(c.sceneIndex[photo.Scene], photo.ID)
		}
	}

	return nil
}

// buildLocationKey 构建地点键
func (c *PhotoClassifier) buildLocationKey(loc *LocationInfo) string {
	parts := make([]string, 0)
	if loc.Country != "" {
		parts = append(parts, loc.Country)
	}
	if loc.City != "" {
		parts = append(parts, loc.City)
	}
	if len(parts) == 0 && loc.Location != "" {
		parts = append(parts, loc.Location)
	}
	return strings.Join(parts, "/")
}

// ClassifyByPeople 按人物分类
func (c *PhotoClassifier) ClassifyByPeople() map[string][]*Photo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string][]*Photo)
	for personID, photoIDs := range c.personIndex {
		photos := make([]*Photo, 0, len(photoIDs))
		for _, photoID := range photoIDs {
			if photo, exists := c.manager.photos[photoID]; exists {
				photoCopy := *photo
				photos = append(photos, &photoCopy)
			}
		}
		result[personID] = photos
	}
	return result
}

// ClassifyByLocation 按地点分类
func (c *PhotoClassifier) ClassifyByLocation() map[string][]*Photo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string][]*Photo)
	for location, photoIDs := range c.locationIndex {
		photos := make([]*Photo, 0, len(photoIDs))
		for _, photoID := range photoIDs {
			if photo, exists := c.manager.photos[photoID]; exists {
				photoCopy := *photo
				photos = append(photos, &photoCopy)
			}
		}
		result[location] = photos
	}
	return result
}

// ClassifyByTime 按时间分类
func (c *PhotoClassifier) ClassifyByTime(groupBy string) map[string][]*Photo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string][]*Photo)

	for _, photo := range c.manager.photos {
		if photo.TakenAt.IsZero() {
			continue
		}

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

		photoCopy := *photo
		result[key] = append(result[key], &photoCopy)
	}

	return result
}

// ClassifyByYear 按年份分类
func (c *PhotoClassifier) ClassifyByYear() map[int][]*Photo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[int][]*Photo)
	for year, photoIDs := range c.yearIndex {
		photos := make([]*Photo, 0, len(photoIDs))
		for _, photoID := range photoIDs {
			if photo, exists := c.manager.photos[photoID]; exists {
				photoCopy := *photo
				photos = append(photos, &photoCopy)
			}
		}
		result[year] = photos
	}
	return result
}

// ClassifyByScene 按场景分类
func (c *PhotoClassifier) ClassifyByScene() map[string][]*Photo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string][]*Photo)
	for scene, photoIDs := range c.sceneIndex {
		photos := make([]*Photo, 0, len(photoIDs))
		for _, photoID := range photoIDs {
			if photo, exists := c.manager.photos[photoID]; exists {
				photoCopy := *photo
				photos = append(photos, &photoCopy)
			}
		}
		result[scene] = photos
	}
	return result
}

// GetClassificationStats 获取分类统计
func (c *PhotoClassifier) GetClassificationStats() *ClassificationStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &ClassificationStats{
		PeopleCount:    len(c.personIndex),
		LocationCount:  len(c.locationIndex),
		MonthCount:     len(c.timeIndex),
		YearCount:      len(c.yearIndex),
		SceneCount:     len(c.sceneIndex),
		PeoplePhotos:   make(map[string]int),
		LocationPhotos: make(map[string]int),
		YearPhotos:     make(map[int]int),
	}

	for personID, photoIDs := range c.personIndex {
		stats.PeoplePhotos[personID] = len(photoIDs)
	}
	for location, photoIDs := range c.locationIndex {
		stats.LocationPhotos[location] = len(photoIDs)
	}
	for year, photoIDs := range c.yearIndex {
		stats.YearPhotos[year] = len(photoIDs)
	}

	return stats
}

// ClassificationStats 分类统计
type ClassificationStats struct {
	PeopleCount    int            `json:"peopleCount"`
	LocationCount  int            `json:"locationCount"`
	MonthCount     int            `json:"monthCount"`
	YearCount      int            `json:"yearCount"`
	SceneCount     int            `json:"sceneCount"`
	PeoplePhotos   map[string]int `json:"peoplePhotos"`
	LocationPhotos map[string]int `json:"locationPhotos"`
	YearPhotos     map[int]int    `json:"yearPhotos"`
}

// ========== 智能相册管理 ==========

// SmartAlbumManager 智能相册管理器
type SmartAlbumManager struct {
	manager     *Manager
	classifier  *PhotoClassifier
	albums      map[string]*SmartAlbum
	mu          sync.RWMutex
	dataPath    string
}

// NewSmartAlbumManager 创建智能相册管理器
func NewSmartAlbumManager(manager *Manager, classifier *PhotoClassifier) *SmartAlbumManager {
	sam := &SmartAlbumManager{
		manager:    manager,
		classifier: classifier,
		albums:     make(map[string]*SmartAlbum),
		dataPath:   filepath.Join(manager.dataDir, "smart-albums.json"),
	}

	// 加载已保存的智能相册
	_ = sam.load()

	// 创建系统智能相册
	sam.createSystemAlbums()

	return sam
}

// createSystemAlbums 创建系统智能相册
func (s *SmartAlbumManager) createSystemAlbums() {
	s.mu.Lock()
	defer s.mu.Unlock()

	systemAlbums := []struct {
		id      string
		name    string
		albumType SmartAlbumType
		criteria SmartAlbumCriteria
	}{
		{
			id:      "system-favorites",
			name:    "收藏",
			albumType: SmartAlbumTypeFavorites,
			criteria: SmartAlbumCriteria{IsFavorite: boolPtr(true)},
		},
		{
			id:      "system-recent",
			name:    "最近照片",
			albumType: SmartAlbumTypeRecent,
			criteria: SmartAlbumCriteria{},
		},
		{
			id:      "system-videos",
			name:    "视频",
			albumType: SmartAlbumTypeVideo,
			criteria: SmartAlbumCriteria{IsVideo: boolPtr(true)},
		},
		{
			id:      "system-selfies",
			name:    "自拍",
			albumType: SmartAlbumTypeSelfie,
			criteria: SmartAlbumCriteria{IsSelfie: boolPtr(true)},
		},
		{
			id:      "system-screenshots",
			name:    "截图",
			albumType: SmartAlbumTypeScreenshot,
			criteria: SmartAlbumCriteria{IsScreenshot: boolPtr(true)},
		},
	}

	for _, sa := range systemAlbums {
		if _, exists := s.albums[sa.id]; !exists {
			s.albums[sa.id] = &SmartAlbum{
				ID:         sa.id,
				Name:       sa.name,
				Type:       sa.albumType,
				Criteria:   sa.criteria,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				AutoUpdate: true,
				IsSystem:   true,
				PhotoIDs:   []string{},
			}
		}
	}
}

// load 加载智能相册数据
func (s *SmartAlbumManager) load() error {
	data, err := os.ReadFile(s.dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var albums []*SmartAlbum
	if err := json.Unmarshal(data, &albums); err != nil {
		return err
	}

	for _, album := range albums {
		s.albums[album.ID] = album
	}
	return nil
}

// save 保存智能相册数据
func (s *SmartAlbumManager) save() error {
	albums := make([]*SmartAlbum, 0)
	for _, album := range s.albums {
		if !album.IsSystem {
			albums = append(albums, album)
		}
	}
	data, err := json.MarshalIndent(albums, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.dataPath, data, 0640)
}

// CreateSmartAlbum 创建智能相册
func (s *SmartAlbumManager) CreateSmartAlbum(name string, albumType SmartAlbumType, criteria SmartAlbumCriteria) (*SmartAlbum, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	album := &SmartAlbum{
		ID:         uuid.New().String(),
		Name:       name,
		Type:       albumType,
		Criteria:   criteria,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		AutoUpdate: true,
		IsSystem:   false,
		PhotoIDs:   []string{},
	}

	// 立即匹配照片
	album.PhotoIDs = s.matchPhotos(&album.Criteria)
	album.PhotoCount = len(album.PhotoIDs)

	s.albums[album.ID] = album

	if err := s.save(); err != nil {
		return nil, err
	}

	return album, nil
}

// matchPhotos 匹配符合条件的照片
func (s *SmartAlbumManager) matchPhotos(criteria *SmartAlbumCriteria) []string {
	s.manager.mu.RLock()
	defer s.manager.mu.RUnlock()

	var photoIDs []string

	for _, photo := range s.manager.photos {
		if s.matchCriteria(photo, criteria) {
			photoIDs = append(photoIDs, photo.ID)
		}
	}

	return photoIDs
}

// matchCriteria 检查照片是否匹配条件
func (s *SmartAlbumManager) matchCriteria(photo *Photo, criteria *SmartAlbumCriteria) bool {
	// 人物匹配
	if len(criteria.PersonIDs) > 0 {
		matched := false
		for _, face := range photo.Faces {
			for _, personID := range criteria.PersonIDs {
				if face.ID == personID {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 地点匹配
	if criteria.Location != "" && photo.Location != nil {
		if !strings.Contains(photo.Location.Location, criteria.Location) {
			return false
		}
	}
	if criteria.City != "" && photo.Location != nil {
		if photo.Location.City != criteria.City {
			return false
		}
	}
	if criteria.Country != "" && photo.Location != nil {
		if photo.Location.Country != criteria.Country {
			return false
		}
	}

	// 时间匹配
	if !criteria.StartDate.IsZero() && photo.TakenAt.Before(criteria.StartDate) {
		return false
	}
	if !criteria.EndDate.IsZero() && photo.TakenAt.After(criteria.EndDate) {
		return false
	}
	if criteria.Year > 0 && photo.TakenAt.Year() != criteria.Year {
		return false
	}
	if criteria.Month > 0 && int(photo.TakenAt.Month()) != criteria.Month {
		return false
	}

	// 场景匹配
	if criteria.Scene != "" && photo.Scene != criteria.Scene {
		return false
	}

	// 布尔条件
	if criteria.IsFavorite != nil && photo.IsFavorite != *criteria.IsFavorite {
		return false
	}
	if criteria.IsSelfie != nil && !*criteria.IsSelfie {
		// 非自拍检查：需要人脸数 > 0 且宽高比接近竖屏
		if len(photo.Faces) > 0 && photo.Width > 0 && photo.Height > 0 {
			aspectRatio := float64(photo.Width) / float64(photo.Height)
			if aspectRatio < 1.0 { // 竖屏可能是自拍
				return false
			}
		}
	}
	if criteria.IsVideo != nil {
		isVideo := photo.Duration > 0
		if isVideo != *criteria.IsVideo {
			return false
		}
	}
	if criteria.IsScreenshot != nil {
		// 截图判断：通常来自特定设备或软件
		if *criteria.IsScreenshot {
			if photo.Device == nil || !isScreenshotDevice(photo.Device) {
				return false
			}
		}
	}
	if criteria.IsPanorama != nil && *criteria.IsPanorama {
		// 全景图判断：宽高比很大
		if photo.Width > 0 && photo.Height > 0 {
			aspectRatio := float64(photo.Width) / float64(photo.Height)
			if aspectRatio < 2.0 { // 全景图宽高比通常 > 2
				return false
			}
		}
	}

	// 尺寸条件
	if criteria.MinWidth > 0 && photo.Width < criteria.MinWidth {
		return false
	}
	if criteria.MinHeight > 0 && photo.Height < criteria.MinHeight {
		return false
	}

	return true
}

// isScreenshotDevice 判断是否为截图设备
func isScreenshotDevice(device *DeviceInfo) bool {
	screenshotApps := []string{"Screenshot", "Screen Capture", "截屏", "截图"}
	for _, app := range screenshotApps {
		if strings.Contains(device.App, app) {
			return true
		}
	}
	return false
}

// UpdateAllSmartAlbums 更新所有智能相册
func (s *SmartAlbumManager) UpdateAllSmartAlbums() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, album := range s.albums {
		if album.AutoUpdate {
			album.PhotoIDs = s.matchPhotos(&album.Criteria)
			album.PhotoCount = len(album.PhotoIDs)
			album.UpdatedAt = time.Now()
		}
	}

	return s.save()
}

// GetSmartAlbum 获取智能相册
func (s *SmartAlbumManager) GetSmartAlbum(albumID string) (*SmartAlbum, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	album, exists := s.albums[albumID]
	if !exists {
		return nil, fmt.Errorf("智能相册不存在")
	}

	albumCopy := *album
	return &albumCopy, nil
}

// ListSmartAlbums 列出智能相册
func (s *SmartAlbumManager) ListSmartAlbums(albumType SmartAlbumType) []*SmartAlbum {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*SmartAlbum, 0)
	for _, album := range s.albums {
		if albumType != "" && album.Type != albumType {
			continue
		}
		albumCopy := *album
		result = append(result, &albumCopy)
	}

	return result
}

// DeleteSmartAlbum 删除智能相册
func (s *SmartAlbumManager) DeleteSmartAlbum(albumID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	album, exists := s.albums[albumID]
	if !exists {
		return fmt.Errorf("智能相册不存在")
	}

	if album.IsSystem {
		return fmt.Errorf("系统智能相册不能删除")
	}

	delete(s.albums, albumID)
	return s.save()
}

// AutoCreateAlbumsFromClassification 根据分类自动创建相册
func (s *SmartAlbumManager) AutoCreateAlbumsFromClassification() error {
	// 更新分类索引
	if err := s.classifier.BuildIndex(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 为每个年份创建相册
	years := s.classifier.ClassifyByYear()
	for year, photos := range years {
		if len(photos) < 5 { // 至少 5 张照片才创建相册
			continue
		}

		albumID := fmt.Sprintf("auto-year-%d", year)
		if _, exists := s.albums[albumID]; !exists {
			s.albums[albumID] = &SmartAlbum{
				ID:         albumID,
				Name:       fmt.Sprintf("%d年", year),
				Type:       SmartAlbumTypeTime,
				Criteria:   SmartAlbumCriteria{Year: year},
				PhotoCount: len(photos),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				AutoUpdate: true,
				IsSystem:   false,
			}
		}
	}

	// 为每个地点创建相册（按城市）
	locations := s.classifier.ClassifyByLocation()
	for location, photos := range locations {
		if len(photos) < 5 {
			continue
		}

		locKey := strings.ReplaceAll(location, "/", "-")
		albumID := fmt.Sprintf("auto-location-%s", locKey)
		if _, exists := s.albums[albumID]; !exists {
			s.albums[albumID] = &SmartAlbum{
				ID:         albumID,
				Name:       location,
				Type:       SmartAlbumTypeLocation,
				Criteria:   SmartAlbumCriteria{Location: location},
				PhotoCount: len(photos),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				AutoUpdate: true,
				IsSystem:   false,
			}
		}
	}

	// 为每个人物创建相册
	people := s.classifier.ClassifyByPeople()
	for personID, photos := range people {
		if len(photos) < 3 {
			continue
		}

		albumID := fmt.Sprintf("auto-person-%s", personID)
		if _, exists := s.albums[albumID]; !exists {
			// 获取人物名称
			personName := personID
			if person, exists := s.manager.persons[personID]; exists {
				personName = person.Name
			}

			s.albums[albumID] = &SmartAlbum{
				ID:         albumID,
				Name:       personName,
				Type:       SmartAlbumTypePeople,
				Criteria:   SmartAlbumCriteria{PersonIDs: []string{personID}},
				PhotoCount: len(photos),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				AutoUpdate: true,
				IsSystem:   false,
			}
		}
	}

	return s.save()
}

// ========== 感知哈希去重 ==========

// PerceptualHash 感知哈希
type PerceptualHash struct {
	Hash       string `json:"hash"`
	Hamming    int    `json:"hamming"`    // 与原始图的汉明距离
	SourcePath string `json:"sourcePath"` // 原始图路径
}

// DuplicatePhoto 重复照片
type DuplicatePhoto struct {
	OriginalID   string   `json:"originalId"`
	OriginalPath string   `json:"originalPath"`
	Duplicates   []string `json:"duplicates"`
	Similarity   float64  `json:"similarity"`
}

// PhotoDeduplicator 照片去重器
type PhotoDeduplicator struct {
	manager    *Manager
	hashSize   int
	threshold  int // 汉明距离阈值
	mu         sync.RWMutex
	hashCache  map[string]string // photoID -> hash
}

// NewPhotoDeduplicator 创建照片去重器
func NewPhotoDeduplicator(manager *Manager) *PhotoDeduplicator {
	return &PhotoDeduplicator{
		manager:   manager,
		hashSize:  8,
		threshold: 10, // 汉明距离小于 10 认为相似
		hashCache: make(map[string]string),
	}
}

// ComputePerceptualHash 计算感知哈希
func (d *PhotoDeduplicator) ComputePerceptualHash(photoID string) (string, error) {
	d.manager.mu.RLock()
	photo, exists := d.manager.photos[photoID]
	d.manager.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("照片不存在")
	}

	// 检查缓存
	d.mu.RLock()
	if hash, cached := d.hashCache[photoID]; cached {
		d.mu.RUnlock()
		return hash, nil
	}
	d.mu.RUnlock()

	// 获取缩略图路径
	thumbPath := d.getBestThumbnail(photoID)
	if thumbPath == "" {
		// 使用原图
		thumbPath = filepath.Join(d.manager.photosDir, photo.Path)
	}

	// 读取图像
	img, err := d.readImage(thumbPath)
	if err != nil {
		return "", err
	}

	// 计算感知哈希
	hash := d.computePHash(img)

	// 缓存结果
	d.mu.Lock()
	d.hashCache[photoID] = hash
	d.mu.Unlock()

	return hash, nil
}

// getBestThumbnail 获取最佳缩略图
func (d *PhotoDeduplicator) getBestThumbnail(photoID string) string {
	sizes := []int{128, 512, 1024}
	for _, size := range sizes {
		thumbPath := filepath.Join(d.manager.thumbsDir, fmt.Sprintf("%s_%d.jpg", photoID, size))
		if _, err := os.Stat(thumbPath); err == nil {
			return thumbPath
		}
	}
	return ""
}

// readImage 读取图像
func (d *PhotoDeduplicator) readImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return d.decodePNG(file)
	default:
		return jpeg.Decode(file)
	}
}

// decodePNG 解码 PNG
func (d *PhotoDeduplicator) decodePNG(r io.Reader) (image.Image, error) {
	img, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// computePHash 计算感知哈希
func (d *PhotoDeduplicator) computePHash(img image.Image) string {
	// 1. 缩放到 hashSize x hashSize
	resized := d.resizeImage(img, d.hashSize+1, d.hashSize)

	// 2. 转换为灰度
	gray := d.toGrayscale(resized)

	// 3. 计算 DCT（简化版：使用均值比较）
	bounds := gray.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 计算每个像素与均值的比较
	pixels := make([]uint8, 0, width*height)
	var sum uint32
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			grayVal := gray.GrayAt(x, y).Y
			pixels = append(pixels, grayVal)
			sum += uint32(grayVal)
		}
	}

	mean := sum / uint32(len(pixels))

	// 生成哈希
	hash := make([]byte, 0, (width*height+7)/8)
	for i := 0; i < len(pixels); i += 8 {
		var b byte
		for j := 0; j < 8 && i+j < len(pixels); j++ {
			if pixels[i+j] > uint8(mean) {
				b |= 1 << uint(7-j)
			}
		}
		hash = append(hash, b)
	}

	return hex.EncodeToString(hash)
}

// resizeImage 缩放图像
func (d *PhotoDeduplicator) resizeImage(img image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

// toGrayscale 转换为灰度图
func (d *PhotoDeduplicator) toGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, color.GrayModel.Convert(img.At(x, y)))
		}
	}
	return gray
}

// HammingDistance 计算汉明距离
func (d *PhotoDeduplicator) HammingDistance(hash1, hash2 string) int {
	if len(hash1) != len(hash2) {
		return -1
	}

	distance := 0
	for i := 0; i < len(hash1); i++ {
		b1 := hash1[i]
		b2 := hash2[i]
		xor := b1 ^ b2
		for xor != 0 {
			distance++
			xor &= xor - 1
		}
	}
	return distance
}

// FindDuplicates 查找重复照片
func (d *PhotoDeduplicator) FindDuplicates() ([]*DuplicatePhoto, error) {
	// 构建所有照片的哈希
	d.manager.mu.RLock()
	photoIDs := make([]string, 0, len(d.manager.photos))
	for id := range d.manager.photos {
		photoIDs = append(photoIDs, id)
	}
	d.manager.mu.RUnlock()

	// 计算所有照片的哈希
	hashes := make(map[string]string) // photoID -> hash
	for _, photoID := range photoIDs {
		hash, err := d.ComputePerceptualHash(photoID)
		if err != nil {
			continue
		}
		hashes[photoID] = hash
	}

	// 找出相似的照片
	duplicates := make([]*DuplicatePhoto, 0)
	processed := make(map[string]bool)

	for id1, hash1 := range hashes {
		if processed[id1] {
			continue
		}

		dup := &DuplicatePhoto{
			OriginalID: id1,
			Duplicates: make([]string, 0),
		}

		for id2, hash2 := range hashes {
			if id1 == id2 || processed[id2] {
				continue
			}

			distance := d.HammingDistance(hash1, hash2)
			if distance >= 0 && distance < d.threshold {
				dup.Duplicates = append(dup.Duplicates, id2)
				processed[id2] = true
			}
		}

		if len(dup.Duplicates) > 0 {
			// 计算相似度
			dup.Similarity = 1.0 - float64(d.HammingDistance(hash1, hashes[dup.Duplicates[0]]))/float64(len(hash1)*8)
			
			// 获取原始照片路径
			d.manager.mu.RLock()
			if photo, exists := d.manager.photos[id1]; exists {
				dup.OriginalPath = photo.Path
			}
			d.manager.mu.RUnlock()

			duplicates = append(duplicates, dup)
			processed[id1] = true
		}
	}

	// 按相似度排序
	sort.Slice(duplicates, func(i, j int) bool {
		return duplicates[i].Similarity > duplicates[j].Similarity
	})

	return duplicates, nil
}

// RemoveDuplicates 移除重复照片
func (d *PhotoDeduplicator) RemoveDuplicates(duplicates []*DuplicatePhoto, keepOriginal bool) (int, error) {
	removed := 0

	for _, dup := range duplicates {
		for _, dupID := range dup.Duplicates {
			if keepOriginal && dupID == dup.OriginalID {
				continue
			}

			if err := d.manager.DeletePhoto(dupID); err != nil {
				continue
			}
			removed++

			// 清理缓存
			d.mu.Lock()
			delete(d.hashCache, dupID)
			d.mu.Unlock()
		}
	}

	return removed, nil
}

// ========== 缩略图缓存 ==========

// ThumbnailCache 缩略图缓存
type ThumbnailCache struct {
	manager     *Manager
	cacheDir    string
	maxSize     int64 // 最大缓存大小（字节）
	maxAge      time.Duration
	mu          sync.RWMutex
	cacheIndex  map[string]*CacheEntry
}

// CacheEntry 缓存条目
type CacheEntry struct {
	PhotoID    string    `json:"photoId"`
	Size       int       `json:"size"`
	Path       string    `json:"path"`
	FileSize   int64     `json:"fileSize"`
	CreatedAt  time.Time `json:"createdAt"`
	AccessedAt time.Time `json:"accessedAt"`
	HitCount   int       `json:"hitCount"`
}

// ThumbnailCacheConfig 缩略图缓存配置
type ThumbnailCacheConfig struct {
	MaxSize    int64         `json:"maxSize"`
	MaxAge     time.Duration `json:"maxAge"`
	Sizes      []int         `json:"sizes"`
	Quality    int           `json:"quality"`
}

// DefaultThumbnailCacheConfig 默认缩略图缓存配置
var DefaultThumbnailCacheConfig = ThumbnailCacheConfig{
	MaxSize: 500 * 1024 * 1024, // 500MB
	MaxAge:  30 * 24 * time.Hour, // 30 天
	Sizes:   []int{128, 512, 1024},
	Quality: 85,
}

// NewThumbnailCache 创建缩略图缓存
func NewThumbnailCache(manager *Manager, config ThumbnailCacheConfig) (*ThumbnailCache, error) {
	cache := &ThumbnailCache{
		manager:    manager,
		cacheDir:   filepath.Join(manager.cacheDir, "thumbnails"),
		maxSize:    config.MaxSize,
		maxAge:     config.MaxAge,
		cacheIndex: make(map[string]*CacheEntry),
	}

	// 创建缓存目录
	if err := os.MkdirAll(cache.cacheDir, 0750); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败：%w", err)
	}

	// 加载缓存索引
	if err := cache.loadIndex(); err != nil {
		// 加载失败不影响运行
		_ = err
	}

	// 清理过期缓存
	go cache.cleanupExpired()

	return cache, nil
}

// loadIndex 加载缓存索引
func (c *ThumbnailCache) loadIndex() error {
	indexFile := filepath.Join(c.cacheDir, "index.json")
	data, err := os.ReadFile(indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var entries []*CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	for _, entry := range entries {
		c.cacheIndex[fmt.Sprintf("%s_%d", entry.PhotoID, entry.Size)] = entry
	}
	return nil
}

// saveIndex 保存缓存索引
func (c *ThumbnailCache) saveIndex() error {
	entries := make([]*CacheEntry, 0)
	for _, entry := range c.cacheIndex {
		entries = append(entries, entry)
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.cacheDir, "index.json"), data, 0640)
}

// Get 获取缩略图（带缓存）
func (c *ThumbnailCache) Get(photoID string, size int) (string, error) {
	key := fmt.Sprintf("%s_%d", photoID, size)

	c.mu.RLock()
	entry, exists := c.cacheIndex[key]
	c.mu.RUnlock()

	// 检查缓存是否存在
	if exists {
		// 检查文件是否存在
		if _, err := os.Stat(entry.Path); err == nil {
			// 更新访问时间
			c.mu.Lock()
			entry.AccessedAt = time.Now()
			entry.HitCount++
			c.mu.Unlock()
			return entry.Path, nil
		}
	}

	// 缓存不存在，生成新的缩略图
	return c.generate(photoID, size)
}

// generate 生成缩略图
func (c *ThumbnailCache) generate(photoID string, size int) (string, error) {
	c.manager.mu.RLock()
	photo, exists := c.manager.photos[photoID]
	c.manager.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("照片不存在")
	}

	srcPath := filepath.Join(c.manager.photosDir, photo.Path)
	dstPath := filepath.Join(c.cacheDir, fmt.Sprintf("%s_%d.jpg", photoID, size))

	// 读取原图
	file, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}

	var img image.Image
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == ".png" {
		img, err = image.Decode(file)
	} else {
		img, err = jpeg.Decode(file)
	}
	_ = file.Close()

	if err != nil {
		return "", err
	}

	// 计算新尺寸
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	newWidth, newHeight := resizeDimensions(width, height, size)

	// 创建缩略图
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	// 保存
	outFile, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}

	err = jpeg.Encode(outFile, dst, &jpeg.Options{Quality: 85})
	_ = outFile.Close()

	if err != nil {
		return "", err
	}

	// 获取文件大小
	info, _ := os.Stat(dstPath)

	// 更新缓存索引
	key := fmt.Sprintf("%s_%d", photoID, size)
	c.mu.Lock()
	c.cacheIndex[key] = &CacheEntry{
		PhotoID:    photoID,
		Size:       size,
		Path:       dstPath,
		FileSize:   info.Size(),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		HitCount:   1,
	}
	c.mu.Unlock()

	// 检查缓存大小
	go c.checkSize()

	return dstPath, nil
}

// checkSize 检查缓存大小并清理
func (c *ThumbnailCache) checkSize() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var totalSize int64
	for _, entry := range c.cacheIndex {
		totalSize += entry.FileSize
	}

	if totalSize <= c.maxSize {
		return
	}

	// 按访问时间排序，删除最旧的
	entries := make([]*CacheEntry, 0, len(c.cacheIndex))
	for _, entry := range c.cacheIndex {
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AccessedAt.Before(entries[j].AccessedAt)
	})

	// 删除直到大小满足要求
	for _, entry := range entries {
		if totalSize <= c.maxSize {
			break
		}

		// 删除文件
		if err := os.Remove(entry.Path); err == nil {
			totalSize -= entry.FileSize
			delete(c.cacheIndex, fmt.Sprintf("%s_%d", entry.PhotoID, entry.Size))
		}
	}

	_ = c.saveIndex()
}

// cleanupExpired 清理过期缓存
func (c *ThumbnailCache) cleanupExpired() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.cacheIndex {
			if now.Sub(entry.CreatedAt) > c.maxAge {
				_ = os.Remove(entry.Path)
				delete(c.cacheIndex, key)
			}
		}
		_ = c.saveIndex()
		c.mu.Unlock()
	}
}

// Clear 清空缓存
func (c *ThumbnailCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 删除所有缓存文件
	for _, entry := range c.cacheIndex {
		_ = os.Remove(entry.Path)
	}

	c.cacheIndex = make(map[string]*CacheEntry)
	return c.saveIndex()
}

// GetStats 获取缓存统计
func (c *ThumbnailCache) GetStats() *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &CacheStats{
		EntryCount: len(c.cacheIndex),
		TotalSize:  0,
		TotalHits:  0,
	}

	for _, entry := range c.cacheIndex {
		stats.TotalSize += entry.FileSize
		stats.TotalHits += entry.HitCount
	}

	return stats
}

// CacheStats 缓存统计
type CacheStats struct {
	EntryCount int   `json:"entryCount"`
	TotalSize  int64 `json:"totalSize"`
	TotalHits  int   `json:"totalHits"`
}

// ========== EXIF 增强提取 ==========

// EXIFExtractor EXIF 提取器（增强版）
type EXIFExtractor struct {
	manager *Manager
}

// NewEXIFExtractor 创建 EXIF 提取器
func NewEXIFExtractor(manager *Manager) *EXIFExtractor {
	return &EXIFExtractor{manager: manager}
}

// ExtractFullEXIF 提取完整 EXIF 信息
func (e *EXIFExtractor) ExtractFullEXIF(photoID string) (*EXIFData, error) {
	e.manager.mu.RLock()
	photo, exists := e.manager.photos[photoID]
	e.manager.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("照片不存在")
	}

	// 使用已有的 extractEXIF 方法
	return e.manager.extractEXIF(filepath.Join(e.manager.photosDir, photo.Path))
}

// ExtractGPS 提取 GPS 信息
func (e *EXIFExtractor) ExtractGPS(photoID string) (*LocationInfo, error) {
	exifData, err := e.ExtractFullEXIF(photoID)
	if err != nil {
		return nil, err
	}

	if exifData.GPSLatitude == 0 && exifData.GPSLongitude == 0 {
		return nil, fmt.Errorf("无 GPS 信息")
	}

	return &LocationInfo{
		Latitude:  exifData.GPSLatitude,
		Longitude: exifData.GPSLongitude,
		Altitude:  exifData.GPSAltitude,
	}, nil
}

// ExtractDateTime 提取拍摄时间
func (e *EXIFExtractor) ExtractDateTime(photoID string) (time.Time, error) {
	exifData, err := e.ExtractFullEXIF(photoID)
	if err != nil {
		return time.Time{}, err
	}

	if exifData.DateTime == "" {
		return time.Time{}, fmt.Errorf("无时间信息")
	}

	// 尝试多种格式解析
	formats := []string{
		"2006:01:02 15:04:05",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006:01:02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, exifData.DateTime); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("无法解析时间：%s", exifData.DateTime)
}

// ExtractCameraInfo 提取相机信息
func (e *EXIFExtractor) ExtractCameraInfo(photoID string) (*DeviceInfo, error) {
	exifData, err := e.ExtractFullEXIF(photoID)
	if err != nil {
		return nil, err
	}

	return &DeviceInfo{
		Brand: exifData.Make,
		Model: exifData.Model,
	}, nil
}

// ========== 辅助函数 ==========

// boolPtr 返回 bool 指针
func boolPtr(v bool) *bool {
	return &v
}

// ========== 内容哈希（用于精确去重） ==========

// ComputeContentHash 计算内容哈希（精确去重）
func (m *Manager) ComputeContentHash(photoID string) (string, error) {
	m.mu.RLock()
	photo, exists := m.photos[photoID]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("照片不存在")
	}

	file, err := os.Open(filepath.Join(m.photosDir, photo.Path))
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// FindExactDuplicates 查找完全相同的照片（基于内容哈希）
func (m *Manager) FindExactDuplicates() (map[string][]string, error) {
	m.mu.RLock()
	photoIDs := make([]string, 0, len(m.photos))
	for id := range m.photos {
		photoIDs = append(photoIDs, id)
	}
	m.mu.RUnlock()

	// 计算所有照片的内容哈希
	hashToIDs := make(map[string][]string)
	for _, photoID := range photoIDs {
		hash, err := m.ComputeContentHash(photoID)
		if err != nil {
			continue
		}
		hashToIDs[hash] = append(hashToIDs[hash], photoID)
	}

	// 筛选出有重复的
	duplicates := make(map[string][]string)
	for hash, ids := range hashToIDs {
		if len(ids) > 1 {
			duplicates[hash] = ids
		}
	}

	return duplicates, nil
}

// ========== 辅助数学函数 ==========

// round 四舍五入
func round(x float64) int {
	return int(math.Floor(x + 0.5))
}