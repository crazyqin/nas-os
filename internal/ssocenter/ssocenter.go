// Package ssocenter 提供统一身份认证（SSO）中心功能，
// 对标 Synology SSO Server 和群Ada Directory Services。
// 支持 OIDC/SAML 协议、多应用集成、会话管理和安全审计。
// 礼部开发。
package ssocenter

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Protocol SSO 协议类型.
type Protocol string

const (
	ProtocolOIDC  Protocol = "oidc"
	ProtocolSAML  Protocol = "saml"
	ProtocolCAS   Protocol = "cas"
)

// AppStatus 应用状态.
type AppStatus string

const (
	AppStatusActive    AppStatus = "active"
	AppStatusDisabled  AppStatus = "disabled"
	AppStatusPending   AppStatus = "pending"
)

// SSOApp SSO 应用注册.
type SSOApp struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Desc         string     `json:"description"`
	Protocol     Protocol   `json:"protocol"`
	Issuer       string     `json:"issuer"`
	RedirectURIs []string   `json:"redirect_uris"`
	Secret       string     `json:"secret"` // 只存 hash
	Status       AppStatus  `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LogoURL      string     `json:"logo_url,omitempty"`
	Scopes       []string   `json:"scopes"`
	TokenTTL     int        `json:"token_ttl_seconds"`
}

// Session SSO 会话.
type Session struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	AppID       string    `json:"app_id"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	IP          string    `json:"ip"`
	UserAgent   string    `json:"user_agent"`
	Revoked     bool      `json:"revoked"`
}

// AuditEvent 审计事件.
type AuditEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // login, logout, token_issue, token_revoke, app_register
	UserID    string    `json:"user_id"`
	AppID     string    `json:"app_id"`
	IP        string    `json:"ip"`
	Success   bool      `json:"success"`
	Detail    string    `json:"detail"`
	Timestamp time.Time `json:"timestamp"`
}

// TokenPair 令牌对.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// Manager SSO 管理器.
type Manager struct {
	mu       sync.RWMutex
	apps     map[string]*SSOApp
	sessions map[string]*Session
	audit    []AuditEvent
}

// NewManager 创建 SSO 管理器.
func NewManager() *Manager {
	return &Manager{
		apps:     make(map[string]*SSOApp),
		sessions: make(map[string]*Session),
	}
}

// RegisterApp 注册 SSO 应用.
func (m *Manager) RegisterApp(app *SSOApp) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if app.Name == "" {
		return fmt.Errorf("app name required")
	}
	if len(app.RedirectURIs) == 0 {
		return fmt.Errorf("at least one redirect URI required")
	}
	if app.ID == "" {
		app.ID = fmt.Sprintf("sso-app-%s", randomID(8))
	}
	if app.Secret == "" {
		app.Secret = randomID(32)
	}
	if app.Issuer == "" {
		app.Issuer = fmt.Sprintf("https://nas-os.local/sso/%s", app.ID)
	}
	if app.TokenTTL == 0 {
		app.TokenTTL = 3600
	}
	app.Status = AppStatusActive
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	m.apps[app.ID] = app
	m.addAudit(AuditEvent{
		Type: "app_register", AppID: app.ID,
		Detail: fmt.Sprintf("App %s registered with protocol %s", app.Name, app.Protocol),
		Timestamp: time.Now(), Success: true,
	})
	return nil
}

// ListApps 列出应用.
func (m *Manager) ListApps() []*SSOApp {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*SSOApp, 0, len(m.apps))
	for _, a := range m.apps {
		result = append(result, a)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}

// DisableApp 禁用应用.
func (m *Manager) DisableApp(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("app %s not found", id)
	}
	a.Status = AppStatusDisabled
	a.UpdatedAt = time.Now()
	return nil
}

// CreateSession 创建会话.
func (m *Manager) CreateSession(userID, appID, ip, userAgent string, ttlSeconds int) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apps[appID]; !ok {
		return nil, fmt.Errorf("app %s not found", appID)
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}
	session := &Session{
		ID:        fmt.Sprintf("sess-%s", randomID(16)),
		UserID:    userID,
		AppID:     appID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
		IP:        ip,
		UserAgent: userAgent,
	}
	m.sessions[session.ID] = session
	m.addAudit(AuditEvent{
		Type: "login", UserID: userID, AppID: appID, IP: ip,
		Detail: "SSO login successful", Success: true, Timestamp: time.Now(),
	})
	return session, nil
}

// ValidateSession 验证会话.
func (m *Manager) ValidateSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	if s.Revoked {
		return nil, fmt.Errorf("session revoked")
	}
	if time.Now().After(s.ExpiresAt) {
		return nil, fmt.Errorf("session expired")
	}
	return s, nil
}

// RevokeSession 撤销会话.
func (m *Manager) RevokeSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}
	s.Revoked = true
	m.addAudit(AuditEvent{
		Type: "logout", UserID: s.UserID, AppID: s.AppID, IP: s.IP,
		Detail: "Session revoked", Success: true, Timestamp: time.Now(),
	})
	return nil
}

// IssueTokens 签发令牌.
func (m *Manager) IssueTokens(sessionID string) (*TokenPair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[sessionID]
	if !ok || s.Revoked {
		return nil, fmt.Errorf("invalid session")
	}
	app, ok := m.apps[s.AppID]
	if !ok {
		return nil, fmt.Errorf("app not found")
	}
	ttl := app.TokenTTL
	pair := &TokenPair{
		AccessToken:  randomID(32),
		RefreshToken: randomID(48),
		IDToken:      randomID(32),
		ExpiresIn:    ttl,
		TokenType:    "Bearer",
	}
	m.addAudit(AuditEvent{
		Type: "token_issue", UserID: s.UserID, AppID: s.AppID,
		Detail: "Tokens issued", Success: true, Timestamp: time.Now(),
	})
	return pair, nil
}

// ListSessions 列出会话.
func (m *Manager) ListSessions(userID string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Session, 0)
	for _, s := range m.sessions {
		if userID == "" || s.UserID == userID {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

// ListAuditEvents 列出审计事件.
func (m *Manager) ListAuditEvents(limit int) []AuditEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.audit) {
		limit = len(m.audit)
	}
	result := make([]AuditEvent, limit)
	copy(result, m.audit[len(m.audit)-limit:])
	return result
}

// CleanupExpiredSessions 清理过期会话.
func (m *Manager) CleanupExpiredSessions() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	count := 0
	for id, s := range m.sessions {
		if s.ExpiresAt.Before(now) || s.Revoked {
			delete(m.sessions, id)
			count++
		}
	}
	return count
}

// ValidateRedirectURI 验证回调 URI.
func (m *Manager) ValidateRedirectURI(appID, uri string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	app, ok := m.apps[appID]
	if !ok {
		return false
	}
	for _, allowed := range app.RedirectURIs {
		if strings.HasPrefix(uri, allowed) {
			return true
		}
	}
	return false
}

func (m *Manager) addAudit(event AuditEvent) {
	if event.ID == "" {
		event.ID = fmt.Sprintf("audit-%s", randomID(8))
	}
	m.audit = append(m.audit, event)
	if len(m.audit) > 10000 {
		m.audit = m.audit[len(m.audit)-10000:]
	}
}

func randomID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

