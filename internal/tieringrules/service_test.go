package tieringrules

import (
	"testing"
	"time"
)

func TestCreateRule(t *testing.T) {
	e := NewEngine()
	req := &CreateRuleRequest{
		Name:       "cold-data",
		Condition:  ConditionAccessFreq,
		Threshold:  5,
		SourcePool: "ssd-pool",
		TargetPool: "hdd-pool",
		Action:     ActionMove,
		Enabled:    true,
	}
	rule, err := e.CreateRule(req)
	if err != nil {
		t.Fatalf("CreateRule failed: %v", err)
	}
	if rule.ID == "" {
		t.Error("expected non-empty ID")
	}
	if rule.Name != "cold-data" {
		t.Errorf("expected name 'cold-data', got %q", rule.Name)
	}
	if rule.Enabled != true {
		t.Error("expected enabled=true")
	}
}

func TestCreateRuleValidation(t *testing.T) {
	e := NewEngine()
	// Empty name
	if _, err := e.CreateRule(&CreateRuleRequest{SourcePool: "a", TargetPool: "b"}); err == nil {
		t.Error("expected error for empty name")
	}
	// Same pools
	if _, err := e.CreateRule(&CreateRuleRequest{Name: "test", SourcePool: "a", TargetPool: "a"}); err == nil {
		t.Error("expected error for same pools")
	}
	// Empty pools
	if _, err := e.CreateRule(&CreateRuleRequest{Name: "test", SourcePool: "", TargetPool: "b"}); err == nil {
		t.Error("expected error for empty source pool")
	}
}

func TestGetRule(t *testing.T) {
	e := NewEngine()
	rule, _ := e.CreateRule(&CreateRuleRequest{
		Name: "test", Condition: ConditionModifyTime, Threshold: 30,
		SourcePool: "ssd", TargetPool: "hdd", Action: ActionMove, Enabled: true,
	})
	got, err := e.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("GetRule failed: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("expected name 'test', got %q", got.Name)
	}
}

func TestGetRuleNotFound(t *testing.T) {
	e := NewEngine()
	if _, err := e.GetRule("nonexistent"); err == nil {
		t.Error("expected error for non-existent rule")
	}
}

func TestListRules(t *testing.T) {
	e := NewEngine()
	e.CreateRule(&CreateRuleRequest{Name: "r1", SourcePool: "a", TargetPool: "b"})
	e.CreateRule(&CreateRuleRequest{Name: "r2", SourcePool: "c", TargetPool: "d"})
	rules := e.ListRules()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestUpdateRule(t *testing.T) {
	e := NewEngine()
	rule, _ := e.CreateRule(&CreateRuleRequest{
		Name: "test", Condition: ConditionAccessFreq, Threshold: 10,
		SourcePool: "ssd", TargetPool: "hdd", Action: ActionMove, Enabled: true,
	})
	updated, err := e.UpdateRule(rule.ID, &CreateRuleRequest{
		Name: "updated", Threshold: 20, Enabled: false,
	})
	if err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}
	if updated.Name != "updated" {
		t.Errorf("expected name 'updated', got %q", updated.Name)
	}
	if updated.Threshold != 20 {
		t.Errorf("expected threshold 20, got %d", updated.Threshold)
	}
	if updated.Enabled != false {
		t.Error("expected enabled=false")
	}
}

func TestDeleteRule(t *testing.T) {
	e := NewEngine()
	rule, _ := e.CreateRule(&CreateRuleRequest{Name: "test", SourcePool: "a", TargetPool: "b"})
	if err := e.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}
	if _, err := e.GetRule(rule.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestEvaluateFile(t *testing.T) {
	e := NewEngine()
	// Rule: access_freq < 5 → match cold files
	e.CreateRule(&CreateRuleRequest{
		Name: "cold", Condition: ConditionAccessFreq, Threshold: 5,
		SourcePool: "ssd", TargetPool: "hdd", Action: ActionMove, Enabled: true,
	})
	// Rule: modify_time > 90 days
	e.CreateRule(&CreateRuleRequest{
		Name: "old", Condition: ConditionModifyTime, Threshold: 90,
		SourcePool: "ssd", TargetPool: "archive", Action: ActionArchive, Enabled: true,
	})
	// Disabled rule — should not match
	e.CreateRule(&CreateRuleRequest{
		Name: "disabled", Condition: ConditionSize, Threshold: 1024,
		SourcePool: "ssd", TargetPool: "hdd", Action: ActionMove, Enabled: false,
	})

	// File accessed 2 times, modified 100 days ago
	file := &FileItem{
		Path:       "/data/file.txt",
		Size:       500,
		AccessFreq: 2,
		ModifyTime: time.Now().Add(-100 * 24 * time.Hour),
	}
	matched := e.EvaluateFile(file)
	if len(matched) != 2 {
		t.Fatalf("expected 2 matched rules, got %d", len(matched))
	}
}

func TestEvaluateFileNoMatch(t *testing.T) {
	e := NewEngine()
	e.CreateRule(&CreateRuleRequest{
		Name: "cold", Condition: ConditionAccessFreq, Threshold: 5,
		SourcePool: "ssd", TargetPool: "hdd", Action: ActionMove, Enabled: true,
	})
	file := &FileItem{
		Path:       "/data/hot.txt",
		AccessFreq: 100, // High frequency, not cold
		ModifyTime: time.Now(),
	}
	matched := e.EvaluateFile(file)
	if len(matched) != 0 {
		t.Fatalf("expected 0 matched rules, got %d", len(matched))
	}
}

func TestExecuteMigration(t *testing.T) {
	e := NewEngine()
	rule, _ := e.CreateRule(&CreateRuleRequest{
		Name: "migrate", Condition: ConditionAccessFreq, Threshold: 5,
		SourcePool: "ssd", TargetPool: "hdd", Action: ActionMove, Enabled: true,
	})
	file := &FileItem{Path: "/data/file.txt", Size: 1024, AccessFreq: 2, ModifyTime: time.Now()}
	rec, err := e.ExecuteMigration(rule, file)
	if err != nil {
		t.Fatalf("ExecuteMigration failed: %v", err)
	}
	if rec.Status != MigrationStatusSuccess {
		t.Errorf("expected success status, got %q", rec.Status)
	}
	if rec.FilePath != "/data/file.txt" {
		t.Errorf("expected path '/data/file.txt', got %q", rec.FilePath)
	}

	history := e.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(history))
	}
}

func TestGetHistory(t *testing.T) {
	e := NewEngine()
	history := e.GetHistory()
	if len(history) != 0 {
		t.Fatalf("expected 0 history records, got %d", len(history))
	}
}