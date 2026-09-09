//go:build nasd_full

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
)

// TestSpaceAnalysisPageServedOnFull locks the space-analysis surface contract:
// when products are linked AND optional UI is enabled, the pretty route
// /space-analysis (used by the landing-page HEAD probe) and the /pages path
// must serve the page.
func TestSpaceAnalysisPageServedOnFull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	for _, d := range []string{"css", "js", "js/vendor", "brand/logo", "pages"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"index.html":                "<html></html>",
		"manifest.json":             "{}",
		"sw.js":                     "// sw",
		"pages/space-analysis.html": "<html>space</html>",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Default()
	cfg.Modules.Optional = true
	if !ProductsLinked() {
		t.Fatal("precondition: full build must link products")
	}
	s := &Server{cfg: cfg, engine: gin.New()}
	s.registerWebUI(root)

	for _, path := range []string{"/space-analysis", "/pages/space-analysis.html"} {
		w := httptest.NewRecorder()
		s.engine.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, w.Code)
		}
	}

	// The landing-page entry probe uses HEAD on the pretty route.
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodHead, "/space-analysis", nil))
	if w.Code != http.StatusOK {
		t.Errorf("HEAD /space-analysis = %d, want 200", w.Code)
	}
}
