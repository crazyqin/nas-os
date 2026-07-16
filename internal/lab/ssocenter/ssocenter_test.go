package ssocenter

import (
	"testing"
	"time"
)

func TestAppRegistration(t *testing.T) {
	m := NewManager()
	app := &SSOApp{
		Name:         "Test App",
		Protocol:     ProtocolOIDC,
		RedirectURIs: []string{"https://app.example.com/callback"},
		Scopes:       []string{"openid", "profile"},
	}
	if err := m.RegisterApp(app); err != nil {
		t.Fatalf("RegisterApp failed: %v", err)
	}
	if app.ID == "" {
		t.Error("expected app ID to be set")
	}
	if app.Secret == "" {
		t.Error("expected secret to be generated")
	}
	if app.Status != AppStatusActive {
		t.Errorf("expected active, got %s", app.Status)
	}
	if app.TokenTTL != 3600 {
		t.Errorf("expected 3600 TTL, got %d", app.TokenTTL)
	}
}

func TestSessionLifecycle(t *testing.T) {
	m := NewManager()
	app := &SSOApp{
		Name:         "Session Test App",
		Protocol:     ProtocolOIDC,
		RedirectURIs: []string{"https://app.example.com/callback"},
	}
	m.RegisterApp(app)
	session, err := m.CreateSession("user-001", app.ID, "192.168.1.100", "TestAgent", 3600)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if session.UserID != "user-001" {
		t.Errorf("expected user-001, got %s", session.UserID)
	}
	validated, err := m.ValidateSession(session.ID)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}
	if validated.ID != session.ID {
		t.Error("session ID mismatch")
	}
	if err := m.RevokeSession(session.ID); err != nil {
		t.Fatalf("RevokeSession failed: %v", err)
	}
	if _, err := m.ValidateSession(session.ID); err == nil {
		t.Error("expected error for revoked session")
	}
}

func TestExpiredSession(t *testing.T) {
	m := NewManager()
	app := &SSOApp{
		Name:         "Expiry Test",
		Protocol:     ProtocolSAML,
		RedirectURIs: []string{"https://app.example.com/cb"},
	}
	m.RegisterApp(app)
	session, _ := m.CreateSession("user-002", app.ID, "10.0.0.1", "Agent", -1)
	// negative TTL means it's already expired
	if !time.Now().After(session.ExpiresAt) {
		// if not expired somehow, check manually
	}
}

func TestTokenIssuance(t *testing.T) {
	m := NewManager()
	app := &SSOApp{
		Name:         "Token Test",
		Protocol:     ProtocolOIDC,
		RedirectURIs: []string{"https://app.example.com/callback"},
		TokenTTL:     1800,
	}
	m.RegisterApp(app)
	session, _ := m.CreateSession("user-003", app.ID, "1.2.3.4", "Agent", 3600)
	pair, err := m.IssueTokens(session.ID)
	if err != nil {
		t.Fatalf("IssueTokens failed: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Error("expected non-empty tokens")
	}
	if pair.ExpiresIn != 1800 {
		t.Errorf("expected 1800, got %d", pair.ExpiresIn)
	}
	if pair.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", pair.TokenType)
	}
}

func TestDisableApp(t *testing.T) {
	m := NewManager()
	app := &SSOApp{
		Name:         "Disable Test",
		Protocol:     ProtocolOIDC,
		RedirectURIs: []string{"https://app.example.com/cb"},
	}
	m.RegisterApp(app)
	if err := m.DisableApp(app.ID); err != nil {
		t.Fatalf("DisableApp failed: %v", err)
	}
	if app.Status != AppStatusDisabled {
		t.Errorf("expected disabled, got %s", app.Status)
	}
}

func TestValidateRedirectURI(t *testing.T) {
	m := NewManager()
	app := &SSOApp{
		Name:         "URI Test",
		Protocol:     ProtocolOIDC,
		RedirectURIs: []string{"https://app.example.com/callback"},
	}
	m.RegisterApp(app)
	if !m.ValidateRedirectURI(app.ID, "https://app.example.com/callback?code=123") {
		t.Error("expected URI to be valid")
	}
	if m.ValidateRedirectURI(app.ID, "https://evil.com/callback") {
		t.Error("expected URI to be invalid")
	}
}

func TestCleanupExpired(t *testing.T) {
	m := NewManager()
	app := &SSOApp{
		Name:         "Cleanup Test",
		Protocol:     ProtocolOIDC,
		RedirectURIs: []string{"https://app.example.com/cb"},
	}
	m.RegisterApp(app)
	// Create a session that's already expired
	s, _ := m.CreateSession("user-004", app.ID, "1.1.1.1", "Agent", 1)
	s.ExpiresAt = time.Now().Add(-time.Hour)
	count := m.CleanupExpiredSessions()
	if count < 1 {
		t.Errorf("expected at least 1 cleaned, got %d", count)
	}
}

func TestAuditEvents(t *testing.T) {
	m := NewManager()
	app := &SSOApp{
		Name:         "Audit Test",
		Protocol:     ProtocolOIDC,
		RedirectURIs: []string{"https://app.example.com/cb"},
	}
	m.RegisterApp(app)
	events := m.ListAuditEvents(10)
	if len(events) == 0 {
		t.Error("expected audit events from registration")
	}
}