package photos

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/draw"
	"image"
	"image/jpeg"
	"image/png"
	"os/exec"
	"strings"
)

// Manager 相册管理器
type Manager struct {
	dataDir        string
	photosDir      string
	thumbsDir      string
	cacheDir       string
	configPath     string
	photos         map[string]*Photo
	albums         map[string]*Album
	persons        map[string]*Person
	uploadSessions map[string]*UploadSession
	mu             sync.RWMutex
	config         *Config
}

// Config 相册配置
type Config struct {
	EnableAI         bool            `json:"enableAI"`
	EnableFaceRec    bool            `json:"enableFaceRec"`
	EnableObjectRec  bool            `json:"enableObjectRec"`
	AutoBackup       bool            `json:"autoBackup"`
	BackupPaths      []string        `json:"backupPaths"`
	ThumbnailConfig  ThumbnailConfig `json:"thumbnailConfig"`
	SupportedFormats []string        `json:"supportedFormats"`
	MaxUploadSize    int64           `json:"maxUploadSize"` // 最大上传大小（字节）
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		EnableAI:        true,
		EnableFaceRec:   true,
		EnableObjectRec: true,
		AutoBackup:      false,
		BackupPaths:     []string{},
		ThumbnailConfig: DefaultThumbnailConfig,
		SupportedFormats: []string{
			".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp",
			".heic", ".heif", ".raw", ".dng", ".cr2", ".nef", ".arw",
			".mp4", ".mov", ".avi", ".mkv", ".webm",
		},
		MaxUploadSize: 500 * 1024 * 1024, // 500MB
	}
}

// NewManager 创建相册管理器
func NewManager(dataDir string) (*Manager, error) {
	m := &Manager{
		dataDir:        dataDir,
		photosDir:      filepath.Join(dataDir, "photos"),
		thumbsDir:      filepath.Join(dataDir, "thumbnails"),
		cacheDir:       filepath.Join(dataDir, "cache"),
		configPath:     filepath.Join(dataDir, "photos-config.json"),
		photos:         make(map[string]*Photo),
		albums:         make(map[string]*Album),
		persons:        make(map[string]*Person),
		uploadSessions: make(map[string]*UploadSession),
		config:         DefaultConfig(),
	}

	// 创建目录
	if err := os.MkdirAll(m.photosDir, 0755); err != nil {
		return nil, fmt.Errorf("创建照片目录失败：%w", err)
	}
	if err := os.MkdirAll(m.thumbsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缩略图目录失败：%w", err)
	}
	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败：%w", err)
	}

	// 加载配置
	configPath := filepath.Join(dataDir, "photos-config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 配置不存在时保存默认配置
		if err := m.saveConfig(); err != nil {
			return nil, fmt.Errorf("保存默认配置失败：%w", err)
		}
	} else {
		if err := m.loadConfig(); err != nil {
			return nil, fmt.Errorf("加载配置失败：%w", err)
		}
	}

	// 加载数据
	if err := m.loadAlbums(); err != nil {
		return nil, fmt.Errorf("加载相册数据失败：%w", err)
	}

	if err := m.loadPersons(); err != nil {
		return nil, fmt.Errorf("加载人物数据失败：%w", err)
	}

	// 扫描照片库
	go m.scanPhotos()

	return m, nil
}

// loadConfig 加载配置
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, m.config)
}

// saveConfig 保存配置
func (m *Manager) saveConfig() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.configPath, data, 0644)
}

// loadAlbums 加载相册数据
func (m *Manager) loadAlbums() error {
	path := filepath.Join(m.dataDir, "albums.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var albums []Album
	if err := json.Unmarshal(data, &albums); err != nil {
		return err
	}

	for i := range albums {
		m.albums[albums[i].ID] = &albums[i]
	}
	return nil
}

// saveAlbums 保存相册数据
func (m *Manager) saveAlbums() error {
	path := filepath.Join(m.dataDir, "albums.json")
	albums := make([]Album, 0, len(m.albums))
	for _, album := range m.albums {
		albums = append(albums, *album)
	}
	data, err := json.MarshalIndent(albums, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// loadPersons 加载人物数据
func (m *Manager) loadPersons() error {
	path := filepath.Join(m.dataDir, "persons.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var persons []Person
	if err := json.Unmarshal(data, &persons); err != nil {
		return err
	}

	for i := range persons {
		m.persons[persons[i].ID] = &persons[i]
	}
	return nil
}

// savePersons 保存人物数据
func (m *Manager) savePersons() error {
	path := filepath.Join(m.dataDir, "persons.json")
	persons := make([]Person, 0, len(m.persons))
	for _, person := range m.persons {
		persons = append(persons, *person)
	}
	data, err := json.MarshalIndent(persons, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// scanPhotos 扫描照片库
func (m *Manager) scanPhotos() {
	filepath.Walk(m.photosDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, supported := range m.config.SupportedFormats {
			if ext == supported {
				// 在后台处理照片
				go m.indexPhoto(path)
				break
			}
		}
		return nil
	})
}

// indexPhoto 索引照片
func (m *Manager) indexPhoto(path string) {
	relPath, _ := filepath.Rel(m.photosDir, path)

	// 生成照片 ID
	id := uuid.New().String()

	photo := &Photo{
		ID:         id,
		Filename:   filepath.Base(path),
		Path:       relPath,
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}

	// 获取文件信息
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	photo.Size = uint64(info.Size())

	// 提取 EXIF 数据
	if exifData, err := m.extractEXIF(path); err == nil {
		photo.EXIF = exifData
		if exifData.DateTime != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", exifData.DateTime); err == nil {
				photo.TakenAt = t
			}
		}

		// 提取 GPS 信息
		if exifData.GPSLatitude != 0 || exifData.GPSLongitude != 0 {
			photo.Location = &LocationInfo{
				Latitude:  exifData.GPSLatitude,
				Longitude: exifData.GPSLongitude,
				Altitude:  exifData.GPSAltitude,
			}
		}
	}

	// 生成缩略图
	if err := m.generateThumbnails(path, id); err != nil {
		// 缩略图生成失败不影响索引
		_ = err // 明确忽略错误，避免 staticcheck 警告
	}

	m.mu.Lock()
	m.photos[id] = photo
	m.mu.Unlock()
}

// extractEXIF 提取 EXIF 数据
func (m *Manager) extractEXIF(path string) (*EXIFData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return nil, err
	}

	exifData := &EXIFData{}

	// 尝试读取各种 EXIF 标签
	if tag, err := x.Get(exif.Make); err == nil {
		exifData.Make, _ = tag.StringVal()
	}
	if tag, err := x.Get(exif.Model); err == nil {
		exifData.Model, _ = tag.StringVal()
	}
	if tag, err := x.Get(exif.DateTime); err == nil {
		exifData.DateTime, _ = tag.StringVal()
	}
	if tag, err := x.Get(exif.FNumber); err == nil {
		if rat, err := tag.Rat(0); err == nil {
			exifData.FNumber, _ = rat.Float64()
		}
	}
	if tag, err := x.Get(exif.ISOSpeedRatings); err == nil {
		if n, err := tag.Int(0); err == nil {
			exifData.ISO = n
		}
	}
	if tag, err := x.Get(exif.FocalLength); err == nil {
		if rat, err := tag.Rat(0); err == nil {
			exifData.FocalLength, _ = rat.Float64()
		}
	}
	if tag, err := x.Get(exif.Flash); err == nil {
		if n, err := tag.Int(0); err == nil {
			exifData.Flash = n != 0
		}
	}
	if tag, err := x.Get(exif.Software); err == nil {
		exifData.Software, _ = tag.StringVal()
	}
	if tag, err := x.Get(exif.Artist); err == nil {
		exifData.Artist, _ = tag.StringVal()
	}

	// GPS 信息（简化处理）

	return exifData, nil
}

// generateThumbnails 生成缩略图
func (m *Manager) generateThumbnails(srcPath string, photoID string) error {
	// 打开源图像
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 解码图像
	var srcImg image.Image
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == ".png" {
		srcImg, err = png.Decode(srcFile)
	} else {
		srcImg, err = jpeg.Decode(srcFile)
	}
	if err != nil {
		// 对于不支持的格式（如 HEIC、RAW），使用 ffmpeg
		return m.generateThumbnailsFFmpeg(srcPath, photoID)
	}

	bounds := srcImg.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 生成不同尺寸的缩略图
	sizes := []int{
		m.config.ThumbnailConfig.SmallSize,
		m.config.ThumbnailConfig.MediumSize,
		m.config.ThumbnailConfig.LargeSize,
	}

	for _, size := range sizes {
		if width <= size && height <= size {
			continue
		}

		// 计算新尺寸（保持宽高比）
		newWidth, newHeight := resizeDimensions(width, height, size)

		// 创建新图像
		dstImg := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

		// 缩放图像
		draw.BiLinear.Scale(dstImg, dstImg.Bounds(), srcImg, srcImg.Bounds(), draw.Over, nil)

		// 保存缩略图
		thumbPath := filepath.Join(m.thumbsDir, fmt.Sprintf("%s_%d.jpg", photoID, size))
		thumbFile, err := os.Create(thumbPath)
		if err != nil {
			continue
		}

		err = jpeg.Encode(thumbFile, dstImg, &jpeg.Options{Quality: m.config.ThumbnailConfig.Quality})
		thumbFile.Close()
		if err != nil {
			continue
		}
	}

	return nil
}

// generateThumbnailsFFmpeg 使用 ffmpeg 生成缩略图（支持 HEIC、RAW、视频）
func (m *Manager) generateThumbnailsFFmpeg(srcPath string, photoID string) error {
	sizes := []int{
		m.config.ThumbnailConfig.SmallSize,
		m.config.ThumbnailConfig.MediumSize,
		m.config.ThumbnailConfig.LargeSize,
	}

	for _, size := range sizes {
		thumbPath := filepath.Join(m.thumbsDir, fmt.Sprintf("%s_%d.jpg", photoID, size))

		// ffmpeg 命令
		cmd := exec.Command("ffmpeg", "-i", srcPath,
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", size, size),
			"-vframes", "1",
			"-q:v", "2",
			thumbPath)

		cmd.Run()
	}

	return nil
}

// resizeDimensions 计算缩放后的尺寸
func resizeDimensions(width, height, maxSize int) (int, int) {
	if width > height {
		if width <= maxSize {
			return width, height
		}
		newWidth := maxSize
		newHeight := height * maxSize / width
		return newWidth, newHeight
	} else {
		if height <= maxSize {
			return width, height
		}
		newHeight := maxSize
		newWidth := width * maxSize / height
		return newWidth, newHeight
	}
}

// CreateAlbum 创建相册
func (m *Manager) CreateAlbum(name, description, userID string) (*Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	album := &Album{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		UserID:      userID,
		PhotoCount:  0,
		IsShared:    false,
		IsFavorite:  false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		SharedWith:  []ShareTarget{},
		Tags:        []string{},
	}

	m.albums[album.ID] = album

	if err := m.saveAlbums(); err != nil {
		return nil, err
	}

	return album, nil
}

// UpdateAlbum 更新相册
func (m *Manager) UpdateAlbum(albumID, name, description string) (*Album, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	album, exists := m.albums[albumID]
	if !exists {
		return nil, fmt.Errorf("相册不存在")
	}

	if name != "" {
		album.Name = name
	}
	if description != "" {
		album.Description = description
	}
	album.UpdatedAt = time.Now()

	m.albums[albumID] = album

	if err := m.saveAlbums(); err != nil {
		return nil, err
	}

	return album, nil
}

// DeleteAlbum 删除相册
func (m *Manager) DeleteAlbum(albumID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.albums[albumID]; !exists {
		return fmt.Errorf("相册不存在")
	}

	delete(m.albums, albumID)

	if err := m.saveAlbums(); err != nil {
		return err
	}

	return nil
}

// GetAlbum 获取相册详情
func (m *Manager) GetAlbum(albumID string) (*Album, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	album, exists := m.albums[albumID]
	if !exists {
		return nil, fmt.Errorf("相册不存在")
	}

	// 复制一份返回
	albumCopy := *album
	return &albumCopy, nil
}

// ListAlbums 列出相册
func (m *Manager) ListAlbums(userID string) []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Album, 0)
	for _, album := range m.albums {
		if userID != "" && album.UserID != userID {
			continue
		}
		albumCopy := *album
		result = append(result, &albumCopy)
	}

	return result
}

// AddPhotoToAlbum 添加照片到相册
func (m *Manager) AddPhotoToAlbum(photoID, albumID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, photoExists := m.photos[photoID]
	if !photoExists {
		return fmt.Errorf("照片不存在")
	}

	album, albumExists := m.albums[albumID]
	if !albumExists {
		return fmt.Errorf("相册不存在")
	}

	photo.AlbumID = albumID
	album.PhotoCount++

	if album.CoverPhotoID == "" {
		album.CoverPhotoID = photoID
	}
	album.UpdatedAt = time.Now()

	if err := m.saveAlbums(); err != nil {
		return err
	}

	return nil
}

// RemovePhotoFromAlbum 从相册移除照片
func (m *Manager) RemovePhotoFromAlbum(photoID, albumID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return fmt.Errorf("照片不存在")
	}

	album, albumExists := m.albums[albumID]
	if !albumExists {
		return fmt.Errorf("相册不存在")
	}

	if photo.AlbumID == albumID {
		photo.AlbumID = ""
		album.PhotoCount--

		if album.CoverPhotoID == photoID {
			album.CoverPhotoID = ""
			// 重新设置封面
			for _, p := range m.photos {
				if p.AlbumID == albumID {
					album.CoverPhotoID = p.ID
					break
				}
			}
		}
		album.UpdatedAt = time.Now()
	}

	if err := m.saveAlbums(); err != nil {
		return err
	}

	return nil
}

// QueryPhotos 查询照片
func (m *Manager) QueryPhotos(query *PhotoQuery) ([]*Photo, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Photo, 0)

	for _, photo := range m.photos {
		// 应用过滤条件
		if query.AlbumID != "" && photo.AlbumID != query.AlbumID {
			continue
		}
		if query.UserID != "" && photo.UserID != query.UserID {
			continue
		}
		if !query.StartDate.IsZero() && photo.TakenAt.Before(query.StartDate) {
			continue
		}
		if !query.EndDate.IsZero() && photo.TakenAt.After(query.EndDate) {
			continue
		}
		if query.IsFavorite != nil && photo.IsFavorite != *query.IsFavorite {
			continue
		}
		if query.IsHidden != nil && photo.IsHidden != *query.IsHidden {
			continue
		}
		if query.MimeType != "" && !strings.HasPrefix(photo.MimeType, query.MimeType) {
			continue
		}

		result = append(result, photo)
	}

	// 排序
	sort.Slice(result, func(i, j int) bool {
		switch query.SortBy {
		case "date", "takenAt", "":
			// 默认按拍摄时间降序
			if query.SortOrder == "asc" {
				return result[i].TakenAt.Before(result[j].TakenAt)
			}
			return result[i].TakenAt.After(result[j].TakenAt)
		case "name", "filename":
			if query.SortOrder == "desc" {
				return result[i].Filename > result[j].Filename
			}
			return result[i].Filename < result[j].Filename
		case "size":
			if query.SortOrder == "desc" {
				return result[i].Size > result[j].Size
			}
			return result[i].Size < result[j].Size
		case "uploadedAt", "createdAt":
			if query.SortOrder == "desc" {
				return result[i].UploadedAt.After(result[j].UploadedAt)
			}
			return result[i].UploadedAt.Before(result[j].UploadedAt)
		case "modifiedAt":
			if query.SortOrder == "desc" {
				return result[i].ModifiedAt.After(result[j].ModifiedAt)
			}
			return result[i].ModifiedAt.Before(result[j].ModifiedAt)
		default:
			// 默认按拍摄时间降序
			return result[i].TakenAt.After(result[j].TakenAt)
		}
	})

	total := len(result)

	// 分页
	if query.Offset > 0 {
		if query.Offset >= len(result) {
			return []*Photo{}, total, nil
		}
		result = result[query.Offset:]
	}
	if query.Limit > 0 && len(result) > query.Limit {
		result = result[:query.Limit]
	}

	return result, total, nil
}

// GetPhoto 获取照片详情
func (m *Manager) GetPhoto(photoID string) (*Photo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return nil, fmt.Errorf("照片不存在")
	}

	photoCopy := *photo
	return &photoCopy, nil
}

// DeletePhoto 删除照片
func (m *Manager) DeletePhoto(photoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return fmt.Errorf("照片不存在")
	}

	// 删除文件
	photoPath := filepath.Join(m.photosDir, photo.Path)
	os.Remove(photoPath)

	// 删除缩略图
	filepath.Glob(filepath.Join(m.thumbsDir, fmt.Sprintf("%s_*.jpg", photoID)))
	for _, thumbPath := range findFiles(filepath.Join(m.thumbsDir, fmt.Sprintf("%s_*.jpg", photoID))) {
		os.Remove(thumbPath)
	}

	delete(m.photos, photoID)

	return nil
}

// ToggleFavorite 切换收藏状态
func (m *Manager) ToggleFavorite(photoID string) (*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, exists := m.photos[photoID]
	if !exists {
		return nil, fmt.Errorf("照片不存在")
	}

	photo.IsFavorite = !photo.IsFavorite
	photo.ModifiedAt = time.Now()

	photoCopy := *photo
	return &photoCopy, nil
}

// CreatePerson 创建人物
func (m *Manager) CreatePerson(name string) (*Person, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	person := &Person{
		ID:         uuid.New().String(),
		Name:       name,
		PhotoCount: 0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	m.persons[person.ID] = person

	if err := m.savePersons(); err != nil {
		return nil, err
	}

	return person, nil
}

// ListPersons 列出人物
func (m *Manager) ListPersons() []*Person {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Person, 0)
	for _, person := range m.persons {
		personCopy := *person
		result = append(result, &personCopy)
	}

	return result
}

// UpdatePerson 更新人物
func (m *Manager) UpdatePerson(personID, name string) (*Person, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	person, exists := m.persons[personID]
	if !exists {
		return nil, fmt.Errorf("人物不存在")
	}

	if name != "" {
		person.Name = name
	}
	person.UpdatedAt = time.Now()

	personCopy := *person
	return &personCopy, nil
}

// DeletePerson 删除人物
func (m *Manager) DeletePerson(personID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.persons[personID]; !exists {
		return fmt.Errorf("人物不存在")
	}

	delete(m.persons, personID)

	if err := m.savePersons(); err != nil {
		return err
	}

	return nil
}

// GetTimeline 获取时间线
func (m *Manager) GetTimeline(userID string, groupBy string) ([]*TimelineGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make(map[string]*TimelineGroup)

	for _, photo := range m.photos {
		if userID != "" && photo.UserID != userID {
			continue
		}

		var period string
		switch groupBy {
		case "year":
			period = photo.TakenAt.Format("2006")
		case "month":
			period = photo.TakenAt.Format("2006-01")
		case "day":
			period = photo.TakenAt.Format("2006-01-02")
		default:
			period = photo.TakenAt.Format("2006-01")
		}

		if _, exists := groups[period]; !exists {
			groups[period] = &TimelineGroup{
				Period: period,
				Photos: make([]*Photo, 0),
			}
		}

		photoCopy := *photo
		groups[period].Photos = append(groups[period].Photos, &photoCopy)
		groups[period].Count++
	}

	result := make([]*TimelineGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, group)
	}

	return result, nil
}

// findFiles 辅助函数：查找匹配的文件
func findFiles(pattern string) []string {
	matches, _ := filepath.Glob(pattern)
	return matches
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *Config {
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *Config) error {
	m.config = config
	return m.saveConfig()
}
