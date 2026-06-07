// Package loadbalancer - 负载均衡器单元测试
package loadbalancer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// ============================================================
// 负载均衡器测试
// ============================================================

func TestBalancer_RoundRobin(t *testing.T) {
	config := LBConfig{
		Algorithm: AlgorithmRoundRobin,
		Backends: []BackendConfig{
			{ID: "1", URL: "http://backend1:8080", Weight: 1},
			{ID: "2", URL: "http://backend2:8080", Weight: 1},
			{ID: "3", URL: "http://backend3:8080", Weight: 1},
		},
	}

	balancer := NewBalancer(config)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// 测试轮询
	selected := make(map[string]int)
	for i := 0; i < 9; i++ {
		backend, err := balancer.Select(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		selected[backend.ID]++
	}

	// 每个后端应该被选中3次
	for id, count := range selected {
		if count != 3 {
			t.Errorf("backend %s selected %d times, expected 3", id, count)
		}
	}
}

func TestBalancer_WeightedRoundRobin(t *testing.T) {
	config := LBConfig{
		Algorithm: AlgorithmWeightedRoundRobin,
		Backends: []BackendConfig{
			{ID: "1", URL: "http://backend1:8080", Weight: 5},
			{ID: "2", URL: "http://backend2:8080", Weight: 3},
			{ID: "3", URL: "http://backend3:8080", Weight: 2},
		},
	}

	balancer := NewBalancer(config)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// 测试加权轮询 - 应该返回后端
	selected := make(map[string]int)
	for i := 0; i < 10; i++ {
		backend, err := balancer.Select(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		selected[backend.ID]++
	}

	// 应该所有请求都被分配
	total := 0
	for _, count := range selected {
		total += count
	}
	if total != 10 {
		t.Errorf("expected 10 total selections, got %d", total)
	}
}

func TestBalancer_LeastConn(t *testing.T) {
	config := LBConfig{
		Algorithm: AlgorithmLeastConn,
		Backends: []BackendConfig{
			{ID: "1", URL: "http://backend1:8080", Weight: 1},
			{ID: "2", URL: "http://backend2:8080", Weight: 1},
			{ID: "3", URL: "http://backend3:8080", Weight: 1},
		},
	}

	balancer := NewBalancer(config)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// 模拟不同连接数
	backend1 := balancer.GetBackend("1")
	backend2 := balancer.GetBackend("2")
	backend3 := balancer.GetBackend("3")

	backend1.ActiveConns = 10
	backend2.ActiveConns = 5
	backend3.ActiveConns = 3

	// 应该选择连接数最少的后端
	backend, err := balancer.Select(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if backend.ID != "3" {
		t.Errorf("selected backend %s, expected 3", backend.ID)
	}
}

func TestBalancer_IPHash(t *testing.T) {
	config := LBConfig{
		Algorithm: AlgorithmIPHash,
		Backends: []BackendConfig{
			{ID: "1", URL: "http://backend1:8080", Weight: 1},
			{ID: "2", URL: "http://backend2:8080", Weight: 1},
		},
	}

	balancer := NewBalancer(config)

	// 相同IP应该总是选择相同的后端
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "192.168.1.100:12345"

	// 多次测试相同IP
	firstBackend, _ := balancer.Select(req1)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		backend, _ := balancer.Select(req)
		if backend.ID != firstBackend.ID {
			t.Errorf("same IP selected different backends: %s vs %s", firstBackend.ID, backend.ID)
		}
	}
}

func TestBalancer_NoHealthyBackend(t *testing.T) {
	config := LBConfig{
		Algorithm: AlgorithmRoundRobin,
		Backends: []BackendConfig{
			{ID: "1", URL: "http://backend1:8080", Weight: 1},
		},
	}

	balancer := NewBalancer(config)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// 将所有后端标记为不健康
	backend := balancer.GetBackend("1")
	backend.SetHealthy(false)

	_, err := balancer.Select(req)
	if err == nil {
		t.Error("expected error when no healthy backend available")
	}
}

// ============================================================
// 健康检查测试
// ============================================================

func TestHealthChecker_HTTPCheck(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := HealthCheckConfig{
		Enabled:            true,
		Type:               HealthCheckHTTP,
		Interval:           1 * time.Second,
		Timeout:            1 * time.Second,
		HealthyThreshold:   1,
		UnhealthyThreshold: 1,
		Path:               "/health",
		ExpectedStatus:     http.StatusOK,
	}

	checker := NewHealthChecker(config)
	backend := &Backend{
		ID:        "1",
		URL:       server.URL,
		IsHealthy: true,
	}

	checker.SetBackends([]*Backend{backend})

	// 执行检查
	result := checker.CheckNow(backend)
	if !result.Healthy {
		t.Errorf("expected healthy, got unhealthy: %s", result.Error)
	}
}

func TestHealthChecker_TCPCheck(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	config := HealthCheckConfig{
		Enabled:            true,
		Type:               HealthCheckTCP,
		Interval:           1 * time.Second,
		Timeout:            1 * time.Second,
		HealthyThreshold:   1,
		UnhealthyThreshold: 1,
	}

	checker := NewHealthChecker(config)
	backend := &Backend{
		ID:        "1",
		URL:       server.URL,
		IsHealthy: true,
	}

	checker.SetBackends([]*Backend{backend})

	// 执行检查
	result := checker.CheckNow(backend)
	if !result.Healthy {
		t.Errorf("expected healthy, got unhealthy: %s", result.Error)
	}
}

func TestHealthChecker_CustomProbe(t *testing.T) {
	config := HealthCheckConfig{
		Enabled:            true,
		Type:               HealthCheckCustom,
		Interval:           1 * time.Second,
		Timeout:            1 * time.Second,
		HealthyThreshold:   1,
		UnhealthyThreshold: 1,
	}

	checker := NewHealthChecker(config)
	backend := &Backend{
		ID:        "1",
		URL:       "http://custom:8080",
		IsHealthy: true,
	}

	// 注册自定义探针
	checker.RegisterProbe("1", func(ctx context.Context, b *Backend) error {
		return nil // 总是返回成功
	})

	checker.SetBackends([]*Backend{backend})

	// 执行检查
	result := checker.CheckNow(backend)
	if !result.Healthy {
		t.Errorf("expected healthy, got unhealthy: %s", result.Error)
	}
}

// ============================================================
// 熔断器测试
// ============================================================

func TestCircuitBreaker_Closed(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	}

	cb := NewCircuitBreaker(config)

	// 正常执行
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cb.GetState() != CircuitClosed {
		t.Errorf("expected closed state, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_Open(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	}

	cb := NewCircuitBreaker(config)

	// 触发失败
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return &testError{}
		})
	}

	// 熔断器应该打开
	if cb.GetState() != CircuitOpen {
		t.Errorf("expected open state, got %s", cb.GetState())
	}

	// 后续请求应该被拒绝
	err := cb.Execute(func() error {
		return nil
	})

	if !IsCircuitOpen(err) {
		t.Errorf("expected circuit open error, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		MaxRequests:      1,
	}

	cb := NewCircuitBreaker(config)

	// 触发失败
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return &testError{}
		})
	}

	// 等待超时
	time.Sleep(150 * time.Millisecond)

	// 应该允许一个请求
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 应该还在半开状态
	if cb.GetState() != CircuitHalfOpen {
		t.Errorf("expected half-open state, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		MaxRequests:      2,
	}

	cb := NewCircuitBreaker(config)

	// 触发失败
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return &testError{}
		})
	}

	// 等待超时
	time.Sleep(150 * time.Millisecond)

	// 成功执行2次
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return nil
		})
	}

	// 应该恢复到关闭状态
	if cb.GetState() != CircuitClosed {
		t.Errorf("expected closed state, got %s", cb.GetState())
	}
}

// testError 测试错误
type testError struct{}

func (e *testError) Error() string {
	return "test error"
}

// ============================================================
// 限流器测试
// ============================================================

func TestTokenBucketLimiter_Allow(t *testing.T) {
	config := RateLimitConfig{
		Enabled:   true,
		Algorithm: RateLimitTokenBucket,
		Rate:      10,
		Burst:     20,
	}

	limiter := NewTokenBucketLimiter(config)

	// 允许突发请求
	for i := 0; i < 20; i++ {
		if !limiter.Allow("test") {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// 第21个请求应该被拒绝
	if limiter.Allow("test") {
		t.Error("request should be rejected after burst exhausted")
	}
}

func TestTokenBucketLimiter_Refill(t *testing.T) {
	config := RateLimitConfig{
		Enabled:   true,
		Algorithm: RateLimitTokenBucket,
		Rate:      100,
		Burst:     10,
	}

	limiter := NewTokenBucketLimiter(config)

	// 耗尽令牌
	for i := 0; i < 10; i++ {
		limiter.Allow("test")
	}

	// 等待100ms，应该补充10个令牌
	time.Sleep(100 * time.Millisecond)

	// 应该允许10个请求
	allowed := 0
	for i := 0; i < 20; i++ {
		if limiter.Allow("test") {
			allowed++
		}
	}

	if allowed < 8 || allowed > 12 {
		t.Errorf("expected around 10 allowed requests, got %d", allowed)
	}
}

func TestSlidingWindowLimiter_Allow(t *testing.T) {
	config := RateLimitConfig{
		Enabled:   true,
		Algorithm: RateLimitSlidingWindow,
		Rate:      10,
		Burst:     10,
	}

	limiter := NewSlidingWindowLimiter(config)

	// 允许请求
	for i := 0; i < 10; i++ {
		if !limiter.Allow("test") {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// 超出限制
	if limiter.Allow("test") {
		t.Error("request should be rejected")
	}
}

func TestIPRateLimiter_Ban(t *testing.T) {
	config := RateLimitConfig{
		Enabled:   true,
		Algorithm: RateLimitTokenBucket,
		Rate:      100,
		Burst:     100,
	}

	limiter := NewIPRateLimiter(config)

	// 封禁IP
	limiter.BanIP("192.168.1.100", 1*time.Second)

	// 检查是否被封禁
	if !limiter.IsBanned("192.168.1.100") {
		t.Error("IP should be banned")
	}

	// 请求应该被拒绝
	if limiter.Allow("192.168.1.100") {
		t.Error("banned IP should not be allowed")
	}

	// 等待解封
	time.Sleep(1100 * time.Millisecond)

	if limiter.IsBanned("192.168.1.100") {
		t.Error("IP should be unbanned")
	}

	if !limiter.Allow("192.168.1.100") {
		t.Error("unbanned IP should be allowed")
	}
}

// ============================================================
// 熔断器管理测试
// ============================================================

func TestBackendCircuitBreakers_Get(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	}

	bcb := NewBackendCircuitBreakers(config)

	// 获取熔断器
	cb1 := bcb.Get("backend1")
	cb2 := bcb.Get("backend2")

	// 相同ID应该返回相同的熔断器
	cb1Again := bcb.Get("backend1")
	if cb1 != cb1Again {
		t.Error("same backend ID should return same circuit breaker")
	}

	// 不同ID应该返回不同的熔断器
	if cb1 == cb2 {
		t.Error("different backend ID should return different circuit breaker")
	}
}

func TestBackendCircuitBreakers_Isolation(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	}

	bcb := NewBackendCircuitBreakers(config)

	cb1 := bcb.Get("backend1")
	cb2 := bcb.Get("backend2")

	// 触发cb1失败
	for i := 0; i < 3; i++ {
		cb1.Execute(func() error {
			return &testError{}
		})
	}

	// cb1应该打开
	if cb1.GetState() != CircuitOpen {
		t.Errorf("cb1 expected open state, got %s", cb1.GetState())
	}

	// cb2应该还是关闭状态
	if cb2.GetState() != CircuitClosed {
		t.Errorf("cb2 expected closed state, got %s", cb2.GetState())
	}
}

// ============================================================
// 中间件测试
// ============================================================

func TestLoggingMiddleware(t *testing.T) {
	config := LoggingConfig{
		Enabled:   true,
		Format:    "json",
		Level:     "info",
		AccessLog: true,
	}

	middleware := NewLoggingMiddleware(config)
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCORSMiddleware(t *testing.T) {
	config := CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		MaxAge:         3600,
	}

	middleware := NewCORSMiddleware(config)
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 测试预检请求
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Error("missing CORS headers")
	}
}

func TestCompressionMiddleware(t *testing.T) {
	config := CompressionConfig{
		Enabled: true,
		MinSize: 100,
		Types:   []string{"text/plain"},
		Level:   6,
	}

	middleware := NewCompressionMiddleware(config)
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World! This is a test response for compression."))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected gzip encoding")
	}
}

func TestCacheMiddleware(t *testing.T) {
	config := CacheConfig{
		Enabled:     true,
		TTL:         1 * time.Minute,
		MaxSize:     100,
		Methods:     []string{"GET"},
		StatusCodes: []int{200},
	}

	requestCount := 0
	middleware := NewCacheMiddleware(config)
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("cached response"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// 第一次请求
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req)

	// 第二次请求
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	// 应该只执行一次处理器
	if requestCount != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}

	// 第二次请求应该有缓存头
	if w2.Header().Get("X-Cache") != "HIT" {
		t.Error("expected cache HIT")
	}
}

// ============================================================
// 并发测试
// ============================================================

func TestBalancer_Concurrent(t *testing.T) {
	config := LBConfig{
		Algorithm: AlgorithmRoundRobin,
		Backends: []BackendConfig{
			{ID: "1", URL: "http://backend1:8080", Weight: 1},
			{ID: "2", URL: "http://backend2:8080", Weight: 1},
			{ID: "3", URL: "http://backend3:8080", Weight: 1},
		},
	}

	balancer := NewBalancer(config)
	var wg sync.WaitGroup

	// 并发请求
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			_, err := balancer.Select(req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 10,
		SuccessThreshold: 5,
		Timeout:          1 * time.Second,
	}

	cb := NewCircuitBreaker(config)
	var wg sync.WaitGroup

	// 并发执行
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cb.Execute(func() error {
				if i%2 == 0 {
					return nil
				}
				return &testError{}
			})
		}(i)
	}

	wg.Wait()
}

func TestRateLimiter_Concurrent(t *testing.T) {
	config := RateLimitConfig{
		Enabled:   true,
		Algorithm: RateLimitTokenBucket,
		Rate:      1000,
		Burst:     1000,
	}

	limiter := NewTokenBucketLimiter(config)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0

	// 并发请求
	for i := 0; i < 2000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow("test") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// 应该大约1000个请求被允许
	if allowed < 800 || allowed > 1200 {
		t.Errorf("expected around 1000 allowed requests, got %d", allowed)
	}
}

// ============================================================
// 配置测试
// ============================================================

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  LBConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: LBConfig{
				ListenAddr: ":8080",
				Algorithm:  AlgorithmRoundRobin,
				Backends: []BackendConfig{
					{URL: "http://backend1:8080", Weight: 1},
				},
			},
			wantErr: false,
		},
		{
			name: "missing listen addr",
			config: LBConfig{
				Algorithm: AlgorithmRoundRobin,
				Backends: []BackendConfig{
					{URL: "http://backend1:8080", Weight: 1},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid algorithm",
			config: LBConfig{
				ListenAddr: ":8080",
				Algorithm:  "invalid",
				Backends: []BackendConfig{
					{URL: "http://backend1:8080", Weight: 1},
				},
			},
			wantErr: true,
		},
		{
			name: "no backends",
			config: LBConfig{
				ListenAddr: ":8080",
				Algorithm:  AlgorithmRoundRobin,
				Backends:   []BackendConfig{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMergeConfig(t *testing.T) {
	base := LBConfig{
		ListenAddr: ":8080",
		Algorithm:  AlgorithmRoundRobin,
		Backends: []BackendConfig{
			{ID: "1", URL: "http://backend1:8080"},
		},
	}

	override := LBConfig{
		ListenAddr: ":9090",
		Backends: []BackendConfig{
			{ID: "2", URL: "http://backend2:8080"},
		},
	}

	result := MergeConfig(base, override)

	if result.ListenAddr != ":9090" {
		t.Errorf("expected listen addr :9090, got %s", result.ListenAddr)
	}

	if len(result.Backends) != 1 {
		t.Errorf("expected 1 backend, got %d", len(result.Backends))
	}

	if result.Backends[0].ID != "2" {
		t.Errorf("expected backend ID 2, got %s", result.Backends[0].ID)
	}
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkBalancer_RoundRobin(b *testing.B) {
	config := LBConfig{
		Algorithm: AlgorithmRoundRobin,
		Backends: []BackendConfig{
			{ID: "1", URL: "http://backend1:8080", Weight: 1},
			{ID: "2", URL: "http://backend2:8080", Weight: 1},
			{ID: "3", URL: "http://backend3:8080", Weight: 1},
		},
	}

	balancer := NewBalancer(config)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		balancer.Select(req)
	}
}

func BenchmarkCircuitBreaker_Execute(b *testing.B) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 100,
		SuccessThreshold: 100,
		Timeout:          1 * time.Second,
	}

	cb := NewCircuitBreaker(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(func() error {
			return nil
		})
	}
}

func BenchmarkTokenBucketLimiter_Allow(b *testing.B) {
	config := RateLimitConfig{
		Enabled:   true,
		Algorithm: RateLimitTokenBucket,
		Rate:      1000000,
		Burst:     1000000,
	}

	limiter := NewTokenBucketLimiter(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow("test")
	}
}
