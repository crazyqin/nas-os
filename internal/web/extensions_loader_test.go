package web

import (
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

	// No extension route should exist when list is empty.
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
	// Handler is registered; unauthenticated may still return 200 with empty list or 4xx from handler logic.
	if w.Code == http.StatusNotFound {
		t.Fatal("agentworkflow routes should be mounted when configured")
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
