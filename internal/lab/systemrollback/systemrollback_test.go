package systemrollback

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager(RollbackConfig{
		SnapshotRoot:    "/snapshots",
		SystemRoot:      "/",
		CompressDefault: "zstd",
	})
}

func TestCreateSnapshot(t *testing.T) {
	m := newTestManager()
	snap := &SystemSnapshot{
		ID:          "snap-1",
		Name:        "系统初始快照",
		Description: "安装后首次快照",
		Type:        SnapshotManual,
	}
	if err := m.CreateSnapshot(snap); err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	got, err := m.GetSnapshot("snap-1")
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	if got.Status != StatusReady {
		t.Errorf("status = %q, want %q", got.Status, StatusReady)
	}
}

func TestDuplicateSnapshot(t *testing.T) {
	m := newTestManager()
	snap := &SystemSnapshot{ID: "s1", Name: "test"}
	_ = m.CreateSnapshot(snap)
	if err := m.CreateSnapshot(snap); err != ErrSnapshotExists {
		t.Errorf("expected ErrSnapshotExists, got %v", err)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	m := newTestManager()
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s1", Name: "test"})
	if err := m.DeleteSnapshot("s1"); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}
	if _, err := m.GetSnapshot("s1"); err != ErrSnapshotNotFound {
		t.Errorf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestListSnapshots(t *testing.T) {
	m := newTestManager()
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s1", Name: "手动", Type: SnapshotManual})
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s2", Name: "自动", Type: SnapshotAuto})
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s3", Name: "更新前", Type: SnapshotPreUpdate})

	manual := m.ListSnapshots(SnapshotManual, 0)
	if len(manual) != 1 {
		t.Errorf("manual snapshots = %d, want 1", len(manual))
	}
	all := m.ListSnapshots("", 0)
	if len(all) != 3 {
		t.Errorf("all snapshots = %d, want 3", len(all))
	}
}

func TestRollback(t *testing.T) {
	m := newTestManager()
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s1", Name: "test", Size: 1024})
	req := RollbackRequest{
		SnapshotID:    "s1",
		BackupCurrent: true,
		RebootAfter:   false,
	}
	result, err := m.Rollback(req)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.BackupID == "" {
		t.Error("expected backup ID")
	}
	snap, _ := m.GetSnapshot("s1")
	if snap.RollbackCount != 1 {
		t.Errorf("RollbackCount = %d, want 1", snap.RollbackCount)
	}
}

func TestDryRunRollback(t *testing.T) {
	m := newTestManager()
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s1", Name: "test"})
	result, err := m.Rollback(RollbackRequest{SnapshotID: "s1", DryRun: true})
	if err != nil {
		t.Fatalf("DryRun failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success for dry run")
	}
}

func TestDiffSnapshots(t *testing.T) {
	m := newTestManager()
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s1", Name: "old"})
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s2", Name: "new"})
	diff, err := m.DiffSnapshots("s1", "s2")
	if err != nil {
		t.Fatalf("DiffSnapshots failed: %v", err)
	}
	if diff.Snapshot1 != "s1" || diff.Snapshot2 != "s2" {
		t.Errorf("unexpected diff: %v", diff)
	}
}

func TestPolicy(t *testing.T) {
	m := newTestManager()
	policy := &SnapshotPolicy{
		ID:            "p1",
		Name:          "每日快照",
		Schedule:      "0 2 * * *",
		SnapshotType:  SnapshotScheduled,
		MaxSnapshots:  7,
		RetentionDays: 30,
		AutoCleanup:   true,
		CompressType:  "zstd",
	}
	if err := m.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}
	list := m.ListPolicies()
	if len(list) != 1 {
		t.Errorf("policies = %d, want 1", len(list))
	}
}

func TestStats(t *testing.T) {
	m := newTestManager()
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s1", Name: "test", Size: 1024, Type: SnapshotManual})
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s2", Name: "test2", Size: 2048, Type: SnapshotAuto})
	_ = m.CreatePolicy(&SnapshotPolicy{ID: "p1", Name: "policy", Enabled: true})
	stats := m.GetStats()
	if stats.TotalSnapshots != 2 {
		t.Errorf("TotalSnapshots = %d, want 2", stats.TotalSnapshots)
	}
	if stats.TotalSize != 3072 {
		t.Errorf("TotalSize = %d, want 3072", stats.TotalSize)
	}
	if stats.ActivePolicies != 1 {
		t.Errorf("ActivePolicies = %d, want 1", stats.ActivePolicies)
	}
}

func TestCleanupExpired(t *testing.T) {
	m := newTestManager()
	expired := time.Now().Add(-time.Hour)
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s1", Name: "old", ExpiresAt: &expired})
	_ = m.CreateSnapshot(&SystemSnapshot{ID: "s2", Name: "new"})
	cleaned := m.CleanupExpired()
	if cleaned != 1 {
		t.Errorf("cleaned = %d, want 1", cleaned)
	}
	if _, err := m.GetSnapshot("s1"); err != ErrSnapshotNotFound {
		t.Error("expected expired snapshot to be deleted")
	}
}
