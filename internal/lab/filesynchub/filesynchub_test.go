package filesynchub

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewFileSyncHub(t *testing.T) {
	fsh := NewFileSyncHub()
	if fsh == nil {
		t.Fatal("expected non-nil FileSyncHub")
	}
}

func TestAddTask(t *testing.T) {
	fsh := NewFileSyncHub()

	task := SyncTask{
		ID:          "test-task",
		Name:        "Test Sync",
		Source:      "/tmp/source",
		Destination: "/tmp/dest",
		Mode:        "mirror",
	}

	err := fsh.AddTask(task)
	if err != nil {
		t.Fatal(err)
	}

	tasks := fsh.ListTasks()
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

func TestAddTaskValidation(t *testing.T) {
	fsh := NewFileSyncHub()

	// Missing ID
	err := fsh.AddTask(SyncTask{Source: "/tmp/src", Destination: "/tmp/dst"})
	if err == nil {
		t.Error("expected error for missing ID")
	}

	// Missing source
	err = fsh.AddTask(SyncTask{ID: "test", Destination: "/tmp/dst"})
	if err == nil {
		t.Error("expected error for missing source")
	}
}

func TestRemoveTask(t *testing.T) {
	fsh := NewFileSyncHub()

	fsh.AddTask(SyncTask{
		ID:          "test-task",
		Source:      "/tmp/source",
		Destination: "/tmp/dest",
	})

	fsh.RemoveTask("test-task")
	tasks := fsh.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestRunSync(t *testing.T) {
	// Create test directories
	srcDir := filepath.Join(os.TempDir(), "synctest-src")
	dstDir := filepath.Join(os.TempDir(), "synctest-dst")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(dstDir, 0755)
	defer os.RemoveAll(srcDir)
	defer os.RemoveAll(dstDir)

	// Create test file
	testFile := filepath.Join(srcDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	fsh := NewFileSyncHub()
	fsh.AddTask(SyncTask{
		ID:          "test-sync",
		Source:      srcDir,
		Destination: dstDir,
	})

	result, err := fsh.RunSync(context.Background(), "test-sync")
	if err != nil {
		t.Fatal(err)
	}

	if result.FilesSynced != 1 {
		t.Errorf("expected 1 file synced, got %d", result.FilesSynced)
	}
}

func TestGetTask(t *testing.T) {
	fsh := NewFileSyncHub()

	fsh.AddTask(SyncTask{
		ID:          "test-task",
		Source:      "/tmp/src",
		Destination: "/tmp/dst",
	})

	task, err := fsh.GetTask("test-task")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "test-task" {
		t.Errorf("expected task ID 'test-task', got '%s'", task.ID)
	}
}

func TestStartStop(t *testing.T) {
	fsh := NewFileSyncHub()
	ctx := context.Background()

	err := fsh.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = fsh.Start(ctx)
	if err == nil {
		t.Error("expected error on double start")
	}

	fsh.Stop()
}
