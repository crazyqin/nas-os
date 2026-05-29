// Package audiostation 提供音乐中心管理功能
package audiostation

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// CreatePlaylist 创建播放列表.
func (m *Manager) CreatePlaylist(input PlaylistInput) (*Playlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成ID
	id := generatePlaylistID()

	// 计算总时长
	duration := 0
	for _, trackID := range input.TrackIDs {
		if track, exists := m.tracks[trackID]; exists {
			duration += track.Duration
		}
	}

	playlist := &Playlist{
		ID:          id,
		Name:        input.Name,
		Description: input.Description,
		TrackCount:  len(input.TrackIDs),
		Duration:    duration,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置封面为第一首歌的封面
	if len(input.TrackIDs) > 0 {
		if track, exists := m.tracks[input.TrackIDs[0]]; exists {
			playlist.CoverPath = track.CoverPath
		}
	}

	m.playlists[id] = playlist
	_ = m.saveConfig()

	return playlist, nil
}

// GetPlaylist 获取播放列表详情（含曲目列表）.
func (m *Manager) GetPlaylist(id string) (*Playlist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	playlist, exists := m.playlists[id]
	if !exists {
		return nil, ErrPlaylistNotFound
	}

	return playlist, nil
}

// ListPlaylists 列出所有播放列表.
func (m *Manager) ListPlaylists() []*Playlist {
	m.mu.RLock()
	defer m.mu.RUnlock()

	playlists := make([]*Playlist, 0, len(m.playlists))
	for _, p := range m.playlists {
		playlists = append(playlists, p)
	}
	return playlists
}

// UpdatePlaylist 更新播放列表.
func (m *Manager) UpdatePlaylist(id string, input PlaylistInput) (*Playlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	playlist, exists := m.playlists[id]
	if !exists {
		return nil, ErrPlaylistNotFound
	}

	playlist.Name = input.Name
	playlist.Description = input.Description
	playlist.TrackCount = len(input.TrackIDs)
	playlist.UpdatedAt = time.Now()

	// 重新计算总时长
	duration := 0
	for _, trackID := range input.TrackIDs {
		if track, exists := m.tracks[trackID]; exists {
			duration += track.Duration
		}
	}
	playlist.Duration = duration

	_ = m.saveConfig()
	return playlist, nil
}

// DeletePlaylist 删除播放列表.
func (m *Manager) DeletePlaylist(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.playlists[id]; !exists {
		return ErrPlaylistNotFound
	}

	delete(m.playlists, id)
	_ = m.saveConfig()
	return nil
}

// generatePlaylistID 生成播放列表ID.
func generatePlaylistID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("pl_%s_%d", hex.EncodeToString(b), time.Now().UnixNano())
}

// SharedPlaylist 分享的播放列表.
type SharedPlaylist struct {
	PlaylistID string    `json:"playlist_id"` // 播放列表 ID
	ShareToken string    `json:"share_token"` // 分享令牌
	ExpiresAt  time.Time `json:"expires_at"`  // 过期时间
	CreatedAt  time.Time `json:"created_at"`  // 创建时间
}

// SharePlaylist 分享播放列表.
func (m *Manager) SharePlaylist(playlistID string, expireHours int) (*SharedPlaylist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.playlists[playlistID]; !exists {
		return nil, ErrPlaylistNotFound
	}

	if expireHours <= 0 {
		expireHours = 72 // 默认 72 小时
	}

	b := make([]byte, 16)
	_, _ = rand.Read(b)

	shared := &SharedPlaylist{
		PlaylistID: playlistID,
		ShareToken: hex.EncodeToString(b),
		ExpiresAt:  time.Now().Add(time.Duration(expireHours) * time.Hour),
		CreatedAt:  time.Now(),
	}

	// 持久化分享信息
	_ = m.saveConfig()
	return shared, nil
}
