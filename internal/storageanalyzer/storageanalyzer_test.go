package storageanalyzer

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

func TestStartScan(t *testing.T) {
	mgr := NewManager()
	result, err := mgr.StartScan("/data")
	if err != nil {
		t.Fatalf("StartScan failed: %v", err)
	}
	if result.ID == "" {
		t.Error("scan ID should not be empty")
	}
	if result.ScanPath != "/data" {
		t.Errorf("expected path /data, got %s", result.ScanPath)
	}
}

func TestGetScanResult(t *testing.T) {
	mgr := NewManager()
	result, _ := mgr.StartScan("/data")

	// 等待扫描完成（模拟）
	fetched, err := mgr.GetScanResult(result.ID)
	if err != nil {
		t.Fatalf("GetScanResult failed: %v", err)
	}
	if fetched.ID != result.ID {
		t.Errorf("expected ID %s, got %s", result.ID, fetched.ID)
	}

	_, err = mgr.GetScanResult("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent scan")
	}
}

func TestListScans(t *testing.T) {
	mgr := NewManager()
	mgr.StartScan("/a")
	mgr.StartScan("/b")
	scans := mgr.ListScans()
	if len(scans) != 2 {
		t.Errorf("expected 2 scans, got %d", len(scans))
	}
}

func TestGetDuplicates(t *testing.T) {
	mgr := NewManager()
	result, _ := mgr.StartScan("/data")

	dupes, err := mgr.GetDuplicates(result.ID, 0)
	if err != nil {
		t.Fatalf("GetDuplicates failed: %v", err)
	}
	if len(dupes) == 0 {
		t.Error("expected duplicates")
	}
}

func TestGetTrend(t *testing.T) {
	mgr := NewManager()
	trend := mgr.GetTrend(30)
	_ = trend // 可能为空，不报错即可
}

func TestAPIScan(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("POST", "/api/v1/storage-analyzer/scan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}
}

func TestAPIListScans(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/storage-analyzer/scans", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIGetTrend(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/storage-analyzer/trend?days=30", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
