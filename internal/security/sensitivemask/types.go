// Package sensitivemask provides sensitive information detection and masking capabilities.
// Inspired by Synology DSM 7.3 AI Console's data protection mechanism.
// This package ensures sensitive data is not leaked to cloud AI services.
package sensitivemask

import (
	"time"
)

// SensitiveType defines the type of sensitive information.
type SensitiveType string

const (
	// TypePhoneNumber 中国手机号.
	TypePhoneNumber SensitiveType = "phone_number"
	// TypeIDCard 中国身份证号.
	TypeIDCard SensitiveType = "id_card"
	// TypeBankCard 银行卡号.
	TypeBankCard SensitiveType = "bank_card"
	// TypeEmail 邮箱地址.
	TypeEmail SensitiveType = "email"
	// TypeAddress 地址信息.
	TypeAddress SensitiveType = "address"
	// TypePassport 护照号码.
	TypePassport SensitiveType = "passport"
	// TypeCreditCard 信用卡号.
	TypeCreditCard SensitiveType = "credit_card"
	// TypeSSN 社会安全号（国际）.
	TypeSSN SensitiveType = "ssn"
	// TypeAPIKey API密钥.
	TypeAPIKey SensitiveType = "api_key"
	// TypePassword 密码相关.
	TypePassword SensitiveType = "password"
	// TypeIPv4 IPv4地址.
	TypeIPv4 SensitiveType = "ipv4"
	// TypeCustom 自定义类型.
	TypeCustom SensitiveType = "custom"
)

// RiskLevel defines the risk level of sensitive information.
type RiskLevel int

// 风险等级常量.
const (
	// RiskLevelLow 低风险：可部分显示.
	RiskLevelLow      RiskLevel = iota // 低风险：可部分显示
	RiskLevelMedium                    // 中风险：需完全脱敏
	RiskLevelHigh                      // 高风险：必须阻止传输
	RiskLevelCritical                  // 严重：立即告警
)

// SensitiveMatch represents a detected sensitive information match.
type SensitiveMatch struct {
	Type        SensitiveType `json:"type"`         // 敏感信息类型
	Value       string        `json:"value"`        // 原始值
	MaskedValue string        `json:"masked_value"` // 脱敏后的值
	StartPos    int           `json:"start_pos"`    // 起始位置
	EndPos      int           `json:"end_pos"`      // 结束位置
	RiskLevel   RiskLevel     `json:"risk_level"`   // 风险等级
	Confidence  float64       `json:"confidence"`   // 置信度 0-1
	Context     string        `json:"context"`      // 上下文（前后各若干字符）
}

// MaskStrategy defines masking strategy.
type MaskStrategy int

// 脱敏策略常量.
const (
	// MaskStrategyNone 不脱敏.
	MaskStrategyNone    MaskStrategy = iota // 不脱敏
	MaskStrategyPartial                     // 部分脱敏（保留部分字符）
	MaskStrategyFull                        // 完全脱敏（全部替换）
	MaskStrategyHash                        // 哈希脱敏（保留可验证性）
	MaskStrategyRemove                      // 移除
)

// DetectorConfig defines configuration for sensitive information detector.
type DetectorConfig struct {
	EnabledTypes   map[SensitiveType]bool `json:"enabled_types"`   // 启用的检测类型
	MinConfidence  float64                `json:"min_confidence"`  // 最小置信度阈值
	ContextLength  int                    `json:"context_length"`  // 上下文长度
	StrictMode     bool                   `json:"strict_mode"`     // 严格模式（更严格的正则）
	CheckChecksum  bool                   `json:"check_checksum"`  // 是否校验校验和（如身份证）
	EnableAI       bool                   `json:"enable_ai"`       // 启用AI辅助检测
	CustomPatterns []CustomPattern        `json:"custom_patterns"` // 自定义模式
}

// CustomPattern defines a custom detection pattern.
type CustomPattern struct {
	Name        string        `json:"name"`
	Pattern     string        `json:"pattern"`
	Type        SensitiveType `json:"type"`
	RiskLevel   RiskLevel     `json:"risk_level"`
	Description string        `json:"description"`
}

// MaskerConfig defines configuration for masker.
type MaskerConfig struct {
	Strategies     map[SensitiveType]MaskStrategy `json:"strategies"`       // 各类型的脱敏策略
	DefaultMask    string                         `json:"default_mask"`     // 默认脱敏字符
	PartialKeepLen map[SensitiveType]int          `json:"partial_keep_len"` // 部分脱敏保留长度
	HashSalt       string                         `json:"hash_salt"`        // 哈希脱敏盐值
}

// DetectionResult represents the result of a detection operation.
type DetectionResult struct {
	Matches        []SensitiveMatch `json:"matches"`
	TotalCount     int              `json:"total_count"`
	HighRiskCount  int              `json:"high_risk_count"`
	HasSensitive   bool             `json:"has_sensitive"`
	ProcessedText  string           `json:"processed_text"` // 脱敏后的文本
	ProcessingTime time.Duration    `json:"processing_time"`
}

// Policy defines a data protection policy.
type Policy struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Detector    DetectorConfig `json:"detector"`
	Masker      MaskerConfig   `json:"masker"`
	Actions     PolicyActions  `json:"actions"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// PolicyActions defines actions to take when sensitive data is detected.
type PolicyActions struct {
	BlockTransmission bool       `json:"block_transmission"` // 阻止传输
	MaskBeforeSend    bool       `json:"mask_before_send"`   // 发送前脱敏
	LogDetection      bool       `json:"log_detection"`      // 记录检测日志
	AlertOnHighRisk   bool       `json:"alert_on_high_risk"` // 高风险告警
	NotifyAdmin       bool       `json:"notify_admin"`       // 通知管理员
	AuditLevel        AuditLevel `json:"audit_level"`        // 审计级别
}

// AuditLevel defines the level of audit logging.
type AuditLevel int

// 审计级别常量.
const (
	// AuditLevelNone 无审计.
	AuditLevelNone     AuditLevel = iota
	AuditLevelBasic               // 基本信息
	AuditLevelDetailed            // 详细信息
	AuditLevelFull                // 完整信息（包含敏感数据脱敏后的内容）
)

// AuditLog represents an audit log entry.
type AuditLog struct {
	ID          string           `json:"id"`
	Timestamp   time.Time        `json:"timestamp"`
	SourceType  string           `json:"source_type"` // 来源类型（api/file/stream）
	SourceID    string           `json:"source_id"`   // 来源标识
	Detections  []SensitiveMatch `json:"detections"`
	Action      string           `json:"action"` // taken action
	UserID      string           `json:"user_id"`
	ServiceName string           `json:"service_name"` // 目标服务（如云AI服务）
	PolicyID    string           `json:"policy_id"`
	Blocked     bool             `json:"blocked"`
}

// ServiceConfig defines configuration for a cloud AI service.
type ServiceConfig struct {
	Name           string   `json:"name"`            // 服务名称
	Endpoint       string   `json:"endpoint"`        // 服务端点
	DataTypes      []string `json:"data_types"`      // 处理的数据类型
	PolicyID       string   `json:"policy_id"`       // 应用的策略ID
	Enabled        bool     `json:"enabled"`         // 是否启用保护
	BypassInternal bool     `json:"bypass_internal"` // 内部网络绕过
}

// Default configurations.
var (
	DefaultDetectorConfig = DetectorConfig{
		EnabledTypes: map[SensitiveType]bool{
			TypePhoneNumber: true,
			TypeIDCard:      true,
			TypeBankCard:    true,
			TypeEmail:       true,
			TypePassport:    true,
			TypeCreditCard:  true,
			TypeAPIKey:      true,
			TypeIPv4:        false, // 默认不检测IP，根据场景开启
		},
		MinConfidence: 0.8,
		ContextLength: 20,
		StrictMode:    false,
		CheckChecksum: true,
		EnableAI:      false,
	}

	DefaultMaskerConfig = MaskerConfig{
		Strategies: map[SensitiveType]MaskStrategy{
			TypePhoneNumber: MaskStrategyPartial,
			TypeIDCard:      MaskStrategyPartial,
			TypeBankCard:    MaskStrategyFull,
			TypeEmail:       MaskStrategyPartial,
			TypePassport:    MaskStrategyFull,
			TypeCreditCard:  MaskStrategyFull,
			TypeAPIKey:      MaskStrategyRemove,
			TypePassword:    MaskStrategyRemove,
			TypeIPv4:        MaskStrategyPartial,
		},
		DefaultMask: "*",
		PartialKeepLen: map[SensitiveType]int{
			TypePhoneNumber: 3, // 保留后3位
			TypeIDCard:      4, // 保留后4位
			TypeEmail:       3, // 用户名保留前3字符
			TypeIPv4:        1, // 保留最后一段
		},
	}
)
