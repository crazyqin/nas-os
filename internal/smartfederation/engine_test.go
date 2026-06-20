package smartfederation

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestRegisterCluster(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	cluster := &FederationCluster{
		ID:       "cluster-1",
		Name:     "main-cluster",
		Endpoint: "https://nas1.example.com",
		State:    ClusterStateActive,
		Region:   "cn-east",
		Nodes:    3,
		Capacity: 1024 * 1024 * 1024 * 1024,
	}
	
	if err := engine.RegisterCluster(cluster); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	got, ok := engine.GetCluster("cluster-1")
	if !ok {
		t.Fatal("expected cluster to be registered")
	}
	if got.Name != "main-cluster" {
		t.Errorf("expected name 'main-cluster', got '%s'", got.Name)
	}
}

func TestStartSyncJob(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	engine.RegisterCluster(&FederationCluster{ID: "c1", State: ClusterStateActive})
	engine.RegisterCluster(&FederationCluster{ID: "c2", State: ClusterStateActive})
	
	job, err := engine.StartSyncJob("c1", "c2", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if job.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", job.Status)
	}
}

func TestGetFederationStatus(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	engine.RegisterCluster(&FederationCluster{ID: "c1", State: ClusterStateActive, Capacity: 1000, Used: 500})
	engine.RegisterCluster(&FederationCluster{ID: "c2", State: ClusterStateActive, Capacity: 2000, Used: 1000})
	
	status := engine.GetFederationStatus()
	
	if status["total_clusters"] != 2 {
		t.Errorf("expected 2 clusters, got %v", status["total_clusters"])
	}
	if status["total_capacity"] != int64(3000) {
		t.Errorf("expected capacity 3000, got %v", status["total_capacity"])
	}
}
