package homelab

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
	tmpDir := t.TempDir()
	mgr := NewManager(filepath.Join(tmpDir, "data.json"))
	require.NoError(t, mgr.Initialize())
	r := gin.New()
	grp := r.Group("")
	NewHandlers(mgr).RegisterRoutes(grp)
	return mgr, r
}

func TestCreateAndListServices(t *testing.T) {
	_, r := setupTest(t)

	svc := Service{ID: "svc-1", Name: "Test NGINX", Type: ServiceDocker, Image: "nginx:latest", Port: 80}
	body, _ := json.Marshal(svc)
	req := httptest.NewRequest(http.MethodPost, "/homelab/services", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/homelab/services", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])
}

func TestServiceLifecycle(t *testing.T) {
	mgr, r := setupTest(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "svc-2", Name: "Redis", Type: ServiceDocker, Image: "redis:latest"}))

	req := httptest.NewRequest(http.MethodPost, "/homelab/services/svc-2/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	svc, _ := mgr.GetService("svc-2")
	assert.Equal(t, StatusRunning, svc.Status)

	req2 := httptest.NewRequest(http.MethodPost, "/homelab/services/svc-2/stop", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	svc, _ = mgr.GetService("svc-2")
	assert.Equal(t, StatusStopped, svc.Status)
}

func TestDuplicateService(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "dup-1", Name: "A", Type: ServiceDocker}))
	err := mgr.CreateService(&Service{ID: "dup-1", Name: "B", Type: ServiceDocker})
	assert.ErrorIs(t, err, ErrServiceExists)
}

func TestTemplates(t *testing.T) {
	_, r := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/homelab/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["total"].(float64) > 0)
}

func TestDeployFromTemplate(t *testing.T) {
	_, r := setupTest(t)
	reqBody := map[string]string{"name": "my-nextcloud"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/homelab/templates/nextcloud/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStackLifecycle(t *testing.T) {
	mgr, r := setupTest(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "stack-svc-1", Name: "Web", Type: ServiceDocker}))
	require.NoError(t, mgr.CreateService(&Service{ID: "stack-svc-2", Name: "DB", Type: ServiceDocker}))

	stack := Stack{ID: "stack-1", Name: "WebApp", Services: []string{"stack-svc-1", "stack-svc-2"}}
	body, _ := json.Marshal(stack)
	req := httptest.NewRequest(http.MethodPost, "/homelab/stacks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodPost, "/homelab/stacks/stack-1/start", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	s, _ := mgr.GetStack("stack-1")
	assert.Equal(t, StatusRunning, s.Status)
}

func TestStats(t *testing.T) {
	mgr, r := setupTest(t)
	require.NoError(t, mgr.CreateService(&Service{ID: "stat-1", Name: "A", Type: ServiceDocker}))
	require.NoError(t, mgr.StartService("stat-1"))

	req := httptest.NewRequest(http.MethodGet, "/homelab/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["running"])
}
