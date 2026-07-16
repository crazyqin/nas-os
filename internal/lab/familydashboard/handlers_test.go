package familydashboard

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

func TestMemberLifecycle(t *testing.T) {
	_, r := setupTest(t)
	m := FamilyMember{ID: "m-1", Name: "爸爸", Role: RoleAdmin, Email: "dad@test.com"}
	body, _ := json.Marshal(m)
	req := httptest.NewRequest(http.MethodPost, "/family/members", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/family/members/m-1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	req3 := httptest.NewRequest(http.MethodGet, "/family/members", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}

func TestChoreLifecycle(t *testing.T) {
	_, r := setupTest(t)
	chore := Chore{ID: "ch-1", Title: "洗碗", AssigneeID: "m-1", Points: 5}
	body, _ := json.Marshal(chore)
	req := httptest.NewRequest(http.MethodPost, "/family/chores", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodPost, "/family/chores/ch-1/complete", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestAllowance(t *testing.T) {
	_, r := setupTest(t)
	a := Allowance{ID: "a-1", MemberID: "m-1", Amount: 10, Reason: "洗碗奖励", Type: "earn"}
	body, _ := json.Marshal(a)
	req := httptest.NewRequest(http.MethodPost, "/family/allowance", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/family/allowance?member_id=m-1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestNotes(t *testing.T) {
	_, r := setupTest(t)
	note := FamilyNote{ID: "n-1", AuthorID: "m-1", Title: "买菜", Content: "鸡蛋、牛奶", Color: "#FFEB3B"}
	body, _ := json.Marshal(note)
	req := httptest.NewRequest(http.MethodPost, "/family/notes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodDelete, "/family/notes/n-1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestScreenTime(t *testing.T) {
	_, r := setupTest(t)
	st := ScreenTime{MemberID: "m-1", Date: "2026-05-29", Minutes: 120, Limit: 180}
	body, _ := json.Marshal(st)
	req := httptest.NewRequest(http.MethodPost, "/family/screen-time", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/family/screen-time?member_id=m-1&date=2026-05-29", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestStats(t *testing.T) {
	_, r := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/family/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
