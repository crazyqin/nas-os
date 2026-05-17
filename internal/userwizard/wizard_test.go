package userwizard

import (
	"testing"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine()
	if engine == nil {
		t.Fatal("NewEngine 返回 nil")
	}

	templates := engine.GetTemplates()
	if len(templates) != 4 {
		t.Fatalf("期望 4 个默认模板，得到 %d", len(templates))
	}
}

func TestGetTemplate(t *testing.T) {
	engine := NewEngine()

	// 测试获取存在的模板
	tmpl, err := engine.GetTemplate("tpl_admin")
	if err != nil {
		t.Fatalf("获取模板失败：%v", err)
	}
	if tmpl.Role != RoleAdmin {
		t.Errorf("期望角色 %s，得到 %s", RoleAdmin, tmpl.Role)
	}

	// 测试获取不存在的模板
	_, err = engine.GetTemplate("not_exist")
	if err == nil {
		t.Error("期望错误，但得到 nil")
	}
}

func TestGetTemplateByRole(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		role     TemplateRole
		expected string
	}{
		{RoleAdmin, "tpl_admin"},
		{RoleStandard, "tpl_standard"},
		{RoleReadOnly, "tpl_readonly"},
		{RoleGuest, "tpl_guest"},
	}

	for _, tt := range tests {
		tmpl, err := engine.GetTemplateByRole(tt.role)
		if err != nil {
			t.Errorf("角色 %s 获取模板失败：%v", tt.role, err)
			continue
		}
		if tmpl.ID != tt.expected {
			t.Errorf("角色 %s：期望模板 ID %s，得到 %s", tt.role, tt.expected, tmpl.ID)
		}
	}
}

func TestAddAndDeleteTemplate(t *testing.T) {
	engine := NewEngine()

	newTpl := &UserTemplate{
		ID:           "tpl_custom",
		Name:         "自定义模板",
		Description:  "测试自定义模板",
		Role:         RoleStandard,
		StorageQuota: 50 * 1024 * 1024 * 1024,
	}

	// 添加模板
	if err := engine.AddTemplate(newTpl); err != nil {
		t.Fatalf("添加模板失败：%v", err)
	}

	// 验证已添加
	if _, err := engine.GetTemplate("tpl_custom"); err != nil {
		t.Fatalf("获取自定义模板失败：%v", err)
	}

	// 测试重复添加
	if err := engine.AddTemplate(newTpl); err == nil {
		t.Error("期望错误，但得到 nil")
	}

	// 删除模板
	if err := engine.DeleteTemplate("tpl_custom"); err != nil {
		t.Fatalf("删除模板失败：%v", err)
	}

	// 验证已删除
	if _, err := engine.GetTemplate("tpl_custom"); err == nil {
		t.Error("期望错误，但得到 nil")
	}
}

func TestDeleteDefaultTemplate(t *testing.T) {
	engine := NewEngine()

	// 尝试删除默认模板（应该失败）
	if err := engine.DeleteTemplate("tpl_admin"); err == nil {
		t.Error("期望错误，但得到 nil")
	}
}

func TestResolveTemplate(t *testing.T) {
	engine := NewEngine()

	// 通过 ID 解析
	tmpl, err := engine.ResolveTemplate("tpl_admin", "")
	if err != nil {
		t.Fatalf("通过 ID 解析失败：%v", err)
	}
	if tmpl.Role != RoleAdmin {
		t.Errorf("期望角色 %s，得到 %s", RoleAdmin, tmpl.Role)
	}

	// 通过角色解析
	tmpl, err = engine.ResolveTemplate("", RoleGuest)
	if err != nil {
		t.Fatalf("通过角色解析失败：%v", err)
	}
	if tmpl.Role != RoleGuest {
		t.Errorf("期望角色 %s，得到 %s", RoleGuest, tmpl.Role)
	}

	// 默认解析（标准用户）
	tmpl, err = engine.ResolveTemplate("", "")
	if err != nil {
		t.Fatalf("默认解析失败：%v", err)
	}
	if tmpl.Role != RoleStandard {
		t.Errorf("期望角色 %s，得到 %s", RoleStandard, tmpl.Role)
	}
}

func TestMapTemplateRoleToUserRole(t *testing.T) {
	tests := []struct {
		input    TemplateRole
		expected string
	}{
		{RoleAdmin, "admin"},
		{RoleStandard, "user"},
		{RoleReadOnly, "user"},
		{RoleGuest, "guest"},
		{"unknown", "user"},
	}

	for _, tt := range tests {
		result := MapTemplateRoleToUserRole(tt.input)
		if result != tt.expected {
			t.Errorf("MapTemplateRoleToUserRole(%s)：期望 %s，得到 %s", tt.input, tt.expected, result)
		}
	}
}

func TestUpdateTemplate(t *testing.T) {
	engine := NewEngine()

	// 更新存在的模板
	updated := &UserTemplate{
		ID:           "tpl_admin",
		Name:         "管理员（已更新）",
		Description:  "更新后的描述",
		Role:         RoleAdmin,
		StorageQuota: 0,
	}

	if err := engine.UpdateTemplate("tpl_admin", updated); err != nil {
		t.Fatalf("更新模板失败：%v", err)
	}

	tmpl, _ := engine.GetTemplate("tpl_admin")
	if tmpl.Name != "管理员（已更新）" {
		t.Errorf("期望名称 '管理员（已更新）'，得到 '%s'", tmpl.Name)
	}

	// 更新不存在的模板
	if err := engine.UpdateTemplate("not_exist", updated); err == nil {
		t.Error("期望错误，但得到 nil")
	}
}
