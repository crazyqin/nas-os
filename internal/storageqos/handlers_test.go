package storageqos

import (
	"testing"
	"time"
)

func TestNewQoSManager(t *testing.T) {
	manager := NewQoSManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	if manager.config == nil {
		t.Fatal("Expected default config")
	}
	if !manager.config.Enabled {
		t.Error("Expected default config to be enabled")
	}
	if manager.config.MetricsInterval != 10 {
		t.Errorf("Expected default metrics interval 10, got %d", manager.config.MetricsInterval)
	}
}

func TestCreatePolicy(t *testing.T) {
	manager := NewQoSManager(nil)

	policy := &QoSPolicy{
		Name:         "数据库卷QoS",
		Description:  "数据库卷性能保障",
		Level:        QoSLevelGold,
		TargetType:   "volume",
		TargetID:     "vol_001",
		MinIOPS:      1000,
		MaxIOPS:      5000,
		MinBandwidth: 100,
		MaxBandwidth: 500,
		LatencyMax:   10,
		Adaptive:     true,
	}

	created, err := manager.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if created.ID == "" {
		t.Error("Expected policy ID to be set")
	}

	if created.Name != "数据库卷QoS" {
		t.Errorf("Expected name '数据库卷QoS', got '%s'", created.Name)
	}

	if created.Level != QoSLevelGold {
		t.Errorf("Expected level 'gold', got '%s'", created.Level)
	}

	// 测试空名称
	_, err = manager.CreatePolicy(&QoSPolicy{})
	if err == nil {
		t.Error("Expected error for empty name")
	}

	// 测试无效优先级
	_, err = manager.CreatePolicy(&QoSPolicy{Name: "test", Level: "invalid"})
	if err == nil {
		t.Error("Expected error for invalid level")
	}

	// 测试IOPS下限大于上限
	_, err = manager.CreatePolicy(&QoSPolicy{
		Name:    "test",
		Level:   QoSLevelSilver,
		MinIOPS: 5000,
		MaxIOPS: 1000,
	})
	if err == nil {
		t.Error("Expected error for min IOPS > max IOPS")
	}

	// 测试带宽下限大于上限
	_, err = manager.CreatePolicy(&QoSPolicy{
		Name:         "test",
		Level:        QoSLevelSilver,
		MinBandwidth: 500,
		MaxBandwidth: 100,
	})
	if err == nil {
		t.Error("Expected error for min bandwidth > max bandwidth")
	}
}

func TestGetPolicy(t *testing.T) {
	manager := NewQoSManager(nil)

	policy, _ := manager.CreatePolicy(&QoSPolicy{
		Name:       "测试策略",
		Level:      QoSLevelSilver,
		TargetType: "volume",
		TargetID:   "vol_002",
		MaxIOPS:    3000,
	})

	fetched, err := manager.GetPolicy(policy.ID)
	if err != nil {
		t.Fatalf("Failed to get policy: %v", err)
	}

	if fetched.ID != policy.ID {
		t.Errorf("Expected policy ID '%s', got '%s'", policy.ID, fetched.ID)
	}

	// 测试不存在的策略
	_, err = manager.GetPolicy("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent policy")
	}
}

func TestListPolicies(t *testing.T) {
	manager := NewQoSManager(nil)

	manager.CreatePolicy(&QoSPolicy{
		Name:  "策略1",
		Level: QoSLevelPlatinum,
	})
	manager.CreatePolicy(&QoSPolicy{
		Name:  "策略2",
		Level: QoSLevelGold,
	})
	manager.CreatePolicy(&QoSPolicy{
		Name:  "策略3",
		Level: QoSLevelBronze,
	})

	policies := manager.ListPolicies()
	if len(policies) != 3 {
		t.Errorf("Expected 3 policies, got %d", len(policies))
	}
}

func TestUpdatePolicy(t *testing.T) {
	manager := NewQoSManager(nil)

	policy, _ := manager.CreatePolicy(&QoSPolicy{
		Name:         "原始策略",
		Level:        QoSLevelSilver,
		TargetType:   "volume",
		TargetID:     "vol_003",
		MaxIOPS:      2000,
		MaxBandwidth: 200,
	})

	updated, err := manager.UpdatePolicy(policy.ID, &QoSPolicy{
		Name:         "更新后的策略",
		Level:        QoSLevelGold,
		MaxIOPS:      4000,
		MaxBandwidth: 400,
		LatencyMax:   5,
	})
	if err != nil {
		t.Fatalf("Failed to update policy: %v", err)
	}

	if updated.Name != "更新后的策略" {
		t.Errorf("Expected name '更新后的策略', got '%s'", updated.Name)
	}

	if updated.Level != QoSLevelGold {
		t.Errorf("Expected level 'gold', got '%s'", updated.Level)
	}

	if updated.MaxIOPS != 4000 {
		t.Errorf("Expected max IOPS 4000, got %d", updated.MaxIOPS)
	}

	// 测试更新不存在的策略
	_, err = manager.UpdatePolicy("nonexistent", &QoSPolicy{Name: "test"})
	if err == nil {
		t.Error("Expected error for nonexistent policy")
	}
}

func TestDeletePolicy(t *testing.T) {
	manager := NewQoSManager(nil)

	policy, _ := manager.CreatePolicy(&QoSPolicy{
		Name:       "待删除策略",
		Level:      QoSLevelBronze,
		TargetType: "volume",
		TargetID:   "vol_004",
		MaxIOPS:    1000,
	})

	err := manager.DeletePolicy(policy.ID)
	if err != nil {
		t.Fatalf("Failed to delete policy: %v", err)
	}

	// 验证已删除
	_, err = manager.GetPolicy(policy.ID)
	if err == nil {
		t.Error("Expected error for deleted policy")
	}

	// 测试删除不存在的策略
	err = manager.DeletePolicy("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent policy")
	}
}

func TestEnableDisablePolicy(t *testing.T) {
	manager := NewQoSManager(nil)

	policy, _ := manager.CreatePolicy(&QoSPolicy{
		Name:       "测试策略",
		Level:      QoSLevelSilver,
		TargetType: "volume",
		TargetID:   "vol_005",
		MaxIOPS:    3000,
	})

	// 禁用
	err := manager.DisablePolicy(policy.ID)
	if err != nil {
		t.Fatalf("Failed to disable policy: %v", err)
	}

	fetched, _ := manager.GetPolicy(policy.ID)
	if fetched.Enabled {
		t.Error("Expected policy to be disabled")
	}

	// 启用
	err = manager.EnablePolicy(policy.ID)
	if err != nil {
		t.Fatalf("Failed to enable policy: %v", err)
	}

	fetched, _ = manager.GetPolicy(policy.ID)
	if !fetched.Enabled {
		t.Error("Expected policy to be enabled")
	}
}

func TestGetPoliciesByLevel(t *testing.T) {
	manager := NewQoSManager(nil)

	manager.CreatePolicy(&QoSPolicy{Name: "Gold1", Level: QoSLevelGold})
	manager.CreatePolicy(&QoSPolicy{Name: "Gold2", Level: QoSLevelGold})
	manager.CreatePolicy(&QoSPolicy{Name: "Silver1", Level: QoSLevelSilver})

	goldPolicies := manager.GetPoliciesByLevel(QoSLevelGold)
	if len(goldPolicies) != 2 {
		t.Errorf("Expected 2 gold policies, got %d", len(goldPolicies))
	}

	silverPolicies := manager.GetPoliciesByLevel(QoSLevelSilver)
	if len(silverPolicies) != 1 {
		t.Errorf("Expected 1 silver policy, got %d", len(silverPolicies))
	}

	platinumPolicies := manager.GetPoliciesByLevel(QoSLevelPlatinum)
	if len(platinumPolicies) != 0 {
		t.Errorf("Expected 0 platinum policies, got %d", len(platinumPolicies))
	}
}

func TestGetEnabledPolicies(t *testing.T) {
	manager := NewQoSManager(nil)

	p1, _ := manager.CreatePolicy(&QoSPolicy{Name: "Enabled1", Level: QoSLevelGold})
	manager.CreatePolicy(&QoSPolicy{Name: "Enabled2", Level: QoSLevelSilver})
	manager.DisablePolicy(p1.ID)

	enabledPolicies := manager.GetEnabledPolicies()
	if len(enabledPolicies) != 1 {
		t.Errorf("Expected 1 enabled policy, got %d", len(enabledPolicies))
	}
}

func TestQoSLevelWeight(t *testing.T) {
	tests := []struct {
		level    QoSLevel
		expected int
	}{
		{QoSLevelPlatinum, 100},
		{QoSLevelGold, 75},
		{QoSLevelSilver, 50},
		{QoSLevelBronze, 25},
		{"invalid", 0},
	}

	for _, tt := range tests {
		weight := GetLevelWeight(tt.level)
		if weight != tt.expected {
			t.Errorf("GetLevelWeight(%s) = %d, expected %d", tt.level, weight, tt.expected)
		}
	}
}

func TestNewTargetManager(t *testing.T) {
	tm := NewTargetManager()
	if tm == nil {
		t.Fatal("Expected target manager to be created")
	}
}

func TestRegisterTarget(t *testing.T) {
	tm := NewTargetManager()

	target := &QoSTarget{
		ID:         "target_001",
		Type:       "volume",
		Name:       "数据卷",
		Path:       "/mnt/data",
		DevicePath: "/dev/sda1",
		CGroupPath: "/sys/fs/cgroup/blkio/storageqos/target_001",
	}

	err := tm.RegisterTarget(target)
	if err != nil {
		t.Fatalf("Failed to register target: %v", err)
	}

	fetched, err := tm.GetTarget("target_001")
	if err != nil {
		t.Fatalf("Failed to get target: %v", err)
	}

	if fetched.Name != "数据卷" {
		t.Errorf("Expected name '数据卷', got '%s'", fetched.Name)
	}

	// 测试无效类型
	err = tm.RegisterTarget(&QoSTarget{ID: "test", Type: "invalid"})
	if err == nil {
		t.Error("Expected error for invalid type")
	}

	// 测试空ID
	err = tm.RegisterTarget(&QoSTarget{Type: "volume"})
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestNewPriorityQueue(t *testing.T) {
	pq := NewPriorityQueue(100)
	if pq == nil {
		t.Fatal("Expected priority queue to be created")
	}

	if pq.GetTotalSize() != 0 {
		t.Errorf("Expected empty queue, got size %d", pq.GetTotalSize())
	}
}

func TestPriorityQueueEnqueueDequeue(t *testing.T) {
	pq := NewPriorityQueue(100)

	// 入队不同优先级
	pq.Enqueue(&IORequest{
		TargetID:  "target1",
		Level:     QoSLevelBronze,
		Operation: "read",
		Size:      1024,
	})

	pq.Enqueue(&IORequest{
		TargetID:  "target2",
		Level:     QoSLevelPlatinum,
		Operation: "write",
		Size:      2048,
	})

	pq.Enqueue(&IORequest{
		TargetID:  "target3",
		Level:     QoSLevelGold,
		Operation: "read",
		Size:      512,
	})

	if pq.GetTotalSize() != 3 {
		t.Errorf("Expected queue size 3, got %d", pq.GetTotalSize())
	}

	// 出队应该按优先级顺序
	req1, err := pq.Dequeue()
	if err != nil {
		t.Fatalf("Failed to dequeue: %v", err)
	}
	if req1.Level != QoSLevelPlatinum {
		t.Errorf("Expected platinum level, got %s", req1.Level)
	}

	req2, err := pq.Dequeue()
	if err != nil {
		t.Fatalf("Failed to dequeue: %v", err)
	}
	if req2.Level != QoSLevelGold {
		t.Errorf("Expected gold level, got %s", req2.Level)
	}

	req3, err := pq.Dequeue()
	if err != nil {
		t.Fatalf("Failed to dequeue: %v", err)
	}
	if req3.Level != QoSLevelBronze {
		t.Errorf("Expected bronze level, got %s", req3.Level)
	}

	// 空队列出队
	_, err = pq.Dequeue()
	if err == nil {
		t.Error("Expected error for empty queue")
	}
}

func TestNewIOController(t *testing.T) {
	ic := NewIOController()
	if ic == nil {
		t.Fatal("Expected IO controller to be created")
	}

	limits := ic.ListIOLimits()
	if len(limits) != 0 {
		t.Errorf("Expected empty limits, got %d", len(limits))
	}
}

func TestNewViolationDetector(t *testing.T) {
	manager := NewQoSManager(nil)
	collector := NewMetricsCollector(10 * time.Second)

	detector := NewViolationDetector(manager, collector, func(v *QoSViolation) {
		// alert callback
	})

	if detector == nil {
		t.Fatal("Expected violation detector to be created")
	}

	violations := detector.GetViolations()
	if len(violations) != 0 {
		t.Errorf("Expected empty violations, got %d", len(violations))
	}
}
