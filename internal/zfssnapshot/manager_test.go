package zfssnapshot

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager(slog.Default())
}

func TestCreatePolicy(t *testing.T) {
	m := newTestManager()
	policy := &SnapshotPolicy{
		ID:       "policy-1",
		Name:     "每小时快照",
		Enabled:  true,
		Datasets: []string{"pool/data"},
		Schedule: "0 * * * *",
		RetentionPolicy: RetentionPolicy{
			Hourly:  24,
			Daily:   7,
			Weekly:  4,
			MaxTotal: 100,
		},
		Recursive: true,
	}

	if err := m.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	got, err := m.GetPolicy("policy-1")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if got.Name != "每小时快照" {
		t.Errorf("expected name '每小时快照', got '%s'", got.Name)
	}
}

func TestDuplicatePolicy(t *testing.T) {
	m := newTestManager()
	policy := &SnapshotPolicy{
		ID:       "dup-1",
		Name:     "测试",
		Datasets: []string{"pool/test"},
	}
	_ = m.CreatePolicy(policy)

	err := m.CreatePolicy(policy)
	if err == nil {
		t.Fatal("expected error for duplicate policy")
	}
}

func TestDeletePolicy(t *testing.T) {
	m := newTestManager()
	policy := &SnapshotPolicy{
		ID:       "del-1",
		Name:     "待删除",
		Datasets: []string{"pool/test"},
	}
	_ = m.CreatePolicy(policy)

	if err := m.DeletePolicy("del-1"); err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	_, err := m.GetPolicy("del-1")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestCreateSnapshot(t *testing.T) {
	m := newTestManager()

	snap, err := m.CreateSnapshot("pool/data", "manual-1", []string{"test"})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if snap.State != StateActive {
		t.Errorf("expected state 'active', got '%s'", snap.State)
	}

	// 重复创建
	_, err = m.CreateSnapshot("pool/data", "manual-1", nil)
	if err == nil {
		t.Fatal("expected error for duplicate snapshot")
	}
}

func TestDeleteSnapshot(t *testing.T) {
	m := newTestManager()
	_, _ = m.CreateSnapshot("pool/data", "to-delete", nil)

	if err := m.DeleteSnapshot("pool/data@to-delete"); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}

	_, err := m.GetSnapshot("pool/data@to-delete")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestDeleteSnapshotWithClones(t *testing.T) {
	m := newTestManager()
	snap, _ := m.CreateSnapshot("pool/data", "with-clones", nil)
	snap.Clones = 2

	err := m.DeleteSnapshot("pool/data@with-clones")
	if err == nil {
		t.Fatal("expected error deleting snapshot with clones")
	}
}

func TestListSnapshots(t *testing.T) {
	m := newTestManager()
	_, _ = m.CreateSnapshot("pool/a", "snap1", nil)
	_, _ = m.CreateSnapshot("pool/a", "snap2", nil)
	_, _ = m.CreateSnapshot("pool/b", "snap3", nil)

	all := m.ListSnapshots("")
	if len(all) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(all))
	}

	filtered := m.ListSnapshots("pool/a")
	if len(filtered) != 2 {
		t.Errorf("expected 2 snapshots for pool/a, got %d", len(filtered))
	}
}

func TestStats(t *testing.T) {
	m := newTestManager()
	_, _ = m.CreateSnapshot("pool/data", "s1", nil)
	_, _ = m.CreateSnapshot("pool/data", "s2", nil)

	stats := m.GetStats()
	if stats.TotalSnapshots != 2 {
		t.Errorf("expected 2 total snapshots, got %d", stats.TotalSnapshots)
	}
}

func TestStartStop(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 重复启动
	if err := m.Start(ctx); err == nil {
		t.Fatal("expected error for double start")
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestListPolicies(t *testing.T) {
	m := newTestManager()
	_ = m.CreatePolicy(&SnapshotPolicy{ID: "p1", Name: "A", Datasets: []string{"d1"}})
	_ = m.CreatePolicy(&SnapshotPolicy{ID: "p2", Name: "B", Datasets: []string{"d2"}})

	policies := m.ListPolicies()
	if len(policies) != 2 {
		t.Errorf("expected 2 policies, got %d", len(policies))
	}
}

func TestUpdatePolicy(t *testing.T) {
	m := newTestManager()
	_ = m.CreatePolicy(&SnapshotPolicy{ID: "up1", Name: "原始", Datasets: []string{"d1"}})

	err := m.UpdatePolicy(&SnapshotPolicy{ID: "up1", Name: "更新后", Datasets: []string{"d1", "d2"}})
	if err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}

	got, _ := m.GetPolicy("up1")
	if got.Name != "更新后" {
		t.Errorf("expected name '更新后', got '%s'", got.Name)
	}
}

func TestUpdateNonexistentPolicy(t *testing.T) {
	m := newTestManager()
	err := m.UpdatePolicy(&SnapshotPolicy{ID: "ghost", Name: "不存在"})
	if err == nil {
		t.Fatal("expected error updating nonexistent policy")
	}
}

func TestSnapshotExpiration(t *testing.T) {
	m := newTestManager()
	expired := time.Now().Add(-1 * time.Hour)
	snap, _ := m.CreateSnapshot("pool/data", "expired-snap", nil)
	snap.ExpiresAt = &expired

	// 触发清理
	m.cleanupExpired(context.Background())

	_, err := m.GetSnapshot("pool/data@expired-snap")
	if err == nil {
		t.Fatal("expected expired snapshot to be cleaned up")
	}
}
