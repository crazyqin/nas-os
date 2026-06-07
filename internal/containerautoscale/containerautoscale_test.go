package containerautoscale

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

func TestNewManager(t *testing.T) {
	m := NewManager(nil, nil)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	cfg := m.GetConfig()
	if !cfg.Enabled {
		t.Error("expected config enabled by default")
	}
}

func TestRegisterContainer(t *testing.T) {
	m := setupTestManager(t)

	c := &Container{
		Name:        "web-app",
		ServiceName: "web-app",
		Image:       "nginx:latest",
		Replicas:    2,
		MinReplicas: 1,
		MaxReplicas: 10,
	}
	m.RegisterContainer(c)

	containers := m.ListContainers()
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].ServiceName != "web-app" {
		t.Errorf("expected service name web-app, got %s", containers[0].ServiceName)
	}
}

func TestUnregisterContainer(t *testing.T) {
	m := setupTestManager(t)

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 2})
	m.UnregisterContainer("web-app")

	containers := m.ListContainers()
	if len(containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(containers))
	}
}

func TestManualScale(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 2, MinReplicas: 1, MaxReplicas: 10})

	event, err := m.ManualScale(ctx, &ScaleRequest{
		ServiceName: "web-app",
		Replicas:    5,
		Reason:      "traffic spike",
	})
	if err != nil {
		t.Fatalf("ManualScale failed: %v", err)
	}
	if !event.Success {
		t.Error("expected successful scale event")
	}
	if event.Direction != ScaleUp {
		t.Errorf("expected scale up, got %s", event.Direction)
	}
	if event.ToReplicas != 5 {
		t.Errorf("expected 5 replicas, got %d", event.ToReplicas)
	}
}

func TestManualScaleDown(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 5, MinReplicas: 1, MaxReplicas: 10})

	event, err := m.ManualScale(ctx, &ScaleRequest{
		ServiceName: "web-app",
		Replicas:    2,
	})
	if err != nil {
		t.Fatalf("ManualScale failed: %v", err)
	}
	if event.Direction != ScaleDown {
		t.Errorf("expected scale down, got %s", event.Direction)
	}
}

func TestManualScaleSameReplicas(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 3})

	_, err := m.ManualScale(ctx, &ScaleRequest{
		ServiceName: "web-app",
		Replicas:    3,
	})
	if err == nil {
		t.Error("expected error for same replicas")
	}
}

func TestManualScaleNotFound(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	_, err := m.ManualScale(ctx, &ScaleRequest{
		ServiceName: "nonexistent",
		Replicas:    3,
	})
	if err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestQuotaEnforcement(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 2, MinReplicas: 1, MaxReplicas: 10})
	m.SetQuota(&ResourceQuota{
		ServiceName: "web-app",
		MaxReplicas: 5,
		MinReplicas: 1,
	})

	// 在配额内
	_, err := m.ManualScale(ctx, &ScaleRequest{ServiceName: "web-app", Replicas: 4})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 超出配额
	_, err = m.ManualScale(ctx, &ScaleRequest{ServiceName: "web-app", Replicas: 10})
	if err == nil {
		t.Error("expected error for exceeding quota")
	}
}

func TestPolicyManagement(t *testing.T) {
	m := setupTestManager(t)

	policy := &ScalePolicy{
		ServiceName: "web-app",
		Strategy:    StrategyThreshold,
		Enabled:     true,
		MetricType:  MetricCPU,
		Threshold: &ThresholdConfig{
			ScaleUpThreshold:   80,
			ScaleDownThreshold: 20,
			EvaluationPeriods:  3,
			ScaleUpStep:        2,
			ScaleDownStep:      1,
		},
		CooldownSec: 300,
	}
	m.SetPolicy(policy)

	got, exists := m.GetPolicy("web-app")
	if !exists {
		t.Fatal("expected policy to exist")
	}
	if got.Strategy != StrategyThreshold {
		t.Errorf("expected strategy threshold, got %s", got.Strategy)
	}

	policies := m.ListPolicies()
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}

	m.DeletePolicy("web-app")
	_, exists = m.GetPolicy("web-app")
	if exists {
		t.Error("expected policy to be deleted")
	}
}

func TestQuotaManagement(t *testing.T) {
	m := setupTestManager(t)

	quota := &ResourceQuota{
		ServiceName:   "web-app",
		MaxCPU:        4.0,
		MaxMemoryMB:   8192,
		MaxReplicas:   10,
		MinReplicas:   1,
		MaxCostPerDay: 100.0,
	}
	m.SetQuota(quota)

	got, exists := m.GetQuota("web-app")
	if !exists {
		t.Fatal("expected quota to exist")
	}
	if got.MaxCPU != 4.0 {
		t.Errorf("expected max CPU 4.0, got %f", got.MaxCPU)
	}

	quotas := m.ListQuotas()
	if len(quotas) != 1 {
		t.Errorf("expected 1 quota, got %d", len(quotas))
	}

	m.DeleteQuota("web-app")
	_, exists = m.GetQuota("web-app")
	if exists {
		t.Error("expected quota to be deleted")
	}
}

func TestRecordAndGetMetrics(t *testing.T) {
	m := setupTestManager(t)
	now := time.Now()

	m.RecordMetric(MetricPoint{Timestamp: now, Type: MetricCPU, Value: 75.0, ServiceName: "web-app"})
	m.RecordMetric(MetricPoint{Timestamp: now, Type: MetricMemory, Value: 60.0, ServiceName: "web-app"})

	query := &MetricsQuery{
		ServiceName: "web-app",
		MetricType:  MetricCPU,
	}
	metrics := m.GetMetrics(query)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Value != 75.0 {
		t.Errorf("expected value 75.0, got %f", metrics[0].Value)
	}
}

func TestScaleEvents(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 2})
	m.ManualScale(ctx, &ScaleRequest{ServiceName: "web-app", Replicas: 5})
	m.ManualScale(ctx, &ScaleRequest{ServiceName: "web-app", Replicas: 3})

	events := m.GetScaleEvents("web-app", 10)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestAlerts(t *testing.T) {
	m := setupTestManager(t)

	// 触发告警（通过超出配额）
	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 2})
	m.SetQuota(&ResourceQuota{ServiceName: "web-app", MaxReplicas: 5, MinReplicas: 1})

	// 创建一个策略来触发告警
	policy := &ScalePolicy{
		ServiceName: "web-app",
		Strategy:    StrategyThreshold,
		Enabled:     true,
		MetricType:  MetricCPU,
		Threshold: &ThresholdConfig{
			ScaleUpThreshold:   80,
			ScaleDownThreshold: 20,
			EvaluationPeriods:  1,
			ScaleUpStep:        10, // 会超出配额
		},
	}
	m.SetPolicy(policy)

	// 手动创建告警测试
	m.createAlert("web-app", AlertWarning, "test_alert", "test message")

	alerts := m.GetAlerts(false, 10)
	if len(alerts) == 0 {
		t.Error("expected at least 1 alert")
	}

	if len(alerts) > 0 {
		err := m.ResolveAlert(alerts[0].ID)
		if err != nil {
			t.Fatalf("failed to resolve alert: %v", err)
		}
	}

	resolvedAlerts := m.GetAlerts(true, 10)
	if len(resolvedAlerts) != 1 {
		t.Errorf("expected 1 resolved alert, got %d", len(resolvedAlerts))
	}
}

func TestPredictInsufficientData(t *testing.T) {
	m := setupTestManager(t)

	result := m.Predict(context.Background(), "web-app", MetricCPU, "15m")
	if result != nil {
		t.Error("expected nil prediction with insufficient data")
	}
}

func TestPredictWithData(t *testing.T) {
	m := setupTestManager(t)

	// 添加足够的数据点
	for i := 0; i < 20; i++ {
		m.RecordMetric(MetricPoint{
			Timestamp:   time.Now().Add(-time.Duration(20-i) * time.Minute),
			Type:        MetricCPU,
			Value:       50.0 + float64(i)*1.5,
			ServiceName: "web-app",
		})
	}

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 2})

	result := m.Predict(context.Background(), "web-app", MetricCPU, "15m")
	if result == nil {
		t.Fatal("expected prediction result")
	}
	if result.ServiceName != "web-app" {
		t.Errorf("expected service web-app, got %s", result.ServiceName)
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("confidence out of range: %f", result.Confidence)
	}
}

func TestCostSuggestions(t *testing.T) {
	m := setupTestManager(t)

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 5, MinReplicas: 1})

	// 模拟低 CPU 使用率
	for i := 0; i < 15; i++ {
		m.RecordMetric(MetricPoint{
			Timestamp:   time.Now(),
			Type:        MetricCPU,
			Value:       10.0,
			ServiceName: "web-app",
		})
	}

	suggestions := m.GenerateCostSuggestions()
	if len(suggestions) == 0 {
		t.Error("expected cost suggestions")
	}
}

func TestGetConfig(t *testing.T) {
	m := setupTestManager(t)

	cfg := m.GetConfig()
	if !cfg.Enabled {
		t.Error("expected enabled")
	}

	cfg.Enabled = false
	m.UpdateConfig(cfg)

	cfg2 := m.GetConfig()
	if cfg2.Enabled {
		t.Error("expected disabled after update")
	}
}

func TestHandler_ListContainers(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 2})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-autoscale/containers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_RegisterContainer(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"name":"web-app","service_name":"web-app","image":"nginx:latest","replicas":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-autoscale/containers", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ManualScale(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 2, MinReplicas: 1, MaxReplicas: 10})

	body := `{"service_name":"web-app","replicas":5,"reason":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-autoscale/scale", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Policies(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 设置策略
	body := `{"service_name":"web-app","strategy":"threshold","metric_type":"cpu","threshold":{"scale_up_threshold":80,"scale_down_threshold":20}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-autoscale/policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// 列出策略
	req = httptest.NewRequest(http.MethodGet, "/api/v1/container-autoscale/policies", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 获取策略
	req = httptest.NewRequest(http.MethodGet, "/api/v1/container-autoscale/policies/web-app", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// 删除策略
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/container-autoscale/policies/web-app", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Quotas(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 设置配额
	body := `{"service_name":"web-app","max_cpu":4,"max_memory_mb":8192,"max_replicas":10,"min_replicas":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-autoscale/quotas", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// 列出配额
	req = httptest.NewRequest(http.MethodGet, "/api/v1/container-autoscale/quotas", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Metrics(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 记录指标
	body := `{"type":"cpu","value":75.5,"service_name":"web-app"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-autoscale/metrics", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// 查询指标
	req = httptest.NewRequest(http.MethodGet, "/api/v1/container-autoscale/metrics?service=web-app&type=cpu", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Events(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	m.RegisterContainer(&Container{ServiceName: "web-app", Name: "web-app", Replicas: 2})
	m.ManualScale(context.Background(), &ScaleRequest{ServiceName: "web-app", Replicas: 5})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-autoscale/events?service=web-app&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Alerts(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	m.createAlert("web-app", AlertWarning, "test", "test message")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-autoscale/alerts?resolved=false", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Config(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 获取配置
	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-autoscale/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 更新配置
	body := `{"enabled":true,"metrics_interval_sec":60}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/container-autoscale/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_CostSuggestions(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-autoscale/cost/suggestions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Predict(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 数据不足时应返回错误
	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-autoscale/predict/web-app?type=cpu", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		// 预期数据不足时返回错误
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 200 or 400, got %d", w.Code)
		}
	}
}
