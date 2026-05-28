package complianceengine

import (
	"context"
	"testing"
	"time"
)

func TestRunAudit(t *testing.T) {
	engine := NewEngine(nil)
	
	report := engine.RunAudit(context.Background())
	if report == nil {
		t.Fatal("expected report, got nil")
	}
	if report.TotalRules == 0 {
		t.Error("expected some rules to be checked")
	}
	if report.Score < 0 || report.Score > 100 {
		t.Errorf("score should be 0-100, got %.1f", report.Score)
	}
}

func TestReportScoring(t *testing.T) {
	engine := NewEngine(nil)
	
	// 添加一个自定义规则
	engine.RegisterRule(&Rule{
		ID:       "custom-pass",
		Name:     "Custom Pass",
		Category: "test",
		Severity: LevelPass,
		Enabled:  true,
	}, func(ctx context.Context, rule *Rule) *CheckResult {
		return &CheckResult{
			RuleID:  rule.ID,
			RuleName: rule.Name,
			Category: rule.Category,
			Status:  StatusDone,
			Level:   LevelPass,
			Message: "pass",
			CheckedAt: time.Now(),
		}
	})
	
	report := engine.RunAudit(context.Background())
	if report.Passed == 0 {
		t.Error("expected at least one pass")
	}
}

func TestCustomRule(t *testing.T) {
	engine := NewEngine(nil)
	
	engine.RegisterRule(&Rule{
		ID:          "test-001",
		Name:        "Test Rule",
		Category:    "test",
		Severity:    LevelFail,
		Description: "Test rule",
		Enabled:     true,
	}, func(ctx context.Context, rule *Rule) *CheckResult {
		return &CheckResult{
			RuleID:    rule.ID,
			RuleName:  rule.Name,
			Category:  rule.Category,
			Status:    StatusDone,
			Level:     LevelFail,
			Message:   "test failure",
			CheckedAt: time.Now(),
		}
	})
	
	report := engine.RunAudit(context.Background())
	found := false
	for _, r := range report.Results {
		if r.RuleID == "test-001" {
			found = true
			if r.Level != LevelFail {
				t.Errorf("expected fail level, got %s", r.Level)
			}
		}
	}
	if !found {
		t.Error("expected test-001 in results")
	}
}

func TestRuleEnableDisable(t *testing.T) {
	engine := NewEngine(nil)
	
	engine.RegisterRule(&Rule{
		ID:      "toggle-001",
		Name:    "Toggle Rule",
		Enabled: true,
	}, func(ctx context.Context, rule *Rule) *CheckResult {
		return &CheckResult{
			RuleID: rule.ID,
			Status: StatusDone,
			Level:  LevelPass,
			CheckedAt: time.Now(),
		}
	})
	
	// 禁用规则
	engine.DisableRule("toggle-001")
	
	report := engine.RunAudit(context.Background())
	for _, r := range report.Results {
		if r.RuleID == "toggle-001" {
			t.Error("disabled rule should not be checked")
		}
	}
	
	// 重新启用
	engine.EnableRule("toggle-001")
	report = engine.RunAudit(context.Background())
	found := false
	for _, r := range report.Results {
		if r.RuleID == "toggle-001" {
			found = true
		}
	}
	if !found {
		t.Error("re-enabled rule should be checked")
	}
}

func TestNotificationCallback(t *testing.T) {
	engine := NewEngine(nil)
	
	notified := false
	engine.SetNotifyFunc(func(report *AuditReport) {
		notified = true
	})
	
	// 添加一个会失败的规则
	engine.RegisterRule(&Rule{
		ID:       "notify-test",
		Name:     "Notify Test",
		Enabled:  true,
	}, func(ctx context.Context, rule *Rule) *CheckResult {
		return &CheckResult{
			RuleID:    rule.ID,
			Status:    StatusDone,
			Level:     LevelCritical,
			Message:   "critical issue",
			CheckedAt: time.Now(),
		}
	})
	
	engine.RunAudit(context.Background())
	if !notified {
		t.Error("expected notification for critical issue")
	}
}

func TestReportHistory(t *testing.T) {
	engine := NewEngine(nil)
	
	// 运行多次审计
	for i := 0; i < 3; i++ {
		engine.RunAudit(context.Background())
	}
	
	reports := engine.GetReports(10)
	if len(reports) != 3 {
		t.Errorf("expected 3 reports, got %d", len(reports))
	}
}

func TestGetLatestReport(t *testing.T) {
	engine := NewEngine(nil)
	
	if engine.GetLatestReport() != nil {
		t.Error("expected nil for no reports")
	}
	
	engine.RunAudit(context.Background())
	report := engine.GetLatestReport()
	if report == nil {
		t.Error("expected report after audit")
	}
}

func TestGetRules(t *testing.T) {
	engine := NewEngine(nil)
	rules := engine.GetRules()
	if len(rules) == 0 {
		t.Error("expected default rules")
	}
}

func TestAuditSummary(t *testing.T) {
	engine := NewEngine(nil)
	
	report := engine.RunAudit(context.Background())
	if report.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestEngineStartStop(t *testing.T) {
	config := DefaultEngineConfig()
	config.AutoRun = true
	config.RunInterval = 100 * time.Millisecond
	
	engine := NewEngine(config)
	engine.Start()
	time.Sleep(50 * time.Millisecond)
	engine.Stop()
}
