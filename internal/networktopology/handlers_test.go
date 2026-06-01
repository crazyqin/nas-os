package networktopology

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestService() *TopologyService {
	return &TopologyService{
		devices:     make(map[string]*TopologyDevice),
		topology:    nil,
		risks:       make([]SecurityRisk, 0),
		perfHistory: make(map[string]*PerformanceHistory),
		events:      make([]DeviceEvent, 0),
		monitors:    make(map[string]*MonitorTarget),
		tasks:       make(map[string]*ScanTask),
		maxEvents:   1000,
		maxHistory:  100,
	}
}

func setupTestRouter() (*gin.Engine, *TopologyService) {
	gin.SetMode(gin.TestMode)
	service := setupTestService()
	handler := NewHandler(service)
	router := gin.New()
	rg := router.Group("/api")
	handler.RegisterRoutes(rg)
	return router, service
}

func TestListDevices(t *testing.T) {
	router, service := setupTestRouter()

	// 添加测试设备
	service.devices["d1"] = &TopologyDevice{
		ID:         "d1",
		IP:         "192.168.1.1",
		DeviceType: DeviceTypeRouter,
		State:      DeviceStateOnline,
		FirstSeen:  time.Now(),
		LastSeen:   time.Now(),
	}
	service.devices["d2"] = &TopologyDevice{
		ID:         "d2",
		IP:         "192.168.1.100",
		DeviceType: DeviceTypeNAS,
		State:      DeviceStateOnline,
		FirstSeen:  time.Now(),
		LastSeen:   time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/network-topology/devices", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 2 {
		t.Errorf("expected 2 devices, got %v", resp["total"])
	}
}

func TestAddDevice(t *testing.T) {
	router, service := setupTestRouter()

	device := TopologyDevice{
		IP:         "192.168.1.50",
		DeviceType: DeviceTypeServer,
	}
	body, _ := json.Marshal(device)

	req := httptest.NewRequest(http.MethodPost, "/api/network-topology/devices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	if len(service.devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(service.devices))
	}
}

func TestAddDeviceNoIP(t *testing.T) {
	router, _ := setupTestRouter()

	device := TopologyDevice{DeviceType: DeviceTypeServer}
	body, _ := json.Marshal(device)

	req := httptest.NewRequest(http.MethodPost, "/api/network-topology/devices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateDevice(t *testing.T) {
	router, service := setupTestRouter()

	service.devices["d1"] = &TopologyDevice{
		ID:         "d1",
		IP:         "192.168.1.1",
		DeviceType: DeviceTypeRouter,
		State:      DeviceStateOnline,
		FirstSeen:  time.Now(),
		LastSeen:   time.Now(),
	}

	update := TopologyDevice{Hostname: "my-router"}
	body, _ := json.Marshal(update)

	req := httptest.NewRequest(http.MethodPut, "/api/network-topology/devices/d1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if service.devices["d1"].Hostname != "my-router" {
		t.Errorf("expected hostname 'my-router', got '%s'", service.devices["d1"].Hostname)
	}
}

func TestUpdateDeviceNotFound(t *testing.T) {
	router, _ := setupTestRouter()

	update := TopologyDevice{Hostname: "test"}
	body, _ := json.Marshal(update)

	req := httptest.NewRequest(http.MethodPut, "/api/network-topology/devices/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteDevice(t *testing.T) {
	router, service := setupTestRouter()

	service.devices["d1"] = &TopologyDevice{
		ID:         "d1",
		IP:         "192.168.1.1",
		DeviceType: DeviceTypeRouter,
		FirstSeen:  time.Now(),
		LastSeen:   time.Now(),
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/network-topology/devices/d1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if len(service.devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(service.devices))
	}
}

func TestDeleteDeviceNotFound(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodDelete, "/api/network-topology/devices/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetLinks(t *testing.T) {
	router, service := setupTestRouter()

	service.topology = &NetworkTopology{
		Edges: []TopologyEdge{
			{Source: "d1", Target: "d2", LinkType: "wired"},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/network-topology/links", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 1 {
		t.Errorf("expected 1 link, got %v", resp["total"])
	}
}

func TestGetLinksEmpty(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/network-topology/links", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 0 {
		t.Errorf("expected 0 links, got %v", resp["total"])
	}
}

func TestGetTopology(t *testing.T) {
	router, service := setupTestRouter()

	service.devices["d1"] = &TopologyDevice{
		ID:         "d1",
		IP:         "192.168.1.1",
		DeviceType: DeviceTypeRouter,
		State:      DeviceStateOnline,
		FirstSeen:  time.Now(),
		LastSeen:   time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/network-topology/topology", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp["success"].(bool) {
		t.Error("expected success to be true")
	}
}

func TestGetStats(t *testing.T) {
	router, service := setupTestRouter()

	service.devices["d1"] = &TopologyDevice{
		ID:         "d1",
		IP:         "192.168.1.1",
		DeviceType: DeviceTypeRouter,
		State:      DeviceStateOnline,
		FirstSeen:  time.Now(),
		LastSeen:   time.Now(),
	}
	service.devices["d2"] = &TopologyDevice{
		ID:         "d2",
		IP:         "192.168.1.100",
		DeviceType: DeviceTypeNAS,
		State:      DeviceStateOffline,
		FirstSeen:  time.Now(),
		LastSeen:   time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/network-topology/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if int(data["totalDevices"].(float64)) != 2 {
		t.Errorf("expected 2 total devices, got %v", data["totalDevices"])
	}
	if int(data["onlineDevices"].(float64)) != 1 {
		t.Errorf("expected 1 online device, got %v", data["onlineDevices"])
	}
	if int(data["offlineDevices"].(float64)) != 1 {
		t.Errorf("expected 1 offline device, got %v", data["offlineDevices"])
	}
}

func TestStartScan(t *testing.T) {
	router, service := setupTestRouter()

	scanReq := ScanRequest{Network: "192.168.1.0/24"}
	body, _ := json.Marshal(scanReq)

	req := httptest.NewRequest(http.MethodPost, "/api/network-topology/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}

	if len(service.tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(service.tasks))
	}
}

func TestStartScanNoNetwork(t *testing.T) {
	router, _ := setupTestRouter()

	scanReq := ScanRequest{}
	body, _ := json.Marshal(scanReq)

	req := httptest.NewRequest(http.MethodPost, "/api/network-topology/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
