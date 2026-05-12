// Package speedtest 测试
package speedtest

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("manager should not be nil")
	}

	servers := m.ListServers()
	if len(servers) == 0 {
		t.Fatal("should have default servers")
	}

	// 验证默认服务器数量
	if len(servers) != 5 {
		t.Errorf("expected 5 default servers, got %d", len(servers))
	}
}

func TestGetBestServer(t *testing.T) {
	m := NewManager()

	best, err := m.GetBestServer()
	if err != nil {
		t.Fatalf("get best server failed: %v", err)
	}
	if best == nil {
		t.Fatal("best server should not be nil")
	}

	// 应该是距离最近的服务器
	servers := m.ListServers()
	for _, s := range servers {
		if s.Distance < best.Distance {
			t.Errorf("found server %s with shorter distance %f than best %s with %f",
				s.Name, s.Distance, best.Name, best.Distance)
		}
	}
}

func TestRunTest(t *testing.T) {
	m := NewManager()

	result, err := m.RunTest("")
	if err != nil {
		t.Fatalf("run test failed: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.ID == "" {
		t.Error("result should have an ID")
	}
	if result.DownloadSpeed <= 0 {
		t.Error("download speed should be positive")
	}
	if result.UploadSpeed <= 0 {
		t.Error("upload speed should be positive")
	}
	if result.Latency <= 0 {
		t.Error("latency should be positive")
	}
	if result.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestRunDownloadTest(t *testing.T) {
	m := NewManager()

	result, err := m.RunDownloadTest("")
	if err != nil {
		t.Fatalf("run download test failed: %v", err)
	}
	if result.DownloadSpeed <= 0 {
		t.Error("download speed should be positive")
	}
	if result.UploadSpeed != 0 {
		t.Error("upload speed should be 0 for download-only test")
	}
}

func TestRunUploadTest(t *testing.T) {
	m := NewManager()

	result, err := m.RunUploadTest("")
	if err != nil {
		t.Fatalf("run upload test failed: %v", err)
	}
	if result.UploadSpeed <= 0 {
		t.Error("upload speed should be positive")
	}
	if result.DownloadSpeed != 0 {
		t.Error("download speed should be 0 for upload-only test")
	}
}

func TestRunLatencyTest(t *testing.T) {
	m := NewManager()

	result, err := m.RunLatencyTest("")
	if err != nil {
		t.Fatalf("run latency test failed: %v", err)
	}
	if result.Latency <= 0 {
		t.Error("latency should be positive")
	}
	if result.Jitter < 0 {
		t.Error("jitter should be non-negative")
	}
	if result.PacketLoss < 0 {
		t.Error("packet loss should be non-negative")
	}
}

func TestRunTestWithSpecificServer(t *testing.T) {
	m := NewManager()
	servers := m.ListServers()

	result, err := m.RunTest(servers[0].ID)
	if err != nil {
		t.Fatalf("run test with specific server failed: %v", err)
	}
	if result.ServerName != servers[0].Name {
		t.Errorf("expected server name %s, got %s", servers[0].Name, result.ServerName)
	}
}

func TestRunTestWithInvalidServer(t *testing.T) {
	m := NewManager()

	_, err := m.RunTest("nonexistent-server-id")
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestAddServer(t *testing.T) {
	m := NewManager()
	initialCount := len(m.ListServers())

	server := m.AddServer(AddServerRequest{
		Name:     "自定义服务器",
		URL:      "https://custom.example.com",
		Location: "深圳",
		Distance: 500,
	})
	if server == nil {
		t.Fatal("server should not be nil")
	}
	if server.ID == "" {
		t.Error("server should have an ID")
	}
	if server.Name != "自定义服务器" {
		t.Errorf("expected name 自定义服务器, got %s", server.Name)
	}

	servers := m.ListServers()
	if len(servers) != initialCount+1 {
		t.Errorf("expected %d servers, got %d", initialCount+1, len(servers))
	}
}

func TestRemoveServer(t *testing.T) {
	m := NewManager()

	server := m.AddServer(AddServerRequest{
		Name: "to remove",
		URL:  "https://remove.example.com",
	})

	err := m.RemoveServer(server.ID)
	if err != nil {
		t.Fatalf("remove server failed: %v", err)
	}

	// 确认已删除
	for _, s := range m.ListServers() {
		if s.ID == server.ID {
			t.Error("server should have been removed")
		}
	}
}

func TestRemoveServerNotFound(t *testing.T) {
	m := NewManager()

	err := m.RemoveServer("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestGetHistory(t *testing.T) {
	m := NewManager()

	// 运行多次测试
	for i := 0; i < 5; i++ {
		m.RunTest("")
	}

	history := m.GetHistory(3)
	if len(history) != 3 {
		t.Errorf("expected 3 history items, got %d", len(history))
	}

	// 获取全部
	all := m.GetHistory(0)
	if len(all) != 5 {
		t.Errorf("expected 5 history items, got %d", len(all))
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()

	// 运行测试
	for i := 0; i < 3; i++ {
		m.RunTest("")
	}

	stats := m.GetStats()
	if stats == nil {
		t.Fatal("stats should not be nil")
	}
	if stats.TestCount != 3 {
		t.Errorf("expected test count 3, got %d", stats.TestCount)
	}
	if stats.AvgDownload <= 0 {
		t.Error("avg download should be positive")
	}
	if stats.AvgUpload <= 0 {
		t.Error("avg upload should be positive")
	}
	if stats.AvgLatency <= 0 {
		t.Error("avg latency should be positive")
	}
	if stats.LastTestTime.IsZero() {
		t.Error("last test time should be set")
	}
}

func TestGetStatsEmpty(t *testing.T) {
	m := NewManager()

	stats := m.GetStats()
	if stats.TestCount != 0 {
		t.Errorf("expected test count 0, got %d", stats.TestCount)
	}
}

func TestClearHistory(t *testing.T) {
	m := NewManager()

	// 运行测试
	m.RunTest("")
	m.RunTest("")

	if len(m.GetHistory(0)) != 2 {
		t.Error("should have 2 results before clear")
	}

	m.ClearHistory()

	if len(m.GetHistory(0)) != 0 {
		t.Error("should have 0 results after clear")
	}

	stats := m.GetStats()
	if stats.TestCount != 0 {
		t.Error("test count should be 0 after clear")
	}
}

func TestServerSortedByDistance(t *testing.T) {
	m := NewManager()

	servers := m.ListServers()
	for i := 1; i < len(servers); i++ {
		if servers[i].Distance < servers[i-1].Distance {
			t.Errorf("servers not sorted by distance: %f < %f",
				servers[i].Distance, servers[i-1].Distance)
		}
	}
}
