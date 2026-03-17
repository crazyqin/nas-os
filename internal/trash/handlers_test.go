package trash

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHandlersTestEnv(t *testing.T) (*Manager, string) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	trashRoot := filepath.Join(tmpDir, "trash")

	config := &Config{
		Enabled:       true,
		RetentionDays: 30,
		MaxSize:       1024 * 1024 * 100, // 100MB
		AutoEmpty:     false,
	}

	mgr, err := NewManager(configPath, trashRoot, config)
	require.NoError(t, err)

	return mgr, tmpDir
}

func createTestContext(t *testing.T, method, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	c.Request = req

	return c, w
}

func TestHandlers_NewHandlers(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)
	assert.NotNil(t, h)
	assert.Equal(t, mgr, h.manager)
}

func TestHandlers_List(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建测试文件并移动到回收站
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	_, err := mgr.MoveToTrash(testFile, "user1")
	require.NoError(t, err)

	c, w := createTestContext(t, "GET", "/trash", nil)

	h.list(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	// JSON 解析后是 []interface{}
	data, ok := resp.Data.([]interface{})
	require.True(t, ok)
	assert.Len(t, data, 1)

	// 检查第一个元素
	item, ok := data[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test.txt", item["name"])
}

func TestHandlers_List_Empty(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	c, w := createTestContext(t, "GET", "/trash", nil)

	h.list(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// 空列表可能是 nil 或 []interface{}
	if resp.Data != nil {
		data, ok := resp.Data.([]interface{})
		require.True(t, ok)
		assert.Empty(t, data)
	}
}

func TestHandlers_GetStats(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content for stats")
	require.NoError(t, os.WriteFile(testFile, content, 0644))

	_, err := mgr.MoveToTrash(testFile, "user1")
	require.NoError(t, err)

	c, w := createTestContext(t, "GET", "/trash/stats", nil)

	h.getStats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(1), data["total_items"])
}

func TestHandlers_GetConfig(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	c, w := createTestContext(t, "GET", "/trash/config", nil)

	h.getConfig(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// JSON 解析后是 map[string]interface{}
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.True(t, data["enabled"].(bool))
	assert.Equal(t, float64(30), data["retention_days"])
}

func TestHandlers_UpdateConfig(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	newRetention := 15
	enabled := false
	reqBody := UpdateConfigRequest{
		RetentionDays: &newRetention,
		Enabled:       &enabled,
	}

	body, _ := json.Marshal(reqBody)
	c, w := createTestContext(t, "PUT", "/trash/config", body)

	h.updateConfig(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证配置更新
	updated := mgr.GetConfig()
	assert.Equal(t, 15, updated.RetentionDays)
	assert.False(t, updated.Enabled)
}

func TestHandlers_UpdateConfig_InvalidJSON(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	c, w := createTestContext(t, "PUT", "/trash/config", []byte("invalid json"))

	h.updateConfig(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_Restore(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建测试文件并移动到回收站
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	item, err := mgr.MoveToTrash(testFile, "user1")
	require.NoError(t, err)

	c, w := createTestContext(t, "POST", "/trash/"+item.ID+"/restore", nil)
	c.Params = gin.Params{{Key: "id", Value: item.ID}}

	h.restore(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证文件已恢复
	_, err = os.Stat(testFile)
	require.NoError(t, err)
}

func TestHandlers_Restore_NotFound(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	c, w := createTestContext(t, "POST", "/trash/nonexistent/restore", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	h.restore(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_RestoreToTarget(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建测试文件并移动到回收站
	originalPath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(originalPath, []byte("test content"), 0644))

	item, err := mgr.MoveToTrash(originalPath, "user1")
	require.NoError(t, err)

	// 恢复到新路径
	newPath := filepath.Join(tmpDir, "restored", "new.txt")
	reqBody := RestoreRequest{
		TargetPath: newPath,
	}

	body, _ := json.Marshal(reqBody)
	c, w := createTestContext(t, "POST", "/trash/"+item.ID+"/restore", body)
	c.Params = gin.Params{{Key: "id", Value: item.ID}}

	h.restore(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证文件已恢复到新位置
	_, err = os.Stat(newPath)
	require.NoError(t, err)
}

func TestHandlers_DeletePermanently(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建测试文件并移动到回收站
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	item, err := mgr.MoveToTrash(testFile, "user1")
	require.NoError(t, err)

	c, w := createTestContext(t, "DELETE", "/trash/"+item.ID, nil)
	c.Params = gin.Params{{Key: "id", Value: item.ID}}

	h.deletePermanently(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证回收站为空
	items := mgr.List()
	assert.Empty(t, items)
}

func TestHandlers_DeletePermanently_NotFound(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	c, w := createTestContext(t, "DELETE", "/trash/nonexistent", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	h.deletePermanently(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_Empty(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建多个测试文件并移动到回收站
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tmpDir, "test"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

		_, err := mgr.MoveToTrash(testFile, "user1")
		require.NoError(t, err)
	}

	// 验证有 3 个项目
	items := mgr.List()
	require.Len(t, items, 3)

	c, w := createTestContext(t, "DELETE", "/trash", nil)

	h.empty(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证回收站为空
	items = mgr.List()
	assert.Empty(t, items)
}

func TestHandlers_MoveToTrash(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	reqBody := MoveToTrashRequest{
		Path:   testFile,
		UserID: "user1",
	}

	body, _ := json.Marshal(reqBody)
	c, w := createTestContext(t, "POST", "/trash/move", body)

	h.MoveToTrash(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证原文件已不存在
	_, err := os.Stat(testFile)
	require.True(t, os.IsNotExist(err))

	// 验证回收站有项目
	items := mgr.List()
	require.Len(t, items, 1)
}

func TestHandlers_MoveToTrash_InvalidRequest(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	c, w := createTestContext(t, "POST", "/trash/move", []byte("{}"))

	h.MoveToTrash(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_MoveToTrash_FileNotFound(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	reqBody := MoveToTrashRequest{
		Path:   "/nonexistent/file.txt",
		UserID: "user1",
	}

	body, _ := json.Marshal(reqBody)
	c, w := createTestContext(t, "POST", "/trash/move", body)

	h.MoveToTrash(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_RegisterRoutes(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	router := gin.New()
	apiGroup := router.Group("/api")
	h.RegisterRoutes(apiGroup)

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		routeMap[route.Method+":"+route.Path] = true
	}

	// 验证路由已注册
	assert.True(t, routeMap["GET:/api/trash"])
	assert.True(t, routeMap["GET:/api/trash/stats"])
	assert.True(t, routeMap["GET:/api/trash/config"])
	assert.True(t, routeMap["PUT:/api/trash/config"])
	assert.True(t, routeMap["DELETE:/api/trash"])
	assert.True(t, routeMap["POST:/api/trash/:id/restore"])
	assert.True(t, routeMap["DELETE:/api/trash/:id"])
}

func TestTrashResponse_DaysLeft(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	_, err := mgr.MoveToTrash(testFile, "user1")
	require.NoError(t, err)

	c, w := createTestContext(t, "GET", "/trash", nil)

	h.list(c)

	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// JSON 解析后是 []interface{}
	data, ok := resp.Data.([]interface{})
	require.True(t, ok)

	item, ok := data[0].(map[string]interface{})
	require.True(t, ok)

	// 验证 DaysLeft 在合理范围内
	daysLeft := int(item["days_left"].(float64))
	assert.GreaterOrEqual(t, daysLeft, 29)
	assert.LessOrEqual(t, daysLeft, 31)
}

func TestAPIResponse_Success(t *testing.T) {
	resp := api.Success("test data")
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "success", resp.Message)
	assert.Equal(t, "test data", resp.Data)
}

func TestAPIResponse_Error(t *testing.T) {
	resp := api.Error(400, "bad request")
	assert.Equal(t, 400, resp.Code)
	assert.Equal(t, "bad request", resp.Message)
}

func TestHandlers_DaysLeft_Expired(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	item, err := mgr.MoveToTrash(testFile, "user1")
	require.NoError(t, err)

	// 手动设置过期时间为过去
	mgr.mu.Lock()
	mgr.items[item.ID].ExpiresAt = time.Now().Add(-24 * time.Hour)
	mgr.mu.Unlock()

	c, w := createTestContext(t, "GET", "/trash", nil)

	h.list(c)

	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// JSON 解析后是 []interface{}
	data, ok := resp.Data.([]interface{})
	require.True(t, ok)

	respItem, ok := data[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(0), respItem["days_left"])
}

func TestHandlers_UpdateConfig_AllFields(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	enabled := false
	retention := 60
	maxSize := int64(1024 * 1024 * 1024) // 1GB
	autoEmpty := false

	reqBody := UpdateConfigRequest{
		Enabled:       &enabled,
		RetentionDays: &retention,
		MaxSize:       &maxSize,
		AutoEmpty:     &autoEmpty,
	}

	body, _ := json.Marshal(reqBody)
	c, w := createTestContext(t, "PUT", "/trash/config", body)

	h.updateConfig(c)

	assert.Equal(t, http.StatusOK, w.Code)

	config := mgr.GetConfig()
	assert.False(t, config.Enabled)
	assert.Equal(t, 60, config.RetentionDays)
	assert.Equal(t, int64(1024*1024*1024), config.MaxSize)
	assert.False(t, config.AutoEmpty)
}

// ========== v2.13.0 补充测试 ==========

func TestHandlers_Get(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建测试文件并移动到回收站
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	item, err := mgr.MoveToTrash(testFile, "user1")
	require.NoError(t, err)

	c, w := createTestContext(t, "GET", "/trash/"+item.ID, nil)
	c.Params = gin.Params{{Key: "id", Value: item.ID}}

	h.get(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test.txt", data["name"])
}

func TestHandlers_Get_NotFound(t *testing.T) {
	mgr, _ := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	c, w := createTestContext(t, "GET", "/trash/nonexistent", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	h.get(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_Empty_WithItems(t *testing.T) {
	mgr, tmpDir := setupHandlersTestEnv(t)
	defer mgr.Empty()

	h := NewHandlers(mgr)

	// 创建多个测试文件
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(tmpDir, "test"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

		_, err := mgr.MoveToTrash(testFile, "user1")
		require.NoError(t, err)
	}

	// 验证有项目
	items := mgr.List()
	require.Len(t, items, 5)

	c, w := createTestContext(t, "DELETE", "/trash", nil)

	h.empty(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证回收站为空
	items = mgr.List()
	assert.Empty(t, items)
}
