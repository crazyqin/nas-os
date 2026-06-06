package smartdownloader

import (
	"testing"
	"time"
)

func TestNewDownloadManager(t *testing.T) {
	scheduleConfig := ScheduleConfig{
		MaxConcurrent: 5,
		QueueSize:     100,
	}

	speedLimit := SpeedLimitConfig{
		GlobalDownload: 1024 * 1024, // 1MB/s
	}

	notifyConfig := NotifyConfig{
		Enabled: false,
	}

	dm := NewDownloadManager(scheduleConfig, speedLimit, notifyConfig)
	if dm == nil {
		t.Fatal("NewDownloadManager returned nil")
	}

	if dm.scheduleConfig.MaxConcurrent != 5 {
		t.Errorf("Expected MaxConcurrent 5, got %d", dm.scheduleConfig.MaxConcurrent)
	}
}

func TestDetectProtocol(t *testing.T) {
	scheduleConfig := ScheduleConfig{}
	speedLimit := SpeedLimitConfig{}
	notifyConfig := NotifyConfig{}

	dm := NewDownloadManager(scheduleConfig, speedLimit, notifyConfig)

	tests := []struct {
		url      string
		expected DownloadProtocol
	}{
		{"http://example.com/file.zip", ProtocolHTTP},
		{"https://example.com/file.zip", ProtocolHTTPS},
		{"ftp://example.com/file.zip", ProtocolFTP},
		{"magnet:?xt=urn:btih:abc123", ProtocolMagnet},
	}

	for _, test := range tests {
		result := dm.detectProtocol(test.url)
		if result != test.expected {
			t.Errorf("detectProtocol(%s) = %s, expected %s", test.url, result, test.expected)
		}
	}
}

func TestExtractFileName(t *testing.T) {
	scheduleConfig := ScheduleConfig{}
	speedLimit := SpeedLimitConfig{}
	notifyConfig := NotifyConfig{}

	dm := NewDownloadManager(scheduleConfig, speedLimit, notifyConfig)

	tests := []struct {
		url      string
		expected string
	}{
		{"http://example.com/file.zip", "file.zip"},
		{"http://example.com/path/to/doc.pdf", "doc.pdf"},
		{"http://example.com/", "index.html"},
	}

	for _, test := range tests {
		result := dm.extractFileName(test.url)
		if result != test.expected {
			t.Errorf("extractFileName(%s) = %s, expected %s", test.url, result, test.expected)
		}
	}
}

func TestValidateURL(t *testing.T) {
	scheduleConfig := ScheduleConfig{}
	speedLimit := SpeedLimitConfig{}
	notifyConfig := NotifyConfig{}

	dm := NewDownloadManager(scheduleConfig, speedLimit, notifyConfig)

	tests := []struct {
		url      string
		protocol DownloadProtocol
		valid    bool
	}{
		{"http://example.com/file.zip", ProtocolHTTP, true},
		{"https://example.com/file.zip", ProtocolHTTPS, true},
		{"magnet:?xt=urn:btih:abc123", ProtocolMagnet, true},
		{"invalid-url", ProtocolHTTP, false},
	}

	for _, test := range tests {
		err := dm.validateURL(test.url, test.protocol)
		if (err == nil) != test.valid {
			t.Errorf("validateURL(%s, %s) valid=%v, expected %v", test.url, test.protocol, err == nil, test.valid)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}

	for _, test := range tests {
		result := formatSize(test.bytes)
		if result != test.expected {
			t.Errorf("formatSize(%d) = %s, expected %s", test.bytes, result, test.expected)
		}
	}
}

func TestQueueOperations(t *testing.T) {
	scheduleConfig := ScheduleConfig{
		MaxConcurrent: 1,
		QueueSize:     10,
	}
	speedLimit := SpeedLimitConfig{}
	notifyConfig := NotifyConfig{}

	dm := NewDownloadManager(scheduleConfig, speedLimit, notifyConfig)

	// 测试队列操作
	item1 := &DownloadItem{
		ID:        "test1",
		Priority:  PriorityLow,
		CreatedAt: time.Now(),
	}

	item2 := &DownloadItem{
		ID:        "test2",
		Priority:  PriorityHigh,
		CreatedAt: time.Now(),
	}

	dm.enqueue(item1)
	dm.enqueue(item2)

	queue := dm.GetQueue()
	if len(queue) != 2 {
		t.Fatalf("Expected queue length 2, got %d", len(queue))
	}

	// 高优先级应该在前面
	if queue[0].ID != "test2" {
		t.Errorf("Expected first item to be test2, got %s", queue[0].ID)
	}
}

func TestGetStats(t *testing.T) {
	scheduleConfig := ScheduleConfig{}
	speedLimit := SpeedLimitConfig{}
	notifyConfig := NotifyConfig{}

	dm := NewDownloadManager(scheduleConfig, speedLimit, notifyConfig)

	stats := dm.GetStats()
	if stats.TotalDownloads != 0 {
		t.Errorf("Expected TotalDownloads 0, got %d", stats.TotalDownloads)
	}
}
