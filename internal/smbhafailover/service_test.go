package smbhafailover

import (
	"testing"
	"time"
)

func TestRegisterAndGetSession(t *testing.T) {
	m := NewFailoverManager()
	s := &SessionState{
		SessionID:  "s1",
		ClientIP:   "192.168.1.100",
		Username:   "admin",
		Domain:     "WORKGROUP",
		SessionKey: "key123",
	}
	if err := m.RegisterSession(s); err != nil {
		t.Fatalf("RegisterSession failed: %v", err)
	}

	got, err := m.GetSession("s1")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if got.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", got.Username)
	}
	if got.LastAccess.IsZero() {
		t.Error("expected LastAccess to be set")
	}
}

func TestRegisterNilSession(t *testing.T) {
	m := NewFailoverManager()
	if err := m.RegisterSession(nil); err == nil {
		t.Error("expected error for nil session")
	}
}

func TestRegisterEmptySessionID(t *testing.T) {
	m := NewFailoverManager()
	s := &SessionState{Username: "test"}
	if err := m.RegisterSession(s); err == nil {
		t.Error("expected error for empty session ID")
	}
}

func TestListSessions(t *testing.T) {
	m := NewFailoverManager()
	m.RegisterSession(&SessionState{SessionID: "s1", Username: "u1"})
	m.RegisterSession(&SessionState{SessionID: "s2", Username: "u2"})

	sessions := m.ListSessions()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestUpdateSessionAccess(t *testing.T) {
	m := NewFailoverManager()
	m.RegisterSession(&SessionState{SessionID: "s1", Username: "u1"})
	time.Sleep(10 * time.Millisecond)
	if err := m.UpdateSessionAccess("s1"); err != nil {
		t.Fatalf("UpdateSessionAccess failed: %v", err)
	}
	s, _ := m.GetSession("s1")
	if s.LastAccess.Before(time.Now().Add(-1 * time.Second)) {
		t.Error("expected LastAccess to be updated")
	}
}

func TestRemoveSession(t *testing.T) {
	m := NewFailoverManager()
	m.RegisterSession(&SessionState{SessionID: "s1", Username: "u1"})
	m.RemoveSession("s1")
	if _, err := m.GetSession("s1"); err == nil {
		t.Error("expected error after remove")
	}
}

func TestCreateAndRestoreSnapshot(t *testing.T) {
	m := NewFailoverManager()
	m.RegisterSession(&SessionState{
		SessionID:  "s1",
		ClientIP:   "10.0.0.1",
		Username:   "admin",
		Domain:     "DOM",
		SessionKey: "k1",
		Shares:     []string{"share1"},
	})
	m.RegisterSession(&SessionState{
		SessionID:  "s2",
		ClientIP:   "10.0.0.2",
		Username:   "user2",
		Domain:     "DOM",
		SessionKey: "k2",
	})

	snap, err := m.CreateSnapshot("node-1")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if snap.Status != SnapshotStatusActive {
		t.Errorf("expected snapshot status active, got %q", snap.Status)
	}
	if len(snap.Sessions) != 2 {
		t.Fatalf("expected 2 sessions in snapshot, got %d", len(snap.Sessions))
	}

	// Remove all sessions, then restore from snapshot
	m.RemoveSession("s1")
	m.RemoveSession("s2")
	if len(m.ListSessions()) != 0 {
		t.Fatal("expected 0 sessions after removal")
	}

	restored, err := m.RestoreSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}
	if len(restored) != 2 {
		t.Fatalf("expected 2 restored sessions, got %d", len(restored))
	}
	if len(m.ListSessions()) != 2 {
		t.Fatalf("expected 2 sessions after restore, got %d", len(m.ListSessions()))
	}

	// Verify snapshot status updated
	snap2, _ := m.GetSnapshot(snap.ID)
	if snap2.Status != SnapshotStatusRestored {
		t.Errorf("expected snapshot status restored, got %q", snap2.Status)
	}
}

func TestRestoreNonExistentSnapshot(t *testing.T) {
	m := NewFailoverManager()
	if _, err := m.RestoreSnapshot("nonexistent"); err == nil {
		t.Error("expected error for non-existent snapshot")
	}
}

func TestGetState(t *testing.T) {
	m := NewFailoverManager()
	state := m.GetState()
	if state.Status != FailoverStatusIdle {
		t.Errorf("expected idle status, got %q", state.Status)
	}
	if state.ActiveNode == "" {
		t.Error("expected non-empty active node")
	}
}

func TestSetActiveNode(t *testing.T) {
	m := NewFailoverManager()
	m.SetActiveNode("node-2")
	state := m.GetState()
	if state.ActiveNode != "node-2" {
		t.Errorf("expected node-2, got %q", state.ActiveNode)
	}
}
