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

func TestCommunityDefaultNoDiscovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	if cfg.CommunityDir() != "" {
		t.Fatal("default community_dir must be empty")
	}
	s := &Server{cfg: cfg}
	r := gin.New()
	api := r.Group("/api/v1")
	s.registerConfiguredExtensions(api)
	if len(s.communityDiscovered) != 0 {
		t.Fatalf("discovered=%v", s.communityDiscovered)
	}
	if s.pkgRuntime == nil || len(s.pkgRuntime.LoadedIDs()) != 0 {
		t.Fatalf("loaded=%v", s.pkgRuntime.LoadedIDs())
	}
}

func TestCommunityDiscoverAndEnableViaConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	pkgDir := filepath.Join(root, "hello-host")
	if err := os.MkdirAll(pkgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "id": "com.example.hello-host",
  "version": "1.0.0",
  "trust": "local",
  "entry": "host-sdk",
  "capabilities": ["host.sdk"]
}`
	if err := os.WriteFile(filepath.Join(pkgDir, "manifest.json"), []byte(manifest), 0o640); err != nil {
		t.Fatal(err)
	}

	data := t.TempDir()
	cfg := config.Default()
	cfg.Paths.DataDir = data
	cfg.Paths.ConfigDir = t.TempDir()
	cfg.Packages.CommunityDir = root
	cfg.Packages.Enabled = []string{"com.example.hello-host"}

	s := &Server{cfg: cfg}
	r := gin.New()
	api := r.Group("/api/v1")
	s.registerConfiguredExtensions(api)

	if len(s.communityDiscovered) != 1 {
		t.Fatalf("discovered=%v", s.communityDiscovered)
	}
	loaded := s.pkgRuntime.LoadedIDs()
	if len(loaded) != 1 || loaded[0] != "com.example.hello-host" {
		t.Fatalf("loaded=%v", loaded)
	}
	marker := filepath.Join(data, "community-packages", "com.example.hello-host", "started")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("host marker missing: %v", err)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	dataObj := body["data"].(map[string]interface{})
	if dataObj["host_api_version"] == "" {
		t.Fatal("missing host_api_version")
	}
	disc, _ := dataObj["community_discovered"].([]interface{})
	if len(disc) != 1 {
		t.Fatalf("community_discovered=%v", disc)
	}
}

func TestCommunityDiscoverWithoutEnabledDoesNotLoad(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	pkgDir := filepath.Join(root, "hello-host")
	_ = os.MkdirAll(pkgDir, 0o750)
	_ = os.WriteFile(filepath.Join(pkgDir, "manifest.json"), []byte(`{
  "id": "com.example.hello-host", "trust": "local", "entry": "host-sdk"
}`), 0o640)

	cfg := config.Default()
	cfg.Packages.CommunityDir = root
	// enabled empty
	s := &Server{cfg: cfg}
	r := gin.New()
	s.registerConfiguredExtensions(r.Group("/api/v1"))
	if len(s.communityDiscovered) != 1 {
		t.Fatal("should discover")
	}
	if len(s.pkgRuntime.LoadedIDs()) != 0 {
		t.Fatal("must not auto-load community packages")
	}
}
