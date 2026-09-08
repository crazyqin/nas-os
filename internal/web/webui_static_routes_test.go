package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
)

// TestWebUIStaticRoutesServed locks in the static-asset contract for pages
// served from pretty routes (/, /login, /dashboard …): those pages reference
// assets relatively (css/…, ../css, ../brand, pages/…) and app.js registers
// the PWA service worker at the origin root (/sw.js, /manifest.json), so the
// server must expose css/js/brand/pages/sw.js at the root as well, not
// only under /webui/*. sw.js precaches the root paths — that is the contract.
func TestWebUIStaticRoutesServed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	for _, d := range []string{"css", "js", "js/vendor", "brand/logo", "pages"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"css/design-system.css":      "body{}",
		"js/app.js":                  "// app",
		"js/vendor/chart.umd.min.js": "/* chart */",
		"brand/logo/logo-32.png":     "png",
		"manifest.json":              "{}",
		"sw.js":                      "// sw",
		"index.html":                 "<html></html>",
		"pages/login.html":           "<html></html>",
		"pages/storage.html":         "<html></html>",
		"pages/api-docs.html":        "<html></html>",
		"pages/plugins.html":         "<html></html>",
		"pages/containers.html":      "<html></html>",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Default()
	cfg.Modules.Optional = false
	s := &Server{cfg: cfg, engine: gin.New()}
	s.registerWebUI(root)

	for _, path := range []string{
		// Root-level assets used by pretty-route pages and the PWA.
		"/css/design-system.css",
		"/js/app.js",
		"/js/vendor/chart.umd.min.js",
		"/brand/logo/logo-32.png",
		"/manifest.json",
		"/sw.js",
		// Pages linked from the landing page ("pages/…").
		"/pages/storage.html",
		// Brand must also resolve under /webui (pages opened via /webui/pages/…).
		"/webui/brand/logo/logo-32.png",
		"/webui/js/vendor/chart.umd.min.js",
		"/webui/pages/storage.html",
		// Existing contract must keep working.
		"/login",
		"/api-docs",
		"/plugins",
		"/webui/css/design-system.css",
	} {
		w := httptest.NewRecorder()
		s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, w.Code)
		}
	}

	// Core gating must keep applying at the root: optional pages stay 404.
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/pages/containers.html", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("GET /pages/containers.html = %d, want 404 (core gate)", w.Code)
	}
}
