package replication

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

func setupTestHandlers(t *testing.T) (*Handlers, *gin.Engine, string) {
	gin.SetMode(gin.TestMode)

	tmpDir, err := os.MkdirTemp("", "handler-test")
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "replication.json")
	mgr, err := NewManager(configPath, nil)
	require.NoError(t, err)

	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	return handlers, router, tmpDir
}

func TestHandlers_CreateTask(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	// 创建源目录
	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	reqBody := CreateTaskRequest{
		Name:       "test-task",
		SourcePath: sourceDir,
		TargetPath: "/tmp/target",
		Type:       "scheduled",
		Schedule:   "hourly",
		Enabled:    true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)

	var task map[string]interface{}
	dataBytes, _ := json.Marshal(resp.Data)
	require.NoError(t, json.Unmarshal(dataBytes, &task))
	assert.Equal(t, "test-task", task["name"])
}

func TestHandlers_ListTasks(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	// 创建源目录
	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// 先创建一个任务
	reqBody := CreateTaskRequest{
		Name:       "test-task",
		SourcePath: sourceDir,
		TargetPath: "/tmp/target",
		Type:       "scheduled",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 列出任务
	req = httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/replications", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)

	var tasks []map[string]interface{}
	dataBytes, _ := json.Marshal(resp.Data)
	require.NoError(t, json.Unmarshal(dataBytes, &tasks))
	assert.Len(t, tasks, 1)
}

func TestHandlers_GetTask(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	// 创建源目录
	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// 先创建一个任务
	reqBody := CreateTaskRequest{
		Name:       "test-task",
		SourcePath: sourceDir,
		TargetPath: "/tmp/target",
		Type:       "scheduled",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var createResp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	var task map[string]interface{}
	dataBytes, _ := json.Marshal(createResp.Data)
	require.NoError(t, json.Unmarshal(dataBytes, &task))
	taskID := task["id"].(string)

	// 获取任务
	req = httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/replications/"+taskID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)
}

func TestHandlers_GetTask_NotFound(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/replications/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_UpdateTask(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	// 创建源目录
	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// 先创建一个任务
	reqBody := CreateTaskRequest{
		Name:       "test-task",
		SourcePath: sourceDir,
		TargetPath: "/tmp/target",
		Type:       "scheduled",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var createResp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	var task map[string]interface{}
	dataBytes, _ := json.Marshal(createResp.Data)
	require.NoError(t, json.Unmarshal(dataBytes, &task))
	taskID := task["id"].(string)

	// 更新任务
	name := "updated-task"
	enabled := false
	updateReq := UpdateTaskRequest{
		Name:    &name,
		Enabled: &enabled,
	}
	body, _ = json.Marshal(updateReq)
	req = httptest.NewRequestWithContext(context.Background(), "PUT", "/api/v1/replications/"+taskID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_DeleteTask(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	// 创建源目录
	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// 先创建一个任务
	reqBody := CreateTaskRequest{
		Name:       "test-task",
		SourcePath: sourceDir,
		TargetPath: "/tmp/target",
		Type:       "scheduled",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var createResp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	var task map[string]interface{}
	dataBytes, _ := json.Marshal(createResp.Data)
	require.NoError(t, json.Unmarshal(dataBytes, &task))
	taskID := task["id"].(string)

	// 删除任务
	req = httptest.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/replications/"+taskID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证删除
	req = httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/replications/"+taskID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_PauseTask(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	// 创建源目录
	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// 先创建一个任务
	reqBody := CreateTaskRequest{
		Name:       "test-task",
		SourcePath: sourceDir,
		TargetPath: "/tmp/target",
		Type:       "scheduled",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var createResp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	var task map[string]interface{}
	dataBytes, _ := json.Marshal(createResp.Data)
	require.NoError(t, json.Unmarshal(dataBytes, &task))
	taskID := task["id"].(string)

	// 暂停任务
	req = httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications/"+taskID+"/pause", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_ResumeTask(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	// 创建源目录
	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// 先创建一个任务
	reqBody := CreateTaskRequest{
		Name:       "test-task",
		SourcePath: sourceDir,
		TargetPath: "/tmp/target",
		Type:       "scheduled",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var createResp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	var task map[string]interface{}
	dataBytes, _ := json.Marshal(createResp.Data)
	require.NoError(t, json.Unmarshal(dataBytes, &task))
	taskID := task["id"].(string)

	// 恢复任务
	req = httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications/"+taskID+"/resume", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetStats(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/replications/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)

	var stats map[string]interface{}
	dataBytes, _ := json.Marshal(resp.Data)
	require.NoError(t, json.Unmarshal(dataBytes, &stats))
	assert.Contains(t, stats, "total_tasks")
}

func TestHandlers_CreateTask_InvalidRequest(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	// 缺少必填字段
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_SyncTask(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	// 创建源目录
	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// 先创建一个任务
	reqBody := CreateTaskRequest{
		Name:       "test-task",
		SourcePath: sourceDir,
		TargetPath: "/tmp/target",
		Type:       "realtime",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var createResp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	var task map[string]interface{}
	dataBytes, _ := json.Marshal(createResp.Data)
	require.NoError(t, json.Unmarshal(dataBytes, &task))
	taskID := task["id"].(string)

	// 同步任务
	req = httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications/"+taskID+"/sync", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_ListConflicts(t *testing.T) {
	mgr, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/replications/conflicts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	_ = mgr
}

func TestHandlers_ResolveConflict_InvalidJSON(t *testing.T) {
	_, router, tmpDir := setupTestHandlers(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/replications/conflicts/nonexistent/resolve", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
