// Package compliance 提供合规报告生成功能
package compliance

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrStandardNotFound 合规标准不存在.
	ErrStandardNotFound = errors.New("合规标准不存在")
	// ErrCheckNotFound 检查项不存在.
	ErrCheckNotFound = errors.New("检查项不存在")
	// ErrInvalidConfig 无效配置.
	ErrInvalidConfig = errors.New("无效的配置")
)

// ========== 合规标准 ==========

// ComplianceStandard 合规标准.
type ComplianceStandard string

const (
	// StandardGDPR GDPR通用数据保护条例.
	StandardGDPR ComplianceStandard = "gdpr"
	// StandardMLPS2 等保2.0.
	StandardMLPS2 ComplianceStandard = "mlps2"
	// StandardISO27001 ISO 27001信息安全管理.
	StandardISO27001 ComplianceStandard = "iso27001"
)

// ========== 检查状态 ==========

// CheckStatus 检查状态.
type CheckStatus string

const (
	// StatusPass 通过.
	StatusPass CheckStatus = "pass"
	// StatusFail 失败.
	StatusFail CheckStatus = "fail"
	// StatusWarning 警告.
	StatusWarning CheckStatus = "warning"
	// StatusSkip 跳过.
	StatusSkip CheckStatus = "skip"
	// StatusError 错误.
	StatusError CheckStatus = "error"
)

// ========== 核心数据结构 ==========

// StandardInfo 合规标准信息.
type StandardInfo struct {
	ID          ComplianceStandard `json:"id"`
	Name        string             `json:"name"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Category    string             `json:"category"`
	CheckCount  int                `json:"check_count"`
}

// CheckItem 检查项.
type CheckItem struct {
	ID          string             `json:"id"`
	Standard    ComplianceStandard `json:"standard"`
	Category    string             `json:"category"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Requirement string             `json:"requirement"`
	Severity    string             `json:"severity"` // critical/high/medium/low
	Automated   bool               `json:"automated"`
}

// CheckResultItem 检查结果.
type CheckResultItem struct {
	CheckID     string      `json:"check_id"`
	CheckName   string      `json:"check_name"`
	Standard    string      `json:"standard"`
	Category    string      `json:"category"`
	Status      CheckStatus `json:"status"`
	Message     string      `json:"message"`
	Details     string      `json:"details,omitempty"`
	Evidence    string      `json:"evidence,omitempty"`
	Remediation string      `json:"remediation,omitempty"`
	CheckedAt   time.Time   `json:"checked_at"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ID               string             `json:"id"`
	GeneratedAt      time.Time          `json:"generated_at"`
	Standard         ComplianceStandard `json:"standard"`
	StandardName     string             `json:"standard_name"`
	OverallScore     float64            `json:"overall_score"`     // 0-100
	OverallStatus    CheckStatus        `json:"overall_status"`
	TotalChecks      int                `json:"total_checks"`
	PassedChecks     int                `json:"passed_checks"`
	FailedChecks     int                `json:"failed_checks"`
	WarningChecks    int                `json:"warning_checks"`
	SkippedChecks    int                `json:"skipped_checks"`
	Results          []CheckResultItem  `json:"results"`
	CategorySummary  []CategorySummary  `json:"category_summary"`
	Recommendations  []Recommendation   `json:"recommendations"`
	ComplianceLevel  string             `json:"compliance_level"` // A/B/C/D
}

// CategorySummary 分类摘要.
type CategorySummary struct {
	Category    string  `json:"category"`
	Total       int     `json:"total"`
	Passed      int     `json:"passed"`
	Failed      int     `json:"failed"`
	Warnings    int     `json:"warnings"`
	Score       float64 `json:"score"`
}

// Recommendation 整改建议.
type Recommendation struct {
	ID          string `json:"id"`
	Priority    string `json:"priority"` // critical/high/medium/low
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Actions     []string `json:"actions"`
	References  []string `json:"references,omitempty"`
}

// ScanConfig 扫描配置.
type ScanConfig struct {
	// Standards 要扫描的合规标准.
	Standards []ComplianceStandard `json:"standards"`
	// Categories 要扫描的分类（空表示全部）.
	Categories []string `json:"categories,omitempty"`
	// SkipCategories 要跳过的分类.
	SkipCategories []string `json:"skip_categories,omitempty"`
	// IncludeManual 是否包含手动检查项.
	IncludeManual bool `json:"include_manual"`
	// OutputFormat 输出格式（json/html/text）.
	OutputFormat string `json:"output_format"`
	// OutputDir 输出目录.
	OutputDir string `json:"output_dir"`
}

// DefaultScanConfig 返回默认扫描配置.
func DefaultScanConfig() *ScanConfig {
	return &ScanConfig{
		Standards:      []ComplianceStandard{StandardGDPR, StandardMLPS2, StandardISO27001},
		IncludeManual:  false,
		OutputFormat:   "json",
		OutputDir:      "/var/log/nas-os/compliance",
	}
}

// ComplianceDashboard 合规仪表盘数据.
type ComplianceDashboard struct {
	GeneratedAt      time.Time              `json:"generated_at"`
	OverallScore     float64                `json:"overall_score"`
	Standards        []StandardSummary      `json:"standards"`
	RecentReports    []ReportSummary        `json:"recent_reports"`
	TopIssues        []CheckResultItem      `json:"top_issues"`
	TrendData        []TrendPoint           `json:"trend_data"`
}

// StandardSummary 标准摘要.
type StandardSummary struct {
	Standard    ComplianceStandard `json:"standard"`
	Name        string             `json:"name"`
	Score       float64            `json:"score"`
	Status      CheckStatus        `json:"status"`
	TotalChecks int                `json:"total_checks"`
	PassedChecks int               `json:"passed_checks"`
}

// ReportSummary 报告摘要.
type ReportSummary struct {
	ID           string             `json:"id"`
	GeneratedAt  time.Time          `json:"generated_at"`
	Standard     ComplianceStandard `json:"standard"`
	Score        float64            `json:"score"`
	Status       CheckStatus        `json:"status"`
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	Date   time.Time `json:"date"`
	Score  float64   `json:"score"`
	Status string    `json:"status"`
}
