package openclaw

import (
	"testing"
	"time"
)

func TestNewOpenClawManager(t *testing.T) {
	config := ManagerConfig{
		DataDir:       "/var/lib/openclaw",
		MaxApps:       10,
		EnableMetrics: true,
		EnableLogs:    true,
	}

	manager := NewOpenClawManager(config)
	if manager == nil {
		t.Fatal("NewOpenClawManager returned nil")
	}
}

func TestOpenClawManager_DeployApp(t *testing.T) {
	config := ManagerConfig{
		MaxApps: 5,
	}

	manager := NewOpenClawManager(config)

	appConfig := map[string]interface{}{
		"port": 8080,
		"env":  "production",
	}

	err := manager.DeployApp("test-app", "1.0.0", appConfig)
	if err != nil {
		t.Fatalf("DeployApp failed: %v", err)
	}

	// 检查应用状态
	app, err := manager.GetApp("test-app")
	if err != nil {
		t.Fatalf("GetApp failed: %v", err)
	}

	if app.Status != StatusDeploying {
		t.Errorf("Expected status 'deploying', got '%s'", app.Status)
	}

	// 等待部署完成
	time.Sleep(6 * time.Second)

	app, _ = manager.GetApp("test-app")
	if app.Status != StatusRunning {
		t.Errorf("Expected status 'running', got '%s'", app.Status)
	}
}

func TestOpenClawManager_StopStartApp(t *testing.T) {
	config := ManagerConfig{
		MaxApps: 5,
	}

	manager := NewOpenClawManager(config)

	// 部署应用
	manager.DeployApp("test-app", "1.0.0", nil)

	// 等待部署完成
	time.Sleep(6 * time.Second)

	// 停止应用
	err := manager.StopApp("test-app")
	if err != nil {
		t.Fatalf("StopApp failed: %v", err)
	}

	app, _ := manager.GetApp("test-app")
	if app.Status != StatusStopped {
		t.Errorf("Expected status 'stopped', got '%s'", app.Status)
	}

	// 启动应用
	err = manager.StartApp("test-app")
	if err != nil {
		t.Fatalf("StartApp failed: %v", err)
	}

	app, _ = manager.GetApp("test-app")
	if app.Status != StatusRunning {
		t.Errorf("Expected status 'running', got '%s'", app.Status)
	}
}

func TestOpenClawManager_ListApps(t *testing.T) {
	config := ManagerConfig{
		MaxApps: 5,
	}

	manager := NewOpenClawManager(config)

	// 部署多个应用
	manager.DeployApp("app1", "1.0.0", nil)
	manager.DeployApp("app2", "2.0.0", nil)

	apps := manager.ListApps()
	if len(apps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(apps))
	}
}

func TestOpenClawManager_RemoveApp(t *testing.T) {
	config := ManagerConfig{
		MaxApps: 5,
	}

	manager := NewOpenClawManager(config)

	// 部署应用
	manager.DeployApp("test-app", "1.0.0", nil)

	// 移除应用
	err := manager.RemoveApp("test-app")
	if err != nil {
		t.Fatalf("RemoveApp failed: %v", err)
	}

	// 验证应用已移除
	_, err = manager.GetApp("test-app")
	if err == nil {
		t.Error("Expected error when getting removed app")
	}
}

func TestOpenClawManager_CreateWorkflow(t *testing.T) {
	config := ManagerConfig{}
	manager := NewOpenClawManager(config)

	workflow := &Workflow{
		ID:          "test-workflow",
		Name:        "Test Workflow",
		Description: "A test workflow",
		Enabled:     true,
		Steps: []WorkflowStep{
			{
				Name: "step1",
				Type: "http",
				Config: map[string]interface{}{
					"url": "http://example.com",
				},
				Timeout: 30 * time.Second,
			},
		},
	}

	err := manager.CreateWorkflow(workflow)
	if err != nil {
		t.Fatalf("CreateWorkflow failed: %v", err)
	}

	// 获取工作流
	retrieved, err := manager.GetWorkflow("test-workflow")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}

	if retrieved.Name != "Test Workflow" {
		t.Errorf("Expected name 'Test Workflow', got '%s'", retrieved.Name)
	}
}

func TestOpenClawManager_ExecuteWorkflow(t *testing.T) {
	config := ManagerConfig{}
	manager := NewOpenClawManager(config)

	workflow := &Workflow{
		ID:      "test-workflow",
		Name:    "Test Workflow",
		Enabled: true,
		Steps: []WorkflowStep{
			{
				Name: "step1",
				Type: "script",
			},
		},
	}

	manager.CreateWorkflow(workflow)

	// 执行工作流
	err := manager.ExecuteWorkflow("test-workflow")
	if err != nil {
		t.Fatalf("ExecuteWorkflow failed: %v", err)
	}

	// 等待执行完成
	time.Sleep(1 * time.Second)
}

func TestOpenClawManager_GetStats(t *testing.T) {
	config := ManagerConfig{
		MaxApps: 5,
	}

	manager := NewOpenClawManager(config)

	// 部署应用
	manager.DeployApp("app1", "1.0.0", nil)
	manager.DeployApp("app2", "2.0.0", nil)

	// 创建工作流
	workflow := &Workflow{
		ID:      "workflow1",
		Name:    "Workflow 1",
		Enabled: true,
	}
	manager.CreateWorkflow(workflow)

	stats := manager.GetStats()

	if stats["total_apps"] != 2 {
		t.Errorf("Expected total_apps 2, got %v", stats["total_apps"])
	}

	if stats["total_workflows"] != 1 {
		t.Errorf("Expected total_workflows 1, got %v", stats["total_workflows"])
	}
}
