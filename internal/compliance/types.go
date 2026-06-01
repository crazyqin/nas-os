// Package compliance 提供合规中心功能，包括 GDPR/CCPA 合规检查、数据分类扫描、合规报告生成等。
package compliance

import (
	"time"
)

// ==================== 合规规则相关 ====================

// ComplianceRule 合规规则
type ComplianceRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Regulation  string    `json:"regulation"`  // GDPR, CCPA, HIPAA 等
	Category    string    `json:"category"`    // data-protection, privacy, security 等
	Severity    string    `json:"severity"`    // critical, high, medium, low
	Description string    `json:"description"`
	Condition   string    `json:"condition"`   // 规则条件
	Action      string    `json:"action"`      // 触发动作
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RuleCategory 规则分类
type RuleCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ==================== 扫描相关 ====================

// ScanResult 扫描结果
type ScanResult struct {
	ID          string             `json:"id"`
	RuleID      string             `json:"rule_id"`
	ResourceID  string             `json:"resource_id"`
	ResourceType string            `json:"resource_type"`
	Status      string             `json:"status"` // compliant, non-compliant, warning
	Details     string             `json:"details"`
	Evidence    []string           `json:"evidence,omitempty"`
	Remediation string             `json:"remediation,omitempty"`
	ScannedAt   time.Time          `json:"scanned_at"`
}

// ScanRequest 扫描请求
type ScanRequest struct {
	Rules       []string `json:"rules,omitempty"`       // 指定规则ID，空则全部
	Resources   []string `json:"resources,omitempty"`   // 指定资源，空则全部
	Regulations []string `json:"regulations,omitempty"` // 指定法规
	Async       bool     `json:"async,omitempty"`
}

// ScanReport 扫描报告
type ScanReport struct {
	ID          string        `json:"id"`
	ScanID      string        `json:"scan_id"`
	Summary     ScanSummary   `json:"summary"`
	Results     []ScanResult  `json:"results"`
	GeneratedAt time.Time     `json:"generated_at"`
}

// ScanSummary 扫描摘要
type ScanSummary struct {
	TotalChecked  int     `json:"total_checked"`
	Compliant     int     `json:"compliant"`
	NonCompliant  int     `json:"non_compliant"`
	Warnings      int     `json:"warnings"`
	ComplianceRate float64 `json:"compliance_rate"`
}

// ==================== 报告相关 ====================

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID            string              `json:"id"`
	Title         string              `json:"title"`
	Regulation    string              `json:"regulation"`
	Period        ReportPeriod        `json:"period"`
	Summary       ReportSummary       `json:"summary"`
	Findings      []Finding           `json:"findings"`
	Recommendations []Recommendation  `json:"recommendations"`
	Status        string              `json:"status"` // draft, final, submitted
	GeneratedAt   time.Time           `json:"generated_at"`
	ApprovedBy    string              `json:"approved_by,omitempty"`
	ApprovedAt    *time.Time          `json:"approved_at,omitempty"`
}

// ReportPeriod 报告周期
type ReportPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	OverallScore     float64 `json:"overall_score"`
	TotalIssues      int     `json:"total_issues"`
	CriticalIssues   int     `json:"critical_issues"`
	ResolvedIssues   int     `json:"resolved_issues"`
	PendingIssues    int     `json:"pending_issues"`
}

// Finding 发现问题
type Finding struct {
	ID          string    `json:"id"`
	RuleID      string    `json:"rule_id"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Evidence    string    `json:"evidence"`
	Status      string    `json:"status"` // open, in-progress, resolved
	DetectedAt  time.Time `json:"detected_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// Recommendation 整改建议
type Recommendation struct {
	ID          string `json:"id"`
	FindingID   string `json:"finding_id"`
	Priority    string `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Effort      string `json:"effort"` // low, medium, high
	Deadline    *time.Time `json:"deadline,omitempty"`
}

// ==================== 整改计划相关 ====================

// RemediationPlan 整改计划
type RemediationPlan struct {
	ID          string             `json:"id"`
	ReportID    string             `json:"report_id"`
	Title       string             `json:"description"`
	Status      string             `json:"status"` // draft, active, completed
	Items       []RemediationItem  `json:"items"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	CompletedAt *time.Time         `json:"completed_at,omitempty"`
}

// RemediationItem 整改项
type RemediationItem struct {
	ID          string     `json:"id"`
	FindingID   string     `json:"finding_id"`
	Action      string     `json:"action"`
	Assignee    string     `json:"assignee"`
	Deadline    time.Time  `json:"deadline"`
	Status      string     `json:"status"` // pending, in-progress, completed
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Notes       string     `json:"notes,omitempty"`
}

// ==================== 数据分类相关 ====================

// DataClassification 数据分类
type DataClassification struct {
	ID          string            `json:"id"`
	ResourceID  string            `json:"resource_id"`
	ResourceType string           `json:"resource_type"`
	Category    string            `json:"category"`   // PII, PHI, financial, confidential, public
	Sensitivity string            `json:"sensitivity"` // high, medium, low
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Owner       string            `json:"owner"`
	Location    string            `json:"location"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// DataCategory 数据类别
type DataCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Guidelines  string `json:"guidelines"`
}

// ScanDataRequest 数据扫描请求
type ScanDataRequest struct {
	Path        string   `json:"path"`
	Recursive   bool     `json:"recursive"`
	Categories  []string `json:"categories,omitempty"`
	MinFileSize int64    `json:"min_file_size,omitempty"`
}

// DataScanResult 数据扫描结果
type DataScanResult struct {
	ID              string              `json:"id"`
	Path            string              `json:"path"`
	Summary         DataScanSummary     `json:"summary"`
	Classifications []DataClassification `json:"classifications"`
	ScannedAt       time.Time           `json:"scanned_at"`
}

// DataScanSummary 数据扫描摘要
type DataScanSummary struct {
	TotalFiles    int            `json:"total_files"`
	TotalSize     int64          `json:"total_bytes"`
	ByCategory    map[string]int `json:"by_category"`
	BySensitivity map[string]int `json:"by_sensitivity"`
}
