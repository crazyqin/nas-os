package smartscrub

import (
	"testing"
)

func TestManager_CreatePolicy(t *testing.T) {
	m := NewManager()

	req := CreatePolicyRequest{
		Name:    "周擦洗",
		Pools:   []string{"tank", "data"},
		Trigger: TriggerSchedule,
		Schedule: "0 2 * * 0",
	}

	policy, err := m.CreatePolicy(req)
	if err != nil {
		t.Fatalf("创建策略失败: %v", err)
	}
	if policy.Name != "周擦洗" {
		t.Errorf("期望名称 '周擦洗', 得到 '%s'", policy.Name)
	}
	if !policy.Enabled {
		t.Error("策略应该默认启用")
	}
}

func TestManager_CreatePolicy_Defaults(t *testing.T) {
	m := NewManager()

	req := CreatePolicyRequest{
		Name:  "默认策略",
		Pools: []string{"tank"},
	}

	policy, err := m.CreatePolicy(req)
	if err != nil {
		t.Fatalf("创建策略失败: %v", err)
	}
	if policy.Trigger != TriggerSchedule {
		t.Errorf("期望触发模式 schedule, 得到 %s", policy.Trigger)
	}
	if policy.Priority != PriorityNormal {
		t.Errorf("期望优先级 normal, 得到 %s", policy.Priority)
	}
}

func TestManager_GetPolicy_NotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetPolicy("nonexistent")
	if err != ErrPolicyNotFound {
		t.Errorf("期望 ErrPolicyNotFound, 得到 %v", err)
	}
}

func TestManager_ListPolicies(t *testing.T) {
	m := NewManager()

	m.CreatePolicy(CreatePolicyRequest{Name: "p1", Pools: []string{"a"}})
	m.CreatePolicy(CreatePolicyRequest{Name: "p2", Pools: []string{"b"}})

	policies := m.ListPolicies()
	if len(policies) != 2 {
		t.Errorf("期望2个策略, 得到 %d", len(policies))
	}
}

func TestManager_DeletePolicy(t *testing.T) {
	m := NewManager()

	policy, _ := m.CreatePolicy(CreatePolicyRequest{Name: "删除测试", Pools: []string{"tank"}})

	if err := m.DeletePolicy(policy.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	_, err := m.GetPolicy(policy.ID)
	if err != ErrPolicyNotFound {
		t.Errorf("期望策略不存在")
	}
}

func TestManager_RunScrub(t *testing.T) {
	m := NewManager()

	policy, _ := m.CreatePolicy(CreatePolicyRequest{Name: "运行测试", Pools: []string{"tank"}})

	record, err := m.RunScrub(policy.ID)
	if err != nil {
		t.Fatalf("运行擦洗失败: %v", err)
	}
	if record.Status != ScrubStatusCompleted {
		t.Errorf("期望 completed, 得到 %s", record.Status)
	}
	if record.Pool != "tank" {
		t.Errorf("期望池 'tank', 得到 '%s'", record.Pool)
	}
}

func TestManager_RunScrub_NotFound(t *testing.T) {
	m := NewManager()

	_, err := m.RunScrub("nonexistent")
	if err != ErrPolicyNotFound {
		t.Errorf("期望 ErrPolicyNotFound, 得到 %v", err)
	}
}

func TestManager_GetRecords(t *testing.T) {
	m := NewManager()

	policy, _ := m.CreatePolicy(CreatePolicyRequest{Name: "记录测试", Pools: []string{"tank"}})
	m.RunScrub(policy.ID)

	records := m.GetRecords(policy.ID)
	if len(records) != 1 {
		t.Errorf("期望1条记录, 得到 %d", len(records))
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager()

	m.CreatePolicy(CreatePolicyRequest{Name: "统计测试", Pools: []string{"tank"}})

	stats := m.GetStats()
	if stats.TotalPolicies != 1 {
		t.Errorf("期望1个策略, 得到 %d", stats.TotalPolicies)
	}
}
