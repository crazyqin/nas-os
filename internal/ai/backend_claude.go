// Package ai provides AI service integration for NAS-OS
// backend_claude.go - Anthropic Claude backend implementation
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

// ClaudeBackend implements Backend for Anthropic Claude API
type ClaudeBackend struct {
	*BaseBackend
	apiKey    string
	version   string // API version (e.g., "2023-06-01")
	maxTokens int
}

// ClaudeConfig holds Claude-specific configuration
type ClaudeConfig struct {
	Endpoint  string
	APIKey    string
	Model     string
	Version   string
	MaxTokens int
	Timeout   time.Duration
}

// NewClaudeBackend creates a new Claude backend
func NewClaudeBackend(config BackendConfig, claudeCfg *ClaudeConfig) *ClaudeBackend {
	if claudeCfg.Version == "" {
		claudeCfg.Version = "2023-06-01"
	}
	if claudeCfg.MaxTokens == 0 {
		claudeCfg.MaxTokens = 4096
	}

	return &ClaudeBackend{
		BaseBackend: NewBaseBackend(config),
		apiKey:      claudeCfg.APIKey,
		version:     claudeCfg.Version,
		maxTokens:   claudeCfg.MaxTokens,
	}
}

// Name returns the backend name
func (b *ClaudeBackend) Name() BackendType {
	return BackendType("claude")
}

// IsHealthy checks if Claude API is accessible
func (b *ClaudeBackend) IsHealthy(ctx context.Context) bool {
	// Claude doesn't have a simple health endpoint, so we check if we can list models
	req, err := http.NewRequestWithContext(ctx, "GET", b.config.Endpoint+"/v1/models", nil)
	if err != nil {
		b.SetHealthy(false)
		return false
	}

	req.Header.Set("x-api-key", b.apiKey)
	req.Header.Set("anthropic-version", b.version)

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

// Chat sends a chat request to Claude
func (b *ClaudeBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	// Convert OpenAI format to Claude format
	claudeReq := b.convertRequest(req)

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	url := strings.TrimSuffix(b.config.Endpoint, "/") + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", b.apiKey)
	httpReq.Header.Set("anthropic-version", b.version)

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
		return nil, fmt.Errorf("claude API error: %s - %s", httpResp.Status, string(respBody))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	return b.convertResponse(&claudeResp, req.Model, start), nil
}

// StreamChat streams chat completion from Claude
func (b *ClaudeBackend) StreamChat(ctx context.Context, req *ChatRequest, callback func(chunk string) error) error {
	claudeReq := b.convertRequest(req)
	claudeReq.Stream = true

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	url := strings.TrimSuffix(b.config.Endpoint, "/") + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", b.apiKey)
	httpReq.Header.Set("anthropic-version", b.version)
	httpReq.Header.Set("Accept", "text/event-stream")

	httpResp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode >= 400 {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("claude API error: %s - %s", httpResp.Status, string(body))
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

		if event.Data == "" {
			continue
		}

		var streamResp claudeStreamEvent
		if err := json.Unmarshal([]byte(event.Data), &streamResp); err != nil {
			continue
		}

		if streamResp.Type == "content_block_delta" && streamResp.Delta != nil {
			if err := callback(streamResp.Delta.Text); err != nil {
				return err
			}
		}
	}

	return nil
}

// Embed is not supported by Claude (use a different backend)
func (b *ClaudeBackend) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	return nil, fmt.Errorf("claude does not support embeddings API")
}

// ListModels lists available Claude models
func (b *ClaudeBackend) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Return known Claude models
	return []ModelInfo{
		{Name: "claude-3-5-sonnet-20241022", ID: "claude-3-5-sonnet-20241022"},
		{Name: "claude-3-5-haiku-20241022", ID: "claude-3-5-haiku-20241022"},
		{Name: "claude-3-opus-20240229", ID: "claude-3-opus-20240229"},
		{Name: "claude-3-sonnet-20240229", ID: "claude-3-sonnet-20240229"},
		{Name: "claude-3-haiku-20240307", ID: "claude-3-haiku-20240307"},
	}, nil
}

// LoadModel is a no-op for Claude (cloud service)
func (b *ClaudeBackend) LoadModel(ctx context.Context, modelName string) error {
	return nil
}

// UnloadModel is a no-op for Claude
func (b *ClaudeBackend) UnloadModel(ctx context.Context, modelName string) error {
	return nil
}

// GetModelInfo gets model information
func (b *ClaudeBackend) GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	models, _ := b.ListModels(ctx)
	for _, m := range models {
		if m.Name == modelName {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("model %s not found", modelName)
}

// convertRequest converts OpenAI format to Claude format
func (b *ClaudeBackend) convertRequest(req *ChatRequest) *claudeRequest {
	claudeReq := &claudeRequest{
		Model:     req.Model,
		MaxTokens: b.maxTokens,
		Messages:  make([]claudeMessage, 0),
	}

	if req.MaxTokens > 0 {
		claudeReq.MaxTokens = req.MaxTokens
	}

	// Extract system message if present
	var systemPrompt string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			break
		}
	}
	claudeReq.System = systemPrompt

	// Convert messages (skip system)
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			continue
		}
		claudeReq.Messages = append(claudeReq.Messages, claudeMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return claudeReq
}

// convertResponse converts Claude response to OpenAI format
func (b *ClaudeBackend) convertResponse(claudeResp *claudeResponse, model string, start time.Time) *ChatResponse {
	content := ""
	if len(claudeResp.Content) > 0 {
		content = claudeResp.Content[0].Text
	}

	return &ChatResponse{
		ID:      claudeResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: claudeResp.StopReason,
			},
		},
		Usage: UsageInfo{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
		LatencyMS: time.Since(start).Milliseconds(),
	}
}

// Claude API types

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
	System    string          `json:"system,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Role         string               `json:"role"`
	Content      []claudeContentBlock `json:"content"`
	Model        string               `json:"model"`
	StopReason   string               `json:"stop_reason"`
	StopSequence string               `json:"stop_sequence,omitempty"`
	Usage        claudeUsage          `json:"usage"`
}

type claudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type claudeStreamEvent struct {
	Type         string              `json:"type"`
	Index        int                 `json:"index,omitempty"`
	Delta        *claudeStreamDelta  `json:"delta,omitempty"`
	Message      *claudeResponse     `json:"message,omitempty"`
	ContentBlock *claudeContentBlock `json:"content_block,omitempty"`
}

type claudeStreamDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
