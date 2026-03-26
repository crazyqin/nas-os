// Package ai provides AI service integration for NAS-OS
// backend.go - Backend adapter interfaces and implementations
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// BackendType represents the type of LLM backend
type BackendType string

const (
	BackendOllama  BackendType = "ollama"
	BackendLocalAI BackendType = "localai"
	BackendVLLM    BackendType = "vllm"
	BackendCustom  BackendType = "custom"
)

// Backend defines the interface for LLM backends
type Backend interface {
	// Name returns the backend name
	Name() BackendType

	// IsHealthy checks if the backend is healthy
	IsHealthy(ctx context.Context) bool

	// Chat sends a chat completion request
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChat streams chat completion
	StreamChat(ctx context.Context, req *ChatRequest, callback func(chunk string) error) error

	// Embed generates embeddings
	Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error)

	// ListModels lists available models
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// LoadModel loads a model (for backends that support it)
	LoadModel(ctx context.Context, modelName string) error

	// UnloadModel unloads a model
	UnloadModel(ctx context.Context, modelName string) error

	// GetModelInfo gets model information
	GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error)
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID        string       `json:"id"`
	Object    string       `json:"object"`
	Created   int64        `json:"created"`
	Model     string       `json:"model"`
	Choices   []ChatChoice `json:"choices"`
	Usage     UsageInfo    `json:"usage"`
	LatencyMS int64        `json:"latency_ms,omitempty"`
}

// ChatChoice represents a choice in the response
type ChatChoice struct {
	Index        int          `json:"index"`
	Message      ChatMessage  `json:"message,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason string       `json:"finish_reason"`
}

// UsageInfo represents token usage information
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// EmbedRequest represents an embedding request
type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbedResponse represents an embedding response
type EmbedResponse struct {
	Object string      `json:"object"`
	Data   []EmbedData `json:"data"`
	Model  string      `json:"model"`
	Usage  UsageInfo   `json:"usage"`
}

// EmbedData represents embedding data
type EmbedData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// ModelInfo represents model information
type ModelInfo struct {
	Name         string            `json:"name"`
	ID           string            `json:"id"`
	Size         int64             `json:"size,omitempty"`
	ModifiedAt   time.Time         `json:"modified_at,omitempty"`
	Digest       string            `json:"digest,omitempty"`
	Details      map[string]any    `json:"details,omitempty"`
	Capabilities []ModelCapability `json:"capabilities,omitempty"`
	Parameters   string            `json:"parameters,omitempty"`
	Quantization string            `json:"quantization,omitempty"`
}

// ModelCapability represents model capabilities
type ModelCapability struct {
	Type string `json:"type"`
}

// BackendConfig holds backend configuration
type BackendConfig struct {
	Type            BackendType
	Endpoint        string
	APIKey          string
	DefaultModel    string
	Timeout         time.Duration
	Headers         map[string]string
	MaxRetries      int
	RetryDelay      time.Duration
	HealthCheckInt  time.Duration
}

// BaseBackend provides common functionality for backends
type BaseBackend struct {
	config     BackendConfig
	httpClient *http.Client
	healthy    bool
	mu         sync.RWMutex
}

// NewBaseBackend creates a new base backend
func NewBaseBackend(config BackendConfig) *BaseBackend {
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.HealthCheckInt == 0 {
		config.HealthCheckInt = 30 * time.Second
	}

	return &BaseBackend{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		healthy: true,
	}
}

// SetHealthy sets the backend health status
func (b *BaseBackend) SetHealthy(healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.healthy = healthy
}

// IsHealthy returns the backend health status
func (b *BaseBackend) IsHealthy() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.healthy
}

// DoRequest performs an HTTP request with retries
func (b *BaseBackend) DoRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	var lastErr error

	for i := 0; i < b.config.MaxRetries; i++ {
		url := strings.TrimSuffix(b.config.Endpoint, "/") + path
		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		if b.config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+b.config.APIKey)
		}
		for k, v := range b.config.Headers {
			req.Header.Set(k, v)
		}

		resp, err := b.httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(b.config.RetryDelay * time.Duration(i+1))
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", b.config.MaxRetries, lastErr)
}

// ==================== Ollama Backend ====================

// OllamaBackend implements Backend for Ollama
type OllamaBackend struct {
	*BaseBackend
	modelPath string
	gpuLayers int
	keepAlive string
}

// NewOllamaBackend creates a new Ollama backend
func NewOllamaBackend(config BackendConfig, modelPath string, gpuLayers int, keepAlive string) *OllamaBackend {
	return &OllamaBackend{
		BaseBackend: NewBaseBackend(config),
		modelPath:   modelPath,
		gpuLayers:   gpuLayers,
		keepAlive:   keepAlive,
	}
}

// Name returns the backend name
func (b *OllamaBackend) Name() BackendType {
	return BackendOllama
}

// IsHealthy checks if Ollama is healthy
func (b *OllamaBackend) IsHealthy(ctx context.Context) bool {
	resp, err := b.DoRequest(ctx, "GET", "/api/tags", nil)
	if err != nil {
		b.SetHealthy(false)
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	healthy := resp.StatusCode == http.StatusOK
	b.SetHealthy(healthy)
	return healthy
}

// Chat sends a chat request to Ollama
func (b *OllamaBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	ollamaReq := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false,
	}
	if req.MaxTokens > 0 {
		ollamaReq["options"] = map[string]any{
			"num_predict": req.MaxTokens,
		}
	}
	if req.Temperature > 0 {
		if opts, ok := ollamaReq["options"].(map[string]any); ok {
			opts["temperature"] = req.Temperature
		} else {
			ollamaReq["options"] = map[string]any{
				"temperature": req.Temperature,
			}
		}
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	resp, err := b.DoRequest(ctx, "POST", "/api/chat", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error: %s - %s", resp.Status, string(respBody))
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, err
	}

	return &ChatResponse{
		ID:      ollamaResp.CreatedAt,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: ollamaResp.Message.Content,
				},
				FinishReason: "stop",
			},
		},
		Usage: UsageInfo{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
		LatencyMS: time.Since(start).Milliseconds(),
	}, nil
}

// StreamChat streams chat completion from Ollama
func (b *OllamaBackend) StreamChat(ctx context.Context, req *ChatRequest, callback func(chunk string) error) error {
	ollamaReq := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return err
	}

	resp, err := b.DoRequest(ctx, "POST", "/api/chat", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk ollamaStreamResponse
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if chunk.Message.Content != "" {
			if err := callback(chunk.Message.Content); err != nil {
				return err
			}
		}

		if chunk.Done {
			break
		}
	}

	return nil
}

// Embed generates embeddings
func (b *OllamaBackend) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	ollamaReq := map[string]any{
		"model":  req.Model,
		"prompt": req.Input,
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	resp, err := b.DoRequest(ctx, "POST", "/api/embeddings", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var ollamaResp ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, err
	}

	return &EmbedResponse{
		Object: "list",
		Data: []EmbedData{
			{
				Object:    "embedding",
				Index:     0,
				Embedding: ollamaResp.Embedding,
			},
		},
		Model: req.Model,
	}, nil
}

// ListModels lists available models
func (b *OllamaBackend) ListModels(ctx context.Context) ([]ModelInfo, error) {
	resp, err := b.DoRequest(ctx, "GET", "/api/tags", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var listResp ollamaListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, len(listResp.Models))
	for i, m := range listResp.Models {
		models[i] = ModelInfo{
			Name:       m.Name,
			ID:         m.Digest,
			Size:       m.Size,
			ModifiedAt: m.ModifiedAt,
			Digest:     m.Digest,
			Details:    m.Details,
		}
	}

	return models, nil
}

// LoadModel loads a model
func (b *OllamaBackend) LoadModel(ctx context.Context, modelName string) error {
	req := map[string]any{
		"model":     modelName,
		"keep_alive": b.keepAlive,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := b.DoRequest(ctx, "POST", "/api/generate", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	return nil
}

// UnloadModel unloads a model
func (b *OllamaBackend) UnloadModel(ctx context.Context, modelName string) error {
	req := map[string]any{
		"model":      modelName,
		"keep_alive": 0,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := b.DoRequest(ctx, "POST", "/api/generate", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	return nil
}

// GetModelInfo gets model information
func (b *OllamaBackend) GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	req := map[string]any{
		"model": modelName,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := b.DoRequest(ctx, "POST", "/api/show", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var showResp ollamaShowResponse
	if err := json.NewDecoder(resp.Body).Decode(&showResp); err != nil {
		return nil, err
	}

	return &ModelInfo{
		Name:       modelName,
		Details:    showResp.Details,
		Parameters: showResp.Parameters,
	}, nil
}

// Ollama response types
type ollamaChatResponse struct {
	Model          string       `json:"model"`
	CreatedAt      string       `json:"created_at"`
	Message        ChatMessage  `json:"message"`
	Done           bool         `json:"done"`
	PromptEvalCount int         `json:"prompt_eval_count"`
	EvalCount      int          `json:"eval_count"`
}

type ollamaStreamResponse struct {
	Model     string      `json:"model"`
	CreatedAt string      `json:"created_at"`
	Message   ChatMessage `json:"message"`
	Done      bool        `json:"done"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

type ollamaListResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name       string         `json:"name"`
	Size       int64          `json:"size"`
	Digest     string         `json:"digest"`
	ModifiedAt time.Time      `json:"modified_at"`
	Details    map[string]any `json:"details"`
}

type ollamaShowResponse struct {
	License    string         `json:"license"`
	Modelfile  string         `json:"modelfile"`
	Parameters string         `json:"parameters"`
	Template   string         `json:"template"`
	Details    map[string]any `json:"details"`
}

// ==================== LocalAI Backend ====================

// LocalAIBackend implements Backend for LocalAI (OpenAI-compatible)
type LocalAIBackend struct {
	*BaseBackend
	threads     int
	contextSize int
}

// NewLocalAIBackend creates a new LocalAI backend
func NewLocalAIBackend(config BackendConfig, threads, contextSize int) *LocalAIBackend {
	return &LocalAIBackend{
		BaseBackend: NewBaseBackend(config),
		threads:     threads,
		contextSize: contextSize,
	}
}

// Name returns the backend name
func (b *LocalAIBackend) Name() BackendType {
	return BackendLocalAI
}

// IsHealthy checks if LocalAI is healthy
func (b *LocalAIBackend) IsHealthy(ctx context.Context) bool {
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
func (b *LocalAIBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return openAICompatChat(ctx, b.BaseBackend, req)
}

// StreamChat streams chat completion
func (b *LocalAIBackend) StreamChat(ctx context.Context, req *ChatRequest, callback func(chunk string) error) error {
	return openAICompatStream(ctx, b.BaseBackend, req, callback)
}

// Embed generates embeddings
func (b *LocalAIBackend) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	return openAICompatEmbed(ctx, b.BaseBackend, req)
}

// ListModels lists available models
func (b *LocalAIBackend) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return openAICompatListModels(ctx, b.BaseBackend)
}

// LoadModel is a no-op for LocalAI (models are loaded on demand)
func (b *LocalAIBackend) LoadModel(ctx context.Context, modelName string) error {
	return nil
}

// UnloadModel is a no-op for LocalAI
func (b *LocalAIBackend) UnloadModel(ctx context.Context, modelName string) error {
	return nil
}

// GetModelInfo gets model information
func (b *LocalAIBackend) GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	return &ModelInfo{Name: modelName}, nil
}

// ==================== vLLM Backend ====================

// VLLMBackend implements Backend for vLLM
type VLLMBackend struct {
	*BaseBackend
	tensorParallelSize int
	gpuMemoryUtil      float64
}

// NewVLLMBackend creates a new vLLM backend
func NewVLLMBackend(config BackendConfig, tensorParallelSize int, gpuMemoryUtil float64) *VLLMBackend {
	return &VLLMBackend{
		BaseBackend:        NewBaseBackend(config),
		tensorParallelSize: tensorParallelSize,
		gpuMemoryUtil:      gpuMemoryUtil,
	}
}

// Name returns the backend name
func (b *VLLMBackend) Name() BackendType {
	return BackendVLLM
}

// IsHealthy checks if vLLM is healthy
func (b *VLLMBackend) IsHealthy(ctx context.Context) bool {
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
func (b *VLLMBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return openAICompatChat(ctx, b.BaseBackend, req)
}

// StreamChat streams chat completion
func (b *VLLMBackend) StreamChat(ctx context.Context, req *ChatRequest, callback func(chunk string) error) error {
	return openAICompatStream(ctx, b.BaseBackend, req, callback)
}

// Embed generates embeddings
func (b *VLLMBackend) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	return openAICompatEmbed(ctx, b.BaseBackend, req)
}

// ListModels lists available models
func (b *VLLMBackend) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return openAICompatListModels(ctx, b.BaseBackend)
}

// LoadModel is a no-op for vLLM (loaded at startup)
func (b *VLLMBackend) LoadModel(ctx context.Context, modelName string) error {
	return fmt.Errorf("vLLM loads models at startup, cannot load dynamically")
}

// UnloadModel is a no-op for vLLM
func (b *VLLMBackend) UnloadModel(ctx context.Context, modelName string) error {
	return fmt.Errorf("vLLM does not support dynamic model unloading")
}

// GetModelInfo gets model information
func (b *VLLMBackend) GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	return &ModelInfo{Name: modelName}, nil
}

// ==================== Helper Functions ====================

func openAICompatChat(ctx context.Context, b *BaseBackend, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	openAIReq := map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
		"stream":      false,
	}

	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, err
	}

	resp, err := b.DoRequest(ctx, "POST", "/v1/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	chatResp.LatencyMS = time.Since(start).Milliseconds()
	return &chatResp, nil
}

func openAICompatStream(ctx context.Context, b *BaseBackend, req *ChatRequest, callback func(chunk string) error) error {
	openAIReq := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}

	body, err := json.Marshal(openAIReq)
	if err != nil {
		return err
	}

	resp, err := b.DoRequest(ctx, "POST", "/v1/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	reader := NewSSEReader(resp.Body)
	for {
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if event.Data == "[DONE]" {
			break
		}

		var streamResp ChatResponse
		if err := json.Unmarshal([]byte(event.Data), &streamResp); err != nil {
			continue
		}

		if len(streamResp.Choices) > 0 && streamResp.Choices[0].Delta != nil {
			if err := callback(streamResp.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
	}

	return nil
}

func openAICompatEmbed(ctx context.Context, b *BaseBackend, req *EmbedRequest) (*EmbedResponse, error) {
	embedReq := map[string]any{
		"model": req.Model,
		"input": req.Input,
	}

	body, err := json.Marshal(embedReq)
	if err != nil {
		return nil, err
	}

	resp, err := b.DoRequest(ctx, "POST", "/v1/embeddings", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	var embedResp EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, err
	}

	return &embedResp, nil
}

func openAICompatListModels(ctx context.Context, b *BaseBackend) ([]ModelInfo, error) {
	resp, err := b.DoRequest(ctx, "GET", "/v1/models", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var modelsResp struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = ModelInfo{
			Name: m.ID,
			ID:   m.ID,
		}
	}

	return models, nil
}