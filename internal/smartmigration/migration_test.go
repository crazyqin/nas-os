package smartmigration

import (
	"testing"
	"time"
)

func TestCreateMigration(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{
		Type: "local",
		Path: "/data/source",
	}

	dest := MigrationEndpoint{
		Type: "local",
		Path: "/data/dest",
	}

	opts := MigrationOptions{
		BandwidthLimit:    100,
		Compression:       true,
		Deduplication:     true,
		Encryption:        true,
		VerifyAfterCopy:   true,
		SyncMode:          "full",
		RetryCount:        3,
		ChunkSize:         64,
		ParallelTransfers: 4,
		PreservePerms:     true,
		PreserveTimestamps: true,
	}

	migration, err := m.CreateMigration("Test Migration", MigrationTypeDisk, source, dest, opts)
	if err != nil {
		t.Fatalf("Failed to create migration: %v", err)
	}

	if migration.Status != StatusPending {
		t.Errorf("Expected status pending, got %s", migration.Status)
	}

	if migration.Name != "Test Migration" {
		t.Errorf("Expected name 'Test Migration', got '%s'", migration.Name)
	}
}

func TestStartMigration(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data/source"}
	dest := MigrationEndpoint{Type: "local", Path: "/data/dest"}
	opts := MigrationOptions{SyncMode: "full"}

	migration, _ := m.CreateMigration("Test", MigrationTypeDisk, source, dest, opts)

	err := m.StartMigration(migration.ID)
	if err != nil {
		t.Fatalf("Failed to start migration: %v", err)
	}

	updated, _ := m.GetMigration(migration.ID)
	if updated.Status != StatusRunning {
		t.Errorf("Expected status running, got %s", updated.Status)
	}
}

func TestPauseResumeMigration(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data/source"}
	dest := MigrationEndpoint{Type: "local", Path: "/data/dest"}
	opts := MigrationOptions{SyncMode: "full"}

	migration, _ := m.CreateMigration("Test", MigrationTypeDisk, source, dest, opts)
	m.StartMigration(migration.ID)

	err := m.PauseMigration(migration.ID)
	if err != nil {
		t.Fatalf("Failed to pause migration: %v", err)
	}

	paused, _ := m.GetMigration(migration.ID)
	if paused.Status != StatusPaused {
		t.Errorf("Expected status paused, got %s", paused.Status)
	}

	err = m.ResumeMigration(migration.ID)
	if err != nil {
		t.Fatalf("Failed to resume migration: %v", err)
	}

	resumed, _ := m.GetMigration(migration.ID)
	if resumed.Status != StatusRunning {
		t.Errorf("Expected status running, got %s", resumed.Status)
	}
}

func TestCancelMigration(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data/source"}
	dest := MigrationEndpoint{Type: "local", Path: "/data/dest"}
	opts := MigrationOptions{SyncMode: "full"}

	migration, _ := m.CreateMigration("Test", MigrationTypeDisk, source, dest, opts)
	m.StartMigration(migration.ID)

	err := m.CancelMigration(migration.ID)
	if err != nil {
		t.Fatalf("Failed to cancel migration: %v", err)
	}

	cancelled, _ := m.GetMigration(migration.ID)
	if cancelled.Status != StatusCancelled {
		t.Errorf("Expected status cancelled, got %s", cancelled.Status)
	}

	if cancelled.EndTime == nil {
		t.Error("Expected end time to be set")
	}
}

func TestUpdateProgress(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data/source"}
	dest := MigrationEndpoint{Type: "local", Path: "/data/dest"}
	opts := MigrationOptions{SyncMode: "full"}

	migration, _ := m.CreateMigration("Test", MigrationTypeDisk, source, dest, opts)
	m.StartMigration(migration.ID)

	progress := MigrationProgress{
		TotalBytes:       1024 * 1024 * 1024,
		TransferredBytes: 512 * 1024 * 1024,
		TotalFiles:       100,
		TransferredFiles: 50,
		PercentComplete:  50,
		CurrentSpeed:     100,
		AverageSpeed:     95,
		CurrentFile:      "/data/file50.dat",
	}

	err := m.UpdateProgress(migration.ID, progress)
	if err != nil {
		t.Fatalf("Failed to update progress: %v", err)
	}

	updated, _ := m.GetMigration(migration.ID)
	if updated.Progress.PercentComplete != 50 {
		t.Errorf("Expected 50%% complete, got %f", updated.Progress.PercentComplete)
	}
}

func TestCompleteMigration(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data/source"}
	dest := MigrationEndpoint{Type: "local", Path: "/data/dest"}
	opts := MigrationOptions{SyncMode: "full"}

	migration, _ := m.CreateMigration("Test", MigrationTypeDisk, source, dest, opts)
	m.StartMigration(migration.ID)

	progress := MigrationProgress{
		TotalBytes:       1024 * 1024 * 1024,
		TransferredBytes: 1024 * 1024 * 1024,
		TotalFiles:       100,
		TransferredFiles: 100,
		PercentComplete:  100,
	}

	m.UpdateProgress(migration.ID, progress)

	updated, _ := m.GetMigration(migration.ID)
	if updated.Status != StatusCompleted {
		t.Errorf("Expected status completed, got %s", updated.Status)
	}

	if updated.EndTime == nil {
		t.Error("Expected end time to be set")
	}
}

func TestCreatePlan(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data/source"}
	dest := MigrationEndpoint{Type: "remote", Host: "nas2.local", Path: "/backup"}

	plan, err := m.CreatePlan("Backup Plan", MigrationTypeFull, source, dest)
	if err != nil {
		t.Fatalf("Failed to create plan: %v", err)
	}

	if len(plan.Steps) == 0 {
		t.Error("Expected plan steps")
	}

	if len(plan.Recommendations) == 0 {
		t.Error("Expected recommendations")
	}
}

func TestListMigrations(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data/source"}
	dest := MigrationEndpoint{Type: "local", Path: "/data/dest"}
	opts := MigrationOptions{SyncMode: "full"}

	m.CreateMigration("Migration 1", MigrationTypeDisk, source, dest, opts)
	m.CreateMigration("Migration 2", MigrationTypeVolume, source, dest, opts)

	all := m.ListMigrations("")
	if len(all) != 2 {
		t.Errorf("Expected 2 migrations, got %d", len(all))
	}

	pending := m.ListMigrations(StatusPending)
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending migrations, got %d", len(pending))
	}
}

func TestEstimateMigration(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data"}

	totalBytes, estimatedTime, err := m.EstimateMigration(source)
	if err != nil {
		t.Fatalf("Failed to estimate migration: %v", err)
	}

	if totalBytes <= 0 {
		t.Error("Expected positive total bytes")
	}

	if estimatedTime <= 0 {
		t.Error("Expected positive estimated time")
	}
}

func TestMigrationEndpointTypes(t *testing.T) {
	local := MigrationEndpoint{Type: "local", Path: "/data"}
	if local.Type != "local" {
		t.Errorf("Expected local type, got %s", local.Type)
	}

	remote := MigrationEndpoint{Type: "remote", Host: "nas2.local", Path: "/data", Protocol: "ssh", Port: 22}
	if remote.Host != "nas2.local" {
		t.Errorf("Expected host nas2.local, got %s", remote.Host)
	}

	cloud := MigrationEndpoint{Type: "cloud", Path: "s3://bucket/data"}
	if cloud.Type != "cloud" {
		t.Errorf("Expected cloud type, got %s", cloud.Type)
	}
}

func TestMigrationOptions(t *testing.T) {
	opts := MigrationOptions{
		BandwidthLimit:    100,
		Compression:       true,
		Deduplication:     true,
		Encryption:        true,
		VerifyAfterCopy:   true,
		SyncMode:          "incremental",
		RetryCount:        3,
		ChunkSize:         64,
		ParallelTransfers: 4,
	}

	if opts.BandwidthLimit != 100 {
		t.Errorf("Expected bandwidth limit 100, got %d", opts.BandwidthLimit)
	}

	if opts.SyncMode != "incremental" {
		t.Errorf("Expected sync mode incremental, got %s", opts.SyncMode)
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data/source"}
	dest := MigrationEndpoint{Type: "local", Path: "/data/dest"}
	opts := MigrationOptions{SyncMode: "full"}

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			m.CreateMigration("Concurrent Test", MigrationTypeDisk, source, dest, opts)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	migrations := m.ListMigrations("")
	if len(migrations) != 10 {
		t.Errorf("Expected 10 migrations, got %d", len(migrations))
	}
}

func TestGetNonExistentMigration(t *testing.T) {
	m := NewManager()

	_, err := m.GetMigration("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent migration")
	}
}

func TestStartNonPendingMigration(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data/source"}
	dest := MigrationEndpoint{Type: "local", Path: "/data/dest"}
	opts := MigrationOptions{SyncMode: "full"}

	migration, _ := m.CreateMigration("Test", MigrationTypeDisk, source, dest, opts)
	m.StartMigration(migration.ID)

	err := m.StartMigration(migration.ID)
	if err == nil {
		t.Error("Expected error when starting already running migration")
	}
}

func TestMigrationDuration(t *testing.T) {
	m := NewManager()

	source := MigrationEndpoint{Type: "local", Path: "/data/source"}
	dest := MigrationEndpoint{Type: "local", Path: "/data/dest"}
	opts := MigrationOptions{SyncMode: "full"}

	migration, _ := m.CreateMigration("Test", MigrationTypeDisk, source, dest, opts)
	m.StartMigration(migration.ID)

	time.Sleep(10 * time.Millisecond)

	progress := MigrationProgress{
		TotalBytes:       100,
		TransferredBytes: 100,
		PercentComplete:  100,
	}
	m.UpdateProgress(migration.ID, progress)

	updated, _ := m.GetMigration(migration.ID)
	if updated.Duration <= 0 {
		t.Error("Expected positive duration")
	}
}
