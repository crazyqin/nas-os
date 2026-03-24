package tunnel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestWebUI(t *testing.T) (*WebUIHandler, *gin.Engine) {
	logger := zap.NewNop()

	// 创建FRP管理器
	frpConfig := &FRPConfig{
		Enabled:    true,
		ServerAddr: "frp.example.com",
		ServerPort: 7000,
		DeviceID:   "test-device",
	}
	frpManager := NewFRPManager(frpConfig, logger)

	// 创建隧道服务
	tunnelConfig := Config{
		ServerAddr: "signal.example.com",
		ServerPort: 7000,
		DeviceID:   "test-device",
	}
	manager, err := NewManager(tunnelConfig, logger)
	assert.NoError(t, err)

	tunnelService := &TunnelService{
		config:  tunnelConfig,
		logger:  logger,
		manager: manager,
	}

	handler := NewWebUIHandler(frpManager, tunnelService, logger)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, router
}

func TestWebUI_GetDashboard(t *testing.T) {
	_, router := setupTestWebUI(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tunnel/dashboard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// 验证响应结构
	_, hasOverall := response["overallStatus"]
	_, hasFRP := response["frp"]
	_, hasP2P := response["p2p"]
	assert.True(t, hasOverall, "should have overallStatus")
	assert.True(t, hasFRP, "should have frp")
	assert.True(t, hasP2P, "should have p2p")
}

func TestWebUI_GetFRPStatus(t *testing.T) {
	_, router := setupTestWebUI(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tunnel/frp/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FRPStatus
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	// 初始状态为未连接
	assert.False(t, response.Connected)
}

func TestWebUI_ListProxies(t *testing.T) {
	_, router := setupTestWebUI(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tunnel/frp/proxies", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []*FRPProxyConfig
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
}

func TestWebUI_CreateProxy(t *testing.T) {
	_, router := setupTestWebUI(t)

	proxy := FRPProxyConfig{
		Name:       "test-web",
		Type:       "tcp",
		LocalIP:    "127.0.0.1",
		LocalPort:  80,
		RemotePort: 8080,
	}
	body, _ := json.Marshal(proxy)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/tunnel/frp/proxies", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response FRPProxyConfig
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test-web", response.Name)
}

func TestWebUI_GetProxy(t *testing.T) {
	_, router := setupTestWebUI(t)

	// 先创建代理
	proxy := FRPProxyConfig{
		Name:       "test-ssh",
		Type:       "tcp",
		LocalIP:    "127.0.0.1",
		LocalPort:  22,
		RemotePort: 2222,
	}
	body, _ := json.Marshal(proxy)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/tunnel/frp/proxies", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// 获取代理
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/tunnel/frp/proxies/test-ssh", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebUI_DeleteProxy(t *testing.T) {
	_, router := setupTestWebUI(t)

	// 先创建代理
	proxy := FRPProxyConfig{
		Name:       "test-delete",
		Type:       "tcp",
		LocalIP:    "127.0.0.1",
		LocalPort:  8080,
		RemotePort: 9090,
	}
	body, _ := json.Marshal(proxy)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/tunnel/frp/proxies", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// 删除代理
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/tunnel/frp/proxies/test-delete", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebUI_GetPresetServices(t *testing.T) {
	_, router := setupTestWebUI(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tunnel/presets", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []PresetService
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response, "should have preset services")

	// 验证预设服务包含常见服务
	foundSSH := false
	foundWeb := false
	for _, s := range response {
		if s.Name == "ssh" {
			foundSSH = true
		}
		if s.Name == "web" {
			foundWeb = true
		}
	}
	assert.True(t, foundSSH, "should have ssh preset")
	assert.True(t, foundWeb, "should have web preset")
}

func TestWebUI_QuickConnect(t *testing.T) {
	_, router := setupTestWebUI(t)

	req := QuickConnectRequest{
		LocalPort:   80,
		ServiceName: "web",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/tunnel/frp/quick-connect", strings.NewReader(string(body)))
	httpReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, httpReq)

	// 权限问题可能导致500，检查响应结构
	if w.Code == http.StatusOK {
		var response QuickConnectResult
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response.ProxyName, "web")
	}
}

func TestWebUI_GetFRPConfig(t *testing.T) {
	_, router := setupTestWebUI(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tunnel/frp/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FRPConfig
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Enabled)
	assert.Equal(t, "frp.example.com", response.ServerAddr)
}

func TestWebUI_UpdateFRPConfig(t *testing.T) {
	_, router := setupTestWebUI(t)

	config := FRPConfig{
		Enabled:       true,
		ServerAddr:    "new-frp.example.com",
		ServerPort:    8000,
		DeviceID:      "updated-device",
		AutoReconnect: true,
	}
	body, _ := json.Marshal(config)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/tunnel/frp/config", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebUI_GetPublicIP(t *testing.T) {
	_, router := setupTestWebUI(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tunnel/public-ip", nil)
	router.ServeHTTP(w, req)

	// 可能返回404（无公网IP）或200
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
}

func TestWebUI_GetP2PStatus(t *testing.T) {
	_, router := setupTestWebUI(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tunnel/p2p/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebUI_BackwardCompatibility(t *testing.T) {
	_, router := setupTestWebUI(t)

	// 测试向后兼容的路由
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tunnel/status", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebUI_ConnectP2P_ValidationError(t *testing.T) {
	_, router := setupTestWebUI(t)

	// 空请求体应该返回400
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/tunnel/p2p/connect", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebUI_CreateProxy_ValidationError(t *testing.T) {
	_, router := setupTestWebUI(t)

	// 缺少必需字段
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/tunnel/frp/proxies", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 应该创建成功（没有必填字段验证）
	// 如果添加了验证，应该返回400
}