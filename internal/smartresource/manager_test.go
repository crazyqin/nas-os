package smartresource

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config == nil {
		t.Error("config not initialized")
	}
	if m.resources == nil {
		t.Error("resources map not initialized")
	}
	if m.allocations == nil {
		t.Error("allocations map not initialized")
	}
}

func TestRegisterNode(t *testing.T) {
	m := NewManager(nil)

	node := &Node{
		ID:   "node-1",
		Name: "Main Node",
		Host: "192.168.1.100",
		Resources: map[ResourceType]*Resource{
			ResourceCPU: {
				ID:   "cpu-1",
				Type: ResourceCPU,
				Name: "CPU",
				Total: 8.0,
				Used:  2.0,
				Unit:  "cores",
			},
			ResourceMemory: {
				ID:   "mem-1",
				Type: ResourceMemory,
				Name: "Memory",
				Total: 16.0,
				Used:  4.0,
				Unit:  "GB",
			},
		},
	}

	err := m.RegisterNode(node)
	if err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	if _, exists := m.nodes["node-1"]; !exists {
		t.Error("node not registered")
	}
	if _, exists := m.resources["cpu-1"]; !exists {
		t.Error("cpu resource not registered")
	}
}

func TestUnregisterNode(t *testing.T) {
	m := NewManager(nil)

	node := &Node{
		ID:   "node-1",
		Name: "Main Node",
		Resources: map[ResourceType]*Resource{
			ResourceCPU: {ID: "cpu-1", Type: ResourceCPU, Total: 8.0, Used: 2.0},
		},
	}
	m.RegisterNode(node)

	// Allocate some resources
	m.AllocateResource(context.Background(), "cpu-1", "service-1", 1.0, 1)

	err := m.UnregisterNode("node-1")
	if err != nil {
		t.Fatalf("UnregisterNode failed: %v", err)
	}

	if _, exists := m.nodes["node-1"]; exists {
		t.Error("node still exists after unregister")
	}
}

func TestUnregisterNodeNotFound(t *testing.T) {
	m := NewManager(nil)

	err := m.UnregisterNode("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestAddResource(t *testing.T) {
	m := NewManager(nil)

	resource := &Resource{
		ID:    "cpu-1",
		Type:  ResourceCPU,
		Name:  "CPU",
		Total: 8.0,
		Used:  2.0,
		Unit:  "cores",
	}

	err := m.AddResource(resource)
	if err != nil {
		t.Fatalf("AddResource failed: %v", err)
	}

	if resource.Available != 6.0 {
		t.Errorf("expected available 6.0, got %f", resource.Available)
	}
}

func TestAddResourceEmptyID(t *testing.T) {
	m := NewManager(nil)

	err := m.AddResource(&Resource{})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestGetResource(t *testing.T) {
	m := NewManager(nil)

	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Name: "CPU", Total: 8.0})

	got, err := m.GetResource("cpu-1")
	if err != nil {
		t.Fatalf("GetResource failed: %v", err)
	}
	if got.Name != "CPU" {
		t.Errorf("expected name 'CPU', got '%s'", got.Name)
	}
}

func TestGetResourceNotFound(t *testing.T) {
	m := NewManager(nil)

	_, err := m.GetResource("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent resource")
	}
}

func TestListResources(t *testing.T) {
	m := NewManager(nil)

	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Name: "CPU 1"})
	m.AddResource(&Resource{ID: "cpu-2", Type: ResourceCPU, Name: "CPU 2"})
	m.AddResource(&Resource{ID: "mem-1", Type: ResourceMemory, Name: "Memory"})

	resources := m.ListResources(ResourceCPU)
	if len(resources) != 2 {
		t.Errorf("expected 2 CPU resources, got %d", len(resources))
	}
}

func TestAllocateResource(t *testing.T) {
	m := NewManager(nil)

	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Total: 8.0, Used: 0.0})

	ctx := context.Background()
	allocation, err := m.AllocateResource(ctx, "cpu-1", "my-service", 2.0, 1)
	if err != nil {
		t.Fatalf("AllocateResource failed: %v", err)
	}

	if allocation.ID == "" {
		t.Error("allocation ID not generated")
	}
	if allocation.Status != StatusAllocated {
		t.Errorf("expected status '%s', got '%s'", StatusAllocated, allocation.Status)
	}

	resource, _ := m.GetResource("cpu-1")
	if resource.Used != 2.0 {
		t.Errorf("expected used 2.0, got %f", resource.Used)
	}
	if resource.Available != 6.0 {
		t.Errorf("expected available 6.0, got %f", resource.Available)
	}
}

func TestAllocateResourceInsufficient(t *testing.T) {
	m := NewManager(nil)

	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Total: 8.0, Used: 6.0})

	ctx := context.Background()
	_, err := m.AllocateResource(ctx, "cpu-1", "my-service", 3.0, 1)
	if err == nil {
		t.Error("expected error for insufficient resources")
	}
}

func TestAllocateResourceNotFound(t *testing.T) {
	m := NewManager(nil)

	ctx := context.Background()
	_, err := m.AllocateResource(ctx, "nonexistent", "my-service", 1.0, 1)
	if err == nil {
		t.Error("expected error for nonexistent resource")
	}
}

func TestReleaseAllocation(t *testing.T) {
	m := NewManager(nil)

	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Total: 8.0, Used: 0.0})

	ctx := context.Background()
	allocation, _ := m.AllocateResource(ctx, "cpu-1", "my-service", 2.0, 1)

	err := m.ReleaseAllocation(allocation.ID)
	if err != nil {
		t.Fatalf("ReleaseAllocation failed: %v", err)
	}

	got, _ := m.GetResource("cpu-1")
	if got.Used != 0.0 {
		t.Errorf("expected used 0.0, got %f", got.Used)
	}
}

func TestReleaseAllocationNotFound(t *testing.T) {
	m := NewManager(nil)

	err := m.ReleaseAllocation("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent allocation")
	}
}

func TestGetAllocations(t *testing.T) {
	m := NewManager(nil)

	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Total: 8.0, Used: 0.0})

	ctx := context.Background()
	m.AllocateResource(ctx, "cpu-1", "service-1", 1.0, 1)
	m.AllocateResource(ctx, "cpu-1", "service-2", 2.0, 2)
	m.AllocateResource(ctx, "cpu-1", "service-1", 1.5, 1)

	allocations := m.GetAllocations("service-1")
	if len(allocations) != 2 {
		t.Errorf("expected 2 allocations, got %d", len(allocations))
	}
}

func TestPredictUsage(t *testing.T) {
	m := NewManager(nil)

	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Total: 8.0, Used: 4.0})

	ctx := context.Background()
	prediction, err := m.PredictUsage(ctx, ResourceCPU)
	if err != nil {
		t.Fatalf("PredictUsage failed: %v", err)
	}

	if prediction.Current == 0 {
		t.Error("current usage not set")
	}
	if prediction.Predicted == 0 {
		t.Error("predicted usage not set")
	}
	if prediction.Confidence == 0 {
		t.Error("confidence not set")
	}
}

func TestPredictUsageNoResources(t *testing.T) {
	m := NewManager(nil)

	ctx := context.Background()
	_, err := m.PredictUsage(ctx, ResourceGPU)
	if err == nil {
		t.Error("expected error for no GPU resources")
	}
}

func TestGetOptimizations(t *testing.T) {
	m := NewManager(nil)

	// Add underutilized resource
	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Name: "CPU", Total: 8.0, Used: 1.0})

	ctx := context.Background()
	opts, err := m.GetOptimizations(ctx)
	if err != nil {
		t.Fatalf("GetOptimizations failed: %v", err)
	}

	if len(opts) == 0 {
		t.Error("expected optimizations")
	}
}

func TestGetOptimizationsHighUsage(t *testing.T) {
	m := NewManager(nil)

	// Add high usage resource
	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Name: "CPU", Total: 8.0, Used: 7.5})

	ctx := context.Background()
	opts, err := m.GetOptimizations(ctx)
	if err != nil {
		t.Fatalf("GetOptimizations failed: %v", err)
	}

	if len(opts) == 0 {
		t.Error("expected optimizations for high usage")
	}
}

func TestGetNodes(t *testing.T) {
	m := NewManager(nil)

	m.RegisterNode(&Node{ID: "node-1", Name: "Node 1"})
	m.RegisterNode(&Node{ID: "node-2", Name: "Node 2"})

	nodes := m.GetNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestGetClusterStats(t *testing.T) {
	m := NewManager(nil)

	m.RegisterNode(&Node{ID: "node-1", Name: "Node 1"})
	m.RegisterNode(&Node{ID: "node-2", Name: "Node 2"})

	stats := m.GetClusterStats()
	if stats["total_nodes"] != 2 {
		t.Errorf("expected 2 total nodes, got %v", stats["total_nodes"])
	}
	if stats["online_nodes"] != 2 {
		t.Errorf("expected 2 online nodes, got %v", stats["online_nodes"])
	}
}

func TestResourceTypes(t *testing.T) {
	types := []ResourceType{ResourceCPU, ResourceMemory, ResourceDisk, ResourceNetwork, ResourceGPU}
	expected := []string{"cpu", "memory", "disk", "network", "gpu"}

	for i, typ := range types {
		if string(typ) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], string(typ))
		}
	}
}

func TestAllocationStatuses(t *testing.T) {
	statuses := []AllocationStatus{StatusPending, StatusAllocated, StatusReleased, StatusFailed}
	expected := []string{"pending", "allocated", "released", "failed"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], string(s))
		}
	}
}

func TestPredictionTrend(t *testing.T) {
	m := NewManager(nil)

	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Total: 8.0, Used: 4.0})

	ctx := context.Background()
	prediction, _ := m.PredictUsage(ctx, ResourceCPU)

	if prediction.Trend != "increasing" && prediction.Trend != "stable" && prediction.Trend != "decreasing" {
		t.Errorf("invalid trend: %s", prediction.Trend)
	}
}

func TestNodeLastSeen(t *testing.T) {
	m := NewManager(nil)

	before := time.Now()
	m.RegisterNode(&Node{ID: "node-1", Name: "Node 1"})
	after := time.Now()

	node, _ := m.nodes["node-1"]
	if node.LastSeen.Before(before) || node.LastSeen.After(after) {
		t.Error("last_seen not set correctly")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager(nil)

	m.AddResource(&Resource{ID: "cpu-1", Type: ResourceCPU, Total: 100.0, Used: 0.0})

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			ctx := context.Background()
			m.AllocateResource(ctx, "cpu-1", "service", 1.0, 1)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	resource, _ := m.GetResource("cpu-1")
	if resource.Used != 10.0 {
		t.Errorf("expected used 10.0, got %f", resource.Used)
	}
}
