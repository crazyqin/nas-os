package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"nas-os/internal/arch"
	"nas-os/internal/config"
	"nas-os/internal/storage"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TestCoreServer_ModulesPlusStorageMgr_NoDoubleRegisterPanic constructs Server
// like production: storage Module (RouteRegistrar) + non-nil storageMgr, then
// setupRoutes. Must not panic with gin "handlers are already registered".
func TestCoreServer_ModulesPlusStorageMgr_NoDoubleRegisterPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mount := t.TempDir()
	mgr, err := storage.NewManager(mount)
	if err != nil {
		t.Logf("storage.NewManager: %v — empty manager still proves dual-register safety", err)
		mgr = &storage.Manager{}
	}

	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	cfg.Paths.MountBase = mount

	// Mimic application storageModule: implements RouteRegistrar but registers nothing.
	modules := []arch.Module{&storageRouteModule{name: "storage"}}

	var s *Server
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("setupRoutes panicked (likely double storage register): %v", r)
			}
		}()
		s = NewServer(cfg, modules, mgr, nil, nil, nil, nil, nil, nil, zap.NewNop())
	}()
	if s == nil || s.engine == nil {
		t.Fatal("nil server")
	}

	// Storage contract must be registered exactly once (admin auth → 401 without token).
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/storage/volumes", nil))
	if w.Code == http.StatusNotFound {
		t.Fatal("expected /api/v1/storage/volumes registered (got 404)")
	}
	// Unauthenticated admin route → 401 is success for "route exists"
	if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden && w.Code != http.StatusOK {
		// 500 with "already registered" would be failure
		if w.Code == http.StatusInternalServerError {
			t.Fatalf("unexpected 500: %s", w.Body.String())
		}
	}

	// DELETE path also registered (auth gate only)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/storage/volumes/tank", nil))
	if w.Code == http.StatusNotFound {
		t.Fatal("expected DELETE /api/v1/storage/volumes/:name registered")
	}
}

// storageRouteModule mimics application storageModule after single-owner fix.
type storageRouteModule struct{ name string }

func (m *storageRouteModule) Name() string                    { return m.name }
func (m *storageRouteModule) Tier() arch.ModuleTier           { return arch.ModuleTierCore }
func (m *storageRouteModule) Dependencies() []string          { return nil }
func (m *storageRouteModule) Init(context.Context) error      { return nil }
func (m *storageRouteModule) Start(context.Context) error     { return nil }
func (m *storageRouteModule) Stop(context.Context) error      { return nil }
func (m *storageRouteModule) Health(context.Context) error    { return nil }
func (m *storageRouteModule) RegisterRoutes(*gin.RouterGroup) {}
