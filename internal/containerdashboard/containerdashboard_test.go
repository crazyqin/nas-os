package containerdashboard

import (
	"testing"
	"time"
)

func TestNewContainerDashboard(t *testing.T) {
	cd := NewContainerDashboard()
	if cd == nil {
		t.Fatal("NewContainerDashboard returned nil")
	}
}

func TestRegisterContainer(t *testing.T) {
	cd := NewContainerDashboard()

	container := &Container{
		ID:    "container1",
		Name:  "nginx",
		Image: "nginx:latest",
	}

	err := cd.RegisterContainer(container)
	if err != nil {
		t.Fatalf("RegisterContainer failed: %v", err)
	}

	if container.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// 测试重复注册
	err = cd.RegisterContainer(container)
	if err == nil {
		t.Error("expected error for duplicate container")
	}
}

func TestGetContainer(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{
		ID:    "container1",
		Name:  "nginx",
		Image: "nginx:latest",
	})

	container, err := cd.GetContainer("container1")
	if err != nil {
		t.Fatalf("GetContainer failed: %v", err)
	}

	if container.Name != "nginx" {
		t.Errorf("expected nginx, got %s", container.Name)
	}

	// 测试不存在的容器
	_, err = cd.GetContainer("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestListContainers(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})
	cd.RegisterContainer(&Container{ID: "c2", Name: "redis", Image: "redis", Status: StatusRunning})
	cd.RegisterContainer(&Container{ID: "c3", Name: "postgres", Image: "postgres", Status: StatusExited})

	// 列出所有容器
	containers := cd.ListContainers("")
	if len(containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(containers))
	}

	// 按状态筛选
	containers = cd.ListContainers(StatusRunning)
	if len(containers) != 2 {
		t.Errorf("expected 2 running containers, got %d", len(containers))
	}
}

func TestStartContainer(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusCreated})

	err := cd.StartContainer("c1")
	if err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	container, _ := cd.GetContainer("c1")
	if container.Status != StatusRunning {
		t.Errorf("expected running, got %v", container.Status)
	}
	if container.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
	if container.Health != HealthStarting {
		t.Errorf("expected starting health, got %v", container.Health)
	}

	// 测试重复启动
	err = cd.StartContainer("c1")
	if err == nil {
		t.Error("expected error for already running container")
	}
}

func TestStopContainer(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})
	cd.StartContainer("c1")

	err := cd.StopContainer("c1")
	if err != nil {
		t.Fatalf("StopContainer failed: %v", err)
	}

	container, _ := cd.GetContainer("c1")
	if container.Status != StatusExited {
		t.Errorf("expected exited, got %v", container.Status)
	}
	if container.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}

	// 测试重复停止
	err = cd.StopContainer("c1")
	if err == nil {
		t.Error("expected error for already stopped container")
	}
}

func TestRestartContainer(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})
	cd.StartContainer("c1")

	err := cd.RestartContainer("c1")
	if err != nil {
		t.Fatalf("RestartContainer failed: %v", err)
	}

	container, _ := cd.GetContainer("c1")
	if container.Status != StatusRunning {
		t.Errorf("expected running, got %v", container.Status)
	}
	if container.RestartCount != 1 {
		t.Errorf("expected restart count 1, got %d", container.RestartCount)
	}
}

func TestDeleteContainer(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})
	cd.StartContainer("c1")

	// 测试非强制删除运行中的容器
	err := cd.DeleteContainer("c1", false)
	if err == nil {
		t.Error("expected error for deleting running container without force")
	}

	// 测试强制删除
	err = cd.DeleteContainer("c1", true)
	if err != nil {
		t.Fatalf("DeleteContainer with force failed: %v", err)
	}

	_, err = cd.GetContainer("c1")
	if err == nil {
		t.Error("expected error for deleted container")
	}
}

func TestResourceUsage(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})

	usage := &ResourceUsage{
		ContainerID:   "c1",
		CPUPercent:    25.5,
		MemoryUsage:   1024 * 1024 * 100,
		MemoryLimit:   1024 * 1024 * 512,
		MemoryPercent: 19.6,
		NetworkRx:     1024,
		NetworkTx:     512,
		BlockRead:     2048,
		BlockWrite:    1024,
		PIDs:          10,
	}

	err := cd.UpdateResourceUsage(usage)
	if err != nil {
		t.Fatalf("UpdateResourceUsage failed: %v", err)
	}

	latest, err := cd.GetResourceUsage("c1")
	if err != nil {
		t.Fatalf("GetResourceUsage failed: %v", err)
	}

	if latest.CPUPercent != 25.5 {
		t.Errorf("expected 25.5, got %f", latest.CPUPercent)
	}

	// 测试不存在的容器
	_, err = cd.GetResourceUsage("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestResourceTimeSeries(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})

	// 添加多个资源使用记录
	for i := 0; i < 10; i++ {
		cd.UpdateResourceUsage(&ResourceUsage{
			ContainerID: "c1",
			CPUPercent:  float64(i * 10),
			MemoryUsage: int64(i * 1024 * 1024),
		})
	}

	// 获取时间序列
	series, err := cd.GetResourceTimeSeries("c1", 1*time.Hour)
	if err != nil {
		t.Fatalf("GetResourceTimeSeries failed: %v", err)
	}

	if len(series.Metrics) != 10 {
		t.Errorf("expected 10 metrics, got %d", len(series.Metrics))
	}
}

func TestContainerLogs(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})

	// 添加日志
	for i := 0; i < 5; i++ {
		cd.AddContainerLog(&ContainerLog{
			ContainerID: "c1",
			Stream:      "stdout",
			Content:     "log entry",
		})
	}

	// 获取日志
	logs, err := cd.GetContainerLogs("c1", 3)
	if err != nil {
		t.Fatalf("GetContainerLogs failed: %v", err)
	}

	if len(logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(logs))
	}

	// 测试不存在的容器
	_, err = cd.GetContainerLogs("nonexistent", 10)
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestLogStream(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})

	// 创建日志流
	stream, err := cd.StreamLogs("c1")
	if err != nil {
		t.Fatalf("StreamLogs failed: %v", err)
	}

	// 添加日志
	go func() {
		time.Sleep(10 * time.Millisecond)
		cd.AddContainerLog(&ContainerLog{
			ContainerID: "c1",
			Stream:      "stdout",
			Content:     "stream log",
		})
	}()

	// 读取日志
	select {
	case log := <-stream.Logs:
		if log.Content != "stream log" {
			t.Errorf("expected 'stream log', got %s", log.Content)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for log")
	}

	// 停止日志流
	cd.StopLogStream("c1")
}

func TestDeployTemplates(t *testing.T) {
	cd := NewContainerDashboard()

	template := &DeployTemplate{
		ID:          "nginx-template",
		Name:        "Nginx Web Server",
		Description: "High-performance HTTP server",
		Category:    "web",
		Image:       "nginx:latest",
		Ports: []PortMapping{
			{HostPort: 80, ContainerPort: 80, Protocol: "tcp"},
		},
		Env: map[string]string{
			"NGINX_HOST": "localhost",
		},
	}

	// 注册模板
	err := cd.RegisterTemplate(template)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	// 重复注册
	err = cd.RegisterTemplate(template)
	if err == nil {
		t.Error("expected error for duplicate template")
	}

	// 获取模板
	tmpl, err := cd.GetTemplate("nginx-template")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if tmpl.Name != "Nginx Web Server" {
		t.Errorf("expected Nginx Web Server, got %s", tmpl.Name)
	}

	// 列出模板
	templates := cd.ListTemplates("web")
	if len(templates) != 1 {
		t.Errorf("expected 1 template, got %d", len(templates))
	}

	// 从模板部署
	container, err := cd.DeployFromTemplate("nginx-template", "my-nginx", map[string]string{
		"NGINX_HOST": "example.com",
	})
	if err != nil {
		t.Fatalf("DeployFromTemplate failed: %v", err)
	}

	if container.Name != "my-nginx" {
		t.Errorf("expected my-nginx, got %s", container.Name)
	}
	if container.Image != "nginx:latest" {
		t.Errorf("expected nginx:latest, got %s", container.Image)
	}
	if container.Status != StatusCreated {
		t.Errorf("expected created, got %v", container.Status)
	}
}

func TestContainerHealth(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})
	cd.StartContainer("c1")

	// 无资源数据时
	health, err := cd.CheckContainerHealth("c1")
	if err != nil {
		t.Fatalf("CheckContainerHealth failed: %v", err)
	}
	if *health != HealthStarting {
		t.Errorf("expected starting, got %v", *health)
	}

	// 添加正常资源使用
	cd.UpdateResourceUsage(&ResourceUsage{
		ContainerID:   "c1",
		CPUPercent:    50,
		MemoryPercent: 60,
	})

	health, err = cd.CheckContainerHealth("c1")
	if err != nil {
		t.Fatalf("CheckContainerHealth failed: %v", err)
	}
	if *health != HealthHealthy {
		t.Errorf("expected healthy, got %v", *health)
	}

	// 添加高CPU使用
	cd.UpdateResourceUsage(&ResourceUsage{
		ContainerID:   "c1",
		CPUPercent:    98,
		MemoryPercent: 60,
	})

	health, err = cd.CheckContainerHealth("c1")
	if err != nil {
		t.Fatalf("CheckContainerHealth failed: %v", err)
	}
	if *health != HealthUnhealthy {
		t.Errorf("expected unhealthy, got %v", *health)
	}

	// 更新健康状态
	err = cd.UpdateContainerHealth("c1", HealthHealthy)
	if err != nil {
		t.Fatalf("UpdateContainerHealth failed: %v", err)
	}

	container, _ := cd.GetContainer("c1")
	if container.Health != HealthHealthy {
		t.Errorf("expected healthy, got %v", container.Health)
	}
}

func TestNetworkTopology(t *testing.T) {
	cd := NewContainerDashboard()

	// 注册容器
	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})
	cd.RegisterContainer(&Container{ID: "c2", Name: "redis", Image: "redis", Status: StatusRunning})

	// 注册网络
	cd.RegisterNetwork(&NetworkInfo{
		ID:     "net1",
		Name:   "app-network",
		Driver: "bridge",
		Subnet: "172.17.0.0/16",
	})

	// 连接容器到网络
	err := cd.ConnectContainerToNetwork("c1", "net1")
	if err != nil {
		t.Fatalf("ConnectContainerToNetwork failed: %v", err)
	}

	err = cd.ConnectContainerToNetwork("c2", "net1")
	if err != nil {
		t.Fatalf("ConnectContainerToNetwork failed: %v", err)
	}

	// 获取网络拓扑
	topology := cd.GetNetworkTopology()
	if len(topology.Networks) != 1 {
		t.Errorf("expected 1 network, got %d", len(topology.Networks))
	}
	if len(topology.Nodes) != 3 { // 1 network + 2 containers
		t.Errorf("expected 3 nodes, got %d", len(topology.Nodes))
	}
	if len(topology.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(topology.Edges))
	}

	// 断开连接
	err = cd.DisconnectContainerFromNetwork("c1", "net1")
	if err != nil {
		t.Fatalf("DisconnectContainerFromNetwork failed: %v", err)
	}

	topology = cd.GetNetworkTopology()
	if len(topology.Edges) != 1 {
		t.Errorf("expected 1 edge after disconnect, got %d", len(topology.Edges))
	}
}

func TestDashboardStats(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})
	cd.RegisterContainer(&Container{ID: "c2", Name: "redis", Image: "redis", Status: StatusRunning})
	cd.RegisterContainer(&Container{ID: "c3", Name: "postgres", Image: "postgres", Status: StatusExited})

	cd.StartContainer("c1")
	cd.StartContainer("c2")

	cd.UpdateResourceUsage(&ResourceUsage{
		ContainerID: "c1",
		CPUPercent:  30,
		MemoryUsage: 1024 * 1024 * 100,
	})

	cd.UpdateResourceUsage(&ResourceUsage{
		ContainerID: "c2",
		CPUPercent:  20,
		MemoryUsage: 1024 * 1024 * 50,
	})

	stats := cd.GetDashboardStats()
	if stats.TotalContainers != 3 {
		t.Errorf("expected 3 total, got %d", stats.TotalContainers)
	}
	if stats.RunningContainers != 2 {
		t.Errorf("expected 2 running, got %d", stats.RunningContainers)
	}
	if stats.StoppedContainers != 1 {
		t.Errorf("expected 1 stopped, got %d", stats.StoppedContainers)
	}
	if stats.CPUUsageTotal != 50 {
		t.Errorf("expected 50 total CPU, got %f", stats.CPUUsageTotal)
	}
}

func TestContainerStats(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})

	cd.UpdateResourceUsage(&ResourceUsage{
		ContainerID:   "c1",
		CPUPercent:    45,
		MemoryUsage:   1024 * 1024 * 200,
		MemoryPercent: 39,
		NetworkRx:     2048,
		NetworkTx:     1024,
		PIDs:          15,
	})

	cd.AddContainerLog(&ContainerLog{
		ContainerID: "c1",
		Stream:      "stdout",
		Content:     "test log",
	})

	stats, err := cd.GetContainerStats("c1")
	if err != nil {
		t.Fatalf("GetContainerStats failed: %v", err)
	}

	if stats["current_cpu"] != float64(45) {
		t.Errorf("expected 45, got %v", stats["current_cpu"])
	}
	if stats["log_count"] != 1 {
		t.Errorf("expected 1 log, got %v", stats["log_count"])
	}
}

func TestPruneStoppedContainers(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx", Status: StatusRunning})
	cd.RegisterContainer(&Container{ID: "c2", Name: "redis", Image: "redis", Status: StatusExited})
	cd.RegisterContainer(&Container{ID: "c3", Name: "postgres", Image: "postgres", Status: StatusDead})

	count := cd.PruneStoppedContainers()
	if count != 2 {
		t.Errorf("expected 2 pruned, got %d", count)
	}

	containers := cd.ListContainers("")
	if len(containers) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(containers))
	}
}

func TestUnregisterContainer(t *testing.T) {
	cd := NewContainerDashboard()

	cd.RegisterContainer(&Container{ID: "c1", Name: "nginx", Image: "nginx"})

	err := cd.UnregisterContainer("c1")
	if err != nil {
		t.Fatalf("UnregisterContainer failed: %v", err)
	}

	_, err = cd.GetContainer("c1")
	if err == nil {
		t.Error("expected error for unregistered container")
	}

	// 测试注销不存在的容器
	err = cd.UnregisterContainer("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}
