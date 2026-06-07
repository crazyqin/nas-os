// Package ai_privacy_guardian AI隐私卫士 - AI推理时的隐私保护
// 自动识别和分类敏感数据、实时数据脱敏、隐私合规检查（GDPR/个人信息保护法）、
// 数据流向追踪、隐私风险评分和告警。
// 对标群晖 AI Console 隐私保护功能。
package ai_privacy_guardian

import (
	"errors"
	"sync"
	"time"
)

// SensitiveCategory 敏感数据分类
type SensitiveCategory string

const (
	CategoryIDCard      SensitiveCategory = "id_card"       // 身份证
	CategoryBankCard    SensitiveCategory = "bank_card"      // 银行卡
	CategoryPhone       SensitiveCategory = "phone"          // 手机号
	CategoryEmail       SensitiveCategory = "email"          // 邮箱
	CategoryPassport    SensitiveCategory = "passport"       // 护照
	CategorySSN         SensitiveCategory = "ssn"            // 社保号
	CategoryLicensePlate SensitiveCategory = "license_plate" // 车牌号
	CategoryAddress     SensitiveCategory = "address"        // 地址
	CategoryName        SensitiveCategory = "name"           // 姓名
	CategoryMedical     SensitiveCategory = "medical"        // 医疗信息
	CategoryBiometric   SensitiveCategory = "biometric"      // 生物特征
	CategoryIPAddress   SensitiveCategory = "ip_address"     // IP地址
)

// MaskStrategy 脱敏策略
type MaskStrategy string

const (
	StrategyPartial MaskStrategy = "partial" // 部分脱敏
	StrategyFull    MaskStrategy = "full"    // 完全脱敏
	StrategyHash    MaskStrategy = "hash"    // 哈希脱敏
	StrategyRedact  MaskStrategy = "redact"  // 删除脱敏
	StrategyToken   MaskStrategy = "token"   // Token化
)

// ComplianceFramework 合规框架
type ComplianceFramework string

const (
	FrameworkGDPR  ComplianceFramework = "gdpr"  // 欧盟通用数据保护条例
	FrameworkPIPL  ComplianceFramework = "pipl"  // 中国个人信息保护法
	FrameworkCCPA  ComplianceFramework = "ccpa"  // 加州消费者隐私法
	FrameworkHIPAA ComplianceFramework = "hipaa" // 健康保险可携性与责任法案
)

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// DataFlowDirection 数据流向
type DataFlowDirection string

const (
	FlowInbound  DataFlowDirection = "inbound"  // 流入AI系统
	FlowOutbound DataFlowDirection = "outbound" // 从AI系统流出
	FlowInternal DataFlowDirection = "internal" // AI系统内部流转
)

// SensitivePattern 敏感数据模式定义
type SensitivePattern struct {
	Name        string            `json:"name"`
	Pattern     string            `json:"pattern"` // 正则表达式
	Category    SensitiveCategory `json:"category"`
	Severity    int               `json:"severity"` // 1-10，10为最高
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
}

// DetectedItem 检测到的敏感数据项
type DetectedItem struct {
	ID       string            `json:"id"`
	Value    string            `json:"value"`    // 原始值
	Masked   string            `json:"masked"`   // 脱敏后
	Category SensitiveCategory `json:"category"`
	Start    int               `json:"start"`
	End      int               `json:"end"`
	Severity int               `json:"severity"`
	LineNum  int               `json:"line_num"`
}

// DetectionResult 检测结果
type DetectionResult struct {
	Items      []DetectedItem       `json:"items"`
	TotalCount int                  `json:"total_count"`
	ByCategory map[SensitiveCategory]int `json:"by_category"`
	ScannedAt  time.Time            `json:"scanned_at"`
}

// MaskRule 脱敏规则
type MaskRule struct {
	Category    SensitiveCategory `json:"category"`
	Strategy    MaskStrategy      `json:"strategy"`
	PrefixKeep  int               `json:"prefix_keep"`  // 保留前N位
	SuffixKeep  int               `json:"suffix_keep"`  // 保留后N位
	MaskChar    string            `json:"mask_char"`    // 掩码字符
	Replacement string            `json:"replacement"`  // 固定替换文本（redact/token策略用）
	Enabled     bool              `json:"enabled"`
}

// MaskResult 脱敏结果
type MaskResult struct {
	Original    string `json:"original"`
	Masked      string `json:"masked"`
	ItemsFound  int    `json:"items_found"`
	ItemsMasked int    `json:"items_masked"`
}

// ComplianceCheckResult 合规检查结果
type ComplianceCheckResult struct {
	Framework   ComplianceFramework `json:"framework"`
	Score       float64             `json:"score"`        // 0-100
	Status      string              `json:"status"`       // compliant, warning, non-compliant
	Issues      []ComplianceIssue   `json:"issues"`
	CheckedAt   time.Time           `json:"checked_at"`
	TotalItems  int                 `json:"total_items"`
	PassedItems int                 `json:"passed_items"`
	FailedItems int                 `json:"failed_items"`
}

// ComplianceIssue 合规问题
type ComplianceIssue struct {
	Type        string   `json:"type"`
	Severity    string   `json:"severity"` // critical, high, medium, low
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Remediation string   `json:"remediation"`
	Items       []string `json:"items,omitempty"`
}

// DataFlowRecord 数据流向记录
type DataFlowRecord struct {
	ID            string            `json:"id"`
	Source        string            `json:"source"`
	Destination   string            `json:"destination"`
	Direction     DataFlowDirection `json:"direction"`
	Categories    []SensitiveCategory `json:"categories"`
	ItemCount     int               `json:"item_count"`
	Masked        bool              `json:"masked"`
	AISessionID   string            `json:"ai_session_id"`
	Timestamp     time.Time         `json:"timestamp"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// RiskAssessment 风险评估
type RiskAssessment struct {
	OverallScore    float64              `json:"overall_score"`    // 0-100
	RiskLevel       RiskLevel            `json:"risk_level"`
	Factors         map[string]float64   `json:"factors"`          // 各因素得分
	Recommendations []string             `json:"recommendations"`
	AssessedAt      time.Time            `json:"assessed_at"`
}

// GuardianConfig 配置
type GuardianConfig struct {
	AutoMask       bool                    `json:"auto_mask"`       // 自动脱敏
	StrictMode     bool                    `json:"strict_mode"`     // 严格模式（不允许未脱敏数据进入AI）
	MaxItemsPerReq int                     `json:"max_items_per_req"`
	AuditEnabled   bool                    `json:"audit_enabled"`
	Frameworks     []ComplianceFramework   `json:"frameworks"`      // 启用的合规框架
}

// DefaultConfig 默认配置
func DefaultConfig() GuardianConfig {
	return GuardianConfig{
		AutoMask:       true,
		StrictMode:     false,
		MaxItemsPerReq: 1000,
		AuditEnabled:   true,
		Frameworks:     []ComplianceFramework{FrameworkGDPR, FrameworkPIPL},
	}
}

// Guardian AI隐私卫士核心引擎
type Guardian struct {
	mu          sync.RWMutex
	config      GuardianConfig
	detector    *Detector
	masker      *Masker
	compliance  *ComplianceChecker
	tracker     *Tracker
	scorer      *Scorer
	flowRecords []DataFlowRecord
	auditLog    []AuditEntry
	maxRecords  int
}

// AuditEntry 审计日志条目
type AuditEntry struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	SessionID string    `json:"session_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// 预定义错误
var (
	ErrContentEmpty      = errors.New("content cannot be empty")
	ErrPatternInvalid    = errors.New("invalid pattern")
	ErrMaxItemsReached   = errors.New("maximum items per request reached")
	ErrStrictModeReject  = errors.New("strict mode: unmasked sensitive data rejected")
	ErrFrameworkUnknown  = errors.New("unknown compliance framework")
	ErrRuleNotFound      = errors.New("mask rule not found for category")
)
