package ollama

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

// Client is an Ollama API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// Config for Ollama client
type Config struct {
	BaseURL string
	Timeout time.Duration
}

// NewClient creates a new Ollama client
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	
	return &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		timeout: cfg.Timeout,
	}
}

// Model represents an Ollama model
type Model struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
	ModifiedAt string `json:"modified_at"`
}

// GenerateRequest for text generation
type GenerateRequest struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	Stream      bool   `json:"stream"`
	Context     []int  `json:"context,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int    `json:"top_k,omitempty"`
	MaxTokens   int    `json:"num_predict,omitempty"`
}

// GenerateResponse from generation
type GenerateResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Context   []int  `json:"context,omitempty"`
	Done      bool   `json:"done"`
	Tokens    int    `json:"tokens_evaluated,omitempty"`
}

// ChatRequest for chat completion
type ChatRequest struct {
	Model    string     `json:"model"`
	Messages []Message  `json:"messages"`
	Stream   bool       `json:"stream"`
}

// Message for chat
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse from chat
type ChatResponse struct {
	Model     string   `json:"model"`
	Message   Message  `json:"message"`
	Done      bool     `json:"done"`
	Tokens    int      `json:"tokens_evaluated,omitempty"`
}

// EmbeddingRequest for embeddings
type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// EmbeddingResponse from embedding
type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// PullRequest for downloading models
type PullRequest struct {
	Name   string `json:"name"`
	Stream bool   `json:"stream"`
}

// PullResponse for model download progress
type PullResponse struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

// ListModels returns available models
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	
	var result struct {
		Models []Model `json:"models"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result.Models, nil
}

// PullModel downloads a model
func (c *Client) PullModel(ctx context.Context, name string, progress func(PullResponse)) error {
	body := PullRequest{
		Name:   name,
		Stream: true,
	}
	
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/pull", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	decoder := json.NewDecoder(resp.Body)
	for {
		var result PullResponse
		if err := decoder.Decode(&result); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		
		if progress != nil {
			progress(result)
		}
		
		if result.Status == "success" {
			break
		}
	}
	
	return nil
}

// Generate generates text from prompt
func (c *Client) Generate(ctx context.Context, req GenerateRequest) (string, error) {
	req.Stream = false
	
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	var result GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	
	return result.Response, nil
}

// GenerateStream generates text with streaming
func (c *Client) GenerateStream(ctx context.Context, req GenerateRequest, callback func(string)) error {
	req.Stream = true
	
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return err
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	decoder := json.NewDecoder(resp.Body)
	for {
		var result GenerateResponse
		if err := decoder.Decode(&result); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		
		if callback != nil && result.Response != "" {
			callback(result.Response)
		}
		
		if result.Done {
			break
		}
	}
	
	return nil
}

// Chat sends chat messages
func (c *Client) Chat(ctx context.Context, req ChatRequest) (Message, error) {
	req.Stream = false
	
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return Message{}, err
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return Message{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()
	
	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Message{}, err
	}
	
	return result.Message, nil
}

// ChatStream sends chat messages with streaming
func (c *Client) ChatStream(ctx context.Context, req ChatRequest, callback func(Message)) error {
	req.Stream = true
	
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return err
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	decoder := json.NewDecoder(resp.Body)
	for {
		var result ChatResponse
		if err := decoder.Decode(&result); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		
		if callback != nil && result.Message.Content != "" {
			callback(result.Message)
		}
		
		if result.Done {
			break
		}
	}
	
	return nil
}

// GetEmbedding generates embedding for text
func (c *Client) GetEmbedding(ctx context.Context, model, prompt string) ([]float64, error) {
	body := EmbeddingRequest{
		Model:  model,
		Prompt: prompt,
	}
	
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var result EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result.Embedding, nil
}

// DeleteModel removes a model
func (c *Client) DeleteModel(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/delete", nil)
	if err != nil {
		return err
	}
	
	// Set model name in body
	body := map[string]string{"name": name}
	jsonBody, _ := json.Marshal(body)
	req.Body = io.NopCloser(bytes.NewReader(jsonBody))
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	
	return nil
}

// ShowModelInfo returns model details
func (c *Client) ShowModelInfo(ctx context.Context, name string) (map[string]interface{}, error) {
	body := map[string]string{"name": name}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/show", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result, nil
}

// IsRunning checks if Ollama server is running
func (c *Client) IsRunning(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL, nil)
	if err != nil {
		return false
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK
}

// HealthCheck returns Ollama health status
func (c *Client) HealthCheck(ctx context.Context) HealthStatus {
	return HealthStatus{
		Running:   c.IsRunning(ctx),
		BaseURL:   c.baseURL,
		Models:    c.getModelCount(ctx),
		Timestamp: time.Now(),
	}
}

func (c *Client) getModelCount(ctx context.Context) int {
	models, err := c.ListModels(ctx)
	if err != nil {
		return 0
	}
	return len(models)
}

// HealthStatus for Ollama
type HealthStatus struct {
	Running   bool      `json:"running"`
	BaseURL   string    `json:"base_url"`
	Models    int       `json:"models"`
	Timestamp time.Time `json:"timestamp"`
}

// RecommendedModels for NAS-OS use cases
var RecommendedModels = []ModelRecommendation{
	{Name: "llama3.2", Description: "通用对话模型 (3B)", UseCase: "chat"},
	{Name: "llama3.1:8b", Description: "通用对话模型 (8B)", UseCase: "chat"},
	{Name: "mistral", Description: "高效推理模型 (7B)", UseCase: "chat"},
	{Name: "codellama", Description: "代码辅助模型", UseCase: "code"},
	{Name: "nomic-embed-text", Description: "文本嵌入模型", UseCase: "embedding"},
	{Name: "phi3", Description: "轻量推理模型 (3.8B)", UseCase: "chat"},
}

// ModelRecommendation for recommended model
type ModelRecommendation struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	UseCase     string `json:"use_case"`
}

// GetRecommendedModelsByUseCase returns models for specific use case
func GetRecommendedModelsByUseCase(useCase string) []ModelRecommendation {
	var result []ModelRecommendation
	for _, model := range RecommendedModels {
		if model.UseCase == useCase || strings.Contains(model.UseCase, useCase) {
			result = append(result, model)
		}
	}
	return result
}