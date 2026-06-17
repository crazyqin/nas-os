// Package ailocalinfer provides local AI inference engine for NAS-OS
// types.go - Type definitions for local inference engine
package ailocalinfer

import (
	"context"
	"sync"
	"time"
)

// ModelType represents the type of AI model
type ModelType string

const (
	ModelTypeTextGeneration   ModelType = "text_generation"
	ModelTypeImageRecognition ModelType = "image_recognition"
	ModelTypeSpeechRecognition ModelType = "speech_recognition"
	ModelTypeEmbedding        ModelType = "embedding"
)

// ModelStatus represents the status of a model
type ModelStatus string

const (
	ModelStatusUnloaded ModelStatus = "unloaded"
	ModelStatusLoading  ModelStatus = "loading"
	ModelStatusReady    ModelStatus = "ready"
	ModelStatusError    ModelStatus = "error"
	ModelStatusUnloading ModelStatus = "unloading"
)

// Model represents a local AI model
type Model struct {
	Name          string            `json:"name"`
	Type          ModelType         `json:"type"`
	Path          string            `json:"path"`
	Size          int64             `json:"size"`
	Parameters    string            `json:"parameters"`
	Quantization  string            `json:"quantization"`
	Backend       string            `json:"backend"`
	Status        ModelStatus       `json:"status"`
	GPURequired   bool              `json:"gpu_required"`
	GPUMemoryMB   int               `json:"gpu_memory_mb"`
	MaxBatchSize  int               `json:"max_batch_size"`
	LoadedAt      *time.Time        `json:"loaded_at,omitempty"`
	LastError     string            `json:"last_error,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ModelInfo represents model information
type ModelInfo struct {
	Name         string            `json:"name"`
	Type         ModelType         `json:"type"`
	Size         int64             `json:"size"`
	Parameters   string            `json:"parameters"`
	Quantization string            `json:"quantization"`
	GPURequired  bool              `json:"gpu_required"`
	GPUMemoryMB  int               `json:"gpu_memory_mb"`
	Capabilities []string          `json:"capabilities"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// InferenceRequest represents a generic inference request
type InferenceRequest struct {
	ModelName string            `json:"model_name"`
	RequestID string            `json:"request_id"`
	Input     interface{}       `json:"input"`
	Options   InferenceOptions  `json:"options"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// InferenceOptions holds inference options
type InferenceOptions struct {
	MaxTokens     int     `json:"max_tokens,omitempty"`
	Temperature   float64 `json:"temperature,omitempty"`
	TopP          float64 `json:"top_p,omitempty"`
	TopK          int     `json:"top_k,omitempty"`
	StopSequences []string `json:"stop_sequences,omitempty"`
	Stream        bool    `json:"stream,omitempty"`
	BatchSize     int     `json:"batch_size,omitempty"`
}

// InferenceResponse represents a generic inference response
type InferenceResponse struct {
	RequestID  string            `json:"request_id"`
	ModelName  string            `json:"model_name"`
	Output     interface{}       `json:"output"`
	LatencyMS  int64             `json:"latency_ms"`
	TokensUsed *TokenUsage       `json:"tokens_used,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// TextGenerationInput represents text generation input
type TextGenerationInput struct {
	Prompt      string `json:"prompt"`
	System      string `json:"system,omitempty"`
	Context     []Message `json:"context,omitempty"`
}

// Message represents a conversation message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// TextGenerationOutput represents text generation output
type TextGenerationOutput struct {
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
}

// ImageRecognitionInput represents image recognition input
type ImageRecognitionInput struct {
	ImageData []byte `json:"image_data"`
	ImageURL  string `json:"image_url,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	Labels    []string `json:"labels,omitempty"`
}

// ImageRecognitionOutput represents image recognition output
type ImageRecognitionOutput struct {
	Description string              `json:"description"`
	Labels      []LabelPrediction   `json:"labels,omitempty"`
	Objects     []ObjectDetection   `json:"objects,omitempty"`
}

// LabelPrediction represents a label prediction
type LabelPrediction struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
}

// ObjectDetection represents an object detection result
type ObjectDetection struct {
	Name       string    `json:"name"`
	Confidence float64   `json:"confidence"`
	BBox       BoundingBox `json:"bbox"`
}

// BoundingBox represents a bounding box
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// SpeechRecognitionInput represents speech recognition input
type SpeechRecognitionInput struct {
	AudioData  []byte `json:"audio_data"`
	AudioURL   string `json:"audio_url,omitempty"`
	Language   string `json:"language,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

// SpeechRecognitionOutput represents speech recognition output
type SpeechRecognitionOutput struct {
	Text       string       `json:"text"`
	Language   string       `json:"language"`
	Confidence float64      `json:"confidence"`
	Segments   []Segment    `json:"segments,omitempty"`
}

// Segment represents a speech segment
type Segment struct {
	Text      string  `json:"text"`
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	Confidence float64 `json:"confidence"`
}

// BatchRequest represents a batch inference request
type BatchRequest struct {
	BatchID  string             `json:"batch_id"`
	Requests []InferenceRequest `json:"requests"`
	Options  BatchOptions       `json:"options"`
}

// BatchOptions holds batch options
type BatchOptions struct {
	MaxConcurrency int           `json:"max_concurrency,omitempty"`
	Timeout        time.Duration `json:"timeout,omitempty"`
	FailOnFirst    bool          `json:"fail_on_first,omitempty"`
}

// BatchResponse represents a batch inference response
type BatchResponse struct {
	BatchID   string              `json:"batch_id"`
	Responses []InferenceResponse `json:"responses"`
	Errors    []BatchError        `json:"errors,omitempty"`
	TotalMS   int64               `json:"total_ms"`
}

// BatchError represents an error in a batch request
type BatchError struct {
	Index     int    `json:"index"`
	RequestID string `json:"request_id"`
	Error     string `json:"error"`
}

// GPUInfo represents GPU information
type GPUInfo struct {
	Available    bool   `json:"available"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	MemoryMB     int    `json:"memory_mb"`
	MemoryUsedMB int    `json:"memory_used_mb"`
	Driver       string `json:"driver"`
	CUDAVersion  string `json:"cuda_version,omitempty"`
}

// PerformanceMetrics represents performance metrics
type PerformanceMetrics struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	AvgLatencyMS       float64       `json:"avg_latency_ms"`
	P50LatencyMS       float64       `json:"p50_latency_ms"`
	P95LatencyMS       float64       `json:"p95_latency_ms"`
	P99LatencyMS       float64       `json:"p99_latency_ms"`
	ThroughputRPS      float64       `json:"throughput_rps"`
	GPUMemoryUsedMB    int           `json:"gpu_memory_used_mb"`
	CPUPercent         float64       `json:"cpu_percent"`
	StartTime          time.Time     `json:"start_time"`
	LastRequestTime    *time.Time    `json:"last_request_time,omitempty"`
	Uptime             time.Duration `json:"uptime"`
}

// InferenceEngineConfig holds configuration for the inference engine
type InferenceEngineConfig struct {
	ModelDir       string        `json:"model_dir"`
	MaxModels      int           `json:"max_models"`
	MaxBatchSize   int           `json:"max_batch_size"`
	GPUMemoryLimit int           `json:"gpu_memory_limit_mb"`
	CacheSize      int           `json:"cache_size"`
	WorkerCount    int           `json:"worker_count"`
	RequestTimeout time.Duration `json:"request_timeout"`
	EnableGPU      bool          `json:"enable_gpu"`
	EnableMetrics  bool          `json:"enable_metrics"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *InferenceEngineConfig {
	return &InferenceEngineConfig{
		ModelDir:       "/var/lib/nas-os/ai/models",
		MaxModels:      10,
		MaxBatchSize:   32,
		GPUMemoryLimit: 8192, // 8GB
		CacheSize:      5,
		WorkerCount:    4,
		RequestTimeout: 30 * time.Second,
		EnableGPU:      true,
		EnableMetrics:  true,
	}
}

// InferenceEngine is the main interface for local inference
type InferenceEngine interface {
	// LoadModel loads a model into memory
	LoadModel(ctx context.Context, modelName string) error

	// UnloadModel unloads a model from memory
	UnloadModel(ctx context.Context, modelName string) error

	// ListModels lists all available models
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// GetModelStatus gets the status of a model
	GetModelStatus(ctx context.Context, modelName string) (*Model, error)

	// Inference runs inference on a single request
	Inference(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error)

	// BatchInference runs inference on a batch of requests
	BatchInference(ctx context.Context, req *BatchRequest) (*BatchResponse, error)

	// GetGPUInfo gets GPU information
	GetGPUInfo(ctx context.Context) (*GPUInfo, error)

	// GetMetrics gets performance metrics
	GetMetrics(ctx context.Context) (*PerformanceMetrics, error)

	// Close closes the engine and releases resources
	Close() error
}

// LRUCache implements a LRU cache for models
type LRUCache struct {
	capacity int
	items    map[string]*CacheItem
	order    *DoublyLinkedList
	mu       sync.RWMutex
}

// CacheItem represents a cached item
type CacheItem struct {
	Key       string
	Value     interface{}
	Size      int64
	AccessAt  time.Time
	Frequency int
	Element   *ListNode
}

// DoublyLinkedList represents a doubly linked list
type DoublyLinkedList struct {
	head *ListNode
	tail *ListNode
	size int
}

// ListNode represents a node in the doubly linked list
type ListNode struct {
	key  string
	prev *ListNode
	next *ListNode
}

// ModelCache manages model caching
type ModelCache struct {
	cache      *LRUCache
	totalSize  int64
	maxSize    int64
	mu         sync.RWMutex
}

// MetricsCollector collects and reports metrics
type MetricsCollector struct {
	requests      []LatencyRecord
	totalRequests int64
	successCount  int64
	failCount     int64
	startTime     time.Time
	lastRequest   *time.Time
	mu            sync.RWMutex
}

// LatencyRecord records latency for a request
type LatencyRecord struct {
	LatencyMS int64
	Timestamp time.Time
}
