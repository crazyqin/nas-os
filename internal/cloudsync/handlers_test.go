package cloudsync

import (
	"context"
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

func setupCloudSyncTestRouter(t *testing.T) (*gin.Engine, *Manager, string) {
	gin.SetMode(gin.TestMode)

	tmpDir, err := os.MkdirTemp("", "cloudsync-handler-test")
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	err = m.Initialize()
	require.NoError(t, err)

	router := gin.New()
	handlers := NewHandlers(m)
	handlers.RegisterRoutes(router.Group("/api/v1"))

	return router, m, tmpDir
}

func TestCloudSyncHandlers_NewHandlers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cloudsync-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "cloudsync.json")
	m := NewManager(configPath)

	h := NewHandlers(m)
	assert.NotNil(t, h)
}

func TestCloudSyncHandlers_CreateProvider(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 使用 WebDAV 类型测试，因为它不需要敏感字段（SecretKey 有 json:"-" 标签）
	body := map[string]interface{}{
		"name":     "test-provider",
		"type":     "webdav",
		"endpoint": "https://webdav.example.com",
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("序列化请求失败: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/cloudsync/providers", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	// 验证提供商已创建
	providers := m.ListProviders()
	assert.Len(t, providers, 1)
}

func TestCloudSyncHandlers_CreateProvider_MissingFields(t *testing.T) {
	router, _, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	body := map[string]interface{}{
		"name": "test-provider",
		// 缺少必需字段
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("序列化请求失败: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/cloudsync/providers", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 应该返回错误
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestCloudSyncHandlers_GetProvider(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 先创建提供商
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	// 测试获取提供商
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/cloudsync/providers/"+provider.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestCloudSyncHandlers_GetProvider_NotFound(t *testing.T) {
	router, _, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/cloudsync/providers/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestCloudSyncHandlers_ListProviders(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建多个提供商
	for i := 0; i < 3; i++ {
		_, err := m.CreateProvider(ProviderConfig{
			Name:      "provider-" + string(rune('A'+i)),
			Type:      ProviderAWSS3,
			AccessKey: "key",
			SecretKey: "secret",
			Bucket:    "bucket",
		})
		require.NoError(t, err)
	}

	// 测试列表
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/cloudsync/providers", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].([]interface{})
	assert.Len(t, data, 3)
}

func TestCloudSyncHandlers_UpdateProvider(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 先创建提供商（使用 WebDAV，不需要敏感字段）
	provider, err := m.CreateProvider(ProviderConfig{
		Name:     "test-provider",
		Type:     ProviderWebDAV,
		Endpoint: "https://webdav.example.com",
	})
	require.NoError(t, err)

	// 更新提供商
	body := ProviderConfig{
		Name:     "updated-provider",
		Type:     ProviderWebDAV,
		Endpoint: "https://updated.webdav.example.com",
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("序列化请求失败: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/cloudsync/providers/"+provider.ID, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证更新
	updated, err := m.GetProvider(provider.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated-provider", updated.Name)
}

func TestCloudSyncHandlers_DeleteProvider(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 先创建提供商
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	// 删除提供商
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/cloudsync/providers/"+provider.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证已删除
	_, err = m.GetProvider(provider.ID)
	assert.Error(t, err)
}

func TestCloudSyncHandlers_TestProvider(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建提供商
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	// 测试连接
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/cloudsync/providers/"+provider.ID+"/test", nil)
	router.ServeHTTP(w, req)

	// 注意：由于使用假的凭据，测试可能失败，但 API 应该正常响应
	assert.NotEqual(t, http.StatusInternalServerError, w.Code)
}

func TestCloudSyncHandlers_CreateSyncTask(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 先创建提供商
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	// 创建同步任务
	body := map[string]interface{}{
		"name":         "test-sync",
		"providerId":   provider.ID,
		"localPath":    "/tmp/test",
		"remotePath":   "/backup",
		"direction":    "bidirect",
		"scheduleType": "manual",
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("序列化请求失败: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/cloudsync/tasks", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestCloudSyncHandlers_GetSyncTask(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建提供商和任务
	provider, _ := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})

	task, err := m.CreateSyncTask(SyncTask{
		Name:       "test-sync",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/test",
		RemotePath: "/backup",
	})
	require.NoError(t, err)

	// 获取任务
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/cloudsync/tasks/"+task.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCloudSyncHandlers_ListSyncTasks(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建提供商和多个任务
	provider, _ := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})

	for i := 0; i < 3; i++ {
		_, err := m.CreateSyncTask(SyncTask{
			Name:       "sync-" + string(rune('A'+i)),
			ProviderID: provider.ID,
			LocalPath:  "/tmp/test",
			RemotePath: "/backup",
		})
		require.NoError(t, err)
	}

	// 列出任务
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/cloudsync/tasks", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	data := resp["data"].([]interface{})
	assert.Len(t, data, 3)
}

func TestCloudSyncHandlers_DeleteSyncTask(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建提供商和任务
	provider, _ := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})

	task, err := m.CreateSyncTask(SyncTask{
		Name:       "test-sync",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/test",
		RemotePath: "/backup",
	})
	require.NoError(t, err)

	// 删除任务
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/cloudsync/tasks/"+task.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证已删除
	_, err = m.GetSyncTask(task.ID)
	assert.Error(t, err)
}

func TestCloudSyncHandlers_GetSyncStatus(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建提供商和任务
	provider, _ := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})

	task, err := m.CreateSyncTask(SyncTask{
		Name:       "test-sync",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/test",
		RemotePath: "/backup",
	})
	require.NoError(t, err)

	// 获取状态
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/cloudsync/tasks/"+task.ID+"/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCloudSyncHandlers_GetStats(t *testing.T) {
	router, _, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/cloudsync/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestCloudSyncHandlers_GetProvidersInfo(t *testing.T) {
	router, _, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/cloudsync/providers-info", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	// 验证返回提供商信息列表
	data, ok := resp["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	assert.Greater(t, len(data), 0)
}

func TestCloudSyncHandlers_UpdateSyncTask(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建提供商和任务
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	task, err := m.CreateSyncTask(SyncTask{
		Name:       "test-sync",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/test",
		RemotePath: "/backup",
	})
	require.NoError(t, err)

	// 更新任务
	body := map[string]interface{}{
		"name":       "updated-sync",
		"providerId": provider.ID,
		"localPath":  "/tmp/updated",
		"remotePath": "/backup/updated",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/cloudsync/tasks/"+task.ID, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	// 验证更新
	updated, err := m.GetSyncTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated-sync", updated.Name)
}

func TestCloudSyncHandlers_UpdateSyncTask_NotFound(t *testing.T) {
	router, _, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	body := map[string]interface{}{
		"name":       "updated-sync",
		"localPath":  "/tmp/updated",
		"remotePath": "/backup/updated",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/cloudsync/tasks/nonexistent", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestCloudSyncHandlers_GetAllStatuses(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建多个任务
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := m.CreateSyncTask(SyncTask{
			Name:       "sync-" + string(rune('A'+i)),
			ProviderID: provider.ID,
			LocalPath:  "/tmp/test",
			RemotePath: "/backup",
		})
		require.NoError(t, err)
	}

	// 获取所有状态
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/cloudsync/statuses", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestCloudSyncHandlers_RunSyncTask(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建提供商和任务
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	task, err := m.CreateSyncTask(SyncTask{
		Name:       "test-sync",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/test",
		RemotePath: "/backup",
	})
	require.NoError(t, err)

	// 运行同步任务
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/cloudsync/tasks/"+task.ID+"/run", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestCloudSyncHandlers_RunSyncTask_NotFound(t *testing.T) {
	router, _, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/cloudsync/tasks/nonexistent/run", nil)
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestCloudSyncHandlers_PauseSyncTask_NotRunning(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建提供商和任务
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	task, err := m.CreateSyncTask(SyncTask{
		Name:       "test-sync",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/test",
		RemotePath: "/backup",
	})
	require.NoError(t, err)

	// 暂停未运行的任务应该返回错误
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/cloudsync/tasks/"+task.ID+"/pause", nil)
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestCloudSyncHandlers_ResumeSyncTask_NotRunning(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建提供商和任务
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	task, err := m.CreateSyncTask(SyncTask{
		Name:       "test-sync",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/test",
		RemotePath: "/backup",
	})
	require.NoError(t, err)

	// 恢复未运行的任务应该返回错误
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/cloudsync/tasks/"+task.ID+"/resume", nil)
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestCloudSyncHandlers_CancelSyncTask_NotRunning(t *testing.T) {
	router, m, tmpDir := setupCloudSyncTestRouter(t)
	defer os.RemoveAll(tmpDir)

	// 创建提供商和任务
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	task, err := m.CreateSyncTask(SyncTask{
		Name:       "test-sync",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/test",
		RemotePath: "/backup",
	})
	require.NoError(t, err)

	// 取消未运行的任务应该返回错误
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/cloudsync/tasks/"+task.ID+"/cancel", nil)
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}
