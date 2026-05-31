package applifecycle

import (
	"testing"
)

func TestInstallApp(t *testing.T) {
	m := NewManager()

	app, err := m.Install("nginx", "nginx:latest", "1.25", InstallOptions{
		Ports:   map[string]string{"80": "8080"},
		Volumes: []string{"/data/nginx:/usr/share/nginx/html"},
		Env:     map[string]string{"NGINX_HOST": "example.com"},
	})
	if err != nil {
		t.Fatalf("安装失败: %v", err)
	}

	if app.Name != "nginx" {
		t.Errorf("期望名称 nginx, 得到 %s", app.Name)
	}

	if app.State != StateInstalling {
		t.Errorf("期望状态 installing, 得到 %s", app.State)
	}
}

func TestAppLifecycle(t *testing.T) {
	m := NewManager()

	// 安装
	app, _ := m.Install("redis", "redis:7", "7.0", InstallOptions{})

	// 等待安装完成
	// 注意：在真实场景中需要同步机制
	app.State = StateRunning

	// 停止
	if err := m.Stop(app.ID); err != nil {
		t.Fatalf("停止失败: %v", err)
	}

	app2, _ := m.GetApp(app.ID)
	if app2.State != StateStopped {
		t.Errorf("期望状态 stopped, 得到 %s", app2.State)
	}

	// 启动
	if err := m.Start(app.ID); err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	app3, _ := m.GetApp(app.ID)
	if app3.State != StateRunning {
		t.Errorf("期望状态 running, 得到 %s", app3.State)
	}
}

func TestUpgradeApp(t *testing.T) {
	m := NewManager()

	app, _ := m.Install("postgres", "postgres:15", "15.0", InstallOptions{})
	app.State = StateRunning

	// 升级
	if err := m.Upgrade(app.ID, "16.0"); err != nil {
		t.Fatalf("升级失败: %v", err)
	}

	// 验证有备份记录
	app2, _ := m.GetApp(app.ID)
	if len(app2.Backups) == 0 {
		t.Error("升级后应有备份记录")
	}
}

func TestListApps(t *testing.T) {
	m := NewManager()

	m.Install("app1", "img1", "1.0", InstallOptions{})
	m.Install("app2", "img2", "2.0", InstallOptions{})

	apps := m.ListApps("")
	if len(apps) != 2 {
		t.Errorf("期望2个应用, 得到 %d", len(apps))
	}
}

func TestDuplicateInstall(t *testing.T) {
	m := NewManager()

	m.Install("nginx", "nginx:latest", "1.25", InstallOptions{})
	_, err := m.Install("nginx", "nginx:latest", "1.26", InstallOptions{})
	if err == nil {
		t.Error("重复安装应返回错误")
	}
}
