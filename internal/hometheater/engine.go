// Package hometheater 提供家庭影院系统核心引擎功能
package hometheater

import (
	"fmt"
	"sync"
	"time"
)

// Engine 家庭影院引擎.
type Engine struct {
	mu              sync.RWMutex
	libraries       map[string]*MediaLibrary
	movies          map[string]*Movie
	shows           map[string]*TVShow
	episodes        map[string]*Episode
	playlists       map[string]*Playlist
	profiles        map[string]*TranscodeProfile
	sessions        map[string]*StreamSession
	transcodeJobs   map[string]*TranscodeJob
	userConfigs     map[string]*UserConfig
	devices         map[string]*DLNADevice
	scanRunning     bool
	running         bool
	startTime       time.Time
	stats           *MediaStats
	onPlaybackEvent func(event string, session *StreamSession)
}

// NewEngine 创建家庭影院引擎.
func NewEngine() *Engine {
	return &Engine{
		libraries:     make(map[string]*MediaLibrary),
		movies:        make(map[string]*Movie),
		shows:         make(map[string]*TVShow),
		episodes:      make(map[string]*Episode),
		playlists:     make(map[string]*Playlist),
		profiles:      make(map[string]*TranscodeProfile),
		sessions:      make(map[string]*StreamSession),
		transcodeJobs: make(map[string]*TranscodeJob),
		userConfigs:   make(map[string]*UserConfig),
		devices:       make(map[string]*DLNADevice),
		stats:         &MediaStats{},
	}
}

// Start 启动引擎.
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	e.running = true
	e.startTime = time.Now()
	return nil
}

// Stop 停止引擎.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	// 停止所有活跃会话
	for _, session := range e.sessions {
		if session.State == PlaybackPlaying || session.State == PlaybackPaused {
			session.State = PlaybackStopped
		}
	}

	e.running = false
	return nil
}

// IsRunning 返回引擎是否运行中.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// ========== 媒体库管理 ==========

// AddLibrary 添加媒体库.
func (e *Engine) AddLibrary(lib *MediaLibrary) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if lib.ID == "" {
		return fmt.Errorf("媒体库ID不能为空")
	}

	if _, exists := e.libraries[lib.ID]; exists {
		return fmt.Errorf("媒体库已存在: %s", lib.ID)
	}

	now := time.Now()
	lib.CreatedAt = now
	lib.UpdatedAt = now
	e.libraries[lib.ID] = lib
	return nil
}

// GetLibrary 获取媒体库.
func (e *Engine) GetLibrary(id string) (*MediaLibrary, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	lib, exists := e.libraries[id]
	if !exists {
		return nil, ErrMediaNotFound
	}
	return lib, nil
}

// ListLibraries 列出所有媒体库.
func (e *Engine) ListLibraries() []*MediaLibrary {
	e.mu.RLock()
	defer e.mu.RUnlock()

	libs := make([]*MediaLibrary, 0, len(e.libraries))
	for _, lib := range e.libraries {
		libs = append(libs, lib)
	}
	return libs
}

// RemoveLibrary 删除媒体库.
func (e *Engine) RemoveLibrary(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.libraries[id]; !exists {
		return ErrMediaNotFound
	}

	// 删除该库下所有媒体
	for mid, movie := range e.movies {
		if movie.LibraryID == id {
			delete(e.movies, mid)
		}
	}
	for sid, show := range e.shows {
		if show.LibraryID == id {
			delete(e.shows, sid)
		}
	}

	delete(e.libraries, id)
	return nil
}

// ========== 电影管理 ==========

// AddMovie 添加电影.
func (e *Engine) AddMovie(movie *Movie) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if movie.ID == "" {
		return fmt.Errorf("电影ID不能为空")
	}

	if _, exists := e.movies[movie.ID]; exists {
		return fmt.Errorf("电影已存在: %s", movie.ID)
	}

	now := time.Now()
	movie.CreatedAt = now
	movie.UpdatedAt = now
	e.movies[movie.ID] = movie
	e.stats.TotalMovies++
	return nil
}

// GetMovie 获取电影.
func (e *Engine) GetMovie(id string) (*Movie, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	movie, exists := e.movies[id]
	if !exists {
		return nil, ErrMovieNotFound
	}
	return movie, nil
}

// ListMovies 列出所有电影.
func (e *Engine) ListMovies() []*Movie {
	e.mu.RLock()
	defer e.mu.RUnlock()

	movies := make([]*Movie, 0, len(e.movies))
	for _, m := range e.movies {
		movies = append(movies, m)
	}
	return movies
}

// RemoveMovie 删除电影.
func (e *Engine) RemoveMovie(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.movies[id]; !exists {
		return ErrMovieNotFound
	}

	delete(e.movies, id)
	e.stats.TotalMovies--
	return nil
}

// UpdateMovie 更新电影.
func (e *Engine) UpdateMovie(movie *Movie) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.movies[movie.ID]; !exists {
		return ErrMovieNotFound
	}

	movie.UpdatedAt = time.Now()
	e.movies[movie.ID] = movie
	return nil
}

// ========== 剧集管理 ==========

// AddTVShow 添加电视剧.
func (e *Engine) AddTVShow(show *TVShow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if show.ID == "" {
		return fmt.Errorf("剧集ID不能为空")
	}

	if _, exists := e.shows[show.ID]; exists {
		return fmt.Errorf("剧集已存在: %s", show.ID)
	}

	now := time.Now()
	show.CreatedAt = now
	show.UpdatedAt = now
	e.shows[show.ID] = show
	e.stats.TotalShows++
	return nil
}

// GetTVShow 获取电视剧.
func (e *Engine) GetTVShow(id string) (*TVShow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	show, exists := e.shows[id]
	if !exists {
		return nil, ErrTVShowNotFound
	}
	return show, nil
}

// ListTVShows 列出所有电视剧.
func (e *Engine) ListTVShows() []*TVShow {
	e.mu.RLock()
	defer e.mu.RUnlock()

	shows := make([]*TVShow, 0, len(e.shows))
	for _, s := range e.shows {
		shows = append(shows, s)
	}
	return shows
}

// AddEpisode 添加剧集.
func (e *Engine) AddEpisode(episode *Episode) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if episode.ID == "" {
		return fmt.Errorf("剧集ID不能为空")
	}

	if _, exists := e.episodes[episode.ID]; exists {
		return fmt.Errorf("剧集已存在: %s", episode.ID)
	}

	now := time.Now()
	episode.CreatedAt = now
	episode.UpdatedAt = now
	e.episodes[episode.ID] = episode
	e.stats.TotalEpisodes++

	// 更新剧集计数
	if show, exists := e.shows[episode.ShowID]; exists {
		show.EpisodeCount++
	}

	return nil
}

// GetEpisode 获取剧集.
func (e *Engine) GetEpisode(id string) (*Episode, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	episode, exists := e.episodes[id]
	if !exists {
		return nil, ErrMediaNotFound
	}
	return episode, nil
}

// ========== 播放列表管理 ==========

// CreatePlaylist 创建播放列表.
func (e *Engine) CreatePlaylist(playlist *Playlist) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if playlist.ID == "" {
		return fmt.Errorf("播放列表ID不能为空")
	}

	if _, exists := e.playlists[playlist.ID]; exists {
		return ErrPlaylistExists
	}

	now := time.Now()
	playlist.CreatedAt = now
	playlist.UpdatedAt = now
	playlist.CurrentIndex = 0
	e.playlists[playlist.ID] = playlist
	return nil
}

// GetPlaylist 获取播放列表.
func (e *Engine) GetPlaylist(id string) (*Playlist, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pl, exists := e.playlists[id]
	if !exists {
		return nil, ErrPlaylistNotFound
	}
	return pl, nil
}

// ListPlaylists 列出播放列表.
func (e *Engine) ListPlaylists(userID string) []*Playlist {
	e.mu.RLock()
	defer e.mu.RUnlock()

	playlists := make([]*Playlist, 0)
	for _, pl := range e.playlists {
		if userID == "" || pl.UserID == userID {
			playlists = append(playlists, pl)
		}
	}
	return playlists
}

// DeletePlaylist 删除播放列表.
func (e *Engine) DeletePlaylist(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.playlists[id]; !exists {
		return ErrPlaylistNotFound
	}

	delete(e.playlists, id)
	return nil
}

// AddToPlaylist 添加媒体到播放列表.
func (e *Engine) AddToPlaylist(playlistID string, item *MediaItem) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	pl, exists := e.playlists[playlistID]
	if !exists {
		return ErrPlaylistNotFound
	}

	item.AddedAt = time.Now()
	pl.Items = append(pl.Items, item)
	pl.UpdatedAt = time.Now()
	return nil
}

// ========== 转码配置管理 ==========

// AddTranscodeProfile 添加转码配置.
func (e *Engine) AddTranscodeProfile(profile *TranscodeProfile) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if profile.ID == "" {
		return ErrInvalidProfile
	}

	if _, exists := e.profiles[profile.ID]; exists {
		return fmt.Errorf("转码配置已存在: %s", profile.ID)
	}

	e.profiles[profile.ID] = profile
	return nil
}

// GetTranscodeProfile 获取转码配置.
func (e *Engine) GetTranscodeProfile(id string) (*TranscodeProfile, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	profile, exists := e.profiles[id]
	if !exists {
		return nil, ErrInvalidProfile
	}
	return profile, nil
}

// ListTranscodeProfiles 列出所有转码配置.
func (e *Engine) ListTranscodeProfiles() []*TranscodeProfile {
	e.mu.RLock()
	defer e.mu.RUnlock()

	profiles := make([]*TranscodeProfile, 0, len(e.profiles))
	for _, p := range e.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// ========== 用户配置管理 ==========

// SetUserConfig 设置用户配置.
func (e *Engine) SetUserConfig(config *UserConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if config.UserID == "" {
		return fmt.Errorf("用户ID不能为空")
	}

	e.userConfigs[config.UserID] = config
	return nil
}

// GetUserConfig 获取用户配置.
func (e *Engine) GetUserConfig(userID string) (*UserConfig, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	config, exists := e.userConfigs[userID]
	if !exists {
		// 返回默认配置
		return &UserConfig{
			UserID:          userID,
			PreferredLang:   "zh",
			SubtitleEnabled: true,
			SubtitleLang:    "zh",
			SubtitleSize:    24,
			AudioLang:       "zh",
			AutoPlay:        true,
			DefaultQuality:  "auto",
			HWAccelEnabled:  true,
		}, nil
	}
	return config, nil
}

// ========== 播放历史 ==========

// UpdateWatchProgress 更新观看进度.
func (e *Engine) UpdateWatchProgress(mediaID string, progress *WatchProgress) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 尝试更新电影
	if movie, exists := e.movies[mediaID]; exists {
		movie.WatchProgress = progress
		movie.LastPlayed = &progress.LastUpdated
		return nil
	}

	// 尝试更新剧集
	if episode, exists := e.episodes[mediaID]; exists {
		episode.WatchProgress = progress
		episode.LastPlayed = &progress.LastUpdated
		return nil
	}

	return ErrMediaNotFound
}

// GetContinueWatching 获取继续观看列表.
func (e *Engine) GetContinueWatching(userID string, limit int) []*MediaItem {
	e.mu.RLock()
	defer e.mu.RUnlock()

	items := make([]*MediaItem, 0)

	for _, movie := range e.movies {
		if movie.WatchProgress != nil && !movie.WatchProgress.Completed && movie.WatchProgress.Percentage > 0 {
			items = append(items, &MediaItem{
				ID:       movie.ID,
				Type:     MediaTypeMovie,
				Title:    movie.Title,
				Duration: movie.Runtime * 60,
				FilePath: movie.FilePath,
			})
		}
	}

	for _, episode := range e.episodes {
		if episode.WatchProgress != nil && !episode.WatchProgress.Completed && episode.WatchProgress.Percentage > 0 {
			items = append(items, &MediaItem{
				ID:       episode.ID,
				Type:     MediaTypeEpisode,
				Title:    episode.Title,
				Duration: episode.Runtime * 60,
				FilePath: episode.FilePath,
			})
		}
	}

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	return items
}

// ========== 统计 ==========

// GetStats 获取媒体统计.
func (e *Engine) GetStats() *MediaStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := *e.stats
	stats.ActiveSessions = len(e.sessions)

	// 计算总大小和时长
	var totalSize int64
	var totalDuration int64
	var totalPlays int64

	for _, movie := range e.movies {
		totalSize += movie.FileSize
		totalDuration += int64(movie.Runtime) * 60
		totalPlays += movie.PlayCount
	}
	for _, episode := range e.episodes {
		totalSize += episode.FileSize
		totalDuration += int64(episode.Runtime) * 60
		totalPlays += episode.PlayCount
	}

	stats.TotalSize = totalSize
	stats.TotalDuration = totalDuration
	stats.TotalPlays = totalPlays

	return &stats
}

// SetOnPlaybackEvent 设置播放事件回调.
func (e *Engine) SetOnPlaybackEvent(fn func(event string, session *StreamSession)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onPlaybackEvent = fn
}

// fireEvent 触发播放事件.
func (e *Engine) fireEvent(event string, session *StreamSession) {
	if e.onPlaybackEvent != nil {
		e.onPlaybackEvent(event, session)
	}
}
