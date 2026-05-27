package backupverify

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:            true,
		AutoVerifyEnabled:  true,
		VerifySchedule:     "0 2 * * *",
		RestoreTestEnabled: true,
		TestRestorePath:    "/tmp/restore-test",
		SpotCheckPercent:   10,
		MaxConcurrent:      3,
		RetentionDays:      90,
	}
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestAddJobAndVerify(t *testing.T) {
	config := &Config{Enabled: true, TestRestorePath: "/tmp/test"}
	manager := NewManager(config)

	job := &BackupJob{
		Name:        "Daily Backup",
		Source:      "/data",
		Destination: "/backup/daily",
		Schedule:    "0 0 * * *",
		Size:        1024 * 1024 * 100,
		FileCount:   1000,
		Enabled:     true,
	}
	if err := manager.AddJob(job); err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	verification, err := manager.RunVerification(job.ID, VerifyFull)
	if err != nil {
		t.Fatalf("RunVerification failed: %v", err)
	}

	if verification.Status != StatusPassed {
		t.Errorf("expected passed, got %s", verification.Status)
	}
	if verification.FilesChecked != 1000 {
		t.Errorf("expected 1000 files, got %d", verification.FilesChecked)
	}
}

func TestRestoreTest(t *testing.T) {
	config := &Config{Enabled: true, TestRestorePath: "/tmp/test"}
	manager := NewManager(config)

	job := &BackupJob{Name: "Test", Source: "/src", Destination: "/dst", Size: 100, FileCount: 10, Enabled: true}
	manager.AddJob(job)

	test, err := manager.RunRestoreTest(job.ID)
	if err != nil {
		t.Fatalf("RunRestoreTest failed: %v", err)
	}
	if !test.Success {
		t.Error("expected restore test to succeed")
	}
}

func TestGenerateReport(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	report := manager.GenerateReport("monthly")
	if report == nil {
		t.Fatal("GenerateReport returned nil")
	}
	if report.Period != "monthly" {
		t.Errorf("expected monthly, got %s", report.Period)
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	stats := manager.GetStats()
	if stats.TotalJobs != 0 {
		t.Errorf("expected 0 jobs, got %d", stats.TotalJobs)
	}
}
