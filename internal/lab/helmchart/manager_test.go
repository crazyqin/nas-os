package helmchart

import (
	"testing"
)

func TestAddRepo(t *testing.T) {
	m := NewManager()

	repo, err := m.AddRepo("test-repo", "https://example.com/charts", "测试仓库")
	if err != nil {
		t.Fatalf("添加仓库失败: %v", err)
	}

	if repo.Name != "test-repo" {
		t.Errorf("期望仓库名 test-repo，实际 %s", repo.Name)
	}

	if repo.URL != "https://example.com/charts" {
		t.Errorf("期望 URL https://example.com/charts，实际 %s", repo.URL)
	}

	if repo.IsBuiltin {
		t.Error("新建仓库不应是内置仓库")
	}
}

func TestAddDuplicateRepo(t *testing.T) {
	m := NewManager()

	_, err := m.AddRepo("test-repo", "https://example.com/charts", "测试仓库")
	if err != nil {
		t.Fatalf("添加仓库失败: %v", err)
	}

	_, err = m.AddRepo("test-repo", "https://example.com/charts2", "重复仓库")
	if err == nil {
		t.Fatal("期望添加重复仓库失败，但成功了")
	}
}

func TestRemoveBuiltinRepo(t *testing.T) {
	m := NewManager()

	err := m.RemoveRepo("stable")
	if err == nil {
		t.Fatal("期望删除内置仓库失败，但成功了")
	}
}

func TestRemoveRepo(t *testing.T) {
	m := NewManager()

	m.AddRepo("test-repo", "https://example.com/charts", "测试仓库")

	err := m.RemoveRepo("test-repo")
	if err != nil {
		t.Fatalf("删除仓库失败: %v", err)
	}

	repos := m.ListRepos()
	for _, repo := range repos {
		if repo.Name == "test-repo" {
			t.Fatal("仓库应该已被删除")
		}
	}
}

func TestInstallChart(t *testing.T) {
	m := NewManager()

	chart, err := m.InstallChart("my-nginx", "nginx", "1.0.0", "default", nil)
	if err != nil {
		t.Fatalf("安装 Chart 失败: %v", err)
	}

	if chart.Name != "my-nginx" {
		t.Errorf("期望安装名 my-nginx，实际 %s", chart.Name)
	}

	if chart.Chart != "nginx" {
		t.Errorf("期望 Chart nginx，实际 %s", chart.Chart)
	}

	if chart.Status != "deployed" {
		t.Errorf("期望状态 deployed，实际 %s", chart.Status)
	}
}

func TestInstallDuplicateChart(t *testing.T) {
	m := NewManager()

	_, err := m.InstallChart("my-nginx", "nginx", "1.0.0", "default", nil)
	if err != nil {
		t.Fatalf("安装 Chart 失败: %v", err)
	}

	_, err = m.InstallChart("my-nginx", "nginx", "2.0.0", "default", nil)
	if err == nil {
		t.Fatal("期望安装重复 Chart 失败，但成功了")
	}
}

func TestUninstallChart(t *testing.T) {
	m := NewManager()

	m.InstallChart("my-nginx", "nginx", "1.0.0", "default", nil)

	err := m.UninstallChart("my-nginx")
	if err != nil {
		t.Fatalf("卸载 Chart 失败: %v", err)
	}

	_, err = m.GetInstalled("my-nginx")
	if err == nil {
		t.Fatal("期望获取已卸载 Chart 失败，但成功了")
	}
}

func TestUpgradeChart(t *testing.T) {
	m := NewManager()

	m.InstallChart("my-nginx", "nginx", "1.0.0", "default", nil)

	chart, err := m.UpgradeChart("my-nginx", "2.0.0", map[string]string{"replicas": "3"})
	if err != nil {
		t.Fatalf("升级 Chart 失败: %v", err)
	}

	if chart.Version != "2.0.0" {
		t.Errorf("期望版本 2.0.0，实际 %s", chart.Version)
	}

	if chart.Values["replicas"] != "3" {
		t.Errorf("期望 replicas=3，实际 %s", chart.Values["replicas"])
	}
}

func TestListInstalled(t *testing.T) {
	m := NewManager()

	m.InstallChart("nginx-1", "nginx", "1.0.0", "default", nil)
	m.InstallChart("redis-1", "redis", "6.0.0", "default", nil)

	installed := m.ListInstalled()
	if len(installed) != 2 {
		t.Errorf("期望 2 个已安装 Chart，实际 %d", len(installed))
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()

	stats := m.GetStats()

	if stats["repository_count"] != 3 {
		t.Errorf("期望 3 个仓库，实际 %v", stats["repository_count"])
	}

	if stats["installed_count"] != 0 {
		t.Errorf("期望 0 个已安装，实际 %v", stats["installed_count"])
	}
}
