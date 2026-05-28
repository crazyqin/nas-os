package privacyscore

import (
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

func TestRunScan(t *testing.T) {
	_, r := setupTest(t)
	req := httptest.NewRequest(http.MethodPost, "/privacy-score/scan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["grade"])
	assert.True(t, data["total_score"].(float64) > 0)
}

func TestGetLatest(t *testing.T) {
	mgr, r := setupTest(t)
	mgr.RunScan()

	req := httptest.NewRequest(http.MethodGet, "/privacy-score/reports/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["grade"])
}

func TestGetLatestNoReport(t *testing.T) {
	_, r := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/privacy-score/reports/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestStats(t *testing.T) {
	mgr, r := setupTest(t)
	mgr.RunScan()

	req := httptest.NewRequest(http.MethodGet, "/privacy-score/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["total_scans"])
}
