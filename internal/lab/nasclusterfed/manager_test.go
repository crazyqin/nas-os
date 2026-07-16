package nasclusterfed

import (
	"os"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nasclusterfed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(&ManagerConfig{
		DataDir:      tmpDir,
		SyncInterval: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr == nil {
		t.Fatal("Expected non-nil manager")
	}

	if mgr.dataDir != tmpDir {
		t.Errorf("DataDir mismatch: got %s, want %s", mgr.dataDir, tmpDir)
	}
}

func TestNewManager_DefaultSyncInterval(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr.syncInterval != 30*time.Second {
		t.Errorf("Expected default sync interval 30s, got %v", mgr.syncInterval)
	}
}

func TestNewManager_EmptyDataDir(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{
		DataDir: "",
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr.dataDir != "" {
		t.Errorf("Expected empty dataDir, got %s", mgr.dataDir)
	}
}

func TestManager_RegisterCluster(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	cluster := &Cluster{
		ID:          "cluster-1",
		Name:        "Test Cluster",
		Description: "测试集群",
		Region:      "cn-east",
		Role:        ClusterRoleLeader,
	}

	err = mgr.RegisterCluster(cluster)
	if err != nil {
		t.Fatalf("Failed to register cluster: %v", err)
	}

	retrieved, err := mgr.GetCluster("cluster-1")
	if err != nil {
		t.Fatalf("Failed to get cluster: %v", err)
	}

	if retrieved.Name != "Test Cluster" {
		t.Errorf("Expected cluster name 'Test Cluster', got %s", retrieved.Name)
	}
}

func TestManager_RegisterCluster_EmptyID(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	cluster := &Cluster{
		Name: "Test Cluster",
	}

	err = mgr.RegisterCluster(cluster)
	if err == nil {
		t.Fatal("Expected error for empty cluster ID")
	}
}

func TestManager_UnregisterCluster(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	cluster := &Cluster{
		ID:   "cluster-1",
		Name: "Test Cluster",
	}

	_ = mgr.RegisterCluster(cluster)

	err = mgr.UnregisterCluster("cluster-1")
	if err != nil {
		t.Fatalf("Failed to unregister cluster: %v", err)
	}

	_, err = mgr.GetCluster("cluster-1")
	if err == nil {
		t.Fatal("Expected error for unregistered cluster")
	}
}

func TestManager_UnregisterCluster_NotExists(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = mgr.UnregisterCluster("non-existent")
	if err == nil {
		t.Fatal("Expected error for non-existent cluster")
	}
}

func TestManager_ListClusters(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterCluster(&Cluster{ID: "c1", Name: "Cluster 1"})
	_ = mgr.RegisterCluster(&Cluster{ID: "c2", Name: "Cluster 2"})

	clusters := mgr.ListClusters()
	if len(clusters) != 2 {
		t.Errorf("Expected 2 clusters, got %d", len(clusters))
	}
}

func TestManager_AddNodeToCluster(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterCluster(&Cluster{ID: "cluster-1", Name: "Test Cluster"})

	node := &ClusterNode{
		ID:       "node-1",
		Hostname: "nas-01",
		IP:       "192.168.1.100",
		Port:     8080,
		Role:     ClusterRoleFollower,
	}

	err = mgr.AddNodeToCluster("cluster-1", node)
	if err != nil {
		t.Fatalf("Failed to add node: %v", err)
	}

	cluster, _ := mgr.GetCluster("cluster-1")
	if len(cluster.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(cluster.Nodes))
	}
}

func TestManager_RemoveNodeFromCluster(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterCluster(&Cluster{ID: "cluster-1", Name: "Test Cluster"})
	_ = mgr.AddNodeToCluster("cluster-1", &ClusterNode{ID: "node-1", Hostname: "nas-01"})

	err = mgr.RemoveNodeFromCluster("cluster-1", "node-1")
	if err != nil {
		t.Fatalf("Failed to remove node: %v", err)
	}

	cluster, _ := mgr.GetCluster("cluster-1")
	if len(cluster.Nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(cluster.Nodes))
	}
}

func TestManager_CreateSyncTask(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterCluster(&Cluster{ID: "cluster-1", Name: "Source"})
	_ = mgr.RegisterCluster(&Cluster{ID: "cluster-2", Name: "Target"})

	task, err := mgr.CreateSyncTask("cluster-1", "cluster-2", SyncModeIncremental)
	if err != nil {
		t.Fatalf("Failed to create sync task: %v", err)
	}

	if task.ID == "" {
		t.Error("Expected non-empty task ID")
	}

	if task.SourceClusterID != "cluster-1" {
		t.Errorf("Expected source cluster 'cluster-1', got %s", task.SourceClusterID)
	}
}

func TestManager_CreateSyncTask_InvalidCluster(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_, err = mgr.CreateSyncTask("non-existent", "cluster-2", SyncModeFull)
	if err == nil {
		t.Fatal("Expected error for non-existent source cluster")
	}
}

func TestManager_GetFederationStatus(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterCluster(&Cluster{ID: "c1", Name: "Cluster 1", Status: ClusterStatusOnline})
	_ = mgr.RegisterCluster(&Cluster{ID: "c2", Name: "Cluster 2", Status: ClusterStatusOffline})

	status := mgr.GetFederationStatus()

	totalClusters, ok := status["totalClusters"].(int)
	if !ok || totalClusters != 2 {
		t.Errorf("Expected totalClusters=2, got %v", status["totalClusters"])
	}

	onlineClusters, ok := status["onlineClusters"].(int)
	if !ok || onlineClusters != 1 {
		t.Errorf("Expected onlineClusters=1, got %v", status["onlineClusters"])
	}
}

func TestManager_GetEvents(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterCluster(&Cluster{ID: "c1", Name: "Cluster 1"})
	_ = mgr.UnregisterCluster("c1")

	events := mgr.GetEvents(10)
	if len(events) < 2 {
		t.Errorf("Expected at least 2 events, got %d", len(events))
	}
}

func TestManager_StartStop(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	mgr.Start()
	if !mgr.running {
		t.Error("Expected manager to be running")
	}

	// 启动两次不应出错
	mgr.Start()

	mgr.Stop()
	if mgr.running {
		t.Error("Expected manager to be stopped")
	}
}

func TestManager_Subscribe(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ch := mgr.Subscribe()
	if ch == nil {
		t.Fatal("Expected non-nil channel")
	}

	// 注册集群应触发事件
	_ = mgr.RegisterCluster(&Cluster{ID: "c1", Name: "Test"})

	select {
	case evt := <-ch:
		if evt.Type != "cluster_registered" {
			t.Errorf("Expected event type 'cluster_registered', got %s", evt.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}
