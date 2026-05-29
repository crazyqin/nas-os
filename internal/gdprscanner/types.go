// Package gdprscanner 提供隐私合规扫描功能
package gdprscanner

import (
	"time"
)

// SensitivityLevel 敏感度级别.
type SensitivityLevel int

const (
	SensitivityLow      SensitivityLevel = iota // 低敏感（邮箱等）
	SensitivityMedium                            // 中敏感（手机号等）
	SensitivityHigh                              // 高敏感（身份证号、银行卡号等）
)

// String 返回敏感度级别的中文描述.
func (l SensitivityLevel) String() string {
	switch l {
	case SensitivityLow:
		return "低"
	case SensitivityMedium:
		return "中"
	case SensitivityHigh:
		return "高"
	default:
		return "未知"
	}
}

// PIICategory PII 数据类别.
type PIICategory string

const (
	CategoryIDCard      PIICategory = "id_card"       // 身份证号
	CategoryPhone       PIICategory = "phone"         // 手机号
	CategoryEmail       PIICategory = "email"         // 邮箱
	CategoryBankCard    PIICategory = "bank_card"     // 银行卡号
	CategoryPassport    PIICategory = "passport"      // 护照号
	CategoryLicensePlate PIICategory = "license_plate" // 车牌号
	CategoryIPAddress   PIICategory = "ip_address"    // IP 地址
)

// PIIPattern PII 匹配模式.
type PIIPattern struct {
	Name        string          `json:"name"`
	Category    PIICategory     `json:"category"`
	Pattern     string          `json:"pattern"`
	Sensitivity SensitivityLevel `json:"sensitivity"`
	Description string          `json:"description"`
}

// PIIMatch PII 匹配结果.
type PIIMatch struct {
	Category    PIICategory     `json:"category"`
	Value       string          `json:"value"`
	Line        int             `json:"line"`
	Column      int             `json:"column"`
	Sensitivity SensitivityLevel `json:"sensitivity"`
	Context     string          `json:"context"`
}

// ScanResult 单个文件扫描结果.
type ScanResult struct {
	FilePath    string      `json:"file_path"`
	ScanTime    time.Time   `json:"scan_time"`
	Matches     []PIIMatch  `json:"matches"`
	TotalMatch  int         `json:"total_match"`
	Error       string      `json:"error,omitempty"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ID          string        `json:"id"`
	ScanTime    time.Time     `json:"scan_time"`
	TotalFiles  int           `json:"total_files"`
	ScannedFiles int          `json:"scanned_files"`
	TotalMatches int          `json:"total_matches"`
	Results     []*ScanResult `json:"results"`
	Summary     CategorySummary `json:"summary"`
	RiskLevel   string        `json:"risk_level"`
	Suggestions []string      `json:"suggestions"`
}

// CategorySummary 分类统计.
type CategorySummary struct {
	IDCardCount    int `json:"id_card_count"`
	PhoneCount     int `json:"phone_count"`
	EmailCount     int `json:"email_count"`
	BankCardCount  int `json:"bank_card_count"`
	PassportCount  int `json:"passport_count"`
	LicensePlateCount int `json:"license_plate_count"`
	IPAddressCount int `json:"ip_address_count"`
}

// MaskingSuggestion 脱敏建议.
type MaskingSuggestion struct {
	Category    PIICategory `json:"category"`
	Strategy    string      `json:"strategy"`
	Example     string      `json:"example"`
	Description string      `json:"description"`
}

// ScanRequest 扫描请求.
type ScanRequest struct {
	Path        string   `json:"path" binding:"required"`
	Extensions  []string `json:"extensions,omitempty"`
	MaxDepth    int      `json:"max_depth,omitempty"`
	ExcludeDirs []string `json:"exclude_dirs,omitempty"`
}

// ScanPathRequest 扫描指定路径请求.
type ScanPathRequest struct {
	Paths      []string `json:"paths" binding:"required,min=1"`
	Extensions []string `json:"extensions,omitempty"`
}
