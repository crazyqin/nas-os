package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newCompatStorageNilManagerRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewStorageHandlers(nil).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func requestRawJSON(t *testing.T, router http.Handler, method, path string) (int, interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	router.ServeHTTP(w, req)
	var body interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v; body=%s", err, w.Body.String())
	}
	return w.Code, body
}

func assertEnvelope(t *testing.T, envelope map[string]interface{}, wantCode float64, wantMsg string) {
	t.Helper()
	if envelope["code"] != wantCode {
		t.Fatalf("code = %v, want %v", envelope["code"], wantCode)
	}
	if msg, _ := envelope["message"].(string); msg != wantMsg && wantMsg != "" {
		// message may vary; only enforce code when wantMsg empty
		if wantMsg != "" && msg != wantMsg {
			t.Fatalf("message = %q, want %q", msg, wantMsg)
		}
	}
}

func TestCompatStorageNilManagerVolumesContract(t *testing.T) {
	status, body := requestRawJSON(t, newCompatStorageNilManagerRouter(), http.MethodGet, "/api/v1/storage/volumes")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%v", status, http.StatusOK, body)
	}
	envelope, ok := body.(map[string]interface{})
	if !ok {
		t.Fatalf("body type = %T, want envelope object; body=%v", body, body)
	}
	assertEnvelope(t, envelope, 0, "success")
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("data type = %T, want array; body=%v", envelope["data"], envelope)
	}
	if len(data) != 0 {
		t.Fatalf("data len = %d, want 0; body=%v", len(data), envelope)
	}
}

func TestCompatStorageNilManagerPoolsContract(t *testing.T) {
	status, body := requestRawJSON(t, newCompatStorageNilManagerRouter(), http.MethodGet, "/api/v1/storage/pools")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%v", status, http.StatusOK, body)
	}
	envelope, ok := body.(map[string]interface{})
	if !ok {
		t.Fatalf("body type = %T, want envelope object; body=%v", body, body)
	}
	assertEnvelope(t, envelope, 0, "success")
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("data type = %T, want array; body=%v", envelope["data"], envelope)
	}
	if len(data) != 0 {
		t.Fatalf("data len = %d, want 0; body=%v", len(data), envelope)
	}
}

func TestCompatStorageNilManagerSnapshotsContract(t *testing.T) {
	status, body := requestRawJSON(t, newCompatStorageNilManagerRouter(), http.MethodGet, "/api/v1/storage/snapshots")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%v", status, http.StatusOK, body)
	}
	envelope, ok := body.(map[string]interface{})
	if !ok {
		t.Fatalf("body type = %T, want envelope object; body=%v", body, body)
	}
	assertEnvelope(t, envelope, 0, "success")
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("data type = %T, want array; body=%v", envelope["data"], envelope)
	}
	if len(data) != 0 {
		t.Fatalf("data len = %d, want 0; body=%v", len(data), envelope)
	}
}
