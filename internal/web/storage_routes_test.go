package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nas-os/internal/arch"
	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
)

// TestLegacyVolumesRoutesNotRegistered asserts destructive removal of /api/v1/volumes.
func TestLegacyVolumesRoutesNotRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	// Collect routes from engine
	var paths []string
	for _, r := range s.engine.Routes() {
		paths = append(paths, r.Method+" "+r.Path)
		if strings.Contains(r.Path, "/volumes") && !strings.Contains(r.Path, "/storage/") {
			// allow only if under storage - legacy is /api/v1/volumes without storage
			if strings.HasPrefix(r.Path, "/api/v1/volumes") {
				t.Fatalf("legacy volumes route still registered: %s %s", r.Method, r.Path)
			}
		}
	}
	// Explicit probe
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/volumes", nil)
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound && w.Code != http.StatusUnauthorized {
		// 404 expected when not registered; 401 if somehow hit auth wrapper only
		if w.Code == http.StatusOK {
			t.Fatalf("legacy /api/v1/volumes must not succeed, got %d", w.Code)
		}
	}
	_ = paths
	_ = arch.ModuleTierCore
}

// TestStorageContractPathRemains documents the single storage contract prefix.
func TestStorageContractPathRemains(t *testing.T) {
	// Storage handlers register under /storage/* (e.g. /storage/volumes resource names).
	// That is NOT the removed legacy /api/v1/volumes surface.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewStorageHandlers(nil)
	h.RegisterRoutes(router.Group("/api/v1"))
	found := false
	for _, r := range router.Routes() {
		if strings.Contains(r.Path, "/storage") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected /storage contract routes from NewStorageHandlers")
	}
}
