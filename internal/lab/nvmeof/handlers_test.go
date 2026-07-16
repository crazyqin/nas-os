package nvmeof

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestHandler(t *testing.T) (*Handler, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")

	mgr, err := NewManager(configPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	handler := NewHandler(mgr, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, mgr, router
}

func TestCreateSubsystem(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{"nqn":"nqn.2026-01.com.nas-os:test-sub","serial_number":"TEST-001","model_number":"Test Model","max_namespaces":16}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/subsystems", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var subsys Subsystem
	if err := json.Unmarshal(w.Body.Bytes(), &subsys); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if subsys.NQN != "nqn.2026-01.com.nas-os:test-sub" {
		t.Errorf("expected NQN nqn.2026-01.com.nas-os:test-sub, got %s", subsys.NQN)
	}
	if subsys.SerialNumber != "TEST-001" {
		t.Errorf("expected serial TEST-001, got %s", subsys.SerialNumber)
	}
	if subsys.MaxNamespaces != 16 {
		t.Errorf("expected max_ns 16, got %d", subsys.MaxNamespaces)
	}
}

func TestCreateSubsystemDuplicate(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{"nqn":"nqn.2026-01.com.nas-os:dup"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/subsystems", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d", w.Code)
	}

	// Duplicate
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/subsystems", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("duplicate: expected 409, got %d", w2.Code)
	}
}

func TestCreateSubsystemInvalidNQN(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{"nqn":"bad-nqn"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/subsystems", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSubsystemMissingBody(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/subsystems", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListSubsystems(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// Create two subsystems
	_, _ = mgr.CreateSubsystem(nil, "nqn.2026-01.com.nas-os:list1", "S1", "M1", 8)
	_, _ = mgr.CreateSubsystem(nil, "nqn.2026-01.com.nas-os:list2", "S2", "M2", 16)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/subsystems", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Subsystems []*Subsystem `json:"subsystems"`
		Total      int          `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("expected 2 subsystems, got %d", resp.Total)
	}
}

func TestGetSubsystem(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	_, _ = mgr.CreateSubsystem(nil, "nqn.2026-01.com.nas-os:get-test", "SN", "MN", 8)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/subsystems/nqn.2026-01.com.nas-os:get-test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSubsystemNotFound(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/subsystems/nqn.2026-01.com.nas-os:nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteSubsystem(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	_, _ = mgr.CreateSubsystem(nil, "nqn.2026-01.com.nas-os:del-test", "SN", "MN", 8)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/nvmeof/subsystems/nqn.2026-01.com.nas-os:del-test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify deleted
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/subsystems/nqn.2026-01.com.nas-os:del-test", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w2.Code)
	}
}

func TestDeleteSubsystemWithNamespaces(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	nqn := "nqn.2026-01.com.nas-os:del-with-ns"
	_, _ = mgr.CreateSubsystem(nil, nqn, "SN", "MN", 8)

	// Create a temp file for namespace
	tmpFile := filepath.Join(t.TempDir(), "ns.img")
	os.WriteFile(tmpFile, make([]byte, 1024*1024), 0644)
	_, _ = mgr.AddNamespace(nil, nqn, tmpFile, 1024*1024, 4096)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/nvmeof/subsystems/"+nqn, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAllowHost(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	nqn := "nqn.2026-01.com.nas-os:host-test"
	_, _ = mgr.CreateSubsystem(nil, nqn, "SN", "MN", 8)

	body := `{"host_nqn":"nqn.2026-01.com.nas-os:host:client1"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/nvmeof/subsystems/"+nqn+"/allow-host", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify host was added
	subsys, _ := mgr.GetSubsystem(nqn)
	if len(subsys.Hosts) != 1 || subsys.Hosts[0] != "nqn.2026-01.com.nas-os:host:client1" {
		t.Errorf("host not added correctly: %v", subsys.Hosts)
	}
}

func TestRevokeHost(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	nqn := "nqn.2026-01.com.nas-os:revoke-test"
	_, _ = mgr.CreateSubsystem(nil, nqn, "SN", "MN", 8)
	_ = mgr.AllowHost(nil, nqn, "nqn.2026-01.com.nas-os:host:client1")

	body := `{"host_nqn":"nqn.2026-01.com.nas-os:host:client1"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/nvmeof/subsystems/"+nqn+"/revoke-host", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	subsys, _ := mgr.GetSubsystem(nqn)
	if len(subsys.Hosts) != 0 {
		t.Errorf("expected 0 hosts after revoke, got %d", len(subsys.Hosts))
	}
}

func TestRevokeHostNotFound(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	nqn := "nqn.2026-01.com.nas-os:revoke-nf"
	_, _ = mgr.CreateSubsystem(nil, nqn, "SN", "MN", 8)

	body := `{"host_nqn":"nqn.2026-01.com.nas-os:host:ghost"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/nvmeof/subsystems/"+nqn+"/revoke-host", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAddNamespace(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	nqn := "nqn.2026-01.com.nas-os:ns-test"
	_, _ = mgr.CreateSubsystem(nil, nqn, "SN", "MN", 8)

	tmpFile := filepath.Join(t.TempDir(), "vol.img")
	os.WriteFile(tmpFile, make([]byte, 4096), 0644)

	body := `{"path":"` + tmpFile + `","size":1073741824,"block_size":4096}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/subsystems/"+nqn+"/namespaces", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var ns Namespace
	if err := json.Unmarshal(w.Body.Bytes(), &ns); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if ns.ID != 1 {
		t.Errorf("expected ns id 1, got %d", ns.ID)
	}
	if ns.Size != 1073741824 {
		t.Errorf("expected size 1073741824, got %d", ns.Size)
	}
}

func TestAddNamespaceDefaultBlockSize(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	nqn := "nqn.2026-01.com.nas-os:ns-default"
	_, _ = mgr.CreateSubsystem(nil, nqn, "SN", "MN", 8)

	tmpFile := filepath.Join(t.TempDir(), "vol2.img")
	os.WriteFile(tmpFile, make([]byte, 4096), 0644)

	body := `{"path":"` + tmpFile + `","size":536870912}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/subsystems/"+nqn+"/namespaces", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var ns Namespace
	json.Unmarshal(w.Body.Bytes(), &ns)
	if ns.BlockSize != 4096 {
		t.Errorf("expected default block_size 4096, got %d", ns.BlockSize)
	}
}

func TestRemoveNamespace(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	nqn := "nqn.2026-01.com.nas-os:rm-ns"
	_, _ = mgr.CreateSubsystem(nil, nqn, "SN", "MN", 8)

	tmpFile := filepath.Join(t.TempDir(), "vol3.img")
	os.WriteFile(tmpFile, make([]byte, 4096), 0644)
	_, _ = mgr.AddNamespace(nil, nqn, tmpFile, 536870912, 4096)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/nvmeof/subsystems/"+nqn+"/namespaces/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRemoveNamespaceInvalidID(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	nqn := "nqn.2026-01.com.nas-os:rm-ns-invalid"
	_, _ = mgr.CreateSubsystem(nil, nqn, "SN", "MN", 8)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/nvmeof/subsystems/"+nqn+"/namespaces/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreatePort(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{"transport":"tcp","address":"127.0.0.1","port":14420}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/ports", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var port Port
	if err := json.Unmarshal(w.Body.Bytes(), &port); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if port.Status != "listening" {
		t.Errorf("expected status listening, got %s", port.Status)
	}
	if port.Port != 14420 {
		t.Errorf("expected port 14420, got %d", port.Port)
	}
}

func TestCreatePortDefaultTransport(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{"address":"127.0.0.1","port":14421}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/ports", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var port Port
	json.Unmarshal(w.Body.Bytes(), &port)
	if port.Transport != "tcp" {
		t.Errorf("expected default transport tcp, got %s", port.Transport)
	}
}

func TestListPorts(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	_, _ = mgr.CreatePort(nil, "tcp", "127.0.0.1", 14422)
	_, _ = mgr.CreatePort(nil, "tcp", "127.0.0.1", 14423)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/ports", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Ports []*Port `json:"ports"`
		Total int     `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 2 {
		t.Errorf("expected 2 ports, got %d", resp.Total)
	}
}

func TestStopPort(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	port, _ := mgr.CreatePort(nil, "tcp", "127.0.0.1", 14424)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/nvmeof/ports/"+port.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStopPortNotFound(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/nvmeof/ports/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetStats(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	_, _ = mgr.CreateSubsystem(nil, "nqn.2026-01.com.nas-os:stats1", "SN", "MN", 8)
	_, _ = mgr.CreatePort(nil, "tcp", "127.0.0.1", 14425)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &stats)

	if stats["subsystems"] != float64(1) {
		t.Errorf("expected 1 subsystem, got %v", stats["subsystems"])
	}
	if stats["ports"] != float64(1) {
		t.Errorf("expected 1 port, got %v", stats["ports"])
	}
}
