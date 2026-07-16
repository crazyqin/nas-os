// Package smartgallery 智能海报墙 - 影视海报展示与管理
// 对标飞牛fnOS海报墙功能
package smartgallery

import (
	"errors"
	"sync"
	"time"
)

// MediaType 媒体类型.
type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"
	MediaTypeTVShow MediaType = "tv_show"
	MediaTypeMusic  MediaType = "music"
	MediaTypePhoto  MediaType = "photo"
)

// PosterStatus 海报状态.
type PosterStatus string

const (
	PosterStatusPending  PosterStatus = "pending"
	PosterStatusFetching PosterStatus = "fetching"
	PosterStatusReady    PosterStatus = "ready"
	PosterStatusFailed   PosterStatus = "failed"
)

// MediaItem 媒体项.
type MediaItem struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Type         MediaType         `json:"type"`
	Year         int               `json:"year,omitempty"`
	Rating       float64           `json:"rating,omitempty"`
	Genres       []string          `json:"genres,omitempty"`
	Directors    []string          `json:"directors,omitempty"`
	Actors       []string          `json:"actors,omitempty"`
	Summary      string            `json:"summary,omitempty"`
	PosterURL    string            `json:"poster_url,omitempty"`
	BackdropURL  string            `json:"backdrop_url,omitempty"`
	FilePath     string            `json:"file_path"`
	FileSize     int64             `json:"file_size"`
	Duration     int               `json:"duration,omitempty"` // 秒
	Resolution   string            `json:"resolution,omitempty"`
	PosterStatus PosterStatus      `json:"poster_status"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// GalleryConfig 画廊配置.
type GalleryConfig struct {
	AutoFetchPoster   bool     `json:"auto_fetch_poster"`
	PosterLanguage    string   `json:"poster_language"` // zh-CN, en-US
	TMDBAPIKey        string   `json:"tmdb_api_key,omitempty"`
	ScanPaths         []string `json:"scan_paths"`
	ScanInterval      int      `json:"scan_interval"` // 分钟
	MaxConcurrent     int      `json:"max_concurrent"`
	EnableAIRecommend bool     `json:"enable_ai_recommend"`
}

// DefaultGalleryConfig 默认画廊配置.
func DefaultGalleryConfig() *GalleryConfig {
	return &GalleryConfig{
		AutoFetchPoster:   true,
		PosterLanguage:    "zh-CN",
		ScanInterval:      60,
		MaxConcurrent:     5,
		EnableAIRecommend: false,
	}
}

// Gallery 海报墙管理器.
type Gallery struct {
	mu       sync.RWMutex
	config   *GalleryConfig
	items    map[string]*MediaItem // id -> item
	byPath   map[string]string     // path -> id
	scanning bool
}

// NewGallery 创建海报墙管理器.
func NewGallery(config *GalleryConfig) *Gallery {
	if config == nil {
		config = DefaultGalleryConfig()
	}

	return &Gallery{
		config: config,
		items:  make(map[string]*MediaItem),
		byPath: make(map[string]string),
	}
}

// Add 添加媒体项.
func (g *Gallery) Add(item *MediaItem) error {
	if item == nil {
		return errors.New("item is nil")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// 检查路径是否已存在
	if existingID, exists := g.byPath[item.FilePath]; exists {
		return errors.New("file already exists: " + existingID)
	}

	// 设置时间戳
	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now

	// 设置海报状态
	if item.PosterURL == "" {
		item.PosterStatus = PosterStatusPending
	} else {
		item.PosterStatus = PosterStatusReady
	}

	g.items[item.ID] = item
	g.byPath[item.FilePath] = item.ID

	return nil
}

// Get 获取媒体项.
func (g *Gallery) Get(id string) (*MediaItem, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	item, exists := g.items[id]
	return item, exists
}

// GetByPath 通过路径获取.
func (g *Gallery) GetByPath(path string) (*MediaItem, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	id, exists := g.byPath[path]
	if !exists {
		return nil, false
	}

	return g.items[id], true
}

// Update 更新媒体项.
func (g *Gallery) Update(id string, update func(*MediaItem)) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	item, exists := g.items[id]
	if !exists {
		return errors.New("item not found: " + id)
	}

	update(item)
	item.UpdatedAt = time.Now()

	return nil
}

// Delete 删除媒体项.
func (g *Gallery) Delete(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	item, exists := g.items[id]
	if !exists {
		return errors.New("item not found: " + id)
	}

	delete(g.byPath, item.FilePath)
	delete(g.items, id)

	return nil
}

// List 列出媒体项.
func (g *Gallery) List(filter *MediaFilter) []*MediaItem {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]*MediaItem, 0)

	for _, item := range g.items {
		if filter != nil && !filter.Match(item) {
			continue
		}
		result = append(result, item)
	}

	return result
}

// MediaFilter 媒体过滤器.
type MediaFilter struct {
	Type      *MediaType `json:"type,omitempty"`
	Genre     string     `json:"genre,omitempty"`
	Year      int        `json:"year,omitempty"`
	MinRating float64    `json:"min_rating,omitempty"`
	Search    string     `json:"search,omitempty"`
}

// Match 检查是否匹配.
func (f *MediaFilter) Match(item *MediaItem) bool {
	if f == nil {
		return true
	}

	if f.Type != nil && item.Type != *f.Type {
		return false
	}

	if f.Genre != "" {
		found := false
		for _, g := range item.Genres {
			if g == f.Genre {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if f.Year > 0 && item.Year != f.Year {
		return false
	}

	if f.MinRating > 0 && item.Rating < f.MinRating {
		return false
	}

	if f.Search != "" {
		// 简单搜索匹配
		search := f.Search
		title := item.Title
		if !containsIgnoreCase(title, search) && !containsIgnoreCase(item.Summary, search) {
			return false
		}
	}

	return true
}

// containsIgnoreCase 忽略大小写包含检查.
func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) == 0 {
		return false
	}

	// 简单实现
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 32
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 32
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Count 统计数量.
func (g *Gallery) Count() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return len(g.items)
}

// GetStats 获取统计信息.
func (g *Gallery) GetStats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := map[string]interface{}{
		"total_items": len(g.items),
		"by_type":     make(map[MediaType]int),
		"by_status":   make(map[PosterStatus]int),
	}

	byType := stats["by_type"].(map[MediaType]int)
	byStatus := stats["by_status"].(map[PosterStatus]int)

	for _, item := range g.items {
		byType[item.Type]++
		byStatus[item.PosterStatus]++
	}

	return stats
}

// GetGenres 获取所有类型.
func (g *Gallery) GetGenres() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	genreSet := make(map[string]bool)
	for _, item := range g.items {
		for _, genre := range item.Genres {
			genreSet[genre] = true
		}
	}

	genres := make([]string, 0, len(genreSet))
	for genre := range genreSet {
		genres = append(genres, genre)
	}

	return genres
}

// GetRecent 获取最近添加.
func (g *Gallery) GetRecent(limit int) []*MediaItem {
	g.mu.RLock()
	defer g.mu.RUnlock()

	items := make([]*MediaItem, 0, len(g.items))
	for _, item := range g.items {
		items = append(items, item)
	}

	// 按创建时间排序（简单冒泡）
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].CreatedAt.After(items[i].CreatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	return items
}

// GetTopRated 获取评分最高.
func (g *Gallery) GetTopRated(limit int) []*MediaItem {
	g.mu.RLock()
	defer g.mu.RUnlock()

	items := make([]*MediaItem, 0, len(g.items))
	for _, item := range g.items {
		if item.Rating > 0 {
			items = append(items, item)
		}
	}

	// 按评分排序
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Rating > items[i].Rating {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	return items
}
