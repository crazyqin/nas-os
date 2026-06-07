// Package musicserver 提供音乐流媒体服务核心业务逻辑
package musicserver

import (
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 音乐服务管理器.
type Manager struct {
	songs         map[string]*Song
	albums        map[string]*Album
	artists       map[string]*Artist
	playlists     map[string]*Playlist
	playlistSongs map[string][]*PlaylistSong // playlistID -> songs
	playQueues    map[string]*PlayQueue      // owner -> queue
	coverArts     map[string]*CoverArt
	lyrics        map[string]*Lyrics             // songID -> lyrics
	playHistory   map[string][]*PlayHistoryEntry // owner -> history
	favorites     map[string][]string            // owner -> songIDs
	mu            sync.RWMutex
}

// PlayHistoryEntry 播放历史记录.
type PlayHistoryEntry struct {
	SongID   string    `json:"song_id"`
	PlayedAt time.Time `json:"played_at"`
	Owner    string    `json:"owner"`
}

// NewManager 创建音乐服务管理器.
func NewManager() *Manager {
	return &Manager{
		songs:         make(map[string]*Song),
		albums:        make(map[string]*Album),
		artists:       make(map[string]*Artist),
		playlists:     make(map[string]*Playlist),
		playlistSongs: make(map[string][]*PlaylistSong),
		playQueues:    make(map[string]*PlayQueue),
		coverArts:     make(map[string]*CoverArt),
		lyrics:        make(map[string]*Lyrics),
		playHistory:   make(map[string][]*PlayHistoryEntry),
		favorites:     make(map[string][]string),
	}
}

// ========== 歌曲管理 ==========

// AddSong 添加歌曲.
func (m *Manager) AddSong(song *Song) *Song {
	m.mu.Lock()
	defer m.mu.Unlock()

	if song.ID == "" {
		song.ID = uuid.New().String()
	}
	if song.CreatedAt.IsZero() {
		song.CreatedAt = time.Now()
	}
	song.UpdatedAt = time.Now()

	m.songs[song.ID] = song

	// 更新专辑
	m.updateAlbum(song)

	// 更新艺术家
	m.updateArtist(song)

	log.Printf("[musicserver] 添加歌曲: %s - %s", song.Artist, song.Title)
	return song
}

// GetSong 获取歌曲.
func (m *Manager) GetSong(id string) (*Song, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	song, ok := m.songs[id]
	if !ok {
		return nil, fmt.Errorf("song %q not found", id)
	}
	return song, nil
}

// ListSongs 列出所有歌曲.
func (m *Manager) ListSongs(owner string) []*Song {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var songs []*Song
	for _, s := range m.songs {
		if owner == "" || s.Owner == owner {
			songs = append(songs, s)
		}
	}

	sort.Slice(songs, func(i, j int) bool {
		return songs[i].Title < songs[j].Title
	})

	return songs
}

// DeleteSong 删除歌曲.
func (m *Manager) DeleteSong(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	song, ok := m.songs[id]
	if !ok {
		return fmt.Errorf("song %q not found", id)
	}

	// 从播放列表中移除
	for _, pls := range m.playlistSongs {
		for i, ps := range pls {
			if ps.SongID == id {
				m.playlistSongs = map[string][]*PlaylistSong{
					"temp": append(pls[:i], pls[i+1:]...),
				}
				break
			}
		}
	}

	// 删除封面
	if song.CoverArtID != "" {
		delete(m.coverArts, song.CoverArtID)
	}

	// 删除歌词
	delete(m.lyrics, id)

	// 从收藏中移除
	for owner, favs := range m.favorites {
		for i, favID := range favs {
			if favID == id {
				m.favorites[owner] = append(favs[:i], favs[i+1:]...)
				break
			}
		}
	}

	delete(m.songs, id)

	// 更新专辑和艺术家
	m.rebuildAlbums()
	m.rebuildArtists()

	log.Printf("[musicserver] 删除歌曲: %s", song.Title)
	return nil
}

// ========== 专辑管理 ==========

// GetAlbum 获取专辑.
func (m *Manager) GetAlbum(id string) (*Album, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	album, ok := m.albums[id]
	if !ok {
		return nil, fmt.Errorf("album %q not found", id)
	}
	return album, nil
}

// ListAlbums 列出所有专辑.
func (m *Manager) ListAlbums(owner string) []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var albums []*Album
	for _, a := range m.albums {
		if owner == "" || a.Owner == owner {
			albums = append(albums, a)
		}
	}

	sort.Slice(albums, func(i, j int) bool {
		return albums[i].Name < albums[j].Name
	})

	return albums
}

// ListAlbumsByArtist 按艺术家列出专辑.
func (m *Manager) ListAlbumsByArtist(artistName, owner string) []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var albums []*Album
	artistLower := strings.ToLower(artistName)
	for _, a := range m.albums {
		if (owner == "" || a.Owner == owner) &&
			strings.ToLower(a.Artist) == artistLower {
			albums = append(albums, a)
		}
	}

	sort.Slice(albums, func(i, j int) bool {
		return albums[i].Year < albums[j].Year
	})

	return albums
}

// ListAlbumsByGenre 按流派列出专辑.
func (m *Manager) ListAlbumsByGenre(genre, owner string) []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var albums []*Album
	genreLower := strings.ToLower(genre)
	for _, a := range m.albums {
		if (owner == "" || a.Owner == owner) &&
			strings.ToLower(a.Genre) == genreLower {
			albums = append(albums, a)
		}
	}

	sort.Slice(albums, func(i, j int) bool {
		return albums[i].Name < albums[j].Name
	})

	return albums
}

// ========== 艺术家管理 ==========

// GetArtist 获取艺术家.
func (m *Manager) GetArtist(id string) (*Artist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	artist, ok := m.artists[id]
	if !ok {
		return nil, fmt.Errorf("artist %q not found", id)
	}
	return artist, nil
}

// ListArtists 列出所有艺术家.
func (m *Manager) ListArtists(owner string) []*Artist {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var artists []*Artist
	for _, a := range m.artists {
		if owner == "" || a.Owner == owner {
			artists = append(artists, a)
		}
	}

	sort.Slice(artists, func(i, j int) bool {
		return artists[i].Name < artists[j].Name
	})

	return artists
}

// ========== 播放列表管理 ==========

// CreatePlaylist 创建播放列表.
func (m *Manager) CreatePlaylist(req CreatePlaylistRequest) *Playlist {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	playlist := &Playlist{
		ID:            uuid.New().String(),
		Name:          req.Name,
		Description:   req.Description,
		Owner:         req.Owner,
		IsPublic:      req.IsPublic,
		SongCount:     0,
		TotalDuration: 0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	m.playlists[playlist.ID] = playlist
	m.playlistSongs[playlist.ID] = []*PlaylistSong{}

	log.Printf("[musicserver] 创建播放列表: %s", playlist.Name)
	return playlist
}

// GetPlaylist 获取播放列表.
func (m *Manager) GetPlaylist(id string) (*Playlist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	playlist, ok := m.playlists[id]
	if !ok {
		return nil, fmt.Errorf("playlist %q not found", id)
	}
	return playlist, nil
}

// ListPlaylists 列出播放列表.
func (m *Manager) ListPlaylists(owner string) []*Playlist {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var playlists []*Playlist
	for _, p := range m.playlists {
		if owner == "" || p.Owner == owner || p.IsPublic {
			playlists = append(playlists, p)
		}
	}

	sort.Slice(playlists, func(i, j int) bool {
		return playlists[i].UpdatedAt.After(playlists[j].UpdatedAt)
	})

	return playlists
}

// UpdatePlaylist 更新播放列表.
func (m *Manager) UpdatePlaylist(id string, req UpdatePlaylistRequest) (*Playlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	playlist, ok := m.playlists[id]
	if !ok {
		return nil, fmt.Errorf("playlist %q not found", id)
	}

	if req.Name != nil {
		playlist.Name = *req.Name
	}
	if req.Description != nil {
		playlist.Description = *req.Description
	}
	if req.IsPublic != nil {
		playlist.IsPublic = *req.IsPublic
	}

	playlist.UpdatedAt = time.Now()

	log.Printf("[musicserver] 更新播放列表: %s", playlist.Name)
	return playlist, nil
}

// DeletePlaylist 删除播放列表.
func (m *Manager) DeletePlaylist(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.playlists[id]; !ok {
		return fmt.Errorf("playlist %q not found", id)
	}

	delete(m.playlists, id)
	delete(m.playlistSongs, id)

	log.Printf("[musicserver] 删除播放列表: %s", id)
	return nil
}

// AddSongToPlaylist 添加歌曲到播放列表.
func (m *Manager) AddSongToPlaylist(playlistID, songID, addedBy string, position *int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	playlist, ok := m.playlists[playlistID]
	if !ok {
		return fmt.Errorf("playlist %q not found", playlistID)
	}

	if _, ok := m.songs[songID]; !ok {
		return fmt.Errorf("song %q not found", songID)
	}

	songs := m.playlistSongs[playlistID]

	// 确定插入位置
	pos := len(songs)
	if position != nil && *position >= 0 && *position <= len(songs) {
		pos = *position
	}

	newEntry := &PlaylistSong{
		SongID:   songID,
		Position: pos,
		AddedAt:  time.Now(),
		AddedBy:  addedBy,
	}

	// 插入到指定位置
	if pos == len(songs) {
		songs = append(songs, newEntry)
	} else {
		songs = append(songs[:pos+1], songs[pos:]...)
		songs[pos] = newEntry
	}

	// 更新位置
	for i := range songs {
		songs[i].Position = i
	}

	m.playlistSongs[playlistID] = songs

	// 更新播放列表统计
	playlist.SongCount = len(songs)
	totalDuration := 0
	for _, ps := range songs {
		if song, ok := m.songs[ps.SongID]; ok {
			totalDuration += song.Duration
		}
	}
	playlist.TotalDuration = totalDuration
	playlist.UpdatedAt = time.Now()

	log.Printf("[musicserver] 添加歌曲到播放列表: %s -> %s", songID, playlistID)
	return nil
}

// RemoveSongFromPlaylist 从播放列表移除歌曲.
func (m *Manager) RemoveSongFromPlaylist(playlistID, songID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	playlist, ok := m.playlists[playlistID]
	if !ok {
		return fmt.Errorf("playlist %q not found", playlistID)
	}

	songs := m.playlistSongs[playlistID]
	found := false
	for i, ps := range songs {
		if ps.SongID == songID {
			songs = append(songs[:i], songs[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("song %q not found in playlist", songID)
	}

	// 更新位置
	for i := range songs {
		songs[i].Position = i
	}

	m.playlistSongs[playlistID] = songs

	// 更新播放列表统计
	playlist.SongCount = len(songs)
	totalDuration := 0
	for _, ps := range songs {
		if song, ok := m.songs[ps.SongID]; ok {
			totalDuration += song.Duration
		}
	}
	playlist.TotalDuration = totalDuration
	playlist.UpdatedAt = time.Now()

	log.Printf("[musicserver] 从播放列表移除歌曲: %s -> %s", songID, playlistID)
	return nil
}

// GetPlaylistSongs 获取播放列表歌曲.
func (m *Manager) GetPlaylistSongs(playlistID string) ([]*Song, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.playlists[playlistID]; !ok {
		return nil, fmt.Errorf("playlist %q not found", playlistID)
	}

	pls := m.playlistSongs[playlistID]
	var songs []*Song
	for _, ps := range pls {
		if song, ok := m.songs[ps.SongID]; ok {
			songs = append(songs, song)
		}
	}

	return songs, nil
}

// ========== 播放队列管理 ==========

// GetPlayQueue 获取播放队列.
func (m *Manager) GetPlayQueue(owner string) *PlayQueue {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, ok := m.playQueues[owner]
	if !ok {
		queue = &PlayQueue{
			ID:           uuid.New().String(),
			Owner:        owner,
			Songs:        []PlayQueueSong{},
			CurrentIndex: 0,
			Shuffle:      false,
			Repeat:       RepeatOff,
			UpdatedAt:    time.Now(),
		}
		m.playQueues[owner] = queue
	}

	return queue
}

// UpdatePlayQueue 更新播放队列.
func (m *Manager) UpdatePlayQueue(owner string, req UpdatePlayQueueRequest) *PlayQueue {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, ok := m.playQueues[owner]
	if !ok {
		queue = &PlayQueue{
			ID:    uuid.New().String(),
			Owner: owner,
		}
		m.playQueues[owner] = queue
	}

	if req.SongIDs != nil {
		queue.Songs = make([]PlayQueueSong, len(req.SongIDs))
		for i, songID := range req.SongIDs {
			queue.Songs[i] = PlayQueueSong{
				SongID:   songID,
				Position: i,
			}
		}
	}

	if req.CurrentIndex != nil {
		queue.CurrentIndex = *req.CurrentIndex
	}
	if req.Shuffle != nil {
		queue.Shuffle = *req.Shuffle
	}
	if req.Repeat != nil {
		queue.Repeat = *req.Repeat
	}

	queue.UpdatedAt = time.Now()

	log.Printf("[musicserver] 更新播放队列: owner=%s, songs=%d", owner, len(queue.Songs))
	return queue
}

// ========== 歌词管理 ==========

// SetLyrics 设置歌词.
func (m *Manager) SetLyrics(songID, format, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.songs[songID]; !ok {
		return fmt.Errorf("song %q not found", songID)
	}

	lyrics := &Lyrics{
		SongID:  songID,
		Format:  format,
		Content: content,
		Lines:   parseLRCLyrics(content),
	}

	m.lyrics[songID] = lyrics
	m.songs[songID].UpdatedAt = time.Now()

	log.Printf("[musicserver] 设置歌词: song=%s, format=%s", songID, format)
	return nil
}

// GetLyrics 获取歌词.
func (m *Manager) GetLyrics(songID string) (*Lyrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lyrics, ok := m.lyrics[songID]
	if !ok {
		return nil, fmt.Errorf("lyrics not found for song %q", songID)
	}
	return lyrics, nil
}

// ========== 封面管理 ==========

// SetCoverArt 设置封面.
func (m *Manager) SetCoverArt(coverArt *CoverArt) *CoverArt {
	m.mu.Lock()
	defer m.mu.Unlock()

	if coverArt.ID == "" {
		coverArt.ID = uuid.New().String()
	}
	if coverArt.CreatedAt.IsZero() {
		coverArt.CreatedAt = time.Now()
	}

	m.coverArts[coverArt.ID] = coverArt

	// 更新歌曲封面引用
	if coverArt.SongID != "" {
		if song, ok := m.songs[coverArt.SongID]; ok {
			song.CoverArtID = coverArt.ID
		}
	}

	log.Printf("[musicserver] 设置封面: %s", coverArt.ID)
	return coverArt
}

// GetCoverArt 获取封面.
func (m *Manager) GetCoverArt(id string) (*CoverArt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	coverArt, ok := m.coverArts[id]
	if !ok {
		return nil, fmt.Errorf("cover art %q not found", id)
	}
	return coverArt, nil
}

// GetCoverArtBySongID 通过歌曲ID获取封面.
func (m *Manager) GetCoverArtBySongID(songID string) (*CoverArt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ca := range m.coverArts {
		if ca.SongID == songID {
			return ca, nil
		}
	}
	return nil, fmt.Errorf("cover art not found for song %q", songID)
}

// ========== 收藏管理 ==========

// SetFavorite 设置收藏状态.
func (m *Manager) SetFavorite(owner, songID string, isFavorite bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.songs[songID]; !ok {
		return fmt.Errorf("song %q not found", songID)
	}

	m.songs[songID].IsFavorite = isFavorite

	favs := m.favorites[owner]
	if isFavorite {
		// 检查是否已收藏
		for _, favID := range favs {
			if favID == songID {
				return nil
			}
		}
		m.favorites[owner] = append(favs, songID)
	} else {
		// 移除收藏
		for i, favID := range favs {
			if favID == songID {
				m.favorites[owner] = append(favs[:i], favs[i+1:]...)
				break
			}
		}
	}

	log.Printf("[musicserver] 设置收藏: owner=%s, song=%s, favorite=%v", owner, songID, isFavorite)
	return nil
}

// GetFavorites 获取收藏列表.
func (m *Manager) GetFavorites(owner string) []*Song {
	m.mu.RLock()
	defer m.mu.RUnlock()

	favIDs := m.favorites[owner]
	var songs []*Song
	for _, id := range favIDs {
		if song, ok := m.songs[id]; ok {
			songs = append(songs, song)
		}
	}

	return songs
}

// ========== 播放记录 ==========

// RecordPlay 记录播放.
func (m *Manager) RecordPlay(owner, songID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	song, ok := m.songs[songID]
	if !ok {
		return fmt.Errorf("song %q not found", songID)
	}

	// 更新歌曲播放统计
	song.PlayCount++
	now := time.Now()
	song.LastPlayed = &now

	// 添加到播放历史
	entry := &PlayHistoryEntry{
		SongID:   songID,
		PlayedAt: now,
		Owner:    owner,
	}

	history := m.playHistory[owner]
	history = append(history, entry)

	// 限制历史记录数量
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}
	m.playHistory[owner] = history

	log.Printf("[musicserver] 记录播放: owner=%s, song=%s", owner, songID)
	return nil
}

// GetRecentlyPlayed 获取最近播放.
func (m *Manager) GetRecentlyPlayed(owner string, limit int) []*Song {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := m.playHistory[owner]
	if limit <= 0 {
		limit = 20
	}

	// 倒序获取
	start := len(history) - limit
	if start < 0 {
		start = 0
	}

	seen := make(map[string]bool)
	var songs []*Song
	for i := len(history) - 1; i >= start; i-- {
		entry := history[i]
		if !seen[entry.SongID] {
			if song, ok := m.songs[entry.SongID]; ok {
				songs = append(songs, song)
				seen[entry.SongID] = true
			}
		}
	}

	return songs
}

// ========== 搜索功能 ==========

// Search 全文搜索.
func (m *Manager) Search(query, owner, searchType string) *SearchResults {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queryLower := strings.ToLower(query)
	results := &SearchResults{}

	// 搜索歌曲
	if searchType == "" || searchType == "song" || searchType == "all" {
		for _, song := range m.songs {
			if (owner == "" || song.Owner == owner) &&
				(strings.Contains(strings.ToLower(song.Title), queryLower) ||
					strings.Contains(strings.ToLower(song.Artist), queryLower) ||
					strings.Contains(strings.ToLower(song.Album), queryLower)) {
				results.Songs = append(results.Songs, song)
			}
		}
		sort.Slice(results.Songs, func(i, j int) bool {
			return results.Songs[i].PlayCount > results.Songs[j].PlayCount
		})
	}

	// 搜索专辑
	if searchType == "" || searchType == "album" || searchType == "all" {
		for _, album := range m.albums {
			if (owner == "" || album.Owner == owner) &&
				(strings.Contains(strings.ToLower(album.Name), queryLower) ||
					strings.Contains(strings.ToLower(album.Artist), queryLower)) {
				results.Albums = append(results.Albums, album)
			}
		}
	}

	// 搜索艺术家
	if searchType == "" || searchType == "artist" || searchType == "all" {
		for _, artist := range m.artists {
			if (owner == "" || artist.Owner == owner) &&
				strings.Contains(strings.ToLower(artist.Name), queryLower) {
				results.Artists = append(results.Artists, artist)
			}
		}
	}

	return results
}

// SearchResults 搜索结果.
type SearchResults struct {
	Songs   []*Song   `json:"songs,omitempty"`
	Albums  []*Album  `json:"albums,omitempty"`
	Artists []*Artist `json:"artists,omitempty"`
}

// ========== 统计信息 ==========

// GetStats 获取播放统计.
func (m *Manager) GetStats(owner string) *PlayStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &PlayStats{
		TotalSongs:   0,
		TotalAlbums:  0,
		TotalArtists: 0,
	}

	// 统计歌曲
	for _, song := range m.songs {
		if owner == "" || song.Owner == owner {
			stats.TotalSongs++
			stats.TotalPlays += song.PlayCount
		}
	}

	// 统计专辑
	for _, album := range m.albums {
		if owner == "" || album.Owner == owner {
			stats.TotalAlbums++
		}
	}

	// 统计艺术家
	for _, artist := range m.artists {
		if owner == "" || artist.Owner == owner {
			stats.TotalArtists++
		}
	}

	// 热门歌曲
	var topSongs []*Song
	for _, song := range m.songs {
		if owner == "" || song.Owner == owner {
			topSongs = append(topSongs, song)
		}
	}
	sort.Slice(topSongs, func(i, j int) bool {
		return topSongs[i].PlayCount > topSongs[j].PlayCount
	})
	for i, song := range topSongs {
		if i >= 10 {
			break
		}
		stats.TopSongs = append(stats.TopSongs, SongStat{
			SongID:    song.ID,
			Title:     song.Title,
			Artist:    song.Artist,
			PlayCount: song.PlayCount,
		})
	}

	// 最近播放
	stats.RecentlyPlayed = m.GetRecentlyPlayed(owner, 10)

	return stats
}

// ========== 内部方法 ==========

// updateAlbum 更新专辑信息.
func (m *Manager) updateAlbum(song *Song) {
	albumKey := strings.ToLower(song.Album + "|" + song.Artist)
	var albumID string

	// 查找现有专辑
	for id, album := range m.albums {
		if strings.ToLower(album.Name+"|"+album.Artist) == albumKey {
			albumID = id
			album.SongCount++
			album.TotalDuration += song.Duration
			album.UpdatedAt = time.Now()
			break
		}
	}

	// 创建新专辑
	if albumID == "" {
		album := &Album{
			ID:            uuid.New().String(),
			Name:          song.Album,
			Artist:        song.Artist,
			AlbumArtist:   song.AlbumArtist,
			Genre:         song.Genre,
			Year:          song.Year,
			CoverArtID:    song.CoverArtID,
			SongCount:     1,
			TotalDuration: song.Duration,
			Owner:         song.Owner,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		m.albums[album.ID] = album
	}
}

// updateArtist 更新艺术家信息.
func (m *Manager) updateArtist(song *Song) {
	artistKey := strings.ToLower(song.Artist)

	// 查找现有艺术家
	for _, artist := range m.artists {
		if strings.ToLower(artist.Name) == artistKey {
			artist.SongCount++
			artist.UpdatedAt = time.Now()
			return
		}
	}

	// 创建新艺术家
	artist := &Artist{
		ID:        uuid.New().String(),
		Name:      song.Artist,
		SongCount: 1,
		Owner:     song.Owner,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.artists[artist.ID] = artist
}

// rebuildAlbums 重建专辑信息.
func (m *Manager) rebuildAlbums() {
	m.albums = make(map[string]*Album)
	for _, song := range m.songs {
		m.updateAlbum(song)
	}
}

// rebuildArtists 重建艺术家信息.
func (m *Manager) rebuildArtists() {
	m.artists = make(map[string]*Artist)
	for _, song := range m.songs {
		m.updateArtist(song)
	}
}

// parseLRCLyrics 解析 LRC 格式歌词.
func parseLRCLyrics(content string) []LyricLine {
	var lines []LyricLine
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析时间标签 [mm:ss.xx]
		if len(line) > 10 && line[0] == '[' && line[9] == ']' {
			timeStr := line[1:9]
			text := line[10:]

			var min, sec, ms int
			if _, err := fmt.Sscanf(timeStr, "%d:%d.%d", &min, &sec, &ms); err == nil {
				duration := time.Duration(min)*time.Minute +
					time.Duration(sec)*time.Second +
					time.Duration(ms)*10*time.Millisecond

				lines = append(lines, LyricLine{
					Time: duration,
					Text: text,
				})
			}
		}
	}

	// 按时间排序
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].Time < lines[j].Time
	})

	return lines
}

// generateShareToken 生成分享 token.
func generateShareToken() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
