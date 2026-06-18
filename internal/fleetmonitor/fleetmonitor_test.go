package fleetmonitor

import (
	"testing"
	"time"
)

func TestNewMonitor(t *testing.T) {
	m := NewMonitor()
	if m == nil {
		t.Fatal("NewMonitor returned nil")
	}

	if len(m.nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(m.nodes))
	}
}

func TestRegisterNode(t *testing.T) {
	m := NewMonitor()

	node := &Node{
		ID:       "node-1",
		Name:     "Primary NAS",
		Hostname: "nas-primary",
		IPAddress: "192.168.1.100",
		Type:     NodeTypePrimary,
		CPU: CPUInfo{
			Model: "Intel i7",
			Cores: 8,
			Usage: 45.5,
		},
		Memory: MemoryInfo{
			Total: 16 * 1024 * 1024 * 1024,
			Used:  8 * 1024 * 1024 * 1024,
		},
	}

	err := m.RegisterNode(node)
	if err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	if node.Status != NodeStatusOnline {
		t.Errorf("Expected status online, got %s", node.Status)
	}

	// Verify node exists
	retrieved, err := m.GetNode("node-1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if retrieved.Name != "Primary NAS" {
		t.Errorf("Expected name 'Primary NAS', got '%s'", retrieved.Name)
	}
}

func TestUnregisterNode(t *testing.T) {
	m := NewMonitor()

	node := &Node{
		ID:   "node-1",
		Name: "Test Node",
	}
	m.RegisterNode(node)

	err := m.UnregisterNode("node-1")
	if err != nil {
		t.Fatalf("UnregisterNode failed: %v", err)
	}

	_, err = m.GetNode("node-1")
	if err == nil {
		t.Error("Expected error for unregistered node")
	}
}

func TestUpdateNodeStatus(t *testing.T) {
	m := NewMonitor()

	node := &Node{
		ID:   "node-1",
		Name: "Test Node",
		CPU: CPUInfo{
			Cores: 4,
			Usage: 30.0,
		},
		Memory: MemoryInfo{
			Total: 8 * 1024 * 1024 * 1024,
			Used:  4 * 1024 * 1024 * 1024,
		},
	}
	m.RegisterNode(node)

	// Update with high CPU
	updated := &Node{
		Status: NodeStatusWarning,
		CPU: CPUInfo{
			Cores: 4,
			Usage: 95.0,
		},
		Memory: MemoryInfo{
			Total: 8 * 1024 * 1024 * 1024,
			Used:  4 * 1024 * 1024 * 1024,
		},
	}

	err := m.UpdateNodeStatus("node-1", updated)
	if err != nil {
		t.Fatalf("UpdateNodeStatus failed: %v", err)
	}

	// Wait for alert check
	time.Sleep(time.Millisecond * 100)

	alerts := m.GetAlerts("node-1", false)
	if len(alerts) == 0 {
		t.Error("Expected alert for high CPU usage")
	}
}

func TestGetStats(t *testing.T) {
	m := NewMonitor()

	// Register multiple nodes
	m.RegisterNode(&Node{
		ID:     "node-1",
		Name:   "Node 1",
		Status: NodeStatusOnline,
		CPU:    CPUInfo{Cores: 4, Usage: 50.0},
		Memory: MemoryInfo{Total: 8 * 1024 * 1024 * 1024, Used: 4 * 1024 * 1024 * 1024},
	})
	m.RegisterNode(&Node{
		ID:     "node-2",
		Name:   "Node 2",
		Status: NodeStatusOnline,
		CPU:    CPUInfo{Cores: 8, Usage: 30.0},
		Memory: MemoryInfo{Total: 16 * 1024 * 1024 * 1024, Used: 8 * 1024 * 1024 * 1024},
	})

	// Wait for stats collection
	time.Sleep(time.Second * 11)

	stats := m.GetStats()
	if stats.TotalNodes != 2 {
		t.Errorf("Expected 2 nodes, got %d", stats.TotalNodes)
	}
	if stats.OnlineNodes != 2 {
		t.Errorf("Expected 2 online nodes, got %d", stats.OnlineNodes)
	}
}

func TestAlertResolution(t *testing.T) {
	m := NewMonitor()

	// Create an alert
	alert := &Alert{
		ID:       "alert-1",
		NodeID:   "node-1",
		Level:    AlertLevelWarning,
		Category: "cpu",
		Message:  "High CPU usage",
	}
	m.alerts[alert.ID] = alert

	// Resolve alert
	err := m.ResolveAlert("alert-1")
	if err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}

	// Verify resolved
	alerts := m.GetAlerts("node-1", true)
	if len(alerts) != 1 {
		t.Errorf("Expected 1 resolved alert, got %d", len(alerts))
	}
}
