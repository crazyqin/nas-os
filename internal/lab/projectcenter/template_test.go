package projectcenter

import (
	"testing"
)

// ========== TemplateManager 测试 ==========

func TestNewTemplateManager(t *testing.T) {
	mgr := NewTemplateManager()

	// 应该有内置模板
	templates := mgr.ListTemplates("")
	if len(templates) < 4 {
		t.Errorf("expected at least 4 builtin templates, got %d", len(templates))
	}
}

func TestCreateTemplate(t *testing.T) {
	mgr := NewTemplateManager()

	req := ProjectTemplate{
		Name:        "自定义模板",
		Description: "自定义描述",
		Category:    "custom",
		Columns: []TemplateColumn{
			{Name: "待办", Status: TaskStatusTodo, Order: 1},
		},
		Tasks: []TemplateTask{
			{Title: "任务1", Priority: PriorityHigh, Phase: "阶段1", Order: 1},
		},
	}

	tmpl, err := mgr.CreateTemplate(req)
	if err != nil {
		t.Fatalf("CreateTemplate failed: %v", err)
	}

	if tmpl.Name != "自定义模板" {
		t.Errorf("expected name '自定义模板', got '%s'", tmpl.Name)
	}
	if tmpl.ID == "" {
		t.Error("expected ID to be set")
	}
	if tmpl.UsageCount != 0 {
		t.Errorf("expected usage count 0, got %d", tmpl.UsageCount)
	}
}

func TestCreateTemplateEmptyName(t *testing.T) {
	mgr := NewTemplateManager()

	req := ProjectTemplate{
		Name: "",
	}

	_, err := mgr.CreateTemplate(req)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGetTemplate(t *testing.T) {
	mgr := NewTemplateManager()

	templates := mgr.ListTemplates("")
	created := templates[0]

	fetched, err := mgr.GetTemplate(created.ID)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, fetched.ID)
	}
}

func TestGetTemplateNotFound(t *testing.T) {
	mgr := NewTemplateManager()

	_, err := mgr.GetTemplate("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent template")
	}
}

func TestUpdateTemplate(t *testing.T) {
	mgr := NewTemplateManager()

	// 创建自定义模板
	tmpl, _ := mgr.CreateTemplate(ProjectTemplate{
		Name:     "原始名称",
		Category: "test",
	})

	updated, err := mgr.UpdateTemplate(tmpl.ID, ProjectTemplate{
		Name:        "新名称",
		Description: "新描述",
	})
	if err != nil {
		t.Fatalf("UpdateTemplate failed: %v", err)
	}

	if updated.Name != "新名称" {
		t.Errorf("expected name '新名称', got '%s'", updated.Name)
	}
	if updated.Description != "新描述" {
		t.Errorf("expected description '新描述', got '%s'", updated.Description)
	}
}

func TestDeleteTemplate(t *testing.T) {
	mgr := NewTemplateManager()

	// 创建非默认模板
	tmpl, _ := mgr.CreateTemplate(ProjectTemplate{
		Name:     "要删除的模板",
		Category: "test",
	})

	err := mgr.DeleteTemplate(tmpl.ID)
	if err != nil {
		t.Fatalf("DeleteTemplate failed: %v", err)
	}

	_, err = mgr.GetTemplate(tmpl.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestDeleteDefaultTemplate(t *testing.T) {
	mgr := NewTemplateManager()

	templates := mgr.ListTemplates("")
	for _, tmpl := range templates {
		if tmpl.IsDefault {
			err := mgr.DeleteTemplate(tmpl.ID)
			if err == nil {
				t.Fatal("expected error when deleting default template")
			}
			break
		}
	}
}

func TestListTemplatesByCategory(t *testing.T) {
	mgr := NewTemplateManager()

	templates := mgr.ListTemplates("software")
	if len(templates) == 0 {
		t.Fatal("expected at least 1 software template")
	}

	for _, tmpl := range templates {
		if tmpl.Category != "software" {
			t.Errorf("expected category 'software', got '%s'", tmpl.Category)
		}
	}
}

func TestGetTemplateCategories(t *testing.T) {
	mgr := NewTemplateManager()

	categories := mgr.GetTemplateCategories()
	if len(categories) < 3 {
		t.Errorf("expected at least 3 categories, got %d", len(categories))
	}

	found := make(map[string]bool)
	for _, cat := range categories {
		found[cat] = true
	}

	expectedCats := []string{"software", "marketing", "research", "general"}
	for _, cat := range expectedCats {
		if !found[cat] {
			t.Errorf("expected category '%s' not found", cat)
		}
	}
}

func TestIncrementUsage(t *testing.T) {
	mgr := NewTemplateManager()

	templates := mgr.ListTemplates("")
	tmpl := templates[0]

	if tmpl.UsageCount != 0 {
		t.Errorf("expected initial usage count 0, got %d", tmpl.UsageCount)
	}

	mgr.IncrementUsage(tmpl.ID)
	mgr.IncrementUsage(tmpl.ID)

	fetched, _ := mgr.GetTemplate(tmpl.ID)
	if fetched.UsageCount != 2 {
		t.Errorf("expected usage count 2, got %d", fetched.UsageCount)
	}
}

func TestCloneTemplate(t *testing.T) {
	mgr := NewTemplateManager()

	templates := mgr.ListTemplates("software")
	original := templates[0]

	cloned, err := mgr.CloneTemplate(original.ID, "克隆的模板")
	if err != nil {
		t.Fatalf("CloneTemplate failed: %v", err)
	}

	if cloned.ID == original.ID {
		t.Error("cloned template should have different ID")
	}
	if cloned.Name != "克隆的模板" {
		t.Errorf("expected name '克隆的模板', got '%s'", cloned.Name)
	}
	if cloned.IsDefault {
		t.Error("cloned template should not be default")
	}
	if cloned.UsageCount != 0 {
		t.Errorf("expected usage count 0, got %d", cloned.UsageCount)
	}
	if len(cloned.Columns) != len(original.Columns) {
		t.Errorf("expected %d columns, got %d", len(original.Columns), len(cloned.Columns))
	}
	if len(cloned.Tasks) != len(original.Tasks) {
		t.Errorf("expected %d tasks, got %d", len(original.Tasks), len(cloned.Tasks))
	}
}

func TestGetDefaultTemplate(t *testing.T) {
	mgr := NewTemplateManager()

	tmpl, err := mgr.GetDefaultTemplate("software")
	if err != nil {
		t.Fatalf("GetDefaultTemplate failed: %v", err)
	}

	if !tmpl.IsDefault {
		t.Error("expected template to be default")
	}
	if tmpl.Category != "software" {
		t.Errorf("expected category 'software', got '%s'", tmpl.Category)
	}
}

func TestSearchTemplates(t *testing.T) {
	mgr := NewTemplateManager()

	// 搜索 "软件"
	results := mgr.SearchTemplates("软件")
	if len(results) == 0 {
		t.Error("expected search results for '软件'")
	}

	// 搜索 "marketing"
	results = mgr.SearchTemplates("marketing")
	if len(results) == 0 {
		t.Error("expected search results for 'marketing'")
	}
}

func TestBuiltinSoftwareTemplate(t *testing.T) {
	mgr := NewTemplateManager()

	tmpl, _ := mgr.GetDefaultTemplate("software")

	if len(tmpl.Columns) < 4 {
		t.Errorf("expected at least 4 columns, got %d", len(tmpl.Columns))
	}
	if len(tmpl.Tasks) < 8 {
		t.Errorf("expected at least 8 tasks, got %d", len(tmpl.Tasks))
	}
	if !tmpl.IsDefault {
		t.Error("software template should be default")
	}
}

func TestBuiltinMarketingTemplate(t *testing.T) {
	mgr := NewTemplateManager()

	templates := mgr.ListTemplates("marketing")
	if len(templates) == 0 {
		t.Fatal("expected marketing template")
	}

	tmpl := templates[0]
	if len(tmpl.Tasks) < 5 {
		t.Errorf("expected at least 5 tasks, got %d", len(tmpl.Tasks))
	}
}

func TestBuiltinResearchTemplate(t *testing.T) {
	mgr := NewTemplateManager()

	templates := mgr.ListTemplates("research")
	if len(templates) == 0 {
		t.Fatal("expected research template")
	}

	tmpl := templates[0]
	if len(tmpl.Tasks) < 5 {
		t.Errorf("expected at least 5 tasks, got %d", len(tmpl.Tasks))
	}
}
