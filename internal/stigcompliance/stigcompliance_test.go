package stigcompliance

import (
	"testing"
)

func TestNewChecker(t *testing.T) {
	cfg := DefaultCheckerConfig()
	checker := NewSTIGComplianceChecker(cfg)
	if checker == nil {
		t.Fatal("checker should not be nil")
	}
	if checker.GetRuleCount() < 10 {
		t.Errorf("expected at least 10 default rules, got %d", checker.GetRuleCount())
	}
}

func TestAddRule(t *testing.T) {
	checker := NewSTIGComplianceChecker(DefaultCheckerConfig())

	rule := &STIGRule{
		ID:          "V-250099",
		Title:       "测试规则",
		Description: "测试用规则",
		Severity:    SeverityCat2,
		Category:    "测试",
		Enabled:     true,
	}

	if err := checker.AddRule(rule); err != nil {
		t.Fatalf("add rule failed: %v", err)
	}
	if err := checker.AddRule(rule); err != ErrRuleExists {
		t.Errorf("expected ErrRuleExists, got %v", err)
	}
}

func TestRemoveRule(t *testing.T) {
	checker := NewSTIGComplianceChecker(DefaultCheckerConfig())

	if err := checker.RemoveRule("V-250001"); err != nil {
		t.Fatalf("remove rule failed: %v", err)
	}
	if err := checker.RemoveRule("V-250001"); err != ErrRuleNotFound {
		t.Errorf("expected ErrRuleNotFound, got %v", err)
	}
}

func TestGetRule(t *testing.T) {
	checker := NewSTIGComplianceChecker(DefaultCheckerConfig())

	rule, err := checker.GetRule("V-250001")
	if err != nil {
		t.Fatalf("get rule failed: %v", err)
	}
	if rule.Title != "密码复杂度要求" {
		t.Errorf("expected 密码复杂度要求, got %s", rule.Title)
	}

	_, err = checker.GetRule("nonexistent")
	if err != ErrRuleNotFound {
		t.Errorf("expected ErrRuleNotFound, got %v", err)
	}
}

func TestRunAudit(t *testing.T) {
	checker := NewSTIGComplianceChecker(DefaultCheckerConfig())

	report := checker.RunAudit()
	if report == nil {
		t.Fatal("report should not be nil")
	}
	if report.TotalRules == 0 {
		t.Error("should have at least some rules to check")
	}
	if report.Score < 0 || report.Score > 100 {
		t.Errorf("score should be 0-100, got %f", report.Score)
	}
	if report.GeneratedAt.IsZero() {
		t.Error("generated_at should be set")
	}
}

func TestReportHistory(t *testing.T) {
	checker := NewSTIGComplianceChecker(DefaultCheckerConfig())

	checker.RunAudit()
	checker.RunAudit()

	history := checker.GetReportHistory()
	if len(history) != 2 {
		t.Errorf("expected 2 reports, got %d", len(history))
	}

	latest := checker.GetLatestReport()
	if latest == nil {
		t.Fatal("latest report should not be nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultCheckerConfig()
	if !cfg.FailOnCat1 {
		t.Error("should fail on cat1 by default")
	}
	if cfg.FailOnCat2 {
		t.Error("should not fail on cat2 by default")
	}
}
