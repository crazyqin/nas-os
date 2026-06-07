// Package licensescan 提供许可证合规扫描功能
// 自动扫描Docker应用和Go依赖的许可证合规性
package licensescan

import "time"

// ========== 许可证分类 ==========

// Category 许可证类别.
type Category string

// 许可证类别常量.
const (
	CategoryPermissive     Category = "permissive"      // 宽松许可证 (MIT/BSD/Apache)
	CategoryWeakCopyleft   Category = "weak_copyleft"   // 弱传染 (LGPL)
	CategoryStrongCopyleft Category = "strong_copyleft" // 强传染 (GPL/AGPL)
	CategoryCustom         Category = "custom"          // 自定义许可证
	CategoryUnknown        Category = "unknown"         // 未知许可证
)

// ========== 合规策略 ==========

// ListType 许可证列表类型.
type ListType string

// 列表类型常量.
const (
	ListWhitelist ListType = "whitelist" // 白名单 - 允许
	ListBlacklist ListType = "blacklist" // 黑名单 - 禁止
	ListGraylist  ListType = "graylist"  // 灰名单 - 需要审批
)

// Policy 合规策略配置.
type Policy struct {
	ID          string    `json:"id"`                    // 策略ID
	Name        string    `json:"name"`                  // 策略名称
	Description string    `json:"description,omitempty"` // 策略描述
	Whitelist   []string  `json:"whitelist,omitempty"`   // 允许的许可证白名单
	Blacklist   []string  `json:"blacklist,omitempty"`   // 禁止的许可证黑名单
	Graylist    []string  `json:"graylist,omitempty"`    // 需要人工审批的灰名单
	DefaultList ListType  `json:"default_list"`          // 不在任何列表中的许可证默认处理
	CreatedAt   time.Time `json:"created_at"`            // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`            // 更新时间
}

// ScanResult 许可证扫描结果.
type ScanResult struct {
	ID         string      `json:"id"`                   // 扫描ID
	ScanType   ScanType    `json:"scan_type"`            // 扫描类型
	Target     string      `json:"target"`               // 扫描目标
	Status     ScanStatus  `json:"status"`               // 扫描状态
	Licenses   []License   `json:"licenses"`             // 发现的许可证列表
	Summary    ScanSummary `json:"summary"`              // 扫描摘要
	Violations []Violation `json:"violations,omitempty"` // 违规项
	StartedAt  time.Time   `json:"started_at"`           // 开始时间
	FinishedAt time.Time   `json:"finished_at"`          // 完成时间
	Error      string      `json:"error,omitempty"`      // 错误信息
}

// ScanType 扫描类型.
type ScanType string

// 扫描类型常量.
const (
	ScanTypeDocker ScanType = "docker" // Docker镜像扫描
	ScanTypeGoMod  ScanType = "go_mod" // Go依赖扫描
	ScanTypeFull   ScanType = "full"   // 全量扫描
)

// ScanStatus 扫描状态.
type ScanStatus string

// 扫描状态常量.
const (
	StatusPending  ScanStatus = "pending"  // 待执行
	StatusRunning  ScanStatus = "running"  // 执行中
	StatusComplete ScanStatus = "complete" // 完成
	StatusFailed   ScanStatus = "failed"   // 失败
)

// License 许可证信息.
type License struct {
	Name       string     `json:"name"`              // 许可证名称
	SPDXID     string     `json:"spdx_id,omitempty"` // SPDX标识符
	Category   Category   `json:"category"`          // 许可证类别
	Compliance Compliance `json:"compliance"`        // 合规状态
	Source     string     `json:"source,omitempty"`  // 来源包/镜像
	Version    string     `json:"version,omitempty"` // 版本
	URL        string     `json:"url,omitempty"`     // 许可证URL
}

// Compliance 合规状态.
type Compliance string

// 合规状态常量.
const (
	ComplianceAllowed Compliance = "allowed" // 允许
	ComplianceDenied  Compliance = "denied"  // 禁止
	ComplianceReview  Compliance = "review"  // 需要审批
	ComplianceUnknown Compliance = "unknown" // 未知
)

// Violation 合规违规项.
type Violation struct {
	LicenseName string   `json:"license_name"`     // 违规许可证名称
	Source      string   `json:"source,omitempty"` // 来源
	ListType    ListType `json:"list_type"`        // 违反的列表类型
	Severity    Severity `json:"severity"`         // 严重程度
	Message     string   `json:"message"`          // 违规描述
}

// Severity 严重程度.
type Severity string

// 严重程度常量.
const (
	SeverityLow      Severity = "low"      // 低
	SeverityMedium   Severity = "medium"   // 中
	SeverityHigh     Severity = "high"     // 高
	SeverityCritical Severity = "critical" // 严重
)

// ScanSummary 扫描摘要.
type ScanSummary struct {
	TotalLicenses  int `json:"total_licenses"`           // 总许可证数
	Allowed        int `json:"allowed"`                  // 允许数
	Denied         int `json:"denied"`                   // 禁止数
	ReviewRequired int `json:"review_required"`          // 需审批数
	Unknown        int `json:"unknown"`                  // 未知数
	TotalPackages  int `json:"total_packages,omitempty"` // 总包数
}

// ========== 扫描报告 ==========

// Report 扫描报告.
type Report struct {
	ID          string        `json:"id"`           // 报告ID
	Title       string        `json:"title"`        // 报告标题
	Format      ReportFormat  `json:"format"`       // 报告格式
	Results     []ScanResult  `json:"results"`      // 扫描结果
	Summary     ReportSummary `json:"summary"`      // 报告总摘要
	GeneratedAt time.Time     `json:"generated_at"` // 生成时间
}

// ReportFormat 报告格式.
type ReportFormat string

// 报告格式常量.
const (
	FormatJSON ReportFormat = "json" // JSON格式
	FormatHTML ReportFormat = "html" // HTML格式
)

// ReportSummary 报告总摘要.
type ReportSummary struct {
	TotalScans      int       `json:"total_scans"`      // 总扫描数
	TotalLicenses   int       `json:"total_licenses"`   // 总许可证数
	TotalViolations int       `json:"total_violations"` // 总违规数
	Compliant       int       `json:"compliant"`        // 合规数
	NonCompliant    int       `json:"non_compliant"`    // 不合规数
	NeedsReview     int       `json:"needs_review"`     // 需审查数
	ScanTime        time.Time `json:"scan_time"`        // 扫描时间
}

// ========== 仪表盘 ==========

// DashboardData 合规仪表盘数据.
type DashboardData struct {
	ComplianceRate   float64            `json:"compliance_rate"`   // 合规率百分比
	TotalScans       int                `json:"total_scans"`       // 总扫描次数
	TotalViolations  int                `json:"total_violations"`  // 总违规数
	LicenseBreakdown map[Category]int   `json:"license_breakdown"` // 许可证类别分布
	RecentScans      []ScanResult       `json:"recent_scans"`      // 最近扫描
	TopViolations    []ViolationSummary `json:"top_violations"`    // 高频违规
	PolicyStatus     PolicyStatus       `json:"policy_status"`     // 策略状态
	LastScanTime     time.Time          `json:"last_scan_time"`    // 最后扫描时间
}

// ViolationSummary 违规汇总.
type ViolationSummary struct {
	LicenseName string   `json:"license_name"` // 许可证名称
	Count       int      `json:"count"`        // 出现次数
	Severity    Severity `json:"severity"`     // 最高严重程度
}

// PolicyStatus 策略状态.
type PolicyStatus struct {
	WhitelistCount int `json:"whitelist_count"` // 白名单数量
	BlacklistCount int `json:"blacklist_count"` // 黑名单数量
	GraylistCount  int `json:"graylist_count"`  // 灰名单数量
}

// ========== 告警 ==========

// Alert 告警信息.
type Alert struct {
	ID         string      `json:"id"`         // 告警ID
	ScanID     string      `json:"scan_id"`    // 关联扫描ID
	Severity   Severity    `json:"severity"`   // 严重程度
	Message    string      `json:"message"`    // 告警信息
	Violations []Violation `json:"violations"` // 违规项
	CreatedAt  time.Time   `json:"created_at"` // 创建时间
	Notified   bool        `json:"notified"`   // 是否已通知
}

// ========== 请求/响应类型 ==========

// ScanRequest 扫描请求.
type ScanRequest struct {
	ScanType ScanType `json:"scan_type"`           // 扫描类型
	Target   string   `json:"target"`              // 扫描目标 (镜像名/路径)
	PolicyID string   `json:"policy_id,omitempty"` // 使用的策略ID
}

// ScanListResponse 扫描列表响应.
type ScanListResponse struct {
	Scans []ScanResult `json:"scans"` // 扫描结果列表
	Total int          `json:"total"` // 总数
}

// PolicyListResponse 策略列表响应.
type PolicyListResponse struct {
	Policies []Policy `json:"policies"` // 策略列表
	Total    int      `json:"total"`    // 总数
}

// AlertListResponse 告警列表响应.
type AlertListResponse struct {
	Alerts []Alert `json:"alerts"` // 告警列表
	Total  int     `json:"total"`  // 总数
}

// ReportListResponse 报告列表响应.
type ReportListResponse struct {
	Reports []Report `json:"reports"` // 报告列表
	Total   int      `json:"total"`   // 总数
}
