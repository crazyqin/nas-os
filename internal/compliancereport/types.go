// Package compliancereport 提供合规报告生成功能
package compliancereport

import (
	"fmt"
	"time"
)

// ComplianceStandard 合规标准.
type ComplianceStandard string

// 支持的合规标准.
const (
	StandardGDPR     ComplianceStandard = "gdpr"     // 欧盟通用数据保护条例
	StandardSOC2     ComplianceStandard = "soc2"     // SOC2 服务组织控制
	StandardDJBH     ComplianceStandard = "djbh"     // 等保 2.0
	StandardISO27001 ComplianceStandard = "iso27001" // ISO/IEC 27001
	StandardHIPAA    ComplianceStandard = "hipaa"    // 健康保险可携性与责任法案
)

// ScanStatus 扫描状态.
type ScanStatus string

const (
	ScanStatusPending  ScanStatus = "pending"
	ScanStatusRunning  ScanStatus = "running"
	ScanStatusComplete ScanStatus = "complete"
	ScanStatusFailed   ScanStatus = "failed"
)

// ComplianceStatus 合规状态.
type ComplianceStatus string

const (
	StatusCompliant     ComplianceStatus = "compliant"      // 合规
	StatusNonCompliant  ComplianceStatus = "non_compliant"  // 不合规
	StatusPendingReview ComplianceStatus = "pending_review" // 待审查
)

// CheckCategory 扫描检查类别.
type CheckCategory string

const (
	CategoryAccessControl  CheckCategory = "access_control"  // 访问控制审计
	CategoryDataEncryption CheckCategory = "data_encryption"  // 数据加密状态
	CategoryLogIntegrity   CheckCategory = "log_integrity"   // 日志完整性
	CategoryBackup         CheckCategory = "backup"          // 备份合规性
	CategoryNetwork        CheckCategory = "network_security" // 网络安全配置
)

// CheckItemStatus 检查项状态.
type CheckItemStatus string

const (
	CheckItemPass    CheckItemStatus = "pass"
	CheckItemFail    CheckItemStatus = "fail"
	CheckItemWarning CheckItemStatus = "warning"
	CheckItemSkip    CheckItemStatus = "skip"
)

// Severity 严重程度.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// ReportFormat 报告格式.
type ReportFormat string

const (
	FormatJSON ReportFormat = "json"
	FormatPDF  ReportFormat = "pdf"
)

// StandardInfo 合规标准信息.
type StandardInfo struct {
	ID          ComplianceStandard `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Version     string             `json:"version"`
	Categories  []CheckCategory    `json:"categories"`
}

// ScanRequest 扫描请求.
type ScanRequest struct {
	Standard ComplianceStandard `json:"standard" binding:"required"`
	Categories []CheckCategory   `json:"categories,omitempty"` // 可选，指定扫描类别
	Format   ReportFormat       `json:"format,omitempty"`      // 报告格式，默认 json
}

// ScanResult 单项扫描结果.
type ScanResult struct {
	CheckID    string          `json:"check_id"`
	Category   CheckCategory   `json:"category"`
	Name       string          `json:"name"`
	Status     CheckItemStatus `json:"status"`
	Severity   Severity        `json:"severity"`
	Message    string          `json:"message"`
	Details    string          `json:"details,omitempty"`
	Timestamp  time.Time       `json:"timestamp"`
}

// Remediation 整改建议.
type Remediation struct {
	CheckID     string   `json:"check_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	Priority    Severity `json:"priority"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ID                string              `json:"id"`
	Standard          ComplianceStandard  `json:"standard"`
	Status            ScanStatus          `json:"status"`
	ComplianceStatus  ComplianceStatus    `json:"compliance_status"`
	Score             int                 `json:"score"` // 0-100
	TotalChecks       int                 `json:"total_checks"`
	Passed            int                 `json:"passed"`
	Failed            int                 `json:"failed"`
	Warnings          int                 `json:"warnings"`
	Skipped           int                 `json:"skipped"`
	Results           []ScanResult        `json:"results"`
	Remediations      []Remediation       `json:"remediations"`
	Summary           string              `json:"summary"`
	Format            ReportFormat        `json:"format"`
	CreatedAt         time.Time           `json:"created_at"`
	CompletedAt       *time.Time          `json:"completed_at,omitempty"`
}

// ComplianceStatusOverview 合规状态总览.
type ComplianceStatusOverview struct {
	OverallStatus     ComplianceStatus             `json:"overall_status"`
	OverallScore      int                          `json:"overall_score"`
	Standards         []StandardStatus             `json:"standards"`
	LastScanTime      *time.Time                   `json:"last_scan_time,omitempty"`
	TotalReports      int                          `json:"total_reports"`
	PendingRemediation int                          `json:"pending_remediation"`
}

// StandardStatus 单个标准的合规状态.
type StandardStatus struct {
	Standard ComplianceStandard `json:"standard"`
	Status   ComplianceStatus   `json:"status"`
	Score    int                `json:"score"`
	LastScan *time.Time         `json:"last_scan,omitempty"`
}

// GenerateID 生成唯一 ID.
func GenerateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
