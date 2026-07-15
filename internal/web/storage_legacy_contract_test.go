package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nas-os/internal/storage"

	"github.com/gin-gonic/gin"
)

func newLegacyStorageTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerLegacyStorageRoutes(router.Group("/api/v1"), &Server{storageMgr: &storage.Manager{}})
	return router
}

func requestJSON(t *testing.T, router http.Handler, method, path string) (int, map[string]interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	router.ServeHTTP(w, req)
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v; body=%s", err, w.Body.String())
	}
	return w.Code, body
}

func assertEnvelope(t *testing.T, got map[string]interface{}, code float64, message string) {
	t.Helper()
	if got["code"] != code {
		t.Fatalf("code = %#v, want %#v; body=%v", got["code"], code, got)
	}
	if got["message"] != message {
		t.Fatalf("message = %#v, want %#v; body=%v", got["message"], message, got)
	}
}

func TestLegacyStorageListVolumesEmptyContract(t *testing.T) {
	status, body := requestJSON(t, newLegacyStorageTestRouter(), http.MethodGet, "/api/v1/volumes")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%v", status, http.StatusOK, body)
	}
	assertEnvelope(t, body, 0, "success")
	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatalf("data type = %T, want array; body=%v", body["data"], body)
	}
	if len(data) != 0 {
		t.Fatalf("data len = %d, want 0; data=%v", len(data), data)
	}
}

func TestLegacyStorageMissingVolumeContract(t *testing.T) {
	status, body := requestJSON(t, newLegacyStorageTestRouter(), http.MethodGet, "/api/v1/volumes/missing")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%v", status, http.StatusNotFound, body)
	}
	assertEnvelope(t, body, 404, "卷不存在")
	if _, exists := body["data"]; exists {
		t.Fatalf("missing volume response must not include data; body=%v", body)
	}
}

func TestLegacyStorageRAIDConfigsContract(t *testing.T) {
	status, body := requestJSON(t, newLegacyStorageTestRouter(), http.MethodGet, "/api/v1/raid-configs")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%v", status, http.StatusOK, body)
	}
	assertEnvelope(t, body, 0, "success")
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data type = %T, want object; body=%v", body["data"], body)
	}
	for _, profile := range []string{"single", "raid0", "raid1", "raid10", "raid5", "raid6"} {
		if _, ok := data[profile]; !ok {
			t.Fatalf("missing RAID profile %q in %v", profile, data)
		}
	}
}

func TestLegacyStorageMissingSubvolumeListContract(t *testing.T) {
	status, body := requestJSON(t, newLegacyStorageTestRouter(), http.MethodGet, "/api/v1/volumes/missing/subvolumes")
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%v", status, http.StatusInternalServerError, body)
	}
	assertEnvelope(t, body, 500, "卷 missing 不存在")
	if _, exists := body["data"]; exists {
		t.Fatalf("error response must not include data; body=%v", body)
	}
}

func TestLegacyStorageMissingSnapshotListContract(t *testing.T) {
	status, body := requestJSON(t, newLegacyStorageTestRouter(), http.MethodGet, "/api/v1/volumes/missing/snapshots")
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%v", status, http.StatusInternalServerError, body)
	}
	assertEnvelope(t, body, 500, "卷 missing 不存在")
	if _, exists := body["data"]; exists {
		t.Fatalf("error response must not include data; body=%v", body)
	}
}
