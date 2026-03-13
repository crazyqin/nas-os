package dedup

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

func setupDedupTestRouter(t *testing.T) (*gin.Engine, *Manager, string) {
	gin.SetMode(gin.TestMode)

	tmpDir, err := os.MkdirTemp("", "dedup-handler-test")
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "dedup.json")

	mgr, err := NewManager(configPath, testConfig())
	require.NoError(t, err)

	router := gin.New()
	handlers := NewHandlers(mgr)
	handlers.RegisterRoutes(router.Group("/api/v1"))

	return router, mgr, tmpDir
}

func TestDedupHandlers_NewHandlers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "dedup.json")
	mgr, err := NewManager(configPath, testConfig())
	require.NoError(t, err)

	h := NewHandlers(mgr)
	assert.NotNil(t, h)
	_ = h // 使用 h 避免未使用警告
}

func TestDedupHandlers_Scan(t *testing.T) {
	router, _, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err := os.WriteFile(file1, []byte("duplicate content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("duplicate content"), 0644)
	require.NoError(t, err)

	// 测试扫描
	body := ScanRequest{
		Paths: []string{tmpDir},
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	// 至少扫描到了我们创建的文件
	assert.GreaterOrEqual(t, int(data["filesScanned"].(float64)), 2)
}

func TestDedupHandlers_Scan_DefaultPaths(t *testing.T) {
	router, _, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 测试空路径扫描（使用默认路径）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/scan", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 应该正常响应
	assert.NotEqual(t, http.StatusInternalServerError, w.Code)
}

func TestDedupHandlers_CancelScan(t *testing.T) {
	router, _, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/scan/cancel", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestDedupHandlers_GetDuplicates(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err := os.WriteFile(file1, []byte("same content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("same content"), 0644)
	require.NoError(t, err)

	// 扫描
	_, err = mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 获取重复列表
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/duplicates", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestDedupHandlers_GetDuplicates_ForUser(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建用户目录和文件
	user1Dir := filepath.Join(tmpDir, "home", "user1")
	os.MkdirAll(user1Dir, 0755)

	file1 := filepath.Join(user1Dir, "file.txt")
	err := os.WriteFile(file1, []byte("content"), 0644)
	require.NoError(t, err)

	// 扫描
	_, err = mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 获取特定用户的重复
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/duplicates?user=user1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDedupHandlers_GetCrossUserDuplicates(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建跨用户重复
	user1Dir := filepath.Join(tmpDir, "home", "user1")
	user2Dir := filepath.Join(tmpDir, "home", "user2")
	os.MkdirAll(user1Dir, 0755)
	os.MkdirAll(user2Dir, 0755)

	file1 := filepath.Join(user1Dir, "file.txt")
	file2 := filepath.Join(user2Dir, "file.txt")
	err := os.WriteFile(file1, []byte("shared content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("shared content"), 0644)
	require.NoError(t, err)

	// 扫描
	_, err = mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 获取跨用户重复
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/duplicates/cross-user", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDedupHandlers_Deduplicate(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	content := []byte("duplicate content for dedup test")
	err := os.WriteFile(file1, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, content, 0644)
	require.NoError(t, err)

	// 扫描
	_, err = mgr.Scan([]string{tmpDir})
	require.NoError(t, err)
	duplicates, _ := mgr.GetDuplicates()
	require.Len(t, duplicates, 1)

	// 执行去重
	body := DeduplicateRequest{
		Checksum:  duplicates[0].Checksum,
		KeepPath:  file1,
		Mode:      "file",
		Action:    "softlink",
		CrossUser: true,
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/deduplicate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDedupHandlers_Deduplicate_MissingFields(t *testing.T) {
	router, _, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 缺少必需字段
	body := map[string]interface{}{
		"mode": "file",
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/deduplicate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDedupHandlers_DeduplicateAll(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建多组重复文件
	for i := 0; i < 3; i++ {
		file1 := filepath.Join(tmpDir, "group"+string(rune('A'+i))+"_1.txt")
		file2 := filepath.Join(tmpDir, "group"+string(rune('A'+i))+"_2.txt")
		content := []byte("duplicate content " + string(rune('A'+i)))
		os.WriteFile(file1, content, 0644)
		os.WriteFile(file2, content, 0644)
	}

	// 扫描
	_, err := mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 批量去重（dry run）
	body := DeduplicateAllRequest{
		Mode:      "file",
		Action:    "hardlink",
		DryRun:    true,
		CrossUser: true,
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/deduplicate/all", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestDedupHandlers_GetReport(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err := os.WriteFile(file1, []byte("duplicate content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("duplicate content"), 0644)
	require.NoError(t, err)

	// 扫描
	_, err = mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 获取报告
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/report", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Contains(t, data, "stats")
	assert.Contains(t, data, "duplicateGroups")
}

func TestDedupHandlers_GetStats(t *testing.T) {
	router, _, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestDedupHandlers_GetUserStats(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建用户目录
	user1Dir := filepath.Join(tmpDir, "home", "user1")
	os.MkdirAll(user1Dir, 0755)

	file := filepath.Join(user1Dir, "file.txt")
	err := os.WriteFile(file, []byte("content"), 0644)
	require.NoError(t, err)

	// 扫描
	_, err = mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 获取用户统计
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/stats/users?user=user1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDedupHandlers_GetUserStats_AllUsers(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建用户目录
	user1Dir := filepath.Join(tmpDir, "home", "user1")
	os.MkdirAll(user1Dir, 0755)

	file := filepath.Join(user1Dir, "file.txt")
	err := os.WriteFile(file, []byte("content"), 0644)
	require.NoError(t, err)

	// 扫描
	_, err = mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 获取所有用户统计
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/stats/users", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDedupHandlers_GetConfig(t *testing.T) {
	router, _, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestDedupHandlers_UpdateConfig(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	newConfig := testConfig()
	newConfig.ChunkSize = 8 * 1024 * 1024
	newConfig.MinFileSize = 2048
	bodyBytes, _ := json.Marshal(newConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/dedup/config", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证更新
	config := mgr.GetConfig()
	assert.Equal(t, int64(8*1024*1024), config.ChunkSize)
}

func TestDedupHandlers_GetAutoTask(t *testing.T) {
	router, _, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/auto", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestDedupHandlers_EnableAutoDedup(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	body := EnableAutoRequest{
		Enabled:  true,
		Schedule: "0 4 * * *",
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/auto/enable", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证更新
	task := mgr.GetAutoTask()
	assert.True(t, task.Enabled)
	assert.Equal(t, "0 4 * * *", task.Schedule)
}

func TestDedupHandlers_RunAutoDedup(t *testing.T) {
	router, _, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	content := []byte("auto dedup test content")
	err := os.WriteFile(file1, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, content, 0644)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/auto/run", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDedupHandlers_GetChunks(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建一些块
	_, err := mgr.CreateChunk([]byte("chunk data 1"))
	require.NoError(t, err)
	_, err = mgr.CreateChunk([]byte("chunk data 2"))
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/chunks", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDedupHandlers_GetSharedChunks(t *testing.T) {
	router, mgr, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建共享块
	data := []byte("shared chunk data")
	mgr.CreateChunkForUser(data, "user1")
	mgr.CreateChunkForUser(data, "user2")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/chunks/shared", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDedupHandlers_ChunkFile(t *testing.T) {
	router, _, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test file content for chunking"), 0644)
	require.NoError(t, err)

	body := ChunkFileRequest{
		FilePath: testFile,
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/chunks/file", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestDedupHandlers_ChunkFile_MissingPath(t *testing.T) {
	router, _, tmpDir := setupDedupTestRouter(t)
	defer os.RemoveAll(tmpDir)

	body := ChunkFileRequest{}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dedup/chunks/file", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}