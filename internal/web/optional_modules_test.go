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
	if s.dockerMgr != nil {
		t.Fatal("dockerMgr must be nil when modules.optional=false")
	}
	if s.vmMgr != nil {
		t.Fatal("vmMgr must be nil when modules.optional=false")
	}
	if s.photosMgr != nil {
		t.Fatal("photosMgr must be nil when modules.optional=false")
	}
	if s.aiSvc != nil {
		t.Fatal("aiSvc must be nil when modules.optional=false")
	}
	if s.cloudsyncMgr != nil {
		t.Fatal("cloudsyncMgr must be nil when modules.optional=false")
	}
	if s.backupMgr != nil {
		t.Fatal("backupMgr must be nil when modules.optional=false")
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
	if s.optimizer == nil && s.projectMgr == nil && s.lockMgr == nil {
		// In optional mode many pure-Go managers should exist
		t.Fatal("expected some optional pure-Go managers when modules.optional=true")
	}
}

// TestPackagesRecommendedSystemEnablesProductConstruction drives NewServer with
// packages.recommended_system only (modules.optional left false) — ADR Stage 1.
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
	if s.optimizer == nil && s.projectMgr == nil && s.lockMgr == nil {
		t.Fatal("expected optional pure-Go managers when packages.recommended_system=true")
	}
}
