// Package mediaposter 智能影视海报墙模块
// 自动刮削影视信息、海报展示、分类浏览
// 对标飞牛fnOS智能影视海报墙功能
package mediaposter

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MediaType 媒体类型
type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"
	MediaTypeTVShow MediaType = "tvshow"
	MediaTypeMusic  MediaType = "music"
)

// MediaItem 媒体项目
type MediaItem struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	TitleCN    string            `json:"title_cn,omitempty"`
	Year       int               `json:"year,omitempty"`
	Type       MediaType         `json:"type"`
	Poster     string            `json:"poster,omitempty"`
	Backdrop   string            `json:"backdrop,omitempty"`
	Rating     float64           `json:"rating,omitempty"`
	Genres     []string          `json:"genres,omitempty"`
	Overview   string            `json:"overview,omitempty"`
	Duration   int               `json:"duration,omitempty"` // 分钟
	Director   string            `json:"director,omitempty"`
	Cast       []string          `json:"cast,omitempty"`
	FilePath   string            `json:"file_path"`
	FileSize   int64             `json:"file_size"`
	Resolution string            `json:"resolution,omitempty"`
	Codec      string            `json:"codec,omitempty"`
	AddedAt    time.Time         `json:"added_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Tags       []string          `json:"tags,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// TVSeason 电视剧季
type TVSeason struct {
	SeasonNumber int         `json:"season_number"`
	Episodes     []TVEpisode `json:"episodes"`
	Poster       string      `json:"poster,omitempty"`
}

// TVEpisode 电视剧集
type TVEpisode struct {
	EpisodeNumber int       `json:"episode_number"`
	Title         string    `json:"title"`
	Overview      string    `json:"overview,omitempty"`
	Duration      int       `json:"duration,omitempty"`
	FilePath      string    `json:"file_path"`
	AddedAt       time.Time `json:"added_at"`
}

// Library 媒体库
type Library struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Type      MediaType `json:"type"`
	Count     int       `json:"count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query     string    `json:"query"`
	Type      MediaType `json:"type,omitempty"`
	Genre     string    `json:"genre,omitempty"`
	Year      int       `json:"year,omitempty"`
	MinRating float64   `json:"min_rating,omitempty"`
	SortBy    string    `json:"sort_by,omitempty"`    // title, year, rating, added_at
	SortOrder string    `json:"sort_order,omitempty"` // asc, desc
	Page      int       `json:"page,omitempty"`
	PageSize  int       `json:"page_size,omitempty"`
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Items      []MediaItem `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// PosterWallConfig 海报墙配置
type PosterWallConfig struct {
	Libraries       []Library `json:"libraries"`
	AutoScan        bool      `json:"auto_scan"`
	ScanInterval    int       `json:"scan_interval"`   // 分钟
	MetadataSource  string    `json:"metadata_source"` // tmdb, douban, imdb
	APIKey          string    `json:"api_key,omitempty"`
	EnableNFO       bool      `json:"enable_nfo"`
	EnableSubtitles bool      `json:"enable_subtitles"`
	ThumbnailSize   string    `json:"thumbnail_size"` // small, medium, large
}

// Service 海报墙服务
type Service struct {
	mu         sync.RWMutex
	config     *PosterWallConfig
	items      map[string]*MediaItem
	libraries  map[string]*Library
	index      map[string][]string // 索引: genre -> item ids
	httpClient *http.Client
}

// NewService 创建海报墙服务
func NewService(config *PosterWallConfig) *Service {
	if config == nil {
		config = &PosterWallConfig{
			AutoScan:       true,
			ScanInterval:   30,
			MetadataSource: "tmdb",
			EnableNFO:      true,
			ThumbnailSize:  "medium",
		}
	}

	return &Service{
		config:     config,
		items:      make(map[string]*MediaItem),
		libraries:  make(map[string]*Library),
		index:      make(map[string][]string),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// AddLibrary 添加媒体库
func (s *Service) AddLibrary(ctx context.Context, lib *Library) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lib.ID == "" {
		lib.ID = generateID()
	}
	lib.CreatedAt = time.Now()
	lib.UpdatedAt = time.Now()

	s.libraries[lib.ID] = lib
	return nil
}

// RemoveLibrary 移除媒体库
func (s *Service) RemoveLibrary(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.libraries[id]; !exists {
		return fmt.Errorf("library not found: %s", id)
	}

	delete(s.libraries, id)
	return nil
}

// GetLibrary 获取媒体库
func (s *Service) GetLibrary(ctx context.Context, id string) (*Library, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lib, exists := s.libraries[id]
	if !exists {
		return nil, fmt.Errorf("library not found: %s", id)
	}
	return lib, nil
}

// ListLibraries 列出所有媒体库
func (s *Service) ListLibraries(ctx context.Context) []*Library {
	s.mu.RLock()
	defer s.mu.RUnlock()

	libs := make([]*Library, 0, len(s.libraries))
	for _, lib := range s.libraries {
		libs = append(libs, lib)
	}
	return libs
}

// ScanLibrary 扫描媒体库
func (s *Service) ScanLibrary(ctx context.Context, libraryID string) error {
	s.mu.Lock()
	lib, exists := s.libraries[libraryID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("library not found: %s", libraryID)
	}
	s.mu.Unlock()

	// 扫描目录
	_ = filepath.Walk(lib.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if isMediaFile(ext) {
			item := &MediaItem{
				ID:        generateID(),
				Title:     strings.TrimSuffix(info.Name(), ext),
				Type:      lib.Type,
				FilePath:  path,
				FileSize:  info.Size(),
				AddedAt:   info.ModTime(),
				UpdatedAt: time.Now(),
			}

			// 尝试读取NFO文件获取元数据
			if s.config.EnableNFO {
				s.loadNFO(path, item)
			}

			s.mu.Lock()
			s.items[item.ID] = item
			s.updateIndex(item)
			lib.Count++
			s.mu.Unlock()
		}

		return nil
	})

	s.mu.Lock()
	lib.UpdatedAt = time.Now()
	s.mu.Unlock()

	return nil
}

// Search 搜索媒体
func (s *Service) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*MediaItem

	// 获取候选集
	if req.Genre != "" {
		if ids, ok := s.index[req.Genre]; ok {
			for _, id := range ids {
				if item, exists := s.items[id]; exists {
					results = append(results, item)
				}
			}
		}
	} else {
		for _, item := range s.items {
			results = append(results, item)
		}
	}

	// 过滤
	filtered := make([]*MediaItem, 0)
	for _, item := range results {
		if req.Type != "" && item.Type != req.Type {
			continue
		}
		if req.Year > 0 && item.Year != req.Year {
			continue
		}
		if req.MinRating > 0 && item.Rating < req.MinRating {
			continue
		}
		if req.Query != "" && !strings.Contains(strings.ToLower(item.Title), strings.ToLower(req.Query)) {
			continue
		}
		filtered = append(filtered, item)
	}

	// 排序
	s.sortItems(filtered, req.SortBy, req.SortOrder)

	// 分页
	total := len(filtered)
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

	items := make([]MediaItem, end-start)
	for i, item := range filtered[start:end] {
		items[i] = *item
	}

	return &SearchResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// GetMediaItem 获取媒体详情
func (s *Service) GetMediaItem(ctx context.Context, id string) (*MediaItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.items[id]
	if !exists {
		return nil, fmt.Errorf("media item not found: %s", id)
	}
	return item, nil
}

// UpdateMetadata 更新元数据
func (s *Service) UpdateMetadata(ctx context.Context, id string, metadata map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.items[id]
	if !exists {
		return fmt.Errorf("media item not found: %s", id)
	}

	if item.Metadata == nil {
		item.Metadata = make(map[string]string)
	}

	for k, v := range metadata {
		item.Metadata[k] = v
	}
	item.UpdatedAt = time.Now()

	return nil
}

// GetGenres 获取所有类型
func (s *Service) GetGenres(ctx context.Context) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	genres := make([]string, 0, len(s.index))
	for genre := range s.index {
		genres = append(genres, genre)
	}
	return genres
}

// GetRecentAdded 获取最近添加
func (s *Service) GetRecentAdded(ctx context.Context, limit int) []MediaItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*MediaItem, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}

	// 按添加时间排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].AddedAt.After(items[j].AddedAt)
	})

	if limit > len(items) {
		limit = len(items)
	}

	result := make([]MediaItem, limit)
	for i := 0; i < limit; i++ {
		result[i] = *items[i]
	}

	return result
}

// 内部方法

func (s *Service) loadNFO(mediaPath string, item *MediaItem) {
	nfoPath := strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath)) + ".nfo"
	// 实现NFO文件读取逻辑
	_ = nfoPath
}

func (s *Service) updateIndex(item *MediaItem) {
	for _, genre := range item.Genres {
		if s.index[genre] == nil {
			s.index[genre] = make([]string, 0)
		}
		s.index[genre] = append(s.index[genre], item.ID)
	}
}

func (s *Service) sortItems(items []*MediaItem, sortBy, sortOrder string) {
	// 实现排序逻辑
}

func isMediaFile(ext string) bool {
	mediaExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".m4v": true, ".ts": true,
		".mp3": true, ".flac": true, ".wav": true, ".aac": true,
	}
	return mediaExts[ext]
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
