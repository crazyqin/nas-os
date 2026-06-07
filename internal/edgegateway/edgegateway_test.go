package edgegateway

import (
	"testing"
	"time"
)

func TestEdgeGateway_RegisterNode(t *testing.T) {
	eg := NewEdgeGateway(nil)

	node := &EdgeNode{
		ID:       "node-1",
		Name:     "边缘网关 A",
		Type:     NodeTypeGateway,
		Location: Location{Latitude: 39.9, Longitude: 116.3, City: "北京", Country: "中国"},
		Tags:     []string{"production", "gateway"},
	}

	err := eg.RegisterNode(node)
	if err != nil {
		t.Fatalf("Failed to register node: %v", err)
	}

	// 验证节点已注册
	registered, err := eg.GetNode("node-1")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	if registered.Name != "边缘网关 A" {
		t.Errorf("Expected name '边缘网关 A', got '%s'", registered.Name)
	}
}

func TestEdgeGateway_SubmitTask(t *testing.T) {
	eg := NewEdgeGateway(nil)

	// 注册节点
	eg.RegisterNode(&EdgeNode{
		ID:   "node-1",
		Name: "计算节点",
		Type: NodeTypeCompute,
	})

	// 提交任务
	task := &EdgeTask{
		ID:       "task-1",
		Name:     "数据处理",
		Type:     "compute",
		Priority: 1,
		Input:    map[string]string{"data": "test"},
	}

	err := eg.SubmitTask(task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// 获取任务状态
	fetched, err := eg.GetTask("task-1")
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if fetched.Status != "pending" && fetched.Status != "queued" {
		t.Errorf("Expected status 'pending' or 'queued', got '%s'", fetched.Status)
	}
}

func TestEdgeGateway_Cache(t *testing.T) {
	eg := NewEdgeGateway(&GatewayConfig{
		CacheSizeMB:     10,
		CacheTTLMinutes: 5,
	})

	// 设置缓存
	eg.CacheSet("key1", "value1", 5*time.Minute)

	// 获取缓存
	val, found := eg.CacheGet("key1")
	if !found {
		t.Fatal("Expected to find cached value")
	}
	if val != "value1" {
		t.Errorf("Expected 'value1', got '%v'", val)
	}

	// 删除缓存
	eg.CacheDelete("key1")
	_, found = eg.CacheGet("key1")
	if found {
		t.Error("Expected cache to be deleted")
	}
}

func TestEdgeGateway_Route(t *testing.T) {
	eg := NewEdgeGateway(nil)

	route := &EdgeRoute{
		ID:          "route-1",
		Source:      "node-1",
		Destination: "cloud",
		NextHop:     "gateway-1",
		Metric:      10,
		Policy:      PolicyLatency,
		Enabled:     true,
	}

	err := eg.AddRoute(route)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// 删除路由
	err = eg.RemoveRoute("route-1")
	if err != nil {
		t.Fatalf("Failed to remove route: %v", err)
	}
}

func TestEdgeGateway_Stats(t *testing.T) {
	eg := NewEdgeGateway(nil)

	// 注册节点
	eg.RegisterNode(&EdgeNode{
		ID:      "node-1",
		Name:    "节点1",
		Type:    NodeTypeGateway,
		Latency: 10,
	})
	eg.RegisterNode(&EdgeNode{
		ID:      "node-2",
		Name:    "节点2",
		Type:    NodeTypeCompute,
		Latency: 20,
	})

	stats := eg.GetStats()

	if stats.TotalNodes != 2 {
		t.Errorf("Expected 2 nodes, got %d", stats.TotalNodes)
	}

	if stats.OnlineNodes != 2 {
		t.Errorf("Expected 2 online nodes, got %d", stats.OnlineNodes)
	}
}

func TestEdgeGateway_SyncNodes(t *testing.T) {
	eg := NewEdgeGateway(nil)

	// 注册节点
	eg.RegisterNode(&EdgeNode{ID: "node-1", Name: "源节点"})
	eg.RegisterNode(&EdgeNode{ID: "node-2", Name: "目标节点"})

	sync, err := eg.SyncNodes("node-1", "node-2")
	if err != nil {
		t.Fatalf("Failed to sync nodes: %v", err)
	}

	if sync.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", sync.Status)
	}
}

func TestEdgeGateway_UnregisterNode(t *testing.T) {
	eg := NewEdgeGateway(nil)

	eg.RegisterNode(&EdgeNode{ID: "node-1", Name: "节点"})
	err := eg.UnregisterNode("node-1")
	if err != nil {
		t.Fatalf("Failed to unregister node: %v", err)
	}

	_, err = eg.GetNode("node-1")
	if err == nil {
		t.Error("Expected error for unregistered node")
	}
}

func TestEdgeNodeType_Constants(t *testing.T) {
	types := []EdgeNodeType{
		NodeTypeGateway, NodeTypeCompute, NodeTypeStorage,
		NodeTypeSensor, NodeTypeHybrid,
	}

	for _, nt := range types {
		if nt == "" {
			t.Error("EdgeNodeType constant should not be empty")
		}
	}
}

func TestNodeStatus_Constants(t *testing.T) {
	statuses := []NodeStatus{
		StatusOnline, StatusOffline, StatusDegraded,
		StatusSyncing, StatusError,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("NodeStatus constant should not be empty")
		}
	}
}

func TestEdgePolicy_Constants(t *testing.T) {
	policies := []EdgePolicy{
		PolicyLocalFirst, PolicyCloudFirst, PolicyBalanced,
		PolicyCostOptimal, PolicyLatency,
	}

	for _, p := range policies {
		if p == "" {
			t.Error("EdgePolicy constant should not be empty")
		}
	}
}

func TestEdgeGateway_MarshalJSON(t *testing.T) {
	eg := NewEdgeGateway(nil)

	data, err := eg.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON")
	}
}

func TestEdgeGateway_GetOnlineNodes(t *testing.T) {
	eg := NewEdgeGateway(nil)

	eg.RegisterNode(&EdgeNode{ID: "n1", Status: StatusOnline})
	eg.RegisterNode(&EdgeNode{ID: "n2"})
	eg.nodes["n2"].Status = StatusOffline
	eg.RegisterNode(&EdgeNode{ID: "n3", Status: StatusOnline})

	nodes := eg.GetOnlineNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 online nodes, got %d", len(nodes))
	}
}

func TestEdgeGateway_GetNodesByType(t *testing.T) {
	eg := NewEdgeGateway(nil)

	eg.RegisterNode(&EdgeNode{ID: "n1", Type: NodeTypeGateway})
	eg.RegisterNode(&EdgeNode{ID: "n2", Type: NodeTypeCompute})
	eg.RegisterNode(&EdgeNode{ID: "n3", Type: NodeTypeGateway})

	nodes := eg.GetNodesByType(NodeTypeGateway)
	if len(nodes) != 2 {
		t.Errorf("Expected 2 gateway nodes, got %d", len(nodes))
	}
}

func TestCacheEntry_Expiration(t *testing.T) {
	eg := NewEdgeGateway(&GatewayConfig{
		CacheTTLMinutes: 1,
	})

	// 设置短期缓存
	eg.CacheSet("short-lived", "data", 100*time.Millisecond)

	// 立即获取
	_, found := eg.CacheGet("short-lived")
	if !found {
		t.Error("Expected to find cached value immediately")
	}

	// 等待过期
	time.Sleep(200 * time.Millisecond)
	_, found = eg.CacheGet("short-lived")
	if found {
		t.Error("Expected cache to be expired")
	}
}
