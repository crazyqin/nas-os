package complianceengine

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := EngineConfig{
		Enabled:       true,
		AutoScan:      true,
		MaxConcurrent: 5,
	}
	m := NewManager(config)
	if m == nil {
		t.Fatal("expected manager, got nil")
	}
	cfg := m.GetConfig()
	if !cfg.Enabled {
		t.Error("expected enabled config")
	}
	if cfg.MaxConcurrent != 5 {
		t.Errorf("expected max concurrent 5, got %d", cfg.MaxConcurrent)
	}
}

func TestMgrCreateRule(t *testing.T) {
	m := NewManager(EngineConfig{})

	rule, err := m.CreateRule(ComplianceRule{
		Standard:    StandardCIS,
		Category:    CategoryAccessControl,
		Severity:    SeverityHigh,
		Title:       "测试规则",
		Description: "测试描述",
		Requirement: "必须启用访问控制",
		Remediation: "启用 ACL",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create rule failed: %v", err)
	}
	if rule.ID == "" {
		t.Error("expected rule ID")
	}
	if rule.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestGetRule(t *testing.T) {
	m := NewManager(EngineConfig{})

	rule, _ := m.CreateRule(ComplianceRule{
		Standard: StandardCIS,
		Category: CategoryAuditLogging,
		Severity: SeverityMedium,
		Title:    "审计日志规则",
		Enabled:  true,
	})

	fetched, err := m.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("get rule failed: %v", err)
	}
	if fetched.Title != "审计日志规则" {
		t.Errorf("expected title '审计日志规则', got '%s'", fetched.Title)
	}

	_, err = m.GetRule("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestMgrListRules(t *testing.T) {
	m := NewManager(EngineConfig{})

	m.CreateRule(ComplianceRule{Standard: StandardCIS, Category: CategoryAccessControl, Title: "rule1", Enabled: true})
	m.CreateRule(ComplianceRule{Standard: StandardGDPR, Category: CategoryDataProtection, Title: "rule2", Enabled: true})
	m.CreateRule(ComplianceRule{Standard: StandardCIS, Category: CategoryNetworkSecurity, Title: "rule3", Enabled: true})

	// 列出所有
	all := m.ListRules("", "")
	if len(all) != 3 {
		t.Errorf("expected 3 rules, got %d", len(all))
	}

	// 按标准过滤
	cisRules := m.ListRules(StandardCIS, "")
	if len(cisRules) != 2 {
		t.Errorf("expected 2 CIS rules, got %d", len(cisRules))
	}

	// 按类别过滤
	dpRules := m.ListRules("", CategoryDataProtection)
	if len(dpRules) != 1 {
		t.Errorf("expected 1 data protection rule, got %d", len(dpRules))
	}
}

func TestUpdateRule(t *testing.T) {
	m := NewManager(EngineConfig{})

	rule, _ := m.CreateRule(ComplianceRule{
		Standard: StandardCIS,
		Title:    "原始标题",
		Enabled:  true,
	})

	updated, err := m.UpdateRule(rule.ID, ComplianceRule{
		Standard: StandardCIS,
		Title:    "更新后标题",
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("update rule failed: %v", err)
	}
	if updated.Title != "更新后标题" {
		t.Errorf("expected updated title, got '%s'", updated.Title)
	}
	if updated.Enabled {
		t.Error("expected rule to be disabled")
	}
	if updated.CreatedAt != rule.CreatedAt {
		t.Error("expected CreatedAt to be preserved")
	}
}

func TestDeleteRule(t *testing.T) {
	m := NewManager(EngineConfig{})

	rule, _ := m.CreateRule(ComplianceRule{Standard: StandardCIS, Title: "delete me", Enabled: true})

	err := m.DeleteRule(rule.ID)
	if err != nil {
		t.Fatalf("delete rule failed: %v", err)
	}

	_, err = m.GetRule(rule.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestMgrStartScan(t *testing.T) {
	m := NewManager(EngineConfig{})

	// 添加规则
	m.CreateRule(ComplianceRule{
		Standard: StandardCIS,
		Category: CategoryAccessControl,
		Severity: SeverityHigh,
		Title:    "访问控制",
		Enabled:  true,
	})
	m.CreateRule(ComplianceRule{
		Standard: StandardCIS,
		Category: CategoryDataProtection,
		Severity: SeverityCritical,
		Title:    "数据保护",
		Enabled:  true,
	})

	scan, err := m.StartScan([]ComplianceStandard{StandardCIS})
	if err != nil {
		t.Fatalf("start scan failed: %v", err)
	}
	if scan.ID == "" {
		t.Error("expected scan ID")
	}
	if scan.Status != StatusRunning {
		t.Errorf("expected running status, got '%s'", scan.Status)
	}

	// 等待扫描完成
	time.Sleep(500 * time.Millisecond)

	fetched, err := m.GetScan(scan.ID)
	if err != nil {
		t.Fatalf("get scan failed: %v", err)
	}
	if fetched.Status != StatusCompleted {
		t.Errorf("expected completed, got '%s'", fetched.Status)
	}
	if fetched.Score < 0 || fetched.Score > 100 {
		t.Errorf("score should be 0-100, got %.1f", fetched.Score)
	}
}

func TestListScans(t *testing.T) {
	m := NewManager(EngineConfig{})
	m.CreateRule(ComplianceRule{Standard: StandardCIS, Category: CategoryAccessControl, Title: "r1", Enabled: true})

	m.StartScan([]ComplianceStandard{StandardCIS})
	time.Sleep(300 * time.Millisecond)

	scans := m.ListScans("")
	if len(scans) == 0 {
		t.Error("expected at least 1 scan")
	}
}

func TestMgrGenerateReport(t *testing.T) {
	m := NewManager(EngineConfig{})
	m.CreateRule(ComplianceRule{
		Standard: StandardCIS,
		Category: CategoryAccessControl,
		Severity: SeverityHigh,
		Title:    "访问控制",
		Enabled:  true,
	})

	scan, _ := m.StartScan([]ComplianceStandard{StandardCIS})
	time.Sleep(500 * time.Millisecond)

	report, err := m.GenerateReport(scan.ID, FormatJSON)
	if err != nil {
		t.Fatalf("generate report failed: %v", err)
	}
	if report.ID == "" {
		t.Error("expected report ID")
	}
	if report.Format != FormatJSON {
		t.Errorf("expected JSON format, got '%s'", report.Format)
	}
	if report.Summary.TotalChecks == 0 {
		t.Error("expected some checks in summary")
	}
}

func TestGapAnalysis(t *testing.T) {
	m := NewManager(EngineConfig{})
	m.CreateRule(ComplianceRule{
		Standard: StandardGDPR,
		Category: CategoryDataProtection,
		Severity: SeverityCritical,
		Title:    "数据加密",
		Enabled:  true,
	})

	analysis, err := m.PerformGapAnalysis([]ComplianceStandard{StandardGDPR})
	if err != nil {
		t.Fatalf("gap analysis failed: %v", err)
	}
	if analysis.ID == "" {
		t.Error("expected analysis ID")
	}
	if analysis.Score < 0 || analysis.Score > 100 {
		t.Errorf("score should be 0-100, got %.1f", analysis.Score)
	}
}

func TestAlertManagement(t *testing.T) {
	m := NewManager(EngineConfig{})
	m.CreateRule(ComplianceRule{
		Standard: StandardCIS,
		Category: CategoryDataProtection,
		Severity: SeverityCritical,
		Title:    "数据保护",
		Enabled:  true,
	})

	// 触发扫描以生成告警
	scan, _ := m.StartScan([]ComplianceStandard{StandardCIS})
	time.Sleep(500 * time.Millisecond)

	alerts := m.ListAlerts("", "")
	_ = scan

	// 如果有告警，测试确认和解决
	if len(alerts) > 0 {
		alert := alerts[0]

		err := m.AcknowledgeAlert(alert.ID)
		if err != nil {
			t.Fatalf("acknowledge alert failed: %v", err)
		}

		fetched, _ := m.GetAlert(alert.ID)
		if fetched.Status != "acknowledged" {
			t.Errorf("expected acknowledged, got '%s'", fetched.Status)
		}

		err = m.ResolveAlert(alert.ID)
		if err != nil {
			t.Fatalf("resolve alert failed: %v", err)
		}
	}
}

func TestTaskManagement(t *testing.T) {
	m := NewManager(EngineConfig{})

	task, err := m.CreateTask(RemediationTask{
		RuleID:      "rule-1",
		Title:       "修复任务",
		Description: "修复描述",
		Priority:    SeverityHigh,
		Commands:    []string{"echo fix"},
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	if task.ID == "" {
		t.Error("expected task ID")
	}
	if task.Status != TaskPending {
		t.Errorf("expected pending, got '%s'", task.Status)
	}

	// 更新状态
	err = m.UpdateTaskStatus(task.ID, TaskCompleted, "修复完成")
	if err != nil {
		t.Fatalf("update task failed: %v", err)
	}

	fetched, _ := m.GetTask(task.ID)
	if fetched.Status != TaskCompleted {
		t.Errorf("expected completed, got '%s'", fetched.Status)
	}
	if fetched.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestStats(t *testing.T) {
	m := NewManager(EngineConfig{})
	m.CreateRule(ComplianceRule{Standard: StandardCIS, Category: CategoryAccessControl, Title: "r1", Enabled: true})

	m.StartScan([]ComplianceStandard{StandardCIS})
	time.Sleep(500 * time.Millisecond)

	stats := m.GetStats()
	if stats.TotalScans == 0 {
		t.Error("expected at least 1 scan")
	}
}

func TestConfigUpdate(t *testing.T) {
	m := NewManager(EngineConfig{Enabled: false})

	cfg := m.GetConfig()
	if cfg.Enabled {
		t.Error("expected disabled")
	}

	m.UpdateConfig(EngineConfig{Enabled: true, AutoScan: true})
	cfg = m.GetConfig()
	if !cfg.Enabled {
		t.Error("expected enabled after update")
	}
	if !cfg.AutoScan {
		t.Error("expected auto scan enabled")
	}
}
