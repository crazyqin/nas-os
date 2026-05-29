package drivesync

import (
	"testing"
	"time"
)

func TestNewDriveSyncManager(t *testing.T) {
	mgr := NewDriveSyncManager(nil)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestCreateAndGetTask(t *testing.T) {
	mgr := NewDriveSyncManager(nil)

	task := &SyncTask{
		ID:         "task1",
		Name:       "My Documents",
		LocalPath:  "/home/user/docs",
		RemotePath: "/docs",
	}

	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, exists := mgr.GetTask("task1")
	if !exists {
		t.Fatal("expected task to exist")
	}
	if got.Name != "My Documents" {
		t.Errorf("expected name 'My Documents', got %q", got.Name)
	}
}

func TestCreateDuplicateTask(t *testing.T) {
	mgr := NewDriveSyncManager(nil)

	task1 := &SyncTask{
		ID:         "task1",
		LocalPath:  "/home/user/docs",
		RemotePath: "/docs",
	}

	task2 := &SyncTask{
		ID:         "task2",
		LocalPath:  "/home/user/docs",  // 相同路径
		RemotePath: "/docs",
	}

	mgr.CreateTask(task1)

	if err := mgr.CreateTask(task2); err == nil {
		t.Error("expected error for duplicate paths")
	}
}

func TestDeleteTask(t *testing.T) {
	mgr := NewDriveSyncManager(nil)

	task := &SyncTask{
		ID:         "task1",
		LocalPath:  "/home/user/docs",
		RemotePath: "/docs",
	}

	mgr.CreateTask(task)

	if err := mgr.DeleteTask("task1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists := mgr.GetTask("task1")
	if exists {
		t.Error("expected task to be deleted")
	}
}

func TestStartAndCompleteSync(t *testing.T) {
	mgr := NewDriveSyncManager(nil)

	task := &SyncTask{
		ID:         "task1",
		LocalPath:  "/home/user/docs",
		RemotePath: "/docs",
	}

	mgr.CreateTask(task)

	// 开始同步
	if err := mgr.StartSync("task1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := mgr.GetTask("task1")
	if got.Status != SyncStatusSyncing {
		t.Errorf("expected syncing status, got %v", got.Status)
	}

	// 完成同步
	if err := mgr.CompleteSync("task1", 10, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ = mgr.GetTask("task1")
	if got.Status != SyncStatusSynced {
		t.Errorf("expected synced status, got %v", got.Status)
	}
	if got.SyncedCount != 10 {
		t.Errorf("expected 10 synced files, got %d", got.SyncedCount)
	}
}

func TestPauseAndResumeSync(t *testing.T) {
	mgr := NewDriveSyncManager(nil)

	task := &SyncTask{
		ID:         "task1",
		LocalPath:  "/home/user/docs",
		RemotePath: "/docs",
	}

	mgr.CreateTask(task)

	// 暂停
	if err := mgr.PauseSync("task1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := mgr.GetTask("task1")
	if got.Status != SyncStatusPaused {
		t.Errorf("expected paused status, got %v", got.Status)
	}

	// 恢复
	if err := mgr.ResumeSync("task1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ = mgr.GetTask("task1")
	if got.Status != SyncStatusPending {
		t.Errorf("expected pending status, got %v", got.Status)
	}
}

func TestAddFileVersion(t *testing.T) {
	mgr := NewDriveSyncManager(nil)

	version := &FileVersion{
		VersionID:  "v1",
		FilePath:   "/docs/file.txt",
		Size:       1024,
		Checksum:   "abc123",
		ModifiedBy: "user1",
		ModifiedAt: time.Now(),
	}

	if err := mgr.AddFileVersion("/docs/file.txt", version); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	versions := mgr.GetFileVersions("/docs/file.txt")
	if len(versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(versions))
	}
}

func TestGetStats(t *testing.T) {
	mgr := NewDriveSyncManager(nil)

	mgr.CreateTask(&SyncTask{
		ID:         "task1",
		LocalPath:  "/home/user/docs",
		RemotePath: "/docs",
	})

	stats := mgr.GetStats()
	totalTasks := stats["total_tasks"].(int)
	if totalTasks != 1 {
		t.Errorf("expected 1 task, got %d", totalTasks)
	}
}
