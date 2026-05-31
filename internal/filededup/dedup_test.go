// Package filededup 存储去重单元测试
package filededup

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

func setupTestManager(t *testing.T) (*ExtendedManager, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "filededup-test")
	require.NoError(t, err)

	config := DefaultConfig()
	config.MinFileSize = 1
	mgr := NewExtendedManager(config)

	return mgr, tmpDir
}

func TestNewExtendedManager(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	require.NotNil(t, mgr)
	assert.NotNil(t, mgr.scans)
	assert.NotNil(t, mgr.scanList)
}

func TestNewHandlers(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	h := NewHandlers(mgr)
	require.NotNil(t, h)
	assert.Equal(t, mgr, h.manager)
}

func TestStartScan(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err := os.WriteFile(file1, []byte("duplicate content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("duplicate content"), 0644)
	require.NoError(t, err)

	config := &ScanConfig{
		Paths: []string{tmpDir},
	}

	result, err := mgr.StartScan(config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.ID)
	assert.Equal(t, ScanStatusCompleted, result.Status)
	assert.GreaterOrEqual(t, result.TotalFiles, 2)
	assert.NotNil(t, result.CompletedAt)
}

func TestGetScanResult(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err := os.WriteFile(file1, []byte("same"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("same"), 0644)
	require.NoError(t, err)

	// 执行扫描
	config := &ScanConfig{Paths: []string{tmpDir}}
	scanResult, err := mgr.StartScan(config)
	require.NoError(t, err)

	// 获取结果
	result, err := mgr.GetScanResult(scanResult.ID)
	require.NoError(t, err)
	assert.Equal(t, scanResult.ID, result.ID)
}

func TestGetScanResult_NotFound(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	result, err := mgr.GetScanResult("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestListScans(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 初始状态应该为空
	scans := mgr.ListScans()
	assert.Len(t, scans, 0)

	// 创建测试文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	err := os.WriteFile(file1, []byte("content"), 0644)
	require.NoError(t, err)

	// 执行扫描
	config := &ScanConfig{Paths: []string{tmpDir}}
	_, err = mgr.StartScan(config)
	require.NoError(t, err)

	// 应该有一个扫描记录
	scans = mgr.ListScans()
	assert.Len(t, scans, 1)
}

func TestDeleteDuplicate(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	content := []byte("duplicate content for delete test")
	err := os.WriteFile(file1, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, content, 0644)
	require.NoError(t, err)

	// 执行扫描
	config := &ScanConfig{Paths: []string{tmpDir}}
	_, err = mgr.StartScan(config)
	require.NoError(t, err)

	// 获取重复组
	groups := mgr.GetDuplicateGroups()
	require.Len(t, groups, 1)

	// 删除重复文件（保留第一个）
	err = mgr.DeleteDuplicate(groups[0].GroupID, 0)
	require.NoError(t, err)

	// 验证文件已被软删除
	softDeleted := mgr.GetSoftDeletedFiles()
	assert.Greater(t, len(softDeleted), 0)
}

func TestDeleteDuplicate_NotFound(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	err := mgr.DeleteDuplicate("nonexistent", 0)
	assert.Error(t, err)
}

func TestGetRecommendations(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err := os.WriteFile(file1, []byte("duplicate content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("duplicate content"), 0644)
	require.NoError(t, err)

	// 执行扫描
	config := &ScanConfig{Paths: []string{tmpDir}}
	_, err = mgr.StartScan(config)
	require.NoError(t, err)

	// 获取建议
	recommendations := mgr.GetRecommendations()
	assert.Len(t, recommendations, 1)
	assert.NotEmpty(t, recommendations[0].KeepFile)
	assert.Greater(t, recommendations[0].WastedSpace, int64(0))
}

func TestGetRecommendations_Empty(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	recommendations := mgr.GetRecommendations()
	assert.Len(t, recommendations, 0)
}

func TestHandlers_RegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	h := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	// 测试路由是否注册
	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		routeMap[route.Method+":"+route.Path] = true
	}

	assert.True(t, routeMap["POST:/api/v1/dedup/scans"])
	assert.True(t, routeMap["GET:/api/v1/dedup/scans"])
	assert.True(t, routeMap["GET:/api/v1/dedup/scans/:id"])
	assert.True(t, routeMap["GET:/api/v1/dedup/scans/:id/groups"])
	assert.True(t, routeMap["DELETE:/api/v1/dedup/files"])
	assert.True(t, routeMap["GET:/api/v1/dedup/recommendations"])
}

func TestHandlers_StartScan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err := os.WriteFile(file1, []byte("duplicate"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("duplicate"), 0644)
	require.NoError(t, err)

	h := NewHandlers(mgr)
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	// 构建请求
	body := ScanConfig{Paths: []string{tmpDir}}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/scans", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result ScanResult
	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
}

func TestHandlers_ListScans(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	h := NewHandlers(mgr)
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/scans", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var scans []*ScanResult
	err := json.Unmarshal(w.Body.Bytes(), &scans)
	require.NoError(t, err)
	assert.Len(t, scans, 0)
}

func TestHandlers_GetScanResult_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	h := NewHandlers(mgr)
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/scans/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetRecommendations(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	h := NewHandlers(mgr)
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/recommendations", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var recommendations []*Recommendation
	err := json.Unmarshal(w.Body.Bytes(), &recommendations)
	require.NoError(t, err)
	assert.Len(t, recommendations, 0)
}

func TestHandlers_DeleteDuplicate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err := os.WriteFile(file1, []byte("duplicate"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("duplicate"), 0644)
	require.NoError(t, err)

	// 执行扫描
	config := &ScanConfig{Paths: []string{tmpDir}}
	_, err = mgr.StartScan(config)
	require.NoError(t, err)

	// 获取重复组
	groups := mgr.GetDuplicateGroups()
	require.Len(t, groups, 1)

	h := NewHandlers(mgr)
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/dedup/files?groupId="+groups[0].GroupID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestScanStatus_Constants(t *testing.T) {
	assert.Equal(t, ScanStatus("pending"), ScanStatusPending)
	assert.Equal(t, ScanStatus("running"), ScanStatusRunning)
	assert.Equal(t, ScanStatus("completed"), ScanStatusCompleted)
	assert.Equal(t, ScanStatus("failed"), ScanStatusFailed)
}

func TestScanConfig_Struct(t *testing.T) {
	config := &ScanConfig{
		Paths:       []string{"/tmp", "/home"},
		MinFileSize: 100,
		MaxFileSize: 1000000,
		IncludeExts: []string{".txt", ".log"},
		Algorithm:   "sha256",
	}

	assert.Len(t, config.Paths, 2)
	assert.Equal(t, int64(100), config.MinFileSize)
	assert.Equal(t, int64(1000000), config.MaxFileSize)
	assert.Contains(t, config.IncludeExts, ".txt")
	assert.Equal(t, "sha256", config.Algorithm)
}

func TestScanResult_Struct(t *testing.T) {
	result := &ScanResult{
		ID:             "scan1",
		Status:         ScanStatusCompleted,
		TotalFiles:     100,
		DuplicateFiles: 10,
		WastedSpace:    5000,
	}

	assert.Equal(t, "scan1", result.ID)
	assert.Equal(t, ScanStatusCompleted, result.Status)
	assert.Equal(t, 100, result.TotalFiles)
	assert.Equal(t, 10, result.DuplicateFiles)
	assert.Equal(t, int64(5000), result.WastedSpace)
	assert.Nil(t, result.Groups)
	assert.Nil(t, result.CompletedAt)
}

func TestRecommendation_Struct(t *testing.T) {
	rec := &Recommendation{
		GroupID:     "group1",
		Hash:        "hash123",
		Files:       []string{"/tmp/file1.txt", "/tmp/file2.txt"},
		KeepFile:    "/tmp/file1.txt",
		WastedSpace: 1024,
		Reason:      "保留最早的文件",
	}

	assert.Equal(t, "group1", rec.GroupID)
	assert.Equal(t, "hash123", rec.Hash)
	assert.Len(t, rec.Files, 2)
	assert.Equal(t, "/tmp/file1.txt", rec.KeepFile)
	assert.Equal(t, int64(1024), rec.WastedSpace)
	assert.Equal(t, "保留最早的文件", rec.Reason)
}
