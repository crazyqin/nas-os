package unifiedbackup

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	manager := NewManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	if manager.config == nil {
		t.Fatal("Expected default config")
	}
}

func TestCreateTask(t *testing.T) {
	manager := NewManager(nil)

	req := &CreateTaskRequest{
		Name: "测试备份任务",
		Source: BackupSource{
			Type: SourceFileSystem,
			Name: "本地文件系统",
			Path: "/data",
		},
		Mode:     BackupModeFull,
		Enabled:  true,
		Retention: RetentionPolicy{
			Mode:     RetentionByCount,
			MaxCount: 10,
		},
	}

	task, err := manager.CreateTask(req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if task.Name != "测试备份任务" {
		t.Errorf("Expected name '测试备份任务', got '%s'", task.Name)
	}

	if task.Status != TaskStatusPending {
		t.Errorf("Expected status pending, got %s", task.Status)
	}
}

func TestCreateTaskInvalidSource(t *testing.T) {
	manager := NewManager(nil)

	req := &CreateTaskRequest{
		Name:   "测试",
		Source: BackupSource{},
	}

	_, err := manager.CreateTask(req)
	if err == nil {
		t.Error("Expected error for invalid source")
	}
}

func TestGetTask(t *testing.T) {
	manager := NewManager(nil)

	req := &CreateTaskRequest{
		Name: "测试任务",
		Source: BackupSource{
			Type: SourceFileSystem,
			Name: "本地",
			Path: "/data",
		},
		Mode: BackupModeFull,
	}

	task, _ := manager.CreateTask(req)

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
	manager := NewManager(nil)

	manager.CreateTask(&CreateTaskRequest{
		Name:   "任务1",
		Source: BackupSource{Type: SourceFileSystem, Name: "源1", Path: "/data1"},
		Mode:   BackupModeFull,
	})

	manager.CreateTask(&CreateTaskRequest{
		Name:   "任务2",
		Source: BackupSource{Type: SourceDatabase, Name: "源2", Path: "/data2"},
		Mode:   BackupModeIncremental,
	})

	tasks := manager.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks))
	}
}

func TestDeleteTask(t *testing.T) {
	manager := NewManager(nil)

	req := &CreateTaskRequest{
		Name:   "待删除任务",
		Source: BackupSource{Type: SourceFileSystem, Name: "源", Path: "/data"},
		Mode:   BackupModeFull,
	}

	task, _ := manager.CreateTask(req)

	err := manager.DeleteTask(task.ID)
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	_, err = manager.GetTask(task.ID)
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestRunTask(t *testing.T) {
	manager := NewManager(nil)

	req := &CreateTaskRequest{
		Name:   "运行任务",
		Source: BackupSource{Type: SourceFileSystem, Name: "源", Path: "/data"},
		Mode:   BackupModeFull,
	}

	task, _ := manager.CreateTask(req)

	err := manager.RunTask(task.ID)
	if err != nil {
		t.Fatalf("RunTask failed: %v", err)
	}

	// 等待任务完成
	time.Sleep(2 * time.Second)

	updated, _ := manager.GetTask(task.ID)
	if updated.Status != TaskStatusCompleted {
		t.Errorf("Expected status completed, got %s", updated.Status)
	}
}

func TestGetRestorePoints(t *testing.T) {
	manager := NewManager(nil)

	req := &CreateTaskRequest{
		Name:   "备份任务",
		Source: BackupSource{Type: SourceFileSystem, Name: "源", Path: "/data"},
		Mode:   BackupModeFull,
	}

	task, _ := manager.CreateTask(req)
	manager.RunTask(task.ID)

	// 等待任务完成
	time.Sleep(2 * time.Second)

	points, err := manager.GetRestorePoints(task.ID)
	if err != nil {
		t.Fatalf("GetRestorePoints failed: %v", err)
	}

	if len(points) == 0 {
		t.Error("Expected at least one restore point")
	}
}

func TestGetStorageStats(t *testing.T) {
	manager := NewManager(nil)

	stats := manager.GetStorageStats()
	if stats == nil {
		t.Fatal("Expected storage stats")
	}

	if stats.TotalCapacity == 0 {
		t.Error("Expected non-zero total capacity")
	}
}
