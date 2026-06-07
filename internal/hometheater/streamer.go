// Package hometheater 提供流媒体服务功能
package hometheater

import (
	"fmt"
	"sync"
	"time"
)

// Streamer 流媒体服务.
type Streamer struct {
	mu              sync.RWMutex
	engine          *Engine
	transcoder      *Transcoder
	sessions        map[string]*StreamSession
	hlsSegments     map[string]*HLSSession
	dashSessions    map[string]*DASHSession
	dlnaDevices     map[string]*DLNADevice
	maxSessions     int
	segmentDuration int // 秒
	maxSegments     int
	bandwidthEst    map[string]int // 会话带宽估计
}

// HLSSession HLS会话.
type HLSSession struct {
	SessionID  string        `json:"session_id"`
	MediaID    string        `json:"media_id"`
	Playlist   string        `json:"playlist"` // m3u8内容
	Segments   []*HLSSegment `json:"segments"`
	CurrentSeq int           `json:"current_seq"`
	EndList    bool          `json:"end_list"`
	CreatedAt  time.Time     `json:"created_at"`
}

// HLSSegment HLS分片.
type HLSSegment struct {
	Sequence   int     `json:"sequence"`
	Duration   float64 `json:"duration"`
	Path       string  `json:"path"`
	Size       int64   `json:"size"`
	Bitrate    int     `json:"bitrate"`
	Resolution string  `json:"resolution"`
}

// DASHSession DASH会话.
type DASHSession struct {
	SessionID   string           `json:"session_id"`
	MediaID     string           `json:"media_id"`
	MPD         string           `json:"mpd"` // MPD内容
	Adaptations []*AdaptationSet `json:"adaptations"`
	CurrentSeg  int              `json:"current_seg"`
	CreatedAt   time.Time        `json:"created_at"`
}

// AdaptationSet DASH自适应集.
type AdaptationSet struct {
	ID              int               `json:"id"`
	Type            string            `json:"type"` // video/audio
	ContentType     string            `json:"content_type"`
	Lang            string            `json:"lang"`
	Representations []*Representation `json:"representations"`
}

// Representation DASH表示.
type Representation struct {
	ID         string `json:"id"`
	Bandwidth  int    `json:"bandwidth"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	FrameRate  string `json:"frame_rate"`
	Codecs     string `json:"codecs"`
	SegmentURL string `json:"segment_url"`
}

// StreamQuality 流媒体质量.
type StreamQuality string

const (
	// QualityAuto 自动.
	QualityAuto StreamQuality = "auto"
	// Quality4K 4K.
	Quality4K StreamQuality = "4k"
	// Quality1080p 1080p.
	Quality1080p StreamQuality = "1080p"
	// Quality720p 720p.
	Quality720p StreamQuality = "720p"
	// Quality480p 480p.
	Quality480p StreamQuality = "480p"
	// Quality360p 360p.
	Quality360p StreamQuality = "360p"
)

// StreamRequest 流媒体请求.
type StreamRequest struct {
	MediaID  string         `json:"media_id"`
	UserID   string         `json:"user_id"`
	Protocol StreamProtocol `json:"protocol"`
	Quality  StreamQuality  `json:"quality"`
	DeviceID string         `json:"device_id"`
	Position float64        `json:"position"` // 开始位置（秒）
}

// NewStreamer 创建流媒体服务.
func NewStreamer(engine *Engine, transcoder *Transcoder) *Streamer {
	return &Streamer{
		engine:          engine,
		transcoder:      transcoder,
		sessions:        make(map[string]*StreamSession),
		hlsSegments:     make(map[string]*HLSSession),
		dashSessions:    make(map[string]*DASHSession),
		dlnaDevices:     make(map[string]*DLNADevice),
		maxSessions:     100,
		segmentDuration: 6,
		maxSegments:     5,
		bandwidthEst:    make(map[string]int),
	}
}

// SetMaxSessions 设置最大会话数.
func (s *Streamer) SetMaxSessions(max int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if max > 0 {
		s.maxSessions = max
	}
}

// CreateSession 创建流媒体会话.
func (s *Streamer) CreateSession(req *StreamRequest) (*StreamSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.sessions) >= s.maxSessions {
		return nil, fmt.Errorf("会话数已达上限: %d", s.maxSessions)
	}

	// 验证媒体存在
	_, err := s.engine.GetMovie(req.MediaID)
	if err != nil {
		_, err = s.engine.GetEpisode(req.MediaID)
		if err != nil {
			return nil, ErrMediaNotFound
		}
	}

	session := &StreamSession{
		ID:            fmt.Sprintf("stream_%d", time.Now().UnixNano()),
		MediaID:       req.MediaID,
		UserID:        req.UserID,
		Protocol:      req.Protocol,
		State:         PlaybackPlaying,
		Position:      req.Position,
		DeviceID:      req.DeviceID,
		StartTime:     time.Now(),
		LastHeartbeat: time.Now(),
	}

	// 根据协议创建会话
	switch req.Protocol {
	case ProtocolHLS:
		if err := s.createHLSSession(session, req); err != nil {
			return nil, err
		}
	case ProtocolDASH:
		if err := s.createDASHSession(session, req); err != nil {
			return nil, err
		}
	case ProtocolDLNA:
		if err := s.createDLNASession(session, req); err != nil {
			return nil, err
		}
	case ProtocolDirect:
		// 直接播放，无需特殊处理
	default:
		return nil, fmt.Errorf("不支持的协议: %s", req.Protocol)
	}

	s.sessions[session.ID] = session
	return session, nil
}

// GetSession 获取流媒体会话.
func (s *Streamer) GetSession(sessionID string) (*StreamSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, ErrStreamNotFound
	}
	return session, nil
}

// ListSessions 列出所有会话.
func (s *Streamer) ListSessions(userID string) []*StreamSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*StreamSession, 0)
	for _, session := range s.sessions {
		if userID == "" || session.UserID == userID {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// EndSession 结束会话.
func (s *Streamer) EndSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrStreamNotFound
	}

	session.State = PlaybackStopped

	// 清理HLS/DASH会话
	delete(s.hlsSegments, sessionID)
	delete(s.dashSessions, sessionID)
	delete(s.sessions, sessionID)

	return nil
}

// Heartbeat 会话心跳.
func (s *Streamer) Heartbeat(sessionID string, position float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrStreamNotFound
	}

	session.LastHeartbeat = time.Now()
	session.Position = position

	return nil
}

// PauseSession 暂停会话.
func (s *Streamer) PauseSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrStreamNotFound
	}

	session.State = PlaybackPaused
	return nil
}

// ResumeSession 恢复会话.
func (s *Streamer) ResumeSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrStreamNotFound
	}

	session.State = PlaybackPlaying
	return nil
}

// SeekSession 跳转位置.
func (s *Streamer) SeekSession(sessionID string, position float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrStreamNotFound
	}

	session.Position = position
	return nil
}

// SwitchQuality 切换画质.
func (s *Streamer) SwitchQuality(sessionID string, quality StreamQuality) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrStreamNotFound
	}

	// 根据画质选择转码配置
	profile := s.getProfileByQuality(quality)
	if profile != nil {
		session.Profile = profile
	}

	return nil
}

// GetHLSPlaylist 获取HLS播放列表.
func (s *Streamer) GetHLSPlaylist(sessionID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hls, exists := s.hlsSegments[sessionID]
	if !exists {
		return "", ErrStreamNotFound
	}

	return hls.Playlist, nil
}

// GetHLSSegment 获取HLS分片.
func (s *Streamer) GetHLSSegment(sessionID string, seq int) (*HLSSegment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hls, exists := s.hlsSegments[sessionID]
	if !exists {
		return nil, ErrStreamNotFound
	}

	for _, seg := range hls.Segments {
		if seg.Sequence == seq {
			return seg, nil
		}
	}

	return nil, fmt.Errorf("分片不存在: %d", seq)
}

// GetDASHMPD 获取DASH MPD.
func (s *Streamer) GetDASHMPD(sessionID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dash, exists := s.dashSessions[sessionID]
	if !exists {
		return "", ErrStreamNotFound
	}

	return dash.MPD, nil
}

// RegisterDLNADevice 注册DLNA设备.
func (s *Streamer) RegisterDLNADevice(device *DLNADevice) {
	s.mu.Lock()
	defer s.mu.Unlock()

	device.LastSeen = time.Now()
	device.Online = true
	s.dlnaDevices[device.ID] = device
}

// UnregisterDLNADevice 注销DLNA设备.
func (s *Streamer) UnregisterDLNADevice(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if device, exists := s.dlnaDevices[deviceID]; exists {
		device.Online = false
	}
}

// ListDLNADevices 列出DLNA设备.
func (s *Streamer) ListDLNADevices() []*DLNADevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*DLNADevice, 0, len(s.dlnaDevices))
	for _, device := range s.dlnaDevices {
		devices = append(devices, device)
	}
	return devices
}

// CastToDLNA 投屏到DLNA设备.
func (s *Streamer) CastToDLNA(sessionID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrStreamNotFound
	}

	device, exists := s.dlnaDevices[deviceID]
	if !exists {
		return ErrDLNADeviceNotFound
	}

	if !device.Online {
		return ErrDLNADeviceNotFound
	}

	session.DeviceID = deviceID
	session.DeviceName = device.Name
	session.Protocol = ProtocolDLNA

	return nil
}

// GetActiveSessionCount 获取活跃会话数.
func (s *Streamer) GetActiveSessionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// CleanupStaleSessions 清理过期会话.
func (s *Streamer) CleanupStaleSessions(timeout time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleaned := 0
	now := time.Now()

	for id, session := range s.sessions {
		if now.Sub(session.LastHeartbeat) > timeout {
			session.State = PlaybackStopped
			delete(s.hlsSegments, id)
			delete(s.dashSessions, id)
			delete(s.sessions, id)
			cleaned++
		}
	}

	return cleaned
}

// 内部方法

// createHLSSession 创建HLS会话.
func (s *Streamer) createHLSSession(session *StreamSession, req *StreamRequest) error {
	hls := &HLSSession{
		SessionID:  session.ID,
		MediaID:    req.MediaID,
		CurrentSeq: 0,
		CreatedAt:  time.Now(),
	}

	// 生成M3U8播放列表
	profile := s.getProfileByQuality(req.Quality)
	if profile != nil {
		session.Profile = profile
	}

	hls.Playlist = s.generateM3U8(hls, profile)
	s.hlsSegments[session.ID] = hls

	return nil
}

// createDASHSession 创建DASH会话.
func (s *Streamer) createDASHSession(session *StreamSession, req *StreamRequest) error {
	dash := &DASHSession{
		SessionID:  session.ID,
		MediaID:    req.MediaID,
		CurrentSeg: 0,
		CreatedAt:  time.Now(),
	}

	// 生成MPD
	dash.MPD = s.generateMPD(dash)
	s.dashSessions[session.ID] = dash

	return nil
}

// createDLNASession 创建DLNA会话.
func (s *Streamer) createDLNASession(session *StreamSession, req *StreamRequest) error {
	if req.DeviceID == "" {
		return fmt.Errorf("DLNA设备ID不能为空")
	}

	device, exists := s.dlnaDevices[req.DeviceID]
	if !exists || !device.Online {
		return ErrDLNADeviceNotFound
	}

	session.DeviceID = device.ID
	session.DeviceName = device.Name

	return nil
}

// generateM3U8 生成M3U8播放列表.
func (s *Streamer) generateM3U8(hls *HLSSession, profile *TranscodeProfile) string {
	m3u8 := "#EXTM3U\n"
	m3u8 += "#EXT-X-VERSION:3\n"
	m3u8 += fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", s.segmentDuration)
	m3u8 += fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d\n", hls.CurrentSeq)

	if profile != nil {
		m3u8 += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n",
			profile.VideoBitrate*1000, profile.Width, profile.Height)
	}

	// 添加分片
	for i := 0; i < s.maxSegments; i++ {
		m3u8 += fmt.Sprintf("#EXTINF:%.3f,\n", float64(s.segmentDuration))
		m3u8 += fmt.Sprintf("segment_%d.ts\n", hls.CurrentSeq+i)
	}

	return m3u8
}

// generateMPD 生成MPD.
func (s *Streamer) generateMPD(dash *DASHSession) string {
	mpd := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011"
     type="dynamic"
     minimumUpdatePeriod="PT5S"
     minBufferTime="PT3S"
     availabilityStartTime="`
	mpd += dash.CreatedAt.Format(time.RFC3339)
	mpd += `"
     profiles="urn:mpeg:dash:profile:isoff-live:2011">
  <Period id="1" start="PT0S">
    <AdaptationSet mimeType="video/mp4">
      <Representation id="720p" bandwidth="2500000" width="1280" height="720"/>
      <Representation id="1080p" bandwidth="5000000" width="1920" height="1080"/>
    </AdaptationSet>
    <AdaptationSet mimeType="audio/mp4">
      <Representation id="audio" bandwidth="128000" codecs="mp4a.40.2"/>
    </AdaptationSet>
  </Period>
</MPD>`
	return mpd
}

// getProfileByQuality 根据画质获取转码配置.
func (s *Streamer) getProfileByQuality(quality StreamQuality) *TranscodeProfile {
	profiles := map[StreamQuality]*TranscodeProfile{
		Quality4K: {
			ID:           "4k",
			Name:         "4K",
			VideoCodec:   CodecH265,
			AudioCodec:   AudioCodecAAC,
			Width:        3840,
			Height:       2160,
			VideoBitrate: 20000,
			AudioBitrate: 256,
		},
		Quality1080p: {
			ID:           "1080p",
			Name:         "1080p",
			VideoCodec:   CodecH264,
			AudioCodec:   AudioCodecAAC,
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000,
			AudioBitrate: 128,
		},
		Quality720p: {
			ID:           "720p",
			Name:         "720p",
			VideoCodec:   CodecH264,
			AudioCodec:   AudioCodecAAC,
			Width:        1280,
			Height:       720,
			VideoBitrate: 2500,
			AudioBitrate: 128,
		},
		Quality480p: {
			ID:           "480p",
			Name:         "480p",
			VideoCodec:   CodecH264,
			AudioCodec:   AudioCodecAAC,
			Width:        854,
			Height:       480,
			VideoBitrate: 1000,
			AudioBitrate: 96,
		},
		Quality360p: {
			ID:           "360p",
			Name:         "360p",
			VideoCodec:   CodecH264,
			AudioCodec:   AudioCodecAAC,
			Width:        640,
			Height:       360,
			VideoBitrate: 500,
			AudioBitrate: 64,
		},
	}

	if profile, ok := profiles[quality]; ok {
		return profile
	}

	// 默认1080p
	return profiles[Quality1080p]
}

// EstimateBandwidth 估计带宽.
func (s *Streamer) EstimateBandwidth(sessionID string, bytesTransferred int64, duration time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if duration.Seconds() == 0 {
		return 0
	}

	bandwidth := int(float64(bytesTransferred*8) / duration.Seconds())
	s.bandwidthEst[sessionID] = bandwidth

	return bandwidth
}

// GetRecommendedQuality 获取推荐画质.
func (s *Streamer) GetRecommendedQuality(sessionID string) StreamQuality {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bandwidth, exists := s.bandwidthEst[sessionID]
	if !exists {
		return Quality720p
	}

	// 根据带宽推荐画质（预留20%余量）
	available := bandwidth * 80 / 100

	switch {
	case available >= 25000000: // 25Mbps
		return Quality4K
	case available >= 8000000: // 8Mbps
		return Quality1080p
	case available >= 4000000: // 4Mbps
		return Quality720p
	case available >= 2000000: // 2Mbps
		return Quality480p
	default:
		return Quality360p
	}
}
