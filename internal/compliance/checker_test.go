package compliance

import (
	"context"
	"testing"
	"time"
)

// mockCheck 模拟检查项
type mockCheck struct {
	id          string
	checkType   CheckType
	name        string
	description string
	passed      bool
	err         error
}

func (m *mockCheck) ID() string          { return m.id }
func (m *mockCheck) Type() CheckType     { return m.checkType }
func (m *mockCheck) Name() string        { return m.name }
func (m *mockCheck) Description() string { return m.description }
func (m *mockCheck) Execute(ctx context.Context) (CheckResult, error) {
	if m.err != nil {
		return CheckResult{}, m.err
	}
	return CheckResult{
		ID:          m.id,
		Type:        m.checkType,
		Name:        m.name,
		Description: m.description,
		Level:       LevelA,
		Passed:      m.passed,
		Message:     "OK",
		Timestamp:   time.Now(),
	}, nil
}

func TestNewChecker(t *testing.T) {
	checker := NewChecker()
	if checker == nil {
		t.Fatal("NewChecker should not return nil")
	}
	if len(checker.checks) != 0 {
		t.Error("new checker should have no checks registered")
	}
}

func TestRegisterCheck(t *testing.T) {
	checker := NewChecker()
	check := &mockCheck{id: "test-1", name: "Test Check"}

	checker.RegisterCheck(check)

	checks := checker.GetRegisteredChecks()
	if len(checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(checks))
	}
	if checks[0].ID() != "test-1" {
		t.Errorf("expected check ID 'test-1', got '%s'", checks[0].ID())
	}
}

func TestRunChecks(t *testing.T) {
	checker := NewChecker()

	// 注册多个检查项
	checker.RegisterCheck(&mockCheck{id: "sec-1", checkType: CheckSecurity, name: "Security Check 1", passed: true})
	checker.RegisterCheck(&mockCheck{id: "sec-2", checkType: CheckSecurity, name: "Security Check 2", passed: false})
	checker.RegisterCheck(&mockCheck{id: "audit-1", checkType: CheckAudit, name: "Audit Check", passed: true})

	report, err := checker.RunChecks(context.Background())
	if err != nil {
		t.Fatalf("RunChecks failed: %v", err)
	}

	if report == nil {
		t.Fatal("report should not be nil")
	}
	if len(report.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(report.Results))
	}
	if report.PassedCount != 2 {
		t.Errorf("expected 2 passed, got %d", report.PassedCount)
	}
	if report.FailedCount != 1 {
		t.Errorf("expected 1 failed, got %d", report.FailedCount)
	}
}

func TestRunChecksByType(t *testing.T) {
	checker := NewChecker()

	checker.RegisterCheck(&mockCheck{id: "sec-1", checkType: CheckSecurity, name: "Security 1", passed: true})
	checker.RegisterCheck(&mockCheck{id: "sec-2", checkType: CheckSecurity, name: "Security 2", passed: true})
	checker.RegisterCheck(&mockCheck{id: "audit-1", checkType: CheckAudit, name: "Audit 1", passed: true})

	report, err := checker.RunChecksByType(context.Background(), CheckSecurity)
	if err != nil {
		t.Fatalf("RunChecksByType failed: %v", err)
	}

	if len(report.Results) != 2 {
		t.Errorf("expected 2 security results, got %d", len(report.Results))
	}
}

func TestCalculateOverallLevel(t *testing.T) {
	tests := []struct {
		name     string
		results  []CheckResult
		expected Level
	}{
		{
			name:     "empty results",
			results:  []CheckResult{},
			expected: LevelD,
		},
		{
			name: "100% passed",
			results: []CheckResult{
				{Passed: true},
				{Passed: true},
				{Passed: true},
			},
			expected: LevelA,
		},
		{
			name: "75% passed",
			results: []CheckResult{
				{Passed: true},
				{Passed: true},
				{Passed: true},
				{Passed: false},
			},
			expected: LevelB,
		},
		{
			name: "50% passed",
			results: []CheckResult{
				{Passed: true},
				{Passed: false},
			},
			expected: LevelC,
		},
		{
			name: "0% passed",
			results: []CheckResult{
				{Passed: false},
				{Passed: false},
			},
			expected: LevelD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := calculateOverallLevel(tt.results)
			if level != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, level)
			}
		})
	}
}

func TestCountPassed(t *testing.T) {
	results := []CheckResult{
		{Passed: true},
		{Passed: false},
		{Passed: true},
		{Passed: true},
		{Passed: false},
	}

	count := countPassed(results)
	if count != 3 {
		t.Errorf("expected 3 passed, got %d", count)
	}
}

func TestGenerateReportID(t *testing.T) {
	id1 := generateReportID()
	id2 := generateReportID()

	if id1 == "" {
		t.Error("report ID should not be empty")
	}
	if id1 == id2 {
		t.Error("report IDs should be unique")
	}
	if len(id1) < 10 {
		t.Error("report ID should be long enough")
	}
}

func TestCheckResultDefaults(t *testing.T) {
	result := CheckResult{
		ID:        "test-1",
		Type:      CheckSecurity,
		Name:      "Test",
		Passed:    true,
		Timestamp: time.Now(),
	}

	if result.ID != "test-1" {
		t.Error("ID should match")
	}
	if result.Type != CheckSecurity {
		t.Error("Type should be security")
	}
	if !result.Passed {
		t.Error("Passed should be true")
	}
}
