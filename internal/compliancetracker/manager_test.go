// Package compliancetracker 提供合规审计追踪功能
package compliancetracker

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := DefaultComplianceConfig()
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager 返回 nil")
	}

	// 测试默认配置
	manager2 := NewManager(nil)
	if manager2 == nil {
		t.Fatal("NewManager(nil) 返回 nil")
	}
}

func TestDefaultComplianceConfig(t *testing.T) {
	config := DefaultComplianceConfig()

	if !config.AutoCheckEnabled {
		t.Error("期望 AutoCheckEnabled 为 true")
	}

	if config.AutoCheckInterval != 60 {
		t.Errorf("期望 AutoCheckInterval 为 60，得到 %d", config.AutoCheckInterval)
	}

	if config.AlertThreshold != 80.0 {
		t.Errorf("期望 AlertThreshold 为 80.0，得到 %f", config.AlertThreshold)
	}

	if config.RetentionDays != 90 {
		t.Errorf("期望 RetentionDays 为 90，得到 %d", config.RetentionDays)
	}
}

func TestAddRule(t *testing.T) {
	manager := NewManager(nil)

	rule := &ComplianceRule{
		ID:       "rule-001",
		Name:     "密码强度规则",
		RuleType: RuleTypeAccess,
		Severity: SeverityHigh,
		Enabled:  true,
		Conditions: []Condition{
			{Field: "password_length", Operator: "gte", Value: "8"},
		},
	}

	err := manager.AddRule(rule)
	if err != nil {
		t.Fatalf("添加规则失败: %v", err)
	}

	// 验证规则已添加
	added, err := manager.GetRule("rule-001")
	if err != nil {
		t.Fatalf("获取规则失败: %v", err)
	}

	if added.Name != "密码强度规则" {
		t.Errorf("期望规则名称为 '密码强度规则'，得到 '%s'", added.Name)
	}
}

func TestAddDuplicateRule(t *testing.T) {
	manager := NewManager(nil)

	rule := &ComplianceRule{
		ID:   "rule-001",
		Name: "测试规则",
	}

	err := manager.AddRule(rule)
	if err != nil {
		t.Fatalf("添加规则失败: %v", err)
	}

	// 添加重复规则
	err = manager.AddRule(rule)
	if err != ErrDuplicateRule {
		t.Errorf("期望 ErrDuplicateRule，得到 %v", err)
	}
}

func TestAddInvalidRule(t *testing.T) {
	manager := NewManager(nil)

	// 空名称规则
	rule := &ComplianceRule{
		ID: "rule-001",
	}

	err := manager.AddRule(rule)
	if err != ErrInvalidRule {
		t.Errorf("期望 ErrInvalidRule，得到 %v", err)
	}
}

func TestUpdateRule(t *testing.T) {
	manager := NewManager(nil)

	// 添加规则
	rule := &ComplianceRule{
		ID:   "rule-001",
		Name: "原始名称",
	}
	manager.AddRule(rule)

	// 更新规则
	rule.Name = "更新后的名称"
	err := manager.UpdateRule(rule)
	if err != nil {
		t.Fatalf("更新规则失败: %v", err)
	}

	// 验证更新
	updated, _ := manager.GetRule("rule-001")
	if updated.Name != "更新后的名称" {
		t.Errorf("期望名称为 '更新后的名称'，得到 '%s'", updated.Name)
	}
}

func TestUpdateNonExistentRule(t *testing.T) {
	manager := NewManager(nil)

	rule := &ComplianceRule{
		ID:   "non-existent",
		Name: "测试",
	}

	err := manager.UpdateRule(rule)
	if err != ErrRuleNotFound {
		t.Errorf("期望 ErrRuleNotFound，得到 %v", err)
	}
}

func TestDeleteRule(t *testing.T) {
	manager := NewManager(nil)

	rule := &ComplianceRule{
		ID:   "rule-001",
		Name: "测试规则",
	}
	manager.AddRule(rule)

	err := manager.DeleteRule("rule-001")
	if err != nil {
		t.Fatalf("删除规则失败: %v", err)
	}

	// 验证已删除
	_, err = manager.GetRule("rule-001")
	if err != ErrRuleNotFound {
		t.Errorf("期望 ErrRuleNotFound，得到 %v", err)
	}
}

func TestDeleteNonExistentRule(t *testing.T) {
	manager := NewManager(nil)

	err := manager.DeleteRule("non-existent")
	if err != ErrRuleNotFound {
		t.Errorf("期望 ErrRuleNotFound，得到 %v", err)
	}
}

func TestListRules(t *testing.T) {
	manager := NewManager(nil)

	// 添加多个规则
	rules := []*ComplianceRule{
		{ID: "rule-001", Name: "规则A", RuleType: RuleTypeAccess, Enabled: true},
		{ID: "rule-002", Name: "规则B", RuleType: RuleTypeEncryption, Enabled: true},
		{ID: "rule-003", Name: "规则C", RuleType: RuleTypeAccess, Enabled: false},
	}

	for _, rule := range rules {
		manager.AddRule(rule)
	}

	// 列出所有规则
	allRules := manager.ListRules("", nil)
	if len(allRules) != 3 {
		t.Errorf("期望3条规则，得到 %d", len(allRules))
	}

	// 按类型过滤
	accessRules := manager.ListRules(RuleTypeAccess, nil)
	if len(accessRules) != 2 {
		t.Errorf("期望2条访问规则，得到 %d", len(accessRules))
	}

	// 按启用状态过滤
	enabled := true
	enabledRules := manager.ListRules("", &enabled)
	if len(enabledRules) != 2 {
		t.Errorf("期望2条启用规则，得到 %d", len(enabledRules))
	}
}

func TestRunCheck(t *testing.T) {
	manager := NewManager(nil)

	// 添加规则
	rule := &ComplianceRule{
		ID:       "rule-001",
		Name:     "测试规则",
		Enabled:  true,
		Severity: SeverityHigh,
		Conditions: []Condition{
			{Field: "value", Operator: "gte", Value: "10"},
		},
	}
	manager.AddRule(rule)

	// 执行检查 - 合规情况
	check, err := manager.RunCheck("rule-001", "15", "system", "tester")
	if err != nil {
		t.Fatalf("执行检查失败: %v", err)
	}

	if check.Status != StatusCompliant {
		t.Errorf("期望状态为 compliant，得到 %s", check.Status)
	}

	// 执行检查 - 不合规情况
	check, err = manager.RunCheck("rule-001", "5", "system", "tester")
	if err != nil {
		t.Fatalf("执行检查失败: %v", err)
	}

	if check.Status != StatusNonCompliant {
		t.Errorf("期望状态为 non_compliant，得到 %s", check.Status)
	}

	if len(check.Violations) == 0 {
		t.Error("期望有违规项")
	}
}

func TestRunCheckNonExistentRule(t *testing.T) {
	manager := NewManager(nil)

	_, err := manager.RunCheck("non-existent", "target", "system", "tester")
	if err != ErrRuleNotFound {
		t.Errorf("期望 ErrRuleNotFound，得到 %v", err)
	}
}

func TestRunCheckDisabledRule(t *testing.T) {
	manager := NewManager(nil)

	rule := &ComplianceRule{
		ID:      "rule-001",
		Name:    "禁用规则",
		Enabled: false,
	}
	manager.AddRule(rule)

	_, err := manager.RunCheck("rule-001", "target", "system", "tester")
	if err != ErrInvalidRule {
		t.Errorf("期望 ErrInvalidRule，得到 %v", err)
	}
}

func TestRunAllChecks(t *testing.T) {
	manager := NewManager(nil)

	// 添加多个规则
	rules := []*ComplianceRule{
		{ID: "rule-001", Name: "规则A", Enabled: true, Conditions: []Condition{{Field: "value", Operator: "gte", Value: "10"}}},
		{ID: "rule-002", Name: "规则B", Enabled: true, Conditions: []Condition{{Field: "value", Operator: "lt", Value: "100"}}},
		{ID: "rule-003", Name: "规则C", Enabled: false}, // 禁用的规则
	}

	for _, rule := range rules {
		manager.AddRule(rule)
	}

	checks, err := manager.RunAllChecks("50", "system", "tester")
	if err != nil {
		t.Fatalf("执行所有检查失败: %v", err)
	}

	// 应该只有2个检查（禁用的规则不执行）
	if len(checks) != 2 {
		t.Errorf("期望2个检查结果，得到 %d", len(checks))
	}
}

func TestQueryChecks(t *testing.T) {
	manager := NewManager(nil)

	// 添加规则并执行检查
	rule := &ComplianceRule{
		ID:         "rule-001",
		Name:       "测试规则",
		Enabled:    true,
		Conditions: []Condition{{Field: "value", Operator: "gte", Value: "10"}},
	}
	manager.AddRule(rule)

	for i := 0; i < 10; i++ {
		manager.RunCheck("rule-001", "target", "system", "tester")
	}

	// 查询所有
	filter := QueryFilter{}
	checks := manager.QueryChecks(filter)
	if len(checks) != 10 {
		t.Errorf("期望10条记录，得到 %d", len(checks))
	}

	// 分页查询
	filter = QueryFilter{Limit: 5}
	checks = manager.QueryChecks(filter)
	if len(checks) != 5 {
		t.Errorf("期望5条记录，得到 %d", len(checks))
	}
}

func TestGenerateReport(t *testing.T) {
	manager := NewManager(nil)

	// 添加规则并执行检查
	rule := &ComplianceRule{
		ID:         "rule-001",
		Name:       "测试规则",
		Enabled:    true,
		Severity:   SeverityHigh,
		Conditions: []Condition{{Field: "value", Operator: "gte", Value: "10"}},
	}
	manager.AddRule(rule)

	// 添加一些合规和不合规的检查
	manager.RunCheck("rule-001", "15", "system", "tester") // 合规
	manager.RunCheck("rule-001", "5", "system", "tester")  // 不合规
	manager.RunCheck("rule-001", "20", "system", "tester") // 合规

	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Hour)

	report, err := manager.GenerateReport(startTime, endTime)
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	if report.TotalChecks != 3 {
		t.Errorf("期望3条记录，得到 %d", report.TotalChecks)
	}

	if report.CompliantCount != 2 {
		t.Errorf("期望2条合规，得到 %d", report.CompliantCount)
	}

	if report.NonCompliantCount != 1 {
		t.Errorf("期望1条不合规，得到 %d", report.NonCompliantCount)
	}

	if report.ID == "" {
		t.Error("报告ID未生成")
	}
}

func TestGenerateReportInvalidTimeRange(t *testing.T) {
	manager := NewManager(nil)

	startTime := time.Now()
	endTime := startTime.Add(-1 * time.Hour)

	_, err := manager.GenerateReport(startTime, endTime)
	if err != ErrInvalidTimeRange {
		t.Errorf("期望 ErrInvalidTimeRange，得到 %v", err)
	}
}

func TestGetAuditLogs(t *testing.T) {
	manager := NewManager(nil)

	// 添加规则触发审计日志
	rule := &ComplianceRule{
		ID:   "rule-001",
		Name: "测试规则",
	}
	manager.AddRule(rule)

	// 获取审计日志
	filter := QueryFilter{}
	logs := manager.GetAuditLogs(filter)

	if len(logs) == 0 {
		t.Error("期望有审计日志")
	}

	// 验证日志内容
	found := false
	for _, log := range logs {
		if log.Action == "add_rule" {
			found = true
			break
		}
	}
	if !found {
		t.Error("未找到添加规则的审计日志")
	}
}

func TestGetComplianceStats(t *testing.T) {
	manager := NewManager(nil)

	// 添加规则
	rule := &ComplianceRule{
		ID:         "rule-001",
		Name:       "测试规则",
		Enabled:    true,
		Conditions: []Condition{{Field: "value", Operator: "gte", Value: "10"}},
	}
	manager.AddRule(rule)

	// 执行检查
	manager.RunCheck("rule-001", "15", "system", "tester")
	manager.RunCheck("rule-001", "5", "system", "tester")

	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Hour)

	stats, err := manager.GetComplianceStats(startTime, endTime)
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	totalChecks, ok := stats["total_checks"].(int)
	if !ok || totalChecks != 2 {
		t.Errorf("期望 total_checks 为 2，得到 %v", stats["total_checks"])
	}

	ruleCount, ok := stats["rule_count"].(int)
	if !ok || ruleCount != 1 {
		t.Errorf("期望 rule_count 为 1，得到 %v", stats["rule_count"])
	}
}

func TestGetComplianceStatsInvalidTimeRange(t *testing.T) {
	manager := NewManager(nil)

	startTime := time.Now()
	endTime := startTime.Add(-1 * time.Hour)

	_, err := manager.GetComplianceStats(startTime, endTime)
	if err != ErrInvalidTimeRange {
		t.Errorf("期望 ErrInvalidTimeRange，得到 %v", err)
	}
}

func TestConditionOperators(t *testing.T) {
	manager := NewManager(nil)

	tests := []struct {
		name      string
		condition Condition
		actual    string
		expected  bool
	}{
		{"eq equal", Condition{Operator: "eq", Value: "10"}, "10", true},
		{"eq not equal", Condition{Operator: "eq", Value: "10"}, "20", false},
		{"ne equal", Condition{Operator: "ne", Value: "10"}, "10", false},
		{"ne not equal", Condition{Operator: "ne", Value: "10"}, "20", true},
		{"gt greater", Condition{Operator: "gt", Value: "10"}, "20", true},
		{"gt less", Condition{Operator: "gt", Value: "10"}, "5", false},
		{"lt less", Condition{Operator: "lt", Value: "10"}, "5", true},
		{"lt greater", Condition{Operator: "lt", Value: "10"}, "20", false},
		{"gte equal", Condition{Operator: "gte", Value: "10"}, "10", true},
		{"gte greater", Condition{Operator: "gte", Value: "10"}, "20", true},
		{"gte less", Condition{Operator: "gte", Value: "10"}, "5", false},
		{"lte equal", Condition{Operator: "lte", Value: "10"}, "10", true},
		{"lte less", Condition{Operator: "lte", Value: "10"}, "5", true},
		{"lte greater", Condition{Operator: "lte", Value: "10"}, "20", false},
		{"contains found", Condition{Operator: "contains", Value: "test"}, "this is a test", true},
		{"contains not found", Condition{Operator: "contains", Value: "test"}, "hello world", false},
		{"regex match", Condition{Operator: "regex", Value: "^test.*"}, "testing", true},
		{"regex no match", Condition{Operator: "regex", Value: "^test.*"}, "hello", false},
		{"unknown operator", Condition{Operator: "unknown", Value: "10"}, "10", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.evaluateCondition(tt.condition, tt.actual)
			if result != tt.expected {
				t.Errorf("条件 %s %s %s, 实际值 '%s': 期望 %v, 得到 %v",
					tt.condition.Field, tt.condition.Operator, tt.condition.Value,
					tt.actual, tt.expected, result)
			}
		})
	}
}

func TestComplianceStatusConstants(t *testing.T) {
	statuses := []ComplianceStatus{
		StatusCompliant,
		StatusNonCompliant,
		StatusPartial,
		StatusPending,
		StatusError,
	}

	expected := []string{"compliant", "non_compliant", "partial", "pending", "error"}

	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("状态 %d: 期望 '%s', 得到 '%s'", i, expected[i], string(status))
		}
	}
}

func TestSeverityLevelConstants(t *testing.T) {
	levels := []SeverityLevel{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}

	expected := []string{"low", "medium", "high", "critical"}

	for i, level := range levels {
		if string(level) != expected[i] {
			t.Errorf("严重程度 %d: 期望 '%s', 得到 '%s'", i, expected[i], string(level))
		}
	}
}

func TestRuleTypeConstants(t *testing.T) {
	types := []RuleType{
		RuleTypeAccess,
		RuleTypeEncryption,
		RuleTypeRetention,
		RuleTypeAudit,
		RuleTypePrivacy,
		RuleTypeCustom,
	}

	expected := []string{"access", "encryption", "retention", "audit", "privacy", "custom"}

	for i, ruleType := range types {
		if string(ruleType) != expected[i] {
			t.Errorf("规则类型 %d: 期望 '%s', 得到 '%s'", i, expected[i], string(ruleType))
		}
	}
}
