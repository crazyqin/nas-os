// Package containersched 测试
package containersched

import (
	"testing"
)

func TestScheduleValidation(t *testing.T) {
	m := NewManager()

	// 测试空 container_id
	_, err := m.Schedule(&ScheduleRequest{
		Image: "nginx",
	})
	if err == nil {
		t.Error("Expected error for empty container_id")
	}

	// 测试空 image
	_, err = m.Schedule(&ScheduleRequest{
		ContainerID: "container-1",
	})
	if err == nil {
		t.Error("Expected error for empty image")
	}
}

func TestScheduleWithConstraints(t *testing.T) {
	m := NewManager()

	// 创建带标签的节点
	m.CreateNode(CreateNodeRequest{
		Name:   "node1",
		Host:   "192.168.1.100",
		Labels: map[string]string{"zone": "east"},
	})
	m.CreateNode(CreateNodeRequest{
		Name:   "node2",
		Host:   "192.168.1.101",
		Labels: map[string]string{"zone": "west"},
	})

	// 使用节点选择器调度
	result, err := m.Schedule(&ScheduleRequest{
		ContainerID: "container-1",
		Image:       "nginx",
		Constraints: &ScheduleConstraints{
			NodeSelector: map[string]string{"zone": "east"},
		},
	})
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if result.NodeName != "node1" {
		t.Errorf("Expected node1, got %s", result.NodeName)
	}
}

func TestScheduleWithAffinity(t *testing.T) {
	m := NewManager()

	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})

	// 先调度一个容器
	m.Schedule(&ScheduleRequest{
		ContainerID:   "container-1",
		ContainerName: "web",
		Image:         "nginx",
	})

	// 调度另一个容器，设置亲和性
	result, err := m.Schedule(&ScheduleRequest{
		ContainerID:   "container-2",
		ContainerName: "api",
		Image:         "node",
		Constraints: &ScheduleConstraints{
			Affinity: []AffinityRule{
				{TargetContainer: "web", Weight: 10},
			},
		},
	})
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if result.NodeName != "node1" {
		t.Errorf("Expected node1 (affinity), got %s", result.NodeName)
	}
}

func TestScheduleWithAntiAffinity(t *testing.T) {
	m := NewManager()

	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})
	m.CreateNode(CreateNodeRequest{
		Name: "node2",
		Host: "192.168.1.101",
	})

	// 先调度一个容器到 node1
	m.Schedule(&ScheduleRequest{
		ContainerID:   "container-1",
		ContainerName: "web",
		Image:         "nginx",
	})

	// 调度另一个容器，设置反亲和性
	result, err := m.Schedule(&ScheduleRequest{
		ContainerID:   "container-2",
		ContainerName: "web-replica",
		Image:         "nginx",
		Constraints: &ScheduleConstraints{
			AntiAffinity: []AffinityRule{
				{TargetContainer: "web", Weight: 20},
			},
		},
	})
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if result.NodeID == "" {
		t.Error("Expected node ID in result")
	}
}

func TestScheduleWithTaints(t *testing.T) {
	m := NewManager()

	// 创建带污点的节点
	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
		Taints: []Taint{
			{Key: "special", Value: "true", Effect: TaintEffectNoSchedule},
		},
	})
	m.CreateNode(CreateNodeRequest{
		Name: "node2",
		Host: "192.168.1.101",
	})

	// 不容忍污点的容器应该调度到 node2
	result, err := m.Schedule(&ScheduleRequest{
		ContainerID: "container-1",
		Image:       "nginx",
	})
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if result.NodeName != "node2" {
		t.Errorf("Expected node2 (no taint), got %s", result.NodeName)
	}

	// 容忍污点的容器可以调度到 node1
	result2, err := m.Schedule(&ScheduleRequest{
		ContainerID: "container-2",
		Image:       "nginx",
		Constraints: &ScheduleConstraints{
			Tolerations: []Toleration{
				{Key: "special", Operator: "Equal", Value: "true", Effect: TaintEffectNoSchedule},
			},
		},
	})
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if result2.NodeID == "" {
		t.Error("Expected node ID in result")
	}
}

func TestScheduleExcludedNodes(t *testing.T) {
	m := NewManager()

	node1, _ := m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})
	m.CreateNode(CreateNodeRequest{
		Name: "node2",
		Host: "192.168.1.101",
	})

	// 排除 node1
	result, err := m.Schedule(&ScheduleRequest{
		ContainerID: "container-1",
		Image:       "nginx",
		Constraints: &ScheduleConstraints{
			ExcludedNodes: []string{node1.ID},
		},
	})
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if result.NodeName != "node2" {
		t.Errorf("Expected node2 (node1 excluded), got %s", result.NodeName)
	}
}

func TestEnqueueValidation(t *testing.T) {
	m := NewManager()

	// 测试空 container_id
	_, err := m.Enqueue(&ScheduleRequest{
		Image: "nginx",
	})
	if err == nil {
		t.Error("Expected error for empty container_id")
	}
}

func TestQueuePriorityOrder(t *testing.T) {
	m := NewManager()

	// 按优先级顺序入队
	m.Enqueue(&ScheduleRequest{
		ContainerID: "low",
		Image:       "nginx",
		Priority:    PriorityLow,
	})
	m.Enqueue(&ScheduleRequest{
		ContainerID: "high",
		Image:       "nginx",
		Priority:    PriorityHigh,
	})
	m.Enqueue(&ScheduleRequest{
		ContainerID: "normal",
		Image:       "nginx",
		Priority:    PriorityNormal,
	})
	m.Enqueue(&ScheduleRequest{
		ContainerID: "critical",
		Image:       "nginx",
		Priority:    PriorityCritical,
	})

	// 出队顺序应该是 critical -> high -> normal -> low
	expectedOrder := []string{"critical", "high", "normal", "low"}
	for _, expected := range expectedOrder {
		item := m.Dequeue()
		if item == nil {
			t.Fatalf("Expected item for %s, got nil", expected)
		}
		if item.Request.ContainerID != expected {
			t.Errorf("Expected %s, got %s", expected, item.Request.ContainerID)
		}
	}
}

func TestAutoScaleEvaluation(t *testing.T) {
	m := NewManager()

	// 创建策略
	m.CreateAutoScalePolicy("web", &AutoScalePolicy{
		Enabled:       true,
		MinReplicas:   1,
		MaxReplicas:   10,
		ScaleUpStep:   2,
		ScaleDownStep: 1,
		Metrics: []ScaleMetric{
			{Type: MetricTypeCPU, Target: 80, Current: 90},
		},
	})

	// 评估应该触发扩缩容
	action, replicas, reason, err := m.EvaluateAutoScale("web")
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if action != "scale_up" {
		t.Errorf("Expected scale_up, got %s", action)
	}
	if replicas <= 0 {
		t.Errorf("Expected positive replicas, got %d", replicas)
	}
	if reason == "" {
		t.Error("Expected non-empty reason")
	}
}

func TestAutoScaleDisabled(t *testing.T) {
	m := NewManager()

	// 创建禁用的策略
	m.CreateAutoScalePolicy("web", &AutoScalePolicy{
		Enabled: false,
		Metrics: []ScaleMetric{
			{Type: MetricTypeCPU, Target: 80, Current: 90},
		},
	})

	action, _, _, err := m.EvaluateAutoScale("web")
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if action != "none" {
		t.Errorf("Expected none when disabled, got %s", action)
	}
}

func TestPowerSaveEvaluation(t *testing.T) {
	m := NewManager()

	// 启用节能模式
	enabled := true
	threshold := 0.5
	minNodes := 1
	m.UpdatePowerSaveConfig(UpdatePowerSaveRequest{
		Enabled:        &enabled,
		Threshold:      &threshold,
		MinActiveNodes: &minNodes,
	})

	// 创建低负载节点
	resources := &NodeResources{
		CPU: CPUResource{
			TotalCores:   4,
			UsedCores:    1,
			FreeCores:    3,
			UsagePercent: 25,
		},
		Memory: MemoryResource{
			TotalBytes:   8 * 1024 * 1024 * 1024,
			UsedBytes:    2 * 1024 * 1024 * 1024,
			FreeBytes:    6 * 1024 * 1024 * 1024,
			UsagePercent: 25,
		},
	}

	node1, _ := m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})
	m.UpdateNodeResources(node1.ID, resources)

	node2, _ := m.CreateNode(CreateNodeRequest{
		Name: "node2",
		Host: "192.168.1.101",
	})
	m.UpdateNodeResources(node2.ID, resources)

	// 评估节能模式
	nodesToDrain, _, err := m.EvaluatePowerSave()
	if err != nil {
		t.Fatalf("evaluate power save failed: %v", err)
	}
	// 应该建议排空一些节点
	if len(nodesToDrain) == 0 {
		t.Error("Expected nodes to drain")
	}
}

func TestRemovePlacement(t *testing.T) {
	m := NewManager()

	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})

	m.Schedule(&ScheduleRequest{
		ContainerID: "container-1",
		Image:       "nginx",
	})

	// 移除放置
	err := m.RemovePlacement("container-1")
	if err != nil {
		t.Fatalf("remove placement failed: %v", err)
	}

	// 再次移除应该失败
	err = m.RemovePlacement("container-1")
	if err == nil {
		t.Error("Expected error when removing non-existent placement")
	}
}

func TestNodeResourcesUpdate(t *testing.T) {
	m := NewManager()

	node, _ := m.CreateNode(CreateNodeRequest{
		Name: "test-node",
		Host: "192.168.1.100",
	})

	// 更新资源
	resources := &NodeResources{
		CPU: CPUResource{
			TotalCores:   8,
			UsedCores:    4,
			FreeCores:    4,
			UsagePercent: 50,
		},
		Memory: MemoryResource{
			TotalBytes:   16 * 1024 * 1024 * 1024,
			UsedBytes:    8 * 1024 * 1024 * 1024,
			FreeBytes:    8 * 1024 * 1024 * 1024,
			UsagePercent: 50,
		},
	}

	updated, err := m.UpdateNodeResources(node.ID, resources)
	if err != nil {
		t.Fatalf("update resources failed: %v", err)
	}
	if updated.Resources.CPU.TotalCores != 8 {
		t.Errorf("Expected 8 cores, got %d", updated.Resources.CPU.TotalCores)
	}
	if updated.Resources.Memory.TotalBytes != 16*1024*1024*1024 {
		t.Errorf("Expected 16GB memory, got %d", updated.Resources.Memory.TotalBytes)
	}
}
