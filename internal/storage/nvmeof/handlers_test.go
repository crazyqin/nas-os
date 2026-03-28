package nvmeof

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestManagerAndHandlers(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(filepath.Join(dir, "nvmeof.json"))
	require.NoError(t, err)

	r := gin.New()
	NewHandlers(mgr).RegisterRoutes(r.Group("/api/v1"))

	// create target
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/nvmeof/targets", mustJSON(t, map[string]interface{}{
		"name":      "prod-target",
		"address":   "10.0.0.10",
		"port":      4420,
		"transport": "tcp",
		"enabled":   true,
		"namespaces": []map[string]interface{}{
			{
				"name":       "ns1",
				"devicePath": "/dev/nvme0n1",
				"size":       1073741824,
				"enabled":    true,
			},
		},
	}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created struct {
		Code int    `json:"code"`
		Data Target `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.Data.ID)
	require.NotEmpty(t, created.Data.NQN)

	// service start
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/nvmeof/start", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// create initiator
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/nvmeof/initiators", mustJSON(t, map[string]interface{}{
		"name":      "client-a",
		"targetNqn": created.Data.NQN,
		"address":   "10.0.0.11",
		"port":      4420,
		"transport": "tcp",
	}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var createdInitiator struct {
		Code int       `json:"code"`
		Data Initiator `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createdInitiator))

	// connect initiator
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/nvmeof/initiators/"+createdInitiator.Data.ID+"/connect", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var connected struct {
		Code int       `json:"code"`
		Data Initiator `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &connected))
	require.True(t, connected.Data.Connected)
	require.NotEmpty(t, connected.Data.DevicePath)

	// status
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/nvmeof/status", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func mustJSON(t *testing.T, v interface{}) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}
