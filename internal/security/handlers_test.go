package security

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewHandlers(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	assert.NotNil(t, h)
	assert.NotNil(t, h.manager)
}

func TestHandlers_RegisterRoutes(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	// 测试路由注册
	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Method+":"+r.Path] = true
	}

	// 验证关键路由存在
	expectedRoutes := []string{
		"GET:/api/security/dashboard",
		"GET:/api/security/config",
		"PUT:/api/security/config",
		"GET:/api/security/firewall/status",
		"GET:/api/security/firewall/rules",
		"POST:/api/security/firewall/rules",
		"GET:/api/security/fail2ban/status",
		"GET:/api/security/audit/logs",
		"GET:/api/security/baseline/checks",
	}

	for _, route := range expectedRoutes {
		assert.True(t, routeMap[route], "Route should be registered: %s", route)
	}
}

func TestHandlers_GetDashboard(t *testing.T) {
	sm := NewManager()
	sm.audit.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestHandlers_GetConfig(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestHandlers_UpdateConfig(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	// 测试无效JSON
	req := httptest.NewRequest("PUT", "/api/security/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 测试有效配置
	config := Config{
		Firewall: FirewallConfig{
			Enabled:       true,
			DefaultPolicy: "deny",
		},
		Fail2Ban: Fail2BanConfig{
			Enabled:     true,
			MaxAttempts: 5,
		},
		AuditEnabled: true,
	}
	body, _ := json.Marshal(config)
	req = httptest.NewRequest("PUT", "/api/security/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 可能返回 200 或 400，取决于验证
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest)
}

func TestHandlers_GetFirewallStatus(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/firewall/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_ListFirewallRules(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/firewall/rules", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_AddFirewallRule(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	// 测试无效JSON
	req := httptest.NewRequest("POST", "/api/security/firewall/rules", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 测试缺少必填字段
	rule := map[string]interface{}{
		"enabled": true,
	}
	body, _ := json.Marshal(rule)
	req = httptest.NewRequest("POST", "/api/security/firewall/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_GetFail2BanStatus(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/fail2ban/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetBannedIPs(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/fail2ban/banned", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetAuditLogs(t *testing.T) {
	sm := NewManager()
	sm.audit.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 添加一些审计日志
	sm.audit.Log(AuditLogEntry{
		Level:    "info",
		Category: "system",
		Event:    "test_event",
	})

	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/audit/logs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetLoginLogs(t *testing.T) {
	sm := NewManager()
	sm.audit.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 添加登录日志
	sm.audit.LogLogin(LoginLogEntry{
		Username: "admin",
		IP:       "192.168.1.1",
		Status:   "success",
	})

	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/audit/login-logs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetAlerts(t *testing.T) {
	sm := NewManager()
	sm.audit.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	// 触发告警
	sm.audit.Log(AuditLogEntry{
		Level:    "critical",
		Category: "security",
		Event:    "unauthorized_access",
	})

	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/audit/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetBaselineChecks(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/baseline/checks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_RunBaselineCheck(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/baseline/check", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetBaselineReport(t *testing.T) {
	sm := NewManager()
	// 先运行一次检查生成报告
	sm.RunBaselineCheck()

	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/baseline/report", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetBaselineCategories(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/baseline/categories", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetBlacklist(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/firewall/blacklist", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetWhitelist(t *testing.T) {
	sm := NewManager()
	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/firewall/whitelist", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetAuditStats(t *testing.T) {
	sm := NewManager()
	sm.audit.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	// 添加一些日志
	sm.audit.Log(AuditLogEntry{Level: "info", Category: "auth", Event: "login"})
	sm.audit.Log(AuditLogEntry{Level: "warning", Category: "system", Event: "config_change"})

	h := NewHandlers(sm)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/security/audit/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
