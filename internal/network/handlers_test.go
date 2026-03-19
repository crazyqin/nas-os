package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDDNSConfigEnabled(t *testing.T) {
	config := DDNSConfig{
		Enabled: true,
	}

	assert.True(t, config.Enabled)
}

func TestDDNSConfigDisabled(t *testing.T) {
	config := DDNSConfig{
		Enabled: false,
	}

	assert.False(t, config.Enabled)
}

func TestPortForwardEnabled(t *testing.T) {
	pf := PortForward{
		Name:         "Test",
		ExternalPort: 8080,
		Protocol:     "tcp",
		InternalIP:   "192.168.1.1",
		InternalPort: 80,
		Enabled:      true,
	}

	assert.True(t, pf.Enabled)
	assert.Equal(t, 8080, pf.ExternalPort)
}

func TestPortForwardDisabled(t *testing.T) {
	pf := PortForward{
		Enabled: false,
	}

	assert.False(t, pf.Enabled)
}

func TestFirewallRuleEnabled(t *testing.T) {
	rule := FirewallRule{
		Name:      "Test Rule",
		Action:    "accept",
		Direction: "in",
		Protocol:  "tcp",
		DestPort:  "22",
		Enabled:   true,
	}

	assert.True(t, rule.Enabled)
	assert.Equal(t, "accept", rule.Action)
	assert.Equal(t, "in", rule.Direction)
}

func TestFirewallRuleDisabled(t *testing.T) {
	rule := FirewallRule{
		Enabled: false,
	}

	assert.False(t, rule.Enabled)
}

func TestManagerDDNSConfigs(t *testing.T) {
	manager := NewManager("")

	manager.ddnsConfigs["test"] = &DDNSConfig{
		Provider: "cloudflare",
		Domain:   "example.com",
		Enabled:  true,
	}

	assert.Equal(t, 1, len(manager.ddnsConfigs))
	assert.Equal(t, "cloudflare", manager.ddnsConfigs["test"].Provider)
}

func TestManagerPortForwards(t *testing.T) {
	manager := NewManager("")

	manager.portForwards["test"] = &PortForward{
		Name:         "Test",
		ExternalPort: 8080,
		Protocol:     "tcp",
		Enabled:      true,
	}

	assert.Equal(t, 1, len(manager.portForwards))
	assert.Equal(t, 8080, manager.portForwards["test"].ExternalPort)
}

func TestManagerFirewallRules(t *testing.T) {
	manager := NewManager("")

	manager.firewallRules["test"] = &FirewallRule{
		Name:    "Test",
		Action:  "accept",
		Enabled: true,
	}

	assert.Equal(t, 1, len(manager.firewallRules))
	assert.Equal(t, "accept", manager.firewallRules["test"].Action)
}

func TestInterfaceWithEmptyFields(t *testing.T) {
	iface := Interface{
		Name:  "eth0",
		State: "up",
	}

	assert.Equal(t, "eth0", iface.Name)
	assert.Equal(t, "up", iface.State)
	assert.Empty(t, iface.IP)
	assert.Empty(t, iface.Gateway)
}

func TestDDNSConfigWithEmptyFields(t *testing.T) {
	config := DDNSConfig{
		Provider: "cloudflare",
		Enabled:  true,
	}

	assert.Equal(t, "cloudflare", config.Provider)
	assert.Empty(t, config.Domain)
	assert.Empty(t, config.LastIP)
}

func TestPortForwardWithComment(t *testing.T) {
	pf := PortForward{
		Name:         "Test",
		ExternalPort: 443,
		Protocol:     "tcp",
		InternalIP:   "10.0.0.1",
		InternalPort: 443,
		Enabled:      true,
		Comment:      "HTTPS forward to web server",
	}

	assert.Equal(t, "HTTPS forward to web server", pf.Comment)
}

func TestFirewallRuleWithComment(t *testing.T) {
	rule := FirewallRule{
		Name:      "Allow HTTPS",
		Action:    "accept",
		Direction: "in",
		Protocol:  "tcp",
		DestPort:  "443",
		Enabled:   true,
		Comment:   "Allow incoming HTTPS traffic",
	}

	assert.Equal(t, "Allow incoming HTTPS traffic", rule.Comment)
}

func TestStatsWithZeroValues(t *testing.T) {
	stats := Stats{}

	assert.Equal(t, int64(0), stats.TotalRxBytes)
	assert.Equal(t, int64(0), stats.TotalTxBytes)
	assert.Nil(t, stats.Interfaces)
}

func TestInterfaceStatsWithZeroValues(t *testing.T) {
	stats := InterfaceStats{
		Name: "eth0",
	}

	assert.Equal(t, "eth0", stats.Name)
	assert.Equal(t, int64(0), stats.RxBytes)
	assert.Equal(t, int64(0), stats.TxBytes)
}

func TestMultipleDDNSConfigs(t *testing.T) {
	manager := NewManager("")

	manager.ddnsConfigs["primary"] = &DDNSConfig{
		Provider: "cloudflare",
		Domain:   "example.com",
		Enabled:  true,
	}

	manager.ddnsConfigs["secondary"] = &DDNSConfig{
		Provider: "duckdns",
		Domain:   "example.duckdns.org",
		Enabled:  true,
	}

	assert.Equal(t, 2, len(manager.ddnsConfigs))
}

func TestMultiplePortForwards(t *testing.T) {
	manager := NewManager("")

	manager.portForwards["http"] = &PortForward{
		ExternalPort: 80,
		InternalPort: 8080,
		Enabled:      true,
	}

	manager.portForwards["https"] = &PortForward{
		ExternalPort: 443,
		InternalPort: 8443,
		Enabled:      true,
	}

	assert.Equal(t, 2, len(manager.portForwards))
}

func TestMultipleFirewallRules(t *testing.T) {
	manager := NewManager("")

	manager.firewallRules["allow-ssh"] = &FirewallRule{
		Name:    "Allow SSH",
		Action:  "accept",
		DestPort: "22",
		Enabled: true,
	}

	manager.firewallRules["allow-http"] = &FirewallRule{
		Name:    "Allow HTTP",
		Action:  "accept",
		DestPort: "80",
		Enabled: true,
	}

	assert.Equal(t, 2, len(manager.firewallRules))
}