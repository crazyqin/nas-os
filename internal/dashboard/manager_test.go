package dashboard

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

// ========== 测试辅助函数 ==========

func TestMain(m *testing.M) {
	// 运行测试
	os.Exit(m.Run())
}

func newTestManager(t *testing.T) *Manager {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dashboards.json")
	return NewManager(storePath, zap.NewNop())
}

// ========== CRUD 测试 ==========

func TestManager_Create(t *testing.T) {
	m := newTestManager(t)

	input := DashboardInput{
		Name:        "测试仪表板",
		Description: "这是一个测试仪表板",
		Layout:      DefaultLayoutConfig(),
		Widgets: []Widget{
			{
				Type:     WidgetTypeSystemStatus,
				Title:    "系统状态",
				Position: GetPresetSize(SizeMedium),
			},
		},
		IsPublic:    true,
		RefreshRate: 30,
	}

	dashboard, err := m.Create(input, "test-user")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if dashboard.ID == "" {
		t.Error("Dashboard ID should not be empty")
	}
	if dashboard.Name != input.Name {
		t.Errorf("Name mismatch: got %s, want %s", dashboard.Name, input.Name)
	}
	if dashboard.Owner != "test-user" {
		t.Errorf("Owner mismatch: got %s, want test-user", dashboard.Owner)
	}
	if len(dashboard.Widgets) != 1 {
		t.Errorf("Widgets count mismatch: got %d, want 1", len(dashboard.Widgets))
	}
	if dashboard.Widgets[0].ID == "" {
		t.Error("Widget ID should be generated")
	}
}

func TestManager_Get(t *testing.T) {
	m := newTestManager(t)

	// 创建仪表板
	input := DashboardInput{
		Name:   "测试仪表板",
		Layout: DefaultLayoutConfig(),
	}
	created, err := m.Create(input, "test-user")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 获取仪表板
	dashboard, err := m.Get(created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if dashboard.ID != created.ID {
		t.Errorf("ID mismatch: got %s, want %s", dashboard.ID, created.ID)
	}

	// 获取不存在的仪表板
	_, err = m.Get("nonexistent")
	if err != ErrDashboardNotFound {
		t.Errorf("Expected ErrDashboardNotFound, got %v", err)
	}
}

func TestManager_List(t *testing.T) {
	m := newTestManager(t)

	// 创建多个仪表板
	names := []string{"仪表板A", "仪表板B", "仪表板C"}
	for i, name := range names {
		input := DashboardInput{
			Name:     name,
			IsPublic: i%2 == 0,
		}
		_, err := m.Create(input, "test-user")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// 列出所有仪表板（包含私有）
	list := m.List(ListFilter{IncludePrivate: true})
	if len(list) != 3 {
		t.Errorf("List count mismatch: got %d, want 3", len(list))
	}

	// 按所有者过滤
	list = m.List(ListFilter{Owner: "test-user", IncludePrivate: true})
	if len(list) != 3 {
		t.Errorf("List count mismatch: got %d, want 3", len(list))
	}

	// 过滤不存在的所有者
	list = m.List(ListFilter{Owner: "other-user", IncludePrivate: true})
	if len(list) != 0 {
		t.Errorf("List count mismatch: got %d, want 0", len(list))
	}
}

func TestManager_Update(t *testing.T) {
	m := newTestManager(t)

	// 创建仪表板
	input := DashboardInput{
		Name:   "原名称",
		Layout: DefaultLayoutConfig(),
	}
	created, err := m.Create(input, "test-user")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 更新仪表板
	updateInput := DashboardInput{
		Name:        "新名称",
		Description: "新描述",
		Layout:      DefaultLayoutConfig(),
		Widgets: []Widget{
			{
				Type:     WidgetTypeCPUUsage,
				Title:    "CPU",
				Position: GetPresetSize(SizeSmall),
			},
		},
	}

	updated, err := m.Update(created.ID, updateInput)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if updated.Name != "新名称" {
		t.Errorf("Name mismatch: got %s, want 新名称", updated.Name)
	}
	if updated.Description != "新描述" {
		t.Errorf("Description mismatch: got %s, want 新描述", updated.Description)
	}
	if len(updated.Widgets) != 1 {
		t.Errorf("Widgets count mismatch: got %d, want 1", len(updated.Widgets))
	}

	// 更新不存在的仪表板
	_, err = m.Update("nonexistent", updateInput)
	if err != ErrDashboardNotFound {
		t.Errorf("Expected ErrDashboardNotFound, got %v", err)
	}
}

func TestManager_Delete(t *testing.T) {
	m := newTestManager(t)

	// 创建仪表板
	input := DashboardInput{
		Name: "测试仪表板",
	}
	created, err := m.Create(input, "test-user")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 删除仪表板
	err = m.Delete(created.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 验证已删除
	_, err = m.Get(created.ID)
	if err != ErrDashboardNotFound {
		t.Errorf("Expected ErrDashboardNotFound after delete, got %v", err)
	}

	// 删除不存在的仪表板
	err = m.Delete("nonexistent")
	if err != ErrDashboardNotFound {
		t.Errorf("Expected ErrDashboardNotFound, got %v", err)
	}
}

// ========== 小组件测试 ==========

func TestManager_AddWidget(t *testing.T) {
	m := newTestManager(t)

	// 创建仪表板
	input := DashboardInput{
		Name:   "测试仪表板",
		Layout: DefaultLayoutConfig(),
	}
	dashboard, err := m.Create(input, "test-user")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 添加小组件
	widget := Widget{
		Type:     WidgetTypeSystemStatus,
		Title:    "系统状态",
		Position: GetPresetSize(SizeMedium),
	}

	added, err := m.AddWidget(dashboard.ID, widget)
	if err != nil {
		t.Fatalf("AddWidget failed: %v", err)
	}

	if added.ID == "" {
		t.Error("Widget ID should be generated")
	}

	// 验证仪表板已更新
	updated, _ := m.Get(dashboard.ID)
	if len(updated.Widgets) != 1 {
		t.Errorf("Widgets count mismatch: got %d, want 1", len(updated.Widgets))
	}
}

func TestManager_UpdateWidget(t *testing.T) {
	m := newTestManager(t)

	// 创建带小组件的仪表板
	input := DashboardInput{
		Name:   "测试仪表板",
		Layout: DefaultLayoutConfig(),
		Widgets: []Widget{
			{
				Type:     WidgetTypeSystemStatus,
				Title:    "系统状态",
				Position: GetPresetSize(SizeMedium),
			},
		},
	}
	dashboard, err := m.Create(input, "test-user")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	widgetID := dashboard.Widgets[0].ID

	// 更新小组件
	updates := Widget{
		Type:     WidgetTypeSystemStatus,
		Title:    "更新后的标题",
		Position: GetPresetSize(SizeLarge),
		Config:   map[string]interface{}{"show_uptime": true},
	}

	updated, err := m.UpdateWidget(dashboard.ID, widgetID, updates)
	if err != nil {
		t.Fatalf("UpdateWidget failed: %v", err)
	}

	if updated.Title != "更新后的标题" {
		t.Errorf("Title mismatch: got %s, want 更新后的标题", updated.Title)
	}
}

func TestManager_RemoveWidget(t *testing.T) {
	m := newTestManager(t)

	// 创建带小组件的仪表板
	input := DashboardInput{
		Name:   "测试仪表板",
		Layout: DefaultLayoutConfig(),
		Widgets: []Widget{
			{
				Type:     WidgetTypeSystemStatus,
				Title:    "系统状态",
				Position: GetPresetSize(SizeMedium),
			},
		},
	}
	dashboard, err := m.Create(input, "test-user")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	widgetID := dashboard.Widgets[0].ID

	// 移除小组件
	err = m.RemoveWidget(dashboard.ID, widgetID)
	if err != nil {
		t.Fatalf("RemoveWidget failed: %v", err)
	}

	// 验证已移除
	updated, _ := m.Get(dashboard.ID)
	if len(updated.Widgets) != 0 {
		t.Errorf("Widgets count mismatch: got %d, want 0", len(updated.Widgets))
	}
}

// ========== 模板测试 ==========

func TestManager_ListTemplates(t *testing.T) {
	m := newTestManager(t)

	templates := m.ListTemplates()
	if len(templates) == 0 {
		t.Error("Should have default templates")
	}

	// 检查预定义模板是否存在
	templateIDs := make(map[string]bool)
	for _, t := range templates {
		templateIDs[t.ID] = true
	}

	expectedIDs := []string{"system-overview", "network-monitor", "user-activity"}
	for _, id := range expectedIDs {
		if !templateIDs[id] {
			t.Errorf("Expected template %s not found", id)
		}
	}
}

func TestManager_CreateFromTemplate(t *testing.T) {
	m := newTestManager(t)

	dashboard, err := m.CreateFromTemplate("system-overview", "我的系统概览", "test-user")
	if err != nil {
		t.Fatalf("CreateFromTemplate failed: %v", err)
	}

	if dashboard.Name != "我的系统概览" {
		t.Errorf("Name mismatch: got %s, want 我的系统概览", dashboard.Name)
	}
	if len(dashboard.Widgets) == 0 {
		t.Error("Dashboard should have widgets from template")
	}
}

// ========== 导入导出测试 ==========

func TestManager_ExportImport(t *testing.T) {
	m := newTestManager(t)

	// 创建仪表板
	input := DashboardInput{
		Name:        "测试仪表板",
		Description: "测试描述",
		Layout:      DefaultLayoutConfig(),
		Widgets: []Widget{
			{
				Type:     WidgetTypeSystemStatus,
				Title:    "系统状态",
				Position: GetPresetSize(SizeMedium),
			},
		},
		Tags: []string{"test", "export"},
	}
	created, err := m.Create(input, "test-user")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 导出
	export, err := m.Export(nil)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if len(export.Dashboards) != 1 {
		t.Errorf("Export count mismatch: got %d, want 1", len(export.Dashboards))
	}

	// 导入到新管理器
	m2 := newTestManager(t)
	imported, err := m2.Import(export, "new-user", false)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if len(imported) != 1 {
		t.Errorf("Import count mismatch: got %d, want 1", len(imported))
	}

	// 验证导入的数据
	importedDashboard := imported[0]
	if importedDashboard.Name != created.Name {
		t.Errorf("Name mismatch: got %s, want %s", importedDashboard.Name, created.Name)
	}
	if importedDashboard.Owner != "new-user" {
		t.Errorf("Owner should be new-user, got %s", importedDashboard.Owner)
	}
}

func TestManager_ExportImportJSON(t *testing.T) {
	m := newTestManager(t)

	// 创建仪表板
	input := DashboardInput{
		Name:   "JSON测试",
		Layout: DefaultLayoutConfig(),
	}
	_, err := m.Create(input, "test-user")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 导出为 JSON
	tmpFile := filepath.Join(t.TempDir(), "export.json")
	err = m.ExportToJSON(nil, tmpFile)
	if err != nil {
		t.Fatalf("ExportToJSON failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Fatal("Export file should exist")
	}

	// 从 JSON 导入
	m2 := newTestManager(t)
	imported, err := m2.ImportFromJSON(tmpFile, "import-user", false)
	if err != nil {
		t.Fatalf("ImportFromJSON failed: %v", err)
	}

	if len(imported) != 1 {
		t.Errorf("Import count mismatch: got %d, want 1", len(imported))
	}
}

// ========== 类型测试 ==========

func TestDefaultLayoutConfig(t *testing.T) {
	config := DefaultLayoutConfig()

	if config.Columns != 12 {
		t.Errorf("Columns mismatch: got %d, want 12", config.Columns)
	}
	if config.RowHeight != 60 {
		t.Errorf("RowHeight mismatch: got %d, want 60", config.RowHeight)
	}
	if config.Gap != 16 {
		t.Errorf("Gap mismatch: got %d, want 16", config.Gap)
	}
}

func TestGetPresetSize(t *testing.T) {
	tests := []struct {
		size     WidgetSize
		expected WidgetPosition
	}{
		{SizeSmall, WidgetPosition{Width: 2, Height: 2}},
		{SizeMedium, WidgetPosition{Width: 4, Height: 2}},
		{SizeLarge, WidgetPosition{Width: 6, Height: 3}},
		{SizeWide, WidgetPosition{Width: 6, Height: 2}},
		{SizeFull, WidgetPosition{Width: 12, Height: 2}},
	}

	for _, test := range tests {
		pos := GetPresetSize(test.size)
		if pos.Width != test.expected.Width || pos.Height != test.expected.Height {
			t.Errorf("GetPresetSize(%s) = %+v, want %+v", test.size, pos, test.expected)
		}
	}
}

// ========== 数据提供者测试 ==========

func TestSystemDataProvider_GetSystemStatus(t *testing.T) {
	provider := NewSystemDataProvider(zap.NewNop())

	status, err := provider.GetSystemStatus()
	if err != nil {
		t.Fatalf("GetSystemStatus failed: %v", err)
	}

	if status.Hostname == "" {
		t.Error("Hostname should not be empty")
	}
	if status.Status == "" {
		t.Error("Status should not be empty")
	}
}

func TestSystemDataProvider_GetStorageUsage(t *testing.T) {
	provider := NewSystemDataProvider(zap.NewNop())

	storage, err := provider.GetStorageUsage(nil)
	if err != nil {
		t.Fatalf("GetStorageUsage failed: %v", err)
	}

	// 应该至少有一个挂载点
	if len(storage) == 0 {
		t.Error("Should have at least one mount point")
	}

	// 检查挂载点的数据格式正确
	for _, s := range storage {
		if s.MountPoint == "" {
			t.Error("MountPoint should not be empty")
		}
		// 某些特殊挂载点可能 Total 为 0
		if s.Total < 0 {
			t.Errorf("Total should not be negative for %s", s.MountPoint)
		}
	}
}

func TestSystemDataProvider_GetNetworkTraffic(t *testing.T) {
	provider := NewSystemDataProvider(zap.NewNop())

	traffic, err := provider.GetNetworkTraffic(nil)
	if err != nil {
		t.Fatalf("GetNetworkTraffic failed: %v", err)
	}

	// 可能有或没有网络接口（取决于测试环境）
	// 只检查返回不为 nil
	if traffic == nil {
		t.Error("Traffic should not be nil")
	}
}

func TestSystemDataProvider_GetUserActivity(t *testing.T) {
	provider := NewSystemDataProvider(zap.NewNop())

	activity, err := provider.GetUserActivity(10)
	if err != nil {
		t.Fatalf("GetUserActivity failed: %v", err)
	}

	// 应该返回一些用户活动数据（即使是模拟数据）
	if len(activity) == 0 {
		t.Log("No user activity data returned (this might be expected)")
	}
}

// ========== WidgetDataFetcher 测试 ==========

func TestWidgetDataFetcher_FetchWidgetData(t *testing.T) {
	provider := NewSystemDataProvider(zap.NewNop())
	fetcher := NewWidgetDataFetcher(provider, zap.NewNop())

	tests := []struct {
		name    string
		widget  *Widget
		wantErr bool
	}{
		{
			name: "SystemStatus",
			widget: &Widget{
				Type: WidgetTypeSystemStatus,
			},
			wantErr: false,
		},
		{
			name: "StorageUsage",
			widget: &Widget{
				Type: WidgetTypeStorageUsage,
			},
			wantErr: false,
		},
		{
			name: "NetworkTraffic",
			widget: &Widget{
				Type: WidgetTypeNetworkTraffic,
			},
			wantErr: false,
		},
		{
			name: "UserActivity",
			widget: &Widget{
				Type: WidgetTypeUserActivity,
			},
			wantErr: false,
		},
		{
			name: "Unsupported",
			widget: &Widget{
				Type: WidgetType("unsupported"),
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := fetcher.FetchWidgetData(test.widget)
			if test.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if data == nil {
					t.Error("Data should not be nil")
				}
			}
		})
	}
}

// ========== 持久化测试 ==========

func TestManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dashboards.json")

	// 创建管理器并添加数据
	m1 := NewManager(storePath, zap.NewNop())
	input := DashboardInput{
		Name:   "持久化测试",
		Layout: DefaultLayoutConfig(),
	}
	created, err := m1.Create(input, "test-user")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 创建新管理器，应该加载持久化数据
	m2 := NewManager(storePath, zap.NewNop())
	loaded, err := m2.Get(created.ID)
	if err != nil {
		t.Fatalf("Get failed after reload: %v", err)
	}

	if loaded.Name != "持久化测试" {
		t.Errorf("Name mismatch after reload: got %s, want 持久化测试", loaded.Name)
	}
}

// ========== 基准测试 ==========

func BenchmarkManager_Create(b *testing.B) {
	m := NewManager("", zap.NewNop())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := DashboardInput{
			Name:   "基准测试仪表板",
			Layout: DefaultLayoutConfig(),
		}
		m.Create(input, "bench-user")
	}
}

func BenchmarkManager_Get(b *testing.B) {
	m := NewManager("", zap.NewNop())

	// 创建测试数据
	input := DashboardInput{
		Name:   "基准测试仪表板",
		Layout: DefaultLayoutConfig(),
	}
	created, _ := m.Create(input, "bench-user")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Get(created.ID)
	}
}

func BenchmarkManager_List(b *testing.B) {
	m := NewManager("", zap.NewNop())

	// 创建 100 个仪表板
	for i := 0; i < 100; i++ {
		input := DashboardInput{
			Name:   "基准测试仪表板",
			Layout: DefaultLayoutConfig(),
		}
		m.Create(input, "bench-user")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.List(ListFilter{})
	}
}
