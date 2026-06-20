package smartcompliance

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestDefaultRules(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	rules := engine.ListRules("")
	if len(rules) < 5 {
		t.Errorf("expected at least 5 default rules, got %d", len(rules))
	}
}

func TestRunAudit(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	result, err := engine.RunAudit(StandardGDPR)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.Status != AuditStatusComplete {
		t.Errorf("expected status complete, got %s", result.Status)
	}
	if result.TotalChecks == 0 {
		t.Error("expected at least one check")
	}
}

func TestAccessPolicy(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	engine.AddAccessPolicy(&AccessPolicy{
		ID:       "policy-1",
		Subject:  "admin",
		Resource: "/api/*",
		Actions:  []string{"read", "write", "delete"},
		Effect:   "allow",
		Priority: 100,
		Enabled:  true,
	})
	
	if !engine.CheckAccess("admin", "/api/v1/users", "read") {
		t.Error("expected admin to have read access")
	}
	
	if engine.CheckAccess("guest", "/api/v1/users", "read") {
		t.Error("expected guest to be denied")
	}
}

func TestCheckAccessDefaultDeny(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	// 没有匹配的策略，默认拒绝
	if engine.CheckAccess("user", "/secret", "read") {
		t.Error("expected default deny")
	}
}

func TestGetComplianceStatus(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	engine.RunAudit(StandardGDPR)
	
	status := engine.GetComplianceStatus()
	
	if status["total_rules"] == 0 {
		t.Error("expected rules to be present")
	}
	if status["total_audits"] == 0 {
		t.Error("expected at least one audit")
	}
}
