// Package config provides configuration structures for NAS-OS
package config

// AIConfig holds the complete AI service configuration
type AIConfig struct {
	// Gateway configuration
	Gateway GatewayConfig `json:"gateway" yaml:"gateway"`

	// Backend configurations
	Backends BackendsConfig `json:"backends" yaml:"backends"`

	// Model management
	ModelManager ModelManagerConfig `json:"modelManager" yaml:"modelManager"`

	// Resource limits
	Resources ResourceConfig `json:"resources" yaml:"resources"`

	// Security settings
	Security AISecurityConfig `json:"security" yaml:"security"`
}

// GatewayConfig defines the API gateway configuration
type GatewayConfig struct {
	// Listen address (e.g., ":8080")
	Listen string `json:"listen" yaml:"listen"`

	// Enable OpenAI-compatible API
	EnableOpenAICompat bool `json:"enableOpenAICompat" yaml:"enableOpenAICompat"`

	// API key for gateway authentication (optional, for external access)
	APIKey string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`

	// Rate limiting
	RateLimit RateLimitConfig `json:"rateLimit" yaml:"rateLimit"`

	// CORS settings
	CORS CORSConfig `json:"cors" yaml:"cors"`

	// Default backend to use
	DefaultBackend string `json:"defaultBackend" yaml:"defaultBackend"`

	// Request timeout in seconds
	RequestTimeout int `json:"requestTimeout" yaml:"requestTimeout"`
}

// RateLimitConfig defines rate limiting settings
type RateLimitConfig struct {
	Enabled          bool `json:"enabled" yaml:"enabled"`
	RequestsPerMin   int  `json:"requestsPerMin" yaml:"requestsPerMin"`
	TokensPerMin     int  `json:"tokensPerMin" yaml:"tokensPerMin"`
	BurstSize        int  `json:"burstSize" yaml:"burstSize"`
	ConcurrencyLimit int  `json:"concurrencyLimit" yaml:"concurrencyLimit"`
}

// CORSConfig defines CORS settings
type CORSConfig struct {
	Enabled        bool     `json:"enabled" yaml:"enabled"`
	AllowedOrigins []string `json:"allowedOrigins" yaml:"allowedOrigins"`
	AllowedMethods []string `json:"allowedMethods" yaml:"allowedMethods"`
	AllowedHeaders []string `json:"allowedHeaders" yaml:"allowedHeaders"`
}

// BackendsConfig holds all backend configurations
type BackendsConfig struct {
	Ollama  OllamaConfig  `json:"ollama" yaml:"ollama"`
	LocalAI LocalAIConfig `json:"localAI" yaml:"localAI"`
	VLLM    VLLMConfig    `json:"vllm" yaml:"vllm"`
	Custom  CustomConfig  `json:"custom" yaml:"custom"`
}

// OllamaConfig defines Ollama backend configuration
type OllamaConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	// Default model to use
	DefaultModel string `json:"defaultModel" yaml:"defaultModel"`
	// GPU layers (0 for CPU only)
	GPULayers int `json:"gpuLayers" yaml:"gpuLayers"`
	// Context window size
	ContextSize int `json:"contextSize" yaml:"contextSize"`
	// Model storage path
	ModelPath string `json:"modelPath" yaml:"modelPath"`
	// Keep models loaded
	KeepAlive string `json:"keepAlive" yaml:"keepAlive"`
}

// LocalAIConfig defines LocalAI backend configuration
type LocalAIConfig struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	Endpoint     string `json:"endpoint" yaml:"endpoint"`
	DefaultModel string `json:"defaultModel" yaml:"defaultModel"`
	// Threads for inference
	Threads int `json:"threads" yaml:"threads"`
	// Context size
	ContextSize int `json:"contextSize" yaml:"contextSize"`
	// Model path
	ModelPath string `json:"modelPath" yaml:"modelPath"`
	// GPU support
	GPU bool `json:"gpu" yaml:"gpu"`
}

// VLLMConfig defines vLLM backend configuration
type VLLMConfig struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	Endpoint     string `json:"endpoint" yaml:"endpoint"`
	DefaultModel string `json:"defaultModel" yaml:"defaultModel"`
	// Tensor parallel size
	TensorParallelSize int `json:"tensorParallelSize" yaml:"tensorParallelSize"`
	// GPU memory utilization (0.0-1.0)
	GPUMemoryUtil float64 `json:"gpuMemoryUtil" yaml:"gpuMemoryUtil"`
	// Max model length
	MaxModelLen int `json:"maxModelLen" yaml:"maxModelLen"`
	// Model path
	ModelPath string `json:"modelPath" yaml:"modelPath"`
}

// CustomConfig defines custom OpenAI-compatible backend
type CustomConfig struct {
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	Name         string            `json:"name" yaml:"name"`
	Endpoint     string            `json:"endpoint" yaml:"endpoint"`
	APIKey       string            `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	DefaultModel string            `json:"defaultModel" yaml:"defaultModel"`
	Headers      map[string]string `json:"headers" yaml:"headers"`
}

// ModelManagerConfig defines model management configuration
type ModelManagerConfig struct {
	// Storage path for models
	StoragePath string `json:"storagePath" yaml:"storagePath"`

	// HuggingFace token for gated models
	HuggingFaceToken string `json:"huggingFaceToken,omitempty" yaml:"huggingFaceToken,omitempty"`

	// Model registry
	Registry RegistryConfig `json:"registry" yaml:"registry"`

	// Auto-download on request
	AutoDownload bool `json:"autoDownload" yaml:"autoDownload"`

	// Cache settings
	Cache CacheConfig `json:"cache" yaml:"cache"`
}

// RegistryConfig defines model registry settings
type RegistryConfig struct {
	// Supported registries
	HuggingFace bool `json:"huggingFace" yaml:"huggingFace"`
	Ollama      bool `json:"ollama" yaml:"ollama"`
	Local       bool `json:"local" yaml:"local"`
}

// CacheConfig defines cache settings
type CacheConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Path       string `json:"path" yaml:"path"`
	MaxSizeGB  int    `json:"maxSizeGB" yaml:"maxSizeGB"`
	ExpiryDays int    `json:"expiryDays" yaml:"expiryDays"`
}

// ResourceConfig defines resource limits
type ResourceConfig struct {
	// Max GPU memory to use (percentage, 0-100)
	MaxGPUMemory int `json:"maxGpuMemory" yaml:"maxGpuMemory"`

	// Max system memory to use (percentage, 0-100)
	MaxSystemMemory int `json:"maxSystemMemory" yaml:"maxSystemMemory"`

	// Max concurrent requests
	MaxConcurrent int `json:"maxConcurrent" yaml:"maxConcurrent"`

	// Max queue size
	MaxQueueSize int `json:"maxQueueSize" yaml:"maxQueueSize"`

	// Enable GPU scheduling
	GPUScheduling bool `json:"gpuScheduling" yaml:"gpuScheduling"`
}

// AISecurityConfig defines security settings
type AISecurityConfig struct {
	// Enable PII de-identification
	EnableDeID bool `json:"enableDeId" yaml:"enableDeId"`

	// Audit logging
	AuditLogging bool `json:"auditLogging" yaml:"auditLogging"`

	// Content filtering
	ContentFiltering bool `json:"contentFiltering" yaml:"contentFiltering"`

	// Allowed models (empty = all)
	AllowedModels []string `json:"allowedModels" yaml:"allowedModels"`

	// Blocked models
	BlockedModels []string `json:"blockedModels" yaml:"blockedModels"`
}

// DefaultAIConfig returns default AI configuration
func DefaultAIConfig() *AIConfig {
	return &AIConfig{
		Gateway: GatewayConfig{
			Listen:             ":11435",
			EnableOpenAICompat: true,
			RequestTimeout:     300,
			DefaultBackend:     "ollama",
			RateLimit: RateLimitConfig{
				Enabled:          true,
				RequestsPerMin:   60,
				TokensPerMin:     100000,
				BurstSize:        10,
				ConcurrencyLimit: 5,
			},
			CORS: CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders: []string{"Authorization", "Content-Type"},
			},
		},
		Backends: BackendsConfig{
			Ollama: OllamaConfig{
				Enabled:      true,
				Endpoint:     "http://localhost:11434",
				DefaultModel: "llama2",
				GPULayers:    0,
				ContextSize:  4096,
				ModelPath:    "/var/lib/nas-os/ai/models/ollama",
				KeepAlive:    "5m",
			},
			LocalAI: LocalAIConfig{
				Enabled:      false,
				Endpoint:     "http://localhost:8080",
				DefaultModel: "llama2",
				Threads:      4,
				ContextSize:  4096,
				ModelPath:    "/var/lib/nas-os/ai/models/localai",
			},
			VLLM: VLLMConfig{
				Enabled:            false,
				Endpoint:           "http://localhost:8000",
				DefaultModel:       "facebook/opt-125m",
				TensorParallelSize: 1,
				GPUMemoryUtil:      0.9,
				MaxModelLen:        2048,
				ModelPath:          "/var/lib/nas-os/ai/models/vllm",
			},
		},
		ModelManager: ModelManagerConfig{
			StoragePath:   "/var/lib/nas-os/ai/models",
			AutoDownload:  false,
			HuggingFaceToken: "",
			Registry: RegistryConfig{
				HuggingFace: true,
				Ollama:      true,
				Local:       true,
			},
			Cache: CacheConfig{
				Enabled:    true,
				Path:       "/var/cache/nas-os/ai",
				MaxSizeGB:  50,
				ExpiryDays: 30,
			},
		},
		Resources: ResourceConfig{
			MaxGPUMemory:   80,
			MaxSystemMemory: 50,
			MaxConcurrent:  10,
			MaxQueueSize:   100,
			GPUScheduling:  true,
		},
		Security: AISecurityConfig{
			EnableDeID:       true,
			AuditLogging:     true,
			ContentFiltering: false,
			AllowedModels:    []string{},
			BlockedModels:    []string{},
		},
	}
}