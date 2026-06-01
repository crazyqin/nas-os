// Package containersched 测试
package containersched

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("Expected manager")
	}
}

func TestMgrCreateNode(t *testing.T) {
	m := NewManager()

	node, err := m.CreateNode(CreateNodeRequest{
		Name: "test-node",
		Host: "192.168.1.100",
		Role: NodeRoleWorker,
	})
	if err != nil {
		t.Fatalf("create node failed: %v", err)
	}
	if node.ID == "" {
		t.Error("Expected node ID")
	}
	if node.Name != "test-node" {
		t.Errorf("Expected name 'test-node', got '%s'", node.Name)
	}
	if node.Status != NodeStatusReady {
		t.Errorf("Expected ready status, got '%s'", node.Status)
	}
}

func TestMgrCreateNodeValidation(t *testing.T) {
	m := NewManager()

	// 测试空名称
	_, err := m.CreateNode(CreateNodeRequest{
		Host: "192.168.1.100",
	})
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

func TestMgrGetNode(t *testing.T) {
	m := NewManager()

	node, _ := m.CreateNode(CreateNodeRequest{
		Name: "test-node",
		Host: "192.168.1.100",
	})

	fetched, err := m.GetNode(node.ID)
	if err != nil {
		t.Fatalf("get node failed: %v", err)
	}
	if fetched.Name != "test-node" {
		t.Errorf("Expected name 'test-node', got '%s'", fetched.Name)
	}

	_, err = m.GetNode("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent node")
	}
}

func TestMgrListNodes(t *testing.T) {
	m := NewManager()

	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})
	m.CreateNode(CreateNodeRequest{
		Name: "node2",
		Host: "192.168.1.101",
	})

	nodes := m.ListNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
}

func TestMgrUpdateNode(t *testing.T) {
	m := NewManager()

	node, _ := m.CreateNode(CreateNodeRequest{
		Name: "test-node",
		Host: "192.168.1.100",
	})

	newName := "updated-node"
	updated, err := m.UpdateNode(node.ID, UpdateNodeRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("update node failed: %v", err)
	}
	if updated.Name != "updated-node" {
		t.Errorf("Expected name 'updated-node', got '%s'", updated.Name)
	}
}

func TestMgrDeleteNode(t *testing.T) {
	m := NewManager()

	node, _ := m.CreateNode(CreateNodeRequest{
		Name: "test-node",
		Host: "192.168.1.100",
	})

	err := m.DeleteNode(node.ID)
	if err != nil {
		t.Fatalf("delete node failed: %v", err)
	}

	_, err = m.GetNode(node.ID)
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestMgrSchedule(t *testing.T) {
	m := NewManager()

	// 创建节点
	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})
	m.CreateNode(CreateNodeRequest{
		Name: "node2",
		Host: "192.168.1.101",
	})

	// 调度容器
	result, err := m.Schedule(&ScheduleRequest{
		ContainerID:   "container-1",
		ContainerName: "web",
		Image:         "nginx:latest",
		Resources: &ResourceRequest{
			CPUCores:    1,
			MemoryBytes: 512 * 1024 * 1024,
		},
		Priority: PriorityNormal,
	})
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if !result.Success {
		t.Error("Expected successful schedule")
	}
	if result.NodeID == "" {
		t.Error("Expected node ID in result")
	}
}

func TestMgrScheduleNoNodes(t *testing.T) {
	m := NewManager()

	_, err := m.Schedule(&ScheduleRequest{
		ContainerID: "container-1",
		Image:       "nginx:latest",
	})
	if err == nil {
		t.Error("Expected error when no nodes available")
	}
}

func TestMgrSchedulePriority(t *testing.T) {
	m := NewManager()

	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})

	// 高优先级
	result1, _ := m.Schedule(&ScheduleRequest{
		ContainerID: "container-high",
		Image:       "nginx",
		Priority:    PriorityCritical,
	})

	// 低优先级
	result2, _ := m.Schedule(&ScheduleRequest{
		ContainerID: "container-low",
		Image:       "nginx",
		Priority:    PriorityLow,
	})

	if result1.Score <= result2.Score {
		t.Error("Expected higher score for higher priority")
	}
}

func TestMgrEnqueueDequeue(t *testing.T) {
	m := NewManager()

	// 入队
	item1, _ := m.Enqueue(&ScheduleRequest{
		ContainerID: "container-1",
		Image:       "nginx",
		Priority:    PriorityLow,
	})
	if item1.Status != QueueItemStatusPending {
		t.Errorf("Expected pending status, got '%s'", item1.Status)
	}

	item2, _ := m.Enqueue(&ScheduleRequest{
		ContainerID: "container-2",
		Image:       "nginx",
		Priority:    PriorityCritical,
	})
	if item2.Status != QueueItemStatusPending {
		t.Errorf("Expected pending status, got '%s'", item2.Status)
	}

	// 出队（应该是高优先级的先出）
	dequeued := m.Dequeue()
	if dequeued == nil {
		t.Fatal("Expected dequeue item")
	}
	if dequeued.Request.ContainerID != "container-2" {
		t.Errorf("Expected container-2 (high priority), got '%s'", dequeued.Request.ContainerID)
	}

	dequeued2 := m.Dequeue()
	if dequeued2 == nil {
		t.Fatal("Expected dequeue item")
	}
	if dequeued2.Request.ContainerID != "container-1" {
		t.Errorf("Expected container-1 (low priority), got '%s'", dequeued2.Request.ContainerID)
	}

	// 队列应该为空
	dequeued3 := m.Dequeue()
	if dequeued3 != nil {
		t.Error("Expected nil from empty queue")
	}
}

func TestMgrAutoScale(t *testing.T) {
	m := NewManager()

	// 创建策略
	policy, err := m.CreateAutoScalePolicy("web", &AutoScalePolicy{
		Enabled:       true,
		MinReplicas:   1,
		MaxReplicas:   10,
		ScaleUpStep:   2,
		ScaleDownStep: 1,
		Metrics: []ScaleMetric{
			{Type: MetricTypeCPU, Target: 80},
		},
	})
	if err != nil {
		t.Fatalf("create auto scale policy failed: %v", err)
	}
	if policy.ID == "" {
		t.Error("Expected policy ID")
	}

	// 获取策略
	fetched, err := m.GetAutoScalePolicy("web")
	if err != nil {
		t.Fatalf("get auto scale policy failed: %v", err)
	}
	if fetched.ContainerName != "web" {
		t.Errorf("Expected container name 'web', got '%s'", fetched.ContainerName)
	}
}

func TestMgrPowerSave(t *testing.T) {
	m := NewManager()

	// 获取默认配置
	config := m.GetPowerSaveConfig()
	if config.Enabled {
		t.Error("Expected power save to be disabled by default")
	}

	// 更新配置
	enabled := true
	threshold := 0.4
	updated := m.UpdatePowerSaveConfig(UpdatePowerSaveRequest{
		Enabled:   &enabled,
		Threshold: &threshold,
	})
	if !updated.Enabled {
		t.Error("Expected power save to be enabled")
	}
	if updated.Threshold != 0.4 {
		t.Errorf("Expected threshold 0.4, got %f", updated.Threshold)
	}
}

func TestMgrPlacement(t *testing.T) {
	m := NewManager()

	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})

	// 调度容器
	m.Schedule(&ScheduleRequest{
		ContainerID:   "container-1",
		ContainerName: "web",
		Image:         "nginx",
	})

	// 获取放置信息
	placement, err := m.GetPlacement("container-1")
	if err != nil {
		t.Fatalf("get placement failed: %v", err)
	}
	if placement.ContainerID != "container-1" {
		t.Errorf("Expected container ID 'container-1', got '%s'", placement.ContainerID)
	}

	// 列出所有放置
	placements := m.ListPlacements()
	if len(placements) != 1 {
		t.Errorf("Expected 1 placement, got %d", len(placements))
	}

	// 移除放置
	err = m.RemovePlacement("container-1")
	if err != nil {
		t.Fatalf("remove placement failed: %v", err)
	}

	// 验证已移除
	_, err = m.GetPlacement("container-1")
	if err == nil {
		t.Error("Expected error after removal")
	}
}

func TestMgrStats(t *testing.T) {
	m := NewManager()

	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})

	m.Schedule(&ScheduleRequest{
		ContainerID: "container-1",
		Image:       "nginx",
	})

	stats := m.GetStats()
	if stats.TotalScheduled != 1 {
		t.Errorf("Expected 1 scheduled, got %d", stats.TotalScheduled)
	}
	if stats.ActiveNodes != 1 {
		t.Errorf("Expected 1 active node, got %d", stats.ActiveNodes)
	}
	if stats.TotalContainers != 1 {
		t.Errorf("Expected 1 container, got %d", stats.TotalContainers)
	}
}
