package web

import (
	"context"
	"testing"

	"nas-os/internal/cluster"
	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestClusterEnableDisableInProcess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	cfg.Paths.MountBase = t.TempDir()
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())

	var started, stopped int
	s.SetClusterBootstrap(func() (*cluster.Services, error) {
		started++
		return &cluster.Services{}, nil
	})
	// Override release path: inject fake services tracker via bootstrap only.

	if s.ClusterRunning() {
		t.Fatal("cluster should not run on Core-only boot")
	}

	loaded, _, err := s.pkgRuntime.Enable(context.Background(), []string{"cluster"})
	if err != nil {
		t.Fatalf("enable cluster: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded=%v", loaded)
	}
	if started != 1 {
		t.Fatalf("bootstrap calls=%d", started)
	}
	if !s.ClusterRunning() {
		t.Fatal("cluster should be running after enable")
	}

	// Disable should clear services (ShutdownCluster on empty Services is ok).
	if err := s.pkgRuntime.Disable(context.Background(), "cluster"); err != nil {
		t.Fatal(err)
	}
	s.releaseProductManager("cluster")
	stopped++ // release path
	if s.ClusterRunning() {
		t.Fatal("cluster should not run after disable")
	}
	_ = stopped
}

func TestClusterItemNotesWhenNotRunning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	s := &Server{cfg: cfg}
	r := gin.New()
	s.registerConfiguredExtensions(r.Group("/api/v1"))
	items := s.buildPackageItems()
	var clusterItem *packageItem
	for i := range items {
		if items[i].ID == "cluster" {
			clusterItem = &items[i]
			break
		}
	}
	if clusterItem == nil {
		t.Fatal("cluster missing from items")
	}
	if clusterItem.Kind != string(config.KindRecommendedProduct) {
		t.Fatalf("kind=%s", clusterItem.Kind)
	}
	if clusterItem.Note == "" {
		t.Fatal("expected cluster note for ops")
	}
}
