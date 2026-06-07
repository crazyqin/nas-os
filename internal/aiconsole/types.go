// Package aiconsole 提供 AI Console 功能模块
// 对标群晖 DSM 7.3 AI Console，提供统一 AI 模型管理、隐私数据脱敏、会话审计等能力
package aiconsole

import (
	"time"
)

// ========== AI 模型配置 ==========

// ModelProvider 模型提供者类型.
type ModelProvider string

const (
	// ProviderOpenAI OpenAI 兼容 API.
	ProviderOpenAI ModelProvider = "openai"
	// ProviderAzureOpenAI Azure OpenAI.
	ProviderAzureOpenAI ModelProvider = "azure_openai"
	// ProviderAWSBedrock AWS Bedrock.
	ProviderAWSBedrock ModelProvider = "aws_bedrock"
	// ProviderGoogleGemini Google Gemini.
	ProviderGoogleGemini ModelProvider = "google_gemini"
	// ProviderDeepSeek DeepSeek.
	ProviderDeepSeek ModelProvider = "deepseek"
	// ProviderDoubao 豆包（字节跳动）.
	ProviderDoubao ModelProvider = "doubao"
	// ProviderKimi Kimi（月之暗面）.
	ProviderKimi ModelProvider = "kimi"
	// ProviderHunyuan 混元（腾讯）.
	ProviderHunyuan ModelProvider = "hunyuan"
	// ProviderLocal 本地 LLM（Ollama 等）.
	ProviderLocal ModelProvider = "local"
	// ProviderCustom 自定义提供者.
	ProviderCustom ModelProvider = "custom"
)

// ModelStatus 模型状态.
type ModelStatus string

const (
	// ModelStatusActive 活跃可用.
	ModelStatusActive ModelStatus = "active"
	// ModelStatusDisabled 已禁用.
	ModelStatusDisabled ModelStatus = "disabled"
	// ModelStatusError 错误状态.
	ModelStatusError ModelStatus = "error"
)

// AIModel AI 模型配置.
type AIModel struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Provider    ModelProvider `json:"provider"`
	Endpoint    string        `json:"endpoint"`
	APIKey      string        `json:"apiKey,omitempty"` // 存储时加密
	ModelName   string        `json:"modelName"`        // 实际模型标识，如 gpt-4、llama3
	MaxTokens   int           `json:"maxTokens"`
	Temperature float64       `json:"temperature"`
	Status      ModelStatus   `json:"status"`
	IsDefault   bool          `json:"isDefault"`
	Enabled     bool          `json:"enabled"`
	Description string        `json:"description,omitempty"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

// CreateModelRequest 创建模型请求.
type CreateModelRequest struct {
	Name        string        `json:"name" binding:"required"`
	Provider    ModelProvider `json:"provider" binding:"required"`
	Endpoint    string        `json:"endpoint" binding:"required"`
	APIKey      string        `json:"apiKey,omitempty"`
	ModelName   string        `json:"modelName" binding:"required"`
	MaxTokens   int           `json:"maxTokens"`
	Temperature float64       `json:"temperature"`
	IsDefault   bool          `json:"isDefault"`
	Description string        `json:"description,omitempty"`
}

// ========== 聊天相关 ==========

// ChatMessage 聊天消息.
type ChatMessage struct {
	Role    string `json:"role" binding:"required"` // system, user, assistant
	Content string `json:"content" binding:"required"`
}

// ChatRequest 聊天请求.
type ChatRequest struct {
	ModelID     string        `json:"modelId"` // 使用已注册模型的 ID
	Messages    []ChatMessage `json:"messages" binding:"required"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"maxTokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatResponse 聊天响应.
type ChatResponse struct {
	ID               string `json:"id"`
	ModelID          string `json:"modelId"`
	Content          string `json:"content"`
	FinishReason     string `json:"finishReason,omitempty"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	TotalTokens      int    `json:"totalTokens"`
	DurationMs       int64  `json:"durationMs"`
	Redacted         bool   `json:"redacted"` // 是否经过脱敏处理
	RedactCount      int    `json:"redactCount"`
}

// ========== 脱敏规则 ==========

// PIIType 个人可识别信息类型.
type PIIType string

const (
	// PIIEmail 邮箱.
	PIIEmail PIIType = "email"
	// PIIPhone 电话号码.
	PIIPhone PIIType = "phone"
	// PIIIDCard 身份证号.
	PIIIDCard PIIType = "id_card"
	// PIIBankCard 银行卡号.
	PIIBankCard PIIType = "bank_card"
	// PIIName 姓名（中文姓名）.
	PIIName PIIType = "name"
	// PIIPassport 护照号.
	PIIPassport PIIType = "passport"
	// PIIIPAddress IP 地址.
	PIIIPAddress PIIType = "ip_address"
	// PIICustom 自定义规则.
	PIICustom PIIType = "custom"
)

// RedactStrategy 脱敏策略.
type RedactStrategy string

const (
	// StrategyMask 掩码替换（如 ****）.
	StrategyMask RedactStrategy = "mask"
	// StrategyPartial 部分显示.
	StrategyPartial RedactStrategy = "partial"
	// StrategyHash 哈希替换.
	StrategyHash RedactStrategy = "hash"
	// StrategyRemove 完全移除.
	StrategyRemove RedactStrategy = "remove"
)

// RedactRule 脱敏规则.
type RedactRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	PIIType     PIIType        `json:"piiType"`
	Pattern     string         `json:"pattern"` // 正则表达式
	Strategy    RedactStrategy `json:"strategy"`
	MaskChar    string         `json:"maskChar,omitempty"`    // 掩码字符，默认 *
	ShowFirst   int            `json:"showFirst,omitempty"`   // 显示前 N 位
	ShowLast    int            `json:"showLast,omitempty"`    // 显示后 N 位
	Replacement string         `json:"replacement,omitempty"` // 自定义替换文本
	Enabled     bool           `json:"enabled"`
	Priority    int            `json:"priority"` // 越大越先处理
	Description string         `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// CreateRuleRequest 创建脱敏规则请求.
type CreateRuleRequest struct {
	Name        string         `json:"name" binding:"required"`
	PIIType     PIIType        `json:"piiType" binding:"required"`
	Pattern     string         `json:"pattern" binding:"required"`
	Strategy    RedactStrategy `json:"strategy" binding:"required"`
	MaskChar    string         `json:"maskChar,omitempty"`
	ShowFirst   int            `json:"showFirst,omitempty"`
	ShowLast    int            `json:"showLast,omitempty"`
	Replacement string         `json:"replacement,omitempty"`
	Enabled     bool           `json:"enabled"`
	Priority    int            `json:"priority"`
	Description string         `json:"description,omitempty"`
}

// UpdateRuleRequest 更新脱敏规则请求.
type UpdateRuleRequest struct {
	Name        *string         `json:"name,omitempty"`
	PIIType     *PIIType        `json:"piiType,omitempty"`
	Pattern     *string         `json:"pattern,omitempty"`
	Strategy    *RedactStrategy `json:"strategy,omitempty"`
	MaskChar    *string         `json:"maskChar,omitempty"`
	ShowFirst   *int            `json:"showFirst,omitempty"`
	ShowLast    *int            `json:"showLast,omitempty"`
	Replacement *string         `json:"replacement,omitempty"`
	Enabled     *bool           `json:"enabled,omitempty"`
	Priority    *int            `json:"priority,omitempty"`
	Description *string         `json:"description,omitempty"`
}

// ========== 脱敏结果 ==========

// RedactDetail 单次脱敏详情.
type RedactDetail struct {
	PIIType  PIIType        `json:"piiType"`
	Start    int            `json:"start"`
	End      int            `json:"end"`
	Original string         `json:"-"` // 不暴露原文
	Replaced string         `json:"replaced"`
	Strategy RedactStrategy `json:"strategy"`
	RuleID   string         `json:"ruleId"`
	RuleName string         `json:"ruleName"`
}

// RedactResult 脱敏结果.
type RedactResult struct {
	Processed    string         `json:"processed"`
	RedactCount  int            `json:"redactCount"`
	Redactions   []RedactDetail `json:"redactions,omitempty"`
	HasRedaction bool           `json:"hasRedaction"`
}

// ========== 审计日志 ==========

// AuditEntry 审计日志条目.
type AuditEntry struct {
	ID               string                 `json:"id"`
	Timestamp        time.Time              `json:"timestamp"`
	UserID           string                 `json:"userId"`
	Username         string                 `json:"username"`
	ModelID          string                 `json:"modelName"`
	ModelName        string                 `json:"modelNameDisplay"`
	Action           string                 `json:"action"`          // chat, model_add, model_delete, rule_change
	RequestSummary   string                 `json:"requestSummary"`  // 脱敏后的请求摘要（截断）
	ResponseSummary  string                 `json:"responseSummary"` // 脱敏后的响应摘要（截断）
	PromptTokens     int                    `json:"promptTokens"`
	CompletionTokens int                    `json:"completionTokens"`
	TotalTokens      int                    `json:"totalTokens"`
	DurationMs       int64                  `json:"durationMs"`
	Success          bool                   `json:"success"`
	ErrorMessage     string                 `json:"errorMessage,omitempty"`
	Redacted         bool                   `json:"redacted"`
	RedactCount      int                    `json:"redactCount"`
	IPAddress        string                 `json:"ipAddress,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// AuditQueryFilter 审计日志查询过滤器.
type AuditQueryFilter struct {
	UserID    string    `form:"userId"`
	Action    string    `form:"action"`
	StartTime time.Time `form:"startTime"`
	EndTime   time.Time `form:"endTime"`
	Success   *bool     `form:"success"`
	Page      int       `form:"page"`
	PageSize  int       `form:"pageSize"`
}
