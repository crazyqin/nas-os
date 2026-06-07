package apigateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

// ==================== 路由测试 ====================

func TestAddRoute(t *testing.T) {
	m := setupTestManager(t)

	route := &Route{
		Name:       "test-route",
		Path:       "/api/test",
		Methods:    []string{"GET", "POST"},
		UpstreamID: "upstream-1",
		Enabled:    true,
	}

	err := m.AddRoute(route)
	if err != nil {
		t.Fatalf("AddRoute failed: %v", err)
	}

	if route.ID == "" {
		t.Error("expected non-empty route ID")
	}

	// 测试重复添加
	err = m.AddRoute(route)
	if err == nil {
		t.Error("expected error for duplicate route")
	}
}

func TestGetRoute(t *testing.T) {
	m := setupTestManager(t)

	route := &Route{
		Name:    "test-route",
		Path:    "/api/test",
		Enabled: true,
	}
	m.AddRoute(route)

	got, err := m.GetRoute(route.ID)
	if err != nil {
		t.Fatalf("GetRoute failed: %v", err)
	}
	if got.Name != route.Name {
		t.Errorf("expected name %s, got %s", route.Name, got.Name)
	}

	// 测试不存在的路由
	_, err = m.GetRoute("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent route")
	}
}

func TestUpdateRoute(t *testing.T) {
	m := setupTestManager(t)

	route := &Route{
		Name:    "test-route",
		Path:    "/api/test",
		Enabled: true,
	}
	m.AddRoute(route)

	route.Name = "updated-route"
	err := m.UpdateRoute(route)
	if err != nil {
		t.Fatalf("UpdateRoute failed: %v", err)
	}

	got, _ := m.GetRoute(route.ID)
	if got.Name != "updated-route" {
		t.Errorf("expected name 'updated-route', got %s", got.Name)
	}
}

func TestDeleteRoute(t *testing.T) {
	m := setupTestManager(t)

	route := &Route{
		Name:    "test-route",
		Path:    "/api/test",
		Enabled: true,
	}
	m.AddRoute(route)

	err := m.DeleteRoute(route.ID)
	if err != nil {
		t.Fatalf("DeleteRoute failed: %v", err)
	}

	_, err = m.GetRoute(route.ID)
	if err == nil {
		t.Error("expected error after deleting route")
	}
}

func TestListRoutes(t *testing.T) {
	m := setupTestManager(t)

	for i := 0; i < 3; i++ {
		m.AddRoute(&Route{
			Name:    fmt.Sprintf("route-%d", i),
			Path:    fmt.Sprintf("/api/test/%d", i),
			Enabled: true,
		})
	}

	routes := m.ListRoutes()
	if len(routes) != 3 {
		t.Errorf("expected 3 routes, got %d", len(routes))
	}
}

func TestMatchRoute(t *testing.T) {
	m := setupTestManager(t)

	m.AddRoute(&Route{
		Name:       "api-users",
		Path:       "/api/users",
		Methods:    []string{"GET", "POST"},
		UpstreamID: "upstream-1",
		Enabled:    true,
	})

	// 匹配成功
	route := m.MatchRoute("GET", "/api/users")
	if route == nil {
		t.Error("expected route to match")
	}

	// 方法不匹配
	route = m.MatchRoute("DELETE", "/api/users")
	if route != nil {
		t.Error("expected route not to match for DELETE")
	}

	// 路径不匹配
	route = m.MatchRoute("GET", "/api/orders")
	if route != nil {
		t.Error("expected route not to match for /api/orders")
	}
}

// ==================== 上游服务测试 ====================

func TestAddUpstream(t *testing.T) {
	m := setupTestManager(t)

	upstream := &Upstream{
		Name:      "test-upstream",
		Algorithm: "round-robin",
		Targets: []Target{
			{Host: "127.0.0.1", Port: 8001, Weight: 100},
			{Host: "127.0.0.1", Port: 8002, Weight: 100},
		},
		Enabled: true,
	}

	err := m.AddUpstream(upstream)
	if err != nil {
		t.Fatalf("AddUpstream failed: %v", err)
	}

	if upstream.ID == "" {
		t.Error("expected non-empty upstream ID")
	}
}

func TestSelectTarget(t *testing.T) {
	m := setupTestManager(t)

	upstream := &Upstream{
		Name:      "test-upstream",
		Algorithm: "round-robin",
		Targets: []Target{
			{ID: "t1", Host: "127.0.0.1", Port: 8001, Weight: 100, Health: "healthy"},
			{ID: "t2", Host: "127.0.0.1", Port: 8002, Weight: 100, Health: "healthy"},
		},
		Enabled: true,
	}
	m.AddUpstream(upstream)

	// 选择目标
	target, err := m.SelectTarget(upstream.ID)
	if err != nil {
		t.Fatalf("SelectTarget failed: %v", err)
	}
	if target == nil {
		t.Error("expected non-nil target")
	}

	// 所有目标不健康
	upstream2 := &Upstream{
		Name:      "unhealthy-upstream",
		Algorithm: "round-robin",
		Targets: []Target{
			{ID: "t3", Host: "127.0.0.1", Port: 8003, Weight: 100, Health: "unhealthy"},
		},
		Enabled: true,
	}
	m.AddUpstream(upstream2)

	_, err = m.SelectTarget(upstream2.ID)
	if err == nil {
		t.Error("expected error for unhealthy targets")
	}
}

func TestAddTarget(t *testing.T) {
	m := setupTestManager(t)

	upstream := &Upstream{
		Name:    "test-upstream",
		Targets: []Target{},
		Enabled: true,
	}
	m.AddUpstream(upstream)

	target := &Target{Host: "127.0.0.1", Port: 8003, Weight: 100}
	err := m.AddTarget(upstream.ID, target)
	if err != nil {
		t.Fatalf("AddTarget failed: %v", err)
	}

	got, _ := m.GetUpstream(upstream.ID)
	if len(got.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(got.Targets))
	}
}

func TestRemoveTarget(t *testing.T) {
	m := setupTestManager(t)

	upstream := &Upstream{
		Name: "test-upstream",
		Targets: []Target{
			{ID: "target-1", Host: "127.0.0.1", Port: 8001, Weight: 100},
		},
		Enabled: true,
	}
	m.AddUpstream(upstream)

	err := m.RemoveTarget(upstream.ID, "target-1")
	if err != nil {
		t.Fatalf("RemoveTarget failed: %v", err)
	}

	got, _ := m.GetUpstream(upstream.ID)
	if len(got.Targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(got.Targets))
	}
}

// ==================== 限流器测试 ====================

func TestTokenBucketLimiter(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 20) // 10/s, burst 20

	// 应该允许 20 个请求（burst）
	for i := 0; i < 20; i++ {
		if !limiter.Allow("test") {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// 第 21 个应该被拒绝
	if limiter.Allow("test") {
		t.Error("21st request should be rejected")
	}

	// 重置后应该允许
	limiter.Reset("test")
	if !limiter.Allow("test") {
		t.Error("request after reset should be allowed")
	}
}

func TestSlidingWindowLimiter(t *testing.T) {
	limiter := NewSlidingWindowLimiter(10, time.Second)

	// 应该允许 10 个请求
	for i := 0; i < 10; i++ {
		if !limiter.Allow("test") {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// 第 11 个应该被拒绝
	if limiter.Allow("test") {
		t.Error("11th request should be rejected")
	}
}

// ==================== 认证测试 ====================

func TestAPIKey(t *testing.T) {
	m := setupTestManager(t)

	keyInfo := &APIKeyInfo{
		Key:        "test-api-key",
		ConsumerID: "consumer-1",
		Name:       "Test Key",
		Enabled:    true,
	}

	err := m.AddAPIKey(keyInfo)
	if err != nil {
		t.Fatalf("AddAPIKey failed: %v", err)
	}

	// 验证有效 key
	validKey, err := m.ValidateAPIKey("test-api-key")
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}
	if validKey.ConsumerID != "consumer-1" {
		t.Errorf("expected consumer-1, got %s", validKey.ConsumerID)
	}

	// 验证无效 key
	_, err = m.ValidateAPIKey("invalid-key")
	if err == nil {
		t.Error("expected error for invalid key")
	}

	// 测试禁用 key
	keyInfo.Enabled = false
	m.DeleteAPIKey("test-api-key")
	m.AddAPIKey(keyInfo)
	_, err = m.ValidateAPIKey("test-api-key")
	if err == nil {
		t.Error("expected error for disabled key")
	}

	// 测试过期 key
	keyInfo.Enabled = true
	keyInfo.ExpiresAt = time.Now().Add(-1 * time.Hour)
	m.DeleteAPIKey("test-api-key")
	m.AddAPIKey(keyInfo)
	_, err = m.ValidateAPIKey("test-api-key")
	if err == nil {
		t.Error("expected error for expired key")
	}
}

func TestListAPIKeys(t *testing.T) {
	m := setupTestManager(t)

	m.AddAPIKey(&APIKeyInfo{Key: "key1", ConsumerID: "c1", Enabled: true})
	m.AddAPIKey(&APIKeyInfo{Key: "key2", ConsumerID: "c1", Enabled: true})
	m.AddAPIKey(&APIKeyInfo{Key: "key3", ConsumerID: "c2", Enabled: true})

	// 列出所有
	keys := m.ListAPIKeys("")
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	// 按消费者过滤
	keys = m.ListAPIKeys("c1")
	if len(keys) != 2 {
		t.Errorf("expected 2 keys for c1, got %d", len(keys))
	}
}

// ==================== 消费者测试 ====================

func TestConsumer(t *testing.T) {
	m := setupTestManager(t)

	consumer := &Consumer{
		Username: "test-user",
		CustomID: "custom-123",
		Enabled:  true,
	}

	err := m.AddConsumer(consumer)
	if err != nil {
		t.Fatalf("AddConsumer failed: %v", err)
	}

	got, err := m.GetConsumer(consumer.ID)
	if err != nil {
		t.Fatalf("GetConsumer failed: %v", err)
	}
	if got.Username != "test-user" {
		t.Errorf("expected username 'test-user', got %s", got.Username)
	}

	// 列出消费者
	consumers := m.ListConsumers()
	if len(consumers) != 1 {
		t.Errorf("expected 1 consumer, got %d", len(consumers))
	}

	// 删除消费者
	err = m.DeleteConsumer(consumer.ID)
	if err != nil {
		t.Fatalf("DeleteConsumer failed: %v", err)
	}
}

// ==================== 熔断器测试 ====================

func TestCircuitBreaker(t *testing.T) {
	m := setupTestManager(t)

	upstream := &Upstream{
		Name:    "test-upstream",
		Targets: []Target{{Host: "127.0.0.1", Port: 8001, Weight: 100}},
		HealthCheck: &HealthCheck{
			Active: &ActiveHealthCheck{
				HTTPPath: "/health",
				Interval: 10 * time.Second,
			},
		},
		Enabled: true,
	}
	m.AddUpstream(upstream)

	// 初始状态应该是 closed
	state, err := m.GetCircuitBreakerState(upstream.ID)
	if err != nil {
		t.Fatalf("GetCircuitBreakerState failed: %v", err)
	}
	if state != StateClosed {
		t.Errorf("expected state closed, got %s", state)
	}
}

// ==================== 请求日志测试 ====================

func TestRequestLog(t *testing.T) {
	m := setupTestManager(t)

	// 记录请求
	m.LogRequest(&RequestLog{
		Method:     "GET",
		Path:       "/api/test",
		StatusCode: 200,
		ClientIP:   "127.0.0.1",
		Duration:   100 * time.Millisecond,
	})

	m.LogRequest(&RequestLog{
		Method:     "POST",
		Path:       "/api/users",
		StatusCode: 201,
		ClientIP:   "127.0.0.1",
		Duration:   150 * time.Millisecond,
	})

	// 获取日志
	logs := m.GetRequestLogs(10)
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}

	// 清除日志
	m.ClearRequestLogs()
	logs = m.GetRequestLogs(10)
	if len(logs) != 0 {
		t.Errorf("expected 0 logs after clear, got %d", len(logs))
	}
}

// ==================== 版本管理测试 ====================

func TestAPIVersion(t *testing.T) {
	m := setupTestManager(t)

	m.AddAPIVersion(&APIVersion{
		Version:     "v1",
		Description: "Version 1",
		Deprecated:  false,
	})

	m.AddAPIVersion(&APIVersion{
		Version:     "v2",
		Description: "Version 2",
		Deprecated:  false,
	})

	// 获取版本
	v, err := m.GetAPIVersion("v1")
	if err != nil {
		t.Fatalf("GetAPIVersion failed: %v", err)
	}
	if v.Version != "v1" {
		t.Errorf("expected v1, got %s", v.Version)
	}

	// 列出版本
	versions := m.ListAPIVersions()
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}

	// 不存在的版本
	_, err = m.GetAPIVersion("v3")
	if err == nil {
		t.Error("expected error for nonexistent version")
	}
}

// ==================== 统计测试 ====================

func TestStats(t *testing.T) {
	m := setupTestManager(t)

	// 记录一些请求
	m.LogRequest(&RequestLog{
		Method:       "GET",
		Path:         "/api/test",
		StatusCode:   200,
		RequestSize:  100,
		ResponseSize: 500,
	})

	stats := m.GetStats()
	if stats.TotalRequests != 1 {
		t.Errorf("expected 1 request, got %d", stats.TotalRequests)
	}
	if stats.BytesReceived != 100 {
		t.Errorf("expected 100 bytes received, got %d", stats.BytesReceived)
	}
	if stats.BytesSent != 500 {
		t.Errorf("expected 500 bytes sent, got %d", stats.BytesSent)
	}
}

// ==================== 网关控制测试 ====================

func TestGatewayStartStop(t *testing.T) {
	m := setupTestManager(t)

	// 启动
	err := m.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Error("expected gateway to be running")
	}

	// 重复启动
	err = m.Start()
	if err == nil {
		t.Error("expected error for duplicate start")
	}

	// 停止
	err = m.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if m.IsRunning() {
		t.Error("expected gateway to be stopped")
	}
}

// ==================== 配置测试 ====================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultGatewayConfig()

	if cfg.ListenPort != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.ListenPort)
	}
	if cfg.MaxBodySize != 10<<20 {
		t.Errorf("expected max body size 10MB, got %d", cfg.MaxBodySize)
	}
	if !cfg.RateLimit.Enabled {
		t.Error("expected rate limit to be enabled")
	}
	if !cfg.CORS.Enabled {
		t.Error("expected CORS to be enabled")
	}
}

func TestGetUpdateConfig(t *testing.T) {
	m := setupTestManager(t)

	cfg := m.GetConfig()
	if cfg.ListenPort != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.ListenPort)
	}

	cfg.ListenPort = 9090
	m.UpdateConfig(cfg)

	cfg = m.GetConfig()
	if cfg.ListenPort != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.ListenPort)
	}
}

// ==================== Handler 测试 ====================

func TestHandler_GetStatus(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-gateway/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetStats(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-gateway/stats", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Routes(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 添加路由
	body := `{"name":"test-route","path":"/api/test","methods":["GET"],"upstream_id":"up-1","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-gateway/routes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// 列出路由
	req = httptest.NewRequest(http.MethodGet, "/api/v1/api-gateway/routes", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	routesData, _ := json.Marshal(resp.Data)
	var routes []Route
	json.Unmarshal(routesData, &routes)

	if len(routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(routes))
	}
}

func TestHandler_Upstreams(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 添加上游
	body := `{"name":"test-upstream","algorithm":"round-robin","targets":[{"host":"127.0.0.1","port":8001,"weight":100}],"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-gateway/upstreams", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// 列出上游
	req = httptest.NewRequest(http.MethodGet, "/api/v1/api-gateway/upstreams", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Consumers(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 添加消费者
	body := `{"username":"test-user","custom_id":"custom-123","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-gateway/consumers", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// 列出消费者
	req = httptest.NewRequest(http.MethodGet, "/api/v1/api-gateway/consumers", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_APIKeys(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 添加 API Key
	body := `{"key":"test-key","consumer_id":"consumer-1","name":"Test Key","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-gateway/api-keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// 列出 API Keys
	req = httptest.NewRequest(http.MethodGet, "/api/v1/api-gateway/api-keys", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Versions(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 添加版本
	body := `{"version":"v1","description":"Version 1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-gateway/versions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// 列出版本
	req = httptest.NewRequest(http.MethodGet, "/api/v1/api-gateway/versions", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Logs(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 先记录一些日志
	m.LogRequest(&RequestLog{
		Method:     "GET",
		Path:       "/api/test",
		StatusCode: 200,
	})

	// 获取日志
	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-gateway/logs?limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 清除日志
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/api-gateway/logs", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_StartStop(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 启动
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-gateway/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 停止
	req = httptest.NewRequest(http.MethodPost, "/api/v1/api-gateway/stop", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Config(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 获取配置
	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-gateway/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 更新配置
	body := `{"listen_port":9090}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/api-gateway/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
