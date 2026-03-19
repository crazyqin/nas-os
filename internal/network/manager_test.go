package network

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestNewManagerBasic 测试创建管理器基本功能
func TestNewManagerBasic(t *testing.T) {
	mgr := NewManager("/tmp/test-network-config")
	if mgr == nil {
		t.Fatal("管理器创建失败")
	}

	if mgr.configPath != "/tmp/test-network-config" {
		t.Errorf("配置路径错误: %s", mgr.configPath)
	}

	if mgr.ddnsConfigs == nil {
		t.Error("ddnsConfigs 未初始化")
	}

	if mgr.portForwards == nil {
		t.Error("portForwards 未初始化")
	}

	if mgr.firewallRules == nil {
		t.Error("firewallRules 未初始化")
	}
}

// TestDDNSConfig 测试 DDNS 配置
func TestDDNSConfig(t *testing.T) {
	mgr := NewManager("/tmp/test-network-config")

	// 测试添加 DDNS 配置
	config := DDNSConfig{
		Provider: "duckdns",
		Domain:   "test.duckdns.org",
		Token:    "test-token",
		Enabled:  true,
		Interval: 300,
	}

	err := mgr.AddDDNS(config)
	if err != nil {
		t.Fatalf("添加 DDNS 配置失败: %v", err)
	}

	// 测试获取 DDNS 配置
	cfg, err := mgr.GetDDNS("test.duckdns.org")
	if err != nil {
		t.Fatalf("获取 DDNS 配置失败: %v", err)
	}

	if cfg.Provider != "duckdns" {
		t.Errorf("Provider 不匹配: %s", cfg.Provider)
	}

	// 测试列出 DDNS 配置
	configs := mgr.ListDDNS()
	if len(configs) != 1 {
		t.Errorf("DDNS 配置数量错误: %d", len(configs))
	}

	// 测试更新 DDNS 配置
	updatedConfig := DDNSConfig{
		Provider: "duckdns",
		Domain:   "test.duckdns.org",
		Token:    "updated-token",
		Enabled:  true,
		Interval: 600,
	}

	err = mgr.UpdateDDNS("test.duckdns.org", updatedConfig)
	if err != nil {
		t.Fatalf("更新 DDNS 配置失败: %v", err)
	}

	cfg, _ = mgr.GetDDNS("test.duckdns.org")
	if cfg.Interval != 600 {
		t.Errorf("Interval 更新失败: %d", cfg.Interval)
	}

	// 测试启用/禁用 DDNS
	err = mgr.EnableDDNS("test.duckdns.org", false)
	if err != nil {
		t.Fatalf("禁用 DDNS 失败: %v", err)
	}

	cfg, _ = mgr.GetDDNS("test.duckdns.org")
	if cfg.Enabled {
		t.Error("DDNS 应该被禁用")
	}

	// 测试删除 DDNS 配置
	err = mgr.DeleteDDNS("test.duckdns.org")
	if err != nil {
		t.Fatalf("删除 DDNS 配置失败: %v", err)
	}

	configs = mgr.ListDDNS()
	if len(configs) != 0 {
		t.Errorf("DDNS 配置应该为空: %d", len(configs))
	}
}

// TestDDNSValidation 测试 DDNS 验证
func TestDDNSValidation(t *testing.T) {
	mgr := NewManager("/tmp/test-network-config")

	// 测试空域名
	err := mgr.AddDDNS(DDNSConfig{Provider: "duckdns"})
	if err == nil {
		t.Error("空域名应该返回错误")
	}

	// 测试空服务商
	err = mgr.AddDDNS(DDNSConfig{Domain: "test.duckdns.org"})
	if err == nil {
		t.Error("空服务商应该返回错误")
	}

	// 测试获取不存在的配置
	_, err = mgr.GetDDNS("not-exist.duckdns.org")
	if err == nil {
		t.Error("获取不存在的配置应该返回错误")
	}

	// 测试更新不存在的配置
	err = mgr.UpdateDDNS("not-exist.duckdns.org", DDNSConfig{})
	if err == nil {
		t.Error("更新不存在的配置应该返回错误")
	}

	// 测试删除不存在的配置
	err = mgr.DeleteDDNS("not-exist.duckdns.org")
	if err == nil {
		t.Error("删除不存在的配置应该返回错误")
	}
}

// TestPortForward 测试端口转发
func TestPortForward(t *testing.T) {
	mgr := NewManager("/tmp/test-network-config")

	// 测试添加端口转发规则
	rule := PortForward{
		Name:         "web-server",
		ExternalPort: 8080,
		Protocol:     "tcp",
		InternalIP:   "192.168.1.100",
		InternalPort: 80,
		Enabled:      false, // 不启用以避免实际执行 iptables
		Comment:      "Web Server",
	}

	err := mgr.AddPortForward(rule)
	if err != nil {
		t.Fatalf("添加端口转发规则失败: %v", err)
	}

	// 测试获取端口转发规则
	r, err := mgr.GetPortForward("web-server")
	if err != nil {
		t.Fatalf("获取端口转发规则失败: %v", err)
	}

	if r.ExternalPort != 8080 {
		t.Errorf("外部端口不匹配: %d", r.ExternalPort)
	}

	// 测试列出端口转发规则
	rules := mgr.ListPortForwards()
	if len(rules) != 1 {
		t.Errorf("端口转发规则数量错误: %d", len(rules))
	}

	// 测试更新端口转发规则
	updatedRule := PortForward{
		Name:         "web-server",
		ExternalPort: 8081,
		Protocol:     "tcp",
		InternalIP:   "192.168.1.101",
		InternalPort: 80,
		Enabled:      false,
	}

	err = mgr.UpdatePortForward("web-server", updatedRule)
	if err != nil {
		t.Fatalf("更新端口转发规则失败: %v", err)
	}

	r, _ = mgr.GetPortForward("web-server")
	if r.ExternalPort != 8081 {
		t.Errorf("外部端口更新失败: %d", r.ExternalPort)
	}

	// 测试删除端口转发规则
	err = mgr.DeletePortForward("web-server")
	if err != nil {
		t.Fatalf("删除端口转发规则失败: %v", err)
	}

	rules = mgr.ListPortForwards()
	if len(rules) != 0 {
		t.Errorf("端口转发规则应该为空: %d", len(rules))
	}
}

// TestPortForwardValidation 测试端口转发验证
func TestPortForwardValidation(t *testing.T) {
	mgr := NewManager("/tmp/test-network-config")

	// 测试空名称
	err := mgr.AddPortForward(PortForward{ExternalPort: 80, InternalIP: "192.168.1.1", InternalPort: 80})
	if err == nil {
		t.Error("空名称应该返回错误")
	}

	// 测试无效外部端口
	err = mgr.AddPortForward(PortForward{Name: "test", ExternalPort: 70000, InternalIP: "192.168.1.1", InternalPort: 80})
	if err == nil {
		t.Error("无效外部端口应该返回错误")
	}

	// 测试无效内部端口
	err = mgr.AddPortForward(PortForward{Name: "test", ExternalPort: 80, InternalIP: "192.168.1.1", InternalPort: 70000})
	if err == nil {
		t.Error("无效内部端口应该返回错误")
	}

	// 测试空内部 IP
	err = mgr.AddPortForward(PortForward{Name: "test", ExternalPort: 80, InternalIP: "", InternalPort: 80})
	if err == nil {
		t.Error("空内部 IP 应该返回错误")
	}

	// 测试无效协议
	err = mgr.AddPortForward(PortForward{Name: "test", ExternalPort: 80, Protocol: "invalid", InternalIP: "192.168.1.1", InternalPort: 80})
	if err == nil {
		t.Error("无效协议应该返回错误")
	}

	// 测试重复名称
	mgr.AddPortForward(PortForward{Name: "dup-test", ExternalPort: 80, InternalIP: "192.168.1.1", InternalPort: 80, Enabled: false})
	err = mgr.AddPortForward(PortForward{Name: "dup-test", ExternalPort: 81, InternalIP: "192.168.1.2", InternalPort: 81, Enabled: false})
	if err == nil {
		t.Error("重复名称应该返回错误")
	}
}

// TestFirewallRule 测试防火墙规则
func TestFirewallRule(t *testing.T) {
	mgr := NewManager("/tmp/test-network-config")

	// 测试添加防火墙规则
	rule := FirewallRule{
		Name:      "allow-ssh",
		Action:    "accept",
		Direction: "in",
		Protocol:  "tcp",
		DestPort:  "22",
		Enabled:   false, // 不启用以避免实际执行 iptables
		Comment:   "Allow SSH",
	}

	err := mgr.AddFirewallRule(rule)
	if err != nil {
		t.Fatalf("添加防火墙规则失败: %v", err)
	}

	// 测试获取防火墙规则
	r, err := mgr.GetFirewallRule("allow-ssh")
	if err != nil {
		t.Fatalf("获取防火墙规则失败: %v", err)
	}

	if r.Protocol != "tcp" {
		t.Errorf("协议不匹配: %s", r.Protocol)
	}

	// 测试列出防火墙规则
	rules := mgr.ListFirewallRules()
	if len(rules) != 1 {
		t.Errorf("防火墙规则数量错误: %d", len(rules))
	}

	// 测试更新防火墙规则
	updatedRule := FirewallRule{
		Name:      "allow-ssh",
		Action:    "accept",
		Direction: "in",
		Protocol:  "tcp",
		DestPort:  "2222",
		Enabled:   false,
	}

	err = mgr.UpdateFirewallRule("allow-ssh", updatedRule)
	if err != nil {
		t.Fatalf("更新防火墙规则失败: %v", err)
	}

	r, _ = mgr.GetFirewallRule("allow-ssh")
	if r.DestPort != "2222" {
		t.Errorf("端口更新失败: %s", r.DestPort)
	}

	// 测试删除防火墙规则
	err = mgr.DeleteFirewallRule("allow-ssh")
	if err != nil {
		t.Fatalf("删除防火墙规则失败: %v", err)
	}

	rules = mgr.ListFirewallRules()
	if len(rules) != 0 {
		t.Errorf("防火墙规则应该为空: %d", len(rules))
	}
}

// TestFirewallValidation 测试防火墙验证
func TestFirewallValidation(t *testing.T) {
	mgr := NewManager("/tmp/test-network-config")

	// 测试空名称
	err := mgr.AddFirewallRule(FirewallRule{Action: "accept", Direction: "in"})
	if err == nil {
		t.Error("空名称应该返回错误")
	}

	// 测试无效协议
	err = mgr.AddFirewallRule(FirewallRule{Name: "test", Protocol: "invalid", Action: "accept", Direction: "in"})
	if err == nil {
		t.Error("无效协议应该返回错误")
	}

	// 测试无效动作
	err = mgr.AddFirewallRule(FirewallRule{Name: "test", Action: "invalid", Direction: "in"})
	if err == nil {
		t.Error("无效动作应该返回错误")
	}

	// 测试无效方向
	err = mgr.AddFirewallRule(FirewallRule{Name: "test", Action: "accept", Direction: "invalid"})
	if err == nil {
		t.Error("无效方向应该返回错误")
	}
}

// TestHandlers 测试 HTTP 处理器
func TestHandlers(t *testing.T) {
	mgr := NewManager("/tmp/test-network-config")
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	// 测试列出 DDNS
	t.Run("ListDDNS", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/network/ddns", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码错误: %d", w.Code)
		}
	})

	// 测试添加 DDNS
	t.Run("AddDDNS", func(t *testing.T) {
		body := `{"provider":"duckdns","domain":"handler-test.duckdns.org","token":"test-token"}`
		req, _ := http.NewRequest("POST", "/api/v1/network/ddns", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码错误: %d, body: %s", w.Code, w.Body.String())
		}
	})

	// 测试获取 DDNS
	t.Run("GetDDNS", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/network/ddns/handler-test.duckdns.org", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码错误: %d", w.Code)
		}
	})

	// 测试列出端口转发
	t.Run("ListPortForwards", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/network/portforwards", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码错误: %d", w.Code)
		}
	})

	// 测试添加端口转发
	t.Run("AddPortForward", func(t *testing.T) {
		body := `{"name":"test-forward","externalPort":9090,"protocol":"tcp","internalIp":"192.168.1.50","internalPort":9090,"enabled":false}`
		req, _ := http.NewRequest("POST", "/api/v1/network/portforwards", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码错误: %d, body: %s", w.Code, w.Body.String())
		}
	})

	// 测试列出防火墙规则
	t.Run("ListFirewallRules", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/network/firewall/rules", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码错误: %d", w.Code)
		}
	})

	// 测试添加防火墙规则
	t.Run("AddFirewallRule", func(t *testing.T) {
		body := `{"name":"test-rule","action":"accept","direction":"in","protocol":"tcp","destPort":"443","enabled":false}`
		req, _ := http.NewRequest("POST", "/api/v1/network/firewall/rules", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码错误: %d, body: %s", w.Code, w.Body.String())
		}
	})
}

// TestDuckDNSProvider 测试 DuckDNS Provider
func TestDuckDNSProvider(t *testing.T) {
	provider := &DuckDNSProvider{Token: "test-token"}

	// 注意：这个测试会实际发起 HTTP 请求
	// 在生产环境中应该 mock HTTP 客户端
	t.Skip("跳过需要网络的测试")

	err := provider.Update("test.duckdns.org", "1.2.3.4")
	// DuckDNS 会返回错误（无效 token），但我们主要测试流程
	if err == nil {
		t.Log("DuckDNS 更新成功")
	}
}

// TestDefaultValues 测试默认值设置
func TestDefaultValues(t *testing.T) {
	mgr := NewManager("/tmp/test-network-config")

	// 测试 DDNS 默认间隔
	config := DDNSConfig{
		Provider: "duckdns",
		Domain:   "default-test.duckdns.org",
		Token:    "test-token",
	}
	mgr.AddDDNS(config)

	cfg, _ := mgr.GetDDNS("default-test.duckdns.org")
	if cfg.Interval != 300 {
		t.Errorf("默认间隔应该是 300 秒: %d", cfg.Interval)
	}

	// 测试端口转发默认协议
	rule := PortForward{
		Name:         "default-proto",
		ExternalPort: 8080,
		InternalIP:   "192.168.1.1",
		InternalPort: 80,
		Enabled:      false,
	}
	mgr.AddPortForward(rule)

	r, _ := mgr.GetPortForward("default-proto")
	if r.Protocol != "tcp" {
		t.Errorf("默认协议应该是 tcp: %s", r.Protocol)
	}

	// 测试防火墙默认值
	fwRule := FirewallRule{
		Name:    "default-values",
		Enabled: false,
	}
	mgr.AddFirewallRule(fwRule)

	fw, _ := mgr.GetFirewallRule("default-values")
	if fw.Protocol != "all" {
		t.Errorf("默认协议应该是 all: %s", fw.Protocol)
	}
	if fw.Action != "accept" {
		t.Errorf("默认动作应该是 accept: %s", fw.Action)
	}
	if fw.Direction != "in" {
		t.Errorf("默认方向应该是 in: %s", fw.Direction)
	}
}

// BenchmarkListDDNS 测试列出 DDNS 性能
func BenchmarkListDDNS(b *testing.B) {
	mgr := NewManager("/tmp/test-network-config")

	// 添加 100 个配置
	for i := 0; i < 100; i++ {
		mgr.AddDDNS(DDNSConfig{
			Provider: "duckdns",
			Domain:   "bench-test.duckdns.org",
			Token:    "test-token",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.ListDDNS()
	}
}

// BenchmarkAddPortForward 测试添加端口转发性能
func BenchmarkAddPortForward(b *testing.B) {
	mgr := NewManager("/tmp/test-network-config")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.AddPortForward(PortForward{
			Name:         "bench-test",
			ExternalPort: 8080 + i,
			InternalIP:   "192.168.1.1",
			InternalPort: 80,
			Enabled:      false,
		})
	}
}
