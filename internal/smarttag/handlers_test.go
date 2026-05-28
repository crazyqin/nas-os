package smarttag

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

func TestListTags(t *testing.T) {
	_, r := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/smart-tag/tags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.True(t, len(data) > 0) // 默认标签
}

func TestCreateAndDeleteTag(t *testing.T) {
	_, r := setupTest(t)
	tag := Tag{ID: "custom-1", Name: "自定义", Color: "#FF0000"}
	body, _ := json.Marshal(tag)
	req := httptest.NewRequest(http.MethodPost, "/smart-tag/tags", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodDelete, "/smart-tag/tags/custom-1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestTagAndUntagFile(t *testing.T) {
	_, r := setupTest(t)
	reqBody := map[string]interface{}{"file_path": "/docs/report.pdf", "tag_ids": []string{"document", "important"}, "user_id": "user-1"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/smart-tag/files/tag", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/smart-tag/files?path=/docs/report.pdf", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestClassify(t *testing.T) {
	_, r := setupTest(t)
	body, _ := json.Marshal(map[string]string{"file_path": "/photos/vacation.jpg"})
	req := httptest.NewRequest(http.MethodPost, "/smart-tag/classify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.True(t, data["score"].(float64) > 0)
}

func TestBatchClassify(t *testing.T) {
	_, r := setupTest(t)
	body, _ := json.Marshal(map[string][]string{"file_paths": {"/a.jpg", "/b.mp4", "/c.pdf"}})
	req := httptest.NewRequest(http.MethodPost, "/smart-tag/classify/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSearchByTag(t *testing.T) {
	mgr, r := setupTest(t)
	require.NoError(t, mgr.TagFile("/docs/a.pdf", []string{"document"}, "user-1"))
	require.NoError(t, mgr.TagFile("/docs/b.pdf", []string{"document"}, "user-1"))
	require.NoError(t, mgr.TagFile("/photos/c.jpg", []string{"photo"}, "user-1"))

	body, _ := json.Marshal(map[string][]string{"tag_ids": {"document"}})
	req := httptest.NewRequest(http.MethodPost, "/smart-tag/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.Equal(t, 2, len(data))
}

func TestStats(t *testing.T) {
	_, r := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/smart-tag/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
