// Package aideidentification - AI隐私脱敏模块
// 可定制的 PII（个人身份信息）脱敏规则，在 AI 处理前自动脱敏
// 参考群晖 DSM 7.3 的 Privacy by design 功能
package aideidentification

import (
	"time"
)

// ============================================================
// 配置类型
// ============================================================

// DeidentificationConfig 脱敏配置
type DeidentificationConfig struct {
	// 默认脱敏策略
	DefaultPolicy string `json:"default_policy"` // "mask", "hash", "replace", "remove"

	// 脱敏占位符
	Placeholder string `json:"placeholder"` // 脱敏后的占位符，默认 "***"

	// 是否保留部分字符（如手机号保留前3后4）
	PartialMask bool `json:"partial_mask"`

	// 部分脱敏保留前缀长度
	PrefixLen int `json:"prefix_len"` // 默认 3

	// 部分脱敏保留后缀长度
	SuffixLen int `json:"suffix_len"` // 默认 4

	// 自定义替换词典
	ReplaceDict map[string]string `json:"replace_dict"` // 类型 -> 替换词

	// 启用的 PII 类型
	EnabledTypes []string `json:"enabled_types"`

	// 审计日志
	AuditLog bool `json:"audit_log"` // 是否记录审计日志
}

// DefaultDeidentificationConfig 默认脱敏配置
func DefaultDeidentificationConfig() DeidentificationConfig {
	return DeidentificationConfig{
		DefaultPolicy: "mask",
		Placeholder:   "***",
		PartialMask:   true,
		PrefixLen:     3,
		SuffixLen:     4,
		ReplaceDict: map[string]string{
			"name":    "[姓名]",
			"phone":   "[手机号]",
			"email":   "[邮箱]",
			"id_card": "[身份证]",
			"address": "[地址]",
		},
		EnabledTypes: []string{
			"name", "phone", "email", "id_card",
			"bank_card", "address", "ip", "license_plate",
		},
		AuditLog: true,
	}
}

// ============================================================
// PII 类型枚举
// ============================================================

// PIIType PII 类型
type PIIType string

const (
	PIITypeName        PIIType = "name"         // 姓名
	PIITypePhone       PIIType = "phone"        // 手机号
	PIITypeEmail       PIIType = "email"        // 邮箱
	PIITypeIDCard      PIIType = "id_card"      // 身份证号
	PIITypeBankCard    PIIType = "bank_card"    // 银行卡号
	PIITypeAddress     PIIType = "address"      // 地址
	PIITypeIP          PIIType = "ip"           // IP地址
	PIITypeLicensePlate PIIType = "license_plate" // 车牌号
	PIITypePassport    PIIType = "passport"     // 护照号
	PIITypeCreditCard  PIIType = "credit_card"  // 信用卡号
)

// ============================================================
// 脱敏策略枚举
// ============================================================

// RedactionPolicy 脱敏策略
type RedactionPolicy string

const (
	PolicyMask    RedactionPolicy = "mask"    // 遮罩：替换为占位符
	PolicyHash    RedactionPolicy = "hash"    // 哈希：替换为哈希值
	PolicyReplace RedactionPolicy = "replace" // 替换：替换为自定义文本
	PolicyRemove  RedactionPolicy = "remove"  // 移除：完全移除
	PolicyPartial RedactionPolicy = "partial" // 部分脱敏：保留部分字符
)

// ============================================================
// 规则类型
// ============================================================

// DeidentificationRule 脱敏规则
type DeidentificationRule struct {
	ID          string          `json:"id"`           // 规则ID
	Name        string          `json:"name"`         // 规则名称
	Description string          `json:"description"`  // 规则描述
	Enabled     bool            `json:"enabled"`      // 是否启用
	PIIType     PIIType         `json:"pii_type"`     // PII类型
	Policy      RedactionPolicy `json:"policy"`       // 脱敏策略
	Pattern     string          `json:"pattern"`      // 正则表达式模式
	Placeholder string          `json:"placeholder"`  // 自定义占位符
	Priority    int             `json:"priority"`     // 优先级（越大越优先）
	CreatedAt   time.Time       `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time       `json:"updated_at"`   // 更新时间
}

// PIIPattern PII 检测模式
type PIIPattern struct {
	Type        PIIType `json:"type"`        // PII类型
	Pattern     string  `json:"pattern"`     // 正则表达式
	Description string  `json:"description"` // 描述
	Examples    []string `json:"examples"`    // 示例（用于测试）
}

// ============================================================
// 脱敏结果类型
// ============================================================

// RedactionResult 单次脱敏结果
type RedactionResult struct {
	RuleID      string          `json:"rule_id"`      // 命中的规则ID
	PIIType     PIIType         `json:"pii_type"`     // PII类型
	Original    string          `json:"original"`     // 原始文本（部分）
	Redacted    string          `json:"redacted"`     // 脱敏后文本
	Policy      RedactionPolicy `json:"policy"`       // 使用的策略
	StartOffset int             `json:"start_offset"` // 起始位置
	EndOffset   int             `json:"end_offset"`   // 结束位置
}

// DeidentificationResult 整体脱敏结果
type DeidentificationResult struct {
	OriginalText  string             `json:"original_text"`  // 原始文本
	RedactedText  string             `json:"redacted_text"`  // 脱敏后文本
	Redactions    []RedactionResult  `json:"redactions"`     // 脱敏记录
	ProcessedAt   time.Time          `json:"processed_at"`   // 处理时间
	TotalRedacted int                `json:"total_redacted"` // 脱敏总数
}

// ============================================================
// 批量处理类型
// ============================================================

// BatchDeidentificationRequest 批量脱敏请求
type BatchDeidentificationRequest struct {
	Texts  []string `json:"texts"`  // 待脱敏文本列表
	RuleID string   `json:"rule_id"` // 指定规则ID（可选，为空则使用所有规则）
}

// BatchDeidentificationResult 批量脱敏结果
type BatchDeidentificationResult struct {
	Results []DeidentificationResult `json:"results"` // 脱敏结果列表
	Summary BatchSummary             `json:"summary"` // 汇总统计
}

// BatchSummary 批量处理汇总
type BatchSummary struct {
	TotalTexts     int `json:"total_texts"`     // 总文本数
	TotalRedacted  int `json:"total_redacted"`  // 总脱敏数
	AvgRedactions  float64 `json:"avg_redactions"` // 平均每文本脱敏数
}

// ============================================================
// 统计类型
// ============================================================

// DeidentificationStats 脱敏统计
type DeidentificationStats struct {
	TotalProcessed  int                `json:"total_processed"`  // 总处理次数
	TotalRedacted   int                `json:"total_redacted"`   // 总脱敏次数
	ByPIIType       map[PIIType]int    `json:"by_pii_type"`     // 按PII类型统计
	ByPolicy        map[string]int     `json:"by_policy"`       // 按策略统计
	TopRules        []RuleUsage        `json:"top_rules"`       // 热门规则
	LastProcessedAt *time.Time         `json:"last_processed_at"` // 最后处理时间
}

// RuleUsage 规则使用统计
type RuleUsage struct {
	RuleID   string `json:"rule_id"`   // 规则ID
	RuleName string `json:"rule_name"` // 规则名称
	HitCount int    `json:"hit_count"` // 命中次数
}

// ============================================================
// 审计日志类型
// ============================================================

// AuditEntry 审计日志条目
type AuditEntry struct {
	ID          string    `json:"id"`           // 日志ID
	Action      string    `json:"action"`       // 操作: "deidentify", "rule_create", "rule_update", "rule_delete"
	RuleID      string    `json:"rule_id"`      // 关联规则ID
	PIIType     PIIType   `json:"pii_type"`     // PII类型
	RedactedLen int       `json:"redacted_len"` // 脱敏文本长度
	Timestamp   time.Time `json:"timestamp"`    // 时间戳
	Source      string    `json:"source"`       // 来源
}

// ============================================================
// HTTP 请求/响应类型
// ============================================================

// CreateRuleRequest 创建规则请求
type CreateRuleRequest struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	PIIType     PIIType         `json:"pii_type" binding:"required"`
	Policy      RedactionPolicy `json:"policy" binding:"required"`
	Pattern     string          `json:"pattern" binding:"required"`
	Placeholder string          `json:"placeholder"`
	Priority    int             `json:"priority"`
}

// UpdateRuleRequest 更新规则请求
type UpdateRuleRequest struct {
	ID          string          `json:"id" binding:"required"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Enabled     *bool           `json:"enabled"`
	Policy      RedactionPolicy `json:"policy"`
	Pattern     string          `json:"pattern"`
	Placeholder string          `json:"placeholder"`
	Priority    *int            `json:"priority"`
}

// DeidentificationRequest 脱敏请求
type DeidentificationRequest struct {
	Text   string `json:"text" binding:"required"`
	RuleID string `json:"rule_id"` // 可选，指定规则
}

// DeidentificationResponse 脱敏响应
type DeidentificationResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RuleListResponse 规则列表响应
type RuleListResponse struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    []DeidentificationRule `json:"data,omitempty"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	Code    int                  `json:"code"`
	Message string               `json:"message"`
	Data    *DeidentificationStats `json:"data,omitempty"`
}

// AuditLogResponse 审计日志响应
type AuditLogResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    []AuditEntry `json:"data,omitempty"`
}
