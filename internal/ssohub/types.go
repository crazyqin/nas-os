// Package ssohub provides SSO/identity federation hub for NAS-OS
// Features: OIDC/SAML provider, MFA management, session federation, identity bridge
// Competitor benchmark: 对标群晖SSO Server, 超越TrueNAS Directory Services
package ssohub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ProviderType represents SSO provider type
type ProviderType string

const (
	ProviderOIDC  ProviderType = "oidc"
	ProviderSAML  ProviderType = "saml"
	ProviderLDAP  ProviderType = "ldap"
	ProviderLocal ProviderType = "local"
)

// MFAType represents MFA type
type MFAType string

const (
	MFATOTP   MFAType = "totp"
	MFAWebAuthn MFAType = "webauthn"
	MFASMS    MFAType = "sms"
	MFAEmail  MFAType = "email"
)

// SessionStatus represents session status
type SessionStatus string

const (
	SessionActive    SessionStatus = "active"
	SessionExpired   SessionStatus = "expired"
	SessionRevoked   SessionStatus = "revoked"
)

// IdentityProvider represents an SSO identity provider
type IdentityProvider struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        ProviderType `json:"type"`
	Issuer      string       `json:"issuer"`
	ClientID    string       `json:"client_id"`
	Scopes      []string     `json:"scopes"`
	Enabled     bool         `json:"enabled"`
	AutoCreate  bool         `json:"auto_create_users"`
	DefaultRole string       `json:"default_role"`
	Metadata    string       `json:"metadata"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// User represents a federated user
type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Provider    string    `json:"provider"`
	ProviderID  string    `json:"provider_id"`
	Roles       []string  `json:"roles"`
	MFAs        []*MFA    `json:"mfas"`
	Enabled     bool      `json:"enabled"`
	LastLogin   time.Time `json:"last_login"`
	CreatedAt   time.Time `json:"created_at"`
}

// MFA represents a multi-factor authentication method
type MFA struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      MFAType   `json:"type"`
	Name      string    `json:"name"`
	Secret    string    `json:"secret,omitempty"`
	Enabled   bool      `json:"enabled"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
}

// Session represents a federated SSO session
type Session struct {
	ID           string        `json:"id"`
	UserID       string        `json:"user_id"`
	Provider     string        `json:"provider"`
	Status       SessionStatus `json:"status"`
	IPAddress    string        `json:"ip_address"`
	UserAgent    string        `json:"user_agent"`
	ExpiresAt    time.Time     `json:"expires_at"`
	LastActivity time.Time     `json:"last_activity"`
	CreatedAt    time.Time     `json:"created_at"`
}

// Role represents a user role
type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}

// SSOStats represents SSO hub statistics
type SSOStats struct {
	TotalProviders  int `json:"total_providers"`
	ActiveProviders int `json:"active_providers"`
	TotalUsers      int `json:"total_users"`
	ActiveUsers     int `json:"active_users"`
	TotalSessions   int `json:"total_sessions"`
	ActiveSessions  int `json:"active_sessions"`
	TotalMFAs       int `json:"total_mfas"`
	EnabledMFAs     int `json:"enabled_mfas"`
}

// Config holds SSO hub configuration
type Config struct {
	Enabled          bool   `json:"enabled"`
	SessionTimeoutMin int   `json:"session_timeout_minutes"`
	MaxSessions      int    `json:"max_sessions"`
	RequireMFA       bool   `json:"require_mfa"`
	AllowSelfReg     bool   `json:"allow_self_registration"`
	DefaultRole      string `json:"default_role"`
	Issuer           string `json:"issuer"`
	TokenExpiryMin   int    `json:"token_expiry_minutes"`
}

// Manager manages SSO hub
type Manager struct {
	config    *Config
	providers map[string]*IdentityProvider
	users     map[string]*User
	sessions  map[string]*Session
	roles     map[string]*Role
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new SSO hub manager
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:    config,
		providers: make(map[string]*IdentityProvider),
		users:     make(map[string]*User),
		sessions:  make(map[string]*Session),
		roles:     make(map[string]*Role),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the SSO hub
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return fmt.Errorf("SSO hub is disabled")
	}
	return nil
}

// Stop stops the SSO hub
func (m *Manager) Stop() {
	m.cancel()
}

// AddProvider adds an identity provider
func (m *Manager) AddProvider(provider *IdentityProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	provider.ID = fmt.Sprintf("idp-%d", time.Now().UnixNano())
	provider.CreatedAt = time.Now()
	provider.UpdatedAt = time.Now()
	m.providers[provider.ID] = provider
	return nil
}

// ListProviders returns all identity providers
func (m *Manager) ListProviders() []*IdentityProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	providers := make([]*IdentityProvider, 0, len(m.providers))
	for _, p := range m.providers {
		providers = append(providers, p)
	}
	return providers
}

// DeleteProvider deletes an identity provider
func (m *Manager) DeleteProvider(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.providers[id]; !ok {
		return fmt.Errorf("provider %s not found", id)
	}
	delete(m.providers, id)
	return nil
}

// AddUser adds a user
func (m *Manager) AddUser(user *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user.ID = fmt.Sprintf("user-%d", time.Now().UnixNano())
	user.CreatedAt = time.Now()
	m.users[user.ID] = user
	return nil
}

// GetUser returns a user by ID
func (m *Manager) GetUser(id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("user %s not found", id)
	}
	return user, nil
}

// ListUsers returns all users
func (m *Manager) ListUsers() []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	users := make([]*User, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, u)
	}
	return users
}

// CreateSession creates a new SSO session
func (m *Manager) CreateSession(userID, ip, ua string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return nil, fmt.Errorf("user %s not found", userID)
	}
	if !user.Enabled {
		return nil, fmt.Errorf("user %s is disabled", userID)
	}

	// Check MFA requirement
	if m.config.RequireMFA && len(user.MFAs) == 0 {
		return nil, fmt.Errorf("MFA required but not configured")
	}

	token := make([]byte, 32)
	rand.Read(token)

	session := &Session{
		ID:           hex.EncodeToString(token),
		UserID:       userID,
		Provider:     user.Provider,
		Status:       SessionActive,
		IPAddress:    ip,
		UserAgent:    ua,
		ExpiresAt:    time.Now().Add(time.Duration(m.config.SessionTimeoutMin) * time.Minute),
		LastActivity: time.Now(),
		CreatedAt:    time.Now(),
	}

	m.sessions[session.ID] = session
	user.LastLogin = time.Now()
	return session, nil
}

// RevokeSession revokes a session
func (m *Manager) RevokeSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	session.Status = SessionRevoked
	return nil
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

// AddMFA adds an MFA method to a user
func (m *Manager) AddMFA(userID string, mfa *MFA) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user %s not found", userID)
	}
	mfa.ID = fmt.Sprintf("mfa-%d", time.Now().UnixNano())
	mfa.UserID = userID
	mfa.CreatedAt = time.Now()
	user.MFAs = append(user.MFAs, mfa)
	return nil
}

// AddRole adds a role
func (m *Manager) AddRole(role *Role) {
	m.mu.Lock()
	defer m.mu.Unlock()
	role.ID = fmt.Sprintf("role-%d", time.Now().UnixNano())
	role.CreatedAt = time.Now()
	m.roles[role.ID] = role
}

// ListRoles returns all roles
func (m *Manager) ListRoles() []*Role {
	m.mu.RLock()
	defer m.mu.RUnlock()
	roles := make([]*Role, 0, len(m.roles))
	for _, r := range m.roles {
		roles = append(roles, r)
	}
	return roles
}

// GetStats returns SSO statistics
func (m *Manager) GetStats() *SSOStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SSOStats{
		TotalProviders: len(m.providers),
		TotalUsers:     len(m.users),
		TotalSessions:  len(m.sessions),
	}

	for _, p := range m.providers {
		if p.Enabled {
			stats.ActiveProviders++
		}
	}
	for _, u := range m.users {
		if u.Enabled {
			stats.ActiveUsers++
		}
		stats.TotalMFAs += len(u.MFAs)
		for _, mfa := range u.MFAs {
			if mfa.Enabled {
				stats.EnabledMFAs++
			}
		}
	}
	for _, s := range m.sessions {
		if s.Status == SessionActive && s.ExpiresAt.After(time.Now()) {
			stats.ActiveSessions++
		}
	}

	return stats
}
