// Package videostation 提供视频站管理器实现
package videostation

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager 视频站管理器
type Manager struct {
	mu         sync.RWMutex
	videos     map[string]*Video
	libraries  map[string]*VideoLibrary
	transcodes map[string]*TranscodeJob
	sessions   map[string]*PlaySession
	subtitles  map[string]*Subtitle
}

// NewManager 创建视频站管理器
func NewManager() *Manager {
	m := &Manager{
		videos:     make(map[string]*Video),
		libraries:  make(map[string]*VideoLibrary),
		transcodes: make(map[string]*TranscodeJob),
		sessions:   make(map[string]*PlaySession),
		subtitles:  make(map[string]*Subtitle),
	}

	// 初始化默认视频库
	m.initDefaultLibraries()
	// 初始化示例视频
	m.initSampleVideos()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d-%04x", time.Now().UnixNano(), rand.Intn(0xffff))
}

// initDefaultLibraries 初始化默认视频库
func (m *Manager) initDefaultLibraries() {
	now := time.Now()
	libs := []VideoLibrary{
		{
			ID: "lib-movies", Name: "电影", Path: "/volume1/video/movies",
			Description: "电影收藏", AutoScan: true, ScanInterval: 60, Enabled: true,
			CreatedAt: now, UpdatedAt: now, LastScanned: &now,
		},
		{
			ID: "lib-tvshows", Name: "电视剧", Path: "/volume1/video/tvshows",
			Description: "电视剧收藏", AutoScan: true, ScanInterval: 30, Enabled: true,
			CreatedAt: now, UpdatedAt: now, LastScanned: &now,
		},
		{
			ID: "lib-home", Name: "家庭视频", Path: "/volume1/video/home",
			Description: "家庭录像和旅行视频", AutoScan: false, Enabled: true,
			CreatedAt: now, UpdatedAt: now,
		},
	}

	for i := range libs {
		m.libraries[libs[i].ID] = &libs[i]
	}
}

// initSampleVideos 初始化示例视频
func (m *Manager) initSampleVideos() {
	now := time.Now()
	videos := []Video{
		{
			ID: "vid-001", Title: "示例电影 A", FilePath: "/volume1/video/movies/movie_a.mp4",
			FileName: "movie_a.mp4", FileSize: 2147483648, Duration: 7200,
			Width: 1920, Height: 1080, VideoCodec: CodecH265, AudioCodec: AACCodec,
			Bitrate: 5000000, Framerate: 23.976, Container: "mp4", LibraryID: "lib-movies",
			Category: "电影", Genre: "科幻", Year: 2024, Rating: 8.5,
			Tags: []string{"科幻", "动作"}, Status: VideoStatusReady,
			CreatedAt: now, UpdatedAt: now, IndexedAt: &now,
		},
		{
			ID: "vid-002", Title: "示例电影 B", FilePath: "/volume1/video/movies/movie_b.mkv",
			FileName: "movie_b.mkv", FileSize: 4294967296, Duration: 5400,
			Width: 3840, Height: 2160, VideoCodec: CodecH265, AudioCodec: AC3Codec,
			Bitrate: 15000000, Framerate: 24, Container: "mkv", LibraryID: "lib-movies",
			Category: "电影", Genre: "剧情", Year: 2023, Rating: 9.0,
			Tags: []string{"剧情", "获奖"}, Status: VideoStatusReady,
			CreatedAt: now, UpdatedAt: now, IndexedAt: &now,
		},
		{
			ID: "vid-003", Title: "电视剧 S01E01", FilePath: "/volume1/video/tvshows/show1/s01e01.mp4",
			FileName: "s01e01.mp4", FileSize: 536870912, Duration: 2700,
			Width: 1920, Height: 1080, VideoCodec: CodecH264, AudioCodec: AACCodec,
			Bitrate: 3000000, Framerate: 30, Container: "mp4", LibraryID: "lib-tvshows",
			Category: "电视剧", Genre: "悬疑", Year: 2024, Rating: 8.8,
			Tags: []string{"悬疑", "犯罪"}, Status: VideoStatusReady,
			CreatedAt: now, UpdatedAt: now, IndexedAt: &now,
		},
		{
			ID: "vid-004", Title: "家庭旅行 2024", FilePath: "/volume1/video/home/travel_2024.mp4",
			FileName: "travel_2024.mp4", FileSize: 1073741824, Duration: 3600,
			Width: 3840, Height: 2160, VideoCodec: CodecH264, AudioCodec: AACCodec,
			Bitrate: 20000000, Framerate: 60, Container: "mp4", LibraryID: "lib-home",
			Category: "家庭", Tags: []string{"旅行", "家庭"}, Status: VideoStatusReady,
			CreatedAt: now, UpdatedAt: now, IndexedAt: &now,
		},
	}

	for i := range videos {
		v := &videos[i]
		m.videos[v.ID] = v

		// 添加示例字幕
		sub := &Subtitle{
			ID: generateID(), VideoID: v.ID, Language: "zh", Label: "中文",
			Type: SubtitleTypeEmbedded, IsDefault: true,
		}
		m.subtitles[sub.ID] = sub
		v.Subtitles = append(v.Subtitles, *sub)

		// 更新库计数
		if lib, ok := m.libraries[v.LibraryID]; ok {
			lib.VideoCount++
			lib.TotalSize += v.FileSize
		}
	}
}

// ListVideos 列出视频
func (m *Manager) ListVideos(libraryID, category, tag string) []Video {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Video, 0)
	for _, v := range m.videos {
		if libraryID != "" && v.LibraryID != libraryID {
			continue
		}
		if category != "" && v.Category != category {
			continue
		}
		if tag != "" {
			found := false
			for _, t := range v.Tags {
				if strings.EqualFold(t, tag) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, *v)
	}
	return result
}

// GetVideo 获取视频详情
func (m *Manager) GetVideo(id string) (*Video, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, ok := m.videos[id]
	if !ok {
		return nil, fmt.Errorf("video not found: %s", id)
	}
	return v, nil
}

// UpdateVideo 更新视频元数据
func (m *Manager) UpdateVideo(id string, req *UpdateVideoRequest) (*Video, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, ok := m.videos[id]
	if !ok {
		return nil, fmt.Errorf("video not found: %s", id)
	}

	if req.Title != "" {
		v.Title = req.Title
	}
	if req.Description != "" {
		v.Description = req.Description
	}
	if req.Tags != nil {
		v.Tags = req.Tags
	}
	if req.Category != "" {
		v.Category = req.Category
	}
	if req.Genre != "" {
		v.Genre = req.Genre
	}
	if req.Year != 0 {
		v.Year = req.Year
	}
	if req.Rating != 0 {
		v.Rating = req.Rating
	}
	v.UpdatedAt = time.Now()

	return v, nil
}

// DeleteVideo 删除视频
func (m *Manager) DeleteVideo(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, ok := m.videos[id]
	if !ok {
		return fmt.Errorf("video not found: %s", id)
	}

	// 更新库计数
	if lib, ok := m.libraries[v.LibraryID]; ok {
		lib.VideoCount--
		lib.TotalSize -= v.FileSize
	}

	// 删除关联字幕
	for _, sub := range v.Subtitles {
		delete(m.subtitles, sub.ID)
	}

	delete(m.videos, id)
	return nil
}

// PlayVideo 准备播放
func (m *Manager) PlayVideo(videoID string, userID string, req *PlayRequest) (*PlayResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, ok := m.videos[videoID]
	if !ok {
		return nil, fmt.Errorf("video not found: %s", videoID)
	}

	// 查找或创建播放会话
	var session *PlaySession
	for _, s := range m.sessions {
		if s.VideoID == videoID && s.UserID == userID {
			session = s
			break
		}
	}

	if session == nil {
		session = &PlaySession{
			ID:         generateID(),
			VideoID:    videoID,
			UserID:     userID,
			DeviceType: "web",
			CreatedAt:  time.Now(),
		}
		m.sessions[session.ID] = session
	}

	session.LastUpdated = time.Now()

	quality := req.Quality
	if quality == "" {
		quality = "original"
	}

	format := req.Format
	if format == "" {
		format = "hls"
	}

	// 构建流 URL
	streamURL := fmt.Sprintf("/api/v1/videostation/videos/%s/stream?format=%s&quality=%s",
		videoID, format, quality)

	return &PlayResponse{
		VideoID:   videoID,
		StreamURL: streamURL,
		Format:    format,
		Quality:   quality,
		Duration:  v.Duration,
		Position:  session.Position,
		SessionID: session.ID,
	}, nil
}

// UpdateSession 更新播放进度
func (m *Manager) UpdateSession(sessionID string, req *SessionUpdateRequest) (*PlaySession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session.Position = req.Position
	if req.Duration > 0 {
		session.Duration = req.Duration
	}
	if session.Duration > 0 {
		session.Progress = (session.Position / session.Duration) * 100
	}
	session.LastUpdated = time.Now()

	return session, nil
}

// GetSessions 获取播放会话
func (m *Manager) GetSessions(userID string) []PlaySession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]PlaySession, 0)
	for _, s := range m.sessions {
		if userID == "" || s.UserID == userID {
			result = append(result, *s)
		}
	}
	return result
}

// CreateTranscodeJob 创建转码任务
func (m *Manager) CreateTranscodeJob(videoID string, req *TranscodeRequest) (*TranscodeJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.videos[videoID]; !ok {
		return nil, fmt.Errorf("video not found: %s", videoID)
	}

	hwAccel := req.HWAccel
	if hwAccel == "" {
		hwAccel = HWAccelNone
	}

	videoBitrate := req.VideoBitrate
	if videoBitrate == 0 {
		videoBitrate = 5000000
	}

	audioBitrate := req.AudioBitrate
	if audioBitrate == 0 {
		audioBitrate = 128000
	}

	job := &TranscodeJob{
		ID:           generateID(),
		VideoID:      videoID,
		Status:       TranscodeStatusPending,
		Format:       req.Format,
		Resolution:   req.Resolution,
		VideoBitrate: videoBitrate,
		AudioBitrate: audioBitrate,
		HWAccel:      hwAccel,
		CreatedAt:    time.Now(),
	}

	m.transcodes[job.ID] = job

	// 模拟异步转码启动
	go m.simulateTranscode(job.ID)

	return job, nil
}

// simulateTranscode 模拟转码过程
func (m *Manager) simulateTranscode(jobID string) {
	m.mu.Lock()
	job, ok := m.transcodes[jobID]
	if !ok {
		m.mu.Unlock()
		return
	}
	now := time.Now()
	job.Status = TranscodeStatusRunning
	job.StartedAt = &now
	job.OutputPath = fmt.Sprintf("/tmp/transcode/%s/%s", job.VideoID, job.Format)
	m.mu.Unlock()

	// 模拟进度更新
	for progress := 0.0; progress <= 100; progress += 10 + rand.Float64()*10 {
		time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)

		m.mu.Lock()
		if job, ok := m.transcodes[jobID]; ok {
			if job.Status == TranscodeStatusCancelled {
				m.mu.Unlock()
				return
			}
			if progress > 100 {
				progress = 100
			}
			job.Progress = progress
		}
		m.mu.Unlock()
	}

	// 完成
	m.mu.Lock()
	defer m.mu.Unlock()
	if job, ok := m.transcodes[jobID]; ok {
		if job.Status != TranscodeStatusCancelled {
			completedAt := time.Now()
			job.Status = TranscodeStatusCompleted
			job.Progress = 100
			job.CompletedAt = &completedAt
		}
	}
}

// GetTranscodeJob 获取转码任务
func (m *Manager) GetTranscodeJob(jobID string) (*TranscodeJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.transcodes[jobID]
	if !ok {
		return nil, fmt.Errorf("transcode job not found: %s", jobID)
	}
	return job, nil
}

// ListTranscodeJobs 列出转码任务
func (m *Manager) ListTranscodeJobs(videoID string) []TranscodeJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]TranscodeJob, 0)
	for _, j := range m.transcodes {
		if videoID == "" || j.VideoID == videoID {
			result = append(result, *j)
		}
	}
	return result
}

// CancelTranscodeJob 取消转码任务
func (m *Manager) CancelTranscodeJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.transcodes[jobID]
	if !ok {
		return fmt.Errorf("transcode job not found: %s", jobID)
	}

	if job.Status != TranscodeStatusPending && job.Status != TranscodeStatusRunning {
		return fmt.Errorf("cannot cancel job in status: %s", job.Status)
	}

	job.Status = TranscodeStatusCancelled
	return nil
}

// ListSubtitles 列出字幕
func (m *Manager) ListSubtitles(videoID string) []Subtitle {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Subtitle, 0)
	for _, s := range m.subtitles {
		if s.VideoID == videoID {
			result = append(result, *s)
		}
	}
	return result
}

// AddSubtitle 添加字幕
func (m *Manager) AddSubtitle(videoID string, sub *Subtitle) (*Subtitle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.videos[videoID]; !ok {
		return nil, fmt.Errorf("video not found: %s", videoID)
	}

	sub.ID = generateID()
	sub.VideoID = videoID
	m.subtitles[sub.ID] = sub

	// 添加到视频
	if v, ok := m.videos[videoID]; ok {
		v.Subtitles = append(v.Subtitles, *sub)
	}

	return sub, nil
}

// DeleteSubtitle 删除字幕
func (m *Manager) DeleteSubtitle(subID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sub, ok := m.subtitles[subID]
	if !ok {
		return fmt.Errorf("subtitle not found: %s", subID)
	}

	// 从视频移除
	if v, ok := m.videos[sub.VideoID]; ok {
		newSubs := make([]Subtitle, 0, len(v.Subtitles))
		for _, s := range v.Subtitles {
			if s.ID != subID {
				newSubs = append(newSubs, s)
			}
		}
		v.Subtitles = newSubs
	}

	delete(m.subtitles, subID)
	return nil
}

// ListLibraries 列出视频库
func (m *Manager) ListLibraries() []VideoLibrary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]VideoLibrary, 0, len(m.libraries))
	for _, l := range m.libraries {
		result = append(result, *l)
	}
	return result
}

// GetLibrary 获取视频库详情
func (m *Manager) GetLibrary(id string) (*VideoLibrary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lib, ok := m.libraries[id]
	if !ok {
		return nil, fmt.Errorf("library not found: %s", id)
	}
	return lib, nil
}

// CreateLibrary 创建视频库
func (m *Manager) CreateLibrary(req *CreateLibraryRequest) *VideoLibrary {
	m.mu.Lock()
	defer m.mu.Unlock()

	scanInterval := req.ScanInterval
	if scanInterval == 0 {
		scanInterval = 60
	}

	lib := &VideoLibrary{
		ID:           generateID(),
		Name:         req.Name,
		Path:         req.Path,
		Description:  req.Description,
		AutoScan:     req.AutoScan,
		ScanInterval: scanInterval,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.libraries[lib.ID] = lib
	return lib
}

// DeleteLibrary 删除视频库
func (m *Manager) DeleteLibrary(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.libraries[id]; !ok {
		return fmt.Errorf("library not found: %s", id)
	}

	// 删除库下所有视频
	for vid, v := range m.videos {
		if v.LibraryID == id {
			for _, sub := range v.Subtitles {
				delete(m.subtitles, sub.ID)
			}
			delete(m.videos, vid)
		}
	}

	delete(m.libraries, id)
	return nil
}

// ScanLibrary 扫描视频库
func (m *Manager) ScanLibrary(id string) (*ScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lib, ok := m.libraries[id]
	if !ok {
		return nil, fmt.Errorf("library not found: %s", id)
	}

	// 模拟扫描过程
	now := time.Now()
	lib.LastScanned = &now
	lib.UpdatedAt = now

	result := &ScanResult{
		LibraryID: id,
		NewVideos: 0,
		Updated:   0,
		Removed:   0,
		Errors:    0,
		ScannedAt: now,
	}

	// 模拟发现新视频
	if lib.Path != "" {
		result.NewVideos = rand.Intn(3)
		for i := 0; i < result.NewVideos; i++ {
			ext := []string{".mp4", ".mkv", ".avi"}[rand.Intn(3)]
			videoID := generateID()
			newVideo := &Video{
				ID:         videoID,
				Title:      fmt.Sprintf("扫描发现的视频 %d", i+1),
				FilePath:   filepath.Join(lib.Path, fmt.Sprintf("video_%d%s", i+1, ext)),
				FileName:   fmt.Sprintf("video_%d%s", i+1, ext),
				FileSize:   int64(100+rand.Intn(4900)) * 1024 * 1024,
				Duration:   float64(600 + rand.Intn(7200)),
				Width:      1920,
				Height:     1080,
				VideoCodec: CodecH264,
				AudioCodec: AACCodec,
				Bitrate:    3000000,
				Framerate:  30,
				Container:  ext[1:],
				LibraryID:  id,
				Status:     VideoStatusReady,
				CreatedAt:  now,
				UpdatedAt:  now,
				IndexedAt:  &now,
			}
			m.videos[videoID] = newVideo
			lib.VideoCount++
			lib.TotalSize += newVideo.FileSize
		}
	}

	return result, nil
}

// GetRecentlyPlayed 获取最近播放
func (m *Manager) GetRecentlyPlayed(userID string, limit int) []Video {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// 按最后更新时间排序会话
	type sessionWithTime struct {
		session *PlaySession
		video   *Video
	}

	var recent []sessionWithTime
	for _, s := range m.sessions {
		if userID != "" && s.UserID != userID {
			continue
		}
		if v, ok := m.videos[s.VideoID]; ok {
			recent = append(recent, sessionWithTime{session: s, video: v})
		}
	}

	// 简单排序（按时间倒序）
	for i := 0; i < len(recent); i++ {
		for j := i + 1; j < len(recent); j++ {
			if recent[j].session.LastUpdated.After(recent[i].session.LastUpdated) {
				recent[i], recent[j] = recent[j], recent[i]
			}
		}
	}

	result := make([]Video, 0, limit)
	for i, r := range recent {
		if i >= limit {
			break
		}
		result = append(result, *r.video)
	}
	return result
}

// GetStats 获取视频统计
func (m *Manager) GetStats() *VideoStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &VideoStats{
		TotalVideos:     len(m.videos),
		TotalLibraries:  len(m.libraries),
		CodecBreakdown:  make(map[string]int),
		FormatBreakdown: make(map[string]int),
	}

	for _, v := range m.videos {
		stats.TotalSize += v.FileSize
		stats.TotalDuration += v.Duration
		stats.CodecBreakdown[string(v.VideoCodec)]++
		stats.FormatBreakdown[v.Container]++
	}

	for _, s := range m.sessions {
		if time.Since(s.LastUpdated) < 30*time.Minute {
			stats.ActiveSessions++
		}
	}

	for _, j := range m.transcodes {
		if j.Status == TranscodeStatusRunning {
			stats.ActiveTranscodes++
		}
	}

	recent := m.GetRecentlyPlayed("", 5)
	stats.RecentlyPlayed = recent

	return stats
}
