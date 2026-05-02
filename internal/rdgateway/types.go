// Package rdgateway 提供浏览器内远程桌面网关功能
// 支持 RDP/VNC 协议代理、WebSocket 隧道、会话管理、剪贴板同步、文件传输
package rdgateway

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

// Protocol 远程桌面协议类型.
type Protocol string

const (
	ProtocolRDP Protocol = "rdp"
	ProtocolVNC Protocol = "vnc"
)

// SessionState 会话状态.
type SessionState string

const (
	StateConnecting SessionState = "connecting"
	StateConnected  SessionState = "connected"
	StateReconnecting SessionState = "reconnecting"
	StateDisconnected SessionState = "disconnected"
	StateError      SessionState = "error"
)

// DisplayInfo 显示器信息.
type DisplayInfo struct {
	ID       int    `json:"id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	IsPrimary bool  `json:"is_primary"`
}

// ClipboardPayload 剪贴板数据.
type ClipboardPayload struct {
	Format  string `json:"format"` // text, html, image
	Content string `json:"content"`
	Binary  []byte `json:"binary,omitempty"`
}

// FileTransferRequest 文件传输请求.
type FileTransferRequest struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	Direction   string `json:"direction"` // upload, download
	Checksum    string `json:"checksum,omitempty"`
}

// AuditEntry 审计日志条目.
type AuditEntry struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Protocol  Protocol  `json:"protocol"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details,omitempty"`
}

// Session 远程桌面会话.
type Session struct {
	ID          string        `json:"id"`
	UserID      string        `json:"user_id"`
	Protocol    Protocol      `json:"protocol"`
	Host        string        `json:"host"`
	Port        int           `json:"port"`
	State       SessionState  `json:"state"`
	Displays    []DisplayInfo `json:"displays"`
	TLSEnabled  bool          `json:"tls_enabled"`
	ConnectedAt *time.Time    `json:"connected_at,omitempty"`
	LastActive  time.Time     `json:"last_active"`
}

// CreateSessionRequest 创建会话请求.
type CreateSessionRequest struct {
	UserID     string   `json:"user_id" binding:"required"`
	Protocol   Protocol `json:"protocol" binding:"required"`
	Host       string   `json:"host" binding:"required"`
	Port       int      `json:"port"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	TLSEnabled bool     `json:"tls_enabled"`
	Displays   []DisplayInfo `json:"displays,omitempty"`
}

// SessionManager 远程桌面会话管理器.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	auditLog []AuditEntry
	tlsCfg   *tls.Config
}

// NewSessionManager 创建会话管理器.
func NewSessionManager(tlsCfg *tls.Config) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
		auditLog: make([]AuditEntry, 0),
		tlsCfg:   tlsCfg,
	}
}

// CreateSession 创建新会话.
func (sm *SessionManager) CreateSession(req CreateSessionRequest) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if req.Protocol != ProtocolRDP && req.Protocol != ProtocolVNC {
		return nil, fmt.Errorf("unsupported protocol: %s", req.Protocol)
	}

	if req.Host == "" {
		return nil, fmt.Errorf("host is required")
	}

	port := req.Port
	if port == 0 {
		if req.Protocol == ProtocolRDP {
			port = 3389
		} else {
			port = 5900
		}
	}

	if req.Displays == nil {
		req.Displays = []DisplayInfo{{ID: 0, Width: 1920, Height: 1080, IsPrimary: true}}
	}

	now := time.Now()
	session := &Session{
		ID:         generateID(),
		UserID:     req.UserID,
		Protocol:   req.Protocol,
		Host:       req.Host,
		Port:       port,
		State:      StateConnecting,
		Displays:   req.Displays,
		TLSEnabled: req.TLSEnabled,
		LastActive: now,
	}

	sm.sessions[session.ID] = session

	// 审计日志
	sm.auditLog = append(sm.auditLog, AuditEntry{
		ID:        generateID(),
		SessionID: session.ID,
		UserID:    req.UserID,
		Action:    "session_create",
		Protocol:  req.Protocol,
		Host:      req.Host,
		Port:      port,
		Timestamp: now,
	})

	return session, nil
}

// GetSession 获取会话.
func (sm *SessionManager) GetSession(id string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	return session, nil
}

// ListSessions 列出所有会话.
func (sm *SessionManager) ListSessions(userID string) []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*Session
	for _, s := range sm.sessions {
		if userID == "" || s.UserID == userID {
			result = append(result, s)
		}
	}
	return result
}

// UpdateSessionState 更新会话状态.
func (sm *SessionManager) UpdateSessionState(id string, state SessionState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}

	oldState := session.State
	session.State = state
	session.LastActive = time.Now()

	if state == StateConnected && session.ConnectedAt == nil {
		now := time.Now()
		session.ConnectedAt = &now
	}

	sm.auditLog = append(sm.auditLog, AuditEntry{
		ID:        generateID(),
		SessionID: session.ID,
		UserID:    session.UserID,
		Action:    fmt.Sprintf("state_change:%s->%s", oldState, state),
		Protocol:  session.Protocol,
		Host:      session.Host,
		Port:      session.Port,
		Timestamp: time.Now(),
	})

	return nil
}

// DisconnectSession 断开会话.
func (sm *SessionManager) DisconnectSession(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}

	session.State = StateDisconnected
	session.LastActive = time.Now()

	sm.auditLog = append(sm.auditLog, AuditEntry{
		ID:        generateID(),
		SessionID: session.ID,
		UserID:    session.UserID,
		Action:    "session_disconnect",
		Protocol:  session.Protocol,
		Host:      session.Host,
		Port:      session.Port,
		Timestamp: time.Now(),
	})

	return nil
}

// ReconnectSession 重连会话.
func (sm *SessionManager) ReconnectSession(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}

	if session.State != StateDisconnected && session.State != StateError {
		return fmt.Errorf("session %q is not in a reconnectable state (current: %s)", id, session.State)
	}

	session.State = StateReconnecting
	session.LastActive = time.Now()

	sm.auditLog = append(sm.auditLog, AuditEntry{
		ID:        generateID(),
		SessionID: session.ID,
		UserID:    session.UserID,
		Action:    "session_reconnect",
		Protocol:  session.Protocol,
		Host:      session.Host,
		Port:      session.Port,
		Timestamp: time.Now(),
	})

	return nil
}

// DeleteSession 删除会话.
func (sm *SessionManager) DeleteSession(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}

	sm.auditLog = append(sm.auditLog, AuditEntry{
		ID:        generateID(),
		SessionID: session.ID,
		UserID:    session.UserID,
		Action:    "session_delete",
		Protocol:  session.Protocol,
		Host:      session.Host,
		Port:      session.Port,
		Timestamp: time.Now(),
	})

	delete(sm.sessions, id)
	return nil
}

// GetAuditLog 获取审计日志.
func (sm *SessionManager) GetAuditLog(sessionID string, limit int) []AuditEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []AuditEntry
	for i := len(sm.auditLog) - 1; i >= 0; i-- {
		entry := sm.auditLog[i]
		if sessionID != "" && entry.SessionID != sessionID {
			continue
		}
		result = append(result, entry)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// TestConnection 测试远程主机连通性.
func (sm *SessionManager) TestConnection(host string, port int, protocol Protocol, useTLS bool) (bool, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	var conn net.Conn
	var err error

	if useTLS {
		cfg := sm.tlsCfg
		if cfg == nil {
			cfg = &tls.Config{InsecureSkipVerify: true}
		}
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", addr, cfg)
	} else {
		conn, err = net.DialTimeout("tcp", addr, 5*time.Second)
	}

	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}

// SessionCount 返回当前会话数.
func (sm *SessionManager) SessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// 模块级别 ID 生成（简单递增，测试可预测）.
var idCounter struct {
	mu    sync.Mutex
	value int
}

// generateID 生成唯一 ID.
func generateID() string {
	idCounter.mu.Lock()
	defer idCounter.mu.Unlock()
	idCounter.value++
	return fmt.Sprintf("rdg-%d-%d", time.Now().UnixNano(), idCounter.value)
}
