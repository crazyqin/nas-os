// Package webrtc 提供 WebRTC 实时流媒体功能
// 用于 NAS 监控摄像头实时预览 / 远程桌面 / 低延迟音视频传输
// 支持 P2P / TURN relay / 多码率自适应 / 录制
package webrtc

import (
	"fmt"
	"sync"
	"time"
)

// SessionState 会话状态.
type SessionState string

const (
	StateNew        SessionState = "new"
	StateConnecting SessionState = "connecting"
	StateConnected  SessionState = "connected"
	StateCompleted  SessionState = "completed"
	StateFailed     SessionState = "failed"
	StateClosed     SessionState = "closed"
)

// MediaType 媒体类型.
type MediaType string

const (
	MediaVideo MediaType = "video"
	MediaAudio MediaType = "audio"
	MediaData  MediaType = "data"
)

// ICECandidate ICE候选.
type ICECandidate struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid"`
	SDPMLineIndex int    `json:"sdpMLineIndex"`
}

// SessionDescription 会话描述.
type SessionDescription struct {
	Type string `json:"type"` // offer, answer
	SDP  string `json:"sdp"`
}

// PeerConnection 对等连接.
type PeerConnection struct {
	mu            sync.RWMutex
	ID            string              `json:"id"`
	State         SessionState        `json:"state"`
	LocalSDP      *SessionDescription `json:"localSdp"`
	RemoteSDP     *SessionDescription `json:"remoteSdp"`
	ICECandidates []*ICECandidate     `json:"iceCandidates"`
	MediaTracks   []*MediaTrack       `json:"mediaTracks"`
	CreatedAt     time.Time           `json:"createdAt"`
	ConnectedAt   time.Time           `json:"connectedAt"`
	BytesSent     int64               `json:"bytesSent"`
	BytesReceived int64               `json:"bytesReceived"`
	PacketsLost   int64               `json:"packetsLost"`
	RTT           time.Duration       `json:"rtt"`
	RemoteAddr    string              `json:"remoteAddr"`
}

// MediaTrack 媒体轨道.
type MediaTrack struct {
	ID        string    `json:"id"`
	Kind      MediaType `json:"kind"`
	Label     string    `json:"label"`
	Enabled   bool      `json:"enabled"`
	Muted     bool      `json:"muted"`
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
	FrameRate float64   `json:"frameRate,omitempty"`
	Bitrate   int       `json:"bitrate,omitempty"`
	Codec     string    `json:"codec,omitempty"`
}

// Stream 媒体流.
type Stream struct {
	mu      sync.RWMutex
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Tracks  []*MediaTrack `json:"tracks"`
	Viewers int           `json:"viewers"`
	Source  string        `json:"source"` // camera, screen, file
	Active  bool          `json:"active"`
}

// Recording 录制任务.
type Recording struct {
	ID        string        `json:"id"`
	StreamID  string        `json:"streamId"`
	Format    string        `json:"format"` // webm, mp4
	StartTime time.Time     `json:"startTime"`
	EndTime   time.Time     `json:"endTime"`
	Duration  time.Duration `json:"duration"`
	Size      int64         `json:"size"`
	Path      string        `json:"path"`
	Status    string        `json:"status"`
}

// SignalingMessage 信令消息.
type SignalingMessage struct {
	Type      string `json:"type"` // offer, answer, candidate, bye
	SessionID string `json:"sessionId"`
	Payload   []byte `json:"payload"`
}

// WebRTCStats WebRTC统计.
type WebRTCStats struct {
	mu                 sync.RWMutex
	TotalSessions      int       `json:"totalSessions"`
	ActiveSessions     int       `json:"activeSessions"`
	TotalStreams       int       `json:"totalStreams"`
	ActiveStreams      int       `json:"activeStreams"`
	TotalRecordings    int       `json:"totalRecordings"`
	ActiveRecordings   int       `json:"activeRecordings"`
	TotalBytesSent     int64     `json:"totalBytesSent"`
	TotalBytesReceived int64     `json:"totalBytesReceived"`
	StartedAt          time.Time `json:"startedAt"`
}

// WebRTCServer WebRTC服务器.
type WebRTCServer struct {
	mu         sync.RWMutex
	config     *WebRTCConfig
	sessions   map[string]*PeerConnection
	streams    map[string]*Stream
	recordings map[string]*Recording
	stats      *WebRTCStats
}

// WebRTCConfig 服务器配置.
type WebRTCConfig struct {
	STUNServer         string `json:"stunServer"`
	TURNServer         string `json:"turnServer"`
	TURNUsername       string `json:"turnUsername"`
	TURNCredential     string `json:"turnCredential"`
	MaxSessions        int    `json:"maxSessions"`
	MaxBitrate         int    `json:"maxBitrate"` // bps
	ICETransportPolicy string `json:"iceTransportPolicy"`
}

// NewWebRTCServer 创建WebRTC服务器.
func NewWebRTCServer(config *WebRTCConfig) *WebRTCServer {
	if config == nil {
		config = &WebRTCConfig{
			STUNServer:  "stun:stun.l.google.com:19302",
			MaxSessions: 100,
			MaxBitrate:  5000000, // 5Mbps
		}
	}
	return &WebRTCServer{
		config:     config,
		sessions:   make(map[string]*PeerConnection),
		streams:    make(map[string]*Stream),
		recordings: make(map[string]*Recording),
		stats:      &WebRTCStats{StartedAt: time.Now()},
	}
}

// CreateSession 创建WebRTC会话.
func (s *WebRTCServer) CreateSession(sessionID string) (*PeerConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.sessions) >= s.config.MaxSessions {
		return nil, fmt.Errorf("max sessions reached: %d", s.config.MaxSessions)
	}

	pc := &PeerConnection{
		ID:        sessionID,
		State:     StateNew,
		CreatedAt: time.Now(),
	}
	s.sessions[sessionID] = pc

	s.stats.mu.Lock()
	s.stats.TotalSessions++
	s.stats.ActiveSessions++
	s.stats.mu.Unlock()

	return pc, nil
}

// CloseSession 关闭会话.
func (s *WebRTCServer) CloseSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pc, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	pc.State = StateClosed
	delete(s.sessions, sessionID)

	s.stats.mu.Lock()
	s.stats.ActiveSessions--
	s.stats.TotalBytesSent += pc.BytesSent
	s.stats.TotalBytesReceived += pc.BytesReceived
	s.stats.mu.Unlock()

	return nil
}

// SetRemoteSDP 设置远程SDP.
func (s *WebRTCServer) SetRemoteSDP(sessionID string, sdp *SessionDescription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pc, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	pc.RemoteSDP = sdp
	if sdp.Type == "offer" {
		pc.State = StateConnecting
	}
	return nil
}

// SetLocalSDP 设置本地SDP.
func (s *WebRTCServer) SetLocalSDP(sessionID string, sdp *SessionDescription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pc, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	pc.LocalSDP = sdp
	return nil
}

// AddICECandidate 添加ICE候选.
func (s *WebRTCServer) AddICECandidate(sessionID string, candidate *ICECandidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pc, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	pc.ICECandidates = append(pc.ICECandidates, candidate)
	return nil
}

// ConnectSession 连接会话.
func (s *WebRTCServer) ConnectSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pc, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	pc.State = StateConnected
	pc.ConnectedAt = time.Now()
	return nil
}

// CreateStream 创建媒体流.
func (s *WebRTCServer) CreateStream(id, name, source string, tracks []*MediaTrack) *Stream {
	s.mu.Lock()
	defer s.mu.Unlock()

	stream := &Stream{
		ID:     id,
		Name:   name,
		Tracks: tracks,
		Source: source,
		Active: true,
	}
	s.streams[id] = stream

	s.stats.mu.Lock()
	s.stats.TotalStreams++
	s.stats.ActiveStreams++
	s.stats.mu.Unlock()

	return stream
}

// GetStream 获取媒体流.
func (s *WebRTCServer) GetStream(id string) (*Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stream, ok := s.streams[id]
	if !ok {
		return nil, fmt.Errorf("stream %s not found", id)
	}
	return stream, nil
}

// ListStreams 列出所有流.
func (s *WebRTCServer) ListStreams() []*Stream {
	s.mu.RLock()
	defer s.mu.RUnlock()

	streams := make([]*Stream, 0, len(s.streams))
	for _, stream := range s.streams {
		streams = append(streams, stream)
	}
	return streams
}

// StartRecording 开始录制.
func (s *WebRTCServer) StartRecording(id, streamID, format, path string) (*Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.streams[streamID]; !ok {
		return nil, fmt.Errorf("stream %s not found", streamID)
	}

	recording := &Recording{
		ID:        id,
		StreamID:  streamID,
		Format:    format,
		StartTime: time.Now(),
		Path:      path,
		Status:    "recording",
	}
	s.recordings[id] = recording

	s.stats.mu.Lock()
	s.stats.TotalRecordings++
	s.stats.ActiveRecordings++
	s.stats.mu.Unlock()

	return recording, nil
}

// StopRecording 停止录制.
func (s *WebRTCServer) StopRecording(id string) (*Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	recording, ok := s.recordings[id]
	if !ok {
		return nil, fmt.Errorf("recording %s not found", id)
	}

	recording.EndTime = time.Now()
	recording.Duration = recording.EndTime.Sub(recording.StartTime)
	recording.Status = "completed"

	s.stats.mu.Lock()
	s.stats.ActiveRecordings--
	s.stats.mu.Unlock()

	return recording, nil
}

// GetStats 获取统计.
func (s *WebRTCServer) GetStats() *WebRTCStats {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()
	return &WebRTCStats{
		TotalSessions:      s.stats.TotalSessions,
		ActiveSessions:     s.stats.ActiveSessions,
		TotalStreams:       s.stats.TotalStreams,
		ActiveStreams:      s.stats.ActiveStreams,
		TotalRecordings:    s.stats.TotalRecordings,
		ActiveRecordings:   s.stats.ActiveRecordings,
		TotalBytesSent:     s.stats.TotalBytesSent,
		TotalBytesReceived: s.stats.TotalBytesReceived,
		StartedAt:          s.stats.StartedAt,
	}
}

// GetSession 获取会话.
func (s *WebRTCServer) GetSession(id string) (*PeerConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pc, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return pc, nil
}

// ListSessions 列出所有会话.
func (s *WebRTCServer) ListSessions() []*PeerConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*PeerConnection, 0, len(s.sessions))
	for _, pc := range s.sessions {
		sessions = append(sessions, pc)
	}
	return sessions
}
