package ztaccess

import (
	"testing"
	"time"
)

func TestNewZTAccess(t *testing.T) {
	zt := NewZTAccess()
	if zt == nil {
		t.Fatal("Expected non-nil ZTAccess")
	}
	if zt.users == nil {
		t.Fatal("Expected non-nil users map")
	}
	if zt.sessions == nil {
		t.Fatal("Expected non-nil sessions map")
	}
}

func TestAuthentication(t *testing.T) {
	zt := NewZTAccess()
	gw := NewGateway(zt)

	// Register a user
	zt.mu.Lock()
	zt.users["user1"] = &UserIdentity{
		UserID:      "user1",
		Username:    "testuser",
		AccessLevel: LevelUser,
		Groups:      []string{"users"},
		Permissions: []string{"read", "write"},
	}
	zt.mu.Unlock()

	device := DeviceInfo{
		DeviceID:    "device1",
		DeviceName:  "Test Device",
		DeviceType:  "desktop",
		OS:          "Linux",
		Browser:     "Chrome",
		Fingerprint: "test-fingerprint",
		IPAddress:   "192.168.1.100",
	}

	session, err := gw.Authenticate("testuser", "password", device)
	if err != nil {
		t.Fatalf("Authentication failed: %v", err)
	}

	if session == nil {
		t.Fatal("Expected non-nil session")
	}
	if session.UserID != "user1" {
		t.Fatalf("Expected user1, got %s", session.UserID)
	}
	if !session.IsActive {
		t.Fatal("Expected active session")
	}
}

func TestSessionValidation(t *testing.T) {
	zt := NewZTAccess()
	gw := NewGateway(zt)

	// Create a session
	sm := NewSessionManager(zt)
	device := DeviceInfo{DeviceID: "d1", IPAddress: "127.0.0.1"}
	session := sm.CreateSession("user1", device, LevelUser)

	// Validate
	validated, err := gw.ValidateSession(session.SessionID)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}
	if validated.SessionID != session.SessionID {
		t.Fatal("Session ID mismatch")
	}
}

func TestSessionExpiry(t *testing.T) {
	zt := NewZTAccess()
	zt.sessionTTL = 1 * time.Millisecond // Very short TTL

	sm := NewSessionManager(zt)
	device := DeviceInfo{DeviceID: "d1", IPAddress: "127.0.0.1"}
	session := sm.CreateSession("user1", device, LevelUser)

	time.Sleep(5 * time.Millisecond)

	gw := NewGateway(zt)
	_, err := gw.ValidateSession(session.SessionID)
	if err != ErrSessionExpired {
		t.Fatalf("Expected session expired, got %v", err)
	}
}

func TestAuthorization(t *testing.T) {
	zt := NewZTAccess()
	gw := NewGateway(zt)

	// Create policy
	zt.mu.Lock()
	zt.policies["policy1"] = &AccessPolicy{
		PolicyID:  "policy1",
		Name:      "Allow Read",
		Resources: []string{"/api/files"},
		Actions:   []string{"read"},
		Enabled:   true,
		Priority:  1,
	}
	zt.mu.Unlock()

	// Create session
	sm := NewSessionManager(zt)
	device := DeviceInfo{DeviceID: "d1", IPAddress: "127.0.0.1"}
	session := sm.CreateSession("user1", device, LevelUser)

	// Authorize
	allowed, err := gw.Authorize(session.SessionID, "/api/files", "read")
	if err != nil {
		t.Fatalf("Authorization failed: %v", err)
	}
	if !allowed {
		t.Fatal("Expected access to be allowed")
	}
}

func TestSessionRevocation(t *testing.T) {
	zt := NewZTAccess()
	gw := NewGateway(zt)

	sm := NewSessionManager(zt)
	device := DeviceInfo{DeviceID: "d1", IPAddress: "127.0.0.1"}
	session := sm.CreateSession("user1", device, LevelUser)

	// Revoke
	err := gw.RevokeSession(session.SessionID)
	if err != nil {
		t.Fatalf("Revocation failed: %v", err)
	}

	// Validate should fail
	_, err = gw.ValidateSession(session.SessionID)
	if err == nil {
		t.Fatal("Expected validation to fail after revocation")
	}
}

func TestAuditLogging(t *testing.T) {
	zt := NewZTAccess()
	am := NewAuditManager(zt)

	am.LogActivity("user1", "read", "/api/files", "success", "127.0.0.1", "test-agent", "test")

	log := am.GetAuditLog(10, map[string]string{"user_id": "user1"})
	if len(log) == 0 {
		t.Fatal("Expected audit log entries")
	}
	if log[0].Action != "read" {
		t.Fatalf("Expected action 'read', got '%s'", log[0].Action)
	}
}

func TestAuditStats(t *testing.T) {
	zt := NewZTAccess()
	am := NewAuditManager(zt)

	am.LogActivity("user1", "read", "/api", "success", "127.0.0.1", "", "")
	am.LogActivity("user2", "write", "/api", "denied", "127.0.0.1", "", "")

	stats := am.GetAuditStats()
	if stats["total"] != 2 {
		t.Fatalf("Expected 2 total, got %v", stats["total"])
	}
}

func TestSessionStats(t *testing.T) {
	zt := NewZTAccess()
	sm := NewSessionManager(zt)

	device := DeviceInfo{DeviceID: "d1", IPAddress: "127.0.0.1"}
	sm.CreateSession("user1", device, LevelUser)
	sm.CreateSession("user2", device, LevelAdmin)

	stats := sm.GetSessionStats()
	if stats["total"] != 2 {
		t.Fatalf("Expected 2 total sessions, got %v", stats["total"])
	}
	if stats["active"] != 2 {
		t.Fatalf("Expected 2 active sessions, got %v", stats["active"])
	}
}

func TestTokenGeneration(t *testing.T) {
	zt := NewZTAccess()
	am := NewAuthManager(zt)

	device := DeviceInfo{DeviceID: "d1", IPAddress: "127.0.0.1"}
	session := &Session{
		SessionID:   "sess1",
		UserID:      "user1",
		Device:      device,
		AccessLevel: LevelUser,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	token, err := am.GenerateToken(session)
	if err != nil {
		t.Fatalf("Token generation failed: %v", err)
	}
	if token == "" {
		t.Fatal("Expected non-empty token")
	}
}

func TestDeviceFingerprint(t *testing.T) {
	zt := NewZTAccess()
	am := NewAuthManager(zt)

	device := DeviceInfo{
		DeviceID:   "d1",
		DeviceType: "desktop",
		OS:         "Linux",
		Browser:    "Chrome",
	}

	fp := am.GenerateDeviceFingerprint(device)
	if fp == "" {
		t.Fatal("Expected non-empty fingerprint")
	}

	if !am.ValidateDeviceFingerprint(device) {
		t.Fatal("Expected fingerprint to be valid")
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	zt := NewZTAccess()
	zt.sessionTTL = 1 * time.Millisecond

	sm := NewSessionManager(zt)
	device := DeviceInfo{DeviceID: "d1", IPAddress: "127.0.0.1"}
	sm.CreateSession("user1", device, LevelUser)

	time.Sleep(5 * time.Millisecond)

	cleaned := sm.CleanupExpiredSessions()
	if cleaned != 1 {
		t.Fatalf("Expected 1 cleaned, got %d", cleaned)
	}
}
