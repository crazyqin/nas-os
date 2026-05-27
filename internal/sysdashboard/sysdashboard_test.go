package sysdashboard

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestCreateDashboard(t *testing.T) {
	m := NewManager()
	d, err := m.CreateDashboard("主仪表盘", 3)
	if err != nil {
		t.Fatalf("CreateDashboard failed: %v", err)
	}
	if d.Name != "主仪表盘" {
		t.Errorf("expected '主仪表盘', got '%s'", d.Name)
	}
	if d.Columns != 3 {
		t.Errorf("expected 3 columns, got %d", d.Columns)
	}
}

func TestGetDashboard(t *testing.T) {
	m := NewManager()
	m.CreateDashboard("test", 2)
	dashboards := m.GetDashboards()
	if len(dashboards) != 1 {
		t.Errorf("expected 1 dashboard, got %d", len(dashboards))
	}
}

func TestUpdateDashboard(t *testing.T) {
	m := NewManager()
	d, _ := m.CreateDashboard("old-name", 2)
	err := m.UpdateDashboard(d.ID, "new-name", 4)
	if err != nil {
		t.Fatalf("UpdateDashboard failed: %v", err)
	}
	updated, _ := m.GetDashboard(d.ID)
	if updated.Name != "new-name" {
		t.Errorf("expected 'new-name', got '%s'", updated.Name)
	}
}

func TestDeleteDashboard(t *testing.T) {
	m := NewManager()
	d, _ := m.CreateDashboard("to-delete", 2)
	err := m.DeleteDashboard(d.ID)
	if err != nil {
		t.Fatalf("DeleteDashboard failed: %v", err)
	}
	_, err = m.GetDashboard(d.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestAddWidget(t *testing.T) {
	m := NewManager()
	d, _ := m.CreateDashboard("test", 3)
	widget := &Widget{
		Type:   WidgetCPUGraph,
		Title:  "CPU使用率",
		X:      0,
		Y:      0,
		Width:  2,
		Height: 1,
	}
	err := m.AddWidget(d.ID, widget)
	if err != nil {
		t.Fatalf("AddWidget failed: %v", err)
	}
	dashboard, _ := m.GetDashboard(d.ID)
	if len(dashboard.Widgets) != 1 {
		t.Errorf("expected 1 widget, got %d", len(dashboard.Widgets))
	}
	if dashboard.Widgets[0].ID == "" {
		t.Error("widget ID should be set")
	}
}

func TestUpdateWidget(t *testing.T) {
	m := NewManager()
	d, _ := m.CreateDashboard("test", 3)
	widget := &Widget{Type: WidgetCPUGraph, Title: "CPU"}
	m.AddWidget(d.ID, widget)
	
	err := m.UpdateWidget(d.ID, widget.ID, 1, 1, 3, 2)
	if err != nil {
		t.Fatalf("UpdateWidget failed: %v", err)
	}
	dashboard, _ := m.GetDashboard(d.ID)
	w := dashboard.Widgets[0]
	if w.X != 1 || w.Y != 1 || w.Width != 3 || w.Height != 2 {
		t.Errorf("widget position not updated correctly")
	}
}

func TestRemoveWidget(t *testing.T) {
	m := NewManager()
	d, _ := m.CreateDashboard("test", 3)
	widget := &Widget{Type: WidgetCPUGraph, Title: "CPU"}
	m.AddWidget(d.ID, widget)
	
	err := m.RemoveWidget(d.ID, widget.ID)
	if err != nil {
		t.Fatalf("RemoveWidget failed: %v", err)
	}
	dashboard, _ := m.GetDashboard(d.ID)
	if len(dashboard.Widgets) != 0 {
		t.Errorf("expected 0 widgets, got %d", len(dashboard.Widgets))
	}
}

func TestGetAlerts(t *testing.T) {
	m := NewManager()
	alerts := m.GetAlerts(false)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestGetSystemOverview(t *testing.T) {
	m := NewManager()
	overview := m.GetSystemOverview()
	if overview == nil {
		t.Fatal("GetSystemOverview returned nil")
	}
}

func TestGetServices(t *testing.T) {
	m := NewManager()
	services := m.GetServices()
	if services == nil {
		t.Fatal("GetServices returned nil")
	}
}
