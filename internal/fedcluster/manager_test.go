package fedcluster

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if len(manager.clusters) != 0 {
		t.Errorf("expected 0 clusters, got %d", len(manager.clusters))
	}

	if manager.lbConfig == nil {
		t.Fatal("lbConfig is nil")
	}
}

func TestCreateCluster(t *testing.T) {
	manager := NewManager()

	cluster, err := manager.CreateCluster("test-cluster", "测试集群", SyncAll)
	if err != nil {
		t.Fatalf("CreateCluster failed: %v", err)
	}

	if cluster.Name != "test-cluster" {
		t.Errorf("expected name 'test-cluster', got '%s'", cluster.Name)
	}

	if cluster.SyncPolicy != SyncAll {
		t.Errorf("expected sync policy 'all', got '%s'", cluster.SyncPolicy)
	}

	// 测试重复创建
	_, err = manager.CreateCluster("test-cluster", "另一个集群", SyncSelective)
	if err == nil {
		t.Error("expected error for duplicate cluster name")
	}
}

func TestJoinNode(t *testing.T) {
	manager := NewManager()

	cluster, _ := manager.CreateCluster("test-cluster", "测试集群", SyncAll)

	node := &ClusterNode{
		Name:     "node-1",
		Hostname: "192.168.1.100",
		Port:     8080,
		Role:     RoleWorker,
	}

	err := manager.JoinNode(cluster.ID, node)
	if err != nil {
		t.Fatalf("JoinNode failed: %v", err)
	}

	if node.ID == "" {
		t.Error("node ID should be set")
	}

	if node.Status != NodeOnline {
		t.Errorf("expected status 'online', got '%s'", node.Status)
	}

	// 测试重复加入
	err = manager.JoinNode(cluster.ID, node)
	if err == nil {
		t.Error("expected error for duplicate node")
	}
}

func TestRemoveNode(t *testing.T) {
	manager := NewManager()

	cluster, _ := manager.CreateCluster("test-cluster", "测试集群", SyncAll)

	node := &ClusterNode{
		Name:     "node-1",
		Hostname: "192.168.1.100",
		Port:     8080,
		Role:     RoleWorker,
	}
	manager.JoinNode(cluster.ID, node)

	err := manager.RemoveNode(cluster.ID, node.ID)
	if err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}

	if len(cluster.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(cluster.Nodes))
	}
}

func TestPromoteNode(t *testing.T) {
	manager := NewManager()

	cluster, _ := manager.CreateCluster("test-cluster", "测试集群", SyncAll)

	masterNode := &ClusterNode{
		Name:     "master",
		Hostname: "192.168.1.100",
		Port:     8080,
		Role:     RoleMaster,
	}
	manager.JoinNode(cluster.ID, masterNode)

	workerNode := &ClusterNode{
		Name:     "worker",
		Hostname: "192.168.1.101",
		Port:     8080,
		Role:     RoleWorker,
	}
	manager.JoinNode(cluster.ID, workerNode)

	err := manager.PromoteNode(cluster.ID, workerNode.ID)
	if err != nil {
		t.Fatalf("PromoteNode failed: %v", err)
	}

	// 验证角色变化
	cluster, _ = manager.GetCluster(cluster.ID)
	if cluster.Nodes[masterNode.ID].Role != RoleWorker {
		t.Errorf("expected old master to be worker, got '%s'", cluster.Nodes[masterNode.ID].Role)
	}
	if cluster.Nodes[workerNode.ID].Role != RoleMaster {
		t.Errorf("expected new master to be master, got '%s'", cluster.Nodes[workerNode.ID].Role)
	}
}

func TestStartSync(t *testing.T) {
	manager := NewManager()

	cluster, _ := manager.CreateCluster("test-cluster", "测试集群", SyncAll)

	sourceNode := &ClusterNode{
		Name:     "source",
		Hostname: "192.168.1.100",
		Port:     8080,
		Role:     RoleWorker,
	}
	manager.JoinNode(cluster.ID, sourceNode)

	targetNode := &ClusterNode{
		Name:     "target",
		Hostname: "192.168.1.101",
		Port:     8080,
		Role:     RoleWorker,
	}
	manager.JoinNode(cluster.ID, targetNode)

	job, err := manager.StartSync(cluster.ID, sourceNode.ID, targetNode.ID, "/data", "/backup")
	if err != nil {
		t.Fatalf("StartSync failed: %v", err)
	}

	if job.Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", job.Status)
	}

	// 更新进度
	err = manager.UpdateSyncProgress(job.ID, 50, 1024*1024*512)
	if err != nil {
		t.Fatalf("UpdateSyncProgress failed: %v", err)
	}

	job, _ = manager.GetSyncJob(job.ID)
	if job.Progress != 50 {
		t.Errorf("expected progress 50, got %f", job.Progress)
	}
}

func TestHealthCheck(t *testing.T) {
	manager := NewManager()

	cluster, _ := manager.CreateCluster("test-cluster", "测试集群", SyncAll)

	node := &ClusterNode{
		Name:     "node-1",
		Hostname: "192.168.1.100",
		Port:     8080,
		Role:     RoleWorker,
	}
	manager.JoinNode(cluster.ID, node)

	health := manager.HealthCheck(cluster.ID)
	if !health[node.ID] {
		t.Error("node should be healthy")
	}
}

func TestGetClusterStats(t *testing.T) {
	manager := NewManager()

	cluster, _ := manager.CreateCluster("test-cluster", "测试集群", SyncAll)

	node := &ClusterNode{
		Name:          "node-1",
		Hostname:      "192.168.1.100",
		Port:          8080,
		Role:          RoleWorker,
		StorageTB:     10,
		UsedStorageTB: 5,
	}
	manager.JoinNode(cluster.ID, node)

	stats, err := manager.GetClusterStats(cluster.ID)
	if err != nil {
		t.Fatalf("GetClusterStats failed: %v", err)
	}

	if stats["total_storage"].(float64) != 10 {
		t.Errorf("expected total_storage 10, got %v", stats["total_storage"])
	}

	if stats["used_storage"].(float64) != 5 {
		t.Errorf("expected used_storage 5, got %v", stats["used_storage"])
	}
}

func TestListClusters(t *testing.T) {
	manager := NewManager()

	manager.CreateCluster("cluster-1", "集群1", SyncAll)
	manager.CreateCluster("cluster-2", "集群2", SyncSelective)

	clusters := manager.ListClusters()
	if len(clusters) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(clusters))
	}
}

func TestSetMaintenanceMode(t *testing.T) {
	manager := NewManager()

	cluster, _ := manager.CreateCluster("test-cluster", "测试集群", SyncAll)

	node := &ClusterNode{
		Name:     "node-1",
		Hostname: "192.168.1.100",
		Port:     8080,
		Role:     RoleWorker,
	}
	manager.JoinNode(cluster.ID, node)

	err := manager.SetMaintenanceMode(cluster.ID, node.ID, true)
	if err != nil {
		t.Fatalf("SetMaintenanceMode failed: %v", err)
	}

	cluster, _ = manager.GetCluster(cluster.ID)
	if cluster.Nodes[node.ID].Status != NodeMaintenance {
		t.Errorf("expected maintenance status, got '%s'", cluster.Nodes[node.ID].Status)
	}

	// 退出维护模式
	err = manager.SetMaintenanceMode(cluster.ID, node.ID, false)
	if err != nil {
		t.Fatalf("SetMaintenanceMode disable failed: %v", err)
	}

	cluster, _ = manager.GetCluster(cluster.ID)
	if cluster.Nodes[node.ID].Status != NodeOnline {
		t.Errorf("expected online status, got '%s'", cluster.Nodes[node.ID].Status)
	}
}

func TestSelectNodeForRequest(t *testing.T) {
	manager := NewManager()

	cluster, _ := manager.CreateCluster("test-cluster", "测试集群", SyncAll)

	node1 := &ClusterNode{
		Name:          "node-1",
		Hostname:      "192.168.1.100",
		Port:          8080,
		Role:          RoleWorker,
		StorageTB:     10,
		UsedStorageTB: 5,
	}
	manager.JoinNode(cluster.ID, node1)

	node2 := &ClusterNode{
		Name:          "node-2",
		Hostname:      "192.168.1.101",
		Port:          8080,
		Role:          RoleWorker,
		StorageTB:     10,
		UsedStorageTB: 3,
	}
	manager.JoinNode(cluster.ID, node2)

	// 测试轮询策略
	manager.lbConfig.Strategy = "round_robin"
	node, err := manager.SelectNodeForRequest(cluster.ID)
	if err != nil {
		t.Fatalf("SelectNodeForRequest failed: %v", err)
	}
	if node == nil {
		t.Fatal("selected node is nil")
	}

	// 测试最少连接策略
	manager.lbConfig.Strategy = "least_connections"
	node, err = manager.SelectNodeForRequest(cluster.ID)
	if err != nil {
		t.Fatalf("SelectNodeForRequest failed: %v", err)
	}
	if node.ID != node2.ID {
		t.Errorf("expected node2 (less usage), got node %s", node.ID)
	}
}

func TestGetEventLog(t *testing.T) {
	manager := NewManager()

	cluster, _ := manager.CreateCluster("test-cluster", "测试集群", SyncAll)

	node := &ClusterNode{
		Name:     "node-1",
		Hostname: "192.168.1.100",
		Port:     8080,
		Role:     RoleWorker,
	}
	manager.JoinNode(cluster.ID, node)

	events := manager.GetEventLog(10)
	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}
}
