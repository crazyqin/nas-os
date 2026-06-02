package smartbackuporch

import (
	"fmt"
	"testing"
)

func TestNewOrchestrator(t *testing.T) {
	orch := NewOrchestrator(nil)
	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	if orch.config.MaxConcurrent != 3 {
		t.Errorf("expected max concurrent 3, got %d", orch.config.MaxConcurrent)
	}
}

func TestRegisterJob(t *testing.T) {
	orch := NewOrchestrator(nil)

	job := &BackupJob{
		ID:   "test-job-1",
		Name: "Test Backup Job",
		Type: BackupTypeFull,
		Source: &SourceConfig{
			Type: "file",
			Path: "/data",
		},
		Targets: []*TargetConfig{
			{
				Type:     TargetLocal,
				Name:     "local",
				Path:     "/backup",
				IsPrimary: true,
			},
		},
		Priority: PriorityNormal,
		Enabled:  true,
	}

	err := orch.RegisterJob(job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 测试重复注册
	err = orch.RegisterJob(job)
	if err == nil {
		t.Fatal("expected error for duplicate job")
	}
}

func TestGetJob(t *testing.T) {
	orch := NewOrchestrator(nil)

	job := &BackupJob{
		ID:   "test-job-2",
		Name: "Test Job 2",
		Type: BackupTypeIncr,
	}

	orch.RegisterJob(job)

	got, err := orch.GetJob("test-job-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Test Job 2" {
		t.Errorf("expected name 'Test Job 2', got '%s'", got.Name)
	}

	_, err = orch.GetJob("non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent job")
	}
}

func TestListJobs(t *testing.T) {
	orch := NewOrchestrator(nil)

	for i := 0; i < 3; i++ {
		orch.RegisterJob(&BackupJob{
			ID:   fmt.Sprintf("job-%d", i),
			Name: fmt.Sprintf("Job %d", i),
			Type: BackupTypeFull,
		})
	}

	jobs := orch.ListJobs()
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(jobs))
	}
}

func TestStartStop(t *testing.T) {
	orch := NewOrchestrator(nil)

	err := orch.Start()
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	err = orch.Start()
	if err == nil {
		t.Fatal("expected error for double start")
	}

	err = orch.Stop()
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	err = orch.Stop()
	if err == nil {
		t.Fatal("expected error for double stop")
	}
}

func TestValidateBackupChain(t *testing.T) {
	orch := NewOrchestrator(nil)

	// 不存在的链
	_, err := orch.ValidateBackupChain("non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent chain")
	}

	// 有效的备份链
	chain := &BackupChain{
		ID:    "chain-1",
		JobID: "job-1",
		FullBackup: &BackupRecord{
			ID:     "full-1",
			Status: StatusCompleted,
		},
		IncrBackups: []*BackupRecord{
			{ID: "incr-1", Status: StatusCompleted},
			{ID: "incr-2", Status: StatusCompleted},
		},
	}
	orch.chains["chain-1"] = chain

	valid, err := orch.ValidateBackupChain("chain-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Fatal("expected chain to be valid")
	}
}

func TestGetMetrics(t *testing.T) {
	orch := NewOrchestrator(nil)
	metrics := orch.GetMetrics()
	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
}
