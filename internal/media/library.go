package media

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// MediaType 媒体类型
type MediaType string

const (
	MediaTypeMovie MediaType = "movie"
	MediaTypeTV    MediaType = "tv"
	MediaTypeMusic MediaType = "music"
	MediaTypePhoto MediaType = "photo"
)

// MediaItem 媒体项
type MediaItem struct {
	ID           string      `json:"id"`
	Path         string      `json:"path"`
	Name         string      `json:"name"`
	Type         MediaType   `json:"type"`
	Size         int64       `json:"size"`
	ModifiedTime time.Time   `json:"modifiedTime"`
	Metadata     interface{} `json:"metadata,omitempty"`
	PosterPath   string      `json:"posterPath,omitempty"`
	IsFavorite   bool        `json:"isFavorite"`
	Tags         []string    `json:"tags"`
	Rating       float64     `json:"rating"`
	PlayCount    int         `json:"playCount"`
	LastPlayed   *time.Time  `json:"lastPlayed,omitempty"`
}

// MediaLibrary 媒体库
type MediaLibrary struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Description    string       `json:"description"`
	Path           string       `json:"path"`
	Type           MediaType    `json:"type"`
	Enabled        bool         `json:"enabled"`
	AutoScan       bool         `json:"autoScan"`
	ScanInterval   int          `json:"scanInterval"` // 分钟
	Items          []*MediaItem `json:"items,omitempty"`
	LastScanTime   *time.Time   `json:"lastScanTime,omitempty"`
	MetadataSource string       `json:"metadataSource"` // tmdb/douban/auto
	TMDBApiKey     string       `json:"tmdbApiKey,omitempty"`
	DoubanApiKey   string       `json:"doubanApiKey,omitempty"`
}

// LibraryManager 媒体库管理器
type LibraryManager struct {
	libraries         map[string]*MediaLibrary
	metadataProviders []MetadataProvider
	configPath        string
	mu                sync.RWMutex
}

// NewLibraryManager 创建媒体库管理器
func NewLibraryManager(configPath string) *LibraryManager {
	lm := &LibraryManager{
		libraries:         make(map[string]*MediaLibrary),
		metadataProviders: make([]MetadataProvider, 0),
		configPath:        configPath,
	}

	// 加载配置
	_ = lm.loadConfig()

	return lm
}

// AddMetadataProvider 添加元数据提供商
func (lm *LibraryManager) AddMetadataProvider(provider MetadataProvider) {
	lm.metadataProviders = append(lm.metadataProviders, provider)
}

// CreateLibrary 创建媒体库
func (lm *LibraryManager) CreateLibrary(name, path string, mediaType MediaType) (*MediaLibrary, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// 检查路径是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("路径不存在: %s", path)
	}

	id := fmt.Sprintf("lib_%d", time.Now().UnixNano())
	library := &MediaLibrary{
		ID:             id,
		Name:           name,
		Path:           path,
		Type:           mediaType,
		Enabled:        true,
		AutoScan:       true,
		ScanInterval:   60, // 默认 60 分钟
		Items:          make([]*MediaItem, 0),
		MetadataSource: "auto",
	}

	lm.libraries[id] = library
	if err := lm.saveConfig(); err != nil {
		return nil, err
	}

	// 自动扫描
	go func() { _ = lm.ScanLibrary(id) }()

	return library, nil
}

// GetLibrary 获取媒体库
func (lm *LibraryManager) GetLibrary(id string) *MediaLibrary {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.libraries[id]
}

// ListLibraries 列出所有媒体库
func (lm *LibraryManager) ListLibraries() []*MediaLibrary {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	libraries := make([]*MediaLibrary, 0, len(lm.libraries))
	for _, lib := range lm.libraries {
		libraries = append(libraries, lib)
	}
	return libraries
}

// UpdateLibrary 更新媒体库
func (lm *LibraryManager) UpdateLibrary(id string, updates map[string]interface{}) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	library, ok := lm.libraries[id]
	if !ok {
		return fmt.Errorf("媒体库不存在: %s", id)
	}

	// 应用更新
	for key, value := range updates {
		switch key {
		case "name":
			if v, ok := value.(string); ok {
				library.Name = v
			}
		case "description":
			if v, ok := value.(string); ok {
				library.Description = v
			}
		case "path":
			if v, ok := value.(string); ok {
				library.Path = v
			}
		case "enabled":
			if v, ok := value.(bool); ok {
				library.Enabled = v
			}
		case "autoScan":
			if v, ok := value.(bool); ok {
				library.AutoScan = v
			}
		case "scanInterval":
			if v, ok := value.(int); ok {
				library.ScanInterval = v
			}
		case "metadataSource":
			if v, ok := value.(string); ok {
				library.MetadataSource = v
			}
		case "tmdbApiKey":
			if v, ok := value.(string); ok {
				library.TMDBApiKey = v
			}
		case "doubanApiKey":
			if v, ok := value.(string); ok {
				library.DoubanApiKey = v
			}
		}
	}

	return lm.saveConfig()
}

// DeleteLibrary 删除媒体库
func (lm *LibraryManager) DeleteLibrary(id string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if _, ok := lm.libraries[id]; !ok {
		return fmt.Errorf("媒体库不存在: %s", id)
	}

	delete(lm.libraries, id)
	return lm.saveConfig()
}

// ScanLibrary 扫描媒体库
func (lm *LibraryManager) ScanLibrary(id string) error {
	lm.mu.Lock()
	library, ok := lm.libraries[id]
	if !ok {
		lm.mu.Unlock()
		return fmt.Errorf("媒体库不存在: %s", id)
	}
	lm.mu.Unlock()

	now := time.Now()
	library.LastScanTime = &now

	// 扫描文件系统
	items, err := lm.scanFileSystem(library.Path, library.Type)
	if err != nil {
		return err
	}

	// 获取元数据
	for _, item := range items {
		lm.fetchMetadata(item)
	}

	lm.mu.Lock()
	library.Items = items
	lm.mu.Unlock()

	return lm.saveConfig()
}

// scanFileSystem 扫描文件系统
func (lm *LibraryManager) scanFileSystem(rootPath string, mediaType MediaType) ([]*MediaItem, error) {
	items := make([]*MediaItem, 0)

	// 视频文件扩展名
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
		".mpg": true, ".mpeg": true, ".3gp": true, ".rmvb": true,
	}

	// 音频文件扩展名
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true, ".aac": true,
		".ogg": true, ".wma": true, ".m4a": true, ".ape": true,
	}

	// 图片文件扩展名
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".webp": true, ".heic": true, ".raw": true,
	}

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		var mediaType MediaType

		switch {
		case videoExts[ext]:
			mediaType = MediaTypeMovie
		case audioExts[ext]:
			mediaType = MediaTypeMusic
		case imageExts[ext]:
			mediaType = MediaTypePhoto
		default:
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		name := strings.TrimSuffix(filepath.Base(path), ext)

		item := &MediaItem{
			ID:           fmt.Sprintf("item_%s_%d", mediaType, time.Now().UnixNano()),
			Path:         path,
			Name:         name,
			Type:         mediaType,
			Size:         info.Size(),
			ModifiedTime: info.ModTime(),
			Tags:         make([]string, 0),
		}

		// 尝试从文件名提取年份（用于电影/电视剧）
		if mediaType == MediaTypeMovie || mediaType == MediaTypeTV {
			yearRegex := regexp.MustCompile(`\b(19|20)\d{2}\b`)
			if matches := yearRegex.FindStringSubmatch(name); len(matches) > 0 {
				item.Tags = append(item.Tags, matches[0])
			}
		}

		items = append(items, item)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return items, nil
}

// fetchMetadata 获取元数据
func (lm *LibraryManager) fetchMetadata(item *MediaItem) {
	if len(lm.metadataProviders) == 0 {
		return
	}

	// 使用第一个提供商获取元数据
	provider := lm.metadataProviders[0]

	// 根据类型搜索
	switch item.Type {
	case MediaTypeMovie:
		if results, err := provider.SearchMovie(item.Name); err == nil && len(results) > 0 {
			item.Metadata = results[0]
			item.PosterPath = results[0].PosterPath
			item.Rating = results[0].Rating
		}
	case MediaTypeTV:
		if results, err := provider.SearchTV(item.Name); err == nil && len(results) > 0 {
			item.Metadata = results[0]
			item.PosterPath = results[0].PosterPath
			item.Rating = results[0].Rating
		}
	}
}

// SearchMedia 搜索媒体
func (lm *LibraryManager) SearchMedia(query string, mediaType MediaType) ([]*MediaItem, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	results := make([]*MediaItem, 0)
	query = strings.ToLower(query)

	for _, lib := range lm.libraries {
		if !lib.Enabled {
			continue
		}
		if mediaType != "" && lib.Type != mediaType {
			continue
		}

		for _, item := range lib.Items {
			if strings.Contains(strings.ToLower(item.Name), query) {
				results = append(results, item)
			}
		}
	}

	return results, nil
}

// GetMediaWall 获取海报墙
func (lm *LibraryManager) GetMediaWall(mediaType MediaType, limit int) ([]*MediaItem, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	items := make([]*MediaItem, 0)

	for _, lib := range lm.libraries {
		if !lib.Enabled {
			continue
		}
		if mediaType != "" && lib.Type != mediaType {
			continue
		}

		for _, item := range lib.Items {
			if item.PosterPath != "" {
				items = append(items, item)
			}
		}
	}

	// 限制数量
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

// loadConfig 加载配置
func (lm *LibraryManager) loadConfig() error {
	data, err := os.ReadFile(lm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var libraries []*MediaLibrary
	if err := json.Unmarshal(data, &libraries); err != nil {
		return err
	}

	for _, lib := range libraries {
		lm.libraries[lib.ID] = lib
	}

	return nil
}

// saveConfig 保存配置
func (lm *LibraryManager) saveConfig() error {
	libraries := make([]*MediaLibrary, 0, len(lm.libraries))
	for _, lib := range lm.libraries {
		libraries = append(libraries, lib)
	}

	data, err := json.MarshalIndent(libraries, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(lm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(lm.configPath, data, 0644)
}
