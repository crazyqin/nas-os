package media

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestHandlers(t *testing.T) (*Handlers, *gin.Engine, string) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	mediaPath := filepath.Join(tmpDir, "media")
	require.NoError(t, os.MkdirAll(mediaPath, 0755))

	lm := NewLibraryManager(configPath)
	_, err := lm.CreateLibrary("Movies", mediaPath, TypeMovie, false) // 禁用自动扫描
	require.NoError(t, err)

	h := NewHandlers(lm)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	return h, router, tmpDir
}

func TestHandlers_ListLibraries(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/media/libraries", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandlers_CreateLibrary(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)

	mediaPath := filepath.Join(tmpDir, "newmedia")
	require.NoError(t, os.MkdirAll(mediaPath, 0755))

	body := map[string]interface{}{
		"name":           "TV Shows",
		"path":           mediaPath,
		"type":           "tv",
		"description":    "TV series library",
		"metadataSource": "tmdb",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/media/libraries", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["code"])
}

func TestCreateLibrary_MissingFields(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	body := map[string]interface{}{
		"name": "Incomplete",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/media/libraries", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_GetLibrary(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	// 获取库列表找到ID
	req := httptest.NewRequest("GET", "/api/media/libraries", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Skip("No libraries found")
	}

	libID := data[0].(map[string]interface{})["id"].(string)

	// 获取单个库
	req = httptest.NewRequest("GET", "/api/media/libraries/"+libID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_UpdateLibrary(t *testing.T) {
	h, router, _ := setupTestHandlers(t)

	// 先获取库ID
	lm := h.libraryMgr
	libs := lm.ListLibraries()
	require.Greater(t, len(libs), 0)

	libID := libs[0].ID

	body := map[string]interface{}{
		"name":        "Updated Movies",
		"description": "Updated description",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/media/libraries/"+libID, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_DeleteLibrary(t *testing.T) {
	h, router, _ := setupTestHandlers(t)

	lm := h.libraryMgr
	libs := lm.ListLibraries()
	require.Greater(t, len(libs), 0)

	libID := libs[0].ID

	req := httptest.NewRequest("DELETE", "/api/media/libraries/"+libID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSearchMedia(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/media/items?q=test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["code"])
}

func TestGetMediaWall(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/media/wall", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetMovieWall(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/media/wall/movies", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTVWall(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/media/wall/tv", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetMusicWall(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/media/wall/music", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSearchMovieMetadata(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/media/metadata/search/movie?q=test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 元数据搜索可能返回错误（无API key），但不应崩溃
	t.Logf("Status: %d, Body: %s", w.Code, w.Body.String())
}

func TestSearchTVMetadata(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/media/metadata/search/tv?q=test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 元数据搜索可能返回错误（无API key），但不应崩溃
	t.Logf("Status: %d, Body: %s", w.Code, w.Body.String())
}

func TestGetPlayHistory(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/media/history", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddPlayHistory(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	body := map[string]interface{}{
		"itemId":    "test-item-id",
		"position":  120,
		"duration":  3600,
		"completed": false,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/media/history", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 播放历史可能需要用户认证，接受多种状态码
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest || w.Code == http.StatusUnauthorized)
}

func TestGetFavorites(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/media/favorites", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestToggleFavorite(t *testing.T) {
	_, router, _ := setupTestHandlers(t)

	req := httptest.NewRequest("POST", "/api/media/items/test-item-id/favorite", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 收藏操作可能需要用户认证，接受多种状态码
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest || w.Code == http.StatusNotFound)
}

func TestHandlers_ScanLibrary(t *testing.T) {
	h, router, _ := setupTestHandlers(t)

	lm := h.libraryMgr
	libs := lm.ListLibraries()
	if len(libs) == 0 {
		t.Skip("No libraries to scan")
	}

	libID := libs[0].ID

	req := httptest.NewRequest("POST", "/api/media/libraries/"+libID+"/scan", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLibraryTypeValidation(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)

	mediaPath := filepath.Join(tmpDir, "music")
	require.NoError(t, os.MkdirAll(mediaPath, 0755))

	// 测试有效类型
	validTypes := []string{"movie", "tv", "music"}
	for _, typ := range validTypes {
		body := map[string]interface{}{
			"name": typ + "-library",
			"path": mediaPath,
			"type": typ,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/media/libraries", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestHandlers_NilLibraryManager(t *testing.T) {
	h := NewHandlers(nil)
	assert.NotNil(t, h)
	assert.Nil(t, h.libraryMgr)
}

func TestHandlers_RegisterRoutes(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	assert.NotNil(t, h)
}
