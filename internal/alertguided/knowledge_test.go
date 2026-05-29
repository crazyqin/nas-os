package alertguided

import (
	"testing"
)

func TestKnowledgeBase_LoadBuiltin(t *testing.T) {
	kb := NewKnowledgeBase()
	entries := kb.List()
	if len(entries) < 10 {
		t.Errorf("expected at least 10 builtin entries, got %d", len(entries))
	}
}

func TestKnowledgeBase_Get(t *testing.T) {
	kb := NewKnowledgeBase()
	entry, ok := kb.Get("disk_space_low")
	if !ok {
		t.Fatal("expected to find disk_space_low")
	}
	if entry.Title != "磁盘空间不足" {
		t.Errorf("expected title 磁盘空间不足, got %s", entry.Title)
	}
	if entry.Category != CategoryStorage {
		t.Errorf("expected category storage, got %s", entry.Category)
	}
	if len(entry.Causes) == 0 {
		t.Error("expected causes to be non-empty")
	}
	if len(entry.Steps) == 0 {
		t.Error("expected steps to be non-empty")
	}
}

func TestKnowledgeBase_GetNotFound(t *testing.T) {
	kb := NewKnowledgeBase()
	_, ok := kb.Get("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestKnowledgeBase_Search(t *testing.T) {
	kb := NewKnowledgeBase()

	tests := []struct {
		keyword  string
		minCount int
	}{
		{"磁盘", 3},
		{"SMART", 1},
		{"CPU", 1},
		{"网络", 1},
		{"zfs", 1},
		{"证书", 1},
		{"备份", 1},
		{"UPS", 1},
	}

	for _, tt := range tests {
		results := kb.Search(tt.keyword)
		if len(results) < tt.minCount {
			t.Errorf("search '%s': expected at least %d results, got %d", tt.keyword, tt.minCount, len(results))
		}
	}
}

func TestKnowledgeBase_SearchByTag(t *testing.T) {
	kb := NewKnowledgeBase()
	results := kb.Search("raid")
	if len(results) == 0 {
		t.Error("expected to find entries with 'raid' tag")
	}
}

func TestKnowledgeBase_LookupByCategory(t *testing.T) {
	kb := NewKnowledgeBase()

	tests := []struct {
		category Category
		minCount int
	}{
		{CategoryStorage, 3},
		{CategoryHardware, 2},
		{CategoryPerformance, 2},
		{CategoryNetwork, 1},
		{CategorySecurity, 1},
		{CategorySystem, 2},
	}

	for _, tt := range tests {
		results := kb.LookupByCategory(tt.category)
		if len(results) < tt.minCount {
			t.Errorf("category %s: expected at least %d, got %d", tt.category, tt.minCount, len(results))
		}
	}
}

func TestKnowledgeBase_Add(t *testing.T) {
	kb := NewKnowledgeBase()
	entry := &KnowledgeEntry{
		ID:       "custom_alert",
		Title:    "自定义告警",
		Category: CategorySystem,
		Severity: SeverityInfo,
		Causes:   []string{"自定义原因"},
		Steps:    []RepairStep{{Order: 1, Title: "步骤1", Description: "描述"}},
	}
	kb.Add(entry)

	got, ok := kb.Get("custom_alert")
	if !ok {
		t.Fatal("expected custom entry to be found")
	}
	if got.Title != "自定义告警" {
		t.Errorf("expected title 自定义告警, got %s", got.Title)
	}
}

func TestKnowledgeBase_AllEntriesHaveSteps(t *testing.T) {
	kb := NewKnowledgeBase()
	for _, entry := range kb.List() {
		if len(entry.Steps) == 0 {
			t.Errorf("entry %s has no steps", entry.ID)
		}
		for _, step := range entry.Steps {
			if step.Title == "" {
				t.Errorf("entry %s step %d has empty title", entry.ID, step.Order)
			}
			if step.Description == "" {
				t.Errorf("entry %s step %d has empty description", entry.ID, step.Order)
			}
		}
	}
}

func TestKnowledgeBase_AllEntriesHaveCauses(t *testing.T) {
	kb := NewKnowledgeBase()
	for _, entry := range kb.List() {
		if len(entry.Causes) == 0 {
			t.Errorf("entry %s has no causes", entry.ID)
		}
	}
}

func TestFormatKnowledgeEntry(t *testing.T) {
	kb := NewKnowledgeBase()
	entry, _ := kb.Get("disk_space_low")
	output := FormatKnowledgeEntry(entry)
	if output == "" {
		t.Error("expected non-empty output")
	}
	// 基本内容检查
	if !containsStr(output, "磁盘空间不足") {
		t.Error("expected output to contain title")
	}
	if !containsStr(output, "修复步骤") {
		t.Error("expected output to contain steps header")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
