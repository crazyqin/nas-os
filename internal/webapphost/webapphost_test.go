package webapphost

import (
	"testing"
	"time"
)

func TestNewWebAppManager(t *testing.T) {
	manager := NewWebAppManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.config == nil {
		t.Fatal("Expected config to be set")
	}

	if manager.config.MaxApps != 100 {
		t.Errorf("Expected max apps to be 100, got %d", manager.config.MaxApps)
	}
}

func TestCreateApp(t *testing.T) {
	manager := NewWebAppManager(nil)

	config := &DeployConfig{
		AppName: "test-app",
		Type:    "docker",
		Image:   "nginx:latest",
		Path:    "/test",
		Port:    8080,
	}

	app, err := manager.CreateApp(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	if app.ID == "" {
		t.Error("Expected ID to be generated")
	}

	if app.Name != "test-app" {
		t.Errorf("Expected name to be test-app, got %s", app.Name)
	}

	if app.Type != "docker" {
		t.Errorf("Expected type to be docker, got %s", app.Type)
	}

	if app.Status != "stopped" {
		t.Errorf("Expected status to be stopped, got %s", app.Status)
	}
}

func TestCreateAppDuplicateName(t *testing.T) {
	manager := NewWebAppManager(nil)

	config := &DeployConfig{
		AppName: "test-app",
		Type:    "docker",
		Image:   "nginx:latest",
	}

	_, err := manager.CreateApp(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// 重复名称应该失败
	_, err = manager.CreateApp(config)
	if err == nil {
		t.Error("Expected error for duplicate app name")
	}
}

func TestCreateAppMaxLimit(t *testing.T) {
	config := &ManagerConfig{
		MaxApps: 2,
	}
	manager := NewWebAppManager(config)

	// 创建 2 个应用
	for i := 0; i < 2; i++ {
		appConfig := &DeployConfig{
			AppName: "test-app-" + string(rune('a'+i)),
			Type:    "docker",
			Image:   "nginx:latest",
		}
		_, err := manager.CreateApp(appConfig)
		if err != nil {
			t.Fatalf("Failed to create app %d: %v", i, err)
		}
	}

	// 第 3 个应该失败
	appConfig := &DeployConfig{
		AppName: "test-app-c",
		Type:    "docker",
		Image:   "nginx:latest",
	}
	_, err := manager.CreateApp(appConfig)
	if err == nil {
		t.Error("Expected error for exceeding max apps limit")
	}
}

func TestGetApp(t *testing.T) {
	manager := NewWebAppManager(nil)

	config := &DeployConfig{
		AppName: "test-app",
		Type:    "docker",
		Image:   "nginx:latest",
	}

	app, err := manager.CreateApp(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// 获取应用
	retrieved, err := manager.GetApp(app.ID)
	if err != nil {
		t.Fatalf("Failed to get app: %v", err)
	}

	if retrieved.ID != app.ID {
		t.Errorf("Expected app ID %s, got %s", app.ID, retrieved.ID)
	}
}

func TestGetAppNotFound(t *testing.T) {
	manager := NewWebAppManager(nil)

	_, err := manager.GetApp("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent app")
	}
}

func TestListApps(t *testing.T) {
	manager := NewWebAppManager(nil)

	// 创建多个应用
	for i := 0; i < 5; i++ {
		config := &DeployConfig{
			AppName: "test-app-" + string(rune('a'+i)),
			Type:    "docker",
			Image:   "nginx:latest",
		}
		_, err := manager.CreateApp(config)
		if err != nil {
			t.Fatalf("Failed to create app: %v", err)
		}
	}

	// 列出所有应用
	apps := manager.ListApps(nil)
	if len(apps) != 5 {
		t.Errorf("Expected 5 apps, got %d", len(apps))
	}

	// 带分页
	opts := &ListOptions{
		Limit:  2,
		Offset: 1,
	}
	apps = manager.ListApps(opts)
	if len(apps) != 2 {
		t.Errorf("Expected 2 apps with pagination, got %d", len(apps))
	}
}

func TestDeleteApp(t *testing.T) {
	manager := NewWebAppManager(nil)

	config := &DeployConfig{
		AppName: "test-app",
		Type:    "docker",
		Image:   "nginx:latest",
	}

	app, err := manager.CreateApp(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// 删除应用
	err = manager.DeleteApp(app.ID)
	if err != nil {
		t.Fatalf("Failed to delete app: %v", err)
	}

	// 确认已删除
	_, err = manager.GetApp(app.ID)
	if err == nil {
		t.Error("Expected error for deleted app")
	}
}

func TestStartStopApp(t *testing.T) {
	manager := NewWebAppManager(nil)

	config := &DeployConfig{
		AppName: "test-app",
		Type:    "docker",
		Image:   "nginx:latest",
	}

	app, err := manager.CreateApp(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// 启动应用
	err = manager.StartApp(app.ID)
	if err != nil {
		t.Fatalf("Failed to start app: %v", err)
	}

	// 检查状态
	status, err := manager.GetAppStatus(app.ID)
	if err != nil {
		t.Fatalf("Failed to get app status: %v", err)
	}
	if status != "running" {
		t.Errorf("Expected status running, got %s", status)
	}

	// 停止应用
	err = manager.StopApp(app.ID)
	if err != nil {
		t.Fatalf("Failed to stop app: %v", err)
	}

	// 检查状态
	status, err = manager.GetAppStatus(app.ID)
	if err != nil {
		t.Fatalf("Failed to get app status: %v", err)
	}
	if status != "stopped" {
		t.Errorf("Expected status stopped, got %s", status)
	}
}

func TestNewTemplateManager(t *testing.T) {
	tm := NewTemplateManager()
	if tm == nil {
		t.Fatal("Expected template manager to be created")
	}

	// 检查内置模板
	if tm.GetTemplateCount() == 0 {
		t.Error("Expected builtin templates to be registered")
	}
}

func TestGetTemplate(t *testing.T) {
	tm := NewTemplateManager()

	// 获取 WordPress 模板
	tmpl, err := tm.GetTemplate("wordpress")
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}

	if tmpl.Name != "wordpress" {
		t.Errorf("Expected template name wordpress, got %s", tmpl.Name)
	}

	if tmpl.Category != "web" {
		t.Errorf("Expected category web, got %s", tmpl.Category)
	}
}

func TestListTemplates(t *testing.T) {
	tm := NewTemplateManager()

	// 列出所有模板
	templates := tm.ListTemplates(nil)
	if len(templates) == 0 {
		t.Error("Expected templates to be listed")
	}

	// 按分类过滤
	opts := &TemplateListOptions{
		Category: "database",
	}
	templates = tm.ListTemplates(opts)
	for _, tmpl := range templates {
		if tmpl.Category != "database" {
			t.Errorf("Expected category database, got %s", tmpl.Category)
		}
	}
}

func TestNewRouter(t *testing.T) {
	r := NewRouter()
	if r == nil {
		t.Fatal("Expected router to be created")
	}
}

func TestAddRoute(t *testing.T) {
	r := NewRouter()

	route := &RouteRule{
		Domain:   "example.com",
		Path:     "/app",
		AppID:    "app-123",
		Priority: 100,
	}

	err := r.AddRoute(route)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	if route.ID == "" {
		t.Error("Expected ID to be generated")
	}

	// 检查路由数量
	if r.GetRouteCount() != 1 {
		t.Errorf("Expected 1 route, got %d", r.GetRouteCount())
	}
}

func TestMatchRoute(t *testing.T) {
	r := NewRouter()

	route := &RouteRule{
		Domain:   "example.com",
		Path:     "/app",
		AppID:    "app-123",
		Priority: 100,
	}

	err := r.AddRoute(route)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// 匹配路由
	matched, err := r.MatchRoute("example.com", "/app/test")
	if err != nil {
		t.Fatalf("Failed to match route: %v", err)
	}

	if matched.AppID != "app-123" {
		t.Errorf("Expected app ID app-123, got %s", matched.AppID)
	}
}

func TestNewSSLManager(t *testing.T) {
	sm := NewSSLManager("/tmp/ssl", "selfsigned")
	if sm == nil {
		t.Fatal("Expected SSL manager to be created")
	}
}

func TestRequestCertificate(t *testing.T) {
	sm := NewSSLManager("/tmp/ssl", "selfsigned")

	entry, err := sm.RequestCertificate("example.com")
	if err != nil {
		t.Fatalf("Failed to request certificate: %v", err)
	}

	if entry.Domain != "example.com" {
		t.Errorf("Expected domain example.com, got %s", entry.Domain)
	}

	if entry.Status != "active" {
		t.Errorf("Expected status active, got %s", entry.Status)
	}

	if entry.Provider != "selfsigned" {
		t.Errorf("Expected provider selfsigned, got %s", entry.Provider)
	}
}

func TestNewEnvManager(t *testing.T) {
	em := NewEnvManager()
	if em == nil {
		t.Fatal("Expected env manager to be created")
	}
}

func TestSetGetEnv(t *testing.T) {
	em := NewEnvManager()

	err := em.SetEnv("app-123", "DATABASE_URL", "postgres://localhost/db")
	if err != nil {
		t.Fatalf("Failed to set env: %v", err)
	}

	value, err := em.GetEnv("app-123", "DATABASE_URL")
	if err != nil {
		t.Fatalf("Failed to get env: %v", err)
	}

	if value != "postgres://localhost/db" {
		t.Errorf("Expected postgres://localhost/db, got %s", value)
	}
}

func TestNewMonitor(t *testing.T) {
	manager := NewWebAppManager(nil)
	monitor := NewMonitor(manager, nil)

	if monitor == nil {
		t.Fatal("Expected monitor to be created")
	}
}

func TestAddAlertRule(t *testing.T) {
	manager := NewWebAppManager(nil)
	monitor := NewMonitor(manager, nil)

	rule := &AlertRule{
		AppID:     "app-123",
		Name:      "High CPU",
		Type:      "cpu",
		Threshold: 80.0,
		Duration:  5 * time.Minute,
		Notify:    []string{"email"},
	}

	err := monitor.AddAlertRule(rule)
	if err != nil {
		t.Fatalf("Failed to add alert rule: %v", err)
	}

	if rule.ID == "" {
		t.Error("Expected ID to be generated")
	}
}

func TestNewDeployer(t *testing.T) {
	manager := NewWebAppManager(nil)
	deployer := NewDeployer(manager)

	if deployer == nil {
		t.Fatal("Expected deployer to be created")
	}
}

func TestDeploy(t *testing.T) {
	manager := NewWebAppManager(nil)
	deployer := NewDeployer(manager)

	config := &DeployConfig{
		AppName: "test-app",
		Type:    "docker",
		Image:   "nginx:latest",
		Path:    "/test",
	}

	app, err := deployer.Deploy(config)
	if err != nil {
		t.Fatalf("Failed to deploy app: %v", err)
	}

	if app.Name != "test-app" {
		t.Errorf("Expected app name test-app, got %s", app.Name)
	}
}

func TestNewMarketplace(t *testing.T) {
	manager := NewWebAppManager(nil)
	tm := NewTemplateManager()
	mp := NewMarketplace(tm, manager)

	if mp == nil {
		t.Fatal("Expected marketplace to be created")
	}
}

func TestBrowseApps(t *testing.T) {
	manager := NewWebAppManager(nil)
	tm := NewTemplateManager()
	mp := NewMarketplace(tm, manager)

	apps := mp.BrowseApps(&MarketSearchParams{
		Limit: 10,
	})

	if len(apps) == 0 {
		t.Error("Expected apps to be listed")
	}
}

func TestGetMarketStats(t *testing.T) {
	manager := NewWebAppManager(nil)
	tm := NewTemplateManager()
	mp := NewMarketplace(tm, manager)

	stats := mp.GetMarketStats()

	if stats.TotalTemplates == 0 {
		t.Error("Expected total templates > 0")
	}
}

func TestValidateDeployConfig(t *testing.T) {
	// 测试空名称
	config := &DeployConfig{
		Type:  "docker",
		Image: "nginx:latest",
	}
	err := validateDeployConfig(config)
	if err == nil {
		t.Error("Expected error for empty app name")
	}

	// 测试无效类型
	config = &DeployConfig{
		AppName: "test",
		Type:    "invalid",
	}
	err = validateDeployConfig(config)
	if err == nil {
		t.Error("Expected error for invalid app type")
	}

	// 测试 Docker 缺少镜像
	config = &DeployConfig{
		AppName: "test",
		Type:    "docker",
	}
	err = validateDeployConfig(config)
	if err == nil {
		t.Error("Expected error for missing docker image")
	}

	// 测试有效配置
	config = &DeployConfig{
		AppName: "test",
		Type:    "docker",
		Image:   "nginx:latest",
	}
	err = validateDeployConfig(config)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if config.Path != "/" {
		t.Errorf("Expected default path /, got %s", config.Path)
	}

	if config.Version != "latest" {
		t.Errorf("Expected default version latest, got %s", config.Version)
	}
}
