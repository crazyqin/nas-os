package smartdownloadmanager

import (
	"fmt"
	"testing"
	"time"
)

func TestNewDownloadManager(t *testing.T) {
	dm := NewDownloadManager("/downloads", 5)
	if dm == nil {
		t.Fatal("NewDownloadManager returned nil")
	}
	if dm.maxConcurrent != 5 {
		t.Errorf("expected maxConcurrent 5, got %d", dm.maxConcurrent)
	}
}

func TestNewDownloadManagerDefaultConcurrent(t *testing.T) {
	dm := NewDownloadManager("/downloads", 0)
	if dm.maxConcurrent != 3 {
		t.Errorf("expected default maxConcurrent 3, got %d", dm.maxConcurrent)
	}
}

func TestAddTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	task := &DownloadTask{
		ID:       "task1",
		URL:      "https://example.com/file.zip",
		Filename: "file.zip",
	}

	err := dm.AddTask(task)
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	if task.Status != StatusQueued {
		t.Errorf("expected queued status, got %v", task.Status)
	}

	if task.Protocol != ProtocolHTTPS {
		t.Errorf("expected https protocol, got %v", task.Protocol)
	}

	if task.Category != CategoryArchive {
		t.Errorf("expected archive category, got %v", task.Category)
	}

	// 测试重复添加
	err = dm.AddTask(task)
	if err == nil {
		t.Error("expected error for duplicate task")
	}
}

func TestAddTaskWithMagnet(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	task := &DownloadTask{
		ID:       "task1",
		URL:      "magnet:?xt=urn:btih:abc123&dn=test",
		Filename: "test.mkv",
	}

	err := dm.AddTask(task)
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	if task.Protocol != ProtocolMagnet {
		t.Errorf("expected magnet protocol, got %v", task.Protocol)
	}

	if task.Category != CategoryVideo {
		t.Errorf("expected video category, got %v", task.Category)
	}
}

func TestRemoveTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{
		ID:       "task1",
		URL:      "https://example.com/file.zip",
		Filename: "file.zip",
	})

	err := dm.RemoveTask("task1")
	if err != nil {
		t.Fatalf("RemoveTask failed: %v", err)
	}

	_, err = dm.GetTask("task1")
	if err == nil {
		t.Error("expected error for removed task")
	}
}

func TestRemoveTaskNotExist(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	err := dm.RemoveTask("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestGetTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{
		ID:       "task1",
		URL:      "https://example.com/file.zip",
		Filename: "file.zip",
	})

	task, err := dm.GetTask("task1")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if task.ID != "task1" {
		t.Errorf("expected task1, got %s", task.ID)
	}
}

func TestGetTaskNotExist(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	_, err := dm.GetTask("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestListTasks(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/a.zip", Filename: "a.zip", Priority: PriorityHigh})
	dm.AddTask(&DownloadTask{ID: "task2", URL: "https://example.com/b.mp4", Filename: "b.mp4", Priority: PriorityLow})
	dm.AddTask(&DownloadTask{ID: "task3", URL: "https://example.com/c.zip", Filename: "c.zip", Priority: PriorityHigh})

	// 按优先级筛选
	tasks := dm.ListTasks("", "", PriorityHigh)
	if len(tasks) != 2 {
		t.Errorf("expected 2 high priority tasks, got %d", len(tasks))
	}

	// 按分类筛选
	tasks = dm.ListTasks("", CategoryVideo, "")
	if len(tasks) != 1 {
		t.Errorf("expected 1 video task, got %d", len(tasks))
	}
}

func TestStartTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/file.zip", Filename: "file.zip"})

	err := dm.StartTask("task1")
	if err != nil {
		t.Fatalf("StartTask failed: %v", err)
	}

	task, _ := dm.GetTask("task1")
	if task.Status != StatusDownloading {
		t.Errorf("expected downloading status, got %v", task.Status)
	}

	if task.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}

	if dm.GetActiveCount() != 1 {
		t.Errorf("expected 1 active task, got %d", dm.GetActiveCount())
	}
}

func TestStartTaskMaxConcurrent(t *testing.T) {
	dm := NewDownloadManager("/downloads", 1)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/a.zip", Filename: "a.zip"})
	dm.AddTask(&DownloadTask{ID: "task2", URL: "https://example.com/b.zip", Filename: "b.zip"})

	dm.StartTask("task1")

	err := dm.StartTask("task2")
	if err == nil {
		t.Error("expected error when exceeding max concurrent")
	}
}

func TestPauseTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/file.zip", Filename: "file.zip"})
	dm.StartTask("task1")

	err := dm.PauseTask("task1")
	if err != nil {
		t.Fatalf("PauseTask failed: %v", err)
	}

	task, _ := dm.GetTask("task1")
	if task.Status != StatusPaused {
		t.Errorf("expected paused status, got %v", task.Status)
	}

	if dm.GetActiveCount() != 0 {
		t.Errorf("expected 0 active tasks, got %d", dm.GetActiveCount())
	}
}

func TestResumeTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	task := &DownloadTask{
		ID:            "task1",
		URL:           "https://example.com/file.zip",
		Filename:      "file.zip",
		ResumeSupport: true,
	}
	dm.AddTask(task)
	dm.StartTask("task1")
	dm.PauseTask("task1")

	err := dm.ResumeTask("task1")
	if err != nil {
		t.Fatalf("ResumeTask failed: %v", err)
	}

	task, _ = dm.GetTask("task1")
	if task.Status != StatusDownloading {
		t.Errorf("expected downloading status, got %v", task.Status)
	}
}

func TestResumeTaskNotSupported(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	task := &DownloadTask{
		ID:            "task1",
		URL:           "https://example.com/file.zip",
		Filename:      "file.zip",
		ResumeSupport: false,
	}
	dm.AddTask(task)
	dm.StartTask("task1")
	dm.PauseTask("task1")

	err := dm.ResumeTask("task1")
	if err == nil {
		t.Error("expected error for unsupported resume")
	}
}

func TestCancelTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/file.zip", Filename: "file.zip"})
	dm.StartTask("task1")

	err := dm.CancelTask("task1")
	if err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}

	task, _ := dm.GetTask("task1")
	if task.Status != StatusCancelled {
		t.Errorf("expected cancelled status, got %v", task.Status)
	}

	if dm.GetActiveCount() != 0 {
		t.Errorf("expected 0 active tasks, got %d", dm.GetActiveCount())
	}
}

func TestCancelCompletedTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/file.zip", Filename: "file.zip", TotalSize: 100})
	dm.StartTask("task1")
	dm.CompleteTask("task1")

	err := dm.CancelTask("task1")
	if err == nil {
		t.Error("expected error when cancelling completed task")
	}
}

func TestUpdateProgress(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/file.zip", Filename: "file.zip", TotalSize: 1000})

	err := dm.UpdateProgress("task1", 500, 100)
	if err != nil {
		t.Fatalf("UpdateProgress failed: %v", err)
	}

	task, _ := dm.GetTask("task1")
	if task.DownloadedSize != 500 {
		t.Errorf("expected 500 downloaded, got %d", task.DownloadedSize)
	}
	if task.Progress != 50.0 {
		t.Errorf("expected 50%% progress, got %.1f", task.Progress)
	}
	if task.Speed != 100 {
		t.Errorf("expected speed 100, got %d", task.Speed)
	}
}

func TestCompleteTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/file.zip", Filename: "file.zip", TotalSize: 1000})
	dm.StartTask("task1")

	err := dm.CompleteTask("task1")
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	task, _ := dm.GetTask("task1")
	if task.Status != StatusCompleted {
		t.Errorf("expected completed status, got %v", task.Status)
	}
	if task.Progress != 100 {
		t.Errorf("expected 100%% progress, got %.1f", task.Progress)
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestFailTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/file.zip", Filename: "file.zip", MaxRetries: 3})
	dm.StartTask("task1")

	// 第一次失败，应该重试
	err := dm.FailTask("task1", "连接超时")
	if err != nil {
		t.Fatalf("FailTask failed: %v", err)
	}

	task, _ := dm.GetTask("task1")
	if task.Status != StatusQueued {
		t.Errorf("expected queued status for retry, got %v", task.Status)
	}
	if task.RetryCount != 1 {
		t.Errorf("expected retry count 1, got %d", task.RetryCount)
	}
}

func TestFailTaskMaxRetries(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/file.zip", Filename: "file.zip", MaxRetries: 1})
	dm.StartTask("task1")

	// 超过最大重试次数
	dm.FailTask("task1", "连接超时")

	task, _ := dm.GetTask("task1")
	if task.Status != StatusFailed {
		t.Errorf("expected failed status, got %v", task.Status)
	}
}

func TestSpeedLimit(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	// 添加一个限速规则（假设当前时间在范围内）
	now := time.Now()
	startTime := fmt.Sprintf("%02d:%02d", now.Hour()-1, now.Minute())
	endTime := fmt.Sprintf("%02d:%02d", now.Hour()+1, now.Minute())

	dm.SetSpeedLimit(SpeedLimit{
		StartTime: startTime,
		EndTime:   endTime,
		MaxSpeed:  1024 * 1024, // 1MB/s
		Enabled:   true,
	})

	limit := dm.GetCurrentSpeedLimit()
	if limit != 1024*1024 {
		t.Errorf("expected speed limit 1MB/s, got %d", limit)
	}
}

func TestClearSpeedLimits(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.SetSpeedLimit(SpeedLimit{
		StartTime: "00:00",
		EndTime:   "23:59",
		MaxSpeed:  1024,
		Enabled:   true,
	})

	dm.ClearSpeedLimits()

	limit := dm.GetCurrentSpeedLimit()
	if limit != 0 {
		t.Errorf("expected no speed limit, got %d", limit)
	}
}

func TestGetStats(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/a.zip", Filename: "a.zip", TotalSize: 1000})
	dm.AddTask(&DownloadTask{ID: "task2", URL: "https://example.com/b.mp4", Filename: "b.mp4", TotalSize: 2000, MaxRetries: 1})
	dm.AddTask(&DownloadTask{ID: "task3", URL: "https://example.com/c.zip", Filename: "c.zip", TotalSize: 3000})

	dm.StartTask("task1")
	dm.CompleteTask("task1")
	dm.StartTask("task2")
	dm.FailTask("task2", "error")

	stats := dm.GetStats()
	if stats.TotalTasks != 3 {
		t.Errorf("expected 3 total tasks, got %d", stats.TotalTasks)
	}
	if stats.CompletedTasks != 1 {
		t.Errorf("expected 1 completed task, got %d", stats.CompletedTasks)
	}
	if stats.FailedTasks != 1 {
		t.Errorf("expected 1 failed task, got %d", stats.FailedTasks)
	}
}

func TestGetNextTask(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.AddTask(&DownloadTask{ID: "task1", URL: "https://example.com/a.zip", Filename: "a.zip", Priority: PriorityLow})
	dm.AddTask(&DownloadTask{ID: "task2", URL: "https://example.com/b.zip", Filename: "b.zip", Priority: PriorityHigh})
	dm.AddTask(&DownloadTask{ID: "task3", URL: "https://example.com/c.zip", Filename: "c.zip", Priority: PriorityMedium})

	next := dm.GetNextTask()
	if next == nil {
		t.Fatal("expected a task, got nil")
	}
	if next.Priority != PriorityHigh {
		t.Errorf("expected high priority task, got %v", next.Priority)
	}
}

func TestSetMaxConcurrent(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	dm.SetMaxConcurrent(10)
	if dm.GetMaxConcurrent() != 10 {
		t.Errorf("expected max concurrent 10, got %d", dm.GetMaxConcurrent())
	}

	// 测试无效值
	dm.SetMaxConcurrent(0)
	if dm.GetMaxConcurrent() != 10 {
		t.Errorf("expected max concurrent unchanged at 10, got %d", dm.GetMaxConcurrent())
	}
}

func TestParseMagnetLink(t *testing.T) {
	magnet := "magnet:?xt=urn:btih:abc123def456&dn=TestFile.mkv"

	infoHash, name, err := ParseMagnetLink(magnet)
	if err != nil {
		t.Fatalf("ParseMagnetLink failed: %v", err)
	}

	if infoHash != "abc123def456" {
		t.Errorf("expected abc123def456, got %s", infoHash)
	}
	if name != "TestFile.mkv" {
		t.Errorf("expected TestFile.mkv, got %s", name)
	}
}

func TestParseMagnetLinkInvalid(t *testing.T) {
	_, _, err := ParseMagnetLink("https://example.com")
	if err == nil {
		t.Error("expected error for invalid magnet link")
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{1099511627776, "1.00 TB"},
	}

	for _, tt := range tests {
		result := FormatFileSize(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatFileSize(%d) = %s, expected %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestFormatSpeed(t *testing.T) {
	result := FormatSpeed(1048576)
	if result != "1.00 MB/s" {
		t.Errorf("expected 1.00 MB/s, got %s", result)
	}
}

func TestDetectProtocol(t *testing.T) {
	tests := []struct {
		url      string
		expected DownloadProtocol
	}{
		{"https://example.com/file.zip", ProtocolHTTPS},
		{"http://example.com/file.zip", ProtocolHTTP},
		{"ftp://example.com/file.zip", ProtocolFTP},
		{"magnet:?xt=urn:btih:abc123", ProtocolMagnet},
		{"example.com/file.zip", ProtocolHTTP},
	}

	for _, tt := range tests {
		result := detectProtocol(tt.url)
		if result != tt.expected {
			t.Errorf("detectProtocol(%s) = %v, expected %v", tt.url, result, tt.expected)
		}
	}
}

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		filename string
		expected FileCategory
	}{
		{"movie.mp4", CategoryVideo},
		{"song.mp3", CategoryMusic},
		{"document.pdf", CategoryDocument},
		{"archive.zip", CategoryArchive},
		{"image.jpg", CategoryImage},
		{"unknown.xyz", CategoryOther},
		{"video.mkv", CategoryVideo},
		{"music.flac", CategoryMusic},
	}

	for _, tt := range tests {
		result := classifyFile(tt.filename)
		if result != tt.expected {
			t.Errorf("classifyFile(%s) = %v, expected %v", tt.filename, result, tt.expected)
		}
	}
}

func TestQueuePriority(t *testing.T) {
	dm := NewDownloadManager("/downloads", 10)

	dm.AddTask(&DownloadTask{ID: "low", URL: "https://example.com/a.zip", Filename: "a.zip", Priority: PriorityLow})
	dm.AddTask(&DownloadTask{ID: "urgent", URL: "https://example.com/b.zip", Filename: "b.zip", Priority: PriorityUrgent})
	dm.AddTask(&DownloadTask{ID: "high", URL: "https://example.com/c.zip", Filename: "c.zip", Priority: PriorityHigh})
	dm.AddTask(&DownloadTask{ID: "medium", URL: "https://example.com/d.zip", Filename: "d.zip", Priority: PriorityMedium})

	// 验证队列顺序
	task1 := dm.GetNextTask()
	if task1.ID != "urgent" {
		t.Errorf("expected urgent task first, got %s", task1.ID)
	}
	dm.StartTask(task1.ID)
	dm.CompleteTask(task1.ID)

	task2 := dm.GetNextTask()
	if task2.ID != "high" {
		t.Errorf("expected high task second, got %s", task2.ID)
	}
	dm.StartTask(task2.ID)
	dm.CompleteTask(task2.ID)

	task3 := dm.GetNextTask()
	if task3.ID != "medium" {
		t.Errorf("expected medium task third, got %s", task3.ID)
	}
	dm.StartTask(task3.ID)
	dm.CompleteTask(task3.ID)

	task4 := dm.GetNextTask()
	if task4.ID != "low" {
		t.Errorf("expected low task fourth, got %s", task4.ID)
	}
}

func TestAutoCategory(t *testing.T) {
	dm := NewDownloadManager("/downloads", 3)

	task := &DownloadTask{
		ID:       "task1",
		URL:      "https://example.com/movie.mp4",
		Filename: "movie.mp4",
	}

	dm.AddTask(task)

	if task.Category != CategoryVideo {
		t.Errorf("expected video category, got %v", task.Category)
	}

	expectedPath := "/downloads/videos"
	if task.SavePath != expectedPath {
		t.Errorf("expected save path %s, got %s", expectedPath, task.SavePath)
	}
}
