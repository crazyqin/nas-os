package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Session 会话信息
type Session struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	LastAccessAt time.Time `json:"last_access_at"`
	IPAddress    string    `json:"ip_address,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
	DeviceID     string    `json:"device_id,omitempty"`
	MFAVerified  bool      `json:"mfa_verified"`
	Roles        []string  `json:"roles,omitempty"`
	Groups       []string  `json:"groups,omitempty"`
}

// SessionConfig 会话配置
type SessionConfig struct {
	TokenExpiry        time.Duration `json:"token_expiry"`          // 令牌有效期
	RefreshTokenExpiry time.Duration `json:"refresh_token_expiry"`  // 刷新令牌有效期
	MaxSessionsPerUser int           `json:"max_sessions_per_user"` // 每用户最大会话数
	EnableRefreshToken bool          `json:"enable_refresh_token"`  // 启用刷新令牌
	SessionFilePath    string        `json:"session_file_path"`     // 会话文件存储路径
	CleanupInterval    time.Duration `json:"cleanup_interval"`      // 清理间隔
}

// DefaultSessionConfig 默认会话配置
var DefaultSessionConfig = SessionConfig{
	TokenExpiry:        24 * time.Hour,
	RefreshTokenExpiry: 7 * 24 * time.Hour,
	MaxSessionsPerUser: 5,
	EnableRefreshToken: true,
	CleanupInterval:    1 * time.Hour,
}

// SessionManager 会话管理器
type SessionManager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session // token -> session
	userSession map[string][]string // userID -> token list
	config      SessionConfig
}

// NewSessionManager 创建会话管理器
func NewSessionManager(config SessionConfig) *SessionManager {
	if config.TokenExpiry == 0 {
		config.TokenExpiry = DefaultSessionConfig.TokenExpiry
	}
	if config.RefreshTokenExpiry == 0 {
		config.RefreshTokenExpiry = DefaultSessionConfig.RefreshTokenExpiry
	}
	if config.MaxSessionsPerUser == 0 {
		config.MaxSessionsPerUser = DefaultSessionConfig.MaxSessionsPerUser
	}

	m := &SessionManager{
		sessions:    make(map[string]*Session),
		userSession: make(map[string][]string),
		config:      config,
	}

	// 加载已存储的会话
	if config.SessionFilePath != "" {
		m.load()
	}

	// 启动清理任务
	go m.cleanupLoop()

	return m
}

// CreateSession 创建新会话
func (m *SessionManager) CreateSession(userID, username, ip, userAgent string, roles, groups []string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查用户会话数量
	tokens := m.userSession[userID]
	if len(tokens) >= m.config.MaxSessionsPerUser {
		// 删除最旧的会话
		oldestToken := tokens[0]
		if session, exists := m.sessions[oldestToken]; exists {
			m.removeSession(session)
		}
	}

	now := time.Now()

	// 生成令牌
	token, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	// 生成刷新令牌
	var refreshToken string
	if m.config.EnableRefreshToken {
		refreshToken, err = generateSecureToken(32)
		if err != nil {
			return nil, err
		}
	}

	session := &Session{
		ID:           generateSecureID(),
		UserID:       userID,
		Username:     username,
		Token:        token,
		RefreshToken: refreshToken,
		CreatedAt:    now,
		ExpiresAt:    now.Add(m.config.TokenExpiry),
		LastAccessAt: now,
		IPAddress:    ip,
		UserAgent:    userAgent,
		Roles:        roles,
		Groups:       groups,
	}

	m.sessions[token] = session
	m.userSession[userID] = append(m.userSession[userID], token)

	m.save()

	return session, nil
}

// ValidateSession 验证会话
func (m *SessionManager) ValidateSession(token string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[token]
	if !exists {
		return nil, ErrSessionNotFound
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		m.removeSession(session)
		return nil, ErrSessionExpired
	}

	// 更新最后访问时间
	session.LastAccessAt = time.Now()

	return session, nil
}

// RefreshSession 刷新会话
func (m *SessionManager) RefreshSession(refreshToken string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找对应的会话
	var session *Session
	for _, s := range m.sessions {
		if s.RefreshToken == refreshToken {
			session = s
			break
		}
	}

	if session == nil {
		return nil, ErrRefreshTokenInvalid
	}

	// 检查刷新令牌是否过期
	refreshExpiry := session.CreatedAt.Add(m.config.RefreshTokenExpiry)
	if time.Now().After(refreshExpiry) {
		m.removeSession(session)
		return nil, ErrRefreshTokenExpired
	}

	// 生成新的令牌
	newToken, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	// 更新会话
	oldToken := session.Token
	session.Token = newToken
	session.ExpiresAt = time.Now().Add(m.config.TokenExpiry)
	session.LastAccessAt = time.Now()

	// 更新索引
	delete(m.sessions, oldToken)
	m.sessions[newToken] = session

	// 更新用户会话列表
	for i, t := range m.userSession[session.UserID] {
		if t == oldToken {
			m.userSession[session.UserID][i] = newToken
			break
		}
	}

	m.save()

	return session, nil
}

// InvalidateSession 使会话失效
func (m *SessionManager) InvalidateSession(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[token]
	if !exists {
		return ErrSessionNotFound
	}

	m.removeSession(session)
	m.save()

	return nil
}

// InvalidateUserSessions 使用户所有会话失效
func (m *SessionManager) InvalidateUserSessions(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tokens := m.userSession[userID]
	for _, token := range tokens {
		if _, exists := m.sessions[token]; exists {
			delete(m.sessions, token)
		}
	}
	delete(m.userSession, userID)

	m.save()

	return nil
}

// GetUserSessions 获取用户的所有会话
func (m *SessionManager) GetUserSessions(userID string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tokens := m.userSession[userID]
	sessions := make([]*Session, 0, len(tokens))

	for _, token := range tokens {
		if session, exists := m.sessions[token]; exists {
			// 复制会话，不返回敏感信息
			sessions = append(sessions, &Session{
				ID:           session.ID,
				UserID:       session.UserID,
				Username:     session.Username,
				CreatedAt:    session.CreatedAt,
				ExpiresAt:    session.ExpiresAt,
				LastAccessAt: session.LastAccessAt,
				IPAddress:    session.IPAddress,
				UserAgent:    session.UserAgent,
				MFAVerified:  session.MFAVerified,
			})
		}
	}

	return sessions
}

// SetMFAVerified 设置 MFA 验证状态
func (m *SessionManager) SetMFAVerified(token string, verified bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[token]
	if !exists {
		return ErrSessionNotFound
	}

	session.MFAVerified = verified
	m.save()

	return nil
}

// UpdateSessionDevice 更新会话设备信息
func (m *SessionManager) UpdateSessionDevice(token, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[token]
	if !exists {
		return ErrSessionNotFound
	}

	session.DeviceID = deviceID
	m.save()

	return nil
}

// removeSession 移除会话（内部方法，需要持有锁）
func (m *SessionManager) removeSession(session *Session) {
	delete(m.sessions, session.Token)

	tokens := m.userSession[session.UserID]
	newTokens := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t != session.Token {
			newTokens = append(newTokens, t)
		}
	}

	if len(newTokens) > 0 {
		m.userSession[session.UserID] = newTokens
	} else {
		delete(m.userSession, session.UserID)
	}
}

// cleanupLoop 定期清理过期会话
func (m *SessionManager) cleanupLoop() {
	interval := m.config.CleanupInterval
	if interval == 0 {
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		m.Cleanup()
	}
}

// Cleanup 清理过期会话
func (m *SessionManager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0

	for _, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			m.removeSession(session)
			count++
		}
	}

	if count > 0 {
		m.save()
	}

	return count
}

// load 加载会话
func (m *SessionManager) load() error {
	if m.config.SessionFilePath == "" {
		return nil
	}

	data, err := os.ReadFile(m.config.SessionFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var sessions []*Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return err
	}

	now := time.Now()
	for _, session := range sessions {
		// 跳过过期会话
		if now.After(session.ExpiresAt) {
			continue
		}

		m.sessions[session.Token] = session
		m.userSession[session.UserID] = append(m.userSession[session.UserID], session.Token)
	}

	return nil
}

// save 保存会话
func (m *SessionManager) save() error {
	if m.config.SessionFilePath == "" {
		return nil
	}

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(m.config.SessionFilePath), 0700); err != nil {
		return err
	}

	return os.WriteFile(m.config.SessionFilePath, data, 0600)
}

// GetStats 获取统计信息
func (m *SessionManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_sessions":        len(m.sessions),
		"total_users":           len(m.userSession),
		"max_sessions_per_user": m.config.MaxSessionsPerUser,
		"token_expiry":          m.config.TokenExpiry.String(),
		"refresh_token_expiry":  m.config.RefreshTokenExpiry.String(),
	}
}

// 辅助函数

func generateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateSecureID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// 错误定义
var (
	ErrSessionNotFound     = errors.New("会话不存在")
	ErrSessionExpired      = errors.New("会话已过期")
	ErrRefreshTokenInvalid = errors.New("刷新令牌无效")
	ErrRefreshTokenExpired = errors.New("刷新令牌已过期")
)

// SessionMiddleware 会话中间件（用于 Gin）
type SessionMiddleware struct {
	sessionManager *SessionManager
}

// NewSessionMiddleware 创建会话中间件
func NewSessionMiddleware(sm *SessionManager) *SessionMiddleware {
	return &SessionMiddleware{
		sessionManager: sm,
	}
}

// RequireSession 需要有效会话的中间件
// 注意：这是一个示例，实际使用时需要配合 Gin 框架
/*
func (m *SessionMiddleware) RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.JSON(401, gin.H{"error": "未提供认证令牌"})
			c.Abort()
			return
		}

		session, err := m.sessionManager.ValidateSession(token)
		if err != nil {
			c.JSON(401, gin.H{"error": "无效或过期的会话"})
			c.Abort()
			return
		}

		// 设置会话信息到上下文
		c.Set("session", session)
		c.Set("user_id", session.UserID)
		c.Set("username", session.Username)
		c.Set("roles", session.Roles)

		c.Next()
	}
}
*/
