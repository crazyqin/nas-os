//go:build nasd_full

package web

import (
	"testing"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TestDefaultConfigSkipsOptionalManagers drives real NewServer with default config.
func TestDefaultConfigSkipsOptionalManagers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	if cfg.Modules.Optional {
		t.Fatal("precondition: optional must default false")
	}
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	if s.hasHolder("dockerMgr") {
		t.Fatal("dockerMgr must be nil when modules.optional=false")
	}
	if s.hasHolder("vmMgr") {
		t.Fatal("vmMgr must be nil when modules.optional=false")
	}
	if s.hasHolder("photosMgr") {
		t.Fatal("photosMgr must be nil when modules.optional=false")
	}
	if s.hasHolder("aiSvc") {
		t.Fatal("aiSvc must be nil when modules.optional=false")
	}
	if s.hasHolder("cloudsyncMgr") {
		t.Fatal("cloudsyncMgr must be nil when modules.optional=false")
	}
	if s.hasHolder("backupMgr") {
		t.Fatal("backupMgr must be nil when modules.optional=false")
	}
	// Non-catalog companions must stay nil on bare Core
	if s.hasHolder("trashMgr") || s.hasHolder("tunnelMgr") || s.hasHolder("frpManager") ||
		s.hasHolder("monitorMgr") || s.hasHolder("optimizer") || s.hasHolder("webdavSrv") ||
		s.hasHolder("ftpSrv") || s.hasHolder("sftpSrv") || s.hasHolder("replMgr") {
		t.Fatal("bulk companions must be nil on default Core surface")
	}
	if s.hasHolder("upsMgr") || s.hasHolder("wolMgr") || s.hasHolder("aclMgr") {
		t.Fatal("bulk-only ups/wol/acl must be nil on Core")
	}
}

// TestOptionalConfigConstructsDockerManager drives NewServer with optional=true.
// Docker may still be nil if daemon unavailable; we assert construction path ran
// by checking that at least one optional subsystem slot is attempted (rbac always
// on core path; when optional, pluginMarket or optimizer may be set).
func TestOptionalConfigEnablesProductConstruction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Modules.Optional = true
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	// optimizer is constructed without external daemon and is optional-gated
	if !s.hasHolder("optimizer") && !s.hasHolder("projectMgr") && !s.hasHolder("lockMgr") {
		// In optional mode many pure-Go managers should exist
		t.Fatal("expected some optional pure-Go managers when modules.optional=true")
	}
}

// TestPackagesRecommendedSystemEnablesProductConstruction drives NewServer with
// packages.recommended_system only (modules.optional left false) — ADR Stage 1.
// Catalog products construct; bulk kitchen-sink companions do not.
func TestPackagesRecommendedSystemEnablesProductConstruction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Packages.RecommendedSystem = true
	if cfg.Modules.Optional {
		t.Fatal("precondition: modules.optional false")
	}
	if !cfg.OptionalProductsEnabled() {
		t.Fatal("precondition: OptionalProductsEnabled via packages")
	}
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	// Catalog product: photos is pure-Go and should construct
	if !s.hasHolder("photosMgr") {
		t.Fatal("photosMgr should construct when packages.recommended_system=true")
	}
	// Bulk companions must stay off under recommended_system alone
	if s.hasHolder("optimizer") || s.hasHolder("projectMgr") || s.hasHolder("lockMgr") ||
		s.hasHolder("trashMgr") || s.hasHolder("tunnelMgr") || s.hasHolder("systemMonitor") {
		t.Fatal("bulk kitchen-sink managers must stay nil under recommended_system without modules.optional")
	}
}

// TestDockerOnlyDoesNotPullBulkCompanions ensures a single catalog product
// does not construct tunnel/trash/ftp companions.
func TestDockerOnlyDoesNotPullBulkCompanions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Packages.Enabled = []string{"docker"}
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	if s.hasHolder("trashMgr") || s.hasHolder("tunnelMgr") || s.hasHolder("frpManager") ||
		s.hasHolder("monitorMgr") || s.hasHolder("optimizer") || s.hasHolder("webdavSrv") {
		t.Fatal("enabling only docker must not construct bulk companions")
	}
	if s.hasHolder("photosMgr") || s.hasHolder("vmMgr") {
		t.Fatal("other catalog products must stay nil when only docker is enabled")
	}
}
