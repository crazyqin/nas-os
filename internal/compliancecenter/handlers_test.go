package compliancecenter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Handler) {
	gin.SetMode(gin.TestMode)
	cc := NewComplianceCenter()
	handler := NewHandler(cc)
	router := gin.New()
	rg := router.Group("/api")
	handler.RegisterRoutes(rg)
	return router, handler
}

func TestListStandards(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/compliance-center/standards", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp["success"].(bool) {
		t.Error("expected success to be true")
	}
	data := resp["data"].([]interface{})
	if len(data) != 7 {
		t.Errorf("expected 7 standards, got %d", len(data))
	}
}

func TestListChecks(t *testing.T) {
	router, handler := setupTestRouter()

	// 添加测试检查项
	handler.cc.AddCheck(ComplianceCheck{
		ID:       "check-1",
		Standard: StandardGDPR,
		Name:     "数据加密检查",
		Score:    80,
		MaxScore: 100,
	})
	handler.cc.AddCheck(ComplianceCheck{
		ID:       "check-2",
		Standard: StandardHIPAA,
		Name:     "访问控制检查",
		Score:    90,
		MaxScore: 100,
	})

	// 无过滤
	req := httptest.NewRequest(http.MethodGet, "/api/compliance-center/checks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 2 {
		t.Errorf("expected 2 checks, got %v", resp["total"])
	}

	// 按标准过滤
	req = httptest.NewRequest(http.MethodGet, "/api/compliance-center/checks?standard=GDPR", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 1 {
		t.Errorf("expected 1 check for GDPR, got %v", resp["total"])
	}
}

func TestRunChecks(t *testing.T) {
	router, handler := setupTestRouter()

	handler.cc.AddCheck(ComplianceCheck{
		ID:       "check-1",
		Standard: StandardGDPR,
		Name:     "数据加密检查",
		Score:    80,
		MaxScore: 100,
	})

	body, _ := json.Marshal(RunChecksRequest{
		Standard: StandardGDPR,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/compliance-center/checks/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if int(data["checked"].(float64)) != 1 {
		t.Errorf("expected 1 checked, got %v", data["checked"])
	}
}

func TestListReports(t *testing.T) {
	router, handler := setupTestRouter()

	// 先生成报告
	handler.cc.AddCheck(ComplianceCheck{
		ID:       "check-1",
		Standard: StandardGDPR,
		Name:     "test",
		Score:    90,
		MaxScore: 100,
	})
	handler.cc.GenerateReport(StandardGDPR)

	req := httptest.NewRequest(http.MethodGet, "/api/compliance-center/reports", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 1 {
		t.Errorf("expected 1 report, got %v", resp["total"])
	}
}

func TestGenerateReport(t *testing.T) {
	router, handler := setupTestRouter()

	handler.cc.AddCheck(ComplianceCheck{
		ID:       "check-1",
		Standard: StandardGDPR,
		Name:     "test",
		Score:    90,
		MaxScore: 100,
	})

	body, _ := json.Marshal(GenerateReportRequest{
		Standard: StandardGDPR,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/compliance-center/reports/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp["success"].(bool) {
		t.Error("expected success to be true")
	}
}

func TestListFindings(t *testing.T) {
	router, handler := setupTestRouter()

	// 添加检查项并生成报告
	handler.cc.AddCheck(ComplianceCheck{
		ID:       "check-1",
		Standard: StandardGDPR,
		Name:     "test",
		Score:    50,
		MaxScore: 100,
	})
	handler.cc.GenerateReport(StandardGDPR)

	req := httptest.NewRequest(http.MethodGet, "/api/compliance-center/findings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	// check-1 分数 50 < 80 (100*0.8), 应该是 finding
	if int(resp["total"].(float64)) != 1 {
		t.Errorf("expected 1 finding, got %v", resp["total"])
	}
}

func TestGetStats(t *testing.T) {
	router, handler := setupTestRouter()

	handler.cc.AddCheck(ComplianceCheck{
		ID:       "check-1",
		Standard: StandardGDPR,
		Name:     "test-pass",
		Score:    90,
		MaxScore: 100,
	})
	handler.cc.AddCheck(ComplianceCheck{
		ID:       "check-2",
		Standard: StandardGDPR,
		Name:     "test-fail",
		Score:    50,
		MaxScore: 100,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/compliance-center/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if int(data["totalChecks"].(float64)) != 2 {
		t.Errorf("expected 2 total checks, got %v", data["totalChecks"])
	}
	if int(data["passedChecks"].(float64)) != 1 {
		t.Errorf("expected 1 passed check, got %v", data["passedChecks"])
	}
	if int(data["failedChecks"].(float64)) != 1 {
		t.Errorf("expected 1 failed check, got %v", data["failedChecks"])
	}
}
