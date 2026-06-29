// Package privacyproxy 提供 AI 数据脱敏代理服务。
// 参考 DSM 7.3 Customized AI de-identification 设计理念：
// Privacy by design — 在数据发送至第三方 AI 之前，于本地完成敏感信息脱敏，
// 确保隐私不外泄。支持自定义脱敏规则、审计日志与合规报告。
package privacyproxy

import (
	"time"
)

// =========================================================================
// 一、脱敏规则数据模型
// =========================================================================

// RuleType 规则类型：内置或自定义
type RuleType string

const (
	RuleTypeBuiltin  RuleType = "builtin"  // 内置规则
	RuleTypeCustom   RuleType = "custom"   // 自定义规则
)

// MaskAction 脱敏动作类型
type MaskAction string

const (
	ActionMask     MaskAction = "mask"     // 掩码替换（保留首尾，中间用 * 代替）
	ActionReplace  MaskAction = "replace"  // 整体替换为固定文本
	ActionHash     MaskAction = "hash"     // SHA-256 哈希替换
	ActionRedact   MaskAction = "redact"   // 完全删除（替换为 [REDACTED]）
)

// MaskRule 单条脱敏规则
type MaskRule struct {
	ID          string    `json:"id"`                     // 规则唯一标识
	Name        string    `json:"name"`                   // 规则名称（中文）
	Type        RuleType  `json:"type"`                   // 规则类型
	Pattern     string    `json:"pattern"`                // 正则表达式（原始字符串）
	Action      MaskAction `json:"action"`                // 脱敏动作
	Replacement string    `json:"replacement,omitempty"` // 替换文本（Action=replace 时生效）
	KeepPrefix  int       `json:"keep_prefix,omitempty"` // 保留前缀字符数（Action=mask 时生效）
	KeepSuffix  int       `json:"keep_suffix,omitempty"` // 保留后缀字符数（Action=mask 时生效）
	Enabled     bool      `json:"enabled"`                // 是否启用
	Priority    int       `json:"priority,omitempty"`    // 优先级（数值越小越先匹配）
	Description string    `json:"description,omitempty"` // 规则描述
	CreatedAt   time.Time `json:"created_at"`             // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`             // 更新时间
}

// MaskConfig 全局脱敏配置
type MaskConfig struct {
	Enabled          bool   `json:"enabled"`                     // 全局开关
	ListenAddr       string `json:"listen_addr"`                 // 代理监听地址，如 127.0.0.1:8420
	DefaultAction    MaskAction `json:"default_action"`         // 默认脱敏动作
	MaxBodyBytes     int64  `json:"max_body_bytes"`              // 请求体最大字节数
	StreamTimeout    time.Duration `json:"stream_timeout"`       // 流式响应超时
	AuditEnabled     bool   `json:"audit_enabled"`               // 审计日志开关
	AuditMaxEntries  int    `json:"audit_max_entries"`           // 审计日志最大条数（环形缓冲）
	BlockedDomains   []string `json:"blocked_domains,omitempty"` // 禁止转发的域名列表
	AllowedDomains   []string `json:"allowed_domains,omitempty"` // 仅允许转发的域名列表（白名单）
}

// DefaultMaskConfig 返回默认配置
func DefaultMaskConfig() *MaskConfig {
	return &MaskConfig{
		Enabled:         true,
		ListenAddr:      "127.0.0.1:8420",
		DefaultAction:   ActionMask,
		MaxBodyBytes:    10 * 1024 * 1024, // 10 MB
		StreamTimeout:   120 * time.Second,
		AuditEnabled:    true,
		AuditMaxEntries: 10000,
		AllowedDomains: []string{
			"api.openai.com",
			"api.anthropic.com",
			"generativelanguage.googleapis.com",
			"dashscope.aliyuncs.com",
			"api.deepseek.com",
			"api.siliconflow.cn",
		},
	}
}

// =========================================================================
// 二、审计日志数据模型
// =========================================================================

// AuditEntry 单条审计日志
type AuditEntry struct {
	ID           string    `json:"id"`             // 日志唯一标识
	Timestamp    time.Time `json:"timestamp"`      // 操作时间
	RuleID       string    `json:"rule_id"`        // 命中规则 ID
	RuleName     string    `json:"rule_name"`      // 命中规则名称
	Original     string    `json:"original"`       // 原始敏感文本（截断至 200 字符）
	Masked       string    `json:"masked"`         // 脱敏后文本（截断至 200 字符）
	TargetAPI    string    `json:"target_api"`     // 目标 AI API 地址
	MatchCount   int       `json:"match_count"`    // 本次请求命中次数
	RequestMethod string   `json:"request_method"` // HTTP 方法
	RequestPath  string    `json:"request_path"`   // 请求路径
	Success      bool      `json:"success"`        // 脱敏是否成功
	Error        string    `json:"error,omitempty"` // 错误信息
}

// AuditQuery 审计日志查询条件
type AuditQuery struct {
	StartTime  *time.Time `json:"start_time,omitempty"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	RuleID     string     `json:"rule_id,omitempty"`
	TargetAPI  string     `json:"target_api,omitempty"`
	Success    *bool      `json:"success,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	GeneratedAt      time.Time         `json:"generated_at"`
	PeriodStart      time.Time         `json:"period_start"`
	PeriodEnd        time.Time         `json:"period_end"`
	TotalRequests    int               `json:"total_requests"`
	TotalMasked      int               `json:"total_masked"`      // 总脱敏次数
	UniqueRulesUsed  int               `json:"unique_rules_used"` // 使用的规则数
	TopRules         []RuleUsageStat   `json:"top_rules"`         // 规则使用排行
	TopTargetAPIs    []APIUsageStat    `json:"top_target_apis"`   // 目标 API 排行
	MaskedExamples   []AuditEntry      `json:"masked_examples"`   // 脱敏示例（最多 20 条）
	ByAction         map[MaskAction]int `json:"by_action"`        // 按脱敏动作统计
}

// RuleUsageStat 规则使用统计
type RuleUsageStat struct {
	RuleID   string `json:"rule_id"`
	RuleName string `json:"rule_name"`
	Count    int    `json:"count"`
}

// APIUsageStat 目标 API 使用统计
type APIUsageStat struct {
	TargetAPI string `json:"target_api"`
	Count     int    `json:"count"`
}
