// Package appleshare 提供 Apple 生态 API 测试
package appleshare

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() (*gin.Engine, *Handlers) {
	gin.SetMode(gin.TestMode)

	config := DefaultAppleShareConfig()
	manager := NewManager(config)
	handlers := NewHandlers(manager)

	r := gin.New()
	v1 := r.Group("/api/v1")
	handlers.RegisterRoutes(v1)

	return r, handlers
}

func TestDiscoverAirPlayDevices(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/apple/airplay/discover", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "discovery completed", resp.Message)
}

func TestCreateTimeMachineShare(t *testing.T) {
	r, _ := setupTestRouter()

	shareReq := CreateTimeMachineShareRequest{
		Name:  "Test Share",
		Path:  "/srv/timemachine/test",
		Quota: 1073741824, // 1GB
	}
	body, _ := json.Marshal(shareReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/apple/timemachine/shares", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.NotNil(t, resp.Data)
}

func TestGetTimeMachineStatus(t *testing.T) {
	r, handlers := setupTestRouter()

	// 先创建一个共享
	share, err := handlers.manager.CreateTimeMachineShare("Test Share", "/srv/timemachine/test", 1073741824)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/apple/timemachine/shares/"+share.ID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestGetTimeMachineStatusNotFound(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/apple/timemachine/shares/non-existent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Code)
}

func TestUpdateSMBConfig(t *testing.T) {
	r, _ := setupTestRouter()

	configReq := UpdateSMBConfigRequest{
		Signing:          true,
		AAPLExtensions:   true,
		Streams:          true,
		VFSFruitEnabled:  true,
		SpotlightEnabled: true,
	}
	body, _ := json.Marshal(configReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/apple/smb/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "SMB config updated", resp.Message)
}

func TestGetSMBConfig(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/apple/smb/config", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestRebuildSpotlightIndex(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/apple/spotlight/rebuild/volume-001", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "spotlight index rebuild started", resp.Message)
}

func TestGetSpotlightStatus(t *testing.T) {
	r, handlers := setupTestRouter()

	// 先触发索引重建
	err := handlers.manager.RebuildSpotlightIndex("volume-001")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/apple/spotlight/status/volume-001", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestGetSpotlightStatusNotFound(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/apple/spotlight/status/non-existent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Code)
}

func TestGetConnectedClients(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/apple/clients", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestListTimeMachineShares(t *testing.T) {
	r, handlers := setupTestRouter()

	// 创建几个共享
	handlers.manager.CreateTimeMachineShare("Share 1", "/srv/tm1", 1073741824)
	handlers.manager.CreateTimeMachineShare("Share 2", "/srv/tm2", 2147483648)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/apple/timemachine/shares", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	shares, ok := resp.Data.([]interface{})
	require.True(t, ok)
	assert.Equal(t, 2, len(shares))
}

func TestListSpotlightIndexes(t *testing.T) {
	r, handlers := setupTestRouter()

	// 触发一些索引重建
	handlers.manager.RebuildSpotlightIndex("vol-1")
	handlers.manager.RebuildSpotlightIndex("vol-2")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/apple/spotlight/indexes", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestListDevices(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/apple/devices", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestCreateDuplicateTimeMachineShare(t *testing.T) {
	r, handlers := setupTestRouter()

	// 先创建一个共享
	handlers.manager.CreateTimeMachineShare("Duplicate", "/srv/tm", 1073741824)

	// 尝试创建同名共享
	shareReq := CreateTimeMachineShareRequest{
		Name:  "Duplicate",
		Path:  "/srv/tm2",
		Quota: 1073741824,
	}
	body, _ := json.Marshal(shareReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/apple/timemachine/shares", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Code)
	assert.Contains(t, resp.Message, "already exists")
}
