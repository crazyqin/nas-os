// Package aidatamasking 提供 AI 数据脱敏引擎功能，支持敏感数据检测与脱敏处理。
// 支持身份证号、手机号、银行卡号、邮箱、IP地址等中国常见敏感信息的检测与脱敏。
package aidatamasking

import "time"

// SensitiveDataType 敏感数据类型
type SensitiveDataType string

const (
	DataTypeIDCard     SensitiveDataType = "id_card"      // 身份证号
	DataTypePhone      SensitiveDataType = "phone"        // 手机号
	DataTypeBankCard   SensitiveDataType = "bank_card"    // 银行卡号
	DataTypeEmail      SensitiveDataType = "email"        // 邮箱
	DataTypeIPAddress  SensitiveDataType = "ip_address"   // IP地址
	DataTypeName       SensitiveDataType = "name"         // 姓名
	DataTypeAddress    SensitiveDataType = "address"      // 地址
	DataTypeLicensePlate SensitiveDataType = "license_plate" // 车牌号
	DataTypePassport   SensitiveDataType = "passport"     // 护照号
	DataTypeSSN        SensitiveDataType = "ssn"          // 社会保障号
)

// MaskingStrategy 脱敏策略
type MaskingStrategy string

const (
	StrategyMask    MaskingStrategy = "mask"    // 掩码：用*替换部分字符
	StrategyReplace MaskingStrategy = "replace" // 替换：用固定文本替换
	StrategyHash    MaskingStrategy = "hash"    // 哈希：用哈希值替换
	StrategyTruncate MaskingStrategy = "truncate" // 截断：截取部分字符
	StrategyRedact  MaskingStrategy = "redact"  // 删除：完全移除
)

// MaskingRule 脱敏规则
type MaskingRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DataType    SensitiveDataType `json:"data_type"`
	Strategy    MaskingStrategy   `json:"strategy"`
	Enabled     bool              `json:"enabled"`
	Pattern     string            `json:"pattern,omitempty"`     // 自定义正则表达式
	Replacement string            `json:"replacement,omitempty"` // 替换文本
	KeepPrefix  int               `json:"keep_prefix,omitempty"` // 保留前缀字符数
	KeepSuffix  int               `json:"keep_suffix,omitempty"` // 保留后缀字符数
	MaskChar    string            `json:"mask_char,omitempty"`   // 掩码字符，默认*
	Description string            `json:"description,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// MaskingResult 脱敏结果
type MaskingResult struct {
	Original  string            `json:"original"`
	Masked    string            `json:"masked"`
	DataType  SensitiveDataType `json:"data_type"`
	Strategy  MaskingStrategy   `json:"strategy"`
	StartPos  int               `json:"start_pos"`
	EndPos    int               `json:"end_pos"`
	RuleID    string            `json:"rule_id"`
}

// MaskingRequest 脱敏请求
type MaskingRequest struct {
	Text    string          `json:"text" binding:"required"`
	Rules   []*MaskingRule  `json:"rules,omitempty"`    // 自定义规则，为空则使用默认规则
	TestMode bool           `json:"test_mode,omitempty"` // 测试模式，返回对比结果
}

// MaskingResponse 脱敏响应
type MaskingResponse struct {
	MaskedText string            `json:"masked_text"`
	Results    []*MaskingResult  `json:"results,omitempty"`   // 测试模式下返回详细结果
	Summary    *MaskingSummary   `json:"summary,omitempty"`   // 脱敏摘要
	CreatedAt  time.Time         `json:"created_at"`
	Duration   time.Duration     `json:"duration"`
}

// MaskingSummary 脱敏摘要
type MaskingSummary struct {
	TotalMatches int                       `json:"total_matches"`
	ByType       map[SensitiveDataType]int `json:"by_type"`
	ByStrategy   map[MaskingStrategy]int   `json:"by_strategy"`
}

// BatchMaskingRequest 批量脱敏请求
type BatchMaskingRequest struct {
	Texts    []string        `json:"texts" binding:"required,min=1"`
	Rules    []*MaskingRule  `json:"rules,omitempty"`
	TestMode bool            `json:"test_mode,omitempty"`
}

// BatchMaskingResponse 批量脱敏响应
type BatchMaskingResponse struct {
	Results   []*MaskingResponse `json:"results"`
	TotalTexts int               `json:"total_texts"`
	TotalMasked int              `json:"total_masked"`
	Duration   time.Duration      `json:"duration"`
}

// MaskingLog 脱敏日志
type MaskingLog struct {
	ID        string            `json:"id"`
	DataType  SensitiveDataType `json:"data_type"`
	Strategy  MaskingStrategy   `json:"strategy"`
	RuleID    string            `json:"rule_id"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`    // mask, unmask, rule_create, rule_update, rule_delete
	UserID    string    `json:"user_id,omitempty"`
	Details   string    `json:"details,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// AIIntegrationConfig AI集成配置
type AIIntegrationConfig struct {
	Enabled        bool   `json:"enabled"`
	PreProcess     bool   `json:"pre_process"`     // 在发送给AI前脱敏
	PostProcess    bool   `json:"post_process"`    // 在AI响应后脱敏
	LogPrompts     bool   `json:"log_prompts"`     // 记录提示词
	MaxPromptLength int   `json:"max_prompt_length"`
}

// AIPromptRequest AI提示词请求
type AIPromptRequest struct {
	Prompt    string              `json:"prompt" binding:"required"`
	Config    *AIIntegrationConfig `json:"config,omitempty"`
	SessionID string              `json:"session_id,omitempty"`
}

// AIPromptResponse AI提示词响应
type AIPromptResponse struct {
	OriginalPrompt  string `json:"original_prompt"`
	MaskedPrompt    string `json:"masked_prompt"`
	HasSensitiveData bool  `json:"has_sensitive_data"`
	MaskingApplied  bool   `json:"masking_applied"`
}

// MaskingEngineConfig 脱敏引擎配置
type MaskingEngineConfig struct {
	Enabled         bool              `json:"enabled"`
	DefaultStrategy MaskingStrategy   `json:"default_strategy"`
	MaxTextLength   int               `json:"max_text_length"`
	CacheEnabled    bool              `json:"cache_enabled"`
	CacheTTLMinutes int               `json:"cache_ttl_minutes"`
	LogEnabled      bool              `json:"log_enabled"`
	AuditEnabled    bool              `json:"audit_enabled"`
	AIIntegration   *AIIntegrationConfig `json:"ai_integration,omitempty"`
}

// DefaultMaskingEngineConfig 默认引擎配置
func DefaultMaskingEngineConfig() *MaskingEngineConfig {
	return &MaskingEngineConfig{
		Enabled:         true,
		DefaultStrategy: StrategyMask,
		MaxTextLength:   1024 * 1024, // 1MB
		CacheEnabled:    false,
		CacheTTLMinutes: 30,
		LogEnabled:      true,
		AuditEnabled:    true,
		AIIntegration: &AIIntegrationConfig{
			Enabled:         true,
			PreProcess:      true,
			PostProcess:     false,
			LogPrompts:      true,
			MaxPromptLength: 10000,
		},
	}
}

// ValidDataTypes 有效的数据类型列表
func ValidDataTypes() []SensitiveDataType {
	return []SensitiveDataType{
		DataTypeIDCard,
		DataTypePhone,
		DataTypeBankCard,
		DataTypeEmail,
		DataTypeIPAddress,
		DataTypeName,
		DataTypeAddress,
		DataTypeLicensePlate,
		DataTypePassport,
		DataTypeSSN,
	}
}

// ValidStrategies 有效的脱敏策略列表
func ValidStrategies() []MaskingStrategy {
	return []MaskingStrategy{
		StrategyMask,
		StrategyReplace,
		StrategyHash,
		StrategyTruncate,
		StrategyRedact,
	}
}

// IsValidDataType 检查数据类型是否有效
func IsValidDataType(dt SensitiveDataType) bool {
	for _, v := range ValidDataTypes() {
		if v == dt {
			return true
		}
	}
	return false
}

// IsValidStrategy 检查脱敏策略是否有效
func IsValidStrategy(s MaskingStrategy) bool {
	for _, v := range ValidStrategies() {
		if v == s {
			return true
		}
	}
	return false
}
