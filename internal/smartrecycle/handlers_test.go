package smartrecycle

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (*Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mgr := NewManager(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, mgr.Initialize())
	r := gin.New()
	NewHandlers(mgr).RegisterRoutes(r.Group(""))
	return mgr, r
}

func TestScan(t *testing.T) {
	_, r := setupTest(t)
	body, _ := json.Marshal(map[string]string{"path": "/data"})
	req := httptest.NewRequest(http.MethodPost, "/smart-recycle/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.True(t, data["total_files"].(float64) > 0)
}

func TestScanEmptyPath(t *testing.T) {
	_, r := setupTest(t)
	body, _ := json.Marshal(map[string]string{"path": ""})
	req := httptest.NewRequest(http.MethodPost, "/smart-recycle/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCleanup(t *testing.T) {
	mgr, r := setupTest(t)
	result, _ := mgr.ScanPath("/data", nil)

	body, _ := json.Marshal(map[string][]string{"item_ids": {result.Items[0].ID, result.Items[1].ID}})
	req := httptest.NewRequest(http.MethodPost, "/smart-recycle/scans/"+result.ID+"/cleanup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	report := resp["data"].(map[string]interface{})
	assert.True(t, report["deleted"].(float64) > 0)
}

func TestStats(t *testing.T) {
	_, r := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/smart-recycle/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetPolicy(t *testing.T) {
	_, r := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/smart-recycle/policy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
