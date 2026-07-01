// Package complreport 提供合规审计报告生成功能，
// 支持 GDPR、PIPL、SOC2、ISO27001 等标准，自动收集合规证据并生成报告。
package complreport

import "time"

// ========== 合规标准类型 ==========

// Standard 合规审计标准.
type Standard string

const (
	// StandardGDPR 欧盟通用数据保护条例.
	StandardGDPR Standard = "GDPR"
	// StandardPIPL 中国个人信息保护法.
	StandardPIPL Standard = "PIPL"
	// StandardSOC2 SOC2 服务组织控制.
	StandardSOC2 Standard = "SOC2"
	// StandardISO27001 ISO/IEC 27001 信息安全管理体系.
	StandardISO27001 Standard = "ISO27001"
	// StandardHIPAA 美国健康保险可携性与责任法案.
	StandardHIPAA Standard = "HIPAA"
	// StandardCCPA 加州消费者隐私法.
	StandardCCPA Standard = "CCPA"
)

// ReportStatus 报告状态.
type ReportStatus string

const (
	// StatusPending 待生成.
	StatusPending ReportStatus = "pending"
	// StatusGenerating 生成中.
	StatusGenerating ReportStatus = "generating"
	// StatusCompleted 已完成.
	StatusCompleted ReportStatus = "completed"
	// StatusFailed 生成失败.
	StatusFailed ReportStatus = "failed"
)

// EvidenceType 合规证据类型.
type EvidenceType string

const (
	// EvidenceAccessLog 访问日志.
	EvidenceAccessLog EvidenceType = "access_log"
	// EvidencePermission 权限配置.
	EvidencePermission EvidenceType = "permission"
	// EvidenceEncryption 加密状态.
	EvidenceEncryption EvidenceType = "encryption"
	// EvidenceBackup 备份记录.
	EvidenceBackup EvidenceType = "backup"
	// EvidenceNetworkConfig 网络配置.
	EvidenceNetworkConfig EvidenceType = "network_config"
	// EvidencePolicy 策略文档.
	EvidencePolicy EvidenceType = "policy"
	// EvidenceAuditTrail 审计追踪.
	EvidenceAuditTrail EvidenceType = "audit_trail"
)

// CheckStatus 检查项状态.
type CheckStatus string

const (
	// CheckPass 通过.
	CheckPass CheckStatus = "pass"
	// CheckFail 不通过.
	CheckFail CheckStatus = "fail"
	// CheckWarning 警告.
	CheckWarning CheckStatus = "warning"
	// CheckNotApplicable 不适用.
	CheckNotApplicable CheckStatus = "n/a"
)

// Severity 严重程度.
type Severity string

const (
	// SeverityLow 低.
	SeverityLow Severity = "low"
	// SeverityMedium 中.
	SeverityMedium Severity = "medium"
	// SeverityHigh 高.
	SeverityHigh Severity = "high"
	// SeverityCritical 严重.
	SeverityCritical Severity = "critical"
)

// ReportFormat 报告格式.
type ReportFormat string

const (
	// FormatJSON JSON 格式.
	FormatJSON ReportFormat = "json"
	// FormatPDF PDF 格式.
	FormatPDF ReportFormat = "pdf"
)

// ========== 证据和检查项类型 ==========

// Evidence 合规证据.
type Evidence struct {
	Type      EvidenceType `json:"type"`       // 证据类型
	Source    string       `json:"source"`     // 证据来源（系统/模块名）
	Title     string       `json:"title"`      // 证据标题
	Summary   string       `json:"summary"`    // 摘要
	Detail    string       `json:"detail,omitempty"` // 详细信息
	Timestamp time.Time    `json:"timestamp"`  // 采集时间
	Status    CheckStatus  `json:"status"`     // 检查状态
	Severity  Severity     `json:"severity,omitempty"` // 严重程度（fail/warning 时填写）
}

// ControlCheck 控制项检查结果.
type ControlCheck struct {
	ID         string      `json:"id"`          // 控制项 ID
	Category   string      `json:"category"`    // 控制类别
	Title      string      `json:"title"`       // 控制项标题
	Status     CheckStatus `json:"status"`      // 检查状态
	Severity   Severity    `json:"severity,omitempty"` // 严重程度
	Evidence   []Evidence  `json:"evidence"`    // 相关证据
	Remediation string    `json:"remediation,omitempty"` // 整改建议
}

// ========== 报告类型 ==========

// Report 合规审计报告.
type Report struct {
	ID           string         `json:"id"`            // 报告唯一标识
	Standard     Standard       `json:"standard"`      // 合规标准
	Title        string         `json:"title"`         // 报告标题
	Status       ReportStatus   `json:"status"`        // 报告状态
	Format       ReportFormat   `json:"format"`        // 报告格式
	Score        int            `json:"score"`         // 合规评分（0-100）
	TotalChecks  int            `json:"total_checks"`  // 总检查项数
	Passed       int            `json:"passed"`        // 通过数
	Failed       int            `json:"failed"`        // 不通过数
	Warnings     int            `json:"warnings"`      // 警告数
	NotApplicable int          `json:"not_applicable"` // 不适用数
	Controls     []ControlCheck `json:"controls"`      // 控制项检查结果
	Summary      string         `json:"summary"`       // 报告摘要
	GeneratedBy  string         `json:"generated_by"`  // 生成者
	CreatedAt    time.Time      `json:"created_at"`    // 创建时间
	CompletedAt  *time.Time     `json:"completed_at,omitempty"` // 完成时间
}

// Schedule 定期报告计划.
type Schedule struct {
	ID          string   `json:"id"`           // 计划唯一标识
	Standard    Standard `json:"standard"`     // 合规标准
	Format      ReportFormat `json:"format"`   // 报告格式
	CronExpr    string   `json:"cron_expr"`    // cron 表达式
	Enabled     bool     `json:"enabled"`      // 是否启用
	GeneratedBy string   `json:"generated_by"` // 生成者
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	LastRunAt   *time.Time `json:"last_run_at,omitempty"` // 上次执行时间
}

// ========== 请求/响应类型 ==========

// GenerateRequest 生成报告请求.
type GenerateRequest struct {
	Standard    Standard      `json:"standard" binding:"required"`  // 合规标准
	Format      ReportFormat  `json:"format,omitempty"`             // 报告格式，默认 json
	Title       string        `json:"title,omitempty"`              // 报告标题
	GeneratedBy string        `json:"generated_by" binding:"required"` // 生成者
}

// ScheduleRequest 创建定期报告计划请求.
type ScheduleRequest struct {
	Standard    Standard      `json:"standard" binding:"required"` // 合规标准
	Format      ReportFormat  `json:"format,omitempty"`            // 报告格式
	CronExpr    string        `json:"cron_expr" binding:"required"` // cron 表达式
	GeneratedBy string        `json:"generated_by" binding:"required"` // 生成者
}

// ListResponse 报告列表响应.
type ListResponse struct {
	Reports []*Report `json:"reports"` // 报告列表
	Total   int       `json:"total"`   // 总数
}
