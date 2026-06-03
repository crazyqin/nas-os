package aiconsole

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Provider 定义 AI 模型提供者接口.
type Provider interface {
	// Name 返回提供者名称.
	Name() ModelProvider
	// Chat 发送聊天请求.
	Chat(ctx context.Context, req *ProviderChatRequest) (*ProviderChatResponse, error)
	// HealthCheck 检查提供者健康状态.
	HealthCheck(ctx context.Context) error
	// SupportedModels 返回支持的模型列表.
	SupportedModels() []string
}

// ProviderChatRequest 提供者聊天请求.
type ProviderChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Stream      bool          `json:"stream"`
}

// ProviderChatResponse 提供者聊天响应.
type ProviderChatResponse struct {
	ID               string `json:"id"`
	Content          string `json:"content"`
	FinishReason     string `json:"finish_reason"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
}

// ProviderConfig 提供者配置.
type ProviderConfig struct {
	Name     ModelProvider `json:"name"`
	Endpoint string        `json:"endpoint"`
	APIKey   string        `json:"apiKey,omitempty"`
	Region   string        `json:"region,omitempty"`  // AWS 区域等
	Extra    map[string]string `json:"extra,omitempty"`
}

// ProviderManager 管理所有 AI 提供者.
type ProviderManager struct {
	mu        sync.RWMutex
	providers map[ModelProvider]Provider
	factory   *ProviderFactory
}

// NewProviderManager 创建提供者管理器.
func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		providers: make(map[ModelProvider]Provider),
		factory:   NewProviderFactory(),
	}
}

// Register 注册提供者.
func (pm *ProviderManager) Register(p Provider) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.providers[p.Name()] = p
}

// Get 获取提供者.
func (pm *ProviderManager) Get(name ModelProvider) (Provider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.providers[name]
	if !ok {
		return nil, fmt.Errorf("提供者 %s 未注册", name)
	}
	return p, nil
}

// List 列出所有已注册提供者.
func (pm *ProviderManager) List() []Provider {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	list := make([]Provider, 0, len(pm.providers))
	for _, p := range pm.providers {
		list = append(list, p)
	}
	return list
}

// CreateProvider 通过配置创建并注册提供者.
func (pm *ProviderManager) CreateProvider(config ProviderConfig) (Provider, error) {
	p, err := pm.factory.Create(config)
	if err != nil {
		return nil, err
	}
	pm.Register(p)
	return p, nil
}

// ProviderFactory 提供者工厂.
type ProviderFactory struct{}

// NewProviderFactory 创建提供者工厂.
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{}
}

// Create 根据配置创建提供者实例.
func (f *ProviderFactory) Create(config ProviderConfig) (Provider, error) {
	switch config.Name {
	case ProviderOpenAI:
		return NewOpenAICompatibleProvider(config), nil
	case ProviderAzureOpenAI:
		return NewAzureOpenAIProvider(config), nil
	case ProviderDeepSeek:
		return NewOpenAICompatibleProvider(config), nil
	case ProviderDoubao:
		return NewOpenAICompatibleProvider(config), nil
	case ProviderKimi:
		return NewOpenAICompatibleProvider(config), nil
	case ProviderHunyuan:
		return NewOpenAICompatibleProvider(config), nil
	case ProviderLocal:
		return NewOpenAICompatibleProvider(config), nil
	case ProviderCustom:
		return NewOpenAICompatibleProvider(config), nil
	default:
		return nil, fmt.Errorf("不支持的提供者类型: %s", config.Name)
	}
}

// ==================== OpenAI 兼容提供者 ====================

// OpenAICompatibleProvider OpenAI 兼容 API 提供者.
type OpenAICompatibleProvider struct {
	config ProviderConfig
	client *http.Client
}

// NewOpenAICompatibleProvider 创建 OpenAI 兼容提供者.
func NewOpenAICompatibleProvider(config ProviderConfig) *OpenAICompatibleProvider {
	return &OpenAICompatibleProvider{
		config: config,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// Name 返回提供者名称.
func (p *OpenAICompatibleProvider) Name() ModelProvider {
	return p.config.Name
}

// Chat 发送聊天请求.
func (p *OpenAICompatibleProvider) Chat(ctx context.Context, req *ProviderChatRequest) (*ProviderChatResponse, error) {
	// 委托给 service 的 callRemoteAPI 实现
	return nil, fmt.Errorf("需要通过 Service.Chat 调用")
}

// HealthCheck 健康检查.
func (p *OpenAICompatibleProvider) HealthCheck(ctx context.Context) error {
	// 尝试访问 endpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("创建健康检查请求失败: %w", err)
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("健康检查请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("服务端错误: HTTP %d", resp.StatusCode)
	}
	return nil
}

// SupportedModels 返回支持的模型列表.
func (p *OpenAICompatibleProvider) SupportedModels() []string {
	return []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo", "deepseek-chat", "deepseek-coder"}
}

// ==================== Azure OpenAI 提供者 ====================

// AzureOpenAIProvider Azure OpenAI 提供者.
type AzureOpenAIProvider struct {
	config ProviderConfig
	client *http.Client
}

// NewAzureOpenAIProvider 创建 Azure OpenAI 提供者.
func NewAzureOpenAIProvider(config ProviderConfig) *AzureOpenAIProvider {
	return &AzureOpenAIProvider{
		config: config,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// Name 返回提供者名称.
func (p *AzureOpenAIProvider) Name() ModelProvider {
	return ProviderAzureOpenAI
}

// Chat 发送聊天请求.
func (p *AzureOpenAIProvider) Chat(ctx context.Context, req *ProviderChatRequest) (*ProviderChatResponse, error) {
	return nil, fmt.Errorf("需要通过 Service.Chat 调用")
}

// HealthCheck 健康检查.
func (p *AzureOpenAIProvider) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("创建健康检查请求失败: %w", err)
	}
	httpReq.Header.Set("api-key", p.config.APIKey)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("健康检查请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("服务端错误: HTTP %d", resp.StatusCode)
	}
	return nil
}

// SupportedModels 返回支持的模型列表.
func (p *AzureOpenAIProvider) SupportedModels() []string {
	return []string{"gpt-4", "gpt-4-turbo", "gpt-35-turbo"}
}
