package immutablebackup

import (
	"testing"
	"time"
)

func TestCreateBackup(t *testing.T) {
	m := NewManager()

	backup, err := m.Create("daily-backup", "Nightly data backup", "/data", "/backups/daily",
		24*time.Hour*30, true, []string{"production", "critical"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if backup.State != StateCreating {
		t.Errorf("expected state creating, got %s", backup.State)
	}
	if backup.Name != "daily-backup" {
		t.Errorf("expected name 'daily-backup', got %q", backup.Name)
	}
	if backup.Retention.Duration != 30*24*time.Hour {
		t.Errorf("expected 30d retention, got %v", backup.Retention.Duration)
	}
	if len(backup.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(backup.Tags))
	}
}

func TestCreateValidation(t *testing.T) {
	m := NewManager()

	// Empty name
	_, err := m.Create("", "desc", "/src", "/dst", time.Hour, false, nil)
	if err == nil {
		t.Error("expected error for empty name")
	}

	// Short retention
	_, err = m.Create("test", "desc", "/src", "/dst", time.Minute, false, nil)
	if err == nil {
		t.Error("expected error for retention < 1h")
	}
}

func TestSealBackup(t *testing.T) {
	m := NewManager()

	backup, _ := m.Create("test", "", "/src", "/dst", 24*time.Hour, false, nil)

	checksum := GenerateChecksum([]byte("test backup data"))
	err := m.Seal(backup.ID, checksum)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	s, _ := m.Get(backup.ID)
	if s.State != StateSealed {
		t.Errorf("expected state sealed, got %s", s.State)
	}
	if s.SealedAt.IsZero() {
		t.Error("expected SealedAt to be set")
	}
	if s.Checksum != checksum {
		t.Errorf("checksum mismatch")
	}
}

func TestSealNonCreating(t *testing.T) {
	m := NewManager()

	backup, _ := m.Create("test", "", "/src", "/dst", 24*time.Hour, false, nil)
	m.Seal(backup.ID, "abc123")

	// Double seal should fail
	err := m.Seal(backup.ID, "def456")
	if err == nil {
		t.Error("expected error for double seal")
	}
}

func TestVerifyIntegrity(t *testing.T) {
	m := NewManager()

	data := []byte("important data")
	checksum := GenerateChecksum(data)

	backup, _ := m.Create("verify-test", "", "/src", "/dst", 24*time.Hour, false, nil)
	m.Seal(backup.ID, checksum)

	// Correct checksum
	ok, err := m.Verify(backup.ID, checksum)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !ok {
		t.Error("expected verification to pass")
	}

	// Tampered checksum
	ok, err = m.Verify(backup.ID, "tampered_checksum")
	if err != nil {
		t.Fatalf("Verify with bad checksum failed: %v", err)
	}
	if ok {
		t.Error("expected verification to fail with tampered checksum")
	}
}

func TestDeleteBeforeExpiry(t *testing.T) {
	m := NewManager()

	backup, _ := m.Create("protected", "", "/src", "/dst", 24*time.Hour, false, nil)
	m.Seal(backup.ID, "abc123")

	// Should not be deletable before expiry
	err := m.Delete(backup.ID)
	if err == nil {
		t.Error("expected error for deleting sealed backup before expiry")
	}
}

func TestDeleteAfterExpiry(t *testing.T) {
	m := NewManager()

	backup, _ := m.Create("short-lived", "", "/src", "/dst", time.Hour, false, nil)
	m.Seal(backup.ID, "abc123")

	// Manually set expiry to the past to simulate expiration
	m.mu.Lock()
	b := m.backups[backup.ID]
	b.Retention.ExpiresAt = time.Now().Add(-time.Second)
	m.mu.Unlock()

	// Should be deletable after expiry
	err := m.Delete(backup.ID)
	if err != nil {
		t.Fatalf("Delete after expiry failed: %v", err)
	}

	s, _ := m.Get(backup.ID)
	if s.State != StateDestroyed {
		t.Errorf("expected state destroyed, got %s", s.State)
	}
}

func TestDeleteNonExistent(t *testing.T) {
	m := NewManager()

	err := m.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent backup")
	}
}

func TestExtendRetention(t *testing.T) {
	m := NewManager()

	backup, _ := m.Create("extendable", "", "/src", "/dst", 24*time.Hour, true, nil)
	m.Seal(backup.ID, "abc123")

	oldExpiry := backup.Retention.ExpiresAt

	err := m.ExtendRetention(backup.ID, 48*time.Hour)
	if err != nil {
		t.Fatalf("ExtendRetention failed: %v", err)
	}

	s, _ := m.Get(backup.ID)
	if !s.Retention.ExpiresAt.After(oldExpiry) {
		t.Error("expected extended expiry to be later")
	}
}

func TestExtendRetentionNotAllowed(t *testing.T) {
	m := NewManager()

	backup, _ := m.Create("fixed", "", "/src", "/dst", 24*time.Hour, false, nil)
	m.Seal(backup.ID, "abc123")

	err := m.ExtendRetention(backup.ID, 48*time.Hour)
	if err == nil {
		t.Error("expected error when extending non-extendable retention")
	}
}

func TestExpire(t *testing.T) {
	m := NewManager()

	backup, _ := m.Create("will-expire", "", "/src", "/dst", time.Hour, false, nil)
	m.Seal(backup.ID, "abc123")

	// Manually set expiry to the past to simulate expiration
	m.mu.Lock()
	b := m.backups[backup.ID]
	b.Retention.ExpiresAt = time.Now().Add(-time.Second)
	m.mu.Unlock()

	count := m.Expire()
	if count != 1 {
		t.Errorf("expected 1 expired, got %d", count)
	}

	s, _ := m.Get(backup.ID)
	if s.State != StateExpired {
		t.Errorf("expected state expired, got %s", s.State)
	}
}

func TestList(t *testing.T) {
	m := NewManager()

	b1, _ := m.Create("b1", "", "/a", "/b", 24*time.Hour, false, nil)
	m.Create("b2", "", "/c", "/d", 24*time.Hour, false, nil)
	m.Create("b3", "", "/e", "/f", 24*time.Hour, false, nil)
	m.Seal(b1.ID, "abc")

	creating := m.List(StateCreating)
	if len(creating) != 2 {
		t.Errorf("expected 2 creating, got %d", len(creating))
	}

	sealed := m.List(StateSealed)
	if len(sealed) != 1 {
		t.Errorf("expected 1 sealed, got %d", len(sealed))
	}

	all := m.List("")
	if len(all) != 3 {
		t.Errorf("expected 3 total, got %d", len(all))
	}
}

func TestAuditLog(t *testing.T) {
	m := NewManager()

	backup, _ := m.Create("audited", "", "/src", "/dst", 24*time.Hour, false, nil)
	m.Seal(backup.ID, "abc123")

	log := m.GetAuditLog()
	if len(log) < 2 {
		t.Errorf("expected at least 2 audit entries, got %d", len(log))
	}

	// Check audit entries
	foundCreate, foundSeal := false, false
	for _, entry := range log {
		if entry.Action == "create" && entry.Success {
			foundCreate = true
		}
		if entry.Action == "seal" && entry.Success {
			foundSeal = true
		}
	}
	if !foundCreate {
		t.Error("expected 'create' audit entry")
	}
	if !foundSeal {
		t.Error("expected 'seal' audit entry")
	}
}

func TestVerifyNonSealedBackup(t *testing.T) {
	m := NewManager()

	backup, _ := m.Create("creating", "", "/src", "/dst", 24*time.Hour, false, nil)

	_, err := m.Verify(backup.ID, "abc")
	if err == nil {
		t.Error("expected error when verifying non-sealed backup")
	}
}

func TestChecksumConsistency(t *testing.T) {
	data1 := []byte("hello world")
	data2 := []byte("hello world")

	c1 := GenerateChecksum(data1)
	c2 := GenerateChecksum(data2)

	if c1 != c2 {
		t.Errorf("same data should produce same checksum: %s != %s", c1, c2)
	}

	data3 := []byte("hello world!")
	c3 := GenerateChecksum(data3)
	if c1 == c3 {
		t.Error("different data should produce different checksums")
	}
}
