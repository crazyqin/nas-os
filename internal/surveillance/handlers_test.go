package surveillance

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestHandlers() (*Handlers, *Manager) {
	manager := NewManager()
	handlers := NewHandlers(manager)
	return handlers, manager
}

func TestListCameras(t *testing.T) {
	handlers, _ := setupTestHandlers()
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/cameras", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestAddCamera(t *testing.T) {
	handlers, _ := setupTestHandlers()
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	cam := Camera{
		ID:       "test-cam",
		Name:     "Test Camera",
		Protocol: ProtocolRTSP,
		RTSPUrl:  "rtsp://test:554/stream",
		Host:     "192.168.1.200",
		Port:     554,
		Location: "Test Location",
	}

	body, _ := json.Marshal(cam)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveillance/cameras", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestGetCamera(t *testing.T) {
	handlers, manager := setupTestHandlers()
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// 先添加摄像头
	cam := &Camera{
		ID:       "test-cam",
		Name:     "Test Camera",
		Protocol: ProtocolRTSP,
		RTSPUrl:  "rtsp://test:554/stream",
		Host:     "192.168.1.200",
		Port:     554,
	}
	manager.AddCamera(cam)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/cameras/test-cam", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetStats(t *testing.T) {
	handlers, _ := setupTestHandlers()
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestRecordings(t *testing.T) {
	handlers, manager := setupTestHandlers()
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// 添加摄像头
	cam := &Camera{
		ID:         "test-cam",
		Name:       "Test Camera",
		Protocol:   ProtocolRTSP,
		RTSPUrl:    "rtsp://test:554/stream",
		Host:       "192.168.1.200",
		Port:       554,
		Resolution: "1920x1080",
		Bitrate:    4096,
	}
	manager.AddCamera(cam)

	// 测试列出录像
	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/recordings", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAlerts(t *testing.T) {
	handlers, _ := setupTestHandlers()
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/alerts", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSchedules(t *testing.T) {
	handlers, manager := setupTestHandlers()
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// 添加摄像头
	cam := &Camera{
		ID:       "test-cam",
		Name:     "Test Camera",
		Protocol: ProtocolRTSP,
		RTSPUrl:  "rtsp://test:554/stream",
		Host:     "192.168.1.200",
		Port:     554,
	}
	manager.AddCamera(cam)

	// 测试列出计划
	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/schedules", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
