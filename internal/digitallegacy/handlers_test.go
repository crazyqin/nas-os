package digitallegacy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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

func TestBeneficiaryLifecycle(t *testing.T) {
	_, r := setupTest(t)
	b := Beneficiary{ID: "b-1", Name: "张三", Email: "zhang@test.com", Relation: "配偶", AccessLevel: AccessFull}
	body, _ := json.Marshal(b)
	req := httptest.NewRequest(http.MethodPost, "/digital-legacy/beneficiaries", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodPost, "/digital-legacy/beneficiaries/b-1/verify", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	req3 := httptest.NewRequest(http.MethodGet, "/digital-legacy/beneficiaries/b-1", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "verified", data["status"])
}

func TestPlanSeal(t *testing.T) {
	mgr, r := setupTest(t)
	plan := LegacyPlan{ID: "p-1", OwnerID: "user-1", Instructions: "将所有文件交给张三"}
	require.NoError(t, mgr.CreatePlan(&plan))

	req := httptest.NewRequest(http.MethodPost, "/digital-legacy/plans/p-1/seal", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/digital-legacy/plans/p-1/seal", nil))
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestDeadmanCheck(t *testing.T) {
	mgr, r := setupTest(t)
	plan := LegacyPlan{
		ID: "p-deadman", OwnerID: "user-1",
		Deadman: &DeadmanConfig{
			Enabled:        true,
			InactivityDays: 30,
			LastActive:     time.Now().AddDate(0, 0, -60),
		},
	}
	require.NoError(t, mgr.CreatePlan(&plan))

	req := httptest.NewRequest(http.MethodGet, "/digital-legacy/deadman/check", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.True(t, len(data) > 0)
}

func TestStats(t *testing.T) {
	_, r := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/digital-legacy/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
