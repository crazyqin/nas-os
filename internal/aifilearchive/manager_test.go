package aifilearchive

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.rules == nil {
		t.Error("rules map not initialized")
	}
	if m.jobs == nil {
		t.Error("jobs map not initialized")
	}
	if m.stats == nil {
		t.Error("stats not initialized")
	}
}

func TestAddRule(t *testing.T) {
	m := NewManager()

	rule := &ArchiveRule{
		ID:          "test-rule-1",
		Name:        "Archive old files",
		Description: "Archive files older than 30 days",
		Conditions: []Condition{
			{
				Type:     ConditionAge,
				Operator: ">",
				Value:    30,
			},
		},
		Action: ArchiveAction{
			Type:     ActionArchive,
			Compress: true,
			Encrypt:  false,
		},
		Priority: 1,
		Enabled:  true,
	}

	err := m.AddRule(rule)
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	if _, exists := m.rules["test-rule-1"]; !exists {
		t.Error("rule not found in map")
	}
}

func TestAddRuleEmptyID(t *testing.T) {
	m := NewManager()

	rule := &ArchiveRule{
		Name: "Test Rule",
	}

	err := m.AddRule(rule)
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestUpdateRule(t *testing.T) {
	m := NewManager()

	rule := &ArchiveRule{
		ID:   "test-rule-1",
		Name: "Original Name",
	}
	m.AddRule(rule)

	rule.Name = "Updated Name"
	err := m.UpdateRule(rule)
	if err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}

	updated, _ := m.GetRule("test-rule-1")
	if updated.Name != "Updated Name" {
		t.Errorf("expected 'Updated Name', got '%s'", updated.Name)
	}
}

func TestUpdateRuleNotFound(t *testing.T) {
	m := NewManager()

	rule := &ArchiveRule{
		ID:   "nonexistent",
		Name: "Test",
	}

	err := m.UpdateRule(rule)
	if err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestDeleteRule(t *testing.T) {
	m := NewManager()

	rule := &ArchiveRule{
		ID:   "test-rule-1",
		Name: "Test Rule",
	}
	m.AddRule(rule)

	err := m.DeleteRule("test-rule-1")
	if err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}

	if _, exists := m.rules["test-rule-1"]; exists {
		t.Error("rule still exists after deletion")
	}
}

func TestDeleteRuleNotFound(t *testing.T) {
	m := NewManager()

	err := m.DeleteRule("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestGetRule(t *testing.T) {
	m := NewManager()

	rule := &ArchiveRule{
		ID:   "test-rule-1",
		Name: "Test Rule",
	}
	m.AddRule(rule)

	got, err := m.GetRule("test-rule-1")
	if err != nil {
		t.Fatalf("GetRule failed: %v", err)
	}
	if got.ID != "test-rule-1" {
		t.Errorf("expected ID 'test-rule-1', got '%s'", got.ID)
	}
}

func TestGetRuleNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetRule("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestListRules(t *testing.T) {
	m := NewManager()

	m.AddRule(&ArchiveRule{ID: "rule-1", Name: "Rule 1"})
	m.AddRule(&ArchiveRule{ID: "rule-2", Name: "Rule 2"})
	m.AddRule(&ArchiveRule{ID: "rule-3", Name: "Rule 3"})

	rules := m.ListRules()
	if len(rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(rules))
	}
}

func TestRunArchive(t *testing.T) {
	m := NewManager()

	rule := &ArchiveRule{
		ID:      "test-rule-1",
		Name:    "Test Rule",
		Enabled: true,
		Action: ArchiveAction{
			Type: ActionArchive,
		},
	}
	m.AddRule(rule)

	ctx := context.Background()
	job, err := m.RunArchive(ctx, "test-rule-1")
	if err != nil {
		t.Fatalf("RunArchive failed: %v", err)
	}

	if job == nil {
		t.Fatal("job is nil")
	}

	if job.RuleID != "test-rule-1" {
		t.Errorf("expected rule ID 'test-rule-1', got '%s'", job.RuleID)
	}

	// Wait for job to complete
	time.Sleep(100 * time.Millisecond)

	got, _ := m.GetJob(job.ID)
	if got.Status != JobCompleted {
		t.Errorf("expected status '%s', got '%s'", JobCompleted, got.Status)
	}
}

func TestRunArchiveDisabledRule(t *testing.T) {
	m := NewManager()

	rule := &ArchiveRule{
		ID:      "test-rule-1",
		Name:    "Test Rule",
		Enabled: false,
	}
	m.AddRule(rule)

	ctx := context.Background()
	_, err := m.RunArchive(ctx, "test-rule-1")
	if err == nil {
		t.Error("expected error for disabled rule")
	}
}

func TestRunArchiveNonexistentRule(t *testing.T) {
	m := NewManager()

	ctx := context.Background()
	_, err := m.RunArchive(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestGetJob(t *testing.T) {
	m := NewManager()

	rule := &ArchiveRule{
		ID:      "test-rule-1",
		Enabled: true,
		Action:  ArchiveAction{Type: ActionArchive},
	}
	m.AddRule(rule)

	ctx := context.Background()
	job, _ := m.RunArchive(ctx, "test-rule-1")

	got, err := m.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if got.ID != job.ID {
		t.Errorf("expected job ID '%s', got '%s'", job.ID, got.ID)
	}
}

func TestGetJobNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetJob("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestListJobs(t *testing.T) {
	m := NewManager()

	rule := &ArchiveRule{
		ID:      "test-rule-1",
		Enabled: true,
		Action:  ArchiveAction{Type: ActionArchive},
	}
	m.AddRule(rule)

	ctx := context.Background()
	m.RunArchive(ctx, "test-rule-1")
	m.RunArchive(ctx, "test-rule-1")

	time.Sleep(100 * time.Millisecond)

	jobs := m.ListJobs()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()

	stats := m.GetStats()
	if stats == nil {
		t.Fatal("stats is nil")
	}

	if stats.TotalArchived != 0 {
		t.Errorf("expected 0 archived, got %d", stats.TotalArchived)
	}
}

func TestClassifyFile(t *testing.T) {
	m := NewManager()

	// Register a mock classifier
	mockClassifier := &mockClassifier{
		result: &AIClassification{
			Category:   "document",
			Confidence: 0.95,
			Tags:       []string{"pdf", "report"},
		},
	}
	m.RegisterClassifier(mockClassifier)

	ctx := context.Background()
	results, err := m.ClassifyFile(ctx, "/path/to/file.pdf")
	if err != nil {
		t.Fatalf("ClassifyFile failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Category != "document" {
		t.Errorf("expected category 'document', got '%s'", results[0].Category)
	}
}

func TestClassifyFileNoClassifiers(t *testing.T) {
	m := NewManager()

	ctx := context.Background()
	results, err := m.ClassifyFile(ctx, "/path/to/file.pdf")
	if err != nil {
		t.Fatalf("ClassifyFile failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

type mockClassifier struct {
	result *AIClassification
	err    error
}

func (c *mockClassifier) Classify(ctx context.Context, filePath string) (*AIClassification, error) {
	return c.result, c.err
}

func (c *mockClassifier) Name() string {
	return "mock-classifier"
}
