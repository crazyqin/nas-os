package multiclusterfed

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		config FederationConfig
	}{
		{name: "空配置", config: FederationConfig{}},
		{name: "自定义配置", config: FederationConfig{
			ClusterID:   "cluster-1",
			ClusterName: "TestCluster",
			ListenPort:  9999,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := New(tt.config)
			if mgr == nil {
				t.Fatal("New返回nil")
			}
			if mgr.clusters == nil {
				t.Error("clusters未初始化")
			}
			if mgr.namespaces == nil {
				t.Error("namespaces未初始化")
			}
		})
	}
}

func TestNewDefaults(t *testing.T) {
	mgr := New(FederationConfig{})

	cfg := mgr.GetConfig()
	if cfg.LoadBalanceStrategy != LoadBalanceRoundRobin {
		t.Errorf("默认负载均衡策略应为round_robin, 实际 %s", cfg.LoadBalanceStrategy)
	}
	if cfg.HealthCheck.Interval != 10*time.Second {
		t.Errorf("默认健康检查间隔应为10s, 实际 %s", cfg.HealthCheck.Interval)
	}
	if cfg.HealthCheck.FailureThreshold != 3 {
		t.Errorf("默认失败阈值应为3, 实际 %d", cfg.HealthCheck.FailureThreshold)
	}
	if cfg.Sync.Workers != 4 {
		t.Errorf("默认同步worker数应为4, 实际 %d", cfg.Sync.Workers)
	}
}

func TestStartStop(t *testing.T) {
	mgr := New(FederationConfig{ClusterID: "c1", ClusterName: "C1"})

	if err := mgr.Start(); err != nil {
		t.Fatalf("Start失败: %v", err)
	}
	if !mgr.IsRunning() {
		t.Error("Start后应为运行状态")
	}

	// 重复Start应安全返回
	if err := mgr.Start(); err != nil {
		t.Errorf("重复Start不应报错: %v", err)
	}

	if err := mgr.Stop(); err != nil {
		t.Fatalf("Stop失败: %v", err)
	}
	if mgr.IsRunning() {
		t.Error("Stop后应为非运行状态")
	}

	// 重复Stop应安全返回
	if err := mgr.Stop(); err != nil {
		t.Errorf("重复Stop不应报错: %v", err)
	}
}

func TestAddClusterNotRunning(t *testing.T) {
	mgr := New(FederationConfig{})
	// 未启动时添加集群应报错
	err := mgr.AddCluster(&Cluster{ID: "c1", Name: "C1", Endpoint: "localhost:9999"})
	if err != ErrManagerNotRunning {
		t.Errorf("期望ErrManagerNotRunning, 实际 %v", err)
	}
}

func TestAddRemoveCluster(t *testing.T) {
	mgr := New(FederationConfig{ClusterID: "local", ClusterName: "Local"})
	mgr.Start()
	defer mgr.Stop()

	// 添加集群
	cluster := &Cluster{
		ID:       "cluster-1",
		Name:     "Cluster 1",
		Endpoint: "127.0.0.1:9999",
		Weight:   10,
		Region:   "cn-east",
	}
	if err := mgr.AddCluster(cluster); err != nil {
		t.Fatalf("AddCluster失败: %v", err)
	}

	// 重复添加应报错
	if err := mgr.AddCluster(cluster); err != ErrClusterAlreadyExists {
		t.Errorf("期望ErrClusterAlreadyExists, 实际 %v", err)
	}

	// 列出集群
	clusters := mgr.ListClusters()
	if len(clusters) != 1 {
		t.Errorf("期望1个集群, 实际 %d", len(clusters))
	}

	// 移除集群
	if err := mgr.RemoveCluster("cluster-1"); err != nil {
		t.Fatalf("RemoveCluster失败: %v", err)
	}

	// 移除不存在的集群
	if err := mgr.RemoveCluster("nonexistent"); err != ErrClusterNotFound {
		t.Errorf("期望ErrClusterNotFound, 实际 %v", err)
	}
}

func TestAddNode(t *testing.T) {
	mgr := New(FederationConfig{ClusterID: "local", ClusterName: "Local"})
	mgr.Start()
	defer mgr.Stop()

	mgr.AddCluster(&Cluster{ID: "c1", Name: "C1", Endpoint: "127.0.0.1:9999"})

	node := &ClusterNode{
		ID:       "node-1",
		Hostname: "nas-node-1",
		Address:  "192.168.1.10:8080",
		State:    ClusterStateOnline,
		Capacity: 1024 * 1024 * 1024 * 100,
		Weight:   10,
	}

	if err := mgr.AddNode("c1", node); err != nil {
		t.Fatalf("AddNode失败: %v", err)
	}

	// 重复添加同一节点应报错
	if err := mgr.AddNode("c1", node); err == nil {
		t.Error("重复添加节点应返回错误")
	}

	// 向不存在的集群添加节点
	if err := mgr.AddNode("nonexistent", node); err != ErrClusterNotFound {
		t.Errorf("期望ErrClusterNotFound, 实际 %v", err)
	}

	// 移除节点
	if err := mgr.RemoveNode("c1", "node-1"); err != nil {
		t.Fatalf("RemoveNode失败: %v", err)
	}

	// 移除不存在的节点
	if err := mgr.RemoveNode("c1", "nonexistent"); err == nil {
		t.Error("移除不存在的节点应返回错误")
	}
}

func TestNamespaceManagement(t *testing.T) {
	mgr := New(FederationConfig{ClusterID: "local", ClusterName: "Local"})
	mgr.Start()
	defer mgr.Stop()

	// 添加集群
	mgr.AddCluster(&Cluster{ID: "c1", Name: "C1", Endpoint: "127.0.0.1:9999"})
	mgr.AddCluster(&Cluster{ID: "c2", Name: "C2", Endpoint: "127.0.0.2:9999"})

	// 创建命名空间
	ns := &Namespace{
		Path:            "/data/shared",
		PrimaryCluster:  "c1",
		ReplicaClusters: []string{"c2"},
		SyncMode:        SyncModeAsync,
	}
	if err := mgr.CreateNamespace(ns); err != nil {
		t.Fatalf("CreateNamespace失败: %v", err)
	}

	// 重复创建应报错
	if err := mgr.CreateNamespace(ns); err != ErrNamespaceConflict {
		t.Errorf("期望ErrNamespaceConflict, 实际 %v", err)
	}

	// 获取命名空间
	got, err := mgr.GetNamespace("/data/shared")
	if err != nil {
		t.Fatalf("GetNamespace失败: %v", err)
	}
	if got.PrimaryCluster != "c1" {
		t.Errorf("主集群应为c1, 实际 %s", got.PrimaryCluster)
	}

	// 列出命名空间
	list := mgr.ListNamespaces()
	if len(list) != 1 {
		t.Errorf("期望1个命名空间, 实际 %d", len(list))
	}

	// 删除命名空间
	if err := mgr.DeleteNamespace("/data/shared"); err != nil {
		t.Fatalf("DeleteNamespace失败: %v", err)
	}

	// 获取已删除的命名空间
	if _, err := mgr.GetNamespace("/data/shared"); err == nil {
		t.Error("获取已删除的命名空间应返回错误")
	}
}

func TestUpdateNodeHeartbeat(t *testing.T) {
	mgr := New(FederationConfig{ClusterID: "local", ClusterName: "Local"})
	mgr.Start()
	defer mgr.Stop()

	mgr.AddCluster(&Cluster{ID: "c1", Name: "C1", Endpoint: "127.0.0.1:9999"})
	mgr.AddNode("c1", &ClusterNode{
		ID:      "node-1",
		Address: "192.168.1.10:8080",
	})

	if err := mgr.UpdateNodeHeartbeat("c1", "node-1"); err != nil {
		t.Fatalf("UpdateNodeHeartbeat失败: %v", err)
	}

	// 不存在的集群
	if err := mgr.UpdateNodeHeartbeat("nonexistent", "node-1"); err != ErrClusterNotFound {
		t.Errorf("期望ErrClusterNotFound, 实际 %v", err)
	}

	// 不存在的节点
	if err := mgr.UpdateNodeHeartbeat("c1", "nonexistent"); err == nil {
		t.Error("更新不存在节点的心跳应返回错误")
	}
}

func TestGetClusterStatus(t *testing.T) {
	mgr := New(FederationConfig{ClusterID: "local", ClusterName: "Local"})
	mgr.Start()
	defer mgr.Stop()

	mgr.AddCluster(&Cluster{ID: "c1", Name: "C1", Endpoint: "127.0.0.1:9999"})
	mgr.AddNode("c1", &ClusterNode{
		ID:       "n1",
		Hostname: "node1",
		Address:  "192.168.1.10:8080",
		State:    ClusterStateOnline,
		Capacity: 1000,
		Used:     500,
	})

	status, err := mgr.GetClusterStatus("c1")
	if err != nil {
		t.Fatalf("GetClusterStatus失败: %v", err)
	}
	if status.ClusterID != "c1" {
		t.Errorf("集群ID应为c1, 实际 %s", status.ClusterID)
	}
	if status.NodeCount != 1 {
		t.Errorf("节点数应为1, 实际 %d", status.NodeCount)
	}

	// 不存在的集群
	if _, err := mgr.GetClusterStatus("nonexistent"); err != ErrClusterNotFound {
		t.Errorf("期望ErrClusterNotFound, 实际 %v", err)
	}
}

func TestGetAllClusterStatus(t *testing.T) {
	mgr := New(FederationConfig{ClusterID: "local", ClusterName: "Local"})
	mgr.Start()
	defer mgr.Stop()

	mgr.AddCluster(&Cluster{ID: "c1", Name: "C1", Endpoint: "127.0.0.1:9999"})
	mgr.AddCluster(&Cluster{ID: "c2", Name: "C2", Endpoint: "127.0.0.2:9999"})

	statuses := mgr.GetAllClusterStatus()
	if len(statuses) != 2 {
		t.Errorf("期望2个状态报告, 实际 %d", len(statuses))
	}
}

func TestTriggerSync(t *testing.T) {
	mgr := New(FederationConfig{ClusterID: "local", ClusterName: "Local"})
	mgr.Start()
	defer mgr.Stop()

	mgr.AddCluster(&Cluster{ID: "c1", Name: "C1", Endpoint: "127.0.0.1:9999"})
	mgr.AddCluster(&Cluster{ID: "c2", Name: "C2", Endpoint: "127.0.0.2:9999"})

	taskID, err := mgr.TriggerSync("c1", "c2", "/data/test")
	if err != nil {
		t.Fatalf("TriggerSync失败: %v", err)
	}
	if taskID == "" {
		t.Error("任务ID不应为空")
	}

	// 不存在的源集群
	_, err = mgr.TriggerSync("nonexistent", "c2", "/data/test")
	if err != ErrClusterNotFound {
		t.Errorf("期望ErrClusterNotFound, 实际 %v", err)
	}
}

func TestGetStats(t *testing.T) {
	mgr := New(FederationConfig{ClusterID: "local", ClusterName: "Local"})

	stats := mgr.GetStats()
	if stats.TotalClusters != 0 {
		t.Errorf("初始集群数应为0, 实际 %d", stats.TotalClusters)
	}
}

func TestGetFailoverEvents(t *testing.T) {
	mgr := New(FederationConfig{})

	events := mgr.GetFailoverEvents()
	if len(events) != 0 {
		t.Errorf("初始故障转移事件应为0, 实际 %d", len(events))
	}
}

func TestGetSyncTasks(t *testing.T) {
	mgr := New(FederationConfig{})

	tasks := mgr.GetSyncTasks()
	if len(tasks) != 0 {
		t.Errorf("初始同步任务应为0, 实际 %d", len(tasks))
	}
}

func TestCreateNamespaceWithInvalidCluster(t *testing.T) {
	mgr := New(FederationConfig{ClusterID: "local", ClusterName: "Local"})
	mgr.Start()
	defer mgr.Stop()

	// 不存在的主集群
	ns := &Namespace{
		Path:           "/data/test",
		PrimaryCluster: "nonexistent",
	}
	if err := mgr.CreateNamespace(ns); err == nil {
		t.Error("引用不存在的主集群应返回错误")
	}
}
