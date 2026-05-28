package datalifecycle

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

	mgr := NewManager()
	handler := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, mgr, router
}

func TestCreatePolicy(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"id": "policy-001",
		"name": "测试策略",
		"description": "测试策略描述",
		"enabled": true,
		"priority": 1,
		"type": "retention",
		"classifications": ["internal", "confidential"],
		"retention": {
			"type": "time",
			"duration": 86400000000000,
			"autoDelete": false
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lifecycle/policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var policy LifecyclePolicy
	if err := json.Unmarshal(w.Body.Bytes(), &policy); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if policy.ID != "policy-001" {
		t.Errorf("expected ID policy-001, got %s", policy.ID)
	}
	if policy.Name != "测试策略" {
		t.Errorf("expected name 测试策略, got %s", policy.Name)
	}
}

func TestCreateRecord(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"id": "record-001",
		"path": "/data/test/file.txt",
		"name": "file.txt",
		"size": 1024,
		"classification": "internal",
		"tags": ["test"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lifecycle/records", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var record DataRecord
	if err := json.Unmarshal(w.Body.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if record.ID != "record-001" {
		t.Errorf("expected ID record-001, got %s", record.ID)
	}
	if record.CurrentPhase != PhaseActive {
		t.Errorf("expected phase active, got %s", record.CurrentPhase)
	}
}

func TestTransitionPhase(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// 创建记录
	mgr.CreateRecord(DataRecord{
		ID:           "transition-record-1",
		Path:         "/data/transition",
		CurrentPhase: PhaseActive,
		CurrentTier:  TierHot,
	})

	body := `{"phase": "archive", "reason": "访问频率低"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lifecycle/records/transition-record-1/transition", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 验证阶段已更新
	record, _ := mgr.GetRecord("transition-record-1")
	if record.CurrentPhase != PhaseArchive {
		t.Errorf("expected phase archive, got %s", record.CurrentPhase)
	}
	if record.CurrentTier != TierCold {
		t.Errorf("expected tier cold, got %s", record.CurrentTier)
	}
}

func TestCreateHold(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"id": "hold-001",
		"type": "legal",
		"name": "法律保留",
		"description": "测试法律保留",
		"filePaths": ["/data/legal/file1.txt"],
		"caseNumber": "CASE-2024-001",
		"issuedBy": "法务部",
		"regulation": "GDPR"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lifecycle/holds", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var hold ComplianceHold
	if err := json.Unmarshal(w.Body.Bytes(), &hold); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if hold.ID != "hold-001" {
		t.Errorf("expected ID hold-001, got %s", hold.ID)
	}
	if !hold.Active {
		t.Errorf("expected hold to be active")
	}
}

func TestCreateMigration(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"id": "migration-001",
		"sourceTier": "hot",
		"targetTier": "cold",
		"sourcePath": "/data/hot",
		"targetPath": "/data/cold",
		"files": [
			{"sourcePath": "/data/hot/file1.txt", "targetPath": "/data/cold/file1.txt", "size": 1024}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lifecycle/migrations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var migration DataMigration
	if err := json.Unmarshal(w.Body.Bytes(), &migration); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if migration.ID != "migration-001" {
		t.Errorf("expected ID migration-001, got %s", migration.ID)
	}
	if migration.Status != MigrationPending {
		t.Errorf("expected status pending, got %s", migration.Status)
	}
}

func TestGetStatus(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/lifecycle/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status LifecycleStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !status.Enabled {
		t.Errorf("expected enabled to be true")
	}
}
