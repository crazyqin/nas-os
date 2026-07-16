package smartfan

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
	fans := mgr.GetFans()
	if len(fans) == 0 {
		t.Error("expected fans")
	}
	sensors := mgr.GetSensors()
	if len(sensors) == 0 {
		t.Error("expected sensors")
	}
}

func TestGetProfiles(t *testing.T) {
	mgr := NewManager()
	profiles := mgr.GetProfiles()
	if len(profiles) < 3 {
		t.Errorf("expected at least 3 profiles, got %d", len(profiles))
	}
}

func TestSetProfile(t *testing.T) {
	mgr := NewManager()
	err := mgr.SetProfile("silent")
	if err != nil {
		t.Fatalf("SetProfile failed: %v", err)
	}
	err = mgr.SetProfile("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestSetFanMode(t *testing.T) {
	mgr := NewManager()
	err := mgr.SetFanMode("fan0", FanModeManual, 80)
	if err != nil {
		t.Fatalf("SetFanMode failed: %v", err)
	}
	fans := mgr.GetFans()
	for _, f := range fans {
		if f.ID == "fan0" && f.Mode != FanModeManual {
			t.Error("expected fan0 to be in manual mode")
		}
	}
}

func TestGetAlertsBasic(t *testing.T) {
	mgr := NewManager()
	alerts := mgr.GetAlerts(false)
	_ = alerts // 不报错即可
}

func TestAPIFans(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/fan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPISensors(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/fan/sensors", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIProfiles(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/fan/profiles", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIApplyProfile(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("POST", "/api/v1/fan/profiles/silent/apply", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
