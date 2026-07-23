//go:build nasd_full

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TestNewServerDockerOnlyNoPanic uses writable TempDir paths so product
// construction succeeds and must not panic on duplicate /system/info routes.
func TestNewServerDockerOnlyNoPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	cfg.Paths.MountBase = t.TempDir()
	cfg.Packages.Enabled = []string{"docker"}
	cfg.Packages.RecommendedSystem = false

	// Must not panic.
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	if s == nil {
		t.Fatal("nil server")
	}
	if !s.hasHolder("dockerMgr") {
		// Docker daemon may be unavailable; isolation still requires no panic
		// and that bulk products are not constructed.
		t.Log("dockerMgr nil (daemon unavailable) — OK if isolation holds")
	}
	// Per-product isolation: only docker wanted → no photos/vm/backup/ai managers.
	if s.hasHolder("photosMgr") {
		t.Fatal("photosMgr must not construct when only docker enabled")
	}
	if s.hasHolder("vmMgr") {
		t.Fatal("vmMgr must not construct when only docker enabled")
	}
	if s.hasHolder("backupMgr") {
		t.Fatal("backupMgr must not construct when only docker enabled")
	}
	if s.hasHolder("aiSvc") {
		t.Fatal("aiSvc must not construct when only docker enabled")
	}
	if s.hasHolder("cloudsyncMgr") {
		t.Fatal("cloudsyncMgr must not construct when only docker enabled")
	}
	// systemMonitor is bulk-only
	if s.hasHolder("systemMonitor") {
		t.Fatal("systemMonitor is bulk-only; must be nil for docker-only")
	}

	// Routes register without panic.
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil))
	// May be 401 without auth depending on middleware — must not 500 from panic.
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("system/info 500: %s", w.Body.String())
	}
}

// TestNewServerRecommendedSystemNoPanic constructs bulk surface with writable paths.
// Avoid hanging on background workers: only assert NewServer returns and route tree built.
func TestNewServerRecommendedSystemNoPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	cfg.Paths.MountBase = t.TempDir()
	cfg.Packages.RecommendedSystem = true

	done := make(chan *Server, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("NewServer panicked: %v", r)
				done <- nil
			}
		}()
		done <- NewServer(cfg, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	}()
	var s *Server
	select {
	case s = <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("NewServer hung under recommended_system")
	}
	if s == nil {
		t.Fatal("nil server")
	}
	// Legacy .so host still off by default.
	if s.hasHolder("pluginMgr") {
		t.Fatal("pluginMgr must stay nil without legacy_so_plugins")
	}
	// Route registration completed if engine has routes (no panic during setupRoutes).
	if s.engine == nil {
		t.Fatal("nil engine")
	}
}

func TestBootWantProductsDockerOnly(t *testing.T) {
	cfg := config.Default()
	cfg.Packages.Enabled = []string{"docker", "voicehub"}
	cfg.Packages.RecommendedSystem = false
	want := bootWantProducts(cfg)
	if !want["docker"] {
		t.Fatal("want docker")
	}
	if want["vm"] || want["photos"] {
		t.Fatalf("unexpected products: %v", want)
	}
	// voicehub is HTTP extension, not recommended product
	if want["voicehub"] {
		t.Fatal("voicehub is not a recommended product")
	}
	_ = filepath.Separator
}

// TestRecommendedSystemIsNotBulkKitchenSink: packages.recommended_system enables
// catalog products only — not modules.optional bulk companions.
func TestRecommendedSystemIsNotBulkKitchenSink(t *testing.T) {
	cfg := config.Default()
	cfg.Packages.RecommendedSystem = true
	cfg.Modules.Optional = false
	if productBulkSurface(cfg) {
		t.Fatal("recommended_system must not enable bulk kitchen-sink surface")
	}
	want := bootWantProducts(cfg)
	for _, id := range config.RecommendedSystemPackageIDs() {
		if !want[id] {
			t.Fatalf("missing product %s", id)
		}
	}
}

// TestRuntimeEnablePhotosConstructsManager drives Runtime Enable for photos
// (same path as packages API after auth) and asserts manager construction.
func TestRuntimeEnablePhotosConstructsManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	cfg.Paths.MountBase = t.TempDir()
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	if s.hasHolder("photosMgr") {
		t.Fatal("photos must be nil on Core-only boot")
	}
	if s.pkgRuntime == nil {
		t.Fatal("pkgRuntime required")
	}
	loaded, _, err := s.pkgRuntime.Enable(context.Background(), []string{"photos"})
	if err != nil {
		t.Fatalf("Enable photos: %v", err)
	}
	if len(loaded) != 1 || loaded[0] != "photos" {
		t.Fatalf("loaded=%v", loaded)
	}
	if !s.hasHolder("photosMgr") {
		t.Fatal("photos manager must be constructed on runtime enable")
	}
	if !s.pkgRuntime.IsLoaded("photos") {
		t.Fatal("photos not loaded")
	}

	// Disable releases manager.
	if err := s.pkgRuntime.Disable(context.Background(), "photos"); err != nil {
		t.Fatal(err)
	}
	s.releaseProductManager("photos")
	if s.hasHolder("photosMgr") {
		t.Fatal("photos manager must be nil after release")
	}
}
