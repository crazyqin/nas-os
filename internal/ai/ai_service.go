// Package ai provides AI service integration for NAS-OS
// service.go - AI Service initialization and management
package ai

import (
	"context"
	"log"
	"net/http"
	"sync"

	"nas-os/pkg/config"
)

// AIService provides the complete AI service
type AIService struct {
	config       *config.AIConfig
	gateway      *Gateway
	modelManager *ModelManager
	resourceMon  *ResourceMonitor
	initialized  bool
	mu           sync.RWMutex
}

// NewAIService creates a new AI service
func NewAIService(cfg *config.AIConfig) (*AIService, error) {
	if cfg == nil {
		cfg = config.DefaultAIConfig()
	}

	svc := &AIService{
		config:      cfg,
		resourceMon: NewResourceMonitor(),
	}

	// Initialize gateway
	svc.gateway = NewGateway(&cfg.Gateway)

	// Initialize model manager
	mgr, err := NewModelManager(&cfg.ModelManager)
	if err != nil {
		log.Printf("⚠️ 模型管理器初始化警告: %v", err)
	} else {
		svc.modelManager = mgr
	}

	// Initialize backends
	if err := svc.initBackends(); err != nil {
		log.Printf("⚠️ 后端初始化警告: %v", err)
	}

	svc.initialized = true
	return svc, nil
}

// initBackends initializes configured backends
func (s *AIService) initBackends() error {
	// Initialize Ollama backend
	if s.config.Backends.Ollama.Enabled {
		backend := NewOllamaBackend(
			BackendConfig{
				Type:         BackendOllama,
				Endpoint:     s.config.Backends.Ollama.Endpoint,
				DefaultModel: s.config.Backends.Ollama.DefaultModel,
				Timeout:      120 * 1000 * 1000 * 1000, // 120s
			},
			s.config.Backends.Ollama.ModelPath,
			s.config.Backends.Ollama.GPULayers,
			s.config.Backends.Ollama.KeepAlive,
		)

		if err := s.gateway.RegisterBackend(BackendOllama, backend); err != nil {
			log.Printf("⚠️ Ollama后端注册失败: %v", err)
		} else {
			log.Println("✅ Ollama后端已注册")
		}
	}

	// Initialize LocalAI backend
	if s.config.Backends.LocalAI.Enabled {
		backend := NewLocalAIBackend(
			BackendConfig{
				Type:         BackendLocalAI,
				Endpoint:     s.config.Backends.LocalAI.Endpoint,
				DefaultModel: s.config.Backends.LocalAI.DefaultModel,
				Timeout:      120 * 1000 * 1000 * 1000,
			},
			s.config.Backends.LocalAI.Threads,
			s.config.Backends.LocalAI.ContextSize,
		)

		if err := s.gateway.RegisterBackend(BackendLocalAI, backend); err != nil {
			log.Printf("⚠️ LocalAI后端注册失败: %v", err)
		} else {
			log.Println("✅ LocalAI后端已注册")
		}
	}

	// Initialize vLLM backend
	if s.config.Backends.VLLM.Enabled {
		backend := NewVLLMBackend(
			BackendConfig{
				Type:         BackendVLLM,
				Endpoint:     s.config.Backends.VLLM.Endpoint,
				DefaultModel: s.config.Backends.VLLM.DefaultModel,
				Timeout:      300 * 1000 * 1000 * 1000, // 5min for vLLM
			},
			s.config.Backends.VLLM.TensorParallelSize,
			s.config.Backends.VLLM.GPUMemoryUtil,
		)

		if err := s.gateway.RegisterBackend(BackendVLLM, backend); err != nil {
			log.Printf("⚠️ vLLM后端注册失败: %v", err)
		} else {
			log.Println("✅ vLLM后端已注册")
		}
	}

	// Initialize custom backend
	if s.config.Backends.Custom.Enabled {
		backend := &CustomBackend{
			BaseBackend: NewBaseBackend(BackendConfig{
				Type:         BackendCustom,
				Endpoint:     s.config.Backends.Custom.Endpoint,
				APIKey:       s.config.Backends.Custom.APIKey,
				DefaultModel: s.config.Backends.Custom.DefaultModel,
				Timeout:      120 * 1000 * 1000 * 1000,
				Headers:      s.config.Backends.Custom.Headers,
			}),
			name: s.config.Backends.Custom.Name,
		}

		if err := s.gateway.RegisterBackend(BackendCustom, backend); err != nil {
			log.Printf("⚠️ 自定义后端注册失败: %v", err)
		} else {
			log.Println("✅ 自定义后端已注册:", s.config.Backends.Custom.Name)
		}
	}

	return nil
}

// GetGateway returns the AI gateway
func (s *AIService) GetGateway() *Gateway {
	return s.gateway
}

// GetModelManager returns the model manager
func (s *AIService) GetModelManager() *ModelManager {
	return s.modelManager
}

// GetResourceMonitor returns the resource monitor
func (s *AIService) GetResourceMonitor() *ResourceMonitor {
	return s.resourceMon
}

// IsInitialized returns whether the service is initialized
func (s *AIService) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized
}

// HealthCheck performs a health check
func (s *AIService) HealthCheck(ctx context.Context) *ServiceHealth {
	health := &ServiceHealth{
		Status:   "healthy",
		Backends: make(map[string]BackendHealth),
	}

	backendStatus := s.gateway.GetBackendStatus(ctx)
	for name, status := range backendStatus {
		health.Backends[string(name)] = BackendHealth{
			Healthy: status.Healthy,
		}
		if !status.Healthy {
			health.Status = "degraded"
		}
	}

	if len(health.Backends) == 0 {
		health.Status = "no_backends"
	}

	return health
}

// Shutdown shuts down the service
func (s *AIService) Shutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initialized = false
	return nil
}

// ServiceHealth represents service health
type ServiceHealth struct {
	Status   string                   `json:"status"`
	Backends map[string]BackendHealth `json:"backends"`
}

// BackendHealth represents backend health
type BackendHealth struct {
	Healthy bool `json:"healthy"`
}

// CustomBackend implements a custom OpenAI-compatible backend
type CustomBackend struct {
	*BaseBackend
	name string
}

// Name returns the backend name
func (b *CustomBackend) Name() BackendType {
	if b.name != "" {
		return BackendType(b.name)
	}
	return BackendCustom
}

// IsHealthy checks backend health
func (b *CustomBackend) IsHealthy(ctx context.Context) bool {
	resp, err := b.DoRequest(ctx, "GET", "/v1/models", nil)
	if err != nil {
		b.SetHealthy(false)
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	healthy := resp.StatusCode == http.StatusOK
	b.SetHealthy(healthy)
	return healthy
}

// Chat sends a chat request
func (b *CustomBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return openAICompatChat(ctx, b.BaseBackend, req)
}

// StreamChat streams chat completion
func (b *CustomBackend) StreamChat(ctx context.Context, req *ChatRequest, callback func(chunk string) error) error {
	return openAICompatStream(ctx, b.BaseBackend, req, callback)
}

// Embed generates embeddings
func (b *CustomBackend) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	return openAICompatEmbed(ctx, b.BaseBackend, req)
}

// ListModels lists available models
func (b *CustomBackend) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return openAICompatListModels(ctx, b.BaseBackend)
}

// LoadModel is a no-op for custom backends
func (b *CustomBackend) LoadModel(ctx context.Context, modelName string) error {
	return nil
}

// UnloadModel is a no-op for custom backends
func (b *CustomBackend) UnloadModel(ctx context.Context, modelName string) error {
	return nil
}

// GetModelInfo gets model information
func (b *CustomBackend) GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	return &ModelInfo{Name: modelName}, nil
}
