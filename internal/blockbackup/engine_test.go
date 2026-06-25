package blockbackup

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNewBlockBackupEngine(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := BackupConfig{
		Compression:   "zstd",
		BlockSize:     4096,
		MaxBandwidth:  100,
		Parallel:      4,
		RetentionDays: 30,
	}

	engine := NewBlockBackupEngine(logger, config)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestBlockBackupEngine_ListJobs(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})

	jobs := engine.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestBlockBackupEngine_GetJob_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})

	job := engine.GetJob("nonexistent")
	if job != nil {
		t.Error("expected nil for nonexistent job")
	}
}

func TestBackupJob_Fields(t *testing.T) {
	job := &BackupJob{
		ID:     "test-123",
		Name:   "test backup",
		Source: "/data",
		Type:   "full",
		Status: "pending",
	}

	if job.ID != "test-123" {
		t.Errorf("expected test-123, got %s", job.ID)
	}
	if job.Type != "full" {
		t.Errorf("expected full, got %s", job.Type)
	}
}

func TestDefaultBackupConfig(t *testing.T) {
	config := BackupConfig{}
	if config.Compression != "" {
		t.Errorf("expected empty compression, got %s", config.Compression)
	}
}

func TestBlockBackupEngine_VerifyBackup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping file test")
	}

	logger, _ := zap.NewDevelopment()
	engine := NewBlockBackupEngine(logger, BackupConfig{})

	ctx := context.Background()
	// 测试不存在的文件
	err := engine.VerifyBackup(ctx, "/nonexistent/file")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
