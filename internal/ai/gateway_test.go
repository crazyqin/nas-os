// Package ai provides AI service integration for NAS-OS
package ai

import (
	"context"
	"sync"
	"testing"
	"time"

	"nas-os/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Gateway Tests ====================

func TestNewGateway(t *testing.T) {
	cfg := &config.GatewayConfig{
		DefaultBackend: "ollama",
		RateLimit: config.RateLimitConfig{
			Enabled:          true,
			RequestsPerMin:   60,
			TokensPerMin:     10000,
			BurstSize:        10,
			ConcurrencyLimit: 5,
		},
	}

	gateway := NewGateway(cfg)

	require.NotNil(t, gateway)
	assert.Equal(t, BackendOllama, gateway.defaultBkd)
	assert.NotNil(t, gateway.limiter)
	assert.NotNil(t, gateway.modelRouter)
	assert.NotNil(t, gateway.monitor)
	assert.True(t, gateway.IsHealthy())
}

func TestNewGateway_DefaultBackend(t *testing.T) {
	cfg := &config.GatewayConfig{
		// No default backend specified
	}

	gateway := NewGateway(cfg)

	assert.Equal(t, BackendOllama, gateway.defaultBkd) // Should default to ollama
}

func TestGateway_RegisterBackend(t *testing.T) {
	gateway := NewGateway(&config.GatewayConfig{})

	backend := &mockBackend{name: BackendOllama}
	err := gateway.RegisterBackend(BackendOllama, backend)

	require.NoError(t, err)
	assert.Len(t, gateway.backends, 1)
}

func TestGateway_RegisterBackend_Duplicate(t *testing.T) {
	gateway := NewGateway(&config.GatewayConfig{})

	backend := &mockBackend{name: BackendOllama}
	_ = gateway.RegisterBackend(BackendOllama, backend)

	// Try to register again
	err := gateway.RegisterBackend(BackendOllama, backend)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestGateway_GetBackendStatus(t *testing.T) {
	gateway := NewGateway(&config.GatewayConfig{})

	// Register backends
	gateway.RegisterBackend(BackendOllama, &mockBackend{name: BackendOllama, healthy: true})
	gateway.RegisterBackend(BackendLocalAI, &mockBackend{name: BackendLocalAI, healthy: false})

	ctx := context.Background()
	status := gateway.GetBackendStatus(ctx)

	assert.Len(t, status, 2)
	assert.True(t, status[BackendOllama].Healthy)
	assert.False(t, status[BackendLocalAI].Healthy)
}

func TestGateway_SetModelRouting(t *testing.T) {
	gateway := NewGateway(&config.GatewayConfig{})

	gateway.SetModelRouting("gpt-4", BackendOllama)

	// Verify route is set
	assert.Equal(t, BackendOllama, gateway.modelRouter.Route("gpt-4"))
}

func TestGateway_GetMetrics(t *testing.T) {
	gateway := NewGateway(&config.GatewayConfig{})

	metrics := gateway.GetMetrics()

	require.NotNil(t, metrics)
	assert.Equal(t, int64(0), metrics.TotalRequests)
	assert.Equal(t, int64(0), metrics.TotalErrors)
}

// ==================== ModelRouter Tests ====================

func TestNewModelRouter(t *testing.T) {
	router := NewModelRouter()

	require.NotNil(t, router)
	assert.NotNil(t, router.routes)
}

func TestModelRouter_SetRoute(t *testing.T) {
	router := NewModelRouter()

	router.SetRoute("llama2", BackendOllama)
	router.SetRoute("gpt-4", BackendVLLM)

	assert.Equal(t, BackendOllama, router.Route("llama2"))
	assert.Equal(t, BackendVLLM, router.Route("gpt-4"))
}

func TestModelRouter_Route_NoRoute(t *testing.T) {
	router := NewModelRouter()

	// No route set, should return empty
	result := router.Route("unknown-model")
	assert.Empty(t, result)
}

func TestModelRouter_ConcurrentAccess(t *testing.T) {
	router := NewModelRouter()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			router.SetRoute(string(rune('a'+i%26)), BackendType("backend"))
			_ = router.Route("test")
		}(i)
	}

	wg.Wait()
}

// ==================== GatewayMonitor Tests ====================

func TestNewGatewayMonitor(t *testing.T) {
	monitor := NewGatewayMonitor()

	require.NotNil(t, monitor)
}

func TestGatewayMonitor_RecordRequest(t *testing.T) {
	monitor := NewGatewayMonitor()

	monitor.RecordRequest(BackendOllama, "chat", 100*time.Millisecond)
	monitor.RecordRequest(BackendOllama, "chat", 200*time.Millisecond)
	monitor.RecordRequest(BackendLocalAI, "embed", 50*time.Millisecond)

	metrics := monitor.GetMetrics()

	assert.Equal(t, int64(3), metrics.TotalRequests)
}

func TestGatewayMonitor_RecordError(t *testing.T) {
	monitor := NewGatewayMonitor()

	monitor.RecordError(BackendOllama, "chat", assert.AnError)
	monitor.RecordError(BackendOllama, "chat", assert.AnError)

	metrics := monitor.GetMetrics()

	assert.Equal(t, int64(2), metrics.TotalErrors)
}

func TestGatewayMonitor_RecordSuccess(t *testing.T) {
	monitor := NewGatewayMonitor()

	monitor.RecordRequest(BackendOllama, "chat", 100*time.Millisecond)
	monitor.RecordSuccess(BackendOllama, "chat", 500, 100*time.Millisecond)

	metrics := monitor.GetMetrics()

	assert.Equal(t, int64(500), metrics.TotalTokens)
}

func TestGatewayMonitor_GetMetrics(t *testing.T) {
	monitor := NewGatewayMonitor()

	// Record various operations
	monitor.RecordRequest(BackendOllama, "chat", 100*time.Millisecond)
	monitor.RecordRequest(BackendOllama, "embed", 50*time.Millisecond)
	monitor.RecordSuccess(BackendOllama, "chat", 100, 100*time.Millisecond)
	monitor.RecordError(BackendOllama, "chat", assert.AnError)

	metrics := monitor.GetMetrics()

	assert.Equal(t, int64(2), metrics.TotalRequests)
	assert.Equal(t, int64(1), metrics.TotalErrors)
	assert.Equal(t, int64(100), metrics.TotalTokens)
	assert.NotNil(t, metrics.RequestsByType)
	assert.NotNil(t, metrics.ErrorsByBackend)
}

// ==================== RateLimiter Tests ====================

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(60, 10000, 10, 5)

	require.NotNil(t, rl)
	assert.Equal(t, 60, rl.requestsPerMin)
	assert.Equal(t, 10000, rl.tokensPerMin)
	assert.Equal(t, 10, rl.burstSize)
	assert.Equal(t, 5, rl.concurrencyLimit)
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(60, 10000, 10, 5)

	ctx := context.Background()
	err := rl.Wait(ctx)

	assert.NoError(t, err)
}

func TestRateLimiter_Wait_ContextCancelled(t *testing.T) {
	rl := NewRateLimiter(60, 10000, 1, 1) // Very small limits

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Wait with cancelled context
	// The behaviour depends on whether tokens are available
	// Just verify it doesn't panic
	_ = rl.Wait(ctx)
}

// ==================== BackendStatus Tests ====================

func TestBackendStatus(t *testing.T) {
	status := BackendStatus{
		Name:    BackendOllama,
		Healthy: true,
	}

	assert.Equal(t, BackendOllama, status.Name)
	assert.True(t, status.Healthy)
}

// ==================== GatewayMetrics Tests ====================

func TestGatewayMetrics(t *testing.T) {
	metrics := GatewayMetrics{
		TotalRequests:  100,
		TotalErrors:    5,
		TotalTokens:    50000,
		AvgLatencyMs:   150,
		RequestsByType: map[string]int64{"chat": 80, "embed": 20},
		ErrorsByBackend: map[string]int64{"ollama": 3, "localai": 2},
		BackendMetrics: map[string]BackendMetrics{
			"ollama": {Requests: 80, Errors: 3, Tokens: 40000},
		},
	}

	assert.Equal(t, int64(100), metrics.TotalRequests)
	assert.Equal(t, int64(5), metrics.TotalErrors)
	assert.Equal(t, int64(50000), metrics.TotalTokens)
	assert.Len(t, metrics.RequestsByType, 2)
	assert.Len(t, metrics.ErrorsByBackend, 2)
}

func TestBackendMetrics(t *testing.T) {
	bm := BackendMetrics{
		Requests:     100,
		Errors:       2,
		Tokens:       5000,
		AvgLatencyMs: 120,
	}

	assert.Equal(t, int64(100), bm.Requests)
	assert.Equal(t, int64(2), bm.Errors)
	assert.Equal(t, int64(5000), bm.Tokens)
	assert.Equal(t, int64(120), bm.AvgLatencyMs)
}

// ==================== Mock Backend ====================

type mockBackend struct {
	name    BackendType
	healthy bool
}

func (m *mockBackend) Name() BackendType {
	return m.name
}

func (m *mockBackend) IsHealthy(ctx context.Context) bool {
	return m.healthy
}

func (m *mockBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{ID: "test"}, nil
}

func (m *mockBackend) StreamChat(ctx context.Context, req *ChatRequest, callback func(chunk string) error) error {
	return callback("test")
}

func (m *mockBackend) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	return &EmbedResponse{}, nil
}

func (m *mockBackend) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{{Name: "test-model"}}, nil
}

func (m *mockBackend) LoadModel(ctx context.Context, modelName string) error {
	return nil
}

func (m *mockBackend) UnloadModel(ctx context.Context, modelName string) error {
	return nil
}

func (m *mockBackend) GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	return &ModelInfo{Name: modelName}, nil
}