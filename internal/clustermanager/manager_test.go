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

	node := &Node{
		Name: "node-1",
		Host: "192.168.1.100",
		Port: 8080,
		Role: RoleWorker,
	}

	err := manager.RegisterNode(node)
	if err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	if node.ID == "" {
		t.Error("Expected node ID to be set")
	}

	if node.Status != NodeStatusJoining {
		t.Errorf("Expected status joining, got %s", node.Status)
	}
}

func TestRegisterNodeEmptyName(t *testing.T) {
	manager := NewClusterManager(nil)

	err := manager.RegisterNode(&Node{Host: "192.168.1.100"})
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

func TestGetNode(t *testing.T) {
	manager := NewClusterManager(nil)

	node := &Node{Name: "node-1", Host: "192.168.1.100", Port: 8080}
	manager.RegisterNode(node)

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

	manager.RegisterNode(&Node{Name: "node-1", Host: "192.168.1.100", Port: 8080})
	manager.RegisterNode(&Node{Name: "node-2", Host: "192.168.1.101", Port: 8080})

	nodes := manager.ListNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
}

func TestUnregisterNode(t *testing.T) {
	manager := NewClusterManager(nil)

	node := &Node{Name: "node-1", Host: "192.168.1.100", Port: 8080}
	manager.RegisterNode(node)

	err := manager.UnregisterNode(node.ID)
	if err != nil {
		t.Fatalf("UnregisterNode failed: %v", err)
	}

	_, err = manager.GetNode(node.ID)
	if err == nil {
		t.Error("Expected error after unregistration")
	}
}

func TestHeartbeat(t *testing.T) {
	manager := NewClusterManager(nil)

	node := &Node{Name: "node-1", Host: "192.168.1.100", Port: 8080}
	manager.RegisterNode(node)

	err := manager.Heartbeat(node.ID)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	updated, _ := manager.GetNode(node.ID)
	if updated.Status != NodeStatusOnline {
		t.Errorf("Expected status online, got %s", updated.Status)
	}
}

func TestCreateCluster(t *testing.T) {
	manager := NewClusterManager(nil)

	cluster, err := manager.CreateCluster("test-cluster")
	if err != nil {
		t.Fatalf("CreateCluster failed: %v", err)
	}

	if cluster.Name != "test-cluster" {
		t.Errorf("Expected name 'test-cluster', got '%s'", cluster.Name)
	}

	if cluster.Status != ClusterStatusHealthy {
		t.Errorf("Expected status healthy, got %s", cluster.Status)
	}
}

func TestCreateClusterEmptyName(t *testing.T) {
	manager := NewClusterManager(nil)

	_, err := manager.CreateCluster("")
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

func TestGetClusterStats(t *testing.T) {
	manager := NewClusterManager(nil)

	manager.RegisterNode(&Node{Name: "node-1", Host: "192.168.1.100", Port: 8080})
	manager.CreateCluster("test-cluster")

	stats := manager.GetClusterStats()
	if stats["total_nodes"] != 1 {
		t.Errorf("Expected 1 total node, got %v", stats["total_nodes"])
	}
}
