package appmarket

import (
	"testing"
	"time"
)

func TestNewAppMarket(t *testing.T) {
	market := NewAppMarket(nil)
	if market == nil {
		t.Fatal("expected non-nil market")
	}

	categories := market.GetCategories()
	if len(categories) == 0 {
		t.Error("expected categories to be populated")
	}
}

func TestRegisterAndGetApp(t *testing.T) {
	market := NewAppMarket(nil)

	app := &App{
		ID:          "nginx",
		Name:        "nginx",
		Title:       "Nginx Web Server",
		Description: "High performance web server",
		Category:    CategoryNetwork,
		Version:     "1.24.0",
		Image:       "nginx",
		ImageTag:    "latest",
	}

	if err := market.RegisterApp(app); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, exists := market.GetApp("nginx")
	if !exists {
		t.Fatal("expected app to exist")
	}
	if got.Title != "Nginx Web Server" {
		t.Errorf("expected title 'Nginx Web Server', got %q", got.Title)
	}
}

func TestRegisterDuplicateApp(t *testing.T) {
	market := NewAppMarket(nil)

	app1 := &App{
		ID:   "nginx",
		Name: "nginx",
	}

	app2 := &App{
		ID:   "nginx",
		Name: "nginx2",
	}

	market.RegisterApp(app1)

	if err := market.RegisterApp(app2); err == nil {
		t.Error("expected error for duplicate app")
	}
}

func TestInstallAndUninstallApp(t *testing.T) {
	market := NewAppMarket(nil)

	app := &App{
		ID:      "nginx",
		Name:    "nginx",
		Version: "1.24.0",
		Image:   "nginx",
	}

	market.RegisterApp(app)

	// 安装
	if err := market.InstallApp("nginx", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	installed, exists := market.GetInstalledApp("nginx")
	if !exists {
		t.Fatal("expected app to be installed")
	}
	if installed.Status != AppStatusInstalling {
		t.Errorf("expected installing status, got %v", installed.Status)
	}

	// 卸载
	if err := market.UninstallApp("nginx"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists = market.GetInstalledApp("nginx")
	if exists {
		t.Error("expected app to be uninstalled")
	}
}

func TestStartAndStopApp(t *testing.T) {
	market := NewAppMarket(nil)

	app := &App{
		ID:      "nginx",
		Name:    "nginx",
		Version: "1.24.0",
		Image:   "nginx",
	}

	market.RegisterApp(app)
	market.InstallApp("nginx", nil)

	// 启动
	if err := market.StartApp("nginx"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	installed, _ := market.GetInstalledApp("nginx")
	if installed.Status != AppStatusInstalled {
		t.Errorf("expected installed status, got %v", installed.Status)
	}

	// 停止
	if err := market.StopApp("nginx"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	installed, _ = market.GetInstalledApp("nginx")
	if installed.Status != AppStatusDisabled {
		t.Errorf("expected disabled status, got %v", installed.Status)
	}
}

func TestSearchApps(t *testing.T) {
	market := NewAppMarket(nil)

	market.RegisterApp(&App{
		ID:          "nginx",
		Name:        "nginx",
		Title:       "Nginx Web Server",
		Description: "High performance web server",
	})

	market.RegisterApp(&App{
		ID:          "redis",
		Name:        "redis",
		Title:       "Redis Cache",
		Description: "In-memory data store",
	})

	results := market.SearchApps("web")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "nginx" {
		t.Errorf("expected nginx, got %q", results[0].ID)
	}
}

func TestListByCategory(t *testing.T) {
	market := NewAppMarket(nil)

	market.RegisterApp(&App{
		ID:       "nginx",
		Name:     "nginx",
		Category: CategoryNetwork,
	})

	market.RegisterApp(&App{
		ID:       "postgres",
		Name:     "postgres",
		Category: CategoryDatabase,
	})

	networkApps := market.ListApps(&[]AppCategory{CategoryNetwork}[0])
	if len(networkApps) != 1 {
		t.Errorf("expected 1 network app, got %d", len(networkApps))
	}
}

func TestGetStats(t *testing.T) {
	market := NewAppMarket(nil)

	market.RegisterApp(&App{
		ID:       "nginx",
		Name:     "nginx",
		Category: CategoryNetwork,
	})

	stats := market.GetStats()
	totalApps := stats["total_apps"].(int)
	if totalApps != 1 {
		t.Errorf("expected 1 app, got %d", totalApps)
	}
}

func TestGetTopApps(t *testing.T) {
	market := NewAppMarket(nil)

	market.RegisterApp(&App{
		ID:        "nginx",
		Name:      "nginx",
		Downloads: 100,
		CreatedAt: time.Now(),
	})

	market.RegisterApp(&App{
		ID:        "redis",
		Name:      "redis",
		Downloads: 50,
		CreatedAt: time.Now(),
	})

	topApps := market.GetTopApps(1)
	if len(topApps) != 1 {
		t.Errorf("expected 1 app, got %d", len(topApps))
	}
	if topApps[0].ID != "nginx" {
		t.Errorf("expected nginx, got %q", topApps[0].ID)
	}
}
