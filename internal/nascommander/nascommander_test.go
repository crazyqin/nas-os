package nascommander

import (
	"fmt"
	"testing"
)

func TestNewCommander(t *testing.T) {
	cmd := NewCommander(nil)
	if cmd == nil {
		t.Fatal("expected non-nil commander")
	}
	if cmd.config.MaxNodes != 100 {
		t.Errorf("expected max nodes 100, got %d", cmd.config.MaxNodes)
	}
}

func TestRegisterNode(t *testing.T) {
	cmd := NewCommander(nil)

	node := &NASNode{
		ID:       "node-1",
		Name:     "NAS-1",
		Hostname: "nas1.local",
		Role:     RolePrimary,
	}

	err := cmd.RegisterNode(node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 测试重复注册
	err = cmd.RegisterNode(node)
	if err == nil {
		t.Fatal("expected error for duplicate node")
	}
}

func TestUnregisterNode(t *testing.T) {
	cmd := NewCommander(nil)

	node := &NASNode{
		ID:   "node-2",
		Name: "NAS-2",
	}
	cmd.RegisterNode(node)

	err := cmd.UnregisterNode("node-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = cmd.GetNode("node-2")
	if err == nil {
		t.Fatal("expected error for unregistered node")
	}
}

func TestGetNode(t *testing.T) {
	cmd := NewCommander(nil)

	node := &NASNode{
		ID:   "node-3",
		Name: "NAS-3",
	}
	cmd.RegisterNode(node)

	got, err := cmd.GetNode("node-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "NAS-3" {
		t.Errorf("expected name 'NAS-3', got '%s'", got.Name)
	}
}

func TestListNodes(t *testing.T) {
	cmd := NewCommander(nil)

	for i := 0; i < 3; i++ {
		cmd.RegisterNode(&NASNode{
			ID:   fmt.Sprintf("node-%d", i),
			Name: fmt.Sprintf("NAS-%d", i),
		})
	}

	nodes := cmd.ListNodes()
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}
}

func TestUpdateNodeMetrics(t *testing.T) {
	cmd := NewCommander(nil)

	node := &NASNode{
		ID:   "node-4",
		Name: "NAS-4",
	}
	cmd.RegisterNode(node)

	metrics := &NodeMetrics{
		CPUUsage:    50.0,
		MemoryUsage: 60.0,
		DiskUsage:   70.0,
	}

	err := cmd.UpdateNodeMetrics("node-4", metrics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := cmd.GetNode("node-4")
	if got.Metrics.CPUUsage != 50.0 {
		t.Errorf("expected CPU 50.0, got %f", got.Metrics.CPUUsage)
	}
}

func TestCreateCluster(t *testing.T) {
	cmd := NewCommander(nil)

	cluster := &Cluster{
		ID:   "cluster-1",
		Name: "Production",
		Nodes: []*NASNode{
			{ID: "node-1", Name: "NAS-1"},
		},
	}

	err := cmd.CreateCluster(cluster)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 测试重复创建
	err = cmd.CreateCluster(cluster)
	if err == nil {
		t.Fatal("expected error for duplicate cluster")
	}
}

func TestAlerts(t *testing.T) {
	cmd := NewCommander(nil)

	alert := &Alert{
		ID:       "alert-1",
		NodeID:   "node-1",
		Level:    "warning",
		Category: "disk",
		Message:  "Disk usage high",
	}

	cmd.AddAlert(alert)

	alerts := cmd.GetAlerts(false)
	if len(alerts) != 1 {
		t.Errorf("expected 1 unacked alert, got %d", len(alerts))
	}

	err := cmd.AcknowledgeAlert("alert-1", "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	alerts = cmd.GetAlerts(false)
	if len(alerts) != 0 {
		t.Errorf("expected 0 unacked alerts, got %d", len(alerts))
	}
}

func TestClusterStatus(t *testing.T) {
	cmd := NewCommander(nil)

	// 空集群
	status := cmd.GetClusterStatus()
	if status != ClusterUnknown {
		t.Errorf("expected unknown, got %s", status)
	}

	// 添加在线节点
	cmd.RegisterNode(&NASNode{ID: "n1", Status: NodeStatusOnline})
	cmd.RegisterNode(&NASNode{ID: "n2", Status: NodeStatusOnline})
	cmd.RegisterNode(&NASNode{ID: "n3", Status: NodeStatusOffline})

	status = cmd.GetClusterStatus()
	if status != ClusterWarning {
		t.Errorf("expected warning, got %s", status)
	}
}

func TestStartStop(t *testing.T) {
	cmd := NewCommander(nil)

	err := cmd.Start()
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	err = cmd.Start()
	if err == nil {
		t.Fatal("expected error for double start")
	}

	err = cmd.Stop()
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}
