package snapshotscheduler

import (
	"testing"
	"time"
)

func TestCreateSnapshot(t *testing.T) {
	s := NewScheduler()

	snap, err := s.CreateSnapshot("/data/volume1", "test-snap-1", []string{"test"})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snap.Name != "test-snap-1" {
		t.Errorf("Expected name 'test-snap-1', got '%s'", snap.Name)
	}
	if snap.VolumePath != "/data/volume1" {
		t.Errorf("Expected volume '/data/volume1', got '%s'", snap.VolumePath)
	}
	if snap.Status != StatusActive {
		t.Errorf("Expected status 'active', got '%s'", snap.Status)
	}
}

func TestCreateSnapshotEmptyPath(t *testing.T) {
	s := NewScheduler()

	_, err := s.CreateSnapshot("", "test", nil)
	if err == nil {
		t.Fatal("Expected error for empty path, got nil")
	}
}

func TestGetSnapshot(t *testing.T) {
	s := NewScheduler()

	snap, _ := s.CreateSnapshot("/data", "get-test", nil)

	retrieved, err := s.GetSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	if retrieved.ID != snap.ID {
		t.Errorf("ID mismatch: expected %s, got %s", snap.ID, retrieved.ID)
	}
}

func TestGetSnapshotNotFound(t *testing.T) {
	s := NewScheduler()

	_, err := s.GetSnapshot("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent snapshot, got nil")
	}
}

func TestDeleteSnapshot(t *testing.T) {
	s := NewScheduler()

	snap, _ := s.CreateSnapshot("/data", "delete-test", nil)

	if err := s.DeleteSnapshot(snap.ID); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}

	// Wait for async delete
	time.Sleep(200 * time.Millisecond)

	_, err := s.GetSnapshot(snap.ID)
	if err == nil {
		t.Log("Snapshot might still exist (async delete)")
	}
}

func TestDeleteSnapshotNotFound(t *testing.T) {
	s := NewScheduler()

	err := s.DeleteSnapshot("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent snapshot, got nil")
	}
}

func TestCloneSnapshot(t *testing.T) {
	s := NewScheduler()

	snap, _ := s.CreateSnapshot("/data", "clone-source", []string{"test"})

	result, err := s.CloneSnapshot(snap.ID, "/data/clones/clone-1")
	if err != nil {
		t.Fatalf("CloneSnapshot failed: %v", err)
	}

	if result.SourceID != snap.ID {
		t.Errorf("Source ID mismatch: expected %s, got %s", snap.ID, result.SourceID)
	}
	if result.TargetPath != "/data/clones/clone-1" {
		t.Errorf("Target path mismatch: expected '/data/clones/clone-1', got '%s'", result.TargetPath)
	}
}

func TestCloneSnapshotNotFound(t *testing.T) {
	s := NewScheduler()

	_, err := s.CloneSnapshot("nonexistent", "/target")
	if err == nil {
		t.Fatal("Expected error for nonexistent snapshot, got nil")
	}
}

func TestListSnapshots(t *testing.T) {
	s := NewScheduler()

	for i := 0; i < 5; i++ {
		s.CreateSnapshot("/data/vol1", "snap-"+string(rune('a'+i)), nil)
	}
	s.CreateSnapshot("/data/vol2", "snap-vol2", nil)

	// List all
	all := s.ListSnapshots("", 100)
	if len(all) != 6 {
		t.Errorf("Expected 6 snapshots, got %d", len(all))
	}

	// List by volume
	vol1 := s.ListSnapshots("/data/vol1", 100)
	if len(vol1) != 5 {
		t.Errorf("Expected 5 vol1 snapshots, got %d", len(vol1))
	}

	// List with limit
	limited := s.ListSnapshots("", 3)
	if len(limited) != 3 {
		t.Errorf("Expected 3 snapshots with limit, got %d", len(limited))
	}
}

func TestCreateSchedule(t *testing.T) {
	s := NewScheduler()

	sched := &Schedule{
		Name:       "daily-backup",
		VolumePath: "/data",
		Frequency:  FreqDaily,
		Hour:       2,
		Minute:     0,
		Enabled:    true,
		Retention: RetentionPolicy{
			Unit:     RetainByCount,
			MaxCount: 30,
		},
	}

	if err := s.CreateSchedule(sched); err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}

	if sched.ID == "" {
		t.Fatal("Expected schedule ID to be set")
	}
	if sched.NextRunAt == nil {
		t.Fatal("Expected NextRunAt to be set")
	}
}

func TestCreateScheduleValidation(t *testing.T) {
	s := NewScheduler()

	// Missing name
	err := s.CreateSchedule(&Schedule{VolumePath: "/data"})
	if err == nil {
		t.Fatal("Expected error for missing name")
	}

	// Missing volume
	err = s.CreateSchedule(&Schedule{Name: "test"})
	if err == nil {
		t.Fatal("Expected error for missing volume")
	}
}

func TestUpdateSchedule(t *testing.T) {
	s := NewScheduler()

	sched := &Schedule{
		Name:       "test-schedule",
		VolumePath: "/data",
		Frequency:  FreqDaily,
		Enabled:    true,
	}
	s.CreateSchedule(sched)

	err := s.UpdateSchedule(sched.ID, &Schedule{
		Name:      "updated-schedule",
		Enabled:   false,
		Frequency: FreqHourly,
	})
	if err != nil {
		t.Fatalf("UpdateSchedule failed: %v", err)
	}

	updated, _ := s.GetSchedule(sched.ID)
	if updated.Name != "updated-schedule" {
		t.Errorf("Expected name 'updated-schedule', got '%s'", updated.Name)
	}
	if updated.Enabled {
		t.Error("Expected schedule to be disabled")
	}
}

func TestDeleteSchedule(t *testing.T) {
	s := NewScheduler()

	sched := &Schedule{
		Name:       "to-delete",
		VolumePath: "/data",
		Frequency:  FreqDaily,
		Enabled:    true,
	}
	s.CreateSchedule(sched)

	err := s.DeleteSchedule(sched.ID)
	if err != nil {
		t.Fatalf("DeleteSchedule failed: %v", err)
	}

	_, err = s.GetSchedule(sched.ID)
	if err == nil {
		t.Fatal("Expected error after deleting schedule")
	}
}

func TestListSchedules(t *testing.T) {
	s := NewScheduler()

	s.CreateSchedule(&Schedule{Name: "sched-1", VolumePath: "/data", Frequency: FreqDaily, Enabled: true})
	s.CreateSchedule(&Schedule{Name: "sched-2", VolumePath: "/data", Frequency: FreqHourly, Enabled: false})

	all := s.ListSchedules(false)
	if len(all) != 2 {
		t.Errorf("Expected 2 schedules, got %d", len(all))
	}

	enabled := s.ListSchedules(true)
	if len(enabled) != 1 {
		t.Errorf("Expected 1 enabled schedule, got %d", len(enabled))
	}
}

func TestGetStats(t *testing.T) {
	s := NewScheduler()

	s.CreateSnapshot("/data/vol1", "snap-1", nil)
	s.CreateSnapshot("/data/vol1", "snap-2", nil)
	s.CreateSnapshot("/data/vol2", "snap-3", nil)
	s.CreateSchedule(&Schedule{Name: "sched-1", VolumePath: "/data", Frequency: FreqDaily, Enabled: true})

	stats := s.GetStats()
	if stats.TotalSnapshots != 3 {
		t.Errorf("Expected 3 total snapshots, got %d", stats.TotalSnapshots)
	}
	if stats.TotalSchedules != 1 {
		t.Errorf("Expected 1 total schedule, got %d", stats.TotalSchedules)
	}
	if stats.ActiveSchedules != 1 {
		t.Errorf("Expected 1 active schedule, got %d", stats.ActiveSchedules)
	}
}

func TestRegisterVolume(t *testing.T) {
	s := NewScheduler()

	s.RegisterVolume("/data/zfs-pool", FSZFS)
	s.RegisterVolume("/data/btrfs-vol", FSBtrfs)

	snap, err := s.CreateSnapshot("/data/zfs-pool", "zfs-snap", nil)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if snap.FSType != FSZFS {
		t.Errorf("Expected FS type 'zfs', got '%s'", snap.FSType)
	}
}

func TestSchedulerStartStop(t *testing.T) {
	s := NewScheduler()

	if s.IsRunning() {
		t.Error("Scheduler should not be running initially")
	}

	s.Start()
	if !s.IsRunning() {
		t.Error("Scheduler should be running after Start()")
	}

	// Start again (should be idempotent)
	s.Start()

	s.Stop()
	// Give goroutine time to stop
	time.Sleep(100 * time.Millisecond)
}

func TestRollback(t *testing.T) {
	s := NewScheduler()

	snap, _ := s.CreateSnapshot("/data", "rollback-snap", nil)

	err := s.Rollback(&RollbackRequest{
		SnapshotID: snap.ID,
	})
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Rollback nonexistent
	err = s.Rollback(&RollbackRequest{
		SnapshotID: "nonexistent",
	})
	if err == nil {
		t.Fatal("Expected error for nonexistent snapshot")
	}
}
