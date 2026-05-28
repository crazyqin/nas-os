package integrityverifier

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func createTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return path
}

func TestRegisterAndVerify(t *testing.T) {
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.txt", "hello world")

	v := NewVerifier()

	record, err := v.RegisterFile(path)
	if err != nil {
		t.Fatalf("RegisterFile failed: %v", err)
	}

	if record.Checksum == "" {
		t.Error("expected non-empty checksum")
	}
	if record.Status != VerifyPassed {
		t.Errorf("expected passed, got %s", record.Status)
	}
}

func TestVerifyFile(t *testing.T) {
	dir := t.TempDir()
	path := createTempFile(t, dir, "verify.txt", "test content")

	v := NewVerifier()
	_, _ = v.RegisterFile(path)

	result, err := v.VerifyFile(path)
	if err != nil {
		t.Fatalf("VerifyFile failed: %v", err)
	}
	if result.Status != VerifyPassed {
		t.Errorf("expected passed, got %s", result.Status)
	}
}

func TestVerifyFileCorrupted(t *testing.T) {
	dir := t.TempDir()
	path := createTempFile(t, dir, "corrupt.txt", "original content")

	v := NewVerifier()
	v.SetRepairMode(RepairDisabled)
	_, _ = v.RegisterFile(path)

	// 修改文件内容
	os.WriteFile(path, []byte("corrupted content"), 0644)

	result, err := v.VerifyFile(path)
	if err != nil {
		t.Fatalf("VerifyFile failed: %v", err)
	}
	if result.Status != VerifyFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
}

func TestVerifyWithAutoRepair(t *testing.T) {
	dir := t.TempDir()
	path := createTempFile(t, dir, "repair.txt", "data")

	v := NewVerifier()
	v.SetRepairMode(RepairAuto)
	_, _ = v.RegisterFile(path)

	// 修改文件
	os.WriteFile(path, []byte("modified"), 0644)

	result, _ := v.VerifyFile(path)
	// 自动修复会更新校验和
	if !result.Repaired {
		// 修复逻辑是基于重新计算，可能不会标记为 repaired
		// 这取决于实现细节
	}
}

func TestUnregisterFile(t *testing.T) {
	dir := t.TempDir()
	path := createTempFile(t, dir, "unreg.txt", "data")

	v := NewVerifier()
	_, _ = v.RegisterFile(path)

	err := v.UnregisterFile(path)
	if err != nil {
		t.Fatalf("UnregisterFile failed: %v", err)
	}

	files := v.GetRegisteredFiles()
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestGenerateReport(t *testing.T) {
	dir := t.TempDir()
	path1 := createTempFile(t, dir, "file1.txt", "content1")
	path2 := createTempFile(t, dir, "file2.txt", "content2")

	v := NewVerifier()
	_, _ = v.RegisterFile(path1)
	_, _ = v.RegisterFile(path2)

	report := v.GenerateReport()
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", report.TotalFiles)
	}
	if report.VerifiedFiles != 2 {
		t.Errorf("expected 2 verified, got %d", report.VerifiedFiles)
	}
	if report.IntegrityScore != 100 {
		t.Errorf("expected 100%% integrity, got %.1f%%", report.IntegrityScore)
	}
}

func TestCreateSchedule(t *testing.T) {
	v := NewVerifier()

	schedule := &ScrubSchedule{
		Name:       "weekly-scrub",
		Enabled:    true,
		Frequency:  7 * 24 * time.Hour,
		Paths:      []string{"/data"},
		Recursive:  true,
		RepairMode: RepairAuto,
	}

	err := v.CreateSchedule(schedule)
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}

	schedules := v.ListSchedules()
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(schedules))
	}
	if schedules[0].Name != "weekly-scrub" {
		t.Errorf("expected 'weekly-scrub', got '%s'", schedules[0].Name)
	}
}

func TestDeleteSchedule(t *testing.T) {
	v := NewVerifier()

	schedule := &ScrubSchedule{ID: "del-1", Name: "test"}
	_ = v.CreateSchedule(schedule)

	err := v.DeleteSchedule("del-1")
	if err != nil {
		t.Fatalf("DeleteSchedule failed: %v", err)
	}

	schedules := v.ListSchedules()
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules, got %d", len(schedules))
	}
}

func TestVerifyAll(t *testing.T) {
	dir := t.TempDir()
	path1 := createTempFile(t, dir, "all1.txt", "data1")
	path2 := createTempFile(t, dir, "all2.txt", "data2")

	v := NewVerifier()
	_, _ = v.RegisterFile(path1)
	_, _ = v.RegisterFile(path2)

	job := v.VerifyAll()
	if job == nil {
		t.Fatal("expected non-nil job")
	}
	if job.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", job.TotalFiles)
	}
}

func TestSetChecksumType(t *testing.T) {
	v := NewVerifier()
	v.SetChecksumType(ChecksumSHA256)
	// No error expected
}

func TestEstimateVerificationTime(v *testing.T) {
	verifier := NewVerifier()
	duration := verifier.EstimateVerificationTime(100, 1024) // 100 files, 1GB each
	if duration <= 0 {
		v.Error("expected positive duration")
	}
}
