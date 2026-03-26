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

// ==================== Gateway Extended Tests ====================

func TestNewGateway_NoRateLimit(t *testing.T) {
	cfg := &config.GatewayConfig{
		DefaultBackend: "ollama",
		RateLimit: config.RateLimitConfig{
			Enabled: false,
		},
	}

	gateway := NewGateway(cfg)

	require.NotNil(t, gateway)
	assert.Nil(t, gateway.limiter)
}

func TestGateway_IsHealthy(t *testing.T) {
	gateway := NewGateway(&config.GatewayConfig{})

	assert.True(t, gateway.IsHealthy())
}

func TestGateway_SetModelRouting(t *testing.T) {
	gateway := NewGateway(&config.GatewayConfig{})

	gateway.SetModelRouting("gpt-4", BackendOllama)

	// Verify route is set
	assert.Equal(t, BackendOllama, gateway.modelRouter.Route("gpt-4"))
}

func TestGateway_GetMetrics_Extended(t *testing.T) {
	gateway := NewGateway(&config.GatewayConfig{})

	metrics := gateway.GetMetrics()

	require.NotNil(t, metrics)
	assert.Equal(t, int64(0), metrics.TotalRequests)
	assert.Equal(t, int64(0), metrics.TotalErrors)
}

// ==================== ModelRouter Extended Tests ====================

func TestNewModelRouter_Extended(t *testing.T) {
	router := NewModelRouter()

	require.NotNil(t, router)
	assert.NotNil(t, router.routes)
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

// ==================== GatewayMonitor Extended Tests ====================

func TestNewGatewayMonitor_Extended(t *testing.T) {
	monitor := NewGatewayMonitor()

	require.NotNil(t, monitor)
}

func TestGatewayMonitor_RecordError_Extended(t *testing.T) {
	monitor := NewGatewayMonitor()

	monitor.RecordError(BackendOllama, "chat", assert.AnError)
	monitor.RecordError(BackendOllama, "chat", assert.AnError)

	metrics := monitor.GetMetrics()

	assert.Equal(t, int64(2), metrics.TotalErrors)
}

func TestGatewayMonitor_RecordSuccess_Extended(t *testing.T) {
	monitor := NewGatewayMonitor()

	monitor.RecordRequest(BackendOllama, "chat", 100*time.Millisecond)
	monitor.RecordSuccess(BackendOllama, "chat", 500, 100*time.Millisecond)

	metrics := monitor.GetMetrics()

	assert.Equal(t, int64(500), metrics.TotalTokens)
}

func TestGatewayMonitor_GetMetrics_Extended(t *testing.T) {
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
		TotalRequests:   100,
		TotalErrors:     5,
		TotalTokens:     50000,
		AvgLatencyMs:    150,
		RequestsByType:  map[string]int64{"chat": 80, "embed": 20},
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

// mockTestBackend is reserved for future test extensions
var _ Backend = (*mockTestBackend)(nil)

type mockTestBackend struct {
	name    BackendType
	healthy bool
}

func (m *mockTestBackend) Name() BackendType {
	return m.name
}

func (m *mockTestBackend) IsHealthy(ctx context.Context) bool {
	return m.healthy
}

func (m *mockTestBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{ID: "test"}, nil
}

func (m *mockTestBackend) StreamChat(ctx context.Context, req *ChatRequest, callback func(chunk string) error) error {
	return callback("test")
}

func (m *mockTestBackend) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	return &EmbedResponse{}, nil
}

func (m *mockTestBackend) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{{Name: "test-model"}}, nil
}

func (m *mockTestBackend) LoadModel(ctx context.Context, modelName string) error {
	return nil
}

func (m *mockTestBackend) UnloadModel(ctx context.Context, modelName string) error {
	return nil
}

func (m *mockTestBackend) GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	return &ModelInfo{Name: modelName}, nil
}
