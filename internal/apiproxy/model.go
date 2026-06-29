// Package apiproxy 提供 OpenAI 兼容的 AI 网关代理
// 支持多 provider 智能路由、负载均衡、故障转移、API Key 管理和配额限速
// 灵感来源：群晖 DSM 7.3 AI Console — Connect DSM with AI models of your choice
package apiproxy

import (
	"time"
)

// ========== AI Provider 数据模型 ==========

// AIProvider 描述一个 AI 服务提供者（云端或本地）
type AIProvider struct {
	// ID 唯一标识
	ID string `json:"id"`
	// Name 提供者名称，如 "OpenAI"、"本地 Ollama"
	Name string `json:"name"`
	// Endpoint API 端点地址，如 "https://api.openai.com" 或 "http://localhost:11434"
	Endpoint string `json:"endpoint"`
	// APIKey 认证密钥（本地模型可为空）
	APIKey string `json:"-"` // json 序列化时隐藏
	// Models 该 provider 支持的模型列表
	Models []string `json:"models"`
	// IsLocal 是否为本地模型
	IsLocal bool `json:"isLocal"`
	// Priority 优先级，数值越小优先级越高（本地模型默认更低）
	Priority int `json:"priority"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// MaxQPS 最大每秒请求数，0 表示不限制
	MaxQPS int `json:"maxQPS"`
	//CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
}

// ProviderHealth provider 健康状态
type ProviderHealth struct {
	ProviderID  string    `json:"providerId"`
	Healthy     bool      `json:"healthy"`
	LastError   string    `json:"lastError,omitempty"`
	LastChecked time.Time `json:"lastChecked"`
	// ConsecutiveFailures 连续失败次数，用于故障转移判定
	ConsecutiveFailures int `json:"consecutiveFailures"`
}

// ========== AI 请求/响应模型 ==========

// AIRequest 统一的 AI 请求结构
type AIRequest struct {
	// Model 模型名称，如 "gpt-4"、"llama3"
	Model string `json:"model"`
	// Messages 对话消息列表（OpenAI 兼容格式）
	Messages []Message `json:"messages"`
	// Temperature 温度参数，控制随机性
	Temperature float64 `json:"temperature,omitempty"`
	// MaxTokens 最大生成 token 数
	MaxTokens int `json:"maxTokens,omitempty"`
	// Stream 是否流式返回
	Stream bool `json:"stream,omitempty"`
	// TopP 核采样参数
	TopP float64 `json:"topP,omitempty"`
	// Stop 停止序列
	Stop []string `json:"stop,omitempty"`
}

// Message OpenAI 兼容的对话消息
type Message struct {
	Role    string `json:"role"`    // system / user / assistant / tool
	Content string `json:"content"`
}

// AIResponse 非流式 AI 响应（OpenAI 兼容格式）
type AIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"` // "chat.completion"
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice 响应选项
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage token 用量统计
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk 流式响应块（OpenAI SSE 格式）
type StreamChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"` // "chat.completion.chunk"
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// StreamChoice 流式响应选项
type StreamChoice struct {
	Index        int          `json:"index"`
	Delta        Message      `json:"delta"`
	FinishReason *string      `json:"finish_reason"` // 可能为 null
}

// ========== API Key 管理模型 ==========

// APIKeyConfig API Key 配置
type APIKeyConfig struct {
	// Key 完整 API Key（仅在创建时返回，后续不可见）
	Key string `json:"key,omitempty"`
	// KeyID Key 的唯一标识
	KeyID string `json:"keyId"`
	// KeyPrefix Key 前缀（用于展示，如 "sk-abc..."）
	KeyPrefix string `json:"keyPrefix"`
	// UserID 所属用户 ID
	UserID string `json:"userId"`
	// Name 备注/标签名
	Name string `json:"name"`
	// AllowedModels 允许使用的模型列表，空表示全部允许
	AllowedModels []string `json:"allowedModels,omitempty"`
	// RateLimitPerMin 每分钟请求限制，0 表示不限制
	RateLimitPerMin int `json:"rateLimitPerMin,omitempty"`
	// DailyQuota 每日 token 配额，0 表示不限制
	DailyQuota int `json:"dailyQuota,omitempty"`
	// MonthlyQuota 每月 token 配额，0 表示不限制
	MonthlyQuota int `json:"monthlyQuota,omitempty"`
	// Revoked 是否已吊销
	Revoked bool `json:"revoked"`
	// ExpiresAt 过期时间，nil 表示永不过期
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// LastUsedAt 最后使用时间
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

// KeyUsageStat Key 使用统计
type KeyUsageStat struct {
	KeyID            string    `json:"keyId"`
	TodayTokens      int       `json:"todayTokens"`
	MonthTokens      int       `json:"monthTokens"`
	TodayRequests    int       `json:"todayRequests"`
	MonthRequests    int       `json:"monthRequests"`
	LastResetDaily   time.Time `json:"lastResetDaily"`
	LastResetMonthly time.Time `json:"lastResetMonthly"`
}

// ========== OpenAI 兼容请求体 ==========

// ChatCompletionRequest OpenAI /v1/chat/completions 请求体
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

// ChatCompletionResponse OpenAI /v1/chat/completions 响应体
type ChatCompletionResponse = AIResponse

// ErrorResponse 错误响应（OpenAI 兼容格式）
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody 错误内容
type ErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}
