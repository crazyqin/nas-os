package zfssnapshot

import (
	"testing"
	"time"
)

func TestNewZFSSnapshotManager(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)
	if mgr == nil {
		t.Fatal("NewZFSSnapshotManager returned nil")
	}
}

func TestCreatePolicy(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	policy := &SnapshotPolicy{
		ID:       "policy1",
		Name:     "每日快照",
		Dataset:  "tank/data",
		Schedule: "0 2 * * *",
		MaxCount: 30,
		Enabled:  true,
	}

	err := mgr.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// 重复创建应失败
	err = mgr.CreatePolicy(policy)
	if err == nil {
		t.Error("expected error for duplicate policy")
	}
}

func TestGetPolicy(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	policy := &SnapshotPolicy{
		ID:   "policy1",
		Name: "每日快照",
	}
	mgr.CreatePolicy(policy)

	got, err := mgr.GetPolicy("policy1")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if got.Name != "每日快照" {
		t.Errorf("expected 每日快照, got %s", got.Name)
	}

	_, err = mgr.GetPolicy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestListPolicies(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	mgr.CreatePolicy(&SnapshotPolicy{ID: "p1", Name: "策略1"})
	mgr.CreatePolicy(&SnapshotPolicy{ID: "p2", Name: "策略2"})

	policies := mgr.ListPolicies()
	if len(policies) != 2 {
		t.Errorf("expected 2 policies, got %d", len(policies))
	}
}

func TestDeletePolicy(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	mgr.CreatePolicy(&SnapshotPolicy{ID: "p1", Name: "策略1"})

	err := mgr.DeletePolicy("p1")
	if err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	_, err = mgr.GetPolicy("p1")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestCreateSnapshot(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	snap, err := mgr.CreateSnapshot("tank/data", "snap1", []string{"daily"})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snap.Dataset != "tank/data" {
		t.Errorf("expected tank/data, got %s", snap.Dataset)
	}

	if snap.FullName != "tank/data@snap1" {
		t.Errorf("expected tank/data@snap1, got %s", snap.FullName)
	}

	// 重复创建应失败
	_, err = mgr.CreateSnapshot("tank/data", "snap1", nil)
	if err == nil {
		t.Error("expected error for duplicate snapshot")
	}
}

func TestGetSnapshot(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	mgr.CreateSnapshot("tank/data", "snap1", nil)

	snap, err := mgr.GetSnapshot("tank/data@snap1")
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	if snap.Name != "snap1" {
		t.Errorf("expected snap1, got %s", snap.Name)
	}
}

func TestListSnapshots(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	mgr.CreateSnapshot("tank/data", "snap1", nil)
	mgr.CreateSnapshot("tank/data", "snap2", nil)
	mgr.CreateSnapshot("tank/backup", "snap3", nil)

	// 列出所有
	all := mgr.ListSnapshots("")
	if len(all) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(all))
	}

	// 按数据集过滤
	dataSnaps := mgr.ListSnapshots("tank/data")
	if len(dataSnaps) != 2 {
		t.Errorf("expected 2 snapshots for tank/data, got %d", len(dataSnaps))
	}
}

func TestDeleteSnapshot(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	mgr.CreateSnapshot("tank/data", "snap1", nil)

	err := mgr.DeleteSnapshot("tank/data@snap1")
	if err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}

	_, err = mgr.GetSnapshot("tank/data@snap1")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestCloneSnapshot(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	mgr.CreateSnapshot("tank/data", "snap1", nil)

	clone, err := mgr.CloneSnapshot("tank/data@snap1", "tank/clone1")
	if err != nil {
		t.Fatalf("CloneSnapshot failed: %v", err)
	}

	if !clone.IsClone {
		t.Error("expected clone to be marked as clone")
	}

	if clone.Origin != "tank/data@snap1" {
		t.Errorf("expected origin tank/data@snap1, got %s", clone.Origin)
	}

	// 验证原快照有克隆记录
	orig, _ := mgr.GetSnapshot("tank/data@snap1")
	if len(orig.Clones) != 1 {
		t.Errorf("expected 1 clone, got %d", len(orig.Clones))
	}
}

func TestRollbackSnapshot(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	mgr.CreateSnapshot("tank/data", "snap1", nil)

	err := mgr.RollbackSnapshot("tank/data@snap1")
	if err != nil {
		t.Fatalf("RollbackSnapshot failed: %v", err)
	}
}

func TestCreateReplication(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	rep := &SnapshotReplication{
		ID:            "rep1",
		SourcePool:    "tank",
		TargetPool:    "backup",
		SourceDataset: "tank/data",
		TargetDataset: "backup/data",
		SnapshotName:  "snap1",
		Encrypted:     true,
	}

	err := mgr.CreateReplication(rep)
	if err != nil {
		t.Fatalf("CreateReplication failed: %v", err)
	}

	// 重复创建应失败
	err = mgr.CreateReplication(rep)
	if err == nil {
		t.Error("expected error for duplicate replication")
	}
}

func TestGetReplication(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	rep := &SnapshotReplication{ID: "rep1"}
	mgr.CreateReplication(rep)

	got, err := mgr.GetReplication("rep1")
	if err != nil {
		t.Fatalf("GetReplication failed: %v", err)
	}
	if got.Status != "pending" {
		t.Errorf("expected pending, got %s", got.Status)
	}
}

func TestListReplications(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	mgr.CreateReplication(&SnapshotReplication{ID: "rep1"})
	mgr.CreateReplication(&SnapshotReplication{ID: "rep2"})

	reps := mgr.ListReplications()
	if len(reps) != 2 {
		t.Errorf("expected 2 replications, got %d", len(reps))
	}
}

func TestAnalyzeSpace(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	mgr.CreateSnapshot("tank/data", "snap1", nil)
	mgr.CreateSnapshot("tank/data", "snap2", nil)

	analysis := mgr.AnalyzeSpace("tank/data")
	if analysis.SnapshotCount != 2 {
		t.Errorf("expected 2 snapshots, got %d", analysis.SnapshotCount)
	}
}

func TestCleanupExpired(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	policy := &SnapshotPolicy{
		ID:       "policy1",
		Dataset:  "tank/data",
		MaxCount: 2,
	}
	mgr.CreatePolicy(policy)

	// 创建3个快照
	mgr.CreateSnapshot("tank/data", "snap1", nil)
	time.Sleep(10 * time.Millisecond)
	mgr.CreateSnapshot("tank/data", "snap2", nil)
	time.Sleep(10 * time.Millisecond)
	mgr.CreateSnapshot("tank/data", "snap3", nil)

	removed, err := mgr.CleanupExpired("policy1")
	if err != nil {
		t.Fatalf("CleanupExpired failed: %v", err)
	}

	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
}

func TestMaxSnapshotsLimit(t *testing.T) {
	mgr := NewZFSSnapshotManager(2)

	mgr.CreateSnapshot("tank/data", "snap1", nil)
	mgr.CreateSnapshot("tank/data", "snap2", nil)

	_, err := mgr.CreateSnapshot("tank/data", "snap3", nil)
	if err == nil {
		t.Error("expected error when exceeding max snapshots")
	}
}

func TestGetStats(t *testing.T) {
	mgr := NewZFSSnapshotManager(100)

	mgr.CreatePolicy(&SnapshotPolicy{ID: "p1"})
	mgr.CreateSnapshot("tank/data", "snap1", nil)

	stats := mgr.GetStats()
	if stats["total_policies"] != 1 {
		t.Errorf("expected 1 policy, got %v", stats["total_policies"])
	}
	if stats["total_snapshots"] != 1 {
		t.Errorf("expected 1 snapshot, got %v", stats["total_snapshots"])
	}
}
