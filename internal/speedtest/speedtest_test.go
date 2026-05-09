package speedtest

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.maxHistory != 50 {
		t.Errorf("maxHistory = %d, 期望 50", m.maxHistory)
	}
	if len(m.history) != 0 {
		t.Errorf("初始历史记录应为空，实际 %d", len(m.history))
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	cfg := &TestConfig{
		ServerURL:    "http://test.example.com",
		Duration:     10,
		Parallel:     4,
		TestFileSize: 100,
		TargetDisk:   "/mnt/data",
	}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.config.ServerURL != "http://test.example.com" {
		t.Errorf("ServerURL = %s, 期望 http://test.example.com", m.config.ServerURL)
	}
	if m.config.Duration != 10 {
		t.Errorf("Duration = %d, 期望 10", m.config.Duration)
	}
}

func TestRunNetworkTest(t *testing.T) {
	cfg := &TestConfig{Duration: 1}
	m := NewManager(cfg)

	result, err := m.RunNetworkTest()
	if err != nil {
		t.Fatalf("RunNetworkTest 失败: %v", err)
	}

	if result.TestType != "network" {
		t.Errorf("TestType = %s, 期望 network", result.TestType)
	}
	if result.DownloadSpeed <= 0 {
		t.Errorf("DownloadSpeed 应 > 0, 实际 %f", result.DownloadSpeed)
	}
	if result.UploadSpeed <= 0 {
		t.Errorf("UploadSpeed 应 > 0, 实际 %f", result.UploadSpeed)
	}
	if result.Latency <= 0 {
		t.Errorf("Latency 应 > 0, 实际 %f", result.Latency)
	}
	if result.Timestamp.IsZero() {
		t.Error("Timestamp 不应为零值")
	}
}

func TestRunDiskTest(t *testing.T) {
	cfg := &TestConfig{Duration: 1}
	m := NewManager(cfg)

	result, err := m.RunDiskTest("/tmp/testdisk")
	if err != nil {
		t.Fatalf("RunDiskTest 失败: %v", err)
	}

	if result.TestType != "disk" {
		t.Errorf("TestType = %s, 期望 disk", result.TestType)
	}
	if result.DiskReadSpeed <= 0 {
		t.Errorf("DiskReadSpeed 应 > 0, 实际 %f", result.DiskReadSpeed)
	}
	if result.DiskWriteSpeed <= 0 {
		t.Errorf("DiskWriteSpeed 应 > 0, 实际 %f", result.DiskWriteSpeed)
	}
	if result.Device != "/tmp/testdisk" {
		t.Errorf("Device = %s, 期望 /tmp/testdisk", result.Device)
	}
}

func TestRunFullTest(t *testing.T) {
	cfg := &TestConfig{Duration: 1}
	m := NewManager(cfg)

	result, err := m.RunFullTest("/mnt/data")
	if err != nil {
		t.Fatalf("RunFullTest 失败: %v", err)
	}

	if result.TestType != "full" {
		t.Errorf("TestType = %s, 期望 full", result.TestType)
	}
	if result.DownloadSpeed <= 0 {
		t.Errorf("DownloadSpeed 应 > 0")
	}
	if result.DiskReadSpeed <= 0 {
		t.Errorf("DiskReadSpeed 应 > 0")
	}
}

func TestGetHistory(t *testing.T) {
	cfg := &TestConfig{Duration: 1}
	m := NewManager(cfg)

	// 初始为空
	history := m.GetHistory(10)
	if len(history) != 0 {
		t.Errorf("初始历史应为空，实际 %d", len(history))
	}

	// 运行 3 次测试
	for i := 0; i < 3; i++ {
		m.RunNetworkTest()
	}

	history = m.GetHistory(10)
	if len(history) != 3 {
		t.Errorf("历史记录应有 3 条，实际 %d", len(history))
	}

	// 测试 limit
	history = m.GetHistory(2)
	if len(history) != 2 {
		t.Errorf("limit=2 应返回 2 条，实际 %d", len(history))
	}

	// 验证顺序（最新的在前）
	if history[0].Timestamp.Before(history[1].Timestamp) {
		t.Error("历史记录应按时间倒序排列")
	}
}

func TestClearHistory(t *testing.T) {
	cfg := &TestConfig{Duration: 1}
	m := NewManager(cfg)

	m.RunNetworkTest()
	m.RunDiskTest("/tmp")

	if len(m.GetHistory(10)) != 2 {
		t.Fatal("测试前应有 2 条记录")
	}

	m.ClearHistory()

	if len(m.GetHistory(10)) != 0 {
		t.Errorf("清空后历史应为空，实际 %d", len(m.GetHistory(10)))
	}
}

func TestGetLatestResult(t *testing.T) {
	cfg := &TestConfig{Duration: 1}
	m := NewManager(cfg)

	// 无记录时返回 nil
	if m.GetLatestResult() != nil {
		t.Error("无记录时应返回 nil")
	}

	m.RunNetworkTest()
	latest := m.GetLatestResult()
	if latest == nil {
		t.Fatal("运行测试后 GetLatestResult 不应返回 nil")
	}
	if latest.TestType != "network" {
		t.Errorf("最新结果 TestType = %s, 期望 network", latest.TestType)
	}

	m.RunDiskTest("/tmp")
	latest = m.GetLatestResult()
	if latest.TestType != "disk" {
		t.Errorf("最新结果 TestType = %s, 期望 disk", latest.TestType)
	}
}

func TestMaxHistoryLimit(t *testing.T) {
	cfg := &TestConfig{Duration: 1}
	m := NewManager(cfg)
	m.maxHistory = 5 // 缩小限制便于测试

	for i := 0; i < 10; i++ {
		m.RunNetworkTest()
	}

	history := m.GetHistory(100)
	if len(history) > 5 {
		t.Errorf("历史记录不应超过 maxHistory=5，实际 %d", len(history))
	}
}

func TestResultTimestamp(t *testing.T) {
	cfg := &TestConfig{Duration: 1}
	m := NewManager(cfg)

	before := time.Now()
	m.RunNetworkTest()
	after := time.Now()

	latest := m.GetLatestResult()
	if latest.Timestamp.Before(before) || latest.Timestamp.After(after) {
		t.Errorf("Timestamp 应在测试期间，实际 %v", latest.Timestamp)
	}
}
