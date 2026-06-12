package smartretention

import (
	"testing"
	"time"
)

func TestManager_CreateRule(t *testing.T) {
	m := NewManager()

	rule := &RetentionRule{
		ID:            "rule-custom",
		Name:          "自定义规则",
		Compliance:    ComplianceGDPR,
		RetentionDays: 365,
		WORMEnabled:   true,
		Enabled:       true,
	}

	err := m.CreateRule(rule)
	if err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}

	fetched, err := m.GetRule("rule-custom")
	if err != nil {
		t.Fatalf("获取规则失败: %v", err)
	}
	if fetched.Name != "自定义规则" {
		t.Errorf("期望名称 自定义规则, 实际 %s", fetched.Name)
	}
}

func TestManager_CreateRule_EmptyID(t *testing.T) {
	m := NewManager()

	err := m.CreateRule(&RetentionRule{ID: ""})
	if err == nil {
		t.Error("空ID应返回错误")
	}
}

func TestManager_CreateRule_Duplicate(t *testing.T) {
	m := NewManager()

	rule := &RetentionRule{ID: "rule-1", Name: "规则1"}
	m.CreateRule(rule)

	err := m.CreateRule(rule)
	if err == nil {
		t.Error("重复创建应返回错误")
	}
}

func TestManager_ListRules(t *testing.T) {
	m := NewManager()

	rules := m.ListRules()
	// 默认有4个规则
	if len(rules) < 4 {
		t.Errorf("期望至少4个规则, 实际 %d", len(rules))
	}
}

func TestManager_RegisterData(t *testing.T) {
	m := NewManager()

	data := &RetainedData{
		ID:     "data-1",
		Name:   "test.txt",
		Path:   "/data/test.txt",
		Size:   1024,
		RuleID: "rule-gdpr-default",
		Owner:  "user1",
	}

	err := m.RegisterData(data)
	if err != nil {
		t.Fatalf("注册数据失败: %v", err)
	}

	fetched, err := m.GetData("data-1")
	if err != nil {
		t.Fatalf("获取数据失败: %v", err)
	}
	if fetched.Status != StatusActive {
		t.Errorf("期望状态 active, 实际 %s", fetched.Status)
	}
}

func TestManager_RegisterData_EmptyID(t *testing.T) {
	m := NewManager()

	err := m.RegisterData(&RetainedData{ID: ""})
	if err == nil {
		t.Error("空ID应返回错误")
	}
}

func TestManager_RegisterData_RuleNotFound(t *testing.T) {
	m := NewManager()

	err := m.RegisterData(&RetainedData{ID: "data-1", RuleID: "non-existent"})
	if err == nil {
		t.Error("不存在的规则应返回错误")
	}
}

func TestManager_LockData(t *testing.T) {
	m := NewManager()

	m.RegisterData(&RetainedData{ID: "data-1", Name: "test", RuleID: "rule-gdpr-default"})

	err := m.LockData("data-1")
	if err != nil {
		t.Fatalf("锁定数据失败: %v", err)
	}

	fetched, _ := m.GetData("data-1")
	if fetched.Status != StatusLocked {
		t.Errorf("期望状态 locked, 实际 %s", fetched.Status)
	}
	if fetched.LockedAt == nil {
		t.Error("锁定时间不应为空")
	}
}

func TestManager_LockData_AlreadyLocked(t *testing.T) {
	m := NewManager()

	m.RegisterData(&RetainedData{ID: "data-1", Name: "test", RuleID: "rule-gdpr-default"})
	m.LockData("data-1")

	err := m.LockData("data-1")
	if err == nil {
		t.Error("已锁定的数据不应能再次锁定")
	}
}

func TestManager_ListData(t *testing.T) {
	m := NewManager()

	m.RegisterData(&RetainedData{ID: "data-1", Name: "a", RuleID: "rule-gdpr-default"})
	m.RegisterData(&RetainedData{ID: "data-2", Name: "b", RuleID: "rule-hipaa-default"})

	items := m.ListData("", 0)
	if len(items) != 2 {
		t.Errorf("期望2个数据项, 实际 %d", len(items))
	}

	activeItems := m.ListData(StatusActive, 0)
	if len(activeItems) != 2 {
		t.Errorf("期望2个活跃数据项, 实际 %d", len(activeItems))
	}
}

func TestManager_EvaluateRetention_Expired(t *testing.T) {
	m := NewManager()

	// 创建一个已过期的数据(使用AutoDelete=false的规则)
	m.CreateRule(&RetentionRule{
		ID: "rule-no-autodelete", Name: "不自动删除", Compliance: ComplianceCustom,
		RetentionDays: 1, WORMEnabled: false, AutoDelete: false,
		WarningDays: 7, Enabled: true,
	})
	m.RegisterData(&RetainedData{ID: "data-1", Name: "old-data.txt", RuleID: "rule-no-autodelete"})
	// 手动设置为已过期
	m.data["data-1"].RetainUntil = time.Now().Add(-24 * time.Hour)

	events := m.EvaluateRetention()
	if len(events) < 1 {
		t.Error("应有过期事件")
	}

	fetched, _ := m.GetData("data-1")
	if fetched.Status != StatusExpired {
		t.Errorf("期望状态 expired, 实际 %s", fetched.Status)
	}
}

func TestManager_EvaluateRetention_Expiring(t *testing.T) {
	m := NewManager()

	// 创建一个即将过期的数据(10天内过期)
	m.RegisterData(&RetainedData{ID: "data-1", Name: "expiring-data.txt", RuleID: "rule-gdpr-default"})
	// 注册后手动设置RetainUntil为10天后(rule-gdpr-default warning_days=30)
	m.data["data-1"].RetainUntil = time.Now().Add(10 * 24 * time.Hour)

	events := m.EvaluateRetention()
	if len(events) < 1 {
		t.Error("应有即将过期事件")
	}

	fetched, _ := m.GetData("data-1")
	if fetched.Status != StatusExpiring {
		t.Errorf("期望状态 expiring, 实际 %s", fetched.Status)
	}
}

func TestManager_ExtendRetention(t *testing.T) {
	m := NewManager()

	m.RegisterData(&RetainedData{ID: "data-1", Name: "test", RuleID: "rule-gdpr-default"})

	originalData, _ := m.GetData("data-1")
	originalExpiry := originalData.RetainUntil

	err := m.ExtendRetention("data-1", 30)
	if err != nil {
		t.Fatalf("延长保留期失败: %v", err)
	}

	fetched, _ := m.GetData("data-1")
	if !fetched.RetainUntil.After(originalExpiry) {
		t.Error("保留期应延长")
	}
}

func TestManager_ExtendRetention_Locked(t *testing.T) {
	m := NewManager()

	m.RegisterData(&RetainedData{ID: "data-1", Name: "test", RuleID: "rule-gdpr-default"})
	m.LockData("data-1")

	err := m.ExtendRetention("data-1", 30)
	if err == nil {
		t.Error("已锁定的数据不应能延长保留期")
	}
}

func TestManager_DeleteData(t *testing.T) {
	m := NewManager()

	m.RegisterData(&RetainedData{ID: "data-1", Name: "test", RuleID: "rule-gdpr-default"})

	err := m.DeleteData("data-1")
	if err != nil {
		t.Fatalf("删除数据失败: %v", err)
	}

	fetched, _ := m.GetData("data-1")
	if fetched.Status != StatusDeleted {
		t.Errorf("期望状态 deleted, 实际 %s", fetched.Status)
	}
}

func TestManager_DeleteData_Locked(t *testing.T) {
	m := NewManager()

	m.RegisterData(&RetainedData{ID: "data-1", Name: "test", RuleID: "rule-gdpr-default"})
	m.LockData("data-1")

	err := m.DeleteData("data-1")
	if err == nil {
		t.Error("已锁定(WORM)的数据不应能删除")
	}
}

func TestManager_GetEvents(t *testing.T) {
	m := NewManager()

	m.RegisterData(&RetainedData{ID: "data-1", Name: "test", RuleID: "rule-gdpr-default"})
	m.LockData("data-1")

	events := m.GetEvents("", 0)
	if len(events) < 2 {
		t.Errorf("期望至少2条事件, 实际 %d", len(events))
	}

	filtered := m.GetEvents("data-1", 0)
	if len(filtered) < 2 {
		t.Errorf("期望至少2条事件, 实际 %d", len(filtered))
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager()

	m.RegisterData(&RetainedData{ID: "data-1", Name: "a", RuleID: "rule-gdpr-default"})
	m.RegisterData(&RetainedData{ID: "data-2", Name: "b", RuleID: "rule-hipaa-default"})

	stats := m.GetStats()
	if stats["total_data"] != 2 {
		t.Errorf("期望2个数据项, 实际 %v", stats["total_data"])
	}
}

func TestManager_DefaultRules(t *testing.T) {
	m := NewManager()

	rules := m.ListRules()
	complianceTypes := make(map[ComplianceType]bool)
	for _, r := range rules {
		complianceTypes[r.Compliance] = true
	}

	if !complianceTypes[ComplianceGDPR] {
		t.Error("应有GDPR默认规则")
	}
	if !complianceTypes[ComplianceHIPAA] {
		t.Error("应有HIPAA默认规则")
	}
	if !complianceTypes[ComplianceSOX] {
		t.Error("应有SOX默认规则")
	}
}

func TestManager_WORMProtection(t *testing.T) {
	m := NewManager()

	// 注册数据
	m.RegisterData(&RetainedData{ID: "data-1", Name: "sensitive.txt", RuleID: "rule-gdpr-default"})

	// 锁定(WORM)
	m.LockData("data-1")

	// 尝试删除 - 应失败
	err := m.DeleteData("data-1")
	if err == nil {
		t.Error("WORM保护的数据不应能删除")
	}

	// 尝试延长保留期 - 应失败
	err = m.ExtendRetention("data-1", 30)
	if err == nil {
		t.Error("WORM保护的数据不应能修改保留期")
	}
}
