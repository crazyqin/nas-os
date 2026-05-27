package networkqos

import (
	"testing"
)

func TestNewQoSManager(t *testing.T) {
	manager := NewQoSManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	if manager.config == nil {
		t.Fatal("Expected default config")
	}
	if manager.config.DefaultPriority != 5 {
		t.Errorf("Expected default priority 5, got %d", manager.config.DefaultPriority)
	}
}

func TestCreateRule(t *testing.T) {
	manager := NewQoSManager(nil)

	rule := &QoSRule{
		Name:     "HTTP流量",
		Priority: 3,
		Protocol: "tcp",
		DestPort: 80,
		MinMbps:  10,
		MaxMbps:  100,
	}

	created, err := manager.CreateRule(rule)
	if err != nil {
		t.Fatalf("Failed to create rule: %v", err)
	}

	if created.ID == "" {
		t.Error("Expected rule ID to be set")
	}

	if created.Name != "HTTP流量" {
		t.Errorf("Expected name 'HTTP流量', got '%s'", created.Name)
	}

	// 测试空名称
	_, err = manager.CreateRule(&QoSRule{})
	if err == nil {
		t.Error("Expected error for empty name")
	}

	// 测试无效优先级
	_, err = manager.CreateRule(&QoSRule{Name: "test", Priority: 11, MaxMbps: 100})
	if err == nil {
		t.Error("Expected error for invalid priority")
	}

	// 测试无效带宽
	_, err = manager.CreateRule(&QoSRule{Name: "test", Priority: 5, MaxMbps: 0})
	if err == nil {
		t.Error("Expected error for invalid bandwidth")
	}
}

func TestGetRule(t *testing.T) {
	manager := NewQoSManager(nil)

	rule, _ := manager.CreateRule(&QoSRule{
		Name:     "SSH流量",
		Priority: 8,
		Protocol: "tcp",
		DestPort: 22,
		MaxMbps:  50,
	})

	fetched, err := manager.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("Failed to get rule: %v", err)
	}

	if fetched.ID != rule.ID {
		t.Errorf("Expected rule ID '%s', got '%s'", rule.ID, fetched.ID)
	}

	// 测试不存在的规则
	_, err = manager.GetRule("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent rule")
	}
}

func TestListRules(t *testing.T) {
	manager := NewQoSManager(nil)

	manager.CreateRule(&QoSRule{Name: "Rule1", Priority: 5, MaxMbps: 100})
	manager.CreateRule(&QoSRule{Name: "Rule2", Priority: 3, MaxMbps: 200})

	rules := manager.ListRules()
	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}
}

func TestUpdateRule(t *testing.T) {
	manager := NewQoSManager(nil)

	rule, _ := manager.CreateRule(&QoSRule{
		Name:     "原始规则",
		Priority: 5,
		MaxMbps:  100,
	})

	updated, err := manager.UpdateRule(rule.ID, &QoSRule{
		Name:     "更新后的规则",
		Priority: 8,
		MaxMbps:  200,
	})
	if err != nil {
		t.Fatalf("Failed to update rule: %v", err)
	}

	if updated.Name != "更新后的规则" {
		t.Errorf("Expected name '更新后的规则', got '%s'", updated.Name)
	}

	if updated.Priority != 8 {
		t.Errorf("Expected priority 8, got %d", updated.Priority)
	}

	// 测试更新不存在的规则
	_, err = manager.UpdateRule("nonexistent", &QoSRule{Name: "test"})
	if err == nil {
		t.Error("Expected error for nonexistent rule")
	}
}

func TestDeleteRule(t *testing.T) {
	manager := NewQoSManager(nil)

	rule, _ := manager.CreateRule(&QoSRule{
		Name:     "待删除规则",
		Priority: 5,
		MaxMbps:  100,
	})

	err := manager.DeleteRule(rule.ID)
	if err != nil {
		t.Fatalf("Failed to delete rule: %v", err)
	}

	// 验证已删除
	_, err = manager.GetRule(rule.ID)
	if err == nil {
		t.Error("Expected error for deleted rule")
	}

	// 测试删除不存在的规则
	err = manager.DeleteRule("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent rule")
	}
}

func TestEnableDisableRule(t *testing.T) {
	manager := NewQoSManager(nil)

	rule, _ := manager.CreateRule(&QoSRule{
		Name:     "测试规则",
		Priority: 5,
		MaxMbps:  100,
	})

	// 禁用
	err := manager.DisableRule(rule.ID)
	if err != nil {
		t.Fatalf("Failed to disable rule: %v", err)
	}

	fetched, _ := manager.GetRule(rule.ID)
	if fetched.Enabled {
		t.Error("Expected rule to be disabled")
	}

	// 启用
	err = manager.EnableRule(rule.ID)
	if err != nil {
		t.Fatalf("Failed to enable rule: %v", err)
	}

	fetched, _ = manager.GetRule(rule.ID)
	if !fetched.Enabled {
		t.Error("Expected rule to be enabled")
	}
}

func TestGetStats(t *testing.T) {
	manager := NewQoSManager(nil)

	// 测试获取不存在的统计
	_, err := manager.GetStats("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent stats")
	}
}

func TestGetAllStats(t *testing.T) {
	manager := NewQoSManager(nil)

	stats := manager.GetAllStats()
	if stats == nil {
		t.Error("Expected stats map to be initialized")
	}
}
