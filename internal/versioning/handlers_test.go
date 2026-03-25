package versioning

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

func setupVersioningTestEnv(t *testing.T) (*Manager, string) {
	gin.SetMode(gin.TestMode)

	tmpDir, err := os.MkdirTemp("", "versioning-handler-test")
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)

	return mgr, tmpDir
}

func createTestContext(t *testing.T, method, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var req *http.Request
	if body != nil {
		req = httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequestWithContext(context.Background(), method, path, nil)
	}
	c.Request = req

	return c, w
}

func TestHandlers_NewHandlers(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)
	assert.NotNil(t, h)
}

func TestHandlers_ListFileVersions(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// 创建版本
	_, err = mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 测试获取版本列表
	c, w := createTestContext(t, "GET", "/files"+testFile+"?versions=true", nil)
	c.Params = gin.Params{{Key: "path", Value: testFile}}
	// 设置查询参数
	c.Request.URL.RawQuery = "versions=true"

	h.listFileVersions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandlers_GetVersion(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)

	// 创建测试文件和版本
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	version, err := mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 测试获取版本
	c, w := createTestContext(t, "GET", "/versions/"+version.ID, nil)
	c.Params = gin.Params{{Key: "id", Value: version.ID}}

	h.getVersion(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetVersion_NotFound(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)

	c, w := createTestContext(t, "GET", "/versions/nonexistent", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	h.getVersion(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_CreateVersion(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// 测试创建版本
	body := CreateVersionRequest{
		UserID:      "user1",
		Description: "test version",
		TriggerType: "manual",
	}
	bodyBytes, _ := json.Marshal(body)

	c, w := createTestContext(t, "POST", "/files"+testFile+"/versions", bodyBytes)
	c.Params = gin.Params{{Key: "path", Value: testFile + "/versions"}}

	h.createVersion(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_RestoreVersion(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)

	// 创建测试文件和版本
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("original content"), 0644)
	require.NoError(t, err)

	version, err := mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 修改文件
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	// 测试恢复版本
	body := RestoreVersionRequest{TargetPath: ""}
	bodyBytes, _ := json.Marshal(body)

	c, w := createTestContext(t, "POST", "/versions/"+version.ID+"/restore", bodyBytes)
	c.Params = gin.Params{{Key: "id", Value: version.ID}}

	h.restoreVersion(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证文件已恢复
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "original content", string(content))
}

func TestHandlers_DeleteVersion(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)

	// 创建测试文件和版本
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	version, err := mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 测试删除版本
	c, w := createTestContext(t, "DELETE", "/versions/"+version.ID, nil)
	c.Params = gin.Params{{Key: "id", Value: version.ID}}

	h.deleteVersion(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证版本已删除
	_, err = mgr.GetVersion(version.ID)
	assert.Error(t, err)
}

func TestHandlers_GetVersionDiff(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)

	// 创建测试文件和版本
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0644)
	require.NoError(t, err)

	version, err := mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 修改文件
	err = os.WriteFile(testFile, []byte("line1\nmodified\nline3\n"), 0644)
	require.NoError(t, err)

	// 测试获取差异
	c, w := createTestContext(t, "GET", "/versions/"+version.ID+"/diff", nil)
	c.Params = gin.Params{{Key: "id", Value: version.ID}}

	h.getVersionDiff(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetStats(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)

	// 创建测试文件和版本
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	_, err = mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 测试获取统计
	c, w := createTestContext(t, "GET", "/versions/stats", nil)

	h.getStats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, true, data["enabled"])
}

func TestHandlers_GetConfig(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)

	c, w := createTestContext(t, "GET", "/versions/config", nil)

	h.getConfig(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandlers_UpdateConfig(t *testing.T) {
	mgr, tmpDir := setupVersioningTestEnv(t)
	defer os.RemoveAll(tmpDir)
	defer mgr.Close()

	h := NewHandlers(mgr)

	newConfig := DefaultConfig()
	newConfig.MaxFileSize = 1024 * 1024 * 1024 // 1GB
	bodyBytes, _ := json.Marshal(newConfig)

	c, w := createTestContext(t, "PUT", "/versions/config", bodyBytes)

	h.updateConfig(c)

	assert.Equal(t, http.StatusOK, w.Code)
}
