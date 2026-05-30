package backupverify

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	if manager.storagePath != tmpDir {
		t.Errorf("expected storage path %s, got %s", tmpDir, manager.storagePath)
	}
}

func TestCreateVerifyTask(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Test Task",
		BackupID:   "backup-001",
		BackupPath: "/tmp/test-backup",
		VerifyType: VerifyIntegrity,
		Schedule:   "0 2 * * *",
	}

	result, err := manager.CreateVerifyTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateVerifyTask failed: %v", err)
	}

	if result.ID == "" {
		t.Error("expected task ID to be set")
	}
	if result.Status != TaskStatusPending {
		t.Errorf("expected status pending, got %s", result.Status)
	}
	if !result.Enabled {
		t.Error("expected task to be enabled")
	}
}

func TestGetTask(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Test Task",
		BackupID:   "backup-001",
		BackupPath: "/tmp/test-backup",
		VerifyType: VerifyIntegrity,
	}

	created, err := manager.CreateVerifyTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateVerifyTask failed: %v", err)
	}

	// Get task
	result, err := manager.GetTask(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if result.ID != created.ID {
		t.Errorf("expected task ID %s, got %s", created.ID, result.ID)
	}
	if result.Name != "Test Task" {
		t.Errorf("expected name Test Task, got %s", result.Name)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	ctx := context.Background()

	_, err := manager.GetTask(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestRunVerify(t *testing.T) {
	// Create test backup directory with files
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	// Create test files
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, name := range testFiles {
		path := filepath.Join(backupDir, name)
		os.WriteFile(path, []byte("test content for "+name), 0644)
	}

	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Integrity Test",
		BackupID:   "backup-001",
		BackupPath: backupDir,
		VerifyType: VerifyIntegrity,
	}

	created, err := manager.CreateVerifyTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateVerifyTask failed: %v", err)
	}

	// Run verification
	result, err := manager.RunVerify(ctx, created.ID)
	if err != nil {
		t.Fatalf("RunVerify failed: %v", err)
	}

	if result.Status != ResultPass {
		t.Errorf("expected status pass, got %s", result.Status)
	}
	if result.FileCount != 3 {
		t.Errorf("expected 3 files, got %d", result.FileCount)
	}
	if result.VerifiedFiles != 3 {
		t.Errorf("expected 3 verified files, got %d", result.VerifiedFiles)
	}
	if result.Duration == 0 {
		t.Error("expected duration to be set")
	}
}

func TestRunVerifyNonexistentTask(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	ctx := context.Background()

	_, err := manager.RunVerify(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestRunVerifyNonexistentPath(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Bad Path Test",
		BackupID:   "backup-002",
		BackupPath: "/nonexistent/path",
		VerifyType: VerifyIntegrity,
	}

	created, err := manager.CreateVerifyTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateVerifyTask failed: %v", err)
	}

	result, err := manager.RunVerify(ctx, created.ID)
	if err != nil {
		t.Fatalf("RunVerify should not return error, got: %v", err)
	}

	if result.Status != ResultFail {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	if result.ErrorMessage == "" {
		t.Error("expected error message to be set")
	}
}

func TestRunRestoreTest(t *testing.T) {
	// Create test backup directory
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	// Create test files
	for i := 0; i < 5; i++ {
		path := filepath.Join(backupDir, "file"+string(rune('0'+i))+".txt")
		os.WriteFile(path, []byte("test content"), 0644)
	}

	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Restore Test",
		BackupID:   "backup-001",
		BackupPath: backupDir,
		VerifyType: VerifyRestore,
	}

	created, err := manager.CreateVerifyTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateVerifyTask failed: %v", err)
	}

	// Run restore test
	result, err := manager.RunRestoreTest(ctx, created.ID)
	if err != nil {
		t.Fatalf("RunRestoreTest failed: %v", err)
	}

	if result.Status != RestoreStatusCompleted {
		t.Errorf("expected status completed, got %s", result.Status)
	}
	if result.RestoredFiles == 0 {
		t.Error("expected restored files > 0")
	}
	if result.VerifiedFiles == 0 {
		t.Error("expected verified files > 0")
	}
}

func TestRunRestoreTestNonexistentTask(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	ctx := context.Background()

	_, err := manager.RunRestoreTest(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestGetBackupHealth(t *testing.T) {
	// Create test backup directory
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)
	os.WriteFile(filepath.Join(backupDir, "test.txt"), []byte("test"), 0644)

	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Health Test",
		BackupID:   "backup-001",
		BackupPath: backupDir,
		VerifyType: VerifyIntegrity,
	}

	created, err := manager.CreateVerifyTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateVerifyTask failed: %v", err)
	}

	// Run verification first
	_, err = manager.RunVerify(ctx, created.ID)
	if err != nil {
		t.Fatalf("RunVerify failed: %v", err)
	}

	// Get health
	health, err := manager.GetBackupHealth(ctx, "backup-001")
	if err != nil {
		t.Fatalf("GetBackupHealth failed: %v", err)
	}

	if health.BackupID != "backup-001" {
		t.Errorf("expected backup ID backup-001, got %s", health.BackupID)
	}
	if health.IntegrityScore < 0 || health.IntegrityScore > 100 {
		t.Errorf("integrity score should be 0-100, got %f", health.IntegrityScore)
	}
}

func TestGetBackupHealthNotVerified(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Health Test",
		BackupID:   "backup-001",
		BackupPath: backupDir,
		VerifyType: VerifyIntegrity,
	}

	_, err := manager.CreateVerifyTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateVerifyTask failed: %v", err)
	}

	// Get health without running verification
	health, err := manager.GetBackupHealth(ctx, "backup-001")
	if err != nil {
		t.Fatalf("GetBackupHealth failed: %v", err)
	}

	if health.VerifyStatus != "not_verified" {
		t.Errorf("expected verify status not_verified, got %s", health.VerifyStatus)
	}
	if health.RiskLevel != RiskHigh {
		t.Errorf("expected risk level high, got %s", health.RiskLevel)
	}
}

func TestGetBackupHealthNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	ctx := context.Background()

	_, err := manager.GetBackupHealth(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent backup")
	}
}

func TestGenerateReport(t *testing.T) {
	// Create test backup directories
	tmpDir := t.TempDir()
	backupDir1 := filepath.Join(tmpDir, "backup1")
	backupDir2 := filepath.Join(tmpDir, "backup2")
	os.MkdirAll(backupDir1, 0755)
	os.MkdirAll(backupDir2, 0755)
	os.WriteFile(filepath.Join(backupDir1, "test.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(backupDir2, "test.txt"), []byte("test"), 0644)

	manager := NewManager(tmpDir)
	ctx := context.Background()

	// Create tasks
	task1 := VerifyTask{
		Name:       "Task 1",
		BackupID:   "backup-001",
		BackupPath: backupDir1,
		VerifyType: VerifyIntegrity,
	}
	task2 := VerifyTask{
		Name:       "Task 2",
		BackupID:   "backup-002",
		BackupPath: backupDir2,
		VerifyType: VerifyIntegrity,
	}

	created1, _ := manager.CreateVerifyTask(ctx, task1)
	created2, _ := manager.CreateVerifyTask(ctx, task2)

	// Run verifications
	manager.RunVerify(ctx, created1.ID)
	manager.RunVerify(ctx, created2.ID)

	// Generate report
	report, err := manager.GenerateReport(ctx)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if report.TotalBackups != 2 {
		t.Errorf("expected 2 total backups, got %d", report.TotalBackups)
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected generated at to be set")
	}
	if len(report.Backups) != 2 {
		t.Errorf("expected 2 backups in report, got %d", len(report.Backups))
	}
}

func TestScheduleVerify(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Scheduled Task",
		BackupID:   "backup-001",
		BackupPath: backupDir,
		VerifyType: VerifyIntegrity,
	}

	created, err := manager.CreateVerifyTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateVerifyTask failed: %v", err)
	}

	// Schedule verification
	err = manager.ScheduleVerify(ctx, created.ID, "0 2 * * *")
	if err != nil {
		t.Fatalf("ScheduleVerify failed: %v", err)
	}

	// Verify task was updated
	updated, _ := manager.GetTask(ctx, created.ID)
	if updated.Schedule != "0 2 * * *" {
		t.Errorf("expected schedule '0 2 * * *', got %s", updated.Schedule)
	}
	if updated.NextRun == nil {
		t.Error("expected next run to be set")
	}
}

func TestGetVerifyHistory(t *testing.T) {
	// Create test backup directory
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)
	os.WriteFile(filepath.Join(backupDir, "test.txt"), []byte("test"), 0644)

	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "History Test",
		BackupID:   "backup-001",
		BackupPath: backupDir,
		VerifyType: VerifyIntegrity,
	}

	created, _ := manager.CreateVerifyTask(ctx, task)

	// Run verification multiple times
	manager.RunVerify(ctx, created.ID)
	manager.RunVerify(ctx, created.ID)

	// Get history
	history := manager.GetVerifyHistory(ctx, created.ID)
	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}
}

func TestGetVerifyHistoryEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	ctx := context.Background()

	history := manager.GetVerifyHistory(ctx, "nonexistent")
	if len(history) != 0 {
		t.Errorf("expected 0 history entries, got %d", len(history))
	}
}

func TestAutoRepair(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)
	os.WriteFile(filepath.Join(backupDir, "test.txt"), []byte("test"), 0644)

	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Repair Test",
		BackupID:   "backup-001",
		BackupPath: backupDir,
		VerifyType: VerifyIntegrity,
	}

	created, _ := manager.CreateVerifyTask(ctx, task)
	manager.RunVerify(ctx, created.ID)

	// Run auto repair
	repaired, err := manager.AutoRepair(ctx, "backup-001")
	if err != nil {
		t.Fatalf("AutoRepair failed: %v", err)
	}

	// Should be 0 since files are not corrupted
	if repaired != 0 {
		t.Errorf("expected 0 repaired files, got %d", repaired)
	}
}

func TestAutoRepairNoResults(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Repair Test",
		BackupID:   "backup-001",
		BackupPath: backupDir,
		VerifyType: VerifyIntegrity,
	}

	_, _ = manager.CreateVerifyTask(ctx, task)

	_, err := manager.AutoRepair(ctx, "backup-001")
	if err == nil {
		t.Error("expected error when no results exist")
	}
}

func TestAutoRepairNoTask(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	ctx := context.Background()

	_, err := manager.AutoRepair(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent backup")
	}
}

func TestGetRecommendations(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	manager := NewManager(tmpDir)
	ctx := context.Background()

	task := VerifyTask{
		Name:       "Recommend Test",
		BackupID:   "backup-001",
		BackupPath: backupDir,
		VerifyType: VerifyIntegrity,
	}

	_, _ = manager.CreateVerifyTask(ctx, task)

	recommendations := manager.GetRecommendations(ctx, "backup-001")
	if len(recommendations) == 0 {
		t.Error("expected recommendations for unverified backup")
	}
}

func TestBackupExists(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Test existing directory
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)
	if !manager.BackupExists(backupDir) {
		t.Error("expected backup to exist")
	}

	// Test nonexistent directory
	if manager.BackupExists("/nonexistent/path") {
		t.Error("expected backup to not exist")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		result := FormatSize(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatSize(%d) = %s, expected %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Millisecond, "0.5s"},
		{90 * time.Second, "1.5m"},
		{3600 * time.Second, "1.0h"},
	}

	for _, tt := range tests {
		result := FormatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("FormatDuration(%v) = %s, expected %s", tt.duration, result, tt.expected)
		}
	}
}
