package storageqos

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestHandler(t *testing.T) (*Handler, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mgr := NewManager(zap.NewNop())
	handler := NewHandler(mgr, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, mgr, router
}

func TestCreatePolicy(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"name": "数据库QoS",
		"description": "数据库高优先级策略",
		"target": "/data/mysql",
		"target_type": "dataset",
		"priority": "high",
		"bandwidth": {
			"read_bps_limit": 100,
			"write_bps_limit": 80,
			"read_iops_limit": 10000,
			"write_iops_limit": 8000
		},
		"burst": {
			"burst_enabled": true,
			"burst_size_mb": 50,
			"burst_duration_sec": 10,
			"burst_replenish_rate_mb": 5
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storageqos/policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var policy QoSPolicy
	if err := json.Unmarshal(w.Body.Bytes(), &policy); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if policy.Name != "数据库QoS" {
		t.Errorf("expected name 数据库QoS, got %s", policy.Name)
	}
	if policy.Priority != IOPriorityHigh {
		t.Errorf("expected priority high, got %s", policy.Priority)
	}
	if policy.Bandwidth.ReadBPSLimit != 100 {
		t.Errorf("expected read_bps_limit 100, got %f", policy.Bandwidth.ReadBPSLimit)
	}
	if !policy.Burst.BurstEnabled {
		t.Error("expected burst enabled")
	}
	if policy.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreatePolicyDuplicateTarget(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"name": "策略1",
		"target": "/data/app",
		"target_type": "dataset",
		"priority": "normal"
	}`

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/storageqos/policies", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d", w1.Code)
	}

	body2 := `{
		"name": "策略2",
		"target": "/data/app",
		"target_type": "dataset",
		"priority": "low"
	}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/storageqos/policies", bytes.NewBufferString(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("duplicate: expected 409, got %d", w2.Code)
	}
}

func TestCreatePolicyMissingRequired(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{"name": "缺字段"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storageqos/policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreatePolicyInvalidPriority(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"name": "无效优先级",
		"target": "/data/test",
		"target_type": "dataset",
		"priority": "ultra"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storageqos/policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListPolicies(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	enabled := true
	_, _ = mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "策略A",
		Target:     "/data/a",
		TargetType: "dataset",
		Priority:   IOPriorityHigh,
		Enabled:    &enabled,
	})
	_, _ = mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "策略B",
		Target:     "/data/b",
		TargetType: "share",
		Priority:   IOPriorityLow,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageqos/policies", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp PolicyListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("expected 2 policies, got %d", resp.Total)
	}
}

func TestGetPolicy(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	policy, _ := mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "查询测试",
		Target:     "/data/query",
		TargetType: "dataset",
		Priority:   IOPriorityNormal,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageqos/policies/"+policy.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got QoSPolicy
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.ID != policy.ID {
		t.Errorf("expected ID %s, got %s", policy.ID, got.ID)
	}
}

func TestGetPolicyNotFound(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageqos/policies/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdatePolicy(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	policy, _ := mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "原始名称",
		Target:     "/data/update",
		TargetType: "dataset",
		Priority:   IOPriorityNormal,
	})

	body := `{
		"name": "更新后名称",
		"priority": "high",
		"bandwidth": {
			"read_bps_limit": 200
		}
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/storageqos/policies/"+policy.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated QoSPolicy
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Name != "更新后名称" {
		t.Errorf("expected name 更新后名称, got %s", updated.Name)
	}
	if updated.Priority != IOPriorityHigh {
		t.Errorf("expected priority high, got %s", updated.Priority)
	}
	if updated.Bandwidth.ReadBPSLimit != 200 {
		t.Errorf("expected read_bps_limit 200, got %f", updated.Bandwidth.ReadBPSLimit)
	}
}

func TestDeletePolicy(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	policy, _ := mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "待删除",
		Target:     "/data/delete",
		TargetType: "dataset",
		Priority:   IOPriorityLow,
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/storageqos/policies/"+policy.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// 验证已删除
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/storageqos/policies/"+policy.ID, nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w2.Code)
	}
}

func TestDeletePolicyNotFound(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/storageqos/policies/ghost", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetMetrics(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	policy, _ := mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "指标测试",
		Target:     "/data/metrics",
		TargetType: "dataset",
		Priority:   IOPriorityNormal,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageqos/policies/"+policy.ID+"/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp MetricsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.PolicyID != policy.ID {
		t.Errorf("expected policy_id %s, got %s", policy.ID, resp.PolicyID)
	}
}

func TestGetStats(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	enabled := true
	_, _ = mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "S1",
		Target:     "/data/s1",
		TargetType: "dataset",
		Priority:   IOPriorityHigh,
		Enabled:    &enabled,
	})
	_, _ = mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "S2",
		Target:     "/data/s2",
		TargetType: "dataset",
		Priority:   IOPriorityLow,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageqos/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &stats)

	if stats["total_policies"] != float64(2) {
		t.Errorf("expected 2 policies, got %v", stats["total_policies"])
	}
}

func TestRecordIO(t *testing.T) {
	mgr := NewManager(zap.NewNop())

	policy, err := mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "I/O测试",
		Target:     "/data/io",
		TargetType: "dataset",
		Priority:   IOPriorityHigh,
		Bandwidth: BandwidthLimit{
			ReadBPSLimit:  50,
			WriteBPSLimit: 50,
		},
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// 正常I/O
	allowed, throttled, err := mgr.RecordIO(policy.ID, 1024*1024, 512*1024) // 1MB读 + 0.5MB写
	if err != nil {
		t.Fatalf("RecordIO failed: %v", err)
	}
	if allowed <= 0 {
		t.Errorf("expected allowed > 0, got %f", allowed)
	}
	t.Logf("正常I/O: allowed=%f MB, throttled=%f MB", allowed, throttled)
}

func TestRecordIOWithBurst(t *testing.T) {
	mgr := NewManager(zap.NewNop())

	policy, err := mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "突发测试",
		Target:     "/data/burst",
		TargetType: "dataset",
		Priority:   IOPriorityHigh,
		Bandwidth: BandwidthLimit{
			ReadBPSLimit:  10,
			WriteBPSLimit: 10,
		},
		Burst: BurstConfig{
			BurstEnabled:         true,
			BurstSizeMB:          50,
			BurstDurationSec:     10,
			BurstReplenishRateMB: 5,
		},
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// 突发I/O
	allowed, _, err := mgr.RecordIO(policy.ID, 10*1024*1024, 10*1024*1024) // 10MB读 + 10MB写
	if err != nil {
		t.Fatalf("RecordIO failed: %v", err)
	}
	if allowed <= 0 {
		t.Errorf("expected allowed > 0 with burst, got %f", allowed)
	}
	t.Logf("突发I/O: allowed=%f MB", allowed)
}

func TestRecordIODisabledPolicy(t *testing.T) {
	mgr := NewManager(zap.NewNop())

	disabled := false
	policy, err := mgr.CreatePolicy(nil, CreatePolicyRequest{
		Name:       "禁用策略",
		Target:     "/data/disabled",
		TargetType: "dataset",
		Priority:   IOPriorityNormal,
		Enabled:    &disabled,
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	allowed, throttled, err := mgr.RecordIO(policy.ID, 100*1024*1024, 100*1024*1024)
	if err != nil {
		t.Fatalf("RecordIO failed: %v", err)
	}
	if throttled != 0 {
		t.Errorf("disabled policy should not throttle, got throttled=%f", throttled)
	}
	if allowed <= 0 {
		t.Errorf("expected allowed > 0 for disabled policy, got %f", allowed)
	}
}
