package domainsync

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(manager *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	handlers := NewAPIHandlers(manager)
	handlers.RegisterRoutes(api)
	return r
}

func TestListOUs(t *testing.T) {
	manager := NewManager()
	r := setupTestRouter(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/domain-sync/ous", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 没有真实的 DC，所以会返回 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestGetConfig(t *testing.T) {
	manager := NewManager()
	r := setupTestRouter(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/domain-sync/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	assert.NotNil(t, resp["data"])
}

func TestGetConfigPasswordMasked(t *testing.T) {
	manager := NewManager()

	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"
	cfg.DCConfig.BindPassword = "super-secret-password"
	_ = manager.UpdateConfig(cfg)

	r := setupTestRouter(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/domain-sync/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.NotContains(t, body, "super-secret-password")
	assert.Contains(t, body, "******")
}

func TestUpdateConfig(t *testing.T) {
	manager := NewManager()
	r := setupTestRouter(manager)

	cfg := SyncConfig{
		DCConfig: DCConfig{
			Host:   "dc.example.com",
			Port:   636,
			Domain: "example.com",
		},
		Strategy:           "incremental",
		SyncUsers:          true,
		SyncGroups:         false,
		ConflictResolution: "merge",
	}

	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/domain-sync/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, "配置已更新", resp["message"])

	// 验证配置已更新
	loaded := manager.GetConfig()
	assert.Equal(t, "dc.example.com", loaded.DCConfig.Host)
	assert.Equal(t, SyncStrategyIncremental, loaded.Strategy)
}

func TestUpdateConfigInvalidJSON(t *testing.T) {
	manager := NewManager()
	r := setupTestRouter(manager)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/domain-sync/config", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestUpdateConfigValidation(t *testing.T) {
	manager := NewManager()
	r := setupTestRouter(manager)

	// 空 host
	cfg := SyncConfig{
		DCConfig: DCConfig{
			Domain: "example.com",
		},
	}

	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/domain-sync/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTriggerSyncNoConfig(t *testing.T) {
	manager := NewManager()
	r := setupTestRouter(manager)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domain-sync/sync", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 没有配置 DC，应该失败
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTriggerSyncInprogress(t *testing.T) {
	manager := NewManager()

	// 模拟同步正在进行
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"
	_ = manager.UpdateConfig(cfg)

	// 手动设置 engine 并标记为 running
	engine := NewSyncEngine(cfg)
	manager.mu.Lock()
	manager.engine = engine
	manager.mu.Unlock()

	// 标记 engine 为 running
	engine.mu.Lock()
	engine.running = true
	engine.mu.Unlock()

	r := setupTestRouter(manager)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domain-sync/sync", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestGetStatus(t *testing.T) {
	manager := NewManager()
	r := setupTestRouter(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/domain-sync/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "idle", data["status"])
	assert.Equal(t, "full", data["strategy"])
}

func TestGetStatusWithEngine(t *testing.T) {
	manager := NewManager()

	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"
	_ = manager.UpdateConfig(cfg)

	engine := NewSyncEngine(cfg)
	// 手动设置 lastResult
	engine.lastResult = &SyncResult{
		ID:     "test-result-id",
		Status: SyncStatusCompleted,
	}
	engine.status = SyncStatusCompleted
	manager.mu.Lock()
	manager.engine = engine
	manager.mu.Unlock()

	r := setupTestRouter(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/domain-sync/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "completed", data["status"])
	assert.NotNil(t, data["last_result"])
}

func TestSanitizeConfig(t *testing.T) {
	cfg := SyncConfig{
		DCConfig: DCConfig{
			Host:           "dc.example.com",
			Port:           636,
			Domain:         "example.com",
			BaseDN:         "DC=example,DC=com",
			BindDN:         "CN=admin,DC=example,DC=com",
			BindPassword:   "my-secret-pwd",
			UseTLS:         true,
			SkipTLSVerify:  false,
			ConnectTimeout: 10000000000,
		},
		Strategy:           "full",
		SelectedOUs:        []string{"OU=Test,DC=example,DC=com"},
		SyncUsers:          true,
		SyncGroups:         true,
		ScheduleInterval:   3600000000000,
		ConflictResolution: "merge",
		PoolSize:           5,
	}

	result := sanitizeConfig(cfg)

	dc := result["dc_config"].(map[string]interface{})
	assert.Equal(t, "dc.example.com", dc["host"])
	assert.Equal(t, 636, dc["port"])
	assert.Equal(t, "******", dc["bind_password"])
	assert.Equal(t, "CN=admin,DC=example,DC=com", dc["bind_dn"])
	assert.Equal(t, true, dc["use_tls"])
	assert.Equal(t, "10s", dc["connect_timeout"])
}

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")

	manager := NewManager()
	handlers := NewAPIHandlers(manager)
	handlers.RegisterRoutes(api)

	// 验证路由已注册
	routes := r.Routes()
	routePaths := make(map[string]bool)
	for _, route := range routes {
		routePaths[route.Path] = true
	}

	assert.True(t, routePaths["/api/v1/domain-sync/ous"])
	assert.True(t, routePaths["/api/v1/domain-sync/config"])
	assert.True(t, routePaths["/api/v1/domain-sync/sync"])
	assert.True(t, routePaths["/api/v1/domain-sync/status"])
}

func TestNewAPIHandlers(t *testing.T) {
	manager := NewManager()
	handlers := NewAPIHandlers(manager)
	assert.NotNil(t, handlers)
	assert.Equal(t, manager, handlers.manager)
}

func TestManagerStartSyncWithContext(t *testing.T) {
	manager := NewManager()

	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "192.0.2.1" // TEST-NET，不会实际连接
	cfg.DCConfig.Domain = "test.local"
	cfg.DCConfig.ConnectTimeout = 200 * time.Millisecond
	_ = manager.UpdateConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := manager.StartSync(ctx)
	assert.Error(t, err) // 连接超时
}

func TestManagerMultipleConfigs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "domainsync.json")

	m, err := NewManagerWithConfig(configPath)
	require.NoError(t, err)

	// 第一次配置
	cfg1 := DefaultSyncConfig()
	cfg1.DCConfig.Host = "dc1.example.com"
	cfg1.DCConfig.Domain = "example.com"
	err = m.UpdateConfig(cfg1)
	require.NoError(t, err)

	loaded1 := m.GetConfig()
	assert.Equal(t, "dc1.example.com", loaded1.DCConfig.Host)

	// 第二次更新配置
	cfg2 := DefaultSyncConfig()
	cfg2.DCConfig.Host = "dc2.example.com"
	cfg2.DCConfig.Domain = "example.com"
	cfg2.Strategy = SyncStrategyIncremental
	err = m.UpdateConfig(cfg2)
	require.NoError(t, err)

	loaded2 := m.GetConfig()
	assert.Equal(t, "dc2.example.com", loaded2.DCConfig.Host)
	assert.Equal(t, SyncStrategyIncremental, loaded2.Strategy)
}
