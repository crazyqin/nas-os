package templates

import (
	"testing"
)

func TestGetTemplates(t *testing.T) {
	templates := GetTemplates()
	if len(templates) == 0 {
		t.Error("GetTemplates should return built-in templates")
	}
}

func TestGetBuiltInTemplates(t *testing.T) {
	templates := GetBuiltInTemplates()
	if len(templates) == 0 {
		t.Error("GetBuiltInTemplates should return templates")
	}

	// 验证模板结构
	for _, tpl := range templates {
		if tpl.ID == "" {
			t.Error("Template ID should not be empty")
		}
		if tpl.Name == "" {
			t.Error("Template Name should not be empty")
		}
		if tpl.Category == "" {
			t.Error("Template Category should not be empty")
		}
	}
}

func TestGetTemplate(t *testing.T) {
	// 测试存在的模板
	tpl, err := GetTemplate("tpl_backup_daily")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if tpl == nil {
		t.Fatal("Template should not be nil")
	}
	if tpl.Name != "每日数据备份" {
		t.Errorf("Expected name '每日数据备份', got %s", tpl.Name)
	}

	// 测试不存在的模板
	tpl, err = GetTemplate("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}

func TestGetCategories(t *testing.T) {
	categories := GetCategories()
	if len(categories) == 0 {
		t.Error("GetCategories should return categories")
	}

	// 验证分类不为空
	for _, cat := range categories {
		if cat == "" {
			t.Error("Category should not be empty string")
		}
	}
}

func TestValidateTemplate(t *testing.T) {
	templates := GetBuiltInTemplates()
	if len(templates) == 0 {
		t.Skip("No templates to validate")
	}

	tpl := &templates[0]
	result := ValidateTemplate(tpl)

	if !result.Valid {
		t.Errorf("Built-in template should be valid, errors: %v", result.Errors)
	}
}

func TestGetTemplateParams(t *testing.T) {
	tpl, err := GetTemplate("tpl_backup_daily")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}

	params := GetTemplateParams(tpl)
	if len(params) == 0 {
		t.Error("Template should have parameters")
	}
}

func TestExportTemplate(t *testing.T) {
	tpl, err := GetTemplate("tpl_backup_daily")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}

	data, err := ExportTemplate(tpl)
	if err != nil {
		t.Fatalf("ExportTemplate failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Exported data should not be empty")
	}
}

func TestImportTemplate(t *testing.T) {
	// 先导出一个模板
	tpl, _ := GetTemplate("tpl_backup_daily")
	data, _ := ExportTemplate(tpl)

	// 再导入
	imported, err := ImportTemplate(data)
	if err != nil {
		t.Fatalf("ImportTemplate failed: %v", err)
	}

	if imported.Name != tpl.Name {
		t.Errorf("Expected name %s, got %s", tpl.Name, imported.Name)
	}
}

func TestCreateWorkflowFromTemplate(t *testing.T) {
	tpl, err := GetTemplate("tpl_backup_daily")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}

	params := map[string]string{
		"source_path":    "/data/important",
		"backup_path":    "/backup",
		"notify_channel": "email",
	}

	wf := CreateWorkflowFromTemplate(tpl, params)

	if wf == nil {
		t.Fatal("Workflow should not be nil")
	}

	if wf.Name == "" {
		t.Error("Workflow name should not be empty")
	}
}

func TestNewTemplateManager(t *testing.T) {
	// 不使用存储路径
	tm, err := NewTemplateManager("")
	if err != nil {
		t.Fatalf("NewTemplateManager failed: %v", err)
	}
	if tm == nil {
		t.Fatal("TemplateManager should not be nil")
	}
}

func TestGetTemplatesByTag(t *testing.T) {
	templates := GetTemplatesByTag("backup")
	if len(templates) == 0 {
		t.Error("Should have templates with 'backup' tag")
	}

	// 验证返回的模板都有 backup 标签
	for _, tpl := range templates {
		found := false
		for _, tag := range tpl.Tags {
			if tag == "backup" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Template %s should have 'backup' tag", tpl.ID)
		}
	}
}

func TestGetTemplatesByCategory(t *testing.T) {
	templates := GetTemplatesByCategory("备份任务")
	if len(templates) == 0 {
		t.Error("Should have templates in '备份任务' category")
	}

	// 验证返回的模板都属于该分类
	for _, tpl := range templates {
		if tpl.Category != "备份任务" {
			t.Errorf("Template %s should be in '备份任务' category", tpl.ID)
		}
	}
}
