// Package nasgateway API网关测试
package nasgateway

import (
	"net/http"
	"testing"
	"time"
)

// ========== Engine 测试 ==========

func TestEngine_StartStop(t *testing.T) {
	engine := NewEngine()

	t.Run("启动引擎", func(t *testing.T) {
		if err := engine.Start(); err != nil {
			t.Fatalf("启动引擎失败: %v", err)
		}
		if !engine.IsRunning() {
			t.Fatal("引擎应该处于运行状态")
		}
	})

	t.Run("重复启动", func(t *testing.T) {
		if err := engine.Start(); err != nil {
			t.Fatalf("重复启动不应报错: %v", err)
		}
	})

	t.Run("停止引擎", func(t *testing.T) {
		if err := engine.Stop(); err != nil {
			t.Fatalf("停止引擎失败: %v", err)
		}
		if engine.IsRunning() {
			t.Fatal("引擎应该处于停止状态")
		}
	})
}

func TestEngine_RouteManagement(t *testing.T) {
	engine := NewEngine()
	engine.Start()

	t.Run("添加路由", func(t *testing.T) {
		route := &Route{
			ID:      "route1",
			Name:    "测试路由",
			Path:    "/api/v1/test",
			Methods: []string{"GET", "POST"},
		}
		if err := engine.AddRoute(route); err != nil {
			t.Fatalf("添加路由失败: %v", err)
		}
	})

	t.Run("重复添加", func(t *testing.T) {
		route := &Route{ID: "route1", Name: "重复路由"}
		if err := engine.AddRoute(route); err == nil {
			t.Fatal("重复添加应该报错")
		}
	})

	t.Run("获取路由", func(t *testing.T) {
		route, err := engine.GetRoute("route1")
		if err != nil {
			t.Fatalf("获取路由失败: %v", err)
		}
		if route.Name != "测试路由" {
			t.Fatalf("期望测试路由，实际: %s", route.Name)
		}
	})

	t.Run("列出路由", func(t *testing.T) {
		routes := engine.ListRoutes()
		if len(routes) != 1 {
			t.Fatalf("期望1条路由，实际: %d", len(routes))
		}
	})

	t.Run("更新路由", func(t *testing.T) {
		route, _ := engine.GetRoute("route1")
		route.Name = "更新后的路由"
		if err := engine.UpdateRoute(route); err != nil {
			t.Fatalf("更新路由失败: %v", err)
		}
		updated, _ := engine.GetRoute("route1")
		if updated.Name != "更新后的路由" {
			t.Fatalf("期望更新后的路由，实际: %s", updated.Name)
		}
	})

	t.Run("查找路由", func(t *testing.T) {
		route, err := engine.FindRoute("GET", "/api/v1/test", "")
		if err != nil {
			t.Fatalf("查找路由失败: %v", err)
		}
		if route.ID != "route1" {
			t.Fatalf("期望route1，实际: %s", route.ID)
		}
	})

	t.Run("删除路由", func(t *testing.T) {
		if err := engine.DeleteRoute("route1"); err != nil {
			t.Fatalf("删除路由失败: %v", err)
		}
		if _, err := engine.GetRoute("route1"); err == nil {
			t.Fatal("删除后应该找不到路由")
		}
	})
}

func TestEngine_UpstreamManagement(t *testing.T) {
	engine := NewEngine()

	t.Run("添加上游服务", func(t *testing.T) {
		upstream := &Upstream{
			ID:   "upstream1",
			Name: "测试上游",
			Targets: []*Target{
				{ID: "t1", Host: "127.0.0.1", Port: 8080, Health: "healthy"},
				{ID: "t2", Host: "127.0.0.1", Port: 8081, Health: "healthy"},
			},
			Algorithm: "round-robin",
		}
		if err := engine.AddUpstream(upstream); err != nil {
			t.Fatalf("添加上游服务失败: %v", err)
		}
	})

	t.Run("获取上游服务", func(t *testing.T) {
		upstream, err := engine.GetUpstream("upstream1")
		if err != nil {
			t.Fatalf("获取上游服务失败: %v", err)
		}
		if len(upstream.Targets) != 2 {
			t.Fatalf("期望2个目标，实际: %d", len(upstream.Targets))
		}
	})

	t.Run("选择目标", func(t *testing.T) {
		target, err := engine.SelectTarget("upstream1")
		if err != nil {
			t.Fatalf("选择目标失败: %v", err)
		}
		if target == nil {
			t.Fatal("目标不应为空")
		}
	})
}

func TestEngine_PolicyManagement(t *testing.T) {
	engine := NewEngine()

	t.Run("添加限流策略", func(t *testing.T) {
		policy := &Policy{
			ID:   "policy1",
			Name: "限流策略",
			Type: PolicyTypeRateLimit,
			Config: map[string]interface{}{
				"rate_limit": map[string]interface{}{
					"requests_per_second": 100,
					"burst":               200,
				},
			},
		}
		if err := engine.AddPolicy(policy); err != nil {
			t.Fatalf("添加策略失败: %v", err)
		}
	})

	t.Run("添加熔断策略", func(t *testing.T) {
		policy := &Policy{
			ID:   "policy2",
			Name: "熔断策略",
			Type: PolicyTypeCircuitBreaker,
			Config: map[string]interface{}{
				"circuit_breaker": map[string]interface{}{
					"failure_threshold": 5,
					"timeout":          30 * time.Second,
				},
			},
		}
		if err := engine.AddPolicy(policy); err != nil {
			t.Fatalf("添加策略失败: %v", err)
		}
	})

	t.Run("列出策略", func(t *testing.T) {
		policies := engine.ListPolicies("")
		if len(policies) != 2 {
			t.Fatalf("期望2条策略，实际: %d", len(policies))
		}
	})

	t.Run("按类型列出", func(t *testing.T) {
		policies := engine.ListPolicies(PolicyTypeRateLimit)
		if len(policies) != 1 {
			t.Fatalf("期望1条限流策略，实际: %d", len(policies))
		}
	})
}

func TestEngine_ProcessRequest(t *testing.T) {
	engine := NewEngine()
	engine.Start()

	// 添加路由
	route := &Route{
		ID:      "route1",
		Path:    "/api/test",
		Methods: []string{"GET"},
	}
	engine.AddRoute(route)

	t.Run("正常请求", func(t *testing.T) {
		route, _, _, err := engine.ProcessRequest("GET", "/api/test", "", "127.0.0.1")
		if err != nil {
			t.Fatalf("处理请求失败: %v", err)
		}
		if route == nil {
			t.Fatal("路由不应为空")
		}
	})

	t.Run("路由不存在", func(t *testing.T) {
		_, _, _, err := engine.ProcessRequest("GET", "/notfound", "", "127.0.0.1")
		if err == nil {
			t.Fatal("应该返回路由不存在错误")
		}
	})
}

func TestEngine_PluginManagement(t *testing.T) {
	engine := NewEngine()

	// 创建测试插件
	plugin := &testPlugin{name: "test-plugin", desc: "测试插件"}

	t.Run("注册插件", func(t *testing.T) {
		config := &PluginConfig{
			Name:    "test-plugin",
			Enabled: true,
		}
		engine.RegisterPlugin(plugin, config)
	})

	t.Run("列出插件", func(t *testing.T) {
		plugins := engine.ListPlugins()
		if len(plugins) != 1 {
			t.Fatalf("期望1个插件，实际: %d", len(plugins))
		}
	})

	t.Run("注销插件", func(t *testing.T) {
		engine.UnregisterPlugin("test-plugin")
		plugins := engine.ListPlugins()
		if len(plugins) != 0 {
			t.Fatalf("期望0个插件，实际: %d", len(plugins))
		}
	})
}

func TestEngine_APIVersion(t *testing.T) {
	engine := NewEngine()

	t.Run("添加API版本", func(t *testing.T) {
		version := &APIVersion{
			Version:     "v1",
			Description: "版本1",
			BasePath:    "/api/v1",
		}
		engine.AddAPIVersion(version)
	})

	t.Run("获取API版本", func(t *testing.T) {
		version, err := engine.GetAPIVersion("v1")
		if err != nil {
			t.Fatalf("获取版本失败: %v", err)
		}
		if version.BasePath != "/api/v1" {
			t.Fatalf("期望/api/v1，实际: %s", version.BasePath)
		}
	})

	t.Run("列出API版本", func(t *testing.T) {
		versions := engine.ListAPIVersions()
		if len(versions) != 1 {
			t.Fatalf("期望1个版本，实际: %d", len(versions))
		}
	})
}

func TestEngine_RequestLog(t *testing.T) {
	engine := NewEngine()

	t.Run("记录请求日志", func(t *testing.T) {
		log := &RequestLog{
			ID:         "log1",
			RequestID:  "req1",
			Method:     "GET",
			Path:       "/api/test",
			StatusCode: 200,
			Latency:    time.Millisecond * 50,
			Timestamp:  time.Now(),
		}
		engine.LogRequest(log)
	})

	t.Run("获取请求日志", func(t *testing.T) {
		logs := engine.GetRequestLogs(10)
		if len(logs) != 1 {
			t.Fatalf("期望1条日志，实际: %d", len(logs))
		}
	})

	t.Run("清空日志", func(t *testing.T) {
		engine.ClearRequestLogs()
		logs := engine.GetRequestLogs(10)
		if len(logs) != 0 {
			t.Fatalf("期望0条日志，实际: %d", len(logs))
		}
	})
}

// ========== RateLimiter 测试 ==========

func TestRateLimiter_TokenBucket(t *testing.T) {
	limiter := NewRateLimiter(AlgorithmTokenBucket, 10, 20)

	t.Run("正常请求", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			if !limiter.Allow("key1") {
				t.Fatalf("第%d个请求应该被允许", i+1)
			}
		}
	})

	t.Run("超出限制", func(t *testing.T) {
		// 先消耗完burst
		for i := 0; i < 20; i++ {
			limiter.Allow("key2")
		}
		if limiter.Allow("key2") {
			t.Fatal("超出burst应该被拒绝")
		}
	})

	t.Run("重置", func(t *testing.T) {
		limiter.Reset("key1")
		if !limiter.Allow("key1") {
			t.Fatal("重置后应该允许请求")
		}
	})
}

func TestRateLimiter_SlidingWindow(t *testing.T) {
	limiter := NewRateLimiter(AlgorithmSlidingWindow, 10, 20)

	t.Run("滑动窗口限流", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			if !limiter.Allow("key1") {
				t.Fatalf("第%d个请求应该被允许", i+1)
			}
		}
	})
}

func TestRateLimiter_FixedWindow(t *testing.T) {
	limiter := NewRateLimiter(AlgorithmFixedWindow, 10, 20)

	t.Run("固定窗口限流", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			if !limiter.Allow("key1") {
				t.Fatalf("第%d个请求应该被允许", i+1)
			}
		}
	})
}

func TestRateLimiter_GetInfo(t *testing.T) {
	limiter := NewRateLimiter(AlgorithmTokenBucket, 100, 200)

	t.Run("获取限制数", func(t *testing.T) {
		if limiter.GetLimit() != 100 {
			t.Fatalf("期望100，实际: %d", limiter.GetLimit())
		}
	})

	t.Run("获取突发限制", func(t *testing.T) {
		if limiter.GetBurst() != 200 {
			t.Fatalf("期望200，实际: %d", limiter.GetBurst())
		}
	})

	t.Run("获取算法", func(t *testing.T) {
		if limiter.GetAlgorithm() != AlgorithmTokenBucket {
			t.Fatalf("期望token_bucket，实际: %s", limiter.GetAlgorithm())
		}
	})
}

// ========== WAF 测试 ==========

func TestWAF_EnableDisable(t *testing.T) {
	waf := NewWAF()

	t.Run("默认启用", func(t *testing.T) {
		if !waf.IsEnabled() {
			t.Fatal("WAF应该默认启用")
		}
	})

	t.Run("禁用", func(t *testing.T) {
		waf.Disable()
		if waf.IsEnabled() {
			t.Fatal("WAF应该被禁用")
		}
	})

	t.Run("启用", func(t *testing.T) {
		waf.Enable()
		if !waf.IsEnabled() {
			t.Fatal("WAF应该被启用")
		}
	})
}

func TestWAF_IPBlacklist(t *testing.T) {
	waf := NewWAF()

	t.Run("添加黑名单", func(t *testing.T) {
		waf.AddToBlacklist("192.168.1.100")
		blacklist := waf.GetBlacklist()
		if len(blacklist) != 1 {
			t.Fatalf("期望1个黑名单IP，实际: %d", len(blacklist))
		}
	})

	t.Run("黑名单IP被阻止", func(t *testing.T) {
		result := waf.Check("192.168.1.100", "/test", "GET", nil)
		if !result.Blocked {
			t.Fatal("黑名单IP应该被阻止")
		}
	})

	t.Run("移除黑名单", func(t *testing.T) {
		waf.RemoveFromBlacklist("192.168.1.100")
		result := waf.Check("192.168.1.100", "/test", "GET", nil)
		if result.Blocked {
			t.Fatal("移除后不应该被阻止")
		}
	})
}

func TestWAF_IPWhitelist(t *testing.T) {
	waf := NewWAF()

	waf.AddToWhitelist("10.0.0.1")
	waf.AddToBlacklist("10.0.0.1")

	t.Run("白名单优先", func(t *testing.T) {
		result := waf.Check("10.0.0.1", "/test", "GET", nil)
		if result.Blocked {
			t.Fatal("白名单IP不应该被阻止")
		}
	})
}

func TestWAF_SQLInjection(t *testing.T) {
	waf := NewWAF()

	tests := []struct {
		name    string
		path    string
		blocked bool
	}{
		{"正常路径", "/api/users", false},
		{"SQL注入-UNION", "/api/users?id=1 union select * from users", true},
		{"SQL注入-OR", "/api/users?id=1 or 1=1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := waf.Check("127.0.0.1", tt.path, "GET", nil)
			if result.Blocked != tt.blocked {
				t.Errorf("期望 blocked=%v，实际 %v", tt.blocked, result.Blocked)
			}
		})
	}
}

func TestWAF_XSS(t *testing.T) {
	waf := NewWAF()

	tests := []struct {
		name    string
		path    string
		blocked bool
	}{
		{"正常路径", "/api/page", false},
		{"XSS-script标签", "/api/page?q=<script>alert(1)</script>", true},
		{"XSS-javascript", "/api/page?q=javascript:alert(1)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := waf.Check("127.0.0.1", tt.path, "GET", nil)
			if result.Blocked != tt.blocked {
				t.Errorf("期望 blocked=%v，实际 %v", tt.blocked, result.Blocked)
			}
		})
	}
}

func TestWAF_PathTraversal(t *testing.T) {
	waf := NewWAF()

	tests := []struct {
		name    string
		path    string
		blocked bool
	}{
		{"正常路径", "/api/files", false},
		{"路径遍历-../", "/api/files/../../../etc/passwd", true},
		{"路径遍历-etc", "/etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := waf.Check("127.0.0.1", tt.path, "GET", nil)
			if result.Blocked != tt.blocked {
				t.Errorf("期望 blocked=%v，实际 %v", tt.blocked, result.Blocked)
			}
		})
	}
}

func TestWAF_CustomRule(t *testing.T) {
	waf := NewWAF()

	rule := &WAFRule{
		ID:       "custom1",
		Name:     "自定义规则",
		Type:     WAFRuleRateLimit,
		Action:   WAFActionBlock,
		Pattern:  `(?i)badbot`,
		Enabled:  true,
		Priority: 50,
	}
	waf.AddRule(rule)

	t.Run("自定义规则触发", func(t *testing.T) {
		result := waf.Check("127.0.0.1", "/api/test", "GET", nil)
		// User-Agent不在路径中，不会触发
		if result.Blocked {
			t.Fatal("路径中没有badbot，不应该被阻止")
		}
	})
}

func TestWAF_Stats(t *testing.T) {
	waf := NewWAF()

	t.Run("初始统计", func(t *testing.T) {
		stats := waf.GetStats()
		if stats.TotalRequests != 0 {
			t.Fatal("初始请求数应为0")
		}
	})

	t.Run("记录统计", func(t *testing.T) {
		waf.Check("127.0.0.1", "/test", "GET", nil)
		waf.Check("192.168.1.1", "/etc/passwd", "GET", nil)

		stats := waf.GetStats()
		if stats.TotalRequests != 2 {
			t.Fatalf("期望2个请求，实际: %d", stats.TotalRequests)
		}
		if stats.Allowed != 1 {
			t.Fatalf("期望1个允许，实际: %d", stats.Allowed)
		}
	})

	t.Run("重置统计", func(t *testing.T) {
		waf.ResetStats()
		stats := waf.GetStats()
		if stats.TotalRequests != 0 {
			t.Fatal("重置后请求数应为0")
		}
	})
}

func TestWAF_CSRFProtector(t *testing.T) {
	protector := NewCSRFProtector()

	t.Run("生成令牌", func(t *testing.T) {
		token := protector.GenerateToken("session1")
		if token == "" {
			t.Fatal("令牌不应为空")
		}
	})

	t.Run("验证令牌", func(t *testing.T) {
		token := protector.GenerateToken("session1")
		if !protector.ValidateToken(token) {
			t.Fatal("令牌应该有效")
		}
	})

	t.Run("无效令牌", func(t *testing.T) {
		if protector.ValidateToken("invalid-token") {
			t.Fatal("无效令牌应该验证失败")
		}
	})
}

// ========== OAuth 测试 ==========

func TestOAuthServer_ClientManagement(t *testing.T) {
	server := NewOAuthServer()

	t.Run("注册客户端", func(t *testing.T) {
		client := &OAuthClient{
			ID:           "client1",
			Secret:       "secret1",
			Name:         "测试客户端",
			RedirectURIs: []string{"http://localhost:8080/callback"},
			GrantTypes:   []GrantType{GrantTypeAuthorizationCode, GrantTypeClientCredentials},
			Scopes:       []string{"read", "write"},
		}
		if err := server.RegisterClient(client); err != nil {
			t.Fatalf("注册客户端失败: %v", err)
		}
	})

	t.Run("重复注册", func(t *testing.T) {
		client := &OAuthClient{ID: "client1"}
		if err := server.RegisterClient(client); err == nil {
			t.Fatal("重复注册应该报错")
		}
	})

	t.Run("获取客户端", func(t *testing.T) {
		client, err := server.GetClient("client1")
		if err != nil {
			t.Fatalf("获取客户端失败: %v", err)
		}
		if client.Name != "测试客户端" {
			t.Fatalf("期望测试客户端，实际: %s", client.Name)
		}
	})

	t.Run("列出客户端", func(t *testing.T) {
		clients := server.ListClients()
		if len(clients) != 1 {
			t.Fatalf("期望1个客户端，实际: %d", len(clients))
		}
	})

	t.Run("删除客户端", func(t *testing.T) {
		if err := server.DeleteClient("client1"); err != nil {
			t.Fatalf("删除客户端失败: %v", err)
		}
	})
}

func TestOAuthServer_AuthorizationCode(t *testing.T) {
	server := NewOAuthServer()

	// 注册客户端
	client := &OAuthClient{
		ID:           "client1",
		Secret:       "secret1",
		RedirectURIs: []string{"http://localhost:8080/callback"},
		GrantTypes:   []GrantType{GrantTypeAuthorizationCode},
		Scopes:       []string{"read"},
	}
	server.RegisterClient(client)

	t.Run("创建授权码", func(t *testing.T) {
		code, err := server.CreateAuthorizationCode("client1", "user1", "http://localhost:8080/callback", []string{"read"})
		if err != nil {
			t.Fatalf("创建授权码失败: %v", err)
		}
		if code.Code == "" {
			t.Fatal("授权码不应为空")
		}
	})

	t.Run("验证授权码", func(t *testing.T) {
		code, _ := server.CreateAuthorizationCode("client1", "user1", "http://localhost:8080/callback", []string{"read"})
		validCode, err := server.ValidateAuthorizationCode(code.Code, "client1", "http://localhost:8080/callback")
		if err != nil {
			t.Fatalf("验证授权码失败: %v", err)
		}
		if validCode.UserID != "user1" {
			t.Fatalf("期望user1，实际: %s", validCode.UserID)
		}
	})

	t.Run("授权码已使用", func(t *testing.T) {
		code, _ := server.CreateAuthorizationCode("client1", "user1", "http://localhost:8080/callback", []string{"read"})
		server.ValidateAuthorizationCode(code.Code, "client1", "http://localhost:8080/callback")
		_, err := server.ValidateAuthorizationCode(code.Code, "client1", "http://localhost:8080/callback")
		if err == nil {
			t.Fatal("已使用的授权码应该报错")
		}
	})
}

func TestOAuthServer_AccessToken(t *testing.T) {
	server := NewOAuthServer()

	client := &OAuthClient{
		ID:             "client1",
		Secret:         "secret1",
		AccessTokenTTL: time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}
	server.RegisterClient(client)

	t.Run("签发令牌", func(t *testing.T) {
		resp, err := server.IssueAccessToken("client1", "user1", []string{"read"})
		if err != nil {
			t.Fatalf("签发令牌失败: %v", err)
		}
		if resp.AccessToken == "" {
			t.Fatal("访问令牌不应为空")
		}
		if resp.RefreshToken == "" {
			t.Fatal("刷新令牌不应为空")
		}
	})

	t.Run("验证令牌", func(t *testing.T) {
		resp, _ := server.IssueAccessToken("client1", "user1", []string{"read"})
		token, err := server.ValidateAccessToken(resp.AccessToken)
		if err != nil {
			t.Fatalf("验证令牌失败: %v", err)
		}
		if token.ClientID != "client1" {
			t.Fatalf("期望client1，实际: %s", token.ClientID)
		}
	})

	t.Run("刷新令牌", func(t *testing.T) {
		resp, _ := server.IssueAccessToken("client1", "user1", []string{"read"})
		newResp, err := server.RefreshAccessToken(resp.RefreshToken, "client1")
		if err != nil {
			t.Fatalf("刷新令牌失败: %v", err)
		}
		if newResp.AccessToken == resp.AccessToken {
			t.Fatal("新令牌应该不同于旧令牌")
		}
	})

	t.Run("撤销令牌", func(t *testing.T) {
		resp, _ := server.IssueAccessToken("client1", "user1", []string{"read"})
		server.RevokeToken(resp.AccessToken)
		_, err := server.ValidateAccessToken(resp.AccessToken)
		if err == nil {
			t.Fatal("撤销的令牌应该无效")
		}
	})
}

func TestOAuthServer_ClientCredentialsGrant(t *testing.T) {
	server := NewOAuthServer()

	client := &OAuthClient{
		ID:         "client1",
		Secret:     "secret1",
		GrantTypes: []GrantType{GrantTypeClientCredentials},
		Scopes:     []string{"read"},
	}
	server.RegisterClient(client)

	t.Run("客户端凭证模式", func(t *testing.T) {
		req := &TokenRequest{
			GrantType:    GrantTypeClientCredentials,
			ClientID:     "client1",
			ClientSecret: "secret1",
		}
		resp, err := server.HandleTokenRequest(req)
		if err != nil {
			t.Fatalf("处理请求失败: %v", err)
		}
		if resp.AccessToken == "" {
			t.Fatal("访问令牌不应为空")
		}
	})

	t.Run("错误的凭证", func(t *testing.T) {
		req := &TokenRequest{
			GrantType:    GrantTypeClientCredentials,
			ClientID:     "client1",
			ClientSecret: "wrong-secret",
		}
		_, err := server.HandleTokenRequest(req)
		if err == nil {
			t.Fatal("错误的凭证应该报错")
		}
	})
}

func TestOAuthServer_PasswordGrant(t *testing.T) {
	server := NewOAuthServer()

	// 注册客户端
	client := &OAuthClient{
		ID:         "client1",
		Secret:     "secret1",
		GrantTypes: []GrantType{GrantTypePassword},
		Scopes:     []string{"read"},
	}
	server.RegisterClient(client)

	// 注册用户
	user := &OAuthUser{
		ID:       "user1",
		Username: "testuser",
		Password: "testpass",
	}
	server.RegisterUser(user)

	t.Run("密码模式", func(t *testing.T) {
		req := &TokenRequest{
			GrantType:    GrantTypePassword,
			ClientID:     "client1",
			ClientSecret: "secret1",
			Username:     "testuser",
			Password:     "testpass",
		}
		resp, err := server.HandleTokenRequest(req)
		if err != nil {
			t.Fatalf("处理请求失败: %v", err)
		}
		if resp.AccessToken == "" {
			t.Fatal("访问令牌不应为空")
		}
	})

	t.Run("错误密码", func(t *testing.T) {
		req := &TokenRequest{
			GrantType:    GrantTypePassword,
			ClientID:     "client1",
			ClientSecret: "secret1",
			Username:     "testuser",
			Password:     "wrongpass",
		}
		_, err := server.HandleTokenRequest(req)
		if err == nil {
			t.Fatal("错误密码应该报错")
		}
	})
}

func TestOAuthServer_Stats(t *testing.T) {
	server := NewOAuthServer()

	client := &OAuthClient{ID: "client1", Secret: "s1"}
	server.RegisterClient(client)
	server.RegisterUser(&OAuthUser{ID: "user1", Username: "test"})
	server.IssueAccessToken("client1", "user1", []string{"read"})

	stats := server.GetStats()
	if stats["clients"] != 1 {
		t.Fatalf("期望1个客户端，实际: %d", stats["clients"])
	}
	if stats["users"] != 1 {
		t.Fatalf("期望1个用户，实际: %d", stats["users"])
	}
	if stats["access_tokens"] != 1 {
		t.Fatalf("期望1个访问令牌，实际: %d", stats["access_tokens"])
	}
}

// ========== 辅助函数测试 ==========

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"精确匹配", "/api/test", "/api/test", true},
		{"通配符", "/api/*", "/api/users", true},
		{"不匹配", "/api/test", "/other/path", false},
		{"全匹配", "*", "/any/path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchPath(tt.pattern, tt.path); got != tt.want {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestGenerateToken(t *testing.T) {
	t1 := generateToken(32)
	t2 := generateToken(32)

	if t1 == "" {
		t.Fatal("令牌不应为空")
	}
	if t1 == t2 {
		t.Fatal("两个令牌应该不同")
	}
}

func TestHashPassword(t *testing.T) {
	hash1 := hashPassword("password")
	hash2 := hashPassword("password")
	hash3 := hashPassword("different")

	if hash1 != hash2 {
		t.Fatal("相同密码的哈希应该相同")
	}
	if hash1 == hash3 {
		t.Fatal("不同密码的哈希应该不同")
	}
}

// ========== 测试辅助类型 ==========

// testPlugin 测试插件.
type testPlugin struct {
	name string
	desc string
}

func (p *testPlugin) Name() string        { return p.name }
func (p *testPlugin) Description() string  { return p.desc }
func (p *testPlugin) Execute(ctx *PluginContext) error { return nil }
func (p *testPlugin) OnRequest(req *http.Request) error  { return nil }
func (p *testPlugin) OnResponse(resp *http.Response) error { return nil }
