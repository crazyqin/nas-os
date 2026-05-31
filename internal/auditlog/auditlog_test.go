package auditlog

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTest(t *testing.T) (*Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mgr := NewManager()
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandler(mgr)
	h.RegisterRoutes(rg)
	return mgr, r
}

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestAddEntry(t *testing.T) {
	mgr := NewManager()
	mgr.AddEntry(AuditEntry{
		Level:    LevelInfo,
		Source:   SourceAuth,
		User:     "admin",
		Action:   "login",
		Resource: "/api/login",
		IP:       "192.168.1.100",
		Success:  true,
	})
	entries := mgr.Query(LogFilter{})
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestQueryFilter(t *testing.T) {
	mgr := NewManager()
	mgr.AddEntry(AuditEntry{Level: LevelInfo, Source: SourceAuth, User: "admin", Action: "login", Success: true})
	mgr.AddEntry(AuditEntry{Level: LevelError, Source: SourceSystem, User: "root", Action: "error", Success: false})
	mgr.AddEntry(AuditEntry{Level: LevelInfo, Source: SourceStorage, User: "admin", Action: "read", Success: true})

	// 按级别过滤
	entries := mgr.Query(LogFilter{Level: LevelError})
	if len(entries) != 1 {
		t.Errorf("expected 1 error entry, got %d", len(entries))
	}

	// 按来源过滤
	entries = mgr.Query(LogFilter{Source: SourceAuth})
	if len(entries) != 1 {
		t.Errorf("expected 1 auth entry, got %d", len(entries))
	}

	// 按用户过滤
	entries = mgr.Query(LogFilter{User: "admin"})
	if len(entries) != 2 {
		t.Errorf("expected 2 admin entries, got %d", len(entries))
	}

	// 限制数量
	entries = mgr.Query(LogFilter{Limit: 1})
	if len(entries) != 1 {
		t.Errorf("expected 1 entry with limit, got %d", len(entries))
	}
}

func TestAnomalies(t *testing.T) {
	mgr := NewManager()
	// 模拟5次失败登录触发暴力破解检测
	for i := 0; i < 6; i++ {
		mgr.AddEntry(AuditEntry{
			Level:   LevelWarning,
			Source:  SourceAuth,
			User:    "admin",
			Action:  "login",
			IP:      "10.0.0.1",
			Success: false,
		})
	}
	anomalies := mgr.GetAnomalies(false)
	if len(anomalies) == 0 {
		t.Error("expected anomalies from brute force detection")
	}
}

func TestResolveAnomaly(t *testing.T) {
	mgr := NewManager()
	// 先触发异常
	for i := 0; i < 6; i++ {
		mgr.AddEntry(AuditEntry{Level: LevelWarning, Source: SourceAuth, Action: "login", IP: "10.0.0.1", Success: false})
	}
	anomalies := mgr.GetAnomalies(false)
	if len(anomalies) > 0 {
		err := mgr.ResolveAnomaly(anomalies[0].ID)
		if err != nil {
			t.Fatalf("ResolveAnomaly failed: %v", err)
		}
		resolved := mgr.GetAnomalies(true)
		if len(resolved) == 0 {
			t.Error("expected resolved anomaly")
		}
	}
}

func TestGenerateReport(t *testing.T) {
	mgr := NewManager()
	mgr.AddEntry(AuditEntry{Level: LevelInfo, Source: SourceAuth, Action: "login", Success: true})
	mgr.AddEntry(AuditEntry{Level: LevelError, Source: SourceSystem, Action: "error", Success: false})
	report := mgr.GenerateReport("last_7_days")
	if report == nil {
		t.Fatal("report should not be nil")
	}
	if report.TotalEvents != 2 {
		t.Errorf("expected 2 events, got %d", report.TotalEvents)
	}
	if report.Score < 0 || report.Score > 100 {
		t.Errorf("score out of range: %d", report.Score)
	}
}

func TestGetStats(t *testing.T) {
	mgr := NewManager()
	mgr.AddEntry(AuditEntry{Level: LevelInfo, Source: SourceAuth, Action: "login", Success: true})
	stats := mgr.GetStats()
	if stats == nil {
		t.Fatal("stats should not be nil")
	}
	if stats["total_entries"] != 1 {
		t.Errorf("expected 1 total entry, got %v", stats["total_entries"])
	}
}

func TestAPILogs(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/audit/logs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIStats(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/audit/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIAnomalies(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/audit/anomalies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIGenerateReport(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("POST", "/api/v1/audit/reports", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}
