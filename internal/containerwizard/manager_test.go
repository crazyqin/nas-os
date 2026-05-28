package containerwizard

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	stats := m.GetStats()
	if stats["totalTemplates"].(int) == 0 {
		t.Error("expected default templates")
	}
}

func TestGetTemplate(t *testing.T) {
	m := NewManager()

	tmpl, err := m.GetTemplate("jellyfin")
	if err != nil {
		t.Fatalf("get template failed: %v", err)
	}
	if tmpl.Name != "Jellyfin" {
		t.Errorf("expected 'Jellyfin', got '%s'", tmpl.Name)
	}
	if !tmpl.Featured {
		t.Error("expected featured")
	}

	_, err = m.GetTemplate("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestListTemplates(t *testing.T) {
	m := NewManager()

	// 列出所有
	all := m.ListTemplates("", "")
	if len(all) < 10 {
		t.Errorf("expected at least 10 templates, got %d", len(all))
	}

	// 按分类
	media := m.ListTemplates(CategoryMedia, "")
	if len(media) == 0 {
		t.Error("expected media templates")
	}

	// 按搜索
	search := m.ListTemplates("", "redis")
	if len(search) == 0 {
		t.Error("expected search results for 'redis'")
	}
}

func TestListFeatured(t *testing.T) {
	m := NewManager()

	featured := m.ListFeatured()
	if len(featured) == 0 {
		t.Error("expected featured templates")
	}
	for _, f := range featured {
		if !f.Featured {
			t.Error("expected all to be featured")
		}
	}
}

func TestDeploy(t *testing.T) {
	m := NewManager()

	task, err := m.Deploy("jellyfin", "我的媒体服务器", map[string]string{
		"POSTGRES_PASSWORD": "secret123",
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	if task.ID == "" {
		t.Error("expected task ID")
	}
	if task.Status != DeployStatusPending {
		t.Errorf("expected pending, got '%s'", task.Status)
	}

	// 等待部署完成
	time.Sleep(500 * time.Millisecond)

	fetched, _ := m.GetDeployTask(task.ID)
	if fetched.Status != DeployStatusRunning {
		t.Errorf("expected running, got '%s'", fetched.Status)
	}
	if fetched.ContainerID == "" {
		t.Error("expected container ID")
	}

	// 重复部署
	_, err = m.Deploy("jellyfin", "重复部署", nil)
	if err == nil {
		t.Error("expected error for duplicate deploy")
	}

	// 不存在的模板
	_, err = m.Deploy("nonexistent", "test", nil)
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestListDeployTasks(t *testing.T) {
	m := NewManager()

	m.Deploy("redis", "redis1", nil)
	m.Deploy("postgres", "pg1", nil)

	tasks := m.ListDeployTasks()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestGetInstalled(t *testing.T) {
	m := NewManager()

	m.Deploy("redis", "cache", nil)

	installed := m.GetInstalled()
	if len(installed) != 1 {
		t.Errorf("expected 1 installed, got %d", len(installed))
	}
}

func TestStackTemplates(t *testing.T) {
	m := NewManager()

	stacks := m.ListStackTemplates()
	if len(stacks) < 2 {
		t.Errorf("expected at least 2 stack templates, got %d", len(stacks))
	}

	stack, err := m.GetStackTemplate("fullstack-web")
	if err != nil {
		t.Fatalf("get stack failed: %v", err)
	}
	if len(stack.Templates) != 4 {
		t.Errorf("expected 4 items, got %d", len(stack.Templates))
	}
}

func TestDeployStack(t *testing.T) {
	m := NewManager()

	tasks, err := m.DeployStack("smart-home", nil)
	if err != nil {
		t.Fatalf("deploy stack failed: %v", err)
	}
	if len(tasks) == 0 {
		t.Error("expected tasks")
	}

	time.Sleep(500 * time.Millisecond)

	for _, task := range tasks {
		fetched, _ := m.GetDeployTask(task.ID)
		if fetched.Status != DeployStatusRunning {
			t.Errorf("expected running for %s, got '%s'", task.Name, fetched.Status)
		}
	}
}

func TestResourceRecommend(t *testing.T) {
	m := NewManager()

	rec, err := m.GetResourceRecommend("jellyfin")
	if err != nil {
		t.Fatalf("get recommend failed: %v", err)
	}
	if rec.CPU <= 0 {
		t.Error("expected positive CPU")
	}
	if rec.Memory <= 0 {
		t.Error("expected positive memory")
	}
}

func TestStats(t *testing.T) {
	m := NewManager()

	m.Deploy("redis", "cache", nil)

	stats := m.GetStats()
	if stats["totalTemplates"].(int) == 0 {
		t.Error("expected templates")
	}
	if stats["totalDeployed"].(int) != 1 {
		t.Errorf("expected 1 deployed, got %v", stats["totalDeployed"])
	}
}
