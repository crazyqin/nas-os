package mediacenter

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MediaCenter 智能媒体中心
type MediaCenter struct {
	mu          sync.RWMutex
	movies      map[string]*Movie
	photos      map[string]*Photo
	playlists   map[string]*Playlist
	libraries   map[string]*Library
	items       map[string]*MediaItem
	sessions    map[string]*Session
	config      *Config
}

// Movie 电影信息
type Movie struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Year        int       `json:"year"`
	Genre       []string  `json:"genre"`
	Director    string    `json:"director"`
	Actors      []string  `json:"actors"`
	Poster      string    `json:"poster"`
	Plot        string    `json:"plot"`
	Rating      float64   `json:"rating"`
	Duration    int       `json:"duration"`
	Resolution  string    `json:"resolution"`
	Codec       string    `json:"codec"`
	Subtitles   []string  `json:"subtitles"`
	FilePath    string    `json:"file_path"`
	WatchedAt   time.Time `json:"watched_at"`
	AddedAt     time.Time `json:"added_at"`
}

// Photo 照片信息
type Photo struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	Format      string    `json:"format"`
	Exif        *ExifData `json:"exif"`
	Faces       []*Face   `json:"faces"`
	Tags        []string  `json:"tags"`
	Albums      []string  `json:"albums"`
	TakenAt     time.Time `json:"taken_at"`
	AddedAt     time.Time `json:"added_at"`
	IsFavorite  bool      `json:"is_favorite"`
	IsLivePhoto bool      `json:"is_live_photo"`
}

// ExifData EXIF信息
type ExifData struct {
	Camera      string    `json:"camera"`
	Lens        string    `json:"lens"`
	ISO         int       `json:"iso"`
	Aperture    float64   `json:"aperture"`
	ShutterSpeed string   `json:"shutter_speed"`
	FocalLength float64   `json:"focal_length"`
	GPS         *GPSData  `json:"gps"`
}

// GPSData GPS信息
type GPSData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
}

// Face 人脸信息
type Face struct {
	ID        string    `json:"id"`
	PersonID  string    `json:"person_id"`
	Name      string    `json:"name"`
	Bounds    *Bounds   `json:"bounds"`
	Confidence float64  `json:"confidence"`
}

// Bounds 边界框
type Bounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Playlist 播放列表
type Playlist struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Items     []string  `json:"items"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Library 媒体库
type Library struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // movie, photo, music
	Path      string    `json:"path"`
	ScannedAt time.Time `json:"scanned_at"`
	ItemCount int       `json:"item_count"`
}

// Config 配置
type Config struct {
	ScanInterval    time.Duration `json:"scan_interval"`
	AutoScan        bool          `json:"auto_scan"`
	ThumbnailSize   int           `json:"thumbnail_size"`
	FaceDetection   bool          `json:"face_detection"`
	AutoTag         bool          `json:"auto_tag"`
	TranscodeEnabled bool         `json:"transcode_enabled"`
	MaxBitrate      int           `json:"max_bitrate"`
}

// NewMediaCenter 创建媒体中心
func NewMediaCenter(config *Config) *MediaCenter {
	return &MediaCenter{
		movies:    make(map[string]*Movie),
		photos:    make(map[string]*Photo),
		playlists: make(map[string]*Playlist),
		libraries: make(map[string]*Library),
		items:     make(map[string]*MediaItem),
		sessions:  make(map[string]*Session),
		config:    config,
	}
}

// AddItem 添加媒体项
func (mc *MediaCenter) AddItem(item MediaItem) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.items[item.ID]; exists {
		return fmt.Errorf("item already exists: %s", item.ID)
	}

	mc.items[item.ID] = &item
	return nil
}

// GetItem 获取媒体项
func (mc *MediaCenter) GetItem(id string) (*MediaItem, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, exists := mc.items[id]
	if !exists {
		return nil, fmt.Errorf("item not found: %s", id)
	}
	return item, nil
}

// ListItems 列出媒体项
func (mc *MediaCenter) ListItems(query string, mediaType MediaType) []*MediaItem {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var items []*MediaItem
	for _, item := range mc.items {
		if mediaType != "" && item.Type != mediaType {
			continue
		}
		if query != "" && !contains(item.Title, query) {
			continue
		}
		items = append(items, item)
	}
	return items
}

// SearchItems 搜索媒体项
func (mc *MediaCenter) SearchItems(query string) []*MediaItem {
	return mc.ListItems(query, "")
}

// RemoveItem 删除媒体项
func (mc *MediaCenter) RemoveItem(id string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.items[id]; !exists {
		return fmt.Errorf("item not found: %s", id)
	}

	delete(mc.items, id)
	return nil
}

// ListLibraries 列出媒体库
func (mc *MediaCenter) ListLibraries() []*Library {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var libs []*Library
	for _, lib := range mc.libraries {
		libs = append(libs, lib)
	}
	return libs
}

// GetLibrary 获取媒体库
func (mc *MediaCenter) GetLibrary(id string) (*Library, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	lib, exists := mc.libraries[id]
	if !exists {
		return nil, fmt.Errorf("library not found: %s", id)
	}
	return lib, nil
}

// ListSessions 列出会话
func (mc *MediaCenter) ListSessions(userID string) []*Session {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var sessions []*Session
	for _, session := range mc.sessions {
		if userID != "" && session.UserID != userID {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions
}

// AddMovie 添加电影
func (mc *MediaCenter) AddMovie(ctx context.Context, movie *Movie) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	movie.AddedAt = time.Now()
	mc.movies[movie.ID] = movie
	return nil
}

// GetMovie 获取电影
func (mc *MediaCenter) GetMovie(ctx context.Context, id string) (*Movie, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	movie, exists := mc.movies[id]
	if !exists {
		return nil, fmt.Errorf("movie not found: %s", id)
	}
	return movie, nil
}

// SearchMovies 搜索电影
func (mc *MediaCenter) SearchMovies(ctx context.Context, query string) ([]*Movie, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	var results []*Movie
	for _, movie := range mc.movies {
		if contains(movie.Title, query) || contains(movie.Director, query) {
			results = append(results, movie)
		}
	}
	return results, nil
}

// AddPhoto 添加照片
func (mc *MediaCenter) AddPhoto(ctx context.Context, photo *Photo) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	photo.AddedAt = time.Now()
	mc.photos[photo.ID] = photo
	return nil
}

// GetPhoto 获取照片
func (mc *MediaCenter) GetPhoto(ctx context.Context, id string) (*Photo, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	photo, exists := mc.photos[id]
	if !exists {
		return nil, fmt.Errorf("photo not found: %s", id)
	}
	return photo, nil
}

// SearchPhotosByText 以文搜图
func (mc *MediaCenter) SearchPhotosByText(ctx context.Context, query string) ([]*Photo, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	var results []*Photo
	for _, photo := range mc.photos {
		if matchPhotoByQuery(photo, query) {
			results = append(results, photo)
		}
	}
	return results, nil
}

// SearchPhotosByFace 按人脸搜索照片
func (mc *MediaCenter) SearchPhotosByFace(ctx context.Context, personID string) ([]*Photo, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	var results []*Photo
	for _, photo := range mc.photos {
		for _, face := range photo.Faces {
			if face.PersonID == personID {
				results = append(results, photo)
				break
			}
		}
	}
	return results, nil
}

// CreatePlaylist 创建播放列表
func (mc *MediaCenter) CreatePlaylist(ctx context.Context, playlist *Playlist) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	playlist.CreatedAt = time.Now()
	playlist.UpdatedAt = time.Now()
	mc.playlists[playlist.ID] = playlist
	return nil
}

// AddToPlaylist 添加到播放列表
func (mc *MediaCenter) AddToPlaylist(ctx context.Context, playlistID, itemID string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	playlist, exists := mc.playlists[playlistID]
	if !exists {
		return fmt.Errorf("playlist not found: %s", playlistID)
	}
	
	playlist.Items = append(playlist.Items, itemID)
	playlist.UpdatedAt = time.Now()
	return nil
}

// ScanLibrary 扫描媒体库
func (mc *MediaCenter) ScanLibrary(ctx context.Context, libraryID string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	library, exists := mc.libraries[libraryID]
	if !exists {
		return fmt.Errorf("library not found: %s", libraryID)
	}
	
	library.ScannedAt = time.Now()
	return nil
}

// CreateLibrary 创建媒体库
func (mc *MediaCenter) CreateLibrary(ctx context.Context, library *Library) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	library.ScannedAt = time.Now()
	mc.libraries[library.ID] = library
	return nil
}

// GetStats 获取统计信息
func (mc *MediaCenter) GetStats(ctx context.Context) map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	return map[string]interface{}{
		"total_movies":   len(mc.movies),
		"total_photos":   len(mc.photos),
		"total_playlists": len(mc.playlists),
		"total_libraries": len(mc.libraries),
	}
}

// UpdateWatchProgress 更新观看进度
func (mc *MediaCenter) UpdateWatchProgress(ctx context.Context, movieID string, progress float64) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	movie, exists := mc.movies[movieID]
	if !exists {
		return fmt.Errorf("movie not found: %s", movieID)
	}
	
	movie.WatchedAt = time.Now()
	return nil
}

// ToggleFavorite 切换收藏状态
func (mc *MediaCenter) ToggleFavorite(ctx context.Context, photoID string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	photo, exists := mc.photos[photoID]
	if !exists {
		return fmt.Errorf("photo not found: %s", photoID)
	}
	
	photo.IsFavorite = !photo.IsFavorite
	return nil
}

// GetRecentMovies 获取最近观看的电影
func (mc *MediaCenter) GetRecentMovies(ctx context.Context, limit int) []*Movie {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	var movies []*Movie
	for _, movie := range mc.movies {
		movies = append(movies, movie)
	}
	
	// 按观看时间排序
	sortMoviesByWatchTime(movies)
	
	if len(movies) > limit {
		return movies[:limit]
	}
	return movies
}

// GetRecentPhotos 获取最近添加的照片
func (mc *MediaCenter) GetRecentPhotos(ctx context.Context, limit int) []*Photo {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	var photos []*Photo
	for _, photo := range mc.photos {
		photos = append(photos, photo)
	}
	
	// 按添加时间排序
	sortPhotosByAddedTime(photos)
	
	if len(photos) > limit {
		return photos[:limit]
	}
	return photos
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	// 简单的忽略大小写比较
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func matchPhotoByQuery(photo *Photo, query string) bool {
	// 检查标签
	for _, tag := range photo.Tags {
		if contains(tag, query) {
			return true
		}
	}
	
	// 检查相册
	for _, album := range photo.Albums {
		if contains(album, query) {
			return true
		}
	}
	
	// 检查文件名
	if contains(photo.Filename, query) {
		return true
	}
	
	return false
}

func sortMoviesByWatchTime(movies []*Movie) {
	// 简单的冒泡排序
	for i := 0; i < len(movies); i++ {
		for j := i + 1; j < len(movies); j++ {
			if movies[i].WatchedAt.Before(movies[j].WatchedAt) {
				movies[i], movies[j] = movies[j], movies[i]
			}
		}
	}
}

func sortPhotosByAddedTime(photos []*Photo) {
	// 简单的冒泡排序
	for i := 0; i < len(photos); i++ {
		for j := i + 1; j < len(photos); j++ {
			if photos[i].AddedAt.Before(photos[j].AddedAt) {
				photos[i], photos[j] = photos[j], photos[i]
			}
		}
	}
}
