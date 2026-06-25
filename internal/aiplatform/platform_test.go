package aiplatform

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

type mockProvider struct {
	name      string
	available bool
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	return &CompletionResponse{Content: "test", Model: req.Model}, nil
}
func (m *mockProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}
func (m *mockProvider) ListModels() []string { return []string{"model-1", "model-2"} }
func (m *mockProvider) IsAvailable() bool     { return m.available }

func TestNewAIPlatform(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := NewAIPlatform(logger)
	if p == nil {
		t.Fatal("expected non-nil platform")
	}
}

func TestAIPlatform_RegisterProvider(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := NewAIPlatform(logger)

	provider := &mockProvider{name: "test", available: true}
	p.RegisterProvider(provider)

	providers := p.ListProviders()
	if len(providers) != 1 || providers[0] != "test" {
		t.Errorf("expected [test], got %v", providers)
	}
}

func TestAIPlatform_RegisterModel(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := NewAIPlatform(logger)

	model := &Model{ID: "gpt-4", Name: "GPT-4", Provider: "openai"}
	p.RegisterModel(model)

	models := p.ListModels()
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}
}

func TestAIPlatform_Complete(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := NewAIPlatform(logger)

	provider := &mockProvider{name: "test", available: true}
	p.RegisterProvider(provider)
	p.RegisterModel(&Model{ID: "model-1", Provider: "test"})

	resp, err := p.Complete(context.Background(), &CompletionRequest{
		Model:    "model-1",
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "test" {
		t.Errorf("expected test, got %s", resp.Content)
	}
}

func TestAIPlatform_Embed(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := NewAIPlatform(logger)

	provider := &mockProvider{name: "test", available: true}
	p.RegisterProvider(provider)
	p.RegisterModel(&Model{ID: "embed-1", Provider: "test"})

	embedding, err := p.Embed(context.Background(), "test text", "embed-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(embedding) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(embedding))
	}
}

func TestAIPlatform_Complete_NoProvider(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := NewAIPlatform(logger)

	_, err := p.Complete(context.Background(), &CompletionRequest{
		Model: "nonexistent",
	})
	if err == nil {
		t.Error("expected error for no provider")
	}
}

func TestAIPlatform_GetProviderStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := NewAIPlatform(logger)

	p.RegisterProvider(&mockProvider{name: "test", available: true})
	stats := p.GetProviderStats()
	if _, ok := stats["test"]; !ok {
		t.Error("expected test provider in stats")
	}
}

func TestLoadBalancer_Next(t *testing.T) {
	lb := &LoadBalancer{
		providers: []Provider{
			&mockProvider{name: "p1", available: true},
			&mockProvider{name: "p2", available: true},
		},
		strategy: "round-robin",
	}

	p1, _ := lb.Next()
	p2, _ := lb.Next()
	if p1.Name() == p2.Name() {
		t.Error("expected different providers in round-robin")
	}
}

func TestLoadBalancer_Empty(t *testing.T) {
	lb := &LoadBalancer{providers: []Provider{}}
	_, err := lb.Next()
	if err == nil {
		t.Error("expected error for empty providers")
	}
}

func TestResponseCache(t *testing.T) {
	cache := &ResponseCache{
		cache: make(map[string]*CacheEntry),
	}

	resp := &CompletionResponse{Content: "cached"}
	cache.Set("key1", resp)

	got := cache.Get("key1")
	if got == nil || got.Content != "cached" {
		t.Error("expected cached response")
	}

	got = cache.Get("nonexistent")
	if got != nil {
		t.Error("expected nil for cache miss")
	}
}

func (c *ResponseCache) Set(key string, resp *CompletionResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = &CacheEntry{Response: resp}
}

func (c *ResponseCache) Get(key string) *CompletionResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.cache[key]
	if !ok {
		return nil
	}
	return entry.Response
}
