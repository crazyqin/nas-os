package lxcmanager

import (
	"testing"
	"time"
)

func TestContainerManager_Create(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)

	if manager == nil {
		t.Fatal("expected manager to be created")
	}

	if manager.config.MaxContainers != config.MaxContainers {
		t.Errorf("MaxContainers = %d, want %d", manager.config.MaxContainers, config.MaxContainers)
	}
}

func TestContainerManager_StartStop(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)

	err := manager.Start()
	if err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}

	if !manager.running {
		t.Error("expected manager to be running")
	}

	err = manager.Stop()
	if err != nil {
		t.Fatalf("failed to stop manager: %v", err)
	}

	if manager.running {
		t.Error("expected manager to be stopped")
	}
}

func TestContainerManager_CreateContainer(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	containerConfig := &Container{
		Name:  "test-container",
		Image: "ubuntu:22.04",
		Command: []string{"/bin/bash"},
	}

	container, err := manager.CreateContainer("test-container", containerConfig)
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	if container.ID == "" {
		t.Error("expected container ID")
	}

	if container.Name != "test-container" {
		t.Errorf("container name = %s, want test-container", container.Name)
	}

	if container.State != ContainerStateStopped {
		t.Errorf("container state = %d, want %d", container.State, ContainerStateStopped)
	}
}

func TestContainerManager_StartContainer(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	containerConfig := &Container{
		Name:  "test-container",
		Image: "ubuntu:22.04",
	}

	container, _ := manager.CreateContainer("test-container", containerConfig)

	err := manager.StartContainer(container.ID)
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// 验证容器状态
	updated, _ := manager.GetContainer(container.ID)
	if updated.State != ContainerStateRunning {
		t.Errorf("container state = %d, want %d", updated.State, ContainerStateRunning)
	}

	if updated.PID == 0 {
		t.Error("expected PID > 0")
	}
}

func TestContainerManager_StopContainer(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	containerConfig := &Container{
		Name:  "test-container",
		Image: "ubuntu:22.04",
	}

	container, _ := manager.CreateContainer("test-container", containerConfig)
	manager.StartContainer(container.ID)

	err := manager.StopContainer(container.ID)
	if err != nil {
		t.Fatalf("failed to stop container: %v", err)
	}

	updated, _ := manager.GetContainer(container.ID)
	if updated.State != ContainerStateStopped {
		t.Errorf("container state = %d, want %d", updated.State, ContainerStateStopped)
	}
}

func TestContainerManager_RestartContainer(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	containerConfig := &Container{
		Name:  "test-container",
		Image: "ubuntu:22.04",
	}

	container, _ := manager.CreateContainer("test-container", containerConfig)
	manager.StartContainer(container.ID)

	err := manager.RestartContainer(container.ID)
	if err != nil {
		t.Fatalf("failed to restart container: %v", err)
	}

	updated, _ := manager.GetContainer(container.ID)
	if updated.State != ContainerStateRunning {
		t.Errorf("container state = %d, want %d", updated.State, ContainerStateRunning)
	}
}

func TestContainerManager_DeleteContainer(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	containerConfig := &Container{
		Name:  "test-container",
		Image: "ubuntu:22.04",
	}

	container, _ := manager.CreateContainer("test-container", containerConfig)

	err := manager.DeleteContainer(container.ID)
	if err != nil {
		t.Fatalf("failed to delete container: %v", err)
	}

	_, err = manager.GetContainer(container.ID)
	if err == nil {
		t.Error("expected error for deleted container")
	}
}

func TestContainerManager_GetContainerByName(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	containerConfig := &Container{
		Name:  "test-container",
		Image: "ubuntu:22.04",
	}

	manager.CreateContainer("test-container", containerConfig)

	container, err := manager.GetContainerByName("test-container")
	if err != nil {
		t.Fatalf("failed to get container by name: %v", err)
	}

	if container.Name != "test-container" {
		t.Errorf("container name = %s, want test-container", container.Name)
	}
}

func TestContainerManager_ListContainers(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	// 创建多个容器
	for i := 0; i < 3; i++ {
		containerConfig := &Container{
			Name:  "container-" + string(rune('0'+i)),
			Image: "ubuntu:22.04",
		}
		manager.CreateContainer("container-"+string(rune('0'+i)), containerConfig)
	}

	containers := manager.ListContainers()
	if len(containers) != 3 {
		t.Errorf("containers count = %d, want 3", len(containers))
	}
}

func TestContainerManager_ListRunningContainers(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	// 创建并启动容器
	for i := 0; i < 3; i++ {
		containerConfig := &Container{
			Name:  "container-" + string(rune('0'+i)),
			Image: "ubuntu:22.04",
		}
		container, _ := manager.CreateContainer("container-"+string(rune('0'+i)), containerConfig)
		manager.StartContainer(container.ID)
	}

	running := manager.ListRunningContainers()
	if len(running) != 3 {
		t.Errorf("running containers count = %d, want 3", len(running))
	}
}

func TestContainerManager_FreezeUnfreeze(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	containerConfig := &Container{
		Name:  "test-container",
		Image: "ubuntu:22.04",
	}

	container, _ := manager.CreateContainer("test-container", containerConfig)
	manager.StartContainer(container.ID)

	// 冻结
	err := manager.FreezeContainer(container.ID)
	if err != nil {
		t.Fatalf("failed to freeze container: %v", err)
	}

	updated, _ := manager.GetContainer(container.ID)
	if updated.State != ContainerStateFrozen {
		t.Errorf("container state = %d, want %d", updated.State, ContainerStateFrozen)
	}

	// 解冻
	err = manager.UnfreezeContainer(container.ID)
	if err != nil {
		t.Fatalf("failed to unfreeze container: %v", err)
	}

	updated, _ = manager.GetContainer(container.ID)
	if updated.State != ContainerStateRunning {
		t.Errorf("container state = %d, want %d", updated.State, ContainerStateRunning)
	}
}

func TestContainerManager_UpdateResources(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	containerConfig := &Container{
		Name:  "test-container",
		Image: "ubuntu:22.04",
	}

	container, _ := manager.CreateContainer("test-container", containerConfig)

	newResources := &ResourceConfig{
		CPULimit:    2.0,
		MemoryLimit: 1024 * 1024 * 1024, // 1GB
	}

	err := manager.UpdateResources(container.ID, newResources)
	if err != nil {
		t.Fatalf("failed to update resources: %v", err)
	}

	updated, _ := manager.GetContainer(container.ID)
	if updated.Resources.CPULimit != 2.0 {
		t.Errorf("CPU limit = %f, want 2.0", updated.Resources.CPULimit)
	}
}

func TestContainerManager_GetStats(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	// 创建容器
	for i := 0; i < 3; i++ {
		containerConfig := &Container{
			Name:  "container-" + string(rune('0'+i)),
			Image: "ubuntu:22.04",
		}
		container, _ := manager.CreateContainer("container-"+string(rune('0'+i)), containerConfig)
		manager.StartContainer(container.ID)
	}

	stats := manager.GetStats()

	if stats.TotalContainers != 3 {
		t.Errorf("TotalContainers = %d, want 3", stats.TotalContainers)
	}

	if stats.RunningContainers != 3 {
		t.Errorf("RunningContainers = %d, want 3", stats.RunningContainers)
	}
}

func TestContainerState_Constants(t *testing.T) {
	if ContainerStateStopped != 0 {
		t.Errorf("ContainerStateStopped = %d, want 0", ContainerStateStopped)
	}

	if ContainerStateRunning != 2 {
		t.Errorf("ContainerStateRunning = %d, want 2", ContainerStateRunning)
	}

	if ContainerStateFrozen != 5 {
		t.Errorf("ContainerStateFrozen = %d, want 5", ContainerStateFrozen)
	}
}

func TestContainerStateToString(t *testing.T) {
	tests := []struct {
		state    ContainerState
		expected string
	}{
		{ContainerStateStopped, "stopped"},
		{ContainerStateRunning, "running"},
		{ContainerStateFrozen, "frozen"},
		{ContainerStateError, "error"},
	}

	for _, tt := range tests {
		result := ContainerStateToString(tt.state)
		if result != tt.expected {
			t.Errorf("ContainerStateToString(%d) = %s, want %s", tt.state, result, tt.expected)
		}
	}
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	if config.MaxContainers <= 0 {
		t.Error("expected MaxContainers > 0")
	}

	if config.DefaultImage == "" {
		t.Error("expected DefaultImage")
	}

	if config.DefaultResources == nil {
		t.Error("expected DefaultResources")
	}

	if config.DefaultResources.CPULimit <= 0 {
		t.Error("expected CPULimit > 0")
	}

	if config.DefaultResources.MemoryLimit <= 0 {
		t.Error("expected MemoryLimit > 0")
	}
}

func TestContainerManager_ConcurrentAccess(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	// 并发创建容器
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			containerConfig := &Container{
				Name:  "container-" + string(rune('0'+i)),
				Image: "ubuntu:22.04",
			}
			container, _ := manager.CreateContainer("container-"+string(rune('0'+i)), containerConfig)
			manager.StartContainer(container.ID)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	time.Sleep(500 * time.Millisecond)

	stats := manager.GetStats()
	if stats.TotalContainers != 10 {
		t.Errorf("TotalContainers = %d, want 10", stats.TotalContainers)
	}
}

func TestContainerManager_MaxContainersLimit(t *testing.T) {
	config := DefaultManagerConfig()
	config.MaxContainers = 2
	manager := NewContainerManager(config)
	manager.Start()
	defer manager.Stop()

	// 创建容器直到达到限制
	for i := 0; i < 2; i++ {
		containerConfig := &Container{
			Name:  "container-" + string(rune('0'+i)),
			Image: "ubuntu:22.04",
		}
		_, err := manager.CreateContainer("container-"+string(rune('0'+i)), containerConfig)
		if err != nil {
			t.Fatalf("failed to create container: %v", err)
		}
	}

	// 尝试创建超出限制的容器
	containerConfig := &Container{
		Name:  "extra-container",
		Image: "ubuntu:22.04",
	}
	_, err := manager.CreateContainer("extra-container", containerConfig)
	if err == nil {
		t.Error("expected error for exceeding max containers")
	}
}
