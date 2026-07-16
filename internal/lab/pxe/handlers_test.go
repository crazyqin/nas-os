package pxe

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Handler) {
	gin.SetMode(gin.TestMode)
	manager := NewPXEManager()
	handler := NewHandler(manager)
	r := gin.New()
	api := r.Group("/api/v1")
	handler.RegisterRoutes(api)
	return r, handler
}

func TestHandleGetConfig(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pxe/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var cfg PXEConfig
	if err := json.NewDecoder(w.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if cfg.TFTPPath == "" {
		t.Error("expected TFTP path to be set")
	}
	if cfg.HTTPPath == "" {
		t.Error("expected HTTP path to be set")
	}
	if cfg.DHCPRange == "" {
		t.Error("expected DHCP range to be set")
	}
}

func TestHandleUpdateConfig(t *testing.T) {
	r, _ := setupTestRouter()

	newPath := "/opt/pxe/tftpboot"
	reqBody := CreatePXEConfigRequest{TFTPPath: &newPath}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/pxe/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var cfg PXEConfig
	if err := json.NewDecoder(w.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if cfg.TFTPPath != newPath {
		t.Errorf("expected TFTP path '%s', got '%s'", newPath, cfg.TFTPPath)
	}
}

func TestHandleListClients(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pxe/clients", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var clients []PXEClient
	if err := json.NewDecoder(w.Body).Decode(&clients); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(clients) < 4 {
		t.Errorf("expected at least 4 clients, got %d", len(clients))
	}
}

func TestHandleGetClient(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pxe/clients/aa:bb:cc:dd:ee:01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var client PXEClient
	if err := json.NewDecoder(w.Body).Decode(&client); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if client.MACAddress != "aa:bb:cc:dd:ee:01" {
		t.Errorf("expected MAC 'aa:bb:cc:dd:ee:01', got '%s'", client.MACAddress)
	}
	if client.Hostname != "node-01" {
		t.Errorf("expected hostname 'node-01', got '%s'", client.Hostname)
	}
}

func TestHandleGetClientNotFound(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pxe/clients/ff:ff:ff:ff:ff:ff", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHandleAddImage(t *testing.T) {
	r, handler := setupTestRouter()

	img := PXEImage{
		ID:   "img-test-alpine",
		Name: "Alpine Linux 3.20",
		Path: "/var/lib/pxe/images/alpine-320",
		Size: 134217728,
		Type: "linux",
	}
	body, _ := json.Marshal(img)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/pxe/images", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	// Verify image exists
	got, err := handler.manager.GetImage("img-test-alpine")
	if err != nil {
		t.Fatalf("image not found after add: %v", err)
	}
	if got.Name != "Alpine Linux 3.20" {
		t.Errorf("expected name 'Alpine Linux 3.20', got '%s'", got.Name)
	}
}

func TestHandleRemoveImage(t *testing.T) {
	r, handler := setupTestRouter()

	// First add an image
	img := PXEImage{ID: "img-to-delete", Name: "Temp Image", Type: "linux"}
	handler.manager.AddBootImage(img)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pxe/images/img-to-delete", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify removed
	_, err := handler.manager.GetImage("img-to-delete")
	if err == nil {
		t.Error("expected error after removing image, got nil")
	}
}

func TestHandleListImages(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pxe/images", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var images []PXEImage
	if err := json.NewDecoder(w.Body).Decode(&images); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(images) < 3 {
		t.Errorf("expected at least 3 images, got %d", len(images))
	}
}

func TestHandleSetBootMenu(t *testing.T) {
	r, handler := setupTestRouter()

	menu := []BootMenuItem{
		{ID: "entry-1", Label: "Ubuntu", ImageID: "img-ubuntu-2404", Kernel: "/vmlinuz", Default: true},
		{ID: "entry-2", Label: "Rescue", ImageID: "img-rescue", Kernel: "/rescue/vmlinuz"},
	}
	body, _ := json.Marshal(menu)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/pxe/boot-menu", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	cfg := handler.manager.GetConfig()
	if len(cfg.BootMenu) != 2 {
		t.Errorf("expected 2 boot menu entries, got %d", len(cfg.BootMenu))
	}
}

func TestHandleGetStats(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pxe/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var stats PXEStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if stats.TotalClients < 4 {
		t.Errorf("expected at least 4 total clients, got %d", stats.TotalClients)
	}
	if stats.TotalImages < 3 {
		t.Errorf("expected at least 3 total images, got %d", stats.TotalImages)
	}
	if stats.BootSuccessRate <= 0 {
		t.Error("expected positive boot success rate")
	}
}

func TestHandleStartStop(t *testing.T) {
	r, handler := setupTestRouter()

	// Start
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pxe/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d on start, got %d", http.StatusOK, w.Code)
	}
	if handler.manager.server.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", handler.manager.server.Status)
	}

	// Start again (should conflict)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusConflict {
		t.Errorf("expected status %d on double-start, got %d", http.StatusConflict, w2.Code)
	}

	// Stop
	stopReq := httptest.NewRequest(http.MethodPost, "/api/v1/pxe/stop", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, stopReq)

	if w3.Code != http.StatusOK {
		t.Errorf("expected status %d on stop, got %d", http.StatusOK, w3.Code)
	}
	if handler.manager.server.Status != "stopped" {
		t.Errorf("expected status 'stopped', got '%s'", handler.manager.server.Status)
	}
}

func TestHandleUpdateClient(t *testing.T) {
	r, handler := setupTestRouter()

	newHostname := "renamed-node"
	reqBody := UpdateClientRequest{Hostname: &newHostname}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/pxe/clients/aa:bb:cc:dd:ee:02", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	client, _ := handler.manager.GetClientByMAC("aa:bb:cc:dd:ee:02")
	if client.Hostname != "renamed-node" {
		t.Errorf("expected hostname 'renamed-node', got '%s'", client.Hostname)
	}
}
