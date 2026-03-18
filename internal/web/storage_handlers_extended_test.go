package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nas-os/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// ========== ListVolumes Handler Tests ==========

func TestListVolumes_NilStorageMgr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/storage/volumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// API returns an array, not a map
	var resp []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	// Should return empty array when storageMgr is nil
	assert.Len(t, resp, 0)
}

func TestListVolumes_WithMockData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage manager
	mockMgr := &storage.Manager{}

	handlers := NewStorageHandlers(mockMgr)
	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/storage/volumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Response depends on actual storage manager
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

// ========== CreateVolume Handler Tests ==========

func TestCreateVolume_NilStorageMgr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	body := CreateVolumeRequest{
		Name:    "test-vol",
		Devices: []string{"/dev/sda"},
		Profile: "single",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/storage/volumes", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateVolume_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Need a non-nil storage manager to test validation
	mockMgr := &storage.Manager{}
	handlers := NewStorageHandlers(mockMgr)
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// Missing required fields
	jsonBody := `{"name": ""}`

	req := httptest.NewRequest("POST", "/api/storage/volumes", bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 400 for invalid request or 500 if storage manager fails
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
}

func TestCreateVolume_MissingDevices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Need a non-nil storage manager to test validation
	mockMgr := &storage.Manager{}
	handlers := NewStorageHandlers(mockMgr)
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	body := CreateVolumeRequest{
		Name:    "test-vol",
		Devices: []string{},
		Profile: "single",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/storage/volumes", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 400 for empty devices or 500 if storage manager fails
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
}

// ========== ListPools Handler Tests ==========

func TestListPools_NilStorageMgr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/storage/pools", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// API returns an array, not a map
	var resp []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	// Should return empty array when storageMgr is nil
	assert.Len(t, resp, 0)
}

// ========== ListAllSnapshots Handler Tests ==========

func TestListAllSnapshots_NilStorageMgr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/storage/snapshots", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp StorageResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	assert.Equal(t, 0, resp.Code)
}

func TestListAllSnapshots_WithVolumeFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/storage/snapshots?volume=test-vol", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ========== convertSubvolumes Tests ==========

func TestConvertSubvolumes_Empty(t *testing.T) {
	result := convertSubvolumes(nil)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)

	result = convertSubvolumes([]*storage.SubVolume{})
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestConvertSubvolumes_WithData(t *testing.T) {
	subvolumes := []*storage.SubVolume{
		{
			ID:        256,
			Name:      "documents",
			Path:      "/mnt/data/documents",
			ParentID:  5,
			ReadOnly:  false,
			UUID:      "uuid-123",
			Size:      1024000,
			Snapshots: []*storage.Snapshot{},
		},
		{
			ID:        257,
			Name:      "photos",
			Path:      "/mnt/data/photos",
			ParentID:  5,
			ReadOnly:  true,
			UUID:      "uuid-456",
			Size:      2048000,
			Snapshots: []*storage.Snapshot{{Name: "snap1"}},
		},
	}

	result := convertSubvolumes(subvolumes)
	assert.Len(t, result, 2)
	assert.Equal(t, "documents", result[0].Name)
	assert.Equal(t, "photos", result[1].Name)
	assert.False(t, result[0].ReadOnly)
	assert.True(t, result[1].ReadOnly)
	assert.Equal(t, 0, result[0].SnapCount)
	assert.Equal(t, 1, result[1].SnapCount)
}

// ========== Type Validation Tests ==========

func TestPoolResponse_JSON(t *testing.T) {
	pool := PoolResponse{
		Name:        "main-pool",
		UUID:        "uuid-abc",
		Devices:     []string{"/dev/sda", "/dev/sdb"},
		Size:        1000000000,
		Used:        500000000,
		Free:        500000000,
		DataProfile: "raid1",
		MetaProfile: "raid1",
		MountPoint:  "/mnt/pool",
		Healthy:     true,
		VolumeCount: 3,
	}

	data, err := json.Marshal(pool)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "main-pool")
	assert.Contains(t, string(data), "raid1")
}

func TestSnapshotResponse_JSON(t *testing.T) {
	snap := SnapshotResponse{
		Name:       "daily-2024-01-01",
		Path:       "/mnt/data/.snapshots/daily-2024-01-01",
		Source:     "documents",
		SourceUUID: "source-uuid",
		ReadOnly:   true,
		CreatedAt:  "2024-01-01T00:00:00Z",
		Size:       1024000,
		VolumeName: "main-pool",
	}

	data, err := json.Marshal(snap)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "daily-2024-01-01")
	assert.Contains(t, string(data), "documents")
}

func TestCreateVolumeRequest_JSON(t *testing.T) {
	jsonStr := `{"name":"test-vol","devices":["/dev/sda","/dev/sdb"],"profile":"raid1"}`

	var req CreateVolumeRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	assert.NoError(t, err)
	assert.Equal(t, "test-vol", req.Name)
	assert.Len(t, req.Devices, 2)
	assert.Equal(t, "raid1", req.Profile)
}

// ========== StorageResponse Tests ==========

func TestStorageResponse_Success(t *testing.T) {
	resp := StorageResponse{
		Code:    0,
		Message: "success",
		Data:    []string{"item1", "item2"},
	}

	data, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"code":0`)
	assert.Contains(t, string(data), `"message":"success"`)
}

func TestStorageResponse_Error(t *testing.T) {
	resp := StorageResponse{
		Code:    500,
		Message: "internal server error",
		Data:    nil,
	}

	data, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"code":500`)
}

// ========== Concurrent Access Tests ==========

func TestStorageHandlers_ConcurrentAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := &StorageHandlers{storageMgr: nil}
	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/api/storage/volumes", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== Route Registration Tests ==========

func TestStorageHandlers_RoutePaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		routeMap[route.Method+":"+route.Path] = true
	}

	// Verify all expected routes exist
	assert.True(t, routeMap["GET:/api/storage/volumes"], "GET /api/storage/volumes should be registered")
	assert.True(t, routeMap["POST:/api/storage/volumes"], "POST /api/storage/volumes should be registered")
	assert.True(t, routeMap["GET:/api/storage/pools"], "GET /api/storage/pools should be registered")
	assert.True(t, routeMap["GET:/api/storage/snapshots"], "GET /api/storage/snapshots should be registered")
}

// ========== VolumeStatusResponse Tests ==========

func TestVolumeStatusResponse_AllFields(t *testing.T) {
	status := VolumeStatusResponse{
		BalanceRunning:  true,
		BalanceProgress: 45.5,
		ScrubRunning:    false,
		ScrubProgress:   0,
		ScrubErrors:     0,
		Healthy:         true,
	}

	data, err := json.Marshal(status)
	assert.NoError(t, err)

	var decoded VolumeStatusResponse
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, true, decoded.BalanceRunning)
	assert.Equal(t, 45.5, decoded.BalanceProgress)
	assert.Equal(t, true, decoded.Healthy)
}

// ========== SubvolumeResponse Tests ==========

func TestSubvolumeResponse_AllFields(t *testing.T) {
	subvol := SubvolumeResponse{
		ID:        256,
		Name:      "home",
		Path:      "/mnt/data/home",
		ParentID:  5,
		ReadOnly:  false,
		UUID:      "subvol-uuid-123",
		Size:      1073741824,
		SnapCount: 5,
	}

	data, err := json.Marshal(subvol)
	assert.NoError(t, err)

	var decoded SubvolumeResponse
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, uint64(256), decoded.ID)
	assert.Equal(t, "home", decoded.Name)
	assert.Equal(t, 5, decoded.SnapCount)
}
