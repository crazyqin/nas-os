// Package remotesupport 提供远程支持隧道功能
package remotesupport

import (
	"path/filepath"
	"testing"
)

// ========== Manager 基础测试 ==========

func TestNewManager(t *testing.T) {
	mgr := NewManager(nil, "")
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestNewManager_WithConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "remotesupport-config.json")

	mgr := NewManager(nil, configPath)
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
}

// ========== 会话管理测试 ==========

func TestCreateSession(t *testing.T) {
	mgr := NewManager(nil, "")

	req := SessionCreateRequest{
		ClientName:  "技术支持-Alice",
		TargetHost:  "192.168.1.100",
		TargetPort:  22,
		AccessLevel: AccessLevelReadWrite,
		BandwidthKB: 2048,
		MaxDuration: 3600,
		Recorded:    true,
	}

	session, token, err := mgr.CreateSession(req)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if session.Status != SessionStatusPending {
		t.Errorf("expected status pending, got %s", session.Status)
	}
	if session.ClientName != "技术支持-Alice" {
		t.Errorf("expected client name 技术支持-Alice, got %s", session.ClientName)
	}
	if session.AccessLevel != AccessLevelReadWrite {
		t.Errorf("expected access level readwrite, got %s", session.AccessLevel)
	}
	if session.Recorded != true {
		t.Error("expected recorded to be true")
	}
	if token.Token == "" {
		t.Error("token should not be empty")
	}
}

func TestCreateSession_DefaultValues(t *testing.T) {
	mgr := NewManager(nil, "")

	req := SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	}

	session, _, err := mgr.CreateSession(req)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if session.AccessLevel != AccessLevelReadOnly {
		t.Errorf("expected default access level readonly, got %s", session.AccessLevel)
	}
	if session.BandwidthKB != 1024 {
		t.Errorf("expected default bandwidth 1024, got %d", session.BandwidthKB)
	}
}

func TestGetSession(t *testing.T) {
	mgr := NewManager(nil, "")

	req := SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	}
	session, _, _ := mgr.CreateSession(req)

	got, err := mgr.GetSession(session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("expected ID %s, got %s", session.ID, got.ID)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	mgr := NewManager(nil, "")

	_, err := mgr.GetSession("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestListSessions(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateSession(SessionCreateRequest{ClientName: "client-1", TargetHost: "10.0.0.1"})
	mgr.CreateSession(SessionCreateRequest{ClientName: "client-2", TargetHost: "10.0.0.2"})

	sessions := mgr.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestUpdateSession(t *testing.T) {
	mgr := NewManager(nil, "")

	session, _, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	})

	active := SessionStatusActive
	updated, err := mgr.UpdateSession(session.ID, SessionUpdateRequest{
		Status: &active,
	})
	if err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}
	if updated.Status != SessionStatusActive {
		t.Errorf("expected status active, got %s", updated.Status)
	}
}

func TestCloseSession(t *testing.T) {
	mgr := NewManager(nil, "")

	session, _, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	})

	err := mgr.CloseSession(session.ID)
	if err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	got, _ := mgr.GetSession(session.ID)
	if got.Status != SessionStatusClosed {
		t.Errorf("expected status closed, got %s", got.Status)
	}
	if got.EndedAt == nil {
		t.Error("ended_at should be set after close")
	}
}

func TestCloseSession_NotFound(t *testing.T) {
	mgr := NewManager(nil, "")

	err := mgr.CloseSession("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestCloseSession_AlreadyClosed(t *testing.T) {
	mgr := NewManager(nil, "")

	session, _, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	})

	mgr.CloseSession(session.ID)
	err := mgr.CloseSession(session.ID)
	if err != ErrSessionClosed {
		t.Errorf("expected ErrSessionClosed, got: %v", err)
	}
}

// ========== 令牌管理测试 ==========

func TestValidateToken(t *testing.T) {
	mgr := NewManager(nil, "")

	_, token, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	})

	session, err := mgr.ValidateToken(TokenValidateRequest{
		Token:    token.Token,
		ClientIP: "192.168.1.50",
	})
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if session.Status != SessionStatusActive {
		t.Errorf("expected status active after validation, got %s", session.Status)
	}
	if session.ClientIP != "192.168.1.50" {
		t.Errorf("expected client IP 192.168.1.50, got %s", session.ClientIP)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	mgr := NewManager(nil, "")

	_, err := mgr.ValidateToken(TokenValidateRequest{
		Token: "invalid-token",
	})
	if err != ErrTokenInvalid {
		t.Errorf("expected ErrTokenInvalid, got: %v", err)
	}
}

func TestValidateToken_AlreadyUsed(t *testing.T) {
	mgr := NewManager(nil, "")

	_, token, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	})

	mgr.ValidateToken(TokenValidateRequest{Token: token.Token, ClientIP: "10.0.0.1"})
	_, err := mgr.ValidateToken(TokenValidateRequest{Token: token.Token, ClientIP: "10.0.0.2"})
	if err != ErrTokenUsed {
		t.Errorf("expected ErrTokenUsed, got: %v", err)
	}
}

// ========== 隧道管理测试 ==========

func TestEstablishTunnel(t *testing.T) {
	mgr := NewManager(nil, "")

	session, token, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "192.168.1.100",
		TargetPort: 22,
	})

	// 先激活会话
	mgr.ValidateToken(TokenValidateRequest{Token: token.Token, ClientIP: "10.0.0.1"})

	tunnel, err := mgr.EstablishTunnel(session.ID)
	if err != nil {
		t.Fatalf("EstablishTunnel failed: %v", err)
	}
	if tunnel.Status != "active" {
		t.Errorf("expected tunnel status active, got %s", tunnel.Status)
	}
	if tunnel.RemoteAddr != "192.168.1.100:22" {
		t.Errorf("expected remote addr 192.168.1.100:22, got %s", tunnel.RemoteAddr)
	}
}

func TestEstablishTunnel_SessionNotActive(t *testing.T) {
	mgr := NewManager(nil, "")

	session, _, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	})

	// 会话未激活
	_, err := mgr.EstablishTunnel(session.ID)
	if err != ErrSessionClosed {
		t.Errorf("expected ErrSessionClosed, got: %v", err)
	}
}

func TestCloseTunnel(t *testing.T) {
	mgr := NewManager(nil, "")

	session, token, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
		TargetPort: 22,
	})

	mgr.ValidateToken(TokenValidateRequest{Token: token.Token, ClientIP: "10.0.0.1"})
	mgr.EstablishTunnel(session.ID)

	err := mgr.CloseTunnel(session.ID)
	if err != nil {
		t.Fatalf("CloseTunnel failed: %v", err)
	}

	tunnel, _ := mgr.GetTunnel(session.ID)
	if tunnel.Status != "closed" {
		t.Errorf("expected tunnel status closed, got %s", tunnel.Status)
	}
}

func TestGetTunnel_NotFound(t *testing.T) {
	mgr := NewManager(nil, "")

	_, err := mgr.GetTunnel("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

// ========== 带宽控制测试 ==========

func TestRecordTransfer(t *testing.T) {
	mgr := NewManager(nil, "")

	session, _, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	})

	err := mgr.RecordTransfer(session.ID, 1024, 2048)
	if err != nil {
		t.Fatalf("RecordTransfer failed: %v", err)
	}

	got, _ := mgr.GetSession(session.ID)
	if got.BytesUp != 1024 {
		t.Errorf("expected bytes_up 1024, got %d", got.BytesUp)
	}
	if got.BytesDown != 2048 {
		t.Errorf("expected bytes_down 2048, got %d", got.BytesDown)
	}
}

func TestRecordTransfer_NotFound(t *testing.T) {
	mgr := NewManager(nil, "")

	err := mgr.RecordTransfer("nonexistent", 1024, 1024)
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

// ========== 审计日志测试 ==========

func TestAddAuditEntry(t *testing.T) {
	mgr := NewManager(nil, "")

	session, _, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	})

	err := mgr.AddAuditEntry(session.ID, "test_action", "测试操作", "10.0.0.1")
	if err != nil {
		t.Fatalf("AddAuditEntry failed: %v", err)
	}

	log, _ := mgr.GetAuditLog(session.ID)
	if len(log) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(log))
	}
	if log[0].Action != "test_action" {
		t.Errorf("expected action test_action, got %s", log[0].Action)
	}
}

func TestGetAuditLog(t *testing.T) {
	mgr := NewManager(nil, "")

	session, _, _ := mgr.CreateSession(SessionCreateRequest{
		ClientName: "test-client",
		TargetHost: "10.0.0.1",
	})

	mgr.AddAuditEntry(session.ID, "action1", "detail1", "src1")
	mgr.AddAuditEntry(session.ID, "action2", "detail2", "src2")

	log, err := mgr.GetAuditLog(session.ID)
	if err != nil {
		t.Fatalf("GetAuditLog failed: %v", err)
	}
	if len(log) != 2 {
		t.Errorf("expected 2 audit entries, got %d", len(log))
	}
}

func TestGetAuditLog_NotFound(t *testing.T) {
	mgr := NewManager(nil, "")

	_, err := mgr.GetAuditLog("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	mgr := NewManager(nil, "")

	mgr.CreateSession(SessionCreateRequest{ClientName: "client-1", TargetHost: "10.0.0.1"})
	mgr.CreateSession(SessionCreateRequest{ClientName: "client-2", TargetHost: "10.0.0.2"})

	stats := mgr.GetStats()
	if stats.TotalSessions != 2 {
		t.Errorf("expected 2 total sessions, got %d", stats.TotalSessions)
	}
}
