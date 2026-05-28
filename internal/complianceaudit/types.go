// Package complianceaudit 提供合规审计功能，支持多标准合规检查和审计报告生成
package complianceaudit

import (
	"time"
)

// ComplianceStandard 合规标准类型
type ComplianceStandard string

const (
	StandardGDPR    ComplianceStandard = "gdpr"     // 欧盟通用数据保护条例
	StandardMLPS2   ComplianceStandard = "mlps2"    // 等保2.0
	StandardISO27001 ComplianceStandard = "iso27001" // ISO/IEC 27001
	StandardSOC2    ComplianceStandard = "soc2"     // SOC 2
)

// CheckStatus 检查状态
type CheckStatus string

const (
	StatusPass CheckStatus = "pass" // 通过
	StatusFail CheckStatus = "fail" // 不通过
	StatusWarn CheckStatus = "warn" // 警告
	StatusSkip CheckStatus = "skip" // 跳过
)

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"      // 低风险
	RiskMedium   RiskLevel = "medium"   // 中风险
	RiskHigh     RiskLevel = "high"     // 高风险
	RiskCritical RiskLevel = "critical" // 严重风险
)

// ReportFormat 报告格式
type ReportFormat string

const (
	FormatJSON ReportFormat = "json" // JSON 格式
	FormatPDF  ReportFormat = "pdf"  // PDF 格式
)

// CheckCategory 检查类别
type CheckCategory string

const (
	CategoryPasswordPolicy   CheckCategory = "password_policy"   // 密码策略
	CategoryAccessControl    CheckCategory = "access_control"    // 访问控制
	CategoryEncryption       CheckCategory = "encryption"        // 加密状态
	CategoryDataProtection   CheckCategory = "data_protection"   // 数据保护
	CategoryAuditLog         CheckCategory = "audit_log"         // 审计日志
	CategoryNetworkSecurity  CheckCategory = "network_security"  // 网络安全
	CategoryIncidentResponse CheckCategory = "incident_response" // 应急响应
)

// ComplianceCheck 合规检查项接口
type ComplianceCheck interface {
	// Name 返回检查项名称
	Name() string
	// Standard 返回适用的合规标准
	Standard() ComplianceStandard
	// Category 返回检查类别
	Category() CheckCategory
	// Description 返回检查项描述
	Description() string
	// Check 执行合规检查
	Check(ctx *CheckContext) *CheckResult
	// GetRemediation 获取整改建议
	GetRemediation(result *CheckResult) *Remediation
}

// CheckContext 检查上下文
type CheckContext struct {
	Timeout   time.Duration `json:"-"`
	Forced    bool          `json:"forced"`     // 是否强制执行
	Target    string        `json:"target"`     // 检查目标
	ExtraData interface{}   `json:"extra_data"` // 额外数据
}

// CheckResult 检查结果
type CheckResult struct {
	Name        string                 `json:"name"`
	Standard    ComplianceStandard     `json:"standard"`
	Category    CheckCategory          `json:"category"`
	Status      CheckStatus            `json:"status"`
	RiskLevel   RiskLevel              `json:"risk_level"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Duration    time.Duration          `json:"duration"`
	Remediation *Remediation           `json:"remediation,omitempty"`
}

// Remediation 整改建议
type Remediation struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	Priority    int      `json:"priority"` // 1-5, 5最高
	Deadline    int      `json:"deadline"` // 建议完成天数
}

// AuditLog 审计日志
type AuditLog struct {
	ID        int64                  `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Actor     string                 `json:"actor"`     // 操作者
	Action    string                 `json:"action"`    // 操作
	Resource  string                 `json:"resource"`  // 资源
	Result    string                 `json:"result"`    // 结果
	Details   map[string]interface{} `json:"details"`   // 详情
	IPAddress string                 `json:"ip_address"` // IP地址
	UserAgent string                 `json:"user_agent"` // 用户代理
}

// ComplianceScore 合规评分
type ComplianceScore struct {
	Overall     float64                      `json:"overall"`      // 总分 0-100
	ByStandard  map[ComplianceStandard]float64 `json:"by_standard"` // 按标准评分
	ByCategory  map[CheckCategory]float64      `json:"by_category"` // 按类别评分
	Trend       []ScoreTrend                  `json:"trend"`       // 趋势
	LastUpdated time.Time                     `json:"last_updated"`
}

// ScoreTrend 评分趋势
type ScoreTrend struct {
	Date  time.Time `json:"date"`
	Score float64   `json:"score"`
}

// ScanConfig 扫描配置
type ScanConfig struct {
	Standards     []ComplianceStandard `json:"standards"`      // 要扫描的标准
	Categories    []CheckCategory      `json:"categories"`     // 要扫描的类别
	Schedule      string               `json:"schedule"`       // 定时扫描 cron 表达式
	Enabled       bool                 `json:"enabled"`        // 是否启用
	NotifyOnFail  bool                 `json:"notify_on_fail"` // 失败时通知
	AutoRemediate bool                 `json:"auto_remediate"` // 自动整改
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	GeneratedAt time.Time           `json:"generated_at"`
	Period      ReportPeriod        `json:"period"`
	Summary     *ReportSummary      `json:"summary"`
	Standards   []*StandardReport   `json:"standards"`
	Findings    []*Finding          `json:"findings"`
	Remediations []*RemediationItem `json:"remediations"`
	Format      ReportFormat        `json:"format"`
}

// Period 报告周期
type ReportPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalChecks  int     `json:"total_checks"`
	Passed       int     `json:"passed"`
	Failed       int     `json:"failed"`
	Warnings     int     `json:"warnings"`
	OverallScore float64 `json:"overall_score"`
	RiskLevel    RiskLevel `json:"risk_level"`
}

// StandardReport 标准报告
type StandardReport struct {
	Standard ComplianceStandard `json:"standard"`
	Score    float64            `json:"score"`
	Checks   []*CheckResult     `json:"checks"`
}

// Finding 发现问题
type Finding struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	RiskLevel   RiskLevel   `json:"risk_level"`
	Standard    ComplianceStandard `json:"standard"`
	Category    CheckCategory `json:"category"`
	Status      CheckStatus `json:"status"`
}

// RemediationItem 整改项
type RemediationItem struct {
	FindingID   string      `json:"finding_id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Steps       []string    `json:"steps"`
	Priority    int         `json:"priority"`
	Status      string      `json:"status"` // pending, in_progress, completed
	AssignedTo  string      `json:"assigned_to"`
	Deadline    time.Time   `json:"deadline"`
}

// DashboardData 仪表盘数据
type DashboardData struct {
	Score           *ComplianceScore    `json:"score"`
	RecentFindings  []*Finding          `json:"recent_findings"`
	Trends          []ScoreTrend        `json:"trends"`
	ActiveRemediations int              `json:"active_remediations"`
	LastScanTime    time.Time           `json:"last_scan_time"`
	StandardsStatus map[ComplianceStandard]StandardStatus `json:"standards_status"`
}

// StandardStatus 标准状态
type StandardStatus struct {
	Score       float64   `json:"score"`
	LastChecked time.Time `json:"last_checked"`
	CheckCount  int       `json:"check_count"`
	PassRate    float64   `json:"pass_rate"`
}
