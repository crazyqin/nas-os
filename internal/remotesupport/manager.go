// Package remotesupport 提供远程支持隧道功能
package remotesupport

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 远程支持隧道管理器.
type Manager struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	sessions   map[string]*Session    // sessionID -> Session
	tokens     map[string]*AccessToken // token -> AccessToken
	tunnels    map[string]*TunnelInfo  // sessionID -> TunnelInfo
	configPath string
}

// RemoteSupportConfig 远程支持配置.
type RemoteSupportConfig struct {
	DefaultBandwidthKB int  `json:"default_bandwidth_kb"` // 默认带宽限制（KB/s）
	DefaultMaxDuration int  `json:"default_max_duration_sec"` // 默认最大持续时间（秒）
	RequireRecording   bool `json:"require_recording"`    // 强制录制
	TokenExpirySec     int  `json:"token_expiry_sec"`     // 令牌过期时间（秒）
}

// NewManager 创建远程支持隧道管理器.
func NewManager(logger *zap.Logger, configPath string) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:     logger,
		sessions:   make(map[string]*Session),
		tokens:     make(map[string]*AccessToken),
		tunnels:    make(map[string]*TunnelInfo),
		configPath: configPath,
	}

	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			logger.Warn("加载远程支持配置失败，使用默认配置", zap.Error(err))
		}
	}

	return m
}

// ========== 会话管理 ==========

// CreateSession 创建远程支持会话.
func (m *Manager) CreateSession(req SessionCreateRequest) (*Session, *AccessToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID := generateID()
	token := generateToken()

	accessLevel := req.AccessLevel
	if accessLevel == "" {
		accessLevel = AccessLevelReadOnly
	}

	bandwidth := req.BandwidthKB
	if bandwidth <= 0 {
		bandwidth = 1024 // 默认 1MB/s
	}

	maxDuration := time.Duration(req.MaxDuration) * time.Second
	if maxDuration <= 0 {
		maxDuration = time.Hour // 默认 1 小时
	}

	now := time.Now()

	session := &Session{
		ID:          sessionID,
		Token:       token,
		Status:      SessionStatusPending,
		AccessLevel: accessLevel,
		ClientName:  req.ClientName,
		TargetHost:  req.TargetHost,
		TargetPort:  req.TargetPort,
		BandwidthKB: bandwidth,
		MaxDuration: maxDuration,
		StartedAt:   now,
		ExpiresAt:   now.Add(maxDuration),
		Recorded:    req.Recorded,
		AuditLog:    make([]AuditEntry, 0),
	}

	accessToken := &AccessToken{
		Token:     token,
		SessionID: sessionID,
		Used:      false,
		ExpiresAt: now.Add(time.Duration(300) * time.Second), // 令牌 5 分钟有效
		CreatedAt: now,
	}

	m.sessions[sessionID] = session
	m.tokens[token] = accessToken

	m.logger.Info("远程支持会话已创建",
		zap.String("session_id", sessionID),
		zap.String("client", req.ClientName),
		zap.String("target", req.TargetHost),
	)

	return session, accessToken, nil
}

// GetSession 获取会话信息.
func (m *Manager) GetSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	return session, nil
}

// ListSessions 列出所有会话.
func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// UpdateSession 更新会话.
func (m *Manager) UpdateSession(sessionID string, req SessionUpdateRequest) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	if req.Status != nil {
		session.Status = *req.Status
		if *req.Status == SessionStatusClosed {
			now := time.Now()
			session.EndedAt = &now
		}
	}
	if req.AccessLevel != nil {
		session.AccessLevel = *req.AccessLevel
	}
	if req.BandwidthKB != nil {
		session.BandwidthKB = *req.BandwidthKB
	}

	return session, nil
}

// CloseSession 关闭会话.
func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	if session.Status == SessionStatusClosed || session.Status == SessionStatusExpired {
		return ErrSessionClosed
	}

	now := time.Now()
	session.Status = SessionStatusClosed
	session.EndedAt = &now

	// 关闭关联隧道
	if _, ok := m.tunnels[sessionID]; ok {
		m.tunnels[sessionID].Status = "closed"
	}

	m.logger.Info("远程支持会话已关闭", zap.String("session_id", sessionID))
	return nil
}

// ========== 令牌管理 ==========

// ValidateToken 验证一次性访问令牌.
func (m *Manager) ValidateToken(req TokenValidateRequest) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	accessToken, exists := m.tokens[req.Token]
	if !exists {
		return nil, ErrTokenInvalid
	}

	if accessToken.Used {
		return nil, ErrTokenUsed
	}

	if time.Now().After(accessToken.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	session, exists := m.sessions[accessToken.SessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	if session.Status == SessionStatusClosed || session.Status == SessionStatusExpired {
		return nil, ErrSessionClosed
	}

	// 标记令牌已使用
	now := time.Now()
	accessToken.Used = true
	accessToken.UsedAt = &now

	// 激活会话
	session.Status = SessionStatusActive
	session.ClientIP = req.ClientIP

	// 记录审计日志
	m.addAuditEntry(session, "token_validated", "令牌验证成功，会话已激活", req.ClientIP)

	m.logger.Info("令牌验证成功",
		zap.String("session_id", session.ID),
		zap.String("client_ip", req.ClientIP),
	)

	return session, nil
}

// ========== 隧道管理 ==========

// EstablishTunnel 建立反向隧道.
func (m *Manager) EstablishTunnel(sessionID string) (*TunnelInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	if session.Status != SessionStatusActive {
		return nil, ErrSessionClosed
	}

	tunnel := &TunnelInfo{
		SessionID:  sessionID,
		LocalAddr:  fmt.Sprintf("0.0.0.0:%d", 22000+randomPort()),
		RemoteAddr: fmt.Sprintf("%s:%d", session.TargetHost, session.TargetPort),
		Status:     "active",
		CreatedAt:  time.Now(),
	}

	m.tunnels[sessionID] = tunnel

	m.addAuditEntry(session, "tunnel_established",
		fmt.Sprintf("隧道已建立: %s -> %s", tunnel.LocalAddr, tunnel.RemoteAddr), "")

	m.logger.Info("隧道已建立",
		zap.String("session_id", sessionID),
		zap.String("local", tunnel.LocalAddr),
		zap.String("remote", tunnel.RemoteAddr),
	)

	return tunnel, nil
}

// CloseTunnel 关闭隧道.
func (m *Manager) CloseTunnel(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, exists := m.tunnels[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	tunnel.Status = "closed"

	if session, ok := m.sessions[sessionID]; ok {
		m.addAuditEntry(session, "tunnel_closed", "隧道已关闭", "")
	}

	m.logger.Info("隧道已关闭", zap.String("session_id", sessionID))
	return nil
}

// GetTunnel 获取隧道信息.
func (m *Manager) GetTunnel(sessionID string) (*TunnelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnel, exists := m.tunnels[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	return tunnel, nil
}

// ========== 带宽控制 ==========

// CheckBandwidth 检查带宽是否超限.
func (m *Manager) CheckBandwidth(sessionID string, bytesTransfer int64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	// 计算当前速率（简化：基于总传输量和会话时长）
	duration := time.Since(session.StartedAt).Seconds()
	if duration <= 0 {
		return nil
	}

	totalBytes := session.BytesUp + session.BytesDown + bytesTransfer
	rateKBps := float64(totalBytes) / 1024.0 / duration

	if rateKBps > float64(session.BandwidthKB) {
		return ErrBandwidthLimit
	}

	return nil
}

// RecordTransfer 记录传输量.
func (m *Manager) RecordTransfer(sessionID string, bytesUp, bytesDown int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	session.BytesUp += bytesUp
	session.BytesDown += bytesDown
	return nil
}

// ========== 审计日志 ==========

// AddAuditEntry 添加审计日志条目.
func (m *Manager) AddAuditEntry(sessionID, action, detail, source string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	m.addAuditEntry(session, action, detail, source)
	return nil
}

// GetAuditLog 获取会话审计日志.
func (m *Manager) GetAuditLog(sessionID string) ([]AuditEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	return session.AuditLog, nil
}

// addAuditEntry 内部添加审计日志（需持有锁）.
func (m *Manager) addAuditEntry(session *Session, action, detail, source string) {
	entry := AuditEntry{
		Timestamp: time.Now(),
		Action:    action,
		Detail:    detail,
		Source:    source,
	}
	session.AuditLog = append(session.AuditLog, entry)
}

// ========== 统计 ==========

// GetStats 获取远程支持统计.
func (m *Manager) GetStats() *SessionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SessionStats{
		TotalSessions: len(m.sessions),
	}

	for _, s := range m.sessions {
		if s.Status == SessionStatusActive {
			stats.ActiveSessions++
		}
		stats.TotalBytesUp += s.BytesUp
		stats.TotalBytesDown += s.BytesDown
		stats.TotalAuditEntries += len(s.AuditLog)
	}

	return stats
}

// ========== 内部方法 ==========

func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config RemoteSupportConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	return nil
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomPort() int {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return int(b[0])<<8 | int(b[1])
}
