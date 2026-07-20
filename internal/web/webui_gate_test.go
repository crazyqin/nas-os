package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestOptionalWebUIPagesBlockedWhenOptionalFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Create temp webui tree
	root := t.TempDir()
	pages := filepath.Join(root, "pages")
	_ = os.MkdirAll(pages, 0o755)
	_ = os.MkdirAll(filepath.Join(root, "css"), 0o755)
	_ = os.WriteFile(filepath.Join(pages, "containers.html"), []byte("optional"), 0o644)
	_ = os.WriteFile(filepath.Join(pages, "storage.html"), []byte("core"), 0o644)
	_ = os.WriteFile(filepath.Join(pages, "app-center.html"), []byte("app-center-core"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "index.html"), []byte("index"), 0o644)

	cfg := config.Default()
	cfg.Modules.Optional = false
	s := &Server{cfg: cfg, engine: gin.New()}
	// Register using temp root (duplicate of registerWebUI logic with test root)
	s.registerWebUI(root)

	// Optional page blocked
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/webui/pages/containers.html", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("containers.html should be 404 when optional=false, got %d body=%s", w.Code, w.Body.String())
	}

	// Core page allowed
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/webui/pages/storage.html", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("storage.html should be OK, got %d", w.Code)
	}

	// Application Center is Core surface (user-clickable packages UI).
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/webui/pages/app-center.html", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("app-center.html should be OK on Core surface, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestOptionalWebUIPagesServedWhenOptionalTrue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	pages := filepath.Join(root, "pages")
	_ = os.MkdirAll(pages, 0o755)
	_ = os.MkdirAll(filepath.Join(root, "css"), 0o755)
	_ = os.WriteFile(filepath.Join(pages, "containers.html"), []byte("optional-ok"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "index.html"), []byte("index"), 0o644)

	cfg := config.Default()
	cfg.Modules.Optional = true
	s := &Server{cfg: cfg, engine: gin.New()}
	s.registerWebUI(root)

	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/webui/pages/containers.html", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("containers.html should be OK when optional=true, got %d", w.Code)
	}
	_ = zap.NewNop
}
