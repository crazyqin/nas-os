// Package containerorch 测试
package containerorch

import (
	"testing"
	"time"
)

func TestCreateProject(t *testing.T) {
	m := NewManager()

	project, err := m.CreateProject(CreateProjectRequest{
		Name:        "test-project",
		Description: "测试项目",
		Namespace:   "default",
		Services: map[string]*ServiceConfig{
			"web": {
				Image: "nginx",
				Tag:   "latest",
				Ports: []PortMapping{
					{HostPort: 8080, ContainerPort: 80},
				},
			},
		},
	})

	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}
	if project == nil {
		t.Fatal("项目不应为 nil")
	}
	if project.Name != "test-project" {
		t.Errorf("项目名称不匹配: %s", project.Name)
	}
	if project.Status != ProjectStatusCreating {
		t.Errorf("项目状态不匹配: %s", project.Status)
	}
}

func TestGetProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	got, err := m.GetProject(project.ID)
	if err != nil {
		t.Fatalf("获取项目失败: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("项目名称不匹配")
	}
}

func TestGetProjectNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetProject("non-existent")
	if err == nil {
		t.Fatal("应该返回错误")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("错误类型不匹配: %v", err)
	}
}

func TestListProjects(t *testing.T) {
	m := NewManager()

	m.CreateProject(CreateProjectRequest{
		Name:      "project1",
		Namespace: "ns1",
		Services:  map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})
	m.CreateProject(CreateProjectRequest{
		Name:      "project2",
		Namespace: "ns2",
		Services:  map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})
	m.CreateProject(CreateProjectRequest{
		Name:      "project3",
		Namespace: "ns1",
		Services:  map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})

	// 列出所有项目
	all := m.ListProjects("")
	if len(all) != 3 {
		t.Errorf("期望3个项目，实际 %d", len(all))
	}

	// 按命名空间过滤
	ns1 := m.ListProjects("ns1")
	if len(ns1) != 2 {
		t.Errorf("期望2个 ns1 项目，实际 %d", len(ns1))
	}
}

func TestUpdateProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "old-name",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	newName := "new-name"
	updated, err := m.UpdateProject(project.ID, UpdateProjectRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "new-name" {
		t.Errorf("项目名称未更新: %s", updated.Name)
	}
	if updated.Status != ProjectStatusUpdating {
		t.Errorf("项目状态应为 updating: %s", updated.Status)
	}
}

func TestDeleteProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "to-delete",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	err := m.DeleteProject(project.ID, false)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	_, err = m.GetProject(project.ID)
	if err == nil {
		t.Error("已删除项目不应存在")
	}
}

func TestDeleteRunningProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "running",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	// 启动项目
	m.StartProject(project.ID)

	// 不强制删除应失败
	err := m.DeleteProject(project.ID, false)
	if err == nil {
		t.Error("删除运行中项目应失败")
	}

	// 强制删除应成功
	err = m.DeleteProject(project.ID, true)
	if err != nil {
		t.Fatalf("强制删除失败: %v", err)
	}
}

func TestStartProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web":  {Image: "nginx", DesiredCount: 2},
			"db":   {Image: "postgres"},
			"redis": {Image: "redis"},
		},
	})

	started, err := m.StartProject(project.ID)
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if started.Status != ProjectStatusRunning {
		t.Errorf("项目状态不匹配: %s", started.Status)
	}

	// 检查服务状态
	for name, svc := range started.Services {
		if svc.Status != ServiceStatusRunning {
			t.Errorf("服务 %s 状态不匹配: %s", name, svc.Status)
		}
		if len(svc.ContainerIDs) != svc.DesiredCount {
			t.Errorf("服务 %s 容器数量不匹配: %d != %d", name, len(svc.ContainerIDs), svc.DesiredCount)
		}
	}
}

func TestStopProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	// 先启动
	m.StartProject(project.ID)

	// 停止
	stopped, err := m.StopProject(project.ID)
	if err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	if stopped.Status != ProjectStatusStopped {
		t.Errorf("项目状态不匹配: %s", stopped.Status)
	}

	// 检查服务状态
	for name, svc := range stopped.Services {
		if svc.Status != ServiceStatusStopped {
			t.Errorf("服务 %s 状态不匹配: %s", name, svc.Status)
		}
	}
}

func TestRestartProject(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	restarted, err := m.RestartProject(project.ID)
	if err != nil {
		t.Fatalf("重启失败: %v", err)
	}
	if restarted.Status != ProjectStatusRunning {
		t.Errorf("项目状态不匹配: %s", restarted.Status)
	}
}

func TestScaleService(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", DesiredCount: 1},
		},
	})

	m.StartProject(project.ID)

	// 扩容到 3
	svc, err := m.ScaleService(project.ID, "web", 3)
	if err != nil {
		t.Fatalf("扩容失败: %v", err)
	}
	if svc.Instances != 3 {
		t.Errorf("实例数不匹配: %d", svc.Instances)
	}
	if len(svc.ContainerIDs) != 3 {
		t.Errorf("容器 ID 数量不匹配: %d", len(svc.ContainerIDs))
	}

	// 缩容到 1
	svc, err = m.ScaleService(project.ID, "web", 1)
	if err != nil {
		t.Fatalf("缩容失败: %v", err)
	}
	if svc.Instances != 1 {
		t.Errorf("实例数不匹配: %d", svc.Instances)
	}
}

func TestScaleServiceInvalid(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	// 负数应失败
	_, err := m.ScaleService(project.ID, "web", -1)
	if err == nil {
		t.Error("负数副本应失败")
	}

	// 超过最大限制应失败
	_, err = m.ScaleService(project.ID, "web", MaxInstances+1)
	if err == nil {
		t.Error("超过最大实例数应失败")
	}
}

func TestDependencies(t *testing.T) {
	m := NewManager()

	// 创建有依赖关系的项目
	project, err := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {
				Image: "nginx",
				DependsOn: []ServiceDependency{
					{ServiceName: "api", Condition: "service_healthy"},
				},
			},
			"api": {
				Image: "node",
				DependsOn: []ServiceDependency{
					{ServiceName: "db", Condition: "service_healthy"},
				},
			},
			"db": {
				Image: "postgres",
			},
		},
	})

	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	// 获取启动顺序
	order, err := m.GetStartupOrder(project.ID)
	if err != nil {
		t.Fatalf("获取启动顺序失败: %v", err)
	}

	if len(order.Stages) == 0 {
		t.Fatal("启动阶段不应为空")
	}

	// db 应该在第一个阶段
	firstStage := order.Stages[0]
	dbFound := false
	for _, name := range firstStage {
		if name == "db" {
			dbFound = true
			break
		}
	}
	if !dbFound {
		t.Error("db 应该在第一个启动阶段")
	}
}

func TestCircularDependency(t *testing.T) {
	m := NewManager()

	// 创建循环依赖
	_, err := m.CreateProject(CreateProjectRequest{
		Name: "circular",
		Services: map[string]*ServiceConfig{
			"a": {
				Image: "a",
				DependsOn: []ServiceDependency{
					{ServiceName: "b"},
				},
			},
			"b": {
				Image: "b",
				DependsOn: []ServiceDependency{
					{ServiceName: "a"},
				},
			},
		},
	})

	if err == nil {
		t.Fatal("循环依赖应返回错误")
	}
	if _, ok := err.(*DependencyError); !ok {
		t.Errorf("错误类型不匹配: %v", err)
	}
}

func TestHealthReport(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	m.StartProject(project.ID)

	report, err := m.GetHealthReport(project.ID)
	if err != nil {
		t.Fatalf("获取健康报告失败: %v", err)
	}

	if report == nil {
		t.Fatal("健康报告不应为 nil")
	}
	if report.ProjectID != project.ID {
		t.Errorf("项目 ID 不匹配")
	}
	if len(report.Services) == 0 {
		t.Error("服务健康状态不应为空")
	}
}

func TestUpdateHealthCheck(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	config := &HealthCheckConfig{
		Test:     []string{"CMD", "curl", "-f", "http://localhost/health"},
		Interval: 30 * time.Second,
		Timeout:  5 * time.Second,
		Retries:  3,
	}

	svc, err := m.UpdateServiceHealthCheck(project.ID, "web", config)
	if err != nil {
		t.Fatalf("更新健康检查失败: %v", err)
	}
	if svc.HealthCheck == nil {
		t.Fatal("健康检查配置不应为 nil")
	}
	if svc.HealthCheck.Interval != 30*time.Second {
		t.Errorf("间隔不匹配: %v", svc.HealthCheck.Interval)
	}
}

func TestUpdateResources(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	limits := &ResourceLimits{
		CPU: &CPULimit{
			Cores:  2,
			Shares: 2048,
		},
		Memory: &MemoryLimit{
			Limit:       1024 * 1024 * 1024, // 1GB
			Reservation: 512 * 1024 * 1024,  // 512MB
		},
	}

	svc, err := m.UpdateServiceResources(project.ID, "web", limits)
	if err != nil {
		t.Fatalf("更新资源限制失败: %v", err)
	}
	if svc.Resources.CPU.Cores != 2 {
		t.Errorf("CPU 核数不匹配: %f", svc.Resources.CPU.Cores)
	}
}

func TestAutoScalePolicy(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", DesiredCount: 1},
		},
	})

	policy := &AutoScalePolicy{
		Enabled:     true,
		MinReplicas: 1,
		MaxReplicas: 10,
		Metrics: []ScalingMetric{
			{Type: "cpu", Target: 70.0},
		},
		Cooldown: 5 * time.Minute,
		ScaleUp: &ScaleRules{
			StepSize: 2,
		},
		ScaleDown: &ScaleRules{
			StepSize: 1,
		},
	}

	svc, err := m.UpdateAutoScalePolicy(project.ID, "web", policy)
	if err != nil {
		t.Fatalf("更新自动扩缩容策略失败: %v", err)
	}
	if svc.Deploy.AutoScale == nil {
		t.Fatal("自动扩缩容策略不应为 nil")
	}
	if !svc.Deploy.AutoScale.Enabled {
		t.Error("自动扩缩容应启用")
	}
}

func TestEvaluateAutoScale(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {
				Image:        "nginx",
				DesiredCount: 2,
				Instances:    2,
				Deploy: &DeployConfig{
					Replicas: 2,
					AutoScale: &AutoScalePolicy{
						Enabled:     true,
						MinReplicas: 1,
						MaxReplicas: 10,
						Metrics: []ScalingMetric{
							{Type: "cpu", Target: 70.0},
						},
						ScaleUp: &ScaleRules{StepSize: 2},
						ScaleDown: &ScaleRules{StepSize: 1},
					},
				},
			},
		},
	})

	m.StartProject(project.ID)

	// 高 CPU 负载应触发扩容
	metrics := &ContainerMetrics{
		ServiceName: "web",
		CPU: CPUMetrics{
			Percent: 90.0,
		},
		Memory: MemoryMetrics{
			Percent: 50.0,
		},
	}

	event, err := m.EvaluateAutoScale(project.ID, "web", metrics)
	if err != nil {
		t.Fatalf("评估自动扩缩容失败: %v", err)
	}
	if event == nil {
		t.Fatal("应触发扩缩容事件")
	}
	if event.Action != "scale" {
		t.Errorf("动作不匹配: %s", event.Action)
	}
	if event.To <= event.From {
		t.Errorf("应该是扩容: %d -> %d", event.From, event.To)
	}
}

func TestAutoScaleEvents(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", DesiredCount: 1, Instances: 1},
		},
	})

	m.StartProject(project.ID)

	// 手动扩容
	m.ScaleService(project.ID, "web", 3)
	m.ScaleService(project.ID, "web", 5)

	events := m.GetAutoScaleEvents(project.ID, 0)
	if len(events) < 2 {
		t.Errorf("应有至少2个事件，实际 %d", len(events))
	}

	// 带限制
	events = m.GetAutoScaleEvents(project.ID, 1)
	if len(events) != 1 {
		t.Errorf("应有1个事件，实际 %d", len(events))
	}
}

func TestGetServiceLogs(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	m.StartProject(project.ID)

	entries, err := m.GetServiceLogs(project.ID, LogQuery{})
	if err != nil {
		t.Fatalf("获取日志失败: %v", err)
	}
	if len(entries) == 0 {
		t.Error("日志不应为空")
	}
}

func TestGetServiceLogsWithTail(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	m.StartProject(project.ID)

	entries, err := m.GetServiceLogs(project.ID, LogQuery{Tail: 1})
	if err != nil {
		t.Fatalf("获取日志失败: %v", err)
	}
	if len(entries) > 1 {
		t.Errorf("tail=1 应返回最多1条日志，实际 %d", len(entries))
	}
}

func TestGetProjectStats(t *testing.T) {
	m := NewManager()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {
				Image:        "nginx",
				DesiredCount: 2,
				Resources: &ResourceLimits{
					CPU:    &CPULimit{Cores: 1},
					Memory: &MemoryLimit{Limit: 512 * 1024 * 1024},
				},
			},
			"db": {
				Image:        "postgres",
				DesiredCount: 1,
				Resources: &ResourceLimits{
					CPU:    &CPULimit{Cores: 2},
					Memory: &MemoryLimit{Limit: 1024 * 1024 * 1024},
				},
			},
		},
	})

	m.StartProject(project.ID)

	stats, err := m.GetProjectStats(project.ID)
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}
	if stats.TotalServices != 2 {
		t.Errorf("服务数量不匹配: %d", stats.TotalServices)
	}
	if stats.TotalCPU != 4 {
		t.Errorf("总 CPU 不匹配: %f", stats.TotalCPU)
	}
}

func TestIsValidServiceName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"web", true},
		{"web-server", true},
		{"api1", true},
		{"123", true},
		{"-web", false},
		{"web-", false},
		{"Web", false},
		{"web_server", false},
		{"", false},
		{"a-very-long-service-name-that-exceeds-sixty-three-characters-limit-ok", false},
	}

	for _, tt := range tests {
		result := isValidServiceName(tt.name)
		if result != tt.valid {
			t.Errorf("isValidServiceName(%q) = %v, want %v", tt.name, result, tt.valid)
		}
	}
}

func TestManagerStop(t *testing.T) {
	m := NewManager()

	// 创建并启动项目
	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	// 停止管理器（不应 panic）
	m.Stop()
}

// ========== Handlers 测试 ==========

func setupHandlers() (*Handlers, *Manager) {
	m := NewManager()
	h := NewHandlers(m)
	return h, m
}

func TestHandlersCreateProject(t *testing.T) {
	h, m := setupHandlers()

	req := CreateProjectRequest{
		Name: "test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	}

	project, err := m.CreateProject(req)
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}
	if h.manager == nil {
		t.Error("manager 不应为 nil")
	}
	_ = project
}

func TestHandlersGetProject(t *testing.T) {
	h, _ := setupHandlers()

	// 测试获取不存在的项目
	_, err := h.manager.GetProject("non-existent")
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestHandlersListProjects(t *testing.T) {
	_, m := setupHandlers()

	m.CreateProject(CreateProjectRequest{
		Name: "project1",
		Services: map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})
	m.CreateProject(CreateProjectRequest{
		Name: "project2",
		Namespace: "ns1",
		Services: map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})

	projects := m.ListProjects("")
	if len(projects) != 2 {
		t.Errorf("期望2个项目，实际 %d", len(projects))
	}

	projects = m.ListProjects("ns1")
	if len(projects) != 1 {
		t.Errorf("期望1个 ns1 项目，实际 %d", len(projects))
	}
}

func TestHandlersUpdateProject(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "old",
		Services: map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})

	newName := "new"
	updated, err := m.UpdateProject(project.ID, UpdateProjectRequest{Name: &newName})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "new" {
		t.Errorf("名称未更新: %s", updated.Name)
	}
}

func TestHandlersDeleteProject(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "to-delete",
		Services: map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})

	err := m.DeleteProject(project.ID, false)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
}

func TestHandlersStartStopProject(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "lifecycle",
		Services: map[string]*ServiceConfig{"web": {Image: "nginx"}},
	})

	// 启动
	started, err := m.StartProject(project.ID)
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if started.Status != ProjectStatusRunning {
		t.Errorf("状态不匹配: %s", started.Status)
	}

	// 停止
	stopped, err := m.StopProject(project.ID)
	if err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	if stopped.Status != ProjectStatusStopped {
		t.Errorf("状态不匹配: %s", stopped.Status)
	}
}

func TestHandlersScaleService(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "scale-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", DesiredCount: 1},
		},
	})
	m.StartProject(project.ID)

	// 扩容
	svc, err := m.ScaleService(project.ID, "web", 5)
	if err != nil {
		t.Fatalf("扩容失败: %v", err)
	}
	if svc.Instances != 5 {
		t.Errorf("实例数不匹配: %d", svc.Instances)
	}
}

func TestHandlersGetStartupOrder(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "order-test",
		Services: map[string]*ServiceConfig{
			"web": {
				Image: "nginx",
				DependsOn: []ServiceDependency{
					{ServiceName: "api"},
				},
			},
			"api": {Image: "node"},
		},
	})

	order, err := m.GetStartupOrder(project.ID)
	if err != nil {
		t.Fatalf("获取启动顺序失败: %v", err)
	}
	if len(order.Stages) == 0 {
		t.Error("启动阶段不应为空")
	}
}

func TestHandlersHealthReport(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "health-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	report, err := m.GetHealthReport(project.ID)
	if err != nil {
		t.Fatalf("获取健康报告失败: %v", err)
	}
	if report == nil {
		t.Fatal("健康报告不应为 nil")
	}
}

func TestHandlersAutoScale(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "autoscale-test",
		Services: map[string]*ServiceConfig{
			"web": {
				Image:        "nginx",
				DesiredCount: 2,
				Instances:    2,
				Deploy: &DeployConfig{
					Replicas: 2,
					AutoScale: &AutoScalePolicy{
						Enabled:     true,
						MinReplicas: 1,
						MaxReplicas: 10,
						Metrics: []ScalingMetric{
							{Type: "cpu", Target: 70.0},
						},
						ScaleUp:   &ScaleRules{StepSize: 2},
						ScaleDown: &ScaleRules{StepSize: 1},
					},
				},
			},
		},
	})
	m.StartProject(project.ID)

	// 更新自动扩缩容策略
	policy := &AutoScalePolicy{
		Enabled:     true,
		MinReplicas: 1,
		MaxReplicas: 5,
		Metrics: []ScalingMetric{
			{Type: "cpu", Target: 80.0},
		},
	}

	svc, err := m.UpdateAutoScalePolicy(project.ID, "web", policy)
	if err != nil {
		t.Fatalf("更新自动扩缩容策略失败: %v", err)
	}
	if svc.Deploy.AutoScale.MaxReplicas != 5 {
		t.Errorf("最大副本数不匹配: %d", svc.Deploy.AutoScale.MaxReplicas)
	}
}

func TestHandlersServiceLogs(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "logs-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	entries, err := m.GetServiceLogs(project.ID, LogQuery{})
	if err != nil {
		t.Fatalf("获取日志失败: %v", err)
	}
	if len(entries) == 0 {
		t.Error("日志不应为空")
	}
}

func TestHandlersServiceLogsNotFound(t *testing.T) {
	_, m := setupHandlers()

	_, err := m.GetServiceLogs("non-existent", LogQuery{})
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestHandlersScaleServiceNotFound(t *testing.T) {
	_, m := setupHandlers()

	_, err := m.ScaleService("non-existent", "web", 1)
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestHandlersUpdateHealthCheckNotFound(t *testing.T) {
	_, m := setupHandlers()

	_, err := m.UpdateServiceHealthCheck("non-existent", "web", &HealthCheckConfig{})
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestHandlersUpdateResourcesNotFound(t *testing.T) {
	_, m := setupHandlers()

	_, err := m.UpdateServiceResources("non-existent", "web", &ResourceLimits{})
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestHandlersUpdateAutoScaleNotFound(t *testing.T) {
	_, m := setupHandlers()

	_, err := m.UpdateAutoScalePolicy("non-existent", "web", &AutoScalePolicy{})
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestHandlersEvaluateAutoScaleNotFound(t *testing.T) {
	_, m := setupHandlers()

	_, err := m.EvaluateAutoScale("non-existent", "web", &ContainerMetrics{})
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestHandlersGetAutoScaleEvents(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "events-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", DesiredCount: 1, Instances: 1},
		},
	})
	m.StartProject(project.ID)

	// 执行一些扩缩容操作
	m.ScaleService(project.ID, "web", 3)
	m.ScaleService(project.ID, "web", 5)

	events := m.GetAutoScaleEvents(project.ID, 10)
	if len(events) < 2 {
		t.Errorf("应有至少2个事件，实际 %d", len(events))
	}
}

func TestHandlersGetProjectStats(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "stats-test",
		Services: map[string]*ServiceConfig{
			"web": {
				Image:        "nginx",
				DesiredCount: 2,
				Resources: &ResourceLimits{
					CPU:    &CPULimit{Cores: 1},
					Memory: &MemoryLimit{Limit: 256 * 1024 * 1024},
				},
			},
		},
	})
	m.StartProject(project.ID)

	stats, err := m.GetProjectStats(project.ID)
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}
	if stats.TotalServices != 1 {
		t.Errorf("服务数量不匹配: %d", stats.TotalServices)
	}
}

func TestHandlersStreamServiceLogs(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "stream-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	stream, err := m.StreamServiceLogs(project.ID, LogQuery{Follow: true})
	if err != nil {
		t.Fatalf("流式日志失败: %v", err)
	}
	if stream == nil {
		t.Fatal("日志流不应为 nil")
	}
	stream.Close()
}

func TestHandlersUpdateProjectServices(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "update-services",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	newServices := map[string]*ServiceConfig{
		"web": {Image: "nginx:latest"},
		"api": {Image: "node:18"},
	}

	updated, err := m.UpdateProject(project.ID, UpdateProjectRequest{Services: newServices})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if len(updated.Services) != 2 {
		t.Errorf("服务数量不匹配: %d", len(updated.Services))
	}
}

func TestHandlersRestartProject(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "restart-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	restarted, err := m.RestartProject(project.ID)
	if err != nil {
		t.Fatalf("重启失败: %v", err)
	}
	if restarted.Status != ProjectStatusRunning {
		t.Errorf("状态不匹配: %s", restarted.Status)
	}
}

func TestHandlersScaleServiceNegative(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "negative-scale",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	_, err := m.ScaleService(project.ID, "web", -5)
	if err == nil {
		t.Error("负数副本应失败")
	}
}

func TestHandlersScaleServiceOverMax(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "over-max",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	_, err := m.ScaleService(project.ID, "web", 200)
	if err == nil {
		t.Error("超过最大实例数应失败")
	}
}

func TestHandlersDependencyValidation(t *testing.T) {
	_, m := setupHandlers()

	// 测试循环依赖
	_, err := m.CreateProject(CreateProjectRequest{
		Name: "circular",
		Services: map[string]*ServiceConfig{
			"a": {Image: "a", DependsOn: []ServiceDependency{{ServiceName: "b"}}},
			"b": {Image: "b", DependsOn: []ServiceDependency{{ServiceName: "a"}}},
		},
	})
	if err == nil {
		t.Error("循环依赖应返回错误")
	}
}

func TestHandlersInvalidServiceName(t *testing.T) {
	_, m := setupHandlers()

	_, err := m.CreateProject(CreateProjectRequest{
		Name: "invalid",
		Services: map[string]*ServiceConfig{
			"Invalid_Name": {Image: "nginx"},
		},
	})
	if err == nil {
		t.Error("无效服务名应返回错误")
	}
}

func TestHandlersDeleteRunningProjectForce(t *testing.T) {
	_, m := setupHandlers()

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "force-delete",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	// 不强制删除应失败
	err := m.DeleteProject(project.ID, false)
	if err == nil {
		t.Error("删除运行中项目应失败")
	}

	// 强制删除应成功
	err = m.DeleteProject(project.ID, true)
	if err != nil {
		t.Fatalf("强制删除失败: %v", err)
	}
}
