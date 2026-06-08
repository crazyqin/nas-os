package securityaudit

import "time"

// ========== 安全配置检查类型 ==========

// SecurityCheckCategory 安全检查类别.
type SecurityCheckCategory string

const (
	CategoryAuth       SecurityCheckCategory = "auth"       // 认证安全
	CategoryNetwork    SecurityCheckCategory = "network"    // 网络安全
	CategorySystem     SecurityCheckCategory = "system"     // 系统安全
	CategoryFile       SecurityCheckCategory = "file"       // 文件系统安全
	CategoryCrypto     SecurityCheckCategory = "crypto"     // 加密安全
	CategoryAccess     SecurityCheckCategory = "access"     // 访问控制
	CategoryPatch      SecurityCheckCategory = "patch"      // 补丁管理
	CategoryBackup     SecurityCheckCategory = "backup"     // 备份安全
	CategoryContainer  SecurityCheckCategory = "container"  // 容器安全
	CategoryCompliance SecurityCheckCategory = "compliance" // 合规检查
)

// SecurityCheckSeverity 安全检查严重程度.
type SecurityCheckSeverity string

const (
	SeverityLow      SecurityCheckSeverity = "low"
	SeverityMedium   SecurityCheckSeverity = "medium"
	SeverityHigh     SecurityCheckSeverity = "high"
	SeverityCritical SecurityCheckSeverity = "critical"
)

// SecurityCheckStatus 安全检查状态.
type SecurityCheckStatus string

const (
	StatusPass    SecurityCheckStatus = "pass"
	StatusFail    SecurityCheckStatus = "fail"
	StatusWarning SecurityCheckStatus = "warning"
	StatusSkip    SecurityCheckStatus = "skip"
)

// SecurityCheck 安全检查项定义.
type SecurityCheck struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Category    SecurityCheckCategory `json:"category"`
	Severity    SecurityCheckSeverity `json:"severity"`
	Enabled     bool                  `json:"enabled"`
	Remediation string                `json:"remediation,omitempty"` // 修复建议
}

// SecurityCheckResult 安全检查结果.
type SecurityCheckResult struct {
	CheckID     string                `json:"check_id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Category    SecurityCheckCategory `json:"category"`
	Severity    SecurityCheckSeverity `json:"severity"`
	Status      SecurityCheckStatus   `json:"status"`
	Message     string                `json:"message"`
	Remediation string                `json:"remediation,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	CheckedAt   time.Time             `json:"checked_at"`
}

// ========== 安全评分类型 ==========

// SecurityScore 安全评分.
type SecurityScore struct {
	Overall     int                    `json:"overall"`      // 0-100 总分
	Auth        int                    `json:"auth"`         // 认证评分
	Network     int                    `json:"network"`      // 网络评分
	System      int                    `json:"system"`       // 系统评分
	File        int                    `json:"file"`         // 文件评分
	Crypto      int                    `json:"crypto"`       // 加密评分
	Access      int                    `json:"access"`       // 访问控制评分
	Patch       int                    `json:"patch"`        // 补丁评分
	Backup      int                    `json:"backup"`       // 备份评分
	Container   int                    `json:"container"`    // 容器评分
	Compliance  int                    `json:"compliance"`   // 合规评分
	Grade       string                 `json:"grade"`        // A+, A, B+, B, C+, C, D, F
	Trend       string                 `json:"trend"`        // up, down, stable
	Details     map[string]interface{} `json:"details,omitempty"`
	CalculatedAt time.Time             `json:"calculated_at"`
}

// SecurityScoreHistory 安全评分历史.
type SecurityScoreHistory struct {
	Timestamp time.Time `json:"timestamp"`
	Score     int       `json:"score"`
	Grade     string    `json:"grade"`
}

// ========== 漏洞扫描类型 ==========

// VulnerabilitySeverity 漏洞严重程度.
type VulnerabilitySeverity string

const (
	VulnSeverityLow      VulnerabilitySeverity = "low"
	VulnSeverityMedium   VulnerabilitySeverity = "medium"
	VulnSeverityHigh     VulnerabilitySeverity = "high"
	VulnSeverityCritical VulnerabilitySeverity = "critical"
)

// VulnerabilityStatus 漏洞状态.
type VulnerabilityStatus string

const (
	VulnStatusOpen      VulnerabilityStatus = "open"
	VulnStatusConfirmed VulnerabilityStatus = "confirmed"
	VulnStatusFixed     VulnerabilityStatus = "fixed"
	VulnStatusAccepted  VulnerabilityStatus = "accepted"
	VulnStatusFalse     VulnerabilityStatus = "false_positive"
)

// Vulnerability 漏洞条目.
type Vulnerability struct {
	ID          string                `json:"id"`
	CVEID       string                `json:"cve_id,omitempty"` // CVE 编号
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Severity    VulnerabilitySeverity `json:"severity"`
	Status      VulnerabilityStatus   `json:"status"`
	Category    string                `json:"category"`     // system, package, config, network
	Affected    string                `json:"affected"`     // 受影响的组件
	Version     string                `json:"version"`      // 受影响的版本
	FixedIn     string                `json:"fixed_in,omitempty"` // 修复版本
	CVSSScore   float64               `json:"cvss_score"`   // CVSS 评分 0-10
	Solution    string                `json:"solution"`     // 解决方案
	References  []string              `json:"references"`   // 参考链接
	FoundAt     time.Time             `json:"found_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// VulnerabilityScanConfig 漏洞扫描配置.
type VulnerabilityScanConfig struct {
	ScanPackages bool     `json:"scan_packages"` // 扫描系统包
	ScanServices bool     `json:"scan_services"` // 扫描服务漏洞
	ScanConfig   bool     `json:"scan_config"`   // 扫描配置漏洞
	ScanNetwork  bool     `json:"scan_network"`  // 扫描网络漏洞
	ScanPorts    bool     `json:"scan_ports"`    // 扫描开放端口
	ExcludeCVEs  []string `json:"exclude_cves"`  // 排除的 CVE
	AutoFix      bool     `json:"auto_fix"`      // 自动修复
}

// VulnerabilityScanReport 漏洞扫描报告.
type VulnerabilityScanReport struct {
	ReportID         string              `json:"report_id"`
	ScanTime         time.Time           `json:"scan_time"`
	Duration         time.Duration       `json:"duration"`
	TotalFound       int                 `json:"total_found"`
	CriticalCount    int                 `json:"critical_count"`
	HighCount        int                 `json:"high_count"`
	MediumCount      int                 `json:"medium_count"`
	LowCount         int                 `json:"low_count"`
	FixedCount       int                 `json:"fixed_count"`
	Vulnerabilities  []Vulnerability     `json:"vulnerabilities"`
	Summary          string              `json:"summary"`
	Recommendations  []string            `json:"recommendations"`
	NextScanTime     *time.Time          `json:"next_scan_time,omitempty"`
}

// ========== 安全加固建议类型 ==========

// HardeningCategory 加固类别.
type HardeningCategory string

const (
	HardeningAuth       HardeningCategory = "auth"       // 认证加固
	HardeningNetwork    HardeningCategory = "network"    // 网络加固
	HardeningSystem     HardeningCategory = "system"     // 系统加固
	HardeningFile       HardeningCategory = "file"       // 文件系统加固
	HardeningCrypto     HardeningCategory = "crypto"     // 加密加固
	HardeningAccess     HardeningCategory = "access"     // 访问控制加固
	HardeningPatch      HardeningCategory = "patch"      // 补丁管理
	HardeningBackup     HardeningCategory = "backup"     // 备份加固
	HardeningContainer  HardeningCategory = "container"  // 容器加固
	HardeningAudit      HardeningCategory = "audit"      // 审计加固
)

// HardeningPriority 加固优先级.
type HardeningPriority string

const (
	PriorityLow      HardeningPriority = "low"
	PriorityMedium   HardeningPriority = "medium"
	PriorityHigh     HardeningPriority = "high"
	PriorityCritical HardeningPriority = "critical"
)

// HardeningEffort 加固工作量.
type HardeningEffort string

const (
	EffortLow    HardeningEffort = "low"    // 简单配置更改
	EffortMedium HardeningEffort = "medium" // 中等工作量
	EffortHigh   HardeningEffort = "high"   // 复杂操作
)

// HardeningSuggestion 安全加固建议.
type HardeningSuggestion struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Category    HardeningCategory `json:"category"`
	Priority    HardeningPriority `json:"priority"`
	Effort      HardeningEffort   `json:"effort"`
	Impact      string            `json:"impact"`     // 预期效果
	Steps       []string          `json:"steps"`      // 实施步骤
	Commands    []string          `json:"commands"`   // 相关命令
	References  []string          `json:"references"` // 参考文档
	Applied     bool              `json:"applied"`    // 是否已应用
	AppliedAt   *time.Time        `json:"applied_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// HardeningReport 加固建议报告.
type HardeningReport struct {
	ReportID      string               `json:"report_id"`
	GeneratedAt   time.Time            `json:"generated_at"`
	TotalItems    int                  `json:"total_items"`
	CriticalCount int                  `json:"critical_count"`
	HighCount     int                  `json:"high_count"`
	MediumCount   int                  `json:"medium_count"`
	LowCount      int                  `json:"low_count"`
	AppliedCount  int                  `json:"applied_count"`
	Suggestions   []HardeningSuggestion `json:"suggestions"`
	ScoreImpact   int                  `json:"score_impact"` // 应用所有建议后预计提升的分数
}

// ========== 审计日志类型 ==========

// AuditEventType 审计事件类型.
type AuditEventType string

const (
	EventSecurityCheck  AuditEventType = "security_check"
	EventVulnScan       AuditEventType = "vulnerability_scan"
	EventHardening      AuditEventType = "hardening"
	EventConfigChange   AuditEventType = "config_change"
	EventAccessDenied   AuditEventType = "access_denied"
	EventLogin          AuditEventType = "login"
	EventLogout         AuditEventType = "logout"
	EventPermission     AuditEventType = "permission_change"
	EventBackup         AuditEventType = "backup"
	EventRestore        AuditEventType = "restore"
)

// AuditEvent 审计事件.
type AuditEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	EventType   AuditEventType         `json:"event_type"`
	Severity    SecurityCheckSeverity  `json:"severity"`
	Actor       string                 `json:"actor"`       // 操作者
	ActorIP     string                 `json:"actor_ip"`    // 操作者 IP
	Resource    string                 `json:"resource"`    // 操作的资源
	Action      string                 `json:"action"`      // 执行的操作
	Details     map[string]interface{} `json:"details,omitempty"`
	Status      string                 `json:"status"`      // success, failure
	Message     string                 `json:"message"`
}

// AuditReport 审计报告.
type AuditReport struct {
	ReportID     string       `json:"report_id"`
	StartTime    time.Time    `json:"start_time"`
	EndTime      time.Time    `json:"end_time"`
	TotalEvents  int          `json:"total_events"`
	ByType       map[AuditEventType]int `json:"by_type"`
	BySeverity   map[SecurityCheckSeverity]int `json:"by_severity"`
	TopActors    []ActorStats `json:"top_actors"`
	TopResources []ResourceStats `json:"top_resources"`
	Timeline     []TimelineEntry `json:"timeline"`
}

// ActorStats 操作者统计.
type ActorStats struct {
	Actor     string `json:"actor"`
	EventCount int   `json:"event_count"`
	FailedCount int  `json:"failed_count"`
}

// ResourceStats 资源统计.
type ResourceStats struct {
	Resource    string `json:"resource"`
	EventCount  int    `json:"event_count"`
	LastAccess  time.Time `json:"last_access"`
}

// TimelineEntry 时间线条目.
type TimelineEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
	Events    []string  `json:"events"`
}

// ========== 安全配置类型 ==========

// SecurityAuditConfig 安全审计配置.
type SecurityAuditConfig struct {
	Enabled             bool                  `json:"enabled"`
	AutoScan            bool                  `json:"auto_scan"`             // 自动扫描
	ScanInterval        time.Duration         `json:"scan_interval"`         // 扫描间隔
	ScoreCalculation    bool                  `json:"score_calculation"`     // 启用评分计算
	HardeningEnabled    bool                  `json:"hardening_enabled"`     // 启用加固建议
	VulnScanConfig      VulnerabilityScanConfig `json:"vuln_scan_config"`
	AlertThreshold      int                   `json:"alert_threshold"`       // 告警阈值分数
	RetentionDays       int                   `json:"retention_days"`        // 日志保留天数
	NotifyOnCritical    bool                  `json:"notify_on_critical"`    // 严重漏洞通知
	AutoRemediate       bool                  `json:"auto_remediate"`        // 自动修复
}

// ========== 通用响应类型 ==========

// Response 通用 API 响应.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 创建成功响应.
func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

// Error 创建错误响应.
func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}
