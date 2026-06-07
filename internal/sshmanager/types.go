package sshmanager

import (
	"sync"
	"time"
)

// SessionStatus 会话状态.
type SessionStatus string

const (
	StatusActive SessionStatus = "active"
	StatusClosed SessionStatus = "closed"
	StatusError  SessionStatus = "error"
	StatusIdle   SessionStatus = "idle"
)

// SSHKey SSH 密钥.
type SSHKey struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"` // rsa, ed25519, ecdsa
	Fingerprint string     `json:"fingerprint"`
	PublicKey   string     `json:"public_key"`
	PrivateKey  string     `json:"private_key,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
}

// SSHSession SSH 会话.
type SSHSession struct {
	ID         string        `json:"id"`
	Host       string        `json:"host"`
	Port       int           `json:"port"`
	User       string        `json:"user"`
	KeyID      string        `json:"key_id"`
	Status     SessionStatus `json:"status"`
	StartedAt  time.Time     `json:"started_at"`
	LastActive time.Time     `json:"last_active"`
	ClosedAt   *time.Time    `json:"closed_at,omitempty"`
	BytesSent  int64         `json:"bytes_sent"`
	BytesRecv  int64         `json:"bytes_recv"`
	// Command 最后执行的命令.
	Command string `json:"command,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Tunnel SSH 隧道.
type Tunnel struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	SessionID     string    `json:"session_id"`
	LocalAddr     string    `json:"local_addr"`
	RemoteAddr    string    `json:"remote_addr"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	BytesTunneled int64     `json:"bytes_tunneled"`
}

// SSHConfig SSH 管理器配置.
type SSHConfig struct {
	DefaultPort     int  `json:"default_port"`
	MaxSessions     int  `json:"max_sessions"`
	SessionTimeoutS int  `json:"session_timeout_s"`
	AllowAgentFwd   bool `json:"allow_agent_forwarding"`
	AllowTCPFwd     bool `json:"allow_tcp_forwarding"`
	AllowPortFwd    bool `json:"allow_port_forwarding"`
}

// Manager SSH 管理器.
type Manager struct {
	mu       sync.RWMutex
	config   *SSHConfig
	keys     map[string]*SSHKey
	sessions map[string]*SSHSession
	tunnels  map[string]*Tunnel
}

// NewManager 创建 SSH 管理器.
func NewManager() *Manager {
	return &Manager{
		config: &SSHConfig{
			DefaultPort:     22,
			MaxSessions:     100,
			SessionTimeoutS: 3600,
			AllowAgentFwd:   true,
			AllowTCPFwd:     true,
			AllowPortFwd:    true,
		},
		keys:     make(map[string]*SSHKey),
		sessions: make(map[string]*SSHSession),
		tunnels:  make(map[string]*Tunnel),
	}
}

// AddKey 添加密钥.
func (m *Manager) AddKey(key *SSHKey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key.CreatedAt = time.Now()
	m.keys[key.ID] = key
}

// GetKey 获取密钥.
func (m *Manager) GetKey(id string) (*SSHKey, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k, ok := m.keys[id]
	return k, ok
}

// ListKeys 列出密钥.
func (m *Manager) ListKeys() []*SSHKey {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]*SSHKey, 0, len(m.keys))
	for _, k := range m.keys {
		keys = append(keys, k)
	}
	return keys
}

// DeleteKey 删除密钥.
func (m *Manager) DeleteKey(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.keys[id]; !ok {
		return false
	}
	delete(m.keys, id)
	return true
}

// CreateSession 创建会话.
func (m *Manager) CreateSession(host string, port int, user, keyID string) (*SSHSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.config.MaxSessions {
		return nil, ErrMaxSessionsReached
	}

	now := time.Now()
	sess := &SSHSession{
		ID:         generateID(),
		Host:       host,
		Port:       port,
		User:       user,
		KeyID:      keyID,
		Status:     StatusActive,
		StartedAt:  now,
		LastActive: now,
	}
	m.sessions[sess.ID] = sess
	return sess, nil
}

// CloseSession 关闭会话.
func (m *Manager) CloseSession(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[id]
	if !ok {
		return false
	}
	now := time.Now()
	sess.Status = StatusClosed
	sess.ClosedAt = &now
	return true
}

// GetSession 获取会话.
func (m *Manager) GetSession(id string) (*SSHSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// ListSessions 列出会话.
func (m *Manager) ListSessions(activeOnly bool) []*SSHSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessions := make([]*SSHSession, 0)
	for _, s := range m.sessions {
		if activeOnly && s.Status != StatusActive {
			continue
		}
		sessions = append(sessions, s)
	}
	return sessions
}

// CreateTunnel 创建隧道.
func (m *Manager) CreateTunnel(name, sessionID, localAddr, remoteAddr string) *Tunnel {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := &Tunnel{
		ID:         generateID(),
		Name:       name,
		SessionID:  sessionID,
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
		Enabled:    true,
		CreatedAt:  time.Now(),
	}
	m.tunnels[t.ID] = t
	return t
}

// ListTunnels 列出隧道.
func (m *Manager) ListTunnels() []*Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tunnels := make([]*Tunnel, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		tunnels = append(tunnels, t)
	}
	return tunnels
}

// DeleteTunnel 删除隧道.
func (m *Manager) DeleteTunnel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tunnels[id]; !ok {
		return false
	}
	delete(m.tunnels, id)
	return true
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *SSHConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *SSHConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}

// GetStats 获取统计.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	active := 0
	for _, s := range m.sessions {
		if s.Status == StatusActive {
			active++
		}
	}
	return map[string]interface{}{
		"total_keys":      len(m.keys),
		"total_sessions":  len(m.sessions),
		"active_sessions": active,
		"total_tunnels":   len(m.tunnels),
	}
}

var idCounter int64

func generateID() string {
	idCounter++
	return time.Now().Format("20060102150405") + "-" + string(rune('A'+idCounter%26))
}
