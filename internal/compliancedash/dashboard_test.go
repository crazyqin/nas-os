package compliancedash

import (
	"testing"
	"time"
)

func TestNewComplianceDashboardManager(t *testing.T) {
	mgr := NewComplianceDashboardManager(nil)
	if mgr == nil {
		t.Fatal("NewComplianceDashboardManager returned nil")
	}
	if mgr.config.ScoreThreshold != 70 {
		t.Errorf("expected threshold 70, got %d", mgr.config.ScoreThreshold)
	}
}

func TestDefaultChecksRegistered(t *testing.T) {
	mgr := NewComplianceDashboardManager(nil)
	if len(mgr.checks) < 10 {
		t.Errorf("expected at least 10 default checks, got %d", len(mgr.checks))
	}
}

func TestRunAllChecks(t *testing.T) {
	mgr := NewComplianceDashboardManager(&DashboardConfig{
		Enabled:       true,
		CheckInterval: 1 * time.Hour,
	})
	mgr.RunAllChecks()
	score := mgr.GetScore()
	if score == nil {
		t.Fatal("GetScore returned nil")
	}
	if score.TotalChecks < 10 {
		t.Errorf("expected at least 10 checks, got %d", score.TotalChecks)
	}
	if score.Overall < 0 || score.Overall > 100 {
		t.Errorf("score out of range: %d", score.Overall)
	}
}

func TestGetResults(t *testing.T) {
	mgr := NewComplianceDashboardManager(nil)
	mgr.RunAllChecks()
	results := mgr.GetResults()
	if len(results) < 10 {
		t.Errorf("expected at least 10 results, got %d", len(results))
	}
	for _, r := range results {
		if r.CheckedAt.IsZero() {
			t.Error("CheckedAt should not be zero")
		}
	}
}

func TestGetReport(t *testing.T) {
	mgr := NewComplianceDashboardManager(nil)
	mgr.RunAllChecks()
	report := mgr.GetReport()
	if report == "" {
		t.Error("report should not be empty")
	}
}

func TestGetTrendsEmpty(t *testing.T) {
	mgr := NewComplianceDashboardManager(nil)
	trends := mgr.GetTrends(30)
	if len(trends) != 0 {
		t.Errorf("expected 0 trends, got %d", len(trends))
	}
}

func TestSeverityWeights(t *testing.T) {
	if severityWeight(SeverityCritical) != 5 {
		t.Error("critical should be weight 5")
	}
	if severityWeight(SeverityHigh) != 4 {
		t.Error("high should be weight 4")
	}
	if severityWeight(SeverityMedium) != 3 {
		t.Error("medium should be weight 3")
	}
}

func TestRegisterCustomCheck(t *testing.T) {
	mgr := NewComplianceDashboardManager(nil)
	mgr.RegisterCheck(&ComplianceCheck{
		ID:       "custom-001",
		Name:     "自定义检查",
		Category: CategoryStorage,
		Severity: SeverityLow,
		CheckFunc: func() *CheckResult {
			return &CheckResult{Passed: true, Score: 100, Message: "ok"}
		},
	})
	mgr.RunAllChecks()
	results := mgr.GetResults()
	if _, ok := results["custom-001"]; !ok {
		t.Error("custom check result not found")
	}
}
