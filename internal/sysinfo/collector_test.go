package sysinfo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector(nil)
	if c == nil {
		t.Fatal("NewCollector returned nil")
	}

	logger := zap.NewNop()
	c2 := NewCollector(logger)
	if c2 == nil {
		t.Fatal("NewCollector with logger returned nil")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
		{1099511627776, "1.0 TiB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.input)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDiskHealth(t *testing.T) {
	tests := []struct {
		usage    float64
		expected string
	}{
		{0, "healthy"},
		{50, "healthy"},
		{84.9, "healthy"},
		{85, "warning"},
		{90, "warning"},
		{94.9, "warning"},
		{95, "critical"},
		{99, "critical"},
		{100, "critical"},
	}

	for _, tt := range tests {
		result := CalcDiskHealth(tt.usage)
		if result != tt.expected {
			t.Errorf("CalcDiskHealth(%f) = %q, want %q", tt.usage, result, tt.expected)
		}
	}
}

func TestMemoryUsagePercent(t *testing.T) {
	tests := []struct {
		total    int64
		used     int64
		expected float64
	}{
		{0, 0, 0},
		{100, 0, 0},
		{100, 50, 50},
		{100, 75, 75},
		{100, 100, 100},
		{8 * 1024 * 1024 * 1024, 4 * 1024 * 1024 * 1024, 50},
	}

	for _, tt := range tests {
		result := CalcMemUsagePercent(tt.total, tt.used)
		// 允许浮点误差
		if diff := result - tt.expected; diff > 0.001 || diff < -0.001 {
			t.Errorf("CalcMemUsagePercent(%d, %d) = %f, want %f", tt.total, tt.used, result, tt.expected)
		}
	}
}

func TestHandler_SysInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := zap.NewNop()
	collector := NewCollector(logger)
	handlers := NewHandlers(collector, logger)

	r := gin.New()
	api := r.Group("/api/v1")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/sysinfo", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}

	if resp.Message != "success" {
		t.Errorf("expected message 'success', got %q", resp.Message)
	}

	// 验证返回数据是 SystemInfo 结构
	data, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal data: %v", err)
	}

	var info SystemInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("failed to unmarshal SystemInfo: %v", err)
	}

	if info.Hostname == "" {
		t.Error("hostname should not be empty")
	}

	if info.Arch == "" {
		t.Error("arch should not be empty")
	}
}

func TestHandler_CPU(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := zap.NewNop()
	collector := NewCollector(logger)
	handlers := NewHandlers(collector, logger)

	r := gin.New()
	api := r.Group("/api/v1")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/sysinfo/cpu", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}

	data, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal data: %v", err)
	}

	var cpu CPUInfo
	if err := json.Unmarshal(data, &cpu); err != nil {
		t.Fatalf("failed to unmarshal CPUInfo: %v", err)
	}

	if cpu.Cores <= 0 {
		t.Errorf("expected cores > 0, got %d", cpu.Cores)
	}
}

func TestHandler_Memory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := zap.NewNop()
	collector := NewCollector(logger)
	handlers := NewHandlers(collector, logger)

	r := gin.New()
	api := r.Group("/api/v1")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/sysinfo/memory", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_Disks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := zap.NewNop()
	collector := NewCollector(logger)
	handlers := NewHandlers(collector, logger)

	r := gin.New()
	api := r.Group("/api/v1")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/sysinfo/disks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_Network(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := zap.NewNop()
	collector := NewCollector(logger)
	handlers := NewHandlers(collector, logger)

	r := gin.New()
	api := r.Group("/api/v1")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/sysinfo/network", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}
