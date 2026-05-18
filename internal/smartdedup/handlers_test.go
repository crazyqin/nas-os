// Package smartdedup 提供内容感知的智能文件去重功能
package smartdedup

import (
	"bytes"
	"context"
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

func setupTestRouter(t *testing.T) (*gin.Engine, *Manager, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tmpDir, err := os.MkdirTemp("", "smartdedup-handler-test")
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "smartdedup.json")
	cfg := testConfig()
	cfg.MinFileSize = 0 // 允许小文件用于测试
	mgr, err := NewManager(configPath, cfg)
	require.NoError(t, err)

	router := gin.New()
	handlers := NewHandlers(mgr)
	handlers.RegisterRoutes(router.Group("/api/v1"))

	return router, mgr, tmpDir
}

func TestNewHandlers(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	h := NewHandlers(mgr)
	assert.NotNil(t, h)
}

func TestHandlers_RegisterRoutes(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 验证路由注册（不 panic 即成功）
	assert.NotNil(t, router)
}

func TestHandlers_Scan(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	content := []byte("test content for scan")
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err := os.WriteFile(file1, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, content, 0644)
	require.NoError(t, err)

	body := ScanRequest{
		Paths: []string{tmpDir},
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/smart-dedup/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["filesScanned"])
}

func TestHandlers_Scan_EmptyBody(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/smart-dedup/scan", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 应返回错误（无路径）
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_CancelScan(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/smart-dedup/scan/cancel", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandlers_Dedup(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	content := []byte("dedup test content")
	source := filepath.Join(tmpDir, "source.txt")
	target := filepath.Join(tmpDir, "target.txt")
	err := os.WriteFile(source, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(target, content, 0644)
	require.NoError(t, err)

	body := DedupRequest{
		Groups: []DuplicateGroup{
			{
				ContentHash: "test-hash",
				Files:       []string{source, target},
				FileCount:   2,
				UniqueSize:  int64(len(content)),
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/smart-dedup/dedup", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandlers_Dedup_InvalidBody(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/smart-dedup/dedup", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_CancelDedup(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/smart-dedup/dedup/cancel", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetDuplicates(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/duplicates", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(0), data["total"])
}

func TestHandlers_GetStats(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandlers_ListEntries(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/entries", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(0), data["total"])
}

func TestHandlers_GetEntry_NotFound(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/entries/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_ListRefs(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/refs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetRef_NotFound(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/refs/nonexistent-hash", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetConfig(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandlers_UpdateConfig(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	enabled := false
	maxWorkers := 8
	body := updateConfigRequest{
		Enabled:    &enabled,
		MaxWorkers: &maxWorkers,
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/smart-dedup/config", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, false, data["enabled"])
}

func TestHandlers_UpdateConfig_InvalidBody(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/smart-dedup/config", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_DetectBackend(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/backend/detect?path="+tmpDir, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_DetectBackend_DefaultPath(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/backend/detect", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_Scan_WithStats(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建文件
	content := []byte("stats test content")
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), content, 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), content, 0644)

	// 执行扫描
	body := ScanRequest{Paths: []string{tmpDir}}
	bodyBytes, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/smart-dedup/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 检查统计
	w = httptest.NewRecorder()
	req, _ = http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/stats", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["totalFilesScanned"])
}

func TestHandlers_FullWorkflow(t *testing.T) {
	router, _, tmpDir := setupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 1. 创建重复文件
	content := []byte("workflow test content")
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file1, content, 0644)
	os.WriteFile(file2, content, 0644)

	// 2. 扫描
	scanBody, _ := json.Marshal(ScanRequest{Paths: []string{tmpDir}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/smart-dedup/scan", bytes.NewReader(scanBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 3. 检查重复组
	w = httptest.NewRecorder()
	req, _ = http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/duplicates", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 4. 检查统计
	w = httptest.NewRecorder()
	req, _ = http.NewRequestWithContext(context.Background(), "GET", "/api/v1/smart-dedup/stats", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var statsResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &statsResp)
	statsData := statsResp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), statsData["totalFilesScanned"])
	assert.True(t, statsData["totalSizeScanned"].(float64) > 0)
}
