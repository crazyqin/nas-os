// Package session 提供SMB/NFS会话监控和管理功能
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionType 会话类型
type SessionType string

const (
	SessionTypeSMB SessionType = "smb"
	SessionTypeNFS SessionType = "nfs"
)

// SessionStatus 会话状态
type SessionStatus string

const (
	StatusActive SessionStatus = "active"
	StatusIdle   SessionStatus = "idle"
	StatusStale  SessionStatus = "stale"
	StatusClosed SessionStatus = "closed"
	StatusKicked SessionStatus = "kicked"
)

// Session 会话信息
type Session struct {
	ID           string        `json:"id"`
	Type         SessionType   `json:"type"`
	User         string        `json:"user"`
	ClientIP     string        `json:"client_ip"`
	ClientName   string        `json:"client_name,omitempty"`
	SharePath    string        `json:"share_path"`
	ShareName    string        `json:"share_name,omitempty"`
	Status       SessionStatus `json:"status"`
	ConnectedAt  time.Time     `json:"connected_at"`
	LastActiveAt time.Time     `json:"last_active_at"`
	BytesRead    int64         `json:"bytes_read"`
	BytesWritten int64         `json:"bytes_written"`
	FilesOpen    int           `json:"files_open"`
	LockedFiles  []string      `json:"locked_files,omitempty"`
	Protocol     string        `json:"protocol,omitempty"`
	Encryption   string        `json:"encryption,omitempty"`
	PID          int           `json:"pid,omitempty"`
}

// SessionStats 会话统计信息
type SessionStats struct {
	TotalSessions      int            `json:"total_sessions"`
	ActiveSessions     int            `json:"active_sessions"`
	SMBSessions        int            `json:"smb_sessions"`
	NFSSessions        int            `json:"nfs_sessions"`
	TotalBytesRead     int64          `json:"total_bytes_read"`
	TotalBytesWritten  int64          `json:"total_bytes_written"`
	TotalFilesOpen     int            `json:"total_files_open"`
	UniqueUsers        int            `json:"unique_users"`
	UniqueClients      int            `json:"unique_clients"`
	SessionsByUser     map[string]int `json:"sessions_by_user,omitempty"`
	SessionsByClient   map[string]int `json:"sessions_by_client,omitempty"`
	SessionsByShare    map[string]int `json:"sessions_by_share,omitempty"`
	LastUpdated        time.Time      `json:"last_updated"`
	AvgSessionDuration time.Duration  `json:"avg_session_duration"`
}

// SessionEvent 会话事件
type SessionEvent struct {
	Type      string    `json:"type"` // connect, disconnect, activity, kick
	SessionID string    `json:"session_id"`
	User      string    `json:"user"`
	ClientIP  string    `json:"client_ip"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
}

// Config 会话管理配置
type Config struct {
	Enabled          bool          `json:"enabled"`
	RefreshInterval  time.Duration `json:"refresh_interval"`
	IdleTimeout      time.Duration `json:"idle_timeout"`
	StaleTimeout     time.Duration `json:"stale_timeout"`
	MaxSessions      int           `json:"max_sessions"`
	HistoryRetention time.Duration `json:"history_retention"`
}

// Manager 会话管理器
type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	config      *Config
	configPath  string
	events      []SessionEvent
	eventMu     sync.RWMutex
	eventBuffer int
	onEvent     func(SessionEvent)
}

// defaultConfig 默认配置
func defaultConfig() *Config {
	return &Config{
		Enabled:          true,
		RefreshInterval:  10 * time.Second,
		IdleTimeout:      30 * time.Minute,
		StaleTimeout:     60 * time.Minute,
		MaxSessions:      10000,
		HistoryRetention: 24 * time.Hour,
	}
}

// NewManager 创建会话管理器
func NewManager(configPath string) (*Manager, error) {
	m := &Manager{
		sessions:    make(map[string]*Session),
		config:      defaultConfig(),
		configPath:  configPath,
		events:      make([]SessionEvent, 0, 1000),
		eventBuffer: 1000,
	}

	if err := m.loadConfig(); err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	return m, nil
}

// loadConfig 加载配置
func (m *Manager) loadConfig() error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := json.Unmarshal(data, m.config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	return nil
}

// SaveConfig 保存配置
func (m *Manager) SaveConfig() error {
	m.mu.RLock()
	config := m.config
	m.mu.RUnlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0640); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *Config) error {
	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	return m.SaveConfig()
}

// AddSession 添加会话
func (m *Manager) AddSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("会话不能为空")
	}

	if session.ID == "" {
		return fmt.Errorf("会话ID不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否超过最大会话数
	if m.config.MaxSessions > 0 && len(m.sessions) >= m.config.MaxSessions {
		// 移除最旧的空闲会话
		m.removeOldestIdleSessionLocked()
	}

	// 设置默认值
	if session.Status == "" {
		session.Status = StatusActive
	}
	if session.ConnectedAt.IsZero() {
		session.ConnectedAt = time.Now()
	}
	if session.LastActiveAt.IsZero() {
		session.LastActiveAt = time.Now()
	}

	m.sessions[session.ID] = session

	// 记录事件
	m.recordEvent("connect", session.ID, session.User, session.ClientIP, "新会话建立")

	return nil
}

// UpdateSession 更新会话
func (m *Manager) UpdateSession(id string, updates *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("会话不存在: %s", id)
	}

	// 更新字段
	if updates.User != "" {
		session.User = updates.User
	}
	if updates.ClientIP != "" {
		session.ClientIP = updates.ClientIP
	}
	if updates.ClientName != "" {
		session.ClientName = updates.ClientName
	}
	if updates.Status != "" {
		session.Status = updates.Status
	}
	if updates.BytesRead > 0 {
		session.BytesRead = updates.BytesRead
	}
	if updates.BytesWritten > 0 {
		session.BytesWritten = updates.BytesWritten
	}
	if updates.FilesOpen > 0 {
		session.FilesOpen = updates.FilesOpen
	}
	if updates.LockedFiles != nil {
		session.LockedFiles = updates.LockedFiles
	}

	session.LastActiveAt = time.Now()

	return nil
}

// RemoveSession 移除会话
func (m *Manager) RemoveSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("会话不存在: %s", id)
	}

	// 记录事件
	m.recordEvent("disconnect", id, session.User, session.ClientIP, "会话断开")

	delete(m.sessions, id)

	return nil
}

// GetSession 获取会话
func (m *Manager) GetSession(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", id)
	}

	return session, nil
}

// ListSessions 列出所有会话
func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}

	return sessions
}

// ListSessionsByType 按类型列出会话
func (m *Manager) ListSessionsByType(sessionType SessionType) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*Session
	for _, s := range m.sessions {
		if s.Type == sessionType {
			sessions = append(sessions, s)
		}
	}

	return sessions
}

// ListSessionsByUser 按用户列出会话
func (m *Manager) ListSessionsByUser(user string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*Session
	for _, s := range m.sessions {
		if s.User == user {
			sessions = append(sessions, s)
		}
	}

	return sessions
}

// ListSessionsByClient 按客户端IP列出会话
func (m *Manager) ListSessionsByClient(clientIP string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*Session
	for _, s := range m.sessions {
		if s.ClientIP == clientIP {
			sessions = append(sessions, s)
		}
	}

	return sessions
}

// KickSession 强制断开会话
func (m *Manager) KickSession(id, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("会话不存在: %s", id)
	}

	// 更新状态
	session.Status = StatusKicked

	// 记录事件
	m.recordEvent("kick", id, session.User, session.ClientIP, reason)

	// 从管理器移除
	delete(m.sessions, id)

	return nil
}

// KickSessionsByUser 强制断开用户所有会话
func (m *Manager) KickSessionsByUser(user, reason string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, session := range m.sessions {
		if session.User == user {
			session.Status = StatusKicked
			m.recordEvent("kick", id, session.User, session.ClientIP, reason)
			delete(m.sessions, id)
			count++
		}
	}

	return count, nil
}

// KickSessionsByClient 强制断开客户端所有会话
func (m *Manager) KickSessionsByClient(clientIP, reason string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, session := range m.sessions {
		if session.ClientIP == clientIP {
			session.Status = StatusKicked
			m.recordEvent("kick", id, session.User, session.ClientIP, reason)
			delete(m.sessions, id)
			count++
		}
	}

	return count, nil
}

// GetStats 获取会话统计
func (m *Manager) GetStats() *SessionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SessionStats{
		TotalSessions:    len(m.sessions),
		SessionsByUser:   make(map[string]int),
		SessionsByClient: make(map[string]int),
		SessionsByShare:  make(map[string]int),
		LastUpdated:      time.Now(),
	}

	var totalDuration time.Duration
	users := make(map[string]struct{})
	clients := make(map[string]struct{})

	for _, session := range m.sessions {
		// 统计活跃会话
		if session.Status == StatusActive {
			stats.ActiveSessions++
		}

		// 按类型统计
		switch session.Type {
		case SessionTypeSMB:
			stats.SMBSessions++
		case SessionTypeNFS:
			stats.NFSSessions++
		}

		// 统计传输量
		stats.TotalBytesRead += session.BytesRead
		stats.TotalBytesWritten += session.BytesWritten
		stats.TotalFilesOpen += session.FilesOpen

		// 统计唯一用户和客户端
		users[session.User] = struct{}{}
		clients[session.ClientIP] = struct{}{}

		// 按用户/客户端/共享统计
		stats.SessionsByUser[session.User]++
		stats.SessionsByClient[session.ClientIP]++
		stats.SessionsByShare[session.SharePath]++

		// 计算平均持续时间
		duration := time.Since(session.ConnectedAt)
		totalDuration += duration
	}

	stats.UniqueUsers = len(users)
	stats.UniqueClients = len(clients)

	if stats.TotalSessions > 0 {
		stats.AvgSessionDuration = totalDuration / time.Duration(stats.TotalSessions)
	}

	return stats
}

// UpdateStats 更新会话统计（从外部数据源更新）
func (m *Manager) UpdateStats(bytesRead, bytesWritten map[string]int64, filesOpen map[string]int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, br := range bytesRead {
		if session, exists := m.sessions[id]; exists {
			session.BytesRead = br
		}
	}

	for id, bw := range bytesWritten {
		if session, exists := m.sessions[id]; exists {
			session.BytesWritten = bw
		}
	}

	for id, fo := range filesOpen {
		if session, exists := m.sessions[id]; exists {
			session.FilesOpen = fo
		}
	}
}

// MarkIdle 标记空闲会话
func (m *Manager) MarkIdle() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0

	for _, session := range m.sessions {
		if session.Status == StatusActive {
			idleTime := now.Sub(session.LastActiveAt)
			if idleTime > m.config.IdleTimeout {
				session.Status = StatusIdle
				count++
			}
		}
	}

	return count
}

// CleanupStale 清理过期会话
func (m *Manager) CleanupStale() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0

	for id, session := range m.sessions {
		idleTime := now.Sub(session.LastActiveAt)
		if idleTime > m.config.StaleTimeout {
			session.Status = StatusStale
			m.recordEvent("disconnect", id, session.User, session.ClientIP, "会话超时")
			delete(m.sessions, id)
			count++
		}
	}

	return count
}

// GetEvents 获取事件历史
func (m *Manager) GetEvents(limit int) []SessionEvent {
	m.eventMu.RLock()
	defer m.eventMu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	// 返回最近的事件
	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}

	result := make([]SessionEvent, limit)
	copy(result, m.events[start:])

	return result
}

// SetEventHandler 设置事件处理器
func (m *Manager) SetEventHandler(handler func(SessionEvent)) {
	m.onEvent = handler
}

// recordEvent 记录事件
func (m *Manager) recordEvent(eventType, sessionID, user, clientIP, message string) {
	event := SessionEvent{
		Type:      eventType,
		SessionID: sessionID,
		User:      user,
		ClientIP:  clientIP,
		Timestamp: time.Now(),
		Message:   message,
	}

	m.eventMu.Lock()
	m.events = append(m.events, event)

	// 限制事件缓冲区大小
	if len(m.events) > m.eventBuffer {
		m.events = m.events[len(m.events)-m.eventBuffer:]
	}
	m.eventMu.Unlock()

	// 调用事件处理器
	if m.onEvent != nil {
		go m.onEvent(event)
	}
}

// removeOldestIdleSessionLocked 移除最旧的空闲会话（已持有锁）
func (m *Manager) removeOldestIdleSessionLocked() {
	var oldestID string
	var oldestTime time.Time

	for id, session := range m.sessions {
		if session.Status == StatusIdle || session.Status == StatusStale {
			if oldestID == "" || session.LastActiveAt.Before(oldestTime) {
				oldestID = id
				oldestTime = session.LastActiveAt
			}
		}
	}

	if oldestID != "" {
		delete(m.sessions, oldestID)
	}
}

// SyncSessions 同步会话（用新数据替换）
func (m *Manager) SyncSessions(sessions []*Session) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 记录旧会话ID
	oldIDs := make(map[string]struct{})
	for id := range m.sessions {
		oldIDs[id] = struct{}{}
	}

	// 构建新会话映射
	newSessions := make(map[string]*Session)
	for _, s := range sessions {
		if s.ID != "" {
			newSessions[s.ID] = s
			delete(oldIDs, s.ID)
		}
	}

	// 记录断开的会话
	for id := range oldIDs {
		if session, exists := m.sessions[id]; exists {
			m.recordEvent("disconnect", id, session.User, session.ClientIP, "会话同步断开")
		}
	}

	// 替换会话列表
	m.sessions = newSessions
}

// Count 获取会话数
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// CountByType 按类型统计会话数
func (m *Manager) CountByType(sessionType SessionType) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, s := range m.sessions {
		if s.Type == sessionType {
			count++
		}
	}

	return count
}
