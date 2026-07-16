// Package microsvcmesh 测试文件
package microsvcmesh

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func setupTestEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine(zap.NewNop(), nil)
}

func setupTestService(name string) *Service {
	return &Service{
		Name:      name,
		Version:   "1.0.0",
		Namespace: "default",
		Endpoints: []*Endpoint{
			{ID: "ep-1", Host: "10.0.0.1", Port: 8080, Protocol: ProtocolHTTP, Weight: 1},
			{ID: "ep-2", Host: "10.0.0.2", Port: 8080, Protocol: ProtocolHTTP, Weight: 1},
		},
	}
}

// ==================== Engine Tests ====================

func TestNewEngine(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		e := setupTestEngine(t)
		if e == nil {
			t.Fatal("expected non-nil engine")
		}
		if e.IsRunning() {
			t.Error("engine should not be running initially")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ListenAddr = ":9090"
		cfg.TracingEnabled = false
		e := NewEngine(zap.NewNop(), cfg)
		if e.config.ListenAddr != ":9090" {
			t.Errorf("expected :9090, got %s", e.config.ListenAddr)
		}
	})
}

func TestEngineStartStop(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()

	if err := e.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if !e.IsRunning() {
		t.Error("engine should be running")
	}

	if err := e.Start(ctx); err == nil {
		t.Error("expected error on double start")
	}

	e.Stop()
	if e.IsRunning() {
		t.Error("engine should not be running after stop")
	}
}

func TestRegisterService(t *testing.T) {
	e := setupTestEngine(t)

	svc := setupTestService("user-service")
	if err := e.RegisterService(svc); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// 重复注册
	if err := e.RegisterService(svc); err == nil {
		t.Error("expected error on double register")
	}

	// 空名称
	if err := e.RegisterService(&Service{}); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestDeregisterService(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterService(setupTestService("test"))
	if err := e.DeregisterService("test", "default"); err != nil {
		t.Fatalf("deregister failed: %v", err)
	}

	if err := e.DeregisterService("nonexistent", "default"); err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestGetService(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterService(setupTestService("my-svc"))

	got, err := e.GetService("my-svc", "default")
	if err != nil {
		t.Fatalf("get service failed: %v", err)
	}
	if got.Name != "my-svc" {
		t.Errorf("expected my-svc, got %s", got.Name)
	}
	if len(got.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(got.Endpoints))
	}

	// 不存在的服务
	_, err = e.GetService("nonexistent", "default")
	if err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestListServices(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterService(setupTestService("svc-a"))
	e.RegisterService(&Service{Name: "svc-b", Namespace: "other"})

	all := e.ListServices("")
	if len(all) != 2 {
		t.Errorf("expected 2 services, got %d", len(all))
	}

	def := e.ListServices("default")
	if len(def) != 1 {
		t.Errorf("expected 1 default service, got %d", len(def))
	}
}

func TestAddEndpoint(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterService(setupTestService("test"))

	ep := &Endpoint{
		Host:     "10.0.0.3",
		Port:     8080,
		Protocol: ProtocolHTTP,
		Weight:   2,
	}

	if err := e.AddEndpoint("test", "default", ep); err != nil {
		t.Fatalf("add endpoint failed: %v", err)
	}
	if ep.ID == "" {
		t.Error("expected non-empty endpoint ID")
	}

	svc, _ := e.GetService("test", "default")
	if len(svc.Endpoints) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(svc.Endpoints))
	}

	// 不存在的服务
	if err := e.AddEndpoint("nonexistent", "default", ep); err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestRemoveEndpoint(t *testing.T) {
	e := setupTestEngine(t)

	svc := setupTestService("test")
	e.RegisterService(svc)

	if err := e.RemoveEndpoint("test", "default", "ep-1"); err != nil {
		t.Fatalf("remove endpoint failed: %v", err)
	}

	got, _ := e.GetService("test", "default")
	if len(got.Endpoints) != 1 {
		t.Errorf("expected 1 endpoint after removal, got %d", len(got.Endpoints))
	}

	// 不存在的端点
	if err := e.RemoveEndpoint("test", "default", "nonexistent"); err == nil {
		t.Error("expected error for nonexistent endpoint")
	}
}

func TestAddRoute(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterService(setupTestService("test"))

	route := &Route{
		Name:     "api-route",
		Prefix:   "/api/v1",
		Methods:  []string{"GET", "POST"},
		Strategy: RouteWeighted,
		Target:   "test",
	}

	if err := e.AddRoute("test", "default", route); err != nil {
		t.Fatalf("add route failed: %v", err)
	}
	if route.ID == "" {
		t.Error("expected non-empty route ID")
	}

	svc, _ := e.GetService("test", "default")
	if len(svc.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(svc.Routes))
	}
}

func TestProxyRequest(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()

	e.RegisterService(setupTestService("my-api"))
	e.Start(ctx)
	defer e.Stop()

	resp, err := e.ProxyRequest(ctx, &ProxyRequest{
		Method:  "GET",
		Path:    "/api/v1/users",
		Service: "my-api",
	})
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Upstream == "" {
		t.Error("expected non-empty upstream")
	}
	if resp.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestProxyRequestServiceNotFound(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()

	_, err := e.ProxyRequest(ctx, &ProxyRequest{
		Method:  "GET",
		Path:    "/",
		Service: "nonexistent",
	})
	if err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestCircuitBreakerState(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterService(setupTestService("test"))

	state, err := e.GetCircuitBreakerState("test", "default")
	if err != nil {
		t.Fatalf("get state failed: %v", err)
	}
	if state != CircuitClosed {
		t.Errorf("expected closed, got %s", state)
	}
}

func TestGetStats(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterService(setupTestService("test"))

	stats := e.GetStats()
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats["services"] != 1 {
		t.Errorf("expected 1 service, got %v", stats["services"])
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("expected enabled")
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("expected :8080, got %s", cfg.ListenAddr)
	}
	if cfg.RequestTimeout != 30 {
		t.Errorf("expected 30s timeout, got %d", cfg.RequestTimeout)
	}
}

// ==================== Proxy Tests ====================

func TestProxyRouteMatching(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()

	svc := setupTestService("api")
	svc.Routes = []*Route{
		{ID: "r1", Prefix: "/api", Strategy: RouteRoundRobin, Methods: []string{"GET"}},
		{ID: "r2", Prefix: "/webhook", Strategy: RouteWeighted, Methods: []string{"POST"}},
	}
	e.RegisterService(svc)
	e.Start(ctx)
	defer e.Stop()

	t.Run("match api route", func(t *testing.T) {
		resp, err := e.ProxyRequest(ctx, &ProxyRequest{
			Method:  "GET",
			Path:    "/api/users",
			Service: "api",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.Headers["X-Strategy"] != string(RouteRoundRobin) {
			t.Errorf("expected round_robin strategy, got %s", resp.Headers["X-Strategy"])
		}
	})

	t.Run("match webhook route", func(t *testing.T) {
		resp, err := e.ProxyRequest(ctx, &ProxyRequest{
			Method:  "POST",
			Path:    "/webhook/github",
			Service: "api",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.Headers["X-Strategy"] != string(RouteWeighted) {
			t.Errorf("expected weighted strategy, got %s", resp.Headers["X-Strategy"])
		}
	})

	t.Run("method mismatch", func(t *testing.T) {
		resp, err := e.ProxyRequest(ctx, &ProxyRequest{
			Method:  "DELETE",
			Path:    "/api/users/1",
			Service: "api",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		// 默认 round-robin
		if resp.Headers["X-Strategy"] != string(RouteRoundRobin) {
			t.Errorf("expected default round_robin, got %s", resp.Headers["X-Strategy"])
		}
	})
}

// ==================== CircuitBreaker Tests ====================

func TestCircuitBreakerClosed(t *testing.T) {
	cb := NewCircuitBreaker(zap.NewNop(), "test", nil)

	if cb.State() != CircuitClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}

	if !cb.Allow() {
		t.Error("should allow in closed state")
	}

	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Error("should remain closed after successes")
	}
}

func TestCircuitBreakerOpen(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 3
	cfg.Timeout = 1
	cb := NewCircuitBreaker(zap.NewNop(), "test", cfg)

	// 触发熔断
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Errorf("expected open, got %s", cb.State())
	}

	if cb.Allow() {
		t.Error("should not allow in open state")
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.SuccessThreshold = 2
	cfg.Timeout = 0 // 立即超时
	cb := NewCircuitBreaker(zap.NewNop(), "test", cfg)

	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	// 等待超时（timeout=0，立即转半开）
	time.Sleep(10 * time.Millisecond)

	if !cb.Allow() {
		t.Fatal("should allow after timeout (half-open)")
	}

	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected half-open, got %s", cb.State())
	}
}

func TestCircuitBreakerRecovery(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.SuccessThreshold = 2
	cfg.Timeout = 0
	cb := NewCircuitBreaker(zap.NewNop(), "test", cfg)

	// 触发熔断
	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(10 * time.Millisecond)
	cb.Allow() // 转半开

	// 成功恢复
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after recovery, got %s", cb.State())
	}
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.Timeout = 0
	cb := NewCircuitBreaker(zap.NewNop(), "test", cfg)

	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(10 * time.Millisecond)
	cb.Allow() // 转半开

	// 半开失败
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Errorf("expected open after half-open failure, got %s", cb.State())
	}
}

func TestCircuitBreakerFailureRate(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 100 // 很高，不会直接触发
	cfg.MinRequests = 10
	cfg.FailureRate = 0.5
	cb := NewCircuitBreaker(zap.NewNop(), "test", cfg)

	// 发送10个请求，6个失败
	for i := 0; i < 4; i++ {
		cb.RecordSuccess()
	}
	for i := 0; i < 6; i++ {
		cb.RecordFailure()
	}

	if cb.State() != CircuitOpen {
		t.Errorf("expected open due to failure rate, got %s", cb.State())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cb := NewCircuitBreaker(zap.NewNop(), "test", cfg)

	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	cb.Reset()
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after reset, got %s", cb.State())
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := NewCircuitBreaker(zap.NewNop(), "svc", nil)
	cb.RecordSuccess()
	cb.RecordFailure()

	stats := cb.GetStats()
	if stats["name"] != "svc" {
		t.Errorf("expected svc, got %v", stats["name"])
	}
	if stats["failures"] != 1 {
		t.Errorf("expected 1 failure, got %v", stats["failures"])
	}
}

// ==================== Tracer Tests ====================

func TestTracerStartSpan(t *testing.T) {
	tracer := NewTracer(zap.NewNop(), 1.0)

	span := tracer.StartSpan("/api/users", "user-service")
	if span.TraceID == "" {
		t.Error("expected non-empty trace ID")
	}
	if span.SpanID == "" {
		t.Error("expected non-empty span ID")
	}
	if span.Name != "/api/users" {
		t.Errorf("expected /api/users, got %s", span.Name)
	}
	if span.Service != "user-service" {
		t.Errorf("expected user-service, got %s", span.Service)
	}
}

func TestTracerChildSpan(t *testing.T) {
	tracer := NewTracer(zap.NewNop(), 1.0)

	parent := tracer.StartSpan("parent", "svc-a")
	child := tracer.StartChildSpan(parent, "child", "svc-b")

	if child.TraceID != parent.TraceID {
		t.Error("child should share parent trace ID")
	}
	if child.ParentID != parent.SpanID {
		t.Error("child should reference parent span ID")
	}
	if child.SpanID == parent.SpanID {
		t.Error("child should have unique span ID")
	}
}

func TestTracerRecordSpan(t *testing.T) {
	tracer := NewTracer(zap.NewNop(), 1.0)

	span := tracer.StartSpan("test", "svc")
	span.EndTime = time.Now()
	span.Duration = span.EndTime.Sub(span.StartTime)
	span.Status = "ok"

	tracer.RecordSpan(span)

	spans := tracer.GetRecentSpans(10)
	if len(spans) != 1 {
		t.Errorf("expected 1 span, got %d", len(spans))
	}
}

func TestTracerGetTrace(t *testing.T) {
	tracer := NewTracer(zap.NewNop(), 1.0)

	parent := tracer.StartSpan("req", "gateway")
	child := tracer.StartChildSpan(parent, "db-query", "db")

	parent.EndTime = time.Now()
	child.EndTime = time.Now()

	tracer.RecordSpan(parent)
	tracer.RecordSpan(child)

	trace := tracer.GetTrace(parent.TraceID)
	if len(trace) != 2 {
		t.Errorf("expected 2 spans in trace, got %d", len(trace))
	}
}

func TestTracerSampling(t *testing.T) {
	// 0% 采样率
	tracer := NewTracer(zap.NewNop(), 0)
	span := tracer.StartSpan("test", "svc")
	tracer.RecordSpan(span)

	spans := tracer.GetRecentSpans(10)
	if len(spans) != 0 {
		t.Errorf("expected 0 spans with 0%% sample rate, got %d", len(spans))
	}
}

func TestTracerAddEvent(t *testing.T) {
	tracer := NewTracer(zap.NewNop(), 1.0)
	span := tracer.StartSpan("test", "svc")

	tracer.AddEvent(span, "cache_miss", map[string]string{"key": "user:123"})

	if len(span.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(span.Events))
	}
	if span.Events[0].Name != "cache_miss" {
		t.Errorf("expected cache_miss, got %s", span.Events[0].Name)
	}
}

func TestTracerStats(t *testing.T) {
	tracer := NewTracer(zap.NewNop(), 0.5)

	span := tracer.StartSpan("test", "svc")
	tracer.RecordSpan(span)

	stats := tracer.GetStats()
	if stats["sample_rate"] != 0.5 {
		t.Errorf("expected 0.5, got %v", stats["sample_rate"])
	}
}

func TestTracerClearSpans(t *testing.T) {
	tracer := NewTracer(zap.NewNop(), 1.0)
	span := tracer.StartSpan("test", "svc")
	tracer.RecordSpan(span)

	tracer.ClearSpans()
	spans := tracer.GetRecentSpans(10)
	if len(spans) != 0 {
		t.Errorf("expected 0 spans after clear, got %d", len(spans))
	}
}

// ==================== MetricsCollector Tests ====================

func TestMetricsRecord(t *testing.T) {
	mc := NewMetricsCollector(zap.NewNop())

	mc.Record(MetricPoint{
		Name:  "request_count",
		Type:  "counter",
		Value: 1,
		Labels: map[string]string{
			"service": "test",
		},
	})

	metrics := mc.GetMetrics("request_count")
	if len(metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Value != 1 {
		t.Errorf("expected value 1, got %f", metrics[0].Value)
	}
}

func TestMetricsGetAll(t *testing.T) {
	mc := NewMetricsCollector(zap.NewNop())

	mc.Record(MetricPoint{Name: "a", Value: 1})
	mc.Record(MetricPoint{Name: "b", Value: 2})
	mc.Record(MetricPoint{Name: "a", Value: 3})

	aMetrics := mc.GetMetrics("a")
	if len(aMetrics) != 2 {
		t.Errorf("expected 2 'a' metrics, got %d", len(aMetrics))
	}

	all := mc.GetMetrics("")
	if len(all) != 3 {
		t.Errorf("expected 3 total metrics, got %d", len(all))
	}
}

func TestMetricsSummary(t *testing.T) {
	mc := NewMetricsCollector(zap.NewNop())

	mc.Record(MetricPoint{Name: "req", Value: 1})
	mc.Record(MetricPoint{Name: "req", Value: 2})
	mc.Record(MetricPoint{Name: "latency", Value: 100})

	summary := mc.GetMetricsSummary()
	if summary["total_points"] != 3 {
		t.Errorf("expected 3 total points, got %v", summary["total_points"])
	}

	names, ok := summary["metric_names"].(map[string]int)
	if !ok {
		t.Fatal("expected metric_names map")
	}
	if names["req"] != 2 {
		t.Errorf("expected 2 req metrics, got %d", names["req"])
	}
}

func TestRecordRequestMetrics(t *testing.T) {
	mc := NewMetricsCollector(zap.NewNop())

	mc.RecordRequestMetrics("user-svc", 200, 50*time.Millisecond)
	mc.RecordRequestMetrics("user-svc", 500, 200*time.Millisecond)

	metrics := mc.GetMetrics("")
	if len(metrics) != 4 { // 2 metrics per request
		t.Errorf("expected 4 metrics, got %d", len(metrics))
	}
}

func TestMetricsClear(t *testing.T) {
	mc := NewMetricsCollector(zap.NewNop())

	mc.Record(MetricPoint{Name: "test", Value: 1})
	mc.Clear()

	metrics := mc.GetMetrics("")
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics after clear, got %d", len(metrics))
	}
}

func TestHTTPStatusClass(t *testing.T) {
	tests := []struct {
		status   int
		expected string
	}{
		{200, "2xx"},
		{301, "3xx"},
		{404, "4xx"},
		{500, "5xx"},
		{100, "other"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := httpStatusClass(tt.status); got != tt.expected {
				t.Errorf("httpStatusClass(%d) = %s, want %s", tt.status, got, tt.expected)
			}
		})
	}
}

func TestGenerateTraceID(t *testing.T) {
	id1 := generateTraceID()
	id2 := generateTraceID()
	if id1 == id2 {
		t.Error("expected unique trace IDs")
	}
	if len(id1) != 32 { // 16 bytes hex = 32 chars
		t.Errorf("expected 32 chars, got %d", len(id1))
	}
}
