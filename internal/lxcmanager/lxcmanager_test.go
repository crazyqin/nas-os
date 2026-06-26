package lxcmanager

import (
	"context"
	"fmt"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.config.MaxContainers != 200 {
		t.Errorf("默认最大容器数 = %d, want 200", m.config.MaxContainers)
	}
	if len(m.containers) != 0 {
		t.Errorf("新管理器应有 0 个容器, got %d", len(m.containers))
	}
}

func TestCreateContainer(t *testing.T) {
	m := NewManager(nil)

	cfg := ContainerConfig{
		Name:        "test-ct",
		Template:    "ubuntu/22.04",
		Hostname:    "testct",
		CPULimit:    2,
		MemoryLimit: 1024 * 1024 * 1024,
	}

	info, err := m.CreateContainer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CreateContainer 失败: %v", err)
	}
	if info.Name != "test-ct" {
		t.Errorf("容器名 = %q, want %q", info.Name, "test-ct")
	}
	if info.Status != StatusCreated {
		t.Errorf("状态 = %q, want %q", info.Status, StatusCreated)
	}
	if info.Config.CPULimit != 2 {
		t.Errorf("CPU限制 = %d, want 2", info.Config.CPULimit)
	}
}

func TestCreateContainerDuplicate(t *testing.T) {
	m := NewManager(nil)
	cfg := ContainerConfig{Name: "dup", Template: "alpine/3.18"}

	if _, err := m.CreateContainer(context.Background(), cfg); err != nil {
		t.Fatalf("首次创建失败: %v", err)
	}
	if _, err := m.CreateContainer(context.Background(), cfg); err == nil {
		t.Fatal("重复创建应返回错误")
	}
}

func TestDestroyContainer(t *testing.T) {
	m := NewManager(nil)
	cfg := ContainerConfig{Name: "destroy-me", Template: "alpine/3.18"}

	m.CreateContainer(context.Background(), cfg)
	if err := m.DestroyContainer(context.Background(), "destroy-me"); err != nil {
		t.Fatalf("DestroyContainer 失败: %v", err)
	}
	if _, err := m.GetContainer("destroy-me"); err == nil {
		t.Fatal("销毁后应无法获取容器")
	}
}

func TestDestroyRunningContainer(t *testing.T) {
	m := NewManager(nil)
	cfg := ContainerConfig{Name: "running-ct", Template: "alpine/3.18"}

	m.CreateContainer(context.Background(), cfg)
	m.StartContainer(context.Background(), "running-ct")

	if err := m.DestroyContainer(context.Background(), "running-ct"); err == nil {
		t.Fatal("销毁运行中容器应返回错误")
	}
}

func TestStartStopContainer(t *testing.T) {
	m := NewManager(nil)
	cfg := ContainerConfig{Name: "lifecycle", Template: "debian/12"}

	m.CreateContainer(context.Background(), cfg)

	if err := m.StartContainer(context.Background(), "lifecycle"); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	info, _ := m.GetContainer("lifecycle")
	if info.Status != StatusRunning {
		t.Errorf("状态 = %q, want %q", info.Status, StatusRunning)
	}

	if err := m.StopContainer(context.Background(), "lifecycle"); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	info, _ = m.GetContainer("lifecycle")
	if info.Status != StatusStopped {
		t.Errorf("状态 = %q, want %q", info.Status, StatusStopped)
	}
}

func TestSetResourceLimits(t *testing.T) {
	m := NewManager(nil)
	cfg := ContainerConfig{Name: "res-ct", Template: "alpine/3.18", CPULimit: 1, MemoryLimit: 512 * 1024 * 1024}
	m.CreateContainer(context.Background(), cfg)

	limits := ResourceLimits{
		MemoryLimit: 2048 * 1024 * 1024,
		CPUQuota:    50000,
	}
	if err := m.SetResourceLimits(context.Background(), "res-ct", limits); err != nil {
		t.Fatalf("SetResourceLimits 失败: %v", err)
	}

	info, _ := m.GetContainer("res-ct")
	if info.Config.MemoryLimit != 2048*1024*1024 {
		t.Errorf("内存限制 = %d, want %d", info.Config.MemoryLimit, 2048*1024*1024)
	}
}

func TestConfigureNetwork(t *testing.T) {
	m := NewManager(nil)
	cfg := ContainerConfig{Name: "net-ct", Template: "alpine/3.18"}
	m.CreateContainer(context.Background(), cfg)

	netCfg := NetworkConfig{
		Type:      NetMACVLAN,
		Bridge:    "br0",
		IPAddress: "192.168.1.100",
		Gateway:   "192.168.1.1",
	}
	if err := m.ConfigureNetwork(context.Background(), "net-ct", netCfg); err != nil {
		t.Fatalf("ConfigureNetwork 失败: %v", err)
	}

	info, _ := m.GetContainer("net-ct")
	if info.Config.Network.Type != NetMACVLAN {
		t.Errorf("网络类型 = %q, want %q", info.Config.Network.Type, NetMACVLAN)
	}
}

func TestTemplateManagement(t *testing.T) {
	m := NewManager(nil)

	tmpl := TemplateInfo{
		Name:        "ubuntu-22.04",
		Alias:       "ubuntu/22.04",
		OS:          "ubuntu",
		Arch:        "amd64",
		Size:        400 * 1024 * 1024,
		Description: "Ubuntu 22.04 LTS",
	}
	if err := m.RegisterTemplate(tmpl); err != nil {
		t.Fatalf("RegisterTemplate 失败: %v", err)
	}

	templates := m.ListTemplates()
	if len(templates) != 1 {
		t.Fatalf("模板数 = %d, want 1", len(templates))
	}
	if templates[0].Name != "ubuntu-22.04" {
		t.Errorf("模板名 = %q, want %q", templates[0].Name, "ubuntu-22.04")
	}

	if err := m.DeleteTemplate("ubuntu-22.04"); err != nil {
		t.Fatalf("DeleteTemplate 失败: %v", err)
	}
	if len(m.ListTemplates()) != 0 {
		t.Error("删除后模板列表应空")
	}
}

func TestMaxContainers(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.MaxContainers = 2
	m := NewManager(cfg)

	for i := 0; i < 2; i++ {
		_, err := m.CreateContainer(context.Background(), ContainerConfig{
			Name:     fmt.Sprintf("ct-%d", i),
			Template: "alpine/3.18",
		})
		if err != nil {
			t.Fatalf("创建容器 %d 失败: %v", i, err)
		}
	}

	_, err := m.CreateContainer(context.Background(), ContainerConfig{
		Name: "ct-overflow", Template: "alpine/3.18",
	})
	if err == nil {
		t.Fatal("超出限制应返回错误")
	}
}
