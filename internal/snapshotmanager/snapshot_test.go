package snapshotmanager

import (
	"context"
	"testing"
	"time"
)

func TestSnapshotManager_CreateSnapshot(t *testing.T) {
	config := &ManagerConfig{
		MaxSnapshots: 100,
		SnapshotDir:  "/tmp/snapshots",
	}

	sm := NewSnapshotManager(config)
	ctx := context.Background()

	// Test create snapshot
	snapshot, err := sm.CreateSnapshot(ctx, "vol1", "test-snapshot", "Test snapshot", map[string]string{"env": "test"})
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	if snapshot.ID == "" {
		t.Error("Snapshot ID should not be empty")
	}

	if snapshot.VolumeID != "vol1" {
		t.Errorf("Expected volume ID 'vol1', got '%s'", snapshot.VolumeID)
	}

	if snapshot.Name != "test-snapshot" {
		t.Errorf("Expected name 'test-snapshot', got '%s'", snapshot.Name)
	}

	// Wait for creation to complete
	time.Sleep(3 * time.Second)

	// Verify snapshot state
	snapshots := sm.ListSnapshots("vol1")
	if len(snapshots) != 1 {
		t.Fatalf("Expected 1 snapshot, got %d", len(snapshots))
	}

	if snapshots[0].State != SnapshotStateActive {
		t.Errorf("Expected state '%s', got '%s'", SnapshotStateActive, snapshots[0].State)
	}
}

func TestSnapshotManager_MaxSnapshotsLimit(t *testing.T) {
	config := &ManagerConfig{
		MaxSnapshots: 2,
		SnapshotDir:  "/tmp/snapshots",
	}

	sm := NewSnapshotManager(config)
	ctx := context.Background()

	// Create max snapshots
	for i := 0; i < 2; i++ {
		_, err := sm.CreateSnapshot(ctx, "vol1", "snapshot-"+string(rune(i)), "Test", nil)
		if err != nil {
			t.Fatalf("Failed to create snapshot %d: %v", i, err)
		}
	}

	// Try to create one more - should fail
	_, err := sm.CreateSnapshot(ctx, "vol1", "extra-snapshot", "Test", nil)
	if err == nil {
		t.Error("Expected error when exceeding max snapshots limit")
	}
}

func TestSnapshotManager_RestoreSnapshot(t *testing.T) {
	config := &ManagerConfig{
		MaxSnapshots: 100,
		SnapshotDir:  "/tmp/snapshots",
	}

	sm := NewSnapshotManager(config)
	ctx := context.Background()

	// Create and wait for snapshot
	snapshot, _ := sm.CreateSnapshot(ctx, "vol1", "restore-test", "Test", nil)
	time.Sleep(3 * time.Second)

	// Restore snapshot
	err := sm.RestoreSnapshot(ctx, snapshot.ID, "vol2")
	if err != nil {
		t.Fatalf("Failed to restore snapshot: %v", err)
	}

	// Wait for restore
	time.Sleep(6 * time.Second)

	// Verify state
	snapshots := sm.ListSnapshots("vol1")
	if snapshots[0].State != SnapshotStateActive {
		t.Errorf("Expected state '%s', got '%s'", SnapshotStateActive, snapshots[0].State)
	}
}

func TestSnapshotManager_DeleteSnapshot(t *testing.T) {
	config := &ManagerConfig{
		MaxSnapshots: 100,
		SnapshotDir:  "/tmp/snapshots",
	}

	sm := NewSnapshotManager(config)
	ctx := context.Background()

	// Create snapshot
	snapshot, _ := sm.CreateSnapshot(ctx, "vol1", "delete-test", "Test", nil)
	time.Sleep(3 * time.Second)

	// Delete snapshot
	err := sm.DeleteSnapshot(ctx, snapshot.ID)
	if err != nil {
		t.Fatalf("Failed to delete snapshot: %v", err)
	}

	// Wait for deletion
	time.Sleep(2 * time.Second)

	// Verify deletion
	snapshots := sm.ListSnapshots("vol1")
	if len(snapshots) != 0 {
		t.Errorf("Expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestSnapshotManager_CreatePolicy(t *testing.T) {
	config := &ManagerConfig{
		MaxSnapshots: 100,
		SnapshotDir:  "/tmp/snapshots",
	}

	sm := NewSnapshotManager(config)
	ctx := context.Background()

	policy := &SnapshotPolicy{
		Name:        "Daily Backup",
		Description: "Daily backup policy",
		VolumeIDs:   []string{"vol1", "vol2"},
		Schedule: &SnapshotSchedule{
			Frequency: "daily",
			Time:      "02:00",
			Enabled:   true,
		},
		Retention: &RetentionRule{
			KeepLast:    7,
			KeepDaily:   30,
			KeepWeekly:  12,
			KeepMonthly: 12,
		},
		Enabled: true,
	}

	err := sm.CreatePolicy(ctx, policy)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if policy.ID == "" {
		t.Error("Policy ID should not be empty")
	}

	stats := sm.GetSnapshotStats()
	if stats["total_policies"] != 1 {
		t.Errorf("Expected 1 policy, got %v", stats["total_policies"])
	}
}

func TestSnapshotManager_GetSnapshotStats(t *testing.T) {
	config := &ManagerConfig{
		MaxSnapshots: 100,
		SnapshotDir:  "/tmp/snapshots",
	}

	sm := NewSnapshotManager(config)
	ctx := context.Background()

	// Create some snapshots
	sm.CreateSnapshot(ctx, "vol1", "snap1", "Test", nil)
	sm.CreateSnapshot(ctx, "vol2", "snap2", "Test", nil)
	time.Sleep(3 * time.Second)

	stats := sm.GetSnapshotStats()
	if stats["total_snapshots"] != 2 {
		t.Errorf("Expected 2 snapshots, got %v", stats["total_snapshots"])
	}

	if _, ok := stats["by_state"]; !ok {
		t.Error("Stats should contain 'by_state'")
	}
}
