// Package emailsecurity 提供邮件安全审核增强功能，对标群晖 DSM 7.3 的邮件安全审核
package emailsecurity

import (
	"time"
)

// SecurityPolicy 安全策略配置.
type SecurityPolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"` // 优先级，数值越小越优先
	// 附件安全配置
	AttachmentScan AttachmentScanConfig `json:"attachment_scan"`
	// 钓鱼检测配置
	PhishingDetection PhishingDetectionConfig `json:"phishing_detection"`
	// 内容合规配置
	ContentCompliance ContentComplianceConfig `json:"content_compliance"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

// AttachmentScanConfig 附件安全扫描配置.
type AttachmentScanConfig struct {
	Enabled           bool     `json:"enabled"`
	BlockExecutables  bool     `json:"block_executables"`   // 阻止可执行文件
	BlockMacroDocs    bool     `json:"block_macro_docs"`    // 阻止含宏的文档
	BlockArchiveTypes []string `json:"block_archive_types"` // 阻止的压缩文件类型
	MaxSizeMB         int      `json:"max_size_mb"`         // 最大附件大小(MB)
	ScanTimeoutSec    int      `json:"scan_timeout_sec"`    // 扫描超时(秒)
}

// PhishingDetectionConfig 钓鱼检测配置.
type PhishingDetectionConfig struct {
	Enabled             bool     `json:"enabled"`
	CheckURLReputation  bool     `json:"check_url_reputation"`  // 检查URL信誉
	BlockSuspiciousURLs bool     `json:"block_suspicious_urls"` // 阻止可疑URL
	WhitelistDomains    []string `json:"whitelist_domains"`     // 白名单域名
	BlacklistDomains    []string `json:"blacklist_domains"`     // 黑名单域名
	MaxURLsPerEmail     int      `json:"max_urls_per_email"`    // 每封邮件最大URL数
}

// ContentComplianceConfig 内容合规配置.
type ContentComplianceConfig struct {
	Enabled           bool     `json:"enabled"`
	KeywordFilters    []string `json:"keyword_filters"`    // 关键词过滤列表
	RegexPatterns     []string `json:"regex_patterns"`     // 正则表达式模式
	BlockConfidential bool     `json:"block_confidential"` // 阻止机密信息泄露
	AlertOnViolation  bool     `json:"alert_on_violation"` // 违规时告警
}

// QuarantineItem 隔离邮件项.
type QuarantineItem struct {
	ID            string     `json:"id"`
	MessageID     string     `json:"message_id"` // 原始邮件ID
	From          string     `json:"from"`
	To            []string   `json:"to"`
	Subject       string     `json:"subject"`
	Reason        string     `json:"reason"`       // 隔离原因
	ThreatLevel   string     `json:"threat_level"` // low, medium, high, critical
	Status        string     `json:"status"`       // pending, approved, rejected, released
	ScanResult    ScanResult `json:"scan_result"`
	QuarantinedBy string     `json:"quarantined_by"`        // 隔离操作者
	ReviewedBy    string     `json:"reviewed_by,omitempty"` // 审批人
	ReviewNote    string     `json:"review_note,omitempty"` // 审批备注
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ExpiresAt     time.Time  `json:"expires_at"` // 过期时间
}

// ScanResult 扫描结果.
type ScanResult struct {
	Threats       []ThreatItem `json:"threats"`
	Score         int          `json:"score"`          // 威胁评分 0-100
	ScanDuration  int64        `json:"scan_duration"`  // 扫描耗时(毫秒)
	ScannerEngine string       `json:"scanner_engine"` // 扫描引擎
	ScannedAt     time.Time    `json:"scanned_at"`
}

// ThreatItem 威胁项.
type ThreatItem struct {
	Type        string `json:"type"`        // attachment, phishing, content, malware
	Name        string `json:"name"`        // 威胁名称
	Description string `json:"description"` // 威胁描述
	Severity    string `json:"severity"`    // low, medium, high, critical
	Location    string `json:"location"`    // 威胁位置（如附件名、URL）
	Action      string `json:"action"`      // block, quarantine, allow
}

// ThreatReport 威胁报告.
type ThreatReport struct {
	ID               string          `json:"id"`
	Period           string          `json:"period"` // daily, weekly, monthly
	StartTime        time.Time       `json:"start_time"`
	EndTime          time.Time       `json:"end_time"`
	TotalScanned     int             `json:"total_scanned"`     // 扫描总数
	TotalBlocked     int             `json:"total_blocked"`     // 阻止总数
	TotalQuarantined int             `json:"total_quarantined"` // 隔离总数
	ThreatsByType    map[string]int  `json:"threats_by_type"`   // 按类型统计
	TopThreats       []ThreatSummary `json:"top_threats"`       // 主要威胁
	GeneratedAt      time.Time       `json:"generated_at"`
}

// ThreatSummary 威胁摘要.
type ThreatSummary struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Count    int    `json:"count"`
	Severity string `json:"severity"`
}

// AuditRule 审计规则.
type AuditRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Condition   string    `json:"condition"` // 触发条件
	Action      string    `json:"action"`    // block, quarantine, alert, log
	Target      string    `json:"target"`    // 目标（如用户、域名）
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ========== 请求/响应结构 ==========

// CreatePolicyRequest 创建安全策略请求.
type CreatePolicyRequest struct {
	Name              string                  `json:"name" binding:"required"`
	Description       string                  `json:"description"`
	Priority          int                     `json:"priority"`
	AttachmentScan    AttachmentScanConfig    `json:"attachment_scan"`
	PhishingDetection PhishingDetectionConfig `json:"phishing_detection"`
	ContentCompliance ContentComplianceConfig `json:"content_compliance"`
}

// UpdatePolicyRequest 更新安全策略请求.
type UpdatePolicyRequest struct {
	Name              *string                  `json:"name,omitempty"`
	Description       *string                  `json:"description,omitempty"`
	Enabled           *bool                    `json:"enabled,omitempty"`
	Priority          *int                     `json:"priority,omitempty"`
	AttachmentScan    *AttachmentScanConfig    `json:"attachment_scan,omitempty"`
	PhishingDetection *PhishingDetectionConfig `json:"phishing_detection,omitempty"`
	ContentCompliance *ContentComplianceConfig `json:"content_compliance,omitempty"`
}

// ReviewQuarantineRequest 审批隔离邮件请求.
type ReviewQuarantineRequest struct {
	Action   string `json:"action" binding:"required"` // approve, reject, release
	Note     string `json:"note"`
	ReviewBy string `json:"review_by" binding:"required"`
}

// ScanEmailRequest 扫描邮件请求.
type ScanEmailRequest struct {
	MessageID   string           `json:"message_id" binding:"required"`
	From        string           `json:"from" binding:"required"`
	To          []string         `json:"to" binding:"required"`
	Subject     string           `json:"subject"`
	Body        string           `json:"body"`
	Attachments []AttachmentInfo `json:"attachments,omitempty"`
}

// AttachmentInfo 附件信息.
type AttachmentInfo struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
	Content  []byte `json:"content,omitempty"` // Base64编码的内容
}

// GenerateReportRequest 生成报告请求.
type GenerateReportRequest struct {
	Period    string `json:"period" binding:"required"` // daily, weekly, monthly
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// ListQuarantineRequest 隔离邮件列表请求.
type ListQuarantineRequest struct {
	Status      string `form:"status"`
	ThreatLevel string `form:"threat_level"`
	Page        int    `form:"page"`
	PageSize    int    `form:"page_size"`
}

// CreateAuditRuleRequest 创建审计规则请求.
type CreateAuditRuleRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Condition   string `json:"condition" binding:"required"`
	Action      string `json:"action" binding:"required"`
	Target      string `json:"target"`
}

// EmailSecurityStats 邮件安全统计.
type EmailSecurityStats struct {
	TotalScanned24h     int     `json:"total_scanned_24h"`
	TotalBlocked24h     int     `json:"total_blocked_24h"`
	TotalQuarantined24h int     `json:"total_quarantined_24h"`
	BlockRate           float64 `json:"block_rate"`      // 阻止率
	QuarantineRate      float64 `json:"quarantine_rate"` // 隔离率
	PendingReview       int     `json:"pending_review"`  // 待审批数
	ActivePolicies      int     `json:"active_policies"` // 活跃策略数
	ActiveRules         int     `json:"active_rules"`    // 活跃规则数
}

// ScanStatus 扫描状态枚举.
const (
	ScanStatusPending   = "pending"
	ScanStatusScanning  = "scanning"
	ScanStatusCompleted = "completed"
	ScanStatusFailed    = "failed"
)

// ThreatLevel 威胁等级枚举.
const (
	ThreatLevelLow      = "low"
	ThreatLevelMedium   = "medium"
	ThreatLevelHigh     = "high"
	ThreatLevelCritical = "critical"
)

// QuarantineStatus 隔离状态枚举.
const (
	QuarantineStatusPending  = "pending"
	QuarantineStatusApproved = "approved"
	QuarantineStatusRejected = "rejected"
	QuarantineStatusReleased = "released"
)

// ThreatType 威胁类型枚举.
const (
	ThreatTypeAttachment = "attachment"
	ThreatTypePhishing   = "phishing"
	ThreatTypeContent    = "content"
	ThreatTypeMalware    = "malware"
)

// AuditAction 审计动作枚举.
const (
	AuditActionBlock      = "block"
	AuditActionQuarantine = "quarantine"
	AuditActionAlert      = "alert"
	AuditActionLog        = "log"
)
