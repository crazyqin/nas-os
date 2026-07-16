// Package audiostation 提供音乐中心管理功能
package audiostation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 音乐中心管理器.
type Manager struct {
	mu           sync.RWMutex
	tracks       map[string]*Track      // trackID -> Track
	albums       map[string]*Album      // albumID -> Album
	artists      map[string]*Artist     // artistID -> Artist
	genres       map[string]*Genre      // genreID -> Genre
	playlists    map[string]*Playlist   // playlistID -> Playlist
	favorites    map[string]bool        // trackID -> isFavorite
	recentPlayed []string               // 最近播放的trackID列表（按时间倒序）
	playHistory  []*PlaybackStats       // 播放历史
	dlnaDevices  map[string]*DLNADevice // deviceID -> DLNADevice
	scanStatus   *ScanStatus
	configPath   string   // 配置持久化路径
	libraryPaths []string // 音乐库扫描路径
}

// NewManager 创建音乐中心管理器.
func NewManager(configPath string, libraryPaths []string) (*Manager, error) {
	m := &Manager{
		tracks:       make(map[string]*Track),
		albums:       make(map[string]*Album),
		artists:      make(map[string]*Artist),
		genres:       make(map[string]*Genre),
		playlists:    make(map[string]*Playlist),
		favorites:    make(map[string]bool),
		recentPlayed: make([]string, 0),
		playHistory:  make([]*PlaybackStats, 0),
		dlnaDevices:  make(map[string]*DLNADevice),
		scanStatus:   &ScanStatus{},
		configPath:   configPath,
		libraryPaths: libraryPaths,
	}

	// 加载持久化配置
	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			return nil, fmt.Errorf("加载音乐库配置失败: %w", err)
		}
	}

	return m, nil
}

// ========== 音乐库管理 ==========

// GetTrack 获取音乐文件.
func (m *Manager) GetTrack(id string) (*Track, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	track, exists := m.tracks[id]
	if !exists {
		return nil, ErrTrackNotFound
	}
	return track, nil
}

// ListTracks 列出音乐文件（支持搜索、排序、过滤）.
func (m *Manager) ListTracks(query LibraryQuery) ([]*Track, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 收集所有曲目
	tracks := make([]*Track, 0, len(m.tracks))
	for _, t := range m.tracks {
		tracks = append(tracks, t)
	}

	// 过滤
	tracks = m.filterTracks(tracks, query)

	// 总数（过滤后、分页前）
	total := len(tracks)

	// 排序
	m.sortTracks(tracks, query.Sort, query.Order)

	// 分页
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PerPage <= 0 {
		query.PerPage = 50
	}
	start := (query.Page - 1) * query.PerPage
	if start >= len(tracks) {
		return []*Track{}, total
	}
	end := start + query.PerPage
	if end > len(tracks) {
		end = len(tracks)
	}

	return tracks[start:end], total
}

// filterTracks 过滤曲目列表.
func (m *Manager) filterTracks(tracks []*Track, query LibraryQuery) []*Track {
	if query.Search == "" && query.Artist == "" && query.Album == "" &&
		query.Genre == "" && query.Year == 0 {
		return tracks
	}

	filtered := make([]*Track, 0)
	searchLower := strings.ToLower(query.Search)

	for _, t := range tracks {
		// 搜索关键词匹配
		if query.Search != "" {
			if !strings.Contains(strings.ToLower(t.Title), searchLower) &&
				!strings.Contains(strings.ToLower(t.Artist), searchLower) &&
				!strings.Contains(strings.ToLower(t.Album), searchLower) {
				continue
			}
		}
		// 艺术家过滤
		if query.Artist != "" && !strings.EqualFold(t.Artist, query.Artist) {
			continue
		}
		// 专辑过滤
		if query.Album != "" && !strings.EqualFold(t.Album, query.Album) {
			continue
		}
		// 流派过滤
		if query.Genre != "" && !strings.EqualFold(t.Genre, query.Genre) {
			continue
		}
		// 年份过滤
		if query.Year > 0 && t.Year != query.Year {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}

// sortTracks 排序曲目.
func (m *Manager) sortTracks(tracks []*Track, sortBy, order string) {
	if sortBy == "" {
		sortBy = "title"
	}
	desc := strings.ToLower(order) == "desc"

	sort.Slice(tracks, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "title":
			less = strings.ToLower(tracks[i].Title) < strings.ToLower(tracks[j].Title)
		case "artist":
			less = strings.ToLower(tracks[i].Artist) < strings.ToLower(tracks[j].Artist)
		case "album":
			less = strings.ToLower(tracks[i].Album) < strings.ToLower(tracks[j].Album)
		case "year":
			less = tracks[i].Year < tracks[j].Year
		case "duration":
			less = tracks[i].Duration < tracks[j].Duration
		case "play_count":
			less = tracks[i].PlayCount < tracks[j].PlayCount
		default:
			less = strings.ToLower(tracks[i].Title) < strings.ToLower(tracks[j].Title)
		}
		if desc {
			return !less
		}
		return less
	})
}

// ========== 专辑管理 ==========

// GetAlbum 获取专辑详情（含曲目列表）.
func (m *Manager) GetAlbum(id string) (*Album, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	album, exists := m.albums[id]
	if !exists {
		return nil, ErrAlbumNotFound
	}

	// 填充曲目列表
	albumWithTracks := *album
	albumWithTracks.Tracks = m.getAlbumTracks(id)
	return &albumWithTracks, nil
}

// ListAlbums 列出专辑.
func (m *Manager) ListAlbums(artist, genre string) []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()

	albums := make([]*Album, 0, len(m.albums))
	for _, a := range m.albums {
		if artist != "" && !strings.EqualFold(a.Artist, artist) {
			continue
		}
		if genre != "" && !strings.EqualFold(a.Genre, genre) {
			continue
		}
		albums = append(albums, a)
	}

	sort.Slice(albums, func(i, j int) bool {
		return strings.ToLower(albums[i].Title) < strings.ToLower(albums[j].Title)
	})
	return albums
}

// getAlbumTracks 获取专辑内的曲目（需调用者持有读锁）.
func (m *Manager) getAlbumTracks(albumID string) []*Track {
	tracks := make([]*Track, 0)
	for _, t := range m.tracks {
		if generateAlbumID(t.Artist, t.Album) == albumID {
			tracks = append(tracks, t)
		}
	}
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].TrackNum < tracks[j].TrackNum
	})
	return tracks
}

// ========== 艺术家管理 ==========

// GetArtist 获取艺术家详情（含专辑列表）.
func (m *Manager) GetArtist(id string) (*Artist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	artist, exists := m.artists[id]
	if !exists {
		return nil, ErrArtistNotFound
	}

	// 填充专辑列表
	artistWithAlbums := *artist
	albums := make([]*Album, 0)
	for _, a := range m.albums {
		if strings.EqualFold(a.Artist, artist.Name) {
			albums = append(albums, a)
		}
	}
	sort.Slice(albums, func(i, j int) bool {
		return albums[i].Year > albums[j].Year
	})
	artistWithAlbums.Albums = albums
	return &artistWithAlbums, nil
}

// ListArtists 列出艺术家.
func (m *Manager) ListArtists() []*Artist {
	m.mu.RLock()
	defer m.mu.RUnlock()

	artists := make([]*Artist, 0, len(m.artists))
	for _, a := range m.artists {
		artists = append(artists, a)
	}
	sort.Slice(artists, func(i, j int) bool {
		return strings.ToLower(artists[i].Name) < strings.ToLower(artists[j].Name)
	})
	return artists
}

// ========== 流派管理 ==========

// ListGenres 列出流派.
func (m *Manager) ListGenres() []*Genre {
	m.mu.RLock()
	defer m.mu.RUnlock()

	genres := make([]*Genre, 0, len(m.genres))
	for _, g := range m.genres {
		genres = append(genres, g)
	}
	sort.Slice(genres, func(i, j int) bool {
		return strings.ToLower(genres[i].Name) < strings.ToLower(genres[j].Name)
	})
	return genres
}

// ========== 收藏管理 ==========

// ToggleFavorite 切换收藏状态.
func (m *Manager) ToggleFavorite(trackID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	track, exists := m.tracks[trackID]
	if !exists {
		return false, ErrTrackNotFound
	}

	track.IsFavorite = !track.IsFavorite
	m.favorites[trackID] = track.IsFavorite

	_ = m.saveConfig()
	return track.IsFavorite, nil
}

// ListFavorites 列出收藏曲目.
func (m *Manager) ListFavorites() []*Track {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Track, 0)
	for _, t := range m.tracks {
		if t.IsFavorite {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Title) < strings.ToLower(result[j].Title)
	})
	return result
}

// ========== 最近播放 ==========

// GetRecentPlayed 获取最近播放列表.
func (m *Manager) GetRecentPlayed(limit int) []*Track {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}
	if limit > len(m.recentPlayed) {
		limit = len(m.recentPlayed)
	}

	result := make([]*Track, 0, limit)
	for _, id := range m.recentPlayed[:limit] {
		if t, exists := m.tracks[id]; exists {
			result = append(result, t)
		}
	}
	return result
}

// RecordPlay 记录播放.
func (m *Manager) RecordPlay(trackID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	track, exists := m.tracks[trackID]
	if !exists {
		return ErrTrackNotFound
	}

	// 更新播放统计
	now := time.Now()
	track.PlayCount++
	track.LastPlayed = &now
	track.UpdatedAt = now

	// 更新最近播放列表（去重并插入到最前面）
	newRecent := make([]string, 0, len(m.recentPlayed)+1)
	newRecent = append(newRecent, trackID)
	for _, id := range m.recentPlayed {
		if id != trackID {
			newRecent = append(newRecent, id)
		}
	}
	m.recentPlayed = newRecent

	// 限制最近播放列表长度
	if len(m.recentPlayed) > 100 {
		m.recentPlayed = m.recentPlayed[:100]
	}

	_ = m.saveConfig()
	return nil
}

// ========== 音乐库统计 ==========

// GetStats 获取音乐库统计.
func (m *Manager) GetStats() *LibraryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &LibraryStats{
		TotalTracks:  len(m.tracks),
		TotalAlbums:  len(m.albums),
		TotalArtists: len(m.artists),
		TotalGenres:  len(m.genres),
	}

	for _, t := range m.tracks {
		stats.TotalSize += t.FileSize
		stats.TotalDuration += t.Duration
		if t.IsFavorite {
			stats.FavoriteCount++
		}
	}

	return stats
}

// GetScanStatus 获取扫描状态.
func (m *Manager) GetScanStatus() *ScanStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scanStatus
}

// ========== DLNA 设备管理 ==========

// ListDLNADevices 列出 DLNA 设备.
func (m *Manager) ListDLNADevices() []*DLNADevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*DLNADevice, 0, len(m.dlnaDevices))
	for _, d := range m.dlnaDevices {
		devices = append(devices, d)
	}
	return devices
}

// CastToDLNA 推送到 DLNA 设备.
func (m *Manager) CastToDLNA(deviceID, trackID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.dlnaDevices[deviceID]
	if !exists {
		return ErrDLNADeviceNotFound
	}

	track, exists := m.tracks[trackID]
	if !exists {
		return ErrTrackNotFound
	}

	// 标记设备正在播放
	device.IsPlaying = true
	device.TrackID = trackID

	// 实际 DLNA 推送逻辑（依赖外部实现）
	_ = track

	return nil
}

// RegisterDLNADevice 注册 DLNA 设备.
func (m *Manager) RegisterDLNADevice(device *DLNADevice) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dlnaDevices[device.ID] = device
}

// ========== 内部索引更新 ==========

// addTrackToIndex 将曲目添加到索引（需调用者持有写锁）.
func (m *Manager) addTrackToIndex(track *Track) {
	m.tracks[track.ID] = track

	// 更新专辑索引
	albumID := generateAlbumID(track.Artist, track.Album)
	if album, exists := m.albums[albumID]; exists {
		album.TrackCount++
		album.Duration += track.Duration
	} else {
		m.albums[albumID] = &Album{
			ID:         albumID,
			Title:      track.Album,
			Artist:     track.AlbumArtist,
			Genre:      track.Genre,
			Year:       track.Year,
			TrackCount: 1,
			Duration:   track.Duration,
			CoverPath:  track.CoverPath,
			CreatedAt:  time.Now(),
		}
	}

	// 更新艺术家索引
	artistID := generateID("artist", track.Artist)
	if artist, exists := m.artists[artistID]; exists {
		artist.TrackCount++
	} else {
		m.artists[artistID] = &Artist{
			ID:         artistID,
			Name:       track.Artist,
			TrackCount: 1,
		}
	}

	// 更新流派索引
	if track.Genre != "" {
		genreID := generateID("genre", track.Genre)
		if genre, exists := m.genres[genreID]; exists {
			genre.TrackCount++
		} else {
			m.genres[genreID] = &Genre{
				ID:         genreID,
				Name:       track.Genre,
				TrackCount: 1,
			}
		}
	}

	// 恢复收藏状态
	if m.favorites[track.ID] {
		track.IsFavorite = true
	}
}

// rebuildIndex 重建所有索引.
func (m *Manager) rebuildIndex() {
	// 清空索引
	m.albums = make(map[string]*Album)
	m.artists = make(map[string]*Artist)
	m.genres = make(map[string]*Genre)

	for _, track := range m.tracks {
		m.addTrackToIndex(track)
	}

	// 更新艺术家的专辑数
	for _, artist := range m.artists {
		albumCount := 0
		for _, album := range m.albums {
			if strings.EqualFold(album.Artist, artist.Name) {
				albumCount++
			}
		}
		artist.AlbumCount = albumCount
	}
}

// ========== 配置持久化 ==========

// persistentConfig 持久化配置.
type persistentConfig struct {
	Tracks       []*Track         `json:"tracks"`
	Playlists    []*Playlist      `json:"playlists"`
	Favorites    map[string]bool  `json:"favorites"`
	RecentPlayed []string         `json:"recent_played"`
	PlayHistory  []*PlaybackStats `json:"play_history"`
	DLNADevices  []*DLNADevice    `json:"dlna_devices"`
}

func (m *Manager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 加载曲目
	for _, t := range pc.Tracks {
		m.tracks[t.ID] = t
	}

	// 加载播放列表
	for _, p := range pc.Playlists {
		m.playlists[p.ID] = p
	}

	// 加载收藏
	if pc.Favorites != nil {
		m.favorites = pc.Favorites
	}

	// 加载最近播放
	if pc.RecentPlayed != nil {
		m.recentPlayed = pc.RecentPlayed
	}

	// 加载播放历史
	if pc.PlayHistory != nil {
		m.playHistory = pc.PlayHistory
	}

	// 加载 DLNA 设备
	for _, d := range pc.DLNADevices {
		m.dlnaDevices[d.ID] = d
	}

	// 重建索引
	m.rebuildIndex()

	return nil
}

func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	// 收集曲目
	tracks := make([]*Track, 0, len(m.tracks))
	for _, t := range m.tracks {
		tracks = append(tracks, t)
	}

	// 收集播放列表
	playlists := make([]*Playlist, 0, len(m.playlists))
	for _, p := range m.playlists {
		playlists = append(playlists, p)
	}

	// 收集 DLNA 设备
	devices := make([]*DLNADevice, 0, len(m.dlnaDevices))
	for _, d := range m.dlnaDevices {
		devices = append(devices, d)
	}

	pc := persistentConfig{
		Tracks:       tracks,
		Playlists:    playlists,
		Favorites:    m.favorites,
		RecentPlayed: m.recentPlayed,
		PlayHistory:  m.playHistory,
		DLNADevices:  devices,
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// generateAlbumID 生成专辑ID.
func generateAlbumID(artist, album string) string {
	return generateID("album", artist+"||"+album)
}

// generateID 生成确定性ID.
func generateID(prefix, name string) string {
	return fmt.Sprintf("%s_%x", prefix, simpleHash(strings.ToLower(strings.TrimSpace(name))))
}

// simpleHash 简单哈希函数.
func simpleHash(s string) uint64 {
	var h uint64
	for _, c := range s {
		h = h*31 + uint64(c)
	}
	return h
}
