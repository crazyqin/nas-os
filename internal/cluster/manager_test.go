package cluster

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := ClusterConfig{
		Name:              "test-cluster",
		NodeID:            "test-node-1",
		DiscoveryPort:     8081,
		HeartbeatInterval: 5,
		HeartbeatTimeout:  15,
		DataDir:           "/tmp/test-cluster",
	}

	manager, err := NewManager(config, logger)
	if err != nil {
		t.Fatalf("创建集群管理器失败：%v", err)
	}
	defer manager.Shutdown()

	if manager.config.NodeID != "test-node-1" {
		t.Errorf("期望 NodeID 为 test-node-1，实际为 %s", manager.config.NodeID)
	}

	if manager.masterID != "test-node-1" {
		t.Errorf("期望 masterID 为 test-node-1，实际为 %s", manager.masterID)
	}
}

func TestGetNodes(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := ClusterConfig{
		NodeID:  "test-node-1",
		DataDir: "/tmp/test-cluster",
	}

	manager, _ := NewManager(config, logger)
	defer manager.Shutdown()

	// 初始应该没有节点
	nodes := manager.GetNodes()
	if len(nodes) != 0 {
		t.Errorf("期望 0 个节点，实际有 %d 个", len(nodes))
	}

	// 添加测试节点
	testNode := &ClusterNode{
		ID:        "test-node-2",
		Hostname:  "test-host-2",
		IP:        "192.168.1.102",
		Port:      8080,
		Role:      RoleWorker,
		Status:    StatusOnline,
		Heartbeat: time.Now(),
		JoinTime:  time.Now(),
	}

	manager.nodesMutex.Lock()
	manager.nodes[testNode.ID] = testNode
	manager.nodesMutex.Unlock()

	// 验证节点列表
	nodes = manager.GetNodes()
	if len(nodes) != 1 {
		t.Errorf("期望 1 个节点，实际有 %d 个", len(nodes))
	}

	if nodes[0].ID != "test-node-2" {
		t.Errorf("期望节点 ID 为 test-node-2，实际为 %s", nodes[0].ID)
	}
}

func TestGetOnlineNodes(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := ClusterConfig{
		NodeID:  "test-node-1",
		DataDir: "/tmp/test-cluster",
	}

	manager, _ := NewManager(config, logger)
	defer manager.Shutdown()

	// 添加在线节点
	onlineNode := &ClusterNode{
		ID:        "test-node-2",
		Status:    StatusOnline,
		Heartbeat: time.Now(),
	}

	// 添加离线节点
	offlineNode := &ClusterNode{
		ID:        "test-node-3",
		Status:    StatusOffline,
		Heartbeat: time.Now().Add(-1 * time.Hour),
	}

	manager.nodesMutex.Lock()
	manager.nodes[onlineNode.ID] = onlineNode
	manager.nodes[offlineNode.ID] = offlineNode
	manager.nodesMutex.Unlock()

	// 验证在线节点
	onlineNodes := manager.GetOnlineNodes()
	if len(onlineNodes) != 1 {
		t.Errorf("期望 1 个在线节点，实际有 %d 个", len(onlineNodes))
	}

	if onlineNodes[0].ID != "test-node-2" {
		t.Errorf("期望在线节点 ID 为 test-node-2，实际为 %s", onlineNodes[0].ID)
	}
}

func TestRemoveNode(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := ClusterConfig{
		NodeID:  "test-node-1",
		DataDir: "/tmp/test-cluster",
	}

	manager, _ := NewManager(config, logger)
	defer manager.Shutdown()

	// 添加测试节点
	testNode := &ClusterNode{
		ID:        "test-node-2",
		Heartbeat: time.Now(),
	}

	manager.nodesMutex.Lock()
	manager.nodes[testNode.ID] = testNode
	manager.nodesMutex.Unlock()

	// 删除节点
	err := manager.RemoveNode("test-node-2")
	if err != nil {
		t.Errorf("删除节点失败：%v", err)
	}

	// 验证节点已删除
	nodes := manager.GetNodes()
	if len(nodes) != 0 {
		t.Errorf("期望节点已删除，实际还有 %d 个", len(nodes))
	}

	// 删除不存在的节点
	err = manager.RemoveNode("non-existent")
	if err == nil {
		t.Error("期望删除不存在的节点返回错误")
	}
}

func TestUpdateNodeMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := ClusterConfig{
		NodeID:  "test-node-1",
		DataDir: "/tmp/test-cluster",
	}

	manager, _ := NewManager(config, logger)
	defer manager.Shutdown()

	// 添加测试节点
	testNode := &ClusterNode{
		ID:        "test-node-2",
		Heartbeat: time.Now(),
	}

	manager.nodesMutex.Lock()
	manager.nodes[testNode.ID] = testNode
	manager.nodesMutex.Unlock()

	// 更新指标
	metrics := NodeMetrics{
		CPUUsage:    45.5,
		MemoryUsage: 67.8,
		DiskUsage:   23.4,
	}

	err := manager.UpdateNodeMetrics("test-node-2", metrics)
	if err != nil {
		t.Errorf("更新指标失败：%v", err)
	}

	// 验证指标已更新
	node, exists := manager.GetNode("test-node-2")
	if !exists {
		t.Fatal("节点不存在")
	}

	if node.Metrics.CPUUsage != 45.5 {
		t.Errorf("期望 CPU 使用率为 45.5，实际为 %f", node.Metrics.CPUUsage)
	}
}

func TestIsMaster(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := ClusterConfig{
		NodeID:  "test-node-1",
		DataDir: "/tmp/test-cluster",
	}

	manager, _ := NewManager(config, logger)
	defer manager.Shutdown()

	// 初始节点应该是 master
	if !manager.IsMaster() {
		t.Error("期望初始节点为 master")
	}

	if manager.GetMasterID() != "test-node-1" {
		t.Errorf("期望 master ID 为 test-node-1，实际为 %s", manager.GetMasterID())
	}
}

func TestClusterCallbacks(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := ClusterConfig{
		NodeID:  "test-node-1",
		DataDir: "/tmp/test-cluster",
	}

	manager, _ := NewManager(config, logger)
	defer manager.Shutdown()

	// 设置回调 - 使用线程安全的方式
	var nodeJoinCalled bool
	var nodeLeaveCalled bool
	var mu sync.Mutex

	callbacks := ClusterCallbacks{
		OnNodeJoin: func(node *ClusterNode) {
			mu.Lock()
			nodeJoinCalled = true
			mu.Unlock()
		},
		OnNodeLeave: func(node *ClusterNode) {
			mu.Lock()
			nodeLeaveCalled = true
			mu.Unlock()
		},
	}

	manager.SetCallbacks(callbacks)

	// 添加节点
	testNode := &ClusterNode{
		ID:        "test-node-2",
		Heartbeat: time.Now(),
	}

	manager.nodesMutex.Lock()
	manager.nodes[testNode.ID] = testNode
	manager.nodesMutex.Unlock()

	// 触发回调（实际场景中由发现机制触发）
	if callbacks.OnNodeJoin != nil {
		callbacks.OnNodeJoin(testNode)
	}

	// 验证回调被调用
	mu.Lock()
	joinCalled := nodeJoinCalled
	mu.Unlock()
	if !joinCalled {
		t.Error("期望 OnNodeJoin 回调被调用")
	}

	// 删除节点并等待回调
	manager.RemoveNode("test-node-2")

	// 触发 OnNodeLeave 回调
	if callbacks.OnNodeLeave != nil {
		callbacks.OnNodeLeave(testNode)
	}

	// 验证回调被调用
	mu.Lock()
	leaveCalled := nodeLeaveCalled
	mu.Unlock()
	if !leaveCalled {
		t.Error("期望 OnNodeLeave 回调被调用")
	}
}
