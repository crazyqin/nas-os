package sysdashboard

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

func TestGetDashboard(t *testing.T) {
	mgr := NewManager()
	data := mgr.GetDashboard()
	if data == nil {
		t.Fatal("GetDashboard returned nil")
	}
	if data.System.Hostname == "" {
		t.Error("hostname should not be empty")
	}
	if len(data.Services) == 0 {
		t.Error("services should not be empty")
	}
	if data.Storage.TotalSpace == 0 {
		t.Error("total space should not be 0")
	}
}

func TestAddActivity(t *testing.T) {
	mgr := NewManager()
	mgr.AddActivity(RecentActivity{
		Type:    "backup",
		Message: "备份完成",
		Level:   "info",
	})
	activities := mgr.GetActivities(10)
	if len(activities) != 1 {
		t.Errorf("expected 1 activity, got %d", len(activities))
	}
}

func TestAlerts(t *testing.T) {
	mgr := NewManager()
	mgr.AddAlert(AlertItem{
		Level:   "warning",
		Message: "磁盘空间不足",
		Source:  "storage",
	})
	alerts := mgr.GetAlerts()
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}

	resolved := mgr.ResolveAlert(alerts[0].ID)
	if !resolved {
		t.Error("expected alert to be resolved")
	}
	if len(mgr.GetAlerts()) != 0 {
		t.Error("expected 0 alerts after resolve")
	}
}

func TestAPIDashboard(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIActivities(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/dashboard/activities?limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIAlerts(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/dashboard/alerts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
