// Package ai provides AI service integration for NAS-OS
// backend_openai_remote.go - OpenAI remote API backend implementation
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIRemoteBackend implements Backend for OpenAI-compatible remote APIs
// Supports OpenAI, DeepSeek, Moonshot, Qwen, and other OpenAI-compatible services
type OpenAIRemoteBackend struct {
	*BaseBackend
	name      string
	apiKey    string
	orgID     string // OpenAI organization ID (optional)
	maxTokens int
}

// OpenAIRemoteConfig holds OpenAI remote backend configuration
type OpenAIRemoteConfig struct {
	Name      string        // Provider name (openai, deepseek, moonshot, etc.)
	Endpoint  string        // API endpoint
	APIKey    string        // API key
	OrgID     string        // Organization ID (OpenAI only)
	Model     string        // Default model
	MaxTokens int           // Default max tokens
	Timeout   time.Duration // Request timeout
}

// NewOpenAIRemoteBackend creates a new OpenAI remote backend
func NewOpenAIRemoteBackend(config BackendConfig, remoteCfg *OpenAIRemoteConfig) *OpenAIRemoteBackend {
	if remoteCfg.MaxTokens == 0 {
		remoteCfg.MaxTokens = 4096
	}

	return &OpenAIRemoteBackend{
		BaseBackend: NewBaseBackend(config),
		name:        remoteCfg.Name,
		apiKey:      remoteCfg.APIKey,
		orgID:       remoteCfg.OrgID,
		maxTokens:   remoteCfg.MaxTokens,
	}
}

// Name returns the backend name
func (b *OpenAIRemoteBackend) Name() BackendType {
	if b.name != "" {
		return BackendType(b.name)
	}
	return BackendType("openai-remote")
}

// IsHealthy checks if the API is accessible
func (b *OpenAIRemoteBackend) IsHealthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", b.config.Endpoint+"/v1/models", nil)
	if err != nil {
		b.SetHealthy(false)
		return false
	}

	b.setHeaders(req)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		b.SetHealthy(false)
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	healthy := resp.StatusCode < 500
	b.SetHealthy(healthy)
	return healthy
}

// Chat sends a chat request
func (b *OpenAIRemoteBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	openAIReq := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false,
	}

	if req.MaxTokens > 0 {
		openAIReq["max_tokens"] = req.MaxTokens
	} else {
		openAIReq["max_tokens"] = b.maxTokens
	}

	if req.Temperature > 0 {
		openAIReq["temperature"] = req.Temperature
	}

	if req.TopP > 0 {
		openAIReq["top_p"] = req.TopP
	}

	if len(req.Stop) > 0 {
		openAIReq["stop"] = req.Stop
	}

	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	url := strings.TrimSuffix(b.config.Endpoint, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	b.setHeaders(httpReq)

	httpResp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if httpResp.StatusCode >= 400 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error: %s - %s", httpResp.Status, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	chatResp.LatencyMS = time.Since(start).Milliseconds()
	return &chatResp, nil
}

// StreamChat streams chat completion
func (b *OpenAIRemoteBackend) StreamChat(ctx context.Context, req *ChatRequest, callback func(chunk string) error) error {
	openAIReq := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}

	if req.MaxTokens > 0 {
		openAIReq["max_tokens"] = req.MaxTokens
	}

	if req.Temperature > 0 {
		openAIReq["temperature"] = req.Temperature
	}

	body, err := json.Marshal(openAIReq)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	url := strings.TrimSuffix(b.config.Endpoint, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	b.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	httpResp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode >= 400 {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("API error: %s - %s", httpResp.Status, string(body))
	}

	reader := NewSSEReader(httpResp.Body)
	for {
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read SSE event failed: %w", err)
		}

		if event.Data == "[DONE]" {
			break
		}

		if event.Data == "" {
			continue
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

// Embed generates embeddings
func (b *OpenAIRemoteBackend) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	embedReq := map[string]any{
		"model": req.Model,
		"input": req.Input,
	}

	body, err := json.Marshal(embedReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	url := strings.TrimSuffix(b.config.Endpoint, "/") + "/v1/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	b.setHeaders(httpReq)

	httpResp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode >= 400 {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("API error: %s - %s", httpResp.Status, string(body))
	}

	var embedResp EmbedResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	return &embedResp, nil
}

// ListModels lists available models
func (b *OpenAIRemoteBackend) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", b.config.Endpoint+"/v1/models", nil)
	if err != nil {
		return nil, err
	}

	b.setHeaders(req)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to list models: %s", resp.Status)
	}

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
			Details: map[string]any{
				"owned_by": m.OwnedBy,
			},
		}
	}

	return models, nil
}

// LoadModel is a no-op for remote APIs
func (b *OpenAIRemoteBackend) LoadModel(ctx context.Context, modelName string) error {
	return nil
}

// UnloadModel is a no-op for remote APIs
func (b *OpenAIRemoteBackend) UnloadModel(ctx context.Context, modelName string) error {
	return nil
}

// GetModelInfo gets model information
func (b *OpenAIRemoteBackend) GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	models, err := b.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	for _, m := range models {
		if m.Name == modelName {
			return &m, nil
		}
	}

	return nil, fmt.Errorf("model %s not found", modelName)
}

// setHeaders sets common headers for requests
func (b *OpenAIRemoteBackend) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.apiKey)

	// OpenAI organization header
	if b.orgID != "" {
		req.Header.Set("OpenAI-Organization", b.orgID)
	}

	// Custom headers from config
	for k, v := range b.config.Headers {
		req.Header.Set(k, v)
	}
}

// ==================== Predefined OpenAI Remote Backend Factories ====================

// NewOpenAIRemoteBackendFromProvider creates a backend for a known provider
func NewOpenAIRemoteBackendFromProvider(provider string, apiKey string, defaultModel string) *OpenAIRemoteBackend {
	var endpoint string
	var name string

	switch strings.ToLower(provider) {
	case "openai":
		endpoint = "https://api.openai.com"
		name = "openai"
	case "deepseek":
		endpoint = "https://api.deepseek.com"
		name = "deepseek"
	case "moonshot", "kimi":
		endpoint = "https://api.moonshot.cn"
		name = "moonshot"
	case "zhipu", "glm":
		endpoint = "https://open.bigmodel.cn/api/paas/v4"
		name = "zhipu"
	case "qwen", "tongyi":
		endpoint = "https://dashscope.aliyuncs.com/compatible-mode"
		name = "qwen"
	default:
		endpoint = provider // Treat as custom endpoint
		name = "custom"
	}

	return NewOpenAIRemoteBackend(
		BackendConfig{
			Type:         BackendType(name),
			Endpoint:     endpoint,
			DefaultModel: defaultModel,
			Timeout:      120 * time.Second,
		},
		&OpenAIRemoteConfig{
			Name:      name,
			Endpoint:  endpoint,
			APIKey:    apiKey,
			Model:     defaultModel,
			MaxTokens: 4096,
		},
	)
}
