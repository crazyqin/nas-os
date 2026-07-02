package cloudui

import (
	"testing"
)

func TestNewCloudUIManager(t *testing.T) {
	config := UIConfig{
		DefaultTheme:        ThemeDark,
		EnableAnimations:    true,
		EnableNotifications: true,
		RefreshInterval:     30,
		MaxDashboards:       10,
	}

	manager := NewCloudUIManager(config)
	if manager == nil {
		t.Fatal("NewCloudUIManager returned nil")
	}

	if manager.GetTheme() != ThemeDark {
		t.Errorf("Expected theme 'dark', got '%s'", manager.GetTheme())
	}
}

func TestCloudUIManager_CreateDashboard(t *testing.T) {
	config := UIConfig{
		DefaultTheme:  ThemeLight,
		MaxDashboards: 5,
	}

	manager := NewCloudUIManager(config)

	dashboard := &Dashboard{
		ID:      "dashboard-1",
		Name:    "System Overview",
		Theme:   ThemeDark,
		Columns: 3,
	}

	err := manager.CreateDashboard(dashboard)
	if err != nil {
		t.Fatalf("CreateDashboard failed: %v", err)
	}

	// 验证仪表板已创建
	retrieved, err := manager.GetDashboard("dashboard-1")
	if err != nil {
		t.Fatalf("GetDashboard failed: %v", err)
	}

	if retrieved.Name != "System Overview" {
		t.Errorf("Expected name 'System Overview', got '%s'", retrieved.Name)
	}
}

func TestCloudUIManager_AddWidget(t *testing.T) {
	config := UIConfig{
		MaxDashboards: 5,
	}

	manager := NewCloudUIManager(config)

	// 创建仪表板
	dashboard := &Dashboard{
		ID:   "dashboard-1",
		Name: "Test Dashboard",
	}
	manager.CreateDashboard(dashboard)

	// 添加小部件
	widget := Widget{
		ID:       "widget-1",
		Name:     "CPU Usage",
		Type:     "gauge",
		Position: Position{X: 0, Y: 0},
		Size:     Size{Width: 1, Height: 1},
		Config: map[string]interface{}{
			"min": 0,
			"max": 100,
		},
	}

	err := manager.AddWidget("dashboard-1", widget)
	if err != nil {
		t.Fatalf("AddWidget failed: %v", err)
	}

	// 验证小部件已添加
	dashboard, _ = manager.GetDashboard("dashboard-1")
	if len(dashboard.Widgets) != 1 {
		t.Errorf("Expected 1 widget, got %d", len(dashboard.Widgets))
	}
}

func TestCloudUIManager_RemoveWidget(t *testing.T) {
	config := UIConfig{
		MaxDashboards: 5,
	}

	manager := NewCloudUIManager(config)

	// 创建仪表板和小部件
	dashboard := &Dashboard{
		ID:   "dashboard-1",
		Name: "Test Dashboard",
	}
	manager.CreateDashboard(dashboard)

	widget := Widget{
		ID:   "widget-1",
		Name: "Test Widget",
		Type: "chart",
	}
	manager.AddWidget("dashboard-1", widget)

	// 移除小部件
	err := manager.RemoveWidget("dashboard-1", "widget-1")
	if err != nil {
		t.Fatalf("RemoveWidget failed: %v", err)
	}

	// 验证小部件已移除
	dashboard, _ = manager.GetDashboard("dashboard-1")
	if len(dashboard.Widgets) != 0 {
		t.Errorf("Expected 0 widgets, got %d", len(dashboard.Widgets))
	}
}

func TestCloudUIManager_RenderDashboard(t *testing.T) {
	config := UIConfig{
		MaxDashboards: 5,
	}

	manager := NewCloudUIManager(config)

	// 创建仪表板
	dashboard := &Dashboard{
		ID:      "dashboard-1",
		Name:    "Test Dashboard",
		Theme:   ThemeDark,
		Columns: 2,
	}
	manager.CreateDashboard(dashboard)

	// 添加小部件
	widget := Widget{
		ID:   "widget-1",
		Name: "Memory Usage",
		Type: "gauge",
	}
	manager.AddWidget("dashboard-1", widget)

	// 渲染仪表板
	html, err := manager.RenderDashboard("dashboard-1")
	if err != nil {
		t.Fatalf("RenderDashboard failed: %v", err)
	}

	if html == "" {
		t.Error("Expected non-empty HTML output")
	}

	t.Logf("Generated HTML length: %d", len(html))
}

func TestCloudUIManager_ExportImportDashboard(t *testing.T) {
	config := UIConfig{
		MaxDashboards: 5,
	}

	manager := NewCloudUIManager(config)

	// 创建仪表板
	dashboard := &Dashboard{
		ID:      "dashboard-1",
		Name:    "Export Test",
		Theme:   ThemeDark,
		Columns: 3,
	}
	manager.CreateDashboard(dashboard)

	// 导出
	exported, err := manager.ExportDashboard("dashboard-1")
	if err != nil {
		t.Fatalf("ExportDashboard failed: %v", err)
	}

	// 修改ID并导入
	exported["id"] = "dashboard-2"
	exported["name"] = "Imported Dashboard"

	err = manager.ImportDashboard(exported)
	if err != nil {
		t.Fatalf("ImportDashboard failed: %v", err)
	}

	// 验证导入的仪表板
	imported, err := manager.GetDashboard("dashboard-2")
	if err != nil {
		t.Fatalf("GetDashboard failed: %v", err)
	}

	if imported.Name != "Imported Dashboard" {
		t.Errorf("Expected name 'Imported Dashboard', got '%s'", imported.Name)
	}
}

func TestCloudUIManager_SetTheme(t *testing.T) {
	config := UIConfig{
		DefaultTheme: ThemeLight,
	}

	manager := NewCloudUIManager(config)

	// 设置新主题
	manager.SetTheme(ThemeDark)

	if manager.GetTheme() != ThemeDark {
		t.Errorf("Expected theme 'dark', got '%s'", manager.GetTheme())
	}
}

func TestCloudUIManager_ListDashboards(t *testing.T) {
	config := UIConfig{
		MaxDashboards: 5,
	}

	manager := NewCloudUIManager(config)

	// 创建多个仪表板
	manager.CreateDashboard(&Dashboard{ID: "dash-1", Name: "Dashboard 1"})
	manager.CreateDashboard(&Dashboard{ID: "dash-2", Name: "Dashboard 2"})

	dashboards := manager.ListDashboards()
	if len(dashboards) != 2 {
		t.Errorf("Expected 2 dashboards, got %d", len(dashboards))
	}
}
