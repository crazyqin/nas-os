package edgenodemanager

import (
	"testing"
	"time"
)

func TestNewEdgeNodeManager(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)
	if m == nil {
		t.Fatal("NewEdgeNodeManager returned nil")
	}
	if m.strategy != StrategyRoundRobin {
		t.Errorf("expected round_robin strategy, got %s", m.strategy)
	}
}

func TestRegisterNode(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	node := &EdgeNode{
		ID:        "node1",
		Name:      "edge-01",
		IPAddress: "192.168.1.100",
		Port:      8080,
		Role:      RoleWorker,
		Region:    "cn-east",
		Zone:      "zone-a",
		Version:   "1.0.0",
	}

	err := m.RegisterNode(node)
	if err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	if node.Status != NodeOnline {
		t.Errorf("expected online status, got %s", node.Status)
	}

	// 测试重复注册
	err = m.RegisterNode(node)
	if err == nil {
		t.Error("expected error for duplicate node")
	}
}

func TestUnregisterNode(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{
		ID:   "node1",
		Name: "edge-01",
	})

	err := m.UnregisterNode("node1")
	if err != nil {
		t.Fatalf("UnregisterNode failed: %v", err)
	}

	_, err = m.GetNode("node1")
	if err == nil {
		t.Error("expected error for unregistered node")
	}

	// 测试注销不存在的节点
	err = m.UnregisterNode("node999")
	if err == nil {
		t.Error("expected error for non-existent node")
	}
}

func TestGetNode(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{
		ID:   "node1",
		Name: "edge-01",
	})

	node, err := m.GetNode("node1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if node.Name != "edge-01" {
		t.Errorf("expected edge-01, got %s", node.Name)
	}

	_, err = m.GetNode("node999")
	if err == nil {
		t.Error("expected error for non-existent node")
	}
}

func TestListNodes(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01", Role: RoleWorker, Region: "cn-east"})
	m.RegisterNode(&EdgeNode{ID: "node2", Name: "edge-02", Role: RoleGateway, Region: "cn-east"})
	m.RegisterNode(&EdgeNode{ID: "node3", Name: "edge-03", Role: RoleWorker, Region: "cn-west"})
	m.UpdateNodeStatus("node2", NodeOffline)

	// 按角色筛选
	nodes := m.ListNodes("", RoleWorker, "")
	if len(nodes) != 2 {
		t.Errorf("expected 2 worker nodes, got %d", len(nodes))
	}

	// 按状态筛选
	nodes = m.ListNodes(NodeOnline, "", "")
	if len(nodes) != 2 {
		t.Errorf("expected 2 online nodes, got %d", len(nodes))
	}

	// 按区域筛选
	nodes = m.ListNodes("", "", "cn-east")
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes in cn-east, got %d", len(nodes))
	}
}

func TestUpdateNodeMetrics(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})

	metrics := &NodeMetrics{
		CPUUsage:    45.5,
		MemoryUsage: 60.2,
		DiskUsage:   30.0,
		NetworkIn:   1024000,
		NetworkOut:  512000,
		LoadAverage: 1.5,
	}

	err := m.UpdateNodeMetrics("node1", metrics)
	if err != nil {
		t.Fatalf("UpdateNodeMetrics failed: %v", err)
	}

	node, _ := m.GetNode("node1")
	if node.Metrics.CPUUsage != 45.5 {
		t.Errorf("expected CPU 45.5, got %f", node.Metrics.CPUUsage)
	}

	err = m.UpdateNodeMetrics("node999", metrics)
	if err == nil {
		t.Error("expected error for non-existent node")
	}
}

func TestUpdateNodeStatus(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})

	err := m.UpdateNodeStatus("node1", NodeMaintenance)
	if err != nil {
		t.Fatalf("UpdateNodeStatus failed: %v", err)
	}

	node, _ := m.GetNode("node1")
	if node.Status != NodeMaintenance {
		t.Errorf("expected maintenance, got %s", node.Status)
	}
}

func TestDiscoverNodes(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{
		ID:     "node1",
		Name:   "edge-01",
		Region: "cn-east",
		Labels: map[string]string{"env": "prod", "tier": "frontend"},
	})
	m.RegisterNode(&EdgeNode{
		ID:     "node2",
		Name:   "edge-02",
		Region: "cn-east",
		Labels: map[string]string{"env": "staging"},
	})
	m.RegisterNode(&EdgeNode{
		ID:     "node3",
		Name:   "edge-03",
		Region: "cn-west",
		Labels: map[string]string{"env": "prod"},
	})

	// 按区域发现
	nodes := m.DiscoverNodes("cn-east", nil)
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes in cn-east, got %d", len(nodes))
	}

	// 按标签发现
	nodes = m.DiscoverNodes("", map[string]string{"env": "prod"})
	if len(nodes) != 2 {
		t.Errorf("expected 2 prod nodes, got %d", len(nodes))
	}

	// 按区域+标签发现
	nodes = m.DiscoverNodes("cn-east", map[string]string{"env": "prod"})
	if len(nodes) != 1 {
		t.Errorf("expected 1 prod node in cn-east, got %d", len(nodes))
	}

	// 离线节点不被发现
	m.UpdateNodeStatus("node1", NodeOffline)
	nodes = m.DiscoverNodes("cn-east", nil)
	if len(nodes) != 1 {
		t.Errorf("expected 1 online node in cn-east, got %d", len(nodes))
	}
}

func TestSubmitTask(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	task := &ComputeTask{
		ID:      "task1",
		Name:    "process-data",
		Payload: []byte("test data"),
	}

	err := m.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	if task.Status != TaskPending {
		t.Errorf("expected pending, got %s", task.Status)
	}
	if task.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", task.MaxRetries)
	}
}

func TestScheduleTask(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})
	m.RegisterNode(&EdgeNode{ID: "node2", Name: "edge-02"})

	task := &ComputeTask{
		ID:   "task1",
		Name: "process-data",
	}
	m.SubmitTask(task)

	err := m.ScheduleTask("task1")
	if err != nil {
		t.Fatalf("ScheduleTask failed: %v", err)
	}

	if task.Status != TaskRunning {
		t.Errorf("expected running, got %s", task.Status)
	}
	if task.AssignedTo == "" {
		t.Error("expected node assignment")
	}

	// 测试无可用节点
	m.UpdateNodeStatus("node1", NodeOffline)
	m.UpdateNodeStatus("node2", NodeOffline)

	task2 := &ComputeTask{ID: "task2", Name: "task2"}
	m.SubmitTask(task2)

	err = m.ScheduleTask("task2")
	if err == nil {
		t.Error("expected error when no nodes available")
	}
}

func TestCompleteTask(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})

	task := &ComputeTask{ID: "task1", Name: "task1"}
	m.SubmitTask(task)
	m.ScheduleTask("task1")

	err := m.CompleteTask("task1", []byte("result data"))
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	if task.Status != TaskCompleted {
		t.Errorf("expected completed, got %s", task.Status)
	}
	if string(task.Result) != "result data" {
		t.Errorf("expected result data, got %s", string(task.Result))
	}

	// 测试不存在的任务
	err = m.CompleteTask("task999", nil)
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestFailTask(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	task := &ComputeTask{
		ID:         "task1",
		Name:       "task1",
		MaxRetries: 2,
	}
	m.SubmitTask(task)

	// 第一次失败 - 应该重试
	err := m.FailTask("task1", "connection timeout")
	if err != nil {
		t.Fatalf("FailTask failed: %v", err)
	}

	if task.Status != TaskPending {
		t.Errorf("expected pending after first retry, got %s", task.Status)
	}
	if task.RetryCount != 1 {
		t.Errorf("expected retry count 1, got %d", task.RetryCount)
	}

	// 第二次失败 - 超过重试次数
	m.FailTask("task1", "connection timeout again")

	if task.Status != TaskFailed {
		t.Errorf("expected failed after max retries, got %s", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("expected completion time to be set")
	}
}

func TestListTasks(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})
	m.RegisterNode(&EdgeNode{ID: "node2", Name: "edge-02"})

	m.SubmitTask(&ComputeTask{ID: "task1", Name: "task1"})
	m.SubmitTask(&ComputeTask{ID: "task2", Name: "task2"})
	m.SubmitTask(&ComputeTask{ID: "task3", Name: "task3"})

	m.ScheduleTask("task1")
	m.ScheduleTask("task2")
	m.CompleteTask("task1", nil)

	// 按状态筛选
	tasks := m.ListTasks("", TaskRunning)
	if len(tasks) != 1 {
		t.Errorf("expected 1 running task, got %d", len(tasks))
	}

	tasks = m.ListTasks("", TaskPending)
	if len(tasks) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(tasks))
	}

	tasks = m.ListTasks("", TaskCompleted)
	if len(tasks) != 1 {
		t.Errorf("expected 1 completed task, got %d", len(tasks))
	}

	// 按节点筛选
	tasks = m.ListTasks("node1", "")
	if len(tasks) != 1 {
		t.Errorf("expected 1 task on node1, got %d", len(tasks))
	}
}

func TestDeploy(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})
	m.RegisterNode(&EdgeNode{ID: "node2", Name: "edge-02"})

	req := &DeployRequest{
		ID:          "deploy1",
		TargetNodes: []string{"node1", "node2"},
		Image:       "myapp",
		Version:     "2.0.0",
	}

	err := m.Deploy(req)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if req.Status != "pending" {
		t.Errorf("expected pending, got %s", req.Status)
	}

	// 测试目标节点不存在
	req2 := &DeployRequest{
		ID:          "deploy2",
		TargetNodes: []string{"node999"},
	}
	err = m.Deploy(req2)
	if err == nil {
		t.Error("expected error for non-existent target node")
	}
}

func TestGetDeploy(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})

	req := &DeployRequest{
		ID:          "deploy1",
		TargetNodes: []string{"node1"},
		Image:       "myapp",
		Version:     "1.0.0",
	}
	m.Deploy(req)

	deploy, err := m.GetDeploy("deploy1")
	if err != nil {
		t.Fatalf("GetDeploy failed: %v", err)
	}
	if deploy.Image != "myapp" {
		t.Errorf("expected myapp, got %s", deploy.Image)
	}

	_, err = m.GetDeploy("deploy999")
	if err == nil {
		t.Error("expected error for non-existent deploy")
	}
}

func TestCompleteDeploy(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})

	req := &DeployRequest{
		ID:          "deploy1",
		TargetNodes: []string{"node1"},
		Image:       "myapp",
	}
	m.Deploy(req)

	err := m.CompleteDeploy("deploy1")
	if err != nil {
		t.Fatalf("CompleteDeploy failed: %v", err)
	}

	deploy, _ := m.GetDeploy("deploy1")
	if deploy.Status != "completed" {
		t.Errorf("expected completed, got %s", deploy.Status)
	}
	if deploy.CompletedAt == nil {
		t.Error("expected completion time to be set")
	}
}

func TestDataSync(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})
	m.RegisterNode(&EdgeNode{ID: "node2", Name: "edge-02"})

	sync := &DataSync{
		ID:          "sync1",
		SourceNode:  "node1",
		TargetNodes: []string{"node2"},
		SyncKey:     "config",
		SyncType:    "full",
	}

	err := m.StartDataSync(sync)
	if err != nil {
		t.Fatalf("StartDataSync failed: %v", err)
	}

	if sync.Status != "syncing" {
		t.Errorf("expected syncing, got %s", sync.Status)
	}

	// 完成同步
	err = m.CompleteDataSync("sync1", 1024000)
	if err != nil {
		t.Fatalf("CompleteDataSync failed: %v", err)
	}

	syncResult, _ := m.GetDataSync("sync1")
	if syncResult.Status != "completed" {
		t.Errorf("expected completed, got %s", syncResult.Status)
	}
	if syncResult.BytesSynced != 1024000 {
		t.Errorf("expected 1024000 bytes, got %d", syncResult.BytesSynced)
	}

	// 测试源节点不存在
	sync2 := &DataSync{
		ID:         "sync2",
		SourceNode: "node999",
	}
	err = m.StartDataSync(sync2)
	if err == nil {
		t.Error("expected error for non-existent source node")
	}
}

func TestGetClusterStats(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})
	m.RegisterNode(&EdgeNode{ID: "node2", Name: "edge-02"})
	m.RegisterNode(&EdgeNode{ID: "node3", Name: "edge-03"})

	m.UpdateNodeStatus("node2", NodeOffline)
	m.UpdateNodeMetrics("node1", &NodeMetrics{CPUUsage: 30, MemoryUsage: 40})
	m.UpdateNodeMetrics("node3", &NodeMetrics{CPUUsage: 50, MemoryUsage: 60})

	m.SubmitTask(&ComputeTask{ID: "task1", Name: "task1"})
	m.SubmitTask(&ComputeTask{ID: "task2", Name: "task2"})
	m.SubmitTask(&ComputeTask{ID: "task3", Name: "task3"})
	m.ScheduleTask("task1")
	m.ScheduleTask("task2")
	m.CompleteTask("task1", nil)

	stats := m.GetClusterStats()

	if stats.TotalNodes != 3 {
		t.Errorf("expected 3 total nodes, got %d", stats.TotalNodes)
	}
	if stats.OnlineNodes != 2 {
		t.Errorf("expected 2 online nodes, got %d", stats.OnlineNodes)
	}
	if stats.OfflineNodes != 1 {
		t.Errorf("expected 1 offline node, got %d", stats.OfflineNodes)
	}
	if stats.TotalTasks != 3 {
		t.Errorf("expected 3 total tasks, got %d", stats.TotalTasks)
	}
	if stats.RunningTasks != 1 {
		t.Errorf("expected 1 running task, got %d", stats.RunningTasks)
	}
	if stats.CompletedTasks != 1 {
		t.Errorf("expected 1 completed task, got %d", stats.CompletedTasks)
	}
	if stats.AvgCPUUsage != 40 {
		t.Errorf("expected avg CPU 40, got %f", stats.AvgCPUUsage)
	}
}

func TestCheckNodeHealth(t *testing.T) {
	m := NewEdgeNodeManager(StrategyRoundRobin)

	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})
	m.RegisterNode(&EdgeNode{ID: "node2", Name: "edge-02"})

	// 模拟 node2 超时
	node2, _ := m.GetNode("node2")
	node2.LastSeen = time.Now().Add(-10 * time.Minute)

	offlineNodes := m.CheckNodeHealth(5 * time.Minute)

	if len(offlineNodes) != 1 {
		t.Errorf("expected 1 offline node, got %d", len(offlineNodes))
	}
	if offlineNodes[0] != "node2" {
		t.Errorf("expected node2 offline, got %s", offlineNodes[0])
	}

	node2, _ = m.GetNode("node2")
	if node2.Status != NodeOffline {
		t.Errorf("expected offline, got %s", node2.Status)
	}

	// 确认 node1 仍然在线
	node1, _ := m.GetNode("node1")
	if node1.Status != NodeOnline {
		t.Errorf("expected node1 online, got %s", node1.Status)
	}
}

func TestLoadBalanceStrategies(t *testing.T) {
	// 测试 Round Robin
	m := NewEdgeNodeManager(StrategyRoundRobin)
	m.RegisterNode(&EdgeNode{ID: "node1", Name: "edge-01"})
	m.RegisterNode(&EdgeNode{ID: "node2", Name: "edge-02"})

	task1 := &ComputeTask{ID: "task1", Name: "task1"}
	task2 := &ComputeTask{ID: "task2", Name: "task2"}
	m.SubmitTask(task1)
	m.SubmitTask(task2)

	m.ScheduleTask("task1")
	m.ScheduleTask("task2")

	// Round robin 应该分配到不同节点
	if task1.AssignedTo == task2.AssignedTo {
		t.Error("round robin should distribute tasks to different nodes")
	}

	// 测试 Least Load
	m2 := NewEdgeNodeManager(StrategyLeastLoad)
	m2.RegisterNode(&EdgeNode{
		ID: "node1", Name: "edge-01",
		Metrics: &NodeMetrics{CPUUsage: 80, MemoryUsage: 70},
	})
	m2.RegisterNode(&EdgeNode{
		ID: "node2", Name: "edge-02",
		Metrics: &NodeMetrics{CPUUsage: 20, MemoryUsage: 30},
	})

	task3 := &ComputeTask{ID: "task3", Name: "task3"}
	m2.SubmitTask(task3)
	m2.ScheduleTask("task3")

	if task3.AssignedTo != "node2" {
		t.Errorf("least load should pick node2, got %s", task3.AssignedTo)
	}

	// 测试 Resource Based
	m3 := NewEdgeNodeManager(StrategyResourceBased)
	m3.RegisterNode(&EdgeNode{
		ID: "node1", Name: "edge-01",
		Metrics: &NodeMetrics{CPUUsage: 60, MemoryUsage: 60, DiskUsage: 60},
	})
	m3.RegisterNode(&EdgeNode{
		ID: "node2", Name: "edge-02",
		Metrics: &NodeMetrics{CPUUsage: 10, MemoryUsage: 10, DiskUsage: 10},
	})

	task4 := &ComputeTask{ID: "task4", Name: "task4"}
	m3.SubmitTask(task4)
	m3.ScheduleTask("task4")

	if task4.AssignedTo != "node2" {
		t.Errorf("resource based should pick node2, got %s", task4.AssignedTo)
	}
}
