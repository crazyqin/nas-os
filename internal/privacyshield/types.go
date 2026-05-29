package privacyshield

import (
	"sync"
	"time"
)

// SensitivePattern 定义敏感数据的匹配模式
type SensitivePattern struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Category    string `json:"category"`
	Severity    int    `json:"severity"` // 1-10, 10为最高风险
	Description string `json:"description"`
}

// MaskRule 定义脱敏规则
type MaskRule struct {
	Name       string `json:"name"`
	Strategy   string `json:"strategy"` // mask, partial, hash, redact
	PrefixKeep int    `json:"prefix_keep"`
	SuffixKeep int    `json:"suffix_keep"`
	MaskChar   string `json:"mask_char"`
}

// SensitiveMatch 敏感数据匹配结果
type SensitiveMatch struct {
	Pattern   SensitivePattern `json:"pattern"`
	Value     string           `json:"value"`
	Start     int              `json:"start"`
	End       int              `json:"end"`
	LineNum   int              `json:"line_num"`
	RiskLevel int              `json:"risk_level"`
}

// ScanResult 文件扫描结果
type ScanResult struct {
	FilePath     string           `json:"file_path"`
	Matches      []SensitiveMatch `json:"matches"`
	TotalMatches int              `json:"total_matches"`
	Categories   map[string]int   `json:"categories"`
	RiskScore    float64          `json:"risk_score"`
	ScannedAt    time.Time        `json:"scanned_at"`
}

// ComplianceReport 合规检查报告
type ComplianceReport struct {
	ID                string            `json:"id"`
	GeneratedAt       time.Time         `json:"generated_at"`
	Framework         string            `json:"framework"` // GDPR, PIPL, etc.
	TotalItems        int               `json:"total_items"`
	CompliantItems    int               `json:"compliant_items"`
	NonCompliantItems int               `json:"non_compliant_items"`
	Issues            []ComplianceIssue `json:"issues"`
	Score             float64           `json:"score"`
	Status            string            `json:"status"` // compliant, warning, non-compliant
}

// ComplianceIssue 合规问题
type ComplianceIssue struct {
	Type        string   `json:"type"`
	Severity    string   `json:"severity"` // critical, high, medium, low
	Description string   `json:"description"`
	Items       []string `json:"items"`
	Remediation string   `json:"remediation"`
}

// RiskScore 风险评估
type RiskScore struct {
	Overall     float64            `json:"overall"`
	Density     float64            `json:"density"`      // 敏感数据密度
	AccessScope float64            `json:"access_scope"` // 访问范围
	Encrypted   float64            `json:"encrypted"`    // 加密状态
	Breakdown   map[string]float64 `json:"breakdown"`
	RiskLevel   string             `json:"risk_level"` // critical, high, medium, low
	AssessedAt  time.Time          `json:"assessed_at"`
}

// Shield 隐私保护盾核心结构
type Shield struct {
	mu       sync.RWMutex
	patterns []SensitivePattern
	rules    map[string]MaskRule
}

// ScanRequest 扫描请求
type ScanRequest struct {
	Content    string   `json:"content"`
	FilePath   string   `json:"file_path"`
	Categories []string `json:"categories,omitempty"`
}

// MaskRequest 脱敏请求
type MaskRequest struct {
	Content  string      `json:"content"`
	Strategy string      `json:"strategy"` // mask, partial, hash, redact
	Options  *MaskOptions `json:"options,omitempty"`
}

// MaskOptions 脱敏选项
type MaskOptions struct {
	PrefixKeep int    `json:"prefix_keep"`
	SuffixKeep int    `json:"suffix_keep"`
	MaskChar   string `json:"mask_char"`
}

// MaskResponse 脱敏响应
type MaskResponse struct {
	Original   string `json:"original"`
	Masked     string `json:"masked"`
	Strategy   string `json:"strategy"`
	MatchCount int    `json:"match_count"`
}

// ComplianceRequest 合规检查请求
type ComplianceRequest struct {
	Framework string `json:"framework"` // GDPR, PIPL, ALL
	Content   string `json:"content"`
	FilePath  string `json:"file_path,omitempty"`
}

// RiskAssessmentRequest 风险评估请求
type RiskAssessmentRequest struct {
	Content     string `json:"content"`
	Encrypted   bool   `json:"encrypted"`
	AccessLevel string `json:"access_level"` // public, internal, private, restricted
}

// DefaultPatterns 返回预定义的中国常用敏感数据模式
func DefaultPatterns() []SensitivePattern {
	return []SensitivePattern{
		{
			Name:        "手机号码",
			Pattern:     `1[3-9]\d{9}`,
			Category:    "phone",
			Severity:    7,
			Description: "中国大陆手机号码",
		},
		{
			Name:        "身份证号码",
			Pattern:     `[1-9]\d{5}(19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`,
			Category:    "id_card",
			Severity:    10,
			Description: "18位居民身份证号码",
		},
		{
			Name:        "电子邮箱",
			Pattern:     `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`,
			Category:    "email",
			Severity:    5,
			Description: "电子邮箱地址",
		},
		{
			Name:        "银行卡号",
			Pattern:     `[1-9]\d{12,18}`,
			Category:    "bank_card",
			Severity:    9,
			Description: "银行卡号（13-19位）",
		},
		{
			Name:        "IP地址",
			Pattern:     `\b((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)\b`,
			Category:    "ip_address",
			Severity:    3,
			Description: "IPv4地址",
		},
		{
			Name:        "护照号码",
			Pattern:     `[EeGg]\d{8}`,
			Category:    "passport",
			Severity:    8,
			Description: "中国护照号码",
		},
		{
			Name:        "社保卡号",
			Pattern:     `\d{9}`,
			Category:    "social_security",
			Severity:    6,
			Description: "社保卡号（9位数字）",
		},
	}
}

// DefaultMaskRules 返回默认脱敏规则
func DefaultMaskRules() map[string]MaskRule {
	return map[string]MaskRule{
		"phone": {
			Name:       "手机号脱敏",
			Strategy:   "partial",
			PrefixKeep: 3,
			SuffixKeep: 4,
			MaskChar:   "*",
		},
		"id_card": {
			Name:       "身份证脱敏",
			Strategy:   "partial",
			PrefixKeep: 3,
			SuffixKeep: 4,
			MaskChar:   "*",
		},
		"email": {
			Name:       "邮箱脱敏",
			Strategy:   "partial",
			PrefixKeep: 2,
			SuffixKeep: 0,
			MaskChar:   "*",
		},
		"bank_card": {
			Name:       "银行卡脱敏",
			Strategy:   "partial",
			PrefixKeep: 4,
			SuffixKeep: 4,
			MaskChar:   "*",
		},
		"ip_address": {
			Name:       "IP脱敏",
			Strategy:   "mask",
			PrefixKeep: 0,
			SuffixKeep: 0,
			MaskChar:   "*",
		},
		"passport": {
			Name:       "护照脱敏",
			Strategy:   "partial",
			PrefixKeep: 1,
			SuffixKeep: 3,
			MaskChar:   "*",
		},
		"social_security": {
			Name:       "社保卡脱敏",
			Strategy:   "partial",
			PrefixKeep: 2,
			SuffixKeep: 3,
			MaskChar:   "*",
		},
	}
}
