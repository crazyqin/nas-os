package ssohub

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:           true,
		SessionTimeoutMin: 60,
		MaxSessions:       100,
		RequireMFA:        false,
		AllowSelfReg:      true,
		DefaultRole:       "user",
		Issuer:            "https://nas.local",
		TokenExpiryMin:    30,
	}
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestAddProvider(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	provider := &IdentityProvider{
		Name:     "Google",
		Type:     ProviderOIDC,
		Issuer:   "https://accounts.google.com",
		ClientID: "test-client-id",
		Scopes:   []string{"openid", "email", "profile"},
		Enabled:  true,
	}
	if err := manager.AddProvider(provider); err != nil {
		t.Fatalf("AddProvider failed: %v", err)
	}

	providers := manager.ListProviders()
	if len(providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(providers))
	}
}

func TestAddUser(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	user := &User{
		Username:    "testuser",
		Email:       "test@example.com",
		DisplayName: "Test User",
		Provider:    "local",
		Roles:       []string{"user"},
		Enabled:     true,
	}
	if err := manager.AddUser(user); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}

	users := manager.ListUsers()
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
}

func TestCreateSession(t *testing.T) {
	config := &Config{Enabled: true, SessionTimeoutMin: 60, RequireMFA: false}
	manager := NewManager(config)

	user := &User{Username: "u", Email: "u@e.com", Provider: "local", Enabled: true}
	manager.AddUser(user)

	session, err := manager.CreateSession(user.ID, "192.168.1.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if session.Status != SessionActive {
		t.Errorf("expected active, got %s", session.Status)
	}
}

func TestRevokeSession(t *testing.T) {
	config := &Config{Enabled: true, SessionTimeoutMin: 60, RequireMFA: false}
	manager := NewManager(config)

	user := &User{Username: "u", Email: "u@e.com", Provider: "local", Enabled: true}
	manager.AddUser(user)

	session, _ := manager.CreateSession(user.ID, "1.1.1.1", "test")
	if err := manager.RevokeSession(session.ID); err != nil {
		t.Fatalf("RevokeSession failed: %v", err)
	}

	sessions := manager.ListSessions()
	if sessions[0].Status != SessionRevoked {
		t.Errorf("expected revoked, got %s", sessions[0].Status)
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	stats := manager.GetStats()
	if stats.TotalProviders != 0 {
		t.Errorf("expected 0 providers, got %d", stats.TotalProviders)
	}
}
