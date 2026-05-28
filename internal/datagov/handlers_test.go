package datagov

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestHandler(t *testing.T) (*Handlers, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	config := DefaultConfig()
	mgr := NewManager(config)
	handler := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, mgr, router
}

func TestCreateAsset(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"id": "test-asset-001",
		"name": "测试数据集",
		"path": "/data/test",
		"type": "file",
		"owner": "admin",
		"classification": "internal",
		"sensitive_types": ["pii"],
		"tags": ["test"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/datagov/assets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var asset DataAsset
	if err := json.Unmarshal(w.Body.Bytes(), &asset); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if asset.ID != "test-asset-001" {
		t.Errorf("expected ID test-asset-001, got %s", asset.ID)
	}
	if asset.Name != "测试数据集" {
		t.Errorf("expected name 测试数据集, got %s", asset.Name)
	}
}

func TestListAssets(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// 创建测试资产
	mgr.CreateAsset(DataAsset{
		ID:             "asset-1",
		Name:           "资产1",
		Classification: ClassificationPublic,
		Owner:          "user1",
	})
	mgr.CreateAsset(DataAsset{
		ID:             "asset-2",
		Name:           "资产2",
		Classification: ClassificationConfidential,
		Owner:          "user2",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/datagov/assets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Assets []*DataAsset `json:"assets"`
		Total  int          `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("expected 2 assets, got %d", resp.Total)
	}
}

func TestScanAsset(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// 创建资产和扫描规则
	mgr.CreateAsset(DataAsset{
		ID:   "scan-asset-1",
		Name: "扫描资产",
		Path: "/data/scan",
	})
	mgr.CreateScanRule(ScanRule{
		ID:       "rule-1",
		Name:     "PII检测",
		Enabled:  true,
		DataType: SensitivePII,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/datagov/assets/scan-asset-1/scan", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Results []*ScanResult `json:"results"`
		Total   int           `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Total == 0 {
		t.Errorf("expected scan results, got 0")
	}
}

func TestCreateRetentionPolicy(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"id": "policy-001",
		"name": "测试保留策略",
		"description": "测试策略描述",
		"retention_days": 365,
		"auto_delete": true,
		"enabled": true
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/datagov/retention-policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var policy RetentionPolicy
	if err := json.Unmarshal(w.Body.Bytes(), &policy); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if policy.ID != "policy-001" {
		t.Errorf("expected ID policy-001, got %s", policy.ID)
	}
}

func TestGenerateComplianceReport(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{"framework": "gdpr"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/datagov/compliance-reports", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var report ComplianceReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if report.Framework != FrameworkGDPR {
		t.Errorf("expected framework gdpr, got %s", report.Framework)
	}
}
