// Package compliancescanner 提供安全合规扫描功能
package compliancescanner

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrRuleNotFound 规则不存在.
	ErrRuleNotFound = errors.New("规则不存在")
	// ErrScanInProgress 扫描正在进行中.
	ErrScanInProgress = errors.New("扫描正在进行中")
	// ErrInvalidConfig 无效配置.
	ErrInvalidConfig = errors.New("无效的配置")
	// ErrRemediationFailed 修复失败.
	ErrRemediationFailed = errors.New("修复失败")
	// ErrScheduleConflict 调度冲突.
	ErrScheduleConflict = errors.New("调度冲突")
)

// ========== 严重级别 ==========

// SeverityLevel 严重级别.
type SeverityLevel string

const (
	// SeverityCritical 严重.
	SeverityCritical SeverityLevel = "critical"
	// SeverityHigh 高危.
	SeverityHigh SeverityLevel = "high"
	// SeverityMedium 中危.
	SeverityMedium SeverityLevel = "medium"
	// SeverityLow 低危.
	SeverityLow SeverityLevel = "low"
	// SeverityInfo 信息.
	SeverityInfo SeverityLevel = "info"
)

// ========== 扫描状态 ==========

// ScanStatus 扫描状态.
type ScanStatus string

const (
	// StatusPending 等待中.
	StatusPending ScanStatus = "pending"
	// StatusRunning 运行中.
	StatusRunning ScanStatus = "running"
	// StatusCompleted 已完成.
	StatusCompleted ScanStatus = "completed"
	// StatusFailed 失败.
	StatusFailed ScanStatus = "failed"
	// StatusCancelled 已取消.
	StatusCancelled ScanStatus = "cancelled"
)

// ========== 检查结果 ==========

// CheckResult 检查结果.
type CheckResult string

const (
	// ResultPass 通过.
	ResultPass CheckResult = "pass"
	// ResultFail 失败.
	ResultFail CheckResult = "fail"
	// ResultWarning 警告.
	ResultWarning CheckResult = "warning"
	// ResultSkip 跳过.
	ResultSkip CheckResult = "skip"
	// ResultError 错误.
	ResultError CheckResult = "error"
)

// ========== 扫描类别 ==========

// ScanCategory 扫描类别.
type ScanCategory string

const (
	// CategorySystemConfig 系统配置.
	CategorySystemConfig ScanCategory = "system_config"
	// CategoryFilePermission 文件权限.
	CategoryFilePermission ScanCategory = "file_permission"
	// CategoryNetworkSecurity 网络安全.
	CategoryNetworkSecurity ScanCategory = "network_security"
	// CategoryServiceSecurity 服务安全.
	CategoryServiceSecurity ScanCategory = "service_security"
	// CategoryCryptoCompliance 加密合规.
	CategoryCryptoCompliance ScanCategory = "crypto_compliance"
)

// ========== 合规标准 ==========

// ComplianceStandard 合规标准.
type ComplianceStandard string

const (
	// StandardCIS CIS基准.
	StandardCIS ComplianceStandard = "cis"
	// StandardMLPS2 等保2.0.
	StandardMLPS2 ComplianceStandard = "mlps2"
	// StandardCustom 自定义标准.
	StandardCustom ComplianceStandard = "custom"
)

// ========== 核心数据结构 ==========

// ScanRule 扫描规则.
type ScanRule struct {
	// ID 规则ID.
	ID string `json:"id"`
	// Name 规则名称.
	Name string `json:"name"`
	// Description 规则描述.
	Description string `json:"description"`
	// Standard 合规标准.
	Standard ComplianceStandard `json:"standard"`
	// Category 扫描类别.
	Category ScanCategory `json:"category"`
	// Severity 严重级别.
	Severity SeverityLevel `json:"severity"`
	// CheckFunc 检查函数名称.
	CheckFunc string `json:"check_func"`
	// RemediationFunc 修复函数名称.
	RemediationFunc string `json:"remediation_func,omitempty"`
	// Remediation建议 修复建议.
	RemediationAdvice string `json:"remediation_advice,omitempty"`
	// References 参考链接.
	References []string `json:"references,omitempty"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// Tags 标签.
	Tags []string `json:"tags,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// ScanResult 扫描结果.
type ScanResult struct {
	// ID 结果ID.
	ID string `json:"id"`
	// ScanID 扫描ID.
	ScanID string `json:"scan_id"`
	// RuleID 规则ID.
	RuleID string `json:"rule_id"`
	// RuleName 规则名称.
	RuleName string `json:"rule_name"`
	// Category 扫描类别.
	Category ScanCategory `json:"category"`
	// Severity 严重级别.
	Severity SeverityLevel `json:"severity"`
	// Result 检查结果.
	Result CheckResult `json:"result"`
	// Details 详情.
	Details string `json:"details,omitempty"`
	// Evidence 证据.
	Evidence string `json:"evidence,omitempty"`
	// Remediation 修复建议.
	Remediation string `json:"remediation,omitempty"`
	// CheckedAt 检查时间.
	CheckedAt time.Time `json:"checked_at"`
	// Duration 检查耗时.
	Duration time.Duration `json:"duration_ms"`
	// Metadata 元数据.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Vulnerability 漏洞信息.
type Vulnerability struct {
	// ID 漏洞ID.
	ID string `json:"id"`
	// CVEID CVE编号.
	CVEID string `json:"cve_id,omitempty"`
	// Name 漏洞名称.
	Name string `json:"name"`
	// Description 漏洞描述.
	Description string `json:"description"`
	// Severity 严重级别.
	Severity SeverityLevel `json:"severity"`
	// CVSSScore CVSS评分.
	CVSSScore float64 `json:"cvss_score,omitempty"`
	// AffectedComponents 影响范围.
	AffectedComponents []string `json:"affected_components"`
	// Remediation 修复方案.
	Remediation string `json:"remediation"`
	// References 参考链接.
	References []string `json:"references,omitempty"`
	// DiscoveredAt 发现时间.
	DiscoveredAt time.Time `json:"discovered_at"`
	// IsResolved 是否已修复.
	IsResolved bool `json:"is_resolved"`
	// ResolvedAt 修复时间.
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	// ID 报告ID.
	ID string `json:"id"`
	// ScanID 扫描ID.
	ScanID string `json:"scan_id"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
	// StartTime 开始时间.
	StartTime time.Time `json:"start_time"`
	// EndTime 结束时间.
	EndTime time.Time `json:"end_time"`
	// Duration 扫描耗时.
	Duration time.Duration `json:"duration_ms"`
	// OverallScore 总体评分 (0-100).
	OverallScore float64 `json:"overall_score"`
	// TotalChecks 总检查数.
	TotalChecks int `json:"total_checks"`
	// PassedChecks 通过数.
	PassedChecks int `json:"passed_checks"`
	// FailedChecks 失败数.
	FailedChecks int `json:"failed_checks"`
	// WarningChecks 警告数.
	WarningChecks int `json:"warning_checks"`
	// SkippedChecks 跳过数.
	SkippedChecks int `json:"skipped_checks"`
	// ErrorChecks 错误数.
	ErrorChecks int `json:"error_checks"`
	// Results 扫描结果列表.
	Results []ScanResult `json:"results"`
	// Vulnerabilities 漏洞列表.
	Vulnerabilities []Vulnerability `json:"vulnerabilities,omitempty"`
	// CategorySummary 分类摘要.
	CategorySummary []CategorySummary `json:"category_summary"`
	// SeveritySummary 严重级别摘要.
	SeveritySummary []SeveritySummary `json:"severity_summary"`
	// Recommendations 整改建议.
	Recommendations []Recommendation `json:"recommendations"`
	// ComplianceLevel 合规等级 (A/B/C/D).
	ComplianceLevel string `json:"compliance_level"`
	// Standards 扫描的合规标准.
	Standards []ComplianceStandard `json:"standards"`
}

// CategorySummary 分类摘要.
type CategorySummary struct {
	// Category 扫描类别.
	Category ScanCategory `json:"category"`
	// Total 总数.
	Total int `json:"total"`
	// Passed 通过数.
	Passed int `json:"passed"`
	// Failed 失败数.
	Failed int `json:"failed"`
	// Warnings 警告数.
	Warnings int `json:"warnings"`
	// Score 得分.
	Score float64 `json:"score"`
}

// SeveritySummary 严重级别摘要.
type SeveritySummary struct {
	// Severity 严重级别.
	Severity SeverityLevel `json:"severity"`
	// Total 总数.
	Total int `json:"total"`
	// Failed 失败数.
	Failed int `json:"failed"`
	// Warnings 警告数.
	Warnings int `json:"warnings"`
}

// Recommendation 整改建议.
type Recommendation struct {
	// ID 建议ID.
	ID string `json:"id"`
	// Priority 优先级.
	Priority SeverityLevel `json:"priority"`
	// Category 扫描类别.
	Category ScanCategory `json:"category"`
	// Title 标题.
	Title string `json:"title"`
	// Description 描述.
	Description string `json:"description"`
	// Actions 操作步骤.
	Actions []string `json:"actions"`
	// References 参考链接.
	References []string `json:"references,omitempty"`
	// EstimatedTime 预计耗时.
	EstimatedTime string `json:"estimated_time,omitempty"`
}

// ScanSchedule 扫描调度.
type ScanSchedule struct {
	// ID 调度ID.
	ID string `json:"id"`
	// Name 调度名称.
	Name string `json:"name"`
	// CronExpr Cron表达式.
	CronExpr string `json:"cron_expr"`
	// Standards 合规标准.
	Standards []ComplianceStandard `json:"standards"`
	// Categories 扫描类别.
	Categories []ScanCategory `json:"categories,omitempty"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// LastRun 上次运行时间.
	LastRun *time.Time `json:"last_run,omitempty"`
	// NextRun 下次运行时间.
	NextRun *time.Time `json:"next_run,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// ScanConfig 扫描配置.
type ScanConfig struct {
	// Standards 要扫描的合规标准.
	Standards []ComplianceStandard `json:"standards"`
	// Categories 要扫描的类别（空表示全部）.
	Categories []ScanCategory `json:"categories,omitempty"`
	// SkipCategories 要跳过的类别.
	SkipCategories []ScanCategory `json:"skip_categories,omitempty"`
	// SkipRules 要跳过的规则ID.
	SkipRules []string `json:"skip_rules,omitempty"`
	// IncludeDisabled 是否包含禁用规则.
	IncludeDisabled bool `json:"include_disabled"`
	// AutoRemediate 是否自动修复.
	AutoRemediate bool `json:"auto_remediate"`
	// MaxConcurrent 最大并发数.
	MaxConcurrent int `json:"max_concurrent"`
	// Timeout 超时时间.
	Timeout time.Duration `json:"timeout"`
	// OutputDir 输出目录.
	OutputDir string `json:"output_dir"`
}

// DefaultScanConfig 返回默认扫描配置.
func DefaultScanConfig() *ScanConfig {
	return &ScanConfig{
		Standards:     []ComplianceStandard{StandardCIS, StandardMLPS2},
		MaxConcurrent: 5,
		Timeout:       30 * time.Minute,
		OutputDir:     "/var/log/nas-os/compliance",
	}
}

// ScanStats 扫描统计.
type ScanStats struct {
	// TotalScans 总扫描次数.
	TotalScans int `json:"total_scans"`
	// LastScanTime 上次扫描时间.
	LastScanTime *time.Time `json:"last_scan_time,omitempty"`
	// AverageScore 平均得分.
	AverageScore float64 `json:"average_score"`
	// PassRate 通过率.
	PassRate float64 `json:"pass_rate"`
	// TopFailedCategories 最常失败的分类.
	TopFailedCategories []string `json:"top_failed_categories"`
	// VulnerabilitiesFound 发现的漏洞数.
	VulnerabilitiesFound int `json:"vulnerabilities_found"`
	// VulnerabilitiesResolved 已修复的漏洞数.
	VulnerabilitiesResolved int `json:"vulnerabilities_resolved"`
}

// RuleUpdateInfo 规则更新信息.
type RuleUpdateInfo struct {
	// Version 规则版本.
	Version string `json:"version"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
	// Changes 更新内容.
	Changes []RuleChange `json:"changes"`
}

// RuleChange 规则变更.
type RuleChange struct {
	// RuleID 规则ID.
	RuleID string `json:"rule_id"`
	// ChangeType 变更类型 (added/modified/deleted).
	ChangeType string `json:"change_type"`
	// Description 变更描述.
	Description string `json:"description"`
}

// RemediationRecord 修复记录.
type RemediationRecord struct {
	// ID 记录ID.
	ID string `json:"id"`
	// RuleID 规则ID.
	RuleID string `json:"rule_id"`
	// ResultID 结果ID.
	ResultID string `json:"result_id"`
	// Action 修复动作.
	Action string `json:"action"`
	// Status 修复状态.
	Status string `json:"status"`
	// Details 修复详情.
	Details string `json:"details,omitempty"`
	// ExecutedAt 执行时间.
	ExecutedAt time.Time `json:"executed_at"`
	// Duration 修复耗时.
	Duration time.Duration `json:"duration_ms"`
	// ExecutedBy 执行者.
	ExecutedBy string `json:"executed_by"`
	// Verified 是否已验证.
	Verified bool `json:"verified"`
	// VerifiedAt 验证时间.
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}
