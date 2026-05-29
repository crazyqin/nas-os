package complianceengine

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestHandler(t *testing.T) (*Handler, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	config := EngineConfig{
		Enabled:       true,
		AutoScan:      true,
		MaxConcurrent: 5,
	}

	mgr := NewManager(config)
	handler := NewHandler(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, mgr, router
}

func TestCreateRule(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"id": "test-rule-001",
		"standard": "cis",
		"category": "access_control",
		"severity": "high",
		"title": "测试规则",
		"description": "测试规则描述",
		"requirement": "必须启用访问控制",
		"remediation": "启用访问控制功能",
		"enabled": true
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/complianceengine/rules", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    ComplianceRule `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Data.ID != "test-rule-001" {
		t.Errorf("expected ID test-rule-001, got %s", resp.Data.ID)
	}
	if resp.Data.Standard != StandardCIS {
		t.Errorf("expected standard cis, got %s", resp.Data.Standard)
	}
}

func TestListRules(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// 创建测试规则
	mgr.CreateRule(ComplianceRule{
		ID:       "rule-1",
		Standard: StandardCIS,
		Category: CategoryAccessControl,
		Title:    "规则1",
		Enabled:  true,
	})
	mgr.CreateRule(ComplianceRule{
		ID:       "rule-2",
		Standard: StandardGDPR,
		Category: CategoryDataProtection,
		Title:    "规则2",
		Enabled:  true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/complianceengine/rules", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Data    []*ComplianceRule `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 rules, got %d", len(resp.Data))
	}
}

func TestStartScan(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// 创建测试规则
	mgr.CreateRule(ComplianceRule{
		ID:       "scan-rule-1",
		Standard: StandardCIS,
		Category: CategoryAccessControl,
		Title:    "扫描规则1",
		Enabled:  true,
	})

	body := `{"standard": "cis"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/complianceengine/scan", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    ComplianceScan `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	// 扫描可能已完成或运行中，取决于执行速度
	if resp.Data.Status != StatusRunning && resp.Data.Status != StatusCompleted {
		t.Errorf("expected status running or completed, got %s", resp.Data.Status)
	}
}

func TestGenerateReport(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// 创建规则和扫描
	mgr.CreateRule(ComplianceRule{
		ID:       "report-rule-1",
		Standard: StandardCIS,
		Category: CategoryAccessControl,
		Title:    "报告规则1",
		Enabled:  true,
	})

	scan, _ := mgr.StartScan([]ComplianceStandard{StandardCIS})

	body := `{"scanId": "` + scan.ID + `", "format": "json"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/complianceengine/reports", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Data    ComplianceReport  `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Data.ScanID != scan.ID {
		t.Errorf("expected scanId %s, got %s", scan.ID, resp.Data.ScanID)
	}
}

func TestGetTrends(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/complianceengine/trends", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int              `json:"code"`
		Message string           `json:"message"`
		Data    ComplianceStats  `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
}
