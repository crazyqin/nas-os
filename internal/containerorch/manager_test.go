package containerorch

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("Expected manager")
	}
}

func TestMgrCreateProject(t *testing.T) {
	m := NewManager()

	project, err := m.CreateProject(CreateProjectRequest{
		Name:        "test-project",
		Description: "测试项目",
		Namespace:   "default",
		Services: map[string]*ServiceConfig{
			"web": {
				Image: "nginx:latest",
				Ports: []PortMapping{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("create project failed: %v", err)
	}
	if project.ID == "" {
		t.Error("Expected project ID")
	}
	if project.Name != "test-project" {
		t.Errorf("Expected name 'test-project', got '%s'", project.Name)
	}
	if project.Status != ProjectStatusCreating {
		t.Errorf("Expected creating status, got '%s'", project.Status)
	}
}

func TestMgrCreateProjectValidation(t *testing.T) {
	m := NewManager()

	// 测试无效服务名称
	_, err := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"INVALID_NAME": {Image: "nginx"},
		},
	})
	if err == nil {
		t.Error("Expected error for invalid service name")
	}
}

func TestMgrGetProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	fetched, err := m.GetProject(project.ID)
	if err != nil {
		t.Fatalf("get project failed: %v", err)
	}
	if fetched.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", fetched.Name)
	}

	_, err = m.GetProject("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent project")
	}
}

func TestMgrListProjects(t *testing.T) {
	m := NewManager()

	m.CreateProject(CreateProjectRequest{
		Name:      "p1",
		Namespace: "ns1",
		Services:  map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})
	m.CreateProject(CreateProjectRequest{
		Name:      "p2",
		Namespace: "ns2",
		Services:  map[string]*ServiceConfig{"api": {Image: "node"}},
	})

	all := m.ListProjects("")
	if len(all) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(all))
	}

	ns1 := m.ListProjects("ns1")
	if len(ns1) != 1 {
		t.Errorf("Expected 1 project in ns1, got %d", len(ns1))
	}
}

func TestMgrUpdateProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name:       "original",
		Services:   map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})

	newName := "updated"
	updated, err := m.UpdateProject(project.ID, UpdateProjectRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("update project failed: %v", err)
	}
	if updated.Name != "updated" {
		t.Errorf("Expected name 'updated', got '%s'", updated.Name)
	}
}

func TestMgrDeleteProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name:      "to-delete",
		Services:  map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})

	err := m.DeleteProject(project.ID, false)
	if err != nil {
		t.Fatalf("delete project failed: %v", err)
	}

	_, err = m.GetProject(project.ID)
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestMgrStartStopProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "lifecycle",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	// 启动
	started, err := m.StartProject(project.ID)
	if err != nil {
		t.Fatalf("start project failed: %v", err)
	}
	if started.Status != ProjectStatusRunning {
		t.Errorf("Expected running, got '%s'", started.Status)
	}

	svc := started.Services["web"]
	if svc.Status != ServiceStatusRunning {
		t.Errorf("Expected service running, got '%s'", svc.Status)
	}
	if svc.Instances != 1 {
		t.Errorf("Expected 1 instance, got %d", svc.Instances)
	}
	if len(svc.ContainerIDs) != 1 {
		t.Errorf("Expected 1 container ID, got %d", len(svc.ContainerIDs))
	}

	// 停止
	stopped, err := m.StopProject(project.ID)
	if err != nil {
		t.Fatalf("stop project failed: %v", err)
	}
	if stopped.Status != ProjectStatusStopped {
		t.Errorf("Expected stopped, got '%s'", stopped.Status)
	}

	// 重启
	restarted, err := m.RestartProject(project.ID)
	if err != nil {
		t.Fatalf("restart project failed: %v", err)
	}
	if restarted.Status != ProjectStatusRunning {
		t.Errorf("Expected running after restart, got '%s'", restarted.Status)
	}
}

func TestMgrScaleService(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "scale-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	// 扩容
	scaled, err := m.ScaleService(project.ID, "web", 3)
	if err != nil {
		t.Fatalf("scale service failed: %v", err)
	}
	if scaled.Instances != 3 {
		t.Errorf("Expected 3 instances, got %d", scaled.Instances)
	}
	if len(scaled.ContainerIDs) != 3 {
		t.Errorf("Expected 3 container IDs, got %d", len(scaled.ContainerIDs))
	}

	// 缩容
	scaled, err = m.ScaleService(project.ID, "web", 1)
	if err != nil {
		t.Fatalf("scale down failed: %v", err)
	}
	if scaled.Instances != 1 {
		t.Errorf("Expected 1 instance, got %d", scaled.Instances)
	}

	// 负数副本
	_, err = m.ScaleService(project.ID, "web", -1)
	if err == nil {
		t.Error("Expected error for negative replicas")
	}
}

func TestMgrDependencyValidation(t *testing.T) {
	m := NewManager()

	// 测试循环依赖
	_, err := m.CreateProject(CreateProjectRequest{
		Name: "circular",
		Services: map[string]*ServiceConfig{
			"a": {Image: "a", DependsOn: []ServiceDependency{{ServiceName: "b"}}},
			"b": {Image: "b", DependsOn: []ServiceDependency{{ServiceName: "a"}}},
		},
	})
	if err == nil {
		t.Error("Expected error for circular dependency")
	}
}

func TestMgrStartupOrder(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "order-test",
		Services: map[string]*ServiceConfig{
			"db":    {Image: "postgres"},
			"redis": {Image: "redis"},
			"api":   {Image: "node", DependsOn: []ServiceDependency{{ServiceName: "db"}, {ServiceName: "redis"}}},
			"web":   {Image: "nginx", DependsOn: []ServiceDependency{{ServiceName: "api"}}},
		},
	})

	order, err := m.GetStartupOrder(project.ID)
	if err != nil {
		t.Fatalf("get startup order failed: %v", err)
	}
	if order.Total != 4 {
		t.Errorf("Expected 4 services, got %d", order.Total)
	}
	if len(order.Stages) == 0 {
		t.Error("Expected at least 1 stage")
	}
}

func TestMgrHealthReport(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "health-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	report, err := m.GetHealthReport(project.ID)
	if err != nil {
		t.Fatalf("get health report failed: %v", err)
	}
	if report.ProjectID != project.ID {
		t.Errorf("Expected project ID '%s', got '%s'", project.ID, report.ProjectID)
	}
	if len(report.Services) == 0 {
		t.Error("Expected at least 1 service in health report")
	}
}

func TestMgrServiceHealthCheck(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "hc-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	config := &HealthCheckConfig{
		Test:     []string{"CMD", "curl", "-f", "http://localhost/"},
		Interval: 30 * 1000000000, // 30s in nanoseconds
		Timeout:  10 * 1000000000,
		Retries:  3,
	}

	svc, err := m.UpdateServiceHealthCheck(project.ID, "web", config)
	if err != nil {
		t.Fatalf("update health check failed: %v", err)
	}
	if svc.HealthCheck == nil {
		t.Error("Expected health check to be set")
	}
}

func TestMgrServiceResources(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "res-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	limits := &ResourceLimits{
		CPU:    &CPULimit{Cores: 2, Shares: 2048},
		Memory: &MemoryLimit{Limit: 1024 * 1024 * 1024},
	}

	svc, err := m.UpdateServiceResources(project.ID, "web", limits)
	if err != nil {
		t.Fatalf("update resources failed: %v", err)
	}
	if svc.Resources == nil {
		t.Error("Expected resources to be set")
	}
	if svc.Resources.CPU.Cores != 2 {
		t.Errorf("Expected 2 CPU cores, got %.0f", svc.Resources.CPU.Cores)
	}
}

func TestMgrProjectStats(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "stats-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	stats, err := m.GetProjectStats(project.ID)
	if err != nil {
		t.Fatalf("get stats failed: %v", err)
	}
	if stats.TotalServices != 1 {
		t.Errorf("Expected 1 service, got %d", stats.TotalServices)
	}
	if stats.RunningServices != 1 {
		t.Errorf("Expected 1 running service, got %d", stats.RunningServices)
	}
}

func TestMgrAutoScalePolicy(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "autoscale-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", Deploy: &DeployConfig{Replicas: 1}},
		},
	})
	m.StartProject(project.ID)

	policy := &AutoScalePolicy{
		Enabled:     true,
		MinReplicas: 1,
		MaxReplicas: 10,
		Metrics: []ScalingMetric{
			{Type: "cpu", Target: 80.0},
		},
		ScaleUp:   &ScaleRules{StepSize: 1},
		ScaleDown: &ScaleRules{StepSize: 1},
	}

	svc, err := m.UpdateAutoScalePolicy(project.ID, "web", policy)
	if err != nil {
		t.Fatalf("update autoscale failed: %v", err)
	}
	if svc.Deploy == nil || svc.Deploy.AutoScale == nil {
		t.Error("Expected autoscale policy to be set")
	}
	if !svc.Deploy.AutoScale.Enabled {
		t.Error("Expected autoscale to be enabled")
	}
}

func TestMgrEvaluateAutoScale(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "eval-scale",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", Deploy: &DeployConfig{
				Replicas: 1,
				AutoScale: &AutoScalePolicy{
					Enabled:     true,
					MinReplicas: 1,
					MaxReplicas: 5,
					Metrics:     []ScalingMetric{{Type: "cpu", Target: 80.0}},
					ScaleUp:     &ScaleRules{StepSize: 1},
					ScaleDown:   &ScaleRules{StepSize: 1},
				},
			}},
		},
	})
	m.StartProject(project.ID)

	// CPU 超标，应该扩容
	event, err := m.EvaluateAutoScale(project.ID, "web", &ContainerMetrics{
		CPU: CPUMetrics{Percent: 95.0},
		Memory: MemoryMetrics{Percent: 30.0},
	})
	if err != nil {
		t.Fatalf("evaluate autoscale failed: %v", err)
	}
	if event == nil {
		t.Error("Expected scale event for high CPU")
	} else if event.Action != "scale" {
		t.Errorf("Expected scale action, got '%s'", event.Action)
	}
}

func TestMgrAutoScaleEvents(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "events-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)
	m.ScaleService(project.ID, "web", 3)

	events := m.GetAutoScaleEvents(project.ID, 10)
	if len(events) == 0 {
		t.Error("Expected at least 1 scale event")
	}
}

func TestMgrGetServiceLogs(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "logs-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	logs, err := m.GetServiceLogs(project.ID, LogQuery{Tail: 10})
	if err != nil {
		t.Fatalf("get logs failed: %v", err)
	}
	if len(logs) == 0 {
		t.Error("Expected at least 1 log entry")
	}
}

func TestMgrStopManager(t *testing.T) {
	m := NewManager()
	m.Stop() // should not panic
}
