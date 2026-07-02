// Package smartgallery 提供智能相册核心管理逻辑
package smartgallery

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 智能相册管理器.
type Manager struct {
	mu      sync.RWMutex
	photos  map[string]*Photo
	albums  map[string]*PhotoGallery
	persons map[string]*Person
	faces   map[string]*Face
	scenes  map[string]*Scene
	tags    map[string]*SmartTag
	dupes   map[string]*DuplicateGroup
	imports map[string]*ImportJob
}

// ImportJob 导入任务.
type ImportJob struct {
	ID         string     `json:"id"`
	Source     string     `json:"source"` // local, url, upload
	Path       string     `json:"path"`
	Status     string     `json:"status"` // pending, running, completed, failed
	TotalFiles int        `json:"total_files"`
	Imported   int        `json:"imported"`
	Skipped    int        `json:"skipped"`
	Failed     int        `json:"failed"`
	Error      string     `json:"error,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// GalleryStats 相册统计.
type GalleryStats struct {
	TotalPhotos     int            `json:"total_photos"`
	TotalAlbums     int            `json:"total_albums"`
	TotalPersons    int            `json:"total_persons"`
	TotalFaces      int            `json:"total_faces"`
	TotalScenes     int            `json:"total_scenes"`
	TotalTags       int            `json:"total_tags"`
	TotalDuplicates int            `json:"total_duplicates"`
	FavoriteCount   int            `json:"favorite_count"`
	HiddenCount     int            `json:"hidden_count"`
	TotalSize       int64          `json:"total_size"`
	ScenesBreakdown map[string]int `json:"scenes_breakdown"`
	TopPersons      []PersonStat   `json:"top_persons"`
	ImportJobs      int            `json:"import_jobs"`
}

// PersonStat 人物统计.
type PersonStat struct {
	Name       string `json:"name"`
	PhotoCount int    `json:"photo_count"`
}

// EXIFInfo EXIF 信息.
type EXIFInfo struct {
	Camera      string    `json:"camera,omitempty"`
	Lens        string    `json:"lens,omitempty"`
	ISO         int       `json:"iso,omitempty"`
	Aperture    string    `json:"aperture,omitempty"`
	Shutter     string    `json:"shutter,omitempty"`
	FocalLength string    `json:"focal_length,omitempty"`
	Flash       bool      `json:"flash,omitempty"`
	Width       int       `json:"width,omitempty"`
	Height      int       `json:"height,omitempty"`
	Orientation int       `json:"orientation,omitempty"`
	TakenAt     time.Time `json:"taken_at,omitempty"`
	GPS         *GPSInfo  `json:"gps,omitempty"`
}

// NewManager 创建智能相册管理器.
func NewManager() *Manager {
	m := &Manager{
		photos:  make(map[string]*Photo),
		albums:  make(map[string]*PhotoGallery),
		persons: make(map[string]*Person),
		faces:   make(map[string]*Face),
		scenes:  make(map[string]*Scene),
		tags:    make(map[string]*SmartTag),
		dupes:   make(map[string]*DuplicateGroup),
		imports: make(map[string]*ImportJob),
	}

	// 初始化默认相册
	m.initDefaultAlbums()

	return m
}

// generateID 生成唯一 ID.
func generateID() string {
	return uuid.New().String()
}

// initDefaultAlbums 初始化默认相册.
func (m *Manager) initDefaultAlbums() {
	defaults := []PhotoGallery{
		{
			ID:          "album-favorites",
			Name:        "收藏夹",
			Description: "我收藏的照片",
			Type:        "smart",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "album-recent",
			Name:        "最近添加",
			Description: "最近30天添加的照片",
			Type:        "smart",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "album-faces",
			Name:        "人物相册",
			Description: "按人物分类的照片",
			Type:        "face",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for i := range defaults {
		album := &defaults[i]
		m.albums[album.ID] = album
	}
}

// ImportPhotos 导入照片.
func (m *Manager) ImportPhotos(source, path string) (*ImportJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job := &ImportJob{
		ID:        generateID(),
		Source:    source,
		Path:      path,
		Status:    "pending",
		StartedAt: time.Now(),
	}
	m.imports[job.ID] = job

	// 模拟导入过程
	go m.processImport(job)

	return job, nil
}

// processImport 处理导入任务.
func (m *Manager) processImport(job *ImportJob) {
	m.mu.Lock()
	job.Status = "running"
	m.mu.Unlock()

	// 模拟导入文件
	totalFiles := 10
	for i := 0; i < totalFiles; i++ {
		m.mu.Lock()
		job.TotalFiles = totalFiles

		// 模拟每个文件的导入
		photoID := generateID()
		photo := &Photo{
			ID:         photoID,
			FilePath:   fmt.Sprintf("%s/photo_%d.jpg", job.Path, i),
			FileName:   fmt.Sprintf("photo_%d.jpg", i),
			MimeType:   "image/jpeg",
			Size:       int64(1024 * 1024 * (1 + i%5)),
			ShotAt:     time.Now().AddDate(0, 0, -i),
			UploadedAt: time.Now(),
			Camera:     "iPhone 15 Pro",
		}

		// 提取 EXIF（模拟）
		m.extractEXIF(photo)

		// 添加场景标签
		m.classifyScene(photo)

		m.photos[photoID] = photo
		job.Imported++
		m.mu.Unlock()

		time.Sleep(100 * time.Millisecond)
	}

	m.mu.Lock()
	now := time.Now()
	job.Status = "completed"
	job.FinishedAt = &now
	m.mu.Unlock()
}

// extractEXIF 提取 EXIF 信息（模拟）.
func (m *Manager) extractEXIF(photo *Photo) {
	// 模拟 EXIF 数据
	if photo.Camera == "" {
		photo.Camera = "Unknown Camera"
	}

	// 模拟 GPS 数据（50% 概率有 GPS）
	if generateID()[0]%2 == 0 {
		photo.GPS = &GPSInfo{
			Latitude:  31.2304 + float64(generateID()[0]%10)*0.01,
			Longitude: 121.4737 + float64(generateID()[1]%10)*0.01,
			Address:   "上海市",
		}
	}
}

// classifyScene 场景分类（模拟）.
func (m *Manager) classifyScene(photo *Photo) {
	categories := []struct {
		label    string
		category string
	}{
		{"海滩", "nature"},
		{"山脉", "nature"},
		{"城市街景", "urban"},
		{"室内", "indoor"},
		{"美食", "food"},
		{"建筑", "architecture"},
		{"动物", "animal"},
		{"花卉", "nature"},
		{"日落", "nature"},
		{"夜景", "urban"},
	}

	// 随机分配 1-2 个场景
	idx := int(photo.ID[0]) % len(categories)
	scene := &Scene{
		ID:         generateID(),
		PhotoID:    photo.ID,
		Label:      categories[idx].label,
		Category:   categories[idx].category,
		Confidence: 0.75 + float64(photo.ID[1]%25)*0.01,
	}

	m.scenes[scene.ID] = scene
	photo.Scenes = append(photo.Scenes, *scene)
}

// GetPhoto 获取照片详情.
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
func (m *Manager) ListPhotos(page, pageSize int) ([]Photo, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]Photo, 0, len(m.photos))
	for _, p := range m.photos {
		if !p.IsHidden {
			all = append(all, *p)
		}
	}

	// 按拍摄时间倒序
	sort.Slice(all, func(i, j int) bool {
		return all[i].ShotAt.After(all[j].ShotAt)
	})

	total := len(all)
	start := (page - 1) * pageSize
	if start >= total {
		return []Photo{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return all[start:end], total
}

// UpdatePhoto 更新照片信息.
func (m *Manager) UpdatePhoto(id string, updates map[string]interface{}) (*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[id]
	if !ok {
		return nil, fmt.Errorf("photo not found: %s", id)
	}

	if v, ok := updates["is_favorite"].(bool); ok {
		photo.IsFavorite = v
	}
	if v, ok := updates["is_hidden"].(bool); ok {
		photo.IsHidden = v
	}
	if v, ok := updates["rating"].(float64); ok {
		rating := int(v)
		if rating >= 0 && rating <= 5 {
			photo.Rating = rating
		}
	}

	return photo, nil
}

// DeletePhoto 删除照片.
func (m *Manager) DeletePhoto(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.photos[id]; !ok {
		return fmt.Errorf("photo not found: %s", id)
	}

	// 删除关联的人脸
	for fid, face := range m.faces {
		if face.PhotoID == id {
			delete(m.faces, fid)
		}
	}

	// 删除关联的场景
	for sid, scene := range m.scenes {
		if scene.PhotoID == id {
			delete(m.scenes, sid)
		}
	}

	delete(m.photos, id)
	return nil
}

// SearchPhotos 搜索照片.
func (m *Manager) SearchPhotos(req *PhotoSearchRequest) *PhotoSearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	var results []Photo

	for _, photo := range m.photos {
		if photo.IsHidden {
			continue
		}

		// 日期过滤
		if req.DateFrom != "" {
			from, err := time.Parse("2006-01-02", req.DateFrom)
			if err == nil && photo.ShotAt.Before(from) {
				continue
			}
		}
		if req.DateTo != "" {
			to, err := time.Parse("2006-01-02", req.DateTo)
			if err == nil && photo.ShotAt.After(to.Add(24*time.Hour)) {
				continue
			}
		}

		// 收藏过滤
		if req.IsFavorite != nil && photo.IsFavorite != *req.IsFavorite {
			continue
		}

		// 场景过滤
		if len(req.Scenes) > 0 {
			hasScene := false
			for _, scene := range photo.Scenes {
				for _, reqScene := range req.Scenes {
					if strings.EqualFold(scene.Label, reqScene) || strings.EqualFold(scene.Category, reqScene) {
						hasScene = true
						break
					}
				}
				if hasScene {
					break
				}
			}
			if !hasScene {
				continue
			}
		}

		// 标签过滤
		if len(req.Tags) > 0 {
			hasTag := false
			for _, tag := range photo.Tags {
				for _, reqTag := range req.Tags {
					if strings.EqualFold(tag.Name, reqTag) {
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

		// 人物过滤
		if len(req.Persons) > 0 {
			hasPerson := false
			for _, face := range photo.Faces {
				for _, reqPerson := range req.Persons {
					if face.PersonID == reqPerson || strings.EqualFold(face.PersonName, reqPerson) {
						hasPerson = true
						break
					}
				}
				if hasPerson {
					break
				}
			}
			if !hasPerson {
				continue
			}
		}

		// 关键词搜索（文件名、标签）
		if req.Query != "" {
			query := strings.ToLower(req.Query)
			match := strings.Contains(strings.ToLower(photo.FileName), query)
			if !match {
				for _, tag := range photo.Tags {
					if strings.Contains(strings.ToLower(tag.Name), query) {
						match = true
						break
					}
				}
			}
			if !match {
				for _, scene := range photo.Scenes {
					if strings.Contains(strings.ToLower(scene.Label), query) {
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
		}

		results = append(results, *photo)
	}

	// 按拍摄时间倒序
	sort.Slice(results, func(i, j int) bool {
		return results[i].ShotAt.After(results[j].ShotAt)
	})

	total := len(results)
	start := (page - 1) * pageSize
	if start >= total {
		return &PhotoSearchResult{
			Photos:   []Photo{},
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		}
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return &PhotoSearchResult{
		Photos:   results[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}

// CreateAlbum 创建相册.
func (m *Manager) CreateAlbum(name, description, albumType string) *PhotoGallery {
	m.mu.Lock()
	defer m.mu.Unlock()

	album := &PhotoGallery{
		ID:          generateID(),
		Name:        name,
		Description: description,
		Type:        albumType,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.albums[album.ID] = album
	return album
}

// GetAlbum 获取相册.
func (m *Manager) GetAlbum(id string) (*PhotoGallery, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	album, ok := m.albums[id]
	if !ok {
		return nil, fmt.Errorf("album not found: %s", id)
	}
	return album, nil
}

// ListAlbums 列出相册.
func (m *Manager) ListAlbums() []PhotoGallery {
	m.mu.RLock()
	defer m.mu.RUnlock()

	albums := make([]PhotoGallery, 0, len(m.albums))
	for _, a := range m.albums {
		albums = append(albums, *a)
	}

	sort.Slice(albums, func(i, j int) bool {
		return albums[i].UpdatedAt.After(albums[j].UpdatedAt)
	})

	return albums
}

// UpdateAlbum 更新相册.
func (m *Manager) UpdateAlbum(id string, name, description string) (*PhotoGallery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	album, ok := m.albums[id]
	if !ok {
		return nil, fmt.Errorf("album not found: %s", id)
	}

	if name != "" {
		album.Name = name
	}
	if description != "" {
		album.Description = description
	}
	album.UpdatedAt = time.Now()

	return album, nil
}

// DeleteAlbum 删除相册.
func (m *Manager) DeleteAlbum(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.albums[id]; !ok {
		return fmt.Errorf("album not found: %s", id)
	}

	// 不允许删除系统相册
	if id == "album-favorites" || id == "album-recent" || id == "album-faces" {
		return fmt.Errorf("cannot delete system album")
	}

	delete(m.albums, id)
	return nil
}

// AddPhotosToAlbum 添加照片到相册.
func (m *Manager) AddPhotosToAlbum(albumID string, photoIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	album, ok := m.albums[albumID]
	if !ok {
		return fmt.Errorf("album not found: %s", albumID)
	}

	album.PhotoCount += len(photoIDs)
	album.UpdatedAt = time.Now()
	return nil
}

// RemovePhotosFromAlbum 从相册移除照片.
func (m *Manager) RemovePhotosFromAlbum(albumID string, photoIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	album, ok := m.albums[albumID]
	if !ok {
		return fmt.Errorf("album not found: %s", albumID)
	}

	album.PhotoCount -= len(photoIDs)
	if album.PhotoCount < 0 {
		album.PhotoCount = 0
	}
	album.UpdatedAt = time.Now()
	return nil
}

// ClusterFaces 人脸识别聚类.
func (m *Manager) ClusterFaces() ([]Person, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟人脸识别和聚类
	persons := []Person{
		{
			ID:         generateID(),
			Name:       "人物A",
			PhotoCount: 5,
			FaceIDs:    []string{},
		},
		{
			ID:         generateID(),
			Name:       "人物B",
			PhotoCount: 3,
			FaceIDs:    []string{},
		},
		{
			ID:         generateID(),
			Name:       "人物C",
			PhotoCount: 2,
			FaceIDs:    []string{},
		},
	}

	for i := range persons {
		person := &persons[i]

		// 为每个人物创建人脸记录
		for j := 0; j < person.PhotoCount; j++ {
			faceID := generateID()
			face := &Face{
				ID:         faceID,
				PersonID:   person.ID,
				PersonName: person.Name,
				BoundingBox: BoundingBox{
					X:      100 + j*50,
					Y:      100 + j*30,
					Width:  150,
					Height: 150,
				},
				Confidence: 0.85 + float64(j)*0.03,
			}
			m.faces[faceID] = face
			person.FaceIDs = append(person.FaceIDs, faceID)
		}

		m.persons[person.ID] = person
	}

	return persons, nil
}

// GetPerson 获取人物信息.
func (m *Manager) GetPerson(id string) (*Person, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	person, ok := m.persons[id]
	if !ok {
		return nil, fmt.Errorf("person not found: %s", id)
	}
	return person, nil
}

// ListPersons 列出人物.
func (m *Manager) ListPersons() []Person {
	m.mu.RLock()
	defer m.mu.RUnlock()

	persons := make([]Person, 0, len(m.persons))
	for _, p := range m.persons {
		persons = append(persons, *p)
	}

	sort.Slice(persons, func(i, j int) bool {
		return persons[i].PhotoCount > persons[j].PhotoCount
	})

	return persons
}

// UpdatePerson 更新人物信息.
func (m *Manager) UpdatePerson(id, name string) (*Person, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	person, ok := m.persons[id]
	if !ok {
		return nil, fmt.Errorf("person not found: %s", id)
	}

	if name != "" {
		person.Name = name

		// 更新关联的人脸记录
		for _, faceID := range person.FaceIDs {
			if face, ok := m.faces[faceID]; ok {
				face.PersonName = name
			}
		}
	}

	return person, nil
}

// AssignFaceToPerson 将人脸分配给人物.
func (m *Manager) AssignFaceToPerson(faceID, personID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	face, ok := m.faces[faceID]
	if !ok {
		return fmt.Errorf("face not found: %s", faceID)
	}

	person, ok := m.persons[personID]
	if !ok {
		return fmt.Errorf("person not found: %s", personID)
	}

	face.PersonID = personID
	face.PersonName = person.Name
	person.FaceIDs = append(person.FaceIDs, faceID)
	person.PhotoCount++

	return nil
}

// GetPhotosByPerson 获取人物的所有照片.
func (m *Manager) GetPhotosByPerson(personID string) ([]Photo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	person, ok := m.persons[personID]
	if !ok {
		return nil, fmt.Errorf("person not found: %s", personID)
	}

	photoMap := make(map[string]bool)
	for _, faceID := range person.FaceIDs {
		if face, ok := m.faces[faceID]; ok {
			photoMap[face.PhotoID] = true
		}
	}

	photos := make([]Photo, 0, len(photoMap))
	for photoID := range photoMap {
		if photo, ok := m.photos[photoID]; ok {
			photos = append(photos, *photo)
		}
	}

	sort.Slice(photos, func(i, j int) bool {
		return photos[i].ShotAt.After(photos[j].ShotAt)
	})

	return photos, nil
}

// ClassifyScenes 场景分类.
func (m *Manager) ClassifyScenes() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sceneCounts := make(map[string]int)
	for _, scene := range m.scenes {
		sceneCounts[scene.Category]++
	}
	return sceneCounts
}

// GetPhotosByScene 按场景获取照片.
func (m *Manager) GetPhotosByScene(sceneLabel string) []Photo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photoIDs := make(map[string]bool)
	for _, scene := range m.scenes {
		if strings.EqualFold(scene.Label, sceneLabel) || strings.EqualFold(scene.Category, sceneLabel) {
			photoIDs[scene.PhotoID] = true
		}
	}

	photos := make([]Photo, 0, len(photoIDs))
	for photoID := range photoIDs {
		if photo, ok := m.photos[photoID]; ok {
			photos = append(photos, *photo)
		}
	}

	sort.Slice(photos, func(i, j int) bool {
		return photos[i].ShotAt.After(photos[j].ShotAt)
	})

	return photos
}

// GetTimeline 获取时间线.
func (m *Manager) GetTimeline(period string, year, month int) []Timeline {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 按日期分组
	dateMap := make(map[string][]Photo)
	for _, photo := range m.photos {
		if photo.IsHidden {
			continue
		}

		var dateKey string
		switch period {
		case "year":
			dateKey = photo.ShotAt.Format("2006")
		case "month":
			dateKey = photo.ShotAt.Format("2006-01")
		default: // day
			dateKey = photo.ShotAt.Format("2006-01-02")
		}

		// 过滤年月
		if year > 0 && photo.ShotAt.Year() != year {
			continue
		}
		if month > 0 && int(photo.ShotAt.Month()) != month {
			continue
		}

		dateMap[dateKey] = append(dateMap[dateKey], *photo)
	}

	// 构建时间线
	timelines := make([]Timeline, 0, len(dateMap))
	for date, photos := range dateMap {
		// 按拍摄时间排序
		sort.Slice(photos, func(i, j int) bool {
			return photos[i].ShotAt.After(photos[j].ShotAt)
		})

		// 选择精选照片（评分最高的）
		var highlight *Photo
		maxRating := 0
		for i := range photos {
			if photos[i].Rating > maxRating {
				maxRating = photos[i].Rating
				highlight = &photos[i]
			}
		}
		if highlight == nil && len(photos) > 0 {
			highlight = &photos[0]
		}

		timelines = append(timelines, Timeline{
			Date:      date,
			Photos:    photos,
			Count:     len(photos),
			Highlight: highlight,
		})
	}

	// 按日期倒序
	sort.Slice(timelines, func(i, j int) bool {
		return timelines[i].Date > timelines[j].Date
	})

	return timelines
}

// AddTag 添加标签.
func (m *Manager) AddTag(photoID, tagName, category string) (*SmartTag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.photos[photoID]; !ok {
		return nil, fmt.Errorf("photo not found: %s", photoID)
	}

	tag := &SmartTag{
		ID:         generateID(),
		Name:       tagName,
		Category:   category,
		Confidence: 1.0, // 手动添加的标签置信度为 1
	}

	m.tags[tag.ID] = tag

	// 添加到照片
	photo := m.photos[photoID]
	photo.Tags = append(photo.Tags, *tag)

	return tag, nil
}

// RemoveTag 移除标签.
func (m *Manager) RemoveTag(photoID, tagID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[photoID]
	if !ok {
		return fmt.Errorf("photo not found: %s", photoID)
	}

	// 从照片移除标签
	for i, tag := range photo.Tags {
		if tag.ID == tagID {
			photo.Tags = append(photo.Tags[:i], photo.Tags[i+1:]...)
			break
		}
	}

	delete(m.tags, tagID)
	return nil
}

// ListTags 列出所有标签.
func (m *Manager) ListTags() []SmartTag {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tags := make([]SmartTag, 0, len(m.tags))
	seen := make(map[string]bool)

	for _, tag := range m.tags {
		if !seen[tag.Name] {
			tags = append(tags, *tag)
			seen[tag.Name] = true
		}
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}

// ToggleFavorite 切换收藏状态.
func (m *Manager) ToggleFavorite(photoID string) (*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[photoID]
	if !ok {
		return nil, fmt.Errorf("photo not found: %s", photoID)
	}

	photo.IsFavorite = !photo.IsFavorite
	return photo, nil
}

// SetRating 设置评分.
func (m *Manager) SetRating(photoID string, rating int) (*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[photoID]
	if !ok {
		return nil, fmt.Errorf("photo not found: %s", photoID)
	}

	if rating < 0 || rating > 5 {
		return nil, fmt.Errorf("rating must be between 0 and 5")
	}

	photo.Rating = rating
	return photo, nil
}

// DetectDuplicates 检测重复照片.
func (m *Manager) DetectDuplicates() []DuplicateGroup {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 按文件大小分组
	sizeMap := make(map[int64][]*Photo)
	for _, photo := range m.photos {
		sizeMap[photo.Size] = append(sizeMap[photo.Size], photo)
	}

	// 找出大小相同的照片组
	for size, photos := range sizeMap {
		if len(photos) < 2 {
			continue
		}

		// 计算哈希（模拟）
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d", size))))

		group := &DuplicateGroup{
			ID:         generateID(),
			Hash:       hash,
			Similarity: 0.95 + float64(len(photos))*0.01,
			SavedSize:  size * int64(len(photos)-1),
		}

		for _, photo := range photos {
			group.PhotoIDs = append(group.PhotoIDs, photo.ID)
		}

		m.dupes[group.ID] = group
	}

	// 返回所有重复组
	groups := make([]DuplicateGroup, 0, len(m.dupes))
	for _, g := range m.dupes {
		groups = append(groups, *g)
	}

	return groups
}

// GetImportJob 获取导入任务状态.
func (m *Manager) GetImportJob(id string) (*ImportJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.imports[id]
	if !ok {
		return nil, fmt.Errorf("import job not found: %s", id)
	}
	return job, nil
}

// ListImportJobs 列出导入任务.
func (m *Manager) ListImportJobs() []ImportJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]ImportJob, 0, len(m.imports))
	for _, j := range m.imports {
		jobs = append(jobs, *j)
	}

	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].StartedAt.After(jobs[j].StartedAt)
	})

	return jobs
}

// GetStats 获取相册统计.
func (m *Manager) GetStats() *GalleryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &GalleryStats{
		TotalPhotos:     len(m.photos),
		TotalAlbums:     len(m.albums),
		TotalPersons:    len(m.persons),
		TotalFaces:      len(m.faces),
		TotalScenes:     len(m.scenes),
		TotalTags:       len(m.tags),
		TotalDuplicates: len(m.dupes),
		ImportJobs:      len(m.imports),
		ScenesBreakdown: make(map[string]int),
	}

	// 统计各场景
	for _, scene := range m.scenes {
		stats.ScenesBreakdown[scene.Category]++
	}

	// 统计收藏和隐藏
	for _, photo := range m.photos {
		if photo.IsFavorite {
			stats.FavoriteCount++
		}
		if photo.IsHidden {
			stats.HiddenCount++
		}
		stats.TotalSize += photo.Size
	}

	// 统计人物 Top 5
	personStats := make([]PersonStat, 0, len(m.persons))
	for _, p := range m.persons {
		personStats = append(personStats, PersonStat{
			Name:       p.Name,
			PhotoCount: p.PhotoCount,
		})
	}
	sort.Slice(personStats, func(i, j int) bool {
		return personStats[i].PhotoCount > personStats[j].PhotoCount
	})
	if len(personStats) > 5 {
		personStats = personStats[:5]
	}
	stats.TopPersons = personStats

	return stats
}
