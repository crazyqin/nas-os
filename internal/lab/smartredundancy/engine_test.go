package smartredundancy

import (
	"fmt"
	"testing"

	"go.uber.org/zap"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestRegisterNode(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	node := &StorageNode{
		ID:       "node-1",
		Name:     "test-node",
		Address:  "192.168.1.100:8080",
		State:    NodeStateOnline,
		Capacity: 1024 * 1024 * 1024 * 1024, // 1TB
		Health:   95.0,
	}

	if err := engine.RegisterNode(node); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := engine.GetNode("node-1")
	if !ok {
		t.Fatal("expected node to be registered")
	}
	if got.Name != "test-node" {
		t.Errorf("expected name 'test-node', got '%s'", got.Name)
	}
}

func TestRegisterNodeInvalidID(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	node := &StorageNode{
		ID: "",
	}

	if err := engine.RegisterNode(node); err != ErrInvalidNodeID {
		t.Errorf("expected ErrInvalidNodeID, got %v", err)
	}
}

func TestGetOnlineNodes(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	engine.RegisterNode(&StorageNode{ID: "1", State: NodeStateOnline, Health: 90})
	engine.RegisterNode(&StorageNode{ID: "2", State: NodeStateOffline, Health: 50})
	engine.RegisterNode(&StorageNode{ID: "3", State: NodeStateOnline, Health: 80})

	online := engine.GetOnlineNodes()
	if len(online) != 2 {
		t.Errorf("expected 2 online nodes, got %d", len(online))
	}
}

func TestCalculatePlacement(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	// 注册3个节点
	for i := 1; i <= 3; i++ {
		engine.RegisterNode(&StorageNode{
			ID:     fmt.Sprintf("node-%d", i),
			State:  NodeStateOnline,
			Health: float64(90 + i),
		})
	}

	policy := &RedundancyPolicy{
		Level:    RedundancyMirror,
		MinNodes: 2,
	}

	placement, err := engine.CalculatePlacement(policy, 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if placement.Primary == "" {
		t.Error("expected primary node to be set")
	}
	if len(placement.Secondary) == 0 {
		t.Error("expected secondary nodes to be set")
	}
}

func TestCalculatePlacementInsufficientNodes(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	engine.RegisterNode(&StorageNode{ID: "1", State: NodeStateOnline, Health: 90})

	policy := &RedundancyPolicy{
		Level:    RedundancyMirror,
		MinNodes: 3,
	}

	_, err := engine.CalculatePlacement(policy, 1024)
	if err != ErrInsufficientNodes {
		t.Errorf("expected ErrInsufficientNodes, got %v", err)
	}
}

func TestGetClusterStatus(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	engine.RegisterNode(&StorageNode{ID: "1", State: NodeStateOnline, Health: 90, Capacity: 1000, Used: 500})
	engine.RegisterNode(&StorageNode{ID: "2", State: NodeStateOffline, Health: 0, Capacity: 2000, Used: 1000})

	status := engine.GetClusterStatus()

	if status["total_nodes"] != 2 {
		t.Errorf("expected 2 total nodes, got %v", status["total_nodes"])
	}
	if status["online"] != 1 {
		t.Errorf("expected 1 online, got %v", status["online"])
	}
	if status["offline"] != 1 {
		t.Errorf("expected 1 offline, got %v", status["offline"])
	}
}
