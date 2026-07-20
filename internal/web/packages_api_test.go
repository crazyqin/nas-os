package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
)

func TestPackagesListDefaultCoreOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	s := &Server{cfg: cfg}
	r := gin.New()
	s.registerConfiguredExtensions(r.Group("/api/v1"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	data := body["data"].(map[string]interface{})
	loaded, _ := data["loaded"].([]interface{})
	if len(loaded) != 0 {
		t.Fatalf("default loaded must be empty, got %v", loaded)
	}
	items, _ := data["items"].([]interface{})
	if len(items) == 0 {
		t.Fatal("items should list official HTTP extensions even when none loaded")
	}
	// None of the official items should be loaded by default.
	for _, raw := range items {
		it := raw.(map[string]interface{})
		if it["loaded"] == true {
			t.Fatalf("item %v loaded by default", it["id"])
		}
	}
}

func TestPackageEnableDisableUpdatesLoaded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	s := &Server{cfg: cfg}
	r := gin.New()
	s.registerConfiguredExtensions(r.Group("/api/v1"))

	// Enable official HTTP extension via real handler path.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packages/netdiag/enable", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("enable status %d %s", w.Code, w.Body.String())
	}
	var enBody map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &enBody)
	if enBody["code"].(float64) != 0 {
		t.Fatalf("enable body %v", enBody)
	}
	if !s.pkgRuntime.IsLoaded("netdiag") {
		t.Fatal("netdiag must be loaded after enable")
	}

	// List reflects loaded.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil))
	var list map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	data := list["data"].(map[string]interface{})
	loaded, _ := data["loaded"].([]interface{})
	found := false
	for _, id := range loaded {
		if id == "netdiag" {
			found = true
		}
	}
	if !found {
		t.Fatalf("list loaded=%v missing netdiag", loaded)
	}

	// Disable.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/packages/netdiag/disable", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("disable %d %s", w.Code, w.Body.String())
	}
	if s.pkgRuntime.IsLoaded("netdiag") {
		t.Fatal("netdiag must not be loaded after disable")
	}

	// Persistence file written under data dir.
	persist := filepath.Join(cfg.Paths.DataDir, "app-center-enabled.json")
	// After disable, file may exist with empty list — either ok.
	if _, err := os.Stat(persist); err == nil {
		raw, _ := os.ReadFile(persist)
		t.Logf("persist file: %s", raw)
	}
}

// TestPackageReEnableAfterDisableNoPanic drives Enable→Disable→Enable on a real
// official HTTP extension. Second enable must not panic gin (duplicate routes)
// and must report loaded again.
func TestPackageReEnableAfterDisableNoPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	s := &Server{cfg: cfg}
	r := gin.New()
	s.registerConfiguredExtensions(r.Group("/api/v1"))

	enable := func() {
		t.Helper()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/packages/netdiag/enable", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("enable status %d %s", w.Code, w.Body.String())
		}
		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["code"].(float64) != 0 {
			t.Fatalf("enable code=%v msg=%v", body["code"], body["message"])
		}
	}
	disable := func() {
		t.Helper()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/packages/netdiag/disable", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("disable status %d %s", w.Code, w.Body.String())
		}
	}

	enable()
	if !s.pkgRuntime.IsLoaded("netdiag") {
		t.Fatal("first enable")
	}
	disable()
	if s.pkgRuntime.IsLoaded("netdiag") {
		t.Fatal("after disable")
	}
	// Second enable — previously panicked gin on duplicate /netdiag/* routes.
	enable()
	if !s.pkgRuntime.IsLoaded("netdiag") {
		t.Fatal("second enable must load again")
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil))
	var list map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	data := list["data"].(map[string]interface{})
	loaded, _ := data["loaded"].([]interface{})
	found := false
	for _, id := range loaded {
		if id == "netdiag" {
			found = true
		}
	}
	if !found {
		t.Fatalf("list after re-enable missing netdiag: %v", loaded)
	}
	// Route still serves (first mount retained).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/netdiag/history", nil))
	if w.Code == http.StatusNotFound {
		t.Fatal("netdiag routes should still be mounted after re-enable")
	}
}

func TestPackageEnableCommunityFixture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	pkgDir := filepath.Join(root, "hello")
	_ = os.MkdirAll(pkgDir, 0o750)
	_ = os.WriteFile(filepath.Join(pkgDir, "manifest.json"), []byte(`{
  "id": "com.example.hello-host",
  "trust": "local",
  "entry": "host-sdk",
  "description": "fixture"
}`), 0o640)

	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	cfg.Packages.CommunityDir = root
	s := &Server{cfg: cfg}
	r := gin.New()
	s.registerConfiguredExtensions(r.Group("/api/v1"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/packages/com.example.hello-host/enable", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("enable %d %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"].(float64) != 0 {
		t.Fatalf("%v", body)
	}
	if !s.pkgRuntime.IsLoaded("com.example.hello-host") {
		t.Fatal("community package not loaded")
	}

	// Disable community package.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/packages/com.example.hello-host/disable", nil))
	if s.pkgRuntime.IsLoaded("com.example.hello-host") {
		t.Fatal("still loaded after disable")
	}
}

func TestAppCenterPageOnCoreAllowlist(t *testing.T) {
	if !coreWebUIPages["app-center.html"] {
		t.Fatal("app-center.html must be on Core WebUI allowlist")
	}
}
