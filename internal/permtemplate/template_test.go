package permtemplate

import (
	"testing"
)

func TestBuiltinTemplates(t *testing.T) {
	m := NewTemplateManager()
	templates := m.List("")
	if len(templates) < 5 {
		t.Errorf("expected at least 5 builtin templates, got %d", len(templates))
	}
	homeUser, ok := m.Get("home_user")
	if !ok {
		t.Fatal("expected home_user template")
	}
	if homeUser.Name != "家庭用户" {
		t.Errorf("expected name '家庭用户', got %s", homeUser.Name)
	}
	if homeUser.Quotas == nil {
		t.Error("expected quotas for home_user")
	}
}

func TestCreateAndUpdate(t *testing.T) {
	m := NewTemplateManager()
	err := m.Create(&PermissionTemplate{
		ID:          "custom1",
		Name:        "自定义模板",
		Description: "测试模板",
		Category:    "custom",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Duplicate
	err = m.Create(&PermissionTemplate{ID: "custom1", Name: "dup"})
	if err == nil {
		t.Error("expected error for duplicate template")
	}
	// Update
	tmpl, _ := m.Get("custom1")
	tmpl.Name = "更新后的模板"
	err = m.Update(tmpl)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	updated, _ := m.Get("custom1")
	if updated.Name != "更新后的模板" {
		t.Errorf("expected updated name, got %s", updated.Name)
	}
	// Cannot modify builtin
	homeUser, _ := m.Get("home_user")
	homeUser.Name = "hacked"
	err = m.Update(homeUser)
	if err == nil {
		t.Error("expected error when modifying builtin template")
	}
}

func TestApply(t *testing.T) {
	m := NewTemplateManager()
	err := m.Apply("home_user", "user1", "admin")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	apps := m.GetApplications("user1")
	if len(apps) != 1 {
		t.Errorf("expected 1 application, got %d", len(apps))
	}
	if apps[0].TemplateID != "home_user" {
		t.Errorf("expected template home_user, got %s", apps[0].TemplateID)
	}
}

func TestDelete(t *testing.T) {
	m := NewTemplateManager()
	m.Create(&PermissionTemplate{ID: "tmp1", Name: "temp"})
	err := m.Delete("tmp1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, ok := m.Get("tmp1")
	if ok {
		t.Error("expected template to be deleted")
	}
	// Cannot delete builtin
	err = m.Delete("home_user")
	if err == nil {
		t.Error("expected error when deleting builtin template")
	}
}
