package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
)

func TestRegisterConfiguredExtensionsEmptyByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{cfg: config.Default()}
	r := gin.New()
	api := r.Group("/api/v1")
	s.registerConfiguredExtensions(api)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agentworkflow/tasks", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("default boot must not mount extensions, got %d", w.Code)
	}
}

func TestRegisterConfiguredExtensionsMountsAgentWorkflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Modules.Extensions = []string{"agentworkflow"}
	s := &Server{cfg: cfg}
	r := gin.New()
	api := r.Group("/api/v1")
	s.registerConfiguredExtensions(api)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agentworkflow/tasks", nil)
	r.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Fatal("agentworkflow routes should be mounted when configured")
	}
}

// TestRegisterConfiguredExtensionsRetainsActiveProtectManager proves enable is not
// construct-and-discard: status route uses retained manager.
func TestRegisterConfiguredExtensionsRetainsActiveProtectManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Modules.Extensions = []string{"activeprotect"}
	s := &Server{cfg: cfg}
	r := gin.New()
	api := r.Group("/api/v1")
	s.registerConfiguredExtensions(api)

	if s.extHolders == nil || s.extHolders.activeProtect == nil {
		t.Fatal("activeprotect manager must be retained on Server")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/activeprotect/status", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status route: %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"].(float64) != 0 {
		t.Fatalf("unexpected body %v", body)
	}
	if len(s.LoadedExtensionNames()) != 1 || s.LoadedExtensionNames()[0] != "activeprotect" {
		t.Fatalf("loaded names %v", s.LoadedExtensionNames())
	}
}

func TestRegisterConfiguredExtensionsComplianceScanRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Modules.Extensions = []string{"compliancescan", "deployorch", "netdiag"}
	s := &Server{cfg: cfg}
	r := gin.New()
	api := r.Group("/api/v1")
	s.registerConfiguredExtensions(api)

	if s.extHolders.complianceScan == nil || s.extHolders.deployOrch == nil || s.extHolders.netDiag == nil {
		t.Fatal("managers must be retained for all three extensions")
	}

	for _, path := range []string{
		"/api/v1/compliancescan/standards",
		"/api/v1/deployorch/nodes",
		"/api/v1/netdiag/history",
		"/api/v1/extensions",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code == http.StatusNotFound {
			t.Fatalf("%s not mounted", path)
		}
	}
}

func TestKnownExtensionNamesCoverDiskPackages(t *testing.T) {
	if len(KnownExtensionNames) < 7 {
		t.Fatalf("expected at least 7 known extensions, got %d", len(KnownExtensionNames))
	}
	want := map[string]bool{
		"activeprotect": true, "agentworkflow": true, "aiguardrails": true,
		"compliancescan": true, "deployorch": true, "netdiag": true, "voicehub": true,
	}
	for _, n := range KnownExtensionNames {
		if !want[n] {
			t.Fatalf("unexpected known extension %q", n)
		}
		delete(want, n)
	}
	if len(want) != 0 {
		t.Fatalf("missing known extensions: %v", want)
	}
}
