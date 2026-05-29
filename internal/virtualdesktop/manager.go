// Package virtualdesktop implements a virtual desktop gateway for remote access
// to VMs and containers. Supports RDP, VNC, and SSH protocols with web-based
// access through HTML5. Inspired by Synology Virtual Machine Manager.
package virtualdesktop

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Protocol represents the remote desktop protocol
type Protocol string

const (
	ProtocolRDP Protocol = "rdp"
	ProtocolVNC Protocol = "vnc"
	ProtocolSSH Protocol = "ssh"
	ProtocolSPICE Protocol = "spice"
)

// DesktopState represents the state of a virtual desktop
type DesktopState string

const (
	DesktopStateRunning  DesktopState = "running"
	DesktopStateStopped  DesktopState = "stopped"
	DesktopStatePaused   DesktopState = "paused"
	DesktopStateError    DesktopState = "error"
	DesktopStateCreating DesktopState = "creating"
)

// SessionState represents a user session state
type SessionState string

const (
	SessionStateActive   SessionState = "active"
	SessionStateIdle     SessionState = "idle"
	SessionStateDisconnected SessionState = "disconnected"
)

// VirtualDesktop represents a virtual desktop instance
type VirtualDesktop struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Protocol    Protocol     `json:"protocol"`
	State       DesktopState `json:"state"`
	VMID        string       `json:"vm_id"`
	ContainerID string       `json:"container_id"`
	Hostname    string       `json:"hostname"`
	Port        int          `json:"port"`
	Resolution  string       `json:"resolution"`
	ColorDepth  int          `json:"color_depth"`
	MaxSessions int          `json:"max_sessions"`
	Encrypted   bool         `json:"encrypted"`
	Username    string       `json:"username"`
	Password    string       `json:"-"` // Not serialized
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// DesktopSession represents an active user session
type DesktopSession struct {
	ID          string       `json:"id"`
	DesktopID   string       `json:"desktop_id"`
	UserID      string       `json:"user_id"`
	Username    string       `json:"username"`
	RemoteAddr  string       `json:"remote_addr"`
	State       SessionState `json:"state"`
	Protocol    Protocol     `json:"protocol"`
	Resolution  string       `json:"resolution"`
	StartedAt   time.Time    `json:"started_at"`
	LastInput   time.Time    `json:"last_input"`
	BytesUp     int64        `json:"bytes_up"`
	BytesDown   int64        `json:"bytes_down"`
	FPS         int          `json:"fps"`
	Latency     int          `json:"latency_ms"`
}

// GatewayConfig configures the virtual desktop gateway
type GatewayConfig struct {
	ListenAddr     string        `json:"listen_addr"`
	ListenPort     int           `json:"listen_port"`
	MaxSessions    int           `json:"max_sessions"`
	IdleTimeout    time.Duration `json:"idle_timeout"`
	SessionTimeout time.Duration `json:"session_timeout"`
	EnableWebRTC   bool          `json:"enable_webrtc"`
	EnableRecording bool         `json:"enable_recording"`
	RecordingPath  string        `json:"recording_path"`
	EnableClipboard bool        `json:"enable_clipboard"`
	EnableDriveRedir bool       `json:"enable_drive_redirect"`
}

// DefaultGatewayConfig returns sensible defaults
func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{
		ListenAddr:      "0.0.0.0",
		ListenPort:      8443,
		MaxSessions:     100,
		IdleTimeout:     30 * time.Minute,
		SessionTimeout:  8 * time.Hour,
		EnableWebRTC:    true,
		EnableClipboard: true,
		EnableDriveRedir: true,
	}
}

// DesktopManager manages virtual desktops and sessions
type DesktopManager struct {
	mu        sync.RWMutex
	config    GatewayConfig
	logger    *zap.Logger
	desktops  map[string]*VirtualDesktop
	sessions  map[string]*DesktopSession
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewDesktopManager creates a new desktop manager
func NewDesktopManager(config GatewayConfig, logger *zap.Logger) *DesktopManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &DesktopManager{
		config:   config,
		logger:   logger,
		desktops: make(map[string]*VirtualDesktop),
		sessions: make(map[string]*DesktopSession),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// CreateDesktop creates a new virtual desktop
func (dm *DesktopManager) CreateDesktop(desktop *VirtualDesktop) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.desktops[desktop.ID]; exists {
		return fmt.Errorf("desktop %s already exists", desktop.ID)
	}

	now := time.Now()
	desktop.CreatedAt = now
	desktop.UpdatedAt = now
	desktop.State = DesktopStateStopped
	dm.desktops[desktop.ID] = desktop

	dm.logger.Info("desktop created",
		zap.String("id", desktop.ID),
		zap.String("name", desktop.Name))
	return nil
}

// StartDesktop starts a virtual desktop
func (dm *DesktopManager) StartDesktop(ctx context.Context, desktopID string) error {
	dm.mu.Lock()
	desktop, ok := dm.desktops[desktopID]
	if !ok {
		dm.mu.Unlock()
		return fmt.Errorf("desktop %s not found", desktopID)
	}
	desktop.State = DesktopStateRunning
	desktop.UpdatedAt = time.Now()
	dm.mu.Unlock()

	dm.logger.Info("desktop started", zap.String("id", desktopID))
	return nil
}

// StopDesktop stops a virtual desktop
func (dm *DesktopManager) StopDesktop(ctx context.Context, desktopID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	desktop, ok := dm.desktops[desktopID]
	if !ok {
		return fmt.Errorf("desktop %s not found", desktopID)
	}

	// Disconnect all sessions
	for _, session := range dm.sessions {
		if session.DesktopID == desktopID {
			session.State = SessionStateDisconnected
		}
	}

	desktop.State = DesktopStateStopped
	desktop.UpdatedAt = time.Now()
	return nil
}

// ConnectSession creates a new user session
func (dm *DesktopManager) ConnectSession(ctx context.Context, desktopID, userID, username, remoteAddr string) (*DesktopSession, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	desktop, ok := dm.desktops[desktopID]
	if !ok {
		return nil, fmt.Errorf("desktop %s not found", desktopID)
	}
	if desktop.State != DesktopStateRunning {
		return nil, fmt.Errorf("desktop %s not running", desktopID)
	}

	// Count active sessions
	activeCount := 0
	for _, s := range dm.sessions {
		if s.DesktopID == desktopID && s.State == SessionStateActive {
			activeCount++
		}
	}
	if activeCount >= desktop.MaxSessions {
		return nil, fmt.Errorf("max sessions reached for desktop %s", desktopID)
	}

	session := &DesktopSession{
		ID:         fmt.Sprintf("sess-%d", time.Now().UnixNano()),
		DesktopID:  desktopID,
		UserID:     userID,
		Username:   username,
		RemoteAddr: remoteAddr,
		State:      SessionStateActive,
		Protocol:   desktop.Protocol,
		Resolution: desktop.Resolution,
		StartedAt:  time.Now(),
		LastInput:  time.Now(),
	}
	dm.sessions[session.ID] = session

	dm.logger.Info("session connected",
		zap.String("session", session.ID),
		zap.String("desktop", desktopID),
		zap.String("user", username))

	return session, nil
}

// DisconnectSession disconnects a user session
func (dm *DesktopManager) DisconnectSession(sessionID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	session, ok := dm.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	session.State = SessionStateDisconnected
	delete(dm.sessions, sessionID)

	dm.logger.Info("session disconnected", zap.String("session", sessionID))
	return nil
}

// GetStats returns desktop manager statistics
func (dm *DesktopManager) GetStats() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	stateCounts := make(map[string]int)
	for _, d := range dm.desktops {
		stateCounts[string(d.State)]++
	}

	activeSessions := 0
	for _, s := range dm.sessions {
		if s.State == SessionStateActive {
			activeSessions++
		}
	}

	return map[string]interface{}{
		"total_desktops":  len(dm.desktops),
		"desktop_states":  stateCounts,
		"total_sessions":  len(dm.sessions),
		"active_sessions": activeSessions,
	}
}

// ListDesktops lists all virtual desktops
func (dm *DesktopManager) ListDesktops() []*VirtualDesktop {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	desktops := make([]*VirtualDesktop, 0, len(dm.desktops))
	for _, d := range dm.desktops {
		desktops = append(desktops, d)
	}
	return desktops
}

// ListSessions lists all active sessions
func (dm *DesktopManager) ListSessions() []*DesktopSession {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	sessions := make([]*DesktopSession, 0, len(dm.sessions))
	for _, s := range dm.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// Stop stops the desktop manager
func (dm *DesktopManager) Stop() {
	dm.cancel()
	dm.wg.Wait()
}
