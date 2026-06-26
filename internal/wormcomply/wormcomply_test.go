package wormcomply

import (
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if len(engine.policies) != 0 {
		t.Fatalf("expected 0 policies, got %d", len(engine.policies))
	}
}

func TestCreatePolicy(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	policy := &RetentionPolicy{
		ID:            "pol-001",
		Name:          "SOX 7-Year Retention",
		Description:   "SOX compliance: retain financial records for 7 years",
		Regulation:    RegSOX,
		RetentionDays: 2555, // 7 years
		Action:        ActionRetain,
		Level:         LevelStrict,
		SharePaths:    []string{"/shares/finance"},
		CreatedBy:     "admin",
	}

	if err := engine.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	if policy.Version != 1 {
		t.Errorf("expected version 1, got %d", policy.Version)
	}
	if policy.State != PolicyStateActive {
		t.Errorf("expected state active, got %s", policy.State)
	}
	if policy.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	// 重复创建应失败
	if err := engine.CreatePolicy(policy); err != ErrPolicyExists {
		t.Errorf("expected ErrPolicyExists, got %v", err)
	}
}

func TestCreatePolicyValidation(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	cases := []struct {
		name   string
		policy *RetentionPolicy
		err    error
	}{
		{
			name:   "empty ID",
			policy: &RetentionPolicy{Name: "test", RetentionDays: 30},
			err:    ErrInvalidPolicy,
		},
		{
			name:   "empty name",
			policy: &RetentionPolicy{ID: "p1", RetentionDays: 30},
			err:    ErrInvalidPolicy,
		},
		{
			name:   "negative retention",
			policy: &RetentionPolicy{ID: "p1", Name: "test", RetentionDays: -1},
			err:    ErrInvalidPolicy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := engine.CreatePolicy(tc.policy); err != tc.err {
				t.Errorf("expected %v, got %v", tc.err, err)
			}
		})
	}
}

func TestUpdatePolicy(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	policy := &RetentionPolicy{
		ID:            "pol-002",
		Name:          "HIPAA Policy",
		Regulation:    RegHIPAA,
		RetentionDays: 2190, // 6 years
		Action:        ActionRetain,
		CreatedBy:     "admin",
	}
	engine.CreatePolicy(policy)

	updated := *policy
	updated.RetentionDays = 2555
	updated.Name = "HIPAA Policy Updated"

	if err := engine.UpdatePolicy("pol-002", &updated); err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}

	got, err := engine.GetPolicy("pol-002")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if got.Version != 2 {
		t.Errorf("expected version 2, got %d", got.Version)
	}
	if got.Name != "HIPAA Policy Updated" {
		t.Errorf("expected updated name, got %s", got.Name)
	}
}

func TestDeletePolicy(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	policy := &RetentionPolicy{
		ID:         "pol-003",
		Name:       "GDPR Policy",
		Regulation: RegGDPR,
		CreatedBy:  "admin",
	}
	engine.CreatePolicy(policy)

	if err := engine.DeletePolicy("pol-003", "admin"); err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	if _, err := engine.GetPolicy("pol-003"); err != ErrPolicyNotFound {
		t.Errorf("expected ErrPolicyNotFound, got %v", err)
	}
}

func TestCheckFileAccess(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	policy := &RetentionPolicy{
		ID:         "pol-004",
		Name:       "SOX Finance",
		Regulation: RegSOX,
		SharePaths: []string{"/shares/finance"},
		Level:      LevelStrict,
		CreatedBy:  "admin",
	}
	engine.CreatePolicy(policy)

	// 删除受保护文件应失败
	err := engine.CheckFileAccess("/shares/finance/report.pdf", "delete", "user1")
	if err == nil {
		t.Error("expected error for deleting protected file")
	}

	// 检查错误类型
	pve, ok := err.(*PolicyViolationError)
	if !ok {
		t.Errorf("expected PolicyViolationError, got %T", err)
	}
	if pve.PolicyID != "pol-004" {
		t.Errorf("expected policy pol-004, got %s", pve.PolicyID)
	}

	// 非保护路径应允许操作
	err = engine.CheckFileAccess("/shares/public/readme.txt", "delete", "user1")
	if err != nil {
		t.Errorf("expected nil error for unprotected path, got %v", err)
	}

	// 读取操作应允许
	err = engine.CheckFileAccess("/shares/finance/report.pdf", "read", "user1")
	if err != nil {
		t.Errorf("expected nil error for read operation, got %v", err)
	}
}

func TestSuspendActivatePolicy(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	policy := &RetentionPolicy{
		ID:         "pol-005",
		Name:       "Test Policy",
		Regulation: RegCustom,
		SharePaths: []string{"/shares/data"},
		CreatedBy:  "admin",
	}
	engine.CreatePolicy(policy)

	// 暂停后应允许文件操作
	if err := engine.SuspendPolicy("pol-005", "admin"); err != nil {
		t.Fatalf("SuspendPolicy failed: %v", err)
	}

	err := engine.CheckFileAccess("/shares/data/file.txt", "delete", "user1")
	if err != nil {
		t.Errorf("expected nil error when policy suspended, got %v", err)
	}

	// 重新激活后应阻止操作
	if err := engine.ActivatePolicy("pol-005", "admin"); err != nil {
		t.Fatalf("ActivatePolicy failed: %v", err)
	}

	err = engine.CheckFileAccess("/shares/data/file.txt", "delete", "user1")
	if err == nil {
		t.Error("expected error after policy reactivated")
	}
}

func TestListPoliciesByRegulation(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	engine.CreatePolicy(&RetentionPolicy{ID: "s1", Name: "SOX1", Regulation: RegSOX, CreatedBy: "a"})
	engine.CreatePolicy(&RetentionPolicy{ID: "s2", Name: "SOX2", Regulation: RegSOX, CreatedBy: "a"})
	engine.CreatePolicy(&RetentionPolicy{ID: "h1", Name: "HIPAA1", Regulation: RegHIPAA, CreatedBy: "a"})

	soxPolicies := engine.ListPoliciesByRegulation(RegSOX)
	if len(soxPolicies) != 2 {
		t.Errorf("expected 2 SOX policies, got %d", len(soxPolicies))
	}

	hipaaPolicies := engine.ListPoliciesByRegulation(RegHIPAA)
	if len(hipaaPolicies) != 1 {
		t.Errorf("expected 1 HIPAA policy, got %d", len(hipaaPolicies))
	}
}

func TestRecordAndResolveViolation(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	engine.CreatePolicy(&RetentionPolicy{ID: "p1", Name: "Test", Regulation: RegGDPR, CreatedBy: "a"})

	v := &ComplianceViolation{
		ID:            "vio-001",
		PolicyID:      "p1",
		FilePath:      "/shares/gdpr/data.csv",
		ViolationType: "delete_blocked",
		Description:   "Attempted delete on GDPR-protected file",
		UserID:        "user1",
		Timestamp:     time.Now(),
	}
	engine.RecordViolation(v)

	violations := engine.GetViolations(true)
	if len(violations) != 1 {
		t.Fatalf("expected 1 unresolved violation, got %d", len(violations))
	}

	if err := engine.ResolveViolation("vio-001", "admin"); err != nil {
		t.Fatalf("ResolveViolation failed: %v", err)
	}

	violations = engine.GetViolations(true)
	if len(violations) != 0 {
		t.Errorf("expected 0 unresolved violations after resolve, got %d", len(violations))
	}
}

func TestGenerateReport(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	engine.CreatePolicy(&RetentionPolicy{ID: "p1", Name: "SOX", Regulation: RegSOX, CreatedBy: "a", State: PolicyStateActive})
	engine.CreatePolicy(&RetentionPolicy{ID: "p2", Name: "HIPAA", Regulation: RegHIPAA, CreatedBy: "a", State: PolicyStateSuspended})

	engine.RecordViolation(&ComplianceViolation{
		ID:        "v1",
		PolicyID:  "p1",
		FilePath:  "/test",
		UserID:    "u1",
		Timestamp: time.Now(),
	})
	engine.RecordViolation(&ComplianceViolation{
		ID:        "v2",
		PolicyID:  "p1",
		FilePath:  "/test2",
		UserID:    "u1",
		Timestamp: time.Now(),
	})
	engine.ResolveViolation("v1", "admin")

	report := engine.GenerateReport("daily")
	if report.TotalPolicies != 2 {
		t.Errorf("expected 2 total policies, got %d", report.TotalPolicies)
	}
	if report.ActivePolicies != 1 {
		t.Errorf("expected 1 active policy, got %d", report.ActivePolicies)
	}
	if report.Violations != 2 {
		t.Errorf("expected 2 violations, got %d", report.Violations)
	}
	if report.UnresolvedVios != 1 {
		t.Errorf("expected 1 unresolved, got %d", report.UnresolvedVios)
	}
	if report.Score != 50 {
		t.Errorf("expected score 50, got %f", report.Score)
	}
	if report.ByRegulation[RegSOX] != 1 {
		t.Errorf("expected 1 SOX policy, got %d", report.ByRegulation[RegSOX])
	}
}

func TestAuditLog(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	engine.CreatePolicy(&RetentionPolicy{ID: "p1", Name: "Test", Regulation: RegSOX, CreatedBy: "admin"})

	log := engine.GetAuditLog(10, "")
	if len(log) == 0 {
		t.Fatal("expected at least 1 audit entry")
	}
	if log[0].Action != "create" {
		t.Errorf("expected first action 'create', got %s", log[0].Action)
	}

	// 过滤特定策略
	log = engine.GetAuditLog(10, "p1")
	if len(log) == 0 {
		t.Fatal("expected audit entries for policy p1")
	}

	log = engine.GetAuditLog(10, "nonexistent")
	if len(log) != 0 {
		t.Errorf("expected 0 entries for nonexistent policy, got %d", len(log))
	}
}
