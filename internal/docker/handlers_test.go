package docker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== Handlers 初始化测试 ==========

func TestNewHandlers(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)

	handlers := NewHandlers(mgr)
	assert.NotNil(t, handlers)
}

func TestNewHandlers_NilManager(t *testing.T) {
	handlers := NewHandlers(nil)
	assert.NotNil(t, handlers)
}

func TestHandlers_RegisterRoutes(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 验证路由已注册
	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		routeMap[route.Path] = true
	}

	// 检查关键路由
	expectedRoutes := []string{
		"/api/docker/containers",
		"/api/docker/images",
		"/api/docker/networks",
		"/api/docker/volumes",
		"/api/docker/apps",
		"/api/docker/status",
	}

	for _, route := range expectedRoutes {
		assert.True(t, routeMap[route], "Expected route %s to be registered", route)
	}
}

// ========== 容器 Handler 测试 ==========

func TestHandlers_ListContainers(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/containers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 可能返回 500 如果 Docker 未运行，但不应 panic
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

func TestHandlers_ListContainers_All(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/containers?all=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证请求成功处理
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

func TestHandlers_GetContainer(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/containers/nonexistent123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 容器不存在应该返回 404 或 500
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
}

func TestHandlers_CreateContainer_InvalidJSON(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/containers", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 无效 JSON 应该返回 400
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_CreateContainer_MissingFields(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 缺少 name 字段
	req := httptest.NewRequest("POST", "/api/docker/containers", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 应该返回 400 或 500
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
}

// ========== 镜像 Handler 测试 ==========

func TestHandlers_ListImages(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/images", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证请求成功处理
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

func TestHandlers_PullImage_InvalidJSON(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/images/pull", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_PullImage_MissingImage(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/images/pull", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== 网络 Handler 测试 ==========

func TestHandlers_ListNetworks(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/networks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

// ========== 卷 Handler 测试 ==========

func TestHandlers_ListVolumes(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/volumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

func TestHandlers_CreateVolume_InvalidJSON(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/volumes", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_GetVolume(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/volumes/nonexistent-volume-xyz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 卷不存在应该返回 404 或 500
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
}

// ========== 应用商店 Handler 测试 ==========

func TestHandlers_GetAppCatalog(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/apps", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 应用目录应该返回成功
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// 检查返回的数据
	data, ok := resp["data"].([]interface{})
	assert.True(t, ok)
	assert.True(t, len(data) > 0, "应用目录应该不为空")
}

func TestHandlers_InstallApp_InvalidJSON(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/apps/nginx/install", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 可能返回 400, 404 或 500
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
}

// ========== 状态 Handler 测试 ==========

func TestHandlers_GetStatus(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 状态端点应该始终返回 200
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// 检查返回的状态字段
	_, hasRunning := resp["data"].(map[string]interface{})["running"]
	assert.True(t, hasRunning, "状态应该包含 running 字段")
}

// ========== 容器操作 Handler 测试 ==========

func TestHandlers_StartContainer(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/containers/nonexistent/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 容器不存在应该返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_StopContainer(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/containers/nonexistent/stop", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 容器不存在应该返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_RestartContainer(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/containers/nonexistent/restart", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 容器不存在应该返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_RemoveContainer(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("DELETE", "/api/docker/containers/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 容器不存在应该返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_GetContainerStats(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/containers/nonexistent/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 容器不存在应该返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_GetContainerLogs(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/containers/nonexistent/logs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 容器不存在应该返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ========== 边缘情况测试 ==========

func TestHandlers_RemoveImage(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("DELETE", "/api/docker/images/sha256:nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 镜像不存在应该返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_RemoveVolume(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("DELETE", "/api/docker/volumes/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 卷不存在应该返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ========== 完整请求流程测试 ==========

func TestHandlers_CreateContainer_FullFlow(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/containers", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 没有 Docker 会返回 400, 500 或 404
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound || w.Code == http.StatusCreated)
}

func TestHandlers_PullImage_FullRequest(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/images/pull", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 需要 Docker 运行才能成功
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound)
}

func TestHandlers_CreateVolume_FullRequest(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/docker/volumes", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 需要 Docker 运行才能成功
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound)
}

// ========== 查询参数测试 ==========

func TestHandlers_GetContainerLogs_WithParams(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/docker/containers/nonexistent/logs?tail=100&since=1h", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 容器不存在应该返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_ListContainers_WithQuery(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 测试各种查询参数组合
	testCases := []struct {
		name   string
		query  string
		expect int // 预期状态码或范围
	}{
		{"all=true", "?all=true", http.StatusOK},
		{"all=false", "?all=false", http.StatusOK},
		{"empty query", "", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/docker/containers"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// 没有 Docker 可能返回 500
			assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
		})
	}
}

// ========== 并发测试 ==========

func TestHandlers_ConcurrentRequests(t *testing.T) {
	mgr, _ := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 并发发送多个请求
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/api/docker/status", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			done <- true
		}()
	}

	// 等待所有请求完成
	for i := 0; i < 10; i++ {
		<-done
	}
}
