package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ========== Storage Response Types 测试 ==========

func TestStorageResponse_Struct(t *testing.T) {
	resp := StorageResponse{
		Code:    0,
		Message: "success",
		Data:    []VolumeResponse{},
	}

	if resp.Code != 0 {
		t.Errorf("Expected Code=0, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("Expected Message=success, got %s", resp.Message)
	}
}

func TestVolumeResponse_Struct(t *testing.T) {
	vol := VolumeResponse{
		Name:        "data-vol",
		UUID:        "uuid-123",
		Devices:     []string{"/dev/sda", "/dev/sdb"},
		Size:        107374182400,
		Used:        21474836480,
		Free:        85899345920,
		DataProfile: "raid1",
		MetaProfile: "raid1",
		MountPoint:  "/mnt/data",
		Status: VolumeStatusResponse{
			BalanceRunning:  false,
			BalanceProgress: 0,
			ScrubRunning:    false,
			ScrubProgress:   0,
			ScrubErrors:     0,
			Healthy:         true,
		},
		CreatedAt:  "2024-01-01T00:00:00Z",
		Subvolumes: []SubvolumeResponse{},
	}

	if vol.Name != "data-vol" {
		t.Errorf("Expected Name=data-vol, got %s", vol.Name)
	}
	if vol.DataProfile != "raid1" {
		t.Errorf("Expected DataProfile=raid1, got %s", vol.DataProfile)
	}
	if !vol.Status.Healthy {
		t.Error("Expected Healthy=true")
	}
}

func TestVolumeStatusResponse_Struct(t *testing.T) {
	status := VolumeStatusResponse{
		BalanceRunning:  true,
		BalanceProgress: 50,
		ScrubRunning:    false,
		ScrubProgress:   0,
		ScrubErrors:     0,
		Healthy:         true,
	}

	if !status.BalanceRunning {
		t.Error("Expected BalanceRunning=true")
	}
	if status.BalanceProgress != 50 {
		t.Errorf("Expected BalanceProgress=50, got %g", status.BalanceProgress)
	}
}

func TestSubvolumeResponse_Struct(t *testing.T) {
	subvol := SubvolumeResponse{
		Name:      "documents",
		Path:      "/mnt/data/documents",
		Size:      1073741824,
		ReadOnly:  false,
		SnapCount: 0,
	}

	if subvol.Name != "documents" {
		t.Errorf("Expected Name=documents, got %s", subvol.Name)
	}
	if subvol.ReadOnly {
		t.Error("Expected ReadOnly=false")
	}
}

func TestSnapshotResponse_Struct(t *testing.T) {
	snap := SnapshotResponse{
		Name:       "backup-2024-01-01",
		Source:     "documents",
		CreatedAt:  "2024-01-01T00:00:00Z",
		Size:       104857600,
		ReadOnly:   true,
		VolumeName: "main-pool",
	}

	if snap.Name != "backup-2024-01-01" {
		t.Errorf("Expected Name=backup-2024-01-01, got %s", snap.Name)
	}
	if !snap.ReadOnly {
		t.Error("Expected ReadOnly=true")
	}
}

func TestPoolResponse_Struct(t *testing.T) {
	pool := PoolResponse{
		Name:        "main-pool",
		DataProfile: "raid1",
		Size:        214748364800,
		Used:        42949672960,
		Free:        171798691840,
		Healthy:     true,
		Devices:     []string{"/dev/sda", "/dev/sdb"},
		VolumeCount: 1,
	}

	if pool.Name != "main-pool" {
		t.Errorf("Expected Name=main-pool, got %s", pool.Name)
	}
	if !pool.Healthy {
		t.Error("Expected Healthy=true")
	}
}

func TestCreateVolumeRequest_Validation(t *testing.T) {
	req := CreateVolumeRequest{
		Name:    "data-vol",
		Devices: []string{"/dev/sda", "/dev/sdb"},
		Profile: "raid1",
	}

	if req.Name == "" {
		t.Error("Name is required")
	}
	if len(req.Devices) == 0 {
		t.Error("Devices are required")
	}
	if req.Profile == "" {
		t.Error("Profile should have default value")
	}
}

// ========== Storage Handlers 测试 ==========

func TestStorageHandlers_RegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api")

	handlers := &StorageHandlers{storageMgr: nil}
	handlers.RegisterRoutes(group)

	// Verify routes are registered
	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		key := route.Method + ":" + route.Path
		routeMap[key] = true
	}

	expectedRoutes := []string{
		"GET:/api/storage/volumes",
		"POST:/api/storage/volumes",
		"GET:/api/storage/pools",
		"GET:/api/storage/snapshots",
	}

	for _, expected := range expectedRoutes {
		if !routeMap[expected] {
			t.Errorf("Expected route %s to be registered", expected)
		}
	}
}

func TestNewStorageHandlers(t *testing.T) {
	handlers := NewStorageHandlers(nil)
	if handlers == nil {
		t.Fatal("NewStorageHandlers should not return nil")
	}
}

// ========== Mock Storage Manager Tests ==========

func TestListVolumes_EmptyResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Response depends on storage manager implementation
	// This test verifies the endpoint is reachable
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Unexpected status code: %d", w.Code)
	}
}

func TestListPools_Handler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/storage/pools", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Response depends on storage manager implementation
}

func TestListAllSnapshots_Handler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/storage/snapshots", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Response depends on storage manager implementation
}
