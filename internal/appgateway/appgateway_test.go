package appgateway

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	manager := NewManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.config == nil {
		t.Fatal("Expected default config to be set")
	}

	if manager.config.ListenPort != 80 {
		t.Errorf("Expected default port 80, got %d", manager.config.ListenPort)
	}

	if manager.config.TLSPort != 443 {
		t.Errorf("Expected default TLS port 443, got %d", manager.config.TLSPort)
	}
}

func TestRegisterApp(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name:     "test-app",
		Port:     8080,
		Protocol: "http",
	}

	err := manager.RegisterApp(app)
	if err != nil {
		t.Fatalf("Failed to register app: %v", err)
	}

	if app.ID == "" {
		t.Error("Expected app ID to be generated")
	}

	if !app.Enabled {
		t.Error("Expected app to be enabled")
	}

	// 测试重复注册
	err = manager.RegisterApp(app)
	if err == nil {
		t.Error("Expected error for duplicate app")
	}

	// 测试空名称
	err = manager.RegisterApp(&Application{Port: 8080})
	if err == nil {
		t.Error("Expected error for empty name")
	}

	// 测试无效端口
	err = manager.RegisterApp(&Application{Name: "test", Port: -1})
	if err == nil {
		t.Error("Expected error for invalid port")
	}
}

func TestUnregisterApp(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
	}
	manager.RegisterApp(app)

	// 注销应用
	err := manager.UnregisterApp(app.ID)
	if err != nil {
		t.Fatalf("Failed to unregister app: %v", err)
	}

	// 验证应用已删除
	_, err = manager.GetApp(app.ID)
	if err == nil {
		t.Error("Expected error for deleted app")
	}

	// 测试删除不存在的应用
	err = manager.UnregisterApp("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent app")
	}
}

func TestGetApp(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
	}
	manager.RegisterApp(app)

	// 获取应用
	fetched, err := manager.GetApp(app.ID)
	if err != nil {
		t.Fatalf("Failed to get app: %v", err)
	}

	if fetched.Name != "test-app" {
		t.Errorf("Expected name 'test-app', got '%s'", fetched.Name)
	}

	// 测试获取不存在的应用
	_, err = manager.GetApp("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent app")
	}
}

func TestUpdateApp(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
	}
	manager.RegisterApp(app)

	// 更新应用
	updated := &Application{
		ID:          app.ID,
		Name:        "updated-app",
		Description: "Updated description",
	}
	err := manager.UpdateApp(updated)
	if err != nil {
		t.Fatalf("Failed to update app: %v", err)
	}

	fetched, _ := manager.GetApp(app.ID)
	if fetched.Name != "updated-app" {
		t.Errorf("Expected name 'updated-app', got '%s'", fetched.Name)
	}
}

func TestListApps(t *testing.T) {
	manager := NewManager(nil)

	manager.RegisterApp(&Application{Name: "app1", Port: 8001})
	manager.RegisterApp(&Application{Name: "app2", Port: 8002})
	manager.RegisterApp(&Application{Name: "app3", Port: 8003})

	apps := manager.ListApps()
	if len(apps) != 3 {
		t.Errorf("Expected 3 apps, got %d", len(apps))
	}
}

func TestEnableDisableApp(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
	}
	manager.RegisterApp(app)

	// 禁用应用
	err := manager.DisableApp(app.ID)
	if err != nil {
		t.Fatalf("Failed to disable app: %v", err)
	}

	fetched, _ := manager.GetApp(app.ID)
	if fetched.Enabled {
		t.Error("Expected app to be disabled")
	}

	// 启用应用
	err = manager.EnableApp(app.ID)
	if err != nil {
		t.Fatalf("Failed to enable app: %v", err)
	}

	fetched, _ = manager.GetApp(app.ID)
	if !fetched.Enabled {
		t.Error("Expected app to be enabled")
	}
}

func TestInstanceManagement(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
	}
	manager.RegisterApp(app)

	// 添加实例
	instance := &AppInstance{
		Host:   "192.168.1.100",
		Port:   8081,
		Weight: 2,
	}
	err := manager.AddInstance(app.ID, instance)
	if err != nil {
		t.Fatalf("Failed to add instance: %v", err)
	}

	// 验证实例已添加
	fetched, _ := manager.GetApp(app.ID)
	if len(fetched.Instances) != 2 { // 默认实例 + 新实例
		t.Errorf("Expected 2 instances, got %d", len(fetched.Instances))
	}

	// 更新实例健康状态
	err = manager.UpdateInstanceHealth(app.ID, instance.ID, "healthy")
	if err != nil {
		t.Fatalf("Failed to update instance health: %v", err)
	}

	// 移除实例
	err = manager.RemoveInstance(app.ID, instance.ID)
	if err != nil {
		t.Fatalf("Failed to remove instance: %v", err)
	}

	fetched, _ = manager.GetApp(app.ID)
	if len(fetched.Instances) != 1 {
		t.Errorf("Expected 1 instance after removal, got %d", len(fetched.Instances))
	}
}

func TestRouteManagement(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
	}
	manager.RegisterApp(app)

	// 添加路由
	route := &RouteRule{
		AppID:  app.ID,
		Domain: "app.nas.local",
		Path:   "/test-app",
	}
	err := manager.AddRoute(route)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	if route.ID == "" {
		t.Error("Expected route ID to be generated")
	}

	// 列出路由
	routes := manager.ListRoutes()
	if len(routes) != 1 {
		t.Errorf("Expected 1 route, got %d", len(routes))
	}

	// 获取路由
	fetched, err := manager.GetRoute(route.ID)
	if err != nil {
		t.Fatalf("Failed to get route: %v", err)
	}
	if fetched.Domain != "app.nas.local" {
		t.Errorf("Expected domain 'app.nas.local', got '%s'", fetched.Domain)
	}

	// 删除路由
	err = manager.DeleteRoute(route.ID)
	if err != nil {
		t.Fatalf("Failed to delete route: %v", err)
	}

	routes = manager.ListRoutes()
	if len(routes) != 0 {
		t.Errorf("Expected 0 routes after deletion, got %d", len(routes))
	}
}

func TestMatchRoute(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
	}
	manager.RegisterApp(app)

	route := &RouteRule{
		AppID:  app.ID,
		Domain: "app.nas.local",
		Path:   "/test-app",
	}
	manager.AddRoute(route)

	// 测试匹配
	matchedRoute, matchedApp := manager.MatchRoute("app.nas.local", "/test-app/api")
	if matchedRoute == nil {
		t.Fatal("Expected route to match")
	}
	if matchedApp.ID != app.ID {
		t.Errorf("Expected app ID '%s', got '%s'", app.ID, matchedApp.ID)
	}

	// 测试不匹配
	matchedRoute, _ = manager.MatchRoute("other.local", "/other")
	if matchedRoute != nil {
		t.Error("Expected route not to match")
	}
}

func TestMatchDomain(t *testing.T) {
	// 精确匹配
	if !matchDomain("app.nas.local", "app.nas.local") {
		t.Error("Expected exact match")
	}

	// 通配符匹配
	if !matchDomain("*.nas.local", "app.nas.local") {
		t.Error("Expected wildcard match")
	}

	// 不匹配
	if matchDomain("app.nas.local", "other.nas.local") {
		t.Error("Expected no match")
	}
}

func TestLoadBalancing(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
		Instances: []AppInstance{
			{ID: "1", Host: "host1", Port: 8001, Weight: 1, Health: "healthy"},
			{ID: "2", Host: "host2", Port: 8002, Weight: 1, Health: "healthy"},
			{ID: "3", Host: "host3", Port: 8003, Weight: 1, Health: "healthy"},
		},
	}
	manager.RegisterApp(app)

	// 测试轮询算法
	manager.SetLoadBalancerAlgorithm(AlgorithmRoundRobin)

	selected1, _ := manager.SelectInstance(app, "192.168.1.1")
	selected2, _ := manager.SelectInstance(app, "192.168.1.1")
	selected3, _ := manager.SelectInstance(app, "192.168.1.1")
	selected4, _ := manager.SelectInstance(app, "192.168.1.1")

	// 应该轮询选择不同的实例
	if selected1.ID == selected2.ID && selected2.ID == selected3.ID && selected3.ID == selected4.ID {
		t.Error("Expected round-robin to select different instances")
	}

	// 测试加权算法
	manager.SetLoadBalancerAlgorithm(AlgorithmWeighted)

	// 测试IP哈希算法
	manager.SetLoadBalancerAlgorithm(AlgorithmIPHash)
	selected, _ := manager.SelectInstance(app, "192.168.1.1")
	if selected == nil {
		t.Error("Expected instance to be selected")
	}

	// 相同IP应选择相同实例
	selected2, _ = manager.SelectInstance(app, "192.168.1.1")
	if selected.ID != selected2.ID {
		t.Error("Expected same IP to select same instance")
	}

	// 测试无不健康实例
	app2 := &Application{
		Name: "test-app2",
		Port: 8080,
		Instances: []AppInstance{
			{ID: "1", Host: "host1", Port: 8001, Health: "unhealthy"},
		},
	}
	manager.RegisterApp(app2)

	_, err := manager.SelectInstance(app2, "192.168.1.1")
	if err == nil {
		t.Error("Expected error when no healthy instances")
	}
}

func TestAccessControl(t *testing.T) {
	manager := NewManager(nil)

	// 测试IP白名单
	app := &Application{
		Name: "test-app",
		Port: 8080,
		Access: &AccessConfig{
			AllowedIPs: []string{"192.168.1.0/24", "10.0.0.1"},
		},
	}
	manager.RegisterApp(app)

	// 允许的IP
	err := manager.CheckAccess(app, "192.168.1.100")
	if err != nil {
		t.Errorf("Expected access allowed, got error: %v", err)
	}

	// 不允许的IP
	err = manager.CheckAccess(app, "172.16.0.1")
	if err == nil {
		t.Error("Expected access denied")
	}

	// 测试IP黑名单
	app2 := &Application{
		Name: "test-app2",
		Port: 8080,
		Access: &AccessConfig{
			BlockedIPs: []string{"192.168.1.100"},
		},
	}
	manager.RegisterApp(app2)

	err = manager.CheckAccess(app2, "192.168.1.100")
	if err == nil {
		t.Error("Expected access denied for blocked IP")
	}

	// 允许的IP
	err = manager.CheckAccess(app2, "192.168.1.200")
	if err != nil {
		t.Errorf("Expected access allowed, got error: %v", err)
	}
}

func TestAPIKeyAuth(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
		Access: &AccessConfig{
			APIKey: "test-api-key",
		},
	}
	manager.RegisterApp(app)

	// 有效的API Key
	err := manager.CheckAPIKey(app, "test-api-key")
	if err != nil {
		t.Errorf("Expected valid API key, got error: %v", err)
	}

	// 无效的API Key
	err = manager.CheckAPIKey(app, "invalid-key")
	if err == nil {
		t.Error("Expected invalid API key error")
	}
}

func TestBasicAuth(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
		Access: &AccessConfig{
			BasicAuth: &BasicAuth{
				Username: "admin",
				Password: "password",
			},
		},
	}
	manager.RegisterApp(app)

	// 有效的认证
	err := manager.CheckBasicAuth(app, "admin", "password")
	if err != nil {
		t.Errorf("Expected valid auth, got error: %v", err)
	}

	// 无效的认证
	err = manager.CheckBasicAuth(app, "admin", "wrong")
	if err == nil {
		t.Error("Expected invalid auth error")
	}
}

func TestHealthCheck(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
		Instances: []AppInstance{
			{ID: "1", Host: "host1", Port: 8001, Health: "healthy"},
			{ID: "2", Host: "host2", Port: 8002, Health: "unhealthy"},
		},
		HealthCheck: &HealthCheckConfig{
			Enabled:  true,
			Path:     "/health",
			Interval: 30 * time.Second,
		},
	}
	manager.RegisterApp(app)

	// 获取健康状态
	status, err := manager.CheckHealth(app.ID)
	if err != nil {
		t.Fatalf("Failed to check health: %v", err)
	}

	if status["1"] != "healthy" {
		t.Errorf("Expected instance 1 to be healthy, got '%s'", status["1"])
	}

	if status["2"] != "unhealthy" {
		t.Errorf("Expected instance 2 to be unhealthy, got '%s'", status["2"])
	}

	// 获取健康实例
	healthy, err := manager.GetHealthyInstances(app.ID)
	if err != nil {
		t.Fatalf("Failed to get healthy instances: %v", err)
	}

	if len(healthy) != 1 {
		t.Errorf("Expected 1 healthy instance, got %d", len(healthy))
	}
}

func TestRequestLogging(t *testing.T) {
	manager := NewManager(nil)

	// 记录请求
	manager.LogRequest(&AccessLog{
		AppID:      "app1",
		AppName:    "Test App",
		Method:     "GET",
		Path:       "/api/test",
		StatusCode: 200,
		ClientIP:   "192.168.1.1",
		Duration:   100 * time.Millisecond,
	})

	// 获取日志
	logs := manager.GetRequestLogs(10)
	if len(logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(logs))
	}

	if logs[0].AppID != "app1" {
		t.Errorf("Expected app ID 'app1', got '%s'", logs[0].AppID)
	}

	// 清除日志
	manager.ClearRequestLogs()
	logs = manager.GetRequestLogs(10)
	if len(logs) != 0 {
		t.Errorf("Expected 0 logs after clear, got %d", len(logs))
	}
}

func TestStats(t *testing.T) {
	manager := NewManager(nil)

	// 注册一些应用
	app1 := &Application{Name: "app1", Port: 8001}
	manager.RegisterApp(app1)
	app2 := &Application{Name: "app2", Port: 8002}
	manager.RegisterApp(app2)
	manager.DisableApp(app2.ID)

	stats := manager.GetStats()

	if stats.TotalApps != 2 {
		t.Errorf("Expected 2 total apps, got %d", stats.TotalApps)
	}

	if stats.ActiveApps != 1 {
		t.Errorf("Expected 1 active app, got %d", stats.ActiveApps)
	}
}

func TestStartStop(t *testing.T) {
	manager := NewManager(nil)

	// 启动
	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	if !manager.IsRunning() {
		t.Error("Expected manager to be running")
	}

	// 重复启动
	err = manager.Start()
	if err == nil {
		t.Error("Expected error for duplicate start")
	}

	// 停止
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop: %v", err)
	}

	if manager.IsRunning() {
		t.Error("Expected manager to be stopped")
	}
}

func TestRouterServeHTTP(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
		Access: &AccessConfig{
			APIKey: "test-key",
		},
	}
	manager.RegisterApp(app)

	route := &RouteRule{
		AppID:  app.ID,
		Domain: "localhost",
		Path:   "/test",
	}
	manager.AddRoute(route)

	router := NewRouter(manager)

	// 测试未匹配的路由
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	req.Host = "unknown.local"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestRouterAPIKeyCheck(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
		Access: &AccessConfig{
			APIKey: "test-key",
		},
	}
	manager.RegisterApp(app)

	route := &RouteRule{
		AppID:  app.ID,
		Domain: "localhost",
		Path:   "/test",
	}
	manager.AddRoute(route)

	router := NewRouter(manager)

	// 测试无效的API Key
	req := httptest.NewRequest("GET", "/test/api", nil)
	req.Host = "localhost"
	req.Header.Set("X-API-Key", "invalid-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	// 测试有效的API Key
	req = httptest.NewRequest("GET", "/test/api", nil)
	req.Host = "localhost"
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 应该不是401（可能是502因为没有真实后端）
	if w.Code == http.StatusUnauthorized {
		t.Error("Expected API key to be accepted")
	}
}

func TestRouterBasicAuth(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
		Access: &AccessConfig{
			RequireAuth: true,
			BasicAuth: &BasicAuth{
				Username: "admin",
				Password: "password",
			},
		},
	}
	manager.RegisterApp(app)

	route := &RouteRule{
		AppID:  app.ID,
		Domain: "localhost",
		Path:   "/test",
	}
	manager.AddRoute(route)

	router := NewRouter(manager)

	// 测试未认证
	req := httptest.NewRequest("GET", "/test/api", nil)
	req.Host = "localhost"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	// 测试有效的 Basic Auth
	creds := base64.StdEncoding.EncodeToString([]byte("admin:password"))
	req = httptest.NewRequest("GET", "/test/api", nil)
	req.Host = "localhost"
	req.Header.Set("Authorization", "Basic "+creds)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 应该不是401
	if w.Code == http.StatusUnauthorized {
		t.Error("Expected basic auth to be accepted")
	}
}

func TestRouterAccessControl(t *testing.T) {
	manager := NewManager(nil)

	app := &Application{
		Name: "test-app",
		Port: 8080,
		Access: &AccessConfig{
			BlockedIPs: []string{"192.168.1.100"},
		},
	}
	manager.RegisterApp(app)

	route := &RouteRule{
		AppID:  app.ID,
		Domain: "localhost",
		Path:   "/test",
	}
	manager.AddRoute(route)

	router := NewRouter(manager)

	// 测试被阻止的IP
	req := httptest.NewRequest("GET", "/test/api", nil)
	req.Host = "localhost"
	req.Header.Set("X-Real-IP", "192.168.1.100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestMatchIP(t *testing.T) {
	// 精确匹配
	if !matchIP("192.168.1.1", "192.168.1.1") {
		t.Error("Expected exact IP match")
	}

	// CIDR匹配
	if !matchIP("192.168.1.0/24", "192.168.1.100") {
		t.Error("Expected CIDR match")
	}

	// 不匹配
	if matchIP("192.168.1.0/24", "10.0.0.1") {
		t.Error("Expected no match")
	}
}

func TestStripPathPrefix(t *testing.T) {
	tests := []struct {
		path     string
		prefix   string
		expected string
	}{
		{"/app/api/test", "/app", "/api/test"},
		{"/api/v1/users", "/api/v1", "/users"},
		{"/test", "", "/test"},
	}

	for _, tt := range tests {
		result := stripPathPrefix(tt.path, tt.prefix)
		if result != tt.expected {
			t.Errorf("stripPathPrefix(%s, %s) = %s, want %s", tt.path, tt.prefix, result, tt.expected)
		}
	}
}

func TestResponseRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

	// 测试 WriteHeader
	rec.WriteHeader(http.StatusNotFound)
	if rec.statusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rec.statusCode)
	}

	// 测试 Write
	rec.Write([]byte("test"))
	if !rec.written {
		t.Error("Expected written to be true")
	}
}

func TestProxyHandler(t *testing.T) {
	manager := NewManager(nil)
	handler := NewProxyHandler(manager)

	httpHandler := handler.Handler()
	if httpHandler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestWebSocketDetection(t *testing.T) {
	// 测试 WebSocket 请求检测
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	if !isWebSocketRequest(req) {
		t.Error("Expected WebSocket request to be detected")
	}

	// 测试普通请求
	req2 := httptest.NewRequest("GET", "/api", nil)
	if isWebSocketRequest(req2) {
		t.Error("Expected non-WebSocket request")
	}
}

func TestGetClientIP(t *testing.T) {
	// 测试 X-Forwarded-For
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
	req.RemoteAddr = "127.0.0.1:12345"

	ip := getClientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("Expected '192.168.1.1', got '%s'", ip)
	}

	// 测试 X-Real-IP
	req.Header.Del("X-Forwarded-For")
	req.Header.Set("X-Real-IP", "10.0.0.1")

	ip = getClientIP(req)
	if ip != "10.0.0.1" {
		t.Errorf("Expected '10.0.0.1', got '%s'", ip)
	}

	// 测试 RemoteAddr
	req.Header.Del("X-Real-IP")
	req.RemoteAddr = "172.16.0.1:54321"

	ip = getClientIP(req)
	if ip != "172.16.0.1" {
		t.Errorf("Expected '172.16.0.1', got '%s'", ip)
	}
}

func TestRouterEndpoints(t *testing.T) {
	manager := NewManager(nil)
	router := NewRouter(manager)

	// 注册应用
	app := &Application{Name: "test-app", Port: 8080}
	manager.RegisterApp(app)

	mux := http.NewServeMux()
	router.RegisterRoutes(mux)

	// 测试获取应用列表
	req := httptest.NewRequest("GET", "/api/appgateway/apps", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var apps []Application
	json.NewDecoder(w.Body).Decode(&apps)
	if len(apps) != 1 {
		t.Errorf("Expected 1 app, got %d", len(apps))
	}

	// 测试获取统计
	req = httptest.NewRequest("GET", "/api/appgateway/stats", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var stats GatewayStats
	json.NewDecoder(w.Body).Decode(&stats)
	if stats.TotalApps != 1 {
		t.Errorf("Expected 1 total app, got %d", stats.TotalApps)
	}
}
