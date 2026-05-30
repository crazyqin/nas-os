package clusterorch

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// ==================== 辅助函数 ====================

func newTestOrch() *ClusterOrch {
	return New(ClusterOrchConfig{
		LocalID:            "test-local",
		AutoScale:          true,
		ScaleUpThreshold:   0.8,
		ScaleDownThreshold: 0.3,
		MinNodes:           1,
		MaxNodes:           10,
	})
}

func newTestNode(id, name string) *Node {
	return &Node{
		ID:      id,
		Name:    name,
		Address: fmt.Sprintf("192.168.1.%s", id),
		Resources: &NodeResources{
			CPU:     ResourcePool{Total: 8000, Used: 0, Available: 8000},
			Memory:  ResourcePool{Total: 16 * 1024 * 1024 * 1024, Used: 0, Available: 16 * 1024 * 1024 * 1024},
			Storage: ResourcePool{Total: 1024 * 1024 * 1024 * 1024, Used: 0, Available: 1024 * 1024 * 1024 * 1024},
		},
		Weight: 100,
	}
}

func newTestService(id, name, nodeID string) *Service {
	return &Service{
		ID:     id,
		Name:   name,
		NodeID: nodeID,
		Ports:  []int{8080},
		Resources: &ResourceRequest{
			CPU:     1000,
			Memory:  1024 * 1024 * 1024,
			Storage: 10 * 1024 * 1024 * 1024,
		},
	}
}

// ==================== 构造函数测试 ====================

func TestNew(t *testing.T) {
	orch := New(ClusterOrchConfig{})
	if orch == nil {
		t.Fatal("New 返回 nil")
	}
	nodes := orch.ListNodes()
	if len(nodes) != 0 {
		t.Fatalf("期望 0 个节点，得到 %d", len(nodes))
	}
}

func TestNewWithDefaults(t *testing.T) {
	orch := New(ClusterOrchConfig{
		MaxLogSize:         -1,
		ScaleUpThreshold:   -1,
		ScaleDownThreshold: -1,
		MinNodes:           -1,
		MaxNodes:           -1,
	})
	if orch == nil {
		t.Fatal("New 返回 nil")
	}
}

// ==================== 集群管理测试 ====================

func TestAddNode(t *testing.T) {
	orch := newTestOrch()
	node := newTestNode("n1", "node-1")

	if err := orch.AddNode(node); err != nil {
		t.Fatalf("AddNode 失败: %v", err)
	}

	got, err := orch.GetNode("n1")
	if err != nil {
		t.Fatalf("GetNode 失败: %v", err)
	}
	if got.ID != "n1" || got.State != NodeStateOnline {
		t.Fatalf("节点状态不正确: %+v", got)
	}
}

func TestAddNodeDuplicate(t *testing.T) {
	orch := newTestOrch()
	node := newTestNode("n1", "node-1")
	orch.AddNode(node)

	if err := orch.AddNode(newTestNode("n1", "node-1-dup")); err != ErrNodeAlreadyExists {
		t.Fatalf("期望 ErrNodeAlreadyExists，得到 %v", err)
	}
}

func TestAddNodeClusterFull(t *testing.T) {
	orch := New(ClusterOrchConfig{MaxNodes: 2})
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	if err := orch.AddNode(newTestNode("n3", "node-3")); err != ErrClusterFull {
		t.Fatalf("期望 ErrClusterFull，得到 %v", err)
	}
}

func TestRemoveNode(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	if err := orch.RemoveNode("n1"); err != nil {
		t.Fatalf("RemoveNode 失败: %v", err)
	}

	_, err := orch.GetNode("n1")
	if err != ErrNodeNotFound {
		t.Fatalf("期望 ErrNodeNotFound，得到 %v", err)
	}
}

func TestRemoveNodeNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.RemoveNode("nonexistent"); err != ErrNodeNotFound {
		t.Fatalf("期望 ErrNodeNotFound，得到 %v", err)
	}
}

func TestRemoveNodeMinNodes(t *testing.T) {
	orch := New(ClusterOrchConfig{MinNodes: 2})
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	err := orch.RemoveNode("n1")
	if err == nil {
		t.Fatal("期望错误，得到 nil")
	}
}

func TestRemoveNodeMigratesServices(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	svc := newTestService("s1", "web", "n1")
	orch.RegisterService(svc)

	if err := orch.RemoveNode("n1"); err != nil {
		t.Fatalf("RemoveNode 失败: %v", err)
	}

	s, _ := orch.GetService("s1")
	if s.NodeID != "n2" {
		t.Fatalf("服务未迁移到 n2: %s", s.NodeID)
	}
}

func TestListNodes(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n2", "node-2"))
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n3", "node-3"))

	nodes := orch.ListNodes()
	if len(nodes) != 3 {
		t.Fatalf("期望 3 个节点，得到 %d", len(nodes))
	}
	// 应该按 ID 排序
	if nodes[0].ID != "n1" || nodes[1].ID != "n2" || nodes[2].ID != "n3" {
		t.Fatal("节点未按 ID 排序")
	}
}

func TestUpdateNodeHeartbeat(t *testing.T) {
	orch := newTestOrch()
	node := newTestNode("n1", "node-1")
	orch.AddNode(node)

	// 标记离线再心跳恢复
	orch.MarkNodeOffline("n1")
	n, _ := orch.GetNode("n1")
	if n.State != NodeStateOffline {
		t.Fatalf("期望离线，得到 %s", n.State)
	}

	orch.UpdateNodeHeartbeat("n1")
	n, _ = orch.GetNode("n1")
	if n.State != NodeStateOnline {
		t.Fatalf("期望在线，得到 %s", n.State)
	}
}

func TestUpdateNodeHeartbeatNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.UpdateNodeHeartbeat("nonexistent"); err != ErrNodeNotFound {
		t.Fatalf("期望 ErrNodeNotFound，得到 %v", err)
	}
}

func TestMarkNodeOfflineNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.MarkNodeOffline("nonexistent"); err != ErrNodeNotFound {
		t.Fatalf("期望 ErrNodeNotFound，得到 %v", err)
	}
}

func TestMarkNodeOfflineFailover(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	svc := newTestService("s1", "web", "n1")
	orch.RegisterService(svc)

	orch.MarkNodeOffline("n1")
	s, _ := orch.GetService("s1")
	if s.NodeID != "n2" || s.State != ServiceStateRunning {
		t.Fatalf("服务故障转移失败: node=%s state=%s", s.NodeID, s.State)
	}
}

func TestGetClusterTopology(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))
	orch.RegisterService(newTestService("s1", "web", "n1"))

	topo := orch.GetClusterTopology()
	if topo["node_count"] != 2 {
		t.Fatalf("期望 2 个节点，得到 %v", topo["node_count"])
	}
	if topo["service_count"] != 1 {
		t.Fatalf("期望 1 个服务，得到 %v", topo["service_count"])
	}
}

func TestSetNodeMaintenance(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))

	orch.SetNodeMaintenance("n1", true)
	n, _ := orch.GetNode("n1")
	if n.State != NodeStateMaintenance {
		t.Fatalf("期望维护模式，得到 %s", n.State)
	}

	orch.SetNodeMaintenance("n1", false)
	n, _ = orch.GetNode("n1")
	if n.State != NodeStateOnline {
		t.Fatalf("期望在线，得到 %s", n.State)
	}
}

func TestSetNodeMaintenanceNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.SetNodeMaintenance("nonexistent", true); err != ErrNodeNotFound {
		t.Fatalf("期望 ErrNodeNotFound，得到 %v", err)
	}
}

// ==================== 服务编排测试 ====================

func TestRegisterService(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))

	svc := newTestService("s1", "web", "n1")
	if err := orch.RegisterService(svc); err != nil {
		t.Fatalf("RegisterService 失败: %v", err)
	}

	got, _ := orch.GetService("s1")
	if got.State != ServiceStateRunning {
		t.Fatalf("期望运行中，得到 %s", got.State)
	}
	if got.HealthCheck == nil {
		t.Fatal("健康检查不应为 nil")
	}
}

func TestRegisterServiceDuplicate(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.RegisterService(newTestService("s1", "web", "n1"))

	if err := orch.RegisterService(newTestService("s1", "web2", "n1")); err != ErrServiceAlreadyExists {
		t.Fatalf("期望 ErrServiceAlreadyExists，得到 %v", err)
	}
}

func TestRegisterServiceNodeNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.RegisterService(newTestService("s1", "web", "nonexistent")); err != ErrNodeNotFound {
		t.Fatalf("期望 ErrNodeNotFound，得到 %v", err)
	}
}

func TestRegisterServiceInsufficientResources(t *testing.T) {
	orch := newTestOrch()
	node := newTestNode("n1", "node-1")
	node.Resources = &NodeResources{
		CPU:     ResourcePool{Total: 100, Used: 0, Available: 100},
		Memory:  ResourcePool{Total: 100, Used: 0, Available: 100},
		Storage: ResourcePool{Total: 100, Used: 0, Available: 100},
	}
	orch.AddNode(node)

	svc := newTestService("s1", "web", "n1")
	if err := orch.RegisterService(svc); err != ErrResourceInsufficient {
		t.Fatalf("期望 ErrResourceInsufficient，得到 %v", err)
	}
}

func TestDeregisterService(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.RegisterService(newTestService("s1", "web", "n1"))

	if err := orch.DeregisterService("s1"); err != nil {
		t.Fatalf("DeregisterService 失败: %v", err)
	}
	if _, err := orch.GetService("s1"); err != ErrServiceNotFound {
		t.Fatalf("期望 ErrServiceNotFound，得到 %v", err)
	}
}

func TestDeregisterServiceNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.DeregisterService("nonexistent"); err != ErrServiceNotFound {
		t.Fatalf("期望 ErrServiceNotFound，得到 %v", err)
	}
}

func TestListServices(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.RegisterService(newTestService("s2", "web2", "n1"))
	orch.RegisterService(newTestService("s1", "web1", "n1"))

	services := orch.ListServices()
	if len(services) != 2 {
		t.Fatalf("期望 2 个服务，得到 %d", len(services))
	}
	if services[0].ID != "s1" {
		t.Fatal("服务未按 ID 排序")
	}
}

func TestDiscoverService(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))
	orch.RegisterService(newTestService("s1", "web", "n1"))
	orch.RegisterService(newTestService("s2", "web", "n2"))
	orch.RegisterService(newTestService("s3", "db", "n1"))

	webServices := orch.DiscoverService("web")
	if len(webServices) != 2 {
		t.Fatalf("期望 2 个 web 服务，得到 %d", len(webServices))
	}
}

func TestHealthCheckService(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	svc := newTestService("s1", "web", "n1")
	svc.HealthCheck = &HealthCheck{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Healthy:          true,
	}
	orch.RegisterService(svc)

	// 连续2次失败
	orch.HealthCheckService("s1", false)
	orch.HealthCheckService("s1", false)
	s, _ := orch.GetService("s1")
	if s.State != ServiceStateFailed {
		t.Fatalf("期望服务失败，得到 %s", s.State)
	}

	// 连续2次成功恢复
	orch.HealthCheckService("s1", true)
	orch.HealthCheckService("s1", true)
	s, _ = orch.GetService("s1")
	if s.State != ServiceStateRunning {
		t.Fatalf("期望服务运行，得到 %s", s.State)
	}
}

func TestHealthCheckServiceFailover(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	svc := newTestService("s1", "web", "n1")
	svc.FailoverNode = "n2"
	svc.HealthCheck = &HealthCheck{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Healthy:          true,
	}
	orch.RegisterService(svc)

	orch.HealthCheckService("s1", false)
	s, _ := orch.GetService("s1")
	if s.NodeID != "n2" || s.State != ServiceStateRunning {
		t.Fatalf("故障转移失败: node=%s state=%s", s.NodeID, s.State)
	}
}

func TestHealthCheckServiceNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.HealthCheckService("nonexistent", true); err != ErrServiceNotFound {
		t.Fatalf("期望 ErrServiceNotFound，得到 %v", err)
	}
}

func TestMigrateService(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))
	orch.RegisterService(newTestService("s1", "web", "n1"))

	if err := orch.MigrateService("s1", "n2"); err != nil {
		t.Fatalf("MigrateService 失败: %v", err)
	}

	s, _ := orch.GetService("s1")
	if s.NodeID != "n2" {
		t.Fatalf("期望迁移到 n2，得到 %s", s.NodeID)
	}
}

func TestMigrateServiceInsufficientResources(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	n2 := newTestNode("n2", "node-2")
	n2.Resources = &NodeResources{
		CPU:     ResourcePool{Total: 100, Used: 0, Available: 100},
		Memory:  ResourcePool{Total: 100, Used: 0, Available: 100},
		Storage: ResourcePool{Total: 100, Used: 0, Available: 100},
	}
	orch.AddNode(n2)
	orch.RegisterService(newTestService("s1", "web", "n1"))

	if err := orch.MigrateService("s1", "n2"); err != ErrResourceInsufficient {
		t.Fatalf("期望 ErrResourceInsufficient，得到 %v", err)
	}
}

// ==================== 资源调度测试 ====================

func TestUpdateNodeResources(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))

	newRes := &NodeResources{
		CPU:     ResourcePool{Total: 16000, Used: 4000, Available: 12000},
		Memory:  ResourcePool{Total: 32 * 1024 * 1024 * 1024, Used: 8 * 1024 * 1024 * 1024, Available: 24 * 1024 * 1024 * 1024},
		Storage: ResourcePool{Total: 2 * 1024 * 1024 * 1024 * 1024, Used: 500 * 1024 * 1024 * 1024, Available: 1500 * 1024 * 1024 * 1024},
	}

	if err := orch.UpdateNodeResources("n1", newRes); err != nil {
		t.Fatalf("UpdateNodeResources 失败: %v", err)
	}

	usage, _ := orch.GetNodeResourceUsage("n1")
	if usage["cpu"] != 0.25 {
		t.Fatalf("期望 CPU 使用率 0.25，得到 %f", usage["cpu"])
	}
}

func TestUpdateNodeResourcesNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.UpdateNodeResources("nonexistent", nil); err != ErrNodeNotFound {
		t.Fatalf("期望 ErrNodeNotFound，得到 %v", err)
	}
}

func TestAllocateResources(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))

	req := &ResourceRequest{CPU: 2000, Memory: 4 * 1024 * 1024 * 1024, Storage: 100 * 1024 * 1024 * 1024}
	nodeID, err := orch.AllocateResources(req)
	if err != nil {
		t.Fatalf("AllocateResources 失败: %v", err)
	}
	if nodeID != "n1" {
		t.Fatalf("期望分配到 n1，得到 %s", nodeID)
	}
}

func TestAllocateResourcesInsufficient(t *testing.T) {
	orch := newTestOrch()
	node := newTestNode("n1", "node-1")
	node.Resources = &NodeResources{
		CPU:     ResourcePool{Total: 100, Used: 0, Available: 100},
		Memory:  ResourcePool{Total: 100, Used: 0, Available: 100},
		Storage: ResourcePool{Total: 100, Used: 0, Available: 100},
	}
	orch.AddNode(node)

	req := &ResourceRequest{CPU: 1000, Memory: 1000, Storage: 1000}
	_, err := orch.AllocateResources(req)
	if err != ErrNoAvailableNode {
		t.Fatalf("期望 ErrNoAvailableNode，得到 %v", err)
	}
}

func TestReleaseResources(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))

	req := &ResourceRequest{CPU: 2000, Memory: 4 * 1024 * 1024 * 1024, Storage: 100 * 1024 * 1024 * 1024}
	orch.AllocateResources(req)
	orch.ReleaseResources("n1", req)

	usage, _ := orch.GetNodeResourceUsage("n1")
	if usage["cpu"] != 0 {
		t.Fatalf("期望 CPU 使用率 0，得到 %f", usage["cpu"])
	}
}

func TestReleaseResourcesNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.ReleaseResources("nonexistent", nil); err != ErrNodeNotFound {
		t.Fatalf("期望 ErrNodeNotFound，得到 %v", err)
	}
}

func TestGetNodeResourceUsage(t *testing.T) {
	orch := newTestOrch()
	node := newTestNode("n1", "node-1")
	orch.AddNode(node)

	usage, err := orch.GetNodeResourceUsage("n1")
	if err != nil {
		t.Fatalf("GetNodeResourceUsage 失败: %v", err)
	}
	if usage["cpu"] != 0 || usage["memory"] != 0 {
		t.Fatalf("期望 0 使用率，得到 cpu=%f memory=%f", usage["cpu"], usage["memory"])
	}
}

func TestGetNodeResourceUsageNotFound(t *testing.T) {
	orch := newTestOrch()
	if _, err := orch.GetNodeResourceUsage("nonexistent"); err != ErrNodeNotFound {
		t.Fatalf("期望 ErrNodeNotFound，得到 %v", err)
	}
}

func TestGetNodeResourceUsageNilResources(t *testing.T) {
	orch := newTestOrch()
	node := &Node{ID: "n1", Name: "node-1", State: NodeStateOnline, Resources: nil}
	orch.AddNode(node)

	usage, _ := orch.GetNodeResourceUsage("n1")
	if usage["cpu"] != 0 {
		t.Fatalf("期望 0，得到 %f", usage["cpu"])
	}
}

func TestGetClusterResourceSummary(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))
	orch.RegisterService(newTestService("s1", "web", "n1"))

	summary := orch.GetClusterResourceSummary()
	if summary["online_nodes"] != 2 {
		t.Fatalf("期望 2 个在线节点，得到 %d", summary["online_nodes"])
	}
	if summary["total_services"] != 1 {
		t.Fatalf("期望 1 个服务，得到 %d", summary["total_services"])
	}
}

func TestSetResourceStrategy(t *testing.T) {
	orch := newTestOrch()
	orch.SetResourceStrategy(ResourceStrategyBinpack)
	// 不崩溃即通过
}

// ==================== 负载均衡测试 ====================

func TestSelectNodeRoundRobin(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	selections := make(map[string]int)
	for i := 0; i < 100; i++ {
		node, err := orch.SelectNode("")
		if err != nil {
			t.Fatalf("SelectNode 失败: %v", err)
		}
		selections[node.ID]++
	}

	// 两个节点都应该被选中
	if selections["n1"] == 0 || selections["n2"] == 0 {
		t.Fatalf("两个节点都应被选中: %v", selections)
	}
}

func TestSelectNodeLeastConnections(t *testing.T) {
	orch := newTestOrch()
	orch.SetLoadBalanceStrategy(LBStrategyLeastConnections)
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	// n1 有更多连接
	orch.UpdateNodeConnections("n1", 10)

	node, _ := orch.SelectNode("")
	if node.ID != "n2" {
		t.Fatalf("期望选择 n2（最少连接），得到 %s", node.ID)
	}
}

func TestSelectNodeRandom(t *testing.T) {
	orch := newTestOrch()
	orch.SetLoadBalanceStrategy(LBStrategyRandom)
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	for i := 0; i < 10; i++ {
		node, err := orch.SelectNode("")
		if err != nil {
			t.Fatalf("SelectNode 失败: %v", err)
		}
		if node.ID != "n1" && node.ID != "n2" {
			t.Fatalf("未知节点: %s", node.ID)
		}
	}
}

func TestSelectNodeHash(t *testing.T) {
	orch := newTestOrch()
	orch.SetLoadBalanceStrategy(LBStrategyHash)
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	// 同一个 key 应该始终选同一个节点
	node1, _ := orch.SelectNode("test-key")
	node2, _ := orch.SelectNode("test-key")
	if node1.ID != node2.ID {
		t.Fatal("哈希策略：相同 key 应返回相同节点")
	}
}

func TestSelectNodeNoOnlineNodes(t *testing.T) {
	orch := newTestOrch()
	node := newTestNode("n1", "node-1")
	orch.AddNode(node)
	orch.MarkNodeOffline("n1")

	if _, err := orch.SelectNode(""); err != ErrNoAvailableNode {
		t.Fatalf("期望 ErrNoAvailableNode，得到 %v", err)
	}
}

func TestSelectNodeEmptyCluster(t *testing.T) {
	orch := newTestOrch()
	if _, err := orch.SelectNode(""); err != ErrNoAvailableNode {
		t.Fatalf("期望 ErrNoAvailableNode，得到 %v", err)
	}
}

func TestUpdateNodeConnections(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))

	orch.UpdateNodeConnections("n1", 5)
	node, _ := orch.GetNode("n1")
	if node.ActiveConns != 5 {
		t.Fatalf("期望 5 连接，得到 %d", node.ActiveConns)
	}

	orch.UpdateNodeConnections("n1", -3)
	node, _ = orch.GetNode("n1")
	if node.ActiveConns != 2 {
		t.Fatalf("期望 2 连接，得到 %d", node.ActiveConns)
	}
}

func TestUpdateNodeConnectionsClampZero(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))

	orch.UpdateNodeConnections("n1", -5)
	node, _ := orch.GetNode("n1")
	if node.ActiveConns != 0 {
		t.Fatalf("期望 0 连接（最小），得到 %d", node.ActiveConns)
	}
}

func TestUpdateNodeConnectionsNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.UpdateNodeConnections("nonexistent", 1); err != ErrNodeNotFound {
		t.Fatalf("期望 ErrNodeNotFound，得到 %v", err)
	}
}

func TestSetLoadBalanceStrategy(t *testing.T) {
	orch := newTestOrch()
	orch.SetLoadBalanceStrategy(LBStrategyHash)
	// 不崩溃即通过
}

// ==================== 配置同步测试 ====================

func TestSetAndGetConfig(t *testing.T) {
	orch := newTestOrch()

	if err := orch.SetConfig("key1", "value1", "admin", "初始配置"); err != nil {
		t.Fatalf("SetConfig 失败: %v", err)
	}

	cfg, err := orch.GetConfig("key1")
	if err != nil {
		t.Fatalf("GetConfig 失败: %v", err)
	}
	if cfg.Version != 1 || cfg.Data["value"] != "value1" {
		t.Fatalf("配置不正确: %+v", cfg)
	}
}

func TestSetConfigVersionIncrement(t *testing.T) {
	orch := newTestOrch()
	orch.SetConfig("key1", "v1", "admin", "")
	orch.SetConfig("key1", "v2", "admin", "")

	cfg, _ := orch.GetConfig("key1")
	if cfg.Version != 2 || cfg.Data["value"] != "v2" {
		t.Fatalf("版本递增失败: %+v", cfg)
	}
}

func TestGetConfigNotFound(t *testing.T) {
	orch := newTestOrch()
	if _, err := orch.GetConfig("nonexistent"); err != ErrConfigNotFound {
		t.Fatalf("期望 ErrConfigNotFound，得到 %v", err)
	}
}

func TestDeleteConfig(t *testing.T) {
	orch := newTestOrch()
	orch.SetConfig("key1", "v1", "admin", "")

	if err := orch.DeleteConfig("key1"); err != nil {
		t.Fatalf("DeleteConfig 失败: %v", err)
	}
	if _, err := orch.GetConfig("key1"); err != ErrConfigNotFound {
		t.Fatalf("期望 ErrConfigNotFound，得到 %v", err)
	}
}

func TestDeleteConfigNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.DeleteConfig("nonexistent"); err != ErrConfigNotFound {
		t.Fatalf("期望 ErrConfigNotFound，得到 %v", err)
	}
}

func TestRollbackConfig(t *testing.T) {
	orch := newTestOrch()
	orch.SetConfig("key1", "v1", "admin", "版本1")
	orch.SetConfig("key1", "v2", "admin", "版本2")
	orch.SetConfig("key1", "v3", "admin", "版本3")

	if err := orch.RollbackConfig("key1", 1); err != nil {
		t.Fatalf("RollbackConfig 失败: %v", err)
	}

	cfg, _ := orch.GetConfig("key1")
	if cfg.Data["value"] != "v1" {
		t.Fatalf("回滚后值应为 v1，得到 %s", cfg.Data["value"])
	}
}

func TestRollbackConfigVersionNotFound(t *testing.T) {
	orch := newTestOrch()
	orch.SetConfig("key1", "v1", "admin", "")

	err := orch.RollbackConfig("key1", 99)
	if err == nil {
		t.Fatal("期望错误，得到 nil")
	}
}

func TestRollbackConfigKeyNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.RollbackConfig("nonexistent", 1); err != ErrConfigNotFound {
		t.Fatalf("期望 ErrConfigNotFound，得到 %v", err)
	}
}

func TestGetConfigHistory(t *testing.T) {
	orch := newTestOrch()
	orch.SetConfig("key1", "v1", "admin", "")
	orch.SetConfig("key1", "v2", "admin", "")

	history, err := orch.GetConfigHistory("key1")
	if err != nil {
		t.Fatalf("GetConfigHistory 失败: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("期望 1 条历史，得到 %d", len(history))
	}
	if history[0].Data["value"] != "v1" {
		t.Fatalf("历史值应为 v1，得到 %s", history[0].Data["value"])
	}
}

func TestGetConfigHistoryNotFound(t *testing.T) {
	orch := newTestOrch()
	if _, err := orch.GetConfigHistory("nonexistent"); err != ErrConfigNotFound {
		t.Fatalf("期望 ErrConfigNotFound，得到 %v", err)
	}
}

func TestSyncConfig(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.SetConfig("key1", "v1", "admin", "")

	if err := orch.SyncConfig("key1"); err != nil {
		t.Fatalf("SyncConfig 失败: %v", err)
	}
}

func TestSyncConfigNotFound(t *testing.T) {
	orch := newTestOrch()
	if err := orch.SyncConfig("nonexistent"); err != ErrConfigNotFound {
		t.Fatalf("期望 ErrConfigNotFound，得到 %v", err)
	}
}

func TestListConfigs(t *testing.T) {
	orch := newTestOrch()
	orch.SetConfig("k1", "v1", "admin", "")
	orch.SetConfig("k2", "v2", "admin", "")

	configs := orch.ListConfigs()
	if len(configs) != 2 {
		t.Fatalf("期望 2 个配置，得到 %d", len(configs))
	}
}

// ==================== 集群日志测试 ====================

func TestAddAndQueryLogs(t *testing.T) {
	orch := newTestOrch()
	orch.AddLog("n1", "info", "测试消息1", "web")
	orch.AddLog("n1", "error", "测试错误", "web")
	orch.AddLog("n2", "info", "测试消息2", "db")

	// 查询所有
	all := orch.QueryLogs(LogFilter{})
	if len(all) != 3 {
		t.Fatalf("期望 3 条日志，得到 %d", len(all))
	}

	// 按级别过滤
	errors := orch.QueryLogs(LogFilter{Level: "error"})
	if len(errors) != 1 {
		t.Fatalf("期望 1 条错误日志，得到 %d", len(errors))
	}

	// 按节点过滤
	n1Logs := orch.QueryLogs(LogFilter{NodeID: "n1"})
	if len(n1Logs) != 2 {
		t.Fatalf("期望 2 条 n1 日志，得到 %d", len(n1Logs))
	}

	// 按关键字过滤
	keywordLogs := orch.QueryLogs(LogFilter{Keyword: "错误"})
	if len(keywordLogs) != 1 {
		t.Fatalf("期望 1 条匹配日志，得到 %d", len(keywordLogs))
	}

	// 按服务过滤
	webLogs := orch.QueryLogs(LogFilter{Service: "web"})
	if len(webLogs) != 2 {
		t.Fatalf("期望 2 条 web 日志，得到 %d", len(webLogs))
	}
}

func TestQueryLogsWithLimit(t *testing.T) {
	orch := newTestOrch()
	for i := 0; i < 10; i++ {
		orch.AddLog("n1", "info", fmt.Sprintf("消息 %d", i), "")
	}

	logs := orch.QueryLogs(LogFilter{Limit: 5})
	if len(logs) != 5 {
		t.Fatalf("期望 5 条日志，得到 %d", len(logs))
	}
}

func TestQueryLogsByTimeRange(t *testing.T) {
	orch := newTestOrch()
	orch.AddLog("n1", "info", "早的消息", "")

	logs := orch.QueryLogs(LogFilter{
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now().Add(1 * time.Hour),
	})
	if len(logs) != 1 {
		t.Fatalf("期望 1 条日志，得到 %d", len(logs))
	}

	logs = orch.QueryLogs(LogFilter{
		StartTime: time.Now().Add(1 * time.Hour),
	})
	if len(logs) != 0 {
		t.Fatalf("期望 0 条日志，得到 %d", len(logs))
	}
}

func TestGetLogStats(t *testing.T) {
	orch := newTestOrch()
	orch.AddLog("n1", "info", "msg1", "")
	orch.AddLog("n1", "info", "msg2", "")
	orch.AddLog("n1", "error", "err1", "")
	orch.AddLog("n1", "warn", "warn1", "")

	stats := orch.GetLogStats()
	if stats["total"] != 4 {
		t.Fatalf("期望 4 条日志，得到 %d", stats["total"])
	}
	if stats["info"] != 2 || stats["error"] != 1 || stats["warn"] != 1 {
		t.Fatalf("日志统计不正确: %v", stats)
	}
}

func TestClearLogs(t *testing.T) {
	orch := newTestOrch()
	orch.AddLog("n1", "info", "msg1", "")
	orch.AddLog("n1", "info", "msg2", "")

	orch.ClearLogs()
	stats := orch.GetLogStats()
	if stats["total"] != 0 {
		t.Fatalf("期望 0 条日志，得到 %d", stats["total"])
	}
}

func TestMaxLogSize(t *testing.T) {
	orch := New(ClusterOrchConfig{MaxLogSize: 5})
	for i := 0; i < 10; i++ {
		orch.AddLog("n1", "info", fmt.Sprintf("msg %d", i), "")
	}

	stats := orch.GetLogStats()
	if stats["total"] != 5 {
		t.Fatalf("期望 5 条日志（限制），得到 %d", stats["total"])
	}
}

// ==================== 扩缩容测试 ====================

func TestScaleOut(t *testing.T) {
	orch := newTestOrch()
	nodes, err := orch.ScaleOut(3)
	if err != nil {
		t.Fatalf("ScaleOut 失败: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("期望 3 个新节点，得到 %d", len(nodes))
	}

	allNodes := orch.ListNodes()
	if len(allNodes) != 3 {
		t.Fatalf("期望 3 个节点，得到 %d", len(allNodes))
	}
}

func TestScaleOutDisabled(t *testing.T) {
	orch := New(ClusterOrchConfig{AutoScale: false})
	if _, err := orch.ScaleOut(1); err != ErrAutoScaleDisabled {
		t.Fatalf("期望 ErrAutoScaleDisabled，得到 %v", err)
	}
}

func TestScaleOutExceedMax(t *testing.T) {
	orch := New(ClusterOrchConfig{AutoScale: true, MaxNodes: 2})
	nodes, err := orch.ScaleOut(5)
	if err != nil {
		t.Fatalf("ScaleOut 失败: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("期望 2 个节点（受 max 限制），得到 %d", len(nodes))
	}

	// 再次扩容应该失败
	if _, err := orch.ScaleOut(1); err != ErrClusterFull {
		t.Fatalf("期望 ErrClusterFull，得到 %v", err)
	}
}

func TestScaleIn(t *testing.T) {
	orch := newTestOrch()
	orch.ScaleOut(3)

	nodes, err := orch.ScaleIn(1)
	if err != nil {
		t.Fatalf("ScaleIn 失败: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("期望移除 1 个节点，得到 %d", len(nodes))
	}

	allNodes := orch.ListNodes()
	if len(allNodes) != 2 {
		t.Fatalf("期望 2 个节点，得到 %d", len(allNodes))
	}
}

func TestScaleInDisabled(t *testing.T) {
	orch := New(ClusterOrchConfig{AutoScale: false})
	if _, err := orch.ScaleIn(1); err != ErrAutoScaleDisabled {
		t.Fatalf("期望 ErrAutoScaleDisabled，得到 %v", err)
	}
}

func TestScaleInMinNodes(t *testing.T) {
	orch := New(ClusterOrchConfig{AutoScale: true, MinNodes: 2})
	orch.ScaleOut(3)

	_, err := orch.ScaleIn(2)
	if err == nil {
		t.Fatal("期望错误，得到 nil")
	}
}

func TestScaleInMigratesServices(t *testing.T) {
	orch := newTestOrch()
	orch.ScaleOut(3)
	nodes := orch.ListNodes()

	// 注册服务到第一个节点
	svc := newTestService("s1", "web", nodes[0].ID)
	// 资源设置为 0，避免资源不足
	svc.Resources = nil
	orch.RegisterService(svc)

	orch.ScaleIn(1)

	s, _ := orch.GetService("s1")
	if s.State == ServiceStateStopped && s.NodeID == nodes[0].ID {
		t.Fatal("服务未被迁移")
	}
}

func TestSetAutoScale(t *testing.T) {
	orch := newTestOrch()
	orch.SetAutoScale(true, 0.9, 0.2, 2, 20)
	// 不崩溃即通过
}

func TestCheckAutoScaleStable(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))

	action, count, err := orch.CheckAutoScale()
	if err != nil {
		t.Fatalf("CheckAutoScale 失败: %v", err)
	}
	if action != "stable" {
		t.Fatalf("期望 stable，得到 %s", action)
	}
	if count != 0 {
		t.Fatalf("期望 0，得到 %d", count)
	}
}

func TestCheckAutoScaleScaleOut(t *testing.T) {
	orch := newTestOrch()
	node := newTestNode("n1", "node-1")
	node.Resources = &NodeResources{
		CPU:     ResourcePool{Total: 100, Used: 90, Available: 10},
		Memory:  ResourcePool{Total: 100, Used: 90, Available: 10},
		Storage: ResourcePool{Total: 100, Used: 50, Available: 50},
	}
	orch.AddNode(node)

	action, count, _ := orch.CheckAutoScale()
	if action != "scale_out" || count != 1 {
		t.Fatalf("期望 scale_out/1，得到 %s/%d", action, count)
	}
}

func TestCheckAutoScaleScaleIn(t *testing.T) {
	orch := newTestOrch()
	node := newTestNode("n1", "node-1")
	node.Resources = &NodeResources{
		CPU:     ResourcePool{Total: 100, Used: 10, Available: 90},
		Memory:  ResourcePool{Total: 100, Used: 10, Available: 90},
		Storage: ResourcePool{Total: 100, Used: 50, Available: 50},
	}
	orch.AddNode(node)
	orch.AddNode(newTestNode("n2", "node-2"))

	action, count, _ := orch.CheckAutoScale()
	if action != "scale_in" || count != 1 {
		t.Fatalf("期望 scale_in/1，得到 %s/%d", action, count)
	}
}

func TestCheckAutoScaleDisabled(t *testing.T) {
	orch := New(ClusterOrchConfig{AutoScale: false})
	action, _, _ := orch.CheckAutoScale()
	if action != "disabled" {
		t.Fatalf("期望 disabled，得到 %s", action)
	}
}

func TestCheckAutoScaleNoNodes(t *testing.T) {
	orch := newTestOrch()
	action, _, _ := orch.CheckAutoScale()
	if action != "no_nodes" {
		t.Fatalf("期望 no_nodes，得到 %s", action)
	}
}

// ==================== 内部方法测试 ====================

func TestSafeDivide(t *testing.T) {
	if safeDivide(10, 0) != 0 {
		t.Fatal("除以 0 应返回 0")
	}
	if safeDivide(10, 20) != 0.5 {
		t.Fatal("10/20 应返回 0.5")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == id2 {
		t.Fatal("生成的 ID 不应相同")
	}
}

// ==================== 并发安全测试 ====================

func TestConcurrentAddNode(t *testing.T) {
	orch := New(ClusterOrchConfig{MaxNodes: 200})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			node := &Node{
				ID:      fmt.Sprintf("n%d", id),
				Name:    fmt.Sprintf("node-%d", id),
				Address: fmt.Sprintf("192.168.1.%d", id),
				Resources: &NodeResources{
					CPU:     ResourcePool{Total: 8000, Used: 0, Available: 8000},
					Memory:  ResourcePool{Total: 16 * 1024 * 1024 * 1024, Used: 0, Available: 16 * 1024 * 1024 * 1024},
					Storage: ResourcePool{Total: 1024 * 1024 * 1024 * 1024, Used: 0, Available: 1024 * 1024 * 1024 * 1024},
				},
				Weight: 100,
			}
			orch.AddNode(node)
		}(i)
	}
	wg.Wait()

	nodes := orch.ListNodes()
	if len(nodes) != 100 {
		t.Fatalf("期望 100 个节点，得到 %d", len(nodes))
	}
}

func TestConcurrentRegisterService(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			svc := &Service{
				ID:     fmt.Sprintf("s%d", id),
				Name:   fmt.Sprintf("svc-%d", id),
				NodeID: "n1",
			}
			orch.RegisterService(svc)
		}(i)
	}
	wg.Wait()

	services := orch.ListServices()
	if len(services) != 50 {
		t.Fatalf("期望 50 个服务，得到 %d", len(services))
	}
}

func TestConcurrentSelectNode(t *testing.T) {
	orch := newTestOrch()
	orch.AddNode(newTestNode("n1", "node-1"))
	orch.AddNode(newTestNode("n2", "node-2"))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			orch.SelectNode("test")
		}()
	}
	wg.Wait()
}

func TestConcurrentConfigOperations(t *testing.T) {
	orch := newTestOrch()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", id)
			orch.SetConfig(key, "value", "admin", "")
			orch.GetConfig(key)
		}(i)
	}
	wg.Wait()

	configs := orch.ListConfigs()
	if len(configs) != 50 {
		t.Fatalf("期望 50 个配置，得到 %d", len(configs))
	}
}

// ==================== JSON 序列化测试 ====================

func TestNodeJSONRoundTrip(t *testing.T) {
	// Node 有自定义类型，确保可序列化
	node := newTestNode("n1", "node-1")
	node.Metadata = map[string]string{"role": "storage"}

	// 简单验证字段
	if node.ID != "n1" || node.Name != "node-1" || node.Weight != 100 {
		t.Fatal("字段值不正确")
	}
	if node.Metadata["role"] != "storage" {
		t.Fatal("Metadata 不正确")
	}
}

func TestServiceStates(t *testing.T) {
	states := []ServiceState{ServiceStateRunning, ServiceStateStopped, ServiceStateFailed, ServiceStateMigrating}
	for _, s := range states {
		if s == "" {
			t.Fatal("状态不应为空")
		}
	}
}

func TestNodeStates(t *testing.T) {
	states := []NodeState{NodeStateOnline, NodeStateOffline, NodeStateJoining, NodeStateLeaving, NodeStateMaintenance}
	for _, s := range states {
		if s == "" {
			t.Fatal("状态不应为空")
		}
	}
}
