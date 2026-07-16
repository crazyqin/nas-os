package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestDefaultWebUIDoesNotFetchLegacyVolumesAPI asserts core-served webui assets
// call /api/v1/storage/volumes (or relative storage/volumes), never legacy /api/v1/volumes.
func TestDefaultWebUIDoesNotFetchLegacyVolumesAPI(t *testing.T) {
	root := filepath.Join("..", "..", "webui")
	// Prefer repo-relative from package dir
	if _, err := os.Stat(root); err != nil {
		// when tests run from module root via go test ./internal/web
		root = filepath.Join("webui")
		if _, err := os.Stat(root); err != nil {
			// try absolute from caller
			wd, _ := os.Getwd()
			// internal/web -> repo root
			root = filepath.Join(wd, "..", "..", "webui")
		}
	}

	coreFiles := []string{
		filepath.Join(root, "index.html"),
		filepath.Join(root, "pages", "storage.html"),
		filepath.Join(root, "sw.js"),
	}
	legacyNeedle := []string{
		"`${API_BASE}/volumes`",
		"${API_BASE}/volumes",
		"/api/v1/volumes",
		`'/api/v1/volumes'`,
		`"/api/v1/volumes"`,
	}
	// Allowed modern forms
	// storage/volumes under API_BASE is correct

	for _, f := range coreFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		text := string(data)
		for _, needle := range legacyNeedle {
			// Allow /api/v1/storage/volumes which contains the substring /volumes after storage/
			// Check line-by-line for actual legacy paths
			for i, line := range strings.Split(text, "\n") {
				if !strings.Contains(line, "volumes") {
					continue
				}
				// skip comments
				trim := strings.TrimSpace(line)
				if strings.HasPrefix(trim, "//") || strings.HasPrefix(trim, "/*") {
					continue
				}
				// filesystem path placeholders like /volumes/data are not API
				if strings.Contains(line, "/volumes/data") || strings.Contains(line, "/volumes/backup") {
					continue
				}
				if strings.Contains(line, "/api/v1/storage/volumes") || strings.Contains(line, "${API_BASE}/storage/volumes") {
					continue
				}
				if strings.Contains(line, "/api/v1/volumes") || strings.Contains(line, "${API_BASE}/volumes") {
					t.Errorf("%s:%d still references legacy volumes API: %s", f, i+1, strings.TrimSpace(line))
				}
				_ = needle
			}
		}
	}
}

// TestStorageHandlersRegisterStorageVolumesContract drives RegisterRoutes and
// asserts /storage/volumes is registered while bare /volumes is not.
func TestStorageHandlersRegisterStorageVolumesContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewStorageHandlers(nil).RegisterRoutes(router.Group("/api/v1"))

	var hasStorage, hasLegacy bool
	for _, r := range router.Routes() {
		if strings.Contains(r.Path, "/storage/volumes") {
			hasStorage = true
		}
		if r.Path == "/api/v1/volumes" || strings.HasPrefix(r.Path, "/api/v1/volumes/") {
			hasLegacy = true
		}
	}
	if !hasStorage {
		t.Fatal("expected /api/v1/storage/volumes routes")
	}
	if hasLegacy {
		t.Fatal("legacy /api/v1/volumes must not be registered")
	}
}
