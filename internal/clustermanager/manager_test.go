package clustermanager

import (
	"testing"
)

func TestNewClusterManager(t *testing.T) {
	manager := NewClusterManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	if manager.config == nil {
		t.Fatal("Expected default config")
	}
}

func TestRegisterNode(t *testing.T) {
	manager := NewClusterManager(nil)

	node, err := manager.RegisterNode(&AddNodeRequest{
		Name:      "node-1",
		IPAddress: "192.168.1.100",
		Port:      8080,
		Type:      NodeTypeStorage,
	})
	if err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	if node.ID == "" {
		t.Error("Expected node ID to be set")
	}

	if node.Status != NodeStatusOnline {
		t.Errorf("Expected status online, got %s", node.Status)
	}
}

func TestRegisterNodeEmptyName(t *testing.T) {
	manager := NewClusterManager(nil)

	_, err := manager.RegisterNode(&AddNodeRequest{IPAddress: "192.168.1.100"})
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

func TestGetNode(t *testing.T) {
	manager := NewClusterManager(nil)

	node, _ := manager.RegisterNode(&AddNodeRequest{
		Name:      "node-1",
		IPAddress: "192.168.1.100",
		Port:      8080,
	})

	fetched, err := manager.GetNode(node.ID)
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if fetched.Name != "node-1" {
		t.Errorf("Expected name 'node-1', got '%s'", fetched.Name)
	}
}

func TestGetNodeNotFound(t *testing.T) {
	manager := NewClusterManager(nil)

	_, err := manager.GetNode("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent node")
	}
}

func TestListNodes(t *testing.T) {
	manager := NewClusterManager(nil)

	manager.RegisterNode(&AddNodeRequest{Name: "node-1", IPAddress: "192.168.1.100", Port: 8080})
	manager.RegisterNode(&AddNodeRequest{Name: "node-2", IPAddress: "192.168.1.101", Port: 8080})

	nodes := manager.ListNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
}

func TestUnregisterNode(t *testing.T) {
	manager := NewClusterManager(nil)

	node, _ := manager.RegisterNode(&AddNodeRequest{
		Name:      "node-1",
		IPAddress: "192.168.1.100",
		Port:      8080,
	})

	err := manager.UnregisterNode(node.ID)
	if err != nil {
		t.Fatalf("UnregisterNode failed: %v", err)
	}

	_, err = manager.GetNode(node.ID)
	if err == nil {
		t.Error("Expected error after unregistration")
	}
}
