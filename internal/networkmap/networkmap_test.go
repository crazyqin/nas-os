package networkmap

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:          true,
		ScanInterval:     30,
		Subnets:          []string{"192.168.1.0/24"},
		AutoDiscover:     true,
		BandwidthMonitor: true,
		AlertNewDevices:  true,
		MaxDevices:       256,
		ScanPorts:        true,
		CommonPorts:      []int{22, 80, 443, 445, 548},
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestAddDevice(t *testing.T) {
	config := &Config{Enabled: true, MaxDevices: 100}
	manager := NewManager(config)

	device := &NetworkDevice{
		ID:         "nas-1",
		Name:       "NAS-Server",
		Hostname:   "nas.local",
		IPAddress:  "192.168.1.100",
		MACAddress: "AA:BB:CC:DD:EE:FF",
		Type:       DeviceNAS,
		Manufacturer: "Synology",
		Model:      "DS920+",
		Connection: ConnEthernet,
	}

	if err := manager.AddDevice(device); err != nil {
		t.Fatalf("AddDevice failed: %v", err)
	}

	got, err := manager.GetDevice("nas-1")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}

	if got.IPAddress != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", got.IPAddress)
	}
}

func TestNetworkStats(t *testing.T) {
	config := &Config{Enabled: true, MaxDevices: 100}
	manager := NewManager(config)

	manager.AddDevice(&NetworkDevice{
		ID:         "dev-1",
		Type:       DeviceNAS,
		Connection: ConnEthernet,
	})
	manager.AddDevice(&NetworkDevice{
		ID:         "dev-2",
		Type:       DevicePC,
		Connection: ConnWiFi,
	})

	stats := manager.GetStats()
	if stats.TotalDevices != 2 {
		t.Errorf("Expected 2 devices, got %d", stats.TotalDevices)
	}
	if stats.ByType["nas"] != 1 {
		t.Errorf("Expected 1 NAS device, got %d", stats.ByType["nas"])
	}
}

func TestTopology(t *testing.T) {
	config := &Config{Enabled: true, MaxDevices: 100}
	manager := NewManager(config)

	manager.AddDevice(&NetworkDevice{
		ID:   "topo-1",
		Type: DeviceRouter,
	})
	manager.AddDevice(&NetworkDevice{
		ID:   "topo-2",
		Type: DeviceNAS,
	})
	manager.AddLink(&NetworkLink{
		ID:       "link-1",
		SourceID: "topo-1",
		TargetID: "topo-2",
		Type:     ConnEthernet,
		Speed:    "1Gbps",
	})

	topology := manager.GetTopology()
	if len(topology.Devices) != 2 {
		t.Errorf("Expected 2 devices in topology, got %d", len(topology.Devices))
	}
	if len(topology.Links) != 1 {
		t.Errorf("Expected 1 link in topology, got %d", len(topology.Links))
	}
}

func TestAlerts(t *testing.T) {
	config := &Config{Enabled: true, AlertNewDevices: true, MaxDevices: 100}
	manager := NewManager(config)

	manager.AddDevice(&NetworkDevice{
		ID:   "alert-dev",
		Name: "New Device",
		Type: DeviceUnknown,
	})

	alerts := manager.GetAlerts(true)
	if len(alerts) < 1 {
		t.Errorf("Expected at least 1 alert, got %d", len(alerts))
	}
}

func TestListDevices(t *testing.T) {
	config := &Config{Enabled: true, MaxDevices: 100}
	manager := NewManager(config)

	manager.AddDevice(&NetworkDevice{ID: "list-1", Type: DevicePC})
	manager.AddDevice(&NetworkDevice{ID: "list-2", Type: DevicePhone})

	devices := manager.ListDevices()
	if len(devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(devices))
	}
}
