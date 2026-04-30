package smartmigrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSmartMigrateManager(t *testing.T) {
	mgr := NewSmartMigrateManager(nil)
	if mgr == nil {
		t.Fatal("NewSmartMigrateManager returned nil")
	}
	if mgr.config.MaxConcurrent != 3 {
		t.Errorf("expected max concurrent 3, got %d", mgr.config.MaxConcurrent)
	}
	if !mgr.config.VerifyChecksum {
		t.Error("expected verify checksum enabled")
	}
}

func TestCreateTaskInvalidSource(t *testing.T) {
	mgr := NewSmartMigrateManager(nil)
	_, err := mgr.CreateTask("test", "/nonexistent/path", "/tmp/dest", TypeCopy, nil)
	if err == nil {
		t.Error("expected error for invalid source path")
	}
}

func TestCreateTaskValidFile(t *testing.T) {
	mgr := NewSmartMigrateManager(nil)
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(srcFile, []byte("hello world"), 0644)

	task, err := mgr.CreateTask("test-copy", srcFile, filepath.Join(tmpDir, "dest.txt"), TypeCopy, nil)
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	if task.TotalBytes != 11 {
		t.Errorf("expected 11 bytes, got %d", task.TotalBytes)
	}
	if task.TotalFiles != 1 {
		t.Errorf("expected 1 file, got %d", task.TotalFiles)
	}
	if task.Status != MigrateStatusPending {
		t.Errorf("expected pending status, got %s", task.Status)
	}
}

func TestListTasksEmpty(t *testing.T) {
	mgr := NewSmartMigrateManager(nil)
	tasks := mgr.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestGetHistoryEmpty(t *testing.T) {
	mgr := NewSmartMigrateManager(nil)
	history := mgr.GetHistory()
	if len(history) != 0 {
		t.Errorf("expected 0 history, got %d", len(history))
	}
}

func TestMigrateTypes(t *testing.T) {
	types := []MigrateType{TypeCopy, TypeMove, TypeSync, TypeReplicate}
	for _, mt := range types {
		if mt == "" {
			t.Error("empty migrate type")
		}
	}
}
