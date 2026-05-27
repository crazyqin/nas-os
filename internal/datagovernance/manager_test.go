package datagovernance

import (
	"testing"
)

func TestNewDataGovernanceManager(t *testing.T) {
	manager := NewDataGovernanceManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	if manager.config == nil {
		t.Fatal("Expected default config")
	}
}

func TestCreatePolicy(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	policy := &Policy{
		Name:        "数据保留策略",
		Description: "保留数据365天",
		Type:        PolicyTypeRetention,
		Enabled:     true,
	}

	err := manager.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	if policy.ID == "" {
		t.Error("Expected policy ID to be set")
	}
}

func TestCreatePolicyEmptyName(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	err := manager.CreatePolicy(&Policy{})
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

func TestGetPolicy(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	policy := &Policy{Name: "测试策略", Type: PolicyTypeRetention}
	manager.CreatePolicy(policy)

	fetched, err := manager.GetPolicy(policy.ID)
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}

	if fetched.Name != "测试策略" {
		t.Errorf("Expected name '测试策略', got '%s'", fetched.Name)
	}
}

func TestGetPolicyNotFound(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	_, err := manager.GetPolicy("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent policy")
	}
}

func TestListPolicies(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	manager.CreatePolicy(&Policy{Name: "策略1", Type: PolicyTypeRetention})
	manager.CreatePolicy(&Policy{Name: "策略2", Type: PolicyTypeClassification})

	policies := manager.ListPolicies()
	if len(policies) != 2 {
		t.Errorf("Expected 2 policies, got %d", len(policies))
	}
}

func TestUpdatePolicy(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	policy := &Policy{Name: "原始名称", Type: PolicyTypeRetention}
	manager.CreatePolicy(policy)

	update := &Policy{Name: "更新名称"}
	err := manager.UpdatePolicy(policy.ID, update)
	if err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}

	updated, _ := manager.GetPolicy(policy.ID)
	if updated.Name != "更新名称" {
		t.Errorf("Expected name '更新名称', got '%s'", updated.Name)
	}
}

func TestDeletePolicy(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	policy := &Policy{Name: "待删除", Type: PolicyTypeRetention}
	manager.CreatePolicy(policy)

	err := manager.DeletePolicy(policy.ID)
	if err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	_, err = manager.GetPolicy(policy.ID)
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestCreateClassification(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	class := &Classification{
		Name:  "机密",
		Level: 1,
		Color: "#ff0000",
	}

	err := manager.CreateClassification(class)
	if err != nil {
		t.Fatalf("CreateClassification failed: %v", err)
	}

	if class.ID == "" {
		t.Error("Expected classification ID to be set")
	}
}

func TestListClassifications(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	manager.CreateClassification(&Classification{Name: "机密", Level: 1})
	manager.CreateClassification(&Classification{Name: "内部", Level: 2})

	classes := manager.ListClassifications()
	if len(classes) != 2 {
		t.Errorf("Expected 2 classifications, got %d", len(classes))
	}
}

func TestCreateRetentionRule(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	rule := &RetentionRule{
		Name:    "日志保留",
		Path:    "/var/log",
		Days:    30,
		Action:  "delete",
		Enabled: true,
	}

	err := manager.CreateRetentionRule(rule)
	if err != nil {
		t.Fatalf("CreateRetentionRule failed: %v", err)
	}

	if rule.ID == "" {
		t.Error("Expected rule ID to be set")
	}
}

func TestGetStats(t *testing.T) {
	manager := NewDataGovernanceManager(nil)

	manager.CreatePolicy(&Policy{Name: "策略1", Type: PolicyTypeRetention})
	manager.CreateClassification(&Classification{Name: "机密", Level: 1})

	stats := manager.GetStats()
	if stats["total_policies"] != 1 {
		t.Errorf("Expected 1 policy, got %v", stats["total_policies"])
	}
	if stats["total_classifications"] != 1 {
		t.Errorf("Expected 1 classification, got %v", stats["total_classifications"])
	}
}
