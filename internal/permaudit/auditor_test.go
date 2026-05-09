package permaudit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func testAuditor() *Auditor {
	return NewAuditor(zap.NewNop())
}

func TestScanUsers_Basic(t *testing.T) {
	a := testAuditor()
	now := time.Now()
	users := []UserPerm{
		{UserID: "u1", UserName: "alice", Groups: []string{"admin"}, IsAdmin: true, LastLogin: now},
		{UserID: "u2", UserName: "bob", Groups: []string{"users"}, LastLogin: now},
	}
	report := a.ScanUsers(users)
	if report.TotalUsers != 2 {
		t.Errorf("expected 2 total users, got %d", report.TotalUsers)
	}
	if report.AdminCount != 1 {
		t.Errorf("expected 1 admin, got %d", report.AdminCount)
	}
	if report.Score > 100 || report.Score < 0 {
		t.Errorf("score out of range: %d", report.Score)
	}
}

func TestOverPrivileged(t *testing.T) {
	a := testAuditor()
	// 普通用户标记为 admin，但不在 admin 组
	u := UserPerm{UserID: "u1", UserName: "bob", Groups: []string{"users"}, IsAdmin: true}
	issues := a.CheckOverPrivileged([]UserPerm{u})
	found := false
	for _, iss := range issues {
		if iss.Type == "over-privileged" && iss.UserID == "u1" {
			found = true
		}
	}
	if !found {
		t.Error("expected over-privileged issue for u1")
	}

	// 正常管理员不应产生问题
	u2 := UserPerm{UserID: "u2", UserName: "alice", Groups: []string{"admin"}, IsAdmin: true}
	issues2 := a.CheckOverPrivileged([]UserPerm{u2})
	if len(issues2) != 0 {
		t.Errorf("expected 0 issues for legit admin, got %d", len(issues2))
	}
}

func TestOrphanPermissions(t *testing.T) {
	a := testAuditor()
	u := UserPerm{UserID: "u1", UserName: "bob", Groups: []string{"users", "deleted-group"}}
	issues := a.CheckOrphans([]UserPerm{u}, []string{"users", "admin"})
	found := false
	for _, iss := range issues {
		if iss.Type == "orphan" && iss.Resource == "deleted-group" {
			found = true
		}
	}
	if !found {
		t.Error("expected orphan issue for deleted-group")
	}
}

func TestStaleUsers(t *testing.T) {
	a := testAuditor()
	stale := UserPerm{
		UserID:    "u1",
		UserName:  "ghost",
		LastLogin: time.Now().AddDate(0, 0, -120),
	}
	fresh := UserPerm{
		UserID:    "u2",
		UserName:  "active",
		LastLogin: time.Now(),
	}
	issues := a.CheckStaleUsers([]UserPerm{stale, fresh}, 30)
	if len(issues) != 1 {
		t.Errorf("expected 1 stale issue, got %d", len(issues))
	}
	if len(issues) > 0 && issues[0].UserID != "u1" {
		t.Errorf("expected stale user u1, got %s", issues[0].UserID)
	}
}

func TestScoreCalculation(t *testing.T) {
	issues := []PermIssue{
		{Severity: "critical"},
		{Severity: "high"},
		{Severity: "medium"},
		{Severity: "low"},
	}
	score := CalculateScore(issues)
	// 100 - 15 - 10 - 5 - 2 = 68
	if score != 68 {
		t.Errorf("expected score 68, got %d", score)
	}

	// 全部 critical，20个 → 100 - 300 = 0（下限）
	bigIssues := make([]PermIssue, 20)
	for i := range bigIssues {
		bigIssues[i] = PermIssue{Severity: "critical"}
	}
	if s := CalculateScore(bigIssues); s != 0 {
		t.Errorf("expected score 0, got %d", s)
	}
}

func TestCleanSetup(t *testing.T) {
	a := testAuditor()
	now := time.Now()
	users := []UserPerm{
		{UserID: "u1", UserName: "alice", Groups: []string{"admin"}, IsAdmin: true, LastLogin: now, PasswordLen: 16},
		{UserID: "u2", UserName: "bob", Groups: []string{"users"}, LastLogin: now, PasswordLen: 12},
	}
	report := a.ScanUsers(users)
	if len(report.Issues) != 0 {
		t.Errorf("expected 0 issues for clean setup, got %d", len(report.Issues))
	}
	if report.Score != 100 {
		t.Errorf("expected score 100, got %d", report.Score)
	}
}

func setupRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)
	return r
}

func TestHandler_Scan(t *testing.T) {
	h := NewHandlers(testAuditor())
	r := setupRouter(h)

	now := time.Now()
	users := []UserPerm{
		{UserID: "u1", UserName: "alice", Groups: []string{"admin"}, IsAdmin: true, LastLogin: now},
	}
	body, _ := json.Marshal(users)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/permaudit/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var report AuditReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.TotalUsers != 1 {
		t.Errorf("expected 1 user, got %d", report.TotalUsers)
	}
}

func TestHandler_Report(t *testing.T) {
	h := NewHandlers(testAuditor())
	r := setupRouter(h)

	// 无报告时返回 404
	req := httptest.NewRequest(http.MethodGet, "/api/v1/permaudit/report", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	// 先扫描，再查报告
	users := []UserPerm{
		{UserID: "u1", UserName: "alice", Groups: []string{"admin"}, IsAdmin: true, LastLogin: time.Now()},
	}
	body, _ := json.Marshal(users)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/permaudit/scan", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatal("scan failed")
	}

	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/permaudit/report", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w3.Code)
	}
	var report AuditReport
	if err := json.Unmarshal(w3.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.TotalUsers != 1 {
		t.Errorf("expected 1 user in report, got %d", report.TotalUsers)
	}
}
