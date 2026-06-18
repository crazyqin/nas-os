package ipv6net

import (
	"testing"
)

func TestNewIPv6Manager(t *testing.T) {
	config := IPv6Config{
		Enabled:    true,
		AutoConfig: true,
		DHCPv6:     false,
		PrivacyExt: true,
		LinkLocal:  true,
		DNS:        []string{"2001:4860:4860::8888", "2001:4860:4860::8844"},
	}

	manager := NewIPv6Manager(config)
	if manager == nil {
		t.Fatal("NewIPv6Manager returned nil")
	}

	if !manager.IsIPv6Enabled() {
		t.Error("Expected IPv6 to be enabled")
	}
}

func TestIPv6Manager_Init(t *testing.T) {
	config := IPv6Config{
		Enabled:    true,
		AutoConfig: true,
		PrivacyExt: true,
	}

	manager := NewIPv6Manager(config)

	err := manager.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 检查是否检测到接口
	interfaces := manager.GetAllInterfaces()
	t.Logf("Detected %d interfaces", len(interfaces))
}

func TestIPv6Manager_GetStatus(t *testing.T) {
	config := IPv6Config{
		Enabled:    true,
		AutoConfig: true,
		DHCPv6:     false,
		PrivacyExt: true,
	}

	manager := NewIPv6Manager(config)

	status := manager.GetStatus()
	if status["enabled"] != true {
		t.Error("Expected enabled to be true")
	}
}

func TestIPv6Manager_SetDNSConfig(t *testing.T) {
	config := IPv6Config{
		Enabled: true,
	}

	manager := NewIPv6Manager(config)

	dns := []string{"2001:4860:4860::8888"}
	manager.SetDNSConfig(dns)

	configuredDNS := manager.GetDNSConfig()
	if len(configuredDNS) != 1 {
		t.Errorf("Expected 1 DNS server, got %d", len(configuredDNS))
	}
}

func TestIPv6Manager_Disabled(t *testing.T) {
	config := IPv6Config{
		Enabled: false,
	}

	manager := NewIPv6Manager(config)

	if manager.IsIPv6Enabled() {
		t.Error("Expected IPv6 to be disabled")
	}

	// Init应该成功但不执行任何操作
	err := manager.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
}
