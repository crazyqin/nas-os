//go:build nasd_full

package application

import (
	"fmt"
	"log"
	"path/filepath"

	"nas-os/internal/cluster"
	"nas-os/internal/config"
	"nas-os/internal/downloader"
	"nas-os/internal/web"

	"go.uber.org/zap"
)

// attachOptionalProducts constructs cluster/downloader when those products are wanted.
func attachOptionalProducts(cfg *config.Config, logger *zap.Logger, hostname string, cleanup *cleanupStack) (clusterSvc any, downloadMgr any) {
	if cfg.OptionalProductsEnabled() || cfg.WantsProduct("cluster") {
		svc, err := cluster.InitializeCluster(cluster.RootConfig{
			Enabled: true,
			NodeID:  hostname,
			DataDir: cfg.Paths.DataDir,
		}, logger)
		if err != nil {
			log.Printf("⚠️ 集群服务初始化警告：%v", err)
		} else if svc != nil {
			cleanup.add("cluster", func() error { return cluster.ShutdownCluster(svc) })
			clusterSvc = svc
			log.Println("✅ 集群服务就绪")
		}
	}
	if cfg.OptionalProductsEnabled() || cfg.WantsProduct("downloader") {
		mgr, err := downloader.NewManager(filepath.Join(cfg.Paths.DataDir, "downloads"), logger)
		if err != nil {
			log.Printf("⚠️ 下载管理初始化警告：%v", err)
		} else {
			cleanup.add("download manager", func() error {
				mgr.Close()
				return nil
			})
			mgr.SetTransmissionURL("http://localhost:9091")
			downloadMgr = mgr
			log.Println("✅ 下载管理模块就绪")
		}
	}
	return clusterSvc, downloadMgr
}

func wireClusterIntoWeb(webServer *web.Server, cfg *config.Config, logger *zap.Logger, hostname string, clusterSvc any) {
	webServer.SetClusterServices(clusterSvc)
	webServer.SetClusterBootstrap(func() (any, error) {
		return cluster.InitializeCluster(cluster.RootConfig{
			Enabled: true,
			NodeID:  hostname,
			DataDir: cfg.Paths.DataDir,
		}, logger)
	})
}

func stopOptionalProducts(downloadManager any, clusterServices any) error {
	var errs []error
	if m, ok := downloadManager.(*downloader.Manager); ok && m != nil {
		m.Close()
	}
	if cs, ok := clusterServices.(*cluster.Services); ok && cs != nil {
		if err := cluster.ShutdownCluster(cs); err != nil {
			errs = append(errs, fmt.Errorf("stop cluster: %w", err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}

func clusterRoleOf(clusterServices any) string {
	if cs, ok := clusterServices.(*cluster.Services); ok && cs != nil && cs.HA.IsLeader() {
		return "leader"
	}
	return "follower"
}
