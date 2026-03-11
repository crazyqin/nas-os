package cluster

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestEdgeNodeManager 测试边缘节点管理器
func TestEdgeNodeManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := EdgeNodeConfig{
		NodeID:           "test-edge-node",
		DataDir:          t.TempDir(),
		HeartbeatInterval: 10 * time.Second,
		HeartbeatTimeout:  30 * time.Second,
		MaxNodes:         100,
	}

	manager, err := NewEdgeNodeManager(config, logger, nil)
	if err != nil {
		t.Fatalf("创建边缘节点管理器失败: %v", err)
	}

	if err := manager.Initialize(); err != nil {
		t.Fatalf("初始化边缘节点管理器失败: %v", err)
	}
	defer manager.Shutdown()

	// 测试注册节点
	node := &EdgeNode{
		ID:        "edge-001",
		Name:      "测试节点",
		Type:      EdgeNodeTypeCompute,
		IPAddress: "192.168.1.100",
		Port:      8080,
		Status:    EdgeNodeStatusOnline,
		Capabilities: EdgeNodeCapabilities{
			CPU:    4,
			Memory: 8192,
			Storage: 100,
			GPU:    false,
			AI:     true,
		},
	}

	if err := manager.RegisterNode(node); err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	// 测试获取节点
	retrieved, exists := manager.GetNode("edge-001")
	if !exists {
		t.Fatal("获取节点失败")
	}

	if retrieved.Name != "测试节点" {
		t.Errorf("节点名称不匹配: got %s, want %s", retrieved.Name, "测试节点")
	}

	// 测试获取所有节点
	nodes := manager.GetNodes()
	if len(nodes) != 1 {
		t.Errorf("节点数量不匹配: got %d, want 1", len(nodes))
	}

	// 测试获取在线节点
	onlineNodes := manager.GetOnlineNodes()
	if len(onlineNodes) != 1 {
		t.Errorf("在线节点数量不匹配: got %d, want 1", len(onlineNodes))
	}

	// 测试更新状态
	if err := manager.UpdateNodeStatus("edge-001", EdgeNodeStatusBusy); err != nil {
		t.Fatalf("更新节点状态失败: %v", err)
	}

	retrieved, _ = manager.GetNode("edge-001")
	if retrieved.Status != EdgeNodeStatusBusy {
		t.Errorf("节点状态不匹配: got %s, want %s", retrieved.Status, EdgeNodeStatusBusy)
	}

	// 测试注销节点
	if err := manager.UnregisterNode("edge-001"); err != nil {
		t.Fatalf("注销节点失败: %v", err)
	}

	_, exists = manager.GetNode("edge-001")
	if exists {
		t.Error("节点应该已被注销")
	}
}

// TestTaskScheduler 测试任务调度器
func TestTaskScheduler(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := TaskSchedulerConfig{
		DataDir:       t.TempDir(),
		MaxConcurrent: 10,
		TaskTimeout:   60,
		RetryAttempts: 3,
	}

	scheduler, err := NewTaskScheduler(config, logger)
	if err != nil {
		t.Fatalf("创建任务调度器失败: %v", err)
	}

	if err := scheduler.Initialize(); err != nil {
		t.Fatalf("初始化任务调度器失败: %v", err)
	}
	defer scheduler.Shutdown()

	// 测试创建任务
	task := &Task{
		Name: "测试任务",
		Type: TaskTypeCompute,
		Requirements: TaskRequirements{
			CPU:    2,
			Memory: 1024,
		},
		Config: TaskConfig{
			Timeout:    60,
			MaxRetries: 3,
			Priority:   TaskPriorityNormal,
		},
	}

	if err := scheduler.CreateTask(task); err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	if task.ID == "" {
		t.Error("任务 ID 应该已生成")
	}

	// 测试获取任务
	retrieved, exists := scheduler.GetTask(task.ID)
	if !exists {
		t.Fatal("获取任务失败")
	}

	if retrieved.Name != "测试任务" {
		t.Errorf("任务名称不匹配: got %s, want %s", retrieved.Name, "测试任务")
	}

	// 测试获取所有任务
	tasks := scheduler.GetTasks()
	if len(tasks) != 1 {
		t.Errorf("任务数量不匹配: got %d, want 1", len(tasks))
	}

	// 测试取消任务
	if err := scheduler.CancelTask(task.ID); err != nil {
		t.Fatalf("取消任务失败: %v", err)
	}

	retrieved, _ = scheduler.GetTask(task.ID)
	if retrieved.Status != TaskStatusCancelled {
		t.Errorf("任务状态不匹配: got %s, want %s", retrieved.Status, TaskStatusCancelled)
	}
}

// TestResultAggregator 测试结果聚合器
func TestResultAggregator(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := ResultAggregatorConfig{
		DataDir:        t.TempDir(),
		MaxResults:     1000,
		Timeout:        60,
		ProcessWorkers: 2,
	}

	agg, err := NewResultAggregator(config, logger)
	if err != nil {
		t.Fatalf("创建结果聚合器失败: %v", err)
	}

	if err := agg.Initialize(); err != nil {
		t.Fatalf("初始化结果聚合器失败: %v", err)
	}
	defer agg.Shutdown()

	// 测试创建聚合
	aggregation, err := agg.CreateAggregation("task-001", AggregationStrategyAll, 3)
	if err != nil {
		t.Fatalf("创建聚合失败: %v", err)
	}

	if aggregation.TaskID != "task-001" {
		t.Errorf("任务 ID 不匹配: got %s, want %s", aggregation.TaskID, "task-001")
	}

	// 测试提交结果
	result := &TaskResult{
		TaskID:   "task-001",
		NodeID:   "edge-001",
		Success:  true,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  time.Second,
	}

	if err := agg.SubmitResult(result); err != nil {
		t.Fatalf("提交结果失败: %v", err)
	}

	// 测试获取聚合
	_, exists := agg.GetAggregation(aggregation.ID)
	if !exists {
		t.Fatal("获取聚合失败")
	}

	// 测试统计
	stats := agg.GetStats()
	if stats["total_aggregations"].(int) != 1 {
		t.Errorf("聚合数量不匹配")
	}
}

// TestEdgeLoadBalancer 测试边缘负载均衡器
func TestEdgeLoadBalancer(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 创建边缘节点管理器
	nodeConfig := EdgeNodeConfig{
		NodeID:           "test-lb-node",
		DataDir:          t.TempDir(),
		HeartbeatInterval: 10 * time.Second,
		HeartbeatTimeout:  30 * time.Second,
	}

	nodeManager, err := NewEdgeNodeManager(nodeConfig, logger, nil)
	if err != nil {
		t.Fatalf("创建边缘节点管理器失败: %v", err)
	}
	nodeManager.Initialize()
	defer nodeManager.Shutdown()

	// 注册测试节点
	nodes := []*EdgeNode{
		{
			ID:        "edge-001",
			Name:      "节点1",
			Type:      EdgeNodeTypeCompute,
			IPAddress: "192.168.1.100",
			Port:      8080,
			Status:    EdgeNodeStatusIdle,
			Priority:  10,
			Weight:    100,
			Capabilities: EdgeNodeCapabilities{
				CPU: 4,
				Memory: 8192,
				Caps: EdgeCapCompute | EdgeCapAI,
			},
			Resources: EdgeNodeResource{
				CPUUsed:    10,
				MemoryUsed: 20,
			},
		},
		{
			ID:        "edge-002",
			Name:      "节点2",
			Type:      EdgeNodeTypeCompute,
			IPAddress: "192.168.1.101",
			Port:      8080,
			Status:    EdgeNodeStatusIdle,
			Priority:  5,
			Weight:    50,
			Capabilities: EdgeNodeCapabilities{
				CPU: 2,
				Memory: 4096,
				Caps: EdgeCapCompute,
			},
			Resources: EdgeNodeResource{
				CPUUsed:    30,
				MemoryUsed: 40,
			},
		},
	}

	for _, node := range nodes {
		nodeManager.RegisterNode(node)
	}

	// 创建负载均衡器
	lbConfig := EdgeLBConfig{
		Strategy:        EdgeLBStrategyLeastLoad,
		ResourceWeight:  0.4,
		LocationWeight:  0.3,
		LatencyWeight:   0.2,
		CapabilityWeight: 0.1,
	}

	lb, err := NewEdgeLoadBalancer(lbConfig, nodeManager, logger)
	if err != nil {
		t.Fatalf("创建边缘负载均衡器失败: %v", err)
	}
	lb.Initialize()
	defer lb.Shutdown()

	// 测试选择节点
	req := SelectNodeRequest{
		SessionID: "session-001",
		Requirements: TaskRequirements{
			CPU:    2,
			Memory: 1024,
		},
	}

	selected, err := lb.SelectNode(req)
	if err != nil {
		t.Fatalf("选择节点失败: %v", err)
	}

	if selected == nil {
		t.Error("应该选择到节点")
	}

	// 测试记录请求
	lb.RecordRequest(selected.ID, true, 10*time.Millisecond)

	// 测试统计
	stats := lb.GetStats()
	if stats.TotalRequests != 1 {
		t.Errorf("请求总数不匹配: got %d, want 1", stats.TotalRequests)
	}

	// 测试不同策略
	strategies := []string{
		EdgeLBStrategyRoundRobin,
		EdgeLBStrategyLeastLoad,
		EdgeLBStrategyResource,
		EdgeLBStrategyWeighted,
	}

	for _, strategy := range strategies {
		lbConfig.Strategy = strategy
		lb.UpdateConfig(lbConfig)

		selected, err := lb.SelectNode(req)
		if err != nil {
			t.Errorf("策略 %s 选择节点失败: %v", strategy, err)
		}
		if selected == nil {
			t.Errorf("策略 %s 应该选择到节点", strategy)
		}
	}
}

// TestEdgeNodeSelection 测试边缘节点选择
func TestEdgeNodeSelection(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	nodeConfig := EdgeNodeConfig{
		NodeID:           "test-select-node",
		DataDir:          t.TempDir(),
		HeartbeatInterval: 10 * time.Second,
		HeartbeatTimeout:  30 * time.Second,
	}

	nodeManager, _ := NewEdgeNodeManager(nodeConfig, logger, nil)
	nodeManager.Initialize()
	defer nodeManager.Shutdown()

	// 注册不同类型的节点
	testNodes := []*EdgeNode{
		{
			ID:     "gpu-node",
			Type:   EdgeNodeTypeCompute,
			Status: EdgeNodeStatusIdle,
			Capabilities: EdgeNodeCapabilities{
				CPU: 8,
				Memory: 16384,
				GPU: true,
				AI: true,
				Caps: EdgeCapCompute | EdgeCapAI | EdgeCapGPU,
			},
			Resources: EdgeNodeResource{CPUUsed: 5, MemoryUsed: 10},
		},
		{
			ID:     "cpu-node",
			Type:   EdgeNodeTypeCompute,
			Status: EdgeNodeStatusIdle,
			Capabilities: EdgeNodeCapabilities{
				CPU: 16,
				Memory: 32768,
				GPU: false,
				AI: false,
				Caps: EdgeCapCompute,
			},
			Resources: EdgeNodeResource{CPUUsed: 50, MemoryUsed: 60},
		},
		{
			ID:     "storage-node",
			Type:   EdgeNodeTypeStorage,
			Status: EdgeNodeStatusIdle,
			Capabilities: EdgeNodeCapabilities{
				CPU: 2,
				Memory: 4096,
				Storage: 1000,
				Caps: EdgeCapStorage,
			},
			Resources: EdgeNodeResource{CPUUsed: 10, MemoryUsed: 20},
		},
	}

	for _, node := range testNodes {
		nodeManager.RegisterNode(node)
	}

	// 测试选择 GPU 节点
	req := TaskRequirements{
		GPU: true,
	}

	selected, err := nodeManager.SelectBestNode(req)
	if err != nil {
		t.Fatalf("选择 GPU 节点失败: %v", err)
	}

	if selected.ID != "gpu-node" {
		t.Errorf("应该选择 GPU 节点: got %s", selected.ID)
	}

	// 测试按能力选择
	req = TaskRequirements{
		Capabilities: EdgeCapStorage,
	}

	selected, err = nodeManager.SelectBestNode(req)
	if err != nil {
		t.Fatalf("选择存储节点失败: %v", err)
	}

	if selected.ID != "storage-node" {
		t.Errorf("应该选择存储节点: got %s", selected.ID)
	}
}

// TestEdgeIntegration 边缘计算集成测试
func TestEdgeIntegration(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 创建边缘计算服务
	config := DefaultEdgeConfig()
	config.DataDir = t.TempDir()
	config.NodeID = "integration-test-node"

	services, err := InitializeEdgeComputing(config, logger, nil)
	if err != nil {
		t.Fatalf("初始化边缘计算服务失败: %v", err)
	}
	defer ShutdownEdgeComputing(services)

	// 测试边缘节点管理
	node := &EdgeNode{
		ID:        "integration-node",
		Name:      "集成测试节点",
		Type:      EdgeNodeTypeCompute,
		IPAddress: "192.168.1.200",
		Port:      8080,
		Status:    EdgeNodeStatusOnline,
		Capabilities: EdgeNodeCapabilities{
			CPU:    4,
			Memory: 8192,
			Caps:   EdgeCapCompute,
		},
	}

	if err := services.NodeManager.RegisterNode(node); err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	// 测试任务调度
	task := &Task{
		Name: "集成测试任务",
		Type: TaskTypeCompute,
		Requirements: TaskRequirements{
			CPU:    1,
			Memory: 512,
		},
	}

	if err := services.TaskScheduler.CreateTask(task); err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	// 测试结果聚合
	agg, err := services.ResultAgg.CreateAggregation(task.ID, AggregationStrategyAny, 1)
	if err != nil {
		t.Fatalf("创建聚合失败: %v", err)
	}

	// 测试负载均衡
	lbReq := SelectNodeRequest{
		Requirements: TaskRequirements{CPU: 1, Memory: 512},
	}

	selected, err := services.LoadBalancer.SelectNode(lbReq)
	if err != nil {
		t.Fatalf("负载均衡选择节点失败: %v", err)
	}

	t.Logf("集成测试完成: 任务 %s 调度到节点 %s, 聚合 ID %s", task.ID, selected.ID, agg.ID)
}