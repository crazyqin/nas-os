package dashboard

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"nas-os/internal/monitor"
)

func TestNewManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dashboard-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(&ManagerConfig{
		DataDir:     tmpDir,
		RefreshRate: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr == nil {
		t.Fatal("Expected non-nil manager")
	}

	if mgr.dataDir != tmpDir {
		t.Errorf("DataDir mismatch: got %s, want %s", mgr.dataDir, tmpDir)
	}
}

func TestNewManager_DefaultRefreshRate(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 默认刷新率应该是 5 秒
	if mgr.refreshTicker != nil {
		t.Error("Expected refreshTicker to be nil before Start()")
	}
}

func TestNewManager_EmptyDataDir(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{
		DataDir: "",
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr.dataDir != "" {
		t.Errorf("Expected empty dataDir, got %s", mgr.dataDir)
	}
}

func TestNewManager_WithMonitorManager(t *testing.T) {
	// 创建实际的 monitor.Manager
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Fatalf("Failed to create monitor manager: %v", err)
	}

	mgr, err := NewManager(&ManagerConfig{
		MonitorManager: monMgr,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr.registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	types := mgr.GetWidgetTypes()
	if len(types) < 4 {
		t.Errorf("Expected at least 4 widget types, got %d", len(types))
	}
}

func TestManager_CreateDashboard(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	dashboard, err := mgr.CreateDashboard("Test Dashboard", "Test Description")
	if err != nil {
		t.Fatalf("Failed to create dashboard: %v", err)
	}

	if dashboard.ID == "" {
		t.Error("Expected non-empty dashboard ID")
	}
	if dashboard.Name != "Test Dashboard" {
		t.Errorf("Name mismatch: got %s, want Test Dashboard", dashboard.Name)
	}
	if dashboard.Description != "Test Description" {
		t.Errorf("Description mismatch: got %s, want Test Description", dashboard.Description)
	}
	if len(dashboard.Widgets) != 0 {
		t.Errorf("Expected empty widgets, got %d", len(dashboard.Widgets))
	}
	if dashboard.Layout.Columns != 2 {
		t.Errorf("Expected default layout columns 2, got %d", dashboard.Layout.Columns)
	}
}

func TestManager_GetDashboard(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	created, err := mgr.CreateDashboard("Test", "Test")
	if err != nil {
		t.Fatalf("Failed to create dashboard: %v", err)
	}

	retrieved, err := mgr.GetDashboard(created.ID)
	if err != nil {
		t.Fatalf("Failed to get dashboard: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, created.ID)
	}
}

func TestManager_GetDashboard_NotFound(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_, err = mgr.GetDashboard("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent dashboard")
	}
}

func TestManager_ListDashboards(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 初始应为空
	list := mgr.ListDashboards()
	if len(list) != 0 {
		t.Errorf("Expected empty list, got %d", len(list))
	}

	// 创建仪表板
	_, _ = mgr.CreateDashboard("Dashboard 1", "Desc 1")
	_, _ = mgr.CreateDashboard("Dashboard 2", "Desc 2")

	list = mgr.ListDashboards()
	if len(list) != 2 {
		t.Errorf("Expected 2 dashboards, got %d", len(list))
	}
}

func TestManager_UpdateDashboard(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	dashboard, _ := mgr.CreateDashboard("Original", "Original Desc")
	dashboard.Name = "Updated"
	dashboard.Description = "Updated Desc"

	err = mgr.UpdateDashboard(dashboard)
	if err != nil {
		t.Fatalf("Failed to update dashboard: %v", err)
	}

	retrieved, _ := mgr.GetDashboard(dashboard.ID)
	if retrieved.Name != "Updated" {
		t.Errorf("Name mismatch: got %s, want Updated", retrieved.Name)
	}
}

func TestManager_UpdateDashboard_NotFound(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	dashboard := &Dashboard{
		ID:          "non-existent",
		Name:        "Test",
		Description: "Test",
	}

	err = mgr.UpdateDashboard(dashboard)
	if err == nil {
		t.Error("Expected error for non-existent dashboard")
	}
}

func TestManager_DeleteDashboard(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	dashboard, _ := mgr.CreateDashboard("Test", "Test")

	err = mgr.DeleteDashboard(dashboard.ID)
	if err != nil {
		t.Fatalf("Failed to delete dashboard: %v", err)
	}

	_, err = mgr.GetDashboard(dashboard.ID)
	if err == nil {
		t.Error("Expected error for deleted dashboard")
	}
}

func TestManager_DeleteDashboard_NotFound(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = mgr.DeleteDashboard("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent dashboard")
	}
}

func TestManager_AddWidget(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	dashboard, _ := mgr.CreateDashboard("Test", "Test")
	widget := &Widget{
		Type:    WidgetTypeCPU,
		Title:   "CPU Monitor",
		Size:    WidgetSizeMedium,
		Enabled: true,
	}

	err = mgr.AddWidget(dashboard.ID, widget)
	if err != nil {
		t.Fatalf("Failed to add widget: %v", err)
	}

	if widget.ID == "" {
		t.Error("Expected widget ID to be generated")
	}

	retrieved, _ := mgr.GetDashboard(dashboard.ID)
	if len(retrieved.Widgets) != 1 {
		t.Errorf("Expected 1 widget, got %d", len(retrieved.Widgets))
	}
}

func TestManager_AddWidget_DashboardNotFound(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	widget := &Widget{
		Type:  WidgetTypeCPU,
		Title: "CPU",
	}

	err = mgr.AddWidget("non-existent", widget)
	if err == nil {
		t.Error("Expected error for non-existent dashboard")
	}
}

func TestManager_RemoveWidget(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	dashboard, _ := mgr.CreateDashboard("Test", "Test")
	widget := &Widget{
		Type:  WidgetTypeCPU,
		Title: "CPU",
	}
	_ = mgr.AddWidget(dashboard.ID, widget)

	err = mgr.RemoveWidget(dashboard.ID, widget.ID)
	if err != nil {
		t.Fatalf("Failed to remove widget: %v", err)
	}

	retrieved, _ := mgr.GetDashboard(dashboard.ID)
	if len(retrieved.Widgets) != 0 {
		t.Errorf("Expected 0 widgets, got %d", len(retrieved.Widgets))
	}
}

func TestManager_RemoveWidget_NotFound(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	dashboard, _ := mgr.CreateDashboard("Test", "Test")

	err = mgr.RemoveWidget(dashboard.ID, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent widget")
	}
}

func TestManager_UpdateWidget(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	dashboard, _ := mgr.CreateDashboard("Test", "Test")
	widget := &Widget{
		Type:  WidgetTypeCPU,
		Title: "Original Title",
	}
	_ = mgr.AddWidget(dashboard.ID, widget)

	widget.Title = "Updated Title"
	err = mgr.UpdateWidget(dashboard.ID, widget)
	if err != nil {
		t.Fatalf("Failed to update widget: %v", err)
	}

	retrieved, _ := mgr.GetDashboard(dashboard.ID)
	if retrieved.Widgets[0].Title != "Updated Title" {
		t.Errorf("Title mismatch: got %s, want Updated Title", retrieved.Widgets[0].Title)
	}
}

func TestManager_Subscribe(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ch := mgr.Subscribe()
	if ch == nil {
		t.Fatal("Expected non-nil channel")
	}

	// 验证频道有缓冲
	if cap(ch) != 100 {
		t.Errorf("Expected channel buffer 100, got %d", cap(ch))
	}

	// 清理
	mgr.Unsubscribe(ch)
}

func TestManager_Unsubscribe(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ch := mgr.Subscribe()
	mgr.Unsubscribe(ch)

	// 验证已从订阅者列表移除
	mgr.mu.RLock()
	count := len(mgr.subscribers)
	mgr.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 subscribers, got %d", count)
	}
}

func TestManager_ExportImport(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建仪表板
	dashboard, _ := mgr.CreateDashboard("Test", "Test")
	_ = mgr.AddWidget(dashboard.ID, &Widget{
		Type:  WidgetTypeCPU,
		Title: "CPU",
	})

	// 导出
	data, err := mgr.ExportDashboard(dashboard.ID)
	if err != nil {
		t.Fatalf("Failed to export dashboard: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty export data")
	}

	// 导入
	imported, err := mgr.ImportDashboard(data)
	if err != nil {
		t.Fatalf("Failed to import dashboard: %v", err)
	}

	if imported.ID == dashboard.ID {
		t.Error("Imported dashboard should have new ID")
	}
	if imported.Name != dashboard.Name {
		t.Errorf("Name mismatch: got %s, want %s", imported.Name, dashboard.Name)
	}
}

func TestManager_CloneDashboard(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建原始仪表板
	original, _ := mgr.CreateDashboard("Original", "Original Desc")
	_ = mgr.AddWidget(original.ID, &Widget{
		Type:  WidgetTypeCPU,
		Title: "CPU",
	})

	// 克隆
	clone, err := mgr.CloneDashboard(original.ID, "Cloned Dashboard")
	if err != nil {
		t.Fatalf("Failed to clone dashboard: %v", err)
	}

	if clone.ID == original.ID {
		t.Error("Clone should have different ID")
	}
	if clone.Name != "Cloned Dashboard" {
		t.Errorf("Clone name mismatch: got %s, want Cloned Dashboard", clone.Name)
	}
	if len(clone.Widgets) != len(original.Widgets) {
		t.Errorf("Widgets count mismatch: got %d, want %d", len(clone.Widgets), len(original.Widgets))
	}
	if clone.IsDefault {
		t.Error("Clone should not be default")
	}
}

func TestManager_CloneDashboard_NotFound(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_, err = mgr.CloneDashboard("non-existent", "Clone")
	if err == nil {
		t.Error("Expected error for non-existent dashboard")
	}
}

func TestManager_SaveLoadDashboards(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dashboard-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(&ManagerConfig{
		DataDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建仪表板
	_, _ = mgr.CreateDashboard("Test 1", "Description 1")
	_, _ = mgr.CreateDashboard("Test 2", "Description 2")

	// 保存
	err = mgr.saveDashboards()
	if err != nil {
		t.Fatalf("Failed to save dashboards: %v", err)
	}

	// 验证文件存在
	path := filepath.Join(tmpDir, "dashboards.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Expected dashboards.json to exist")
	}

	// 创建新管理器并加载
	mgr2, err := NewManager(&ManagerConfig{
		DataDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create second manager: %v", err)
	}

	list := mgr2.ListDashboards()
	if len(list) != 2 {
		t.Errorf("Expected 2 dashboards, got %d", len(list))
	}
}

func TestManager_StartStop(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 启动
	mgr.Start()
	time.Sleep(100 * time.Millisecond)

	if !mgr.running {
		t.Error("Expected manager to be running")
	}

	// 再次启动应该无操作
	mgr.Start()

	// 停止
	mgr.Stop()
	time.Sleep(100 * time.Millisecond)

	if mgr.running {
		t.Error("Expected manager to be stopped")
	}
}

func TestManager_RunWithContext(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		mgr.RunWithContext(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	if !mgr.running {
		t.Error("Expected manager to be running")
	}

	// 取消上下文
	cancel()
	wg.Wait()

	if mgr.running {
		t.Error("Expected manager to be stopped after context cancel")
	}
}

func TestManager_CreateDefaultDashboard(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	dashboard, err := mgr.CreateDefaultDashboard()
	if err != nil {
		t.Fatalf("Failed to create default dashboard: %v", err)
	}

	if dashboard.Name != "系统监控" {
		t.Errorf("Name mismatch: got %s, want 系统监控", dashboard.Name)
	}
	if !dashboard.IsDefault {
		t.Error("Expected dashboard to be default")
	}
	if len(dashboard.Widgets) != 4 {
		t.Errorf("Expected 4 default widgets, got %d", len(dashboard.Widgets))
	}
}

func TestManager_GetWidgetTypes(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Fatalf("Failed to create monitor manager: %v", err)
	}

	mgr, err := NewManager(&ManagerConfig{
		MonitorManager: monMgr,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	types := mgr.GetWidgetTypes()
	if len(types) < 4 {
		t.Errorf("Expected at least 4 widget types, got %d", len(types))
	}

	// 验证包含基本类型
	typeMap := make(map[WidgetType]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	if !typeMap[WidgetTypeCPU] {
		t.Error("Expected CPU widget type")
	}
	if !typeMap[WidgetTypeMemory] {
		t.Error("Expected Memory widget type")
	}
	if !typeMap[WidgetTypeDisk] {
		t.Error("Expected Disk widget type")
	}
	if !typeMap[WidgetTypeNetwork] {
		t.Error("Expected Network widget type")
	}
}

func TestManager_GetDashboardState_NotFound(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_, err = mgr.GetDashboardState("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent dashboard")
	}
}

func TestManager_GetWidgetData_NotFound(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_, err = mgr.GetWidgetData("non-existent", "widget-id")
	if err == nil {
		t.Error("Expected error for non-existent dashboard")
	}
}

func TestManager_RefreshDashboard_NotFound(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = mgr.RefreshDashboard("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent dashboard")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Error("Expected non-empty ID")
	}
	if id1 == id2 {
		t.Error("Expected different IDs")
	}
}

func TestManager_PublishEvent(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 订阅事件
	ch := mgr.Subscribe()

	// 创建仪表板（会触发事件）
	_, _ = mgr.CreateDashboard("Test", "Test")

	// 等待事件
	select {
	case event := <-ch:
		if event.Type != "create" {
			t.Errorf("Expected event type 'create', got %s", event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive event")
	}

	mgr.Unsubscribe(ch)
}

func TestManager_ConcurrentAccess(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	var wg sync.WaitGroup
	const goroutines = 10
	const operations = 100

	// 并发创建仪表板
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				_, _ = mgr.CreateDashboard("Test", "Test")
			}
		}(i)
	}

	// 并发读取
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				_ = mgr.ListDashboards()
			}
		}()
	}

	wg.Wait()

	list := mgr.ListDashboards()
	expected := goroutines * operations
	if len(list) != expected {
		t.Errorf("Expected %d dashboards, got %d", expected, len(list))
	}
}