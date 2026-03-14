package downloader

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()

	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}

	m.Close()
}

func TestNewManagerCreatesDirectory(t *testing.T) {
	tempDir := filepath.Join(t.TempDir(), "nonexistent")
	logger := zap.NewNop()

	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Error("NewManager() should create data directory")
	}
}

func TestCreateTask(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	req := CreateTaskRequest{
		URL:      "https://example.com/test.zip",
		Name:     "Test Download",
		Type:     TypeHTTP,
		DestPath: filepath.Join(tempDir, "downloads"),
	}

	task, err := m.CreateTask(req)
	if err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	if task == nil {
		t.Fatal("CreateTask() returned nil task")
	}

	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}

	if task.Name != "Test Download" {
		t.Errorf("Task Name = %s, expected 'Test Download'", task.Name)
	}

	if task.Status != StatusWaiting {
		t.Errorf("Task Status = %s, expected 'waiting'", task.Status)
	}
}

func TestCreateTaskWithAutoType(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	tests := []struct {
		url          string
		expectedType DownloadType
	}{
		{"https://example.com/file.zip", TypeHTTP},
		{"http://example.com/file.zip", TypeHTTP},
		{"magnet:?xt=urn:btih:test", TypeMagnet},
		{"https://example.com/file.torrent", TypeBT},
		{"ftp://example.com/file.zip", TypeFTP},
	}

	for _, tt := range tests {
		task, err := m.CreateTask(CreateTaskRequest{URL: tt.url})
		if err != nil {
			t.Fatalf("CreateTask() error for %s: %v", tt.url, err)
		}

		if task.Type != tt.expectedType {
			t.Errorf("URL %s: Type = %s, expected %s", tt.url, task.Type, tt.expectedType)
		}
	}
}

func TestGetTask(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{
		URL:  "https://example.com/test.zip",
		Name: "Test",
	})

	// 获取存在的任务
	retrieved, exists := m.GetTask(task.ID)
	if !exists {
		t.Error("GetTask() should find existing task")
	}

	if retrieved.ID != task.ID {
		t.Errorf("GetTask() ID = %s, expected %s", retrieved.ID, task.ID)
	}

	// 获取不存在的任务
	_, exists = m.GetTask("nonexistent")
	if exists {
		t.Error("GetTask() should not find nonexistent task")
	}
}

func TestListTasks(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	// 创建多个任务
	m.CreateTask(CreateTaskRequest{URL: "https://example.com/1.zip", Name: "Task 1"})
	m.CreateTask(CreateTaskRequest{URL: "https://example.com/2.zip", Name: "Task 2"})
	m.CreateTask(CreateTaskRequest{URL: "https://example.com/3.zip", Name: "Task 3"})

	// 列出所有任务
	tasks := m.ListTasks("")
	if len(tasks) != 3 {
		t.Errorf("ListTasks() returned %d tasks, expected 3", len(tasks))
	}

	// 按状态过滤
	m.UpdateTask(tasks[0].ID, UpdateTaskRequest{Status: StatusDownloading})
	downloading := m.ListTasks(StatusDownloading)
	if len(downloading) != 1 {
		t.Errorf("ListTasks(downloading) returned %d tasks, expected 1", len(downloading))
	}
}

func TestUpdateTask(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{
		URL:  "https://example.com/test.zip",
		Name: "Test",
	})

	updated, err := m.UpdateTask(task.ID, UpdateTaskRequest{Status: StatusPaused})
	if err != nil {
		t.Fatalf("UpdateTask() returned error: %v", err)
	}

	if updated.Status != StatusPaused {
		t.Errorf("UpdateTask() Status = %s, expected 'paused'", updated.Status)
	}

	// 更新不存在的任务
	_, err = m.UpdateTask("nonexistent", UpdateTaskRequest{Status: StatusPaused})
	if err == nil {
		t.Error("UpdateTask() should return error for nonexistent task")
	}
}

func TestDeleteTask(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{
		URL:  "https://example.com/test.zip",
		Name: "Test",
	})

	err = m.DeleteTask(task.ID, false)
	if err != nil {
		t.Fatalf("DeleteTask() returned error: %v", err)
	}

	// 确认任务已删除
	_, exists := m.GetTask(task.ID)
	if exists {
		t.Error("Task should be deleted")
	}

	// 删除不存在的任务
	err = m.DeleteTask("nonexistent", false)
	if err == nil {
		t.Error("DeleteTask() should return error for nonexistent task")
	}
}

func TestGetStats(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	// 创建多个任务
	m.CreateTask(CreateTaskRequest{URL: "https://example.com/1.zip"})
	m.CreateTask(CreateTaskRequest{URL: "https://example.com/2.zip"})
	m.CreateTask(CreateTaskRequest{URL: "https://example.com/3.zip"})

	stats := m.GetStats()

	if stats.TotalTasks != 3 {
		t.Errorf("Stats.TotalTasks = %d, expected 3", stats.TotalTasks)
	}

	if stats.Waiting != 3 {
		t.Errorf("Stats.Waiting = %d, expected 3", stats.Waiting)
	}
}

func TestPauseAndResumeTask(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{
		URL:  "https://example.com/test.zip",
		Name: "Test",
	})

	err = m.PauseTask(task.ID)
	if err != nil {
		t.Fatalf("PauseTask() returned error: %v", err)
	}

	retrieved, _ := m.GetTask(task.ID)
	if retrieved.Status != StatusPaused {
		t.Errorf("Task Status = %s, expected 'paused'", retrieved.Status)
	}

	err = m.ResumeTask(task.ID)
	if err != nil {
		t.Fatalf("ResumeTask() returned error: %v", err)
	}

	retrieved, _ = m.GetTask(task.ID)
	if retrieved.Status != StatusDownloading {
		t.Errorf("Task Status = %s, expected 'downloading'", retrieved.Status)
	}
}

func TestSetOnTaskUpdate(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	var called bool
	var mu sync.Mutex
	m.SetOnTaskUpdate(func(task *DownloadTask) {
		mu.Lock()
		called = true
		mu.Unlock()
	})

	m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip"})

	mu.Lock()
	wasCalled := called
	mu.Unlock()

	if !wasCalled {
		t.Error("OnTaskUpdate callback should be called on CreateTask")
	}
}

func TestTaskPersistence(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()

	// 创建第一个管理器
	m1, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}

	task, _ := m1.CreateTask(CreateTaskRequest{
		URL:  "https://example.com/test.zip",
		Name: "Persistent Task",
	})
	taskID := task.ID

	m1.Close()

	// 创建第二个管理器，应该加载已保存的任务
	m2, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m2.Close()

	retrieved, exists := m2.GetTask(taskID)
	if !exists {
		t.Fatal("Task should persist after manager restart")
	}

	if retrieved.Name != "Persistent Task" {
		t.Errorf("Task Name = %s, expected 'Persistent Task'", retrieved.Name)
	}
}

func TestDetectType(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		url      string
		expected DownloadType
	}{
		{"magnet:?xt=urn:btih:abc123", TypeMagnet},
		{"https://example.com/file.torrent", TypeBT},
		{"https://example.com/file.zip", TypeHTTP},
		{"http://example.com/file.zip", TypeHTTP},
		{"ftp://example.com/file.zip", TypeFTP},
		{"unknown://example.com/file", TypeHTTP}, // 默认
	}

	for _, tt := range tests {
		result := m.detectType(tt.url)
		if result != tt.expected {
			t.Errorf("detectType(%s) = %s, expected %s", tt.url, result, tt.expected)
		}
	}
}

func TestExtractNameFromURL(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		url      string
		expected string
	}{
		{"https://example.com/file.zip", "file.zip"},
		{"magnet:?xt=urn:btih:abc&dn=Test+File", "Test File"},
		{"magnet:?xt=urn:btih:abc", "Unknown Torrent"},
		{"https://example.com/path/to/file.mp4", "file.mp4"},
	}

	for _, tt := range tests {
		result := m.extractNameFromURL(tt.url)
		if result != tt.expected {
			t.Errorf("extractNameFromURL(%s) = %s, expected %s", tt.url, result, tt.expected)
		}
	}
}

func TestSetTransmissionConfig(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	m.SetTransmissionURL("http://localhost:9091")
	m.SetTransmissionAuth("user", "pass")

	// 配置应该被设置，没有 panic 即可
}

func TestSetQbittorrentConfig(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	m.SetQbittorrentURL("http://localhost:8080")
	m.SetQbittorrentAuth("user", "pass")

	// 配置应该被设置，没有 panic 即可
}

func TestTaskCreatedAt(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	before := time.Now()
	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip"})
	after := time.Now()

	if task.CreatedAt.Before(before) || task.CreatedAt.After(after) {
		t.Error("Task.CreatedAt should be set to current time")
	}

	if task.UpdatedAt.Before(before) || task.UpdatedAt.After(after) {
		t.Error("Task.UpdatedAt should be set to current time")
	}
}