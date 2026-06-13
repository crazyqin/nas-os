package clustermgr2

import (
	"testing"
)

func TestManagerStartStop(t *testing.T) {
	m := NewManager(nil)
	if m.IsRunning() {
		t.Fatal("新创建的管理器不应在运行")
	}
	if err := m.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("管理器应该在运行")
	}
	if err := m.Start(); err == nil {
		t.Fatal("重复启动应返回错误")
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	if m.IsRunning() {
		t.Fatal("管理器不应在运行")
	}
}

func TestAddNode(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	node := &ClusterNode{
		ID:      "node-1",
		Name:    "Node 1",
		Address: "192.168.1.10",
		Status:  NodeStatusOnline,
		Region:  "cn-east",
	}
	if err := m.AddNode(node); err != nil {
		t.Fatalf("添加节点失败: %v", err)
	}
	nodes := m.ListNodes("")
	if len(nodes) != 1 {
		t.Fatalf("期望1个节点，实际 %d", len(nodes))
	}
}

func TestAddNodeNotRunning(t *testing.T) {
	m := NewManager(nil)
	node := &ClusterNode{ID: "node-1", Name: "Node 1"}
	if err := m.AddNode(node); err == nil {
		t.Fatal("未运行时添加节点应返回错误")
	}
}

func TestRemoveNode(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.AddNode(&ClusterNode{ID: "node-1", Name: "Node 1", Status: NodeStatusOnline})
	if err := m.RemoveNode("node-1"); err != nil {
		t.Fatalf("移除节点失败: %v", err)
	}
	if len(m.ListNodes("")) != 0 {
		t.Fatal("节点应该已被移除")
	}
}

func TestRemoveNodeWithWorkloads(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.AddNode(&ClusterNode{ID: "node-1", Name: "Node 1", Status: NodeStatusOnline})
	m.DeployWorkload(&Workload{ID: "wl-1", Name: "test", NodeID: "node-1", Type: WorkloadStorage})
	if err := m.RemoveNode("node-1"); err == nil {
		t.Fatal("移除有工作负载的节点应返回错误")
	}
}

func TestRemoveNodeNotFound(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	if err := m.RemoveNode("nonexistent"); err == nil {
		t.Fatal("移除不存在的节点应返回错误")
	}
}

func TestDeployWorkload(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.AddNode(&ClusterNode{ID: "node-1", Name: "Node 1", Status: NodeStatusOnline})
	wl := &Workload{
		ID:     "wl-1",
		Name:   "Web Server",
		Type:   WorkloadCompute,
		NodeID: "node-1",
		CPU:    2.0,
		MemoryGB: 4,
	}
	if err := m.DeployWorkload(wl); err != nil {
		t.Fatalf("部署工作负载失败: %v", err)
	}
	if wl.Status != "running" {
		t.Fatal("工作负载状态应为running")
	}
}

func TestDeployWorkloadOfflineNode(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.AddNode(&ClusterNode{ID: "node-1", Name: "Node 1", Status: NodeStatusOffline})
	wl := &Workload{ID: "wl-1", Name: "test", NodeID: "node-1", Type: WorkloadStorage}
	if err := m.DeployWorkload(wl); err == nil {
		t.Fatal("部署到离线节点应返回错误")
	}
}

func TestMigrateWorkload(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.AddNode(&ClusterNode{ID: "node-1", Name: "Node 1", Status: NodeStatusOnline})
	m.AddNode(&ClusterNode{ID: "node-2", Name: "Node 2", Status: NodeStatusOnline})
	m.DeployWorkload(&Workload{ID: "wl-1", Name: "test", NodeID: "node-1", Type: WorkloadStorage})

	if err := m.MigrateWorkload("wl-1", "node-2"); err != nil {
		t.Fatalf("迁移工作负载失败: %v", err)
	}
	wls := m.ListWorkloads("node-2")
	if len(wls) != 1 {
		t.Fatal("工作负载应迁移到node-2")
	}
}

func TestMigrateWorkloadNotFound(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	if err := m.MigrateWorkload("nonexistent", "node-1"); err == nil {
		t.Fatal("迁移不存在的工作负载应返回错误")
	}
}

func TestListNodes(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.AddNode(&ClusterNode{ID: "node-1", Name: "Node 1", Status: NodeStatusOnline})
	m.AddNode(&ClusterNode{ID: "node-2", Name: "Node 2", Status: NodeStatusOffline})
	m.AddNode(&ClusterNode{ID: "node-3", Name: "Node 3", Status: NodeStatusOnline})

	all := m.ListNodes("")
	if len(all) != 3 {
		t.Fatalf("期望3个节点，实际 %d", len(all))
	}
	online := m.ListNodes(NodeStatusOnline)
	if len(online) != 2 {
		t.Fatalf("期望2个在线节点，实际 %d", len(online))
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.AddNode(&ClusterNode{ID: "node-1", Name: "Node 1", Status: NodeStatusOnline})
	m.AddNode(&ClusterNode{ID: "node-2", Name: "Node 2", Status: NodeStatusOffline})
	m.DeployWorkload(&Workload{ID: "wl-1", Name: "test", NodeID: "node-1", Type: WorkloadStorage})

	stats := m.GetStats()
	if stats["total_nodes"] != 2 {
		t.Fatalf("期望2个节点，实际 %v", stats["total_nodes"])
	}
	if stats["online_nodes"] != 1 {
		t.Fatalf("期望1个在线节点，实际 %v", stats["online_nodes"])
	}
	if stats["total_workloads"] != 1 {
		t.Fatalf("期望1个工作负载，实际 %v", stats["total_workloads"])
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultClusterConfig()
	if config.ClusterName == "" {
		t.Fatal("集群名不能为空")
	}
	if config.MaxNodes != 100 {
		t.Fatal("最大节点数错误")
	}
	if !config.EnableLB {
		t.Fatal("默认应启用负载均衡")
	}
}
