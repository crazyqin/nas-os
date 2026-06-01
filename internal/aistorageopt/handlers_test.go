package aistorageopt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)
	return router
}

func TestNewHandler(t *testing.T) {
	opt := New(Config{Enabled: true})
	handler := NewHandler(opt)
	if handler == nil {
		t.Fatal("NewHandler 返回 nil")
	}
	if handler.manager != opt {
		t.Fatal("manager 指针不匹配")
	}
}

func TestRegisterRoutes(t *testing.T) {
	opt := New(Config{Enabled: true})
	handler := NewHandler(opt)
	router := setupTestRouter(handler)

	routes := router.Routes()
	expectedRoutes := []string{
		"/api/v1/ai-storage-opt/optimizations",
		"/api/v1/ai-storage-opt/optimizations/execute",
		"/api/v1/ai-storage-opt/tiers",
		"/api/v1/ai-storage-opt/tiers/migrate",
		"/api/v1/ai-storage-opt/stats",
	}

	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	for _, expected := range expectedRoutes {
		if !routeMap[expected] {
			t.Errorf("路由 %s 未注册", expected)
		}
	}
}

func TestGetOptimizations(t *testing.T) {
	opt := New(Config{Enabled: true})
	handler := NewHandler(opt)
	router := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/ai-storage-opt/optimizations", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if _, ok := resp["prediction"]; !ok {
		t.Fatal("响应缺少 prediction 字段")
	}
	if _, ok := resp["report"]; !ok {
		t.Fatal("响应缺少 report 字段")
	}
}

func TestExecuteOptimization(t *testing.T) {
	opt := New(Config{Enabled: true})
	handler := NewHandler(opt)
	router := setupTestRouter(handler)

	body := ExecuteOptimizationRequest{Type: "tier_migration"}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/ai-storage-opt/optimizations/execute", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["message"] != "优化执行完成" {
		t.Fatalf("期望消息 '优化执行完成', got %v", resp["message"])
	}
}

func TestExecuteOptimizationInvalidRequest(t *testing.T) {
	opt := New(Config{Enabled: true})
	handler := NewHandler(opt)
	router := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/ai-storage-opt/optimizations/execute", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望状态码 400, got %d", w.Code)
	}
}

func TestGetTiers(t *testing.T) {
	opt := New(Config{Enabled: true})
	// 添加测试策略
	opt.AddTierPolicy(&TierPolicy{
		ID:      "policy1",
		Name:    "冷数据迁移",
		Enabled: true,
		Tier:    TierCold,
	})

	handler := NewHandler(opt)
	router := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/ai-storage-opt/tiers", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["total"].(float64) != 1 {
		t.Fatalf("期望 1 个策略, got %v", resp["total"])
	}
}

func TestMigrateTier(t *testing.T) {
	opt := New(Config{Enabled: true})
	handler := NewHandler(opt)
	router := setupTestRouter(handler)

	body := MigrateTierRequest{
		SourceTier: TierHot,
		TargetTier: TierCold,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/ai-storage-opt/tiers/migrate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["source_tier"] != string(TierHot) {
		t.Fatalf("source_tier 不匹配: %v", resp["source_tier"])
	}
	if resp["target_tier"] != string(TierCold) {
		t.Fatalf("target_tier 不匹配: %v", resp["target_tier"])
	}
}

func TestGetStats(t *testing.T) {
	opt := New(Config{Enabled: true})
	// 添加测试数据
	opt.RecordAccess("file1", 1024, 512)
	opt.RecordMetrics(&StorageMetrics{
		TotalSpace: 1024 * 1024 * 1024,
		UsedSpace:  512 * 1024 * 1024,
		FreeSpace:  512 * 1024 * 1024,
	})

	handler := NewHandler(opt)
	router := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/ai-storage-opt/stats", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["access_patterns"].(float64) != 1 {
		t.Fatalf("期望 1 个访问模式, got %v", resp["access_patterns"])
	}
	if resp["metrics_count"].(float64) != 1 {
		t.Fatalf("期望 1 条指标, got %v", resp["metrics_count"])
	}
}
