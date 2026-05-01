package rdma

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestManager() *RDMAManager {
	config := DefaultConfig()
	config.Enabled = true
	config.FallbackToTCP = true
	return NewRDMAManager(config)
}

func setupTestRouter(manager *RDMAManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	handler := NewHandler(manager)
	handler.RegisterRoutes(v1)
	return router
}

func TestNewRDMAManager(t *testing.T) {
	config := DefaultConfig()
	mgr := NewRDMAManager(config)

	if mgr == nil {
		t.Fatal("NewRDMAManager returned nil")
	}
	if len(mgr.devices) != 0 {
		t.Errorf("expected 0 devices initially, got %d", len(mgr.devices))
	}
	if len(mgr.conns) != 0 {
		t.Errorf("expected 0 connections initially, got %d", len(mgr.conns))
	}
	if mgr.config.FallbackToTCP != true {
		t.Error("expected FallbackToTCP to be true by default")
	}
}

func TestStartStop(t *testing.T) {
	mgr := setupTestManager()

	if err := mgr.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if !mgr.running {
		t.Error("expected manager to be running after Start()")
	}

	// 应扫描到设备
	devices := mgr.GetDevices()
	if len(devices) == 0 {
		t.Error("expected at least 1 RDMA device after start")
	}

	// 重复启动应返回错误
	if err := mgr.Start(); err == nil {
		t.Error("expected error on double Start()")
	}

	mgr.Stop()
	if mgr.running {
		t.Error("expected manager to be stopped after Stop()")
	}
}

func TestConnectionLifecycle(t *testing.T) {
	mgr := setupTestManager()
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer mgr.Stop()

	// 建立连接
	conn, err := mgr.EstablishConnection("mlx5_0", "192.168.1.100:4791", ProtocolISCSI)
	if err != nil {
		t.Fatalf("EstablishConnection failed: %v", err)
	}
	if conn.State != StateActive {
		t.Errorf("expected state active, got %s", conn.State)
	}
	if conn.Protocol != ProtocolISCSI {
		t.Errorf("expected protocol iscsi, got %s", conn.Protocol)
	}
	if conn.Transport != TransportRoCE {
		t.Errorf("expected transport roce, got %s", conn.Transport)
	}

	conns := mgr.GetConnections()
	if len(conns) != 1 {
		t.Errorf("expected 1 connection, got %d", len(conns))
	}

	// 关闭连接
	if err := mgr.CloseConnection(conn.ID); err != nil {
		t.Fatalf("CloseConnection failed: %v", err)
	}
	if len(mgr.conns) != 0 {
		t.Error("expected 0 connections after close")
	}
}

func TestFallbackToTCP(t *testing.T) {
	mgr := setupTestManager()
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer mgr.Stop()

	// 使用不存在的设备，应自动回退到TCP
	conn, err := mgr.EstablishConnection("nonexistent", "192.168.1.200:4791", ProtocolNFS)
	if err != nil {
		t.Fatalf("expected fallback connection, got error: %v", err)
	}
	if !conn.IsFallback {
		t.Error("expected fallback connection")
	}
	if conn.Transport != TransportTCP {
		t.Errorf("expected TCP transport, got %s", conn.Transport)
	}
}

func TestMultipathGroup(t *testing.T) {
	mgr := setupTestManager()
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer mgr.Stop()

	// 创建多条连接
	conn1, _ := mgr.EstablishConnection("mlx5_0", "192.168.1.100:4791", ProtocolISCSI)
	conn2, _ := mgr.EstablishConnection("mlx5_1", "192.168.1.101:4791", ProtocolISCSI)

	// 创建多路径组
	grp, err := mgr.CreateMultipathGroup([]string{conn1.ID, conn2.ID}, "failover")
	if err != nil {
		t.Fatalf("CreateMultipathGroup failed: %v", err)
	}
	if grp.TotalPaths != 2 {
		t.Errorf("expected 2 paths, got %d", grp.TotalPaths)
	}
	if grp.ActivePaths != 2 {
		t.Errorf("expected 2 active paths, got %d", grp.ActivePaths)
	}
	if grp.Policy != "failover" {
		t.Errorf("expected policy failover, got %s", grp.Policy)
	}

	// 验证多路径状态API
	groups := mgr.GetMultipathStatus()
	if len(groups) != 1 {
		t.Errorf("expected 1 multipath group, got %d", len(groups))
	}
}

func TestGetStats(t *testing.T) {
	mgr := setupTestManager()
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer mgr.Stop()

	// 建立一些连接
	mgr.EstablishConnection("mlx5_0", "192.168.1.100:4791", ProtocolISCSI)
	mgr.EstablishConnection("mlx5_1", "192.168.1.101:4791", ProtocolNFS)

	stats := mgr.GetStats()
	if stats.TotalConnections != 2 {
		t.Errorf("expected 2 total connections, got %d", stats.TotalConnections)
	}
	if stats.ActiveConnections != 2 {
		t.Errorf("expected 2 active connections, got %d", stats.ActiveConnections)
	}
	if stats.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestAPIGetStatus(t *testing.T) {
	mgr := setupTestManager()
	mgr.Start()
	defer mgr.Stop()

	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rdma/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["code"].(float64) != 0 {
		t.Errorf("expected code 0, got %v", resp["code"])
	}
}

func TestAPIGetDevices(t *testing.T) {
	mgr := setupTestManager()
	mgr.Start()
	defer mgr.Stop()

	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rdma/devices", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("expected at least 1 device")
	}
}

func TestAPIUpdateConfig(t *testing.T) {
	mgr := setupTestManager()
	mgr.Start()
	defer mgr.Stop()

	router := setupTestRouter(mgr)

	body := `{"fallbackToTcp": false, "maxLatencyMs": 2.0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdma/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// 验证配置已更新
	mgr.mu.RLock()
	if mgr.config.MaxLatencyMs != 2.0 {
		t.Errorf("expected MaxLatencyMs 2.0, got %f", mgr.config.MaxLatencyMs)
	}
	if mgr.config.FallbackToTCP != false {
		t.Error("expected FallbackToTCP to be false")
	}
	mgr.mu.RUnlock()
}

func TestAPIEnableDisable(t *testing.T) {
	mgr := setupTestManager()
	mgr.Start()
	defer mgr.Stop()

	router := setupTestRouter(mgr)

	// 测试禁用
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rdma/disable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for disable, got %d", w.Code)
	}
	if mgr.running {
		t.Error("expected manager to be stopped after disable")
	}

	// 测试启用
	req = httptest.NewRequest(http.MethodPost, "/api/v1/rdma/enable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for enable, got %d", w.Code)
	}
	if !mgr.running {
		t.Error("expected manager to be running after enable")
	}
}
