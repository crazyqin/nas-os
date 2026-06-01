// Package compliancedashboard 实现企业合规仪表板
// 支持 GDPR、等保2.0、ISO27001 等合规标准
// 提供安全评分、漏洞扫描、数据分类、访问审计、合规报告生成
package compliancedashboard

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNotRunning 引擎未运行.
	ErrNotRunning = errors.New("compliance engine not running")
	// ErrReportNotFound 报告不存在.
	ErrReportNotFound = errors.New("report not found")
	// ErrTaskNotFound 定时任务不存在.
	ErrTaskNotFound = errors.New("scheduled task not found")
	// ErrInvalidConfig 配置无效.
	ErrInvalidConfig = errors.New("invalid configuration")
	// ErrAlreadyRunning 引擎已在运行.
	ErrAlreadyRunning = errors.New("compliance engine already running")
)

// ========== 合规框架 ==========

// ComplianceFramework 合规框架.
type ComplianceFramework string

const (
	// FrameworkGDPR GDPR通用数据保护条例.
	FrameworkGDPR ComplianceFramework = "GDPR"
	// FrameworkMLPS2 等保2.0.
	FrameworkMLPS2 ComplianceFramework = "MLPS2.0"
	// FrameworkISO27001 ISO27001信息安全管理.
	FrameworkISO27001 ComplianceFramework = "ISO27001"
	// FrameworkSOC2 SOC2服务组织控制报告.
	FrameworkSOC2 ComplianceFramework = "SOC2"
	// FrameworkHIPAA HIPAA健康保险可携性和责任法案.
	FrameworkHIPAA ComplianceFramework = "HIPAA"
)

// ========== 合规状态 ==========

// ComplianceStatus 合规状态.
type ComplianceStatus string

const (
	// StatusCompliant 合规.
	StatusCompliant ComplianceStatus = "compliant"
	// StatusNonCompliant 不合规.
	StatusNonCompliant ComplianceStatus = "non_compliant"
	// StatusPartial 部分合规.
	StatusPartial ComplianceStatus = "partial"
	// StatusInProgress 进行中.
	StatusInProgress ComplianceStatus = "in_progress"
	// StatusNotApplicable 不适用.
	StatusNotApplicable ComplianceStatus = "not_applicable"
)

// ========== 安全评分维度 ==========

// ScoreDimension 安全评分维度.
type ScoreDimension string

const (
	// DimPasswordStrength 密码强度.
	DimPasswordStrength ScoreDimension = "password_strength"
	// DimAccessControl 访问控制.
	DimAccessControl ScoreDimension = "access_control"
	// DimEncryption 加密状态.
	DimEncryption ScoreDimension = "encryption"
	// DimBackup 备份状态.
	DimBackup ScoreDimension = "backup"
)

// ========== 漏洞严重程度 ==========

// VulnSeverity 漏洞严重程度.
type VulnSeverity string

const (
	// VulnCritical 严重漏洞.
	VulnCritical VulnSeverity = "critical"
	// VulnHigh 高危漏洞.
	VulnHigh VulnSeverity = "high"
	// VulnMedium 中危漏洞.
	VulnMedium VulnSeverity = "medium"
	// VulnLow 低危漏洞.
	VulnLow VulnSeverity = "low"
	// VulnInfo 信息级.
	VulnInfo VulnSeverity = "info"
)

// ========== 数据分类级别 ==========

// DataClassification 数据分类级别.
type DataClassification string

const (
	// DataPublic 公开数据.
	DataPublic DataClassification = "public"
	// DataInternal 内部数据.
	DataInternal DataClassification = "internal"
	// DataConfidential 机密数据.
	DataConfidential DataClassification = "confidential"
	// DataRestricted 受限数据.
	DataRestricted DataClassification = "restricted"
)

// ========== 敏感信息类型 ==========

// SensitiveDataType 敏感信息类型.
type SensitiveDataType string

const (
	// SensitivePII 个人身份信息.
	SensitivePII SensitiveDataType = "PII"
	// SensitivePHI 受保护健康信息.
	SensitivePHI SensitiveDataType = "PHI"
	// SensitivePCI 支付卡信息.
	SensitivePCI SensitiveDataType = "PCI"
	// SensitiveCredential 凭据信息.
	SensitiveCredential SensitiveDataType = "credential"
)

// ========== 报告格式 ==========

// ReportFormat 报告输出格式.
type ReportFormat string

const (
	// FormatJSON JSON格式.
	FormatJSON ReportFormat = "json"
	// FormatHTML HTML格式.
	FormatHTML ReportFormat = "html"
	// FormatPDF PDF格式.
	FormatPDF ReportFormat = "pdf"
)

// ========== 核心配置 ==========

// Config 合规仪表板配置.
type Config struct {
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// EnabledFrameworks 启用的合规框架.
	EnabledFrameworks []ComplianceFramework `json:"enabledFrameworks"`
	// AutoScan 是否自动扫描.
	AutoScan bool `json:"autoScan"`
	// ScanIntervalHours 扫描间隔（小时）.
	ScanIntervalHours int `json:"scanIntervalHours"`
	// AlertThreshold 告警阈值（合规分数低于此值触发告警）.
	AlertThreshold float64 `json:"alertThreshold"`
	// NotifyEmail 通知邮箱.
	NotifyEmail string `json:"notifyEmail"`
	// RetentionDays 报告保留天数.
	RetentionDays int `json:"retentionDays"`
	// DataScanPaths 数据扫描路径.
	DataScanPaths []string `json:"dataScanPaths"`
}

// ========== 安全评分 ==========

// SecurityScore 安全评分.
type SecurityScore struct {
	// Overall 总分 (0-100).
	Overall float64 `json:"overall"`
	// Dimensions 各维度评分.
	Dimensions map[ScoreDimension]DimensionScore `json:"dimensions"`
	// CalculatedAt 计算时间.
	CalculatedAt time.Time `json:"calculatedAt"`
}

// DimensionScore 维度评分详情.
type DimensionScore struct {
	// Score 分数 (0-100).
	Score float64 `json:"score"`
	// Weight 权重.
	Weight float64 `json:"weight"`
	// Items 评分项.
	Items []ScoreItem `json:"items"`
	// Description 说明.
	Description string `json:"description"`
}

// ScoreItem 评分明细项.
type ScoreItem struct {
	// Name 名称.
	Name string `json:"name"`
	// Score 分数.
	Score float64 `json:"score"`
	// MaxScore 满分.
	MaxScore float64 `json:"maxScore"`
	// Status 状态.
	Status ComplianceStatus `json:"status"`
	// Detail 详情.
	Detail string `json:"detail"`
}

// ========== 合规检查 ==========

// ComplianceCheck 合规检查项.
type ComplianceCheck struct {
	// ID 检查项ID.
	ID string `json:"id"`
	// Framework 所属框架.
	Framework ComplianceFramework `json:"framework"`
	// Category 分类.
	Category string `json:"category"`
	// Name 名称.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description"`
	// Status 状态.
	Status ComplianceStatus `json:"status"`
	// Score 分数.
	Score float64 `json:"score"`
	// MaxScore 满分.
	MaxScore float64 `json:"maxScore"`
	// Severity 严重程度.
	Severity string `json:"severity"`
	// Evidence 证据.
	Evidence []string `json:"evidence"`
	// Remediation 整改建议.
	Remediation string `json:"remediation"`
	// LastChecked 最后检查时间.
	LastChecked time.Time `json:"lastChecked"`
	// CheckedBy 检查者.
	CheckedBy string `json:"checkedBy"`
}

// ========== 漏洞扫描 ==========

// VulnReport 漏洞扫描报告.
type VulnReport struct {
	// ID 报告ID.
	ID string `json:"id"`
	// ScanTime 扫描时间.
	ScanTime time.Time `json:"scanTime"`
	// TotalVulns 总漏洞数.
	TotalVulns int `json:"totalVulns"`
	// CriticalCount 严重漏洞数.
	CriticalCount int `json:"criticalCount"`
	// HighCount 高危漏洞数.
	HighCount int `json:"highCount"`
	// MediumCount 中危漏洞数.
	MediumCount int `json:"mediumCount"`
	// LowCount 低危漏洞数.
	LowCount int `json:"lowCount"`
	// Vulnerabilities 漏洞列表.
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	// DependencyVulns 依赖项漏洞.
	DependencyVulns []DependencyVuln `json:"dependencyVulns"`
	// ConfigIssues 配置问题.
	ConfigIssues []ConfigIssue `json:"configIssues"`
}

// Vulnerability 单个漏洞.
type Vulnerability struct {
	// ID 漏洞ID.
	ID string `json:"id"`
	// CVEID CVE编号.
	CVEID string `json:"cveId"`
	// Title 标题.
	Title string `json:"title"`
	// Description 描述.
	Description string `json:"description"`
	// Severity 严重程度.
	Severity VulnSeverity `json:"severity"`
	// CVSSScore CVSS评分.
	CVSSScore float64 `json:"cvssScore"`
	// AffectedComponent 受影响组件.
	AffectedComponent string `json:"affectedComponent"`
	// FixedVersion 修复版本.
	FixedVersion string `json:"fixedVersion"`
	// Remediation 修复建议.
	Remediation string `json:"remediation"`
	// DetectedAt 检测时间.
	DetectedAt time.Time `json:"detectedAt"`
}

// DependencyVuln 依赖项漏洞.
type DependencyVuln struct {
	// PackageName 包名.
	PackageName string `json:"packageName"`
	// CurrentVersion 当前版本.
	CurrentVersion string `json:"currentVersion"`
	// FixedVersion 修复版本.
	FixedVersion string `json:"fixedVersion"`
	// CVEID CVE编号.
	CVEID string `json:"cveId"`
	// Severity 严重程度.
	Severity VulnSeverity `json:"severity"`
	// Description 描述.
	Description string `json:"description"`
}

// ConfigIssue 配置审计问题.
type ConfigIssue struct {
	// ID 问题ID.
	ID string `json:"id"`
	// Category 分类.
	Category string `json:"category"`
	// Title 标题.
	Title string `json:"title"`
	// Description 描述.
	Description string `json:"description"`
	// Severity 严重程度.
	Severity VulnSeverity `json:"severity"`
	// CurrentValue 当前值.
	CurrentValue string `json:"currentValue"`
	// RecommendedValue 推荐值.
	RecommendedValue string `json:"recommendedValue"`
	// Remediation 整改建议.
	Remediation string `json:"remediation"`
}

// ========== 数据分类与敏感信息 ==========

// DataScanResult 数据扫描结果.
type DataScanResult struct {
	// ID 结果ID.
	ID string `json:"id"`
	// ScanTime 扫描时间.
	ScanTime time.Time `json:"scanTime"`
	// TotalFiles 扫描文件总数.
	TotalFiles int `json:"totalFiles"`
	// SensitiveFiles 敏感文件数.
	SensitiveFiles int `json:"sensitiveFiles"`
	// Findings 发现列表.
	Findings []SensitiveFinding `json:"findings"`
	// Classification 分类统计.
	Classification map[DataClassification]int `json:"classification"`
}

// SensitiveFinding 敏感信息发现.
type SensitiveFinding struct {
	// ID 发现ID.
	ID string `json:"id"`
	// FilePath 文件路径.
	FilePath string `json:"filePath"`
	// DataType 敏感数据类型.
	DataType SensitiveDataType `json:"dataType"`
	// Classification 数据分类.
	Classification DataClassification `json:"classification"`
	// MatchCount 匹配数量.
	MatchCount int `json:"matchCount"`
	// Sample 样本（脱敏）.
	Sample string `json:"sample"`
	// LineNumber 行号.
	LineNumber int `json:"lineNumber"`
	// DetectedAt 检测时间.
	DetectedAt time.Time `json:"detectedAt"`
}

// ========== 访问控制审计 ==========

// AccessAuditResult 访问审计结果.
type AccessAuditResult struct {
	// ID 结果ID.
	ID string `json:"id"`
	// AuditTime 审计时间.
	AuditTime time.Time `json:"auditTime"`
	// AnomalyLogins 异常登录检测.
	AnomalyLogins []AnomalyLogin `json:"anomalyLogins"`
	// PermissionChanges 权限变更记录.
	PermissionChanges []PermissionChange `json:"permissionChanges"`
	// Summary 摘要.
	Summary AccessAuditSummary `json:"summary"`
}

// AnomalyLogin 异常登录.
type AnomalyLogin struct {
	// ID 记录ID.
	ID string `json:"id"`
	// UserID 用户ID.
	UserID string `json:"userId"`
	// UserName 用户名.
	UserName string `json:"userName"`
	// SourceIP 来源IP.
	SourceIP string `json:"sourceIp"`
	// LoginTime 登录时间.
	LoginTime time.Time `json:"loginTime"`
	// AnomalyType 异常类型.
	AnomalyType string `json:"anomalyType"`
	// Description 描述.
	Description string `json:"description"`
	// RiskLevel 风险等级.
	RiskLevel string `json:"riskLevel"`
	// Location 地理位置.
	Location string `json:"location"`
}

// PermissionChange 权限变更.
type PermissionChange struct {
	// ID 记录ID.
	ID string `json:"id"`
	// Timestamp 时间.
	Timestamp time.Time `json:"timestamp"`
	// OperatorID 操作者ID.
	OperatorID string `json:"operatorId"`
	// OperatorName 操作者名.
	OperatorName string `json:"operatorName"`
	// TargetUser 目标用户.
	TargetUser string `json:"targetUser"`
	// ResourceType 资源类型.
	ResourceType string `json:"resourceType"`
	// ResourcePath 资源路径.
	ResourcePath string `json:"resourcePath"`
	// OldPermissions 旧权限.
	OldPermissions string `json:"oldPermissions"`
	// NewPermissions 新权限.
	NewPermissions string `json:"newPermissions"`
	// Action 操作（grant/revoke/modify）.
	Action string `json:"action"`
	// Reason 原因.
	Reason string `json:"reason"`
}

// AccessAuditSummary 访问审计摘要.
type AccessAuditSummary struct {
	// TotalLogins 总登录次数.
	TotalLogins int `json:"totalLogins"`
	// FailedLogins 失败登录次数.
	FailedLogins int `json:"failedLogins"`
	// AnomalyCount 异常登录数.
	AnomalyCount int `json:"anomalyCount"`
	// PermissionChangeCount 权限变更数.
	PermissionChangeCount int `json:"permissionChangeCount"`
	// UniqueUsers 活跃用户数.
	UniqueUsers int `json:"uniqueUsers"`
	// RiskScore 风险评分.
	RiskScore float64 `json:"riskScore"`
}

// ========== 整改建议引擎 ==========

// ========== 整改建议引擎 ==========

// Remediation 整改建议.
type Remediation struct {
	// ID 建议ID.
	ID string `json:"id"`
	// FindingID 关联发现项ID.
	FindingID string `json:"findingId"`
	// Priority 优先级 (1-5, 1最高).
	Priority int `json:"priority"`
	// Title 标题.
	Title string `json:"title"`
	// Description 描述.
	Description string `json:"description"`
	// Steps 整改步骤.
	Steps []string `json:"steps"`
	// EffortLevel 工作量 (low/medium/high).
	EffortLevel string `json:"effortLevel"`
	// ImpactLevel 影响级别.
	ImpactLevel string `json:"impactLevel"`
	// EstimatedDays 预估天数.
	EstimatedDays int `json:"estimatedDays"`
	// Status 状态 (pending/in_progress/completed).
	Status string `json:"status"`
	// AssignedTo 指派人.
	AssignedTo string `json:"assignedTo"`
	// DueDate 截止日期.
	DueDate *time.Time `json:"dueDate,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
}

// ========== 定时审计任务 ==========

// ScheduledTask 定时审计任务.
type ScheduledTask struct {
	// ID 任务ID.
	ID string `json:"id"`
	// Name 任务名.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description"`
	// Framework 合规框架.
	Framework ComplianceFramework `json:"framework"`
	// CronExpr Cron表达式.
	CronExpr string `json:"cronExpr"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// LastRun 上次运行时间.
	LastRun *time.Time `json:"lastRun,omitempty"`
	// NextRun 下次运行时间.
	NextRun *time.Time `json:"nextRun,omitempty"`
	// LastStatus 上次状态.
	LastStatus string `json:"lastStatus"`
	// RunCount 运行次数.
	RunCount int `json:"runCount"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
}

// ========== 仪表板统计 ==========
