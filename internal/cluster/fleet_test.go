// Package cluster 舰队管理单元测试
package cluster

import (
	"testing"
	"time"
)

// ========== Fleet Tests ==========

func TestNewFleet(t *testing.T) {
	cfg := &FleetConfig{
		NodeID:   "node-1",
		NodeName: "Master",
		Address:  "192.168.1.100",
		Port:     8080,
		DataDir:  t.TempDir(),
	}

	fleet := NewFleet(cfg)
	if fleet == nil {
		t.Fatal("创建舰队失败")
	}

	// 验证本节点已注册
	node, ok := fleet.GetNode("node-1")
	if !ok {
		t.Fatal("本节点应已注册")
	}
	if node.Role != FleetRoleMaster {
		t.Errorf("本节点角色应为master, got %s", node.Role)
	}
	if node.State != FleetStateOnline {
		t.Errorf("本节点状态应为online, got %s", node.State)
	}
}

func TestFleetNodeRegistration(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: t.TempDir(),
	})

	// 注册节点
	node := &FleetNode{
		ID:      "worker-1",
		Name:    "Worker 1",
		Address: "192.168.1.101",
		Port:    8080,
	}
	if err := fleet.RegisterNode(node); err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	// 验证节点存在
	registered, ok := fleet.GetNode("worker-1")
	if !ok {
		t.Fatal("节点应已注册")
	}
	if registered.Role != FleetRoleWorker {
		t.Errorf("默认角色应为worker, got %s", registered.Role)
	}

	// 重复注册
	if err := fleet.RegisterNode(node); err == nil {
		t.Error("重复注册应失败")
	}

	// 注册空ID
	if err := fleet.RegisterNode(&FleetNode{}); err == nil {
		t.Error("空ID注册应失败")
	}

	// 注销节点
	if err := fleet.UnregisterNode("worker-1"); err != nil {
		t.Fatalf("注销节点失败: %v", err)
	}

	_, ok = fleet.GetNode("worker-1")
	if ok {
		t.Error("节点应已被注销")
	}
}

func TestFleetListNodes(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: t.TempDir(),
	})

	fleet.RegisterNode(&FleetNode{
		ID: "w1", Name: "W1", Address: "10.0.0.1", Role: FleetRoleWorker,
	})
	fleet.RegisterNode(&FleetNode{
		ID: "w2", Name: "W2", Address: "10.0.0.2", Role: FleetRoleWorker,
	})
	fleet.RegisterNode(&FleetNode{
		ID: "s1", Name: "S1", Address: "10.0.0.3", Role: FleetRoleStandby,
	})

	// 列出所有
	all := fleet.ListNodes(nil)
	if len(all) != 4 { // master + w1 + w2 + s1
		t.Errorf("应有4个节点, got %d", len(all))
	}

	// 按角色过滤
	workers := fleet.ListNodes(&NodeFilter{Role: FleetRoleWorker})
	if len(workers) != 2 {
		t.Errorf("应有2个worker, got %d", len(workers))
	}
}

func TestFleetNodeStateUpdate(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: t.TempDir(),
	})

	fleet.RegisterNode(&FleetNode{
		ID: "w1", Name: "W1", Address: "10.0.0.1",
	})

	// 更新状态
	if err := fleet.UpdateNodeState("w1", FleetStateDegraded); err != nil {
		t.Fatalf("更新状态失败: %v", err)
	}

	node, _ := fleet.GetNode("w1")
	if node.State != FleetStateDegraded {
		t.Errorf("状态应为degraded, got %s", node.State)
	}

	// 更新不存在的节点
	if err := fleet.UpdateNodeState("nonexistent", FleetStateOnline); err == nil {
		t.Error("更新不存在的节点应失败")
	}
}

func TestFleetNodeRoleUpdate(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: t.TempDir(),
	})

	fleet.RegisterNode(&FleetNode{
		ID: "w1", Name: "W1", Address: "10.0.0.1",
	})

	if err := fleet.SetNodeRole("w1", FleetRoleStandby); err != nil {
		t.Fatalf("设置角色失败: %v", err)
	}

	node, _ := fleet.GetNode("w1")
	if node.Role != FleetRoleStandby {
		t.Errorf("角色应为standby, got %s", node.Role)
	}
}

func TestFleetNodeMetrics(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: t.TempDir(),
	})

	fleet.RegisterNode(&FleetNode{
		ID: "w1", Name: "W1", Address: "10.0.0.1",
	})

	metrics := &NodeMetrics{
		CPUUsage:    45.5,
		MemoryUsage: 60.2,
		DiskUsage:   30.0,
	}

	if err := fleet.UpdateNodeMetrics("w1", metrics); err != nil {
		t.Fatalf("更新指标失败: %v", err)
	}

	node, _ := fleet.GetNode("w1")
	if node.State != FleetStateOnline {
		t.Errorf("收到心跳后状态应为online, got %s", node.State)
	}
	if node.Metrics == nil {
		t.Fatal("指标不应为空")
	}
	if node.Metrics.CPUUsage != 45.5 {
		t.Errorf("CPU使用率应为45.5, got %f", node.Metrics.CPUUsage)
	}
}

// ========== NodeGroup Tests ==========

func TestFleetGroups(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: t.TempDir(),
	})

	fleet.RegisterNode(&FleetNode{ID: "w1", Name: "W1", Address: "10.0.0.1"})
	fleet.RegisterNode(&FleetNode{ID: "w2", Name: "W2", Address: "10.0.0.2"})

	// 创建分组
	group := &NodeGroup{
		ID:      "storage-group",
		Name:    "存储节点",
		NodeIDs: []string{"w1", "w2"},
	}
	if err := fleet.CreateGroup(group); err != nil {
		t.Fatalf("创建分组失败: %v", err)
	}

	// 重复创建
	if err := fleet.CreateGroup(group); err == nil {
		t.Error("重复创建分组应失败")
	}

	// 获取分组
	g, ok := fleet.GetGroup("storage-group")
	if !ok {
		t.Fatal("分组应存在")
	}
	if len(g.NodeIDs) != 2 {
		t.Errorf("分组应有2个节点, got %d", len(g.NodeIDs))
	}

	// 添加节点到分组
	fleet.RegisterNode(&FleetNode{ID: "w3", Name: "W3", Address: "10.0.0.3"})
	if err := fleet.AddNodeToGroup("storage-group", "w3"); err != nil {
		t.Fatalf("添加节点失败: %v", err)
	}

	g, _ = fleet.GetGroup("storage-group")
	if len(g.NodeIDs) != 3 {
		t.Errorf("分组应有3个节点, got %d", len(g.NodeIDs))
	}

	// 从分组移除节点
	if err := fleet.RemoveNodeFromGroup("storage-group", "w3"); err != nil {
		t.Fatalf("移除节点失败: %v", err)
	}

	// 删除分组
	if err := fleet.DeleteGroup("storage-group"); err != nil {
		t.Fatalf("删除分组失败: %v", err)
	}

	_, ok = fleet.GetGroup("storage-group")
	if ok {
		t.Error("分组应已被删除")
	}
}

func TestFleetGroupsInvalidNode(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: t.TempDir(),
	})

	// 创建分组时引用不存在的节点
	group := &NodeGroup{
		ID:      "bad-group",
		Name:    "Bad",
		NodeIDs: []string{"nonexistent"},
	}
	if err := fleet.CreateGroup(group); err == nil {
		t.Error("引用不存在节点的分组创建应失败")
	}
}

// ========== CrossNodeTask Tests ==========

func TestFleetScheduleTask(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:   "master",
		DataDir:  t.TempDir(),
		MaxNodes: 10,
	})

	fleet.RegisterNode(&FleetNode{
		ID: "w1", Name: "W1", Address: "10.0.0.1",
		Capabilities: []string{"storage"},
		Metrics: &NodeMetrics{CPUUsage: 20, MemoryUsage: 30, DiskUsage: 40},
	})

	task := &CrossNodeTask{
		Type:               FleetTaskBackup,
		Name:               "全量备份",
		Description:        "备份所有数据",
		RequiredCapability: "storage",
		Parameters:         map[string]string{"path": "/data"},
		Priority:           TaskPriorityHigh,
	}

	if err := fleet.ScheduleTask(task); err != nil {
		t.Fatalf("调度任务失败: %v", err)
	}

	if task.ID == "" {
		t.Error("任务ID不应为空")
	}

	// 等待任务被调度循环处理
	time.Sleep(200 * time.Millisecond)

	// 获取任务
	scheduled, ok := fleet.GetTask(task.ID)
	if !ok {
		t.Fatal("任务应存在")
	}
	if scheduled.Status != TaskStatusCompleted {
		t.Errorf("任务状态应为completed, got %s", scheduled.Status)
	}
}

func TestFleetScheduleTaskNoNodes(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: t.TempDir(),
	})

	task := &CrossNodeTask{
		Type:               FleetTaskBackup,
		Name:               "备份",
		RequiredCapability: "nonexistent-cap",
	}

	if err := fleet.ScheduleTask(task); err == nil {
		t.Error("无可用节点时调度应失败")
	}
}

func TestFleetTaskCancel(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: t.TempDir(),
	})

	// 取消不存在的任务
	if err := fleet.CancelTask("nonexistent"); err == nil {
		t.Error("取消不存在的任务应失败")
	}
}

func TestFleetListTasks(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:   "master",
		DataDir:  t.TempDir(),
		MaxNodes: 10,
	})

	fleet.RegisterNode(&FleetNode{
		ID: "w1", Name: "W1", Address: "10.0.0.1",
		Metrics: &NodeMetrics{CPUUsage: 10, MemoryUsage: 10, DiskUsage: 10},
	})

	fleet.ScheduleTask(&CrossNodeTask{Type: FleetTaskBackup, Name: "T1"})
	fleet.ScheduleTask(&CrossNodeTask{Type: FleetTaskSync, Name: "T2"})

	tasks := fleet.ListTasks(nil)
	if len(tasks) < 2 {
		t.Errorf("应有至少2个任务, got %d", len(tasks))
	}

	// 按类型过滤
	backupTasks := fleet.ListTasks(&TaskFilter{Type: FleetTaskBackup})
	if len(backupTasks) < 1 {
		t.Error("应有备份任务")
	}
}

// ========== HealthAggregator Tests ==========

func TestHealthAggregator(t *testing.T) {
	ha := NewHealthAggregator()

	// 更新节点健康
	ha.UpdateNode("node-1", &NodeMetrics{
		CPUUsage:    50,
		MemoryUsage: 60,
		DiskUsage:   30,
	})

	health, ok := ha.GetNodeHealth("node-1")
	if !ok {
		t.Fatal("节点健康数据应存在")
	}
	if health.Status != "healthy" {
		t.Errorf("状态应为healthy, got %s", health.Status)
	}
	if health.OverallScore <= 0 {
		t.Error("综合分数应 > 0")
	}

	// 高负载节点
	ha.UpdateNode("node-2", &NodeMetrics{
		CPUUsage:    95,
		MemoryUsage: 92,
		DiskUsage:   88,
	})

	health, ok = ha.GetNodeHealth("node-2")
	if !ok {
		t.Fatal("节点健康数据应存在")
	}
	if health.Status != "degraded" && health.Status != "unhealthy" {
		t.Errorf("高负载状态应为degraded或unhealthy, got %s", health.Status)
	}
	if len(health.Issues) == 0 {
		t.Error("高负载应有告警")
	}
}

func TestHealthAggregatorClusterHealth(t *testing.T) {
	ha := NewHealthAggregator()

	ha.UpdateNode("n1", &NodeMetrics{CPUUsage: 30, MemoryUsage: 40, DiskUsage: 20})
	ha.UpdateNode("n2", &NodeMetrics{CPUUsage: 50, MemoryUsage: 60, DiskUsage: 40})

	nodes := map[string]*FleetNode{
		"n1": {ID: "n1", State: FleetStateOnline},
		"n2": {ID: "n2", State: FleetStateOnline},
	}

	ch := ha.GetClusterHealth(nodes)
	if ch.Status != "healthy" {
		t.Errorf("集群状态应为healthy, got %s", ch.Status)
	}
	if ch.TotalNodes != 2 {
		t.Errorf("总节点数应为2, got %d", ch.TotalNodes)
	}
	if ch.OnlineNodes != 2 {
		t.Errorf("在线节点应为2, got %d", ch.OnlineNodes)
	}
	if ch.OverallScore <= 0 {
		t.Error("综合分数应 > 0")
	}

	// 有离线节点
	nodes["n3"] = &FleetNode{ID: "n3", State: FleetStateOffline}
	ch = ha.GetClusterHealth(nodes)
	if ch.OfflineNodes != 1 {
		t.Errorf("离线节点应为1, got %d", ch.OfflineNodes)
	}
	if ch.Status != "degraded" {
		t.Errorf("有离线节点集群状态应为degraded, got %s", ch.Status)
	}
}

// ========== AlertAggregator Tests ==========

func TestAlertAggregator(t *testing.T) {
	aa := NewAlertAggregator()

	// 添加告警
	aa.AddAlert(&ClusterAlert{
		NodeID:  "node-1",
		NodeName: "Node 1",
		Level:   "warning",
		Type:    "cpu",
		Message: "CPU使用率过高",
	})

	aa.AddAlert(&ClusterAlert{
		NodeID:  "node-2",
		NodeName: "Node 2",
		Level:   "error",
		Type:    "disk",
		Message: "磁盘空间不足",
	})

	// 列出所有告警
	alerts := aa.GetAlerts(nil)
	if len(alerts) != 2 {
		t.Errorf("应有2个告警, got %d", len(alerts))
	}

	// 按级别过滤
	warnings := aa.GetAlerts(&AlertFilter{Level: "warning"})
	if len(warnings) != 1 {
		t.Errorf("应有1个warning, got %d", len(warnings))
	}

	errors := aa.GetAlerts(&AlertFilter{Level: "error"})
	if len(errors) != 1 {
		t.Errorf("应有1个error, got %d", len(errors))
	}
}

func TestAlertAggregatorAck(t *testing.T) {
	aa := NewAlertAggregator()

	aa.AddAlert(&ClusterAlert{
		ID:      "alert-1",
		NodeID:  "node-1",
		Level:   "warning",
		Message: "test",
	})

	// 确认告警
	if err := aa.AckAlert("alert-1", "admin"); err != nil {
		t.Fatalf("确认告警失败: %v", err)
	}

	alerts := aa.GetAlerts(&AlertFilter{UnackedOnly: true})
	if len(alerts) != 0 {
		t.Error("已确认的告警不应出现在unacked列表中")
	}

	// 确认不存在的告警
	if err := aa.AckAlert("nonexistent", "admin"); err == nil {
		t.Error("确认不存在的告警应失败")
	}
}

func TestAlertAggregatorResolve(t *testing.T) {
	aa := NewAlertAggregator()

	aa.AddAlert(&ClusterAlert{
		ID:      "alert-2",
		NodeID:  "node-1",
		Level:   "error",
		Message: "test",
	})

	// 解决告警
	if err := aa.ResolveAlert("alert-2"); err != nil {
		t.Fatalf("解决告警失败: %v", err)
	}

	alerts := aa.GetAlerts(&AlertFilter{UnresolvedOnly: true})
	if len(alerts) != 0 {
		t.Error("已解决的告警不应出现在unresolved列表中")
	}

	// 解决不存在的告警
	if err := aa.ResolveAlert("nonexistent"); err == nil {
		t.Error("解决不存在的告警应失败")
	}
}

func TestAlertAggregatorFilter(t *testing.T) {
	aa := NewAlertAggregator()

	aa.AddAlert(&ClusterAlert{
		ID: "a1", NodeID: "n1", Level: "info", Message: "info1",
	})
	aa.AddAlert(&ClusterAlert{
		ID: "a2", NodeID: "n1", Level: "warning", Message: "warn1",
	})
	aa.AddAlert(&ClusterAlert{
		ID: "a3", NodeID: "n2", Level: "error", Message: "err1",
	})

	// 按节点过滤
	n1Alerts := aa.GetAlerts(&AlertFilter{NodeID: "n1"})
	if len(n1Alerts) != 2 {
		t.Errorf("n1应有2个告警, got %d", len(n1Alerts))
	}

	n2Alerts := aa.GetAlerts(&AlertFilter{NodeID: "n2"})
	if len(n2Alerts) != 1 {
		t.Errorf("n2应有1个告警, got %d", len(n2Alerts))
	}
}

// ========== CrossNodeTaskQueue Tests ==========

func TestCrossNodeTaskQueue(t *testing.T) {
	q := NewCrossNodeTaskQueue()

	task1 := &CrossNodeTask{ID: "t1", Priority: TaskPriorityLow, Name: "Low"}
	task2 := &CrossNodeTask{ID: "t2", Priority: TaskPriorityHigh, Name: "High"}
	task3 := &CrossNodeTask{ID: "t3", Priority: TaskPriorityNormal, Name: "Normal"}

	q.Enqueue(task1)
	q.Enqueue(task2)
	q.Enqueue(task3)

	// 高优先级应先出
	dequeued := q.Dequeue()
	if dequeued.ID != "t2" {
		t.Errorf("高优先级应先出, got %s", dequeued.ID)
	}

	// 获取任务
	task, ok := q.Get("t1")
	if !ok {
		t.Fatal("任务t1应存在")
	}
	if task.Name != "Low" {
		t.Errorf("名称应为Low, got %s", task.Name)
	}

	// 列出任务
	tasks := q.List(nil)
	if len(tasks) != 3 {
		t.Errorf("应有3个任务, got %d", len(tasks))
	}

	// 取消任务
	if err := q.Cancel("t3"); err != nil {
		t.Fatalf("取消任务失败: %v", err)
	}

	task, _ = q.Get("t3")
	if task.Status != TaskStatusCancelled {
		t.Errorf("任务状态应为cancelled, got %s", task.Status)
	}

	// 取消不存在的任务
	if err := q.Cancel("nonexistent"); err == nil {
		t.Error("取消不存在的任务应失败")
	}
}

func TestCrossNodeTaskQueueEmpty(t *testing.T) {
	q := NewCrossNodeTaskQueue()

	task := q.Dequeue()
	if task != nil {
		t.Error("空队列出队应返回nil")
	}
}

// ========== FleetSummary Tests ==========

func TestFleetSummary(t *testing.T) {
	fleet := NewFleet(&FleetConfig{
		NodeID:   "master",
		DataDir:  t.TempDir(),
		MaxNodes: 10,
	})

	fleet.RegisterNode(&FleetNode{
		ID: "w1", Name: "W1", Address: "10.0.0.1",
		Metrics: &NodeMetrics{CPUUsage: 30, MemoryUsage: 40, DiskUsage: 20},
	})
	fleet.RegisterNode(&FleetNode{
		ID: "w2", Name: "W2", Address: "10.0.0.2",
		Metrics: &NodeMetrics{CPUUsage: 50, MemoryUsage: 60, DiskUsage: 40},
	})

	// 更新指标（触发心跳更新状态）
	fleet.UpdateNodeMetrics("w1", &NodeMetrics{CPUUsage: 30, MemoryUsage: 40, DiskUsage: 20})
	fleet.UpdateNodeMetrics("w2", &NodeMetrics{CPUUsage: 50, MemoryUsage: 60, DiskUsage: 40})

	summary := fleet.GetFleetSummary()
	if summary.TotalNodes != 3 {
		t.Errorf("总节点应为3, got %d", summary.TotalNodes)
	}
	if summary.OnlineNodes < 1 {
		t.Error("至少应有1个在线节点")
	}
	if summary.ClusterHealth == nil {
		t.Error("集群健康不应为空")
	}
}

func TestFleetPersistence(t *testing.T) {
	dataDir := t.TempDir()

	// 创建舰队并注册节点
	fleet1 := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: dataDir,
	})
	fleet1.RegisterNode(&FleetNode{
		ID: "w1", Name: "W1", Address: "10.0.0.1",
	})
	fleet1.CreateGroup(&NodeGroup{
		ID:      "g1",
		Name:    "Group 1",
		NodeIDs: []string{"w1"},
	})

	// 停止并保存
	fleet1.Stop()

	// 重新创建（应加载持久化数据）
	fleet2 := NewFleet(&FleetConfig{
		NodeID:  "master",
		MaxNodes: 10,
		DataDir: dataDir,
	})

	node, ok := fleet2.GetNode("w1")
	if !ok {
		t.Fatal("持久化的节点应被加载")
	}
	if node.Name != "W1" {
		t.Errorf("节点名称应为W1, got %s", node.Name)
	}

	group, ok := fleet2.GetGroup("g1")
	if !ok {
		t.Fatal("持久化的分组应被加载")
	}
	if group.Name != "Group 1" {
		t.Errorf("分组名称应为Group 1, got %s", group.Name)
	}
}
