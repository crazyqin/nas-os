package appmigrate

import (
	"testing"
	"time"
)

func TestMigrationManager_StartMigration(t *testing.T) {
	mm := NewMigrationManager()

	mm.RegisterApp(&App{
		ID:        "app-001",
		Name:      "Nextcloud",
		PoolID:    "pool-hdd",
		SizeBytes: 10 * 1024 * 1024 * 1024,
	})

	task, err := mm.StartMigration("app-001", "pool-ssd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Status != StatusRunning {
		t.Errorf("expected running, got %q", task.Status)
	}
	if task.SourcePool != "pool-hdd" {
		t.Errorf("expected source pool-hdd, got %q", task.SourcePool)
	}
}

func TestMigrationManager_DuplicatePoolError(t *testing.T) {
	mm := NewMigrationManager()

	mm.RegisterApp(&App{ID: "app-001", Name: "Test", PoolID: "pool-ssd"})

	_, err := mm.StartMigration("app-001", "pool-ssd")
	if err == nil {
		t.Error("expected error for same pool migration")
	}
}

func TestMigrationManager_ConcurrentMigrationBlock(t *testing.T) {
	mm := NewMigrationManager()

	mm.RegisterApp(&App{ID: "app-001", Name: "Test", PoolID: "pool-a", SizeBytes: 1024})

	mm.StartMigration("app-001", "pool-b")

	_, err := mm.StartMigration("app-001", "pool-c")
	if err == nil {
		t.Error("expected error for concurrent migration")
	}
}

func TestMigrationManager_UpdateProgress(t *testing.T) {
	mm := NewMigrationManager()

	mm.RegisterApp(&App{ID: "app-001", Name: "Test", PoolID: "pool-a", SizeBytes: 1000})
	task, _ := mm.StartMigration("app-001", "pool-b")

	mm.UpdateProgress(task.ID, 50, 500)
	task, _ = mm.GetTask(task.ID)
	if task.Progress != 50 {
		t.Errorf("expected 50%% progress, got %f", task.Progress)
	}

	mm.UpdateProgress(task.ID, 100, 1000)
	task, _ = mm.GetTask(task.ID)
	if task.Status != StatusCompleted {
		t.Errorf("expected completed, got %q", task.Status)
	}

	// 验证应用已迁移到新池
	app, _ := mm.GetApp("app-001")
	if app.PoolID != "pool-b" {
		t.Errorf("expected pool-b, got %q", app.PoolID)
	}
}

func TestMigrationManager_Rollback(t *testing.T) {
	mm := NewMigrationManager()

	mm.RegisterApp(&App{ID: "app-001", Name: "Test", PoolID: "pool-a", SizeBytes: 1000})
	task, _ := mm.StartMigration("app-001", "pool-b")
	mm.UpdateProgress(task.ID, 100, 1000)

	mm.Rollback(task.ID)
	task, _ = mm.GetTask(task.ID)
	if task.Status != StatusRolledBack {
		t.Errorf("expected rolled_back, got %q", task.Status)
	}

	app, _ := mm.GetApp("app-001")
	if app.PoolID != "pool-a" {
		t.Errorf("expected pool-a after rollback, got %q", app.PoolID)
	}
}

func TestMigrationManager_ListTasks(t *testing.T) {
	mm := NewMigrationManager()

	mm.RegisterApp(&App{ID: "a1", Name: "App1", PoolID: "p1", SizeBytes: 100})
	mm.RegisterApp(&App{ID: "a2", Name: "App2", PoolID: "p1", SizeBytes: 200})

	mm.StartMigration("a1", "p2")
	mm.StartMigration("a2", "p2")

	all := mm.ListTasks("")
	if len(all) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(all))
	}

	running := mm.ListTasks(StatusRunning)
	if len(running) != 2 {
		t.Errorf("expected 2 running, got %d", len(running))
	}
}

func TestMigrationManager_EstimateTime(t *testing.T) {
	mm := NewMigrationManager()

	mm.RegisterApp(&App{ID: "a1", Name: "Big", PoolID: "p1", SizeBytes: 1024 * 1024 * 1024}) // 1GB

	dur, err := mm.EstimateMigrationTime("a1", 100) // 100 MB/s
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Duration(10) * time.Second // 1GB / 100MB/s ≈ 10s
	if dur != expected {
		t.Errorf("expected %v, got %v", expected, dur)
	}
}

func TestMigrationManager_NotFound(t *testing.T) {
	mm := NewMigrationManager()

	_, err := mm.StartMigration("nonexistent", "pool-b")
	if err == nil {
		t.Error("expected error for nonexistent app")
	}
}
