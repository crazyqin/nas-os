package datamigration

import (
	"testing"
)

func TestCreateMigration(t *testing.T) {
	m := NewManager("/tmp/test_migration.json")
	m.Initialize()

	req := CreateMigrationRequest{
		Name:       "测试迁移",
		SourceType: "local",
		Source:     Source{Type: "local", Path: "/data/source"},
		TargetType: "local",
		Target:     Target{Type: "local", Path: "/data/target"},
		Options:    MigrationOptions{Parallel: 4, Verify: true},
	}

	migration, err := m.CreateMigration(req)
	if err != nil {
		t.Fatalf("CreateMigration failed: %v", err)
	}
	if migration.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", migration.Status)
	}
}

func TestStartMigration(t *testing.T) {
	m := NewManager("/tmp/test_migration.json")
	m.Initialize()

	migration, _ := m.CreateMigration(CreateMigrationRequest{
		Name:       "测试",
		SourceType: "local",
		Source:     Source{Type: "local", Path: "/src"},
		TargetType: "local",
		Target:     Target{Type: "local", Path: "/dst"},
	})

	err := m.StartMigration(migration.ID)
	if err != nil {
		t.Fatalf("StartMigration failed: %v", err)
	}

	updated, _ := m.GetMigration(migration.ID)
	if updated.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", updated.Status)
	}
}

func TestPauseResumeMigration(t *testing.T) {
	m := NewManager("/tmp/test_migration.json")
	m.Initialize()

	migration, _ := m.CreateMigration(CreateMigrationRequest{
		Name:       "测试",
		SourceType: "local",
		Source:     Source{Type: "local", Path: "/src"},
		TargetType: "local",
		Target:     Target{Type: "local", Path: "/dst"},
	})

	m.StartMigration(migration.ID)

	m.PauseMigration(migration.ID)
	paused, _ := m.GetMigration(migration.ID)
	if paused.Status != "paused" {
		t.Errorf("Expected status 'paused', got '%s'", paused.Status)
	}

	m.ResumeMigration(migration.ID)
	resumed, _ := m.GetMigration(migration.ID)
	if resumed.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", resumed.Status)
	}
}

func TestCancelMigration(t *testing.T) {
	m := NewManager("/tmp/test_migration.json")
	m.Initialize()

	migration, _ := m.CreateMigration(CreateMigrationRequest{
		Name:       "测试",
		SourceType: "local",
		Source:     Source{Type: "local", Path: "/src"},
		TargetType: "local",
		Target:     Target{Type: "local", Path: "/dst"},
	})

	m.StartMigration(migration.ID)
	m.CancelMigration(migration.ID)

	cancelled, _ := m.GetMigration(migration.ID)
	if cancelled.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got '%s'", cancelled.Status)
	}
}

func TestCompleteMigration(t *testing.T) {
	m := NewManager("/tmp/test_migration.json")
	m.Initialize()

	migration, _ := m.CreateMigration(CreateMigrationRequest{
		Name:       "测试",
		SourceType: "local",
		Source:     Source{Type: "local", Path: "/src"},
		TargetType: "local",
		Target:     Target{Type: "local", Path: "/dst"},
	})

	m.StartMigration(migration.ID)
	m.CompleteMigration(migration.ID)

	completed, _ := m.GetMigration(migration.ID)
	if completed.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", completed.Status)
	}
	if completed.Progress.Percent != 100 {
		t.Errorf("Expected 100%% progress, got %.2f%%", completed.Progress.Percent)
	}
}

func TestUpdateProgress(t *testing.T) {
	m := NewManager("/tmp/test_migration.json")
	m.Initialize()

	migration, _ := m.CreateMigration(CreateMigrationRequest{
		Name:       "测试",
		SourceType: "local",
		Source:     Source{Type: "local", Path: "/src"},
		TargetType: "local",
		Target:     Target{Type: "local", Path: "/dst"},
	})

	m.StartMigration(migration.ID)

	m.UpdateProgress(migration.ID, Progress{
		TotalFiles:     100,
		CompletedFiles: 50,
		TotalBytes:     1000000,
		CompletedBytes: 500000,
		Speed:          10000,
		CurrentFile:    "file50.txt",
	})

	updated, _ := m.GetMigration(migration.ID)
	if updated.Progress.Percent != 50 {
		t.Errorf("Expected 50%% progress, got %.2f%%", updated.Progress.Percent)
	}
}

func TestListMigrations(t *testing.T) {
	m := NewManager("/tmp/test_migration.json")
	m.Initialize()

	m.CreateMigration(CreateMigrationRequest{
		Name: "任务1", SourceType: "local",
		Source: Source{Type: "local", Path: "/a"}, TargetType: "local",
		Target: Target{Type: "local", Path: "/b"},
	})
	m.CreateMigration(CreateMigrationRequest{
		Name: "任务2", SourceType: "local",
		Source: Source{Type: "local", Path: "/c"}, TargetType: "local",
		Target: Target{Type: "local", Path: "/d"},
	})

	tasks := m.ListMigrations("")
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks))
	}
}

func TestGetPlans(t *testing.T) {
	m := NewManager("/tmp/test_migration.json")
	m.Initialize()

	plans := m.ListPlans()
	if len(plans) < 3 {
		t.Errorf("Expected at least 3 default plans, got %d", len(plans))
	}
}
