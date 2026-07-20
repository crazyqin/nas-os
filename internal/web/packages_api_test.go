package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		t.Fatal("items should list official catalog even when none loaded")
	}
	for _, raw := range items {
		it := raw.(map[string]interface{})
		if it["loaded"] == true {
			t.Fatalf("item %v loaded by default", it["id"])
		}
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
