package lxcha

import (
	"testing"
	"time"
)

func TestManagerStartStop(t *testing.T) {
	m := NewManager(nil)
	if m.IsRunning() {
		t.Fatal("新创建的管理器不应在运行")
	}
	if err := m.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("管理器应该在运行")
	}
	if err := m.Start(); err == nil {
		t.Fatal("重复启动应返回错误")
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	if m.IsRunning() {
		t.Fatal("管理器不应在运行")
	}
}

func TestCreateContainer(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	c := &Container{
		ID:       "ct-1",
		Name:     "web-server",
		NodeID:   "node-1",
		Image:    "ubuntu:22.04",
		CPUCores: 2,
		MemoryMB: 2048,
		StorageMB: 10240,
	}
	if err := m.CreateContainer(c); err != nil {
		t.Fatalf("创建容器失败: %v", err)
	}
	if c.Status != StatusStopped {
		t.Fatal("容器初始状态应为stopped")
	}
}

func TestCreateContainerDuplicate(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "test", NodeID: "node-1"})
	if err := m.CreateContainer(&Container{ID: "ct-1", Name: "test2", NodeID: "node-1"}); err == nil {
		t.Fatal("创建重复容器应返回错误")
	}
}

func TestCreateContainerNotRunning(t *testing.T) {
	m := NewManager(nil)
	if err := m.CreateContainer(&Container{ID: "ct-1", Name: "test", NodeID: "node-1"}); err == nil {
		t.Fatal("未运行时创建容器应返回错误")
	}
}

func TestStartStopContainer(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "test", NodeID: "node-1"})

	if err := m.StartContainer("ct-1"); err != nil {
		t.Fatalf("启动容器失败: %v", err)
	}
	c, _ := m.GetContainer("ct-1")
	if c.Status != StatusRunning {
		t.Fatal("容器状态应为running")
	}
	if c.StartedAt == nil {
		t.Fatal("StartedAt不应为空")
	}

	if err := m.StopContainer("ct-1"); err != nil {
		t.Fatalf("停止容器失败: %v", err)
	}
	c, _ = m.GetContainer("ct-1")
	if c.Status != StatusStopped {
		t.Fatal("容器状态应为stopped")
	}
}

func TestStartContainerAlreadyRunning(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "test", NodeID: "node-1"})
	m.StartContainer("ct-1")
	if err := m.StartContainer("ct-1"); err == nil {
		t.Fatal("重复启动应返回错误")
	}
}

func TestStartContainerNotFound(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	if err := m.StartContainer("nonexistent"); err == nil {
		t.Fatal("启动不存在的容器应返回错误")
	}
}

func TestStopContainerNotFound(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	if err := m.StopContainer("nonexistent"); err == nil {
		t.Fatal("停止不存在的容器应返回错误")
	}
}

func TestDeleteContainer(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "test", NodeID: "node-1"})
	if err := m.DeleteContainer("ct-1"); err != nil {
		t.Fatalf("删除容器失败: %v", err)
	}
}

func TestDeleteRunningContainer(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "test", NodeID: "node-1"})
	m.StartContainer("ct-1")
	if err := m.DeleteContainer("ct-1"); err == nil {
		t.Fatal("删除运行中的容器应返回错误")
	}
}

func TestDeleteContainerNotFound(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	if err := m.DeleteContainer("nonexistent"); err == nil {
		t.Fatal("删除不存在的容器应返回错误")
	}
}

func TestListContainers(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "web", NodeID: "node-1"})
	m.CreateContainer(&Container{ID: "ct-2", Name: "db", NodeID: "node-1"})
	m.CreateContainer(&Container{ID: "ct-3", Name: "cache", NodeID: "node-2"})
	m.StartContainer("ct-1")

	all := m.ListContainers("", "")
	if len(all) != 3 {
		t.Fatalf("期望3个容器，实际 %d", len(all))
	}
	node1 := m.ListContainers("node-1", "")
	if len(node1) != 2 {
		t.Fatalf("期望2个node-1容器，实际 %d", len(node1))
	}
	running := m.ListContainers("", StatusRunning)
	if len(running) != 1 {
		t.Fatalf("期望1个运行中容器，实际 %d", len(running))
	}
}

func TestFailoverContainer(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "web", NodeID: "node-1"})
	m.StartContainer("ct-1")

	event, err := m.FailoverContainer("ct-1", "node-2", "节点故障")
	if err != nil {
		t.Fatalf("故障转移失败: %v", err)
	}
	if !event.Success {
		t.Fatal("故障转移应成功")
	}
	if event.SourceNode != "node-1" {
		t.Fatal("源节点应为node-1")
	}
	if event.TargetNode != "node-2" {
		t.Fatal("目标节点应为node-2")
	}

	c, _ := m.GetContainer("ct-1")
	if c.NodeID != "node-2" {
		t.Fatal("容器应迁移到node-2")
	}
}

func TestFailoverContainerNotEnabled(t *testing.T) {
	config := DefaultHAConfig()
	config.Enabled = false
	m := NewManager(config)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "web", NodeID: "node-1"})
	_, err := m.FailoverContainer("ct-1", "node-2", "test")
	if err == nil {
		t.Fatal("HA未启用时应返回错误")
	}
}

func TestFailoverContainerNotFound(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	_, err := m.FailoverContainer("nonexistent", "node-2", "test")
	if err == nil {
		t.Fatal("故障转移不存在的容器应返回错误")
	}
}

func TestGetFailoverEvents(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "web", NodeID: "node-1"})
	m.FailoverContainer("ct-1", "node-2", "故障1")
	m.FailoverContainer("ct-1", "node-3", "故障2")

	events := m.GetFailoverEvents("ct-1")
	if len(events) != 2 {
		t.Fatalf("期望2个事件，实际 %d", len(events))
	}
	allEvents := m.GetFailoverEvents("")
	if len(allEvents) != 2 {
		t.Fatalf("期望2个事件，实际 %d", len(allEvents))
	}
}

func TestEnableGPU(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "ml-server", NodeID: "node-1"})
	gpu := &GPUConfig{
		Enabled:  true,
		DeviceID: "gpu-0",
		Type:     "nvidia",
		MemoryMB: 8192,
	}
	if err := m.EnableGPU("ct-1", gpu); err != nil {
		t.Fatalf("启用GPU失败: %v", err)
	}
	c, _ := m.GetContainer("ct-1")
	if c.GPU == nil || c.GPU.Type != "nvidia" {
		t.Fatal("GPU配置不正确")
	}
}

func TestEnableGPUNotEnabled(t *testing.T) {
	config := DefaultHAConfig()
	config.EnableGPUPassthrough = false
	m := NewManager(config)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "test", NodeID: "node-1"})
	if err := m.EnableGPU("ct-1", &GPUConfig{Enabled: true}); err == nil {
		t.Fatal("GPU直通未启用时应返回错误")
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateContainer(&Container{ID: "ct-1", Name: "web", NodeID: "node-1"})
	m.CreateContainer(&Container{ID: "ct-2", Name: "db", NodeID: "node-1"})
	m.StartContainer("ct-1")

	stats := m.GetStats()
	if stats["total_containers"] != 2 {
		t.Fatalf("期望2个容器，实际 %v", stats["total_containers"])
	}
	if stats["running_containers"] != 1 {
		t.Fatalf("期望1个运行中容器，实际 %v", stats["running_containers"])
	}
	if stats["ha_enabled"] != true {
		t.Fatal("HA应该启用")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultHAConfig()
	if !config.Enabled {
		t.Fatal("默认应启用HA")
	}
	if config.Mode != HAModeActivePassive {
		t.Fatal("默认模式应为active_passive")
	}
	if config.HeartbeatInterval != 5*time.Second {
		t.Fatal("心跳间隔错误")
	}
	if config.MaxRetries != 3 {
		t.Fatal("最大重试次数错误")
	}
	if !config.EnableGPUPassthrough {
		t.Fatal("默认应启用GPU直通")
	}
}
