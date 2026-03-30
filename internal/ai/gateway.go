// Package ai provides AI service integration for NAS-OS
// gateway.go - Unified API Gateway for AI inference
package ai

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"nas-os/pkg/config"
)

// Gateway provides unified AI inference interface
type Gateway struct {
	config      *config.GatewayConfig
	backends    map[BackendType]Backend
	defaultBkd  BackendType
	modelRouter *ModelRouter
	limiter     *RateLimiter
	monitor     *GatewayMonitor
	healthy     atomic.Bool

	mu sync.RWMutex
}

// NewGateway creates a new AI gateway
func NewGateway(cfg *config.GatewayConfig) *Gateway {
	g := &Gateway{
		config:   cfg,
		backends: make(map[BackendType]Backend),
		monitor:  NewGatewayMonitor(),
	}

	if cfg.DefaultBackend != "" {
		g.defaultBkd = BackendType(cfg.DefaultBackend)
	} else {
		g.defaultBkd = BackendOllama
	}

	if cfg.RateLimit.Enabled {
		g.limiter = NewRateLimiter(
			cfg.RateLimit.RequestsPerMin,
			cfg.RateLimit.TokensPerMin,
			cfg.RateLimit.BurstSize,
			cfg.RateLimit.ConcurrencyLimit,
		)
	}

	g.modelRouter = NewModelRouter()
	g.healthy.Store(true)

	return g
}

// RegisterBackend registers a backend with the gateway
func (g *Gateway) RegisterBackend(backendType BackendType, backend Backend) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.backends[backendType]; exists {
		return fmt.Errorf("backend %s already registered", backendType)
	}

	g.backends[backendType] = backend
	return nil
}

// Chat performs chat completion through the appropriate backend
func (g *Gateway) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	// Rate limiting
	if g.limiter != nil {
		if err := g.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit exceeded: %w", err)
		}
	}

	// Route to appropriate backend
	backend, err := g.routeRequest(req.Model)
	if err != nil {
		return nil, err
	}

	// Check health
	if !backend.IsHealthy(ctx) {
		// Try fallback backend
		backend, err = g.getFallbackBackend(ctx)
		if err != nil {
			return nil, fmt.Errorf("no healthy backend available")
		}
	}

	// Record request
	g.monitor.RecordRequest(backend.Name(), "chat", time.Since(start))

	// Forward request
	resp, err := backend.Chat(ctx, req)
	if err != nil {
		g.monitor.RecordError(backend.Name(), "chat", err)
		return nil, err
	}

	// Record success
	g.monitor.RecordSuccess(backend.Name(), "chat", resp.Usage.TotalTokens, time.Since(start))

	return resp, nil
}

// StreamChat performs streaming chat completion
func (g *Gateway) StreamChat(ctx context.Context, req *ChatRequest, callback func(chunk string) error) error {
	start := time.Now()

	// Rate limiting
	if g.limiter != nil {
		if err := g.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit exceeded: %w", err)
		}
	}

	backend, err := g.routeRequest(req.Model)
	if err != nil {
		return err
	}

	if !backend.IsHealthy(ctx) {
		backend, err = g.getFallbackBackend(ctx)
		if err != nil {
			return fmt.Errorf("no healthy backend available")
		}
	}

	g.monitor.RecordRequest(backend.Name(), "stream_chat", time.Since(start))

	err = backend.StreamChat(ctx, req, callback)
	if err != nil {
		g.monitor.RecordError(backend.Name(), "stream_chat", err)
		return err
	}

	g.monitor.RecordSuccess(backend.Name(), "stream_chat", 0, time.Since(start))
	return nil
}

// Embed generates embeddings
func (g *Gateway) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	start := time.Now()

	if g.limiter != nil {
		if err := g.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit exceeded: %w", err)
		}
	}

	backend, err := g.routeRequest(req.Model)
	if err != nil {
		return nil, err
	}

	if !backend.IsHealthy(ctx) {
		backend, err = g.getFallbackBackend(ctx)
		if err != nil {
			return nil, fmt.Errorf("no healthy backend available")
		}
	}

	g.monitor.RecordRequest(backend.Name(), "embed", time.Since(start))

	resp, err := backend.Embed(ctx, req)
	if err != nil {
		g.monitor.RecordError(backend.Name(), "embed", err)
		return nil, err
	}

	g.monitor.RecordSuccess(backend.Name(), "embed", resp.Usage.TotalTokens, time.Since(start))
	return resp, nil
}

// ListModels lists all available models across backends
func (g *Gateway) ListModels(ctx context.Context) ([]ModelInfo, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var allModels []ModelInfo
	for _, backend := range g.backends {
		models, err := backend.ListModels(ctx)
		if err != nil {
			continue
		}
		for i := range models {
			models[i].Details = map[string]any{
				"backend": string(backend.Name()),
			}
		}
		allModels = append(allModels, models...)
	}

	return allModels, nil
}

// LoadModel loads a model on the specified backend
func (g *Gateway) LoadModel(ctx context.Context, backendType BackendType, modelName string) error {
	g.mu.RLock()
	backend, exists := g.backends[backendType]
	g.mu.RUnlock()

	if !exists {
		return fmt.Errorf("backend %s not found", backendType)
	}

	return backend.LoadModel(ctx, modelName)
}

// UnloadModel unloads a model
func (g *Gateway) UnloadModel(ctx context.Context, backendType BackendType, modelName string) error {
	g.mu.RLock()
	backend, exists := g.backends[backendType]
	g.mu.RUnlock()

	if !exists {
		return fmt.Errorf("backend %s not found", backendType)
	}

	return backend.UnloadModel(ctx, modelName)
}

// GetBackendStatus returns status of all backends
func (g *Gateway) GetBackendStatus(ctx context.Context) map[BackendType]BackendStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()

	status := make(map[BackendType]BackendStatus)
	for name, backend := range g.backends {
		status[name] = BackendStatus{
			Name:    name,
			Healthy: backend.IsHealthy(ctx),
		}
	}

	return status
}

// GetMetrics returns gateway metrics
func (g *Gateway) GetMetrics() *GatewayMetrics {
	return g.monitor.GetMetrics()
}

// SetModelRouting sets routing rules for specific models
func (g *Gateway) SetModelRouting(model string, backend BackendType) {
	g.modelRouter.SetRoute(model, backend)
}

// IsHealthy returns gateway health status
func (g *Gateway) IsHealthy() bool {
	return g.healthy.Load()
}

// routeRequest routes a request to the appropriate backend
func (g *Gateway) routeRequest(model string) (Backend, error) {
	// Check model routing rules first
	if routedBackend := g.modelRouter.Route(model); routedBackend != "" {
		g.mu.RLock()
		backend, exists := g.backends[routedBackend]
		g.mu.RUnlock()

		if exists {
			return backend, nil
		}
	}

	// Use default backend
	g.mu.RLock()
	backend, exists := g.backends[g.defaultBkd]
	g.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("default backend %s not found", g.defaultBkd)
	}

	return backend, nil
}

// getFallbackBackend returns a healthy fallback backend
func (g *Gateway) getFallbackBackend(ctx context.Context) (Backend, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for name, backend := range g.backends {
		if name == g.defaultBkd {
			continue
		}
		if backend.IsHealthy(ctx) {
			return backend, nil
		}
	}

	return nil, fmt.Errorf("no healthy fallback backend available")
}

// BackendStatus represents backend status
type BackendStatus struct {
	Name    BackendType `json:"name"`
	Healthy bool        `json:"healthy"`
}

// GatewayMetrics represents gateway metrics
type GatewayMetrics struct {
	TotalRequests   int64                     `json:"totalRequests"`
	TotalErrors     int64                     `json:"totalErrors"`
	TotalTokens     int64                     `json:"totalTokens"`
	AvgLatencyMs    int64                     `json:"avgLatencyMs"`
	RequestsByType  map[string]int64          `json:"requestsByType"`
	ErrorsByBackend map[string]int64          `json:"errorsByBackend"`
	BackendMetrics  map[string]BackendMetrics `json:"backendMetrics"`
}

// BackendMetrics represents per-backend metrics
type BackendMetrics struct {
	Requests     int64 `json:"requests"`
	Errors       int64 `json:"errors"`
	Tokens       int64 `json:"tokens"`
	AvgLatencyMs int64 `json:"avgLatencyMs"`
}

// ModelRouter handles model-to-backend routing
type ModelRouter struct {
	routes map[string]BackendType
	mu     sync.RWMutex
}

// NewModelRouter creates a new model router
func NewModelRouter() *ModelRouter {
	return &ModelRouter{
		routes: make(map[string]BackendType),
	}
}

// SetRoute sets a model routing rule
func (r *ModelRouter) SetRoute(model string, backend BackendType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[model] = backend
}

// Route returns the backend for a model
func (r *ModelRouter) Route(model string) BackendType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.routes[model]
}

// RateLimiter implements rate limiting for the gateway
type RateLimiter struct {
	requestsPerMin   int
	tokensPerMin     int
	burstSize        int
	concurrencyLimit int

	requests    chan struct{}
	concurrency chan struct{}
	_           int64 // requestCount - reserved for future rate tracking
	_           int64 // tokenCount - reserved for future token tracking
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMin, tokensPerMin, burstSize, concurrencyLimit int) *RateLimiter {
	rl := &RateLimiter{
		requestsPerMin:   requestsPerMin,
		tokensPerMin:     tokensPerMin,
		burstSize:        burstSize,
		concurrencyLimit: concurrencyLimit,
		requests:         make(chan struct{}, burstSize),
		concurrency:      make(chan struct{}, concurrencyLimit),
	}

	// Pre-fill burst tokens
	for i := 0; i < burstSize; i++ {
		rl.requests <- struct{}{}
	}

	// Start replenishment
	go rl.replenish()

	return rl
}

// Wait waits for rate limiter permission
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.requests:
		select {
		case rl.concurrency <- struct{}{}:
			go func() {
				<-ctx.Done()
				<-rl.concurrency
			}()
			return nil
		default:
			// Return token
			rl.requests <- struct{}{}
			return fmt.Errorf("concurrency limit reached")
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (rl *RateLimiter) replenish() {
	ticker := time.NewTicker(time.Minute / time.Duration(rl.requestsPerMin))
	defer ticker.Stop()

	for range ticker.C {
		select {
		case rl.requests <- struct{}{}:
		default:
		}
	}
}

// GatewayMonitor monitors gateway activity
type GatewayMonitor struct {
	totalRequests   atomic.Int64
	totalErrors     atomic.Int64
	totalTokens     atomic.Int64
	totalLatency    atomic.Int64
	requestsByType  sync.Map
	errorsByBackend sync.Map
	backendMetrics  sync.Map
}

// NewGatewayMonitor creates a new monitor
func NewGatewayMonitor() *GatewayMonitor {
	return &GatewayMonitor{}
}

// RecordRequest records a request
func (m *GatewayMonitor) RecordRequest(backend BackendType, reqType string, latency time.Duration) {
	m.totalRequests.Add(1)
	m.totalLatency.Add(latency.Milliseconds())

	// Update by-type counter
	counter, _ := m.requestsByType.LoadOrStore(reqType, new(atomic.Int64))
	if c, ok := counter.(*atomic.Int64); ok {
		c.Add(1)
	}

	// Update backend metrics
	bmRaw, _ := m.backendMetrics.LoadOrStore(string(backend), &BackendMetrics{})
	if bm, ok := bmRaw.(*BackendMetrics); ok {
		bm.Requests++
		bm.AvgLatencyMs = (bm.AvgLatencyMs + latency.Milliseconds()) / 2
	}
}

// RecordSuccess records a successful request
func (m *GatewayMonitor) RecordSuccess(backend BackendType, reqType string, tokens int, latency time.Duration) {
	m.totalTokens.Add(int64(tokens))

	bmRaw, ok := m.backendMetrics.Load(string(backend))
	if ok {
		bm, ok := bmRaw.(*BackendMetrics)
		if ok {
			bm.Tokens += int64(tokens)
		}
	}
}

// RecordError records an error
func (m *GatewayMonitor) RecordError(backend BackendType, reqType string, err error) {
	m.totalErrors.Add(1)

	counter, _ := m.errorsByBackend.LoadOrStore(string(backend), new(atomic.Int64))
	if c, ok := counter.(*atomic.Int64); ok {
		c.Add(1)
	}
}

// GetMetrics returns current metrics
func (m *GatewayMonitor) GetMetrics() *GatewayMetrics {
	requestsByType := make(map[string]int64)
	m.requestsByType.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			if v, ok := value.(*atomic.Int64); ok {
				requestsByType[k] = v.Load()
			}
		}
		return true
	})

	errorsByBackend := make(map[string]int64)
	m.errorsByBackend.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			if v, ok := value.(*atomic.Int64); ok {
				errorsByBackend[k] = v.Load()
			}
		}
		return true
	})

	backendMetrics := make(map[string]BackendMetrics)
	m.backendMetrics.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			if v, ok := value.(*BackendMetrics); ok {
				backendMetrics[k] = *v
			}
		}
		return true
	})

	reqCount := m.totalRequests.Load()
	var avgLatency int64
	if reqCount > 0 {
		avgLatency = m.totalLatency.Load() / reqCount
	}

	return &GatewayMetrics{
		TotalRequests:   reqCount,
		TotalErrors:     m.totalErrors.Load(),
		TotalTokens:     m.totalTokens.Load(),
		AvgLatencyMs:    avgLatency,
		RequestsByType:  requestsByType,
		ErrorsByBackend: errorsByBackend,
		BackendMetrics:  backendMetrics,
	}
}
