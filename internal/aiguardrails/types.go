// Package aiguardrails 提供 AI 安全护栏功能，
// 参考群晖 DSM 7.4 的 AI 治理功能，实现输入/输出过滤、敏感数据检测、
// PII 检测、Prompt Injection 防护和审计日志。
package aiguardrails

import "time"

// ========== 护栏策略类型 ==========

// PolicyType 护栏策略类型.
type PolicyType string

const (
	// PolicyInputFilter 输入过滤策略.
	PolicyInputFilter PolicyType = "input_filter"
	// PolicyOutputFilter 输出过滤策略.
	PolicyOutputFilter PolicyType = "output_filter"
	// PolicySensitiveData 敏感数据检测策略.
	PolicySensitiveData PolicyType = "sensitive_data"
	// PolicyPII 个人身份信息检测策略.
	PolicyPII PolicyType = "pii_detection"
	// PolicyPromptInjection Prompt Injection 防护策略.
	PolicyPromptInjection PolicyType = "prompt_injection"
	// PolicyContentSafety 内容安全策略.
	PolicyContentSafety PolicyType = "content_safety"
)

// RuleType 规则匹配类型.
type RuleType string

const (
	// RuleRegex 正则匹配规则.
	RuleRegex RuleType = "regex"
	// RuleKeyword 关键词匹配规则.
	RuleKeyword RuleType = "keyword"
	// RuleSemantic 语义匹配规则.
	RuleSemantic RuleType = "semantic"
	// RuleLength 长度限制规则.
	RuleLength RuleType = "length"
	// RulePattern 模式匹配规则.
	RulePattern RuleType = "pattern"
)

// Action 执行动作类型.
type Action string

const (
	// ActionAllow 放行.
	ActionAllow Action = "allow"
	// ActionBlock 阻止.
	ActionBlock Action = "block"
	// ActionWarn 警告.
	ActionWarn Action = "warn"
	// ActionRedact 脱敏替换.
	ActionRedact Action = "redact"
	// ActionQuarantine 隔离.
	ActionQuarantine Action = "quarantine"
)

// Severity 严重程度.
type Severity string

const (
	// SeverityLow 低.
	SeverityLow Severity = "low"
	// SeverityMedium 中.
	SeverityMedium Severity = "medium"
	// SeverityHigh 高.
	SeverityHigh Severity = "high"
	// SeverityCritical 严重.
	SeverityCritical Severity = "critical"
)

// PolicyStatus 策略状态.
type PolicyStatus string

const (
	// StatusEnabled 已启用.
	StatusEnabled PolicyStatus = "enabled"
	// StatusDisabled 已禁用.
	StatusDisabled PolicyStatus = "disabled"
)

// ========== 核心结构定义 ==========

// GuardrailRule 护栏规则.
type GuardrailRule struct {
	ID          string   `json:"id"`                    // 规则唯一标识
	Name        string   `json:"name"`                  // 规则名称
	Type        RuleType `json:"type"`                  // 规则类型
	Pattern     string   `json:"pattern"`               // 匹配模式（正则表达式或关键词）
	Description string   `json:"description,omitempty"` // 规则描述
	Severity    Severity `json:"severity"`              // 严重程度
	Action      Action   `json:"action"`                // 命中时执行的动作
	Enabled     bool     `json:"enabled"`               // 是否启用
}

// GuardrailPolicy 护栏策略.
type GuardrailPolicy struct {
	ID          string          `json:"id"`                    // 策略唯一标识
	Name        string          `json:"name"`                  // 策略名称
	Type        PolicyType      `json:"type"`                  // 策略类型
	Description string          `json:"description,omitempty"` // 策略描述
	Rules       []GuardrailRule `json:"rules"`                 // 规则列表
	Status      PolicyStatus    `json:"status"`                // 策略状态
	Priority    int             `json:"priority"`              // 优先级（数字越小越高）
	CreatedAt   time.Time       `json:"created_at"`            // 创建时间
	UpdatedAt   time.Time       `json:"updated_at"`            // 更新时间
	CreatedBy   string          `json:"created_by"`            // 创建者
}

// AIGuardrailConfig AI 护栏全局配置.
type AIGuardrailConfig struct {
	Enabled              bool     `json:"enabled"`                    // 全局是否启用
	MaxInputLength       int      `json:"max_input_length"`           // 最大输入长度
	MaxOutputLength      int      `json:"max_output_length"`          // 最大输出长度
	RedactPII            bool     `json:"redact_pii"`                 // 是否自动脱敏 PII
	BlockPromptInjection bool     `json:"block_prompt_injection"`     // 是否阻止 Prompt Injection
	LogAllRequests       bool     `json:"log_all_requests"`           // 是否记录所有请求
	RetentionDays        int      `json:"retention_days"`             // 审计日志保留天数
	WhitelistModels      []string `json:"whitelist_models,omitempty"` // 白名单模型
	BlacklistModels      []string `json:"blacklist_models,omitempty"` // 黑名单模型
}

// ========== 检测结果类型 ==========

// DetectionResult 检测结果.
type DetectionResult struct {
	Hit          bool       `json:"hit"`                     // 是否命中规则
	RuleID       string     `json:"rule_id,omitempty"`       // 命中规则 ID
	RuleName     string     `json:"rule_name,omitempty"`     // 命中规则名称
	PolicyType   PolicyType `json:"policy_type,omitempty"`   // 策略类型
	Severity     Severity   `json:"severity,omitempty"`      // 严重程度
	Action       Action     `json:"action"`                  // 执行动作
	MatchedText  string     `json:"matched_text,omitempty"`  // 命中内容片段
	RedactedText string     `json:"redacted_text,omitempty"` // 脱敏后内容
	Message      string     `json:"message,omitempty"`       // 说明消息
}

// FilterRequest 过滤请求.
type FilterRequest struct {
	Text      string `json:"text" binding:"required"` // 待检测文本
	Direction string `json:"direction,omitempty"`     // 方向：input/output
	Model     string `json:"model,omitempty"`         // 目标模型
	User      string `json:"user,omitempty"`          // 用户标识
	ClientIP  string `json:"client_ip,omitempty"`     // 客户端 IP
}

// FilterResponse 过滤响应.
type FilterResponse struct {
	Allowed   bool              `json:"allowed"`          // 是否放行
	Results   []DetectionResult `json:"results"`          // 检测结果列表
	CleanText string            `json:"clean_text"`       // 处理后文本
	Action    Action            `json:"action"`           // 最终动作
	Reason    string            `json:"reason,omitempty"` // 原因
}

// ========== 审计日志类型 ==========

// AuditLogEntry 审计日志条目.
type AuditLogEntry struct {
	ID         string            `json:"id"`                    // 日志唯一标识
	Timestamp  time.Time         `json:"timestamp"`             // 记录时间
	User       string            `json:"user,omitempty"`        // 操作用户
	ClientIP   string            `json:"client_ip,omitempty"`   // 客户端 IP
	Direction  string            `json:"direction"`             // input/output
	Model      string            `json:"model,omitempty"`       // 目标模型
	InputText  string            `json:"input_text"`            // 输入文本（截断）
	OutputText string            `json:"output_text,omitempty"` // 输出文本（截断）
	Action     Action            `json:"action"`                // 执行动作
	Results    []DetectionResult `json:"results,omitempty"`     // 检测结果
	Reason     string            `json:"reason,omitempty"`      // 原因
}

// ========== 请求/响应类型 ==========

// PolicyRequest 创建/更新策略请求.
type PolicyRequest struct {
	Name        string          `json:"name" binding:"required"`       // 策略名称
	Type        PolicyType      `json:"type" binding:"required"`       // 策略类型
	Description string          `json:"description,omitempty"`         // 描述
	Rules       []GuardrailRule `json:"rules"`                         // 规则列表
	Priority    int             `json:"priority,omitempty"`            // 优先级
	CreatedBy   string          `json:"created_by" binding:"required"` // 创建者
}

// ConfigRequest 更新全局配置请求.
type ConfigRequest struct {
	Enabled              bool     `json:"enabled"`                     // 是否启用
	MaxInputLength       int      `json:"max_input_length,omitempty"`  // 最大输入长度
	MaxOutputLength      int      `json:"max_output_length,omitempty"` // 最大输出长度
	RedactPII            bool     `json:"redact_pii"`                  // 是否脱敏 PII
	BlockPromptInjection bool     `json:"block_prompt_injection"`      // 是否阻止注入
	LogAllRequests       bool     `json:"log_all_requests"`            // 是否记录所有请求
	RetentionDays        int      `json:"retention_days,omitempty"`    // 保留天数
	WhitelistModels      []string `json:"whitelist_models,omitempty"`  // 白名单
	BlacklistModels      []string `json:"blacklist_models,omitempty"`  // 黑名单
}

// AuditQuery 审计日志查询.
type AuditQuery struct {
	Direction string     `json:"direction,omitempty"`  // 方向过滤
	User      string     `json:"user,omitempty"`       // 用户过滤
	Action    Action     `json:"action,omitempty"`     // 动作过滤
	StartTime *time.Time `json:"start_time,omitempty"` // 起始时间
	EndTime   *time.Time `json:"end_time,omitempty"`   // 结束时间
	Limit     int        `json:"limit,omitempty"`      // 返回条数
}

// ListResponse 列表响应.
type ListResponse struct {
	Items interface{} `json:"items"` // 列表项
	Total int         `json:"total"` // 总数
}
