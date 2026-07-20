//go:build !nasd_full

package application

import (
	"log"

	"nas-os/internal/config"
	"nas-os/internal/web"

	"go.uber.org/zap"
)

func attachOptionalProducts(cfg *config.Config, logger *zap.Logger, hostname string, cleanup *cleanupStack) (clusterSvc any, downloadMgr any) {
	if cfg.OptionalProductsEnabled() || cfg.WantsProduct("cluster") || cfg.WantsProduct("downloader") {
		log.Println("ℹ️  core build: cluster/downloader not linked (rebuild with -tags nasd_full)")
	}
	return nil, nil
}

func wireClusterIntoWeb(webServer *web.Server, cfg *config.Config, logger *zap.Logger, hostname string, clusterSvc any) {
	// No-op: cluster types not linked in core build.
	_ = webServer
	_ = cfg
	_ = logger
	_ = hostname
	_ = clusterSvc
}

func stopOptionalProducts(downloadManager any, clusterServices any) error {
	return nil
}

func clusterRoleOf(clusterServices any) string {
	return "follower"
}
