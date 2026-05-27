package reverseproxy

import (
	"testing"
)

func TestNewReverseProxyManager(t *testing.T) {
	manager := NewReverseProxyManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.config == nil {
		t.Fatal("Expected default config to be set")
	}

	if manager.config.MaxProxies != 100 {
		t.Errorf("Expected max proxies to be 100, got %d", manager.config.MaxProxies)
	}
}

func TestAddProxy(t *testing.T) {
	manager := NewReverseProxyManager(nil)

	rule := &ProxyRule{
		Name:      "test-proxy",
		Domain:    "example.com",
		TargetURL: "http://localhost:8080",
		Path:      "/",
	}

	err := manager.AddProxy(rule)
	if err != nil {
		t.Fatalf("Failed to add proxy: %v", err)
	}

	if rule.ID == "" {
		t.Error("Expected ID to be generated")
	}

	if !rule.Enabled {
		t.Error("Expected proxy to be enabled")
	}
}

func TestAddProxyValidation(t *testing.T) {
	manager := NewReverseProxyManager(nil)

	// 测试缺少域名
	rule1 := &ProxyRule{
		TargetURL: "http://localhost:8080",
	}
	err := manager.AddProxy(rule1)
	if err == nil {
		t.Error("Expected error for missing domain")
	}

	// 测试缺少目标URL
	rule2 := &ProxyRule{
		Domain: "example.com",
	}
	err = manager.AddProxy(rule2)
	if err == nil {
		t.Error("Expected error for missing target URL")
	}

	// 测试无效的目标URL
	rule3 := &ProxyRule{
		Domain:    "example.com",
		TargetURL: "://invalid",
	}
	err = manager.AddProxy(rule3)
	if err == nil {
		t.Error("Expected error for invalid target URL")
	}
}

func TestRemoveProxy(t *testing.T) {
	manager := NewReverseProxyManager(nil)

	// 添加代理
	rule := &ProxyRule{
		Domain:    "example.com",
		TargetURL: "http://localhost:8080",
	}
	manager.AddProxy(rule)

	// 移除代理
	err := manager.RemoveProxy(rule.ID)
	if err != nil {
		t.Fatalf("Failed to remove proxy: %v", err)
	}

	// 验证已移除
	_, err = manager.GetProxy(rule.ID)
	if err == nil {
		t.Error("Expected error for removed proxy")
	}

	// 测试移除不存在的代理
	err = manager.RemoveProxy("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent proxy")
	}
}

func TestManagerUpdateProxy(t *testing.T) {
	manager := NewReverseProxyManager(nil)

	// 添加代理
	rule := &ProxyRule{
		Domain:    "example.com",
		TargetURL: "http://localhost:8080",
	}
	manager.AddProxy(rule)

	// 更新代理
	update := &ProxyRule{
		Domain:    "newdomain.com",
		TargetURL: "http://localhost:9090",
	}
	err := manager.UpdateProxy(rule.ID, update)
	if err != nil {
		t.Fatalf("Failed to update proxy: %v", err)
	}

	// 验证更新
	updated, _ := manager.GetProxy(rule.ID)
	if updated.Domain != "newdomain.com" {
		t.Errorf("Expected domain 'newdomain.com', got '%s'", updated.Domain)
	}
}

func TestManagerGetProxy(t *testing.T) {
	manager := NewReverseProxyManager(nil)

	// 添加代理
	rule := &ProxyRule{
		Domain:    "example.com",
		TargetURL: "http://localhost:8080",
	}
	manager.AddProxy(rule)

	// 获取代理
	proxy, err := manager.GetProxy(rule.ID)
	if err != nil {
		t.Fatalf("Failed to get proxy: %v", err)
	}

	if proxy.Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", proxy.Domain)
	}

	// 测试获取不存在的代理
	_, err = manager.GetProxy("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent proxy")
	}
}

func TestManagerListProxies(t *testing.T) {
	manager := NewReverseProxyManager(nil)

	// 添加一些代理
	manager.AddProxy(&ProxyRule{
		Domain:    "example1.com",
		TargetURL: "http://localhost:8080",
	})
	manager.AddProxy(&ProxyRule{
		Domain:    "example2.com",
		TargetURL: "http://localhost:8081",
	})

	proxies := manager.ListProxies()
	if len(proxies) != 2 {
		t.Errorf("Expected 2 proxies, got %d", len(proxies))
	}
}

func TestEnableDisableProxy(t *testing.T) {
	manager := NewReverseProxyManager(nil)

	// 添加代理
	rule := &ProxyRule{
		Domain:    "example.com",
		TargetURL: "http://localhost:8080",
	}
	manager.AddProxy(rule)

	// 禁用代理
	err := manager.DisableProxy(rule.ID)
	if err != nil {
		t.Fatalf("Failed to disable proxy: %v", err)
	}

	proxy, _ := manager.GetProxy(rule.ID)
	if proxy.Enabled {
		t.Error("Expected proxy to be disabled")
	}

	// 启用代理
	err = manager.EnableProxy(rule.ID)
	if err != nil {
		t.Fatalf("Failed to enable proxy: %v", err)
	}

	proxy, _ = manager.GetProxy(rule.ID)
	if !proxy.Enabled {
		t.Error("Expected proxy to be enabled")
	}
}

func TestManagerGetStats(t *testing.T) {
	manager := NewReverseProxyManager(nil)

	// 添加一些代理
	manager.AddProxy(&ProxyRule{
		Domain:    "example1.com",
		TargetURL: "http://localhost:8080",
		Enabled:   true,
	})
	manager.AddProxy(&ProxyRule{
		Domain:    "example2.com",
		TargetURL: "http://localhost:8081",
	})
	proxies := manager.ListProxies()
	for _, p := range proxies {
		if p.Domain == "example2.com" {
			manager.DisableProxy(p.ID)
		}
	}

	stats := manager.GetStats()
	if stats.TotalProxies != 2 {
		t.Errorf("Expected 2 total proxies, got %d", stats.TotalProxies)
	}

	if stats.ActiveProxies != 1 {
		t.Errorf("Expected 1 active proxy, got %d", stats.ActiveProxies)
	}
}

func TestFindProxyByDomain(t *testing.T) {
	manager := NewReverseProxyManager(nil)

	// 添加代理
	rule := &ProxyRule{
		Domain:    "example.com",
		TargetURL: "http://localhost:8080",
		Enabled:   true,
	}
	manager.AddProxy(rule)

	// 查找代理
	proxy, err := manager.FindProxyByDomain("example.com")
	if err != nil {
		t.Fatalf("Failed to find proxy: %v", err)
	}

	if proxy.Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", proxy.Domain)
	}

	// 测试查找不存在的域名
	_, err = manager.FindProxyByDomain("nonexistent.com")
	if err == nil {
		t.Error("Expected error for nonexistent domain")
	}
}

func TestUpdateRequestCount(t *testing.T) {
	manager := NewReverseProxyManager(nil)

	// 添加代理
	rule := &ProxyRule{
		Domain:    "example.com",
		TargetURL: "http://localhost:8080",
	}
	manager.AddProxy(rule)

	// 更新请求计数
	manager.UpdateRequestCount(rule.ID)
	manager.UpdateRequestCount(rule.ID)

	proxy, _ := manager.GetProxy(rule.ID)
	if proxy.RequestCount != 2 {
		t.Errorf("Expected request count 2, got %d", proxy.RequestCount)
	}
}
