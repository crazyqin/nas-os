package truecommand

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := TrueCommandConfig{
		PollInterval: 60 * time.Second,
		MaxSystems:   50,
	}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.config.MaxSystems != 50 {
		t.Errorf("期望 MaxSystems=50, 实际 %d", m.config.MaxSystems)
	}
}

func TestManager_StartStop(t *testing.T) {
	cfg := TrueCommandConfig{}
	m := NewManager(cfg)
	if err := m.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if !m.running {
		t.Error("期望 running=true")
	}
	m.Stop()
	if m.running {
		t.Error("期望 running=false")
	}
}

func TestManager_SystemLifecycle(t *testing.T) {
	cfg := TrueCommandConfig{MaxSystems: 10}
	m := NewManager(cfg)

	// 注册系统
	system := &NASSystem{
		ID:           "nas1",
		Name:         "主存储",
		Host:         "192.168.1.100",
		Port:         80,
		Version:      "v2.542.0",
		CPUCores:     8,
		MemoryTotal:  16 * 1024 * 1024 * 1024,
		StorageTotal: 10 * 1024 * 1024 * 1024 * 1024,
	}
	if err := m.RegisterSystem(system); err != nil {
		t.Fatalf("RegisterSystem 失败: %v", err)
	}

	// 获取系统
	got, err := m.GetSystem("nas1")
	if err != nil {
		t.Fatalf("GetSystem 失败: %v", err)
	}
	if got.Name != "主存储" {
		t.Errorf("期望 name=主存储, 实际 %s", got.Name)
	}

	// 列表
	systems := m.ListSystems()
	if len(systems) != 1 {
		t.Errorf("期望 1 个系统, 实际 %d", len(systems))
	}

	// 重复注册
	if err := m.RegisterSystem(system); err == nil {
		t.Error("重复注册应报错")
	}

	// 注销
	if err := m.UnregisterSystem("nas1"); err != nil {
		t.Fatalf("UnregisterSystem 失败: %v", err)
	}
}

func TestManager_ClusterLifecycle(t *testing.T) {
	cfg := TrueCommandConfig{}
	m := NewManager(cfg)

	// 创建集群
	cluster := &Cluster{
		ID:   "ha1",
		Name: "HA集群",
		Type: ClusterTypeHA,
	}
	if err := m.CreateCluster(cluster); err != nil {
		t.Fatalf("CreateCluster 失败: %v", err)
	}

	// 获取集群
	got, err := m.GetCluster("ha1")
	if err != nil {
		t.Fatalf("GetCluster 失败: %v", err)
	}
	if got.Name != "HA集群" {
		t.Errorf("期望 name=HA集群, 实际 %s", got.Name)
	}

	// 添加系统到集群
	m.RegisterSystem(&NASSystem{ID: "nas1", Name: "test1", Host: "192.168.1.1"})
	m.RegisterSystem(&NASSystem{ID: "nas2", Name: "test2", Host: "192.168.1.2"})
	if err := m.AddSystemToCluster("nas1", "ha1"); err != nil {
		t.Fatalf("AddSystemToCluster 失败: %v", err)
	}
	if err := m.AddSystemToCluster("nas2", "ha1"); err != nil {
		t.Fatalf("AddSystemToCluster 失败: %v", err)
	}

	cluster, _ = m.GetCluster("ha1")
	if len(cluster.Members) != 2 {
		t.Errorf("期望 2 个成员, 实际 %d", len(cluster.Members))
	}

	// 删除集群
	if err := m.DeleteCluster("ha1"); err != nil {
		t.Fatalf("DeleteCluster 失败: %v", err)
	}
}

func TestManager_DashboardLifecycle(t *testing.T) {
	cfg := TrueCommandConfig{}
	m := NewManager(cfg)

	// 创建仪表板
	dashboard := &Dashboard{
		ID:   "d1",
		Name: "总览",
		Widgets: []Widget{
			{ID: "w1", Type: WidgetTypeCPU, Title: "CPU使用率"},
			{ID: "w2", Type: WidgetTypeMemory, Title: "内存使用"},
		},
	}
	if err := m.CreateDashboard(dashboard); err != nil {
		t.Fatalf("CreateDashboard 失败: %v", err)
	}

	// 获取仪表板
	got, err := m.GetDashboard("d1")
	if err != nil {
		t.Fatalf("GetDashboard 失败: %v", err)
	}
	if len(got.Widgets) != 2 {
		t.Errorf("期望 2 个组件, 实际 %d", len(got.Widgets))
	}

	// 列表
	dashboards := m.ListDashboards()
	if len(dashboards) != 1 {
		t.Errorf("期望 1 个仪表板, 实际 %d", len(dashboards))
	}
}

func TestManager_GetStats(t *testing.T) {
	cfg := TrueCommandConfig{}
	m := NewManager(cfg)
	m.RegisterSystem(&NASSystem{
		ID:           "nas1",
		Name:         "test",
		Host:         "192.168.1.1",
		CPUUsage:     50.0,
		MemoryTotal:  8 * 1024 * 1024 * 1024,
		StorageTotal: 1024 * 1024 * 1024 * 1024,
	})

	stats := m.GetStats()
	if stats.TotalSystems != 1 {
		t.Errorf("期望 1 个系统, 实际 %d", stats.TotalSystems)
	}
	if stats.OnlineSystems != 1 {
		t.Errorf("期望 1 个在线系统, 实际 %d", stats.OnlineSystems)
	}
}
