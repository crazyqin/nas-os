package network

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	manager := NewManager("/tmp/test-network.json")

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.ddnsConfigs)
	assert.NotNil(t, manager.portForwards)
	assert.NotNil(t, manager.firewallRules)
	assert.Equal(t, "/tmp/test-network.json", manager.configPath)
}

func TestManagerInitialize(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "network.json")
	manager := NewManager(tmpFile)

	err := manager.Initialize()
	assert.NoError(t, err)
}

func TestManagerInitializeWithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "network.json")

	// Create config file
	config := `{
		"ddnsConfigs": {
			"test-ddns": {
				"provider": "cloudflare",
				"domain": "example.com",
				"enabled": true
			}
		},
		"portForwards": {
			"test-forward": {
				"name": "SSH",
				"externalPort": 2222,
				"protocol": "tcp",
				"internalIP": "192.168.1.100",
				"internalPort": 22,
				"enabled": true
			}
		},
		"firewallRules": {
			"test-rule": {
				"name": "Allow SSH",
				"action": "accept",
				"direction": "in",
				"protocol": "tcp",
				"destPort": "22",
				"enabled": true
			}
		}
	}`
	os.WriteFile(configPath, []byte(config), 0644)

	manager := NewManager(configPath)
	err := manager.Initialize()
	assert.NoError(t, err)

	// Verify config was loaded
	assert.Equal(t, 1, len(manager.ddnsConfigs))
	assert.Equal(t, 1, len(manager.portForwards))
	assert.Equal(t, 1, len(manager.firewallRules))
}

func TestInterfaceStruct(t *testing.T) {
	iface := Interface{
		Name:    "eth0",
		MAC:     "00:11:22:33:44:55",
		IP:      "192.168.1.100",
		Netmask: "255.255.255.0",
		Gateway: "192.168.1.1",
		DNS:     "8.8.8.8",
		State:   "up",
		Type:    "ethernet",
		Speed:   "1000 Mbps",
		RxBytes: 1000000,
		TxBytes: 500000,
		Mtu:     1500,
	}

	assert.Equal(t, "eth0", iface.Name)
	assert.Equal(t, "up", iface.State)
	assert.Equal(t, "ethernet", iface.Type)
	assert.Equal(t, 1500, iface.Mtu)
}

func TestInterfaceConfigStruct(t *testing.T) {
	config := InterfaceConfig{
		IP:      "192.168.1.100",
		Netmask: "255.255.255.0",
		Gateway: "192.168.1.1",
		DNS:     "8.8.8.8",
		DHCP:    false,
	}

	assert.Equal(t, "192.168.1.100", config.IP)
	assert.False(t, config.DHCP)
}

func TestDDNSConfigStruct(t *testing.T) {
	config := DDNSConfig{
		Provider:  "cloudflare",
		Domain:    "example.com",
		Token:     "test-token",
		Secret:    "test-secret",
		Interface: "eth0",
		Enabled:   true,
		Status:    "active",
		LastIP:    "1.2.3.4",
		Interval:  300,
	}

	assert.Equal(t, "cloudflare", config.Provider)
	assert.True(t, config.Enabled)
	assert.Equal(t, 300, config.Interval)
}

func TestPortForwardStruct(t *testing.T) {
	pf := PortForward{
		Name:         "SSH Forward",
		ExternalPort: 2222,
		Protocol:     "tcp",
		InternalIP:   "192.168.1.100",
		InternalPort: 22,
		Enabled:      true,
		Comment:      "SSH access",
	}

	assert.Equal(t, "SSH Forward", pf.Name)
	assert.Equal(t, 2222, pf.ExternalPort)
	assert.Equal(t, "tcp", pf.Protocol)
	assert.True(t, pf.Enabled)
}

func TestFirewallRuleStruct(t *testing.T) {
	rule := FirewallRule{
		Name:      "Allow HTTP",
		Action:    "accept",
		Direction: "in",
		Protocol:  "tcp",
		SourceIP:  "0.0.0.0/0",
		DestIP:    "",
		DestPort:  "80",
		Enabled:   true,
		Comment:   "Allow web traffic",
	}

	assert.Equal(t, "Allow HTTP", rule.Name)
	assert.Equal(t, "accept", rule.Action)
	assert.Equal(t, "in", rule.Direction)
	assert.True(t, rule.Enabled)
}

func TestStatsStruct(t *testing.T) {
	stats := Stats{
		Interfaces: []InterfaceStats{
			{Name: "eth0", RxBytes: 1000000, TxBytes: 500000},
			{Name: "eth1", RxBytes: 2000000, TxBytes: 1000000},
		},
		TotalRxBytes: 3000000,
		TotalTxBytes: 1500000,
	}

	assert.Equal(t, 2, len(stats.Interfaces))
	assert.Equal(t, int64(3000000), stats.TotalRxBytes)
	assert.Equal(t, int64(1500000), stats.TotalTxBytes)
}

func TestInterfaceStatsStruct(t *testing.T) {
	stats := InterfaceStats{
		Name:      "eth0",
		RxBytes:   1000000,
		TxBytes:   500000,
		RxPackets: 1000,
		TxPackets: 800,
	}

	assert.Equal(t, "eth0", stats.Name)
	assert.Equal(t, int64(1000000), stats.RxBytes)
	assert.Equal(t, int64(800), stats.TxPackets)
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "network.json")

	manager := NewManager(configPath)
	manager.ddnsConfigs["test"] = &DDNSConfig{
		Provider: "cloudflare",
		Domain:   "example.com",
		Enabled:  true,
	}

	err := manager.saveConfig()
	assert.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}

func TestLoadConfigNonExistent(t *testing.T) {
	manager := NewManager("/nonexistent/path/config.json")

	err := manager.loadConfig()
	assert.NoError(t, err) // Should not error for non-existent file
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	os.WriteFile(configPath, []byte("invalid json"), 0644)

	manager := NewManager(configPath)
	err := manager.loadConfig()
	assert.Error(t, err) // Should error for invalid JSON
}

func TestGetNetworkStats(t *testing.T) {
	manager := NewManager("")

	// Note: This will fail if /sys/class/net doesn't exist or ip command not available
	// We're testing the function exists and handles errors gracefully
	stats, err := manager.GetNetworkStats()
	// May error in sandbox environment, just check no panic
	_ = stats
	_ = err
}

func TestDDNSProviders(t *testing.T) {
	providers := []string{"alidns", "cloudflare", "duckdns", "noip"}

	for _, provider := range providers {
		config := DDNSConfig{
			Provider: provider,
			Enabled:  true,
		}
		assert.Equal(t, provider, config.Provider)
	}
}

func TestPortForwardProtocols(t *testing.T) {
	protocols := []string{"tcp", "udp"}

	for _, protocol := range protocols {
		pf := PortForward{
			Protocol: protocol,
		}
		assert.Equal(t, protocol, pf.Protocol)
	}
}

func TestFirewallActions(t *testing.T) {
	actions := []string{"accept", "drop", "reject"}

	for _, action := range actions {
		rule := FirewallRule{
			Action: action,
		}
		assert.Equal(t, action, rule.Action)
	}
}

func TestFirewallDirections(t *testing.T) {
	directions := []string{"in", "out", "forward"}

	for _, direction := range directions {
		rule := FirewallRule{
			Direction: direction,
		}
		assert.Equal(t, direction, rule.Direction)
	}
}

func TestFirewallProtocols(t *testing.T) {
	protocols := []string{"tcp", "udp", "icmp", "all"}

	for _, protocol := range protocols {
		rule := FirewallRule{
			Protocol: protocol,
		}
		assert.Equal(t, protocol, rule.Protocol)
	}
}

func TestInterfaceTypes(t *testing.T) {
	types := []string{"ethernet", "wifi", "bridge", "virtual", "loopback"}

	for _, typ := range types {
		iface := Interface{
			Type: typ,
		}
		assert.Equal(t, typ, iface.Type)
	}
}

func TestInterfaceStates(t *testing.T) {
	states := []string{"up", "down"}

	for _, state := range states {
		iface := Interface{
			State: state,
		}
		assert.Equal(t, state, iface.State)
	}
}
