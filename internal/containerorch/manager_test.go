package containerorch

import (
	"testing"
)

func TestNewContainerOrchManager(t *testing.T) {
	manager := NewContainerOrchManager(nil)
	if manager == nil {
		t.Fatal("Expected manager")
	}
	if manager.config.MaxContainers != 100 {
		t.Errorf("Expected max containers 100, got %d", manager.config.MaxContainers)
	}
}

func TestCreateContainer(t *testing.T) {
	manager := NewContainerOrchManager(nil)

	ctr := &Container{
		Name:  "test-nginx",
		Image: "nginx:latest",
		Ports: []PortMapping{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
	}

	created, err := manager.CreateContainer(ctr)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	if created.ID == "" {
		t.Error("Expected ID to be set")
	}
	if created.Status != StatusCreated {
		t.Errorf("Expected status 'created', got '%s'", created.Status)
	}

	// 测试空名称
	_, err = manager.CreateContainer(&Container{Image: "nginx"})
	if err == nil {
		t.Error("Expected error for empty name")
	}

	// 测试空镜像
	_, err = manager.CreateContainer(&Container{Name: "test"})
	if err == nil {
		t.Error("Expected error for empty image")
	}

	// 测试重复名称
	_, err = manager.CreateContainer(&Container{Name: "test-nginx", Image: "nginx"})
	if err == nil {
		t.Error("Expected error for duplicate name")
	}
}

func TestContainerLifecycle(t *testing.T) {
	manager := NewContainerOrchManager(nil)

	ctr, _ := manager.CreateContainer(&Container{Name: "test", Image: "nginx"})

	// 启动
	err := manager.StartContainer(ctr.ID)
	if err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	fetched, _ := manager.GetContainer(ctr.ID)
	if fetched.Status != StatusRunning {
		t.Errorf("Expected running, got '%s'", fetched.Status)
	}

	// 重复启动
	err = manager.StartContainer(ctr.ID)
	if err == nil {
		t.Error("Expected error for starting running container")
	}

	// 停止
	err = manager.StopContainer(ctr.ID)
	if err != nil {
		t.Fatalf("Failed to stop: %v", err)
	}

	// 重启
	err = manager.RestartContainer(ctr.ID)
	if err != nil {
		t.Fatalf("Failed to restart: %v", err)
	}

	fetched, _ = manager.GetContainer(ctr.ID)
	if fetched.RestartCount != 1 {
		t.Errorf("Expected restart count 1, got %d", fetched.RestartCount)
	}
}

func TestRemoveContainer(t *testing.T) {
	manager := NewContainerOrchManager(nil)

	ctr, _ := manager.CreateContainer(&Container{Name: "test", Image: "nginx"})
	manager.StartContainer(ctr.ID)

	// 非强制删除运行中的容器
	err := manager.RemoveContainer(ctr.ID, false)
	if err == nil {
		t.Error("Expected error for removing running container without force")
	}

	// 强制删除
	err = manager.RemoveContainer(ctr.ID, true)
	if err != nil {
		t.Fatalf("Failed to force remove: %v", err)
	}

	_, err = manager.GetContainer(ctr.ID)
	if err == nil {
		t.Error("Expected error for removed container")
	}
}

func TestListContainers(t *testing.T) {
	manager := NewContainerOrchManager(nil)

	manager.CreateContainer(&Container{Name: "c1", Image: "nginx"})
	manager.CreateContainer(&Container{Name: "c2", Image: "redis"})

	ctrs := manager.ListContainers()
	if len(ctrs) != 2 {
		t.Errorf("Expected 2, got %d", len(ctrs))
	}
}

func TestNetworkOperations(t *testing.T) {
	manager := NewContainerOrchManager(nil)

	net, err := manager.CreateNetwork(&Network{
		Name:   "test-net",
		Driver: "bridge",
		Subnet: "172.20.0.0/16",
	})
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	if net.ID == "" {
		t.Error("Expected ID to be set")
	}

	nets := manager.ListNetworks()
	if len(nets) != 1 {
		t.Errorf("Expected 1 network, got %d", len(nets))
	}

	err = manager.RemoveNetwork(net.ID)
	if err != nil {
		t.Fatalf("Failed to remove network: %v", err)
	}
}

func TestVolumeOperations(t *testing.T) {
	manager := NewContainerOrchManager(nil)

	vol, err := manager.CreateVolume(&Volume{
		Name:   "test-vol",
		Driver: "local",
	})
	if err != nil {
		t.Fatalf("Failed to create volume: %v", err)
	}

	if vol.ID == "" {
		t.Error("Expected ID to be set")
	}

	vols := manager.ListVolumes()
	if len(vols) != 1 {
		t.Errorf("Expected 1 volume, got %d", len(vols))
	}

	err = manager.RemoveVolume(vol.ID)
	if err != nil {
		t.Fatalf("Failed to remove volume: %v", err)
	}
}

func TestStackOperations(t *testing.T) {
	manager := NewContainerOrchManager(nil)

	stack, err := manager.DeployStack(&Stack{
		Name: "test-stack",
		Services: []StackService{
			{Name: "web", Image: "nginx", Replicas: 2},
			{Name: "api", Image: "node:18", Replicas: 1},
		},
	})
	if err != nil {
		t.Fatalf("Failed to deploy stack: %v", err)
	}

	if stack.ID == "" {
		t.Error("Expected ID to be set")
	}

	stacks := manager.ListStacks()
	if len(stacks) != 1 {
		t.Errorf("Expected 1 stack, got %d", len(stacks))
	}
}

func TestGetStats(t *testing.T) {
	manager := NewContainerOrchManager(nil)

	manager.CreateContainer(&Container{Name: "c1", Image: "nginx"})
	manager.CreateContainer(&Container{Name: "c2", Image: "redis"})

	stats := manager.GetStats()
	if stats["total_containers"] != 2 {
		t.Errorf("Expected 2 containers, got %v", stats["total_containers"])
	}
}
