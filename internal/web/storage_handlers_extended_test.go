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
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== convertSubvolumes 测试 ==========

func TestConvertSubvolumes(t *testing.T) {
	subvolumes := []*storage.SubVolume{
		{
			ID:        256,
			Name:      "documents",
			Path:      "/mnt/data/documents",
			ParentID:  5,
			ReadOnly:  false,
			UUID:      "subvol-uuid-1",
			Size:      1073741824,
			Snapshots: []*storage.Snapshot{},
		},
		{
			ID:        257,
			Name:      "photos",
			Path:      "/mnt/data/photos",
			ParentID:  5,
			ReadOnly:  true,
			UUID:      "subvol-uuid-2",
			Size:      2147483648,
			Snapshots: []*storage.Snapshot{{Name: "snap1"}},
		},
	}

	result := convertSubvolumes(subvolumes)

	assert.Len(t, result, 2)
	assert.Equal(t, uint64(256), result[0].ID)
	assert.Equal(t, "documents", result[0].Name)
	assert.False(t, result[0].ReadOnly)
	assert.Equal(t, 0, result[0].SnapCount)

	assert.Equal(t, "photos", result[1].Name)
	assert.True(t, result[1].ReadOnly)
	assert.Equal(t, 1, result[1].SnapCount)
}

func TestConvertSubvolumes_Empty(t *testing.T) {
	result := convertSubvolumes(nil)
	assert.Empty(t, result)

	result = convertSubvolumes([]*storage.SubVolume{})
	assert.Empty(t, result)
}

// ========== ListVolumes 详细测试 ==========

func TestListVolumes_WithMockManager(t *testing.T) {
	// 创建一个不使用真实 storage manager 的测试
	router := gin.New()
	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/storage/volumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp StorageResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// ========== CreateVolume 测试 ==========

func TestCreateVolume_NilManager(t *testing.T) {
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

	req := httptest.NewRequest("POST", "/api/storage/volumes", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// nil manager should return 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateVolume_InvalidJSON(t *testing.T) {
	router := gin.New()
	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/storage/volumes", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateVolume_MissingFields(t *testing.T) {
	router := gin.New()
	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// Missing devices
	body := map[string]interface{}{
		"name": "test-vol",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/storage/volumes", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Binding validation should fail
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== ListPools 测试 ==========

func TestListPools_NilManager(t *testing.T) {
	router := gin.New()
	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/storage/pools", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp StorageResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// ========== ListAllSnapshots 测试 ==========

func TestListAllSnapshots_NilManager(t *testing.T) {
	router := gin.New()
	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/storage/snapshots", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp StorageResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestListAllSnapshots_WithVolumeFilter(t *testing.T) {
	router := gin.New()
	handlers := &StorageHandlers{storageMgr: nil}
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/storage/snapshots?volume=main-pool", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ========== API Types 测试 ==========

func TestRespondSuccess(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		respondSuccess(c, map[string]string{"key": "value"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
	assert.Equal(t, "success", resp["message"])
}

func TestRespondBadRequest(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		respondBadRequest(c, "invalid request")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRespondNotFound(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		respondNotFound(c, "resource not found")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRespondInternalError(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		respondInternalError(c, "internal error")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRespondServiceUnavailable(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		respondServiceUnavailable(c, "service unavailable")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestRespondError(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		respondError(c, 418, 1001, "I'm a teapot")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTeapot, w.Code)
}