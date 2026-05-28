// Package webterminal 提供 Web 终端（WebSocket SSH）功能
// Version: v2.0.0 - 完整 Web 终端管理器
package webterminal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// TerminalSession 终端会话
type TerminalSession struct {
	ID         string    `json:"id"`
	User       string    `json:"user"`
	TabID      string    `json:"tabId"`      // 多标签页支持
	StartedAt  time.Time `json:"startedAt"`
	LastActive time.Time `json:"lastActive"`
	Status     string    `json:"status"`     // active, closed, recording
	Remote     string    `json:"remote"`
	Cols       int       `json:"cols"`
	Rows       int       `json:"rows"`
	AuthToken  string    `json:"-"`          // 认证令牌
	CommandHistory []CommandRecord `json:"commandHistory"` // 命令历史
	Recording  *Recording `json:"recording,omitempty"` // 录制状态
}

// CommandRecord 命令记录
type CommandRecord struct {
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
	Output    string    `json:"output"`
	ExitCode  int       `json:"exitCode"`
}

// Recording 终端录制
type Recording struct {
	ID        string          `json:"id"`
	StartTime time.Time       `json:"startTime"`
	EndTime   *time.Time      `json:"endTime,omitempty"`
	Events    []RecordingEvent `json:"events"`
}

// RecordingEvent 录制事件
type RecordingEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // input, output, resize
	Data      []byte    `json:"data"`
}

// TerminalConfig 终端配置
type TerminalConfig struct {
	MaxSessions      int           `json:"maxSessions"`
	MaxTabsPerUser   int           `json:"maxTabsPerUser"`
	IdleTimeout      time.Duration `json:"idleTimeout"`
	AllowRoot        bool          `json:"allowRoot"`
	DefaultShell     string        `json:"defaultShell"`
	AllowedUsers     []string      `json:"allowedUsers"`
	CommandWhitelist []string      `json:"commandWhitelist"`
	CommandBlacklist []string      `json:"commandBlacklist"`
	EnableRecording  bool          `json:"enableRecording"`
	EnableHistory    bool          `json:"enableHistory"`
	HistoryMaxSize   int           `json:"historyMaxSize"`
	BufferSize       int           `json:"bufferSize"`
}

// OutputBuffer 终端输出缓冲
type OutputBuffer struct {
	mu      sync.RWMutex
	data    []byte
	maxSize int
}

// NewOutputBuffer 创建输出缓冲
func NewOutputBuffer(maxSize int) *OutputBuffer {
	return &OutputBuffer{
		data:    make([]byte, 0, maxSize),
		maxSize: maxSize,
	}
}

// Write 写入缓冲
func (b *OutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 如果缓冲已满，移除旧数据
	if len(b.data)+len(p) > b.maxSize {
		overflow := len(b.data) + len(p) - b.maxSize
		b.data = b.data[overflow:]
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

// Read 读取缓冲
func (b *OutputBuffer) Read() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]byte, len(b.data))
	copy(result, b.data)
	return result
}

// Clear 清空缓冲
func (b *OutputBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = b.data[:0]
}

// Manager 终端管理器
type Manager struct {
	mu        sync.RWMutex
	sessions  map[string]*TerminalSession
	tabs      map[string]map[string]*TerminalSession // user -> tabID -> session
	config    TerminalConfig
	authFunc  func(r *http.Request) (string, error) // 认证函数
	stopCh    chan struct{}
}

// NewManager 创建终端管理器
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*TerminalSession),
		tabs:     make(map[string]map[string]*TerminalSession),
		config: TerminalConfig{
			MaxSessions:    10,
			MaxTabsPerUser: 5,
			IdleTimeout:    30 * time.Minute,
			DefaultShell:   "/bin/bash",
			EnableRecording: true,
			EnableHistory:  true,
			HistoryMaxSize: 1000,
			BufferSize:     1024 * 1024, // 1MB
		},
		stopCh: make(chan struct{}),
	}
}

// SetAuthFunc 设置认证函数
func (m *Manager) SetAuthFunc(authFunc func(r *http.Request) (string, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authFunc = authFunc
}

// Authenticate 认证请求
func (m *Manager) Authenticate(r *http.Request) (string, error) {
	m.mu.RLock()
	authFunc := m.authFunc
	m.mu.RUnlock()

	if authFunc == nil {
		// 默认认证：从 header 获取用户
		user := r.Header.Get("X-User")
		if user == "" {
			return "", fmt.Errorf("未提供用户信息")
		}
		return user, nil
	}
	return authFunc(r)
}

// CheckAuthorization 检查用户授权
func (m *Manager) CheckAuthorization(user string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查允许的用户列表
	if len(m.config.AllowedUsers) > 0 {
		allowed := false
		for _, u := range m.config.AllowedUsers {
			if u == user {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("用户 %s 未授权", user)
		}
	}

	return nil
}

// CheckCommandAllowed 检查命令是否允许执行
func (m *Manager) CheckCommandAllowed(command string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查黑名单
	for _, blocked := range m.config.CommandBlacklist {
		if matched, _ := matchPattern(command, blocked); matched {
			return fmt.Errorf("命令被禁止: %s", blocked)
		}
	}

	// 如果有白名单，检查白名单
	if len(m.config.CommandWhitelist) > 0 {
		allowed := false
		for _, pattern := range m.config.CommandWhitelist {
			if matched, _ := matchPattern(command, pattern); matched {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("命令不在白名单中")
		}
	}

	return nil
}

// matchPattern 简单模式匹配
func matchPattern(s, pattern string) (bool, error) {
	// 简单实现：支持 * 通配符
	if pattern == "*" {
		return true, nil
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(s) >= len(prefix) && s[:len(prefix)] == prefix, nil
	}
	return s == pattern, nil
}

// ListSessions 列出所有活跃会话
func (m *Manager) ListSessions() []TerminalSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []TerminalSession
	for _, s := range m.sessions {
		if s.Status == "active" || s.Status == "recording" {
			sessions = append(sessions, *s)
		}
	}
	return sessions
}

// ListUserSessions 列出用户的会话
func (m *Manager) ListUserSessions(user string) []TerminalSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []TerminalSession
	for _, s := range m.sessions {
		if s.User == user && (s.Status == "active" || s.Status == "recording") {
			sessions = append(sessions, *s)
		}
	}
	return sessions
}

// GetUserTabCount 获取用户的标签页数量
func (m *Manager) GetUserTabCount(user string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if userTabs, ok := m.tabs[user]; ok {
		count := 0
		for _, s := range userTabs {
			if s.Status == "active" || s.Status == "recording" {
				count++
			}
		}
		return count
	}
	return 0
}

// CreateSession 创建新会话
func (m *Manager) CreateSession(user, tabID, remote string) (*TerminalSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查总会话数限制
	activeCount := 0
	for _, s := range m.sessions {
		if s.Status == "active" || s.Status == "recording" {
			activeCount++
		}
	}
	if activeCount >= m.config.MaxSessions {
		return nil, fmt.Errorf("终端会话数已达上限 (%d)", m.config.MaxSessions)
	}

	// 检查用户标签页限制
	if userTabs, ok := m.tabs[user]; ok {
		userTabCount := 0
		for _, s := range userTabs {
			if s.Status == "active" || s.Status == "recording" {
				userTabCount++
			}
		}
		if userTabCount >= m.config.MaxTabsPerUser {
			return nil, fmt.Errorf("用户 %s 的标签页数已达上限 (%d)", user, m.config.MaxTabsPerUser)
		}
	}

	// 创建会话
	sessionID := fmt.Sprintf("term-%d-%s", time.Now().UnixNano(), user)
	session := &TerminalSession{
		ID:         sessionID,
		User:       user,
		TabID:      tabID,
		StartedAt:  time.Now(),
		LastActive: time.Now(),
		Status:     "active",
		Remote:     remote,
		Cols:       80,
		Rows:       24,
		AuthToken:  fmt.Sprintf("auth-%d", time.Now().UnixNano()),
	}

	m.sessions[sessionID] = session

	// 更新标签页映射
	if m.tabs[user] == nil {
		m.tabs[user] = make(map[string]*TerminalSession)
	}
	m.tabs[user][tabID] = session

	return session, nil
}

// GetSession 获取会话信息
func (m *Manager) GetSession(id string) (*TerminalSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在", id)
	}
	return session, nil
}

// GetSessionByTab 根据标签页获取会话
func (m *Manager) GetSessionByTab(user, tabID string) (*TerminalSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if userTabs, ok := m.tabs[user]; ok {
		if session, ok := userTabs[tabID]; ok {
			return session, nil
		}
	}
	return nil, fmt.Errorf("用户 %s 的标签页 %s 不存在", user, tabID)
}

// UpdateSessionActivity 更新会话活动时间
func (m *Manager) UpdateSessionActivity(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}
	session.LastActive = time.Now()
	return nil
}

// CloseSession 关闭终端会话
func (m *Manager) CloseSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}

	// 如果正在录制，停止录制
	if session.Recording != nil && session.Recording.EndTime == nil {
		now := time.Now()
		session.Recording.EndTime = &now
	}

	session.Status = "closed"

	// 从标签页映射中移除
	if userTabs, ok := m.tabs[session.User]; ok {
		delete(userTabs, session.TabID)
	}

	return nil
}

// GetConfig 获取终端配置
func (m *Manager) GetConfig() TerminalConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新终端配置
func (m *Manager) UpdateConfig(config TerminalConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// AddCommandHistory 添加命令历史
func (m *Manager) AddCommandHistory(sessionID string, record CommandRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}

	if !m.config.EnableHistory {
		return nil
	}

	session.CommandHistory = append(session.CommandHistory, record)

	// 限制历史记录大小
	if len(session.CommandHistory) > m.config.HistoryMaxSize {
		session.CommandHistory = session.CommandHistory[len(session.CommandHistory)-m.config.HistoryMaxSize:]
	}

	return nil
}

// GetCommandHistory 获取命令历史
func (m *Manager) GetCommandHistory(sessionID string) ([]CommandRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}

	result := make([]CommandRecord, len(session.CommandHistory))
	copy(result, session.CommandHistory)
	return result, nil
}

// StartRecording 开始录制
func (m *Manager) StartRecording(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}

	if !m.config.EnableRecording {
		return fmt.Errorf("录制功能未启用")
	}

	if session.Recording != nil && session.Recording.EndTime == nil {
		return fmt.Errorf("会话 %s 正在录制中", sessionID)
	}

	session.Recording = &Recording{
		ID:        fmt.Sprintf("rec-%d", time.Now().UnixNano()),
		StartTime: time.Now(),
		Events:    make([]RecordingEvent, 0),
	}
	session.Status = "recording"

	return nil
}

// StopRecording 停止录制
func (m *Manager) StopRecording(sessionID string) (*Recording, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}

	if session.Recording == nil || session.Recording.EndTime != nil {
		return nil, fmt.Errorf("会话 %s 没有在录制", sessionID)
	}

	now := time.Now()
	session.Recording.EndTime = &now
	session.Status = "active"

	return session.Recording, nil
}

// AddRecordingEvent 添加录制事件
func (m *Manager) AddRecordingEvent(sessionID string, event RecordingEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}

	if session.Recording == nil || session.Recording.EndTime != nil {
		return nil // 不在录制中，忽略
	}

	session.Recording.Events = append(session.Recording.Events, event)
	return nil
}

// GetRecording 获取录制内容
func (m *Manager) GetRecording(sessionID string) (*Recording, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}

	if session.Recording == nil {
		return nil, fmt.Errorf("会话 %s 没有录制内容", sessionID)
	}

	return session.Recording, nil
}

// CleanupIdle 清理空闲会话
func (m *Manager) CleanupIdle() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cleaned := 0
	now := time.Now()
	for id, s := range m.sessions {
		if (s.Status == "active" || s.Status == "recording") && now.Sub(s.LastActive) > m.config.IdleTimeout {
			// 如果正在录制，停止录制
			if s.Recording != nil && s.Recording.EndTime == nil {
				s.Recording.EndTime = &now
			}

			s.Status = "closed"
			cleaned++

			// 从标签页映射中移除
			if userTabs, ok := m.tabs[s.User]; ok {
				delete(userTabs, s.TabID)
			}

			_ = id // 避免未使用变量警告
		}
	}
	return cleaned
}

// ActiveCount 活跃会话数
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, s := range m.sessions {
		if s.Status == "active" || s.Status == "recording" {
			count++
		}
	}
	return count
}

// GetUserStats 获取用户统计
func (m *Manager) GetUserStats(user string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"user":        user,
		"activeTabs":  0,
		"totalTabs":   0,
		"totalCommands": 0,
	}

	if userTabs, ok := m.tabs[user]; ok {
		for _, s := range userTabs {
			stats["totalTabs"] = stats["totalTabs"].(int) + 1
			if s.Status == "active" || s.Status == "recording" {
				stats["activeTabs"] = stats["activeTabs"].(int) + 1
			}
			stats["totalCommands"] = stats["totalCommands"].(int) + len(s.CommandHistory)
		}
	}

	return stats
}

// Cleanup 清理所有资源
func (m *Manager) Cleanup() {
	close(m.stopCh)
}

// ValidateSessionToken 验证会话令牌
func (m *Manager) ValidateSessionToken(sessionID, token string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return false
	}
	return session.AuthToken == token
}

// UpdateSessionSize 更新终端大小
func (m *Manager) UpdateSessionSize(sessionID string, cols, rows int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}

	session.Cols = cols
	session.Rows = rows
	return nil
}

// ToJSON 序列化为 JSON
func (s *TerminalSession) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// GetDefaultShell 获取默认 Shell
func (m *Manager) GetDefaultShell() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	shell := m.config.DefaultShell
	if shell == "" {
		shell = "/bin/bash"
		if _, err := os.Stat(shell); err != nil {
			shell = "/bin/sh"
		}
	}
	return shell
}

// CreateShellCommand 创建 Shell 命令
func (m *Manager) CreateShellCommand(session *TerminalSession) (*exec.Cmd, error) {
	shell := m.GetDefaultShell()

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TERM=xterm-256color"),
		fmt.Sprintf("COLUMNS=%d", session.Cols),
		fmt.Sprintf("LINES=%d", session.Rows),
		fmt.Sprintf("USER=%s", session.User),
	)

	return cmd, nil
}

// Stats 管理器统计
type Stats struct {
	TotalSessions  int `json:"totalSessions"`
	ActiveSessions int `json:"activeSessions"`
	TotalUsers     int `json:"totalUsers"`
	TotalCommands  int `json:"totalCommands"`
	TotalRecordings int `json:"totalRecordings"`
}

// GetStats 获取管理器统计
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := Stats{
		TotalSessions: len(m.sessions),
	}

	users := make(map[string]bool)
	for _, s := range m.sessions {
		users[s.User] = true
		stats.TotalCommands += len(s.CommandHistory)

		if s.Status == "active" || s.Status == "recording" {
			stats.ActiveSessions++
		}

		if s.Recording != nil {
			stats.TotalRecordings++
		}
	}
	stats.TotalUsers = len(users)

	return stats
}
