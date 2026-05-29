// Package audiostation 提供音乐中心管理功能
package audiostation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HLSSegment HLS 切片信息.
type HLSSegment struct {
	Index    int     `json:"index"`    // 切片序号
	Duration float64 `json:"duration"` // 切片时长（秒）
	FilePath string  `json:"-"`        // 切片文件路径
	URL      string  `json:"url"`      // 切片访问 URL
}

// HLSPlaylist HLS 播放列表.
type HLSPlaylist struct {
	TrackID      string       `json:"track_id"`      // 曲目 ID
	MasterURL    string       `json:"master_url"`    // Master playlist URL
	MediaURL     string       `json:"media_url"`     // Media playlist URL
	Segments     []*HLSSegment `json:"segments"`     // 切片列表
	Duration     float64      `json:"duration"`      // 总时长（秒）
	TargetDur    float64      `json:"target_dur"`    // 目标切片时长
	CreatedAt    time.Time    `json:"created_at"`    // 创建时间
}

// HLSStreamSession HLS 流媒体会话.
type HLSStreamSession struct {
	ID           string        `json:"id"`            // 会话 ID
	TrackID      string        `json:"track_id"`      // 曲目 ID
	Playlist     *HLSPlaylist  `json:"playlist"`      // HLS 播放列表
	CreatedAt    time.Time     `json:"created_at"`    // 创建时间
	ExpiresAt    time.Time     `json:"expires_at"`    // 过期时间
}

// HLSManager HLS 流媒体管理器.
type HLSManager struct {
	mu           sync.RWMutex
	manager      *Manager
	sessions     map[string]*HLSStreamSession // sessionID -> session
	outputDir    string                       // HLS 输出目录
	targetDur    float64                      // 目标切片时长（秒）
}

// NewHLSManager 创建 HLS 管理器.
func NewHLSManager(mgr *Manager, outputDir string, targetDuration float64) *HLSManager {
	if targetDuration <= 0 {
		targetDuration = 10.0 // 默认 10 秒
	}
	return &HLSManager{
		manager:   mgr,
		sessions:  make(map[string]*HLSStreamSession),
		outputDir: outputDir,
		targetDur: targetDuration,
	}
}

// CreateStream 创建 HLS 流媒体会话.
func (h *HLSManager) CreateStream(trackID, baseURL string) (*HLSStreamSession, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 获取曲目信息
	track, err := h.manager.GetTrack(trackID)
	if err != nil {
		return nil, err
	}

	// 检查文件是否存在
	if _, err := os.Stat(track.FilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("音乐文件不存在: %s", track.FilePath)
	}

	// 生成会话 ID
	sessionID := generateHLSSessionID(trackID)

	// 创建 HLS 输出目录
	sessionDir := filepath.Join(h.outputDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0750); err != nil {
		return nil, fmt.Errorf("创建 HLS 目录失败: %w", err)
	}

	// 生成切片列表（基于时长估算）
	playlist := h.generatePlaylist(track, sessionID, baseURL)

	session := &HLSStreamSession{
		ID:        sessionID,
		TrackID:   trackID,
		Playlist:  playlist,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour), // 1 小时过期
	}

	h.sessions[sessionID] = session
	return session, nil
}

// GetStream 获取 HLS 流媒体会话.
func (h *HLSManager) GetStream(sessionID string) (*HLSStreamSession, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	session, exists := h.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("HLS 会话不存在: %s", sessionID)
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("HLS 会话已过期: %s", sessionID)
	}

	return session, nil
}

// GenerateMasterPlaylist 生成 Master Playlist (M3U8).
func (h *HLSManager) GenerateMasterPlaylist(sessionID string) (string, error) {
	session, err := h.GetStream(sessionID)
	if err != nil {
		return "", err
	}

	track, err := h.manager.GetTrack(session.TrackID)
	if err != nil {
		return "", err
	}

	// 根据音频码率选择带宽
	bandwidth := track.Bitrate * 1000 // bps
	if bandwidth <= 0 {
		bandwidth = 128000 // 默认 128kbps
	}

	master := fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-STREAM-INF:BANDWIDTH=%d,CODECS=\"mp4a.40.2\"\n%s\n",
		bandwidth, session.Playlist.MediaURL)

	return master, nil
}

// GenerateMediaPlaylist 生成 Media Playlist (M3U8).
func (h *HLSManager) GenerateMediaPlaylist(sessionID string) (string, error) {
	session, err := h.GetStream(sessionID)
	if err != nil {
		return "", err
	}

	playlist := session.Playlist
	maxDuration := 0.0
	for _, seg := range playlist.Segments {
		if seg.Duration > maxDuration {
			maxDuration = seg.Duration
		}
	}

	m3u8 := fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n",
		int(maxDuration)+1)

	for _, seg := range playlist.Segments {
		m3u8 += fmt.Sprintf("#EXTINF:%.3f,\n%s\n", seg.Duration, seg.URL)
	}

	m3u8 += "#EXT-X-ENDLIST\n"
	return m3u8, nil
}

// CleanupExpiredSessions 清理过期会话.
func (h *HLSManager) CleanupExpiredSessions() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	count := 0
	now := time.Now()
	for id, session := range h.sessions {
		if now.After(session.ExpiresAt) {
			// 清理切片文件
			sessionDir := filepath.Join(h.outputDir, id)
			os.RemoveAll(sessionDir)
			delete(h.sessions, id)
			count++
		}
	}
	return count
}

// GetActiveSessions 获取活跃会话数.
func (h *HLSManager) GetActiveSessions() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

// generatePlaylist 生成 HLS 播放列表.
func (h *HLSManager) generatePlaylist(track *Track, sessionID, baseURL string) *HLSPlaylist {
	duration := float64(track.Duration)
	if duration <= 0 {
		duration = 180.0 // 默认 3 分钟
	}

	// 计算切片数量
	segmentCount := int(duration/h.targetDur) + 1
	segments := make([]*HLSSegment, 0, segmentCount)

	remaining := duration
	for i := 0; i < segmentCount; i++ {
		segDur := h.targetDur
		if remaining < segDur {
			segDur = remaining
		}
		if segDur <= 0 {
			break
		}

		segments = append(segments, &HLSSegment{
			Index:    i,
			Duration: segDur,
			FilePath: filepath.Join(h.outputDir, sessionID, fmt.Sprintf("segment_%03d.ts", i)),
			URL:      fmt.Sprintf("%s/hls/%s/segment_%03d.ts", baseURL, sessionID, i),
		})
		remaining -= segDur
	}

	return &HLSPlaylist{
		TrackID:   track.ID,
		MasterURL: fmt.Sprintf("%s/hls/%s/master.m3u8", baseURL, sessionID),
		MediaURL:  fmt.Sprintf("%s/hls/%s/media.m3u8", baseURL, sessionID),
		Segments:  segments,
		Duration:  duration,
		TargetDur: h.targetDur,
		CreatedAt: time.Now(),
	}
}

// generateHLSSessionID 生成 HLS 会话 ID.
func generateHLSSessionID(trackID string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s_%d", trackID, time.Now().UnixNano())))
	return hex.EncodeToString(hash[:8])
}
