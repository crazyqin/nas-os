package downloadstation

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.downloadDir != config.DownloadDir {
		t.Errorf("downloadDir = %s, want %s", manager.downloadDir, config.DownloadDir)
	}
}

func TestCreateTask(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()
	config.AutoStart = false

	manager := NewManager(config)
	defer manager.Stop()

	tests := []struct {
		name    string
		req     CreateTaskRequest
		wantErr bool
	}{
		{
			name: "HTTP download",
			req: CreateTaskRequest{
				URL: "https://example.com/file.zip",
			},
			wantErr: false,
		},
		{
			name: "FTP download",
			req: CreateTaskRequest{
				URL: "ftp://ftp.example.com/file.zip",
			},
			wantErr: false,
		},
		{
			name: "Magnet link",
			req: CreateTaskRequest{
				URL: "magnet:?xt=urn:btih:1234567890abcdef",
			},
			wantErr: false,
		},
		{
			name: "Custom name and path",
			req: CreateTaskRequest{
				URL:      "https://example.com/file.zip",
				Name:     "custom-name.zip",
				FilePath: "/custom/path/file.zip",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := manager.CreateTask(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && task == nil {
				t.Error("CreateTask() returned nil task")
			}
		})
	}
}

func TestGetTask(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()
	config.AutoStart = false

	manager := NewManager(config)
	defer manager.Stop()

	// 创建任务
	req := CreateTaskRequest{
		URL: "https://example.com/file.zip",
	}
	task, err := manager.CreateTask(req)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	// 获取任务
	got, err := manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}

	if got.ID != task.ID {
		t.Errorf("GetTask() ID = %s, want %s", got.ID, task.ID)
	}
	if got.URL != task.URL {
		t.Errorf("GetTask() URL = %s, want %s", got.URL, task.URL)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()

	manager := NewManager(config)
	defer manager.Stop()

	_, err := manager.GetTask("non-existent-id")
	if err == nil {
		t.Error("GetTask() expected error for non-existent task")
	}
}

func TestListTasks(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()
	config.AutoStart = false

	manager := NewManager(config)
	defer manager.Stop()

	// 创建多个任务
	for i := 0; i < 5; i++ {
		req := CreateTaskRequest{
			URL: "https://example.com/file" + string(rune('0'+i)) + ".zip",
		}
		_, err := manager.CreateTask(req)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
	}

	tasks := manager.ListTasks()
	if len(tasks) != 5 {
		t.Errorf("ListTasks() returned %d tasks, want 5", len(tasks))
	}
}

func TestDeleteTask(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()
	config.AutoStart = false

	manager := NewManager(config)
	defer manager.Stop()

	// 创建任务
	req := CreateTaskRequest{
		URL: "https://example.com/file.zip",
	}
	task, err := manager.CreateTask(req)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	// 删除任务
	err = manager.DeleteTask(task.ID)
	if err != nil {
		t.Fatalf("DeleteTask() error = %v", err)
	}

	// 验证已删除
	_, err = manager.GetTask(task.ID)
	if err == nil {
		t.Error("GetTask() expected error after deletion")
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()

	manager := NewManager(config)
	defer manager.Stop()

	err := manager.DeleteTask("non-existent-id")
	if err == nil {
		t.Error("DeleteTask() expected error for non-existent task")
	}
}

func TestUpdateTask(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()
	config.AutoStart = false

	manager := NewManager(config)
	defer manager.Stop()

	// 创建任务
	req := CreateTaskRequest{
		URL: "https://example.com/file.zip",
	}
	task, err := manager.CreateTask(req)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	// 更新任务
	updateReq := UpdateTaskRequest{
		Priority: PriorityHigh,
		MaxSpeed: 1024 * 1024, // 1MB/s
	}
	updated, err := manager.UpdateTask(task.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}

	if updated.Priority != PriorityHigh {
		t.Errorf("UpdateTask() Priority = %d, want %d", updated.Priority, PriorityHigh)
	}
	if updated.MaxSpeed != 1024*1024 {
		t.Errorf("UpdateTask() MaxSpeed = %d, want %d", updated.MaxSpeed, 1024*1024)
	}
}

func TestGetStats(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()
	config.AutoStart = false

	manager := NewManager(config)
	defer manager.Stop()

	// 创建一些任务
	for i := 0; i < 3; i++ {
		req := CreateTaskRequest{
			URL: "https://example.com/file" + string(rune('0'+i)) + ".zip",
		}
		_, err := manager.CreateTask(req)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
	}

	stats := manager.GetStats()
	if stats.TotalTasks != 3 {
		t.Errorf("GetStats() TotalTasks = %d, want 3", stats.TotalTasks)
	}
}

func TestGetHistory(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()

	manager := NewManager(config)
	defer manager.Stop()

	history := manager.GetHistory()
	if history == nil {
		t.Error("GetHistory() returned nil")
	}
	if len(history) != 0 {
		t.Errorf("GetHistory() returned %d entries, want 0", len(history))
	}
}

func TestClearHistory(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()

	manager := NewManager(config)
	defer manager.Stop()

	manager.ClearHistory()
	history := manager.GetHistory()
	if len(history) != 0 {
		t.Errorf("GetHistory() returned %d entries after clear, want 0", len(history))
	}
}

func TestGetConfig(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()
	config.MaxConcurrent = 5

	manager := NewManager(config)
	defer manager.Stop()

	got := manager.GetConfig()
	if got.MaxConcurrent != 5 {
		t.Errorf("GetConfig() MaxConcurrent = %d, want 5", got.MaxConcurrent)
	}
	if got.DownloadDir != config.DownloadDir {
		t.Errorf("GetConfig() DownloadDir = %s, want %s", got.DownloadDir, config.DownloadDir)
	}
}

func TestUpdateConfig(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()
	config.MaxConcurrent = 3

	manager := NewManager(config)
	defer manager.Stop()

	newConfig := QueueConfig{
		MaxConcurrent: 10,
		MaxSpeedTotal: 1024 * 1024 * 10, // 10MB/s
		DownloadDir:   t.TempDir(),
	}

	manager.UpdateConfig(newConfig)
	got := manager.GetConfig()

	if got.MaxConcurrent != 10 {
		t.Errorf("GetConfig() MaxConcurrent = %d, want 10", got.MaxConcurrent)
	}
	if got.MaxSpeedTotal != 1024*1024*10 {
		t.Errorf("GetConfig() MaxSpeedTotal = %d, want %d", got.MaxSpeedTotal, 1024*1024*10)
	}
}

func TestDefaultQueueConfig(t *testing.T) {
	config := DefaultQueueConfig()

	if config.MaxConcurrent != 3 {
		t.Errorf("DefaultQueueConfig() MaxConcurrent = %d, want 3", config.MaxConcurrent)
	}
	if config.MaxRetries != 3 {
		t.Errorf("DefaultQueueConfig() MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.RetryDelay != 30 {
		t.Errorf("DefaultQueueConfig() RetryDelay = %d, want 30", config.RetryDelay)
	}
	if !config.AutoStart {
		t.Error("DefaultQueueConfig() AutoStart should be true")
	}
	if config.DownloadDir != "/downloads" {
		t.Errorf("DefaultQueueConfig() DownloadDir = %s, want /downloads", config.DownloadDir)
	}
}

func TestQueueBasicOperations(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()

	queue := NewDownloadQueue(config)

	// 创建任务
	task := &DownloadTask{
		ID:        "test-task-1",
		Name:      "test.zip",
		URL:       "https://example.com/test.zip",
		Status:    TaskStatusPending,
		Priority:  PriorityNormal,
		CreatedAt: time.Now(),
	}

	// Push
	queue.Push(task)
	if queue.Len() != 1 {
		t.Errorf("queue.Len() = %d, want 1", queue.Len())
	}

	// Pop
	popped := queue.Pop()
	if popped == nil {
		t.Fatal("queue.Pop() returned nil")
	}
	if popped.ID != task.ID {
		t.Errorf("queue.Pop() ID = %s, want %s", popped.ID, task.ID)
	}
	if queue.Len() != 0 {
		t.Errorf("queue.Len() = %d after pop, want 0", queue.Len())
	}
}

func TestQueuePriority(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()

	queue := NewDownloadQueue(config)

	// 创建不同优先级的任务
	lowTask := &DownloadTask{
		ID:        "low",
		Priority:  PriorityLow,
		CreatedAt: time.Now(),
	}
	highTask := &DownloadTask{
		ID:        "high",
		Priority:  PriorityHigh,
		CreatedAt: time.Now(),
	}
	normalTask := &DownloadTask{
		ID:        "normal",
		Priority:  PriorityNormal,
		CreatedAt: time.Now(),
	}

	// 按顺序 Push
	queue.Push(lowTask)
	queue.Push(normalTask)
	queue.Push(highTask)

	// 应该按优先级 Pop
	first := queue.Pop()
	if first.ID != "high" {
		t.Errorf("first pop = %s, want high", first.ID)
	}

	second := queue.Pop()
	if second.ID != "normal" {
		t.Errorf("second pop = %s, want normal", second.ID)
	}

	third := queue.Pop()
	if third.ID != "low" {
		t.Errorf("third pop = %s, want low", third.ID)
	}
}

func TestQueueActiveCount(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()
	config.MaxConcurrent = 2

	queue := NewDownloadQueue(config)

	// 检查初始状态
	if queue.ActiveCount() != 0 {
		t.Errorf("initial ActiveCount() = %d, want 0", queue.ActiveCount())
	}
	if !queue.CanStart() {
		t.Error("CanStart() should be true initially")
	}

	// 添加活跃任务
	task1 := &DownloadTask{ID: "task1"}
	task2 := &DownloadTask{ID: "task2"}
	queue.StartTask(task1)
	queue.StartTask(task2)

	if queue.ActiveCount() != 2 {
		t.Errorf("ActiveCount() = %d, want 2", queue.ActiveCount())
	}
	if queue.CanStart() {
		t.Error("CanStart() should be false when at max concurrent")
	}

	// 完成一个任务
	queue.CompleteTask("task1")
	if queue.ActiveCount() != 1 {
		t.Errorf("ActiveCount() after complete = %d, want 1", queue.ActiveCount())
	}
	if !queue.CanStart() {
		t.Error("CanStart() should be true after completing a task")
	}
}

func TestSpeedLimiter(t *testing.T) {
	// 测试无限制
	limiter := NewSpeedLimiter(0, 0)
	if !limiter.Allow(1024) {
		t.Error("Allow() should return true when no limit")
	}

	// 测试有限制
	limiter = NewSpeedLimiter(1024, 0) // 1KB/s
	if !limiter.Allow(512) {
		t.Error("Allow() should return true when under limit")
	}
	if !limiter.Allow(512) {
		t.Error("Allow() should return true when at limit")
	}
	// 等待一段时间让令牌桶补充
	time.Sleep(1100 * time.Millisecond)
	if !limiter.Allow(1024) {
		t.Error("Allow() should return true after waiting")
	}
}

func TestSpeedLimiterWait(t *testing.T) {
	limiter := NewSpeedLimiter(1024, 0) // 1KB/s

	// 消耗所有令牌
	limiter.Allow(1024)

	// 等待时间应该大于 0
	waitTime := limiter.Wait(512)
	if waitTime <= 0 {
		t.Errorf("Wait() = %v, want > 0", waitTime)
	}
}

func TestDetectDownloadType(t *testing.T) {
	tests := []struct {
		url  string
		want DownloadType
	}{
		{"https://example.com/file.zip", DownloadTypeHTTP},
		{"http://example.com/file.zip", DownloadTypeHTTP},
		{"ftp://ftp.example.com/file.zip", DownloadTypeFTP},
		{"magnet:?xt=urn:btih:1234567890abcdef", DownloadTypeMagnet},
		{"https://example.com/file.torrent", DownloadTypeBT},
		{"unknown://example.com/file", DownloadTypeHTTP},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := detectDownloadType(tt.url)
			if got != tt.want {
				t.Errorf("detectDownloadType(%s) = %s, want %s", tt.url, got, tt.want)
			}
		})
	}
}

func TestGenerateFileName(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		downloadType DownloadType
		wantEmpty    bool
	}{
		{
			name:         "HTTP URL with filename",
			url:          "https://example.com/path/to/file.zip",
			downloadType: DownloadTypeHTTP,
			wantEmpty:    false,
		},
		{
			name:         "Magnet with name",
			url:          "magnet:?xt=urn:btih:123&dn=Test+File",
			downloadType: DownloadTypeMagnet,
			wantEmpty:    false,
		},
		{
			name:         "Magnet without name",
			url:          "magnet:?xt=urn:btih:123",
			downloadType: DownloadTypeMagnet,
			wantEmpty:    false,
		},
		{
			name:         "BT torrent",
			url:          "https://example.com/file.torrent",
			downloadType: DownloadTypeBT,
			wantEmpty:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateFileName(tt.url, tt.downloadType)
			if tt.wantEmpty && got != "" {
				t.Errorf("generateFileName() = %s, want empty", got)
			}
			if !tt.wantEmpty && got == "" {
				t.Error("generateFileName() returned empty string")
			}
		})
	}
}

func TestExtractMagnetName(t *testing.T) {
	tests := []struct {
		magnet string
		want   string
	}{
		{"magnet:?xt=urn:btih:123&dn=Test+File", "Test File"},
		{"magnet:?xt=urn:btih:123&dn=Test%20File", "Test File"},
		{"magnet:?xt=urn:btih:123", ""},
		{"invalid", ""},
	}

	for _, tt := range tests {
		t.Run(tt.magnet, func(t *testing.T) {
			got := extractMagnetName(tt.magnet)
			if got != tt.want {
				t.Errorf("extractMagnetName(%s) = %s, want %s", tt.magnet, got, tt.want)
			}
		})
	}
}

func TestCalculateChecksum(t *testing.T) {
	// 创建临时文件
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World!")

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// 测试 MD5
	md5Hash, err := CalculateChecksum(filePath, ChecksumMD5)
	if err != nil {
		t.Fatalf("CalculateChecksum(MD5) error = %v", err)
	}
	if md5Hash == "" {
		t.Error("CalculateChecksum(MD5) returned empty hash")
	}

	// 测试 SHA256
	sha256Hash, err := CalculateChecksum(filePath, ChecksumSHA256)
	if err != nil {
		t.Fatalf("CalculateChecksum(SHA256) error = %v", err)
	}
	if sha256Hash == "" {
		t.Error("CalculateChecksum(SHA256) returned empty hash")
	}

	// 验证哈希值不同
	if md5Hash == sha256Hash {
		t.Error("MD5 and SHA256 hashes should be different")
	}

	// 测试不支持的类型
	_, err = CalculateChecksum(filePath, "unsupported")
	if err == nil {
		t.Error("CalculateChecksum(unsupported) expected error")
	}
}

func TestRSSManager(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()

	manager := NewManager(config)
	defer manager.Stop()

	rssManager := NewRSSManager(manager)
	defer rssManager.Stop()

	// 测试添加订阅
	req := AddRSSRequest{
		URL:      "https://example.com/feed.xml",
		Title:    "Test Feed",
		Interval: 30,
	}

	feed, err := rssManager.AddFeed(req)
	if err != nil {
		t.Fatalf("AddFeed() error = %v", err)
	}

	if feed.URL != req.URL {
		t.Errorf("AddFeed() URL = %s, want %s", feed.URL, req.URL)
	}
	if feed.Title != req.Title {
		t.Errorf("AddFeed() Title = %s, want %s", feed.Title, req.Title)
	}
	if feed.Interval != req.Interval {
		t.Errorf("AddFeed() Interval = %d, want %d", feed.Interval, req.Interval)
	}
	if !feed.Enabled {
		t.Error("AddFeed() should be enabled by default")
	}

	// 测试获取订阅
	got, err := rssManager.GetFeed(feed.ID)
	if err != nil {
		t.Fatalf("GetFeed() error = %v", err)
	}
	if got.ID != feed.ID {
		t.Errorf("GetFeed() ID = %s, want %s", got.ID, feed.ID)
	}

	// 测试列出订阅
	feeds := rssManager.ListFeeds()
	if len(feeds) != 1 {
		t.Errorf("ListFeeds() returned %d feeds, want 1", len(feeds))
	}

	// 测试更新订阅
	updateReq := AddRSSRequest{
		Title:    "Updated Title",
		Interval: 60,
	}
	updated, err := rssManager.UpdateFeed(feed.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateFeed() error = %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("UpdateFeed() Title = %s, want Updated Title", updated.Title)
	}
	if updated.Interval != 60 {
		t.Errorf("UpdateFeed() Interval = %d, want 60", updated.Interval)
	}

	// 测试启用/禁用
	err = rssManager.EnableFeed(feed.ID, false)
	if err != nil {
		t.Fatalf("EnableFeed() error = %v", err)
	}
	got, _ = rssManager.GetFeed(feed.ID)
	if got.Enabled {
		t.Error("EnableFeed() should be disabled")
	}

	// 测试删除订阅
	err = rssManager.DeleteFeed(feed.ID)
	if err != nil {
		t.Fatalf("DeleteFeed() error = %v", err)
	}
	feeds = rssManager.ListFeeds()
	if len(feeds) != 0 {
		t.Errorf("ListFeeds() returned %d feeds after deletion, want 0", len(feeds))
	}
}

func TestRSSManagerErrors(t *testing.T) {
	config := DefaultQueueConfig()
	config.DownloadDir = t.TempDir()

	manager := NewManager(config)
	defer manager.Stop()

	rssManager := NewRSSManager(manager)
	defer rssManager.Stop()

	// 测试获取不存在的订阅
	_, err := rssManager.GetFeed("non-existent")
	if err == nil {
		t.Error("GetFeed() expected error for non-existent feed")
	}

	// 测试删除不存在的订阅
	err = rssManager.DeleteFeed("non-existent")
	if err == nil {
		t.Error("DeleteFeed() expected error for non-existent feed")
	}

	// 测试更新不存在的订阅
	_, err = rssManager.UpdateFeed("non-existent", AddRSSRequest{})
	if err == nil {
		t.Error("UpdateFeed() expected error for non-existent feed")
	}

	// 测试启用不存在的订阅
	err = rssManager.EnableFeed("non-existent", true)
	if err == nil {
		t.Error("EnableFeed() expected error for non-existent feed")
	}

	// 测试获取不存在订阅的条目
	_, err = rssManager.GetFeedItems("non-existent")
	if err == nil {
		t.Error("GetFeedItems() expected error for non-existent feed")
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{".*\\.zip$", "file.zip", true},
		{".*\\.zip$", "file.txt", false},
		{"test", "this is a test", true},
		{"test", "no match", false},
		{"[0-9]+", "file123", true},
		{"[0-9]+", "file", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.text, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.text)
			if got != tt.want {
				t.Errorf("matchPattern(%s, %s) = %v, want %v", tt.pattern, tt.text, got, tt.want)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Mon, 02 Jan 2006 15:04:05 -0700", true},
		{"Mon, 02 Jan 2006 15:04:05 MST", true},
		{"2006-01-02T15:04:05Z", true},
		{"2006-01-02 15:04:05", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseTime(tt.input)
			if tt.want && got.IsZero() {
				t.Errorf("parseTime(%s) returned zero time, expected valid time", tt.input)
			}
			if !tt.want && !got.IsZero() {
				t.Errorf("parseTime(%s) returned non-zero time, expected zero time", tt.input)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "xyz", false},
		{"", "", true},
		{"a", "a", true},
		{"a", "b", false},
		{"abc", "abcd", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("contains(%s, %s) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}
