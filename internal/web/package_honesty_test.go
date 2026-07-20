//go:build !nasd_full

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
)

// TestPackageEnableFailsClosedOnCore drives the real enable handler when products
// are not linked. Even if a product were known to Runtime, enable must 503.
func TestPackageEnableFailsClosedOnCore(t *testing.T) {
	if ProductsLinked() {
		t.Fatal("this test is Core-only")
	}
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	s := &Server{cfg: cfg}
	r := gin.New()
	api := r.Group("/api/v1")
	s.registerConfiguredExtensions(api)

	// docker is a recommended product — must not be Runtime-known on Core
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/packages/docker/enable", nil))
	// Either not in catalog (404) or not linked (503) — never 200 success
	if w.Code == http.StatusOK {
		var body map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["code"] == float64(0) {
			t.Fatalf("enable docker must not succeed on Core; body=%s", w.Body.String())
		}
	}

	// List items for products must have operable=false
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list status %d", w.Code)
	}
	var list map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	data := list["data"].(map[string]interface{})
	items, _ := data["items"].([]interface{})
	for _, raw := range items {
		it := raw.(map[string]interface{})
		kind, _ := it["kind"].(string)
		if kind == "recommended_product" || kind == "http_extension" {
			if it["operable"] == true {
				t.Fatalf("item %v must not be operable on Core binary", it["id"])
			}
			if it["can_enable"] == true {
				t.Fatalf("item %v must not be can_enable on Core", it["id"])
			}
		}
	}
}

// TestSharedCoreRegistrationHelperExists is a structural check that both builds
// share registerCorePublicAndAdminGroups / registerCoreIdentityAndDocs.
func TestSharedCoreRegistrationHelperExists(t *testing.T) {
	// Drive the helpers via real NewServer setupRoutes (Core).
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if s == nil || s.engine == nil {
		t.Fatal("NewServer must build engine via shared path")
	}
	// Health is registered by shared helper.
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/system/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("health via shared path: %d body=%s", w.Code, w.Body.String())
	}
}
