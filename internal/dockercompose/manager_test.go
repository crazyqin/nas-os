package dockercompose

import (
	"testing"
)

func TestCreateProject(t *testing.T) {
	m := NewManager()

	project, err := m.CreateProject("test-app", "/home/user/test-app/docker-compose.yml")
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	if project.Name != "test-app" {
		t.Errorf("期望项目名 test-app，实际 %s", project.Name)
	}

	if project.Status != "stopped" {
		t.Errorf("期望状态 stopped，实际 %s", project.Status)
	}

	if len(project.Networks) != 1 {
		t.Errorf("期望 1 个默认网络，实际 %d", len(project.Networks))
	}
}

func TestCreateDuplicateProject(t *testing.T) {
	m := NewManager()

	_, err := m.CreateProject("test-app", "/path/to/compose.yml")
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	_, err = m.CreateProject("test-app", "/another/path/compose.yml")
	if err == nil {
		t.Fatal("期望创建重复项目失败，但成功了")
	}
}

func TestDeleteProject(t *testing.T) {
	m := NewManager()

	m.CreateProject("test-app", "/path/to/compose.yml")

	err := m.DeleteProject("test-app")
	if err != nil {
		t.Fatalf("删除项目失败: %v", err)
	}

	_, err = m.GetProject("test-app")
	if err == nil {
		t.Fatal("期望获取已删除项目失败，但成功了")
	}
}

func TestAddService(t *testing.T) {
	m := NewManager()

	m.CreateProject("test-app", "/path/to/compose.yml")

	service := &Service{
		Name:  "web",
		Image: "nginx:latest",
		Ports: []string{"8080:80"},
	}

	err := m.AddService("test-app", service)
	if err != nil {
		t.Fatalf("添加服务失败: %v", err)
	}

	project, _ := m.GetProject("test-app")
	if len(project.Services) != 1 {
		t.Errorf("期望 1 个服务，实际 %d", len(project.Services))
	}

	if service.Replicas != 1 {
		t.Errorf("期望默认副本数 1，实际 %d", service.Replicas)
	}

	if service.RestartPolicy != "unless-stopped" {
		t.Errorf("期望重启策略 unless-stopped，实际 %s", service.RestartPolicy)
	}
}

func TestRemoveService(t *testing.T) {
	m := NewManager()

	m.CreateProject("test-app", "/path/to/compose.yml")
	m.AddService("test-app", &Service{Name: "web", Image: "nginx:latest"})

	err := m.RemoveService("test-app", "web")
	if err != nil {
		t.Fatalf("移除服务失败: %v", err)
	}

	project, _ := m.GetProject("test-app")
	if len(project.Services) != 0 {
		t.Errorf("期望 0 个服务，实际 %d", len(project.Services))
	}
}

func TestStartStopProject(t *testing.T) {
	m := NewManager()

	m.CreateProject("test-app", "/path/to/compose.yml")
	m.AddService("test-app", &Service{Name: "web", Image: "nginx:latest"})
	m.AddService("test-app", &Service{Name: "db", Image: "postgres:14"})

	err := m.StartProject("test-app")
	if err != nil {
		t.Fatalf("启动项目失败: %v", err)
	}

	project, _ := m.GetProject("test-app")
	if project.Status != "running" {
		t.Errorf("期望状态 running，实际 %s", project.Status)
	}

	for _, svc := range project.Services {
		if svc.Status != "running" {
			t.Errorf("服务 %s 期望状态 running，实际 %s", svc.Name, svc.Status)
		}
	}

	err = m.StopProject("test-app")
	if err != nil {
		t.Fatalf("停止项目失败: %v", err)
	}

	project, _ = m.GetProject("test-app")
	if project.Status != "stopped" {
		t.Errorf("期望状态 stopped，实际 %s", project.Status)
	}
}

func TestScaleService(t *testing.T) {
	m := NewManager()

	m.CreateProject("test-app", "/path/to/compose.yml")
	m.AddService("test-app", &Service{Name: "web", Image: "nginx:latest"})

	err := m.ScaleService("test-app", "web", 5)
	if err != nil {
		t.Fatalf("扩缩容失败: %v", err)
	}

	project, _ := m.GetProject("test-app")
	if project.Services["web"].Replicas != 5 {
		t.Errorf("期望副本数 5，实际 %d", project.Services["web"].Replicas)
	}
}

func TestGetProjectStats(t *testing.T) {
	m := NewManager()

	m.CreateProject("test-app", "/path/to/compose.yml")
	m.AddService("test-app", &Service{Name: "web", Image: "nginx:latest"})
	m.AddService("test-app", &Service{Name: "db", Image: "postgres:14"})

	stats, err := m.GetProjectStats("test-app")
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	if stats["total_services"] != 2 {
		t.Errorf("期望 2 个服务，实际 %v", stats["total_services"])
	}

	if stats["running_services"] != 0 {
		t.Errorf("期望 0 个运行中服务，实际 %v", stats["running_services"])
	}
}

func TestListProjects(t *testing.T) {
	m := NewManager()

	m.CreateProject("app1", "/path/to/app1/compose.yml")
	m.CreateProject("app2", "/path/to/app2/compose.yml")

	projects := m.ListProjects()
	if len(projects) != 2 {
		t.Errorf("期望 2 个项目，实际 %d", len(projects))
	}
}
