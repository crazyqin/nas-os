package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	// SSOT: cfg mirror + file contain netdiag.
	if !containsStr(s.cfg.Packages.Enabled, "netdiag") {
		t.Fatalf("cfg.Packages.Enabled not synced: %v", s.cfg.Packages.Enabled)
	}
	persist := filepath.Join(cfg.Paths.DataDir, "app-center-enabled.json")
	raw, err := os.ReadFile(persist)
	if err != nil {
		t.Fatalf("SSOT file missing: %v", err)
	}
	if !strings.Contains(string(raw), "netdiag") {
		t.Fatalf("SSOT file: %s", raw)
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
	if containsStr(s.cfg.Packages.Enabled, "netdiag") {
		t.Fatalf("cfg still has netdiag after disable: %v", s.cfg.Packages.Enabled)
	}
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
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
	// After disable, routes must be inactive (503), not open.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/netdiag/history", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled package routes should be 503, got %d body=%s", w.Code, w.Body.String())
	}

	// Second enable — must not panic gin; routes active again.
	enable()
	if !s.pkgRuntime.IsLoaded("netdiag") {
		t.Fatal("second enable must load again")
	}

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
		t.Fatalf("list after re-enable missing netdiag: %v", loaded)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/netdiag/history", nil))
	if w.Code == http.StatusServiceUnavailable || w.Code == http.StatusNotFound {
		t.Fatalf("netdiag routes should work after re-enable, got %d", w.Code)
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

func TestRecommendedProductEnableDisableViaPackagesAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	s := &Server{cfg: cfg}
	r := gin.New()
	s.registerConfiguredExtensions(r.Group("/api/v1"))

	// List includes recommended products as operable product kind.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil))
	var list map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	items, _ := list["data"].(map[string]interface{})["items"].([]interface{})
	var dockerItem map[string]interface{}
	for _, raw := range items {
		it := raw.(map[string]interface{})
		if it["id"] == "docker" {
			dockerItem = it
			break
		}
	}
	if dockerItem == nil {
		t.Fatal("docker product missing from items")
	}
	if dockerItem["kind"] != "recommended_product" || dockerItem["operable"] != true {
		t.Fatalf("docker item dishonest: %v", dockerItem)
	}
	if dockerItem["loaded"] == true {
		t.Fatal("docker must not be loaded by default")
	}

	// Enable docker product.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/packages/docker/enable", nil))
	var en map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &en)
	if en["code"].(float64) != 0 {
		t.Fatalf("enable docker: %v", en)
	}
	if !s.pkgRuntime.IsLoaded("docker") {
		t.Fatal("docker not loaded")
	}

	// Disable.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/packages/docker/disable", nil))
	if s.pkgRuntime.IsLoaded("docker") {
		t.Fatal("docker still loaded after disable")
	}
}

func TestPluginsPageIsNotMockMarket(t *testing.T) {
	// Structural: plugins.html must redirect / not ship mock install list as primary.
	var data []byte
	var err error
	for _, p := range []string{
		"webui/pages/plugins.html",
		filepath.Join("..", "..", "webui", "pages", "plugins.html"),
	} {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("plugins.html not found: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "app-center") {
		t.Fatal("plugins.html must route users to app-center")
	}
	if strings.Contains(s, "const plugins = [") {
		t.Fatal("plugins.html must not keep mock plugins array as install surface")
	}
}
