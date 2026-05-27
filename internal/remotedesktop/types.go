// Package remotedesktop provides browser-based remote desktop gateway for NAS-OS
// Features: VNC/RDP over WebSocket, session recording, multi-monitor, clipboard sync
// Competitor benchmark: 对标群晖Remote Desktop, 超越TrueNAS IPMI/KVM能力
package remotedesktop

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Protocol represents the remote desktop protocol
type Protocol string

const (
	ProtocolVNC Protocol = "vnc"
	ProtocolRDP Protocol = "rdp"
	ProtocolSSH Protocol = "ssh"
)

// SessionStatus represents session status
type SessionStatus string

const (
	StatusConnecting  SessionStatus = "connecting"
	StatusConnected   SessionStatus = "connected"
	StatusDisconnected SessionStatus = "disconnected"
	StatusError       SessionStatus = "error"
)

// Session represents a remote desktop session
type Session struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Protocol    Protocol      `json:"protocol"`
	Host        string        `json:"host"`
	Port        int           `json:"port"`
	Status      SessionStatus `json:"status"`
	Width       int           `json:"width"`
	Height      int           `json:"height"`
	ColorDepth  int           `json:"color_depth"`
	UserID      string        `json:"user_id"`
	StartedAt   time.Time     `json:"started_at"`
	LastInputAt time.Time     `json:"last_input_at"`
	Recording   bool          `json:"recording"`
	ClipSync    bool          `json:"clipboard_sync"`
	Tags        []string      `json:"tags"`
}

// Host represents a remote host configuration
type Host struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Hostname    string   `json:"hostname"`
	Protocol    Protocol `json:"protocol"`
	Port        int      `json:"port"`
	Username    string   `json:"username"`
	PasswordRef string   `json:"password_ref"` // Reference to vault
	Resolution  string   `json:"resolution"`
	Tags        []string `json:"tags"`
	Enabled     bool     `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// Recording represents a session recording
type Recording struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Host      string    `json:"host"`
	Protocol  Protocol  `json:"protocol"`
	Duration  int64     `json:"duration_seconds"`
	Size      int64     `json:"size_bytes"`
	Path      string    `json:"path"`
	UserID    string    `json:"user_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// ClipboardEntry represents a clipboard sync entry
type ClipboardEntry struct {
	SessionID string    `json:"session_id"`
	Content   string    `json:"content"`
	Direction string    `json:"direction"` // "in" or "out"
	Timestamp time.Time `json:"timestamp"`
}

// SessionStats represents session statistics
type SessionStats struct {
	TotalSessions    int   `json:"total_sessions"`
	ActiveSessions   int   `json:"active_sessions"`
	TotalRecordings  int   `json:"total_recordings"`
	TotalHosts       int   `json:"total_hosts"`
	TotalBandwidth   int64 `json:"total_bandwidth_bytes"`
	AvgLatency       int   `json:"avg_latency_ms"`
}

// Config holds remote desktop configuration
type Config struct {
	Enabled          bool   `json:"enabled"`
	WebSocketPort    int    `json:"websocket_port"`
	MaxSessions      int    `json:"max_sessions"`
	RecordingEnabled bool   `json:"recording_enabled"`
	ClipSyncEnabled  bool   `json:"clipboard_sync_enabled"`
	DefaultWidth     int    `json:"default_width"`
	DefaultHeight    int    `json:"default_height"`
	DefaultColor     int    `json:"default_color_depth"`
	SessionTimeout   int    `json:"session_timeout_minutes"`
}

// Manager manages remote desktop sessions
type Manager struct {
	config    *Config
	sessions  map[string]*Session
	hosts     map[string]*Host
	recordings []*Recording
	clipboard  map[string][]*ClipboardEntry
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new remote desktop manager
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:     config,
		sessions:   make(map[string]*Session),
		hosts:      make(map[string]*Host),
		recordings: make([]*Recording, 0),
		clipboard:  make(map[string][]*ClipboardEntry),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the remote desktop manager
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return fmt.Errorf("remote desktop is disabled")
	}
	return nil
}

// Stop stops the remote desktop manager
func (m *Manager) Stop() {
	m.cancel()
}

// CreateSession creates a new remote desktop session
func (m *Manager) CreateSession(hostID string, userID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	host, ok := m.hosts[hostID]
	if !ok {
		return nil, fmt.Errorf("host %s not found", hostID)
	}

	if !host.Enabled {
		return nil, fmt.Errorf("host %s is disabled", hostID)
	}

	if len(m.sessions) >= m.config.MaxSessions {
		return nil, fmt.Errorf("maximum sessions reached (%d)", m.config.MaxSessions)
	}

	session := &Session{
		ID:          fmt.Sprintf("sess-%d", time.Now().UnixNano()),
		Name:        fmt.Sprintf("%s@%s", host.Username, host.Name),
		Protocol:    host.Protocol,
		Host:        host.Hostname,
		Port:        host.Port,
		Status:      StatusConnecting,
		Width:       m.config.DefaultWidth,
		Height:      m.config.DefaultHeight,
		ColorDepth:  m.config.DefaultColor,
		UserID:      userID,
		StartedAt:   time.Now(),
		LastInputAt: time.Now(),
		Recording:   m.config.RecordingEnabled,
		ClipSync:    m.config.ClipSyncEnabled,
		Tags:        host.Tags,
	}

	m.sessions[session.ID] = session
	return session, nil
}

// GetSession returns a session by ID
func (m *Manager) GetSession(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return session, nil
}

// ListSessions returns all sessions
func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// EndSession ends a remote desktop session
func (m *Manager) EndSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}

	session.Status = StatusDisconnected
	return nil
}

// AddHost adds a new remote host
func (m *Manager) AddHost(host *Host) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	host.ID = fmt.Sprintf("host-%d", time.Now().UnixNano())
	host.CreatedAt = time.Now()
	m.hosts[host.ID] = host
	return nil
}

// ListHosts returns all configured hosts
func (m *Manager) ListHosts() []*Host {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hosts := make([]*Host, 0, len(m.hosts))
	for _, h := range m.hosts {
		hosts = append(hosts, h)
	}
	return hosts
}

// DeleteHost deletes a remote host
func (m *Manager) DeleteHost(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.hosts[id]; !ok {
		return fmt.Errorf("host %s not found", id)
	}
	delete(m.hosts, id)
	return nil
}

// GetStats returns remote desktop statistics
func (m *Manager) GetStats() *SessionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SessionStats{
		TotalSessions: len(m.sessions),
		TotalHosts:    len(m.hosts),
	}

	for _, s := range m.sessions {
		if s.Status == StatusConnected {
			stats.ActiveSessions++
		}
	}
	stats.TotalRecordings = len(m.recordings)
	return stats
}
