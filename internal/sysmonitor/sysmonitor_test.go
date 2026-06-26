package sysmonitor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if !cfg.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if cfg.Interval != 30 {
		t.Errorf("Expected Interval 30, got %d", cfg.Interval)
	}
	if cfg.CPUAlert != 90.0 {
		t.Errorf("Expected CPUAlert 90.0, got %f", cfg.CPUAlert)
	}
	if cfg.MemAlert != 90.0 {
		t.Errorf("Expected MemAlert 90.0, got %f", cfg.MemAlert)
	}
	if cfg.DiskAlert != 95.0 {
		t.Errorf("Expected DiskAlert 95.0, got %f", cfg.DiskAlert)
	}
	if cfg.HistoryMaxSize != 120 {
		t.Errorf("Expected HistoryMaxSize 120, got %d", cfg.HistoryMaxSize)
	}
	if cfg.TopProcessCount != 10 {
		t.Errorf("Expected TopProcessCount 10, got %d", cfg.TopProcessCount)
	}
}

func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("with config and logger", func(t *testing.T) {
		cfg := &Config{Enabled: true, Interval: 60}
		m := NewManager(cfg, logger)
		if m == nil {
			t.Fatal("NewManager returned nil")
		}
		if m.config != cfg {
			t.Error("Config not set correctly")
		}
		if m.logger != logger {
			t.Error("Logger not set correctly")
		}
	})

	t.Run("with nil config", func(t *testing.T) {
		m := NewManager(nil, logger)
		if m == nil {
			t.Fatal("NewManager returned nil")
		}
		if m.config == nil {
			t.Fatal("Config should be set to default")
		}
		if !m.config.Enabled {
			t.Error("Default config should be enabled")
		}
	})

	t.Run("with nil logger", func(t *testing.T) {
		cfg := &Config{Enabled: true, Interval: 60}
		m := NewManager(cfg, nil)
		if m == nil {
			t.Fatal("NewManager returned nil")
		}
		if m.logger == nil {
			t.Fatal("Logger should be set to nop")
		}
	})
}

func TestManagerStartStop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &Config{Enabled: true, Interval: 60}
	m := NewManager(cfg, logger)

	// 测试启动
	err := m.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("Manager should be running")
	}

	// 测试重复启动
	err = m.Start()
	if err != nil {
		t.Fatalf("Second Start failed: %v", err)
	}

	// 测试停止
	err = m.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if m.IsRunning() {
		t.Fatal("Manager should be stopped")
	}

	// 测试重复停止
	err = m.Stop()
	if err != nil {
		t.Fatalf("Second Stop failed: %v", err)
	}
}

func TestManagerCollect(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &Config{
		Enabled:         true,
		Interval:        1,
		CPUAlert:        90.0,
		MemAlert:        90.0,
		DiskAlert:       95.0,
		HistoryMaxSize:  10,
		TopProcessCount: 5,
	}
	m := NewManager(cfg, logger)

	// 启动并等待采集
	err := m.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 等待采集完成
	time.Sleep(2 * time.Second)

	// 测试系统概览
	t.Run("overview", func(t *testing.T) {
		overview := m.GetOverview()
		if overview == nil {
			t.Fatal("Overview should not be nil after collection")
		}
		if overview.Hostname == "" {
			t.Error("Hostname should not be empty")
		}
		if overview.CPUCores <= 0 {
			t.Error("CPUCores should be positive")
		}
		if overview.MemTotal == 0 {
			t.Error("MemTotal should not be 0")
		}
		if overview.Timestamp == 0 {
			t.Error("Timestamp should be set")
		}
		t.Logf("Hostname: %s, OS: %s, CPU Cores: %d", overview.Hostname, overview.OS, overview.CPUCores)
		t.Logf("Memory: %d/%d (%.1f%%)", overview.MemUsed, overview.MemTotal, overview.MemPercent)
	})

	// 测试进程列表
	t.Run("processes", func(t *testing.T) {
		processes := m.GetProcesses()
		if len(processes) == 0 {
			t.Error("Processes should not be empty")
		}
		// 检查是否按 CPU 排序
		for i := 1; i < len(processes); i++ {
			if processes[i-1].CPUPercent < processes[i].CPUPercent {
				t.Error("Processes should be sorted by CPU percent (descending)")
				break
			}
		}
		t.Logf("Top %d processes by CPU:", len(processes))
		for i, p := range processes {
			t.Logf("  %d. PID=%d Name=%s CPU=%.1f%% Mem=%.1f%%", i+1, p.PID, p.Name, p.CPUPercent, p.MemPercent)
		}
	})

	// 测试磁盘使用
	t.Run("disk usage", func(t *testing.T) {
		diskUsage := m.GetDiskUsage()
		if len(diskUsage) == 0 {
			t.Error("Disk usage should not be empty")
		}
		for _, du := range diskUsage {
			if du.Total == 0 {
				t.Errorf("Disk %s total should not be 0", du.MountPoint)
			}
			t.Logf("Disk %s: %d/%d (%.1f%%) [%s]", du.MountPoint, du.Used, du.Total, du.UsedPercent, du.FSType)
		}
	})

	// 测试网络信息
	t.Run("network", func(t *testing.T) {
		network := m.GetNetwork()
		if network.Timestamp == 0 {
			t.Error("Network timestamp should be set")
		}
		t.Logf("Network: TCP=%d UDP=%d Listen=%d Established=%d",
			network.TCPCount, network.UDPCount, network.ListenCount, network.EstablishedCount)
		t.Logf("Bytes Sent=%d Recv=%d", network.BytesSent, network.BytesRecv)
	})

	// 测试系统负载
	t.Run("load", func(t *testing.T) {
		loadInfo := m.GetLoad()
		if loadInfo.CPUCores <= 0 {
			t.Error("CPUCores should be positive")
		}
		if loadInfo.Timestamp == 0 {
			t.Error("Timestamp should be set")
		}
		t.Logf("Load: 1min=%.2f 5min=%.2f 15min=%.2f Cores=%d",
			loadInfo.Load1, loadInfo.Load5, loadInfo.Load15, loadInfo.CPUCores)
	})

	// 测试运行时间
	t.Run("uptime", func(t *testing.T) {
		uptime := m.GetUptime()
		if uptime.Uptime == 0 {
			t.Error("Uptime should not be 0")
		}
		if uptime.BootTime == 0 {
			t.Error("BootTime should not be 0")
		}
		if uptime.BootTimeStr == "" {
			t.Error("BootTimeStr should not be empty")
		}
		if uptime.UptimeStr == "" {
			t.Error("UptimeStr should not be empty")
		}
		t.Logf("Uptime: %s, Boot: %s, Users: %d",
			uptime.UptimeStr, uptime.BootTimeStr, uptime.NumUsers)
	})

	// 测试告警
	t.Run("alerts", func(t *testing.T) {
		alerts := m.GetAlerts()
		t.Logf("Alerts count: %d", len(alerts))
		for _, alert := range alerts {
			t.Logf("  [%s] %s: %s", alert.Level, alert.Type, alert.Message)
		}
	})

	// 测试历史记录
	t.Run("history", func(t *testing.T) {
		history := m.GetHistory()
		if len(history) == 0 {
			t.Error("History should have at least one point")
		}
		for _, point := range history {
			if point.Timestamp == 0 {
				t.Error("History point timestamp should be set")
			}
		}
		t.Logf("History points: %d", len(history))
		if len(history) > 0 {
			last := history[len(history)-1]
			t.Logf("  Last: CPU=%.1f%% Mem=%.1f%% Disk=%.1f%%",
				last.CPUPercent, last.MemPercent, last.DiskPercent)
		}
	})
}

func TestManagerHistoryLimit(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &Config{
		Enabled:         true,
		Interval:        1,
		HistoryMaxSize:  3,
		TopProcessCount: 5,
	}
	m := NewManager(cfg, logger)

	// 模拟添加历史记录
	m.mu.Lock()
	m.overview = &SystemOverview{
		CPUPercent: 50.0,
		MemPercent: 60.0,
	}
	for i := 0; i < 5; i++ {
		m.history = append(m.history, HistoryPoint{
			Timestamp:  int64(i),
			CPUPercent: float64(i * 10),
		})
	}
	m.mu.Unlock()

	// 手动触发 recordHistory 来测试限制
	m.recordHistory()

	history := m.GetHistory()
	if len(history) > cfg.HistoryMaxSize {
		t.Errorf("History length %d exceeds max size %d", len(history), cfg.HistoryMaxSize)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"1 minute", time.Minute, "1分钟"},
		{"30 minutes", 30 * time.Minute, "30分钟"},
		{"1 hour", time.Hour, "1小时0分钟"},
		{"2 hours 30 min", 2*time.Hour + 30*time.Minute, "2小时30分钟"},
		{"1 day", 24 * time.Hour, "1天0小时0分钟"},
		{"3 days 5 hours 15 min", 3*24*time.Hour + 5*time.Hour + 15*time.Minute, "3天5小时15分钟"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestHandlerNew(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultConfig()
	m := NewManager(cfg, logger)

	h := NewHandler(m)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
	if h.manager != m {
		t.Error("Handler manager not set correctly")
	}
}

func TestHandlerRegisterRoutes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultConfig()
	m := NewManager(cfg, logger)
	h := NewHandler(m)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 验证路由注册
	routes := []string{
		"/api/v1/sys/overview",
		"/api/v1/sys/processes",
		"/api/v1/sys/diskusage",
		"/api/v1/sys/network",
		"/api/v1/sys/load",
		"/api/v1/sys/uptime",
		"/api/v1/sys/alerts",
		"/api/v1/sys/history",
	}

	for _, route := range routes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		// 路由应该存在（不是404）
		if rr.Code == http.StatusNotFound {
			t.Errorf("Route %s not registered", route)
		}
	}
}

func TestHandlerEndpoints(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &Config{
		Enabled:         true,
		Interval:        1,
		CPUAlert:        90.0,
		MemAlert:        90.0,
		DiskAlert:       95.0,
		HistoryMaxSize:  10,
		TopProcessCount: 5,
	}
	m := NewManager(cfg, logger)

	// 启动并等待采集
	err := m.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 等待采集完成
	time.Sleep(2 * time.Second)

	h := NewHandler(m)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	t.Run("GET /api/v1/sys/overview", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sys/overview", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var overview SystemOverview
		if err := json.NewDecoder(rr.Body).Decode(&overview); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if overview.Hostname == "" {
			t.Error("Hostname should not be empty")
		}
		t.Logf("Overview: Hostname=%s OS=%s CPU=%.1f%%", overview.Hostname, overview.OS, overview.CPUPercent)
	})

	t.Run("GET /api/v1/sys/processes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sys/processes", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		processes, ok := response["processes"].([]interface{})
		if !ok {
			t.Fatal("Processes should be an array")
		}
		count, ok := response["count"].(float64)
		if !ok {
			t.Fatal("Count should be a number")
		}
		if int(count) != len(processes) {
			t.Errorf("Count %d doesn't match array length %d", int(count), len(processes))
		}
		t.Logf("Processes: %d", len(processes))
	})

	t.Run("GET /api/v1/sys/diskusage", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sys/diskusage", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		disks, ok := response["disks"].([]interface{})
		if !ok {
			t.Fatal("Disks should be an array")
		}
		t.Logf("Disks: %d", len(disks))
	})

	t.Run("GET /api/v1/sys/network", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sys/network", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var network NetworkInfo
		if err := json.NewDecoder(rr.Body).Decode(&network); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		t.Logf("Network: TCP=%d UDP=%d Listen=%d", network.TCPCount, network.UDPCount, network.ListenCount)
	})

	t.Run("GET /api/v1/sys/load", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sys/load", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var loadInfo LoadInfo
		if err := json.NewDecoder(rr.Body).Decode(&loadInfo); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		t.Logf("Load: 1min=%.2f 5min=%.2f 15min=%.2f", loadInfo.Load1, loadInfo.Load5, loadInfo.Load15)
	})

	t.Run("GET /api/v1/sys/uptime", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sys/uptime", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var uptime UptimeInfo
		if err := json.NewDecoder(rr.Body).Decode(&uptime); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		t.Logf("Uptime: %s, Boot: %s", uptime.UptimeStr, uptime.BootTimeStr)
	})

	t.Run("GET /api/v1/sys/alerts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sys/alerts", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		alerts, ok := response["alerts"].([]interface{})
		if !ok {
			t.Fatal("Alerts should be an array")
		}
		t.Logf("Alerts: %d", len(alerts))
	})

	t.Run("GET /api/v1/sys/history", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sys/history", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		history, ok := response["history"].([]interface{})
		if !ok {
			t.Fatal("History should be an array")
		}
		t.Logf("History points: %d", len(history))
	})
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultConfig()
	m := NewManager(cfg, logger)
	h := NewHandler(m)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	routes := []string{
		"/api/v1/sys/overview",
		"/api/v1/sys/processes",
		"/api/v1/sys/diskusage",
		"/api/v1/sys/network",
		"/api/v1/sys/load",
		"/api/v1/sys/uptime",
		"/api/v1/sys/alerts",
		"/api/v1/sys/history",
	}

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, route := range routes {
		for _, method := range methods {
			t.Run(method+" "+route, func(t *testing.T) {
				req := httptest.NewRequest(method, route, nil)
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, req)

				if rr.Code != http.StatusMethodNotAllowed {
					t.Errorf("Expected status 405, got %d", rr.Code)
				}
			})
		}
	}
}

func TestHandlerOverviewNotReady(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultConfig()
	m := NewManager(cfg, logger)
	h := NewHandler(m)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 不启动 Manager，直接测试 Overview
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sys/overview", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["error"] == "" {
		t.Error("Error message should not be empty")
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	data := map[string]interface{}{
		"key": "value",
		"num": 42,
	}

	writeJSON(rr, http.StatusOK, data)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Expected Content-Type application/json; charset=utf-8, got %s", contentType)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("Expected key=value, got %v", result["key"])
	}
	if result["num"] != float64(42) {
		t.Errorf("Expected num=42, got %v", result["num"])
	}
}

func TestConcurrentAccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &Config{
		Enabled:         true,
		Interval:        1,
		HistoryMaxSize:  100,
		TopProcessCount: 10,
	}
	m := NewManager(cfg, logger)

	err := m.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 等待采集
	time.Sleep(2 * time.Second)

	// 并发读取测试
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			m.GetOverview()
			m.GetProcesses()
			m.GetDiskUsage()
			m.GetNetwork()
			m.GetLoad()
			m.GetUptime()
			m.GetAlerts()
			m.GetHistory()
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 100; i++ {
		<-done
	}
}
