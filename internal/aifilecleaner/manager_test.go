package aifilecleaner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	manager := NewManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	if manager.config == nil {
		t.Fatal("Expected default config")
	}
	if manager.config.LargeFileThresholdMB != 100 {
		t.Errorf("Expected 100MB threshold, got %d", manager.config.LargeFileThresholdMB)
	}
}

func TestScan(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "test1.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test2.txt"), []byte("world"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "test3.txt"), []byte("sub"), 0644)

	manager := NewManager(&ScanConfig{
		RootPath:             tmpDir,
		LargeFileThresholdMB: 1,
		StaleDays:            1,
		MaxDepth:             5,
	})

	result, err := manager.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if result.TotalFiles < 3 {
		t.Errorf("Expected at least 3 files, got %d", result.TotalFiles)
	}
}

func TestCreateCleanTask(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	manager := NewManager(nil)

	task, err := manager.CreateCleanTask([]string{testFile}, DeleteModeRecycle)
	if err != nil {
		t.Fatalf("CreateCleanTask failed: %v", err)
	}

	if task.Status != TaskStatusPending {
		t.Errorf("Expected status pending, got %s", task.Status)
	}

	if task.Mode != DeleteModeRecycle {
		t.Errorf("Expected mode recycle, got %s", task.Mode)
	}
}

func TestCreateCleanTaskEmptyFiles(t *testing.T) {
	manager := NewManager(nil)

	_, err := manager.CreateCleanTask([]string{}, DeleteModeRecycle)
	if err == nil {
		t.Error("Expected error for empty files")
	}
}

func TestGetTask(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	manager := NewManager(nil)
	task, _ := manager.CreateCleanTask([]string{testFile}, DeleteModePermanent)

	fetched, err := manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if fetched.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, fetched.ID)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	manager := NewManager(nil)

	_, err := manager.GetTask("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent task")
	}
}

func TestListTasks(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "test1.txt")
	file2 := filepath.Join(tmpDir, "test2.txt")
	os.WriteFile(file1, []byte("test1"), 0644)
	os.WriteFile(file2, []byte("test2"), 0644)

	manager := NewManager(nil)
	manager.CreateCleanTask([]string{file1}, DeleteModeRecycle)
	manager.CreateCleanTask([]string{file2}, DeleteModePermanent)

	tasks := manager.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks))
	}
}

func TestCancelTask(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	manager := NewManager(nil)
	task, _ := manager.CreateCleanTask([]string{testFile}, DeleteModeRecycle)

	// 任务未运行时取消应该失败
	err := manager.CancelTask(task.ID)
	if err == nil {
		t.Error("Expected error when cancelling non-running task")
	}
}

func TestGetScanResultNoData(t *testing.T) {
	manager := NewManager(nil)

	_, err := manager.GetScanResult()
	if err == nil {
		t.Error("Expected error when no scan data")
	}
}

func TestFindDuplicates(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建重复文件
	content := []byte("duplicate content")
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), content, 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), content, 0644)
	os.WriteFile(filepath.Join(tmpDir, "unique.txt"), []byte("unique"), 0644)

	manager := NewManager(nil)
	duplicates, err := manager.FindDuplicates([]string{tmpDir})
	if err != nil {
		t.Fatalf("FindDuplicates failed: %v", err)
	}

	if len(duplicates) == 0 {
		t.Error("Expected to find duplicates")
	}
}
