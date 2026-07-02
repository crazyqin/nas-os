package aiplatform

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AIPlatform 统一AI推理平台.
type AIPlatform struct {
	logger       *zap.Logger
	providers    map[string]Provider
	models       map[string]*Model
	registry     *ModelRegistry
	loadBalancer *LoadBalancer
	cache        *ResponseCache
	mu           sync.RWMutex
}

// Provider AI推理提供商接口.
type Provider interface {
	Name() string
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	Embed(ctx context.Context, text string) ([]float32, error)
	ListModels() []string
	IsAvailable() bool
}

// Model 模型信息.
type Model struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Provider      string            `json:"provider"`
	Type          string            `json:"type"` // llm, embedding, vision, multimodal
	MaxTokens     int               `json:"max_tokens"`
	ContextWindow int               `json:"context_window"`
	PricePer1k    float64           `json:"price_per_1k"`
	Capabilities  []string          `json:"capabilities"`
	Props         map[string]string `json:"props"`
}

// CompletionRequest 补全请求.
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float32   `json:"temperature"`
	TopP        float32   `json:"top_p"`
	Stream      bool      `json:"stream"`
	Stop        []string  `json:"stop"`
}

// Message 消息.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResponse 补全响应.
type CompletionResponse struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Model   string `json:"model"`
	Usage   Usage  `json:"usage"`
	Latency int64  `json:"latency_ms"`
}

// Usage 使用量.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ModelRegistry 模型注册表.
type ModelRegistry struct {
	models map[string]*Model
	mu     sync.RWMutex
}

// LoadBalancer 负载均衡器.
type LoadBalancer struct {
	providers []Provider
	strategy  string // round-robin, least-latency, random
	index     int
	mu        sync.Mutex
}

// ResponseCache 响应缓存.
type ResponseCache struct {
	cache map[string]*CacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
}

// CacheEntry 缓存条目.
type CacheEntry struct {
	Response  *CompletionResponse
	CreatedAt time.Time
}

// NewAIPlatform 创建AI平台.
func NewAIPlatform(logger *zap.Logger) *AIPlatform {
	return &AIPlatform{
		logger:    logger,
		providers: make(map[string]Provider),
		models:    make(map[string]*Model),
		registry: &ModelRegistry{
			models: make(map[string]*Model),
		},
		loadBalancer: &LoadBalancer{
			strategy: "round-robin",
		},
		cache: &ResponseCache{
			cache: make(map[string]*CacheEntry),
			ttl:   5 * time.Minute,
		},
	}
}

// RegisterProvider 注册推理提供商.
func (ap *AIPlatform) RegisterProvider(provider Provider) {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	ap.providers[provider.Name()] = provider
	ap.loadBalancer.providers = append(ap.loadBalancer.providers, provider)

	// 注册提供商的模型
	for _, modelID := range provider.ListModels() {
		ap.registry.models[modelID] = &Model{
			ID:       modelID,
			Provider: provider.Name(),
		}
	}

	ap.logger.Info("Registered AI provider",
		zap.String("provider", provider.Name()),
		zap.Int("models", len(provider.ListModels())))
}

// RegisterModel 注册模型.
func (ap *AIPlatform) RegisterModel(model *Model) {
	ap.registry.mu.Lock()
	defer ap.registry.mu.Unlock()
	ap.registry.models[model.ID] = model
	ap.models[model.ID] = model
}

// Complete 执行补全.
func (ap *AIPlatform) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	// 检查缓存
	cacheKey := ap.cacheKey(req)
	if cached := ap.cacheGet(cacheKey); cached != nil {
		ap.logger.Debug("Cache hit", zap.String("model", req.Model))
		return cached, nil
	}

	// 选择提供商
	provider, err := ap.selectProvider(req.Model)
	if err != nil {
		return nil, err
	}

	// 执行补全
	resp, err := provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("completion failed: %w", err)
	}

	resp.Latency = time.Since(start).Milliseconds()

	// 缓存响应
	if !req.Stream {
		ap.cacheSet(cacheKey, resp)
	}

	ap.logger.Info("Completion done",
		zap.String("model", req.Model),
		zap.Int64("latency_ms", resp.Latency),
		zap.Int("tokens", resp.Usage.TotalTokens))

	return resp, nil
}

// Stream 流式补全.
func (ap *AIPlatform) Stream(ctx context.Context, req *CompletionRequest) (<-chan *CompletionResponse, error) {
	req.Stream = true

	provider, err := ap.selectProvider(req.Model)
	if err != nil {
		return nil, err
	}

	ch := make(chan *CompletionResponse, 100)
	go func() {
		defer close(ch)
		resp, err := provider.Complete(ctx, req)
		if err != nil {
			return
		}
		ch <- resp
	}()

	return ch, nil
}

// Embed 向量化.
func (ap *AIPlatform) Embed(ctx context.Context, text string, model string) ([]float32, error) {
	provider, err := ap.selectProvider(model)
	if err != nil {
		return nil, err
	}
	return provider.Embed(ctx, text)
}

// selectProvider 选择提供商.
func (ap *AIPlatform) selectProvider(modelID string) (Provider, error) {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	// 优先使用指定模型的提供商
	if model, ok := ap.registry.models[modelID]; ok {
		if provider, ok := ap.providers[model.Provider]; ok {
			if provider.IsAvailable() {
				return provider, nil
			}
		}
	}

	// 使用负载均衡
	return ap.loadBalancer.Next()
}

// Next 负载均衡选择下一个提供商.
func (lb *LoadBalancer) Next() (Provider, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	switch lb.strategy {
	case "round-robin":
		provider := lb.providers[lb.index%len(lb.providers)]
		lb.index++
		return provider, nil
	default:
		return lb.providers[0], nil
	}
}

func (ap *AIPlatform) cacheKey(req *CompletionRequest) string {
	return fmt.Sprintf("%s:%v", req.Model, req.Messages)
}

func (ap *AIPlatform) cacheGet(key string) *CompletionResponse {
	ap.cache.mu.RLock()
	defer ap.cache.mu.RUnlock()

	entry, ok := ap.cache.cache[key]
	if !ok {
		return nil
	}
	if time.Since(entry.CreatedAt) > ap.cache.ttl {
		delete(ap.cache.cache, key)
		return nil
	}
	return entry.Response
}

func (ap *AIPlatform) cacheSet(key string, resp *CompletionResponse) {
	ap.cache.mu.Lock()
	defer ap.cache.mu.Unlock()

	ap.cache.cache[key] = &CacheEntry{
		Response:  resp,
		CreatedAt: time.Now(),
	}
}

// ListProviders 列出提供商.
func (ap *AIPlatform) ListProviders() []string {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	var names []string
	for name := range ap.providers {
		names = append(names, name)
	}
	return names
}

// ListModels 列出模型.
func (ap *AIPlatform) ListModels() []*Model {
	ap.registry.mu.RLock()
	defer ap.registry.mu.RUnlock()

	var models []*Model
	for _, m := range ap.registry.models {
		models = append(models, m)
	}
	return models
}

// GetProviderStats 获取提供商统计.
func (ap *AIPlatform) GetProviderStats() map[string]interface{} {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	stats := make(map[string]interface{})
	for name, provider := range ap.providers {
		stats[name] = map[string]interface{}{
			"available": provider.IsAvailable(),
			"models":    len(provider.ListModels()),
		}
	}
	return stats
}
