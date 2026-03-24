// Package ai provides AI service integration for NAS-OS
// This file implements OpenAI-compatible API client
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

// OpenAICompatibleConfig OpenAI兼容API配置
type OpenAICompatibleConfig struct {
	// API配置
	APIKey      string
	Endpoint    string // API端点，如 https://api.openai.com/v1
	Model       string // 模型名称
	MaxTokens   int
	Temperature float64

	// 自定义配置
	Headers     map[string]string // 自定义请求头
	Provider    Provider          // 提供商标识
}

// OpenAICompatibleClient OpenAI兼容客户端
type OpenAICompatibleClient struct {
	config     *OpenAICompatibleConfig
	httpClient *http.Client
}

// NewOpenAICompatibleClient 创建OpenAI兼容客户端
func NewOpenAICompatibleClient(config *OpenAICompatibleConfig) (*OpenAICompatibleClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if config.Endpoint == "" {
		config.Endpoint = "https://api.openai.com/v1"
	}

	if config.Model == "" {
		config.Model = "gpt-3.5-turbo"
	}

	endpoint := strings.TrimSuffix(config.Endpoint, "/")
	config.Endpoint = endpoint

	httpClient := &http.Client{
		Timeout: 120 * time.Second, // AI请求可能较慢
	}

	return &OpenAICompatibleClient{
		config:     config,
		httpClient: httpClient,
	}, nil
}

// Chat 发送聊天请求
func (c *OpenAICompatibleClient) Chat(ctx context.Context, req *Request) (*Response, error) {
	// 构建请求体
	chatReq := OpenAIChatRequest{
		Model:       c.config.Model,
		Messages:    make([]OpenAIMessage, len(req.Messages)),
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
		Stream:      false,
	}

	// 如果请求中指定了模型，使用请求中的模型
	if req.Model != "" {
		chatReq.Model = req.Model
	}

	// 如果请求中指定了MaxTokens，使用请求中的值
	if req.MaxTokens > 0 {
		chatReq.MaxTokens = req.MaxTokens
	}

	// 如果请求中指定了Temperature，使用请求中的值
	if req.Temperature > 0 {
		chatReq.Temperature = req.Temperature
	}

	// 转换消息
	for i, msg := range req.Messages {
		chatReq.Messages[i] = OpenAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// 发送请求
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	url := c.config.Endpoint + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	// 添加自定义请求头
	for key, value := range c.config.Headers {
		httpReq.Header.Set(key, value)
	}

	// 发送请求
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	// 读取响应
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	// 检查HTTP状态码
	if httpResp.StatusCode >= 400 {
		var errResp OpenAIErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error: %s - %s", httpResp.Status, string(respBody))
	}

	// 解析响应
	var chatResp OpenAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	// 构建返回结果
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	choice := chatResp.Choices[0]
	provider := c.config.Provider
	if provider == "" {
		provider = ProviderCustom
	}

	return &Response{
		Content:      choice.Message.Content,
		Model:        chatResp.Model,
		Provider:     provider,
		TokensUsed:   chatResp.Usage.TotalTokens,
		FinishReason: choice.FinishReason,
	}, nil
}

// StreamChat 流式聊天
func (c *OpenAICompatibleClient) StreamChat(ctx context.Context, req *Request, callback func(chunk string) error) error {
	// 构建请求体
	chatReq := OpenAIChatRequest{
		Model:       c.config.Model,
		Messages:    make([]OpenAIMessage, len(req.Messages)),
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
		Stream:      true,
	}

	if req.Model != "" {
		chatReq.Model = req.Model
	}

	for i, msg := range req.Messages {
		chatReq.Messages[i] = OpenAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	url := c.config.Endpoint + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	for key, value := range c.config.Headers {
		httpReq.Header.Set(key, value)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode >= 400 {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("API error: %s - %s", httpResp.Status, string(body))
	}

	// 读取SSE流
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

		var chatResp OpenAIChatResponse
		if err := json.Unmarshal([]byte(event.Data), &chatResp); err != nil {
			continue
		}

		if len(chatResp.Choices) > 0 {
			delta := chatResp.Choices[0].Delta
			if delta.Content != "" {
				if err := callback(delta.Content); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Embed 生成文本嵌入
func (c *OpenAICompatibleClient) Embed(ctx context.Context, text string) ([]float32, error) {
	req := OpenAIEmbedRequest{
		Model: "text-embedding-ada-002",
		Input: text,
	}

	// 如果配置中指定了嵌入模型
	if strings.Contains(c.config.Model, "embed") {
		req.Model = c.config.Model
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	url := c.config.Endpoint + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	for key, value := range c.config.Headers {
		httpReq.Header.Set(key, value)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode >= 400 {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("API error: %s - %s", httpResp.Status, string(body))
	}

	var embedResp OpenAIEmbedResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	if len(embedResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	return embedResp.Data[0].Embedding, nil
}

// GetProvider 返回提供商类型
func (c *OpenAICompatibleClient) GetProvider() Provider {
	if c.config.Provider != "" {
		return c.config.Provider
	}
	return ProviderOpenAI
}

// ==================== OpenAI API Types ====================

// OpenAIChatRequest 聊天请求
type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	N           int             `json:"n,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
}

// OpenAIMessage 消息
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// OpenAIChatResponse 聊天响应
type OpenAIChatResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []OpenAIChoice     `json:"choices"`
	Usage   OpenAIUsage        `json:"usage"`
}

// OpenAIChoice 选择
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message,omitempty"`
	Delta        OpenAIDelta   `json:"delta,omitempty"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIDelta 流式增量
type OpenAIDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// OpenAIUsage 使用量
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIErrorResponse 错误响应
type OpenAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// OpenAIEmbedRequest 嵌入请求
type OpenAIEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// OpenAIEmbedResponse 嵌入响应
type OpenAIEmbedResponse struct {
	Object string          `json:"object"`
	Data   []OpenAIEmbedData `json:"data"`
	Model  string          `json:"model"`
	Usage  OpenAIUsage     `json:"usage"`
}

// OpenAIEmbedData 嵌入数据
type OpenAIEmbedData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// ==================== SSE Reader ====================

// SSEReader SSE读取器
type SSEReader struct {
	reader io.Reader
	buffer []byte
}

// NewSSEReader 创建SSE读取器
func NewSSEReader(reader io.Reader) *SSEReader {
	return &SSEReader{
		reader: reader,
		buffer: make([]byte, 0, 4096),
	}
}

// SSEEvent SSE事件
type SSEEvent struct {
	Event string
	Data  string
}

// ReadEvent 读取事件
func (r *SSEReader) ReadEvent() (*SSEEvent, error) {
	event := &SSEEvent{}

	for {
		// 读取一行
		line, err := r.readLine()
		if err != nil {
			return nil, err
		}

		// 空行表示事件结束
		if len(line) == 0 {
			if event.Data != "" {
				return event, nil
			}
			continue
		}

		// 解析行
		if strings.HasPrefix(line, "event:") {
			event.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if event.Data != "" {
				event.Data += "\n"
			}
			event.Data += data
		}
	}
}

// readLine 读取一行
func (r *SSEReader) readLine() (string, error) {
	for {
		// 检查buffer中是否有换行符
		for i, b := range r.buffer {
			if b == '\n' {
				line := string(r.buffer[:i])
				r.buffer = r.buffer[i+1:]
				return line, nil
			}
		}

		// 读取更多数据
		buf := make([]byte, 1024)
		n, err := r.reader.Read(buf)
		if err != nil {
			if len(r.buffer) > 0 {
				line := string(r.buffer)
				r.buffer = nil
				return line, nil
			}
			return "", err
		}

		r.buffer = append(r.buffer, buf[:n]...)
	}
}

// ==================== Predefined Providers ====================

// Predefined OpenAI-compatible providers
var (
	// OpenAI官方
	OpenAIEndpoint = "https://api.openai.com/v1"
	
	// Azure OpenAI (需要配置deployment)
	AzureOpenAIEndpoint = "https://YOUR_RESOURCE.openai.azure.com/openai/deployments/YOUR_DEPLOYMENT"
	
	// 国内常用OpenAI兼容服务
	// 这些服务提供OpenAI兼容的API接口
	
	// DeepSeek
	DeepSeekEndpoint = "https://api.deepseek.com/v1"
	
	// 智谱AI (GLM)
	ZhipuAIEndpoint = "https://open.bigmodel.cn/api/paas/v4"
	
	// 通义千问 (Qwen)
	QwenEndpoint = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	
	// 文心一言 (需要适配)
	// ERNIEEndpoint = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat"
	
	// Moonshot (Kimi)
	MoonshotEndpoint = "https://api.moonshot.cn/v1"
	
	// 本地LLM服务
	// OllamaEndpoint = "http://localhost:11434/v1"
	// LocalAIEndpoint = "http://localhost:8080/v1"
)

// NewOpenAIClient 创建OpenAI官方客户端
func NewOpenAIClient(apiKey, model string) (*OpenAICompatibleClient, error) {
	return NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:      apiKey,
		Endpoint:    OpenAIEndpoint,
		Model:       model,
		Provider:    ProviderOpenAI,
		MaxTokens:   4096,
		Temperature: 0.7,
	})
}

// NewDeepSeekClient 创建DeepSeek客户端
func NewDeepSeekClient(apiKey, model string) (*OpenAICompatibleClient, error) {
	if model == "" {
		model = "deepseek-chat"
	}
	return NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:      apiKey,
		Endpoint:    DeepSeekEndpoint,
		Model:       model,
		Provider:    ProviderCustom,
		MaxTokens:   4096,
		Temperature: 0.7,
	})
}

// NewZhipuAIClient 创建智谱AI客户端
func NewZhipuAIClient(apiKey, model string) (*OpenAICompatibleClient, error) {
	if model == "" {
		model = "glm-4"
	}
	return NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:      apiKey,
		Endpoint:    ZhipuAIEndpoint,
		Model:       model,
		Provider:    ProviderBaidu, // 使用百度作为标识（智谱是国产AI）
		MaxTokens:   4096,
		Temperature: 0.7,
	})
}

// NewQwenClient 创建通义千问客户端
func NewQwenClient(apiKey, model string) (*OpenAICompatibleClient, error) {
	if model == "" {
		model = "qwen-turbo"
	}
	return NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:      apiKey,
		Endpoint:    QwenEndpoint,
		Model:       model,
		Provider:    ProviderCustom,
		MaxTokens:   4096,
		Temperature: 0.7,
	})
}

// NewMoonshotClient 创建Moonshot客户端
func NewMoonshotClient(apiKey, model string) (*OpenAICompatibleClient, error) {
	if model == "" {
		model = "moonshot-v1-8k"
	}
	return NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:      apiKey,
		Endpoint:    MoonshotEndpoint,
		Model:       model,
		Provider:    ProviderCustom,
		MaxTokens:   4096,
		Temperature: 0.7,
	})
}

// NewLocalLLMClient 创建本地LLM客户端
func NewLocalLLMClient(endpoint, model string) (*OpenAICompatibleClient, error) {
	if model == "" {
		model = "llama2"
	}
	return NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:      "local", // 本地LLM通常不需要API Key
		Endpoint:    endpoint,
		Model:       model,
		Provider:    ProviderLocal,
		MaxTokens:   4096,
		Temperature: 0.7,
	})
}

// ==================== OpenAI-Compatible API List ====================

// OpenAICompatibleProvider OpenAI兼容提供商信息
type OpenAICompatibleProvider struct {
	Name        string
	Endpoint    string
	Description string
	Models      []string
	RequiresKey bool
}

// GetOpenAICompatibleProviders 返回支持的OpenAI兼容提供商列表
func GetOpenAICompatibleProviders() []OpenAICompatibleProvider {
	return []OpenAICompatibleProvider{
		{
			Name:        "OpenAI",
			Endpoint:    OpenAIEndpoint,
			Description: "OpenAI官方API",
			Models:      []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
			RequiresKey: true,
		},
		{
			Name:        "DeepSeek",
			Endpoint:    DeepSeekEndpoint,
			Description: "DeepSeek AI",
			Models:      []string{"deepseek-chat", "deepseek-coder"},
			RequiresKey: true,
		},
		{
			Name:        "智谱AI",
			Endpoint:    ZhipuAIEndpoint,
			Description: "智谱AI GLM系列模型",
			Models:      []string{"glm-4", "glm-4-flash", "glm-3-turbo"},
			RequiresKey: true,
		},
		{
			Name:        "通义千问",
			Endpoint:    QwenEndpoint,
			Description: "阿里云通义千问",
			Models:      []string{"qwen-turbo", "qwen-plus", "qwen-max"},
			RequiresKey: true,
		},
		{
			Name:        "Moonshot",
			Endpoint:    MoonshotEndpoint,
			Description: "Moonshot AI (Kimi)",
			Models:      []string{"moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k"},
			RequiresKey: true,
		},
		{
			Name:        "本地LLM",
			Endpoint:    "http://localhost:11434/v1",
			Description: "本地部署的LLM服务 (Ollama/LocalAI)",
			Models:      []string{"自定义模型"},
			RequiresKey: false,
		},
	}
}