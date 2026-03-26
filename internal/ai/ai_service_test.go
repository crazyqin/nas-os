// Package ai provides AI service integration for NAS-OS
package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nas-os/pkg/config"
)

// ==================== Backend Tests ====================

func TestOllamaBackend_IsHealthy(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ollamaListResponse{Models: []ollamaModel{}})
		}
	}))
	defer server.Close()

	backend := NewOllamaBackend(
		BackendConfig{
			Type:     BackendOllama,
			Endpoint: server.URL,
		},
		"/tmp/models",
		0,
		"5m",
	)

	ctx := context.Background()
	if !backend.IsHealthy(ctx) {
		t.Error("Expected backend to be healthy")
	}
}

func TestOllamaBackend_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ollamaChatResponse{
				Model:     "llama2",
				Message:   ChatMessage{Role: "assistant", Content: "Hello!"},
				Done:      true,
				EvalCount: 10,
			})
		}
	}))
	defer server.Close()

	backend := NewOllamaBackend(
		BackendConfig{
			Type:     BackendOllama,
			Endpoint: server.URL,
		},
		"/tmp/models",
		0,
		"5m",
	)

	ctx := context.Background()
	resp, err := backend.Chat(ctx, &ChatRequest{
		Model:    "llama2",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("Expected at least one choice")
	}

	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("Expected 'Hello!', got '%s'", resp.Choices[0].Message.Content)
	}
}

func TestLocalAIBackend_IsHealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{},
			})
		}
	}))
	defer server.Close()

	backend := NewLocalAIBackend(
		BackendConfig{
			Type:     BackendLocalAI,
			Endpoint: server.URL,
		},
		4,
		4096,
	)

	ctx := context.Background()
	if !backend.IsHealthy(ctx) {
		t.Error("Expected backend to be healthy")
	}
}

func TestVLLMBackend_IsHealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{},
			})
		}
	}))
	defer server.Close()

	backend := NewVLLMBackend(
		BackendConfig{
			Type:     BackendVLLM,
			Endpoint: server.URL,
		},
		1,
		0.9,
	)

	ctx := context.Background()
	if !backend.IsHealthy(ctx) {
		t.Error("Expected backend to be healthy")
	}
}

// ==================== Gateway Tests ====================

func TestGateway_RegisterBackend(t *testing.T) {
	cfg := config.DefaultAIConfig()
	gateway := NewGateway(&cfg.Gateway)

	backend := NewOllamaBackend(
		BackendConfig{
			Type:     BackendOllama,
			Endpoint: "http://localhost:11434",
		},
		"/tmp/models",
		0,
		"5m",
	)

	err := gateway.RegisterBackend(BackendOllama, backend)
	if err != nil {
		t.Fatalf("Failed to register backend: %v", err)
	}

	// Try registering again - should fail
	err = gateway.RegisterBackend(BackendOllama, backend)
	if err == nil {
		t.Error("Expected error when registering duplicate backend")
	}
}

func TestGateway_RateLimiter(t *testing.T) {
	cfg := &config.GatewayConfig{
		DefaultBackend: "ollama",
		RateLimit: config.RateLimitConfig{
			Enabled:          true,
			RequestsPerMin:   60,
			TokensPerMin:     100000,
			BurstSize:        5,
			ConcurrencyLimit: 3,
		},
	}

	gateway := NewGateway(cfg)

	if gateway.limiter == nil {
		t.Error("Expected rate limiter to be initialized")
	}
}

func TestGateway_GetBackendStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ollamaListResponse{Models: []ollamaModel{}})
		}
	}))
	defer server.Close()

	cfg := config.DefaultAIConfig()
	gateway := NewGateway(&cfg.Gateway)

	backend := NewOllamaBackend(
		BackendConfig{
			Type:     BackendOllama,
			Endpoint: server.URL,
		},
		"/tmp/models",
		0,
		"5m",
	)

	gateway.RegisterBackend(BackendOllama, backend)

	ctx := context.Background()
	status := gateway.GetBackendStatus(ctx)

	if len(status) != 1 {
		t.Errorf("Expected 1 backend, got %d", len(status))
	}

	if !status[BackendOllama].Healthy {
		t.Error("Expected Ollama backend to be healthy")
	}
}

func TestModelRouter_Route(t *testing.T) {
	router := NewModelRouter()

	// No route set
	if router.Route("llama2") != "" {
		t.Error("Expected empty route for unset model")
	}

	// Set route
	router.SetRoute("llama2", BackendOllama)

	if router.Route("llama2") != BackendOllama {
		t.Error("Expected Ollama route for llama2")
	}
}

// ==================== Model Manager Tests ====================

func TestModelRegistry_Register(t *testing.T) {
	registry := NewModelRegistry()

	model := &LocalModel{
		Name:      "test-model",
		Source:    "ollama",
		Installed: true,
	}

	registry.Register("test-model", model)

	retrieved := registry.Get("test-model")
	if retrieved == nil {
		t.Fatal("Expected to retrieve model")
	}

	if retrieved.Name != "test-model" {
		t.Errorf("Expected 'test-model', got '%s'", retrieved.Name)
	}
}

func TestModelRegistry_List(t *testing.T) {
	registry := NewModelRegistry()

	registry.Register("model1", &LocalModel{Name: "model1"})
	registry.Register("model2", &LocalModel{Name: "model2"})

	models := registry.List()
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
}

func TestModelRegistry_Unregister(t *testing.T) {
	registry := NewModelRegistry()

	registry.Register("test-model", &LocalModel{Name: "test-model"})
	registry.Unregister("test-model")

	if registry.Get("test-model") != nil {
		t.Error("Expected model to be unregistered")
	}
}

// ==================== Rate Limiter Tests ====================

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(60, 100000, 10, 10)

	ctx := context.Background()

	// Should succeed for burst requests
	for i := 0; i < 5; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Errorf("Expected wait to succeed, got: %v", err)
		}
	}
}

// ==================== Gateway Monitor Tests ====================

func TestGatewayMonitor_RecordRequest(t *testing.T) {
	monitor := NewGatewayMonitor()

	monitor.RecordRequest(BackendOllama, "chat", 100*time.Millisecond)
	monitor.RecordRequest(BackendOllama, "chat", 200*time.Millisecond)
	monitor.RecordSuccess(BackendOllama, "chat", 100, 150*time.Millisecond)
	monitor.RecordError(BackendOllama, "chat", nil)

	metrics := monitor.GetMetrics()

	if metrics.TotalRequests != 2 {
		t.Errorf("Expected 2 requests, got %d", metrics.TotalRequests)
	}

	if metrics.TotalTokens != 100 {
		t.Errorf("Expected 100 tokens, got %d", metrics.TotalTokens)
	}

	if metrics.TotalErrors != 1 {
		t.Errorf("Expected 1 error, got %d", metrics.TotalErrors)
	}
}

// ==================== SSE Reader Tests ====================

func TestSSEReaderParsing(t *testing.T) {
	sseData := "event: message\ndata: hello\n\nevent: done\ndata: [DONE]\n\n"
	reader := NewSSEReader(strings.NewReader(sseData))

	event, err := reader.ReadEvent()
	if err != nil {
		t.Fatalf("Failed to read event: %v", err)
	}

	if event.Event != "message" {
		t.Errorf("Expected event 'message', got '%s'", event.Event)
	}

	if event.Data != "hello" {
		t.Errorf("Expected data 'hello', got '%s'", event.Data)
	}
}

// ==================== Config Tests ====================

func TestDefaultAIConfig(t *testing.T) {
	cfg := config.DefaultAIConfig()

	if cfg.Gateway.Listen != ":11435" {
		t.Errorf("Expected default listen ':11435', got '%s'", cfg.Gateway.Listen)
	}

	if !cfg.Gateway.EnableOpenAICompat {
		t.Error("Expected OpenAI compat to be enabled by default")
	}

	if !cfg.Backends.Ollama.Enabled {
		t.Error("Expected Ollama to be enabled by default")
	}

	if cfg.Resources.MaxConcurrent != 10 {
		t.Errorf("Expected MaxConcurrent 10, got %d", cfg.Resources.MaxConcurrent)
	}
}

// ==================== Helper Function Tests ====================

func TestTrimModelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"llama2", "llama2"},
		{"llama2:7b", "llama2"},
		{"registry/namespace/model:tag", "model"},
		{"model:latest", "model"},
	}

	for _, tt := range tests {
		result := TrimModelName(tt.input)
		if result != tt.expected {
			t.Errorf("TrimModelName(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		bytesSec float64
		expected string
	}{
		{500, "500 B/s"},
		{2048, "2.0 KB/s"},
		{2048 * 1024, "2.0 MB/s"},
	}

	for _, tt := range tests {
		result := formatSpeed(tt.bytesSec)
		if !strings.Contains(result, strings.Split(tt.expected, " ")[1]) {
			t.Errorf("formatSpeed(%v) = %s, expected to contain unit from %s", tt.bytesSec, result, tt.expected)
		}
	}
}

// ==================== Integration Tests ====================

func TestService_Initialization(t *testing.T) {
	cfg := config.DefaultAIConfig()
	cfg.Backends.Ollama.Enabled = false // Disable actual connection

	svc, err := NewAIService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	if !svc.IsInitialized() {
		t.Error("Expected service to be initialized")
	}

	health := svc.HealthCheck(context.Background())
	if health.Status == "" {
		t.Error("Expected health status")
	}
}

func TestService_GetGateway(t *testing.T) {
	cfg := config.DefaultAIConfig()
	svc, _ := NewAIService(cfg)

	gateway := svc.GetGateway()
	if gateway == nil {
		t.Error("Expected gateway to be returned")
	}
}

func TestService_GetModelManager(t *testing.T) {
	cfg := config.DefaultAIConfig()
	cfg.ModelManager.StoragePath = t.TempDir() // Use temp dir for test
	svc, _ := NewAIService(cfg)

	mgr := svc.GetModelManager()
	if mgr == nil {
		t.Error("Expected model manager to be returned")
	}
}

// ==================== Benchmark Tests ====================

func BenchmarkOllamaBackend_Chat(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ollamaChatResponse{
			Model:   "llama2",
			Message: ChatMessage{Role: "assistant", Content: "Hello!"},
			Done:    true,
		})
	}))
	defer server.Close()

	backend := NewOllamaBackend(
		BackendConfig{
			Type:     BackendOllama,
			Endpoint: server.URL,
		},
		"/tmp/models",
		0,
		"5m",
	)

	ctx := context.Background()
	req := &ChatRequest{
		Model:    "llama2",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Chat(ctx, req)
	}
}

func BenchmarkGatewayMonitor_RecordRequest(b *testing.B) {
	monitor := NewGatewayMonitor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.RecordRequest(BackendOllama, "chat", 100*time.Millisecond)
	}
}

func BenchmarkRateLimiter_Wait(b *testing.B) {
	rl := NewRateLimiter(10000, 1000000, 100, 50)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Wait(ctx)
	}
}
